package services

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"catalogizer/database"

	"go.uber.org/zap"
)

// Advanced reader service with Kindle/Moon Reader Pro-like experience
type ReaderService struct {
	db                  *database.DB
	logger              *zap.Logger
	cacheService        *CacheService
	translationService  *TranslationService
	localizationService *LocalizationService
}

// Reading session structure
type ReadingSession struct {
	ID              string          `json:"id"`
	UserID          int64           `json:"user_id"`
	BookID          string          `json:"book_id"`
	DeviceID        string          `json:"device_id"`
	DeviceName      string          `json:"device_name"`
	StartedAt       time.Time       `json:"started_at"`
	LastActiveAt    time.Time       `json:"last_active_at"`
	CurrentPosition ReadingPosition `json:"current_position"`
	ReadingSettings ReadingSettings `json:"reading_settings"`
	SyncStatus      SyncStatus      `json:"sync_status"`
	ReadingStats    ReadingStats    `json:"reading_stats"`
	IsActive        bool            `json:"is_active"`
}

// Reading position with multiple granularities
type ReadingPosition struct {
	BookID          string          `json:"book_id"`
	ChapterID       string          `json:"chapter_id,omitempty"`
	PageNumber      int             `json:"page_number"`
	WordOffset      int             `json:"word_offset"`
	CharacterOffset int             `json:"character_offset"`
	PercentComplete float64         `json:"percent_complete"`
	Location        string          `json:"location,omitempty"` // Kindle location equivalent
	CFI             string          `json:"cfi,omitempty"`      // EPUB Canonical Fragment Identifier
	Timestamp       time.Time       `json:"timestamp"`
	PositionContext PositionContext `json:"position_context"`
	Confidence      float64         `json:"confidence"`
}

type PositionContext struct {
	SurroundingText string `json:"surrounding_text"`
	ParagraphStart  string `json:"paragraph_start"`
	SentenceStart   string `json:"sentence_start"`
	ChapterTitle    string `json:"chapter_title,omitempty"`
	SectionTitle    string `json:"section_title,omitempty"`
}

// Reading settings for personalization
type ReadingSettings struct {
	FontFamily      string                `json:"font_family"`
	FontSize        int                   `json:"font_size"`
	LineHeight      float64               `json:"line_height"`
	TextAlign       string                `json:"text_align"`
	Theme           string                `json:"theme"`
	BackgroundColor string                `json:"background_color"`
	TextColor       string                `json:"text_color"`
	PageMargins     PageMargins           `json:"page_margins"`
	ColumnsPerPage  int                   `json:"columns_per_page"`
	PageTransition  string                `json:"page_transition"`
	AutoScroll      bool                  `json:"auto_scroll"`
	AutoScrollSpeed int                   `json:"auto_scroll_speed"`
	ReadingMode     string                `json:"reading_mode"` // day, night, sepia, etc.
	Brightness      float64               `json:"brightness"`
	BlueLight       BlueLightFilter       `json:"blue_light_filter"`
	Hyphenation     bool                  `json:"hyphenation"`
	Justification   bool                  `json:"justification"`
	StatusBar       StatusBarSettings     `json:"status_bar"`
	Gestures        GestureSettings       `json:"gestures"`
	Accessibility   AccessibilitySettings `json:"accessibility"`
}

type PageMargins struct {
	Top    int `json:"top"`
	Bottom int `json:"bottom"`
	Left   int `json:"left"`
	Right  int `json:"right"`
}

type BlueLightFilter struct {
	Enabled      bool    `json:"enabled"`
	Intensity    float64 `json:"intensity"`
	AutoSchedule bool    `json:"auto_schedule"`
	StartTime    string  `json:"start_time"`
	EndTime      string  `json:"end_time"`
}

type StatusBarSettings struct {
	Visible        bool   `json:"visible"`
	ShowProgress   bool   `json:"show_progress"`
	ShowTime       bool   `json:"show_time"`
	ShowBattery    bool   `json:"show_battery"`
	ShowPageNumber bool   `json:"show_page_number"`
	Position       string `json:"position"`
}

type GestureSettings struct {
	TapToTurn        bool     `json:"tap_to_turn"`
	SwipeToTurn      bool     `json:"swipe_to_turn"`
	VolumeKeys       bool     `json:"volume_keys"`
	TapZones         TapZones `json:"tap_zones"`
	SwipeSensitivity float64  `json:"swipe_sensitivity"`
}

