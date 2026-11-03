package filesystem

// Unit tests (httptest, section 11.4.289(C)(1)) for the transfer-timeout contract of WebDAVClient: a transfer that keeps moving bytes is never cut
// by the metadata cap; a transfer that stalls is cut by the idle timeout; a metadata operation is bounded as a whole. The LIVE counterpart is
// TestLiveYandex_Suite (the pre-fix client failed its 1 MiB upload with Client.Timeout; evidence docs/qa/webdav-yandex-live-20261103/run_RED_pre_fix_client_base05709505.log).

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func lowerTimeouts(t *testing.T, op, idle time.Duration) {
	t.Helper()
	po, pi, pr := webdavOpTimeout, webdavIdleTimeout, webdavResponseWait
	webdavOpTimeout, webdavIdleTimeout, webdavResponseWait = op, idle, 5*time.Second
	t.Cleanup(func() { webdavOpTimeout, webdavIdleTimeout, webdavResponseWait = po, pi, pr })
}

func TestWebDAVTransfer_SlowButProgressingDownloadSucceeds(t *testing.T) {
	lowerTimeouts(t, 300*time.Millisecond, 400*time.Millisecond)
	_, c := newTestWebDAVServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" {
			w.WriteHeader(207)
			return
		}
		fl := w.(http.Flusher)
		for i := 0; i < 12; i++ { // 12 x 100 ms = 1.2 s > opTimeout (300 ms) and > idleTimeout (400 ms) in total
			_, _ = w.Write(bytes.Repeat([]byte{'a'}, 1000))
			fl.Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}))
	rc, err := c.ReadFile(context.Background(), "/big.bin")
	require.NoError(t, err)
	b, err := io.ReadAll(rc)
	require.NoError(t, err, "a download that keeps progressing must not be cut by the metadata cap")
	require.Len(t, b, 12000)
	require.NoError(t, rc.Close())
}

func TestWebDAVTransfer_StalledDownloadIsCutByIdleTimeout(t *testing.T) {
	lowerTimeouts(t, 5*time.Second, 300*time.Millisecond)
	_, c := newTestWebDAVServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" {
			w.WriteHeader(207)
			return
		}
		_, _ = w.Write([]byte("first"))
		w.(http.Flusher).Flush()
		select {
		case <-r.Context().Done():
		case <-time.After(10 * time.Second):
		}
	}))
	rc, err := c.ReadFile(context.Background(), "/stall.bin")
	require.NoError(t, err)
	t0 := time.Now()
	_, err = io.ReadAll(rc)
	require.Error(t, err, "a stalled body must fail, not hang")
	require.Less(t, time.Since(t0), 3*time.Second)
	_ = rc.Close()
}

func TestWebDAVTransfer_SlowButProgressingUploadSucceeds(t *testing.T) {
	lowerTimeouts(t, 300*time.Millisecond, 400*time.Millisecond)
	var got int
	_, c := newTestWebDAVServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" {
			w.WriteHeader(207)
			return
		}
		buf := make([]byte, 1000)
		for {
			n, err := r.Body.Read(buf)
			got += n
			if err != nil {
				break
			}
			time.Sleep(100 * time.Millisecond) // throttled server: about 10 KB/s
		}
		w.WriteHeader(201)
	}))
	require.NoError(t, c.WriteFile(context.Background(), "/up.bin", bytes.NewReader(bytes.Repeat([]byte{'b'}, 12000))))
	require.Equal(t, 12000, got)
}

func TestWebDAVTransfer_SizedUploadKeepsContentLength(t *testing.T) {
	var cl int64
	var te []string
	_, c := newTestWebDAVServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			cl, te = r.ContentLength, r.TransferEncoding
			_, _ = io.Copy(io.Discard, r.Body)
			w.WriteHeader(201)
			return
		}
		w.WriteHeader(207)
	}))
	require.NoError(t, c.WriteFile(context.Background(), "/s.bin", bytes.NewReader(make([]byte, 5000))))
	require.Equal(t, int64(5000), cl, "a sized body must stay sized (wrapping must not force chunked)")
	require.Empty(t, te)
	require.NoError(t, c.WriteFile(context.Background(), "/s2.bin", strings.NewReader("hello")))
	require.Equal(t, int64(5), cl)
}

