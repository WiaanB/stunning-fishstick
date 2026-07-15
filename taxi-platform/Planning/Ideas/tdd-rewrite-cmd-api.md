# TDD Rewrite: cmd/api
Status: Raised
Date: 2026-07-08

## TLDR
Delete and rebuild `cmd/api/main.go` test-first, decomposing the process wiring enough that its
conditional logic (shutdown sequencing, health check) is actually testable rather than only
reachable by running the real binary.

## Details
Requirements to preserve:
- `run()` wiring order: signal-context → DSN from `DATABASE_URL` env (default
  `postgres://localhost:5432/taxi_platform`) → `postgres.NewPool` → `eventbus.New(4, 256)` →
  `postgres.NewDispatcher(pool, publishToBus(bus), 100, time.Second)` → dispatcher goroutine →
  HTTP mux with only `GET /healthz` → server on `API_ADDR` env (default `:8080`) → graceful
  shutdown on signal with a 5s HTTP shutdown timeout, waiting for the dispatcher to observe
  context cancellation.
- `outboxEvent` adapter wrapping `postgres.OutboxRecord` to satisfy `eventbus.Event`;
  `publishToBus(bus)` closing over the bus.
- `healthHandler(pool interface{ Ping(context.Context) error })` — 503 + JSON error body on ping
  failure, 200 + `{"status":"ok"}` on success. Keep the structural-interface parameter (not
  `*pgxpool.Pool` directly) — it's what makes this testable without a real pool.
- Defer ordering: `bus.Close()` before `pool.Close()` (stop consuming before closing the DB the
  dispatcher reads from) — today this relies on Go's LIFO defer order rather than being explicit;
  consider making it explicit during the rewrite since it's exactly the kind of thing a test
  should pin down.

Testing approach: the TDD skill already says once HTTP handlers exist, use `net/http/httptest`
and the same red-green-refactor discipline — `healthHandler` exists today, so this applies now.
Write `httptest`-based tests for its 200/503 branches before rebuilding it. Extract `run()`'s
shutdown-select branch into a function testable with fake/channel inputs rather than real signals
and a real listener, and TDD that in isolation. Pure construction/wiring lines (`pool := ...`,
`bus := ...`) stay under the TDD skill's "trivial glue" exception and don't need tests of their
own.

Depends on [[tdd-rewrite-postgres-platform]] and [[tdd-rewrite-eventbus]] having settled
interfaces first.

## Related
- [[tdd-rewrite-initiative]]
- [[Backend Scaffolding]]
