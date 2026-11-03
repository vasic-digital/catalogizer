package handlers

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"time"

	"catalogizer/internal/services"
	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/smb"
	"catalogizer/utils"

	"github.com/gin-gonic/gin"
)

// DownloadHandler handles file download operations
type DownloadHandler struct {
	fileRepo       *repository.FileRepository
	smbPool        *smb.SmbConnectionPool
	tempDir        string
	maxArchiveSize int64
	chunkSize      int
}

// NewDownloadHandler creates a new download handler
func NewDownloadHandler(fileRepo *repository.FileRepository, tempDir string, maxArchiveSize int64, chunkSize int) *DownloadHandler {
	return &DownloadHandler{
		fileRepo:       fileRepo,
		smbPool:        smb.NewSmbConnectionPool(10), // Max 10 concurrent SMB connections
		tempDir:        tempDir,
		maxArchiveSize: maxArchiveSize,
		chunkSize:      chunkSize,
	}
}

// DownloadFile godoc
// @Summary Download a file
// @Description Download a specific file by ID with streaming support
// @Tags download
// @Produce application/octet-stream
// @Param id path int true "File ID"
// @Param inline query bool false "Display inline instead of download" default(false)
// @Success 200 {file} binary
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 404 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/download/file/{id} [get]
func (h *DownloadHandler) DownloadFile(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse file ID
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid file ID", err)
		return
	}

	// Get file information
	file, err := h.fileRepo.GetFileByID(ctx, id)
	if err != nil {
		if err.Error() == "file not found" {
			utils.SendErrorResponse(c, http.StatusNotFound, "File not found", err)
			return
		}
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get file info", err)
		return
	}

	// Check if it's a directory
	if file.IsDirectory {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Cannot download directory as file. Use directory download endpoint", nil)
		return
	}

	// Check if file is deleted
	if file.Deleted {
		utils.SendErrorResponse(c, http.StatusNotFound, "File has been deleted", nil)
		return
	}

	// Get SMB root information
	smbRoots, err := h.fileRepo.GetStorageRoots(ctx)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get SMB root info", err)
		return
	}

	var smbRoot *models.StorageRoot
	for _, root := range smbRoots {
		if root.ID == file.StorageRootID {
			smbRoot = &root
			break
		}
	}

	if smbRoot == nil {
		utils.SendErrorResponse(c, http.StatusNotFound, "SMB root not found", nil)
		return
	}

	// Create SMB connection
	host := ""
	if smbRoot.Host != nil {
		host = *smbRoot.Host
	}
	port := 445
	if smbRoot.Port != nil {
		port = *smbRoot.Port
	}
	share := ""
	if smbRoot.Path != nil {
		share = *smbRoot.Path
	}
		user, pass, dom := services.ResolveSMBIdentity(smbRoot)
	if smbRoot.Username != nil {
		user = *smbRoot.Username
	}
	if smbRoot.Domain != nil {
		dom = *smbRoot.Domain
	}

	smbConfig := &smb.SmbConfig{
		Host:     host,
		Port:     port,
		Share:    share,
		Username: user,
		Password: pass,
		Domain:   dom,
	}

	connectionKey := fmt.Sprintf("%s:%d:%s:%s", host, port, share, user)
	smbClient, err := h.smbPool.GetConnection(connectionKey, smbConfig)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to connect to SMB share", err)
		return
	}

	// Open file for reading
	reader, err := smbClient.ReadFile(file.Path)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to open file", err)
		return
	}
	defer reader.Close()

	// Set headers
	inline := c.Query("inline") == "true"
	disposition := "attachment"
	if inline {
		disposition = "inline"
	}

	c.Header("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disposition, file.Name))
	c.Header("Content-Length", strconv.FormatInt(file.Size, 10))

	// Set content type based on file extension
	if file.MimeType != nil && *file.MimeType != "" {
		c.Header("Content-Type", *file.MimeType)
	} else {
		c.Header("Content-Type", "application/octet-stream")
	}

	// Stream file content
	c.Stream(func(w io.Writer) bool {
		buffer := make([]byte, h.chunkSize)
		n, err := reader.Read(buffer)
		if err != nil {
			if err != io.EOF {
				c.Header("X-Download-Error", err.Error())
			}
			return false
		}
		_, writeErr := w.Write(buffer[:n])
		return writeErr == nil
	})
}

