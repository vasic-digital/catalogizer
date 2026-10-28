package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"catalogizer/internal/logging"
	"catalogizer/internal/pathguard"
	"catalogizer/models"
	"catalogizer/repository"

	"cloud.google.com/go/storage"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"google.golang.org/api/option"
)

type SyncService struct {
	syncRepo      *repository.SyncRepository
	userRepo      *repository.UserRepository
	authService   *AuthService
	webdavClients map[int]*WebDAVClient
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
}

func NewSyncService(syncRepo *repository.SyncRepository, userRepo *repository.UserRepository, authService *AuthService) *SyncService {
	ctx, cancel := context.WithCancel(context.Background())
	return &SyncService{
		syncRepo:      syncRepo,
		userRepo:      userRepo,
		authService:   authService,
		webdavClients: make(map[int]*WebDAVClient),
		ctx:           ctx,
		cancel:        cancel,
	}
}

// Close cancels any in-flight sync goroutines and waits for them to finish.
func (s *SyncService) Close() {
	s.cancel()
	s.wg.Wait()
}

func (s *SyncService) CreateSyncEndpoint(userID int, endpoint *models.SyncEndpoint) (*models.SyncEndpoint, error) {
	if s == nil || s.syncRepo == nil {
		return nil, fmt.Errorf("sync service not properly configured")
	}
	endpoint.UserID = userID
	endpoint.CreatedAt = time.Now()
	endpoint.UpdatedAt = time.Now()
	endpoint.Status = models.SyncStatusActive

	if err := s.validateSyncEndpoint(endpoint); err != nil {
		return nil, fmt.Errorf("invalid sync endpoint: %w", err)
	}

	if err := s.testConnection(endpoint); err != nil {
		return nil, fmt.Errorf("connection test failed: %w", err)
	}

	id, err := s.syncRepo.CreateEndpoint(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync endpoint: %w", err)
	}

	endpoint.ID = id
	return endpoint, nil
}

func (s *SyncService) GetUserEndpoints(userID int) ([]models.SyncEndpoint, error) {
	return s.syncRepo.GetUserEndpoints(userID)
}

func (s *SyncService) GetEndpoint(endpointID int, userID int) (*models.SyncEndpoint, error) {
	endpoint, err := s.syncRepo.GetEndpoint(endpointID)
	if err != nil {
		return nil, err
	}

	if endpoint.UserID != userID {
		hasPermission, err := s.authService.CheckPermission(userID, models.PermissionViewShares)
		if err != nil || !hasPermission {
			return nil, fmt.Errorf("unauthorized to view this endpoint")
		}
	}

	return endpoint, nil
}

func (s *SyncService) UpdateEndpoint(endpointID int, userID int, updates *models.UpdateSyncEndpointRequest) (*models.SyncEndpoint, error) {
	endpoint, err := s.syncRepo.GetEndpoint(endpointID)
	if err != nil {
		return nil, err
	}

	if endpoint.UserID != userID {
		hasPermission, err := s.authService.CheckPermission(userID, models.PermissionEditShares)
		if err != nil || !hasPermission {
			return nil, fmt.Errorf("unauthorized to update this endpoint")
		}
	}

	if updates.Name != "" {
		endpoint.Name = updates.Name
	}

	if updates.URL != "" {
		endpoint.URL = updates.URL
	}

	if updates.Username != "" {
		endpoint.Username = updates.Username
	}

	if updates.Password != "" {
		endpoint.Password = updates.Password
	}

	if updates.SyncDirection != "" {
		endpoint.SyncDirection = updates.SyncDirection
	}

	if updates.LocalPath != "" {
		endpoint.LocalPath = updates.LocalPath
	}

	if updates.RemotePath != "" {
		endpoint.RemotePath = updates.RemotePath
	}

	if updates.SyncSettings != nil {
		endpoint.SyncSettings = updates.SyncSettings
	}

	if updates.IsActive != nil {
		if *updates.IsActive {
			endpoint.Status = models.SyncStatusActive
		} else {
			endpoint.Status = models.SyncStatusInactive
		}
	}

	endpoint.UpdatedAt = time.Now()

	if err := s.validateSyncEndpoint(endpoint); err != nil {
		return nil, fmt.Errorf("invalid endpoint update: %w", err)
	}

	if err := s.testConnection(endpoint); err != nil {
		return nil, fmt.Errorf("connection test failed: %w", err)
	}

	err = s.syncRepo.UpdateEndpoint(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to update endpoint: %w", err)
	}

	return endpoint, nil
}

