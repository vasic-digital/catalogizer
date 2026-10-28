package services

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// syncTestDBCounter gives each newSyncTestDB invocation a unique
// shared-cache identity so parallel tests don't collide on the
// same in-memory DB name.
var syncTestDBCounter uint64

// newSyncTestDB creates an in-memory SQLite database with sync
// tables for testing.
//
// DEFER-QA-2026-04-22-001 fix — the database uses SQLite's
// shared-cache in-memory mode (`file:syncdb-<n>?mode=memory&cache=shared`)
// rather than the plain `:memory:` DSN. Under `go test -race -p 2`
// SyncService.StartSync spawns a goroutine that holds one connection
// while the test's next StartSync call opens another; with `:memory:`
// each connection gets its OWN private in-memory DB, and the second
// call fails with "no such table: sync_sessions". The shared-cache
// mode wires all connections that name the same file: URI to the
// same schema + data.
//
// MaxOpenConns is clamped to 1 so SQLite's writer-lock semantics
// don't surface spurious "database is locked" under concurrent writes
// (all write paths serialize; reads are rare in these tests).
func newSyncTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()
	id := atomic.AddUint64(&syncTestDBCounter, 1)
	dsn := fmt.Sprintf(
		"file:syncdb-%d-%d?mode=memory&cache=shared",
		time.Now().UnixNano(), id,
	)
	rawDB, err := sql.Open("sqlite3", dsn)
	require.NoError(t, err)
	rawDB.SetMaxOpenConns(1)

	db := database.WrapDB(rawDB, database.DialectSQLite)
	require.NotNil(t, db)

	// Create sync tables
	schema := `
	CREATE TABLE IF NOT EXISTS sync_endpoints (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		name TEXT NOT NULL,
		type TEXT NOT NULL,
		url TEXT NOT NULL,
		username TEXT DEFAULT '',
		password TEXT DEFAULT '',
		sync_direction TEXT NOT NULL,
		local_path TEXT NOT NULL,
		remote_path TEXT DEFAULT '',
		sync_settings TEXT,
		status TEXT NOT NULL DEFAULT 'active',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		last_sync_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS sync_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		endpoint_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL,
		status TEXT NOT NULL,
		sync_type TEXT NOT NULL,
		started_at DATETIME NOT NULL,
		completed_at DATETIME,
		duration INTEGER,
		total_files INTEGER DEFAULT 0,
		synced_files INTEGER DEFAULT 0,
		failed_files INTEGER DEFAULT 0,
		skipped_files INTEGER DEFAULT 0,
		error_message TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (endpoint_id) REFERENCES sync_endpoints(id)
	);

	CREATE TABLE IF NOT EXISTS sync_schedules (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		endpoint_id INTEGER NOT NULL,
		user_id INTEGER NOT NULL,
		frequency TEXT NOT NULL,
		last_run DATETIME,
		next_run DATETIME,
		is_active BOOLEAN DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (endpoint_id) REFERENCES sync_endpoints(id)
	);
	`
	_, err = rawDB.Exec(schema)
	require.NoError(t, err)

	cleanup := func() {
		rawDB.Close()
	}

	return db, cleanup
}

// ---------------------------------------------------------------------------
// SyncRepository — CreateEndpoint / GetEndpoint / UpdateEndpoint / DeleteEndpoint
// ---------------------------------------------------------------------------

func TestSyncRepository_CreateAndGetEndpoint(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)

	endpoint := &models.SyncEndpoint{
		UserID:        1,
		Name:          "Test WebDAV",
		Type:          models.SyncTypeWebDAV,
		URL:           "https://example.com/webdav",
		Username:      "testuser",
		Password:      "testpass",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data/local",
		RemotePath:    "/data/remote",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	id, err := repo.CreateEndpoint(endpoint)
	require.NoError(t, err)
	assert.Greater(t, id, 0)

	// Retrieve
	got, err := repo.GetEndpoint(id)
	require.NoError(t, err)
	assert.Equal(t, id, got.ID)
	assert.Equal(t, "Test WebDAV", got.Name)
	assert.Equal(t, models.SyncTypeWebDAV, got.Type)
	assert.Equal(t, models.SyncDirectionUpload, got.SyncDirection)
	assert.Equal(t, "/data/local", got.LocalPath)
	assert.Equal(t, "/data/remote", got.RemotePath)
	assert.Equal(t, models.SyncStatusActive, got.Status)
}

func TestSyncRepository_GetEndpoint_NotFound(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)

	_, err := repo.GetEndpoint(9999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint not found")
}

func TestSyncRepository_UpdateEndpoint(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)

	endpoint := &models.SyncEndpoint{
		UserID:        1,
		Name:          "Original Name",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp/sync",
		SyncDirection: models.SyncDirectionBidirectional,
		LocalPath:     "/original/path",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	id, err := repo.CreateEndpoint(endpoint)
	require.NoError(t, err)

	// Update
	endpoint.ID = id
	endpoint.Name = "Updated Name"
	endpoint.URL = "file:///tmp/updated"
	endpoint.UpdatedAt = time.Now()
	now := time.Now()
	endpoint.LastSyncAt = &now

	err = repo.UpdateEndpoint(endpoint)
	require.NoError(t, err)

	// Verify
	got, err := repo.GetEndpoint(id)
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", got.Name)
	assert.Equal(t, "file:///tmp/updated", got.URL)
	assert.NotNil(t, got.LastSyncAt)
}

func TestSyncRepository_DeleteEndpoint(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)

	endpoint := &models.SyncEndpoint{
		UserID:        1,
		Name:          "To Delete",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp/del",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/del/path",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	id, err := repo.CreateEndpoint(endpoint)
	require.NoError(t, err)

	err = repo.DeleteEndpoint(id)
	require.NoError(t, err)

	// Verify deleted
	_, err = repo.GetEndpoint(id)
	assert.Error(t, err)
}

