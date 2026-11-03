package handlers

import (
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"catalogizer/internal/services"
)

// SubtitleHandler handles subtitle-related HTTP requests
type SubtitleHandler struct {
	subtitleService *services.SubtitleService
	logger          *zap.Logger
}

// NewSubtitleHandler creates a new subtitle handler
func NewSubtitleHandler(subtitleService *services.SubtitleService, logger *zap.Logger) *SubtitleHandler {
	return &SubtitleHandler{
		subtitleService: subtitleService,
		logger:          logger,
	}
}

// SubtitleSearchResponse represents the response for subtitle search
type SubtitleSearchResponse struct {
	Success bool                            `json:"success"`
	Message string                          `json:"message,omitempty"`
	Results []services.SubtitleSearchResult `json:"results"`
	Count   int                             `json:"count"`
}

// newSubtitleSearchResponse builds the search body. The list is always a JSON
// array (never omitted, never null): catalog-web iterates it unguarded.
func newSubtitleSearchResponse(results []services.SubtitleSearchResult) SubtitleSearchResponse {
	if results == nil {
		results = []services.SubtitleSearchResult{}
	}
	return SubtitleSearchResponse{Success: true, Results: results, Count: len(results)}
}

// newSubtitleListResponse builds the per-media list body (always an array).
func newSubtitleListResponse(mediaID int64, subtitles []services.SubtitleTrack) SubtitleListResponse {
	if subtitles == nil {
		subtitles = []services.SubtitleTrack{}
	}
	return SubtitleListResponse{Success: true, Subtitles: subtitles, MediaItemID: mediaID}
}

// SubtitleDownloadResponse represents the response for subtitle download
type SubtitleDownloadResponse struct {
	Success bool                    `json:"success"`
	Message string                  `json:"message,omitempty"`
	Track   *services.SubtitleTrack `json:"track,omitempty"`
}

// SubtitleUploadResponse represents the response for subtitle upload
type SubtitleUploadResponse struct {
	Success bool                    `json:"success"`
	Message string                  `json:"message,omitempty"`
	Track   *services.SubtitleTrack `json:"track,omitempty"`
}

// SubtitleListResponse represents the response for subtitle list
type SubtitleListResponse struct {
	Success     bool                     `json:"success"`
	Message     string                   `json:"message,omitempty"`
	Subtitles   []services.SubtitleTrack `json:"subtitles"`
	MediaItemID int64                    `json:"media_item_id"`
}

// SubtitleSyncResponse represents the response for subtitle sync verification
type SubtitleSyncResponse struct {
	Success    bool                         `json:"success"`
	Message    string                       `json:"message,omitempty"`
	SyncResult *services.SubtitleSyncResult `json:"sync_result,omitempty"`
}

// SubtitleTranslationResponse represents the response for subtitle translation
type SubtitleTranslationResponse struct {
	Success         bool                    `json:"success"`
	Message         string                  `json:"message,omitempty"`
	TranslatedTrack *services.SubtitleTrack `json:"translated_track,omitempty"`
}

