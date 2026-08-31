package config_test

import (
	"testing"

	"gshlan/gshbot/config"
)

func TestNew(t *testing.T) {
	discord := config.Discord{
		Name:         "gshbot",
		GuildID:      "guild-1",
		Token:        "token-1",
		Prefix:       "!",
		DBName:       "db",
		DBColName:    "col",
		MoritzUserId: "user-1",
	}

	cfg := config.New(discord)

	if cfg.Discord != discord {
		t.Fatalf("expected Config.Discord to equal %+v, got %+v", discord, cfg.Discord)
	}
}
