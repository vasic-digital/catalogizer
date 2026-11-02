package repository

import (
	"database/sql"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newMockConfigRepo(t *testing.T) (*ConfigurationRepository, sqlmock.Sqlmock) {
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	return NewConfigurationRepository(db), mock
}

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

func TestConfigurationRepository_Constructor(t *testing.T) {
	sqlDB, _, err := sqlmock.New()
	require.NoError(t, err)
	db := database.WrapDB(sqlDB, database.DialectSQLite)
	repo := NewConfigurationRepository(db)
	assert.NotNil(t, repo)
}

// ---------------------------------------------------------------------------
// SaveConfiguration
// ---------------------------------------------------------------------------

func TestConfigurationRepository_SaveConfiguration(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		config  *models.SystemConfiguration
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name: "success",
			config: &models.SystemConfiguration{
				Version:   "1.0.0",
				CreatedAt: now,
				UpdatedAt: now,
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT OR REPLACE INTO system_configuration").
					WithArgs(1, "1.0.0", sqlmock.AnyArg(), now, now).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
		},
		{
			name: "database error",
			config: &models.SystemConfiguration{
				Version:   "1.0.0",
				CreatedAt: now,
				UpdatedAt: now,
			},
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT OR REPLACE INTO system_configuration").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockConfigRepo(t)
			tt.setup(mock)

			err := repo.SaveConfiguration(tt.config)
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
// GetConfiguration
// ---------------------------------------------------------------------------

func TestConfigurationRepository_GetConfiguration(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
		check   func(t *testing.T, config *models.SystemConfiguration)
	}{
		{
			name: "success",
			setup: func(mock sqlmock.Sqlmock) {
				configJSON := `{"version":"1.0.0"}`
				mock.ExpectQuery("SELECT version, configuration, created_at, updated_at").
					WillReturnRows(sqlmock.NewRows([]string{"version", "configuration", "created_at", "updated_at"}).
						AddRow("1.0.0", configJSON, now, now))
			},
			check: func(t *testing.T, config *models.SystemConfiguration) {
				assert.Equal(t, "1.0.0", config.Version)
			},
		},
		{
			name: "not found",
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT version, configuration, created_at, updated_at").
					WillReturnError(sql.ErrNoRows)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockConfigRepo(t)
			tt.setup(mock)

			config, err := repo.GetConfiguration()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, config)
			tt.check(t, config)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// SaveWizardProgress / GetWizardProgress
// ---------------------------------------------------------------------------

func TestConfigurationRepository_WizardProgress(t *testing.T) {
	now := time.Now()

	t.Run("save wizard progress", func(t *testing.T) {
		repo, mock := newMockConfigRepo(t)
		progress := &models.WizardProgress{
			UserID:      1,
			CurrentStep: "database",
			StepData:    map[string]interface{}{"host": "localhost"},
			AllData:     map[string]interface{}{"step1": "done"},
			UpdatedAt:   now,
		}

		mock.ExpectExec("INSERT OR REPLACE INTO wizard_progress").
			WithArgs(1, "database", sqlmock.AnyArg(), sqlmock.AnyArg(), now).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.SaveWizardProgress(progress)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("get wizard progress", func(t *testing.T) {
		repo, mock := newMockConfigRepo(t)

		mock.ExpectQuery("SELECT user_id, current_step, step_data, all_data, updated_at").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"user_id", "current_step", "step_data", "all_data", "updated_at"}).
				AddRow(1, "database", `{"host":"localhost"}`, `{"step1":"done"}`, now))

		progress, err := repo.GetWizardProgress(1)
		require.NoError(t, err)
		assert.Equal(t, 1, progress.UserID)
		assert.Equal(t, "database", progress.CurrentStep)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("get wizard progress not found", func(t *testing.T) {
		repo, mock := newMockConfigRepo(t)

		mock.ExpectQuery("SELECT user_id, current_step, step_data, all_data, updated_at").
			WithArgs(999).
			WillReturnError(sql.ErrNoRows)

		progress, err := repo.GetWizardProgress(999)
		assert.NoError(t, err)
		assert.NotNil(t, progress)
		assert.Equal(t, 999, progress.UserID)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// DeleteWizardProgress
// ---------------------------------------------------------------------------

func TestConfigurationRepository_DeleteWizardProgress(t *testing.T) {
	tests := []struct {
		name    string
		userID  int
		setup   func(mock sqlmock.Sqlmock)
		wantErr bool
	}{
		{
			name:   "success",
			userID: 1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM wizard_progress WHERE user_id").
					WithArgs(1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name:   "database error",
			userID: 1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM wizard_progress WHERE user_id").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockConfigRepo(t)
			tt.setup(mock)

			err := repo.DeleteWizardProgress(tt.userID)
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
// MarkWizardCompleted / IsWizardCompleted
// ---------------------------------------------------------------------------

func TestConfigurationRepository_WizardCompletion(t *testing.T) {
	t.Run("mark wizard completed", func(t *testing.T) {
		repo, mock := newMockConfigRepo(t)
		mock.ExpectExec("INSERT OR REPLACE INTO wizard_completion").
			WithArgs(1, sqlmock.AnyArg()).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.MarkWizardCompleted(1)
		assert.NoError(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("is wizard completed - true", func(t *testing.T) {
		repo, mock := newMockConfigRepo(t)
		mock.ExpectQuery("SELECT COUNT").
			WithArgs(1).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

		completed, err := repo.IsWizardCompleted(1)
		require.NoError(t, err)
		assert.True(t, completed)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("is wizard completed - false", func(t *testing.T) {
		repo, mock := newMockConfigRepo(t)
		mock.ExpectQuery("SELECT COUNT").
			WithArgs(2).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		completed, err := repo.IsWizardCompleted(2)
		require.NoError(t, err)
		assert.False(t, completed)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// GetConfigurationHistory
// ---------------------------------------------------------------------------

func TestConfigurationRepository_GetConfigurationHistory(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name    string
		limit   int
		setup   func(mock sqlmock.Sqlmock)
		want    int
		wantErr bool
	}{
		{
			name:  "returns history",
			limit: 10,
			setup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "version", "created_at", "updated_at"}).
					AddRow(1, "1.0.0", now, now).
					AddRow(2, "1.1.0", now, now)
				mock.ExpectQuery("SELECT id, version, created_at, updated_at").
					WithArgs(10).
					WillReturnRows(rows)
			},
			want: 2,
		},
		{
			name:  "empty history",
			limit: 10,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT id, version, created_at, updated_at").
					WithArgs(10).
					WillReturnRows(sqlmock.NewRows([]string{"id", "version", "created_at", "updated_at"}))
			},
			want: 0,
		},
		{
			name:  "database error",
			limit: 10,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT id, version, created_at, updated_at").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockConfigRepo(t)
			tt.setup(mock)

			history, err := repo.GetConfigurationHistory(tt.limit)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, history, tt.want)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

// ---------------------------------------------------------------------------
// GetConfigurationBackups
// ---------------------------------------------------------------------------

func TestConfigurationRepository_GetConfigurationBackups(t *testing.T) {
	now := time.Now()

	repo, mock := newMockConfigRepo(t)
	rows := sqlmock.NewRows([]string{"id", "name", "version", "created_at"}).
		AddRow(1, "backup-1", "1.0.0", now).
		AddRow(2, "backup-2", "1.1.0", now)
	mock.ExpectQuery("SELECT id, name, version, created_at").
		WillReturnRows(rows)

	backups, err := repo.GetConfigurationBackups()
	require.NoError(t, err)
	assert.Len(t, backups, 2)
	assert.Equal(t, "backup-1", backups[0].Name)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// DeleteConfigurationBackup
// ---------------------------------------------------------------------------

func TestConfigurationRepository_DeleteConfigurationBackup(t *testing.T) {
	tests := []struct {
		name     string
		backupID int
		setup    func(mock sqlmock.Sqlmock)
		wantErr  bool
	}{
		{
			name:     "success",
			backupID: 1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM configuration_backups WHERE id").
					WithArgs(1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
		},
		{
			name:     "database error",
			backupID: 1,
			setup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("DELETE FROM configuration_backups WHERE id").
					WillReturnError(sql.ErrConnDone)
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, mock := newMockConfigRepo(t)
			tt.setup(mock)

			err := repo.DeleteConfigurationBackup(tt.backupID)
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
// CleanupOldHistory
// ---------------------------------------------------------------------------

func TestConfigurationRepository_CleanupOldHistory(t *testing.T) {
	olderThan := time.Now().Add(-30 * 24 * time.Hour)

	repo, mock := newMockConfigRepo(t)
	mock.ExpectExec("DELETE FROM system_configuration_history WHERE created_at").
		WithArgs(olderThan).
		WillReturnResult(sqlmock.NewResult(0, 5))

	err := repo.CleanupOldHistory(olderThan)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ---------------------------------------------------------------------------
// GetConfigurationStatistics
// ---------------------------------------------------------------------------

func TestConfigurationRepository_GetConfigurationStatistics(t *testing.T) {
	now := time.Now()

	t.Run("success", func(t *testing.T) {
		repo, mock := newMockConfigRepo(t)

		mock.ExpectQuery("SELECT COUNT.+ FROM system_configuration_history").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))
		mock.ExpectQuery("SELECT COUNT.+ FROM configuration_backups").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
		mock.ExpectQuery("SELECT COUNT.+ FROM configuration_templates").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(5))
		mock.ExpectQuery("SELECT COUNT.+ FROM wizard_completion").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))
		mock.ExpectQuery("SELECT updated_at FROM system_configuration").
			WillReturnRows(sqlmock.NewRows([]string{"updated_at"}).AddRow(now))

		stats, err := repo.GetConfigurationStatistics()
		require.NoError(t, err)
		assert.Equal(t, 10, stats.TotalConfigurations)
		assert.Equal(t, 3, stats.TotalBackups)
		assert.Equal(t, 5, stats.TotalTemplates)
		assert.Equal(t, 2, stats.WizardCompletions)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("database error on total configurations", func(t *testing.T) {
		repo, mock := newMockConfigRepo(t)

		mock.ExpectQuery("SELECT COUNT.+ FROM system_configuration_history").
			WillReturnError(sql.ErrConnDone)

		_, err := repo.GetConfigurationStatistics()
		assert.Error(t, err)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}

// ---------------------------------------------------------------------------
// DeleteConfigurationTemplate
// ---------------------------------------------------------------------------

func TestConfigurationRepository_DeleteConfigurationTemplate(t *testing.T) {
	repo, mock := newMockConfigRepo(t)
	mock.ExpectExec("DELETE FROM configuration_templates WHERE id").
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := repo.DeleteConfigurationTemplate(1)
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ===========================================================================
// Real SQLite-backed tests for uncovered functions
// ===========================================================================

func newRealConfigRepo(t *testing.T) *ConfigurationRepository {
	t.Helper()
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { sqlDB.Close() })

	db := database.WrapDB(sqlDB, database.DialectSQLite)

	_, err = sqlDB.Exec(`
		CREATE TABLE system_configuration (
			id INTEGER PRIMARY KEY,
			version TEXT NOT NULL,
			configuration TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`
		CREATE TABLE system_configuration_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version TEXT NOT NULL,
			configuration TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`
		CREATE TABLE configuration_backups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			version TEXT NOT NULL,
			configuration TEXT NOT NULL,
			created_at DATETIME NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`
		CREATE TABLE configuration_templates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			category TEXT NOT NULL,
			configuration TEXT NOT NULL,
			created_at DATETIME NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`
		CREATE TABLE wizard_progress (
			user_id INTEGER PRIMARY KEY,
			current_step TEXT NOT NULL,
			step_data TEXT NOT NULL,
			all_data TEXT NOT NULL,
			updated_at DATETIME NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`
		CREATE TABLE wizard_completion (
			user_id INTEGER PRIMARY KEY,
			completed_at DATETIME NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`
		CREATE TABLE wizard_sessions (
			session_id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			current_step INTEGER NOT NULL,
			total_steps INTEGER NOT NULL,
			step_data TEXT NOT NULL,
			configuration TEXT NOT NULL,
			started_at DATETIME NOT NULL,
			last_activity DATETIME NOT NULL,
			is_completed INTEGER NOT NULL DEFAULT 0,
			config_type TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	_, err = sqlDB.Exec(`
		CREATE TABLE configuration_profiles (
			profile_id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			description TEXT NOT NULL,
			user_id INTEGER NOT NULL,
			configuration TEXT NOT NULL,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			is_active INTEGER NOT NULL DEFAULT 1,
			tags TEXT NOT NULL
		)
	`)
	require.NoError(t, err)

	return NewConfigurationRepository(db)
}

// ---------------------------------------------------------------------------
// SaveConfigurationHistory
// ---------------------------------------------------------------------------

func TestConfigurationRepository_SaveConfigurationHistory_Real(t *testing.T) {
	repo := newRealConfigRepo(t)

	now := time.Now().Truncate(time.Second)
	config := &models.SystemConfiguration{
		Version:   "1.0.0",
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := repo.SaveConfigurationHistory(config)
	require.NoError(t, err)

	// Verify by reading back
	history, err := repo.GetConfigurationHistory(10)
	require.NoError(t, err)
	require.Len(t, history, 1)
	assert.Equal(t, "1.0.0", history[0].Version)
}

// ---------------------------------------------------------------------------
// CreateConfigurationBackup
// ---------------------------------------------------------------------------

func TestConfigurationRepository_CreateConfigurationBackup_Real(t *testing.T) {
	repo := newRealConfigRepo(t)

	now := time.Now().Truncate(time.Second)
	config := &models.SystemConfiguration{
		Version:   "2.0.0",
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := repo.CreateConfigurationBackup("my-backup", config)
	require.NoError(t, err)

	// Verify via GetConfigurationBackups
	backups, err := repo.GetConfigurationBackups()
	require.NoError(t, err)
	require.Len(t, backups, 1)
	assert.Equal(t, "my-backup", backups[0].Name)
	assert.Equal(t, "2.0.0", backups[0].Version)
}

// ---------------------------------------------------------------------------
// RestoreConfigurationBackup
// ---------------------------------------------------------------------------

func TestConfigurationRepository_RestoreConfigurationBackup_Real(t *testing.T) {
	repo := newRealConfigRepo(t)

	now := time.Now().Truncate(time.Second)
	config := &models.SystemConfiguration{
		Version:   "3.0.0",
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := repo.CreateConfigurationBackup("restore-test", config)
	require.NoError(t, err)

	backups, err := repo.GetConfigurationBackups()
	require.NoError(t, err)
	require.Len(t, backups, 1)

	t.Run("restore succeeds", func(t *testing.T) {
		restored, err := repo.RestoreConfigurationBackup(backups[0].ID)
		require.NoError(t, err)
		assert.Equal(t, "3.0.0", restored.Version)

		// Verify it's set as current config
		current, err := repo.GetConfiguration()
		require.NoError(t, err)
		assert.Equal(t, "3.0.0", current.Version)
	})

	t.Run("restore nonexistent backup", func(t *testing.T) {
		_, err := repo.RestoreConfigurationBackup(999)
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// GetConfigurationBackups (real) + DeleteConfigurationBackup (real)
// ---------------------------------------------------------------------------

func TestConfigurationRepository_GetAndDeleteConfigurationBackups_Real(t *testing.T) {
	repo := newRealConfigRepo(t)

	now := time.Now().Truncate(time.Second)
	config := &models.SystemConfiguration{Version: "1.0", CreatedAt: now, UpdatedAt: now}

	err := repo.CreateConfigurationBackup("b1", config)
	require.NoError(t, err)
	err = repo.CreateConfigurationBackup("b2", config)
	require.NoError(t, err)

	backups, err := repo.GetConfigurationBackups()
	require.NoError(t, err)
	assert.Len(t, backups, 2)

	err = repo.DeleteConfigurationBackup(backups[0].ID)
	require.NoError(t, err)

	backups, err = repo.GetConfigurationBackups()
	require.NoError(t, err)
	assert.Len(t, backups, 1)
}

// ---------------------------------------------------------------------------
// GetConfigurationTemplates
// ---------------------------------------------------------------------------

func TestConfigurationRepository_GetConfigurationTemplates_Real(t *testing.T) {
	repo := newRealConfigRepo(t)

	now := time.Now().Truncate(time.Second)
	tmpl := &models.ConfigurationTemplate{
		Name:        "Default",
		Description: "Default configuration",
		Category:    "general",
		Configuration: &models.SystemConfiguration{
			Version: "1.0.0",
		},
		CreatedAt: now,
	}

	err := repo.CreateConfigurationTemplate(tmpl)
	require.NoError(t, err)

	t.Run("returns templates", func(t *testing.T) {
		templates, err := repo.GetConfigurationTemplates()
		require.NoError(t, err)
		require.Len(t, templates, 1)
		assert.Equal(t, "Default", templates[0].Name)
		assert.Equal(t, "general", templates[0].Category)
		assert.NotNil(t, templates[0].Configuration)
	})

	t.Run("empty when all deleted", func(t *testing.T) {
		err := repo.DeleteConfigurationTemplate(tmpl.ID)
		require.NoError(t, err)

		templates, err := repo.GetConfigurationTemplates()
		require.NoError(t, err)
		assert.Empty(t, templates)
	})
}

// ---------------------------------------------------------------------------
// SaveWizardSession / GetWizardSession
// ---------------------------------------------------------------------------

func TestConfigurationRepository_SaveAndGetWizardSession_Real(t *testing.T) {
	repo := newRealConfigRepo(t)

	now := time.Now().Truncate(time.Second)
	session := &models.WizardSession{
		SessionID:     "sess-123",
		UserID:        1,
		CurrentStep:   2,
		TotalSteps:    5,
		StepData:      map[string]interface{}{"name": "test"},
		Configuration: map[string]interface{}{"db_type": "sqlite"},
		StartedAt:     now,
		LastActivity:  now,
		IsCompleted:   false,
		ConfigType:    "initial",
	}

	err := repo.SaveWizardSession(session)
	require.NoError(t, err)

	t.Run("get existing session", func(t *testing.T) {
		got, err := repo.GetWizardSession("sess-123")
		require.NoError(t, err)
		assert.Equal(t, "sess-123", got.SessionID)
		assert.Equal(t, 1, got.UserID)
		assert.Equal(t, 2, got.CurrentStep)
		assert.Equal(t, 5, got.TotalSteps)
		assert.Equal(t, "initial", got.ConfigType)
		assert.False(t, got.IsCompleted)
	})

	t.Run("session not found", func(t *testing.T) {
		_, err := repo.GetWizardSession("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "wizard session not found")
	})

	t.Run("update existing session", func(t *testing.T) {
		session.CurrentStep = 4
		session.IsCompleted = true
		err := repo.SaveWizardSession(session)
		require.NoError(t, err)

		got, err := repo.GetWizardSession("sess-123")
		require.NoError(t, err)
		assert.Equal(t, 4, got.CurrentStep)
		assert.True(t, got.IsCompleted)
	})
}

// ---------------------------------------------------------------------------
// GetConfigurationHistory (real, with data)
// ---------------------------------------------------------------------------

func TestConfigurationRepository_GetConfigurationHistory_Real(t *testing.T) {
	repo := newRealConfigRepo(t)

	now := time.Now().Truncate(time.Second)

	for i := 0; i < 3; i++ {
		config := &models.SystemConfiguration{
			Version:   "1.0." + string(rune('0'+i)),
			CreatedAt: now.Add(time.Duration(i) * time.Hour),
			UpdatedAt: now.Add(time.Duration(i) * time.Hour),
		}
		err := repo.SaveConfigurationHistory(config)
		require.NoError(t, err)
	}

	t.Run("returns history limited", func(t *testing.T) {
		history, err := repo.GetConfigurationHistory(2)
		require.NoError(t, err)
		assert.Len(t, history, 2)
	})

	t.Run("returns all history", func(t *testing.T) {
		history, err := repo.GetConfigurationHistory(10)
		require.NoError(t, err)
		assert.Len(t, history, 3)
	})
}

// ---------------------------------------------------------------------------
// ApplyConfigurationTemplate
// ---------------------------------------------------------------------------

func TestConfigurationRepository_ApplyConfigurationTemplate_Real(t *testing.T) {
	repo := newRealConfigRepo(t)

	now := time.Now().Truncate(time.Second)
	tmpl := &models.ConfigurationTemplate{
		Name:        "Production",
		Description: "Production config",
		Category:    "production",
		Configuration: &models.SystemConfiguration{
			Version: "5.0.0",
		},
		CreatedAt: now,
	}

	err := repo.CreateConfigurationTemplate(tmpl)
	require.NoError(t, err)

	t.Run("apply template", func(t *testing.T) {
		applied, err := repo.ApplyConfigurationTemplate(tmpl.ID)
		require.NoError(t, err)
		assert.Equal(t, "5.0.0", applied.Version)

		// Verify current configuration was updated
		current, err := repo.GetConfiguration()
		require.NoError(t, err)
		assert.Equal(t, "5.0.0", current.Version)
	})

	t.Run("apply nonexistent template", func(t *testing.T) {
		_, err := repo.ApplyConfigurationTemplate(999)
		assert.Error(t, err)
	})
}

// ---------------------------------------------------------------------------
// SaveConfigurationProfile / GetConfigurationProfile / GetUserConfigurationProfiles
// ---------------------------------------------------------------------------

func TestConfigurationRepository_ConfigurationProfiles_Real(t *testing.T) {
	repo := newRealConfigRepo(t)

	now := time.Now().Truncate(time.Second)
	profile := &models.ConfigurationProfile{
		ProfileID:     "prof-1",
		Name:          "Dev Profile",
		Description:   "Development settings",
		UserID:        1,
		Configuration: map[string]interface{}{"debug": true},
		CreatedAt:     now,
		UpdatedAt:     now,
		IsActive:      true,
		Tags:          []string{"dev", "local"},
	}

	t.Run("save profile", func(t *testing.T) {
		err := repo.SaveConfigurationProfile(profile)
		require.NoError(t, err)
	})

	t.Run("get profile by ID", func(t *testing.T) {
		got, err := repo.GetConfigurationProfile("prof-1")
		require.NoError(t, err)
		assert.Equal(t, "Dev Profile", got.Name)
		assert.Equal(t, "Development settings", got.Description)
		assert.Equal(t, 1, got.UserID)
		assert.True(t, got.IsActive)
		assert.Equal(t, []string{"dev", "local"}, got.Tags)
	})

	t.Run("get nonexistent profile", func(t *testing.T) {
		_, err := repo.GetConfigurationProfile("nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "configuration profile not found")
	})

	t.Run("get user profiles", func(t *testing.T) {
		// Add another profile for user 1
		profile2 := &models.ConfigurationProfile{
			ProfileID:     "prof-2",
			Name:          "Test Profile",
			Description:   "Test settings",
			UserID:        1,
			Configuration: map[string]interface{}{"test": true},
			CreatedAt:     now.Add(time.Hour),
			UpdatedAt:     now.Add(time.Hour),
			IsActive:      false,
			Tags:          []string{"test"},
		}
		err := repo.SaveConfigurationProfile(profile2)
		require.NoError(t, err)

		profiles, err := repo.GetUserConfigurationProfiles(1)
		require.NoError(t, err)
		assert.Len(t, profiles, 2)
	})

	t.Run("get user profiles empty", func(t *testing.T) {
		profiles, err := repo.GetUserConfigurationProfiles(999)
		require.NoError(t, err)
		assert.Empty(t, profiles)
	})
}
