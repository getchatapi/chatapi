# Checkpoint — Vision

> **The open-source human-agent communication layer. Single binary. Zero setup. Any framework.**

---

## The Gap in the Agentic Stack

Two protocols are becoming the backbone of agentic AI:

| Protocol | Solves |
|---|---|
| MCP (Anthropic) | Agent ↔ Tools / Data |
| A2A (Google + Linux Foundation) | Agent ↔ Agent |
| **Checkpoint** | **Agent ↔ Human** |

A2A and MCP are reaching critical mass. The human side has no standard. Every team builds it from scratch. That is the gap Checkpoint fills.

---

## The Problem

Agents are running autonomously — writing code, browsing the web, managing files, calling APIs, making decisions. But agents still need humans. For approval. For escalation. For oversight. For context only a human has.

The way this is solved today:

- LangGraph pauses graph execution — but notifying the human is your problem
- CrewAI has `human_input=True` — but delivering that to a real user is your problem
- OpenAI Assistants surfaces `requires_action` — but reaching the human is your problem
- Claude stops and asks — but only if a human is sitting at the terminal

**The "reach the human" part is always left to the developer.** No standard. No protocol. No infrastructure.

The result: every agentic system reinvents the same plumbing — webhooks, polling loops, Slack bots, email notifications, custom dashboards — just to get a yes/no from a human.

Checkpoint solves this once.

---

## What Checkpoint Is

A lightweight, self-hosted service that runs alongside your agent infrastructure. Agents send messages to it. Humans receive them in real time. Humans respond. Agents continue.

```
[ LangGraph agent ]  →  POST /rooms/{id}/messages
[ CrewAI agent    ]  →  POST /rooms/{id}/messages
[ Any MCP agent   ]  →  MCP tool: send_human_message()
          ↓
      [ Checkpoint ]
          ↓
  [ Human — dashboard / mobile / any WebSocket client ]
```

One binary. One file on disk. Any agent framework. Full audit trail. Free forever.

---

## Core Scenarios

### Approval workflows
Agent is about to take a risky or irreversible action. It sends a structured approval request. Human sees it instantly, approves or rejects inline. Agent receives the decision via webhook and continues or aborts.

```
agent  →  { type: "approval_request", action: "deploy to prod", context: "..." }
human  →  clicks Approve in dashboard
agent  ←  webhook: { approved: true, by: "pascal", at: "2026-03-30T10:42:00Z" }
```

### Escalation
Agent hits an ambiguous requirement or unexpected state. It escalates to a human. Human provides guidance. Agent continues with full context. Every exchange persisted.

### Human-on-the-loop monitoring
Long-running agent posts status updates as it works. Human team watches a room — can intervene, redirect, or ask questions mid-task without blocking agent progress.

### Multi-agent + human oversight
Multiple agents coordinating on a task. Humans observe the inter-agent conversation and can jump in at any point. Full replay of everything that happened.

### Async task completion
Agent finishes a long job. Sends a completion notification. Human gets push notification, comes back to review results, ask follow-up questions, or kick off the next task.

---

## The MCP Wedge

Checkpoint exposes itself as an MCP server. Any MCP-compatible agent — Claude, GPT-4, LangChain, CrewAI, LangGraph — gets human communication for free, with zero custom integration:

```
# Any MCP-compatible agent, out of the box:
send_human_message(room="deploy-approvals", content="Ready to deploy v2.3 to prod. Confirm?")
# → human receives instantly via WebSocket
# → human replies "go ahead"
# → agent gets the response and continues
```

This is the wedge into the entire agentic ecosystem. Every framework that supports MCP gets human-agent communication for free, from day one.

---

## The Dashboard — Making the Invisible Visible

Traefik became the default reverse proxy not just because it works, but because its dashboard made developers trust it. You could see everything — routers, services, traffic — in real time. That visibility is a feature.

Checkpoint has the same opportunity. The dashboard is mission control for your agent team.

**Agents panel** — every registered agent, live status: `idle` / `working` / `waiting for human`. Current task. How long it has been running.

**Pending approvals** — the most critical panel. All approval requests across all agents, waiting for a human. Agent name, what it wants to do, context, time waiting. Approve or reject inline without leaving the dashboard.

**Active rooms** — all conversations in progress. Participants (human avatars + agent icons), last message, unread count.

**Message flow graph** — the Traefik moment. A live graph showing agents → Checkpoint → humans. Messages flowing as edges. Rooms as nodes. The entire communication topology of your agentic system at a glance.

**Audit trail** — every agent message, every human decision, timestamped and searchable. Replay any conversation. Answer "what did agent X do yesterday?" with a filter.

| Traefik | Checkpoint |
|---|---|
| Routers, services, middlewares | Agents, rooms, message types |
| Traffic flowing through routes | Messages flowing between agents and humans |
| Health status per service | Agent status (idle / working / waiting) |
| Real-time, auto-updates | Real-time WebSocket push |

