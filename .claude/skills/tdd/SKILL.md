---
name: tdd
description: Use this skill whenever implementing new behavior, fixing a bug, or changing domain/service logic in this repo's Go code (internal/, cmd/) — write a failing test first, then the minimal code to pass it, then refactor. Does not apply to pure doc edits, config, or migrations with no code branching.
---

# TDD for taxi-platform

Strict red-green-refactor for code in this repo:

1. **Red** — write a test that exercises the new behavior or reproduces the bug. Run it and confirm it fails for the expected reason (not a compile error).
2. **Green** — write the minimal code to make it pass. Don't build out more than the test demands.
3. **Refactor** — clean up implementation or test code with the suite green throughout, re-running after each change.

**Exception**: trivial wiring or glue with no conditional, no state transition, and no invariant (e.g. adding a struct field, threading a new constructor argument) doesn't need a test written first. Any `if`, state transition, or invariant does.

## Where tests live, how to run them

- White-box tests: `package trip`, not `package trip_test`. Same package as the code under test, so tests can reach unexported identifiers directly (see `internal/trip/domain_test.go`, which is `package trip` and calls `NewTrip`, `tr.State`, etc. without exporting them).
- File name: `<source>_test.go` beside the file it covers (`domain.go` → `domain_test.go`).
- No Makefile or CI wired up yet — run tests directly with `go test ./...` from the repo root (or `go test ./internal/trip/...` for a single package while iterating).

## Style — match `internal/trip/domain_test.go`

- Plain stdlib `testing`. No assertion library (no testify) — anywhere in the module's dependency graph.
- `t.Fatalf`/`t.Fatal` with explicit `if err != nil` checks, not a `require`/`assert` helper.
- Scenario-named test functions describing the behavior under test — `TestHappyPathTransitions`, `TestIllegalTransitionRejected`, `TestVerifyCodeMismatch`, `TestOccupancyFloorInvariant` — not `TestNewTrip`/`TestQuote` named after the method. Reach for table-driven tests only when cases are genuinely uniform (same shape, differing only in input/output pairs); don't force a table onto a multi-step scenario like the happy-path walk in `TestHappyPathTransitions`.
- Assert typed errors with `errors.As`, never by matching on `err.Error()` strings:
  ```go
  var floorErr *OccupancyFloorError
  if !errors.As(err, &floorErr) {
      t.Fatalf("expected *OccupancyFloorError, got %T: %v", err, err)
  }
  ```
  (verbatim from `TestOccupancyFloorInvariant`, `internal/trip/domain_test.go:96-98`)

## Test doubles

- Hand-roll a fake struct implementing the package's own interface, defined right in the test file — never a mocking library or generated mocks. See `fakeRepo` and `fakePublisher` in `internal/trip/domain_test.go:67-81`, which implement `Repository` and `EventPublisher` (from `internal/trip/service.go`) with just enough behavior for the test at hand (`fakeRepo.InProgressSeatCount` returns a fixed value; `Save`/`FindByID` are stubbed since `TestOccupancyFloorInvariant` never calls them).
- Prefer a real collaborator over a fake when it's cheap and has no external dependencies — e.g. `eventbus.Bus` (`internal/platform/eventbus/eventbus.go`) can be constructed and exercised directly in a test with no fake needed. Reserve fakes for the actual I/O boundary (`Repository`, `EventPublisher`).

## What to test where

- **Domain layer** (`internal/trip/domain.go`): state transitions, both legal (via `CanTransition`) and illegal, invariants, and event emission via `PendingEvents()`. Pure, no I/O — this is the bulk of the coverage and the cheapest to write.
- **Service layer** (`internal/trip/service.go`): orchestration and error wrapping (e.g. `AdjustOccupancy`, `MarkNoShow`), using fakes for `Repository`/`EventPublisher` to isolate from real persistence.
- **Infra touching a real Postgres** (`internal/platform/postgres/{pool.go,outbox.go}`): tested against a real, disposable Postgres via `testcontainers-go` — see `internal/platform/postgres/main_test.go`. One container is started per package test run (`TestMain`), not per test, with `migrations/0001_init.sql` applied as its init script so the test schema can never drift from the real one; individual tests get isolation via a truncate-tables helper rather than a fresh container each. There's no `*testing.T` inside `TestMain`, so Docker's availability is checked there directly and stored in a package-level flag; tests that need the database check that flag and call `t.Skip` themselves if Docker isn't running, so `go test ./...` stays the one command that runs everything (skipping gracefully rather than failing where Docker isn't available, and with no build tag gating it). Future Postgres-touching code (sqlc queries) should follow this same pattern; Redis integration code will need its own equivalent test setup when that roadmap step lands, following the same shape (real disposable instance via testcontainers, shared per test run, not invented ad hoc).
- **Future HTTP handlers** (`internal/api`, not yet built per the roadmap): once that package lands, use `net/http/httptest` and apply the same red-green-refactor discipline — write the failing handler test against `httptest.NewRecorder`/`httptest.NewServer` before writing the handler.
