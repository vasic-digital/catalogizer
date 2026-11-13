//go:build linux
// +build linux

package filesystem

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	pathpkg "path"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// procMountsPath is the kernel mount table; a variable so a test can present a mount table without a privileged mount.
var procMountsPath = "/proc/mounts"

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

// Connect establishes the NFS connection.
//
// Root cause of WF22 N1 (every NFS scan failed "not connected"): isMounted was a stub that returned true whenever /proc/mounts could be opened,
// so Connect decided "already mounted", set connected=true and NEVER mounted, while IsConnected additionally required mounted=true, which was
// never set. The behaviour had nothing to do with privileges. isMounted now really parses /proc/mounts for an NFS mount at the mount point:
//   - a mount point that is ALREADY an NFS mount (mounted by the host, or bind-mounted into a rootless container) connects with no privilege at
//     all, and Disconnect then leaves it alone (this client did not mount it);
//   - otherwise Connect mounts it itself, which needs CAP_SYS_ADMIN, and fails with the mount error when it does not have it.
func (c *NFSClient) Connect(ctx context.Context) error {
	if c.isMounted() {
		c.connected = true // mounted by someone else: readable as it is, and never unmounted by this client (c.mounted stays false)
		return nil
	}

	// Create mount point directory if it doesn't exist
	if err := os.MkdirAll(c.mountPoint, 0755); err != nil {
		return fmt.Errorf("failed to create mount point %s: %w", c.mountPoint, err)
	}

	// SEC-DIAL-1: the host is vetted by the operator-class policy and the kernel is handed the vetted ADDRESS (source and addr=), never the name,
	// so a name that resolves differently later cannot redirect the mount (anti DNS rebinding); host, export and options are checked for injection.
	source, options, err := planNFSMount(ctx, c.config)
	if err != nil {
		return err
	}
	if err := nfsMount(source, c.mountPoint, "nfs", 0, options); err != nil {
		// the resolved address is not echoed: the status endpoint shows this text to callers that are not administrators
		return fmt.Errorf("failed to mount NFS share %s:%s to %s: %w", c.config.Host, c.config.Path, c.mountPoint, err)
	}

	c.mounted = true
	c.connected = true
	return nil
}

// nfsMount is mount(2); tests replace it to observe the mount call without privileges.
var nfsMount = syscall.Mount

// planNFSMount turns the configuration into the (source, data) of mount(2). It refuses a host the operator-class policy refuses, a malformed
// host / export / option string, and an addr= / mountaddr= option naming a refused address. The server address in source and in addr= is the
// first vetted address of the host (mount(2) does not resolve names, the kernel NFS client needs the address).
func planNFSMount(ctx context.Context, cfg NFSConfig) (source, data string, err error) {
	if err := ValidateStorageHostSyntax(cfg.Host); err != nil {
		return "", "", fmt.Errorf("%w", ErrStorageHostRefused)
	}
	if err := ValidateNFSExport(cfg.Path); err != nil {
		return "", "", err
	}
	options := "vers=3"
	if cfg.Options != "" {
		options = cfg.Options
	}
	opts, err := ParseNFSOptions(options)
	if err != nil {
		return "", "", err
	}
	vctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	srvIP, err := resolveNFSServer(vctx, cfg.Host)
	if err != nil {
		return "", "", err
	}
	hasAddr := false
	for _, o := range opts {
		if !nfsAddrOptions[o.Key] {
			continue
		}
		if o.Key == "addr" {
			hasAddr = true
		}
		oip := net.ParseIP(o.Value)
		if oip == nil {
			return "", "", fmt.Errorf("%w", ErrStorageHostRefused)
		}
		if _, verr := storageOperatorResolve(vctx, oip.String()); verr != nil {
			return "", "", fmt.Errorf("%w", ErrStorageHostRefused)
		}
	}
	ip := srvIP.String()
	if !hasAddr {
		options += ",addr=" + ip
	}
	srv := ip
	if strings.Contains(ip, ":") {
		srv = "[" + ip + "]"
	}
	return srv + ":" + cfg.Path, options, nil
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
	return c.connected && c.isMounted()
}

// TestConnection tests the NFS connection
func (c *NFSClient) TestConnection(ctx context.Context) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}
	_, err := os.Stat(c.mountPoint)
	return err
}

// isMounted reports whether the mount point is an NFS (nfs / nfs4) mount in /proc/mounts.
func (c *NFSClient) isMounted() bool {
	data, err := os.ReadFile(procMountsPath)
	if err != nil {
		return false
	}
	return mountsContainNFS(string(data), c.mountPoint)
}

// mountsContainNFS parses /proc/mounts text: a line is "source target fstype options dump pass" with the characters space, tab, newline and
// backslash octal-escaped in source and target. It reports whether target is mounted with an nfs / nfs4 file system type.
func mountsContainNFS(mounts, target string) bool {
	want := filepath.Clean(target)
	for _, line := range strings.Split(mounts, "\n") {
		f := strings.Fields(line)
		if len(f) < 3 {
			continue
		}
		if filepath.Clean(unescapeMount(f[1])) == want && (f[2] == "nfs" || f[2] == "nfs4") {
			return true
		}
	}
	return false
}

// unescapeMount decodes the \040-style octal escapes of a /proc/mounts field.
func unescapeMount(s string) string {
	if !strings.Contains(s, "\\") {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+4 <= len(s) {
			if n, ok := octal3(s[i+1 : i+4]); ok {
				b.WriteByte(n)
				i += 3
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func octal3(s string) (byte, bool) {
	if len(s) != 3 {
		return 0, false
	}
	var n int
	for _, ch := range []byte(s) {
		if ch < '0' || ch > '7' {
			return 0, false
		}
		n = n*8 + int(ch-'0')
	}
	if n > 255 {
		return 0, false
	}
	return byte(n), true
}

// resolvePath resolves a relative path within the NFS mount point (ConfinePath: a ".." segment cannot leave the mount, a name containing ".." is kept).
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

// ReadFile reads a file from the NFS mount
func (c *NFSClient) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}
	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return nil, rerr
	}
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
	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return nil, rerr
	}
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
	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return nil, rerr
	}
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
	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return false, rerr
	}
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
	fullPath, rerr := c.resolveConfined(path)
	if rerr != nil {
		return rerr
	}
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
	fullPath, rerr := c.resolveConfinedNoFollow(path)
	if rerr != nil {
		return rerr
	}
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
	fullPath, rerr := c.resolveConfinedNoFollow(path)
	if rerr != nil {
		return rerr
	}
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
func (c *NFSClient) GetProtocol() string {
	return "nfs"
}

// GetConfig returns the NFS configuration
func (c *NFSClient) GetConfig() interface{} {
	return &c.config
}
