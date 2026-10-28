package services

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/models"
	"catalogizer/repository"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/webdav"
)

func newSyncSvc(t *testing.T) (*SyncService, *repository.SyncRepository) {
	t.Helper()
	db, cleanup := newSyncTestDB(t)
	t.Cleanup(cleanup)
	repo := repository.NewSyncRepository(db)
	svc := NewSyncService(repo, nil, nil)
	t.Cleanup(svc.Close)
	return svc, repo
}

func settingsJSON(s string) *string { return &s }

// F3/M1: an endpoint stored BEFORE the confinement existed (or edited in the DB) must be
// refused again at run time. Mirror sync would otherwise delete files in the destination.
func TestPathConfine_RunTimeRecheckRefusesLegacyEndpoint(t *testing.T) {
	allowed := t.TempDir()
	t.Setenv(pathGuardEnvSync, allowed)
	t.Setenv(pathGuardEnvRead, allowed)
	src := filepath.Join(allowed, "src")
	require.NoError(t, os.MkdirAll(src, 0o755))
	victim := t.TempDir()
	precious := filepath.Join(victim, "precious.txt")
	require.NoError(t, os.WriteFile(precious, []byte("keep"), 0o600))

	svc, repo := newSyncSvc(t)
	id, err := repo.CreateEndpoint(&models.SyncEndpoint{
		UserID: 1, Name: "legacy", Type: models.SyncTypeLocal, URL: "file:///x",
		SyncDirection: models.SyncDirectionDownload, LocalPath: src, RemotePath: src,
		SyncSettings: settingsJSON(`{"source_directory":"` + src + `","destination_directory":"` + victim + `","sync_mode":"mirror"}`),
		Status:       models.SyncStatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	require.NoError(t, err)

	sess, err := svc.StartSync(id, 1)
	require.NoError(t, err)
	svc.wg.Wait()

	_, serr := os.Stat(precious)
	assert.NoError(t, serr, "mirror sync deleted a file outside the sync roots")
	got, err := repo.GetSession(sess.ID)
	require.NoError(t, err)
	assert.NotEqual(t, models.SyncSessionStatusCompleted, got.Status, "legacy out-of-roots endpoint must not complete")
}

// F3/M2 (UNIT-CLASS, §11.4.289: a fixture PROPFIND server serving a hostile listing; the
// positive control below uses a REAL golang.org/x/net/webdav server on a temp dir).
// A remote listing entry that climbs out of the local root must be refused, not written.
func TestPathConfine_WebDAVListingEscapeRefused(t *testing.T) {
	allowed := t.TempDir()
	t.Setenv(pathGuardEnvSync, allowed)
	t.Setenv(pathGuardEnvRead, allowed)

	// --- positive control: REAL WebDAV server, legit file downloads under the local root.
	davRoot := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(davRoot, "remote"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(davRoot, "remote", "ok.txt"), []byte("real"), 0o644))
	real := httptest.NewServer(&webdav.Handler{FileSystem: webdav.Dir(davRoot), LockSystem: webdav.NewMemLS()})
	defer real.Close()
	local := filepath.Join(allowed, "local")
	require.NoError(t, os.MkdirAll(local, 0o755))
	svc, repo := newSyncSvc(t)
	ep := &models.SyncEndpoint{ID: 1, UserID: 1, Name: "dav", Type: models.SyncTypeWebDAV, URL: real.URL,
		SyncDirection: models.SyncDirectionDownload, LocalPath: local, RemotePath: "/remote"}
	sess := &models.SyncSession{EndpointID: 1, UserID: 1, Status: models.SyncSessionStatusRunning, StartedAt: time.Now(), SyncType: models.SyncTypeManual}
	sid, err := repo.CreateSession(sess)
	require.NoError(t, err)
	sess.ID = sid
	require.NoError(t, svc.downloadFromWebDAV(sess, ep, NewWebDAVClient(real.URL, "", "")))
	b, rerr := os.ReadFile(filepath.Join(local, "ok.txt"))
	require.NoError(t, rerr, "control: real WebDAV download must land under the local root")
	assert.Equal(t, "real", string(b))

	// --- hostile listing: a FILE entry named ".." (resolves to the parent of the remote dir).
	var gets atomic.Int32
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "PROPFIND":
			w.Header().Set("Content-Type", "application/xml")
			w.WriteHeader(http.StatusMultiStatus)
			fmt.Fprint(w, hostileListing(false))
		default:
			gets.Add(1)
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, "PWNED")
		}
	}))
	defer evil.Close()
	files, err := NewWebDAVClient(evil.URL, "", "").ListFiles("/remote")
	require.NoError(t, err)
	t.Logf("hostile listing produced paths: %v", paths(files))
	ep2 := &models.SyncEndpoint{ID: 2, UserID: 1, Name: "evil", Type: models.SyncTypeWebDAV, URL: evil.URL,
		SyncDirection: models.SyncDirectionDownload, LocalPath: local, RemotePath: "/remote"}
	sess2 := &models.SyncSession{EndpointID: 2, UserID: 1, Status: models.SyncSessionStatusRunning, StartedAt: time.Now(), SyncType: models.SyncTypeManual}
	sid2, err := repo.CreateSession(sess2)
	require.NoError(t, err)
	sess2.ID = sid2
	before := dirSnapshot(t, allowed)
	_ = svc.downloadFromWebDAV(sess2, ep2, NewWebDAVClient(evil.URL, "", ""))
	assert.EqualValues(t, 0, gets.Load(), "a listing entry escaping the local root must be refused before any download request")
	assert.GreaterOrEqual(t, sess2.FailedFiles, 1, "escaping entry must be counted as failed")
	assert.Equal(t, before, dirSnapshot(t, allowed), "hostile listing wrote outside the local root")
}

