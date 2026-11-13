package services

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var logShareTokenShape = regexp.MustCompile(`^log_share_[A-Za-z0-9_-]{43}$`)

func newShareFixture(t *testing.T) (*LogManagementService, *database.DB, *models.LogCollection) {
	t.Helper()
	db := setupTestDB(t)
	svc := NewLogManagementService(repository.NewLogManagementRepository(db))
	col, err := svc.CollectLogs(1, &models.LogCollectionRequest{Name: "c", Components: []string{"api"}, LogLevel: "info"})
	require.NoError(t, err)
	return svc, db, col
}

func createShare(t *testing.T, svc *LogManagementService, col *models.LogCollection) *models.LogShare {
	t.Helper()
	share, err := svc.CreateLogShare(1, &models.LogShareRequest{CollectionID: col.ID, ShareType: "link", Permissions: []string{"read"}})
	require.NoError(t, err)
	return share
}

func TestLogShareToken_EntropyUniquenessShape(t *testing.T) {
	seen := make(map[string]struct{}, 100000)
	for i := 0; i < 100000; i++ {
		tok, err := newLogShareToken(logShareRand)
		require.NoError(t, err)
		require.Len(t, tok, logShareTokenLen)
		require.Regexp(t, logShareTokenShape, tok)
		require.True(t, validLogShareTokenFormat(tok))
		if _, dup := seen[tok]; dup {
			t.Fatalf("token collision after %d tokens", i)
		}
		seen[tok] = struct{}{}
	}
}

func TestLogShareToken_FormatRejectsLegacyAndMalformed(t *testing.T) {
	good, err := newLogShareToken(logShareRand)
	require.NoError(t, err)
	for _, bad := range []string{
		"", "log_share_1700000000_123456789", good[:len(good)-1], good + "A", strings.Replace(good, "log_share_", "log_shar__", 1),
		hashLogShareToken(good), "log_share_" + strings.Repeat("!", 43), "log_share_" + strings.Repeat("A", 42) + "=",
	} {
		assert.False(t, validLogShareTokenFormat(bad), "%q", bad)
	}
}

func TestLogShareToken_EntropyFailureFailsClosed(t *testing.T) {
	for name, r := range map[string]func() error{
		"error reader": func() error { _, err := newLogShareToken(iotest.ErrReader(errors.New("no entropy"))); return err },
		"short reader": func() error { _, err := newLogShareToken(strings.NewReader("only-a-few-bytes")); return err },
	} {
		assert.Error(t, r(), name)
	}

	svc, db, col := newShareFixture(t)
	prev := logShareRand
	logShareRand = iotest.ErrReader(errors.New("no entropy"))
	t.Cleanup(func() { logShareRand = prev })
	share, err := svc.CreateLogShare(1, &models.LogShareRequest{CollectionID: col.ID, ShareType: "link", Permissions: []string{"read"}})
	assert.Error(t, err)
	assert.Nil(t, share)
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM log_shares`).Scan(&n))
	assert.Equal(t, 0, n, "no share row exists after an entropy failure")
}

func TestLogShare_HashAtRestAndRoundTrip(t *testing.T) {
	svc, db, col := newShareFixture(t)
	share := createShare(t, svc, col)
	require.Regexp(t, logShareTokenShape, share.ShareToken, "the creator receives the plaintext once")

	var stored string
	require.NoError(t, db.QueryRow(`SELECT share_token FROM log_shares WHERE id = ?`, share.ID).Scan(&stored))
	assert.Equal(t, hashLogShareToken(share.ShareToken), stored)
	assert.NotContains(t, stored, share.ShareToken)
	assert.Regexp(t, `^[0-9a-f]{64}$`, stored)

	// a full dump of every column of every share row carries no plaintext
	rows, err := db.Query(`SELECT * FROM log_shares`)
	require.NoError(t, err)
	defer rows.Close()
	cols, _ := rows.Columns()
	for rows.Next() {
		vals := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		require.NoError(t, rows.Scan(ptrs...))
		assert.NotContains(t, fmt.Sprint(vals...), share.ShareToken)
	}

	got, err := svc.GetLogShare(share.ShareToken)
	require.NoError(t, err)
	assert.Equal(t, share.ID, got.ID)
	assert.Equal(t, col.ID, got.CollectionID)
	assert.Empty(t, got.ShareToken, "the stored hash is not echoed")
}

func TestLogShare_EveryFailureIsTheSameError(t *testing.T) {
	svc, db, col := newShareFixture(t)
	good := createShare(t, svc, col)
	expired := createShare(t, svc, col)
	revoked := createShare(t, svc, col)
	_, err := db.Exec(`UPDATE log_shares SET expires_at = ? WHERE id = ?`, time.Now().Add(-time.Minute), expired.ID)
	require.NoError(t, err)
	require.NoError(t, svc.RevokeLogShare(revoked.ID, 1))
	// a legacy row, as it exists in a database that has not run migration v28
	_, err = db.Exec(`INSERT INTO log_shares (collection_id, user_id, share_token, is_active, expires_at, permissions, recipients) VALUES (?, 1, 'log_share_1700000000_42', 1, ?, '["read"]', '[]')`,
		col.ID, time.Now().Add(time.Hour))
	require.NoError(t, err)
	unknown, err := newLogShareToken(logShareRand)
	require.NoError(t, err)

	_, err = svc.GetLogShare(good.ShareToken)
	require.NoError(t, err, "control: the live share validates")
	for name, tok := range map[string]string{
		"expired": expired.ShareToken, "revoked": revoked.ShareToken, "legacy": "log_share_1700000000_42",
		"unknown": unknown, "malformed": "x", "empty": "", "stored hash presented as token": hashLogShareToken(good.ShareToken),
	} {
		got, err := svc.GetLogShare(tok)
		assert.Nil(t, got, name)
		assert.Same(t, ErrLogShareNotFound, err, "%s must be indistinguishable from an unknown token", name)
	}
}

func TestLogShare_ComparisonIsConstantTime(t *testing.T) {
	assert.True(t, logShareHashEqual("abc", "abc"))
	assert.False(t, logShareHashEqual("abc", "abd"))
	assert.False(t, logShareHashEqual("abc", "ab"))
	// source-class check (a behavioural test cannot tell == from a constant-time compare): the helper uses crypto/subtle
	src, err := os.ReadFile("log_share_token.go")
	require.NoError(t, err)
	assert.Contains(t, string(src), "subtle.ConstantTimeCompare")
}

func TestLogShare_SourceHasNoClockDerivedOrMathRandToken(t *testing.T) {
	for _, f := range []string{"log_share_token.go", "log_management_service.go"} {
		src, err := os.ReadFile(f)
		require.NoError(t, err)
		assert.NotContains(t, string(src), "math/rand", f)
		assert.NotContains(t, string(src), `"log_share_%d`, f)
	}
}
