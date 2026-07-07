# Backend Scaffolding

Go backend actively being scaffolded. Committed to Git.

## Structure
- `internal/trip/`
  - `domain.go` — state machine ([[03 Domain - Trip State Machine]])
  - `events.go` — domain events
  - `service.go` — service, occupancy floor invariant
- `internal/platform/eventbus/` — async worker-pool event bus, fail-loud error handling, swappable for NATS/Kafka
- `internal/platform/postgres/outbox.go` — transactional outbox writer + polling dispatcher (`FOR UPDATE SKIP LOCKED`, batch polling, marks rows dispatched)
- `migrations/0001_init.sql` — `trips` and `outbox_events` tables
- `cmd/api/main.go` — wires pool, bus, dispatcher, health endpoint
- `cmd/mockclient/main.go` — passenger/driver goroutine loops stubbed for HTTP-based load testing without real mobile apps

## Outstanding
- `go.mod` present but **no `go.sum`** — run `go mod tidy` locally

## Related
- [[08 Learnings and Principles]] (outbox pattern rationale)
- [[05 Roadmap]]
