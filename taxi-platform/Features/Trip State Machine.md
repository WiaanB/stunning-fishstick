# Trip State Machine

## TLDR
Domain-level state machine governing a trip's lifecycle from request through completion, enforcing legal transitions and a hard invariant on vehicle occupancy.

## Capabilities
- State machine with legal-transition checking: Requested → Quoted → AwaitingPayment → CodeIssued → DriverAssigned → EnRoute → CodeVerified → InProgress → Completed
- Side branches from the happy path: **Cancelled**, **NoShow**
- Multi-seat booking via `seat_count` on trips (one payer booking for multiple passengers)
- Occupancy floor invariant: a driver's manual occupancy decrement cannot push total below the sum of `seat_count` across all InProgress booked trips for that vehicle — unused booked seats must be explicitly marked NoShow instead
- Domain event emission on state transitions

## Implementation
- `internal/trip/domain.go` — state machine
- `internal/trip/events.go` — domain events
- `internal/trip/service.go` — service enforcing the occupancy floor invariant

## Status
Core domain layer implemented and unit-tested (`internal/trip/domain_test.go`). HTTP handlers not yet built — see [[Roadmap]].

## Related
- [[Backend Scaffolding]]
- [[Roadmap]]
- [[tdd-rewrite-initiative]]