func hostileListing(dotdotIsCollection bool) string {
	coll := ""
	if dotdotIsCollection {
		coll = "<D:collection/>"
	}
	return `<?xml version="1.0"?><D:multistatus xmlns:D="DAV:">` +
		`<D:response><D:href>/remote/</D:href><D:propstat><D:prop><D:resourcetype><D:collection/></D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>` +
		`<D:response><D:href>/remote/%2e%2e</D:href><D:propstat><D:prop><D:resourcetype>` + coll + `</D:resourcetype></D:prop><D:status>HTTP/1.1 200 OK</D:status></D:propstat></D:response>` +
		`</D:multistatus>`
}

// A hostile server listing a directory named ".." (or a self-referential tree) made
// ListFiles recurse until the goroutine stack overflowed and killed the whole process
// (found while writing the M2 test; unfixed code: "fatal error: stack overflow").
func TestWebDAVListFiles_HostileDirectoryLoopTerminates(t *testing.T) {
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusMultiStatus)
		fmt.Fprint(w, hostileListing(true))
	}))
	defer evil.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = NewWebDAVClient(evil.URL, "", "").ListFiles("/remote")
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("ListFiles did not terminate on a self-referential hostile listing")
	}
}

func paths(fs []*WebDAVFile) []string {
	var o []string
	for _, f := range fs {
		o = append(o, f.Path)
	}
	return o
}

func dirSnapshot(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	_ = filepath.Walk(root, func(p string, i os.FileInfo, err error) error {
		if err == nil {
			out = append(out, fmt.Sprintf("%s|%d", p, i.Size()))
		}
		return nil
	})
	return out
}

