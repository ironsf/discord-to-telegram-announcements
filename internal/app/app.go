package app

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"announcementsbot/internal/config"
	"announcementsbot/internal/discord"
	"announcementsbot/internal/logger"
	"announcementsbot/internal/model"
	"announcementsbot/internal/processor"
	"announcementsbot/internal/store"
	"announcementsbot/internal/telegram"
)

func Run(ctx context.Context) error {
	config.LoadDotEnv(os.Getenv("ENV_FILE"))
	cfg, err := config.Load(os.Getenv("CONFIG_PATH"))
	if err != nil {
		return err
	}

	lg := logger.New(cfg.Runtime.LogLevel)
	st, err := store.Open(cfg.Storage.SQLitePath)
	if err != nil {
		return err
	}
	defer st.Close()

	if cfg.Storage.PruneOnStartup {
		deleted, err := st.PruneForwardedAnnouncementsToMax(cfg.Storage.MaxForwardedAnnouncements)
		if err != nil {
			lg.Error("Startup prune failed", map[string]any{"error": err.Error()})
		} else if deleted > 0 {
			lg.Info("Startup prune completed", map[string]any{"deletedCount": deleted, "limit": cfg.Storage.MaxForwardedAnnouncements})
		}
	}

	tg := telegram.New(os.Getenv("TELEGRAM_BOT_TOKEN"), cfg.Runtime.TelegramTimeoutMS, cfg.Telegram.MessageFormat)
	proc := processor.New(cfg, st, tg, lg)

	allowedIDs := make([]string, 0)
	for _, ch := range cfg.Discord.AllowedChannels {
		if ch.Enabled {
			allowedIDs = append(allowedIDs, ch.ID)
		}
	}
	lg.Info("Starting StakeCraft updates relay bot", map[string]any{
		"guildId":           cfg.Discord.GuildID,
		"allowedChannels":   len(allowedIDs),
		"allowedChannelIds": allowedIDs,
		"retentionMax":      cfg.Storage.MaxForwardedAnnouncements,
	})

	dc := discord.New(os.Getenv("DISCORD_BOT_TOKEN"), cfg, lg,
		func(callCtx context.Context, event *model.AnnouncementEvent) error {
			return proc.HandleEvent(callCtx, event)
		},
		func(callCtx context.Context, key, message string) error {
			return proc.SendOpsAlertOnce(callCtx, key, message)
		},
	)

	if err := dc.Start(ctx); err != nil {
		return err
	}
	defer dc.Stop()

	sigCtx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-sigCtx.Done()

	lg.Info("Shutdown signal received", map[string]any{"reason": "signal"})
	return nil
}
