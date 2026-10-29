package services

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/webdav"
)

// Review round 8, item 1 (HIGH, introduced by the round-7 S1 fix): the sync roots were confined
// by confineSyncPaths and then resolved again with EvalSymlinks when the walk started. A root
// swapped for a symlink to an outside directory in between was followed: a swapped SOURCE copied
// outside files into the destination, a swapped DESTINATION let mirror cleanup delete outside
// files. The roots are now resolved and confined in one step (pathguard.ConfineData returns the
// resolved path) and the walk uses that path.
//
// The tests below perform the swap after confineSyncPaths, i.e. they reproduce the race window
// deterministically by doing the swap between the two steps (reviewer probes, adopted).

type rv8SwapFixture struct {
	base, allowed, outside, src, dst string
}

func newRV8SwapFixture(t *testing.T) rv8SwapFixture {
	t.Helper()
	base := t.TempDir()
	f := rv8SwapFixture{base: base, allowed: filepath.Join(base, "allowed"), outside: filepath.Join(base, "outside")}
	f.src, f.dst = filepath.Join(f.allowed, "src"), filepath.Join(f.allowed, "dst")
	for _, d := range []string{f.src, f.dst, f.outside} {
		require.NoError(t, os.MkdirAll(d, 0o755))
	}
	require.NoError(t, os.WriteFile(filepath.Join(f.src, "a.txt"), []byte("A"), 0o644))
	t.Setenv(pathGuardEnvSync, f.allowed)
	t.Setenv(pathGuardEnvRead, f.allowed)
	return f
}

func rv8LocalEndpoint(src, dst, mode string) *models.SyncEndpoint {
	settings := fmt.Sprintf(`{"source_directory":%q,"destination_directory":%q,"sync_mode":%q}`, src, dst, mode)
	return &models.SyncEndpoint{ID: 1, UserID: 1, Type: models.SyncTypeLocal, LocalPath: src,
		SyncDirection: models.SyncDirectionBidirectional, SyncSettings: &settings}
}

// swap replaces the directory p with a symlink to target.
func rv8Swap(t *testing.T, p, target string) {
	t.Helper()
	require.NoError(t, os.RemoveAll(p))
	require.NoError(t, os.Symlink(target, p))
}

var rv8Modes = []string{"mirror", "incremental", "bidirectional"}

func TestRV8_SourceRootSwapAfterConfineIsRefused(t *testing.T) {
	for _, mode := range rv8Modes {
		t.Run(mode, func(t *testing.T) {
			f := newRV8SwapFixture(t)
			require.NoError(t, os.WriteFile(filepath.Join(f.outside, "secret.txt"), []byte("SECRET"), 0o600))
			svc, _ := newSyncSvc(t)
			ep := rv8LocalEndpoint(f.src, f.dst, mode)
			require.NoError(t, svc.confineSyncPaths(ep))
			rv8Swap(t, f.src, f.outside)
			err := svc.performLocalSync(&models.SyncSession{}, ep)
			assert.Error(t, err, "a source root swapped to an outside directory was accepted")
			_, serr := os.Lstat(filepath.Join(f.dst, "secret.txt"))
			assert.True(t, os.IsNotExist(serr), "outside file copied into the destination through a swapped source root")
		})
	}
}

func TestRV8_DestinationRootSwapAfterConfineIsRefused(t *testing.T) {
	for _, mode := range rv8Modes {
		t.Run(mode, func(t *testing.T) {
			f := newRV8SwapFixture(t)
			precious := filepath.Join(f.outside, "precious.txt")
			require.NoError(t, os.WriteFile(precious, []byte("KEEP"), 0o600))
			svc, _ := newSyncSvc(t)
			ep := rv8LocalEndpoint(f.src, f.dst, mode)
			require.NoError(t, svc.confineSyncPaths(ep))
			rv8Swap(t, f.dst, f.outside)
			err := svc.performLocalSync(&models.SyncSession{}, ep)
			assert.Error(t, err, "a destination root swapped to an outside directory was accepted")
			b, perr := os.ReadFile(precious)
			assert.NoError(t, perr, "mirror cleanup deleted a file outside the roots through a swapped destination")
			assert.Equal(t, "KEEP", string(b))
			_, aerr := os.Lstat(filepath.Join(f.outside, "a.txt"))
			assert.True(t, os.IsNotExist(aerr), "a file was written outside the roots through a swapped destination")
		})
	}
}

