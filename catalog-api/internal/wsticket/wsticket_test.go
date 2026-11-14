package wsticket

import (
	"encoding/base64"
	"fmt"
	"sync"
	"sync/atomic"
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
		_, _, err := s.Issue(Claims{UserID: fmt.Sprint(i)})
		require.NoError(t, err)
	}
	_, _, err := s.Issue(Claims{UserID: "x"})
	assert.ErrorIs(t, err, ErrBusy)
}

// ---- SEC-WS-1 F2 fix round 1 ----

func TestPerUserCapDoesNotStarveOtherUsers(t *testing.T) {
	s := NewStore()
	for i := 0; i < MaxPerUser; i++ {
		_, _, err := s.Issue(Claims{UserID: "A"})
		require.NoError(t, err)
	}
	_, _, err := s.Issue(Claims{UserID: "A"})
	assert.ErrorIs(t, err, ErrUserBusy, "user A is capped")
	_, _, err = s.Issue(Claims{UserID: "B"})
	assert.NoError(t, err, "user B must still mint while A floods")
}

func TestPerUserCapReleasedByRedeemAndExpiry(t *testing.T) {
	now := time.Now()
	s := NewStore().WithClock(func() time.Time { return now })
	var first string
	for i := 0; i < MaxPerUser; i++ {
		tk, _, _ := s.Issue(Claims{UserID: "A"})
		if i == 0 {
			first = tk
		}
	}
	_, err := s.Redeem(first)
	require.NoError(t, err)
	_, _, err = s.Issue(Claims{UserID: "A"})
	assert.NoError(t, err, "a redeemed ticket frees a slot")
	_, _, err = s.Issue(Claims{UserID: "A"})
	assert.ErrorIs(t, err, ErrUserBusy)
	now = now.Add(TTL + time.Second)
	_, _, err = s.Issue(Claims{UserID: "A"})
	assert.NoError(t, err, "expiry frees the slots")
	assert.Equal(t, 1, s.LenForTest(), "expired entries were purged")
}

func TestPurgeIsAmortisedNotFullScan(t *testing.T) {
	s := NewStore()
	for i := 0; i < 2000; i++ {
		_, _, err := s.Issue(Claims{UserID: "u" + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)) + string(rune('a'+(i/676)%26))})
		require.NoError(t, err)
	}
	// no entry was expired: a full-map scan per Issue would examine ~2M entries, an amortised purge a small multiple of 2000
	assert.Less(t, s.ScannedForTest(), 6000, "purge must not walk the whole live map on every Issue")
}

func TestDoubleRedeemRace(t *testing.T) {
	s := NewStore()
	tk, _, err := s.Issue(Claims{UserID: "7"})
	require.NoError(t, err)
	var wg sync.WaitGroup
	var ok int32
	start := make(chan struct{})
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if _, err := s.Redeem(tk); err == nil {
				atomic.AddInt32(&ok, 1)
			}
		}()
	}
	close(start)
	wg.Wait()
	assert.EqualValues(t, 1, ok, "exactly one of 16 concurrent redemptions may win")
}

func TestClaimsCarryRoleAndTokenExpiry(t *testing.T) {
	s := NewStore()
	exp := time.Now().Add(time.Hour).Truncate(time.Second)
	admin, _, _ := s.Issue(Claims{UserID: "1", RoleID: 1, TokenExpiry: exp})
	user, _, _ := s.Issue(Claims{UserID: "2", RoleID: 2})
	ca, err := s.Redeem(admin)
	require.NoError(t, err)
	cu, err := s.Redeem(user)
	require.NoError(t, err)
	assert.Equal(t, 1, ca.RoleID)
	assert.Equal(t, 2, cu.RoleID)
	assert.True(t, ca.TokenExpiry.Equal(exp))
	assert.True(t, cu.TokenExpiry.IsZero())
}

func TestContainsPlaintextCanFail(t *testing.T) {
	s := NewStore()
	tk, _, _ := s.Issue(Claims{UserID: "1"})
	assert.False(t, s.ContainsPlaintextForTest(tk), "golden-good: hash-only store")
	// golden-bad: a store that keeps the raw ticket bytes as key, and one that keeps the text in a claim, must be detected
	raw, err := base64.RawURLEncoding.DecodeString(tk)
	require.NoError(t, err)
	var k [32]byte
	copy(k[:], raw)
	bad := NewStore()
	bad.live[k] = entry{}
	assert.True(t, bad.ContainsPlaintextForTest(tk), "raw ticket bytes as key must be detected")
	bad2 := NewStore()
	bad2.live[hashOf("other")] = entry{claims: Claims{Username: tk}}
	assert.True(t, bad2.ContainsPlaintextForTest(tk), "ticket text inside a stored claim must be detected")
}

func TestMintLimiter(t *testing.T) {
	now := time.Now()
	l := NewLimiter(3, time.Minute).WithClock(func() time.Time { return now })
	for i := 0; i < 3; i++ {
		ok, _ := l.Allow("A")
		require.True(t, ok)
	}
	ok, retry := l.Allow("A")
	assert.False(t, ok)
	assert.Greater(t, retry, time.Duration(0))
	ok, _ = l.Allow("B")
	assert.True(t, ok, "limit is per user")
	now = now.Add(time.Minute + time.Second)
	ok, _ = l.Allow("A")
	assert.True(t, ok, "window slides")
}
