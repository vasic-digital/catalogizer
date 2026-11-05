package services

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Reviewer-authored (round 1) probes.

func TestRV_ForcedTagAndBOM(t *testing.T) {
	e := newSidecarEnv(t)
	e.video(t, "Movie.mp4")
	e.write(t, "Movie.en.forced.srt", "\xef\xbb\xbf"+srtBody)
	e.scan(t)
	tr := e.tracks(t, e.fileID(t, "Movie.mp4"))
	require.Len(t, tr, 1)
	assert.True(t, tr[0].IsForced, "forced tag must set is_forced")
	assert.Equal(t, "en", tr[0].LanguageCode)
	require.NotNil(t, tr[0].Content)
	assert.NotContains(t, *tr[0].Content, "\xef\xbb\xbf", "BOM must be stripped")
}

func TestRV_FIFOSidecarDoesNotHangScan(t *testing.T) {
	e := newSidecarEnv(t)
	e.video(t, "Movie.mp4")
	require.NoError(t, syscall.Mkfifo(filepath.Join(e.root, "Movie.en.srt"), 0o644))
	done := make(chan struct{})
	go func() { e.scan(t); close(done) }()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		// unblock the reader so the test process can exit
		f, _ := os.OpenFile(filepath.Join(e.root, "Movie.en.srt"), os.O_WRONLY, 0)
		if f != nil {
			f.Close()
		}
		t.Fatal("scan hung on a FIFO sidecar")
	}
	var n int
	require.NoError(t, e.db.QueryRow("SELECT COUNT(*) FROM files WHERE name = 'Movie.en.srt'").Scan(&n))
	t.Logf("FIFO cataloged rows: %d", n)
	assert.Equal(t, 0, e.allTrackCount(t))
}

func TestRV_SymlinkFileCataloged(t *testing.T) {
	e := newSidecarEnv(t)
	e.video(t, "Movie.mp4")
	outside := filepath.Join(t.TempDir(), "x.srt")
	require.NoError(t, os.WriteFile(outside, []byte(srtBody), 0o644))
	require.NoError(t, os.Symlink(outside, filepath.Join(e.root, "Movie.it.srt")))
	e.scan(t)
	var n int
	require.NoError(t, e.db.QueryRow("SELECT COUNT(*) FROM files WHERE name = 'Movie.it.srt'").Scan(&n))
	t.Logf("symlink file cataloged rows: %d", n)
}

func TestRV_SymlinkedDirEscape(t *testing.T) {
	e := newSidecarEnv(t)
	out := t.TempDir()
	e.video(t, "Movie.mp4") // ensures ffmpeg present
	require.NoError(t, os.Rename(filepath.Join(e.root, "Movie.mp4"), filepath.Join(out, "Movie.mp4")))
	require.NoError(t, os.WriteFile(filepath.Join(out, "Movie.en.srt"), []byte(srtBody+"\nOUTSIDE\n"), 0o644))
	require.NoError(t, os.Symlink(out, filepath.Join(e.root, "link")))
	e.scan(t)
	var n int
	require.NoError(t, e.db.QueryRow("SELECT COUNT(*) FROM files WHERE path LIKE '/link/%'").Scan(&n))
	t.Logf("files cataloged under symlinked dir: %d", n)
	assert.Equal(t, 0, e.allTrackCount(t), "a sidecar resolving outside the root must not be imported")
}

// F1/F2: the read path itself must be race-safe (adopted from review round 1; R1 mutant target).
func TestReadConfinedSidecar_FIFOReturnsPromptly(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, syscall.Mkfifo(filepath.Join(root, "a.srt"), 0o644))
	done := make(chan bool, 1)
	go func() { _, ok := readConfinedSidecar(root, "/a.srt"); done <- ok }()
	select {
	case ok := <-done:
		assert.False(t, ok)
	case <-time.After(5 * time.Second):
		t.Fatal("readConfinedSidecar blocked on a FIFO")
	}
}

// A symlink at the leaf (the state after an attacker swaps the checked file) must not be read.
func TestReadConfinedSidecar_LeafSymlinkOutsideRefused(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(out, "x.srt"), []byte(srtBody), 0o644))
	require.NoError(t, os.Symlink(filepath.Join(out, "x.srt"), filepath.Join(root, "a.srt")))
	_, ok := readConfinedSidecar(root, "/a.srt")
	assert.False(t, ok)
}

