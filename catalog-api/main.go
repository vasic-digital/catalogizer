package main

import (
	"catalogizer/internal/logredact"
	"catalogizer/internal/streamticket"
	"errors"
	"catalogizer/challenges"
	root_config "catalogizer/config"
	"catalogizer/database"
	"catalogizer/filesystem"
	root_handlers "catalogizer/handlers"
	"catalogizer/internal/auth"
	internal_config "catalogizer/internal/config"
	"catalogizer/internal/eventbus"
	"catalogizer/internal/firebase"
	"catalogizer/internal/handlers"
	"catalogizer/internal/infra"
	"catalogizer/internal/logging"
	"catalogizer/internal/media/providers"
	"catalogizer/internal/metrics"
	"catalogizer/internal/middleware"
	"catalogizer/internal/modules"
	"catalogizer/internal/pathguard"
	"catalogizer/internal/services"
	root_middleware "catalogizer/middleware"
	"catalogizer/internal/netguard"
	"catalogizer/models"
	root_repository "catalogizer/repository"
	root_services "catalogizer/services"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/proxy"

	"digital.vasic.storage/pkg/object"
	"digital.vasic.storage/pkg/s3"

	"digital.vasic.assets/pkg/defaults"
	"digital.vasic.assets/pkg/event"
	"digital.vasic.assets/pkg/manager"
	"digital.vasic.assets/pkg/resolver"
	asset_store "digital.vasic.assets/pkg/store"
	"digital.vasic.containers/pkg/discovery"
	"digital.vasic.discovery/pkg/broadcast"
	"github.com/gin-gonic/gin"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/quic-go/quic-go/http3"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// Version information injected via ldflags at build time
var (
	Version     = "dev"
	BuildNumber = "0"
	BuildDate   = "unknown"
)

// atoi converts string to int with default fallback
func atoi(s string) int {
	if i, err := strconv.Atoi(s); err == nil {
	return i
	}
	return 8080 // default port
}

// findAvailablePort tries to bind to a port starting from startPort and returns the first available port.
func findAvailablePort(host string, startPort, maxAttempts int) (int, error) {
	discoverer := discovery.NewTCPDiscoverer()
	for i := 0; i < maxAttempts; i++ {
	port := startPort + i
	target := discovery.DiscoveryTarget{
			Name:    "catalog-api",
			Host:    host,
			Port:    strconv.Itoa(port),
			Method:  "tcp",
			Timeout: 100 * time.Millisecond,
	}
	reachable, err := discoverer.Discover(context.Background(), target)
	if err != nil || !reachable {
			// Port is free or unreachable
			return port, nil
	}
	}
	return 0, fmt.Errorf("no available port in range %d-%d", startPort, startPort+maxAttempts-1)
}

// writePortFile writes the bound port to a file for service discovery.
func writePortFile(port int) error {
	portFile := ".service-port"
	data := fmt.Sprintf("%d", port)
	return os.WriteFile(portFile, []byte(data), 0600)
}

// getOutboundIP returns the preferred LAN IP of this machine.
// Prefers 192.168.x.x or 172.x.x.x over 10.x.x.x (VPN) interfaces.
func getOutboundIP() string {
	ifaces, err := net.Interfaces()
	if err == nil {
	for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip == nil || ip.IsLoopback() || ip.To4() == nil {
					continue
				}
				// Skip VPN/tunnel interfaces
				if strings.HasPrefix(iface.Name, "wg") || strings.HasPrefix(iface.Name, "tun") || strings.HasPrefix(iface.Name, "vpn") {
					continue
				}
				return ip.String()
			}
	}
	}
	// Fallback: use outbound connection detection
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
	return "127.0.0.1"
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

// getOrCreateSelfSignedCert loads a cached TLS certificate or generates a new one.
func getOrCreateSelfSignedCert() (tls.Certificate, error) {
	cacheDir := filepath.Join(".", "cache", "tls")
	certPath := filepath.Join(cacheDir, "cert.pem")
	keyPath := filepath.Join(cacheDir, "key.pem")

	// Try loading cached cert. Paths are hardcoded under ./cache/tls — safe.
	// #nosec G304 — controlled internal cache directory, not user input.
	if certPEM, err := os.ReadFile(certPath); err == nil {
	// #nosec G304 — controlled internal cache directory, not user input.
	if keyPEM, err := os.ReadFile(keyPath); err == nil {
			cert, err := tls.X509KeyPair(certPEM, keyPEM)
			if err == nil {
				// Verify cert is not expired
				if len(cert.Certificate) > 0 {
					x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
					if err == nil && time.Now().Before(x509Cert.NotAfter) {
						return cert, nil
					}
				}
			}
	}
	}

	// Generate new cert
	cert, certPEM, keyPEM, err := generateSelfSignedCert()
	if err != nil {
	return tls.Certificate{}, err
	}

	// Cache to disk
	if err := os.MkdirAll(cacheDir, 0700); err != nil {
	return cert, fmt.Errorf("failed to create cert cache directory: %w", err)
	}
	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
	return cert, fmt.Errorf("failed to write certificate file: %w", err)
	}
	if err := os.WriteFile(keyPath, keyPEM, 0600); err != nil {
	return cert, fmt.Errorf("failed to write key file: %w", err)
	}

	return cert, nil
}

// generateSelfSignedCert creates a self-signed TLS certificate for development.
func generateSelfSignedCert() (tls.Certificate, []byte, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
	return tls.Certificate{}, nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	template := x509.Certificate{
	SerialNumber: big.NewInt(1),
	Subject: pkix.Name{
			Organization: []string{"Catalogizer Development"},
	},
	NotBefore:             time.Now(),
	NotAfter:              time.Now().Add(365 * 24 * time.Hour),
	KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	BasicConstraintsValid: true,
	IPAddresses:           []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
	DNSNames:              []string{"localhost"},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
	return tls.Certificate{}, nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	certPEM := pem.EncodeToMemory(&pem.Block{
	Type:  "CERTIFICATE",
	Bytes: derBytes,
	})
	keyPEM := pem.EncodeToMemory(&pem.Block{
	Type:  "RSA PRIVATE KEY",
	Bytes: x509.MarshalPKCS1PrivateKey(priv),
	})

	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
	return tls.Certificate{}, nil, nil, fmt.Errorf("failed to load key pair: %w", err)
	}

	return cert, certPEM, keyPEM, nil
}

