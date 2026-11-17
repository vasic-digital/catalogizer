package services

import (
	"catalogizer/database"
	"catalogizer/filesystem"
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// UniversalRenameTracker handles rename detection across all supported protocols
type UniversalRenameTracker struct {
	db               *database.DB
	logger           *zap.Logger
	pendingMoves     map[string]*UniversalPendingMove // key: protocol:storageRoot:hash:size
	pendingMovesMu   sync.RWMutex
	cleanupInterval  time.Duration
	moveWindow       time.Duration
	stopCh           chan struct{}
	wg               sync.WaitGroup
	protocolHandlers map[string]ProtocolHandler
}

// UniversalPendingMove tracks a potential move operation across any protocol
type UniversalPendingMove struct {
	Path         string
	StorageRoot  string
	Protocol     string
	Size         int64
	FileHash     *string
	IsDirectory  bool
	DeletedAt    time.Time
	FileID       int64
	ProtocolData map[string]interface{} // Protocol-specific metadata
}

// ProtocolHandler defines protocol-specific operations for rename handling
type ProtocolHandler interface {
	// GetFileIdentifier creates a unique identifier for a file in this protocol
	GetFileIdentifier(ctx context.Context, path string, size int64, isDir bool) (string, error)

	// PerformMove executes the actual move operation for this protocol
	PerformMove(ctx context.Context, client filesystem.FileSystemClient, oldPath, newPath string, isDir bool) error

	// ValidateMove checks if a move operation is valid for this protocol
	ValidateMove(ctx context.Context, client filesystem.FileSystemClient, oldPath, newPath string) error

	// GetMoveWindow returns the protocol-specific move detection window
	GetMoveWindow() time.Duration

	// SupportsRealTimeNotification indicates if the protocol supports real-time change notifications
	SupportsRealTimeNotification() bool
}

// NewUniversalRenameTracker creates a new universal rename tracker
func NewUniversalRenameTracker(db *database.DB, logger *zap.Logger) *UniversalRenameTracker {
	tracker := &UniversalRenameTracker{
		db:               db,
		logger:           logger,
		pendingMoves:     make(map[string]*UniversalPendingMove),
		cleanupInterval:  30 * time.Second,
		moveWindow:       10 * time.Second, // Default window, can be overridden per protocol
		stopCh:           make(chan struct{}),
		protocolHandlers: make(map[string]ProtocolHandler),
	}

	// Register default protocol handlers
	tracker.RegisterProtocolHandler("local", NewLocalProtocolHandler(logger))
	tracker.RegisterProtocolHandler("smb", NewSMBProtocolHandler(logger))
	tracker.RegisterProtocolHandler("ftp", NewFTPProtocolHandler(logger))
	tracker.RegisterProtocolHandler("nfs", NewNFSProtocolHandler(logger))
	tracker.RegisterProtocolHandler("webdav", NewWebDAVProtocolHandler(logger))

	return tracker
}

// RegisterProtocolHandler registers a protocol-specific handler
func (rt *UniversalRenameTracker) RegisterProtocolHandler(protocol string, handler ProtocolHandler) {
	rt.protocolHandlers[protocol] = handler
}

// Start begins the universal rename tracking service
func (rt *UniversalRenameTracker) Start() error {
	rt.logger.Info("Starting universal rename tracker service")

	// Start cleanup worker
	rt.wg.Add(1)
	go rt.cleanupWorker()

	// Create rename tracking tables if they don't exist
	if err := rt.InitializeTables(); err != nil {
		return fmt.Errorf("failed to initialize rename tracking tables: %w", err)
	}

	return nil
}

// Stop stops the universal rename tracking service
func (rt *UniversalRenameTracker) Stop() {
	rt.logger.Info("Stopping universal rename tracker service")
	close(rt.stopCh)
	rt.wg.Wait()
	rt.logger.Info("Universal rename tracker service stopped")
}

// TrackDelete tracks a file/directory deletion for potential move detection
func (rt *UniversalRenameTracker) TrackDelete(ctx context.Context, fileID int64, path, storageRoot, protocol string, size int64, fileHash *string, isDirectory bool, protocolData map[string]interface{}) {
	handler, exists := rt.protocolHandlers[protocol]
	if !exists {
		rt.logger.Warn("No handler for protocol", zap.String("protocol", protocol))
		return
	}

	// Use hash-based key when available for precise matching;
	// fall back to protocol handler identifier (size-based).
	var identifier string
	if fileHash != nil {
		identifier = rt.createFallbackKey(protocol, storageRoot, fileHash, size, isDirectory)
	} else {
		var err error
		identifier, err = handler.GetFileIdentifier(ctx, path, size, isDirectory)
		if err != nil {
			rt.logger.Error("Failed to get file identifier",
				zap.String("protocol", protocol),
				zap.String("path", path),
				zap.Error(err))
			identifier = rt.createFallbackKey(protocol, storageRoot, fileHash, size, isDirectory)
		}
	}

	key := fmt.Sprintf("%s:%s:%s", protocol, storageRoot, identifier)

	rt.pendingMovesMu.Lock()
	rt.pendingMoves[key] = &UniversalPendingMove{
		Path:         path,
		StorageRoot:  storageRoot,
		Protocol:     protocol,
		Size:         size,
		FileHash:     fileHash,
		IsDirectory:  isDirectory,
		DeletedAt:    time.Now(),
		FileID:       fileID,
		ProtocolData: protocolData,
	}
	rt.pendingMovesMu.Unlock()

	rt.logger.Debug("Tracking potential universal move deletion",
		zap.String("path", path),
		zap.String("storage_root", storageRoot),
		zap.String("protocol", protocol),
		zap.Int64("file_id", fileID))
}

// DetectCreate checks if a file creation is actually a move from a deletion
func (rt *UniversalRenameTracker) DetectCreate(ctx context.Context, newPath, storageRoot, protocol string, size int64, fileHash *string, isDirectory bool, protocolData map[string]interface{}) (*UniversalPendingMove, bool) {
	handler, exists := rt.protocolHandlers[protocol]
	if !exists {
		rt.logger.Warn("No handler for protocol", zap.String("protocol", protocol))
		return nil, false
	}

	// Use hash-based key when available for precise matching;
	// fall back to protocol handler identifier (size-based).
	var identifier string
	if fileHash != nil {
		identifier = rt.createFallbackKey(protocol, storageRoot, fileHash, size, isDirectory)
	} else {
		var err error
		identifier, err = handler.GetFileIdentifier(ctx, newPath, size, isDirectory)
		if err != nil {
			rt.logger.Error("Failed to get file identifier for create detection",
				zap.String("protocol", protocol),
				zap.String("path", newPath),
				zap.Error(err))
			identifier = rt.createFallbackKey(protocol, storageRoot, fileHash, size, isDirectory)
		}
	}

	key := fmt.Sprintf("%s:%s:%s", protocol, storageRoot, identifier)

	rt.pendingMovesMu.Lock()
	pendingMove, exists := rt.pendingMoves[key]
	if exists {
		delete(rt.pendingMoves, key)
	}
	rt.pendingMovesMu.Unlock()

	if !exists {
		return nil, false
	}

	// Check if the move happened within the protocol-specific time window
	moveWindow := handler.GetMoveWindow()
	if time.Since(pendingMove.DeletedAt) > moveWindow {
		rt.logger.Debug("Move window expired",
			zap.String("protocol", protocol),
			zap.String("old_path", pendingMove.Path),
			zap.String("new_path", newPath),
			zap.Duration("elapsed", time.Since(pendingMove.DeletedAt)),
			zap.Duration("window", moveWindow))
		return nil, false
	}

	rt.logger.Info("Detected universal file/directory move",
		zap.String("old_path", pendingMove.Path),
		zap.String("new_path", newPath),
		zap.String("storage_root", storageRoot),
		zap.String("protocol", protocol),
		zap.Bool("is_directory", isDirectory))

	return pendingMove, true
}

// ProcessMove handles a detected move operation efficiently across protocols
func (rt *UniversalRenameTracker) ProcessMove(ctx context.Context, client filesystem.FileSystemClient, oldMove *UniversalPendingMove, newPath string) error {
	handler, exists := rt.protocolHandlers[oldMove.Protocol]
	if !exists {
		return fmt.Errorf("no handler for protocol: %s", oldMove.Protocol)
	}

	// Validate the move operation
	if err := handler.ValidateMove(ctx, client, oldMove.Path, newPath); err != nil {
		return fmt.Errorf("move validation failed: %w", err)
	}

	tx, err := rt.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Record the rename event
	renameEventID, err := rt.recordUniversalRenameEvent(ctx, tx, oldMove, newPath)
	if err != nil {
		return fmt.Errorf("failed to record rename event: %w", err)
	}

	// Perform protocol-specific move if needed
	if handler.SupportsRealTimeNotification() {
		// For protocols with real-time notifications, just update database
		if oldMove.IsDirectory {
			err = rt.moveDirectory(tx, oldMove.Path, newPath, oldMove.StorageRoot)
		} else {
			err = rt.moveFile(tx, oldMove.FileID, newPath)
		}
	} else {
		// For polling-based protocols, perform actual file system move
		if err = handler.PerformMove(ctx, client, oldMove.Path, newPath, oldMove.IsDirectory); err != nil {
			rt.markRenameEventStatus(tx, renameEventID, "failed")
			return fmt.Errorf("failed to perform protocol move: %w", err)
		}

		if oldMove.IsDirectory {
			err = rt.moveDirectory(tx, oldMove.Path, newPath, oldMove.StorageRoot)
		} else {
			err = rt.moveFile(tx, oldMove.FileID, newPath)
		}
	}

	if err != nil {
		rt.markRenameEventStatus(tx, renameEventID, "failed")
		return fmt.Errorf("failed to process move: %w", err)
	}

	// Mark rename event as processed
	if err = rt.markRenameEventStatus(tx, renameEventID, "processed"); err != nil {
		return fmt.Errorf("failed to mark rename event as processed: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit move transaction: %w", err)
	}

	rt.logger.Info("Successfully processed universal move operation",
		zap.String("old_path", oldMove.Path),
		zap.String("new_path", newPath),
		zap.String("protocol", oldMove.Protocol),
		zap.Bool("is_directory", oldMove.IsDirectory),
		zap.Int64("rename_event_id", renameEventID))

	return nil
}

// createFallbackKey creates a fallback key when protocol-specific identification fails
func (rt *UniversalRenameTracker) createFallbackKey(protocol, storageRoot string, fileHash *string, size int64, isDirectory bool) string {
	hashStr := "nil"
	if fileHash != nil {
		hashStr = *fileHash
	}

	dirStr := "false"
	if isDirectory {
		dirStr = "true"
	}

	return fmt.Sprintf("fallback:%s:%s:%d:%s", protocol, hashStr, size, dirStr)
}

// moveFile updates a single file's path and metadata
func (rt *UniversalRenameTracker) moveFile(tx *database.Tx, fileID int64, newPath string) error {
	newName := filepath.Base(newPath)
	newDir := filepath.Dir(newPath)

	var parentID *int64
	if newDir != "/" && newDir != "." {
		parentQuery := `SELECT id FROM files WHERE path = ? AND is_directory = true LIMIT 1`
		err := tx.QueryRow(parentQuery, newDir).Scan(&parentID)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("failed to get parent directory: %w", err)
		}
	}

	updateQuery := `
		UPDATE files
		SET path = ?, name = ?, parent_id = ?, modified_at = CURRENT_TIMESTAMP,
		    last_scan_at = CURRENT_TIMESTAMP
		WHERE id = ?`

	_, err := tx.Exec(updateQuery, newPath, newName, parentID, fileID)
	if err != nil {
		return fmt.Errorf("failed to update file path: %w", err)
	}

	return nil
}

