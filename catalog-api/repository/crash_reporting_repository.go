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

type CrashReportingRepository struct {
	db *database.DB
}

func NewCrashReportingRepository(db *database.DB) *CrashReportingRepository {
	return &CrashReportingRepository{db: db}
}

func (r *CrashReportingRepository) CreateCrashReport(report *models.CrashReport) error {
	contextJSON, err := json.Marshal(report.Context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	systemInfoJSON, err := json.Marshal(report.SystemInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal system info: %w", err)
	}

	query := `
		INSERT INTO crash_reports (
			user_id, signal, message, stack_trace, context, system_info,
			fingerprint, status, reported_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`

	id, err := r.db.InsertReturningID(context.Background(), query,
		report.UserID, report.Signal, report.Message, report.StackTrace,
		string(contextJSON), string(systemInfoJSON), report.Fingerprint,
		report.Status, report.ReportedAt)

	if err != nil {
		return fmt.Errorf("failed to create crash report: %w", err)
	}

	report.ID = int(id)
	return nil
}

func (r *CrashReportingRepository) GetCrashReport(id int) (*models.CrashReport, error) {
	query := `
		SELECT id, user_id, signal, message, stack_trace, context, system_info,
			   fingerprint, status, reported_at, resolved_at
		FROM crash_reports WHERE id = ?`

	var report models.CrashReport
	var contextJSON, systemInfoJSON string
	var resolvedAt sql.NullTime

	err := r.db.QueryRow(query, id).Scan(
		&report.ID, &report.UserID, &report.Signal, &report.Message,
		&report.StackTrace, &contextJSON, &systemInfoJSON,
		&report.Fingerprint, &report.Status, &report.ReportedAt, &resolvedAt)

	if err != nil {
		return nil, fmt.Errorf("failed to get crash report: %w", err)
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

func (r *CrashReportingRepository) UpdateCrashReport(report *models.CrashReport) error {
	contextJSON, err := json.Marshal(report.Context)
	if err != nil {
		return fmt.Errorf("failed to marshal context: %w", err)
	}

	systemInfoJSON, err := json.Marshal(report.SystemInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal system info: %w", err)
	}

	query := `
		UPDATE crash_reports SET
			signal = ?, message = ?, stack_trace = ?, context = ?,
			system_info = ?, fingerprint = ?, status = ?, resolved_at = ?
		WHERE id = ?`

	_, err = r.db.Exec(query,
		report.Signal, report.Message, report.StackTrace,
		string(contextJSON), string(systemInfoJSON), report.Fingerprint,
		report.Status, report.ResolvedAt, report.ID)

	if err != nil {
		return fmt.Errorf("failed to update crash report: %w", err)
	}

	return nil
}

func (r *CrashReportingRepository) GetCrashReportsByUser(userID int, filters *models.CrashReportFilters) ([]*models.CrashReport, error) {
	query := `
		SELECT id, user_id, signal, message, stack_trace, context, system_info,
			   fingerprint, status, reported_at, resolved_at
		FROM crash_reports
		WHERE user_id = ?`

	args := []interface{}{userID}

	if filters != nil {
		if filters.Signal != "" {
			query += " AND signal = ?"
			args = append(args, filters.Signal)
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
		return nil, fmt.Errorf("failed to get crash reports: %w", err)
	}
	defer rows.Close()

	reports := []*models.CrashReport{}
	for rows.Next() {
		var report models.CrashReport
		var contextJSON, systemInfoJSON string
		var resolvedAt sql.NullTime

		err := rows.Scan(
			&report.ID, &report.UserID, &report.Signal, &report.Message,
			&report.StackTrace, &contextJSON, &systemInfoJSON,
			&report.Fingerprint, &report.Status, &report.ReportedAt, &resolvedAt)

		if err != nil {
			return nil, fmt.Errorf("failed to scan crash report: %w", err)
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

func (r *CrashReportingRepository) DeleteCrashReport(id int) error {
	query := "DELETE FROM crash_reports WHERE id = ?"
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete crash report: %w", err)
	}
	return nil
}

func (r *CrashReportingRepository) GetRecentCrashCount(duration time.Duration) (int, error) {
	since := time.Now().Add(-duration)
	query := `
		SELECT COUNT(*)
		FROM crash_reports
		WHERE reported_at > ?`

	var count int
	err := r.db.QueryRow(query, since).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("failed to get recent crash count: %w", err)
	}

	return count, nil
}

func (r *CrashReportingRepository) GetCrashStatistics(userID int) (*models.CrashStatistics, error) {
	stats := &models.CrashStatistics{}

	// Total crashes
	err := r.db.QueryRow("SELECT COUNT(*) FROM crash_reports WHERE user_id = ?", userID).Scan(&stats.TotalCrashes)
	if err != nil {
		return nil, fmt.Errorf("failed to get total crashes: %w", err)
	}

	// Crashes by signal
	signalQuery := `
		SELECT signal, COUNT(*)
		FROM crash_reports
		WHERE user_id = ?
		GROUP BY signal`

	rows, err := r.db.Query(signalQuery, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get crash signals: %w", err)
	}
	defer rows.Close()

	stats.CrashesBySignal = make(map[string]int)
	for rows.Next() {
		var signal string
		var count int
		if err := rows.Scan(&signal, &count); err != nil {
			return nil, fmt.Errorf("failed to scan crash signal: %w", err)
		}
		stats.CrashesBySignal[signal] = count
	}

	// Recent crashes (last 24 hours)
	cutoffTime := time.Now().Add(-24 * time.Hour)
	err = r.db.QueryRow(`
		SELECT COUNT(*)
		FROM crash_reports
		WHERE user_id = ? AND reported_at > ?`, userID, cutoffTime).Scan(&stats.RecentCrashes)
	if err != nil {
		return nil, fmt.Errorf("failed to get recent crashes: %w", err)
	}

	// Resolved crashes
	err = r.db.QueryRow(`
		SELECT COUNT(*)
		FROM crash_reports
		WHERE user_id = ? AND status = ?`, userID, models.CrashStatusResolved).Scan(&stats.ResolvedCrashes)
	if err != nil {
		return nil, fmt.Errorf("failed to get resolved crashes: %w", err)
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
		FROM crash_reports
		WHERE user_id = ? AND resolved_at IS NOT NULL`, avgDurationExpr)
	var avgResolution sql.NullFloat64
	err = r.db.QueryRow(avgQuery, userID).Scan(&avgResolution)
	if err != nil {
		return nil, fmt.Errorf("failed to get average resolution time: %w", err)
	}
	stats.AvgResolutionTime = avgResolution.Float64

	// Crash rate (crashes per day over last 30 days)
	cutoffTime30Days := time.Now().Add(-30 * 24 * time.Hour)
	err = r.db.QueryRow(`
		SELECT CAST(COUNT(*) AS FLOAT) / 30
		FROM crash_reports
		WHERE user_id = ? AND reported_at > ?`, userID, cutoffTime30Days).Scan(&stats.CrashRate)
	if err != nil {
		return nil, fmt.Errorf("failed to get crash rate: %w", err)
	}

	return stats, nil
}

func (r *CrashReportingRepository) GetCrashesByFingerprint(fingerprint string, limit int) ([]*models.CrashReport, error) {
	query := `
		SELECT id, user_id, signal, message, stack_trace, context, system_info,
			   fingerprint, status, reported_at, resolved_at
		FROM crash_reports
		WHERE fingerprint = ?
		ORDER BY reported_at DESC
		LIMIT ?`

	rows, err := r.db.Query(query, fingerprint, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get crashes by fingerprint: %w", err)
	}
	defer rows.Close()

	reports := []*models.CrashReport{}
	for rows.Next() {
		var report models.CrashReport
		var contextJSON, systemInfoJSON string
		var resolvedAt sql.NullTime

		err := rows.Scan(
			&report.ID, &report.UserID, &report.Signal, &report.Message,
			&report.StackTrace, &contextJSON, &systemInfoJSON,
			&report.Fingerprint, &report.Status, &report.ReportedAt, &resolvedAt)

		if err != nil {
			return nil, fmt.Errorf("failed to scan crash report: %w", err)
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

func (r *CrashReportingRepository) CleanupOldReports(olderThan time.Time) error {
	query := "DELETE FROM crash_reports WHERE reported_at < ?"
	_, err := r.db.Exec(query, olderThan)
	if err != nil {
		return fmt.Errorf("failed to cleanup old crash reports: %w", err)
	}
	return nil
}

func (r *CrashReportingRepository) GetTopCrashes(userID int, limit int, timeRange time.Duration) ([]*models.TopCrash, error) {
	since := time.Now().Add(-timeRange)
	query := `
		SELECT fingerprint, COUNT(*) as count, MAX(reported_at) as last_seen,
			   MIN(reported_at) as first_seen, message, signal
		FROM crash_reports
		WHERE user_id = ? AND reported_at > ?
		GROUP BY fingerprint
		ORDER BY count DESC
		LIMIT ?`

	rows, err := r.db.Query(query, userID, since, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get top crashes: %w", err)
	}
	defer rows.Close()

	topCrashes := []*models.TopCrash{}
	for rows.Next() {
		var topCrash models.TopCrash
		err := rows.Scan(
			&topCrash.Fingerprint, &topCrash.Count, &topCrash.LastSeen,
			&topCrash.FirstSeen, &topCrash.Message, &topCrash.Signal)

		if err != nil {
			return nil, fmt.Errorf("failed to scan top crash: %w", err)
		}

		topCrashes = append(topCrashes, &topCrash)
	}

	return topCrashes, nil
}

func (r *CrashReportingRepository) GetCrashTrends(userID int, days int) ([]*models.CrashTrend, error) {
	cutoffTime := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	query := `
		SELECT DATE(reported_at) as date, COUNT(*) as count
		FROM crash_reports
		WHERE user_id = ? AND reported_at > ?
		GROUP BY DATE(reported_at)
		ORDER BY date ASC`

	rows, err := r.db.Query(query, userID, cutoffTime)
	if err != nil {
		return nil, fmt.Errorf("failed to get crash trends: %w", err)
	}
	defer rows.Close()

	trends := []*models.CrashTrend{}
	for rows.Next() {
		var trend models.CrashTrend
		var dateStr string
		err := rows.Scan(&dateStr, &trend.Count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan crash trend: %w", err)
		}

		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse date: %w", err)
		}
		trend.Date = date

		trends = append(trends, &trend)
	}

	return trends, nil
}
