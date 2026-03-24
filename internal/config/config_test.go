package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigDefaultsMessageFormatToCard(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "discord-token")
	t.Setenv("TELEGRAM_BOT_TOKEN", "telegram-token")

	configPath := writeConfigFixture(t, `{
  "discord": {
    "guildId": "guild",
    "allowedChannels": [
      {
        "id": "1",
        "name": "solana-mb",
        "enabled": true,
        "keywords": ["release"]
      }
    ]
  },
  "telegram": {
    "mainChatId": "main",
    "opsChatId": "ops"
  },
  "runtime": {},
  "storage": {
    "sqlitePath": "data/app.sqlite",
    "maxForwardedAnnouncements": 5
  }
}`)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Telegram.MessageFormat != "card" {
		t.Fatalf("expected default card format, got %q", cfg.Telegram.MessageFormat)
	}
}

func TestConfigAcceptsKnownMessageFormats(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "discord-token")
	t.Setenv("TELEGRAM_BOT_TOKEN", "telegram-token")

	for _, format := range []string{"card", "minimal"} {
		configPath := writeConfigFixture(t, `{
  "discord": {
    "guildId": "guild",
    "allowedChannels": [
      {
        "id": "1",
        "name": "solana-mb",
        "enabled": true,
        "keywords": ["release"]
      }
    ]
  },
  "telegram": {
    "mainChatId": "main",
    "opsChatId": "ops",
    "messageFormat": "`+format+`"
  },
  "runtime": {},
  "storage": {
    "sqlitePath": "data/app.sqlite",
    "maxForwardedAnnouncements": 5
  }
}`)

		cfg, err := Load(configPath)
		if err != nil {
			t.Fatalf("load config with format %q: %v", format, err)
		}
		if cfg.Telegram.MessageFormat != format {
			t.Fatalf("expected format %q, got %q", format, cfg.Telegram.MessageFormat)
		}
	}
}

func TestConfigLoadsChannelThemeAndTitleBold(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "discord-token")
	t.Setenv("TELEGRAM_BOT_TOKEN", "telegram-token")

	configPath := writeConfigFixture(t, `{
  "discord": {
    "guildId": "guild",
    "allowedChannels": [
      {
        "id": "1",
        "name": "solana-tds",
        "enabled": true,
        "keywords": ["release"],
        "theme": "green",
        "titleBold": true
      }
    ]
  },
  "telegram": {
    "mainChatId": "main",
    "opsChatId": "ops",
    "messageFormat": "card"
  },
  "runtime": {},
  "storage": {
    "sqlitePath": "data/app.sqlite",
    "maxForwardedAnnouncements": 5
  }
}`)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	ch := cfg.Discord.AllowedChannels[0]
	if ch.Theme != "green" {
		t.Fatalf("expected theme green, got %q", ch.Theme)
	}
	if !ch.TitleBold {
		t.Fatal("expected titleBold=true")
	}
}

func TestConfigRejectsUnknownMessageFormat(t *testing.T) {
	t.Setenv("DISCORD_BOT_TOKEN", "discord-token")
	t.Setenv("TELEGRAM_BOT_TOKEN", "telegram-token")

	configPath := writeConfigFixture(t, `{
  "discord": {
    "guildId": "guild",
    "allowedChannels": [
      {
        "id": "1",
        "name": "solana-mb",
        "enabled": true,
        "keywords": ["release"]
      }
    ]
  },
  "telegram": {
    "mainChatId": "main",
    "opsChatId": "ops",
    "messageFormat": "loud"
  },
  "runtime": {},
  "storage": {
    "sqlitePath": "data/app.sqlite",
    "maxForwardedAnnouncements": 5
  }
}`)

	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected validation error for unknown message format")
	}
	if !strings.Contains(err.Error(), "telegram.messageFormat") {
		t.Fatalf("expected telegram.messageFormat validation error, got: %v", err)
	}
}

func writeConfigFixture(t *testing.T, data string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("write config fixture: %v", err)
	}
	return path
}
