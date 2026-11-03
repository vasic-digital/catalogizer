package smb

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"catalogizer/internal/metrics"
	"digital.vasic.recovery/pkg/facade"
	"go.uber.org/zap"
)

// ConnectionState represents the state of an SMB connection
type ConnectionState int

const (
	StateConnected ConnectionState = iota
	StateDisconnected
	StateReconnecting
	StateOffline
)

func (s ConnectionState) String() string {
	switch s {
	case StateConnected:
		return "connected"
	case StateDisconnected:
		return "disconnected"
	case StateReconnecting:
		return "reconnecting"
	case StateOffline:
		return "offline"
	default:
		return "unknown"
	}
}

// HealthMetric returns the Prometheus health metric value for this state.
func (s ConnectionState) HealthMetric() float64 {
	switch s {
	case StateConnected:
		return metrics.SMBHealthy
	case StateReconnecting:
		return metrics.SMBDegraded
	case StateDisconnected, StateOffline:
		return metrics.SMBOffline
	default:
		return metrics.SMBOffline
	}
}

// SMBSource represents a configured SMB source with resilience capabilities
type SMBSource struct {
	ID                  string          `json:"id"`
	Name                string          `json:"name"`
	Path                string          `json:"path"`
	Username            string          `json:"username"`
	Password            string          `json:"password"`
	Domain              string          `json:"domain"`
	State               ConnectionState `json:"state"`
	LastConnected       time.Time       `json:"last_connected"`
	LastError           string          `json:"last_error,omitempty"`
	RetryAttempts       int             `json:"retry_attempts"`
	MaxRetryAttempts    int             `json:"max_retry_attempts"`
	RetryDelay          time.Duration   `json:"retry_delay"`
	ConnectionTimeout   time.Duration   `json:"connection_timeout"`
	HealthCheckInterval time.Duration   `json:"health_check_interval"`
	IsEnabled           bool            `json:"is_enabled"`
	mutex               sync.RWMutex
}

// ResilientSMBManager manages multiple SMB sources with automatic recovery.
// When ResilienceFacade is set (via SetResilienceFacade), connection attempts
// are routed through the digital.vasic.recovery circuit breaker for each source,
// providing centralized fault-tolerance metrics alongside the per-source retry logic.
type ResilientSMBManager struct {
	sources       map[string]*SMBSource
	logger        *zap.Logger
	offlineCache  *OfflineCache
	healthChecker *HealthChecker
	eventChannel  chan SMBEvent
	stopChannel   chan struct{}
	wg            sync.WaitGroup
	mutex         sync.RWMutex
	startTime     time.Time
	// resilience is the optional digital.vasic.recovery facade from the module registry.
	// When set, connection attempts are wrapped in the module's circuit breaker.
	resilience *facade.Resilience
}

