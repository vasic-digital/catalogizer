package services

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ============================================================================
// LocalizationService — Configuration Management Tests (pure functions)
// ============================================================================

func newTestLocalizationServiceUtil() *LocalizationService {
	return NewLocalizationService(nil, zap.NewNop(), nil, nil)
}

func TestLocalization_ConvertWizardToConfiguration(t *testing.T) {
	svc := newTestLocalizationServiceUtil()
	ctx := context.Background()

	wizardStep := &WizardLocalizationStep{
		UserID:                42,
		PrimaryLanguage:       "fr",
		SecondaryLanguages:    []string{"en", "es"},
		SubtitleLanguages:     []string{"fr", "en"},
		LyricsLanguages:       []string{"fr"},
		MetadataLanguages:     []string{"fr", "en"},
		AutoTranslate:         true,
		AutoDownloadSubtitles: true,
		AutoDownloadLyrics:    false,
		PreferredRegion:       "FR",
		DateFormat:            "DD/MM/YYYY",
		TimeFormat:            "24h",
		NumberFormat:          "fr-FR",
		CurrencyCode:          "EUR",
	}

	config, err := svc.ConvertWizardToConfiguration(ctx, wizardStep)
	require.NoError(t, err)
	require.NotNil(t, config)

	assert.Equal(t, "1.0", config.Version)
	assert.Equal(t, int64(42), config.ExportedBy)
	assert.Equal(t, "wizard", config.ConfigType)
	assert.Equal(t, wizardStep, config.WizardStep)
	require.NotNil(t, config.Localization)
	assert.Equal(t, "fr", config.Localization.PrimaryLanguage)
	assert.Equal(t, []string{"en", "es"}, config.Localization.SecondaryLanguages)
	assert.True(t, config.Localization.AutoTranslate)
	assert.Equal(t, "EUR", config.Localization.CurrencyCode)
	assert.Contains(t, config.Tags, "wizard")
	assert.Contains(t, config.Tags, "generated")
}

func TestLocalization_ConvertLocalizationToWizardStep(t *testing.T) {
	svc := newTestLocalizationServiceUtil()

	localization := &UserLocalization{
		UserID:                10,
		PrimaryLanguage:       "de",
		SecondaryLanguages:    []string{"en"},
		SubtitleLanguages:     []string{"de", "en"},
		LyricsLanguages:       []string{"de"},
		MetadataLanguages:     []string{"de", "en"},
		AutoTranslate:         true,
		AutoDownloadSubtitles: true,
		AutoDownloadLyrics:    true,
		PreferredRegion:       "DE",
		DateFormat:            "YYYY-MM-DD",
		TimeFormat:            "24h",
		NumberFormat:          "de-DE",
		CurrencyCode:          "EUR",
	}

	step := svc.convertLocalizationToWizardStep(localization)
	require.NotNil(t, step)
	assert.Equal(t, int64(10), step.UserID)
	assert.Equal(t, "de", step.PrimaryLanguage)
	assert.Equal(t, []string{"en"}, step.SecondaryLanguages)
	assert.Equal(t, "EUR", step.CurrencyCode)
	assert.True(t, step.AutoTranslate)
}

func TestLocalization_GetDefaultLocalizationTemplate(t *testing.T) {
	svc := newTestLocalizationServiceUtil()

	tmpl := svc.getDefaultLocalizationTemplate()
	require.NotNil(t, tmpl)
	assert.Equal(t, "en", tmpl.PrimaryLanguage)
	assert.Equal(t, []string{}, tmpl.SecondaryLanguages)
	assert.Equal(t, "US", tmpl.PreferredRegion)
	assert.Equal(t, "MM/DD/YYYY", tmpl.DateFormat)
	assert.Equal(t, "12h", tmpl.TimeFormat)
	assert.Equal(t, "USD", tmpl.CurrencyCode)
	assert.False(t, tmpl.AutoTranslate)
	assert.True(t, tmpl.AutoDownloadSubtitles)
	assert.True(t, tmpl.AutoDownloadLyrics)
}

func TestLocalization_GetDefaultWizardStepTemplate(t *testing.T) {
	svc := newTestLocalizationServiceUtil()

	tmpl := svc.getDefaultWizardStepTemplate()
	require.NotNil(t, tmpl)
	assert.Equal(t, "en", tmpl.PrimaryLanguage)
	assert.Equal(t, "US", tmpl.PreferredRegion)
	assert.Equal(t, "MM/DD/YYYY", tmpl.DateFormat)
	assert.Equal(t, "12h", tmpl.TimeFormat)
	assert.Equal(t, "USD", tmpl.CurrencyCode)
}

func TestLocalization_GetDefaultMediaSettingsTemplate(t *testing.T) {
	svc := newTestLocalizationServiceUtil()

	tmpl := svc.getDefaultMediaSettingsTemplate()
	require.NotNil(t, tmpl)
	assert.Equal(t, "high", tmpl.DefaultQuality)
	assert.True(t, tmpl.AutoPlay)
	assert.False(t, tmpl.CrossfadeEnabled)
	assert.Equal(t, 3000, tmpl.CrossfadeDuration)
	assert.Equal(t, "flat", tmpl.EqualizerPreset)
	assert.Equal(t, "none", tmpl.RepeatMode)
	assert.False(t, tmpl.ShuffleEnabled)
	assert.Equal(t, 1.0, tmpl.VolumeLevel)
	assert.False(t, tmpl.ReplayGainEnabled)
}

func TestLocalization_GetDefaultPlaylistSettingsTemplate(t *testing.T) {
	svc := newTestLocalizationServiceUtil()

	tmpl := svc.getDefaultPlaylistSettingsTemplate()
	require.NotNil(t, tmpl)
	assert.False(t, tmpl.AutoCreatePlaylists)
	assert.Equal(t, "standard", tmpl.DefaultPlaylistType)
	assert.False(t, tmpl.CollaborativeDefault)
	assert.False(t, tmpl.PublicDefault)
}

func TestLocalization_GetMediaPlayerConfig(t *testing.T) {
	svc := newTestLocalizationServiceUtil()
	ctx := context.Background()

	config, err := svc.getMediaPlayerConfig(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, config)
	assert.Equal(t, "high", config.DefaultQuality)
	assert.True(t, config.AutoPlay)
	assert.True(t, config.CrossfadeEnabled)
	assert.Equal(t, 3000, config.CrossfadeDuration)
	assert.Equal(t, 1.0, config.VolumeLevel)
}

func TestLocalization_GetPlaylistConfig(t *testing.T) {
	svc := newTestLocalizationServiceUtil()
	ctx := context.Background()

	config, err := svc.getPlaylistConfig(ctx, 1)
	require.NoError(t, err)
	require.NotNil(t, config)
	assert.True(t, config.AutoCreatePlaylists)
	assert.Contains(t, config.SmartPlaylistRules, "recently_played")
	assert.Contains(t, config.SmartPlaylistRules, "top_rated")
	assert.Equal(t, "standard", config.DefaultPlaylistType)
}

func TestLocalization_ApplyConfigurationEdit_Description(t *testing.T) {
	svc := newTestLocalizationServiceUtil()

	config := &ConfigurationExport{
		Version:     "1.0",
		ConfigType:  "full",
		Description: "old description",
	}

	err := svc.applyConfigurationEdit(config, "description", "new description")
	require.NoError(t, err)
	assert.Equal(t, "new description", config.Description)

	// Wrong type
	err = svc.applyConfigurationEdit(config, "description", 42)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be a string")
}

