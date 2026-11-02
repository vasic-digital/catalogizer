package services

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"catalogizer/database"

	"go.uber.org/zap"
)

type CacheService struct {
	db        *database.DB
	logger    *zap.Logger
	wg        sync.WaitGroup // Tracks background goroutines for graceful shutdown
	shutdown  chan struct{}  // Signals shutdown to prevent new goroutines
	closeMu   sync.Mutex     // Guards shutdown-check + wg.Add atomicity
	closeOnce sync.Once      // Ensures Close() is safe to call multiple times
}

type CacheEntry struct {
	ID        int64     `json:"id" db:"id"`
	CacheKey  string    `json:"cache_key" db:"cache_key"`
	Value     string    `json:"value" db:"value"`
	ExpiresAt time.Time `json:"expires_at" db:"expires_at"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type MediaMetadataCache struct {
	ID           int64     `json:"id"`
	MediaItemID  int64     `json:"media_item_id"`
	MetadataType string    `json:"metadata_type"`
	Provider     string    `json:"provider"`
	Data         string    `json:"data"`
	Quality      float64   `json:"quality"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type APICache struct {
	ID          int64     `json:"id"`
	Provider    string    `json:"provider"`
	Endpoint    string    `json:"endpoint"`
	RequestHash string    `json:"request_hash"`
	Response    string    `json:"response"`
	StatusCode  int       `json:"status_code"`
	ExpiresAt   time.Time `json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type ThumbnailCache struct {
	ID        int64     `json:"id"`
	VideoID   int64     `json:"video_id"`
	Position  int64     `json:"position"`
	URL       string    `json:"url"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	FileSize  int64     `json:"file_size"`
	CreatedAt time.Time `json:"created_at"`
}

type CacheStats struct {
	TotalEntries     int64            `json:"total_entries"`
	TotalSize        int64            `json:"total_size"`
	HitRate          float64          `json:"hit_rate"`
	MissRate         float64          `json:"miss_rate"`
	ExpiredEntries   int64            `json:"expired_entries"`
	CachesByType     map[string]int64 `json:"caches_by_type"`
	CachesByProvider map[string]int64 `json:"caches_by_provider"`
	RecentActivity   []CacheActivity  `json:"recent_activity"`
}

type CacheActivity struct {
	Type      string    `json:"type"`
	Key       string    `json:"key"`
	Provider  string    `json:"provider"`
	Hit       bool      `json:"hit"`
	Timestamp time.Time `json:"timestamp"`
}

const (
	DefaultCacheTTL     = 24 * time.Hour
	MetadataCacheTTL    = 7 * 24 * time.Hour
	ThumbnailCacheTTL   = 30 * 24 * time.Hour
	APICacheTTL         = 1 * time.Hour
	TranslationCacheTTL = 30 * 24 * time.Hour
	SubtitleCacheTTL    = 7 * 24 * time.Hour
	LyricsCacheTTL      = 14 * 24 * time.Hour
	CoverArtCacheTTL    = 30 * 24 * time.Hour

	// Cache size limits to prevent unbounded growth
	MaxCacheEntriesTotal    = 100000 // Maximum total cache entries
	MaxCacheEntriesPerTable = 25000  // Maximum entries per table
	CacheSizeCheckInterval  = 100    // Check size every N operations
)

// CacheCleanupInterval is the interval between automatic cache cleanups
const CacheCleanupInterval = 1 * time.Hour

// NewCacheService creates a new cache service with automatic cleanup
func NewCacheService(db *database.DB, logger *zap.Logger) *CacheService {
	s := &CacheService{
		db:       db,
		logger:   logger,
		shutdown: make(chan struct{}),
	}

	// Start automatic cleanup goroutine
	s.wg.Add(1)
	go s.cleanupLoop()

	logger.Info("Cache service created",
		zap.Duration("cleanup_interval", CacheCleanupInterval))

	return s
}

// cleanupContext returns a context that is cancelled when the cleanup
// timeout expires OR the service is shutting down, whichever comes first.
func (s *CacheService) cleanupContext(timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	go func() {
		select {
		case <-s.shutdown:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

// cleanupLoop runs periodic cache cleanup
func (s *CacheService) cleanupLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(CacheCleanupInterval)
	defer ticker.Stop()

	// Run initial cleanup
	ctx, cancel := s.cleanupContext(5 * time.Minute)
	if err := s.CleanupExpired(ctx); err != nil {
		s.logger.Error("Initial cache cleanup failed", zap.Error(err))
	}
	cancel()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := s.cleanupContext(5 * time.Minute)
			if err := s.CleanupExpired(ctx); err != nil {
				s.logger.Error("Cache cleanup failed", zap.Error(err))
			}
			cancel()
		case <-s.shutdown:
			s.logger.Info("Cache cleanup loop stopping")
			return
		}
	}
}

// Close gracefully shuts down the cache service, waiting for pending operations.
// Safe to call multiple times.
func (s *CacheService) Close() {
	s.closeOnce.Do(func() {
		s.closeMu.Lock()
		close(s.shutdown)
		s.closeMu.Unlock()

		done := make(chan struct{})
		go func() {
			s.wg.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			s.logger.Warn("Cache service shutdown timed out after 10s, some goroutines may still be running")
		}
	})
}

func (s *CacheService) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// If no database is available (e.g., in tests), skip operation
	if s.db == nil {
		return nil
	}

	// Check cache size limits
	if err := s.enforceCacheSizeLimit(ctx); err != nil {
		s.logger.Warn("Failed to enforce cache size limit", zap.Error(err))
		// Continue anyway, don't fail the Set operation
	}

	s.logger.Debug("Setting cache entry",
		zap.String("key", key),
		zap.Duration("ttl", ttl))

	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	expiresAt := time.Now().Add(ttl)

	query := `
		INSERT OR REPLACE INTO cache_entries (cache_key, value, expires_at, created_at, updated_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	_, err = s.db.ExecContext(ctx, query, key, string(valueJSON), expiresAt)
	if err != nil {
		s.logger.Error("Failed to set cache entry", zap.Error(err))
		return fmt.Errorf("failed to set cache entry: %w", err)
	}

	s.recordCacheActivity(ctx, "SET", key, "", true)
	return nil
}

func (s *CacheService) Get(ctx context.Context, key string, dest interface{}) (bool, error) {
	// If no database is available (e.g., in tests), return not found
	if s.db == nil {
		return false, nil
	}

	s.logger.Debug("Getting cache entry", zap.String("key", key))

	query := `
		SELECT value, expires_at
		FROM cache_entries
		WHERE cache_key = ? AND expires_at > CURRENT_TIMESTAMP
	`

	var valueJSON string
	var expiresAt time.Time

	err := s.db.QueryRowContext(ctx, query, key).Scan(&valueJSON, &expiresAt)
	if err == sql.ErrNoRows {
		s.recordCacheActivity(ctx, "GET", key, "", false)
		return false, nil
	}
	if err != nil {
		s.logger.Error("Failed to get cache entry", zap.Error(err))
		s.recordCacheActivity(ctx, "GET", key, "", false)
		return false, fmt.Errorf("failed to get cache entry: %w", err)
	}

	if err := json.Unmarshal([]byte(valueJSON), dest); err != nil {
		s.logger.Error("Failed to unmarshal cache value", zap.Error(err))
		s.recordCacheActivity(ctx, "GET", key, "", false)
		return false, fmt.Errorf("failed to unmarshal cache value: %w", err)
	}

	s.recordCacheActivity(ctx, "GET", key, "", true)
	return true, nil
}

// enforceCacheSizeLimit removes oldest entries if cache exceeds limits
func (s *CacheService) enforceCacheSizeLimit(ctx context.Context) error {
	if s.db == nil {
		return nil
	}

	// Check total entries count
	var totalCount int64
	countQuery := `SELECT COUNT(*) FROM cache_entries`
	if err := s.db.QueryRowContext(ctx, countQuery).Scan(&totalCount); err != nil {
		return fmt.Errorf("failed to count cache entries: %w", err)
	}

	if totalCount >= MaxCacheEntriesTotal {
		// Remove oldest 10% of entries
		entriesToRemove := int64(float64(MaxCacheEntriesTotal) * 0.1)
		deleteQuery := `
			DELETE FROM cache_entries 
			WHERE id IN (
				SELECT id FROM cache_entries 
				ORDER BY created_at ASC 
				LIMIT ?
			)
		`
		result, err := s.db.ExecContext(ctx, deleteQuery, entriesToRemove)
		if err != nil {
			return fmt.Errorf("failed to remove old cache entries: %w", err)
		}

		rowsAffected, _ := result.RowsAffected()
		s.logger.Warn("Cache size limit reached, removed oldest entries",
			zap.Int64("removed", rowsAffected),
			zap.Int64("total_remaining", totalCount-rowsAffected))
	}

	return nil
}

func (s *CacheService) Delete(ctx context.Context, key string) error {
	// If no database is available (e.g., in tests), skip operation
	if s.db == nil {
		return nil
	}

	s.logger.Debug("Deleting cache entry", zap.String("key", key))

	query := `DELETE FROM cache_entries WHERE cache_key = ?`

	result, err := s.db.ExecContext(ctx, query, key)
	if err != nil {
		s.logger.Error("Failed to delete cache entry", zap.Error(err))
		return fmt.Errorf("failed to delete cache entry: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	s.recordCacheActivity(ctx, "DELETE", key, "", rowsAffected > 0)

	return nil
}

func (s *CacheService) Clear(ctx context.Context, pattern string) error {
	// If no database is available (e.g., in tests), skip operation
	if s.db == nil {
		return nil
	}

	s.logger.Info("Clearing cache entries", zap.String("pattern", pattern))

	var query string
	var args []interface{}

	if pattern == "" {
		query = `DELETE FROM cache_entries`
	} else {
		query = `DELETE FROM cache_entries WHERE cache_key LIKE ?`
		args = append(args, pattern)
	}

	result, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		s.logger.Error("Failed to clear cache entries", zap.Error(err))
		return fmt.Errorf("failed to clear cache entries: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	s.logger.Info("Cleared cache entries", zap.Int64("count", rowsAffected))

	return nil
}

func (s *CacheService) SetMediaMetadata(ctx context.Context, mediaItemID int64, metadataType, provider string, data interface{}, quality float64) error {
	// If no database is available (e.g., in tests), skip operation
	if s.db == nil {
		return nil
	}

	s.logger.Debug("Setting media metadata cache",
		zap.Int64("media_item_id", mediaItemID),
		zap.String("type", metadataType),
		zap.String("provider", provider))

	dataJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	expiresAt := time.Now().Add(MetadataCacheTTL)

	query := `
		INSERT OR REPLACE INTO media_metadata_cache (
			media_item_id, metadata_type, provider, data, quality, expires_at, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	_, err = s.db.ExecContext(ctx, query, mediaItemID, metadataType, provider, string(dataJSON), quality, expiresAt)
	if err != nil {
		s.logger.Error("Failed to set media metadata cache", zap.Error(err))
		return fmt.Errorf("failed to set media metadata: %w", err)
	}

	cacheKey := fmt.Sprintf("metadata:%d:%s:%s", mediaItemID, metadataType, provider)
	s.recordCacheActivity(ctx, "SET", cacheKey, provider, true)

	return nil
}

func (s *CacheService) GetMediaMetadata(ctx context.Context, mediaItemID int64, metadataType, provider string, dest interface{}) (bool, float64, error) {
	// If no database is available (e.g., in tests), return not found
	if s.db == nil {
		return false, 0, nil
	}

	s.logger.Debug("Getting media metadata cache",
		zap.Int64("media_item_id", mediaItemID),
		zap.String("type", metadataType),
		zap.String("provider", provider))

	query := `
		SELECT data, quality, expires_at
		FROM media_metadata_cache
		WHERE media_item_id = ? AND metadata_type = ? AND provider = ? AND expires_at > CURRENT_TIMESTAMP
		ORDER BY quality DESC, updated_at DESC
		LIMIT 1
	`

	var dataJSON string
	var quality float64
	var expiresAt time.Time

	err := s.db.QueryRowContext(ctx, query, mediaItemID, metadataType, provider).Scan(&dataJSON, &quality, &expiresAt)
	cacheKey := fmt.Sprintf("metadata:%d:%s:%s", mediaItemID, metadataType, provider)

	if err == sql.ErrNoRows {
		s.recordCacheActivity(ctx, "GET", cacheKey, provider, false)
		return false, 0, nil
	}
	if err != nil {
		s.logger.Error("Failed to get media metadata cache", zap.Error(err))
		s.recordCacheActivity(ctx, "GET", cacheKey, provider, false)
		return false, 0, fmt.Errorf("failed to get media metadata: %w", err)
	}

	if err := json.Unmarshal([]byte(dataJSON), dest); err != nil {
		s.logger.Error("Failed to unmarshal metadata", zap.Error(err))
		s.recordCacheActivity(ctx, "GET", cacheKey, provider, false)
		return false, 0, fmt.Errorf("failed to unmarshal metadata: %w", err)
	}

	s.recordCacheActivity(ctx, "GET", cacheKey, provider, true)
	return true, quality, nil
}

func (s *CacheService) SetAPIResponse(ctx context.Context, provider, endpoint string, requestData interface{}, response interface{}, statusCode int, ttl time.Duration) error {
	// If no database is available (e.g., in tests), skip operation
	if s.db == nil {
		return nil
	}

	s.logger.Debug("Setting API response cache",
		zap.String("provider", provider),
		zap.String("endpoint", endpoint))

	requestHash, err := s.hashRequest(requestData)
	if err != nil {
		return fmt.Errorf("failed to hash request: %w", err)
	}

	responseJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	expiresAt := time.Now().Add(ttl)

	query := `
		INSERT OR REPLACE INTO api_cache (provider, endpoint, request_hash, response, status_code, expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	_, err = s.db.ExecContext(ctx, query, provider, endpoint, requestHash, string(responseJSON), statusCode, expiresAt)
	if err != nil {
		s.logger.Error("Failed to set API response cache", zap.Error(err))
		return fmt.Errorf("failed to set API response: %w", err)
	}

	cacheKey := fmt.Sprintf("api:%s:%s:%s", provider, endpoint, requestHash)
	s.recordCacheActivity(ctx, "SET", cacheKey, provider, true)

	return nil
}

func (s *CacheService) GetAPIResponse(ctx context.Context, provider, endpoint string, requestData interface{}, dest interface{}) (bool, int, error) {
	// If no database is available (e.g., in tests), return not found
	if s.db == nil {
		return false, 0, nil
	}

	s.logger.Debug("Getting API response cache",
		zap.String("provider", provider),
		zap.String("endpoint", endpoint))

	requestHash, err := s.hashRequest(requestData)
	if err != nil {
		return false, 0, fmt.Errorf("failed to hash request: %w", err)
	}

	query := `
		SELECT response, status_code, expires_at
		FROM api_cache
		WHERE provider = ? AND endpoint = ? AND request_hash = ? AND expires_at > CURRENT_TIMESTAMP
	`

	var responseJSON string
	var statusCode int
	var expiresAt time.Time

	err = s.db.QueryRowContext(ctx, query, provider, endpoint, requestHash).Scan(&responseJSON, &statusCode, &expiresAt)
	cacheKey := fmt.Sprintf("api:%s:%s:%s", provider, endpoint, requestHash)

	if err == sql.ErrNoRows {
		s.recordCacheActivity(ctx, "GET", cacheKey, provider, false)
		return false, 0, nil
	}
	if err != nil {
		s.logger.Error("Failed to get API response cache", zap.Error(err))
		s.recordCacheActivity(ctx, "GET", cacheKey, provider, false)
		return false, 0, fmt.Errorf("failed to get API response: %w", err)
	}

	if err := json.Unmarshal([]byte(responseJSON), dest); err != nil {
		s.logger.Error("Failed to unmarshal API response", zap.Error(err))
		s.recordCacheActivity(ctx, "GET", cacheKey, provider, false)
		return false, statusCode, fmt.Errorf("failed to unmarshal API response: %w", err)
	}

	s.recordCacheActivity(ctx, "GET", cacheKey, provider, true)
	return true, statusCode, nil
}

func (s *CacheService) SetThumbnail(ctx context.Context, videoID, position int64, url string, width, height int, fileSize int64) error {
	// If no database is available (e.g., in tests), skip operation
	if s.db == nil {
		return nil
	}

	s.logger.Debug("Setting thumbnail cache",
		zap.Int64("video_id", videoID),
		zap.Int64("position", position))

	query := s.db.Dialect().UpsertSQL("thumbnail_cache",
		[]string{"video_id", "position", "url", "width", "height", "file_size", "created_at"},
		[]string{"video_id", "position", "width", "height"})

	_, err := s.db.ExecContext(ctx, query, videoID, position, url, width, height, fileSize, time.Now().UTC())
	if err != nil {
		s.logger.Error("Failed to set thumbnail cache", zap.Error(err))
		return fmt.Errorf("failed to set thumbnail: %w", err)
	}

	cacheKey := fmt.Sprintf("thumbnail:%d:%d:%dx%d", videoID, position, width, height)
	s.recordCacheActivity(ctx, "SET", cacheKey, "thumbnail", true)

	return nil
}

func (s *CacheService) GetThumbnail(ctx context.Context, videoID, position int64, width, height int) (*ThumbnailCache, error) {
	// If no database is available (e.g., in tests), return nil
	if s.db == nil {
		return nil, nil
	}

	s.logger.Debug("Getting thumbnail cache",
		zap.Int64("video_id", videoID),
		zap.Int64("position", position))

	query := `
		SELECT id, video_id, position, url, width, height, file_size, created_at
		FROM thumbnail_cache
		WHERE video_id = ? AND position = ? AND width = ? AND height = ?
		ORDER BY created_at DESC
		LIMIT 1
	`

	var thumbnail ThumbnailCache
	err := s.db.QueryRowContext(ctx, query, videoID, position, width, height).Scan(
		&thumbnail.ID, &thumbnail.VideoID, &thumbnail.Position, &thumbnail.URL,
		&thumbnail.Width, &thumbnail.Height, &thumbnail.FileSize, &thumbnail.CreatedAt,
	)

	cacheKey := fmt.Sprintf("thumbnail:%d:%d:%dx%d", videoID, position, width, height)

	if err == sql.ErrNoRows {
		s.recordCacheActivity(ctx, "GET", cacheKey, "thumbnail", false)
		return nil, nil
	}
	if err != nil {
		s.logger.Error("Failed to get thumbnail cache", zap.Error(err))
		s.recordCacheActivity(ctx, "GET", cacheKey, "thumbnail", false)
		return nil, fmt.Errorf("failed to get thumbnail: %w", err)
	}

	s.recordCacheActivity(ctx, "GET", cacheKey, "thumbnail", true)
	return &thumbnail, nil
}

func (s *CacheService) SetTranslation(ctx context.Context, sourceText, sourceLang, targetLang, provider string, translation string) error {
	s.logger.Debug("Setting translation cache",
		zap.String("source_lang", sourceLang),
		zap.String("target_lang", targetLang),
		zap.String("provider", provider))

	key := fmt.Sprintf("translation:%s:%s:%s:%s", provider, sourceLang, targetLang, s.hashString(sourceText))

	translationData := map[string]interface{}{
		"source_text": sourceText,
		"source_lang": sourceLang,
		"target_lang": targetLang,
		"translation": translation,
		"provider":    provider,
		"cached_at":   time.Now(),
	}

	return s.Set(ctx, key, translationData, TranslationCacheTTL)
}

func (s *CacheService) GetTranslation(ctx context.Context, sourceText, sourceLang, targetLang, provider string) (string, bool, error) {
	s.logger.Debug("Getting translation cache",
		zap.String("source_lang", sourceLang),
		zap.String("target_lang", targetLang),
		zap.String("provider", provider))

	key := fmt.Sprintf("translation:%s:%s:%s:%s", provider, sourceLang, targetLang, s.hashString(sourceText))

	var translationData map[string]interface{}
	found, err := s.Get(ctx, key, &translationData)
	if err != nil || !found {
		return "", false, err
	}

	if translation, ok := translationData["translation"].(string); ok {
		return translation, true, nil
	}

	return "", false, fmt.Errorf("invalid translation data in cache")
}

func (s *CacheService) SetSubtitle(ctx context.Context, videoID int64, language, provider string, subtitle *SubtitleTrack) error {
	s.logger.Debug("Setting subtitle cache",
		zap.Int64("video_id", videoID),
		zap.String("language", language),
		zap.String("provider", provider))

	key := fmt.Sprintf("subtitle:%d:%s:%s", videoID, language, provider)
	return s.Set(ctx, key, subtitle, SubtitleCacheTTL)
}

func (s *CacheService) GetSubtitle(ctx context.Context, videoID int64, language, provider string) (*SubtitleTrack, bool, error) {
	s.logger.Debug("Getting subtitle cache",
		zap.Int64("video_id", videoID),
		zap.String("language", language),
		zap.String("provider", provider))

	key := fmt.Sprintf("subtitle:%d:%s:%s", videoID, language, provider)

	var subtitle SubtitleTrack
	found, err := s.Get(ctx, key, &subtitle)
	if err != nil || !found {
		return nil, false, err
	}

	return &subtitle, true, nil
}

func (s *CacheService) SetLyrics(ctx context.Context, artist, title, provider string, lyrics *LyricsData) error {
	s.logger.Debug("Setting lyrics cache",
		zap.String("artist", artist),
		zap.String("title", title),
		zap.String("provider", provider))

	key := fmt.Sprintf("lyrics:%s:%s:%s", provider, s.hashString(artist), s.hashString(title))
	return s.Set(ctx, key, lyrics, LyricsCacheTTL)
}

func (s *CacheService) GetLyrics(ctx context.Context, artist, title, provider string) (*LyricsData, bool, error) {
	s.logger.Debug("Getting lyrics cache",
		zap.String("artist", artist),
		zap.String("title", title),
		zap.String("provider", provider))

	key := fmt.Sprintf("lyrics:%s:%s:%s", provider, s.hashString(artist), s.hashString(title))

	var lyrics LyricsData
	found, err := s.Get(ctx, key, &lyrics)
	if err != nil || !found {
		return nil, false, err
	}

	return &lyrics, true, nil
}

func (s *CacheService) SetCoverArt(ctx context.Context, artist, album, provider string, coverArt *CoverArt) error {
	s.logger.Debug("Setting cover art cache",
		zap.String("artist", artist),
		zap.String("album", album),
		zap.String("provider", provider))

	key := fmt.Sprintf("coverart:%s:%s:%s", provider, s.hashString(artist), s.hashString(album))
	return s.Set(ctx, key, coverArt, CoverArtCacheTTL)
}

func (s *CacheService) GetCoverArt(ctx context.Context, artist, album, provider string) (*CoverArt, bool, error) {
	s.logger.Debug("Getting cover art cache",
		zap.String("artist", artist),
		zap.String("album", album),
		zap.String("provider", provider))

	key := fmt.Sprintf("coverart:%s:%s:%s", provider, s.hashString(artist), s.hashString(album))

	var coverArt CoverArt
	found, err := s.Get(ctx, key, &coverArt)
	if err != nil || !found {
		return nil, false, err
	}

	return &coverArt, true, nil
}

func (s *CacheService) GetStats(ctx context.Context) (*CacheStats, error) {
	s.logger.Debug("Getting cache statistics")

	stats := &CacheStats{
		CachesByType:     make(map[string]int64),
		CachesByProvider: make(map[string]int64),
	}

	// If no database is available (e.g., in tests), return empty stats
	if s.db == nil {
		return stats, nil
	}

	if err := s.getBasicStats(ctx, stats); err != nil {
		return nil, err
	}

	if err := s.getCachesByType(ctx, stats); err != nil {
		s.logger.Warn("Failed to get caches by type", zap.Error(err))
	}

	if err := s.getCachesByProvider(ctx, stats); err != nil {
		s.logger.Warn("Failed to get caches by provider", zap.Error(err))
	}

	if err := s.getRecentActivity(ctx, stats); err != nil {
		s.logger.Warn("Failed to get recent activity", zap.Error(err))
	}

	if err := s.calculateHitRate(ctx, stats); err != nil {
		s.logger.Warn("Failed to calculate hit rate", zap.Error(err))
	}

	return stats, nil
}

func (s *CacheService) CleanupExpired(ctx context.Context) error {
	// If no database is available (e.g., in tests), skip operation
	if s.db == nil {
		return nil
	}

	s.logger.Info("Cleaning up expired cache entries")

	tables := []string{
		"cache_entries",
		"media_metadata_cache",
		"api_cache",
	}

	totalCleaned := int64(0)

	for _, table := range tables {
		// Safe: table names are hardcoded string constants defined in the tables slice above, not user input
		query := fmt.Sprintf("DELETE FROM %s WHERE expires_at <= CURRENT_TIMESTAMP", table)
		result, err := s.db.ExecContext(ctx, query)
		if err != nil {
			s.logger.Error("Failed to cleanup expired entries",
				zap.String("table", table),
				zap.Error(err))
			continue
		}

		rowsAffected, _ := result.RowsAffected()
		totalCleaned += rowsAffected

		s.logger.Debug("Cleaned expired entries",
			zap.String("table", table),
			zap.Int64("count", rowsAffected))
	}

	s.logger.Info("Completed cache cleanup", zap.Int64("total_cleaned", totalCleaned))
	return nil
}

func (s *CacheService) hashRequest(data interface{}) (string, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return "", err
	}
	return s.hashString(string(jsonData)), nil
}

func (s *CacheService) hashString(text string) string {
	hash := sha256.Sum256([]byte(text))
	return hex.EncodeToString(hash[:])
}

func (s *CacheService) recordCacheActivity(ctx context.Context, activityType, key, provider string, hit bool) {
	// Atomically check shutdown and register goroutine to prevent race with Close()
	s.closeMu.Lock()
	select {
	case <-s.shutdown:
		s.closeMu.Unlock()
		return
	default:
	}
	s.wg.Add(1)
	s.closeMu.Unlock()

	go func() {
		defer s.wg.Done()
		writeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		query := `
			INSERT INTO cache_activity (type, cache_key, provider, hit, timestamp)
			VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		`
		_, err := s.db.ExecContext(writeCtx, query, activityType, key, provider, hit)
		if err != nil {
			s.logger.Debug("Failed to record cache activity", zap.Error(err))
		}
	}()
}

func (s *CacheService) getBasicStats(ctx context.Context, stats *CacheStats) error {
	query := `
		SELECT
			COUNT(*) as total_entries,
			COALESCE(SUM(LENGTH(value)), 0) as total_size,
			COUNT(CASE WHEN expires_at <= CURRENT_TIMESTAMP THEN 1 END) as expired_entries
		FROM cache_entries
	`

	return s.db.QueryRowContext(ctx, query).Scan(
		&stats.TotalEntries, &stats.TotalSize, &stats.ExpiredEntries)
}

func (s *CacheService) getCachesByType(ctx context.Context, stats *CacheStats) error {
	query := `
		SELECT
			CASE
				WHEN cache_key LIKE 'translation:%' THEN 'translation'
				WHEN cache_key LIKE 'subtitle:%' THEN 'subtitle'
				WHEN cache_key LIKE 'lyrics:%' THEN 'lyrics'
				WHEN cache_key LIKE 'coverart:%' THEN 'coverart'
				WHEN cache_key LIKE 'api:%' THEN 'api'
				WHEN cache_key LIKE 'metadata:%' THEN 'metadata'
				WHEN cache_key LIKE 'thumbnail:%' THEN 'thumbnail'
				ELSE 'other'
			END as cache_type,
			COUNT(*) as count
		FROM cache_entries
		WHERE expires_at > CURRENT_TIMESTAMP
		GROUP BY cache_type
		ORDER BY count DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cacheType string
		var count int64
		if err := rows.Scan(&cacheType, &count); err == nil {
			stats.CachesByType[cacheType] = count
		}
	}

	return nil
}