func TestSyncRepository_GetUserEndpoints(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)

	for i := 0; i < 3; i++ {
		ep := &models.SyncEndpoint{
			UserID:        42,
			Name:          "EP_" + string(rune('A'+i)),
			Type:          models.SyncTypeLocal,
			URL:           "file:///tmp",
			SyncDirection: models.SyncDirectionUpload,
			LocalPath:     "/data",
			Status:        models.SyncStatusActive,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		_, err := repo.CreateEndpoint(ep)
		require.NoError(t, err)
	}

	// Different user
	ep := &models.SyncEndpoint{
		UserID:        99,
		Name:          "Other User",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	_, err := repo.CreateEndpoint(ep)
	require.NoError(t, err)

	endpoints, err := repo.GetUserEndpoints(42)
	require.NoError(t, err)
	assert.Equal(t, 3, len(endpoints))

	endpoints99, err := repo.GetUserEndpoints(99)
	require.NoError(t, err)
	assert.Equal(t, 1, len(endpoints99))
}

// ---------------------------------------------------------------------------
// SyncRepository — CreateSession / GetSession / UpdateSession / GetUserSessions
// ---------------------------------------------------------------------------

func TestSyncRepository_CreateAndGetSession(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)

	session := &models.SyncSession{
		EndpointID:  1,
		UserID:      1,
		Status:      models.SyncSessionStatusRunning,
		SyncType:    models.SyncTypeManual,
		StartedAt:   time.Now(),
		TotalFiles:  100,
		SyncedFiles: 0,
	}

	id, err := repo.CreateSession(session)
	require.NoError(t, err)
	assert.Greater(t, id, 0)

	got, err := repo.GetSession(id)
	require.NoError(t, err)
	assert.Equal(t, id, got.ID)
	assert.Equal(t, models.SyncSessionStatusRunning, got.Status)
	assert.Equal(t, models.SyncTypeManual, got.SyncType)
	assert.Equal(t, 100, got.TotalFiles)
}

func TestSyncRepository_GetSession_NotFound(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	_, err := repo.GetSession(9999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "session not found")
}

func TestSyncRepository_UpdateSession(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)

	session := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusRunning,
		SyncType:   models.SyncTypeManual,
		StartedAt:  time.Now(),
	}

	id, err := repo.CreateSession(session)
	require.NoError(t, err)

	// Update to completed
	session.ID = id
	session.Status = models.SyncSessionStatusCompleted
	now := time.Now()
	session.CompletedAt = &now
	dur := 5 * time.Minute
	session.Duration = &dur
	session.TotalFiles = 50
	session.SyncedFiles = 45
	session.FailedFiles = 3
	session.SkippedFiles = 2

	err = repo.UpdateSession(session)
	require.NoError(t, err)

	got, err := repo.GetSession(id)
	require.NoError(t, err)
	assert.Equal(t, models.SyncSessionStatusCompleted, got.Status)
	assert.NotNil(t, got.CompletedAt)
	assert.NotNil(t, got.Duration)
	assert.Equal(t, 50, got.TotalFiles)
	assert.Equal(t, 45, got.SyncedFiles)
	assert.Equal(t, 3, got.FailedFiles)
	assert.Equal(t, 2, got.SkippedFiles)
}

func TestSyncRepository_UpdateSession_WithError(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)

	session := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusRunning,
		SyncType:   models.SyncTypeManual,
		StartedAt:  time.Now(),
	}

	id, err := repo.CreateSession(session)
	require.NoError(t, err)

	session.ID = id
	session.Status = models.SyncSessionStatusFailed
	now := time.Now()
	session.CompletedAt = &now
	errMsg := "connection timeout"
	session.ErrorMessage = &errMsg

	err = repo.UpdateSession(session)
	require.NoError(t, err)

	got, err := repo.GetSession(id)
	require.NoError(t, err)
	assert.Equal(t, models.SyncSessionStatusFailed, got.Status)
	require.NotNil(t, got.ErrorMessage)
	assert.Equal(t, "connection timeout", *got.ErrorMessage)
}

func TestSyncRepository_GetUserSessions(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)

	for i := 0; i < 5; i++ {
		session := &models.SyncSession{
			EndpointID: 1,
			UserID:     42,
			Status:     models.SyncSessionStatusCompleted,
			SyncType:   models.SyncTypeManual,
			StartedAt:  time.Now().Add(-time.Duration(i) * time.Hour),
		}
		_, err := repo.CreateSession(session)
		require.NoError(t, err)
	}

	// Get first page
	sessions, err := repo.GetUserSessions(42, 3, 0)
	require.NoError(t, err)
	assert.Equal(t, 3, len(sessions))

	// Get second page
	sessions, err = repo.GetUserSessions(42, 3, 3)
	require.NoError(t, err)
	assert.Equal(t, 2, len(sessions))
}

// ---------------------------------------------------------------------------
// SyncRepository — CreateSchedule / GetActiveSchedules
// ---------------------------------------------------------------------------

func TestSyncRepository_CreateSchedule(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)

	schedule := &models.SyncSchedule{
		EndpointID: 1,
		UserID:     1,
		Frequency:  models.SyncFrequencyHourly,
		IsActive:   true,
		CreatedAt:  time.Now(),
	}

	id, err := repo.CreateSchedule(schedule)
	require.NoError(t, err)
	assert.Greater(t, id, 0)
}

func TestSyncRepository_GetActiveSchedules(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)

	// Create active and inactive schedules
	active := &models.SyncSchedule{
		EndpointID: 1,
		UserID:     1,
		Frequency:  models.SyncFrequencyDaily,
		IsActive:   true,
		CreatedAt:  time.Now(),
	}
	_, err := repo.CreateSchedule(active)
	require.NoError(t, err)

	inactive := &models.SyncSchedule{
		EndpointID: 2,
		UserID:     1,
		Frequency:  models.SyncFrequencyWeekly,
		IsActive:   false,
		CreatedAt:  time.Now(),
	}
	_, err = repo.CreateSchedule(inactive)
	require.NoError(t, err)

	schedules, err := repo.GetActiveSchedules()
	require.NoError(t, err)
	assert.Equal(t, 1, len(schedules))
	assert.Equal(t, models.SyncFrequencyDaily, schedules[0].Frequency)
}

// ---------------------------------------------------------------------------
// SyncRepository — GetStatistics
// ---------------------------------------------------------------------------

func TestSyncRepository_GetStatistics(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)

	startDate := time.Now().Add(-24 * time.Hour)
	endDate := time.Now().Add(time.Hour)

	// Create sessions with different statuses
	for _, status := range []string{
		models.SyncSessionStatusCompleted,
		models.SyncSessionStatusCompleted,
		models.SyncSessionStatusFailed,
	} {
		session := &models.SyncSession{
			EndpointID:  1,
			UserID:      1,
			Status:      status,
			SyncType:    models.SyncTypeManual,
			StartedAt:   time.Now(),
			SyncedFiles: 10,
			FailedFiles: 2,
		}
		id, err := repo.CreateSession(session)
		require.NoError(t, err)

		// Mark completed with duration
		session.ID = id
		now := time.Now()
		session.CompletedAt = &now
		dur := 30 * time.Second
		session.Duration = &dur
		err = repo.UpdateSession(session)
		require.NoError(t, err)
	}

	stats, err := repo.GetStatistics(nil, startDate, endDate)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.TotalSessions)
	assert.Equal(t, 2, stats.ByStatus[models.SyncSessionStatusCompleted])
	assert.Equal(t, 1, stats.ByStatus[models.SyncSessionStatusFailed])

	// Filter by user
	userID := 1
	stats, err = repo.GetStatistics(&userID, startDate, endDate)
	require.NoError(t, err)
	assert.Equal(t, 3, stats.TotalSessions)
}