func TestRV8_UploadSourceSwapAfterConfineIsRefused(t *testing.T) {
	t.Run("webdav", func(t *testing.T) {
		f := newRV8SwapFixture(t)
		require.NoError(t, os.WriteFile(filepath.Join(f.outside, "secret.txt"), []byte("SECRET"), 0o600))
		davRoot := t.TempDir()
		srv := httptest.NewServer(&webdav.Handler{FileSystem: webdav.Dir(davRoot), LockSystem: webdav.NewMemLS()})
		defer srv.Close()
		svc, _ := newSyncSvc(t)
		ep := &models.SyncEndpoint{ID: 1, UserID: 1, Name: "dav", Type: models.SyncTypeWebDAV, URL: srv.URL,
			SyncDirection: models.SyncDirectionUpload, LocalPath: f.src, RemotePath: "/r"}
		require.NoError(t, svc.confineSyncPaths(ep))
		rv8Swap(t, f.src, f.outside)
		err := svc.uploadToWebDAV(newRV5Session(t, svc, 1), ep, NewWebDAVClient(srv.URL, "", ""))
		assert.Error(t, err)
		var sent []string
		filepath.Walk(davRoot, func(p string, i os.FileInfo, e error) error {
			if e == nil && !i.IsDir() {
				sent = append(sent, p)
			}
			return nil
		})
		assert.Empty(t, sent, "files from outside the roots were uploaded through a swapped source root")
	})
	for _, kind := range []string{"s3", "gcs"} {
		t.Run(kind+" (shared uploadSourceDir)", func(t *testing.T) {
			f := newRV8SwapFixture(t)
			ep := &models.SyncEndpoint{ID: 1, UserID: 1, Type: kind, SyncDirection: models.SyncDirectionUpload, LocalPath: f.src}
			require.NoError(t, (&SyncService{}).confineSyncPaths(ep))
			rv8Swap(t, f.src, f.outside)
			_, err := uploadSourceDir(map[string]interface{}{}, ep)
			assert.Error(t, err, "uploadSourceDir resolved a swapped root to a directory outside the roots")
			_, err = uploadSourceDir(map[string]interface{}{"source_directory": f.src}, ep)
			assert.Error(t, err, "source_directory swapped to outside was accepted")
		})
	}
}

