package filesystem

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/textproto"
	"os"
	pathpkg "path"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/jlaffaye/ftp"
)

// FTPConfig contains FTP connection configuration
type FTPConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Path     string `json:"path"` // Base path on the FTP server
	// SpoolDir is where CopyFile keeps its temporary file when the server allows only one control session. Empty = os.TempDir().
	SpoolDir string `json:"spool_dir,omitempty"`
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
func (c *FTPClient) dial(ctx context.Context, extra ...ftp.DialOption) (*ftp.ServerConn, string, error) {
	addr := net.JoinHostPort(c.config.Host, fmt.Sprintf("%d", c.config.Port))

	opts := append([]ftp.DialOption{ftp.DialWithTimeout(30 * time.Second), ftp.DialWithContext(ctx)}, extra...)
	client, err := ftp.Dial(addr, opts...)
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

// ctxReader makes a copy honour its context between reads: every Read first checks it. A Read that is already blocked inside the
// network is unblocked separately (see watchCancel).
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

// connTracker remembers every network connection a dial function created (control and data connections of one ServerConn), so a
// cancelled copy can close them and unblock a Read / Write stalled inside the network.
type connTracker struct {
	mu     sync.Mutex
	conns  []net.Conn
	closed bool
}

func (t *connTracker) dialFunc(ctx context.Context) func(network, address string) (net.Conn, error) {
	d := net.Dialer{Timeout: 30 * time.Second}
	return func(network, address string) (net.Conn, error) {
		conn, err := d.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		t.mu.Lock()
		defer t.mu.Unlock()
		if t.closed {
			_ = conn.Close()
			return nil, context.Canceled
		}
		t.conns = append(t.conns, conn)
		return conn, nil
	}
}

func (t *connTracker) closeAll() {
	t.mu.Lock()
	t.closed = true
	conns := t.conns
	t.conns = nil
	t.mu.Unlock()
	for _, c := range conns {
		_ = c.Close()
	}
}

// watchCancel makes a cancelled ctx interrupt a transfer that is blocked in the network: the source data connection gets an expired
// deadline (ftp.Response.SetDeadline) and, when the destination connection was dialled with a connTracker, all of its connections are
// closed. The returned function stops the watcher.
//
// What it cannot do (documented limit): on the single-session fallback the destination STOR runs on the client's own control connection,
// which is not tracked, so a STOR write stalled inside the network is only interrupted when the server or the OS ends it.
func watchCancel(ctx context.Context, resp *ftp.Response, tr *connTracker) func() bool {
	return context.AfterFunc(ctx, func() {
		if resp != nil {
			_ = resp.SetDeadline(time.Now())
		}
		if tr != nil {
			tr.closeAll()
		}
	})
}

// tempSibling returns a unique name in the destination's directory. The copy is stored under this name and only renamed over the
// destination after the whole transfer is confirmed, so the destination is never partially written and a failed copy never touches it.
func tempSibling(dst string) string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		binary.BigEndian.PutUint64(b[:], uint64(time.Now().UnixNano()))
	}
	// the temp name adds 26 bytes to the base name; servers cap a name at NAME_MAX (255 on most file systems), so a long base is cut
	// (at a rune boundary) to keep the whole temp name well below that. The destination keeps its own full name.
	base := pathpkg.Base(dst)
	if len(base) > maxTempBase {
		cut := maxTempBase
		for cut > 0 && !utf8.RuneStart(base[cut]) {
			cut--
		}
		base = base[:cut]
	}
	return pathpkg.Join(pathpkg.Dir(dst), "."+base+".ftpcopy-"+hex.EncodeToString(b[:])+".part")
}

// maxTempBase is the longest part of the destination's base name that is kept in the temp name (200 + 26 added bytes < 255).
const maxTempBase = 200

// storeTemp STORs r under a fresh temp name next to dst. On failure the temp (and only the temp: this copy created nothing else) is
// removed again; if that removal fails the error names the temp that was left behind.
func (c *FTPClient) storeTemp(ctx context.Context, w *ftp.ServerConn, dstFull string, r io.Reader) (string, error) {
	tmp := tempSibling(dstFull)
	if err := w.Stor(tmp, &ctxReader{ctx: ctx, r: r}); err != nil {
		err = fmt.Errorf("failed to store %s (as %s): %w", dstFull, tmp, err)
		if derr := c.discardTemp(w, tmp); derr != nil {
			err = fmt.Errorf("%w; %v", err, derr)
		}
		return "", err
	}
	return tmp, nil
}

// tempNeverCreated reports whether a DELE error says the file did not exist (550 "no such file" and friends), i.e. there is nothing to
// clean up. Any other answer (421, a 550 "permission denied", ...) says nothing about whether the temp exists.
func tempNeverCreated(err error) bool {
	var te *textproto.Error
	if !errors.As(err, &te) || te.Code != 550 {
		return false
	}
	m := strings.ToLower(te.Msg)
	return strings.Contains(m, "no such") || strings.Contains(m, "not found") || strings.Contains(m, "not exist") || strings.Contains(m, "does not exist")
}

