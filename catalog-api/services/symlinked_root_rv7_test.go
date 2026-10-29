package services

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"catalogizer/models"

	"golang.org/x/net/webdav"
)

// rv7LocalSync drives a local sync through performLocalSync, the entry point that resolves and
// confines the roots (since review round 8 the per-mode walkers walk the roots as given).
func rv7LocalSync(t *testing.T, src, dst, mode string) error {
	t.Helper()
	return (&SyncService{}).performLocalSync(&models.SyncSession{}, rv8LocalEndpoint(src, dst, mode))
}

// Reviewer probe adopted verbatim (round-7 review of e5b65d1d, item S1; only the test names changed): an upload source that is a symlink to a directory inside
// the root passes requireUploadSourceDir (os.Stat follows it) but filepath.Walk does not
// descend into a symlinked root, so the upload reports success with zero files.
func TestRV7_ReviewerProbe_LinkedSourceDir(t *testing.T) {
	base, src, _, _, _ := perFileFixture(t)
	link := filepath.Join(base, "srclink")
	if err := os.Symlink(src, link); err != nil {
		t.Fatal(err)
	}
	allowSyncDirs(t, base)
	davRoot := t.TempDir()
	srv := httptest.NewServer(&webdav.Handler{FileSystem: webdav.Dir(davRoot), LockSystem: webdav.NewMemLS()})
	defer srv.Close()
	svc, _ := newSyncSvc(t)
	ep := &models.SyncEndpoint{ID: 1, UserID: 1, Name: "dav", Type: models.SyncTypeWebDAV, URL: srv.URL,
		SyncDirection: models.SyncDirectionUpload, LocalPath: link, RemotePath: "/r"}
	sess := newRV5Session(t, svc, 1)
	err := svc.uploadToWebDAV(sess, ep, NewWebDAVClient(srv.URL, "", ""))
	var n int
	filepath.Walk(davRoot, func(p string, i os.FileInfo, e error) error {
		if e == nil && !i.IsDir() {
			n++
		}
		return nil
	})
	t.Logf("PROBE symlinked-source: err=%v total=%d synced=%d remote_files=%d", err, sess.TotalFiles, sess.SyncedFiles, n)
	if err == nil && n == 0 {
		t.Errorf("symlinked upload source: success with 0 files uploaded (silent no-op)")
	}
	// Review round 8, item 2 (M12): the remote name is computed against the resolved source
	// root, so a.txt lands at /r/a.txt (not /r/../src/a.txt, i.e. outside RemotePath).
	if _, serr := os.Stat(filepath.Join(davRoot, "r", "a.txt")); serr != nil {
		t.Errorf("symlinked upload source: a.txt not uploaded as /r/a.txt: %v", serr)
	}
	if n != 1 {
		t.Errorf("symlinked upload source: %d remote files, want exactly 1 (/r/a.txt)", n)
	}
}

// Probe: mirror local sync with a symlinked source directory.
func TestRV7_ReviewerProbe_LinkedSourceMirror(t *testing.T) {
	base, src, dst, _, _ := perFileFixture(t)
	link := filepath.Join(base, "srclink")
	if err := os.Symlink(src, link); err != nil {
		t.Fatal(err)
	}
	allowSyncDirs(t, base)
	err := rv7LocalSync(t, link, dst, "mirror")
	_, serr := os.Stat(filepath.Join(dst, "a.txt"))
	t.Logf("PROBE mirror symlinked source: err=%v dst/a.txt stat=%v", err, serr)
	if err == nil && serr != nil {
		t.Errorf("mirror with symlinked source: success, nothing copied")
	}
}

// S1 class-wide: every root is resolved once (performLocalSync / uploadSourceDir), so a symlinked root is walked as the
// directory it names instead of being reported as itself and skipped (a silent 0-file sync).
func TestRV7_LinkedRootsAreWalked(t *testing.T) {
	t.Run("incremental source", func(t *testing.T) {
		base, src, dst, _, _ := perFileFixture(t)
		link := filepath.Join(base, "srclink")
		if err := os.Symlink(src, link); err != nil {
			t.Fatal(err)
		}
		allowSyncDirs(t, base)
		if err := rv7LocalSync(t, link, dst, "incremental"); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(dst, "a.txt")); err != nil {
			t.Errorf("incremental with a symlinked source copied nothing: %v", err)
		}
	})
	t.Run("bidirectional reverse pass with linked destination", func(t *testing.T) {
		base, src, dst, _, _ := perFileFixture(t)
		if err := os.WriteFile(filepath.Join(dst, "b.txt"), []byte("b"), 0o644); err != nil {
			t.Fatal(err)
		}
		link := filepath.Join(base, "dstlink")
		if err := os.Symlink(dst, link); err != nil {
			t.Fatal(err)
		}
		allowSyncDirs(t, base)
		if err := rv7LocalSync(t, src, link, "bidirectional"); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(filepath.Join(src, "b.txt")); err != nil {
			t.Errorf("reverse pass over a symlinked destination copied nothing: %v", err)
		}
	})
	t.Run("S3/GCS source resolves to the linked directory", func(t *testing.T) {
		base, src, _, _, _ := perFileFixture(t)
		link := filepath.Join(base, "srclink")
		if err := os.Symlink(src, link); err != nil {
			t.Fatal(err)
		}
		allowSyncDirs(t, base)
		got, err := uploadSourceDir(map[string]interface{}{}, &models.SyncEndpoint{LocalPath: link})
		if err != nil {
			t.Fatal(err)
		}
		real, _ := filepath.EvalSymlinks(src)
		if got != real {
			t.Errorf("uploadSourceDir(%q) = %q, want the resolved directory %q (filepath.Walk skips a symlinked root)", link, got, real)
		}
	})
	t.Run("link to a file is still refused", func(t *testing.T) {
		base, src, _, _, _ := perFileFixture(t)
		link := filepath.Join(base, "filelink")
		if err := os.Symlink(filepath.Join(src, "a.txt"), link); err != nil {
			t.Fatal(err)
		}
		allowSyncDirs(t, base)
		if _, err := uploadSourceDir(map[string]interface{}{}, &models.SyncEndpoint{LocalPath: link}); err == nil {
			t.Errorf("a symlink to a regular file was accepted as an upload source")
		}
	})
}

// The destination root is resolved as well (by performLocalSync): a mirror sync into a linked destination removes the
// destination files that are not in the source (cleanupDestination walks the destination root).
func TestRV7_MirrorLinkedDestinationIsCleaned(t *testing.T) {
	base, src, dst, _, _ := perFileFixture(t)
	stale := filepath.Join(dst, "stale.txt")
	if err := os.WriteFile(stale, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "dstlink")
	if err := os.Symlink(dst, link); err != nil {
		t.Fatal(err)
	}
	allowSyncDirs(t, base)
	if err := rv7LocalSync(t, src, link, "mirror"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dst, "a.txt")); err != nil {
		t.Errorf("mirror into a linked destination copied nothing: %v", err)
	}
	if _, err := os.Lstat(stale); !os.IsNotExist(err) {
		t.Errorf("mirror into a linked destination left a file that is not in the source: %v", err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the destination link itself was changed: %v", err)
	}
}
