# Trip State Machine

## TLDR
Domain-level state machine governing a trip's lifecycle from request through completion, enforcing legal transitions and a hard invariant on vehicle occupancy.

## Capabilities
- State machine with legal-transition checking: Requested → Quoted → AwaitingPayment → CodeIssued → DriverAssigned → EnRoute → CodeVerified → InProgress → Completed
- Side branches from the happy path: **Cancelled**, **NoShow**
- Multi-seat booking via `seat_count` on trips (one payer booking for multiple passengers)
- Occupancy floor invariant: a driver's manual occupancy decrement cannot push total below the sum of `seat_count` across all InProgress booked trips for that vehicle — unused booked seats must be explicitly marked NoShow instead
- Domain event emission on state transitions; `VerifyCode` gates on `CanTransition` before comparing codes, consistent with every other transition method, so an illegal-state call surfaces as a transition error rather than a misleading code-mismatch error
- `Repository.Save` takes ownership of persisting a trip's row and its pending domain events together as one atomic unit (no separate `EventPublisher` call in the write path) — `Service.RequestTrip` and `Service.MarkNoShow` are both covered by tests using a hand-rolled fake `Repository`

## Implementation
- `internal/trip/domain.go` — state machine
- `internal/trip/events.go` — domain events, including transition methods
- `internal/trip/service.go` — `Repository` interface and `Service` (`RequestTrip`, `MarkNoShow`, `AdjustOccupancy`) enforcing the occupancy floor invariant

## Status
Core domain layer and service layer implemented and unit-tested (`internal/trip/domain_test.go`, `events_test.go`, `service_test.go`) — TDD rewrite completed, see [[tdd-rewrite-trip-domain]]. HTTP handlers not yet built — see [[Roadmap]].

## Related
- [[Backend Scaffolding]]
- [[Roadmap]]
- [[tdd-rewrite-initiative]]
- [[tdd-rewrite-trip-domain]]
