package eventbus

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testEvent struct {
	typ string
}

func (e testEvent) EventType() string { return e.typ }

func TestSubscriptionDeliversToCorrectHandler(t *testing.T) {
	ctx := context.Background()
	bus := New(1, 1)
	defer bus.Close()

	var receivedByA atomic.Bool
	var receivedByB atomic.Bool

	bus.Subscribe("event.A", func(_ context.Context, e Event) error {
		if e.EventType() != "event.A" {
			t.Fatalf("handler for A got %s", e.EventType())
		}
		receivedByA.Store(true)
		return nil
	})
	bus.Subscribe("event.B", func(_ context.Context, e Event) error {
		receivedByB.Store(true)
		return nil
	})

	if err := bus.Publish(ctx, testEvent{typ: "event.A"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	bus.Close()

	if !receivedByA.Load() {
		t.Fatal("handler for event.A was not called")
	}
	if receivedByB.Load() {
		t.Fatal("handler for event.B should not have been called")
	}
}

func TestMultipleHandlersForSameEvent(t *testing.T) {
	ctx := context.Background()
	bus := New(1, 1)
	defer bus.Close()

	var first, second atomic.Bool
	bus.Subscribe("event.X", func(_ context.Context, _ Event) error {
		first.Store(true)
		return nil
	})
	bus.Subscribe("event.X", func(_ context.Context, _ Event) error {
		second.Store(true)
		return nil
	})

	if err := bus.Publish(ctx, testEvent{typ: "event.X"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	bus.Close()

	if !first.Load() || !second.Load() {
		t.Fatalf("expected both handlers to run, got first=%v second=%v", first.Load(), second.Load())
	}
}

func TestPublishBlocksOnFullQueue(t *testing.T) {
	ctx := context.Background()
	bus := New(1, 1)
	defer bus.Close()

	blocker := make(chan struct{})
	bus.Subscribe("slow", func(_ context.Context, _ Event) error {
		<-blocker
		return nil
	})

	// First event is pulled by the single worker and blocks; the one queue slot is free.
	if err := bus.Publish(ctx, testEvent{typ: "slow"}); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	// Fill the lone queue slot so the next Publish must block.
	if err := bus.Publish(ctx, testEvent{typ: "slow"}); err != nil {
		t.Fatalf("second publish: %v", err)
	}

	done := make(chan struct{})
	go func() {
		if err := bus.Publish(ctx, testEvent{typ: "slow"}); err != nil {
			t.Errorf("third publish: %v", err)
		}
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("third publish should have blocked on full queue")
	case <-time.After(100 * time.Millisecond):
	}

	close(blocker)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("blocked publish did not unblock after queue drained")
	}
}

func TestPublishRespectsContextCancellation(t *testing.T) {
	bus := New(1, 1)
	defer bus.Close()

	handlerStarted := make(chan struct{})
	var closeHandlerStarted sync.Once
	blocker := make(chan struct{})
	bus.Subscribe("slow", func(_ context.Context, _ Event) error {
		closeHandlerStarted.Do(func() { close(handlerStarted) })
		<-blocker
		return nil
	})

	// Start one event and wait until the worker has picked it up.
	if err := bus.Publish(context.Background(), testEvent{typ: "slow"}); err != nil {
		t.Fatalf("first publish: %v", err)
	}
	select {
	case <-handlerStarted:
	case <-time.After(time.Second):
		t.Fatal("handler did not start")
	}

	// With the single worker blocked and a queue size of 1, the next event
	// will fill the queue, so the following Publish must block waiting for a slot.
	if err := bus.Publish(context.Background(), testEvent{typ: "slow"}); err != nil {
		t.Fatalf("second publish: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- bus.Publish(ctx, testEvent{typ: "slow"})
	}()

	cancel()

	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %T: %v", err, err)
		}
	case <-time.After(time.Second):
		t.Fatal("Publish did not return after context cancellation")
	}

	// Unblock the background worker so Close() can drain cleanly.
	close(blocker)
}

func TestHandlerErrorCallsErrorHandlerAndContinuesSiblings(t *testing.T) {
	ctx := context.Background()

	var mu sync.Mutex
	var recordedType string
	var recordedErr error
	errorHandler := func(eventType string, err error) {
		mu.Lock()
		defer mu.Unlock()
		recordedType = eventType
		recordedErr = err
	}

	bus := New(1, 1, WithErrorHandler(errorHandler))
	defer bus.Close()

	boomErr := errors.New("boom")
	secondRan := make(chan struct{})
	bus.Subscribe("event.Y", func(_ context.Context, _ Event) error {
		return boomErr
	})
	bus.Subscribe("event.Y", func(_ context.Context, _ Event) error {
		close(secondRan)
		return nil
	})

	if err := bus.Publish(ctx, testEvent{typ: "event.Y"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	bus.Close()

	select {
	case <-secondRan:
	case <-time.After(time.Second):
		t.Fatal("second sibling handler did not run after first handler errored")
	}

	mu.Lock()
	defer mu.Unlock()
	if recordedType != "event.Y" {
		t.Fatalf("expected error handler to receive event type event.Y, got %q", recordedType)
	}
	if recordedErr != boomErr {
		t.Fatalf("expected error handler to receive %v, got %v", boomErr, recordedErr)
	}
}

func TestCloseDrainsInFlightWork(t *testing.T) {
	ctx := context.Background()
	bus := New(1, 1)

	done := make(chan struct{})
	bus.Subscribe("event.Z", func(_ context.Context, _ Event) error {
		defer close(done)
		time.Sleep(50 * time.Millisecond)
		return nil
	})

	if err := bus.Publish(ctx, testEvent{typ: "event.Z"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	bus.Close()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("in-flight handler was not drained before Close returned")
	}
}

func TestDoubleCloseIsNoOp(t *testing.T) {
	bus := New(1, 1)
	bus.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		bus.Close()
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("second Close call deadlocked")
	}
}

func TestDispatchRunsHandlerSynchronously(t *testing.T) {
	ctx := context.Background()
	bus := New(1, 1)
	defer bus.Close()

	var ran atomic.Bool
	bus.Subscribe("event.sync", func(_ context.Context, _ Event) error {
		time.Sleep(20 * time.Millisecond)
		ran.Store(true)
		return nil
	})

	if err := bus.Dispatch(ctx, testEvent{typ: "event.sync"}); err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if !ran.Load() {
		t.Fatal("expected the handler to have run before Dispatch returned")
	}
}

func TestDispatchReturnsHandlerError(t *testing.T) {
	ctx := context.Background()
	bus := New(1, 1)
	defer bus.Close()

	boomErr := errors.New("boom")
	bus.Subscribe("event.err", func(_ context.Context, _ Event) error {
		return boomErr
	})

	err := bus.Dispatch(ctx, testEvent{typ: "event.err"})
	if !errors.Is(err, boomErr) {
		t.Fatalf("expected %v, got %v", boomErr, err)
	}
}

func TestDispatchRunsAllSiblingsEvenIfOneErrors(t *testing.T) {
	ctx := context.Background()
	bus := New(1, 1)
	defer bus.Close()

	var secondRan atomic.Bool
	bus.Subscribe("event.siblings", func(_ context.Context, _ Event) error {
		return errors.New("boom")
	})
	bus.Subscribe("event.siblings", func(_ context.Context, _ Event) error {
		secondRan.Store(true)
		return nil
	})

	if err := bus.Dispatch(ctx, testEvent{typ: "event.siblings"}); err == nil {
		t.Fatal("expected the first handler's error to surface")
	}
	if !secondRan.Load() {
		t.Fatal("expected the second sibling handler to still run")
	}
}

func TestDispatchAfterCloseReturnsErrBusClosed(t *testing.T) {
	ctx := context.Background()
	bus := New(1, 1)
	bus.Close()

	err := bus.Dispatch(ctx, testEvent{typ: "anything"})
	if !errors.Is(err, ErrBusClosed) {
		t.Fatalf("expected ErrBusClosed, got %T: %v", err, err)
	}
}

func TestPublishCloseRaceDoesNotPanic(t *testing.T) {
	for iter := 0; iter < 200; iter++ {
		bus := New(4, 4)

		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("Publish panicked racing Close: %v", r)
					}
				}()
				if err := bus.Publish(context.Background(), testEvent{typ: "race"}); err != nil && !errors.Is(err, ErrBusClosed) {
					t.Errorf("unexpected Publish error: %v", err)
				}
			}()
		}

		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Close()
		}()

		wg.Wait()
	}
}

func TestPublishAfterCloseReturnsErrBusClosed(t *testing.T) {
	ctx := context.Background()
	bus := New(1, 1)
	bus.Close()

	err := bus.Publish(ctx, testEvent{typ: "anything"})
	if !errors.Is(err, ErrBusClosed) {
		t.Fatalf("expected ErrBusClosed, got %T: %v", err, err)
	}
}
