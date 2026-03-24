package model

type AnnouncementEventType string

const (
	EventCreated AnnouncementEventType = "created"
	EventEdited  AnnouncementEventType = "edited"
)

type AnnouncementEvent struct {
	Platform       string
	GuildID        string
	GuildName      string
	ChannelID      string
	ChannelName    string
	MessageID      string
	AuthorName     string
	TimestampISO   string
	ContentText    string
	EmbedText      string
	AttachmentURLs []string
	Permalink      string
	EventType      AnnouncementEventType
}

type ChannelConfig struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Enabled  bool     `json:"enabled"`
	Keywords []string `json:"keywords"`
}

type AppConfig struct {
	Discord struct {
		GuildID         string          `json:"guildId"`
		AllowedChannels []ChannelConfig `json:"allowedChannels"`
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
		PruneOnStartup            bool   `json:"pruneOnStartup"`
	} `json:"storage"`
}

type ForwardRequest struct {
	SourcePrefix   string
	Text           string
	Permalink      string
	AttachmentURLs []string
	TimestampISO   string
	AuthorName     string
}

type EditNoticeRequest struct {
	SourcePrefix string
	ChannelName  string
	Permalink    string
	Text         string
	TimestampISO string
}

type ForwardResult struct {
	ChatID    string
	MessageID string
}

type FilterDecision struct {
	Matched         bool
	MatchedKeywords []string
}
