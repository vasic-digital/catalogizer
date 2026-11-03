package services

import (
	"context"
	"database/sql"
	"image"
	"testing"
	"time"

	"catalogizer/database"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newCoverArtTestDB creates an in-memory SQLite database with cover_art tables.
func newCoverArtTestDB(t *testing.T) (*database.DB, func()) {
	t.Helper()
	rawDB, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)

	db := database.WrapDB(rawDB, database.DialectSQLite)
	require.NotNil(t, db)

	schema := `
	CREATE TABLE IF NOT EXISTS cover_art (
		id TEXT PRIMARY KEY,
		media_item_id INTEGER NOT NULL,
		source TEXT NOT NULL,
		url TEXT,
		local_path TEXT,
		width INTEGER,
		height INTEGER,
		format TEXT NOT NULL DEFAULT 'jpeg',
		size INTEGER,
		quality TEXT NOT NULL DEFAULT 'original',
		is_default INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		cached_at DATETIME
	);

	CREATE TABLE IF NOT EXISTS cover_art_cache (
		id TEXT PRIMARY KEY,
		cache_key TEXT NOT NULL,
		provider TEXT NOT NULL,
		title TEXT NOT NULL,
		artist TEXT NOT NULL,
		album TEXT,
		url TEXT NOT NULL,
		thumbnail_url TEXT,
		width INTEGER DEFAULT 0,
		height INTEGER DEFAULT 0,
		format TEXT NOT NULL DEFAULT 'jpeg',
		quality TEXT NOT NULL DEFAULT 'high',
		size INTEGER,
		match_score REAL DEFAULT 0.0,
		source TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_cover_art_media_item ON cover_art(media_item_id);
	CREATE INDEX IF NOT EXISTS idx_cover_art_cache_key ON cover_art_cache(cache_key);
	`
	_, err = rawDB.Exec(schema)
	require.NoError(t, err)

	cleanup := func() {
		rawDB.Close()
	}

	return db, cleanup
}

// ---------------------------------------------------------------------------
// GetCoverArt — with pre-cached cover art record in DB
// ---------------------------------------------------------------------------

