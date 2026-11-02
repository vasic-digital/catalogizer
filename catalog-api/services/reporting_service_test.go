package services

import (
	"fmt"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/stretchr/testify/assert"
)

func TestReportingService_CalculateSystemHealth(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	tests := []struct {
		name          string
		totalUsers    int
		activeUsers   int
		mediaAccesses int
	}{
		{
			name:          "healthy system with active users",
			totalUsers:    100,
			activeUsers:   80,
			mediaAccesses: 5000,
		},
		{
			name:          "system with low activity",
			totalUsers:    100,
			activeUsers:   5,
			mediaAccesses: 10,
		},
		{
			name:          "empty system",
			totalUsers:    0,
			activeUsers:   0,
			mediaAccesses: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			health := service.calculateSystemHealth(tt.totalUsers, tt.activeUsers, tt.mediaAccesses)
			assert.NotEmpty(t, health.Status)
		})
	}
}

func TestReportingService_ExtractDateRange(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	tests := []struct {
		name    string
		params  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid date range",
			params: map[string]interface{}{
				"start_date": "2025-01-01",
				"end_date":   "2025-01-31",
			},
			wantErr: false,
		},
		{
			name:    "missing start_date",
			params:  map[string]interface{}{"end_date": "2025-01-31"},
			wantErr: true,
		},
		{
			name:    "missing end_date",
			params:  map[string]interface{}{"start_date": "2025-01-01"},
			wantErr: true,
		},
		{
			name:    "empty params",
			params:  map[string]interface{}{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := service.extractDateRange(tt.params)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.False(t, start.IsZero())
				assert.False(t, end.IsZero())
				assert.True(t, end.After(start) || end.Equal(start))
			}
		})
	}
}

func TestReportingService_CountUniqueUsers(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	tests := []struct {
		name     string
		logs     []models.MediaAccessLog
		expected int
	}{
		{
			name:     "no logs",
			logs:     []models.MediaAccessLog{},
			expected: 0,
		},
		{
			name: "unique users",
			logs: []models.MediaAccessLog{
				{UserID: 1, MediaID: 1, AccessTime: time.Now()},
				{UserID: 2, MediaID: 1, AccessTime: time.Now()},
				{UserID: 3, MediaID: 1, AccessTime: time.Now()},
			},
			expected: 3,
		},
		{
			name: "duplicate users",
			logs: []models.MediaAccessLog{
				{UserID: 1, MediaID: 1, AccessTime: time.Now()},
				{UserID: 2, MediaID: 1, AccessTime: time.Now()},
				{UserID: 1, MediaID: 2, AccessTime: time.Now()},
				{UserID: 3, MediaID: 1, AccessTime: time.Now()},
				{UserID: 2, MediaID: 3, AccessTime: time.Now()},
			},
			expected: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := service.countUniqueUsers(tt.logs)
			assert.Equal(t, tt.expected, count)
		})
	}
}

func TestReportingService_AnalyzeTimeDistribution(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	tests := []struct {
		name string
		logs []models.MediaAccessLog
	}{
		{
			name: "empty logs",
			logs: []models.MediaAccessLog{},
		},
		{
			name: "various access times",
			logs: []models.MediaAccessLog{
				{UserID: 1, MediaID: 1, AccessTime: time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC)},
				{UserID: 1, MediaID: 2, AccessTime: time.Date(2025, 1, 1, 14, 0, 0, 0, time.UTC)},
				{UserID: 1, MediaID: 3, AccessTime: time.Date(2025, 1, 1, 20, 0, 0, 0, time.UTC)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			distribution := service.analyzeTimeDistribution(tt.logs)
			assert.NotNil(t, distribution)
		})
	}
}

func TestReportingService_GetMostActiveHour(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	tests := []struct {
		name string
		logs []models.MediaAccessLog
	}{
		{
			name: "empty logs",
			logs: []models.MediaAccessLog{},
		},
		{
			name: "peak at 14:00",
			logs: []models.MediaAccessLog{
				{UserID: 1, MediaID: 1, AccessTime: time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC)},
				{UserID: 1, MediaID: 2, AccessTime: time.Date(2025, 1, 1, 14, 0, 0, 0, time.UTC)},
				{UserID: 2, MediaID: 3, AccessTime: time.Date(2025, 1, 1, 14, 30, 0, 0, time.UTC)},
				{UserID: 3, MediaID: 4, AccessTime: time.Date(2025, 1, 1, 14, 45, 0, 0, time.UTC)},
				{UserID: 1, MediaID: 5, AccessTime: time.Date(2025, 1, 1, 20, 0, 0, 0, time.UTC)},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hour := service.getMostActiveHour(tt.logs)
			assert.GreaterOrEqual(t, hour, 0)
			assert.LessOrEqual(t, hour, 23)
		})
	}
}

func TestReportingService_FormatReport(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	tests := []struct {
		name       string
		format     string
		reportType string
		data       interface{}
		wantErr    bool
	}{
		{
			name:       "json format",
			format:     "json",
			reportType: "generic",
			data:       map[string]interface{}{"key": "value"},
			wantErr:    false,
		},
		{
			name:       "unsupported format",
			format:     "xml",
			reportType: "generic",
			data:       map[string]interface{}{"key": "value"},
			wantErr:    true,
		},
		{
			name:       "empty data json",
			format:     "json",
			reportType: "generic",
			data:       map[string]interface{}{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := service.formatReport(tt.data, tt.format, tt.reportType)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
			}
		})
	}
}

func TestReportingService_FilterLogsByDateRange(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	startDate := time.Date(2025, 1, 5, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name     string
		logs     []models.MediaAccessLog
		expected int
	}{
		{
			name:     "empty logs",
			logs:     []models.MediaAccessLog{},
			expected: 0,
		},
		{
			name: "all within range",
			logs: []models.MediaAccessLog{
				{UserID: 1, MediaID: 1, AccessTime: time.Date(2025, 1, 6, 10, 0, 0, 0, time.UTC)},
				{UserID: 2, MediaID: 2, AccessTime: time.Date(2025, 1, 10, 14, 0, 0, 0, time.UTC)},
			},
			expected: 2,
		},
		{
			name: "some outside range",
			logs: []models.MediaAccessLog{
				{UserID: 1, MediaID: 1, AccessTime: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)},
				{UserID: 2, MediaID: 2, AccessTime: time.Date(2025, 1, 10, 14, 0, 0, 0, time.UTC)},
				{UserID: 3, MediaID: 3, AccessTime: time.Date(2025, 1, 20, 18, 0, 0, 0, time.UTC)},
			},
			expected: 1,
		},
		{
			name: "all outside range",
			logs: []models.MediaAccessLog{
				{UserID: 1, MediaID: 1, AccessTime: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)},
				{UserID: 2, MediaID: 2, AccessTime: time.Date(2025, 1, 20, 14, 0, 0, 0, time.UTC)},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.filterLogsByDateRange(tt.logs, startDate, endDate)
			assert.Equal(t, tt.expected, len(result))
		})
	}
}

func TestReportingService_AnalyzeUserAccessPatterns(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	tests := []struct {
		name string
		logs []models.MediaAccessLog
	}{
		{
			name: "empty logs",
			logs: []models.MediaAccessLog{},
		},
		{
			name: "logs with various hours and days",
			logs: []models.MediaAccessLog{
				{UserID: 1, MediaID: 1, AccessTime: time.Date(2025, 1, 6, 8, 0, 0, 0, time.UTC)},  // Monday
				{UserID: 1, MediaID: 2, AccessTime: time.Date(2025, 1, 6, 14, 0, 0, 0, time.UTC)}, // Monday
				{UserID: 1, MediaID: 3, AccessTime: time.Date(2025, 1, 7, 20, 0, 0, 0, time.UTC)}, // Tuesday
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patterns := service.analyzeUserAccessPatterns(tt.logs)
			assert.NotNil(t, patterns)
			assert.Contains(t, patterns, "hourly")
			assert.Contains(t, patterns, "daily")
		})
	}
}

