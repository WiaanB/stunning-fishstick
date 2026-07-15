# TDD Rewrite: cmd/mockclient
Status: Raised
Date: 2026-07-08

## TLDR
Delete and rebuild `cmd/mockclient/main.go` test-first — it's untested today, and its own
package comment already documents it as a stub (passenger and driver loops both just ping
`/healthz`).

## Details
Requirements to preserve:
- Flags: `-api-addr` (default `http://localhost:8080`), `-passengers` (default 5), `-drivers`
  (default 3), `-interval` (default 2s).
- `main()` spins up `passengers`+`drivers` goroutines via `sync.WaitGroup` on a signal-aware
  context.
- `loop(ctx, interval, action)` — generic ticker-driven loop respecting `ctx.Done()`.
- `ping(ctx, client, apiAddr)` — `GET {apiAddr}/healthz`, errors on non-200 or transport failure.
- `simulatePassenger`/`simulateDriver` both currently just call `ping` on each tick — preserve
  this stub behavior. The package comment already says these become real trip-endpoint calls once
  `cmd/api` grows beyond `/healthz`; don't invent those endpoints in this ticket, that's separate,
  later work once the trip HTTP handlers exist.

Testing approach: `ping` and `loop` have no real external dependency beyond an HTTP client, so
they're directly testable with `httptest.NewServer`. Write failing tests first for `ping`'s
success/non-200/transport-error paths and `loop`'s tick/cancel behavior (short intervals, a
cancelled context) before rebuilding.

## Related
- [[tdd-rewrite-initiative]]
- [[Backend Scaffolding]]