func (s *SyncService) DeleteEndpoint(endpointID int, userID int) error {
	endpoint, err := s.syncRepo.GetEndpoint(endpointID)
	if err != nil {
		return err
	}

	if endpoint.UserID != userID {
		hasPermission, err := s.authService.CheckPermission(userID, models.PermissionDeleteShares)
		if err != nil || !hasPermission {
			return fmt.Errorf("unauthorized to delete this endpoint")
		}
	}

	return s.syncRepo.DeleteEndpoint(endpointID)
}

func (s *SyncService) StartSync(endpointID int, userID int) (*models.SyncSession, error) {
	endpoint, err := s.syncRepo.GetEndpoint(endpointID)
	if err != nil {
		return nil, err
	}

	if endpoint.UserID != userID {
		hasPermission, err := s.authService.CheckPermission(userID, models.PermissionEditShares)
		if err != nil || !hasPermission {
			return nil, fmt.Errorf("unauthorized to sync this endpoint")
		}
	}

	if endpoint.Status != models.SyncStatusActive {
		return nil, fmt.Errorf("endpoint is not active")
	}

	session := &models.SyncSession{
		EndpointID: endpointID,
		UserID:     userID,
		Status:     models.SyncSessionStatusRunning,
		StartedAt:  time.Now(),
		SyncType:   models.SyncTypeManual,
	}

	sessionID, err := s.syncRepo.CreateSession(session)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync session: %w", err)
	}

	session.ID = sessionID

	// Return a snapshot copy to the caller so the goroutine can
	// safely mutate the original session without a data race.
	returnCopy := *session

	// performSync is naturally bounded: it dispatches to a protocol-specific
	// handler (WebDAV/Cloud/Local), performs finite file operations, updates
	// the session status in the DB, and returns.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.performSync(session, endpoint)
	}()

	return &returnCopy, nil
}

func (s *SyncService) performSync(session *models.SyncSession, endpoint *models.SyncEndpoint) {
	defer func() {
		if r := recover(); r != nil {
			s.handleSyncError(session, fmt.Errorf("sync panic: %v", r))
		}
	}()

	var err error

	// Endpoints stored before the path confinement existed (or edited in the DB) are
	// re-checked at execution time (§11.4.115).
	if cerr := s.confineSyncPaths(endpoint); cerr != nil {
		s.handleSyncError(session, fmt.Errorf("sync refused: %w", cerr))
		return
	}

	switch endpoint.Type {
	case models.SyncTypeWebDAV:
		err = s.performWebDAVSync(session, endpoint)
	case models.SyncTypeCloudStorage:
		err = s.performCloudSync(session, endpoint)
	case models.SyncTypeLocal:
		err = s.performLocalSync(session, endpoint)
	default:
		err = fmt.Errorf("unsupported sync type: %s", endpoint.Type)
	}

	if err != nil {
		s.handleSyncError(session, err)
		return
	}

	s.handleSyncSuccess(session)
}

func (s *SyncService) performWebDAVSync(session *models.SyncSession, endpoint *models.SyncEndpoint) error {
	client, err := s.getWebDAVClient(endpoint)
	if err != nil {
		return fmt.Errorf("failed to get WebDAV client: %w", err)
	}

	switch endpoint.SyncDirection {
	case models.SyncDirectionUpload:
		return s.uploadToWebDAV(session, endpoint, client)
	case models.SyncDirectionDownload:
		return s.downloadFromWebDAV(session, endpoint, client)
	case models.SyncDirectionBidirectional:
		if err := s.uploadToWebDAV(session, endpoint, client); err != nil {
			return err
		}
		return s.downloadFromWebDAV(session, endpoint, client)
	default:
		return fmt.Errorf("unsupported sync direction: %s", endpoint.SyncDirection)
	}
}

