package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	pathpkg "path"
	"strconv"
	"strings"
	"sync"
	"time"
)

// WebDAVConfig contains WebDAV connection configuration
type WebDAVConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
	Path     string `json:"path"` // Base path on the WebDAV server
}

// WebDAVClient implements FileSystemClient for WebDAV protocol
type WebDAVClient struct {
	config    *WebDAVConfig
	client    *http.Client
	baseURL   *url.URL
	connected bool
}

// MaxPropfindBytes bounds the size of one PROPFIND answer. An answer above it is an ERROR (the listing would be incomplete), never a truncation.
// A variable so tests can lower it.
var MaxPropfindBytes int64 = 64 << 20

// WebDAV timeouts. A transfer (ReadFile / WriteFile) has NO overall cap: a whole-request http.Client.Timeout also covers the body, so a file that
// takes longer than the cap to move is cut even while it makes progress (the LIVE Yandex run uploaded 1 MiB in 67 s against the former 30 s cap and
// failed, and every video streamed through ReadFile would have been cut after 30 s). A transfer is bounded by the caller's context and by an
// IDLE timeout instead: it fails only when no byte moves for webdavIdleTimeout. Metadata operations (PROPFIND, HEAD, MKCOL, DELETE, COPY) are
// bounded as a whole by webdavOpTimeout. Variables so tests can lower them.
var (
	webdavOpTimeout   = 30 * time.Second
	webdavIdleTimeout = 60 * time.Second
	// webdavResponseWait is how long the server may take to answer an upload after its last body byte was handed to the transport. Socket
	// buffers can hold seconds to minutes of a throttled upload (the real Yandex service accepts about 16 KiB/s), so the idle timeout cannot
	// apply there; only a server that never answers is cut.
	webdavResponseWait = 10 * time.Minute
)

// newWebDAVHTTPClient builds the client: connection-phase timeouts on the transport, no whole-request Timeout.
func newWebDAVHTTPClient() *http.Client {
	return &http.Client{Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           (&net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout:   15 * time.Second,
		MaxIdleConns:          16,
		MaxIdleConnsPerHost:   8,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     true,
	}}
}

// opContext bounds one metadata operation as a whole.
func opContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, webdavOpTimeout)
}

// idleGuard cancels ctx when no activity was reported for d.
type idleGuard struct {
	mu     sync.Mutex
	t      *time.Timer
	d      time.Duration
	cancel context.CancelFunc
}

func newIdleGuard(ctx context.Context, d time.Duration) (context.Context, *idleGuard) {
	c, cancel := context.WithCancel(ctx)
	g := &idleGuard{d: d, cancel: cancel}
	g.t = time.AfterFunc(d, cancel)
	return c, g
}

func (g *idleGuard) touch() { g.mu.Lock(); g.t.Reset(g.d); g.mu.Unlock() }

// rearm switches the idle period (the body is finished, the response is awaited).
func (g *idleGuard) rearm(d time.Duration) { g.mu.Lock(); g.d = d; g.t.Reset(d); g.mu.Unlock() }
func (g *idleGuard) stop()                 { g.mu.Lock(); g.t.Stop(); g.mu.Unlock(); g.cancel() }

// idleBody reports every read to the guard and stops the guard on Close (download and upload bodies).
type idleBody struct {
	rc          io.ReadCloser
	g           *idleGuard
	want        int64 // upload: the declared Content-Length (0: unknown, the end is the EOF)
	read        int64
	stopOnClose bool // download: closing the body ends the guard; upload: the request body is closed before the response arrives, the guard must live on
}

func (b *idleBody) Read(p []byte) (int, error) {
	b.g.touch()
	n, err := b.rc.Read(p)
	b.read += int64(n)
	// the transport reads exactly Content-Length bytes and may never ask for the EOF: the last byte is the end of the body
	if !b.stopOnClose && (err == io.EOF || (b.want > 0 && b.read >= b.want)) {
		b.g.rearm(webdavResponseWait)
	} else {
		b.g.touch()
	}
	return n, err
}
func (b *idleBody) Close() error {
	if b.stopOnClose {
		b.g.stop()
	}
	return b.rc.Close()
}

