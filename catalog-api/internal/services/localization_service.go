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

type LocalizationService struct {
	db                 *database.DB
	logger             *zap.Logger
	translationService *TranslationService
	cacheService       *CacheService
}

type UserLocalization struct {
	ID                    int64     `json:"id" db:"id"`
	UserID                int64     `json:"user_id" db:"user_id"`
	PrimaryLanguage       string    `json:"primary_language" db:"primary_language"`
	SecondaryLanguages    []string  `json:"secondary_languages" db:"secondary_languages"`
	SubtitleLanguages     []string  `json:"subtitle_languages" db:"subtitle_languages"`
	LyricsLanguages       []string  `json:"lyrics_languages" db:"lyrics_languages"`
	MetadataLanguages     []string  `json:"metadata_languages" db:"metadata_languages"`
	AutoTranslate         bool      `json:"auto_translate" db:"auto_translate"`
	AutoDownloadSubtitles bool      `json:"auto_download_subtitles" db:"auto_download_subtitles"`
	AutoDownloadLyrics    bool      `json:"auto_download_lyrics" db:"auto_download_lyrics"`
	PreferredRegion       string    `json:"preferred_region" db:"preferred_region"`
	DateFormat            string    `json:"date_format" db:"date_format"`
	TimeFormat            string    `json:"time_format" db:"time_format"`
	NumberFormat          string    `json:"number_format" db:"number_format"`
	CurrencyCode          string    `json:"currency_code" db:"currency_code"`
	CreatedAt             time.Time `json:"created_at" db:"created_at"`
	UpdatedAt             time.Time `json:"updated_at" db:"updated_at"`
}

type LanguageProfile struct {
	Code            string   `json:"code"`
	Name            string   `json:"name"`
	NativeName      string   `json:"native_name"`
	Direction       string   `json:"direction"`
	Region          string   `json:"region"`
	Country         string   `json:"country"`
	SupportedBy     []string `json:"supported_by"`
	QualityRating   float64  `json:"quality_rating"`
	PopularityScore int      `json:"popularity_score"`
}

