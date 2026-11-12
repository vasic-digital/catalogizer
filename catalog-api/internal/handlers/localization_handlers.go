package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"catalogizer/internal/services"
)

type LocalizationHandlers struct {
	logger              *zap.Logger
	localizationService *services.LocalizationService
}

func NewLocalizationHandlers(
	logger *zap.Logger,
	localizationService *services.LocalizationService,
) *LocalizationHandlers {
	return &LocalizationHandlers{
		logger:              logger,
		localizationService: localizationService,
	}
}

func (h *LocalizationHandlers) RegisterRoutes(router *mux.Router) {
	api := router.PathPrefix("/api/v1").Subrouter()

	// Installation Wizard Routes
	api.HandleFunc("/wizard/localization/defaults", h.GetWizardDefaults).Methods("GET", "OPTIONS")
	api.HandleFunc("/wizard/localization/setup", h.SetupWizardLocalization).Methods("POST", "OPTIONS")

	// JSON Configuration Routes
	api.HandleFunc("/wizard/configuration/export", h.ExportConfiguration).Methods("POST", "OPTIONS")
	api.HandleFunc("/wizard/configuration/import", h.ImportConfiguration).Methods("POST", "OPTIONS")
	api.HandleFunc("/wizard/configuration/validate", h.ValidateConfiguration).Methods("POST", "OPTIONS")
	api.HandleFunc("/wizard/configuration/edit", h.EditConfiguration).Methods("POST", "OPTIONS")
	api.HandleFunc("/wizard/configuration/templates", h.GetConfigurationTemplates).Methods("GET", "OPTIONS")

	// Localization Management Routes
	api.HandleFunc("/localization", h.GetUserLocalization).Methods("GET", "OPTIONS")
	api.HandleFunc("/localization", h.UpdateUserLocalization).Methods("PUT", "OPTIONS")
	api.HandleFunc("/localization/languages", h.GetSupportedLanguages).Methods("GET", "OPTIONS")
	api.HandleFunc("/localization/languages/{languageCode}", h.GetLanguageProfile).Methods("GET", "OPTIONS")
	api.HandleFunc("/localization/preferences/{contentType}", h.GetContentLanguagePreferences).Methods("GET", "OPTIONS")
	api.HandleFunc("/localization/stats", h.GetLocalizationStats).Methods("GET", "OPTIONS")

	// Content-specific Routes
	api.HandleFunc("/localization/detect", h.DetectLanguage).Methods("POST", "OPTIONS")
	api.HandleFunc("/localization/check-support", h.CheckLanguageSupport).Methods("POST", "OPTIONS")
	api.HandleFunc("/localization/format-datetime", h.FormatDateTime).Methods("POST", "OPTIONS")

	// Add CORS middleware
	api.Use(h.corsMiddleware)
}

// Installation Wizard Handlers

func (h *LocalizationHandlers) GetWizardDefaults(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Getting wizard localization defaults")

	userAgent := r.Header.Get("User-Agent")
	acceptLanguage := r.Header.Get("Accept-Language")

	detectedLanguage := h.localizationService.DetectUserLanguage(r.Context(), userAgent, acceptLanguage)
	defaults := h.localizationService.GetWizardDefaults(r.Context(), detectedLanguage)

	h.sendSuccess(w, map[string]interface{}{
		"detected_language": detectedLanguage,
		"defaults":          defaults,
	})
}

func (h *LocalizationHandlers) SetupWizardLocalization(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("Setting up wizard localization")

	var req services.WizardLocalizationStep
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserID == 0 {
		userID := h.getUserID(r)
		if userID == 0 {
			h.sendError(w, "User not authenticated", http.StatusUnauthorized)
			return
		}
		req.UserID = userID
	}

	if req.PrimaryLanguage == "" {
		h.sendError(w, "Primary language is required", http.StatusBadRequest)
		return
	}

	localization, err := h.localizationService.SetupUserLocalization(r.Context(), &req)
	if err != nil {
		h.logger.Error("Failed to setup user localization", zap.Error(err))
		h.sendError(w, "Failed to setup localization preferences", http.StatusInternalServerError)
		return
	}

	h.sendSuccess(w, map[string]interface{}{
		"localization": localization,
		"message":      "Localization preferences configured successfully",
	})
}

