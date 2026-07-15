package trip

import (
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
	if got, want := len(tr.Events()), len(steps)+1; got != want { // +1 for the initial TripRequested event
		t.Fatalf("expected %d pending events, got %d", want, got)
	}
}

// TestEventsDoesNotClearPending pins that Events() is a non-destructive peek:
// a caller that reads events before persistence and then fails to commit must
// still see them on the next read, so a retried save doesn't silently drop
// them.
func TestEventsDoesNotClearPending(t *testing.T) {
	tr, err := NewTrip(1)
	if err != nil {
		t.Fatalf("NewTrip: %v", err)
	}
	first := tr.Events()
	second := tr.Events()
	if len(first) != 1 || len(second) != 1 {
		t.Fatalf("expected Events() to return the same pending events on repeated calls, got %d then %d", len(first), len(second))
	}
}

func TestClearEventsEmptiesPending(t *testing.T) {
	tr, err := NewTrip(1)
	if err != nil {
		t.Fatalf("NewTrip: %v", err)
	}
	tr.ClearEvents()
	if got := len(tr.Events()); got != 0 {
		t.Fatalf("expected no pending events after ClearEvents, got %d", got)
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

	err := tr.VerifyCode("WRONG")
	var mismatchErr *CodeMismatchError
	if !errors.As(err, &mismatchErr) {
		t.Fatalf("expected *CodeMismatchError, got %T: %v", err, err)
	}
	if tr.State != StateEnRoute {
		t.Fatalf("state should be unchanged after failed verification, got %s", tr.State)
	}
}

// TestVerifyCodeIllegalStateTakesPrecedence pins down that VerifyCode checks
// CanTransition before comparing codes, consistent with every other
// transition method — an illegal-state call surfaces as a transition error
// even when the presented code is also wrong, rather than masking the real
// problem behind a CodeMismatchError.
func TestVerifyCodeIllegalStateTakesPrecedence(t *testing.T) {
	tr, err := NewTrip(1)
	if err != nil {
		t.Fatalf("NewTrip: %v", err)
	}
	// Fresh trip is Requested; VerifyCode (-> CodeVerified) is illegal from
	// here regardless of the code, and no code has been issued yet either.
	err = tr.VerifyCode("WRONG")
	if err == nil {
		t.Fatal("expected an error verifying code from Requested state")
	}
	var mismatchErr *CodeMismatchError
	if errors.As(err, &mismatchErr) {
		t.Fatalf("expected a transition error, not *CodeMismatchError: %v", err)
	}
	if tr.State != StateRequested {
		t.Fatalf("state should be unchanged, got %s", tr.State)
	}
}
