---
title: "AI Bots"
weight: 11
---

# AI Bots

A bot in ChatAPI is a named identity with a stable `bot_id`. It can operate in two modes:

| Mode | How it works |
|---|---|
| **Managed** | Set `llm_base_url` when registering. ChatAPI calls the LLM, streams the response, and stores the message. No agent process required. |
| **External** | No LLM config. Your agent process connects via JWT and handles everything itself. |

Both modes use the same room membership and streaming event protocol.

---

## Managed bots

### 1. Set the API key on the server

The key lives in an environment variable on the ChatAPI server — never in the database:

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

Fields:

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

That's it. The bot now responds to every message sent in the room.

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
  const { bot_id, room_id, message, history } = await req.json();

  // Retrieve context relevant to the user's message
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

Your RAG pipeline, prompt engineering, and personalisation all live here. Swap your vector store or model without touching ChatAPI.

---

## Streaming events

When a managed bot responds, clients receive:

```
message.stream.start  → bot has started responding
message.stream.delta  → one token chunk (repeat until done)
message.stream.end    → stream complete, message persisted with seq
message.stream.error  → LLM call failed (discard any partial content)
```

See [WebSocket API](/api/websocket/) for the full event schema.

---

## External bots

If you want full control — your own agent process, custom retry logic, multi-step reasoning, tool calls — register a bot without LLM config:

```bash
curl -X POST http://localhost:8080/bots \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name": "My Agent"}'
# {"bot_id": "bot_abc123", ...}
```

Your agent mints a JWT with `sub: "bot_abc123"` and connects to ChatAPI over WebSocket:

```javascript
const botJWT = signJWT({ sub: 'bot_abc123' }, process.env.JWT_SECRET);
const ws = new WebSocket(`wss://your-chatapi.com/ws?token=${botJWT}`);

ws.onmessage = async (event) => {
  const msg = JSON.parse(event.data);
  if (msg.type !== 'message' || msg.sender_id === 'bot_abc123') return;

  const reply = await callYourLLMPipeline(msg.content);

  ws.send(JSON.stringify({
    type: 'send_message',
    data: { room_id: msg.room_id, content: reply },
  }));
};
```

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
