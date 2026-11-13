package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"catalogizer/database"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newThrottleForTest(t *testing.T) (*LoginThrottle, *sql.DB, *time.Time) {
	t.Helper()
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
	return th, raw, &now
}

func accountRow(t *testing.T, raw *sql.DB, subject string) (failures int, blocked, lastFailure int64) {
	t.Helper()
	require.NoError(t, raw.QueryRow(`SELECT failures, blocked_until_ms, last_failure_ms FROM login_throttle WHERE subject = ? AND scope = 'account'`, subject).Scan(&failures, &blocked, &lastFailure))
	return
}

// R4-2: another request blocks the account between Begin's read of the account row and its reserve. The reserve then refuses,
// and Begin must refuse too (removing the "not ok -> refuse" after the account reserve lets the attempt through).
func TestBegin_AccountBlockedBetweenReadAndReserveIsRefused(t *testing.T) {
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
}

// R4-1: a success gives back exactly its own account charge, restoring the row as it was.
func TestSuccess_RefundsOwnAccountChargeOnly(t *testing.T) {
	th, raw, now := newThrottleForTest(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ { // prior attacker charges from other addresses
		d, err := th.Begin(ctx, "u:1", "198.51.100."+string(rune('1'+i)))
		require.NoError(t, err)
		require.True(t, d.Allowed)
	}
	now2 := now.Add(10 * time.Minute)
	th.SetClock(func() time.Time { return now2 })
	f0, b0, l0 := accountRow(t, raw, "u:1")
	d, err := th.Begin(ctx, "u:1", "203.0.113.9")
	require.NoError(t, err)
	require.True(t, d.Allowed)
	f1, _, _ := accountRow(t, raw, "u:1")
	require.Equal(t, f0+1, f1)
	require.NoError(t, th.Success(ctx, "u:1", "203.0.113.9", true, d.AccountRefund))
	f2, b2, l2 := accountRow(t, raw, "u:1")
	assert.Equal(t, f0, f2, "own charge refunded, the 3 earlier charges remain")
	assert.Equal(t, b0, b2)
	assert.Equal(t, l0, l2, "the decay clock is restored too")
}

// A charge that itself set the block is undone as well (the 20th charge is evaluated and blocks; the owner's success lifts it).
func TestSuccess_RefundUndoesTheBlockThisChargeSet(t *testing.T) {
	th, raw, _ := newThrottleForTest(t)
	ctx := context.Background()
	for i := 0; i < 19; i++ {
		d, err := th.Begin(ctx, "u:1", "10.0.0."+string(rune('a'+i)))
		require.NoError(t, err)
		require.True(t, d.Allowed)
	}
	d, err := th.Begin(ctx, "u:1", "203.0.113.9") // 20th: allowed, sets the block
	require.NoError(t, err)
	require.True(t, d.Allowed)
	_, blocked, _ := accountRow(t, raw, "u:1")
	require.Greater(t, blocked, int64(0), "the 20th charge set a block")
	require.NoError(t, th.Success(ctx, "u:1", "203.0.113.9", false, d.AccountRefund))
	f, blocked, _ := accountRow(t, raw, "u:1")
	assert.Equal(t, 19, f)
	assert.Equal(t, int64(0), blocked)
}

// If any other attempt touched the row after the charge, nothing is restored: an attacker's charge is never erased.
func TestSuccess_NoRefundWhenRowChangedMeanwhile(t *testing.T) {
	th, raw, _ := newThrottleForTest(t)
	ctx := context.Background()
	d, err := th.Begin(ctx, "u:1", "203.0.113.9")
	require.NoError(t, err)
	_, err = th.Begin(ctx, "u:1", "198.51.100.7") // another attempt (the attacker's) after the owner's charge
	require.NoError(t, err)
	require.NoError(t, th.Success(ctx, "u:1", "203.0.113.9", false, d.AccountRefund))
	f, _, _ := accountRow(t, raw, "u:1")
	assert.Equal(t, 2, f, "both charges stay")
}
