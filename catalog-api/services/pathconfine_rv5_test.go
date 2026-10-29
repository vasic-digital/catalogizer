package services

// Review round 5: the WebDAV upload walker, file targets equal to their root, and mutation
// killers for checks round 4 added (V5, V9, V10).

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/internal/pathguard"
	"catalogizer/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/webdav"
)

func newRV5Session(t *testing.T, svc *SyncService, endpointID int) *models.SyncSession {
	t.Helper()
	sess := &models.SyncSession{EndpointID: endpointID, UserID: 1, Status: models.SyncSessionStatusRunning, StartedAt: time.Now(), SyncType: models.SyncTypeManual}
	sid, err := svc.syncRepo.CreateSession(sess)
	require.NoError(t, err)
	sess.ID = sid
	return sess
}

// RV5-A: uploadToWebDAV walked the local root and opened every non-directory entry, so a
// symlink planted in the source (to a secret outside the root, or to the database) or a hard
// link of the database was read and PUT to the remote server. Driven through the real upload
// loop against a real WebDAV server (golang.org/x/net/webdav).
func TestSync_WebDAVUploadSkipsLinksPlantedInTheSource(t *testing.T) {
	base, src, _, db, secret := perFileFixture(t)
	allowSyncDirs(t, base) // the upload source is confined (review round 8)
	require.NoError(t, os.Symlink(secret, filepath.Join(src, "leak")))
	require.NoError(t, os.Symlink(db, filepath.Join(src, "dbl")))
	require.NoError(t, os.Link(db, filepath.Join(src, "hl")))

	davRoot := t.TempDir()
	srv := httptest.NewServer(&webdav.Handler{FileSystem: webdav.Dir(davRoot), LockSystem: webdav.NewMemLS()})
	defer srv.Close()

	svc, _ := newSyncSvc(t)
	ep := &models.SyncEndpoint{ID: 1, UserID: 1, Name: "dav", Type: models.SyncTypeWebDAV, URL: srv.URL,
		SyncDirection: models.SyncDirectionUpload, LocalPath: src, RemotePath: "/r"}
	sess := newRV5Session(t, svc, 1)
	require.NoError(t, svc.uploadToWebDAV(sess, ep, NewWebDAVClient(srv.URL, "", "")))

	got, err := os.ReadFile(filepath.Join(davRoot, "r", "a.txt"))
	require.NoError(t, err, "control: the regular file must be uploaded")
	assert.Equal(t, "a", string(got))
	for _, n := range []string{"leak", "dbl", "hl"} {
		_, lerr := os.Lstat(filepath.Join(davRoot, "r", n))
		assert.True(t, os.IsNotExist(lerr), "%s must not be uploaded (lstat err=%v)", n, lerr)
	}
	assert.Equal(t, 1, sess.SyncedFiles, "only a.txt is synced")
}

// RV5-B (WebDAV download): a listing entry whose path equals the remote root maps to the local
// root itself. ConfineData accepted it, then MkdirAll(Dir(local root)) touched the root's PARENT
// and the download was requested and written onto the root path.
func TestSync_WebDAVDownloadRefusesEntryNamingTheLocalRoot(t *testing.T) {
	allowed := t.TempDir()
	t.Setenv(pathGuardEnvSync, allowed)
	t.Setenv(pathGuardEnvRead, allowed)
	local := filepath.Join(allowed, "local")
	require.NoError(t, os.MkdirAll(local, 0o755))

	future := time.Now().Add(24 * time.Hour).UTC().Format(http.TimeFormat)
	listing := `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:">` +
		`<D:response><D:href>/remote/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>` +
		`<D:response><D:href>/remote/%2e</D:href><D:propstat><D:prop><D:resourcetype></D:resourcetype><D:getlastmodified>` + future +
		`</D:getlastmodified><D:getcontentlength>5</D:getcontentlength></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>` +
		`</D:multistatus>`
	var gets atomic.Int32
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" {
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			fmt.Fprint(w, listing)
			return
		}
		gets.Add(1)
		fmt.Fprint(w, "PWNED")
	}))
	defer evil.Close()

	// precondition (§11.4.199): the listing really yields an entry equal to the remote root
	files, err := NewWebDAVClient(evil.URL, "", "").ListFiles("/remote")
	require.NoError(t, err)
	reached := false
	for _, f := range files {
		if filepath.Clean(f.Path) == "/remote" && !f.IsDir {
			reached = true
		}
	}
	require.True(t, reached, "precondition not reached: listing paths %v", paths(files))

	svc, _ := newSyncSvc(t)
	ep := &models.SyncEndpoint{ID: 2, UserID: 1, Name: "evil", Type: models.SyncTypeWebDAV, URL: evil.URL,
		SyncDirection: models.SyncDirectionDownload, LocalPath: local, RemotePath: "/remote"}
	sess := newRV5Session(t, svc, 2)
	before := dirSnapshot(t, filepath.Dir(allowed))
	_ = svc.downloadFromWebDAV(sess, ep, NewWebDAVClient(evil.URL, "", ""))
	assert.EqualValues(t, 0, gets.Load(), "an entry naming the local root itself must be refused before any download request")
	assert.GreaterOrEqual(t, sess.FailedFiles, 1, "the entry is counted as failed")
	assert.Equal(t, before, dirSnapshot(t, filepath.Dir(allowed)), "nothing may change at or above the local root")
}

