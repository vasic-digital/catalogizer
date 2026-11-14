package repository

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"catalogizer/database"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-LOGIN-LOCK-2: tests of the invariant "attacker-observable state is a function only of attacker-caused events and time,
// never of an owner success event". Unit-class evidence (SQLite in memory, injected clock); no PostgreSQL, no deployed binary.

type world struct {
	th      *LoginThrottle
	raw     *sql.DB
	now     *time.Time
	subject string
}

func newWorld(t testing.TB, subject string) *world {
	raw, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	raw.SetMaxOpenConns(1)
	_, err = raw.Exec(`CREATE TABLE login_throttle (subject TEXT NOT NULL, scope TEXT NOT NULL, failures INTEGER NOT NULL DEFAULT 0,
		blocked_until_ms BIGINT NOT NULL DEFAULT 0, last_failure_ms BIGINT NOT NULL DEFAULT 0, last_success_ms BIGINT NOT NULL DEFAULT 0,
		version BIGINT NOT NULL DEFAULT 0, PRIMARY KEY (subject, scope))`)
	require.NoError(t, err)
	th := NewLoginThrottle(database.WrapDB(raw, database.DialectSQLite))
	now := time.Unix(1_800_000_000, 0)
	th.SetClock(func() time.Time { return now })
	return &world{th: th, raw: raw, now: &now, subject: subject}
}

func newThrottleForTest(t *testing.T) (*LoginThrottle, *sql.DB, *time.Time) {
	w := newWorld(t, "u:1")
	return w.th, w.raw, w.now
}

func accountRow(t testing.TB, raw *sql.DB, subject string) (failures int, blocked, lastFailure, version int64) {
	t.Helper()
	err := raw.QueryRow(`SELECT failures, blocked_until_ms, last_failure_ms, version FROM login_throttle WHERE subject = ? AND scope = 'account'`, subject).Scan(&failures, &blocked, &lastFailure, &version)
	if err == sql.ErrNoRows {
		return 0, 0, 0, 0
	}
	require.NoError(t, err)
	return
}

// probe is one attacker guess: Begin, and when allowed the (always wrong) guess is evaluated and settled by Fail. It returns
// what the attacker sees.
func (w *world) probe(t testing.TB, addr string) string {
	d, err := w.th.Begin(context.Background(), w.subject, addr)
	require.NoError(t, err)
	if !d.Allowed {
		return fmt.Sprintf("refused:%ds", int(d.RetryAfter/time.Second))
	}
	require.NoError(t, w.th.Fail(context.Background(), w.subject, d.Attempt))
	return "evaluated"
}

