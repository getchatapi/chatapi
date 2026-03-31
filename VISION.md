# ChatAPI Vision

## What ChatAPI Is

ChatAPI is the **open-source, self-hosted chat backend for AI-powered apps**.

One Docker container. Your data, your server, your LLM. Free forever.

---

## The Problem We Solve

Every developer building an AI product needs the same infrastructure:

- A user types something → AI responds in real time
- Conversation history is persisted
- Other participants (human or AI) see it instantly
- User gets notified when a long-running AI task finishes

Right now developers either:
- Wire this from scratch (WebSockets + a database + retry logic)
- Use OpenAI Assistants API (vendor lock-in, no self-hosting, expensive)
- Use Vercel AI SDK (frontend only, no persistence, no multi-user)
- Use Stream/Sendbird (expensive, no AI-native features)

**Nobody owns the open-source, self-hosted chat backend for AI apps. That is the gap ChatAPI fills.**

---

## Who We Are For

Developers building AI products — copilots, assistants, agents, support bots — who want:

- Real-time chat between users and AI, out of the box
- Self-hosted, no vendor dependency
- Works with any LLM (OpenAI, Anthropic, Ollama, any OpenAI-compatible endpoint)
- Simple enough to integrate in an afternoon
- Free forever (no seat pricing, no message limits)

Secondary audience: marketplace and SaaS developers who need buyer-seller messaging or support chat (original use case, still valid).

---

## What Makes ChatAPI Different

| | ChatAPI | OpenAI Assistants | Stream / Sendbird | Roll your own |
|---|---|---|---|---|
| Self-hosted | Yes | No | No | Yes |
| AI-native (streaming, bots) | Yes | Yes | No | You build it |
| Open source | Yes | No | No | N/A |
| Free | Yes | Pay per token | Expensive | Infrastructure cost |
| Works with any LLM | Yes | No | No | You build it |
| Production-ready chat infra | Yes | No | Yes | You build it |

---

## The Bot Model

ChatAPI does not try to be an AI framework. LangChain, CrewAI, Claude, AutoGen — they already do agents better than a chat server ever could. We do not compete with them.

Instead, ChatAPI is **what agents use to talk to humans**. The agent lives outside. ChatAPI is the real-time messaging layer between the agent and the user.

```
Your AI Agent (LangChain / CrewAI / Claude / custom)
        ↕  REST API
     ChatAPI room
        ↕  WebSocket
      End user
```

A bot is just a room participant that happens to be software. There are two ways to be a bot:

**Webhook-driven bot** (bring your own agent) — ChatAPI calls your endpoint when a message arrives. Your agent processes it with full agentic capability — tools, planning, memory, multi-step reasoning — and posts the reply back via `POST /rooms/{id}/messages`. No constraints on how your AI works.

**Built-in LLM bot** (batteries included) — register a bot with a model config, attach it to a room, done. ChatAPI handles the LLM call, context injection, and reply. Good for simple Q&A, support FAQ, basic assistants. No code required.

This distinction matters: we are not building another chatbot. We are building the communication infrastructure that any AI agent can plug into.

---

## Core Product Pillars

### 1. AI Bot Participants
First-class bot users in any room. Two modes: webhook-driven (your agent) or built-in LLM (no code).

- `POST /bots` — register a bot (webhook URL, or model config for built-in)
- Attach to any room at creation or after
- Webhook bots: ChatAPI POSTs inbound messages to your agent endpoint; agent replies via REST
- Built-in bots: ChatAPI calls the LLM with room history, posts reply automatically
- Works with OpenAI, Anthropic, Ollama, any OpenAI-compatible endpoint

### 2. Streaming Responses
AI responses stream token by token over the existing WebSocket connection.

- `message.stream.start` — new AI response begins
- `message.stream.delta` — token arrives
- `message.stream.end` — response complete, message persisted

No polling. No waiting for the full response before rendering.

### 3. Thread Context
When a bot is triggered, ChatAPI automatically passes room history as LLM context. The developer does not wire this — it just works.

Configurable: how many messages of history to include, system prompt per bot, per room overrides.

### 4. JS Client Widget
A drop-in widget that looks and feels like a modern AI chat interface.

- Streaming cursor, typing indicator, message history
- `<ChatAPI room="..." bot="..." />` — that's the integration
- Under 10kb, no framework dependency
- Backed by ChatAPI, talking to whatever LLM the developer configured

---

## What We Do NOT Build

- Horizontal scaling / Redis pub-sub / multi-instance WebSocket — this leads to becoming RocketChat. Single-process SQLite is a deliberate trade for simplicity.
- A hosted SaaS version — the self-hosted promise is the product.
- A visual chat builder / no-code tool — we are a developer product.
- Competing with Slack / Discord — we are infrastructure, not an end-user product.
- An AI framework or agent runtime — LangChain, CrewAI, Claude already do this. We are the messaging layer, not the brain.

---

## Roadmap

### Phase 1 — AI-native core
- [ ] Bot participant registration (`POST /bots`, attach to rooms)
- [ ] LLM call on inbound message (OpenAI-compatible + Anthropic)
- [ ] Streaming WebSocket events (`message.stream.*`)
- [ ] Thread context injection (auto room history in LLM prompt)

### Phase 2 — Developer experience
- [ ] JS client widget with streaming UI
- [ ] Quickstart: zero to working AI chat in 10 minutes
- [ ] Example integration repo (Next.js)
- [ ] Hosted clickable demo

### Phase 3 — Power features
- [ ] RAG via retrieval endpoint (bot config: `retrieval_url`, ChatAPI calls it, injects context)
- [ ] Human-in-the-loop handoff (bot escalates to human participant in same room)
- [ ] Per-room bot config overrides (system prompt, model, temperature)
- [ ] Built-in knowledge base upload (sqlite-vec, keeps single-container promise)

---

## Competitive Positioning

We are not competing with Slack. We are not competing with OpenAI.

We are the **infrastructure layer** for developers who are already using an LLM and need the chat layer around it — real-time delivery, history, notifications, multi-user — without building it from scratch or paying Stream/Sendbird prices.

The pitch: **"Your AI app needs a chat backend. This is it."**
