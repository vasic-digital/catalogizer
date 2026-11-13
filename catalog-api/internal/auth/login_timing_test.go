package auth

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func medianLogin(t *testing.T, s *AuthService, user string, n int) time.Duration {
	t.Helper()
	ds := make([]time.Duration, 0, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		_, err := s.Login(user, "wrong-password-xx", "127.0.0.1", "ua")
		ds = append(ds, time.Since(start))
		require.Error(t, err)
	}
	sort.Slice(ds, func(i, j int) bool { return ds[i] < ds[j] })
	return ds[len(ds)/2]
}

// D9SR3-F3 class sibling (Roadmap-Item: D9SR3-F3): internal/auth Login must also do a
// constant-cost compare for an unknown user. Unit-class timing test, real bcrypt + sqlite.
func TestLogin_UnknownUserCostsLikeWrongPassword(t *testing.T) {
	s := setupRealDBService(t)
	defer s.db.Close()
	h, err := bcrypt.GenerateFromPassword([]byte("right-password"), bcrypt.DefaultCost)
	require.NoError(t, err)
	_, err = s.db.Exec(`INSERT INTO users (username, email, password_hash, first_name, last_name, role, is_active)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, "realuser", "realuser@test.com", string(h), "T", "U", RoleUser, true)
	require.NoError(t, err)

	known := medianLogin(t, s, "realuser", 5)
	unknown := medianLogin(t, s, "no-such-user-zz", 5)
	t.Logf("median wrong-password(real user)=%v unknown-user=%v", known, unknown)
	assert.Greater(t, int64(known), int64(5*time.Millisecond), "control: real-user path must do bcrypt work")
	assert.GreaterOrEqual(t, int64(unknown)*2, int64(known), "unknown-user login must burn a dummy bcrypt compare (%v vs %v)", unknown, known)
}