// redactURL renders raw for an error message WITHOUT its credentials: a URL userinfo password becomes "xxxxx" (url.URL.Redacted). An
// unparsable value is not echoed at all (it may be a credential-bearing string that failed to parse).
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "<unparsable url>"
	}
	return u.Redacted()
}

// NewWebDAVClient creates a new WebDAV client. config.Path is the root collection UNDER the URL's own path (WF22 F5): url
// https://nas/remote.php/dav/files/alice with path /Movies addresses .../alice/Movies, it does not replace the URL's path.
func NewWebDAVClient(config *WebDAVConfig) (*WebDAVClient, error) {
	baseURL, err := url.Parse(config.URL)
	if err != nil {
		// url.Error would echo the whole URL, userinfo included, into an error that ends up in the scan "reason" (WF22 R9).
		var ue *url.Error
		if errors.As(err, &ue) {
			return nil, fmt.Errorf("invalid WebDAV URL: %v", ue.Err)
		}
		return nil, fmt.Errorf("invalid WebDAV URL")
	}
	if config.Path != "" && config.Path != "/" {
		baseURL.Path = pathpkg.Join("/", baseURL.Path, config.Path)
		baseURL.RawPath = ""
	}

	return &WebDAVClient{
		config:  config,
		client:  newWebDAVHTTPClient(),
		baseURL: baseURL,
	}, nil
}

// SetHTTPTransport replaces the HTTP transport (instrumentation: request counting, outbound guards in live tests). It must be called before Connect.
func (c *WebDAVClient) SetHTTPTransport(rt http.RoundTripper) { c.client.Transport = rt }

// Connect establishes the WebDAV connection
func (c *WebDAVClient) Connect(ctx context.Context) error {
	ctx, cancel := opContext(ctx)
	defer cancel()
	// Test the connection with a PROPFIND request
	req, err := http.NewRequestWithContext(ctx, "PROPFIND", c.baseURL.String(), nil)
	if err != nil {
		return fmt.Errorf("failed to create PROPFIND request: %w", err)
	}

	if c.config.Username != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	req.Header.Set("Depth", "0")

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to WebDAV server: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMultiStatus && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("WebDAV server returned status %d", resp.StatusCode)
	}

	c.connected = true
	return nil
}

// Disconnect closes the WebDAV connection
func (c *WebDAVClient) Disconnect(ctx context.Context) error {
	c.connected = false
	return nil
}

// IsConnected returns true if the client is connected
func (c *WebDAVClient) IsConnected() bool {
	return c.connected
}

// TestConnection tests the WebDAV connection
func (c *WebDAVClient) TestConnection(ctx context.Context) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	return c.Connect(ctx) // Re-test connection
}

// resolveURL resolves a relative path to a full WebDAV URL. The path is a slash path whatever the host OS and is cleaned as if rooted, so a
// ".." SEGMENT cannot climb out of the base; a name that merely CONTAINS ".." ("Wait.. Live", "...And Justice for All") is kept as it is (WF22 R11).
func (c *WebDAVClient) resolveURL(p string) string {
	cleanPath := ConfinePath(p)
	u := *c.baseURL
	u.Path = pathpkg.Join(u.Path, cleanPath)
	u.RawPath = ""
	return u.String()
}

// ReadFile reads a file from the WebDAV server
func (c *WebDAVClient) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	fullURL := c.resolveURL(path)
	ctx, guard := newIdleGuard(ctx, webdavIdleTimeout)
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		guard.stop()
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}

	if c.config.Username != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		guard.stop()
		return nil, fmt.Errorf("failed to retrieve WebDAV file %s: %w", redactURL(fullURL), err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		guard.stop()
		return nil, fmt.Errorf("WebDAV server returned status %d for file %s", resp.StatusCode, redactURL(fullURL))
	}

	guard.touch()
	return &idleBody{rc: resp.Body, g: guard, stopOnClose: true}, nil
}

