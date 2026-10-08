# Protocol Implementation Guide

Comprehensive guide for implementing and using network protocols in Catalogizer.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [FTP Protocol](#ftp-protocol)
- [NFS Protocol](#nfs-protocol)
- [WebDAV Protocol](#webdav-protocol)
- [SMB/CIFS Protocol](#smbcifs-protocol)
- [Adding New Protocols](#adding-new-protocols)
- [Testing Protocols](#testing-protocols)
- [Troubleshooting](#troubleshooting)

---

## Architecture Overview

### UnifiedClient Interface

All protocol implementations conform to the `UnifiedClient` interface defined in `catalog-api/filesystem/interface.go`:

```go
type UnifiedClient interface {
    Connect() error
    Disconnect() error
    ListFiles(path string) ([]FileInfo, error)
    ReadFile(path string) ([]byte, error)
    WriteFile(path string, data []byte) error
    DeleteFile(path string) error
    Stat(path string) (FileInfo, error)
    IsConnected() bool
}

type FileInfo struct {
    Name         string
    Path         string
    Size         int64
    ModTime      time.Time
    IsDir        bool
    Permissions  string
}
```

### Factory Pattern

Protocol clients are created using the factory pattern in `filesystem/factory.go`:

```go
func NewUnifiedClient(config ProtocolConfig) (UnifiedClient, error) {
    switch config.Type {
    case "ftp":
        return NewFTPClient(config.FTP), nil
    case "nfs":
        return NewNFSClient(config.NFS), nil
    case "webdav":
        return NewWebDAVClient(config.WebDAV), nil
    case "smb":
        return NewSMBClient(config.SMB), nil
    case "local":
        return NewLocalClient(config.Local), nil
    default:
        return nil, fmt.Errorf("unsupported protocol: %s", config.Type)
    }
}
```

### Configuration Structure

```go
type ProtocolConfig struct {
    Type   string         // "ftp", "nfs", "webdav", "smb", "local"
    FTP    *FTPConfig
    NFS    *NFSConfig
    WebDAV *WebDAVConfig
    SMB    *SMBConfig
    Local  *LocalConfig
}
```

---

## FTP Protocol

### Overview

File Transfer Protocol (FTP) client implementation for accessing remote FTP servers.

**Features:**
- Active and Passive mode support
- TLS/SSL encryption (FTPS)
- Connection pooling
- Automatic reconnection on failure
- Progress tracking for large files

### Configuration

```go
type FTPConfig struct {
    Host     string `json:"host"`      // FTP server hostname
    Port     int    `json:"port"`      // Default: 21
    Username string `json:"username"`  // FTP username
    Password string `json:"password"`  // FTP password
    Path     string `json:"path"`      // Base directory on server (settings key "path")
}
// Settings the factory does not consume (tls, passive_mode, timeout, base_path, ...) are rejected, not ignored. FTPS is not implemented yet.
```

### Example Usage

```go
import "catalogizer/filesystem"

// Create FTP configuration
config := filesystem.ProtocolConfig{
    Type: "ftp",
    FTP: &filesystem.FTPConfig{
        Host:     "ftp.example.com",
        Port:     21,
        Username: "ftpuser",
        Password: "ftppass",
        TLS:      false,
        Timeout:  30,
        BasePath: "/media",
    },
}

// Create client
client, err := filesystem.NewUnifiedClient(config)
if err != nil {
    log.Fatal(err)
}

// Connect
if err := client.Connect(); err != nil {
    log.Fatal(err)
}
defer client.Disconnect()

// List files
files, err := client.ListFiles("/movies")
if err != nil {
    log.Fatal(err)
}

for _, file := range files {
    fmt.Printf("%s (%d bytes)\n", file.Name, file.Size)
}
```

### API Endpoints

**Add FTP Storage Root:**
```http
POST /api/v1/storage-roots
Content-Type: application/json

{
  "name": "FTP Server",
  "protocol": "ftp",
  "settings": {
    "host": "ftp.example.com",
    "port": 21,
    "username": "ftpuser",
    "password": "ftppass",
    "path": "/media"
  }
}
```

### Implementation Details

**File:** `catalog-api/filesystem/ftp_client.go`

**Connection Management:**
- Connection pooling with max 5 concurrent connections
- Automatic reconnection with exponential backoff
- Keep-alive using NOOP command every 30 seconds

**Error Handling:**
- Retries on transient errors (network timeout, temporary failure)
- Graceful degradation when connection lost
- Detailed error logging for debugging

**Performance Optimizations:**
- Parallel file transfers using worker pools
- Chunked reading for large files
- Directory listing caching (configurable TTL)

### Troubleshooting

**Connection timeout:**
```bash
# Test FTP connection manually
ftp ftp.example.com
# Or using lftp
lftp -u username,password ftp.example.com
```

**Passive mode issues:**
- If behind NAT/firewall, ensure passive mode is enabled
- Configure passive port range on FTP server
- Check firewall rules for port 21 and passive ports

**TLS certificate errors:**
```go
// Skip certificate verification (development only)
config.FTP.TLS = true
config.FTP.InsecureSkipVerify = true  // DON'T USE IN PRODUCTION
```

---

## NFS Protocol

### Overview

Network File System (NFS) client for accessing Unix/Linux network shares.

**Features:**
- NFSv3 and NFSv4 support
- Kerberos authentication (NFSv4)
- Read/write caching
- Platform-specific implementations (Linux, macOS, Windows via WSL)

### Configuration

```go
type NFSConfig struct {
    Host       string `json:"host"`        // NFS server hostname
    Path       string `json:"path"`        // Exported directory path (settings key "path"; "export_path" is rejected with a hint)
    MountPoint string `json:"mount_point"` // Local mount point
    Options    string `json:"options"`     // Mount options, e.g. "vers=3,ro" (version and read-only are mount options, not keys)
}
```

### Example Usage

```go
config := filesystem.ProtocolConfig{
    Type: "nfs",
    NFS: &filesystem.NFSConfig{
        Host:       "192.168.1.100",
        ExportPath: "/export/media",
        MountPoint: "/mnt/nfs-media",
        Version:    4,
        ReadOnly:   false,
        Options:    "soft,timeo=30,retrans=3",
    },
}

client, err := filesystem.NewUnifiedClient(config)
if err != nil {
    log.Fatal(err)
}

// Mount NFS share
if err := client.Connect(); err != nil {
    log.Fatal(err)
}
defer client.Disconnect()

// Access files
files, err := client.ListFiles("/movies")
```

### Platform-Specific Implementations

**Linux:**
```bash
# NFSv4 with automatic mount
sudo mount -t nfs4 192.168.1.100:/export/media /mnt/nfs-media

# NFSv3
sudo mount -t nfs -o vers=3 192.168.1.100:/export/media /mnt/nfs-media
```

**macOS:**
```bash
# Mount NFS share
sudo mount -t nfs -o vers=4 192.168.1.100:/export/media /Volumes/nfs-media
```

**Windows (via WSL):**
```powershell
# Enable NFS client feature
Enable-WindowsOptionalFeature -FeatureName ServicesForNFS-ClientOnly -Online

# Mount NFS share
mount \\192.168.1.100\export\media Z:
```

### Implementation Details

**File:** `catalog-api/filesystem/nfs_client.go`

**Mount Management:**
- Automatic mount on Connect()
- Unmount on Disconnect()
- Mount point validation and creation
- Persistent mount entries (optional via /etc/fstab)

**Permissions:**
- NFS mount requires root privileges on Linux/macOS
- Catalogizer uses setuid wrapper or systemd service with elevated privileges
- User-space NFS implementations supported (go-nfs library)

**Caching:**
- Attribute caching (actimeo) for performance
- Read-ahead caching for sequential access
- Write-behind caching (sync vs async)

### Security Considerations

**NFSv4 with Kerberos:**
```go
config.NFS.Version = 4
config.NFS.Options = "sec=krb5,vers=4.2"
config.NFS.KerberosKeytab = "/etc/krb5.keytab"
```

**Restrict by IP:**
```bash
# On NFS server (/etc/exports)
/export/media 192.168.1.0/24(rw,sync,no_subtree_check,no_root_squash)
```

### Troubleshooting

**Permission denied:**
```bash
# Check NFS server exports
showmount -e 192.168.1.100

# Check mount options
mount | grep nfs
```

**Stale file handle:**
```bash
# Unmount and remount
sudo umount -f /mnt/nfs-media
sudo mount -t nfs4 192.168.1.100:/export/media /mnt/nfs-media
```

**Performance issues:**
```bash
# Use rsize/wsize options for better performance
mount -o rsize=32768,wsize=32768,vers=4 ...
```

---

## WebDAV Protocol

### Overview

Web Distributed Authoring and Versioning (WebDAV) client for HTTP-based file access.

**Features:**
- HTTP and HTTPS support
- Multiple authentication methods (Basic, Digest, OAuth)
- SSL/TLS certificate validation
- Large file streaming
- Partial downloads (Range requests)

### Configuration

```go
type WebDAVConfig struct {
    URL      string `json:"url"`       // WebDAV server URL
    Username string `json:"username"`  // Username
    Password string `json:"password"`  // Password
    Path     string `json:"path"`      // Root collection under the URL's own path (settings key "path")
}
```

### Example Usage

```go
config := filesystem.ProtocolConfig{
    Type: "webdav",
    WebDAV: &filesystem.WebDAVConfig{
        URL:      "https://webdav.example.com",
        Username: "webdavuser",
        Password: "webdavpass",
        TLS:      true,
        BasePath: "/remote.php/dav/files/user",
        Timeout:  60,
    },
}

client, err := filesystem.NewUnifiedClient(config)
if err != nil {
    log.Fatal(err)
}

if err := client.Connect(); err != nil {
    log.Fatal(err)
}
defer client.Disconnect()

// Upload file
data := []byte("file content")
err = client.WriteFile("/documents/file.txt", data)
```

### Supported Services

**Nextcloud/ownCloud:**
```go
config.WebDAV.URL = "https://cloud.example.com"
config.WebDAV.BasePath = "/remote.php/dav/files/username"
```

**Apache mod_dav:**
```go
config.WebDAV.URL = "https://example.com"
config.WebDAV.BasePath = "/webdav"
```

**Box.com:**
```go
config.WebDAV.URL = "https://dav.box.com/dav"
config.WebDAV.Username = "user@example.com"
```

### Implementation Details

**File:** `catalog-api/filesystem/webdav_client.go`

**HTTP Methods:**
- `PROPFIND` - List files and get metadata
- `GET` - Download files
- `PUT` - Upload files
- `DELETE` - Delete files
- `MKCOL` - Create directories
- `MOVE` - Rename/move files
- `COPY` - Copy files

**Authentication:**
```go
// Basic authentication (default)
req.SetBasicAuth(username, password)

// Digest authentication
// Automatically handled by HTTP client

// OAuth 2.0 (Nextcloud, etc.)
req.Header.Set("Authorization", "Bearer "+token)
```

**SSL/TLS:**
```go
transport := &http.Transport{
    TLSClientConfig: &tls.Config{
        InsecureSkipVerify: false,  // Verify certificates
        MinVersion:         tls.VersionTLS12,
    },
}
```

### Troubleshooting

**401 Unauthorized:**
- Verify username and password
- Check if two-factor authentication is enabled (use app password)
- Ensure correct base path for service (Nextcloud uses `/remote.php/dav/files/user`)

**SSL certificate errors:**
```bash
# Test WebDAV connection
curl -u username:password -X PROPFIND https://webdav.example.com/
```

**Large file uploads failing:**
- Increase `Timeout` in config
- Check server-side upload limits (PHP max_upload_size, nginx client_max_body_size)

---

## SMB/CIFS Protocol

### Overview

Server Message Block (SMB/CIFS) client for Windows network shares and Samba.

**Features:**
- SMBv2/v3 support
- Circuit breaker pattern for resilience
- Offline caching
- Exponential backoff retry
- Connection pooling

### Configuration

```go
type SMBConfig struct {
    Host     string `json:"host"`      // SMB server hostname/IP
    Share    string `json:"share"`     // Share name
    Username string `json:"username"`  // Username
    Password string `json:"password"`  // Password
    Domain   string `json:"domain"`    // Windows domain (optional)
    Port     int    `json:"port"`      // Default: 445
    Workgroup string `json:"workgroup"` // Workgroup name
}
```

### Example Usage

```go
config := filesystem.ProtocolConfig{
    Type: "smb",
    SMB: &filesystem.SMBConfig{
        Host:     "192.168.1.50",
        Share:    "Media",
        Username: "smbuser",
        Password: "smbpass",
        Domain:   "WORKGROUP",
        Port:     445,
    },
}

client, err := filesystem.NewUnifiedClient(config)
if err != nil {
    log.Fatal(err)
}

if err := client.Connect(); err != nil {
    log.Fatal(err)
}
defer client.Disconnect()

// Read file
data, err := client.ReadFile("/movies/video.mp4")
```

### Advanced Features

**Circuit Breaker Pattern:**

Protects against repeated connection failures:

```go
type CircuitBreaker struct {
    maxFailures  int           // Open circuit after N failures
    resetTimeout time.Duration // Time before attempting reset
    state        State         // Closed, Open, HalfOpen
}

// Usage is automatic in SMBClient
// Circuit opens after 5 consecutive failures
// Stays open for 60 seconds before trying again
```

**Offline Cache:**

Caches file metadata and content when connection unavailable:

```go
// Enable offline cache
client.EnableOfflineCache(true)
client.SetCacheTTL(5 * time.Minute)

// Reads from cache when SMB unavailable
data, err := client.ReadFile("/path/file.txt")
// Returns cached data if available
```

**Retry with Exponential Backoff:**

```go
// Automatically retries failed operations
// Backoff: 1s, 2s, 4s, 8s, 16s
// Max retries: 5
```

### Implementation Details

**File:** `catalog-api/filesystem/smb_client.go`

**Libraries Used:**
- `github.com/hirochachacha/go-smb2` - Pure Go SMB2/3 client

**Connection Management:**
- Connection pool with max 10 connections
- Idle timeout: 5 minutes
- Automatic cleanup of stale connections

**Performance:**
- Parallel file transfers
- Chunked reading (64KB chunks)
- Opportunistic locking for performance

### Platform Compatibility

**Linux (via CIFS kernel module):**
```bash
# Mount SMB share
sudo mount -t cifs //192.168.1.50/Media /mnt/smb \
  -o username=smbuser,password=smbpass,vers=3.0
```

**macOS:**
```bash
# Connect via Finder
# Go → Connect to Server → smb://192.168.1.50/Media

# Or via command line
mount_smbfs //smbuser:smbpass@192.168.1.50/Media /Volumes/smb
```

**Windows:**
```powershell
# Map network drive
net use Z: \\192.168.1.50\Media /user:smbuser smbpass
```

### Troubleshooting

**Connection refused (port 445):**
```bash
# Test SMB connectivity
smbclient -L //192.168.1.50 -U smbuser

# Check if port 445 is open
nc -zv 192.168.1.50 445
```

**Authentication failed:**
- Verify username/password
- Check domain/workgroup name
- Ensure SMB user has permissions on share

**Performance issues:**
- Use SMBv3 for better performance
- Enable SMB multichannel (Windows Server 2012+)
- Check network bandwidth and latency

---

## Adding New Protocols

### Step 1: Implement UnifiedClient Interface

Create new file `catalog-api/filesystem/newprotocol_client.go`:

```go
package filesystem

import (
    "fmt"
    "time"
)

type NewProtocolClient struct {
    config     *NewProtocolConfig
    connected  bool
    // Add protocol-specific fields
}

func NewNewProtocolClient(config *NewProtocolConfig) *NewProtocolClient {
    return &NewProtocolClient{
        config: config,
    }
}

func (c *NewProtocolClient) Connect() error {
    // Implement connection logic
    c.connected = true
    return nil
}

func (c *NewProtocolClient) Disconnect() error {
    c.connected = false
    return nil
}

func (c *NewProtocolClient) ListFiles(path string) ([]FileInfo, error) {
    if !c.connected {
        return nil, fmt.Errorf("not connected")
    }
    // Implement file listing
    return []FileInfo{}, nil
}

func (c *NewProtocolClient) ReadFile(path string) ([]byte, error) {
    // Implement file reading
    return nil, nil
}

func (c *NewProtocolClient) WriteFile(path string, data []byte) error {
    // Implement file writing
    return nil
}

func (c *NewProtocolClient) DeleteFile(path string) error {
    // Implement file deletion
    return nil
}

func (c *NewProtocolClient) Stat(path string) (FileInfo, error) {
    // Implement file stat
    return FileInfo{}, nil
}

func (c *NewProtocolClient) IsConnected() bool {
    return c.connected
}
```

### Step 2: Add Configuration

In `catalog-api/filesystem/config.go`:

```go
type NewProtocolConfig struct {
    Host     string `json:"host"`
    Port     int    `json:"port"`
    Username string `json:"username"`
    Password string `json:"password"`
    // Add protocol-specific config fields
}
```

Update `ProtocolConfig`:

```go
type ProtocolConfig struct {
    Type        string
    // ... existing fields ...
    NewProtocol *NewProtocolConfig `json:"newprotocol,omitempty"`
}
```

### Step 3: Update Factory

In `catalog-api/filesystem/factory.go`:

```go
func NewUnifiedClient(config ProtocolConfig) (UnifiedClient, error) {
    switch config.Type {
    // ... existing cases ...
    case "newprotocol":
        if config.NewProtocol == nil {
            return nil, fmt.Errorf("newprotocol config is required")
        }
        return NewNewProtocolClient(config.NewProtocol), nil
    default:
        return nil, fmt.Errorf("unsupported protocol: %s", config.Type)
    }
}
```

### Step 4: Add Tests

Create `catalog-api/filesystem/newprotocol_client_test.go`:

```go
package filesystem

import (
    "testing"
)

func TestNewProtocolClient_Connect(t *testing.T) {
    config := &NewProtocolConfig{
        Host: "localhost",
        Port: 1234,
    }

    client := NewNewProtocolClient(config)

    if err := client.Connect(); err != nil {
        t.Fatalf("Connect failed: %v", err)
    }

    if !client.IsConnected() {
        t.Error("Expected client to be connected")
    }
}

// Add more tests...
```

### Step 5: Document

Update this guide with protocol-specific documentation.

---

## Testing Protocols

### Unit Tests

```bash
# Test specific protocol
go test -v ./filesystem -run TestFTPClient
go test -v ./filesystem -run TestWebDAVClient

# Test all protocols
go test -v ./filesystem/...
```

### Integration Tests

Located in `catalog-api/tests/integration/`:

```bash
# Run integration tests (requires real servers)
go test -v ./tests/integration/
```

### Mock Servers

Test helpers provide mock servers for testing:

```go
import "catalogizer/internal/tests"

func TestWebDAVIntegration(t *testing.T) {
    mockServer := tests.NewWebDAVMockServer(t)
    defer mockServer.Close()

    mockServer.AddFile("/test.txt", []byte("content"))

    config := filesystem.ProtocolConfig{
        Type: "webdav",
        WebDAV: &filesystem.WebDAVConfig{
            URL: mockServer.URL(),
        },
    }

    client, _ := filesystem.NewUnifiedClient(config)
    // Test client operations...
}
```

---

## Troubleshooting

### General Debugging

**Enable debug logging:**
```bash
export LOG_LEVEL=debug
go run main.go
```

**Test protocol connectivity:**
```go
client, _ := filesystem.NewUnifiedClient(config)
if err := client.Connect(); err != nil {
    log.Printf("Connection failed: %v", err)
    // Check error type for specific handling
}
```

### Connection Issues

**Firewall blocking:**
```bash
# Check if port is open
nc -zv hostname port

# FTP: port 21
# NFS: port 2049
# WebDAV: port 443 (HTTPS) or 80 (HTTP)
# SMB: port 445
```

**DNS resolution:**
```bash
# Test hostname resolution
nslookup hostname
dig hostname
```

### Authentication Issues

**Verify credentials:**
```bash
# FTP
ftp hostname

# SMB
smbclient -L //hostname -U username

# WebDAV
curl -u username:password -X PROPFIND https://hostname/
```

### Performance Issues

**Enable profiling:**
```go
import _ "net/http/pprof"

// In main.go
go func() {
    log.Println(http.ListenAndServe("localhost:6060", nil))
}()
```

**Monitor operations:**
```bash
# CPU profiling
go tool pprof http://localhost:6060/debug/pprof/profile

# Memory profiling
go tool pprof http://localhost:6060/debug/pprof/heap
```

---

## Best Practices

1. **Always call Disconnect()** in defer after Connect()
2. **Handle connection failures** with retry logic
3. **Use context for timeouts** on long operations
4. **Implement circuit breaker** for unreliable connections
5. **Cache metadata** to reduce network calls
6. **Log errors** with sufficient context for debugging
7. **Test with real servers** before production deployment
8. **Monitor connection pools** to prevent leaks
9. **Validate paths** to prevent directory traversal attacks
10. **Use TLS/SSL** for sensitive data transfers

---

## Additional Resources

- [UnifiedClient Interface Documentation](../architecture/FILESYSTEM_INTERFACE.md)
- [Configuration Reference](CONFIGURATION_REFERENCE.md)
- [API Documentation](../api/API_DOCUMENTATION.md)
- [Testing Guide](../testing/TESTING_GUIDE.md)
