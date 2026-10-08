package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	pathpkg "path"
	"strconv"
	"strings"
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
		client:  &http.Client{Timeout: 30 * time.Second},
		baseURL: baseURL,
	}, nil
}

// Connect establishes the WebDAV connection
func (c *WebDAVClient) Connect(ctx context.Context) error {
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
	cleanPath := pathpkg.Clean("/" + p)
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
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create GET request: %w", err)
	}

	if c.config.Username != "" {
		req.SetBasicAuth(c.config.Username, c.config.Password)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve WebDAV file %s: %w", redactURL(fullURL), err)
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("WebDAV server returned status %d for file %s", resp.StatusCode, redactURL(fullURL))
	}

	return resp.Body, nil
}

// WriteFile writes a file to the WebDAV server
func (c *WebDAVClient) WriteFile(ctx context.Context, path string, data io.Reader) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	fullURL := c.resolveURL(path)
	req, err := http.NewRequestWithContext(ctx, "PUT", fullURL, data)
	if err != nil {
		return fmt.Errorf("failed to create PUT request: %w", err)
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