type TapZones struct {
	LeftTurn   bool `json:"left_turn"`
	RightTurn  bool `json:"right_turn"`
	CenterMenu bool `json:"center_menu"`
}

type AccessibilitySettings struct {
	TextToSpeech    TTSSettings `json:"text_to_speech"`
	HighContrast    bool        `json:"high_contrast"`
	LargeText       bool        `json:"large_text"`
	ScreenReader    bool        `json:"screen_reader"`
	VoiceNavigation bool        `json:"voice_navigation"`
}

type TTSSettings struct {
	Enabled       bool    `json:"enabled"`
	Voice         string  `json:"voice"`
	Speed         float64 `json:"speed"`
	Pitch         float64 `json:"pitch"`
	AutoPlay      bool    `json:"auto_play"`
	HighlightText bool    `json:"highlight_text"`
}

// Sync status for cross-device reading
type SyncStatus struct {
	LastSyncAt      time.Time      `json:"last_sync_at"`
	IsSynced        bool           `json:"is_synced"`
	ConflictExists  bool           `json:"conflict_exists"`
	ConflictDetails []SyncConflict `json:"conflict_details,omitempty"`
	SyncVersion     int64          `json:"sync_version"`
}

type SyncConflict struct {
	DeviceID     string          `json:"device_id"`
	DeviceName   string          `json:"device_name"`
	Position     ReadingPosition `json:"position"`
	Timestamp    time.Time       `json:"timestamp"`
	ConflictType string          `json:"conflict_type"`
}

// Reading statistics and analytics
type ReadingStats struct {
	TotalReadingTime int64               `json:"total_reading_time_seconds"`
	SessionTime      int64               `json:"session_time_seconds"`
	PagesRead        int                 `json:"pages_read"`
	WordsRead        int                 `json:"words_read"`
	ReadingSpeed     float64             `json:"reading_speed_wpm"`
	AverageSpeed     float64             `json:"average_speed_wpm"`
	DailyGoal        int                 `json:"daily_goal_minutes"`
	DailyProgress    int                 `json:"daily_progress_minutes"`
	WeeklyStats      WeeklyReadingStats  `json:"weekly_stats"`
	MonthlyStats     MonthlyReadingStats `json:"monthly_stats"`
	ReadingStreak    int                 `json:"reading_streak_days"`
	LongestStreak    int                 `json:"longest_streak_days"`
	BooksCompleted   int                 `json:"books_completed"`
	PagesPerSession  float64             `json:"pages_per_session"`
}

type WeeklyReadingStats struct {
	Week          string `json:"week"`
	TotalTime     int64  `json:"total_time_seconds"`
	PagesRead     int    `json:"pages_read"`
	SessionsCount int    `json:"sessions_count"`
	DaysActive    int    `json:"days_active"`
}

type MonthlyReadingStats struct {
	Month          string  `json:"month"`
	TotalTime      int64   `json:"total_time_seconds"`
	PagesRead      int     `json:"pages_read"`
	BooksCompleted int     `json:"books_completed"`
	AverageDaily   float64 `json:"average_daily_minutes"`
}

// Bookmarks and annotations
type Bookmark struct {
	ID        string          `json:"id"`
	UserID    int64           `json:"user_id"`
	BookID    string          `json:"book_id"`
	Position  ReadingPosition `json:"position"`
	Title     string          `json:"title"`
	Note      string          `json:"note,omitempty"`
	Tags      []string        `json:"tags,omitempty"`
	Color     string          `json:"color,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
	IsPublic  bool            `json:"is_public"`
	ShareURL  string          `json:"share_url,omitempty"`
}

type Highlight struct {
	ID            string          `json:"id"`
	UserID        int64           `json:"user_id"`
	BookID        string          `json:"book_id"`
	StartPosition ReadingPosition `json:"start_position"`
	EndPosition   ReadingPosition `json:"end_position"`
	SelectedText  string          `json:"selected_text"`
	Note          string          `json:"note,omitempty"`
	Color         string          `json:"color"`
	Type          string          `json:"type"` // highlight, underline, strikethrough
	Tags          []string        `json:"tags,omitempty"`
	CreatedAt     time.Time       `json:"created_at"`
	UpdatedAt     time.Time       `json:"updated_at"`
	IsPublic      bool            `json:"is_public"`
	ShareURL      string          `json:"share_url,omitempty"`
}

type Annotation struct {
	ID          string          `json:"id"`
	UserID      int64           `json:"user_id"`
	BookID      string          `json:"book_id"`
	Position    ReadingPosition `json:"position"`
	Type        string          `json:"type"` // note, drawing, voice, image
	Content     string          `json:"content"`
	ContentType string          `json:"content_type"`
	ContentURL  string          `json:"content_url,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	IsPublic    bool            `json:"is_public"`
	ShareURL    string          `json:"share_url,omitempty"`
}

