package services

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/webdav"
)

// N1 follow-through (review round 3): a WebDAV download entry is checked with the
// symlink-resolving ConfineData, not the lexical Within. A symlink under local_path that points
// outside the sync root must not carry a downloaded file out. The server is a REAL
// golang.org/x/net/webdav handler on a temp dir; the control file lands normally.
func TestPathConfine_WebDAVDownloadThroughLocalSymlinkRefused(t *testing.T) {
	allowed := t.TempDir()
	outside := t.TempDir()
	t.Setenv(pathGuardEnvSync, allowed)
	t.Setenv(pathGuardEnvRead, allowed)

	davRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(davRoot, "remote", "lnk"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(davRoot, "remote", "ok.txt"), []byte("real"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(davRoot, "remote", "lnk", "pwn.txt"), []byte("PWNED"), 0o644))
	srv := httptest.NewServer(&webdav.Handler{FileSystem: webdav.Dir(davRoot), LockSystem: webdav.NewMemLS()})
	defer srv.Close()

	local := filepath.Join(allowed, "local")
	require.NoError(t, os.MkdirAll(local, 0o755))
	require.NoError(t, os.Symlink(outside, filepath.Join(local, "lnk")))

	svc, repo := newSyncSvc(t)
	ep := &models.SyncEndpoint{ID: 1, UserID: 1, Name: "dav", Type: models.SyncTypeWebDAV, URL: srv.URL,
		SyncDirection: models.SyncDirectionDownload, LocalPath: local, RemotePath: "/remote"}
	sess := &models.SyncSession{EndpointID: 1, UserID: 1, Status: models.SyncSessionStatusRunning, StartedAt: time.Now(), SyncType: models.SyncTypeManual}
	sid, err := repo.CreateSession(sess)
	require.NoError(t, err)
	sess.ID = sid
	_ = svc.downloadFromWebDAV(sess, ep, NewWebDAVClient(srv.URL, "", ""))

	b, rerr := os.ReadFile(filepath.Join(local, "ok.txt"))
	require.NoError(t, rerr, "control: the ordinary file must download")
	assert.Equal(t, "real", string(b))
	_, serr := os.Stat(filepath.Join(outside, "pwn.txt"))
	assert.True(t, os.IsNotExist(serr), "a download followed lnk out of the sync root")
	assert.GreaterOrEqual(t, sess.FailedFiles, 1, "the escaping entry must be counted as failed")
}
