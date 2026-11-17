package database

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// TransactionConfig holds configuration for transaction timeouts and behavior
type TransactionConfig struct {
	// TransactionTimeout is the maximum duration a transaction can run (default: 30s)
	TransactionTimeout time.Duration
	// QueryTimeout is the maximum duration for individual queries (default: 10s)
	QueryTimeout time.Duration
	// MaxRetries is the number of retries for deadlock scenarios (default: 3)
	MaxRetries int
	// RetryDelay is the delay between retries (default: 100ms)
	RetryDelay time.Duration
}

// DefaultTransactionConfig returns default transaction configuration
func DefaultTransactionConfig() TransactionConfig {
	return TransactionConfig{
		TransactionTimeout: 30 * time.Second,
		QueryTimeout:       10 * time.Second,
		MaxRetries:         3,
		RetryDelay:         100 * time.Millisecond,
	}
}

// TxDeadlockDetector tracks transaction ordering to detect potential deadlocks
type TxDeadlockDetector struct {
	mu         sync.RWMutex
	activeTxs  map[string]time.Time
	txOrder    []string
	maxTxOrder int
}

// NewTxDeadlockDetector creates a new deadlock detector
func NewTxDeadlockDetector() *TxDeadlockDetector {
	return &TxDeadlockDetector{
		activeTxs:  make(map[string]time.Time),
		txOrder:    make([]string, 0, 100),
		maxTxOrder: 1000,
	}
}

// RecordStart records the start of a transaction
func (d *TxDeadlockDetector) RecordStart(txID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.activeTxs[txID] = time.Now()
	d.txOrder = append(d.txOrder, txID)
	if len(d.txOrder) > d.maxTxOrder {
		d.txOrder = d.txOrder[len(d.txOrder)-d.maxTxOrder:]
	}
}

// RecordEnd records the end of a transaction
func (d *TxDeadlockDetector) RecordEnd(txID string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.activeTxs, txID)
}

// GetLongRunning returns transactions running longer than threshold
func (d *TxDeadlockDetector) GetLongRunning(threshold time.Duration) []string {
	d.mu.RLock()
	defer d.mu.RUnlock()

	now := time.Now()
	var longRunning []string
	for txID, start := range d.activeTxs {
		if now.Sub(start) > threshold {
			longRunning = append(longRunning, txID)
		}
	}
	return longRunning
}

// TransactionOptions provides options for transaction execution
type TransactionOptions struct {
	TxConfig *TransactionConfig
	ReadOnly bool
}

// Transaction represents a database transaction with timeout and deadlock detection
type Transaction struct {
	*sql.Tx
	ctx          context.Context
	cancel       context.CancelFunc
	config       TransactionConfig
	detector     *TxDeadlockDetector
	db           *DB
	txID         string
	startTime    time.Time
	isRolledBack bool
	mu           sync.RWMutex
}

// TxContext provides a context-aware transaction wrapper for the DB
type TxContext struct {
	db       *DB
	config   TransactionConfig
	detector *TxDeadlockDetector
}

// NewTxContext creates a new transaction context
func NewTxContext(db *DB, config TransactionConfig) *TxContext {
	return &TxContext{
		db:       db,
		config:   config,
		detector: NewTxDeadlockDetector(),
	}
}

// Begin starts a new transaction with timeout
func (tc *TxContext) Begin(ctx context.Context, opts *sql.TxOptions) (*Transaction, error) {
	txCtx, cancel := context.WithTimeout(ctx, tc.config.TransactionTimeout)

	sqlTx, err := tc.db.BeginTx(txCtx, opts)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}

	txID := generateTxID()
	tx := &Transaction{
		Tx:        sqlTx.Tx,
		ctx:       txCtx,
		cancel:    cancel,
		config:    tc.config,
		detector:  tc.detector,
		db:        tc.db,
		txID:      txID,
		startTime: time.Now(),
	}

	tc.detector.RecordStart(txID)

	return tx, nil
}

