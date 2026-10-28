package config

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config represents the API configuration
type Config struct {
	Server   ServerConfig   `json:"server"`
	Database DatabaseConfig `json:"database"`
	Auth     AuthConfig     `json:"auth"`
	Catalog  CatalogConfig  `json:"catalog"`
	Storage  StorageConfig  `json:"storage"`
	Logging  LoggingConfig  `json:"logging"`
	Proxy    ProxyConfig    `json:"proxy"`
}

// ServerConfig contains server-related configuration
type ServerConfig struct {
	Host         string `json:"host"`
	Port         int    `json:"port"`
	ReadTimeout  int    `json:"read_timeout"`
	WriteTimeout int    `json:"write_timeout"`
	IdleTimeout  int    `json:"idle_timeout"`
	EnableCORS   bool   `json:"enable_cors"`
	EnableHTTPS  bool   `json:"enable_https"`
}

// DatabaseConfig contains database connection configuration.
// Type selects the backend: "postgres" (default) or "sqlite".
type DatabaseConfig struct {
	Type string `json:"type"` // "postgres" or "sqlite"
	// PostgreSQL fields
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Name     string `json:"name"`
	User     string `json:"user"`
	Password string `json:"password"`
	SSLMode  string `json:"ssl_mode"`
	// SQLite fields (used when Type="sqlite")
	Path        string `json:"path"`
	EnableWAL   bool   `json:"enable_wal"`
	CacheSize   int    `json:"cache_size"`
	BusyTimeout int    `json:"busy_timeout"`
	// Common
	MaxOpenConnections int `json:"max_open_connections"`
	MaxIdleConnections int `json:"max_idle_connections"`
	ConnMaxLifetime    int `json:"conn_max_lifetime"`
	ConnMaxIdleTime    int `json:"conn_max_idle_time"`
}

// AuthConfig contains authentication configuration
type AuthConfig struct {
	JWTSecret          string `json:"jwt_secret"`
	JWTExpirationHours int    `json:"jwt_expiration_hours"`
	EnableAuth         bool   `json:"enable_auth"`
	AdminUsername      string `json:"admin_username"`
	AdminPassword      string `json:"admin_password"`
}

// CatalogConfig contains catalog-specific configuration
type CatalogConfig struct {
	DefaultPageSize      int      `json:"default_page_size"`
	MaxPageSize          int      `json:"max_page_size"`
	EnableCache          bool     `json:"enable_cache"`
	CacheTTLMinutes      int      `json:"cache_ttl_minutes"`
	MaxConcurrentScans   int      `json:"max_concurrent_scans"`
	ScannerConcurrency   int      `json:"scanner_concurrency"`
	DownloadChunkSize    int      `json:"download_chunk_size"`
	MaxArchiveSize int64  `json:"max_archive_size"`
	TempDir        string `json:"temp_dir"`
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level      string `json:"level"`
	Format     string `json:"format"`
	Output     string `json:"output"`
	MaxSize    int    `json:"max_size"`
	MaxBackups int    `json:"max_backups"`
	MaxAge     int    `json:"max_age"`
	Compress   bool   `json:"compress"`
}

// StorageConfig contains storage configuration.
type StorageConfig struct {
	Type      string `json:"type"`      // "local" or "s3"
	Endpoint  string `json:"endpoint"`  // e.g. "minio:9000"
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
	Bucket    string `json:"bucket"`    // e.g. "catalogizer-covers"
	UseSSL    bool   `json:"use_ssl"`
	Region    string `json:"region"`
}

// ProxyConfig contains HTTP/SOCKS proxy configuration for external API access.
type ProxyConfig struct {
	Enabled  bool   `json:"enabled"`
	URL      string `json:"url"`      // e.g. "socks5://nezha.local:1080"
	HTTPURL  string `json:"http_url"` // e.g. "http://nezha.local:3128"
	Username string `json:"username"`
	Password string `json:"password"`
}

func (p ProxyConfig) IsEnabled() bool   { return p.Enabled }
func (p ProxyConfig) GetURL() string    { return p.URL }
func (p ProxyConfig) GetHTTPURL() string { return p.HTTPURL }
func (p ProxyConfig) GetUsername() string { return p.Username }
func (p ProxyConfig) GetPassword() string { return p.Password }

// LoadConfig loads configuration from file or creates default
func LoadConfig(configPath string) (*Config, error) {
	config := getDefaultConfig()

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config file
		if err := saveConfig(config, configPath); err != nil {
			return nil, fmt.Errorf("failed to create default config: %w", err)
		}
		return config, nil
	}

	// Load existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return config, nil
}