// R4-2: another request blocks the account between Begin's read of the account row and its token. Begin must refuse (the
// re-read after the token), and must charge nothing to the address.
func TestBegin_AccountBlockedBetweenReadAndTokenIsRefused(t *testing.T) {
	th, raw, now := newThrottleForTest(t)
	ctx := context.Background()
	th.testAfterAccountRead = func() {
		_, err := raw.Exec(`INSERT OR REPLACE INTO login_throttle (subject, scope, failures, blocked_until_ms, last_failure_ms, version) VALUES ('u:1', 'account', 20, ?, ?, 7)`,
			now.Add(time.Minute).UnixMilli(), now.UnixMilli())
		require.NoError(t, err)
	}
	d, err := th.Begin(ctx, "u:1", "192.0.2.1")
	require.NoError(t, err)
	assert.False(t, d.Allowed, "an account blocked after the read must still refuse this attempt")
	assert.Greater(t, d.RetryAfter, time.Duration(0))
	var n int
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM login_throttle WHERE subject='u:1' AND scope LIKE 'ip:%' AND failures > 0`).Scan(&n))
	assert.Equal(t, 0, n, "a refused attempt is not charged to the address")
	require.NoError(t, raw.QueryRow(`SELECT COUNT(*) FROM login_throttle WHERE subject='u:1' AND scope LIKE 'pend:%'`).Scan(&n))
	assert.Equal(t, 0, n, "and leaves no token behind")
}

// RV5-1 repro (the CAT-LOGIN-LOCK-1 defect): account at AccountFree-1 attacker failures; the owner logs in from a new address
// while an attacker probes. The owner's success must not change what the attacker sees: the attacker's probe during the
// owner's window is evaluated (as against a nonexistent name), its failure sets the block, and the owner's success afterwards
// does not end that block.
func TestInvariant_RV51_OwnerSuccessNeverEndsAccountBlock(t *testing.T) {
	real, ghost := newWorld(t, "u:1"), newWorld(t, "n:ghost")
	for i := 0; i < 19; i++ {
		require.Equal(t, real.probe(t, fmt.Sprintf("10.0.0.%d", i)), ghost.probe(t, fmt.Sprintf("10.0.0.%d", i)))
	}
	d, err := real.th.Begin(context.Background(), "u:1", "203.0.113.9") // owner, new address, bcrypt running
	require.NoError(t, err)
	require.True(t, d.Allowed)
	assert.Equal(t, ghost.probe(t, "10.9.9.9"), real.probe(t, "10.9.9.9"), "attacker probe in the owner's window") // 20th failure: block
	require.NoError(t, real.th.Success(context.Background(), "u:1", "203.0.113.9", true, d.Attempt))
	for i := 0; i < 3; i++ {
		a := real.probe(t, fmt.Sprintf("10.8.8.%d", i))
		assert.Equal(t, ghost.probe(t, fmt.Sprintf("10.8.8.%d", i)), a)
		assert.Contains(t, a, "refused", "the block the attacker set is still there after the owner's success")
	}
	f, blocked, _, _ := accountRow(t, real.raw, "u:1")
	f2, blocked2, _, _ := accountRow(t, ghost.raw, "n:ghost")
	assert.Equal(t, f2, f)
	assert.Equal(t, blocked2, blocked)
}

// R4-1: an owner who logs in from many new addresses with nobody attacking is never throttled, and never charged.
func TestInvariant_R41_OwnerFromManyNewAddressesNeverBlocked(t *testing.T) {
	w := newWorld(t, "u:1")
	ctx := context.Background()
	for i := 0; i < 80; i++ {
		d, err := w.th.Begin(ctx, "u:1", fmt.Sprintf("198.51.%d.%d", i/200, i%200))
		require.NoError(t, err)
		require.True(t, d.Allowed, "login %d from a new address", i)
		require.NoError(t, w.th.Success(ctx, "u:1", fmt.Sprintf("198.51.%d.%d", i/200, i%200), false, d.Attempt))
		*w.now = w.now.Add(time.Second)
	}
	f, blocked, _, _ := accountRow(t, w.raw, "u:1")
	assert.Equal(t, 0, f)
	assert.Equal(t, int64(0), blocked)
	// two overlapping logins (two tabs) also pass
	d1, _ := w.th.Begin(ctx, "u:1", "203.0.113.1")
	d2, _ := w.th.Begin(ctx, "u:1", "203.0.113.2")
	require.True(t, d1.Allowed && d2.Allowed)
	require.NoError(t, w.th.Success(ctx, "u:1", "203.0.113.1", false, d1.Attempt))
	require.NoError(t, w.th.Success(ctx, "u:1", "203.0.113.2", false, d2.Attempt))
}

// RV5-2: an attempt that began before an administrator's Clear must not write into the state created after it. The old design
// recreated the row at version 1, so the owner's stale undo token erased an attacker's post-Clear charge.
func TestInvariant_RV52_StaleAttemptAfterClearWritesNothing(t *testing.T) {
	w := newWorld(t, "u:1")
	ctx := context.Background()
	stale, err := w.th.Begin(ctx, "u:1", "203.0.113.9")
	require.NoError(t, err)
	require.True(t, stale.Allowed)
	require.NoError(t, w.th.Clear(ctx, 1))
	require.Equal(t, "evaluated", w.probe(t, "10.0.0.1")) // attacker's post-Clear charge
	f0, b0, l0, v0 := accountRow(t, w.raw, "u:1")
	require.Equal(t, 1, f0)
	require.NoError(t, w.th.Success(ctx, "u:1", "203.0.113.9", true, stale.Attempt))
	f1, b1, l1, v1 := accountRow(t, w.raw, "u:1")
	assert.Equal(t, []any{f0, b0, l0, v0}, []any{f1, b1, l1, v1}, "stale Success leaves the account row byte-identical")
	// and a stale Fail charges nothing either
	stale2, _ := w.th.Begin(ctx, "u:1", "203.0.113.10")
	require.NoError(t, w.th.Clear(ctx, 1))
	require.Equal(t, "evaluated", w.probe(t, "10.0.0.2"))
	f2, b2, l2, v2 := accountRow(t, w.raw, "u:1")
	require.NoError(t, w.th.Fail(ctx, "u:1", stale2.Attempt))
	f3, b3, l3, v3 := accountRow(t, w.raw, "u:1")
	assert.Equal(t, []any{f2, b2, l2, v2}, []any{f3, b3, l3, v3}, "stale Fail leaves the account row byte-identical")
}

// RV5-4: a token is bound to its subject; settling it for another subject is refused and changes nothing.
func TestInvariant_RV54_TokenIsBoundToItsSubject(t *testing.T) {
	w := newWorld(t, "u:1")
	ctx := context.Background()
	d, err := w.th.Begin(ctx, "u:1", "203.0.113.9")
	require.NoError(t, err)
	require.True(t, d.Allowed)
	assert.ErrorIs(t, w.th.Success(ctx, "u:2", "203.0.113.9", true, d.Attempt), ErrAttemptSubject)
	assert.ErrorIs(t, w.th.Fail(ctx, "u:2", d.Attempt), ErrAttemptSubject)
	var n int
	require.NoError(t, w.raw.QueryRow(`SELECT COUNT(*) FROM login_throttle WHERE subject = 'u:2'`).Scan(&n))
	assert.Equal(t, 0, n, "nothing was written for the other subject")
	require.NoError(t, w.raw.QueryRow(`SELECT COUNT(*) FROM login_throttle WHERE subject = 'u:1' AND scope LIKE 'pend:%'`).Scan(&n))
	assert.Equal(t, 1, n, "the token is still held")
	require.NoError(t, w.th.Fail(ctx, "u:1", d.Attempt))
	f, _, _, _ := accountRow(t, w.raw, "u:1")
	assert.Equal(t, 1, f)
	// settling twice charges once
	require.NoError(t, w.th.Fail(ctx, "u:1", d.Attempt))
	f, _, _, _ = accountRow(t, w.raw, "u:1")
	assert.Equal(t, 1, f, "a token is settled once")
}

// The in-flight bound: a burst of parallel attempts is cut at MaxInflight tokens, and a refused attempt takes no token.
func TestInvariant_InflightTokensAreBounded(t *testing.T) {
	w := newWorld(t, "u:1")
	ctx := context.Background()
	var got []Decision
	refused := 0
	for i := 0; i < 10; i++ {
		d, err := w.th.Begin(ctx, "u:1", fmt.Sprintf("10.0.1.%d", i))
		require.NoError(t, err)
		if d.Allowed {
			got = append(got, d)
		} else {
			refused++
		}
	}
	assert.Len(t, got, 4, "MaxInflight default")
	assert.Equal(t, 6, refused)
	for i, d := range got {
		require.NoError(t, w.th.Fail(ctx, "u:1", d.Attempt), "attempt %d", i)
	}
	f, _, _, _ := accountRow(t, w.raw, "u:1")
	assert.Equal(t, 4, f, "each evaluated attempt is charged once")
	// an expired token no longer counts (a crashed attempt does not hold the account forever)
	d, _ := w.th.Begin(ctx, "u:1", "10.0.2.1")
	require.True(t, d.Allowed)
	*w.now = w.now.Add(3 * time.Minute)
	for i := 0; i < 4; i++ {
		x, err := w.th.Begin(ctx, "u:1", fmt.Sprintf("10.0.3.%d", i))
		require.NoError(t, err)
		assert.True(t, x.Allowed, "slot %d free again after PendingTTL", i)
	}
}

// A refusal at the per-address step releases the token it already took (otherwise refusals would fill the in-flight bound).
func TestInvariant_BucketRefusalReleasesToken(t *testing.T) {
	w := newWorld(t, "u:1")
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		d, err := w.th.Begin(ctx, "u:1", "203.0.113.1")
		require.NoError(t, err)
		require.True(t, d.Allowed)
		require.NoError(t, w.th.Fail(ctx, "u:1", d.Attempt))
	}
	for i := 0; i < 6; i++ {
		d, err := w.th.Begin(ctx, "u:1", "203.0.113.1")
		require.NoError(t, err)
		require.False(t, d.Allowed, "the address bucket (5 free) is spent")
	}
	var n int
	require.NoError(t, w.raw.QueryRow(`SELECT COUNT(*) FROM login_throttle WHERE subject='u:1' AND scope LIKE 'pend:%'`).Scan(&n))
	assert.Equal(t, 0, n, "refused attempts leave no token")
}

// ---- model-based interleaving test -------------------------------------------------------------------------------------

const modelFree = 6

type op byte

const (
	opAttack  op = 'a' // attacker probe from a fresh address (Begin + wrong guess + Fail)
	opOwnBeg  op = 'o' // owner Begin from a new address (token held, bcrypt window open)
	opOwnDone op = 's' // owner Success for the oldest open owner attempt (remembered client)
	opKnown   op = 'k' // owner Begin+Success from a known-good address (never charged to the account)
	opClear   op = 'c' // administrator Clear (applied to both worlds, see the test comment)
	opT5      op = 't' // 5 seconds pass
	opT35     op = 'u' // 35 seconds pass
	opT16m    op = 'v' // 16 minutes pass
)

// modelRun applies seq to the "real account" world (owner events happen) and to the "nonexistent name" world (they cannot),
// returns the attacker's traces (every probe, including the closing probes) of both.
func modelRun(t testing.TB, seq []op, seed int) (real, ghost []string) {
	rw, gw := newWorld(t, "u:1"), newWorld(t, "n:ghost")
	// A small account budget keeps the enumeration cheap; the logic under test does not depend on the number (the default
	// 20 is exercised by TestInvariant_RV51 and the handler tests).
	rw.th.SetPolicy(LoginThrottlePolicy{AccountFree: modelFree})
	gw.th.SetPolicy(LoginThrottlePolicy{AccountFree: modelFree})
	defer func() { _ = rw.raw.Close(); _ = gw.raw.Close() }() // thousands of runs: do not hold every database until the test ends
	ctx := context.Background()
	rec := func(w *world, trace *[]string, addr string) { *trace = append(*trace, w.probe(t, addr)) }
	addr := 0
	next := func() string { addr++; return fmt.Sprintf("10.%d.%d.%d", addr/65000, (addr/250)%250, 1+addr%250) }
	// owner's known-good address exists before the sequence (a login the owner completed earlier)
	d0, err := rw.th.Begin(ctx, "u:1", "192.0.2.77")
	require.NoError(t, err)
	require.NoError(t, rw.th.Success(ctx, "u:1", "192.0.2.77", true, d0.Attempt))
	for i := 0; i < seed; i++ {
		a := next()
		rec(rw, &real, a)
		rec(gw, &ghost, a)
	}
	var open []Decision
	oaddr := 0
	for _, o := range seq {
		switch o {
		case opAttack:
			a := next()
			rec(rw, &real, a)
			rec(gw, &ghost, a)
		case opOwnBeg:
			oaddr++
			d, err := rw.th.Begin(ctx, "u:1", fmt.Sprintf("203.0.%d.%d", oaddr/250, 1+oaddr%250))
			require.NoError(t, err)
			if d.Allowed {
				open = append(open, d)
			}
		case opOwnDone:
			if len(open) > 0 {
				require.NoError(t, rw.th.Success(ctx, "u:1", "203.0.113.200", true, open[0].Attempt))
				open = open[1:]
			}
		case opKnown:
			d, err := rw.th.Begin(ctx, "u:1", "192.0.2.77")
			require.NoError(t, err)
			if d.Allowed {
				require.NoError(t, rw.th.Success(ctx, "u:1", "192.0.2.77", true, d.Attempt))
			}
		case opClear:
			require.NoError(t, rw.th.Clear(ctx, 1))
			require.NoError(t, gw.th.Clear(ctx, 1)) // models the same administrative reset; a nonexistent name has no Clear (residual 3)
			// (the ghost world's subject is "n:ghost", so clear it directly)
			_, err := gw.raw.Exec(`DELETE FROM login_throttle WHERE subject = 'n:ghost'`)
			require.NoError(t, err)
		case opT5, opT35, opT16m:
			d := map[op]time.Duration{opT5: 5 * time.Second, opT35: 35 * time.Second, opT16m: 16 * time.Minute}[o]
			*rw.now = rw.now.Add(d)
			*gw.now = gw.now.Add(d)
		}
	}
	// settle every owner attempt still in flight, then let the attacker probe until well past the threshold
	for _, d := range open {
		require.NoError(t, rw.th.Success(ctx, "u:1", "203.0.113.200", true, d.Attempt))
	}
	for i := 0; i < 10; i++ {
		a := next()
		rec(rw, &real, a)
		rec(gw, &ghost, a)
		if i == 4 {
			*rw.now = rw.now.Add(40 * time.Second)
			*gw.now = gw.now.Add(40 * time.Second)
		}
	}
	return
}

func enumerate(alpha []op, n int, visit func([]op)) {
	seq := make([]op, 0, n)
	var rec func()
	rec = func() {
		visit(seq)
		if len(seq) == n {
			return
		}
		for _, o := range alpha {
			seq = append(seq, o)
			rec()
			seq = seq[:len(seq)-1]
		}
	}
	rec()
}

// The invariant, exhaustively over short interleavings: for every sequence of {attacker failure from a new address, owner
// success from a new address (open + settle apart, so the bcrypt window overlaps attacker probes), owner success from a
// known-good address, administrator Clear, time} the attacker's trace against the real account equals the trace against a
// nonexistent name that received only the attacker events and time.
func TestInvariant_AttackerTraceIsIndependentOfOwnerSuccess(t *testing.T) {
	alpha := []op{opAttack, opOwnBeg, opOwnDone, opKnown, opClear, opT35, opT16m}
	seeds := []struct{ seed, depth int }{{0, 4}, {modelFree - 2, 4}, {modelFree - 1, 4}, {modelFree, 4}}
	total := 0
	for _, sd := range seeds {
		if testing.Short() {
			sd.depth-- // the full enumeration takes about two minutes
		}
		enumerate(alpha, sd.depth, func(seq []op) {
			real, ghost := modelRun(t, seq, sd.seed)
			total++
			if fmt.Sprint(real) != fmt.Sprint(ghost) {
				t.Fatalf("seed %d sequence %q: attacker trace differs\nreal : %v\nghost: %v", sd.seed, string(opsToBytes(seq)), real, ghost)
			}
		})
	}
	t.Logf("%d interleavings checked", total)
	// control needle: the oracle in the model can see a difference (a world where an owner event DOES change state)
	rw, gw := newWorld(t, "u:1"), newWorld(t, "n:ghost")
	for i := 0; i < 19; i++ {
		rw.probe(t, fmt.Sprintf("10.0.0.%d", i))
		gw.probe(t, fmt.Sprintf("10.0.0.%d", i))
	}
	_, err := rw.raw.Exec(`UPDATE login_throttle SET failures = failures + 1 WHERE subject = 'u:1' AND scope = 'account'`) // an owner charge
	require.NoError(t, err)
	assert.NotEqual(t, gw.probe(t, "10.7.7.7")+gw.probe(t, "10.7.7.8"), rw.probe(t, "10.7.7.7")+rw.probe(t, "10.7.7.8"), "a leaked owner charge is visible in the trace")
}

func opsToBytes(s []op) []byte {
	b := make([]byte, len(s))
	for i, o := range s {
		b[i] = byte(o)
	}
	return b
}

// An owner with no attacker is never blocked, over every interleaving of owner events and time (R4-1, exhaustive).
func TestInvariant_OwnerWithoutAttackerIsNeverBlocked(t *testing.T) {
	alpha := []op{opOwnBeg, opOwnDone, opKnown, opT35, opT16m}
	n := 0
	enumerate(alpha, 6, func(seq []op) {
		w := newWorld(t, "u:1")
		defer func() { _ = w.raw.Close() }()
		ctx := context.Background()
		var open []Decision
		oaddr := 0
		for _, o := range seq {
			switch o {
			case opOwnBeg:
				oaddr++
				d, err := w.th.Begin(ctx, "u:1", fmt.Sprintf("203.0.%d.%d", oaddr/250, 1+oaddr%250))
				require.NoError(t, err)
				if len(open) < 2 { // two tabs at most
					require.True(t, d.Allowed, "owner refused in %q", string(opsToBytes(seq)))
					open = append(open, d)
				} else if d.Allowed {
					require.NoError(t, w.th.Success(ctx, "u:1", "203.0.113.1", false, d.Attempt))
				}
			case opOwnDone:
				if len(open) > 0 {
					require.NoError(t, w.th.Success(ctx, "u:1", "203.0.113.1", false, open[0].Attempt))
					open = open[1:]
				}
			case opKnown:
				d, err := w.th.Begin(ctx, "u:1", "192.0.2.77")
				require.NoError(t, err)
				require.True(t, d.Allowed)
				require.NoError(t, w.th.Success(ctx, "u:1", "192.0.2.77", true, d.Attempt))
			case opT5:
				*w.now = w.now.Add(5 * time.Second)
			case opT35:
				*w.now = w.now.Add(35 * time.Second)
			case opT16m:
				*w.now = w.now.Add(16 * time.Minute)
			}
		}
		f, blocked, _, _ := accountRow(t, w.raw, "u:1")
		require.Equal(t, 0, f, "%q", string(opsToBytes(seq)))
		require.Equal(t, int64(0), blocked)
		n++
	})
	t.Logf("%d owner-only interleavings checked", n)
}