// SearchSubtitles handles subtitle search requests
// @Summary Search subtitles
// @Description Search for subtitles across multiple providers
// @Tags subtitles
// @Accept json
// @Produce json
// @Param media_path query string true "Media file path"
// @Param title query string false "Media title"
// @Param year query int false "Release year"
// @Param season query int false "TV season number"
// @Param episode query int false "TV episode number"
// @Param languages query []string false "Language codes (comma separated)"
// @Param providers query []string false "Providers to search (comma separated)"
// @Success 200 {object} SubtitleSearchResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subtitles/search [get]
func (h *SubtitleHandler) SearchSubtitles(c *gin.Context) {
	h.logger.Info("Subtitle search request received")

	// Parse query parameters
	request := &services.SubtitleSearchRequest{
		MediaPath: c.Query("media_path"),
	}

	// Optional parameters
	if title := c.Query("title"); title != "" {
		request.Title = &title
	}

	if yearStr := c.Query("year"); yearStr != "" {
		if year, err := strconv.Atoi(yearStr); err == nil {
			request.Year = &year
		}
	}

	if seasonStr := c.Query("season"); seasonStr != "" {
		if season, err := strconv.Atoi(seasonStr); err == nil {
			request.Season = &season
		}
	}

	if episodeStr := c.Query("episode"); episodeStr != "" {
		if episode, err := strconv.Atoi(episodeStr); err == nil {
			request.Episode = &episode
		}
	}

	if languagesStr := c.Query("languages"); languagesStr != "" {
		request.Languages = strings.Split(languagesStr, ",")
		for i, lang := range request.Languages {
			request.Languages[i] = strings.TrimSpace(lang)
		}
	}

	if providersStr := c.Query("providers"); providersStr != "" {
		providerStrings := strings.Split(providersStr, ",")
		for _, provStr := range providerStrings {
			provStr = strings.TrimSpace(provStr)
			switch services.SubtitleProvider(provStr) {
			case services.ProviderOpenSubtitles, services.ProviderSubDB, services.ProviderYifySubtitles,
				services.ProviderSubscene, services.ProviderAddic7ed:
				request.Providers = append(request.Providers, services.SubtitleProvider(provStr))
			}
		}
	}

	// Validate required parameters
	if request.MediaPath == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "media_path is required",
			Code:    "MISSING_MEDIA_PATH",
		})
		return
	}

	// Search for subtitles
	ctx := c.Request.Context()
	results, err := h.subtitleService.SearchSubtitles(ctx, request)
	if err != nil {
		h.logger.Error("Failed to search subtitles", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   "Failed to search subtitles: " + err.Error(),
			Code:    "SEARCH_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, newSubtitleSearchResponse(results))
}

// DownloadSubtitle handles subtitle download requests
// @Summary Download subtitle
// @Description Download a subtitle by result ID
// @Tags subtitles
// @Accept json
// @Produce json
// @Param request body services.SubtitleDownloadRequest true "Download request"
// @Success 200 {object} SubtitleDownloadResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subtitles/download [post]
func (h *SubtitleHandler) DownloadSubtitle(c *gin.Context) {
	h.logger.Info("Subtitle download request received")

	var request services.SubtitleDownloadRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Error("Invalid request format", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "Invalid request format: " + err.Error(),
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Validate required parameters
	if request.MediaItemID == 0 || request.ResultID == "" || request.Language == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "media_item_id, result_id, and language are required",
			Code:    "MISSING_REQUIRED_FIELDS",
		})
		return
	}

	// Download subtitle
	ctx := c.Request.Context()
	track, err := h.subtitleService.DownloadSubtitle(ctx, &request)
	if err != nil {
		h.logger.Error("Failed to download subtitle",
			zap.Int64("media_item_id", request.MediaItemID),
			zap.String("result_id", request.ResultID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   "Failed to download subtitle: " + err.Error(),
			Code:    "DOWNLOAD_FAILED",
		})
		return
	}

	h.logger.Info("Subtitle downloaded successfully",
		zap.String("subtitle_id", track.ID),
		zap.String("language", track.Language))

	c.JSON(http.StatusOK, SubtitleDownloadResponse{
		Success: true,
		Track:   track,
	})
}

