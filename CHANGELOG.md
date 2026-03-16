# Changelog

All notable changes to ChatAPI are documented here.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).
Versions follow [Semantic Versioning](https://semver.org/).

---

## [Unreleased]

### Security
- API keys are now stored as SHA-256 hashes in the database (`api_key_hash` column). Plaintext keys are returned exactly once at creation time and cannot be recovered. Tenants created with v0.1.0 must be re-created after upgrading.
- Removed WebSocket `api_key` query parameter fallback — API keys must be supplied via the `X-API-Key` header only. Query parameters appear in server access logs.
- Server now refuses to start if `MASTER_API_KEY` is unset (previously defaulted to empty string, making admin endpoints unprotected).
- WebSocket origin checking is now enforced. Configure allowed origins via `WS_ALLOWED_ORIGINS` (comma-separated). Use `*` for development only.

### Added
- `GET /metrics` endpoint exposing operational counters: `active_connections`, `messages_sent`, `broadcast_drops`, `delivery_attempts`, `delivery_failures`, `uptime_seconds`.
- `WS_MAX_CONNECTIONS_PER_USER` config option (default `5`). Connections beyond the limit receive a `1008 Policy Violation` close frame.
- `WS_ALLOWED_ORIGINS` config option for WebSocket origin validation.

### Fixed
- Deadlock in `cleanupStalePresence`: calling `IsUserOnline` while holding the write lock caused a hang. Fixed by inlining the connection check.
- `GET /health` incorrectly returned `503` on a fresh (empty) database due to a nil-vs-empty-slice check bug in `testDatabaseWrite`.
- `go.mod` declared `go 1.21` but the code uses `net/http` features added in Go 1.22 (`r.PathValue`, method routing). Minimum version is now correctly declared as `go 1.22`.

### Changed
- Broadcast drop events are now logged at `ERROR` level (previously `WARN`) and include the message type and a running total counter.
- `GET /metrics` and `GET /health` are public endpoints (no authentication required).

### Migration notes (v0.1.0 → unreleased)

**Breaking**: The `tenants.api_key` column has been renamed to `api_key_hash` and now stores a SHA-256 hash (migration `009_hash_api_keys.sql`). Any tenants created with v0.1.0 will no longer authenticate because the stored value is plaintext, not a hash. Re-create all tenants after upgrading.

**Breaking**: WebSocket clients that relied on `?api_key=...` query parameter authentication must switch to the `X-API-Key` header.

**Required**: `MASTER_API_KEY` must be set before starting the server. The server exits with an error if it is missing.

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
- Hugo-based documentation site.
