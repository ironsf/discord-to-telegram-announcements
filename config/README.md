# Config Guide

This folder contains the application config template.

Use [config.example.json] as the starting point and create your real runtime config as `config.json`.

## Quick Start

1. Copy the example file:

```bash
cp config/config.example.json config/config.json
```

2. Replace placeholder values:
- `discord.guildId`
- `telegram.mainChatId`
- `telegram.opsChatId`

3. Adjust Discord channels and keywords to match the channels you want to forward.

## Top-Level Sections

### `discord`

Controls which Discord server and channels are monitored.

- `guildId`: Discord server ID.
- `allowedChannels`: list of channels the bot is allowed to read and forward.

Each channel object supports:

- `id`: Discord channel ID.
- `name`: internal channel label used in logs and Telegram message headers.
- `enabled`: whether this channel is active.
- `keywords`: case-insensitive keyword match list. If the incoming message contains one of these values, it may be forwarded.
- `theme`: visual Telegram marker for the channel header.
  Supported values:
  - `green`
  - `blue`
  - `yellow`
  - `red`
  - `orange`
  - `purple`
  - `white`
  - `gray`
  - `grey`
- `titleBold`: makes the channel title bold in Telegram output.

Example:

```json
{
  "id": "969324288152829962",
  "name": "solana-tds",
  "enabled": true,
  "keywords": ["solana", "tds", "upgrade"],
  "theme": "green",
  "titleBold": true
}
```

### `telegram`

Controls where messages are sent and how they are formatted.

- `mainChatId`: Telegram chat ID for forwarded announcements.
- `opsChatId`: Telegram chat ID for internal bot alerts.
- `messageFormat`: output style for forwarded messages.

Supported `messageFormat` values:

- `card`: richer layout with labels and separated sections.
- `minimal`: more compact layout.

### `runtime`

Controls runtime behavior.

- `logLevel`: logging verbosity, usually `info` or `debug`.
- `telegramTimeoutMs`: Telegram API timeout in milliseconds.
- `alertCooldownSeconds`: minimum delay between repeated ops alerts with the same key.

### `storage`

Controls local SQLite persistence.

- `sqlitePath`: path to the SQLite database file.
- `maxForwardedAnnouncements`: retention limit for forwarded announcements.
- `pruneOnStartup`: whether old records should be pruned during startup.

## Notes

- Keep `config.json` local and do not commit it with real IDs or secrets.
- Commit only [config.example.json] with placeholder values.
- Discord and Telegram bot tokens are not stored in this file. They should be provided through environment variables.
