//go:build linux
// +build linux

package filesystem

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-FS-SYMLINK-1 review round 2.

func readStr(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	require.NoError(t, err)
	return string(b)
}

// R2-1 premise: GNU cp "src dir" writes through an existing dir/<base(src)> link. This is the control showing why the darwin client must not
// shell out (skipped when cp is absent).
func TestCATFSSYMLINK1_R2_ExternalCpWritesThroughDstDirEntry(t *testing.T) {
	if _, err := exec.LookPath("cp"); err != nil {
		t.Skip("cp not available")
	}
	fx := newSymFixture(t)
	box := filepath.Join(fx.root, "box")
	require.NoError(t, os.MkdirAll(box, 0o755))
	require.NoError(t, os.Symlink(filepath.Join(fx.outside, "secret.txt"), filepath.Join(box, "a.txt")))
	out, err := exec.Command("cp", filepath.Join(fx.root, "real", "a.txt"), box).CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Equal(t, "A", readStr(t, filepath.Join(fx.outside, "secret.txt")), "control: external cp changed the outside file")
}

func TestCATFSSYMLINK1_R2_CopyDstDirEntryLinkOutIsRefused(t *testing.T) {
	ctx := context.Background()
	fx := newSymFixture(t)
	box := filepath.Join(fx.root, "box")
	require.NoError(t, os.MkdirAll(box, 0o755))
	require.NoError(t, os.Symlink(filepath.Join(fx.outside, "secret.txt"), filepath.Join(box, "a.txt")))
	before := outsideSnapshot(t, fx.outside)
	err := copyConfinedFile(ctx, fx.root, filepath.Join(fx.root, "real", "a.txt"), box, "/real/a.txt", "/box", false)
	assert.True(t, errors.Is(err, ErrCopyDestIsDirectory), "dst = existing directory is refused: %v", err)
	assert.Equal(t, before, outsideSnapshot(t, fx.outside), "outside tree unchanged")
	assert.Equal(t, "SECRET", readStr(t, filepath.Join(fx.outside, "secret.txt")))
	// the same through an in-root link to that directory
	require.NoError(t, os.Symlink(box, filepath.Join(fx.root, "boxlink")))
	err = copyConfinedFile(ctx, fx.root, filepath.Join(fx.root, "real", "a.txt"), filepath.Join(fx.root, "boxlink"), "/real/a.txt", "/boxlink", false)
	assert.True(t, errors.Is(err, ErrCopyDestIsDirectory), "%v", err)
	assert.Equal(t, "SECRET", readStr(t, filepath.Join(fx.outside, "secret.txt")))
}

func TestCATFSSYMLINK1_R2_CopyPlainDirDstDefined(t *testing.T) {
	fx := newSymFixture(t)
	d := filepath.Join(fx.root, "plain")
	require.NoError(t, os.MkdirAll(d, 0o755))
	err := copyConfinedFile(context.Background(), fx.root, filepath.Join(fx.root, "real", "a.txt"), d, "/real/a.txt", "/plain", false)
	assert.True(t, errors.Is(err, ErrCopyDestIsDirectory), "plain directory destination is refused, no implicit dir/<base>: %v", err)
	_, serr := os.Stat(filepath.Join(d, "a.txt"))
	assert.True(t, os.IsNotExist(serr), "nothing written below the directory")
}

func TestCATFSSYMLINK1_R2_CopyRefusesEscapesAndKeepsInRootWorking(t *testing.T) {
	ctx := context.Background()
	fx := newSymFixture(t)
	f := func(p string) string { return filepath.Join(fx.root, filepath.FromSlash(p)) }
	cp := func(s, d string) error { return copyConfinedFile(ctx, fx.root, f(s), f(d), "/"+s, "/"+d, false) }
	// dst is an in-root link to an outside file, src is below an escaping link
	assert.True(t, errors.Is(cp("real/a.txt", "outfile"), ErrPathEscapesRoot))
	assert.True(t, errors.Is(cp("outdir/secret.txt", "x.txt"), ErrPathEscapesRoot))
	assert.True(t, errors.Is(cp("real/a.txt", "outdir/new.txt"), ErrPathEscapesRoot))
	assert.Equal(t, "SECRET", readStr(t, filepath.Join(fx.outside, "secret.txt")))
	_, serr := os.Stat(filepath.Join(fx.outside, "new.txt"))
	assert.True(t, os.IsNotExist(serr))
	// golden-FALSE: legitimate copies
	require.NoError(t, cp("real/a.txt", "real/b.txt"))
	assert.Equal(t, "A", readStr(t, f("real/b.txt")))
	require.NoError(t, cp("real/a.txt", "inlink/c.txt")) // through an in-root dir link
	assert.Equal(t, "A", readStr(t, f("real/c.txt")))
	require.NoError(t, os.WriteFile(f("real/b.txt"), []byte("B2"), 0o644))
	require.NoError(t, cp("real/b.txt", "infile")) // overwrite through an in-root file link
	assert.Equal(t, "B2", readStr(t, f("real/a.txt")))
	// opt-out
	require.NoError(t, copyConfinedFile(ctx, fx.root, f("real/b.txt"), f("outfile"), "/real/b.txt", "/outfile", true))
	assert.Equal(t, "B2", readStr(t, filepath.Join(fx.outside, "secret.txt")))
}

// R2-4: the storage base itself is a symbolic link to the real directory. confineReal must compare against the REAL root.
func TestCATFSSYMLINK1_R2_SymlinkedBase(t *testing.T) {
	ctx := context.Background()
	top := t.TempDir()
	realBase := filepath.Join(top, "realbase")
	outside := filepath.Join(top, "outside")
	for _, d := range []string{filepath.Join(realBase, "real"), outside} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(realBase, "real", "a.txt"), []byte("A"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("S"), 0o644))
	base := filepath.Join(top, "linkbase")
	require.NoError(t, os.Symlink(realBase, base))
	require.NoError(t, os.Symlink("real", filepath.Join(realBase, "inlink")))
	require.NoError(t, os.Symlink(outside, filepath.Join(realBase, "outdir")))

	_, err := confineReal(base, filepath.Join(base, "real", "a.txt"), "/real/a.txt", false, true)
	assert.NoError(t, err, "plain file under a symlinked base")
	_, err = confineReal(base, filepath.Join(base, "inlink", "a.txt"), "/inlink/a.txt", false, true)
	assert.NoError(t, err, "in-root link under a symlinked base")
	_, err = confineReal(base, base, "/", false, true)
	assert.NoError(t, err, "the symlinked base itself")
	_, err = confineReal(base, filepath.Join(base, "outdir", "secret.txt"), "/outdir/secret.txt", false, true)
	assert.True(t, errors.Is(err, ErrPathEscapesRoot), "escape under a symlinked base: %v", err)
	// through the client
	lc := NewLocalClient(&LocalConfig{BasePath: base})
	require.NoError(t, lc.Connect(ctx))
	_, err = lc.GetFileInfo(ctx, "/inlink/a.txt")
	assert.NoError(t, err)
	_, err = lc.GetFileInfo(ctx, "/outdir/secret.txt")
	assert.True(t, errors.Is(err, ErrPathEscapesRoot), "%v", err)
}
