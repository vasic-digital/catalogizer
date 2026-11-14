package repository

// Reviewer round-4 tests (not part of the change under review). Unit-class (in-memory SQLite, injected clock).

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// RV4-M7: each charge on an already blocked account sets the block end to exactly now + delay(new count) (same instant).
func TestRV4_BlockedChargeExtendsBlockByDoubling(t *testing.T) {
	w := newWorld(t, "u:1")
	ctx := context.Background()
	pol := w.th.pol
	for i := 0; i < pol.AccountFree; i++ {
		require.Equal(t, "evaluated", w.probe(t, fmt.Sprintf("10.9.0.%d", i)))
	}
	_, b0, _, _ := accountRow(t, w.raw, "u:1")
	*w.now = time.UnixMilli(b0 + 1)
	var open []Decision
	for i := 0; i < pol.MaxInflight; i++ {
		d, err := w.th.Begin(ctx, "u:1", fmt.Sprintf("10.9.6.%d", i))
		require.NoError(t, err)
		require.True(t, d.Allowed)
		open = append(open, d)
	}
	for k, d := range open {
		require.NoError(t, w.th.Fail(ctx, "u:1", d.Attempt))
		f, bl, _, _ := accountRow(t, w.raw, "u:1")
		want := w.now.UnixMilli() + delayFor(f, pol.AccountFree, pol.Base, pol.Cap).Milliseconds()
		require.Equal(t, want, bl, "charge %d (failures %d): block end = now + doubling delay", k, f)
	}
}

// RV4-M1: with AccountDecay shorter than the block, a charge on a blocked account whose count decayed must not end the block.
func TestRV4_BlockedChargeNeverShortensAfterDecay(t *testing.T) {
	w := newWorld(t, "u:1")
	ctx := context.Background()
	w.th.SetPolicy(LoginThrottlePolicy{Base: 10 * time.Minute, Cap: 15 * time.Minute, AccountDecay: time.Minute})
	pol := w.th.pol
	for i := 0; i < pol.AccountFree-1; i++ {
		require.Equal(t, "evaluated", w.probe(t, fmt.Sprintf("10.7.0.%d", i)))
	}
	a, err := w.th.Begin(ctx, "u:1", "10.7.1.1")
	require.NoError(t, err)
	b, err := w.th.Begin(ctx, "u:1", "10.7.1.2")
	require.NoError(t, err)
	require.True(t, a.Allowed && b.Allowed)
	require.NoError(t, w.th.Fail(ctx, "u:1", a.Attempt))
	_, bl0, _, _ := accountRow(t, w.raw, "u:1")
	require.Greater(t, bl0, w.now.UnixMilli())
	*w.now = w.now.Add(90 * time.Second) // past AccountDecay, inside PendingTTL and inside the block
	require.NoError(t, w.th.Fail(ctx, "u:1", b.Attempt))
	_, bl1, _, _ := accountRow(t, w.raw, "u:1")
	require.GreaterOrEqual(t, bl1, bl0, "a charge must never shorten a block")
}
