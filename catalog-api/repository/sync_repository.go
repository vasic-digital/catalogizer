package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"catalogizer/database"
	"catalogizer/models"
)

type SyncRepository struct {
	db *database.DB
}

func NewSyncRepository(db *database.DB) *SyncRepository {
	return &SyncRepository{db: db}
}

func (r *SyncRepository) CreateEndpoint(endpoint *models.SyncEndpoint) (int, error) {
	query := `
		INSERT INTO sync_endpoints (user_id, name, type, url, username, password, sync_direction,
								   local_path, remote_path, sync_settings, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	id, err := r.db.InsertReturningID(context.Background(), query,
		endpoint.UserID, endpoint.Name, endpoint.Type, endpoint.URL, endpoint.Username,
		endpoint.Password, endpoint.SyncDirection, endpoint.LocalPath, endpoint.RemotePath,
		endpoint.SyncSettings, endpoint.Status, endpoint.CreatedAt, endpoint.UpdatedAt)

	if err != nil {
		return 0, fmt.Errorf("failed to create sync endpoint: %w", err)
	}

	return int(id), nil
}

func (r *SyncRepository) GetEndpoint(endpointID int) (*models.SyncEndpoint, error) {
	query := `
		SELECT id, user_id, name, type, url, username, password, sync_direction,
			   local_path, remote_path, sync_settings, status, created_at, updated_at, last_sync_at
		FROM sync_endpoints
		WHERE id = ?
	`

	endpoint := &models.SyncEndpoint{}
	var syncSettings sql.NullString
	var lastSyncAt sql.NullTime

	err := r.db.QueryRow(query, endpointID).Scan(
		&endpoint.ID, &endpoint.UserID, &endpoint.Name, &endpoint.Type, &endpoint.URL,
		&endpoint.Username, &endpoint.Password, &endpoint.SyncDirection, &endpoint.LocalPath,
		&endpoint.RemotePath, &syncSettings, &endpoint.Status, &endpoint.CreatedAt,
		&endpoint.UpdatedAt, &lastSyncAt)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("endpoint not found")
		}
		return nil, fmt.Errorf("failed to get endpoint: %w", err)
	}

	if syncSettings.Valid {
		endpoint.SyncSettings = &syncSettings.String
	}

	if lastSyncAt.Valid {
		endpoint.LastSyncAt = &lastSyncAt.Time
	}

	return endpoint, nil
}

func (r *SyncRepository) UpdateEndpoint(endpoint *models.SyncEndpoint) error {
	query := `
		UPDATE sync_endpoints
		SET name = ?, type = ?, url = ?, username = ?, password = ?, sync_direction = ?,
			local_path = ?, remote_path = ?, sync_settings = ?, status = ?, updated_at = ?, last_sync_at = ?
		WHERE id = ?
	`

	var lastSyncAt sql.NullTime
	if endpoint.LastSyncAt != nil {
		lastSyncAt = sql.NullTime{Time: *endpoint.LastSyncAt, Valid: true}
	}

	_, err := r.db.Exec(query,
		endpoint.Name, endpoint.Type, endpoint.URL, endpoint.Username, endpoint.Password,
		endpoint.SyncDirection, endpoint.LocalPath, endpoint.RemotePath, endpoint.SyncSettings,
		endpoint.Status, endpoint.UpdatedAt, lastSyncAt, endpoint.ID)

	return err
}

func (r *SyncRepository) DeleteEndpoint(endpointID int) error {
	query := `DELETE FROM sync_endpoints WHERE id = ?`
	_, err := r.db.Exec(query, endpointID)
	return err
}

func (r *SyncRepository) GetUserEndpoints(userID int) ([]models.SyncEndpoint, error) {
	query := `
		SELECT id, user_id, name, type, url, username, password, sync_direction,
			   local_path, remote_path, sync_settings, status, created_at, updated_at, last_sync_at
		FROM sync_endpoints
		WHERE user_id = ?
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user endpoints: %w", err)
	}
	defer rows.Close()

	return r.scanEndpoints(rows)
}

