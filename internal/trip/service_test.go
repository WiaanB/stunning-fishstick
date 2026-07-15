package trip

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

// fakeRepo is a hand-rolled Repository double. Save records whatever trip
// and events it was given, clearing them only after the simulated persist
// succeeds (the way a real implementation would clear only after its
// transaction commits) so tests can assert on what got persisted.
type fakeRepo struct {
	inProgressSeats int

	findByIDTrip *Trip
	findByIDErr  error

	saveErr     error
	savedTrip   *Trip
	savedEvents []Event
}

func (f *fakeRepo) Save(ctx context.Context, t *Trip) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.savedTrip = t
	f.savedEvents = t.Events()
	t.ClearEvents()
	return nil
}

func (f *fakeRepo) FindByID(ctx context.Context, id uuid.UUID) (*Trip, error) {
	if f.findByIDErr != nil {
		return nil, f.findByIDErr
	}
	return f.findByIDTrip, nil
}

func (f *fakeRepo) InProgressSeatCount(ctx context.Context, vehicleID uuid.UUID) (int, error) {
	return f.inProgressSeats, nil
}

func TestOccupancyFloorInvariant(t *testing.T) {
	ctx := context.Background()
	vehicleID := uuid.New()
	svc := NewService(&fakeRepo{inProgressSeats: 3})

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

func TestRequestTrip(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{}
	svc := NewService(repo)

	tr, err := svc.RequestTrip(ctx, 2)
	if err != nil {
		t.Fatalf("RequestTrip: %v", err)
	}
	if tr.State != StateRequested {
		t.Fatalf("expected Requested, got %s", tr.State)
	}
	if repo.savedTrip != tr {
		t.Fatal("expected the new trip to be saved")
	}
	if len(repo.savedEvents) != 1 {
		t.Fatalf("expected 1 saved event, got %d", len(repo.savedEvents))
	}
	if got := repo.savedEvents[0].EventType(); got != "trip.requested" {
		t.Fatalf("expected trip.requested event, got %s", got)
	}
	// Save already cleared the events, so a second read must be empty.
	if got := len(tr.Events()); got != 0 {
		t.Fatalf("expected pending events already drained, got %d", got)
	}
}

func TestMarkNoShow(t *testing.T) {
	ctx := context.Background()
	tr, err := NewTrip(1)
	if err != nil {
		t.Fatalf("NewTrip: %v", err)
	}
	_ = tr.Quote(1000)
	_ = tr.AwaitPayment()
	_ = tr.IssueCode("A1B2")
	_ = tr.AssignDriver(uuid.New())
	tr.ClearEvents() // drain setup events, isolate the assertions below to MarkNoShow

	repo := &fakeRepo{findByIDTrip: tr}
	svc := NewService(repo)

	if err := svc.MarkNoShow(ctx, tr.ID); err != nil {
		t.Fatalf("MarkNoShow: %v", err)
	}
	if tr.State != StateNoShow {
		t.Fatalf("expected NoShow, got %s", tr.State)
	}
	if repo.savedTrip != tr {
		t.Fatal("expected the trip to be saved")
	}
	if len(repo.savedEvents) != 1 || repo.savedEvents[0].EventType() != "trip.no_show" {
		t.Fatalf("expected 1 trip.no_show event, got %v", repo.savedEvents)
	}
}

func TestMarkNoShowFindByIDError(t *testing.T) {
	ctx := context.Background()
	repo := &fakeRepo{findByIDErr: errors.New("boom")}
	svc := NewService(repo)

	if err := svc.MarkNoShow(ctx, uuid.New()); err == nil {
		t.Fatal("expected error when FindByID fails")
	}
}

// TestMarkNoShowFailedSaveLeavesEventsForRetry pins the fix for the
// destructive-read bug: a Save failure must not have drained tr's events,
// so retrying MarkNoShow against the same in-memory aggregate can still
// persist the trip.no_show event rather than silently losing it.
func TestMarkNoShowFailedSaveLeavesEventsForRetry(t *testing.T) {
	ctx := context.Background()
	tr, err := NewTrip(1)
	if err != nil {
		t.Fatalf("NewTrip: %v", err)
	}
	_ = tr.Quote(1000)
	_ = tr.AwaitPayment()
	_ = tr.IssueCode("A1B2")
	_ = tr.AssignDriver(uuid.New())
	tr.ClearEvents() // drain setup events, isolate the assertions below to MarkNoShow

	repo := &fakeRepo{findByIDTrip: tr, saveErr: errors.New("boom")}
	svc := NewService(repo)

	if err := svc.MarkNoShow(ctx, tr.ID); err == nil {
		t.Fatal("expected error when Save fails")
	}
	if got := len(tr.Events()); got != 1 || tr.Events()[0].EventType() != "trip.no_show" {
		t.Fatalf("expected the trip.no_show event to survive the failed save for retry, got %v", tr.Events())
	}
}

func TestMarkNoShowIllegalTransitionNotSaved(t *testing.T) {
	ctx := context.Background()
	tr, err := NewTrip(1) // Requested state: MarkNoShow is not a legal transition from here
	if err != nil {
		t.Fatalf("NewTrip: %v", err)
	}
	repo := &fakeRepo{findByIDTrip: tr}
	svc := NewService(repo)

	if err := svc.MarkNoShow(ctx, tr.ID); err == nil {
		t.Fatal("expected error marking no-show from Requested state")
	}
	if repo.savedTrip != nil {
		t.Fatal("expected Save not to be called when the domain transition fails")
	}
}
