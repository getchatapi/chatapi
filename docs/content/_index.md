+++
title = "ChatAPI Documentation"
type = "book"
weight = 1
+++

<p align="center">
  <img src="/logo.svg" width="220" alt="ChatAPI" />
</p>

<p align="center">Real-time chat infrastructure for AI-powered apps.</p>

<p align="center">
  <a href="https://golang.org/"><img src="https://img.shields.io/badge/go-1.22+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go version" /></a>
  <a href="https://github.com/getchatapi/chatapi/blob/main/LICENSE"><img src="https://img.shields.io/badge/license-MIT-00ED64?style=flat-square&labelColor=001E2B" alt="License" /></a>
  <a href="https://github.com/getchatapi/chatapi/releases"><img src="https://img.shields.io/github/v/release/getchatapi/chatapi?style=flat-square&color=00ED64&labelColor=001E2B" alt="Release" /></a>
  <a href="https://github.com/getchatapi/chatapi/actions"><img src="https://img.shields.io/github/actions/workflow/status/getchatapi/chatapi/ci.yml?style=flat-square&labelColor=001E2B" alt="CI" /></a>
</p>

<p align="center">
  <a href="/getting-started/">Quick Start</a> ·
  <a href="/api/rest/">API Reference</a> ·
  <a href="/guides/bots/">AI Bots</a> ·
  <a href="https://github.com/getchatapi/chatapi">GitHub</a>
</p>

---

ChatAPI is a self-hosted messaging server for apps where AI participates in conversations. It handles real-time delivery, message history, presence, typing indicators, and LLM streaming — so you focus on your product, not the plumbing.

## Features

- **Managed AI bots** — register a bot with an LLM provider URL; ChatAPI calls the model, streams tokens back via `message.stream.*` events, and stores the reply
- **Real-time messaging** — rooms (DM or group) with presence, typing indicators, and at-least-once delivery
- **JWT auth** — your backend signs tokens, ChatAPI validates them; no vendor accounts, no sessions
- **Offline delivery** — messages queue for offline users; webhook fires so you can send push notifications
- **Escalation support** — webhook can return `{"skip": true}` to silence a bot when a human agent takes over
- **Single binary** — SQLite included, no external services required
- **Portable** — swap SQLite → PostgreSQL or local pub/sub → Redis by implementing one interface

## Quick start

```bash
docker run -d \
  -p 8080:8080 \
  -e JWT_SECRET=$(openssl rand -base64 32) \
  -e ALLOWED_ORIGINS="*" \
  -v chatapi-data:/data \
  hastenr/chatapi:latest
```

```bash
curl http://localhost:8080/health
# {"status":"ok","db_writable":true}
```

## Documentation

- [Getting Started](/getting-started/) — Installation, configuration, and first API call
- [REST API](/api/rest/) — HTTP endpoint reference
- [WebSocket API](/api/websocket/) — Real-time event reference
- [AI Bots](/guides/bots/) — Register a bot and connect it to a room
- [Architecture](/architecture/) — System design and internals
