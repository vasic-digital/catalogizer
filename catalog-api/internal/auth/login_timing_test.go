package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// interleavedMedians alternates calls to a and b so host load drift hits both equally.
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

func loginFn(s *AuthService, user string) func() error {
	return func() error {
		_, err := s.Login(user, "wrong-password-xx", "127.0.0.1", "ua")
		return err
	}
}

func insertUser(t *testing.T, s *AuthService, name, hash string) {
	t.Helper()
	_, err := s.db.Exec(`INSERT INTO users (username, email, password_hash, first_name, last_name, role, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, name, name+"@test.com", hash, "T", "U", RoleUser, true)
	require.NoError(t, err)
}

// D9SR3-F3 class sibling (Roadmap-Item: D9SR3-F3): internal/auth Login must also do a
// constant-cost compare for an unknown user. Unit-class timing test, real bcrypt + sqlite.
func TestLogin_UnknownUserCostsLikeWrongPassword(t *testing.T) {
	s := setupRealDBService(t)
	defer s.db.Close()
	h, err := bcrypt.GenerateFromPassword([]byte("right-password"), bcrypt.DefaultCost)
	require.NoError(t, err)
	insertUser(t, s, "realuser", string(h))

	known, unknown := interleavedMedians(t, 7, loginFn(s, "realuser"), loginFn(s, "no-such-user-zz"))
	t.Logf("median wrong-password(real user)=%v unknown-user=%v", known, unknown)
	assert.Greater(t, int64(known), int64(5*time.Millisecond), "control: real-user path must do bcrypt work")
	assert.GreaterOrEqual(t, float64(unknown), 0.75*float64(known), "unknown-user login must burn a dummy bcrypt compare (%v vs %v)", unknown, known)
}

// R1-1: a user whose stored hash is not bcrypt (legacy 64-hex SHA-256) makes bcrypt return
// instantly; it must not be the fast outlier.
func TestLogin_LegacyHashUserCostsLikeWrongPassword(t *testing.T) {
	s := setupRealDBService(t)
	defer s.db.Close()
	h, err := bcrypt.GenerateFromPassword([]byte("right-password"), bcrypt.DefaultCost)
	require.NoError(t, err)
	insertUser(t, s, "realuser", string(h))
	sum := sha256.Sum256([]byte("right-password" + "salt"))
	insertUser(t, s, "legacyuser", hex.EncodeToString(sum[:]))

	known, legacy := interleavedMedians(t, 7, loginFn(s, "realuser"), loginFn(s, "legacyuser"))
	t.Logf("median wrong-password(bcrypt user)=%v legacy-hash user=%v", known, legacy)
	assert.Greater(t, int64(known), int64(5*time.Millisecond), "control: bcrypt path must do work")
	assert.GreaterOrEqual(t, float64(legacy), 0.75*float64(known), "legacy-hash user must burn a bcrypt compare (%v vs %v)", legacy, known)
}

// R1-2: the dummy hash must be a DefaultCost bcrypt hash, else the burn costs a different amount.
func TestDummyHashCost(t *testing.T) {
	require.NotEmpty(t, dummyHash)
	cost, err := bcrypt.Cost(dummyHash)
	require.NoError(t, err)
	assert.Equal(t, bcrypt.DefaultCost, cost)
	require.NotPanics(t, func() { mustDummyHash() })
}
