package tests

import (
	"context"
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

	db := database.WrapDB(sqlDB, database.DialectSQLite)

	// Build the schema with the real migrations (PX-F6): a hand-written copy drifts from them
	// (any later migration, for example v25 adding conversion_jobs.warning, would otherwise have to be mirrored by hand).
	require.NoError(t, db.RunMigrations(context.Background()))

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
		Name:        "ConversionAdmin", // "Admin" is seeded by the migrations
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
	sourceFile := filepath.Join(tempDir, "test.pdf")
	targetFile := filepath.Join(tempDir, "test_copy.txt")

	// Create a test source file
	err = os.WriteFile(sourceFile, []byte("This is a test file"), 0644)
	require.NoError(t, err)

	// Test 1: Create conversion job
	request := &models.ConversionRequest{
		SourcePath:     sourceFile,
		TargetPath:     targetFile,
		SourceFormat:   "pdf",
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