// getDefaultConfig returns default configuration
func getDefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host:         "localhost",
			Port:         28080,
			ReadTimeout:  900,
			WriteTimeout: 900,
			IdleTimeout:  120,
			EnableCORS:   true,
			EnableHTTPS: true, // Enable HTTPS by default for security
		},
		Database: DatabaseConfig{
			Type:               "postgres",
			Host:               "localhost",
			Port:               25432,
			Name:               "catalogizer",
			User:               "catalogizer",
			Password:           "catalogizer_dev",
			SSLMode:            "disable",
			Path:               "./catalog.db",
			MaxOpenConnections: 25,
			MaxIdleConnections: 5,
			ConnMaxLifetime:    300,
			ConnMaxIdleTime:    60,
			EnableWAL:          true,
			CacheSize:          -2000,
			BusyTimeout:        5000,
		},
		Auth: AuthConfig{
			JWTSecret:          "", // Must be set via environment variable
			JWTExpirationHours: 24,
			EnableAuth:         true, // Enable auth by default for security
			AdminUsername:      "",   // Must be set via environment variable
			AdminPassword:      "",   // Must be set via environment variable
		},
		Catalog: CatalogConfig{
			DefaultPageSize:      100,
			MaxPageSize:          1000,
			EnableCache:          true,
			CacheTTLMinutes:      15,
			MaxConcurrentScans:   3,
			ScannerConcurrency:   4,
			DownloadChunkSize:    1024 * 1024,            // 1MB
			MaxArchiveSize:       1024 * 1024 * 1024 * 5, // 5GB
			TempDir: os.TempDir() + "/catalog-api", // Use system temp directory
		},
		Storage: StorageConfig{
			Type:   "local",
			Bucket: "catalogizer-covers",
			Region: "us-east-1",
		},
		Proxy: ProxyConfig{
			Enabled: false,
		},
		Logging: LoggingConfig{
			Level:      "info",
			Format:     "json",
			Output:     "stdout",
			MaxSize:    100,
			MaxBackups: 3,
			MaxAge:     28,
			Compress:   true,
		},
	}
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return fmt.Errorf("invalid server port: %d", config.Server.Port)
	}

	// Apply DATABASE_* env overrides
	if dbType := os.Getenv("DATABASE_TYPE"); dbType != "" {
		config.Database.Type = dbType
	}
	if dbHost := os.Getenv("DATABASE_HOST"); dbHost != "" {
		config.Database.Host = dbHost
	}
	if dbPort := os.Getenv("DATABASE_PORT"); dbPort != "" {
		if p, err := strconv.Atoi(dbPort); err == nil {
			config.Database.Port = p
		}
	}
	if dbName := os.Getenv("DATABASE_NAME"); dbName != "" {
		config.Database.Name = dbName
	} else if dbName := os.Getenv("POSTGRES_DB"); dbName != "" {
		// Fallback: POSTGRES_DB is the env var the postgres container is
		// configured with. Honor it for the DB name when DATABASE_NAME is
		// unset so the API picks up the same database the container uses.
		// DATABASE_NAME always takes precedence when both are set.
		config.Database.Name = dbName
	}
	if dbUser := os.Getenv("DATABASE_USER"); dbUser != "" {
		config.Database.User = dbUser
	}
	if dbPass := os.Getenv("DATABASE_PASSWORD"); dbPass != "" {
		config.Database.Password = dbPass
	}
	if dbSSL := os.Getenv("DATABASE_SSL_MODE"); dbSSL != "" {
		config.Database.SSLMode = dbSSL
	}

	// Apply STORAGE_* env overrides
	if storageType := os.Getenv("STORAGE_TYPE"); storageType != "" {
		config.Storage.Type = storageType
	}
	if storageEndpoint := os.Getenv("STORAGE_ENDPOINT"); storageEndpoint != "" {
		config.Storage.Endpoint = storageEndpoint
	}
	if storageAccessKey := os.Getenv("STORAGE_ACCESS_KEY"); storageAccessKey != "" {
		config.Storage.AccessKey = storageAccessKey
	}
	if storageSecretKey := os.Getenv("STORAGE_SECRET_KEY"); storageSecretKey != "" {
		config.Storage.SecretKey = storageSecretKey
	}
	if storageBucket := os.Getenv("STORAGE_BUCKET"); storageBucket != "" {
		config.Storage.Bucket = storageBucket
	}
	if storageUseSSL := os.Getenv("STORAGE_USE_SSL"); storageUseSSL != "" {
		config.Storage.UseSSL = storageUseSSL == "true" || storageUseSSL == "1"
	}
	if storageRegion := os.Getenv("STORAGE_REGION"); storageRegion != "" {
		config.Storage.Region = storageRegion
	}

	// Validate database config based on type
	dbType := config.Database.Type
	if dbType == "" {
		dbType = "postgres"
	}
	switch dbType {
	case "postgres":
		if config.Database.Host == "" {
			return fmt.Errorf("database host cannot be empty for postgres")
		}
		if config.Database.Name == "" {
			return fmt.Errorf("database name cannot be empty for postgres")
		}
	case "sqlite":
		if config.Database.Path == "" {
			return fmt.Errorf("database path cannot be empty for sqlite")
		}
	default:
		return fmt.Errorf("unsupported database type: %s", dbType)
	}

	if config.Auth.EnableAuth {
		// Environment variables override config file values (config.json may have ${VAR} placeholders)
		if envJWT := os.Getenv("JWT_SECRET"); envJWT != "" {
			config.Auth.JWTSecret = envJWT
		}
		if envUser := os.Getenv("ADMIN_USERNAME"); envUser != "" {
			config.Auth.AdminUsername = envUser
		}
		if envPass := os.Getenv("ADMIN_PASSWORD"); envPass != "" {
			config.Auth.AdminPassword = envPass
		}

		// Validate required security settings
		if config.Auth.JWTSecret == "" {
			return fmt.Errorf("JWT secret must be set via JWT_SECRET environment variable or config")
		}
		if len(config.Auth.JWTSecret) < 32 {
			return fmt.Errorf("JWT secret must be at least 32 characters long")
		}
		if config.Auth.AdminUsername == "" || config.Auth.AdminPassword == "" {
			return fmt.Errorf("admin credentials must be set via ADMIN_USERNAME and ADMIN_PASSWORD environment variables")
		}
		if isPlaceholderSecret(config.Auth.JWTSecret) {
			return fmt.Errorf("JWT secret is a known placeholder/default value; generate a random secret")
		}
		if isPlaceholderPassword(config.Auth.AdminPassword) {
			return fmt.Errorf("admin password is a known placeholder/default value; set a real ADMIN_PASSWORD")
		}
	}

	if config.Catalog.DefaultPageSize <= 0 {
		return fmt.Errorf("default page size must be positive")
	}

	if config.Catalog.MaxPageSize < config.Catalog.DefaultPageSize {
		return fmt.Errorf("max page size must be >= default page size")
	}

	return nil
}