func TestLocalization_ApplyConfigurationEdit_Tags(t *testing.T) {
	svc := newTestLocalizationServiceUtil()

	config := &ConfigurationExport{
		Version:    "1.0",
		ConfigType: "full",
	}

	tags := []interface{}{"tag1", "tag2", "tag3"}
	err := svc.applyConfigurationEdit(config, "tags", tags)
	require.NoError(t, err)
	assert.Equal(t, []string{"tag1", "tag2", "tag3"}, config.Tags)

	// Wrong type: not an array
	err = svc.applyConfigurationEdit(config, "tags", "not-an-array")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be an array")

	// Wrong type: array with non-strings
	err = svc.applyConfigurationEdit(config, "tags", []interface{}{"ok", 42})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be strings")
}

func TestLocalization_ApplyConfigurationEdit_UnknownPath(t *testing.T) {
	svc := newTestLocalizationServiceUtil()
	config := &ConfigurationExport{}

	err := svc.applyConfigurationEdit(config, "nonexistent", "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown configuration path")
}

func TestLocalization_ApplyLocalizationEdit(t *testing.T) {
	svc := newTestLocalizationServiceUtil()

	config := &ConfigurationExport{
		Version:    "1.0",
		ConfigType: "full",
	}

	// Apply localization edit creates Localization if nil
	err := svc.applyConfigurationEdit(config, "localization.primary_language", "fr")
	require.NoError(t, err)
	require.NotNil(t, config.Localization)
	assert.Equal(t, "fr", config.Localization.PrimaryLanguage)

	// Auto translate
	err = svc.applyConfigurationEdit(config, "localization.auto_translate", true)
	require.NoError(t, err)
	assert.True(t, config.Localization.AutoTranslate)

	// Auto translate wrong type
	err = svc.applyConfigurationEdit(config, "localization.auto_translate", "yes")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "must be a boolean")

	// Preferred region
	err = svc.applyConfigurationEdit(config, "localization.preferred_region", "DE")
	require.NoError(t, err)
	assert.Equal(t, "DE", config.Localization.PreferredRegion)

	// Preferred region wrong type
	err = svc.applyConfigurationEdit(config, "localization.preferred_region", 42)
	assert.Error(t, err)

	// Primary language wrong type
	err = svc.applyConfigurationEdit(config, "localization.primary_language", 42)
	assert.Error(t, err)

	// Unknown field
	err = svc.applyConfigurationEdit(config, "localization.unknown_field", "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown localization field")

	// Empty parts
	loc := &UserLocalization{}
	err = svc.applyLocalizationEdit(loc, []string{}, "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid localization path")
}

func TestLocalization_ApplyWizardStepEdit(t *testing.T) {
	svc := newTestLocalizationServiceUtil()

	config := &ConfigurationExport{
		Version:    "1.0",
		ConfigType: "full",
	}

	// Apply wizard step edit creates WizardStep if nil
	err := svc.applyConfigurationEdit(config, "wizard_step.primary_language", "ja")
	require.NoError(t, err)
	require.NotNil(t, config.WizardStep)
	assert.Equal(t, "ja", config.WizardStep.PrimaryLanguage)

	// Wrong type
	err = svc.applyConfigurationEdit(config, "wizard_step.primary_language", 42)
	assert.Error(t, err)

	// Auto translate
	err = svc.applyConfigurationEdit(config, "wizard_step.auto_translate", true)
	require.NoError(t, err)
	assert.True(t, config.WizardStep.AutoTranslate)

	// Auto translate wrong type
	err = svc.applyConfigurationEdit(config, "wizard_step.auto_translate", "yes")
	assert.Error(t, err)

	// Unknown field
	err = svc.applyConfigurationEdit(config, "wizard_step.unknown", "value")
	assert.Error(t, err)

	// Empty parts
	ws := &WizardLocalizationStep{}
	err = svc.applyWizardStepEdit(ws, []string{}, "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid wizard step path")
}

func TestLocalization_ApplyMediaSettingsEdit(t *testing.T) {
	svc := newTestLocalizationServiceUtil()

	config := &ConfigurationExport{
		Version:    "1.0",
		ConfigType: "full",
	}

	// Apply media settings edit creates MediaSettings if nil
	err := svc.applyConfigurationEdit(config, "media_settings.default_quality", "lossless")
	require.NoError(t, err)
	require.NotNil(t, config.MediaSettings)
	assert.Equal(t, "lossless", config.MediaSettings.DefaultQuality)

	// Wrong type
	err = svc.applyConfigurationEdit(config, "media_settings.default_quality", 42)
	assert.Error(t, err)

	// Volume level
	err = svc.applyConfigurationEdit(config, "media_settings.volume_level", 0.75)
	require.NoError(t, err)
	assert.Equal(t, 0.75, config.MediaSettings.VolumeLevel)

	// Volume level wrong type
	err = svc.applyConfigurationEdit(config, "media_settings.volume_level", "loud")
	assert.Error(t, err)

	// Unknown field
	err = svc.applyConfigurationEdit(config, "media_settings.unknown", "value")
	assert.Error(t, err)

	// Empty parts
	ms := &MediaPlayerConfig{}
	err = svc.applyMediaSettingsEdit(ms, []string{}, "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid media settings path")
}

func TestLocalization_ApplyPlaylistSettingsEdit(t *testing.T) {
	svc := newTestLocalizationServiceUtil()

	config := &ConfigurationExport{
		Version:    "1.0",
		ConfigType: "full",
	}

	// Apply playlist settings edit creates PlaylistSettings if nil
	err := svc.applyConfigurationEdit(config, "playlist_settings.auto_create_playlists", true)
	require.NoError(t, err)
	require.NotNil(t, config.PlaylistSettings)
	assert.True(t, config.PlaylistSettings.AutoCreatePlaylists)

	// Wrong type
	err = svc.applyConfigurationEdit(config, "playlist_settings.auto_create_playlists", "yes")
	assert.Error(t, err)

	// Unknown field
	err = svc.applyConfigurationEdit(config, "playlist_settings.unknown", "value")
	assert.Error(t, err)

	// Empty parts
	ps := &PlaylistConfig{}
	err = svc.applyPlaylistSettingsEdit(ps, []string{}, "value")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid playlist settings path")
}

func TestLocalization_EditConfiguration(t *testing.T) {
	svc := newTestLocalizationServiceUtil()
	ctx := context.Background()

	configJSON := `{
		"version": "1.0",
		"config_type": "full",
		"description": "test config",
		"tags": ["original"]
	}`

	edits := map[string]interface{}{
		"description": "updated config",
	}

	result, err := svc.EditConfiguration(ctx, 1, configJSON, edits)
	require.NoError(t, err)
	assert.Contains(t, result, "updated config")

	// Invalid JSON
	_, err = svc.EditConfiguration(ctx, 1, "invalid json", edits)
	assert.Error(t, err)

	// Invalid edit path
	badEdits := map[string]interface{}{
		"nonexistent": "value",
	}
	_, err = svc.EditConfiguration(ctx, 1, configJSON, badEdits)
	assert.Error(t, err)
}

