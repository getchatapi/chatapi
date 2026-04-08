---
title: "Architecture"
weight: 40
---

# Architecture Overview

ChatAPI is a single-binary chat service built on standard Go, SQLite, and JWT auth. It is designed for single-instance deployments with clean interfaces that make it straightforward to scale out when needed.

## Layered Architecture

```
┌───────────────────────────────────────────────────┐
│  Handlers (internal/handlers/)                    │
│  REST (rest/)  ·  WebSocket (ws/)                 │
└──────────────────────┬────────────────────────────┘
                       │
┌──────────────────────▼────────────────────────────┐
│  Services (internal/services/)                    │
│  chatroom · message · delivery · realtime · bot   │
└──────────────────────┬────────────────────────────┘
                       │
┌──────────────────────▼────────────────────────────┐
│  Repository interfaces (internal/repository/)     │
│  Implemented by: internal/repository/sqlite/      │
└──────────────────────┬────────────────────────────┘
                       │
┌──────────────────────▼────────────────────────────┐
│  SQLite (WAL mode)  ·  Broker (LocalBroker)       │
└───────────────────────────────────────────────────┘
```

### Handlers

- `internal/handlers/rest/` — HTTP handlers. All auth and HTTP status decisions live here.
- `internal/handlers/ws/` — WebSocket handler. Manages per-user connection pools, reads client messages, routes broker events.

### Services

Business logic lives in `internal/services/<name>/service.go`. Services receive dependencies via constructor injection and never import handlers.

| Package | Responsibility |
|---------|---------------|
| `chatroom` | Room creation, membership, metadata |
| `message` | Message storage, sequencing, editing |
| `delivery` | Undelivered queue, retry, webhook calls for offline users |
| `realtime` | WebSocket connection registry, pub/sub fan-out |
| `bot` | Bot registration, LLM invocation, streaming |
| `webhook` | Outbound HTTP calls to your backend for offline delivery |

### Repository interfaces

All SQL lives in `internal/repository/sqlite/`. Services depend on repository interfaces, not concrete SQLite types. To swap in PostgreSQL, implement the same interfaces in `internal/repository/postgres/` — no service code changes required.

### Broker interface

The `broker.Broker` interface abstracts pub/sub fan-out. The default is `LocalBroker` (in-process channel-based). To run multiple ChatAPI instances behind a load balancer, implement `broker.Broker` backed by Redis Pub/Sub. The WebSocket handler and realtime service are the only callers.

## Security Model

### Authentication

All REST endpoints (except `GET /health` and `GET /metrics`) require:

```
Authorization: Bearer <jwt>
```

WebSocket connections accept:

- `?token=<jwt>` query parameter — for browser clients (browsers cannot set custom WebSocket headers)
- `Authorization: Bearer <jwt>` header — for server-side clients

The JWT must be signed with HS256 using `JWT_SECRET`. The `sub` claim is the user ID for the request.

There are no API keys, no master keys, no session tokens, and no `POST /ws/token` endpoint.

### Authorization

- Users can only list/read rooms they are members of.
- Users can only edit or delete their own messages.
- Bot endpoints require a valid JWT; the `sub` is used to scope bot ownership.

### CORS

WebSocket origin checks and REST CORS headers are both controlled by `ALLOWED_ORIGINS` (comma-separated). Use `*` for local development. Leave it unset to reject all browser-origin connections.

## Database Schema

ChatAPI uses SQLite with WAL mode. Migrations live in `internal/db/migrations/`.

