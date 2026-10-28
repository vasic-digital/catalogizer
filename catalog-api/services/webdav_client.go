package services

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/studio-b12/gowebdav"
)

type WebDAVClient struct {
	client   *gowebdav.Client
	baseURL  string
	username string
	password string
}

type WebDAVFile struct {
	Path    string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

func NewWebDAVClient(url, username, password string) *WebDAVClient {
	client := gowebdav.NewClient(url, username, password)

	return &WebDAVClient{
		client:   client,
		baseURL:  url,
		username: username,
		password: password,
	}
}

func (c *WebDAVClient) TestConnection() error {
	_, err := c.client.ReadDir("/")
	if err != nil {
		return fmt.Errorf("WebDAV connection test failed: %w", err)
	}
	return nil
}

// maxListDepth bounds ListFiles recursion: a hostile server can otherwise answer every
// PROPFIND with a directory entry that points back at itself and overflow the stack.
const maxListDepth = 32

func (c *WebDAVClient) ListFiles(remotePath string) ([]*WebDAVFile, error) {
	return c.listFiles(remotePath, 0)
}

func (c *WebDAVClient) listFiles(remotePath string, depth int) ([]*WebDAVFile, error) {
	files, err := c.client.ReadDir(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	var webdavFiles []*WebDAVFile
	for _, file := range files {
		webdavFile := &WebDAVFile{
			Path:    filepath.Join(remotePath, file.Name()),
			Size:    file.Size(),
			ModTime: file.ModTime(),
			IsDir:   file.IsDir(),
		}
		webdavFiles = append(webdavFiles, webdavFile)

		// Recursively list subdirectories if needed
		if file.IsDir() && depth < maxListDepth && file.Name() != "." && file.Name() != ".." {
			subFiles, err := c.listFiles(webdavFile.Path, depth+1)
			if err == nil {
				webdavFiles = append(webdavFiles, subFiles...)
			}
		}
	}

	return webdavFiles, nil
}

func (c *WebDAVClient) UploadFile(localPath, remotePath string) error {
	// Ensure remote directory exists
	remoteDir := filepath.Dir(remotePath)
	if err := c.client.MkdirAll(remoteDir, 0755); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Upload to WebDAV
	err = c.client.WriteStream(remotePath, localFile, 0644)
	if err != nil {
		return fmt.Errorf("failed to upload file: %w", err)
	}

	return nil
}

func (c *WebDAVClient) DownloadFile(remotePath, localPath string) error {
	// Create local directory if it doesn't exist
	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %w", err)
	}

	// Open remote file
	reader, err := c.client.ReadStream(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer reader.Close()

	// Create local file
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	// Copy data
	_, err = io.Copy(localFile, reader)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}

	return nil
}

func (c *WebDAVClient) GetModTime(remotePath string) (time.Time, error) {
	info, err := c.client.Stat(remotePath)
	if err != nil {
		return time.Time{}, fmt.Errorf("failed to get file info: %w", err)
	}

	return info.ModTime(), nil
}

func (c *WebDAVClient) DeleteFile(remotePath string) error {
	err := c.client.Remove(remotePath)
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}

	return nil
}

func (c *WebDAVClient) MoveFile(sourcePath, destPath string) error {
	err := c.client.Rename(sourcePath, destPath, false)
	if err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	return nil
}

