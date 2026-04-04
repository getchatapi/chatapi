---
title: "REST API"
weight: 21
---

# REST API Reference

## Authentication

All endpoints except `GET /health` and `GET /metrics` require a JWT Bearer token:

```
Authorization: Bearer <jwt>
```

Your backend signs JWTs with `JWT_SECRET`. The `sub` claim is the user ID for the request.

Error responses use a flat JSON shape:

```json
{"error": "not_found", "message": "room not found"}
```

Common status codes: `400` invalid request, `401` missing/invalid token, `403` forbidden, `404` not found, `500` server error.

---

## Health & Metrics

### Health check

```http
GET /health
```

No authentication required. Returns `503` if the database is not writable.

```json
{"status": "ok", "db_writable": true}
```

### Server metrics

```http
GET /metrics
```

No authentication required. Live counters since server start.

```json
{
  "active_connections": 42,
  "messages_sent": 18340,
  "broadcast_drops": 3,
  "delivery_attempts": 21005,
  "delivery_failures": 12,
  "uptime_seconds": 86400
}
```

---

## Rooms

### List rooms

Returns all rooms the authenticated user belongs to.

```http
GET /rooms
Authorization: Bearer <token>
```

```json
{
  "rooms": [
    {
      "room_id": "room_abc123",
      "type": "dm",
      "unique_key": "dm:alice:bob",
      "name": null,
      "metadata": "{\"listing_id\":\"lst_99\"}",
      "last_seq": 42,
      "created_at": "2026-04-02T12:00:00Z"
    }
  ]
}
```

### Create room

```http
POST /rooms
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{
  "type": "dm",
  "members": ["alice", "bob"],
  "name": "Optional name",
  "metadata": "{\"order_id\":\"ord_42\"}"
}
```

- `type`: `"dm"` | `"group"` | `"channel"` — required
- `members`: array of user IDs — required
- `name`: optional display name
- `metadata`: optional arbitrary JSON string for app-level context (listing IDs, order IDs, etc.)

Returns `201` with the created room object. Returns `409` if a DM between those members already exists.

### Get room

```http
GET /rooms/{room_id}
Authorization: Bearer <token>
```

Returns the room object. `403` if the user is not a member.

### Update room

```http
PUT /rooms/{room_id}
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{
  "name": "New name",
  "metadata": "{\"order_id\":\"ord_99\"}"
}
```

Both fields are optional. Omitting a field leaves it unchanged. Returns the updated room object.

### Delete room

```http
DELETE /rooms/{room_id}
Authorization: Bearer <token>
```

Returns `204 No Content`.

---

## Members

### List members

```http
GET /rooms/{room_id}/members
Authorization: Bearer <token>
```

```json
{
  "members": [
    {"user_id": "alice", "role": "member", "joined_at": "2026-04-02T12:00:00Z"},
    {"user_id": "bob",   "role": "member", "joined_at": "2026-04-02T12:00:00Z"}
  ]
}
```

### Add member

```http
POST /rooms/{room_id}/members
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{"user_id": "charlie"}
```

Returns `201` on success.

### Remove member

```http
DELETE /rooms/{room_id}/members/{user_id}
Authorization: Bearer <token>
```

Returns `204 No Content`.

---

## Messages

### Get messages

```http
GET /rooms/{room_id}/messages?after_seq=40&limit=50
Authorization: Bearer <token>
```

Query parameters:
- `after_seq` — return messages with `seq > after_seq` (optional, default 0)
- `limit` — max messages to return (optional, default 50, max 100)

```json
{
  "messages": [
    {
      "message_id": "msg_abc123",
      "sender_id": "alice",
      "seq": 41,
      "content": "Hello!",
      "meta": null,
      "created_at": "2026-04-02T12:10:00Z"
    }
  ]
}
```

### Send message

```http
POST /rooms/{room_id}/messages
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{
  "content": "Hello!",
  "meta": "{\"mentions\":[\"bob\"]}"
}
```

- `content`: message text — required
- `meta`: optional arbitrary JSON string (client-defined metadata, mentions, reactions, etc.)

```json
{"message_id": "msg_abc123", "seq": 43, "created_at": "2026-04-02T12:10:00Z"}
```

### Edit message

Only the original sender may edit a message.

```http
PUT /rooms/{room_id}/messages/{message_id}
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{"content": "Updated content"}
```

Returns the updated message object. `403` if not the sender.

### Delete message

Only the original sender may delete a message.

```http
DELETE /rooms/{room_id}/messages/{message_id}
Authorization: Bearer <token>
```

Returns `204 No Content`. `403` if not the sender.

---

## Acknowledgments

Mark messages as delivered up to a given sequence number.

```http
POST /acks
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{"room_id": "room_abc123", "seq": 43}
```

Returns `200 OK`.

---

## Notifications

### Send notification

```http
POST /notify
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{
  "topic": "order.shipped",
  "payload": {"order_id": "12345", "tracking": "1Z999AA..."},
  "targets": {
    "user_ids": ["alice", "bob"],
    "room_id": "room_abc123",
    "topic_subscribers": true
  }
}
```

`targets` controls who receives the notification. All target fields are optional and can be combined. If `targets` is omitted, the notification is broadcast to all online users.

```json
{"notification_id": "notif_abc123", "created_at": "2026-04-02T12:15:00Z"}
```

---

## Notification Subscriptions

### Subscribe to a topic

```http
POST /subscriptions
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{"topic": "order.shipped"}
```

Returns `201`:

```json
{
  "id": 1,
  "subscriber_id": "alice",
  "topic": "order.shipped",
  "created_at": "2026-04-02T12:00:00Z"
}
```

### List subscriptions

```http
GET /subscriptions
Authorization: Bearer <token>
```

```json
{
  "subscriptions": [
    {"id": 1, "subscriber_id": "alice", "topic": "order.shipped", "created_at": "2026-04-02T12:00:00Z"}
  ]
}
```

### Unsubscribe

```http
DELETE /subscriptions/{id}
Authorization: Bearer <token>
```

Returns `204 No Content`.

---

## Bots

### Register a bot

```http
POST /bots
Authorization: Bearer <token>
Content-Type: application/json
```

```json
{
  "name": "Support Bot",
  "mode": "llm",
  "provider": "openai",
  "base_url": "https://api.openai.com/v1",
  "model": "gpt-4o",
  "api_key": "sk-...",
  "system_prompt": "You are a helpful support agent.",
  "max_context": 20
}
```

Fields:
- `name`: display name — required
- `mode`: `"llm"` (ChatAPI calls the LLM) or `"external"` (bot connects via JWT like any user) — required
- `provider`: `"openai"` or `"anthropic"` — required for `llm` mode
- `base_url`: optional override for OpenAI-compatible endpoints (Ollama, Groq, LM Studio, etc.)
- `model`: model ID (e.g. `"gpt-4o"`, `"claude-3-5-sonnet-20241022"`)
- `api_key`: LLM provider API key
- `system_prompt`: system message prepended to every LLM request
- `max_context`: number of recent messages to include as context (default 20)

Returns `201` with the created bot object (without `api_key`).

### List bots

```http
GET /bots
Authorization: Bearer <token>
```

### Get bot

```http
GET /bots/{bot_id}
Authorization: Bearer <token>
```

### Delete bot

```http
DELETE /bots/{bot_id}
Authorization: Bearer <token>
```

Returns `204 No Content`.

---

## Content types and formats

- All request bodies: `application/json` (UTF-8)
- All timestamps: ISO 8601 (`2026-04-02T12:00:00Z`)
