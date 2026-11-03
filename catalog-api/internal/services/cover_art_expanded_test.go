package services

import (
	"catalogizer/database"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ---------------------------------------------------------------------------
// CoverArt type constants
// ---------------------------------------------------------------------------

func TestCoverArtProvider_Constants(t *testing.T) {
	providers := []CoverArtProvider{
		CoverArtProviderMusicBrainz,
		CoverArtProviderLastFM,
		CoverArtProviderSpotify,
		CoverArtProviderDeezer,
		CoverArtProviderITunes,
		CoverArtProviderDiscogs,
		CoverArtProviderEmbedded,
		CoverArtProviderLocal,
	}

	seen := make(map[CoverArtProvider]bool)
	for _, p := range providers {
		assert.NotEmpty(t, string(p), "provider constant should not be empty")
		assert.False(t, seen[p], "duplicate provider constant: %s", p)
		seen[p] = true
	}
	assert.Equal(t, 8, len(providers), "expected 8 CoverArtProvider constants")
}

func TestCoverArtQuality_Constants(t *testing.T) {
	qualities := []CoverArtQuality{
		QualityThumbnail,
		QualityMedium,
		QualityHigh,
		QualityOriginal,
	}

	seen := make(map[CoverArtQuality]bool)
	for _, q := range qualities {
		assert.NotEmpty(t, string(q))
		assert.False(t, seen[q], "duplicate quality constant: %s", q)
		seen[q] = true
	}
	assert.Equal(t, 4, len(qualities))
}

// ---------------------------------------------------------------------------
// SearchCoverArt
// ---------------------------------------------------------------------------

func TestCoverArtService_SearchCoverArt_DefaultProviders(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	request := &CoverArtSearchRequest{
		Title:    "Nevermind",
		Artist:   "Nirvana",
		Quality:  QualityHigh,
		UseCache: false,
	}

	results, err := service.SearchCoverArt(context.Background(), request)
	// CAT-001: no provider client exists, so the search reports failure
	// honestly instead of returning invented results.
	assert.ErrorIs(t, err, ErrCoverArtProviderNotImplemented)
	assert.Empty(t, results)
}

func TestCoverArtService_SearchCoverArt_SpecificProviders(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	request := &CoverArtSearchRequest{
		Title:     "Test Album",
		Artist:    "Test Artist",
		Quality:   QualityMedium,
		Providers: []CoverArtProvider{CoverArtProviderITunes},
		UseCache:  false,
	}

	results, err := service.SearchCoverArt(context.Background(), request)
	// CAT-001: no provider client exists, so the search reports failure
	// honestly instead of returning invented results.
	assert.ErrorIs(t, err, ErrCoverArtProviderNotImplemented)
	assert.Empty(t, results)
}

func TestCoverArtService_SearchCoverArt_UnsupportedProvider(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	request := &CoverArtSearchRequest{
		Title:     "Test",
		Artist:    "Test",
		Quality:   QualityHigh,
		Providers: []CoverArtProvider{CoverArtProvider("unsupported")},
		UseCache:  false,
	}

	// Unsupported providers are logged and skipped, not errors
	results, err := service.SearchCoverArt(context.Background(), request)
	assert.Error(t, err) // the only provider failed, so nothing was searched
	assert.Empty(t, results)
}

func TestCoverArtService_SearchCoverArt_WithAlbum(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	album := "Nevermind"
	request := &CoverArtSearchRequest{
		Title:     "Smells Like Teen Spirit",
		Artist:    "Nirvana",
		Album:     &album,
		Quality:   QualityHigh,
		Providers: []CoverArtProvider{CoverArtProviderMusicBrainz},
		UseCache:  false,
	}

	results, err := service.SearchCoverArt(context.Background(), request)
	// CAT-001: no provider client exists, so the search reports failure
	// honestly instead of returning invented results.
	assert.ErrorIs(t, err, ErrCoverArtProviderNotImplemented)
	assert.Empty(t, results)
}

// ---------------------------------------------------------------------------
// searchProvider — individual providers
// ---------------------------------------------------------------------------

// CAT-001: no cover-art provider client exists; results must not be invented.
func TestCoverArtService_SearchMusicBrainz_NotFabricated(t *testing.T) {
	service := NewCoverArtService(database.WrapDB(nil, database.DialectSQLite), zap.NewNop())
	results, err := service.searchMusicBrainz(context.Background(), &CoverArtSearchRequest{Title: "T", Artist: "A", Quality: QualityHigh})
	assert.ErrorIs(t, err, ErrCoverArtProviderNotImplemented)
	assert.Empty(t, results)
}

// CAT-001: no cover-art provider client exists; results must not be invented.
func TestCoverArtService_SearchLastFM_NotFabricated(t *testing.T) {
	service := NewCoverArtService(database.WrapDB(nil, database.DialectSQLite), zap.NewNop())
	results, err := service.searchLastFM(context.Background(), &CoverArtSearchRequest{Title: "T", Artist: "A", Quality: QualityHigh})
	assert.ErrorIs(t, err, ErrCoverArtProviderNotImplemented)
	assert.Empty(t, results)
}

// CAT-001: no cover-art provider client exists; results must not be invented.
func TestCoverArtService_SearchITunes_NotFabricated(t *testing.T) {
	service := NewCoverArtService(database.WrapDB(nil, database.DialectSQLite), zap.NewNop())
	results, err := service.searchITunes(context.Background(), &CoverArtSearchRequest{Title: "T", Artist: "A", Quality: QualityHigh})
	assert.ErrorIs(t, err, ErrCoverArtProviderNotImplemented)
	assert.Empty(t, results)
}

// ---------------------------------------------------------------------------
// downloadImage
// ---------------------------------------------------------------------------

func TestCoverArtService_DownloadImage(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("fake image data"))
	}))
	defer server.Close()

	data, err := service.downloadImage(context.Background(), server.URL)
	require.NoError(t, err)
	assert.Equal(t, "fake image data", string(data))
}

