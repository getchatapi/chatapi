---
title: "API Reference"
weight: 20
---

# API Reference

ChatAPI provides REST and WebSocket APIs for messaging and notifications. All endpoints require authentication and return JSON.

## Authentication

### Standard endpoints

```
X-API-Key: <your-tenant-api-key>
X-User-Id: <user-identifier>
```

- **X-API-Key** — Identifies your tenant. Keys are stored as SHA-256 hashes; the plaintext is returned only once at tenant creation.
- **X-User-Id** — Identifies the user making the request.

### Admin endpoints

```
X-Master-Key: <your-master-api-key>
```

Used only for `POST /admin/tenants`. Set via `MASTER_API_KEY` env var.

## REST API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/rooms` | List rooms the user belongs to |
| `POST` | `/rooms` | Create a room (DM, group, or channel) |
| `GET` | `/rooms/{room_id}` | Get room details |
| `GET` | `/rooms/{room_id}/members` | List room members |
| `POST` | `/rooms/{room_id}/messages` | Send a message |
| `GET` | `/rooms/{room_id}/messages` | Get messages (paginated) |
| `POST` | `/acks` | Acknowledge message delivery |
| `POST` | `/notify` | Send a notification to users or topic subscribers |
| `POST` | `/subscriptions` | Subscribe the authenticated user to a notification topic |
| `GET` | `/subscriptions` | List the user's notification subscriptions |
| `DELETE` | `/subscriptions/{id}` | Unsubscribe |
| `POST` | `/ws/token` | Issue a short-lived WebSocket token (browser clients) |
| `GET` | `/health` | Service health check |
| `GET` | `/metrics` | Live server counters |
| `POST` | `/admin/tenants` | Create a tenant (`X-Master-Key` required) |
| `GET` | `/admin/dead-letters` | View failed deliveries |

## WebSocket API

Connect to `/ws` with either:

- **Token** (browser): `GET /ws?token=<ws-token>` — obtain via `POST /ws/token` first
- **Header** (Node.js / server): pass `X-API-Key` + `X-User-Id` in the WebSocket upgrade request

### Client → Server events

| `type` | Description |
|--------|-------------|
| `send_message` | Send a message to a room |
| `ack` | Acknowledge messages up to a sequence number |
| `typing` | Broadcast typing start/stop |

### Server → Client events

| `type` | Description |
|--------|-------------|
| `message` | New message in a room |
| `ack.received` | Another user acknowledged messages |
| `typing` | Another user's typing status |
| `presence.update` | User came online or went offline |
| `notification` | Topic-based notification delivered to subscriber |
| `server.shutdown` | Server is restarting — reconnect after `reconnect_after_ms` ms |

## SDK

The official TypeScript SDK is available on npm:

```bash
npm install @hastenr/chatapi-sdk
```

```typescript
import { ChatAPI } from '@hastenr/chatapi-sdk';

const client = new ChatAPI({
  baseURL: 'https://your-chatapi.com',
  apiKey: 'your-api-key',
  userId: 'user123',
  displayName: 'Alice',
});

await client.connect();

// Messaging
client.on('message', (ev) => console.log(ev.content));
client.sendMessage('room_abc', 'Hello!');

// Notifications
await client.subscriptions.subscribe('order.updates');
client.on('notification', (ev) => {
  const payload = JSON.parse(ev.payload);
  console.log(payload.message);
});
```

See the [npm page](https://www.npmjs.com/package/@hastenr/chatapi-sdk) for the full SDK reference.

## Rate Limiting

- Default: 100 requests/second per tenant (configurable via `DEFAULT_RATE_LIMIT`)
- Exceeded requests return `429` with a `Retry-After: 60` header

## Reference

- [REST API Reference](/api/rest/) — Full endpoint documentation with examples
- [WebSocket API Reference](/api/websocket/) — Full event reference
- [API Playground](/api/playground/) — Interactive Swagger UI