// Reading requests and responses
type StartReadingRequest struct {
	UserID                 int64            `json:"user_id"`
	BookID                 string           `json:"book_id"`
	DeviceInfo             DeviceInfo       `json:"device_info"`
	ReadingSettings        *ReadingSettings `json:"reading_settings,omitempty"`
	ResumeFromLastPosition bool             `json:"resume_from_last_position"`
}

type ReaderUpdatePositionRequest struct {
	SessionID   string          `json:"session_id"`
	Position    ReadingPosition `json:"position"`
	ReadingTime int64           `json:"reading_time_seconds"`
	PagesRead   int             `json:"pages_read"`
	WordsRead   int             `json:"words_read"`
	AutoSync    bool            `json:"auto_sync"`
}

type CreateBookmarkRequest struct {
	UserID   int64           `json:"user_id"`
	BookID   string          `json:"book_id"`
	Position ReadingPosition `json:"position"`
	Title    string          `json:"title"`
	Note     string          `json:"note,omitempty"`
	Tags     []string        `json:"tags,omitempty"`
	Color    string          `json:"color,omitempty"`
	IsPublic bool            `json:"is_public"`
}

type CreateHighlightRequest struct {
	UserID        int64           `json:"user_id"`
	BookID        string          `json:"book_id"`
	StartPosition ReadingPosition `json:"start_position"`
	EndPosition   ReadingPosition `json:"end_position"`
	SelectedText  string          `json:"selected_text"`
	Note          string          `json:"note,omitempty"`
	Color         string          `json:"color"`
	Type          string          `json:"type"`
	Tags          []string        `json:"tags,omitempty"`
	IsPublic      bool            `json:"is_public"`
}

type ReaderDeviceInfo struct {
	DeviceID   string `json:"device_id"`
	DeviceName string `json:"device_name"`
	DeviceType string `json:"device_type"`
	ScreenSize string `json:"screen_size,omitempty"`
	OS         string `json:"os,omitempty"`
	AppVersion string `json:"app_version,omitempty"`
}

// Book content structure for reading
type BookContent struct {
	BookID            string       `json:"book_id"`
	Format            string       `json:"format"` // epub, pdf, mobi, txt
	Chapters          []Chapter    `json:"chapters"`
	TableOfContents   []TOCEntry   `json:"table_of_contents"`
	Metadata          BookMetadata `json:"metadata"`
	TotalPages        int          `json:"total_pages"`
	TotalWords        int          `json:"total_words"`
	EstimatedReadTime int          `json:"estimated_read_time_minutes"`
}

type BookChapter struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Number      int       `json:"number"`
	Content     string    `json:"content"`
	HTMLContent string    `json:"html_content,omitempty"`
	WordCount   int       `json:"word_count"`
	PageCount   int       `json:"page_count"`
	StartPage   int       `json:"start_page"`
	EndPage     int       `json:"end_page"`
	Sections    []Section `json:"sections,omitempty"`
}

type Section struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	Content       string `json:"content"`
	Level         int    `json:"level"`
	StartPosition int    `json:"start_position"`
	EndPosition   int    `json:"end_position"`
}

func NewReaderService(
	db *database.DB,
	logger *zap.Logger,
	cacheService *CacheService,
	translationService *TranslationService,
	localizationService *LocalizationService,
) *ReaderService {
	return &ReaderService{
		db:                  db,
		logger:              logger,
		cacheService:        cacheService,
		translationService:  translationService,
		localizationService: localizationService,
	}
}

