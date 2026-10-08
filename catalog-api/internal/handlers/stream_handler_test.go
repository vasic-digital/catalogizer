package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/filesystem"
	"catalogizer/internal/models"
	root_models "catalogizer/models"

	"github.com/gin-gonic/gin"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// --- Mock catalog service for stream handler tests ---

type streamMockCatalogService struct {
	files map[string]*models.FileInfo
}

func (m *streamMockCatalogService) SetDB(db *database.DB) {}
func (m *streamMockCatalogService) ListPath(path, sortBy, sortOrder string, limit, offset int) ([]models.FileInfo, error) {
	return nil, nil
}
func (m *streamMockCatalogService) GetFileInfo(pathOrID string) (*models.FileInfo, error) {
	if f, ok := m.files[pathOrID]; ok {
		return f, nil
	}
	return nil, nil
}
func (m *streamMockCatalogService) SearchFiles(req *models.SearchRequest) ([]models.FileInfo, int64, error) {
	return nil, 0, nil
}
func (m *streamMockCatalogService) GetDirectoriesBySize(smbRoot string, limit int) ([]models.DirectoryStats, error) {
	return nil, nil
}
func (m *streamMockCatalogService) GetDuplicateGroups(smbRoot string, minCount, limit int) ([]models.DuplicateGroup, error) {
	return nil, nil
}
func (m *streamMockCatalogService) GetSMBRoots() ([]string, error)     { return nil, nil }
func (m *streamMockCatalogService) GetDuplicatesCount() (int64, error) { return 0, nil }
func (m *streamMockCatalogService) GetDirectoriesBySizeLimited(limit int) ([]models.DirectoryStats, error) {
	return nil, nil
}
func (m *streamMockCatalogService) ListDirectory(path string) ([]models.FileInfo, error) {
	return nil, nil
}
func (m *streamMockCatalogService) Search(query, fileType string, limit, offset int) ([]models.FileInfo, error) {
	return nil, nil
}
func (m *streamMockCatalogService) SearchDuplicates() ([]models.DuplicateGroup, error) {
	return nil, nil
}
func (m *streamMockCatalogService) GetFileInfoByPath(path string) (*models.FileInfo, error) {
	return nil, nil
}

// --- Mock filesystem client ---

// mockReadSeekCloser implements io.ReadSeekCloser for testing seekable streams.
type mockReadSeekCloser struct {
	*bytes.Reader
	closed bool
}

func newMockReadSeekCloser(data []byte) *mockReadSeekCloser {
	return &mockReadSeekCloser{Reader: bytes.NewReader(data)}
}

func (m *mockReadSeekCloser) Close() error {
	m.closed = true
	return nil
}

// mockReadCloser implements only io.ReadCloser (no seeking) for testing non-seekable streams.
type mockReadCloser struct {
	io.Reader
	closed bool
}

func newMockReadCloser(data []byte) *mockReadCloser {
	return &mockReadCloser{Reader: bytes.NewReader(data)}
}

func (m *mockReadCloser) Close() error {
	m.closed = true
	return nil
}

// mockFSClient implements filesystem.FileSystemClient for testing.
type mockFSClient struct {
	connected   bool
	reader      io.ReadCloser
	readFileErr error
	connectErr  error
	protocol    string
	fileSize    int64
}

