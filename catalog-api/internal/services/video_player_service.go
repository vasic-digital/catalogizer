package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"catalogizer/database"

	"go.uber.org/zap"
)

type VideoPlayerService struct {
	db                 *database.DB
	logger             *zap.Logger
	mediaPlayerService *MediaPlayerService
	positionService    *PlaybackPositionService
	subtitleService    *SubtitleService
	coverArtService    *CoverArtService
	translationService *TranslationService
}

type VideoPlaybackSession struct {
	ID               string          `json:"id"`
	UserID           int64           `json:"user_id"`
	CurrentVideo     *VideoContent   `json:"current_video"`
	Playlist         []VideoContent  `json:"playlist"`
	PlaylistIndex    int             `json:"playlist_index"`
	PlayMode         VideoPlayMode   `json:"play_mode"`
	AutoPlay         bool            `json:"auto_play"`
	AutoPlayNext     bool            `json:"auto_play_next"`
	Volume           float64         `json:"volume"`
	IsMuted          bool            `json:"is_muted"`
	PlaybackSpeed    float64         `json:"playback_speed"`
	PlaybackState    PlaybackState   `json:"playback_state"`
	Position         int64           `json:"position"`
	Duration         int64           `json:"duration"`
	BufferedRanges   []BufferedRange `json:"buffered_ranges"`
	SubtitleTracks   []SubtitleTrack `json:"subtitle_tracks"`
	ActiveSubtitle   *string         `json:"active_subtitle"`
	AudioTracks      []AudioTrack    `json:"audio_tracks"`
	ActiveAudioTrack *int64          `json:"active_audio_track"`
	VideoQuality     VideoQuality    `json:"video_quality"`
	DeviceInfo       DeviceInfo      `json:"device_info"`
	ViewingProgress  ViewingProgress `json:"viewing_progress"`
	Chapters         []Chapter       `json:"chapters"`
	Bookmarks        []VideoBookmark `json:"bookmarks"`
	WatchParty       *WatchPartyInfo `json:"watch_party"`
	LastActivity     time.Time       `json:"last_activity"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

type VideoContent struct {
	ID                int64            `json:"id"`
	Title             string           `json:"title"`
	OriginalTitle     string           `json:"original_title"`
	Description       string           `json:"description"`
	Type              VideoType        `json:"type"`
	FilePath          string           `json:"file_path"`
	FileSize          int64            `json:"file_size"`
	Duration          int64            `json:"duration"`
	Resolution        string           `json:"resolution"`
	AspectRatio       string           `json:"aspect_ratio"`
	FrameRate         float64          `json:"frame_rate"`
	Bitrate           int64            `json:"bitrate"`
	Codec             string           `json:"codec"`
	HDR               bool             `json:"hdr"`
	DolbyVision       bool             `json:"dolby_vision"`
	DolbyAtmos        bool             `json:"dolby_atmos"`
	Year              int              `json:"year"`
	ReleaseDate       *time.Time       `json:"release_date"`
	Genres            []string         `json:"genres"`
	Directors         []string         `json:"directors"`
	Actors            []string         `json:"actors"`
	Writers           []string         `json:"writers"`
	Rating            *float64         `json:"rating"`
	IMDbID            string           `json:"imdb_id"`
	TMDbID            string           `json:"tmdb_id"`
	Language          string           `json:"language"`
	Country           string           `json:"country"`
	PlayCount         int64            `json:"play_count"`
	LastPlayed        *time.Time       `json:"last_played"`
	DateAdded         time.Time        `json:"date_added"`
	UserRating        *int             `json:"user_rating"`
	IsFavorite        bool             `json:"is_favorite"`
	WatchedPercentage float64          `json:"watched_percentage"`
	CoverArt          *CoverArt        `json:"cover_art"`
	Backdrop          *CoverArt        `json:"backdrop"`
	Trailer           *TrailerInfo     `json:"trailer"`
	SeriesInfo        *SeriesInfo      `json:"series_info"`
	EpisodeInfo       *EpisodeInfo     `json:"episode_info"`
	MovieInfo         *MovieInfo       `json:"movie_info"`
	Thumbnails        []VideoThumbnail `json:"thumbnails"`
	VideoStreams      []VideoStream    `json:"video_streams"`
	AudioStreams      []AudioStream    `json:"audio_streams"`
	SubtitleStreams   []SubtitleStream `json:"subtitle_streams"`
}

type VideoType string

const (
	VideoTypeMovie   VideoType = "movie"
	VideoTypeEpisode VideoType = "episode"
	VideoTypeClip    VideoType = "clip"
	VideoTypeTrailer VideoType = "trailer"
	VideoTypeOther   VideoType = "other"
)

type VideoPlayMode string

const (
	VideoPlayModeSingle   VideoPlayMode = "single"
	VideoPlayModeEpisode  VideoPlayMode = "episode"
	VideoPlayModeSeason   VideoPlayMode = "season"
	VideoPlayModeSeries   VideoPlayMode = "series"
	VideoPlayModePlaylist VideoPlayMode = "playlist"
)

type VideoQuality string

const (
	Quality480p  VideoQuality = "480p"
	Quality720p  VideoQuality = "720p"
	Quality1080p VideoQuality = "1080p"
	Quality1440p VideoQuality = "1440p"
	Quality2160p VideoQuality = "2160p" // 4K
	Quality4320p VideoQuality = "4320p" // 8K
	QualityAuto  VideoQuality = "auto"
)

type ViewingProgress struct {
	StartedAt        time.Time `json:"started_at"`
	TotalWatchTime   int64     `json:"total_watch_time"`
	SessionWatchTime int64     `json:"session_watch_time"`
	PauseCount       int       `json:"pause_count"`
	SeekCount        int       `json:"seek_count"`
	RewindCount      int       `json:"rewind_count"`
	FastForwardCount int       `json:"fast_forward_count"`
	QualityChanges   int       `json:"quality_changes"`
	BufferingEvents  int       `json:"buffering_events"`
	TotalBufferTime  int64     `json:"total_buffer_time"`
}

type VideoChapter struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	StartTime int64     `json:"start_time"`
	EndTime   int64     `json:"end_time"`
	Thumbnail *CoverArt `json:"thumbnail"`
}

type VideoBookmark struct {
	ID          int64     `json:"id"`
	UserID      int64     `json:"user_id"`
	VideoID     int64     `json:"video_id"`
	Position    int64     `json:"position"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Thumbnail   *CoverArt `json:"thumbnail"`
	CreatedAt   time.Time `json:"created_at"`
}