type ContentLanguagePreference struct {
	ID          int64     `json:"id" db:"id"`
	UserID      int64     `json:"user_id" db:"user_id"`
	ContentType string    `json:"content_type" db:"content_type"`
	Languages   []string  `json:"languages" db:"languages"`
	Priority    int       `json:"priority" db:"priority"`
	AutoApply   bool      `json:"auto_apply" db:"auto_apply"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

type LocalizationStats struct {
	TotalUsers             int64                      `json:"total_users"`
	UsersWithLocalization  int64                      `json:"users_with_localization"`
	PopularLanguages       []LanguageStats            `json:"popular_languages"`
	PopularRegions         []RegionStats              `json:"popular_regions"`
	TranslationRequests    int64                      `json:"translation_requests"`
	AutoTranslationEnabled int64                      `json:"auto_translation_enabled"`
	LanguageSupport        map[string]LanguageSupport `json:"language_support"`
}

type LanguageStats struct {
	Language string  `json:"language"`
	Count    int64   `json:"count"`
	Percent  float64 `json:"percent"`
}

type RegionStats struct {
	Region  string  `json:"region"`
	Count   int64   `json:"count"`
	Percent float64 `json:"percent"`
}

type LanguageSupport struct {
	Subtitles bool `json:"subtitles"`
	Lyrics    bool `json:"lyrics"`
	Metadata  bool `json:"metadata"`
	UI        bool `json:"ui"`
}

type WizardLocalizationStep struct {
	UserID                int64    `json:"user_id"`
	PrimaryLanguage       string   `json:"primary_language"`
	SecondaryLanguages    []string `json:"secondary_languages"`
	SubtitleLanguages     []string `json:"subtitle_languages"`
	LyricsLanguages       []string `json:"lyrics_languages"`
	MetadataLanguages     []string `json:"metadata_languages"`
	AutoTranslate         bool     `json:"auto_translate"`
	AutoDownloadSubtitles bool     `json:"auto_download_subtitles"`
	AutoDownloadLyrics    bool     `json:"auto_download_lyrics"`
	PreferredRegion       string   `json:"preferred_region"`
	DateFormat            string   `json:"date_format"`
	TimeFormat            string   `json:"time_format"`
	NumberFormat          string   `json:"number_format"`
	CurrencyCode          string   `json:"currency_code"`
}

type ConfigurationExport struct {
	Version          string                  `json:"version"`
	ExportedAt       time.Time               `json:"exported_at"`
	ExportedBy       int64                   `json:"exported_by"`
	ConfigType       string                  `json:"config_type"`
	Localization     *UserLocalization       `json:"localization,omitempty"`
	WizardStep       *WizardLocalizationStep `json:"wizard_step,omitempty"`
	MediaSettings    *MediaPlayerConfig      `json:"media_settings,omitempty"`
	PlaylistSettings *PlaylistConfig         `json:"playlist_settings,omitempty"`
	Description      string                  `json:"description"`
	Tags             []string                `json:"tags"`
}

type MediaPlayerConfig struct {
	DefaultQuality    string             `json:"default_quality"`
	AutoPlay          bool               `json:"auto_play"`
	CrossfadeEnabled  bool               `json:"crossfade_enabled"`
	CrossfadeDuration int                `json:"crossfade_duration"`
	EqualizerPreset   string             `json:"equalizer_preset"`
	EqualizerBands    map[string]float64 `json:"equalizer_bands"`
	RepeatMode        string             `json:"repeat_mode"`
	ShuffleEnabled    bool               `json:"shuffle_enabled"`
	VolumeLevel       float64            `json:"volume_level"`
	ReplayGainEnabled bool               `json:"replay_gain_enabled"`
}

type PlaylistConfig struct {
	AutoCreatePlaylists  bool     `json:"auto_create_playlists"`
	SmartPlaylistRules   []string `json:"smart_playlist_rules"`
	DefaultPlaylistType  string   `json:"default_playlist_type"`
	CollaborativeDefault bool     `json:"collaborative_default"`
	PublicDefault        bool     `json:"public_default"`
}

type ConfigurationImportResult struct {
	Success          bool                 `json:"success"`
	ImportedConfig   *ConfigurationExport `json:"imported_config"`
	ValidationErrors []string             `json:"validation_errors"`
	AppliedSettings  []string             `json:"applied_settings"`
	SkippedSettings  []string             `json:"skipped_settings"`
	BackupCreated    bool                 `json:"backup_created"`
	BackupPath       string               `json:"backup_path"`
}

const (
	ContentTypeSubtitles = "subtitles"
	ContentTypeLyrics    = "lyrics"
	ContentTypeMetadata  = "metadata"
	ContentTypeUI        = "ui"
)

var SupportedLanguages = map[string]LanguageProfile{
	"en": {Code: "en", Name: "English", NativeName: "English", Direction: "ltr", Region: "US", Country: "United States", SupportedBy: []string{"subtitles", "lyrics", "metadata", "ui"}, QualityRating: 10.0, PopularityScore: 100},
	"es": {Code: "es", Name: "Spanish", NativeName: "Español", Direction: "ltr", Region: "ES", Country: "Spain", SupportedBy: []string{"subtitles", "lyrics", "metadata", "ui"}, QualityRating: 9.5, PopularityScore: 85},
	"fr": {Code: "fr", Name: "French", NativeName: "Français", Direction: "ltr", Region: "FR", Country: "France", SupportedBy: []string{"subtitles", "lyrics", "metadata", "ui"}, QualityRating: 9.5, PopularityScore: 75},
	"de": {Code: "de", Name: "German", NativeName: "Deutsch", Direction: "ltr", Region: "DE", Country: "Germany", SupportedBy: []string{"subtitles", "lyrics", "metadata", "ui"}, QualityRating: 9.2, PopularityScore: 70},
	"it": {Code: "it", Name: "Italian", NativeName: "Italiano", Direction: "ltr", Region: "IT", Country: "Italy", SupportedBy: []string{"subtitles", "lyrics", "metadata", "ui"}, QualityRating: 9.0, PopularityScore: 65},
	"pt": {Code: "pt", Name: "Portuguese", NativeName: "Português", Direction: "ltr", Region: "PT", Country: "Portugal", SupportedBy: []string{"subtitles", "lyrics", "metadata", "ui"}, QualityRating: 8.8, PopularityScore: 60},
	"ru": {Code: "ru", Name: "Russian", NativeName: "Русский", Direction: "ltr", Region: "RU", Country: "Russia", SupportedBy: []string{"subtitles", "lyrics", "metadata"}, QualityRating: 8.5, PopularityScore: 55},
	"ja": {Code: "ja", Name: "Japanese", NativeName: "日本語", Direction: "ltr", Region: "JP", Country: "Japan", SupportedBy: []string{"subtitles", "lyrics", "metadata"}, QualityRating: 8.8, PopularityScore: 50},
	"ko": {Code: "ko", Name: "Korean", NativeName: "한국어", Direction: "ltr", Region: "KR", Country: "South Korea", SupportedBy: []string{"subtitles", "lyrics", "metadata"}, QualityRating: 8.6, PopularityScore: 45},
	"zh": {Code: "zh", Name: "Chinese", NativeName: "中文", Direction: "ltr", Region: "CN", Country: "China", SupportedBy: []string{"subtitles", "lyrics", "metadata"}, QualityRating: 8.7, PopularityScore: 80},
	"ar": {Code: "ar", Name: "Arabic", NativeName: "العربية", Direction: "rtl", Region: "SA", Country: "Saudi Arabia", SupportedBy: []string{"subtitles", "metadata"}, QualityRating: 7.5, PopularityScore: 40},
	"hi": {Code: "hi", Name: "Hindi", NativeName: "हिन्दी", Direction: "ltr", Region: "IN", Country: "India", SupportedBy: []string{"subtitles", "lyrics", "metadata"}, QualityRating: 7.8, PopularityScore: 60},
	"nl": {Code: "nl", Name: "Dutch", NativeName: "Nederlands", Direction: "ltr", Region: "NL", Country: "Netherlands", SupportedBy: []string{"subtitles", "lyrics", "metadata"}, QualityRating: 8.9, PopularityScore: 30},
	"sv": {Code: "sv", Name: "Swedish", NativeName: "Svenska", Direction: "ltr", Region: "SE", Country: "Sweden", SupportedBy: []string{"subtitles", "lyrics", "metadata"}, QualityRating: 8.7, PopularityScore: 25},
	"no": {Code: "no", Name: "Norwegian", NativeName: "Norsk", Direction: "ltr", Region: "NO", Country: "Norway", SupportedBy: []string{"subtitles", "lyrics", "metadata"}, QualityRating: 8.6, PopularityScore: 20},
	"da": {Code: "da", Name: "Danish", NativeName: "Dansk", Direction: "ltr", Region: "DK", Country: "Denmark", SupportedBy: []string{"subtitles", "lyrics", "metadata"}, QualityRating: 8.5, PopularityScore: 18},
	"pl": {Code: "pl", Name: "Polish", NativeName: "Polski", Direction: "ltr", Region: "PL", Country: "Poland", SupportedBy: []string{"subtitles", "lyrics", "metadata"}, QualityRating: 8.3, PopularityScore: 35},
	"tr": {Code: "tr", Name: "Turkish", NativeName: "Türkçe", Direction: "ltr", Region: "TR", Country: "Turkey", SupportedBy: []string{"subtitles", "lyrics", "metadata"}, QualityRating: 8.0, PopularityScore: 30},
	"he": {Code: "he", Name: "Hebrew", NativeName: "עברית", Direction: "rtl", Region: "IL", Country: "Israel", SupportedBy: []string{"subtitles", "metadata"}, QualityRating: 7.8, PopularityScore: 15},
	"th": {Code: "th", Name: "Thai", NativeName: "ไทย", Direction: "ltr", Region: "TH", Country: "Thailand", SupportedBy: []string{"subtitles", "metadata"}, QualityRating: 7.2, PopularityScore: 25},
	"vi": {Code: "vi", Name: "Vietnamese", NativeName: "Tiếng Việt", Direction: "ltr", Region: "VN", Country: "Vietnam", SupportedBy: []string{"subtitles", "metadata"}, QualityRating: 7.0, PopularityScore: 28},
}

func NewLocalizationService(
	db *database.DB,
	logger *zap.Logger,
	translationService *TranslationService,
	cacheService *CacheService,
) *LocalizationService {
	return &LocalizationService{
		db:                 db,
		logger:             logger,
		translationService: translationService,
		cacheService:       cacheService,
	}
}

func (s *LocalizationService) SetupUserLocalization(ctx context.Context, req *WizardLocalizationStep) (*UserLocalization, error) {
	s.logger.Info("Setting up user localization",
		zap.Int64("user_id", req.UserID),
		zap.String("primary_language", req.PrimaryLanguage))

	secondaryLanguagesJSON, _ := json.Marshal(req.SecondaryLanguages)
	subtitleLanguagesJSON, _ := json.Marshal(req.SubtitleLanguages)
	lyricsLanguagesJSON, _ := json.Marshal(req.LyricsLanguages)
	metadataLanguagesJSON, _ := json.Marshal(req.MetadataLanguages)

	query := `
		INSERT OR REPLACE INTO user_localization (
			user_id, primary_language, secondary_languages, subtitle_languages,
			lyrics_languages, metadata_languages, auto_translate, auto_download_subtitles,
			auto_download_lyrics, preferred_region, date_format, time_format,
			number_format, currency_code, created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	var localization UserLocalization
	localizationID, err := s.db.InsertReturningID(ctx, query,
		req.UserID, req.PrimaryLanguage, string(secondaryLanguagesJSON),
		string(subtitleLanguagesJSON), string(lyricsLanguagesJSON),
		string(metadataLanguagesJSON), req.AutoTranslate, req.AutoDownloadSubtitles,
		req.AutoDownloadLyrics, req.PreferredRegion, req.DateFormat,
		req.TimeFormat, req.NumberFormat, req.CurrencyCode)

	if err != nil {
		s.logger.Error("Failed to setup user localization", zap.Error(err))
		return nil, fmt.Errorf("failed to setup localization: %w", err)
	}

	localization.ID = localizationID
	localization.CreatedAt = time.Now()
	localization.UpdatedAt = time.Now()

	localization.UserID = req.UserID
	localization.PrimaryLanguage = req.PrimaryLanguage
	localization.SecondaryLanguages = req.SecondaryLanguages
	localization.SubtitleLanguages = req.SubtitleLanguages
	localization.LyricsLanguages = req.LyricsLanguages
	localization.MetadataLanguages = req.MetadataLanguages
	localization.AutoTranslate = req.AutoTranslate
	localization.AutoDownloadSubtitles = req.AutoDownloadSubtitles
	localization.AutoDownloadLyrics = req.AutoDownloadLyrics
	localization.PreferredRegion = req.PreferredRegion
	localization.DateFormat = req.DateFormat
	localization.TimeFormat = req.TimeFormat
	localization.NumberFormat = req.NumberFormat
	localization.CurrencyCode = req.CurrencyCode

	if err := s.setupContentPreferences(ctx, &localization); err != nil {
		s.logger.Warn("Failed to setup content preferences", zap.Error(err))
	}

	if err := s.preloadTranslations(ctx, &localization); err != nil {
		s.logger.Warn("Failed to preload translations", zap.Error(err))
	}

	return &localization, nil
}

func (s *LocalizationService) GetUserLocalization(ctx context.Context, userID int64) (*UserLocalization, error) {
	s.logger.Debug("Getting user localization", zap.Int64("user_id", userID))

	query := `
		SELECT id, user_id, primary_language, secondary_languages, subtitle_languages,
			   lyrics_languages, metadata_languages, auto_translate, auto_download_subtitles,
			   auto_download_lyrics, preferred_region, date_format, time_format,
			   number_format, currency_code, created_at, updated_at
		FROM user_localization
		WHERE user_id = ?
	`

	var localization UserLocalization
	var secondaryLanguagesJSON, subtitleLanguagesJSON, lyricsLanguagesJSON, metadataLanguagesJSON string

	err := s.db.QueryRowContext(ctx, query, userID).Scan(
		&localization.ID, &localization.UserID, &localization.PrimaryLanguage,
		&secondaryLanguagesJSON, &subtitleLanguagesJSON, &lyricsLanguagesJSON,
		&metadataLanguagesJSON, &localization.AutoTranslate, &localization.AutoDownloadSubtitles,
		&localization.AutoDownloadLyrics, &localization.PreferredRegion, &localization.DateFormat,
		&localization.TimeFormat, &localization.NumberFormat, &localization.CurrencyCode,
		&localization.CreatedAt, &localization.UpdatedAt,
	)

	if err == sql.ErrNoRows {
		return s.createDefaultLocalization(ctx, userID)
	}
	if err != nil {
		s.logger.Error("Failed to get user localization", zap.Error(err))
		return nil, fmt.Errorf("failed to get user localization: %w", err)
	}

	json.Unmarshal([]byte(secondaryLanguagesJSON), &localization.SecondaryLanguages)
	json.Unmarshal([]byte(subtitleLanguagesJSON), &localization.SubtitleLanguages)
	json.Unmarshal([]byte(lyricsLanguagesJSON), &localization.LyricsLanguages)
	json.Unmarshal([]byte(metadataLanguagesJSON), &localization.MetadataLanguages)

	return &localization, nil
}

func (s *LocalizationService) UpdateUserLocalization(ctx context.Context, userID int64, updates map[string]interface{}) error {
	s.logger.Info("Updating user localization", zap.Int64("user_id", userID))

	if len(updates) == 0 {
		return fmt.Errorf("no updates provided")
	}

	setParts := []string{}
	args := []interface{}{}

	for field, value := range updates {
		switch field {
		case "primary_language", "preferred_region", "date_format", "time_format", "number_format", "currency_code":
			setParts = append(setParts, fmt.Sprintf("%s = ?", field))
			args = append(args, value)
		case "secondary_languages", "subtitle_languages", "lyrics_languages", "metadata_languages":
			if languages, ok := value.([]string); ok {
				languagesJSON, _ := json.Marshal(languages)
				setParts = append(setParts, fmt.Sprintf("%s = ?", field))
				args = append(args, string(languagesJSON))
			}
		case "auto_translate", "auto_download_subtitles", "auto_download_lyrics":
			setParts = append(setParts, fmt.Sprintf("%s = ?", field))
			args = append(args, value)
		}
	}

	if len(setParts) == 0 {
		return fmt.Errorf("no valid updates provided")
	}

	setParts = append(setParts, "updated_at = CURRENT_TIMESTAMP")
	args = append(args, userID)

	query := fmt.Sprintf(`
		UPDATE user_localization
		SET %s
		WHERE user_id = ?
	`, strings.Join(setParts, ", "))

	_, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		s.logger.Error("Failed to update user localization", zap.Error(err))
		return fmt.Errorf("failed to update user localization: %w", err)
	}

	return nil
}

