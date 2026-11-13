//go:build darwin
// +build darwin

package filesystem

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// NFSConfig contains NFS connection configuration
type NFSConfig struct {
	Host       string `json:"host"`
	Path       string `json:"path"`        // Export path on NFS server
	MountPoint string `json:"mount_point"` // Local mount point
	Options    string `json:"options"`     // Mount options
	// AllowSymlinkEscape switches the symlink confinement OFF for this root (settings key allow_symlink_escape). Default false: a symbolic
	// link on the mount that resolves outside the mount point is refused with ErrPathEscapesRoot (CAT-FS-SYMLINK-1).
	AllowSymlinkEscape bool `json:"allow_symlink_escape"`
}

// NFSClient for macOS using mount command and basic file operations
type NFSClient struct {
	config     NFSConfig
	mountPoint string
	connected  bool
	mounted    bool
}

func NewNFSClient(config NFSConfig) (*NFSClient, error) {
	if config.MountPoint == "" {
		return nil, fmt.Errorf("mount point is required")
	}

	// Create mount point directory if it doesn't exist
	if err := os.MkdirAll(config.MountPoint, 0755); err != nil {
		return nil, fmt.Errorf("failed to create mount point: %w", err)
	}

	return &NFSClient{
		config:     config,
		mountPoint: config.MountPoint,
	}, nil
}

func (c *NFSClient) Connect(ctx context.Context) error {
	// Check if already mounted
	if c.mounted && c.connected {
		return nil
	}

	// Build mount command for macOS
	options := c.config.Options
	if options == "" {
		options = "resvport,soft,intr,tcp"
	}

	// SEC-DIAL-1: syntax-check host / export / options and mount the VETTED address, never the name.
	if err := ValidateStorageHostSyntax(c.config.Host); err != nil {
		return fmt.Errorf("%w", ErrStorageHostRefused)
	}
	if err := ValidateNFSExport(c.config.Path); err != nil {
		return err
	}
	if _, err := ParseNFSOptions(options); err != nil {
		return err
	}
	srvIP, err := resolveNFSServer(ctx, c.config.Host)
	if err != nil {
		return err
	}
	srv := srvIP.String()
	if strings.Contains(srv, ":") {
		srv = "[" + srv + "]"
	}
	source := srv + ":" + c.config.Path

	// Create mount command
	args := []string{
		"-t", "nfs",
		"-o", options,
		source,
		c.mountPoint,
	}

	cmd := exec.CommandContext(ctx, "mount", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// the mount output names the vetted address; it is not shown to callers that are not administrators
		return fmt.Errorf("NFS mount failed: %w, output: %s", err, scrubResolvedAddrs(string(output), c.config.Host))
	}

	// Verify mount is active
	time.Sleep(100 * time.Millisecond) // Small delay for mount to settle

	if _, err := os.Stat(c.mountPoint); err != nil {
		return fmt.Errorf("mount point not accessible after mount: %w", err)
	}

	c.mounted = true
	c.connected = true

	return nil
}

func (c *NFSClient) Disconnect(ctx context.Context) error {
	if !c.mounted {
		return nil
	}

	// Unmount using system command
	cmd := exec.CommandContext(ctx, "umount", c.mountPoint)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Try forced unmount if normal unmount fails
		forceCmd := exec.CommandContext(ctx, "umount", "-f", c.mountPoint)
		forceOutput, forceErr := forceCmd.CombinedOutput()
		if forceErr != nil {
			return fmt.Errorf("unmount failed: %w (force: %w), output: %s, force output: %s",
				err, forceErr, string(output), string(forceOutput))
		}
	}

	c.connected = false
	c.mounted = false
	return nil
}

func (c *NFSClient) TestConnection(ctx context.Context) error {
	if !c.connected || !c.mounted {
		return fmt.Errorf("not connected or not mounted")
	}

	// Test by checking mount point accessibility
	info, err := os.Stat(c.mountPoint)
	if err != nil {
		return fmt.Errorf("mount point not accessible: %w", err)
	}

	if !info.IsDir() {
		return fmt.Errorf("mount point is not a directory")
	}

	return nil
}

