package services

import (
	"catalogizer/internal/netguard"
	"context"
	"database/sql"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"catalogizer/database"

	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// newCoverArtFullTestDB creates an in-memory SQLite DB with all needed tables.
func newCoverArtFullTestDB(t *testing.T) (*database.DB, func()) {
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
	CREATE TABLE IF NOT EXISTS external_metadata (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		media_item_id INTEGER NOT NULL,
		provider TEXT NOT NULL,
		cover_url TEXT,
		last_fetched DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_cover_art_media_item ON cover_art(media_item_id);
	CREATE INDEX IF NOT EXISTS idx_cover_art_cache_key ON cover_art_cache(cache_key);
	`
	_, err = rawDB.Exec(schema)
	require.NoError(t, err)

	return db, func() { rawDB.Close() }
}

// =============================================================================
// DownloadCoverArt — tests the download + process pipeline
// =============================================================================

func TestCoverArtService_DownloadCoverArt_Success(t *testing.T) {
	db, cleanup := newCoverArtFullTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)
	// production client is strict (ignores the allow-list); the fetch path is tested through
	// the allow-list-aware seam, strictness in outbound_strict_resolvers_test.go
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	service.httpClient = netguard.Client(5*time.Second, 3, netguard.HTTPOrHTTPS)

	tmpDir := t.TempDir()
	service.cacheDir = tmpDir

	// Create a test JPEG image to serve
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	// Serve the image via HTTP
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		jpeg.Encode(w, img, &jpeg.Options{Quality: 90})
	}))
	defer ts.Close()

	// Insert a cover_art record with the URL from our test server
	ctx := context.Background()
	_, err := db.ExecContext(ctx,
		`INSERT INTO cover_art (id, media_item_id, source, url, width, height, format, quality, is_default, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"dl_test_001", 42, "itunes", ts.URL, 100, 100, "jpeg", "high", 0, time.Now())
	require.NoError(t, err)

	request := &CoverArtDownloadRequest{
		MediaItemID: 42,
		ResultID:    "dl_test_001",
		Quality:     QualityHigh,
	}

	coverArt, err := service.DownloadCoverArt(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, coverArt)
	assert.Equal(t, int64(42), coverArt.MediaItemID)
	assert.NotNil(t, coverArt.LocalPath)
	assert.Equal(t, "jpeg", coverArt.Format)
}

func TestCoverArtService_DownloadCoverArt_DownloadFailure(t *testing.T) {
	db, cleanup := newCoverArtFullTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)
	service.cacheDir = t.TempDir()

	ctx := context.Background()

	// Fallback download info has empty URL which will fail
	request := &CoverArtDownloadRequest{
		MediaItemID: 99,
		ResultID:    "nonexistent",
		Quality:     QualityHigh,
	}

	_, err := service.DownloadCoverArt(ctx, request)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to download image")
}

func TestCoverArtService_DownloadCoverArt_SetAsDefault(t *testing.T) {
	db, cleanup := newCoverArtFullTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)
	// production client is strict (ignores the allow-list); the fetch path is tested through
	// the allow-list-aware seam, strictness in outbound_strict_resolvers_test.go
	t.Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	service.httpClient = netguard.Client(5*time.Second, 3, netguard.HTTPOrHTTPS)
	service.cacheDir = t.TempDir()

	// Serve a valid JPEG
	img := image.NewRGBA(image.Rect(0, 0, 50, 50))
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		jpeg.Encode(w, img, &jpeg.Options{Quality: 80})
	}))
	defer ts.Close()

	ctx := context.Background()
	_, err := db.ExecContext(ctx,
		`INSERT INTO cover_art (id, media_item_id, source, url, width, height, format, quality, is_default, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"dl_default", 55, "musicbrainz", ts.URL, 50, 50, "jpeg", "high", 0, time.Now())
	require.NoError(t, err)

	request := &CoverArtDownloadRequest{
		MediaItemID:  55,
		ResultID:     "dl_default",
		Quality:      QualityHigh,
		SetAsDefault: true,
	}

	coverArt, err := service.DownloadCoverArt(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, coverArt)
}

// =============================================================================
// processAndSaveCoverArt — image decode + save
// =============================================================================

func TestCoverArtService_ProcessAndSaveCoverArt_InvalidImageData(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)
	service.cacheDir = t.TempDir()

	result := &CoverArtSearchResult{
		ID:       "test-result",
		Provider: CoverArtProviderLocal,
	}
	request := &CoverArtDownloadRequest{
		MediaItemID: 1,
		Quality:     QualityHigh,
	}

	_, err := service.processAndSaveCoverArt(context.Background(), 1, []byte("not an image"), result, request)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode image")
}

// =============================================================================
// ScanLocalCoverArt — directory scanning
// =============================================================================

func TestCoverArtService_ScanLocalCoverArt_WithCoverFiles(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	tmpDir := t.TempDir()

	// Create a cover.jpg file (valid JPEG)
	img := image.NewRGBA(image.Rect(0, 0, 200, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 200; x++ {
			img.Set(x, y, color.RGBA{R: 0, G: 128, B: 255, A: 255})
		}
	}

	coverPath := filepath.Join(tmpDir, "cover.jpg")
	f, err := os.Create(coverPath)
	require.NoError(t, err)
	require.NoError(t, jpeg.Encode(f, img, &jpeg.Options{Quality: 90}))
	f.Close()

	request := &LocalCoverArtScanRequest{
		MediaItemID: 10,
		Directory:   tmpDir,
		Recursive:   false,
	}

	results, err := service.ScanLocalCoverArt(context.Background(), request)
	require.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "local", results[0].Source)
	assert.Equal(t, int64(10), results[0].MediaItemID)
}

func TestCoverArtService_ScanLocalCoverArt_Recursive(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subalbum")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	// Create album.jpg in subdirectory
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	albumPath := filepath.Join(subDir, "album.jpg")
	f, err := os.Create(albumPath)
	require.NoError(t, err)
	require.NoError(t, jpeg.Encode(f, img, &jpeg.Options{Quality: 80}))
	f.Close()

	request := &LocalCoverArtScanRequest{
		MediaItemID: 20,
		Directory:   tmpDir,
		Recursive:   true,
	}

	results, err := service.ScanLocalCoverArt(context.Background(), request)
	require.NoError(t, err)
	assert.Len(t, results, 1)
}

func TestCoverArtService_ScanLocalCoverArt_NonImageFile(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	tmpDir := t.TempDir()

	// Create a file named cover.jpg but with invalid image content
	coverPath := filepath.Join(tmpDir, "cover.jpg")
	require.NoError(t, os.WriteFile(coverPath, []byte("not a real image"), 0644))

	request := &LocalCoverArtScanRequest{
		MediaItemID: 30,
		Directory:   tmpDir,
		Recursive:   false,
	}

	// Should not error but should skip the invalid file
	results, err := service.ScanLocalCoverArt(context.Background(), request)
	require.NoError(t, err)
	assert.Empty(t, results)
}

// =============================================================================
// GetCoverURL — URL resolution with priority
// =============================================================================

func TestCoverArtService_GetCoverURL_FromCoverArtTable(t *testing.T) {
	db, cleanup := newCoverArtFullTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	// Insert cover art with local_path
	_, err := db.ExecContext(ctx,
		`INSERT INTO cover_art (id, media_item_id, source, url, local_path, width, height, format, quality, is_default, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"url_test_1", 100, "itunes", "https://example.com/cover.jpg", "/cache/100.jpg", 500, 500, "jpeg", "high", 1, time.Now())
	require.NoError(t, err)

	url := service.GetCoverURL(ctx, 100, "music_album")
	assert.Equal(t, "/api/v1/cover/100", url)
}