func TestLocalization_ValidateConfiguration_Extended(t *testing.T) {
	svc := newTestLocalizationServiceUtil()

	tests := []struct {
		name       string
		config     ConfigurationExport
		wantErrors int
	}{
		{
			"valid full config",
			ConfigurationExport{
				Version:    "1.0",
				ConfigType: "full",
				Localization: &UserLocalization{
					PrimaryLanguage: "en",
					DateFormat:      "MM/DD/YYYY",
					TimeFormat:      "12h",
				},
				MediaSettings: &MediaPlayerConfig{
					DefaultQuality:    "high",
					VolumeLevel:       0.8,
					CrossfadeDuration: 3000,
				},
			},
			0,
		},
		{
			"empty version",
			ConfigurationExport{
				ConfigType: "full",
			},
			1,
		},
		{
			"invalid config type",
			ConfigurationExport{
				Version:    "1.0",
				ConfigType: "invalid",
			},
			1,
		},
		{
			"unsupported primary language",
			ConfigurationExport{
				Version:    "1.0",
				ConfigType: "full",
				Localization: &UserLocalization{
					PrimaryLanguage: "xx",
					DateFormat:      "MM/DD/YYYY",
					TimeFormat:      "12h",
				},
			},
			1,
		},
		{
			"empty primary language",
			ConfigurationExport{
				Version:    "1.0",
				ConfigType: "full",
				Localization: &UserLocalization{
					PrimaryLanguage: "",
					DateFormat:      "MM/DD/YYYY",
					TimeFormat:      "12h",
				},
			},
			1,
		},
		{
			"unsupported secondary language",
			ConfigurationExport{
				Version:    "1.0",
				ConfigType: "full",
				Localization: &UserLocalization{
					PrimaryLanguage:    "en",
					SecondaryLanguages: []string{"xx"},
					DateFormat:         "MM/DD/YYYY",
					TimeFormat:         "12h",
				},
			},
			1,
		},
		{
			"invalid date format",
			ConfigurationExport{
				Version:    "1.0",
				ConfigType: "full",
				Localization: &UserLocalization{
					PrimaryLanguage: "en",
					DateFormat:      "INVALID",
					TimeFormat:      "12h",
				},
			},
			1,
		},
		{
			"invalid time format",
			ConfigurationExport{
				Version:    "1.0",
				ConfigType: "full",
				Localization: &UserLocalization{
					PrimaryLanguage: "en",
					DateFormat:      "MM/DD/YYYY",
					TimeFormat:      "INVALID",
				},
			},
			1,
		},
		{
			"invalid media quality",
			ConfigurationExport{
				Version:    "1.0",
				ConfigType: "full",
				MediaSettings: &MediaPlayerConfig{
					DefaultQuality: "ultra-hd",
					VolumeLevel:    0.5,
				},
			},
			1,
		},
		{
			"volume out of range",
			ConfigurationExport{
				Version:    "1.0",
				ConfigType: "full",
				MediaSettings: &MediaPlayerConfig{
					DefaultQuality: "high",
					VolumeLevel:    1.5,
				},
			},
			1,
		},
		{
			"negative volume",
			ConfigurationExport{
				Version:    "1.0",
				ConfigType: "full",
				MediaSettings: &MediaPlayerConfig{
					DefaultQuality: "high",
					VolumeLevel:    -0.1,
				},
			},
			1,
		},
		{
			"crossfade out of range",
			ConfigurationExport{
				Version:    "1.0",
				ConfigType: "full",
				MediaSettings: &MediaPlayerConfig{
					DefaultQuality:    "high",
					VolumeLevel:       0.5,
					CrossfadeDuration: 15000,
				},
			},
			1,
		},
		{
			"valid wizard config type",
			ConfigurationExport{
				Version:    "1.0",
				ConfigType: "wizard",
			},
			0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errors := svc.validateConfiguration(&tc.config)
			assert.Equal(t, tc.wantErrors, len(errors), "errors: %v", errors)
		})
	}
}

func TestLocalization_ImportMediaSettings(t *testing.T) {
	svc := newTestLocalizationServiceUtil()
	ctx := context.Background()

	err := svc.importMediaSettings(ctx, 1, &MediaPlayerConfig{
		DefaultQuality: "high",
	})
	assert.NoError(t, err)
}

func TestLocalization_ImportPlaylistSettings(t *testing.T) {
	svc := newTestLocalizationServiceUtil()
	ctx := context.Background()

	err := svc.importPlaylistSettings(ctx, 1, &PlaylistConfig{
		AutoCreatePlaylists: true,
	})
	assert.NoError(t, err)
}

// ============================================================================
// LyricsService — Additional Pure Function Tests
// ============================================================================

func newTestLyricsServiceUtil() *LyricsService {
	return NewLyricsService(nil, zap.NewNop())
}

func TestLyrics_TimePtr(t *testing.T) {
	now := time.Now()
	ptr := timePtr(now)
	require.NotNil(t, ptr)
	assert.Equal(t, now, *ptr)
}

func TestLyrics_FloatPtr(t *testing.T) {
	f := 3.14
	ptr := floatPtr(f)
	require.NotNil(t, ptr)
	assert.Equal(t, f, *ptr)
}

func TestLyrics_GetCachedLyrics(t *testing.T) {
	svc := newTestLyricsServiceUtil()
	ctx := context.Background()

	// Always returns nil (cache miss stub)
	result := svc.getCachedLyrics(ctx, "Title", "Artist")
	assert.Nil(t, result)
}

func TestLyrics_GetCachedTranslation(t *testing.T) {
	svc := newTestLyricsServiceUtil()
	ctx := context.Background()

	// Always returns nil (cache miss stub)
	result := svc.getCachedTranslation(ctx, "lyrics-123", "fr")
	assert.Nil(t, result)
}

func TestLyrics_GetLyricsDownloadInfo_NotFabricated(t *testing.T) {
	svc := newTestLyricsServiceUtil()
	result, err := svc.getLyricsDownloadInfo(context.Background(), "test-result-id")
	assert.ErrorIs(t, err, ErrLyricsProviderNotImplemented)
	assert.Nil(t, result)
}

// CAT-001: no lyrics provider has a client, so search must fail loudly and
// auto-sync must not invent a 3-minute timeline.
func TestLyrics_SearchAndSyncDoNotFabricate(t *testing.T) {
	svc := newTestLyricsServiceUtil()
	ctx := context.Background()

	res, err := svc.SearchLyrics(ctx, &LyricsSearchRequest{Title: "Bohemian Rhapsody", Artist: "Queen"})
	assert.ErrorIs(t, err, ErrLyricsProviderNotImplemented)
	assert.Empty(t, res)

	lines, err := svc.autoSynchronizeLyrics(ctx, &LyricsData{Content: "a\nb\nc"}, "/nonexistent.mp3")
	assert.ErrorIs(t, err, ErrLyricsProviderNotImplemented)
	assert.Empty(t, lines)
}

func TestLyrics_PreserveLyricsTiming_EqualLines(t *testing.T) {
	svc := newTestLyricsServiceUtil()

	originalSync := []LyricsLine{
		{StartTime: 0.0, Text: "Hello"},
		{StartTime: 5.0, Text: "World"},
		{StartTime: 10.0, Text: "End"},
	}

	translatedText := "Bonjour\nMonde\nFin"

	result := svc.preserveLyricsTiming(originalSync, translatedText)
	require.Len(t, result, 3)
	assert.Equal(t, 0.0, result[0].StartTime)
	assert.Equal(t, "Bonjour", result[0].Text)
	assert.Equal(t, 5.0, result[1].StartTime)
	assert.Equal(t, "Monde", result[1].Text)
	assert.Equal(t, 10.0, result[2].StartTime)
	assert.Equal(t, "Fin", result[2].Text)
}

func TestLyrics_PreserveLyricsTiming_MismatchedLines(t *testing.T) {
	svc := newTestLyricsServiceUtil()

	originalSync := []LyricsLine{
		{StartTime: 0.0, Text: "Hello"},
		{StartTime: 5.0, Text: "World"},
		{StartTime: 10.0, Text: "End"},
	}

	// Only 2 translated lines vs 3 original
	translatedText := "Bonjour\nMonde"

	result := svc.preserveLyricsTiming(originalSync, translatedText)
	require.Len(t, result, 2) // min(3, 2) = 2
	assert.Equal(t, 0.0, result[0].StartTime)
	assert.Equal(t, "Bonjour", result[0].Text)
	assert.Equal(t, 5.0, result[1].StartTime)
	assert.Equal(t, "Monde", result[1].Text)
}