// BeginWithRetry starts a transaction with retry logic for deadlocks
func (tc *TxContext) BeginWithRetry(ctx context.Context, opts *sql.TxOptions) (*Transaction, error) {
	var lastErr error

	for i := 0; i <= tc.config.MaxRetries; i++ {
		if i > 0 {
			time.Sleep(tc.config.RetryDelay * time.Duration(i))
		}

		tx, err := tc.Begin(ctx, opts)
		if err == nil {
			return tx, nil
		}

		lastErr = err

		// Check if it's a deadlock error
		if !isDeadlockError(err) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("transaction failed after %d retries: %w", tc.config.MaxRetries, lastErr)
}

// Exec executes a query within the transaction with timeout
func (tx *Transaction) Exec(query string, args ...interface{}) (sql.Result, error) {
	ctx, cancel := context.WithTimeout(tx.ctx, tx.config.QueryTimeout)
	defer cancel()

	query = tx.db.rewriteQuery(query)
	return tx.Tx.ExecContext(ctx, query, args...)
}

// Query executes a query within the transaction.
//
// The returned *sql.Rows is consumed by the caller AFTER this method returns
// (via rows.Next/Scan).  We therefore must NOT use context.WithTimeout +
// defer cancel() here: the deferred cancel fires when Query returns, which
// cancels the query context before the caller iterates the result set.  That
// race lets database/sql's awaitDone goroutine close the rows with
// context.Canceled in between the caller's rows.Next() and rows.Scan(),
// producing spurious "context canceled" errors under scheduler load.  This is
// the same hazard db.Query (connection.go) documents and avoids.
//
// We pass tx.ctx directly: it already carries the transaction timeout
// (TransactionConfig.TransactionTimeout, applied in Begin) and is cancelled by
// Commit/Rollback — the correct lifetime, since the rows are only valid for the
// duration of the transaction.  Per-statement timeout safety is provided by the
// driver (SQLite _busy_timeout, PostgreSQL statement_timeout).
func (tx *Transaction) Query(query string, args ...interface{}) (*sql.Rows, error) {
	query = tx.db.rewriteQuery(query)
	return tx.Tx.QueryContext(tx.ctx, query, args...)
}

// QueryRow executes a query returning a single row within the transaction.
//
// *sql.Row buffers its result lazily — the read happens inside Scan(), which
// the caller invokes after this method returns.  Using context.WithTimeout +
// defer cancel() here races with Scan: the deferred cancel fires on return and
// can cancel the context before the driver finishes reading the row, producing
// spurious "context canceled" errors (reproduced under parallel load at
// transaction_coverage_test.go:63).  This mirrors db.QueryRow (connection.go),
// which documents and avoids the same hazard.
//
// We pass tx.ctx directly: it carries the transaction timeout and is cancelled
// only by Commit/Rollback, after the row has been scanned.  Per-statement
// timeout safety comes from the driver (SQLite _busy_timeout, PostgreSQL
// statement_timeout).
func (tx *Transaction) QueryRow(query string, args ...interface{}) *sql.Row {
	query = tx.db.rewriteQuery(query)
	return tx.Tx.QueryRowContext(tx.ctx, query, args...)
}

// Commit commits the transaction with cleanup.
// Note: RecordEnd is called outside tx.mu to avoid nested locking (tx.mu -> detector.mu).
// RecordEnd acquires its own mutex internally, so calling it under tx.mu would
// create a lock ordering dependency that risks deadlock under contention.
func (tx *Transaction) Commit() error {
	var shouldRecord bool
	var txID string
	err := func() error {
		tx.mu.Lock()
		defer tx.mu.Unlock()

		if tx.isRolledBack {
			return fmt.Errorf("transaction already rolled back")
		}

		txID = tx.txID
		shouldRecord = true
		return tx.Tx.Commit()
	}()

	tx.cancel()
	if shouldRecord {
		tx.detector.RecordEnd(txID) // Called outside tx.mu lock
	}

	return err
}

// Rollback rolls back the transaction with cleanup.
// Note: RecordEnd is called outside tx.mu to avoid nested locking (tx.mu -> detector.mu).
// RecordEnd acquires its own mutex internally, so calling it under tx.mu would
// create a lock ordering dependency that risks deadlock under contention.
func (tx *Transaction) Rollback() error {
	var shouldRecord bool
	var txID string
	err := func() error {
		tx.mu.Lock()
		defer tx.mu.Unlock()

		if tx.isRolledBack {
			return nil
		}

		tx.isRolledBack = true
		txID = tx.txID
		shouldRecord = true
		return tx.Tx.Rollback()
	}()

	tx.cancel()
	if shouldRecord {
		tx.detector.RecordEnd(txID) // Called outside tx.mu lock
	}

	return err
}

// IsTimeout returns true if the transaction timed out
func (tx *Transaction) IsTimeout() bool {
	select {
	case <-tx.ctx.Done():
		return tx.ctx.Err() == context.DeadlineExceeded
	default:
		return false
	}
}

// Duration returns how long the transaction has been running
func (tx *Transaction) Duration() time.Duration {
	return time.Since(tx.startTime)
}

// IsLongRunning returns true if transaction has been running longer than threshold
func (tx *Transaction) IsLongRunning(threshold time.Duration) bool {
	return tx.Duration() > threshold
}

// RunInTransaction executes a function within a transaction with automatic rollback on error
func (tc *TxContext) RunInTransaction(ctx context.Context, fn func(*Transaction) error) error {
	tx, err := tc.Begin(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	if err := fn(tx); err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit()
}

// RunInTransactionWithRetry executes a function with retry logic
func (tc *TxContext) RunInTransactionWithRetry(ctx context.Context, fn func(*Transaction) error) error {
	var lastErr error

	for i := 0; i <= tc.config.MaxRetries; i++ {
		if i > 0 {
			time.Sleep(tc.config.RetryDelay * time.Duration(i))
		}

		err := tc.RunInTransaction(ctx, fn)
		if err == nil {
			return nil
		}

		lastErr = err

		// Only retry on deadlock errors
		if !isDeadlockError(err) {
			return err
		}
	}

	return fmt.Errorf("transaction failed after %d retries: %w", tc.config.MaxRetries, lastErr)
}

// Helper functions

// txIDCounter guarantees txID uniqueness even when generateTxID is called
// multiple times within the same clock tick — time.Now() alone can repeat on
// fast loops / coarse-resolution timers (it did: duplicate
// "tx_<nano>_<nano>" collisions), so a strictly-increasing atomic counter is
// the unique component (the timestamp stays for ordering/debugging).
var txIDCounter atomic.Uint64

func generateTxID() string {
	return fmt.Sprintf("tx_%d_%d", time.Now().UnixNano(), txIDCounter.Add(1))
}

// isDeadlockError checks if an error is a deadlock error
func isDeadlockError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())
	deadlockKeywords := []string{
		"deadlock",
		"lock timeout",
		"database is locked",
		"busy",
		"serialization failure",
		"concurrent update",
	}

	for _, keyword := range deadlockKeywords {
		if strings.Contains(errStr, keyword) {
			return true
		}
	}

	return false
}

