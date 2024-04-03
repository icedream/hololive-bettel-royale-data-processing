package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/davecgh/go-spew/spew"
	"github.com/icedream/hololive-bettel-royale-data-processing/internal/database"
	"github.com/icedream/hololive-bettel-royale-data-processing/internal/discord"
	_ "github.com/mattn/go-sqlite3"
	"gorm.io/gorm"
)

const (
	mainChannelID     = `1224009923457847428`
	shoppingChannelID = `1224017701744410695`
	botAuthorID       = `693167035068317736`
)

type backupFile struct {
	firstMessageTime time.Time
	file             *os.File
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Need a subcommand to execute")
	}

	db, err := database.OpenSQLite("main.db")
	if err != nil {
		panic(err)
	}

	switch os.Args[1] {
	case "dump":
		if err := runDump(db); err != nil {
			panic(err)
		}

	case "import":
		if err := runImport(db); err != nil {
			panic(err)
		}

	case "reset":
		if err := db.AutoMigrate(); err != nil {
			panic(err)
		}
		if err := db.Export(os.Stdout); err != nil {
			panic(err)
		}

	default:
		log.Fatal("Invalid subcommand")
	}
}

type channelState struct {
	LastKnownIsGameRunning bool
	LastKnownGame          database.Game
	LastKnownRound         database.Round
}

type Processor struct {
	db *database.Database

	channels        map[string]*channelState
	LastKnownGameID int
}

func runDump(db *database.Database) error {
	// figure out last effective update time
	timestamps := []time.Time{}
	var lastGame database.Game
	if tx := db.GORM().Last(&lastGame); errors.Is(tx.Error, gorm.ErrRecordNotFound) {
	} else if tx.Error != nil {
		return tx.Error
	} else {
		timestamps = append(timestamps, lastGame.CountdownStartTime)
		if lastGame.StartTime != nil {
			timestamps = append(timestamps, *lastGame.EndTime)
		}
		if lastGame.EndTime != nil {
			timestamps = append(timestamps, *lastGame.EndTime)
		}
	}
	var lastRound database.Round
	if tx := db.GORM().Last(&lastRound); errors.Is(tx.Error, gorm.ErrRecordNotFound) {
	} else if tx.Error != nil {
		return tx.Error
	} else {
		timestamps = append(timestamps, lastRound.PostTime)
	}
	var latestTimestamp time.Time
	for _, ts := range timestamps {
		if ts.After(latestTimestamp) {
			latestTimestamp = ts
		}
	}

	if _, err := os.Stdout.WriteString(fmt.Sprintf(`--
-- Hololive Battle Royale statistics dump
--
-- This dump is entirely compatible with SQLite.
--
-- Latest game time considered in this dump: %s
--
-- DO NOT EDIT. This was autogenerated by a tool.
--

`, latestTimestamp)); err != nil {
		return err
	}

	if err := db.Export(os.Stdout); err != nil {
		return err
	}

	return nil
}

func runImport(db *database.Database) error {
	if err := db.AutoMigrate(); err != nil {
		return err
	}

	p := newProcessor(db)
	processDir := func(channelIDs ...string) error {
		backupFiles := []backupFile{}

		walkFunc := func(path string, info fs.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.EqualFold(filepath.Ext(info.Name()), ".json") {
				return nil
			}
			r, err := os.OpenFile(path, os.O_RDONLY, 0o400)
			if err != nil {
				return err
			}
			// r will be closed in second loop

			// only extract first message timestamp for sorting
			var backup struct {
				Messages []struct {
					Timestamp time.Time `json:"timestamp"`
				} `json:"messages"`
			}
			if err := json.NewDecoder(r).Decode(&backup); err != nil {
				return err
			}
			if _, err := r.Seek(0, io.SeekStart); err != nil {
				return err
			}

			firstMessage := backup.Messages[0]
			backupFiles = append(backupFiles, backupFile{
				file:             r,
				firstMessageTime: firstMessage.Timestamp,
			})

			return nil
		}

		// extract timestamps from each discord export
		for _, channelID := range channelIDs {
			if err := filepath.Walk("discord-exports/"+channelID, walkFunc); err != nil {
				return err
			}
		}

		// sort backups by which timestamps they start from so they are linear history
		slices.SortFunc(backupFiles, func(a, b backupFile) int {
			if a.firstMessageTime == b.firstMessageTime {
				return strings.Compare(a.file.Name(), b.file.Name())
			}
			if a.firstMessageTime.After(b.firstMessageTime) {
				return 1
			}
			return -1
		})

		// actually process the backups
		for _, backupFile := range backupFiles {
			var backup discord.Backup
			if err := json.NewDecoder(backupFile.file).Decode(&backup); err != nil {
				return fmt.Errorf("failed to parse message export %s: %w", backupFile.file.Name(), err)
			}
			backupFile.file.Close()
			if err := p.processExport(backup); err != nil {
				return fmt.Errorf("failure in message export %s: %w", backupFile.file.Name(), err)
			}
		}
		return nil
	}

	// if err := processDir(shoppingChannelID); err != nil {
	// 	return err
	// }
	// if err := processDir(mainChannelID); err != nil {
	// 	return err
	// }

	if err := processDir(shoppingChannelID, mainChannelID); err != nil {
		return err
	}

	return nil
}

