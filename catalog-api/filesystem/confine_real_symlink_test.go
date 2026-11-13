//go:build linux
// +build linux

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

// CAT-FS-SYMLINK-1: real-path confinement for every sink of LocalClient and NFSClient (one shared helper, confineReal).

type symFixture struct {
	root, outside string
}

// newSymFixture builds a storage root and a sibling directory outside it, with escaping and in-root links of every shape.
func newSymFixture(t *testing.T) symFixture {
	t.Helper()
	top := t.TempDir()
	root := filepath.Join(top, "root")
	outside := filepath.Join(top, "outside")
	for _, d := range []string{root, outside, filepath.Join(root, "real"), filepath.Join(outside, "dir")} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}
	sib := filepath.Join(top, "root-sibling") // shares the string prefix of the root: a HasPrefix check would call it inside
	require.NoError(t, os.MkdirAll(sib, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(sib, "f.txt"), []byte("F"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("SECRET"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "real", "a.txt"), []byte("A"), 0o644))
	ln := func(target, name string) { require.NoError(t, os.Symlink(target, filepath.Join(root, name))) }
	ln(outside, "outdir")                                 // directory link out
	ln(filepath.Join(outside, "secret.txt"), "outfile")   // file link out
	ln(filepath.Join(outside, "nonexistent"), "dangling") // dangling link whose target is outside
	ln(filepath.Join(root, "missing_in"), "dangling_in")  // dangling link whose target is inside
	ln(sib, "sib")
	ln("..", "uproot")                                 // relative link to the parent of the root
	ln("loop2", "loop1")                               // loop
	ln("loop1", "loop2")                               // loop
	ln("real", "inlink")                               // in-root directory link (stays inside)
	ln(filepath.Join(root, "real", "a.txt"), "infile") // in-root file link (stays inside)
	ln("../outside/dir", "relout")                     // relative escape
	ln("real/../../outside", "dotdotout")              // escape through a ".." inside the target
	return symFixture{root: root, outside: outside}
}

func outsideSnapshot(t *testing.T, outside string) string {
	t.Helper()
	var sb strings.Builder
	require.NoError(t, filepath.Walk(outside, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		sb.WriteString(p)
		if !fi.IsDir() {
			b, _ := os.ReadFile(p)
			sb.WriteString("=" + string(b))
		}
		sb.WriteString("\n")
		return nil
	}))
	return sb.String()
}

// clientsFor returns a connected LocalClient and a connected NFSClient (mount table faked, no mount) over the same root.
func clientsFor(t *testing.T, root string, allow bool) map[string]FileSystemClient {
	t.Helper()
	ctx := context.Background()
	lc := NewLocalClient(&LocalConfig{BasePath: root, AllowSymlinkEscape: allow})
	require.NoError(t, lc.Connect(ctx))
	withProcMounts(t, "nas:/export "+root+" nfs4 rw 0 0\n")
	nc, err := NewNFSClient(NFSConfig{MountPoint: root, AllowSymlinkEscape: allow})
	require.NoError(t, err)
	require.NoError(t, nc.Connect(ctx))
	return map[string]FileSystemClient{"local": lc, "nfs": nc}
}

type symOp struct {
	name string
	run  func(c FileSystemClient) error
}

