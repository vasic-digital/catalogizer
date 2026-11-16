package services

import (
	"bytes"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"catalogizer/internal/logging"
	"catalogizer/models"
	"catalogizer/repository"
)

// ErrReportNotFound is returned when a report does not exist OR belongs to another user. Both cases are one error on
// purpose so that no caller can tell them apart (no id-existence oracle).
var ErrReportNotFound = errors.New("report not found")

type ErrorReportingService struct {
	errorRepo        *repository.ErrorReportingRepository
	crashRepo        *repository.CrashReportingRepository
	config           *ErrorReportingConfig
	httpClient       *http.Client
	enabled          bool
	crashlyticAPIKey string
	wg               sync.WaitGroup
}

type ErrorReportingConfig struct {
	CrashlyticsEnabled  bool   `json:"crashlytics_enabled"`
	CrashlyticsAPIKey   string `json:"crashlytics_api_key"`
	SlackWebhookURL     string `json:"slack_webhook_url"`
	EmailNotifications  bool   `json:"email_notifications"`
	SentryDSN           string `json:"sentry_dsn"`
	AutoReporting       bool   `json:"auto_reporting"`
	MaxErrorsPerHour    int    `json:"max_errors_per_hour"`
	RetentionDays       int    `json:"retention_days"`
	IncludeStackTrace   bool   `json:"include_stack_trace"`
	IncludeSystemInfo   bool   `json:"include_system_info"`
	FilterSensitiveData bool   `json:"filter_sensitive_data"`
}

func NewErrorReportingService(errorRepo *repository.ErrorReportingRepository, crashRepo *repository.CrashReportingRepository) *ErrorReportingService {
	config := &ErrorReportingConfig{
		CrashlyticsEnabled:  false,
		EmailNotifications:  true,
		AutoReporting:       true,
		MaxErrorsPerHour:    100,
		RetentionDays:       30,
		IncludeStackTrace:   true,
		IncludeSystemInfo:   true,
		FilterSensitiveData: true,
	}

	return &ErrorReportingService{
		errorRepo:  errorRepo,
		crashRepo:  crashRepo,
		config:     config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		enabled:    true,
	}
}

// Close waits for all in-flight notification and external service goroutines to finish.
func (s *ErrorReportingService) Close() {
	s.wg.Wait()
}

func (s *ErrorReportingService) ReportError(userID int, errorReport *models.ErrorReportRequest) (*models.ErrorReport, error) {
	if !s.enabled {
		return nil, fmt.Errorf("error reporting is disabled")
	}

	// Check rate limiting
	if s.config.MaxErrorsPerHour > 0 {
		count, err := s.errorRepo.GetErrorCountInLastHour(userID)
		if err == nil && count >= s.config.MaxErrorsPerHour {
			return nil, fmt.Errorf("error reporting rate limit exceeded")
		}
	}

	// Create error report
	report := &models.ErrorReport{
		UserID:     userID,
		Level:      errorReport.Level,
		Message:    errorReport.Message,
		ErrorCode:  errorReport.ErrorCode,
		Component:  errorReport.Component,
		StackTrace: errorReport.StackTrace,
		Context:    errorReport.Context,
		UserAgent:  errorReport.UserAgent,
		URL:        errorReport.URL,
		ReportedAt: time.Now(),
		Status:     models.ErrorStatusNew,
	}

	// Filter sensitive data
	if s.config.FilterSensitiveData {
		report = s.filterSensitiveData(report)
	}

	// Add system information
	if s.config.IncludeSystemInfo {
		report.SystemInfo = s.collectSystemInfo()
	}

	// Generate fingerprint for deduplication
	report.Fingerprint = s.generateFingerprint(report)

	// Save to database
	if err := s.errorRepo.CreateErrorReport(report); err != nil {
		return nil, fmt.Errorf("failed to save error report: %w", err)
	}

	// Send notifications asynchronously
	if s.config.AutoReporting {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.sendNotifications(report)
		}()
	}

	// Send to external services
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.sendToExternalServices(report)
	}()

	return report, nil
}

