package handlers

import (
	"archive/tar"
	"archive/zip"
	"catalogizer/internal/models"
	"catalogizer/internal/services"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

type DownloadHandler struct {
	catalogService *services.CatalogService
	smbService     *services.SMBService
	tempDir        string
	maxArchiveSize int64
	chunkSize      int
	logger         *zap.Logger
	// nonSMBFileHandler serves files whose storage root is not a configured SMB
	// host (local, NFS, WebDAV, FTP roots); wired to the stream handler in main.
	nonSMBFileHandler gin.HandlerFunc
}

// SetNonSMBFileHandler sets the handler used by DownloadFile for storage roots
// that are not configured SMB hosts.
func (h *DownloadHandler) SetNonSMBFileHandler(fn gin.HandlerFunc) { h.nonSMBFileHandler = fn }

func (h *DownloadHandler) isSMBRoot(name string) bool {
	if h.smbService == nil {
		return false
	}
	for _, n := range h.smbService.GetHosts() {
		if n == name {
			return true
		}
	}
	return false
}

func NewDownloadHandler(catalogService *services.CatalogService, smbService *services.SMBService, tempDir string, maxArchiveSize int64, chunkSize int, logger *zap.Logger) *DownloadHandler {
	return &DownloadHandler{
		catalogService: catalogService,
		smbService:     smbService,
		tempDir:        tempDir,
		maxArchiveSize: maxArchiveSize,
		chunkSize:      chunkSize,
		logger:         logger,
	}
}

// @Summary Download a single file
// @Description Download a file from the catalog
// @Tags download
// @Param id path int true "File ID"
// @Produce application/octet-stream
// @Success 200 {file} binary
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/download/file/{id} [get]
func (h *DownloadHandler) DownloadFile(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid file ID"})
		return
	}

	fileInfo, err := h.catalogService.GetFileInfo(strconv.FormatInt(id, 10))
	if err != nil {
		h.logger.Error("Failed to get file info", zap.Int64("id", id), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get file information"})
		return
	}

	if fileInfo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "File not found"})
		return
	}

	if fileInfo.IsDirectory {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot download directory as single file"})
		return
	}

	if !h.isSMBRoot(fileInfo.SmbRoot) && h.nonSMBFileHandler != nil {
		h.nonSMBFileHandler(c)
		return
	}

	// Stream directly from SMB to HTTP response with Range support
	reader, _, err := h.smbService.StreamFile(fileInfo.SmbRoot, fileInfo.Path)
	if err != nil {
		h.logger.Error("Failed to open SMB stream", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to stream file"})
		return
	}
	defer reader.Close()

	// Determine content type from file extension
	contentType := "application/octet-stream"
	if fileInfo.MimeType != nil && *fileInfo.MimeType != "" {
		contentType = *fileInfo.MimeType
	} else {
		ext := strings.ToLower(filepath.Ext(fileInfo.Name))
		switch ext {
		case ".mp4":
			contentType = "video/mp4"
		case ".mkv":
			contentType = "video/x-matroska"
		case ".avi":
			contentType = "video/x-msvideo"
		case ".mp3":
			contentType = "audio/mpeg"
		case ".flac":
			contentType = "audio/flac"
		}
	}

	// Set streaming headers
	disposition := "inline"
	if c.Query("download") == "true" {
		disposition = "attachment"
	}
	c.Header("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, sanitizeContentDisposition(fileInfo.Name)))
	c.Header("Content-Type", contentType)

	// Use http.ServeContent which handles Range requests, If-Modified-Since, etc.
	// This is critical for video players (ExoPlayer) that need seeking support.
	http.ServeContent(c.Writer, c.Request, fileInfo.Name, fileInfo.LastModified, reader)

	h.logger.Info("File streamed successfully", zap.String("file", fileInfo.Name), zap.Int64("id", id))
}

// @Summary Download directory as archive
// @Description Download a directory and its contents as a compressed archive
// @Tags download
// @Param path path string true "Directory path"
// @Param format query string false "Archive format (zip, tar, tar.gz)" default(zip)
// @Produce application/octet-stream
// @Success 200 {file} binary
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/download/directory/{path} [get]
func (h *DownloadHandler) DownloadDirectory(c *gin.Context) {
	path := c.Param("path")
	if path == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Path is required"})
		return
	}

	format := c.DefaultQuery("format", "zip")
	if format != "zip" && format != "tar" && format != "tar.gz" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format. Supported: zip, tar, tar.gz"})
		return
	}

	// Clean and validate path to prevent traversal attacks
	path = strings.TrimPrefix(path, "/")
	path = filepath.Clean(path)
	if path == ".." || strings.HasPrefix(path, "../") || strings.Contains(path, "/../") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid path"})
		return
	}

	// Get directory listing recursively
	files, err := h.getDirectoryContentsRecursive(path)
	if err != nil {
		if errors.Is(err, services.ErrPathNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Directory not found"})
			return
		}
		h.logger.Error("Failed to get directory contents", zap.String("path", path), zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read directory"})
		return
	}

	if len(files) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Directory not found or empty"})
		return
	}

	// Check total size
	var totalSize int64
	for _, file := range files {
		totalSize += file.Size
	}

	if totalSize > h.maxArchiveSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Directory too large for download",
			"total_size": totalSize,
			"max_size":   h.maxArchiveSize,
		})
		return
	}

	// Create archive
	filename := filepath.Base(path) + "." + format
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	switch format {
	case "zip":
		c.Header("Content-Type", "application/zip")
		h.createZipArchive(c.Writer, files)
	case "tar":
		c.Header("Content-Type", "application/x-tar")
		h.createTarArchive(c.Writer, files, false)
	case "tar.gz":
		c.Header("Content-Type", "application/gzip")
		h.createTarArchive(c.Writer, files, true)
	}

	h.logger.Info("Directory downloaded successfully", zap.String("path", path), zap.String("format", format))
}

