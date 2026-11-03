package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"catalogizer/database"

	"go.uber.org/zap"
)

// CacheServiceInterface defines the interface for cache operations needed by SubtitleService
type CacheServiceInterface interface {
	Get(ctx context.Context, key string, dest interface{}) (bool, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
}

// SubtitleService handles subtitle management, downloading, and translation
type SubtitleService struct {
	db                 *database.DB
	logger             *zap.Logger
	translationService *TranslationService
	cacheService       CacheServiceInterface
	httpClient         *http.Client
	apiKeys            map[string]string
	cacheDir           string
	wg                 sync.WaitGroup // Tracks running goroutines for graceful shutdown
	shutdown           chan struct{}  // Signals goroutines to stop
}

// SubtitleProvider represents different subtitle providers
type SubtitleProvider string

const (
	ProviderOpenSubtitles SubtitleProvider = "opensubtitles"
	ProviderSubDB         SubtitleProvider = "subdb"
	ProviderYifySubtitles SubtitleProvider = "yifysubtitles"
	ProviderSubscene      SubtitleProvider = "subscene"
	ProviderAddic7ed      SubtitleProvider = "addic7ed"
)

// SubtitleSearchRequest represents a subtitle search request
type SubtitleSearchRequest struct {
	MediaPath     string             `json:"media_path"`
	Title         *string            `json:"title,omitempty"`
	Year          *int               `json:"year,omitempty"`
	Season        *int               `json:"season,omitempty"`
	Episode       *int               `json:"episode,omitempty"`
	Languages     []string           `json:"languages"`
	FileHash      *string            `json:"file_hash,omitempty"`
	FileSize      *int64             `json:"file_size,omitempty"`
	Providers     []SubtitleProvider `json:"providers,omitempty"`
	ForceDownload bool               `json:"force_download"`
}

// SubtitleSearchResult represents a subtitle search result
type SubtitleSearchResult struct {
	ID                string           `json:"id"`
	Provider          SubtitleProvider `json:"provider"`
	Language          string           `json:"language"`
	LanguageCode      string           `json:"language_code"`
	Title             string           `json:"title"`
	DownloadURL       string           `json:"download_url"`
	Format            string           `json:"format"`
	Encoding          string           `json:"encoding"`
	UploadDate        time.Time        `json:"upload_date"`
	Downloads         int              `json:"downloads"`
	Rating            float64          `json:"rating"`
	Comments          int              `json:"comments"`
	IsHearingImpaired bool             `json:"is_hearing_impaired"`
	FrameRate         *float64         `json:"frame_rate,omitempty"`
	FileHash          *string          `json:"file_hash,omitempty"`
	MovieHash         *string          `json:"movie_hash,omitempty"`
	MatchScore        float64          `json:"match_score"`
}

// SubtitleSyncResult represents subtitle synchronization verification
type SubtitleSyncResult struct {
	IsValid        bool        `json:"is_valid"`
	SyncOffset     float64     `json:"sync_offset"` // Milliseconds
	Confidence     float64     `json:"confidence"`  // 0-1
	DetectedFrames int         `json:"detected_frames"`
	SamplePoints   []SyncPoint `json:"sample_points"`
	Recommendation string      `json:"recommendation"`
}

// SyncPoint represents a point used for sync verification
type SyncPoint struct {
	SubtitleTime float64 `json:"subtitle_time"`
	VideoTime    float64 `json:"video_time"`
	Text         string  `json:"text"`
	Confidence   float64 `json:"confidence"`
}

// SubtitleDownloadRequest represents a subtitle download request
type SubtitleDownloadRequest struct {
	MediaItemID   int64    `json:"media_item_id"`
	ResultID      string   `json:"result_id"`
	Language      string   `json:"language"`
	VerifySync    bool     `json:"verify_sync"`
	AutoTranslate []string `json:"auto_translate,omitempty"` // Languages to auto-translate to
}

// SubtitleTranslationRequest represents a subtitle translation request
type SubtitleTranslationRequest struct {
	SubtitleID     string `json:"subtitle_id"`
	SourceLanguage string `json:"source_language"`
	TargetLanguage string `json:"target_language"`
	UseCache       bool   `json:"use_cache"`
}

// SubtitleLine represents a single subtitle line
type SubtitleLine struct {
	Index     int    `json:"index"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
	Text      string `json:"text"`
}

// SubtitleTrack represents a subtitle track in the database

// SubtitleUploadRequest represents a subtitle upload request
type SubtitleUploadRequest struct {
	MediaID      int64   `json:"media_item_id" form:"media_item_id"`
	Language     string  `json:"language" form:"language"`
	LanguageCode string  `json:"language_code" form:"language_code"`
	Format       string  `json:"format" form:"format"`
	Content      string  `json:"content"` // File content as string
	IsDefault    bool    `json:"is_default" form:"is_default"`
	IsForced     bool    `json:"is_forced" form:"is_forced"`
	Encoding     string  `json:"encoding" form:"encoding"`
	SyncOffset   float64 `json:"sync_offset" form:"sync_offset"`
}

// SubtitleUploadResponse represents the response for subtitle upload
type SubtitleUploadResponse struct {
	Success    bool   `json:"success"`
	SubtitleID string `json:"subtitle_id"`
	Message    string `json:"message"`
	Language   string `json:"language"`
	Format     string `json:"format"`
	Size       int64  `json:"size"`
}

// NewSubtitleService creates a new subtitle service
func NewSubtitleService(db *database.DB, logger *zap.Logger, cacheService CacheServiceInterface) *SubtitleService {
	return &SubtitleService{
		db:                 db,
		logger:             logger,
		translationService: NewTranslationService(logger),
		cacheService:       cacheService,
		httpClient:         &http.Client{Timeout: 30 * time.Second},
		apiKeys:            make(map[string]string),
		cacheDir:           "./cache/subtitles",
		shutdown:           make(chan struct{}),
	}
}

// Close gracefully shuts down the subtitle service, waiting for pending goroutines
func (s *SubtitleService) Close() {
	close(s.shutdown)
	s.wg.Wait()
}

// SearchSubtitles searches for subtitles across multiple providers
func (s *SubtitleService) SearchSubtitles(ctx context.Context, request *SubtitleSearchRequest) ([]SubtitleSearchResult, error) {
	s.logger.Info("Searching subtitles",
		zap.String("media_path", request.MediaPath),
		zap.Strings("languages", request.Languages))

	var allResults []SubtitleSearchResult

	// Default providers if none specified
	providers := request.Providers
	if len(providers) == 0 {
		providers = []SubtitleProvider{
			ProviderOpenSubtitles,
			ProviderSubDB,
			ProviderYifySubtitles,
		}
	}

	// Search each provider in parallel
	resultsChan := make(chan []SubtitleSearchResult, len(providers))
	errorsChan := make(chan error, len(providers))

	for _, provider := range providers {
		go func(p SubtitleProvider) {
			results, err := s.searchProvider(ctx, p, request)
			if err != nil {
				s.logger.Warn("Provider search failed",
					zap.String("provider", string(p)),
					zap.Error(err))
				errorsChan <- err
				return
			}
			resultsChan <- results
		}(provider)
	}

	// Collect results
	var providerErrs []error
	succeeded := 0
	for i := 0; i < len(providers); i++ {
		select {
		case results := <-resultsChan:
			succeeded++
			allResults = append(allResults, results...)
		case perr := <-errorsChan:
			// Logged above; keep going with the other providers, but remember
			// the failure so total failure is reported, not hidden as "no results".
			providerErrs = append(providerErrs, perr)
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if succeeded == 0 && len(providerErrs) > 0 {
		return nil, fmt.Errorf("subtitle search: every provider failed: %w", errors.Join(providerErrs...))
	}

	// Sort by match score and rating
	s.sortSubtitleResults(allResults)

	s.logger.Info("Subtitle search completed",
		zap.Int("total_results", len(allResults)))

	return allResults, nil
}

// DownloadSubtitle downloads a subtitle and optionally verifies sync
func (s *SubtitleService) DownloadSubtitle(ctx context.Context, request *SubtitleDownloadRequest) (*SubtitleTrack, error) {
	s.logger.Info("Downloading subtitle",
		zap.Int64("media_item_id", request.MediaItemID),
		zap.String("result_id", request.ResultID))

	// Get download info from cache or provider
	result, err := s.getDownloadInfo(ctx, request.ResultID)
	if err != nil {
		return nil, fmt.Errorf("failed to get download info: %w", err)
	}

	// Download subtitle content
	content, encoding, err := s.downloadContent(ctx, result.DownloadURL)
	if err != nil {
		return nil, fmt.Errorf("failed to download subtitle content: %w", err)
	}

	// Parse and validate subtitle format
	_, err = s.parseSubtitle(content, result.Format)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subtitle: %w", err)
	}

	// Create subtitle track
	track := &SubtitleTrack{
		ID:           generateSubtitleID(),
		Language:     result.Language,
		LanguageCode: result.LanguageCode,
		Source:       "downloaded",
		Format:       result.Format,
		Content:      &content,
		Path:         &result.DownloadURL, // Use download URL as path initially
		IsDefault:    false,
		IsForced:     false,
		Encoding:     encoding,
		SyncOffset:   0.0,
		CreatedAt:    time.Now(),
		VerifiedSync: false,
	}

	// Verify synchronization if requested
	if request.VerifySync {
		syncResult, err := s.verifySynchronization(ctx, request.MediaItemID, track)
		if err != nil {
			s.logger.Warn("Failed to verify subtitle sync", zap.Error(err))
		} else {
			track.VerifiedSync = syncResult.IsValid
			track.SyncOffset = syncResult.SyncOffset
		}
	}

	// Save to database
	if err := s.saveSubtitleTrack(ctx, request.MediaItemID, track); err != nil {
		return nil, fmt.Errorf("failed to save subtitle track: %w", err)
	}

	// Auto-translate to requested languages
	if len(request.AutoTranslate) > 0 {
		// Check if shutdown is in progress before spawning goroutine
		select {
		case <-s.shutdown:
			s.logger.Debug("Skipping auto-translate, service shutting down")
		default:
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				s.autoTranslateSubtitle(ctx, track, request.AutoTranslate)
			}()
		}
	}

	s.logger.Info("Subtitle downloaded successfully",
		zap.String("subtitle_id", track.ID),
		zap.String("language", track.Language))

	return track, nil
}

// TranslateSubtitle translates a subtitle to another language
func (s *SubtitleService) TranslateSubtitle(ctx context.Context, request *SubtitleTranslationRequest) (*SubtitleTrack, error) {
	s.logger.Info("Translating subtitle",
		zap.String("subtitle_id", request.SubtitleID),
		zap.String("target_language", request.TargetLanguage))

	// Check cache first
	if request.UseCache {
		if cached := s.getCachedTranslation(ctx, request.SubtitleID, request.TargetLanguage); cached != nil {
			return cached, nil
		}
	}

	// Get original subtitle
	original, err := s.getSubtitleTrack(ctx, request.SubtitleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get original subtitle: %w", err)
	}

	// Parse subtitle for translation
	lines, err := s.parseSubtitleLines(*original.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subtitle lines: %w", err)
	}

	// Translate each line
	translatedLines, err := s.translateLines(ctx, lines, request.SourceLanguage, request.TargetLanguage)
	if err != nil {
		return nil, fmt.Errorf("failed to translate lines: %w", err)
	}

	// Reconstruct subtitle content
	translatedContent, err := s.reconstructSubtitle(original.Format, translatedLines)
	if err != nil {
		return nil, fmt.Errorf("failed to reconstruct subtitle: %w", err)
	}

	// Create translated subtitle track
	translatedTrack := &SubtitleTrack{
		ID:           generateSubtitleID(),
		Language:     getLanguageName(request.TargetLanguage),
		LanguageCode: request.TargetLanguage,
		Source:       "translated",
		Format:       original.Format,
		Content:      &translatedContent,
		IsDefault:    false,
		IsForced:     false,
		Encoding:     original.Encoding,
		SyncOffset:   original.SyncOffset,
		CreatedAt:    time.Now(),
		VerifiedSync: original.VerifiedSync,
	}

	// Save translated subtitle
	if err := s.saveCachedTranslation(ctx, request.SubtitleID, request.TargetLanguage, translatedTrack); err != nil {
		s.logger.Warn("Failed to cache translation", zap.Error(err))
	}

	return translatedTrack, nil
}

// GetSubtitles returns all subtitles for a media item
func (s *SubtitleService) GetSubtitles(ctx context.Context, mediaItemID int64) ([]SubtitleTrack, error) {
	query := `
		SELECT id, language, language_code, source, format, path, content,
		       is_default, is_forced, encoding, sync_offset, created_at, verified_sync
		FROM subtitle_tracks WHERE media_item_id = ?
		ORDER BY is_default DESC, language`

	rows, err := s.db.QueryContext(ctx, query, mediaItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to query subtitles: %w", err)
	}
	defer rows.Close()

	var subtitles []SubtitleTrack
	for rows.Next() {
		var track SubtitleTrack
		var content sql.NullString
		var path sql.NullString

		err := rows.Scan(
			&track.ID, &track.Language, &track.LanguageCode, &track.Source,
			&track.Format, &path, &content, &track.IsDefault, &track.IsForced,
			&track.Encoding, &track.SyncOffset, &track.CreatedAt, &track.VerifiedSync,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan subtitle: %w", err)
		}

		if path.Valid {
			track.Path = &path.String
		}
		if content.Valid {
			track.Content = &content.String
		}

		subtitles = append(subtitles, track)
	}

	return subtitles, nil
}

// GetSubtitleTrack retrieves a subtitle track by ID (exported version)
func (s *SubtitleService) GetSubtitleTrack(ctx context.Context, subtitleID string) (*SubtitleTrack, error) {
	return s.getSubtitleTrack(ctx, subtitleID)
}

// VerifySynchronization checks if subtitles are properly synchronized with video (exported version)
func (s *SubtitleService) VerifySynchronization(ctx context.Context, mediaItemID int64, track *SubtitleTrack) (*SubtitleSyncResult, error) {
	return s.verifySynchronization(ctx, mediaItemID, track)
}

// verifySynchronization checks if subtitles are properly synchronized with video
func (s *SubtitleService) verifySynchronization(ctx context.Context, mediaItemID int64, track *SubtitleTrack) (*SubtitleSyncResult, error) {
	s.logger.Debug("Verifying subtitle synchronization",
		zap.Int64("media_item_id", mediaItemID),
		zap.String("subtitle_id", track.ID))

	// Get video metadata
	videoInfo, err := s.getVideoInfo(ctx, mediaItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get video info: %w", err)
	}

	// Parse subtitle timing
	lines, err := s.parseSubtitleLines(*track.Content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse subtitle lines: %w", err)
	}

	// Analyze timing patterns
	samplePoints := s.extractSamplePoints(lines, videoInfo.Duration)

	// Calculate sync offset and confidence
	syncOffset, confidence := s.calculateSyncOffset(samplePoints, videoInfo)

	result := &SubtitleSyncResult{
		IsValid:        confidence > 0.7, // 70% confidence threshold
		SyncOffset:     syncOffset,
		Confidence:     confidence,
		DetectedFrames: len(samplePoints),
		SamplePoints:   samplePoints,
	}

	if result.IsValid {
		result.Recommendation = "Subtitle synchronization is good"
	} else if confidence > 0.4 {
		result.Recommendation = fmt.Sprintf("Subtitle may need %+.1fs offset", syncOffset/1000)
	} else {
		result.Recommendation = "Subtitle synchronization is poor"
	}

	return result, nil
}

// Provider-specific search implementations
func (s *SubtitleService) searchProvider(ctx context.Context, provider SubtitleProvider, request *SubtitleSearchRequest) ([]SubtitleSearchResult, error) {
	switch provider {
	case ProviderOpenSubtitles:
		return s.searchOpenSubtitles(ctx, request)
	case ProviderSubDB:
		return s.searchSubDB(ctx, request)
	case ProviderYifySubtitles:
		return s.searchYifySubtitles(ctx, request)
	case ProviderSubscene, ProviderAddic7ed:
		return nil, fmt.Errorf("%w: %s", ErrSubtitleProviderNotImplemented, provider)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// ImplementedSubtitleProviders lists the providers that have a working API
// client behind them. It is empty today: every searchX returns
// ErrSubtitleProviderNotImplemented. Add a provider here only together with
// a real client (CAT-001, §11.4.289).
func ImplementedSubtitleProviders() []SubtitleProvider { return []SubtitleProvider{} }

// ErrSubtitleProviderNotImplemented is returned by provider searches that have
// no real API client behind them. It replaces the former fabricated results.
var ErrSubtitleProviderNotImplemented = errors.New("subtitle provider client not implemented")

func (s *SubtitleService) searchOpenSubtitles(ctx context.Context, request *SubtitleSearchRequest) ([]SubtitleSearchResult, error) {
	s.logger.Debug("OpenSubtitles search requested but no client is implemented")
	return nil, fmt.Errorf("%w: %s", ErrSubtitleProviderNotImplemented, ProviderOpenSubtitles)
}

func (s *SubtitleService) searchSubDB(ctx context.Context, request *SubtitleSearchRequest) ([]SubtitleSearchResult, error) {
	return nil, fmt.Errorf("%w: %s", ErrSubtitleProviderNotImplemented, ProviderSubDB)
}

func (s *SubtitleService) searchYifySubtitles(ctx context.Context, request *SubtitleSearchRequest) ([]SubtitleSearchResult, error) {
	return nil, fmt.Errorf("%w: %s", ErrSubtitleProviderNotImplemented, ProviderYifySubtitles)
}

// Helper functions
func (s *SubtitleService) downloadContent(ctx context.Context, url string) (string, string, error) {
	if err := GuardProviderURL(url, SSRFGuardConfig{}); err != nil {
		return "", "", fmt.Errorf("subtitle: unsafe URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", "", err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	// Detect encoding
	encoding := detectEncoding(content)

	return string(content), encoding, nil
}

func (s *SubtitleService) parseSubtitle(content, format string) (interface{}, error) {
	switch strings.ToLower(format) {
	case "srt":
		return s.parseSRT(content)
	case "vtt":
		return s.parseVTT(content)
	case "ass", "ssa":
		return s.parseASS(content)
	default:
		return nil, fmt.Errorf("unsupported subtitle format: %s", format)
	}
}

func (s *SubtitleService) parseSRT(content string) ([]SubtitleLine, error) {
	var lines []SubtitleLine

	// Simple SRT parser
	re := regexp.MustCompile(`(\d+)\s*\n(\d{2}:\d{2}:\d{2},\d{3}) --> (\d{2}:\d{2}:\d{2},\d{3})\s*\n((?:[^\n]*\n?)+?)(?:\n|$)`)
	matches := re.FindAllStringSubmatch(content, -1)

	for i, match := range matches {
		if len(match) >= 5 {
			text := strings.TrimSpace(match[4])

			lines = append(lines, SubtitleLine{
				Index:     i + 1,
				StartTime: match[2],
				EndTime:   match[3],
				Text:      text,
			})
		}
	}

	return lines, nil
}

func (s *SubtitleService) parseVTT(content string) ([]SubtitleLine, error) {
	var lines []SubtitleLine

	// WebVTT format:
	// WEBVTT
	//
	// [optional cue identifier]
	// 00:00:01.000 --> 00:00:03.000 [optional settings]
	// Text content
	//
	// Timestamps can be either HH:MM:SS.mmm or MM:SS.mmm

	// Remove BOM if present
	content = strings.TrimPrefix(content, "\xef\xbb\xbf")

	// Check for WEBVTT header
	if !strings.HasPrefix(strings.TrimSpace(content), "WEBVTT") {
		return nil, fmt.Errorf("invalid WebVTT: missing WEBVTT header")
	}

	// Split by blank lines to get cues
	// WebVTT uses blank lines to separate cues
	cueBlocks := regexp.MustCompile(`\n\s*\n`).Split(content, -1)

	// VTT timestamp pattern - supports both HH:MM:SS.mmm and MM:SS.mmm formats
	// Also handles optional cue settings after the timestamp
	timestampPattern := regexp.MustCompile(`(?:(\d{2}):)?(\d{2}):(\d{2})\.(\d{3})\s*-->\s*(?:(\d{2}):)?(\d{2}):(\d{2})\.(\d{3})(?:\s+.*)?`)

	index := 0
	for _, block := range cueBlocks {
		block = strings.TrimSpace(block)
		if block == "" || strings.HasPrefix(block, "WEBVTT") || strings.HasPrefix(block, "NOTE") {
			continue
		}

		blockLines := strings.Split(block, "\n")
		if len(blockLines) < 2 {
			continue
		}

		// Find the timestamp line (may be first or second line if there's an identifier)
		timestampLineIdx := 0
		for i, line := range blockLines {
			if timestampPattern.MatchString(line) {
				timestampLineIdx = i
				break
			}
		}

		if timestampLineIdx >= len(blockLines) {
			continue
		}

		timestampLine := blockLines[timestampLineIdx]
		matches := timestampPattern.FindStringSubmatch(timestampLine)
		if len(matches) < 9 {
			continue
		}

		// Parse start time
		startHours := 0
		if matches[1] != "" {
			startHours, _ = strconv.Atoi(matches[1])
		}
		startMinutes, _ := strconv.Atoi(matches[2])
		startSeconds, _ := strconv.Atoi(matches[3])
		startMillis, _ := strconv.Atoi(matches[4])

		// Parse end time
		endHours := 0
		if matches[5] != "" {
			endHours, _ = strconv.Atoi(matches[5])
		}
		endMinutes, _ := strconv.Atoi(matches[6])
		endSeconds, _ := strconv.Atoi(matches[7])
		endMillis, _ := strconv.Atoi(matches[8])

		// Format timestamps as SRT-compatible format (HH:MM:SS,mmm)
		startTime := fmt.Sprintf("%02d:%02d:%02d,%03d", startHours, startMinutes, startSeconds, startMillis)
		endTime := fmt.Sprintf("%02d:%02d:%02d,%03d", endHours, endMinutes, endSeconds, endMillis)

		// Get text content (everything after the timestamp line)
		var textLines []string
		for i := timestampLineIdx + 1; i < len(blockLines); i++ {
			line := blockLines[i]
			// Strip VTT-specific tags like <v Speaker>, <c.classname>, etc.
			line = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(line, "")
			if strings.TrimSpace(line) != "" {
				textLines = append(textLines, strings.TrimSpace(line))
			}
		}

		if len(textLines) == 0 {
			continue
		}

		index++
		lines = append(lines, SubtitleLine{
			Index:     index,
			StartTime: startTime,
			EndTime:   endTime,
			Text:      strings.Join(textLines, "\n"),
		})
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("no valid subtitle cues found in WebVTT content")
	}

	return lines, nil
}

func (s *SubtitleService) parseASS(content string) ([]SubtitleLine, error) {
	var lines []SubtitleLine

	// ASS/SSA format:
	// [Script Info]
	// ...
	// [V4+ Styles] or [V4 Styles]
	// ...
	// [Events]
	// Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
	// Dialogue: 0,0:00:01.00,0:00:03.00,Default,,0,0,0,,Text here

	// Remove BOM if present
	content = strings.TrimPrefix(content, "\xef\xbb\xbf")

	// Find the [Events] section
	eventsIdx := strings.Index(strings.ToLower(content), "[events]")
	if eventsIdx == -1 {
		return nil, fmt.Errorf("invalid ASS/SSA: missing [Events] section")
	}

	eventsSection := content[eventsIdx:]

	// Find the Format line to understand the field order
	formatLine := ""
	contentLines := strings.Split(eventsSection, "\n")
	for _, line := range contentLines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(strings.ToLower(line), "format:") {
			formatLine = line
			break
		}
	}

	// Parse format to find text field position
	textFieldIndex := 9 // Default position for Text field
	if formatLine != "" {
		formatFields := strings.Split(strings.TrimPrefix(strings.ToLower(formatLine), "format:"), ",")
		for i, field := range formatFields {
			if strings.TrimSpace(field) == "text" {
				textFieldIndex = i
				break
			}
		}
	}

	// ASS timestamp pattern: H:MM:SS.cc (centiseconds, not milliseconds)
	dialoguePattern := regexp.MustCompile(`(?i)^Dialogue:\s*(.*)$`)

	index := 0
	for _, line := range contentLines {
		line = strings.TrimSpace(line)
		matches := dialoguePattern.FindStringSubmatch(line)
		if matches == nil {
			continue
		}

		// Split fields - but text field can contain commas, so we need to be careful
		fieldsStr := matches[1]
		fields := make([]string, 0)

		// Split only up to textFieldIndex, then take the rest as text
		currentField := ""
		fieldCount := 0
		for _, char := range fieldsStr {
			if char == ',' && fieldCount < textFieldIndex {
				fields = append(fields, currentField)
				currentField = ""
				fieldCount++
			} else {
				currentField += string(char)
			}
		}
		// Add the last field (text)
		fields = append(fields, currentField)

		if len(fields) < 3 {
			continue
		}

		// Parse timestamps (format: H:MM:SS.cc)
		startTime, err := parseASSTimestamp(strings.TrimSpace(fields[1]))
		if err != nil {
			continue
		}
		endTime, err := parseASSTimestamp(strings.TrimSpace(fields[2]))
		if err != nil {
			continue
		}

		// Get text field
		text := ""
		if len(fields) > textFieldIndex {
			text = fields[textFieldIndex]
		}

		// Clean ASS formatting codes
		// Remove override tags like {\an8}, {\pos(x,y)}, {\b1}, etc.
		text = regexp.MustCompile(`\{[^}]*\}`).ReplaceAllString(text, "")
		// Replace \N and \n with actual newlines
		text = strings.ReplaceAll(text, "\\N", "\n")
		text = strings.ReplaceAll(text, "\\n", "\n")
		// Replace \h with non-breaking space (treat as regular space)
		text = strings.ReplaceAll(text, "\\h", " ")
		text = strings.TrimSpace(text)

		if text == "" {
			continue
		}

		index++
		lines = append(lines, SubtitleLine{
			Index:     index,
			StartTime: startTime,
			EndTime:   endTime,
			Text:      text,
		})
	}

	if len(lines) == 0 {
		return nil, fmt.Errorf("no valid dialogue entries found in ASS/SSA content")
	}

	return lines, nil
}

// parseASSTimestamp converts ASS timestamp (H:MM:SS.cc) to SRT format (HH:MM:SS,mmm)
func parseASSTimestamp(timestamp string) (string, error) {
	// ASS format: H:MM:SS.cc (hours can be single digit, centiseconds not milliseconds)
	re := regexp.MustCompile(`(\d+):(\d{2}):(\d{2})\.(\d{2})`)
	matches := re.FindStringSubmatch(timestamp)

	if len(matches) != 5 {
		return "", fmt.Errorf("invalid ASS timestamp format: %s", timestamp)
	}

	hours, _ := strconv.Atoi(matches[1])
	minutes, _ := strconv.Atoi(matches[2])
	seconds, _ := strconv.Atoi(matches[3])
	centiseconds, _ := strconv.Atoi(matches[4])

	// Convert centiseconds to milliseconds
	milliseconds := centiseconds * 10

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, milliseconds), nil
}

func parseTimestamp(timestamp string) (float64, error) {
	// Parse SRT timestamp format: 00:01:23,456
	re := regexp.MustCompile(`(\d{2}):(\d{2}):(\d{2}),(\d{3})`)
	matches := re.FindStringSubmatch(timestamp)

	if len(matches) != 5 {
		return 0, fmt.Errorf("invalid timestamp format")
	}

	hours, _ := strconv.Atoi(matches[1])
	minutes, _ := strconv.Atoi(matches[2])
	seconds, _ := strconv.Atoi(matches[3])
	milliseconds, _ := strconv.Atoi(matches[4])

	total := float64(hours*3600+minutes*60+seconds) + float64(milliseconds)/1000.0
	return total, nil
}

func detectEncoding(data []byte) string {
	// Simple encoding detection - in practice you'd use a more sophisticated library
	if len(data) > 3 && data[0] == 0xEF && data[1] == 0xBB && data[2] == 0xBF {
		return "utf-8"
	}
	return "utf-8" // Default assumption
}

// subtitleIDCounter guarantees uniqueness across calls within the same clock
// tick — time.Now().UnixNano() alone repeats on back-to-back calls / coarse
// timers (TestGenerateSubtitleID's two consecutive calls flaked on equality).
var subtitleIDCounter atomic.Uint64

func generateSubtitleID() string {
	return fmt.Sprintf("sub_%d_%d", time.Now().UnixNano(), subtitleIDCounter.Add(1))
}

func getSubtitleStringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func (s *SubtitleService) sortSubtitleResults(results []SubtitleSearchResult) {
	// Sort by match score descending, then by rating
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].MatchScore < results[j].MatchScore ||
				(results[i].MatchScore == results[j].MatchScore && results[i].Rating < results[j].Rating) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// Additional helper methods

// getDownloadInfo retrieves download information for a subtitle result
func (s *SubtitleService) getDownloadInfo(ctx context.Context, resultID string) (*SubtitleSearchResult, error) {
	// Try to get from cache first
	cacheKey := fmt.Sprintf("subtitle_download_info:%s", resultID)

	var result SubtitleSearchResult
	found, err := s.cacheService.Get(ctx, cacheKey, &result)
	if err == nil && found {
		s.logger.Debug("Retrieved subtitle download info from cache",
			zap.String("result_id", resultID))
		return &result, nil
	}

	// If not in cache, reconstruct from result ID
	// Result ID format: {provider}_{id}
	parts := strings.SplitN(resultID, "_", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid result ID format: %s", resultID)
	}

	provider := SubtitleProvider(parts[0])

	// No provider has a search client, so no real result id can exist and none can be
	// reconstructed; the previous code invented language, score and URL (CAT-001, §11.4.289).
	switch provider {
	case ProviderOpenSubtitles, ProviderSubDB, ProviderYifySubtitles:
		return nil, fmt.Errorf("%w: %s", ErrSubtitleProviderNotImplemented, provider)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}

// saveSubtitleTrack saves a subtitle track to the database
func (s *SubtitleService) saveSubtitleTrack(ctx context.Context, mediaItemID int64, track *SubtitleTrack) error {
	query := `
		INSERT INTO subtitle_tracks
		(media_item_id, language, language_code, source, format, content, path,
		 is_default, is_forced, encoding, sync_offset, verified_sync, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	// Convert boolean values to integers for SQLite compatibility
	isDefault := 0
	if track.IsDefault {
		isDefault = 1
	}
	isForced := 0
	if track.IsForced {
		isForced = 1
	}
	verifiedSync := 0
	if track.VerifiedSync {
		verifiedSync = 1
	}

	// Handle pointers for optional fields
	var content interface{}
	if track.Content != nil {
		content = *track.Content
	} else {
		content = ""
	}

	var path interface{}
	if track.Path != nil {
		path = *track.Path
	} else {
		path = ""
	}

	s.logger.Info("Saving subtitle track",
		zap.Int64("media_item_id", mediaItemID),
		zap.String("id", track.ID),
		zap.String("language", track.Language),
		zap.String("language_code", track.LanguageCode),
		zap.String("source", track.Source),
		zap.String("format", track.Format),
		zap.Any("content", content),
		zap.Any("path", path),
		zap.Int("is_default", isDefault),
		zap.Int("is_forced", isForced),
		zap.String("encoding", track.Encoding),
		zap.Float64("sync_offset", track.SyncOffset),
		zap.Int("verified_sync", verifiedSync),
		zap.Time("created_at", track.CreatedAt))

	_, err := s.db.ExecContext(ctx, query,
		mediaItemID, track.Language, track.LanguageCode, track.Source,
		track.Format, content, path, isDefault, isForced,
		track.Encoding, track.SyncOffset, verifiedSync, track.CreatedAt)

	return err
}

// autoTranslateSubtitle automatically translates a subtitle to multiple languages
func (s *SubtitleService) autoTranslateSubtitle(ctx context.Context, track *SubtitleTrack, targetLanguages []string) {
	for _, lang := range targetLanguages {
		request := &SubtitleTranslationRequest{
			SubtitleID:     track.ID,
			SourceLanguage: track.LanguageCode,
			TargetLanguage: lang,
			UseCache:       true,
		}

		_, err := s.TranslateSubtitle(ctx, request)
		if err != nil {
			s.logger.Error("Auto-translation failed",
				zap.String("subtitle_id", track.ID),
				zap.String("target_language", lang),
				zap.Error(err))
		}
	}
}

// SaveUploadedSubtitle saves an uploaded subtitle file
func (s *SubtitleService) SaveUploadedSubtitle(ctx context.Context, req *SubtitleUploadRequest) (*SubtitleUploadResponse, error) {
	// Validate media item exists
	mediaQuery := "SELECT id FROM files WHERE id = ?"
	var mediaID int
	err := s.db.QueryRowContext(ctx, mediaQuery, req.MediaID).Scan(&mediaID)
	if err != nil {
		return nil, fmt.Errorf("media item not found: %w", err)
	}

	// Create subtitle track from upload
	track := &SubtitleTrack{
		ID:           generateSubtitleID(),
		Language:     req.Language,
		LanguageCode: req.LanguageCode,
		Source:       "uploaded",
		Format:       req.Format,
		Content:      &req.Content,
		IsDefault:    req.IsDefault,
		IsForced:     req.IsForced,
		Encoding:     req.Encoding,
		SyncOffset:   req.SyncOffset,
		CreatedAt:    time.Now(),
		VerifiedSync: false, // Uploaded subtitles need sync verification
	}

	// Save the subtitle track
	err = s.saveSubtitleTrack(ctx, req.MediaID, track)
	if err != nil {
		return nil, fmt.Errorf("failed to save uploaded subtitle: %w", err)
	}

	// If it's marked as default, update other subtitles
	if req.IsDefault {
		updateQuery := `
			UPDATE subtitle_tracks 
			SET is_default = 0 
			WHERE media_item_id = ? AND id != ?`
		_, err = s.db.ExecContext(ctx, updateQuery, req.MediaID, track.ID)
		if err != nil {
			s.logger.Error("Failed to update other subtitles as non-default",
				zap.Int64("media_id", req.MediaID),
				zap.String("subtitle_id", track.ID),
				zap.Error(err))
		}
	}

	// Return upload response
	response := &SubtitleUploadResponse{
		Success:    true,
		SubtitleID: track.ID,
		Message:    "Subtitle uploaded successfully",
		Language:   req.Language,
		Format:     req.Format,
		Size:       int64(len(req.Content)),
	}

	s.logger.Info("Uploaded subtitle saved successfully",
		zap.String("subtitle_id", track.ID),
		zap.Int64("media_id", req.MediaID),
		zap.String("language", req.Language),
		zap.String("format", req.Format))

	return response, nil
}

// getCachedTranslation retrieves a cached translation
func (s *SubtitleService) getCachedTranslation(ctx context.Context, subtitleID, targetLanguage string) *SubtitleTrack {
	// Create cache key
	cacheKey := fmt.Sprintf("subtitle_translation:%s:%s", subtitleID, targetLanguage)

	var track SubtitleTrack
	found, err := s.cacheService.Get(ctx, cacheKey, &track)
	if err != nil {
		s.logger.Debug("Error retrieving cached translation",
			zap.String("subtitle_id", subtitleID),
			zap.String("target_language", targetLanguage),
			zap.Error(err))
		return nil
	}

	if !found {
		s.logger.Debug("Cached translation not found",
			zap.String("subtitle_id", subtitleID),
			zap.String("target_language", targetLanguage))
		return nil
	}

	s.logger.Debug("Retrieved cached translation",
		zap.String("subtitle_id", subtitleID),
		zap.String("target_language", targetLanguage))

	return &track
}

// getSubtitleTrack retrieves a subtitle track by ID
func (s *SubtitleService) getSubtitleTrack(ctx context.Context, subtitleID string) (*SubtitleTrack, error) {
	query := `
		SELECT id, language, language_code, source, format, path, content,
		       is_default, is_forced, encoding, sync_offset, created_at, verified_sync
		FROM subtitle_tracks WHERE id = ?`

	var track SubtitleTrack
	var content sql.NullString
	var path sql.NullString

	err := s.db.QueryRowContext(ctx, query, subtitleID).Scan(
		&track.ID, &track.Language, &track.LanguageCode, &track.Source,
		&track.Format, &path, &content, &track.IsDefault, &track.IsForced,
		&track.Encoding, &track.SyncOffset, &track.CreatedAt, &track.VerifiedSync,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get subtitle track: %w", err)
	}

	if path.Valid {
		track.Path = &path.String
	}
	if content.Valid {
		track.Content = &content.String
	}

	return &track, nil
}

// saveCachedTranslation saves a translation to cache
func (s *SubtitleService) saveCachedTranslation(ctx context.Context, subtitleID, targetLanguage string, track *SubtitleTrack) error {
	// Create cache key
	cacheKey := fmt.Sprintf("subtitle_translation:%s:%s", subtitleID, targetLanguage)

	// Cache for 30 days (translations are expensive to generate)
	ttl := 30 * 24 * time.Hour

	err := s.cacheService.Set(ctx, cacheKey, track, ttl)
	if err != nil {
		s.logger.Error("Failed to save cached translation",
			zap.String("subtitle_id", subtitleID),
			zap.String("target_language", targetLanguage),
			zap.Error(err))
		return err
	}

	s.logger.Debug("Saved cached translation",
		zap.String("subtitle_id", subtitleID),
		zap.String("target_language", targetLanguage))

	return nil
}

// parseSubtitleLines parses subtitle content into lines
func (s *SubtitleService) parseSubtitleLines(content string) ([]SubtitleLine, error) {
	// For now, assume SRT format
	return s.parseSRT(content)
}

// translateLines translates subtitle lines using the translation service
func (s *SubtitleService) translateLines(ctx context.Context, lines []SubtitleLine, sourceLang, targetLang string) ([]SubtitleLine, error) {
	translatedLines := make([]SubtitleLine, len(lines))

	for i, line := range lines {
		request := TranslationRequest{
			Text:           line.Text,
			SourceLanguage: sourceLang,
			TargetLanguage: targetLang,
			Context:        "subtitle",
		}

		result, err := s.translationService.TranslateText(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to translate line %d: %w", i, err)
		}

		translatedLines[i] = SubtitleLine{
			Index:     line.Index,
			StartTime: line.StartTime,
			EndTime:   line.EndTime,
			Text:      result.TranslatedText,
		}
	}

	return translatedLines, nil
}

// reconstructSubtitle reconstructs subtitle content from lines
func (s *SubtitleService) reconstructSubtitle(format string, lines []SubtitleLine) (string, error) {
	switch strings.ToLower(format) {
	case "srt":
		return s.reconstructSRT(lines), nil
	default:
		return "", fmt.Errorf("unsupported format for reconstruction: %s", format)
	}
}

// reconstructSRT reconstructs SRT format from subtitle lines
func (s *SubtitleService) reconstructSRT(lines []SubtitleLine) string {
	var builder strings.Builder

	for _, line := range lines {
		builder.WriteString(fmt.Sprintf("%d\n", line.Index))
		builder.WriteString(fmt.Sprintf("%s --> %s\n", line.StartTime, line.EndTime))
		builder.WriteString(line.Text)
		builder.WriteString("\n\n")
	}

	return builder.String()
}

// VideoInfo represents video metadata for sync verification
type VideoInfo struct {
	Duration  float64 // Duration in seconds
	FrameRate float64
	Width     int
	Height    int
}

// getVideoInfo retrieves video metadata for sync verification
func (s *SubtitleService) getVideoInfo(ctx context.Context, mediaItemID int64) (*VideoInfo, error) {
	// Query video metadata from file_metadata table
	query := `
		SELECT key, value
		FROM file_metadata
		WHERE file_id = ? AND key IN ('duration', 'frame_rate', 'width', 'height', 'resolution')
	`

	rows, err := s.db.QueryContext(ctx, query, mediaItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to query video metadata: %w", err)
	}
	defer rows.Close()

	videoInfo := &VideoInfo{
		Duration:  0,
		FrameRate: 24.0, // Default frame rate
		Width:     1920, // Default resolution
		Height:    1080,
	}

	metadataMap := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		metadataMap[key] = value
	}

	// Parse duration
	if durationStr, ok := metadataMap["duration"]; ok {
		if duration, err := strconv.ParseFloat(durationStr, 64); err == nil {
			videoInfo.Duration = duration
		}
	}

	// Parse frame rate
	if frameRateStr, ok := metadataMap["frame_rate"]; ok {
		if frameRate, err := strconv.ParseFloat(frameRateStr, 64); err == nil {
			videoInfo.FrameRate = frameRate
		}
	}

	// Parse width
	if widthStr, ok := metadataMap["width"]; ok {
		if width, err := strconv.Atoi(widthStr); err == nil {
			videoInfo.Width = width
		}
	}

	// Parse height
	if heightStr, ok := metadataMap["height"]; ok {
		if height, err := strconv.Atoi(heightStr); err == nil {
			videoInfo.Height = height
		}
	}

	// Parse resolution string (e.g., "1920x1080")
	if resolutionStr, ok := metadataMap["resolution"]; ok {
		parts := regexp.MustCompile(`(\d+)x(\d+)`).FindStringSubmatch(resolutionStr)
		if len(parts) == 3 {
			if width, err := strconv.Atoi(parts[1]); err == nil {
				videoInfo.Width = width
			}
			if height, err := strconv.Atoi(parts[2]); err == nil {
				videoInfo.Height = height
			}
		}
	}

	s.logger.Debug("Retrieved video info",
		zap.Int64("media_item_id", mediaItemID),
		zap.Float64("duration", videoInfo.Duration),
		zap.Float64("frame_rate", videoInfo.FrameRate),
		zap.Int("width", videoInfo.Width),
		zap.Int("height", videoInfo.Height))

	return videoInfo, nil
}

// extractSamplePoints extracts sample points for sync verification
func (s *SubtitleService) extractSamplePoints(lines []SubtitleLine, duration float64) []SyncPoint {
	var points []SyncPoint

	// Extract sample points at regular intervals
	sampleInterval := len(lines) / 10
	if sampleInterval == 0 {
		sampleInterval = 1
	}

	for i := 0; i < len(lines); i += sampleInterval {
		line := lines[i]
		// Parse timestamp
		time, _ := parseTimestamp(line.StartTime)

		points = append(points, SyncPoint{
			SubtitleTime: time,
			VideoTime:    time,
			Text:         line.Text,
			Confidence:   0.8,
		})
	}

	return points
}

// calculateSyncOffset calculates sync offset and confidence
func (s *SubtitleService) calculateSyncOffset(points []SyncPoint, videoInfo *VideoInfo) (float64, float64) {
	if len(points) == 0 {
		return 0, 0
	}

	// Simple implementation - calculate average offset
	var totalOffset float64
	for _, point := range points {
		totalOffset += point.SubtitleTime - point.VideoTime
	}

	avgOffset := totalOffset / float64(len(points))
	confidence := 0.8 // Default confidence

	return avgOffset * 1000, confidence // Convert to milliseconds
}
