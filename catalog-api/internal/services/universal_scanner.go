package services

import (
	"catalogizer/database"
	"catalogizer/filesystem"
	scancontrol "catalogizer/internal/concurrency"
	"catalogizer/internal/eventbus"
	"catalogizer/models"
	"context"
	"errors"
	"fmt"
	"mime"
	pathpkg "path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/semaphore"
)

// UniversalScanner handles file system scanning across all supported protocols
type UniversalScanner struct {
	db                 *database.DB
	logger             *zap.Logger
	renameTracker      *UniversalRenameTracker
	clientFactory      filesystem.ClientFactory
	aggregationService *AggregationService
	eventBus           *eventbus.EventBus
	scanQueue          chan ScanJob
	workers            int
	maxConcurrentScans int
	scanSem            *scancontrol.Semaphore
	stopCh             chan struct{}
	wg                 sync.WaitGroup
	protocolScannersMu sync.RWMutex
	protocolScanners   map[string]ProtocolScanner
	activeScansMu      sync.RWMutex
	activeScans        map[string]*ScanStatus
}

// ScanJob represents a scan operation for any protocol
type ScanJob struct {
	ID              string
	StorageRoot     *models.StorageRoot
	Path            string
	Priority        int
	ScanType        string // full, incremental, verify
	MaxDepth        int
	IncludePatterns []string
	ExcludePatterns []string
	Context         context.Context
}

// ScanStatus tracks the status of an active scan
type ScanStatus struct {
	JobID           string
	StorageRootName string
	Protocol        string
	StartTime       time.Time
	CurrentPath     string
	FilesProcessed  int64
	FilesFound      int64
	FilesUpdated    int64
	FilesDeleted    int64
	ErrorCount      int64
	Status          string // running, completed, failed, cancelled
	// Reason is the machine-readable reason of a failed or cancelled scan (empty otherwise). PA-03: a scan that did not complete says why.
	Reason  string
	secrets []string // literal secrets of the storage root, scrubbed from Reason (never exported, never copied into a snapshot)
	mu      sync.RWMutex
}

// ProtocolScanner defines protocol-specific scanning behavior
type ProtocolScanner interface {
	// ScanPath performs a scan of the specified path
	ScanPath(ctx context.Context, client filesystem.FileSystemClient, job ScanJob, status *ScanStatus) error

	// GetScanStrategy returns the optimal scanning strategy for this protocol
	GetScanStrategy() ScanStrategy

	// SupportsIncrementalScan indicates if the protocol supports incremental scanning
	SupportsIncrementalScan() bool

	// GetOptimalBatchSize returns the optimal batch size for database operations
	GetOptimalBatchSize() int
}

// ScanStrategy defines how scanning should be performed
type ScanStrategy struct {
	UseRecursiveListing     bool
	BatchSize               int
	ParallelDirectories     bool
	ChecksumCalculation     bool
	MetadataExtraction      bool
	RealTimeChangeDetection bool
}

// NewUniversalScanner creates a new universal file system scanner.
// The scannerConcurrency parameter controls the semaphore weight for
// concurrent scan operations. If <= 0, it defaults to 4.
func NewUniversalScanner(db *database.DB, logger *zap.Logger, renameTracker *UniversalRenameTracker, clientFactory filesystem.ClientFactory, scannerConcurrency ...int) *UniversalScanner {
	concurrency := 4
	if len(scannerConcurrency) > 0 && scannerConcurrency[0] > 0 {
		concurrency = scannerConcurrency[0]
	}

	scanner := &UniversalScanner{
		db:                 db,
		logger:             logger,
		renameTracker:      renameTracker,
		clientFactory:      clientFactory,
		scanQueue:          make(chan ScanJob, 100),
		workers:            concurrency,
		maxConcurrentScans: concurrency,
		scanSem:            scancontrol.NewSemaphore(concurrency),
		stopCh:             make(chan struct{}),
		protocolScanners:   make(map[string]ProtocolScanner),
		activeScans:        make(map[string]*ScanStatus),
	}

	// Register protocol scanners
	scanner.RegisterProtocolScanner("local", NewLocalScanner(db, logger))
	scanner.RegisterProtocolScanner("smb", NewSMBScanner(db, logger))
	scanner.RegisterProtocolScanner("ftp", NewFTPScanner(logger).WithDB(db))
	scanner.RegisterProtocolScanner("nfs", NewNFSScanner(logger).WithDB(db))
	scanner.RegisterProtocolScanner("webdav", NewWebDAVScanner(logger).WithDB(db))
	// Pluggable protocols (sftp, ftps, nfs3, ...) registered before the scanner exists get the generic scanner too; later ones are
	// resolved on demand in processScanJob.
	for _, proto := range filesystem.RegisteredProtocols() {
		scanner.RegisterProtocolScanner(proto, NewGenericProtocolScanner(proto, db, logger))
	}

	return scanner
}

// SetAggregationService sets the aggregation service for post-scan entity creation.
func (s *UniversalScanner) SetAggregationService(svc *AggregationService) {
	s.aggregationService = svc
}

