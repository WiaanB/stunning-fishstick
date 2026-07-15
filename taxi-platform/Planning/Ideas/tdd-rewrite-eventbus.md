# TDD Rewrite: internal/platform/eventbus
Status: Completed
Date: 2026-07-08

## TLDR
Delete `internal/platform/eventbus/eventbus.go` and rebuild the in-process async worker-pool
pub/sub bus test-first — it has zero tests today despite real concurrency logic (worker pool,
backpressure, error handling, close semantics).

## Details
Requirements to preserve:
- `New(workers, queueSize int) *Bus`, clamping both to a minimum of 1.
- `Subscribe(eventType string, h Handler)` — no event replay for subscribers that join late.
- `Publish(ctx, events...) error` — enqueues each event; blocks on a full queue as backpressure,
  or returns a wrapped `ctx.Err()` if the context is cancelled while blocked.
- `dispatch` looks up handlers under `RLock` and invokes them sequentially; on error it calls the
  configurable `errorHandler` (default `log.Printf`-based) without stopping sibling handlers or
  retrying. `WithErrorHandler(fn) Option` overrides it.
- `Close()` drains workers via `wg.Wait()`.
- Handlers are expected to be idempotent (at-least-once delivery).
- `eventbus.Event` deliberately duplicates `trip.Event`'s shape via structural typing rather than
  importing it, keeping this a leaf dependency swappable for NATS/Kafka later — preserve that
  design, don't take a direct dependency on any domain package.

Also fix, as part of the rebuild (review finding): `Close()`/`Publish()` aren't currently safe
against double-close or publish-after-close panics. Decide the intended behavior (e.g. a second
`Close()` is a no-op, `Publish()` after `Close()` returns an error rather than panicking), write
the test first, then implement to match.

Tests to write (no I/O boundary here, so no fakes needed at all per the TDD skill — use real
`Bus` instances directly): Subscribe+Publish delivers to the right handler; multiple handlers for
one event type all run; a full queue backpressures `Publish`; `Publish` respects context
cancellation; the error handler fires on handler error without stopping siblings; `Close` drains
in-flight work; double-`Close` and publish-after-`Close` behave per the decision above.

## Related
- [[tdd-rewrite-initiative]]
- [[Backend Scaffolding]]
