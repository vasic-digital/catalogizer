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

// CAT-FS-SYMLINK-1 review round 1 (F1, F2, F4, F5, F6).

// F1: the NFSClient file of every platform must compile with the shared factory (NFSConfig.AllowSymlinkEscape on windows/darwin too).
// Run: GOOS=windows go build ./filesystem/  and  GOOS=darwin go build ./filesystem/  (this test runs both).
func TestCATFSSYMLINK1_CrossCompileWindowsAndDarwin(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("go toolchain not on PATH: cross-compile check not runnable (hardware/toolchain absent)")
	}
	for _, goos := range []string{"windows", "darwin"} {
		cmd := exec.Command(goBin, "build", "-buildvcs=false", "-o", os.DevNull, ".")
		cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH=amd64", "CGO_ENABLED=0", "GOMAXPROCS=2")
		out, err := cmd.CombinedOutput()
		assert.NoError(t, err, "GOOS=%s go build ./filesystem/ failed:\n%s", goos, out)
	}
}

// F2: the destination of a move is judged with Follow semantics (mv into a link-to-directory follows it); the source stays NoFollow.
func TestCATFSSYMLINK1_MovePairRefusesDstLinkOut(t *testing.T) {
	fx := newSymFixture(t)
	full := func(p string) string { return filepath.Join(fx.root, filepath.FromSlash(p)) }
	_, _, err := resolveMovePair(fx.root, full("real/a.txt"), full("outdir"), "/real/a.txt", "/outdir", false)
	assert.True(t, errors.Is(err, ErrPathEscapesRoot), "dst = in-root link to an outside directory: %v", err)
	_, _, err = resolveMovePair(fx.root, full("real/a.txt"), full("outdir/x.txt"), "/real/a.txt", "/outdir/x.txt", false)
	assert.True(t, errors.Is(err, ErrPathEscapesRoot), "dst below an escaping link: %v", err)
	// src leaf link out is moved as a link (NoFollow): allowed
	_, _, err = resolveMovePair(fx.root, full("outfile"), full("moved.txt"), "/outfile", "/moved.txt", false)
	assert.NoError(t, err, "moving the link itself")
	_, _, err = resolveMovePair(fx.root, full("outdir/secret.txt"), full("moved.txt"), "/outdir/secret.txt", "/moved.txt", false)
	assert.True(t, errors.Is(err, ErrPathEscapesRoot), "src below an escaping link: %v", err)
	// in-root dst link to an in-root dir is fine
	_, _, err = resolveMovePair(fx.root, full("real/a.txt"), full("inlink"), "/real/a.txt", "/inlink", false)
	assert.NoError(t, err)
	// opt-out
	_, _, err = resolveMovePair(fx.root, full("real/a.txt"), full("outdir"), "/real/a.txt", "/outdir", true)
	assert.NoError(t, err)
}

// F4 + F6: NFS DeleteDirectory of the mount point itself (or an in-root link pointing at it) is refused, like LocalClient.
func TestCATFSSYMLINK1_NFSDeleteDirectoryRefusesRoot(t *testing.T) {
	ctx := context.Background()
	fx := newSymFixture(t)
	require.NoError(t, os.Symlink(fx.root, filepath.Join(fx.root, "toroot")))
	for which, c := range clientsFor(t, fx.root, false) {
		for _, p := range []string{"/", "", ".", "/real/..", "/toroot"} {
			err := c.DeleteDirectory(ctx, p)
			assert.True(t, errors.Is(err, ErrRefuseDeleteRoot), "%s path %q: %v", which, p, err)
		}
		_, serr := os.Stat(filepath.Join(fx.root, "real", "a.txt"))
		assert.NoError(t, serr, "%s: root content must be intact", which)
	}
}

// F5: a relative base path / mount point is normalised at construction, so operations do not all fail.
func TestCATFSSYMLINK1_RelativeBaseIsNormalised(t *testing.T) {
	ctx := context.Background()
	top := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(top, "base", "d"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(top, "base", "d", "f.txt"), []byte("x"), 0o644))
	wd, _ := os.Getwd()
	require.NoError(t, os.Chdir(top))
	defer os.Chdir(wd)
	lc := NewLocalClient(&LocalConfig{BasePath: "base"})
	require.NoError(t, lc.Connect(ctx))
	_, err := lc.GetFileInfo(ctx, "/d/f.txt")
	assert.NoError(t, err, "local relative base")
	withProcMounts(t, "nas:/e "+filepath.Join(top, "base")+" nfs4 rw 0 0\n")
	nc, err := NewNFSClient(NFSConfig{MountPoint: "base"})
	require.NoError(t, err)
	require.NoError(t, nc.Connect(ctx))
	_, err = nc.GetFileInfo(ctx, "/d/f.txt")
	assert.NoError(t, err, "nfs relative mount point")
}