// Localization Management Handlers

func (h *LocalizationHandlers) GetUserLocalization(w http.ResponseWriter, r *http.Request) {
	userID := h.getUserID(r)
	if userID == 0 {
		h.sendError(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	localization, err := h.localizationService.GetUserLocalization(r.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to get user localization", zap.Error(err))
		h.sendError(w, "Failed to get localization preferences", http.StatusInternalServerError)
		return
	}

	h.sendSuccess(w, localization)
}

func (h *LocalizationHandlers) UpdateUserLocalization(w http.ResponseWriter, r *http.Request) {
	userID := h.getUserID(r)
	if userID == 0 {
		h.sendError(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		h.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := h.localizationService.UpdateUserLocalization(r.Context(), userID, updates)
	if err != nil {
		h.logger.Error("Failed to update user localization", zap.Error(err))
		h.sendError(w, "Failed to update localization preferences", http.StatusInternalServerError)
		return
	}

	h.sendSuccess(w, map[string]string{"message": "Localization preferences updated successfully"})
}

func (h *LocalizationHandlers) GetSupportedLanguages(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Getting supported languages")

	languages, err := h.localizationService.GetSupportedLanguages(r.Context())
	if err != nil {
		h.logger.Error("Failed to get supported languages", zap.Error(err))
		h.sendError(w, "Failed to get supported languages", http.StatusInternalServerError)
		return
	}

	h.sendSuccess(w, languages)
}

func (h *LocalizationHandlers) GetLanguageProfile(w http.ResponseWriter, r *http.Request) {
	languageCode := mux.Vars(r)["languageCode"]

	profile, err := h.localizationService.GetLanguageProfile(r.Context(), languageCode)
	if err != nil {
		h.logger.Error("Failed to get language profile",
			zap.String("language", languageCode),
			zap.Error(err))
		h.sendError(w, "Language not supported", http.StatusNotFound)
		return
	}

	h.sendSuccess(w, profile)
}

func (h *LocalizationHandlers) GetContentLanguagePreferences(w http.ResponseWriter, r *http.Request) {
	userID := h.getUserID(r)
	if userID == 0 {
		h.sendError(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	contentType := mux.Vars(r)["contentType"]

	languages, err := h.localizationService.GetPreferredLanguagesForContent(r.Context(), userID, contentType)
	if err != nil {
		h.logger.Error("Failed to get content language preferences",
			zap.Int64("user_id", userID),
			zap.String("content_type", contentType),
			zap.Error(err))
		h.sendError(w, "Failed to get language preferences", http.StatusInternalServerError)
		return
	}

	autoTranslate, _ := h.localizationService.ShouldAutoTranslate(r.Context(), userID, contentType)
	autoDownload, _ := h.localizationService.ShouldAutoDownload(r.Context(), userID, contentType)

	h.sendSuccess(w, map[string]interface{}{
		"languages":      languages,
		"auto_translate": autoTranslate,
		"auto_download":  autoDownload,
	})
}

func (h *LocalizationHandlers) GetLocalizationStats(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Getting localization statistics")

	stats, err := h.localizationService.GetLocalizationStats(r.Context())
	if err != nil {
		h.logger.Error("Failed to get localization stats", zap.Error(err))
		h.sendError(w, "Failed to get localization statistics", http.StatusInternalServerError)
		return
	}

	h.sendSuccess(w, stats)
}

// JSON Configuration Handlers

func (h *LocalizationHandlers) ExportConfiguration(w http.ResponseWriter, r *http.Request) {
	userID := h.getUserID(r)
	if userID == 0 {
		h.sendError(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	var req struct {
		ConfigType  string   `json:"config_type"`
		Description string   `json:"description"`
		Tags        []string `json:"tags"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ConfigType == "" {
		req.ConfigType = "full"
	}

	config, err := h.localizationService.ExportConfiguration(r.Context(), userID, req.ConfigType, req.Description, req.Tags)
	if err != nil {
		h.logger.Error("Failed to export configuration",
			zap.Int64("user_id", userID),
			zap.String("config_type", req.ConfigType),
			zap.Error(err))
		h.sendError(w, "Failed to export configuration", http.StatusInternalServerError)
		return
	}

	h.sendSuccess(w, config)
}

func (h *LocalizationHandlers) ImportConfiguration(w http.ResponseWriter, r *http.Request) {
	userID := h.getUserID(r)
	if userID == 0 {
		h.sendError(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	var req struct {
		ConfigJSON string          `json:"config_json"`
		Options    map[string]bool `json:"options"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ConfigJSON == "" {
		h.sendError(w, "Configuration JSON is required", http.StatusBadRequest)
		return
	}

	if req.Options == nil {
		req.Options = map[string]bool{
			"overwrite_existing": false,
			"backup_current":     true,
			"validate_only":      false,
		}
	}

	result, err := h.localizationService.ImportConfiguration(r.Context(), userID, req.ConfigJSON, req.Options)
	if err != nil {
		h.logger.Error("Failed to import configuration",
			zap.Int64("user_id", userID),
			zap.Error(err))
		h.sendError(w, "Failed to import configuration", http.StatusBadRequest)
		return
	}

	h.sendSuccess(w, result)
}

func (h *LocalizationHandlers) ValidateConfiguration(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConfigJSON string `json:"config_json"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ConfigJSON == "" {
		h.sendError(w, "Configuration JSON is required", http.StatusBadRequest)
		return
	}

	validation, err := h.localizationService.ValidateConfigurationJSON(r.Context(), req.ConfigJSON)
	if err != nil {
		h.logger.Error("Failed to validate configuration", zap.Error(err))
		h.sendError(w, "Failed to validate configuration", http.StatusInternalServerError)
		return
	}

	h.sendSuccess(w, validation)
}

func (h *LocalizationHandlers) EditConfiguration(w http.ResponseWriter, r *http.Request) {
	userID := h.getUserID(r)
	if userID == 0 {
		h.sendError(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	var req struct {
		ConfigJSON string                 `json:"config_json"`
		Edits      map[string]interface{} `json:"edits"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.ConfigJSON == "" {
		h.sendError(w, "Configuration JSON is required", http.StatusBadRequest)
		return
	}

	if req.Edits == nil {
		h.sendError(w, "Edits are required", http.StatusBadRequest)
		return
	}

	editedConfig, err := h.localizationService.EditConfiguration(r.Context(), userID, req.ConfigJSON, req.Edits)
	if err != nil {
		h.logger.Error("Failed to edit configuration",
			zap.Int64("user_id", userID),
			zap.Error(err))
		h.sendError(w, "Failed to edit configuration", http.StatusBadRequest)
		return
	}

	h.sendSuccess(w, map[string]interface{}{
		"edited_config": editedConfig,
		"message":       "Configuration edited successfully",
	})
}

func (h *LocalizationHandlers) GetConfigurationTemplates(w http.ResponseWriter, r *http.Request) {
	h.logger.Debug("Getting configuration templates")

	// Get all template types
	templateTypes := []string{"localization", "media", "playlists", "full"}
	templates := make(map[string]interface{})

	for _, templateType := range templateTypes {
		template, err := h.localizationService.GetConfigurationTemplate(r.Context(), templateType)
		if err != nil {
			h.logger.Warn("Failed to get template",
				zap.String("type", templateType),
				zap.Error(err))
			continue
		}
		templates[templateType] = template
	}

	h.sendSuccess(w, map[string]interface{}{
		"templates": templates,
		"count":     len(templates),
	})
}

// Content-specific Handlers

func (h *LocalizationHandlers) DetectLanguage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserAgent      string `json:"user_agent"`
		AcceptLanguage string `json:"accept_language"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.UserAgent == "" {
		req.UserAgent = r.Header.Get("User-Agent")
	}
	if req.AcceptLanguage == "" {
		req.AcceptLanguage = r.Header.Get("Accept-Language")
	}

	detectedLanguage := h.localizationService.DetectUserLanguage(r.Context(), req.UserAgent, req.AcceptLanguage)

	h.sendSuccess(w, map[string]interface{}{
		"detected_language": detectedLanguage,
		"confidence":        1.0,
	})
}

func (h *LocalizationHandlers) CheckLanguageSupport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		LanguageCode string `json:"language_code"`
		ContentType  string `json:"content_type"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.LanguageCode == "" {
		h.sendError(w, "Language code is required", http.StatusBadRequest)
		return
	}

	if req.ContentType == "" {
		h.sendError(w, "Content type is required", http.StatusBadRequest)
		return
	}

	supported := h.localizationService.IsLanguageSupported(r.Context(), req.LanguageCode, req.ContentType)

	profile, err := h.localizationService.GetLanguageProfile(r.Context(), req.LanguageCode)
	var qualityRating float64
	var providers []string

	if err == nil {
		qualityRating = profile.QualityRating
		providers = profile.SupportedBy
	}

	h.sendSuccess(w, map[string]interface{}{
		"supported":      supported,
		"language_code":  req.LanguageCode,
		"content_type":   req.ContentType,
		"quality_rating": qualityRating,
		"providers":      providers,
	})
}

func (h *LocalizationHandlers) FormatDateTime(w http.ResponseWriter, r *http.Request) {
	userID := h.getUserID(r)
	if userID == 0 {
		h.sendError(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	var req struct {
		Timestamp string `json:"timestamp"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.sendError(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Timestamp == "" {
		h.sendError(w, "Timestamp is required", http.StatusBadRequest)
		return
	}

	timestamp, err := parseTimestamp(req.Timestamp)
	if err != nil {
		h.sendError(w, "Invalid timestamp format", http.StatusBadRequest)
		return
	}

	formatted, err := h.localizationService.FormatDateTimeForUser(r.Context(), userID, timestamp)
	if err != nil {
		h.logger.Error("Failed to format datetime", zap.Error(err))
		h.sendError(w, "Failed to format datetime", http.StatusInternalServerError)
		return
	}

	h.sendSuccess(w, map[string]interface{}{
		"original":  req.Timestamp,
		"formatted": formatted,
		"timezone":  timestamp.Location().String(),
	})
}

// Helper Methods

func (h *LocalizationHandlers) sendSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(APIResponse{
		Success: true,
		Data:    data,
	})
}

func (h *LocalizationHandlers) sendError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(APIResponse{
		Success: false,
		Error:   message,
	})
}

func (h *LocalizationHandlers) getUserID(r *http.Request) int64 {
	// Get user ID from authenticated context (set by auth middleware)
	// Never trust client-provided headers for user identity
	if userID, ok := r.Context().Value("user_id").(int64); ok {
		return userID
	}
	if userID, ok := r.Context().Value("user_id").(int); ok {
		return int64(userID)
	}
	return 0
}

func (h *LocalizationHandlers) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
		if allowedOrigins == "" {
			allowedOrigins = "http://localhost:5173,http://localhost:3000"
		}
		for _, o := range strings.Split(allowedOrigins, ",") {
			if strings.TrimSpace(o) == origin && origin != "" {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				break
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Accept-Language")
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func parseTimestamp(timestampStr string) (time.Time, error) {
	formats := []string{
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"1136239445", // Unix timestamp
	}

	for _, format := range formats {
		if timestamp, err := time.Parse(format, timestampStr); err == nil {
			return timestamp, nil
		}
	}

	// Try parsing as Unix timestamp
	if unix, err := strconv.ParseInt(timestampStr, 10, 64); err == nil {
		return time.Unix(unix, 0), nil
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", timestampStr)
}
