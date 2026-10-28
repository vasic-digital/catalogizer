package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"catalogizer/internal/services"
	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/smb"
	"catalogizer/utils"

	"github.com/gin-gonic/gin"
)

// CopyHandler handles file copy operations
type CopyHandler struct {
	fileRepo *repository.FileRepository
	smbPool  *smb.SmbConnectionPool
	tempDir  string
}

// NewCopyHandler creates a new copy handler
func NewCopyHandler(fileRepo *repository.FileRepository, tempDir string) *CopyHandler {
	return &CopyHandler{
		fileRepo: fileRepo,
		smbPool:  smb.NewSmbConnectionPool(10),
		tempDir:  tempDir,
	}
}

// CopyToSmb godoc
// @Summary Copy file/directory to SMB location
// @Description Copy a file or directory from one SMB location to another
// @Tags copy
// @Accept json
// @Produce json
// @Param body body SmbCopyRequest true "Copy request"
// @Success 200 {object} CopyResponse
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 404 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/copy/smb [post]
func (h *CopyHandler) CopyToSmb(c *gin.Context) {
	ctx := c.Request.Context()

	var req SmbCopyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate request
	if err := h.validateSmbCopyRequest(&req); err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid copy request", err)
		return
	}

	// Get source file information
	sourceFile, err := h.fileRepo.GetFileByID(ctx, req.SourceFileID)
	if err != nil {
		if err.Error() == "file not found" {
			utils.SendErrorResponse(c, http.StatusNotFound, "Source file not found", err)
			return
		}
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get source file info", err)
		return
	}

	// Get storage roots
	storageRoots, err := h.fileRepo.GetStorageRoots(ctx)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get storage roots", err)
		return
	}

	// Find source and destination storage roots
	var sourceStorageRoot, destStorageRoot *models.StorageRoot
	for _, root := range storageRoots {
		if root.ID == sourceFile.StorageRootID {
			sourceStorageRoot = &root
		}
		if root.Name == req.DestinationSmbRoot {
			destStorageRoot = &root
		}
	}

	if sourceStorageRoot == nil {
		utils.SendErrorResponse(c, http.StatusNotFound, "Source storage root not found", nil)
		return
	}
	if destStorageRoot == nil {
		utils.SendErrorResponse(c, http.StatusNotFound, "Destination storage root not found", nil)
		return
	}

	// Create SMB connections
	sourceSmbClient, err := h.createSmbClient(sourceStorageRoot)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to connect to source SMB", err)
		return
	}

	destSmbClient, err := h.createSmbClient(destStorageRoot)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to connect to destination SMB", err)
		return
	}

	// Perform copy operation
	startTime := time.Now()
	var result CopyResponse

	if sourceFile.IsDirectory {
		result, err = h.copyDirectoryToSmb(ctx, sourceSmbClient, destSmbClient, sourceFile.Path, req.DestinationPath, req.OverwriteExisting)
	} else {
		result, err = h.copyFileToSmb(ctx, sourceSmbClient, destSmbClient, sourceFile.Path, req.DestinationPath, req.OverwriteExisting)
	}

	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Copy operation failed", err)
		return
	}

	result.TimeTaken = time.Since(startTime)
	result.Success = true

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// CopyToLocal godoc
// @Summary Copy file/directory to local computer
// @Description Copy a file or directory from SMB to local computer
// @Tags copy
// @Accept json
// @Produce json
// @Param body body LocalCopyRequest true "Copy request"
// @Success 200 {object} CopyResponse
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 404 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/copy/local [post]
func (h *CopyHandler) CopyToLocal(c *gin.Context) {
	ctx := c.Request.Context()

	var req LocalCopyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	// Validate request
	if err := h.validateLocalCopyRequest(&req); err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid copy request", err)
		return
	}

	// Get source file information
	sourceFile, err := h.fileRepo.GetFileByID(ctx, req.SourceFileID)
	if err != nil {
		if err.Error() == "file not found" {
			utils.SendErrorResponse(c, http.StatusNotFound, "Source file not found", err)
			return
		}
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get source file info", err)
		return
	}

	// Get SMB roots
	smbRoots, err := h.fileRepo.GetStorageRoots(ctx)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get SMB roots", err)
		return
	}

	// Find source SMB root
	var sourceSmbRoot *models.StorageRoot
	for _, root := range smbRoots {
		if root.ID == sourceFile.StorageRootID {
			sourceSmbRoot = &root
			break
		}
	}

	if sourceSmbRoot == nil {
		utils.SendErrorResponse(c, http.StatusNotFound, "Source SMB root not found", nil)
		return
	}

	// Create SMB connection
	sourceSmbClient, err := h.createSmbClient(sourceSmbRoot)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to connect to source SMB", err)
		return
	}

	// Perform copy operation
	startTime := time.Now()
	var result CopyResponse

	if sourceFile.IsDirectory {
		result, err = h.copyDirectoryToLocal(ctx, sourceSmbClient, sourceFile.Path, req.DestinationPath, req.OverwriteExisting)
	} else {
		result, err = h.copyFileToLocal(ctx, sourceSmbClient, sourceFile.Path, req.DestinationPath, req.OverwriteExisting)
	}

	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Copy operation failed", err)
		return
	}

	result.TimeTaken = time.Since(startTime)
	result.Success = true

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// CopyFromLocal godoc
// @Summary Copy file/directory from local computer to SMB
// @Description Copy a file or directory from local computer to SMB location
// @Tags copy
// @Accept multipart/form-data
// @Produce json
// @Param destination_smb_root formData string true "Destination SMB root name"
// @Param destination_path formData string true "Destination path on SMB"
// @Param overwrite_existing formData bool false "Overwrite existing files" default(false)
// @Param file formData file true "File to upload"
// @Success 200 {object} CopyResponse
// @Failure 400 {object} utils.SendErrorResponse
// @Failure 500 {object} utils.SendErrorResponse
// @Router /api/copy/upload [post]
func (h *CopyHandler) CopyFromLocal(c *gin.Context) {
	ctx := c.Request.Context()

	// Parse form data
	destSmbRootName := c.PostForm("destination_smb_root")
	destPath := c.PostForm("destination_path")
	overwriteStr := c.DefaultPostForm("overwrite_existing", "false")
	overwrite := overwriteStr == "true"

	if destSmbRootName == "" || destPath == "" {
		utils.SendErrorResponse(c, http.StatusBadRequest, "destination_smb_root and destination_path are required", nil)
		return
	}

	if err := safeRemotePath(destPath); err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid destination_path", err)
		return
	}

	// Get uploaded file
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Failed to get uploaded file", err)
		return
	}
	defer file.Close()

	uploadName, err := safeChildName(header.Filename)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid file name", err)
		return
	}

	// Get SMB roots
	smbRoots, err := h.fileRepo.GetStorageRoots(ctx)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to get SMB roots", err)
		return
	}

	// Find destination SMB root
	var destSmbRoot *models.StorageRoot
	for _, root := range smbRoots {
		if root.Name == destSmbRootName {
			destSmbRoot = &root
			break
		}
	}

	if destSmbRoot == nil {
		utils.SendErrorResponse(c, http.StatusNotFound, "Destination SMB root not found", nil)
		return
	}

	// Create SMB connection
	destSmbClient, err := h.createSmbClient(destSmbRoot)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to connect to destination SMB", err)
		return
	}

	// Perform upload
	startTime := time.Now()
	fullDestPath := filepath.Join(destPath, uploadName)

	// Check if file exists and overwrite policy
	if exists, err := destSmbClient.FileExists(fullDestPath); err == nil && exists && !overwrite {
		utils.SendErrorResponse(c, http.StatusConflict, "File already exists and overwrite is disabled", nil)
		return
	}

	// Copy file
	err = destSmbClient.WriteFile(fullDestPath, file)
	if err != nil {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Failed to copy file to SMB", err)
		return
	}

	result := CopyResponse{
		Success:     true,
		BytesCopied: header.Size,
		FilesCount:  1,
		TimeTaken:   time.Since(startTime),
		SourcePath:  uploadName,
		DestPath:    fullDestPath,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// Helper methods

func (h *CopyHandler) createSmbClient(storageRoot *models.StorageRoot) (*smb.SmbClient, error) {
	host := ""
	if storageRoot.Host != nil {
		host = *storageRoot.Host
	}
	port := 445
	if storageRoot.Port != nil {
		port = *storageRoot.Port
	}
	share := ""
	if storageRoot.Path != nil {
		share = *storageRoot.Path
	}
		user, pass, dom := services.ResolveSMBIdentity(storageRoot)
	if storageRoot.Username != nil {
		user = *storageRoot.Username
	}
	if storageRoot.Domain != nil {
		dom = *storageRoot.Domain
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
	return h.smbPool.GetConnection(connectionKey, smbConfig)
}

func (h *CopyHandler) copyFileToSmb(ctx context.Context, sourceSmbClient, destSmbClient *smb.SmbClient, sourcePath, destPath string, overwrite bool) (CopyResponse, error) {
	// Check if destination exists
	if exists, err := destSmbClient.FileExists(destPath); err == nil && exists && !overwrite {
		return CopyResponse{}, fmt.Errorf("destination file exists and overwrite is disabled")
	}

	// Read source file
	sourceReader, err := sourceSmbClient.ReadFile(sourcePath)
	if err != nil {
		return CopyResponse{}, fmt.Errorf("failed to read source file: %w", err)
	}
	defer sourceReader.Close()

	// Write to destination
	err = destSmbClient.WriteFile(destPath, sourceReader)
	if err != nil {
		return CopyResponse{}, fmt.Errorf("failed to write destination file: %w", err)
	}

	// Get file info for size
	sourceInfo, err := sourceSmbClient.GetFileInfo(sourcePath)
	if err != nil {
		return CopyResponse{}, fmt.Errorf("failed to get source file info: %w", err)
	}

	return CopyResponse{
		BytesCopied: sourceInfo.Size,
		FilesCount:  1,
		SourcePath:  sourcePath,
		DestPath:    destPath,
	}, nil
}

func (h *CopyHandler) copyDirectoryToSmb(ctx context.Context, sourceSmbClient, destSmbClient *smb.SmbClient, sourcePath, destPath string, overwrite bool) (CopyResponse, error) {
	var totalBytes int64
	var filesCount int

	// Create destination directory
	err := destSmbClient.CreateDirectory(destPath)
	if err != nil {
		return CopyResponse{}, fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Recursively copy contents
	err = h.copyDirectoryContentsToSmb(ctx, sourceSmbClient, destSmbClient, sourcePath, destPath, overwrite, &totalBytes, &filesCount)
	if err != nil {
		return CopyResponse{}, err
	}

	return CopyResponse{
		BytesCopied: totalBytes,
		FilesCount:  filesCount,
		SourcePath:  sourcePath,
		DestPath:    destPath,
	}, nil
}

func (h *CopyHandler) copyDirectoryContentsToSmb(ctx context.Context, sourceSmbClient, destSmbClient *smb.SmbClient, sourcePath, destPath string, overwrite bool, totalBytes *int64, filesCount *int) error {
	files, err := sourceSmbClient.ListDirectory(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to list directory %s: %w", sourcePath, err)
	}

	for _, file := range files {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		childName, err := safeChildName(file.Name)
		if err != nil {
			return fmt.Errorf("refusing unsafe remote entry name: %w", err)
		}
		sourceFilePath := filepath.Join(sourcePath, childName)
		destFilePath := filepath.Join(destPath, childName)

		if file.IsDir {
			// Create subdirectory
			err := destSmbClient.CreateDirectory(destFilePath)
			if err != nil {
				return fmt.Errorf("failed to create directory %s: %w", destFilePath, err)
			}

			// Recursively copy subdirectory
			err = h.copyDirectoryContentsToSmb(ctx, sourceSmbClient, destSmbClient, sourceFilePath, destFilePath, overwrite, totalBytes, filesCount)
			if err != nil {
				return err
			}
		} else {
			// Copy file
			result, err := h.copyFileToSmb(ctx, sourceSmbClient, destSmbClient, sourceFilePath, destFilePath, overwrite)
			if err != nil {
				return err
			}
			*totalBytes += result.BytesCopied
			*filesCount += result.FilesCount
		}
	}

	return nil
}

func (h *CopyHandler) copyFileToLocal(ctx context.Context, sourceSmbClient *smb.SmbClient, sourcePath, destPath string, overwrite bool) (CopyResponse, error) {
	// Check if destination exists
	if _, err := os.Stat(destPath); err == nil && !overwrite {
		return CopyResponse{}, fmt.Errorf("destination file exists and overwrite is disabled")
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return CopyResponse{}, fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Read source file
	sourceReader, err := sourceSmbClient.ReadFile(sourcePath)
	if err != nil {
		return CopyResponse{}, fmt.Errorf("failed to read source file: %w", err)
	}
	defer sourceReader.Close()

	// Create destination file
	destFile, err := os.Create(destPath)
	if err != nil {
		return CopyResponse{}, fmt.Errorf("failed to create destination file: %w", err)
	}
	defer destFile.Close()

	// Copy data
	bytesCopied, err := io.Copy(destFile, sourceReader)
	if err != nil {
		return CopyResponse{}, fmt.Errorf("failed to copy file data: %w", err)
	}

	return CopyResponse{
		BytesCopied: bytesCopied,
		FilesCount:  1,
		SourcePath:  sourcePath,
		DestPath:    destPath,
	}, nil
}

func (h *CopyHandler) copyDirectoryToLocal(ctx context.Context, sourceSmbClient *smb.SmbClient, sourcePath, destPath string, overwrite bool) (CopyResponse, error) {
	var totalBytes int64
	var filesCount int

	// Create destination directory
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return CopyResponse{}, fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Recursively copy contents
	err := h.copyDirectoryContentsToLocal(ctx, sourceSmbClient, sourcePath, destPath, overwrite, &totalBytes, &filesCount)
	if err != nil {
		return CopyResponse{}, err
	}

	return CopyResponse{
		BytesCopied: totalBytes,
		FilesCount:  filesCount,
		SourcePath:  sourcePath,
		DestPath:    destPath,
	}, nil
}

func (h *CopyHandler) copyDirectoryContentsToLocal(ctx context.Context, sourceSmbClient *smb.SmbClient, sourcePath, destPath string, overwrite bool, totalBytes *int64, filesCount *int) error {
	files, err := sourceSmbClient.ListDirectory(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to list directory %s: %w", sourcePath, err)
	}

	for _, file := range files {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		childName, err := safeChildName(file.Name)
		if err != nil {
			return fmt.Errorf("refusing unsafe remote entry name: %w", err)
		}
		sourceFilePath := filepath.Join(sourcePath, childName)
		destFilePath := filepath.Join(destPath, childName)

		if file.IsDir {
			// Create subdirectory
			if err := os.MkdirAll(destFilePath, 0755); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", destFilePath, err)
			}

			// Recursively copy subdirectory
			err = h.copyDirectoryContentsToLocal(ctx, sourceSmbClient, sourceFilePath, destFilePath, overwrite, totalBytes, filesCount)
			if err != nil {
				return err
			}
		} else {
			// Copy file
			result, err := h.copyFileToLocal(ctx, sourceSmbClient, sourceFilePath, destFilePath, overwrite)
			if err != nil {
				return err
			}
			*totalBytes += result.BytesCopied
			*filesCount += result.FilesCount
		}
	}

	return nil
}

func (h *CopyHandler) validateSmbCopyRequest(req *SmbCopyRequest) error {
	if req.SourceFileID <= 0 {
		return fmt.Errorf("source_file_id is required")
	}
	if req.DestinationSmbRoot == "" {
		return fmt.Errorf("destination_smb_root is required")
	}
	if req.DestinationPath == "" {
		return fmt.Errorf("destination_path is required")
	}
	if err := safeRemotePath(req.DestinationPath); err != nil {
		return fmt.Errorf("invalid destination_path: %w", err)
	}
	return nil
}

func (h *CopyHandler) validateLocalCopyRequest(req *LocalCopyRequest) error {
	if req.SourceFileID <= 0 {
		return fmt.Errorf("source_file_id is required")
	}
	if req.DestinationPath == "" {
		return fmt.Errorf("destination_path is required")
	}
	// Local writes are confined to the configured temp/staging root; this also
	// rejects ../ traversal, absolute paths outside it and symlink escapes.
	resolved, err := containedJoin(h.tempDir, req.DestinationPath)
	if err != nil {
		return fmt.Errorf("invalid destination_path: %w", err)
	}
	req.DestinationPath = resolved
	return nil
}

// Request/Response types

type SmbCopyRequest struct {
	SourceFileID       int64  `json:"source_file_id" binding:"required"`
	DestinationSmbRoot string `json:"destination_smb_root" binding:"required"`
	DestinationPath    string `json:"destination_path" binding:"required"`
	OverwriteExisting  bool   `json:"overwrite_existing"`
}

type LocalCopyRequest struct {
	SourceFileID      int64  `json:"source_file_id" binding:"required"`
	DestinationPath   string `json:"destination_path" binding:"required"`
	OverwriteExisting bool   `json:"overwrite_existing"`
}

type CopyResponse struct {
	Success     bool          `json:"success"`
	BytesCopied int64         `json:"bytes_copied"`
	FilesCount  int           `json:"files_count"`
	TimeTaken   time.Duration `json:"time_taken"`
	SourcePath  string        `json:"source_path"`
	DestPath    string        `json:"dest_path"`
}

// Close closes the copy handler and its resources
func (h *CopyHandler) Close() {
	h.smbPool.CloseAll()
}