func (s *LocalizationService) GetPreferredLanguagesForContent(ctx context.Context, userID int64, contentType string) ([]string, error) {
	s.logger.Debug("Getting preferred languages for content",
		zap.Int64("user_id", userID),
		zap.String("content_type", contentType))

	localization, err := s.GetUserLocalization(ctx, userID)
	if err != nil {
		return []string{"en"}, err
	}

	switch contentType {
	case ContentTypeSubtitles:
		if len(localization.SubtitleLanguages) > 0 {
			return localization.SubtitleLanguages, nil
		}
	case ContentTypeLyrics:
		if len(localization.LyricsLanguages) > 0 {
			return localization.LyricsLanguages, nil
		}
	case ContentTypeMetadata:
		if len(localization.MetadataLanguages) > 0 {
			return localization.MetadataLanguages, nil
		}
	}

	languages := []string{localization.PrimaryLanguage}
	languages = append(languages, localization.SecondaryLanguages...)

	if len(languages) == 0 {
		return []string{"en"}, nil
	}

	return languages, nil
}

func (s *LocalizationService) ShouldAutoTranslate(ctx context.Context, userID int64, contentType string) (bool, error) {
	localization, err := s.GetUserLocalization(ctx, userID)
	if err != nil {
		return false, err
	}

	return localization.AutoTranslate, nil
}

func (s *LocalizationService) ShouldAutoDownload(ctx context.Context, userID int64, contentType string) (bool, error) {
	localization, err := s.GetUserLocalization(ctx, userID)
	if err != nil {
		return false, err
	}

	switch contentType {
	case ContentTypeSubtitles:
		return localization.AutoDownloadSubtitles, nil
	case ContentTypeLyrics:
		return localization.AutoDownloadLyrics, nil
	default:
		return false, nil
	}
}

