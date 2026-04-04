# ChatAPI

Self-hosted, open source chat infrastructure for AI-powered apps.

[![Go Version](https://img.shields.io/badge/go-1.22+-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)

A single binary that gives your app real-time messaging, AI bot participants, and LLM streaming — without Sendbird's per-MAU pricing or handing your conversation data to a third party.

---

## Why ChatAPI

| | ChatAPI | Stream / Sendbird | Roll your own |
|---|---|---|---|
| Self-hosted | ✓ | ✗ | ✓ |
| AI bots native | ✓ | Add-on | You build it |
| LLM streaming | ✓ | ✗ | You build it |
| Open source | ✓ | ✗ | N/A |
| Per-MAU pricing | Free | Expensive | Infra cost |

---

## Features

- **Real-time WebSocket messaging** — DM, group, and channel rooms
- **AI bots as first-class participants** — add a bot to any room like a user
- **LLM streaming** — token-by-token streaming over WebSocket (`message.stream.*`)
- **Works with any LLM** — OpenAI, Anthropic, Ollama, or any OpenAI-compatible endpoint
- **Durable delivery** — store-then-send with retry for offline users
- **Presence and typing indicators** — built-in
- **JWT auth** — your backend issues tokens, ChatAPI validates them
- **TypeScript SDK** — `npm install @hastenr/chatapi-sdk`
- **Portable by design** — repository interfaces let you swap SQLite for PostgreSQL; broker interface lets you swap in Redis for horizontal scaling

---

## Quick Start

```bash
git clone https://github.com/hastenr/chatapi.git
cd chatapi
cp .env.example .env          # set JWT_SECRET
go run ./cmd/chatapi
```

```bash
curl http://localhost:8080/health
# {"status":"ok","service":"chatapi","uptime":"2s","db_writable":true}
```

---

## How It Works

ChatAPI is chat infrastructure, not an AI framework. Your AI agent lives outside — ChatAPI is the real-time messaging layer between your agent and your users.

```
Your AI agent (any LLM / framework)
        ↕  REST API
    ChatAPI room
        ↕  WebSocket
      End user
```

**LLM bots (built-in)** — register a bot with a model config, add it to a room. ChatAPI calls the LLM, injects conversation history, and streams the reply. No code required for simple assistants.

**External bots** — any service can join a room as a bot participant via JWT, just like a user. Your agent calls the REST API. You own the logic.

---

## Scalability

ChatAPI runs on a single instance out of the box — SQLite, no external services, deploys on a $6 VPS.

When you need more:
- **PostgreSQL**: implement `repository.RoomRepository` (and others) with `$1` placeholders — zero service changes
- **Horizontal scaling**: implement `broker.Broker` backed by Redis pub/sub — zero service changes

The interfaces are defined. The SQLite implementations ship with the binary.

---

## Deploy

| Platform | |
|---|---|
| Docker Compose | `cp .env.example .env && docker compose up -d` |
| Fly.io | `fly launch` |
| Railway | Import repo, add a Volume at `/data` |
| Binary | See [Releases](https://github.com/hastenr/chatapi/releases) |

> Mount a persistent volume at `/data` for the SQLite database.

---

## Configuration

```env
LISTEN_ADDR=:8080
DATABASE_DSN=file:/data/chatapi.db?_journal_mode=WAL
JWT_SECRET=your-secret-here
ALLOWED_ORIGINS=https://yourapp.com
```

---

## License

MIT — see [LICENSE](LICENSE).
