package database

import (
	"database/sql"
	"os"
	"strings"
	"testing"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The root database/schema_v3_multiuser.sql is not loaded by any Go code or
// script, but an operator can apply it by hand. It must never create a usable
// credential: the former seed row carried the published bcrypt hash of the
// password "password". Real SQLite, no mocks.
func TestSchemaV3Multiuser_SeedsNoUsableCredential(t *testing.T) {
	raw, err := os.ReadFile("../../database/schema_v3_multiuser.sql")
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "92IXUNpkjO0rOQ5byMi", "published default hash must not be in the file")

	db, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer db.Close()
	db.SetMaxOpenConns(1)
	_, err = db.Exec(string(raw))
	require.NoError(t, err, "schema file must still apply")

	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n))
	assert.Equal(t, 0, n, "applying the schema must not create any user")
	var hashes int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM users WHERE password_hash LIKE '$2y$10$92IX%'`).Scan(&hashes))
	assert.Equal(t, 0, hashes)
	assert.True(t, strings.Contains(string(raw), "CREATE TABLE IF NOT EXISTS users"))
}
