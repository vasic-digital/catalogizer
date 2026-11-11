package tests

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/services"

	_ "github.com/mutecomm/go-sqlcipher"
)

// TestDatabase represents a test database instance
type TestDatabase struct {
	DB   *database.DB
	Path string
}

// TestSuite represents a complete test suite with all dependencies
type TestSuite struct {
	DB                    *TestDatabase
	UserRepo              *repository.UserRepository
	AuthService           *services.AuthService
	AnalyticsRepo         *repository.AnalyticsRepository
	FavoritesRepo         *repository.FavoritesRepository
	ConversionRepo        *repository.ConversionRepository
	SyncRepo              *repository.SyncRepository
	ErrorRepo             *repository.ErrorReportingRepository
	CrashRepo             *repository.CrashReportingRepository
	LogRepo               *repository.LogManagementRepository
	ConfigRepo            *repository.ConfigurationRepository
	AnalyticsService      *services.AnalyticsService
	FavoritesService      *services.FavoritesService
	ConversionService     *services.ConversionService
	SyncService           *services.SyncService
	ErrorReportingService *services.ErrorReportingService
	LogManagementService  *services.LogManagementService
	ConfigurationService  *services.ConfigurationService
}

// SetupTestDatabase creates a test database with all required tables
func SetupTestDatabase(t *testing.T) *TestDatabase {
	// Create temporary database file
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	db := database.WrapDB(sqlDB, database.DialectSQLite)

	// Create all required tables
	if err := createTestTables(sqlDB); err != nil {
		t.Fatalf("Failed to create test tables: %v", err)
	}

	return &TestDatabase{
		DB:   db,
		Path: dbPath,
	}
}

// CleanupTestDatabase closes and removes the test database
func (td *TestDatabase) Cleanup() {
	if td.DB != nil {
		td.DB.Close()
	}
	if td.Path != "" {
		os.Remove(td.Path)
	}
}

// SetupTestSuite creates a complete test suite with all services and repositories
func SetupTestSuite(t *testing.T) *TestSuite {
	// Create test database
	testDB := SetupTestDatabase(t)

	// Create repositories
	userRepo := repository.NewUserRepository(testDB.DB)
	analyticsRepo := repository.NewAnalyticsRepository(testDB.DB)
	favoritesRepo := repository.NewFavoritesRepository(testDB.DB)
	conversionRepo := repository.NewConversionRepository(testDB.DB)
	syncRepo := repository.NewSyncRepository(testDB.DB)
	errorRepo := repository.NewErrorReportingRepository(testDB.DB)
	crashRepo := repository.NewCrashReportingRepository(testDB.DB)
	logRepo := repository.NewLogManagementRepository(testDB.DB)
	configRepo := repository.NewConfigurationRepository(testDB.DB)

	// Create auth service
	authService := services.NewAuthService(userRepo, "test-jwt-secret")

	// Create services
	analyticsService := services.NewAnalyticsService(analyticsRepo)
	favoritesService := services.NewFavoritesService(favoritesRepo, authService)
	conversionService := services.NewConversionService(conversionRepo, userRepo, authService)
	syncService := services.NewSyncService(syncRepo, userRepo, authService)
	errorReportingService := services.NewErrorReportingService(errorRepo, crashRepo)
	logManagementService := services.NewLogManagementService(logRepo)
	configurationService := services.NewConfigurationService(configRepo, "/tmp/test_config.json")

	return &TestSuite{
		DB:                    testDB,
		UserRepo:              userRepo,
		AuthService:           authService,
		AnalyticsRepo:         analyticsRepo,
		FavoritesRepo:         favoritesRepo,
		ConversionRepo:        conversionRepo,
		SyncRepo:              syncRepo,
		ErrorRepo:             errorRepo,
		CrashRepo:             crashRepo,
		LogRepo:               logRepo,
		ConfigRepo:            configRepo,
		AnalyticsService:      analyticsService,
		FavoritesService:      favoritesService,
		ConversionService:     conversionService,
		SyncService:           syncService,
		ErrorReportingService: errorReportingService,
		LogManagementService:  logManagementService,
		ConfigurationService:  configurationService,
	}
}

// Cleanup cleans up the test suite
func (ts *TestSuite) Cleanup() {
	if ts.DB != nil {
		ts.DB.Cleanup()
	}
}