func (s *ErrorReportingService) ReportCrash(userID int, crashReport *models.CrashReportRequest) (*models.CrashReport, error) {
	if !s.enabled {
		return nil, fmt.Errorf("crash reporting is disabled")
	}

	// Create crash report
	report := &models.CrashReport{
		UserID:     userID,
		Signal:     crashReport.Signal,
		Message:    crashReport.Message,
		StackTrace: crashReport.StackTrace,
		Context:    crashReport.Context,
		ReportedAt: time.Now(),
		Status:     models.CrashStatusNew,
	}

	// Add system information
	report.SystemInfo = s.collectSystemInfo()

	// Generate fingerprint for deduplication
	report.Fingerprint = s.generateCrashFingerprint(report)

	// Save to database
	if err := s.crashRepo.CreateCrashReport(report); err != nil {
		return nil, fmt.Errorf("failed to save crash report: %w", err)
	}

	// Send critical notifications immediately
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.sendCrashNotifications(report)
	}()

	// Send to Crashlytics
	if s.config.CrashlyticsEnabled {
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.sendToCrashlytics(report)
		}()
	}

	return report, nil
}

func (s *ErrorReportingService) GetErrorReport(id int, userID int) (*models.ErrorReport, error) {
	report, err := s.errorRepo.GetErrorReport(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get error report: %w", err)
	}

	// Check if user has access
	if report.UserID != userID {
		return nil, fmt.Errorf("access denied")
	}

	return report, nil
}

func (s *ErrorReportingService) GetCrashReport(id int, userID int) (*models.CrashReport, error) {
	report, err := s.crashRepo.GetCrashReport(id)
	if err != nil {
		return nil, fmt.Errorf("failed to get crash report: %w", err)
	}

	// Check if user has access
	if report.UserID != userID {
		return nil, fmt.Errorf("access denied")
	}

	return report, nil
}

func (s *ErrorReportingService) UpdateErrorStatus(id int, userID int, status string) error {
	report, err := s.errorRepo.GetErrorReport(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrReportNotFound
		}
		return fmt.Errorf("failed to get error report: %w", err)
	}

	if report.UserID != userID {
		return ErrReportNotFound
	}

	report.Status = status
	if status == models.ErrorStatusResolved {
		now := time.Now()
		report.ResolvedAt = &now
	}

	return s.errorRepo.UpdateErrorReport(report)
}

func (s *ErrorReportingService) UpdateCrashStatus(id int, userID int, status string) error {
	report, err := s.crashRepo.GetCrashReport(id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrReportNotFound
		}
		return fmt.Errorf("failed to get crash report: %w", err)
	}

	if report.UserID != userID {
		return ErrReportNotFound
	}

	report.Status = status
	if status == models.CrashStatusResolved {
		now := time.Now()
		report.ResolvedAt = &now
	}

	return s.crashRepo.UpdateCrashReport(report)
}

func (s *ErrorReportingService) GetErrorReportsByUser(userID int, filters *models.ErrorReportFilters) ([]*models.ErrorReport, error) {
	return s.errorRepo.GetErrorReportsByUser(userID, filters)
}

func (s *ErrorReportingService) GetCrashReportsByUser(userID int, filters *models.CrashReportFilters) ([]*models.CrashReport, error) {
	return s.crashRepo.GetCrashReportsByUser(userID, filters)
}

func (s *ErrorReportingService) GetErrorStatistics(userID int) (*models.ErrorStatistics, error) {
	return s.errorRepo.GetErrorStatistics(userID)
}

func (s *ErrorReportingService) GetCrashStatistics(userID int) (*models.CrashStatistics, error) {
	return s.crashRepo.GetCrashStatistics(userID)
}

func (s *ErrorReportingService) GetSystemHealth() (*models.SystemHealth, error) {
	health := &models.SystemHealth{
		CheckedAt: time.Now(),
		Status:    "healthy",
		Metrics:   make(map[string]interface{}),
	}

	// Check recent error rates
	recentErrors, err := s.errorRepo.GetRecentErrorCount(1 * time.Hour)
	if err == nil {
		health.Metrics["recent_errors"] = recentErrors
		if recentErrors > 100 {
			health.Status = "degraded"
		}
	}

	// Check recent crash rates
	recentCrashes, err := s.crashRepo.GetRecentCrashCount(1 * time.Hour)
	if err == nil {
		health.Metrics["recent_crashes"] = recentCrashes
		if recentCrashes > 5 {
			health.Status = "critical"
		}
	}

	// Check system resources
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	health.Metrics["memory_used"] = m.Alloc
	health.Metrics["memory_total"] = m.Sys
	health.Metrics["goroutines"] = runtime.NumGoroutine()

	return health, nil
}

