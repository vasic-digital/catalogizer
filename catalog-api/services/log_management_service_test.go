package services

import (
	"strings"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
)

func TestLogManagementService_IsLogLevelIncluded(t *testing.T) {
	service := NewLogManagementService(nil)

	tests := []struct {
		name     string
		level    string
		minLevel string
		expected bool
	}{
		{
			name:     "debug includes debug",
			level:    "debug",
			minLevel: "debug",
			expected: true,
		},
		{
			name:     "info includes info",
			level:    "info",
			minLevel: "info",
			expected: true,
		},
		{
			name:     "warning includes warning",
			level:    "warning",
			minLevel: "warning",
			expected: true,
		},
		{
			name:     "error includes error",
			level:    "error",
			minLevel: "error",
			expected: true,
		},
		{
			name:     "debug excluded by info minimum",
			level:    "debug",
			minLevel: "info",
			expected: false,
		},
		{
			name:     "info excluded by warning minimum",
			level:    "info",
			minLevel: "warning",
			expected: false,
		},
		{
			name:     "error included by debug minimum",
			level:    "error",
			minLevel: "debug",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isLogLevelIncluded(tt.level, tt.minLevel)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogManagementService_ExtractErrorPattern(t *testing.T) {
	service := NewLogManagementService(nil)

	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "simple error message",
			message: "connection refused: dial tcp 127.0.0.1:5432",
		},
		{
			name:    "short message",
			message: "timeout",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.extractErrorPattern(tt.message)
			assert.NotEmpty(t, result)
		})
	}
}

func TestLogManagementService_GenerateShareToken(t *testing.T) {
	service := NewLogManagementService(nil)

	token1, err := service.generateShareToken()
	assert.NoError(t, err)
	token2, err := service.generateShareToken()
	assert.NoError(t, err)

	assert.NotEmpty(t, token1)
	assert.NotEmpty(t, token2)
	assert.NotEqual(t, token1, token2)
}