func (m *mockFSClient) Connect(ctx context.Context) error {
	if m.connectErr != nil {
		return m.connectErr
	}
	m.connected = true
	return nil
}
func (m *mockFSClient) Disconnect(ctx context.Context) error {
	m.connected = false
	return nil
}
func (m *mockFSClient) IsConnected() bool                        { return m.connected }
func (m *mockFSClient) TestConnection(ctx context.Context) error { return nil }
func (m *mockFSClient) ReadFile(ctx context.Context, path string) (io.ReadCloser, error) {
	if m.readFileErr != nil {
		return nil, m.readFileErr
	}
	return m.reader, nil
}
func (m *mockFSClient) WriteFile(ctx context.Context, path string, data io.Reader) error {
	return nil
}
func (m *mockFSClient) GetFileInfo(ctx context.Context, path string) (*filesystem.FileInfo, error) {
	return &filesystem.FileInfo{Size: m.fileSize}, nil
}
func (m *mockFSClient) FileExists(ctx context.Context, path string) (bool, error) { return true, nil }
func (m *mockFSClient) DeleteFile(ctx context.Context, path string) error         { return nil }
func (m *mockFSClient) CopyFile(ctx context.Context, src, dst string) error       { return nil }
func (m *mockFSClient) ListDirectory(ctx context.Context, path string) ([]*filesystem.FileInfo, error) {
	return nil, nil
}
func (m *mockFSClient) CreateDirectory(ctx context.Context, path string) error { return nil }
func (m *mockFSClient) DeleteDirectory(ctx context.Context, path string) error { return nil }
func (m *mockFSClient) GetProtocol() string                                    { return m.protocol }
func (m *mockFSClient) GetConfig() interface{}                                 { return nil }

// mockSeekableFSClient implements both filesystem.FileSystemClient and filesystem.SeekableClient.
// This simulates protocols that support random access (SMB, local).
type mockSeekableFSClient struct {
	mockFSClient
	seekableReader filesystem.ReadSeekCloser
	openSeekErr    error
}

func (m *mockSeekableFSClient) OpenSeekable(ctx context.Context, path string) (filesystem.ReadSeekCloser, error) {
	if m.openSeekErr != nil {
		return nil, m.openSeekErr
	}
	return m.seekableReader, nil
}

// mockClientFactory returns a pre-configured mock filesystem client.
type mockClientFactory struct {
	client    filesystem.FileSystemClient
	createErr error
}

func (f *mockClientFactory) CreateClient(config *filesystem.StorageConfig) (filesystem.FileSystemClient, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	return f.client, nil
}

func (f *mockClientFactory) SupportedProtocols() []string {
	return []string{"smb", "ftp", "nfs", "webdav", "local"}
}

// --- Test helpers ---

func setupStreamTestDB(t *testing.T) *database.DB {
	t.Helper()

	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	_, err = sqlDB.Exec("PRAGMA foreign_keys = ON")
	require.NoError(t, err)

	// Create storage_roots table
	_, err = sqlDB.Exec(`
		CREATE TABLE IF NOT EXISTS storage_roots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			protocol TEXT NOT NULL,
			host TEXT,
			port INTEGER,
			path TEXT,
			username TEXT,
			password TEXT,
			domain TEXT,
			mount_point TEXT,
			options TEXT,
			url TEXT,
			enabled BOOLEAN DEFAULT 1,
			max_depth INTEGER DEFAULT 10,
			enable_duplicate_detection BOOLEAN DEFAULT 1,
			enable_metadata_extraction BOOLEAN DEFAULT 1,
			include_patterns TEXT,
			exclude_patterns TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			last_scan_at DATETIME
		)
	`)
	require.NoError(t, err)

	return database.WrapDB(sqlDB, database.DialectSQLite)
}

func insertStorageRoot(t *testing.T, db *database.DB, name, protocol, host string, port int, path, username, password string) {
	t.Helper()
	_, err := db.Exec(`
		INSERT INTO storage_roots (name, protocol, host, port, path, username, password, domain, enabled)
		VALUES (?, ?, ?, ?, ?, ?, ?, 'WORKGROUP', 1)
	`, name, protocol, host, port, path, username, password)
	require.NoError(t, err)
}

func setupStreamRouter(handler *StreamHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	api.GET("/stream/:id", handler.StreamFile)
	return router
}

// --- Tests ---

func TestStreamHandler_InvalidFileID(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	db := setupStreamTestDB(t)
	defer db.Close()

	catalog := &streamMockCatalogService{files: map[string]*models.FileInfo{}}
	factory := &mockClientFactory{}

	handler := NewStreamHandler(catalog, db, factory, logger)
	router := setupStreamRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stream/abc", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Invalid file ID", resp["error"])
}

