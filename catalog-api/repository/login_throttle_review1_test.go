package repository

// Tests adopted from the independent review of CAT-LOGIN-LOCK-2 (round 1, findings F1/F2).

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// R1-T1: an account that is blocked while every in-flight slot is taken must answer the block's remaining time, not the
// in-flight Base delay (kills the "first account check removed" mutant M04 that the author recorded as equivalent).
func TestInvariant_R1_BlockedAccountWithFullSlotsAnswersBlockEnd(t *testing.T) {
	w := newWorld(t, "u:1")
	ctx := context.Background()
	var held []Decision
	for i := 0; i < 4; i++ {
		d, err := w.th.Begin(ctx, "u:1", fmt.Sprintf("10.1.0.%d", i))
		require.NoError(t, err)
		require.True(t, d.Allowed)
		held = append(held, d)
	}
	_, err := w.raw.Exec(`INSERT OR REPLACE INTO login_throttle (subject, scope, failures, blocked_until_ms, last_failure_ms, version) VALUES ('u:1','account',20,?,?,9)`,
		w.now.Add(10*time.Minute).UnixMilli(), w.now.UnixMilli())
	require.NoError(t, err)
	d, err := w.th.Begin(ctx, "u:1", "10.1.1.1")
	require.NoError(t, err)
	require.False(t, d.Allowed)
	assert.Equal(t, 10*time.Minute, d.RetryAfter, "a blocked account answers the end of its block")
	_ = held
}

// R1-T2: the in-flight bound holds under real goroutine concurrency (not only for a sequential loop).
func TestInvariant_R1_InflightBoundHoldsUnderConcurrency(t *testing.T) {
	for round := 0; round < 20; round++ {
		w := newWorld(t, "u:1")
		ctx := context.Background()
		var wg sync.WaitGroup
		var allowed atomic.Int32
		start := make(chan struct{})
		for i := 0; i < 40; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				d, err := w.th.Begin(ctx, "u:1", fmt.Sprintf("10.2.%d.%d", round, i))
				if err == nil && d.Allowed {
					allowed.Add(1)
				}
			}(i)
		}
		close(start)
		wg.Wait()
		require.LessOrEqual(t, int(allowed.Load()), 4, "round %d: in-flight tokens exceed MaxInflight", round)
		require.GreaterOrEqual(t, int(allowed.Load()), 1)
	}
}

// R1-T3: an owner success from a new address while the account is below the threshold leaves the account row exactly as an
// attacker left it (no reset, no decay-timer refresh). Compares the full account row of a real and a ghost world.
func TestInvariant_R1_OwnerSuccessBelowThresholdLeavesAccountRowIdentical(t *testing.T) {
	real, ghost := newWorld(t, "u:1"), newWorld(t, "n:ghost")
	ctx := context.Background()
	for i := 0; i < 7; i++ {
		real.probe(t, fmt.Sprintf("10.3.0.%d", i))
		ghost.probe(t, fmt.Sprintf("10.3.0.%d", i))
	}
	*real.now = real.now.Add(time.Hour)
	*ghost.now = ghost.now.Add(time.Hour)
	d, err := real.th.Begin(ctx, "u:1", "203.0.113.50")
	require.NoError(t, err)
	require.True(t, d.Allowed)
	require.NoError(t, real.th.Success(ctx, "u:1", "203.0.113.50", false, d.Attempt))
	require.NoError(t, real.th.Success(ctx, "u:1", "203.0.113.50", true, Decision{}.Attempt))
	f1, b1, l1, _ := accountRow(t, real.raw, "u:1")
	f2, b2, l2, _ := accountRow(t, ghost.raw, "n:ghost")
	assert.Equal(t, []any{f2, b2, l2}, []any{f1, b1, l1})
	// and the long-run trace: 25h later (past AccountDecay) and within it
	for _, step := range []time.Duration{0, 23 * time.Hour} {
		*real.now = real.now.Add(step)
		*ghost.now = ghost.now.Add(step)
		for i := 0; i < 15; i++ {
			a := fmt.Sprintf("10.4.%d.%d", int(step/time.Hour), i)
			require.Equal(t, ghost.probe(t, a), real.probe(t, a), "probe %d after %v", i, step)
		}
	}
}
