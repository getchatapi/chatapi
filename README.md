<p align="center">
  <img src="docs/static/logo.svg" width="240" alt="ChatAPI" />
</p>

<p align="center">
  Self-hosted chat infrastructure for AI-powered apps.
</p>

<p align="center">
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go version" /></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-00ED64?style=flat-square&labelColor=001E2B" alt="License" /></a>
  <a href="https://github.com/hastenr/chatapi/releases"><img src="https://img.shields.io/github/v/release/hastenr/chatapi?style=flat-square&color=00ED64&labelColor=001E2B" alt="Release" /></a>
  <a href="https://github.com/hastenr/chatapi/actions"><img src="https://img.shields.io/github/actions/workflow/status/hastenr/chatapi/ci.yml?style=flat-square&labelColor=001E2B" alt="CI" /></a>
</p>

<p align="center">
  <a href="https://docs.chatapi.cloud/">Docs</a> ·
  <a href="https://docs.chatapi.cloud/getting-started/">Quick Start</a> ·
  <a href="https://docs.chatapi.cloud/api/rest/">API Reference</a> ·
  <a href="https://docs.chatapi.cloud/guides/bots/">AI Bots</a>
</p>

---

A single binary that gives your app real-time messaging, AI bot participants, and LLM streaming — without Sendbird's per-MAU pricing or handing your conversation data to a third party.

## Features

- **Real-time WebSocket messaging** — DM and group rooms with presence and typing indicators
- **AI bots as first-class participants** — add a bot to any room like a regular user
- **LLM streaming** — token-by-token streaming over WebSocket (`message.stream.*`)
- **Works with any LLM** — OpenAI, Anthropic, Ollama, or any OpenAI-compatible endpoint
- **Durable delivery** — store-then-send with retry; webhook fires when a user is offline so you can send push notifications
- **JWT auth** — your backend signs tokens, ChatAPI validates them. No API keys, no sessions
- **TypeScript SDK** — `npm install @hastenr/chatapi-sdk`
- **Portable by design** — swap SQLite → PostgreSQL or local pub/sub → Redis without touching business logic

## Quick Start

> Requires CGO for the SQLite driver. Install gcc if needed: `brew install gcc` / `apt install build-essential`

```bash
git clone https://github.com/hastenr/chatapi.git
cd chatapi
cp .env.example .env    # set JWT_SECRET
go run ./cmd/chatapi
```

```bash
curl http://localhost:8080/health
# {"status":"ok","db_writable":true}
```

Or run with Docker — no build toolchain needed:

```bash
docker run -d \
  -p 8080:8080 \
  -e JWT_SECRET=$(openssl rand -base64 32) \
  -e ALLOWED_ORIGINS="*" \
  -v chatapi-data:/data \
  -e DATABASE_DSN=file:/data/chatapi.db \
  hastenr/chatapi:latest
```

## How It Works

ChatAPI is the messaging layer — not an AI framework. Your agent lives outside; ChatAPI connects it to your users.

```
Your AI agent (any LLM / framework)
        ↕  REST API
    ChatAPI room
        ↕  WebSocket
      End user
```

**LLM bots (built-in)** — register a bot with a model and API key, add it to a room. ChatAPI calls the LLM, injects conversation history as context, and streams the reply.

**External bots** — any process joins a room via JWT, just like a user. Your agent handles all the logic over REST or WebSocket.

## Why ChatAPI

| | ChatAPI | Stream / Sendbird | Roll your own |
|---|---|---|---|
| Self-hosted | ✓ | ✗ | ✓ |
| AI bots native | ✓ | Add-on | You build it |
| LLM streaming | ✓ | ✗ | You build it |
| Open source | ✓ | ✗ | N/A |
| Per-MAU pricing | Free | Expensive | Infra cost |

## Deploy

| Platform | |
|---|---|
| Docker Compose | `cp .env.example .env && docker compose up -d` |
| Fly.io | `fly launch` |
| Railway | Import repo, add a volume at `/data` |
| Binary | [Releases](https://github.com/hastenr/chatapi/releases) |

## Configuration

Only two variables are required:

```env
JWT_SECRET=your-secret-here          # generate: openssl rand -base64 32
ALLOWED_ORIGINS=https://yourapp.com  # required for browser clients
```

Everything else has a sensible default. See [`.env.example`](.env.example).

## Scaling

Runs on a single instance out of the box — SQLite, no external services, deploys on a $6 VPS.

When you outgrow it:
- **PostgreSQL** — implement the repository interfaces with `$1` placeholders. Zero service changes.
- **Horizontal scaling** — implement `broker.Broker` backed by Redis pub/sub. Zero service changes.

## License

MIT — see [LICENSE](LICENSE).