func TestReportingService_AnalyzeUserDeviceUsage(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	platform := "iOS"
	deviceModel := "iPhone 14"

	tests := []struct {
		name     string
		logs     []models.MediaAccessLog
		expected int
	}{
		{
			name:     "empty logs",
			logs:     []models.MediaAccessLog{},
			expected: 0,
		},
		{
			name: "logs with device info",
			logs: []models.MediaAccessLog{
				{
					UserID: 1, MediaID: 1,
					AccessTime: time.Now(),
					DeviceInfo: &models.DeviceInfo{Platform: &platform, DeviceModel: &deviceModel},
				},
			},
			expected: 1,
		},
		{
			name: "logs without device info",
			logs: []models.MediaAccessLog{
				{UserID: 1, MediaID: 1, AccessTime: time.Now(), DeviceInfo: nil},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.analyzeUserDeviceUsage(tt.logs)
			assert.Equal(t, tt.expected, len(result))
		})
	}
}

func TestReportingService_AnalyzeUserLocations(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	tests := []struct {
		name     string
		logs     []models.MediaAccessLog
		expected int
	}{
		{
			name:     "empty logs",
			logs:     []models.MediaAccessLog{},
			expected: 0,
		},
		{
			name: "logs with locations",
			logs: []models.MediaAccessLog{
				{
					UserID: 1, MediaID: 1, AccessTime: time.Now(),
					Location: &models.Location{Latitude: 40.71, Longitude: -74.00},
				},
				{
					UserID: 2, MediaID: 2, AccessTime: time.Now(),
					Location: &models.Location{Latitude: 51.51, Longitude: -0.13},
				},
			},
			expected: 2,
		},
		{
			name: "logs without locations",
			logs: []models.MediaAccessLog{
				{UserID: 1, MediaID: 1, AccessTime: time.Now(), Location: nil},
			},
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.analyzeUserLocations(tt.logs)
			assert.Equal(t, tt.expected, len(result))
		})
	}
}

func TestReportingService_AnalyzeUserPopularContent(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	tests := []struct {
		name     string
		logs     []models.MediaAccessLog
		expected int
	}{
		{
			name:     "empty logs",
			logs:     []models.MediaAccessLog{},
			expected: 0,
		},
		{
			name: "multiple accesses same media",
			logs: []models.MediaAccessLog{
				{UserID: 1, MediaID: 1, AccessTime: time.Now()},
				{UserID: 1, MediaID: 1, AccessTime: time.Now()},
				{UserID: 1, MediaID: 2, AccessTime: time.Now()},
			},
			expected: 2, // 2 unique media items
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.analyzeUserPopularContent(tt.logs)
			assert.Equal(t, tt.expected, len(result))
		})
	}
}