func TestLyrics_PreserveLyricsTiming_EmptyTranslation(t *testing.T) {
	svc := newTestLyricsServiceUtil()

	originalSync := []LyricsLine{
		{StartTime: 0.0, Text: "Hello"},
	}

	result := svc.preserveLyricsTiming(originalSync, "")
	assert.Empty(t, result)
}

func TestLyrics_DetectConcertSetlist(t *testing.T) {
	svc := newTestLyricsServiceUtil()
	ctx := context.Background()

	request := &ConcertLyricsRequest{
		MediaItemID: 1,
		Artist:      "Pink Floyd",
	}

	setlist, err := svc.detectConcertSetlist(ctx, request)
	require.NoError(t, err)
	assert.Empty(t, setlist) // Mock returns empty
}

// ============================================================================
// RecommendationService — Pure Utility Functions
// ============================================================================

func TestRecommendation_Abs(t *testing.T) {
	assert.Equal(t, 5.0, abs(5.0))
	assert.Equal(t, 5.0, abs(-5.0))
	assert.Equal(t, 0.0, abs(0.0))
	assert.Equal(t, 0.001, abs(-0.001))
}

func TestRecommendation_ParseYear(t *testing.T) {
	// parseYear always returns 2023 (mock implementation)
	assert.Equal(t, 2023, parseYear("2024"))
	assert.Equal(t, 2023, parseYear("1999"))
	assert.Equal(t, 0, parseYear(""))
}

// ============================================================================
// DuplicateDetectionService — Additional Uncovered Methods
// ============================================================================

func TestDuplicateDetection_CalculateSimilarity_EmptyTitles(t *testing.T) {
	svc := newTestDuplicateDetectionService()
	ctx := context.Background()

	item1 := &DuplicateItem{Title: ""}
	item2 := &DuplicateItem{Title: "Something"}
	analysis := svc.calculateSimilarity(ctx, item1, item2, MediaTypeMovie)
	assert.Equal(t, 0.0, analysis.OverallScore)
}

func TestDuplicateDetection_CalculateSimilarity_DifferentTypes(t *testing.T) {
	svc := newTestDuplicateDetectionService()
	ctx := context.Background()

	item1 := &DuplicateItem{Title: "Test Song", Artist: "Beatles", Year: 1969}
	item2 := &DuplicateItem{Title: "Test Song", Artist: "Beatles", Year: 1969}

	// Music type
	analysis := svc.calculateSimilarity(ctx, item1, item2, MediaTypeMusic)
	assert.Greater(t, analysis.OverallScore, 0.5)
	assert.Greater(t, analysis.TitleSimilarity, 0.8)

	// Book type
	book1 := &DuplicateItem{Title: "War and Peace", Author: "Tolstoy", Year: 1869}
	book2 := &DuplicateItem{Title: "War and Peace", Author: "Tolstoy", Year: 1869}
	analysis = svc.calculateSimilarity(ctx, book1, book2, MediaTypeBook)
	assert.Greater(t, analysis.OverallScore, 0.3)

	// Software type
	sw1 := &DuplicateItem{Title: "Visual Studio Code", Year: 2024}
	sw2 := &DuplicateItem{Title: "Visual Studio Code", Year: 2024}
	analysis = svc.calculateSimilarity(ctx, sw1, sw2, MediaTypeSoftware)
	assert.Greater(t, analysis.OverallScore, 0.3)

	// Game type
	g1 := &DuplicateItem{Title: "The Witcher 3", Year: 2015}
	g2 := &DuplicateItem{Title: "The Witcher 3", Year: 2015}
	analysis = svc.calculateSimilarity(ctx, g1, g2, MediaTypeGame)
	assert.Greater(t, analysis.OverallScore, 0.3)

	// Generic/unknown type
	gen1 := &DuplicateItem{Title: "Some Item", Year: 2024}
	gen2 := &DuplicateItem{Title: "Some Item", Year: 2024}
	analysis = svc.calculateSimilarity(ctx, gen1, gen2, "unknown_type")
	assert.Greater(t, analysis.OverallScore, 0.3)
}

func TestDuplicateDetection_GetSimilarityWeights_AllTypes(t *testing.T) {
	svc := newTestDuplicateDetectionService()

	types := []MediaType{
		MediaTypeMovie, MediaTypeMusic, MediaTypeBook,
		MediaTypeSoftware, MediaTypeGame, "unknown",
	}

	for _, mt := range types {
		weights := svc.getSimilarityWeights(mt)
		assert.Contains(t, weights, "title", "type %s should have title weight", mt)
		assert.Contains(t, weights, "metadata", "type %s should have metadata weight", mt)
		assert.Contains(t, weights, "file", "type %s should have file weight", mt)
	}
}

func TestDuplicateDetection_NormalizeText_Extended(t *testing.T) {
	svc := newTestDuplicateDetectionService()

	// Test various edge cases
	assert.Equal(t, "", svc.normalizeText("the a an of"))
	assert.NotEmpty(t, svc.normalizeText("Hello-World"))
	// Numbers preserved
	result := svc.normalizeText("2001: A Space Odyssey")
	assert.Contains(t, result, "2001")
	assert.Contains(t, result, "space")
	assert.Contains(t, result, "odyssey")
}

// ============================================================================
// Localization — FormatDateTimeForUser edge cases (partial, no DB)
// FormatDateTimeForUser needs GetUserLocalization which needs DB,
// so we test the error path (nil DB returns default format)
// ============================================================================

// FormatDateTimeForUser requires a non-nil DB (panics with nil DB),
// so it is tested in services_integration_test.go with a real DB.
// We skip it here.

// ============================================================================
// MusicPlayerService — Additional Queue/Repeat Tests
// ============================================================================

func TestMusicPlayer_GetNextTrackIndex_RepeatTrack(t *testing.T) {
	svc := newTestMusicPlayerServiceUtil()

	session := &MusicPlaybackSession{
		Queue:      []MusicTrack{{ID: 1}, {ID: 2}, {ID: 3}},
		QueueIndex: 1,
		RepeatMode: RepeatModeTrack,
	}
	assert.Equal(t, 1, svc.getNextTrackIndex(session))
}

func TestMusicPlayer_GetNextTrackIndex_NoRepeat(t *testing.T) {
	svc := newTestMusicPlayerServiceUtil()

	session := &MusicPlaybackSession{
		Queue:      []MusicTrack{{ID: 1}, {ID: 2}, {ID: 3}},
		QueueIndex: 1,
		RepeatMode: RepeatModeOff,
	}
	// Normal next
	assert.Equal(t, 2, svc.getNextTrackIndex(session))

	// At end: returns -1 for no repeat
	session.QueueIndex = 2
	assert.Equal(t, -1, svc.getNextTrackIndex(session))
}

func TestMusicPlayer_GetPreviousTrackIndex_NoRepeat(t *testing.T) {
	svc := newTestMusicPlayerServiceUtil()

	session := &MusicPlaybackSession{
		Queue:      []MusicTrack{{ID: 1}, {ID: 2}, {ID: 3}},
		QueueIndex: 1,
		RepeatMode: RepeatModeOff,
	}
	assert.Equal(t, 0, svc.getPreviousTrackIndex(session))

	// At beginning: returns -1 for no repeat
	session.QueueIndex = 0
	assert.Equal(t, -1, svc.getPreviousTrackIndex(session))
}