func TestCoverArtService_GetCoverURL_FromLocalPath(t *testing.T) {
	db, cleanup := newCoverArtFullTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	// Insert cover art with local path but no URL
	_, err := db.ExecContext(ctx,
		`INSERT INTO cover_art (id, media_item_id, source, url, local_path, width, height, format, quality, is_default, created_at)
		 VALUES (?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?)`,
		"url_test_2", 101, "local", "/cache/cover.jpg", 300, 300, "jpeg", "high", 1, time.Now())
	require.NoError(t, err)

	url := service.GetCoverURL(ctx, 101, "music_album")
	assert.Equal(t, "/api/v1/cover/101", url)
}

func TestCoverArtService_GetCoverURL_FromExternalMetadata(t *testing.T) {
	db, cleanup := newCoverArtFullTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	// No cover_art record, but external_metadata has a cover URL
	_, err := db.ExecContext(ctx,
		`INSERT INTO external_metadata (media_item_id, provider, cover_url, last_fetched)
		 VALUES (?, ?, ?, ?)`,
		102, "tmdb", "https://tmdb.org/poster.jpg", time.Now())
	require.NoError(t, err)

	url := service.GetCoverURL(ctx, 102, "movie")
	assert.Equal(t, "/api/v1/image-proxy?url=https%3A%2F%2Ftmdb.org%2Fposter.jpg", url)
}

