package handlers

import (
	"catalogizer/models"
	"catalogizer/services"
	"sync"
	"time"
)

// AuthServiceAdapter adapts *services.AuthService to satisfy the various auth interfaces
// used by UserHandler, RoleHandler, ErrorReportingHandler, and LogManagementHandler.
type AuthServiceAdapter struct {
	Inner *services.AuthService
}

// CheckPermission satisfies UserAuthServiceInterface, RoleAuthServiceInterface,
// ErrorReportingAuthServiceInterface, LogManagementAuthServiceInterface
func (a *AuthServiceAdapter) CheckPermission(userID int, permission string) (bool, error) {
	return a.Inner.CheckPermission(userID, permission)
}

// RecordAuditEvent forwards to the audit trail of the wrapped service (see recordAuth).
func (a *AuthServiceAdapter) RecordAuditEvent(userID int, eventType, ipAddress, userAgent, details string) {
	a.Inner.RecordAuditEvent(userID, eventType, ipAddress, userAgent, details)
}

// GetCurrentUser satisfies UserAuthServiceInterface
func (a *AuthServiceAdapter) GetCurrentUser(token string) (*models.User, error) {
	return a.Inner.GetCurrentUser(token)
}

// HashPassword satisfies UserAuthServiceInterface by delegating to HashPasswordForUser
func (a *AuthServiceAdapter) HashPassword(password string) (string, error) {
	hash, _, err := a.Inner.HashPasswordForUser(password)
	return hash, err
}

// ValidatePassword satisfies UserAuthServiceInterface
func (a *AuthServiceAdapter) ValidatePassword(password string) error {
	return a.Inner.ValidatePassword(password)
}

// GenerateSecureToken satisfies UserAuthServiceInterface
func (a *AuthServiceAdapter) GenerateSecureToken(length int) (string, error) {
	return a.Inner.GenerateSecureToken(length)
}

// ResetPassword satisfies UserAuthServiceInterface
func (a *AuthServiceAdapter) ResetPassword(userID int, newPassword string) error {
	return a.Inner.ResetPassword(userID, newPassword)
}

// LockAccount satisfies UserAuthServiceInterface
func (a *AuthServiceAdapter) LockAccount(userID int, lockUntil time.Time) error {
	return a.Inner.LockAccount(userID, lockUntil)
}

// UnlockAccount satisfies UserAuthServiceInterface
func (a *AuthServiceAdapter) UnlockAccount(userID int) error {
	return a.Inner.UnlockAccount(userID)
}

// ValidateToken satisfies ConfigurationAuthServiceInterface
func (a *AuthServiceAdapter) ValidateToken(tokenString string) (*models.User, error) {
	return a.Inner.GetCurrentUser(tokenString)
}

// ConfigurationServiceAdapter adapts *services.ConfigurationService
type ConfigurationServiceAdapter struct {
	Inner *services.ConfigurationService
}

func (a *ConfigurationServiceAdapter) GetWizardStep(stepID string) (*models.WizardStep, error) {
	return a.Inner.GetWizardStep(stepID)
}

func (a *ConfigurationServiceAdapter) ValidateWizardStep(stepID string, data map[string]interface{}) (*models.ValidationResult, error) {
	result, err := a.Inner.ValidateWizardStep(stepID, data)
	if err != nil {
		return nil, err
	}
	// Convert map-based errors/warnings to slices
	var errs []string
	for k, v := range result.Errors {
		errs = append(errs, k+": "+v)
	}
	var warns []string
	for k, v := range result.Warnings {
		warns = append(warns, k+": "+v)
	}
	return &models.ValidationResult{
		IsValid:  result.Valid,
		Errors:   errs,
		Warnings: warns,
	}, nil
}

func (a *ConfigurationServiceAdapter) SaveWizardProgress(userID int, stepID string, data map[string]interface{}) error {
	return a.Inner.SaveWizardProgress(userID, stepID, data)
}

func (a *ConfigurationServiceAdapter) GetWizardProgress(userID int) (*models.WizardProgress, error) {
	return a.Inner.GetWizardProgress(userID)
}

