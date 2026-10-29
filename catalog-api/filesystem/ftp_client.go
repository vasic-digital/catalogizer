package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	pathpkg "path"
	"strings"
	"sync"
	"time"

	"github.com/jlaffaye/ftp"
)

// FTPConfig contains FTP connection configuration
type FTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Path     string `json:"path"` // Base path on the FTP server
}

// FTPClient implements FileSystemClient for FTP protocol
type FTPClient struct {
	config    *FTPConfig
	client    *ftp.ServerConn
	connected bool
	// base is the ABSOLUTE base directory every path is resolved against. Connect does CWD <config.Path>; if the path was relative
	// ("movies") the server resolved it against the login directory, so prefixing the relative text again would address
	// <login>/movies/movies (WF22 R18). base is what the server says the working directory is after the CWD.
	base string
	// mu guards the fields client, connected, base and gen: every read and write of them (Connect, Disconnect, reconnectIf, IsConnected and
	// session()) holds it. Every protocol operation (ReadFile, ListDirectory, ...) calls session() ONCE and uses only the (conn, base) it
	// returned, so a concurrent Disconnect / redial can no longer hand it a nil *ftp.ServerConn or a torn client/base pair.
	// Scope of the claim: this makes the client's OWN fields race-free. The underlying *ftp.ServerConn is not goroutine-safe: two
	// operations on one client at the same time, or a Disconnect's QUIT overlapping an operation that is mid-command on that connection,
	// still interleave on the control connection (an in-flight operation then fails with a connection error). The production callers create
	// one client per job via ClientFactory.CreateClient (no goroutine sharing was found in universal_scanner.go; other handlers were not audited).
	mu sync.Mutex
	// gen counts Disconnect calls. A reconnect started before a Disconnect must not install its new connection afterwards.
	gen uint64
}

// NewFTPClient creates a new FTP client
func NewFTPClient(config *FTPConfig) *FTPClient {
	return &FTPClient{
		config:    config,
		connected: false,
	}
}

// Connect establishes the FTP connection
func (c *FTPClient) Connect(ctx context.Context) error {
	client, base, err := c.dial(ctx)
	if err != nil {
		return err
	}
	c.mu.Lock()
	old := c.client
	c.client, c.base, c.connected = client, base, true
	c.mu.Unlock()
	if old != nil {
		_ = old.Quit() // a previous connection is ended, not leaked (an operation still running on it fails with a connection error)
	}
	return nil
}

// dial opens, logs in and positions a new control connection without touching the client's fields.
func (c *FTPClient) dial(ctx context.Context) (*ftp.ServerConn, string, error) {
	addr := net.JoinHostPort(c.config.Host, fmt.Sprintf("%d", c.config.Port))

	client, err := ftp.Dial(addr, ftp.DialWithTimeout(30*time.Second), ftp.DialWithContext(ctx))
	if err != nil {
		return nil, "", fmt.Errorf("failed to connect to FTP server: %w", err)
	}

	err = client.Login(c.config.Username, c.config.Password)
	if err != nil {
		client.Quit()
		return nil, "", fmt.Errorf("failed to login to FTP server: %w", err)
	}

	// Change to base directory if specified
	if c.config.Path != "" {
		err = client.ChangeDir(c.config.Path)
		if err != nil {
			client.Quit()
			return nil, "", fmt.Errorf("failed to change to base directory %s: %w", c.config.Path, err)
		}
	}

	base := ""
	if c.config.Path != "" {
		if cwd, err := client.CurrentDir(); err == nil && strings.HasPrefix(cwd, "/") {
			base = pathpkg.Clean(cwd)
		} else {
			// The server does not answer PWD (or answers something unusable): an absolute configured path is used as it is; a relative one is
			// taken as relative to "/" (the login directory of a chrooted account).
			base = pathpkg.Clean("/" + c.config.Path)
		}
	}
	return client, base, nil
}

