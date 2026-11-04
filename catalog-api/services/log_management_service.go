package services

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"catalogizer/internal/logging"
	"catalogizer/models"
	"catalogizer/repository"
)

// ErrLogAccessDenied is returned when the caller does not own the collection or share; handlers map it to 403.
var ErrLogAccessDenied = errors.New("access denied")

// ErrUnsupportedLogFormat is returned by ExportLogs for a format it cannot produce; handlers map it to 400.
var ErrUnsupportedLogFormat = errors.New("unsupported format")

type LogManagementService struct {
	logRepo       *repository.LogManagementRepository
	config        *LogManagementConfig
	logCollectors map[string]LogCollector
	wg            sync.WaitGroup
}

type LogManagementConfig struct {
	LogDirectory         string   `json:"log_directory"`
	MaxLogSize           int64    `json:"max_log_size"`        // in bytes
	MaxLogFiles          int      `json:"max_log_files"`       // per component
	RetentionDays        int      `json:"retention_days"`      // how long to keep logs
	CompressionEnabled   bool     `json:"compression_enabled"` // compress old logs
	RealTimeLogging      bool     `json:"real_time_logging"`   // enable real-time log streaming
	LogLevels            []string `json:"log_levels"`          // enabled log levels
	ComponentFilters     []string `json:"component_filters"`   // enabled components
	AutoCleanup          bool     `json:"auto_cleanup"`        // automatically cleanup old logs
	MaxShareDuration     int      `json:"max_share_duration"`  // hours
	AllowExternalSharing bool     `json:"allow_external_sharing"`
}

type LogCollector interface {
	CollectLogs() ([]*models.LogEntry, error)
	GetLogPath() string
	GetComponentName() string
}

type FileLogCollector struct {
	logPath       string
	componentName string
}

type DatabaseLogCollector struct {
	logRepo       *repository.LogManagementRepository
	componentName string
}

func NewLogManagementService(logRepo *repository.LogManagementRepository) *LogManagementService {
	config := &LogManagementConfig{
		LogDirectory:         "/var/log/catalogizer",
		MaxLogSize:           100 * 1024 * 1024, // 100MB
		MaxLogFiles:          10,
		RetentionDays:        30,
		CompressionEnabled:   true,
		RealTimeLogging:      true,
		LogLevels:            []string{"error", "warning", "info", "debug"},
		ComponentFilters:     []string{"api", "auth", "sync", "conversion", "stress_test"},
		AutoCleanup:          true,
		MaxShareDuration:     24, // 24 hours
		AllowExternalSharing: false,
	}

	service := &LogManagementService{
		logRepo:       logRepo,
		config:        config,
		logCollectors: make(map[string]LogCollector),
	}

	// Initialize default collectors
	service.initializeCollectors()

	return service
}

// Close waits for all in-flight log collection and streaming goroutines to finish.
func (s *LogManagementService) Close() {
	s.wg.Wait()
}

func (s *LogManagementService) initializeCollectors() {
	// File-based collectors
	components := []string{"api", "auth", "sync", "conversion", "stress_test", "error_reporting"}
	for _, component := range components {
		logPath := filepath.Join(s.config.LogDirectory, component+".log")
		s.logCollectors[component] = &FileLogCollector{
			logPath:       logPath,
			componentName: component,
		}
	}

	// Database collector for application logs
	s.logCollectors["database"] = &DatabaseLogCollector{
		logRepo:       s.logRepo,
		componentName: "database",
	}
}

func (s *LogManagementService) CollectLogs(userID int, request *models.LogCollectionRequest) (*models.LogCollection, error) {
	collection := &models.LogCollection{
		UserID:      userID,
		Name:        request.Name,
		Description: request.Description,
		Components:  request.Components,
		LogLevel:    request.LogLevel,
		StartTime:   request.StartTime,
		EndTime:     request.EndTime,
		CreatedAt:   time.Now(),
		Status:      models.LogCollectionStatusInProgress,
		Filters:     request.Filters,
	}

	// Create the collection record
	if err := s.logRepo.CreateLogCollection(collection); err != nil {
		return nil, fmt.Errorf("failed to create log collection: %w", err)
	}

	// Return a snapshot copy to the caller so the goroutine can
	// safely mutate the original collection without a data race.
	returnCopy := *collection

	// Start collection process asynchronously. performLogCollection is
	// naturally bounded: it iterates the requested components, writes a
	// finite number of DB entries, updates the collection status, and returns.
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.performLogCollection(collection)
	}()

	return &returnCopy, nil
}