func TestStreamHandler_FileNotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	db := setupStreamTestDB(t)
	defer db.Close()

	catalog := &streamMockCatalogService{files: map[string]*models.FileInfo{}}
	factory := &mockClientFactory{}

	handler := NewStreamHandler(catalog, db, factory, logger)
	router := setupStreamRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stream/999", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "File not found", resp["error"])
}

func TestStreamHandler_CannotStreamDirectory(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	db := setupStreamTestDB(t)
	defer db.Close()

	catalog := &streamMockCatalogService{
		files: map[string]*models.FileInfo{
			"1": {
				ID:          1,
				Name:        "movies",
				Path:        "/movies",
				IsDirectory: true,
				SmbRoot:     "nas",
			},
		},
	}
	factory := &mockClientFactory{}

	handler := NewStreamHandler(catalog, db, factory, logger)
	router := setupStreamRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stream/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Cannot stream a directory", resp["error"])
}

func TestStreamHandler_StorageRootNotFound(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	db := setupStreamTestDB(t)
	defer db.Close()

	// File references a storage root that doesn't exist in DB
	catalog := &streamMockCatalogService{
		files: map[string]*models.FileInfo{
			"1": {
				ID:      1,
				Name:    "movie.mp4",
				Path:    "/movies/movie.mp4",
				Size:    1000000,
				SmbRoot: "nonexistent-root",
			},
		},
	}
	factory := &mockClientFactory{}

	handler := NewStreamHandler(catalog, db, factory, logger)
	router := setupStreamRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stream/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Storage root not found", resp["error"])
}

func TestStreamHandler_FailedToCreateClient(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	db := setupStreamTestDB(t)
	defer db.Close()

	insertStorageRoot(t, db, "nas", "smb", "192.168.0.241", 445, "media", "user", "pass")

	catalog := &streamMockCatalogService{
		files: map[string]*models.FileInfo{
			"1": {
				ID:      1,
				Name:    "movie.mp4",
				Path:    "/movies/movie.mp4",
				Size:    1000000,
				SmbRoot: "nas",
			},
		},
	}
	factory := &mockClientFactory{createErr: fmt.Errorf("unsupported protocol")}

	handler := NewStreamHandler(catalog, db, factory, logger)
	router := setupStreamRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stream/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Failed to connect to storage", resp["error"])
}

func TestStreamHandler_FailedToConnect(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	db := setupStreamTestDB(t)
	defer db.Close()

	insertStorageRoot(t, db, "nas", "smb", "192.168.0.241", 445, "media", "user", "pass")

	catalog := &streamMockCatalogService{
		files: map[string]*models.FileInfo{
			"1": {
				ID:      1,
				Name:    "movie.mp4",
				Path:    "/movies/movie.mp4",
				Size:    1000000,
				SmbRoot: "nas",
			},
		},
	}
	fsClient := &mockFSClient{connectErr: fmt.Errorf("connection refused")}
	factory := &mockClientFactory{client: fsClient}

	handler := NewStreamHandler(catalog, db, factory, logger)
	router := setupStreamRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stream/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "Failed to connect to storage backend", resp["error"])
}

func TestStreamHandler_FileNotAccessible(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	db := setupStreamTestDB(t)
	defer db.Close()

	insertStorageRoot(t, db, "nas", "smb", "192.168.0.241", 445, "media", "user", "pass")

	catalog := &streamMockCatalogService{
		files: map[string]*models.FileInfo{
			"1": {
				ID:      1,
				Name:    "movie.mp4",
				Path:    "/movies/movie.mp4",
				Size:    1000000,
				SmbRoot: "nas",
			},
		},
	}
	fsClient := &mockFSClient{readFileErr: fmt.Errorf("file not found on share")}
	factory := &mockClientFactory{client: fsClient}

	handler := NewStreamHandler(catalog, db, factory, logger)
	router := setupStreamRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stream/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "File not accessible on storage", resp["error"])
}