func symOps() []symOp {
	ctx := context.Background()
	rd := func(p string) symOp {
		return symOp{"read " + p, func(c FileSystemClient) error {
			r, err := c.ReadFile(ctx, p)
			if err == nil {
				r.Close()
			}
			return err
		}}
	}
	stat := func(p string) symOp {
		return symOp{"stat " + p, func(c FileSystemClient) error { _, err := c.GetFileInfo(ctx, p); return err }}
	}
	list := func(p string) symOp {
		return symOp{"list " + p, func(c FileSystemClient) error { _, err := c.ListDirectory(ctx, p); return err }}
	}
	exists := func(p string) symOp {
		return symOp{"exists " + p, func(c FileSystemClient) error { _, err := c.FileExists(ctx, p); return err }}
	}
	wr := func(p string) symOp {
		return symOp{"write " + p, func(c FileSystemClient) error { return c.WriteFile(ctx, p, strings.NewReader("PWNED")) }}
	}
	mk := func(p string) symOp {
		return symOp{"mkdir " + p, func(c FileSystemClient) error { return c.CreateDirectory(ctx, p) }}
	}
	cp := func(s, d string) symOp {
		return symOp{"copy " + s + " -> " + d, func(c FileSystemClient) error { return c.CopyFile(ctx, s, d) }}
	}
	delf := func(p string) symOp {
		return symOp{"deletefile " + p, func(c FileSystemClient) error { return c.DeleteFile(ctx, p) }}
	}
	deld := func(p string) symOp {
		return symOp{"deletedir " + p, func(c FileSystemClient) error { return c.DeleteDirectory(ctx, p) }}
	}
	var ops []symOp
	for _, p := range []string{"/outfile", "/outdir/secret.txt", "/dotdotout/secret.txt", "/dangling", "/uproot/outside/secret.txt", "/sib/f.txt"} {
		ops = append(ops, rd(p), stat(p), exists(p))
	}
	for _, p := range []string{"/outdir", "/relout", "/dotdotout", "/uproot/outside", "/outdir/"} {
		ops = append(ops, list(p), stat(p))
	}
	ops = append(ops,
		wr("/outdir/new.txt"), wr("/outdir/sub/deep/new.txt"), wr("/dangling"), wr("/relout/new.txt"), wr("/uproot/outside/new.txt"), wr("/outfile"),
		mk("/outdir/newdir"), mk("/outdir/a/b/c"), mk("/dangling"),
		cp("/outfile", "/copy_in.txt"), cp("/outdir/secret.txt", "/copy_in.txt"), cp("/real/a.txt", "/outdir/c.txt"), cp("/real/a.txt", "/dangling"),
		cp("/real/a.txt", "/outdir/x/y/c.txt"),
		delf("/outdir/secret.txt"), delf("/uproot/outside/secret.txt"), deld("/outdir/dir"), deld("/relout/dir"),
	)
	return ops
}

func TestCATFSSYMLINK1_EscapingLinkIsRefusedOnEverySink(t *testing.T) {
	for _, op := range symOps() {
		for _, which := range []string{"local", "nfs"} {
			op, which := op, which
			t.Run(which+"/"+op.name, func(t *testing.T) {
				fx := newSymFixture(t)
				c := clientsFor(t, fx.root, false)[which]
				before := outsideSnapshot(t, fx.outside)
				err := op.run(c)
				require.Error(t, err, "must be refused")
				assert.True(t, errors.Is(err, ErrPathEscapesRoot), "want ErrPathEscapesRoot, got %v", err)
				assert.Equal(t, before, outsideSnapshot(t, fx.outside), "nothing outside the root may change")
			})
		}
	}
}

func TestCATFSSYMLINK1_LoopIsRefused(t *testing.T) {
	ctx := context.Background()
	for which, c := range clientsFor(t, newSymFixture(t).root, false) {
		_, err := c.GetFileInfo(ctx, "/loop1/x")
		assert.True(t, errors.Is(err, ErrSymlinkLoop), "%s stat: %v", which, err)
		err = c.WriteFile(ctx, "/loop1/x", strings.NewReader("x"))
		assert.True(t, errors.Is(err, ErrSymlinkLoop), "%s write: %v", which, err)
	}
}

func TestCATFSSYMLINK1_DeletingAnEscapingLinkRemovesOnlyTheLink(t *testing.T) {
	ctx := context.Background()
	for _, which := range []string{"local", "nfs"} {
		fx := newSymFixture(t)
		c := clientsFor(t, fx.root, false)[which]
		before := outsideSnapshot(t, fx.outside)
		require.NoError(t, c.DeleteFile(ctx, "/outfile"), which)
		_, err := os.Lstat(filepath.Join(fx.root, "outfile"))
		assert.True(t, os.IsNotExist(err), "%s: the link is gone", which)
		require.NoError(t, c.DeleteDirectory(ctx, "/outdir"), which)
		_, err = os.Lstat(filepath.Join(fx.root, "outdir"))
		assert.True(t, os.IsNotExist(err), "%s: the directory link is gone", which)
		assert.Equal(t, before, outsideSnapshot(t, fx.outside), "%s: the targets are untouched", which)
	}
}

