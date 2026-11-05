package filesystem

import (
	"catalogizer/internal/netguard"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	pathpkg "path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
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
// takes longer than the cap to move is cut even while it makes progress (LIVE RED, committed as
// docs/qa/webdav-yandex-live-20261103/run_RED_pre_fix_client_base05709505.log: the pre-fix client from commit 05709505 failed the 1 MiB upload of step
// 04 with "Client.Timeout exceeded" against the real service; every video streamed through ReadFile would be cut the same way). A transfer is bounded by the caller's context and by an
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
	// SECQ-1/W5: operator-class outbound policy. The dial checks the RESOLVED address of every connection (loopback, link-local and
	// cloud-metadata addresses are refused, RFC 1918 / ULA / public stay reachable; see netguard/operator.go), every redirect hop is
	// re-checked, and an environment proxy is honoured only after the destination host was checked.
	return &http.Client{CheckRedirect: netguard.OperatorCheckRedirect, Transport: netguard.OperatorTransport()}
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
	fired  atomic.Bool // set only by the timer: the idle period elapsed (as opposed to a caller cancel or a normal stop)
}

// idleErr turns a transport/body error into a diagnosable one when the idle timer, not the caller, ended the transfer.
func (g *idleGuard) idleErr(err error, phase string) error {
	if err == nil || err == io.EOF || !g.fired.Load() {
		return err
	}
	g.mu.Lock()
	d := g.d
	g.mu.Unlock()
	return fmt.Errorf("webdav idle timeout: no data for %v (%s): %w", d, phase, err)
}

func newIdleGuard(ctx context.Context, d time.Duration) (context.Context, *idleGuard) {
	c, cancel := context.WithCancel(ctx)
	g := &idleGuard{d: d, cancel: cancel}
	g.t = time.AfterFunc(d, func() { g.fired.Store(true); cancel() })
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
	err = b.g.idleErr(err, fmt.Sprintf("body, %d bytes transferred", b.read))
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
		err = guard.idleErr(err, "waiting for response headers")
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
	want, getBody, err := c.putOnce(ctx, fullURL, data, -1)
	if err != nil {
		return err
	}

	// Defensive hardening (WD-F1/SECQ-2; NOT a proven fix: no lost acknowledged write was ever reproduced live). After a 2xx PUT the target is
	// HEADed: absent, or a Content-Length different from the bytes sent (when both are known), triggers one replay of the PUT (only when the body
	// can be replayed) and a second check; still wrong is an error. LIMITS: this is an existence + SIZE check only. Same-size different content
	// is NOT detected (an ETag/MD5 comparison is a tracked follow-up), a body of unknown length is checked for existence only, and a server that
	// answers HEAD with 405/501 cannot be verified and the 2xx is accepted (logged).
	vErr := c.verifyStored(ctx, fullURL, want)
	if vErr == nil {
		return nil
	}
	if errors.Is(vErr, errWebDAVHeadUnsupported) {
		log.Printf("webdav: %s: HEAD not usable for read-back (unsupported or refused), upload acknowledged but not verifiable", redactURL(fullURL))
		return nil
	}
	if getBody == nil {
		return fmt.Errorf("WebDAV upload of %s was acknowledged but not verified and the body cannot be replayed: %w", redactURL(fullURL), vErr)
	}
	body, gerr := getBody()
	if gerr != nil {
		return fmt.Errorf("WebDAV upload of %s was acknowledged but not verified (%v): %w", redactURL(fullURL), gerr, vErr)
	}
	if _, _, err := c.putOnce(ctx, fullURL, body, want); err != nil {
		return fmt.Errorf("WebDAV re-upload of %s after failed verification (%v): %w", redactURL(fullURL), vErr, err)
	}
	if err := c.verifyStored(ctx, fullURL, want); err != nil {
		return fmt.Errorf("WebDAV upload of %s was acknowledged twice but verification still fails: %w", redactURL(fullURL), err)
	}
	return nil
}

// putOnce performs one PUT. It returns the declared body length (-1 when unknown) and a replay function (nil when the body cannot be replayed).
// knownLen >= 0 (a replay) restores the original Content-Length on a body whose type hides it, so the replay is not sent chunked.
func (c *WebDAVClient) putOnce(ctx context.Context, fullURL string, data io.Reader, knownLen int64) (int64, func() (io.ReadCloser, error), error) {
	ctx, guard := newIdleGuard(ctx, webdavIdleTimeout)
	defer guard.stop()
	req, err := http.NewRequestWithContext(ctx, "PUT", fullURL, data)
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create PUT request: %w", err)
	}
	if knownLen >= 0 && req.Body != nil {
		req.ContentLength = knownLen
		if knownLen == 0 {
			req.Body = http.NoBody
		}
	}
	want, getBody := req.ContentLength, req.GetBody
	if req.ContentLength == 0 && req.Body != nil && req.Body != http.NoBody {
		want = -1 // unknown length (file, pipe, MultiReader): ContentLength 0 means "unknown" here, never "0 bytes"
	}
	// Wrap AFTER NewRequest so that Content-Length and GetBody it derived from the concrete reader type are kept (a wrapped reader passed in
	// would make the upload chunked); every body read feeds the idle timer.
	if req.Body != nil && req.Body != http.NoBody {
		req.Body = &idleBody{rc: req.Body, g: guard, want: req.ContentLength}
	} else {
		// An empty body: nothing is read, so the guard would never leave the idle period; the response is awaited at once. NoBody is left
		// unwrapped so the request stays a sized zero-length one (a wrapped NoBody goes out chunked with ContentLength -1).
		guard.rearm(webdavResponseWait)
	}

	if c.config.Username != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		err = guard.idleErr(err, "upload, waiting for the server")
		return 0, nil, fmt.Errorf("failed to upload WebDAV file %s: %w", redactURL(fullURL), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return 0, nil, fmt.Errorf("WebDAV server returned status %d for file %s", resp.StatusCode, redactURL(fullURL))
	}
	return want, getBody, nil
}

var errWebDAVHeadUnsupported = errors.New("HEAD not supported")

// verifyStored HEADs the target: present with 200, and, when both the sent length (want >= 0) and a Content-Length header are known, equal.
func (c *WebDAVClient) verifyStored(ctx context.Context, fullURL string, want int64) error {
	ctx, cancel := opContext(ctx)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "HEAD", fullURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create HEAD request: %w", err)
	}
	if c.config.Username != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("read-back HEAD failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusMethodNotAllowed || resp.StatusCode == http.StatusNotImplemented ||
		resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("read-back HEAD answered %d: %w", resp.StatusCode, errWebDAVHeadUnsupported)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("read-back HEAD answered status %d (the file could not be confirmed after an acknowledged write)", resp.StatusCode)
	}
	if want >= 0 {
		if cl := resp.Header.Get("Content-Length"); cl != "" {
			if got, perr := strconv.ParseInt(cl, 10, 64); perr == nil && got != want {
				return fmt.Errorf("read-back size %d differs from the %d bytes sent", got, want)
			}
		}
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
