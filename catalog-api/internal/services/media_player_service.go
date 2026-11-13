package services

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"catalogizer/database"

	"go.uber.org/zap"
)

// MediaPlayerService handles all media playback functionality
type MediaPlayerService struct {
	db                 *database.DB
	logger             *zap.Logger
	cacheService       *CacheService
	lyricsService      *LyricsService
	subtitleService    *SubtitleService
	coverArtService    *CoverArtService
	translationService *TranslationService
	positionTracker    *PlaybackPositionService
	playlistService    *PlaylistService
}

// MediaItem represents a media file with all its metadata
type MediaItem struct {
	ID        int64     `json:"id" db:"id"`
	Path      string    `json:"path" db:"path"`
	Filename  string    `json:"filename" db:"filename"`
	Title     string    `json:"title" db:"title"`
	MediaType MediaType `json:"media_type" db:"media_type"`
	MimeType  string    `json:"mime_type" db:"mime_type"`
	Size      int64     `json:"size" db:"size"`
	Duration  *float64  `json:"duration,omitempty" db:"duration"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`

	// Music-specific metadata
	Artist      *string `json:"artist,omitempty" db:"artist"`
	Album       *string `json:"album,omitempty" db:"album"`
	AlbumArtist *string `json:"album_artist,omitempty" db:"album_artist"`
	Genre       *string `json:"genre,omitempty" db:"genre"`
	Year        *int    `json:"year,omitempty" db:"year"`
	TrackNumber *int    `json:"track_number,omitempty" db:"track_number"`
	DiscNumber  *int    `json:"disc_number,omitempty" db:"disc_number"`

	// Video-specific metadata
	VideoCodec *string  `json:"video_codec,omitempty" db:"video_codec"`
	AudioCodec *string  `json:"audio_codec,omitempty" db:"audio_codec"`
	Resolution *string  `json:"resolution,omitempty" db:"resolution"`
	Framerate  *float64 `json:"framerate,omitempty" db:"framerate"`
	Bitrate    *int64   `json:"bitrate,omitempty" db:"bitrate"`

	// TV Show/Series metadata
	SeriesTitle  *string `json:"series_title,omitempty" db:"series_title"`
	Season       *int    `json:"season,omitempty" db:"season"`
	Episode      *int    `json:"episode,omitempty" db:"episode"`
	EpisodeTitle *string `json:"episode_title,omitempty" db:"episode_title"`

	// Additional metadata
	Description *string         `json:"description,omitempty" db:"description"`
	Language    *string         `json:"language,omitempty" db:"language"`
	Subtitles   []SubtitleTrack `json:"subtitles,omitempty"`
	CoverArt    *CoverArt       `json:"cover_art,omitempty"`
	Lyrics      *LyricsData     `json:"lyrics,omitempty"`
	Chapters    []Chapter       `json:"chapters,omitempty"`

	// Playback metadata
	LastPosition *float64   `json:"last_position,omitempty" db:"last_position"`
	PlayCount    int        `json:"play_count" db:"play_count"`
	LastPlayed   *time.Time `json:"last_played,omitempty" db:"last_played"`
	IsFavorite   bool       `json:"is_favorite" db:"is_favorite"`
	Rating       *int       `json:"rating,omitempty" db:"rating"` // 1-5 stars

	// Cached external data
	ExternalData map[string]interface{} `json:"external_data,omitempty"`
}

// PlaybackSession represents an active playback session
type PlaybackSession struct {
	ID              string        `json:"id"`
	UserID          string        `json:"user_id"`
	MediaItem       *MediaItem    `json:"media_item"`
	PlaylistID      *string       `json:"playlist_id,omitempty"`
	CurrentPosition float64       `json:"current_position"`
	State           PlaybackState `json:"state"`
	Volume          float64       `json:"volume"`
	PlaybackRate    float64       `json:"playback_rate"`
	RepeatMode      RepeatMode    `json:"repeat_mode"`
	ShuffleEnabled  bool          `json:"shuffle_enabled"`
	StartedAt       time.Time     `json:"started_at"`
	UpdatedAt       time.Time     `json:"updated_at"`

	// Current subtitle and audio tracks
	CurrentSubtitle *SubtitleTrack `json:"current_subtitle,omitempty"`
	CurrentAudio    *AudioTrack    `json:"current_audio,omitempty"`

	// Player-specific settings
	PlayerSettings map[string]interface{} `json:"player_settings,omitempty"`
}

