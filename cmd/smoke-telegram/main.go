package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"announcementsbot/internal/config"
	"announcementsbot/internal/telegram"
)

func main() {
	config.LoadDotEnv(os.Getenv("ENV_FILE"))
	cfg, err := config.Load(os.Getenv("CONFIG_PATH"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "smoke failed: %v\n", err)
		os.Exit(1)
	}

	text := strings.TrimSpace(strings.Join(os.Args[1:], " "))
	if text == "" {
		text = "Telegram smoke test from StakeCraft relay bot\nTime: " + time.Now().UTC().Format(time.RFC3339)
	}

	tg := telegram.New(os.Getenv("TELEGRAM_BOT_TOKEN"), cfg.Runtime.TelegramTimeoutMS, cfg.Telegram.MessageFormat)
	res, err := tg.PublishText(context.Background(), cfg.Telegram.MainChatID, "#smoke_test\n"+text)
	if err != nil {
		fmt.Fprintf(os.Stderr, "smoke failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("ok chat=%s message_id=%s\n", res.ChatID, res.MessageID)
}