// Disconnect closes the FTP connection
func (c *FTPClient) Disconnect(ctx context.Context) error {
	c.mu.Lock()
	conn := c.client
	c.client = nil
	c.connected = false
	c.gen++
	c.mu.Unlock()
	if conn != nil {
		return conn.Quit()
	}
	return nil
}

// IsConnected returns true if the client is connected
func (c *FTPClient) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connected && c.client != nil
}

// session returns a consistent snapshot of the control connection and base directory, or a "not connected" error. It is the only way an
// operation reads c.client / c.base: the lock is held while they are read, and the operation then uses the returned values only.
func (c *FTPClient) session() (*ftp.ServerConn, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.connected || c.client == nil {
		return nil, "", fmt.Errorf("not connected")
	}
	return c.client, c.base, nil
}

// TestConnection tests the FTP connection
func (c *FTPClient) TestConnection(ctx context.Context) error {
	conn, _, err := c.session()
	if err != nil {
		return err
	}
	_, err = conn.CurrentDir()
	return err
}

// resolvePath resolves a path within the FTP base directory (always as a slash path, whatever the host OS). The request is confined with
// ConfinePath first, so "/../../etc" under "/srv/movies" is "/srv/movies/etc", never "/etc" (WF22 N2).
func (c *FTPClient) resolvePath(base, p string) string {
	if base != "" {
		return JoinConfined(base, p)
	}
	if c.config.Path != "" { // not connected through Connect (unit use): the configured path, made absolute
		return JoinConfined(pathpkg.Join("/", c.config.Path), p)
	}
	return ConfineKeepForm(p)
}

// ReadFile reads a file from the FTP server
func (c *FTPClient) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	conn, base, err := c.session()
	if err != nil {
		return nil, err
	}
	fullPath := c.resolvePath(base, path)
	resp, err := conn.Retr(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve FTP file %s: %w", fullPath, err)
	}
	return &ftpReadCloser{resp: resp, c: c, conn: conn, ctx: context.WithoutCancel(ctx)}, nil
}

// ftpReadCloser closes an FTP data transfer and keeps the client usable afterwards. Closing a RETR before its end (header sniffing,
// thumbnails, range-like reads) makes some servers (measured on a real Synology FTP service) drop the CONTROL connection; the next command then
// failed with "EOF" while IsConnected() still said true. When Close fails, the dead session is discarded and a
// new one is opened, so the next operation works. A transfer that was read to its end keeps its normal Close error semantics.
type ftpReadCloser struct {
	resp   *ftp.Response
	c      *FTPClient
	conn   *ftp.ServerConn // the control connection this transfer ran on
	ctx    context.Context // caller's context minus its cancellation (Close usually runs after the request context ended)
	atEOF  bool
	closed bool
}

func (r *ftpReadCloser) Read(p []byte) (int, error) {
	n, err := r.resp.Read(p)
	if err == io.EOF {
		r.atEOF = true
	}
	return n, err
}

// Close ends the transfer. Semantics (asserted by the unit and live tests):
//   - clean close: nil;
//   - the server dropped the control connection (io.EOF) after an early close: the session is re-established and nil is returned
//     (the caller aborted on purpose); if the reader had been read to EOF the original error is returned;
//   - any other failure (a 4xx/5xx status such as 426): returned as is, the dead session is replaced when this client still owns it;
//   - if the client was Disconnect()ed or already moved to another connection since ReadFile, nothing is redialled; a redial that was
//     already in flight when Disconnect ran is discarded, and if it fails the failure is not reported (the user disconnected on purpose).
//     Without a Disconnect, a failed redial is returned as "... and reconnect failed".
func (r *ftpReadCloser) Close() error {
	if r.closed {
		return nil
	}
	r.closed = true
	err := r.resp.Close()
	if err == nil {
		return nil
	}
	if rerr := r.c.reconnectIf(r.ctx, r.conn); rerr != nil {
		return fmt.Errorf("FTP transfer close failed (%v) and reconnect failed: %w", err, rerr)
	}
	if !r.atEOF && errors.Is(err, io.EOF) {
		return nil // aborted early by the caller on purpose; the session has been re-established
	}
	return err
}