// SetEventBus sets the system event bus for publishing scan lifecycle events.
func (s *UniversalScanner) SetEventBus(bus *eventbus.EventBus) {
	s.eventBus = bus
}

// publishScanEvent publishes a scan lifecycle event to the system event bus.
func (s *UniversalScanner) publishScanEvent(job ScanJob, status *ScanStatus, scanErr error) {
	if s.eventBus == nil {
		return
	}

	eventType := eventbus.EventScanCompleted
	snapshot := status.GetSnapshot()
	if scanErr != nil {
		eventType = eventbus.EventScanFailed
		if snapshot.Status == "cancelled" {
			eventType = eventbus.EventScanCancelled
		}
	}

	payload := map[string]interface{}{
		"job_id":          job.ID,
		"storage_root":    job.StorageRoot.Name,
		"protocol":        job.StorageRoot.Protocol,
		"status":          snapshot.Status,
		"start_time":      snapshot.StartTime,
		"files_processed": snapshot.FilesProcessed,
		"files_found":     snapshot.FilesFound,
		"files_updated":   snapshot.FilesUpdated,
		"files_deleted":   snapshot.FilesDeleted,
		"error_count":     snapshot.ErrorCount,
	}
	if scanErr != nil {
		// The scrubbed reason, never scanErr.Error(): a client error can embed a URL with credentials (WF22 R9).
		payload["error"] = snapshot.Reason
		payload["reason"] = snapshot.Reason
	}

	evt := eventbus.NewEvent(eventType, "universal-scanner", payload)
	s.eventBus.Publish(evt)
}

// RegisterProtocolScanner registers a protocol-specific scanner
func (s *UniversalScanner) RegisterProtocolScanner(protocol string, scanner ProtocolScanner) {
	s.protocolScannersMu.Lock()
	defer s.protocolScannersMu.Unlock()
	s.protocolScanners[protocol] = scanner
}

// Start begins the universal scanning service
func (s *UniversalScanner) Start() error {
	s.logger.Info("Starting universal scanner service", zap.Int("workers", s.workers))

	// Start worker goroutines
	for i := 0; i < s.workers; i++ {
		s.wg.Add(1)
		go s.scanWorker(i)
	}

	return nil
}

// Stop stops the universal scanning service
func (s *UniversalScanner) Stop() {
	s.logger.Info("Stopping universal scanner service")
	close(s.stopCh)
	s.wg.Wait()
	s.logger.Info("Universal scanner service stopped")
}

// QueueScan adds a scan job to the queue
func (s *UniversalScanner) QueueScan(job ScanJob) error {
	select {
	case s.scanQueue <- job:
		s.logger.Debug("Queued scan job",
			zap.String("job_id", job.ID),
			zap.String("storage_root", job.StorageRoot.Name),
			zap.String("protocol", job.StorageRoot.Protocol),
			zap.String("path", job.Path))
		return nil
	default:
		return fmt.Errorf("scan queue is full")
	}
}

// scanWorker processes scan jobs
func (s *UniversalScanner) scanWorker(workerID int) {
	defer s.wg.Done()

	s.logger.Info("Universal scan worker started", zap.Int("worker_id", workerID))

	for {
		select {
		case <-s.stopCh:
			return
		case job := <-s.scanQueue:
			s.processScanJob(job, workerID)
		}
	}
}

