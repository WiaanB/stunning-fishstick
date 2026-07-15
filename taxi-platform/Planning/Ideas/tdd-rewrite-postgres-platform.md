# TDD Rewrite: internal/platform/postgres
Status: Raised
Date: 2026-07-08

## TLDR
Delete and rebuild `internal/platform/postgres/{pool.go,outbox.go}` test-first — **blocked** on
deciding a Postgres integration-test strategy first, since this repo has no test database or CI
wired up yet.

## Details
**Prerequisite — decided 2026-07-15**: this package will be tested against a real Postgres via
`testcontainers-go` (a disposable container per test run, no manually-managed shared instance,
CI-ready later without extra setup). Not implemented yet — this only unblocks starting the
red-green-refactor loop here. Sequenced after [[tdd-rewrite-eventbus]] and
[[tdd-rewrite-trip-domain]] (both done) and [[tdd-rewrite-cmd-mockclient]] (done).

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