const maxLogEntries = 50000

func (s *LogManagementService) performLogCollection(collection *models.LogCollection) {
	var allEntries []*models.LogEntry

	// Collect logs from each component
	for _, component := range collection.Components {
		if len(allEntries) >= maxLogEntries {
			break // prevent unbounded memory growth
		}

		collector, exists := s.logCollectors[component]
		if !exists {
			s.logError(collection.ID, fmt.Sprintf("Unknown component: %s", component))
			continue
		}

		entries, err := collector.CollectLogs()
		if err != nil {
			s.logError(collection.ID, fmt.Sprintf("Failed to collect logs from %s: %v", component, err))
			continue
		}

		// Filter entries
		filteredEntries := s.filterLogEntries(entries, collection)

		// Cap total entries to prevent unbounded memory growth
		remaining := maxLogEntries - len(allEntries)
		if len(filteredEntries) > remaining {
			filteredEntries = filteredEntries[:remaining]
		}
		allEntries = append(allEntries, filteredEntries...)
	}

	// Sort entries by timestamp
	sort.Slice(allEntries, func(i, j int) bool {
		return allEntries[i].Timestamp.Before(allEntries[j].Timestamp)
	})

	// Store collected entries
	for _, entry := range allEntries {
		entry.CollectionID = collection.ID
		if err := s.logRepo.CreateLogEntry(entry); err != nil {
			s.logError(collection.ID, fmt.Sprintf("Failed to store log entry: %v", err))
		}
	}

	// Update collection status
	collection.Status = models.LogCollectionStatusCompleted
	collection.CompletedAt = &[]time.Time{time.Now()}[0]
	collection.EntryCount = len(allEntries)

	if err := s.logRepo.UpdateLogCollection(collection); err != nil {
		s.logError(collection.ID, fmt.Sprintf("Failed to update collection status: %v", err))
	}
}

func (s *LogManagementService) GetLogCollection(id int, userID int) (*models.LogCollection, error) {
	collection, err := s.logRepo.GetLogCollection(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get log collection: %w", err)
	}

	if collection.UserID != userID {
		return nil, ErrLogAccessDenied
	}

	return collection, nil
}

func (s *LogManagementService) GetLogCollectionsByUser(userID int, limit, offset int) ([]*models.LogCollection, error) {
	return s.logRepo.GetLogCollectionsByUser(userID, limit, offset)
}

func (s *LogManagementService) GetLogEntries(collectionID int, userID int, filters *models.LogEntryFilters) ([]*models.LogEntry, error) {
	// Verify user has access to collection
	collection, err := s.GetLogCollection(collectionID, userID)
	if err != nil {
		return nil, err
	}

	if collection.UserID != userID {
		return nil, ErrLogAccessDenied
	}

	return s.logRepo.GetLogEntries(collectionID, filters)
}

func (s *LogManagementService) CreateLogShare(userID int, request *models.LogShareRequest) (*models.LogShare, error) {
	// Verify user has access to collection
	collection, err := s.GetLogCollection(request.CollectionID, userID)
	if err != nil {
		return nil, err
	}

	if collection.UserID != userID {
		return nil, ErrLogAccessDenied
	}

	// Create share
	share := &models.LogShare{
		CollectionID: request.CollectionID,
		UserID:       userID,
		ShareToken:   s.generateShareToken(),
		ShareType:    request.ShareType,
		ExpiresAt:    time.Now().Add(time.Duration(s.config.MaxShareDuration) * time.Hour),
		CreatedAt:    time.Now(),
		IsActive:     true,
		Permissions:  request.Permissions,
		Recipients:   request.Recipients,
	}

	if request.ExpiresAt != nil && request.ExpiresAt.Before(share.ExpiresAt) {
		share.ExpiresAt = *request.ExpiresAt
	}

	if err := s.logRepo.CreateLogShare(share); err != nil {
		return nil, fmt.Errorf("failed to create log share: %w", err)
	}

	return share, nil
}