func (r *SyncRepository) CreateSession(session *models.SyncSession) (int, error) {
	query := `
		INSERT INTO sync_sessions (endpoint_id, user_id, status, sync_type, started_at,
								  total_files, synced_files, failed_files, skipped_files)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	id, err := r.db.InsertReturningID(context.Background(), query,
		session.EndpointID, session.UserID, session.Status, session.SyncType, session.StartedAt,
		session.TotalFiles, session.SyncedFiles, session.FailedFiles, session.SkippedFiles)

	if err != nil {
		return 0, fmt.Errorf("failed to create sync session: %w", err)
	}

	return int(id), nil
}

func (r *SyncRepository) GetSession(sessionID int) (*models.SyncSession, error) {
	query := `
		SELECT id, endpoint_id, user_id, status, sync_type, started_at, completed_at,
			   duration, total_files, synced_files, failed_files, skipped_files, error_message
		FROM sync_sessions
		WHERE id = ?
	`

	session := &models.SyncSession{}
	var completedAt sql.NullTime
	var durationSeconds sql.NullInt64
	var errorMessage sql.NullString

	err := r.db.QueryRow(query, sessionID).Scan(
		&session.ID, &session.EndpointID, &session.UserID, &session.Status, &session.SyncType,
		&session.StartedAt, &completedAt, &durationSeconds, &session.TotalFiles,
		&session.SyncedFiles, &session.FailedFiles, &session.SkippedFiles, &errorMessage)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("session not found")
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if completedAt.Valid {
		session.CompletedAt = &completedAt.Time
	}

	if durationSeconds.Valid {
		duration := time.Duration(durationSeconds.Int64) * time.Second
		session.Duration = &duration
	}

	if errorMessage.Valid {
		session.ErrorMessage = &errorMessage.String
	}

	return session, nil
}

func (r *SyncRepository) UpdateSession(session *models.SyncSession) error {
	query := `
		UPDATE sync_sessions
		SET status = ?, completed_at = ?, duration = ?, total_files = ?, synced_files = ?,
			failed_files = ?, skipped_files = ?, error_message = ?, updated_at = ?
		WHERE id = ?
	`

	var completedAt sql.NullTime
	var durationSeconds sql.NullInt64
	var errorMessage sql.NullString

	if session.CompletedAt != nil {
		completedAt = sql.NullTime{Time: *session.CompletedAt, Valid: true}
	}

	if session.Duration != nil {
		durationSeconds = sql.NullInt64{Int64: int64(session.Duration.Seconds()), Valid: true}
	}

	if session.ErrorMessage != nil {
		errorMessage = sql.NullString{String: *session.ErrorMessage, Valid: true}
	}

	_, err := r.db.Exec(query,
		session.Status, completedAt, durationSeconds, session.TotalFiles, session.SyncedFiles,
		session.FailedFiles, session.SkippedFiles, errorMessage, time.Now(), session.ID)

	return err
}

func (r *SyncRepository) GetUserSessions(userID int, limit, offset int) ([]models.SyncSession, error) {
	query := `
		SELECT id, endpoint_id, user_id, status, sync_type, started_at, completed_at,
			   duration, total_files, synced_files, failed_files, skipped_files, error_message
		FROM sync_sessions
		WHERE user_id = ?
		ORDER BY started_at DESC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get user sessions: %w", err)
	}
	defer rows.Close()

	return r.scanSessions(rows)
}

func (r *SyncRepository) CreateSchedule(schedule *models.SyncSchedule) (int, error) {
	query := `
		INSERT INTO sync_schedules (endpoint_id, user_id, frequency, is_active, created_at)
		VALUES (?, ?, ?, ?, ?)
	`

	id, err := r.db.InsertReturningID(context.Background(), query,
		schedule.EndpointID, schedule.UserID, schedule.Frequency, schedule.IsActive, schedule.CreatedAt)

	if err != nil {
		return 0, fmt.Errorf("failed to create sync schedule: %w", err)
	}

	return int(id), nil
}