func TestCoverArtService_GetCoverURL_Placeholder(t *testing.T) {
	db, cleanup := newCoverArtFullTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	// No cover art at all, should return placeholder
	url := service.GetCoverURL(ctx, 999, "game")
	assert.Equal(t, "/api/v1/cover/placeholder/game", url)
}

func TestCoverArtService_GetCoverURL_NilDB(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	url := service.GetCoverURL(context.Background(), 1, "movie")
	assert.Equal(t, "/api/v1/cover/placeholder/movie", url)
}

// =============================================================================
// GetCoverURLsBatch — batch URL resolution
// =============================================================================

func TestCoverArtService_GetCoverURLsBatch_Empty(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	result := service.GetCoverURLsBatch(context.Background(), []CoverURLRequest{})
	assert.Empty(t, result)
}

func TestCoverArtService_GetCoverURLsBatch_NilDB(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	items := []CoverURLRequest{
		{ID: 1, MediaTypeName: "movie"},
		{ID: 2, MediaTypeName: "song"},
	}

	result := service.GetCoverURLsBatch(context.Background(), items)
	assert.Len(t, result, 2)
	assert.Equal(t, "/api/v1/cover/placeholder/movie", result[1])
	assert.Equal(t, "/api/v1/cover/placeholder/song", result[2])
}