// DownloadDirectory godoc
// @Summary Download directory as ZIP
// @Description Download a directory and its contents as a ZIP archive
// @Tags download
// @Produce application/zip
// @Param smb_root path string true "SMB root name"
// @Param path query string true "Directory path"
// @Param recursive query bool false "Include subdirectories recursively" default(true)
// @Param max_depth query int false "Maximum recursion depth" default(-1)
// @Success 200 {file} binary
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 404 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/download/directory/{smb_root} [get]
func (h *DownloadHandler) DownloadDirectory(c *gin.Context) {
	ctx := c.Request.Context()

	smbRootName := c.Param("smb_root")
	dirPath := c.Query("path")
	recursive := c.DefaultQuery("recursive", "true") == "true"
	maxDepth, _ := strconv.Atoi(c.DefaultQuery("max_depth", "-1"))

	if dirPath == "" {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Directory path is required", nil)
		return
	}

	// Get SMB root information
	smbRoots, err := h.fileRepo.GetStorageRoots(ctx)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get SMB root info", err)
		return
	}

	var smbRoot *models.StorageRoot
	for _, root := range smbRoots {
		if root.Name == smbRootName {
			smbRoot = &root
			break
		}
	}

	if smbRoot == nil {
		utils.SendErrorResponse(c, http.StatusNotFound, "SMB root not found", nil)
		return
	}

	// Create SMB connection
	host := ""
	if smbRoot.Host != nil {
		host = *smbRoot.Host
	}
	port := 445
	if smbRoot.Port != nil {
		port = *smbRoot.Port
	}
	share := ""
	if smbRoot.Path != nil {
		share = *smbRoot.Path
	}
		user, pass, dom := services.ResolveSMBIdentity(smbRoot)
	if smbRoot.Username != nil {
		user = *smbRoot.Username
	}
	if smbRoot.Domain != nil {
		dom = *smbRoot.Domain
	}

	smbConfig := &smb.SmbConfig{
		Host:     host,
		Port:     port,
		Share:    share,
		Username: user,
		Password: pass,
		Domain:   dom,
	}

	connectionKey := fmt.Sprintf("%s:%d:%s:%s", host, port, share, user)
	smbClient, err := h.smbPool.GetConnection(connectionKey, smbConfig)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to connect to SMB share", err)
		return
	}

	// Check if directory exists
	fileInfo, err := smbClient.GetFileInfo(dirPath)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusNotFound, "Directory not found", err)
		return
	}

	if !fileInfo.IsDir {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Path is not a directory", nil)
		return
	}

	// Set headers for ZIP download
	dirName := filepath.Base(dirPath)
	if dirName == "." || dirName == "/" {
		dirName = "root"
	}
	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.zip"`, dirName))

	// Stream ZIP creation
	c.Stream(func(w io.Writer) bool {
		return h.createZipStream(ctx, w, smbClient, dirPath, recursive, maxDepth, 0)
	})
}

// createZipStream creates a ZIP archive and streams it to the writer
func (h *DownloadHandler) createZipStream(ctx context.Context, w io.Writer, smbClient *smb.SmbClient, basePath string, recursive bool, maxDepth, currentDepth int) bool {
	zipWriter := zip.NewWriter(w)
	defer zipWriter.Close()

	var totalSize int64
	err := h.addDirectoryToZip(ctx, zipWriter, smbClient, basePath, "", recursive, maxDepth, currentDepth, &totalSize)
	return err == nil
}

