package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

// The limiter lets a fixed number of actions through and never two for the same
// manager.
func TestActionLimiterCapsConcurrency(t *testing.T) {
	limiter := newActionLimiter(2)
	ctx := context.Background()

	first, err := limiter.acquire(ctx, "brew")
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	second, err := limiter.acquire(ctx, "npm")
	if err != nil {
		t.Fatalf("second acquire: %v", err)
	}

	// A third manager is over the global limit.
	third := make(chan func())
	go func() {
		release, err := limiter.acquire(ctx, "pip")
		if err != nil {
			close(third)
			return
		}
		third <- release
	}()
	select {
	case <-third:
		t.Fatal("a third action started while two were running")
	case <-time.After(50 * time.Millisecond):
	}

	first()
	select {
	case release := <-third:
		if release == nil {
			t.Fatal("the third acquire failed")
		}
		release()
	case <-time.After(time.Second):
		t.Fatal("freeing a slot did not let the next action start")
	}
	second()
}

func TestActionLimiterSerializesOneManager(t *testing.T) {
	limiter := newActionLimiter(3)
	ctx := context.Background()
	release, err := limiter.acquire(ctx, "brew")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	next := make(chan error)
	go func() {
		second, err := limiter.acquire(ctx, "brew")
		if err == nil {
			second()
		}
		next <- err
	}()
	select {
	case err := <-next:
		t.Fatalf("a second action for the same manager started: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	release()
	select {
	case err := <-next:
		if err != nil {
			t.Fatalf("second acquire: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("the manager slot was never freed")
	}
}

// Waiters for a held manager do not consume the global slots other managers
// need. This is the difference between a single-manager batch (one active task,
// many waiters) and a batch that starves every other manager.
func TestActionLimiterDoesNotStarveOtherManagers(t *testing.T) {
	limiter := newActionLimiter(2)
	release, err := limiter.acquire(context.Background(), "brew")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	for range 6 {
		go func() {
			release, err := limiter.acquire(context.Background(), "brew")
			if err == nil {
				release()
			}
		}()
	}
	// Let the waiters reach the manager slot before asking for another manager.
	time.Sleep(30 * time.Millisecond)

	npm := make(chan error, 1)
	go func() {
		release, err := limiter.acquire(context.Background(), "npm")
		if err == nil {
			release()
		}
		npm <- err
	}()
	select {
	case err := <-npm:
		if err != nil {
			t.Fatalf("npm acquire: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("waiters for one manager exhausted the global slots")
	}
	release()
}

// A cancelled wait must give back the global slot it took.
func TestActionLimiterReleasesOnCancel(t *testing.T) {
	limiter := newActionLimiter(1)
	release, err := limiter.acquire(context.Background(), "brew")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error)
	go func() {
		_, err := limiter.acquire(ctx, "npm")
		done <- err
	}()
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled wait = %v, want context.Canceled", err)
	}

	release()
	again, err := limiter.acquire(context.Background(), "pip")
	if err != nil {
		t.Fatalf("the global slot was not released: %v", err)
	}
	again()
}