// Post-check swap: the file passes the path checks, then is replaced by a symlink to an outside
// file before the open. Simulated through the test hook that runs between check and open.
func TestReadConfinedSidecar_SwapBetweenCheckAndOpenRefused(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	p := filepath.Join(root, "a.srt")
	require.NoError(t, os.WriteFile(p, []byte(srtBody), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(out, "x.srt"), []byte(srtBody+"\nOUTSIDE\n"), 0o644))
	sidecarAfterCheck = func() {
		_ = os.Remove(p)
		_ = os.Symlink(filepath.Join(out, "x.srt"), p)
	}
	defer func() { sidecarAfterCheck = nil }()
	s, ok := readConfinedSidecar(root, "/a.srt")
	assert.False(t, ok, "swapped-in symlink must not be read; got %q", s)
}

func TestReadConfinedSidecar_RegularFileReadAndBOMStripped(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "a.srt"), []byte("\xef\xbb\xbf"+srtBody), 0o644))
	s, ok := readConfinedSidecar(root, "/a.srt")
	require.True(t, ok)
	assert.Equal(t, srtBody, s)
}

// Swap to a DIFFERENT regular file inside the root: only the SameFile identity check refuses it.
func TestReadConfinedSidecar_SwapToOtherRegularFileRefused(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "a.srt")
	require.NoError(t, os.WriteFile(p, []byte(srtBody), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.srt"), []byte(srtBody+"\nOTHER\n"), 0o644))
	sidecarAfterCheck = func() { _ = os.Rename(filepath.Join(root, "b.srt"), p) }
	defer func() { sidecarAfterCheck = nil }()
	s, ok := readConfinedSidecar(root, "/a.srt")
	assert.False(t, ok, "a different file than the checked one must not be read; got %q", s)
}

// Swap to a symlink to another file INSIDE the root (descriptor realpath stays inside): only the
// leaf O_NOFOLLOW / identity check refuses it.
func TestReadConfinedSidecar_SwapToInsideSymlinkRefused(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "a.srt")
	require.NoError(t, os.WriteFile(p, []byte(srtBody), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "b.srt"), []byte(srtBody+"\nOTHER\n"), 0o644))
	sidecarAfterCheck = func() { _ = os.Remove(p); _ = os.Symlink(filepath.Join(root, "b.srt"), p) }
	defer func() { sidecarAfterCheck = nil }()
	s, ok := readConfinedSidecar(root, "/a.srt")
	assert.False(t, ok, "got %q", s)
}

// Parent directory swapped for a symlink to an outside directory holding a HARDLINK of the same
// file: leaf is regular, same inode, not a symlink; only the descriptor realpath re-check refuses.
func TestReadConfinedSidecar_ParentDirSwapToOutsideRefused(t *testing.T) {
	root, out := t.TempDir(), t.TempDir()
	require.NoError(t, os.Mkdir(filepath.Join(root, "sub"), 0o755))
	p := filepath.Join(root, "sub", "a.srt")
	require.NoError(t, os.WriteFile(p, []byte(srtBody), 0o644))
	require.NoError(t, os.Link(p, filepath.Join(out, "a.srt")))
	sidecarAfterCheck = func() {
		_ = os.Rename(filepath.Join(root, "sub"), filepath.Join(root, "sub.bak"))
		_ = os.Symlink(out, filepath.Join(root, "sub"))
	}
	defer func() { sidecarAfterCheck = nil }()
	s, ok := readConfinedSidecar(root, "/sub/a.srt")
	assert.False(t, ok, "descriptor resolving outside the root must be refused; got %q", s)
}

// The file is swapped for a FIFO after the checks: the open must not block (O_NONBLOCK) and the
// descriptor must be refused (not regular).
func TestReadConfinedSidecar_SwapToFIFOAfterCheckDoesNotBlock(t *testing.T) {
	root := t.TempDir()
	p := filepath.Join(root, "a.srt")
	require.NoError(t, os.WriteFile(p, []byte(srtBody), 0o644))
	sidecarAfterCheck = func() { _ = os.Remove(p); _ = syscall.Mkfifo(p, 0o644) }
	defer func() { sidecarAfterCheck = nil }()
	done := make(chan bool, 1)
	go func() { _, ok := readConfinedSidecar(root, "/a.srt"); done <- ok }()
	select {
	case ok := <-done:
		assert.False(t, ok)
	case <-time.After(5 * time.Second):
		f, _ := os.OpenFile(p, os.O_WRONLY|syscall.O_NONBLOCK, 0)
		if f != nil {
			f.Close()
		}
		t.Fatal("open blocked on a FIFO swapped in after the check")
	}
}