func TestReportingService_GetLastActivityTime(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	tests := []struct {
		name     string
		logs     []models.MediaAccessLog
		expected time.Time
	}{
		{
			name:     "empty logs returns zero time",
			logs:     []models.MediaAccessLog{},
			expected: time.Time{},
		},
		{
			name: "single log",
			logs: []models.MediaAccessLog{
				{UserID: 1, MediaID: 1, AccessTime: time.Date(2025, 1, 10, 14, 0, 0, 0, time.UTC)},
			},
			expected: time.Date(2025, 1, 10, 14, 0, 0, 0, time.UTC),
		},
		{
			name: "multiple logs returns latest",
			logs: []models.MediaAccessLog{
				{UserID: 1, MediaID: 1, AccessTime: time.Date(2025, 1, 5, 10, 0, 0, 0, time.UTC)},
				{UserID: 1, MediaID: 2, AccessTime: time.Date(2025, 1, 15, 20, 0, 0, 0, time.UTC)},
				{UserID: 1, MediaID: 3, AccessTime: time.Date(2025, 1, 10, 14, 0, 0, 0, time.UTC)},
			},
			expected: time.Date(2025, 1, 15, 20, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.getLastActivityTime(tt.logs)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReportingService_GetPreferredDevices(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	tests := []struct {
		name string
		logs []models.MediaAccessLog
	}{
		{
			name: "empty logs",
			logs: []models.MediaAccessLog{},
		},
		{
			name: "logs with device info",
			logs: func() []models.MediaAccessLog {
				platform := "Android"
				model := "Pixel 7"
				return []models.MediaAccessLog{
					{UserID: 1, MediaID: 1, AccessTime: time.Now(), DeviceInfo: &models.DeviceInfo{Platform: &platform, DeviceModel: &model}},
				}
			}(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.getPreferredDevices(tt.logs)
			// Result may be nil for empty device info
			if len(tt.logs) > 0 && tt.logs[0].DeviceInfo != nil {
				assert.NotNil(t, result)
			}
		})
	}
}

func TestReportingService_GetAccessedLocations(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	logs := []models.MediaAccessLog{
		{UserID: 1, MediaID: 1, AccessTime: time.Now(), Location: &models.Location{Latitude: 40.71, Longitude: -74.00}},
	}

	result := service.getAccessedLocations(logs)
	assert.NotNil(t, result)
	assert.Equal(t, 1, len(result))
}

func TestReportingService_GenerateActivitySummary(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	tests := []struct {
		name       string
		activities []models.UserActivitySummary
	}{
		{
			name:       "empty activities",
			activities: []models.UserActivitySummary{},
		},
		{
			name: "multiple activities",
			activities: []models.UserActivitySummary{
				{TotalAccesses: 10},
				{TotalAccesses: 20},
				{TotalAccesses: 30},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			summary := service.generateActivitySummary(tt.activities)
			if len(tt.activities) == 0 {
				assert.Equal(t, 0, summary.TotalUsers)
			} else {
				assert.Equal(t, len(tt.activities), summary.TotalUsers)
				assert.Equal(t, 60, summary.TotalAccesses)
				assert.Equal(t, 20.0, summary.AverageAccesses)
			}
		})
	}
}

func TestReportingService_CalculateUsageStatistics(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	stats := service.calculateUsageStatistics(startDate, endDate)
	// With nil repository, returns empty stats
	assert.Empty(t, stats.PeakHours)
	assert.Equal(t, 0, stats.AverageDaily)
}

func TestReportingService_CalculatePerformanceMetrics(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	metrics := service.calculatePerformanceMetrics(startDate, endDate)
	// With nil repository, returns zero values
	assert.Equal(t, 0.0, metrics.ResponseTime)
	assert.Equal(t, 0, metrics.Throughput)
}

func TestReportingService_CalculateSecurityMetrics(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	metrics := service.calculateSecurityMetrics(startDate, endDate)
	// No database: the metrics are unmeasurable, not "low threat, score 100".
	assert.Equal(t, "unknown", metrics.ThreatLevel)
	assert.Contains(t, metrics.Unavailable, "failed_logins")
}

func TestReportingService_CalculateAverageSessionDuration(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	tests := []struct {
		name     string
		sessions []models.SessionData
		expected time.Duration
	}{
		{
			name:     "empty sessions",
			sessions: []models.SessionData{},
			expected: 0,
		},
		{
			name: "single session",
			sessions: []models.SessionData{
				{Duration: 10 * time.Minute},
			},
			expected: 10 * time.Minute,
		},
		{
			name: "multiple sessions",
			sessions: []models.SessionData{
				{Duration: 10 * time.Minute},
				{Duration: 20 * time.Minute},
				{Duration: 30 * time.Minute},
			},
			expected: 20 * time.Minute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.calculateAverageSessionDuration(tt.sessions)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestReportingService_CalculateResponseTimes(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	result := service.calculateResponseTimes(startDate, endDate)
	// With nil repository, returns zero values
	assert.Equal(t, 0.0, result.Average)
	assert.Equal(t, 0.0, result.Min)
	assert.Equal(t, 0.0, result.Max)
}

func TestReportingService_CalculateSystemLoad(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	result := service.calculateSystemLoad(startDate, endDate)
	// CPU / Disk / Network aren't instrumented yet (Phase 6 of the
	// completion roadmap wires a full system metrics collector that
	// populates these); they must remain 0 until that lands.
	assert.Equal(t, 0.0, result.CPU)
	assert.Equal(t, 0.0, result.Disk)
	assert.Equal(t, 0.0, result.Network)
	// Memory is read from the Go runtime's live MemStats, so it reflects
	// actual heap usage of the test process — always non-zero, always
	// less than 100% (Memory is reported as HeapInuse / SysBytes).
	assert.Greater(t, result.Memory, 0.0, "Memory should reflect real runtime heap usage")
	assert.Less(t, result.Memory, 100.0, "Memory is a percentage (HeapInuse/SysBytes)")
}

func TestReportingService_CalculateErrorRates(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	startDate := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	result := service.calculateErrorRates(startDate, endDate)
	// With nil repository, returns zero value
	assert.Equal(t, 0.0, result.Total)
}

func TestReportingService_AnalyzeUserEngagement(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	logs := []models.MediaAccessLog{
		{UserID: 1, MediaID: 1, AccessTime: time.Now()},
	}

	engagement := service.analyzeUserEngagement(logs)
	// Single log entry means no sessions can be calculated
	assert.Equal(t, 0.0, engagement.AverageSessionTime)
	assert.Equal(t, 0.0, engagement.ReturnRate)
}

func TestReportingService_ExtractDateRange_InvalidFormats(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	tests := []struct {
		name   string
		params map[string]interface{}
	}{
		{
			name:   "invalid start_date format",
			params: map[string]interface{}{"start_date": "not-a-date", "end_date": "2025-01-31"},
		},
		{
			name:   "invalid end_date format",
			params: map[string]interface{}{"start_date": "2025-01-01", "end_date": "not-a-date"},
		},
		{
			name:   "non-string start_date",
			params: map[string]interface{}{"start_date": 12345, "end_date": "2025-01-31"},
		},
		{
			name:   "non-string end_date",
			params: map[string]interface{}{"start_date": "2025-01-01", "end_date": 12345},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := service.extractDateRange(tt.params)
			assert.Error(t, err)
		})
	}
}

// ---------------------------------------------------------------------------
// GenerateReport
// ---------------------------------------------------------------------------

func TestReportingService_GenerateReport_UnsupportedType(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report, err := service.GenerateReport("unsupported_type", "json", map[string]interface{}{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported report type")
	assert.Nil(t, report)
}

func TestReportingService_GenerateReport_UserAnalytics_MissingUserID(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report, err := service.GenerateReport("user_analytics", "json", map[string]interface{}{
		"start_date": "2025-01-01",
		"end_date":   "2025-01-31",
	})
	assert.Error(t, err)
	// With nil repositories, returns "repository not initialized" error
	assert.Contains(t, err.Error(), "repository not initialized")
	assert.Nil(t, report)
}

func TestReportingService_GenerateReport_UserAnalytics_MissingDateRange(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report, err := service.GenerateReport("user_analytics", "json", map[string]interface{}{
		"user_id": 1,
	})
	assert.Error(t, err)
	assert.Nil(t, report)
}

func TestReportingService_GenerateReport_SystemOverview_MissingDateRange(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report, err := service.GenerateReport("system_overview", "json", map[string]interface{}{})
	assert.Error(t, err)
	assert.Nil(t, report)
}

func TestReportingService_GenerateReport_MediaAnalytics_MissingDateRange(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report, err := service.GenerateReport("media_analytics", "json", map[string]interface{}{})
	assert.Error(t, err)
	assert.Nil(t, report)
}

func TestReportingService_GenerateReport_SecurityAudit_MissingDateRange(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report, err := service.GenerateReport("security_audit", "json", map[string]interface{}{})
	assert.Error(t, err)
	assert.Nil(t, report)
}

func TestReportingService_GenerateReport_PerformanceMetrics_MissingDateRange(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report, err := service.GenerateReport("performance_metrics", "json", map[string]interface{}{})
	assert.Error(t, err)
	assert.Nil(t, report)
}

// ---------------------------------------------------------------------------
// formatReport
// ---------------------------------------------------------------------------

func TestReportingService_FormatReport_UnsupportedFormat(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	content, err := service.formatReport(map[string]string{"test": "data"}, "invalid_format", "test_report")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
	assert.Nil(t, content)
}

// ---------------------------------------------------------------------------
// calculateSystemHealth
// ---------------------------------------------------------------------------

func TestReportingService_CalculateSystemHealth_AllScenarios(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	tests := []struct {
		name          string
		totalUsers    int
		activeUsers   int
		mediaAccesses int
		expectHealthy bool
	}{
		{
			name:          "healthy system",
			totalUsers:    100,
			activeUsers:   80,
			mediaAccesses: 5000,
			expectHealthy: true,
		},
		{
			name:          "warning system - low activity",
			totalUsers:    100,
			activeUsers:   20,
			mediaAccesses: 100,
			expectHealthy: false,
		},
		{
			name:          "critical system - no users",
			totalUsers:    0,
			activeUsers:   0,
			mediaAccesses: 0,
			expectHealthy: false,
		},
		{
			name:          "single active user",
			totalUsers:    1,
			activeUsers:   1,
			mediaAccesses: 10,
			expectHealthy: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			health := service.calculateSystemHealth(tt.totalUsers, tt.activeUsers, tt.mediaAccesses)
			assert.NotNil(t, health)
			assert.NotEmpty(t, health.Status)
		})
	}
}

// ---------------------------------------------------------------------------
// extractDateRange additional tests
// ---------------------------------------------------------------------------

func TestReportingService_ExtractDateRange_ValidRange(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	start, end, err := service.extractDateRange(map[string]interface{}{
		"start_date": "2025-01-01",
		"end_date":   "2025-01-31",
	})
	assert.NoError(t, err)
	assert.NotNil(t, start)
	assert.NotNil(t, end)
	assert.True(t, start.Before(end) || start.Equal(end))
}

// ---------------------------------------------------------------------------
// Wrapper function tests
// ---------------------------------------------------------------------------

func TestReportingService_AnalyzeUserTimePatterns(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	logs := []models.MediaAccessLog{
		{AccessTime: time.Date(2025, 1, 1, 9, 0, 0, 0, time.UTC)},
		{AccessTime: time.Date(2025, 1, 1, 14, 0, 0, 0, time.UTC)},
	}

	result := service.analyzeUserTimePatterns(logs)
	assert.NotNil(t, result)

	hourly, ok := result["hourly"].(map[int]int)
	assert.True(t, ok)
	assert.Equal(t, 1, hourly[9])
	assert.Equal(t, 1, hourly[14])
}

func TestReportingService_AnalyzeAccessPatterns(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	logs := []models.MediaAccessLog{
		{AccessTime: time.Date(2025, 1, 6, 9, 0, 0, 0, time.UTC)}, // Monday
	}

	result := service.analyzeAccessPatterns(logs)
	assert.NotNil(t, result)

	hourly, ok := result["hourly"].(map[int]int)
	assert.True(t, ok)
	assert.Equal(t, 1, hourly[9])
}

func TestReportingService_AnalyzeGeographicDistribution(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	logs := []models.MediaAccessLog{
		{Location: &models.Location{Latitude: 40.71, Longitude: -74.01}},
		{Location: &models.Location{Latitude: 51.51, Longitude: -0.13}},
	}

	result := service.analyzeGeographicDistribution(logs)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result["40.71,-74.01"])
	assert.Equal(t, 1, result["51.51,-0.13"])
}

func TestReportingService_AnalyzeGeographicDistribution_Empty(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	result := service.analyzeGeographicDistribution([]models.MediaAccessLog{})
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

func TestReportingService_AnalyzeDeviceDistribution(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	android := "Android"
	pixel := "Pixel 7"
	ios := "iOS"
	iphone := "iPhone 15"

	logs := []models.MediaAccessLog{
		{DeviceInfo: &models.DeviceInfo{Platform: &android, DeviceModel: &pixel}},
		{DeviceInfo: &models.DeviceInfo{Platform: &ios, DeviceModel: &iphone}},
	}

	result := service.analyzeDeviceDistribution(logs)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result["Android Pixel 7"])
	assert.Equal(t, 1, result["iOS iPhone 15"])
}

func TestReportingService_AnalyzeDeviceDistribution_Empty(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	result := service.analyzeDeviceDistribution([]models.MediaAccessLog{})
	assert.NotNil(t, result)
	assert.Empty(t, result)
}

// ---------------------------------------------------------------------------
// formatAsMarkdown tests
// ---------------------------------------------------------------------------

func TestReportingService_FormatAsMarkdown_UserAnalytics(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	displayName := "Test User"
	report := &models.UserAnalyticsReport{
		User: &models.User{
			ID:          1,
			Username:    "testuser",
			DisplayName: &displayName,
			Email:       "test@example.com",
			CreatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalMediaAccesses: 150,
		TotalEvents:        42,
	}

	content, err := service.formatAsMarkdown(report, "user_analytics")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "# User Analytics Report")
	assert.Contains(t, contentStr, "Test User")
	assert.Contains(t, contentStr, "testuser")
	assert.Contains(t, contentStr, "Total Media Accesses: 150")
	assert.Contains(t, contentStr, "Total Events: 42")
	assert.Contains(t, contentStr, "2025-01-01")
}

func TestReportingService_FormatAsMarkdown_UserAnalytics_NilDisplayName(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.UserAnalyticsReport{
		User: &models.User{
			ID:          1,
			Username:    "testuser",
			DisplayName: nil,
			Email:       "test@example.com",
			CreatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalMediaAccesses: 0,
		TotalEvents:        0,
	}

	content, err := service.formatAsMarkdown(report, "user_analytics")
	assert.NoError(t, err)
	assert.NotNil(t, content)
	assert.Contains(t, string(content), "# User Analytics Report")
}

func TestReportingService_FormatAsMarkdown_SystemOverview(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.SystemOverviewReport{
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalUsers:         100,
		ActiveUsers:        80,
		TotalMediaAccesses: 5000,
		TotalEvents:        200,
	}

	content, err := service.formatAsMarkdown(report, "system_overview")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "# System Overview Report")
	assert.Contains(t, contentStr, "Total Users: 100")
	assert.Contains(t, contentStr, "Active Users: 80")
	assert.Contains(t, contentStr, "Total Media Accesses: 5000")
	assert.Contains(t, contentStr, "Total Events: 200")
}

func TestReportingService_FormatAsMarkdown_DefaultType(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	data := map[string]string{"key": "value"}

	content, err := service.formatAsMarkdown(data, "unknown_type")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "# unknown_type Report")
	assert.Contains(t, contentStr, "```json")
}

// ---------------------------------------------------------------------------
// formatAsHTML tests
// ---------------------------------------------------------------------------

func TestReportingService_FormatAsHTML_UserAnalytics(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	displayName := "Test User"
	report := &models.UserAnalyticsReport{
		User: &models.User{
			ID:          1,
			Username:    "testuser",
			DisplayName: &displayName,
			Email:       "test@example.com",
			CreatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalMediaAccesses: 150,
		TotalEvents:        42,
	}

	content, err := service.formatAsHTML(report, "user_analytics")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "<!DOCTYPE html>")
	assert.Contains(t, contentStr, "testuser")
	assert.Contains(t, contentStr, "Test User")
	assert.Contains(t, contentStr, "test@example.com")
	assert.Contains(t, contentStr, "Total Media Accesses: 150")
	assert.Contains(t, contentStr, "Total Events: 42")
}

func TestReportingService_FormatAsHTML_UserAnalytics_NilDisplayName(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.UserAnalyticsReport{
		User: &models.User{
			ID:          1,
			Username:    "testuser",
			DisplayName: nil,
			Email:       "test@example.com",
			CreatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalMediaAccesses: 0,
		TotalEvents:        0,
	}

	content, err := service.formatAsHTML(report, "user_analytics")
	assert.NoError(t, err)
	assert.NotNil(t, content)
	assert.Contains(t, string(content), "<!DOCTYPE html>")
}

func TestReportingService_FormatAsHTML_SystemOverview(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.SystemOverviewReport{
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalUsers:         100,
		ActiveUsers:        80,
		TotalMediaAccesses: 5000,
		TotalEvents:        200,
	}

	content, err := service.formatAsHTML(report, "system_overview")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "<!DOCTYPE html>")
	assert.Contains(t, contentStr, "Total Users: 100")
	assert.Contains(t, contentStr, "Active Users: 80")
	assert.Contains(t, contentStr, "Total Media Accesses: 5000")
	assert.Contains(t, contentStr, "Total Events: 200")
}

func TestReportingService_FormatAsHTML_DefaultType(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	data := map[string]string{"key": "value"}

	content, err := service.formatAsHTML(data, "unknown_type")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "<!DOCTYPE html>")
	assert.Contains(t, contentStr, "<pre>")
}

// ---------------------------------------------------------------------------
// formatReport with markdown and html
// ---------------------------------------------------------------------------

func TestReportingService_FormatReport_Markdown(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	data := map[string]string{"test": "data"}
	content, err := service.formatReport(data, "markdown", "generic_report")
	assert.NoError(t, err)
	assert.NotNil(t, content)
	assert.Contains(t, string(content), "# generic_report Report")
}

func TestReportingService_FormatReport_HTML(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	data := map[string]string{"test": "data"}
	content, err := service.formatReport(data, "html", "generic_report")
	assert.NoError(t, err)
	assert.NotNil(t, content)
	assert.Contains(t, string(content), "<!DOCTYPE html>")
}

// ---------------------------------------------------------------------------
// Additional helper method tests
// ---------------------------------------------------------------------------

func TestReportingService_GenerateActivitySummary_SingleActivity(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	activities := []models.UserActivitySummary{
		{TotalAccesses: 50},
	}

	summary := service.generateActivitySummary(activities)
	assert.Equal(t, 1, summary.TotalUsers)
	assert.Equal(t, 50, summary.TotalAccesses)
	assert.Equal(t, 50.0, summary.AverageAccesses)
}

func TestReportingService_GetMostActiveHour_SingleAccess(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	logs := []models.MediaAccessLog{
		{AccessTime: time.Date(2025, 1, 1, 15, 0, 0, 0, time.UTC)},
	}

	hour := service.getMostActiveHour(logs)
	assert.Equal(t, 15, hour)
}

func TestReportingService_AnalyzeTimeDistribution_AllSlots(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	logs := []models.MediaAccessLog{
		{AccessTime: time.Date(2025, 1, 1, 8, 0, 0, 0, time.UTC)},  // morning
		{AccessTime: time.Date(2025, 1, 1, 14, 0, 0, 0, time.UTC)}, // afternoon
		{AccessTime: time.Date(2025, 1, 1, 19, 0, 0, 0, time.UTC)}, // evening
		{AccessTime: time.Date(2025, 1, 1, 2, 0, 0, 0, time.UTC)},  // night
	}

	dist := service.analyzeTimeDistribution(logs)
	assert.Equal(t, 1, dist["morning"])
	assert.Equal(t, 1, dist["afternoon"])
	assert.Equal(t, 1, dist["evening"])
	assert.Equal(t, 1, dist["night"])
}

// ===========================================================================
// Additional formatReport tests
// ===========================================================================

func TestReportingService_FormatReport_JSON_Complex(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	data := map[string]interface{}{
		"nested": map[string]interface{}{
			"key": "value",
		},
		"array": []int{1, 2, 3},
	}

	result, err := service.formatReport(data, "json", "default")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, string(result), "nested")
	assert.Contains(t, string(result), "key")
}

func TestReportingService_FormatReport_PDF_ReturnsError(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	data := map[string]interface{}{"test": "data"}

	// PDF generation fails without unipdf license
	_, err := service.formatReport(data, "pdf", "default")
	assert.Error(t, err)
}

// ===========================================================================
// Format methods: edge cases, special characters, empty data, large data
// ===========================================================================

func TestReportingService_FormatAsMarkdown_SpecialCharacters(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	displayName := "Test <User> & \"Special\" 'Chars'"
	report := &models.UserAnalyticsReport{
		User: &models.User{
			ID:          1,
			Username:    "user<script>alert(1)</script>",
			DisplayName: &displayName,
			Email:       "test+special@example.com",
			CreatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalMediaAccesses: 42,
		TotalEvents:        7,
	}

	content, err := service.formatAsMarkdown(report, "user_analytics")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "Test <User> & \"Special\" 'Chars'")
	assert.Contains(t, contentStr, "user<script>alert(1)</script>")
	assert.Contains(t, contentStr, "Total Media Accesses: 42")
}

func TestReportingService_FormatAsMarkdown_UnicodeCharacters(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	displayName := "\u041c\u0438\u043b\u043e\u0448 \u0412\u0430\u0441\u0438\u0107"
	report := &models.UserAnalyticsReport{
		User: &models.User{
			ID:          1,
			Username:    "\u7528\u6237\u540d",
			DisplayName: &displayName,
			Email:       "unicode@\u4f8b\u3048.jp",
			CreatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalMediaAccesses: 10,
		TotalEvents:        5,
	}

	content, err := service.formatAsMarkdown(report, "user_analytics")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "\u041c\u0438\u043b\u043e\u0448 \u0412\u0430\u0441\u0438\u0107")
	assert.Contains(t, contentStr, "\u7528\u6237\u540d")
}

func TestReportingService_FormatAsHTML_SpecialCharacters(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	displayName := "User <b>Bold</b> & \"Quoted\""
	report := &models.UserAnalyticsReport{
		User: &models.User{
			ID:          1,
			Username:    "user&name",
			DisplayName: &displayName,
			Email:       "test@example.com",
			CreatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalMediaAccesses: 10,
		TotalEvents:        5,
	}

	content, err := service.formatAsHTML(report, "user_analytics")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "<!DOCTYPE html>")
	// Content is injected as template.HTML (raw), so & is NOT escaped
	assert.Contains(t, contentStr, "user&name")
	assert.Contains(t, contentStr, "User <b>Bold</b>")
}

func TestReportingService_FormatAsHTML_UnicodeCharacters(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	displayName := "\u65e5\u672c\u8a9e\u30e6\u30fc\u30b6\u30fc"
	report := &models.UserAnalyticsReport{
		User: &models.User{
			ID:          1,
			Username:    "\u0422\u0435\u0441\u0442",
			DisplayName: &displayName,
			Email:       "test@example.com",
			CreatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalMediaAccesses: 0,
		TotalEvents:        0,
	}

	content, err := service.formatAsHTML(report, "user_analytics")
	assert.NoError(t, err)
	assert.NotNil(t, content)
	assert.Contains(t, string(content), "<!DOCTYPE html>")
}

func TestReportingService_FormatAsMarkdown_EmptyUserAnalytics(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.UserAnalyticsReport{
		User: &models.User{
			ID:          0,
			Username:    "",
			DisplayName: nil,
			Email:       "",
			CreatedAt:   time.Time{},
		},
		StartDate:          time.Time{},
		EndDate:            time.Time{},
		TotalMediaAccesses: 0,
		TotalEvents:        0,
		MediaAccessLogs:    nil,
		Events:             nil,
	}

	content, err := service.formatAsMarkdown(report, "user_analytics")
	assert.NoError(t, err)
	assert.NotNil(t, content)
	assert.Contains(t, string(content), "# User Analytics Report")
	assert.Contains(t, string(content), "Total Media Accesses: 0")
	assert.Contains(t, string(content), "Total Events: 0")
}

func TestReportingService_FormatAsHTML_EmptySystemOverview(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.SystemOverviewReport{
		StartDate:          time.Time{},
		EndDate:            time.Time{},
		TotalUsers:         0,
		ActiveUsers:        0,
		TotalMediaAccesses: 0,
		TotalEvents:        0,
	}

	content, err := service.formatAsHTML(report, "system_overview")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "<!DOCTYPE html>")
	assert.Contains(t, contentStr, "Total Users: 0")
	assert.Contains(t, contentStr, "Active Users: 0")
	assert.Contains(t, contentStr, "Total Media Accesses: 0")
	assert.Contains(t, contentStr, "Total Events: 0")
}

func TestReportingService_FormatAsMarkdown_EmptySystemOverview(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.SystemOverviewReport{
		StartDate:          time.Time{},
		EndDate:            time.Time{},
		TotalUsers:         0,
		ActiveUsers:        0,
		TotalMediaAccesses: 0,
		TotalEvents:        0,
	}

	content, err := service.formatAsMarkdown(report, "system_overview")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "# System Overview Report")
	assert.Contains(t, contentStr, "Total Users: 0")
	assert.Contains(t, contentStr, "Active Users: 0")
}

func TestReportingService_FormatAsMarkdown_SecurityAuditFallback(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.SecurityAuditReport{
		StartDate:           time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:             time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		FailedLoginAttempts: 5,
		SuccessfulLogins:    100,
		SuspiciousActivity:  []models.SecurityIncident{},
	}

	content, err := service.formatAsMarkdown(report, "security_audit")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "# security_audit Report")
	assert.Contains(t, contentStr, "```json")
}

func TestReportingService_FormatAsMarkdown_PerformanceMetricsFallback(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.PerformanceMetricsReport{
		StartDate:              time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:                time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		AverageSessionDuration: 10 * time.Minute,
		TotalSessions:          50,
	}

	content, err := service.formatAsMarkdown(report, "performance_metrics")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "# performance_metrics Report")
	assert.Contains(t, contentStr, "```json")
}