// processScanJob processes a single scan job
func (s *UniversalScanner) processScanJob(job ScanJob, workerID int) {
	// Panic recovery: log the panic, move the VISIBLE status to failed (with a reason), increment error_count, and publish the event. The
	// active status entry is the one clients poll; leaving it "running" for the 60 s retention window would hide the failure (WF22 P0).
	defer func() {
		if r := recover(); r != nil {
			s.logger.Error("Panic recovered in processScanJob",
				zap.String("job_id", job.ID),
				zap.String("storage_root", job.StorageRoot.Name),
				zap.Any("panic", r))
			panicErr := fmt.Errorf("panic: %v", r)
			s.activeScansMu.RLock()
			status := s.activeScans[job.ID]
			s.activeScansMu.RUnlock()
			if status == nil {
				status = &ScanStatus{
					JobID:           job.ID,
					StorageRootName: job.StorageRoot.Name,
					Protocol:        job.StorageRoot.Protocol,
					StartTime:       time.Now(),
					secrets:         secretsOf(job.StorageRoot),
				}
			}
			status.incrementCounters(0, 0, 0, 0, 1)
			status.fail(panicErr)
			s.publishScanEvent(job, status, panicErr)
		}
	}()

	// Acquire semaphore to limit concurrent scans
	if err := s.scanSem.Acquire(job.Context); err != nil {
		s.logger.Debug("Scan job cancelled before acquiring semaphore",
			zap.String("job_id", job.ID),
			zap.Error(err))
		return
	}
	defer s.scanSem.Release()

	s.logger.Debug("Processing scan job",
		zap.Int("worker_id", workerID),
		zap.String("job_id", job.ID),
		zap.String("storage_root", job.StorageRoot.Name),
		zap.String("protocol", job.StorageRoot.Protocol))

	// Create scan status
	status := &ScanStatus{
		JobID:           job.ID,
		StorageRootName: job.StorageRoot.Name,
		Protocol:        job.StorageRoot.Protocol,
		StartTime:       time.Now(),
		Status:          "running",
		secrets:         secretsOf(job.StorageRoot),
	}

	// Track active scan
	s.activeScansMu.Lock()
	s.activeScans[job.ID] = status
	s.activeScansMu.Unlock()

	// Retain completed scan status for 60 seconds so polling clients
	// can read the final result before it is garbage-collected.
	defer func() {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			select {
			case <-time.After(60 * time.Second):
			case <-s.stopCh:
			}
			s.activeScansMu.Lock()
			delete(s.activeScans, job.ID)
			s.activeScansMu.Unlock()
		}()
	}()

	// Get protocol scanner
	s.protocolScannersMu.RLock()
	protocolScanner, exists := s.protocolScanners[job.StorageRoot.Protocol]
	s.protocolScannersMu.RUnlock()
	if !exists && filesystem.IsRegisteredProtocol(job.StorageRoot.Protocol) {
		// A protocol registered after this scanner was created: the generic scanner handles it.
		protocolScanner = NewGenericProtocolScanner(job.StorageRoot.Protocol, s.db, s.logger)
		s.RegisterProtocolScanner(job.StorageRoot.Protocol, protocolScanner)
		exists = true
	}
	if !exists {
		s.logger.Error("No scanner for protocol",
			zap.String("protocol", job.StorageRoot.Protocol),
			zap.String("job_id", job.ID))
		scanErr := fmt.Errorf("no scanner for protocol %s", job.StorageRoot.Protocol)
		status.fail(scanErr)
		s.publishScanEvent(job, status, scanErr)
		return
	}

	// Create filesystem client
	client, err := s.clientFactory.CreateClient(&filesystem.StorageConfig{
		ID:       job.StorageRoot.Name,
		Name:     job.StorageRoot.Name,
		Protocol: job.StorageRoot.Protocol,
		Settings: s.storageRootToSettings(job.StorageRoot),
	})
	if err != nil {
		s.logger.Error("Failed to create filesystem client",
			zap.String("protocol", job.StorageRoot.Protocol),
			zap.String("job_id", job.ID),
			zap.Error(err))
		status.fail(err)
		s.publishScanEvent(job, status, err)
		return
	}

	// Connect to filesystem
	if err := client.Connect(job.Context); err != nil {
		s.logger.Error("Failed to connect to filesystem",
			zap.String("protocol", job.StorageRoot.Protocol),
			zap.String("job_id", job.ID),
			zap.String("storage_root", job.StorageRoot.Name),
			zap.Error(err))
		status.incrementCounters(0, 0, 0, 0, 1)
		status.fail(err)
		s.publishScanEvent(job, status, err)
		return
	}
	defer client.Disconnect(job.Context)

	// Perform the scan
	if err := protocolScanner.ScanPath(job.Context, client, job, status); err != nil {
		s.logger.Error("Scan failed",
			zap.String("job_id", job.ID),
			zap.Error(err))
		// Only an operator CANCELLATION is "cancelled"; a deadline is a failure (the scan did not finish), reported with kind "timeout" (WF22 R7).
		if job.Context != nil && job.Context.Err() != nil && errors.Is(err, job.Context.Err()) && !errors.Is(job.Context.Err(), context.DeadlineExceeded) {
			status.cancel(err)
		} else {
			status.fail(err)
		}
		s.publishScanEvent(job, status, err)
		return
	}

	status.updateStatus("completed")
	s.publishScanEvent(job, status, nil)
	snapshot := status.GetSnapshot()
	s.logger.Info("Scan completed successfully",
		zap.String("job_id", job.ID),
		zap.String("storage_root", job.StorageRoot.Name),
		zap.Int64("files_processed", snapshot.FilesProcessed),
		zap.Duration("duration", time.Since(snapshot.StartTime)))

	// Run post-scan aggregation to create media entities from scanned files
	if s.aggregationService != nil {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			if err := s.aggregationService.AggregateAfterScan(job.Context, int64(job.StorageRoot.ID)); err != nil {
				s.logger.Error("Post-scan aggregation failed",
					zap.String("job_id", job.ID),
					zap.Error(err))
			}
		}()
	}
}

// GetActiveScanStatus returns the status of an active scan
func (s *UniversalScanner) GetActiveScanStatus(jobID string) (*ScanStatus, bool) {
	s.activeScansMu.RLock()
	defer s.activeScansMu.RUnlock()
	status, exists := s.activeScans[jobID]
	return status, exists
}