```sql
-- Rooms
CREATE TABLE rooms (
  room_id    TEXT PRIMARY KEY,
  type       TEXT NOT NULL,     -- 'dm' | 'group'
  unique_key TEXT NULL,         -- deterministic key for DMs
  name       TEXT NULL,
  metadata   JSON NULL,         -- arbitrary app-level context
  last_seq   INTEGER DEFAULT 0,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Room membership
CREATE TABLE room_members (
  chatroom_id TEXT NOT NULL,
  user_id     TEXT NOT NULL,
  role        TEXT DEFAULT 'member',
  joined_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (chatroom_id, user_id)
);

-- Messages with per-room sequencing
CREATE TABLE messages (
  message_id  TEXT PRIMARY KEY,
  chatroom_id TEXT NOT NULL,
  sender_id   TEXT NOT NULL,
  seq         INTEGER NOT NULL,
  content     TEXT NOT NULL,
  meta        TEXT NULL,        -- arbitrary JSON string per message
  created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Per-user delivery tracking (last ACKed seq per room)
CREATE TABLE delivery_state (
  user_id     TEXT NOT NULL,
  chatroom_id TEXT NOT NULL,
  last_ack    INTEGER DEFAULT 0,
  updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (user_id, chatroom_id)
);

-- Undelivered message queue (for offline users)
CREATE TABLE undelivered_messages (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id         TEXT NOT NULL,
  chatroom_id     TEXT NOT NULL,
  message_id      TEXT NOT NULL,
  seq             INTEGER NOT NULL,
  attempts        INTEGER DEFAULT 0,
  created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
  last_attempt_at DATETIME NULL
);

-- Bots
CREATE TABLE bots (
  bot_id        TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  mode          TEXT NOT NULL DEFAULT 'llm',  -- 'llm' | 'external'
  provider      TEXT,                          -- 'openai' | 'anthropic'
  base_url      TEXT,                          -- override for OpenAI-compatible endpoints
  model         TEXT,
  api_key       TEXT,
  system_prompt TEXT,
  max_context   INTEGER NOT NULL DEFAULT 20,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
```

## Data Flow

### Message sending

1. Client sends `POST /rooms/{id}/messages` or a `send_message` WebSocket frame.
2. Handler validates JWT; services validate room membership.
3. A transaction atomically increments `rooms.last_seq` and inserts the message.
4. The message is fanned out to all online room members via the Broker.
5. Offline members are written to `undelivered_messages` for background retry.

### LLM bot response

1. A message arrives in a room that has a registered `llm` bot.
2. The bot service fetches the last N messages (`max_context`) as context.
3. It calls the LLM provider (OpenAI or Anthropic). If the provider supports streaming, tokens are forwarded as `message.stream.delta` WebSocket events.
4. When the stream ends, the complete message is stored and a `message.stream.end` event is broadcast.

### WebSocket connection lifecycle

1. Client connects to `ws://.../ws` with a JWT (query param or header).
2. Server validates the token, registers the connection, and broadcasts `presence.update` online.
3. The client fetches missed messages via `GET /rooms/{id}/messages?after_seq=<last_seen>` for each room.
4. On disconnect, the server waits a short grace period before broadcasting `presence.update` offline.

## Scaling

### Vertical (default)

SQLite WAL mode supports high read concurrency with a single writer. Suitable for most single-instance deployments.

### Swap the storage backend

Implement the repository interfaces in `internal/repository/postgres/` and point `db.New()` at PostgreSQL. No service changes required.

### Horizontal (multi-instance)

1. Implement `broker.Broker` backed by Redis Pub/Sub (`internal/broker/redis/`).
2. Use a PostgreSQL repo.
3. Place a load balancer in front. Sticky sessions are not required because the broker fans out across instances.

## Operational Notes

### Health and metrics

| Endpoint | Auth | Description |
|----------|------|-------------|
| `GET /health` | none | Returns `{"status":"ok","db_writable":true}` or 503 |
| `GET /metrics` | none | Live counters: `active_connections`, `messages_sent`, `broadcast_drops`, `delivery_attempts`, `delivery_failures`, `uptime_seconds` |

### TLS

ChatAPI binds plain HTTP. Terminate TLS at a reverse proxy.

**Caddy (automatic HTTPS):**
```
your.domain.com {
    reverse_proxy localhost:8080
}
```

**nginx:**
```nginx
location / {
    proxy_pass         http://localhost:8080;
    proxy_http_version 1.1;
    proxy_set_header   Upgrade    $http_upgrade;
    proxy_set_header   Connection "upgrade";
    proxy_set_header   Host       $host;
}
```
