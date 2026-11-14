package wsticket

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIssueRedeemSingleUse(t *testing.T) {
	s := NewStore()
	tk, exp, err := s.Issue(Claims{UserID: "7", Username: "u", SessionID: "1", RoleID: 2})
	require.NoError(t, err)
	assert.Len(t, tk, 43) // 32 bytes base64url
	assert.WithinDuration(t, time.Now().Add(TTL), exp, 2*time.Second)
	cl, err := s.Redeem(tk)
	require.NoError(t, err)
	assert.Equal(t, Claims{UserID: "7", Username: "u", SessionID: "1", RoleID: 2}, cl)
	_, err = s.Redeem(tk)
	assert.ErrorIs(t, err, ErrInvalid, "second redemption")
}

func TestExpiryAndConsumption(t *testing.T) {
	now := time.Now()
	s := NewStore().WithClock(func() time.Time { return now })
	tk, _, _ := s.Issue(Claims{UserID: "1"})
	now = now.Add(TTL)
	_, err := s.Redeem(tk)
	assert.ErrorIs(t, err, ErrInvalid, "expired at exactly TTL")
	now = now.Add(-TTL)
	_, err = s.Redeem(tk)
	assert.ErrorIs(t, err, ErrInvalid, "an expired attempt consumed it")
}

func TestUniqueAndHashOnly(t *testing.T) {
	s := NewStore()
	a, _, _ := s.Issue(Claims{UserID: "1"})
	b, _, _ := s.Issue(Claims{UserID: "1"})
	assert.NotEqual(t, a, b)
	assert.False(t, s.ContainsPlaintextForTest(a))
	for _, bad := range []string{"", "x", a + "x"} {
		_, err := s.Redeem(bad)
		assert.ErrorIs(t, err, ErrInvalid)
	}
}

func TestBoundedStore(t *testing.T) {
	s := NewStore()
	for i := 0; i < maxLive; i++ {
		_, _, err := s.Issue(Claims{UserID: "1"})
		require.NoError(t, err)
	}
	_, _, err := s.Issue(Claims{UserID: "1"})
	assert.ErrorIs(t, err, ErrBusy)
}