// @Summary Create archive from multiple files
// @Description Create and download an archive containing specified files
// @Tags download
// @Accept json
// @Param request body models.DownloadRequest true "Download request"
// @Produce application/octet-stream
// @Success 200 {file} binary
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /api/v1/download/archive [post]
func (h *DownloadHandler) DownloadArchive(c *gin.Context) {
	var req models.DownloadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request format"})
		return
	}

	if len(req.Paths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No paths specified"})
		return
	}

	if req.Format == "" {
		req.Format = "zip"
	}

	if req.Format != "zip" && req.Format != "tar" && req.Format != "tar.gz" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid format. Supported: zip, tar, tar.gz"})
		return
	}

	// Get file information for all paths
	var files []models.FileInfo
	var totalSize int64

	for _, path := range req.Paths {
		// This is a simplified implementation - you'd need to implement path-to-ID conversion
		// or modify the catalog service to search by path
		fileList, err := h.getFilesByPath(path, req.SmbRoot)
		if err != nil {
			h.logger.Error("Failed to get files for path", zap.String("path", path), zap.Error(err))
			continue
		}

		for _, file := range fileList {
			totalSize += file.Size
			files = append(files, file)
		}
	}

	if totalSize > h.maxArchiveSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":      "Total size too large for download",
			"total_size": totalSize,
			"max_size":   h.maxArchiveSize,
		})
		return
	}

	// Create archive
	filename := fmt.Sprintf("archive_%d.%s", time.Now().Unix(), req.Format)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	switch req.Format {
	case "zip":
		c.Header("Content-Type", "application/zip")
		h.createZipArchive(c.Writer, files)
	case "tar":
		c.Header("Content-Type", "application/x-tar")
		h.createTarArchive(c.Writer, files, false)
	case "tar.gz":
		c.Header("Content-Type", "application/gzip")
		h.createTarArchive(c.Writer, files, true)
	}

	h.logger.Info("Archive downloaded successfully", zap.Int("file_count", len(files)), zap.String("format", req.Format))
}

func (h *DownloadHandler) createZipArchive(w io.Writer, files []models.FileInfo) error {
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	for _, file := range files {
		if file.IsDirectory {
			continue // Skip directories for now
		}

		// Download file to temp location
		tempFile, err := os.CreateTemp(h.tempDir, "zip_*")
		if err != nil {
			h.logger.Error("Failed to create temp file for zip", zap.Error(err))
			continue
		}

		err = h.smbService.DownloadFile(file.SmbRoot, file.Path, tempFile.Name())
		if err != nil {
			h.logger.Error("Failed to download file for zip", zap.String("path", file.Path), zap.Error(err))
			tempFile.Close()
			os.Remove(tempFile.Name())
			continue
		}

		// Add to zip (sanitize path to prevent Zip Slip)
		zipFile, err := zipWriter.Create(sanitizeArchivePath(file.Path))
		if err != nil {
			h.logger.Error("Failed to create zip entry", zap.String("path", file.Path), zap.Error(err))
			tempFile.Close()
			os.Remove(tempFile.Name())
			continue
		}

		tempFile.Seek(0, 0)
		_, err = io.Copy(zipFile, tempFile)
		tempFile.Close()
		os.Remove(tempFile.Name())

		if err != nil {
			h.logger.Error("Failed to write zip entry", zap.String("path", file.Path), zap.Error(err))
			continue
		}
	}

	return nil
}