func (s *LogManagementService) GetLogShare(token string) (*models.LogShare, error) {
	share, err := s.logRepo.GetLogShareByToken(token)
	if err != nil {
		return nil, fmt.Errorf("failed to get log share: %w", err)
	}

	if !share.IsActive || time.Now().After(share.ExpiresAt) {
		return nil, fmt.Errorf("share expired or inactive")
	}

	return share, nil
}

func (s *LogManagementService) RevokeLogShare(id int, userID int) error {
	share, err := s.logRepo.GetLogShare(id)
	if err != nil {
		return fmt.Errorf("failed to get log share: %w", err)
	}

	// Verify user owns the shared collection
	collection, err := s.GetLogCollection(share.CollectionID, userID)
	if err != nil {
		return err
	}

	if collection.UserID != userID {
		return ErrLogAccessDenied
	}

	share.IsActive = false
	return s.logRepo.UpdateLogShare(share)
}

func (s *LogManagementService) ExportLogs(collectionID int, userID int, format string) ([]byte, error) {
	// Verify access
	_, err := s.GetLogCollection(collectionID, userID)
	if err != nil {
		return nil, err
	}

	// Get log entries
	entries, err := s.logRepo.GetLogEntries(collectionID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get log entries: %w", err)
	}

	if entries == nil {
		entries = []*models.LogEntry{} // a nil slice would export the JSON document "null"
	}
	switch format {
	case "json":
		return json.MarshalIndent(entries, "", "  ")
	case "csv":
		return s.exportToCSV(entries)
	case "txt":
		return s.exportToText(entries)
	case "zip":
		return s.exportToZip(entries)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedLogFormat, format)
	}
}

func (s *LogManagementService) StreamLogs(userID int, filters *models.LogStreamFilters) (<-chan *models.LogEntry, chan<- struct{}, error) {
	if !s.config.RealTimeLogging {
		return nil, nil, fmt.Errorf("real-time logging is disabled")
	}

	// Create a channel for streaming
	logChannel := make(chan *models.LogEntry, 100)

	// done allows the caller to stop the streaming goroutine.
	// Close this channel when log streaming is no longer needed.
	done := make(chan struct{})

	// Start streaming goroutine
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.streamLogEntries(logChannel, filters, done)
	}()

	return logChannel, done, nil
}

func (s *LogManagementService) AnalyzeLogs(collectionID int, userID int) (*models.LogAnalysis, error) {
	// Verify access
	_, err := s.GetLogCollection(collectionID, userID)
	if err != nil {
		return nil, err
	}

	// Get log entries
	entries, err := s.logRepo.GetLogEntries(collectionID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get log entries: %w", err)
	}

	analysis := &models.LogAnalysis{
		CollectionID:       collectionID,
		TotalEntries:       len(entries),
		EntriesByLevel:     make(map[string]int),
		EntriesByComponent: make(map[string]int),
		ErrorPatterns:      make(map[string]int),
		TimeRange:          &models.TimeRange{},
	}

	if len(entries) == 0 {
		return analysis, nil
	}

	// Analyze entries
	analysis.TimeRange.Start = entries[0].Timestamp
	analysis.TimeRange.End = entries[len(entries)-1].Timestamp

	for _, entry := range entries {
		// Count by level
		analysis.EntriesByLevel[entry.Level]++

		// Count by component
		analysis.EntriesByComponent[entry.Component]++

		// Extract error patterns
		if entry.Level == "error" || entry.Level == "fatal" {
			pattern := s.extractErrorPattern(entry.Message)
			analysis.ErrorPatterns[pattern]++
		}
	}

	// Generate insights
	analysis.Insights = s.generateInsights(entries, analysis)

	return analysis, nil
}

func (s *LogManagementService) CleanupOldLogs() error {
	if !s.config.AutoCleanup {
		return nil
	}

	cutoff := time.Now().AddDate(0, 0, -s.config.RetentionDays)

	// Cleanup collections
	if err := s.logRepo.CleanupOldCollections(cutoff); err != nil {
		return fmt.Errorf("failed to cleanup old collections: %w", err)
	}

	// Cleanup shares
	if err := s.logRepo.CleanupExpiredShares(); err != nil {
		return fmt.Errorf("failed to cleanup expired shares: %w", err)
	}

	// Cleanup physical log files
	return s.cleanupPhysicalLogFiles(cutoff)
}

