package filesystem

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-001 review F1: ConfinePath is lexical; a symlink inside the root must not let any file operation reach outside it.
func TestCAT001_LocalClient_SymlinkEscapeIsRefusedForEveryOperation(t *testing.T) {
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("s"), 0o644))
	base := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(base, "dir")))
	require.NoError(t, os.Symlink(filepath.Join(outside, "secret.txt"), filepath.Join(base, "file")))
	require.NoError(t, os.WriteFile(filepath.Join(base, "inside.txt"), []byte("i"), 0o644))
	c := NewLocalClient(&LocalConfig{BasePath: base})
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))

	_, err := c.ListDirectory(ctx, "/dir")
	assert.True(t, errors.Is(err, ErrPathEscapesRoot), "list via dir symlink: %v", err)
	_, err = c.ReadFile(ctx, "/file")
	assert.True(t, errors.Is(err, ErrPathEscapesRoot), "read via file symlink: %v", err)
	_, err = c.OpenSeekable(ctx, "/dir/secret.txt")
	assert.True(t, errors.Is(err, ErrPathEscapesRoot), "open via dir symlink: %v", err)
	_, err = c.GetFileInfo(ctx, "/file")
	assert.True(t, errors.Is(err, ErrPathEscapesRoot), "stat via file symlink: %v", err)
	err = c.WriteFile(ctx, "/dir/new.txt", stringsReader("x"))
	assert.True(t, errors.Is(err, ErrPathEscapesRoot), "write via dir symlink: %v", err)
	_, statErr := os.Stat(filepath.Join(outside, "new.txt"))
	assert.True(t, os.IsNotExist(statErr), "nothing was written outside the root")

	// controls: ordinary access still works, including a path that does not exist yet
	r, err := c.ReadFile(ctx, "/inside.txt")
	require.NoError(t, err)
	r.Close()
	require.NoError(t, c.WriteFile(ctx, "/sub/new.txt", stringsReader("ok")))
	_, err = os.Stat(filepath.Join(base, "sub", "new.txt"))
	assert.NoError(t, err)
}

func TestCAT001_CheckLocalRoot(t *testing.T) {
	base := t.TempDir()
	other := t.TempDir()
	require.NoError(t, os.Symlink(other, filepath.Join(base, "out")))
	assert.NoError(t, CheckLocalRoot(filepath.Join(base, "x", "not-yet"), []string{base}), "a missing tail under the base is allowed")
	assert.NoError(t, CheckLocalRoot(base, []string{base}))
	assert.ErrorIs(t, CheckLocalRoot("/", []string{base}), ErrLocalRootNotAllowed)
	assert.ErrorIs(t, CheckLocalRoot(other, []string{base}), ErrLocalRootNotAllowed)
	assert.ErrorIs(t, CheckLocalRoot(filepath.Join(base, "out"), []string{base}), ErrLocalRootNotAllowed, "symlink out of the base")
	assert.ErrorIs(t, CheckLocalRoot(filepath.Join(base, "out", "new"), []string{base}), ErrLocalRootNotAllowed, "missing tail behind an escaping symlink")
	assert.ErrorIs(t, CheckLocalRoot(base, nil), ErrLocalRootNotAllowed, "no base configured denies")
	assert.ErrorIs(t, CheckLocalRoot("relative/path", []string{base}), ErrLocalRootNotAllowed)
	assert.ErrorIs(t, CheckLocalRoot(base+"-sibling", []string{base}), ErrLocalRootNotAllowed, "prefix-sharing sibling is not inside")
}

func stringsReader(s string) *strings.Reader { return strings.NewReader(s) }

// CAT-001 review F9: DeleteDirectory("/") (or ".", "") must not remove the storage root itself and everything under it.
func TestCAT001_LocalClient_DeleteDirectoryRefusesTheStorageRoot(t *testing.T) {
	base := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(base, "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(base, "sub", "keep.txt"), []byte("k"), 0o644))
	c := NewLocalClient(&LocalConfig{BasePath: base})
	ctx := context.Background()
	require.NoError(t, c.Connect(ctx))

	for _, p := range []string{"/", "", ".", "/sub/.."} {
		err := c.DeleteDirectory(ctx, p)
		assert.True(t, errors.Is(err, ErrRefuseDeleteRoot), "path %q: %v", p, err)
		_, statErr := os.Stat(filepath.Join(base, "sub", "keep.txt"))
		assert.NoError(t, statErr, "path %q must not delete content", p)
	}
	// control: an ordinary subdirectory is still deleted
	require.NoError(t, c.DeleteDirectory(ctx, "/sub"))
	_, statErr := os.Stat(filepath.Join(base, "sub"))
	assert.True(t, os.IsNotExist(statErr))
}