func (s *SyncService) uploadToWebDAV(session *models.SyncSession, endpoint *models.SyncEndpoint, client *WebDAVClient) error {
	localFiles, err := s.scanLocalFiles(endpoint.LocalPath)
	if err != nil {
		return fmt.Errorf("failed to scan local files: %w", err)
	}

	session.TotalFiles = len(localFiles)
	s.syncRepo.UpdateSession(session)

	for _, localFile := range localFiles {
		relativePath, err := filepath.Rel(endpoint.LocalPath, localFile)
		if err != nil {
			continue
		}

		remotePath := filepath.Join(endpoint.RemotePath, relativePath)
		remotePath = filepath.ToSlash(remotePath)

		if s.shouldSkipFile(localFile, endpoint) {
			continue
		}

		remoteModTime, err := client.GetModTime(remotePath)
		if err == nil {
			localInfo, err := os.Stat(localFile)
			if err != nil {
				continue
			}

			if !localInfo.ModTime().After(remoteModTime) {
				session.SkippedFiles++
				continue
			}
		}

		err = client.UploadFile(localFile, remotePath)
		if err != nil {
			session.FailedFiles++
			s.logSyncError(session, fmt.Sprintf("Failed to upload %s: %v", localFile, err))
		} else {
			session.SyncedFiles++
		}

		s.syncRepo.UpdateSession(session)
	}

	return nil
}

func (s *SyncService) downloadFromWebDAV(session *models.SyncSession, endpoint *models.SyncEndpoint, client *WebDAVClient) error {
	remoteFiles, err := client.ListFiles(endpoint.RemotePath)
	if err != nil {
		return fmt.Errorf("failed to list remote files: %w", err)
	}

	session.TotalFiles += len(remoteFiles)
	s.syncRepo.UpdateSession(session)

	for _, remoteFile := range remoteFiles {
		relativePath, err := filepath.Rel(endpoint.RemotePath, remoteFile.Path)
		if err != nil {
			continue
		}

		localPath := filepath.Join(endpoint.LocalPath, relativePath)
		if !pathguard.Within(endpoint.LocalPath, localPath) {
			// a remote listing entry that climbs out of the sync root (§11.4.115)
			session.FailedFiles++
			s.logSyncError(session, fmt.Sprintf("Refusing remote path %q: escapes local root", remoteFile.Path))
			continue
		}

		if s.shouldSkipRemoteFile(remoteFile, endpoint) {
			continue
		}

		localInfo, err := os.Stat(localPath)
		if err == nil {
			if !remoteFile.ModTime.After(localInfo.ModTime()) {
				session.SkippedFiles++
				continue
			}
		}

		err = os.MkdirAll(filepath.Dir(localPath), 0755)
		if err != nil {
			session.FailedFiles++
			continue
		}

		err = client.DownloadFile(remoteFile.Path, localPath)
		if err != nil {
			session.FailedFiles++
			s.logSyncError(session, fmt.Sprintf("Failed to download %s: %v", remoteFile.Path, err))
		} else {
			session.SyncedFiles++
		}

		s.syncRepo.UpdateSession(session)
	}

	return nil
}

func (s *SyncService) performCloudSync(session *models.SyncSession, endpoint *models.SyncEndpoint) error {
	ctx := context.Background()

	switch endpoint.Type {
	case "s3":
		return s.performS3Sync(ctx, session, endpoint)
	case "google_drive":
		return s.performGoogleCloudStorageSync(ctx, session, endpoint)
	default:
		return fmt.Errorf("unsupported cloud storage type: %s", endpoint.Type)
	}
}