func TestStreamHandler_SuccessfulStream_Seekable(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	db := setupStreamTestDB(t)
	defer db.Close()

	insertStorageRoot(t, db, "nas", "smb", "192.168.0.241", 445, "media", "user", "pass")

	fileContent := []byte("fake video content for testing seekable stream")
	mimeType := "video/mp4"

	catalog := &streamMockCatalogService{
		files: map[string]*models.FileInfo{
			"1": {
				ID:           1,
				Name:         "movie.mp4",
				Path:         "/movies/movie.mp4",
				Size:         int64(len(fileContent)),
				SmbRoot:      "nas",
				MimeType:     &mimeType,
				LastModified: time.Now(),
			},
		},
	}
	mockReader := newMockReadSeekCloser(fileContent)
	fsClient := &mockFSClient{reader: mockReader, protocol: "smb"}
	factory := &mockClientFactory{client: fsClient}

	handler := NewStreamHandler(catalog, db, factory, logger)
	router := setupStreamRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stream/1", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "video/mp4", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "movie.mp4")
	assert.Equal(t, fileContent, w.Body.Bytes())
	assert.True(t, mockReader.closed)
}

func TestStreamHandler_SuccessfulStream_NonSeekable(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	db := setupStreamTestDB(t)
	defer db.Close()

	insertStorageRoot(t, db, "ftp-server", "ftp", "192.168.0.100", 21, "/media", "user", "pass")

	fileContent := []byte("fake audio content for testing non-seekable stream")

	catalog := &streamMockCatalogService{
		files: map[string]*models.FileInfo{
			"2": {
				ID:      2,
				Name:    "song.mp3",
				Path:    "/music/song.mp3",
				Size:    int64(len(fileContent)),
				SmbRoot: "ftp-server",
			},
		},
	}
	mockReader := newMockReadCloser(fileContent)
	fsClient := &mockFSClient{reader: mockReader, protocol: "ftp"}
	factory := &mockClientFactory{client: fsClient}

	handler := NewStreamHandler(catalog, db, factory, logger)
	router := setupStreamRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stream/2", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"))
	assert.Equal(t, fmt.Sprintf("%d", len(fileContent)), w.Header().Get("Content-Length"))
	assert.Equal(t, fileContent, w.Body.Bytes())
	assert.True(t, mockReader.closed)
}