// saveConfig saves configuration to file
func saveConfig(config *Config, configPath string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetDatabaseURL returns the database connection URL
func (c *Config) GetDatabaseURL() string {
	dbType := c.Database.Type
	if dbType == "" {
		dbType = "postgres"
	}

	if dbType == "postgres" {
		sslMode := c.Database.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}
		return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
			c.Database.User, c.Database.Password,
			c.Database.Host, c.Database.Port,
			c.Database.Name, sslMode)
	}

	// SQLite
	params := "?_busy_timeout=5000&_journal_mode=WAL&_synchronous=NORMAL&_foreign_keys=1"
	if c.Database.EnableWAL {
		params += "&_wal_autocheckpoint=1000"
	}
	if c.Database.CacheSize != 0 {
		params += fmt.Sprintf("&_cache_size=%d", c.Database.CacheSize)
	}
	return c.Database.Path + params
}

// GetServerAddress returns the server address
func (c *Config) GetServerAddress() string {
	return net.JoinHostPort(c.Server.Host, strconv.Itoa(c.Server.Port))
}

// placeholderMarkers are substrings that identify documentation placeholders
// and published defaults (matched case-insensitively). Any secret containing
// one of them was copied from an example file and is therefore public.
var placeholderMarkers = []string{
	"change_me", "change-me", "change-this", "changeme", "your-super-secret",
	"your_secret", "your-secret", "placeholder", "replace-me", "replace_me",
}

// placeholderPasswords are exact (case-insensitive) weak defaults.
var placeholderPasswords = []string{"admin", "password", "admin_password", "catalogizer_password"}

func isPlaceholderSecret(v string) bool {
	l := strings.ToLower(v)
	for _, m := range placeholderMarkers {
		if strings.Contains(l, m) {
			return true
		}
	}
	return false
}

func isPlaceholderPassword(v string) bool {
	if isPlaceholderSecret(v) {
		return true
	}
	l := strings.ToLower(v)
	for _, p := range placeholderPasswords {
		if l == p {
			return true
		}
	}
	return false
}