// RV5-B (conversion): a target (or source) equal to the conversion root was accepted; the
// LibreOffice output and the multi-page writer then wrote into Dir(target), the root's parent.
func TestConversion_TargetOrSourceEqualToTheRootIsRefused(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "conv")
	require.NoError(t, os.MkdirAll(root, 0o755))
	src := filepath.Join(root, "in.pdf")
	require.NoError(t, os.WriteFile(src, []byte("%PDF"), 0o644))
	t.Setenv(pathGuardEnvConv, root)
	t.Setenv(pathGuardEnvRead, root)

	for _, tgt := range []string{root, root + "/", root + "/.", root + "/sub/.."} {
		err := confineConversionPaths(src, tgt)
		assert.Error(t, err, "target %q is the root itself; LibreOffice would write %q (outside the root)", tgt, libreOfficeHTMLOutput(src, tgt))
	}
	for _, s := range []string{root, root + "/."} {
		assert.Error(t, confineConversionPaths(s, filepath.Join(root, "out.html")), "source %q is the root itself", s)
	}
	// positive control: a real file pair below the root
	assert.NoError(t, confineConversionPaths(src, filepath.Join(root, "out.html")))
	assert.NoError(t, confineConversionPaths(src, filepath.Join(root, "sub", "out.html")))
}

// T-1 (V10 killer): copyFile must refuse a destination that is a hard link of the database.
// Opening it with O_TRUNC would truncate the database itself.
func TestSync_CopyFileRefusesADestinationHardLinkToTheDatabase(t *testing.T) {
	_, src, dst, db, _ := perFileFixture(t)
	require.NoError(t, os.Link(db, filepath.Join(dst, "a.txt")))
	before, err := os.ReadFile(db)
	require.NoError(t, err)

	err = (&SyncService{}).copyFile(filepath.Join(src, "a.txt"), filepath.Join(dst, "a.txt"), 0o644, src, dst)
	assert.ErrorIs(t, err, pathguard.ErrProtected)
	after, rerr := os.ReadFile(db)
	require.NoError(t, rerr)
	assert.Equal(t, before, after, "the database must not be truncated or overwritten")

	// positive control: a plain destination is written
	require.NoError(t, (&SyncService{}).copyFile(filepath.Join(src, "a.txt"), filepath.Join(dst, "b.txt"), 0o644, src, dst))
	b, _ := os.ReadFile(filepath.Join(dst, "b.txt"))
	assert.Equal(t, "a", string(b))
}

// T-2 (V9 killer): a symlink planted in the DESTINATION pointing back into the source root must
// not redirect the mirror write into the source.
func TestSync_MirrorRefusesADestinationLinkIntoTheSource(t *testing.T) {
	_, src, dst, _, _ := perFileFixture(t)
	victim := filepath.Join(src, "victim.txt")
	require.NoError(t, os.WriteFile(victim, []byte("keep"), 0o644))
	require.NoError(t, os.Symlink(victim, filepath.Join(dst, "a.txt")))

	_ = (&SyncService{}).performMirrorSync(src, dst, &models.SyncSession{})
	got, err := os.ReadFile(victim)
	require.NoError(t, err)
	assert.Equal(t, "keep", string(got), "the source file behind the destination link must not be overwritten")
}