// Start a new reading session
func (s *ReaderService) StartReading(ctx context.Context, req *StartReadingRequest) (*ReadingSession, error) {
	s.logger.Info("Starting reading session",
		zap.Int64("user_id", req.UserID),
		zap.String("book_id", req.BookID),
		zap.String("device_id", req.DeviceInfo.DeviceID))

	// Generate session ID
	sessionID := s.generateSessionID(req.UserID, req.BookID, req.DeviceInfo.DeviceID)

	// Get last reading position if resuming
	var position ReadingPosition
	if req.ResumeFromLastPosition {
		if lastPos, err := s.getLastReadingPosition(ctx, req.UserID, req.BookID); err == nil {
			position = *lastPos
		}
	}

	// Get user's reading settings
	settings := s.getDefaultReadingSettings()
	if req.ReadingSettings != nil {
		settings = *req.ReadingSettings
	} else if userSettings, err := s.getUserReadingSettings(ctx, req.UserID); err == nil {
		settings = *userSettings
	}

	// Create reading session
	session := &ReadingSession{
		ID:              sessionID,
		UserID:          req.UserID,
		BookID:          req.BookID,
		DeviceID:        req.DeviceInfo.DeviceID,
		DeviceName:      req.DeviceInfo.DeviceName,
		StartedAt:       time.Now(),
		LastActiveAt:    time.Now(),
		CurrentPosition: position,
		ReadingSettings: settings,
		SyncStatus:      SyncStatus{IsSynced: true, SyncVersion: 1},
		ReadingStats:    s.initializeReadingStats(ctx, req.UserID, req.BookID),
		IsActive:        true,
	}

	// Store session in database
	if err := s.storeReadingSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to store reading session: %w", err)
	}

	// Update user's reading history
	s.updateReadingHistory(ctx, req.UserID, req.BookID, time.Now())

	s.logger.Info("Reading session started successfully",
		zap.String("session_id", sessionID),
		zap.Float64("resume_percent", position.PercentComplete))

	return session, nil
}

// Update reading position
func (s *ReaderService) UpdatePosition(ctx context.Context, req *ReaderUpdatePositionRequest) (*ReadingSession, error) {
	s.logger.Debug("Updating reading position",
		zap.String("session_id", req.SessionID),
		zap.Float64("percent_complete", req.Position.PercentComplete))

	// Get current session
	session, err := s.getReadingSession(ctx, req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get reading session: %w", err)
	}

	// Update position and stats
	session.CurrentPosition = req.Position
	session.LastActiveAt = time.Now()

	// Update reading statistics
	s.updateReadingStats(&session.ReadingStats, req.ReadingTime, req.PagesRead, req.WordsRead)

	// Handle auto-sync if enabled
	if req.AutoSync {
		if err := s.syncAcrossDevices(ctx, session); err != nil {
			s.logger.Warn("Failed to sync across devices", zap.Error(err))
		}
	}

	// Store updated session
	if err := s.storeReadingSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to update reading session: %w", err)
	}

	// Store position for future resume
	if err := s.storeReadingPosition(ctx, session.UserID, session.BookID, req.Position); err != nil {
		s.logger.Warn("Failed to store reading position", zap.Error(err))
	}

	return session, nil
}

// Create a bookmark
func (s *ReaderService) CreateBookmark(ctx context.Context, req *CreateBookmarkRequest) (*Bookmark, error) {
	s.logger.Info("Creating bookmark",
		zap.Int64("user_id", req.UserID),
		zap.String("book_id", req.BookID),
		zap.String("title", req.Title))

	bookmark := &Bookmark{
		ID:        s.generateBookmarkID(req.UserID, req.BookID, req.Position),
		UserID:    req.UserID,
		BookID:    req.BookID,
		Position:  req.Position,
		Title:     req.Title,
		Note:      req.Note,
		Tags:      req.Tags,
		Color:     req.Color,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		IsPublic:  req.IsPublic,
	}

	if req.IsPublic {
		bookmark.ShareURL = s.generateShareURL("bookmark", bookmark.ID)
	}

	// Store bookmark
	if err := s.storeBookmark(ctx, bookmark); err != nil {
		return nil, fmt.Errorf("failed to store bookmark: %w", err)
	}

	return bookmark, nil
}