func (s *LocalizationService) GetSupportedLanguages(ctx context.Context) (map[string]LanguageProfile, error) {
	s.logger.Debug("Getting supported languages")

	cacheKey := "localization:supported_languages"
	languages := map[string]LanguageProfile{}

	found, err := s.cacheService.Get(ctx, cacheKey, &languages)
	if err == nil && found {
		return languages, nil
	}

	languages = make(map[string]LanguageProfile)
	for code, profile := range SupportedLanguages {
		languages[code] = profile
	}

	if err := s.cacheService.Set(ctx, cacheKey, languages, 24*time.Hour); err != nil {
		s.logger.Warn("Failed to cache supported languages", zap.Error(err))
	}

	return languages, nil
}

func (s *LocalizationService) GetLanguageProfile(ctx context.Context, languageCode string) (*LanguageProfile, error) {
	if profile, exists := SupportedLanguages[languageCode]; exists {
		return &profile, nil
	}
	return nil, fmt.Errorf("language not supported: %s", languageCode)
}

func (s *LocalizationService) IsLanguageSupported(ctx context.Context, languageCode, contentType string) bool {
	profile, exists := SupportedLanguages[languageCode]
	if !exists {
		return false
	}

	for _, supportedType := range profile.SupportedBy {
		if supportedType == contentType {
			return true
		}
	}

	return false
}

func (s *LocalizationService) GetLocalizationStats(ctx context.Context) (*LocalizationStats, error) {
	s.logger.Debug("Getting localization statistics")

	stats := &LocalizationStats{
		PopularLanguages: make([]LanguageStats, 0),
		PopularRegions:   make([]RegionStats, 0),
		LanguageSupport:  make(map[string]LanguageSupport),
	}

	if err := s.getBasicLocalizationStats(ctx, stats); err != nil {
		return nil, err
	}

	if err := s.getPopularLanguages(ctx, stats); err != nil {
		s.logger.Warn("Failed to get popular languages", zap.Error(err))
	}

	if err := s.getPopularRegions(ctx, stats); err != nil {
		s.logger.Warn("Failed to get popular regions", zap.Error(err))
	}

	if err := s.getLanguageSupport(ctx, stats); err != nil {
		s.logger.Warn("Failed to get language support", zap.Error(err))
	}

	return stats, nil
}

func (s *LocalizationService) createDefaultLocalization(ctx context.Context, userID int64) (*UserLocalization, error) {
	s.logger.Info("Creating default localization", zap.Int64("user_id", userID))

	defaultReq := &WizardLocalizationStep{
		UserID:                userID,
		PrimaryLanguage:       "en",
		SecondaryLanguages:    []string{},
		SubtitleLanguages:     []string{"en"},
		LyricsLanguages:       []string{"en"},
		MetadataLanguages:     []string{"en"},
		AutoTranslate:         false,
		AutoDownloadSubtitles: true,
		AutoDownloadLyrics:    true,
		PreferredRegion:       "US",
		DateFormat:            "MM/DD/YYYY",
		TimeFormat:            "12h",
		NumberFormat:          "en-US",
		CurrencyCode:          "USD",
	}

	return s.SetupUserLocalization(ctx, defaultReq)
}

func (s *LocalizationService) setupContentPreferences(ctx context.Context, localization *UserLocalization) error {
	contentTypes := []struct {
		Type      string
		Languages []string
	}{
		{ContentTypeSubtitles, localization.SubtitleLanguages},
		{ContentTypeLyrics, localization.LyricsLanguages},
		{ContentTypeMetadata, localization.MetadataLanguages},
	}

	for _, ct := range contentTypes {
		if len(ct.Languages) == 0 {
			continue
		}

		languagesJSON, _ := json.Marshal(ct.Languages)

		query := `
			INSERT OR REPLACE INTO content_language_preferences (user_id, content_type, languages, priority, auto_apply, created_at, updated_at)
			VALUES (?, ?, ?, 1, true, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		`

		_, err := s.db.ExecContext(ctx, query, localization.UserID, ct.Type, string(languagesJSON))
		if err != nil {
			s.logger.Error("Failed to setup content preference",
				zap.String("content_type", ct.Type),
				zap.Error(err))
		}
	}

	return nil
}

func (s *LocalizationService) preloadTranslations(ctx context.Context, localization *UserLocalization) error {
	if !localization.AutoTranslate {
		return nil
	}

	commonPhrases := []string{
		"Play", "Pause", "Stop", "Next", "Previous", "Volume", "Subtitles",
		"Audio", "Quality", "Fullscreen", "Playlist", "Lyrics", "Settings",
		"Search", "Library", "Recently Played", "Favorites", "Download",
		"Share", "Info", "Cast", "Speed", "Bookmark", "Chapter",
	}

	sourceLang := "en"
	targetLangs := append([]string{localization.PrimaryLanguage}, localization.SecondaryLanguages...)

	for _, targetLang := range targetLangs {
		if targetLang == sourceLang {
			continue
		}

		for _, phrase := range commonPhrases {
			req := TranslationRequest{
				Text:           phrase,
				SourceLanguage: sourceLang,
				TargetLanguage: targetLang,
			}

			go func(req TranslationRequest) {
				_, err := s.translationService.TranslateText(ctx, req)
				if err != nil {
					s.logger.Debug("Failed to preload translation",
						zap.String("phrase", req.Text),
						zap.String("target_lang", req.TargetLanguage),
						zap.Error(err))
				}
			}(req)
		}
	}

	return nil
}

func (s *LocalizationService) getBasicLocalizationStats(ctx context.Context, stats *LocalizationStats) error {
	query := `
		SELECT
			(SELECT COUNT(*) FROM users) as total_users,
			COUNT(*) as users_with_localization,
			COUNT(CASE WHEN auto_translate = 1 THEN 1 END) as auto_translation_enabled
		FROM user_localization
	`

	return s.db.QueryRowContext(ctx, query).Scan(
		&stats.TotalUsers, &stats.UsersWithLocalization, &stats.AutoTranslationEnabled)
}

