package handlers

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-LOGIN-LOCK-1 round-2 tests (review findings F1, F2, F5). Same unit-class evidence as login_lock_test.go.

const llIP = "198.51.100.7"

// spend makes n wrong guesses as `name` from llIP and returns the status of the last one.
func (e *llEnv) spend(name string, n int) int {
	code := 0
	for i := 0; i < n; i++ {
		code = e.loginFrom(llIP, name, wrong(i)).Code
	}
	return code
}

// F1: the throttle subject of a name that matches no account must follow the SAME equality as the user lookup (exact), so
// no normalisation difference can tell an existing name from a nonexistent one. Every row: the variant is NOT an account.
func TestLoginLock_SubjectDerivationMatchesLookupEquality(t *testing.T) {
	variants := map[string]string{
		"upper case":            "VICTIM",
		"title case":            "Victim",
		"leading space":         " victim",
		"trailing space":        "victim ",
		"trailing dot":          "victim.",
		"trailing NUL":          "victim\x00",
		"embedded NUL":          "vic\x00tim",
		"tab":                   "\tvictim",
		"newline":               "victim\n",
		"email upper case":      "VICTIM@example.test",
		"email trailing space":  "victim@example.test ",
		"email with plus tag":   "victim+x@example.test",
		"unicode full-width v":  "ｖictim",
		"unicode composed form": "café",
		"unicode decomposed":    "café",
		"empty-ish":             " ",
	}
	for name, v := range variants {
		v := v
		t.Run(name, func(t *testing.T) {
			// (a) Self-consistency: a nonexistent name is throttled exactly like an existing one (6th attempt refused).
			e := newLLEnv(t)
			assert.Equal(t, http.StatusTooManyRequests, e.spend(v, 6), "variant %q must be throttled after its own budget", v)
			e2 := newLLEnv(t)
			assert.Equal(t, http.StatusTooManyRequests, e2.spend("victim", 6), "an existing name behaves the same")

			// (b) No sharing with the real account in either direction: the real account's lookup is exact, so the throttle is.
			e3 := newLLEnv(t)
			require.Equal(t, http.StatusUnauthorized, e3.spend(v, 5))
			assert.Equal(t, http.StatusUnauthorized, e3.loginFrom(llIP, "victim", wrong(99)).Code,
				"budget spent on %q must not throttle the real account (that would reveal it exists)", v)
			e4 := newLLEnv(t)
			require.Equal(t, http.StatusUnauthorized, e4.spend("victim", 5))
			assert.Equal(t, http.StatusUnauthorized, e4.loginFrom(llIP, v, wrong(99)).Code,
				"budget spent on the real account must not throttle %q", v)
		})
	}
}

// F1 (class, nonexistent pairs): two DIFFERENT strings that both match no account must not share a budget either; if they did,
// the pair "Victim"/"victim" would answer 429 when "victim" does not exist and 401 when it does.
func TestLoginLock_DistinctNonexistentNamesDoNotShareABudget(t *testing.T) {
	pairs := [][2]string{
		{"Ghost", "ghost"}, {"GHOST", "ghost"}, {" ghost", "ghost"}, {"ghost ", "ghost"}, {"\tghost", "ghost"},
		{"ghost.", "ghost"}, {"ghost\x00", "ghost"}, {"gh\x00ost", "ghost"}, {"ghost\n", "ghost"},
		{"GHOST@example.test", "ghost@example.test"}, {"ghost@example.test ", "ghost@example.test"},
		{"\uff47host", "ghost"}, {"caf\u00e9", "cafe\u0301"},
	}
	for _, p := range pairs {
		for _, ab := range [][2]string{{p[0], p[1]}, {p[1], p[0]}} {
			e := newLLEnv(t)
			require.Equal(t, http.StatusUnauthorized, e.spend(ab[0], 5), "%q", ab[0])
			assert.Equal(t, http.StatusUnauthorized, e.loginFrom(llIP, ab[1], wrong(99)).Code,
				"budget spent on %q must not throttle the different name %q", ab[0], ab[1])
		}
	}
}

// Reviewer probe, adopted: five wrong guesses as a case variant of a name that does NOT exist, then one as the real account.
// With case-folded subjects the nonexistent one answered 429 and the existing one 401: an existence oracle.
func TestLoginLock_CaseFoldOracleProbe(t *testing.T) {
	// Existing account: 5 attempts as "Victim", then 1 as "victim".
	e := newLLEnv(t)
	require.Equal(t, http.StatusUnauthorized, e.spend("Victim", 5))
	existing := e.loginFrom(llIP, "victim", wrong(1)).Code
	// Nonexistent account: 5 attempts as "Ghost", then 1 as "ghost".
	require.Equal(t, http.StatusUnauthorized, e.spend("Ghost", 5))
	nonexistent := e.loginFrom(llIP, "ghost", wrong(1)).Code
	assert.Equal(t, existing, nonexistent, "the follow-up must answer the same for an existing and a nonexistent name")
	assert.Equal(t, http.StatusUnauthorized, nonexistent)
}

// F2 (accepted, documented behaviour): a username and the e-mail of the SAME account share one budget, so a client that
// submits both learns they belong together. It needs both names to be known already.
func TestLoginLock_UsernameEmailLinkageIsDocumentedBehaviour(t *testing.T) {
	e := newLLEnv(t)
	require.Equal(t, http.StatusUnauthorized, e.spend("victim", 5))
	assert.Equal(t, http.StatusTooManyRequests, e.loginFrom(llIP, "victim@example.test", wrong(1)).Code,
		"same account: the budget is shared (documented linkage)")
	assert.Equal(t, http.StatusUnauthorized, e.loginFrom(llIP, "bystander", wrong(1)).Code, "another account is unaffected")
}