func TestReportingService_FormatAsMarkdown_MediaAnalyticsFallback(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.MediaAnalyticsReport{
		MediaID:       42,
		StartDate:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:       time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalAccesses: 100,
		UniqueUsers:   25,
	}

	content, err := service.formatAsMarkdown(report, "media_analytics")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "# media_analytics Report")
	assert.Contains(t, contentStr, "```json")
	assert.Contains(t, contentStr, "42")
}

func TestReportingService_FormatAsMarkdown_UserActivityFallback(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.UserActivityReport{
		StartDate:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:       time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalUsers:    10,
		TotalAccesses: 500,
	}

	content, err := service.formatAsMarkdown(report, "user_activity")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "# user_activity Report")
	assert.Contains(t, contentStr, "```json")
}

func TestReportingService_FormatAsHTML_SecurityAuditFallback(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.SecurityAuditReport{
		StartDate:           time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:             time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		FailedLoginAttempts: 3,
		SuccessfulLogins:    50,
		SuspiciousActivity:  []models.SecurityIncident{},
	}

	content, err := service.formatAsHTML(report, "security_audit")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "<!DOCTYPE html>")
	assert.Contains(t, contentStr, "<pre>")
}

func TestReportingService_FormatAsHTML_MediaAnalyticsFallback(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.MediaAnalyticsReport{
		MediaID:       7,
		StartDate:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:       time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalAccesses: 200,
		UniqueUsers:   50,
	}

	content, err := service.formatAsHTML(report, "media_analytics")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "<!DOCTYPE html>")
	assert.Contains(t, contentStr, "<pre>")
}

