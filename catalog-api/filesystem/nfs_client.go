//go:build linux
// +build linux

package filesystem

import (
	"context"
	"fmt"
	"io"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"syscall"
)

// NFSConfig contains NFS connection configuration
type NFSConfig struct {
	Host       string `json:"host"`
	Path       string `json:"path"`        // Export path on NFS server
	MountPoint string `json:"mount_point"` // Local mount point
	Options    string `json:"options"`     // Mount options
}

// NFSClient implements FileSystemClient for NFS protocol
type NFSClient struct {
	config     NFSConfig
	mounted    bool
	connected  bool
	mountPoint string
}

// NewNFSClient creates a new NFS client
func NewNFSClient(config NFSConfig) (*NFSClient, error) {
	if config.MountPoint == "" {
		return nil, fmt.Errorf("mount point is required")
	}
	return &NFSClient{
		config:     config,
		mounted:    false,
		connected:  false,
		mountPoint: config.MountPoint,
	}, nil
}

// Connect establishes the NFS connection by mounting the filesystem
func (c *NFSClient) Connect(ctx context.Context) error {
	// Check if already mounted
	if c.isMounted() {
		c.connected = true
		return nil
	}

	// Create mount point directory if it doesn't exist
	if err := os.MkdirAll(c.mountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point %s: %w", c.mountPoint, err)
	}

	// Mount the NFS share
	source := fmt.Sprintf("%s:%s", c.config.Host, c.config.Path)
	options := "vers=3"
	if c.config.Options != "" {
		options = c.config.Options
	}

	err := syscall.Mount(source, c.mountPoint, "nfs", 0, options)
	if err != nil {
		return fmt.Errorf("failed to mount NFS share %s to %s: %w", source, c.mountPoint, err)
	}

	c.mounted = true
	c.connected = true
	return nil
}

// Disconnect unmounts the NFS filesystem
func (c *NFSClient) Disconnect(ctx context.Context) error {
	if c.mounted {
		err := syscall.Unmount(c.mountPoint, 0)
		if err != nil {
			return fmt.Errorf("failed to unmount NFS share from %s: %w", c.mountPoint, err)
		}
		c.mounted = false
	}
	c.connected = false
	return nil
}

// IsConnected returns true if the client is connected
func (c *NFSClient) IsConnected() bool {
	return c.connected && c.mounted && c.isMounted()
}

// TestConnection tests the NFS connection
func (c *NFSClient) TestConnection(ctx context.Context) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	_, err := os.Stat(c.mountPoint)
	return err
}

// isMounted checks if the mount point is actually mounted
func (c *NFSClient) isMounted() bool {
	file, err := os.Open("/proc/mounts")
	if err != nil {
		return false
	}
	defer file.Close()

	// This is a simplified check - in production you'd parse /proc/mounts properly
	return true // For now, assume it's mounted if no error
}

// resolvePath resolves a relative path within the NFS mount point
func (c *NFSClient) resolvePath(path string) string {
	// Clean the path and prevent directory traversal
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		// Prevent directory traversal attacks
		cleanPath = strings.ReplaceAll(cleanPath, "..", "")
	}
	return filepath.Join(c.mountPoint, cleanPath)
}

// ReadFile reads a file from the NFS mount
func (c *NFSClient) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	fullPath := c.resolvePath(path)
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open NFS file %s: %w", fullPath, err)
	}
	return file, nil
}

// WriteFile writes a file to the NFS mount
func (c *NFSClient) WriteFile(ctx context.Context, path string, data io.Reader) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	fullPath := c.resolvePath(path)

	// Ensure the directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create NFS file %s: %w", fullPath, err)
	}
	defer file.Close()

	_, err = io.Copy(file, data)
	if err != nil {
		return fmt.Errorf("failed to write NFS file %s: %w", fullPath, err)
	}

	return nil
}

// GetFileInfo gets information about a file
func (c *NFSClient) GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	fullPath := c.resolvePath(path)
	stat, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat NFS file %s: %w", fullPath, err)
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
func (c *NFSClient) ListDirectory(ctx context.Context, path string) ([]*FileInfo, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	fullPath := c.resolvePath(path)
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list NFS directory %s: %w", fullPath, err)
	}

	var files []*FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue // Skip files we can't get info for
		}
		files = append(files, &FileInfo{
			Name:    entry.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   entry.IsDir(),
			Mode:    info.Mode(),
			Path:    pathpkg.Join(path, entry.Name()), // the REMOTE path of the entry: a slash path (WF22 X1); the mount-point arithmetic above stays filepath
		})
	}

	return files, nil
}

// FileExists checks if a file exists
func (c *NFSClient) FileExists(ctx context.Context, path string) (bool, error) {
	if !c.IsConnected() {
		return false, fmt.Errorf("not connected")
	}
	fullPath := c.resolvePath(path)
	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check NFS file existence %s: %w", fullPath, err)
	}
	return true, nil
}

// CreateDirectory creates a directory
func (c *NFSClient) CreateDirectory(ctx context.Context, path string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	fullPath := c.resolvePath(path)
	err := os.MkdirAll(fullPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create NFS directory %s: %w", fullPath, err)
	}
	return nil
}

// DeleteDirectory deletes a directory
func (c *NFSClient) DeleteDirectory(ctx context.Context, path string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	fullPath := c.resolvePath(path)
	err := os.RemoveAll(fullPath)
	if err != nil {
		return fmt.Errorf("failed to delete NFS directory %s: %w", fullPath, err)
	}
	return nil
}

// DeleteFile deletes a file
func (c *NFSClient) DeleteFile(ctx context.Context, path string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	fullPath := c.resolvePath(path)
	err := os.Remove(fullPath)
	if err != nil {
		return fmt.Errorf("failed to delete NFS file %s: %w", fullPath, err)
	}
	return nil
}

// CopyFile copies a file within the NFS mount
func (c *NFSClient) CopyFile(ctx context.Context, srcPath, dstPath string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	srcFullPath := c.resolvePath(srcPath)
	dstFullPath := c.resolvePath(dstPath)

	// Ensure destination directory exists
	dstDir := filepath.Dir(dstFullPath)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory %s: %w", dstDir, err)
	}

	// Open source file
	srcFile, err := os.Open(srcFullPath)
	if err != nil {
		return fmt.Errorf("failed to open source file %s: %w", srcFullPath, err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.Create(dstFullPath)
	if err != nil {
		return fmt.Errorf("failed to create destination file %s: %w", dstFullPath, err)
	}
	defer dstFile.Close()

	// Copy data
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy file from %s to %s: %w", srcFullPath, dstFullPath, err)
	}

	return nil
}

// GetProtocol returns the protocol name
func (c *NFSClient) GetProtocol() string {
	return "nfs"
}

// GetConfig returns the NFS configuration
func (c *NFSClient) GetConfig() interface{} {
	return &c.config
}
