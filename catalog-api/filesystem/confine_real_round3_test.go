//go:build linux
// +build linux

package filesystem

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-FS-SYMLINK-1 review round 3 (R3-1 same-file data loss, R3-2 source last-component link, R3-3 ctx, R3-4 mode, R3-5 mv premise).

type r3Fx struct {
	symFixture
	f  func(string) string
	cp func(ctx context.Context, s, d string) error
}

func newR3(t *testing.T) r3Fx {
	t.Helper()
	fx := newSymFixture(t)
	f := func(p string) string { return filepath.Join(fx.root, filepath.FromSlash(p)) }
	return r3Fx{fx, f, func(ctx context.Context, s, d string) error {
		return copyConfinedFile(ctx, fx.root, f(s), f(d), "/"+s, "/"+d, false)
	}}
}

func TestCATFSSYMLINK1_R3_SameFileIsRefusedAndNothingTruncated(t *testing.T) {
	ctx := context.Background()
	x := newR3(t)
	require.NoError(t, os.Link(x.f("real/a.txt"), x.f("real/hard.txt")))
	cases := map[string][2]string{
		"same path":          {"real/a.txt", "real/a.txt"},
		"dir link alias":     {"real/a.txt", "inlink/a.txt"},
		"file link to src":   {"real/a.txt", "infile"},
		"hard link":          {"real/a.txt", "real/hard.txt"},
		"hard link reversed": {"real/hard.txt", "real/a.txt"},
	}
	for name, c := range cases {
		err := x.cp(ctx, c[0], c[1])
		assert.True(t, errors.Is(err, ErrCopySameFile), "%s: %v", name, err)
		assert.Equal(t, "A", readStr(t, x.f("real/a.txt")), "%s: source must keep its content", name)
	}
	// the same through all three clients' CopyFile (linux local and nfs share the helper)
	for which, c := range clientsFor(t, x.root, false) {
		err := c.CopyFile(ctx, "/real/a.txt", "/inlink/a.txt")
		assert.True(t, errors.Is(err, ErrCopySameFile), "%s: %v", which, err)
		assert.Equal(t, "A", readStr(t, x.f("real/a.txt")), which)
	}
}

// moveFile in NFSProtocolHandler is CopyFile then DeleteFile(old): a same-file "move" must not reach the delete with an emptied file.
func TestCATFSSYMLINK1_R3_ControlExternalCpTruncatesNeverUsed(t *testing.T) {
	// GNU cp refuses the same file (the behaviour the helper matches).
	if _, err := exec.LookPath("cp"); err != nil {
		t.Skip("cp not available")
	}
	x := newR3(t)
	out, err := exec.Command("cp", x.f("real/a.txt"), x.f("inlink/a.txt")).CombinedOutput()
	require.Error(t, err)
	assert.Contains(t, string(out), "same file")
}

func TestCATFSSYMLINK1_R3_SourceMustBeRegularFile(t *testing.T) {
	ctx := context.Background()
	x := newR3(t)
	require.NoError(t, os.MkdirAll(x.f("adir"), 0o755))
	require.NoError(t, syscallMkfifo(x.f("fifo")))
	require.NoError(t, os.WriteFile(x.f("real/keep.txt"), []byte("KEEP"), 0o644))
	for _, s := range []string{"adir", "fifo", "inlink"} {
		err := x.cp(ctx, s, "real/keep.txt")
		assert.True(t, errors.Is(err, ErrCopySourceNotRegular), "%s: %v", s, err)
		assert.Equal(t, "KEEP", readStr(t, x.f("real/keep.txt")), "existing dst untouched for src %s", s)
		err = x.cp(ctx, s, "real/new_from_"+s)
		assert.Error(t, err)
		_, serr := os.Lstat(x.f("real/new_from_" + s))
		assert.True(t, os.IsNotExist(serr), "no dst created for src %s", s)
	}
}

func TestCATFSSYMLINK1_R3_FailedOrCancelledCopyKeepsExistingDstAndLeavesNoTemp(t *testing.T) {
	x := newR3(t)
	big := strings.Repeat("0123456789abcdef", 3*(1<<20)/16) // 3 MiB: several chunks
	require.NoError(t, os.WriteFile(x.f("real/big.txt"), []byte(big), 0o644))
	require.NoError(t, os.WriteFile(x.f("real/dst.txt"), []byte("OLD"), 0o644))
	ctx, cancel := context.WithCancel(context.Background())
	n := 0
	copyChunkHook = func() {
		n++
		if n == 1 {
			cancel()
		}
	}
	defer func() { copyChunkHook = nil }()
	err := x.cp(ctx, "real/big.txt", "real/dst.txt")
	assert.True(t, errors.Is(err, context.Canceled), "cancel during the loop is honoured: %v", err)
	assert.Equal(t, "OLD", readStr(t, x.f("real/dst.txt")), "existing dst never truncated")
	ents, rerr := os.ReadDir(x.f("real"))
	require.NoError(t, rerr)
	for _, e := range ents {
		assert.False(t, strings.HasPrefix(e.Name(), ".copy-"), "temp file left behind: %s", e.Name())
	}
}

