# Backend Scaffolding

## TLDR
The Go backend's foundational infrastructure — package layout, the async event bus, and the transactional outbox — that every feature is built on top of.

## Capabilities
- `internal/trip/` domain package (state machine, events, service — see [[Trip State Machine]])
 - `internal/platform/eventbus/` — async worker-pool event bus with fail-loud error handling, swappable for NATS/Kafka later; `Close()` is idempotent and `Publish()` after close returns `ErrBusClosed`
- `internal/platform/postgres/outbox.go` — transactional outbox writer + polling dispatcher (`FOR UPDATE SKIP LOCKED`, batch polling, marks rows dispatched); tested against a real disposable Postgres via `testcontainers-go` (one container per package test run, `migrations/0001_init.sql` applied as its init script)
- `migrations/0001_init.sql` — `trips` and `outbox_events` tables
- `cmd/api/main.go` — wires pool, bus, dispatcher, health endpoint; shutdown-branch selection (`awaitShutdown`), shutdown ordering (`closeInOrder`), and env defaulting (`getEnvOrDefault`) are decomposed into unit-tested functions, with real DB/HTTP wiring left as untested glue
- `cmd/mockclient/main.go` — passenger/driver goroutine loops stubbed for HTTP-based load testing without real mobile apps

## Implementation
- `internal/trip/domain.go`, `events.go`, `service.go`, `domain_test.go`, `events_test.go`, `service_test.go`
- `internal/platform/eventbus/eventbus.go`, `internal/platform/eventbus/eventbus_test.go`
- `internal/platform/postgres/pool.go`, `outbox.go`, `pool_test.go`, `outbox_test.go`, `main_test.go` (shared `TestMain` container setup)
- `migrations/0001_init.sql`
- `cmd/api/main.go`, `cmd/api/main_test.go`
- `cmd/mockclient/main.go`, `cmd/mockclient/main_test.go`

## Status
Actively being scaffolded. `go.sum` now present (added alongside the `internal/platform/postgres` rewrite's new dependency).

All four TDD rewrites complete — event bus, trip domain, `cmd/mockclient`, `cmd/api`, and `internal/platform/postgres` — see [[tdd-rewrite-initiative]].

## Related
- [[Trip State Machine]]
- [[03 Learnings and Principles]] (outbox pattern rationale)
- [[Roadmap]]
- [[tdd-rewrite-initiative]]