// @title Catalog API
// @version 2.0
// @description REST API for browsing and searching multi-protocol file catalog (SMB, FTP, NFS, WebDAV, Local)
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	if err := filesystem.RegisterS3Protocol(); err != nil {
		fmt.Fprintf(os.Stderr, "register s3 storage protocol: %v\n", err)
		os.Exit(1)
	}
	startTime := time.Now()

	// Parse command line flags
	testMode := flag.Bool("test-mode", false, "Run in test mode with additional logging")
	flag.Parse()

	// Initialize structured logger
	env := os.Getenv("APP_ENV")
	if env == "" {
	env = "production"
	}
	if *testMode {
	env = "development"
	}
	if err := logging.Init(env); err != nil {
	fmt.Fprintf(os.Stderr, "Failed to initialize logger: %v\n", err)
	os.Exit(1)
	}
	defer func() {
	if err := logging.Sync(); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to sync logger: %v\n", err)
	}
	}()
	logger := logging.Logger

	if *testMode {
	logging.Info("Running in test mode")
	}

	// Register all external Go modules (digital.vasic.* family)
	moduleRegistry := modules.RegisterModules()
	defer moduleRegistry.Stop()

	// Initialize Firebase Admin SDK for Analytics + Crashlytics.
	// When GOOGLE_APPLICATION_CREDENTIALS is unset the app runs
	// in disabled mode — safe no-ops, no crash, no panic.
	firebaseApp := firebase.New(firebase.Config{Logger: logger})
	logging.Info(firebaseApp.Summary())

	// Log a startup event via Firebase Analytics (best-effort, no-op when disabled).
	firebaseApp.LogEvent(context.Background(), firebase.EventStartup, map[string]string{
		"version": Version,
		"build":   BuildNumber,
	})

	// Load configuration
	cfg, err := root_config.LoadConfig("config.json")
	if err != nil {
	logging.Fatal("Failed to load configuration", logging.ErrorField(err))
	}

	// Override sensitive config with environment variables (security best practice)
	if jwtSecret := os.Getenv("JWT_SECRET"); jwtSecret != "" {
	cfg.Auth.JWTSecret = jwtSecret
	}
	if adminUser := os.Getenv("ADMIN_USERNAME"); adminUser != "" {
	cfg.Auth.AdminUsername = adminUser
	}
	if adminPass := os.Getenv("ADMIN_PASSWORD"); adminPass != "" {
	cfg.Auth.AdminPassword = adminPass
	}
	if port := os.Getenv("SERVER_PORT"); port != "" {
	cfg.Server.Port = atoi(port) // Use helper function
	}
	if host := os.Getenv("HOST"); host != "" {
	cfg.Server.Host = host // Allow overriding bind address (e.g., 0.0.0.0 for containers)
	}
	if ginMode := os.Getenv("GIN_MODE"); ginMode != "" {
	gin.SetMode(ginMode)
	}

	// Apply DATABASE_* env overrides before creating connection
	if dbType := os.Getenv("DATABASE_TYPE"); dbType != "" {
	cfg.Database.Type = dbType
	}
	if dbHost := os.Getenv("DATABASE_HOST"); dbHost != "" {
	cfg.Database.Host = dbHost
	}
	if dbPort := os.Getenv("DATABASE_PORT"); dbPort != "" {
	cfg.Database.Port = atoi(dbPort)
	}
	if dbName := os.Getenv("DATABASE_NAME"); dbName != "" {
	cfg.Database.Name = dbName
	}
	if dbUser := os.Getenv("DATABASE_USER"); dbUser != "" {
	cfg.Database.User = dbUser
	}
	if dbPass := os.Getenv("DATABASE_PASSWORD"); dbPass != "" {
	cfg.Database.Password = dbPass
	}
	if dbSSL := os.Getenv("DATABASE_SSL_MODE"); dbSSL != "" {
	cfg.Database.SSLMode = dbSSL
	}

	if keyFile := os.Getenv("DATABASE_ENCRYPTION_KEY_FILE"); keyFile != "" {
	cfg.Database.EncryptionKeyFile = keyFile
	}

	// Default SQLite path if not set
	if cfg.Database.Path == "" {
	cfg.Database.Path = "./data/catalogizer.db"
	}

	// CAT-001 offline maintenance commands (server must be stopped):
	//   catalog-api db-rekey            rotate the SQLCipher key
	//   catalog-api db-restore <file>   restore an encrypted backup
	if flag.NArg() > 0 && (flag.Arg(0) == "db-rekey" || flag.Arg(0) == "db-restore") {
		os.Exit(runDBMaintenance(&cfg.Database, flag.Args()))
	}
	// Offline recovery (CAT-021): validate the replacement password before any
	// database work; the rotation itself runs after the DB is connected.
	if flag.NArg() > 0 && flag.Arg(0) == "admin-reset-password" {
	if _, err := newAdminPasswordFromEnv(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	}
	// Default SSLMode
	if cfg.Database.SSLMode == "" {
	cfg.Database.SSLMode = "disable"
	}

	// §11.4.76: provision the distributed runtime infra (postgres/redis/minio)
	// via the containers submodule BEFORE connecting to the database. Off by
	// default (INFRA_PROVISION_ENABLED unset/false) so existing behaviour is
	// unchanged; when enabled it is idempotent — already-running infra is
	// TCP-discovered and skipped, only a genuinely-down stack is provisioned.
	// Fail-soft by default: log and continue (the DB connect below then fails
	// loudly if the infra is truly unavailable); fail-fast when
	// INFRA_PROVISION_REQUIRED=true.
	if err := infra.Provision(context.Background()); err != nil {
		required, _ := strconv.ParseBool(os.Getenv("INFRA_PROVISION_REQUIRED"))
		if required {
			logging.Fatal("infra provisioning failed (INFRA_PROVISION_REQUIRED=true)", logging.ErrorField(err))
		}
		logging.Errorf("infra provisioning failed (continuing; set INFRA_PROVISION_REQUIRED=true to make fatal): %v", err)
	}

	// Initialize single database connection
	databaseDB, err := database.NewConnection(&cfg.Database)
	if err != nil {
	logging.Fatal("Failed to initialize database", logging.ErrorField(err))
	}
	logging.Infof("Database connected: %s", databaseDB.DatabaseType())

	// Start module background services (memory monitor, health checks)
	ctx := context.Background()
	moduleRegistry.StartBackgroundServices(ctx)

	// Run database migrations
	// The database lives in the default write root ./data; request-named sync/conversion
	// paths may never be it, a SQLite companion of it, its key file, or a directory holding
	// one of them (review round 3, N2).
	pathguard.ProtectDatabase(cfg.Database.Path)
	if keyFile, kerr := database.ResolveKeyFilePath(&cfg.Database); kerr == nil {
		pathguard.Protect(keyFile)
	} else {
		// Not fatal: with no key location there is no key file to protect; the database layer
		// reports a missing key itself when encryption needs one.
		logging.Warn("Database key file location unknown; it is not added to the protected paths",
			logging.ErrorField(kerr))
	}
	logging.Infof("Filesystem path confinement roots: %s", pathguard.Describe())
	logging.Info("Running database migrations...")
	if err := databaseDB.RunMigrations(ctx); err != nil {
	logging.Fatal("Failed to run database migrations", logging.ErrorField(err))
	}
	logging.Info("Database migrations completed successfully")

	// Seed default admin user if none exists
	if flag.NArg() > 0 && flag.Arg(0) == "admin-reset-password" {
	os.Exit(runAdminReset(databaseDB, flag.Args(), os.Stdout, os.Stderr))
	}
	handleSeedAdminError(seedDefaultAdmin(databaseDB, cfg.Auth.AdminUsername, cfg.Auth.AdminPassword),
	func(m string, e error) { logging.Fatal(m, logging.ErrorField(e)) },
	func(m string, e error) { logging.Warnf("%s: %v", m, e) })
	if _, err := reportLegacyPasswordHashes(databaseDB, logging.Warnf); err != nil {
		logging.Warnf("legacy password hash count unavailable: %v", err)
	}

	// Initialize services
	// Convert config to internal format
	// Load SMB hosts from storage_roots table (authoritative source of SMB credentials)
	var smbHosts []internal_config.SMBHost
	smbRows, smbErr := databaseDB.Query(
	`SELECT name, host, port, path, username, password, domain FROM storage_roots WHERE protocol = 'smb' AND enabled = 1`,
	)
	if smbErr == nil {
	for smbRows.Next() {
			var name string
			var host, share, username, password, domain *string
			var port *int
			if err := smbRows.Scan(&name, &host, &port, &share, &username, &password, &domain); err != nil {
				logging.Warnf("Failed to scan SMB storage root: %v", err)
				continue
			}
			h := internal_config.SMBHost{Name: name, Port: 445}
			if host != nil {
				h.Host = *host
			}
			if port != nil {
				h.Port = *port
			}
			if share != nil {
				h.Share = *share
			}
			if username != nil {
				h.Username = *username
			}
			if password != nil {
				h.Password = *password
			}
			if domain != nil {
				h.Domain = *domain
			}
			smbHosts = append(smbHosts, h)
			logging.Infof("Loaded SMB host from DB: %s (%s:%d/%s)", h.Name, h.Host, h.Port, h.Share)
	}
	if err := smbRows.Close(); err != nil {
			logging.Warnf("Failed to close SMB rows: %v", err)
	}
	} else {
	logging.Warnf("Failed to query SMB storage roots: %v", smbErr)
	}

	internalCfg := &internal_config.Config{
	Server: internal_config.ServerConfig{
			Host:         cfg.Server.Host,
			Port:         fmt.Sprintf("%d", cfg.Server.Port),
			ReadTimeout:  cfg.Server.ReadTimeout,
			WriteTimeout: cfg.Server.WriteTimeout,
			IdleTimeout:  cfg.Server.IdleTimeout,
			EnableCORS:   cfg.Server.EnableCORS,
			EnableHTTPS:  cfg.Server.EnableHTTPS,
	},
	Database: internal_config.DatabaseConfig{
			Database: cfg.Database.Path,
	},
	SMB: internal_config.SMBConfig{
			Hosts:     smbHosts,
			Timeout:   30,
			ChunkSize: cfg.Catalog.DownloadChunkSize,
	},
	Catalog: internal_config.CatalogConfig{
			TempDir:           cfg.Catalog.TempDir,
			MaxArchiveSize:    cfg.Catalog.MaxArchiveSize,
			DownloadChunkSize: cfg.Catalog.DownloadChunkSize,
	},
	}

	catalogService := services.NewCatalogService(internalCfg, logger)
	catalogService.SetDB(databaseDB)
	smbService := services.NewSMBService(internalCfg, logger)
	smbDiscoveryService := services.NewSMBDiscoveryServiceFromConfig(internalCfg, logger)
	bindingIngester := services.NewBindingIngester(databaseDB, logger)

	// fileRepository is shared by stats, browse, and recommendation handlers
	fileRepository := root_repository.NewFileRepository(databaseDB)

	// Lazy initialization: RecommendationService and its unique dependencies
	// (mediaRecognitionService, duplicateDetectionService) are only needed when
	// recommendation endpoints are hit. Deferred via sync.Once to reduce startup cost.
	var recommendationHandlerOnce sync.Once
	var lazyRecommendationHandler *root_handlers.RecommendationHandler
	getRecommendationHandler := func() *root_handlers.RecommendationHandler {
	recommendationHandlerOnce.Do(func() {
			mediaRecognitionService := services.NewMediaRecognitionService(databaseDB, logger, nil, nil, "", "", "", "", "", "")
			duplicateDetectionService := services.NewDuplicateDetectionService(databaseDB, logger, nil)
			recommendationService := services.NewRecommendationService(
				mediaRecognitionService,
				duplicateDetectionService,
				fileRepository,
				databaseDB,
			)
			lazyRecommendationHandler = root_handlers.NewRecommendationHandler(recommendationService)
	})
	return lazyRecommendationHandler
	}

	// Initialize repositories (eager — used by core services or multiple handlers)
	userRepo := root_repository.NewUserRepository(databaseDB)
	analyticsRepo := root_repository.NewAnalyticsRepository(databaseDB)
	configurationRepo := root_repository.NewConfigurationRepository(databaseDB)
	errorReportingRepo := root_repository.NewErrorReportingRepository(databaseDB)
	crashReportingRepo := root_repository.NewCrashReportingRepository(databaseDB)
	logManagementRepo := root_repository.NewLogManagementRepository(databaseDB)
	favoritesRepo := root_repository.NewFavoritesRepository(databaseDB)

	// Initialize authentication services (eager — core dependency for many handlers)
	jwtSecret := cfg.Auth.JWTSecret
	if jwtSecret == "" {
	// Generate a cryptographically secure random secret at startup
	secretBytes := make([]byte, 32)
	if _, err := rand.Read(secretBytes); err != nil {
			logging.Fatal("Failed to generate JWT secret", logging.ErrorField(err))
	}
	jwtSecret = hex.EncodeToString(secretBytes)
	logging.Warn("No JWT secret configured. Generated ephemeral secret. Set Auth.JWTSecret in config for persistent sessions across restarts.")
	}
	authService := root_services.NewAuthServiceWithAudit(userRepo, jwtSecret, databaseDB)
	analyticsService := root_services.NewAnalyticsService(analyticsRepo)
	reportingService := root_services.NewReportingService(analyticsRepo, userRepo, databaseDB)
	configurationService := root_services.NewConfigurationService(configurationRepo, "./config.json")
	errorReportingService := root_services.NewErrorReportingService(errorReportingRepo, crashReportingRepo)
	logManagementService := root_services.NewLogManagementService(logManagementRepo)
	favoritesService := root_services.NewFavoritesService(favoritesRepo, authService)

	// ConversionService is eager: the background worker must run from startup so queued jobs are
	// processed even before any conversion endpoint is hit. Only the HTTP handler is lazy.
	conversionRepo := root_repository.NewConversionRepository(databaseDB)
	conversionService := root_services.NewConversionService(conversionRepo, userRepo, authService)
	conversionWorkerCtx, conversionWorkerCancel := context.WithCancel(context.Background())
	conversionWorkerDone := make(chan struct{})
	go func() {
	defer close(conversionWorkerDone)
	conversionService.RunWorker(conversionWorkerCtx, 2*time.Second)
	}()
	var conversionHandlerOnce sync.Once
	var lazyConversionHandler *root_handlers.ConversionHandler
	getConversionHandler := func() *root_handlers.ConversionHandler {
	conversionHandlerOnce.Do(func() {
			lazyConversionHandler = root_handlers.NewConversionHandler(conversionService, authService)
	})
	return lazyConversionHandler
	}

	// Lazy initialization: PlaylistService — only needed when playlist endpoints are hit
	var playlistHandlerOnce sync.Once
	var lazyPlaylistHandler *root_handlers.PlaylistHandler
	getPlaylistHandler := func() *root_handlers.PlaylistHandler {
	playlistHandlerOnce.Do(func() {
			playlistRepo := root_repository.NewPlaylistRepository(databaseDB)
			playlistService := root_services.NewPlaylistService(playlistRepo)
			lazyPlaylistHandler = root_handlers.NewPlaylistHandler(playlistService, logger)
	})
	return lazyPlaylistHandler
	}

	// Initialize internal auth service and middleware for rate limiting
	internalAuthService := auth.NewAuthService(databaseDB, jwtSecret, logger)
	authMiddleware := auth.NewAuthMiddleware(internalAuthService, logger)

	// Initialize Redis client for distributed rate limiting
	redisClient := redis.NewClient(&redis.Options{
	Addr:         os.Getenv("REDIS_ADDR"),
	Password:     os.Getenv("REDIS_PASSWORD"),
	DB:           0,
	PoolSize:     10,
	MinIdleConns: 3,
	MaxRetries:   3,
	DialTimeout:  5 * time.Second,
	ReadTimeout:  3 * time.Second,
	WriteTimeout: 3 * time.Second,
	PoolTimeout:  4 * time.Second,
	// N2: without this the 500 ms context in the rate limiter does not bound a
	// black-holed call (ReadTimeout 3s governs); verified by
	// TestN2_ContextTimeoutEnabledBoundsBlackHoleCall.
	ContextTimeoutEnabled: true,
	})

	// Test Redis connection
	if _, err := redisClient.Ping(context.Background()).Result(); err != nil {
	logging.Warnf("Redis connection failed (%v), falling back to in-memory rate limiting", err)
	redisClient = nil
	} else {
	logging.Info("Redis connected successfully for distributed rate limiting")
	}

	// Initialize challenge service
	challengeService := root_services.NewChallengeService(
	filepath.Join(".", "data", "challenge_results"),
	)
	challenges.RegisterAll(challengeService)

	// Initialize media entity repositories
	mediaItemRepo := root_repository.NewMediaItemRepository(databaseDB)
	mediaFileRepo := root_repository.NewMediaFileRepository(databaseDB)
	extMetaRepo := root_repository.NewExternalMetadataRepository(databaseDB)
	userMetaRepo := root_repository.NewUserMetadataRepository(databaseDB)
	dirAnalysisRepo := root_repository.NewDirectoryAnalysisRepository(databaseDB)
	mediaCollectionRepo := root_repository.NewMediaCollectionRepository(databaseDB)

	// Initialize universal scanner for file system scanning
	clientFactory := filesystem.NewDefaultClientFactory()
	scannerConcurrency := cfg.Catalog.ScannerConcurrency
	if scannerConcurrency <= 0 {
	scannerConcurrency = 4 // default
	}
	universalScanner := services.NewUniversalScanner(databaseDB, logger, nil, clientFactory, scannerConcurrency)
	if err := universalScanner.Start(); err != nil {
	logging.Fatalf("Failed to start universal scanner: %v", err)
	}
	defer universalScanner.Stop()

	// Initialize aggregation service and hook into scanner
	aggregationService := services.NewAggregationService(databaseDB, logger, mediaItemRepo, mediaFileRepo, dirAnalysisRepo, extMetaRepo)
	universalScanner.SetAggregationService(aggregationService)

	// Document text indexing after each scan (D4): pdf/txt/md text of local roots becomes searchable
	docTextIndexer := root_services.NewDocumentTextIndexer(databaseDB)
	universalScanner.SetPostScanIndexer(func(ctx context.Context, rootID int64) error {
		indexed, failed, err := docTextIndexer.IndexStorageRoot(ctx, rootID)
		if err == nil {
			logger.Info("Document text indexed", zap.Int64("storage_root_id", rootID), zap.Int("indexed", indexed), zap.Int("failed", failed))
		}
		return err
	})

	// System event bus for decoupled communication between components.
	// Publishers (scanner) and subscribers (WebSocket bridge) are independent.
	sysEventBusCfg := eventbus.DefaultConfig()
	sysEventBus := eventbus.New(sysEventBusCfg)
	defer sysEventBus.Close()
	universalScanner.SetEventBus(sysEventBus)

	// CacheService is eager — used during shutdown and potentially by other services
	cacheService := services.NewCacheService(databaseDB, logger)

	// Lazy initialization: SubtitleService — only needed when subtitle endpoints are hit
	var subtitleHandlerOnce sync.Once
	var lazySubtitleHandler *root_handlers.SubtitleHandler
	getSubtitleHandler := func() *root_handlers.SubtitleHandler {
	subtitleHandlerOnce.Do(func() {
			subtitleService := services.NewSubtitleService(databaseDB, logger, cacheService)
			lazySubtitleHandler = root_handlers.NewSubtitleHandler(subtitleService, logger).WithAdminChecker(userRepo.IsAdmin)
	})
	return lazySubtitleHandler
	}

	// Initialize handlers (eager — needed at startup or used by core routes)
	catalogHandler := handlers.NewCatalogHandler(catalogService, smbService, logger)
	downloadHandler := handlers.NewDownloadHandler(catalogService, smbService, cfg.Catalog.TempDir, cfg.Catalog.MaxArchiveSize, cfg.Catalog.DownloadChunkSize, logger)
	streamHandler := handlers.NewStreamHandler(catalogService, databaseDB, clientFactory, logger)
	downloadHandler.SetNonSMBFileHandler(streamHandler.StreamFile)
	copyHandler := handlers.NewCopyHandler(catalogService, smbService, cfg.Catalog.TempDir, logger)
	smbDiscoveryHandler := handlers.NewSMBDiscoveryHandler(smbDiscoveryService, logger)
	probeAndIngestHandler := handlers.NewProbeAndIngestHandler(smbDiscoveryService, bindingIngester, logger)
	authHandler := root_handlers.NewAuthHandler(authService)
	androidTVMediaHandler := root_handlers.NewAndroidTVMediaHandler(databaseDB)

	// Collection handler
	collectionHandler := root_handlers.NewCollectionHandler(mediaCollectionRepo).WithAdminChecker(userRepo.IsAdmin)

	// Challenge handler
	challengeHandler := root_handlers.NewChallengeHandler(challengeService)

	// Stats handler
	statsRepo := root_repository.NewStatsRepository(databaseDB)
	statsHandler := root_handlers.NewStatsHandler(fileRepository, statsRepo)

	// Media browse handler (wires /media/search and /media/stats to the database)
	mediaBrowseHandler := root_handlers.NewMediaBrowseHandler(fileRepository, statsRepo, databaseDB)

	// WebSocket handler for real-time updates
	wsHandler := root_handlers.NewWebSocketHandler(logger)

	// EventBus bridge forwards system events to WebSocket clients.
	eventBusBridge := handlers.NewEventBusBridge(sysEventBus, wsHandler, logger)
	eventBusBridge.Start()
	defer eventBusBridge.Stop()

	// Initialize asset management system
	assetRepo := root_repository.NewAssetRepository(databaseDB)
	assetStore, err := asset_store.NewFileStore(filepath.Join(".", "cache", "assets"))
	if err != nil {
	logging.Warnf("Failed to create asset store: %v", err)
	}
	assetEventBus := event.NewInMemoryBus()

	// Image quality gate + last-resort LLM fallback.
	imageQualityRepo := root_repository.NewImageQualityRepository(databaseDB)
	gate := func(inner resolver.Resolver, source string) resolver.Resolver {
	return services.NewQualityGate(inner,
			services.WithQualityRepository(imageQualityRepo),
			services.WithSourceLabel(source),
	)
	}
	assetResolver := resolver.NewChain(
	gate(services.NewCachedFileResolver(filepath.Join(".", "cache", "cover_art"), 1), "cache"),
	gate(services.NewExternalMetadataResolver(databaseDB, 2), "external_metadata"),
	gate(services.NewLocalScanResolver(4), "local_scan"),
	gate(services.NewFanartTVResolver(11), "fanart"),
	gate(services.NewCoverArtArchiveResolver(21), "cover_art_archive"),
	gate(services.NewIGDBResolver(22), "igdb"),
	gate(services.NewLLMImageSearchResolver(90), "llm_image_search"),
	)
	assetManager := manager.New(
	manager.WithStore(assetStore),
	manager.WithResolver(assetResolver),
	manager.WithEventBus(assetEventBus),
	manager.WithDefaults(defaults.NewEmbeddedProvider()),
	manager.WithWorkers(4),
	)
	defer assetManager.Stop()

	// Background image-quality revalidator: touches stale image_quality_assessments
	// rows so the cover pipeline re-resolves them on next access.
	qualityRevalidator := services.NewQualityRevalidator(imageQualityRepo, logger)
	qualityRevalidator.Start(context.Background())
	defer qualityRevalidator.Stop()
	assetHandler := root_handlers.NewAssetHandler(assetManager, assetRepo)

	// Bridge asset events to WebSocket clients
	assetEventBus.Subscribe(func(evt event.Event) {
	if evt.Type == event.AssetReady || evt.Type == event.AssetFailed {
			wsHandler.BroadcastToClients(map[string]interface{}{
				"type":        "asset_update",
				"action":      string(evt.Type),
				"asset_id":    string(evt.AssetID),
				"asset_type":  string(evt.AssetType),
				"entity_type": evt.Metadata["entity_type"],
				"entity_id":   evt.Metadata["entity_id"],
			})
	}
	})

	// Initialize S3-compatible object storage if configured
	var storageClient object.ObjectStore
	if cfg.Storage.Type == "s3" && cfg.Storage.Endpoint != "" {
	s3Cfg := &s3.Config{
			Endpoint:  cfg.Storage.Endpoint,
			AccessKey: cfg.Storage.AccessKey,
			SecretKey: cfg.Storage.SecretKey,
			UseSSL:    cfg.Storage.UseSSL,
			Region:    cfg.Storage.Region,
	}
	s3Client, err := s3.NewClient(s3Cfg, nil)
	if err != nil {
			logging.With(logging.ErrorField(err)).Warn("Failed to create S3 storage client, falling back to local filesystem")
	} else {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			if err := s3Client.Connect(ctx); err != nil {
				logging.With(logging.ErrorField(err)).Warn("Failed to connect to S3 storage, falling back to local filesystem")
			} else {
				// Ensure bucket exists
				exists, err := s3Client.BucketExists(ctx, cfg.Storage.Bucket)
				if err != nil {
					logging.With(logging.ErrorField(err)).Warn("Failed to check S3 bucket, falling back to local filesystem")
				} else if !exists {
					if err := s3Client.CreateBucket(ctx, object.BucketConfig{Name: cfg.Storage.Bucket}); err != nil {
						logging.With(logging.ErrorField(err)).Warn("Failed to create S3 bucket, falling back to local filesystem")
					} else {
						storageClient = s3Client
						logging.With(logging.String("bucket", cfg.Storage.Bucket), logging.String("endpoint", cfg.Storage.Endpoint)).Info("S3 storage connected")
					}
				} else {
					storageClient = s3Client
					logging.With(logging.String("bucket", cfg.Storage.Bucket), logging.String("endpoint", cfg.Storage.Endpoint)).Info("S3 storage connected")
				}
			}
	}
	}

	// Cover art service for universal cover images
	coverArtService := services.NewCoverArtService(databaseDB, logger)
	coverArtService.SetProxyConfig(cfg.Proxy)
	if storageClient != nil {
	coverArtService.SetObjectStore(storageClient, cfg.Storage.Bucket)
	}
	coverArtService.SetClientFactory(clientFactory)

	// Cover handler for placeholder SVGs and cover image serving
	coverHandler := root_handlers.NewCoverHandler(coverArtService).WithQualityRepository(imageQualityRepo)

	// Comic-pages handler: pages through .cbz/.cbr archives for the
	// reader clients. Resolves the entity's archive file via the same
	// MediaFileRepository StreamEntity uses.
	comicPagesHandler := root_handlers.NewComicPagesHandler(mediaFileRepo, databaseDB, clientFactory)

	// PDF-pages handler: renders PDF book pages on demand for the reader
	// clients (e.g. the Android TV book reader). Resolves the entity's PDF
	// file over its storage protocol via the same MediaFileRepository +
	// ClientFactory the comic handler uses, then rasterises one page at a
	// time with poppler (isolated pdfinfo/pdftoppm child processes).
	pdfPagesHandler := root_handlers.NewPdfPagesHandler(mediaFileRepo, databaseDB, clientFactory)

	// Media entity handler for structured media browsing
	mediaEntityHandler := root_handlers.NewMediaEntityHandler(mediaItemRepo, mediaFileRepo, extMetaRepo, userMetaRepo, databaseDB)
	mediaEntityHandler.SetCoverArtService(coverArtService)
	mediaEntityHandler.SetProxyConfig(cfg.Proxy)
	llmProvider := providers.NewLLMProvider(buildProxyHTTPClient(cfg.Proxy), logger)
	mediaEntityHandler.SetLLMProvider(llmProvider)

	// Playback session tracking handler — records every
	// reproduction session (video, audio, book, comic, game)
	// and surfaces the rolled-up per-user progress + full
	// history via /api/v1/playback/sessions/*, /entities/:id/
	// progress, and /entities/:id/history.
	playbackSessionRepo := root_repository.NewPlaybackSessionRepository(databaseDB)
	playbackHandler := root_handlers.NewPlaybackHandler(playbackSessionRepo)

	// Scan handler for storage roots and scan operations
	scanHandler := root_handlers.NewScanHandler(universalScanner, databaseDB)

	// Create service adapters to bridge interface differences between services and handlers
	authAdapter := &root_handlers.AuthServiceAdapter{Inner: authService}
	configAdapter := &root_handlers.ConfigurationServiceAdapter{Inner: configurationService}
	errorAdapter := &root_handlers.ErrorReportingServiceAdapter{Inner: errorReportingService}
	logAdapter := &root_handlers.LogManagementServiceAdapter{Inner: logManagementService}

	// User management, role, configuration, error reporting, and log management handlers
	userHandler := root_handlers.NewUserHandler(userRepo, authAdapter)
	roleHandler := root_handlers.NewRoleHandler(userRepo, authAdapter)
	configurationHandler := root_handlers.NewConfigurationHandler(configAdapter, authAdapter)
	errorReportingHandler := root_handlers.NewErrorReportingHandler(errorAdapter, authAdapter)
	logManagementHandler := root_handlers.NewLogManagementHandler(logAdapter, authAdapter)

	// Admin handler for system info, user management, storage and backup administration
	adminHandler := root_handlers.NewAdminHandler(authAdapter, userRepo, databaseDB, Version)

	// Build service info for discovery announcements
	serviceInfo := broadcast.ServiceInfo{
	Service:      "catalogizer-api",
	Version:      Version,
	Build:        BuildNumber,
	Host:         getOutboundIP(),
	Port:         cfg.Server.Port,
	Protocol:     "http",
	Name:         "Catalogizer API",
	InstanceID:   fmt.Sprintf("catalogizer-%d", time.Now().UnixNano()),
	Capabilities: []string{"catalog", "media", "streaming", "sync", "websocket", "entities"},
	Database:     cfg.Database.Type,
	}

	// Start UDP multicast announcer for LAN discovery. The Discovery
	// library is project-agnostic: catalog-api pins the legacy
	// "catalogizer" wire namespace so existing mobile/web/desktop
	// clients (which expect "catalogizer-announce" envelopes + the
	// "CATALOGIZER_DISCOVER" probe body) keep working unchanged.
	discoveryCfg := broadcast.DefaultConfig()
	discoveryCfg.MessageNamespace = "catalogizer"
	announcer := broadcast.NewAnnouncer(serviceInfo, discoveryCfg)
	if err := announcer.Start(); err != nil {
	logger.Warn("Failed to start discovery announcer", zap.Error(err))
	} else {
	logger.Info("Discovery announcer started", zap.String("multicast", broadcast.DefaultMulticastGroup), zap.Int("port", broadcast.DefaultPort))
	}
	defer announcer.Stop()

	// Start UDP broadcast responder for on-demand discovery.
	// Clients send "CATALOGIZER_DISCOVER" to UDP port 19820 and get
	// service info back — the "catalogizer" namespace preserves the
	// historical wire identity on this project.
	responder := broadcast.NewResponderWithConfig(serviceInfo, broadcast.Config{
	Port:             broadcast.DefaultResponderPort,
	MessageNamespace: "catalogizer",
	})
	if err := responder.Start(); err != nil {
	logger.Warn("Failed to start discovery responder", zap.Error(err))
	} else {
	logger.Info("Discovery responder started", zap.Int("port", broadcast.DefaultResponderPort))
	}
	defer responder.Stop()

	// Search and browse handlers (file-level search and directory browsing)
	searchHandler := root_handlers.NewSearchHandler(fileRepository)
	browseHandler := root_handlers.NewBrowseHandler(fileRepository)

	// Lazy initialization: SyncService — only needed when sync endpoints are hit
	var syncHandlerOnce sync.Once
	var lazySyncHandler *root_handlers.SyncHandler
	getSyncHandler := func() *root_handlers.SyncHandler {
	syncHandlerOnce.Do(func() {
			syncRepo := root_repository.NewSyncRepository(databaseDB)
			syncService := root_services.NewSyncService(syncRepo, userRepo, authService)
			lazySyncHandler = root_handlers.NewSyncHandler(syncService, authService)
	})
	return lazySyncHandler
	}

	// Analytics, reporting, and favorites handlers (eager — lightweight, commonly used)
	analyticsHandler := root_handlers.NewAnalyticsHandler(analyticsService, logger)
	reportingHandler := root_handlers.NewReportingHandler(reportingService, logger)
	favoritesHandler := root_handlers.NewFavoritesHandler(favoritesService, logger)

	// Media query handler — replaces former stub endpoints with real DB implementations
	mediaQueryHandler := root_handlers.NewMediaQueryHandler(databaseDB, mediaItemRepo, userRepo)

	// Initialize JWT middleware
	// Session revocation + live admin check are installed together (main_jwtwire.go).
	jwtMiddleware := newAuthMiddleware(jwtSecret, databaseDB, userRepo)
	ticketSigner := streamticket.New(jwtSecret)
	jwtMiddleware.SetStreamTicketSigner(ticketSigner)
	streamHandler.SetTicketSigner(ticketSigner)

	// Initialize tiered rate limiters using internal auth middleware.
	// Constitution Article V §5.1 category 7 (DDoS/rate-limit) requires
	// that brute-force attempts against login/register be actively
	// rejected, not just logged.
	//
	// Three tiers:
	//   - loginRateLimiter  (30 rpm per IP) : only /login and /register.
	//     Stops brute-force flooders within one minute while leaving
	//     enough headroom for a legitimate user fat-fingering their
	//     password a handful of times + the challenge runner which
	//     re-authenticates ~1 rps during a full bank.
	//   - authRateLimiter   (600 rpm per IP): token ops (/refresh,
	//     /logout, /status, /me). These are cheap reads and the
	//     challenge runner hits them heavily.
	//   - defaultRateLimiter (2000 rpm per IP): everything else.
	loginRateLimiter := authMiddleware.RateLimitByUser(30, "1m")
	authRateLimiter := authMiddleware.RateLimitByUser(600, "1m")
	defaultRateLimiter := authMiddleware.RateLimitByUser(2000, "1m")

	// Redis-based distributed rate limiting: activates automatically when
	// Redis is available AND REDIS_RATE_LIMIT=true is set.
	if redisClient != nil && os.Getenv("REDIS_RATE_LIMIT") == "true" {
	logger.Info("Activating Redis-based distributed rate limiting")
	// Operator decision 2026-11-12: a Redis outage after startup falls back to
	// the in-memory tiers (600/2000 per minute), not 503. /login and /register
	// keep using the in-memory loginRateLimiter.
	// root_middleware.NewRedisTierLimiters wires each tier to ITS OWN in-memory
	// fallback (tested in middleware, CAT-OPA-1).
	authRateLimiter, defaultRateLimiter = root_middleware.NewRedisTierLimiters(redisClient, authRateLimiter, defaultRateLimiter)
	} else if redisClient != nil {
	logger.Info("Redis available but REDIS_RATE_LIMIT not set — using in-memory rate limiting")
	}

	// Setup Gin router
	// gin.Default() equivalent whose access log redacts credential-bearing query parameters (ticket, access_token, token).
	router := gin.New()
	router.Use(gin.LoggerWithFormatter(func(p gin.LogFormatterParams) string {
		return fmt.Sprintf("[GIN] %s | %3d | %13v | %15s | %-7s %#v\n%s",
			p.TimeStamp.Format("2006/01/02 - 15:04:05"), p.StatusCode, p.Latency, p.ClientIP, p.Method, logredact.Path(p.Request.URL.Path, p.Request.URL.RawQuery), p.ErrorMessage)
	}), gin.Recovery())

	// RULE-GO-005 / Master Plan §4.2 — lock down X-Forwarded-For
	// handling. Gin's default TrustedProxies is 0.0.0.0/0, which
	// lets any peer spoof the client IP via X-Forwarded-For and
	// bypass per-IP rate limits. Policy:
	//   - TRUSTED_PROXIES unset:    refuse to trust any proxy
	//     header; c.ClientIP() returns the actual peer IP.
	//   - TRUSTED_PROXIES=auto:     DEPRECATED, client-forgeable
	//     (CAT-TRUSTEDPROXIES-AUTO-1); trusts loopback + RFC-1918;
	//     refused in release mode unless TRUSTED_PROXIES_ALLOW_AUTO=true.
	//   - TRUSTED_PROXIES=<cidr>,...: explicit allow-list.
	warnTrustedProxies(logger, os.Getenv("TRUSTED_PROXIES"))
	if err := applyTrustedProxies(router, os.Getenv("TRUSTED_PROXIES")); err != nil {
		// applyTrustedProxies has already failed closed (no proxy trusted).
		logger.Error("TRUSTED_PROXIES rejected; trusting NO proxy header (every client behind a reverse proxy will share its address)", zap.Error(err))
	}

	// Service discovery (PUBLIC - must respond in < 2 seconds for Android TV)
	// Register BEFORE middleware chain to avoid slow middleware (CORS, metrics, compression)
	// This ensures the /discovery endpoint responds quickly for LAN discovery probes
	router.GET("/discovery", func(c *gin.Context) {
	host := serviceInfo.Host
	port := cfg.Server.Port
	c.JSON(200, gin.H{
			"service":        serviceInfo.Service,
			"name":           serviceInfo.Name,
			"version":        Version,
			"build":          BuildNumber,
			"build_date":     BuildDate,
			"host":           host,
			"port":           port,
			"protocol":       "http",
			"websocket_url":  fmt.Sprintf("ws://%s:%d/ws", host, port),
			"api_base_url":   fmt.Sprintf("http://%s:%d/api/v1", host, port),
			"capabilities":   serviceInfo.Capabilities,
			"database":       cfg.Database.Type,
			"instance_id":    serviceInfo.InstanceID,
			"uptime_seconds": int(time.Since(startTime).Seconds()),
	})
	})

	// Middleware
	router.Use(root_middleware.SecurityHeaders())
	router.Use(root_middleware.ConcurrencyLimiter(100))
	router.Use(root_middleware.RequestTimeout(60 * time.Second))
	router.Use(root_middleware.CORS())
	router.Use(metrics.GinMiddleware())
	router.Use(middleware.Logger(logger))
	router.Use(middleware.ErrorHandler())
	// Crashlytics middleware: captures HTTP 5xx responses as non-fatal
	// errors to Firebase. Safe no-op when Firebase is disabled.
	router.Use(firebase.GinMiddleware(firebaseApp))
	router.Use(root_middleware.RequestID())
	router.Use(root_middleware.InputValidation(root_middleware.DefaultInputValidationConfig()))
	router.Use(middleware.CompressionMiddleware(middleware.DefaultCompressionConfig()))

	// Start runtime metrics collector (goroutines, memory)
	metrics.StartRuntimeCollector(15 * time.Second)

	// Prometheus metrics endpoint
	// Authz round 2: the exposition lists routes, hosts, runtime internals and
	// per-endpoint traffic, so it is administrator-only (a scraper presents an
	// administrator bearer token or the dedicated METRICS_SCRAPE_TOKEN, valid on this route only). /health and /health/deep stay public.
	router.GET("/metrics", metricsScrapeDetect(metricsScrapeToken()), metricsSkipIfScrape(jwtMiddleware.RequireAuth()), metricsSkipIfScrape(jwtMiddleware.RequireAdmin()), gin.WrapH(promhttp.Handler()))

	// pprof endpoints — OPT-IN via HELIX_PPROF_ENABLED=true.
	//
	// DEFER-QA-2026-04-21-002 follow-up: heap/goroutine profiling was
	// previously unavailable, so the 2026-04-20 RunAll memory burst
	// (peak 53.5× baseline heap) could only be assessed from logs. With
	// this flag set, /debug/pprof/{heap,goroutine,profile,trace,…}
	// become available (standard net/http/pprof handlers). The flag
	// defaults OFF so untrusted networks never see the endpoints by
	// accident — operators flip it on, capture profiles, flip it off.
	if os.Getenv("HELIX_PPROF_ENABLED") == "true" {
	pprofGroup := router.Group("/debug/pprof")
	// Authz round 2: heap/goroutine/cmdline output exposes memory contents and
	// process arguments; the flag turns the routes on, the admin gate decides who
	// may read them.
	pprofGroup.Use(jwtMiddleware.RequireAuth(), jwtMiddleware.RequireAdmin(), root_middleware.PprofSamplingGuard(60)) // SECQ-3: sampling window capped at 60 s, one at a time
	pprofGroup.GET("/", gin.WrapF(pprof.Index))
	pprofGroup.GET("/cmdline", gin.WrapF(pprof.Cmdline))
	pprofGroup.GET("/profile", gin.WrapF(pprof.Profile))
	pprofGroup.POST("/symbol", gin.WrapF(pprof.Symbol))
	pprofGroup.GET("/symbol", gin.WrapF(pprof.Symbol))
	pprofGroup.GET("/trace", gin.WrapF(pprof.Trace))
	pprofGroup.GET("/allocs", gin.WrapH(pprof.Handler("allocs")))
	pprofGroup.GET("/block", gin.WrapH(pprof.Handler("block")))
	pprofGroup.GET("/goroutine", gin.WrapH(pprof.Handler("goroutine")))
	pprofGroup.GET("/heap", gin.WrapH(pprof.Handler("heap")))
	pprofGroup.GET("/mutex", gin.WrapH(pprof.Handler("mutex")))
	pprofGroup.GET("/threadcreate", gin.WrapH(pprof.Handler("threadcreate")))
	logging.Info("pprof endpoints enabled at /debug/pprof/*")
	}

	// Health check (short-lived cache to reduce redundant health polling).
	//
	// /api/v1/health is registered as an alias here — several HelixQA
	// test banks + external monitors probe that path. Before the alias,
	// the 2026-04-20 Article VII RunAll log showed 11× 404 on it
	// (bank/monitor noise, not product failures). Keeping both paths
	// unauthenticated matches the canonical /health contract.
	healthHandler := func(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
			"status":       "healthy",
			"time":         time.Now().UTC(),
			"version":      Version,
			"build_number": BuildNumber,
			"build_date":   BuildDate,
	})
	}
	router.GET("/health", root_middleware.CacheHeaders(5), healthHandler)
	router.GET("/api/v1/health", root_middleware.CacheHeaders(5), healthHandler)

	// Deep health check — pings the database with a 100ms timeout so
	// a slow DB never blocks callers for an unreasonable duration.
	deepHealthChecker := metrics.NewHealthChecker(databaseDB, Version)
	router.GET("/health/deep", root_middleware.CacheHeaders(5), func(c *gin.Context) {
	const deepTimeout = 100 * time.Millisecond

	type result struct {
			resp metrics.HealthCheckResponse
	}

	ch := make(chan result, 1)
	go func() {
			ch <- result{resp: deepHealthChecker.Check(c.Request.Context())}
	}()

	select {
	case r := <-ch:
			status := http.StatusOK
			if r.resp.Status == metrics.HealthStatusUnhealthy {
				status = http.StatusServiceUnavailable
			}
			c.JSON(status, r.resp)

	case <-time.After(deepTimeout):
			c.JSON(http.StatusOK, gin.H{
				"status":  "degraded",
				"time":    time.Now().UTC(),
				"version": Version,
				"message": "health check exceeded 100ms timeout",
				"components": gin.H{
					"timeout": gin.H{
						"status":  "degraded",
						"message": "deep health check took too long",
					},
				},
			})
	}
	})

	// WebSocket endpoint: authenticated (Bearer header or ?token=, session-checked) and Origin-checked; see registerWebSocketRoute
	registerWebSocketRoute(router, jwtMiddleware, wsHandler)

	// Image proxy — serves external images (TMDB, etc.) through the API.
	// When the upstream CDN is unreachable, falls back to a type-specific
	// placeholder SVG so client apps still render something useful.
	router.GET("/api/v1/image-proxy", func(c *gin.Context) {
	imageURL := c.Query("url")
	if imageURL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "url parameter required"})
			return
	}
	// Exact parsed-host match, https only (review round 1, F4): a substring test let
	// http://127.0.0.1/internal?x=image.tmdb.org through.
	parsedImage, perr := validateImageProxyURL(imageURL)
	if perr != nil {
			c.JSON(http.StatusForbidden, gin.H{"error": "domain not allowed"})
			return
	}
	imageURL = parsedImage.String()

	// Build a proxy client that can work around DNS hijacking.
	proxyClient := newImageProxyClient(imageURL, cfg.Proxy)
	resp, err := proxyClient.Get(imageURL)
	if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			mediaType := c.DefaultQuery("type", "movie")
			svg := services.GeneratePlaceholderSVG(mediaType)
			c.Header("Content-Type", "image/svg+xml")
			c.Header("Cache-Control", "public, max-age=3600")
			c.Data(http.StatusOK, "image/svg+xml", svg)
			return
	}
	defer resp.Body.Close()
	c.Header("Content-Type", resp.Header.Get("Content-Type"))
	c.Header("Cache-Control", "public, max-age=86400") // Cache 24h
	c.Status(resp.StatusCode)
	if _, copyErr := io.Copy(c.Writer, resp.Body); copyErr != nil {
			logger.Warn("Image proxy stream copy failed", zap.Error(copyErr))
	}
	})

	// Asset serving (public — no auth needed for serving images)
	// StaticCacheHeaders() sets Cache-Control: public, max-age=31536000, immutable
	// for fingerprinted/content-hashed assets. Also suitable for future static file
	// server routes (e.g., router.Static("/static", "./public", root_middleware.StaticCacheHeaders())).
	router.GET("/api/v1/assets/:id", root_middleware.StaticCacheHeaders(), assetHandler.ServeAsset)

	// Cover image serving (public — no auth needed for serving cover images)
	router.GET("/api/v1/cover/placeholder/:type", root_middleware.CacheHeaders(86400), coverHandler.ServePlaceholder)
	router.GET("/api/v1/cover/url/:id", root_middleware.CacheHeaders(300), coverHandler.GetCoverURL)
	router.GET("/api/v1/cover/:id", root_middleware.CacheHeaders(86400), coverHandler.ServeCover)

	// Authentication routes (no auth required)
	authGroup := router.Group("/api/v1/auth")
	{
	// Strict rate limiting for brute-force-prone write operations
	// (30 rpm per IP). Applied ONLY to login/register.
	authGroup.POST("/login", loginRateLimiter, authHandler.LoginGin)
	authGroup.POST("/register", loginRateLimiter, func(c *gin.Context) {
			authHandler.RegisterGin(c, userRepo)
	})
	// Medium rate limiting for token operations (600 rpm)
	authGroup.POST("/refresh", authRateLimiter, authHandler.RefreshTokenGin)
	authGroup.POST("/logout", authRateLimiter, authHandler.LogoutGin)
	authGroup.GET("/me", authRateLimiter, jwtMiddleware.RequireAuth(), authHandler.GetCurrentUserGin)
	authGroup.GET("/status", authRateLimiter, authHandler.GetAuthStatusGin)
	authGroup.GET("/permissions", authRateLimiter, jwtMiddleware.RequireAuth(), authHandler.GetPermissionsGin)
	authGroup.GET("/profile", authRateLimiter, jwtMiddleware.RequireAuth(), authHandler.GetCurrentUserGin)
	authGroup.GET("/init-status", defaultRateLimiter, mediaQueryHandler.GetInitStatus)
	// change-password verifies the current password (bcrypt compare) behind a
	// JWT: failed verifications are limited per user id (CAT-CHPW-RL-1).
	root_middleware.RegisterChangePassword(authGroup, "/change-password", defaultRateLimiter, jwtMiddleware.RequireAuth(), mediaQueryHandler.ChangePassword,
		root_middleware.NewFailedAttemptLimiter(root_middleware.CredentialMaxFailures, root_middleware.CredentialFailureWindow))
	}

	// API routes
	api := router.Group("/api/v1")
	// NOTE: image-proxy is registered ABOVE (outside this group) to be publicly accessible
	// without authentication - needed for Android TV and other clients that need to
	// fetch cover images from TMDB through the API proxy.
	api.Use(jwtMiddleware.RequireAuth()) // Apply auth middleware to all API routes
	api.Use(defaultRateLimiter)          // Apply general rate limiting to API
	{
	// Catalog browsing endpoints
	api.GET("/catalog", catalogHandler.ListRoot)
	api.GET("/catalog/*path", catalogHandler.ListPath)
	api.GET("/catalog-info/*path", catalogHandler.GetFileInfo)

	// Search endpoints
	api.GET("/search", catalogHandler.Search)
	api.GET("/search/duplicates", catalogHandler.SearchDuplicates)
	api.GET("/search/files", searchHandler.SearchFiles)
	api.GET("/search/files/duplicates", searchHandler.SearchDuplicates)
	api.POST("/search/advanced", searchHandler.AdvancedSearch)

	// Download endpoints
	api.GET("/download/file/:id", downloadHandler.DownloadFile)
	api.GET("/download/directory/*path", downloadHandler.DownloadDirectory)
	api.POST("/download/archive", downloadHandler.DownloadArchive)

	// Streaming endpoint — proxies file data from any storage backend (SMB, FTP, NFS, WebDAV, local)
	api.GET("/stream/:id", streamHandler.StreamFile)
	// Short-lived stream ticket for header-less players (web <video>): minted with the normal Authorization header, accepted only
	// by GET/HEAD /stream/:id (middleware.RequireAuth). Replaces the session JWT in the query string for web clients.
	api.POST("/stream/:id/ticket", streamHandler.MintTicket)

	// Storage-operator routes (P2, authz review r3): creating storage roots,
	// listing their directories, queueing scans, SMB discovery/probe/ingest,
	// server-side copy and sync-endpoint mutation act on the host's storage and
	// network, so they are admin-only (RequireAdmin: live DB decision, a stale
	// token claim never grants, a self-registered role-2 user is refused).
	adminGate := jwtMiddleware.RequireAdmin()

	// File operations
	api.POST("/copy/storage", adminGate, copyHandler.CopyToStorage)
	api.POST("/copy/local", adminGate, copyHandler.CopyToLocal)
	api.POST("/copy/upload", adminGate, copyHandler.CopyFromLocal)

	// Media browsing endpoints (must be before :id to prevent route conflict)
	api.GET("/media/search", mediaBrowseHandler.SearchMedia)
	api.GET("/media/stats", mediaBrowseHandler.GetMediaStats)
	api.GET("/media/recent", mediaQueryHandler.GetRecentMedia)
	api.GET("/media/popular", mediaQueryHandler.GetPopularMedia)
	api.GET("/media/by-path", mediaQueryHandler.GetMediaByPath)
	api.POST("/media/analyze", mediaQueryHandler.AnalyzeMedia)

	// Media operations
	api.GET("/media/:id", androidTVMediaHandler.GetMediaByID)
	api.PUT("/media/:id/progress", androidTVMediaHandler.UpdateWatchProgress)
	api.PUT("/media/:id/favorite", androidTVMediaHandler.UpdateFavoriteStatus)
	api.POST("/media/:id/refresh", mediaQueryHandler.RefreshMediaMetadata)
	api.GET("/media/:id/quality", mediaQueryHandler.GetMediaQuality)

	// Image-quality manual revalidation endpoint: operators post an
	// empty body to force the QualityRevalidator to sweep stale
	// rows now, bypassing the 7-day natural tick. Accepts an
	// optional { "stale_age_seconds": N } override to scope the
	// sweep window for the single call.
	// P1 fix (docs/nexus/remaining-work.md): cap the admin
	// revalidate route at 6 requests/minute per client IP so a
	// runaway operator (or an attacker who bypassed auth) cannot
	// stampede the provider chain.
	// Admin-only (review R2 D1/P1): these were reachable by any authenticated user.
	api.POST("/admin/image-quality/revalidate", jwtMiddleware.RequireAdmin(), root_middleware.RateLimiter(6), func(c *gin.Context) {
			var req struct {
				StaleAgeSeconds int `json:"stale_age_seconds"`
				Limit           int `json:"limit"`
			}
			if bindErr := c.ShouldBindJSON(&req); bindErr != nil {
				// Empty or malformed body → use defaults below
			}
			staleAge := 7 * 24 * 3600
			if req.StaleAgeSeconds > 0 {
				staleAge = req.StaleAgeSeconds
			}
			limit := 256
			if req.Limit > 0 && req.Limit <= 4096 {
				limit = req.Limit
			}
			ctx := c.Request.Context()
			cutoff := time.Now().Add(-time.Duration(staleAge) * time.Second)
			rows, err := imageQualityRepo.SampleForRevalidation(ctx, cutoff, limit)
			if err != nil {
				logger.Warn("image quality revalidation sample failed (not returned to client)", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sample images for revalidation"})
				return
			}
			touched := 0
			for _, row := range rows {
				if err := imageQualityRepo.TouchLastChecked(ctx, row.ID); err != nil {
					continue
				}
				touched++
			}
			c.JSON(http.StatusOK, gin.H{
				"sampled":          len(rows),
				"touched":          touched,
				"stale_age_seconds": staleAge,
				"limit":            limit,
			})
	})

	// Cover art batch generation — creates video-frame thumbnails for items
	// that don't have cover art yet.  This is useful when external CDNs are
	// unreachable and the backend must generate its own covers.
	api.POST("/admin/covers/generate-thumbnails", jwtMiddleware.RequireAdmin(), func(c *gin.Context) {
			var req struct {
				Limit int `json:"limit" binding:"min=1,max=500"`
			}
			if err := c.ShouldBindJSON(&req); err != nil {
				// default limit when body is empty
				req.Limit = 50
			}
			generated, err := coverArtService.GenerateMissingVideoThumbnails(c.Request.Context(), req.Limit)
			if err != nil {
				logger.Warn("video thumbnail generation failed (not returned to client)", zap.Error(err))
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate video thumbnails"})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"generated": generated,
				"message":   "thumbnail generation completed",
			})
	})

	// Recommendation endpoints (lazy — handler initialized on first request)
	recGroup := api.Group("/recommendations")
	{
			recGroup.GET("/similar/:media_id", func(c *gin.Context) { getRecommendationHandler().GetSimilarItems(c) })
			recGroup.GET("/trending", func(c *gin.Context) { getRecommendationHandler().GetTrendingItems(c) })
			recGroup.GET("/personalized/:user_id", jwtMiddleware.RequireSelfOrAdmin("user_id"), func(c *gin.Context) { getRecommendationHandler().GetPersonalizedRecommendations(c) })
	}

	// Subtitle endpoints (lazy — handler initialized on first request)
	root_handlers.RegisterSubtitleRoutes(api, getSubtitleHandler, adminGate)
	root_handlers.RegisterStorageRoutes(api, scanHandler, jwtMiddleware)

	// Statistics and sorting
	api.GET("/stats/directories/by-size", catalogHandler.GetDirectoriesBySize)
	api.GET("/stats/duplicates/count", catalogHandler.GetDuplicatesCount)

	// Advanced statistics endpoints
	statsGroup := api.Group("/stats")
	statsGroup.Use(root_middleware.CacheHeaders(60)) // 1-minute cache for statistics
	{
			statsGroup.GET("/overall", statsHandler.GetOverallStats)
			statsGroup.GET("/smb/:smb_root", statsHandler.GetSmbRootStats)
			statsGroup.GET("/filetypes", statsHandler.GetFileTypeStats)
			statsGroup.GET("/sizes", statsHandler.GetSizeDistribution)
			statsGroup.GET("/duplicates", statsHandler.GetDuplicateStats)
			statsGroup.GET("/duplicates/groups", statsHandler.GetTopDuplicateGroups)
			statsGroup.GET("/access", statsHandler.GetAccessPatterns)
			statsGroup.GET("/growth", statsHandler.GetGrowthTrends)
			statsGroup.GET("/scans", statsHandler.GetScanHistory)
	}

	// SMB Discovery endpoints
	smbGroup := api.Group("/smb")
	smbGroup.Use(adminGate) // P2: every /smb/* route is admin-only
	{
			smbGroup.POST("/discover", smbDiscoveryHandler.DiscoverShares)
			smbGroup.GET("/discover", smbDiscoveryHandler.DiscoverSharesGET)
			smbGroup.POST("/test", smbDiscoveryHandler.TestConnection)
			smbGroup.GET("/test", smbDiscoveryHandler.TestConnectionGET)
			smbGroup.POST("/browse", smbDiscoveryHandler.BrowseShare)
			smbGroup.GET("/identities", smbDiscoveryHandler.ListIdentities)
			smbGroup.POST("/probe", smbDiscoveryHandler.ProbeHost)
	smbGroup.POST("/probe-and-ingest", probeAndIngestHandler.ProbeAndIngest)
	}

	// Scan endpoints
	scanGroup := api.Group("/scans")
	{
			root_handlers.RegisterScanRoutes(scanGroup, scanHandler, jwtMiddleware)
	}

	// Conversion endpoints (lazy — handler initialized on first request)
	conversionGroup := api.Group("/conversion")
	{
			conversionGroup.POST("/jobs", func(c *gin.Context) { getConversionHandler().CreateJob(c) })
			conversionGroup.GET("/jobs", func(c *gin.Context) { getConversionHandler().ListJobs(c) })
			conversionGroup.GET("/jobs/:id", func(c *gin.Context) { getConversionHandler().GetJob(c) })
			conversionGroup.POST("/jobs/:id/cancel", func(c *gin.Context) { getConversionHandler().CancelJob(c) })
			conversionGroup.DELETE("/jobs/:id", func(c *gin.Context) { getConversionHandler().DeleteJob(c) })
			conversionGroup.POST("/jobs/:id/retry", func(c *gin.Context) { getConversionHandler().RetryJob(c) })
			conversionGroup.GET("/jobs/:id/download", func(c *gin.Context) { getConversionHandler().DownloadJobFile(c) })
			conversionGroup.GET("/formats", func(c *gin.Context) { getConversionHandler().GetSupportedFormats(c) })
	}

	// Admin endpoints (system info, user management, storage, backups).
	// Closes W2 from docs/nexus/remaining-work.md: double-submit-cookie
	// CSRF guard derived from the JWT secret so mutating admin calls
	// reject cross-origin POST/PUT/DELETE. GET/HEAD/OPTIONS mint a
	// fresh token on the X-CSRF-Token response header + cookie.
	// `wrap` adapts net/http handlers (from vasic-digital/handlers)
	// to gin handlers. Declared before any group that needs it.
	wrap := root_handlers.WrapHTTPHandler

	adminGroup := api.Group("/admin")
	// Article XI §11.5: gate every /admin/* endpoint on the admin
	// role. Prior to 2026-04-29 the admin group had only auth +
	// CSRF — any authenticated user could call /admin/system-info,
	// /admin/users, /admin/storage, /admin/backups. Caught by
	// FQA-API-010 in the real-binary bank verification.
	adminGroup.Use(jwtMiddleware.RequireAdmin())
	if csrfGuard, csrfErr := root_middleware.NewCSRF([]byte(jwtSecret), adminCSRFOptions(os.Getenv, logging.Warnf)...); csrfErr != nil {
			logging.Warnf("CSRF guard disabled on admin group: %v", csrfErr)
	} else {
			adminGroup.Use(csrfGuard.Handler())
	}
	{
			adminGroup.GET("/system-info", adminHandler.GetSystemInfo)
			adminGroup.GET("/users", adminHandler.GetUsers)
			adminGroup.PUT("/users/:id", adminHandler.UpdateUser)
			adminGroup.GET("/storage", adminHandler.GetStorageInfo)
			adminGroup.GET("/backups", adminHandler.GetBackups)
			adminGroup.POST("/backups", adminHandler.CreateBackup)
			adminGroup.POST("/backups/:id/restore", adminHandler.RestoreBackup)
			adminGroup.POST("/storage/scan", adminHandler.ScanStorage)

			// FIX-QA-2026-04-21-003: bank probes /api/v1/admin/{config,
			// errors,health,logs}; each returned 404. The underlying
			// data lives under /configuration, /errors, /logs already —
			// these aliases delegate to the canonical handlers so the
			// admin surface reads as a superset (what the bank expects)
			// without duplicating handler logic.
			adminGroup.GET("/config", wrap(configurationHandler.GetConfiguration))
			adminGroup.GET("/errors", wrap(errorReportingHandler.ListErrorReports))
			adminGroup.GET("/health", wrap(errorReportingHandler.GetSystemHealth))
			adminGroup.GET("/logs", wrap(logManagementHandler.ListLogCollections))
	}

	// User management endpoints
	usersGroup := api.Group("/users")
	{
			usersGroup.POST("", wrap(userHandler.CreateUser))
			usersGroup.GET("", wrap(userHandler.ListUsers))
			usersGroup.GET("/:id", wrap(userHandler.GetUser))
			usersGroup.PUT("/:id", wrap(userHandler.UpdateUser))
			usersGroup.DELETE("/:id", wrap(userHandler.DeleteUser))
			usersGroup.POST("/:id/reset-password", wrap(userHandler.ResetPassword))
			usersGroup.POST("/:id/lock", wrap(userHandler.LockAccount))
			usersGroup.POST("/:id/unlock", wrap(userHandler.UnlockAccount))
	}

	// Role management endpoints
	rolesGroup := api.Group("/roles")
	{
			rolesGroup.POST("", wrap(roleHandler.CreateRole))
			rolesGroup.GET("", wrap(roleHandler.ListRoles))
			rolesGroup.GET("/:id", wrap(roleHandler.GetRole))
			rolesGroup.PUT("/:id", wrap(roleHandler.UpdateRole))
			rolesGroup.DELETE("/:id", wrap(roleHandler.DeleteRole))
			rolesGroup.GET("/permissions", wrap(roleHandler.GetPermissions))
	}

	// Configuration endpoints
	// Wizard writes reach config.json and probe hosts: system.configure (or admin) only.
	configGate := jwtMiddleware.RequirePermission(models.PermissionSystemConfig)
	configGroup := api.Group("/configuration")
	{
			configGroup.GET("", wrap(configurationHandler.GetConfiguration))
			configGroup.POST("/test", wrap(configurationHandler.TestConfiguration))
			configGroup.GET("/status", wrap(configurationHandler.GetSystemStatus))
			configGroup.GET("/wizard/step/:step_id", wrap(configurationHandler.GetWizardStep))
			configGroup.POST("/wizard/step/:step_id/validate", configGate, wrap(configurationHandler.ValidateWizardStep))
			configGroup.POST("/wizard/step/:step_id/save", configGate, wrap(configurationHandler.SaveWizardProgress))
			configGroup.GET("/wizard/progress", wrap(configurationHandler.GetWizardProgress))
			configGroup.POST("/wizard/complete", configGate, wrap(configurationHandler.CompleteWizard))
	}

	// Error reporting endpoints
	errorsGroup := api.Group("/errors")
	{
			errorsGroup.POST("/report", wrap(errorReportingHandler.ReportError))
			errorsGroup.POST("/crash", wrap(errorReportingHandler.ReportCrash))
			errorsGroup.GET("/reports", wrap(errorReportingHandler.ListErrorReports))
			errorsGroup.GET("/reports/:id", wrap(errorReportingHandler.GetErrorReport))
			errorsGroup.PUT("/reports/:id/status", wrap(errorReportingHandler.UpdateErrorStatus))
			errorsGroup.GET("/crashes", wrap(errorReportingHandler.ListCrashReports))
			errorsGroup.GET("/crashes/:id", wrap(errorReportingHandler.GetCrashReport))
			errorsGroup.PUT("/crashes/:id/status", wrap(errorReportingHandler.UpdateCrashStatus))
			errorsGroup.GET("/statistics", wrap(errorReportingHandler.GetErrorStatistics))
			errorsGroup.GET("/crash-statistics", wrap(errorReportingHandler.GetCrashStatistics))
			errorsGroup.GET("/health", wrap(errorReportingHandler.GetSystemHealth))
	}

	// Log management endpoints
	logsGroup := api.Group("/logs")
	{
			logsGroup.POST("/collect", wrap(logManagementHandler.CreateLogCollection))
			logsGroup.GET("/collections", wrap(logManagementHandler.ListLogCollections))
			logsGroup.GET("/collections/:id", wrap(logManagementHandler.GetLogCollection))
			logsGroup.GET("/collections/:id/entries", wrap(logManagementHandler.GetLogEntries))
			logsGroup.POST("/collections/:id/export", wrap(logManagementHandler.ExportLogs))
			logsGroup.GET("/collections/:id/analyze", wrap(logManagementHandler.AnalyzeLogs))
			logsGroup.POST("/share", wrap(logManagementHandler.CreateLogShare))
			logsGroup.GET("/share/:token", wrap(logManagementHandler.GetLogShare))
			logsGroup.DELETE("/share/:id", wrap(logManagementHandler.RevokeLogShare))
			logsGroup.GET("/stream", wrap(logManagementHandler.StreamLogs))
			logsGroup.GET("/statistics", wrap(logManagementHandler.GetLogStatistics))
	}

	// Media collection endpoints
	collectionsGroup := api.Group("/collections")
	{
			collectionsGroup.GET("", collectionHandler.ListCollections)
			collectionsGroup.POST("", collectionHandler.CreateCollection)
			collectionsGroup.GET("/:id", collectionHandler.GetCollection)
			collectionsGroup.GET("/:id/items", collectionHandler.ListCollectionItems)
			collectionsGroup.PUT("/:id", collectionHandler.UpdateCollection)
			collectionsGroup.DELETE("/:id", collectionHandler.DeleteCollection)
	}

	// Asset management endpoints (authenticated)
	assetsGroup := api.Group("/assets")
	{
			assetsGroup.POST("/request", adminGate, assetHandler.RequestAsset)
			assetsGroup.GET("/by-entity/:type/:id", assetHandler.GetByEntity)
	}

	// Media entity endpoints (structured media browsing)
	entityGroup := api.Group("/entities")
	entityGroup.Use(root_middleware.CacheHeaders(300)) // 5-minute cache for entity browsing
	{
			entityGroup.GET("", mediaEntityHandler.ListEntities)
			entityGroup.GET("/search", mediaEntityHandler.SearchEntities)
			entityGroup.GET("/types", mediaEntityHandler.GetEntityTypes)
			entityGroup.GET("/stats", mediaEntityHandler.GetEntityStats)
			entityGroup.GET("/duplicates", mediaEntityHandler.ListDuplicateGroups)
			entityGroup.GET("/browse/:type", mediaEntityHandler.BrowseByType)
			entityGroup.GET("/:id", mediaEntityHandler.GetEntity)
			entityGroup.GET("/:id/children", mediaEntityHandler.GetEntityChildren)
			entityGroup.GET("/:id/files", mediaEntityHandler.GetEntityFiles)
			entityGroup.GET("/:id/metadata", mediaEntityHandler.GetEntityMetadata)
			entityGroup.GET("/:id/duplicates", mediaEntityHandler.GetEntityDuplicates)
			entityGroup.GET("/:id/stream", mediaEntityHandler.StreamEntity)
			entityGroup.GET("/:id/download", mediaEntityHandler.DownloadEntity)

			// Comic reader: list pages in a .cbz/.cbr archive and
			// stream an individual page image by 0-based index.
			entityGroup.GET("/:id/pages", comicPagesHandler.ListComicPages)
			entityGroup.GET("/:id/pages/:n", comicPagesHandler.GetComicPage)

			// Book reader: count pages in a PDF and render an individual
			// page image by 0-based index.
			entityGroup.GET("/:id/pdf-pages", pdfPagesHandler.ListPdfPages)
			entityGroup.GET("/:id/pdf-pages/:n", pdfPagesHandler.GetPdfPage)
			entityGroup.GET("/:id/install-info", mediaEntityHandler.GetInstallInfo)
			entityGroup.POST("/:id/metadata/refresh", adminGate, mediaEntityHandler.RefreshEntityMetadata)
			entityGroup.PUT("/:id/user-metadata", mediaEntityHandler.UpdateUserMetadata)
			entityGroup.POST("/:id/user-metadata", mediaEntityHandler.UpdateUserMetadata)
			entityGroup.POST("/enrich", adminGate, mediaEntityHandler.EnrichAllEntities)

			// Playback session tracking: per-entity progress
			// summary (used by the card badge) and full history
			// drawer (clicking the badge).
			entityGroup.GET("/:id/progress", playbackHandler.GetProgressForEntity)
			entityGroup.GET("/:id/history", playbackHandler.ListHistoryForEntity)
	}

	// /api/v1/playback/sessions — start/progress/end lifecycle.
	playbackGroup := api.Group("/playback")
	{
			playbackGroup.POST("/sessions/start", playbackHandler.StartSession)
			playbackGroup.POST("/sessions/progress", playbackHandler.ProgressSession)
			playbackGroup.POST("/sessions/end", playbackHandler.EndSession)
	}

	// Analytics endpoints
	analyticsGroup := api.Group("/analytics")
	{
			analyticsGroup.POST("/access", analyticsHandler.LogMediaAccess)
			analyticsGroup.POST("/event", analyticsHandler.LogEvent)
			analyticsGroup.GET("/user/:user_id", jwtMiddleware.RequireSelfOrAdmin("user_id"), analyticsHandler.GetUserAnalytics)
			analyticsGroup.GET("/system", jwtMiddleware.RequireAdmin(), analyticsHandler.GetSystemAnalytics)
			analyticsGroup.GET("/media/:media_id", jwtMiddleware.RequireAdmin(), analyticsHandler.GetMediaAnalytics)
			analyticsGroup.POST("/reports", jwtMiddleware.RequireAdmin(), analyticsHandler.CreateReport)
	}

	// Reporting endpoints
	reportingGroup := api.Group("/reports")
	{
			reportingGroup.GET("/usage", jwtMiddleware.RequireAdmin(), reportingHandler.GetUsageReport)
			reportingGroup.GET("/performance", jwtMiddleware.RequireAdmin(), reportingHandler.GetPerformanceReport)
	}

	// Favorites endpoints
	favoritesGroup := api.Group("/favorites")
	{
			favoritesGroup.GET("", favoritesHandler.ListFavorites)
			favoritesGroup.POST("", favoritesHandler.AddFavorite)
			favoritesGroup.DELETE("/:entity_type/:entity_id", favoritesHandler.RemoveFavorite)
			favoritesGroup.GET("/check/:entity_type/:entity_id", favoritesHandler.CheckFavorite)
	}

	// Playlist endpoints (lazy — handler initialized on first request)
	playlistGroup := api.Group("/playlists")
	{
			playlistGroup.GET("", func(c *gin.Context) { getPlaylistHandler().ListPlaylists(c) })
			playlistGroup.POST("", func(c *gin.Context) { getPlaylistHandler().CreatePlaylist(c) })
			playlistGroup.GET("/:id", func(c *gin.Context) { getPlaylistHandler().GetPlaylist(c) })
			playlistGroup.PUT("/:id", func(c *gin.Context) { getPlaylistHandler().UpdatePlaylist(c) })
			playlistGroup.DELETE("/:id", func(c *gin.Context) { getPlaylistHandler().DeletePlaylist(c) })
			playlistGroup.POST("/:id/items", func(c *gin.Context) { getPlaylistHandler().AddItem(c) })
			playlistGroup.DELETE("/:id/items/:item_id", func(c *gin.Context) { getPlaylistHandler().RemoveItem(c) })
	}

	// Browse endpoints (directory browsing and file info)
	browseGroup := api.Group("/browse")
	{
			browseGroup.GET("/roots", browseHandler.GetStorageRoots)
			browseGroup.GET("/directory/*path", browseHandler.BrowseDirectory)
			browseGroup.GET("/file-info/*path", browseHandler.GetFileInfo)
			browseGroup.GET("/directory-sizes/*path", browseHandler.GetDirectorySizes)
			browseGroup.GET("/duplicates/*path", browseHandler.GetDirectoryDuplicates)
	}

	// Sync endpoints (lazy — handler initialized on first request)
	syncGroup := api.Group("/sync")
	{
			syncGroup.POST("/endpoints", adminGate, func(c *gin.Context) { getSyncHandler().CreateEndpoint(c) })
			syncGroup.GET("/endpoints", func(c *gin.Context) { getSyncHandler().GetUserEndpoints(c) })
			syncGroup.GET("/endpoints/:id", func(c *gin.Context) { getSyncHandler().GetEndpoint(c) })
			syncGroup.PUT("/endpoints/:id", adminGate, func(c *gin.Context) { getSyncHandler().UpdateEndpoint(c) })
			syncGroup.DELETE("/endpoints/:id", adminGate, func(c *gin.Context) { getSyncHandler().DeleteEndpoint(c) })
			syncGroup.POST("/endpoints/:id/sync", adminGate, func(c *gin.Context) { getSyncHandler().StartSync(c) })
			syncGroup.GET("/sessions", func(c *gin.Context) { getSyncHandler().GetUserSessions(c) })
			syncGroup.GET("/sessions/:id", func(c *gin.Context) { getSyncHandler().GetSession(c) })
			syncGroup.POST("/schedules", adminGate, func(c *gin.Context) { getSyncHandler().ScheduleSync(c) })
			syncGroup.GET("/statistics", func(c *gin.Context) { getSyncHandler().GetSyncStatistics(c) })
			syncGroup.POST("/cleanup", jwtMiddleware.RequireAdmin(), func(c *gin.Context) { getSyncHandler().CleanupOldSessions(c) })
	}

	// Challenge endpoints
	challengeGroup := api.Group("/challenges")
	{
			challengeGroup.GET("", challengeHandler.ListChallenges)
			challengeGroup.GET("/:id", challengeHandler.GetChallenge)
			// Running challenges executes server-side code paths: administrators only (P2).
			challengeGroup.POST("/:id/run", jwtMiddleware.RequireAdmin(), challengeHandler.RunChallenge)
			challengeGroup.POST("/run", jwtMiddleware.RequireAdmin(), challengeHandler.RunAll)
			challengeGroup.POST("/run/category/:category", jwtMiddleware.RequireAdmin(), challengeHandler.RunByCategory)
			challengeGroup.GET("/results", challengeHandler.GetResults)
	}

	// --------------------------------------------------------------
	// Challenge-compatible alias endpoints
	// --------------------------------------------------------------
	// These routes exist solely so the HelixQA test banks and the
	// challenge userflow suites don't need to know the internal
	// route layout. Each alias delegates to an existing handler,
	// OR returns a well-formed empty-state payload so downstream
	// tests can exercise JSON validation without crashing.

	// /api/v1/browse → list storage roots (the bank treats it as
	// the "browse root" entry point; existing /browse/roots returns
	// the actual data, and we expose both).
	api.GET("/browse", browseHandler.GetStorageRoots)

	// /api/v1/analytics/stats → overall stats (challenge bank
	// phrases it this way, the canonical route is /stats/overall).
	api.GET("/analytics/stats", statsHandler.GetOverallStats)

	// /api/v1/analytics/duplicates → duplicate stats.
	api.GET("/analytics/duplicates", statsHandler.GetDuplicateStats)

	// /api/v1/users/me → current authenticated user. Canonical
	// route is /auth/me; this alias matches the test bank.
	api.GET("/users/me", authHandler.GetCurrentUserGin)

	// /api/v1/languages → list of supported subtitle languages
	// (a proxy for the wider locale/translation surface — the
	// challenge bank just wants SOMETHING back on this path).
	api.GET("/languages", func(c *gin.Context) {
			getSubtitleHandler().GetSupportedLanguages(c)
	})

	// /api/v1/recommendations → trending items as the default list.
	// The per-target endpoints live under /recommendations/{similar,
	// trending, personalized}; the bank expects a list on the root.
	api.GET("/recommendations", func(c *gin.Context) {
			getRecommendationHandler().GetTrendingItems(c)
	})
	// /api/v1/sync/history → wraps /sync/sessions.
	api.GET("/sync/history", func(c *gin.Context) {
			getSyncHandler().GetUserSessions(c)
	})
	// /localization/languages proxies the real subtitle language list.
	api.GET("/localization/languages", func(c *gin.Context) {
			getSubtitleHandler().GetSupportedLanguages(c)
	})
	// Derived-from-real-state alias routes (media-types, stats, scan history,
	// sync status/providers, files, subtitles). Routes with no real source
	// answer 501 instead of canned data (CAT-001, §11.4.289). See handlers/alias_routes.go.
	root_handlers.NewAliasHandler(databaseDB, browseHandler).RegisterAliasRoutes(api)
	}

	// Find available port for HTTP server
	startPort := cfg.Server.Port
	if startPort <= 0 {
	startPort = 8080
	}
	port, err := findAvailablePort(cfg.Server.Host, startPort, 10)
	if err != nil {
	logger.Fatal("Failed to find available port", zap.Error(err))
	}
	cfg.Server.Port = port
	addr := cfg.GetServerAddress()

	// Write port to file for service discovery
	if err := writePortFile(port); err != nil {
	logger.Warn("Failed to write port file", zap.Error(err))
	}

	logger.Info("Selected HTTP port", zap.Int("port", port))

	// Create HTTP server
	srv := &http.Server{
	Addr:              addr,
	Handler:           router,
	ReadTimeout:       time.Duration(cfg.Server.ReadTimeout) * time.Second,
	ReadHeaderTimeout: 5 * time.Second,
	WriteTimeout:      time.Duration(cfg.Server.WriteTimeout) * time.Second,
	IdleTimeout:       time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}
	// HTTPS server for TLS and HTTP/2 (future HTTP/3)
	var httpsServer *http.Server
	var http3Server *http3.Server

	// Load or generate TLS certificate for HTTP/3 (cached across restarts)
	cert, err := getOrCreateSelfSignedCert()
	if err != nil {
	logger.Error("Failed to get TLS certificate for HTTP/3", zap.Error(err))
	} else {
	// Create TLS config with the certificate
	tlsConfig := &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{"h3", "h2", "http/1.1"},
	}

	// Start HTTPS server on port 28443 (HTTP/2 with TLS, fallback for HTTP/3)
	httpsAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, 28443)
	httpsServer = &http.Server{
			Addr:              httpsAddr,
			Handler:           router,
			TLSConfig:         tlsConfig,
			ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
			logger.Info("Starting HTTPS server (HTTP/2 with TLS)", zap.String("address", httpsAddr))
			if err := httpsServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				logger.Error("HTTPS server failed", zap.Error(err))
			}
	}()

	// Add Alt-Svc header to advertise HTTP/3 support
	router.Use(func(c *gin.Context) {
			c.Header("Alt-Svc", `h3=":28443"; ma=86400`)
			c.Next()
	})

	// Start HTTP/3 server on UDP port 28443
	http3Server = &http3.Server{
			Addr:      httpsAddr,
			Handler:   router,
			TLSConfig: tlsConfig,
	}
	go func() {
			logger.Info("Starting HTTP/3 (QUIC) server", zap.String("address", httpsAddr))
			if err := http3Server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Error("HTTP/3 server failed", zap.Error(err))
			}
	}()
	}

	// Start server in a goroutine
	go func() {
	logger.Info("Starting catalog API server", zap.String("address", cfg.GetServerAddress()))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("Failed to start server", zap.Error(err))
	}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info("Shutting down server...")

	// The context is used to inform the server it has 30 seconds to finish
	// the request it is currently handling
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop the conversion worker: running converter tools are killed, their jobs go back to
	// `pending`, and we wait for the job goroutines to finish.
	// The wait is bounded by its own 10s timer (not by the shutdown context; at most 10s of the
	// 30s shutdown budget are spent). If the worker has not stopped, the database is NOT closed
	// below (job goroutines may still write), and the process exit ends the rest. Converter tools are killed by process group on cancel, so this
	// normally takes milliseconds.
	conversionWorkerCancel()
	conversionWorkerStopped := false
	select {
	case <-conversionWorkerDone:
	conversionWorkerStopped = true
	case <-time.After(10 * time.Second):
	logger.Error("Conversion worker did not stop within 10s; the database will not be closed while jobs may still write")
	}

	// Stop runtime metrics collector
	metrics.StopRuntimeCollector()

	// Stop WebSocket handler (closes all client connections, stops cleanup goroutine)
	wsHandler.Stop()

	// Stop cache service cleanup goroutine
	cacheService.Close()

	// Wait for background TMDB enrichment goroutines to finish
	mediaEntityHandler.Close()

	// Wait for background log stream relay goroutines to finish
	logAdapter.Close()

	// Stop all middleware cleanup goroutines (rate limiters, etc.) so they
	// don't outlive the server and leak until process exit.
	root_middleware.StopAll()

	// Shutdown HTTP server (stops accepting new connections, waits for in-flight requests)
	if err := srv.Shutdown(shutdownCtx); err != nil {
	logger.Error("HTTP server shutdown error", zap.Error(err))
	}

	// Shutdown HTTPS server if started
	if httpsServer != nil {
	if err := httpsServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("HTTPS server shutdown error", zap.Error(err))
	} else {
			logger.Info("HTTPS server shut down gracefully")
	}
	}

	// Shutdown HTTP/3 server if started
	if http3Server != nil {
	if err := http3Server.Close(); err != nil {
			logger.Error("HTTP/3 server shutdown error", zap.Error(err))
	} else {
			logger.Info("HTTP/3 server shut down gracefully")
	}
	}

	// Close Redis connection if available
	if redisClient != nil {
	if err := redisClient.Close(); err != nil {
			logger.Error("Redis connection close error", zap.Error(err))
	} else {
			logger.Info("Redis connection closed")
	}
	}

	// Close database connection (never while conversion job goroutines may still write)
	if !conversionWorkerStopped {
	logger.Warn("Skipping database close: conversion worker still running")
	} else if err := databaseDB.Close(); err != nil {
	logger.Error("Database close error", zap.Error(err))
	} else {
	logger.Info("Database connection closed")
	}

	// Clean up service port file
	if err := os.Remove(".service-port"); err != nil && !os.IsNotExist(err) {
	logger.Warn("Failed to remove service port file", zap.Error(err))
	}

	logger.Info("Server exited cleanly")
}

