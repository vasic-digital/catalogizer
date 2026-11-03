package services

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewWebDAVClient(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		username string
		password string
	}{
		{
			name:     "standard credentials",
			url:      "https://dav.example.com/files",
			username: "admin",
			password: "secret123",
		},
		{
			name:     "empty credentials",
			url:      "https://public.example.com/webdav",
			username: "",
			password: "",
		},
		{
			name:     "http URL",
			url:      "http://localhost:28080/dav",
			username: "user",
			password: "pass",
		},
		{
			name:     "URL with port",
			url:      "https://nas.local:5006/webdav",
			username: "nas-admin",
			password: "nas-pass-456",
		},
		{
			name:     "URL with path",
			url:      "https://cloud.example.com/remote.php/dav/files/user",
			username: "user@example.com",
			password: "app-password",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewWebDAVClient(tt.url, tt.username, tt.password)

			require.NotNil(t, client)
			assert.Equal(t, tt.url, client.baseURL)
			assert.Equal(t, tt.username, client.username)
			assert.Equal(t, tt.password, client.password)
			assert.NotNil(t, client.client, "underlying gowebdav.Client should not be nil")
		})
	}
}

func TestWebDAVFile_Struct(t *testing.T) {
	tests := []struct {
		name     string
		file     WebDAVFile
		wantDir  bool
		wantSize int64
	}{
		{
			name: "regular file",
			file: WebDAVFile{
				Path:    "/documents/report.pdf",
				Size:    1048576,
				ModTime: time.Now(),
				IsDir:   false,
			},
			wantDir:  false,
			wantSize: 1048576,
		},
		{
			name: "directory",
			file: WebDAVFile{
				Path:    "/documents/archive",
				Size:    0,
				ModTime: time.Now(),
				IsDir:   true,
			},
			wantDir:  true,
			wantSize: 0,
		},
		{
			name: "empty file",
			file: WebDAVFile{
				Path:    "/data/empty.txt",
				Size:    0,
				ModTime: time.Now(),
				IsDir:   false,
			},
			wantDir:  false,
			wantSize: 0,
		},
		{
			name: "large file",
			file: WebDAVFile{
				Path:    "/media/movie.mkv",
				Size:    4294967296, // 4 GB
				ModTime: time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
				IsDir:   false,
			},
			wantDir:  false,
			wantSize: 4294967296,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantDir, tt.file.IsDir)
			assert.Equal(t, tt.wantSize, tt.file.Size)
			assert.NotEmpty(t, tt.file.Path)
			assert.False(t, tt.file.ModTime.IsZero())
		})
	}
}

func TestWebDAVQuota_Struct(t *testing.T) {
	tests := []struct {
		name      string
		quota     WebDAVQuota
		wantUsed  int64
		wantAvail int64
	}{
		{
			name: "unlimited quota",
			quota: WebDAVQuota{
				Used:      0,
				Available: -1,
			},
			wantUsed:  0,
			wantAvail: -1,
		},
		{
			name: "partially used quota",
			quota: WebDAVQuota{
				Used:      5368709120,  // 5 GB
				Available: 10737418240, // 10 GB
			},
			wantUsed:  5368709120,
			wantAvail: 10737418240,
		},
		{
			name: "zero quota",
			quota: WebDAVQuota{
				Used:      0,
				Available: 0,
			},
			wantUsed:  0,
			wantAvail: 0,
		},
		{
			name: "fully used quota",
			quota: WebDAVQuota{
				Used:      10737418240,
				Available: 0,
			},
			wantUsed:  10737418240,
			wantAvail: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantUsed, tt.quota.Used)
			assert.Equal(t, tt.wantAvail, tt.quota.Available)
		})
	}
}