// GetSubtitles handles requests to get all subtitles for a media item
// @Summary Get media subtitles
// @Description Get all subtitle tracks for a media item
// @Tags subtitles
// @Accept json
// @Produce json
// @Param media_id path int true "Media item ID"
// @Success 200 {object} SubtitleListResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subtitles/media/{media_id} [get]
func (h *SubtitleHandler) GetSubtitles(c *gin.Context) {
	mediaIDStr := c.Param("media_id")
	if mediaIDStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "media_id is required",
			Code:    "MISSING_MEDIA_ID",
		})
		return
	}

	mediaID, err := strconv.ParseInt(mediaIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "Invalid media_id format",
			Code:    "INVALID_MEDIA_ID",
		})
		return
	}

	h.logger.Info("Get subtitles request", zap.Int64("media_id", mediaID))

	// Get subtitles
	ctx := c.Request.Context()
	subtitles, err := h.subtitleService.GetSubtitles(ctx, mediaID)
	if err != nil {
		h.logger.Error("Failed to get subtitles",
			zap.Int64("media_id", mediaID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   "Failed to get subtitles: " + err.Error(),
			Code:    "GET_SUBTITLES_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, newSubtitleListResponse(mediaID, subtitles))
}

// VerifySubtitleSync handles requests to verify subtitle synchronization
// @Summary Verify subtitle sync
// @Description Verify if a subtitle is properly synchronized with video
// @Tags subtitles
// @Accept json
// @Produce json
// @Param media_id path int true "Media item ID"
// @Param subtitle_id path string true "Subtitle ID"
// @Success 200 {object} SubtitleSyncResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subtitles/{subtitle_id}/verify-sync/{media_id} [get]
func (h *SubtitleHandler) VerifySubtitleSync(c *gin.Context) {
	subtitleID := c.Param("subtitle_id")
	mediaIDStr := c.Param("media_id")

	if subtitleID == "" || mediaIDStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "subtitle_id and media_id are required",
			Code:    "MISSING_REQUIRED_PARAMS",
		})
		return
	}

	mediaID, err := strconv.ParseInt(mediaIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "Invalid media_id format",
			Code:    "INVALID_MEDIA_ID",
		})
		return
	}

	h.logger.Info("Verify subtitle sync request",
		zap.String("subtitle_id", subtitleID),
		zap.Int64("media_id", mediaID))

	// Get subtitle track
	ctx := c.Request.Context()
	track, err := h.subtitleService.GetSubtitleTrack(ctx, subtitleID)
	if err != nil {
		h.logger.Error("Failed to get subtitle track",
			zap.String("subtitle_id", subtitleID),
			zap.Error(err))
		c.JSON(http.StatusNotFound, ErrorResponse{
			Success: false,
			Error:   "Subtitle not found: " + err.Error(),
			Code:    "SUBTITLE_NOT_FOUND",
		})
		return
	}

	// Verify synchronization
	syncResult, err := h.subtitleService.VerifySynchronization(ctx, mediaID, track)
	if err != nil {
		h.logger.Error("Failed to verify subtitle sync",
			zap.String("subtitle_id", subtitleID),
			zap.Int64("media_id", mediaID),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   "Failed to verify subtitle sync: " + err.Error(),
			Code:    "SYNC_VERIFICATION_FAILED",
		})
		return
	}

	c.JSON(http.StatusOK, SubtitleSyncResponse{
		Success:    true,
		SyncResult: syncResult,
	})
}

// TranslateSubtitle handles subtitle translation requests
// @Summary Translate subtitle
// @Description Translate a subtitle to another language
// @Tags subtitles
// @Accept json
// @Produce json
// @Param request body services.SubtitleTranslationRequest true "Translation request"
// @Success 200 {object} SubtitleTranslationResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subtitles/translate [post]
func (h *SubtitleHandler) TranslateSubtitle(c *gin.Context) {
	h.logger.Info("Subtitle translation request received")

	var request services.SubtitleTranslationRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.logger.Error("Invalid request format", zap.Error(err))
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "Invalid request format: " + err.Error(),
			Code:    "INVALID_REQUEST",
		})
		return
	}

	// Validate required parameters
	if request.SubtitleID == "" || request.SourceLanguage == "" || request.TargetLanguage == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "subtitle_id, source_language, and target_language are required",
			Code:    "MISSING_REQUIRED_FIELDS",
		})
		return
	}

	// Translate subtitle
	ctx := c.Request.Context()
	translatedTrack, err := h.subtitleService.TranslateSubtitle(ctx, &request)
	if err != nil {
		h.logger.Error("Failed to translate subtitle",
			zap.String("subtitle_id", request.SubtitleID),
			zap.String("source_language", request.SourceLanguage),
			zap.String("target_language", request.TargetLanguage),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   "Failed to translate subtitle: " + err.Error(),
			Code:    "TRANSLATION_FAILED",
		})
		return
	}

	h.logger.Info("Subtitle translated successfully",
		zap.String("subtitle_id", request.SubtitleID),
		zap.String("target_language", request.TargetLanguage))

	c.JSON(http.StatusOK, SubtitleTranslationResponse{
		Success:         true,
		TranslatedTrack: translatedTrack,
	})
}