// reconnectIf replaces the control connection `old` with a fresh one, but only while `old` is still the client's current connection and
// no Disconnect happened meanwhile: the new connection is dialled outside the lock and installed only if the Disconnect generation is
// unchanged, otherwise it is Quit and discarded. A late Close can therefore never resurrect a disconnected client, not even when
// Disconnect runs while the redial is in flight.
func (c *FTPClient) reconnectIf(ctx context.Context, old *ftp.ServerConn) error {
	c.mu.Lock()
	if c.client != old || old == nil {
		c.mu.Unlock()
		return nil
	}
	gen := c.gen
	c.client = nil
	c.connected = false
	c.mu.Unlock()
	_ = old.Quit()
	rctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	fresh, base, err := c.dial(rctx)
	if err != nil {
		c.mu.Lock()
		disconnected := c.gen != gen
		c.mu.Unlock()
		if disconnected {
			return nil // the user disconnected on purpose while the redial was in flight; its failure is not an error for Close
		}
		return err
	}
	c.mu.Lock()
	if c.gen != gen || c.client != nil {
		c.mu.Unlock()
		_ = fresh.Quit() // Disconnect (or another Connect) won the race: do not install
		return nil
	}
	c.client, c.base, c.connected = fresh, base, true
	c.mu.Unlock()
	return nil
}

// WriteFile writes a file to the FTP server
func (c *FTPClient) WriteFile(ctx context.Context, path string, data io.Reader) error {
	conn, base, err := c.session()
	if err != nil {
		return err
	}
	fullPath := c.resolvePath(base, path)

	// Ensure the directory exists
	dir := pathpkg.Dir(fullPath)
	if dir != "." && dir != "/" {
		if err := conn.MakeDir(dir); err != nil {
			// Ignore error if directory already exists
			// FTP doesn't have a standard way to check if directory exists
		}
	}

	err = conn.Stor(fullPath, data)
	if err != nil {
		return fmt.Errorf("failed to store FTP file %s: %w", fullPath, err)
	}
	return nil
}

// GetFileInfo gets information about a file
func (c *FTPClient) GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	conn, base, err := c.session()
	if err != nil {
		return nil, err
	}
	fullPath := c.resolvePath(base, path)

	// SIZE is defined for plain files only: a directory answers "550 not a plain file". Try it first (cheap, files), and on
	// failure resolve the entry through the parent listing, which also tells a directory from a missing path.
	size, sizeErr := conn.FileSize(fullPath)
	if sizeErr == nil {
		return &FileInfo{
			Name:    pathpkg.Base(path),
			Size:    size,
			ModTime: c.fileModTime(conn, fullPath),
			IsDir:   false,
			Mode:    0644,
			Path:    path,
		}, nil
	}
	name := pathpkg.Base(fullPath)
	if fullPath == "/" || name == "." || name == "/" {
		return &FileInfo{Name: name, IsDir: true, Mode: 0755, Path: path, ModTime: time.Time{}}, nil
	}
	entries, listErr := conn.List(pathpkg.Dir(fullPath))
	if listErr != nil {
		return nil, fmt.Errorf("failed to get FTP file info %s: %w", fullPath, sizeErr)
	}
	for _, entry := range entries {
		if entry.Name != name {
			continue
		}
		var sz int64
		if entry.Size > uint64(1<<63-1) {
			sz = 1<<63 - 1
		} else {
			sz = int64(entry.Size)
		}
		isDir := entry.Type == ftp.EntryTypeFolder
		mode := fs.FileMode(0644)
		if isDir {
			mode = 0755
		}
		return &FileInfo{Name: name, Size: sz, ModTime: entry.Time, IsDir: isDir, Mode: mode, Path: path}, nil
	}
	return nil, fmt.Errorf("failed to get FTP file info %s: %w", fullPath, sizeErr)
}

