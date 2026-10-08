package services

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"catalogizer/database"
	"catalogizer/filesystem"
	"catalogizer/models"

	"digital.vasic.storage/pkg/object"

	"go.uber.org/zap"
	"golang.org/x/image/draw"
	"golang.org/x/net/proxy"
)

// ProxyConfiger is the minimal interface required by CoverArtService for proxy settings.
type ProxyConfiger interface {
	IsEnabled() bool
	GetURL() string
	GetHTTPURL() string
	GetUsername() string
	GetPassword() string
}

// CoverArtService handles cover art retrieval, processing, and caching
type CoverArtService struct {
	db         *database.DB
	logger     *zap.Logger
	httpClient *http.Client
	proxyCfg   ProxyConfiger
	apiKeys    map[string]string
	cacheDir   string
	store      object.ObjectStore
	bucket     string
	fsFactory  filesystem.ClientFactory
}

// CoverArtProvider represents different cover art providers
type CoverArtProvider string

const (
	CoverArtProviderMusicBrainz CoverArtProvider = "musicbrainz"
	CoverArtProviderLastFM      CoverArtProvider = "lastfm"
	CoverArtProviderSpotify     CoverArtProvider = "spotify"
	CoverArtProviderDeezer      CoverArtProvider = "deezer"
	CoverArtProviderITunes      CoverArtProvider = "itunes"
	CoverArtProviderDiscogs     CoverArtProvider = "discogs"
	CoverArtProviderEmbedded    CoverArtProvider = "embedded"
	CoverArtProviderLocal       CoverArtProvider = "local"
)

// CoverArtQuality represents different quality levels
type CoverArtQuality string

const (
	QualityThumbnail CoverArtQuality = "thumbnail" // 150x150
	QualityMedium    CoverArtQuality = "medium"    // 300x300
	QualityHigh      CoverArtQuality = "high"      // 600x600
	QualityOriginal  CoverArtQuality = "original"  // Original size
)

// CoverArtSearchRequest represents a cover art search request
type CoverArtSearchRequest struct {
	Title         string             `json:"title"`
	Artist        string             `json:"artist"`
	Album         *string            `json:"album,omitempty"`
	Year          *int               `json:"year,omitempty"`
	MusicBrainzID *string            `json:"musicbrainz_id,omitempty"`
	SpotifyID     *string            `json:"spotify_id,omitempty"`
	Quality       CoverArtQuality    `json:"quality"`
	Providers     []CoverArtProvider `json:"providers,omitempty"`
	UseCache      bool               `json:"use_cache"`
}

// CoverArtSearchResult represents a cover art search result
type CoverArtSearchResult struct {
	ID           string           `json:"id"`
	Provider     CoverArtProvider `json:"provider"`
	Title        string           `json:"title"`
	Artist       string           `json:"artist"`
	Album        *string          `json:"album,omitempty"`
	URL          string           `json:"url"`
	ThumbnailURL *string          `json:"thumbnail_url,omitempty"`
	Width        int              `json:"width"`
	Height       int              `json:"height"`
	Format       string           `json:"format"`
	Quality      CoverArtQuality  `json:"quality"`
	Size         *int64           `json:"size,omitempty"`
	MatchScore   float64          `json:"match_score"`
	Copyright    *string          `json:"copyright,omitempty"`
	Source       string           `json:"source"`
}

// CoverArtDownloadRequest represents a cover art download request
type CoverArtDownloadRequest struct {
	MediaItemID   int64             `json:"media_item_id"`
	ResultID      string            `json:"result_id"`
	Quality       CoverArtQuality   `json:"quality"`
	GenerateSizes []CoverArtQuality `json:"generate_sizes,omitempty"`
	SetAsDefault  bool              `json:"set_as_default"`
}

// VideoThumbnailRequest represents a video thumbnail generation request
type VideoThumbnailRequest struct {
	MediaItemID   int64             `json:"media_item_id"`
	VideoPath     string            `json:"video_path"`
	Timestamps    []float64         `json:"timestamps,omitempty"` // Seconds
	Quality       CoverArtQuality   `json:"quality"`
	GenerateSizes []CoverArtQuality `json:"generate_sizes,omitempty"`
	Count         int               `json:"count"` // Number of thumbnails to generate
}

// LocalCoverArtScanRequest represents a request to scan for local cover art
type LocalCoverArtScanRequest struct {
	MediaItemID int64  `json:"media_item_id"`
	Directory   string `json:"directory"`
	Recursive   bool   `json:"recursive"`
}

// CoverArtProcessingOptions represents image processing options
type CoverArtProcessingOptions struct {
	Width           int     `json:"width"`
	Height          int     `json:"height"`
	Quality         int     `json:"quality"`                    // JPEG quality 1-100
	Format          string  `json:"format"`                     // "jpeg", "png", "webp"
	Crop            bool    `json:"crop"`                       // Crop to exact dimensions
	PreserveAspect  bool    `json:"preserve_aspect"`            // Preserve aspect ratio
	BackgroundColor *string `json:"background_color,omitempty"` // Hex color for padding
}

// NewCoverArtService creates a new cover art service
func NewCoverArtService(db *database.DB, logger *zap.Logger) *CoverArtService {
	return &CoverArtService{
		db:         db,
		logger:     logger,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		apiKeys:    make(map[string]string),
		cacheDir:   "./cache/cover_art",
	}
}

// SetProxyConfig configures the proxy used for external cover art downloads.
func (s *CoverArtService) SetProxyConfig(cfg ProxyConfiger) {
	s.proxyCfg = cfg
}

// SetObjectStore configures the optional object store for cover art caching.
func (s *CoverArtService) SetObjectStore(store object.ObjectStore, bucket string) {
	s.store = store
	s.bucket = bucket
}

// SetClientFactory configures the filesystem client factory for reading remote files.
func (s *CoverArtService) SetClientFactory(factory filesystem.ClientFactory) {
	s.fsFactory = factory
}

// HasObjectStore returns true when an object store is configured.
func (s *CoverArtService) HasObjectStore() bool {
	return s.store != nil && s.bucket != ""
}

// GetObjectStore returns the configured object store.
func (s *CoverArtService) GetObjectStore() object.ObjectStore {
	return s.store
}

// GetBucket returns the configured object store bucket.
func (s *CoverArtService) GetBucket() string {
	return s.bucket
}

