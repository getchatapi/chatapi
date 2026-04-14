# Roadmap

ChatAPI is stable and in use. This document describes what's planned, what's being considered, and what's firmly out of scope.

## In progress / next

- **PostgreSQL adapter** — implement `internal/repository/postgres/` against the existing repository interfaces. Zero service changes required; swap via `DATABASE_DSN`. See `CONTRIBUTING.md` if you want to contribute this.
- **Redis broker** — implement `internal/broker/redis.go` against the `broker.Broker` interface to enable horizontal scaling across multiple ChatAPI instances.

## Planned

- **Go 1.22 → latest** — upgrade the module to track a current Go release.
- **TypeScript SDK** — continue expanding coverage of bot endpoints and streaming event types as the API stabilises.
- **Example applications** — a reference implementation showing a full AI-powered chat app (Next.js frontend + webhook backend) wired to ChatAPI.

## Under consideration

- **Message reactions** — lightweight emoji reactions on messages.
- **Read receipts** — per-user read position beyond the current ack mechanism.
- **Room archiving** — soft-delete rooms without losing history.

## Out of scope

These will not be added to the core project:

- **Multi-tenancy** — ChatAPI is a single-workspace deployment. Run multiple instances for isolation.
- **MCP server** — the REST API is sufficient for agent integration.
- **Oversight / HITL primitives** — approval workflows, human-in-the-loop escalation. This is a separate problem; see [Checkpoint](https://github.com/getchatapi/checkpoint) when available.
- **Hosted SaaS** — self-hosted only.
- **Agent framework or LLM orchestration** — ChatAPI is the communication layer, not the brain.
- **Horizontal scaling built into core** — use the broker interface and a Redis implementation.
- **No-code / visual builders**.

## Contributing

See `CONTRIBUTING.md`. The most impactful open contributions right now are the PostgreSQL adapter and the Redis broker — both are well-scoped and the interfaces are already defined.
