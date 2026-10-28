package database

// CAT-001 review round 4 follow-ups (test-instrumentation and wording):
// descriptor-leak tests for probeInUse (reviewer mutants Z2, Z6) and the
// message for a non-busy open error. Real go-sqlcipher driver, real files.

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openFDs counts this process's open file descriptors (Linux /proc).
func openFDs(t *testing.T) int {
	t.Helper()
	ents, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		t.Skip("cannot count descriptors on this platform: " + err.Error())
	}
	return len(ents)
}

// Z2: on the wrong-key path (COUNT(*) fails with NOTADB, classified "not in
// use") the probe must still close its handle.
func TestHardening_Z2_ProbeWrongKeyClosesHandle(t *testing.T) {
	cfg, dbPath, _ := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "z2")
	require.NoError(t, db.Close())
	other, err := newKey()
	require.NoError(t, err)

	before := openFDs(t)
	for i := 0; i < 5; i++ {
		rel, err := probeInUse(dbPath, other)
		require.NoError(t, err)
		rel()
	}
	assert.LessOrEqual(t, openFDs(t), before, "probeInUse leaked descriptors on the wrong-key path")
}

// Z6: when the read succeeds but the exclusive lock cannot be taken (another
// connection has the database open), the probe must refuse AND close its handle.
func TestHardening_Z6_ProbeLockFailureClosesHandle(t *testing.T) {
	cfg, dbPath, keyPath := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "z6")
	require.NoError(t, db.Close())
	key := keyOfCfg(t, keyPath)

	// SQLite parks a descriptor closed while another connection in the same
	// process still holds locks on the same inode until those locks go, so the
	// baseline is taken before the holder exists and the comparison after it
	// has been closed: a handle the probe failed to close stays open.
	base := openFDs(t)
	holder, err := sql.Open("sqlite3", keyedDSN(dbPath, key, "_busy_timeout=200"))
	require.NoError(t, err)
	holder.SetMaxOpenConns(1)
	require.NoError(t, verifyKeyed(holder)) // open and readable, but not exclusive

	old := lockBusyMS
	lockBusyMS = 200
	defer func() { lockBusyMS = old }()

	for i := 0; i < 5; i++ {
		rel, err := probeInUse(dbPath, key)
		if rel != nil {
			rel()
		}
		require.Error(t, err, "a database another connection has open must be reported in use")
		assert.Contains(t, err.Error(), "in use")
	}
	require.NoError(t, holder.Close())
	assert.LessOrEqual(t, openFDs(t), base, "probeInUse leaked descriptors when the lock could not be taken")
}

// A success must hand back a release that closes the handle too.
func TestHardening_Z7_ProbeReleaseClosesHandle(t *testing.T) {
	cfg, dbPath, keyPath := encEnv(t)
	db, err := NewConnection(cfg)
	require.NoError(t, err)
	seed(t, db, "z7")
	require.NoError(t, db.Close())
	key := keyOfCfg(t, keyPath)

	before := openFDs(t)
	for i := 0; i < 5; i++ {
		rel, err := probeInUse(dbPath, key)
		require.NoError(t, err)
		rel()
	}
	assert.LessOrEqual(t, openFDs(t), before)
}

// W1: a non-busy open error (here: a directory at the database path) must be
// reported as the underlying error, not as "SQLCipher is not active", and
// must not carry the key.
func TestHardening_W1_ProbeOpenErrorReportedSeparately(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "isadir.db")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	key, err := newKey()
	require.NoError(t, err)

	rel, err := probeInUse(dir, key)
	if rel != nil {
		rel()
	}
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "not active", "an open failure is not a missing-cipher condition")
	assert.Contains(t, err.Error(), "cannot open", "the error must say the database could not be opened")
	assert.NotContains(t, err.Error(), key, "the key must never appear in an error")
}
