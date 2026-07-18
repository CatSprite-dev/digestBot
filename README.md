# digestBot

A self-hosted Telegram bot that reads large public chats and generates LLM-powered digests on demand.

## Description

digestBot connects to Telegram as a userbot (MTProto) and silently collects messages from chats you add. When you ask for a digest, it summarizes what happened while you were away — and remembers the previous digest as context, so summaries stay coherent over time.

Built with Go, SQLite, and any OpenAI-compatible LLM API (tested with Groq).

## Motivation

Large Telegram chats with thousands of members move too fast to read manually. I needed something that reads them for me and surfaces what actually matters — concrete topics, useful advice, tools mentioned — without me scrolling through hundreds of messages.

## Quick Start

**Prerequisites:** Go 1.25+, Docker, a Telegram account, Telegram API credentials from [my.telegram.org](https://my.telegram.org)

**1. Clone and configure**

```bash
git clone https://github.com/CatSprite-dev/digestBot.git
cd digestBot
cp .env.example .env
# fill in your credentials
```

**2. Authorize the userbot (one-time)**

```bash
mkdir -p data
go run ./cmd/qrauth
# scan the QR with Telegram on your phone:
# Settings → Devices → Link Desktop Device
```

**3. Run**

```bash
docker compose up -d
```

## Usage

Talk to your bot in Telegram DMs:

| Command | Description |
|---|---|
| `/add @username` | Start tracking a chat and load its recent history |
| `/remove @username` | Stop tracking a chat |
| `/chats` | List all tracked chats |
| `/digest @username` | Generate a digest of new messages since your last one |

Digests are per-user — each person gets their own cursor and context.

## Contributing

Pull requests are welcome. For major changes, open an issue first to discuss what you'd like to change.