package repository

// CAT-LOGIN-LOCK-2 round 2 (finding F8, F11): the evaluated-before-block bound AccountFree + MaxInflight - 1 holds because the
// token -> charge hand-off in Fail is one transaction; plus structural pins of the lock order that SQLite cannot observe.
// Adapted from the reviewer's replay (login_throttle_rv2_test.go), which called dropToken and reserve separately: that
// interleaving no longer exists, so the replay now goes through Fail itself.

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// F18 (round-3 review): this test, the Waves test and the Concurrent test PASS on the round-2 code (097b519f3) as well, because
// SQLite serialises every connection (SetMaxOpenConns(1)) and the interleaving they are meant to expose cannot happen there. They
// pin the bound as a regression guard on SQLite only. The test that is RED on the round-2 code is the PostgreSQL one,
// TestLoginThrottlePostgres_EvaluatedBeforeBlockBound (live-class); the structural pin TestInvariant_LockOrderPinned fails on
// any change of the statement order inside Fail/takeToken.
func TestInvariant_EvaluatedBeforeBlockBound(t *testing.T) {
	w := newWorld(t, "u:1")
	ctx := context.Background()
	pol := w.th.pol
	bound := pol.AccountFree + pol.MaxInflight - 1
	evaluated := 0
	for i := 0; i < pol.AccountFree-1; i++ {
		require.Equal(t, "evaluated", w.probe(t, fmt.Sprintf("10.7.0.%d", i)))
		evaluated++
	}
	var open []Decision
	for i := 0; i < pol.MaxInflight; i++ {
		d, err := w.th.Begin(ctx, "u:1", fmt.Sprintf("10.7.1.%d", i))
		require.NoError(t, err)
		require.True(t, d.Allowed)
		open = append(open, d)
		evaluated++
	}
	require.NoError(t, w.th.Fail(ctx, "u:1", open[0].Attempt)) // the 20th charge: token and charge change hands atomically
	d, err := w.th.Begin(ctx, "u:1", "10.7.2.1")
	require.NoError(t, err)
	require.False(t, d.Allowed, "the freed slot must not be usable before the account is blocked")
	for _, o := range open[1:] {
		require.NoError(t, w.th.Fail(ctx, "u:1", o.Attempt))
	}
	require.Equal(t, bound, evaluated)
	require.LessOrEqual(t, evaluated, bound)
}

// Waves of parallel attempts that all fail: the number evaluated before the block must not depend on timing.
// (Passes on the round-2 code too, see the F18 note above: a SQLite regression guard, not proof of the atomic hand-off.)
func TestInvariant_EvaluatedBeforeBlockWaves(t *testing.T) {
	w := newWorld(t, "u:1")
	ctx := context.Background()
	pol := w.th.pol
	evaluated := 0
	for i := 0; i < pol.AccountFree-1; i++ {
		require.Equal(t, "evaluated", w.probe(t, fmt.Sprintf("10.7.0.%d", i)))
		evaluated++
	}
	for wave := 0; wave < 4; wave++ {
		var open []Decision
		for i := 0; i < pol.MaxInflight; i++ {
			d, err := w.th.Begin(ctx, "u:1", fmt.Sprintf("10.7.%d.%d", 10+wave, i))
			require.NoError(t, err)
			if d.Allowed {
				evaluated++
				open = append(open, d)
			}
		}
		for _, d := range open {
			require.NoError(t, w.th.Fail(ctx, "u:1", d.Attempt))
		}
	}
	require.LessOrEqual(t, evaluated, pol.AccountFree+pol.MaxInflight-1)
}

// The same bound under real goroutine concurrency (attackers that guess in a loop). Passes on the round-2 code too (F18 note).
func TestInvariant_EvaluatedBeforeBlockConcurrent(t *testing.T) {
	for round := 0; round < 10; round++ {
		w := newWorld(t, "u:1")
		ctx := context.Background()
		pol := w.th.pol
		var evaluated atomic.Int32
		var wg sync.WaitGroup
		for g := 0; g < 12; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				for i := 0; i < 8; i++ {
					d, err := w.th.Begin(ctx, "u:1", fmt.Sprintf("10.6.%d.%d.%d", round, g, i))
					if err != nil || !d.Allowed {
						continue
					}
					evaluated.Add(1)
					_ = w.th.Fail(ctx, "u:1", d.Attempt)
				}
			}(g)
		}
		wg.Wait()
		require.LessOrEqual(t, int(evaluated.Load()), pol.AccountFree+pol.MaxInflight-1, "round %d", round)
	}
}

func fnBody(t *testing.T, src, sig string) string {
	i := strings.Index(src, sig)
	require.True(t, i >= 0, "function %q not found", sig)
	rest := src[i:]
	j := regexp.MustCompile(`\n}\n`).FindStringIndex(rest)
	require.NotNil(t, j)
	return rest[:j[1]]
}

// Structural pins (F11): SQLite serialises a transaction by itself, so a behaviour test cannot see the lock order that matters
// on PostgreSQL (the real-PostgreSQL test is the behavioural proof and SKIPs without podman and the images). These pin the order
// lock, then count, then insert (takeToken) and begin, then delete token, then charge, then one commit (Fail, which takes no slots lock: atomic commit is what closes the gap), and that the lock is
// an UPDATE of an existing slots row (taking the lock only by a fresh-row insert conflict is not a lock once the row exists).
func TestInvariant_LockOrderPinned(t *testing.T) {
	b, err := os.ReadFile("login_throttle.go")
	require.NoError(t, err)
	src := string(b)
	lock := fnBody(t, src, "func (t *LoginThrottle) lockSlots(")
	require.Contains(t, lock, "UPDATE login_throttle SET")
	require.Contains(t, lock, "slotsScope")
	take := fnBody(t, src, "func (t *LoginThrottle) takeToken(")
	order := func(body string, marks ...string) {
		last := -1
		for _, m := range marks {
			k := strings.Index(body, m)
			if m == "tx.Commit()" {
				k = strings.LastIndex(body, m) // Fail has an early commit on the not-owner path; the charge path commits last
			}
			require.True(t, k >= 0, "missing %q", m)
			require.True(t, k > last, "%q out of order", m)
			last = k
		}
	}
	order(take, "BeginTx", "lockSlots(", "SELECT COUNT(*)", "INSERT INTO login_throttle (subject, scope, last_failure_ms)", "tx.Commit()")
	fail := fnBody(t, src, "func (t *LoginThrottle) Fail(")
	order(fail, "BeginTx", "DELETE FROM login_throttle", "t.reserve(ctx, q,", "tx.Commit()")
	require.NotContains(t, fail, "t.db.ExecContext", "Fail statements must run on the transaction")
	require.NotContains(t, fail, "t.reserve(ctx, t.db", "the charge must run on the transaction")
	require.NotContains(t, take, "t.db.ExecContext", "takeToken statements must run on the transaction")
}
