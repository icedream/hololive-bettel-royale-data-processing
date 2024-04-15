DOTNET = dotnet
GIT = git
GO = go
GREP = grep

DCE_SRC_PATH = submodules/DiscordChatExporter
DCE_BUILT_BIN_PATH := $(DCE_SRC_PATH)/DiscordChatExporter.Cli/bin/Release/net8.0/DiscordChatExporter.Cli.dll
DCE_BIN_PATH = $(DCE_BUILT_BIN_PATH)
DCE_DEPS := $(DCE_BIN_PATH)
DCE_BUILD_CONFIGURATION = Release
DCE_CMD = $(DOTNET) $(DCE_BIN_PATH)

DISCORD_TOKEN = 
BATTLE_ROYALE_CHANNEL_ID = 1224009923457847428
BATTLE_ROYALE_SHOPPING_CHANNEL_ID = 1224017701744410695

EXPORTS_PATH = discord-exports
DATABASE_PATH = main.db
SQL_DUMPS_PATH = sql

BINS := process-discord-exports

.PHONY: all
all: $(BINS) dumps

.PHONY: bins
bins: $(BINS)

.PHONY: build-dce
build-dce: submodules/DiscordChatExporter/DiscordChatExporter.Cli/bin/Release/net8.0/DiscordChatExporter.Cli.dll

.PHONY: discord-token
discord-token: $(DCE_DEPS)
	@[ -n $(DISCORD_TOKEN) ] || (echo "ERROR: DISCORD_TOKEN needs to be set. Here's a guide how to get this information:"; $(DCE_CMD) guide; exit 1)

###############################
# GIT SUBMODULES AUTO CHECKOUT

submodules/DiscordChatExporter: .git
	@[ -d $(CURDIR)/.git ] || (echo "ERROR: Need to check out with Git instead of downloading as source tarball for this to work." >&2; exit 1)
	$(GIT) submodule update --init $@

##############################
# DISCORD CHAT EXPORTER BUILD

$(DCE_SRC_PATH)/DiscordChatExporter.Cli/bin/Release/net8.0/DiscordChatExporter.Cli.dll: $(DCE_SRC_PATH) $(DCE_SRC_PATH)/DiscordChatExporter.Cli/DiscordChatExporter.Cli.csproj
	$(DOTNET) build --configuration $(DCE_BUILD_CONFIGURATION) $(DCE_SRC_PATH)/DiscordChatExporter.Cli/DiscordChatExporter.Cli.csproj

######################
# DISCORD CHAT EXPORT

.PHONY: clean-discord-exports
clean-discord-exports:
	$(RM) -r discord-exports

dce-help: $(DCE_DEPS)
	$(DCE_CMD) --help

discord-export-channel: $(DCE_DEPS) discord-token
	$(DCE_CMD) export -t $(DISCORD_TOKEN) -f json --markdown false --utc -p 100 \
		-o $(EXPORTS_PATH)/$(DISCORD_CHANNEL_ID)/$(shell date +%s)/ \
		-c $(DISCORD_CHANNEL_ID) \
		--after $(shell $(MAKE) -s discord-export-last-message-id DISCORD_CHANNEL_ID=$(DISCORD_CHANNEL_ID))

discord-export: $(DCE_DEPS) discord-token
	-$(MAKE) discord-export-channel DISCORD_CHANNEL_ID=$(BATTLE_ROYALE_CHANNEL_ID)
	-$(MAKE) discord-export-channel DISCORD_CHANNEL_ID=$(BATTLE_ROYALE_SHOPPING_CHANNEL_ID)

# Returns last message ID in local archives, if no archive exists will output 0
# instead of an ID.
discord-export-last-message-id:
	([ ! -d $(EXPORTS_PATH)/$(DISCORD_CHANNEL_ID) ] || find $(EXPORTS_PATH)/$(DISCORD_CHANNEL_ID) -name '*.json' -not -path '*/manual_fixup/*' -exec cat {} \;) |\
	(jq -r '.messages .[] .id' && echo 0) |\
	sort -n |\
	uniq |\
	tail -n1

#############
# PROCESSING

.PHONY: clean-db
clean-db:
	$(RM) $(DATABASE_PATH)

.PHONY: database
database: $(DATABASE_PATH)

.PHONY: clean-dumps
clean-dumps:
	$(RM) $(SQL_DUMPS_PATH)/all.sql

.PHONY: dumps
dumps: $(SQL_DUMPS_PATH)/all.sql

process-discord-exports:
	$(GO) build -v -o $@ ./cmd/process-discord-exports

# go code deps
process-discord-exports: $(wildcard ./cmd/process-discord-exports/*.go)
process-discord-exports: $(wildcard ./internal/*/*.go)

$(DATABASE_PATH): process-discord-exports
	@[ -d $(EXPORTS_PATH)/$(BATTLE_ROYALE_CHANNEL_ID) ] || (echo "ERROR: No discord export of battle royale channel exists yet, run \`make discord-export\` to create one."; exit 1)
	./process-discord-exports --exports-path=$(EXPORTS_PATH) --database-path=$(DATABASE_PATH) import

# TODO - Do not use this yet as it does not reset sqlite sequence number tables!
.PHONY: reset-db
reset-db:
	./process-discord-exports --exports-path=$(EXPORTS_PATH) --database-path=$(DATABASE_PATH) reset

$(SQL_DUMPS_PATH)/all.sql: $(DATABASE_PATH) process-discord-exports
	./process-discord-exports --exports-path=$(EXPORTS_PATH) --database-path=$(DATABASE_PATH) dump >$@

.PHONY: check-sql-dump-changed
check-sql-dump-changed:
	@! $(GIT) diff --quiet --exit-code $(SQL_DUMPS_PATH)/all.sql || (echo "SQL dump did not change"; exit 1)

.PHONY: check-sql-game-time-changed
check-sql-game-time-changed:
	@$(GIT) diff $(SQL_DUMPS_PATH)/all.sql |\
	$(GREP) -Eq '^\+-- Latest game time considered in this dump:' ||\
	(echo "SQL game time did not change"; exit 1)

.PHONY: sql-game-time
sql-game-time:
	@grep -Po '^-- Latest game time considered in this dump: \K.+' $(SQL_DUMPS_PATH)/all.sql

.PHONY: commit-sql-dump
commit-sql-dump: check-sql-dump-changed check-sql-game-time-changed
	$(GIT) commit -m "Update SQL dump up to $(shell $(MAKE) -s sql-game-time)." -- $(SQL_DUMPS_PATH)/all.sql

.PHONY: refresh-dumps
refresh-dumps: discord-export clean-db clean-dumps dumps

.PHONY: refresh-push-dumps
refresh-push-dumps: refresh-dumps commit-sql-dump
	$(GIT) push

#######
# SQLC

sqlc-ts: go.mod go.sum
	$(GO) build -v -mod=mod github.com/stephen/sqlc-ts

.PHONY: sqlc
sqlc: sqlc.yml $(wildcard sql/queries/*.sql) sqlc-ts
	$(GO) run -v -mod=mod github.com/sqlc-dev/sqlc/cmd/sqlc generate