// Create a highlight
func (s *ReaderService) CreateHighlight(ctx context.Context, req *CreateHighlightRequest) (*Highlight, error) {
	s.logger.Info("Creating highlight",
		zap.Int64("user_id", req.UserID),
		zap.String("book_id", req.BookID),
		zap.String("text", req.SelectedText[:min(50, len(req.SelectedText))]))

	highlight := &Highlight{
		ID:            s.generateHighlightID(req.UserID, req.BookID, req.StartPosition),
		UserID:        req.UserID,
		BookID:        req.BookID,
		StartPosition: req.StartPosition,
		EndPosition:   req.EndPosition,
		SelectedText:  req.SelectedText,
		Note:          req.Note,
		Color:         req.Color,
		Type:          req.Type,
		Tags:          req.Tags,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		IsPublic:      req.IsPublic,
	}

	if req.IsPublic {
		highlight.ShareURL = s.generateShareURL("highlight", highlight.ID)
	}

	// Store highlight
	if err := s.storeHighlight(ctx, highlight); err != nil {
		return nil, fmt.Errorf("failed to store highlight: %w", err)
	}

	return highlight, nil
}

// Get user's bookmarks for a book
func (s *ReaderService) GetBookmarks(ctx context.Context, userID int64, bookID string) ([]Bookmark, error) {
	query := `
		SELECT id, user_id, book_id, position_data, title, note, tags, color,
		       created_at, updated_at, is_public, share_url
		FROM reading_bookmarks
		WHERE user_id = ? AND book_id = ?
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, userID, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	bookmarks := []Bookmark{}
	for rows.Next() {
		var bookmark Bookmark
		var positionJSON, tagsJSON string
		var shareURL sql.NullString

		err := rows.Scan(
			&bookmark.ID, &bookmark.UserID, &bookmark.BookID,
			&positionJSON, &bookmark.Title, &bookmark.Note,
			&tagsJSON, &bookmark.Color, &bookmark.CreatedAt,
			&bookmark.UpdatedAt, &bookmark.IsPublic, &shareURL,
		)
		if err != nil {
			continue
		}

		// Parse position JSON
		if err := json.Unmarshal([]byte(positionJSON), &bookmark.Position); err != nil {
			continue
		}

		// Parse tags JSON
		if err := json.Unmarshal([]byte(tagsJSON), &bookmark.Tags); err != nil {
			bookmark.Tags = []string{}
		}

		if shareURL.Valid {
			bookmark.ShareURL = shareURL.String
		}

		bookmarks = append(bookmarks, bookmark)
	}

	return bookmarks, nil
}

// Get user's highlights for a book
func (s *ReaderService) GetHighlights(ctx context.Context, userID int64, bookID string) ([]Highlight, error) {
	query := `
		SELECT id, user_id, book_id, start_position_data, end_position_data,
		       selected_text, note, color, type, tags, created_at, updated_at,
		       is_public, share_url
		FROM reading_highlights
		WHERE user_id = ? AND book_id = ?
		ORDER BY created_at ASC
	`

	rows, err := s.db.QueryContext(ctx, query, userID, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	highlights := []Highlight{}
	for rows.Next() {
		var highlight Highlight
		var startPosJSON, endPosJSON, tagsJSON string
		var shareURL sql.NullString

		err := rows.Scan(
			&highlight.ID, &highlight.UserID, &highlight.BookID,
			&startPosJSON, &endPosJSON, &highlight.SelectedText,
			&highlight.Note, &highlight.Color, &highlight.Type,
			&tagsJSON, &highlight.CreatedAt, &highlight.UpdatedAt,
			&highlight.IsPublic, &shareURL,
		)
		if err != nil {
			continue
		}

		// Parse position JSONs
		json.Unmarshal([]byte(startPosJSON), &highlight.StartPosition)
		json.Unmarshal([]byte(endPosJSON), &highlight.EndPosition)

		// Parse tags JSON
		if err := json.Unmarshal([]byte(tagsJSON), &highlight.Tags); err != nil {
			highlight.Tags = []string{}
		}

		if shareURL.Valid {
			highlight.ShareURL = shareURL.String
		}

		highlights = append(highlights, highlight)
	}

	return highlights, nil
}

// Get reading statistics for a user
func (s *ReaderService) GetReadingStats(ctx context.Context, userID int64, period string) (*ReadingStats, error) {
	var stats ReadingStats

	// Get total reading statistics from reading_stats table
	query := `
		SELECT
			COALESCE(total_reading_time, 0) as total_time,
			COALESCE(pages_read, 0) as total_pages,
			COALESCE(words_read, 0) as total_words,
			COALESCE(average_speed, 0) as avg_speed,
			COALESCE(books_completed, 0) as books_read
		FROM reading_stats
		WHERE user_id = ?
	`

	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&stats.TotalReadingTime,
		&stats.PagesRead,
		&stats.WordsRead,
		&stats.AverageSpeed,
		&stats.BooksCompleted,
	)
	if err != nil {
		return nil, err
	}

	// Calculate reading streak
	stats.ReadingStreak = s.calculateReadingStreak(ctx, userID)

	// Get daily goal progress
	stats.DailyGoal = s.getUserDailyGoal(ctx, userID)
	stats.DailyProgress = s.getTodayReadingTime(ctx, userID)

	return &stats, nil
}

// Sync reading progress across devices
func (s *ReaderService) SyncAcrossDevices(ctx context.Context, userID int64, bookID string) error {
	s.logger.Info("Syncing reading progress across devices",
		zap.Int64("user_id", userID),
		zap.String("book_id", bookID))

	// Get all active sessions for this book
	sessions, err := s.getActiveSessionsForBook(ctx, userID, bookID)
	if err != nil {
		return err
	}

	if len(sessions) <= 1 {
		return nil // No sync needed
	}

	// Find the most recent position
	var latestSession *ReadingSession
	var latestTime time.Time

	for _, session := range sessions {
		if session.CurrentPosition.Timestamp.After(latestTime) {
			latestTime = session.CurrentPosition.Timestamp
			latestSession = &session
		}
	}

	if latestSession == nil {
		return nil
	}

	// Update all other sessions with the latest position
	for _, session := range sessions {
		if session.ID != latestSession.ID {
			session.CurrentPosition = latestSession.CurrentPosition
			session.SyncStatus.LastSyncAt = time.Now()
			session.SyncStatus.IsSynced = true
			session.SyncStatus.SyncVersion++

			if err := s.storeReadingSession(ctx, &session); err != nil {
				s.logger.Error("Failed to sync session",
					zap.String("session_id", session.ID),
					zap.Error(err))
			}
		}
	}

	return nil
}

// Helper methods
func (s *ReaderService) generateSessionID(userID int64, bookID, deviceID string) string {
	// CAT-LOGSHARE-TOK-1 class fix: the session id is the only capability for a reading session, so its tail is 128 random bits
	// (crypto/rand.Text), not the clock; the readable prefix is informational only.
	return fmt.Sprintf("session_%d_%s_%s_%s", userID, bookID, deviceID, rand.Text())
}

func (s *ReaderService) generateBookmarkID(userID int64, bookID string, position ReadingPosition) string {
	return fmt.Sprintf("bookmark_%d_%s_%d_%d", userID, bookID, position.PageNumber, time.Now().Unix())
}

func (s *ReaderService) generateHighlightID(userID int64, bookID string, position ReadingPosition) string {
	return fmt.Sprintf("highlight_%d_%s_%d_%d", userID, bookID, position.PageNumber, time.Now().Unix())
}

func (s *ReaderService) generateShareURL(itemType, itemID string) string {
	return fmt.Sprintf("https://catalogizer.com/share/%s/%s", itemType, itemID)
}

func (s *ReaderService) getDefaultReadingSettings() ReadingSettings {
	return ReadingSettings{
		FontFamily:      "serif",
		FontSize:        16,
		LineHeight:      1.5,
		TextAlign:       "justify",
		Theme:           "light",
		BackgroundColor: "#ffffff",
		TextColor:       "#000000",
		PageMargins:     PageMargins{Top: 20, Bottom: 20, Left: 15, Right: 15},
		ColumnsPerPage:  1,
		PageTransition:  "slide",
		AutoScroll:      false,
		AutoScrollSpeed: 5,
		ReadingMode:     "day",
		Brightness:      1.0,
		BlueLight:       BlueLightFilter{Enabled: false, Intensity: 0.3},
		Hyphenation:     true,
		Justification:   true,
		StatusBar:       StatusBarSettings{Visible: true, ShowProgress: true, Position: "bottom"},
		Gestures:        GestureSettings{TapToTurn: true, SwipeToTurn: true, VolumeKeys: false},
		Accessibility:   AccessibilitySettings{},
	}
}

func (s *ReaderService) getUserReadingSettings(ctx context.Context, userID int64) (*ReadingSettings, error) {
	query := `SELECT settings_data FROM user_reading_settings WHERE user_id = ?`

	var settingsJSON string
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&settingsJSON)
	if err != nil {
		return nil, err
	}

	var settings ReadingSettings
	if err := json.Unmarshal([]byte(settingsJSON), &settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

func (s *ReaderService) initializeReadingStats(ctx context.Context, userID int64, bookID string) ReadingStats {
	// Get existing stats or create new ones
	stats := ReadingStats{
		DailyGoal: s.getUserDailyGoal(ctx, userID),
	}

	// Calculate current streak
	stats.ReadingStreak = s.calculateReadingStreak(ctx, userID)

	return stats
}

func (s *ReaderService) updateReadingStats(stats *ReadingStats, readingTime int64, pages, words int) {
	stats.SessionTime += readingTime
	stats.TotalReadingTime += readingTime
	stats.PagesRead += pages
	stats.WordsRead += words

	// Calculate reading speed (words per minute)
	if readingTime > 0 {
		stats.ReadingSpeed = float64(words) / (float64(readingTime) / 60.0)
	}

	// Update daily progress
	stats.DailyProgress += int(readingTime / 60) // Convert to minutes
}

func (s *ReaderService) getLastReadingPosition(ctx context.Context, userID int64, bookID string) (*ReadingPosition, error) {
	query := `
		SELECT position_data
		FROM reading_positions
		WHERE user_id = ? AND book_id = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var positionJSON string
	err := s.db.QueryRowContext(ctx, query, userID, bookID).Scan(&positionJSON)
	if err != nil {
		return nil, err
	}

	var position ReadingPosition
	if err := json.Unmarshal([]byte(positionJSON), &position); err != nil {
		return nil, err
	}

	return &position, nil
}