// discardTemp removes the temp file this copy created. If the connection it was written on is gone (a cancelled copy), a short-lived
// fresh session is tried once. A pre-existing destination is never deleted here. The returned error is non-nil exactly when the temp may
// still exist on the server; it names the temp so an orphan is never silent (it is also logged).
func (c *FTPClient) discardTemp(w *ftp.ServerConn, tmp string) error {
	var last error
	if w != nil {
		derr := w.Delete(tmp)
		if derr == nil || tempNeverCreated(derr) {
			return nil
		}
		last = derr
		var te *textproto.Error
		if errors.As(derr, &te) && te.Code >= 500 {
			// a permanent refusal (e.g. permission denied): a fresh session would be refused too
			return orphanErr(tmp, last)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	fresh, _, err := c.dial(ctx)
	if err != nil {
		if last == nil {
			last = err
		}
		return orphanErr(tmp, fmt.Errorf("%v (cleanup session: %v)", last, err))
	}
	defer func() { _ = fresh.Quit() }()
	if derr := fresh.Delete(tmp); derr != nil && !tempNeverCreated(derr) {
		return orphanErr(tmp, derr)
	}
	return nil
}

func orphanErr(tmp string, cause error) error {
	log.Printf("ftp: temporary file %s may remain on the server: %v", tmp, cause)
	return fmt.Errorf("temporary file %s may remain on the server (cleanup failed: %v)", tmp, cause)
}

// commitTemp renames the confirmed temp over dst. If the server refuses (e.g. rename onto an existing file is not allowed) the temp is
// removed and the destination is left exactly as it was: that is reported, never "fixed" by deleting the destination. If the connection
// fails during the rename the outcome is unknown and is reported as such (the rename may or may not have been applied).
func (c *FTPClient) commitTemp(w *ftp.ServerConn, tmp, dstFull string) error {
	if err := w.Rename(tmp, dstFull); err != nil {
		derr := c.discardTemp(w, tmp)
		var te *textproto.Error
		var out error
		if errors.As(err, &te) {
			out = fmt.Errorf("server refused to move the finished copy onto %s (existing destination, if any, left unchanged): %w", dstFull, err)
		} else {
			out = fmt.Errorf("connection failed while moving the finished copy onto %s: the rename may or may not have been applied: %w", dstFull, err)
		}
		if derr != nil {
			out = fmt.Errorf("%w; %v", out, derr)
		}
		return out
	}
	return nil
}

// closeResp closes a RETR response, i.e. waits for the server's final reply on the control connection. That read has no deadline of its
// own: a server that never answers would block it forever even after ctx is cancelled. So on cancel it stops waiting and returns
// ctx.Err(); the caller then replaces the session (reconnectIf closes the old control connection, which also ends the abandoned read).
func (c *FTPClient) closeResp(ctx context.Context, resp *ftp.Response) error {
	done := make(chan error, 1)
	go func() { done <- resp.Close() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// spoolDir is where the single-session fallback keeps its temporary file: FTPConfig.SpoolDir, default os.TempDir().
func (c *FTPClient) spoolDir() string {
	if c.config != nil && c.config.SpoolDir != "" {
		return c.config.SpoolDir
	}
	return os.TempDir()
}

// freeSpaceFn reports the bytes available to this process in dir (replaceable in tests). An error means "unknown".
var freeSpaceFn = freeSpace

// CopyFile copies a file on the FTP server. FTP has no copy command, so the source is streamed (RETR on this client's control connection)
// into a STOR on a SECOND control connection: nothing is buffered in memory or on disk, and no command is ever issued on a control
// connection while its RETR data connection is open (some servers reject that). If the server refuses a second session, the copy falls back
// to a temporary file in SpoolDir (bounded by disk, refused up front when the source SIZE clearly exceeds the free space, removed on every
// path).
//
// The data is stored under a unique temporary name beside the destination and renamed into place only after the source transfer is
// confirmed, so a failed, cancelled or refused copy never deletes or truncates a pre-existing destination; only the temp this copy
// created is removed. Cancelling ctx interrupts a transfer that is blocked in the network (see watchCancel for the one documented limit).
func (c *FTPClient) CopyFile(ctx context.Context, srcPath, dstPath string) error {
	conn, base, err := c.session()
	if err != nil {
		return err
	}
	srcFullPath := c.resolvePath(base, srcPath)
	dstFullPath := c.resolvePath(base, dstPath)

	tr := &connTracker{}
	wconn, _, derr := c.dial(ctx, ftp.DialWithDialFunc(tr.dialFunc(ctx)))
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
	stopWatch := watchCancel(ctx, resp, tr)
	defer stopWatch()
	if dstDir := pathpkg.Dir(dstFullPath); dstDir != "." && dstDir != "/" {
		_ = wconn.MakeDir(dstDir) // ignore error if the directory exists
	}
	tmp, storErr := c.storeTemp(ctx, wconn, dstFullPath, resp)
	closeErr := c.closeResp(ctx, resp)
	if closeErr != nil {
		// like ftpReadCloser.Close: the source session may be dead or mid-transfer, so it is replaced and the client is not left "connected" to a corpse
		_ = c.reconnectIf(context.WithoutCancel(ctx), conn)
	}
	if cerr := ctx.Err(); cerr != nil && (storErr != nil || closeErr != nil) {
		err := fmt.Errorf("copy of %s cancelled: %w", srcFullPath, cerr)
		if storErr == nil {
			if derr := c.discardTemp(wconn, tmp); derr != nil {
				err = fmt.Errorf("%w; %v", err, derr)
			}
		}
		return err
	}
	if storErr != nil {
		return storErr
	}
	if closeErr != nil {
		err := fmt.Errorf("source transfer of %s was not confirmed by the server: %w", srcFullPath, closeErr)
		if derr := c.discardTemp(wconn, tmp); derr != nil { // unconfirmed source transfer: nothing is moved into place
			err = fmt.Errorf("%w; %v", err, derr)
		}
		return err
	}
	// the transfer is confirmed: disarm the cancel watcher so a late cancel cannot close the destination connection in the middle of the
	// rename. If it already fired the connections are closed, so the copy is abandoned (cancelled) instead of committed.
	if !stopWatch() {
		err := fmt.Errorf("copy of %s cancelled before it was moved into place: %w", srcFullPath, context.Cause(ctx))
		if derr := c.discardTemp(wconn, tmp); derr != nil {
			err = fmt.Errorf("%w; %v", err, derr)
		}
		return err
	}
	return c.commitTemp(wconn, tmp, dstFullPath)
}

// copyViaTempFile is the fallback for servers that allow a single control session: RETR to a private temp file in SpoolDir, close it, then
// MKD/STOR (to a temp name, then rename) on the same connection.
func (c *FTPClient) copyViaTempFile(ctx context.Context, conn *ftp.ServerConn, srcFullPath, dstFullPath string, dialErr error) (err error) {
	dir := c.spoolDir()
	if size, serr := conn.FileSize(srcFullPath); serr == nil && size > 0 {
		if free, ferr := freeSpaceFn(dir); ferr == nil && free < uint64(size) {
			return fmt.Errorf("cannot copy %s: the server allows no second session (%v) and the spool directory %s has %d bytes free for %d bytes", srcFullPath, dialErr, dir, free, size)
		}
	}
	f, ferr := os.CreateTemp(dir, "ftpcopy-*")
	if ferr != nil {
		return fmt.Errorf("failed to open a second FTP session (%v) and to create a temporary file in %s: %w", dialErr, dir, ferr)
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()
	resp, rerr := conn.Retr(srcFullPath)
	if rerr != nil {
		return fmt.Errorf("failed to retrieve source file %s: %w", srcFullPath, rerr)
	}
	stopWatch := watchCancel(ctx, resp, nil)
	defer stopWatch()
	_, cerr := io.Copy(f, &ctxReader{ctx: ctx, r: resp})
	closeErr := c.closeResp(ctx, resp)
	if closeErr != nil {
		_ = c.reconnectIf(context.WithoutCancel(ctx), conn)
	}
	if cerr != nil {
		if xerr := ctx.Err(); xerr != nil {
			return fmt.Errorf("copy of %s cancelled: %w", srcFullPath, xerr)
		}
		return fmt.Errorf("failed to read source file %s: %w", srcFullPath, cerr)
	}
	if closeErr != nil {
		return fmt.Errorf("source transfer of %s was not confirmed by the server: %w", srcFullPath, closeErr)
	}
	if _, serr := f.Seek(0, io.SeekStart); serr != nil {
		return serr
	}
	// the source session was replaced only on error paths, which returned above, so conn is still the live control connection
	if dstDir := pathpkg.Dir(dstFullPath); dstDir != "." && dstDir != "/" {
		_ = conn.MakeDir(dstDir)
	}
	tmp, serr := c.storeTemp(ctx, conn, dstFullPath, f)
	if serr != nil {
		if xerr := ctx.Err(); xerr != nil {
			return fmt.Errorf("copy of %s cancelled: %w", srcFullPath, xerr)
		}
		return serr
	}
	return c.commitTemp(conn, tmp, dstFullPath)
}

// GetProtocol returns the protocol name
func (c *FTPClient) GetProtocol() string {
	return "ftp"
}

// GetConfig returns the FTP configuration
func (c *FTPClient) GetConfig() interface{} {
	return c.config
}
