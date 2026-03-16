# ChatAPI

A lightweight, multitenant chat service built in Go with SQLite, WebSocket support, and durable message delivery.

[![Documentation](https://img.shields.io/badge/docs-hugo-blue)](https://hastenr.github.io/chatapi/)
[![Go Version](https://img.shields.io/badge/go-1.22+-blue)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green)](LICENSE)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/hastenr/chatapi)](https://github.com/hastenr/chatapi/releases)
[![Docker Image Version (latest by date)](https://img.shields.io/docker/v/hastenr/chatapi)](https://hub.docker.com/r/hastenr/chatapi)

## Deploy

[![Deploy on Railway](https://railway.app/button.svg)](https://railway.app/new/template?template=https://github.com/hastenr/chatapi)
[![Deploy to Render](https://render.com/images/deploy-to-render-button.svg)](https://render.com/deploy?repo=https://github.com/hastenr/chatapi)

| Platform | Guide |
|----------|-------|
| **Docker Compose** | `cp .env.example .env && docker compose up -d` |
| **Fly.io** | `fly launch` then `fly secrets set MASTER_API_KEY=...` |
| **Railway** | Import repo, add a Volume at `/data`, set `MASTER_API_KEY` |
| **Render** | Click the button above — `MASTER_API_KEY` is auto-generated |
| **Binary** | See [Releases](https://github.com/hastenr/chatapi/releases) |

> All containerised deployments mount a persistent volume at `/data` for the SQLite database.

## Features

- **Multitenant**: API key-based tenancy with per-tenant rate limiting
- **Real-time messaging**: WebSocket connections for instant delivery
- **Durable delivery**: Store-then-send with at-least-once guarantees
- **Message sequencing**: Per-room ordering with client acknowledgments
- **Room metadata**: Attach arbitrary app-level context to rooms at creation time
- **Offline webhooks**: POST to your backend when a message arrives for an offline user
- **TypeScript SDK**: First-class client SDK — `npm install @hastenr/chatapi-sdk`
- **SQLite backend**: WAL mode for concurrent reads/writes

## Quick Start

```bash
git clone https://github.com/hastenr/chatapi.git
cd chatapi
go mod download
MASTER_API_KEY=your-secret-key go run ./cmd/chatapi
```

```bash
curl http://localhost:8080/health
# {"status":"ok","service":"chatapi","uptime":"5s","db_writable":true}
```

## Documentation

📚 **[Full documentation](https://hastenr.github.io/chatapi/)** — API reference, configuration, guides, and architecture.

## License

MIT License - see [LICENSE](LICENSE) file for details.
