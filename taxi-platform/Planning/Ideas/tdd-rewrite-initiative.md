# TDD Rewrite of Existing Backend Packages
Status: Done
Date: 2026-07-08
Completed: 2026-07-15

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
3. **Done** — [[tdd-rewrite-cmd-api]]. Confirmed its "postgres interfaces settled" dependency was
   already satisfied (postgres-platform's rewrite doesn't change interfaces), so this proceeded
   ahead of wave 4 as anticipated.
4. **Done** — [[tdd-rewrite-postgres-platform]], the last package. Added `testcontainers-go` as a
   new dependency (`go.mod`/`go.sum`, the latter previously missing entirely); one Postgres
   container per package test run via `TestMain`, `migrations/0001_init.sql` applied as its init
   script so test and real schema can't drift.

All four packages in this initiative are now rewritten test-first. See [[Backend Scaffolding]] for
current file pointers.

## Related
- [[tdd-rewrite-trip-domain]]
- [[tdd-rewrite-eventbus]]
- [[tdd-rewrite-postgres-platform]]
- [[tdd-rewrite-cmd-api]]
- [[tdd-rewrite-cmd-mockclient]]
- [[Backend Scaffolding]]
- [[Trip State Machine]]
- [[Roadmap]]