func (s *ErrorReportingService) UpdateConfiguration(config *ErrorReportingConfig) error {
	s.config = config
	return nil
}

func (s *ErrorReportingService) GetConfiguration() *ErrorReportingConfig {
	return s.config
}

func (s *ErrorReportingService) CleanupOldReports(olderThan time.Time) error {
	if err := s.errorRepo.CleanupOldReports(olderThan); err != nil {
		return fmt.Errorf("failed to cleanup old error reports: %w", err)
	}

	if err := s.crashRepo.CleanupOldReports(olderThan); err != nil {
		return fmt.Errorf("failed to cleanup old crash reports: %w", err)
	}

	return nil
}

func (s *ErrorReportingService) ExportReports(userID int, filters *models.ExportFilters) ([]byte, error) {
	var reports []interface{}

	// Get error reports
	if filters.IncludeErrors {
		errorReports, err := s.errorRepo.GetErrorReportsByUser(userID, &models.ErrorReportFilters{
			StartDate: filters.StartDate,
			EndDate:   filters.EndDate,
			Level:     filters.Level,
			Component: filters.Component,
			Limit:     filters.Limit,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get error reports: %w", err)
		}
		for _, report := range errorReports {
			reports = append(reports, report)
		}
	}

	// Get crash reports
	if filters.IncludeCrashes {
		crashReports, err := s.crashRepo.GetCrashReportsByUser(userID, &models.CrashReportFilters{
			StartDate: filters.StartDate,
			EndDate:   filters.EndDate,
			Signal:    filters.Signal,
			Limit:     filters.Limit,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get crash reports: %w", err)
		}
		for _, report := range crashReports {
			reports = append(reports, report)
		}
	}

	// Export based on format
	switch filters.Format {
	case "json":
		return json.MarshalIndent(reports, "", "  ")
	case "csv":
		return s.exportToCSV(reports)
	default:
		return nil, fmt.Errorf("unsupported export format: %s", filters.Format)
	}
}

// Helper methods

func (s *ErrorReportingService) filterSensitiveData(report *models.ErrorReport) *models.ErrorReport {
	// Remove sensitive patterns from message and stack trace
	sensitivePatterns := []string{
		"password", "token", "key", "secret", "auth",
		"email", "phone", "ssn", "credit",
	}

	for _, pattern := range sensitivePatterns {
		report.Message = strings.ReplaceAll(strings.ToLower(report.Message), pattern, "[REDACTED]")
		report.StackTrace = strings.ReplaceAll(strings.ToLower(report.StackTrace), pattern, "[REDACTED]")
	}

	// Filter context data
	if report.Context != nil {
		filteredContext := make(map[string]interface{})
		for key, value := range report.Context {
			keyLower := strings.ToLower(key)
			isSensitive := false
			for _, pattern := range sensitivePatterns {
				if strings.Contains(keyLower, pattern) {
					isSensitive = true
					break
				}
			}
			if !isSensitive {
				filteredContext[key] = value
			} else {
				filteredContext[key] = "[REDACTED]"
			}
		}
		report.Context = filteredContext
	}

	return report
}

func (s *ErrorReportingService) collectSystemInfo() map[string]interface{} {
	info := make(map[string]interface{})

	info["os"] = runtime.GOOS
	info["arch"] = runtime.GOARCH
	info["go_version"] = runtime.Version()
	info["num_cpu"] = runtime.NumCPU()
	info["num_goroutine"] = runtime.NumGoroutine()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	info["memory_alloc"] = m.Alloc
	info["memory_total_alloc"] = m.TotalAlloc
	info["memory_sys"] = m.Sys

	if hostname, err := os.Hostname(); err == nil {
		info["hostname"] = hostname
	}

	if wd, err := os.Getwd(); err == nil {
		info["working_directory"] = wd
	}

	return info
}

func (s *ErrorReportingService) generateFingerprint(report *models.ErrorReport) string {
	// Article XI §11.5: a minimal payload (level only, no component
	// or error_code) hex-encodes to fewer than 16 chars and the
	// `[:16]` slice panicked with "slice bounds out of range
	// [:16] with length 14". The gin recovery middleware swallowed
	// the panic into HTTP 500 with empty body — exactly the empty-
	// 500 §11.9 forbids. Caught by FQA-API-271 in the 2026-04-29
	// real-binary bank verification.
	data := fmt.Sprintf("%s:%s:%s", report.Level, report.Component, report.ErrorCode)
	return truncateOrPad(fmt.Sprintf("%x", data), 16)
}

func (s *ErrorReportingService) generateCrashFingerprint(report *models.CrashReport) string {
	// Same panic class as generateFingerprint above — short signal
	// + empty message hex-encodes to <16 chars.
	data := fmt.Sprintf("%s:%s", report.Signal, report.Message)
	return truncateOrPad(fmt.Sprintf("%x", data), 16)
}

// truncateOrPad returns the first n bytes of s, padding with '0'
// if s is shorter than n. Safe for arbitrarily short input — never
// panics with slice-bounds-out-of-range. Used by fingerprint
// generators that must produce a fixed-width identifier.
func truncateOrPad(s string, n int) string {
	if len(s) >= n {
		return s[:n]
	}
	out := make([]byte, n)
	copy(out, s)
	for i := len(s); i < n; i++ {
		out[i] = '0'
	}
	return string(out)
}

func (s *ErrorReportingService) sendNotifications(report *models.ErrorReport) {
	// Send Slack notification
	if s.config.SlackWebhookURL != "" {
		s.sendSlackNotification(report)
	}

	// Send email notification
	if s.config.EmailNotifications {
		s.sendEmailNotification(report)
	}
}

func (s *ErrorReportingService) sendCrashNotifications(report *models.CrashReport) {
	// Send critical notifications for crashes
	if s.config.SlackWebhookURL != "" {
		s.sendSlackCrashNotification(report)
	}

	if s.config.EmailNotifications {
		s.sendEmailCrashNotification(report)
	}
}

func (s *ErrorReportingService) sendSlackNotification(report *models.ErrorReport) error {
	if s.config.SlackWebhookURL == "" {
		return nil
	}

	message := map[string]interface{}{
		"text": fmt.Sprintf("🚨 Error Report: %s", report.Message),
		"attachments": []map[string]interface{}{
			{
				"color": s.getColorForLevel(report.Level),
				"fields": []map[string]interface{}{
					{"title": "Level", "value": report.Level, "short": true},
					{"title": "Component", "value": report.Component, "short": true},
					{"title": "Error Code", "value": report.ErrorCode, "short": true},
					{"title": "Time", "value": report.ReportedAt.Format(time.RFC3339), "short": true},
				},
			},
		},
	}

	jsonData, _ := json.Marshal(message)
	_, err := s.httpClient.Post(s.config.SlackWebhookURL, "application/json", bytes.NewBuffer(jsonData))
	return err
}

func (s *ErrorReportingService) sendSlackCrashNotification(report *models.CrashReport) error {
	if s.config.SlackWebhookURL == "" {
		return nil
	}

	message := map[string]interface{}{
		"text": fmt.Sprintf("💥 CRASH REPORT: %s", report.Message),
		"attachments": []map[string]interface{}{
			{
				"color": "danger",
				"fields": []map[string]interface{}{
					{"title": "Signal", "value": report.Signal, "short": true},
					{"title": "Time", "value": report.ReportedAt.Format(time.RFC3339), "short": true},
				},
			},
		},
	}

	jsonData, _ := json.Marshal(message)
	_, err := s.httpClient.Post(s.config.SlackWebhookURL, "application/json", bytes.NewBuffer(jsonData))
	return err
}

// Log-based notification: upgrade to SMTP by implementing EmailSender interface
func (s *ErrorReportingService) sendEmailNotification(report *models.ErrorReport) error {
	subject := fmt.Sprintf("[%s] Error in %s: %s", report.Level, report.Component, report.ErrorCode)
	bodySummary := report.Message
	if len(bodySummary) > 200 {
		bodySummary = bodySummary[:200] + "..."
	}
	if logging.Logger != nil {
		logging.With(
			logging.String("subject", subject),
			logging.String("body", bodySummary),
			logging.String("reported_at", report.ReportedAt.Format(time.RFC3339)),
			logging.String("fingerprint", report.Fingerprint),
		).Info("Email notification sent for error report")
	}
	return nil
}

// Log-based notification: upgrade to SMTP by implementing EmailSender interface
func (s *ErrorReportingService) sendEmailCrashNotification(report *models.CrashReport) error {
	subject := fmt.Sprintf("[CRASH] Signal: %s", report.Signal)
	bodySummary := report.Message
	if len(bodySummary) > 200 {
		bodySummary = bodySummary[:200] + "..."
	}
	if logging.Logger != nil {
		logging.With(
			logging.String("subject", subject),
			logging.String("body", bodySummary),
			logging.String("reported_at", report.ReportedAt.Format(time.RFC3339)),
			logging.String("fingerprint", report.Fingerprint),
		).Info("Email notification sent for crash report")
	}
	return nil
}

func (s *ErrorReportingService) sendToExternalServices(report *models.ErrorReport) {
	// Send to Sentry
	if s.config.SentryDSN != "" {
		s.sendToSentry(report)
	}
}

func (s *ErrorReportingService) sendToSentry(report *models.ErrorReport) error {
	if s.config.SentryDSN == "" || s.httpClient == nil {
		return nil
	}

	event := map[string]interface{}{
		"event_id":  fmt.Sprintf("%x", report.ReportedAt.UnixNano()),
		"timestamp": report.ReportedAt.UTC().Format(time.RFC3339),
		"level":     strings.ToLower(report.Level),
		"logger":    "catalogizer",
		"platform":  "go",
		"message": map[string]interface{}{
			"formatted": report.Message,
		},
		"tags": map[string]string{
			"component":  report.Component,
			"error_code": report.ErrorCode,
		},
	}
	if report.StackTrace != "" {
		event["extra"] = map[string]interface{}{
			"stack_trace": report.StackTrace,
		}
	}
	if report.SystemInfo != nil {
		event["contexts"] = map[string]interface{}{
			"os": report.SystemInfo,
		}
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal sentry event: %w", err)
	}

	resp, err := s.httpClient.Post(s.config.SentryDSN, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send to sentry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("sentry returned status %d", resp.StatusCode)
	}

	return nil
}

func (s *ErrorReportingService) sendToCrashlytics(report *models.CrashReport) error {
	if s.config.CrashlyticsAPIKey == "" || s.httpClient == nil {
		return nil
	}

	event := map[string]interface{}{
		"event_id":    fmt.Sprintf("%x", report.ReportedAt.UnixNano()),
		"timestamp":   report.ReportedAt.UTC().Format(time.RFC3339),
		"signal":      report.Signal,
		"message":     report.Message,
		"stack_trace": report.StackTrace,
		"fingerprint": report.Fingerprint,
	}
	if report.SystemInfo != nil {
		event["system_info"] = report.SystemInfo
	}

	jsonData, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal crashlytics event: %w", err)
	}

	req, err := http.NewRequest("POST", "https://firebase-crashlytics.googleapis.com/v1/events", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create crashlytics request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.config.CrashlyticsAPIKey)

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send to crashlytics: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("crashlytics returned status %d", resp.StatusCode)
	}

	return nil
}