func TestCoverArtService_DownloadImage_HTTPError(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := service.downloadImage(context.Background(), server.URL)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestCoverArtService_DownloadImage_InvalidURL(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	_, err := service.downloadImage(context.Background(), "http://localhost:1/nonexistent")
	assert.Error(t, err)
}

// ---------------------------------------------------------------------------
// generateTimestamps — expanded
// ---------------------------------------------------------------------------

func TestCoverArtService_GenerateTimestamps_ZeroCountDefault(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	// count <= 0 defaults to 3
	timestamps := service.generateTimestamps(120.0, 0)
	assert.Len(t, timestamps, 3)
}

func TestCoverArtService_GenerateTimestamps_NegativeCountDefault(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	timestamps := service.generateTimestamps(120.0, -5)
	assert.Len(t, timestamps, 3) // defaults to 3
}

func TestCoverArtService_GenerateTimestamps_EvenlyDistributed(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	timestamps := service.generateTimestamps(100.0, 4)
	assert.Len(t, timestamps, 4)

	// Timestamps should be evenly distributed
	interval := 100.0 / 5.0 // duration / (count+1)
	for i, ts := range timestamps {
		expected := interval * float64(i+1)
		assert.InDelta(t, expected, ts, 0.01,
			"timestamp %d should be evenly distributed", i)
	}
}

// ---------------------------------------------------------------------------
// sortCoverArtResults — expanded
// ---------------------------------------------------------------------------

func TestCoverArtService_SortCoverArtResults_EqualScores(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	results := []CoverArtSearchResult{
		{MatchScore: 0.9, Width: 200},
		{MatchScore: 0.9, Width: 800},
		{MatchScore: 0.9, Width: 500},
	}

	service.sortCoverArtResults(results)

	// Equal scores should sort by width descending
	assert.Equal(t, 800, results[0].Width)
	assert.Equal(t, 500, results[1].Width)
	assert.Equal(t, 200, results[2].Width)
}

func TestCoverArtService_SortCoverArtResults_EmptySlice(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	// Should not panic on empty slice
	service.sortCoverArtResults(nil)
	service.sortCoverArtResults([]CoverArtSearchResult{})
}

func TestCoverArtService_SortCoverArtResults_Single(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	results := []CoverArtSearchResult{{MatchScore: 0.5, Width: 300}}
	service.sortCoverArtResults(results)
	assert.Len(t, results, 1)
	assert.Equal(t, 0.5, results[0].MatchScore)
}

// ---------------------------------------------------------------------------
// resizeImage
// ---------------------------------------------------------------------------

func TestCoverArtService_ResizeImage_ExactDimensions(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	// Create a 100x100 test image
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))

	options := &CoverArtProcessingOptions{
		Width:          50,
		Height:         50,
		PreserveAspect: false,
	}

	result := service.resizeImage(src, options)
	bounds := result.Bounds()
	assert.Equal(t, 50, bounds.Dx())
	assert.Equal(t, 50, bounds.Dy())
}

