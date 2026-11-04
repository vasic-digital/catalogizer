package repository

import (
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockAnalyticsRepo(t *testing.T) (*AnalyticsRepository, sqlmock.Sqlmock) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	return NewAnalyticsRepository(db), mock
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

func TestAnalyticsRepository_Constructor(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	repo := NewAnalyticsRepository(db)
	assert.NotNil(t, repo)
}

// ---------------------------------------------------------------------------
// LogMediaAccess
// ---------------------------------------------------------------------------

func TestAnalyticsRepository_LogMediaAccess(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		access  *models.MediaAccessLog
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success without optional fields",
			access: &models.MediaAccessLog{
				UserID:     1,
				MediaID:    42,
				Action:     "play",
				AccessTime: now,
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO media_access_logs").
					WithArgs(1, 42, "play", nil, nil, nil, nil, nil, now).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name: "success with device info and location",
			access: &models.MediaAccessLog{
				UserID:     1,
				MediaID:    42,
				Action:     "play",
				DeviceInfo: &models.DeviceInfo{},
				Location:   &models.Location{Latitude: 40.7, Longitude: -74.0},
				AccessTime: now,
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO media_access_logs").
					WithArgs(1, 42, "play", sqlmock.AnyArg(), sqlmock.AnyArg(), nil, nil, nil, now).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name: "database error",
			access: &models.MediaAccessLog{
				UserID:     1,
				MediaID:    42,
				Action:     "play",
				AccessTime: now,
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO media_access_logs").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockAnalyticsRepo(t)
			tt.setup(mock)

			err := repo.LogMediaAccess(tt.access)
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
// LogEvent
// ---------------------------------------------------------------------------

func TestAnalyticsRepository_LogEvent(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		event   *models.AnalyticsEvent
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			event: &models.AnalyticsEvent{
				UserID:        1,
				EventType:     "media_access",
				EventCategory: "playback",
				Data:          `{"file_type":"video"}`,
				Timestamp:     now,
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO analytics_events").
					WithArgs(1, "media_access", "playback", `{"file_type":"video"}`, nil, nil, nil, nil, now).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name: "database error",
			event: &models.AnalyticsEvent{
				UserID:    1,
				EventType: "test",
				Timestamp: now,
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO analytics_events").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockAnalyticsRepo(t)
			tt.setup(mock)

			err := repo.LogEvent(tt.event)
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
// GetMediaAccessLogs
// ---------------------------------------------------------------------------

func TestAnalyticsRepository_GetMediaAccessLogs(t *testing.T) {
	now := time.Now()
	accessLogColumns := []string{
		"id", "user_id", "media_id", "action", "device_info", "location",
		"ip_address", "user_agent", "playback_duration", "access_time",
	}

	tests := []struct {
		name    string
		userID  int
		mediaID *int
		limit   int
		offset  int
		setup   func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name:   "all logs with no filters",
			userID: 0,
			limit:  10,
			offset: 0,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(accessLogColumns).
					AddRow(1, 1, 42, "play", nil, nil, nil, nil, nil, now)
				mock.ExpectQuery("SELECT .+ FROM media_access_logs").
					WithArgs(10, 0).
					WillReturnRows(rows)
			},
			want: 1,
		},
		{
			name:   "filter by user ID",
			userID: 1,
			limit:  10,
			offset: 0,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows(accessLogColumns).
					AddRow(1, 1, 42, "play", nil, nil, nil, nil, nil, now)
				mock.ExpectQuery("SELECT .+ FROM media_access_logs").
					WithArgs(1, 10, 0).
					WillReturnRows(rows)
			},
			want: 1,
		},
		{
			name:   "database error",
			userID: 0,
			limit:  10,
			offset: 0,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT .+ FROM media_access_logs").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockAnalyticsRepo(t)
			tt.setup(mock)

			logs, err := repo.GetMediaAccessLogs(tt.userID, tt.mediaID, tt.limit, tt.offset)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, logs, tt.want)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetTotalUsers
// ---------------------------------------------------------------------------

func TestAnalyticsRepository_GetTotalUsers(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name: "returns count",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(25))
			},
			wantCount: 25,
		},
		{
			name: "database error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockAnalyticsRepo(t)
			tt.setup(mock)

			count, err := repo.GetTotalUsers()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantCount, count)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetActiveUsers
// ---------------------------------------------------------------------------

func TestAnalyticsRepository_GetActiveUsers(t *testing.T) {
	startDate := time.Now().Add(-24 * time.Hour)
	endDate := time.Now()

	tests := []struct {
		name      string
		setup     func(mock sqlmock.Sqlmock)
		wantCount int
		wantErr   bool
	}{
		{
			name: "returns active users count",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT\\(DISTINCT user_id\\)").
					WithArgs(startDate, endDate).
					WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))
			},
			wantCount: 10,
		},
		{
			name: "database error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT COUNT\\(DISTINCT user_id\\)").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockAnalyticsRepo(t)
			tt.setup(mock)

			count, err := repo.GetActiveUsers(startDate, endDate)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantCount, count)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetTotalMediaAccesses
// ---------------------------------------------------------------------------

func TestAnalyticsRepository_GetTotalMediaAccesses(t *testing.T) {
	startDate := time.Now().Add(-24 * time.Hour)
	endDate := time.Now()

	repo, mock := newMockAnalyticsRepo(t)
	mock.ExpectQuery("SELECT COUNT").
		WithArgs(startDate, endDate).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(100))

	count, err := repo.GetTotalMediaAccesses(startDate, endDate)
	require.NoError(t, err)
	assert.Equal(t, 100, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetTotalEvents
// ---------------------------------------------------------------------------

func TestAnalyticsRepository_GetTotalEvents(t *testing.T) {
	startDate := time.Now().Add(-24 * time.Hour)
	endDate := time.Now()

	repo, mock := newMockAnalyticsRepo(t)
	mock.ExpectQuery("SELECT COUNT").
		WithArgs(startDate, endDate).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(50))

	count, err := repo.GetTotalEvents(startDate, endDate)
	require.NoError(t, err)
	assert.Equal(t, 50, count)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetTopAccessedMedia
// ---------------------------------------------------------------------------

func TestAnalyticsRepository_GetTopAccessedMedia(t *testing.T) {
	startDate := time.Now().Add(-24 * time.Hour)
	endDate := time.Now()

	tests := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name: "returns top accessed media",
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"media_id", "access_count"}).
					AddRow(42, 100).
					AddRow(43, 50)
				mock.ExpectQuery("SELECT media_id, COUNT").
					WithArgs(startDate, endDate, 10).
					WillReturnRows(rows)
			},
			want: 2,
		},
		{
			name: "empty result",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT media_id, COUNT").
					WithArgs(startDate, endDate, 10).
					WillReturnRows(sqlmock.NewRows([]string{"media_id", "access_count"}))
			},
			want: 0,
		},
		{
			name: "database error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT media_id, COUNT").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockAnalyticsRepo(t)
			tt.setup(mock)

			results, err := repo.GetTopAccessedMedia(startDate, endDate, 10)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, results, tt.want)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// An empty result must be a non-nil slice: it is serialised into the