// moveDirectory updates a directory and all its children
func (rt *UniversalRenameTracker) moveDirectory(tx *database.Tx, oldPath, newPath, storageRootName string) error {
	query := `
		SELECT id, path, is_directory
		FROM files
		WHERE storage_root_id = (SELECT id FROM storage_roots WHERE name = ?)
		  AND (path = ? OR path LIKE ?)
		ORDER BY LENGTH(path) ASC`

	oldPathPattern := oldPath + "/%"
	rows, err := tx.Query(query, storageRootName, oldPath, oldPathPattern)
	if err != nil {
		return fmt.Errorf("failed to query directory contents: %w", err)
	}
	defer rows.Close()

	type fileUpdate struct {
		ID          int64
		OldPath     string
		IsDirectory bool
	}

	var updates []fileUpdate
	for rows.Next() {
		var update fileUpdate
		if err := rows.Scan(&update.ID, &update.OldPath, &update.IsDirectory); err != nil {
			return fmt.Errorf("failed to scan file for update: %w", err)
		}
		updates = append(updates, update)
	}

	for _, update := range updates {
		var updatedPath string
		if update.OldPath == oldPath {
			updatedPath = newPath
		} else {
			relativePath := update.OldPath[len(oldPath):]
			updatedPath = newPath + relativePath
		}

		if err := rt.moveFile(tx, update.ID, updatedPath); err != nil {
			return fmt.Errorf("failed to update path for file ID %d: %w", update.ID, err)
		}
	}

	return nil
}

