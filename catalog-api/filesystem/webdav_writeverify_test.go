package filesystem

// UNIT-CLASS tests (constitution 11.4.289(C)): a real net/http handler (an in-memory WebDAV-ish store) stands in for the service to drive the
// defensive read-back of WebDAVClient.WriteFile (WD-F1/SECQ-2, "defensive hardening, not a proven fix"). They prove the client's decisions; they
// are NOT evidence that the real service works (the live evidence is docs/qa/wdf1-repro-20261105/).

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memDAV struct {
	mu         sync.Mutex
	files      map[string][]byte
	puts       int
	heads      int
	dropPuts   int  // first N PUTs are acknowledged (201) but NOT stored
	truncate   int  // first N PUTs store only half of the body (acked)
	noLength   bool // HEAD answers without Content-Length
	headStatus int  // when non-zero HEAD answers this status (a server without HEAD support)
	extra      int  // first N PUTs store MORE bytes than sent (acked)
	headLen    int  // when non-zero HEAD reports this Content-Length regardless of the stored size
	putLens    []string
	putTE      [][]string
}

func (m *memDAV) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	switch r.Method {
	case "PROPFIND":
		w.WriteHeader(http.StatusMultiStatus)
	case "PUT":
		b, _ := io.ReadAll(r.Body)
		m.puts++
		m.putLens = append(m.putLens, r.Header.Get("Content-Length")+"|"+itoa(int(r.ContentLength)))
		m.putTE = append(m.putTE, r.TransferEncoding)
		switch {
		case m.extra > 0:
			m.extra--
			m.files[r.URL.Path] = append(append([]byte{}, b...), 'X', 'X')
		case m.dropPuts > 0:
			m.dropPuts--
		case m.truncate > 0:
			m.truncate--
			m.files[r.URL.Path] = b[:len(b)/2]
		default:
			m.files[r.URL.Path] = b
		}
		w.WriteHeader(http.StatusCreated)
	case "HEAD":
		m.heads++
		if m.headStatus != 0 {
			w.WriteHeader(m.headStatus)
			return
		}
		b, ok := m.files[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if !m.noLength {
			n := len(b)
			if m.headLen != 0 {
				n = m.headLen
			}
			w.Header().Set("Content-Length", itoa(n))
		}
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func newMemDAV(t *testing.T, m *memDAV) *WebDAVClient {
	m.files = map[string][]byte{}
	srv := httptest.NewServer(m)
	t.Cleanup(srv.Close)
	c, _ := NewWebDAVClient(&WebDAVConfig{URL: srv.URL, Username: "u", Password: "p"})
	require.NoError(t, c.Connect(context.Background()))
	return c
}

func TestWriteFile_Verify_AckedButDropped_IsRetriedAndStored(t *testing.T) {
	m := &memDAV{dropPuts: 1}
	c := newMemDAV(t, m)
	data := detBytes("v1", 5000)
	require.NoError(t, c.WriteFile(context.Background(), "/a.bin", bytes.NewReader(data)))
	assert.Equal(t, data, m.files["/a.bin"], "an acknowledged write must be present after WriteFile returns nil")
	assert.Equal(t, 2, m.puts, "exactly one bounded retry")
}

func TestWriteFile_Verify_AckedButNeverStored_ReturnsError(t *testing.T) {
	m := &memDAV{dropPuts: 99}
	c := newMemDAV(t, m)
	err := c.WriteFile(context.Background(), "/a.bin", bytes.NewReader(detBytes("v2", 5000)))
	require.Error(t, err, "a write the service acknowledged but never stored must not be reported as success")
	assert.Equal(t, 2, m.puts, "one retry only, never unbounded")
}

func TestWriteFile_Verify_TruncatedStore_IsRetried(t *testing.T) {
	m := &memDAV{truncate: 1}
	c := newMemDAV(t, m)
	data := detBytes("v3", 5000)
	require.NoError(t, c.WriteFile(context.Background(), "/a.bin", bytes.NewReader(data)))
	assert.Equal(t, data, m.files["/a.bin"])
	assert.Equal(t, 2, m.puts)
}

func TestWriteFile_Verify_TruncatedAlways_ReturnsError(t *testing.T) {
	m := &memDAV{truncate: 99}
	c := newMemDAV(t, m)
	require.Error(t, c.WriteFile(context.Background(), "/a.bin", bytes.NewReader(detBytes("v4", 5000))))
}

func TestWriteFile_Verify_HealthyService_OnePutOneHead(t *testing.T) {
	m := &memDAV{}
	c := newMemDAV(t, m)
	data := detBytes("v5", 5000)
	require.NoError(t, c.WriteFile(context.Background(), "/a.bin", bytes.NewReader(data)))
	assert.Equal(t, 1, m.puts)
	assert.Equal(t, 1, m.heads)
}

func TestWriteFile_Verify_NoContentLengthOnHead_ExistenceOnly(t *testing.T) {
	m := &memDAV{noLength: true}
	c := newMemDAV(t, m)
	require.NoError(t, c.WriteFile(context.Background(), "/a.bin", bytes.NewReader(detBytes("v6", 100))))
	assert.Equal(t, 1, m.puts, "no length to compare: existence is enough, no spurious retry")
}

func TestWriteFile_Verify_UnknownLengthBody_ExistenceOnlyNoRetryPossible(t *testing.T) {
	m := &memDAV{dropPuts: 99}
	c := newMemDAV(t, m)
	// a plain reader of unknown length cannot be replayed: absent after the ack is an error, not a second PUT
	err := c.WriteFile(context.Background(), "/a.bin", io.MultiReader(bytes.NewReader(detBytes("v7", 100))))
	require.Error(t, err)
	assert.Equal(t, 1, m.puts)
}

// F1 (review): an upload whose length is unknown (http.NewRequest gives ContentLength 0) must not be compared against 0 bytes.
func TestWriteFile_Verify_UnknownLengthBody_HealthyServer_Succeeds(t *testing.T) {
	m := &memDAV{}
	c := newMemDAV(t, m)
	data := detBytes("v8", 5000)
	require.NoError(t, c.WriteFile(context.Background(), "/a.bin", io.MultiReader(bytes.NewReader(data))))
	assert.Equal(t, data, m.files["/a.bin"])
	assert.Equal(t, 1, m.puts, "healthy server, existence-only verification: no second PUT")
}

func TestWriteFile_Verify_UnknownLengthBody_File_HealthyServer_Succeeds(t *testing.T) {
	m := &memDAV{}
	c := newMemDAV(t, m)
	data := detBytes("v9", 5000)
	f, err := os.CreateTemp(t.TempDir(), "up")
	require.NoError(t, err)
	_, _ = f.Write(data)
	_, _ = f.Seek(0, 0)
	defer f.Close()
	require.NoError(t, c.WriteFile(context.Background(), "/a.bin", f))
	assert.Equal(t, 1, m.puts)
}

// F2 (review): a server that does not implement HEAD cannot be verified: accept the 2xx, never upload twice.
func TestWriteFile_Verify_HeadNotSupported_AcceptsWithoutSecondPut(t *testing.T) {
	for _, code := range []int{http.StatusMethodNotAllowed, http.StatusNotImplemented} {
		m := &memDAV{headStatus: code}
		c := newMemDAV(t, m)
		require.NoError(t, c.WriteFile(context.Background(), "/a.bin", bytes.NewReader(detBytes("v10", 100))))
		assert.Equal(t, 1, m.puts, "HEAD %d: cannot verify, accept, no double upload", code)
	}
}

// R2-1: the replay PUT must keep the known Content-Length and must not be chunked.
func TestWriteFile_Verify_ReplayPut_KeepsContentLength_NotChunked(t *testing.T) {
	m := &memDAV{dropPuts: 1}
	c := newMemDAV(t, m)
	data := detBytes("r21", 4000)
	require.NoError(t, c.WriteFile(context.Background(), "/a.bin", bytes.NewReader(data)))
	require.Equal(t, 2, m.puts)
	require.Len(t, m.putLens, 2)
	assert.Equal(t, "4000|4000", m.putLens[0])
	assert.Equal(t, "4000|4000", m.putLens[1], "replay PUT must carry the original Content-Length")
	assert.Empty(t, m.putTE[1], "replay PUT must not be chunked")
}

// R2-2: HEAD 401/403 after a 2xx PUT = cannot verify (like 405/501): accept, no second PUT, no false failure.
func TestWriteFile_Verify_HeadForbidden_CannotVerify_NoSecondPut(t *testing.T) {
	for _, code := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		m := &memDAV{headStatus: code}
		c := newMemDAV(t, m)
		require.NoError(t, c.WriteFile(context.Background(), "/a.bin", bytes.NewReader(detBytes("r22", 100))))
		assert.Equal(t, 1, m.puts, "HEAD %d: cannot verify, no double upload", code)
	}
}

// R2-2: the error never says "absent" when the server refused, and carries the real HEAD status.
func TestVerifyStored_ErrorMessage_RealStatus(t *testing.T) {
	m := &memDAV{headStatus: http.StatusForbidden}
	c := newMemDAV(t, m)
	err := c.verifyStored(context.Background(), c.resolveURL("/a.bin"), 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "403")
	assert.NotContains(t, err.Error(), "absent")
	m2 := &memDAV{headStatus: http.StatusNotFound}
	c2 := newMemDAV(t, m2)
	err = c2.verifyStored(context.Background(), c2.resolveURL("/a.bin"), 10)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

// R2-3 (M2): a server storing MORE bytes than sent is a mismatch (replay), not success.
func TestWriteFile_Verify_StoredMoreThanSent_IsRetried(t *testing.T) {
	m := &memDAV{extra: 1}
	c := newMemDAV(t, m)
	data := detBytes("r23", 3000)
	require.NoError(t, c.WriteFile(context.Background(), "/a.bin", bytes.NewReader(data)))
	assert.Equal(t, data, m.files["/a.bin"])
	assert.Equal(t, 2, m.puts)
	m2 := &memDAV{extra: 99}
	c2 := newMemDAV(t, m2)
	require.Error(t, c2.WriteFile(context.Background(), "/a.bin", bytes.NewReader(detBytes("r23b", 3000))))
}

// R2-3 (M3): an empty upload has a KNOWN length of 0; a HEAD reporting Content-Length 1 is a mismatch.
func TestWriteFile_Verify_EmptyUpload_LengthMismatchDetected(t *testing.T) {
	m := &memDAV{headLen: 1}
	c := newMemDAV(t, m)
	require.Error(t, c.WriteFile(context.Background(), "/e.bin", bytes.NewReader(nil)))
	assert.Equal(t, 2, m.puts, "empty body is replayable: one bounded retry before the error")
	m2 := &memDAV{}
	c2 := newMemDAV(t, m2)
	require.NoError(t, c2.WriteFile(context.Background(), "/e.bin", bytes.NewReader(nil)))
	assert.Equal(t, 1, m2.puts)
}
