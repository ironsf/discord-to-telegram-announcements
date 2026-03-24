package processor

import (
	"context"

	"announcementsbot/internal/logger"
	"announcementsbot/internal/model"
	"announcementsbot/internal/store"
	"announcementsbot/internal/telegram"
)

type Processor struct {
	cfg        *model.AppConfig
	store      *store.Store
	telegram   *telegram.Client
	logger     *logger.Logger
	channelMap map[string]model.ChannelConfig
}

func New(cfg *model.AppConfig, st *store.Store, tg *telegram.Client, lg *logger.Logger) *Processor {
	chMap := make(map[string]model.ChannelConfig)
	for _, c := range cfg.Discord.AllowedChannels {
		if c.Enabled {
			chMap[c.ID] = c
		}
	}
	return &Processor{cfg: cfg, store: st, telegram: tg, logger: lg, channelMap: chMap}
}

func (p *Processor) HandleEvent(ctx context.Context, event *model.AnnouncementEvent) error {
	channel, ok := p.channelMap[event.ChannelID]
	if !ok {
		return nil
	}

	decision := evaluateKeywords(event, channel)
	state, err := p.store.UpsertSeenMessage(event.MessageID, event.ChannelID, contentHash(event), decision.Matched)
	if err != nil {
		return err
	}

	if event.EventType == model.EventEdited {
		return p.handleEdit(ctx, event, channel, decision.Matched, state.Forwarded)
	}
	if !decision.Matched {
		p.logger.Debug("Message skipped: no keyword match", map[string]any{"channelId": event.ChannelID, "messageId": event.MessageID})
		return nil
	}
	if state.Forwarded {
		p.logger.Debug("Message already forwarded", map[string]any{"messageId": event.MessageID})
		return nil
	}

	res, err := p.telegram.PublishAnnouncement(ctx, p.cfg.Telegram.MainChatID, model.ForwardRequest{
		SourcePrefix:   "[" + channel.Name + "]",
		Theme:          channel.Theme,
		TitleBold:      channel.TitleBold,
		Text:           firstNonEmpty(event.ContentText, event.EmbedText),
		Permalink:      event.Permalink,
		AttachmentURLs: event.AttachmentURLs,
		TimestampISO:   event.TimestampISO,
		AuthorName:     event.AuthorName,
	})
	if err != nil {
		return err
	}

	err = p.store.MarkForwarded(store.PersistForwardInput{
		MessageID:       event.MessageID,
		ChannelID:       event.ChannelID,
		ChannelName:     channel.Name,
		MatchedKeywords: decision.MatchedKeywords,
		TelegramChatID:  res.ChatID,
		TelegramMsgID:   res.MessageID,
	})
	if err != nil {
		return err
	}

	p.logger.Info("Forwarded announcement", map[string]any{
		"channelId":         event.ChannelID,
		"channelName":       channel.Name,
		"messageId":         event.MessageID,
		"telegramMessageId": res.MessageID,
	})

	if _, err := p.store.PruneForwardedAnnouncementsToMax(p.cfg.Storage.MaxForwardedAnnouncements); err != nil {
		_ = p.SendOpsAlertOnce(ctx, "retention-prune-failed", "Retention prune failed: "+err.Error())
		p.logger.Error("Retention prune failed", map[string]any{"error": err.Error()})
	}

	return nil
}

func (p *Processor) handleEdit(ctx context.Context, event *model.AnnouncementEvent, channel model.ChannelConfig, matchedNow, wasForwarded bool) error {
	if !wasForwarded || !matchedNow {
		return nil
	}
	eHash := contentHash(event)
	exists, err := p.store.HasEditHash(event.MessageID, eHash)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	res, err := p.telegram.PublishText(ctx, p.cfg.Telegram.MainChatID, telegram.FormatEditNotice(model.EditNoticeRequest{
		SourcePrefix: "[" + channel.Name + "]",
		Theme:        channel.Theme,
		TitleBold:    channel.TitleBold,
		ChannelName:  channel.Name,
		Permalink:    event.Permalink,
		Text:         firstNonEmpty(event.ContentText, event.EmbedText),
		TimestampISO: event.TimestampISO,
	}))
	if err != nil {
		return err
	}
	if err := p.store.RecordEditNotice(event.MessageID, eHash, res.MessageID); err != nil {
		return err
	}
	p.logger.Info("Posted edit notice", map[string]any{"messageId": event.MessageID, "telegramMessageId": res.MessageID})
	return nil
}

func (p *Processor) SendOpsAlertOnce(ctx context.Context, key, text string) error {
	allowed, err := p.store.ShouldSendAlert(key, p.cfg.Runtime.AlertCooldownSecond)
	if err != nil {
		return err
	}
	if !allowed {
		return nil
	}
	_, err = p.telegram.PublishOpsAlert(ctx, p.cfg.Telegram.OpsChatID, text)
	if err != nil {
		p.logger.Error("Failed to send ops alert", map[string]any{"key": key, "error": err.Error()})
		return err
	}
	return p.store.MarkAlertSent(key)
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