// seedDefaultAdmin creates a default admin user if none exists in the database.
// Uses the same password hashing scheme as services.AuthService (bcrypt(password + salt)).
func seedDefaultAdmin(db *database.DB, username, password string) error {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users WHERE role_id = 1").Scan(&count)
	if err != nil {
	return fmt.Errorf("check admin count: %w", err)
	}
	if count > 0 {
	if !root_config.AllowInsecureDefaults() {
		if err := refuseCompromisedAdminHashes(db); err != nil {
			return err
		}
	}
	return nil // admin already exists
	}

	// Never seed an empty or placeholder password (CAT-021), regardless of
	// whether auth is currently enabled.
	if password == "" || (root_config.IsPlaceholderPassword(password) && !root_config.AllowInsecureDefaults()) {
	return fmt.Errorf("refusing to seed admin %q: ADMIN_PASSWORD is empty or a known placeholder/default", username)
	}

	// Generate salt
	saltBytes := make([]byte, 16)
	if _, err := rand.Read(saltBytes); err != nil {
	return fmt.Errorf("generate salt: %w", err)
	}
	salt := hex.EncodeToString(saltBytes)

	// Hash password with salt (same as services.AuthService.hashPassword)
	hash, err := root_services.HashPasswordWithSalt(password, salt)
	if err != nil {
	return fmt.Errorf("hash password: %w", err)
	}

	_, err = db.Exec(
	`INSERT INTO users (username, email, password_hash, salt, role_id, first_name, last_name, display_name, is_active)
	VALUES (?, ?, ?, ?, 1, 'System', 'Administrator', 'Admin', ?)`,
	username, username+"@catalogizer.local", hash, salt, 1,
	)
	if err != nil {
	return fmt.Errorf("insert admin user: %w", err)
	}

	logging.Infof("Default admin user '%s' created", username)
	return nil
}

