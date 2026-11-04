package services

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"catalogizer/database"

	"go.uber.org/zap"
)

// RenameEvent represents a file/directory rename operation
type RenameEvent struct {
	ID            int64      `json:"id"`
	StorageRootID int64      `json:"storage_root_id"`
	OldPath       string     `json:"old_path"`
	NewPath       string     `json:"new_path"`
	IsDirectory   bool       `json:"is_directory"`
	Size          int64      `json:"size"`
	FileHash      *string    `json:"file_hash,omitempty"`
	DetectedAt    time.Time  `json:"detected_at"`
	ProcessedAt   *time.Time `json:"processed_at,omitempty"`
	Status        string     `json:"status"` // pending, processed, failed
}

// PendingMove tracks a potential move operation
type PendingMove struct {
	Path        string
	StorageRoot string
	Size        int64
	FileHash    *string
	IsDirectory bool
	DeletedAt   time.Time
	FileID      int64
}

const maxPendingMoves = 10000 // prevent unbounded map growth

// RenameTracker efficiently detects and handles file/directory renames
type RenameTracker struct {
	db              *database.DB
	logger          *zap.Logger
	PendingMoves    map[string]*PendingMove // key: storageRoot:hash:size
	PendingMovesMu  sync.RWMutex
	cleanupInterval time.Duration
	moveWindow      time.Duration // time window to detect moves
	stopCh          chan struct{}
	wg              sync.WaitGroup
}

// NewRenameTracker creates a new rename tracker
func NewRenameTracker(db *database.DB, logger *zap.Logger) *RenameTracker {
	return &RenameTracker{
		db:              db,
		logger:          logger,
		PendingMoves:    make(map[string]*PendingMove),
		cleanupInterval: 30 * time.Second,
		moveWindow:      5 * time.Second, // moves should happen within 5 seconds
		stopCh:          make(chan struct{}),
	}
}

// Start begins the rename tracking service
func (rt *RenameTracker) Start() error {
	rt.logger.Info("Starting rename tracker service")

	// Start cleanup worker
	rt.wg.Add(1)
	go rt.cleanupWorker()

	// Create rename tracking tables if they don't exist
	if err := rt.InitializeTables(); err != nil {
		return fmt.Errorf("failed to initialize rename tracking tables: %w", err)
	}

	return nil
}

// Stop stops the rename tracking service
func (rt *RenameTracker) Stop() {
	rt.logger.Info("Stopping rename tracker service")
	close(rt.stopCh)
	rt.wg.Wait()
	rt.logger.Info("Rename tracker service stopped")
}

// TrackDelete tracks a file/directory deletion for potential move detection
func (rt *RenameTracker) TrackDelete(ctx context.Context, fileID int64, path, storageRoot string, size int64, fileHash *string, isDirectory bool) {
	// Create move tracking key
	key := rt.CreateMoveKey(storageRoot, fileHash, size, isDirectory)

	rt.PendingMovesMu.Lock()
	// Evict oldest entries if map exceeds size limit
	if len(rt.PendingMoves) >= maxPendingMoves {
		rt.evictOldestLocked()
	}
	rt.PendingMoves[key] = &PendingMove{
		Path:        path,
		StorageRoot: storageRoot,
		Size:        size,
		FileHash:    fileHash,
		IsDirectory: isDirectory,
		DeletedAt:   time.Now(),
		FileID:      fileID,
	}
	rt.PendingMovesMu.Unlock()

	rt.logger.Debug("Tracking potential move deletion",
		zap.String("path", path),
		zap.String("storage_root", storageRoot),
		zap.Int64("file_id", fileID))
}