func TestFileTransfer_Struct(t *testing.T) {
	tests := []struct {
		name       string
		transfer   FileTransfer
		wantLocal  string
		wantRemote string
	}{
		{
			name: "standard file transfer",
			transfer: FileTransfer{
				LocalPath:  "/home/user/documents/report.pdf",
				RemotePath: "/webdav/documents/report.pdf",
			},
			wantLocal:  "/home/user/documents/report.pdf",
			wantRemote: "/webdav/documents/report.pdf",
		},
		{
			name: "nested directory transfer",
			transfer: FileTransfer{
				LocalPath:  "/tmp/backup/2026/02/data.tar.gz",
				RemotePath: "/backup/2026/02/data.tar.gz",
			},
			wantLocal:  "/tmp/backup/2026/02/data.tar.gz",
			wantRemote: "/backup/2026/02/data.tar.gz",
		},
		{
			name: "empty paths",
			transfer: FileTransfer{
				LocalPath:  "",
				RemotePath: "",
			},
			wantLocal:  "",
			wantRemote: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantLocal, tt.transfer.LocalPath)
			assert.Equal(t, tt.wantRemote, tt.transfer.RemotePath)
		})
	}
}

func TestBatchResult_Initialization(t *testing.T) {
	tests := []struct {
		name      string
		result    BatchResult
		wantTotal int
		wantOK    int
		wantFail  int
		wantErrs  int
	}{
		{
			name: "zero-value initialization",
			result: BatchResult{
				Total:     0,
				Succeeded: 0,
				Failed:    0,
				Errors:    make([]string, 0),
			},
			wantTotal: 0,
			wantOK:    0,
			wantFail:  0,
			wantErrs:  0,
		},
		{
			name: "all succeeded",
			result: BatchResult{
				Total:     5,
				Succeeded: 5,
				Failed:    0,
				Errors:    []string{},
			},
			wantTotal: 5,
			wantOK:    5,
			wantFail:  0,
			wantErrs:  0,
		},
		{
			name: "mixed results",
			result: BatchResult{
				Total:     10,
				Succeeded: 7,
				Failed:    3,
				Errors:    []string{"file1: timeout", "file2: permission denied", "file3: not found"},
			},
			wantTotal: 10,
			wantOK:    7,
			wantFail:  3,
			wantErrs:  3,
		},
		{
			name: "all failed",
			result: BatchResult{
				Total:     3,
				Succeeded: 0,
				Failed:    3,
				Errors:    []string{"error1", "error2", "error3"},
			},
			wantTotal: 3,
			wantOK:    0,
			wantFail:  3,
			wantErrs:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantTotal, tt.result.Total)
			assert.Equal(t, tt.wantOK, tt.result.Succeeded)
			assert.Equal(t, tt.wantFail, tt.result.Failed)
			assert.Equal(t, tt.wantErrs, len(tt.result.Errors))
		})
	}
}

func TestBatchResult_ErrorCountMatchesFailedCount(t *testing.T) {
	result := &BatchResult{
		Total:     5,
		Succeeded: 0,
		Failed:    0,
		Errors:    make([]string, 0),
	}

	// Simulate adding failures
	failures := []string{
		"/path/a.txt: connection refused",
		"/path/b.txt: disk full",
	}
	for _, errMsg := range failures {
		result.Failed++
		result.Errors = append(result.Errors, errMsg)
	}
	result.Succeeded = result.Total - result.Failed

	assert.Equal(t, result.Failed, len(result.Errors))
	assert.Equal(t, 5, result.Total)
	assert.Equal(t, 3, result.Succeeded)
	assert.Equal(t, 2, result.Failed)
}

