package media

import (
	"catalogizer/internal/config"
	"catalogizer/internal/media/analyzer"
	"catalogizer/internal/media/database"
	"catalogizer/internal/media/detector"
	"catalogizer/internal/media/models"
	"catalogizer/internal/media/providers"
	"catalogizer/internal/media/realtime"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	catalogDB "catalogizer/database"

	"go.uber.org/zap"
)

// MediaManager orchestrates all media-related functionality
type MediaManager struct {
	config          *config.Config
	logger          *zap.Logger
	mediaDB         *database.MediaDatabase
	detector        *detector.DetectionEngine
	providerManager *providers.ProviderManager
	analyzer        *analyzer.MediaAnalyzer
	changeWatcher   *realtime.SMBChangeWatcher
	started         bool
}

// MediaConfig represents media-specific configuration
type MediaConfig struct {
	DatabasePath     string            `json:"database_path"`
	DatabasePassword string            `json:"database_password"`
	APIKeys          map[string]string `json:"api_keys"`
	WatchPaths       []WatchPath       `json:"watch_paths"`
	AnalysisWorkers  int               `json:"analysis_workers"`
	EnableRealtime   bool              `json:"enable_realtime"`
}

// WatchPath represents a path to monitor for changes
type WatchPath struct {
	SmbRoot   string `json:"smb_root"`
	LocalPath string `json:"local_path"`
	Enabled   bool   `json:"enabled"`
}

// NewMediaManager creates a new media manager
func NewMediaManager(cfg *config.Config, logger *zap.Logger, proxyCfg providers.ProxyConfiger) (*MediaManager, error) {
	// Initialize encrypted database
	dbPassword := os.Getenv("MEDIA_DB_PASSWORD")
	if dbPassword == "" {
		return nil, fmt.Errorf("MEDIA_DB_PASSWORD environment variable is required")
	}
	dbConfig := database.DatabaseConfig{
		Path:     "media_catalog.db",
		Password: dbPassword,
	}

	mediaDB, err := database.NewMediaDatabase(dbConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize media database: %w", err)
	}

	// Initialize detection engine
	detectionEngine := detector.NewDetectionEngine(logger)

	// Load detection rules and media types from database
	if err := loadDetectionRules(mediaDB, detectionEngine); err != nil {
		logger.Error("Failed to load detection rules", zap.Error(err))
	}

	// Initialize provider manager
	providerManager := providers.NewProviderManagerWithProxy(logger, proxyCfg)

	// Initialize analyzer
	mediaAnalyzer := analyzer.NewMediaAnalyzer(catalogDB.WrapDB(mediaDB.GetDB(), catalogDB.DialectSQLite), detectionEngine, providerManager, logger)

	// Initialize change watcher
	changeWatcher := realtime.NewSMBChangeWatcher(mediaDB, mediaAnalyzer, logger)

	mm := &MediaManager{
		config:          cfg,
		logger:          logger,
		mediaDB:         mediaDB,
		detector:        detectionEngine,
		providerManager: providerManager,
		analyzer:        mediaAnalyzer,
		changeWatcher:   changeWatcher,
	}

	return mm, nil
}

// Start starts all media services
func (mm *MediaManager) Start() error {
	if mm.started {
		return nil
	}

	mm.logger.Info("Starting Media Manager")

	// Start analyzer
	mm.analyzer.Start()

	// Start change watcher
	if err := mm.changeWatcher.Start(); err != nil {
		return fmt.Errorf("failed to start change watcher: %w", err)
	}

	// Add watch paths (this would normally come from config)
	watchPaths := []WatchPath{
		{SmbRoot: "nas1", LocalPath: "/mnt/smb/nas1", Enabled: true},
		{SmbRoot: "nas2", LocalPath: "/mnt/smb/nas2", Enabled: true},
	}

	for _, path := range watchPaths {
		if path.Enabled {
			if err := mm.changeWatcher.WatchSMBPath(path.SmbRoot, path.LocalPath); err != nil {
				mm.logger.Error("Failed to watch SMB path",
					zap.String("smb_root", path.SmbRoot),
					zap.String("local_path", path.LocalPath),
					zap.Error(err))
			}
		}
	}

	mm.started = true
	mm.logger.Info("Media Manager started successfully")

	return nil
}

