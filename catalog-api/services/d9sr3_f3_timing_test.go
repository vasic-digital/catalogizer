package services

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// interleavedMedians alternates calls to a and b (ABAB...) so host load drift hits both
// series equally, and returns the median of each.
func interleavedMedians(t *testing.T, n int, a, b func() error) (time.Duration, time.Duration) {
	t.Helper()
	da := make([]time.Duration, 0, n)
	db := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		s := time.Now()
		require.Error(t, a())
		da = append(da, time.Since(s))
		s = time.Now()
		require.Error(t, b())
		db = append(db, time.Since(s))
	}
	med := func(d []time.Duration) time.Duration {
		sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
		return d[len(d)/2]
	}
	return med(da), med(db)
}

func loginFn(t *testing.T, e *d9sEnv, user string) func() error {
	return func() error {
		_, err := e.login(user, "wrong-password-xx")
		if err != nil {
			require.ErrorIs(t, err, ErrInvalidCredentials)
		}
		return err
	}
}

// D9SR3-F3 (Roadmap-Item: D9SR3-F3, reproduce-first 11.4.292): an unknown username must cost
// about the same as a wrong password for a real user, otherwise response time enumerates
// usernames. Unit-class timing test (real bcrypt, real sqlite repo).
func TestD9SR3F3_UnknownUserLoginCostsLikeWrongPassword(t *testing.T) {
	e := newD9sEnv(t)
	e.mkUser(t, "realuser", 2)
	known, unknown := interleavedMedians(t, 7, loginFn(t, e, "realuser"), loginFn(t, e, "no-such-user-zz"))
	t.Logf("median wrong-password(real user)=%v unknown-user=%v", known, unknown)
	assert.Greater(t, int64(known), int64(5*time.Millisecond), "control: the real-user path must be doing bcrypt work")
	assert.GreaterOrEqual(t, float64(unknown), 0.75*float64(known),
		"unknown-user login must do a constant-cost dummy bcrypt compare (took %v vs %v)", unknown, known)
}

// R1-1: a user whose stored hash is the legacy 64-hex SHA-256 shape must not be the fast
// outlier (wrong password against it used to return in microseconds).
func TestD9SR3F3_LegacyHashUserLoginCostsLikeWrongPassword(t *testing.T) {
	e := newD9sEnv(t)
	id := e.mkUser(t, "realuser", 2)
	lid := e.mkUser(t, "legacyuser", 2)
	_ = id
	sum := sha256.Sum256([]byte(d9sPw + "0123456789abcdef0123456789abcdef"))
	_, err := e.sqlDB.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hex.EncodeToString(sum[:]), lid)
	require.NoError(t, err)
	known, legacy := interleavedMedians(t, 7, loginFn(t, e, "realuser"), loginFn(t, e, "legacyuser"))
	t.Logf("median wrong-password(v2 user)=%v legacy-hash user=%v", known, legacy)
	assert.Greater(t, int64(known), int64(5*time.Millisecond), "control: v2 path must do bcrypt work")
	assert.GreaterOrEqual(t, float64(legacy), 0.75*float64(known),
		"wrong password against a legacy-hash user must burn a bcrypt compare (took %v vs %v)", legacy, known)
}

// The dummy hash must be a well-formed v2 hash at the same cost real hashes use,
// otherwise the compare would return early or cost a different amount.
func TestD9SR3F3_DummyHashShapeAndCost(t *testing.T) {
	h := dummyPasswordHash()
	require.True(t, strings.HasPrefix(h, v2Prefix))
	cost, err := bcrypt.Cost([]byte(strings.TrimPrefix(h, v2Prefix)))
	require.NoError(t, err)
	assert.Equal(t, bcrypt.DefaultCost, cost)
}

// R1-4: the dummy hash is built eagerly and can never be empty (a failure panics at init).
func TestD9SR3F3_DummyHashNeverEmpty(t *testing.T) {
	require.NotEmpty(t, dummyPasswordHash())
	require.NotPanics(t, func() { mustDummyHash() })
}