// addDirectoryToZip recursively adds directory contents to ZIP
func (h *DownloadHandler) addDirectoryToZip(ctx context.Context, zipWriter *zip.Writer, smbClient *smb.SmbClient, smbPath, zipPath string, recursive bool, maxDepth, currentDepth int, totalSize *int64) error {
	// Check context cancellation
	if ctx.Err() != nil {
		return ctx.Err()
	}

	// Check depth limit
	if maxDepth >= 0 && currentDepth >= maxDepth {
		return nil
	}

	// Check size limit
	if *totalSize > h.maxArchiveSize {
		return fmt.Errorf("archive size limit exceeded")
	}

	// List directory contents
	files, err := smbClient.ListDirectory(smbPath)
	if err != nil {
		return fmt.Errorf("failed to list directory %s: %w", smbPath, err)
	}

	for _, file := range files {
		fullSmbPath := filepath.Join(smbPath, file.Name)
		fullZipPath := filepath.Join(zipPath, file.Name)

		if file.IsDir {
			// Add directory entry
			dirHeader := &zip.FileHeader{
				Name:     fullZipPath + "/",
				Method:   zip.Store,
				Modified: file.ModTime,
			}
			_, err := zipWriter.CreateHeader(dirHeader)
			if err != nil {
				return fmt.Errorf("failed to create directory entry %s: %w", fullZipPath, err)
			}

			// Recursively add subdirectory if enabled
			if recursive {
				err = h.addDirectoryToZip(ctx, zipWriter, smbClient, fullSmbPath, fullZipPath, recursive, maxDepth, currentDepth+1, totalSize)
				if err != nil {
					return err
				}
			}
		} else {
			// Add file
			*totalSize += file.Size
			if *totalSize > h.maxArchiveSize {
				return fmt.Errorf("archive size limit exceeded")
			}

			fileHeader := &zip.FileHeader{
				Name:     fullZipPath,
				Method:   zip.Deflate,
				Modified: file.ModTime,
			}

			fileWriter, err := zipWriter.CreateHeader(fileHeader)
			if err != nil {
				return fmt.Errorf("failed to create file entry %s: %w", fullZipPath, err)
			}

			// Read and copy file content
			fileReader, err := smbClient.ReadFile(fullSmbPath)
			if err != nil {
				return fmt.Errorf("failed to read file %s: %w", fullSmbPath, err)
			}

			_, err = io.Copy(fileWriter, fileReader)
			fileReader.Close()
			if err != nil {
				return fmt.Errorf("failed to copy file %s: %w", fullSmbPath, err)
			}
		}
	}

	return nil
}

// GetDownloadInfo godoc
// @Summary Get download information
// @Description Get information about a file or directory before downloading
// @Tags download
// @Accept json
// @Produce json
// @Param id path int true "File ID"
// @Success 200 {object} DownloadInfo
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 404 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/download/info/{id} [get]
func (h *DownloadHandler) GetDownloadInfo(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse file ID
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid file ID", err)
		return
	}

	// Get file information
	file, err := h.fileRepo.GetFileByID(ctx, id)
	if err != nil {
		if err.Error() == "file not found" {
			utils.SendErrorResponse(c, http.StatusNotFound, "File not found", err)
			return
		}
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get file info", err)
		return
	}

	info := DownloadInfo{
		FileID:      file.ID,
		Name:        file.Name,
		Path:        file.Path,
		Size:        file.Size,
		IsDirectory: file.IsDirectory,
		MimeType:    file.MimeType,
		Extension:   file.Extension,
		ModifiedAt:  file.ModifiedAt,
		Deleted:     file.Deleted,
	}

	// CAT-001: no archive estimate is computed here, so none is reported (the field is omitted).

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    info,
	})
}

// DownloadInfo represents download information
type DownloadInfo struct {
	FileID               int64     `json:"file_id"`
	Name                 string    `json:"name"`
	Path                 string    `json:"path"`
	Size                 int64     `json:"size"`
	IsDirectory          bool      `json:"is_directory"`
	MimeType             *string   `json:"mime_type"`
	Extension            *string   `json:"extension"`
	ModifiedAt           time.Time `json:"modified_at"`
	Deleted              bool      `json:"deleted"`
	EstimatedArchiveSize int64     `json:"estimated_archive_size,omitempty"`
}

// Close closes the download handler and its resources
func (h *DownloadHandler) Close() {
	h.smbPool.CloseAll()
}
