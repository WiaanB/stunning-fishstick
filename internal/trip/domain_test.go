package trip

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestHappyPathTransitions(t *testing.T) {
	tr, err := NewTrip(2)
	if err != nil {
		t.Fatalf("NewTrip: %v", err)
	}
	steps := []func() error{
		func() error { return tr.Quote(1500) },
		func() error { return tr.AwaitPayment() },
		func() error { return tr.IssueCode("A1B2") },
		func() error { return tr.AssignDriver(uuid.New()) },
		func() error { return tr.MarkEnRoute() },
		func() error { return tr.VerifyCode("A1B2") },
		func() error { return tr.Start() },
		func() error { return tr.Complete() },
	}
	for i, step := range steps {
		if err := step(); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	if tr.State != StateCompleted {
		t.Fatalf("expected Completed, got %s", tr.State)
	}
	if got, want := len(tr.PendingEvents()), len(steps)+1; got != want { // +1 for the initial TripRequested event
		t.Fatalf("expected %d pending events, got %d", want, got)
	}
}

func TestIllegalTransitionRejected(t *testing.T) {
	tr, err := NewTrip(1)
	if err != nil {
		t.Fatalf("NewTrip: %v", err)
	}
	if err := tr.Start(); err == nil {
		t.Fatal("expected error jumping straight to InProgress from Requested")
	}
}

func TestVerifyCodeMismatch(t *testing.T) {
	tr, _ := NewTrip(1)
	_ = tr.Quote(1000)
	_ = tr.AwaitPayment()
	_ = tr.IssueCode("REAL")
	_ = tr.AssignDriver(uuid.New())
	_ = tr.MarkEnRoute()

	if err := tr.VerifyCode("WRONG"); err == nil {
		t.Fatal("expected code mismatch error")
	}
	if tr.State != StateEnRoute {
		t.Fatalf("state should be unchanged after failed verification, got %s", tr.State)
	}
}

// fakeRepo backs the occupancy invariant test; only InProgressSeatCount is
// exercised by Service.AdjustOccupancy.
type fakeRepo struct {
	inProgressSeats int
}

func (f *fakeRepo) Save(ctx context.Context, t *Trip) error { return nil }
func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*Trip, error) {
	return nil, errors.New("not implemented in fakeRepo")
}
func (f *fakeRepo) InProgressSeatCount(ctx context.Context, vehicleID uuid.UUID) (int, error) {
	return f.inProgressSeats, nil
}

type fakePublisher struct{}

func (fakePublisher) Publish(ctx context.Context, events ...Event) error { return nil }

func TestOccupancyFloorInvariant(t *testing.T) {
	ctx := context.Background()
	vehicleID := uuid.New()
	svc := NewService(&fakeRepo{inProgressSeats: 3}, fakePublisher{})

	if err := svc.AdjustOccupancy(ctx, vehicleID, 3); err != nil {
		t.Fatalf("occupancy at the floor should be allowed: %v", err)
	}

	err := svc.AdjustOccupancy(ctx, vehicleID, 2)
	if err == nil {
		t.Fatal("expected OccupancyFloorError dropping below committed in-progress seats")
	}
	var floorErr *OccupancyFloorError
	if !errors.As(err, &floorErr) {
		t.Fatalf("expected *OccupancyFloorError, got %T: %v", err, err)
	}
}