// golden-FALSE: legitimate symlinked layouts keep working - a NAS mount reached through a
// symlinked sync root (reviewer probe, adopted), and a link to a directory inside the roots.
func TestRV8_SymlinkedRootLayoutsKeepWorking(t *testing.T) {
	for _, mode := range rv8Modes {
		t.Run("nas root link/"+mode, func(t *testing.T) {
			base := t.TempDir()
			real := filepath.Join(base, "nas")
			require.NoError(t, os.MkdirAll(filepath.Join(real, "src", "d"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(real, "src", "d", "f.txt"), []byte("F"), 0o644))
			lnk := filepath.Join(base, "data")
			require.NoError(t, os.Symlink(real, lnk))
			t.Setenv(pathGuardEnvSync, lnk)
			t.Setenv(pathGuardEnvRead, lnk)
			svc, _ := newSyncSvc(t)
			ep := rv8LocalEndpoint(filepath.Join(lnk, "src"), filepath.Join(lnk, "dst"), mode)
			require.NoError(t, svc.confineSyncPaths(ep))
			require.NoError(t, svc.performLocalSync(&models.SyncSession{}, ep))
			b, err := os.ReadFile(filepath.Join(real, "dst", "d", "f.txt"))
			require.NoError(t, err)
			assert.Equal(t, "F", string(b))
		})
	}
	t.Run("in-root link as upload source", func(t *testing.T) {
		f := newRV8SwapFixture(t)
		lnk := filepath.Join(f.allowed, "srclink")
		require.NoError(t, os.Symlink(f.src, lnk))
		got, err := uploadSourceDir(map[string]interface{}{}, &models.SyncEndpoint{LocalPath: lnk})
		require.NoError(t, err)
		want, _ := filepath.EvalSymlinks(f.src)
		assert.Equal(t, want, got)
	})
}

// Review round 8, item 3 (M7): all destinations are checked before the first rename, so a
// refusal of a LATER output publishes nothing (ReadDir order is lexical: out-0 before out-1).
func TestRV8_PublishIsAllOrNothing(t *testing.T) {
	root, outside, src := rv6ConvFixture(t)
	rv7FakeTool(t, "convert", "multi")
	v := rv7Victim(t, outside)
	require.NoError(t, os.Symlink(v, filepath.Join(root, "out-1.bmp")))
	assert.Error(t, (&ConversionService{}).convertImage(&models.ConversionJob{SourcePath: src, TargetPath: filepath.Join(root, "out.bmp")}))
	_, err := os.Lstat(filepath.Join(root, "out-0.bmp"))
	assert.True(t, os.IsNotExist(err), "out-0.bmp was published although out-1.bmp was refused")
	rv7AssertVictim(t, v, "write through the planted out-1.bmp link")
	rv7NoPrivateDirLeft(t, root)
}

// Review round 8, item 3 (M16): the private converter directory is created 0700.
func TestRV8_PrivateDirIsOwnerOnly(t *testing.T) {
	root, _, src := rv6ConvFixture(t)
	rec := filepath.Join(t.TempDir(), "rec")
	t.Setenv("REC", rec)
	bin := t.TempDir()
	script := "#!/bin/sh\nfor a in \"$@\"; do out=\"$a\"; done\nstat -c '%a' \"$(dirname \"$out\")\" > \"$REC\"\nprintf OUT > \"$out\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "ffmpeg"), []byte(script), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	require.NoError(t, (&ConversionService{}).convertVideo(&models.ConversionJob{SourcePath: src, TargetPath: filepath.Join(root, "o.mp4")}))
	b, err := os.ReadFile(rec)
	require.NoError(t, err)
	assert.Equal(t, "700", strings.TrimSpace(string(b)))
}

// Review round 8, item 5: replacing an existing regular target keeps its permission bits (the
// published file is a new inode written by the tool under its own umask).
func TestRV8_ExistingTargetModeIsKept(t *testing.T) {
	root, _, src := rv6ConvFixture(t)
	bin := t.TempDir()
	script := "#!/bin/sh\nfor a in \"$@\"; do out=\"$a\"; done\numask 077\nprintf OUT > \"$out\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "ffmpeg"), []byte(script), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	tgt := filepath.Join(root, "o.mp4")
	require.NoError(t, os.WriteFile(tgt, []byte("OLD"), 0o644))
	require.NoError(t, os.Chmod(tgt, 0o664))
	require.NoError(t, (&ConversionService{}).convertVideo(&models.ConversionJob{SourcePath: src, TargetPath: tgt}))
	fi, err := os.Stat(tgt)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o664), fi.Mode().Perm())
	b, _ := os.ReadFile(tgt)
	assert.Equal(t, "OUT", string(b))
	// a new target (nothing to keep) gets the tool's own mode
	nt := filepath.Join(root, "n.mp4")
	require.NoError(t, (&ConversionService{}).convertVideo(&models.ConversionJob{SourcePath: src, TargetPath: nt}))
	fi, err = os.Stat(nt)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fi.Mode().Perm())
}

// performLocalSync uses the root set of the sync mode: a bidirectional sync writes its source,
// so a source that is only in the read roots is refused there too (not only in confineSyncPaths).
func TestRV8_BidirectionalSourceMustBeWritable(t *testing.T) {
	base := t.TempDir()
	ro, rw := filepath.Join(base, "ro"), filepath.Join(base, "rw")
	require.NoError(t, os.MkdirAll(filepath.Join(ro, "src"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(rw, "dst"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(rw, "dst", "b.txt"), []byte("B"), 0o644))
	t.Setenv(pathGuardEnvSync, rw)
	t.Setenv(pathGuardEnvRead, ro)
	svc, _ := newSyncSvc(t)
	err := svc.performLocalSync(&models.SyncSession{}, rv8LocalEndpoint(filepath.Join(ro, "src"), filepath.Join(rw, "dst"), "bidirectional"))
	assert.Error(t, err)
	_, serr := os.Lstat(filepath.Join(ro, "src", "b.txt"))
	assert.True(t, os.IsNotExist(serr), "bidirectional sync wrote into a read-only root")
	// golden-FALSE: the same source is fine for a one-way mirror
	require.NoError(t, svc.performLocalSync(&models.SyncSession{}, rv8LocalEndpoint(filepath.Join(ro, "src"), filepath.Join(rw, "dst"), "mirror")))
}

// An unknown sync mode is refused before anything touches the file system.
func TestRV8_UnsupportedModeCreatesNothing(t *testing.T) {
	f := newRV8SwapFixture(t)
	nd := filepath.Join(f.allowed, "new-dst")
	err := (&SyncService{}).performLocalSync(&models.SyncSession{}, rv8LocalEndpoint(f.src, nd, "weird"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported sync mode")
	_, serr := os.Lstat(nd)
	assert.True(t, os.IsNotExist(serr), "the destination was created for a sync that was refused")
}