// WriteFile writes a file to the WebDAV server
func (c *WebDAVClient) WriteFile(ctx context.Context, path string, data io.Reader) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	fullURL := c.resolveURL(path)
	ctx, guard := newIdleGuard(ctx, webdavIdleTimeout)
	defer guard.stop()
	req, err := http.NewRequestWithContext(ctx, "PUT", fullURL, data)
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %w", err)
	}
	// Wrap AFTER NewRequest so that Content-Length and GetBody it derived from the concrete reader type are kept (a wrapped reader passed in
	// would make the upload chunked); every body read feeds the idle timer.
	if req.Body != nil {
		req.Body = &idleBody{rc: req.Body, g: guard, want: req.ContentLength}
	}

	if c.config.Username != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload WebDAV file %s: %w", redactURL(fullURL), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("WebDAV server returned status %d for file %s", resp.StatusCode, redactURL(fullURL))
	}

	return nil
}

// GetFileInfo gets information about a file
func (c *WebDAVClient) GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	ctx, cancel := opContext(ctx)
	defer cancel()
	fullURL := c.resolveURL(path)
	req, err := http.NewRequestWithContext(ctx, "HEAD", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create HEAD request: %w", err)
	}

	if c.config.Username != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get WebDAV file info %s: %w", redactURL(fullURL), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("WebDAV server returned status %d for file %s", resp.StatusCode, redactURL(fullURL))
	}

	// Parse content length
	size := int64(0)
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		if s, err := strconv.ParseInt(cl, 10, 64); err == nil {
			size = s
		}
	}

	// Parse last modified; a missing or unparsable value is the zero time, never "now" (a made-up time would look like a change on every scan)
	var modTime time.Time
	if lm := resp.Header.Get("Last-Modified"); lm != "" {
		if t, err := time.Parse(time.RFC1123, lm); err == nil {
			modTime = t
		}
	}

	// Check if it's a directory (simplified check)
	isDir := strings.HasSuffix(path, "/") || resp.Header.Get("Content-Type") == "httpd/unix-directory"

	return &FileInfo{
		Name:    pathpkg.Base(path),
		Size:    size,
		ModTime: modTime,
		IsDir:   isDir,
		Mode:    0644, // Default mode
		Path:    path,
	}, nil
}

// ListDirectory lists files in a directory
func (c *WebDAVClient) ListDirectory(ctx context.Context, path string) ([]*FileInfo, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	ctx, cancel := opContext(ctx)
	defer cancel()
	fullURL := c.resolveURL(path)
	// A collection URL without a trailing slash makes servers answer 301 to the slash form, and the Go client then replays the PROPFIND as a GET
	// (status 200 with an HTML page). Ask for the slash form directly.
	if !strings.HasSuffix(fullURL, "/") {
		fullURL += "/"
	}
	req, err := http.NewRequestWithContext(ctx, "PROPFIND", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create PROPFIND request: %w", err)
	}

	if c.config.Username != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	req.Header.Set("Depth", "1")
	req.Header.Set("Content-Type", "application/xml")

	// Create a minimal PROPFIND request body
	body := `<?xml version="1.0" encoding="utf-8" ?>
<D:propfind xmlns:D="DAV:">
	<D:prop>
		<D:displayname/>
		<D:getcontentlength/>
		<D:getlastmodified/>
		<D:resourcetype/>
	</D:prop>
</D:propfind>`

	req.Body = io.NopCloser(strings.NewReader(body))
	req.ContentLength = int64(len(body))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list WebDAV directory %s: %w", redactURL(fullURL), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMultiStatus {
		return nil, fmt.Errorf("WebDAV server returned status %d for directory %s", resp.StatusCode, redactURL(fullURL))
	}

	// PA-03: a namespace-aware, streaming parse (webdav_propfind.go; the former string splitting found nothing in an Apache answer). The read is
	// bounded by MaxPropfindBytes and by the entry limit the scanner put in ctx: an oversized answer is an error, never a silent truncation.
	limit, _ := ListLimit(ctx)
	entries, err := parsePropfindStream(&boundedReader{r: resp.Body, max: MaxPropfindBytes}, fullURL, limit)
	if err != nil {
		if errors.Is(err, errPropfindTooLarge) {
			return nil, fmt.Errorf("WebDAV directory %s: %w (bound %d bytes)", redactURL(fullURL), err, MaxPropfindBytes)
		}
		return nil, fmt.Errorf("failed to read WebDAV response for %s: %w", redactURL(fullURL), err)
	}
	return entries, nil
}