// DetectCreate checks if a file creation is actually a move from a deletion
func (rt *RenameTracker) DetectCreate(ctx context.Context, newPath, storageRoot string, size int64, fileHash *string, isDirectory bool) (*PendingMove, bool) {
	key := rt.CreateMoveKey(storageRoot, fileHash, size, isDirectory)

	rt.PendingMovesMu.Lock()
	pendingMove, exists := rt.PendingMoves[key]
	if exists {
		delete(rt.PendingMoves, key)
	}
	rt.PendingMovesMu.Unlock()

	if !exists {
		return nil, false
	}

	// Check if the move happened within the time window
	if time.Since(pendingMove.DeletedAt) > rt.moveWindow {
		rt.logger.Debug("Move window expired",
			zap.String("old_path", pendingMove.Path),
			zap.String("new_path", newPath),
			zap.Duration("elapsed", time.Since(pendingMove.DeletedAt)))
		return nil, false
	}

	rt.logger.Info("Detected file/directory move",
		zap.String("old_path", pendingMove.Path),
		zap.String("new_path", newPath),
		zap.String("storage_root", storageRoot),
		zap.Bool("is_directory", isDirectory))

	return pendingMove, true
}

// ProcessMove handles a detected move operation efficiently
func (rt *RenameTracker) ProcessMove(ctx context.Context, oldMove *PendingMove, newPath string) error {
	tx, err := rt.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Record the rename event
	renameEventID, err := rt.recordRenameEvent(ctx, tx, oldMove, newPath)
	if err != nil {
		return fmt.Errorf("failed to record rename event: %w", err)
	}

	if oldMove.IsDirectory {
		// Handle directory move - update all child paths
		err = rt.moveDirectory(tx, oldMove.Path, newPath, oldMove.StorageRoot)
	} else {
		// Handle file move - update single file
		err = rt.moveFile(tx, oldMove.FileID, newPath)
	}

	if err != nil {
		// Mark rename event as failed
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

	rt.logger.Info("Successfully processed move operation",
		zap.String("old_path", oldMove.Path),
		zap.String("new_path", newPath),
		zap.Bool("is_directory", oldMove.IsDirectory),
		zap.Int64("rename_event_id", renameEventID))

	return nil
}

// moveFile updates a single file's path and metadata
func (rt *RenameTracker) moveFile(tx *sql.Tx, fileID int64, newPath string) error {
	// Extract new filename and directory info
	newName := filepath.Base(newPath)
	newDir := filepath.Dir(newPath)

	// Get parent directory ID
	var parentID *int64
	if newDir != "/" && newDir != "." {
		parentQuery := `SELECT id FROM files WHERE path = ? AND is_directory = true LIMIT 1`
		err := tx.QueryRow(parentQuery, newDir).Scan(&parentID)
		if err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("failed to get parent directory: %w", err)
		}
	}

	// Update file record
	updateQuery := `
		UPDATE files
		SET path = ?, name = ?, parent_id = ?, updated_at = CURRENT_TIMESTAMP,
		    last_scan_at = CURRENT_TIMESTAMP
		WHERE id = ?`

	_, err := tx.Exec(updateQuery, newPath, newName, parentID, fileID)
	if err != nil {
		return fmt.Errorf("failed to update file path: %w", err)
	}

	return nil
}

