# Domain: Trip State Machine

## States
Requested → Quoted → AwaitingPayment → CodeIssued → DriverAssigned → EnRoute → CodeVerified → InProgress → Completed

Side branches: **Cancelled**, **NoShow**

## Multi-seat booking
Supported via `seat_count` on trips (e.g., one payer booking for multiple passengers).

## Key domain invariant
Driver manual occupancy decrement cannot push total below the sum of `seat_count` across all InProgress booked trips for that vehicle. Unused booked seats must be explicitly marked **NoShow**.

## Implementation
- `internal/trip/domain.go` — state machine
- `internal/trip/events.go` — domain events
- `internal/trip/service.go` — service enforcing the occupancy floor invariant

See [[04 Backend Scaffolding]].