func TestLogManagementService_MatchesFilters(t *testing.T) {
	service := NewLogManagementService(nil)

	tests := []struct {
		name     string
		entry    *models.LogEntry
		filters  map[string]interface{}
		expected bool
	}{
		{
			name: "matching message_contains filter",
			entry: &models.LogEntry{
				Message:   "error connecting to database",
				Level:     "error",
				Component: "api",
				Timestamp: time.Now(),
			},
			filters:  map[string]interface{}{"message_contains": "database"},
			expected: true,
		},
		{
			name: "no matching message_contains filter",
			entry: &models.LogEntry{
				Message:   "server started successfully",
				Level:     "info",
				Component: "api",
				Timestamp: time.Now(),
			},
			filters:  map[string]interface{}{"message_contains": "database"},
			expected: false,
		},
		{
			name: "empty filters match all",
			entry: &models.LogEntry{
				Message:   "any message",
				Level:     "info",
				Component: "api",
				Timestamp: time.Now(),
			},
			filters:  map[string]interface{}{},
			expected: true,
		},
		{
			name: "component filter match",
			entry: &models.LogEntry{
				Message:   "something happened",
				Level:     "info",
				Component: "auth",
				Timestamp: time.Now(),
			},
			filters:  map[string]interface{}{"component": "auth"},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.matchesFilters(tt.entry, tt.filters)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogManagementService_GenerateInsights(t *testing.T) {
	service := NewLogManagementService(nil)

	tests := []struct {
		name     string
		entries  []*models.LogEntry
		analysis *models.LogAnalysis
	}{
		{
			name: "some entries with analysis",
			entries: []*models.LogEntry{
				{Level: "info", Component: "api", Message: "server started"},
			},
			analysis: &models.LogAnalysis{
				TotalEntries:       1,
				EntriesByLevel:     map[string]int{"info": 1},
				EntriesByComponent: map[string]int{"api": 1},
				ErrorPatterns:      map[string]int{},
			},
		},
		{
			name: "with some errors",
			entries: []*models.LogEntry{
				{Level: "error", Component: "api", Message: "test error"},
				{Level: "info", Component: "api", Message: "test info"},
			},
			analysis: &models.LogAnalysis{
				TotalEntries:       2,
				EntriesByLevel:     map[string]int{"error": 1, "info": 1},
				EntriesByComponent: map[string]int{"api": 2},
				ErrorPatterns:      map[string]int{"test error": 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			insights := service.generateInsights(tt.entries, tt.analysis)
			assert.NotNil(t, insights)
		})
	}
}

func TestLogManagementService_GetConfiguration(t *testing.T) {
	service := NewLogManagementService(nil)

	config := service.GetConfiguration()
	assert.NotNil(t, config)
	assert.Equal(t, "/var/log/catalogizer", config.LogDirectory)
	assert.Equal(t, int64(100*1024*1024), config.MaxLogSize)
	assert.Equal(t, 10, config.MaxLogFiles)
	assert.Equal(t, 30, config.RetentionDays)
	assert.True(t, config.CompressionEnabled)
	assert.True(t, config.RealTimeLogging)
	assert.True(t, config.AutoCleanup)
	assert.Equal(t, 24, config.MaxShareDuration)
	assert.False(t, config.AllowExternalSharing)
	assert.Contains(t, config.LogLevels, "error")
	assert.Contains(t, config.LogLevels, "warning")
	assert.Contains(t, config.LogLevels, "info")
	assert.Contains(t, config.LogLevels, "debug")
}

func TestLogManagementService_UpdateConfiguration(t *testing.T) {
	service := NewLogManagementService(nil)

	newConfig := &LogManagementConfig{
		LogDirectory:       "/custom/logs",
		MaxLogSize:         50 * 1024 * 1024,
		MaxLogFiles:        5,
		RetentionDays:      7,
		CompressionEnabled: false,
		RealTimeLogging:    false,
		LogLevels:          []string{"error"},
		AutoCleanup:        false,
	}

	err := service.UpdateConfiguration(newConfig)
	assert.NoError(t, err)

	config := service.GetConfiguration()
	assert.Equal(t, "/custom/logs", config.LogDirectory)
	assert.Equal(t, 5, config.MaxLogFiles)
	assert.Equal(t, 7, config.RetentionDays)
	assert.False(t, config.CompressionEnabled)
}

func TestLogManagementService_ExportToCSV(t *testing.T) {
	service := NewLogManagementService(nil)

	tests := []struct {
		name    string
		entries []*models.LogEntry
	}{
		{
			name:    "empty entries",
			entries: []*models.LogEntry{},
		},
		{
			name: "single entry",
			entries: []*models.LogEntry{
				{
					Timestamp: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
					Level:     "error",
					Component: "api",
					Message:   "connection failed",
				},
			},
		},
		{
			name: "entry with quotes in message",
			entries: []*models.LogEntry{
				{
					Timestamp: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
					Level:     "error",
					Component: "api",
					Message:   `failed to parse "config.json"`,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.exportToCSV(tt.entries)
			assert.NoError(t, err)
			assert.NotNil(t, result)
			// Header should always be present
			assert.Contains(t, string(result), "Timestamp,Level,Component,Message,Context")
		})
	}
}

func TestLogManagementService_ExportToText(t *testing.T) {
	service := NewLogManagementService(nil)

	tests := []struct {
		name    string
		entries []*models.LogEntry
	}{
		{
			name:    "empty entries",
			entries: []*models.LogEntry{},
		},
		{
			name: "single entry",
			entries: []*models.LogEntry{
				{
					Timestamp: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
					Level:     "error",
					Component: "api",
					Message:   "connection failed",
				},
			},
		},
		{
			name: "multiple entries",
			entries: []*models.LogEntry{
				{
					Timestamp: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
					Level:     "info",
					Component: "api",
					Message:   "server started",
				},
				{
					Timestamp: time.Date(2025, 1, 1, 10, 5, 0, 0, time.UTC),
					Level:     "warning",
					Component: "auth",
					Message:   "slow query",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.exportToText(tt.entries)
			assert.NoError(t, err)
			if len(tt.entries) > 0 {
				assert.NotEmpty(t, result)
				// Verify the output contains the level in uppercase
				for _, entry := range tt.entries {
					assert.Contains(t, string(result), "["+strings.ToUpper(entry.Level)+"]")
				}
			} else {
				assert.Empty(t, result)
			}
		})
	}
}

func TestLogManagementService_ExportToZip(t *testing.T) {
	service := NewLogManagementService(nil)

	entries := []*models.LogEntry{
		{
			Timestamp: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC),
			Level:     "info",
			Component: "api",
			Message:   "test entry",
		},
	}

	result, err := service.exportToZip(entries)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	// ZIP files start with PK magic bytes
	assert.True(t, len(result) > 2)
	assert.Equal(t, byte('P'), result[0])
	assert.Equal(t, byte('K'), result[1])
}

func TestLogManagementService_FilterLogEntries(t *testing.T) {
	service := NewLogManagementService(nil)

	startTime := time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)
	endTime := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		entries    []*models.LogEntry
		collection *models.LogCollection
		expected   int
	}{
		{
			name:    "no time filter, no level filter",
			entries: []*models.LogEntry{{Level: "info", Timestamp: time.Now(), Component: "api"}},
			collection: &models.LogCollection{
				LogLevel: "",
			},
			expected: 1,
		},
		{
			name: "time filter excludes entry",
			entries: []*models.LogEntry{
				{Level: "info", Timestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), Component: "api"},
			},
			collection: &models.LogCollection{
				StartTime: &startTime,
				EndTime:   &endTime,
			},
			expected: 0,
		},
		{
			name: "time filter includes entry",
			entries: []*models.LogEntry{
				{Level: "info", Timestamp: time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC), Component: "api"},
			},
			collection: &models.LogCollection{
				StartTime: &startTime,
				EndTime:   &endTime,
			},
			expected: 1,
		},
		{
			name: "level filter excludes entry",
			entries: []*models.LogEntry{
				{Level: "debug", Timestamp: time.Now(), Component: "api"},
			},
			collection: &models.LogCollection{
				LogLevel: "warning",
			},
			expected: 0,
		},
		{
			name: "custom filters exclude entry",
			entries: []*models.LogEntry{
				{Level: "info", Timestamp: time.Now(), Component: "api", Message: "server started"},
			},
			collection: &models.LogCollection{
				Filters: map[string]interface{}{"message_contains": "database"},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.filterLogEntries(tt.entries, tt.collection)
			assert.Equal(t, tt.expected, len(result))
		})
	}
}

func TestLogManagementService_IsLogLevelIncluded_UnknownLevels(t *testing.T) {
	service := NewLogManagementService(nil)

	tests := []struct {
		name     string
		level    string
		min      string
		expected bool
	}{
		{
			name:     "unknown entry level",
			level:    "trace",
			min:      "debug",
			expected: false,
		},
		{
			name:     "unknown filter level allows all",
			level:    "debug",
			min:      "trace",
			expected: true,
		},
		{
			name:     "fatal included by error",
			level:    "fatal",
			min:      "error",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.isLogLevelIncluded(tt.level, tt.min)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogManagementService_GenerateInsights_HighErrorRate(t *testing.T) {
	service := NewLogManagementService(nil)

	entries := []*models.LogEntry{
		{Level: "error", Component: "api", Message: "error 1"},
		{Level: "error", Component: "api", Message: "error 2"},
		{Level: "error", Component: "api", Message: "error 3"},
		{Level: "error", Component: "api", Message: "error 4"},
		{Level: "error", Component: "api", Message: "error 5"},
		{Level: "info", Component: "auth", Message: "info 1"},
	}

	analysis := &models.LogAnalysis{
		TotalEntries:       6,
		EntriesByLevel:     map[string]int{"error": 5, "info": 1},
		EntriesByComponent: map[string]int{"api": 5, "auth": 1},
		ErrorPatterns:      map[string]int{"error": 5},
	}

	insights := service.generateInsights(entries, analysis)
	// Should have high error rate insight
	found := false
	for _, insight := range insights {
		if assert.NotEmpty(t, insight) {
			if len(insight) > 0 && insight[0] == 'H' {
				found = true
			}
		}
	}
	assert.True(t, found, "Expected high error rate insight")
}

func TestLogManagementService_ExtractErrorPattern_LongMessage(t *testing.T) {
	service := NewLogManagementService(nil)

	longMessage := "connection refused dial tcp 127.0.0.1:5432 connect connection refused more details here"
	result := service.extractErrorPattern(longMessage)
	assert.Contains(t, result, "...")
}

func TestLogManagementService_FileLogCollector(t *testing.T) {
	collector := &FileLogCollector{
		logPath:       "/var/log/test.log",
		componentName: "test",
	}

	assert.Equal(t, "/var/log/test.log", collector.GetLogPath())
	assert.Equal(t, "test", collector.GetComponentName())
}

func TestLogManagementService_MatchesFilters_LevelFilter(t *testing.T) {
	service := NewLogManagementService(nil)

	entry := &models.LogEntry{
		Message:   "test message",
		Level:     "error",
		Component: "api",
		Timestamp: time.Now(),
	}

	// Component filter mismatch
	result := service.matchesFilters(entry, map[string]interface{}{"component": "auth"})
	assert.False(t, result)

	// Component filter match
	result = service.matchesFilters(entry, map[string]interface{}{"component": "api"})
	assert.True(t, result)

	// Case-insensitive message contains
	result = service.matchesFilters(entry, map[string]interface{}{"message_contains": "TEST"})
	assert.True(t, result)
}

func TestLogManagementService_ExportToCSV_Empty(t *testing.T) {
	service := NewLogManagementService(nil)

	data, err := service.exportToCSV([]*models.LogEntry{})
	assert.NoError(t, err)
	assert.NotNil(t, data)
	assert.Contains(t, string(data), "Timestamp,Level,Component")
}

func TestLogManagementService_ExportToText_Empty(t *testing.T) {
	service := NewLogManagementService(nil)

	data, err := service.exportToText([]*models.LogEntry{})
	assert.NoError(t, err)
	assert.Equal(t, "", string(data))
}

func TestLogManagementService_FilterLogEntries_ByLogLevel(t *testing.T) {
	service := NewLogManagementService(nil)

	now := time.Now()
	entries := []*models.LogEntry{
		{Timestamp: now, Level: "debug", Message: "debug msg"},
		{Timestamp: now, Level: "info", Message: "info msg"},
		{Timestamp: now, Level: "error", Message: "error msg"},
	}

	collection := &models.LogCollection{
		LogLevel: "error",
	}

	filtered := service.filterLogEntries(entries, collection)
	assert.Len(t, filtered, 1) // only error (error level >= error)
	assert.Equal(t, "error msg", filtered[0].Message)
}

func TestLogManagementService_FilterLogEntries_ByTimeRange(t *testing.T) {
	service := NewLogManagementService(nil)

	now := time.Now()
	past := now.Add(-2 * time.Hour)
	future := now.Add(2 * time.Hour)

	entries := []*models.LogEntry{
		{Timestamp: past, Level: "info", Message: "past"},
		{Timestamp: now, Level: "info", Message: "now"},
		{Timestamp: future, Level: "info", Message: "future"},
	}

	startTime := now.Add(-1 * time.Hour)
	endTime := now.Add(1 * time.Hour)
	collection := &models.LogCollection{
		StartTime: &startTime,
		EndTime:   &endTime,
	}

	filtered := service.filterLogEntries(entries, collection)
	assert.Len(t, filtered, 1)
	assert.Equal(t, "now", filtered[0].Message)
}

func TestLogManagementService_GenerateInsights_LowErrorRate(t *testing.T) {
	service := NewLogManagementService(nil)

	analysis := &models.LogAnalysis{
		TotalEntries:       100,
		EntriesByLevel:     map[string]int{"error": 5, "info": 95},
		EntriesByComponent: map[string]int{"api": 50},
	}

	entries := []*models.LogEntry{}
	insights := service.generateInsights(entries, analysis)

	assert.NotEmpty(t, insights)
	assert.Contains(t, insights[0], "api")
}

func TestDatabaseLogCollector_GetComponentName(t *testing.T) {
	collector := &DatabaseLogCollector{
		componentName: "database",
	}

	assert.Equal(t, "database", collector.GetComponentName())
}

func TestDatabaseLogCollector_GetLogPath(t *testing.T) {
	collector := &DatabaseLogCollector{
		componentName: "database",
	}

	// Database collector returns "database" as path identifier
	assert.Equal(t, "database", collector.GetLogPath())
}

func TestLogManagementService_matchesFilters(t *testing.T) {
	service := NewLogManagementService(nil)

	entry := &models.LogEntry{
		Level:     "error",
		Component: "api",
		Message:   "Test error message",
	}

	// No filters - should match
	result := service.matchesFilters(entry, nil)
	assert.True(t, result)

	// Empty filters - should match
	result = service.matchesFilters(entry, map[string]interface{}{})
	assert.True(t, result)

	// Component filter mismatch
	result = service.matchesFilters(entry, map[string]interface{}{"component": "auth"})
	assert.False(t, result)

	// Component filter match
	result = service.matchesFilters(entry, map[string]interface{}{"component": "api"})
	assert.True(t, result)

	// Case-insensitive message contains
	result = service.matchesFilters(entry, map[string]interface{}{"message_contains": "TEST"})
	assert.True(t, result)

	// Message contains - no match
	result = service.matchesFilters(entry, map[string]interface{}{"message_contains": "xyz"})
	assert.False(t, result)

	// Multiple filters - all match
	result = service.matchesFilters(entry, map[string]interface{}{
		"component":        "api",
		"message_contains": "error",
	})
	assert.True(t, result)

	// Multiple filters - one mismatch
	result = service.matchesFilters(entry, map[string]interface{}{
		"component":        "auth",
		"message_contains": "error",
	})
	assert.False(t, result)
}
