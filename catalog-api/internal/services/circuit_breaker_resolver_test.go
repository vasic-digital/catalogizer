package services

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"digital.vasic.assets/pkg/resolver"
)

// stubResolver lets tests control CanResolve + Resolve outcomes.
type stubResolver struct {
	name     string
	priority int
	canFn    func() bool
	resolve  func() (*resolver.ResolveResult, error)
	calls    int
}

func (s *stubResolver) Name() string     { return s.name }
func (s *stubResolver) Priority() int    { return s.priority }
func (s *stubResolver) CanResolve(_ context.Context, _ *resolver.ResolveRequest) bool {
	if s.canFn != nil {
		return s.canFn()
	}
	return true
}
func (s *stubResolver) Resolve(_ context.Context, _ *resolver.ResolveRequest) (*resolver.ResolveResult, error) {
	s.calls++
	if s.resolve != nil {
		return s.resolve()
	}
	return &resolver.ResolveResult{Content: io.NopCloser(strings.NewReader("ok"))}, nil
}

// TestCircuitBreakerResolver_P2_OpensAfterConsecutiveFailures locks
// in P2 from docs/nexus/remaining-work.md: a slow upstream tripping
// N consecutive errors moves the breaker to open, and subsequent
// Resolve calls short-circuit without hitting the inner resolver.
func TestCircuitBreakerResolver_P2_OpensAfterConsecutiveFailures(t *testing.T) {
	inner := &stubResolver{
		name: "sick-provider",
		resolve: func() (*resolver.ResolveResult, error) {
			return nil, errors.New("upstream 500")
		},
	}
	br := NewCircuitBreakerResolver(inner,
		WithCircuitBreakerFailureLimit(3),
		WithCircuitBreakerOpenFor(2*time.Second),
	)

	// Three failures trip the breaker.
	for i := 0; i < 3; i++ {
		if _, err := br.Resolve(context.Background(), &resolver.ResolveRequest{}); err == nil {
			t.Fatalf("iteration %d: expected inner error", i)
		}
	}
	if inner.calls != 3 {
		t.Fatalf("inner calls = %d, want 3", inner.calls)
	}

	// Fourth call must short-circuit with ErrCircuitBreakerOpen.
	_, err := br.Resolve(context.Background(), &resolver.ResolveRequest{})
	if err == nil || !errors.Is(err, ErrCircuitBreakerOpen) {
		t.Fatalf("expected ErrCircuitBreakerOpen, got %v", err)
	}
	if inner.calls != 3 {
		t.Errorf("inner must NOT be called while breaker is open, calls=%d", inner.calls)
	}

	// CanResolve must return false while the breaker is open so the
	// chain advances past this provider.
	if br.CanResolve(context.Background(), &resolver.ResolveRequest{}) {
		t.Error("CanResolve must return false while breaker open")
	}
}

// TestCircuitBreakerResolver_P2_ClosesAfterCooldown covers the
// recovery path — after the open window elapses, the next call is
// allowed through.
func TestCircuitBreakerResolver_P2_ClosesAfterCooldown(t *testing.T) {
	failing := true
	inner := &stubResolver{
		name: "recovers",
		resolve: func() (*resolver.ResolveResult, error) {
			if failing {
				return nil, errors.New("boom")
			}
			return &resolver.ResolveResult{Content: io.NopCloser(strings.NewReader("ok"))}, nil
		},
	}
	br := NewCircuitBreakerResolver(inner,
		WithCircuitBreakerFailureLimit(2),
		WithCircuitBreakerOpenFor(50*time.Millisecond),
	)
	_, _ = br.Resolve(context.Background(), &resolver.ResolveRequest{})
	_, _ = br.Resolve(context.Background(), &resolver.ResolveRequest{}) // trip

	// Wait for the open window to elapse.
	time.Sleep(100 * time.Millisecond)
	failing = false

	res, err := br.Resolve(context.Background(), &resolver.ResolveRequest{})
	if err != nil {
		t.Fatalf("after cooldown, call must succeed: %v", err)
	}
	if res == nil {
		t.Fatal("expected non-nil result")
	}
}

// TestCircuitBreakerResolver_P2_SuccessResetsCounter guards against
// a stuck consec-fail counter that would trip the breaker even when
// the provider is healthy.
func TestCircuitBreakerResolver_P2_SuccessResetsCounter(t *testing.T) {
	callCount := 0
	inner := &stubResolver{
		name: "flaky",
		resolve: func() (*resolver.ResolveResult, error) {
			callCount++
			if callCount%3 == 0 {
				return nil, errors.New("blip")
			}
			return &resolver.ResolveResult{Content: io.NopCloser(strings.NewReader("ok"))}, nil
		},
	}
	br := NewCircuitBreakerResolver(inner,
		WithCircuitBreakerFailureLimit(2),
	)
	for i := 0; i < 10; i++ {
		_, _ = br.Resolve(context.Background(), &resolver.ResolveRequest{})
	}
	// With 1/3 failure rate and limit=2, the counter resets before
	// tripping; breaker must still be closed.
	if br.isOpen() {
		t.Error("occasional failures must not trip a breaker with limit=2")
	}
}