// performS3Sync syncs files with Amazon S3
func (s *SyncService) performS3Sync(ctx context.Context, session *models.SyncSession, endpoint *models.SyncEndpoint) error {
	// Parse configuration
	syncConfig := make(map[string]interface{})
	if endpoint.SyncSettings != nil {
		if err := json.Unmarshal([]byte(*endpoint.SyncSettings), &syncConfig); err != nil {
			return fmt.Errorf("failed to parse S3 config: %w", err)
		}
	}

	// Extract S3 configuration
	bucket, ok := syncConfig["bucket"].(string)
	if !ok {
		return fmt.Errorf("S3 bucket not specified")
	}

	region, _ := syncConfig["region"].(string)
	if region == "" {
		region = "us-east-1"
	}

	accessKey, ok := syncConfig["access_key"].(string)
	if !ok {
		return fmt.Errorf("S3 access key not specified")
	}

	secretKey, ok := syncConfig["secret_key"].(string)
	if !ok {
		return fmt.Errorf("S3 secret key not specified")
	}

	// Create AWS configuration
	awsCfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(region),
		config.WithCredentialsProvider(aws.CredentialsProviderFunc(func(ctx context.Context) (aws.Credentials, error) {
			return aws.Credentials{
				AccessKeyID:     accessKey,
				SecretAccessKey: secretKey,
			}, nil
		})),
	)
	if err != nil {
		return fmt.Errorf("failed to create AWS config: %w", err)
	}

	// Create S3 client
	client := s3.NewFromConfig(awsCfg)

	// Get source directory from sync settings
	sourceDir, ok := syncConfig["source_directory"].(string)
	if !ok {
		// Fallback to local path
		sourceDir = endpoint.LocalPath
		if sourceDir == "" {
			return fmt.Errorf("source directory not specified")
		}
	}

	// Walk through source directory and upload files
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("failed to calculate relative path: %w", err)
		}

		// Convert to forward slashes for S3
		s3Key := filepath.ToSlash(relPath)

		// Open file
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", path, err)
		}
		defer file.Close()

		// Upload to S3
		_, err = client.PutObject(ctx, &s3.PutObjectInput{
			Bucket: aws.String(bucket),
			Key:    aws.String(s3Key),
			Body:   file,
		})
		if err != nil {
			return fmt.Errorf("failed to upload %s to S3: %w", s3Key, err)
		}

		// Update sync session progress
		s.updateSyncProgress(session, fmt.Sprintf("Uploaded: %s", s3Key))

		return nil
	})

	if err != nil {
		return fmt.Errorf("S3 sync failed: %w", err)
	}

	return nil
}

// performGoogleCloudStorageSync syncs files with Google Cloud Storage
func (s *SyncService) performGoogleCloudStorageSync(ctx context.Context, session *models.SyncSession, endpoint *models.SyncEndpoint) error {
	// Parse configuration
	syncConfig := make(map[string]interface{})
	if endpoint.SyncSettings != nil {
		if err := json.Unmarshal([]byte(*endpoint.SyncSettings), &syncConfig); err != nil {
			return fmt.Errorf("failed to parse Google Cloud Storage config: %w", err)
		}
	}

	// Extract GCS configuration
	bucket, ok := syncConfig["bucket"].(string)
	if !ok {
		return fmt.Errorf("GCS bucket not specified")
	}

	credentialsFile, _ := syncConfig["credentials_file"].(string)

	// Create GCS client
	var client *storage.Client
	var err error

	if credentialsFile != "" {
		// Use credentials file for GCS authentication
		// Note: option.WithCredentialsFile is deprecated; migrate to cloud.google.com/go/auth
		// when upgrading Google Cloud dependencies
		client, err = storage.NewClient(ctx, option.WithCredentialsFile(credentialsFile))
	} else {
		// Use default credentials
		client, err = storage.NewClient(ctx)
	}

	if err != nil {
		return fmt.Errorf("failed to create GCS client: %w", err)
	}
	defer client.Close()

	// Get source directory from sync settings
	sourceDir, ok := syncConfig["source_directory"].(string)
	if !ok {
		// Fallback to local path
		sourceDir = endpoint.LocalPath
		if sourceDir == "" {
			return fmt.Errorf("source directory not specified")
		}
	}

	// Walk through source directory and upload files
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return fmt.Errorf("failed to calculate relative path: %w", err)
		}

		// Convert to forward slashes for GCS
		gcsObject := filepath.ToSlash(relPath)

		// Open file
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", path, err)
		}
		defer file.Close()

		// Create GCS object writer
		wc := client.Bucket(bucket).Object(gcsObject).NewWriter(ctx)
		defer wc.Close()

		// Copy file to GCS
		_, err = io.Copy(wc, file)
		if err != nil {
			return fmt.Errorf("failed to upload %s to GCS: %w", gcsObject, err)
		}

		// Close writer to complete upload
		if err := wc.Close(); err != nil {
			return fmt.Errorf("failed to complete upload of %s to GCS: %w", gcsObject, err)
		}

		// Update sync session progress
		s.updateSyncProgress(session, fmt.Sprintf("Uploaded: %s", gcsObject))

		return nil
	})

	if err != nil {
		return fmt.Errorf("google cloud storage sync failed: %w", err)
	}

	return nil
}

