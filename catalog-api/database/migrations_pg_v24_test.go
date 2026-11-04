package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"catalogizer/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real-PostgreSQL run of the production migrations up to v24 (subtitle_tracks.uploaded_by).
// It needs a PostgreSQL server you own for the test: set CATALOGIZER_PG_TEST_HOST, _PORT, _USER and
// _PASSWORD (read from the environment only, never printed). The test creates a THROWAWAY database
// on it, never touches any other database, and drops it afterwards. Without the variables it is an
// honest SKIP (not a pass).
func TestMigrationsV24_RealPostgres(t *testing.T) {
	host := os.Getenv("CATALOGIZER_PG_TEST_HOST")
	port, _ := strconv.Atoi(os.Getenv("CATALOGIZER_PG_TEST_PORT"))
	user := os.Getenv("CATALOGIZER_PG_TEST_USER")
	pass := os.Getenv("CATALOGIZER_PG_TEST_PASSWORD")
	if host == "" || port == 0 || user == "" {
		t.Skip("no PostgreSQL configured (CATALOGIZER_PG_TEST_HOST/PORT/USER/PASSWORD): honest SKIP, not a pass")
	}
	ctx := context.Background()
	admin, err := sql.Open("postgres", fmt.Sprintf("postgres://%s:%s@%s:%d/postgres?sslmode=disable", user, pass, host, port))
	require.NoError(t, err)
	defer admin.Close()
	require.NoError(t, admin.PingContext(ctx))

	name := fmt.Sprintf("tmp_subs_v24_%d", time.Now().UnixNano())
	_, err = admin.ExecContext(ctx, "CREATE DATABASE "+name)
	require.NoError(t, err)
	defer func() { _, _ = admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name) }()

	db, err := NewConnection(&config.DatabaseConfig{Type: "postgres", Host: host, Port: port, Name: name, User: user, Password: pass, SSLMode: "disable"})
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, db.RunMigrations(ctx), "all production migrations must apply on real PostgreSQL")
	require.NoError(t, db.RunMigrations(ctx), "a second run must be a no-op (idempotent)")

	var dataType, nullable string
	require.NoError(t, db.QueryRowContext(ctx, `SELECT data_type, is_nullable FROM information_schema.columns
		WHERE table_name='subtitle_tracks' AND column_name='uploaded_by'`).Scan(&dataType, &nullable))
	assert.Equal(t, "integer", dataType)
	assert.Equal(t, "YES", nullable)

	// the column is usable: a row with and without an uploader
	_, err = db.ExecContext(ctx, `INSERT INTO storage_roots (name, protocol, path) VALUES ('r','local','/x')`)
	require.NoError(t, err)
	var rootID int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM storage_roots WHERE name='r'`).Scan(&rootID))
	fileID, err := db.InsertReturningID(ctx, `INSERT INTO files (storage_root_id, path, name, size, modified_at) VALUES (?, '/a.mkv', 'a.mkv', 1, CURRENT_TIMESTAMP)`, rootID)
	require.NoError(t, err)
	withUp, err := db.InsertReturningID(ctx, `INSERT INTO subtitle_tracks (media_item_id, language, language_code, source, format, content, uploaded_by) VALUES (?, 'English','en','upload','srt','x', 7)`, fileID)
	require.NoError(t, err)
	legacy, err := db.InsertReturningID(ctx, `INSERT INTO subtitle_tracks (media_item_id, language, language_code, source, format, content) VALUES (?, 'English','en','downloaded','srt','x')`, fileID)
	require.NoError(t, err)
	var up sql.NullInt64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT uploaded_by FROM subtitle_tracks WHERE id=?`, withUp).Scan(&up))
	assert.Equal(t, int64(7), up.Int64)
	require.NoError(t, db.QueryRowContext(ctx, `SELECT uploaded_by FROM subtitle_tracks WHERE id=?`, legacy).Scan(&up))
	assert.False(t, up.Valid, "legacy rows have no uploader")
}