// recordUniversalRenameEvent creates a record of the rename operation
func (rt *UniversalRenameTracker) recordUniversalRenameEvent(ctx context.Context, tx *database.Tx, oldMove *UniversalPendingMove, newPath string) (int64, error) {
	query := `
		INSERT INTO universal_rename_events (storage_root_id, protocol, old_path, new_path, is_directory, size, file_hash, detected_at, status)
		VALUES ((SELECT id FROM storage_roots WHERE name = ?), ?, ?, ?, ?, ?, ?, ?, 'pending')`

	id, err := rt.db.TxInsertReturningID(ctx, tx, query, oldMove.StorageRoot, oldMove.Protocol, oldMove.Path, newPath, oldMove.IsDirectory, oldMove.Size, oldMove.FileHash, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to insert universal rename event: %w", err)
	}

	return id, nil
}

// markRenameEventStatus updates the status of a rename event
func (rt *UniversalRenameTracker) markRenameEventStatus(tx *database.Tx, eventID int64, status string) error {
	query := `UPDATE universal_rename_events SET status = ?, processed_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := tx.Exec(query, status, eventID)
	return err
}

// cleanupWorker periodically cleans up expired pending moves
func (rt *UniversalRenameTracker) cleanupWorker() {
	defer rt.wg.Done()

	ticker := time.NewTicker(rt.cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-rt.stopCh:
			return
		case <-ticker.C:
			rt.cleanupExpiredMoves()
		}
	}
}

// cleanupExpiredMoves removes pending moves that have exceeded their protocol-specific time windows
func (rt *UniversalRenameTracker) cleanupExpiredMoves() {
	rt.pendingMovesMu.Lock()
	defer rt.pendingMovesMu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	for key, move := range rt.pendingMoves {
		handler, exists := rt.protocolHandlers[move.Protocol]
		if !exists {
			// Use default window if no handler
			if now.Sub(move.DeletedAt) > rt.moveWindow {
				expiredKeys = append(expiredKeys, key)
			}
			continue
		}

		moveWindow := handler.GetMoveWindow()
		if now.Sub(move.DeletedAt) > moveWindow {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		move := rt.pendingMoves[key]
		delete(rt.pendingMoves, key)

		rt.logger.Debug("Cleaned up expired universal pending move",
			zap.String("path", move.Path),
			zap.String("storage_root", move.StorageRoot),
			zap.String("protocol", move.Protocol),
			zap.Duration("age", now.Sub(move.DeletedAt)))
	}

	if len(expiredKeys) > 0 {
		rt.logger.Debug("Cleaned up expired universal pending moves", zap.Int("count", len(expiredKeys)))
	}
}

// GetStatistics returns statistics about universal rename detection
func (rt *UniversalRenameTracker) GetStatistics() map[string]interface{} {
	rt.pendingMovesMu.RLock()

	// Count pending moves by protocol
	pendingByProtocol := make(map[string]int)
	totalPending := 0

	for _, move := range rt.pendingMoves {
		pendingByProtocol[move.Protocol]++
		totalPending++
	}

	rt.pendingMovesMu.RUnlock()

	stats := map[string]interface{}{
		"total_pending_moves": totalPending,
		"pending_by_protocol": pendingByProtocol,
		"supported_protocols": rt.getSupportedProtocols(),
	}

	// Get database statistics
	var totalRenames, successfulRenames int
	rt.db.QueryRow("SELECT COUNT(*) FROM universal_rename_events").Scan(&totalRenames)
	rt.db.QueryRow("SELECT COUNT(*) FROM universal_rename_events WHERE status = 'processed'").Scan(&successfulRenames)

	stats["total_renames"] = totalRenames
	stats["successful_renames"] = successfulRenames

	if totalRenames > 0 {
		stats["success_rate"] = float64(successfulRenames) / float64(totalRenames) * 100
	} else {
		stats["success_rate"] = 0.0
	}

	return stats
}

// getSupportedProtocols returns a list of supported protocols
func (rt *UniversalRenameTracker) getSupportedProtocols() []string {
	protocols := make([]string, 0, len(rt.protocolHandlers))
	for protocol := range rt.protocolHandlers {
		protocols = append(protocols, protocol)
	}
	return protocols
}

// InitializeTables creates the universal rename tracking tables
func (rt *UniversalRenameTracker) InitializeTables() error {
	query := `
		CREATE TABLE IF NOT EXISTS universal_rename_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			storage_root_id INTEGER NOT NULL,
			protocol TEXT NOT NULL,
			old_path TEXT NOT NULL,
			new_path TEXT NOT NULL,
			is_directory BOOLEAN NOT NULL,
			size INTEGER NOT NULL,
			file_hash TEXT,
			detected_at TIMESTAMP NOT NULL,
			processed_at TIMESTAMP,
			status TEXT NOT NULL DEFAULT 'pending',
			FOREIGN KEY (storage_root_id) REFERENCES storage_roots (id)
		);

		CREATE INDEX IF NOT EXISTS idx_universal_rename_events_storage_root ON universal_rename_events(storage_root_id);
		CREATE INDEX IF NOT EXISTS idx_universal_rename_events_protocol ON universal_rename_events(protocol);
		CREATE INDEX IF NOT EXISTS idx_universal_rename_events_detected_at ON universal_rename_events(detected_at);
		CREATE INDEX IF NOT EXISTS idx_universal_rename_events_status ON universal_rename_events(status);
	`

	_, err := rt.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create universal rename tracking tables: %w", err)
	}

	return nil
}
