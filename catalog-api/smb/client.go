package smb

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/hirochachacha/go-smb2"
)

// defaultDialTimeout bounds the TCP connect phase of NewSmbClient so
// unreachable SMB servers cannot block tests or shutdown for minutes.
const defaultDialTimeout = 5 * time.Second

// SmbClient represents an SMB client connection
type SmbClient struct {
	conn    net.Conn
	session *smb2.Session
	share   *smb2.Share
	config  *SmbConfig
	sub     string // sub-directory of the share this client is confined to ("" = whole share)
}

// SmbConfig contains SMB connection configuration
type SmbConfig struct {
	Host     string
	Port     int
	Share    string
	Username string
	Password string
	Domain   string
}

// NewSmbClient creates a new SMB client.
// Validates input and uses a bounded dial timeout so unreachable hosts
// fail fast instead of hanging on the OS default TCP timeout.
func NewSmbClient(config *SmbConfig) (*SmbClient, error) {
	if config == nil {
		return nil, fmt.Errorf("SMB config cannot be nil")
	}
	if config.Host == "" {
		return nil, fmt.Errorf("SMB config: Host is required")
	}
	if config.Port <= 0 || config.Port > 65535 {
		return nil, fmt.Errorf("SMB config: Port %d is out of range", config.Port)
	}
	if config.Share == "" {
		return nil, fmt.Errorf("SMB config: Share is required")
	}

	// Share may be "share" or "share/sub/dir": the share is mounted, every path is confined below the sub-directory.
	shareName, sub, err := SplitShare(config.Share)
	if err != nil {
		return nil, fmt.Errorf("SMB config: %w", err)
	}

	// Establish TCP connection with a bounded timeout.
	addr := net.JoinHostPort(config.Host, fmt.Sprintf("%d", config.Port))
	conn, err := net.DialTimeout("tcp", addr, defaultDialTimeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SMB server: %w", err)
	}

	// Create SMB session
	d := &smb2.Dialer{
		Initiator: &smb2.NTLMInitiator{
			User:     config.Username,
			Password: config.Password,
			Domain:   config.Domain,
		},
	}

	session, err := d.Dial(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create SMB session: %w", err)
	}

	// Mount share
	share, err := session.Mount(shareName)
	if err != nil {
		session.Logoff()
		conn.Close()
		return nil, fmt.Errorf("failed to mount SMB share: %w", err)
	}

	return &SmbClient{
		conn:    conn,
		session: session,
		share:   share,
		config:  config,
		sub:     sub,
	}, nil
}

// rp maps a root-relative path to the share-relative path sent to the server (confined below the sub-directory, see ResolveSub).
func (c *SmbClient) rp(p string) string { return ResolveSub(c.sub, p) }

// TestConnection tests the SMB connection
func (c *SmbClient) TestConnection() error {
	// Try to list the root directory
	_, err := c.share.ReadDir(c.rp("."))
	return err
}

// ReadFile reads a file from the SMB share
func (c *SmbClient) ReadFile(path string) (io.ReadCloser, error) {
	file, err := c.share.Open(c.rp(path))
	if err != nil {
		return nil, fmt.Errorf("failed to open SMB file %s: %w", path, err)
	}
	return file, nil
}

// WriteFile writes a file to the SMB share
func (c *SmbClient) WriteFile(path string, data io.Reader) error {
	file, err := c.share.Create(c.rp(path))
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
func (c *SmbClient) GetFileInfo(path string) (*FileInfo, error) {
	stat, err := c.share.Stat(c.rp(path))
	if err != nil {
		return nil, fmt.Errorf("failed to stat SMB file %s: %w", path, err)
	}

	return &FileInfo{
		Name:    stat.Name(),
		Size:    stat.Size(),
		ModTime: stat.ModTime(),
		IsDir:   stat.IsDir(),
		Mode:    stat.Mode(),
	}, nil
}

// ListDirectory lists files in a directory
func (c *SmbClient) ListDirectory(path string) ([]*FileInfo, error) {
	entries, err := c.share.ReadDir(c.rp(path))
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
		})
	}

	return files, nil
}

// FileExists checks if a file exists
func (c *SmbClient) FileExists(path string) (bool, error) {
	_, err := c.share.Stat(c.rp(path))
	if err != nil {
		if isNotExistError(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check SMB file existence %s: %w", path, err)
	}
	return true, nil
}

// CreateDirectory creates a directory
func (c *SmbClient) CreateDirectory(path string) error {
	err := c.share.Mkdir(c.rp(path), 0755)
	if err != nil {
		return fmt.Errorf("failed to create SMB directory %s: %w", path, err)
	}
	return nil
}

// DeleteFile deletes a file
func (c *SmbClient) DeleteFile(path string) error {
	err := c.share.Remove(c.rp(path))
	if err != nil {
		return fmt.Errorf("failed to delete SMB file %s: %w", path, err)
	}
	return nil
}

// CopyFile copies a file within the SMB share
func (c *SmbClient) CopyFile(srcPath, dstPath string) error {
	// Read source file
	srcFile, err := c.share.Open(c.rp(srcPath))
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcPath, err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := c.share.Create(c.rp(dstPath))
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

// Close closes the SMB connection
func (c *SmbClient) Close() error {
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

	if len(errs) > 0 {
		return fmt.Errorf("errors closing SMB client: %v", errs)
	}

	return nil
}

// GetConfig returns the SMB configuration
func (c *SmbClient) GetConfig() *SmbConfig {
	return c.config
}

// Helper function to check if error is "file not found"
func isNotExistError(err error) bool {
	// This is a simplified check - in practice you might want to check
	// for specific SMB error codes
	return err != nil && (err.Error() == "file does not exist" || err.Error() == "no such file or directory")
}