// SubtitleTrack represents a subtitle track
type SubtitleTrack struct {
	ID           string    `json:"id"`
	Language     string    `json:"language"`
	LanguageCode string    `json:"language_code"`
	Source       string    `json:"source"` // "embedded", "external", "downloaded"
	Format       string    `json:"format"` // "srt", "vtt", "ass", etc.
	Path         *string   `json:"path,omitempty"`
	Content      *string   `json:"content,omitempty"`
	IsDefault    bool      `json:"is_default"`
	IsForced     bool      `json:"is_forced"`
	Encoding     string    `json:"encoding"`
	SyncOffset   float64   `json:"sync_offset"` // Milliseconds offset for sync adjustment
	CreatedAt    time.Time `json:"created_at"`
	VerifiedSync bool      `json:"verified_sync"` // Whether sync has been verified
}

// AudioTrack represents an audio track
type AudioTrack struct {
	ID         string  `json:"id"`
	Language   string  `json:"language"`
	Codec      string  `json:"codec"`
	Channels   int     `json:"channels"`
	Bitrate    *int64  `json:"bitrate,omitempty"`
	SampleRate *int    `json:"sample_rate,omitempty"`
	IsDefault  bool    `json:"is_default"`
	Title      *string `json:"title,omitempty"`
}

// Chapter represents a chapter or bookmark in media
type Chapter struct {
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	StartTime float64  `json:"start_time"`
	EndTime   *float64 `json:"end_time,omitempty"`
	Thumbnail *string  `json:"thumbnail,omitempty"`
}

// CoverArt represents cover art metadata
type CoverArt struct {
	ID          string     `json:"id"`
	MediaItemID int64      `json:"media_item_id"`
	Source      string     `json:"source"` // "embedded", "local", "musicbrainz", "lastfm", etc.
	URL         *string    `json:"url,omitempty"`
	LocalPath   *string    `json:"local_path,omitempty"`
	Width       *int       `json:"width,omitempty"`
	Height      *int       `json:"height,omitempty"`
	Format      string     `json:"format"` // "jpeg", "png", "webp"
	Size        *int64     `json:"size,omitempty"`
	Quality     string     `json:"quality"` // "thumbnail", "medium", "high", "original"
	CreatedAt   time.Time  `json:"created_at"`
	CachedAt    *time.Time `json:"cached_at,omitempty"`
}

// LyricsData represents lyrics information
type LyricsData struct {
	ID           string            `json:"id"`
	MediaItemID  int64             `json:"media_item_id"`
	Source       string            `json:"source"` // "embedded", "genius", "musixmatch", etc.
	Language     string            `json:"language"`
	Content      string            `json:"content"`
	IsSynced     bool              `json:"is_synced"`
	SyncData     []LyricsLine      `json:"sync_data,omitempty"`
	Translations map[string]string `json:"translations,omitempty"` // language_code -> translated content
	CreatedAt    time.Time         `json:"created_at"`
	CachedAt     *time.Time        `json:"cached_at,omitempty"`
}

// LyricsLine represents a synchronized lyrics line
type LyricsLine struct {
	StartTime float64  `json:"start_time"`
	EndTime   *float64 `json:"end_time,omitempty"`
	Text      string   `json:"text"`
}

// PlaybackRequest represents a request to start playback
type PlaybackRequest struct {
	MediaItemID    int64                  `json:"media_item_id"`
	PlaylistID     *string                `json:"playlist_id,omitempty"`
	StartPosition  *float64               `json:"start_position,omitempty"`
	Volume         *float64               `json:"volume,omitempty"`
	PlaybackRate   *float64               `json:"playback_rate,omitempty"`
	SubtitleLang   *string                `json:"subtitle_lang,omitempty"`
	AudioTrackID   *string                `json:"audio_track_id,omitempty"`
	PlayerSettings map[string]interface{} `json:"player_settings,omitempty"`
}

