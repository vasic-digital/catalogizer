package repository

import (
	"database/sql"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockConversionRepo(t *testing.T) (*ConversionRepository, sqlmock.Sqlmock) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	return NewConversionRepository(db), mock
}

var conversionJobColumns = []string{
	"id", "user_id", "source_path", "target_path", "source_format", "target_format",
	"conversion_type", "quality", "settings", "priority", "status", "created_at",
	"started_at", "completed_at", "scheduled_for", "duration", "error_message", "warning",
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

func TestConversionRepository_Constructor(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	repo := NewConversionRepository(db)
	assert.NotNil(t, repo)
}

// ---------------------------------------------------------------------------
// CreateJob
// ---------------------------------------------------------------------------

func TestConversionRepository_CreateJob(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		job     *models.ConversionJob
		setup   func(mock sqlmock.Sqlmock)
		wantID  int
		wantErr bool
	}{
		{
			name: "success",
			job: &models.ConversionJob{
				UserID:         1,
				SourcePath:     "/media/video.avi",
				TargetPath:     "/media/video.mp4",
				SourceFormat:   "avi",
				TargetFormat:   "mp4",
				ConversionType: "video",
				Quality:        "high",
				Priority:       1,
				Status:         "pending",
				CreatedAt:      now,
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO conversion_jobs").
					WithArgs(1, "/media/video.avi", "/media/video.mp4", "avi", "mp4",
						"video", "high", nil, 1, "pending", now, nil).
					WillReturnResult(sqlmock.NewResult(42, 1))
			},
			wantID: 42,
		},
		{
			name: "database error",
			job: &models.ConversionJob{
				UserID:    1,
				Status:    "pending",
				CreatedAt: now,
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO conversion_jobs").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockConversionRepo(t)
			tt.setup(mock)

			id, err := repo.CreateJob(tt.job)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantID, id)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetJob
// ---------------------------------------------------------------------------

func TestConversionRepository_GetJob(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		jobID   int
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
		check   func(t *testing.T, job *models.ConversionJob)
	}{
		{
			name:  "success",
			jobID: 1,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(conversionJobColumns).
					AddRow(1, 1, "/src.avi", "/tgt.mp4", "avi", "mp4",
						"video", "high", nil, 1, "completed", now,
						now, now, nil, int64(120), nil, nil)
				mock.ExpectQuery("SELECT .+ FROM conversion_jobs WHERE id").
					WithArgs(1).
					WillReturnRows(rows)
			},
			check: func(t *testing.T, job *models.ConversionJob) {
				assert.Equal(t, 1, job.ID)
				assert.Equal(t, "completed", job.Status)
				assert.NotNil(t, job.Duration)
			},
		},
		{
			name:  "not found",
			jobID: 999,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM conversion_jobs WHERE id").
					WithArgs(999).
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: true,
		},
		{
			name:  "database error",
			jobID: 1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM conversion_jobs WHERE id").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockConversionRepo(t)
			tt.setup(mock)

			job, err := repo.GetJob(tt.jobID)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, job)
			tt.check(t, job)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// UpdateJob
// ---------------------------------------------------------------------------

func TestConversionRepository_UpdateJob(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		job     *models.ConversionJob
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			job: &models.ConversionJob{
				ID:        1,
				Status:    "completed",
				StartedAt: &now,
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE conversion_jobs").
					WithArgs("completed", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name: "database error",
			job:  &models.ConversionJob{ID: 1, Status: "failed"},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("UPDATE conversion_jobs").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockConversionRepo(t)
			tt.setup(mock)

			err := repo.UpdateJob(tt.job)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetJobsByStatus
// ---------------------------------------------------------------------------

func TestConversionRepository_GetJobsByStatus(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		status  string
		setup   func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name:   "returns jobs",
			status: "pending",
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(conversionJobColumns).
					AddRow(1, 1, "/src.avi", "/tgt.mp4", "avi", "mp4",
						"video", "high", nil, 1, "pending", now,
						nil, nil, nil, nil, nil, nil)
				mock.ExpectQuery("SELECT .+ FROM conversion_jobs WHERE status").
					WithArgs("pending", 10, 0).
					WillReturnRows(rows)
			},
			want: 1,
		},
		{
			name:   "empty result",
			status: "running",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM conversion_jobs WHERE status").
					WithArgs("running", 10, 0).
					WillReturnRows(sqlmock.NewRows(conversionJobColumns))
			},
			want: 0,
		},
		{
			name:   "database error",
			status: "pending",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM conversion_jobs WHERE status").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockConversionRepo(t)
			tt.setup(mock)

			jobs, err := repo.GetJobsByStatus(tt.status, 10, 0)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, jobs, tt.want)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetActiveJobsCount
// ---------------------------------------------------------------------------

func TestConversionRepository_GetActiveJobsCount(t *testing.T) {
	repo, mock := newMockConversionRepo(t)
	mock.ExpectQuery("SELECT COUNT").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))

	count, err := repo.GetActiveJobsCount()
	require.NoError(t, err)
	assert.Equal(t, 5, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetJobsCountByUser
// ---------------------------------------------------------------------------

func TestConversionRepository_GetJobsCountByUser(t *testing.T) {
	repo, mock := newMockConversionRepo(t)
	mock.ExpectQuery("SELECT COUNT").
		WithArgs(1).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(8))

	count, err := repo.GetJobsCountByUser(1)
	require.NoError(t, err)
	assert.Equal(t, 8, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// CleanupJobs
// ---------------------------------------------------------------------------

func TestConversionRepository_CleanupJobs(t *testing.T) {
	olderThan := time.Now().Add(-30 * 24 * time.Hour)

	tests := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM conversion_jobs").
					WithArgs(olderThan).
					WillReturnResult(sqlmock.NewResult(0, 10))
			},
		},
		{
			name: "database error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM conversion_jobs").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockConversionRepo(t)
			tt.setup(mock)

			err := repo.CleanupJobs(olderThan)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetPopularFormats
// ---------------------------------------------------------------------------

func TestConversionRepository_GetPopularFormats(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name: "returns formats",
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"target_format", "count"}).
					AddRow("mp4", 100).
					AddRow("mp3", 50)
				mock.ExpectQuery("SELECT target_format, COUNT").
					WithArgs(10).
					WillReturnRows(rows)
			},
			want: 2,
		},
		{
			name: "database error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT target_format, COUNT").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockConversionRepo(t)
			tt.setup(mock)

			formats, err := repo.GetPopularFormats(10)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, formats, tt.want)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetUserJobs
// ---------------------------------------------------------------------------

func TestConversionRepository_GetUserJobs(t *testing.T) {
	now := time.Now()

	t.Run("without status filter", func(t *testing.T) {
		repo, mock := newMockConversionRepo(t)
		rows := sqlmock.NewRows(conversionJobColumns).
			AddRow(1, 1, "/src.avi", "/tgt.mp4", "avi", "mp4",
				"video", "high", nil, 1, "pending", now,
				nil, nil, nil, nil, nil, nil)
		mock.ExpectQuery("SELECT .+ FROM conversion_jobs").
			WithArgs(1, 10, 0).
			WillReturnRows(rows)

		jobs, err := repo.GetUserJobs(1, nil, 10, 0)
		require.NoError(t, err)
		assert.Len(t, jobs, 1)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("with status filter", func(t *testing.T) {
		repo, mock := newMockConversionRepo(t)
		status := "completed"
		rows := sqlmock.NewRows(conversionJobColumns).
			AddRow(1, 1, "/src.avi", "/tgt.mp4", "avi", "mp4",
				"video", "high", nil, 1, "completed", now,
				now, now, nil, int64(120), nil, nil)
		mock.ExpectQuery("SELECT .+ FROM conversion_jobs").
			WithArgs(1, "completed", 10, 0).
			WillReturnRows(rows)

		jobs, err := repo.GetUserJobs(1, &status, 10, 0)
		require.NoError(t, err)
		assert.Len(t, jobs, 1)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
