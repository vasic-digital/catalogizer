package services

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func medianLoginDuration(t *testing.T, e *d9sEnv, user, pw string, n int) time.Duration {
	t.Helper()
	ds := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		_, err := e.login(user, pw)
		ds = append(ds, time.Since(start))
		require.ErrorIs(t, err, ErrInvalidCredentials)
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	return ds[len(ds)/2]
}

// D9SR3-F3 (Roadmap-Item: D9SR3-F3, reproduce-first 11.4.292): an unknown username must cost
// about the same as a wrong password for a real user, otherwise response time enumerates
// usernames. Unit-class timing test (real bcrypt, real sqlite repo).
func TestD9SR3F3_UnknownUserLoginCostsLikeWrongPassword(t *testing.T) {
	e := newD9sEnv(t)
	e.mkUser(t, "realuser", 2)
	const n = 5
	known := medianLoginDuration(t, e, "realuser", "wrong-password-xx", n)
	unknown := medianLoginDuration(t, e, "no-such-user-zz", "wrong-password-xx", n)
	t.Logf("median wrong-password(real user)=%v unknown-user=%v", known, unknown)
	assert.Greater(t, int64(known), int64(5*time.Millisecond), "control: the real-user path must be doing bcrypt work for the comparison to mean anything")
	assert.GreaterOrEqual(t, int64(unknown)*2, int64(known),
		"unknown-user login must do a constant-cost dummy bcrypt compare (took %v vs %v)", unknown, known)
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
