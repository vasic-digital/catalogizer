package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDefaultConfig(t *testing.T) {
	config := getDefaultConfig()

	assert.NotNil(t, config)

	// Server defaults
	assert.Equal(t, "localhost", config.Server.Host)
	assert.Equal(t, 28080, config.Server.Port)
	assert.True(t, config.Server.EnableCORS)
	assert.True(t, config.Server.EnableHTTPS)

	// Database defaults
	assert.Equal(t, "./catalog.db", config.Database.Path)
	assert.Equal(t, 25, config.Database.MaxOpenConnections)
	assert.True(t, config.Database.EnableWAL)

	// Auth defaults
	assert.Equal(t, 24, config.Auth.JWTExpirationHours)
	assert.True(t, config.Auth.EnableAuth)

	// Catalog defaults
	assert.Equal(t, 100, config.Catalog.DefaultPageSize)
	assert.Equal(t, 1000, config.Catalog.MaxPageSize)
	assert.True(t, config.Catalog.EnableCache)

	// Logging defaults
	assert.Equal(t, "info", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
}

func TestValidateConfig_ValidConfig(t *testing.T) {
	// Set required env vars
	os.Setenv("JWT_SECRET", "this-is-a-super-long-secret-key-for-testing")
	os.Setenv("ADMIN_USERNAME", "admin")
	os.Setenv("ADMIN_PASSWORD", "password123")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("ADMIN_USERNAME")
		os.Unsetenv("ADMIN_PASSWORD")
	}()

	config := getDefaultConfig()
	err := validateConfig(config)
	assert.NoError(t, err)
}

func TestValidateConfig_InvalidPort(t *testing.T) {
	config := getDefaultConfig()
	config.Auth.EnableAuth = false // Disable auth to isolate port validation

	tests := []struct {
		name string
		port int
	}{
		{"negative port", -1},
		{"zero port", 0},
		{"port too high", 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config.Server.Port = tt.port
			err := validateConfig(config)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "invalid server port")
		})
	}
}

func TestValidateConfig_EmptyDatabasePath(t *testing.T) {
	config := getDefaultConfig()
	config.Auth.EnableAuth = false
	config.Database.Type = "sqlite"
	config.Database.Path = ""

	err := validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database path cannot be empty for sqlite")
}

func TestValidateConfig_AuthValidation(t *testing.T) {
	config := getDefaultConfig()
	config.Auth.EnableAuth = true

	// Clear env vars
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("ADMIN_USERNAME")
	os.Unsetenv("ADMIN_PASSWORD")

	// No JWT secret
	err := validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JWT secret must be set")

	// JWT secret too short
	config.Auth.JWTSecret = "short"
	err = validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least 32 characters")

	// Valid JWT but no admin credentials
	config.Auth.JWTSecret = "this-is-a-super-long-secret-key-for-testing"
	err = validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "admin credentials must be set")
}

func TestValidateConfig_PageSizeValidation(t *testing.T) {
	os.Setenv("JWT_SECRET", "this-is-a-super-long-secret-key-for-testing")
	os.Setenv("ADMIN_USERNAME", "admin")
	os.Setenv("ADMIN_PASSWORD", "password123")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("ADMIN_USERNAME")
		os.Unsetenv("ADMIN_PASSWORD")
	}()

	config := getDefaultConfig()

	// Invalid default page size
	config.Catalog.DefaultPageSize = 0
	err := validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default page size must be positive")

	// Max page size less than default
	config.Catalog.DefaultPageSize = 100
	config.Catalog.MaxPageSize = 50
	err = validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max page size must be >= default page size")
}

func TestGetDatabaseURL_Postgres(t *testing.T) {
	config := getDefaultConfig()
	// Default is postgres
	url := config.GetDatabaseURL()
	assert.Contains(t, url, "postgres://")
	assert.Contains(t, url, "catalogizer")
	assert.Contains(t, url, "sslmode=disable")
}

func TestGetDatabaseURL_SQLite(t *testing.T) {
	config := getDefaultConfig()
	config.Database.Type = "sqlite"
	config.Database.Path = "./catalog.db"
	config.Database.BusyTimeout = 5000
	config.Database.EnableWAL = true

	url := config.GetDatabaseURL()
	assert.Contains(t, url, "./catalog.db")
	assert.Contains(t, url, "_busy_timeout=5000")
	assert.Contains(t, url, "_journal_mode=WAL")
	assert.Contains(t, url, "_foreign_keys=1")
	assert.Contains(t, url, "_wal_autocheckpoint=1000")
}

func TestGetDatabaseURL_WithCustomCacheSize(t *testing.T) {
	config := getDefaultConfig()
	config.Database.Type = "sqlite"
	config.Database.Path = "./catalog.db"
	config.Database.CacheSize = -4000

	url := config.GetDatabaseURL()
	assert.Contains(t, url, "_cache_size=-4000")
}

func TestGetDatabaseURL_DisabledWAL(t *testing.T) {
	config := getDefaultConfig()
	config.Database.Type = "sqlite"
	config.Database.Path = "./catalog.db"
	config.Database.EnableWAL = false

	url := config.GetDatabaseURL()
	assert.NotContains(t, url, "_wal_autocheckpoint")
}

