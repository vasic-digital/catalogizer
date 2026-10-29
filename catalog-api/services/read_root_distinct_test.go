package services

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/webdav"
)

// Read roots and sync write roots as DIFFERENT directories. The configured-roots checks of
// review round 9 must keep sync working from a read root (golden-FALSE) while a write through
// an ancestor swapped to the read root itself is refused (the read root is not a write root).
// Probes P1, P2 and P5 adopted from the round-10 review.

// Golden-FALSE (review round 10, probe P1): read roots and sync write roots are DIFFERENT directories (a media
// library mirrored into ./data). Mirror and incremental from the read root must keep working.
func TestReadRootSourceDistinctFromSyncRoot(t *testing.T) {
	base := t.TempDir()
	media := filepath.Join(base, "media")
	data := filepath.Join(base, "data")
	require.NoError(t, os.MkdirAll(filepath.Join(media, "lib", "d"), 0o755))
	require.NoError(t, os.MkdirAll(data, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(media, "lib", "d", "f.txt"), []byte("F"), 0o644))
	t.Setenv(pathGuardEnvRead, media)
	t.Setenv(pathGuardEnvSync, data)
	for _, mode := range []string{"mirror", "incremental"} {
		dst := filepath.Join(data, "out-"+mode)
		require.NoError(t, rv7LocalSync(t, filepath.Join(media, "lib"), dst, mode), mode)
		b, err := os.ReadFile(filepath.Join(dst, "d", "f.txt"))
		require.NoError(t, err, mode)
		assert.Equal(t, "F", string(b), mode)
	}
	files, err := (&SyncService{}).scanLocalFiles(filepath.Join(media, "lib"))
	require.NoError(t, err)
	assert.Len(t, files, 1, "upload scan from a read root")
}

// Golden-FALSE (review round 10, probe P2): bidirectional with both sides in the sync root while a separate read
// root exists; the reverse direction reads the destination side (a write root).
func TestBidirectionalReadsWriteRootWhenReadRootDiffers(t *testing.T) {
	base := t.TempDir()
	media := filepath.Join(base, "media")
	data := filepath.Join(base, "data")
	require.NoError(t, os.MkdirAll(media, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(data, "a"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(data, "b"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(data, "b", "back.txt"), []byte("B"), 0o644))
	t.Setenv(pathGuardEnvRead, media)
	t.Setenv(pathGuardEnvSync, data)
	require.NoError(t, rv7LocalSync(t, filepath.Join(data, "a"), filepath.Join(data, "b"), "bidirectional"))
	b, err := os.ReadFile(filepath.Join(data, "a", "back.txt"))
	require.NoError(t, err)
	assert.Equal(t, "B", string(b))
}

// Review round 10, probe P5: the ancestor is swapped for a link to the READ root ITSELF (so prefixesStayWithin
// enters the read root at the root). Copy, WebDAV download and cleanup must still refuse,
// because the read root is not a write root.
func TestSwapToReadRootItselfIsRefused(t *testing.T) {
	base := t.TempDir()
	media := filepath.Join(base, "media")
	data := filepath.Join(base, "data")
	require.NoError(t, os.MkdirAll(filepath.Join(data, "a", "dst"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(data, "src"), 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(media, "dst"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(data, "src", "x.txt"), []byte("X"), 0o644))
	keep := filepath.Join(media, "dst", "movie.mkv")
	require.NoError(t, os.WriteFile(keep, []byte("M"), 0o644))
	t.Setenv(pathGuardEnvRead, media)
	t.Setenv(pathGuardEnvSync, data)
	rs, err := confinedWalkRoot(filepath.Join(data, "src"), []string{data})
	require.NoError(t, err)
	rd, err := confinedWalkRoot(filepath.Join(data, "a", "dst"), []string{data})
	require.NoError(t, err)
	require.NoError(t, os.RemoveAll(filepath.Join(data, "a")))
	require.NoError(t, os.Symlink(media, filepath.Join(data, "a")))
	ierr := (&SyncService{}).performIncrementalSync(rs, rd, &models.SyncSession{})
	t.Logf("incr err: %v", ierr)
	_, lerr := os.Lstat(filepath.Join(media, "dst", "x.txt"))
	assert.True(t, os.IsNotExist(lerr), "copy wrote into the read root through a swap")
	cerr := (&SyncService{}).cleanupDestination(filepath.Join(data, "src"), rd, &models.SyncSession{})
	t.Logf("cleanup err: %v", cerr)
	_, serr := os.Stat(keep)
	assert.NoError(t, serr, "cleanup removed a read-root file through a swap")

	davRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(davRoot, "r"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(davRoot, "r", "f.txt"), []byte("F"), 0o644))
	srv := httptest.NewServer(&webdav.Handler{FileSystem: webdav.Dir(davRoot), LockSystem: webdav.NewMemLS()})
	defer srv.Close()
	svc, _ := newSyncSvc(t)
	ep := &models.SyncEndpoint{ID: 1, UserID: 1, Name: "dav", Type: models.SyncTypeWebDAV, URL: srv.URL,
		SyncDirection: models.SyncDirectionDownload, LocalPath: filepath.Join(data, "a", "dst"), RemotePath: "/r"}
	sess := newRV5Session(t, svc, 1)
	_ = svc.downloadFromWebDAV(sess, ep, NewWebDAVClient(srv.URL, "", ""))
	_, lerr = os.Lstat(filepath.Join(media, "dst", "f.txt"))
	assert.True(t, os.IsNotExist(lerr), "WebDAV download wrote into the read root through a swap")
}
