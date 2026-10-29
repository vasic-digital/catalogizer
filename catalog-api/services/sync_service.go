package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
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
	sourceRoot, err := requireUploadSourceDir(endpoint.LocalPath)
	if err != nil {
		return err
	}
	localFiles, err := s.scanLocalFiles(sourceRoot)
	if err != nil {
		return fmt.Errorf("failed to scan local files: %w", err)
	}

	session.TotalFiles = len(localFiles)
	s.syncRepo.UpdateSession(session)

	for _, localFile := range localFiles {
		relativePath, err := filepath.Rel(sourceRoot, localFile)
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
		// a remote listing entry that climbs out of the sync root, lexically or through a
		// symlink under it, or that names a protected file (§11.4.115). Confine resolves like
		// the kernel; filepath.Join above already removed every ".." lexically, and that
		// cleaned path is the one written below, so both see the same path. It is a FILE target:
		// an entry equal to the remote root maps to the local root itself, whose Dir is the
		// root's parent (review round 5, RV5-B), so ConfineDataFile refuses it.
		if _, gerr := pathguard.ConfineDataFile(localPath, []string{endpoint.LocalPath}); gerr != nil {
			session.FailedFiles++
			s.logSyncError(session, fmt.Sprintf("Refusing remote path %q: escapes local root: %v", remoteFile.Path, gerr))
			continue
		}
		// endpoint.LocalPath was only checked when the endpoint was saved, and a swapped
		// ancestor moves it together with localPath; the configured roots do not move (review
		// round 9, F1).
		if _, gerr := pathguard.ConfineDataFile(localPath, pathguard.SyncWriteRoots()); gerr != nil {
			session.FailedFiles++
			s.logSyncError(session, fmt.Sprintf("Refusing local path %q: outside the sync roots: %v", localPath, gerr))
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

	// Resolve and check the upload source before any client is built: a single regular file
	// or a missing path is refused loudly instead of reporting a 0-file success (review round
	// 6, F3).
	sourceDir, err := uploadSourceDir(syncConfig, endpoint)
	if err != nil {
		return err
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

	// Walk through source directory and upload files
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}
		if !uploadableFile(path, info, sourceDir) {
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

	// Resolve and check the upload source before any client is built: a single regular file
	// or a missing path is refused loudly instead of reporting a 0-file success (review round
	// 6, F3).
	sourceDir, err := uploadSourceDir(syncConfig, endpoint)
	if err != nil {
		return err
	}

	// Create GCS client
	var client *storage.Client

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

	// Walk through source directory and upload files
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}
		if !uploadableFile(path, info, sourceDir) {
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

	// Resolve and confine both roots in ONE step and walk the path that step returns (review
	// round 8, item 1). confineSyncPaths ran earlier; resolving the roots again without a new
	// check followed a root swapped for a symlink in between: a swapped source copied outside
	// files in, a swapped destination let mirror cleanup delete outside files. The source is
	// read (written too in bidirectional mode); the destination is written and cleaned.
	switch syncMode {
	case "mirror", "incremental", "bidirectional":
	default:
		return fmt.Errorf("unsupported sync mode: %s", syncMode)
	}
	write := pathguard.SyncWriteRoots()
	srcRoots := pathguard.Readable(write)
	if syncMode == "bidirectional" {
		srcRoots = write
	}
	sourceDir, err := confinedWalkRoot(sourceDir, srcRoots)
	switch {
	case errors.Is(err, errNotADirectory):
		return fmt.Errorf("source path is not a directory: %w", err)
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("source directory does not exist: %w", err)
	case err != nil:
		return fmt.Errorf("source directory: %w", err)
	}
	dst, err := pathguard.ConfineData(destDir, write)
	if err != nil {
		return fmt.Errorf("destination directory: %w", err)
	}
	// Create destination directory if it doesn't exist, then check the result again: the
	// created (or existing) directory is what the walk writes into.
	if err := os.MkdirAll(dst, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	destDir, err = confinedWalkRoot(dst, write)
	if err != nil {
		return fmt.Errorf("destination directory: %w", err)
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
// sourceDir and destDir are the resolved, confined roots performLocalSync passes in; they are
// walked as given (a symlinked root is not resolved again here, review round 8).
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
			// The directory form: destDir itself (relPath ".") is valid. A directory symlink
			// planted in the destination would otherwise let MkdirAll create directories
			// outside the destination root (review round 6, F1).
			if _, err := pathguard.ConfineData(destPath, []string{destDir}); err != nil {
				return fmt.Errorf("destination directory %q refused: %w", destPath, err)
			}
			// destDir follows a swapped ancestor; the configured roots do not (round 9, F1).
			if _, err := pathguard.ConfineData(destPath, pathguard.SyncWriteRoots()); err != nil {
				return fmt.Errorf("destination directory %q refused: %w", destPath, err)
			}
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
		if !uploadableFile(sourcePath, info, sourceDir) {
			return nil
		}
		if err := s.copyFile(sourcePath, destPath, info.Mode(), sourceDir, destDir); err != nil {
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
// Like performMirrorSync it walks the roots as given.
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
		if !uploadableFile(sourcePath, info, sourceDir) {
			return nil
		}
		if err := s.copyFile(sourcePath, destPath, info.Mode(), sourceDir, destDir); err != nil {
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

// uploadableFile reports whether a file met while walking root may be read and sent: only a
// regular file (filepath.Walk reports a symlink as itself, and os.Open would follow it, so a
// link planted in the source to the database or to anything outside the root would be read),
// confined to root and not a protected application file or a hard link of one. Anything else is
// logged and skipped (review round 4). Every walker that reads local files to send them uses it:
// S3, GCS, local mirror and incremental, and the WebDAV upload scan (review round 5, RV5-A).
func uploadableFile(path string, info os.FileInfo, root string) bool {
	if !info.Mode().IsRegular() {
		logSkippedFile(path, "not a regular file (symlink, device, socket or pipe)")
		return false
	}
	if _, err := pathguard.ConfineDataFile(path, []string{root}); err != nil {
		logSkippedFile(path, err.Error())
		return false
	}
	// root is a resolved path string; ConfineDataFile resolves it again, so an ancestor of root
	// swapped for an outside link moves root and path together and the check above passes. The
	// configured roots do not move with the swap (review round 9, F1).
	if _, err := pathguard.ConfineDataFile(path, syncReadRoots()); err != nil {
		logSkippedFile(path, err.Error())
		return false
	}
	return true
}

// syncReadRoots are the configured roots a sync may read from (read roots plus sync write roots).
// The per-file guards check against these as well as against the walk root: a resolved walk root
// is re-resolved by every check, so on its own it follows a swapped ancestor (review round 9, F1).
func syncReadRoots() []string { return pathguard.Readable(pathguard.SyncWriteRoots()) }

// confineRemoval checks a path cleanupDestination is about to remove. os.Remove resolves every
// component but the last and removes the last without following it, so the PARENT is confined
// (directory form) to the configured sync write roots and to the walk root; the entry itself may
// be a link pointing anywhere (removing the link is harmless).
func confineRemoval(p, walkRoot string) error {
	parent := filepath.Dir(p)
	if _, err := pathguard.ConfineData(parent, []string{walkRoot}); err != nil {
		return fmt.Errorf("refusing to remove %q: %w", p, err)
	}
	if _, err := pathguard.ConfineData(parent, pathguard.SyncWriteRoots()); err != nil {
		return fmt.Errorf("refusing to remove %q: %w", p, err)
	}
	return nil
}

func logSkippedFile(path, why string) {
	if logging.Logger != nil {
		logging.With(logging.String("path", path), logging.String("reason", why)).Warn("Sync skipped a file")
	}
}

// copyFile copies a file with proper permissions. src must lie in srcRoot and dst in dstRoot
// after symlink resolution, and neither may be a protected application file: a symlink planted
// in the destination would otherwise redirect the write (review round 4). Both are FILE paths,
// strictly below their root (review round 5).
func (s *SyncService) copyFile(src, dst string, mode os.FileMode, srcRoot, dstRoot string) error {
	if _, err := pathguard.ConfineDataFile(src, []string{srcRoot}); err != nil {
		return fmt.Errorf("source %q: %w", src, err)
	}
	if _, err := pathguard.ConfineDataFile(dst, []string{dstRoot}); err != nil {
		return fmt.Errorf("destination %q: %w", dst, err)
	}
	// The roots above are resolved strings that follow a swapped ancestor; the configured roots
	// do not (review round 9, F1).
	write := pathguard.SyncWriteRoots()
	if _, err := pathguard.ConfineDataFile(src, syncReadRoots()); err != nil {
		return fmt.Errorf("source %q: %w", src, err)
	}
	if _, err := pathguard.ConfineDataFile(dst, write); err != nil {
		return fmt.Errorf("destination %q: %w", dst, err)
	}
	// Ensure destination directory exists. ConfineDataFile above already refuses a dst whose
	// prefixes leave dstRoot; the parent is checked in the directory form as well so the
	// MkdirAll call carries its own guard (review round 6, F1).
	if _, err := pathguard.ConfineData(filepath.Dir(dst), []string{dstRoot}); err != nil {
		return fmt.Errorf("destination directory %q: %w", filepath.Dir(dst), err)
	}
	if _, err := pathguard.ConfineData(filepath.Dir(dst), write); err != nil {
		return fmt.Errorf("destination directory %q: %w", filepath.Dir(dst), err)
	}
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
			// The destination root itself is never removed.
			if relPath == "." {
				return nil
			}
			// Each removal is confined first: destDir is a resolved string that follows a
			// swapped ancestor, so the parent is checked against the configured roots too
			// (review round 9, F1). A refusal fails the cleanup closed.
			if err := confineRemoval(destPath, destDir); err != nil {
				return err
			}
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

// confineSyncPaths verifies every local filesystem path the endpoint will read or write
// (§11.4.115). Paths a sync only READS (upload source, local-mirror source) may lie in the
// read roots (CATALOGIZER_READ_ROOTS, default MEDIA_ROOT_PATH + ./data) or the sync write
// roots; paths it WRITES (download target, destination_directory, either side of a
// bidirectional sync) must lie in the sync write roots (CATALOGIZER_SYNC_ROOTS, default
// ./data). sync_settings directories are checked for every endpoint type, because the
// S3/GCS/local drivers all honour source_directory.
func (s *SyncService) confineSyncPaths(endpoint *models.SyncEndpoint) error {
	write := pathguard.SyncWriteRoots()
	readable := pathguard.Readable(write)
	check := func(label, p string, roots []string) error {
		if _, err := pathguard.ConfineData(p, roots); err != nil {
			return fmt.Errorf("%s %q rejected: %w", label, p, err)
		}
		return nil
	}
	// local_path is written by download/bidirectional (and unknown directions), only read by upload
	localRoots := write
	if endpoint.SyncDirection == models.SyncDirectionUpload {
		localRoots = readable
	}

	var cfg map[string]interface{}
	if endpoint.SyncSettings != nil && *endpoint.SyncSettings != "" {
		if err := json.Unmarshal([]byte(*endpoint.SyncSettings), &cfg); err != nil {
			cfg = nil // unparsable settings are rejected by the driver itself
		}
	}
	mode, _ := cfg["sync_mode"].(string)
	srcRoots := readable
	if mode == "bidirectional" {
		srcRoots = write // bidirectional sync writes the source side too
	}
	dirSet := map[string]bool{}
	if v, ok := cfg["source_directory"].(string); ok && v != "" {
		dirSet["source_directory"] = true
		if err := check("source_directory", v, srcRoots); err != nil {
			return err
		}
	}
	if v, ok := cfg["destination_directory"].(string); ok && v != "" {
		if err := check("destination_directory", v, write); err != nil {
			return err
		}
	}

	if endpoint.LocalPath != "" {
		roots := localRoots
		// the local driver uses local_path as the source when source_directory is absent
		if endpoint.Type == models.SyncTypeLocal && !dirSet["source_directory"] && mode == "bidirectional" {
			roots = write
		}
		if err := check("local path", endpoint.LocalPath, roots); err != nil {
			return err
		}
	}
	// remote_path of a local-type endpoint is a local path (never used as a write target)
	if endpoint.Type == models.SyncTypeLocal && endpoint.RemotePath != "" {
		if err := check("remote path", endpoint.RemotePath, readable); err != nil {
			return err
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

// uploadSourceDir returns the directory an S3 or GCS upload walks: source_directory from the
// sync settings, else local_path. It must exist and be a directory (review round 6, F3); the
// returned path is the resolved directory, so a symlinked source is walked (review round 7, S1).
func uploadSourceDir(syncConfig map[string]interface{}, endpoint *models.SyncEndpoint) (string, error) {
	sourceDir, ok := syncConfig["source_directory"].(string)
	if !ok {
		sourceDir = endpoint.LocalPath
		if sourceDir == "" {
			return "", fmt.Errorf("source directory not specified")
		}
	}
	return requireUploadSourceDir(sourceDir)
}

// requireUploadSourceDir refuses an upload source that is missing, is not a directory, or does
// not resolve inside the roots an upload may read. The walkers below accept a regular-file root
// and then skip it, which reported a successful sync of zero files; the local driver already
// refused this case. It returns the resolved directory: filepath.Walk reports a symlinked root
// as the link itself and does not descend, so walking the unresolved path uploaded zero files
// and reported success (review round 7, S1).
func requireUploadSourceDir(dir string) (string, error) {
	resolved, err := confinedWalkRoot(dir, pathguard.Readable(pathguard.SyncWriteRoots()))
	if err != nil {
		return "", fmt.Errorf("upload source %q: %w", dir, err)
	}
	return resolved, nil
}

// confinedWalkRoot resolves dir and confines the result in one step (pathguard.ConfineData
// returns the resolved path it checked) and refuses anything that is not a directory. The
// caller walks the returned path, never resolves dir again: an earlier check of dir followed
// by a later unchecked resolution is exactly the window a root swapped for a symlink to an
// outside directory used (review round 8, item 1). A root swapped AFTER this call is not
// followed either: filepath.Walk does not descend into a symlinked root.
func confinedWalkRoot(dir string, roots []string) (string, error) {
	resolved, err := pathguard.ConfineData(dir, roots)
	if err != nil {
		return "", err
	}
	fi, err := os.Stat(resolved)
	if err != nil {
		return "", err
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("%q: %w", dir, errNotADirectory)
	}
	return resolved, nil
}

// errNotADirectory is returned by confinedWalkRoot for a root that resolves to a non-directory.
var errNotADirectory = errors.New("is not a directory")

// scanLocalFiles lists the files the WebDAV upload sends. Only uploadableFile entries are listed:
// the upload opens each path with os.Open, which follows a symlink planted in the source and
// reads a hard link of the database (review round 5, RV5-A; the other walkers got this in round 4).
func (s *SyncService) scanLocalFiles(path string) ([]string, error) {
	var files []string

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && uploadableFile(filePath, info, path) {
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
