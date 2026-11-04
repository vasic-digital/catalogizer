package services

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SyncState represents the current state of a sync operation
type SyncState string

const (
	SyncStateIdle         SyncState = "idle"
	SyncStateConnecting   SyncState = "connecting"
	SyncStateScanning     SyncState = "scanning"
	SyncStateTransferring SyncState = "transferring"
	SyncStateVerifying    SyncState = "verifying"
	SyncStateCompleted    SyncState = "completed"
	SyncStateFailed       SyncState = "failed"
	SyncStateCancelled    SyncState = "cancelled"
)

// SyncOperation represents a single sync operation with state management
type SyncOperation struct {
	ID         int
	SessionID  int
	EndpointID int
	UserID     int
	State      SyncState
	StartTime  time.Time
	EndTime    *time.Time
	Progress   int // 0-100
	TotalFiles int
	Processed  int
	Failed     int
	Skipped    int
	Error      error
	CancelFunc context.CancelFunc
	mu         sync.RWMutex
}

// StateMachine manages sync state transitions
type SyncStateMachine struct {
	operation   *SyncOperation
	transitions map[SyncState][]SyncState
	mu          sync.RWMutex
}

// NewSyncStateMachine creates a new state machine for a sync operation
func NewSyncStateMachine(op *SyncOperation) *SyncStateMachine {
	return &SyncStateMachine{
		operation: op,
		transitions: map[SyncState][]SyncState{
			SyncStateIdle:         {SyncStateConnecting, SyncStateCancelled},
			SyncStateConnecting:   {SyncStateScanning, SyncStateFailed, SyncStateCancelled},
			SyncStateScanning:     {SyncStateTransferring, SyncStateFailed, SyncStateCancelled},
			SyncStateTransferring: {SyncStateVerifying, SyncStateFailed, SyncStateCancelled},
			SyncStateVerifying:    {SyncStateCompleted, SyncStateFailed},
			SyncStateCompleted:    {},
			SyncStateFailed:       {},
			SyncStateCancelled:    {},
		},
	}
}

// CanTransition checks if a state transition is valid
func (sm *SyncStateMachine) CanTransition(to SyncState) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	current := sm.operation.State
	allowed, exists := sm.transitions[current]
	if !exists {
		return false
	}

	for _, state := range allowed {
		if state == to {
			return true
		}
	}
	return false
}

// Transition attempts to transition to a new state
func (sm *SyncStateMachine) Transition(to SyncState) error {
	if !sm.CanTransition(to) {
		return fmt.Errorf("invalid state transition from %s to %s", sm.operation.State, to)
	}

	sm.operation.mu.Lock()
	defer sm.operation.mu.Unlock()
	sm.operation.State = to

	if to == SyncStateCompleted || to == SyncStateFailed || to == SyncStateCancelled {
		now := time.Now()
		sm.operation.EndTime = &now
	}

	return nil
}

// PermissionChecker defines the interface for checking user permissions
type PermissionChecker interface {
	CheckPermission(userID int, permission string) (bool, error)
}

// SyncOperationManager manages active sync operations
type SyncOperationManager struct {
	operations map[int]*SyncOperation
	mu         sync.RWMutex
	timeout    time.Duration
}

// NewSyncOperationManager creates a new operation manager
func NewSyncOperationManager(timeout time.Duration) *SyncOperationManager {
	if timeout <= 0 {
		timeout = 30 * time.Minute // Default 30 minute timeout
	}

	return &SyncOperationManager{
		operations: make(map[int]*SyncOperation),
		timeout:    timeout,
	}
}

// Register registers a new sync operation
func (m *SyncOperationManager) Register(sessionID, endpointID, userID int) (*SyncOperation, context.Context) {
	ctx, cancel := context.WithTimeout(context.Background(), m.timeout)

	op := &SyncOperation{
		ID:         sessionID,
		SessionID:  sessionID,
		EndpointID: endpointID,
		UserID:     userID,
		State:      SyncStateIdle,
		StartTime:  time.Now(),
		CancelFunc: cancel,
	}

	m.mu.Lock()
	m.operations[sessionID] = op
	m.mu.Unlock()

	return op, ctx
}

// Get retrieves an operation by session ID
func (m *SyncOperationManager) Get(sessionID int) (*SyncOperation, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	op, exists := m.operations[sessionID]
	return op, exists
}

// Cancel cancels an operation
func (m *SyncOperationManager) Cancel(sessionID int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	op, exists := m.operations[sessionID]
	if !exists {
		return fmt.Errorf("operation not found")
	}

	op.mu.Lock()
	defer op.mu.Unlock()

	if op.CancelFunc != nil {
		op.CancelFunc()
	}
	op.State = SyncStateCancelled
	now := time.Now()
	op.EndTime = &now

	return nil
}

