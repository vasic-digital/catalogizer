package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"catalogizer/internal/pathguard"
	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// N2 (review round 3): with the defaults, ./data is both a write root and the home of the
// SQLite database ./data/catalogizer.db. A conversion target, a sync download target or a sync
// source must not be the database, a SQLite companion file of it, or a directory containing it.
func TestPathConfine_DatabaseIsNotAConversionOrSyncTarget(t *testing.T) {
	data := t.TempDir()
	db := filepath.Join(data, "catalogizer.db")
	require.NoError(t, os.WriteFile(db, []byte("SQLite format 3\x00"), 0o600))
	protectDatabaseForTest(t, db)
	t.Setenv(pathGuardEnvConv, data)
	t.Setenv(pathGuardEnvSync, data)
	t.Setenv(pathGuardEnvRead, data)
	src := filepath.Join(data, "in.txt")
	require.NoError(t, os.WriteFile(src, []byte("x"), 0o644))

	// conversion
	for _, target := range []string{db, db + "-wal", db + "-shm"} {
		err := confineConversionPaths(src, target)
		assert.ErrorIs(t, err, pathguard.ErrProtected, "conversion target %s", target)
	}
	assert.ErrorIs(t, confineConversionPaths(db, filepath.Join(data, "out.txt")), pathguard.ErrProtected,
		"conversion source = database (reading it out through a conversion)")
	assert.NoError(t, confineConversionPaths(src, filepath.Join(data, "out.txt")), "control: ordinary output")

	// sync
	s := &SyncService{}
	for _, tc := range []struct {
		name string
		ep   models.SyncEndpoint
	}{
		{"download into the database directory", models.SyncEndpoint{Type: "webdav", SyncDirection: models.SyncDirectionDownload, LocalPath: data}},
		{"upload of the database directory", models.SyncEndpoint{Type: "webdav", SyncDirection: models.SyncDirectionUpload, LocalPath: data}},
		{"download onto the database file", models.SyncEndpoint{Type: "webdav", SyncDirection: models.SyncDirectionDownload, LocalPath: db}},
	} {
		err := s.confineSyncPaths(&tc.ep)
		if assert.Error(t, err, tc.name) {
			assert.ErrorIs(t, err, pathguard.ErrProtected, tc.name)
			assert.True(t, strings.Contains(err.Error(), "local path"), tc.name+": %v", err)
		}
	}
	ok := models.SyncEndpoint{Type: "webdav", SyncDirection: models.SyncDirectionDownload, LocalPath: filepath.Join(data, "sync")}
	assert.NoError(t, s.confineSyncPaths(&ok), "control: a sub-directory of ./data is still a valid target")
}

// protectDatabaseForTest adds db (and its companions) to the global protected list for one test
// and restores the previous list afterwards. Without the restore the temp database stayed
// protected for the rest of the package run, and every later test whose roots contain the temp
// directory got ErrProtected (review round 4).
func protectDatabaseForTest(t *testing.T, db string) {
	t.Helper()
	prev := pathguard.SetProtected(pathguard.ProtectedPaths())
	t.Cleanup(func() { pathguard.SetProtected(prev) })
	pathguard.ProtectDatabase(db)
}
