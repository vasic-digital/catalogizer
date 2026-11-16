package filesystem

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"path"
	"strings"
	"time"

	"catalogizer/smb"

	"github.com/hirochachacha/go-smb2"
)

// SmbConfig contains SMB connection configuration
type SmbConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Share    string `json:"share"` // "share" or "share/sub/dir": the root is confined to the sub-directory (smb.SplitShare)
	Username string `json:"username"`
	Password string `json:"password"`
	Domain   string `json:"domain"`
}

// SmbClient implements FileSystemClient for SMB protocol
type SmbClient struct {
	conn    net.Conn
	session *smb2.Session
	share   *smb2.Share
	config  *SmbConfig
	sub     string // sub-directory of the share this client is confined to, fixed by initShare (Connect) together with the mount
}

// NewSmbClient creates a new SMB client
func NewSmbClient(config *SmbConfig) *SmbClient {
	return &SmbClient{
		config: config,
	}
}

// Connect establishes the SMB connection
func (c *SmbClient) Connect(ctx context.Context) error {
	shareName, err := c.initShare()
	if err != nil {
		return err
	}
	// Establish TCP connection with timeout
	if err := ctx.Err(); err != nil {
		return err
	}
	// SEC-DIAL-1: the operator-class dialer vets the resolved address and connects to that exact IP (never the name).
	conn, err := dialStorage(ctx, c.config.Host, fmt.Sprintf("%d", c.config.Port), 10*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to SMB server: %w", err)
	}

	// Create SMB session
	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     c.config.Username,
			Password: c.config.Password,
			Domain:   c.config.Domain,
		},
	}

	// ctx bounds the SMB negotiation and authentication (ST-D1); the session itself is not bound to it afterwards.
	session, err := d.DialContext(ctx, conn)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create SMB session: %w", err)
	}

	// Mount share
	share, err := session.WithContext(ctx).Mount(shareName)
	if err != nil {
		session.Logoff()
		conn.Close()
		return fmt.Errorf("failed to mount SMB share: %w", err)
	}

	c.conn = conn
	c.session = session
	c.share = share.WithContext(context.Background()) // the mount ran under ctx; later operations bind their own ctx
	return nil
}

// Disconnect closes the SMB connection
func (c *SmbClient) Disconnect(ctx context.Context) error {
	var errs []error

	if c.share != nil {
		if err := c.share.Umount(); err != nil {
			errs = append(errs, fmt.Errorf("failed to unmount share: %w", err))
		}
	}

	if c.session != nil {
		if err := c.session.Logoff(); err != nil {
			errs = append(errs, fmt.Errorf("failed to logoff session: %w", err))
		}
	}

	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close connection: %w", err))
		}
	}

	// The handles are dead whatever the close results were: forget them so IsConnected() is truthful and a later call returns
	// "not connected" instead of blocking forever on a closed session (found by the live SMB tests).
	c.share, c.session, c.conn = nil, nil, nil

	if len(errs) > 0 {
		return fmt.Errorf("errors closing SMB client: %v", errs)
	}

	return nil
}

// IsConnected returns true if the client is connected
func (c *SmbClient) IsConnected() bool {
	return c.share != nil && c.session != nil && c.conn != nil
}

// initShare validates the configured share path and fixes the sub-directory once, at the same moment the share name to mount is chosen, so
// the mount and the confinement can never disagree (an invalid spec fails Connect; there is no later re-parse that could fail open).
func (c *SmbClient) initShare() (shareName string, err error) {
	shareName, sub, err := smb.SplitShare(c.config.Share)
	if err != nil {
		return "", fmt.Errorf("invalid SMB share path: %w", err)
	}
	c.sub = sub
	return shareName, nil
}

// resolvePath maps a root-relative path to the share-relative path sent to the server. For a plain share name it is smbPath (unchanged
// behaviour); for "share/sub/dir" the path is confined (ConfinePath) and joined below the sub-directory captured at Connect
// (smb.ResolveSub), so no caller path can leave it.
func (c *SmbClient) resolvePath(p string) string {
	if c.sub != "" {
		return smb.ResolveSub(c.sub, ConfinePath(p))
	}
	return smbPath(p)
}

// opShare is the entry of every SMB operation: a cancelled context fails at once without network traffic (ST-D1), and the returned
// share handle is bound to ctx so a cancellation or deadline ends the request in flight.
func (c *SmbClient) opShare(ctx context.Context) (*smb2.Share, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	return c.share.WithContext(ctx), nil
}