func (r *SyncRepository) GetActiveSchedules() ([]models.SyncSchedule, error) {
	query := `
		SELECT id, endpoint_id, user_id, frequency, last_run, next_run, is_active, created_at
		FROM sync_schedules
		WHERE is_active = 1
		ORDER BY next_run ASC
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get active schedules: %w", err)
	}
	defer rows.Close()

	return r.scanSchedules(rows)
}

func (r *SyncRepository) GetStatistics(userID *int, startDate, endDate time.Time) (*models.SyncStatistics, error) {
	whereClause := "WHERE started_at BETWEEN ? AND ?"
	args := []interface{}{startDate, endDate}

	if userID != nil {
		whereClause += " AND user_id = ?"
		args = append(args, *userID)
	}

	// Get session counts by status
	statusQuery := fmt.Sprintf(`
		SELECT status, COUNT(*) as count
		FROM sync_sessions
		%s
		GROUP BY status
	`, whereClause)

	statusRows, err := r.db.Query(statusQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get status statistics: %w", err)
	}
	defer statusRows.Close()

	stats := &models.SyncStatistics{
		StartDate: startDate,
		EndDate:   endDate,
		ByStatus:  make(map[string]int),
		ByType:    make(map[string]int),
	}

	for statusRows.Next() {
		var status string
		var count int
		err := statusRows.Scan(&status, &count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan status stats: %w", err)
		}
		stats.ByStatus[status] = count
		stats.TotalSessions += count
	}

	// Get session counts by sync type
	typeQuery := fmt.Sprintf(`
		SELECT sync_type, COUNT(*) as count
		FROM sync_sessions
		%s
		GROUP BY sync_type
	`, whereClause)

	typeRows, err := r.db.Query(typeQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get type statistics: %w", err)
	}
	defer typeRows.Close()

	for typeRows.Next() {
		var syncType string
		var count int
		err := typeRows.Scan(&syncType, &count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan type stats: %w", err)
		}
		stats.ByType[syncType] = count
	}

	// Get total files synced
	filesQuery := fmt.Sprintf(`
		SELECT SUM(synced_files) as total_synced, SUM(failed_files) as total_failed
		FROM sync_sessions
		%s AND status = 'completed'
	`, whereClause)

	var totalSynced, totalFailed sql.NullInt64
	err = r.db.QueryRow(filesQuery, args...).Scan(&totalSynced, &totalFailed)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get file statistics: %w", err)
	}

	if totalSynced.Valid {
		stats.TotalFilesSynced = int(totalSynced.Int64)
	}

	if totalFailed.Valid {
		stats.TotalFilesFailed = int(totalFailed.Int64)
	}

	// Get average duration for completed sessions
	durationQuery := fmt.Sprintf(`
		SELECT AVG(duration) as avg_duration
		FROM sync_sessions
		%s AND status = 'completed' AND duration IS NOT NULL
	`, whereClause)

	var avgDurationSeconds sql.NullFloat64
	err = r.db.QueryRow(durationQuery, args...).Scan(&avgDurationSeconds)
	if err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("failed to get duration statistics: %w", err)
	}

	if avgDurationSeconds.Valid {
		avgDuration := time.Duration(avgDurationSeconds.Float64) * time.Second
		stats.AverageDuration = &avgDuration
	}

	// Calculate success rate
	if completed, ok := stats.ByStatus[models.SyncSessionStatusCompleted]; ok {
		if failed, failedOk := stats.ByStatus[models.SyncSessionStatusFailed]; failedOk {
			total := completed + failed
			if total > 0 {
				stats.SuccessRate = float64(completed) / float64(total) * 100
			}
		} else if completed > 0 {
			stats.SuccessRate = 100.0
		}
	}

	return stats, nil
}

func (r *SyncRepository) CleanupSessions(olderThan time.Time) error {
	query := `
		DELETE FROM sync_sessions
		WHERE completed_at < ? AND status IN ('completed', 'failed', 'cancelled')
	`

	_, err := r.db.Exec(query, olderThan)
	return err
}

func (r *SyncRepository) GetEndpointsByType(syncType string) ([]models.SyncEndpoint, error) {
	query := `
		SELECT id, user_id, name, type, url, username, password, sync_direction,
			   local_path, remote_path, sync_settings, status, created_at, updated_at, last_sync_at
		FROM sync_endpoints
		WHERE type = ? AND status = 'active'
		ORDER BY created_at DESC
	`

	rows, err := r.db.Query(query, syncType)
	if err != nil {
		return nil, fmt.Errorf("failed to get endpoints by type: %w", err)
	}
	defer rows.Close()

	return r.scanEndpoints(rows)
}

func (r *SyncRepository) scanEndpoints(rows *sql.Rows) ([]models.SyncEndpoint, error) {
	endpoints := []models.SyncEndpoint{}

	for rows.Next() {
		var endpoint models.SyncEndpoint
		var syncSettings sql.NullString
		var lastSyncAt sql.NullTime

		err := rows.Scan(
			&endpoint.ID, &endpoint.UserID, &endpoint.Name, &endpoint.Type, &endpoint.URL,
			&endpoint.Username, &endpoint.Password, &endpoint.SyncDirection, &endpoint.LocalPath,
			&endpoint.RemotePath, &syncSettings, &endpoint.Status, &endpoint.CreatedAt,
			&endpoint.UpdatedAt, &lastSyncAt)

		if err != nil {
			return nil, fmt.Errorf("failed to scan endpoint: %w", err)
		}

		if syncSettings.Valid {
			endpoint.SyncSettings = &syncSettings.String
		}

		if lastSyncAt.Valid {
			endpoint.LastSyncAt = &lastSyncAt.Time
		}

		endpoints = append(endpoints, endpoint)
	}

	return endpoints, nil
}

func (r *SyncRepository) scanSessions(rows *sql.Rows) ([]models.SyncSession, error) {
	sessions := []models.SyncSession{}

	for rows.Next() {
		var session models.SyncSession
		var completedAt sql.NullTime
		var durationSeconds sql.NullInt64
		var errorMessage sql.NullString

		err := rows.Scan(
			&session.ID, &session.EndpointID, &session.UserID, &session.Status, &session.SyncType,
			&session.StartedAt, &completedAt, &durationSeconds, &session.TotalFiles,
			&session.SyncedFiles, &session.FailedFiles, &session.SkippedFiles, &errorMessage)

		if err != nil {
			return nil, fmt.Errorf("failed to scan session: %w", err)
		}

		if completedAt.Valid {
			session.CompletedAt = &completedAt.Time
		}

		if durationSeconds.Valid {
			duration := time.Duration(durationSeconds.Int64) * time.Second
			session.Duration = &duration
		}

		if errorMessage.Valid {
			session.ErrorMessage = &errorMessage.String
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (r *SyncRepository) scanSchedules(rows *sql.Rows) ([]models.SyncSchedule, error) {
	schedules := []models.SyncSchedule{}

	for rows.Next() {
		var schedule models.SyncSchedule
		var lastRun, nextRun sql.NullTime

		err := rows.Scan(
			&schedule.ID, &schedule.EndpointID, &schedule.UserID, &schedule.Frequency,
			&lastRun, &nextRun, &schedule.IsActive, &schedule.CreatedAt)

		if err != nil {
			return nil, fmt.Errorf("failed to scan schedule: %w", err)
		}

		if lastRun.Valid {
			schedule.LastRun = &lastRun.Time
		}

		if nextRun.Valid {
			schedule.NextRun = &nextRun.Time
		}

		schedules = append(schedules, schedule)
	}

	return schedules, nil
}