// errCompromisedAdmin marks an existing admin account that still uses a
// published default password.
var errCompromisedAdmin = errors.New("existing admin account uses a published default password")

// refuseCompromisedAdminHashes checks stored (salted) admin hashes against the
// denylist so upgraded installs cannot keep the compromised default.
func refuseCompromisedAdminHashes(db *database.DB) error {
	rows, err := db.Query("SELECT username, password_hash, COALESCE(salt,'') FROM users WHERE role_id = 1")
	if err != nil {
	return fmt.Errorf("%w: %v", errAdminHashCheck, err)
	}
	defer rows.Close()
	type acct struct{ user, hash, salt string }
	var accts []acct
	for rows.Next() {
	var a acct
	if err := rows.Scan(&a.user, &a.hash, &a.salt); err != nil {
		return fmt.Errorf("%w: %v", errAdminHashCheck, err)
	}
	accts = append(accts, a)
	}
	if err := rows.Err(); err != nil {
	return fmt.Errorf("%w: %v", errAdminHashCheck, err)
	}
	for _, a := range accts {
	for _, c := range root_config.PlaceholderPasswordCandidates() {
		if ok, _ := root_services.CheckPassword(c, a.salt, a.hash); ok {
		return fmt.Errorf("%w: account %q; run the offline recovery `catalog-api admin-reset-password` (with ADMIN_NEW_PASSWORD set) before starting", errCompromisedAdmin, a.user)
		}
	}
	}
	return nil
}