// TestConnection tests the SMB connection
func (c *SmbClient) TestConnection(ctx context.Context) error {
	share, err := c.opShare(ctx)
	if err != nil {
		return err
	}
	// Try to list the root directory
	_, err = share.ReadDir(c.resolvePath("."))
	return err
}

// ReadFile reads a file from the SMB share
func (c *SmbClient) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	file, err := c.share.Open(c.resolvePath(path))
	if err != nil {
		return nil, fmt.Errorf("failed to open SMB file %s: %w", path, err)
	}
	return file, nil
}

// OpenSeekable opens an SMB file with seek support for random access.
// The SMB2 protocol natively supports random access via smb2.File.Seek(),
// enabling HTTP Range requests for video streaming (like VLC does with libsmb2).
func (c *SmbClient) OpenSeekable(ctx context.Context, path string) (ReadSeekCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	file, err := c.share.Open(c.resolvePath(path))
	if err != nil {
		return nil, fmt.Errorf("failed to open SMB file %s: %w", path, err)
	}
	// smb2.File implements Read, Seek, and Close — it is a full ReadSeekCloser.
	return file, nil
}

// WriteFile writes a file to the SMB share
func (c *SmbClient) WriteFile(ctx context.Context, path string, data io.Reader) error {
	share, err := c.opShare(ctx)
	if err != nil {
		return err
	}
	file, err := share.Create(c.resolvePath(path))
	if err != nil {
		return fmt.Errorf("failed to create SMB file %s: %w", path, err)
	}
	defer file.Close()

	_, err = io.Copy(file, data)
	if err != nil {
		return fmt.Errorf("failed to write SMB file %s: %w", path, err)
	}

	return nil
}

// GetFileInfo gets information about a file
func (c *SmbClient) GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	share, err := c.opShare(ctx)
	if err != nil {
		return nil, err
	}
	stat, err := share.Stat(c.resolvePath(path))
	if err != nil {
		return nil, fmt.Errorf("failed to stat SMB file %s: %w", path, err)
	}

	return &FileInfo{
		Name:    stat.Name(),
		Size:    stat.Size(),
		ModTime: stat.ModTime(),
		IsDir:   stat.IsDir(),
		Mode:    stat.Mode(),
		Path:    path,
	}, nil
}

// ListDirectory lists files in a directory
func (c *SmbClient) ListDirectory(ctx context.Context, path string) ([]*FileInfo, error) {
	share, err := c.opShare(ctx)
	if err != nil {
		return nil, err
	}
	entries, err := share.ReadDir(c.resolvePath(path))
	if err != nil {
		return nil, fmt.Errorf("failed to list SMB directory %s: %w", path, err)
	}

	var files []*FileInfo
	for _, entry := range entries {
		files = append(files, &FileInfo{
			Name:    entry.Name(),
			Size:    entry.Size(),
			ModTime: entry.ModTime(),
			IsDir:   entry.IsDir(),
			Mode:    entry.Mode(),
			Path:    path + "/" + entry.Name(),
		})
	}

	return files, nil
}

// FileExists checks if a file exists
func (c *SmbClient) FileExists(ctx context.Context, path string) (bool, error) {
	share, err := c.opShare(ctx)
	if err != nil {
		return false, err
	}
	_, err = share.Stat(c.resolvePath(path))
	if err != nil {
		if isNotExistError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check SMB file existence %s: %w", path, err)
	}
	return true, nil
}

// CreateDirectory creates a directory
func (c *SmbClient) CreateDirectory(ctx context.Context, path string) error {
	share, err := c.opShare(ctx)
	if err != nil {
		return err
	}
	err = share.Mkdir(c.resolvePath(path), 0755)
	if err != nil {
		return fmt.Errorf("failed to create SMB directory %s: %w", path, err)
	}
	return nil
}

// DeleteDirectory deletes a directory
func (c *SmbClient) DeleteDirectory(ctx context.Context, path string) error {
	share, err := c.opShare(ctx)
	if err != nil {
		return err
	}
	err = share.Remove(c.resolvePath(path))
	if err != nil {
		return fmt.Errorf("failed to delete SMB directory %s: %w", path, err)
	}
	return nil
}

// DeleteFile deletes a file
func (c *SmbClient) DeleteFile(ctx context.Context, path string) error {
	share, err := c.opShare(ctx)
	if err != nil {
		return err
	}
	err = share.Remove(c.resolvePath(path))
	if err != nil {
		return fmt.Errorf("failed to delete SMB file %s: %w", path, err)
	}
	return nil
}

// copyFile is the file surface the copy path needs; *smb2.File satisfies it. copyFS is the share surface. They are the injectable seam that
// lets hermetic tests drive the real copy logic with failing writers/closers (CAT-SMB-COPY-1 review: mutants R4/R6/R7/M5).
type copyFile interface {
	io.ReadWriteCloser
	Stat() (fs.FileInfo, error)
}

