package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/textproto"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newProductionCopyRouter registers the copy handlers on a real gin
// engine, with the production constructor. Services are nil: every request
// below must be rejected by input validation BEFORE any service is touched,
// and a panic (nil service reached) is turned into 500 by gin.Recovery, which
// the assertions treat as a failure of the guard.
//
// It registers /copy/storage, /copy/local, /copy/upload (the routes wired in
// catalog-api/main.go) plus /copy/smb and /smb/list/*path, which are NOT
// routed in main.go today; they are registered here so the SMB handlers'
// validation stays covered should they be routed.
func newProductionCopyRouter(t *testing.T, root string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewCopyHandler(nil, nil, root, zap.NewNop())
	r := gin.New()
	r.Use(gin.Recovery())
	api := r.Group("/api/v1")
	api.POST("/copy/storage", h.CopyToStorage)
	api.POST("/copy/local", h.CopyToLocal)
	api.POST("/copy/upload", h.CopyFromLocal)
	api.POST("/copy/smb", h.CopyToSMB)
	api.GET("/smb/list/*path", h.ListSMBPath)
	return r
}

func postJSON(r *gin.Engine, url string, body map[string]any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", url, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRoutedCopyToLocal_RejectsEscapingDestinations(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "x")
	require.NoError(t, os.MkdirAll(root, 0o755))
	require.NoError(t, os.MkdirAll(filepath.Join(base, "xy"), 0o755))
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "link")))
	r := newProductionCopyRouter(t, root)

	cases := map[string]string{
		"dotdot":         "../escaped.txt",
		"deep dotdot":    "a/../../escaped.txt",
		"absolute":       filepath.Join(outside, "abs.txt"),
		"sibling prefix": "../xy/f",
		"symlink":        "link/f",
		"nul":            "a\x00b",
	}
	for name, dest := range cases {
		t.Run(name, func(t *testing.T) {
			w := postJSON(r, "/api/v1/copy/local", map[string]any{
				"source_path": "nas:/share/file.bin", "destination_path": dest, "overwrite": true,
			})
			assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
		})
	}
	for _, p := range []string{filepath.Join(outside, "abs.txt"), filepath.Join(outside, "f"), filepath.Join(base, "escaped.txt"), filepath.Join(base, "xy", "f")} {
		_, err := os.Lstat(p)
		assert.True(t, os.IsNotExist(err), "must not exist: %s", p)
	}
}

func TestRoutedCopyToLocal_AllowsDestinationInsideRoot(t *testing.T) {
	root := t.TempDir()
	r := newProductionCopyRouter(t, root)
	w := postJSON(r, "/api/v1/copy/local", map[string]any{
		"source_path": "nas:/share/file.bin", "destination_path": "sub/dir/file.bin",
	})
	// Validation passes; the nil SMB service is then reached (recovered -> 500).
	assert.NotEqual(t, http.StatusBadRequest, w.Code, "legit in-root destination rejected: %s", w.Body.String())
}

func TestRoutedCopyToLocal_RejectsTraversalInRemoteSource(t *testing.T) {
	r := newProductionCopyRouter(t, t.TempDir())
	w := postJSON(r, "/api/v1/copy/local", map[string]any{
		"source_path": `nas:/share/..\..\secret`, "destination_path": "ok.bin",
	})
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}

func TestRoutedCopyToSMB_RejectsTraversalInRemotePaths(t *testing.T) {
	r := newProductionCopyRouter(t, t.TempDir())
	for _, c := range []map[string]any{
		{"source_path": "a:/s/../../x", "destination_path": "b:/d/f"},
		{"source_path": "a:/s/f", "destination_path": `b:\d\..\..\f`},
	} {
		w := postJSON(r, "/api/v1/copy/smb", c)
		assert.Equal(t, http.StatusBadRequest, w.Code, "case=%v body=%s", c, w.Body.String())
	}
}

func upload(r *gin.Engine, filename, destination string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	h.Set("Content-Type", "application/octet-stream")
	pw, _ := mw.CreatePart(h)
	_, _ = pw.Write([]byte("data"))
	_ = mw.WriteField("destination", destination)
	_ = mw.Close()
	req := httptest.NewRequest("POST", "/api/v1/copy/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestRoutedCopyFromLocal_RejectsUnsafeNamesAndDestinations(t *testing.T) {
	r := newProductionCopyRouter(t, t.TempDir())
	for name, c := range map[string][2]string{
		"backslash filename": {`..\..\evil.txt`, "nas:/share/dir"},
		"dotdot destination": {"ok.txt", "nas:/share/../../etc/x"},
		"backslash dest":     {"ok.txt", `nas:\share\..\..\x`},
	} {
		t.Run(name, func(t *testing.T) {
			w := upload(r, c[0], c[1])
			assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
		})
	}
}

func TestRoutedListSMBPath_RejectsTraversal(t *testing.T) {
	r := newProductionCopyRouter(t, t.TempDir())
	req := httptest.NewRequest("GET", "/api/v1/smb/list/share/..%2f..%2fetc?host=nas", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
}