// GetAllActiveScanStatuses returns all active scan statuses
func (s *UniversalScanner) GetAllActiveScanStatuses() map[string]*ScanStatus {
	s.activeScansMu.RLock()
	defer s.activeScansMu.RUnlock()

	statuses := make(map[string]*ScanStatus)
	for id, status := range s.activeScans {
		// Use GetSnapshot for thread-safe copy of status fields
		snapshot := status.GetSnapshot()
		statuses[id] = &snapshot
	}
	return statuses
}

// storageRootToSettings converts StorageRoot to filesystem settings. PA-01: it delegates to filesystem.SettingsFromRoot, the single
// settings-key vocabulary shared with the client factory, the stream handler and the comic-pages handler.
func (s *UniversalScanner) storageRootToSettings(root *models.StorageRoot) map[string]interface{} {
	return filesystem.SettingsFromRoot(root, ResolveSMBIdentity)
}

// updateStatus safely updates the scan status.
// Progress updates use mutex-protected field writes rather than channel sends,
// so they are inherently non-blocking and cannot stall the scanner.
func (s *ScanStatus) updateStatus(newStatus string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = newStatus
}

// fail marks the scan failed and records why. The reason is scrubbed of credentials (scrubSecrets) before it is stored.
func (s *ScanStatus) fail(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = "failed"
	if err != nil {
		s.Reason = scrubSecrets(err.Error(), s.secrets)
	}
}

// cancel marks the scan cancelled and records why.
func (s *ScanStatus) cancel(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Status = "cancelled"
	if err != nil {
		s.Reason = scrubSecrets(err.Error(), s.secrets)
	}
}

// setNote records a remark on a scan that did not fail (for example the sub-directories it had to skip). It never overwrites a failure reason.
func (s *ScanStatus) setNote(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Reason == "" {
		s.Reason = scrubSecrets(msg, s.secrets)
	}
}

// updateCurrentPath safely updates the current path being scanned
func (s *ScanStatus) updateCurrentPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CurrentPath = path
}

// incrementCounters safely increments the various counters
func (s *ScanStatus) incrementCounters(processed, found, updated, deleted, errors int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.FilesProcessed += processed
	s.FilesFound += found
	s.FilesUpdated += updated
	s.FilesDeleted += deleted
	s.ErrorCount += errors
}

// GetSnapshot returns a thread-safe snapshot of the scan status
func (s *ScanStatus) GetSnapshot() ScanStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return ScanStatus{
		JobID:           s.JobID,
		StorageRootName: s.StorageRootName,
		Protocol:        s.Protocol,
		StartTime:       s.StartTime,
		CurrentPath:     s.CurrentPath,
		FilesProcessed:  s.FilesProcessed,
		FilesFound:      s.FilesFound,
		FilesUpdated:    s.FilesUpdated,
		FilesDeleted:    s.FilesDeleted,
		ErrorCount:      s.ErrorCount,
		Status:          s.Status,
		Reason:          s.Reason,
	}
}

// LocalScanner implements protocol-specific scanning for local filesystem
type LocalScanner struct {
	db             *database.DB
	logger         *zap.Logger
	sem            *semaphore.Weighted
	maxConcurrency int
}

func NewLocalScanner(db *database.DB, logger *zap.Logger) *LocalScanner {
	maxConcurrency := 10 // optimal for local filesystem
	return &LocalScanner{
		db:             db,
		logger:         logger,
		sem:            semaphore.NewWeighted(int64(maxConcurrency)),
		maxConcurrency: maxConcurrency,
	}
}

func (s *LocalScanner) ScanPath(ctx context.Context, client filesystem.FileSystemClient, job ScanJob, status *ScanStatus) error {
	return s.scanDirectory(ctx, client, job.Path, job, status, 0)
}

func (s *LocalScanner) scanDirectory(ctx context.Context, client filesystem.FileSystemClient, path string, job ScanJob, status *ScanStatus, depth int) error {
	if depth > job.MaxDepth {
		return nil
	}

	status.updateCurrentPath(path)

	files, err := client.ListDirectory(ctx, path)
	if err != nil {
		status.incrementCounters(0, 0, 0, 0, 1)
		return fmt.Errorf("failed to list directory %s: %w", path, err)
	}

	for _, file := range files {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fullPath := pathpkg.Join(path, file.Name) // a catalog path is a slash path whatever the host OS (WF22 X1)

		// Process file/directory
		if err := s.processFileInfo(ctx, client, fullPath, file, job, status); err != nil {
			s.logger.Error("Failed to process file",
				zap.String("path", fullPath),
				zap.Error(err))
			status.incrementCounters(0, 0, 0, 0, 1)
		}

		// Recurse into subdirectories
		if file.IsDir {
			if err := s.scanDirectory(ctx, client, fullPath, job, status, depth+1); err != nil {
				s.logger.Error("Failed to scan subdirectory",
					zap.String("path", fullPath),
					zap.Error(err))
			}
		}
	}

	return nil
}

func (s *LocalScanner) processFileInfo(ctx context.Context, client filesystem.FileSystemClient, path string, file *filesystem.FileInfo, job ScanJob, status *ScanStatus) error {
	return insertFileRecord(ctx, s.db, path, file, job, status, s.logger)
}

