package services

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"catalogizer/internal/pathguard"
	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/webdav"
)

// Review round 9, F1. The walk roots are resolved and confined once (round 8), but an ANCESTOR
// of a resolved root (allowed/a for allowed/a/dst) can be swapped for a link to an outside tree
// after that check. The per-file guards confined against the walk-root STRING, which
// ConfineData(File) resolves again through the same swapped ancestor, so both sides moved
// together and the check passed. The per-file guards now also confine against the CONFIGURED
// roots (environment), which the swap does not move. Reviewer probes adopted from
// rv9.jXwU/c/catalog-api/services/zz_rv9_probe_test.go.

// rv9Fixture builds base/allowed (the only configured root) and base/outside, and returns a
// swap function that replaces allowed/a with a link to outside/a.
func rv9Fixture(t *testing.T) (allowed, outside string, swap func()) {
	t.Helper()
	base := t.TempDir()
	allowed = filepath.Join(base, "allowed")
	outside = filepath.Join(base, "outside")
	require.NoError(t, os.MkdirAll(filepath.Join(allowed, "a"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(outside, "a"), 0o755))
	t.Setenv(pathGuardEnvSync, allowed)
	t.Setenv(pathGuardEnvRead, allowed)
	swap = func() {
		require.NoError(t, os.RemoveAll(filepath.Join(allowed, "a")))
		require.NoError(t, os.Symlink(filepath.Join(outside, "a"), filepath.Join(allowed, "a")))
	}
	return allowed, outside, swap
}

// rv9-P1: mirror cleanup must not delete through a swapped ancestor of the destination root.
func TestRV9_CleanupRefusesSwappedAncestor(t *testing.T) {
	allowed, outside, swap := rv9Fixture(t)
	src := filepath.Join(allowed, "src")
	dst := filepath.Join(allowed, "a", "dst")
	require.NoError(t, os.MkdirAll(src, 0o755))
	require.NoError(t, os.MkdirAll(dst, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(outside, "a", "dst", "emptydir"), 0o755))
	precious := filepath.Join(outside, "a", "dst", "precious.txt")
	require.NoError(t, os.WriteFile(precious, []byte("KEEP"), 0o600))
	rd, err := confinedWalkRoot(dst, []string{allowed})
	require.NoError(t, err)
	swap()
	cerr := (&SyncService{}).cleanupDestination(src, rd, &models.SyncSession{})
	assert.Error(t, cerr, "cleanup through a swapped ancestor must fail closed")
	_, perr := os.Stat(precious)
	assert.NoError(t, perr, "cleanup deleted a file outside the roots through a swapped ancestor")
	_, derr := os.Stat(filepath.Join(outside, "a", "dst", "emptydir"))
	assert.NoError(t, derr, "cleanup removed a directory outside the roots through a swapped ancestor")
}

// rv9-P2: incremental and mirror copy must not write through a swapped ancestor (destination),
// and the mirror directory guard must not create directories there.
func TestRV9_CopyRefusesSwappedAncestor(t *testing.T) {
	for _, mode := range []string{"incremental", "mirror"} {
		t.Run(mode, func(t *testing.T) {
			allowed, outside, swap := rv9Fixture(t)
			src := filepath.Join(allowed, "src")
			dst := filepath.Join(allowed, "a", "dst")
			require.NoError(t, os.MkdirAll(filepath.Join(src, "d"), 0o755))
			require.NoError(t, os.MkdirAll(dst, 0o755))
			require.NoError(t, os.MkdirAll(filepath.Join(outside, "a", "dst"), 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(src, "x.txt"), []byte("X"), 0o644))
			require.NoError(t, os.WriteFile(filepath.Join(src, "d", "y.txt"), []byte("Y"), 0o644))
			rs, err := confinedWalkRoot(src, []string{allowed})
			require.NoError(t, err)
			rd, err := confinedWalkRoot(dst, []string{allowed})
			require.NoError(t, err)
			swap()
			svc := &SyncService{}
			var serr error
			if mode == "mirror" {
				serr = svc.performMirrorSync(rs, rd, &models.SyncSession{})
			} else {
				serr = svc.performIncrementalSync(rs, rd, &models.SyncSession{})
			}
			assert.Error(t, serr, "%s through a swapped ancestor must fail closed", mode)
			for _, p := range []string{"x.txt", "d", filepath.Join("d", "y.txt")} {
				_, lerr := os.Lstat(filepath.Join(outside, "a", "dst", p))
				assert.True(t, os.IsNotExist(lerr), "%s wrote %s outside the roots through a swapped ancestor", mode, p)
			}
		})
	}
}

// Source side of the same class: a swapped ancestor of the SOURCE root must not let the copy
// or the upload scan read an outside file.
func TestRV9_ReadRefusesSwappedSourceAncestor(t *testing.T) {
	allowed, outside, swap := rv9Fixture(t)
	src := filepath.Join(allowed, "a", "src")
	dst := filepath.Join(allowed, "dst")
	require.NoError(t, os.MkdirAll(src, 0o755))
	require.NoError(t, os.MkdirAll(dst, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(outside, "a", "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "a", "src", "secret.txt"), []byte("SECRET"), 0o600))
	rs, err := confinedWalkRoot(src, []string{allowed})
	require.NoError(t, err)
	rd, err := confinedWalkRoot(dst, []string{allowed})
	require.NoError(t, err)
	swap()
	_ = (&SyncService{}).performIncrementalSync(rs, rd, &models.SyncSession{})
	_, lerr := os.Lstat(filepath.Join(dst, "secret.txt"))
	assert.True(t, os.IsNotExist(lerr), "copy read an outside file through a swapped source ancestor")
	files, _ := (&SyncService{}).scanLocalFiles(rs)
	assert.Empty(t, files, "upload scan listed an outside file through a swapped source ancestor")
}

// WebDAV download wrote below endpoint.LocalPath with a per-file check against LocalPath only,
// so a swapped ancestor of LocalPath sent the download outside the roots.
func TestRV9_WebDAVDownloadRefusesSwappedAncestor(t *testing.T) {
	allowed, outside, swap := rv9Fixture(t)
	local := filepath.Join(allowed, "a", "dl")
	require.NoError(t, os.MkdirAll(local, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(outside, "a", "dl"), 0o755))
	davRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(davRoot, "r"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(davRoot, "r", "f.txt"), []byte("F"), 0o644))
	srv := httptest.NewServer(&webdav.Handler{FileSystem: webdav.Dir(davRoot), LockSystem: webdav.NewMemLS()})
	defer srv.Close()
	svc, _ := newSyncSvc(t)
	ep := &models.SyncEndpoint{ID: 1, UserID: 1, Name: "dav", Type: models.SyncTypeWebDAV, URL: srv.URL,
		SyncDirection: models.SyncDirectionDownload, LocalPath: local, RemotePath: "/r"}
	swap()
	sess := newRV5Session(t, svc, 1)
	_ = svc.downloadFromWebDAV(sess, ep, NewWebDAVClient(srv.URL, "", ""))
	_, lerr := os.Lstat(filepath.Join(outside, "a", "dl", "f.txt"))
	assert.True(t, os.IsNotExist(lerr), "WebDAV download wrote outside the roots through a swapped ancestor")
	assert.GreaterOrEqual(t, sess.FailedFiles, 1, "the refused download must be counted as failed")
}

// Golden-FALSE (rv9-P4 and the NAS layout): with no swap, a configured root that is itself a
// symlink (a NAS mount point) and roots that contain linked directories keep working: the
// configured-roots check is the same resolution pathguard applies everywhere, so a link that
// stays inside the roots is admitted. The probe's two-sided expectation is kept: the walk-root
// check alone passes the swapped path, the configured-roots check refuses it.
func TestRV9_ConfiguredRootsCheckAdmitsLegitimateLayouts(t *testing.T) {
	t.Run("probe_sketch", func(t *testing.T) {
		allowed, _, swap := rv9Fixture(t)
		dst := filepath.Join(allowed, "a", "dst")
		require.NoError(t, os.MkdirAll(dst, 0o755))
		rd, err := confinedWalkRoot(dst, []string{allowed})
		require.NoError(t, err)
		f := filepath.Join(rd, "x.txt")
		_, e0 := pathguard.ConfineDataFile(f, []string{allowed})
		assert.NoError(t, e0, "before the swap the configured-roots check must admit the path")
		swap()
		_, e1 := pathguard.ConfineDataFile(f, []string{rd})
		_, e2 := pathguard.ConfineDataFile(f, []string{allowed})
		assert.NoError(t, e1, "walk-root-only check passes the swapped path (the gap being closed)")
		assert.Error(t, e2, "configured-roots check refuses the swapped path")
	})
	t.Run("nas_link_root_all_modes", func(t *testing.T) {
		base := t.TempDir()
		nas := filepath.Join(base, "nas")
		mount := filepath.Join(base, "mnt")
		require.NoError(t, os.MkdirAll(filepath.Join(nas, "src", "d"), 0o755))
		require.NoError(t, os.MkdirAll(filepath.Join(nas, "dst"), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(nas, "src", "d", "f.txt"), []byte("F"), 0o644))
		require.NoError(t, os.WriteFile(filepath.Join(nas, "dst", "stale.txt"), []byte("S"), 0o644))
		require.NoError(t, os.Symlink(nas, mount))
		t.Setenv(pathGuardEnvSync, mount)
		t.Setenv(pathGuardEnvRead, mount)
		for _, mode := range []string{"mirror", "incremental", "bidirectional"} {
			require.NoError(t, rv7LocalSync(t, filepath.Join(mount, "src"), filepath.Join(mount, "dst"), mode), mode)
			b, err := os.ReadFile(filepath.Join(nas, "dst", "d", "f.txt"))
			require.NoError(t, err, mode)
			assert.Equal(t, "F", string(b), mode)
		}
		files, err := (&SyncService{}).scanLocalFiles(filepath.Join(mount, "src"))
		require.NoError(t, err)
		assert.Len(t, files, 1, "upload scan under a NAS link root")
	})
}

// Review round 9, F5 (mutant N6): only the permission bits of a replaced target are kept, never
// setuid, setgid or sticky. A pre-created 06755/01755 target must come out as plain 0755.
func TestRV9_KeptTargetModeDropsSpecialBits(t *testing.T) {
	root, _, src := rv6ConvFixture(t)
	bin := t.TempDir()
	script := "#!/bin/sh\nfor a in \"$@\"; do out=\"$a\"; done\numask 077\nprintf OUT > \"$out\"\n"
	require.NoError(t, os.WriteFile(filepath.Join(bin, "ffmpeg"), []byte(script), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, special := range []os.FileMode{os.ModeSetuid | os.ModeSetgid, os.ModeSticky} {
		tgt := filepath.Join(root, "s"+special.String()[:2]+".mp4")
		require.NoError(t, os.WriteFile(tgt, []byte("OLD"), 0o644))
		require.NoError(t, os.Chmod(tgt, 0o755|special))
		fi, err := os.Stat(tgt)
		require.NoError(t, err)
		if fi.Mode()&special == 0 {
			t.Skipf("file system did not keep %v on the fixture (cannot test)", special)
		}
		require.NoError(t, (&ConversionService{}).convertVideo(&models.ConversionJob{SourcePath: src, TargetPath: tgt}))
		fi, err = os.Stat(tgt)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0o755), fi.Mode().Perm(), "permission bits kept")
		assert.Zero(t, fi.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky), "special bits must not be kept: %v", fi.Mode())
	}
}

// Sink-level guard (round-9 mutant M9-2): both copyFile callers run uploadableFile first, which
// already refuses a swapped source ancestor, so a walker-level test cannot see copyFile's own
// source check. copyFile is a sink in its own right and must refuse on its own.
func TestRV9_CopyFileRefusesSwappedSourceAncestorDirectly(t *testing.T) {
	allowed, outside, swap := rv9Fixture(t)
	src := filepath.Join(allowed, "a", "src")
	dst := filepath.Join(allowed, "dst")
	require.NoError(t, os.MkdirAll(src, 0o755))
	require.NoError(t, os.MkdirAll(dst, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(outside, "a", "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "a", "src", "secret.txt"), []byte("SECRET"), 0o600))
	rs, err := confinedWalkRoot(src, []string{allowed})
	require.NoError(t, err)
	rd, err := confinedWalkRoot(dst, []string{allowed})
	require.NoError(t, err)
	swap()
	err = (&SyncService{}).copyFile(filepath.Join(rs, "secret.txt"), filepath.Join(rd, "secret.txt"), 0o644, rs, rd)
	assert.Error(t, err, "copyFile read a source through a swapped ancestor")
	_, lerr := os.Lstat(filepath.Join(dst, "secret.txt"))
	assert.True(t, os.IsNotExist(lerr), "outside content was copied into the destination")
}

// Round-9 mutant M9-9: the destination root itself is never removed, and reaching it with the
// source gone is a normal "nothing to do" (nil), not an error.
func TestRV9_CleanupKeepsDestinationRoot(t *testing.T) {
	allowed, _, _ := rv9Fixture(t)
	dst := filepath.Join(allowed, "dst")
	require.NoError(t, os.MkdirAll(dst, 0o755))
	rd, err := confinedWalkRoot(dst, []string{allowed})
	require.NoError(t, err)
	missing := filepath.Join(allowed, "gone")
	err = (&SyncService{}).cleanupDestination(missing, rd, &models.SyncSession{})
	assert.NoError(t, err, "an empty destination root with a missing source must not be an error")
	fi, serr := os.Stat(dst)
	assert.True(t, serr == nil && fi.IsDir(), "the destination root was removed")
}
