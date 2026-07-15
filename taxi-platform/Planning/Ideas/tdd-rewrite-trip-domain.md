# TDD Rewrite: internal/trip
Status: Raised
Date: 2026-07-08

## TLDR
Delete `internal/trip/{domain.go,events.go,service.go}` and rebuild the trip aggregate, its
11-state machine, 11 domain events, and the `Service` orchestration layer strictly test-first —
`domain_test.go` already matches the TDD skill's conventions and can stay as a starting spec, but
the service layer's test gap gets closed as part of the rebuild, not bolted on after.

## Details
Requirements to preserve (today's observable behavior):
- 11 states (`Requested, Quoted, AwaitingPayment, CodeIssued, DriverAssigned, EnRoute,
  CodeVerified, InProgress, Completed, Cancelled, NoShow`) and the current legal-transition table,
  including that `InProgress` only reaches `Completed` (no cancel/no-show once code is verified).
  Preserve this exactly unless product says otherwise — flag it for confirmation, don't change it
  silently.
- `CanTransition(from, to State) bool`.
- `Trip` aggregate (`ID, State, SeatCount, VehicleID, Code, CreatedAt, UpdatedAt`, pending events);
  `NewTrip(seatCount int)` validates `seatCount >= 1`.
- `PendingEvents()` drains (returns + clears) pending events.
- 11 domain events with `EventType()`/`AggregateID()`. Fix the naming mismatch found in review:
  the `TripStarted` struct emits event type `"trip.in_progress"` — pick one name during the
  rewrite and make the test lock it in.
- Transition methods `Quote, AwaitPayment, IssueCode, AssignDriver, MarkEnRoute, VerifyCode,
  Start, Complete, Cancel, MarkNoShow`. `VerifyCode`'s wrong-code path currently returns
  `CodeMismatchError` before checking `CanTransition` at all — decide during the rewrite whether
  to keep that shortcut or route it through the transition table first, and write a test that
  pins down whichever is chosen.
- `CodeMismatchError`, `OccupancyFloorError` typed errors, asserted via `errors.As`.
- `Service`: `Repository`/`EventPublisher` interfaces, `RequestTrip`, `MarkNoShow`,
  `AdjustOccupancy`.
- **Design fix, not just a test gap**: today's `Service.apply` calls `repo.Save` then
  `events.Publish` as two independent calls, so no real `Repository` implementation can satisfy
  the same-transaction guarantee the doc comments promise. Since this package is being rebuilt
  from scratch, shape `Repository`/`EventPublisher` so a real implementation *can* persist the
  trip row and its pending events atomically (e.g. `Repository.Save` taking ownership of
  persisting pending events too, or an explicit transaction-scoped interface). Write this as a
  driving test/spec, not an afterthought.
- Add the missing service-layer tests: `Service.apply`'s Save-then-Publish orchestration,
  `RequestTrip`, `MarkNoShow` — untested today (the existing `fakeRepo.FindByID` always errors).
- **Out of scope**: exposing the other 7 domain transitions at the service layer — that's
  Roadmap step 1 (HTTP handlers), not this rewrite.

## Related
- [[tdd-rewrite-initiative]]
- [[Trip State Machine]]
