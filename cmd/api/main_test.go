package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"taxi-platform/internal/platform/eventbus"
	"taxi-platform/internal/platform/postgres"
)

// fakePinger is a hand-rolled double for the structural Ping interface
// healthHandler depends on, so it's testable without a real *pgxpool.Pool.
type fakePinger struct {
	err error
}

func (f fakePinger) Ping(ctx context.Context) error { return f.err }

func TestHealthHandlerOK(t *testing.T) {
	h := healthHandler(fakePinger{})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	h(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "ok" {
		t.Fatalf("expected status \"ok\", got %v", body)
	}
}

func TestHealthHandlerUnavailable(t *testing.T) {
	h := healthHandler(fakePinger{err: errors.New("db down")})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)

	h(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected status 503, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "unavailable" {
		t.Fatalf("expected status \"unavailable\", got %v", body)
	}
}

// --- awaitShutdown ------------------------------------------------------

func TestAwaitShutdownOnServerError(t *testing.T) {
	wantErr := errors.New("listen boom")
	serverErrs := make(chan error, 1)
	serverErrs <- wantErr
	dispatcherErrs := make(chan error, 1)

	shutdownCalled := false
	shutdown := func(context.Context) error { shutdownCalled = true; return nil }

	err := awaitShutdown(context.Background(), shutdown, serverErrs, dispatcherErrs)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
	if shutdownCalled {
		t.Fatal("shutdown should not be called when the server errors first")
	}
}

func TestAwaitShutdownOnDispatcherError(t *testing.T) {
	wantErr := errors.New("dispatcher boom")
	serverErrs := make(chan error, 1)
	dispatcherErrs := make(chan error, 1)
	dispatcherErrs <- wantErr

	shutdown := func(context.Context) error { return nil }

	err := awaitShutdown(context.Background(), shutdown, serverErrs, dispatcherErrs)
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected %v, got %v", wantErr, err)
	}
}

func TestAwaitShutdownGracefulOnContextDone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	serverErrs := make(chan error)
	dispatcherErrs := make(chan error, 1)
	shutdownCalled := make(chan struct{})

	shutdown := func(context.Context) error {
		close(shutdownCalled)
		return nil
	}
	// Only send the dispatcher's cancellation error after shutdown actually
	// ran, so the select in awaitShutdown has just one ready case (ctx.Done)
	// when it starts — otherwise a preloaded dispatcherErrs would race with
	// ctx.Done() and make this test flaky.
	go func() {
		<-shutdownCalled
		dispatcherErrs <- nil
	}()

	done := make(chan error, 1)
	go func() { done <- awaitShutdown(ctx, shutdown, serverErrs, dispatcherErrs) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("awaitShutdown did not return")
	}
	select {
	case <-shutdownCalled:
	default:
		t.Fatal("expected shutdown to be called")
	}
}

func TestAwaitShutdownPropagatesShutdownErrorWithoutWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	wantErr := errors.New("shutdown boom")
	serverErrs := make(chan error)
	dispatcherErrs := make(chan error) // never sent to: proves this path doesn't wait on it

	shutdown := func(context.Context) error { return wantErr }

	done := make(chan error, 1)
	go func() { done <- awaitShutdown(ctx, shutdown, serverErrs, dispatcherErrs) }()

	select {
	case err := <-done:
		if !errors.Is(err, wantErr) {
			t.Fatalf("expected %v, got %v", wantErr, err)
		}
	case <-time.After(time.Second):
		t.Fatal("awaitShutdown blocked instead of returning the shutdown error")
	}
}

// --- closeInOrder ---------------------------------------------------------

type recordingCloser struct {
	name  string
	order *[]string
}

func (c recordingCloser) Close() { *c.order = append(*c.order, c.name) }

func TestCloseInOrderClosesSequentially(t *testing.T) {
	var order []string
	closeInOrder(recordingCloser{"bus", &order}, recordingCloser{"pool", &order})

	if len(order) != 2 || order[0] != "bus" || order[1] != "pool" {
		t.Fatalf("expected [bus pool], got %v", order)
	}
}

// --- getEnvOrDefault -------------------------------------------------------

// TestPublishToBusWaitsForHandlerCompletion pins that publishToBus hands
// records to the bus synchronously (via Dispatch), not fire-and-forget (via
// Publish) — the outbox dispatcher only acks a row once its PublishFunc
// returns, so an async hand-off here would recreate the ack-before-handler
// bug even though pollOnce itself is now correct.
func TestPublishToBusWaitsForHandlerCompletion(t *testing.T) {
	bus := eventbus.New(1, 1)
	defer bus.Close()

	var handlerRan atomic.Bool
	bus.Subscribe("trip.requested", func(_ context.Context, _ eventbus.Event) error {
		time.Sleep(20 * time.Millisecond)
		handlerRan.Store(true)
		return nil
	})

	publish := publishToBus(bus)
	if err := publish(context.Background(), postgres.OutboxRecord{EventType: "trip.requested"}); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if !handlerRan.Load() {
		t.Fatal("expected the handler to have run before publishToBus's PublishFunc returned")
	}
}

func TestGetEnvOrDefaultUnset(t *testing.T) {
	t.Setenv("API_TEST_VAR", "")
	if got := getEnvOrDefault("API_TEST_VAR", "fallback"); got != "fallback" {
		t.Fatalf("expected fallback, got %q", got)
	}
}

func TestGetEnvOrDefaultSet(t *testing.T) {
	t.Setenv("API_TEST_VAR", "value")
	if got := getEnvOrDefault("API_TEST_VAR", "fallback"); got != "value" {
		t.Fatalf("expected value, got %q", got)
	}
}