func (c *NFSClient) ListDirectory(ctx context.Context, path string) ([]*FileInfo, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}

	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return nil, rerr
	}

	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	var files []*FileInfo
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue // Skip entries with permission errors
		}

		fileInfo := &FileInfo{
			Name:    entry.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
			IsDir:   info.IsDir(),
			Path:    path,
		}
		files = append(files, fileInfo)
	}

	return files, nil
}

func (c *NFSClient) GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}

	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return nil, rerr
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	return &FileInfo{
		Name:    filepath.Base(path),
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
		Path:    path,
	}, nil
}

func (c *NFSClient) CreateDirectory(ctx context.Context, path string) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}

	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return rerr
	}
	return os.MkdirAll(fullPath, 0755)
}

func (c *NFSClient) DeleteDirectory(ctx context.Context, path string) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}

	fullPath, rerr := c.resolveConfinedNoFollow(path)
	if rerr != nil {
		return rerr
	}
	return os.RemoveAll(fullPath)
}

func (c *NFSClient) DeleteFile(ctx context.Context, path string) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}

	fullPath, rerr := c.resolveConfinedNoFollow(path)
	if rerr != nil {
		return rerr
	}
	return os.Remove(fullPath)
}

func (c *NFSClient) CopyFile(ctx context.Context, src, dst string) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}

	srcPath, rerr := c.resolveConfined(src)
	if rerr != nil {
		return rerr
	}
	dstPath, rerr := c.resolveConfined(dst)
	if rerr != nil {
		return rerr
	}

	// Simple file copy using system commands for better performance
	cmd := exec.CommandContext(ctx, "cp", srcPath, dstPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("copy failed: %w, output: %s", err, string(output))
	}

	return nil
}

func (c *NFSClient) MoveFile(ctx context.Context, src, dst string) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}

	srcPath, rerr := c.resolveConfinedNoFollow(src)
	if rerr != nil {
		return rerr
	}
	dstPath, rerr := c.resolveConfinedNoFollow(dst)
	if rerr != nil {
		return rerr
	}

	// Use system move command
	cmd := exec.CommandContext(ctx, "mv", srcPath, dstPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("move failed: %w, output: %s", err, string(output))
	}

	return nil
}

func (c *NFSClient) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	if !c.connected {
		return nil, fmt.Errorf("not connected")
	}

	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return nil, rerr
	}
	return os.Open(fullPath)
}

func (c *NFSClient) WriteFile(ctx context.Context, path string, data io.Reader) error {
	if !c.connected {
		return fmt.Errorf("not connected")
	}

	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return rerr
	}
	file, err := os.Create(fullPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, data)
	return err
}

func (c *NFSClient) FileExists(ctx context.Context, path string) (bool, error) {
	if !c.connected {
		return false, fmt.Errorf("not connected")
	}

	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return false, rerr
	}
	_, err := os.Stat(fullPath)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (c *NFSClient) IsConnected() bool {
	return c.connected && c.mounted
}

func (c *NFSClient) GetRootPath() string {
	return c.mountPoint
}

func (c *NFSClient) GetProtocol() string {
	return "nfs"
}

func (c *NFSClient) GetConfig() interface{} {
	// Return a pointer for cross-platform parity (§11.4.81): the generic
	// nfs_client.go returns &c.config and both NFS tests assert .(*NFSConfig);
	// returning a value here panicked the tests on darwin/windows only.
	return &c.config
}

// resolvePath resolves a path within the mount point; ConfinePath: a ".." segment cannot leave the mount (WF22 N7 class A).
func (c *NFSClient) resolvePath(path string) string {
	return filepath.Join(c.mountPoint, filepath.FromSlash(ConfinePath(path)))
}

// resolveConfined is resolvePath plus the real-path check (confineReal): a link on the mount that leaves the mount point is refused.
func (c *NFSClient) resolveConfined(path string) (string, error) {
	return confineReal(c.mountPoint, c.resolvePath(path), path, c.config.AllowSymlinkEscape, true)
}

// resolveConfinedNoFollow is resolveConfined for delete/rename sinks: the last component (a link) is acted on itself, not followed.
func (c *NFSClient) resolveConfinedNoFollow(path string) (string, error) {
	return confineReal(c.mountPoint, c.resolvePath(path), path, c.config.AllowSymlinkEscape, false)
}