type copyFS interface {
	Open(name string) (copyFile, error)
	Create(name string) (copyFile, error)
	Stat(name string) (fs.FileInfo, error)
	Remove(name string) error
	Rename(oldpath, newpath string) error
}

type smbCopyShare struct{ s *smb2.Share }

func (a smbCopyShare) Open(n string) (copyFile, error) {
	f, err := a.s.Open(n)
	if err != nil {
		return nil, err
	}
	return f, nil
}
func (a smbCopyShare) Create(n string) (copyFile, error) {
	f, err := a.s.Create(n)
	if err != nil {
		return nil, err
	}
	return f, nil
}
func (a smbCopyShare) Stat(n string) (fs.FileInfo, error) { return a.s.Stat(n) }
func (a smbCopyShare) Remove(n string) error              { return a.s.Remove(n) }
func (a smbCopyShare) Rename(o, n string) error           { return a.s.Rename(o, n) }

// copyCleanupBudget bounds the temp-file cleanup, which runs on a fresh context so it still works after the caller's deadline expired.
const copyCleanupBudget = 10 * time.Second

// ErrCopyDestReadOnly is returned when the destination exists and is read-only. go-smb2's Share.Remove would clear the read-only attribute
// and delete it, so a copy would silently overwrite a file the owner marked read-only; it is refused instead (SMBCOPYR-m1).
var ErrCopyDestReadOnly = errors.New("copy destination is read-only")

// CopyFile copies a file within the SMB share (CAT-SMB-COPY-1). The old code created the destination, which truncates it, before one byte of
// the source was read: a copy onto the same file emptied the source and reported success. Now: the same cleaned resolved path is refused
// (ErrCopySameFile); a source that is not a regular file, a destination that is a directory and a read-only destination are refused with nothing
// written; the data is written to a unique temp file beside the destination and only a complete copy replaces the destination, so a failed or
// cancelled copy never truncates or deletes a pre-existing destination. Limits (SMB2 rename cannot replace, ReplaceIfExists is 0 in go-smb2):
// replacing an existing destination is remove + rename, so a crash between the two leaves the finished copy under the temp name and no
// destination. Temp files are named ".copy-<16 hex>.tmp" beside the destination: they are visible to a directory scan while a copy runs (or
// after a crash) and scans may skip that pattern. Destination identity (ACL, times, file id, hard links) is not preserved: the destination is a
// new file. A different spelling of the same file (case alias on a case-folding server) is not detected by name; the temp-first order keeps
// its bytes (the alias is replaced by the full copy of itself).
func (c *SmbClient) CopyFile(ctx context.Context, srcPath, dstPath string) error {
	if _, err := c.opShare(ctx); err != nil {
		return err
	}
	return copyFileOn(ctx, func(cx context.Context) copyFS { return smbCopyShare{c.share.WithContext(cx)} },
		srcPath, dstPath, c.resolvePath(srcPath), c.resolvePath(dstPath))
}

