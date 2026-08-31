package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"gshlan/gshbot/config"
)

func TestLoad_Success(t *testing.T) {
	yamlContent := `
discord:
  name: gshbot
  guild_id: "guild-1"
  token: "token-1"
  prefix: "!"
  dbname: "db"
  dbcolname: "col"
  moritz_user_id: "user-1"
`
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(yamlContent), 0o644); err != nil {
		t.Fatalf("failed to write fixture config: %v", err)
	}

	cfg := config.Load(path)

	want := config.Discord{
		Name:         "gshbot",
		GuildID:      "guild-1",
		Token:        "token-1",
		Prefix:       "!",
		DBName:       "db",
		DBColName:    "col",
		MoritzUserId: "user-1",
	}
	if cfg.Discord != want {
		t.Fatalf("expected Discord config %+v, got %+v", want, cfg.Discord)
	}
}

func TestLoad_PanicsOnMissingFile(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected Load to panic for a missing config file")
		}
	}()

	config.Load(filepath.Join(t.TempDir(), "does-not-exist.yml"))
}