func TestCATFSSYMLINK1_R3_CancelledContextCreatesNothing(t *testing.T) {
	x := newR3(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	created := false
	copyTempHook = func(string) { created = true }
	defer func() { copyTempHook = nil }()
	err := x.cp(ctx, "real/a.txt", "real/never.txt")
	assert.True(t, errors.Is(err, context.Canceled), "%v", err)
	assert.False(t, created, "pre-check: not even a temp file is created")
	_, serr := os.Lstat(x.f("real/never.txt"))
	assert.True(t, os.IsNotExist(serr))
}

// R3-2: the LAST component of the source is an escaping link (the source is followed).
func TestCATFSSYMLINK1_R3_SourceLastComponentLinkOutIsRefused(t *testing.T) {
	ctx := context.Background()
	x := newR3(t)
	before := outsideSnapshot(t, x.outside)
	for _, s := range []string{"outfile", "dangling", "sib/f.txt", "outdir"} {
		err := x.cp(ctx, s, "x.txt")
		assert.True(t, errors.Is(err, ErrPathEscapesRoot), "src %s: %v", s, err)
		_, serr := os.Lstat(x.f("x.txt"))
		assert.True(t, os.IsNotExist(serr), "outside content never copied in (src %s)", s)
	}
	assert.Equal(t, before, outsideSnapshot(t, x.outside))
}

func TestCATFSSYMLINK1_R3_TempFileLivesNextToTheRealTargetAndModeIsKept(t *testing.T) {
	ctx := context.Background()
	x := newR3(t)
	require.NoError(t, os.WriteFile(x.f("real/exec.sh"), []byte("#!/bin/sh\n"), 0o750))
	require.NoError(t, os.Chmod(x.f("real/exec.sh"), 0o750))
	var tmpDir string
	copyTempHook = func(p string) { tmpDir = filepath.Dir(p) }
	defer func() { copyTempHook = nil }()
	require.NoError(t, x.cp(ctx, "real/exec.sh", "inlink/sub/out.sh")) // through a dir link, new parent created
	real, err := filepath.EvalSymlinks(x.f("real/sub"))
	require.NoError(t, err)
	assert.Equal(t, real, tmpDir, "temp file is created in the resolved confined parent")
	fi, err := os.Stat(x.f("real/sub/out.sh"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o750), fi.Mode().Perm(), "source permission bits preserved")
	assert.Equal(t, "#!/bin/sh\n", readStr(t, x.f("real/sub/out.sh")))
	// write-through of an in-root link (documented): the link survives, the target changes
	require.NoError(t, os.WriteFile(x.f("real/b.txt"), []byte("B"), 0o644))
	require.NoError(t, x.cp(ctx, "real/b.txt", "infile"))
	lfi, err := os.Lstat(x.f("infile"))
	require.NoError(t, err)
	assert.True(t, lfi.Mode()&os.ModeSymlink != 0, "link survives")
	assert.Equal(t, "B", readStr(t, x.f("real/a.txt")))
}

func TestCATFSSYMLINK1_R3_DirDstErrorIsUniformOnLinuxClients(t *testing.T) {
	ctx := context.Background()
	x := newR3(t)
	for which, c := range clientsFor(t, x.root, false) {
		err := c.CopyFile(ctx, "/real/a.txt", "/real")
		assert.True(t, errors.Is(err, ErrCopyDestIsDirectory), "%s: %v", which, err)
	}
}

// R3-5: the premise "rename(2) replaces the link in the destination directory, the outside file is untouched" pinned against GNU mv.
func TestCATFSSYMLINK1_R3_ControlMvReplacesLinkInDestDir(t *testing.T) {
	if _, err := exec.LookPath("mv"); err != nil {
		t.Skip("mv not available")
	}
	x := newR3(t)
	box := x.f("box")
	require.NoError(t, os.MkdirAll(box, 0o755))
	require.NoError(t, os.Symlink(filepath.Join(x.outside, "secret.txt"), filepath.Join(box, "a.txt")))
	out, err := exec.Command("mv", "-T", x.f("real/a.txt"), filepath.Join(box, "a.txt")).CombinedOutput()
	require.NoError(t, err, string(out))
	assert.Equal(t, "SECRET", readStr(t, filepath.Join(x.outside, "secret.txt")), "outside untouched")
	fi, err := os.Lstat(filepath.Join(box, "a.txt"))
	require.NoError(t, err)
	assert.True(t, fi.Mode().IsRegular(), "the link was replaced by the moved file")
}