// /analytics/system response as "top_accessed_media" and a JSON null there
// broke catalog-web (clients iterate the field).
func TestAnalyticsRepository_GetTopAccessedMedia_EmptyIsNonNilSlice(t *testing.T) {
	startDate := time.Now().Add(-24 * time.Hour)
	endDate := time.Now()
	repo, mock := newMockAnalyticsRepo(t)
	mock.ExpectQuery("SELECT media_id, COUNT").
		WithArgs(startDate, endDate, 10).
		WillReturnRows(sqlmock.NewRows([]string{"media_id", "access_count"}))

	results, err := repo.GetTopAccessedMedia(startDate, endDate, 10)
	require.NoError(t, err)
	assert.NotNil(t, results, "empty result must be [] (non-nil), not nil")
	b, err := json.Marshal(results)
	require.NoError(t, err)
	assert.Equal(t, "[]", string(b))
}

// ---------------------------------------------------------------------------
// GetFileTypeData
// ---------------------------------------------------------------------------

func TestAnalyticsRepository_GetFileTypeData(t *testing.T) {
	startDate := time.Now().Add(-24 * time.Hour)
	endDate := time.Now()

	tests := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock)
		want    map[string]int
		wantErr bool
	}{
		{
			name: "returns file type data",
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"file_type", "count"}).
					AddRow("video", 50).
					AddRow("audio", 30)
				mock.ExpectQuery("SELECT").
					WithArgs(startDate, endDate).
					WillReturnRows(rows)
			},
			want: map[string]int{"video": 50, "audio": 30},
		},
		{
			name: "database error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockAnalyticsRepo(t)
			tt.setup(mock)

			result, err := repo.GetFileTypeData(startDate, endDate)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetSessionData
// ---------------------------------------------------------------------------

func TestAnalyticsRepository_GetSessionData(t *testing.T) {
	startDate := time.Now().Add(-24 * time.Hour)
	endDate := time.Now()
	sessionStart := time.Now().Add(-2 * time.Hour)
	sessionEnd := time.Now().Add(-1 * time.Hour)

	tests := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name: "returns session data",
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"user_id", "session_start", "session_end", "duration_seconds"}).
					AddRow(1, sessionStart, sessionEnd, 3600.0)
				mock.ExpectQuery("SELECT user_id").
					WithArgs(startDate, endDate).
					WillReturnRows(rows)
			},
			want: 1,
		},
		{
			name: "database error",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT user_id").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockAnalyticsRepo(t)
			tt.setup(mock)

			results, err := repo.GetSessionData(startDate, endDate)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, results, tt.want)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestAnalyticsRepository_GetSessionData_PostgreSQLDialect(t *testing.T) {
	startDate := time.Now().Add(-24 * time.Hour)
	endDate := time.Now()
	sessionStart := time.Now().Add(-2 * time.Hour)
	sessionEnd := time.Now().Add(-1 * time.Hour)

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := database.WrapDB(sqlDB, database.DialectPostgres)
	repo := NewAnalyticsRepository(db)

	rows := sqlmock.NewRows([]string{"user_id", "session_start", "session_end", "duration_seconds"}).
		AddRow(1, sessionStart, sessionEnd, 3600.0)
	mock.ExpectQuery("SELECT user_id").
		WithArgs(startDate, endDate).
		WillReturnRows(rows)

	results, err := repo.GetSessionData(startDate, endDate)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, 1, results[0].UserID)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ===========================================================================