func TestMusicPlayer_GetPreviousTrackIndex_RepeatTrack(t *testing.T) {
	svc := newTestMusicPlayerServiceUtil()

	// RepeatModeTrack doesn't affect getPreviousTrackIndex - it still goes to prev
	session := &MusicPlaybackSession{
		Queue:      []MusicTrack{{ID: 1}, {ID: 2}, {ID: 3}},
		QueueIndex: 1,
		RepeatMode: RepeatModeTrack,
	}
	assert.Equal(t, 0, svc.getPreviousTrackIndex(session))

	// At index 0 with RepeatModeTrack: returns -1 (no wrap)
	session.QueueIndex = 0
	assert.Equal(t, -1, svc.getPreviousTrackIndex(session))
}

// ============================================================================
// BookRecognitionProvider — Additional uncovered methods
// ============================================================================

func TestBookRecognition_ExtractMetadataFromContent_Empty(t *testing.T) {
	provider := newTestBookRecognitionProvider()

	metadata := provider.extractMetadataFromContent("")
	assert.NotNil(t, metadata)
	assert.Empty(t, metadata.ChapterTitles)
}

func TestBookRecognition_DetectLanguage_Short(t *testing.T) {
	provider := newTestBookRecognitionProvider()

	// Very short text defaults
	result := provider.detectLanguage("hi")
	assert.NotEmpty(t, result)
}

func TestBookRecognition_ExtractKeywords_Empty(t *testing.T) {
	provider := newTestBookRecognitionProvider()

	keywords := provider.extractKeywords("")
	assert.Empty(t, keywords)
}

func TestBookRecognition_ExtractTopics_Empty(t *testing.T) {
	provider := newTestBookRecognitionProvider()

	topics := provider.extractTopics("")
	assert.Empty(t, topics)
}

func TestBookRecognition_CalculateGoogleBooksConfidence_Edge(t *testing.T) {
	provider := newTestBookRecognitionProvider()

	// 0 rating, 0 count
	assert.Equal(t, 0.5, provider.calculateGoogleBooksConfidence(0, 0))

	// Very high rating
	assert.Equal(t, 0.8, provider.calculateGoogleBooksConfidence(5.0, 500))
}

func TestBookRecognition_MapCrossrefType_Empty(t *testing.T) {
	provider := newTestBookRecognitionProvider()

	assert.Equal(t, MediaTypeBook, provider.mapCrossrefType(""))
}

func TestBookRecognition_GetFileExtension_Edge(t *testing.T) {
	provider := newTestBookRecognitionProvider()

	assert.Equal(t, "", provider.getFileExtension(""))
	assert.Equal(t, "txt", provider.getFileExtension(".txt"))
}

// ============================================================================
// SubtitleService — Additional uncovered parse paths
// ============================================================================

func TestSubtitleParseASS_EmptyEvents(t *testing.T) {
	svc := newTestSubtitleServiceUtil()

	// No [Events] section
	content := "[Script Info]\nTitle: Test\n"
	_, err := svc.parseASS(content)
	assert.Error(t, err)
}

func TestSubtitleParseASS_NoDialogue(t *testing.T) {
	svc := newTestSubtitleServiceUtil()

	content := "[Script Info]\nTitle: Test\n\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n"
	_, err := svc.parseASS(content)
	assert.Error(t, err)
}

func TestSubtitleParseSRT_InvalidTimestamp(t *testing.T) {
	svc := newTestSubtitleServiceUtil()

	content := "1\nNOT_A_TIMESTAMP --> ALSO_NOT\nHello\n\n"
	// parseSRT is lenient - skips invalid blocks
	lines, err := svc.parseSRT(content)
	require.NoError(t, err)
	assert.Empty(t, lines)
}

func TestSubtitleParseVTT_ValidMultipleCues(t *testing.T) {
	svc := newTestSubtitleServiceUtil()

	content := "WEBVTT\n\n00:00:01.000 --> 00:00:03.000\nFirst\n\n00:00:04.000 --> 00:00:06.000\nSecond\n\n"
	lines, err := svc.parseVTT(content)
	require.NoError(t, err)
	require.Len(t, lines, 2)
	assert.Equal(t, "First", lines[0].Text)
	assert.Equal(t, "Second", lines[1].Text)
}

// ============================================================================
// Localization — GetWizardDefaults for non-English
// ============================================================================

func TestLocalization_GetWizardDefaults_French(t *testing.T) {
	svc := newTestLocalizationServiceUtil()
	ctx := context.Background()

	defaults := svc.GetWizardDefaults(ctx, "fr")
	require.NotNil(t, defaults)
	assert.Equal(t, "fr", defaults.PrimaryLanguage)
	assert.True(t, defaults.AutoTranslate, "non-English should auto-translate")
	assert.Contains(t, defaults.SubtitleLanguages, "fr")
	assert.Contains(t, defaults.SubtitleLanguages, "en")
	assert.Equal(t, "EUR", defaults.CurrencyCode)
}

func TestLocalization_GetWizardDefaults_Unknown(t *testing.T) {
	svc := newTestLocalizationServiceUtil()
	ctx := context.Background()

	defaults := svc.GetWizardDefaults(ctx, "xx")
	require.NotNil(t, defaults)
	assert.Equal(t, "en", defaults.PrimaryLanguage, "unknown language falls back to en")
	assert.False(t, defaults.AutoTranslate)
}

func TestLocalization_GetWizardDefaults_Japanese(t *testing.T) {
	svc := newTestLocalizationServiceUtil()
	ctx := context.Background()

	defaults := svc.GetWizardDefaults(ctx, "ja")
	require.NotNil(t, defaults)
	assert.Equal(t, "ja", defaults.PrimaryLanguage)
	assert.True(t, defaults.AutoTranslate)
	assert.Equal(t, "JPY", defaults.CurrencyCode)
}

// ============================================================================
// DB-backed tests for LocalizationService
// ============================================================================

