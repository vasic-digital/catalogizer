package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

// §11.4.115 RED/GREEN: an authenticated user MUST NOT be able to point a local sync
// endpoint at a directory outside the configured sync roots (arbitrary read/write/delete
// via mirror-mode cleanupDestination). Drives the ROUTED POST /sync/endpoints + /sync.
func (s *SyncHandlerTestSuite) TestPathConfine_LocalSyncCannotTouchOutsideRoots() {
	allowed := s.T().TempDir()
	s.T().Setenv("CATALOGIZER_SYNC_ROOTS", allowed)

	src := filepath.Join(allowed, "src")
	s.Require().NoError(os.MkdirAll(src, 0o755))
	victim := s.T().TempDir() // outside the allowed root
	precious := filepath.Join(victim, "precious.txt")
	s.Require().NoError(os.WriteFile(precious, []byte("keep"), 0o600))

	body := map[string]interface{}{
		"name": "evil", "type": "local", "url": "file:///x", "sync_direction": "upload",
		"local_path":    src,
		"remote_path":   src,
		"sync_settings": `{"destination_directory":"` + victim + `","sync_mode":"mirror"}`,
	}
	w := s.doRequest("POST", "/sync/endpoints", body, true)
	if w.Code == http.StatusCreated {
		// legacy-accepted endpoint: execution MUST still refuse; run it and wait.
		resp := s.parseResponse(w)
		id := int(resp["data"].(map[string]interface{})["id"].(float64))
		s.doRequest("POST", "/sync/endpoints/"+strconv.Itoa(id)+"/sync", nil, true)
		s.syncService.Close()
	}
	_, err := os.Stat(precious)
	s.NoError(err, "file outside the sync roots was deleted by mirror sync")
	s.Equal(http.StatusBadRequest, w.Code, "endpoint with destination outside roots must be rejected")
}

func (s *SyncHandlerTestSuite) TestPathConfine_LocalPathOutsideRootsRejected() {
	allowed := s.T().TempDir()
	s.T().Setenv("CATALOGIZER_SYNC_ROOTS", allowed)
	body := map[string]interface{}{
		"name": "evil2", "type": "local", "url": "file:///x", "sync_direction": "download",
		"local_path": "/etc", "remote_path": allowed,
	}
	w := s.doRequest("POST", "/sync/endpoints", body, true)
	s.Equal(http.StatusBadRequest, w.Code)
}

// F2: the documented use case (docs/api/SYNC_API.md): push the read-only media library
// (MEDIA_ROOT_PATH, mounted :ro) to a remote. local_path is only READ for upload, so a read
// root is enough; the same path for a download (which WRITES local_path) is refused.
func (s *SyncHandlerTestSuite) TestPathConfine_UploadFromMediaRootAccepted_DownloadRefused() {
	media := s.T().TempDir()
	writeRoot := s.T().TempDir()
	s.T().Setenv("CATALOGIZER_SYNC_ROOTS", writeRoot)
	s.T().Setenv("CATALOGIZER_READ_ROOTS", media)
	mk := func(dir string) map[string]interface{} {
		return map[string]interface{}{"name": "lib-" + dir, "type": "webdav", "url": "http://example.com/dav",
			"sync_direction": dir, "local_path": filepath.Join(media, "movies"), "remote_path": "/backup/movies"}
	}
	// 502 = path validation passed, the (deliberately unreachable) WebDAV connection test failed
	s.NotEqual(http.StatusBadRequest, s.doRequest("POST", "/sync/endpoints", mk("upload"), true).Code, "upload from the media root must pass path validation")
	s.Equal(http.StatusBadRequest, s.doRequest("POST", "/sync/endpoints", mk("download"), true).Code)
}
