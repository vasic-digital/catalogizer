package repository

// CAT-LOGIN-LOCK-2 round 4 (review round 3 findings F14-F19). SQLite tests here are UNIT-CLASS evidence (in-memory SQLite, injected
// clock, no deployed binary). The PostgreSQL counterparts are in login_throttle_pg_test.go (live-class, real PostgreSQL).
// RM2b (lockSlots reporting success when its UPDATE matched zero rows) is NOT testable on SQLite: it needs a concurrent purge
// between the two statements, and SQLite serialises every connection here; only the PostgreSQL tests can exercise it, and
// UNCONFIRMED whether they kill it (not mutation-run on PostgreSQL in this round).
// RED on the round-3 code (9311c22b3): TestInvariant_R4_EveryEvaluatedAttemptIsCharged (F14) only; the tests below it pin
// behaviour that was already correct and exist to kill mutants (F15-F17): cap time window, Fail error rollback, purge boundary.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func r4Count(t *testing.T, w *world, q string, args ...interface{}) int {
	t.Helper()
	var n int
	require.NoError(t, w.raw.QueryRow(q, args...).Scan(&n))
	return n
}

// F14 (reviewer RV3-1): after the first block expires, MaxInflight attempts that all Begin before the first Fail are all
// evaluated; every one of them must be charged, including those that settle on an account the first Fail just re-blocked.
func TestInvariant_R4_EveryEvaluatedAttemptIsCharged(t *testing.T) {
	w := newWorld(t, "u:1")
	ctx := context.Background()
	pol := w.th.pol
	for i := 0; i < pol.AccountFree; i++ {
		require.Equal(t, "evaluated", w.probe(t, fmt.Sprintf("10.9.0.%d", i)))
	}
	require.Contains(t, w.probe(t, "10.9.1.1"), "refused")
	totalEvaluated, totalCharged := 0, 0
	for window := 0; window < 6; window++ {
		before := r4Count(t, w, `SELECT failures FROM login_throttle WHERE subject='u:1' AND scope='account'`)
		var bu int64
		require.NoError(t, w.raw.QueryRow(`SELECT blocked_until_ms FROM login_throttle WHERE subject='u:1' AND scope='account'`).Scan(&bu))
		*w.now = time.UnixMilli(bu + 1)
		var open []Decision
		for i := 0; i < pol.MaxInflight; i++ {
			d, err := w.th.Begin(ctx, "u:1", fmt.Sprintf("10.9.%d.%d", 10+window, i))
			require.NoError(t, err)
			if d.Allowed {
				open = append(open, d)
			}
		}
		for _, d := range open {
			require.NoError(t, w.th.Fail(ctx, "u:1", d.Attempt))
		}
		after := r4Count(t, w, `SELECT failures FROM login_throttle WHERE subject='u:1' AND scope='account'`)
		t.Logf("window %d: evaluated %d, charged %d", window, len(open), after-before)
		totalEvaluated += len(open)
		totalCharged += after - before
	}
	t.Logf("after the first block: evaluated %d, charged %d", totalEvaluated, totalCharged)
	require.Equal(t, totalEvaluated, totalCharged, "every evaluated attempt after the first block must be charged")
}

// F14 sanity: charging an already blocked account never shortens the block, never exceeds Cap from now, and raises the failure
// count by one per charged attempt. It does NOT assert the doubling of the block end (it cannot fail on a mutant that keeps the
// old end): that is TestRV4_BlockedChargeExtendsBlockByDoubling (round-4 review RV4-1).
func TestInvariant_R4_ChargingBlockedAccountIsMonotoneAndCapped(t *testing.T) {
	w := newWorld(t, "u:1")
	ctx := context.Background()
	pol := w.th.pol
	for i := 0; i < pol.AccountFree; i++ {
		require.Equal(t, "evaluated", w.probe(t, fmt.Sprintf("10.9.0.%d", i)))
	}
	_, blocked0, _, _ := accountRow(t, w.raw, "u:1")
	var open []Decision
	*w.now = time.UnixMilli(blocked0 + 1)
	for i := 0; i < pol.MaxInflight; i++ {
		d, err := w.th.Begin(ctx, "u:1", fmt.Sprintf("10.9.5.%d", i))
		require.NoError(t, err)
		require.True(t, d.Allowed)
		open = append(open, d)
	}
	prev := int64(0)
	for _, d := range open {
		require.NoError(t, w.th.Fail(ctx, "u:1", d.Attempt))
		_, bl, _, _ := accountRow(t, w.raw, "u:1")
		require.GreaterOrEqual(t, bl, prev, "a charge must never shorten the block")
		require.LessOrEqual(t, bl, w.now.UnixMilli()+pol.Cap.Milliseconds(), "never longer than Cap from now")
		prev = bl
	}
	f, _, _, _ := accountRow(t, w.raw, "u:1")
	require.Equal(t, pol.AccountFree+pol.MaxInflight, f)
}

