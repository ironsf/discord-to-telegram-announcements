package discord

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	"announcementsbot/internal/logger"
	"announcementsbot/internal/model"
)

type EventHandler func(ctx context.Context, event *model.AnnouncementEvent) error

type OpsHandler func(ctx context.Context, key, message string) error

type Adapter struct {
	token      string
	guildID    string
	allowed    map[string]model.ChannelConfig
	logger     *logger.Logger
	onEvent    EventHandler
	onOpsIssue OpsHandler
	session    *discordgo.Session
	mu         sync.Mutex
	started    bool
}

func New(token string, cfg *model.AppConfig, lg *logger.Logger, onEvent EventHandler, onOpsIssue OpsHandler) *Adapter {
	allowed := make(map[string]model.ChannelConfig)
	for _, ch := range cfg.Discord.AllowedChannels {
		if ch.Enabled {
			allowed[ch.ID] = ch
		}
	}
	return &Adapter{token: token, guildID: cfg.Discord.GuildID, allowed: allowed, logger: lg, onEvent: onEvent, onOpsIssue: onOpsIssue}
}

func (a *Adapter) Start(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.started {
		return nil
	}

	dg, err := discordgo.New("Bot " + a.token)
	if err != nil {
		return err
	}
	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages | discordgo.IntentMessageContent
	dg.StateEnabled = true

	dg.AddHandler(func(s *discordgo.Session, r *discordgo.Ready) {
		a.logger.Info("Discord client ready", map[string]any{"user": r.User.Username + "#" + r.User.Discriminator})
		if err := a.validateChannels(context.Background(), s); err != nil {
			a.logger.Error("Channel validation failed", map[string]any{"error": err.Error()})
		}
	})

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageCreate) {
		a.processMessageCreate(ctx, s, m)
	})

	dg.AddHandler(func(s *discordgo.Session, m *discordgo.MessageUpdate) {
		a.processMessageUpdate(ctx, s, m)
	})

	if err := dg.Open(); err != nil {
		return err
	}

	a.session = dg
	a.started = true
	return nil
}

func (a *Adapter) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.started || a.session == nil {
		return nil
	}
	a.started = false
	return a.session.Close()
}

func (a *Adapter) validateChannels(ctx context.Context, s *discordgo.Session) error {
	guild, err := s.State.Guild(a.guildID)
	if err != nil || guild == nil {
		if _, fetchErr := s.Guild(a.guildID); fetchErr != nil {
			msg := fmt.Sprintf("Configured guild not found or inaccessible: %s", a.guildID)
			_ = a.onOpsIssue(ctx, "discord-guild-missing", msg)
			return fmt.Errorf(msg)
		}
	}

	problems := make([]string, 0)
	for _, cfg := range a.allowed {
		ch, err := s.Channel(cfg.ID)
		if err != nil || ch == nil {
			problems = append(problems, fmt.Sprintf("%s (%s): not found", cfg.Name, cfg.ID))
			continue
		}
		if ch.Type != discordgo.ChannelTypeGuildText && ch.Type != discordgo.ChannelTypeGuildNews {
			problems = append(problems, fmt.Sprintf("%s (%s): unsupported type %d", cfg.Name, cfg.ID, ch.Type))
		}
	}

	if len(problems) > 0 {
		_ = a.onOpsIssue(ctx, "discord-channel-validation", "Channel validation issues:\n- "+strings.Join(problems, "\n- "))
		a.logger.Warn("Configured channel validation issues", map[string]any{"count": len(problems)})
		return nil
	}
	a.logger.Info("Configured channels validated", map[string]any{"count": len(a.allowed)})
	return nil
}

func (a *Adapter) processMessageCreate(ctx context.Context, s *discordgo.Session, m *discordgo.MessageCreate) {
	msg := m.Message
	if msg == nil {
		return
	}
	event, ok := a.normalizeMessage(msg, model.EventCreated)
	if !ok {
		return
	}
	a.logger.Debug("Accepted Discord message event", map[string]any{"eventType": string(event.EventType), "messageId": event.MessageID, "channelId": event.ChannelID, "channelName": event.ChannelName, "author": event.AuthorName, "hasContent": event.ContentText != "", "hasEmbeds": event.EmbedText != ""})
	if err := a.onEvent(ctx, event); err != nil {
		a.logger.Error("Failed to process announcement event", map[string]any{"error": err.Error(), "channelId": event.ChannelID, "messageId": event.MessageID, "eventType": event.EventType})
		_ = a.onOpsIssue(context.Background(), "event-processing-failure", fmt.Sprintf("Failed to process Discord message %s: %s", event.MessageID, err.Error()))
	}
}