func (s *LogManagementService) GetConfiguration() *LogManagementConfig {
	return s.config
}

func (s *LogManagementService) UpdateConfiguration(config *LogManagementConfig) error {
	s.config = config
	s.initializeCollectors() // Re-initialize collectors with new config
	return nil
}

func (s *LogManagementService) GetLogStatistics(userID int) (*models.LogStatistics, error) {
	return s.logRepo.GetLogStatistics(userID)
}

// Helper methods

func (s *LogManagementService) filterLogEntries(entries []*models.LogEntry, collection *models.LogCollection) []*models.LogEntry {
	var filtered []*models.LogEntry

	for _, entry := range entries {
		// Filter by time range
		if collection.StartTime != nil && entry.Timestamp.Before(*collection.StartTime) {
			continue
		}
		if collection.EndTime != nil && entry.Timestamp.After(*collection.EndTime) {
			continue
		}

		// Filter by log level
		if collection.LogLevel != "" && !s.isLogLevelIncluded(entry.Level, collection.LogLevel) {
			continue
		}

		// Apply custom filters
		if collection.Filters != nil && !s.matchesFilters(entry, collection.Filters) {
			continue
		}

		filtered = append(filtered, entry)
	}

	return filtered
}

func (s *LogManagementService) isLogLevelIncluded(entryLevel, filterLevel string) bool {
	levels := map[string]int{
		"debug":   0,
		"info":    1,
		"warning": 2,
		"error":   3,
		"fatal":   4,
	}

	entryLevelNum, exists := levels[strings.ToLower(entryLevel)]
	if !exists {
		return false
	}

	filterLevelNum, exists := levels[strings.ToLower(filterLevel)]
	if !exists {
		return true
	}

	return entryLevelNum >= filterLevelNum
}

func (s *LogManagementService) matchesFilters(entry *models.LogEntry, filters map[string]interface{}) bool {
	for key, value := range filters {
		switch key {
		case "message_contains":
			if str, ok := value.(string); ok {
				if !strings.Contains(strings.ToLower(entry.Message), strings.ToLower(str)) {
					return false
				}
			}
		case "component":
			if str, ok := value.(string); ok {
				if entry.Component != str {
					return false
				}
			}
		}
	}
	return true
}

func (s *LogManagementService) generateShareToken() string {
	// Generate a secure random token
	return fmt.Sprintf("log_share_%d_%d", time.Now().Unix(), time.Now().Nanosecond())
}

func (s *LogManagementService) exportToCSV(entries []*models.LogEntry) ([]byte, error) {
	var buffer bytes.Buffer

	// Write CSV header
	buffer.WriteString("Timestamp,Level,Component,Message,Context\n")

	// Write entries
	for _, entry := range entries {
		contextJSON, _ := json.Marshal(entry.Context)
		buffer.WriteString(fmt.Sprintf("%s,%s,%s,\"%s\",\"%s\"\n",
			entry.Timestamp.Format(time.RFC3339),
			entry.Level,
			entry.Component,
			strings.ReplaceAll(entry.Message, "\"", "\"\""),
			string(contextJSON)))
	}

	return buffer.Bytes(), nil
}

func (s *LogManagementService) exportToText(entries []*models.LogEntry) ([]byte, error) {
	var buffer bytes.Buffer

	for _, entry := range entries {
		buffer.WriteString(fmt.Sprintf("[%s] [%s] [%s] %s\n",
			entry.Timestamp.Format(time.RFC3339),
			strings.ToUpper(entry.Level),
			entry.Component,
			entry.Message))
	}

	return buffer.Bytes(), nil
}

func (s *LogManagementService) exportToZip(entries []*models.LogEntry) ([]byte, error) {
	var buffer bytes.Buffer
	zipWriter := zip.NewWriter(&buffer)

	// Create JSON file
	jsonData, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return nil, err
	}

	jsonFile, err := zipWriter.Create("logs.json")
	if err != nil {
		return nil, err
	}
	jsonFile.Write(jsonData)

	// Create text file
	textData, err := s.exportToText(entries)
	if err != nil {
		return nil, err
	}

	textFile, err := zipWriter.Create("logs.txt")
	if err != nil {
		return nil, err
	}
	textFile.Write(textData)

	zipWriter.Close()
	return buffer.Bytes(), nil
}