type WatchPartyInfo struct {
	ID           string    `json:"id"`
	HostUserID   int64     `json:"host_user_id"`
	Participants []int64   `json:"participants"`
	SyncEnabled  bool      `json:"sync_enabled"`
	ChatEnabled  bool      `json:"chat_enabled"`
	CreatedAt    time.Time `json:"created_at"`
}

type TrailerInfo struct {
	URL      string `json:"url"`
	Provider string `json:"provider"`
	Quality  string `json:"quality"`
	Duration int64  `json:"duration"`
}

type SeriesInfo struct {
	ID            int64      `json:"id"`
	Title         string     `json:"title"`
	Description   string     `json:"description"`
	TotalSeasons  int        `json:"total_seasons"`
	TotalEpisodes int        `json:"total_episodes"`
	Status        string     `json:"status"`
	FirstAired    *time.Time `json:"first_aired"`
	LastAired     *time.Time `json:"last_aired"`
	Network       string     `json:"network"`
	Creator       []string   `json:"creator"`
}

type EpisodeInfo struct {
	SeriesID      int64      `json:"series_id"`
	SeasonNumber  int        `json:"season_number"`
	EpisodeNumber int        `json:"episode_number"`
	AirDate       *time.Time `json:"air_date"`
	Runtime       int        `json:"runtime"`
	GuestStars    []string   `json:"guest_stars"`
	NextEpisodeID *int64     `json:"next_episode_id"`
	PrevEpisodeID *int64     `json:"prev_episode_id"`
}

type MovieInfo struct {
	Budget              int64    `json:"budget"`
	Revenue             int64    `json:"revenue"`
	Runtime             int      `json:"runtime"`
	Collection          string   `json:"collection"`
	Studio              []string `json:"studio"`
	ProductionCompanies []string `json:"production_companies"`
}

type VideoThumbnail struct {
	ID        int64     `json:"id"`
	VideoID   int64     `json:"video_id"`
	Position  int64     `json:"position"`
	URL       string    `json:"url"`
	Width     int       `json:"width"`
	Height    int       `json:"height"`
	CreatedAt time.Time `json:"created_at"`
}

type VideoStream struct {
	ID       int64   `json:"id"`
	Index    int     `json:"index"`
	Codec    string  `json:"codec"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Bitrate  int64   `json:"bitrate"`
	FPS      float64 `json:"fps"`
	Language string  `json:"language"`
	Title    string  `json:"title"`
	Default  bool    `json:"default"`
}

type AudioStream struct {
	ID       int64  `json:"id"`
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Channels int    `json:"channels"`
	Bitrate  int64  `json:"bitrate"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Default  bool   `json:"default"`
}

type SubtitleStream struct {
	ID       int64  `json:"id"`
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Language string `json:"language"`
	Title    string `json:"title"`
	Default  bool   `json:"default"`
	Forced   bool   `json:"forced"`
	External bool   `json:"external"`
	FilePath string `json:"file_path"`
}

type PlayVideoRequest struct {
	UserID       int64         `json:"user_id"`
	VideoID      int64         `json:"video_id"`
	PlayMode     VideoPlayMode `json:"play_mode"`
	StartTime    *int64        `json:"start_time"`
	Quality      VideoQuality  `json:"quality"`
	DeviceInfo   DeviceInfo    `json:"device_info"`
	AutoPlay     bool          `json:"auto_play"`
	SeriesID     *int64        `json:"series_id"`
	SeasonNumber *int          `json:"season_number"`
	PlaylistID   *int64        `json:"playlist_id"`
}

type PlaySeriesRequest struct {
	UserID       int64        `json:"user_id"`
	SeriesID     int64        `json:"series_id"`
	SeasonNumber *int         `json:"season_number"`
	StartEpisode *int         `json:"start_episode"`
	Quality      VideoQuality `json:"quality"`
	DeviceInfo   DeviceInfo   `json:"device_info"`
	AutoPlay     bool         `json:"auto_play"`
}

type UpdateVideoPlaybackRequest struct {
	SessionID      string         `json:"session_id"`
	Position       *int64         `json:"position"`
	State          *PlaybackState `json:"state"`
	Volume         *float64       `json:"volume"`
	IsMuted        *bool          `json:"is_muted"`
	PlaybackSpeed  *float64       `json:"playback_speed"`
	Quality        *VideoQuality  `json:"quality"`
	ActiveSubtitle *string        `json:"active_subtitle"`
	ActiveAudio    *int64         `json:"active_audio"`
}

type VideoSeekRequest struct {
	SessionID string `json:"session_id"`
	Position  int64  `json:"position"`
}