// Complete marks an operation as complete
func (m *SyncOperationManager) Complete(sessionID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if op, exists := m.operations[sessionID]; exists {
		op.mu.Lock()
		op.State = SyncStateCompleted
		now := time.Now()
		op.EndTime = &now
		if op.CancelFunc != nil {
			op.CancelFunc()
		}
		op.mu.Unlock()
	}
}

// Fail marks an operation as failed
func (m *SyncOperationManager) Fail(sessionID int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if op, exists := m.operations[sessionID]; exists {
		op.mu.Lock()
		op.State = SyncStateFailed
		op.Error = err
		now := time.Now()
		op.EndTime = &now
		if op.CancelFunc != nil {
			op.CancelFunc()
		}
		op.mu.Unlock()
	}
}

// Cleanup removes completed operations older than the given duration
func (m *SyncOperationManager) Cleanup(olderThan time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for id, op := range m.operations {
		op.mu.RLock()
		shouldRemove := (op.State == SyncStateCompleted || op.State == SyncStateFailed || op.State == SyncStateCancelled) &&
			op.EndTime != nil && now.Sub(*op.EndTime) > olderThan
		op.mu.RUnlock()

		if shouldRemove {
			delete(m.operations, id)
		}
	}
}

// UpdateProgress updates the progress of an operation
func (m *SyncOperationManager) UpdateProgress(sessionID int, processed, failed, skipped, total int) {
	m.mu.RLock()
	op, exists := m.operations[sessionID]
	m.mu.RUnlock()

	if !exists {
		return
	}

	op.mu.Lock()
	defer op.mu.Unlock()

	op.Processed = processed
	op.Failed = failed
	op.Skipped = skipped
	op.TotalFiles = total

	if total > 0 {
		op.Progress = (processed + failed + skipped) * 100 / total
	}
}

// GetAllActive returns all active operations
func (m *SyncOperationManager) GetAllActive() []*SyncOperation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	active := []*SyncOperation{}
	for _, op := range m.operations {
		op.mu.RLock()
		isActive := op.State != SyncStateCompleted && op.State != SyncStateFailed && op.State != SyncStateCancelled
		op.mu.RUnlock()

		if isActive {
			active = append(active, op)
		}
	}

	return active
}

// SyncMetrics tracks sync performance metrics
type SyncMetrics struct {
	TotalOperations int64
	CompletedCount  int64
	FailedCount     int64
	CancelledCount  int64
	TotalBytes      int64
	TotalDuration   time.Duration
	AverageDuration time.Duration
	LastSyncTime    time.Time
	mu              sync.RWMutex
}

// RecordOperation records a completed sync operation
func (m *SyncMetrics) RecordOperation(success bool, bytesTransferred int64, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalOperations++
	m.TotalBytes += bytesTransferred

	if success {
		m.CompletedCount++
	} else {
		m.FailedCount++
	}

	m.TotalDuration += duration
	if m.TotalOperations > 0 {
		m.AverageDuration = m.TotalDuration / time.Duration(m.TotalOperations)
	}
	m.LastSyncTime = time.Now()
}

// RecordCancellation records a cancelled operation
func (m *SyncMetrics) RecordCancellation() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalOperations++
	m.CancelledCount++
}

// GetStats returns current metrics
func (m *SyncMetrics) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"total_operations": m.TotalOperations,
		"completed":        m.CompletedCount,
		"failed":           m.FailedCount,
		"cancelled":        m.CancelledCount,
		"total_bytes":      m.TotalBytes,
		"average_duration": m.AverageDuration.String(),
		"last_sync":        m.LastSyncTime,
	}
}

// SafeSyncContext provides a context-safe way to run sync operations
func SafeSyncContext(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	done := make(chan error, 1)

	go func() {
		done <- fn(ctx)
	}()

	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("sync operation timed out or was cancelled: %w", ctx.Err())
	}
}

// SyncTimeoutConfig provides timeout configuration for different sync phases
type SyncTimeoutConfig struct {
	ConnectionTimeout time.Duration
	ScanTimeout       time.Duration
	TransferTimeout   time.Duration
	VerifyTimeout     time.Duration
}

// DefaultSyncTimeoutConfig returns default timeout configuration
func DefaultSyncTimeoutConfig() SyncTimeoutConfig {
	return SyncTimeoutConfig{
		ConnectionTimeout: 30 * time.Second,
		ScanTimeout:       5 * time.Minute,
		TransferTimeout:   30 * time.Minute,
		VerifyTimeout:     5 * time.Minute,
	}
}
