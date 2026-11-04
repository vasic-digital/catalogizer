package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"catalogizer/database"
	"catalogizer/models"
)

type ConversionRepository struct {
	db *database.DB
}

func NewConversionRepository(db *database.DB) *ConversionRepository {
	return &ConversionRepository{db: db}
}

func (r *ConversionRepository) CreateJob(job *models.ConversionJob) (int, error) {
	query := `
		INSERT INTO conversion_jobs (user_id, source_path, target_path, source_format, target_format,
									conversion_type, quality, settings, priority, status, created_at, scheduled_for)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	// scheduled_for is stored in UTC (and GetDueJobs compares in UTC): SQLite compares the text
	// form including the offset, and Postgres TIMESTAMP (no zone) drops the offset, so a value in
	// another zone would otherwise be compared as the wrong instant.
	var scheduled interface{}
	if job.ScheduledFor != nil {
		scheduled = job.ScheduledFor.UTC()
	}
	id, err := r.db.InsertReturningID(context.Background(), query,
		job.UserID, job.SourcePath, job.TargetPath, job.SourceFormat, job.TargetFormat,
		job.ConversionType, job.Quality, job.Settings, job.Priority, job.Status,
		job.CreatedAt, scheduled)

	if err != nil {
		return 0, fmt.Errorf("failed to create conversion job: %w", err)
	}

	return int(id), nil
}

func (r *ConversionRepository) GetJob(jobID int) (*models.ConversionJob, error) {
	query := `
		SELECT id, user_id, source_path, target_path, source_format, target_format,
			   conversion_type, quality, settings, priority, status, created_at,
			   started_at, completed_at, scheduled_for, duration, error_message
		FROM conversion_jobs
		WHERE id = ?
	`

	job := &models.ConversionJob{}
	var settings, errorMessage sql.NullString
	var startedAt, completedAt, scheduledFor sql.NullTime
	var durationSeconds sql.NullInt64

	err := r.db.QueryRow(query, jobID).Scan(
		&job.ID, &job.UserID, &job.SourcePath, &job.TargetPath, &job.SourceFormat, &job.TargetFormat,
		&job.ConversionType, &job.Quality, &settings, &job.Priority, &job.Status, &job.CreatedAt,
		&startedAt, &completedAt, &scheduledFor, &durationSeconds, &errorMessage)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, models.ErrConversionJobNotFound
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	if settings.Valid {
		job.Settings = &settings.String
	}

	if errorMessage.Valid {
		job.ErrorMessage = &errorMessage.String
	}

	if startedAt.Valid {
		job.StartedAt = &startedAt.Time
	}

	if completedAt.Valid {
		job.CompletedAt = &completedAt.Time
	}

	if scheduledFor.Valid {
		job.ScheduledFor = &scheduledFor.Time
	}

	if durationSeconds.Valid {
		duration := time.Duration(durationSeconds.Int64) * time.Second
		job.Duration = &duration
	}

	return job, nil
}

func (r *ConversionRepository) UpdateJob(job *models.ConversionJob) error {
	_, err := r.updateJob(job, nil)
	return err
}

// UpdateJobIf writes the job only while its stored status is one of from (an atomic
// UPDATE ... WHERE status IN (...)). It reports whether the row was written: false means the
// state changed underneath the caller (for example the owner cancelled a running job) and the
// stored state wins.
func (r *ConversionRepository) UpdateJobIf(job *models.ConversionJob, from ...string) (bool, error) {
	return r.updateJob(job, from)
}

func (r *ConversionRepository) updateJob(job *models.ConversionJob, from []string) (bool, error) {
	query := `
		UPDATE conversion_jobs
		SET status = ?, started_at = ?, completed_at = ?, duration = ?, error_message = ?
		WHERE id = ?
	`

	var startedAt, completedAt sql.NullTime
	var durationSeconds sql.NullInt64
	var errorMessage sql.NullString

	if job.StartedAt != nil {
		startedAt = sql.NullTime{Time: *job.StartedAt, Valid: true}
	}

	if job.CompletedAt != nil {
		completedAt = sql.NullTime{Time: *job.CompletedAt, Valid: true}
	}

	if job.Duration != nil {
		durationSeconds = sql.NullInt64{Int64: int64(job.Duration.Seconds()), Valid: true}
	}

	if job.ErrorMessage != nil {
		errorMessage = sql.NullString{String: *job.ErrorMessage, Valid: true}
	}

	args := []interface{}{job.Status, startedAt, completedAt, durationSeconds, errorMessage, job.ID}
	if len(from) > 0 {
		marks := make([]string, len(from))
		for i, f := range from {
			marks[i] = "?"
			args = append(args, f)
		}
		query += " AND status IN (" + strings.Join(marks, ",") + ")"
	}
	res, err := r.db.Exec(query, args...)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0 || len(from) == 0, nil
}

// GetDueJobs returns pending jobs that are due now (not scheduled for the future), highest
// priority first. The schedule filter is in SQL so future-scheduled rows cannot fill the window.
func (r *ConversionRepository) GetDueJobs(now time.Time, limit int) ([]models.ConversionJob, error) {
	rows, err := r.db.Query(`
		SELECT id, user_id, source_path, target_path, source_format, target_format,
			   conversion_type, quality, settings, priority, status, created_at,
			   started_at, completed_at, scheduled_for, duration, error_message
		FROM conversion_jobs
		WHERE status = ? AND (scheduled_for IS NULL OR scheduled_for <= ?)
		ORDER BY priority DESC, created_at ASC
		LIMIT ?`, models.ConversionStatusPending, now.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get due jobs: %w", err)
	}
	defer rows.Close()
	return r.scanJobs(rows)
}

// NormalizeLegacyScheduledFor rewrites pending rows whose scheduled_for text still carries a
// non-UTC offset (written before CreateJob normalised to UTC) to the UTC form. SQLite only: the
// stored text holds the original offset, so the rewrite is exact. On Postgres (TIMESTAMP without
// zone) the offset was dropped at write time and cannot be recovered, so nothing is done there.
// Idempotent: a second run finds nothing to rewrite. Rows whose text cannot be parsed are left
// alone and counted in skipped.
func (r *ConversionRepository) NormalizeLegacyScheduledFor() (rewritten, skipped int, err error) {
	if d := r.db.Dialect(); d == nil || !d.IsSQLite() {
		return 0, 0, nil
	}
	rows, err := r.db.Query(`SELECT id, CAST(scheduled_for AS TEXT) FROM conversion_jobs
		WHERE status = ? AND scheduled_for IS NOT NULL
		  AND CAST(scheduled_for AS TEXT) NOT LIKE '%+00:00' AND CAST(scheduled_for AS TEXT) NOT LIKE '%Z'`,
		models.ConversionStatusPending)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to scan legacy scheduled_for: %w", err)
	}
	type legacy struct {
		id  int
		txt string
	}
	var found []legacy
	for rows.Next() {
		var l legacy
		if err := rows.Scan(&l.id, &l.txt); err != nil {
			rows.Close()
			return 0, 0, err
		}
		found = append(found, l)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return 0, 0, err
	}
	rows.Close()
	layouts := []string{"2006-01-02 15:04:05.999999999-07:00", "2006-01-02T15:04:05.999999999-07:00",
		"2006-01-02 15:04:05.999999999 -0700 MST", "2006-01-02 15:04:05.999999999 -0700 -0700"}
	for _, l := range found {
		var t time.Time
		ok := false
		for _, layout := range layouts {
			if p, perr := time.Parse(layout, l.txt); perr == nil {
				t, ok = p, true
				break
			}
		}
		if !ok {
			skipped++
			continue
		}
		// conditional on the text still being what we read: a concurrent writer wins
		res, uerr := r.db.Exec(`UPDATE conversion_jobs SET scheduled_for = ? WHERE id = ? AND CAST(scheduled_for AS TEXT) = ?`,
			t.UTC(), l.id, l.txt)
		if uerr != nil {
			return rewritten, skipped, uerr
		}
		if n, _ := res.RowsAffected(); n == 1 {
			rewritten++
		}
	}
	return rewritten, skipped, nil
}

// ClaimJob atomically moves a pending job to running. It reports whether THIS caller won the
// claim, so two workers (or a worker and a retry) can never run one job twice.
func (r *ConversionRepository) ClaimJob(jobID int, startedAt time.Time) (bool, error) {
	res, err := r.db.Exec(`UPDATE conversion_jobs SET status = ?, started_at = ? WHERE id = ? AND status = ?`,
		models.ConversionStatusRunning, startedAt, jobID, models.ConversionStatusPending)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n == 1, nil
}

// RequeueRunning returns jobs left in `running` (a previous process died or was stopped mid-job)
// to `pending`. Only valid while this process is the sole worker for the database.
func (r *ConversionRepository) RequeueRunning() (int64, error) {
	res, err := r.db.Exec(`UPDATE conversion_jobs SET status = ?, started_at = NULL WHERE status = ?`,
		models.ConversionStatusPending, models.ConversionStatusRunning)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *ConversionRepository) GetUserJobs(userID int, status *string, limit, offset int) ([]models.ConversionJob, error) {
	whereClause := "WHERE user_id = ?"
	args := []interface{}{userID}

	if status != nil {
		whereClause += " AND status = ?"
		args = append(args, *status)
	}

	query := fmt.Sprintf(`
		SELECT id, user_id, source_path, target_path, source_format, target_format,
			   conversion_type, quality, settings, priority, status, created_at,
			   started_at, completed_at, scheduled_for, duration, error_message
		FROM conversion_jobs
		%s
		ORDER BY created_at DESC
		LIMIT ? OFFSET ?
	`, whereClause)

	args = append(args, limit, offset)

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get user jobs: %w", err)
	}
	defer rows.Close()

	return r.scanJobs(rows)
}

func (r *ConversionRepository) GetJobsByStatus(status string, limit, offset int) ([]models.ConversionJob, error) {
	query := `
		SELECT id, user_id, source_path, target_path, source_format, target_format,
			   conversion_type, quality, settings, priority, status, created_at,
			   started_at, completed_at, scheduled_for, duration, error_message
		FROM conversion_jobs
		WHERE status = ?
		ORDER BY priority DESC, created_at ASC
		LIMIT ? OFFSET ?
	`

	rows, err := r.db.Query(query, status, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get jobs by status: %w", err)
	}
	defer rows.Close()

	return r.scanJobs(rows)
}

func (r *ConversionRepository) GetStatistics(userID *int, startDate, endDate time.Time) (*models.ConversionStatistics, error) {
	whereClause := "WHERE created_at BETWEEN ? AND ?"
	args := []interface{}{startDate, endDate}

	if userID != nil {
		whereClause += " AND user_id = ?"
		args = append(args, *userID)
	}

	// Get total counts by status
	statusQuery := fmt.Sprintf(`
		SELECT status, COUNT(*) as count
		FROM conversion_jobs
		%s
		GROUP BY status
	`, whereClause)

	statusRows, err := r.db.Query(statusQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get status statistics: %w", err)
	}
	defer statusRows.Close()

	stats := &models.ConversionStatistics{
		StartDate: startDate,
		EndDate:   endDate,
		ByStatus:  make(map[string]int),
		ByType:    make(map[string]int),
		ByFormat:  make(map[string]int),
	}

	for statusRows.Next() {
		var status string
		var count int
		err := statusRows.Scan(&status, &count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan status stats: %w", err)
		}
		stats.ByStatus[status] = count
		stats.TotalJobs += count
	}

	// Get counts by conversion type
	typeQuery := fmt.Sprintf(`
		SELECT conversion_type, COUNT(*) as count
		FROM conversion_jobs
		%s
		GROUP BY conversion_type
	`, whereClause)

	typeRows, err := r.db.Query(typeQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get type statistics: %w", err)
	}
	defer typeRows.Close()

	for typeRows.Next() {
		var convType string
		var count int
		err := typeRows.Scan(&convType, &count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan type stats: %w", err)
		}
		stats.ByType[convType] = count
	}

	// Get counts by target format
	formatQuery := fmt.Sprintf(`
		SELECT target_format, COUNT(*) as count
		FROM conversion_jobs
		%s
		GROUP BY target_format
		ORDER BY count DESC
		LIMIT 10
	`, whereClause)

	formatRows, err := r.db.Query(formatQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get format statistics: %w", err)
	}
	defer formatRows.Close()

	for formatRows.Next() {
		var format string
		var count int
		err := formatRows.Scan(&format, &count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan format stats: %w", err)
		}
		stats.ByFormat[format] = count
	}

	// Get average duration for completed jobs
	durationQuery := fmt.Sprintf(`
		SELECT AVG(duration) as avg_duration
		FROM conversion_jobs
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
	if completed, ok := stats.ByStatus[models.ConversionStatusCompleted]; ok {
		if failed, failedOk := stats.ByStatus[models.ConversionStatusFailed]; failedOk {
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

func (r *ConversionRepository) CleanupJobs(olderThan time.Time) error {
	query := `
		DELETE FROM conversion_jobs
		WHERE completed_at < ? AND status IN ('completed', 'failed', 'cancelled')
	`

	_, err := r.db.Exec(query, olderThan)
	return err
}

func (r *ConversionRepository) GetActiveJobsCount() (int, error) {
	query := `SELECT COUNT(*) FROM conversion_jobs WHERE status IN ('pending', 'running')`
	var count int
	err := r.db.QueryRow(query).Scan(&count)
	return count, err
}

func (r *ConversionRepository) GetJobsCountByUser(userID int) (int, error) {
	query := `SELECT COUNT(*) FROM conversion_jobs WHERE user_id = ?`
	var count int
	err := r.db.QueryRow(query, userID).Scan(&count)
	return count, err
}

func (r *ConversionRepository) GetPopularFormats(limit int) ([]models.FormatPopularity, error) {
	query := `
		SELECT target_format, COUNT(*) as count
		FROM conversion_jobs
		WHERE status = 'completed'
		GROUP BY target_format
		ORDER BY count DESC
		LIMIT ?
	`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get popular formats: %w", err)
	}
	defer rows.Close()

	formats := []models.FormatPopularity{}
	for rows.Next() {
		var format models.FormatPopularity
		err := rows.Scan(&format.Format, &format.Count)
		if err != nil {
			return nil, fmt.Errorf("failed to scan format popularity: %w", err)
		}
		formats = append(formats, format)
	}

	return formats, nil
}

func (r *ConversionRepository) scanJobs(rows *sql.Rows) ([]models.ConversionJob, error) {
	jobs := []models.ConversionJob{} // never nil: an empty list must marshal as [], not null

	for rows.Next() {
		var job models.ConversionJob
		var settings, errorMessage sql.NullString
		var startedAt, completedAt, scheduledFor sql.NullTime
		var durationSeconds sql.NullInt64

		err := rows.Scan(
			&job.ID, &job.UserID, &job.SourcePath, &job.TargetPath, &job.SourceFormat, &job.TargetFormat,
			&job.ConversionType, &job.Quality, &settings, &job.Priority, &job.Status, &job.CreatedAt,
			&startedAt, &completedAt, &scheduledFor, &durationSeconds, &errorMessage)

		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}

		if settings.Valid {
			job.Settings = &settings.String
		}

		if errorMessage.Valid {
			job.ErrorMessage = &errorMessage.String
		}

		if startedAt.Valid {
			job.StartedAt = &startedAt.Time
		}

		if completedAt.Valid {
			job.CompletedAt = &completedAt.Time
		}

		if scheduledFor.Valid {
			job.ScheduledFor = &scheduledFor.Time
		}

		if durationSeconds.Valid {
			duration := time.Duration(durationSeconds.Int64) * time.Second
			job.Duration = &duration
		}

		jobs = append(jobs, job)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to iterate jobs: %w", err)
	}

	return jobs, nil
}