type CreateVideoBookmarkRequest struct {
	SessionID   string `json:"session_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type WatchHistoryRequest struct {
	UserID    int64      `json:"user_id"`
	StartDate *time.Time `json:"start_date"`
	EndDate   *time.Time `json:"end_date"`
	VideoType *VideoType `json:"video_type"`
	Limit     int        `json:"limit"`
	Offset    int        `json:"offset"`
}

type WatchHistory struct {
	ID             int64        `json:"id"`
	UserID         int64        `json:"user_id"`
	VideoID        int64        `json:"video_id"`
	VideoContent   VideoContent `json:"video_content"`
	WatchedAt      time.Time    `json:"watched_at"`
	WatchDuration  int64        `json:"watch_duration"`
	CompletionRate float64      `json:"completion_rate"`
	StoppedAt      int64        `json:"stopped_at"`
	DeviceInfo     string       `json:"device_info"`
	Quality        string       `json:"quality"`
}

func NewVideoPlayerService(
	db *database.DB,
	logger *zap.Logger,
	mediaPlayerService *MediaPlayerService,
	positionService *PlaybackPositionService,
	subtitleService *SubtitleService,
	coverArtService *CoverArtService,
	translationService *TranslationService,
) *VideoPlayerService {
	return &VideoPlayerService{
		db:                 db,
		logger:             logger,
		mediaPlayerService: mediaPlayerService,
		positionService:    positionService,
		subtitleService:    subtitleService,
		coverArtService:    coverArtService,
		translationService: translationService,
	}
}

func (s *VideoPlayerService) PlayVideo(ctx context.Context, req *PlayVideoRequest) (*VideoPlaybackSession, error) {
	s.logger.Info("Starting video playback",
		zap.Int64("user_id", req.UserID),
		zap.Int64("video_id", req.VideoID),
		zap.String("play_mode", string(req.PlayMode)))

	video, err := s.getVideoContent(ctx, req.VideoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get video content: %w", err)
	}

	session := &VideoPlaybackSession{
		ID:            generateSessionID(),
		UserID:        req.UserID,
		CurrentVideo:  video,
		Playlist:      []VideoContent{*video},
		PlaylistIndex: 0,
		PlayMode:      req.PlayMode,
		AutoPlay:      req.AutoPlay,
		AutoPlayNext:  true,
		Volume:        1.0,
		IsMuted:       false,
		PlaybackSpeed: 1.0,
		PlaybackState: PlaybackStatePlaying,
		Position:      0,
		Duration:      video.Duration,
		VideoQuality:  req.Quality,
		DeviceInfo:    req.DeviceInfo,
		ViewingProgress: ViewingProgress{
			StartedAt: time.Now(),
		},
		LastActivity: time.Now(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if req.StartTime != nil {
		session.Position = *req.StartTime
	} else {
		position, err := s.positionService.GetPosition(ctx, req.UserID, req.VideoID)
		if err == nil && position != nil && position.PercentComplete < 90 {
			session.Position = position.Position
		}
	}

	if err := s.loadVideoStreams(ctx, session, video.ID); err != nil {
		s.logger.Warn("Failed to load video streams", zap.Error(err))
	}

	if err := s.loadSubtitles(ctx, session, video.ID); err != nil {
		s.logger.Warn("Failed to load subtitles", zap.Error(err))
	}

	if err := s.loadChapters(ctx, session, video.ID); err != nil {
		s.logger.Warn("Failed to load chapters", zap.Error(err))
	}

	switch req.PlayMode {
	case VideoPlayModeEpisode:
		if video.EpisodeInfo != nil {
			if err := s.loadEpisodePlaylist(ctx, session, video.EpisodeInfo.SeriesID, video.EpisodeInfo.SeasonNumber); err != nil {
				s.logger.Warn("Failed to load episode playlist", zap.Error(err))
			}
		}
	case VideoPlayModeSeason:
		if req.SeasonNumber != nil && video.SeriesInfo != nil {
			if err := s.loadSeasonPlaylist(ctx, session, video.SeriesInfo.ID, *req.SeasonNumber); err != nil {
				s.logger.Warn("Failed to load season playlist", zap.Error(err))
			}
		}
	case VideoPlayModeSeries:
		if req.SeriesID != nil {
			if err := s.loadSeriesPlaylist(ctx, session, *req.SeriesID); err != nil {
				s.logger.Warn("Failed to load series playlist", zap.Error(err))
			}
		}
	case VideoPlayModePlaylist:
		if req.PlaylistID != nil {
			if err := s.loadVideoPlaylist(ctx, session, *req.PlaylistID); err != nil {
				s.logger.Warn("Failed to load video playlist", zap.Error(err))
			}
		}
	}

	if err := s.saveVideoSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	if err := s.recordVideoPlayback(ctx, req.UserID, video.ID); err != nil {
		s.logger.Warn("Failed to record playback", zap.Error(err))
	}

	return session, nil
}

func (s *VideoPlayerService) PlaySeries(ctx context.Context, req *PlaySeriesRequest) (*VideoPlaybackSession, error) {
	s.logger.Info("Starting series playback",
		zap.Int64("user_id", req.UserID),
		zap.Int64("series_id", req.SeriesID))

	_, err := s.getSeriesInfo(ctx, req.SeriesID)
	if err != nil {
		return nil, fmt.Errorf("failed to get series info: %w", err)
	}

	seasonNumber := 1
	if req.SeasonNumber != nil {
		seasonNumber = *req.SeasonNumber
	}

	episodes, err := s.getSeasonEpisodes(ctx, req.SeriesID, seasonNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get season episodes: %w", err)
	}

	if len(episodes) == 0 {
		return nil, fmt.Errorf("no episodes found for season %d", seasonNumber)
	}

	startIndex := 0
	if req.StartEpisode != nil && *req.StartEpisode < len(episodes) {
		startIndex = *req.StartEpisode
	}

	session := &VideoPlaybackSession{
		ID:            generateSessionID(),
		UserID:        req.UserID,
		CurrentVideo:  &episodes[startIndex],
		Playlist:      episodes,
		PlaylistIndex: startIndex,
		PlayMode:      VideoPlayModeSeries,
		AutoPlay:      req.AutoPlay,
		AutoPlayNext:  true,
		Volume:        1.0,
		IsMuted:       false,
		PlaybackSpeed: 1.0,
		PlaybackState: PlaybackStatePlaying,
		Position:      0,
		Duration:      episodes[startIndex].Duration,
		VideoQuality:  req.Quality,
		DeviceInfo:    req.DeviceInfo,
		ViewingProgress: ViewingProgress{
			StartedAt: time.Now(),
		},
		LastActivity: time.Now(),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	position, err := s.positionService.GetPosition(ctx, req.UserID, episodes[startIndex].ID)
	if err == nil && position != nil && position.PercentComplete < 90 {
		session.Position = position.Position
	}

	if err := s.loadVideoStreams(ctx, session, episodes[startIndex].ID); err != nil {
		s.logger.Warn("Failed to load video streams", zap.Error(err))
	}

	if err := s.loadSubtitles(ctx, session, episodes[startIndex].ID); err != nil {
		s.logger.Warn("Failed to load subtitles", zap.Error(err))
	}

	if err := s.saveVideoSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return session, nil
}

func (s *VideoPlayerService) GetVideoSession(ctx context.Context, sessionID string) (*VideoPlaybackSession, error) {
	s.logger.Debug("Getting video session", zap.String("session_id", sessionID))

	query := `
		SELECT session_data, updated_at
		FROM video_playback_sessions
		WHERE id = ? AND expires_at > CURRENT_TIMESTAMP
	`

	var sessionData string
	var updatedAt time.Time
	err := s.db.QueryRowContext(ctx, query, sessionID).Scan(&sessionData, &updatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("session not found or expired")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session VideoPlaybackSession
	if err := json.Unmarshal([]byte(sessionData), &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	session.UpdatedAt = updatedAt
	return &session, nil
}

func (s *VideoPlayerService) UpdateVideoPlayback(ctx context.Context, req *UpdateVideoPlaybackRequest) (*VideoPlaybackSession, error) {
	s.logger.Debug("Updating video playback", zap.String("session_id", req.SessionID))

	session, err := s.GetVideoSession(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}

	if req.Position != nil {
		oldPosition := session.Position
		session.Position = *req.Position
		session.ViewingProgress.SessionWatchTime += *req.Position - oldPosition

		if err := s.positionService.UpdatePosition(ctx, &UpdatePositionRequest{
			UserID:          session.UserID,
			MediaItemID:     session.CurrentVideo.ID,
			Position:        *req.Position,
			Duration:        session.Duration,
			DeviceInfo:      session.DeviceInfo.DeviceName,
			PlaybackQuality: string(session.VideoQuality),
		}); err != nil {
			s.logger.Warn("Failed to update position", zap.Error(err))
		}
	}

	if req.State != nil {
		session.PlaybackState = *req.State
		if *req.State == PlaybackStatePaused {
			session.ViewingProgress.PauseCount++
		}
	}

	if req.Volume != nil {
		session.Volume = *req.Volume
	}

	if req.IsMuted != nil {
		session.IsMuted = *req.IsMuted
	}

	if req.PlaybackSpeed != nil {
		session.PlaybackSpeed = *req.PlaybackSpeed
	}

	if req.Quality != nil {
		if session.VideoQuality != *req.Quality {
			session.ViewingProgress.QualityChanges++
		}
		session.VideoQuality = *req.Quality
	}

	if req.ActiveSubtitle != nil {
		session.ActiveSubtitle = req.ActiveSubtitle
	}

	if req.ActiveAudio != nil {
		session.ActiveAudioTrack = req.ActiveAudio
	}

	session.LastActivity = time.Now()
	session.UpdatedAt = time.Now()

	if err := s.saveVideoSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return session, nil
}

func (s *VideoPlayerService) NextVideo(ctx context.Context, sessionID string) (*VideoPlaybackSession, error) {
	s.logger.Debug("Skipping to next video", zap.String("session_id", sessionID))

	session, err := s.GetVideoSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if session.PlaylistIndex >= len(session.Playlist)-1 {
		session.PlaybackState = PlaybackStateStopped
		return session, nil
	}

	session.PlaylistIndex++
	session.CurrentVideo = &session.Playlist[session.PlaylistIndex]
	session.Position = 0
	session.Duration = session.CurrentVideo.Duration

	position, err := s.positionService.GetPosition(ctx, session.UserID, session.CurrentVideo.ID)
	if err == nil && position != nil && position.PercentComplete < 90 {
		session.Position = position.Position
	}

	if err := s.loadVideoStreams(ctx, session, session.CurrentVideo.ID); err != nil {
		s.logger.Warn("Failed to load video streams", zap.Error(err))
	}

	if err := s.loadSubtitles(ctx, session, session.CurrentVideo.ID); err != nil {
		s.logger.Warn("Failed to load subtitles", zap.Error(err))
	}

	session.LastActivity = time.Now()
	session.UpdatedAt = time.Now()

	if err := s.saveVideoSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	if err := s.recordVideoPlayback(ctx, session.UserID, session.CurrentVideo.ID); err != nil {
		s.logger.Warn("Failed to record playback", zap.Error(err))
	}

	return session, nil
}

func (s *VideoPlayerService) PreviousVideo(ctx context.Context, sessionID string) (*VideoPlaybackSession, error) {
	s.logger.Debug("Skipping to previous video", zap.String("session_id", sessionID))

	session, err := s.GetVideoSession(ctx, sessionID)
	if err != nil {
		return nil, err
	}

	if session.Position > 10000 {
		session.Position = 0
	} else if session.PlaylistIndex > 0 {
		session.PlaylistIndex--
		session.CurrentVideo = &session.Playlist[session.PlaylistIndex]
		session.Duration = session.CurrentVideo.Duration

		position, err := s.positionService.GetPosition(ctx, session.UserID, session.CurrentVideo.ID)
		if err == nil && position != nil && position.PercentComplete < 90 {
			session.Position = position.Position
		} else {
			session.Position = 0
		}

		if err := s.loadVideoStreams(ctx, session, session.CurrentVideo.ID); err != nil {
			s.logger.Warn("Failed to load video streams", zap.Error(err))
		}

		if err := s.loadSubtitles(ctx, session, session.CurrentVideo.ID); err != nil {
			s.logger.Warn("Failed to load subtitles", zap.Error(err))
		}
	} else {
		session.Position = 0
	}

	session.LastActivity = time.Now()
	session.UpdatedAt = time.Now()

	if err := s.saveVideoSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return session, nil
}

func (s *VideoPlayerService) SeekVideo(ctx context.Context, req *VideoSeekRequest) (*VideoPlaybackSession, error) {
	s.logger.Debug("Seeking in video",
		zap.String("session_id", req.SessionID),
		zap.Int64("position", req.Position))

	session, err := s.GetVideoSession(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}

	if req.Position < 0 {
		req.Position = 0
	}
	if req.Position > session.Duration {
		req.Position = session.Duration
	}

	if req.Position < session.Position {
		session.ViewingProgress.RewindCount++
	} else if req.Position > session.Position+5000 {
		session.ViewingProgress.FastForwardCount++
	}

	session.ViewingProgress.SeekCount++
	session.Position = req.Position
	session.LastActivity = time.Now()
	session.UpdatedAt = time.Now()

	if err := s.saveVideoSession(ctx, session); err != nil {
		return nil, fmt.Errorf("failed to save session: %w", err)
	}

	return session, nil
}

func (s *VideoPlayerService) CreateVideoBookmark(ctx context.Context, req *CreateVideoBookmarkRequest) (*VideoBookmark, error) {
	s.logger.Info("Creating video bookmark", zap.String("session_id", req.SessionID))

	session, err := s.GetVideoSession(ctx, req.SessionID)
	if err != nil {
		return nil, err
	}

	thumbnail, err := s.generateThumbnail(ctx, session.CurrentVideo.ID, session.Position)
	if err != nil {
		s.logger.Warn("Failed to generate thumbnail", zap.Error(err))
	}

	query := `
		INSERT INTO video_bookmarks (user_id, video_id, position, title, description, thumbnail_url, created_at)
		VALUES (?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	var bookmark VideoBookmark
	var thumbnailURL string
	if thumbnail != nil && thumbnail.URL != nil {
		thumbnailURL = *thumbnail.URL
	}

	bookmarkID, err := s.db.InsertReturningID(ctx, query,
		session.UserID, session.CurrentVideo.ID, session.Position,
		req.Title, req.Description, thumbnailURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create bookmark: %w", err)
	}

	bookmark.ID = bookmarkID
	bookmark.CreatedAt = time.Now()

	bookmark.UserID = session.UserID
	bookmark.VideoID = session.CurrentVideo.ID
	bookmark.Position = session.Position
	bookmark.Title = req.Title
	bookmark.Description = req.Description
	bookmark.Thumbnail = thumbnail

	return &bookmark, nil
}

func (s *VideoPlayerService) GetWatchHistory(ctx context.Context, req *WatchHistoryRequest) ([]WatchHistory, error) {
	s.logger.Debug("Getting watch history", zap.Int64("user_id", req.UserID))

	baseQuery := `
		SELECT vh.id, vh.user_id, vh.video_id, vh.watched_at, vh.watch_duration,
			   vh.completion_rate, vh.stopped_at, vh.device_info, vh.quality
		FROM video_watch_history vh
		WHERE vh.user_id = ?
	`

	args := []interface{}{req.UserID}

	if req.StartDate != nil {
		baseQuery += " AND vh.watched_at >= ?"
		args = append(args, *req.StartDate)
	}

	if req.EndDate != nil {
		baseQuery += " AND vh.watched_at <= ?"
		args = append(args, *req.EndDate)
	}

	if req.VideoType != nil {
		baseQuery += " AND mi.type = ?"
		args = append(args, string(*req.VideoType))
		baseQuery = strings.Replace(baseQuery, "FROM video_watch_history vh",
			"FROM video_watch_history vh INNER JOIN media_items mi ON vh.video_id = mi.id", 1)
	}

	baseQuery += " ORDER BY vh.watched_at DESC"

	if req.Limit > 0 {
		baseQuery += " LIMIT ?"
		args = append(args, req.Limit)
	}

	if req.Offset > 0 {
		baseQuery += " OFFSET ?"
		args = append(args, req.Offset)
	}

	rows, err := s.db.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to get watch history: %w", err)
	}
	defer rows.Close()

	history := []WatchHistory{}
	var videoIDs []int64

	for rows.Next() {
		var item WatchHistory
		err := rows.Scan(
			&item.ID, &item.UserID, &item.VideoID, &item.WatchedAt,
			&item.WatchDuration, &item.CompletionRate, &item.StoppedAt,
			&item.DeviceInfo, &item.Quality,
		)
		if err != nil {
			continue
		}
		history = append(history, item)
		videoIDs = append(videoIDs, item.VideoID)
	}

	videoMap, err := s.getVideoContentsMap(ctx, videoIDs)
	if err != nil {
		s.logger.Warn("Failed to load video contents", zap.Error(err))
	} else {
		for i := range history {
			if video, exists := videoMap[history[i].VideoID]; exists {
				history[i].VideoContent = video
			}
		}
	}

	return history, nil
}

