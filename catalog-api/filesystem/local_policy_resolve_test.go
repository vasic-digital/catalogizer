package filesystem

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"catalogizer/internal/pathguard"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ResolveLocalRoot: base confinement, symlink resolution (the resolved path is returned) and refusal of protected application files.
func TestResolveLocalRoot(t *testing.T) {
	base := t.TempDir()
	base, err := filepath.EvalSymlinks(base)
	require.NoError(t, err)
	movies := filepath.Join(base, "movies")
	require.NoError(t, os.MkdirAll(movies, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(base, "keys"), 0o755))
	db := filepath.Join(base, "catalog.db")
	key := filepath.Join(base, "keys", "db.key")
	require.NoError(t, os.WriteFile(db, []byte("x"), 0o600))
	require.NoError(t, os.WriteFile(key, []byte("k"), 0o600))
	prev := pathguard.SetProtected([]string{db, db + "-wal", db + "-shm", db + "-journal", key})
	defer pathguard.SetProtected(prev)
	bases := []string{base}

	got, err := ResolveLocalRoot(movies, bases)
	require.NoError(t, err)
	assert.Equal(t, movies, got)

	// not-yet-existing child of an allowed base is accepted (registered before its directory exists)
	got, err = ResolveLocalRoot(filepath.Join(movies, "later"), bases)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(movies, "later"), got)

	// symlink inside the base to another place inside the base: the TARGET is returned
	real := filepath.Join(base, "real")
	require.NoError(t, os.MkdirAll(real, 0o755))
	link := filepath.Join(base, "link")
	require.NoError(t, os.Symlink(real, link))
	got, err = ResolveLocalRoot(link, bases)
	require.NoError(t, err)
	assert.Equal(t, real, got, "the resolved path, not the symlink")

	// outside / escaping / relative / empty / no bases
	outside := t.TempDir()
	esc := filepath.Join(base, "escape")
	require.NoError(t, os.Symlink(outside, esc))
	for name, p := range map[string]string{"outside": outside, "symlink escape": esc, "relative": "movies", "empty": "", "dotdot": filepath.Join(movies, "..", "..")} {
		_, err := ResolveLocalRoot(p, bases)
		assert.True(t, errors.Is(err, ErrLocalRootNotAllowed), "%s must be refused, got %v", name, err)
	}
	_, err = ResolveLocalRoot(movies, nil)
	assert.True(t, errors.Is(err, ErrLocalRootNotAllowed), "no configured base denies everything")

	// protected application files: the file itself, SQLite companions, the key file, and any directory containing one
	for name, p := range map[string]string{
		"directory holding the db": base, "db": db, "wal": db + "-wal", "shm": db + "-shm", "journal": db + "-journal",
		"key dir": filepath.Join(base, "keys"), "key file": key,
	} {
		_, err := ResolveLocalRoot(p, bases)
		assert.True(t, errors.Is(err, ErrLocalRootNotAllowed), "%s must be refused, got %v", name, err)
	}
	// CheckLocalRoot is the same decision
	assert.Error(t, CheckLocalRoot(db, bases))
	assert.NoError(t, CheckLocalRoot(movies, bases))
}