// fileModTime returns the real modification time: MDTM when the server supports it, else the entry time from the parent listing, else
// the zero time. It never invents "now" (the old behaviour made every FTP file look freshly modified).
func (c *FTPClient) fileModTime(conn *ftp.ServerConn, fullPath string) time.Time {
	if conn.IsGetTimeSupported() {
		if t, err := conn.GetTime(fullPath); err == nil {
			return t
		}
	}
	if entries, err := conn.List(pathpkg.Dir(fullPath)); err == nil {
		name := pathpkg.Base(fullPath)
		for _, e := range entries {
			if e.Name == name {
				return e.Time
			}
		}
	}
	return time.Time{}
}

// ListDirectory lists files in a directory
func (c *FTPClient) ListDirectory(ctx context.Context, path string) ([]*FileInfo, error) {
	conn, base, err := c.session()
	if err != nil {
		return nil, err
	}
	fullPath := c.resolvePath(base, path)

	entries, err := conn.List(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list FTP directory %s: %w", fullPath, err)
	}

	var files []*FileInfo
	for _, entry := range entries {
		// An MLSD server (RFC 3659) lists the directory itself ("." cdir) and its parent (".." pdir); they are not children (WF22 R16).
		if entry.Name == "." || entry.Name == ".." {
			continue
		}
		// Safe conversion: Check for overflow before converting uint64 to int64
		var size int64
		if entry.Size > uint64(1<<63-1) {
			// File size exceeds int64 max, clamp to max value
			size = 1<<63 - 1
		} else {
			size = int64(entry.Size)
		}

		files = append(files, &FileInfo{
			Name:    entry.Name,
			Size:    size,
			ModTime: entry.Time,
			IsDir:   entry.Type == ftp.EntryTypeFolder,
			Mode:    0644, // Default mode
			Path:    path + "/" + entry.Name,
		})
	}

	return files, nil
}

// FileExists checks if a file exists
func (c *FTPClient) FileExists(ctx context.Context, path string) (bool, error) {
	conn, base, err := c.session()
	if err != nil {
		return false, err
	}
	fullPath := c.resolvePath(base, path)

	// Try to get file size - if it succeeds, file exists
	_, err = conn.FileSize(fullPath)
	if err != nil {
		// Try to list the file's directory to see if it exists
		dir := pathpkg.Dir(fullPath)
		name := pathpkg.Base(fullPath)
		entries, err := conn.List(dir)
		if err != nil {
			return false, fmt.Errorf("failed to check FTP file existence %s: %w", fullPath, err)
		}
		for _, entry := range entries {
			if entry.Name == name {
				return true, nil
			}
		}
		return false, nil
	}
	return true, nil
}

// CreateDirectory creates a directory
func (c *FTPClient) CreateDirectory(ctx context.Context, path string) error {
	conn, base, err := c.session()
	if err != nil {
		return err
	}
	fullPath := c.resolvePath(base, path)
	err = conn.MakeDir(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create FTP directory %s: %w", fullPath, err)
	}
	return nil
}

// DeleteDirectory deletes a directory
func (c *FTPClient) DeleteDirectory(ctx context.Context, path string) error {
	conn, base, err := c.session()
	if err != nil {
		return err
	}
	fullPath := c.resolvePath(base, path)
	err = conn.RemoveDir(fullPath)
	if err != nil {
		return fmt.Errorf("failed to delete FTP directory %s: %w", fullPath, err)
	}
	return nil
}

// DeleteFile deletes a file
func (c *FTPClient) DeleteFile(ctx context.Context, path string) error {
	conn, base, err := c.session()
	if err != nil {
		return err
	}
	fullPath := c.resolvePath(base, path)
	err = conn.Delete(fullPath)
	if err != nil {
		return fmt.Errorf("failed to delete FTP file %s: %w", fullPath, err)
	}
	return nil
}