func (s *VideoPlayerService) GetContinueWatching(ctx context.Context, userID int64, limit int) ([]VideoContent, error) {
	s.logger.Debug("Getting continue watching", zap.Int64("user_id", userID))

	cutoff := time.Now().Add(-30 * 24 * time.Hour)
	query := `
		SELECT DISTINCT pp.media_item_id
		FROM playback_positions pp
		INNER JOIN media_items mi ON pp.media_item_id = mi.id
		WHERE pp.user_id = ?
		  AND mi.type = 'video'
		  AND pp.percent_complete BETWEEN 5 AND 90
		  AND pp.last_played > ?
		ORDER BY pp.last_played DESC
		LIMIT ?
	`

	rows, err := s.db.QueryContext(ctx, query, userID, cutoff, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to get continue watching: %w", err)
	}
	defer rows.Close()

	var videoIDs []int64
	for rows.Next() {
		var videoID int64
		if err := rows.Scan(&videoID); err == nil {
			videoIDs = append(videoIDs, videoID)
		}
	}

	videos, err := s.getVideoContents(ctx, videoIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to get video contents: %w", err)
	}

	return videos, nil
}

func (s *VideoPlayerService) getVideoContent(ctx context.Context, videoID int64) (*VideoContent, error) {
	query := `
		SELECT id, title, original_title, description, type, file_path, file_size,
			   duration, resolution, aspect_ratio, frame_rate, bitrate, codec,
			   hdr, dolby_vision, dolby_atmos, year, release_date, genres,
			   directors, actors, writers, rating, imdb_id, tmdb_id, language,
			   country, play_count, last_played, date_added, user_rating,
			   is_favorite, watched_percentage
		FROM media_items
		WHERE id = ? AND type = 'video'
	`

	var video VideoContent
	var releaseDate sql.NullTime
	var lastPlayed sql.NullTime
	var genresJSON, directorsJSON, actorsJSON, writersJSON sql.NullString
	var rating sql.NullFloat64
	var userRating sql.NullInt64

	err := s.db.QueryRowContext(ctx, query, videoID).Scan(
		&video.ID, &video.Title, &video.OriginalTitle, &video.Description,
		&video.Type, &video.FilePath, &video.FileSize, &video.Duration,
		&video.Resolution, &video.AspectRatio, &video.FrameRate, &video.Bitrate,
		&video.Codec, &video.HDR, &video.DolbyVision, &video.DolbyAtmos,
		&video.Year, &releaseDate, &genresJSON, &directorsJSON, &actorsJSON,
		&writersJSON, &rating, &video.IMDbID, &video.TMDbID, &video.Language,
		&video.Country, &video.PlayCount, &lastPlayed, &video.DateAdded,
		&userRating, &video.IsFavorite, &video.WatchedPercentage,
	)

	if err != nil {
		return nil, err
	}

	if releaseDate.Valid {
		video.ReleaseDate = &releaseDate.Time
	}
	if lastPlayed.Valid {
		video.LastPlayed = &lastPlayed.Time
	}
	if rating.Valid {
		video.Rating = &rating.Float64
	}
	if userRating.Valid {
		ratingInt := int(userRating.Int64)
		video.UserRating = &ratingInt
	}

	if genresJSON.Valid {
		json.Unmarshal([]byte(genresJSON.String), &video.Genres)
	}
	if directorsJSON.Valid {
		json.Unmarshal([]byte(directorsJSON.String), &video.Directors)
	}
	if actorsJSON.Valid {
		json.Unmarshal([]byte(actorsJSON.String), &video.Actors)
	}
	if writersJSON.Valid {
		json.Unmarshal([]byte(writersJSON.String), &video.Writers)
	}

	if err := s.loadVideoMetadata(ctx, &video); err != nil {
		s.logger.Warn("Failed to load video metadata", zap.Error(err))
	}

	return &video, nil
}