// moveDirectory updates a directory and all its children
func (rt *RenameTracker) moveDirectory(tx *sql.Tx, oldPath, newPath, storageRoot string) error {
	// Get all files/directories that need to be updated
	query := `
		SELECT id, path, is_directory
		FROM files
		WHERE storage_root_id = (SELECT id FROM storage_roots WHERE name = ?)
		  AND (path = ? OR path LIKE ?)
		ORDER BY LENGTH(path) ASC` // Process parents before children

	oldPathPattern := oldPath + "/%"
	rows, err := tx.Query(query, storageRoot, oldPath, oldPathPattern)
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

	// Update each file/directory path
	for _, update := range updates {
		var updatedPath string
		if update.OldPath == oldPath {
			// This is the directory itself
			updatedPath = newPath
		} else {
			// This is a child - replace the old path prefix with new path
			relativePath := update.OldPath[len(oldPath):]
			updatedPath = newPath + relativePath
		}

		// Update the file record
		newName := filepath.Base(updatedPath)
		newDir := filepath.Dir(updatedPath)

		// Get parent directory ID
		var parentID *int64
		if newDir != "/" && newDir != "." && newDir != newPath {
			parentQuery := `SELECT id FROM files WHERE path = ? AND is_directory = true LIMIT 1`
			err := tx.QueryRow(parentQuery, newDir).Scan(&parentID)
			if err != nil && err != sql.ErrNoRows {
				return fmt.Errorf("failed to get parent directory for %s: %w", updatedPath, err)
			}
		}

		updateQuery := `
			UPDATE files
			SET path = ?, name = ?, parent_id = ?, updated_at = CURRENT_TIMESTAMP,
			    last_scan_at = CURRENT_TIMESTAMP
			WHERE id = ?`

		_, err := tx.Exec(updateQuery, updatedPath, newName, parentID, update.ID)
		if err != nil {
			return fmt.Errorf("failed to update path for file ID %d: %w", update.ID, err)
		}

		rt.logger.Debug("Updated file path",
			zap.String("old_path", update.OldPath),
			zap.String("new_path", updatedPath),
			zap.Int64("file_id", update.ID))
	}

	return nil
}

// recordRenameEvent creates a record of the rename operation
func (rt *RenameTracker) recordRenameEvent(ctx context.Context, tx *sql.Tx, oldMove *PendingMove, newPath string) (int64, error) {
	query := `
		INSERT INTO rename_events (storage_root_id, old_path, new_path, is_directory, size, file_hash, detected_at, status)
		VALUES ((SELECT id FROM storage_roots WHERE name = ?), ?, ?, ?, ?, ?, ?, 'pending')`

	id, err := rt.db.TxInsertReturningID(ctx, tx, query, oldMove.StorageRoot, oldMove.Path, newPath, oldMove.IsDirectory, oldMove.Size, oldMove.FileHash, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to insert rename event: %w", err)
	}

	return id, nil
}

// markRenameEventStatus updates the status of a rename event
func (rt *RenameTracker) markRenameEventStatus(tx *sql.Tx, eventID int64, status string) error {
	query := `UPDATE rename_events SET status = ?, processed_at = CURRENT_TIMESTAMP WHERE id = ?`
	_, err := tx.Exec(query, status, eventID)
	return err
}

// createMoveKey creates a unique key for tracking potential moves
func (rt *RenameTracker) CreateMoveKey(storageRoot string, fileHash *string, size int64, isDirectory bool) string {
	hashStr := "nil"
	if fileHash != nil {
		hashStr = *fileHash
	}

	dirStr := "false"
	if isDirectory {
		dirStr = "true"
	}

	return fmt.Sprintf("%s:%s:%d:%s", storageRoot, hashStr, size, dirStr)
}