func TestStreamHandler_RangeRequest_Seekable(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	db := setupStreamTestDB(t)
	defer db.Close()

	insertStorageRoot(t, db, "local-store", "local", "", 0, "/mnt/data", "", "")

	fileContent := []byte("0123456789abcdef") // 16 bytes
	mimeType := "video/mp4"

	catalog := &streamMockCatalogService{
		files: map[string]*models.FileInfo{
			"3": {
				ID:           3,
				Name:         "clip.mp4",
				Path:         "/videos/clip.mp4",
				Size:         int64(len(fileContent)),
				SmbRoot:      "local-store",
				MimeType:     &mimeType,
				LastModified: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}
	mockReader := newMockReadSeekCloser(fileContent)
	fsClient := &mockFSClient{reader: mockReader, protocol: "local"}
	factory := &mockClientFactory{client: fsClient}

	handler := NewStreamHandler(catalog, db, factory, logger)
	router := setupStreamRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stream/3", nil)
	req.Header.Set("Range", "bytes=5-10")
	router.ServeHTTP(w, req)

	// http.ServeContent should return 206 Partial Content
	assert.Equal(t, http.StatusPartialContent, w.Code)
	assert.Contains(t, w.Header().Get("Content-Range"), "bytes 5-10/16")
	assert.Equal(t, "56789a", w.Body.String())
}

func TestStreamHandler_DownloadQueryParam(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	db := setupStreamTestDB(t)
	defer db.Close()

	insertStorageRoot(t, db, "nas", "smb", "192.168.0.241", 445, "media", "user", "pass")

	fileContent := []byte("test content")
	mimeType := "image/jpeg"

	catalog := &streamMockCatalogService{
		files: map[string]*models.FileInfo{
			"4": {
				ID:           4,
				Name:         "photo.jpg",
				Path:         "/photos/photo.jpg",
				Size:         int64(len(fileContent)),
				SmbRoot:      "nas",
				MimeType:     &mimeType,
				LastModified: time.Now(),
			},
		},
	}
	mockReader := newMockReadSeekCloser(fileContent)
	fsClient := &mockFSClient{reader: mockReader, protocol: "smb"}
	factory := &mockClientFactory{client: fsClient}

	handler := NewStreamHandler(catalog, db, factory, logger)
	router := setupStreamRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stream/4?download=true", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, w.Header().Get("Content-Disposition"), "photo.jpg")
}

func TestStreamHandler_MimeTypeDetection(t *testing.T) {
	tests := []struct {
		name         string
		fileName     string
		storedMime   *string
		expectedMime string
	}{
		{"MP4 from stored mime", "video.mp4", strPtr("video/mp4"), "video/mp4"},
		{"MKV from extension", "video.mkv", nil, "video/x-matroska"},
		{"MP3 from extension", "song.mp3", nil, "audio/mpeg"},
		{"FLAC from extension", "song.flac", nil, "audio/flac"},
		{"JPEG from extension", "photo.jpg", nil, "image/jpeg"},
		{"PNG from extension", "image.png", nil, "image/png"},
		{"PDF from extension", "doc.pdf", nil, "application/pdf"},
		{"Unknown from extension", "file.xyz", nil, "application/octet-stream"},
		{"AVI from extension", "movie.avi", nil, "video/x-msvideo"},
		{"WAV from extension", "sound.wav", nil, "audio/wav"},
		{"WebM from extension", "clip.webm", nil, "video/webm"},
		{"EPUB from extension", "book.epub", nil, "application/epub+zip"},
		{"MOV from extension", "clip.mov", nil, "video/quicktime"},
		{"AAC from extension", "track.aac", nil, "audio/aac"},
		{"OGG from extension", "track.ogg", nil, "audio/ogg"},
		{"GIF from extension", "anim.gif", nil, "image/gif"},
		{"WebP from extension", "img.webp", nil, "image/webp"},
		{"SVG from extension", "icon.svg", nil, "image/svg+xml"},
		{"BMP from extension", "img.bmp", nil, "image/bmp"},
		{"CBZ from extension", "comic.cbz", nil, "application/x-cbz"},
		{"CBR from extension", "comic.cbr", nil, "application/x-cbr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info := &models.FileInfo{
				Name:     tt.fileName,
				MimeType: tt.storedMime,
			}
			result := detectContentType(info)
			assert.Equal(t, tt.expectedMime, result)
		})
	}
}