func (s *ReaderService) storeReadingSession(ctx context.Context, session *ReadingSession) error {
	sessionJSON, _ := json.Marshal(session)
	positionJSON, _ := json.Marshal(session.CurrentPosition)
	settingsJSON, _ := json.Marshal(session.ReadingSettings)
	statsJSON, _ := json.Marshal(session.ReadingStats)

	query := `
		INSERT OR REPLACE INTO reading_sessions (
			id, user_id, book_id, device_id, device_name, started_at, last_active_at,
			current_position, reading_settings, reading_stats, is_active, session_data
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		session.ID, session.UserID, session.BookID, session.DeviceID, session.DeviceName,
		session.StartedAt, session.LastActiveAt, string(positionJSON), string(settingsJSON),
		string(statsJSON), session.IsActive, string(sessionJSON),
	)

	return err
}

func (s *ReaderService) getReadingSession(ctx context.Context, sessionID string) (*ReadingSession, error) {
	query := `SELECT session_data FROM reading_sessions WHERE id = ?`

	var sessionJSON string
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(&sessionJSON)
	if err != nil {
		return nil, err
	}

	var session ReadingSession
	if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
		return nil, err
	}

	return &session, nil
}

func (s *ReaderService) storeReadingPosition(ctx context.Context, userID int64, bookID string, position ReadingPosition) error {
	positionJSON, _ := json.Marshal(position)

	query := `
		INSERT OR REPLACE INTO reading_positions (
			user_id, book_id, position_data, page_number, percent_complete, timestamp
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		userID, bookID, string(positionJSON), position.PageNumber,
		position.PercentComplete, position.Timestamp,
	)

	return err
}