func TestCoverArtService_ResizeImage_PreserveAspect(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	// Create a 200x100 wide image
	src := image.NewRGBA(image.Rect(0, 0, 200, 100))

	options := &CoverArtProcessingOptions{
		Width:          100,
		Height:         100,
		PreserveAspect: true,
	}

	result := service.resizeImage(src, options)
	bounds := result.Bounds()

	// With a 2:1 aspect ratio and target 100x100, height should be reduced
	assert.Equal(t, 100, bounds.Dx())
	assert.Equal(t, 50, bounds.Dy())
}

// ---------------------------------------------------------------------------
// ProcessImage
// ---------------------------------------------------------------------------

func TestCoverArtService_ProcessImage_JPEG(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.jpg")
	outputPath := filepath.Join(tmpDir, "output.jpg")

	// Create a test JPEG
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}

	f, err := os.Create(inputPath)
	require.NoError(t, err)
	require.NoError(t, jpeg.Encode(f, img, &jpeg.Options{Quality: 90}))
	f.Close()

	options := &CoverArtProcessingOptions{
		Width:          50,
		Height:         50,
		Quality:        80,
		Format:         "jpeg",
		PreserveAspect: false,
	}

	err = service.ProcessImage(inputPath, outputPath, options)
	require.NoError(t, err)

	// Verify output file exists
	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))
}

func TestCoverArtService_ProcessImage_PNG(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	tmpDir := t.TempDir()
	inputPath := filepath.Join(tmpDir, "input.png")
	outputPath := filepath.Join(tmpDir, "output.png")

	// Create a test PNG
	img := image.NewRGBA(image.Rect(0, 0, 80, 80))
	f, err := os.Create(inputPath)
	require.NoError(t, err)
	require.NoError(t, png.Encode(f, img))
	f.Close()

	options := &CoverArtProcessingOptions{
		Width:          40,
		Height:         40,
		Format:         "png",
		PreserveAspect: false,
	}

	err = service.ProcessImage(inputPath, outputPath, options)
	require.NoError(t, err)

	info, err := os.Stat(outputPath)
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))
}