func TestStreamHandler_StorageRootToSettings_AllProtocols(t *testing.T) {
	t.Run("SMB", func(t *testing.T) {
		root := &root_models.StorageRoot{
			Protocol: "smb",
			Host:     strPtr("192.168.0.241"),
			Port:     intPtr(445),
			Path:     strPtr("media"),
			Username: strPtr("user"),
			Password: strPtr("pass"),
			Domain:   strPtr("WORKGROUP"),
		}
		settings := storageRootToSettings(root)
		assert.Equal(t, "192.168.0.241", settings["host"])
		assert.Equal(t, 445, settings["port"])
		assert.Equal(t, "media", settings["share"])
		assert.Equal(t, "user", settings["username"])
		assert.Equal(t, "pass", settings["password"])
		assert.Equal(t, "WORKGROUP", settings["domain"])
	})

	t.Run("FTP", func(t *testing.T) {
		root := &root_models.StorageRoot{
			Protocol: "ftp",
			Host:     strPtr("ftp.example.com"),
			Port:     intPtr(21),
			Username: strPtr("ftpuser"),
			Password: strPtr("ftppass"),
		}
		settings := storageRootToSettings(root)
		assert.Equal(t, "ftp.example.com", settings["host"])
		assert.Equal(t, 21, settings["port"])
		assert.Equal(t, "ftpuser", settings["username"])
		assert.Equal(t, "ftppass", settings["password"])
	})

	t.Run("NFS", func(t *testing.T) {
		root := &root_models.StorageRoot{
			Protocol:   "nfs",
			Host:       strPtr("nfs.example.com"),
			Path:       strPtr("/export/media"),
			MountPoint: strPtr("/mnt/nfs"),
			Options:    strPtr("vers=3"),
		}
		settings := storageRootToSettings(root)
		assert.Equal(t, "nfs.example.com", settings["host"])
		assert.Equal(t, "/export/media", settings["path"])
		assert.Equal(t, "/mnt/nfs", settings["mount_point"])
		assert.Equal(t, "vers=3", settings["options"])
	})

	t.Run("WebDAV", func(t *testing.T) {
		root := &root_models.StorageRoot{
			Protocol: "webdav",
			URL:      strPtr("https://dav.example.com/media"),
			Username: strPtr("davuser"),
			Password: strPtr("davpass"),
		}
		settings := storageRootToSettings(root)
		assert.Equal(t, "https://dav.example.com/media", settings["url"])
		assert.Equal(t, "davuser", settings["username"])
		assert.Equal(t, "davpass", settings["password"])
	})

	t.Run("Local", func(t *testing.T) {
		root := &root_models.StorageRoot{
			Protocol: "local",
			Path:     strPtr("/mnt/data"),
		}
		settings := storageRootToSettings(root)
		assert.Equal(t, "/mnt/data", settings["base_path"])
	})

	t.Run("NilFields", func(t *testing.T) {
		root := &root_models.StorageRoot{
			Protocol: "smb",
		}
		settings := storageRootToSettings(root)
		assert.Empty(t, settings)
	})
}

// --- SeekableClient Range Request Tests ---

func TestStreamHandler_SeekableClient_FullStream(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	db := setupStreamTestDB(t)
	defer db.Close()

	insertStorageRoot(t, db, "nas", "smb", "192.168.0.241", 445, "media", "user", "pass")

	fileContent := []byte("fake video content for seekable client test")
	mimeType := "video/mp4"

	catalog := &streamMockCatalogService{
		files: map[string]*models.FileInfo{
			"10": {
				ID:           10,
				Name:         "seekable.mp4",
				Path:         "/movies/seekable.mp4",
				Size:         int64(len(fileContent)),
				SmbRoot:      "nas",
				MimeType:     &mimeType,
				LastModified: time.Now(),
			},
		},
	}
	seekReader := newMockReadSeekCloser(fileContent)
	fsClient := &mockSeekableFSClient{
		mockFSClient:   mockFSClient{protocol: "smb"},
		seekableReader: seekReader,
	}
	factory := &mockClientFactory{client: fsClient}

	handler := NewStreamHandler(catalog, db, factory, logger)
	router := setupStreamRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stream/10", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "video/mp4", w.Header().Get("Content-Type"))
	assert.Equal(t, "bytes", w.Header().Get("Accept-Ranges"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "seekable.mp4")
	assert.Equal(t, fileContent, w.Body.Bytes())
}

func TestStreamHandler_SeekableClient_RangeRequest_MidFile(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	db := setupStreamTestDB(t)
	defer db.Close()

	insertStorageRoot(t, db, "nas", "smb", "192.168.0.241", 445, "media", "user", "pass")

	// 32 bytes: "0123456789abcdef0123456789ABCDEF"
	fileContent := []byte("0123456789abcdef0123456789ABCDEF")
	mimeType := "video/mp4"

	catalog := &streamMockCatalogService{
		files: map[string]*models.FileInfo{
			"11": {
				ID:           11,
				Name:         "range-test.mp4",
				Path:         "/movies/range-test.mp4",
				Size:         int64(len(fileContent)),
				SmbRoot:      "nas",
				MimeType:     &mimeType,
				LastModified: time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			},
		},
	}
	seekReader := newMockReadSeekCloser(fileContent)
	fsClient := &mockSeekableFSClient{
		mockFSClient:   mockFSClient{protocol: "smb"},
		seekableReader: seekReader,
	}
	factory := &mockClientFactory{client: fsClient}

	handler := NewStreamHandler(catalog, db, factory, logger)
	router := setupStreamRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stream/11", nil)
	req.Header.Set("Range", "bytes=10-19")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusPartialContent, w.Code)
	assert.Contains(t, w.Header().Get("Content-Range"), "bytes 10-19/32")
	assert.Equal(t, "abcdef0123", w.Body.String())
}

