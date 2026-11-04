package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"catalogizer/database"
	"catalogizer/models"
)

type ErrorReportingRepository struct {
	db *database.DB
}

func NewErrorReportingRepository(db *database.DB) *ErrorReportingRepository {
	return &ErrorReportingRepository{db: db}
}

func (r *ErrorReportingRepository) CreateErrorReport(report *models.ErrorReport) error {
	contextJSON, err := json.Marshal(report.Context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	systemInfoJSON, err := json.Marshal(report.SystemInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal system info: %w", err)
	}

	query := `
		INSERT INTO error_reports (
			user_id, level, message, error_code, component, stack_trace,
			context, system_info, user_agent, url, fingerprint, status, reported_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	id, err := r.db.InsertReturningID(context.Background(), query,
		report.UserID, report.Level, report.Message, report.ErrorCode,
		report.Component, report.StackTrace, string(contextJSON),
		string(systemInfoJSON), report.UserAgent, report.URL,
		report.Fingerprint, report.Status, report.ReportedAt)

	if err != nil {
		return fmt.Errorf("failed to create error report: %w", err)
	}

	report.ID = int(id)
	return nil
}

func (r *ErrorReportingRepository) GetErrorReport(id int) (*models.ErrorReport, error) {
	query := `
		SELECT id, user_id, level, message, error_code, component, stack_trace,
			   context, system_info, user_agent, url, fingerprint, status,
			   reported_at, resolved_at
		FROM error_reports WHERE id = ?`

	var report models.ErrorReport
	var contextJSON, systemInfoJSON string
	var resolvedAt sql.NullTime

	err := r.db.QueryRow(query, id).Scan(
		&report.ID, &report.UserID, &report.Level, &report.Message,
		&report.ErrorCode, &report.Component, &report.StackTrace,
		&contextJSON, &systemInfoJSON, &report.UserAgent, &report.URL,
		&report.Fingerprint, &report.Status, &report.ReportedAt, &resolvedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to get error report: %w", err)
	}

	if resolvedAt.Valid {
		report.ResolvedAt = &resolvedAt.Time
	}

	if err := json.Unmarshal([]byte(contextJSON), &report.Context); err != nil {
		return nil, fmt.Errorf("failed to unmarshal context: %w", err)
	}

	if err := json.Unmarshal([]byte(systemInfoJSON), &report.SystemInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal system info: %w", err)
	}

	return &report, nil
}

func (r *ErrorReportingRepository) UpdateErrorReport(report *models.ErrorReport) error {
	contextJSON, err := json.Marshal(report.Context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	systemInfoJSON, err := json.Marshal(report.SystemInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal system info: %w", err)
	}

	query := `
		UPDATE error_reports SET
			level = ?, message = ?, error_code = ?, component = ?,
			stack_trace = ?, context = ?, system_info = ?, user_agent = ?,
			url = ?, fingerprint = ?, status = ?, resolved_at = ?
		WHERE id = ?`

	_, err = r.db.Exec(query,
		report.Level, report.Message, report.ErrorCode, report.Component,
		report.StackTrace, string(contextJSON), string(systemInfoJSON),
		report.UserAgent, report.URL, report.Fingerprint, report.Status,
		report.ResolvedAt, report.ID)

	if err != nil {
		return fmt.Errorf("failed to update error report: %w", err)
	}

	return nil
}

func (r *ErrorReportingRepository) GetErrorReportsByUser(userID int, filters *models.ErrorReportFilters) ([]*models.ErrorReport, error) {
	query := `
		SELECT id, user_id, level, message, error_code, component, stack_trace,
			   context, system_info, user_agent, url, fingerprint, status,
			   reported_at, resolved_at
		FROM error_reports
		WHERE user_id = ?`

	args := []interface{}{userID}

	if filters != nil {
		if filters.Level != "" {
			query += " AND level = ?"
			args = append(args, filters.Level)
		}

		if filters.Component != "" {
			query += " AND component = ?"
			args = append(args, filters.Component)
		}

		if filters.Status != "" {
			query += " AND status = ?"
			args = append(args, filters.Status)
		}

		if filters.StartDate != nil {
			query += " AND reported_at >= ?"
			args = append(args, *filters.StartDate)
		}

		if filters.EndDate != nil {
			query += " AND reported_at <= ?"
			args = append(args, *filters.EndDate)
		}
	}

	query += " ORDER BY reported_at DESC"

	if filters != nil && filters.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filters.Limit)

		if filters.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filters.Offset)
		}
	}

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get error reports: %w", err)
	}
	defer rows.Close()

	reports := []*models.ErrorReport{}
	for rows.Next() {
		var report models.ErrorReport
		var contextJSON, systemInfoJSON string
		var resolvedAt sql.NullTime

		err := rows.Scan(
			&report.ID, &report.UserID, &report.Level, &report.Message,
			&report.ErrorCode, &report.Component, &report.StackTrace,
			&contextJSON, &systemInfoJSON, &report.UserAgent, &report.URL,
			&report.Fingerprint, &report.Status, &report.ReportedAt, &resolvedAt)

		if err != nil {
			return nil, fmt.Errorf("failed to scan error report: %w", err)
		}

		if resolvedAt.Valid {
			report.ResolvedAt = &resolvedAt.Time
		}

		if err := json.Unmarshal([]byte(contextJSON), &report.Context); err != nil {
			return nil, fmt.Errorf("failed to unmarshal context: %w", err)
		}

		if err := json.Unmarshal([]byte(systemInfoJSON), &report.SystemInfo); err != nil {
			return nil, fmt.Errorf("failed to unmarshal system info: %w", err)
		}

		reports = append(reports, &report)
	}

	return reports, nil
}

func (r *ErrorReportingRepository) DeleteErrorReport(id int) error {
	query := "DELETE FROM error_reports WHERE id = ?"
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete error report: %w", err)
	}
	return nil
}

func (r *ErrorReportingRepository) GetErrorCountInLastHour(userID int) (int, error) {
	cutoffTime := time.Now().Add(-1 * time.Hour)
	query := `
		SELECT COUNT(*)
		FROM error_reports
		WHERE user_id = ? AND reported_at > ?`

	var count int
	err := r.db.QueryRow(query, userID, cutoffTime).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get error count: %w", err)
	}

	return count, nil
}

func (r *ErrorReportingRepository) GetRecentErrorCount(duration time.Duration) (int, error) {
	since := time.Now().Add(-duration)
	query := `
		SELECT COUNT(*)
		FROM error_reports
		WHERE reported_at > ?`

	var count int
	err := r.db.QueryRow(query, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get recent error count: %w", err)
	}

	return count, nil
}

func (r *ErrorReportingRepository) GetErrorStatistics(userID int) (*models.ErrorStatistics, error) {
	stats := &models.ErrorStatistics{}

	// Total errors
	err := r.db.QueryRow("SELECT COUNT(*) FROM error_reports WHERE user_id = ?", userID).Scan(&stats.TotalErrors)
	if err != nil {
		return nil, fmt.Errorf("failed to get total errors: %w", err)
	}

	// Errors by level
	levelQuery := `
		SELECT level, COUNT(*)
		FROM error_reports
		WHERE user_id = ?
		GROUP BY level`

	rows, err := r.db.Query(levelQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get error levels: %w", err)
	}
	defer rows.Close()

	stats.ErrorsByLevel = make(map[string]int)
	for rows.Next() {
		var level string
		var count int
		if err := rows.Scan(&level, &count); err != nil {
			return nil, fmt.Errorf("failed to scan error level: %w", err)
		}
		stats.ErrorsByLevel[level] = count
	}

	// Errors by component
	componentQuery := `
		SELECT component, COUNT(*)
		FROM error_reports
		WHERE user_id = ?
		GROUP BY component
		ORDER BY COUNT(*) DESC
		LIMIT 10`

	rows, err = r.db.Query(componentQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get error components: %w", err)
	}
	defer rows.Close()

	stats.ErrorsByComponent = make(map[string]int)
	for rows.Next() {
		var component string
		var count int
		if err := rows.Scan(&component, &count); err != nil {
			return nil, fmt.Errorf("failed to scan error component: %w", err)
		}
		stats.ErrorsByComponent[component] = count
	}

	// Recent errors (last 24 hours)
	cutoffTime := time.Now().Add(-24 * time.Hour)
	err = r.db.QueryRow(`
		SELECT COUNT(*)
		FROM error_reports
		WHERE user_id = ? AND reported_at > ?`, userID, cutoffTime).Scan(&stats.RecentErrors)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent errors: %w", err)
	}

	// Resolved errors
	err = r.db.QueryRow(`
		SELECT COUNT(*)
		FROM error_reports
		WHERE user_id = ? AND status = ?`, userID, models.ErrorStatusResolved).Scan(&stats.ResolvedErrors)
	if err != nil {
		return nil, fmt.Errorf("failed to get resolved errors: %w", err)
	}

	// Average resolution time (in hours)
	avgDurationExpr := "(julianday(resolved_at) - julianday(reported_at)) * 24"
	if r.db.Dialect().IsPostgres() {
		avgDurationExpr = "EXTRACT(EPOCH FROM (resolved_at - reported_at)) / 3600"
	}
	avgQuery := fmt.Sprintf(`
		SELECT COALESCE(AVG(
			CASE
				WHEN resolved_at IS NOT NULL
				THEN %s
				ELSE NULL
			END
		), 0.0)
		FROM error_reports
		WHERE user_id = ? AND resolved_at IS NOT NULL`, avgDurationExpr)
	var avgResolution sql.NullFloat64
	err = r.db.QueryRow(avgQuery, userID).Scan(&avgResolution)
	if err != nil {
		return nil, fmt.Errorf("failed to get average resolution time: %w", err)
	}
	stats.AvgResolutionTime = avgResolution.Float64

	return stats, nil
}

func (r *ErrorReportingRepository) GetErrorsByFingerprint(fingerprint string, limit int) ([]*models.ErrorReport, error) {
	query := `
		SELECT id, user_id, level, message, error_code, component, stack_trace,
			   context, system_info, user_agent, url, fingerprint, status,
			   reported_at, resolved_at
		FROM error_reports
		WHERE fingerprint = ?
		ORDER BY reported_at DESC
		LIMIT ?`

	rows, err := r.db.Query(query, fingerprint, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get errors by fingerprint: %w", err)
	}
	defer rows.Close()

	reports := []*models.ErrorReport{}
	for rows.Next() {
		var report models.ErrorReport
		var contextJSON, systemInfoJSON string
		var resolvedAt sql.NullTime

		err := rows.Scan(
			&report.ID, &report.UserID, &report.Level, &report.Message,
			&report.ErrorCode, &report.Component, &report.StackTrace,
			&contextJSON, &systemInfoJSON, &report.UserAgent, &report.URL,
			&report.Fingerprint, &report.Status, &report.ReportedAt, &resolvedAt)

		if err != nil {
			return nil, fmt.Errorf("failed to scan error report: %w", err)
		}

		if resolvedAt.Valid {
			report.ResolvedAt = &resolvedAt.Time
		}

		if err := json.Unmarshal([]byte(contextJSON), &report.Context); err != nil {
			return nil, fmt.Errorf("failed to unmarshal context: %w", err)
		}

		if err := json.Unmarshal([]byte(systemInfoJSON), &report.SystemInfo); err != nil {
			return nil, fmt.Errorf("failed to unmarshal system info: %w", err)
		}

		reports = append(reports, &report)
	}

	return reports, nil
}

func (r *ErrorReportingRepository) CleanupOldReports(olderThan time.Time) error {
	query := "DELETE FROM error_reports WHERE reported_at < ?"
	_, err := r.db.Exec(query, olderThan)
	if err != nil {
		return fmt.Errorf("failed to cleanup old error reports: %w", err)
	}
	return nil
}

func (r *ErrorReportingRepository) GetTopErrors(userID int, limit int, timeRange time.Duration) ([]*models.TopError, error) {
	since := time.Now().Add(-timeRange)
	query := `
		SELECT fingerprint, COUNT(*) as count, MAX(reported_at) as last_seen,
			   MIN(reported_at) as first_seen, message, component, level
		FROM error_reports
		WHERE user_id = ? AND reported_at > ?
		GROUP BY fingerprint
		ORDER BY count DESC
		LIMIT ?`

	rows, err := r.db.Query(query, userID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get top errors: %w", err)
	}
	defer rows.Close()

	topErrors := []*models.TopError{}
	for rows.Next() {
		var topError models.TopError
		err := rows.Scan(
			&topError.Fingerprint, &topError.Count, &topError.LastSeen,
			&topError.FirstSeen, &topError.Message, &topError.Component, &topError.Level)

		if err != nil {
			return nil, fmt.Errorf("failed to scan top error: %w", err)
		}

		topErrors = append(topErrors, &topError)
	}

	return topErrors, nil
}