func (s *LocalScanner) GetScanStrategy() ScanStrategy {
	return ScanStrategy{
		UseRecursiveListing:     true,
		BatchSize:               1000,
		ParallelDirectories:     true,
		ChecksumCalculation:     true,
		MetadataExtraction:      true,
		RealTimeChangeDetection: true,
	}
}

func (s *LocalScanner) SupportsIncrementalScan() bool {
	return true
}

func (s *LocalScanner) GetOptimalBatchSize() int {
	return 1000
}

// SMBScanner implements protocol-specific scanning for SMB
type SMBScanner struct {
	db     *database.DB
	logger *zap.Logger
}

func NewSMBScanner(db *database.DB, logger *zap.Logger) *SMBScanner {
	return &SMBScanner{db: db, logger: logger}
}

func (s *SMBScanner) ScanPath(ctx context.Context, client filesystem.FileSystemClient, job ScanJob, status *ScanStatus) error {
	// SMB-specific scanning logic
	return s.scanDirectory(ctx, client, job.Path, job, status, 0)
}

func (s *SMBScanner) scanDirectory(ctx context.Context, client filesystem.FileSystemClient, path string, job ScanJob, status *ScanStatus, depth int) error {
	// Similar to LocalScanner but with SMB-specific optimizations
	if depth > job.MaxDepth {
		return nil
	}

	status.updateCurrentPath(path)

	files, err := client.ListDirectory(ctx, path)
	if err != nil {
		status.incrementCounters(0, 0, 0, 0, 1)
		return fmt.Errorf("failed to list SMB directory %s: %w", path, err)
	}

	// Process files in batches for better SMB performance
	batchSize := s.GetOptimalBatchSize()
	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}

		batch := files[i:end]
		for _, file := range batch {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			fullPath := pathpkg.Join(path, file.Name) // a remote path is a slash path whatever the host OS (PA-01 filepath->path)

			if err := insertFileRecord(ctx, s.db, fullPath, file, job, status, s.logger); err != nil {
				s.logger.Error("Failed to insert file record",
					zap.String("path", fullPath),
					zap.Error(err))
				status.incrementCounters(0, 0, 0, 0, 1)
			}

			if file.IsDir {
				s.logger.Warn("Scanning directory recursively", zap.String("path", fullPath))
				if err := s.scanDirectory(ctx, client, fullPath, job, status, depth+1); err != nil {
					s.logger.Error("Failed to scan SMB subdirectory",
						zap.String("path", fullPath),
						zap.Error(err))
				}
			}
		}
	}

	return nil
}

func (s *SMBScanner) GetScanStrategy() ScanStrategy {
	return ScanStrategy{
		UseRecursiveListing:     false, // SMB benefits from controlled recursion
		BatchSize:               500,   // Smaller batches for network efficiency
		ParallelDirectories:     false, // Avoid overwhelming SMB server
		ChecksumCalculation:     false, // Expensive over network
		MetadataExtraction:      true,
		RealTimeChangeDetection: false,
	}
}

func (s *SMBScanner) SupportsIncrementalScan() bool {
	return true
}

func (s *SMBScanner) GetOptimalBatchSize() int {
	return 500
}

