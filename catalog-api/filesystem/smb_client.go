package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"strings"
	"time"

	"github.com/hirochachacha/go-smb2"
)

// SmbConfig contains SMB connection configuration
type SmbConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Share    string `json:"share"`
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
}

// NewSmbClient creates a new SMB client
func NewSmbClient(config *SmbConfig) *SmbClient {
	return &SmbClient{
		config: config,
	}
}

// Connect establishes the SMB connection
func (c *SmbClient) Connect(ctx context.Context) error {
	// Establish TCP connection with timeout
	addr := net.JoinHostPort(c.config.Host, fmt.Sprintf("%d", c.config.Port))
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
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

	session, err := d.Dial(conn)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create SMB session: %w", err)
	}

	// Mount share
	share, err := session.Mount(c.config.Share)
	if err != nil {
		session.Logoff()
		conn.Close()
		return fmt.Errorf("failed to mount SMB share: %w", err)
	}

	c.conn = conn
	c.session = session
	c.share = share
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

// TestConnection tests the SMB connection
func (c *SmbClient) TestConnection(ctx context.Context) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	// Try to list the root directory
	_, err := c.share.ReadDir(".")
	return err
}

// ReadFile reads a file from the SMB share
func (c *SmbClient) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	file, err := c.share.Open(smbPath(path))
	if err != nil {
		return nil, fmt.Errorf("failed to open SMB file %s: %w", path, err)
	}
	return file, nil
}

// OpenSeekable opens an SMB file with seek support for random access.
// The SMB2 protocol natively supports random access via smb2.File.Seek(),
// enabling HTTP Range requests for video streaming (like VLC does with libsmb2).
func (c *SmbClient) OpenSeekable(ctx context.Context, path string) (ReadSeekCloser, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	file, err := c.share.Open(smbPath(path))
	if err != nil {
		return nil, fmt.Errorf("failed to open SMB file %s: %w", path, err)
	}
	// smb2.File implements Read, Seek, and Close — it is a full ReadSeekCloser.
	return file, nil
}

// WriteFile writes a file to the SMB share
func (c *SmbClient) WriteFile(ctx context.Context, path string, data io.Reader) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	file, err := c.share.Create(smbPath(path))
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
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	stat, err := c.share.Stat(smbPath(path))
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
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	entries, err := c.share.ReadDir(smbPath(path))
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
	if !c.IsConnected() {
		return false, fmt.Errorf("not connected")
	}
	_, err := c.share.Stat(smbPath(path))
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
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	err := c.share.Mkdir(smbPath(path), 0755)
	if err != nil {
		return fmt.Errorf("failed to create SMB directory %s: %w", path, err)
	}
	return nil
}

// DeleteDirectory deletes a directory
func (c *SmbClient) DeleteDirectory(ctx context.Context, path string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	err := c.share.Remove(smbPath(path))
	if err != nil {
		return fmt.Errorf("failed to delete SMB directory %s: %w", path, err)
	}
	return nil
}

// DeleteFile deletes a file
func (c *SmbClient) DeleteFile(ctx context.Context, path string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	err := c.share.Remove(smbPath(path))
	if err != nil {
		return fmt.Errorf("failed to delete SMB file %s: %w", path, err)
	}
	return nil
}

// CopyFile copies a file within the SMB share
func (c *SmbClient) CopyFile(ctx context.Context, srcPath, dstPath string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	// Read source file
	srcFile, err := c.share.Open(smbPath(srcPath))
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcPath, err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := c.share.Create(smbPath(dstPath))
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dstPath, err)
	}
	defer dstFile.Close()

	// Copy data
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file from %s to %s: %w", srcPath, dstPath, err)
	}

	return nil
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
	if !hasDotDotSegment(p) {
		return p
	}
	rel := strings.TrimPrefix(ConfinePath(p), "/")
	if rel == "" {
		return "."
	}
	return rel
}
