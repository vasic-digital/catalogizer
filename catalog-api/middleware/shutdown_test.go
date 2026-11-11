package middleware

import (
	"sync/atomic"
	"testing"
	"time"
)

// resetRegistry clears the package shutdown registry so tests can run in
// isolation from each other and from other tests that touch the registry.
func resetRegistry() {
	stopMu.Lock()
	stopRegistry = nil
	stopMu.Unlock()
}

func registrySize() int {
	stopMu.Lock()
	defer stopMu.Unlock()
	return len(stopRegistry)
}

func TestStopAllInvokesRegisteredStops(t *testing.T) {
	resetRegistry()

	var calls atomic.Int32
	registerStop(func() { calls.Add(1) })
	registerStop(func() { calls.Add(1) })
	registerStop(func() { calls.Add(1) })

	StopAll()

	if got := calls.Load(); got != 3 {
		t.Fatalf("expected 3 stops invoked, got %d", got)
	}
}

func TestStopAllIsIdempotent(t *testing.T) {
	resetRegistry()

	var calls atomic.Int32
	registerStop(func() { calls.Add(1) })

	StopAll()
	StopAll() // second call must not invoke the stop function again
	StopAll()

	if got := calls.Load(); got != 1 {
		t.Fatalf("expected 1 stop invoked, got %d", got)
	}
}

func TestRateLimiterRegistersStop(t *testing.T) {
	resetRegistry()

	_ = RateLimiter(100)

	if n := registrySize(); n != 1 {
		t.Fatalf("expected 1 registered stop after RateLimiter, got %d", n)
	}

	StopAll()

	if n := registrySize(); n != 0 {
		t.Fatalf("expected empty registry after StopAll, got %d", n)
	}

	// Give the goroutine a moment to actually exit.
	time.Sleep(20 * time.Millisecond)
}