// GetSupportedLanguages handles requests to get supported subtitle languages
// @Summary Get supported languages
// @Description Get list of supported subtitle languages
// @Tags subtitles
// @Accept json
// @Produce json
// @Success 200 {object} LanguageListResponse
// @Router /api/v1/subtitles/languages [get]
func (h *SubtitleHandler) GetSupportedLanguages(c *gin.Context) {
	h.logger.Info("Get supported languages request received")

	// This is a mock implementation - in real scenario, this would be from a database or API
	languages := []SubtitleLanguage{
		{Code: "en", Name: "English", NativeName: "English"},
		{Code: "es", Name: "Spanish", NativeName: "Español"},
		{Code: "fr", Name: "French", NativeName: "Français"},
		{Code: "de", Name: "German", NativeName: "Deutsch"},
		{Code: "it", Name: "Italian", NativeName: "Italiano"},
		{Code: "pt", Name: "Portuguese", NativeName: "Português"},
		{Code: "ru", Name: "Russian", NativeName: "Русский"},
		{Code: "ja", Name: "Japanese", NativeName: "日本語"},
		{Code: "ko", Name: "Korean", NativeName: "한국어"},
		{Code: "zh", Name: "Chinese", NativeName: "中文"},
		{Code: "ar", Name: "Arabic", NativeName: "العربية"},
		{Code: "hi", Name: "Hindi", NativeName: "हिन्दी"},
		{Code: "nl", Name: "Dutch", NativeName: "Nederlands"},
		{Code: "sv", Name: "Swedish", NativeName: "Svenska"},
		{Code: "no", Name: "Norwegian", NativeName: "Norsk"},
		{Code: "da", Name: "Danish", NativeName: "Dansk"},
		{Code: "fi", Name: "Finnish", NativeName: "Suomi"},
		{Code: "pl", Name: "Polish", NativeName: "Polski"},
		{Code: "tr", Name: "Turkish", NativeName: "Türkçe"},
	}

	c.JSON(http.StatusOK, LanguageListResponse{
		Success:   true,
		Languages: languages,
		Count:     len(languages),
	})
}

// GetSupportedProviders handles requests to get supported subtitle providers
// @Summary Get supported providers
// @Description Get list of supported subtitle providers
// @Tags subtitles
// @Accept json
// @Produce json
// @Success 200 {object} ProviderListResponse
// @Router /api/v1/subtitles/providers [get]
func (h *SubtitleHandler) GetSupportedProviders(c *gin.Context) {
	h.logger.Info("Get supported providers request received")

	providers := []services.SubtitleProvider{
		services.ProviderOpenSubtitles,
		services.ProviderSubDB,
		services.ProviderYifySubtitles,
		services.ProviderSubscene,
		services.ProviderAddic7ed,
	}

	providerInfo := make([]ProviderInfo, len(providers))
	for i, provider := range providers {
		providerInfo[i] = ProviderInfo{
			Provider:    string(provider),
			Name:        getProviderDisplayName(provider),
			Description: getProviderDescription(provider),
			Supported:   true,
		}
	}

	c.JSON(http.StatusOK, ProviderListResponse{
		Success:   true,
		Providers: providerInfo,
		Count:     len(providerInfo),
	})
}

// Helper types
type LanguageListResponse struct {
	Success   bool               `json:"success"`
	Languages []SubtitleLanguage `json:"languages"`
	Count     int                `json:"count"`
}

type ProviderListResponse struct {
	Success   bool           `json:"success"`
	Providers []ProviderInfo `json:"providers"`
	Count     int            `json:"count"`
}

