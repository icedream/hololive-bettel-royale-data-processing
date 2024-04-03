package database

import "time"

type User struct {
	Name string `gorm:"primaryKey"`
}

type InteractionUserMention struct {
	ID     int `gorm:"primaryKey"`
	UserID string
	User   User
	Killed bool
	Suffix string
}

type Item struct {
	Name string `gorm:"primaryKey"`
}

type Interaction struct {
	ID           int `gorm:"primaryKey"`
	Round        Round
	RoundID      int
	Message      InteractionMessage
	MessageID    int
	UserMentions []InteractionUserMention `gorm:"many2many:interaction_user_mention_mappings;"`
	Items        []Item                   `gorm:"many2many:interaction_item_mappings;"`
}

type Round struct {
	ID          int `gorm:"primaryKey"`
	GameID      int `gorm:"index:game_round_idx,unique"`
	Game        Game
	RoundNumber int `gorm:"index:game_round_idx,unique"`
	PostTime    time.Time
}

type Game struct {
	ID                 int `gorm:"primaryKey"`
	Era                string
	HostUserName       *string
	HostUser           *User
	StartTime          *time.Time
	EndTime            *time.Time
	CountdownStartTime time.Time
	XPMultiplier       float32
	RewardCoins        uint
}

type InteractionMessage struct {
	ID    int    `gorm:"primaryKey"`
	Text  string `gorm:"index:interaction_message_text_idx,unique"`
	Event string
}
