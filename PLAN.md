# ChatAPI — Implementation Plan

## What ChatAPI Is

**The real-time communication layer between your AI and your humans.**

Self-hosted, open-source, single binary. Whether the human is a customer chatting with an AI copilot, an operator approving an agent action, or a support agent taking over from a bot — ChatAPI is the infrastructure underneath it all.

---

## Architectural Decisions

### Drop Multi-Tenancy

Multi-tenancy was designed for a hosted SaaS model. For a self-hosted product the deployer is the only tenant. Removing it eliminates:
- API keys in client code (security risk)
- Tenant isolation complexity
- A concept that confuses the mental model

**Replacement:** JWT-based auth. Your app authenticates your users and issues short-lived tokens. ChatAPI validates the JWT. Your server-side secret never touches the client. This is how Pusher and Ably work.

```
Your backend  →  signs JWT for authenticated user
User          →  connects to ChatAPI with JWT
ChatAPI       →  validates JWT (configurable secret or JWKS URL)
```

### SQLite — Intentionally

Single-process SQLite is a deliberate trade for simplicity. It means:
- Zero infrastructure dependencies
- Single binary deployment
- Trivial backup (copy a file)
- No connection pooling daemon

Horizontal scaling is a non-goal. If you need it, you've outgrown this tool.

### No Webhooks

We do not use webhooks. Webhooks require the bot/agent to run an HTTP server, add retry/signature logic on both sides, and create tight coupling between ChatAPI and your agent's deployment. The attack surface and operational overhead are not worth it.

**Replacement:** MCP-first. Agents connect to ChatAPI as an MCP client and call tools proactively. The agent controls the loop — it decides when to listen, when to act, and when to wait. No server required on the agent side.

For cases where the agent needs to pause and wait for a human reply, `await_response` is the primitive: a single blocking MCP tool call that returns when the human responds or when the timeout expires.

### MCP Server Built-In

ChatAPI ships with an MCP (Model Context Protocol) server. Any MCP-compatible agent — Claude, Cursor, or any agent built on the protocol — can use ChatAPI with zero integration code. The agent discovers tools automatically:

```
send_message(room_id, content)
get_messages(room_id, limit, after_seq)
create_room(name, members)
is_user_online(user_id)
request_approval(room_id, action, context)
await_response(room_id, timeout_seconds) → {reply, user_id, seq}
```

`await_response` is the core oversight primitive. It lets an agent pause mid-workflow, show context to a human, and block until the human responds. The agent then decides how to proceed based on the reply. No polling, no callback URL, no webhook infrastructure.

This is the distribution strategy. Every developer dropping in an MCP client is a potential user.

---

## The Three Pillars

### 1. Real-Time Rooms
Rooms with any mix of human and AI participants. WebSocket delivery, persistent history, delivery guarantees, presence tracking. **Already built.**

### 2. Bot Participants
AI agents as first-class room members. Bots are added to rooms like any user. They connect via MCP or via the REST API using a JWT issued for their bot_id.

**LLM-backed bots** — register a bot with a model config (provider, model, API key, system prompt). ChatAPI handles the LLM call, injects room history as context, streams the reply. Zero code required for simple assistants.

**Custom agents** — any agent connecting via MCP or REST, making its own decisions. The bot_id is just a user_id that happens to be a machine.

### 3. Oversight Primitives
Structured message types that make AI agents safe to deploy in production:
- Approval requests — agent asks a human before acting
- Escalation — bot hands off to a human participant
- Acknowledgement — human confirms they've seen something
- Audit trail — every agent decision and human response is logged and replayable

The key primitive is `await_response`: agent sends a message, calls `await_response(room_id, timeout)`, and gets back the human's reply. The agent's goroutine/thread sleeps — no webhooks, no polling.

---

## Roadmap

### Phase 0 — Foundation Cleanup ✓
- [x] Rooms, messages, WebSocket delivery
- [x] Presence tracking
- [x] Delivery retry worker
- [x] Test coverage across all services and handlers
- [x] Remove multi-tenancy, replace with JWT auth
- [x] Remove API keys from all flows
- [x] Remove webhook config and offline notification webhooks

### Phase 1 — Bot Participants
- [ ] `POST /bots` — register a bot (LLM config or external agent)
- [ ] `POST /rooms/{id}/members` — add bot as participant (bot_id as user_id)
- [ ] Trigger built-in LLM bots on inbound message (goroutine, non-blocking)
- [ ] Built-in LLM bots: OpenAI-compatible provider (covers OpenAI, Ollama, Groq, local models)
- [ ] Built-in LLM bots: Anthropic provider
- [ ] Streaming WebSocket events: `message.stream.start`, `message.stream.delta`, `message.stream.end`
- [ ] Thread context injection: room history passed as LLM context automatically

### Phase 2 — MCP Server
- [ ] MCP server built into the binary (listens on separate port or same)
- [ ] Tools: `send_message`, `get_messages`, `create_room`, `is_user_online`
- [ ] Tools: `request_approval`, `await_response` (blocking until human replies or timeout)
- [ ] MCP auth: same JWT validation as REST
- [ ] Documentation: how to connect Claude Desktop, Cursor, custom agents

### Phase 3 — Oversight Primitives
- [ ] Structured message types: approval request, approval response, escalation, ack
- [ ] Room state: pending / active / resolved
- [ ] `await_response` blocks the MCP call until human replies or timeout — no webhook needed
- [ ] Audit log endpoint: full trace of agent decisions and human responses

### Phase 4 — Developer Experience
- [ ] JS client widget: streaming cursor, typing indicator, approval UI
- [ ] Quickstart: zero to working AI chat in 10 minutes
- [ ] Example repo: Next.js app with ChatAPI + Claude
- [ ] Example: autonomous agent with human-in-the-loop approval via MCP

---

## Data Model Changes

### Remove
```sql
DROP TABLE tenants;
-- tenant_id columns kept as single-value ("default") for forward compat — no migration needed
```

### Add
```sql
CREATE TABLE bots (
    bot_id        TEXT PRIMARY KEY,
    name          TEXT NOT NULL,
    mode          TEXT NOT NULL,        -- 'llm' | 'external'
    -- llm config
    provider      TEXT,                 -- 'openai' | 'anthropic'
    base_url      TEXT,                 -- for ollama / openai-compatible endpoints
    model         TEXT,
    api_key       TEXT,
    system_prompt TEXT,
    max_context   INT DEFAULT 20,
    created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
);
```

---

## Config

```env
LISTEN_ADDR=:8080
JWT_SECRET=your-secret-here
ALLOWED_ORIGINS=https://yourapp.com
DB_PATH=./chatapi.db
```

No `WEBHOOK_URL`, no `WEBHOOK_SECRET`. Agents pull; ChatAPI does not push to external services.

---

## What We Don't Build

- Horizontal scaling / Redis pub-sub / multi-instance WebSocket
- A hosted SaaS version (open source only)
- A visual chat builder or no-code tool
- An agent framework or LLM orchestration layer
- Multi-tenancy (single workspace per deployment)
- Webhook endpoints for bots or offline delivery
- Competing with Slack, Discord, or any end-user chat product