func TestGetServerAddress(t *testing.T) {
	config := getDefaultConfig()

	addr := config.GetServerAddress()
	assert.Equal(t, "localhost:28080", addr)

	config.Server.Host = "0.0.0.0"
	config.Server.Port = 9000
	addr = config.GetServerAddress()
	assert.Equal(t, "0.0.0.0:9000", addr)
}

func TestLoadConfig_CreateDefault(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.json")

	// Disable auth for simpler testing
	os.Setenv("JWT_SECRET", "this-is-a-super-long-secret-key-for-testing")
	os.Setenv("ADMIN_USERNAME", "admin")
	os.Setenv("ADMIN_PASSWORD", "password123")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("ADMIN_USERNAME")
		os.Unsetenv("ADMIN_PASSWORD")
	}()

	// Load config (should create default)
	config, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)

	// File should now exist
	_, err = os.Stat(configPath)
	assert.NoError(t, err)
}

func TestLoadConfig_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	err := os.WriteFile(configPath, []byte("not valid json"), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.json")

	config := getDefaultConfig()
	err := saveConfig(config, configPath)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(configPath)
	assert.NoError(t, err)

	// Verify permissions (should be 0600)
	info, _ := os.Stat(configPath)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

func TestLoadConfig_CreatesDefaultFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "new-config.json")

	// The auto-create path validates like any other load, so secrets must be set.
	t.Setenv("JWT_SECRET", "f3a91c07d2b84e5a9c6d1e0b7a38f245c9d0e1f2a3b4c5d6")
	t.Setenv("ADMIN_USERNAME", "admin")
	t.Setenv("ADMIN_PASSWORD", "a-real-looking-pass-9f3k")

	config, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.NotNil(t, config)
	assert.Equal(t, 28080, config.Server.Port)

	// File should now exist
	_, err = os.Stat(configPath)
	assert.NoError(t, err)
}

func TestLoadConfig_LoadsExistingValidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Set env vars for validation
	os.Setenv("JWT_SECRET", "this-is-a-super-long-secret-key-for-testing-purposes")
	os.Setenv("ADMIN_USERNAME", "admin")
	os.Setenv("ADMIN_PASSWORD", "password123")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("ADMIN_USERNAME")
		os.Unsetenv("ADMIN_PASSWORD")
	}()

	// Create a valid config file first
	config := getDefaultConfig()
	err := saveConfig(config, configPath)
	require.NoError(t, err)

	// Load it back
	loaded, err := LoadConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, config.Server.Port, loaded.Server.Port)
	assert.Equal(t, config.Database.Path, loaded.Database.Path)
}

func TestLoadConfig_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create config with invalid port
	config := getDefaultConfig()
	config.Server.Port = -1
	config.Auth.EnableAuth = false
	err := saveConfig(config, configPath)
	require.NoError(t, err)

	_, err = LoadConfig(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid configuration")
}

func TestValidateConfig_SQLiteValid(t *testing.T) {
	config := getDefaultConfig()
	config.Database.Type = "sqlite"
	config.Database.Path = "./test.db"
	config.Auth.EnableAuth = false

	err := validateConfig(config)
	assert.NoError(t, err)
}

func TestValidateConfig_SQLiteMissingPath(t *testing.T) {
	config := getDefaultConfig()
	config.Database.Type = "sqlite"
	config.Database.Path = ""
	config.Auth.EnableAuth = false

	err := validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database path cannot be empty")
}

func TestValidateConfig_UnsupportedDBType(t *testing.T) {
	config := getDefaultConfig()
	config.Database.Type = "mysql"
	config.Auth.EnableAuth = false

	err := validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported database type")
}

func TestValidateConfig_PostgresMissingHost(t *testing.T) {
	config := getDefaultConfig()
	config.Database.Type = "postgres"
	config.Database.Host = ""
	config.Auth.EnableAuth = false

	err := validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database host cannot be empty")
}

func TestValidateConfig_PostgresMissingName(t *testing.T) {
	config := getDefaultConfig()
	config.Database.Type = "postgres"
	config.Database.Host = "localhost"
	config.Database.Name = ""
	config.Auth.EnableAuth = false

	err := validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database name cannot be empty")
}

func TestValidateConfig_DatabaseEnvOverrides(t *testing.T) {
	os.Setenv("DATABASE_TYPE", "sqlite")
	os.Setenv("DATABASE_HOST", "override-host")
	os.Setenv("DATABASE_PORT", "9999")
	os.Setenv("DATABASE_NAME", "override-db")
	os.Setenv("DATABASE_USER", "override-user")
	os.Setenv("DATABASE_PASSWORD", "override-pass")
	os.Setenv("DATABASE_SSL_MODE", "require")
	defer func() {
		os.Unsetenv("DATABASE_TYPE")
		os.Unsetenv("DATABASE_HOST")
		os.Unsetenv("DATABASE_PORT")
		os.Unsetenv("DATABASE_NAME")
		os.Unsetenv("DATABASE_USER")
		os.Unsetenv("DATABASE_PASSWORD")
		os.Unsetenv("DATABASE_SSL_MODE")
	}()

	config := getDefaultConfig()
	config.Auth.EnableAuth = false
	err := validateConfig(config)
	assert.NoError(t, err)

	assert.Equal(t, "sqlite", config.Database.Type)
	assert.Equal(t, "override-host", config.Database.Host)
	assert.Equal(t, 9999, config.Database.Port)
	assert.Equal(t, "override-db", config.Database.Name)
	assert.Equal(t, "override-user", config.Database.User)
	assert.Equal(t, "override-pass", config.Database.Password)
	assert.Equal(t, "require", config.Database.SSLMode)
}