func TestLocalization_FormatDateTimeForUser_WithDB(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	cacheService := NewCacheService(db, zap.NewNop())
	defer cacheService.Close()
	svc := NewLocalizationService(db, zap.NewNop(), nil, cacheService)
	ctx := context.Background()

	// Setup a localization with specific date/time formats
	step := &WizardLocalizationStep{
		UserID:                1,
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

	_, err := svc.SetupUserLocalization(ctx, step)
	require.NoError(t, err)

	ts := time.Date(2024, 6, 15, 14, 30, 0, 0, time.UTC)
	result, err := svc.FormatDateTimeForUser(ctx, 1, ts)
	require.NoError(t, err)
	assert.Contains(t, result, "06") // month
	assert.Contains(t, result, "15") // day

	// Test with DD/MM/YYYY 24h format
	err = svc.UpdateUserLocalization(ctx, 1, map[string]interface{}{
		"date_format": "DD/MM/YYYY",
		"time_format": "24h",
	})
	require.NoError(t, err)

	result, err = svc.FormatDateTimeForUser(ctx, 1, ts)
	require.NoError(t, err)
	assert.Contains(t, result, "15/06/2024")
	assert.Contains(t, result, "14:30")

	// Test YYYY-MM-DD format
	err = svc.UpdateUserLocalization(ctx, 1, map[string]interface{}{
		"date_format": "YYYY-MM-DD",
	})
	require.NoError(t, err)

	result, err = svc.FormatDateTimeForUser(ctx, 1, ts)
	require.NoError(t, err)
	assert.Contains(t, result, "2024-06-15")
}

func TestLocalization_GetPreferredLanguagesForContent_WithDB(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	cacheService := NewCacheService(db, zap.NewNop())
	defer cacheService.Close()
	svc := NewLocalizationService(db, zap.NewNop(), nil, cacheService)
	ctx := context.Background()

	step := &WizardLocalizationStep{
		UserID:             1,
		PrimaryLanguage:    "fr",
		SecondaryLanguages: []string{"en", "es"},
		SubtitleLanguages:  []string{"fr", "en"},
		LyricsLanguages:    []string{"fr"},
		MetadataLanguages:  []string{"fr", "en", "de"},
		PreferredRegion:    "FR",
		DateFormat:         "DD/MM/YYYY",
		TimeFormat:         "24h",
		NumberFormat:       "fr-FR",
		CurrencyCode:       "EUR",
	}

	_, err := svc.SetupUserLocalization(ctx, step)
	require.NoError(t, err)

	// Subtitles
	langs, err := svc.GetPreferredLanguagesForContent(ctx, 1, ContentTypeSubtitles)
	require.NoError(t, err)
	assert.Equal(t, []string{"fr", "en"}, langs)

	// Lyrics
	langs, err = svc.GetPreferredLanguagesForContent(ctx, 1, ContentTypeLyrics)
	require.NoError(t, err)
	assert.Equal(t, []string{"fr"}, langs)

	// Metadata
	langs, err = svc.GetPreferredLanguagesForContent(ctx, 1, ContentTypeMetadata)
	require.NoError(t, err)
	assert.Equal(t, []string{"fr", "en", "de"}, langs)

	// UI (not specifically set, falls back to primary + secondary)
	langs, err = svc.GetPreferredLanguagesForContent(ctx, 1, ContentTypeUI)
	require.NoError(t, err)
	assert.Contains(t, langs, "fr")
}

func TestLocalization_ShouldAutoDownload_WithDB(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	cacheService := NewCacheService(db, zap.NewNop())
	defer cacheService.Close()
	svc := NewLocalizationService(db, zap.NewNop(), nil, cacheService)
	ctx := context.Background()

	step := &WizardLocalizationStep{
		UserID:                1,
		PrimaryLanguage:       "en",
		SubtitleLanguages:     []string{"en"},
		LyricsLanguages:       []string{"en"},
		MetadataLanguages:     []string{"en"},
		AutoDownloadSubtitles: true,
		AutoDownloadLyrics:    false,
		PreferredRegion:       "US",
		DateFormat:            "MM/DD/YYYY",
		TimeFormat:            "12h",
		NumberFormat:          "en-US",
		CurrencyCode:          "USD",
	}

	_, err := svc.SetupUserLocalization(ctx, step)
	require.NoError(t, err)

	autoSubs, err := svc.ShouldAutoDownload(ctx, 1, ContentTypeSubtitles)
	require.NoError(t, err)
	assert.True(t, autoSubs)

	autoLyrics, err := svc.ShouldAutoDownload(ctx, 1, ContentTypeLyrics)
	require.NoError(t, err)
	assert.False(t, autoLyrics)

	// Unknown content type
	autoOther, err := svc.ShouldAutoDownload(ctx, 1, "unknown")
	require.NoError(t, err)
	assert.False(t, autoOther)
}

func TestLocalization_GetLanguageProfile(t *testing.T) {
	svc := newTestLocalizationServiceUtil()
	ctx := context.Background()

	profile, err := svc.GetLanguageProfile(ctx, "en")
	require.NoError(t, err)
	assert.Equal(t, "English", profile.Name)
	assert.Equal(t, "ltr", profile.Direction)

	profile, err = svc.GetLanguageProfile(ctx, "ar")
	require.NoError(t, err)
	assert.Equal(t, "rtl", profile.Direction)

	_, err = svc.GetLanguageProfile(ctx, "xx")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not supported")
}

func TestLocalization_UpdateUserLocalization_WithDB(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	cacheService := NewCacheService(db, zap.NewNop())
	defer cacheService.Close()
	svc := NewLocalizationService(db, zap.NewNop(), nil, cacheService)
	ctx := context.Background()

	step := &WizardLocalizationStep{
		UserID:            1,
		PrimaryLanguage:   "en",
		SubtitleLanguages: []string{"en"},
		LyricsLanguages:   []string{"en"},
		MetadataLanguages: []string{"en"},
		PreferredRegion:   "US",
		DateFormat:        "MM/DD/YYYY",
		TimeFormat:        "12h",
		NumberFormat:      "en-US",
		CurrencyCode:      "USD",
	}

	_, err := svc.SetupUserLocalization(ctx, step)
	require.NoError(t, err)

	// Update multiple fields
	err = svc.UpdateUserLocalization(ctx, 1, map[string]interface{}{
		"primary_language":    "de",
		"preferred_region":    "DE",
		"currency_code":       "EUR",
		"secondary_languages": []string{"en", "fr"},
		"auto_translate":      true,
	})
	require.NoError(t, err)

	// Verify update
	loc, err := svc.GetUserLocalization(ctx, 1)
	require.NoError(t, err)
	assert.Equal(t, "de", loc.PrimaryLanguage)
	assert.Equal(t, "DE", loc.PreferredRegion)
	assert.Equal(t, "EUR", loc.CurrencyCode)

	// Empty updates
	err = svc.UpdateUserLocalization(ctx, 1, map[string]interface{}{})
	assert.Error(t, err)
}

func TestLocalization_ExportConfiguration_WithDB(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	// Create configuration_exports table
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS configuration_exports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL, config_type TEXT NOT NULL,
		config_data TEXT NOT NULL, description TEXT DEFAULT '',
		tags TEXT DEFAULT '[]',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)

	cacheService := NewCacheService(db, zap.NewNop())
	defer cacheService.Close()
	svc := NewLocalizationService(db, zap.NewNop(), nil, cacheService)
	ctx := context.Background()

	step := &WizardLocalizationStep{
		UserID:            1,
		PrimaryLanguage:   "en",
		SubtitleLanguages: []string{"en"},
		LyricsLanguages:   []string{"en"},
		MetadataLanguages: []string{"en"},
		PreferredRegion:   "US",
		DateFormat:        "MM/DD/YYYY",
		TimeFormat:        "12h",
		NumberFormat:      "en-US",
		CurrencyCode:      "USD",
	}

	_, err = svc.SetupUserLocalization(ctx, step)
	require.NoError(t, err)

	// Export localization config
	export, err := svc.ExportConfiguration(ctx, 1, "localization", "test export", []string{"test"})
	require.NoError(t, err)
	require.NotNil(t, export)
	assert.Equal(t, "1.0", export.Version)
	assert.Equal(t, "localization", export.ConfigType)
	assert.Equal(t, "test export", export.Description)
	require.NotNil(t, export.Localization)
	assert.Equal(t, "en", export.Localization.PrimaryLanguage)
	require.NotNil(t, export.WizardStep)

	// Export full config
	export, err = svc.ExportConfiguration(ctx, 1, "full", "full export", []string{"full"})
	require.NoError(t, err)
	require.NotNil(t, export)
	assert.Equal(t, "full", export.ConfigType)
	require.NotNil(t, export.MediaSettings)
	require.NotNil(t, export.PlaylistSettings)

	// Export media config
	export, err = svc.ExportConfiguration(ctx, 1, "media", "media export", []string{})
	require.NoError(t, err)
	require.NotNil(t, export)
	require.NotNil(t, export.MediaSettings)
	assert.Nil(t, export.Localization)

	// Export playlists config
	export, err = svc.ExportConfiguration(ctx, 1, "playlists", "playlists export", []string{})
	require.NoError(t, err)
	require.NotNil(t, export)
	require.NotNil(t, export.PlaylistSettings)
}