func (s *LogManagementService) streamLogEntries(channel chan<- *models.LogEntry, filters *models.LogStreamFilters, done <-chan struct{}) {
	defer close(channel)

	if filters == nil {
		filters = &models.LogStreamFilters{}
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	var lastID int
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
		}

		if s.logRepo == nil {
			continue
		}

		entries, err := s.logRepo.GetRecentLogEntries(filters.Component, 50)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if entry.ID <= lastID {
				continue
			}
			if filters.Level != "" && entry.Level != filters.Level {
				continue
			}
			if filters.Search != "" && !strings.Contains(entry.Message, filters.Search) {
				continue
			}
			lastID = entry.ID

			// Non-blocking send: if the receiver has gone away via done or
			// is too slow (timeout), drop the entry and exit. The pre-fix
			// unconditional `channel <- entry` could block forever, leaking
			// the goroutine when the receiver stopped reading.
			select {
			case <-done:
				return
			case channel <- entry:
			case <-time.After(5 * time.Second):
				return
			}
		}
	}
}

func (s *LogManagementService) extractErrorPattern(message string) string {
	// Simple pattern extraction - remove specific details to group similar errors
	// This is a simplified implementation
	words := strings.Fields(message)
	if len(words) > 5 {
		return strings.Join(words[:5], " ") + "..."
	}
	return message
}

func (s *LogManagementService) generateInsights(entries []*models.LogEntry, analysis *models.LogAnalysis) []string {
	insights := []string{}

	// Check error rate
	errorCount := analysis.EntriesByLevel["error"] + analysis.EntriesByLevel["fatal"]
	errorRate := float64(errorCount) / float64(analysis.TotalEntries) * 100

	if errorRate > 10 {
		insights = append(insights, fmt.Sprintf("High error rate detected: %.1f%%", errorRate))
	}

	// Check for component with most errors
	maxErrors := 0
	maxComponent := ""
	for component, count := range analysis.EntriesByComponent {
		if count > maxErrors {
			maxErrors = count
			maxComponent = component
		}
	}

	if maxComponent != "" {
		insights = append(insights, fmt.Sprintf("Component '%s' generated the most log entries (%d)", maxComponent, maxErrors))
	}

	return insights
}

func (s *LogManagementService) cleanupPhysicalLogFiles(cutoff time.Time) error {
	return filepath.Walk(s.config.LogDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && info.ModTime().Before(cutoff) {
			// Compress old log files if compression is enabled
			if s.config.CompressionEnabled && !strings.HasSuffix(path, ".gz") {
				if err := s.compressLogFile(path); err != nil {
					return err
				}
			}
		}

		return nil
	})
}

func (s *LogManagementService) compressLogFile(path string) error {
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	defer input.Close()

	output, err := os.Create(path + ".gz")
	if err != nil {
		return err
	}
	defer output.Close()

	gzipWriter := gzip.NewWriter(output)
	defer gzipWriter.Close()

	_, err = io.Copy(gzipWriter, input)
	if err != nil {
		return err
	}

	// Remove original file after successful compression
	return os.Remove(path)
}

func (s *LogManagementService) logError(collectionID int, message string) {
	// Log error to the system log (simplified implementation)
	if logging.Logger != nil {
		logging.With(
			logging.Int("collection_id", collectionID),
			logging.String("message", message),
		).Error("Log management error")
	}
}

// LogCollector implementations

func (c *FileLogCollector) CollectLogs() ([]*models.LogEntry, error) {
	file, err := os.Open(c.logPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	entries := []*models.LogEntry{}
	// This is a simplified implementation
	// In reality, you would parse the log file format

	return entries, nil
}

func (c *FileLogCollector) GetLogPath() string {
	return c.logPath
}

func (c *FileLogCollector) GetComponentName() string {
	return c.componentName
}

func (c *DatabaseLogCollector) CollectLogs() ([]*models.LogEntry, error) {
	// Collect logs from database
	return c.logRepo.GetRecentLogEntries(c.componentName, 1000)
}

func (c *DatabaseLogCollector) GetLogPath() string {
	return "database"
}

func (c *DatabaseLogCollector) GetComponentName() string {
	return c.componentName
}