func (a *ConfigurationServiceAdapter) CompleteWizard(userID int, finalData map[string]interface{}) (*models.SystemConfiguration, error) {
	return a.Inner.CompleteWizard(userID, finalData)
}

func (a *ConfigurationServiceAdapter) GetConfiguration() (*models.Configuration, error) {
	sysCfg, err := a.Inner.GetConfiguration()
	if err != nil {
		return nil, err
	}
	return &models.Configuration{
		ID:        sysCfg.Version,
		UpdatedAt: sysCfg.UpdatedAt,
	}, nil
}

func (a *ConfigurationServiceAdapter) TestConfiguration(config *models.Configuration) (*models.ValidationResult, error) {
	return &models.ValidationResult{IsValid: true}, nil
}

func (a *ConfigurationServiceAdapter) GetConfigurationSchema() (*models.ConfigurationSchema, error) {
	return &models.ConfigurationSchema{}, nil
}

// ErrorReportingServiceAdapter adapts *services.ErrorReportingService
type ErrorReportingServiceAdapter struct {
	Inner *services.ErrorReportingService
}

func (a *ErrorReportingServiceAdapter) ReportError(userID int, request *models.ErrorReportRequest) (*models.ErrorReport, error) {
	return a.Inner.ReportError(userID, request)
}

func (a *ErrorReportingServiceAdapter) ReportCrash(userID int, request *models.CrashReportRequest) (*models.CrashReport, error) {
	return a.Inner.ReportCrash(userID, request)
}

func (a *ErrorReportingServiceAdapter) GetErrorReport(reportID int, userID int) (*models.ErrorReport, error) {
	return a.Inner.GetErrorReport(reportID, userID)
}

func (a *ErrorReportingServiceAdapter) GetCrashReport(reportID int, userID int) (*models.CrashReport, error) {
	return a.Inner.GetCrashReport(reportID, userID)
}

func (a *ErrorReportingServiceAdapter) GetErrorReportsByUser(userID int, filters *models.ErrorReportFilters) ([]models.ErrorReport, error) {
	ptrs, err := a.Inner.GetErrorReportsByUser(userID, filters)
	if err != nil {
		return nil, err
	}
	result := make([]models.ErrorReport, len(ptrs))
	for i, p := range ptrs {
		result[i] = *p
	}
	return result, nil
}

func (a *ErrorReportingServiceAdapter) GetCrashReportsByUser(userID int, filters *models.CrashReportFilters) ([]models.CrashReport, error) {
	ptrs, err := a.Inner.GetCrashReportsByUser(userID, filters)
	if err != nil {
		return nil, err
	}
	result := make([]models.CrashReport, len(ptrs))
	for i, p := range ptrs {
		result[i] = *p
	}
	return result, nil
}

func (a *ErrorReportingServiceAdapter) UpdateErrorStatus(reportID int, userID int, status string) error {
	return a.Inner.UpdateErrorStatus(reportID, userID, status)
}

func (a *ErrorReportingServiceAdapter) UpdateCrashStatus(reportID int, userID int, status string) error {
	return a.Inner.UpdateCrashStatus(reportID, userID, status)
}

func (a *ErrorReportingServiceAdapter) GetErrorStatistics(userID int) (*models.ErrorStatistics, error) {
	return a.Inner.GetErrorStatistics(userID)
}

func (a *ErrorReportingServiceAdapter) GetCrashStatistics(userID int) (*models.CrashStatistics, error) {
	return a.Inner.GetCrashStatistics(userID)
}

func (a *ErrorReportingServiceAdapter) GetSystemHealth() (*models.SystemHealth, error) {
	return a.Inner.GetSystemHealth()
}

func (a *ErrorReportingServiceAdapter) UpdateConfiguration(config *services.ErrorReportingConfig) error {
	return a.Inner.UpdateConfiguration(config)
}

func (a *ErrorReportingServiceAdapter) GetConfiguration() (*services.ErrorReportingConfig, error) {
	cfg := a.Inner.GetConfiguration()
	return cfg, nil
}

func (a *ErrorReportingServiceAdapter) CleanupOldReports(olderThan time.Time) error {
	return a.Inner.CleanupOldReports(olderThan)
}

