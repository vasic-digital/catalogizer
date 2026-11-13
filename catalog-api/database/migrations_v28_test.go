package database

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-LOGSHARE-TOK-1 migration v28: legacy plaintext (time-derived) log-share tokens are retired, hashed ones are untouched,
// and the run is idempotent.
func TestMigrationV28_RetiresLegacyLogShareTokens(t *testing.T) {
	db, raw := newMigratedDB(t)
	ctx := context.Background()

	_, err := raw.Exec(`INSERT INTO log_collections (id, user_id, name) VALUES (1, 1, 'c')`)
	require.NoError(t, err)
	hash := strings.Repeat("ab", 32)
	for _, tok := range []string{"log_share_1700000000_123456789", "log_share_1700000001_5", hash} {
		_, err = raw.Exec(`INSERT INTO log_shares (collection_id, user_id, share_token, is_active, permissions, recipients) VALUES (1, 1, ?, 1, '["read"]', '[]')`, tok)
		require.NoError(t, err)
	}

	rerun := func() {
		_, err := raw.Exec(`DELETE FROM migrations WHERE version = 28`)
		require.NoError(t, err)
		require.NoError(t, db.RunMigrations(ctx))
	}
	dump := func() map[int]struct {
		tok    string
		active int
	} {
		rows, err := raw.Query(`SELECT id, share_token, is_active FROM log_shares ORDER BY id`)
		require.NoError(t, err)
		defer rows.Close()
		out := map[int]struct {
			tok    string
			active int
		}{}
		for rows.Next() {
			var id, a int
			var tk string
			require.NoError(t, rows.Scan(&id, &tk, &a))
			out[id] = struct {
				tok    string
				active int
			}{tk, a}
		}
		return out
	}

	rerun()
	d := dump()
	require.Len(t, d, 3)
	for id := 1; id <= 2; id++ {
		assert.Equal(t, 0, d[id].active, "legacy row %d is deactivated", id)
		assert.True(t, strings.HasPrefix(d[id].tok, "revoked-legacy-"), "legacy plaintext overwritten: %q", d[id].tok)
		assert.NotContains(t, d[id].tok, "1700000000")
	}
	assert.Equal(t, hash, d[3].tok, "a hashed token is untouched")
	assert.Equal(t, 1, d[3].active)

	before := dump()
	rerun() // idempotent
	assert.Equal(t, before, dump())
}

// The PostgreSQL statement differs only in the boolean literal, has no SQLite-only construct and no `?` marker.
func TestMigrationV28_DialectStatements(t *testing.T) {
	sqlite, pg := legacyLogShareTokenSQL(false), legacyLogShareTokenSQL(true)
	assert.Contains(t, sqlite, "is_active = 0")
	assert.Contains(t, pg, "is_active = FALSE")
	assert.Equal(t, strings.Replace(sqlite, "is_active = 0", "is_active = FALSE", 1), pg)
	for _, q := range []string{sqlite, pg} {
		assert.Contains(t, q, `LIKE 'log\_share\_%' ESCAPE '\'`)
		assert.NotContains(t, q, "?")
		assert.NotContains(t, strings.ToUpper(q), "IFNULL")
		assert.NotContains(t, strings.ToUpper(q), "GLOB")
	}
}