func (c *WebDAVClient) CopyFile(sourcePath, destPath string) error {
	err := c.client.Copy(sourcePath, destPath, false)
	if err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

func (c *WebDAVClient) CreateDirectory(remotePath string) error {
	err := c.client.Mkdir(remotePath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	return nil
}

func (c *WebDAVClient) FileExists(remotePath string) bool {
	_, err := c.client.Stat(remotePath)
	return err == nil
}

func (c *WebDAVClient) GetFileSize(remotePath string) (int64, error) {
	info, err := c.client.Stat(remotePath)
	if err != nil {
		return 0, fmt.Errorf("failed to get file info: %w", err)
	}

	return info.Size(), nil
}

func (c *WebDAVClient) GetQuota() (*WebDAVQuota, error) {
	// This would require parsing PROPFIND response for quota information
	// For now, return a placeholder
	return &WebDAVQuota{
		Used:      0,
		Available: -1, // Unlimited
	}, nil
}

type WebDAVQuota struct {
	Used      int64 `json:"used"`
	Available int64 `json:"available"` // -1 for unlimited
}

// Batch operations for efficiency

func (c *WebDAVClient) UploadBatch(files []FileTransfer) (*BatchResult, error) {
	result := &BatchResult{
		Total:     len(files),
		Succeeded: 0,
		Failed:    0,
		Errors:    make([]string, 0),
	}

	for _, file := range files {
		err := c.UploadFile(file.LocalPath, file.RemotePath)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", file.LocalPath, err))
		} else {
			result.Succeeded++
		}
	}

	return result, nil
}

func (c *WebDAVClient) DownloadBatch(files []FileTransfer) (*BatchResult, error) {
	result := &BatchResult{
		Total:     len(files),
		Succeeded: 0,
		Failed:    0,
		Errors:    make([]string, 0),
	}

	for _, file := range files {
		err := c.DownloadFile(file.RemotePath, file.LocalPath)
		if err != nil {
			result.Failed++
			result.Errors = append(result.Errors, fmt.Sprintf("%s: %v", file.RemotePath, err))
		} else {
			result.Succeeded++
		}
	}

	return result, nil
}

type FileTransfer struct {
	LocalPath  string
	RemotePath string
}

type BatchResult struct {
	Total     int      `json:"total"`
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors"`
}

// Sync-specific methods

func (c *WebDAVClient) SyncDirectory(localPath, remotePath string, direction string) (*SyncResult, error) {
	result := &SyncResult{
		UploadedFiles:   0,
		DownloadedFiles: 0,
		SkippedFiles:    0,
		FailedFiles:     0,
		Errors:          make([]string, 0),
	}

	switch direction {
	case "upload":
		return c.syncUpload(localPath, remotePath, result)
	case "download":
		return c.syncDownload(localPath, remotePath, result)
	case "bidirectional":
		// First upload, then download
		result, err := c.syncUpload(localPath, remotePath, result)
		if err != nil {
			return result, err
		}
		return c.syncDownload(localPath, remotePath, result)
	default:
		return result, fmt.Errorf("invalid sync direction: %s", direction)
	}
}

func (c *WebDAVClient) syncUpload(localPath, remotePath string, result *SyncResult) (*SyncResult, error) {
	err := filepath.Walk(localPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Skip hidden files and temporary files
		if strings.HasPrefix(info.Name(), ".") || strings.HasSuffix(info.Name(), ".tmp") {
			result.SkippedFiles++
			return nil
		}

		relPath, err := filepath.Rel(localPath, path)
		if err != nil {
			result.FailedFiles++
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to get relative path for %s: %v", path, err))
			return nil
		}

		remoteFilePath := filepath.Join(remotePath, relPath)
		remoteFilePath = filepath.ToSlash(remoteFilePath) // Convert to forward slashes for WebDAV

		// Check if remote file exists and compare modification times
		remoteModTime, err := c.GetModTime(remoteFilePath)
		if err == nil {
			// Remote file exists, check if local is newer
			if !info.ModTime().After(remoteModTime) {
				result.SkippedFiles++
				return nil
			}
		}

		// Upload the file
		err = c.UploadFile(path, remoteFilePath)
		if err != nil {
			result.FailedFiles++
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to upload %s: %v", path, err))
		} else {
			result.UploadedFiles++
		}

		return nil
	})

	return result, err
}

func (c *WebDAVClient) syncDownload(localPath, remotePath string, result *SyncResult) (*SyncResult, error) {
	remoteFiles, err := c.ListFiles(remotePath)
	if err != nil {
		return result, fmt.Errorf("failed to list remote files: %w", err)
	}

	for _, remoteFile := range remoteFiles {
		if remoteFile.IsDir {
			continue
		}

		// Skip hidden files and temporary files
		fileName := filepath.Base(remoteFile.Path)
		if strings.HasPrefix(fileName, ".") || strings.HasSuffix(fileName, ".tmp") {
			result.SkippedFiles++
			continue
		}

		relPath, err := filepath.Rel(remotePath, remoteFile.Path)
		if err != nil {
			result.FailedFiles++
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to get relative path for %s: %v", remoteFile.Path, err))
			continue
		}

		localFilePath := filepath.Join(localPath, relPath)

		// Check if local file exists and compare modification times
		localInfo, err := os.Stat(localFilePath)
		if err == nil {
			// Local file exists, check if remote is newer
			if !remoteFile.ModTime.After(localInfo.ModTime()) {
				result.SkippedFiles++
				continue
			}
		}

		// Download the file
		err = c.DownloadFile(remoteFile.Path, localFilePath)
		if err != nil {
			result.FailedFiles++
			result.Errors = append(result.Errors, fmt.Sprintf("Failed to download %s: %v", remoteFile.Path, err))
		} else {
			result.DownloadedFiles++
		}
	}

	return result, nil
}

type SyncResult struct {
	UploadedFiles   int      `json:"uploaded_files"`
	DownloadedFiles int      `json:"downloaded_files"`
	SkippedFiles    int      `json:"skipped_files"`
	FailedFiles     int      `json:"failed_files"`
	Errors          []string `json:"errors"`
}