type ProviderInfo struct {
	Provider    string `json:"provider"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Supported   bool   `json:"supported"`
}

// Helper types for responses (these should be moved to a common types package)
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
	Code    string `json:"code"`
}

// SubtitleLanguage represents a supported subtitle language
type SubtitleLanguage struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	NativeName string `json:"native_name"`
}

// Helper functions
func getProviderDisplayName(provider services.SubtitleProvider) string {
	switch provider {
	case services.ProviderOpenSubtitles:
		return "OpenSubtitles"
	case services.ProviderSubDB:
		return "SubDB"
	case services.ProviderYifySubtitles:
		return "YIFY Subtitles"
	case services.ProviderSubscene:
		return "Subscene"
	case services.ProviderAddic7ed:
		return "Addic7ed"
	default:
		return string(provider)
	}
}

func getProviderDescription(provider services.SubtitleProvider) string {
	switch provider {
	case services.ProviderOpenSubtitles:
		return "Large subtitle database with multiple languages"
	case services.ProviderSubDB:
		return "Hash-based subtitle matching"
	case services.ProviderYifySubtitles:
		return "Subtitles for YIFY movie releases"
	case services.ProviderSubscene:
		return "Community-driven subtitle site"
	case services.ProviderAddic7ed:
		return "TV show subtitles with translations"
	default:
		return "Subtitle provider"
	}
}

// UploadSubtitle handles subtitle upload requests
// @Summary Upload subtitle
// @Description Upload a subtitle file for a media item
// @Tags subtitles
// @Accept multipart/form-data
// @Produce json
// @Param media_item_id formData int true "Media item ID"
// @Param language formData string true "Subtitle language"
// @Param language_code formData string true "Language code (e.g., 'en')"
// @Param file formData file true "Subtitle file"
// @Success 200 {object} SubtitleUploadResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/subtitles/upload [post]
func (h *SubtitleHandler) UploadSubtitle(c *gin.Context) {
	h.logger.Info("Subtitle upload request received")

	// Get form values
	mediaIDStr := c.PostForm("media_item_id")
	language := c.PostForm("language")
	languageCode := c.PostForm("language_code")

	// Validate required parameters
	if mediaIDStr == "" || language == "" || languageCode == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "media_item_id, language, and language_code are required",
			Code:    "MISSING_REQUIRED_FIELDS",
		})
		return
	}

	mediaID, err := strconv.ParseInt(mediaIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "Invalid media_item_id format",
			Code:    "INVALID_MEDIA_ID",
		})
		return
	}

	// Get uploaded file
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Error:   "file is required",
			Code:    "MISSING_FILE",
		})
		return
	}

	// Open the uploaded file
	fileContent, err := file.Open()
	if err != nil {
		h.logger.Error("Failed to open uploaded file", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   "Failed to open uploaded file",
			Code:    "FILE_OPEN_ERROR",
		})
		return
	}
	defer fileContent.Close()

	// Read file content
	fileContentBytes, err := io.ReadAll(fileContent)
	if err != nil {
		h.logger.Error("Failed to read uploaded file", zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   "Failed to read uploaded file",
			Code:    "FILE_READ_ERROR",
		})
		return
	}

	// Store content as string
	contentStr := string(fileContentBytes)

	// Detect subtitle format from file extension
	format := "srt" // Default
	ext := strings.ToLower(filepath.Ext(file.Filename))
	switch ext {
	case ".srt":
		format = "srt"
	case ".vtt":
		format = "vtt"
	case ".ass", ".ssa":
		format = "ass"
	case ".txt":
		format = "txt"
	}

	// Create subtitle upload request
	request := &services.SubtitleUploadRequest{
		MediaID:      mediaID,
		Language:     language,
		LanguageCode: languageCode,
		Format:       format,
		Content:      contentStr,
		IsDefault:    false,
		IsForced:     false,
		Encoding:     "utf-8",
		SyncOffset:   0.0,
	}

	// Save to database
	ctx := c.Request.Context()
	response, err := h.subtitleService.SaveUploadedSubtitle(ctx, request)
	if err != nil {
		h.logger.Error("Failed to save uploaded subtitle",
			zap.Int64("media_item_id", mediaID),
			zap.String("language", language),
			zap.Error(err))
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Error:   "Failed to save uploaded subtitle",
			Code:    "SAVE_ERROR",
		})
		return
	}

	h.logger.Info("Subtitle uploaded successfully",
		zap.Int64("media_item_id", mediaID),
		zap.String("subtitle_id", response.SubtitleID),
		zap.String("language", language))

	c.JSON(http.StatusOK, SubtitleUploadResponse{
		Success: true,
		Message: "Subtitle uploaded successfully",
	})
}
