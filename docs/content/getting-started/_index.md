---
title: "Getting Started"
weight: 10
---

# Getting Started

## Install

### Docker (recommended)

```bash
docker run -d \
  -p 8080:8080 \
  -e JWT_SECRET=$(openssl rand -base64 32) \
  -e ALLOWED_ORIGINS="*" \
  -v chatapi-data:/data \
  hastenr/chatapi:latest
```

### Docker Compose

```bash
git clone https://github.com/getchatapi/chatapi.git
cd chatapi
cp .env.example .env   # fill in JWT_SECRET
docker compose up -d
```

### Build from source

Requires Go 1.22+ and gcc (for the SQLite driver).

```bash
git clone https://github.com/getchatapi/chatapi.git
cd chatapi
go build -o bin/chatapi ./cmd/chatapi
export JWT_SECRET=$(openssl rand -base64 32)
export ALLOWED_ORIGINS="*"
./bin/chatapi
```

### Health check

```bash
curl http://localhost:8080/health
# {"status":"ok","db_writable":true}
```

---

## Configuration

All configuration is via environment variables.

| Variable | Default | Description |
|----------|---------|-------------|
| `JWT_SECRET` | **required** | HS256 secret for validating Bearer tokens. Generate: `openssl rand -base64 32` |
| `ALLOWED_ORIGINS` | *(none)* | Comma-separated CORS origins. Required for browser clients. Use `*` for local dev. |
| `WEBHOOK_URL` | *(none)* | Your backend URL. Required if you use bots (provides system prompt). Also called for offline push notifications. |
| `WEBHOOK_SECRET` | *(none)* | HMAC-SHA256 secret — ChatAPI signs webhook requests with this. Verify the `X-ChatAPI-Signature` header on your end. |
| `LISTEN_ADDR` | `:8080` | Server listen address |
| `DATABASE_DSN` | `file:./chatapi.db` (binary) / `file:/data/chatapi.db` (Docker) | SQLite connection string. Mount a volume at `/data` in Docker to persist data. |
| `RATE_LIMIT_MESSAGES` | `10` | Sustained message sends per second per user. `0` to disable. |
| `RATE_LIMIT_MESSAGES_BURST` | `20` | Burst allowance on top of the sustained rate. |
| `WS_MAX_CONNECTIONS_PER_USER` | `5` | Max concurrent WebSocket connections per user. |
| `WORKER_INTERVAL` | `30s` | How often the delivery worker retries undelivered messages. |
| `SHUTDOWN_DRAIN_TIMEOUT` | `10s` | Graceful shutdown timeout. |

LLM API keys (e.g. `GEMINI_API_KEY`, `OPENAI_API_KEY`) are not ChatAPI config — they're arbitrary env var names you reference when registering a bot via `llm_api_key_env`.

---

## Authentication

ChatAPI uses JWT Bearer tokens. Your backend mints tokens signed with `JWT_SECRET`; ChatAPI validates the signature and reads the `sub` claim as the user ID.

The token needs a `sub` claim and must be signed with HS256:

```javascript
// Node.js example
import jwt from 'jsonwebtoken';

const token = jwt.sign(
  { sub: 'user_alice' },
  process.env.JWT_SECRET,
  { expiresIn: '24h' }
);
```

```go
// Go example
claims := jwt.MapClaims{
    "sub": "user_alice",
    "exp": time.Now().Add(24 * time.Hour).Unix(),
}
token, _ := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
    SignedString([]byte(os.Getenv("JWT_SECRET")))
```

Pass the token as a Bearer header on REST calls, or as a query param on WebSocket connections:

```
Authorization: Bearer <token>          # REST
ws://localhost:8080/ws?token=<token>   # WebSocket (browser)
```

---

## First API call

```bash
TOKEN="<your-signed-jwt>"

# Create a DM room between two users
curl -X POST http://localhost:8080/rooms \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"type": "dm", "members": ["alice", "bob"]}'
```

```json
{
  "room_id": "room_abc123",
  "type": "dm",
  "last_seq": 0,
  "created_at": "2026-04-12T10:00:00Z"
}
```

---

## First WebSocket connection

```javascript
// Browser
const ws = new WebSocket(`ws://localhost:8080/ws?token=${token}`);

ws.onopen = () => {
  ws.send(JSON.stringify({
    type: 'send_message',
    data: { room_id: 'room_abc123', content: 'Hello!' }
  }));
};

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  // { type: 'message', room_id, seq, message_id, sender_id, content, created_at }
};
```

```javascript
// Node.js / server
import WebSocket from 'ws';

const ws = new WebSocket('ws://localhost:8080/ws', {
  headers: { Authorization: `Bearer ${token}` }
});
```

---

## Next steps

- [REST API](/api/rest/) — Full endpoint reference
- [WebSocket API](/api/websocket/) — All events and client messages
- [AI Bots](/guides/bots/) — Register a bot and add it to a room