// SafeRollback performs a safe rollback that won't panic
// Logs rollback errors for debugging but doesn't return them since rollback is typically a cleanup operation
func SafeRollback(tx *sql.Tx) {
	if tx != nil {
		if err := tx.Rollback(); err != nil {
			// Log rollback error but don't fail - transaction may already be rolled back or committed
			// This is acceptable behavior for cleanup operations
			fmt.Fprintf(os.Stderr, "Warning: transaction rollback failed (may be already completed): %v\n", err)
		}
	}
}

// TxLockOrder represents lock ordering for avoiding deadlocks
type TxLockOrder struct {
	mu     sync.Mutex
	order  []string
	lookup map[string]int
}

// NewTxLockOrder creates a new lock order manager
func NewTxLockOrder(tables []string) *TxLockOrder {
	lookup := make(map[string]int, len(tables))
	for i, table := range tables {
		lookup[table] = i
	}

	return &TxLockOrder{
		order:  tables,
		lookup: lookup,
	}
}

// GetOrder returns the lock order for a table
func (lo *TxLockOrder) GetOrder(table string) int {
	lo.mu.Lock()
	defer lo.mu.Unlock()

	if order, ok := lo.lookup[table]; ok {
		return order
	}

	// Add new table to order
	order := len(lo.order)
	lo.order = append(lo.order, table)
	lo.lookup[table] = order
	return order
}

// SortTables sorts tables by lock order
func (lo *TxLockOrder) SortTables(tables []string) []string {
	sorted := make([]string, len(tables))
	copy(sorted, tables)

	// Simple bubble sort by order
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if lo.GetOrder(sorted[i]) > lo.GetOrder(sorted[j]) {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted
}

// SafeCommit commits a transaction and handles rollback on error
func SafeCommit(tx *sql.Tx) error {
	if tx == nil {
		return fmt.Errorf("cannot commit nil transaction")
	}
	if err := tx.Commit(); err != nil {
		// Rollback on commit failure - if rollback also fails, wrap both errors
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("commit failed: %v (rollback also failed: %v)", err, rbErr)
		}
		return fmt.Errorf("commit failed: %w", err)
	}
	return nil
}

// WithTransactionTimeout creates a context with transaction timeout
func WithTransactionTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}

// WithQueryTimeout creates a context with query timeout
func WithQueryTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, timeout)
}