func (a *Adapter) processMessageUpdate(ctx context.Context, s *discordgo.Session, m *discordgo.MessageUpdate) {
	message := m.Message
	if message == nil {
		return
	}
	if message.Content == "" && len(message.Embeds) == 0 {
		full, err := s.ChannelMessage(message.ChannelID, message.ID)
		if err == nil && full != nil {
			message = full
		}
	}
	event, ok := a.normalizeMessage(message, model.EventEdited)
	if !ok {
		return
	}
	a.logger.Debug("Accepted Discord message event", map[string]any{"eventType": string(event.EventType), "messageId": event.MessageID, "channelId": event.ChannelID, "channelName": event.ChannelName, "author": event.AuthorName, "hasContent": event.ContentText != "", "hasEmbeds": event.EmbedText != ""})
	if err := a.onEvent(ctx, event); err != nil {
		a.logger.Error("Failed to process announcement event", map[string]any{"error": err.Error(), "channelId": event.ChannelID, "messageId": event.MessageID, "eventType": event.EventType})
		_ = a.onOpsIssue(context.Background(), "event-processing-failure", fmt.Sprintf("Failed to process Discord message %s: %s", event.MessageID, err.Error()))
	}
}

func (a *Adapter) normalizeMessage(msg *discordgo.Message, eventType model.AnnouncementEventType) (*model.AnnouncementEvent, bool) {
	if msg == nil || msg.Author == nil {
		a.logger.Debug("Skipping message without author", map[string]any{"eventType": string(eventType)})
		return nil, false
	}
	if msg.Author.Bot && a.session != nil && a.session.State != nil && a.session.State.User != nil && msg.Author.ID == a.session.State.User.ID {
		a.logger.Debug("Skipping self message", map[string]any{"eventType": string(eventType), "messageId": msg.ID})
		return nil, false
	}
	if msg.GuildID == "" {
		a.logger.Debug("Skipping non-guild message", map[string]any{"eventType": string(eventType), "messageId": msg.ID, "channelId": msg.ChannelID})
		return nil, false
	}
	if msg.GuildID != a.guildID {
		a.logger.Debug("Skipping message from other guild", map[string]any{"eventType": string(eventType), "messageId": msg.ID, "channelId": msg.ChannelID, "guildId": msg.GuildID, "expectedGuildId": a.guildID})
		return nil, false
	}
	cfg, ok := a.allowed[msg.ChannelID]
	if !ok {
		a.logger.Debug("Skipping message from non-allowlisted channel", map[string]any{"eventType": string(eventType), "messageId": msg.ID, "channelId": msg.ChannelID})
		return nil, false
	}

	channelName := cfg.Name
	if channelName == "" {
		channelName = msg.ChannelID
	}

	embedParts := make([]string, 0, len(msg.Embeds))
	for _, e := range msg.Embeds {
		fields := make([]string, 0, len(e.Fields))
		for _, f := range e.Fields {
			fields = append(fields, strings.TrimSpace(f.Name+" "+f.Value))
		}
		embedParts = append(embedParts, strings.TrimSpace(strings.Join([]string{e.Title, e.Description, strings.Join(fields, " ")}, " ")))
	}
	attachmentURLs := make([]string, 0, len(msg.Attachments))
	for _, a := range msg.Attachments {
		attachmentURLs = append(attachmentURLs, a.URL)
	}

	ts := msg.Timestamp
	if msg.EditedTimestamp != nil {
		ts = *msg.EditedTimestamp
	}

	permalink := fmt.Sprintf("https://discord.com/channels/%s/%s/%s", msg.GuildID, msg.ChannelID, msg.ID)
	return &model.AnnouncementEvent{
		Platform:       "discord",
		GuildID:        msg.GuildID,
		GuildName:      "",
		ChannelID:      msg.ChannelID,
		ChannelName:    channelName,
		MessageID:      msg.ID,
		AuthorName:     msg.Author.Username,
		TimestampISO:   ts.Format(time.RFC3339Nano),
		ContentText:    msg.Content,
		EmbedText:      strings.TrimSpace(strings.Join(embedParts, "\n")),
		AttachmentURLs: attachmentURLs,
		Permalink:      permalink,
		EventType:      eventType,
	}, true
}
