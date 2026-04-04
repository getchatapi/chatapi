# Changelog

All notable changes to ChatAPI are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versions follow [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

### Changed — Breaking
- **Auth**: API key authentication removed. All clients now authenticate with JWT Bearer tokens (`Authorization: Bearer <token>`). Your backend signs JWTs with `JWT_SECRET`; ChatAPI validates them. The `sub` claim is the user ID.
- **Config**: `MASTER_API_KEY` removed. Set `JWT_SECRET` instead.
- **Config**: `DEFAULT_RATE_LIMIT` removed.
- **WebSocket**: Connect with `?token=<jwt>` instead of `?api_key=...`.

### Added
- **AI bots**: Register LLM-backed bots via `POST /bots`. Bots join rooms as first-class participants and auto-respond to inbound messages.
- **LLM streaming**: Bot replies stream token-by-token over WebSocket as `message.stream.start`, `message.stream.delta`, and `message.stream.end` events. Works with OpenAI, Anthropic, Ollama, and any OpenAI-compatible endpoint.
- **`POST /rooms/{id}/members`**: Add users or bots to an existing room.
- **Bot CRUD**: `GET /bots`, `GET /bots/{id}`, `DELETE /bots/{id}`.
- **Repository pattern**: All SQL extracted to `internal/repository/sqlite/`. Services depend on interfaces — swap SQLite for PostgreSQL by implementing the same interfaces in a new adapter with zero service changes.
- **Broker interface**: `internal/broker/` decouples WebSocket broadcast from transport. Default `LocalBroker` uses an in-process channel. Implement `broker.Broker` backed by Redis pub/sub to enable horizontal scaling.
- **TypeScript SDK**: Updated to JWT auth, added bot endpoints and streaming event types.

### Removed
- MCP server — REST API is sufficient for agent integration.
- Oversight / HITL primitives (`request_approval`, `await_response`, room state machine) — deferred to a separate project.
- Webhooks for offline delivery notification — removed in favour of simplicity.
- `IssueWSToken` / `ConsumeWSToken` — dead code from the old API key auth flow.

---

## [0.1.0] — 2025-12-20

### Added
- Initial release.
- Multitenant chat service with SQLite backend.
- REST API for rooms, messages, acknowledgments, and notifications.
- WebSocket API for real-time messaging, presence, and typing indicators.
- Durable message delivery with at-least-once guarantees and configurable retry logic.
- Per-room monotonic message sequencing.
- Background delivery worker and WAL checkpoint worker.
- Docker image and pre-built binaries for Linux, macOS, and Windows.