// T-2 (V5 killer): the protected key file is itself a symlink to the real key; a hard link of
// the REAL key planted in the source must still be refused.
func TestSync_UploadRefusesAHardLinkOfASymlinkedKeyFile(t *testing.T) {
	_, src, _, _, _ := perFileFixture(t)
	keyDir := t.TempDir()
	realKey := filepath.Join(keyDir, "real.key")
	require.NoError(t, os.WriteFile(realKey, []byte("KEY"), 0o600))
	keyLink := filepath.Join(keyDir, "catalogizer.key")
	require.NoError(t, os.Symlink(realKey, keyLink))
	prev := pathguard.SetProtected(append(pathguard.ProtectedPaths(), keyLink))
	t.Cleanup(func() { pathguard.SetProtected(prev) })

	hl := filepath.Join(src, "hk")
	require.NoError(t, os.Link(realKey, hl))
	info, err := os.Lstat(hl)
	require.NoError(t, err)
	require.True(t, info.Mode().IsRegular(), "precondition: a hard link is a regular file")
	assert.False(t, uploadableFile(hl, info, src), "a hard link of the real key behind a protected symlink must be refused")
	a, _ := os.Lstat(filepath.Join(src, "a.txt"))
	assert.True(t, uploadableFile(filepath.Join(src, "a.txt"), a, src), "control: a plain file is uploadable")
}

// Positive control adopted from the round-5 reviewer: a normal local mirror and incremental sync
// between two sub-directories of the data dir (which holds the database) still work, also when
// the data root is reached through a symlink alias.
func TestSync_LegitMirrorInsideTheDataDir(t *testing.T) {
	for _, alias := range []bool{false, true} {
		t.Run(fmt.Sprintf("alias=%v", alias), func(t *testing.T) {
			base := t.TempDir()
			real := filepath.Join(base, "vol")
			data := real
			require.NoError(t, os.MkdirAll(filepath.Join(real, "src", "sub", "deep"), 0o755))
			if alias {
				data = filepath.Join(base, "data")
				require.NoError(t, os.Symlink(real, data))
			}
			db := filepath.Join(data, "catalogizer.db")
			require.NoError(t, os.WriteFile(db, []byte("SQLite format 3\x00"), 0o600))
			protectDatabaseForTest(t, db)
			t.Setenv(pathGuardEnvSync, data)
			t.Setenv(pathGuardEnvRead, data)
			names := []string{"a.txt", "sub/b.txt", "sub/deep/c.txt", ".hidden"}
			for _, f := range names {
				require.NoError(t, os.WriteFile(filepath.Join(data, "src", f), []byte(f), 0o644))
			}
			src, dst := filepath.Join(data, "src"), filepath.Join(data, "dst")
			settings := `{"source_directory":"` + src + `","destination_directory":"` + dst + `","sync_mode":"mirror"}`
			ep := &models.SyncEndpoint{Type: models.SyncTypeLocal, LocalPath: src, SyncDirection: models.SyncDirectionUpload, SyncSettings: &settings}
			s := &SyncService{}
			require.NoError(t, s.confineSyncPaths(ep), "legit endpoint refused")
			require.NoError(t, s.performLocalSync(&models.SyncSession{}, ep), "legit mirror failed")
			for _, f := range names {
				b, err := os.ReadFile(filepath.Join(dst, f))
				require.NoError(t, err, "%s not mirrored", f)
				assert.Equal(t, f, string(b))
			}
			future := time.Now().Add(time.Hour)
			require.NoError(t, os.WriteFile(filepath.Join(src, "a.txt"), []byte("A2"), 0o644))
			require.NoError(t, os.Chtimes(filepath.Join(src, "a.txt"), future, future))
			require.NoError(t, s.performIncrementalSync(src, dst, &models.SyncSession{}))
			b, _ := os.ReadFile(filepath.Join(dst, "a.txt"))
			assert.Equal(t, "A2", string(b), "incremental did not update")
		})
	}
}
