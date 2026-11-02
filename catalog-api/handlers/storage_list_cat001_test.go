package handlers

// CAT-001: GET /storage/list/*path answered 200 with an always-empty file list ("Return mock data for now"). It must list the real
// directory of the real storage root through the real client (local protocol here: real temp directory, real SQLite, real factory).

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCAT001_ListStoragePath_ListsTheRealDirectory(t *testing.T) {
	base := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(base, "movies", "sub"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(base, "movies", "a.mkv"), []byte("12345"), 0o644))
	h := statusHarness(t)
	h.r.GET("/storage/list/*path", h.handle.ListStoragePath)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "lst", "protocol": "local", "path": base})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	id := strconv.FormatInt(int64(body["id"].(float64)), 10)

	code, out := h.do("GET", "/storage/list/movies?storage_id="+id, nil)
	require.Equal(t, http.StatusOK, code, "%v", out)
	files, _ := out["files"].([]interface{})
	names := map[string]bool{}
	for _, f := range files {
		m := f.(map[string]interface{})
		names[m["name"].(string)] = m["is_dir"].(bool)
		if m["name"] == "a.mkv" {
			assert.EqualValues(t, 5, m["size"])
		}
	}
	assert.Equal(t, map[string]bool{"a.mkv": false, "sub": true}, names)
}

func TestCAT001_ListStoragePath_ErrorsAreHonest(t *testing.T) {
	h := statusHarness(t)
	h.r.GET("/storage/list/*path", h.handle.ListStoragePath)
	code, _ := h.do("GET", "/storage/list/x", nil)
	assert.Equal(t, http.StatusBadRequest, code, "storage_id required")
	code, _ = h.do("GET", "/storage/list/x?storage_id=9999", nil)
	assert.Equal(t, http.StatusNotFound, code, "unknown root")
	code, _ = h.do("GET", "/storage/list/x?storage_id=abc", nil)
	assert.Equal(t, http.StatusBadRequest, code, "non-numeric id")

	base := t.TempDir()
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "lst2", "protocol": "local", "path": base})
	require.Equal(t, http.StatusCreated, code)
	id := strconv.FormatInt(int64(body["id"].(float64)), 10)
	code, out := h.do("GET", "/storage/list/does-not-exist?storage_id="+id, nil)
	assert.Equal(t, http.StatusNotFound, code, "a missing directory is an error, not an empty listing: %v", out)
}
