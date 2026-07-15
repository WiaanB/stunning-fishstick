# TDD Rewrite: internal/platform/postgres
Status: Raised
Date: 2026-07-08

## TLDR
Delete and rebuild `internal/platform/postgres/{pool.go,outbox.go}` test-first — **blocked** on
deciding a Postgres integration-test strategy first, since this repo has no test database or CI
wired up yet.

## Details
**Prerequisite**: before red-green-refactor can start here, decide how this package gets tested
against a real Postgres (e.g. `testcontainers-go`, a local docker-compose Postgres, or similar).
That's a tooling decision for the user to make — the TDD skill explicitly says to treat this gap
as an open item rather than inventing tooling unilaterally. Sequence this ticket after that
decision, and after [[tdd-rewrite-eventbus]] and [[tdd-rewrite-trip-domain]].

Requirements to preserve:
- `pool.go`: `NewPool(ctx, dsn) (*pgxpool.Pool, error)` — opens a pool, pings it, closes the pool
  and returns a wrapped error on ping failure. No pool tuning (`MaxConns`/`MinConns`/
  `MaxConnLifetime`) is exposed today — decide during the rewrite whether to add configurability
  or keep pgxpool defaults, and write a test/spec either way.
- `outbox.go`: `OutboxRecord{ID, AggregateID, EventType, Payload, CreatedAt}`;
  `InsertOutbox(ctx, tx, aggregateID, eventType, payload)` marshals `payload` to JSON and inserts
  into `outbox_events`, must run inside the caller's transaction; `PublishFunc` as the
  dependency-free seam to `eventbus`; `Dispatcher{pool, publish, batchSize, interval}`,
  `NewDispatcher` defaulting `batchSize=100`/`interval=1s` on invalid input; `Run(ctx)` — ticker
  loop that logs and continues on poll errors, returns `ctx.Err()` on cancellation; `pollOnce` —
  `SELECT ... FOR UPDATE SKIP LOCKED` up to `batchSize` undispatched rows ordered by
  `created_at`, calls `publish` per record, marks `dispatched_at` on success in the same tx,
  leaves failed records undispatched and continues rather than failing the whole batch, commits
  at the end.
- No migration changes needed — `idx_trips_vehicle_in_progress` and
  `idx_outbox_events_undispatched` already match these query shapes.
- Document, don't necessarily fix in this ticket: "dispatched" today only means hand-off to the
  in-process bus succeeded, not that a handler actually ran — a crash after commit but before the
  queued handler runs loses the event silently. This is consistent with the documented plan to
  swap the bus for something durable later; note it as a known limitation rather than folding a
  fix into this ticket's scope.

## Related
- [[tdd-rewrite-initiative]]
- [[Backend Scaffolding]]