func TestCoverArtService_GetCoverArt_Found(t *testing.T) {
	db, cleanup := newCoverArtTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	// Insert a default cover art record
	coverURL := "https://example.com/cover.jpg"
	localPath := "/cache/cover_art/test.jpg"
	_, err := db.ExecContext(ctx,
		`INSERT INTO cover_art (id, media_item_id, source, url, local_path, width, height, format, size, quality, is_default, created_at, cached_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"cover_001", 42, "musicbrainz", coverURL, localPath,
		500, 500, "jpeg", 102400, "high", 1,
		time.Now(), time.Now())
	require.NoError(t, err)

	// Retrieve
	art, err := service.GetCoverArt(ctx, 42)
	require.NoError(t, err)
	require.NotNil(t, art)
	assert.Equal(t, "cover_001", art.ID)
	assert.Equal(t, int64(42), art.MediaItemID)
	assert.Equal(t, "musicbrainz", art.Source)
	require.NotNil(t, art.URL)
	assert.Equal(t, coverURL, *art.URL)
	require.NotNil(t, art.LocalPath)
	assert.Equal(t, localPath, *art.LocalPath)
	assert.Equal(t, "jpeg", art.Format)
	require.NotNil(t, art.Size)
	assert.Equal(t, int64(102400), *art.Size)
	assert.NotNil(t, art.CachedAt)
}

func TestCoverArtService_GetCoverArt_NotFound(t *testing.T) {
	db, cleanup := newCoverArtTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	art, err := service.GetCoverArt(ctx, 9999)
	assert.NoError(t, err) // Returns nil, nil for no rows
	assert.Nil(t, art)
}

func TestCoverArtService_GetCoverArt_NullableFields(t *testing.T) {
	db, cleanup := newCoverArtTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	// Insert cover art with NULL url, local_path, size, cached_at
	_, err := db.ExecContext(ctx,
		`INSERT INTO cover_art (id, media_item_id, source, url, local_path, width, height, format, size, quality, is_default, created_at, cached_at)
		 VALUES (?, ?, ?, NULL, NULL, ?, ?, ?, NULL, ?, ?, ?, NULL)`,
		"cover_null", 100, "embedded",
		300, 300, "png", "medium", 1, time.Now())
	require.NoError(t, err)

	art, err := service.GetCoverArt(ctx, 100)
	require.NoError(t, err)
	require.NotNil(t, art)
	assert.Equal(t, "cover_null", art.ID)
	assert.Nil(t, art.URL)
	assert.Nil(t, art.LocalPath)
	assert.Nil(t, art.Size)
	assert.Nil(t, art.CachedAt)
}

// ---------------------------------------------------------------------------
// GetCoverArt — only returns default cover art
// ---------------------------------------------------------------------------

func TestCoverArtService_GetCoverArt_OnlyReturnsDefault(t *testing.T) {
	db, cleanup := newCoverArtTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	// Insert non-default cover art
	_, err := db.ExecContext(ctx,
		`INSERT INTO cover_art (id, media_item_id, source, width, height, format, quality, is_default, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"cover_nodef", 50, "local", 200, 200, "jpeg", "medium", 0, time.Now())
	require.NoError(t, err)

	// Should return nil (no default found)
	art, err := service.GetCoverArt(ctx, 50)
	assert.NoError(t, err)
	assert.Nil(t, art)
}

// ---------------------------------------------------------------------------
// SearchCoverArt — with cached results
// ---------------------------------------------------------------------------

func TestCoverArtService_SearchCoverArt_WithCachedResult(t *testing.T) {
	db, cleanup := newCoverArtTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	// Generate a cache key for the request
	request := &CoverArtSearchRequest{
		Title:   "Test Song",
		Artist:  "Test Artist",
		Quality: QualityHigh,
	}
	cacheKey := service.generateCacheKey(request)

	// Insert cached result
	album := "Test Album"
	_, err := db.ExecContext(ctx,
		`INSERT INTO cover_art_cache (id, cache_key, provider, title, artist, album, url, thumbnail_url, width, height, format, quality, size, match_score, source, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"cached_001", cacheKey, "musicbrainz", "Test Song", "Test Artist",
		album, "https://example.com/cached.jpg", "https://example.com/thumb.jpg",
		600, 600, "jpeg", "high", 204800, 0.95, "coverartarchive.org",
		time.Now()) // Fresh cache entry
	require.NoError(t, err)

	// Search with cache
	request.UseCache = true
	results, err := service.SearchCoverArt(ctx, request)
	require.NoError(t, err)
	assert.Equal(t, 1, len(results))
	assert.Equal(t, "cached_001", results[0].ID)
	assert.Equal(t, CoverArtProviderMusicBrainz, results[0].Provider)
	assert.Equal(t, 0.95, results[0].MatchScore)
}

func TestCoverArtService_SearchCoverArt_CacheMiss(t *testing.T) {
	db, cleanup := newCoverArtTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	request := &CoverArtSearchRequest{
		Title:    "Nonexistent Song",
		Artist:   "Unknown Artist",
		Quality:  QualityHigh,
		UseCache: true,
	}

	// Should fall through to providers (mock results)
	results, err := service.SearchCoverArt(ctx, request)
	// CAT-001: no provider client exists, so the search reports failure
	// honestly instead of returning invented results.
	assert.ErrorIs(t, err, ErrCoverArtProviderNotImplemented)
	assert.Empty(t, results)
}

func TestCoverArtService_SearchCoverArt_WithSpecificProviders(t *testing.T) {
	db, cleanup := newCoverArtTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	request := &CoverArtSearchRequest{
		Title:     "Test Title",
		Artist:    "Test Artist",
		Quality:   QualityMedium,
		UseCache:  false,
		Providers: []CoverArtProvider{CoverArtProviderITunes},
	}

	results, err := service.SearchCoverArt(ctx, request)
	// CAT-001: no provider client exists, so the search reports failure
	// honestly instead of returning invented results.
	assert.ErrorIs(t, err, ErrCoverArtProviderNotImplemented)
	assert.Empty(t, results)
}

func TestCoverArtService_SearchCoverArt_AllDefaultProviders(t *testing.T) {
	db, cleanup := newCoverArtTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	request := &CoverArtSearchRequest{
		Title:    "Test",
		Artist:   "Artist",
		Quality:  QualityHigh,
		UseCache: false,
	}

	results, err := service.SearchCoverArt(ctx, request)
	// CAT-001: no provider client exists, so the search reports failure
	// honestly instead of returning invented results.
	assert.ErrorIs(t, err, ErrCoverArtProviderNotImplemented)
	assert.Empty(t, results)
}

// ---------------------------------------------------------------------------
// setDefaultCoverArt
// ---------------------------------------------------------------------------

func TestCoverArtService_SetDefaultCoverArt(t *testing.T) {
	db, cleanup := newCoverArtTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	// Insert two cover arts for same media item
	_, err := db.ExecContext(ctx,
		`INSERT INTO cover_art (id, media_item_id, source, width, height, format, quality, is_default, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"art1", 10, "musicbrainz", 300, 300, "jpeg", "high", 1, time.Now())
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO cover_art (id, media_item_id, source, width, height, format, quality, is_default, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"art2", 10, "lastfm", 500, 500, "jpeg", "high", 0, time.Now())
	require.NoError(t, err)

	// Set art2 as default
	err = service.setDefaultCoverArt(ctx, 10, "art2")
	require.NoError(t, err)

	// Verify art1 is no longer default
	var isDefault1 int
	err = db.QueryRowContext(ctx, "SELECT is_default FROM cover_art WHERE id = ?", "art1").Scan(&isDefault1)
	require.NoError(t, err)
	assert.Equal(t, 0, isDefault1)

	// Verify art2 is now default
	var isDefault2 int
	err = db.QueryRowContext(ctx, "SELECT is_default FROM cover_art WHERE id = ?", "art2").Scan(&isDefault2)
	require.NoError(t, err)
	assert.Equal(t, 1, isDefault2)
}

// ---------------------------------------------------------------------------
// getCoverArtDownloadInfo
// ---------------------------------------------------------------------------

func TestCoverArtService_GetCoverArtDownloadInfo_FromDB(t *testing.T) {
	db, cleanup := newCoverArtTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	// Insert a cover art record
	_, err := db.ExecContext(ctx,
		`INSERT INTO cover_art (id, media_item_id, source, url, width, height, format, quality, is_default, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"dl_001", 5, "itunes", "https://example.com/download.jpg", 600, 600, "jpeg", "high", 0, time.Now())
	require.NoError(t, err)

	result, err := service.getCoverArtDownloadInfo(ctx, "dl_001")
	require.NoError(t, err)
	assert.Equal(t, "dl_001", result.ID)
	assert.Equal(t, "https://example.com/download.jpg", result.URL)
	assert.Equal(t, CoverArtQuality("high"), result.Quality)
}

func TestCoverArtService_GetCoverArtDownloadInfo_NotInDB(t *testing.T) {
	db, cleanup := newCoverArtTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	// Result not in DB — falls back to local provider
	result, err := service.getCoverArtDownloadInfo(ctx, "nonexistent_id")
	require.NoError(t, err)
	assert.Equal(t, "nonexistent_id", result.ID)
	assert.Equal(t, CoverArtProviderLocal, result.Provider)
	assert.Equal(t, QualityHigh, result.Quality)
	assert.Equal(t, "cache", result.Source)
}

// ---------------------------------------------------------------------------
// sortCoverArtResults — more complex sort cases
// ---------------------------------------------------------------------------

func TestCoverArtService_SortCoverArtResults_SameScore(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	results := []CoverArtSearchResult{
		{ID: "a", MatchScore: 0.9, Width: 200},
		{ID: "b", MatchScore: 0.9, Width: 800},
		{ID: "c", MatchScore: 0.9, Width: 500},
	}

	service.sortCoverArtResults(results)

	// Same score: sorted by width descending
	assert.Equal(t, "b", results[0].ID) // 800
	assert.Equal(t, "c", results[1].ID) // 500
	assert.Equal(t, "a", results[2].ID) // 200
}

func TestCoverArtService_SortCoverArtResults_Empty(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	var results []CoverArtSearchResult
	service.sortCoverArtResults(results) // Should not panic
}

func TestCoverArtService_SortCoverArtResults_SingleItem(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	results := []CoverArtSearchResult{{ID: "only", MatchScore: 1.0, Width: 100}}
	service.sortCoverArtResults(results)
	assert.Equal(t, "only", results[0].ID)
}

// ---------------------------------------------------------------------------
// generateTimestamps — edge cases
// ---------------------------------------------------------------------------

func TestCoverArtService_GenerateTimestamps_ZeroCount(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	// count <= 0 defaults to 3
	timestamps := service.generateTimestamps(120.0, 0)
	assert.Equal(t, 3, len(timestamps))
}

func TestCoverArtService_GenerateTimestamps_NegativeCount(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	timestamps := service.generateTimestamps(120.0, -5)
	assert.Equal(t, 3, len(timestamps))
}

func TestCoverArtService_GenerateTimestamps_Values(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	// 120s / (4+1) = 24s interval
	timestamps := service.generateTimestamps(120.0, 4)
	assert.Equal(t, 4, len(timestamps))
	assert.InDelta(t, 24.0, timestamps[0], 0.001)
	assert.InDelta(t, 48.0, timestamps[1], 0.001)
	assert.InDelta(t, 72.0, timestamps[2], 0.001)
	assert.InDelta(t, 96.0, timestamps[3], 0.001)
}

// ---------------------------------------------------------------------------
// resizeImage — aspect ratio calculation
// ---------------------------------------------------------------------------

func TestCoverArtService_ResizeImage(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	// Create a simple test image: 800x600
	img := image.NewRGBA(image.Rect(0, 0, 800, 600))

	// Resize with preserve aspect
	result := service.resizeImage(img, &CoverArtProcessingOptions{
		Width:          400,
		Height:         400,
		PreserveAspect: true,
	})

	bounds := result.Bounds()
	// 800x600 -> aspect 4:3; target 400x400
	// Width/Height ratio: 400/400 = 1.0 > 4/3 ≈ 1.33? No, 1.0 < 1.33.
	// So dstHeight = dstWidth / aspectRatio = 400 / (800/600) = 300
	assert.Equal(t, 400, bounds.Dx())
	assert.Equal(t, 300, bounds.Dy())
}

func TestCoverArtService_ResizeImage_NoPreserve(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	img := image.NewRGBA(image.Rect(0, 0, 800, 600))

	result := service.resizeImage(img, &CoverArtProcessingOptions{
		Width:          300,
		Height:         300,
		PreserveAspect: false,
	})

	bounds := result.Bounds()
	assert.Equal(t, 300, bounds.Dx())
	assert.Equal(t, 300, bounds.Dy())
}

func TestCoverArtService_ResizeImage_TallImage(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	// Tall image: 400 wide, 800 tall
	img := image.NewRGBA(image.Rect(0, 0, 400, 800))

	result := service.resizeImage(img, &CoverArtProcessingOptions{
		Width:          400,
		Height:         400,
		PreserveAspect: true,
	})

	bounds := result.Bounds()
	// 400x800 -> aspect 0.5; target 400x400
	// 400/400 = 1.0 > 0.5 -> dstWidth = dstHeight * aspect = 400 * 0.5 = 200
	assert.Equal(t, 200, bounds.Dx())
	assert.Equal(t, 400, bounds.Dy())
}

// ---------------------------------------------------------------------------
// generateCacheKey — deterministic output
// ---------------------------------------------------------------------------

func TestCoverArtService_GenerateCacheKey_Deterministic(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	req := &CoverArtSearchRequest{
		Title:   "Test",
		Artist:  "Artist",
		Quality: QualityHigh,
	}

	key1 := service.generateCacheKey(req)
	key2 := service.generateCacheKey(req)
	assert.Equal(t, key1, key2)
	assert.Len(t, key1, 64) // SHA-256 hex length
}

func TestCoverArtService_GenerateCacheKey_DifferentInputs(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	key1 := service.generateCacheKey(&CoverArtSearchRequest{Title: "A", Artist: "B", Quality: "high"})
	key2 := service.generateCacheKey(&CoverArtSearchRequest{Title: "C", Artist: "D", Quality: "high"})
	assert.NotEqual(t, key1, key2)
}

func TestCoverArtService_GenerateCacheKey_WithAlbum(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	album := "Best Hits"
	req := &CoverArtSearchRequest{
		Title:   "Song",
		Artist:  "Artist",
		Album:   &album,
		Quality: QualityMedium,
	}

	key := service.generateCacheKey(req)
	assert.NotEmpty(t, key)
	assert.Len(t, key, 64) // SHA-256 hex length

	// Without album should produce different key
	reqNoAlbum := &CoverArtSearchRequest{
		Title:   "Song",
		Artist:  "Artist",
		Quality: QualityMedium,
	}
	keyNoAlbum := service.generateCacheKey(reqNoAlbum)
	assert.NotEqual(t, key, keyNoAlbum)
}

// ---------------------------------------------------------------------------
// searchProvider — unsupported provider
// ---------------------------------------------------------------------------

func TestCoverArtService_SearchProvider_Unsupported(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	ctx := context.Background()
	_, err := service.searchProvider(ctx, CoverArtProvider("unknown"), &CoverArtSearchRequest{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported provider")
}
