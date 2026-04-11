---
title: "AI Bots"
weight: 11
---

# AI Bots

A bot in ChatAPI is an AI participant registered with an LLM provider. When a user sends a message in a room the bot belongs to, ChatAPI calls the LLM, streams the response back via `message.stream.*` events, and stores the final message. No agent process required.

---

## Setup

### 1. Set the API key on the server

The key lives in an environment variable — never in the database:

```env
GEMINI_API_KEY=AIza...
```

ChatAPI supports any provider with an OpenAI-compatible `/chat/completions` endpoint:

| Provider | `llm_base_url` |
|---|---|
| Gemini | `https://generativelanguage.googleapis.com/v1beta/openai/` |
| OpenAI | `https://api.openai.com/v1` |
| Ollama (local) | `http://localhost:11434/v1` |
| OpenRouter | `https://openrouter.ai/api/v1` |

### 2. Register the bot

```bash
curl -X POST http://localhost:8080/bots \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Support Bot",
    "llm_base_url": "https://generativelanguage.googleapis.com/v1beta/openai/",
    "llm_api_key_env": "GEMINI_API_KEY",
    "model": "gemini-2.0-flash",
    "system_prompt_webhook": "https://yourapp.com/api/chatapi/system-prompt"
  }'
```

| Field | Required | Description |
|---|---|---|
| `name` | Yes | Display name |
| `llm_base_url` | Yes | OpenAI-compatible base URL |
| `llm_api_key_env` | Yes | Name of the env var holding the API key |
| `model` | Yes | Model identifier (e.g. `gemini-2.0-flash`, `gpt-4o`) |
| `system_prompt_webhook` | No | URL called before every LLM request — returns the system prompt |

### 3. Add the bot to a room

```bash
curl -X POST http://localhost:8080/rooms/room_abc123/members \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"user_id": "bot_abc123"}'
```

The bot now responds to every message sent in that room.

---

## System prompt webhook

The webhook is how your application injects context — RAG results, customer data, prompt instructions — into the LLM call. It is called **before every LLM request**, so the system prompt can be dynamic per message.

### Request

ChatAPI sends a `POST` to `system_prompt_webhook` with:

```json
{
  "bot_id": "bot_abc123",
  "room_id": "room_abc123",
  "message": {
    "message_id": "msg_def456",
    "sender_id": "alice",
    "content": "What is the refund policy?",
    "created_at": "2026-04-11T10:00:00Z"
  },
  "history": [
    { "role": "user",      "content": "Hi there" },
    { "role": "assistant", "content": "Hello! How can I help?" },
    { "role": "user",      "content": "What is the refund policy?" }
  ]
}
```

`history` contains up to 20 of the most recent messages in the room, formatted as OpenAI role/content pairs. The last entry is always the message that triggered the bot.

### Response

Your webhook must return:

```json
{
  "system_prompt": "You are a support agent for Acme Corp.\n\nRefund policy: ..."
}
```

ChatAPI uses `system_prompt` as the `system` message at the top of the LLM messages array.

### Example — Next.js API route

```typescript
// app/api/chatapi/system-prompt/route.ts
export async function POST(req: Request) {
  const { message, history } = await req.json();

  const docs = await vectorSearch(message.content);
  const customer = await db.customers.findByUserId(message.sender_id);

  return Response.json({
    system_prompt: `You are a support agent for Acme Corp.
Tone: professional, concise.
Customer: ${customer.name} (plan: ${customer.plan})

Relevant knowledge base:
${docs.map(d => d.content).join('\n\n')}`,
  });
}
```

Your RAG pipeline, prompt engineering, and personalisation all live here. ChatAPI is the transport layer — your app is the brain.

---

## Streaming events

When a bot responds, clients receive:

| Event | Description |
|---|---|
| `message.stream.start` | Bot has started responding |
| `message.stream.delta` | One token chunk (repeats until done) |
| `message.stream.end` | Stream complete — message persisted with `seq` |
| `message.stream.error` | LLM call failed — discard any partial content |

See [WebSocket API](/api/websocket/) for the full event schema.

---

## Manage bots

```bash
# List all bots
curl http://localhost:8080/bots -H "Authorization: Bearer $TOKEN"

# Get a specific bot
curl http://localhost:8080/bots/bot_abc123 -H "Authorization: Bearer $TOKEN"

# Delete a bot
curl -X DELETE http://localhost:8080/bots/bot_abc123 -H "Authorization: Bearer $TOKEN"
```

---

## Next steps

- [WebSocket API](/api/websocket/) — Full event reference including stream events
- [REST API](/api/rest/) — Bot and room endpoint reference
- [TypeScript SDK](/sdk/) — `chat.bots.create(...)` and streaming event handlers