func (s *SyncService) performLocalSync(session *models.SyncSession, endpoint *models.SyncEndpoint) error {
	// Parse configuration
	syncConfig := make(map[string]interface{})
	if endpoint.SyncSettings != nil {
		if err := json.Unmarshal([]byte(*endpoint.SyncSettings), &syncConfig); err != nil {
			return fmt.Errorf("failed to parse local sync config: %w", err)
		}
	}

	// Get source directory from sync settings or endpoint local path
	sourceDir, ok := syncConfig["source_directory"].(string)
	if !ok {
		sourceDir = endpoint.LocalPath
		if sourceDir == "" {
			return fmt.Errorf("source directory not specified")
		}
	}

	// Get destination directory
	destDir, ok := syncConfig["destination_directory"].(string)
	if !ok {
		return fmt.Errorf("destination directory not specified")
	}

	// Get sync mode (optional)
	syncMode := "mirror" // default
	if mode, ok := syncConfig["sync_mode"].(string); ok {
		syncMode = mode
	}

	// Verify source directory exists
	sourceInfo, err := os.Stat(sourceDir)
	if err != nil {
		return fmt.Errorf("source directory does not exist: %w", err)
	}
	if !sourceInfo.IsDir() {
		return fmt.Errorf("source path is not a directory")
	}

	// Create destination directory if it doesn't exist
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Perform sync based on mode
	switch syncMode {
	case "mirror":
		err = s.performMirrorSync(sourceDir, destDir, session)
	case "incremental":
		err = s.performIncrementalSync(sourceDir, destDir, session)
	case "bidirectional":
		err = s.performBidirectionalSync(sourceDir, destDir, session)
	default:
		return fmt.Errorf("unsupported sync mode: %s", syncMode)
	}

	if err != nil {
		return fmt.Errorf("local sync failed: %w", err)
	}

	return nil
}

// performMirrorSync creates exact copy of source in destination
func (s *SyncService) performMirrorSync(sourceDir, destDir string, session *models.SyncSession) error {
	// Walk through source directory
	err := filepath.Walk(sourceDir, func(sourcePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(sourceDir, sourcePath)
		if err != nil {
			return fmt.Errorf("failed to calculate relative path: %w", err)
		}

		// Build destination path
		destPath := filepath.Join(destDir, relPath)

		// Handle directories
		if info.IsDir() {
			// Create destination directory
			if err := os.MkdirAll(destPath, info.Mode()); err != nil {
				return fmt.Errorf("failed to create directory %s: %w", destPath, err)
			}
			return nil
		}

		// Handle files
		// Check if file needs to be copied
		destInfo, err := os.Stat(destPath)
		if err == nil {
			// Compare modification times
			if !info.ModTime().After(destInfo.ModTime()) {
				// Source is not newer, skip
				return nil
			}

			// Compare file sizes
			if info.Size() == destInfo.Size() && info.ModTime().Equal(destInfo.ModTime()) {
				// Files are identical, skip
				return nil
			}
		}

		// Copy file
		if err := s.copyFile(sourcePath, destPath, info.Mode()); err != nil {
			return fmt.Errorf("failed to copy file %s to %s: %w", sourcePath, destPath, err)
		}

		// Update sync session progress
		s.updateSyncProgress(session, fmt.Sprintf("Synced: %s", relPath))

		return nil
	})

	if err != nil {
		return fmt.Errorf("mirror sync failed: %w", err)
	}

	// Remove files in destination that don't exist in source
	err = s.cleanupDestination(sourceDir, destDir, session)
	if err != nil {
		return fmt.Errorf("failed to cleanup destination: %w", err)
	}

	return nil
}

