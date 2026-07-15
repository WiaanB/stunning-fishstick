# TDD Rewrite of Existing Backend Packages
Status: Raised
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
1. [[tdd-rewrite-eventbus]] and [[tdd-rewrite-trip-domain]] first — no external dependencies,
   straightforward unit testing, nothing else blocks on a decision.
2. [[tdd-rewrite-cmd-mockclient]] next — also dependency-free, `httptest`-friendly.
3. [[tdd-rewrite-cmd-api]] — partially testable via `httptest`, partially needs the wiring
   decomposed to be testable at all; depends on the eventbus/postgres interfaces being settled.
4. [[tdd-rewrite-postgres-platform]] last — **blocked** on deciding a Postgres integration-test
   strategy first (no test DB or CI wired up in this repo yet). The TDD skill explicitly treats
   this as "an open item rather than skipping tests silently or inventing tooling that doesn't
   exist in this repo" — that decision belongs to the user, not this initiative.

## Related
- [[tdd-rewrite-trip-domain]]
- [[tdd-rewrite-eventbus]]
- [[tdd-rewrite-postgres-platform]]
- [[tdd-rewrite-cmd-api]]
- [[tdd-rewrite-cmd-mockclient]]
- [[Backend Scaffolding]]
- [[Trip State Machine]]
- [[Roadmap]]