func TestReportingService_FormatAsHTML_PerformanceMetricsFallback(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.PerformanceMetricsReport{
		StartDate:              time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:                time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		AverageSessionDuration: 5 * time.Minute,
		TotalSessions:          20,
	}

	content, err := service.formatAsHTML(report, "performance_metrics")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "<!DOCTYPE html>")
	assert.Contains(t, contentStr, "<pre>")
}

func TestReportingService_FormatAsHTML_UserActivityFallback(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.UserActivityReport{
		StartDate:     time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:       time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalUsers:    5,
		TotalAccesses: 100,
	}

	content, err := service.formatAsHTML(report, "user_activity")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "<!DOCTYPE html>")
	assert.Contains(t, contentStr, "<pre>")
}

// ===========================================================================
// formatReport via public API with typed report data
// ===========================================================================

func TestReportingService_FormatReport_Markdown_UserAnalytics(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	displayName := "Report User"
	report := &models.UserAnalyticsReport{
		User: &models.User{
			ID:          5,
			Username:    "reportuser",
			DisplayName: &displayName,
			Email:       "report@example.com",
			CreatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalMediaAccesses: 300,
		TotalEvents:        75,
	}

	content, err := service.formatReport(report, "markdown", "user_analytics")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "# User Analytics Report")
	assert.Contains(t, contentStr, "reportuser")
	assert.Contains(t, contentStr, "Total Media Accesses: 300")
}