// PlaybackUpdateRequest represents a request to update playback state
type PlaybackUpdateRequest struct {
	SessionID       string         `json:"session_id"`
	Position        *float64       `json:"position,omitempty"`
	State           *PlaybackState `json:"state,omitempty"`
	Volume          *float64       `json:"volume,omitempty"`
	PlaybackRate    *float64       `json:"playback_rate,omitempty"`
	RepeatMode      *RepeatMode    `json:"repeat_mode,omitempty"`
	ShuffleEnabled  *bool          `json:"shuffle_enabled,omitempty"`
	SubtitleTrackID *string        `json:"subtitle_track_id,omitempty"`
	AudioTrackID    *string        `json:"audio_track_id,omitempty"`
}

// NewMediaPlayerService creates a new media player service
func NewMediaPlayerService(db *database.DB, logger *zap.Logger) *MediaPlayerService {
	cacheService := NewCacheService(db, logger)
	return &MediaPlayerService{
		db:                 db,
		logger:             logger,
		cacheService:       cacheService,
		lyricsService:      NewLyricsService(db, logger),
		subtitleService:    NewSubtitleService(db, logger, cacheService),
		coverArtService:    NewCoverArtService(db, logger),
		translationService: NewTranslationService(logger),
		positionTracker:    NewPlaybackPositionService(db, logger),
		playlistService:    NewPlaylistService(db, logger),
	}
}

// Close gracefully shuts down the media player service and its internal cache service
func (s *MediaPlayerService) Close() {
	if s.cacheService != nil {
		s.cacheService.Close()
	}
}

// StartPlayback initiates playback of a media item
func (s *MediaPlayerService) StartPlayback(ctx context.Context, userID string, request *PlaybackRequest) (*PlaybackSession, error) {
	s.logger.Info("Starting playback",
		zap.String("user_id", userID),
		zap.Int64("media_item_id", request.MediaItemID))

	// Get media item
	mediaItem, err := s.GetMediaItem(ctx, request.MediaItemID)
	if err != nil {
		return nil, fmt.Errorf("failed to get media item: %w", err)
	}

	// Create playback session
	defaultZero := 0.0
	defaultOne := 1.0
	session := &PlaybackSession{
		ID:              generateSessionID(),
		UserID:          userID,
		MediaItem:       mediaItem,
		PlaylistID:      request.PlaylistID,
		CurrentPosition: getFloatValue(request.StartPosition, mediaItem.LastPosition, &defaultZero),
		State:           PlaybackStateLoading,
		Volume:          getFloatValue(request.Volume, nil, &defaultOne),
		PlaybackRate:    getFloatValue(request.PlaybackRate, nil, &defaultOne),
		RepeatMode:      RepeatModeOff,
		ShuffleEnabled:  false,
		StartedAt:       time.Now(),
		UpdatedAt:       time.Now(),
		PlayerSettings:  request.PlayerSettings,
	}

	// Load subtitles if video
	if mediaItem.MediaType == MediaTypeVideo {
		subtitles, err := s.subtitleService.GetSubtitles(ctx, mediaItem.ID)
		if err != nil {
			s.logger.Warn("Failed to load subtitles", zap.Error(err))
		} else {
			mediaItem.Subtitles = subtitles
			// Set default subtitle track
			if request.SubtitleLang != nil {
				session.CurrentSubtitle = s.findSubtitleByLanguage(subtitles, *request.SubtitleLang)
			}
		}
	}

	// Load lyrics if music
	if mediaItem.MediaType == MediaTypeMusic {
		lyrics, err := s.lyricsService.GetLyrics(ctx, mediaItem.ID)
		if err != nil {
			s.logger.Warn("Failed to load lyrics", zap.Error(err))
		} else {
			mediaItem.Lyrics = lyrics
		}
	}

	// Load cover art
	coverArt, err := s.coverArtService.GetCoverArt(ctx, mediaItem.ID)
	if err != nil {
		s.logger.Warn("Failed to load cover art", zap.Error(err))
	} else {
		mediaItem.CoverArt = coverArt
	}

	// Save session
	if err := s.savePlaybackSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save playback session: %w", err)
	}

	// Update play count and last played asynchronously.
	// Use a dedicated timeout context since the request context may be
	// cancelled before the DB write completes.
	go func() {
		bgCtx, bgCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer bgCancel()
		s.updatePlaybackStats(bgCtx, mediaItem.ID)
	}()

	return session, nil
}