func (h *DownloadHandler) createTarArchive(w io.Writer, files []models.FileInfo, compress bool) error {
	var tarWriter *tar.Writer

	if compress {
		gzWriter := gzip.NewWriter(w)
		defer gzWriter.Close()
		tarWriter = tar.NewWriter(gzWriter)
	} else {
		tarWriter = tar.NewWriter(w)
	}
	defer tarWriter.Close()

	for _, file := range files {
		if file.IsDirectory {
			continue // Skip directories for now
		}

		// Download file to temp location
		tempFile, err := os.CreateTemp(h.tempDir, "tar_*")
		if err != nil {
			h.logger.Error("Failed to create temp file for tar", zap.Error(err))
			continue
		}

		err = h.smbService.DownloadFile(file.SmbRoot, file.Path, tempFile.Name())
		if err != nil {
			h.logger.Error("Failed to download file for tar", zap.String("path", file.Path), zap.Error(err))
			tempFile.Close()
			os.Remove(tempFile.Name())
			continue
		}

		// Create tar header (sanitize path to prevent Tar Slip)
		header := &tar.Header{
			Name:    sanitizeArchivePath(file.Path),
			Size:    file.Size,
			Mode:    0644,
			ModTime: file.LastModified,
		}

		err = tarWriter.WriteHeader(header)
		if err != nil {
			h.logger.Error("Failed to write tar header", zap.String("path", file.Path), zap.Error(err))
			tempFile.Close()
			os.Remove(tempFile.Name())
			continue
		}

		tempFile.Seek(0, 0)
		_, err = io.Copy(tarWriter, tempFile)
		tempFile.Close()
		os.Remove(tempFile.Name())

		if err != nil {
			h.logger.Error("Failed to write tar entry", zap.String("path", file.Path), zap.Error(err))
			continue
		}
	}

	return nil
}

// sanitizeArchivePath prevents path traversal (Zip Slip / Tar Slip) attacks
// by removing any ../ components from archive entry names.
func sanitizeArchivePath(p string) string {
	cleaned := filepath.Clean(p)
	cleaned = strings.TrimPrefix(cleaned, "/")
	for strings.HasPrefix(cleaned, "../") {
		cleaned = strings.TrimPrefix(cleaned, "../")
	}
	if cleaned == ".." || cleaned == "" {
		cleaned = "unnamed"
	}
	return cleaned
}

// sanitizeContentDisposition removes characters that could be used for
// header injection in Content-Disposition filenames.
func sanitizeContentDisposition(name string) string {
	name = strings.ReplaceAll(name, "\"", "_")
	name = strings.ReplaceAll(name, "\r", "")
	name = strings.ReplaceAll(name, "\n", "")
	return name
}

// Helper functions
func (h *DownloadHandler) getDirectoryContentsRecursive(path string) ([]models.FileInfo, error) {
	if h.catalogService == nil {
		return []models.FileInfo{}, nil
	}

	var result []models.FileInfo
	files, err := h.catalogService.ListDirectory(path)
	if err != nil {
		return nil, err
	}

	for _, f := range files {
		result = append(result, f)
		if f.IsDirectory {
			subFiles, err := h.getDirectoryContentsRecursive(f.Path)
			if err != nil {
				continue
			}
			result = append(result, subFiles...)
		}
	}

	return result, nil
}

func (h *DownloadHandler) getFilesByPath(path, smbRoot string) ([]models.FileInfo, error) {
	if h.catalogService == nil {
		return []models.FileInfo{}, nil
	}

	files, err := h.catalogService.ListDirectory(path)
	if err != nil {
		return nil, err
	}

	var result []models.FileInfo
	for _, f := range files {
		if !f.IsDirectory {
			result = append(result, f)
		}
	}
	return result, nil
}