// ensureDirectoryPathExists ensures all directory components in a path exist in the database,
// creating them if necessary, and returns the parent directory ID for the given path.
// Returns nil if the path has no parent (root directory).
func ensureDirectoryPathExists(ctx context.Context, db *database.DB, storageRootID int64, fullPath string, logger *zap.Logger) (*int64, error) {
	logger.Info("ensureDirectoryPathExists called", zap.String("fullPath", fullPath), zap.Int64("storageRootID", storageRootID))
	if fullPath == "" || fullPath == "." || fullPath == "/" {
		return nil, nil
	}

	// NOTE: A path with no "/" separator is a single top-level directory
	// (e.g. "The Matrix (1999)" when scanning the share root). It MUST still
	// be resolved — a file living directly inside it needs parent_id set to
	// that directory's id, otherwise aggregation's child-file lookup
	// (parent_id = dirID) finds nothing and the directory (and all its media)
	// is dropped from browse-entity creation (entities_created = 0). The loop
	// below already handles the single-component case correctly, so we do NOT
	// early-return here.

	// Split fullPath into components and create each directory in the chain.
	// fullPath is the parent directory path of the file being inserted (e.g. "media/movies"),
	// so we iterate over ALL its components to ensure each directory exists.
	components := strings.Split(fullPath, "/")
	var currentPath string
	var parentID *int64 = nil

	for _, comp := range components {
		if comp == "" {
			continue
		}
		if currentPath == "" {
			currentPath = comp
		} else {
			currentPath = currentPath + "/" + comp
		}

		// Check if directory exists
		logger.Warn("Checking directory existence", zap.String("currentPath", currentPath), zap.Int64("storageRootID", storageRootID), zap.Any("parentID", parentID))
		var dirID int64
		err := db.QueryRowContext(ctx,
			"SELECT id FROM files WHERE path = ? AND storage_root_id = ? AND is_directory = 1 LIMIT 1",
			currentPath, storageRootID,
		).Scan(&dirID)
		if err == nil {
			// Directory exists. BACKFILL its parent_id when we now know the
			// parent and the row was previously stamped NULL (DEFECT-E #2): the
			// row may have been created out of order (INSERT OR IGNORE) before
			// its parent chain was resolved, leaving an inner directory wrongly
			// at the top level (parent_id IS NULL). Only fill a NULL — never
			// overwrite an existing parent.
			logger.Warn("Directory already exists", zap.String("path", currentPath), zap.Int64("id", dirID))
			if parentID != nil {
				if _, uerr := db.ExecContext(ctx,
					`UPDATE files SET parent_id = ? WHERE id = ? AND parent_id IS NULL`,
					*parentID, dirID,
				); uerr != nil {
					logger.Warn("Failed to backfill directory parent_id",
						zap.String("path", currentPath), zap.Int64("id", dirID), zap.Error(uerr))
				}
			}
			parentID = &dirID
			continue
		}
		logger.Warn("Directory does not exist, will create", zap.String("path", currentPath), zap.Error(err))

		// Directory doesn't exist, create it
		logger.Warn("Creating directory", zap.String("path", currentPath), zap.Any("parentID", parentID))
		var insertedID int64
		if db.Dialect().IsSQLite() {
			// Use INSERT OR IGNORE with transaction
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				return nil, fmt.Errorf("begin transaction for directory %s: %w", currentPath, err)
			}
			defer tx.Rollback()

			// Temporarily disable foreign key checks for SQLite
			_, _ = tx.ExecContext(ctx, "PRAGMA foreign_keys = OFF")

			_, err = tx.ExecContext(ctx,
				`INSERT OR IGNORE INTO files (storage_root_id, path, name, extension, mime_type, file_type, size, is_directory, modified_at, last_scan_at, parent_id)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?)`,
				storageRootID, currentPath, comp, "", "", "other", 0, true, time.Now(), parentID,
			)
			if err != nil {
				return nil, fmt.Errorf("insert directory %s: %w", currentPath, err)
			}

			// Get the inserted ID
			err = tx.QueryRowContext(ctx,
				"SELECT id FROM files WHERE path = ? AND storage_root_id = ?",
				currentPath, storageRootID,
			).Scan(&insertedID)
			if err != nil {
				return nil, fmt.Errorf("get directory ID for %s: %w", currentPath, err)
			}

			_, _ = tx.ExecContext(ctx, "PRAGMA foreign_keys = ON")
			if err := tx.Commit(); err != nil {
				return nil, fmt.Errorf("commit directory creation transaction: %w", err)
			}
		} else {
			// PostgreSQL path
			err = db.QueryRowContext(ctx,
				`INSERT INTO files (storage_root_id, path, name, extension, mime_type, file_type, size, is_directory, modified_at, last_scan_at, parent_id)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
				 ON CONFLICT(storage_root_id, path) DO UPDATE SET
				   last_scan_at = CURRENT_TIMESTAMP,
				   deleted = false
				 RETURNING id`,
				storageRootID, currentPath, comp, "", "", "other", 0, true, time.Now(), parentID,
			).Scan(&insertedID)
			if err != nil {
				return nil, fmt.Errorf("insert directory %s: %w", currentPath, err)
			}
		}

		logger.Info("Created directory record",
			zap.String("path", currentPath),
			zap.Int64("id", insertedID),
			zap.Any("parent_id", parentID))
		parentID = &insertedID
	}

	logger.Warn("ensureDirectoryPathExists returning", zap.String("fullPath", fullPath), zap.Any("parentID", parentID))
	return parentID, nil
}