func TestCoverArtService_ProcessImage_NonexistentInput(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	err := service.ProcessImage("/nonexistent/file.jpg", "/tmp/out.jpg", &CoverArtProcessingOptions{
		Width: 50, Height: 50, Format: "jpeg",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open input file")
}

// ---------------------------------------------------------------------------
// ScanLocalCoverArt
// ---------------------------------------------------------------------------

func TestCoverArtService_ScanLocalCoverArt_NoFiles(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	tmpDir := t.TempDir()
	request := &LocalCoverArtScanRequest{
		MediaItemID: 1,
		Directory:   tmpDir,
		Recursive:   false,
	}

	results, err := service.ScanLocalCoverArt(context.Background(), request)
	require.NoError(t, err)
	assert.Empty(t, results)
}

func TestCoverArtService_ScanLocalCoverArt_NonexistentDir(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	request := &LocalCoverArtScanRequest{
		MediaItemID: 1,
		Directory:   "/nonexistent/directory",
		Recursive:   false,
	}

	_, err := service.ScanLocalCoverArt(context.Background(), request)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read directory")
}

// ---------------------------------------------------------------------------
// getCoverArtDownloadInfo
// ---------------------------------------------------------------------------

func TestCoverArtService_GetCoverArtDownloadInfo_NilDB(t *testing.T) {
	mockDB := database.WrapDB(nil, database.DialectSQLite)
	logger := zap.NewNop()
	service := NewCoverArtService(mockDB, logger)

	// With nil underlying DB, should fall back to default result
	result, err := service.getCoverArtDownloadInfo(context.Background(), "test-id")
	require.NoError(t, err)
	assert.Equal(t, "test-id", result.ID)
	assert.Equal(t, CoverArtProviderLocal, result.Provider)
	assert.Equal(t, "cache", result.Source)
}

// ---------------------------------------------------------------------------
// Request/Response types
// ---------------------------------------------------------------------------

func TestCoverArtSearchRequest_Fields(t *testing.T) {
	album := "Test Album"
	year := 2024
	mbID := "mb-123"
	spotifyID := "sp-456"

	req := &CoverArtSearchRequest{
		Title:         "Test Track",
		Artist:        "Test Artist",
		Album:         &album,
		Year:          &year,
		MusicBrainzID: &mbID,
		SpotifyID:     &spotifyID,
		Quality:       QualityHigh,
		Providers: []CoverArtProvider{
			CoverArtProviderMusicBrainz,
			CoverArtProviderITunes,
		},
		UseCache: true,
	}

	assert.Equal(t, "Test Track", req.Title)
	assert.Equal(t, "Test Artist", req.Artist)
	assert.NotNil(t, req.Album)
	assert.Equal(t, "Test Album", *req.Album)
	assert.NotNil(t, req.Year)
	assert.Equal(t, 2024, *req.Year)
	assert.True(t, req.UseCache)
	assert.Len(t, req.Providers, 2)
}

func TestVideoThumbnailRequest_Fields(t *testing.T) {
	req := &VideoThumbnailRequest{
		MediaItemID:   42,
		VideoPath:     "/path/to/video.mp4",
		Timestamps:    []float64{10.0, 30.0, 60.0},
		Quality:       QualityMedium,
		GenerateSizes: []CoverArtQuality{QualityThumbnail, QualityHigh},
		Count:         5,
	}

	assert.Equal(t, int64(42), req.MediaItemID)
	assert.Equal(t, "/path/to/video.mp4", req.VideoPath)
	assert.Len(t, req.Timestamps, 3)
	assert.Equal(t, 5, req.Count)
}

func TestCoverArtProcessingOptions_Fields(t *testing.T) {
	bgColor := "#ffffff"
	opts := &CoverArtProcessingOptions{
		Width:           300,
		Height:          300,
		Quality:         90,
		Format:          "jpeg",
		Crop:            true,
		PreserveAspect:  false,
		BackgroundColor: &bgColor,
	}

	assert.Equal(t, 300, opts.Width)
	assert.Equal(t, 300, opts.Height)
	assert.Equal(t, 90, opts.Quality)
	assert.True(t, opts.Crop)
	assert.False(t, opts.PreserveAspect)
	assert.NotNil(t, opts.BackgroundColor)
}

func TestCoverArtService_SearchCoverArt_AllProvidersFailIsError(t *testing.T) {
	service := NewCoverArtService(database.WrapDB(nil, database.DialectSQLite), zap.NewNop())
	results, err := service.SearchCoverArt(context.Background(), &CoverArtSearchRequest{Title: "T", Artist: "A"})
	assert.ErrorIs(t, err, ErrCoverArtProviderNotImplemented)
	assert.Empty(t, results)
}