func TestReportingService_FormatReport_HTML_UserAnalytics(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	displayName := "HTML User"
	report := &models.UserAnalyticsReport{
		User: &models.User{
			ID:          3,
			Username:    "htmluser",
			DisplayName: &displayName,
			Email:       "html@example.com",
			CreatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalMediaAccesses: 50,
		TotalEvents:        10,
	}

	content, err := service.formatReport(report, "html", "user_analytics")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "<!DOCTYPE html>")
	assert.Contains(t, contentStr, "htmluser")
	assert.Contains(t, contentStr, "Total Media Accesses: 50")
}

func TestReportingService_FormatReport_Markdown_SystemOverview(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.SystemOverviewReport{
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalUsers:         500,
		ActiveUsers:        400,
		TotalMediaAccesses: 25000,
		TotalEvents:        10000,
	}

	content, err := service.formatReport(report, "markdown", "system_overview")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "# System Overview Report")
	assert.Contains(t, contentStr, "Total Users: 500")
	assert.Contains(t, contentStr, "Active Users: 400")
}

func TestReportingService_FormatReport_HTML_SystemOverview(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.SystemOverviewReport{
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalUsers:         250,
		ActiveUsers:        200,
		TotalMediaAccesses: 12000,
		TotalEvents:        5000,
	}

	content, err := service.formatReport(report, "html", "system_overview")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "<!DOCTYPE html>")
	assert.Contains(t, contentStr, "Total Users: 250")
	assert.Contains(t, contentStr, "Total Media Accesses: 12000")
}

func TestReportingService_FormatReport_JSON_UserAnalytics(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	displayName := "JSON User"
	report := &models.UserAnalyticsReport{
		User: &models.User{
			ID:          10,
			Username:    "jsonuser",
			DisplayName: &displayName,
			Email:       "json@example.com",
			CreatedAt:   time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		},
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalMediaAccesses: 999,
		TotalEvents:        111,
	}

	content, err := service.formatReport(report, "json", "user_analytics")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "jsonuser")
	assert.Contains(t, contentStr, "999")
}

func TestReportingService_FormatReport_JSON_SystemOverview(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report := &models.SystemOverviewReport{
		StartDate:          time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:            time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC),
		TotalUsers:         1000,
		ActiveUsers:        750,
		TotalMediaAccesses: 50000,
		TotalEvents:        20000,
	}

	content, err := service.formatReport(report, "json", "system_overview")
	assert.NoError(t, err)
	assert.NotNil(t, content)

	contentStr := string(content)
	assert.Contains(t, contentStr, "1000")
	assert.Contains(t, contentStr, "50000")
}

// ===========================================================================
// GenerateReport: unsupported format error propagation
// ===========================================================================

