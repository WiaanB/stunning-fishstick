# Event system reliability fixes after full code review

Status: Fixed
Date found: 2026-07-15
Date fixed: 2026-07-15

## TL;DR

A full review of the backend found five reliability and correctness issues in the event flow. Two can cause event loss or panics in production; the others hurt ordering, retryability, and lock contention.

## Details

### 1. `eventbus.Bus` can panic when `Publish` races with `Close` (High)

`Publish` checks `closed` under `errMu`, unlocks, then sends on `b.queue`. `Close` can close that channel between the unlock and the send, causing a concurrent publisher to panic with "send on closed channel".

Affected: `internal/platform/eventbus/eventbus.go:110-123`, `:130-140`

### 2. Outbox acknowledges events before handlers actually finish (High)

`Dispatcher.pollOnce` updates `outbox_events.dispatched_at` only after `publish` (the in-process bus enqueue) succeeds. That means:

- A crash after the row is marked dispatched but before a worker runs the handler silently loses the event.
- A handler failure only triggers the error handler; the row is still marked dispatched, so the event will never be retried.

This makes "at-least-once delivery" only true up to the bus queue, not up to actual side effects (payments, notifications).

Affected: `internal/platform/postgres/outbox.go:137-152`, `internal/platform/eventbus/eventbus.go:87-96`, `cmd/api/main.go:122-125`

### 3. `PendingEvents` clears events before the repository knows the tx committed (High)

A repository must call `t.PendingEvents()` to get private events into the outbox. `PendingEvents` mutates `t.pending = nil` and returns the slice. If the transaction then fails, the aggregate object has no events left; retrying the operation is either a no-op or an illegal transition, so the event is lost.

Affected: `internal/trip/domain.go:103-109`, `internal/trip/service.go:15-21`

### 4. Event ordering per aggregate is not guaranteed (Medium)

- Outbox `SELECT` orders only by `created_at`; ties are non-deterministic.
- Concurrent dispatchers can claim different batches from the same aggregate.
- The four-worker bus may run a later trip event before an earlier one.

State-dependent consumers can observe invalid sequences for the same trip.

Affected: `internal/platform/postgres/outbox.go:108-115`, `internal/platform/eventbus/eventbus.go:73-84`, `cmd/api/main.go:40`

### 5. Dispatcher holds DB locks while calling `PublishFunc` (Medium)

`pollOnce` starts a transaction, selects rows `FOR UPDATE SKIP LOCKED`, then calls `publish` for each record inside the same tx. If the bus queue is full or `PublishFunc` blocks, row locks are held for the duration, limiting throughput and increasing contention across dispatcher instances.

Affected: `internal/platform/postgres/outbox.go:102-150`

## Proposed fixes / approaches

1. **Close race in eventbus**
   - Use a `select` with a `done` channel or a single writer goroutine so `Close` and `Publish` coordinate without allowing a send after close.
   - Add a test that runs `Publish` and `Close` concurrently under the race detector.

2. **Ack only after handlers complete**
   - Make `PublishFunc` return only after the in-process handler(s) have completed, or introduce a synchronous dispatch mode for the outbox path. Alternatively, move hand-off to a durable consumer. Document the trade-off if left as a known limitation.

3. **Non-destructive event reads for repository lateral effects**
   - Let `Repository.Save` see pending events without the domain clearing them; clear only after a successful commit, or hand the repository a copy and drain the aggregate after the service call based on success/failure. Keep the TDD loop.

4. **Per-aggregate ordering**
   - Add `aggregate_id` ordering or hashing so rows for the same aggregate are dispatched in sequence. Consider a single dispatcher handling one aggregate at a time, or batch by aggregate.

5. **Shorten lock window**
   - Select/claim IDs, commit the claim transaction, then publish outside the transaction, and mark dispatched in a separate small transaction. Or use advisory locks per aggregate.

## Acceptance criteria

- [x] `go test -race ./...` still passes after all changes.
- [x] A new test demonstrates `Publish`-vs-`Close` race safety.
- [x] Outbox rows are not marked dispatched until their handlers have actually run (or a documented, review-approved exception exists).
- [x] Retrying `Service.MarkNoShow` / `RequestTrip` after a failed repository commit does not silently drop events or fail due to state mutation.
- [x] Events for the same aggregate are dispatched in `created_at` order, with deterministic tie-breaking.
- [x] Matching vault notes are updated if any behavior/capability changes.

## Resolution

1. **`eventbus.Bus` Publish/Close race** — `Publish` now holds a dedicated
   `closeMu` `RWMutex` as a read-lock across its whole check-then-send
   section; `Close` takes the write-lock across `closed = true;
   close(b.queue)`. `TestPublishCloseRaceDoesNotPanic` races many concurrent
   `Publish` calls against `Close` under `-race`.
2. **Ack before handlers finish** — the real bug was in `cmd/api/main.go`'s
   `publishToBus`, which handed records to `bus.Publish` (async enqueue,
   returns before any handler runs). Added `Bus.Dispatch`, which runs
   handlers synchronously on the caller's goroutine, and switched
   `publishToBus` to use it. `Dispatcher.pollOnce` already only acked after
   its `PublishFunc` returned; now that return actually means "handled."
3. **Destructive `PendingEvents`** — split into non-destructive `Trip.Events()`
   and explicit `Trip.ClearEvents()`. No real `Repository` exists yet, so
   this was a footgun fix rather than a live bug; the contract is now safe
   by construction for whoever implements it.
4. **Ordering** — `outbox_events` gained a `seq bigserial` tiebreak
   (migration `0002_outbox_ordering.sql`); `pollOnce`'s claim query orders
   by `created_at, seq`. Cross-instance ordering is handled by fix 5's
   per-aggregate claim exclusivity.
5. **Lock contention** — `pollOnce` now claims rows (`FOR UPDATE SKIP
   LOCKED`, at most one row per aggregate at a time via a `claimed_at`
   lease) in a short transaction, publishes each **outside** any
   transaction, then acks/releases in a separate statement. A slow handler
   for one aggregate no longer blocks dispatch of unrelated aggregates or
   other dispatcher instances (`TestSlowHandlerDoesNotBlockOtherAggregates`,
   `TestConcurrentPollOnceNeverClaimsSameAggregateTwice`).

## Related

- [[Backend Scaffolding]]
- [[Trip State Machine]]
- [[02 Architecture Principles]]
- [[03 Learnings and Principles]]
- [[Roadmap]]