// performIncrementalSync only copies newer or missing files
func (s *SyncService) performIncrementalSync(sourceDir, destDir string, session *models.SyncSession) error {
	// Walk through source directory
	err := filepath.Walk(sourceDir, func(sourcePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories for incremental sync
		if info.IsDir() {
			return nil
		}

		// Calculate relative path
		relPath, err := filepath.Rel(sourceDir, sourcePath)
		if err != nil {
			return fmt.Errorf("failed to calculate relative path: %w", err)
		}

		// Build destination path
		destPath := filepath.Join(destDir, relPath)

		// Check if destination exists
		destInfo, err := os.Stat(destPath)
		if err == nil {
			// File exists, check if source is newer
			if !info.ModTime().After(destInfo.ModTime()) {
				// Source is not newer, skip
				return nil
			}
		}

		// Copy file
		if err := s.copyFile(sourcePath, destPath, info.Mode()); err != nil {
			return fmt.Errorf("failed to copy file %s to %s: %w", sourcePath, destPath, err)
		}

		// Update sync session progress
		s.updateSyncProgress(session, fmt.Sprintf("Incrementally synced: %s", relPath))

		return nil
	})

	return err
}

// performBidirectionalSync syncs in both directions
func (s *SyncService) performBidirectionalSync(sourceDir, destDir string, session *models.SyncSession) error {
	// For bidirectional sync, we perform incremental sync in both directions
	// First sync source to destination
	if err := s.performIncrementalSync(sourceDir, destDir, session); err != nil {
		return fmt.Errorf("source to destination sync failed: %w", err)
	}

	// Then sync destination to source
	if err := s.performIncrementalSync(destDir, sourceDir, session); err != nil {
		return fmt.Errorf("destination to source sync failed: %w", err)
	}

	return nil
}

// copyFile copies a file with proper permissions
func (s *SyncService) copyFile(src, dst string, mode os.FileMode) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	// Open source file
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// Create destination file
	destFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer destFile.Close()

	// Copy file content
	_, err = io.Copy(destFile, sourceFile)
	return err
}

// cleanupDestination removes files in destination that don't exist in source
func (s *SyncService) cleanupDestination(sourceDir, destDir string, session *models.SyncSession) error {
	// Walk through destination directory
	return filepath.Walk(destDir, func(destPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path
		relPath, err := filepath.Rel(destDir, destPath)
		if err != nil {
			return fmt.Errorf("failed to calculate relative path: %w", err)
		}

		// Build corresponding source path
		sourcePath := filepath.Join(sourceDir, relPath)

		// Check if corresponding source file exists
		_, err = os.Stat(sourcePath)
		if err != nil {
			// Source file doesn't exist, remove destination file
			if info.IsDir() {
				// Remove directory if it's empty
				if err := os.Remove(destPath); err != nil {
					// Directory not empty, skip
					return nil
				}
			} else {
				// Remove file
				if err := os.Remove(destPath); err != nil {
					return fmt.Errorf("failed to remove %s: %w", destPath, err)
				}
			}

			// Update sync session progress
			s.updateSyncProgress(session, fmt.Sprintf("Removed: %s", relPath))
		}

		return nil
	})
}

func (s *SyncService) handleSyncSuccess(session *models.SyncSession) {
	session.Status = models.SyncSessionStatusCompleted
	session.CompletedAt = &time.Time{}
	*session.CompletedAt = time.Now()

	if session.StartedAt != (time.Time{}) {
		duration := session.CompletedAt.Sub(session.StartedAt)
		session.Duration = &duration
	}

	s.syncRepo.UpdateSession(session)
	s.notifyUser(session, "Sync completed successfully")
}

func (s *SyncService) handleSyncError(session *models.SyncSession, syncError error) {
	session.Status = models.SyncSessionStatusFailed
	session.CompletedAt = &time.Time{}
	*session.CompletedAt = time.Now()
	errorMsg := syncError.Error()
	session.ErrorMessage = &errorMsg

	if session.StartedAt != (time.Time{}) {
		duration := session.CompletedAt.Sub(session.StartedAt)
		session.Duration = &duration
	}

	s.syncRepo.UpdateSession(session)
	s.notifyUser(session, fmt.Sprintf("Sync failed: %s", syncError.Error()))
}

func (s *SyncService) updateSyncProgress(session *models.SyncSession, message string) {
	// In a full implementation, this would update the session with current progress
	// For now, we just log the progress update
	if logging.Logger != nil {
		logging.With(logging.Int("session_id", session.ID), logging.String("message", message)).Debug("Sync progress")
	}
}

func (s *SyncService) logSyncError(session *models.SyncSession, message string) {
	// In a full implementation, this would log to a sync error log
	if logging.Logger != nil {
		logging.With(logging.Int("session_id", session.ID), logging.String("message", message)).Warn("Sync error")
	}
}