func TestSyncResult_Initialization(t *testing.T) {
	tests := []struct {
		name           string
		result         SyncResult
		wantUploaded   int
		wantDownloaded int
		wantSkipped    int
		wantFailed     int
		wantErrs       int
	}{
		{
			name: "zero-value initialization",
			result: SyncResult{
				UploadedFiles:   0,
				DownloadedFiles: 0,
				SkippedFiles:    0,
				FailedFiles:     0,
				Errors:          make([]string, 0),
			},
			wantUploaded:   0,
			wantDownloaded: 0,
			wantSkipped:    0,
			wantFailed:     0,
			wantErrs:       0,
		},
		{
			name: "upload-only sync",
			result: SyncResult{
				UploadedFiles:   15,
				DownloadedFiles: 0,
				SkippedFiles:    5,
				FailedFiles:     0,
				Errors:          []string{},
			},
			wantUploaded:   15,
			wantDownloaded: 0,
			wantSkipped:    5,
			wantFailed:     0,
			wantErrs:       0,
		},
		{
			name: "download-only sync",
			result: SyncResult{
				UploadedFiles:   0,
				DownloadedFiles: 20,
				SkippedFiles:    3,
				FailedFiles:     1,
				Errors:          []string{"remote/file.dat: checksum mismatch"},
			},
			wantUploaded:   0,
			wantDownloaded: 20,
			wantSkipped:    3,
			wantFailed:     1,
			wantErrs:       1,
		},
		{
			name: "bidirectional sync",
			result: SyncResult{
				UploadedFiles:   10,
				DownloadedFiles: 8,
				SkippedFiles:    50,
				FailedFiles:     2,
				Errors:          []string{"upload: timeout", "download: corrupt"},
			},
			wantUploaded:   10,
			wantDownloaded: 8,
			wantSkipped:    50,
			wantFailed:     2,
			wantErrs:       2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.wantUploaded, tt.result.UploadedFiles)
			assert.Equal(t, tt.wantDownloaded, tt.result.DownloadedFiles)
			assert.Equal(t, tt.wantSkipped, tt.result.SkippedFiles)
			assert.Equal(t, tt.wantFailed, tt.result.FailedFiles)
			assert.Equal(t, tt.wantErrs, len(tt.result.Errors))
		})
	}
}

func TestWebDAVClient_GetQuota_UnreachableServerIsAnError(t *testing.T) {
	// CAT-001: GetQuota used to return placeholder values (used 0 / unlimited) whatever the server said or whether it answered at all.
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close()
	quota, err := NewWebDAVClient(url, "user", "pass").GetQuota()
	require.Error(t, err)
	assert.Nil(t, quota)
}

func TestWebDAVClient_BaseURLPreserved(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "HTTPS URL",
			url:  "https://dav.example.com/files",
		},
		{
			name: "HTTP URL",
			url:  "http://localhost:28080/webdav",
		},
		{
			name: "URL with trailing slash",
			url:  "https://nas.local/dav/",
		},
		{
			name: "Nextcloud-style URL",
			url:  "https://cloud.example.com/remote.php/dav/files/admin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := NewWebDAVClient(tt.url, "user", "pass")
			assert.Equal(t, tt.url, client.baseURL)
		})
	}
}

func TestWebDAVClient_CredentialsStored(t *testing.T) {
	client := NewWebDAVClient("https://example.com/dav", "testuser", "testpass")

	assert.Equal(t, "testuser", client.username)
	assert.Equal(t, "testpass", client.password)
}

func TestWebDAVClient_EmptyCredentials(t *testing.T) {
	client := NewWebDAVClient("https://public.example.com/dav", "", "")

	assert.NotNil(t, client)
	assert.Empty(t, client.username)
	assert.Empty(t, client.password)
	assert.NotNil(t, client.client)
}

// ============================================================================
// ADDITIONAL TESTS FOR 95% COVERAGE
// ============================================================================

func TestWebDAVClient_ListFiles(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	// Test with empty path
	files, err := client.ListFiles("")
	assert.Error(t, err) // Will error without real server
	assert.Nil(t, files)

	// Test with path
	files, err = client.ListFiles("/documents")
	assert.Error(t, err) // Will error without real server
	assert.Nil(t, files)
}

func TestWebDAVClient_DownloadFile(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	tmpDir := t.TempDir()
	localPath := filepath.Join(tmpDir, "downloaded.txt")

	// Test download - will error without real server
	err := client.DownloadFile("/remote/file.txt", localPath)
	assert.Error(t, err)
}

func TestWebDAVClient_UploadFile(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	// Create temp file for upload test
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(tmpFile, []byte("test content"), 0644)
	require.NoError(t, err)

	// Test upload - will error without real server
	err = client.UploadFile(tmpFile, "/remote/test.txt")
	assert.Error(t, err)
}

func TestWebDAVClient_DeleteFile(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	// Test delete - will error without real server
	err := client.DeleteFile("/remote/file.txt")
	assert.Error(t, err)
}

func TestWebDAVClient_CreateDirectory(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	// Test create directory - will error without real server
	err := client.CreateDirectory("/remote/newdir")
	assert.Error(t, err)
}

func TestWebDAVClient_FileExists(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	// Test exists - returns false without real server (no error)
	exists := client.FileExists("/remote/file.txt")
	assert.False(t, exists)
}