// F15: the cap counts tokens inside PendingTTL (a window), not only tokens of this instant (kills RM6: counting only
// last_failure_ms > now-1). Then past PendingTTL they expire.
func TestInvariant_R4_CapWindowHoldsWithinTTLAndExpiresAfter(t *testing.T) {
	w := newWorld(t, "u:1")
	ctx := context.Background()
	pol := w.th.pol
	for i := 0; i < pol.MaxInflight; i++ {
		d, err := w.th.Begin(ctx, "u:1", fmt.Sprintf("10.3.0.%d", i))
		require.NoError(t, err)
		require.True(t, d.Allowed)
	}
	start := *w.now
	for _, step := range []time.Duration{time.Second, pol.PendingTTL / 2, pol.PendingTTL - time.Second} {
		*w.now = start.Add(step) // offsets from the instant the tokens were taken, not cumulative
		d, err := w.th.Begin(ctx, "u:1", "10.3.1.1")
		require.NoError(t, err)
		require.False(t, d.Allowed, "tokens younger than PendingTTL still hold the cap (clock advanced %v)", step)
	}
	*w.now = start.Add(pol.PendingTTL + time.Second)
	d, err := w.th.Begin(ctx, "u:1", "10.3.2.1")
	require.NoError(t, err)
	require.True(t, d.Allowed, "tokens older than PendingTTL no longer count")
}

// F16 (reviewer RV3-2, RM5): a Fail whose transaction errors rolls back: the token stays (still in flight, fails closed) and
// nothing is charged. Induced by an injected charge error AND by a cancelled context.
func TestInvariant_R4_FailErrorRollsBackAndKeepsToken(t *testing.T) {
	for _, mode := range []string{"charge-error", "cancelled-context"} {
		t.Run(mode, func(t *testing.T) {
			w := newWorld(t, "u:1")
			ctx := context.Background()
			d, err := w.th.Begin(ctx, "u:1", "10.8.0.1")
			require.NoError(t, err)
			require.True(t, d.Allowed)
			fctx := ctx
			if mode == "charge-error" {
				// a trigger makes the charge statement itself fail, after the token was deleted in the same transaction
				_, err := w.raw.Exec(`CREATE TRIGGER no_account_charge BEFORE INSERT ON login_throttle WHEN NEW.scope = 'account' BEGIN SELECT RAISE(ABORT, 'injected charge failure'); END`)
				require.NoError(t, err)
			} else {
				c, cancel := context.WithCancel(ctx)
				cancel()
				fctx = c
			}
			require.Error(t, w.th.Fail(fctx, "u:1", d.Attempt))
			require.Equal(t, 1, r4Count(t, w, `SELECT COUNT(*) FROM login_throttle WHERE subject='u:1' AND substr(scope,1,5)='pend:'`), "token stays")
			require.Equal(t, 0, r4Count(t, w, `SELECT COUNT(*) FROM login_throttle WHERE subject='u:1' AND scope='account' AND failures > 0`), "nothing charged")
		})
	}
}

// F17 (RM4): the purge removes in-flight tokens older than PendingTTL and keeps younger ones.
func TestInvariant_R4_PurgeBoundaryOfInflightTokens(t *testing.T) {
	w := newWorld(t, "u:1")
	ctx := context.Background()
	pol := w.th.pol
	old, err := w.th.Begin(ctx, "u:1", "10.2.0.1")
	require.NoError(t, err)
	require.True(t, old.Allowed)
	*w.now = w.now.Add(pol.PendingTTL + time.Second) // old is now past TTL
	young, err := w.th.Begin(ctx, "u:1", "10.2.0.2") // its maybePurge purges old (purge window elapsed), takes a fresh token
	require.NoError(t, err)
	require.True(t, young.Allowed)
	require.Equal(t, 0, r4Count(t, w, `SELECT COUNT(*) FROM login_throttle WHERE subject='u:1' AND scope=?`, old.Attempt.scope), "older than TTL: purged")
	require.Equal(t, 1, r4Count(t, w, `SELECT COUNT(*) FROM login_throttle WHERE subject='u:1' AND scope=?`, young.Attempt.scope), "younger than TTL: kept")
	// and a token younger than TTL survives a purge run at a later instant that is still inside its TTL
	*w.now = w.now.Add(pol.PendingTTL - time.Second)
	w.th.mu.Lock()
	w.th.purge = time.Time{}
	w.th.mu.Unlock()
	w.th.maybePurge(ctx, *w.now, pol)
	require.Equal(t, 1, r4Count(t, w, `SELECT COUNT(*) FROM login_throttle WHERE subject='u:1' AND scope=?`, young.Attempt.scope), "inside TTL: not purged")
}

// Pins delayFor with literal values (the RV4 tests use delayFor as their own oracle, so they cannot kill a change inside it):
// kills reviewer mutants M3 (extra doubling) and M4 (cap doubled). Unit-class.
func TestInvariant_R4_DelayForLiteralValues(t *testing.T) {
	base, cap := 30*time.Second, 15*time.Minute
	require.Equal(t, time.Duration(0), delayFor(19, 20, base, cap))
	require.Equal(t, 30*time.Second, delayFor(20, 20, base, cap))
	require.Equal(t, 60*time.Second, delayFor(21, 20, base, cap))
	require.Equal(t, 120*time.Second, delayFor(22, 20, base, cap))
	require.Equal(t, 15*time.Minute, delayFor(25, 20, base, cap)) // 30s<<5 = 16m, just over Cap
	require.Equal(t, cap, delayFor(40, 20, base, cap))
	require.Equal(t, cap, delayFor(1000, 20, base, cap))
}