func copyFileOn(ctx context.Context, newFS func(context.Context) copyFS, srcPath, dstPath, srcR, dstR string) error {
	// The data transfer runs on a share bound to a context that does NOT end with the caller's cancel: a request cut off in flight leaves
	// the temp's handle open on the server, and the temp could then be neither closed nor removed. The caller's cancel and deadline are
	// honoured between chunks instead (copyChunked), so a cancelled copy stops, closes its handles and removes the temp (live test, mid-transfer).
	tctx, tcancel := context.WithCancel(context.WithoutCancel(ctx))
	defer tcancel()
	if dl, ok := ctx.Deadline(); ok {
		var dcancel context.CancelFunc
		tctx, dcancel = context.WithTimeout(context.WithoutCancel(ctx), time.Until(dl)+15*time.Second)
		defer dcancel()
	}
	share := newFS(tctx)
	// cleanup runs on a fresh bounded context: the transfer context may be past its deadline exactly when cleanup is needed (SMBCOPYR-m3).
	removeTemp := func(name string) {
		cctx, ccancel := context.WithTimeout(context.WithoutCancel(ctx), copyCleanupBudget)
		defer ccancel()
		_ = newFS(cctx).Remove(name)
	}
	if path.Clean(srcR) == path.Clean(dstR) {
		return fmt.Errorf("%w: %s and %s", ErrCopySameFile, srcPath, dstPath)
	}
	srcFile, err := share.Open(srcR)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcPath, err)
	}
	defer srcFile.Close()
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat source file %s: %w", srcPath, err)
	}
	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("%w: %s", ErrCopySourceNotRegular, srcPath)
	}
	dstInfo, dstErr := share.Stat(dstR)
	dstExists := dstErr == nil
	if dstExists && dstInfo.IsDir() {
		return fmt.Errorf("%w: %s", ErrCopyDestIsDirectory, dstPath)
	}
	if dstExists && dstInfo.Mode().Perm()&0o222 == 0 {
		return fmt.Errorf("%w: %s", ErrCopyDestReadOnly, dstPath)
	}
	if dstErr != nil && !isNotExistError(dstErr) {
		return fmt.Errorf("failed to stat destination %s: %w", dstPath, dstErr)
	}

	var rnd [8]byte
	if _, err := rand.Read(rnd[:]); err != nil {
		return fmt.Errorf("failed to name the temp file: %w", err)
	}
	tmpR := path.Join(path.Dir(dstR), ".copy-"+hex.EncodeToString(rnd[:])+".tmp")
	tmp, err := share.Create(tmpR)
	if err != nil {
		return fmt.Errorf("failed to create temp file for %s: %w", dstPath, err)
	}
	err = copyChunked(ctx, tmp, srcFile)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		removeTemp(tmpR)
		return fmt.Errorf("failed to copy file from %s to %s: %w", srcPath, dstPath, err)
	}
	if cerr := srcFile.Close(); cerr != nil { // a case-folded alias of the source may be the destination: release the handle before it is replaced
		removeTemp(tmpR)
		return fmt.Errorf("failed to close source file %s: %w", srcPath, cerr)
	}
	if dstExists {
		if err := share.Remove(dstR); err != nil {
			removeTemp(tmpR)
			return fmt.Errorf("failed to replace destination %s: %w", dstPath, err)
		}
	}
	if err := share.Rename(tmpR, dstR); err != nil {
		return fmt.Errorf("failed to move the finished copy into place for %s (the copy is kept as %s beside it): %w", dstPath, path.Base(tmpR), err)
	}
	return nil
}

// copyChunked copies src to dst in copyChunk steps and checks ctx before every step and once more after EOF, so a cancel or deadline stops the
// transfer between two requests without cutting a request off in flight, and a cancel that arrives with the last chunk still fails the copy.
func copyChunked(ctx context.Context, dst io.Writer, src io.Reader) error {
	buf := make([]byte, copyChunk)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, rerr := src.Read(buf)
		if n > 0 {
			if _, werr := dst.Write(buf[:n]); werr != nil {
				return werr
			}
		}
		if rerr == io.EOF {
			return ctx.Err()
		}
		if rerr != nil {
			return rerr
		}
	}
}

// GetProtocol returns the protocol name
func (c *SmbClient) GetProtocol() string {
	return "smb"
}

// GetConfig returns the SMB configuration
func (c *SmbClient) GetConfig() interface{} {
	return c.config
}

// isNotExistError reports whether err means "the path does not exist". go-smb2 returns *fs.PathError values whose text carries
// the operation and path ("stat X: file does not exist"), so the error chain is inspected, not just its text.
func isNotExistError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, fs.ErrNotExist) {
		return true
	}
	return err.Error() == "file does not exist" || err.Error() == "no such file or directory"
}

// smbPath applies the shared containment rule (ConfinePath) to a share-relative path: a path with a ".." SEGMENT is confined to the share root
// and returned share-relative; every other path is handed to the share exactly as given (WF22 N2 class A).
func smbPath(p string) string {
	// S1: a backslash is a path separator on the wire and cannot occur inside an SMB name, so a `..` between backslashes is a traversal
	// segment exactly like one between slashes (go-smb2 maps / to \ and resolves nothing itself). SMB-only: hasDotDotSegment is shared
	// with FTP/WebDAV, where a backslash is an ordinary filename character.
	if norm := strings.Map(func(r rune) rune {
		if r == '\\' {
			return '/'
		}
		return r
	}, p); hasDotDotSegment(norm) {
		p = norm
	}
	if !hasDotDotSegment(p) {
		// ST-D3: go-smb2 rejects a path that starts with a separator ("leading '\\' is not allowed"), so no caller can have a working
		// "/x" today; accept it like the FTP and WebDAV clients do by reading it as share-relative. Paths that do not start with "/"
		// (including "" and "movies/a") are handed over exactly as before.
		if strings.HasPrefix(p, "/") {
			if rel := strings.TrimLeft(p, "/"); rel != "" {
				return rel
			}
			return "."
		}
		return p
	}
	rel := strings.TrimPrefix(ConfinePath(p), "/")
	if rel == "" {
		return "."
	}
	return rel
}