// CreateTestUser creates a test user for testing
func CreateTestUser(t *testing.T, db *database.DB, userID int) *models.User {
	user := &models.User{
		ID:       userID,
		Username: fmt.Sprintf("testuser%d", userID),
		Email:    fmt.Sprintf("test%d@example.com", userID),
		RoleID:   1,
		IsActive: true,
	}

	query := `INSERT INTO users (id, username, email, role_id, is_active, created_at) VALUES (?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query, user.ID, user.Username, user.Email, user.RoleID, user.IsActive, time.Now())
	if err != nil {
		t.Fatalf("Failed to create test user: %v", err)
	}

	return user
}

// CreateTestMediaItem creates a test media item for testing
func CreateTestMediaItem(t *testing.T, db *database.DB, itemID int, userID int) *models.MediaItem {
	item := &models.MediaItem{
		ID:     itemID,
		UserID: userID,
		Title:  fmt.Sprintf("Test Media %d", itemID),
		Type:   "video",
		Path:   fmt.Sprintf("/test/media/%d.mp4", itemID),
		Size:   1024 * 1024,
	}

	query := `INSERT INTO media_items (id, user_id, title, type, path, size, created_at, updated_at)
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := db.Exec(query, item.ID, item.UserID, item.Title, item.Type, item.Path, item.Size, time.Now(), time.Now())
	if err != nil {
		t.Fatalf("Failed to create test media item: %v", err)
	}

	return item
}

// AssertEqual checks if two values are equal
func AssertEqual(t *testing.T, expected, actual interface{}, message string) {
	if expected != actual {
		t.Errorf("%s: expected %v, got %v", message, expected, actual)
	}
}

// AssertNotNil checks if a value is not nil
func AssertNotNil(t *testing.T, value interface{}, message string) {
	if value == nil {
		t.Errorf("%s: expected non-nil value", message)
	}
}

// AssertNil checks if a value is nil
func AssertNil(t *testing.T, value interface{}, message string) {
	if value != nil {
		t.Errorf("%s: expected nil value, got %v", message, value)
	}
}

// AssertError checks if an error occurred
func AssertError(t *testing.T, err error, message string) {
	if err == nil {
		t.Errorf("%s: expected error but got none", message)
	}
}

// AssertNoError checks if no error occurred
func AssertNoError(t *testing.T, err error, message string) {
	if err != nil {
		t.Errorf("%s: unexpected error: %v", message, err)
	}
}

// AssertContains checks if a string contains a substring
func AssertContains(t *testing.T, str, substr, message string) {
	if !strings.Contains(str, substr) {
		t.Errorf("%s: expected '%s' to contain '%s'", message, str, substr)
	}
}

// AssertHTTPStatus checks HTTP response status
func AssertHTTPStatus(t *testing.T, expected int, response *httptest.ResponseRecorder, message string) {
	if response.Code != expected {
		t.Errorf("%s: expected status %d, got %d", message, expected, response.Code)
	}
}

// AssertJSONResponse checks JSON response structure
func AssertJSONResponse(t *testing.T, response *httptest.ResponseRecorder, expected interface{}, message string) {
	var actual interface{}
	err := json.Unmarshal(response.Body.Bytes(), &actual)
	if err != nil {
		t.Fatalf("%s: failed to parse JSON response: %v", message, err)
	}

	expectedJSON, _ := json.Marshal(expected)
	actualJSON, _ := json.Marshal(actual)

	if string(expectedJSON) != string(actualJSON) {
		t.Errorf("%s: JSON mismatch\nExpected: %s\nActual: %s", message, expectedJSON, actualJSON)
	}
}