func (s *ReaderService) storeBookmark(ctx context.Context, bookmark *Bookmark) error {
	positionJSON, _ := json.Marshal(bookmark.Position)
	tagsJSON, _ := json.Marshal(bookmark.Tags)

	query := `
		INSERT OR REPLACE INTO reading_bookmarks (
			id, user_id, book_id, position_data, title, note, tags, color,
			created_at, updated_at, is_public, share_url
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		bookmark.ID, bookmark.UserID, bookmark.BookID, string(positionJSON),
		bookmark.Title, bookmark.Note, string(tagsJSON), bookmark.Color,
		bookmark.CreatedAt, bookmark.UpdatedAt, bookmark.IsPublic, bookmark.ShareURL,
	)

	return err
}

func (s *ReaderService) storeHighlight(ctx context.Context, highlight *Highlight) error {
	startPosJSON, _ := json.Marshal(highlight.StartPosition)
	endPosJSON, _ := json.Marshal(highlight.EndPosition)
	tagsJSON, _ := json.Marshal(highlight.Tags)

	query := `
		INSERT OR REPLACE INTO reading_highlights (
			id, user_id, book_id, start_position_data, end_position_data,
			selected_text, note, color, type, tags, created_at, updated_at,
			is_public, share_url
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := s.db.ExecContext(ctx, query,
		highlight.ID, highlight.UserID, highlight.BookID, string(startPosJSON), string(endPosJSON),
		highlight.SelectedText, highlight.Note, highlight.Color, highlight.Type, string(tagsJSON),
		highlight.CreatedAt, highlight.UpdatedAt, highlight.IsPublic, highlight.ShareURL,
	)

	return err
}

func (s *ReaderService) updateReadingHistory(ctx context.Context, userID int64, bookID string, timestamp time.Time) {
	query := `
		INSERT OR REPLACE INTO reading_history (
			user_id, book_id, last_read_at, read_count
		) VALUES (?, ?, ?, COALESCE((SELECT read_count FROM reading_history WHERE user_id = ? AND book_id = ?), 0) + 1)
	`

	s.db.ExecContext(ctx, query, userID, bookID, timestamp, userID, bookID)
}

func (s *ReaderService) syncAcrossDevices(ctx context.Context, session *ReadingSession) error {
	return s.SyncAcrossDevices(ctx, session.UserID, session.BookID)
}

func (s *ReaderService) getActiveSessionsForBook(ctx context.Context, userID int64, bookID string) ([]ReadingSession, error) {
	query := `SELECT session_data FROM reading_sessions WHERE user_id = ? AND book_id = ? AND is_active = 1`

	rows, err := s.db.QueryContext(ctx, query, userID, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []ReadingSession
	for rows.Next() {
		var sessionJSON string
		if err := rows.Scan(&sessionJSON); err != nil {
			continue
		}

		var session ReadingSession
		if err := json.Unmarshal([]byte(sessionJSON), &session); err != nil {
			continue
		}

		sessions = append(sessions, session)
	}

	return sessions, nil
}

func (s *ReaderService) calculateReadingStreak(ctx context.Context, userID int64) int {
	query := `
		SELECT DATE(last_read_at) as read_date
		FROM reading_history
		WHERE user_id = ?
		ORDER BY last_read_at DESC
	`

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return 0
	}
	defer rows.Close()

	streak := 0
	expectedDate := time.Now().Truncate(24 * time.Hour)

	for rows.Next() {
		var readDate time.Time
		if err := rows.Scan(&readDate); err != nil {
			continue
		}

		if readDate.Equal(expectedDate) {
			streak++
			expectedDate = expectedDate.AddDate(0, 0, -1)
		} else {
			break
		}
	}

	return streak
}

func (s *ReaderService) getUserDailyGoal(ctx context.Context, userID int64) int {
	query := `SELECT daily_goal_minutes FROM user_reading_goals WHERE user_id = ?`

	var goal int
	err := s.db.QueryRowContext(ctx, query, userID).Scan(&goal)
	if err != nil {
		return 30 // Default 30 minutes
	}

	return goal
}

func (s *ReaderService) getTodayReadingTime(ctx context.Context, userID int64) int {
	today := time.Now().Truncate(24 * time.Hour)
	tomorrow := today.AddDate(0, 0, 1)

	query := `
		SELECT COALESCE(SUM(session_time), 0) / 60 as minutes
		FROM reading_sessions
		WHERE user_id = ? AND started_at >= ? AND started_at < ?
	`

	var minutes int
	err := s.db.QueryRowContext(ctx, query, userID, today, tomorrow).Scan(&minutes)
	if err != nil {
		return 0
	}

	return minutes
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
