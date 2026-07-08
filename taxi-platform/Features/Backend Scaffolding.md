# Backend Scaffolding

## TLDR
The Go backend's foundational infrastructure — package layout, the async event bus, and the transactional outbox — that every feature is built on top of.

## Capabilities
- `internal/trip/` domain package (state machine, events, service — see [[Trip State Machine]])
- `internal/platform/eventbus/` — async worker-pool event bus with fail-loud error handling, swappable for NATS/Kafka later
- `internal/platform/postgres/outbox.go` — transactional outbox writer + polling dispatcher (`FOR UPDATE SKIP LOCKED`, batch polling, marks rows dispatched)
- `migrations/0001_init.sql` — `trips` and `outbox_events` tables
- `cmd/api/main.go` — wires pool, bus, dispatcher, health endpoint
- `cmd/mockclient/main.go` — passenger/driver goroutine loops stubbed for HTTP-based load testing without real mobile apps

## Implementation
- `internal/trip/domain.go`, `events.go`, `service.go`
- `internal/platform/eventbus/eventbus.go`
- `internal/platform/postgres/outbox.go`
- `migrations/0001_init.sql`
- `cmd/api/main.go`, `cmd/mockclient/main.go`

## Status
Actively being scaffolded. `go.mod` present but **no `go.sum`** — run `go mod tidy` locally.

## Related
- [[Trip State Machine]]
- [[03 Learnings and Principles]] (outbox pattern rationale)
- [[Roadmap]]
- [[tdd-rewrite-initiative]]