// Real SQLite-backed tests for uncovered functions
// ===========================================================================

func newRealAnalyticsRepo(t *testing.T) *AnalyticsRepository {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	db := database.WrapDB(sqlDB, database.DialectSQLite)

	_, err = sqlDB.Exec(`
		CREATE TABLE media_access_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			media_id INTEGER NOT NULL,
			action TEXT NOT NULL,
			device_info TEXT,
			location TEXT,
			ip_address TEXT,
			user_agent TEXT,
			playback_duration INTEGER,
			access_time DATETIME NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`
		CREATE TABLE analytics_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			event_type TEXT NOT NULL,
			event_category TEXT NOT NULL,
			data TEXT,
			device_info TEXT,
			location TEXT,
			ip_address TEXT,
			user_agent TEXT,
			timestamp DATETIME NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT NOT NULL,
			created_at DATETIME NOT NULL
		)
	`)
	require.NoError(t, err)

	return NewAnalyticsRepository(db)
}

func seedAnalyticsData(t *testing.T, repo *AnalyticsRepository) time.Time {
	t.Helper()
	now := time.Now().Truncate(time.Second)

	// Seed users
	_, err := repo.db.Exec(`INSERT INTO users (username, created_at) VALUES (?, ?)`, "alice", now.Add(-24*time.Hour))
	require.NoError(t, err)
	_, err = repo.db.Exec(`INSERT INTO users (username, created_at) VALUES (?, ?)`, "bob", now.Add(-12*time.Hour))
	require.NoError(t, err)

	// Seed media access logs
	logs := []models.MediaAccessLog{
		{UserID: 1, MediaID: 10, Action: "play", AccessTime: now.Add(-1 * time.Hour)},
		{UserID: 1, MediaID: 11, Action: "view", AccessTime: now.Add(-2 * time.Hour)},
		{UserID: 2, MediaID: 10, Action: "play", AccessTime: now.Add(-3 * time.Hour)},
	}
	for _, log := range logs {
		err := repo.LogMediaAccess(&log)
		require.NoError(t, err)
	}

	// Seed analytics events
	events := []models.AnalyticsEvent{
		{UserID: 1, EventType: "media_access", EventCategory: "playback", Data: `{"file_type":"video"}`, Timestamp: now.Add(-1 * time.Hour)},
		{UserID: 1, EventType: "search", EventCategory: "navigation", Data: `{"query":"test"}`, Timestamp: now.Add(-2 * time.Hour)},
		{UserID: 2, EventType: "media_access", EventCategory: "playback", Data: `{"file_type":"audio"}`, Timestamp: now.Add(-3 * time.Hour)},
	}
	for _, event := range events {
		err := repo.LogEvent(&event)
		require.NoError(t, err)
	}

	return now
}

// ---------------------------------------------------------------------------
// GetUserMediaAccessLogs
// ---------------------------------------------------------------------------

func TestAnalyticsRepository_GetUserMediaAccessLogs_Real(t *testing.T) {
	repo := newRealAnalyticsRepo(t)
	now := seedAnalyticsData(t, repo)

	startDate := now.Add(-5 * time.Hour)
	endDate := now.Add(time.Hour)

	t.Run("returns logs for user 1", func(t *testing.T) {
		logs, err := repo.GetUserMediaAccessLogs(1, startDate, endDate)
		require.NoError(t, err)
		assert.Len(t, logs, 2)
		for _, log := range logs {
			assert.Equal(t, 1, log.UserID)
		}
	})

	t.Run("returns logs for user 2", func(t *testing.T) {
		logs, err := repo.GetUserMediaAccessLogs(2, startDate, endDate)
		require.NoError(t, err)
		assert.Len(t, logs, 1)
		assert.Equal(t, 2, logs[0].UserID)
	})

	t.Run("empty for nonexistent user", func(t *testing.T) {
		logs, err := repo.GetUserMediaAccessLogs(999, startDate, endDate)
		require.NoError(t, err)
		assert.Empty(t, logs)
	})

	t.Run("empty for narrow date range", func(t *testing.T) {
		logs, err := repo.GetUserMediaAccessLogs(1, now.Add(-10*time.Hour), now.Add(-9*time.Hour))
		require.NoError(t, err)
		assert.Empty(t, logs)
	})
}

// ---------------------------------------------------------------------------
// GetUserEvents
// ---------------------------------------------------------------------------

func TestAnalyticsRepository_GetUserEvents_Real(t *testing.T) {
	repo := newRealAnalyticsRepo(t)
	now := seedAnalyticsData(t, repo)

	startDate := now.Add(-5 * time.Hour)
	endDate := now.Add(time.Hour)

	t.Run("returns events for user 1", func(t *testing.T) {
		events, err := repo.GetUserEvents(1, startDate, endDate)
		require.NoError(t, err)
		assert.Len(t, events, 2)
		for _, event := range events {
			assert.Equal(t, 1, event.UserID)
		}
	})

	t.Run("returns events for user 2", func(t *testing.T) {
		events, err := repo.GetUserEvents(2, startDate, endDate)
		require.NoError(t, err)
		assert.Len(t, events, 1)
	})

	t.Run("empty for nonexistent user", func(t *testing.T) {
		events, err := repo.GetUserEvents(999, startDate, endDate)
		require.NoError(t, err)
		assert.Empty(t, events)
	})
}

// ---------------------------------------------------------------------------
// GetUserGrowthData
// ---------------------------------------------------------------------------

func TestAnalyticsRepository_GetUserGrowthData_Real(t *testing.T) {
	repo := newRealAnalyticsRepo(t)
	now := seedAnalyticsData(t, repo)

	startDate := now.Add(-48 * time.Hour)
	endDate := now.Add(time.Hour)

	t.Run("returns growth data", func(t *testing.T) {
		data, err := repo.GetUserGrowthData(startDate, endDate)
		require.NoError(t, err)
		assert.NotEmpty(t, data)
		totalUsers := 0
		for _, point := range data {
			totalUsers += point.UserCount
		}
		assert.Equal(t, 2, totalUsers)
	})

	t.Run("empty for future date range", func(t *testing.T) {
		futureStart := now.Add(24 * time.Hour)
		futureEnd := now.Add(48 * time.Hour)
		data, err := repo.GetUserGrowthData(futureStart, futureEnd)
		require.NoError(t, err)
		assert.Empty(t, data)
	})
}

// ---------------------------------------------------------------------------
// GetAllMediaAccessLogs
// ---------------------------------------------------------------------------

func TestAnalyticsRepository_GetAllMediaAccessLogs_Real(t *testing.T) {
	repo := newRealAnalyticsRepo(t)
	now := seedAnalyticsData(t, repo)

	startDate := now.Add(-5 * time.Hour)
	endDate := now.Add(time.Hour)

	t.Run("returns all logs in range", func(t *testing.T) {
		logs, err := repo.GetAllMediaAccessLogs(startDate, endDate)
		require.NoError(t, err)
		assert.Len(t, logs, 3)
	})

	t.Run("returns subset for narrow range", func(t *testing.T) {
		narrowStart := now.Add(-90 * time.Minute)
		narrowEnd := now.Add(-30 * time.Minute)
		logs, err := repo.GetAllMediaAccessLogs(narrowStart, narrowEnd)
		require.NoError(t, err)
		assert.Len(t, logs, 1)
	})

	t.Run("empty for out-of-range", func(t *testing.T) {
		logs, err := repo.GetAllMediaAccessLogs(now.Add(-20*time.Hour), now.Add(-10*time.Hour))
		require.NoError(t, err)
		assert.Empty(t, logs)
	})
}

// ---------------------------------------------------------------------------
// GetGeographicData
// ---------------------------------------------------------------------------

func TestAnalyticsRepository_GetGeographicData_Real(t *testing.T) {
	repo := newRealAnalyticsRepo(t)

	now := time.Now().Truncate(time.Second)
	country := "US"
	city := "New York"
	location := models.Location{Latitude: 40.7, Longitude: -74.0, Country: &country, City: &city}
	locationJSON, _ := json.Marshal(location)

	_, err := repo.db.Exec(
		`INSERT INTO media_access_logs (user_id, media_id, action, location, access_time) VALUES (?, ?, ?, ?, ?)`,
		1, 10, "play", string(locationJSON), now,
	)
	require.NoError(t, err)

	_, err = repo.db.Exec(
		`INSERT INTO media_access_logs (user_id, media_id, action, location, access_time) VALUES (?, ?, ?, ?, ?)`,
		2, 11, "view", string(locationJSON), now.Add(-time.Hour),
	)
	require.NoError(t, err)

	startDate := now.Add(-2 * time.Hour)
	endDate := now.Add(time.Hour)

	t.Run("returns geographic data", func(t *testing.T) {
		result, err := repo.GetGeographicData(startDate, endDate)
		require.NoError(t, err)
		assert.NotNil(t, result)

		locations, ok := result["locations"].([]map[string]interface{})
		assert.True(t, ok)
		assert.NotEmpty(t, locations)

		countries, ok := result["countries"].(map[string]int)
		assert.True(t, ok)
		assert.Equal(t, 2, countries["US"])
	})

	t.Run("empty for no-location data range", func(t *testing.T) {
		result, err := repo.GetGeographicData(now.Add(-20*time.Hour), now.Add(-10*time.Hour))
		require.NoError(t, err)
		assert.NotNil(t, result)
		locations := result["locations"]
		// nil would serialise as JSON null where analytics/system documents an array
		assert.NotNil(t, locations)
		assert.Empty(t, locations)
	})
}
