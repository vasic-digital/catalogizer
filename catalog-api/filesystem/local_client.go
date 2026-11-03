package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalConfig contains local filesystem configuration
type LocalConfig struct {
	BasePath string `json:"base_path"` // Base directory path
}

// LocalClient implements FileSystemClient for local filesystem
type LocalClient struct {
	config    *LocalConfig
	basePath  string
	connected bool
}

// NewLocalClient creates a new local filesystem client
func NewLocalClient(config *LocalConfig) *LocalClient {
	return &LocalClient{
		config:    config,
		basePath:  config.BasePath,
		connected: false,
	}
}

// Connect establishes the connection (for local filesystem, this just validates the path)
func (c *LocalClient) Connect(ctx context.Context) error {
	// Validate that the base path exists and is accessible
	info, err := os.Stat(c.basePath)
	if err != nil {
		return fmt.Errorf("failed to access base path %s: %w", c.basePath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("base path %s is not a directory", c.basePath)
	}
	c.connected = true
	return nil
}

// Disconnect closes the connection (no-op for local filesystem)
func (c *LocalClient) Disconnect(ctx context.Context) error {
	c.connected = false
	return nil
}

// IsConnected returns true if the client is connected
func (c *LocalClient) IsConnected() bool {
	return c.connected
}

// TestConnection tests the connection
func (c *LocalClient) TestConnection(ctx context.Context) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	_, err := os.Stat(c.basePath)
	return err
}

// resolvePath resolves a relative path to an absolute path within the base directory. The confinement rule is ConfinePath (shared with every
// protocol client): a ".." segment cannot climb out of the base, and a name that merely contains ".." is kept (WF22 N7).
func (c *LocalClient) resolvePath(path string) string {
	return filepath.Join(c.basePath, filepath.FromSlash(ConfinePath(filepath.ToSlash(path))))
}

// ErrPathEscapesRoot is returned when a path, after symbolic links are resolved, lies outside the storage root.
// ErrRefuseDeleteRoot is returned when a delete would remove the storage root itself.
var ErrRefuseDeleteRoot = errors.New("refusing to delete the storage root")

var ErrPathEscapesRoot = errors.New("path resolves outside the storage root")

// resolveConfined is resolvePath plus the symlink check: ConfinePath is lexical, so a link inside the root that points outside
// would otherwise be followed. The deepest existing ancestor of the target is resolved with EvalSymlinks and must stay inside the
// resolved base directory (a path that does not exist yet is judged by its existing parent, so WriteFile/CreateDirectory still work).
func (c *LocalClient) resolveConfined(path string) (string, error) {
	full := c.resolvePath(path)
	base, err := filepath.EvalSymlinks(c.basePath)
	if err != nil {
		return "", fmt.Errorf("resolve base path %s: %w", c.basePath, err)
	}
	probe := full
	for {
		real, err := filepath.EvalSymlinks(probe)
		if err == nil {
			if !withinDir(base, real) {
				return "", fmt.Errorf("%w: %s", ErrPathEscapesRoot, path)
			}
			return full, nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return full, nil
		}
		probe = parent
	}
}

// withinDir reports whether target is dir or lies below it (both already symlink-resolved).
func withinDir(dir, target string) bool {
	rel, err := filepath.Rel(dir, target)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// ReadFile reads a file from the local filesystem
func (c *LocalClient) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return nil, rerr
	}
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open local file %s: %w", fullPath, err)
	}
	return file, nil
}

// OpenSeekable opens a local file with seek support for random access.
// os.File natively supports Seek, enabling HTTP Range requests for video streaming.
func (c *LocalClient) OpenSeekable(ctx context.Context, path string) (ReadSeekCloser, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return nil, rerr
	}
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open local file %s: %w", fullPath, err)
	}
	// os.File implements Read, Seek, and Close.
	return file, nil
}

// WriteFile writes a file to the local filesystem
func (c *LocalClient) WriteFile(ctx context.Context, path string, data io.Reader) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return rerr
	}

	// Ensure the directory exists
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create local file %s: %w", fullPath, err)
	}
	defer file.Close()

	_, err = io.Copy(file, data)
	if err != nil {
		return fmt.Errorf("failed to write local file %s: %w", fullPath, err)
	}

	return nil
}

// GetFileInfo gets information about a file
func (c *LocalClient) GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return nil, rerr
	}
	stat, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat local file %s: %w", fullPath, err)
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
func (c *LocalClient) ListDirectory(ctx context.Context, path string) ([]*FileInfo, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return nil, rerr
	}
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to list local directory %s: %w", fullPath, err)
	}

	limit, limited := ListLimit(ctx)
	var files []*FileInfo
	for _, entry := range entries {
		if limited && len(files) >= limit {
			break
		}
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
			Path:    filepath.Join(path, entry.Name()),
		})
	}

	return files, nil
}

// FileExists checks if a file exists
func (c *LocalClient) FileExists(ctx context.Context, path string) (bool, error) {
	if !c.IsConnected() {
		return false, fmt.Errorf("not connected")
	}
	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return false, rerr
	}
	_, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check local file existence %s: %w", fullPath, err)
	}
	return true, nil
}

// CreateDirectory creates a directory
func (c *LocalClient) CreateDirectory(ctx context.Context, path string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return rerr
	}
	err := os.MkdirAll(fullPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create local directory %s: %w", fullPath, err)
	}
	return nil
}

// DeleteDirectory deletes a directory
func (c *LocalClient) DeleteDirectory(ctx context.Context, path string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return rerr
	}
	if baseReal, berr := filepath.EvalSymlinks(c.basePath); berr == nil {
		if real, rerr := filepath.EvalSymlinks(fullPath); rerr == nil && filepath.Clean(real) == filepath.Clean(baseReal) {
			return ErrRefuseDeleteRoot
		}
	}
	if filepath.Clean(fullPath) == filepath.Clean(c.basePath) {
		return ErrRefuseDeleteRoot
	}
	err := os.RemoveAll(fullPath)
	if err != nil {
		return fmt.Errorf("failed to delete local directory %s: %w", fullPath, err)
	}
	return nil
}

// DeleteFile deletes a file
func (c *LocalClient) DeleteFile(ctx context.Context, path string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return rerr
	}
	err := os.Remove(fullPath)
	if err != nil {
		return fmt.Errorf("failed to delete local file %s: %w", fullPath, err)
	}
	return nil
}

// CopyFile copies a file within the local filesystem
func (c *LocalClient) CopyFile(ctx context.Context, srcPath, dstPath string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	srcFullPath, rerr := c.resolveConfined(srcPath)
	if rerr != nil {
		return rerr
	}
	dstFullPath, rerr := c.resolveConfined(dstPath)
	if rerr != nil {
		return rerr
	}

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
func (c *LocalClient) GetProtocol() string {
	return "local"
}

// GetConfig returns the local configuration
func (c *LocalClient) GetConfig() interface{} {
	return c.config
}