func TestValidateConfig_PostgresDBFallback(t *testing.T) {
	// POSTGRES_DB is the env var the postgres container sets (see
	// docker-compose.dev.yml / docker-compose.yml). The API must honor it as a
	// fallback for the database name when DATABASE_NAME is unset, while
	// DATABASE_NAME (when present) always wins.

	t.Run("POSTGRES_DB sets name when DATABASE_NAME unset", func(t *testing.T) {
		os.Unsetenv("DATABASE_NAME")
		os.Setenv("POSTGRES_DB", "catalogizer_dev")
		defer os.Unsetenv("POSTGRES_DB")

		config := getDefaultConfig()
		config.Auth.EnableAuth = false
		err := validateConfig(config)
		assert.NoError(t, err)
		assert.Equal(t, "catalogizer_dev", config.Database.Name)
	})

	t.Run("DATABASE_NAME wins when both set", func(t *testing.T) {
		os.Setenv("DATABASE_NAME", "explicit_db")
		os.Setenv("POSTGRES_DB", "container_db")
		defer func() {
			os.Unsetenv("DATABASE_NAME")
			os.Unsetenv("POSTGRES_DB")
		}()

		config := getDefaultConfig()
		config.Auth.EnableAuth = false
		err := validateConfig(config)
		assert.NoError(t, err)
		assert.Equal(t, "explicit_db", config.Database.Name)
	})
}

func TestValidateConfig_AuthEnvOverrides(t *testing.T) {
	os.Setenv("JWT_SECRET", "this-is-a-very-long-jwt-secret-for-test-purposes-only")
	os.Setenv("ADMIN_USERNAME", "env-admin")
	os.Setenv("ADMIN_PASSWORD", "env-password")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("ADMIN_USERNAME")
		os.Unsetenv("ADMIN_PASSWORD")
	}()

	config := getDefaultConfig()
	config.Auth.EnableAuth = true
	err := validateConfig(config)
	assert.NoError(t, err)

	assert.Equal(t, "this-is-a-very-long-jwt-secret-for-test-purposes-only", config.Auth.JWTSecret)
	assert.Equal(t, "env-admin", config.Auth.AdminUsername)
	assert.Equal(t, "env-password", config.Auth.AdminPassword)
}

func TestValidateConfig_ShortJWTSecret(t *testing.T) {
	os.Setenv("JWT_SECRET", "short")
	os.Setenv("ADMIN_USERNAME", "admin")
	os.Setenv("ADMIN_PASSWORD", "password")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("ADMIN_USERNAME")
		os.Unsetenv("ADMIN_PASSWORD")
	}()

	config := getDefaultConfig()
	config.Auth.EnableAuth = true
	err := validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "at least 32 characters")
}

func TestValidateConfig_MissingAdminCredentials(t *testing.T) {
	os.Setenv("JWT_SECRET", "this-is-a-very-long-jwt-secret-for-test-purposes-only")
	os.Unsetenv("ADMIN_USERNAME")
	os.Unsetenv("ADMIN_PASSWORD")
	defer os.Unsetenv("JWT_SECRET")

	config := getDefaultConfig()
	config.Auth.EnableAuth = true
	config.Auth.AdminUsername = ""
	config.Auth.AdminPassword = ""
	err := validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "admin credentials")
}

func TestValidateConfig_InvalidPageSize(t *testing.T) {
	config := getDefaultConfig()
	config.Auth.EnableAuth = false
	config.Catalog.DefaultPageSize = 0
	err := validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default page size must be positive")
}

func TestValidateConfig_MaxPageSizeLessThanDefault(t *testing.T) {
	config := getDefaultConfig()
	config.Auth.EnableAuth = false
	config.Catalog.DefaultPageSize = 100
	config.Catalog.MaxPageSize = 50
	err := validateConfig(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max page size must be >= default page size")
}

func TestGetDatabaseURL_EmptyType(t *testing.T) {
	config := getDefaultConfig()
	config.Database.Type = ""

	url := config.GetDatabaseURL()
	assert.Contains(t, url, "postgres://")
}

func TestGetDatabaseURL_EmptySSLMode(t *testing.T) {
	config := getDefaultConfig()
	config.Database.Type = "postgres"
	config.Database.SSLMode = ""

	url := config.GetDatabaseURL()
	assert.Contains(t, url, "sslmode=disable")
}