func TestWebDAVTransfer_MetadataOperationBoundedAsAWhole(t *testing.T) {
	lowerTimeouts(t, 300*time.Millisecond, 5*time.Second)
	_, c := newTestWebDAVServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" {
			w.WriteHeader(207)
			return
		}
		select { // HEAD hangs
		case <-r.Context().Done():
		case <-time.After(5 * time.Second):
		}
	}))
	t0 := time.Now()
	_, err := c.GetFileInfo(context.Background(), "/hang")
	require.Error(t, err)
	require.Less(t, time.Since(t0), 2*time.Second)
}

func TestWebDAVClient_DefaultHTTPClientHasNoWholeRequestTimeout(t *testing.T) {
	c, err := NewWebDAVClient(&WebDAVConfig{URL: "https://example.invalid"})
	require.NoError(t, err)
	require.Zero(t, c.client.Timeout, "a whole-request Timeout cuts long transfers that are making progress")
	tr, ok := c.client.Transport.(*http.Transport)
	require.True(t, ok)
	require.NotZero(t, tr.TLSHandshakeTimeout)
	require.NotNil(t, tr.DialContext)
}

// F1: an empty upload must stay an empty sized upload (Content-Length 0, no chunked encoding) and must still be bounded by the response wait.
func TestWebDAVTransfer_EmptyUploadIsSizedZeroNotChunked(t *testing.T) {
	var cl int64 = -2
	var te []string
	_, c := newTestWebDAVServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" {
			cl, te = r.ContentLength, r.TransferEncoding
			w.WriteHeader(201)
			return
		}
		w.WriteHeader(207)
	}))
	require.NoError(t, c.WriteFile(context.Background(), "/empty.bin", bytes.NewReader(nil)))
	require.Equal(t, int64(0), cl, "an empty body must be sent with Content-Length 0")
	require.Empty(t, te, "an empty body must not go out chunked")
}

// F8 / mutant M2: closing a download body must stop its idle guard.
func TestWebDAVTransfer_DownloadCloseStopsIdleGuard(t *testing.T) {
	_, c := newTestWebDAVServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PROPFIND" {
			w.WriteHeader(207)
			return
		}
		_, _ = w.Write([]byte("x"))
	}))
	rc, err := c.ReadFile(context.Background(), "/f")
	require.NoError(t, err)
	ib, ok := rc.(*idleBody)
	require.True(t, ok)
	_, _ = io.ReadAll(rc)
	require.NoError(t, rc.Close())
	require.False(t, ib.g.t.Stop(), "Close must have stopped the idle timer (Stop reports false for an already stopped timer)")
}

// F8 / mutants M3 and M6: ListDirectory and Connect are bounded as a whole by the metadata cap.
func TestWebDAVTransfer_ListDirectoryAndConnectBoundedAsAWhole(t *testing.T) {
	lowerTimeouts(t, 300*time.Millisecond, 5*time.Second)
	var hang bool
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		h := hang
		mu.Unlock()
		if h {
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
			return
		}
		w.WriteHeader(207)
	}))
	t.Cleanup(srv.Close)
	c, err := NewWebDAVClient(&WebDAVConfig{URL: srv.URL, Username: "u", Password: "p"})
	require.NoError(t, err)
	require.NoError(t, c.Connect(context.Background()))
	mu.Lock()
	hang = true
	mu.Unlock()
	t0 := time.Now()
	_, err = c.ListDirectory(context.Background(), "/d")
	require.Error(t, err)
	require.Less(t, time.Since(t0), 2*time.Second, "ListDirectory must be bounded by the metadata cap")
	c2, err := NewWebDAVClient(&WebDAVConfig{URL: srv.URL, Username: "u", Password: "p"})
	require.NoError(t, err)
	t0 = time.Now()
	require.Error(t, c2.Connect(context.Background()))
	require.Less(t, time.Since(t0), 2*time.Second, "Connect must be bounded by the metadata cap")
}