func newProcessor(db *database.Database) *Processor {
	return &Processor{
		db:       db,
		channels: map[string]*channelState{},
	}
}

func (p *Processor) channelState(c discord.Channel) *channelState {
	state, ok := p.channels[c.ID]
	if !ok {
		state = &channelState{}
		p.channels[c.ID] = state
	}
	return state
}

func (p *Processor) storeCurrentGame(c discord.Channel) error {
	// log.Printf("Storing game: %+v", p.LastKnownGame)
	tx := p.db.GORM().Save(&p.channelState(c).LastKnownGame)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (p *Processor) storeCurrentRound(c discord.Channel) error {
	// log.Printf("Storing round: %+v", p.LastKnownRound)
	tx := p.db.GORM().Create(&p.channelState(c).LastKnownRound)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (p *Processor) storeInteraction(i *database.Interaction) error {
	// log.Printf("Storing interaction: %+v", i)
	tx := p.db.GORM().Create(i)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (p *Processor) storeUser(u *database.User) error {
	// log.Printf("Storing user: %+v", u)
	tx := p.db.GORM().Save(u)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (p *Processor) lookupUserName(m discord.Message, name string) (*database.User, error) {
	if len(name) == 0 {
		return nil, errors.New("empty username")
	}
	u := database.UserNameObservation{}
	// next-best guess principle, also partially trust that people didn't take
	// each other's nicknames (which thanks to april fools is an even more than
	// usual unreliable unassumption... oh well, we'll see how it holds)
	tx := p.db.GORM().
		Preload("User").
		Where("name = ?", name).
		Last(&u)
	err := tx.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		log.Printf("WARNING: Could not find ID of user name %s for message ID %s, leaving null for now", name, m.ID)
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u.User, err
}

func (p *Processor) lookupUserID(id string) (database.User, error) {
	u := database.User{}
	if len(id) == 0 {
		return u, errors.New("empty username")
	}
	tx := p.db.GORM().
		Where("id = ?", id).
		First(&u)
	err := tx.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	if err != nil {
		return u, err
	}
	if tx.RowsAffected == 0 {
		u.ID = id
		err = p.storeUser(&u)
	}
	return u, err
}

func (p *Processor) storeItem(i *database.Item) error {
	// log.Printf("Storing item: %+v", i)
	tx := p.db.GORM().Save(i)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (p *Processor) lookupItem(name string) (database.Item, error) {
	i := database.Item{}
	tx := p.db.GORM().
		Where("name = ?", name).
		First(&i)
	err := tx.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	if err != nil {
		return i, err
	}
	if tx.RowsAffected == 0 {
		i.Name = name
		err = p.storeItem(&i)
	}
	return i, err
}

func (p *Processor) storeInteractionMessage(i *database.InteractionMessage) error {
	// log.Printf("Storing interaction message: %+v", i)
	tx := p.db.GORM().Save(i)
	if tx.Error != nil {
		return tx.Error
	}
	return nil
}

func (p *Processor) lookupInteractionMessage(text, event string) (database.InteractionMessage, error) {
	i := database.InteractionMessage{
		Text:  text,
		Event: event,
	}
	tx := p.db.GORM().
		Where("text = ?", text).
		Where("event = ?", event).
		First(&i)
	err := tx.Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		err = nil
	}
	if err != nil {
		return i, err
	}
	if tx.RowsAffected == 0 {
		i.Text = text
		i.Event = event
		err = p.storeInteractionMessage(&i)
	}
	return i, err
}

func (p *Processor) wrapError(msg discord.Message, err error) error {
	err = fmt.Errorf("failure at message ID %s: %w", msg.ID, err)
	return err
}

func (p *Processor) observeUserName(m discord.Message, id, name string) error {
	user, err := p.lookupUserID(id)
	if err != nil {
		return err
	}

	var lastChange database.UserNameObservation
	if tx := p.db.GORM().
		Where("user_id = ?", id).
		Last(&lastChange); tx.Error == nil {
		if lastChange.Name == name {
			return nil // this nickname is already observed to be the latest one
		}
	} else if !errors.Is(tx.Error, gorm.ErrRecordNotFound) {
		return err
	}

	// log.Printf("Observed %s/%s via message ID %s", user.ID, name, m.ID)
	lastChange = database.UserNameObservation{
		User:   user,
		UserID: user.ID,
		Name:   name,
		Time:   m.Timestamp,
	}
	if tx := p.db.GORM().Create(&lastChange); tx.Error != nil {
		return tx.Error
	}

	// find games up to this point that had the host user nulled but not the name
	var games []*database.Game
	if tx := p.db.GORM().
		Where(`host_user_id IS NULL`).
		Where(`host_user_name = ?`, name).
		Find(&games); tx.Error == nil {
		if len(games) > 0 {
			// update these records since we now have a user to fill out with
			for _, game := range games {
				game.HostUser = &user
				game.HostUserID = &user.ID
				if tx := p.db.GORM().Save(game); tx.Error != nil {
					return tx.Error
				}
			}
			log.Printf("WARNING: Fixed up %d games for %s/%s", len(games), user.ID, name)
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// find interactions up to this point that had user ID nulled
	var records []*database.InteractionUserMention
	if tx := p.db.GORM().
		Where(`user_id IS NULL`).
		Where(`user_name = ?`, name).
		Find(&records); tx.Error == nil {
		if len(records) > 0 {
			// update these records since we now have a user to fill out with
			for _, record := range records {
				record.User = &user
				record.UserID = &user.ID
				if tx := p.db.GORM().Save(record); tx.Error != nil {
					return tx.Error
				}
			}
			log.Printf("WARNING: Fixed up %d entries for %s/%s", len(records), user.ID, name)
		} else if name == "menardi" {
			log.Fatal("Something went wrong here")
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	return nil
}

func (p *Processor) processExport(backup discord.Backup) error {
	for _, msg := range backup.Messages {
		// in an attempt to collect up-to-message-date username changes, let's
		// try to extract all possible hints
		if msg.Interaction != nil {
			// record interacted user
			if err := p.observeUserName(msg, msg.Interaction.User.ID, msg.Interaction.User.Name); err != nil {
				return err
			}
		}
		if len(msg.Mentions) > 0 {
			// record mentioned users
			for _, mention := range msg.Mentions {
				if err := p.observeUserName(msg, mention.ID, mention.Name); err != nil {
					return err
				}
			}
		}
		if len(msg.Reactions) > 0 {
			// record reacting users
			for _, reaction := range msg.Reactions {
				for _, user := range reaction.Users {
					if err := p.observeUserName(msg, user.ID, user.Name); err != nil {
						return err
					}
				}
			}
		}
		if msg.Author.ID != botAuthorID {
			// record author user
			if err := p.observeUserName(msg, msg.Author.ID, msg.Author.Name); err != nil {
				return err
			}
			continue
		}
		if len(msg.Embeds) < 1 {
			continue
		}
	embedLoop:
		for embedIndex, embed := range msg.Embeds {
			var err error
			switch {
			case strings.Contains(embed.Title, "hosted by ") ||
				(embed.Footer != nil && embed.Footer.Text == "Automatic Session"):
				err = p.processGameCountdownStart(backup.Channel, msg, embed)
			case strings.HasPrefix(embed.Description, "Starting in "):
				// ignore continued countdown
			case strings.HasPrefix(embed.Title, "Started a new "):
				err = p.processGameStart(backup.Channel, msg, embed)
			case embed.Title == "Rumble Royale session cancelled":
				err = p.processGameCancelled(backup.Channel, msg)
				break embedLoop
			case embedIndex == 0 && strings.Contains(embed.Title, "WINNER!"):
				err = p.processGameWinner(backup.Channel, msg)
				break embedLoop
			case strings.Contains(embed.Author.Name, "'s balance") ||
				strings.Contains(embed.Author.Name, "'s backpacks") ||
				strings.Contains(embed.Author.Name, "'s Classic Era Items and Skins") &&
					msg.Interaction != nil:
				// extract recorded past nickname from responses to user-specific command
				fields := strings.SplitN(embed.Author.Name, "'", 2)
				if err := p.observeUserName(msg, msg.Interaction.User.ID, unescapeMarkdown(fields[0])); err != nil {
					return err
				}
				break embedLoop
			case strings.HasPrefix(embed.Title, "__Round "):
				err = p.processRound(backup.Channel, msg, embed)
			case embed.Description == "Your inventory is empty.":
				// ignore
			case embed.Description == "You already have this title equipped!":
				// ignore
			case strings.Contains(embed.Author.Name, "'s Profile"):
				// ignore, could have been requested by any user
			case strings.Contains(embed.Title, "'s Battle History"):
				// ignore, could have been requested by any user
			case strings.Contains(embed.Title, "Event Quests"):
				// ignore
			case strings.HasPrefix(embed.Title, "Season") && strings.HasSuffix(embed.Title, "| Overview"):
				// ignore
			case strings.HasSuffix(embed.Title, "Leaderboard:") || strings.HasPrefix(embed.Title, "Leaderboard "):
				// ignore
			case embed.Author.Name == "Title Change":
				// ignore
			case embed.Author.Name == "Quotes | View" ||
				embed.Title == "Quotes | View" ||
				embed.Title == "Quotes | Select":
				// ignore
			case embed.Author.Name == "Banners":
				// ignore
			case embed.Title == "Banners":
				// ignore
			case strings.HasPrefix(embed.Title, "COSMETICS"):
				// ignore
			case strings.Contains(embed.Title, "umble") && strings.Contains(embed.Title, "ass") && strings.Contains(embed.Title, "eason"):
				// ignore
			case strings.Contains(embed.Description, "Thanks for voting! Enjoy your free"):
				// ignore free gems for voting
			case embed.Title == "Vote for Rumble Royale":
				// ignore reward for voting
			case embed.Title == "Rumble Royale Info":
				// ignore bot describing itself
			case embed.Title == "Rumble Royale Overview":
				// ignore bot describing itself
			case embed.Title == "Rumble Royale Commands":
				// ignore bot describing itself
			case strings.Contains(embed.Title, "Era Phrases"):
				// ignore
			case embed.Title == "Black Market" || embed.Author.Name == "Black Market":
				// ignore black market messages
			case embed.Title == "We're glad you're enjoying the bot":
				// ignore support message
			case embed.Title == "Backpack Rewards!":
				// ignore
			case strings.Contains(embed.Title, "Weekly Reward"):
				// ignore
			case strings.Contains(embed.Title, "Daily Reward"):
				// ignore
			default:
				// desc, userInteractions, err := p.extractUsers(msg, embed.Description)
				// if err != nil {
				// 	return p.wrapError(msg, err)
				// }
				log.Printf("ignoring unhandled message:\n\n%s\n\n%s\n\n", msg.Content, spew.Sdump(embed))
			}
			if err != nil {
				return p.wrapError(msg, err)
			}
		}
	}

	return nil
}

const rxStrDiscordUsername = `([a-z0-9_\.\\]*[a-z0-9_\.])(\s+[^\*]+)?`

var rxUserFormatted = regexp.MustCompile(
	`(?:~~\*\*` + rxStrDiscordUsername + `\*\*~~|\*\*` + rxStrDiscordUsername + `\*\*)`)

func unescapeMarkdown(msg string) string {
	msg = strings.ReplaceAll(msg, `\_`, `_`)
	msg = strings.ReplaceAll(msg, `\*`, `*`)
	msg = strings.ReplaceAll(msg, `\.`, `.`)
	msg = strings.ReplaceAll(msg, `\\`, `\`)
	return msg
}

func (p *Processor) extractUsers(m discord.Message, msg string) (string, []database.InteractionUserMention, error) {
	matches := rxUserFormatted.FindAllStringSubmatch(msg, -1)
	userInteractions := []database.InteractionUserMention{}
	for _, match := range matches {
		var userInteraction database.InteractionUserMention
		switch {
		case len(match[1]) != 0: // ~~**USERNAME SUFFIX**~~
			userName := unescapeMarkdown(match[1])
			userInteraction = database.InteractionUserMention{
				UserName: userName,
				Killed:   true,
				Suffix:   match[2],
			}
		case len(match[3]) != 0: // **USERNAME SUFFIX**
			userName := unescapeMarkdown(match[3])
			userInteraction = database.InteractionUserMention{
				UserName: userName,
				Suffix:   match[4],
			}
		}
		index := slices.Index(userInteractions, userInteraction)
		if index < 0 {
			index = len(userInteractions)
			userInteractions = append(userInteractions, userInteraction)
		}
		msg = strings.Replace(msg, match[0], fmt.Sprintf(`{{users[%d]}}`, index), 1)
	}
	// fill out users if possible
	for i := range userInteractions {
		user, err := p.lookupUserName(m, userInteractions[i].UserName)
		if err != nil {
			return msg, nil, err
		}
		if user != nil {
			userInteractions[i].User = user
			userInteractions[i].UserID = &user.ID
		}
	}
	return msg, userInteractions, nil
}

var rxItemFormatted = regexp.MustCompile(`__([^_]+)__`)

func (p *Processor) extractItems(msg string) (string, []database.Item, error) {
	matches := rxItemFormatted.FindAllStringSubmatch(msg, -1)
	items := []database.Item{}
	if len(matches) > 1 {
		return "", nil, errors.New("more than 1 item mention found in message which is not yet supported: " + msg)
	}
	for _, match := range matches {
		item, err := p.lookupItem(unescapeMarkdown(match[1]))
		if err != nil {
			return "", nil, err
		}
		msg = strings.Replace(msg, match[0], `{{item}}`, 1)
		items = append(items, item)
	}
	return msg, items, nil
}

func filterGraphic(r rune) rune {
	if unicode.IsGraphic(r) || unicode.IsSpace(r) {
		return r
	}
	return -1
}

var rxEra = regexp.MustCompile(`(?m)Era:\s+(<:.+:.+>)?\s*([^\r\n]+?)\s*(?:$|\n)`)

func (p *Processor) createNewGame(c discord.Channel, m discord.Message, era string, hostName *string) error {
	cs := p.channelState(c)
	p.LastKnownGameID++
	cs.LastKnownGame = database.Game{
		ID:               p.LastKnownGameID,
		Era:              era,
		HostUserName:     hostName,
		DiscordChannelID: c.ID,
	}
	if hostName != nil {
		hostUser, err := p.lookupUserName(m, *hostName)
		if err != nil {
			return err
		}
		if hostUser != nil {
			cs.LastKnownGame.HostUser = hostUser
			cs.LastKnownGame.HostUserID = &hostUser.ID
		}
	}
	cs.LastKnownRound = database.Round{
		GameID: cs.LastKnownGame.ID,
		Game:   cs.LastKnownGame,
	}
	cs.LastKnownIsGameRunning = false
	return nil
}

func (p *Processor) createNewRound(c discord.Channel, roundNumber int) {
	cs := p.channelState(c)
	cs.LastKnownRound = database.Round{
		GameID:      cs.LastKnownGame.ID,
		Game:        cs.LastKnownGame,
		RoundNumber: roundNumber,
	}
}

func (p *Processor) processGameCountdownStart(c discord.Channel, m discord.Message, e discord.Embed) error {
	/*
		Embed title: "Rumble Royale hosted by astelzoom"

		Embed description:

		"Era: <:wol:696302964985298964>Classic \n\nClick the emoji below to join. Starting in 2 minutes!"
	*/

	cleanDesc := strings.Map(filterGraphic, e.Description)
	cleanTitle := strings.Map(filterGraphic, e.Title)

	// find era
	era := ""
	if match := rxEra.FindStringSubmatch(cleanDesc); match != nil {
		era = match[2]
	}

	// find host
	var hostUserName *string
	if fields := strings.SplitN(cleanTitle, " hosted by ", 2); len(fields) == 2 {
		username := unescapeMarkdown(fields[1])
		hostUserName = &username
	}

	// start countdown of new game
	if err := p.createNewGame(c, m, era, hostUserName); err != nil {
		return err
	}
	p.channelState(c).LastKnownGame.CountdownStartTime = m.Timestamp

	// log.Printf("New game counting down: %+v\n", p.LastKnownGame)
	return nil
}

func (p *Processor) processGameWinner(c discord.Channel, m discord.Message) error {
	/*
		Embed title: "<:Crwn2:872850260756664350> **__WINNER!__**"

		Embed description:

		"**technobean**\n**Reward:** 6200 <:gold:695955554199142421>\n<:xp:860094804984725504> **1.5x XP multiplier!**"

		Additional embeds contain fields listing Runners-up, Most Kills (optional), Most Revives (optional).
	*/

	cs := p.channelState(c)
	if !cs.LastKnownIsGameRunning {
		if cs.LastKnownGame.ID == 0 {
			log.Printf("WARNING: Ignoring first incomplete round data: %+v", m)
			return nil
		}
		log.Printf("ERROR: Seeing game winner data for an unknown game, failing: %+v", m)
		return errors.New("found game winner data when no game considered running")
	}

	// TODO - mark user as winner of this round?
	// TODO - add up reward & xp?
	// TODO - add runners-up?
	// TODO - add most kills?
	// TODO - add most revives?

	// log.Printf("Game has a winner: %+v", p.LastKnownGame)

	cs.LastKnownGame.EndTime = &m.Timestamp
	if err := p.storeCurrentGame(c); err != nil {
		return err
	}

	cs.LastKnownIsGameRunning = false

	return nil
}

func (p *Processor) processGameCancelled(c discord.Channel, m discord.Message) error {
	ch := p.channelState(c)
	if !ch.LastKnownIsGameRunning {
		if ch.LastKnownGame.ID == 0 {
			log.Printf("WARNING: Ignoring first incomplete round data: %+v", m)
			return nil
		}
		log.Printf("ERROR: Seeing game cancellation for an unknown game, failing: %+v", m)
		return errors.New("found game cancellation when no game considered running")
	}

	ch.LastKnownGame.EndTime = &m.Timestamp
	ch.LastKnownGame.Cancelled = true
	if err := p.storeCurrentGame(c); err != nil {
		return err
	}

	ch.LastKnownIsGameRunning = false

	return nil
}

func (p *Processor) processGameSummary(c discord.Channel, e discord.Embed) error {
	cs := p.channelState(c)
	if !cs.LastKnownIsGameRunning {
		if cs.LastKnownGame.ID == 0 {
			log.Printf("WARNING: Ignoring first incomplete round data: %+v", e)
			return nil
		}
		log.Printf("ERROR: Seeing game summary data for an unknown game, failing: %+v", e)
		return errors.New("found game summary data when no game considered running")
	}

	// TODO
	return nil
}

var rxInteraction = regexp.MustCompile(`(?m)^(<:.+:\d+>)\s+\|\s+([^\n]+?)\s*$`)

func (p *Processor) processRound(c discord.Channel, m discord.Message, e discord.Embed) error {
	/*
		Embed title: "__Round 1__"

		Embed description:

		"<:egg_launcher:1217934777030545418> | **thiocyanate** found a war torn rabbit merchant who dealt in arms and ammunition. They picked the __Egg Launcher__.\n<:easter_cauldron:1217935344910205030> | **divinelegacy** looted a __Easter Cauldron__ off another player who failed to use its special revival properties.\n<:K:861698472154759199> | **tokki\\_egg** accidentally dropped a basket of lizard eggs and the hatchlings immediately butchered ~~**leerdix**~~ and brought back the body as tribute.\n<:ra:696057593835290665> | **technobean** spent the day in a strange warehouse after being promised an eggquisite adventure.\n<:K:861698472154759199> | **toniiz\\.** passed over ~~**junki**~~. Then came back and shot them.\n<:ra:696057593835290665> | **luigisensei** spent the day in a strange warehouse after being promised an eggquisite adventure.\n<:K:861698472154759199> | **technobean** neglected to tell ~~**chiasacomfy**~~ their candy was expired. \n\nPlayers Left: 27"

	*/

	cs := p.channelState(c)
	if !cs.LastKnownIsGameRunning {
		if cs.LastKnownGame.ID == 0 {
			log.Printf("WARNING: Ignoring first incomplete round data: %+v", e)
			return nil
		}
		log.Printf("ERROR: Seeing round data for an unknown game, failing: %+v", e)
		return errors.New("found round data when no game considered running")
	}

	// check if it is an event
	if strings.Contains(e.Title, " - ") {
		return p.processEventRound(c, m, e)
	}

	// log.Printf("Round (STD): %+v\n", p.LastKnownRound)
	cs.LastKnownRound.PostTime = m.Timestamp
	if err := p.storeCurrentRound(c); err != nil {
		return err
	}

	for _, line := range rxInteraction.FindAllStringSubmatch(e.Description, -1) {
		line, userInteractions, err := p.extractUsers(m, line[2])
		if err != nil {
			return err
		}
		line, items, err := p.extractItems(line)
		if err != nil {
			return err
		}
		interactionMessage, err := p.lookupInteractionMessage(line, "")
		if err != nil {
			return err
		}
		i := database.Interaction{
			Message:      interactionMessage,
			MessageID:    interactionMessage.ID,
			Round:        cs.LastKnownRound,
			RoundID:      cs.LastKnownRound.ID,
			UserMentions: userInteractions,
			Items:        items,
		}
		if err := p.storeInteraction(&i); err != nil {
			return err
		}
	}

	p.createNewRound(c, cs.LastKnownRound.RoundNumber+1)

	return nil
}

var (
	rxXPMultiplier = regexp.MustCompile(`([\d\.]+)x\s+XP\s+multiplier`)
	rxPrize        = regexp.MustCompile(`\*\*Prize:\*\*\s+(\d+)`)
)

func (p *Processor) processGameStart(c discord.Channel, m discord.Message, e discord.Embed) error {
	/*
		Embed title: "Started a new Rumble Royale session"

		Embed description:

		"**Number of participants:** 30\n**Era:** <:easter_leghorse:1094271879616921680>Easter\n**Prize:** 6000 <:gold:695955554199142421>\n**Gold Per Kill:** 60 <:gold:695955554199142421>\n\n\n<:xp:860094804984725504> **1.5x XP multiplier!**"
	*/

	cleanDesc := strings.Map(filterGraphic, e.Description)

	cs := p.channelState(c)
	cs.LastKnownIsGameRunning = true
	cs.LastKnownGame.StartTime = &m.Timestamp

	// extract rewarded coins
	if match := rxPrize.FindStringSubmatch(cleanDesc); match != nil {
		prize, err := strconv.ParseUint(match[1], 10, 64)
		if err != nil {
			return err
		}
		cs.LastKnownGame.RewardCoins = uint(prize)
	}

	// extract XP multiplier
	if match := rxXPMultiplier.FindStringSubmatch(cleanDesc); match != nil {
		multiplier64, err := strconv.ParseFloat(match[1], 32)
		if err != nil {
			return err
		}
		cs.LastKnownGame.XPMultiplier = float32(multiplier64)
	}

	if err := p.storeCurrentGame(c); err != nil {
		return err
	}

	// log.Printf("New game started: %+v\n", p.LastKnownGame)

	return nil
}

func (p *Processor) processEventRound(c discord.Channel, m discord.Message, e discord.Embed) error {
	/*
		Embed title: "__Round 1__ - STORM"

		Embed description:

		"A storm is gathering in the arena!\nPlayers are getting hit by lightning.\n\nThe following players died:\n<:S:861698472272986122> | ~~**toniiz\\.**~~\n<:S:861698472272986122> | ~~**chiasacomfy**~~\n<:S:861698472272986122> | ~~**pomegranede**~~\n<:S:861698472272986122> | ~~**technobean**~~\n<:S:861698472272986122> | ~~**f4b11**~~\n<:S:861698472272986122> | ~~**luigisensei**~~\n<:S:861698472272986122> | ~~**alexhero**~~\n<:S:861698472272986122> | ~~**leerdix**~~\n\nPlayers Left: 26"
	*/

	cs := p.channelState(c)
	if !cs.LastKnownIsGameRunning {
		if cs.LastKnownGame.ID == 0 {
			log.Printf("WARNING: Ignoring first incomplete round data: %+v", e)
			return nil
		}
		log.Printf("ERROR: Seeing event round data for an unknown game, failing: %+v", e)
		return errors.New("found event round data when no game considered running")
	}

	// log.Printf("Round (EVENT): %+v\n", p.LastKnownRound)
	cs.LastKnownRound.PostTime = m.Timestamp
	if err := p.storeCurrentRound(c); err != nil {
		return err
	}

	cleanDesc := strings.Map(filterGraphic, e.Description)
	cleanTitle := strings.Map(filterGraphic, e.Title)

	eventTitle := strings.SplitN(cleanTitle, " - ", 2)[1]
	eventDescription := strings.SplitN(cleanDesc, "\n\n", 2)[0]
	cleanDesc, userInteractions, err := p.extractUsers(m, cleanDesc)
	if err != nil {
		return err
	}
	_, items, err := p.extractItems(cleanDesc)
	if err != nil {
		return err
	}
	interactionMessage, err := p.lookupInteractionMessage(eventDescription, eventTitle)
	if err != nil {
		return err
	}
	i := database.Interaction{
		Message:      interactionMessage,
		MessageID:    interactionMessage.ID,
		Round:        cs.LastKnownRound,
		RoundID:      cs.LastKnownRound.ID,
		UserMentions: userInteractions,
		Items:        items,
	}
	if err := p.storeInteraction(&i); err != nil {
		return err
	}

	p.createNewRound(c, cs.LastKnownRound.RoundNumber+1)

	return nil
}

// func (p *Processor) processEventStorm(e discord.Embed) error {
// 	/*
// 		Embed title: "__Round 1__ - STORM"

// 		Embed description:

// 		"A storm is gathering in the arena!\nPlayers are getting hit by lightning.\n\nThe following players died:\n<:S:861698472272986122> | ~~**toniiz\\.**~~\n<:S:861698472272986122> | ~~**chiasacomfy**~~\n<:S:861698472272986122> | ~~**pomegranede**~~\n<:S:861698472272986122> | ~~**technobean**~~\n<:S:861698472272986122> | ~~**f4b11**~~\n<:S:861698472272986122> | ~~**luigisensei**~~\n<:S:861698472272986122> | ~~**alexhero**~~\n<:S:861698472272986122> | ~~**leerdix**~~\n\nPlayers Left: 26"
// 	*/
// 	return nil
// }

// func (p *Processor) processEventUFO(e discord.Embed) error {
// 	/*
// 		Embed title: "__Round 4__ - ALIEN ABDUCTION"

// 		Embed description:

// 		"Is that a UFO? Aliens are scooping up players like free samples!\n\nThe following players died:\n<:S:861698472272986122> | ~~**tacticaldragon57**~~\n<:S:861698472272986122> | ~~**rusthie**~~\n<:S:861698472272986122> | ~~**edsky the Mummy**~~\n<:S:861698472272986122> | ~~**tetradx**~~\n<:S:861698472272986122> | ~~**luigisensei**~~\n<:S:861698472272986122> | ~~**jt0219 the Mummy**~~\n<:S:861698472272986122> | ~~**madslingshoter**~~\n<:S:861698472272986122> | ~~**cydral**~~\n<:S:861698472272986122> | ~~**fries2443**~~\n\nPlayers Left: 25"
// 	*/
// 	return nil
// }

// func (p *Processor) processEventResurrection(e discord.Embed) error {
// 	/*
// 		Embed title: "__Round 5__ - RESURRECTION"

// 		Embed description:

// 		"Looks like some zombies have retained their intelligence.\nThey are back in the fight!\n\nThe following players were revived:\n<:re:695955553259880470> | **super\\_mctea**\n<:re:695955553259880470> | **graxsnag the Holy**\n<:re:695955553259880470> | **lavapurg the Mummy**\n<:re:695955553259880470> | **\\_kazma**\n<:re:695955553259880470> | **daftsuki the One**\n\nPlayers Left: 24"
// 	*/
// 	return nil
// }
