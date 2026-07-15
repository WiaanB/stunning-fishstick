# Backend Scaffolding

## TLDR
The Go backend's foundational infrastructure — package layout, the async event bus, and the transactional outbox — that every feature is built on top of.

## Capabilities
- `internal/trip/` domain package (state machine, events, service — see [[Trip State Machine]]); `Trip.Events()` is a non-destructive peek at pending domain events, `Trip.ClearEvents()` explicitly drops them — a `Repository.Save` must only call `ClearEvents()` after its persist transaction commits, so a failed save never loses events for a retry
- `internal/platform/eventbus/` — worker-pool event bus with fail-loud error handling, swappable for NATS/Kafka later; `Close()` is idempotent and `Publish()` after close returns `ErrBusClosed`; `Publish`/`Close` coordinate via a dedicated `RWMutex` so a concurrent `Close` can never close the queue mid-send. `Dispatch()` runs handlers synchronously on the caller's goroutine (no queue hand-off) for callers that must know handlers actually completed, not just that the event was enqueued
- `internal/platform/postgres/outbox.go` — transactional outbox writer + polling dispatcher. `pollOnce` claims rows in a short transaction (`FOR UPDATE SKIP LOCKED`, at most one row per aggregate at a time across all Dispatcher instances via a `claimed_at` lease), publishes each **outside** any transaction so a slow handler holds no row lock, then acks (or releases the claim on failure) in a separate statement — so a row is only marked dispatched once its handler has actually run, and per-aggregate ordering holds even under concurrent dispatcher instances. `created_at, seq` ordering gives a deterministic tiebreak for same-timestamp rows. Tested against a real disposable Postgres via `testcontainers-go` (one container per package test run, `migrations/*.sql` applied in order as init scripts)
- `migrations/0001_init.sql` — `trips` and `outbox_events` tables
- `migrations/0002_outbox_ordering.sql` — adds `outbox_events.seq` (monotonic ordering tiebreak) and `claimed_at` (cross-instance claim lease)
- `cmd/api/main.go` — wires pool, bus, dispatcher, health endpoint; `publishToBus` hands outbox records to the bus via `Dispatch` (synchronous), not `Publish`, so the outbox dispatcher's ack-after-handler guarantee actually holds; shutdown-branch selection (`awaitShutdown`), shutdown ordering (`closeInOrder`), and env defaulting (`getEnvOrDefault`) are decomposed into unit-tested functions, with real DB/HTTP wiring left as untested glue
- `cmd/mockclient/main.go` — passenger/driver goroutine loops stubbed for HTTP-based load testing without real mobile apps

## Implementation
- `internal/trip/domain.go`, `events.go`, `service.go`, `domain_test.go`, `events_test.go`, `service_test.go`
- `internal/platform/eventbus/eventbus.go`, `internal/platform/eventbus/eventbus_test.go`
- `internal/platform/postgres/pool.go`, `outbox.go`, `pool_test.go`, `outbox_test.go`, `main_test.go` (shared `TestMain` container setup)
- `migrations/0001_init.sql`, `migrations/0002_outbox_ordering.sql`
- `cmd/api/main.go`, `cmd/api/main_test.go`
- `cmd/mockclient/main.go`, `cmd/mockclient/main_test.go`

## Status
Actively being scaffolded. `go.sum` now present (added alongside the `internal/platform/postgres` rewrite's new dependency).

All four TDD rewrites complete — event bus, trip domain, `cmd/mockclient`, `cmd/api`, and `internal/platform/postgres` — see [[tdd-rewrite-initiative]].

The five reliability/correctness issues from [[event-system-reliability-fixes]] are fixed: the `Publish`/`Close` race, ack-before-handler-completion, the destructive `PendingEvents` read, outbox ordering ties, and dispatcher lock contention.

## Related
- [[Trip State Machine]]
- [[03 Learnings and Principles]] (outbox pattern rationale)
- [[Roadmap]]
- [[tdd-rewrite-initiative]]