// UpdatePlayback updates the playback state
func (s *MediaPlayerService) UpdatePlayback(ctx context.Context, userID string, request *PlaybackUpdateRequest) error {
	s.logger.Debug("Updating playback",
		zap.String("session_id", request.SessionID),
		zap.String("user_id", userID))

	session, err := s.getPlaybackSession(ctx, request.SessionID, userID)
	if err != nil {
		return fmt.Errorf("failed to get playback session: %w", err)
	}

	// Update fields if provided
	if request.Position != nil {
		session.CurrentPosition = *request.Position
		// Save position to database for resume functionality.
		// Use a dedicated timeout context since the request context may be
		// cancelled before the DB write completes.
		userIDInt, _ := strconv.ParseInt(userID, 10, 64)
		go func() {
			bgCtx, bgCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer bgCancel()
			s.positionTracker.UpdatePosition(bgCtx, &UpdatePositionRequest{
				UserID:      userIDInt,
				MediaItemID: session.MediaItem.ID,
				Position:    int64(*request.Position * 1000), // Convert seconds to milliseconds
				Duration:    0,                               // Duration unknown at this point
			})
		}()
	}

	if request.State != nil {
		session.State = *request.State
	}

	if request.Volume != nil {
		session.Volume = *request.Volume
	}

	if request.PlaybackRate != nil {
		session.PlaybackRate = *request.PlaybackRate
	}

	if request.RepeatMode != nil {
		session.RepeatMode = *request.RepeatMode
	}

	if request.ShuffleEnabled != nil {
		session.ShuffleEnabled = *request.ShuffleEnabled
	}

	// Update subtitle track
	if request.SubtitleTrackID != nil {
		session.CurrentSubtitle = s.findSubtitleByID(session.MediaItem.Subtitles, *request.SubtitleTrackID)
	}

	session.UpdatedAt = time.Now()

	// Save updated session
	return s.savePlaybackSession(ctx, session)
}

// GetMediaItem retrieves a media item by ID
func (s *MediaPlayerService) GetMediaItem(ctx context.Context, id int64) (*MediaItem, error) {
	query := `
		SELECT id, path, filename, title, media_type, mime_type, size, duration,
		       artist, album, album_artist, genre, year, track_number, disc_number,
		       video_codec, audio_codec, resolution, framerate, bitrate,
		       series_title, season, episode, episode_title,
		       description, language, last_position, play_count, last_played,
		       is_favorite, rating, created_at, updated_at
		FROM media_items WHERE id = ?`

	var item MediaItem
	var lastPlayed sql.NullTime

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&item.ID, &item.Path, &item.Filename, &item.Title, &item.MediaType, &item.MimeType,
		&item.Size, &item.Duration, &item.Artist, &item.Album, &item.AlbumArtist,
		&item.Genre, &item.Year, &item.TrackNumber, &item.DiscNumber,
		&item.VideoCodec, &item.AudioCodec, &item.Resolution, &item.Framerate, &item.Bitrate,
		&item.SeriesTitle, &item.Season, &item.Episode, &item.EpisodeTitle,
		&item.Description, &item.Language, &item.LastPosition, &item.PlayCount,
		&lastPlayed, &item.IsFavorite, &item.Rating, &item.CreatedAt, &item.UpdatedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("media item not found")
		}
		return nil, fmt.Errorf("failed to get media item: %w", err)
	}

	if lastPlayed.Valid {
		item.LastPlayed = &lastPlayed.Time
	}

	return &item, nil
}