func (s *CacheService) getCachesByProvider(ctx context.Context, stats *CacheStats) error {
	query := `
		SELECT provider, COUNT(*) as count
		FROM api_cache
		WHERE expires_at > CURRENT_TIMESTAMP
		GROUP BY provider
		ORDER BY count DESC
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var provider string
		var count int64
		if err := rows.Scan(&provider, &count); err == nil {
			stats.CachesByProvider[provider] = count
		}
	}

	return nil
}

func (s *CacheService) getRecentActivity(ctx context.Context, stats *CacheStats) error {
	cutoff := time.Now().Add(-1 * time.Hour)
	query := `
		SELECT type, cache_key, provider, hit, timestamp
		FROM cache_activity
		WHERE timestamp > ?
		ORDER BY timestamp DESC
		LIMIT 100
	`

	rows, err := s.db.QueryContext(ctx, query, cutoff)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var activity CacheActivity
		if err := rows.Scan(&activity.Type, &activity.Key, &activity.Provider, &activity.Hit, &activity.Timestamp); err == nil {
			stats.RecentActivity = append(stats.RecentActivity, activity)
		}
	}

	return nil
}

func (s *CacheService) calculateHitRate(ctx context.Context, stats *CacheStats) error {
	cutoff24h := time.Now().Add(-24 * time.Hour)
	query := `
		SELECT
			COUNT(CASE WHEN hit = true THEN 1 END) as hits,
			COUNT(CASE WHEN hit = false THEN 1 END) as misses,
			COUNT(*) as total
		FROM cache_activity
		WHERE timestamp > ?
	`

	var hits, misses, total int64
	err := s.db.QueryRowContext(ctx, query, cutoff24h).Scan(&hits, &misses, &total)
	if err != nil {
		return err
	}

	if total > 0 {
		stats.HitRate = float64(hits) / float64(total) * 100
		stats.MissRate = float64(misses) / float64(total) * 100
	}

	return nil
}

// Warmup pre-populates hot entries into the cache. It runs the top-N most
// recent cache_activity entries with hit=true through the existing Get()
// path, which re-reads their underlying cache_entries rows and records a
// cache-activity event — effectively paging cold cache rows into the DB
// buffer pool after a restart.
//
// Safe to call with a nil db (no-op, returns nil). Safe to call
// concurrently — each warmup runs in its own transaction via Get().
func (s *CacheService) Warmup(ctx context.Context) error {
	if s.db == nil {
		s.logger.Info("Cache warmup skipped: no database configured")
		return nil
	}

	s.logger.Info("Starting cache warmup")
	start := time.Now()

	// Pull up to 500 distinct keys from the recent hit history. Keys that
	// appear multiple times are collapsed via SELECT DISTINCT so we don't
	// warm the same entry twice.
	const warmupLimit = 500
	rows, err := s.db.QueryContext(ctx,
		`SELECT DISTINCT key FROM cache_activity WHERE hit = true ORDER BY timestamp DESC LIMIT ?`,
		warmupLimit,
	)
	if err != nil {
		s.logger.Warn("Cache warmup query failed", zap.Error(err))
		return nil // degrade gracefully
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			continue
		}
		if k != "" {
			keys = append(keys, k)
		}
	}
	if err := rows.Err(); err != nil {
		s.logger.Warn("Cache warmup row scan error", zap.Error(err))
	}

	warmed := 0
	for _, k := range keys {
		// Discard the unmarshaled value — we only care that the row is
		// pulled into the DB buffer pool.
		var dest json.RawMessage
		if found, err := s.Get(ctx, k, &dest); err == nil && found {
			warmed++
		}
	}

	s.logger.Info("Cache warmup complete",
		zap.Int("candidates", len(keys)),
		zap.Int("warmed", warmed),
		zap.Duration("elapsed", time.Since(start)))
	return nil
}

func (s *CacheService) InvalidateByPattern(ctx context.Context, pattern string) error {
	// If no database is available (e.g., in tests), skip operation
	if s.db == nil {
		return nil
	}

	s.logger.Info("Invalidating cache entries by pattern", zap.String("pattern", pattern))

	query := `DELETE FROM cache_entries WHERE cache_key LIKE ?`
	result, err := s.db.ExecContext(ctx, query, pattern)
	if err != nil {
		return fmt.Errorf("failed to invalidate cache entries: %w", err)
	}

	rowsAffected, _ := result.RowsAffected()
	s.logger.Info("Invalidated cache entries", zap.Int64("count", rowsAffected))

	return nil
}
