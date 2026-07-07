package trip

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

// Repository is the persistence boundary for trips. The concrete
// implementation (sqlc-generated queries over Postgres, per docs/05
// Roadmap) lives outside this package; the domain only depends on this
// interface.
type Repository interface {
	Save(ctx context.Context, t *Trip) error
	FindByID(ctx context.Context, id uuid.UUID) (*Trip, error)
	// InProgressSeatCount returns the sum of SeatCount across all trips
	// currently InProgress for the given vehicle.
	InProgressSeatCount(ctx context.Context, vehicleID uuid.UUID) (int, error)
}

// EventPublisher hands domain events to the outbox for durable, at-least-
// once dispatch. See internal/platform/postgres/outbox.go.
type EventPublisher interface {
	Publish(ctx context.Context, events ...Event) error
}

// Service is the application-layer entry point for trip commands: it loads
// state, invokes the domain method, and persists the resulting state and
// events atomically (repository implementations are expected to wrap
// Save + Publish in a single DB transaction).
type Service struct {
	repo   Repository
	events EventPublisher
}

func NewService(repo Repository, events EventPublisher) *Service {
	return &Service{repo: repo, events: events}
}

// OccupancyFloorError is returned when a manual occupancy adjustment would
// drop a vehicle's occupancy below the seats already committed to
// InProgress trips. See docs/03 "Key domain invariant".
type OccupancyFloorError struct {
	VehicleID    uuid.UUID
	Requested    int
	CommittedMin int
}

func (e *OccupancyFloorError) Error() string {
	return fmt.Sprintf(
		"trip: cannot set occupancy to %d for vehicle %s, below committed floor of %d in-progress seats; mark unused seats NoShow first",
		e.Requested, e.VehicleID, e.CommittedMin,
	)
}

// AdjustOccupancy validates a driver's manual occupancy decrement against
// the occupancy floor invariant: total occupancy can never drop below the
// sum of seat_count across all InProgress trips booked for that vehicle.
// Callers must mark no-show seats via Trip.MarkNoShow before the floor
// will move down.
func (s *Service) AdjustOccupancy(ctx context.Context, vehicleID uuid.UUID, newOccupancy int) error {
	if newOccupancy < 0 {
		return fmt.Errorf("trip: occupancy cannot be negative, got %d", newOccupancy)
	}
	floor, err := s.repo.InProgressSeatCount(ctx, vehicleID)
	if err != nil {
		return fmt.Errorf("trip: load committed seat count: %w", err)
	}
	if newOccupancy < floor {
		return &OccupancyFloorError{VehicleID: vehicleID, Requested: newOccupancy, CommittedMin: floor}
	}
	return nil
}

// apply persists the trip and dispatches its pending events as a single
// unit of work.
func (s *Service) apply(ctx context.Context, t *Trip) error {
	events := t.PendingEvents()
	if err := s.repo.Save(ctx, t); err != nil {
		return fmt.Errorf("trip: save: %w", err)
	}
	if len(events) > 0 {
		if err := s.events.Publish(ctx, events...); err != nil {
			return fmt.Errorf("trip: publish events: %w", err)
		}
	}
	return nil
}

// RequestTrip starts a new trip for the given seat count.
func (s *Service) RequestTrip(ctx context.Context, seatCount int) (*Trip, error) {
	t, err := NewTrip(seatCount)
	if err != nil {
		return nil, err
	}
	if err := s.apply(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// MarkNoShow loads a trip, marks it NoShow, and persists the result. This
// is the required step before a vehicle's occupancy floor can move down
// for that trip's seats.
func (s *Service) MarkNoShow(ctx context.Context, tripID uuid.UUID) error {
	t, err := s.repo.FindByID(ctx, tripID)
	if err != nil {
		return fmt.Errorf("trip: find %s: %w", tripID, err)
	}
	if err := t.MarkNoShow(); err != nil {
		return err
	}
	return s.apply(ctx, t)
}