// Existing configs that use in-root links keep working, including create-new paths below them, dangling in-root links, "..", a trailing slash, a
// name containing "..", a link to the root itself and a hard link.
func TestCATFSSYMLINK1_InRootLinksStillWork(t *testing.T) {
	ctx := context.Background()
	for _, which := range []string{"local", "nfs"} {
		fx := newSymFixture(t)
		require.NoError(t, os.Link(filepath.Join(fx.outside, "secret.txt"), filepath.Join(fx.root, "hard.txt")))
		require.NoError(t, os.Symlink(fx.root, filepath.Join(fx.root, "self")))
		require.NoError(t, os.WriteFile(filepath.Join(fx.root, "real", "Wait.. Live"), []byte("w"), 0o644))
		c := clientsFor(t, fx.root, false)[which]
		for _, p := range []string{"/inlink/a.txt", "/infile", "/hard.txt", "/self/real/a.txt", "/real/../real/a.txt", "/inlink/../real/a.txt", "/real/Wait.. Live"} {
			r, err := c.ReadFile(ctx, p)
			require.NoError(t, err, "%s read %s", which, p)
			r.Close()
		}
		for _, p := range []string{"/inlink", "/inlink/", "/self/"} {
			_, err := c.ListDirectory(ctx, p)
			require.NoError(t, err, "%s list %s", which, p)
		}
		require.NoError(t, c.WriteFile(ctx, "/inlink/new/deep.txt", strings.NewReader("n")), which)
		b, err := os.ReadFile(filepath.Join(fx.root, "real", "new", "deep.txt"))
		require.NoError(t, err, which)
		assert.Equal(t, "n", string(b))
		require.NoError(t, c.WriteFile(ctx, "/dangling_in", strings.NewReader("d")), which)
		b, err = os.ReadFile(filepath.Join(fx.root, "missing_in"))
		require.NoError(t, err, which)
		assert.Equal(t, "d", string(b), "a dangling link that points inside creates its target inside")
		require.NoError(t, c.CopyFile(ctx, "/infile", "/inlink/copy.txt"), which)
		ok, err := c.FileExists(ctx, "/nothing/here")
		require.NoError(t, err, which)
		assert.False(t, ok)
	}
}

func TestCATFSSYMLINK1_OptOutAllowsEscape(t *testing.T) {
	ctx := context.Background()
	for which, c := range clientsFor(t, newSymFixture(t).root, true) {
		r, err := c.ReadFile(ctx, "/outfile")
		require.NoError(t, err, which)
		r.Close()
		require.NoError(t, c.WriteFile(ctx, "/outdir/new.txt", strings.NewReader("ok")), which)
	}
}

func TestCATFSSYMLINK1_DefaultIsConfinementOn(t *testing.T) {
	assert.False(t, (&LocalConfig{}).AllowSymlinkEscape)
	assert.False(t, (NFSConfig{}).AllowSymlinkEscape)
	f := &DefaultClientFactory{}
	fx := newSymFixture(t)
	c, err := f.CreateClient(&StorageConfig{Protocol: "local", Settings: map[string]interface{}{"base_path": fx.root}})
	require.NoError(t, err)
	require.NoError(t, c.Connect(context.Background()))
	_, err = c.ReadFile(context.Background(), "/outfile")
	assert.True(t, errors.Is(err, ErrPathEscapesRoot), "factory default: %v", err)
	c, err = f.CreateClient(&StorageConfig{Protocol: "local", Settings: map[string]interface{}{"base_path": fx.root, "allow_symlink_escape": true}})
	require.NoError(t, err)
	require.NoError(t, c.Connect(context.Background()))
	r, err := c.ReadFile(context.Background(), "/outfile")
	require.NoError(t, err)
	r.Close()
	assert.NoError(t, ValidateSettings("local", map[string]interface{}{"base_path": fx.root, "allow_symlink_escape": true}))
	assert.NoError(t, ValidateSettings("nfs", map[string]interface{}{"host": "h", "path": "/p", "mount_point": "/m", "allow_symlink_escape": false}))
	assert.Error(t, ValidateSettings("local", map[string]interface{}{"base_path": fx.root, "allow_symlink_escape": "yes"}), "a non-boolean is refused")
	assert.Error(t, ValidateSettings("ftp", map[string]interface{}{"host": "h", "allow_symlink_escape": true}), "other protocols do not take the key")
}
