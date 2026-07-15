package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestPingSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := ping(context.Background(), srv.Client(), srv.URL); err != nil {
		t.Fatalf("ping: %v", err)
	}
}

func TestPingNonOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	if err := ping(context.Background(), srv.Client(), srv.URL); err == nil {
		t.Fatal("expected an error for a non-200 response")
	}
}

func TestPingTransportError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	client := srv.Client()
	addr := srv.URL
	srv.Close() // nothing is listening at addr anymore

	if err := ping(context.Background(), client, addr); err == nil {
		t.Fatal("expected a transport error against a closed server")
	}
}

func TestLoopTicksUntilCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ticks := make(chan struct{}, 10)
	done := make(chan struct{})

	go func() {
		loop(ctx, time.Millisecond, func() { ticks <- struct{}{} })
		close(done)
	}()

	for i := 0; i < 3; i++ {
		select {
		case <-ticks:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for a tick")
		}
	}
	cancel()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("loop did not return after context cancellation")
	}
}

func TestLoopRespectsAlreadyCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	done := make(chan struct{})
	go func() {
		loop(ctx, time.Hour, func() { called = true })
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("loop did not return immediately for an already-cancelled context")
	}
	if called {
		t.Fatal("action should not run once the context is already cancelled")
	}
}

func TestSimulatePassengerPingsHealthz(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			atomic.AddInt32(&hits, 1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	simulatePassenger(ctx, srv.Client(), srv.URL, 1, 5*time.Millisecond)

	if atomic.LoadInt32(&hits) == 0 {
		t.Fatal("expected at least one /healthz hit")
	}
}

func TestSimulateDriverPingsHealthz(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			atomic.AddInt32(&hits, 1)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	simulateDriver(ctx, srv.Client(), srv.URL, 1, 5*time.Millisecond)

	if atomic.LoadInt32(&hits) == 0 {
		t.Fatal("expected at least one /healthz hit")
	}
}