// SearchCoverArt searches for cover art across multiple providers
func (s *CoverArtService) SearchCoverArt(ctx context.Context, request *CoverArtSearchRequest) ([]CoverArtSearchResult, error) {
	s.logger.Info("Searching cover art",
		zap.String("title", request.Title),
		zap.String("artist", request.Artist),
		zap.String("album", getStringValue(request.Album)))

	// Check cache first if requested
	if request.UseCache {
		if cached := s.getCachedCoverArt(ctx, request); cached != nil {
			return []CoverArtSearchResult{*cached}, nil
		}
	}

	var allResults []CoverArtSearchResult

	// Default providers if none specified
	providers := request.Providers
	if len(providers) == 0 {
		providers = []CoverArtProvider{
			CoverArtProviderMusicBrainz,
			CoverArtProviderLastFM,
			CoverArtProviderITunes,
		}
	}

	// Search each provider
	for _, provider := range providers {
		results, err := s.searchProvider(ctx, provider, request)
		if err != nil {
			s.logger.Warn("Provider search failed",
				zap.String("provider", string(provider)),
				zap.Error(err))
			continue
		}
		allResults = append(allResults, results...)
	}

	// Sort by match score and quality
	s.sortCoverArtResults(allResults)

	s.logger.Info("Cover art search completed",
		zap.Int("total_results", len(allResults)))

	return allResults, nil
}

// DownloadCoverArt downloads and processes cover art
func (s *CoverArtService) DownloadCoverArt(ctx context.Context, request *CoverArtDownloadRequest) (*CoverArt, error) {
	s.logger.Info("Downloading cover art",
		zap.Int64("media_item_id", request.MediaItemID),
		zap.String("result_id", request.ResultID))

	// Get download info
	result, err := s.getCoverArtDownloadInfo(ctx, request.ResultID)
	if err != nil {
		return nil, fmt.Errorf("failed to get download info: %w", err)
	}

	// Download image
	imageData, err := s.downloadImage(ctx, result.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}

	// Process and save image
	coverArt, err := s.processAndSaveCoverArt(ctx, request.MediaItemID, imageData, result, request)
	if err != nil {
		return nil, fmt.Errorf("failed to process cover art: %w", err)
	}

	// Generate additional sizes if requested
	if len(request.GenerateSizes) > 0 {
		go s.generateAdditionalSizes(ctx, coverArt, request.GenerateSizes)
	}

	// Set as default if requested
	if request.SetAsDefault {
		if err := s.setDefaultCoverArt(ctx, request.MediaItemID, coverArt.ID); err != nil {
			s.logger.Warn("Failed to set as default cover art", zap.Error(err))
		}
	}

	return coverArt, nil
}

// GenerateVideoThumbnails generates thumbnails for video files
func (s *CoverArtService) GenerateVideoThumbnails(ctx context.Context, request *VideoThumbnailRequest) ([]*CoverArt, error) {
	s.logger.Info("Generating video thumbnails",
		zap.Int64("media_item_id", request.MediaItemID),
		zap.String("video_path", request.VideoPath))

	// Get video duration
	duration, err := s.getVideoDuration(request.VideoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get video duration: %w", err)
	}

	// Generate timestamps if not provided
	timestamps := request.Timestamps
	if len(timestamps) == 0 {
		timestamps = s.generateTimestamps(duration, request.Count)
	}

	var thumbnails []*CoverArt

	// Generate thumbnail for each timestamp
	for i, timestamp := range timestamps {
		thumbnail, err := s.generateVideoThumbnail(ctx, request, timestamp, i)
		if err != nil {
			s.logger.Warn("Failed to generate thumbnail",
				zap.Float64("timestamp", timestamp),
				zap.Error(err))
			continue
		}
		thumbnails = append(thumbnails, thumbnail)
	}

	return thumbnails, nil
}

// ScanLocalCoverArt scans directory for local cover art files
func (s *CoverArtService) ScanLocalCoverArt(ctx context.Context, request *LocalCoverArtScanRequest) ([]*CoverArt, error) {
	s.logger.Info("Scanning local cover art",
		zap.Int64("media_item_id", request.MediaItemID),
		zap.String("directory", request.Directory))

	var coverArts []*CoverArt

	// Common cover art filenames
	coverFilenames := []string{
		"cover.jpg", "cover.jpeg", "cover.png",
		"folder.jpg", "folder.jpeg", "folder.png",
		"album.jpg", "album.jpeg", "album.png",
		"front.jpg", "front.jpeg", "front.png",
		"albumart.jpg", "albumart.jpeg", "albumart.png",
	}

	// Scan directory
	files, err := os.ReadDir(request.Directory)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory: %w", err)
	}

	for _, file := range files {
		if file.IsDir() && request.Recursive {
			// Recursively scan subdirectories
			subRequest := *request
			subRequest.Directory = filepath.Join(request.Directory, file.Name())
			subCoverArts, err := s.ScanLocalCoverArt(ctx, &subRequest)
			if err != nil {
				s.logger.Warn("Failed to scan subdirectory", zap.Error(err))
				continue
			}
			coverArts = append(coverArts, subCoverArts...)
			continue
		}

		// Check if file is a potential cover art
		filename := strings.ToLower(file.Name())
		for _, coverFilename := range coverFilenames {
			if filename == coverFilename || strings.HasPrefix(filename, strings.TrimSuffix(coverFilename, filepath.Ext(coverFilename))) {
				filePath := filepath.Join(request.Directory, file.Name())
				coverArt, err := s.processLocalCoverArt(ctx, request.MediaItemID, filePath)
				if err != nil {
					s.logger.Warn("Failed to process local cover art",
						zap.String("file", filePath),
						zap.Error(err))
					continue
				}
				coverArts = append(coverArts, coverArt)
				break
			}
		}
	}

	return coverArts, nil
}

// GetCoverArt returns cover art for a media item
func (s *CoverArtService) GetCoverArt(ctx context.Context, mediaItemID int64) (*CoverArt, error) {
	if s.db == nil {
		return nil, nil
	}

	query := `
		SELECT id, media_item_id, source, url, local_path, width, height,
		       format, size, quality, created_at, cached_at
		FROM cover_art WHERE media_item_id = ? AND is_default = 1
		ORDER BY created_at DESC LIMIT 1`

	var coverArt CoverArt
	var url, localPath sql.NullString
	var size sql.NullInt64
	var cachedAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, mediaItemID).Scan(
		&coverArt.ID, &coverArt.MediaItemID, &coverArt.Source,
		&url, &localPath, &coverArt.Width, &coverArt.Height,
		&coverArt.Format, &size, &coverArt.Quality,
		&coverArt.CreatedAt, &cachedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // No cover art found
		}
		return nil, fmt.Errorf("failed to get cover art: %w", err)
	}

	if url.Valid {
		coverArt.URL = &url.String
	}
	if localPath.Valid {
		coverArt.LocalPath = &localPath.String
	}
	if size.Valid {
		coverArt.Size = &size.Int64
	}
	if cachedAt.Valid {
		coverArt.CachedAt = &cachedAt.Time
	}

	return &coverArt, nil
}