func (s *ErrorReportingService) getColorForLevel(level string) string {
	switch strings.ToLower(level) {
	case "error", "fatal":
		return "danger"
	case "warning", "warn":
		return "warning"
	case "info":
		return "good"
	default:
		return "#36a64f"
	}
}

func (s *ErrorReportingService) exportToCSV(reports []interface{}) ([]byte, error) {
	if len(reports) == 0 {
		return []byte(""), nil
	}

	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Determine report type and write appropriate header
	firstReport := reports[0]

	switch firstReport.(type) {
	case *models.ErrorReport:
		// Write header for error reports
		header := []string{
			"ID", "User ID", "Level", "Message", "Error Code", "Component",
			"Stack Trace", "User Agent", "URL", "Fingerprint", "Status",
			"Reported At", "Resolved At",
		}
		if err := writer.Write(header); err != nil {
			return nil, fmt.Errorf("failed to write CSV header: %w", err)
		}

		// Write error report data
		for _, r := range reports {
			errorReport, ok := r.(*models.ErrorReport)
			if !ok {
				continue
			}

			resolvedAt := ""
			if errorReport.ResolvedAt != nil {
				resolvedAt = errorReport.ResolvedAt.Format(time.RFC3339)
			}

			// Escape special characters in text fields
			row := []string{
				strconv.Itoa(errorReport.ID),
				strconv.Itoa(errorReport.UserID),
				errorReport.Level,
				escapeCSVField(errorReport.Message),
				errorReport.ErrorCode,
				errorReport.Component,
				escapeCSVField(errorReport.StackTrace),
				errorReport.UserAgent,
				errorReport.URL,
				errorReport.Fingerprint,
				errorReport.Status,
				errorReport.ReportedAt.Format(time.RFC3339),
				resolvedAt,
			}
			if err := writer.Write(row); err != nil {
				return nil, fmt.Errorf("failed to write CSV row: %w", err)
			}
		}

	case *models.CrashReport:
		// Write header for crash reports
		header := []string{
			"ID", "User ID", "Signal", "Message", "Stack Trace",
			"Fingerprint", "Status", "Reported At", "Resolved At",
		}
		if err := writer.Write(header); err != nil {
			return nil, fmt.Errorf("failed to write CSV header: %w", err)
		}

		// Write crash report data
		for _, r := range reports {
			crashReport, ok := r.(*models.CrashReport)
			if !ok {
				continue
			}

			resolvedAt := ""
			if crashReport.ResolvedAt != nil {
				resolvedAt = crashReport.ResolvedAt.Format(time.RFC3339)
			}

			row := []string{
				strconv.Itoa(crashReport.ID),
				strconv.Itoa(crashReport.UserID),
				crashReport.Signal,
				escapeCSVField(crashReport.Message),
				escapeCSVField(crashReport.StackTrace),
				crashReport.Fingerprint,
				crashReport.Status,
				crashReport.ReportedAt.Format(time.RFC3339),
				resolvedAt,
			}
			if err := writer.Write(row); err != nil {
				return nil, fmt.Errorf("failed to write CSV row: %w", err)
			}
		}

	default:
		// Handle mixed report types - use a combined format
		header := []string{
			"Type", "ID", "User ID", "Level/Signal", "Message", "Error Code", "Component",
			"Stack Trace", "User Agent", "URL", "Fingerprint", "Status",
			"Reported At", "Resolved At",
		}
		if err := writer.Write(header); err != nil {
			return nil, fmt.Errorf("failed to write CSV header: %w", err)
		}

		for _, r := range reports {
			var row []string

			switch report := r.(type) {
			case *models.ErrorReport:
				resolvedAt := ""
				if report.ResolvedAt != nil {
					resolvedAt = report.ResolvedAt.Format(time.RFC3339)
				}
				row = []string{
					"error",
					strconv.Itoa(report.ID),
					strconv.Itoa(report.UserID),
					report.Level,
					escapeCSVField(report.Message),
					report.ErrorCode,
					report.Component,
					escapeCSVField(report.StackTrace),
					report.UserAgent,
					report.URL,
					report.Fingerprint,
					report.Status,
					report.ReportedAt.Format(time.RFC3339),
					resolvedAt,
				}
			case *models.CrashReport:
				resolvedAt := ""
				if report.ResolvedAt != nil {
					resolvedAt = report.ResolvedAt.Format(time.RFC3339)
				}
				row = []string{
					"crash",
					strconv.Itoa(report.ID),
					strconv.Itoa(report.UserID),
					report.Signal,
					escapeCSVField(report.Message),
					"", // Error Code (N/A for crash)
					"", // Component (N/A for crash)
					escapeCSVField(report.StackTrace),
					"", // User Agent (N/A for crash)
					"", // URL (N/A for crash)
					report.Fingerprint,
					report.Status,
					report.ReportedAt.Format(time.RFC3339),
					resolvedAt,
				}
			default:
				continue
			}

			if err := writer.Write(row); err != nil {
				return nil, fmt.Errorf("failed to write CSV row: %w", err)
			}
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, fmt.Errorf("CSV writer error: %w", err)
	}

	return buf.Bytes(), nil
}

// escapeCSVField handles special characters in CSV fields
// Newlines and other control characters are replaced with spaces to ensure valid CSV
func escapeCSVField(s string) string {
	// Replace newlines with spaces (CSV spec allows this within quoted fields,
	// but many parsers have issues with it)
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	// Replace tabs with spaces
	s = strings.ReplaceAll(s, "\t", " ")
	return s
}
