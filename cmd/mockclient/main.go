// cmd/mockclient simulates passenger and driver traffic against the API
// over HTTP, so the backend can be load tested without real mobile apps.
// See taxi-platform/02 Architecture Principles.md. The request/accept
// loops are stubs against /healthz until the trip HTTP handlers land
// (taxi-platform/05 Roadmap.md, item 1); swap simulatePassenger's and
// simulateDriver's request bodies for real trip endpoints as they ship.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	apiAddr := flag.String("api-addr", "http://localhost:8080", "base URL of the API server")
	passengers := flag.Int("passengers", 5, "number of simulated passenger loops")
	drivers := flag.Int("drivers", 3, "number of simulated driver loops")
	interval := flag.Duration("interval", 2*time.Second, "time between simulated actions per client")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	httpClient := &http.Client{Timeout: 5 * time.Second}

	var wg sync.WaitGroup
	for i := 0; i < *passengers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			simulatePassenger(ctx, httpClient, *apiAddr, id, *interval)
		}(i)
	}
	for i := 0; i < *drivers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			simulateDriver(ctx, httpClient, *apiAddr, id, *interval)
		}(i)
	}

	log.Printf("mockclient: running %d passenger(s), %d driver(s) against %s", *passengers, *drivers, *apiAddr)
	wg.Wait()
	log.Println("mockclient: stopped")
}

// simulatePassenger loops requesting trips. Today it just pings /healthz
// on each tick as a placeholder for POST /trips once that handler exists.
func simulatePassenger(ctx context.Context, client *http.Client, apiAddr string, id int, interval time.Duration) {
	loop(ctx, interval, func() {
		if err := ping(ctx, client, apiAddr); err != nil {
			log.Printf("passenger[%d]: %v", id, err)
		}
	})
}

// simulateDriver loops as if polling for trip assignments. Same
// /healthz placeholder as simulatePassenger until GET /trips/assignments
// (or equivalent) exists.
func simulateDriver(ctx context.Context, client *http.Client, apiAddr string, id int, interval time.Duration) {
	loop(ctx, interval, func() {
		if err := ping(ctx, client, apiAddr); err != nil {
			log.Printf("driver[%d]: %v", id, err)
		}
	})
}

func loop(ctx context.Context, interval time.Duration, action func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			action()
		}
	}
}

func ping(ctx context.Context, client *http.Client, apiAddr string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiAddr+"/healthz", nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}