func (s *SyncService) notifyUser(session *models.SyncSession, message string) {
	// In a full implementation, this would send notifications
	if logging.Logger != nil {
		logging.With(
			logging.Int("user_id", session.UserID),
			logging.Int("session_id", session.ID),
			logging.String("message", message),
		).Debug("Sync notification")
	}
}

func (s *SyncService) GetUserSessions(userID int, limit, offset int) ([]models.SyncSession, error) {
	return s.syncRepo.GetUserSessions(userID, limit, offset)
}

func (s *SyncService) GetSession(sessionID int, userID int) (*models.SyncSession, error) {
	session, err := s.syncRepo.GetSession(sessionID)
	if err != nil {
		return nil, err
	}

	if session.UserID != userID {
		hasPermission, err := s.authService.CheckPermission(userID, models.PermissionViewShares)
		if err != nil || !hasPermission {
			return nil, fmt.Errorf("unauthorized to view this session")
		}
	}

	return session, nil
}

func (s *SyncService) ScheduleSync(endpointID int, userID int, schedule *models.SyncSchedule) (*models.SyncSchedule, error) {
	endpoint, err := s.syncRepo.GetEndpoint(endpointID)
	if err != nil {
		return nil, err
	}

	if endpoint.UserID != userID {
		hasPermission, err := s.authService.CheckPermission(userID, models.PermissionEditShares)
		if err != nil || !hasPermission {
			return nil, fmt.Errorf("unauthorized to schedule sync for this endpoint")
		}
	}

	schedule.EndpointID = endpointID
	schedule.UserID = userID
	schedule.CreatedAt = time.Now()
	schedule.IsActive = true

	id, err := s.syncRepo.CreateSchedule(schedule)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync schedule: %w", err)
	}

	schedule.ID = id
	return schedule, nil
}

func (s *SyncService) GetSyncStatistics(userID *int, startDate, endDate time.Time) (*models.SyncStatistics, error) {
	return s.syncRepo.GetStatistics(userID, startDate, endDate)
}

func (s *SyncService) ProcessScheduledSyncs() error {
	schedules, err := s.syncRepo.GetActiveSchedules()
	if err != nil {
		return err
	}

	for _, schedule := range schedules {
		if s.shouldRunSchedule(&schedule) {
			_, err := s.StartSync(schedule.EndpointID, schedule.UserID)
			if err != nil {
				if logging.Logger != nil {
					logging.With(
						logging.Int("endpoint_id", schedule.EndpointID),
						logging.ErrorField(err),
					).Error("Failed to start scheduled sync")
				}
			}
		}
	}

	return nil
}

func (s *SyncService) shouldRunSchedule(schedule *models.SyncSchedule) bool {
	now := time.Now()

	switch schedule.Frequency {
	case models.SyncFrequencyHourly:
		return schedule.LastRun == nil || schedule.LastRun.Add(time.Hour).Before(now)
	case models.SyncFrequencyDaily:
		return schedule.LastRun == nil || schedule.LastRun.Add(24*time.Hour).Before(now)
	case models.SyncFrequencyWeekly:
		return schedule.LastRun == nil || schedule.LastRun.Add(7*24*time.Hour).Before(now)
	case models.SyncFrequencyMonthly:
		return schedule.LastRun == nil || schedule.LastRun.AddDate(0, 1, 0).Before(now)
	default:
		return false
	}
}

// syncAllowedRoots are the directories local paths of a sync endpoint must stay inside
// (§11.4.115). Configure with CATALOGIZER_SYNC_ROOTS (colon-separated absolute paths);
// the default is ./data/sync. Read on every call so a config change needs no restart.
func syncAllowedRoots() []string {
	return pathguard.RootsFromEnv("CATALOGIZER_SYNC_ROOTS", filepath.Join(".", "data", "sync"))
}

