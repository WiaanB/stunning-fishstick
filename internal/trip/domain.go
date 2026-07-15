// Package trip implements the trip domain: its state machine, events, and
// invariants. See taxi-platform/03 Domain - Trip State Machine.md.
package trip

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// State is a step in the trip lifecycle.
type State string

const (
	StateRequested       State = "requested"
	StateQuoted          State = "quoted"
	StateAwaitingPayment State = "awaiting_payment"
	StateCodeIssued      State = "code_issued"
	StateDriverAssigned  State = "driver_assigned"
	StateEnRoute         State = "en_route"
	StateCodeVerified    State = "code_verified"
	StateInProgress      State = "in_progress"
	StateCompleted       State = "completed"
	StateCancelled       State = "cancelled"
	StateNoShow          State = "no_show"
)

// transitions enumerates the only legal State -> State moves. Side branches
// (Cancelled, NoShow) are listed against every state they can be reached from.
var transitions = map[State][]State{
	StateRequested:       {StateQuoted, StateCancelled},
	StateQuoted:          {StateAwaitingPayment, StateCancelled},
	StateAwaitingPayment: {StateCodeIssued, StateCancelled},
	StateCodeIssued:      {StateDriverAssigned, StateCancelled},
	StateDriverAssigned:  {StateEnRoute, StateCancelled, StateNoShow},
	StateEnRoute:         {StateCodeVerified, StateCancelled, StateNoShow},
	StateCodeVerified:    {StateInProgress},
	StateInProgress:      {StateCompleted},
	StateCompleted:       {},
	StateCancelled:       {},
	StateNoShow:          {},
}

// CanTransition reports whether moving from `from` to `to` is a legal step.
func CanTransition(from, to State) bool {
	for _, s := range transitions[from] {
		if s == to {
			return true
		}
	}
	return false
}

// Trip is the aggregate root for a single ride, possibly booked for
// multiple seats by one payer.
type Trip struct {
	ID        uuid.UUID
	State     State
	SeatCount int
	VehicleID *uuid.UUID
	Code      string
	CreatedAt time.Time
	UpdatedAt time.Time

	pending []Event
}

// NewTrip creates a trip in the Requested state for the given seat count.
func NewTrip(seatCount int) (*Trip, error) {
	if seatCount < 1 {
		return nil, fmt.Errorf("trip: seat count must be at least 1, got %d", seatCount)
	}
	now := time.Now().UTC()
	t := &Trip{
		ID:        uuid.New(),
		State:     StateRequested,
		SeatCount: seatCount,
		CreatedAt: now,
		UpdatedAt: now,
	}
	t.record(TripRequested{TripID: t.ID, SeatCount: seatCount, OccurredAt: now})
	return t, nil
}

// transition validates and applies a state change, recording the domain
// event produced. Callers should use the specific methods below rather than
// calling this directly.
func (t *Trip) transition(to State, event Event) error {
	if !CanTransition(t.State, to) {
		return fmt.Errorf("trip: illegal transition %s -> %s", t.State, to)
	}
	t.State = to
	t.UpdatedAt = time.Now().UTC()
	t.record(event)
	return nil
}

func (t *Trip) record(e Event) {
	t.pending = append(t.pending, e)
}

// PendingEvents returns and clears the events accumulated since the last
// call. Repository implementations drain these in the same call that
// persists the trip row, so both land in the outbox atomically.
func (t *Trip) PendingEvents() []Event {
	events := t.pending
	t.pending = nil
	return events
}