// Stop stops all media services
func (mm *MediaManager) Stop() {
	if !mm.started {
		return
	}

	mm.logger.Info("Stopping Media Manager")

	// Stop services in reverse order
	mm.changeWatcher.Stop()
	mm.analyzer.Stop()

	// Close database
	if err := mm.mediaDB.Close(); err != nil {
		mm.logger.Error("Failed to close media database", zap.Error(err))
	}

	mm.started = false
	mm.logger.Info("Media Manager stopped")
}

// GetDatabase returns the media database
func (mm *MediaManager) GetDatabase() *database.MediaDatabase {
	return mm.mediaDB
}

// GetAnalyzer returns the media analyzer
func (mm *MediaManager) GetAnalyzer() *analyzer.MediaAnalyzer {
	return mm.analyzer
}

// GetChangeWatcher returns the change watcher
func (mm *MediaManager) GetChangeWatcher() *realtime.SMBChangeWatcher {
	return mm.changeWatcher
}

// AnalyzeAllDirectories triggers analysis of all directories in the catalog
func (mm *MediaManager) AnalyzeAllDirectories(ctx context.Context) error {
	mm.logger.Info("Starting full directory analysis")

	// Get all unique directory paths from the catalog
	query := `
		SELECT DISTINCT
			CASE
				WHEN is_directory = 1 THEN path
				ELSE substr(path, 1, length(path) - length(name) - 1)
			END as directory_path,
			smb_root
		FROM files
		WHERE directory_path != ''
		ORDER BY smb_root, directory_path
	`

	rows, err := mm.mediaDB.GetDB().Query(query)
	if err != nil {
		return fmt.Errorf("failed to get directories: %w", err)
	}
	defer rows.Close()

	var directories []struct {
		Path    string
		SmbRoot string
	}

	for rows.Next() {
		var dir struct {
			Path    string
			SmbRoot string
		}
		if err := rows.Scan(&dir.Path, &dir.SmbRoot); err != nil {
			continue
		}
		directories = append(directories, dir)
	}

	mm.logger.Info("Found directories to analyze", zap.Int("count", len(directories)))

	// Queue all directories for analysis
	for i, dir := range directories {
		priority := 10 - (i / 100) // Decrease priority for later items
		if priority < 1 {
			priority = 1
		}

		if err := mm.analyzer.AnalyzeDirectory(ctx, dir.Path, dir.SmbRoot, priority); err != nil {
			mm.logger.Error("Failed to queue directory analysis",
				zap.String("path", dir.Path),
				zap.String("smb_root", dir.SmbRoot),
				zap.Error(err))
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	mm.logger.Info("All directories queued for analysis")
	return nil
}

// RefreshExternalMetadata refreshes metadata for all media items
func (mm *MediaManager) RefreshExternalMetadata(ctx context.Context, olderThan time.Duration) error {
	mm.logger.Info("Refreshing external metadata", zap.Duration("older_than", olderThan))

	cutoff := time.Now().Add(-olderThan)

	query := `
		SELECT mi.id, mi.title, mi.year, mt.name as media_type
		FROM media_items mi
		JOIN media_types mt ON mi.media_type_id = mt.id
		WHERE mi.id NOT IN (
			SELECT media_item_id FROM external_metadata
			WHERE last_fetched > ?
		)
		OR mi.id IN (
			SELECT media_item_id FROM external_metadata
			WHERE last_fetched < ?
		)
		ORDER BY mi.last_updated DESC
		LIMIT 100
	`

	rows, err := mm.mediaDB.GetDB().Query(query, cutoff, cutoff)
	if err != nil {
		return fmt.Errorf("failed to get media items for metadata refresh: %w", err)
	}
	defer rows.Close()

	var mediaItems []struct {
		ID        int64
		Title     string
		Year      *int
		MediaType string
	}

	for rows.Next() {
		var item struct {
			ID        int64
			Title     string
			Year      *int
			MediaType string
		}
		if err := rows.Scan(&item.ID, &item.Title, &item.Year, &item.MediaType); err != nil {
			continue
		}
		mediaItems = append(mediaItems, item)
	}

	mm.logger.Info("Found media items for metadata refresh", zap.Int("count", len(mediaItems)))

	// Refresh metadata for each item
	for _, item := range mediaItems {
		// Get best match from providers
		bestResult, providerName, err := mm.providerManager.GetBestMatch(
			ctx, item.Title, item.MediaType, item.Year,
		)
		if err != nil || bestResult == nil {
			mm.logger.Debug("No metadata found",
				zap.String("title", item.Title),
				zap.String("media_type", item.MediaType))
			continue
		}

		// Get detailed metadata
		metadata, err := mm.providerManager.GetDetails(ctx, providerName, bestResult.ExternalID)
		if err != nil {
			mm.logger.Error("Failed to get metadata details",
				zap.String("title", item.Title),
				zap.String("provider", providerName),
				zap.Error(err))
			continue
		}

		metadata.MediaItemID = item.ID

		// Save to database
		saveQuery := `
			INSERT OR REPLACE INTO external_metadata
			(media_item_id, provider, external_id, data, rating, review_url, cover_url, trailer_url, last_fetched)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		`

		_, err = mm.mediaDB.GetDB().Exec(saveQuery,
			metadata.MediaItemID, metadata.Provider, metadata.ExternalID, metadata.Data,
			metadata.Rating, metadata.ReviewURL, metadata.CoverURL, metadata.TrailerURL, metadata.LastFetched,
		)
		if err != nil {
			mm.logger.Error("Failed to save metadata",
				zap.String("title", item.Title),
				zap.Error(err))
		} else {
			mm.logger.Debug("Metadata refreshed",
				zap.String("title", item.Title),
				zap.String("provider", providerName))
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Rate limiting - don't overwhelm APIs
		time.Sleep(100 * time.Millisecond)
	}

	mm.logger.Info("External metadata refresh completed")
	return nil
}

// GetStatistics returns comprehensive media statistics
func (mm *MediaManager) GetStatistics() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Database statistics
	dbStats, err := mm.mediaDB.GetStats()
	if err != nil {
		mm.logger.Error("Failed to get database stats", zap.Error(err))
	} else {
		stats["database"] = dbStats
	}

	// Change statistics (last 24 hours)
	since := time.Now().Add(-24 * time.Hour)
	changeStats, err := mm.changeWatcher.GetChangeStatistics(since)
	if err != nil {
		mm.logger.Error("Failed to get change stats", zap.Error(err))
	} else {
		stats["changes_24h"] = changeStats
	}

	// Media type distribution
	typeDistribution, err := mm.getMediaTypeDistribution()
	if err != nil {
		mm.logger.Error("Failed to get media type distribution", zap.Error(err))
	} else {
		stats["media_types"] = typeDistribution
	}

	// Quality distribution
	qualityDistribution, err := mm.getQualityDistribution()
	if err != nil {
		mm.logger.Error("Failed to get quality distribution", zap.Error(err))
	} else {
		stats["quality"] = qualityDistribution
	}

	// External metadata coverage
	metadataCoverage, err := mm.getMetadataCoverage()
	if err != nil {
		mm.logger.Error("Failed to get metadata coverage", zap.Error(err))
	} else {
		stats["metadata_coverage"] = metadataCoverage
	}

	stats["timestamp"] = time.Now()
	stats["uptime"] = mm.started

	return stats, nil
}

// Helper methods

func loadDetectionRules(mediaDB *database.MediaDatabase, engine *detector.DetectionEngine) error {
	// Load media types
	mediaTypesQuery := `
		SELECT id, name, description, detection_patterns, metadata_providers, created_at, updated_at
		FROM media_types
		ORDER BY id
	`

	rows, err := mediaDB.GetDB().Query(mediaTypesQuery)
	if err != nil {
		return err
	}
	defer rows.Close()

	var mediaTypes []models.MediaType
	for rows.Next() {
		var mt models.MediaType
		var patternsJSON, providersJSON string

		err := rows.Scan(&mt.ID, &mt.Name, &mt.Description, &patternsJSON, &providersJSON, &mt.CreatedAt, &mt.UpdatedAt)
		if err != nil {
			continue
		}

		// Parse JSON (simplified - would use proper JSON parsing)
		mt.DetectionPatterns = []string{patternsJSON}
		mt.MetadataProviders = []string{providersJSON}

		mediaTypes = append(mediaTypes, mt)
	}

	// Load detection rules
	rulesQuery := `
		SELECT id, media_type_id, rule_name, rule_type, pattern, confidence_weight, enabled, priority, created_at
		FROM detection_rules
		WHERE enabled = true
		ORDER BY priority DESC
	`

	ruleRows, err := mediaDB.GetDB().Query(rulesQuery)
	if err != nil {
		return err
	}
	defer ruleRows.Close()

	var rules []models.DetectionRule
	for ruleRows.Next() {
		var rule models.DetectionRule
		err := ruleRows.Scan(
			&rule.ID, &rule.MediaTypeID, &rule.RuleName, &rule.RuleType,
			&rule.Pattern, &rule.ConfidenceWeight, &rule.Enabled, &rule.Priority, &rule.CreatedAt,
		)
		if err != nil {
			continue
		}
		rules = append(rules, rule)
	}

	// Load rules into engine
	engine.LoadRules(rules, mediaTypes)

	return nil
}

func (mm *MediaManager) getMediaTypeDistribution() (map[string]int, error) {
	query := `
		SELECT mt.name, COUNT(mi.id) as count
		FROM media_types mt
		LEFT JOIN media_items mi ON mt.id = mi.media_type_id
		GROUP BY mt.id, mt.name
		ORDER BY count DESC
	`

	rows, err := mm.mediaDB.GetDB().Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	distribution := make(map[string]int)
	for rows.Next() {
		var mediaType string
		var count int
		if err := rows.Scan(&mediaType, &count); err != nil {
			continue
		}
		distribution[mediaType] = count
	}

	return distribution, nil
}

func (mm *MediaManager) getQualityDistribution() (map[string]int, error) {
	return models.QualityDistribution(mm.mediaDB.GetDB(), true)
}

func (mm *MediaManager) getMetadataCoverage() (map[string]interface{}, error) {
	coverage := make(map[string]interface{})

	// Total media items
	var totalItems int
	err := mm.mediaDB.GetDB().QueryRow("SELECT COUNT(*) FROM media_items").Scan(&totalItems)
	if err != nil {
		return nil, err
	}
	coverage["total_items"] = totalItems

	// Items with external metadata
	var itemsWithMetadata int
	err = mm.mediaDB.GetDB().QueryRow(
		"SELECT COUNT(DISTINCT media_item_id) FROM external_metadata",
	).Scan(&itemsWithMetadata)
	if err != nil {
		return nil, err
	}
	coverage["items_with_metadata"] = itemsWithMetadata

	// Coverage percentage
	if totalItems > 0 {
		coverage["coverage_percentage"] = float64(itemsWithMetadata) / float64(totalItems) * 100
	} else {
		coverage["coverage_percentage"] = 0.0
	}

	// Coverage by provider
	providerQuery := `
		SELECT provider, COUNT(*) as count
		FROM external_metadata
		GROUP BY provider
		ORDER BY count DESC
	`

	rows, err := mm.mediaDB.GetDB().Query(providerQuery)
	if err != nil {
		return coverage, nil // Return partial results
	}
	defer rows.Close()

	providerCoverage := make(map[string]int)
	for rows.Next() {
		var provider string
		var count int
		if err := rows.Scan(&provider, &count); err != nil {
			continue
		}
		providerCoverage[provider] = count
	}
	coverage["by_provider"] = providerCoverage

	return coverage, nil
}

// ExportData exports media data for backup or migration
func (mm *MediaManager) ExportData(exportPath string) error {
	mm.logger.Info("Exporting media data", zap.String("path", exportPath))

	// Create backup of the encrypted database
	backupPath := filepath.Join(exportPath, fmt.Sprintf("media_backup_%d.db", time.Now().Unix()))
	if err := mm.mediaDB.Backup(backupPath); err != nil {
		return fmt.Errorf("failed to backup database: %w", err)
	}

	// Export statistics
	stats, err := mm.GetStatistics()
	if err != nil {
		return fmt.Errorf("failed to get statistics: %w", err)
	}

	statsPath := filepath.Join(exportPath, "media_stats.json")
	statsJSON, _ := json.Marshal(stats)
	if err := os.WriteFile(statsPath, statsJSON, 0644); err != nil {
		return fmt.Errorf("failed to export statistics: %w", err)
	}

	mm.logger.Info("Media data exported successfully",
		zap.String("backup_path", backupPath),
		zap.String("stats_path", statsPath))

	return nil
}