func (s *VideoPlayerService) getVideoContents(ctx context.Context, videoIDs []int64) ([]VideoContent, error) {
	if len(videoIDs) == 0 {
		return []VideoContent{}, nil
	}

	videoMap, err := s.getVideoContentsMap(ctx, videoIDs)
	if err != nil {
		return nil, err
	}

	var videos []VideoContent
	for _, id := range videoIDs {
		if video, exists := videoMap[id]; exists {
			videos = append(videos, video)
		}
	}

	return videos, nil
}

func (s *VideoPlayerService) getVideoContentsMap(ctx context.Context, videoIDs []int64) (map[int64]VideoContent, error) {
	if len(videoIDs) == 0 {
		return make(map[int64]VideoContent), nil
	}

	placeholders := make([]string, len(videoIDs))
	args := make([]interface{}, len(videoIDs))
	for i, id := range videoIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT id, title, original_title, description, type, file_path, file_size,
			   duration, resolution, aspect_ratio, frame_rate, bitrate, codec,
			   hdr, dolby_vision, dolby_atmos, year, release_date, genres,
			   directors, actors, writers, rating, imdb_id, tmdb_id, language,
			   country, play_count, last_played, date_added, user_rating,
			   is_favorite, watched_percentage
		FROM media_items
		WHERE id IN (%s) AND type = 'video'
	`, strings.Join(placeholders, ","))

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	videoMap := make(map[int64]VideoContent)
	for rows.Next() {
		var video VideoContent
		var releaseDate sql.NullTime
		var lastPlayed sql.NullTime
		var genresJSON, directorsJSON, actorsJSON, writersJSON sql.NullString
		var rating sql.NullFloat64
		var userRating sql.NullInt64

		err := rows.Scan(
			&video.ID, &video.Title, &video.OriginalTitle, &video.Description,
			&video.Type, &video.FilePath, &video.FileSize, &video.Duration,
			&video.Resolution, &video.AspectRatio, &video.FrameRate, &video.Bitrate,
			&video.Codec, &video.HDR, &video.DolbyVision, &video.DolbyAtmos,
			&video.Year, &releaseDate, &genresJSON, &directorsJSON, &actorsJSON,
			&writersJSON, &rating, &video.IMDbID, &video.TMDbID, &video.Language,
			&video.Country, &video.PlayCount, &lastPlayed, &video.DateAdded,
			&userRating, &video.IsFavorite, &video.WatchedPercentage,
		)

		if err != nil {
			continue
		}

		if releaseDate.Valid {
			video.ReleaseDate = &releaseDate.Time
		}
		if lastPlayed.Valid {
			video.LastPlayed = &lastPlayed.Time
		}
		if rating.Valid {
			video.Rating = &rating.Float64
		}
		if userRating.Valid {
			ratingInt := int(userRating.Int64)
			video.UserRating = &ratingInt
		}

		if genresJSON.Valid {
			json.Unmarshal([]byte(genresJSON.String), &video.Genres)
		}
		if directorsJSON.Valid {
			json.Unmarshal([]byte(directorsJSON.String), &video.Directors)
		}
		if actorsJSON.Valid {
			json.Unmarshal([]byte(actorsJSON.String), &video.Actors)
		}
		if writersJSON.Valid {
			json.Unmarshal([]byte(writersJSON.String), &video.Writers)
		}

		videoMap[video.ID] = video
	}

	return videoMap, nil
}

func (s *VideoPlayerService) getSeriesInfo(ctx context.Context, seriesID int64) (*SeriesInfo, error) {
	query := `
		SELECT id, title, description, total_seasons, total_episodes, status,
			   first_aired, last_aired, network, creator
		FROM series
		WHERE id = ?
	`

	var series SeriesInfo
	var firstAired, lastAired sql.NullTime
	var creatorJSON sql.NullString

	err := s.db.QueryRowContext(ctx, query, seriesID).Scan(
		&series.ID, &series.Title, &series.Description, &series.TotalSeasons,
		&series.TotalEpisodes, &series.Status, &firstAired, &lastAired,
		&series.Network, &creatorJSON,
	)

	if err != nil {
		return nil, err
	}

	if firstAired.Valid {
		series.FirstAired = &firstAired.Time
	}
	if lastAired.Valid {
		series.LastAired = &lastAired.Time
	}
	if creatorJSON.Valid {
		json.Unmarshal([]byte(creatorJSON.String), &series.Creator)
	}

	return &series, nil
}

func (s *VideoPlayerService) getSeasonEpisodes(ctx context.Context, seriesID int64, seasonNumber int) ([]VideoContent, error) {
	query := `
		SELECT mi.id
		FROM media_items mi
		INNER JOIN episodes e ON mi.id = e.media_item_id
		WHERE e.series_id = ? AND e.season_number = ? AND mi.type = 'video'
		ORDER BY e.episode_number ASC
	`

	rows, err := s.db.QueryContext(ctx, query, seriesID, seasonNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var episodeIDs []int64
	for rows.Next() {
		var episodeID int64
		if err := rows.Scan(&episodeID); err == nil {
			episodeIDs = append(episodeIDs, episodeID)
		}
	}

	return s.getVideoContents(ctx, episodeIDs)
}

func (s *VideoPlayerService) loadVideoMetadata(ctx context.Context, video *VideoContent) error {
	if err := s.loadSeriesInfo(ctx, video); err != nil {
		s.logger.Debug("Failed to load series info", zap.Error(err))
	}

	if err := s.loadEpisodeInfo(ctx, video); err != nil {
		s.logger.Debug("Failed to load episode info", zap.Error(err))
	}

	if err := s.loadMovieInfo(ctx, video); err != nil {
		s.logger.Debug("Failed to load movie info", zap.Error(err))
	}

	return nil
}

func (s *VideoPlayerService) loadSeriesInfo(ctx context.Context, video *VideoContent) error {
	if video.Type != VideoTypeEpisode {
		return nil
	}

	query := `
		SELECT s.id, s.title, s.description, s.total_seasons, s.total_episodes,
			   s.status, s.first_aired, s.last_aired, s.network, s.creator
		FROM series s
		INNER JOIN episodes e ON s.id = e.series_id
		WHERE e.media_item_id = ?
	`

	var series SeriesInfo
	var firstAired, lastAired sql.NullTime
	var creatorJSON sql.NullString

	err := s.db.QueryRowContext(ctx, query, video.ID).Scan(
		&series.ID, &series.Title, &series.Description, &series.TotalSeasons,
		&series.TotalEpisodes, &series.Status, &firstAired, &lastAired,
		&series.Network, &creatorJSON,
	)

	if err != nil {
		return err
	}

	if firstAired.Valid {
		series.FirstAired = &firstAired.Time
	}
	if lastAired.Valid {
		series.LastAired = &lastAired.Time
	}
	if creatorJSON.Valid {
		json.Unmarshal([]byte(creatorJSON.String), &series.Creator)
	}

	video.SeriesInfo = &series
	return nil
}

func (s *VideoPlayerService) loadEpisodeInfo(ctx context.Context, video *VideoContent) error {
	if video.Type != VideoTypeEpisode {
		return nil
	}

	query := `
		SELECT series_id, season_number, episode_number, air_date, runtime, guest_stars,
			   next_episode_id, prev_episode_id
		FROM episodes
		WHERE media_item_id = ?
	`

	var episode EpisodeInfo
	var airDate sql.NullTime
	var guestStarsJSON sql.NullString
	var nextEpisodeID, prevEpisodeID sql.NullInt64

	err := s.db.QueryRowContext(ctx, query, video.ID).Scan(
		&episode.SeriesID, &episode.SeasonNumber, &episode.EpisodeNumber,
		&airDate, &episode.Runtime, &guestStarsJSON, &nextEpisodeID, &prevEpisodeID,
	)

	if err != nil {
		return err
	}

	if airDate.Valid {
		episode.AirDate = &airDate.Time
	}
	if guestStarsJSON.Valid {
		json.Unmarshal([]byte(guestStarsJSON.String), &episode.GuestStars)
	}
	if nextEpisodeID.Valid {
		nextID := nextEpisodeID.Int64
		episode.NextEpisodeID = &nextID
	}
	if prevEpisodeID.Valid {
		prevID := prevEpisodeID.Int64
		episode.PrevEpisodeID = &prevID
	}

	video.EpisodeInfo = &episode
	return nil
}

func (s *VideoPlayerService) loadMovieInfo(ctx context.Context, video *VideoContent) error {
	if video.Type != VideoTypeMovie {
		return nil
	}

	query := `
		SELECT budget, revenue, runtime, collection, studio, production_companies
		FROM movies
		WHERE media_item_id = ?
	`

	var movie MovieInfo
	var studioJSON, companiesJSON sql.NullString

	err := s.db.QueryRowContext(ctx, query, video.ID).Scan(
		&movie.Budget, &movie.Revenue, &movie.Runtime, &movie.Collection,
		&studioJSON, &companiesJSON,
	)

	if err != nil {
		return err
	}

	if studioJSON.Valid {
		json.Unmarshal([]byte(studioJSON.String), &movie.Studio)
	}
	if companiesJSON.Valid {
		json.Unmarshal([]byte(companiesJSON.String), &movie.ProductionCompanies)
	}

	video.MovieInfo = &movie
	return nil
}

func (s *VideoPlayerService) loadVideoStreams(ctx context.Context, session *VideoPlaybackSession, videoID int64) error {
	query := `
		SELECT id, stream_index, codec, width, height, bitrate, fps, language, title, is_default
		FROM video_streams
		WHERE media_item_id = ?
		ORDER BY stream_index ASC
	`

	rows, err := s.db.QueryContext(ctx, query, videoID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var videoStreams []VideoStream
	for rows.Next() {
		var stream VideoStream
		err := rows.Scan(
			&stream.ID, &stream.Index, &stream.Codec, &stream.Width, &stream.Height,
			&stream.Bitrate, &stream.FPS, &stream.Language, &stream.Title, &stream.Default,
		)
		if err != nil {
			continue
		}
		videoStreams = append(videoStreams, stream)
	}

	session.CurrentVideo.VideoStreams = videoStreams

	audioQuery := `
		SELECT id, stream_index, codec, channels, bitrate, language, title, is_default
		FROM audio_streams
		WHERE media_item_id = ?
		ORDER BY stream_index ASC
	`

	audioRows, err := s.db.QueryContext(ctx, audioQuery, videoID)
	if err != nil {
		return err
	}
	defer audioRows.Close()

	var audioStreams []AudioStream
	var audioTracks []AudioTrack
	for audioRows.Next() {
		var stream AudioStream
		err := audioRows.Scan(
			&stream.ID, &stream.Index, &stream.Codec, &stream.Channels,
			&stream.Bitrate, &stream.Language, &stream.Title, &stream.Default,
		)
		if err != nil {
			continue
		}
		audioStreams = append(audioStreams, stream)

		audioTrack := AudioTrack{
			ID:        fmt.Sprintf("%d", stream.ID),
			Language:  stream.Language,
			Title:     &stream.Title,
			Codec:     stream.Codec,
			Channels:  stream.Channels,
			Bitrate:   &stream.Bitrate,
			IsDefault: stream.Default,
		}
		audioTracks = append(audioTracks, audioTrack)

		if stream.Default && session.ActiveAudioTrack == nil {
			session.ActiveAudioTrack = &stream.ID
		}
	}

	session.CurrentVideo.AudioStreams = audioStreams
	session.AudioTracks = audioTracks

	return nil
}

func (s *VideoPlayerService) loadSubtitles(ctx context.Context, session *VideoPlaybackSession, videoID int64) error {
	subtitleStreams, err := s.getSubtitleStreams(ctx, videoID)
	if err != nil {
		s.logger.Warn("Failed to get subtitle streams", zap.Error(err))
	} else {
		session.CurrentVideo.SubtitleStreams = subtitleStreams
	}

	subtitleTracks, err := s.getSubtitleTracks(ctx, videoID)
	if err != nil {
		s.logger.Warn("Failed to get subtitle tracks", zap.Error(err))
	} else {
		session.SubtitleTracks = subtitleTracks

		// Set active subtitle to the default track if available
		for _, track := range subtitleTracks {
			if track.IsDefault && session.ActiveSubtitle == nil {
				// Use track ID as the active subtitle identifier
				session.ActiveSubtitle = &track.ID
				s.logger.Debug("Set default subtitle track",
					zap.String("id", track.ID),
					zap.String("language", track.Language))
				break
			}
		}
	}

	return nil
}

func (s *VideoPlayerService) getSubtitleStreams(ctx context.Context, videoID int64) ([]SubtitleStream, error) {
	query := `
		SELECT id, stream_index, codec, language, title, is_default, is_forced, is_external, file_path
		FROM subtitle_streams
		WHERE media_item_id = ?
		ORDER BY stream_index ASC
	`

	rows, err := s.db.QueryContext(ctx, query, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var streams []SubtitleStream
	for rows.Next() {
		var stream SubtitleStream
		err := rows.Scan(
			&stream.ID, &stream.Index, &stream.Codec, &stream.Language,
			&stream.Title, &stream.Default, &stream.Forced, &stream.External, &stream.FilePath,
		)
		if err != nil {
			continue
		}
		streams = append(streams, stream)
	}

	return streams, nil
}

func (s *VideoPlayerService) getSubtitleTracks(ctx context.Context, videoID int64) ([]SubtitleTrack, error) {
	query := `
		SELECT id, language, language_code, source, format, path, content,
			   is_default, is_forced, encoding, sync_offset, created_at, verified_sync
		FROM subtitle_tracks
		WHERE media_item_id = ?
		ORDER BY is_default DESC, language ASC
	`

	rows, err := s.db.QueryContext(ctx, query, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tracks []SubtitleTrack
	for rows.Next() {
		var track SubtitleTrack
		var path sql.NullString
		var content sql.NullString

		err := rows.Scan(
			&track.ID, &track.Language, &track.LanguageCode, &track.Source,
			&track.Format, &path, &content, &track.IsDefault, &track.IsForced,
			&track.Encoding, &track.SyncOffset, &track.CreatedAt, &track.VerifiedSync,
		)
		if err != nil {
			continue
		}

		if path.Valid {
			track.Path = &path.String
		}
		if content.Valid {
			track.Content = &content.String
		}

		tracks = append(tracks, track)
	}

	return tracks, nil
}

func (s *VideoPlayerService) loadChapters(ctx context.Context, session *VideoPlaybackSession, videoID int64) error {
	query := `
		SELECT id, title, start_time, end_time, thumbnail_url
		FROM video_chapters
		WHERE media_item_id = ?
		ORDER BY start_time ASC
	`

	rows, err := s.db.QueryContext(ctx, query, videoID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var chapters []Chapter
	for rows.Next() {
		var chapter Chapter
		var thumbnailURL sql.NullString
		err := rows.Scan(
			&chapter.ID, &chapter.Title, &chapter.StartTime, &chapter.EndTime, &thumbnailURL,
		)
		if err != nil {
			continue
		}

		if thumbnailURL.Valid {
			chapter.Thumbnail = &thumbnailURL.String
		}

		chapters = append(chapters, chapter)
	}

	session.Chapters = chapters
	return nil
}

func (s *VideoPlayerService) loadEpisodePlaylist(ctx context.Context, session *VideoPlaybackSession, seriesID int64, seasonNumber int) error {
	episodes, err := s.getSeasonEpisodes(ctx, seriesID, seasonNumber)
	if err != nil {
		return err
	}

	session.Playlist = episodes
	for i, episode := range episodes {
		if episode.ID == session.CurrentVideo.ID {
			session.PlaylistIndex = i
			break
		}
	}

	return nil
}

func (s *VideoPlayerService) loadSeasonPlaylist(ctx context.Context, session *VideoPlaybackSession, seriesID int64, seasonNumber int) error {
	return s.loadEpisodePlaylist(ctx, session, seriesID, seasonNumber)
}

func (s *VideoPlayerService) loadSeriesPlaylist(ctx context.Context, session *VideoPlaybackSession, seriesID int64) error {
	query := `
		SELECT mi.id
		FROM media_items mi
		INNER JOIN episodes e ON mi.id = e.media_item_id
		WHERE e.series_id = ? AND mi.type = 'video'
		ORDER BY e.season_number ASC, e.episode_number ASC
	`

	rows, err := s.db.QueryContext(ctx, query, seriesID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var episodeIDs []int64
	for rows.Next() {
		var episodeID int64
		if err := rows.Scan(&episodeID); err == nil {
			episodeIDs = append(episodeIDs, episodeID)
		}
	}

	episodes, err := s.getVideoContents(ctx, episodeIDs)
	if err != nil {
		return err
	}

	session.Playlist = episodes
	for i, episode := range episodes {
		if episode.ID == session.CurrentVideo.ID {
			session.PlaylistIndex = i
			break
		}
	}

	return nil
}

func (s *VideoPlayerService) loadVideoPlaylist(ctx context.Context, session *VideoPlaybackSession, playlistID int64) error {
	return nil
}

func (s *VideoPlayerService) generateThumbnail(ctx context.Context, videoID, position int64) (*CoverArt, error) {
	video, err := s.getVideoContent(ctx, videoID)
	if err != nil {
		return nil, err
	}

	thumbnailRequest := &VideoThumbnailRequest{
		MediaItemID: videoID,
		VideoPath:   video.FilePath,
		Timestamps:  []float64{float64(position)},
		Quality:     QualityThumbnail,
		Count:       1,
	}

	thumbnails, err := s.coverArtService.GenerateVideoThumbnails(ctx, thumbnailRequest)
	if err != nil {
		return nil, err
	}

	if len(thumbnails) > 0 {
		return thumbnails[0], nil
	}

	return nil, fmt.Errorf("failed to generate thumbnail")
}

func (s *VideoPlayerService) saveVideoSession(ctx context.Context, session *VideoPlaybackSession) error {
	sessionData, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	expiresAt := time.Now().Add(24 * time.Hour)
	query := `
		INSERT OR REPLACE INTO video_playback_sessions (id, user_id, session_data, expires_at, updated_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	_, err = s.db.ExecContext(ctx, query, session.ID, session.UserID, string(sessionData), expiresAt)
	return err
}

func (s *VideoPlayerService) recordVideoPlayback(ctx context.Context, userID, videoID int64) error {
	query := `
		UPDATE media_items
		SET play_count = play_count + 1, last_played = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := s.db.ExecContext(ctx, query, videoID)
	if err != nil {
		return err
	}

	historyQuery := `
		INSERT INTO video_watch_history (user_id, video_id, watched_at, watch_duration, completion_rate, stopped_at, device_info, quality)
		VALUES (?, ?, CURRENT_TIMESTAMP, 0, 0, 0, '', '')
	`

	_, err = s.db.ExecContext(ctx, historyQuery, userID, videoID)
	return err
}