// ProcessImage processes an image with specified options
func (s *CoverArtService) ProcessImage(inputPath string, outputPath string, options *CoverArtProcessingOptions) error {
	s.logger.Debug("Processing image",
		zap.String("input", inputPath),
		zap.String("output", outputPath))

	// Open input image
	inputFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %w", err)
	}
	defer inputFile.Close()

	// Decode image
	img, format, err := image.Decode(inputFile)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// Resize image
	resizedImg := s.resizeImage(img, options)

	// Create output file
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	// Encode image
	switch strings.ToLower(options.Format) {
	case "jpeg", "jpg":
		return jpeg.Encode(outputFile, resizedImg, &jpeg.Options{Quality: options.Quality})
	case "png":
		return png.Encode(outputFile, resizedImg)
	default:
		// Default to original format
		switch format {
		case "jpeg":
			return jpeg.Encode(outputFile, resizedImg, &jpeg.Options{Quality: options.Quality})
		case "png":
			return png.Encode(outputFile, resizedImg)
		default:
			return jpeg.Encode(outputFile, resizedImg, &jpeg.Options{Quality: options.Quality})
		}
	}
}

// Provider-specific implementations
func (s *CoverArtService) searchProvider(ctx context.Context, provider CoverArtProvider, request *CoverArtSearchRequest) ([]CoverArtSearchResult, error) {
	switch provider {
	case CoverArtProviderMusicBrainz:
		return s.searchMusicBrainz(ctx, request)
	case CoverArtProviderLastFM:
		return s.searchLastFM(ctx, request)
	case CoverArtProviderITunes:
		return s.searchITunes(ctx, request)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

func (s *CoverArtService) searchMusicBrainz(ctx context.Context, request *CoverArtSearchRequest) ([]CoverArtSearchResult, error) {
	s.logger.Debug("Searching MusicBrainz",
		zap.String("artist", request.Artist),
		zap.String("album", getStringValue(request.Album)))

	// Mock implementation for demonstration
	result := CoverArtSearchResult{
		ID:         "mb_1",
		Provider:   CoverArtProviderMusicBrainz,
		Title:      request.Title,
		Artist:     request.Artist,
		Album:      request.Album,
		URL:        "https://coverartarchive.org/sample/front.jpg",
		Width:      500,
		Height:     500,
		Format:     "jpeg",
		Quality:    QualityHigh,
		MatchScore: 0.9,
		Source:     "coverartarchive.org",
	}

	return []CoverArtSearchResult{result}, nil
}

func (s *CoverArtService) searchLastFM(ctx context.Context, request *CoverArtSearchRequest) ([]CoverArtSearchResult, error) {
	s.logger.Debug("Searching Last.FM")

	result := CoverArtSearchResult{
		ID:         "lastfm_1",
		Provider:   CoverArtProviderLastFM,
		Title:      request.Title,
		Artist:     request.Artist,
		Album:      request.Album,
		URL:        "https://lastfm-img2.akamaized.net/sample.jpg",
		Width:      300,
		Height:     300,
		Format:     "jpeg",
		Quality:    QualityMedium,
		MatchScore: 0.85,
		Source:     "last.fm",
	}

	return []CoverArtSearchResult{result}, nil
}

func (s *CoverArtService) searchITunes(ctx context.Context, request *CoverArtSearchRequest) ([]CoverArtSearchResult, error) {
	s.logger.Debug("Searching iTunes")

	result := CoverArtSearchResult{
		ID:         "itunes_1",
		Provider:   CoverArtProviderITunes,
		Title:      request.Title,
		Artist:     request.Artist,
		Album:      request.Album,
		URL:        "https://is1-ssl.mzstatic.com/sample.jpg",
		Width:      600,
		Height:     600,
		Format:     "jpeg",
		Quality:    QualityHigh,
		MatchScore: 0.92,
		Source:     "itunes.apple.com",
	}

	return []CoverArtSearchResult{result}, nil
}

// Helper functions
func (s *CoverArtService) downloadImage(ctx context.Context, imageURL string) ([]byte, error) {
	if err := GuardProviderURL(imageURL, SSRFGuardConfig{}); err != nil {
		return nil, fmt.Errorf("cover_art: unsafe URL: %w", err)
	}
	client := s.httpClient
	if s.proxyCfg != nil && s.proxyCfg.IsEnabled() {
		transport := &http.Transport{}
		if s.proxyCfg.GetURL() != "" {
			parsedProxy, err := url.Parse(s.proxyCfg.GetURL())
			if err == nil && parsedProxy.Scheme == "socks5" {
				var auth *proxy.Auth
				if s.proxyCfg.GetUsername() != "" || s.proxyCfg.GetPassword() != "" {
					auth = &proxy.Auth{User: s.proxyCfg.GetUsername(), Password: s.proxyCfg.GetPassword()}
				}
				SOCKS5Dialer, err := proxy.SOCKS5("tcp", parsedProxy.Host, auth, proxy.Direct)
				if err == nil {
					transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
						return SOCKS5Dialer.Dial(network, addr)
					}
					client = &http.Client{Timeout: 30 * time.Second, Transport: transport}
				}
			}
		}
		if client == s.httpClient && s.proxyCfg.GetHTTPURL() != "" {
			parsedHTTPProxy, err := url.Parse(s.proxyCfg.GetHTTPURL())
			if err == nil {
				transport.Proxy = http.ProxyURL(parsedHTTPProxy)
				client = &http.Client{Timeout: 30 * time.Second, Transport: transport}
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, "GET", imageURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return io.ReadAll(resp.Body)
}

func (s *CoverArtService) resizeImage(img image.Image, options *CoverArtProcessingOptions) image.Image {
	bounds := img.Bounds()
	srcWidth := bounds.Dx()
	srcHeight := bounds.Dy()

	dstWidth := options.Width
	dstHeight := options.Height

	if options.PreserveAspect {
		// Calculate dimensions preserving aspect ratio
		aspectRatio := float64(srcWidth) / float64(srcHeight)
		if float64(dstWidth)/float64(dstHeight) > aspectRatio {
			dstWidth = int(float64(dstHeight) * aspectRatio)
		} else {
			dstHeight = int(float64(dstWidth) / aspectRatio)
		}
	}

	// Create destination image
	dst := image.NewRGBA(image.Rect(0, 0, dstWidth, dstHeight))

	// Resize image
	draw.BiLinear.Scale(dst, dst.Bounds(), img, bounds, draw.Over, nil)

	return dst
}

func (s *CoverArtService) generateTimestamps(duration float64, count int) []float64 {
	if count <= 0 {
		count = 3 // Default to 3 thumbnails
	}

	var timestamps []float64
	interval := duration / float64(count+1)

	for i := 1; i <= count; i++ {
		timestamps = append(timestamps, interval*float64(i))
	}

	return timestamps
}

func (s *CoverArtService) generateCoverArtID() string {
	return fmt.Sprintf("cover_%d", time.Now().UnixNano())
}

func (s *CoverArtService) generateCacheKey(request *CoverArtSearchRequest) string {
	data := fmt.Sprintf("%s_%s_%s_%s", request.Artist, request.Title,
		getStringValue(request.Album), request.Quality)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

func (s *CoverArtService) sortCoverArtResults(results []CoverArtSearchResult) {
	// Sort by match score descending, then by quality/size
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].MatchScore < results[j].MatchScore ||
				(results[i].MatchScore == results[j].MatchScore && results[i].Width < results[j].Width) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// getCachedCoverArt retrieves cached cover art from the database
func (s *CoverArtService) getCachedCoverArt(ctx context.Context, request *CoverArtSearchRequest) *CoverArtSearchResult {
	// Generate cache key based on request parameters
	cacheKey := s.generateCacheKey(request)

	query := `
		SELECT id, provider, title, artist, album, url, thumbnail_url, width, height,
		       format, quality, size, match_score, source, created_at
		FROM cover_art_cache
		WHERE cache_key = ? AND created_at > ?
		ORDER BY match_score DESC
		LIMIT 1
	`

	// Cache valid for 30 days
	cacheExpiry := time.Now().Add(-30 * 24 * time.Hour)

	var result CoverArtSearchResult
	var album sql.NullString
	var thumbnailURL sql.NullString
	var size sql.NullInt64
	var createdAt time.Time

	err := s.db.QueryRowContext(ctx, query, cacheKey, cacheExpiry).Scan(
		&result.ID, &result.Provider, &result.Title, &result.Artist,
		&album, &result.URL, &thumbnailURL, &result.Width, &result.Height,
		&result.Format, &result.Quality, &size, &result.MatchScore,
		&result.Source, &createdAt,
	)

	if err != nil {
		if err != sql.ErrNoRows {
			s.logger.Warn("Failed to get cached cover art", zap.Error(err))
		}
		return nil
	}

	if album.Valid {
		result.Album = &album.String
	}
	if thumbnailURL.Valid {
		result.ThumbnailURL = &thumbnailURL.String
	}
	if size.Valid {
		result.Size = &size.Int64
	}

	return &result
}

// getCoverArtDownloadInfo retrieves cover art download information
func (s *CoverArtService) getCoverArtDownloadInfo(ctx context.Context, resultID string) (*CoverArtSearchResult, error) {
	if s.db != nil {
		var url, provider, quality string
		err := s.db.QueryRowContext(ctx,
			"SELECT url, source, quality FROM cover_art WHERE id = ?", resultID,
		).Scan(&url, &provider, &quality)
		if err == nil {
			return &CoverArtSearchResult{
				ID:       resultID,
				Provider: CoverArtProvider(provider),
				URL:      url,
				Quality:  CoverArtQuality(quality),
				Source:   provider,
			}, nil
		}
	}

	return &CoverArtSearchResult{
		ID:       resultID,
		Provider: CoverArtProviderLocal,
		URL:      "",
		Quality:  QualityHigh,
		Source:   "cache",
	}, nil
}

// processAndSaveCoverArt processes and saves cover art
func (s *CoverArtService) processAndSaveCoverArt(ctx context.Context, mediaItemID int64, imageData []byte, result *CoverArtSearchResult, request *CoverArtDownloadRequest) (*CoverArt, error) {
	// Decode image
	img, format, err := image.Decode(strings.NewReader(string(imageData)))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	// Get image dimensions
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Generate local path
	coverID := s.generateCoverArtID()
	filename := fmt.Sprintf("%s.%s", coverID, format)
	localPath := filepath.Join(s.cacheDir, filename)

	// Ensure cache directory exists
	if err := os.MkdirAll(s.cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Save image
	outFile, err := os.Create(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create file: %w", err)
	}
	defer outFile.Close()

	switch format {
	case "jpeg", "jpg":
		err = jpeg.Encode(outFile, img, &jpeg.Options{Quality: 90})
	case "png":
		err = png.Encode(outFile, img)
	default:
		err = jpeg.Encode(outFile, img, &jpeg.Options{Quality: 90})
	}

	if err != nil {
		return nil, fmt.Errorf("failed to save image: %w", err)
	}

	// Get file size
	fileInfo, err := os.Stat(localPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}
	size := fileInfo.Size()

	// Create CoverArt record
	now := time.Now()
	coverArt := &CoverArt{
		ID:          coverID,
		MediaItemID: mediaItemID,
		Source:      string(result.Provider),
		LocalPath:   &localPath,
		Width:       &width,
		Height:      &height,
		Format:      format,
		Size:        &size,
		Quality:     string(request.Quality),
		CreatedAt:   now,
		CachedAt:    &now,
	}

	if err := s.saveCoverArtToDB(ctx, coverArt); err != nil {
		s.logger.Warn("Failed to save cover art to database", zap.Error(err))
	}

	return coverArt, nil
}

// generateAdditionalSizes generates additional sizes of cover art
func (s *CoverArtService) generateAdditionalSizes(ctx context.Context, coverArt *CoverArt, sizes []CoverArtQuality) {
	if coverArt.LocalPath == nil {
		s.logger.Warn("Cannot generate additional sizes: no local path")
		return
	}

	// Open source image
	srcFile, err := os.Open(*coverArt.LocalPath)
	if err != nil {
		s.logger.Error("Failed to open source image", zap.Error(err))
		return
	}
	defer srcFile.Close()

	srcImg, _, err := image.Decode(srcFile)
	if err != nil {
		s.logger.Error("Failed to decode source image", zap.Error(err))
		return
	}

	// Generate each size
	for _, quality := range sizes {
		targetSize := 0
		switch quality {
		case QualityThumbnail:
			targetSize = 150
		case QualityMedium:
			targetSize = 300
		case QualityHigh:
			targetSize = 600
		default:
			continue
		}

		// Resize image
		newImg := image.NewRGBA(image.Rect(0, 0, targetSize, targetSize))
		draw.ApproxBiLinear.Scale(newImg, newImg.Bounds(), srcImg, srcImg.Bounds(), draw.Over, nil)

		// Save resized image
		resizedFilename := fmt.Sprintf("%s_%s.%s", coverArt.ID, quality, coverArt.Format)
		resizedPath := filepath.Join(s.cacheDir, resizedFilename)

		outFile, err := os.Create(resizedPath)
		if err != nil {
			s.logger.Error("Failed to create resized file",
				zap.String("quality", string(quality)),
				zap.Error(err))
			continue
		}

		if coverArt.Format == "png" {
			err = png.Encode(outFile, newImg)
		} else {
			err = jpeg.Encode(outFile, newImg, &jpeg.Options{Quality: 85})
		}
		outFile.Close()

		if err != nil {
			s.logger.Error("Failed to encode resized image",
				zap.String("quality", string(quality)),
				zap.Error(err))
		}
	}
}

// setDefaultCoverArt sets cover art as default for a media item
func (s *CoverArtService) setDefaultCoverArt(ctx context.Context, mediaItemID int64, coverArtID string) error {
	// In a real implementation, this would update the database to mark the cover art as default
	query := `UPDATE cover_art SET is_default = 0 WHERE media_item_id = ?`
	_, err := s.db.ExecContext(ctx, query, mediaItemID)
	if err != nil {
		return err
	}

	query = `UPDATE cover_art SET is_default = 1 WHERE id = ? AND media_item_id = ?`
	_, err = s.db.ExecContext(ctx, query, coverArtID, mediaItemID)
	return err
}

// saveCoverArtToDB persists a CoverArt record to the database.
func (s *CoverArtService) saveCoverArtToDB(ctx context.Context, coverArt *CoverArt) error {
	if s.db == nil {
		return nil
	}

	query := `
		INSERT INTO cover_art (id, media_item_id, source, url, local_path, width, height, format, size, quality, is_default, created_at, cached_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			source = excluded.source,
			url = excluded.url,
			local_path = excluded.local_path,
			width = excluded.width,
			height = excluded.height,
			format = excluded.format,
			size = excluded.size,
			quality = excluded.quality,
			is_default = excluded.is_default,
			cached_at = excluded.cached_at
	`
	if s.db.Dialect().IsPostgres() {
		query = `
			INSERT INTO cover_art (id, media_item_id, source, url, local_path, width, height, format, size, quality, is_default, created_at, cached_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
			ON CONFLICT(id) DO UPDATE SET
				source = excluded.source,
				url = excluded.url,
				local_path = excluded.local_path,
				width = excluded.width,
				height = excluded.height,
				format = excluded.format,
				size = excluded.size,
				quality = excluded.quality,
				is_default = excluded.is_default,
				cached_at = excluded.cached_at
		`
	}

	var url, localPath sql.NullString
	var width, height sql.NullInt64
	var size sql.NullInt64
	var cachedAt sql.NullTime

	if coverArt.URL != nil {
		url = sql.NullString{String: *coverArt.URL, Valid: true}
	}
	if coverArt.LocalPath != nil {
		localPath = sql.NullString{String: *coverArt.LocalPath, Valid: true}
	}
	if coverArt.Width != nil {
		width = sql.NullInt64{Int64: int64(*coverArt.Width), Valid: true}
	}
	if coverArt.Height != nil {
		height = sql.NullInt64{Int64: int64(*coverArt.Height), Valid: true}
	}
	if coverArt.Size != nil {
		size = sql.NullInt64{Int64: *coverArt.Size, Valid: true}
	}
	if coverArt.CachedAt != nil {
		cachedAt = sql.NullTime{Time: *coverArt.CachedAt, Valid: true}
	}

	_, err := s.db.ExecContext(ctx, query,
		coverArt.ID, coverArt.MediaItemID, coverArt.Source,
		url, localPath, width, height, coverArt.Format, size,
		coverArt.Quality, 0, coverArt.CreatedAt, cachedAt,
	)
	return err
}

// getVideoDuration gets the duration of a video file using ffprobe
func (s *CoverArtService) getVideoDuration(videoPath string) (float64, error) {
	output, err := exec.Command("ffprobe",
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	).Output()
	if err == nil {
		if duration, parseErr := strconv.ParseFloat(strings.TrimSpace(string(output)), 64); parseErr == nil && duration > 0 {
			return duration, nil
		}
	}

	// Fallback: estimate from file size (assume ~5 Mbps bitrate)
	if info, statErr := os.Stat(videoPath); statErr == nil {
		estimated := float64(info.Size()) / (5 * 1024 * 1024 / 8)
		if estimated > 0 {
			return estimated, nil
		}
	}

	return 120.0, nil
}

// generateVideoThumbnail generates a thumbnail for a video at a specific timestamp
func (s *CoverArtService) generateVideoThumbnail(ctx context.Context, request *VideoThumbnailRequest, timestamp float64, index int) (*CoverArt, error) {
	coverID := fmt.Sprintf("video_thumb_%d_%d", request.MediaItemID, index)
	now := time.Now()

	outputPath := filepath.Join(s.cacheDir, coverID+".jpg")

	// Try ffmpeg to extract a frame
	err := exec.CommandContext(ctx, "ffmpeg",
		"-ss", fmt.Sprintf("%.2f", timestamp),
		"-i", request.VideoPath,
		"-vframes", "1",
		"-q:v", "2",
		"-y",
		outputPath,
	).Run()

	if err == nil {
		if info, statErr := os.Stat(outputPath); statErr == nil {
			size := info.Size()
			coverArt := &CoverArt{
				ID:          coverID,
				MediaItemID: request.MediaItemID,
				Source:      "video_thumbnail",
				LocalPath:   &outputPath,
				Format:      "jpeg",
				Size:        &size,
				Quality:     string(request.Quality),
				CreatedAt:   now,
				CachedAt:    &now,
			}
			if s.db != nil {
				if dbErr := s.saveCoverArtToDB(ctx, coverArt); dbErr != nil {
					s.logger.Warn("Failed to save video thumbnail to database", zap.Error(dbErr))
				} else {
					_ = s.setDefaultCoverArt(ctx, request.MediaItemID, coverArt.ID)
				}
			}
			return coverArt, nil
		}
	}

	return nil, fmt.Errorf("failed to generate video thumbnail for %s", request.VideoPath)
}

// processLocalCoverArt processes local cover art file
func (s *CoverArtService) processLocalCoverArt(ctx context.Context, mediaItemID int64, filePath string) (*CoverArt, error) {
	// Open and decode image
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, format, err := image.Decode(file)
	if err != nil {
		return nil, err
	}

	// Get image dimensions
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Get file size
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	size := fileInfo.Size()

	// Create CoverArt record
	coverID := s.generateCoverArtID()
	now := time.Now()

	coverArt := &CoverArt{
		ID:          coverID,
		MediaItemID: mediaItemID,
		Source:      "local",
		LocalPath:   &filePath,
		Width:       &width,
		Height:      &height,
		Format:      format,
		Size:        &size,
		Quality:     "original",
		CreatedAt:   now,
	}

	if err := s.saveCoverArtToDB(ctx, coverArt); err != nil {
		s.logger.Warn("Failed to save local cover art to database", zap.Error(err))
	}

	return coverArt, nil
}

// CacheExternalCoverArt downloads an external cover image, persists it locally,
// and saves a record in the cover_art table so subsequent requests are served
// directly from the backend without hitting external CDNs.
func (s *CoverArtService) CacheExternalCoverArt(ctx context.Context, mediaItemID int64, imageURL string) (*CoverArt, error) {
	if s.db == nil {
		return nil, fmt.Errorf("no database connection")
	}

	// Check if we already have a local cover for this item
	var existingID string
	err := s.db.QueryRowContext(ctx,
		`SELECT id FROM cover_art WHERE media_item_id = ? AND is_default = 1 AND local_path IS NOT NULL AND local_path != '' LIMIT 1`,
		mediaItemID).Scan(&existingID)
	if err == nil && existingID != "" {
		return nil, fmt.Errorf("local cover art already exists")
	}

	s.logger.Info("Caching external cover art",
		zap.Int64("media_item_id", mediaItemID),
		zap.String("url", imageURL))

	imageData, err := s.downloadImage(ctx, imageURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download image: %w", err)
	}

	// Decode to validate and get dimensions
	img, format, err := image.Decode(strings.NewReader(string(imageData)))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	coverID := s.generateCoverArtID()
	now := time.Now()
	var storageKey string
	var size int64

	if s.HasObjectStore() {
		// Upload to object store (S3/MinIO)
		filename := fmt.Sprintf("%s.jpg", coverID)
		storageKey = "covers/" + filename

		var buf bytes.Buffer
		err = jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
		if err != nil {
			return nil, fmt.Errorf("failed to encode image: %w", err)
		}
		size = int64(buf.Len())

		putErr := s.store.PutObject(ctx, s.bucket, storageKey, &buf, size,
			object.WithContentType("image/jpeg"),
			object.WithMetadata(map[string]string{"media_item_id": strconv.FormatInt(mediaItemID, 10)}),
		)
		if putErr != nil {
			s.logger.Warn("Failed to upload cover art to object store, falling back to local cache",
				zap.Error(putErr))
			storageKey = ""
		}
	}

	if storageKey == "" {
		// Fallback to local filesystem
		filename := fmt.Sprintf("%s.%s", coverID, format)
		localPath := filepath.Join(s.cacheDir, filename)

		if err := os.MkdirAll(s.cacheDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create cache directory: %w", err)
		}

		outFile, err := os.Create(localPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create file: %w", err)
		}
		defer outFile.Close()

		switch format {
		case "jpeg", "jpg":
			err = jpeg.Encode(outFile, img, &jpeg.Options{Quality: 90})
		case "png":
			err = png.Encode(outFile, img)
		default:
			err = jpeg.Encode(outFile, img, &jpeg.Options{Quality: 90})
		}
		if err != nil {
			return nil, fmt.Errorf("failed to encode image: %w", err)
		}

		fileInfo, err := os.Stat(localPath)
		if err != nil {
			return nil, fmt.Errorf("failed to stat file: %w", err)
		}
		size = fileInfo.Size()
		storageKey = localPath
	}

	coverArt := &CoverArt{
		ID:          coverID,
		MediaItemID: mediaItemID,
		Source:      "cached_external",
		LocalPath:   &storageKey,
		Width:       &width,
		Height:      &height,
		Format:      "jpeg",
		Size:        &size,
		Quality:     "high",
		CreatedAt:   now,
		CachedAt:    &now,
	}

	if err := s.saveCoverArtToDB(ctx, coverArt); err != nil {
		return nil, fmt.Errorf("failed to save cover art: %w", err)
	}
	if err := s.setDefaultCoverArt(ctx, mediaItemID, coverArt.ID); err != nil {
		s.logger.Warn("Failed to set default cover art", zap.Error(err))
	}

	return coverArt, nil
}

// GenerateMissingVideoThumbnails scans media items that have no local cover art,
// finds their primary video file, and generates a thumbnail using ffmpeg.
// Returns the number of thumbnails successfully generated.
func (s *CoverArtService) GenerateMissingVideoThumbnails(ctx context.Context, limit int) (int, error) {
	if s.db == nil {
		return 0, fmt.Errorf("no database connection")
	}
	if limit <= 0 {
		limit = 50
	}

	// Find media items with no local cover art that have a primary video file
	query := `
		SELECT mi.id, f.path || '/' || f.name AS full_path,
		       ` + models.StorageRootConnColumnsFor("sr") + `
		FROM media_items mi
		JOIN media_types mt ON mi.media_type_id = mt.id
		JOIN media_files mf ON mi.id = mf.media_item_id
		JOIN files f ON mf.file_id = f.id
		JOIN storage_roots sr ON f.storage_root_id = sr.id
		LEFT JOIN cover_art ca ON mi.id = ca.media_item_id AND ca.is_default = 1 AND ca.local_path IS NOT NULL
		WHERE ca.id IS NULL
		  AND mt.name IN ('movie', 'tv_show', 'tv_episode', 'video')
		  AND f.name NOT LIKE '%.srt'
		  AND f.name NOT LIKE '%.sub'
		  AND f.name NOT LIKE '%.nfo'
		  AND f.name NOT LIKE '%.txt'
		  AND f.name NOT LIKE '%.jpg'
		  AND f.name NOT LIKE '%.png'
		ORDER BY mi.id
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, limit)
	if err != nil {
		return 0, fmt.Errorf("failed to query missing thumbnails: %w", err)
	}
	defer rows.Close()

	var tasks []thumbnailTask
	for rows.Next() {
		var t thumbnailTask
		if err := rows.Scan(&t.mediaItemID, &t.videoPath, &t.protocol, &t.rootPath, &t.host, &t.port, &t.username, &t.password, &t.domain, &t.url, &t.mountPoint, &t.options); err == nil {
			tasks = append(tasks, t)
		}
	}

	generated := 0
	for _, t := range tasks {
		resolvedPath := t.videoPath
		isTemp := false

		if t.protocol == "local" && t.rootPath.Valid && t.rootPath.String != "" {
			resolvedPath = filepath.Join(t.rootPath.String, t.videoPath)
		} else if t.protocol != "local" && s.fsFactory != nil {
			// Copy remote file to temp location for ffmpeg
			tempPath, err := s.copyRemoteFileToTemp(ctx, t)
			if err != nil {
				s.logger.Warn("Failed to copy remote file for thumbnail generation",
					zap.Int64("media_item_id", t.mediaItemID),
					zap.String("path", t.videoPath),
					zap.Error(err))
				continue
			}
			resolvedPath = tempPath
			isTemp = true
		}

		thumb, err := s.generateAndSaveVideoThumbnail(ctx, t.mediaItemID, resolvedPath)
		if isTemp {
			_ = os.Remove(resolvedPath)
		}
		if err != nil {
			s.logger.Warn("Failed to generate video thumbnail",
				zap.Int64("media_item_id", t.mediaItemID),
				zap.String("path", t.videoPath),
				zap.Error(err))
			continue
		}
		if thumb != nil {
			generated++
		}
	}

	return generated, nil
}

// thumbnailTask holds the data needed to generate a video thumbnail.
type thumbnailTask struct {
	mediaItemID int64
	videoPath   string
	protocol    string
	rootPath    sql.NullString
	host        sql.NullString
	port        sql.NullInt64
	username    sql.NullString
	password    sql.NullString
	domain      sql.NullString
	url         sql.NullString
	mountPoint  sql.NullString
	options     sql.NullString
}

// storageRoot rebuilds the connection fields of the task's storage root, the input of the single settings mapping.
func (t thumbnailTask) storageRoot() *models.StorageRoot {
	str := func(n sql.NullString) *string {
		if !n.Valid {
			return nil
		}
		v := n.String
		return &v
	}
	root := &models.StorageRoot{Protocol: t.protocol, Host: str(t.host), Path: str(t.rootPath), Username: str(t.username), Password: str(t.password),
		Domain: str(t.domain), URL: str(t.url), MountPoint: str(t.mountPoint), Options: str(t.options)}
	if t.port.Valid {
		p := int(t.port.Int64)
		root.Port = &p
	}
	return root
}

// copyRemoteFileToTemp copies a remote file (SMB, FTP, etc.) to a temporary
// local path so ffmpeg can process it.
func (s *CoverArtService) copyRemoteFileToTemp(ctx context.Context, t thumbnailTask) (string, error) {
	// The same single mapping every other CreateClient caller uses (filesystem.SettingsFromRoot). The hand-built SMB-shaped map this replaced
	// carried share/domain/username/password for EVERY protocol, which the strict factory refuses for ftp, nfs and webdav (WF22 R1).
	settings := filesystem.SettingsFromRoot(t.storageRoot(), nil)

	config := &filesystem.StorageConfig{
		Protocol: t.protocol,
		Settings: settings,
	}

	client, err := s.fsFactory.CreateClient(config)
	if err != nil {
		return "", fmt.Errorf("failed to create filesystem client: %w", err)
	}
	if err := client.Connect(ctx); err != nil {
		return "", fmt.Errorf("failed to connect filesystem client: %w", err)
	}
	defer client.Disconnect(ctx)

	reader, err := client.ReadFile(ctx, t.videoPath)
	if err != nil {
		return "", fmt.Errorf("failed to read remote file: %w", err)
	}
	defer reader.Close()

	tmpFile, err := os.CreateTemp(s.cacheDir, "video_thumb_*.tmp")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, reader); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to copy remote file to temp: %w", err)
	}

	return tmpFile.Name(), nil
}

// generateAndSaveVideoThumbnail creates a single thumbnail for a video file
// and persists it to the database.
func (s *CoverArtService) generateAndSaveVideoThumbnail(ctx context.Context, mediaItemID int64, videoPath string) (*CoverArt, error) {
	duration, err := s.getVideoDuration(videoPath)
	if err != nil {
		return nil, err
	}

	timestamp := duration * 0.25
	if timestamp < 5 {
		timestamp = 5
	}

	coverID := fmt.Sprintf("video_thumb_%d", mediaItemID)
	now := time.Now()
	outputPath := filepath.Join(s.cacheDir, coverID+".jpg")

	// Fast seek: place -ss before -i
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-ss", fmt.Sprintf("%.2f", timestamp),
		"-i", videoPath,
		"-vframes", "1",
		"-q:v", "2",
		"-y",
		outputPath,
	)
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg failed: %w", err)
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		return nil, fmt.Errorf("thumbnail file not created: %w", err)
	}

	var storageKey string
	var size int64

	if s.HasObjectStore() {
		storageKey = "covers/" + coverID + ".jpg"
		file, err := os.Open(outputPath)
		if err != nil {
			_ = os.Remove(outputPath)
			return nil, fmt.Errorf("failed to open thumbnail: %w", err)
		}
		size = info.Size()
		putErr := s.store.PutObject(ctx, s.bucket, storageKey, file, size,
			object.WithContentType("image/jpeg"),
			object.WithMetadata(map[string]string{"media_item_id": strconv.FormatInt(mediaItemID, 10), "source": "video_thumbnail"}),
		)
		file.Close()
		if putErr != nil {
			s.logger.Warn("Failed to upload video thumbnail to object store, falling back to local cache",
				zap.Error(putErr))
			storageKey = outputPath
		}
		// Keep local copy as fallback; could be cleaned up later.
	} else {
		storageKey = outputPath
		size = info.Size()
	}

	coverArt := &CoverArt{
		ID:          coverID,
		MediaItemID: mediaItemID,
		Source:      "video_thumbnail",
		LocalPath:   &storageKey,
		Format:      "jpeg",
		Size:        &size,
		Quality:     "high",
		CreatedAt:   now,
		CachedAt:    &now,
	}

	if err := s.saveCoverArtToDB(ctx, coverArt); err != nil {
		return nil, fmt.Errorf("failed to save thumbnail to database: %w", err)
	}
	if err := s.setDefaultCoverArt(ctx, mediaItemID, coverArt.ID); err != nil {
		s.logger.Warn("Failed to set default cover art", zap.Error(err))
	}

	return coverArt, nil
}

// GetCoverURL returns a cover image URL for any media item.
// Tries in order:
//  1. Cached cover art from cover_art table (local file takes priority)
//  2. External metadata cover URL routed through the backend image-proxy
//  3. Type-specific placeholder SVG
func (s *CoverArtService) GetCoverURL(ctx context.Context, mediaItemID int64, mediaTypeName string) string {
	// 1. Check if we have locally cached cover art
	if s.db != nil {
		var localPath sql.NullString
		err := s.db.QueryRowContext(ctx,
			`SELECT local_path FROM cover_art
			 WHERE media_item_id = ? AND is_default = 1 AND local_path IS NOT NULL AND local_path != ''
			 ORDER BY created_at DESC LIMIT 1`, mediaItemID).Scan(&localPath)
		if err == nil && localPath.Valid && localPath.String != "" {
			return fmt.Sprintf("/api/v1/cover/%d", mediaItemID)
		}
	}

	// 2. Check external_metadata for a cover URL and route through our proxy
	// so client apps never talk directly to external CDNs.
	if s.db != nil {
		var coverURL sql.NullString
		err := s.db.QueryRowContext(ctx,
			`SELECT cover_url FROM external_metadata
			 WHERE media_item_id = ? AND cover_url IS NOT NULL AND cover_url != ''
			 ORDER BY last_fetched DESC LIMIT 1`, mediaItemID).Scan(&coverURL)
		if err == nil && coverURL.Valid && coverURL.String != "" {
			if strings.HasPrefix(coverURL.String, "/api/v1/image-proxy") {
				return coverURL.String
			}
			encoded := url.QueryEscape(coverURL.String)
			proxyURL := fmt.Sprintf("/api/v1/image-proxy?url=%s", encoded)
			// Kick off background caching so future requests serve locally.
			go func(itemID int64, extURL string) {
				cacheCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()
				if _, cacheErr := s.CacheExternalCoverArt(cacheCtx, itemID, extURL); cacheErr != nil {
					s.logger.Debug("Background cover art caching failed",
						zap.Int64("media_item_id", itemID),
						zap.Error(cacheErr))
				}
			}(mediaItemID, coverURL.String)
			return proxyURL
		}
	}

	// 3. Fall back to type-specific placeholder SVG
	return fmt.Sprintf("/api/v1/cover/placeholder/%s", mediaTypeName)
}

// GetCoverURLsBatch returns cover image URLs for multiple media items at once.
// This is more efficient than calling GetCoverURL in a loop because it batches
// the database queries. All returned URLs are backend-routed; no raw external
// CDN URLs are ever sent to clients.
func (s *CoverArtService) GetCoverURLsBatch(ctx context.Context, items []CoverURLRequest) map[int64]string {
	result := make(map[int64]string, len(items))
	if len(items) == 0 {
		return result
	}

	// Set placeholder defaults for all items first
	itemTypeMap := make(map[int64]string, len(items))
	for _, item := range items {
		itemTypeMap[item.ID] = item.MediaTypeName
		result[item.ID] = fmt.Sprintf("/api/v1/cover/placeholder/%s", item.MediaTypeName)
	}

	if s.db == nil {
		return result
	}

	// Build IN clause for IDs
	ids := make([]interface{}, len(items))
	placeholders := make([]string, len(items))
	for i, item := range items {
		ids[i] = item.ID
		placeholders[i] = "?"
	}
	inClause := strings.Join(placeholders, ",")

	// 1. Batch query external_metadata cover URLs and route through proxy
	query := fmt.Sprintf(
		`SELECT media_item_id, cover_url FROM external_metadata
		 WHERE media_item_id IN (%s) AND cover_url IS NOT NULL AND cover_url != ''
		 ORDER BY last_fetched DESC`, inClause)

	rows, err := s.db.QueryContext(ctx, query, ids...)
	if err == nil {
		defer rows.Close()
		seen := make(map[int64]bool)
		for rows.Next() {
			var itemID int64
			var coverURL string
			if err := rows.Scan(&itemID, &coverURL); err == nil && !seen[itemID] {
				if strings.HasPrefix(coverURL, "/api/v1/image-proxy") {
					result[itemID] = coverURL
				} else {
					result[itemID] = fmt.Sprintf("/api/v1/image-proxy?url=%s", url.QueryEscape(coverURL))
				}
				seen[itemID] = true
			}
		}
	}

	// 2. Batch query locally cached cover_art (overrides external_metadata)
	query = fmt.Sprintf(
		`SELECT media_item_id, local_path FROM cover_art
		 WHERE media_item_id IN (%s) AND is_default = 1 AND local_path IS NOT NULL AND local_path != ''
		 ORDER BY created_at DESC`, inClause)

	rows2, err := s.db.QueryContext(ctx, query, ids...)
	if err == nil {
		defer rows2.Close()
		seen := make(map[int64]bool)
		for rows2.Next() {
			var itemID int64
			var localPath sql.NullString
			if err := rows2.Scan(&itemID, &localPath); err == nil && !seen[itemID] {
				if localPath.Valid && localPath.String != "" {
					result[itemID] = fmt.Sprintf("/api/v1/cover/%d", itemID)
					seen[itemID] = true
				}
			}
		}
	}

	return result
}

// CoverURLRequest is used by GetCoverURLsBatch to identify items and their types.
type CoverURLRequest struct {
	ID            int64
	MediaTypeName string
}

// GeneratePlaceholderSVG generates a cinematic placeholder SVG image for a given media type.
// The design mimics streaming-service placeholder cards with rich gradients,
// subtle texture, and large type-friendly icons.
func GeneratePlaceholderSVG(mediaType string) []byte {
    gradients := map[string][2]string{
        "movie":        {"#1a1a2e", "#16213e"},
        "tv_show":      {"#2d132c", "#801336"},
        "tv_season":    {"#1b1b2f", "#4a4e69"},
        "tv_episode":   {"#1b1b2f", "#4a4e69"},
        "music_artist": {"#0f3460", "#533483"},
        "music_album":  {"#1a1a2e", "#16213e"},
        "song":         {"#0f3460", "#e94560"},
        "game":         {"#1a1a2e", "#0f3460"},
        "software":     {"#2c3e50", "#3498db"},
        "book":         {"#2d3436", "#636e72"},
        "comic":        {"#2d132c", "#c0392b"},
    }

    accentColors := map[string]string{
        "movie":        "#e94560",
        "tv_show":      "#ff6b6b",
        "tv_season":    "#74b9ff",
        "tv_episode":   "#74b9ff",
        "music_artist": "#a29bfe",
        "music_album":  "#fd79a8",
        "song":         "#00b894",
        "game":         "#f39c12",
        "software":     "#00cec9",
        "book":         "#fab1a0",
        "comic":        "#e17055",
    }

    icons := map[string]string{
        "movie":        "&#127916;",
        "tv_show":      "&#128250;",
        "tv_season":    "&#128250;",
        "tv_episode":   "&#128250;",
        "music_artist": "&#127908;",
        "music_album":  "&#128191;",
        "song":         "&#127925;",
        "game":         "&#127918;",
        "software":     "&#128187;",
        "book":         "&#128214;",
        "comic":        "&#128214;",
    }

    labels := map[string]string{
        "movie":        "MOVIE",
        "tv_show":      "TV SHOW",
        "tv_season":    "SEASON",
        "tv_episode":   "EPISODE",
        "music_artist": "ARTIST",
        "music_album":  "ALBUM",
        "song":         "SONG",
        "game":         "GAME",
        "software":     "SOFTWARE",
        "book":         "BOOK",
        "comic":        "COMIC",
    }

    g, ok := gradients[mediaType]
    if !ok {
        g = [2]string{"#1a1a2e", "#16213e"}
    }
    accent, ok := accentColors[mediaType]
    if !ok {
        accent = "#e94560"
    }
    icon, ok := icons[mediaType]
    if !ok {
        icon = "&#128196;"
    }
    label, ok := labels[mediaType]
    if !ok {
        label = strings.ReplaceAll(strings.ToUpper(mediaType), "_", " ")
    }

    svg := fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" width="300" height="450" viewBox="0 0 300 450">
  <defs>
    <linearGradient id="bg" x1="0%%" y1="0%%" x2="0%%" y2="100%%">
      <stop offset="0%%" stop-color="%s"/>
      <stop offset="60%%" stop-color="%s"/>
      <stop offset="100%%" stop-color="#0d0d0d"/>
    </linearGradient>
    <linearGradient id="glow" x1="0%%" y1="0%%" x2="100%%" y2="100%%">
      <stop offset="0%%" stop-color="%s" stop-opacity="0.35"/>
      <stop offset="100%%" stop-color="%s" stop-opacity="0.05"/>
    </linearGradient>
    <filter id="shadow">
      <feDropShadow dx="0" dy="4" stdDeviation="6" flood-color="black" flood-opacity="0.4"/>
    </filter>
  </defs>
  <rect width="300" height="450" rx="8" fill="url(#bg)"/>
  <path d="M0 450 L300 0 L300 450 Z" fill="url(#glow)" opacity="0.6"/>
  <rect x="0" y="0" width="300" height="4" rx="8" fill="%s"/>
  <text x="150" y="200" font-size="96" text-anchor="middle" dominant-baseline="middle" fill="white" opacity="0.95" filter="url(#shadow)">%s</text>
  <text x="150" y="300" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="16" font-weight="700" letter-spacing="3" text-anchor="middle" dominant-baseline="middle" fill="%s">%s</text>
  <line x1="100" y1="330" x2="200" y2="330" stroke="white" stroke-width="1" stroke-opacity="0.2"/>
</svg>`, g[0], g[1], accent, accent, accent, icon, accent, label)

    return []byte(svg)
}
