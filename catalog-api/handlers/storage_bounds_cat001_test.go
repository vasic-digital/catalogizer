package handlers

// CAT-001 review F2: /storage/list must bound the listing (WithListLimit) and every Disconnect must be bounded by a deadline.
// The fake client/factory below are unit-class doubles that observe WHAT the handler hands the client; the real-directory
// behaviour is covered by the local-protocol tests.

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"catalogizer/filesystem"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type observedClient struct {
	filesystem.FileSystemClient
	mu            sync.Mutex
	listLimit     int
	listLimited   bool
	disconnectHas bool // Disconnect ctx carried a deadline
	disconnected  chan struct{}
}

func (o *observedClient) Connect(ctx context.Context) error        { return nil }
func (o *observedClient) TestConnection(ctx context.Context) error { return nil }
func (o *observedClient) ListDirectory(ctx context.Context, p string) ([]*filesystem.FileInfo, error) {
	o.mu.Lock()
	o.listLimit, o.listLimited = filesystem.ListLimit(ctx)
	o.mu.Unlock()
	var out []*filesystem.FileInfo
	for i := 0; i < 50; i++ {
		out = append(out, &filesystem.FileInfo{Name: "f" + strconv.Itoa(i), Path: "/f" + strconv.Itoa(i)})
	}
	return out, nil
}
func (o *observedClient) Disconnect(ctx context.Context) error {
	_, has := ctx.Deadline()
	o.mu.Lock()
	o.disconnectHas = has
	o.mu.Unlock()
	close(o.disconnected)
	return nil
}

type observedFactory struct{ c *observedClient }

func (f observedFactory) CreateClient(*filesystem.StorageConfig) (filesystem.FileSystemClient, error) {
	return f.c, nil
}
func (f observedFactory) SupportedProtocols() []string { return []string{"local"} }

func TestCAT001_F2_ListIsBoundedAndTruncationIsReported(t *testing.T) {
	base := t.TempDir()
	h := statusHarness(t)
	t.Setenv(filesystem.AllowedLocalBasesEnv, base)
	h.r.GET("/storage/list/*path", h.handle.ListStoragePath)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "b", "protocol": "local", "path": base})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	oc := &observedClient{disconnected: make(chan struct{})}
	h.handle.clientFactory = observedFactory{oc}
	h.handle.listMaxEntries = 10

	code, out := h.do("GET", "/storage/list/?storage_id="+idOf(body), nil)
	require.Equal(t, http.StatusOK, code, "%v", out)
	oc.mu.Lock()
	assert.True(t, oc.listLimited, "the handler must hand the client a list limit")
	assert.Equal(t, 11, oc.listLimit, "limit+1 so that truncation stays detectable")
	oc.mu.Unlock()
	files, _ := out["files"].([]interface{})
	assert.Len(t, files, 10)
	assert.Equal(t, true, out["truncated"])
	<-oc.disconnected
	oc.mu.Lock()
	assert.True(t, oc.disconnectHas, "Disconnect must run under a deadline")
	oc.mu.Unlock()
}

func TestCAT001_F2_StatusDisconnectIsBounded(t *testing.T) {
	base := t.TempDir()
	h := statusHarness(t)
	t.Setenv(filesystem.AllowedLocalBasesEnv, base)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "s", "protocol": "local", "path": base})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	oc := &observedClient{disconnected: make(chan struct{})}
	h.handle.clientFactory = observedFactory{oc}
	id, _ := strconv.ParseInt(idOf(body), 10, 64)
	code, out := h.do("GET", statusURL(id), nil)
	require.Equal(t, http.StatusOK, code, "%v", out)
	select {
	case <-oc.disconnected:
	case <-time.After(5 * time.Second):
		t.Fatal("Disconnect never ran")
	}
	oc.mu.Lock()
	assert.True(t, oc.disconnectHas, "Disconnect must run under a deadline")
	oc.mu.Unlock()
}

// Real local client: the limit is honoured (filesystem layer).
func TestCAT001_F2_LocalClientHonoursListLimit(t *testing.T) {
	base := t.TempDir()
	for i := 0; i < 20; i++ {
		require.NoError(t, os.WriteFile(filepath.Join(base, "f"+strconv.Itoa(i)), nil, 0o644))
	}
	c := filesystem.NewLocalClient(&filesystem.LocalConfig{BasePath: base})
	require.NoError(t, c.Connect(context.Background()))
	got, err := c.ListDirectory(filesystem.WithListLimit(context.Background(), 5), "/")
	require.NoError(t, err)
	assert.Len(t, got, 5)
}

// CAT-001 review F11 (M1): a disabled root is never listed.
func TestCAT001_F11_DisabledRootListIs409(t *testing.T) {
	base := t.TempDir()
	h := statusHarness(t)
	t.Setenv(filesystem.AllowedLocalBasesEnv, base)
	h.r.GET("/storage/list/*path", h.handle.ListStoragePath)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "d", "protocol": "local", "path": base})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	_, err := h.db.Exec(`UPDATE storage_roots SET enabled = 0 WHERE name = 'd'`)
	require.NoError(t, err)
	code, out := h.do("GET", "/storage/list/?storage_id="+idOf(body), nil)
	assert.Equal(t, http.StatusConflict, code, "%v", out)
}

// CAT-001 review F11 (M13): List must disconnect the client it connected (bounded wait, no hang on a mutant).
func TestCAT001_F11_ListDisconnects(t *testing.T) {
	base := t.TempDir()
	h := statusHarness(t)
	t.Setenv(filesystem.AllowedLocalBasesEnv, base)
	h.r.GET("/storage/list/*path", h.handle.ListStoragePath)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "l", "protocol": "local", "path": base})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	oc := &observedClient{disconnected: make(chan struct{})}
	h.handle.clientFactory = observedFactory{oc}
	code, out := h.do("GET", "/storage/list/?storage_id="+idOf(body), nil)
	require.Equal(t, http.StatusOK, code, "%v", out)
	select {
	case <-oc.disconnected:
	case <-time.After(5 * time.Second):
		t.Fatal("ListStoragePath never disconnected its client")
	}
}
