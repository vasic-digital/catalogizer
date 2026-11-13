//go:build windows
// +build windows

package filesystem

import (
	"context"
	"fmt"
	"io"
)

// NFSConfig contains NFS connection configuration
type NFSConfig struct {
	Host       string `json:"host"`
	Path       string `json:"path"`        // Export path on NFS server
	MountPoint string `json:"mount_point"` // Local mount point
	Options    string `json:"options"`     // Mount options
	// AllowSymlinkEscape mirrors the linux/darwin field so the shared factory compiles on every platform (CAT-FS-SYMLINK-1 round 1 F1);
	// this client performs no file operation, so it has no effect here.
	AllowSymlinkEscape bool `json:"allow_symlink_escape"`
}

// NFSClient for Windows (NFS is not natively supported)
type NFSClient struct {
	config     NFSConfig
	mountPoint string
	connected  bool
}

func NewNFSClient(config NFSConfig) (*NFSClient, error) {
	return &NFSClient{
		config:     config,
		mountPoint: config.MountPoint,
	}, nil
}

func (c *NFSClient) Connect(ctx context.Context) error {
	return fmt.Errorf("NFS mounting not supported on Windows platform")
}

func (c *NFSClient) Disconnect(ctx context.Context) error {
	c.connected = false
	return nil
}

func (c *NFSClient) TestConnection(ctx context.Context) error {
	return fmt.Errorf("NFS not supported on Windows platform")
}

func (c *NFSClient) ListDirectory(ctx context.Context, path string) ([]*FileInfo, error) {
	return nil, fmt.Errorf("NFS not supported on Windows platform")
}

func (c *NFSClient) GetFileInfo(ctx context.Context, path string) (*FileInfo, error) {
	return nil, fmt.Errorf("NFS not supported on Windows platform")
}

func (c *NFSClient) CreateDirectory(ctx context.Context, path string) error {
	return fmt.Errorf("NFS not supported on Windows platform")
}

func (c *NFSClient) DeleteDirectory(ctx context.Context, path string) error {
	return fmt.Errorf("NFS not supported on Windows platform")
}

func (c *NFSClient) DeleteFile(ctx context.Context, path string) error {
	return fmt.Errorf("NFS not supported on Windows platform")
}

func (c *NFSClient) CopyFile(ctx context.Context, src, dst string) error {
	return fmt.Errorf("NFS not supported on Windows platform")
}

func (c *NFSClient) MoveFile(ctx context.Context, src, dst string) error {
	return fmt.Errorf("NFS not supported on Windows platform")
}

func (c *NFSClient) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	return nil, fmt.Errorf("NFS not supported on Windows platform")
}

func (c *NFSClient) WriteFile(ctx context.Context, path string, data io.Reader) error {
	return fmt.Errorf("NFS not supported on Windows platform")
}

func (c *NFSClient) FileExists(ctx context.Context, path string) (bool, error) {
	return false, fmt.Errorf("NFS not supported on Windows platform")
}

func (c *NFSClient) IsConnected() bool {
	return c.connected
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