func TestReportingService_GenerateReport_UnsupportedFormat(t *testing.T) {
	db := setupTestDB(t)
	_, err := db.Exec(authAuditDDL)
	assert.NoError(t, err)
	service := NewReportingService(nil, nil, db)

	report, err := service.GenerateReport("security_audit", "xml", map[string]interface{}{
		"start_date": "2025-01-01",
		"end_date":   "2025-01-31",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported format")
	assert.Nil(t, report)
}

func TestReportingService_GenerateReport_MediaAnalytics_MissingMediaID(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report, err := service.GenerateReport("media_analytics", "json", map[string]interface{}{
		"start_date": "2025-01-01",
		"end_date":   "2025-01-31",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "media_id parameter required")
	assert.Nil(t, report)
}

func TestReportingService_GenerateReport_UserActivity_MissingDates(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	report, err := service.GenerateReport("user_activity", "json", map[string]interface{}{})
	assert.Error(t, err)
	assert.Nil(t, report)
}

// ===========================================================================
// calculateSystemHealth: threshold boundary tests
// ===========================================================================

func TestReportingService_CalculateSystemHealth_ExcellentThreshold(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	// 100 total, 80 active => activeRatio=0.8 => 0.8*50=40, mediaAccesses>0 => +30, activeUsers>10 => +20 => 90 => excellent
	health := service.calculateSystemHealth(100, 80, 5000)
	assert.Equal(t, "excellent", health.Status)
	assert.GreaterOrEqual(t, health.Score, 80.0)
}

func TestReportingService_CalculateSystemHealth_GoodThreshold(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	// 100 total, 20 active => activeRatio=0.2 => 0.2*50=10, mediaAccesses>0 => +30, activeUsers>10 => +20 => 60 => good
	health := service.calculateSystemHealth(100, 20, 100)
	assert.Equal(t, "good", health.Status)
	assert.GreaterOrEqual(t, health.Score, 60.0)
	assert.Less(t, health.Score, 80.0)
}

func TestReportingService_CalculateSystemHealth_FairThreshold(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	// 100 total, 5 active => activeRatio=0.05 => 0.05*50=2.5, mediaAccesses>0 => +30, activeUsers<=10 => +0 => 32.5
	// Wait: 5 is not > 10, so no +20. Score = 2.5 + 30 = 32.5 => poor (< 40)
	// Let's get fair: need score >= 40 and < 60
	// 100 total, 10 active => 0.1*50=5, +30=35, 10 is not > 10 => 35 => poor
	// 100 total, 15 active, 100 accesses => 0.15*50=7.5, +30=37.5, 15>10 => +20 => 57.5 => fair? No, 57.5 < 60 => fair
	health := service.calculateSystemHealth(100, 15, 100)
	assert.Equal(t, "fair", health.Status)
	assert.GreaterOrEqual(t, health.Score, 40.0)
	assert.Less(t, health.Score, 60.0)
}

func TestReportingService_CalculateSystemHealth_PoorThreshold(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	// 0 total => no active ratio bonus, 0 accesses => no +30, 0 active => no +20 => 0 => poor
	health := service.calculateSystemHealth(0, 0, 0)
	assert.Equal(t, "poor", health.Status)
	assert.Less(t, health.Score, 40.0)
}

func TestReportingService_CalculateSystemHealth_ZeroTotalUsersNonZeroActive(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	// Edge: totalUsers=0 but activeUsers=5 (inconsistent data)
	health := service.calculateSystemHealth(0, 5, 100)
	assert.NotEmpty(t, health.Status)
}

// ===========================================================================
// Large data sets
// ===========================================================================

func TestReportingService_FormatReport_JSON_LargeData(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	largeData := make(map[string]interface{})
	for i := 0; i < 1000; i++ {
		largeData[fmt.Sprintf("key_%d", i)] = fmt.Sprintf("value_%d", i)
	}

	content, err := service.formatReport(largeData, "json", "generic")
	assert.NoError(t, err)
	assert.NotNil(t, content)
	assert.Greater(t, len(content), 10000) // Should be substantial output
}

func TestReportingService_FormatAsMarkdown_LargeData(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	largeData := make(map[string]interface{})
	for i := 0; i < 500; i++ {
		largeData[fmt.Sprintf("item_%d", i)] = fmt.Sprintf("data_%d", i)
	}

	content, err := service.formatAsMarkdown(largeData, "unknown_type")
	assert.NoError(t, err)
	assert.NotNil(t, content)
	assert.Contains(t, string(content), "# unknown_type Report")
	assert.Contains(t, string(content), "```json")
}

func TestReportingService_FormatAsHTML_LargeData(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	largeData := make(map[string]interface{})
	for i := 0; i < 500; i++ {
		largeData[fmt.Sprintf("field_%d", i)] = i
	}

	content, err := service.formatAsHTML(largeData, "unknown_type")
	assert.NoError(t, err)
	assert.NotNil(t, content)
	assert.Contains(t, string(content), "<!DOCTYPE html>")
	assert.Contains(t, string(content), "<pre>")
}

func TestReportingService_CountUniqueUsers_LargeDataSet(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	var logs []models.MediaAccessLog
	for i := 0; i < 10000; i++ {
		logs = append(logs, models.MediaAccessLog{
			UserID:     i % 100, // 100 unique users
			MediaID:    i % 50,
			AccessTime: time.Now(),
		})
	}

	count := service.countUniqueUsers(logs)
	assert.Equal(t, 100, count)
}

func TestReportingService_FilterLogsByDateRange_LargeDataSet(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	startDate := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)

	var logs []models.MediaAccessLog
	for i := 0; i < 1000; i++ {
		day := (i % 31) + 1
		logs = append(logs, models.MediaAccessLog{
			UserID:     i,
			MediaID:    i,
			AccessTime: time.Date(2025, 1, day, 12, 0, 0, 0, time.UTC),
		})
	}

	filtered := service.filterLogsByDateRange(logs, startDate, endDate)
	for _, log := range filtered {
		assert.True(t, log.AccessTime.After(startDate))
		assert.True(t, log.AccessTime.Before(endDate))
	}
}

// ===========================================================================
// filterLogsByDateRange: boundary conditions
// ===========================================================================

func TestReportingService_FilterLogsByDateRange_BoundaryExact(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	startDate := time.Date(2025, 1, 10, 0, 0, 0, 0, time.UTC)
	endDate := time.Date(2025, 1, 20, 0, 0, 0, 0, time.UTC)

	logs := []models.MediaAccessLog{
		{UserID: 1, MediaID: 1, AccessTime: startDate},                                     // exactly at start (not After)
		{UserID: 2, MediaID: 2, AccessTime: endDate},                                       // exactly at end (not Before)
		{UserID: 3, MediaID: 3, AccessTime: startDate.Add(1 * time.Second)},                // just after start
		{UserID: 4, MediaID: 4, AccessTime: endDate.Add(-1 * time.Second)},                 // just before end
		{UserID: 5, MediaID: 5, AccessTime: time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC)}, // middle
	}

	filtered := service.filterLogsByDateRange(logs, startDate, endDate)
	// filterLogsByDateRange uses strictly After(start) AND Before(end)
	// So exact start and exact end should be excluded
	assert.Equal(t, 3, len(filtered))
}

func TestReportingService_FilterLogsByDateRange_SameStartEnd(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	sameDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)

	logs := []models.MediaAccessLog{
		{UserID: 1, MediaID: 1, AccessTime: sameDate},
	}

	// When start == end, nothing can be both After(start) and Before(end) simultaneously
	filtered := service.filterLogsByDateRange(logs, sameDate, sameDate)
	assert.Equal(t, 0, len(filtered))
}

// ===========================================================================
// calculateUsageStatistics edge cases
// ===========================================================================

func TestReportingService_CalculateUsageStatistics_SameDay(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	sameDate := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	stats := service.calculateUsageStatistics(sameDate, sameDate)
	// With nil repository, returns empty stats
	assert.Empty(t, stats.PeakHours)
	assert.Equal(t, 0, stats.AverageDaily)
}

func TestReportingService_CalculateUsageStatistics_InvertedRange(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	// End before start -- days will be < 1, should use fallback of 1
	start := time.Date(2025, 6, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	stats := service.calculateUsageStatistics(start, end)
	// With nil repository, returns empty stats
	assert.Empty(t, stats.PeakHours)
	assert.Equal(t, 0, stats.AverageDaily)
}

// ===========================================================================
// calculateErrorRates edge cases
// ===========================================================================

func TestReportingService_CalculateErrorRates_SameDay(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	sameDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	result := service.calculateErrorRates(sameDate, sameDate)
	// With nil repository, returns zero values
	assert.Equal(t, 0.0, result.Total)
	assert.Equal(t, 0.0, result.HTTP4xx)
	assert.Equal(t, 0.0, result.HTTP5xx)
	assert.Equal(t, 0.0, result.Timeouts)
}

func TestReportingService_CalculateErrorRates_LongPeriod(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	start := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	result := service.calculateErrorRates(start, end)
	// With nil repository, returns zero values
	assert.Equal(t, 0.0, result.Total)
	assert.Equal(t, 0.0, result.HTTP4xx)
}

// ===========================================================================
// calculateSecurityMetrics edge cases
// ===========================================================================

func TestReportingService_CalculateSecurityMetrics_LongPeriod(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	start := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC)
	metrics := service.calculateSecurityMetrics(start, end)
	assert.Equal(t, "unknown", metrics.ThreatLevel)
	assert.Contains(t, metrics.Unavailable, "failed_logins")
}

func TestReportingService_CalculateSecurityMetrics_SameDay(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	sameDate := time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC)
	metrics := service.calculateSecurityMetrics(sameDate, sameDate)
	assert.Equal(t, "unknown", metrics.ThreatLevel)
	assert.Contains(t, metrics.Unavailable, "failed_logins")
}

// ===========================================================================
// analyzeTimeDistribution edge cases
// ===========================================================================

func TestReportingService_AnalyzeTimeDistribution_BoundaryHours(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	logs := []models.MediaAccessLog{
		{AccessTime: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},    // midnight = night
		{AccessTime: time.Date(2025, 1, 1, 5, 59, 59, 0, time.UTC)},  // just before morning = night
		{AccessTime: time.Date(2025, 1, 1, 6, 0, 0, 0, time.UTC)},    // morning boundary
		{AccessTime: time.Date(2025, 1, 1, 11, 59, 59, 0, time.UTC)}, // end of morning
		{AccessTime: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)},   // afternoon boundary
		{AccessTime: time.Date(2025, 1, 1, 17, 59, 59, 0, time.UTC)}, // end of afternoon
		{AccessTime: time.Date(2025, 1, 1, 18, 0, 0, 0, time.UTC)},   // evening boundary
		{AccessTime: time.Date(2025, 1, 1, 21, 59, 59, 0, time.UTC)}, // end of evening
		{AccessTime: time.Date(2025, 1, 1, 22, 0, 0, 0, time.UTC)},   // night boundary
		{AccessTime: time.Date(2025, 1, 1, 23, 59, 59, 0, time.UTC)}, // end of night
	}

	dist := service.analyzeTimeDistribution(logs)
	assert.Equal(t, 4, dist["night"])     // 0:00, 5:59, 22:00, 23:59
	assert.Equal(t, 2, dist["morning"])   // 6:00, 11:59
	assert.Equal(t, 2, dist["afternoon"]) // 12:00, 17:59
	assert.Equal(t, 2, dist["evening"])   // 18:00, 21:59
}