// dnsCacheEntry holds resolved IPs with an expiration time.
type dnsCacheEntry struct {
	ips       []string
	expiresAt time.Time
}

var (
	dnsCache      = make(map[string]dnsCacheEntry)
	dnsCacheMu    sync.RWMutex
	dnsCacheTTL   = 5 * time.Minute
	dnsDoHClient  = &http.Client{Timeout: 5 * time.Second}
)

// resolveViaDOH queries Cloudflare DNS-over-HTTPS for A records.
func resolveViaDOH(hostname string) ([]string, error) {
	reqURL := fmt.Sprintf("https://cloudflare-dns.com/dns-query?name=%s&type=A", url.QueryEscape(hostname))
	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
	return nil, err
	}
	req.Header.Set("accept", "application/dns-json")
	resp, err := dnsDoHClient.Do(req)
	if err != nil {
	return nil, err
	}
	defer resp.Body.Close()

	var payload struct {
	Status int `json:"Status"`
	Answer []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
	} `json:"Answer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
	return nil, err
	}
	var ips []string
	for _, ans := range payload.Answer {
	if ans.Type == 1 {
			ips = append(ips, ans.Data)
	}
	}
	return ips, nil
}

// resolveHostDynamic returns non-loopback IPs for a hostname.
// It prefers DNS-over-HTTPS (which gives real CDN edge IPs) and only falls
// back to local DNS when DoH is unavailable. This avoids caching bad local
// DNS results (e.g., CloudFront IPs that reject direct IP access).
func resolveHostDynamic(host string) []string {
	// Check cache first.
	dnsCacheMu.RLock()
	entry, ok := dnsCache[host]
	dnsCacheMu.RUnlock()
	if ok && time.Now().Before(entry.expiresAt) {
	return entry.ips
	}

	// Prefer DNS-over-HTTPS for accurate CDN edge IPs.
	valid, err := resolveViaDOH(host)
	if err == nil && len(valid) > 0 {
	dnsCacheMu.Lock()
	dnsCache[host] = dnsCacheEntry{ips: valid, expiresAt: time.Now().Add(dnsCacheTTL)}
	dnsCacheMu.Unlock()
	return valid
	}

	// Fall back to local DNS only when DoH fails.
	addrs, err := net.LookupHost(host)
	if err == nil && len(addrs) > 0 {
	var localValid []string
	for _, a := range addrs {
			if a != "127.0.0.1" && a != "::1" {
				localValid = append(localValid, a)
			}
	}
	if len(localValid) > 0 {
			// Do not cache local DNS results so we can retry DoH later.
			return localValid
	}
	}

	return nil
}

// buildProxyHTTPClient returns a basic http.Client that routes through the
// configured proxy (SOCKS5 or HTTP) when enabled.
func buildProxyHTTPClient(proxyCfg root_config.ProxyConfig) *http.Client {
	transport := &http.Transport{}
	if proxyCfg.Enabled {
	if proxyCfg.URL != "" {
			parsedProxy, err := url.Parse(proxyCfg.URL)
			if err == nil && parsedProxy.Scheme == "socks5" {
				var auth *proxy.Auth
				if proxyCfg.Username != "" || proxyCfg.Password != "" {
					auth = &proxy.Auth{User: proxyCfg.Username, Password: proxyCfg.Password}
				}
				SOCKS5Dialer, err := proxy.SOCKS5("tcp", parsedProxy.Host, auth, proxy.Direct)
				if err == nil {
					transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
						return SOCKS5Dialer.Dial(network, addr)
					}
					return &http.Client{Timeout: 30 * time.Second, Transport: transport}
				}
			}
	}
	if proxyCfg.HTTPURL != "" {
			parsedHTTPProxy, err := url.Parse(proxyCfg.HTTPURL)
			if err == nil {
				transport.Proxy = http.ProxyURL(parsedHTTPProxy)
				return &http.Client{Timeout: 30 * time.Second, Transport: transport}
			}
	}
	}
	return &http.Client{Timeout: 30 * time.Second}
}

// buildImageProxyClient returns an http.Client for fetching images from external CDNs.
// When a proxy is configured it is used first. When local DNS returns loopback
// addresses, it falls back to DNS-over-HTTPS to resolve real IPs dynamically
// and dials those IPs while preserving TLS SNI.
func buildImageProxyClient(imageURL string, proxyCfg root_config.ProxyConfig) *http.Client {
	parsed, err := url.Parse(imageURL)
	if err != nil {
	return &http.Client{Timeout: 15 * time.Second}
	}
	host := parsed.Hostname()

	transport := &http.Transport{
	TLSClientConfig: &tls.Config{ServerName: host},
	}

	// Use configured proxy if enabled.
	if proxyCfg.Enabled {
	if proxyCfg.URL != "" {
			parsedProxy, err := url.Parse(proxyCfg.URL)
			if err == nil && parsedProxy.Scheme == "socks5" {
				var auth *proxy.Auth
				if proxyCfg.Username != "" || proxyCfg.Password != "" {
					auth = &proxy.Auth{User: proxyCfg.Username, Password: proxyCfg.Password}
				}
				 SOCKS5Dialer, err := proxy.SOCKS5("tcp", parsedProxy.Host, auth, proxy.Direct)
				if err == nil {
					transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
						return SOCKS5Dialer.Dial(network, addr)
					}
					return &http.Client{Timeout: 15 * time.Second, Transport: transport}
				}
			}
	}
	if proxyCfg.HTTPURL != "" {
			parsedHTTPProxy, err := url.Parse(proxyCfg.HTTPURL)
			if err == nil {
				transport.Proxy = http.ProxyURL(parsedHTTPProxy)
				return &http.Client{Timeout: 15 * time.Second, Transport: transport}
			}
	}
	}

	ips := resolveHostDynamic(host)
	if len(ips) == 0 {
	transport.DialContext = netguard.DialContext // checked + pinned
	return &http.Client{Timeout: 15 * time.Second, Transport: transport}
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
			port = "443"
	}
	var lastErr error
	for _, ip := range ips {
			if netguard.IsBlockedIP(net.ParseIP(ip)) {
				lastErr = fmt.Errorf("%w: %s", netguard.ErrBlocked, ip)
				continue
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip, port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
	}
	return nil, lastErr
	}
	return &http.Client{Timeout: 15 * time.Second, Transport: transport}
}

// imageProxyHosts is the exact set of CDN hosts the public image proxy may fetch from.
var imageProxyHosts = map[string]bool{"image.tmdb.org": true, "img.omdbapi.com": true, "images.igdb.com": true}

// validateImageProxyURL accepts only https://<allow-listed host>[:443]/... with no
// credentials: the host is compared as parsed, never as a substring of the raw string.
func validateImageProxyURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}
	if u.Scheme != "https" || u.User != nil || !imageProxyHosts[strings.ToLower(u.Hostname())] {
		return nil, fmt.Errorf("image proxy: destination not allowed")
	}
	if port := u.Port(); port != "" && port != "443" {
		return nil, fmt.Errorf("image proxy: destination not allowed")
	}
	u.Host = strings.ToLower(u.Host)
	return u, nil
}

// imageRedirectOK allows a redirect only to another allow-listed https URL.
func imageRedirectOK(u *url.URL) bool {
	_, err := validateImageProxyURL(u.String())
	return err == nil
}

// newImageProxyClient is buildImageProxyClient plus a redirect policy: a response from
// an allow-listed host may only redirect to another allow-listed https URL.
func newImageProxyClient(imageURL string, proxyCfg root_config.ProxyConfig) *http.Client {
	cl := buildImageProxyClient(imageURL, proxyCfg)
	cl.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 3 || !imageRedirectOK(req.URL) {
			return http.ErrUseLastResponse
		}
		return nil
	}
	return cl
}

// adminCSRFOptions resolves the admin CSRF options from the process environment.
// It deliberately reads the RAW APP_ENV: -test-mode overrides the logger env to
// "development", which must never let CSRF_INSECURE_DEV weaken a production deploy.
func adminCSRFOptions(getenv func(string) string, warnf func(string, ...any)) []root_middleware.CSRFOption {
	return root_middleware.CSRFOptionsFromEnv(getenv(root_middleware.CSRFInsecureDevEnvVar), getenv("APP_ENV"), warnf)
}