// SMBEvent represents events from SMB operations
type SMBEvent struct {
	Type      EventType   `json:"type"`
	SourceID  string      `json:"source_id"`
	Path      string      `json:"path,omitempty"`
	Error     error       `json:"error,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Data      interface{} `json:"data,omitempty"`
}

type EventType int

const (
	EventConnected EventType = iota
	EventDisconnected
	EventReconnecting
	EventOffline
	EventFileChange
	EventError
	EventHealthCheck
)

func (e EventType) String() string {
	switch e {
	case EventConnected:
		return "connected"
	case EventDisconnected:
		return "disconnected"
	case EventReconnecting:
		return "reconnecting"
	case EventOffline:
		return "offline"
	case EventFileChange:
		return "file_change"
	case EventError:
		return "error"
	case EventHealthCheck:
		return "health_check"
	default:
		return "unknown"
	}
}

// OfflineCache stores metadata when SMB sources are unavailable
type OfflineCache struct {
	entries map[string]*CacheEntry
	maxSize int
	mutex   sync.RWMutex
	logger  *zap.Logger
}

type CacheEntry struct {
	Path        string                 `json:"path"`
	Metadata    map[string]interface{} `json:"metadata"`
	LastSeen    time.Time              `json:"last_seen"`
	IsAvailable bool                   `json:"is_available"`
	SourceID    string                 `json:"source_id"`
}

// HealthChecker periodically checks SMB source health.
// inFlight tracks checks currently running to prevent goroutine stacking
// when a check takes longer than the tick interval.
type HealthChecker struct {
	manager  *ResilientSMBManager
	interval time.Duration
	timeout  time.Duration
	logger   *zap.Logger
	ticker   *time.Ticker
	ctx      context.Context
	cancel   context.CancelFunc

	inFlightMu sync.Mutex
	inFlight   map[string]bool
}

// NewResilientSMBManager creates a new resilient SMB manager
func NewResilientSMBManager(logger *zap.Logger, cacheSize int) *ResilientSMBManager {
	manager := &ResilientSMBManager{
		sources:      make(map[string]*SMBSource),
		logger:       logger,
		offlineCache: NewOfflineCache(cacheSize, logger),
		eventChannel: make(chan SMBEvent, 1000),
		stopChannel:  make(chan struct{}),
		startTime:    time.Now(),
	}

	manager.healthChecker = NewHealthChecker(manager, 60*time.Second, 30*time.Second, logger)
	return manager
}

// NewOfflineCache creates a new offline cache
func NewOfflineCache(maxSize int, logger *zap.Logger) *OfflineCache {
	return &OfflineCache{
		entries: make(map[string]*CacheEntry),
		maxSize: maxSize,
		logger:  logger,
	}
}

// NewHealthChecker creates a new health checker
func NewHealthChecker(manager *ResilientSMBManager, interval, timeout time.Duration, logger *zap.Logger) *HealthChecker {
	ctx, cancel := context.WithCancel(context.Background())
	return &HealthChecker{
		manager:  manager,
		interval: interval,
		timeout:  timeout,
		logger:   logger,
		ctx:      ctx,
		cancel:   cancel,
		inFlight: make(map[string]bool),
	}
}

// SetResilienceFacade injects the digital.vasic.recovery facade from the module
// registry. When set, connectSource wraps TCP dials through the module's circuit
// breaker, unifying SMB fault-tolerance with the centralized resilience layer.
func (m *ResilientSMBManager) SetResilienceFacade(r *facade.Resilience) {
	m.resilience = r
}

// AddSource adds a new SMB source to the manager
func (m *ResilientSMBManager) AddSource(source *SMBSource) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if source.ID == "" {
		source.ID = fmt.Sprintf("smb_%d", time.Now().Unix())
	}

	// Set default values
	if source.MaxRetryAttempts == 0 {
		source.MaxRetryAttempts = 5
	}
	if source.RetryDelay == 0 {
		source.RetryDelay = 30 * time.Second
	}
	if source.ConnectionTimeout == 0 {
		source.ConnectionTimeout = 30 * time.Second
	}
	if source.HealthCheckInterval == 0 {
		source.HealthCheckInterval = 60 * time.Second
	}

	source.State = StateDisconnected
	metrics.SetSMBHealth(source.ID, source.State.HealthMetric())
	source.IsEnabled = true

	m.sources[source.ID] = source

	// Try initial connection
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.connectSource(source)
	}()

	m.logger.Info("SMB source added",
		zap.String("id", source.ID),
		zap.String("name", source.Name),
		zap.String("path", source.Path))

	return nil
}

// RemoveSource removes an SMB source from the manager
// Lock ordering: source.mutex -> manager.mutex (to avoid deadlock)
func (m *ResilientSMBManager) RemoveSource(sourceID string) error {
	// First acquire manager lock to get the source
	m.mutex.Lock()
	source, exists := m.sources[sourceID]
	if !exists {
		m.mutex.Unlock()
		return fmt.Errorf("source not found: %s", sourceID)
	}

	// Snapshot source ID before releasing manager lock
	sourceIDCopy := source.ID

	// Release manager lock before acquiring source lock (avoid nested locking)
	m.mutex.Unlock()

	// Now safely lock the source and disable it
	source.mutex.Lock()
	source.IsEnabled = false
	metrics.SetSMBHealth(sourceIDCopy, metrics.SMBOffline)
	source.mutex.Unlock()

	// Re-acquire manager lock to remove from map
	m.mutex.Lock()
	delete(m.sources, sourceID)
	m.mutex.Unlock()

	m.logger.Info("SMB source removed", zap.String("id", sourceID))
	return nil
}

// Start begins monitoring all SMB sources
func (m *ResilientSMBManager) Start() error {
	m.logger.Info("Starting resilient SMB manager")

	// Start health checker
	m.healthChecker.Start()

	// Start event processor
	m.wg.Add(1)
	go m.processEvents()

	// Start monitoring existing sources
	m.mutex.RLock()
	sources := make([]*SMBSource, 0, len(m.sources))
	for _, source := range m.sources {
		if source.IsEnabled {
			sources = append(sources, source)
		}
	}
	m.mutex.RUnlock()

	for _, source := range sources {
		m.wg.Add(1)
		go m.monitorSource(source)
	}

	return nil
}

// Stop shuts down the SMB manager
func (m *ResilientSMBManager) Stop() error {
	m.logger.Info("Stopping resilient SMB manager")

	// Stop health checker
	m.healthChecker.Stop()

	// Signal shutdown
	close(m.stopChannel)

	// Wait for goroutines to finish
	m.wg.Wait()

	// Drain event channel to prevent goroutine leaks from blocked senders
	for {
		select {
		case <-m.eventChannel:
		default:
			goto drained
		}
	}
drained:

	return nil
}

// connectSource attempts to connect to an SMB source
func (m *ResilientSMBManager) connectSource(source *SMBSource) error {
	source.mutex.Lock()
	defer source.mutex.Unlock()

	if !source.IsEnabled {
		return fmt.Errorf("source is disabled")
	}

	source.State = StateReconnecting
	metrics.SetSMBHealth(source.ID, source.State.HealthMetric())
	m.sendEvent(SMBEvent{
		Type:      EventReconnecting,
		SourceID:  source.ID,
		Timestamp: time.Now(),
	})

	// Real TCP reachability probe of the SMB port (not an SMB session/auth check)
	ctx, cancel := context.WithTimeout(context.Background(), source.ConnectionTimeout)
	defer cancel()

	err := m.attemptConnection(ctx, source)
	if err != nil {
		source.State = StateDisconnected
		metrics.SetSMBHealth(source.ID, source.State.HealthMetric())
		source.LastError = err.Error()
		source.RetryAttempts++

		m.logger.Error("Failed to connect to SMB source",
			zap.String("id", source.ID),
			zap.String("path", source.Path),
			zap.Error(err),
			zap.Int("retry_attempts", source.RetryAttempts))

		m.sendEvent(SMBEvent{
			Type:      EventDisconnected,
			SourceID:  source.ID,
			Error:     err,
			Timestamp: time.Now(),
		})

		// Schedule retry if not exceeded max attempts
		if source.RetryAttempts < source.MaxRetryAttempts {
			m.wg.Add(1)
			go func() {
				defer m.wg.Done()
				m.scheduleRetry(source)
			}()
		} else {
			source.State = StateOffline
			metrics.SetSMBHealth(source.ID, source.State.HealthMetric())
			m.sendEvent(SMBEvent{
				Type:      EventOffline,
				SourceID:  source.ID,
				Timestamp: time.Now(),
			})
		}

		return err
	}

	// Connection successful
	source.State = StateConnected
	metrics.SetSMBHealth(source.ID, source.State.HealthMetric())
	source.LastConnected = time.Now()
	source.LastError = ""
	source.RetryAttempts = 0

	m.logger.Info("Successfully connected to SMB source",
		zap.String("id", source.ID),
		zap.String("path", source.Path))

	m.sendEvent(SMBEvent{
		Type:      EventConnected,
		SourceID:  source.ID,
		Timestamp: time.Now(),
	})

	return nil
}

// extractHost parses the hostname from an SMB path (UNC \\host\share or //host/share format).
// Returns the host string, or an error if the path cannot be parsed.
func extractHost(path string) (string, error) {
	// Normalize: accept both \\host\share and //host/share
	normalized := strings.ReplaceAll(path, "\\", "/")
	normalized = strings.TrimPrefix(normalized, "//")
	parts := strings.SplitN(normalized, "/", 2)
	if len(parts) == 0 || parts[0] == "" {
		return "", fmt.Errorf("cannot parse host from SMB path: %s", path)
	}
	return parts[0], nil
}

// attemptConnection performs the actual SMB connection by dialing TCP port 445.
// When ResilienceFacade is set, the dial is wrapped in the module's circuit breaker
// (keyed by "smb-<sourceID>"), providing centralized fault-tolerance metrics.
func (m *ResilientSMBManager) attemptConnection(ctx context.Context, source *SMBSource) error {
	host, err := extractHost(source.Path)
	if err != nil {
		return fmt.Errorf("invalid SMB path %q: %w", source.Path, err)
	}

	// If host doesn't already include a port, default to SMB port 445
	address := host
	if _, _, splitErr := net.SplitHostPort(host); splitErr != nil {
		address = net.JoinHostPort(host, "445")
	}

	m.logger.Debug("Attempting TCP connection to SMB endpoint",
		zap.String("address", address),
		zap.String("source_id", source.ID))

	dial := func() error {
		dialer := &net.Dialer{}
		conn, dialErr := dialer.DialContext(ctx, "tcp", address)
		if dialErr != nil {
			return fmt.Errorf("TCP connection to %s failed: %w", address, dialErr)
		}
		conn.Close()
		return nil
	}

	// Route through digital.vasic.recovery circuit breaker when available
	if m.resilience != nil {
		return m.resilience.Execute("smb-"+source.ID, dial)
	}
	return dial()
}

// scheduleRetry schedules a retry attempt for a failed source
func (m *ResilientSMBManager) scheduleRetry(source *SMBSource) {
	// Read retry fields under source mutex to avoid racing with ForceReconnect
	source.mutex.RLock()
	retryAttempts := source.RetryAttempts
	retryDelay := source.RetryDelay
	source.mutex.RUnlock()

	delay := retryDelay * time.Duration(retryAttempts)
	if delay > 5*time.Minute {
		delay = 5 * time.Minute // Cap maximum delay
	}

	m.logger.Info("Scheduling retry for SMB source",
		zap.String("id", source.ID),
		zap.Duration("delay", delay),
		zap.Int("attempt", retryAttempts+1))

	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-timer.C:
		m.connectSource(source)
	case <-m.stopChannel:
		return
	}
}

// monitorSource continuously monitors an SMB source.
// The caller must call m.wg.Add(1) before launching this goroutine.
func (m *ResilientSMBManager) monitorSource(source *SMBSource) {
	defer m.wg.Done()

	ticker := time.NewTicker(10 * time.Second) // Monitor every 10 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if source.IsEnabled && source.State == StateConnected {
				m.checkSourceHealth(source)
			}
		case <-m.stopChannel:
			return
		}
	}
}

// checkSourceHealth performs a health check on an SMB source
func (m *ResilientSMBManager) checkSourceHealth(source *SMBSource) {
	// Real TCP reachability probe of the SMB port (not an SMB session/auth check)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := m.performHealthCheck(ctx, source)

	m.sendEvent(SMBEvent{
		Type:      EventHealthCheck,
		SourceID:  source.ID,
		Error:     err,
		Timestamp: time.Now(),
	})

	if err != nil {
		m.logger.Warn("SMB source health check failed",
			zap.String("id", source.ID),
			zap.Error(err))

		source.mutex.Lock()
		source.State = StateDisconnected
		metrics.SetSMBHealth(source.ID, source.State.HealthMetric())
		source.LastError = err.Error()
		source.mutex.Unlock()

		// Attempt reconnection
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.connectSource(source)
		}()
	}
}

// performHealthCheck verifies SMB endpoint reachability via TCP dial to port 445
func (m *ResilientSMBManager) performHealthCheck(ctx context.Context, source *SMBSource) error {
	host, err := extractHost(source.Path)
	if err != nil {
		return fmt.Errorf("health check failed, invalid SMB path %q: %w", source.Path, err)
	}

	address := host
	if _, _, splitErr := net.SplitHostPort(host); splitErr != nil {
		address = net.JoinHostPort(host, "445")
	}

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil {
		return fmt.Errorf("health check TCP dial to %s failed: %w", address, err)
	}
	conn.Close()

	return nil
}

// processEvents processes SMB events
func (m *ResilientSMBManager) processEvents() {
	defer m.wg.Done()

	for {
		select {
		case event := <-m.eventChannel:
			m.handleEvent(event)
		case <-m.stopChannel:
			return
		}
	}
}

// handleEvent handles individual SMB events
func (m *ResilientSMBManager) handleEvent(event SMBEvent) {
	m.logger.Debug("Processing SMB event",
		zap.String("type", event.Type.String()),
		zap.String("source_id", event.SourceID))

	switch event.Type {
	case EventConnected:
		m.onSourceConnected(event.SourceID)
	case EventDisconnected:
		m.onSourceDisconnected(event.SourceID, event.Error)
	case EventOffline:
		m.onSourceOffline(event.SourceID)
	case EventFileChange:
		m.onFileChange(event.SourceID, event.Path)
	case EventError:
		m.onError(event.SourceID, event.Error)
	}
}

// sendEvent sends an event to the event channel with backpressure.
// If the channel is full, the oldest event is drained to make room for the new one.
func (m *ResilientSMBManager) sendEvent(event SMBEvent) {
	select {
	case m.eventChannel <- event:
		return
	default:
	}

	// Channel full: drain the oldest event to make room
	select {
	case dropped := <-m.eventChannel:
		m.logger.Warn("Event channel full, dropped oldest event to make room",
			zap.String("dropped_type", dropped.Type.String()),
			zap.String("dropped_source", dropped.SourceID),
			zap.String("new_type", event.Type.String()),
			zap.String("new_source", event.SourceID))
	default:
	}

	// Try sending again (may still fail if another goroutine filled it, which is acceptable)
	select {
	case m.eventChannel <- event:
	default:
		m.logger.Warn("Event channel still full after drain attempt, dropping new event",
			zap.String("type", event.Type.String()),
			zap.String("source_id", event.SourceID))
	}
}

// Event handlers
func (m *ResilientSMBManager) onSourceConnected(sourceID string) {
	// Source reconnected, process any cached changes
	m.offlineCache.ProcessCachedChanges(sourceID)
}

func (m *ResilientSMBManager) onSourceDisconnected(sourceID string, err error) {
	// Source disconnected, enable offline mode
	m.offlineCache.EnableOfflineMode(sourceID)
}

func (m *ResilientSMBManager) onSourceOffline(sourceID string) {
	// Source is offline, full offline mode
	m.logger.Warn("SMB source is now offline", zap.String("source_id", sourceID))
}

func (m *ResilientSMBManager) onFileChange(sourceID, path string) {
	// Handle file change, with offline caching if needed
	if m.isSourceConnected(sourceID) {
		// Process change immediately
		m.processFileChange(sourceID, path)
	} else {
		// Cache change for later processing
		m.offlineCache.CacheChange(sourceID, path)
	}
}

func (m *ResilientSMBManager) onError(sourceID string, err error) {
	m.logger.Error("SMB source error",
		zap.String("source_id", sourceID),
		zap.Error(err))
}

// Utility methods
// Lock ordering: manager.mutex -> source.mutex (RLock both)
func (m *ResilientSMBManager) isSourceConnected(sourceID string) bool {
	// First acquire manager read lock
	m.mutex.RLock()
	source, exists := m.sources[sourceID]
	if !exists {
		m.mutex.RUnlock()
		return false
	}
	m.mutex.RUnlock()

	// Now safely check source state
	source.mutex.RLock()
	isConnected := source.State == StateConnected
	source.mutex.RUnlock()

	return isConnected
}

func (m *ResilientSMBManager) processFileChange(sourceID, path string) {
	// Process file change logic here
	m.logger.Info("Processing file change",
		zap.String("source_id", sourceID),
		zap.String("path", path))
}

// GetSourceStatus returns the status of all sources
// Lock ordering: manager.mutex (RLock) -> iterate sources -> source.mutex (RLock each)
func (m *ResilientSMBManager) GetSourceStatus() map[string]interface{} {
	// First, snapshot all sources under manager lock
	m.mutex.RLock()
	sourcesSnapshot := make(map[string]*SMBSource)
	for id, source := range m.sources {
		sourcesSnapshot[id] = source
	}
	m.mutex.RUnlock()

	// Now safely read each source's status
	status := make(map[string]interface{})
	for id, source := range sourcesSnapshot {
		source.mutex.RLock()
		status[id] = map[string]interface{}{
			"name":           source.Name,
			"path":           source.Path,
			"state":          source.State.String(),
			"last_connected": source.LastConnected,
			"last_error":     source.LastError,
			"retry_attempts": source.RetryAttempts,
			"is_enabled":     source.IsEnabled,
		}
		source.mutex.RUnlock()
	}

	return status
}

// GetStartTime returns when the manager was started
func (m *ResilientSMBManager) GetStartTime() time.Time {
	return m.startTime
}

// ForceReconnect forces a reconnection attempt for a specific source
func (m *ResilientSMBManager) ForceReconnect(sourceID string) error {
	m.mutex.RLock()
	source, exists := m.sources[sourceID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("source not found: %s", sourceID)
	}

	if !source.IsEnabled {
		return fmt.Errorf("source is disabled: %s", sourceID)
	}

	// Reset retry attempts to allow reconnection
	source.mutex.Lock()
	source.RetryAttempts = 0
	source.mutex.Unlock()

	// Trigger reconnection
	m.wg.Add(1)
	go func() {
		defer m.wg.Done()
		m.connectSource(source)
	}()

	m.logger.Info("Force reconnect initiated", zap.String("source_id", sourceID))
	return nil
}

// UpdateSource updates an existing SMB source
func (m *ResilientSMBManager) UpdateSource(sourceID string, updates interface{}) error {
	m.mutex.RLock()
	_, exists := m.sources[sourceID]
	m.mutex.RUnlock()

	if !exists {
		return fmt.Errorf("source not found: %s", sourceID)
	}

	// This would handle the actual update logic
	// For now, just log the update attempt
	m.logger.Info("SMB source update requested",
		zap.String("source_id", sourceID))

	return nil
}

// OfflineCache methods
func (c *OfflineCache) CacheChange(sourceID, path string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	// Evict oldest entries if cache is full
	if len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	key := fmt.Sprintf("%s:%s", sourceID, path)
	c.entries[key] = &CacheEntry{
		Path:        path,
		LastSeen:    time.Now(),
		IsAvailable: false,
		SourceID:    sourceID,
		Metadata:    make(map[string]interface{}),
	}

	c.logger.Debug("Cached file change",
		zap.String("source_id", sourceID),
		zap.String("path", path))
}

func (c *OfflineCache) ProcessCachedChanges(sourceID string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	processed := 0
	for _, entry := range c.entries {
		if entry.SourceID == sourceID && !entry.IsAvailable {
			// Process cached change
			c.logger.Info("Processing cached change",
				zap.String("source_id", sourceID),
				zap.String("path", entry.Path))

			entry.IsAvailable = true
			processed++
		}
	}

	c.logger.Info("Processed cached changes",
		zap.String("source_id", sourceID),
		zap.Int("count", processed))
}

func (c *OfflineCache) EnableOfflineMode(sourceID string) {
	c.logger.Info("Enabling offline mode for source", zap.String("source_id", sourceID))
}

func (c *OfflineCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range c.entries {
		if oldestKey == "" || entry.LastSeen.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.LastSeen
		}
	}

	if oldestKey != "" {
		delete(c.entries, oldestKey)
		c.logger.Debug("Evicted cache entry", zap.String("key", oldestKey))
	}
}

// HealthChecker methods
func (h *HealthChecker) Start() {
	h.ticker = time.NewTicker(h.interval)
	go h.run()
}

func (h *HealthChecker) Stop() {
	if h.ticker != nil {
		h.ticker.Stop()
	}
	h.cancel()
}

func (h *HealthChecker) run() {
	for {
		select {
		case <-h.ticker.C:
			h.performHealthChecks()
		case <-h.ctx.Done():
			return
		}
	}
}

func (h *HealthChecker) performHealthChecks() {
	h.manager.mutex.RLock()
	sources := make([]*SMBSource, 0, len(h.manager.sources))
	for _, source := range h.manager.sources {
		if source.IsEnabled {
			sources = append(sources, source)
		}
	}
	h.manager.mutex.RUnlock()

	// Skip any source whose previous health check is still running. This
	// prevents goroutine stacking when a check takes longer than the tick
	// interval (which would otherwise spawn unbounded fan-out over time).
	// Health-check goroutines are tracked on the manager's WaitGroup so
	// graceful shutdown can wait for them.
	for _, source := range sources {
		srcID := source.ID
		h.inFlightMu.Lock()
		if h.inFlight[srcID] {
			h.inFlightMu.Unlock()
			h.logger.Debug("Skipping health check: previous check still running",
				zap.String("source_id", srcID))
			continue
		}
		h.inFlight[srcID] = true
		h.inFlightMu.Unlock()

		h.manager.wg.Add(1)
		go func(s *SMBSource) {
			defer h.manager.wg.Done()
			defer func() {
				h.inFlightMu.Lock()
				delete(h.inFlight, s.ID)
				h.inFlightMu.Unlock()
			}()
			h.manager.checkSourceHealth(s)
		}(source)
	}
}
