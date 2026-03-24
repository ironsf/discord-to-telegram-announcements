package store

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

type SeenState struct {
	Forwarded bool
}

type PersistForwardInput struct {
	MessageID       string
	ChannelID       string
	ChannelName     string
	MatchedKeywords []string
	TelegramChatID  string
	TelegramMsgID   string
}

type Counts struct {
	ForwardedAnnouncements int
	ProcessedMessages      int
	Maps                   int
	Edits                  int
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		_ = db.Close()
		return nil, err
	}
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS processed_messages (
  source_platform TEXT NOT NULL,
  source_channel_id TEXT NOT NULL,
  source_message_id TEXT NOT NULL PRIMARY KEY,
  content_hash TEXT NOT NULL,
  first_seen_at TEXT NOT NULL,
  matched INTEGER NOT NULL,
  forwarded INTEGER NOT NULL,
  forwarded_at TEXT,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS forwarded_announcements (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  source_message_id TEXT NOT NULL UNIQUE,
  source_channel_id TEXT NOT NULL,
  source_channel_name TEXT NOT NULL,
  telegram_chat_id TEXT NOT NULL,
  telegram_message_id TEXT NOT NULL,
  forwarded_at TEXT NOT NULL,
  matched_keywords_csv TEXT NOT NULL,
  FOREIGN KEY (source_message_id) REFERENCES processed_messages(source_message_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS forwarded_message_map (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  source_message_id TEXT NOT NULL,
  telegram_chat_id TEXT NOT NULL,
  telegram_message_id TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (source_message_id) REFERENCES forwarded_announcements(source_message_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS edit_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  source_message_id TEXT NOT NULL,
  edit_hash TEXT NOT NULL,
  edited_at TEXT NOT NULL,
  update_notice_telegram_message_id TEXT,
  FOREIGN KEY (source_message_id) REFERENCES forwarded_announcements(source_message_id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS ops_alert_state (
  alert_key TEXT PRIMARY KEY,
  last_sent_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_forwarded_announcements_forwarded_at ON forwarded_announcements(forwarded_at);
CREATE INDEX IF NOT EXISTS idx_forwarded_announcements_source_message_id ON forwarded_announcements(source_message_id);
CREATE INDEX IF NOT EXISTS idx_edit_events_source_message_id ON edit_events(source_message_id);
`)
	return err
}

func (s *Store) UpsertSeenMessage(messageID, channelID, contentHash string, matched bool) (SeenState, error) {
	row := s.db.QueryRow(`SELECT forwarded FROM processed_messages WHERE source_message_id = ?`, messageID)
	var forwarded int
	err := row.Scan(&forwarded)
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if errors.Is(err, sql.ErrNoRows) {
		_, err = s.db.Exec(`INSERT INTO processed_messages (
source_platform, source_channel_id, source_message_id, content_hash, first_seen_at, matched, forwarded, forwarded_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, 0, NULL, ?)`, "discord", channelID, messageID, contentHash, now, boolToInt(matched), now)
		return SeenState{Forwarded: false}, err
	}
	if err != nil {
		return SeenState{}, err
	}
	_, err = s.db.Exec(`UPDATE processed_messages SET content_hash = ?, matched = ?, updated_at = ? WHERE source_message_id = ?`, contentHash, boolToInt(matched), now, messageID)
	if err != nil {
		return SeenState{}, err
	}
	return SeenState{Forwarded: forwarded == 1}, nil
}

func (s *Store) MarkForwarded(input PersistForwardInput) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.Exec(`UPDATE processed_messages SET forwarded = 1, forwarded_at = ?, updated_at = ? WHERE source_message_id = ?`, now, now, input.MessageID); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT OR REPLACE INTO forwarded_announcements (
source_message_id, source_channel_id, source_channel_name, telegram_chat_id, telegram_message_id, forwarded_at, matched_keywords_csv
) VALUES (?, ?, ?, ?, ?, ?, ?)`, input.MessageID, input.ChannelID, input.ChannelName, input.TelegramChatID, input.TelegramMsgID, now, csvJoin(input.MatchedKeywords)); err != nil {
		return err
	}
	if _, err = tx.Exec(`INSERT INTO forwarded_message_map (
source_message_id, telegram_chat_id, telegram_message_id, created_at
) VALUES (?, ?, ?, ?)`, input.MessageID, input.TelegramChatID, input.TelegramMsgID, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) HasEditHash(messageID, editHash string) (bool, error) {
	row := s.db.QueryRow(`SELECT 1 FROM edit_events WHERE source_message_id = ? AND edit_hash = ?`, messageID, editHash)
	var one int
	err := row.Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) RecordEditNotice(messageID, editHash, telegramMsgID string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(`INSERT INTO edit_events (source_message_id, edit_hash, edited_at, update_notice_telegram_message_id) VALUES (?, ?, ?, ?)`, messageID, editHash, now, nullable(telegramMsgID))
	return err
}

func (s *Store) PruneForwardedAnnouncementsToMax(limit int) (int, error) {
	if limit < 1 {
		return 0, fmt.Errorf("retention limit must be >= 1")
	}
	row := s.db.QueryRow(`SELECT COUNT(*) FROM forwarded_announcements`)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	if count <= limit {
		return 0, nil
	}
	overflow := count - limit

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	rows, err := tx.Query(`SELECT source_message_id FROM forwarded_announcements ORDER BY datetime(forwarded_at) ASC, id ASC LIMIT ?`, overflow)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	victims := make([]string, 0, overflow)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return 0, err
		}
		victims = append(victims, id)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	for _, id := range victims {
		if _, err := tx.Exec(`DELETE FROM forwarded_announcements WHERE source_message_id = ?`, id); err != nil {
			return 0, err
		}
		if _, err := tx.Exec(`DELETE FROM processed_messages WHERE source_message_id = ?`, id); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(victims), nil
}

func (s *Store) ShouldSendAlert(alertKey string, cooldownSeconds int) (bool, error) {
	row := s.db.QueryRow(`SELECT last_sent_at FROM ops_alert_state WHERE alert_key = ?`, alertKey)
	var last string
	err := row.Scan(&last)
	if errors.Is(err, sql.ErrNoRows) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	t, err := time.Parse(time.RFC3339Nano, last)
	if err != nil {
		return true, nil
	}
	return time.Since(t) >= time.Duration(cooldownSeconds)*time.Second, nil
}

func (s *Store) MarkAlertSent(alertKey string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(`INSERT INTO ops_alert_state (alert_key, last_sent_at) VALUES (?, ?) ON CONFLICT(alert_key) DO UPDATE SET last_sent_at = excluded.last_sent_at`, alertKey, now)
	return err
}

func (s *Store) Counts() (Counts, error) {
	q := func(sqlText string) (int, error) {
		row := s.db.QueryRow(sqlText)
		var c int
		if err := row.Scan(&c); err != nil {
			return 0, err
		}
		return c, nil
	}
	forwarded, err := q(`SELECT COUNT(*) FROM forwarded_announcements`)
	if err != nil {
		return Counts{}, err
	}
	processed, err := q(`SELECT COUNT(*) FROM processed_messages`)
	if err != nil {
		return Counts{}, err
	}
	maps, err := q(`SELECT COUNT(*) FROM forwarded_message_map`)
	if err != nil {
		return Counts{}, err
	}
	edits, err := q(`SELECT COUNT(*) FROM edit_events`)
	if err != nil {
		return Counts{}, err
	}
	return Counts{
		ForwardedAnnouncements: forwarded,
		ProcessedMessages:      processed,
		Maps:                   maps,
		Edits:                  edits,
	}, nil
}

func (s *Store) HasForwarded(messageID string) (bool, error) {
	row := s.db.QueryRow(`SELECT 1 FROM forwarded_announcements WHERE source_message_id = ?`, messageID)
	var one int
	err := row.Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func boolToInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func csvJoin(items []string) string {
	if len(items) == 0 {
		return ""
	}
	out := items[0]
	for i := 1; i < len(items); i++ {
		out += "," + items[i]
	}
	return out
}
