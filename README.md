# StakeCraft Announcements Relay Bot (Go)

Discord `updates` channels -> Telegram relay bot with:
- channel allowlist
- keyword filtering (case-insensitive contains)
- edit notices
- SQLite dedup + retention pruning (`maxForwardedAnnouncements`)
- URL formatting as code + disabled Telegram link preview

## Build

Requirements: Go 1.23+

```bash
go mod tidy
go build -o bin/announcements-bot ./cmd/bot
go build -o bin/smoke-telegram ./cmd/smoke-telegram
```

## Run

1. Copy env/config templates:
```bash
cp .env.example .env
cp config/config.example.json config/config.json
```
2. Fill:
- `.env`: `DISCORD_BOT_TOKEN`, `TELEGRAM_BOT_TOKEN`
- `config/config.json`: guild id, channel ids, telegram chat ids

3. Start binary:
```bash
CONFIG_PATH=config/config.json ENV_FILE=.env ./bin/announcements-bot
```

## Smoke test Telegram

```bash
CONFIG_PATH=config/config.json ENV_FILE=.env ./bin/smoke-telegram "hello from smoke"
```

## systemd

Unit file: `deploy/systemd/announcements-bot.service`

Install on server:
```bash
sudo cp deploy/systemd/announcements-bot.service /etc/systemd/system/
sudo useradd --system --create-home --home-dir /opt/announcements-bot annbot || true
sudo mkdir -p /opt/announcements-bot/bin /opt/announcements-bot/config /opt/announcements-bot/data
sudo cp bin/announcements-bot /opt/announcements-bot/bin/
sudo cp .env /opt/announcements-bot/.env
sudo cp config/config.json /opt/announcements-bot/config/config.json
sudo chown -R annbot:annbot /opt/announcements-bot
sudo systemctl daemon-reload
sudo systemctl enable --now announcements-bot
sudo systemctl status announcements-bot
sudo journalctl -u announcements-bot -f
```

## Notes

- Bot needs Discord intents: `SERVER MEMBERS INTENT` not required, but `MESSAGE CONTENT INTENT` must be enabled.
- Discord OAuth scope: `bot`.
- Required channel permissions: `View Channels`, `Read Message History`.