func TestLocalization_ImportConfiguration_WithDB(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	// Create required tables
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS configuration_exports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL, config_type TEXT NOT NULL,
		config_data TEXT NOT NULL, description TEXT DEFAULT '',
		tags TEXT DEFAULT '[]',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS configuration_import_log (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL, import_data TEXT NOT NULL,
		success BOOLEAN DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)

	cacheService := NewCacheService(db, zap.NewNop())
	defer cacheService.Close()
	svc := NewLocalizationService(db, zap.NewNop(), nil, cacheService)
	ctx := context.Background()

	// Setup initial localization
	step := &WizardLocalizationStep{
		UserID:            1,
		PrimaryLanguage:   "en",
		SubtitleLanguages: []string{"en"},
		LyricsLanguages:   []string{"en"},
		MetadataLanguages: []string{"en"},
		PreferredRegion:   "US",
		DateFormat:        "MM/DD/YYYY",
		TimeFormat:        "12h",
		NumberFormat:      "en-US",
		CurrencyCode:      "USD",
	}
	_, err = svc.SetupUserLocalization(ctx, step)
	require.NoError(t, err)

	// Create a valid config JSON
	config := ConfigurationExport{
		Version:    "1.0",
		ConfigType: "localization",
		Localization: &UserLocalization{
			PrimaryLanguage: "fr",
			DateFormat:      "DD/MM/YYYY",
			TimeFormat:      "24h",
		},
	}
	configJSON, _ := json.Marshal(config)

	// Import with import_localization
	options := map[string]bool{
		"import_localization": true,
	}
	result, err := svc.ImportConfiguration(ctx, 1, string(configJSON), options)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)

	// Invalid JSON
	_, err = svc.ImportConfiguration(ctx, 1, "invalid json", options)
	assert.Error(t, err)

	// Invalid config (validation fails)
	badConfig := ConfigurationExport{
		Version:    "",
		ConfigType: "invalid",
	}
	badJSON, _ := json.Marshal(badConfig)
	result, err = svc.ImportConfiguration(ctx, 1, string(badJSON), map[string]bool{})
	assert.Error(t, err)
	assert.False(t, result.Success)
	assert.Greater(t, len(result.ValidationErrors), 0)
}

// ============================================================================
// MediaRecognitionService — Additional Pure Function Tests
// ============================================================================

func newTestMediaRecognitionService() *MediaRecognitionService {
	return NewMediaRecognitionService(nil, zap.NewNop(), nil, nil, "", "", "", "", "", "")
}

func TestMediaRecognition_DetermineMatchType(t *testing.T) {
	svc := newTestMediaRecognitionService()

	assert.Equal(t, "exact", svc.determineMatchType(0.95))
	assert.Equal(t, "exact", svc.determineMatchType(1.0))
	assert.Equal(t, "high", svc.determineMatchType(0.9))
	assert.Equal(t, "high", svc.determineMatchType(0.85))
	assert.Equal(t, "medium", svc.determineMatchType(0.8))
	assert.Equal(t, "low", svc.determineMatchType(0.5))
	assert.Equal(t, "low", svc.determineMatchType(0.0))
}

func TestMediaRecognition_CalculateSimilarity_Stub(t *testing.T) {
	svc := newTestMediaRecognitionService()

	result := &MediaRecognitionResult{Title: "Test"}
	score := svc.calculateSimilarity(result, "Test", "{}", "{}")
	assert.Equal(t, 0.6, score) // Exact title match scores 0.6 (title weight)
}

func TestMediaRecognition_LooksLikeGame(t *testing.T) {
	svc := newTestMediaRecognitionService()

	tests := []struct {
		name     string
		fileName string
		expected bool
	}{
		{"ISO file", "game.iso", true},
		{"ROM file", "mario.rom", true},
		{"GBA file", "pokemon.gba", true},
		{"NSP file", "zelda.nsp", true},
		{"game keyword", "PC Game Collection.zip", true},
		{"steam keyword", "steam backup.zip", true},
		{"CODEX keyword", "game-CODEX.rar", true},
		{"repack keyword", "Title.repack.iso", true},
		{"normal video", "movie.mp4", false},
		{"normal audio", "song.mp3", false},
		{"document", "report.pdf", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, svc.looksLikeGame(tc.fileName))
		})
	}
}

func TestMediaRecognition_GetProviders(t *testing.T) {
	svc := newTestMediaRecognitionService()

	// All provider getters return empty slices (placeholders)
	assert.Empty(t, svc.getMusicProviders())
	assert.Empty(t, svc.getBookProviders())
	assert.Empty(t, svc.getGameProviders())
}

// ============================================================================
// VideoPlayerService — DB-backed Tests
// ============================================================================

func TestVideoPlayer_PlayVideo_WithDB(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	positionSvc := NewPlaybackPositionService(db, zap.NewNop())
	svc := NewVideoPlayerService(db, zap.NewNop(), nil, positionSvc, nil, nil, nil)
	ctx := context.Background()

	// Insert a video media item
	_, err := db.ExecContext(ctx, `
		INSERT INTO media_items (id, path, title, type, file_path, duration, resolution, codec,
			aspect_ratio, frame_rate, bitrate, file_size, year, language, country,
			genres, directors, actors, writers, imdb_id, tmdb_id, hdr, dolby_vision, dolby_atmos,
			original_title, description, play_count, watched_percentage, is_favorite, user_rating, rating)
		VALUES (100, '/movies/inception.mkv', 'Inception', 'video', '/movies/inception.mkv',
			8880000, '1920x1080', 'h264', '16:9', 23.976, 8000000, 15000000000, 2010,
			'en', 'US', '["Sci-Fi","Thriller"]', '["Christopher Nolan"]', '["Leonardo DiCaprio"]',
			'["Christopher Nolan"]', 'tt1375666', '27205', 0, 0, 0,
			'Inception', 'A mind-bending thriller', 5, 75.5, 0, 0, 0)
	`)
	require.NoError(t, err)

	req := &PlayVideoRequest{
		UserID:   1,
		VideoID:  100,
		PlayMode: VideoPlayModeSingle,
		Quality:  Quality1080p,
		AutoPlay: true,
	}

	session, err := svc.PlayVideo(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, int64(1), session.UserID)
	assert.Equal(t, "Inception", session.CurrentVideo.Title)
	assert.Equal(t, VideoPlayModeSingle, session.PlayMode)
	assert.Equal(t, PlaybackStatePlaying, session.PlaybackState)
	assert.Equal(t, 1.0, session.Volume)
	assert.Equal(t, 1.0, session.PlaybackSpeed)
	assert.True(t, session.AutoPlay)

	// Play with start time
	startTime := int64(5000)
	req2 := &PlayVideoRequest{
		UserID:    1,
		VideoID:   100,
		PlayMode:  VideoPlayModeSingle,
		Quality:   Quality720p,
		AutoPlay:  false,
		StartTime: &startTime,
	}

	session2, err := svc.PlayVideo(ctx, req2)
	require.NoError(t, err)
	assert.Equal(t, int64(5000), session2.Position)

	// Play non-existent video
	req3 := &PlayVideoRequest{
		UserID:   1,
		VideoID:  999,
		PlayMode: VideoPlayModeSingle,
	}
	_, err = svc.PlayVideo(ctx, req3)
	assert.Error(t, err)
}

