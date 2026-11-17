package database

// CAT-PG-TX-SWEEP-1: Tx rewrites every statement, and several callers (user_repository, login_throttle, TxInsertReturningID)
// already rewrote theirs before the Tx type existed, so the rewrite MUST be idempotent: a second pass changes nothing.

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRewriteQueryIsIdempotentOnPostgres(t *testing.T) {
	db := &DB{dialect: Dialect{Type: DialectPostgres}}
	queries := []string{
		`SELECT id FROM users WHERE username = ? AND is_active = 1`,
		`INSERT OR IGNORE INTO login_throttle (subject, scope) VALUES (?, ?)`,
		`UPDATE files SET deleted = 0, parent_id = COALESCE(?, parent_id) WHERE path = ?`,
		`INSERT INTO t (a) VALUES (?) ON CONFLICT (a) DO NOTHING`,
		`UPDATE playback_sessions SET completed = ? WHERE id = ?`,
		`DELETE FROM roles WHERE id = ? AND is_system = 0`,
	}
	for _, q := range queries {
		once := db.rewriteQuery(q)
		assert.Equal(t, once, db.rewriteQuery(once), "second rewrite changed: %s", q)
	}
	assert.NotEqual(t, queries[0], db.rewriteQuery(queries[0]), "control: the rewrite must actually change a `?` query")
}