// ===========================================================================
// getMostActiveHour edge cases
// ===========================================================================

func TestReportingService_GetMostActiveHour_TiedHours(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	logs := []models.MediaAccessLog{
		{AccessTime: time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)},
		{AccessTime: time.Date(2025, 1, 1, 15, 0, 0, 0, time.UTC)},
	}

	hour := service.getMostActiveHour(logs)
	// With a tie, any valid hour is acceptable
	assert.True(t, hour == 10 || hour == 15)
}

// ===========================================================================
// analyzeUserPopularContent edge cases
// ===========================================================================

func TestReportingService_AnalyzeUserPopularContent_SingleMedia(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	logs := []models.MediaAccessLog{
		{UserID: 1, MediaID: 42, AccessTime: time.Now()},
		{UserID: 2, MediaID: 42, AccessTime: time.Now()},
		{UserID: 3, MediaID: 42, AccessTime: time.Now()},
	}

	result := service.analyzeUserPopularContent(logs)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, 42, result[0].MediaID)
	assert.Equal(t, 3, result[0].AccessCount)
}

// ===========================================================================
// generateActivitySummary edge cases
// ===========================================================================

func TestReportingService_GenerateActivitySummary_LargeDataSet(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	var activities []models.UserActivitySummary
	for i := 0; i < 1000; i++ {
		activities = append(activities, models.UserActivitySummary{
			TotalAccesses: 10,
		})
	}

	summary := service.generateActivitySummary(activities)
	assert.Equal(t, 1000, summary.TotalUsers)
	assert.Equal(t, 10000, summary.TotalAccesses)
	assert.Equal(t, 10.0, summary.AverageAccesses)
}

func TestReportingService_GenerateActivitySummary_UnevenDistribution(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	activities := []models.UserActivitySummary{
		{TotalAccesses: 0},
		{TotalAccesses: 0},
		{TotalAccesses: 0},
		{TotalAccesses: 100},
	}

	summary := service.generateActivitySummary(activities)
	assert.Equal(t, 4, summary.TotalUsers)
	assert.Equal(t, 100, summary.TotalAccesses)
	assert.Equal(t, 25.0, summary.AverageAccesses)
}

// ===========================================================================
// calculateAverageSessionDuration edge cases
// ===========================================================================

func TestReportingService_CalculateAverageSessionDuration_VeryLongSessions(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	sessions := []models.SessionData{
		{Duration: 24 * time.Hour},
		{Duration: 48 * time.Hour},
	}

	avg := service.calculateAverageSessionDuration(sessions)
	assert.Equal(t, 36*time.Hour, avg)
}

func TestReportingService_CalculateAverageSessionDuration_ZeroDuration(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	sessions := []models.SessionData{
		{Duration: 0},
		{Duration: 0},
	}

	avg := service.calculateAverageSessionDuration(sessions)
	assert.Equal(t, time.Duration(0), avg)
}

// ===========================================================================
// FormatReport with nil/empty data
// ===========================================================================

func TestReportingService_FormatReport_JSON_NilData(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	content, err := service.formatReport(nil, "json", "generic")
	assert.NoError(t, err)
	assert.Equal(t, "null", string(content))
}

func TestReportingService_FormatAsMarkdown_NilData(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	content, err := service.formatAsMarkdown(nil, "unknown_type")
	assert.NoError(t, err)
	assert.NotNil(t, content)
	assert.Contains(t, string(content), "# unknown_type Report")
}

func TestReportingService_FormatAsHTML_NilData(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	content, err := service.formatAsHTML(nil, "unknown_type")
	assert.NoError(t, err)
	assert.NotNil(t, content)
	assert.Contains(t, string(content), "<!DOCTYPE html>")
}

// ===========================================================================
// getLastActivityTime edge cases
// ===========================================================================

func TestReportingService_GetLastActivityTime_UnorderedLogs(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	logs := []models.MediaAccessLog{
		{UserID: 1, MediaID: 1, AccessTime: time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)},
		{UserID: 1, MediaID: 2, AccessTime: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)},
		{UserID: 1, MediaID: 3, AccessTime: time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)},
		{UserID: 1, MediaID: 4, AccessTime: time.Date(2025, 3, 10, 8, 0, 0, 0, time.UTC)},
	}

	result := service.getLastActivityTime(logs)
	assert.Equal(t, time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC), result)
}

// ===========================================================================
// analyzeUserDeviceUsage edge cases
// ===========================================================================

func TestReportingService_AnalyzeUserDeviceUsage_NilFields(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	logs := []models.MediaAccessLog{
		{UserID: 1, MediaID: 1, AccessTime: time.Now(), DeviceInfo: &models.DeviceInfo{Platform: nil, DeviceModel: nil}},
	}

	result := service.analyzeUserDeviceUsage(logs)
	// Should still count the device, with empty strings
	assert.Equal(t, 1, len(result))
	assert.Equal(t, 1, result[" "])
}

func TestReportingService_AnalyzeUserDeviceUsage_MixedNilAndValid(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	platform := "Web"
	model := "Chrome"

	logs := []models.MediaAccessLog{
		{UserID: 1, MediaID: 1, AccessTime: time.Now(), DeviceInfo: nil},
		{UserID: 2, MediaID: 2, AccessTime: time.Now(), DeviceInfo: &models.DeviceInfo{Platform: &platform, DeviceModel: &model}},
		{UserID: 3, MediaID: 3, AccessTime: time.Now(), DeviceInfo: nil},
	}

	result := service.analyzeUserDeviceUsage(logs)
	assert.Equal(t, 1, len(result))
	assert.Equal(t, 1, result["Web Chrome"])
}

// ===========================================================================
// analyzeUserLocations edge cases
// ===========================================================================

func TestReportingService_AnalyzeUserLocations_DuplicateLocations(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	logs := []models.MediaAccessLog{
		{UserID: 1, MediaID: 1, AccessTime: time.Now(), Location: &models.Location{Latitude: 40.71, Longitude: -74.00}},
		{UserID: 2, MediaID: 2, AccessTime: time.Now(), Location: &models.Location{Latitude: 40.71, Longitude: -74.00}},
		{UserID: 3, MediaID: 3, AccessTime: time.Now(), Location: &models.Location{Latitude: 51.51, Longitude: -0.13}},
	}

	result := service.analyzeUserLocations(logs)
	assert.Equal(t, 2, len(result))
	assert.Equal(t, 2, result["40.71,-74.00"])
	assert.Equal(t, 1, result["51.51,-0.13"])
}

// ===========================================================================
// extractDateRange edge cases
// ===========================================================================

func TestReportingService_ExtractDateRange_BothNonString(t *testing.T) {
	// Article XI §11.5: this test originally asserted that an int
	// param produced "start_date parameter required" — that was a
	// vague catch-all message from the prior implementation that
	// didn't distinguish "missing" from "wrong type". The new
	// paramAsDate (added 2026-04-29 alongside the fix for
	// FQA-API-277/278 type mismatch) returns a precise
	// "invalid start_date type <T>" message instead. Both are
	// errors; the new message is strictly more useful for
	// debugging.
	service := NewReportingService(nil, nil, nil)

	_, _, err := service.extractDateRange(map[string]interface{}{
		"start_date": 20250101,
		"end_date":   20250131,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid start_date type int",
		"non-string non-time.Time value must surface its actual type so the caller can fix the wrong-type plumbing, not assume missing")
}

func TestReportingService_ExtractDateRange_NilParams(t *testing.T) {
	service := NewReportingService(nil, nil, nil)

	_, _, err := service.extractDateRange(nil)
	assert.Error(t, err)
}
