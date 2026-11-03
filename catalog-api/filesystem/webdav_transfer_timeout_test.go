package filesystem

// Unit tests (httptest, section 11.4.289(C)(1)) for the transfer-timeout contract of WebDAVClient: a transfer that keeps moving bytes is never cut
// by the metadata cap; a transfer that stalls is cut by the idle timeout; a metadata operation is bounded as a whole. The LIVE counterpart is
// TestLiveYandex_Suite (a 1 MiB upload took 67 s against the real service and failed under the former 30 s whole-request cap).

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
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
