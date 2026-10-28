package tests

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/services"

	"database/sql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/mutecomm/go-sqlcipher"
)

// TestConversionIntegration tests the full conversion flow
func TestConversionIntegration(t *testing.T) {
	// Setup in-memory database
	sqlDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	defer sqlDB.Close()

	// Create database schema
	err = createConversionSchema(sqlDB)
	require.NoError(t, err)

	db := database.WrapDB(sqlDB, database.DialectSQLite)

	// Create repositories
	userRepo := repository.NewUserRepository(db)
	conversionRepo := repository.NewConversionRepository(db)

	// Create test user with conversion permissions
	user := &models.User{
		Username:     "testuser",
		Email:        "test@example.com",
		PasswordHash: "hash",
		Salt:         "salt",
		RoleID:       1, // Admin role
		IsActive:     true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	userID, err := userRepo.Create(user)
	require.NoError(t, err)
	user.ID = userID

	// Create test role with permissions
	roleDesc := "Administrator"
	role := &models.Role{
		Name:        "Admin",
		Description: &roleDesc,
		Permissions: models.Permissions{
			models.PermissionConversionCreate,
			models.PermissionConversionView,
			models.PermissionConversionManage,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	roleID, err := userRepo.CreateRole(role)
	require.NoError(t, err)
	role.ID = roleID

	// Update user role
	user.RoleID = roleID
	err = userRepo.Update(user)
	require.NoError(t, err)

	// Create auth service
	jwtSecret := "test-secret"
	authService := services.NewAuthService(userRepo, jwtSecret)

	// Create conversion service
	conversionService := services.NewConversionService(conversionRepo, userRepo, authService)

	// Create temp directory for test files
	tempDir := t.TempDir()
	// conversion paths are confined to configured roots (pathguard); allow this test's dir
	t.Setenv("CATALOGIZER_CONVERSION_ROOTS", tempDir)
	sourceFile := filepath.Join(tempDir, "test.txt")
	targetFile := filepath.Join(tempDir, "test_copy.txt")

	// Create a test source file
	err = os.WriteFile(sourceFile, []byte("This is a test file"), 0644)
	require.NoError(t, err)

	// Test 1: Create conversion job
	request := &models.ConversionRequest{
		SourcePath:     sourceFile,
		TargetPath:     targetFile,
		SourceFormat:   "txt",
		TargetFormat:   "txt",
		ConversionType: models.ConversionTypeDocument,
		Quality:        "medium",
		Priority:       1,
	}

	job, err := conversionService.CreateConversionJob(userID, request)
	require.NoError(t, err)
	assert.NotNil(t, job)
	assert.Equal(t, userID, job.UserID)
	assert.Equal(t, sourceFile, job.SourcePath)
	assert.Equal(t, targetFile, job.TargetPath)
	assert.Equal(t, models.ConversionStatusPending, job.Status)
	assert.Equal(t, models.ConversionTypeDocument, job.ConversionType)

	// Test 2: Get job by ID
	retrievedJob, err := conversionService.GetJob(job.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, job.ID, retrievedJob.ID)
	assert.Equal(t, job.SourcePath, retrievedJob.SourcePath)
	assert.Equal(t, job.TargetPath, retrievedJob.TargetPath)

	// Test 3: Get user jobs
	jobs, err := conversionService.GetUserJobs(userID, nil, 50, 0)
	require.NoError(t, err)
	assert.Len(t, jobs, 1)
	assert.Equal(t, job.ID, jobs[0].ID)

	// Test 4: Get supported formats
	formats := conversionService.GetSupportedFormats()
	assert.NotNil(t, formats)
	assert.NotEmpty(t, formats.Video.Input)
	assert.NotEmpty(t, formats.Audio.Input)
	assert.NotEmpty(t, formats.Image.Input)
	assert.NotEmpty(t, formats.Document.Input)

	// Test 5: Cancel job
	err = conversionService.CancelJob(job.ID, userID)
	require.NoError(t, err)

	// Verify job is cancelled
	cancelledJob, err := conversionService.GetJob(job.ID, userID)
	require.NoError(t, err)
	assert.Equal(t, models.ConversionStatusCancelled, cancelledJob.Status)
}

// createConversionSchema creates the necessary database tables for conversion testing
func createConversionSchema(db *sql.DB) error {
	// Create user table with all required columns
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			email_verified BOOLEAN DEFAULT 0,
			password_hash TEXT NOT NULL,
			salt TEXT NOT NULL,
			first_name TEXT,
			last_name TEXT,
			display_name TEXT,
			avatar_url TEXT,
			time_zone TEXT DEFAULT 'UTC',
			language TEXT DEFAULT 'en',
			settings TEXT DEFAULT '{}',
			role_id INTEGER,
			is_active BOOLEAN DEFAULT 1,
			is_locked BOOLEAN DEFAULT 0,
			locked_until DATETIME,
			failed_login_attempts INTEGER DEFAULT 0,
			last_login_at DATETIME,
			last_login_ip TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS roles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			description TEXT,
			permissions TEXT DEFAULT '[]',
			is_system INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS conversion_jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			source_path TEXT NOT NULL,
			target_path TEXT NOT NULL,
			source_format TEXT NOT NULL,
			target_format TEXT NOT NULL,
			conversion_type TEXT NOT NULL,
			quality TEXT,
			settings TEXT,
			status TEXT NOT NULL,
			progress INTEGER DEFAULT 0,
			error_message TEXT,
			priority INTEGER DEFAULT 1,
			scheduled_for DATETIME,
			duration DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			started_at DATETIME,
			completed_at DATETIME,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			return err
		}
	}

	return nil
}