func (a *ErrorReportingServiceAdapter) ExportReports(userID int, filters *models.ExportFilters) ([]byte, error) {
	return a.Inner.ExportReports(userID, filters)
}

// LogManagementServiceAdapter adapts *services.LogManagementService
type LogManagementServiceAdapter struct {
	Inner    *services.LogManagementService
	streamWg sync.WaitGroup // tracks background StreamLogs relay goroutines
}

func (a *LogManagementServiceAdapter) CollectLogs(userID int, request *models.LogCollectionRequest) (*models.LogCollection, error) {
	return a.Inner.CollectLogs(userID, request)
}

func (a *LogManagementServiceAdapter) GetLogCollection(collectionID int, userID int) (*models.LogCollection, error) {
	return a.Inner.GetLogCollection(collectionID, userID)
}

func (a *LogManagementServiceAdapter) GetLogCollectionsByUser(userID int, limit, offset int) ([]models.LogCollection, error) {
	ptrs, err := a.Inner.GetLogCollectionsByUser(userID, limit, offset)
	if err != nil {
		return nil, err
	}
	result := make([]models.LogCollection, len(ptrs))
	for i, p := range ptrs {
		result[i] = *p
	}
	return result, nil
}

func (a *LogManagementServiceAdapter) GetLogEntries(collectionID int, userID int, filters *models.LogEntryFilters) ([]models.LogEntry, error) {
	ptrs, err := a.Inner.GetLogEntries(collectionID, userID, filters)
	if err != nil {
		return nil, err
	}
	result := make([]models.LogEntry, len(ptrs))
	for i, p := range ptrs {
		result[i] = *p
	}
	return result, nil
}

func (a *LogManagementServiceAdapter) CreateLogShare(userID int, request *models.LogShareRequest) (*models.LogShare, error) {
	return a.Inner.CreateLogShare(userID, request)
}

func (a *LogManagementServiceAdapter) GetLogShare(token string) (*models.LogShare, error) {
	return a.Inner.GetLogShare(token)
}

func (a *LogManagementServiceAdapter) RevokeLogShare(shareID int, userID int) error {
	return a.Inner.RevokeLogShare(shareID, userID)
}

func (a *LogManagementServiceAdapter) ExportLogs(collectionID int, userID int, format string) ([]byte, error) {
	return a.Inner.ExportLogs(collectionID, userID, format)
}

func (a *LogManagementServiceAdapter) StreamLogs(userID int, filters *models.LogStreamFilters) (<-chan models.LogEntry, chan<- struct{}, error) {
	ptrCh, done, err := a.Inner.StreamLogs(userID, filters)
	if err != nil {
		return nil, nil, err
	}
	ch := make(chan models.LogEntry, 64)
	a.streamWg.Add(1)
	go func() {
		defer a.streamWg.Done()
		defer close(ch)
		for p := range ptrCh {
			if p != nil {
				select {
				case ch <- *p:
				default:
					// Drop entry if consumer is not keeping up to avoid goroutine leak
				}
			}
		}
	}()
	return ch, done, nil
}

// Close waits for all background stream relay goroutines to finish.
// Must be called during shutdown to avoid goroutine leaks.
func (a *LogManagementServiceAdapter) Close() {
	a.streamWg.Wait()
}

func (a *LogManagementServiceAdapter) AnalyzeLogs(collectionID int, userID int) (*models.LogAnalysis, error) {
	return a.Inner.AnalyzeLogs(collectionID, userID)
}

func (a *LogManagementServiceAdapter) GetLogStatistics(userID int) (*models.LogStatistics, error) {
	return a.Inner.GetLogStatistics(userID)
}

func (a *LogManagementServiceAdapter) GetConfiguration() *services.LogManagementConfig {
	return a.Inner.GetConfiguration()
}

func (a *LogManagementServiceAdapter) UpdateConfiguration(config *services.LogManagementConfig) error {
	return a.Inner.UpdateConfiguration(config)
}

func (a *LogManagementServiceAdapter) CleanupOldLogs() error {
	return a.Inner.CleanupOldLogs()
}