func TestVideoPlayer_UpdatePlayback_WithDB(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	positionSvc := NewPlaybackPositionService(db, zap.NewNop())
	svc := NewVideoPlayerService(db, zap.NewNop(), nil, positionSvc, nil, nil, nil)
	ctx := context.Background()

	// Insert video
	_, err := db.ExecContext(ctx, `
		INSERT INTO media_items (id, path, title, type, file_path, duration, resolution, codec,
			aspect_ratio, frame_rate, bitrate, file_size, year, language, country,
			genres, directors, actors, writers, imdb_id, tmdb_id, hdr, dolby_vision, dolby_atmos,
			original_title, description, play_count, watched_percentage, is_favorite, user_rating, rating)
		VALUES (200, '/movies/test.mkv', 'Test Video', 'video', '/movies/test.mkv',
			120000, '1920x1080', 'h264', '16:9', 24.0, 5000000, 5000000000, 2024,
			'en', 'US', '[]', '[]', '[]', '[]', '', '', 0, 0, 0,
			'Test Video', 'A test video', 0, 0.0, 0, 0, 0)
	`)
	require.NoError(t, err)

	// Create a session first
	req := &PlayVideoRequest{
		UserID:   1,
		VideoID:  200,
		PlayMode: VideoPlayModeSingle,
		Quality:  Quality720p,
	}
	session, err := svc.PlayVideo(ctx, req)
	require.NoError(t, err)

	// Update playback
	pos := int64(60000)
	vol := 0.8
	pausedState := PlaybackStatePaused
	updateReq := &UpdateVideoPlaybackRequest{
		SessionID: session.ID,
		Position:  &pos,
		State:     &pausedState,
		Volume:    &vol,
	}

	updated, err := svc.UpdateVideoPlayback(ctx, updateReq)
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, int64(60000), updated.Position)
	assert.Equal(t, PlaybackStatePaused, updated.PlaybackState)
	assert.Equal(t, 0.8, updated.Volume)
}

func TestVideoPlayer_GetWatchHistory_WithDB(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	svc := NewVideoPlayerService(db, zap.NewNop(), nil, nil, nil, nil, nil)
	ctx := context.Background()

	// Create video_watch_history table
	_, err := db.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS video_watch_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			video_id INTEGER NOT NULL,
			watched_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			watch_duration INTEGER DEFAULT 0,
			completion_rate REAL DEFAULT 0,
			stopped_at INTEGER DEFAULT 0,
			device_info TEXT DEFAULT '',
			quality TEXT DEFAULT ''
		)
	`)
	require.NoError(t, err)

	// Insert watch history
	_, err = db.ExecContext(ctx, `
		INSERT INTO video_watch_history (user_id, video_id, watched_at, watch_duration, completion_rate, stopped_at, device_info, quality)
		VALUES (1, 301, '2024-01-01 10:00:00', 50000, 50.0, 25000, 'desktop', '1080p')
	`)
	require.NoError(t, err)

	history, err := svc.GetWatchHistory(ctx, &WatchHistoryRequest{UserID: 1, Limit: 10, Offset: 0})
	require.NoError(t, err)
	assert.NotNil(t, history)
}

func TestVideoPlayer_GetContinueWatching_WithDB(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	svc := NewVideoPlayerService(db, zap.NewNop(), nil, nil, nil, nil, nil)
	ctx := context.Background()

	items, err := svc.GetContinueWatching(ctx, 1, 10)
	require.NoError(t, err)
	assert.NotNil(t, items)
}

// ============================================================================
// MusicPlayerService — DB-backed Tests
// ============================================================================

func TestMusicPlayer_PlayTrack_WithDB(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	// Create music_playback_sessions table
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS music_playback_sessions (
		id TEXT PRIMARY KEY, user_id INTEGER NOT NULL,
		session_data TEXT NOT NULL, expires_at DATETIME NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	)`)
	require.NoError(t, err)

	positionSvc := NewPlaybackPositionService(db, zap.NewNop())
	svc := NewMusicPlayerService(db, zap.NewNop(), nil, nil, positionSvc, nil, nil, nil)
	ctx := context.Background()

	// Insert a music track
	_, err = db.ExecContext(ctx, `
		INSERT INTO media_items (id, path, title, type, file_path, duration,
			artist, album, genre, year, track_number, disc_number,
			bitrate, sample_rate, channels, format, codec, file_size,
			resolution, aspect_ratio, frame_rate, language, country,
			genres, directors, actors, writers, imdb_id, tmdb_id,
			hdr, dolby_vision, dolby_atmos, original_title, description,
			play_count, watched_percentage, is_favorite, user_rating, rating)
		VALUES (400, '/music/song.flac', 'Test Song', 'audio', '/music/song.flac', 240000,
			'Test Artist', 'Test Album', 'Rock', 2024, 1, 1,
			1411, 44100, 2, 'flac', 'flac', 30000000,
			'', '', 0, 'en', 'US',
			'["Rock"]', '[]', '[]', '[]', '', '',
			0, 0, 0, '', '',
			0, 0.0, 0, 0, 0)
	`)
	require.NoError(t, err)

	session, err := svc.PlayTrack(ctx, &PlayTrackRequest{UserID: 1, TrackID: 400})
	require.NoError(t, err)
	require.NotNil(t, session)
	assert.Equal(t, int64(1), session.UserID)
	require.NotNil(t, session.CurrentTrack)
	assert.Equal(t, "Test Song", session.CurrentTrack.Title)
	assert.Equal(t, "Test Artist", session.CurrentTrack.Artist)
	assert.Equal(t, PlaybackStatePlaying, session.PlaybackState)

	// Play non-existent track
	_, err = svc.PlayTrack(ctx, &PlayTrackRequest{UserID: 1, TrackID: 999})
	assert.Error(t, err)
}

func TestMusicPlayer_SetEqualizer_WithDB(t *testing.T) {
	db := setupIntegrationTestDB(t)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS music_playback_sessions (
		id TEXT PRIMARY KEY, user_id INTEGER NOT NULL,
		session_data TEXT NOT NULL, expires_at DATETIME NOT NULL,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id)
	)`)
	require.NoError(t, err)

	positionSvc := NewPlaybackPositionService(db, zap.NewNop())
	svc := NewMusicPlayerService(db, zap.NewNop(), nil, nil, positionSvc, nil, nil, nil)
	ctx := context.Background()

	// Insert a track
	_, err = db.ExecContext(ctx, `
		INSERT INTO media_items (id, path, title, type, file_path, duration,
			artist, album, genre, year, track_number, disc_number,
			bitrate, sample_rate, channels, format, codec, file_size,
			resolution, aspect_ratio, frame_rate, language, country,
			genres, directors, actors, writers, imdb_id, tmdb_id,
			hdr, dolby_vision, dolby_atmos, original_title, description,
			play_count, watched_percentage, is_favorite, user_rating, rating)
		VALUES (500, '/music/eq.flac', 'EQ Test', 'audio', '/music/eq.flac', 180000,
			'Artist', 'Album', 'Pop', 2024, 1, 1,
			320, 44100, 2, 'mp3', 'mp3', 5000000,
			'', '', 0, '', '',
			'[]', '[]', '[]', '[]', '', '',
			0, 0, 0, '', '',
			0, 0.0, 0, 0, 0)
	`)
	require.NoError(t, err)

	// Create session
	session, err := svc.PlayTrack(ctx, &PlayTrackRequest{UserID: 1, TrackID: 500})
	require.NoError(t, err)

	// Set equalizer
	eqBands := map[string]float64{
		"60Hz":  2.0,
		"230Hz": 1.5,
		"910Hz": 0.0,
		"4kHz":  -1.0,
		"14kHz": 3.0,
	}
	err = svc.SetEqualizer(ctx, session.ID, "custom", eqBands)
	require.NoError(t, err)
}

func TestMediaRecognition_DetectFromFileName(t *testing.T) {
	svc := newTestMediaRecognitionService()

	tests := []struct {
		name     string
		fileName string
		expected MediaType
	}{
		{"MP4 video", "movie.mp4", MediaTypeMovie},
		{"MKV video", "video.mkv", MediaTypeMovie},
		{"MP3 audio", "song.mp3", MediaTypeMusic},
		{"FLAC audio", "album.flac", MediaTypeMusic},
		{"PDF document", "book.pdf", MediaTypeDocument},
		{"JPG image", "photo.jpg", MediaTypeImage},
		{"PNG image", "screenshot.png", MediaTypeImage},
		{"Unknown", "file.xyz", MediaTypeUnknown},
		{"No extension", "noext", MediaTypeUnknown},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, svc.detectFromFileName(tc.fileName))
		})
	}
}
