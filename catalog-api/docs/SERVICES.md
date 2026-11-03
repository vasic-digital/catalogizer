# Services Documentation

This document provides detailed information about the services available in the Catalog API.

## Table of Contents

1. [FileSystemService](#filesystemservice)
2. [MediaRecognitionService](#mediarecognitionservice)
3. [ReaderService](#readerservice)
4. [CatalogService](#catalogservice)
5. [ConversionService](#conversionservice)
6. [FavoritesService](#favoritesservice)
7. [ReportingService](#reportingservice)
8. [SyncService](#syncservice)

---

## FileSystemService

The FileSystemService provides a unified interface for interacting with multiple filesystem protocols, enabling seamless file operations across different storage backends.

### Overview

Located at: `internal/services/filesystem_service.go`

The FileSystemService acts as a facade for the filesystem factory, providing high-level operations while managing client lifecycle and connections automatically.

### Supported Protocols

#### 1. Local Filesystem
Direct access to the local filesystem.

**Use Cases:**
- Temporary file storage
- Local cache management
- Development and testing

**Configuration:**
```json
{
  "id": "local-storage",
  "name": "Local Storage",
  "protocol": "local",
  "enabled": true,
  "settings": {
    "base_path": "/data/storage"
  }
}
```

#### 2. SMB (Server Message Block)
Windows file sharing protocol for network storage.

**Use Cases:**
- Windows network shares
- NAS devices
- Enterprise file servers

**Configuration:**
```json
{
  "id": "smb-nas",
  "name": "SMB NAS Storage",
  "protocol": "smb",
  "enabled": true,
  "settings": {
    "host": "192.168.1.100",
    "port": 445,
    "share": "shared",
    "username": "user",
    "password": "password",
    "domain": "WORKGROUP"
  }
}
```

#### 3. FTP (File Transfer Protocol)
Remote file transfer over FTP.

**Use Cases:**
- Legacy file servers
- Web hosting file transfers
- Backup solutions

**Configuration:**
```json
{
  "id": "ftp-server",
  "name": "FTP Server",
  "protocol": "ftp",
  "enabled": true,
  "settings": {
    "host": "ftp.example.com",
    "port": 21,
    "username": "ftpuser",
    "password": "ftppass",
    "path": "/files",
    "passive": true
  }
}
```

#### 4. NFS (Network File System)
Unix/Linux network file sharing protocol.

**Use Cases:**
- Linux/Unix network shares
- High-performance file sharing
- Container storage

**Configuration:**
```json
{
  "id": "nfs-storage",
  "name": "NFS Storage",
  "protocol": "nfs",
  "enabled": true,
  "settings": {
    "host": "nfs.example.com",
    "path": "/export/data",
    "mount_point": "/mnt/nfs",
    "options": "vers=3,tcp,rsize=32768,wsize=32768"
  }
}
```

#### 5. WebDAV
HTTP-based file access protocol.

**Use Cases:**
- Cloud storage services
- Web-based file management
- Cross-platform file sharing

**Configuration:**
```json
{
  "id": "webdav-cloud",
  "name": "WebDAV Cloud Storage",
  "protocol": "webdav",
  "enabled": true,
  "settings": {
    "url": "https://cloud.example.com/webdav",
    "username": "user",
    "password": "password",
    "insecure_skip_verify": false
  }
}
```

### Methods

#### GetClient

Creates a filesystem client for the specified protocol.

```go
func (fs *FileSystemService) GetClient(config *filesystem.StorageConfig) (filesystem.Client, error)
```

**Parameters:**
- `config`: Storage configuration containing protocol and connection details

**Returns:**
- `filesystem.Client`: A client instance for the specified protocol
- `error`: Error if client creation fails

**Example:**
```go
config := &filesystem.StorageConfig{
    ID:       "local-test",
    Name:     "Local Test Storage",
    Protocol: "local",
    Enabled:  true,
    Settings: map[string]interface{}{
        "base_path": "/tmp",
    },
}

client, err := fsService.GetClient(config)
if err != nil {
    return err
}
```

#### ListFiles

Lists files in a directory, automatically connecting if needed.

```go
func (fs *FileSystemService) ListFiles(ctx context.Context, client filesystem.Client, path string) ([]filesystem.FileInfo, error)
```

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `client`: Filesystem client instance
- `path`: Directory path to list

**Returns:**
- `[]filesystem.FileInfo`: Array of file information structs
- `error`: Error if listing fails

**Example:**
```go
files, err := fsService.ListFiles(ctx, client, "/documents")
if err != nil {
    return err
}

for _, file := range files {
    fmt.Printf("File: %s, Size: %d, IsDir: %v\n",
        file.Name, file.Size, file.IsDirectory)
}
```

#### GetFileInfo

Retrieves information about a specific file.

```go
func (fs *FileSystemService) GetFileInfo(ctx context.Context, client filesystem.Client, path string) (*filesystem.FileInfo, error)
```

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `client`: Filesystem client instance
- `path`: File path

**Returns:**
- `*filesystem.FileInfo`: Pointer to file information struct
- `error`: Error if operation fails

**Example:**
```go
info, err := fsService.GetFileInfo(ctx, client, "/documents/report.pdf")
if err != nil {
    return err
}

fmt.Printf("File size: %d bytes\n", info.Size)
fmt.Printf("Modified: %v\n", info.ModifiedTime)
```

#### CopyFile

Copies a file from source to destination (implementation depends on client).

```go
func (fs *FileSystemService) CopyFile(ctx context.Context, sourceClient filesystem.Client, sourcePath string, destClient filesystem.Client, destPath string) error
```

**Parameters:**
- `ctx`: Context for cancellation and timeouts
- `sourceClient`: Source filesystem client
- `sourcePath`: Source file path
- `destClient`: Destination filesystem client
- `destPath`: Destination file path

**Returns:**
- `error`: Error if copy fails

### Connection Management

The FileSystemService automatically manages connections:

1. **Lazy Connection**: Connections are established on first use
2. **Connection Pooling**: Reuses existing connections when possible
3. **Auto-Reconnect**: Automatically reconnects on connection failures
4. **Graceful Cleanup**: Properly closes connections on shutdown

### Error Handling

The service provides comprehensive error handling:

```go
// Connection errors
ErrConnectionFailed = errors.New("failed to connect to storage")

// Operation errors
ErrFileNotFound = errors.New("file not found")
ErrPermissionDenied = errors.New("permission denied")
ErrInvalidPath = errors.New("invalid path")

// Protocol errors
ErrUnsupportedProtocol = errors.New("unsupported protocol")
ErrInvalidConfiguration = errors.New("invalid configuration")
```

### Testing

The service has comprehensive test coverage:

**Test Location:** `internal/services/filesystem_service_test.go`

**Test Coverage:**
- Unit tests: 7 tests (all passing)
- Integration tests: Included
- Coverage: 80%+

**Test Categories:**
1. Service initialization tests
2. Client creation tests for all protocols
3. Connection handling tests
4. File operation tests
5. Error handling tests

**Run Tests:**
```bash
# Run service tests
go test ./internal/services/filesystem_service_test.go ./internal/services/filesystem_service.go -v

# Run with coverage
go test ./internal/services/... -cover

# Run integration tests
go test ./tests/integration/filesystem_operations_test.go -v
```

### Performance Considerations

1. **Connection Pooling**: Reuses connections to minimize overhead
2. **Concurrent Operations**: Supports concurrent file operations
3. **Stream Processing**: Uses streaming for large file transfers
4. **Resource Cleanup**: Automatic cleanup of unused connections

### Security

1. **Credential Management**: Secure handling of authentication credentials
2. **Path Validation**: Validates paths to prevent directory traversal
3. **Access Control**: Respects filesystem permissions
4. **SSL/TLS Support**: Encrypted connections for supported protocols

---

## MediaRecognitionService

AI-powered media recognition service for movies, music, books, games, and software.

### Overview

Located at: `internal/services/media_recognition_service.go`

Provides comprehensive media recognition using multiple external APIs and advanced metadata extraction.

### Supported Media Types

1. **Movies & TV Shows** - TMDb and OMDb integration
2. **Music** - Audio fingerprinting with Last.fm, MusicBrainz, AcoustID
3. **Books** - OCR and metadata lookup with Google Books, Open Library
4. **Games** - IGDB integration
5. **Software** - Package manager and GitHub integration

### Key Features

- Multi-source metadata aggregation
- Confidence scoring
- Caching for performance
- Batch processing support
- Duplicate detection using AI similarity

---

## ReaderService

Premium reading experience service with position tracking and synchronization.

### Overview

Located at: `internal/services/reader_service.go`

Provides a Kindle-like reading experience with comprehensive position tracking across devices.

### Features

1. **Multi-granular Position Tracking**
   - Page tracking
   - Word position
   - Character position
   - CFI (EPUB) support

2. **Cross-device Synchronization**
   - Real-time position sync
   - Conflict resolution
   - Multiple device support

3. **Annotations**
   - Bookmarks
   - Highlights
   - Notes
   - Search capabilities

4. **Reading Analytics**
   - Reading speed tracking
   - Time analytics
   - Reading streaks
   - Goal tracking

---

## CatalogService

Main catalog management service for browsing and searching files.

### Overview

Located at: `internal/services/catalog_service.go`

Provides core catalog functionality including browsing, searching, and statistics.

### Key Features

1. **Catalog Browsing**
   - Multi-root support
   - Pagination
   - Sorting options

2. **Advanced Search**
   - Full-text search
   - Multiple filters
   - Duplicate detection

3. **Statistics & Analytics**
   - Directory size analysis
   - File type distribution
   - Growth trends

---

## Best Practices

### Using FileSystemService

1. **Always use context with timeout:**
```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

files, err := fsService.ListFiles(ctx, client, "/path")
```

2. **Handle errors appropriately:**
```go
if err != nil {
    if errors.Is(err, filesystem.ErrConnectionFailed) {
        // Handle connection errors
    } else if errors.Is(err, filesystem.ErrFileNotFound) {
        // Handle missing files
    } else {
        // Handle other errors
    }
}
```

3. **Close clients when done:**
```go
defer client.Close()
```

4. **Use appropriate buffer sizes for large files:**
```go
// For large file transfers
const bufferSize = 64 * 1024 // 64KB buffer
```

### Configuration Management

1. Store sensitive credentials securely
2. Use environment variables for secrets
3. Validate configuration before use
4. Implement configuration hot-reload when possible

### Monitoring and Logging

1. Log all service operations
2. Track operation duration
3. Monitor connection health
4. Alert on repeated failures

---

## Troubleshooting

### Common Issues

#### Connection Failures

**Problem:** Unable to connect to storage
**Solutions:**
- Verify network connectivity
- Check credentials
- Verify firewall rules
- Check protocol-specific settings

#### Permission Denied

**Problem:** Access denied to files/directories
**Solutions:**
- Verify user permissions
- Check share/export permissions
- Validate credential configuration

#### Slow Performance

**Problem:** File operations are slow
**Solutions:**
- Enable connection pooling
- Increase buffer sizes
- Use batch operations
- Check network bandwidth

---

## Additional Resources

- [API Documentation](./README.md)
- [Testing Guide](./TESTING.md)
- [Configuration Guide](../config.json.example)
- [Examples](./examples.md)
