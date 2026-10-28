package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- containedJoin: local destination containment -------------------------

func TestContainedJoin_RejectsTraversal(t *testing.T) {
	root := t.TempDir()
	bad := []string{
		"../escape",
		"a/../../escape",
		"a/b/../../../escape",
		"/etc/passwd",
		root + "/../escape",
		"a\x00b",
		"..",
	}
	for _, p := range bad {
		got, err := containedJoin(root, p)
		assert.Error(t, err, "path %q must be rejected (got %q)", p, got)
	}
}

func TestContainedJoin_AcceptsContained(t *testing.T) {
	root := t.TempDir()
	for _, p := range []string{"a.txt", "sub/dir/a.txt", root + "/x/y", "./a/./b"} {
		got, err := containedJoin(root, p)
		require.NoError(t, err, p)
		assert.True(t, got == root || strings.HasPrefix(got, root+string(os.PathSeparator)), "%q escaped root: %q", p, got)
	}
}

func TestContainedJoin_RejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "link")))

	_, err := containedJoin(root, "link/file.txt")
	assert.Error(t, err, "path through symlink pointing outside root must be rejected")
	_, err = containedJoin(root, "link/new/dir/file.txt")
	assert.Error(t, err, "non-existent tail below an escaping symlink must be rejected")
}

func TestContainedJoin_EmptyRootRejected(t *testing.T) {
	_, err := containedJoin("", "a.txt")
	assert.Error(t, err)
}

// --- safeChildName: single path element from listings / uploads ------------

func TestSafeChildName(t *testing.T) {
	for _, n := range []string{"", ".", "..", "../x", "a/b", `a\b`, `..\x`, "x\x00y", "/abs"} {
		_, err := safeChildName(n)
		assert.Error(t, err, "name %q must be rejected", n)
	}
	for _, n := range []string{"movie.mkv", "a b.txt", "..hidden", "ü.mp3"} {
		got, err := safeChildName(n)
		assert.NoError(t, err, n)
		assert.Equal(t, n, got)
	}
}

func TestSafeRemotePath(t *testing.T) {
	for _, p := range []string{"../x", "a/../../x", `a\..\x`, "a\x00b", "/a/b/../../.."} {
		assert.Error(t, safeRemotePath(p), "remote path %q must be rejected", p)
	}
	for _, p := range []string{"/media/movies", "a/b", "a/..b/c", "/"} {
		assert.NoError(t, safeRemotePath(p), p)
	}
}

// --- handler level: traversal rejected before any repository/SMB access ----

func newSecurityRouter(tempDir string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewCopyHandler(nil, tempDir) // nil repo: a request that reaches the repo would panic -> 500 via Recovery
	r := gin.New()
	r.Use(gin.Recovery())
	r.POST("/api/copy/smb", h.CopyToSmb)
	r.POST("/api/copy/local", h.CopyToLocal)
	r.POST("/api/copy/upload", h.CopyFromLocal)
	return r
}

func TestCopyToLocal_RejectsTraversalDestination(t *testing.T) {
	r := newSecurityRouter(t.TempDir())
	for _, dest := range []string{"../../etc/cron.d/x", "/etc/passwd", "a/../../../x"} {
		body := `{"source_file_id":1,"destination_path":"` + dest + `"}`
		req := httptest.NewRequest("POST", "/api/copy/local", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equal(t, http.StatusBadRequest, w.Code, "dest %q body=%s", dest, w.Body.String())
	}
}

func TestCopyToSmb_RejectsTraversalDestination(t *testing.T) {
	r := newSecurityRouter(t.TempDir())
	body := `{"source_file_id":1,"destination_smb_root":"r","destination_path":"../../other"}`
	req := httptest.NewRequest("POST", "/api/copy/smb", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func multipartUpload(t *testing.T, destRoot, destPath, filename string) *http.Request {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	require.NoError(t, mw.WriteField("destination_smb_root", destRoot))
	require.NoError(t, mw.WriteField("destination_path", destPath))
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	h.Set("Content-Type", "application/octet-stream")
	part, err := mw.CreatePart(h)
	require.NoError(t, err)
	_, _ = part.Write([]byte("data"))
	require.NoError(t, mw.Close())
	req := httptest.NewRequest("POST", "/api/copy/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestCopyFromLocal_RejectsTraversalDestinationPath(t *testing.T) {
	r := newSecurityRouter(t.TempDir())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, multipartUpload(t, "r", "../../etc", "ok.txt"))
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

func TestCopyFromLocal_RejectsBackslashTraversalFilename(t *testing.T) {
	// Go's multipart parser strips '/' directories (filepath.Base) on Linux but
	// keeps backslashes, which an SMB server treats as separators.
	r := newSecurityRouter(t.TempDir())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, multipartUpload(t, "r", "/media", `..\..\evil.txt`))
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}
