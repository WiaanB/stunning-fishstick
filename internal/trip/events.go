package trip

import (
	"time"

	"github.com/google/uuid"
)

// Event is a domain event emitted by a Trip as it moves through its state
// machine. Concrete events carry only the data needed to reconstruct what
// happened; the outbox persists them as JSON keyed by EventType().
type Event interface {
	EventType() string
	AggregateID() uuid.UUID
}

type TripRequested struct {
	TripID     uuid.UUID
	SeatCount  int
	OccurredAt time.Time
}

func (e TripRequested) EventType() string      { return "trip.requested" }
func (e TripRequested) AggregateID() uuid.UUID { return e.TripID }

type TripQuoted struct {
	TripID     uuid.UUID
	FareCents  int64
	OccurredAt time.Time
}

func (e TripQuoted) EventType() string      { return "trip.quoted" }
func (e TripQuoted) AggregateID() uuid.UUID { return e.TripID }

type TripPaymentAwaited struct {
	TripID     uuid.UUID
	OccurredAt time.Time
}

func (e TripPaymentAwaited) EventType() string      { return "trip.payment_awaited" }
func (e TripPaymentAwaited) AggregateID() uuid.UUID { return e.TripID }

type TripCodeIssued struct {
	TripID     uuid.UUID
	Code       string
	OccurredAt time.Time
}

func (e TripCodeIssued) EventType() string      { return "trip.code_issued" }
func (e TripCodeIssued) AggregateID() uuid.UUID { return e.TripID }

type TripDriverAssigned struct {
	TripID     uuid.UUID
	VehicleID  uuid.UUID
	OccurredAt time.Time
}

func (e TripDriverAssigned) EventType() string      { return "trip.driver_assigned" }
func (e TripDriverAssigned) AggregateID() uuid.UUID { return e.TripID }

type TripEnRoute struct {
	TripID     uuid.UUID
	OccurredAt time.Time
}

func (e TripEnRoute) EventType() string      { return "trip.en_route" }
func (e TripEnRoute) AggregateID() uuid.UUID { return e.TripID }

type TripCodeVerified struct {
	TripID     uuid.UUID
	OccurredAt time.Time
}

func (e TripCodeVerified) EventType() string      { return "trip.code_verified" }
func (e TripCodeVerified) AggregateID() uuid.UUID { return e.TripID }

type TripStarted struct {
	TripID     uuid.UUID
	OccurredAt time.Time
}

func (e TripStarted) EventType() string      { return "trip.in_progress" }
func (e TripStarted) AggregateID() uuid.UUID { return e.TripID }

type TripCompleted struct {
	TripID     uuid.UUID
	OccurredAt time.Time
}

func (e TripCompleted) EventType() string      { return "trip.completed" }
func (e TripCompleted) AggregateID() uuid.UUID { return e.TripID }

type TripCancelled struct {
	TripID     uuid.UUID
	Reason     string
	OccurredAt time.Time
}

func (e TripCancelled) EventType() string      { return "trip.cancelled" }
func (e TripCancelled) AggregateID() uuid.UUID { return e.TripID }

type TripNoShow struct {
	TripID     uuid.UUID
	OccurredAt time.Time
}

func (e TripNoShow) EventType() string      { return "trip.no_show" }
func (e TripNoShow) AggregateID() uuid.UUID { return e.TripID }

// --- Transition methods -----------------------------------------------
//
// Each method validates the move via transition() and records the matching
// event. Keeping them on Trip (rather than free functions) keeps the state
// machine and its event emission colocated, per docs/02 "Command -> domain
// method -> event -> handler".

func (t *Trip) Quote(fareCents int64) error {
	now := time.Now().UTC()
	return t.transition(StateQuoted, TripQuoted{TripID: t.ID, FareCents: fareCents, OccurredAt: now})
}

func (t *Trip) AwaitPayment() error {
	now := time.Now().UTC()
	return t.transition(StateAwaitingPayment, TripPaymentAwaited{TripID: t.ID, OccurredAt: now})
}

func (t *Trip) IssueCode(code string) error {
	now := time.Now().UTC()
	if err := t.transition(StateCodeIssued, TripCodeIssued{TripID: t.ID, Code: code, OccurredAt: now}); err != nil {
		return err
	}
	t.Code = code
	return nil
}

func (t *Trip) AssignDriver(vehicleID uuid.UUID) error {
	now := time.Now().UTC()
	if err := t.transition(StateDriverAssigned, TripDriverAssigned{TripID: t.ID, VehicleID: vehicleID, OccurredAt: now}); err != nil {
		return err
	}
	t.VehicleID = &vehicleID
	return nil
}

func (t *Trip) MarkEnRoute() error {
	now := time.Now().UTC()
	return t.transition(StateEnRoute, TripEnRoute{TripID: t.ID, OccurredAt: now})
}

func (t *Trip) VerifyCode(code string) error {
	if code != t.Code {
		return &CodeMismatchError{TripID: t.ID}
	}
	now := time.Now().UTC()
	return t.transition(StateCodeVerified, TripCodeVerified{TripID: t.ID, OccurredAt: now})
}

func (t *Trip) Start() error {
	now := time.Now().UTC()
	return t.transition(StateInProgress, TripStarted{TripID: t.ID, OccurredAt: now})
}

func (t *Trip) Complete() error {
	now := time.Now().UTC()
	return t.transition(StateCompleted, TripCompleted{TripID: t.ID, OccurredAt: now})
}

func (t *Trip) Cancel(reason string) error {
	now := time.Now().UTC()
	return t.transition(StateCancelled, TripCancelled{TripID: t.ID, Reason: reason, OccurredAt: now})
}

func (t *Trip) MarkNoShow() error {
	now := time.Now().UTC()
	return t.transition(StateNoShow, TripNoShow{TripID: t.ID, OccurredAt: now})
}

// CodeMismatchError is returned by VerifyCode when the presented boarding
// code doesn't match the one issued for the trip.
type CodeMismatchError struct {
	TripID uuid.UUID
}

func (e *CodeMismatchError) Error() string {
	return "trip: boarding code mismatch for trip " + e.TripID.String()
}
