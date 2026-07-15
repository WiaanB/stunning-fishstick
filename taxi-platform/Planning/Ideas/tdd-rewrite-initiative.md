# TDD Rewrite of Existing Backend Packages
Status: In Progress
Date: 2026-07-08

## TLDR
Delete and reimplement every existing Go package under strict red-green-refactor TDD, preserving
today's observable behavior/requirements — most of the codebase has zero test coverage today
(only `internal/trip/domain_test.go` exists, and even `internal/trip`'s service layer is
untested), and this rebuilds it all test-first rather than retrofitting tests onto existing code.

## Details
Approach, per package: delete the current implementation, then follow the TDD skill's
red-green-refactor loop (failing test → minimal code → refactor) to rebuild it, using the
requirements listed in each package's own ticket as the spec to satisfy. This is a rewrite of
implementation, not of contracts — HTTP-visible behavior, event shapes, and table schemas stay
the same unless a ticket says otherwise.

Recommended sequencing:
1. **Done** — [[tdd-rewrite-eventbus]] and [[tdd-rewrite-trip-domain]], no external dependencies,
   straightforward unit testing, nothing else blocked on a decision.
2. **Done** — [[tdd-rewrite-cmd-mockclient]], also dependency-free, `httptest`-friendly.
3. [[tdd-rewrite-cmd-api]] next — partially testable via `httptest`, partially needs the wiring
   decomposed to be testable at all. Its own ticket depends on postgres/eventbus interfaces being
   settled; eventbus is done, and since the postgres-platform rewrite is scoped as test-coverage
   only (no interface changes), cmd/api can reasonably proceed before wave 4 rather than waiting
   on it — confirm this reading when cmd/api's turn comes.
4. [[tdd-rewrite-postgres-platform]] last — Postgres integration-test strategy decided:
   `testcontainers-go` (real disposable Postgres per test run). Implementation still open.

## Related
- [[tdd-rewrite-trip-domain]]
- [[tdd-rewrite-eventbus]]
- [[tdd-rewrite-postgres-platform]]
- [[tdd-rewrite-cmd-api]]
- [[tdd-rewrite-cmd-mockclient]]
- [[Backend Scaffolding]]
- [[Trip State Machine]]
- [[Roadmap]]