// WaitForCondition waits for a condition to be true with timeout
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, message string) {
	start := time.Now()
	for time.Since(start) < timeout {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("%s: condition not met within timeout", message)
}

// GenerateTestData generates test data for various scenarios
type TestDataGenerator struct {
	UserID  int
	ItemID  int
	EventID int
}

func NewTestDataGenerator() *TestDataGenerator {
	return &TestDataGenerator{
		UserID:  1000,
		ItemID:  2000,
		EventID: 3000,
	}
}

func (g *TestDataGenerator) NextUserID() int {
	g.UserID++
	return g.UserID
}

func (g *TestDataGenerator) NextItemID() int {
	g.ItemID++
	return g.ItemID
}

func (g *TestDataGenerator) NextEventID() int {
	g.EventID++
	return g.EventID
}

// createTestTables creates all required tables for testing
func createTestTables(db *sql.DB) error {
	tables := []string{
		// Users table
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY,
			username TEXT UNIQUE NOT NULL,
			email TEXT UNIQUE NOT NULL,
			password_hash TEXT,
			salt TEXT,
			role_id INTEGER NOT NULL DEFAULT 1,
			first_name TEXT,
			last_name TEXT,
			display_name TEXT,
			avatar_url TEXT,
			time_zone TEXT,
			language TEXT,
			settings TEXT DEFAULT '{}',
			is_active BOOLEAN DEFAULT 1,
			is_locked BOOLEAN DEFAULT 0,
			locked_until DATETIME,
			failed_login_attempts INTEGER DEFAULT 0,
			last_login_at DATETIME,
			last_login_ip TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Media items table
		`CREATE TABLE IF NOT EXISTS media_items (
			id INTEGER PRIMARY KEY,
			user_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			type TEXT NOT NULL,
			path TEXT NOT NULL,
			size INTEGER,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,

		// Analytics events table
		`CREATE TABLE IF NOT EXISTS analytics_events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER,
			event_type TEXT NOT NULL,
			event_category TEXT,
			entity_type TEXT,
			entity_id INTEGER,
			data TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			session_id TEXT,
			ip_address TEXT,
			user_agent TEXT,
			device_info TEXT,
			location TEXT
		)`,

		// Favorites table
		`CREATE TABLE IF NOT EXISTS favorites (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			entity_type TEXT NOT NULL,
			entity_id INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, entity_type, entity_id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,

		// Conversion jobs table
		`CREATE TABLE IF NOT EXISTS conversion_jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			source_path TEXT NOT NULL,
			target_path TEXT NOT NULL,
			source_format TEXT NOT NULL,
			target_format TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			progress INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			started_at DATETIME,
			completed_at DATETIME,
			error_message TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,

		// Sync endpoints table
		`CREATE TABLE IF NOT EXISTS sync_endpoints (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			url TEXT NOT NULL,
			username TEXT,
			password TEXT,
			settings TEXT,
			status TEXT NOT NULL DEFAULT 'inactive',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,

		// Sync sessions table
		`CREATE TABLE IF NOT EXISTS sync_sessions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			endpoint_id INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			direction TEXT NOT NULL,
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			files_processed INTEGER DEFAULT 0,
			bytes_transferred INTEGER DEFAULT 0,
			error_message TEXT,
			FOREIGN KEY (endpoint_id) REFERENCES sync_endpoints(id)
		)`,

		// Stress tests table
		`CREATE TABLE IF NOT EXISTS stress_tests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			type TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'created',
			scenarios TEXT,
			configuration TEXT,
			concurrent_users INTEGER NOT NULL,
			duration_seconds INTEGER NOT NULL,
			ramp_up_time INTEGER DEFAULT 0,
			created_by INTEGER NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			started_at DATETIME,
			completed_at DATETIME,
			FOREIGN KEY (created_by) REFERENCES users(id)
		)`,

		// Stress test executions table
		`CREATE TABLE IF NOT EXISTS stress_test_executions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			stress_test_id INTEGER NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			started_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			metrics TEXT,
			results TEXT,
			error_message TEXT,
			FOREIGN KEY (stress_test_id) REFERENCES stress_tests(id)
		)`,

		// Error reports table
		`CREATE TABLE IF NOT EXISTS error_reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			level TEXT NOT NULL,
			message TEXT NOT NULL,
			error_code TEXT,
			component TEXT,
			stack_trace TEXT,
			context TEXT,
			system_info TEXT,
			user_agent TEXT,
			url TEXT,
			fingerprint TEXT,
			status TEXT NOT NULL DEFAULT 'new',
			reported_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			resolved_at DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,

		// Crash reports table
		`CREATE TABLE IF NOT EXISTS crash_reports (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			signal TEXT NOT NULL,
			message TEXT NOT NULL,
			stack_trace TEXT,
			context TEXT,
			system_info TEXT,
			fingerprint TEXT,
			status TEXT NOT NULL DEFAULT 'new',
			reported_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			resolved_at DATETIME,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,

		// Log collections table
		`CREATE TABLE IF NOT EXISTS log_collections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			components TEXT,
			log_level TEXT,
			start_time DATETIME,
			end_time DATETIME,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			completed_at DATETIME,
			status TEXT NOT NULL DEFAULT 'pending',
			entry_count INTEGER DEFAULT 0,
			filters TEXT,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,

		// Log entries table
		`CREATE TABLE IF NOT EXISTS log_entries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			collection_id INTEGER NOT NULL,
			timestamp DATETIME NOT NULL,
			level TEXT NOT NULL,
			component TEXT NOT NULL,
			message TEXT NOT NULL,
			context TEXT,
			FOREIGN KEY (collection_id) REFERENCES log_collections(id)
		)`,

		// Log shares table
		`CREATE TABLE IF NOT EXISTS log_shares (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			collection_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			share_token TEXT UNIQUE NOT NULL,
			share_type TEXT NOT NULL,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			accessed_at DATETIME,
			is_active BOOLEAN NOT NULL DEFAULT 1,
			permissions TEXT,
			recipients TEXT,
			FOREIGN KEY (collection_id) REFERENCES log_collections(id),
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,

		// System configuration table
		`CREATE TABLE IF NOT EXISTS system_configuration (
			id INTEGER PRIMARY KEY DEFAULT 1,
			version TEXT NOT NULL,
			configuration TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Wizard progress table
		`CREATE TABLE IF NOT EXISTS wizard_progress (
			user_id INTEGER PRIMARY KEY,
			current_step TEXT NOT NULL,
			step_data TEXT,
			all_data TEXT,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,

		// Wizard completion table
		`CREATE TABLE IF NOT EXISTS wizard_completion (
			user_id INTEGER PRIMARY KEY,
			completed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (user_id) REFERENCES users(id)
		)`,

		// Configuration history table
		`CREATE TABLE IF NOT EXISTS system_configuration_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			version TEXT NOT NULL,
			configuration TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Configuration backups table
		`CREATE TABLE IF NOT EXISTS configuration_backups (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			version TEXT NOT NULL,
			configuration TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Configuration templates table
		`CREATE TABLE IF NOT EXISTS configuration_templates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			category TEXT NOT NULL,
			configuration TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, table := range tables {
		if _, err := db.Exec(table); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	return nil
}

// Benchmark helper functions
func BenchmarkSetup(b *testing.B) *TestSuite {
	// Disable logging during benchmarks
	log.SetOutput(io.Discard)

	// Create a temporary database
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "benchmark.db")

	sqlDB, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		b.Fatalf("Failed to open benchmark database: %v", err)
	}

	if err := createTestTables(sqlDB); err != nil {
		b.Fatalf("Failed to create benchmark tables: %v", err)
	}

	testDB := &TestDatabase{DB: database.WrapDB(sqlDB, database.DialectSQLite), Path: dbPath}

	// Create repositories
	userRepo := repository.NewUserRepository(testDB.DB)
	analyticsRepo := repository.NewAnalyticsRepository(testDB.DB)
	favoritesRepo := repository.NewFavoritesRepository(testDB.DB)
	conversionRepo := repository.NewConversionRepository(testDB.DB)
	syncRepo := repository.NewSyncRepository(testDB.DB)
	errorRepo := repository.NewErrorReportingRepository(testDB.DB)
	crashRepo := repository.NewCrashReportingRepository(testDB.DB)
	logRepo := repository.NewLogManagementRepository(testDB.DB)
	configRepo := repository.NewConfigurationRepository(testDB.DB)

	// Create auth service
	authService := services.NewAuthService(userRepo, "test-jwt-secret")

	// Create services
	analyticsService := services.NewAnalyticsService(analyticsRepo)
	favoritesService := services.NewFavoritesService(favoritesRepo, authService)
	conversionService := services.NewConversionService(conversionRepo, userRepo, authService)
	syncService := services.NewSyncService(syncRepo, userRepo, authService)
	errorReportingService := services.NewErrorReportingService(errorRepo, crashRepo)
	logManagementService := services.NewLogManagementService(logRepo)
	configurationService := services.NewConfigurationService(configRepo, "/tmp/benchmark_config.json")

	return &TestSuite{
		DB:                    testDB,
		UserRepo:              userRepo,
		AuthService:           authService,
		AnalyticsRepo:         analyticsRepo,
		FavoritesRepo:         favoritesRepo,
		ConversionRepo:        conversionRepo,
		SyncRepo:              syncRepo,
		ErrorRepo:             errorRepo,
		CrashRepo:             crashRepo,
		LogRepo:               logRepo,
		ConfigRepo:            configRepo,
		AnalyticsService:      analyticsService,
		FavoritesService:      favoritesService,
		ConversionService:     conversionService,
		SyncService:           syncService,
		ErrorReportingService: errorReportingService,
		LogManagementService:  logManagementService,
		ConfigurationService:  configurationService,
	}
}