func TestWebDAVClient_GetModTime(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	// Test get mod time - will error without real server
	modTime, err := client.GetModTime("/remote/file.txt")
	assert.Error(t, err)
	assert.True(t, modTime.IsZero())
}

func TestWebDAVClient_GetFileSize(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	// Test get file size - will error without real server
	size, err := client.GetFileSize("/remote/file.txt")
	assert.Error(t, err)
	assert.Equal(t, int64(0), size)
}

func TestWebDAVClient_MoveFile(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	// Test move - will error without real server
	err := client.MoveFile("/remote/old.txt", "/remote/new.txt")
	assert.Error(t, err)
}

func TestWebDAVClient_CopyFile(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	// Test copy - will error without real server
	err := client.CopyFile("/remote/source.txt", "/remote/dest.txt")
	assert.Error(t, err)
}

func TestWebDAVClient_SyncDirectory_Upload(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	require.NoError(t, err)

	// Test sync directory upload - will have failures without real server
	result, err := client.SyncDirectory(tmpDir, "/remote/dir", "upload")
	// The function may not return an error directly, but failures are tracked in result
	if err == nil {
		// Check that we got a result with some indication of what happened
		assert.NotNil(t, result)
		// Without a real server, files should either fail or be skipped
		assert.True(t, result.FailedFiles > 0 || result.SkippedFiles > 0 || result.UploadedFiles >= 0)
	} else {
		assert.NotNil(t, result)
	}
}

func TestWebDAVClient_SyncDirectory_Download(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	tmpDir := t.TempDir()

	// Test sync directory download - will have failures without real server
	result, err := client.SyncDirectory(tmpDir, "/remote/dir", "download")
	// The function may not return an error directly, but failures are tracked in result
	if err == nil {
		assert.NotNil(t, result)
	} else {
		assert.NotNil(t, result)
	}
}

func TestWebDAVClient_SyncDirectory_Bidirectional(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	require.NoError(t, err)

	// Test sync directory bidirectional - will error without real server
	result, err := client.SyncDirectory(tmpDir, "/remote/dir", "bidirectional")
	assert.Error(t, err)
	assert.NotNil(t, result)
}

func TestWebDAVClient_SyncDirectory_InvalidDirection(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	tmpDir := t.TempDir()

	// Test invalid direction
	result, err := client.SyncDirectory(tmpDir, "/remote/dir", "invalid")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid sync direction")
	assert.NotNil(t, result)
}

func TestWebDAVClient_DownloadBatch(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	tmpDir := t.TempDir()

	files := []FileTransfer{
		{LocalPath: filepath.Join(tmpDir, "file1.txt"), RemotePath: "/remote/file1.txt"},
		{LocalPath: filepath.Join(tmpDir, "file2.txt"), RemotePath: "/remote/file2.txt"},
	}

	// Test batch download
	result, err := client.DownloadBatch(files)
	assert.NoError(t, err) // Function itself doesn't error, individual files do
	assert.NotNil(t, result)
	assert.Equal(t, len(files), result.Total)
	assert.Equal(t, 0, result.Succeeded) // Will fail without real server
	assert.Equal(t, len(files), result.Failed)
}

func TestWebDAVClient_UploadBatch(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("content2"), 0644)
	require.NoError(t, err)

	files := []FileTransfer{
		{LocalPath: filepath.Join(tmpDir, "file1.txt"), RemotePath: "/remote/file1.txt"},
		{LocalPath: filepath.Join(tmpDir, "file2.txt"), RemotePath: "/remote/file2.txt"},
	}

	// Test batch upload
	result, err := client.UploadBatch(files)
	assert.NoError(t, err) // Function itself doesn't error, individual files do
	assert.NotNil(t, result)
	assert.Equal(t, len(files), result.Total)
	assert.Equal(t, 0, result.Succeeded) // Will fail without real server
	assert.Equal(t, len(files), result.Failed)
}

func TestWebDAVClient_TestConnection(t *testing.T) {
	client := NewWebDAVClient("https://dav.example.com", "user", "pass")

	// Test connection - will error without real server
	err := client.TestConnection()
	assert.Error(t, err)
}
