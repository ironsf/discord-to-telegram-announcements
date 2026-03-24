package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"announcementsbot/internal/model"
)

type rawConfig struct {
	Discord struct {
		GuildID         string       `json:"guildId"`
		AllowedChannels []rawChannel `json:"allowedChannels"`
	} `json:"discord"`
	Telegram struct {
		MainChatID    string `json:"mainChatId"`
		OpsChatID     string `json:"opsChatId"`
		MessageFormat string `json:"messageFormat"`
	} `json:"telegram"`
	Runtime struct {
		LogLevel            string `json:"logLevel"`
		TelegramTimeoutMS   int    `json:"telegramTimeoutMs"`
		AlertCooldownSecond int    `json:"alertCooldownSeconds"`
	} `json:"runtime"`
	Storage struct {
		SQLitePath                string `json:"sqlitePath"`
		MaxForwardedAnnouncements int    `json:"maxForwardedAnnouncements"`
		PruneOnStartup            *bool  `json:"pruneOnStartup"`
	} `json:"storage"`
}

type rawChannel struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Enabled  *bool    `json:"enabled"`
	Keywords []string `json:"keywords"`
}

func LoadDotEnv(path string) {
	if path == "" {
		path = ".env"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.Index(line, "=")
		if i <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:i])
		value := strings.Trim(strings.TrimSpace(line[i+1:]), "\"'")
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			_ = os.Setenv(key, value)
		}
	}
}

func Load(configPath string) (*model.AppConfig, error) {
	if configPath == "" {
		configPath = os.Getenv("CONFIG_PATH")
	}
	if configPath == "" {
		configPath = "config/config.json"
	}

	fullPath, err := filepath.Abs(configPath)
	if err != nil {
		return nil, err
	}
	rawData, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, err
	}

	var raw rawConfig
	if err := json.Unmarshal(rawData, &raw); err != nil {
		return nil, err
	}

	cfg := &model.AppConfig{}
	cfg.Discord.GuildID = raw.Discord.GuildID
	cfg.Discord.AllowedChannels = make([]model.ChannelConfig, 0, len(raw.Discord.AllowedChannels))
	for _, ch := range raw.Discord.AllowedChannels {
		enabled := true
		if ch.Enabled != nil {
			enabled = *ch.Enabled
		}
		cfg.Discord.AllowedChannels = append(cfg.Discord.AllowedChannels, model.ChannelConfig{
			ID:       ch.ID,
			Name:     ch.Name,
			Enabled:  enabled,
			Keywords: ch.Keywords,
		})
	}
	cfg.Telegram.MainChatID = raw.Telegram.MainChatID
	cfg.Telegram.OpsChatID = raw.Telegram.OpsChatID
	cfg.Telegram.MessageFormat = strings.TrimSpace(raw.Telegram.MessageFormat)
	cfg.Runtime.LogLevel = raw.Runtime.LogLevel
	cfg.Runtime.TelegramTimeoutMS = raw.Runtime.TelegramTimeoutMS
	cfg.Runtime.AlertCooldownSecond = raw.Runtime.AlertCooldownSecond
	cfg.Storage.SQLitePath = raw.Storage.SQLitePath
	cfg.Storage.MaxForwardedAnnouncements = raw.Storage.MaxForwardedAnnouncements
	if raw.Storage.PruneOnStartup == nil {
		cfg.Storage.PruneOnStartup = true
	} else {
		cfg.Storage.PruneOnStartup = *raw.Storage.PruneOnStartup
	}

	applyDefaults(cfg)
	if err := validate(cfg); err != nil {
		return nil, err
	}
	cfg.Storage.SQLitePath, err = filepath.Abs(cfg.Storage.SQLitePath)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

func applyDefaults(cfg *model.AppConfig) {
	if cfg.Runtime.LogLevel == "" {
		cfg.Runtime.LogLevel = "info"
	}
	if cfg.Runtime.TelegramTimeoutMS <= 0 {
		cfg.Runtime.TelegramTimeoutMS = 15000
	}
	if cfg.Runtime.AlertCooldownSecond <= 0 {
		cfg.Runtime.AlertCooldownSecond = 300
	}
	if cfg.Telegram.MessageFormat == "" {
		cfg.Telegram.MessageFormat = "card"
	}
	if cfg.Storage.SQLitePath == "" {
		cfg.Storage.SQLitePath = "data/app.sqlite"
	}
	if cfg.Storage.MaxForwardedAnnouncements <= 0 {
		cfg.Storage.MaxForwardedAnnouncements = 500
	}
}

func validate(cfg *model.AppConfig) error {
	if strings.TrimSpace(os.Getenv("DISCORD_BOT_TOKEN")) == "" {
		return errors.New("invalid config: DISCORD_BOT_TOKEN env must be a non-empty string")
	}
	if strings.TrimSpace(os.Getenv("TELEGRAM_BOT_TOKEN")) == "" {
		return errors.New("invalid config: TELEGRAM_BOT_TOKEN env must be a non-empty string")
	}
	if strings.TrimSpace(cfg.Discord.GuildID) == "" {
		return errors.New("invalid config: discord.guildId must be a non-empty string")
	}
	if len(cfg.Discord.AllowedChannels) == 0 {
		return errors.New("invalid config: discord.allowedChannels must contain at least one channel")
	}
	for i, ch := range cfg.Discord.AllowedChannels {
		if strings.TrimSpace(ch.ID) == "" {
			return fmt.Errorf("invalid config: discord.allowedChannels[%d].id must be a non-empty string", i)
		}
		if strings.TrimSpace(ch.Name) == "" {
			return fmt.Errorf("invalid config: discord.allowedChannels[%d].name must be a non-empty string", i)
		}
	}
	if strings.TrimSpace(cfg.Telegram.MainChatID) == "" {
		return errors.New("invalid config: telegram.mainChatId must be a non-empty string")
	}
	if strings.TrimSpace(cfg.Telegram.OpsChatID) == "" {
		return errors.New("invalid config: telegram.opsChatId must be a non-empty string")
	}
	if cfg.Telegram.MessageFormat != "card" && cfg.Telegram.MessageFormat != "minimal" {
		return errors.New("invalid config: telegram.messageFormat must be one of: card, minimal")
	}
	if cfg.Storage.MaxForwardedAnnouncements < 1 {
		return errors.New("invalid config: storage.maxForwardedAnnouncements must be >= 1")
	}
	return nil
}