// ---------------------------------------------------------------------------
// SyncRepository — CleanupSessions
// ---------------------------------------------------------------------------

func TestSyncRepository_CleanupSessions(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)

	// Create old completed session
	session := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusCompleted,
		SyncType:   models.SyncTypeManual,
		StartedAt:  time.Now().Add(-48 * time.Hour),
	}
	id, err := repo.CreateSession(session)
	require.NoError(t, err)

	// Complete it with old date
	session.ID = id
	oldTime := time.Now().Add(-47 * time.Hour)
	session.CompletedAt = &oldTime
	err = repo.UpdateSession(session)
	require.NoError(t, err)

	// Create recent session
	recentSession := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusCompleted,
		SyncType:   models.SyncTypeManual,
		StartedAt:  time.Now(),
	}
	recentID, err := repo.CreateSession(recentSession)
	require.NoError(t, err)

	recentSession.ID = recentID
	now := time.Now()
	recentSession.CompletedAt = &now
	err = repo.UpdateSession(recentSession)
	require.NoError(t, err)

	// Cleanup old sessions (older than 24 hours)
	err = repo.CleanupSessions(time.Now().Add(-24 * time.Hour))
	require.NoError(t, err)

	// Old session should be deleted
	_, err = repo.GetSession(id)
	assert.Error(t, err)

	// Recent session should still exist
	got, err := repo.GetSession(recentID)
	require.NoError(t, err)
	assert.Equal(t, recentID, got.ID)
}

// ---------------------------------------------------------------------------
// SyncService — copyFile
// ---------------------------------------------------------------------------

func TestSyncService_CopyFile(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "source.txt")
	err := os.WriteFile(srcFile, []byte("hello sync world"), 0644)
	require.NoError(t, err)

	// Copy to a new subdirectory
	dstFile := filepath.Join(tmpDir, "sub", "dest.txt")
	err = service.copyFile(srcFile, dstFile, 0644)
	require.NoError(t, err)

	// Verify content
	content, err := os.ReadFile(dstFile)
	require.NoError(t, err)
	assert.Equal(t, "hello sync world", string(content))
}

func TestSyncService_CopyFile_NonexistentSource(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	tmpDir := t.TempDir()
	err := service.copyFile("/nonexistent/source.txt", filepath.Join(tmpDir, "dest.txt"), 0644)
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// SyncService — shouldSkipFile additional patterns
// ---------------------------------------------------------------------------

func TestSyncService_ShouldSkipFile_TempExtension(t *testing.T) {
	service := NewSyncService(nil, nil, nil)

	assert.True(t, service.shouldSkipFile("file.temp", &models.SyncEndpoint{}))
	assert.True(t, service.shouldSkipFile("file.tmp", &models.SyncEndpoint{}))
	assert.True(t, service.shouldSkipFile(".DS_Store", &models.SyncEndpoint{}))
	assert.False(t, service.shouldSkipFile("document.pdf", &models.SyncEndpoint{}))
	assert.False(t, service.shouldSkipFile("video.mp4", &models.SyncEndpoint{}))
}

// ---------------------------------------------------------------------------
// SyncService — handleSyncSuccess / handleSyncError (pure logic)
// ---------------------------------------------------------------------------

func TestSyncService_HandleSyncSuccess(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	session := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusRunning,
		SyncType:   models.SyncTypeManual,
		StartedAt:  time.Now().Add(-5 * time.Minute),
	}

	id, err := repo.CreateSession(session)
	require.NoError(t, err)
	session.ID = id

	service.handleSyncSuccess(session)

	assert.Equal(t, models.SyncSessionStatusCompleted, session.Status)
	assert.NotNil(t, session.CompletedAt)
	assert.NotNil(t, session.Duration)
}

func TestSyncService_HandleSyncError(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	session := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusRunning,
		SyncType:   models.SyncTypeManual,
		StartedAt:  time.Now().Add(-2 * time.Minute),
	}

	id, err := repo.CreateSession(session)
	require.NoError(t, err)
	session.ID = id

	service.handleSyncError(session, assert.AnError)

	assert.Equal(t, models.SyncSessionStatusFailed, session.Status)
	assert.NotNil(t, session.CompletedAt)
	assert.NotNil(t, session.ErrorMessage)
	assert.NotNil(t, session.Duration)
}

// ---------------------------------------------------------------------------
// SyncRepository — GetEndpointsByType
// ---------------------------------------------------------------------------

func TestSyncRepository_GetEndpointsByType(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)

	// Create endpoints of different types
	types := []string{models.SyncTypeWebDAV, models.SyncTypeWebDAV, models.SyncTypeLocal}
	for i, typ := range types {
		ep := &models.SyncEndpoint{
			UserID:        1,
			Name:          "EP_" + string(rune('A'+i)),
			Type:          typ,
			URL:           "test://url",
			SyncDirection: models.SyncDirectionUpload,
			LocalPath:     "/data",
			Status:        models.SyncStatusActive,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		_, err := repo.CreateEndpoint(ep)
		require.NoError(t, err)
	}

	webdavEPs, err := repo.GetEndpointsByType(models.SyncTypeWebDAV)
	require.NoError(t, err)
	assert.Equal(t, 2, len(webdavEPs))

	localEPs, err := repo.GetEndpointsByType(models.SyncTypeLocal)
	require.NoError(t, err)
	assert.Equal(t, 1, len(localEPs))

	cloudEPs, err := repo.GetEndpointsByType(models.SyncTypeCloudStorage)
	require.NoError(t, err)
	assert.Equal(t, 0, len(cloudEPs))
}

// ---------------------------------------------------------------------------
// SyncService — UpdateEndpoint (0% coverage)
// ---------------------------------------------------------------------------