The dashboard directly answers the #1 reason teams fear deploying autonomous agents: they cannot see what the agents are doing.

---

## Technical Philosophy — Extreme Simplicity

Checkpoint is designed to be the simplest possible thing that works correctly. Every decision is made in favour of simplicity over features.

### Single binary
Download it. Run it. No installer, no config files, no runtime dependencies.

```bash
./checkpoint
# running on :8080
```

### Zero CGO — pure Go
No C dependencies. The binary compiles anywhere. Small Docker images. No toolchain headaches.

### bbolt for storage
Not SQLite. Not PostgreSQL. [bbolt](https://github.com/etcd-io/bbolt) — a pure Go embedded key-value store. One `.db` file on disk. Handles concurrent writes correctly. Battle-tested (powers etcd). Append-heavy workloads like message logs are its sweet spot.

### No multi-tenancy
Checkpoint is a single-team deployment. You run one instance alongside your agent infrastructure. No tenant isolation, no API key management, no per-tenant schemas. Your agents, your humans, your data.

### Dashboard embedded in the binary
The UI is compiled into the binary via Go's `embed`. No static file server, no CDN, no separate frontend build step.

### Estimated binary size: ~9MB
| Component | Size |
|---|---|
| Go runtime + HTTP/WS | ~8MB |
| bbolt | ~500KB |
| Dashboard (embedded) | ~200KB |
| **Total** | **~9MB** |

### Embeddable as a Go library
Run it standalone as a sidecar, or embed it directly in your own Go service — no separate process needed.

```go
import "github.com/you/checkpoint"

cp := checkpoint.New(checkpoint.Config{
    Port:        8080,
    StoragePath: "./checkpoint.db",
})
cp.Start()
```

---

## What Gets Added to the Core

The existing ChatAPI foundation — rooms, participants, WebSocket, webhooks — maps directly to this use case. Key additions:

**Structured message types** — beyond plain text:
- `approval_request` — action, context, options (approve/reject/defer)
- `status_update` — agent ID, current task, progress
- `escalation` — reason, context, what the agent needs
- `task_complete` — results, next steps

**Message actions** — approve/reject/defer built into the protocol. Agents receive a structured decision, not a string to parse.

**Agent identity** — agents as first-class participants with metadata: capabilities, current task, framework, status.

**MCP server** — built-in. Any MCP-compatible agent communicates with humans without a custom SDK.

**A2A compatibility** — rooms addressable via A2A so frameworks already using it work natively.

---

## What We Do NOT Build

- An agent framework or orchestrator — LangChain, CrewAI, LangGraph do this
- An observability/tracing platform — LangSmith, AgentOps do this (read-only; Checkpoint is interactive)
- A2A or MCP replacements — complementary, not competing
- Horizontal scaling / multi-instance — single-process, single-file is deliberate
- A hosted SaaS — self-hosted is the product
- Multi-tenancy — one deployment, one team

---

## Why This Wins on Adoption

**Unclaimed territory.** No competitor owns agent↔human communication. First mover with a good open-source release wins the category.

**Acute pain.** Developers building agentic systems are hitting this problem right now. The moment they find Checkpoint it solves something they've been hacking around.

**MCP means zero integration cost.** If you already use an MCP-compatible agent framework, Checkpoint just works. No SDK to learn.

**Self-hosted is a requirement, not a nice-to-have.** Agents touch sensitive systems. Approval workflows and audit trails cannot live in a third-party SaaS.

**The dashboard builds trust.** Showing a team what their agents are doing in real time is the thing that makes autonomous agents safe to deploy. That's the GitHub star moment.

---

## Roadmap

### Phase 1 — Core
- [ ] Replace SQLite with bbolt, remove multi-tenancy
- [ ] Structured message types (approval_request, status_update, escalation, task_complete)
- [ ] Message actions (approve/reject/defer) in WebSocket protocol
- [ ] Agent identity (first-class agent participants with metadata)
- [ ] Webhook delivery of human decisions back to agents
- [ ] Single binary build, embedded dashboard scaffold

### Phase 2 — Ecosystem
- [ ] MCP server (any MCP-compatible agent gets human comms for free)
- [ ] A2A protocol compatibility
- [ ] LangGraph integration example
- [ ] CrewAI integration example
- [ ] Go library interface (embeddable)

### Phase 3 — Governance
- [ ] Message flow graph in dashboard (live topology view)
- [ ] Audit trail search and replay
- [ ] Approval policies (who can approve what, escalation rules)
- [ ] Agent activity and approval metrics

---

## The Pitch

> "Your agents can act autonomously. But some decisions still need a human. Checkpoint is how your agents reach people — in real time, with full audit trail, on any device. One binary. Zero setup. Any framework."

---

## Origin

Built from ChatAPI — a production-grade real-time messaging service. The rooms, WebSocket, webhooks, and delivery infrastructure carry over directly. Checkpoint is ChatAPI repointed at the most urgent unsolved problem in agentic AI.