func TestStreamHandler_SeekableClient_RangeRequest_EndOfFile(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	db := setupStreamTestDB(t)
	defer db.Close()

	insertStorageRoot(t, db, "nas", "smb", "192.168.0.241", 445, "media", "user", "pass")

	fileContent := []byte("0123456789abcdef") // 16 bytes
	mimeType := "video/mp4"

	catalog := &streamMockCatalogService{
		files: map[string]*models.FileInfo{
			"12": {
				ID:           12,
				Name:         "end-range.mp4",
				Path:         "/movies/end-range.mp4",
				Size:         int64(len(fileContent)),
				SmbRoot:      "nas",
				MimeType:     &mimeType,
				LastModified: time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC),
			},
		},
	}
	seekReader := newMockReadSeekCloser(fileContent)
	fsClient := &mockSeekableFSClient{
		mockFSClient:   mockFSClient{protocol: "smb"},
		seekableReader: seekReader,
	}
	factory := &mockClientFactory{client: fsClient}

	handler := NewStreamHandler(catalog, db, factory, logger)
	router := setupStreamRouter(handler)

	// Request last 6 bytes using suffix range (bytes=-6)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stream/12", nil)
	req.Header.Set("Range", "bytes=-6")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusPartialContent, w.Code)
	assert.Contains(t, w.Header().Get("Content-Range"), "bytes 10-15/16")
	assert.Equal(t, "abcdef", w.Body.String())
}

func TestStreamHandler_SeekableClient_OpenSeekableError(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	db := setupStreamTestDB(t)
	defer db.Close()

	insertStorageRoot(t, db, "nas", "smb", "192.168.0.241", 445, "media", "user", "pass")

	catalog := &streamMockCatalogService{
		files: map[string]*models.FileInfo{
			"13": {
				ID:      13,
				Name:    "error.mp4",
				Path:    "/movies/error.mp4",
				Size:    1000,
				SmbRoot: "nas",
			},
		},
	}
	fsClient := &mockSeekableFSClient{
		mockFSClient: mockFSClient{protocol: "smb"},
		openSeekErr:  fmt.Errorf("SMB seek open failed"),
	}
	factory := &mockClientFactory{client: fsClient}

	handler := NewStreamHandler(catalog, db, factory, logger)
	router := setupStreamRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stream/13", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var resp map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, "File not accessible on storage", resp["error"])
}

func TestStreamHandler_NonSeekableClient_AcceptRangesNone(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	db := setupStreamTestDB(t)
	defer db.Close()

	insertStorageRoot(t, db, "ftp-server", "ftp", "192.168.0.100", 21, "/media", "user", "pass")

	fileContent := []byte("non-seekable stream data")

	catalog := &streamMockCatalogService{
		files: map[string]*models.FileInfo{
			"14": {
				ID:      14,
				Name:    "stream.mp3",
				Path:    "/music/stream.mp3",
				Size:    int64(len(fileContent)),
				SmbRoot: "ftp-server",
			},
		},
	}
	// mockFSClient does NOT implement SeekableClient, and mockReadCloser
	// does NOT implement io.ReadSeeker — so this exercises the no-seek fallback.
	mockReader := newMockReadCloser(fileContent)
	fsClient := &mockFSClient{reader: mockReader, protocol: "ftp"}
	factory := &mockClientFactory{client: fsClient}

	handler := NewStreamHandler(catalog, db, factory, logger)
	router := setupStreamRouter(handler)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/stream/14", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "none", w.Header().Get("Accept-Ranges"))
	assert.Equal(t, fmt.Sprintf("%d", len(fileContent)), w.Header().Get("Content-Length"))
	assert.Equal(t, fileContent, w.Body.Bytes())
}

// --- Helpers ---

func strPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}