func TestSyncService_UpdateEndpoint(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	// Create a local-type endpoint (avoids network in testConnection)
	endpoint := &models.SyncEndpoint{
		UserID:        1,
		Name:          "Original",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp/sync",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data/local",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	id, err := repo.CreateEndpoint(endpoint)
	require.NoError(t, err)

	t.Run("owner can update own endpoint", func(t *testing.T) {
		updated, err := service.UpdateEndpoint(id, 1, &models.UpdateSyncEndpointRequest{
			Name: "Updated Name",
			URL:  "file:///tmp/new-sync",
		})
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", updated.Name)
		assert.Equal(t, "file:///tmp/new-sync", updated.URL)
	})

	t.Run("update with multiple fields", func(t *testing.T) {
		isActive := true
		updated, err := service.UpdateEndpoint(id, 1, &models.UpdateSyncEndpointRequest{
			Username:      "newuser",
			Password:      "newpass",
			SyncDirection: models.SyncDirectionDownload,
			LocalPath:     "/new/local",
			IsActive:      &isActive,
		})
		require.NoError(t, err)
		assert.Equal(t, "newuser", updated.Username)
		assert.Equal(t, models.SyncDirectionDownload, updated.SyncDirection)
		assert.Equal(t, "/new/local", updated.LocalPath)
	})

	t.Run("nonexistent endpoint returns error", func(t *testing.T) {
		_, err := service.UpdateEndpoint(9999, 1, &models.UpdateSyncEndpointRequest{
			Name: "Ghost",
		})
		assert.Error(t, err)
	})

	t.Run("set inactive via IsActive pointer", func(t *testing.T) {
		isActive := false
		updated, err := service.UpdateEndpoint(id, 1, &models.UpdateSyncEndpointRequest{
			IsActive: &isActive,
		})
		require.NoError(t, err)
		assert.Equal(t, models.SyncStatusInactive, updated.Status)
	})

	t.Run("set sync settings", func(t *testing.T) {
		settings := `{"key":"value"}`
		updated, err := service.UpdateEndpoint(id, 1, &models.UpdateSyncEndpointRequest{
			SyncSettings: &settings,
		})
		require.NoError(t, err)
		require.NotNil(t, updated.SyncSettings)
		assert.Equal(t, settings, *updated.SyncSettings)
	})
}

// ---------------------------------------------------------------------------
// SyncService — GetSession (37.5% coverage)
// ---------------------------------------------------------------------------

func TestSyncService_GetSession(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	session := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusRunning,
		SyncType:   models.SyncTypeManual,
		StartedAt:  time.Now(),
	}

	id, err := repo.CreateSession(session)
	require.NoError(t, err)

	t.Run("owner can view own session", func(t *testing.T) {
		got, err := service.GetSession(id, 1)
		require.NoError(t, err)
		assert.Equal(t, id, got.ID)
	})

	t.Run("nonexistent session returns error", func(t *testing.T) {
		_, err := service.GetSession(9999, 1)
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// SyncService — ScheduleSync (0% coverage)
// ---------------------------------------------------------------------------

func TestSyncService_ScheduleSync(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	// Create an endpoint first
	endpoint := &models.SyncEndpoint{
		UserID:        1,
		Name:          "Schedule Test",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	epID, err := repo.CreateEndpoint(endpoint)
	require.NoError(t, err)

	t.Run("owner can schedule sync", func(t *testing.T) {
		schedule, err := service.ScheduleSync(epID, 1, &models.SyncSchedule{
			Frequency: models.SyncFrequencyDaily,
		})
		require.NoError(t, err)
		assert.Equal(t, epID, schedule.EndpointID)
		assert.Equal(t, 1, schedule.UserID)
		assert.True(t, schedule.IsActive)
		assert.Equal(t, models.SyncFrequencyDaily, schedule.Frequency)
		assert.Greater(t, schedule.ID, 0)
	})

	t.Run("nonexistent endpoint returns error", func(t *testing.T) {
		_, err := service.ScheduleSync(9999, 1, &models.SyncSchedule{
			Frequency: models.SyncFrequencyWeekly,
		})
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// SyncService — GetEndpoint (37.5% coverage)
// ---------------------------------------------------------------------------

func TestSyncService_GetEndpoint(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	endpoint := &models.SyncEndpoint{
		UserID:        1,
		Name:          "Get Test",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	epID, err := repo.CreateEndpoint(endpoint)
	require.NoError(t, err)

	t.Run("owner can get own endpoint", func(t *testing.T) {
		got, err := service.GetEndpoint(epID, 1)
		require.NoError(t, err)
		assert.Equal(t, "Get Test", got.Name)
	})

	t.Run("nonexistent endpoint returns error", func(t *testing.T) {
		_, err := service.GetEndpoint(9999, 1)
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// SyncService — logSyncError / updateSyncProgress / notifyUser (0% coverage)
// ---------------------------------------------------------------------------

func TestSyncService_LogSyncError(t *testing.T) {
	// bluff-scan: no-assert-ok (service-method smoke — public method must not panic on standard inputs)
	service := NewSyncService(nil, nil, nil)
	session := &models.SyncSession{ID: 42, UserID: 1}

	// These are simple fmt.Printf wrappers — just verify they don't panic
	service.logSyncError(session, "test error message")
	service.updateSyncProgress(session, "50% complete")
	service.notifyUser(session, "sync completed")
}

// ---------------------------------------------------------------------------
// SyncService — GetSyncStatistics
// ---------------------------------------------------------------------------

func TestSyncService_GetSyncStatistics(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	stats, err := service.GetSyncStatistics(nil, time.Now().Add(-24*time.Hour), time.Now())
	require.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, 0, stats.TotalSessions)
}

// ---------------------------------------------------------------------------
// SyncService — GetUserSessions
// ---------------------------------------------------------------------------

func TestSyncService_GetUserSessions(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	sessions, err := service.GetUserSessions(1, 10, 0)
	require.NoError(t, err)
	assert.Empty(t, sessions)
}

// ============================================================================
// EXPANDED DB-BACKED TESTS — StartSync, CreateSyncEndpoint, DeleteEndpoint,
// GetUserEndpoints, ProcessScheduledSyncs, CleanupOldSessions, performSync
// ============================================================================

func TestSyncService_CreateSyncEndpoint_WithDB(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	t.Run("create local endpoint successfully", func(t *testing.T) {
		endpoint := &models.SyncEndpoint{
			Name:          "Local Sync EP",
			Type:          models.SyncTypeLocal,
			URL:           "file:///tmp/sync",
			SyncDirection: models.SyncDirectionUpload,
			LocalPath:     "/data/local",
			RemotePath:    "/data/remote",
		}

		got, err := service.CreateSyncEndpoint(1, endpoint)
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Greater(t, got.ID, 0)
		assert.Equal(t, 1, got.UserID)
		assert.Equal(t, models.SyncStatusActive, got.Status)
		assert.NotZero(t, got.CreatedAt)
		assert.NotZero(t, got.UpdatedAt)
	})

	t.Run("create cloud storage endpoint successfully", func(t *testing.T) {
		endpoint := &models.SyncEndpoint{
			Name:          "Cloud EP",
			Type:          models.SyncTypeCloudStorage,
			URL:           "s3://my-bucket",
			SyncDirection: models.SyncDirectionBidirectional,
			LocalPath:     "/data/cloud",
			RemotePath:    "/",
		}

		got, err := service.CreateSyncEndpoint(2, endpoint)
		require.NoError(t, err)
		assert.NotNil(t, got)
		assert.Equal(t, 2, got.UserID)
		assert.Equal(t, models.SyncTypeCloudStorage, got.Type)
	})

	t.Run("validation failure missing name", func(t *testing.T) {
		endpoint := &models.SyncEndpoint{
			Type:          models.SyncTypeLocal,
			URL:           "file:///tmp",
			SyncDirection: models.SyncDirectionUpload,
			LocalPath:     "/data",
		}

		_, err := service.CreateSyncEndpoint(1, endpoint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sync endpoint")
	})

	t.Run("validation failure invalid type", func(t *testing.T) {
		endpoint := &models.SyncEndpoint{
			Name:          "Bad Type",
			Type:          "ftp",
			URL:           "ftp://example.com",
			SyncDirection: models.SyncDirectionUpload,
			LocalPath:     "/data",
		}

		_, err := service.CreateSyncEndpoint(1, endpoint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sync endpoint")
	})

	t.Run("validation failure invalid direction", func(t *testing.T) {
		endpoint := &models.SyncEndpoint{
			Name:          "Bad Dir",
			Type:          models.SyncTypeLocal,
			URL:           "file:///tmp",
			SyncDirection: "push",
			LocalPath:     "/data",
		}

		_, err := service.CreateSyncEndpoint(1, endpoint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sync endpoint")
	})

	t.Run("validation failure missing URL", func(t *testing.T) {
		endpoint := &models.SyncEndpoint{
			Name:          "No URL",
			Type:          models.SyncTypeLocal,
			SyncDirection: models.SyncDirectionUpload,
			LocalPath:     "/data",
		}

		_, err := service.CreateSyncEndpoint(1, endpoint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sync endpoint")
	})

	t.Run("validation failure missing local path", func(t *testing.T) {
		endpoint := &models.SyncEndpoint{
			Name:          "No Local Path",
			Type:          models.SyncTypeLocal,
			URL:           "file:///tmp",
			SyncDirection: models.SyncDirectionUpload,
		}

		_, err := service.CreateSyncEndpoint(1, endpoint)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid sync endpoint")
	})
}

func TestSyncService_CreateSyncEndpoint_SetsTimestamps(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	before := time.Now().Add(-time.Second)

	endpoint := &models.SyncEndpoint{
		Name:          "Timestamp Test",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data",
		RemotePath:    "/remote",
	}

	got, err := service.CreateSyncEndpoint(1, endpoint)
	require.NoError(t, err)

	after := time.Now().Add(time.Second)

	assert.True(t, got.CreatedAt.After(before), "CreatedAt should be after test start")
	assert.True(t, got.CreatedAt.Before(after), "CreatedAt should be before test end")
	assert.True(t, got.UpdatedAt.After(before), "UpdatedAt should be after test start")
	assert.True(t, got.UpdatedAt.Before(after), "UpdatedAt should be before test end")
	assert.Equal(t, models.SyncStatusActive, got.Status)
}

func TestSyncService_StartSync_WithDB(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	// Create an active local-type endpoint
	endpoint := &models.SyncEndpoint{
		UserID:        1,
		Name:          "Active EP",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	epID, err := repo.CreateEndpoint(endpoint)
	require.NoError(t, err)

	t.Run("start sync on active endpoint returns session", func(t *testing.T) {
		session, err := service.StartSync(epID, 1)
		require.NoError(t, err)
		assert.NotNil(t, session)
		assert.Greater(t, session.ID, 0)
		assert.Equal(t, epID, session.EndpointID)
		assert.Equal(t, 1, session.UserID)
		assert.Equal(t, models.SyncSessionStatusRunning, session.Status)
		assert.Equal(t, models.SyncTypeManual, session.SyncType)
		assert.NotZero(t, session.StartedAt)

		// Wait briefly for goroutine to start
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("start sync on inactive endpoint returns error", func(t *testing.T) {
		inactiveEP := &models.SyncEndpoint{
			UserID:        1,
			Name:          "Inactive EP",
			Type:          models.SyncTypeLocal,
			URL:           "file:///tmp",
			SyncDirection: models.SyncDirectionUpload,
			LocalPath:     "/data",
			Status:        models.SyncStatusInactive,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		inactiveID, err := repo.CreateEndpoint(inactiveEP)
		require.NoError(t, err)

		_, err = service.StartSync(inactiveID, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "endpoint is not active")
	})

	t.Run("start sync on nonexistent endpoint returns error", func(t *testing.T) {
		_, err := service.StartSync(9999, 1)
		assert.Error(t, err)
	})

	t.Run("start sync on error-status endpoint returns error", func(t *testing.T) {
		errorEP := &models.SyncEndpoint{
			UserID:        1,
			Name:          "Error EP",
			Type:          models.SyncTypeLocal,
			URL:           "file:///tmp",
			SyncDirection: models.SyncDirectionUpload,
			LocalPath:     "/data",
			Status:        models.SyncStatusError,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		errorID, err := repo.CreateEndpoint(errorEP)
		require.NoError(t, err)

		_, err = service.StartSync(errorID, 1)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "endpoint is not active")
	})
}

func TestSyncService_StartSync_ReturnsSnapshotCopy(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	endpoint := &models.SyncEndpoint{
		UserID:        1,
		Name:          "Snapshot Test EP",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	epID, err := repo.CreateEndpoint(endpoint)
	require.NoError(t, err)

	session, err := service.StartSync(epID, 1)
	require.NoError(t, err)

	// The returned session is a snapshot; verify its fields are populated
	assert.Equal(t, epID, session.EndpointID)
	assert.Equal(t, 1, session.UserID)
	assert.Equal(t, models.SyncSessionStatusRunning, session.Status)

	// Wait for background goroutine to complete
	time.Sleep(100 * time.Millisecond)
}

func TestSyncService_MultipleStartSync_CreatesSeparateSessions(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	endpoint := &models.SyncEndpoint{
		UserID:        1,
		Name:          "Multi Sync",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	epID, err := repo.CreateEndpoint(endpoint)
	require.NoError(t, err)

	session1, err := service.StartSync(epID, 1)
	require.NoError(t, err)

	session2, err := service.StartSync(epID, 1)
	require.NoError(t, err)

	// Each call should create a unique session
	assert.NotEqual(t, session1.ID, session2.ID)

	// Wait for background goroutines
	time.Sleep(100 * time.Millisecond)
}

func TestSyncService_DeleteEndpoint_WithDB(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	endpoint := &models.SyncEndpoint{
		UserID:        1,
		Name:          "To Delete",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	epID, err := repo.CreateEndpoint(endpoint)
	require.NoError(t, err)

	t.Run("owner can delete own endpoint", func(t *testing.T) {
		err := service.DeleteEndpoint(epID, 1)
		require.NoError(t, err)

		// Verify deleted
		_, err = repo.GetEndpoint(epID)
		assert.Error(t, err)
	})

	t.Run("delete nonexistent endpoint returns error", func(t *testing.T) {
		err := service.DeleteEndpoint(9999, 1)
		assert.Error(t, err)
	})
}

// setupSyncAuthDB creates users and roles tables on top of newSyncTestDB
// so that AuthService.CheckPermission can resolve user roles.
func setupSyncAuthDB(t *testing.T, db *database.DB) (*SyncService, *repository.SyncRepository) {
	t.Helper()

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL, email TEXT UNIQUE NOT NULL,
		password_hash TEXT, salt TEXT, role_id INTEGER NOT NULL DEFAULT 1,
		first_name TEXT, last_name TEXT, display_name TEXT, avatar_url TEXT,
		time_zone TEXT, language TEXT, settings TEXT DEFAULT '{}',
		is_active BOOLEAN DEFAULT 1, is_locked BOOLEAN DEFAULT 0,
		locked_until DATETIME, failed_login_attempts INTEGER DEFAULT 0,
		last_login_at DATETIME, last_login_ip TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS roles (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE, description TEXT,
		permissions TEXT DEFAULT '{}', is_system BOOLEAN DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)

	// Regular user role without share permissions
	_, err = db.Exec(`INSERT OR IGNORE INTO roles (id, name, description, permissions, is_system) VALUES (1, 'user', 'Regular user', '["read","write"]', 1)`)
	require.NoError(t, err)

	// User 999 is a regular user (no share permissions)
	_, err = db.Exec(`INSERT OR IGNORE INTO users (id, username, email, password_hash, salt, role_id, is_active) VALUES (999, 'otheruser', 'other@example.com', '', '', 1, 1)`)
	require.NoError(t, err)

	repo := repository.NewSyncRepository(db)
	userRepo := repository.NewUserRepository(db)
	authService := NewAuthService(userRepo, "test-secret")
	service := NewSyncService(repo, nil, authService)

	return service, repo
}

func TestSyncService_GetUserEndpoints_WithDB(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	// Create endpoints for user 1
	for i := 0; i < 3; i++ {
		ep := &models.SyncEndpoint{
			UserID:        1,
			Name:          "EP_" + string(rune('A'+i)),
			Type:          models.SyncTypeLocal,
			URL:           "file:///tmp",
			SyncDirection: models.SyncDirectionUpload,
			LocalPath:     "/data",
			Status:        models.SyncStatusActive,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		_, err := repo.CreateEndpoint(ep)
		require.NoError(t, err)
	}

	// Create endpoint for user 2
	ep := &models.SyncEndpoint{
		UserID:        2,
		Name:          "Other User EP",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	_, err := repo.CreateEndpoint(ep)
	require.NoError(t, err)

	t.Run("get endpoints for user with 3 endpoints", func(t *testing.T) {
		endpoints, err := service.GetUserEndpoints(1)
		require.NoError(t, err)
		assert.Len(t, endpoints, 3)
	})

	t.Run("get endpoints for user with 1 endpoint", func(t *testing.T) {
		endpoints, err := service.GetUserEndpoints(2)
		require.NoError(t, err)
		assert.Len(t, endpoints, 1)
	})

	t.Run("get endpoints for user with no endpoints", func(t *testing.T) {
		endpoints, err := service.GetUserEndpoints(999)
		require.NoError(t, err)
		assert.Empty(t, endpoints)
	})
}

func TestSyncService_GetEndpoint_NonOwnerNoPermission(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	service, repo := setupSyncAuthDB(t, db)

	endpoint := &models.SyncEndpoint{
		UserID:        1,
		Name:          "Private EP",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	id, err := repo.CreateEndpoint(endpoint)
	require.NoError(t, err)

	// User 999 (regular user without view_shares) tries to view user 1's endpoint
	_, err = service.GetEndpoint(id, 999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestSyncService_GetSession_NonOwnerNoPermission(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	service, repo := setupSyncAuthDB(t, db)

	session := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusRunning,
		SyncType:   models.SyncTypeManual,
		StartedAt:  time.Now(),
	}

	id, err := repo.CreateSession(session)
	require.NoError(t, err)

	// User 999 (regular user without view_shares) tries to view user 1's session
	_, err = service.GetSession(id, 999)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestSyncService_UpdateEndpoint_NonOwnerNoPermission(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	service, repo := setupSyncAuthDB(t, db)

	endpoint := &models.SyncEndpoint{
		UserID:        1,
		Name:          "Owner's EP",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	id, err := repo.CreateEndpoint(endpoint)
	require.NoError(t, err)

	// User 999 (regular user without edit_shares) tries to update user 1's endpoint
	_, err = service.UpdateEndpoint(id, 999, &models.UpdateSyncEndpointRequest{
		Name: "Hijacked",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestSyncService_UpdateEndpoint_WithRemotePath(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	endpoint := &models.SyncEndpoint{
		UserID:        1,
		Name:          "Remote Path Test",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data",
		RemotePath:    "/old/remote",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	id, err := repo.CreateEndpoint(endpoint)
	require.NoError(t, err)

	updated, err := service.UpdateEndpoint(id, 1, &models.UpdateSyncEndpointRequest{
		RemotePath: "/new/remote",
	})
	require.NoError(t, err)
	assert.Equal(t, "/new/remote", updated.RemotePath)
}

func TestSyncService_ScheduleSync_NonOwnerNoPermission(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	service, repo := setupSyncAuthDB(t, db)

	endpoint := &models.SyncEndpoint{
		UserID:        1,
		Name:          "Protected Schedule",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	epID, err := repo.CreateEndpoint(endpoint)
	require.NoError(t, err)

	// User 999 (regular user without edit_shares) tries to schedule sync on user 1's endpoint
	_, err = service.ScheduleSync(epID, 999, &models.SyncSchedule{
		Frequency: models.SyncFrequencyDaily,
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unauthorized")
}

func TestSyncService_ProcessScheduledSyncs_WithDB(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	// Create an active endpoint
	endpoint := &models.SyncEndpoint{
		UserID:        1,
		Name:          "Scheduled EP",
		Type:          models.SyncTypeLocal,
		URL:           "file:///tmp",
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/data",
		Status:        models.SyncStatusActive,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	epID, err := repo.CreateEndpoint(endpoint)
	require.NoError(t, err)

	t.Run("processes due schedules", func(t *testing.T) {
		// Create an hourly schedule that has never run
		schedule := &models.SyncSchedule{
			EndpointID: epID,
			UserID:     1,
			Frequency:  models.SyncFrequencyHourly,
			IsActive:   true,
			CreatedAt:  time.Now(),
		}

		_, err := repo.CreateSchedule(schedule)
		require.NoError(t, err)

		err = service.ProcessScheduledSyncs()
		assert.NoError(t, err)

		// Wait for background sync goroutine
		time.Sleep(100 * time.Millisecond)

		// Verify a session was created
		sessions, err := repo.GetUserSessions(1, 10, 0)
		require.NoError(t, err)
		assert.Greater(t, len(sessions), 0)
	})

	t.Run("no error when no active schedules", func(t *testing.T) {
		db2, cleanup2 := newSyncTestDB(t)
		defer cleanup2()

		repo2 := repository.NewSyncRepository(db2)
		service2 := NewSyncService(repo2, nil, nil)

		err := service2.ProcessScheduledSyncs()
		assert.NoError(t, err)
	})
}

func TestSyncService_CleanupOldSessions_WithDB(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	// Create an old completed session
	oldSession := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusCompleted,
		SyncType:   models.SyncTypeManual,
		StartedAt:  time.Now().Add(-72 * time.Hour),
	}

	oldID, err := repo.CreateSession(oldSession)
	require.NoError(t, err)

	oldSession.ID = oldID
	oldTime := time.Now().Add(-71 * time.Hour)
	oldSession.CompletedAt = &oldTime
	err = repo.UpdateSession(oldSession)
	require.NoError(t, err)

	// Create a recent completed session
	recentSession := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusCompleted,
		SyncType:   models.SyncTypeManual,
		StartedAt:  time.Now(),
	}

	recentID, err := repo.CreateSession(recentSession)
	require.NoError(t, err)

	recentSession.ID = recentID
	now := time.Now()
	recentSession.CompletedAt = &now
	err = repo.UpdateSession(recentSession)
	require.NoError(t, err)

	// Create an old running session (should NOT be cleaned up)
	runningSession := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusRunning,
		SyncType:   models.SyncTypeManual,
		StartedAt:  time.Now().Add(-72 * time.Hour),
	}

	runningID, err := repo.CreateSession(runningSession)
	require.NoError(t, err)

	// Cleanup sessions older than 24 hours
	err = service.CleanupOldSessions(time.Now().Add(-24 * time.Hour))
	require.NoError(t, err)

	// Old completed session should be deleted
	_, err = repo.GetSession(oldID)
	assert.Error(t, err)

	// Recent session should still exist
	got, err := repo.GetSession(recentID)
	require.NoError(t, err)
	assert.Equal(t, recentID, got.ID)

	// Running session should still exist (not in cleanup status list)
	got, err = repo.GetSession(runningID)
	require.NoError(t, err)
	assert.Equal(t, runningID, got.ID)
}

func TestSyncService_GetSyncStatistics_WithSessions(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	startDate := time.Now().Add(-24 * time.Hour)
	endDate := time.Now().Add(time.Hour)

	// Create completed sessions
	for i := 0; i < 3; i++ {
		session := &models.SyncSession{
			EndpointID:  1,
			UserID:      1,
			Status:      models.SyncSessionStatusCompleted,
			SyncType:    models.SyncTypeManual,
			StartedAt:   time.Now(),
			SyncedFiles: 10,
			FailedFiles: 1,
		}
		id, err := repo.CreateSession(session)
		require.NoError(t, err)

		session.ID = id
		now := time.Now()
		session.CompletedAt = &now
		dur := 30 * time.Second
		session.Duration = &dur
		err = repo.UpdateSession(session)
		require.NoError(t, err)
	}

	// Create a failed session
	failedSession := &models.SyncSession{
		EndpointID:  1,
		UserID:      1,
		Status:      models.SyncSessionStatusFailed,
		SyncType:    models.SyncTypeScheduled,
		StartedAt:   time.Now(),
		FailedFiles: 5,
	}
	fid, err := repo.CreateSession(failedSession)
	require.NoError(t, err)

	failedSession.ID = fid
	now := time.Now()
	failedSession.CompletedAt = &now
	errMsg := "connection refused"
	failedSession.ErrorMessage = &errMsg
	err = repo.UpdateSession(failedSession)
	require.NoError(t, err)

	t.Run("statistics with all users", func(t *testing.T) {
		stats, err := service.GetSyncStatistics(nil, startDate, endDate)
		require.NoError(t, err)
		assert.Equal(t, 4, stats.TotalSessions)
		assert.Equal(t, 3, stats.ByStatus[models.SyncSessionStatusCompleted])
		assert.Equal(t, 1, stats.ByStatus[models.SyncSessionStatusFailed])
		assert.Equal(t, 30, stats.TotalFilesSynced)
		assert.Greater(t, stats.SuccessRate, float64(0))
	})

	t.Run("statistics filtered by user", func(t *testing.T) {
		userID := 1
		stats, err := service.GetSyncStatistics(&userID, startDate, endDate)
		require.NoError(t, err)
		assert.Equal(t, 4, stats.TotalSessions)
	})

	t.Run("statistics for nonexistent user returns empty", func(t *testing.T) {
		userID := 999
		stats, err := service.GetSyncStatistics(&userID, startDate, endDate)
		require.NoError(t, err)
		assert.Equal(t, 0, stats.TotalSessions)
	})

	t.Run("statistics with narrow date range returns empty", func(t *testing.T) {
		farPast := time.Now().Add(-30 * 24 * time.Hour)
		farPastEnd := time.Now().Add(-29 * 24 * time.Hour)
		stats, err := service.GetSyncStatistics(nil, farPast, farPastEnd)
		require.NoError(t, err)
		assert.Equal(t, 0, stats.TotalSessions)
	})
}

func TestSyncService_GetUserSessions_WithPagination(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	// Create 7 sessions for user 1
	for i := 0; i < 7; i++ {
		session := &models.SyncSession{
			EndpointID: 1,
			UserID:     1,
			Status:     models.SyncSessionStatusCompleted,
			SyncType:   models.SyncTypeManual,
			StartedAt:  time.Now().Add(-time.Duration(i) * time.Hour),
		}
		_, err := repo.CreateSession(session)
		require.NoError(t, err)
	}

	t.Run("first page", func(t *testing.T) {
		sessions, err := service.GetUserSessions(1, 3, 0)
		require.NoError(t, err)
		assert.Len(t, sessions, 3)
	})

	t.Run("second page", func(t *testing.T) {
		sessions, err := service.GetUserSessions(1, 3, 3)
		require.NoError(t, err)
		assert.Len(t, sessions, 3)
	})

	t.Run("third page partial", func(t *testing.T) {
		sessions, err := service.GetUserSessions(1, 3, 6)
		require.NoError(t, err)
		assert.Len(t, sessions, 1)
	})

	t.Run("beyond last page", func(t *testing.T) {
		sessions, err := service.GetUserSessions(1, 3, 10)
		require.NoError(t, err)
		assert.Empty(t, sessions)
	})
}

// ---------------------------------------------------------------------------
// performSync dispatch — various endpoint types
// ---------------------------------------------------------------------------

func TestSyncService_PerformSync_UnsupportedEndpointType(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	session := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusRunning,
		SyncType:   models.SyncTypeManual,
		StartedAt:  time.Now(),
	}

	id, err := repo.CreateSession(session)
	require.NoError(t, err)
	session.ID = id

	endpoint := &models.SyncEndpoint{
		ID:            1,
		Type:          "unsupported_type",
		SyncDirection: models.SyncDirectionUpload,
	}

	service.performSync(session, endpoint)

	// The session should be marked as failed
	got, err := repo.GetSession(id)
	require.NoError(t, err)
	assert.Equal(t, models.SyncSessionStatusFailed, got.Status)
	assert.NotNil(t, got.ErrorMessage)
	assert.Contains(t, *got.ErrorMessage, "unsupported sync type")
}

func TestSyncService_PerformSync_LocalType_Mirror(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	allowSyncDirs(t, srcDir, dstDir)

	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "test.txt"), []byte("sync test"), 0644))

	settings := `{"source_directory":"` + srcDir + `","destination_directory":"` + dstDir + `","sync_mode":"mirror"}`

	session := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusRunning,
		SyncType:   models.SyncTypeManual,
		StartedAt:  time.Now(),
	}

	id, err := repo.CreateSession(session)
	require.NoError(t, err)
	session.ID = id

	endpoint := &models.SyncEndpoint{
		ID:            1,
		Type:          models.SyncTypeLocal,
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     srcDir,
		SyncSettings:  &settings,
	}

	service.performSync(session, endpoint)

	// Verify file was synced
	content, err := os.ReadFile(filepath.Join(dstDir, "test.txt"))
	require.NoError(t, err)
	assert.Equal(t, "sync test", string(content))

	// Session should be completed
	got, err := repo.GetSession(id)
	require.NoError(t, err)
	assert.Equal(t, models.SyncSessionStatusCompleted, got.Status)
}

func TestSyncService_PerformSync_LocalType_Incremental(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	allowSyncDirs(t, srcDir, dstDir)

	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "new.txt"), []byte("new file"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dstDir, "existing.txt"), []byte("existing"), 0644))

	settings := `{"source_directory":"` + srcDir + `","destination_directory":"` + dstDir + `","sync_mode":"incremental"}`

	session := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusRunning,
		SyncType:   models.SyncTypeManual,
		StartedAt:  time.Now(),
	}

	id, err := repo.CreateSession(session)
	require.NoError(t, err)
	session.ID = id

	endpoint := &models.SyncEndpoint{
		ID:            1,
		Type:          models.SyncTypeLocal,
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     srcDir,
		SyncSettings:  &settings,
	}

	service.performSync(session, endpoint)

	// New file should be copied
	content, err := os.ReadFile(filepath.Join(dstDir, "new.txt"))
	require.NoError(t, err)
	assert.Equal(t, "new file", string(content))

	// Existing file should NOT be removed (incremental mode)
	_, err = os.Stat(filepath.Join(dstDir, "existing.txt"))
	assert.NoError(t, err)
}

func TestSyncService_PerformSync_LocalType_Bidirectional(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	srcDir := t.TempDir()
	dstDir := t.TempDir()
	allowSyncDirs(t, srcDir, dstDir)

	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "from_src.txt"), []byte("from source"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dstDir, "from_dst.txt"), []byte("from dest"), 0644))

	settings := `{"source_directory":"` + srcDir + `","destination_directory":"` + dstDir + `","sync_mode":"bidirectional"}`

	session := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusRunning,
		SyncType:   models.SyncTypeManual,
		StartedAt:  time.Now(),
	}

	id, err := repo.CreateSession(session)
	require.NoError(t, err)
	session.ID = id

	endpoint := &models.SyncEndpoint{
		ID:            1,
		Type:          models.SyncTypeLocal,
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     srcDir,
		SyncSettings:  &settings,
	}

	service.performSync(session, endpoint)

	// Both files should exist in both directories
	_, err = os.Stat(filepath.Join(dstDir, "from_src.txt"))
	assert.NoError(t, err)
	_, err = os.Stat(filepath.Join(srcDir, "from_dst.txt"))
	assert.NoError(t, err)

	got, err := repo.GetSession(id)
	require.NoError(t, err)
	assert.Equal(t, models.SyncSessionStatusCompleted, got.Status)
}

func TestSyncService_PerformSync_LocalType_BadConfig(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	settings := `invalid json`

	session := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusRunning,
		SyncType:   models.SyncTypeManual,
		StartedAt:  time.Now(),
	}

	id, err := repo.CreateSession(session)
	require.NoError(t, err)
	session.ID = id

	endpoint := &models.SyncEndpoint{
		ID:            1,
		Type:          models.SyncTypeLocal,
		SyncDirection: models.SyncDirectionUpload,
		LocalPath:     "/tmp",
		SyncSettings:  &settings,
	}

	service.performSync(session, endpoint)

	got, err := repo.GetSession(id)
	require.NoError(t, err)
	assert.Equal(t, models.SyncSessionStatusFailed, got.Status)
	assert.NotNil(t, got.ErrorMessage)
}

// ---------------------------------------------------------------------------
// HandleSyncSuccess / HandleSyncError — zero-value StartedAt
// ---------------------------------------------------------------------------

func TestSyncService_HandleSyncSuccess_ZeroStartTime(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	session := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusRunning,
		SyncType:   models.SyncTypeManual,
		// StartedAt is zero value
	}

	id, err := repo.CreateSession(session)
	require.NoError(t, err)
	session.ID = id

	service.handleSyncSuccess(session)

	assert.Equal(t, models.SyncSessionStatusCompleted, session.Status)
	assert.NotNil(t, session.CompletedAt)
}

func TestSyncService_HandleSyncError_ZeroStartTime(t *testing.T) {
	db, cleanup := newSyncTestDB(t)
	defer cleanup()

	repo := repository.NewSyncRepository(db)
	service := NewSyncService(repo, nil, nil)

	session := &models.SyncSession{
		EndpointID: 1,
		UserID:     1,
		Status:     models.SyncSessionStatusRunning,
		SyncType:   models.SyncTypeManual,
		// StartedAt is zero value
	}

	id, err := repo.CreateSession(session)
	require.NoError(t, err)
	session.ID = id

	testErr := assert.AnError
	service.handleSyncError(session, testErr)

	assert.Equal(t, models.SyncSessionStatusFailed, session.Status)
	assert.NotNil(t, session.CompletedAt)
	assert.NotNil(t, session.ErrorMessage)
}