// F5 / M5: Retry-After rounds UP to whole seconds, so a client that waits it out is not refused again.
func TestLoginLock_RetryAfterRoundsUp(t *testing.T) {
	assert.Equal(t, 30, retryAfterSeconds(29500*time.Millisecond))
	assert.Equal(t, 1, retryAfterSeconds(time.Millisecond))
	assert.Equal(t, 1, retryAfterSeconds(0))
	assert.Equal(t, 30, retryAfterSeconds(30*time.Second))
	// End to end with a clock that is not on a whole second: the advertised wait must be enough.
	e := newLLEnv(t)
	e.advance(500 * time.Millisecond)
	e.spend("victim", 5)
	w := e.loginFrom(llIP, "victim", wrong(1))
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	var secs int
	_, err := fmt.Sscanf(w.Header().Get("Retry-After"), "%d", &secs)
	require.NoError(t, err)
	e.advance(time.Duration(secs) * time.Second)
	assert.Equal(t, http.StatusUnauthorized, e.loginFrom(llIP, "victim", wrong(2)).Code, "waiting the advertised time is enough")
}

// F5 / M3: a correct password on a NOT-loginable account clears this address's charge too (it is not a guess).
func TestLoginLock_InactiveCorrectPasswordClearsAddressBucket(t *testing.T) {
	e := newLLEnv(t)
	_, err := e.raw.Exec(`UPDATE users SET is_active = 0 WHERE id = ?`, e.uid)
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, e.spend("victim", 4))
	e.loginFrom(llIP, "victim", llPass) // 5th charged attempt (sets the block), correct password, account not loginable
	// Success cleared the address charge and block, so this address is not refused.
	assert.NotEqual(t, http.StatusTooManyRequests, e.loginFrom(llIP, "victim", wrong(7)).Code)
}

// F5 / M7: the same inactive-account success must NOT record the address as known-good.
func TestLoginLock_InactiveCorrectPasswordDoesNotRememberClient(t *testing.T) {
	e := newLLEnv(t)
	_, err := e.raw.Exec(`UPDATE users SET is_active = 0 WHERE id = ?`, e.uid)
	require.NoError(t, err)
	e.loginFrom(llIP, "victim", llPass)
	processed, _ := e.attackAccount(25, 100)
	require.Equal(t, 20, processed)
	assert.Equal(t, http.StatusTooManyRequests, e.loginFrom(llIP, "victim", wrong(1)).Code,
		"an address that only proved the password of an inactive account is not known-good")
}

// F5 / M4: known-good expires after KnownGoodTTL (30 days); before it the bypass holds.
func TestLoginLock_KnownGoodExpires(t *testing.T) {
	// The address row must outlive the stale-row purge for this to test the TTL itself, so the address makes one failed attempt
	// three minutes before the check (a recent failure keeps its row; its old success marker is what the TTL decides on).
	for _, tc := range []struct {
		name  string
		total time.Duration
		want  int
	}{
		{"within TTL", 29 * 24 * time.Hour, http.StatusUnauthorized},
		{"past TTL", 30*24*time.Hour + time.Minute, http.StatusTooManyRequests},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newLLEnv(t)
			require.Equal(t, http.StatusOK, e.loginFrom(llIP, "victim", llPass).Code)
			e.advance(tc.total - 3*time.Minute)
			require.Equal(t, http.StatusUnauthorized, e.loginFrom(llIP, "victim", wrong(0)).Code)
			e.advance(3 * time.Minute)
			processed, _ := e.attackAccount(25, 100)
			require.Equal(t, 19, processed, "20 account charges in all, one already spent by the address above")
			assert.Equal(t, tc.want, e.loginFrom(llIP, "victim", wrong(1)).Code)
		})
	}
}

// F6: the purge done inside a login request deletes at most one batch, so one request never does unbounded work.
func TestLoginLock_PurgeIsBatched(t *testing.T) {
	e := newLLEnv(t)
	const total = 2500
	tx, err := e.raw.Begin()
	require.NoError(t, err)
	for i := 0; i < total; i++ {
		_, err := tx.Exec(`INSERT INTO login_throttle (subject, scope, failures, blocked_until_ms, last_failure_ms) VALUES (?, 'account', 1, 1, 1)`, fmt.Sprintf("n:stale%05d", i))
		require.NoError(t, err)
	}
	require.NoError(t, tx.Commit())
	e.loginFrom(llIP, "victim", wrong(1)) // triggers the purge
	var left int
	require.NoError(t, e.raw.QueryRow(`SELECT COUNT(*) FROM login_throttle WHERE subject LIKE 'n:stale%'`).Scan(&left))
	assert.Equal(t, total-2000, left, "exactly one batch of 2000 purged per run")
	e.advance(2 * time.Minute)
	e.loginFrom(llIP, "victim", wrong(2))
	require.NoError(t, e.raw.QueryRow(`SELECT COUNT(*) FROM login_throttle WHERE subject LIKE 'n:stale%'`).Scan(&left))
	assert.Equal(t, 0, left, "the next run finishes it")
}