// Helper functions
func generateSessionID() string {
	// CAT-LOGSHARE-TOK-1 class fix: hardening of a currently unwired service (not a live exposure): the id is looked up without an owner check, so it should be a capability: 128 random bits.
	return "session_" + rand.Text()
}

func getFloatValue(values ...*float64) float64 {
	for _, v := range values {
		if v != nil {
			return *v
		}
	}
	return 0.0
}

func (s *MediaPlayerService) findSubtitleByLanguage(subtitles []SubtitleTrack, lang string) *SubtitleTrack {
	for _, sub := range subtitles {
		if sub.Language == lang || sub.LanguageCode == lang {
			return &sub
		}
	}
	return nil
}

func (s *MediaPlayerService) findSubtitleByID(subtitles []SubtitleTrack, id string) *SubtitleTrack {
	for _, sub := range subtitles {
		if sub.ID == id {
			return &sub
		}
	}
	return nil
}

func (s *MediaPlayerService) savePlaybackSession(ctx context.Context, session *PlaybackSession) error {
	// Implementation would save to Redis or database
	// For now, we'll use in-memory storage or database
	s.logger.Debug("Saving playback session", zap.String("session_id", session.ID))
	return nil
}

func (s *MediaPlayerService) getPlaybackSession(ctx context.Context, sessionID, userID string) (*PlaybackSession, error) {
	// Implementation would retrieve from Redis or database
	s.logger.Debug("Getting playback session", zap.String("session_id", sessionID))
	return nil, fmt.Errorf("session not found")
}

func (s *MediaPlayerService) updatePlaybackStats(ctx context.Context, mediaItemID int64) {
	query := `UPDATE media_items SET play_count = play_count + 1, last_played = ? WHERE id = ?`
	_, err := s.db.ExecContext(ctx, query, time.Now(), mediaItemID)
	if err != nil {
		s.logger.Error("Failed to update playback stats", zap.Error(err))
	}
}

// GetSupportedMediaTypes returns all supported media types
func (s *MediaPlayerService) GetSupportedMediaTypes() []MediaType {
	return []MediaType{
		MediaTypeMusic,
		MediaTypeVideo,
		MediaTypeGame,
		MediaTypeSoftware,
		MediaTypeEbook,
		MediaTypeDocument,
	}
}

// GetMediaTypeFromExtension determines media type from file extension
func GetMediaTypeFromExtension(filename string) MediaType {
	ext := strings.ToLower(filepath.Ext(filename))

	// Music formats
	musicExts := map[string]bool{
		".mp3": true, ".flac": true, ".wav": true, ".aac": true, ".ogg": true,
		".m4a": true, ".wma": true, ".opus": true, ".aiff": true, ".ape": true,
	}

	// Video formats
	videoExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true, ".mov": true, ".wmv": true,
		".flv": true, ".webm": true, ".m4v": true, ".3gp": true, ".ts": true,
		".m2ts": true, ".vob": true, ".ogv": true,
	}

	// Game formats
	gameExts := map[string]bool{
		".exe": true, ".msi": true, ".deb": true, ".rpm": true, ".dmg": true,
		".app": true, ".apk": true, ".ipa": true,
	}

	// Document formats
	docExts := map[string]bool{
		".pdf": true, ".doc": true, ".docx": true, ".txt": true, ".rtf": true,
		".odt": true, ".pages": true,
	}

	// Ebook formats
	ebookExts := map[string]bool{
		".epub": true, ".mobi": true, ".azw": true, ".azw3": true, ".fb2": true,
		".lit": true, ".pdb": true,
	}

	if musicExts[ext] {
		return MediaTypeMusic
	} else if videoExts[ext] {
		return MediaTypeVideo
	} else if gameExts[ext] {
		return MediaTypeGame
	} else if docExts[ext] {
		return MediaTypeDocument
	} else if ebookExts[ext] {
		return MediaTypeEbook
	}

	return MediaTypeSoftware // Default fallback
}