// confineSyncPaths verifies every local filesystem path the endpoint will read or write:
// LocalPath always; RemotePath only for local-type endpoints (it is a local path there);
// and the source/destination directories carried in sync_settings.
func (s *SyncService) confineSyncPaths(endpoint *models.SyncEndpoint) error {
	roots := syncAllowedRoots()
	check := func(label, p string) error {
		if _, err := pathguard.Confine(p, roots); err != nil {
			return fmt.Errorf("%s %q rejected: %w", label, p, err)
		}
		return nil
	}
	if endpoint.LocalPath != "" {
		if err := check("local path", endpoint.LocalPath); err != nil {
			return err
		}
	}
	if endpoint.Type == models.SyncTypeLocal {
		if endpoint.RemotePath != "" {
			if err := check("remote path", endpoint.RemotePath); err != nil {
				return err
			}
		}
		if endpoint.SyncSettings != nil {
			cfg := map[string]interface{}{}
			if err := json.Unmarshal([]byte(*endpoint.SyncSettings), &cfg); err == nil {
				for _, k := range []string{"source_directory", "destination_directory"} {
					if v, ok := cfg[k].(string); ok && v != "" {
						if err := check(k, v); err != nil {
							return err
						}
					}
				}
			}
		}
	}
	return nil
}

func (s *SyncService) validateSyncEndpoint(endpoint *models.SyncEndpoint) error {
	if endpoint.Name == "" {
		return fmt.Errorf("name is required")
	}

	if endpoint.URL == "" {
		return fmt.Errorf("URL is required")
	}

	if endpoint.Type == "" {
		return fmt.Errorf("type is required")
	}

	if endpoint.SyncDirection == "" {
		return fmt.Errorf("sync direction is required")
	}

	if endpoint.LocalPath == "" {
		return fmt.Errorf("local path is required")
	}

	if err := s.confineSyncPaths(endpoint); err != nil {
		return fmt.Errorf("invalid local path: %w", err)
	}

	validTypes := []string{models.SyncTypeWebDAV, models.SyncTypeCloudStorage, models.SyncTypeLocal}
	if !s.isValidType(endpoint.Type, validTypes) {
		return fmt.Errorf("invalid sync type: %s", endpoint.Type)
	}

	validDirections := []string{models.SyncDirectionUpload, models.SyncDirectionDownload, models.SyncDirectionBidirectional}
	if !s.isValidType(endpoint.SyncDirection, validDirections) {
		return fmt.Errorf("invalid sync direction: %s", endpoint.SyncDirection)
	}

	return nil
}

func (s *SyncService) isValidType(value string, validValues []string) bool {
	for _, valid := range validValues {
		if value == valid {
			return true
		}
	}
	return false
}

func (s *SyncService) testConnection(endpoint *models.SyncEndpoint) error {
	switch endpoint.Type {
	case models.SyncTypeWebDAV:
		client, err := s.getWebDAVClient(endpoint)
		if err != nil {
			return err
		}
		return client.TestConnection()
	default:
		return nil // Skip test for other types for now
	}
}

func (s *SyncService) getWebDAVClient(endpoint *models.SyncEndpoint) (*WebDAVClient, error) {
	if client, exists := s.webdavClients[endpoint.ID]; exists {
		return client, nil
	}

	client := NewWebDAVClient(endpoint.URL, endpoint.Username, endpoint.Password)
	s.webdavClients[endpoint.ID] = client

	return client, nil
}

func (s *SyncService) scanLocalFiles(path string) ([]string, error) {
	var files []string

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			files = append(files, filePath)
		}

		return nil
	})

	return files, err
}

func (s *SyncService) shouldSkipFile(filePath string, endpoint *models.SyncEndpoint) bool {
	fileName := filepath.Base(filePath)

	// Skip hidden files
	if strings.HasPrefix(fileName, ".") {
		return true
	}

	// Skip temporary files
	if strings.HasSuffix(fileName, ".tmp") || strings.HasSuffix(fileName, ".temp") {
		return true
	}

	// Check file size limits if configured
	if endpoint.SyncSettings != nil {
		// This would parse JSON settings and check file size limits
	}

	return false
}

func (s *SyncService) shouldSkipRemoteFile(file *WebDAVFile, endpoint *models.SyncEndpoint) bool {
	fileName := filepath.Base(file.Path)

	// Skip hidden files
	if strings.HasPrefix(fileName, ".") {
		return true
	}

	// Skip temporary files
	if strings.HasSuffix(fileName, ".tmp") || strings.HasSuffix(fileName, ".temp") {
		return true
	}

	return false
}

func (s *SyncService) calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (s *SyncService) CleanupOldSessions(olderThan time.Time) error {
	return s.syncRepo.CleanupSessions(olderThan)
}