// insertFileRecord inserts or updates a file record in the database.
// This is the core function that makes the scanner actually populate
// the catalog, shared by all protocol scanners.
//
// PERFORMANCE OPTIMIZATION OPPORTUNITY: Batch Inserts
// Currently each file is inserted individually, requiring one round-trip
// per file (or two for SQLite due to the transaction + PRAGMA pattern).
// For large scans (85K+ files on NAS), this is the primary bottleneck.
//
// Recommended approach:
//  1. Accumulate files in a slice up to GetOptimalBatchSize() (e.g., 500 for SMB, 1000 for local).
//  2. Build a single multi-row INSERT ... VALUES (...), (...), ... statement.
//  3. For PostgreSQL, use ON CONFLICT(storage_root_id, path) DO UPDATE SET ... on the batch.
//  4. For SQLite, use a single transaction wrapping all INSERTs in the batch.
//  5. Requires pre-resolving storageRootID and parentIDs for the batch,
//     which can be done via a single directory-existence query per batch.
//
// Estimated improvement: 5-10x throughput for network protocols (SMB, FTP, WebDAV)
// where per-statement overhead dominates, and 2-3x for local filesystem.
//
// PERFORMANCE OPTIMIZATION OPPORTUNITY: Incremental Scanning
// Currently every scan re-processes all files. Incremental scanning would:
//  1. Query files.last_scan_at for the storage root to get the previous scan timestamp.
//  2. Compare file modified_at against the previous scan timestamp.
//  3. Only INSERT/UPDATE files that are new or modified since the last scan.
//  4. Mark files not seen in the current scan as deleted (soft delete).
//  5. Skip unchanged directories entirely when their modified_at is older than last_scan_at.
//
// The ScanJob.ScanType field already supports "incremental" but the logic
// is not yet implemented in the protocol scanners.
func insertFileRecord(ctx context.Context, db *database.DB, path string, file *filesystem.FileInfo, job ScanJob, status *ScanStatus, logger *zap.Logger) error {
	if db == nil {
		status.incrementCounters(1, 1, 0, 0, 0)
		return nil
	}
	logger.Warn("insertFileRecord dialect check",
		zap.Bool("is_sqlite", db.Dialect().IsSQLite()),
		zap.Bool("is_postgres", db.Dialect().IsPostgres()),
		zap.String("db_type", db.DatabaseType()))

	// Resolve storage root ID
	var storageRootID int64
	err := db.QueryRowContext(ctx,
		"SELECT id FROM storage_roots WHERE name = ? LIMIT 1",
		job.StorageRoot.Name,
	).Scan(&storageRootID)
	if err != nil {
		// Storage root not in DB yet — insert it
		logger.Warn("Inserting storage root", zap.String("name", job.StorageRoot.Name), zap.Bool("is_postgres", db.Dialect().IsPostgres()), zap.Bool("is_sqlite", db.Dialect().IsSQLite()))
		if db.Dialect().IsPostgres() {
			// PostgreSQL: use INSERT ... ON CONFLICT DO NOTHING + RETURNING
			err2 := db.QueryRowContext(ctx,
				`INSERT INTO storage_roots (name, protocol, host, port, path, username, password, domain, enabled, max_depth)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, true, ?)
				 ON CONFLICT (name) DO NOTHING
				 RETURNING id`,
				job.StorageRoot.Name, job.StorageRoot.Protocol,
				job.StorageRoot.Host, job.StorageRoot.Port, job.StorageRoot.Path,
				job.StorageRoot.Username, job.StorageRoot.Password, job.StorageRoot.Domain,
				job.StorageRoot.MaxDepth,
			).Scan(&storageRootID)
			if err2 != nil {
				// ON CONFLICT DO NOTHING returns no rows — re-query
				err3 := db.QueryRowContext(ctx,
					"SELECT id FROM storage_roots WHERE name = ? LIMIT 1",
					job.StorageRoot.Name,
				).Scan(&storageRootID)
				if err3 != nil {
					return fmt.Errorf("storage root %q not found and could not be created: %w", job.StorageRoot.Name, err)
				}
			}
		} else {
			// SQLite path
			insertedID, insertErr := db.InsertReturningID(ctx,
				`INSERT OR IGNORE INTO storage_roots (name, protocol, host, port, path, username, password, domain, enabled, max_depth)
				 VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1, ?)`,
				job.StorageRoot.Name, job.StorageRoot.Protocol,
				job.StorageRoot.Host, job.StorageRoot.Port, job.StorageRoot.Path,
				job.StorageRoot.Username, job.StorageRoot.Password, job.StorageRoot.Domain,
				job.StorageRoot.MaxDepth,
			)
			if insertErr != nil {
				return fmt.Errorf("insert storage root: %w", insertErr)
			}
			storageRootID = insertedID
			if storageRootID == 0 {
				err2 := db.QueryRowContext(ctx,
					"SELECT id FROM storage_roots WHERE name = ? LIMIT 1",
					job.StorageRoot.Name,
				).Scan(&storageRootID)
				if err2 != nil {
					return fmt.Errorf("storage root %q not found and could not be created: %w", job.StorageRoot.Name, err)
				}
			}
		}
	}

	// Validate storage root ID
	if storageRootID <= 0 {
		return fmt.Errorf("invalid storage root ID %d for %q", storageRootID, job.StorageRoot.Name)
	}

	name := file.Name
	ext := strings.TrimPrefix(filepath.Ext(name), ".")
	mimeType := mime.TypeByExtension("." + ext)
	fileType := classifyFileType(ext)

	// Resolve parent directory ID (if path has a parent)
	var parentID *int64
	parentPath := pathpkg.Dir(path) // a remote path is a slash path whatever the host OS
	if parentPath != "." && parentPath != "/" && parentPath != "" {
		logger.Warn("Calling ensureDirectoryPathExists for file", zap.String("path", path), zap.String("parentPath", parentPath))
		pid, err := ensureDirectoryPathExists(ctx, db, storageRootID, parentPath, logger)
		if err != nil {
			logger.Error("Failed to ensure parent directory exists",
				zap.String("path", path),
				zap.String("parent_path", parentPath),
				zap.Error(err))
			// Continue with NULL parent_id rather than failing entirely
			status.incrementCounters(0, 0, 0, 0, 1)
		} else {
			parentID = pid
			logger.Warn("Parent directory resolved", zap.String("path", path), zap.Any("parentID", parentID))
		}
	}

	modifiedAt := file.ModTime
	if modifiedAt.IsZero() {
		modifiedAt = time.Now()
	}

	isDir := file.IsDir

	// Upsert: insert or update on conflict (same storage_root + path)
	// Use transaction for SQLite to handle foreign key constraints safely
	logger.Warn("File upsert dialect", zap.Bool("is_sqlite", db.Dialect().IsSQLite()), zap.Bool("is_postgres", db.Dialect().IsPostgres()), zap.String("path", path))
	if db.Dialect().IsSQLite() {
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin transaction for file insertion: %w", err)
		}
		defer tx.Rollback()

		// Temporarily disable foreign key checks for SQLite
		_, _ = tx.ExecContext(ctx, "PRAGMA foreign_keys = OFF")

		logger.Warn("SQLite file insertion", zap.String("path", path), zap.Bool("isDir", isDir), zap.Any("parentID", parentID), zap.Int64("storageRootID", storageRootID))
		// UPDATE-then-INSERT, not INSERT OR REPLACE: REPLACE deletes the old row and inserts a new one, so every rescan renumbered files.id while
		// media_files / media_items point at it (the foreign-key check is off here). The row keeps its id, created_at and hashes (WF22 R17b).
		// Not "INSERT .. ON CONFLICT DO UPDATE" either: the SQLite bundled with go-sqlcipher is older than 3.24 and rejects the syntax. Both
		// statements run in one transaction, which holds SQLite's write lock from the UPDATE on, so no other writer can slip a row in between.
		res, uerr := tx.ExecContext(ctx,
			`UPDATE files SET name = ?, extension = ?, mime_type = ?, file_type = ?, size = ?, is_directory = ?, modified_at = ?,
			   last_scan_at = CURRENT_TIMESTAMP, parent_id = COALESCE(?, parent_id), deleted = 0, deleted_at = NULL
			 WHERE storage_root_id = ? AND path = ?`,
			name, ext, mimeType, fileType, file.Size, isDir, modifiedAt, parentID, storageRootID, path,
		)
		err = uerr
		if err == nil {
			if n, _ := res.RowsAffected(); n == 0 {
				_, err = tx.ExecContext(ctx,
					`INSERT INTO files (storage_root_id, path, name, extension, mime_type, file_type, size, is_directory, modified_at, last_scan_at, parent_id)
					 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?)`,
					storageRootID, path, name, ext, mimeType, fileType, file.Size,
					isDir, modifiedAt, parentID,
				)
			}
		}
		if err != nil {
			return fmt.Errorf("insert file %s: %w", path, err)
		}

		_, _ = tx.ExecContext(ctx, "PRAGMA foreign_keys = ON")

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit file insertion transaction: %w", err)
		}
	} else {
		// PostgreSQL: use ON CONFLICT
		_, err = db.ExecContext(ctx,
			`INSERT INTO files (storage_root_id, path, name, extension, mime_type, file_type, size, is_directory, modified_at, last_scan_at, parent_id)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, ?)
			 ON CONFLICT(storage_root_id, path) DO UPDATE SET
			   size = excluded.size,
			   modified_at = excluded.modified_at,
			   last_scan_at = CURRENT_TIMESTAMP,
			   deleted = false,
			   deleted_at = NULL`,
			storageRootID, path, name, ext, mimeType, fileType, file.Size,
			isDir, modifiedAt, parentID,
		)
		if err != nil {
			return fmt.Errorf("insert file %s: %w", path, err)
		}
	}

	status.incrementCounters(1, 1, 0, 0, 0)
	return nil
}

// classifyFileType returns a file_type category based on extension.
func classifyFileType(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case "mp4", "mkv", "avi", "mov", "wmv", "flv", "m4v", "ts", "webm", "mpg", "mpeg":
		return "video"
	case "mp3", "flac", "wav", "m4a", "aac", "ogg", "wma", "ape", "opus":
		return "audio"
	case "jpg", "jpeg", "png", "gif", "bmp", "webp", "svg", "tiff", "ico":
		return "image"
	case "pdf", "epub", "mobi", "djvu", "cbr", "cbz", "cb7", "cbt":
		return "book"
	case "exe", "msi", "dmg", "pkg", "iso", "img", "deb", "rpm", "apk", "appimage":
		return "software"
	case "zip", "rar", "7z", "tar", "gz", "bz2", "xz", "zst":
		return "archive"
	case "txt", "md", "doc", "docx", "rtf", "odt", "csv", "xls", "xlsx":
		return "document"
	case "html", "htm", "css", "js", "go", "py", "java", "c", "cpp", "rs", "sh":
		return "code"
	default:
		return "other"
	}
}

// FTP, NFS and WebDAV (and every registered protocol) are scanned by GenericScanner (generic_scanner.go, PA-03).
