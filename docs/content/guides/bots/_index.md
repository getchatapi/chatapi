---
title: "AI Bots"
weight: 11
---

# AI Bots

A bot in ChatAPI is a named identity that connects to rooms just like any other user — via a JWT signed with your `JWT_SECRET`. You register the bot to get a stable `bot_id`, then your agent process authenticates as that ID.

ChatAPI handles the messaging layer. All LLM logic — which model to call, what context to include, how to structure the response — lives in your agent.

## Register a bot

```bash
TOKEN="<your-signed-jwt>"

curl -X POST http://localhost:8080/bots \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "Support Bot"}'
```

Response:

```json
{
  "bot_id": "bot_abc123",
  "name": "Support Bot",
  "created_at": "2026-04-02T12:00:00Z"
}
```

## Add the bot to a room

```bash
curl -X POST http://localhost:8080/rooms/room_abc123/members \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"user_id": "bot_abc123"}'
```

## Connect your agent

Your agent process mints a JWT with `sub` set to the `bot_id`, then connects over WebSocket or REST like any other user:

```javascript
const botJWT = signJWT({ sub: "bot_abc123" }, process.env.JWT_SECRET);
const ws = new WebSocket(`wss://your-chatapi.com/ws?token=${botJWT}`);

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);

  // Only respond to real user messages
  if (msg.type !== "message" || msg.sender_id === "bot_abc123") return;

  // Call your LLM, stream tokens, reply
  const reply = await callYourLLM(msg.content);

  ws.send(JSON.stringify({
    type: "send_message",
    data: { room_id: msg.room_id, content: reply },
  }));
};
```

Your agent can use any LLM or framework — LangChain, the Anthropic SDK, raw OpenAI calls, a local model. ChatAPI doesn't care.

## Streaming responses

If your agent streams tokens, broadcast them via WebSocket so clients can display the response incrementally:

```javascript
ws.send(JSON.stringify({
  type: "send_message",
  data: { room_id: msg.room_id, content: "message.stream.start" },
}));

for await (const token of llmStream) {
  ws.send(JSON.stringify({
    type: "send_message",
    data: { room_id: msg.room_id, content: token },
  }));
}
```

Connected clients receive `message.stream.start` → `message.stream.delta` → `message.stream.end` events and can render the response token by token. See [WebSocket API](/api/websocket/) for the full event schema.

## Manage bots

```bash
# List all bots
curl http://localhost:8080/bots \
  -H "Authorization: Bearer $TOKEN"

# Get a specific bot
curl http://localhost:8080/bots/bot_abc123 \
  -H "Authorization: Bearer $TOKEN"

# Delete a bot
curl -X DELETE http://localhost:8080/bots/bot_abc123 \
  -H "Authorization: Bearer $TOKEN"
```

## Next steps

- [WebSocket API](/api/websocket/) — Full event reference
- [REST API](/api/rest/) — Bot and room endpoint reference