func TestCoverArtService_GetCoverURLsBatch_WithData(t *testing.T) {
	db, cleanup := newCoverArtFullTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	// Item 200: cover_art with local_path (highest priority)
	_, err := db.ExecContext(ctx,
		`INSERT INTO cover_art (id, media_item_id, source, url, local_path, width, height, format, quality, is_default, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"batch_1", 200, "itunes", "https://itunes.com/200.jpg", "/cache/200.jpg", 500, 500, "jpeg", "high", 1, time.Now())
	require.NoError(t, err)

	// Item 201: only external_metadata
	_, err = db.ExecContext(ctx,
		`INSERT INTO external_metadata (media_item_id, provider, cover_url, last_fetched)
		 VALUES (?, ?, ?, ?)`,
		201, "tmdb", "https://tmdb.org/201.jpg", time.Now())
	require.NoError(t, err)

	// Item 202: no cover at all

	items := []CoverURLRequest{
		{ID: 200, MediaTypeName: "music_album"},
		{ID: 201, MediaTypeName: "movie"},
		{ID: 202, MediaTypeName: "game"},
	}

	result := service.GetCoverURLsBatch(ctx, items)
	assert.Len(t, result, 3)
	assert.Equal(t, "/api/v1/cover/200", result[200])
	assert.Equal(t, "/api/v1/image-proxy?url=https%3A%2F%2Ftmdb.org%2F201.jpg", result[201])
	assert.Equal(t, "/api/v1/cover/placeholder/game", result[202])
}

func TestCoverArtService_GetCoverURLsBatch_CoverArtOverridesExternal(t *testing.T) {
	db, cleanup := newCoverArtFullTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	// Both external_metadata and cover_art exist for item 300
	_, err := db.ExecContext(ctx,
		`INSERT INTO external_metadata (media_item_id, provider, cover_url, last_fetched)
		 VALUES (?, ?, ?, ?)`,
		300, "tmdb", "https://tmdb.org/300.jpg", time.Now())
	require.NoError(t, err)

	_, err = db.ExecContext(ctx,
		`INSERT INTO cover_art (id, media_item_id, source, url, local_path, width, height, format, quality, is_default, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"batch_300", 300, "local", "https://local/300.jpg", "/cache/300.jpg", 600, 600, "jpeg", "high", 1, time.Now())
	require.NoError(t, err)

	items := []CoverURLRequest{
		{ID: 300, MediaTypeName: "movie"},
	}

	result := service.GetCoverURLsBatch(ctx, items)
	// cover_art with local_path should override external_metadata
	assert.Equal(t, "/api/v1/cover/300", result[300])
}

func TestCoverArtService_GetCoverURLsBatch_LocalPathFallback(t *testing.T) {
	db, cleanup := newCoverArtFullTestDB(t)
	defer cleanup()

	logger := zap.NewNop()
	service := NewCoverArtService(db, logger)

	ctx := context.Background()

	// cover_art with local_path but no url
	_, err := db.ExecContext(ctx,
		`INSERT INTO cover_art (id, media_item_id, source, url, local_path, width, height, format, quality, is_default, created_at)
		 VALUES (?, ?, ?, NULL, ?, ?, ?, ?, ?, ?, ?)`,
		"batch_lp", 400, "embedded", "/cache/400.jpg", 300, 300, "jpeg", "original", 1, time.Now())
	require.NoError(t, err)

	items := []CoverURLRequest{
		{ID: 400, MediaTypeName: "song"},
	}

	result := service.GetCoverURLsBatch(ctx, items)
	assert.Equal(t, "/api/v1/cover/400", result[400])
}

// =============================================================================
// GeneratePlaceholderSVG — static function
// =============================================================================

func TestGeneratePlaceholderSVG_KnownTypes_Coverage(t *testing.T) {
	types := []string{"movie", "tv_show", "tv_season", "tv_episode",
		"music_artist", "music_album", "song", "game", "software", "book", "comic"}

	for _, mediaType := range types {
		t.Run(mediaType, func(t *testing.T) {
			svg := GeneratePlaceholderSVG(mediaType)
			assert.NotEmpty(t, svg)
			assert.True(t, strings.Contains(string(svg), "<svg"))
			assert.True(t, strings.Contains(string(svg), "</svg>"))
		})
	}
}

func TestGeneratePlaceholderSVG_UnknownType_Coverage(t *testing.T) {
	svg := GeneratePlaceholderSVG("unknown_type")
	assert.NotEmpty(t, svg)
	assert.True(t, strings.Contains(string(svg), "<svg"))
}

// =============================================================================
// processLocalCoverArt — local file processing
// =============================================================================

func TestCoverArtService_ProcessLocalCoverArt_Success(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	tmpDir := t.TempDir()

	// Create a valid JPEG
	img := image.NewRGBA(image.Rect(0, 0, 150, 150))
	filePath := filepath.Join(tmpDir, "test_cover.jpg")
	f, err := os.Create(filePath)
	require.NoError(t, err)
	require.NoError(t, jpeg.Encode(f, img, &jpeg.Options{Quality: 90}))
	f.Close()

	coverArt, err := service.processLocalCoverArt(context.Background(), 77, filePath)
	require.NoError(t, err)
	require.NotNil(t, coverArt)
	assert.Equal(t, int64(77), coverArt.MediaItemID)
	assert.Equal(t, "local", coverArt.Source)
	assert.Equal(t, "jpeg", coverArt.Format)
	assert.NotNil(t, coverArt.Width)
	assert.Equal(t, 150, *coverArt.Width)
	assert.NotNil(t, coverArt.Height)
	assert.Equal(t, 150, *coverArt.Height)
	assert.NotNil(t, coverArt.Size)
	assert.Greater(t, *coverArt.Size, int64(0))
}

func TestCoverArtService_ProcessLocalCoverArt_NonexistentFile(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	_, err := service.processLocalCoverArt(context.Background(), 1, "/nonexistent/cover.jpg")
	assert.Error(t, err)
}

func TestCoverArtService_ProcessLocalCoverArt_InvalidImage(t *testing.T) {
	logger := zap.NewNop()
	service := NewCoverArtService(nil, logger)

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "bad.jpg")
	require.NoError(t, os.WriteFile(filePath, []byte("not an image"), 0644))

	_, err := service.processLocalCoverArt(context.Background(), 1, filePath)
	assert.Error(t, err)
}

// =============================================================================
// CoverURLRequest struct fields
// =============================================================================

func TestCoverURLRequest_Fields(t *testing.T) {
	req := CoverURLRequest{
		ID:            42,
		MediaTypeName: "movie",
	}
	assert.Equal(t, int64(42), req.ID)
	assert.Equal(t, "movie", req.MediaTypeName)
}