// cleanupWorker periodically cleans up expired pending moves
func (rt *RenameTracker) cleanupWorker() {
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

// evictOldestLocked removes the oldest 10% of entries when map is full.
// Must be called with PendingMovesMu held.
func (rt *RenameTracker) evictOldestLocked() {
	evictCount := maxPendingMoves / 10
	if evictCount < 1 {
		evictCount = 1
	}

	type entry struct {
		key       string
		deletedAt time.Time
	}
	entries := make([]entry, 0, len(rt.PendingMoves))
	for k, v := range rt.PendingMoves {
		entries = append(entries, entry{key: k, deletedAt: v.DeletedAt})
	}

	// Sort by DeletedAt ascending (oldest first)
	for i := 0; i < len(entries)-1; i++ {
		for j := i + 1; j < len(entries); j++ {
			if entries[j].deletedAt.Before(entries[i].deletedAt) {
				entries[i], entries[j] = entries[j], entries[i]
			}
		}
	}

	for i := 0; i < evictCount && i < len(entries); i++ {
		delete(rt.PendingMoves, entries[i].key)
	}

	rt.logger.Warn("PendingMoves map reached capacity, evicted oldest entries",
		zap.Int("evicted", evictCount),
		zap.Int("max_capacity", maxPendingMoves))
}

// cleanupExpiredMoves removes pending moves that have exceeded the time window
func (rt *RenameTracker) cleanupExpiredMoves() {
	rt.PendingMovesMu.Lock()
	defer rt.PendingMovesMu.Unlock()

	now := time.Now()
	expiredKeys := make([]string, 0)

	for key, move := range rt.PendingMoves {
		if now.Sub(move.DeletedAt) > rt.moveWindow {
			expiredKeys = append(expiredKeys, key)
		}
	}

	for _, key := range expiredKeys {
		move := rt.PendingMoves[key]
		delete(rt.PendingMoves, key)

		rt.logger.Debug("Cleaned up expired pending move",
			zap.String("path", move.Path),
			zap.String("storage_root", move.StorageRoot),
			zap.Duration("age", now.Sub(move.DeletedAt)))
	}

	if len(expiredKeys) > 0 {
		rt.logger.Debug("Cleaned up expired pending moves", zap.Int("count", len(expiredKeys)))
	}
}

// GetRenameEvents returns recent rename events for monitoring
func (rt *RenameTracker) GetRenameEvents(ctx context.Context, limit int) ([]RenameEvent, error) {
	query := `
		SELECT re.id, re.storage_root_id, re.old_path, re.new_path, re.is_directory,
		       re.size, re.file_hash, re.detected_at, re.processed_at, re.status
		FROM rename_events re
		ORDER BY re.detected_at DESC
		LIMIT ?`

	rows, err := rt.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query rename events: %w", err)
	}
	defer rows.Close()

	events := []RenameEvent{}
	for rows.Next() {
		var event RenameEvent
		err := rows.Scan(
			&event.ID, &event.StorageRootID, &event.OldPath, &event.NewPath,
			&event.IsDirectory, &event.Size, &event.FileHash, &event.DetectedAt,
			&event.ProcessedAt, &event.Status,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan rename event: %w", err)
		}
		events = append(events, event)
	}

	return events, nil
}

// GetStatistics returns statistics about rename detection
func (rt *RenameTracker) GetStatistics() map[string]interface{} {
	rt.PendingMovesMu.RLock()
	pendingCount := len(rt.PendingMoves)
	rt.PendingMovesMu.RUnlock()

	stats := map[string]interface{}{
		"pending_moves": pendingCount,
		"move_window":   rt.moveWindow.String(),
	}

	// Get database statistics
	var totalRenames, successfulRenames int
	rt.db.QueryRow("SELECT COUNT(*) FROM rename_events").Scan(&totalRenames)
	rt.db.QueryRow("SELECT COUNT(*) FROM rename_events WHERE status = 'processed'").Scan(&successfulRenames)

	stats["total_renames"] = totalRenames
	stats["successful_renames"] = successfulRenames

	if totalRenames > 0 {
		stats["success_rate"] = float64(successfulRenames) / float64(totalRenames) * 100
	} else {
		stats["success_rate"] = 0.0
	}

	return stats
}

// InitializeTables creates the rename tracking tables
func (rt *RenameTracker) InitializeTables() error {
	query := `
		CREATE TABLE IF NOT EXISTS rename_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			storage_root_id INTEGER NOT NULL,
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

		CREATE INDEX IF NOT EXISTS idx_rename_events_storage_root ON rename_events(storage_root_id);
		CREATE INDEX IF NOT EXISTS idx_rename_events_detected_at ON rename_events(detected_at);
		CREATE INDEX IF NOT EXISTS idx_rename_events_status ON rename_events(status);
	`

	_, err := rt.db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create rename tracking tables: %w", err)
	}

	return nil
}