// FileExists checks if a file exists
func (c *WebDAVClient) FileExists(ctx context.Context, path string) (bool, error) {
	if !c.IsConnected() {
		return false, fmt.Errorf("not connected")
	}

	ctx, cancel := opContext(ctx)
	defer cancel()
	fullURL := c.resolveURL(path)
	req, err := http.NewRequestWithContext(ctx, "HEAD", fullURL, nil)
	if err != nil {
		return false, fmt.Errorf("failed to create HEAD request: %w", err)
	}

	if c.config.Username != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to check WebDAV file existence %s: %w", redactURL(fullURL), err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// CreateDirectory creates a directory
func (c *WebDAVClient) CreateDirectory(ctx context.Context, path string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	ctx, cancel := opContext(ctx)
	defer cancel()
	fullURL := c.resolveURL(path)
	req, err := http.NewRequestWithContext(ctx, "MKCOL", fullURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create MKCOL request: %w", err)
	}

	if c.config.Username != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create WebDAV directory %s: %w", redactURL(fullURL), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("WebDAV server returned status %d for directory %s", resp.StatusCode, redactURL(fullURL))
	}

	return nil
}

// DeleteDirectory deletes a directory
func (c *WebDAVClient) DeleteDirectory(ctx context.Context, path string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	ctx, cancel := opContext(ctx)
	defer cancel()
	fullURL := c.resolveURL(path)
	req, err := http.NewRequestWithContext(ctx, "DELETE", fullURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request: %w", err)
	}

	if c.config.Username != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete WebDAV directory %s: %w", redactURL(fullURL), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("WebDAV server returned status %d for directory %s", resp.StatusCode, redactURL(fullURL))
	}

	return nil
}

// DeleteFile deletes a file
func (c *WebDAVClient) DeleteFile(ctx context.Context, path string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	ctx, cancel := opContext(ctx)
	defer cancel()
	fullURL := c.resolveURL(path)
	req, err := http.NewRequestWithContext(ctx, "DELETE", fullURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create DELETE request: %w", err)
	}

	if c.config.Username != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete WebDAV file %s: %w", redactURL(fullURL), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("WebDAV server returned status %d for file %s", resp.StatusCode, redactURL(fullURL))
	}

	return nil
}

// CopyFile copies a file on the WebDAV server
func (c *WebDAVClient) CopyFile(ctx context.Context, srcPath, dstPath string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	ctx, cancel := opContext(ctx)
	defer cancel()
	srcURL := c.resolveURL(srcPath)
	dstURL := c.resolveURL(dstPath)

	req, err := http.NewRequestWithContext(ctx, "COPY", srcURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create COPY request: %w", err)
	}

	if c.config.Username != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	req.Header.Set("Destination", dstURL)

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to copy WebDAV file from %s to %s: %w", redactURL(srcURL), redactURL(dstURL), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("WebDAV server returned status %d for copy operation", resp.StatusCode)
	}

	return nil
}

// GetProtocol returns the protocol name
func (c *WebDAVClient) GetProtocol() string {
	return "webdav"
}

// GetConfig returns the WebDAV configuration
func (c *WebDAVClient) GetConfig() interface{} {
	return c.config
}