// ctxReader makes a copy honour its context: every Read first checks it.
type ctxReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *ctxReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// CopyFile copies a file on the FTP server. FTP has no copy command, so the source is streamed (RETR on this client's control connection)
// into a STOR on a SECOND control connection: nothing is buffered in memory or on disk, and no command is ever issued on a control
// connection while its RETR data connection is open (some servers reject that). If the server refuses a second session, the copy falls back
// to a temporary file (bounded by disk, removed on every path). A failed or unconfirmed copy removes the partial destination.
func (c *FTPClient) CopyFile(ctx context.Context, srcPath, dstPath string) error {
	conn, base, err := c.session()
	if err != nil {
		return err
	}
	srcFullPath := c.resolvePath(base, srcPath)
	dstFullPath := c.resolvePath(base, dstPath)

	wconn, _, derr := c.dial(ctx)
	if derr != nil {
		if cerr := ctx.Err(); cerr != nil {
			return cerr // cancelled: not a reason to fall back
		}
		return c.copyViaTempFile(ctx, conn, srcFullPath, dstFullPath, derr)
	}
	defer func() { _ = wconn.Quit() }()

	resp, err := conn.Retr(srcFullPath)
	if err != nil {
		return fmt.Errorf("failed to retrieve source file %s: %w", srcFullPath, err)
	}
	if dstDir := pathpkg.Dir(dstFullPath); dstDir != "." && dstDir != "/" {
		_ = wconn.MakeDir(dstDir) // ignore error if the directory exists
	}
	storErr := wconn.Stor(dstFullPath, &ctxReader{ctx: ctx, r: resp})
	closeErr := resp.Close()
	if closeErr != nil {
		// like ftpReadCloser.Close: the source session may be dead, so it is replaced and the client is not left "connected" to a corpse
		_ = c.reconnectIf(context.WithoutCancel(ctx), conn)
	}
	if storErr != nil || closeErr != nil {
		_ = wconn.Delete(dstFullPath) // best effort: no partial / unverified destination
		if storErr != nil {
			return fmt.Errorf("failed to store destination file %s: %w", dstFullPath, storErr)
		}
		return fmt.Errorf("source transfer of %s was not confirmed by the server: %w", srcFullPath, closeErr)
	}
	return nil
}

// copyViaTempFile is the fallback for servers that allow a single control session: RETR to a private temp file, close it, then MKD/STOR on
// the same connection.
func (c *FTPClient) copyViaTempFile(ctx context.Context, conn *ftp.ServerConn, srcFullPath, dstFullPath string, dialErr error) (err error) {
	f, ferr := os.CreateTemp("", "ftpcopy-*")
	if ferr != nil {
		return fmt.Errorf("failed to open a second FTP session (%v) and to create a temporary file: %w", dialErr, ferr)
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	resp, rerr := conn.Retr(srcFullPath)
	if rerr != nil {
		return fmt.Errorf("failed to retrieve source file %s: %w", srcFullPath, rerr)
	}
	_, cerr := io.Copy(f, &ctxReader{ctx: ctx, r: resp})
	closeErr := resp.Close()
	if closeErr != nil {
		_ = c.reconnectIf(context.WithoutCancel(ctx), conn)
	}
	if cerr != nil {
		return fmt.Errorf("failed to read source file %s: %w", srcFullPath, cerr)
	}
	if closeErr != nil {
		return fmt.Errorf("source transfer of %s was not confirmed by the server: %w", srcFullPath, closeErr)
	}
	if _, serr := f.Seek(0, io.SeekStart); serr != nil {
		return serr
	}
	// the control connection may have been replaced by reconnectIf only on error paths, which returned above
	if dstDir := pathpkg.Dir(dstFullPath); dstDir != "." && dstDir != "/" {
		_ = conn.MakeDir(dstDir)
	}
	if err := conn.Stor(dstFullPath, &ctxReader{ctx: ctx, r: f}); err != nil {
		_ = conn.Delete(dstFullPath)
		return fmt.Errorf("failed to store destination file %s: %w", dstFullPath, err)
	}
	return nil
}

// GetProtocol returns the protocol name
func (c *FTPClient) GetProtocol() string {
	return "ftp"
}

// GetConfig returns the FTP configuration
func (c *FTPClient) GetConfig() interface{} {
	return c.config
}