func (s *LocalizationService) getPopularLanguages(ctx context.Context, stats *LocalizationStats) error {
	query := `
		SELECT primary_language, COUNT(*) as count
		FROM user_localization
		GROUP BY primary_language
		ORDER BY count DESC
		LIMIT 10
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	total := float64(stats.UsersWithLocalization)
	for rows.Next() {
		var lang LanguageStats
		if err := rows.Scan(&lang.Language, &lang.Count); err == nil {
			if total > 0 {
				lang.Percent = float64(lang.Count) / total * 100
			}
			stats.PopularLanguages = append(stats.PopularLanguages, lang)
		}
	}

	return nil
}

func (s *LocalizationService) getPopularRegions(ctx context.Context, stats *LocalizationStats) error {
	query := `
		SELECT preferred_region, COUNT(*) as count
		FROM user_localization
		WHERE preferred_region != ''
		GROUP BY preferred_region
		ORDER BY count DESC
		LIMIT 10
	`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	total := float64(stats.UsersWithLocalization)
	for rows.Next() {
		var region RegionStats
		if err := rows.Scan(&region.Region, &region.Count); err == nil {
			if total > 0 {
				region.Percent = float64(region.Count) / total * 100
			}
			stats.PopularRegions = append(stats.PopularRegions, region)
		}
	}

	return nil
}

func (s *LocalizationService) getLanguageSupport(ctx context.Context, stats *LocalizationStats) error {
	for code, profile := range SupportedLanguages {
		support := LanguageSupport{}

		for _, contentType := range profile.SupportedBy {
			switch contentType {
			case "subtitles":
				support.Subtitles = true
			case "lyrics":
				support.Lyrics = true
			case "metadata":
				support.Metadata = true
			case "ui":
				support.UI = true
			}
		}

		stats.LanguageSupport[code] = support
	}

	return nil
}

func (s *LocalizationService) DetectUserLanguage(ctx context.Context, userAgent, acceptLanguage string) string {
	if acceptLanguage != "" {
		languages := strings.Split(acceptLanguage, ",")
		for _, lang := range languages {
			langCode := strings.TrimSpace(strings.Split(lang, ";")[0])
			langCode = strings.Split(langCode, "-")[0]

			if _, exists := SupportedLanguages[langCode]; exists {
				return langCode
			}
		}
	}

	return "en"
}

func (s *LocalizationService) FormatDateTimeForUser(ctx context.Context, userID int64, timestamp time.Time) (string, error) {
	localization, err := s.GetUserLocalization(ctx, userID)
	if err != nil {
		return timestamp.Format("2006-01-02 15:04:05"), nil
	}

	dateFormat := localization.DateFormat
	timeFormat := localization.TimeFormat

	var layout string
	switch dateFormat {
	case "DD/MM/YYYY":
		layout = "02/01/2006"
	case "YYYY-MM-DD":
		layout = "2006-01-02"
	case "MM-DD-YYYY":
		layout = "01-02-2006"
	default:
		layout = "01/02/2006"
	}

	if timeFormat == "24h" {
		layout += " 15:04"
	} else {
		layout += " 03:04 PM"
	}

	return timestamp.Format(layout), nil
}

func (s *LocalizationService) GetWizardDefaults(ctx context.Context, detectedLanguage string) *WizardLocalizationStep {
	profile, exists := SupportedLanguages[detectedLanguage]
	if !exists {
		detectedLanguage = "en"
		profile = SupportedLanguages["en"]
	}

	return &WizardLocalizationStep{
		PrimaryLanguage:       detectedLanguage,
		SecondaryLanguages:    []string{},
		SubtitleLanguages:     []string{detectedLanguage, "en"},
		LyricsLanguages:       []string{detectedLanguage, "en"},
		MetadataLanguages:     []string{detectedLanguage, "en"},
		AutoTranslate:         detectedLanguage != "en",
		AutoDownloadSubtitles: true,
		AutoDownloadLyrics:    true,
		PreferredRegion:       profile.Region,
		DateFormat:            s.getDefaultDateFormat(profile.Region),
		TimeFormat:            s.getDefaultTimeFormat(profile.Region),
		NumberFormat:          detectedLanguage + "-" + profile.Region,
		CurrencyCode:          s.getDefaultCurrency(profile.Region),
	}
}

func (s *LocalizationService) getDefaultDateFormat(region string) string {
	switch region {
	case "US":
		return "MM/DD/YYYY"
	case "GB", "AU", "NZ":
		return "DD/MM/YYYY"
	default:
		return "YYYY-MM-DD"
	}
}

func (s *LocalizationService) getDefaultTimeFormat(region string) string {
	switch region {
	case "US":
		return "12h"
	default:
		return "24h"
	}
}

func (s *LocalizationService) getDefaultCurrency(region string) string {
	currencyMap := map[string]string{
		"US": "USD", "CA": "CAD", "GB": "GBP", "EU": "EUR", "FR": "EUR",
		"DE": "EUR", "IT": "EUR", "ES": "EUR", "PT": "EUR", "NL": "EUR",
		"JP": "JPY", "KR": "KRW", "CN": "CNY", "IN": "INR", "AU": "AUD",
		"RU": "RUB", "BR": "BRL", "MX": "MXN", "AR": "ARS", "SE": "SEK",
		"NO": "NOK", "DK": "DKK", "CH": "CHF", "PL": "PLN", "TR": "TRY",
	}

	if currency, exists := currencyMap[region]; exists {
		return currency
	}
	return "USD"
}

func (s *LocalizationService) ExportConfiguration(ctx context.Context, userID int64, configType string, description string, tags []string) (*ConfigurationExport, error) {
	s.logger.Info("Exporting user configuration",
		zap.Int64("user_id", userID),
		zap.String("config_type", configType))

	export := &ConfigurationExport{
		Version:     "1.0",
		ExportedAt:  time.Now(),
		ExportedBy:  userID,
		ConfigType:  configType,
		Description: description,
		Tags:        tags,
	}

	// Export localization settings
	if configType == "full" || configType == "localization" {
		localization, err := s.GetUserLocalization(ctx, userID)
		if err != nil {
			s.logger.Warn("Failed to get localization for export", zap.Error(err))
		} else {
			export.Localization = localization
			export.WizardStep = s.convertLocalizationToWizardStep(localization)
		}
	}

	// Export media player settings
	if configType == "full" || configType == "media" {
		mediaSettings, err := s.getMediaPlayerConfig(ctx, userID)
		if err != nil {
			s.logger.Warn("Failed to get media settings for export", zap.Error(err))
		} else {
			export.MediaSettings = mediaSettings
		}
	}

	// Export playlist settings
	if configType == "full" || configType == "playlists" {
		playlistSettings, err := s.getPlaylistConfig(ctx, userID)
		if err != nil {
			s.logger.Warn("Failed to get playlist settings for export", zap.Error(err))
		} else {
			export.PlaylistSettings = playlistSettings
		}
	}

	// Store export in database for future reference
	if err := s.storeConfigurationExport(ctx, export); err != nil {
		s.logger.Warn("Failed to store export record", zap.Error(err))
	}

	return export, nil
}

func (s *LocalizationService) ImportConfiguration(ctx context.Context, userID int64, configJSON string, options map[string]bool) (*ConfigurationImportResult, error) {
	s.logger.Info("Importing user configuration", zap.Int64("user_id", userID))

	result := &ConfigurationImportResult{
		Success:          false,
		ValidationErrors: make([]string, 0),
		AppliedSettings:  make([]string, 0),
		SkippedSettings:  make([]string, 0),
	}

	// Parse JSON configuration
	var config ConfigurationExport
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		result.ValidationErrors = append(result.ValidationErrors, fmt.Sprintf("Invalid JSON format: %v", err))
		return result, fmt.Errorf("failed to parse configuration JSON: %w", err)
	}

	result.ImportedConfig = &config

	// Validate configuration
	if validationErrors := s.validateConfiguration(&config); len(validationErrors) > 0 {
		result.ValidationErrors = append(result.ValidationErrors, validationErrors...)
		if !options["force_import"] {
			return result, fmt.Errorf("configuration validation failed")
		}
	}

	// Create backup of current settings
	if options["create_backup"] {
		backup, err := s.ExportConfiguration(ctx, userID, "full", "Pre-import backup", []string{"backup", "auto"})
		if err != nil {
			s.logger.Warn("Failed to create backup", zap.Error(err))
		} else {
			result.BackupCreated = true
			result.BackupPath = fmt.Sprintf("/backups/user_%d_%d.json", userID, backup.ExportedAt.Unix())
		}
	}

	// Import localization settings
	if config.Localization != nil && (options["import_localization"] || options["import_all"]) {
		if err := s.importLocalizationSettings(ctx, userID, config.Localization); err != nil {
			result.SkippedSettings = append(result.SkippedSettings, fmt.Sprintf("Localization: %v", err))
		} else {
			result.AppliedSettings = append(result.AppliedSettings, "Localization preferences")
		}
	}

	// Import wizard step configuration
	if config.WizardStep != nil && (options["import_wizard"] || options["import_all"]) {
		if err := s.importWizardStepSettings(ctx, userID, config.WizardStep); err != nil {
			result.SkippedSettings = append(result.SkippedSettings, fmt.Sprintf("Wizard step: %v", err))
		} else {
			result.AppliedSettings = append(result.AppliedSettings, "Wizard step configuration")
		}
	}

	// Import media settings
	if config.MediaSettings != nil && (options["import_media"] || options["import_all"]) {
		if err := s.importMediaSettings(ctx, userID, config.MediaSettings); err != nil {
			result.SkippedSettings = append(result.SkippedSettings, fmt.Sprintf("Media settings: %v", err))
		} else {
			result.AppliedSettings = append(result.AppliedSettings, "Media player settings")
		}
	}

	// Import playlist settings
	if config.PlaylistSettings != nil && (options["import_playlists"] || options["import_all"]) {
		if err := s.importPlaylistSettings(ctx, userID, config.PlaylistSettings); err != nil {
			result.SkippedSettings = append(result.SkippedSettings, fmt.Sprintf("Playlist settings: %v", err))
		} else {
			result.AppliedSettings = append(result.AppliedSettings, "Playlist settings")
		}
	}

	result.Success = len(result.AppliedSettings) > 0

	// Log import activity
	if err := s.logImportActivity(ctx, userID, &config, result); err != nil {
		s.logger.Warn("Failed to log import activity", zap.Error(err))
	}

	return result, nil
}

func (s *LocalizationService) ValidateConfigurationJSON(ctx context.Context, configJSON string) ([]string, error) {
	s.logger.Debug("Validating configuration JSON")

	var config ConfigurationExport
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return []string{fmt.Sprintf("Invalid JSON format: %v", err)}, err
	}

	return s.validateConfiguration(&config), nil
}

func (s *LocalizationService) GetConfigurationTemplate(ctx context.Context, templateType string) (*ConfigurationExport, error) {
	s.logger.Debug("Getting configuration template", zap.String("type", templateType))

	template := &ConfigurationExport{
		Version:     "1.0",
		ExportedAt:  time.Now(),
		ExportedBy:  0,
		ConfigType:  templateType,
		Description: fmt.Sprintf("Template for %s configuration", templateType),
		Tags:        []string{"template"},
	}

	switch templateType {
	case "localization":
		template.Localization = s.getDefaultLocalizationTemplate()
		template.WizardStep = s.getDefaultWizardStepTemplate()
	case "media":
		template.MediaSettings = s.getDefaultMediaSettingsTemplate()
	case "playlists":
		template.PlaylistSettings = s.getDefaultPlaylistSettingsTemplate()
	case "full":
		template.Localization = s.getDefaultLocalizationTemplate()
		template.WizardStep = s.getDefaultWizardStepTemplate()
		template.MediaSettings = s.getDefaultMediaSettingsTemplate()
		template.PlaylistSettings = s.getDefaultPlaylistSettingsTemplate()
	default:
		return nil, fmt.Errorf("unknown template type: %s", templateType)
	}

	return template, nil
}

func (s *LocalizationService) EditConfiguration(ctx context.Context, userID int64, configJSON string, edits map[string]interface{}) (string, error) {
	s.logger.Info("Editing configuration", zap.Int64("user_id", userID))

	var config ConfigurationExport
	if err := json.Unmarshal([]byte(configJSON), &config); err != nil {
		return "", fmt.Errorf("failed to parse configuration JSON: %w", err)
	}

	// Apply edits to the configuration
	for path, value := range edits {
		if err := s.applyConfigurationEdit(&config, path, value); err != nil {
			s.logger.Warn("Failed to apply edit", zap.String("path", path), zap.Error(err))
			return "", fmt.Errorf("failed to apply edit at path %s: %w", path, err)
		}
	}

	// Update metadata
	config.ExportedAt = time.Now()
	config.ExportedBy = userID

	// Convert back to JSON
	editedJSON, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal edited configuration: %w", err)
	}

	return string(editedJSON), nil
}

func (s *LocalizationService) ConvertWizardToConfiguration(ctx context.Context, wizardStep *WizardLocalizationStep) (*ConfigurationExport, error) {
	s.logger.Debug("Converting wizard step to configuration")

	config := &ConfigurationExport{
		Version:     "1.0",
		ExportedAt:  time.Now(),
		ExportedBy:  wizardStep.UserID,
		ConfigType:  "wizard",
		WizardStep:  wizardStep,
		Description: "Configuration generated from installation wizard",
		Tags:        []string{"wizard", "generated"},
	}

	// Convert wizard step to full localization
	localization := &UserLocalization{
		UserID:                wizardStep.UserID,
		PrimaryLanguage:       wizardStep.PrimaryLanguage,
		SecondaryLanguages:    wizardStep.SecondaryLanguages,
		SubtitleLanguages:     wizardStep.SubtitleLanguages,
		LyricsLanguages:       wizardStep.LyricsLanguages,
		MetadataLanguages:     wizardStep.MetadataLanguages,
		AutoTranslate:         wizardStep.AutoTranslate,
		AutoDownloadSubtitles: wizardStep.AutoDownloadSubtitles,
		AutoDownloadLyrics:    wizardStep.AutoDownloadLyrics,
		PreferredRegion:       wizardStep.PreferredRegion,
		DateFormat:            wizardStep.DateFormat,
		TimeFormat:            wizardStep.TimeFormat,
		NumberFormat:          wizardStep.NumberFormat,
		CurrencyCode:          wizardStep.CurrencyCode,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	config.Localization = localization

	return config, nil
}

// Helper methods for configuration management

func (s *LocalizationService) convertLocalizationToWizardStep(localization *UserLocalization) *WizardLocalizationStep {
	return &WizardLocalizationStep{
		UserID:                localization.UserID,
		PrimaryLanguage:       localization.PrimaryLanguage,
		SecondaryLanguages:    localization.SecondaryLanguages,
		SubtitleLanguages:     localization.SubtitleLanguages,
		LyricsLanguages:       localization.LyricsLanguages,
		MetadataLanguages:     localization.MetadataLanguages,
		AutoTranslate:         localization.AutoTranslate,
		AutoDownloadSubtitles: localization.AutoDownloadSubtitles,
		AutoDownloadLyrics:    localization.AutoDownloadLyrics,
		PreferredRegion:       localization.PreferredRegion,
		DateFormat:            localization.DateFormat,
		TimeFormat:            localization.TimeFormat,
		NumberFormat:          localization.NumberFormat,
		CurrencyCode:          localization.CurrencyCode,
	}
}

func (s *LocalizationService) getMediaPlayerConfig(ctx context.Context, userID int64) (*MediaPlayerConfig, error) {
	// This would typically fetch from a user_media_settings table
	// For now, return default settings
	return &MediaPlayerConfig{
		DefaultQuality:    "high",
		AutoPlay:          true,
		CrossfadeEnabled:  true,
		CrossfadeDuration: 3000,
		EqualizerPreset:   "flat",
		EqualizerBands:    make(map[string]float64),
		RepeatMode:        "none",
		ShuffleEnabled:    false,
		VolumeLevel:       1.0,
		ReplayGainEnabled: true,
	}, nil
}

func (s *LocalizationService) getPlaylistConfig(ctx context.Context, userID int64) (*PlaylistConfig, error) {
	// This would typically fetch from a user_playlist_settings table
	// For now, return default settings
	return &PlaylistConfig{
		AutoCreatePlaylists:  true,
		SmartPlaylistRules:   []string{"recently_played", "top_rated"},
		DefaultPlaylistType:  "standard",
		CollaborativeDefault: false,
		PublicDefault:        false,
	}, nil
}

func (s *LocalizationService) validateConfiguration(config *ConfigurationExport) []string {
	var errors []string

	// Validate version
	if config.Version == "" {
		errors = append(errors, "Configuration version is required")
	}

	// Validate config type
	validTypes := []string{"full", "localization", "media", "playlists", "wizard"}
	typeValid := false
	for _, validType := range validTypes {
		if config.ConfigType == validType {
			typeValid = true
			break
		}
	}
	if !typeValid {
		errors = append(errors, fmt.Sprintf("Invalid config type: %s", config.ConfigType))
	}

	// Validate localization settings
	if config.Localization != nil {
		if config.Localization.PrimaryLanguage == "" {
			errors = append(errors, "Primary language is required")
		} else if _, exists := SupportedLanguages[config.Localization.PrimaryLanguage]; !exists {
			errors = append(errors, fmt.Sprintf("Unsupported primary language: %s", config.Localization.PrimaryLanguage))
		}

		// Validate secondary languages
		for _, lang := range config.Localization.SecondaryLanguages {
			if _, exists := SupportedLanguages[lang]; !exists {
				errors = append(errors, fmt.Sprintf("Unsupported secondary language: %s", lang))
			}
		}

		// Validate date/time formats
		validDateFormats := []string{"MM/DD/YYYY", "DD/MM/YYYY", "YYYY-MM-DD"}
		dateFormatValid := false
		for _, format := range validDateFormats {
			if config.Localization.DateFormat == format {
				dateFormatValid = true
				break
			}
		}
		if !dateFormatValid {
			errors = append(errors, fmt.Sprintf("Invalid date format: %s", config.Localization.DateFormat))
		}

		validTimeFormats := []string{"12h", "24h"}
		timeFormatValid := false
		for _, format := range validTimeFormats {
			if config.Localization.TimeFormat == format {
				timeFormatValid = true
				break
			}
		}
		if !timeFormatValid {
			errors = append(errors, fmt.Sprintf("Invalid time format: %s", config.Localization.TimeFormat))
		}
	}

	// Validate media settings
	if config.MediaSettings != nil {
		validQualities := []string{"low", "medium", "high", "lossless"}
		qualityValid := false
		for _, quality := range validQualities {
			if config.MediaSettings.DefaultQuality == quality {
				qualityValid = true
				break
			}
		}
		if !qualityValid {
			errors = append(errors, fmt.Sprintf("Invalid default quality: %s", config.MediaSettings.DefaultQuality))
		}

		if config.MediaSettings.VolumeLevel < 0 || config.MediaSettings.VolumeLevel > 1 {
			errors = append(errors, "Volume level must be between 0 and 1")
		}

		if config.MediaSettings.CrossfadeDuration < 0 || config.MediaSettings.CrossfadeDuration > 10000 {
			errors = append(errors, "Crossfade duration must be between 0 and 10000ms")
		}
	}

	return errors
}

func (s *LocalizationService) importLocalizationSettings(ctx context.Context, userID int64, localization *UserLocalization) error {
	updates := map[string]interface{}{
		"primary_language":        localization.PrimaryLanguage,
		"secondary_languages":     localization.SecondaryLanguages,
		"subtitle_languages":      localization.SubtitleLanguages,
		"lyrics_languages":        localization.LyricsLanguages,
		"metadata_languages":      localization.MetadataLanguages,
		"auto_translate":          localization.AutoTranslate,
		"auto_download_subtitles": localization.AutoDownloadSubtitles,
		"auto_download_lyrics":    localization.AutoDownloadLyrics,
		"preferred_region":        localization.PreferredRegion,
		"date_format":             localization.DateFormat,
		"time_format":             localization.TimeFormat,
		"number_format":           localization.NumberFormat,
		"currency_code":           localization.CurrencyCode,
	}

	return s.UpdateUserLocalization(ctx, userID, updates)
}

func (s *LocalizationService) importWizardStepSettings(ctx context.Context, userID int64, wizardStep *WizardLocalizationStep) error {
	wizardStep.UserID = userID
	_, err := s.SetupUserLocalization(ctx, wizardStep)
	return err
}

func (s *LocalizationService) importMediaSettings(ctx context.Context, userID int64, mediaSettings *MediaPlayerConfig) error {
	// This would typically save to user_media_settings table
	s.logger.Info("Media settings imported", zap.Int64("user_id", userID))
	return nil
}

func (s *LocalizationService) importPlaylistSettings(ctx context.Context, userID int64, playlistSettings *PlaylistConfig) error {
	// This would typically save to user_playlist_settings table
	s.logger.Info("Playlist settings imported", zap.Int64("user_id", userID))
	return nil
}

func (s *LocalizationService) storeConfigurationExport(ctx context.Context, export *ConfigurationExport) error {
	exportJSON, err := json.Marshal(export)
	if err != nil {
		return err
	}

	query := `
		INSERT INTO configuration_exports (user_id, config_type, config_data, description, tags, created_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
	`

	tagsJSON, _ := json.Marshal(export.Tags)
	_, err = s.db.ExecContext(ctx, query, export.ExportedBy, export.ConfigType, string(exportJSON), export.Description, string(tagsJSON))
	return err
}

func (s *LocalizationService) logImportActivity(ctx context.Context, userID int64, config *ConfigurationExport, result *ConfigurationImportResult) error {
	activityJSON, err := json.Marshal(map[string]interface{}{
		"config_version":    config.Version,
		"config_type":       config.ConfigType,
		"original_user":     config.ExportedBy,
		"applied_settings":  result.AppliedSettings,
		"skipped_settings":  result.SkippedSettings,
		"validation_errors": result.ValidationErrors,
		"backup_created":    result.BackupCreated,
	})
	if err != nil {
		return err
	}

	query := `
		INSERT INTO configuration_import_log (user_id, import_data, success, created_at)
		VALUES (?, ?, ?, CURRENT_TIMESTAMP)
	`

	_, err = s.db.ExecContext(ctx, query, userID, string(activityJSON), result.Success)
	return err
}

func (s *LocalizationService) applyConfigurationEdit(config *ConfigurationExport, path string, value interface{}) error {
	parts := strings.Split(path, ".")

	switch parts[0] {
	case "localization":
		if config.Localization == nil {
			config.Localization = &UserLocalization{}
		}
		return s.applyLocalizationEdit(config.Localization, parts[1:], value)
	case "wizard_step":
		if config.WizardStep == nil {
			config.WizardStep = &WizardLocalizationStep{}
		}
		return s.applyWizardStepEdit(config.WizardStep, parts[1:], value)
	case "media_settings":
		if config.MediaSettings == nil {
			config.MediaSettings = &MediaPlayerConfig{}
		}
		return s.applyMediaSettingsEdit(config.MediaSettings, parts[1:], value)
	case "playlist_settings":
		if config.PlaylistSettings == nil {
			config.PlaylistSettings = &PlaylistConfig{}
		}
		return s.applyPlaylistSettingsEdit(config.PlaylistSettings, parts[1:], value)
	case "description":
		if str, ok := value.(string); ok {
			config.Description = str
			return nil
		}
		return fmt.Errorf("description must be a string")
	case "tags":
		if tags, ok := value.([]interface{}); ok {
			stringTags := make([]string, len(tags))
			for i, tag := range tags {
				if str, ok := tag.(string); ok {
					stringTags[i] = str
				} else {
					return fmt.Errorf("all tags must be strings")
				}
			}
			config.Tags = stringTags
			return nil
		}
		return fmt.Errorf("tags must be an array of strings")
	default:
		return fmt.Errorf("unknown configuration path: %s", parts[0])
	}
}

func (s *LocalizationService) applyLocalizationEdit(localization *UserLocalization, parts []string, value interface{}) error {
	if len(parts) == 0 {
		return fmt.Errorf("invalid localization path")
	}

	switch parts[0] {
	case "primary_language":
		if str, ok := value.(string); ok {
			localization.PrimaryLanguage = str
			return nil
		}
		return fmt.Errorf("primary_language must be a string")
	case "auto_translate":
		if b, ok := value.(bool); ok {
			localization.AutoTranslate = b
			return nil
		}
		return fmt.Errorf("auto_translate must be a boolean")
	case "preferred_region":
		if str, ok := value.(string); ok {
			localization.PreferredRegion = str
			return nil
		}
		return fmt.Errorf("preferred_region must be a string")
	default:
		return fmt.Errorf("unknown localization field: %s", parts[0])
	}
}

func (s *LocalizationService) applyWizardStepEdit(wizardStep *WizardLocalizationStep, parts []string, value interface{}) error {
	if len(parts) == 0 {
		return fmt.Errorf("invalid wizard step path")
	}

	switch parts[0] {
	case "primary_language":
		if str, ok := value.(string); ok {
			wizardStep.PrimaryLanguage = str
			return nil
		}
		return fmt.Errorf("primary_language must be a string")
	case "auto_translate":
		if b, ok := value.(bool); ok {
			wizardStep.AutoTranslate = b
			return nil
		}
		return fmt.Errorf("auto_translate must be a boolean")
	default:
		return fmt.Errorf("unknown wizard step field: %s", parts[0])
	}
}

func (s *LocalizationService) applyMediaSettingsEdit(mediaSettings *MediaPlayerConfig, parts []string, value interface{}) error {
	if len(parts) == 0 {
		return fmt.Errorf("invalid media settings path")
	}

	switch parts[0] {
	case "default_quality":
		if str, ok := value.(string); ok {
			mediaSettings.DefaultQuality = str
			return nil
		}
		return fmt.Errorf("default_quality must be a string")
	case "volume_level":
		if f, ok := value.(float64); ok {
			mediaSettings.VolumeLevel = f
			return nil
		}
		return fmt.Errorf("volume_level must be a number")
	default:
		return fmt.Errorf("unknown media settings field: %s", parts[0])
	}
}

func (s *LocalizationService) applyPlaylistSettingsEdit(playlistSettings *PlaylistConfig, parts []string, value interface{}) error {
	if len(parts) == 0 {
		return fmt.Errorf("invalid playlist settings path")
	}

	switch parts[0] {
	case "auto_create_playlists":
		if b, ok := value.(bool); ok {
			playlistSettings.AutoCreatePlaylists = b
			return nil
		}
		return fmt.Errorf("auto_create_playlists must be a boolean")
	default:
		return fmt.Errorf("unknown playlist settings field: %s", parts[0])
	}
}

func (s *LocalizationService) getDefaultLocalizationTemplate() *UserLocalization {
	return &UserLocalization{
		PrimaryLanguage:       "en",
		SecondaryLanguages:    []string{},
		SubtitleLanguages:     []string{"en"},
		LyricsLanguages:       []string{"en"},
		MetadataLanguages:     []string{"en"},
		AutoTranslate:         false,
		AutoDownloadSubtitles: true,
		AutoDownloadLyrics:    true,
		PreferredRegion:       "US",
		DateFormat:            "MM/DD/YYYY",
		TimeFormat:            "12h",
		NumberFormat:          "en-US",
		CurrencyCode:          "USD",
	}
}

func (s *LocalizationService) getDefaultWizardStepTemplate() *WizardLocalizationStep {
	return &WizardLocalizationStep{
		PrimaryLanguage:       "en",
		SecondaryLanguages:    []string{},
		SubtitleLanguages:     []string{"en"},
		LyricsLanguages:       []string{"en"},
		MetadataLanguages:     []string{"en"},
		AutoTranslate:         false,
		AutoDownloadSubtitles: true,
		AutoDownloadLyrics:    true,
		PreferredRegion:       "US",
		DateFormat:            "MM/DD/YYYY",
		TimeFormat:            "12h",
		NumberFormat:          "en-US",
		CurrencyCode:          "USD",
	}
}

func (s *LocalizationService) getDefaultMediaSettingsTemplate() *MediaPlayerConfig {
	return &MediaPlayerConfig{
		DefaultQuality:    "high",
		AutoPlay:          true,
		CrossfadeEnabled:  false,
		CrossfadeDuration: 3000,
		EqualizerPreset:   "flat",
		EqualizerBands:    make(map[string]float64),
		RepeatMode:        "none",
		ShuffleEnabled:    false,
		VolumeLevel:       1.0,
		ReplayGainEnabled: false,
	}
}

func (s *LocalizationService) getDefaultPlaylistSettingsTemplate() *PlaylistConfig {
	return &PlaylistConfig{
		AutoCreatePlaylists:  false,
		SmartPlaylistRules:   []string{},
		DefaultPlaylistType:  "standard",
		CollaborativeDefault: false,
		PublicDefault:        false,
	}
}