// F2: READ roots (media library) are separate from WRITE roots.
func TestPathConfine_ReadVersusWriteRoots(t *testing.T) {
	media := t.TempDir()     // read-only library, like MEDIA_ROOT_PATH mounted :ro
	writeRoot := t.TempDir() // sync write root
	other := t.TempDir()
	t.Setenv(pathGuardEnvRead, media)
	t.Setenv(pathGuardEnvSync, writeRoot)
	svc, _ := newSyncSvc(t)

	mk := func(typ, dir string, local string, settings string) *models.SyncEndpoint {
		e := &models.SyncEndpoint{Name: "n", Type: typ, URL: "http://x", SyncDirection: dir, LocalPath: local, RemotePath: "/r"}
		if typ == models.SyncTypeLocal {
			e.RemotePath = media
		}
		if settings != "" {
			e.SyncSettings = &settings
		}
		return e
	}
	// documented use case: webdav push (upload) of /media/movies — local_path is only READ.
	assert.NoError(t, svc.validateSyncEndpoint(mk(models.SyncTypeWebDAV, models.SyncDirectionUpload, filepath.Join(media, "movies"), "")), "upload from the media root must be accepted")
	assert.Error(t, svc.validateSyncEndpoint(mk(models.SyncTypeWebDAV, models.SyncDirectionDownload, filepath.Join(media, "movies"), "")), "download WRITES local_path: media root is not a write root")
	assert.Error(t, svc.validateSyncEndpoint(mk(models.SyncTypeWebDAV, models.SyncDirectionBidirectional, filepath.Join(media, "movies"), "")), "bidirectional writes local_path")
	assert.NoError(t, svc.validateSyncEndpoint(mk(models.SyncTypeWebDAV, models.SyncDirectionDownload, filepath.Join(writeRoot, "d"), "")))
	assert.Error(t, svc.validateSyncEndpoint(mk(models.SyncTypeWebDAV, models.SyncDirectionUpload, other, "")), "outside every root")

	// local mirror: source may be the media root, destination must be a write root
	okSettings := `{"source_directory":"` + media + `","destination_directory":"` + filepath.Join(writeRoot, "d") + `","sync_mode":"mirror"}`
	assert.NoError(t, svc.validateSyncEndpoint(mk(models.SyncTypeLocal, models.SyncDirectionUpload, filepath.Join(writeRoot, "x"), okSettings)))
	badDest := `{"source_directory":"` + media + `","destination_directory":"` + media + `","sync_mode":"mirror"}`
	assert.Error(t, svc.validateSyncEndpoint(mk(models.SyncTypeLocal, models.SyncDirectionUpload, filepath.Join(writeRoot, "x"), badDest)), "destination in a read-only root")
	bidi := `{"source_directory":"` + media + `","destination_directory":"` + filepath.Join(writeRoot, "d") + `","sync_mode":"bidirectional"}`
	assert.Error(t, svc.validateSyncEndpoint(mk(models.SyncTypeLocal, models.SyncDirectionUpload, filepath.Join(writeRoot, "x"), bidi)), "bidirectional mode writes the source side")

	// F6: sync_settings directories are checked for EVERY endpoint type, not only local.
	for _, typ := range []string{models.SyncTypeWebDAV, models.SyncTypeCloudStorage} {
		evil := `{"source_directory":"` + other + `"}`
		assert.Error(t, svc.validateSyncEndpoint(mk(typ, models.SyncDirectionUpload, filepath.Join(media, "m"), evil)), "source_directory outside roots, type "+typ)
		evilDest := `{"destination_directory":"` + other + `"}`
		assert.Error(t, svc.validateSyncEndpoint(mk(typ, models.SyncDirectionUpload, filepath.Join(media, "m"), evilDest)), "destination_directory outside roots, type "+typ)
	}
}

// F2/F5: conversion — inputs may come from the read roots, outputs only to write roots;
// stored pending jobs are re-checked when started.
func TestPathConfine_ConversionReadWriteAndStartRecheck(t *testing.T) {
	media := t.TempDir()
	out := t.TempDir()
	t.Setenv(pathGuardEnvRead, media)
	t.Setenv(pathGuardEnvConv, out)
	db := setupTestDB(t)
	repo := repository.NewConversionRepository(db)
	svc := NewConversionService(repo, nil, nil)
	mk := func(src, dst string) *models.ConversionRequest {
		return &models.ConversionRequest{SourcePath: src, TargetPath: dst, SourceFormat: "png", TargetFormat: "jpg", ConversionType: "image"}
	}
	_, err := svc.CreateConversionJob(1, mk(filepath.Join(media, "in.png"), filepath.Join(out, "o.jpg")))
	assert.NoError(t, err, "documented use: read from media root, write to the conversion root")
	_, err = svc.CreateConversionJob(1, mk(filepath.Join(media, "in.png"), filepath.Join(media, "o.jpg")))
	assert.Error(t, err, "output into the read-only media root must be refused")

	// legacy pending job stored before the upgrade
	job := &models.ConversionJob{UserID: 1, SourcePath: "/etc/passwd", TargetPath: filepath.Join(out, "x.jpg"),
		SourceFormat: "png", TargetFormat: "jpg", ConversionType: models.ConversionTypeImage,
		Status: models.ConversionStatusPending, CreatedAt: time.Now()}
	id, err := repo.CreateJob(job)
	require.NoError(t, err)
	assert.Error(t, svc.StartConversion(id), "stored job with a source outside the roots must not start")
	got, err := repo.GetJob(id)
	require.NoError(t, err)
	assert.Equal(t, models.ConversionStatusFailed, got.Status)
}
