package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

// TranslationService handles AI-powered text translation
type TranslationService struct {
	logger     *zap.Logger
	httpClient *http.Client
	providers  map[string]TranslationProvider
	cache      map[string]*TranslationResult // Simple in-memory cache
}

// TranslationProvider represents a translation API provider
type TranslationProvider interface {
	Translate(ctx context.Context, request *TranslationRequest) (*TranslationResult, error)
	GetName() string
	GetSupportedLanguages() []string
	IsAvailable() bool
}

// TranslationRequest represents a translation request
type TranslationRequest struct {
	Text           string            `json:"text"`
	SourceLanguage string            `json:"source_language"`
	TargetLanguage string            `json:"target_language"`
	Context        string            `json:"context,omitempty"` // "lyrics", "subtitle", "general"
	Metadata       map[string]string `json:"metadata,omitempty"`
}

// TranslationResult represents a translation result
type TranslationResult struct {
	OriginalText     string    `json:"original_text"`
	TranslatedText   string    `json:"translated_text"`
	SourceLanguage   string    `json:"source_language"`
	TargetLanguage   string    `json:"target_language"`
	Provider         string    `json:"provider"`
	Confidence       float64   `json:"confidence"`
	DetectedLanguage *string   `json:"detected_language,omitempty"`
	Alternatives     []string  `json:"alternatives,omitempty"`
	ProcessingTime   float64   `json:"processing_time"` // Milliseconds
	CachedAt         time.Time `json:"cached_at"`
}

// BatchTranslationRequest represents a batch translation request
type BatchTranslationRequest struct {
	Texts          []string `json:"texts"`
	SourceLanguage string   `json:"source_language"`
	TargetLanguage string   `json:"target_language"`
	Context        string   `json:"context,omitempty"`
	PreserveFormat bool     `json:"preserve_format"`
}

// BatchTranslationResult represents a batch translation result
type BatchTranslationResult struct {
	Results      []TranslationResult `json:"results"`
	TotalTime    float64             `json:"total_time"`
	SuccessCount int                 `json:"success_count"`
	Provider     string              `json:"provider"`
}

// LanguageDetectionRequest represents a language detection request
type LanguageDetectionRequest struct {
	Text string `json:"text"`
}

// LanguageDetectionResult represents a language detection result
type LanguageDetectionResult struct {
	Language   string  `json:"language"`
	Code       string  `json:"code"`
	Confidence float64 `json:"confidence"`
	Provider   string  `json:"provider"`
}

// SupportedLanguage represents a supported language
type SupportedLanguage struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	NativeName string `json:"native_name"`
	Flag       string `json:"flag"`      // Unicode flag emoji
	Direction  string `json:"direction"` // "ltr" or "rtl"
	IsPopular  bool   `json:"is_popular"`
}

// NewTranslationService creates a new translation service
func NewTranslationService(logger *zap.Logger) *TranslationService {
	service := &TranslationService{
		logger:     logger,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		providers:  make(map[string]TranslationProvider),
		cache:      make(map[string]*TranslationResult),
	}

	// Initialize providers
	service.initializeProviders()

	return service
}

// TranslateText translates text using the best available provider
func (s *TranslationService) TranslateText(ctx context.Context, request TranslationRequest) (*TranslationResult, error) {
	s.logger.Debug("Translating text",
		zap.String("source_lang", request.SourceLanguage),
		zap.String("target_lang", request.TargetLanguage),
		zap.String("context", request.Context))

	// Check cache first
	cacheKey := s.generateCacheKey(&request)
	if cached, exists := s.cache[cacheKey]; exists {
		s.logger.Debug("Using cached translation")
		return cached, nil
	}

	// Get available providers in priority order
	providers := s.getAvailableProviders()
	if len(providers) == 0 {
		return nil, fmt.Errorf("no translation providers available")
	}

	var lastError error
	startTime := time.Now()

	// Try providers in order until one succeeds
	for _, provider := range providers {
		result, err := provider.Translate(ctx, &request)
		if err != nil {
			s.logger.Warn("Provider failed",
				zap.String("provider", provider.GetName()),
				zap.Error(err))
			lastError = err
			continue
		}

		// Calculate processing time
		result.ProcessingTime = float64(time.Since(startTime).Nanoseconds()) / 1e6
		result.CachedAt = time.Now()

		// Cache the result
		s.cache[cacheKey] = result

		s.logger.Info("Translation completed",
			zap.String("provider", result.Provider),
			zap.Float64("confidence", result.Confidence),
			zap.Float64("processing_time_ms", result.ProcessingTime))

		return result, nil
	}

	return nil, fmt.Errorf("all translation providers failed, last error: %w", lastError)
}

// TranslateBatch translates multiple texts in a single request
func (s *TranslationService) TranslateBatch(ctx context.Context, request *BatchTranslationRequest) (*BatchTranslationResult, error) {
	s.logger.Info("Translating batch",
		zap.Int("text_count", len(request.Texts)),
		zap.String("source_lang", request.SourceLanguage),
		zap.String("target_lang", request.TargetLanguage))

	startTime := time.Now()
	var results []TranslationResult
	successCount := 0

	// Translate each text
	for _, text := range request.Texts {
		translationRequest := TranslationRequest{
			Text:           text,
			SourceLanguage: request.SourceLanguage,
			TargetLanguage: request.TargetLanguage,
			Context:        request.Context,
		}

		result, err := s.TranslateText(ctx, translationRequest)
		if err != nil {
			s.logger.Warn("Failed to translate text in batch", zap.Error(err))
			// Add empty result to maintain order
			results = append(results, TranslationResult{
				OriginalText:   text,
				TranslatedText: text, // Fallback to original
				SourceLanguage: request.SourceLanguage,
				TargetLanguage: request.TargetLanguage,
				Provider:       "fallback",
				Confidence:     0.0,
			})
		} else {
			results = append(results, *result)
			successCount++
		}
	}

	totalTime := float64(time.Since(startTime).Nanoseconds()) / 1e6

	batchResult := &BatchTranslationResult{
		Results:      results,
		TotalTime:    totalTime,
		SuccessCount: successCount,
		Provider:     "batch",
	}

	return batchResult, nil
}

// DetectLanguage detects the language of the given text
func (s *TranslationService) DetectLanguage(ctx context.Context, request *LanguageDetectionRequest) (*LanguageDetectionResult, error) {
	s.logger.Debug("Detecting language", zap.String("text_preview", s.getTextPreview(request.Text)))

	// Get available providers
	providers := s.getAvailableProviders()
	if len(providers) == 0 {
		return nil, fmt.Errorf("no language detection providers available")
	}

	// Use first available provider for language detection
	// In practice, you might want to try multiple providers and compare results
	provider := providers[0]

	// For now, use a simple heuristic approach
	detectedLang := s.simpleLanguageDetection(request.Text)

	result := &LanguageDetectionResult{
		Language:   getLanguageName(detectedLang),
		Code:       detectedLang,
		Confidence: 0.8, // Mock confidence
		Provider:   provider.GetName(),
	}

	return result, nil
}

// GetSupportedLanguages returns all supported languages
func (s *TranslationService) GetSupportedLanguages() []SupportedLanguage {
	return []SupportedLanguage{
		{Code: "en", Name: "English", NativeName: "English", Flag: "🇺🇸", Direction: "ltr", IsPopular: true},
		{Code: "es", Name: "Spanish", NativeName: "Español", Flag: "🇪🇸", Direction: "ltr", IsPopular: true},
		{Code: "fr", Name: "French", NativeName: "Français", Flag: "🇫🇷", Direction: "ltr", IsPopular: true},
		{Code: "de", Name: "German", NativeName: "Deutsch", Flag: "🇩🇪", Direction: "ltr", IsPopular: true},
		{Code: "it", Name: "Italian", NativeName: "Italiano", Flag: "🇮🇹", Direction: "ltr", IsPopular: true},
		{Code: "pt", Name: "Portuguese", NativeName: "Português", Flag: "🇵🇹", Direction: "ltr", IsPopular: true},
		{Code: "ru", Name: "Russian", NativeName: "Русский", Flag: "🇷🇺", Direction: "ltr", IsPopular: true},
		{Code: "ja", Name: "Japanese", NativeName: "日本語", Flag: "🇯🇵", Direction: "ltr", IsPopular: true},
		{Code: "ko", Name: "Korean", NativeName: "한국어", Flag: "🇰🇷", Direction: "ltr", IsPopular: true},
		{Code: "zh", Name: "Chinese", NativeName: "中文", Flag: "🇨🇳", Direction: "ltr", IsPopular: true},
		{Code: "ar", Name: "Arabic", NativeName: "العربية", Flag: "🇸🇦", Direction: "rtl", IsPopular: true},
		{Code: "hi", Name: "Hindi", NativeName: "हिन्दी", Flag: "🇮🇳", Direction: "ltr", IsPopular: true},
		{Code: "th", Name: "Thai", NativeName: "ไทย", Flag: "🇹🇭", Direction: "ltr", IsPopular: false},
		{Code: "vi", Name: "Vietnamese", NativeName: "Tiếng Việt", Flag: "🇻🇳", Direction: "ltr", IsPopular: false},
		{Code: "tr", Name: "Turkish", NativeName: "Türkçe", Flag: "🇹🇷", Direction: "ltr", IsPopular: false},
		{Code: "pl", Name: "Polish", NativeName: "Polski", Flag: "🇵🇱", Direction: "ltr", IsPopular: false},
		{Code: "nl", Name: "Dutch", NativeName: "Nederlands", Flag: "🇳🇱", Direction: "ltr", IsPopular: false},
		{Code: "sv", Name: "Swedish", NativeName: "Svenska", Flag: "🇸🇪", Direction: "ltr", IsPopular: false},
		{Code: "da", Name: "Danish", NativeName: "Dansk", Flag: "🇩🇰", Direction: "ltr", IsPopular: false},
		{Code: "no", Name: "Norwegian", NativeName: "Norsk", Flag: "🇳🇴", Direction: "ltr", IsPopular: false},
	}
}

// Initialize providers
func (s *TranslationService) initializeProviders() {
	// Initialize free translation providers
	s.providers["google_translate_free"] = NewGoogleTranslateFreeProvider(s.httpClient, s.logger)
	s.providers["libre_translate"] = NewLibreTranslateProvider(s.httpClient, s.logger)
	s.providers["mymemory"] = NewMyMemoryProvider(s.httpClient, s.logger)

	// Note: In production, you would also initialize paid providers like:
	// s.providers["google_translate_api"] = NewGoogleTranslateAPIProvider(apiKey, s.httpClient, s.logger)
	// s.providers["azure_translator"] = NewAzureTranslatorProvider(apiKey, s.httpClient, s.logger)
	// s.providers["aws_translate"] = NewAWSTranslateProvider(credentials, s.httpClient, s.logger)
}

// Get available providers in priority order
func (s *TranslationService) getAvailableProviders() []TranslationProvider {
	var available []TranslationProvider

	// Priority order: paid providers first, then free providers
	providerNames := []string{
		"google_translate_api",  // Paid (if configured)
		"azure_translator",      // Paid (if configured)
		"aws_translate",         // Paid (if configured)
		"google_translate_free", // Free
		"libre_translate",       // Free
		"mymemory",              // Free
	}

	for _, name := range providerNames {
		if provider, exists := s.providers[name]; exists && provider.IsAvailable() {
			available = append(available, provider)
		}
	}

	return available
}

// Generate cache key for translation request
func (s *TranslationService) generateCacheKey(request *TranslationRequest) string {
	return fmt.Sprintf("%s_%s_%s_%s",
		request.SourceLanguage,
		request.TargetLanguage,
		request.Context,
		s.hashText(request.Text))
}

// Simple text hashing for cache keys
func (s *TranslationService) hashText(text string) string {
	if len(text) > 50 {
		return fmt.Sprintf("%s...%s_%d", text[:20], text[len(text)-20:], len(text))
	}
	return text
}

// Simple language detection (in practice, use a proper language detection library)
func (s *TranslationService) simpleLanguageDetection(text string) string {
	text = strings.ToLower(text)

	// Simple keyword-based detection
	if strings.Contains(text, "the ") || strings.Contains(text, " and ") || strings.Contains(text, " is ") {
		return "en"
	}
	if strings.Contains(text, " el ") || strings.Contains(text, " la ") || strings.Contains(text, " es ") {
		return "es"
	}
	if strings.Contains(text, " le ") || strings.Contains(text, " la ") || strings.Contains(text, " est ") {
		return "fr"
	}
	if strings.Contains(text, " der ") || strings.Contains(text, " die ") || strings.Contains(text, " ist ") {
		return "de"
	}

	// Default to English
	return "en"
}

// Get text preview for logging
func (s *TranslationService) getTextPreview(text string) string {
	if len(text) > 100 {
		return text[:100] + "..."
	}
	return text
}

// Helper function to get language name from code
func getLanguageName(code string) string {
	languages := map[string]string{
		"en": "English",
		"es": "Spanish",
		"fr": "French",
		"de": "German",
		"it": "Italian",
		"pt": "Portuguese",
		"ru": "Russian",
		"ja": "Japanese",
		"ko": "Korean",
		"zh": "Chinese",
		"ar": "Arabic",
		"hi": "Hindi",
		"th": "Thai",
		"vi": "Vietnamese",
		"tr": "Turkish",
		"pl": "Polish",
		"nl": "Dutch",
		"sv": "Swedish",
		"da": "Danish",
		"no": "Norwegian",
	}

	if name, exists := languages[code]; exists {
		return name
	}
	return code
}

// Provider implementations would be in separate files
// Here are the interfaces they would implement:

// GoogleTranslateFreeProvider implements free Google Translate
type GoogleTranslateFreeProvider struct {
	httpClient *http.Client
	logger     *zap.Logger
	baseURL    string
}

func NewGoogleTranslateFreeProvider(httpClient *http.Client, logger *zap.Logger) *GoogleTranslateFreeProvider {
	return &GoogleTranslateFreeProvider{
		httpClient: httpClient,
		logger:     logger,
		baseURL:    "https://translate.googleapis.com/translate_a/single",
	}
}

func (p *GoogleTranslateFreeProvider) Translate(ctx context.Context, request *TranslationRequest) (*TranslationResult, error) {
	return nil, fmt.Errorf("%w: GoogleTranslateFree", ErrTranslationProviderNotImplemented)
}

func (p *GoogleTranslateFreeProvider) GetName() string {
	return "google_translate_free"
}

func (p *GoogleTranslateFreeProvider) GetSupportedLanguages() []string {
	return []string{"en", "es", "fr", "de", "it", "pt", "ru", "ja", "ko", "zh", "ar", "hi"}
}

func (p *GoogleTranslateFreeProvider) IsAvailable() bool {
	return false // no HTTP client implemented
}

// LibreTranslateProvider implements LibreTranslate
type LibreTranslateProvider struct {
	httpClient *http.Client
	logger     *zap.Logger
	baseURL    string
}

func NewLibreTranslateProvider(httpClient *http.Client, logger *zap.Logger) *LibreTranslateProvider {
	return &LibreTranslateProvider{
		httpClient: httpClient,
		logger:     logger,
		baseURL:    "https://libretranslate.de/translate",
	}
}

func (p *LibreTranslateProvider) Translate(ctx context.Context, request *TranslationRequest) (*TranslationResult, error) {
	return nil, fmt.Errorf("%w: LibreTranslate", ErrTranslationProviderNotImplemented)
}

func (p *LibreTranslateProvider) GetName() string {
	return "libre_translate"
}

func (p *LibreTranslateProvider) GetSupportedLanguages() []string {
	return []string{"en", "es", "fr", "de", "it", "pt", "ru", "ja", "zh", "ar"}
}

func (p *LibreTranslateProvider) IsAvailable() bool {
	return false // no HTTP client implemented
}

// MyMemoryProvider implements MyMemory translation
type MyMemoryProvider struct {
	httpClient *http.Client
	logger     *zap.Logger
	baseURL    string
}

func NewMyMemoryProvider(httpClient *http.Client, logger *zap.Logger) *MyMemoryProvider {
	return &MyMemoryProvider{
		httpClient: httpClient,
		logger:     logger,
		baseURL:    "https://api.mymemory.translated.net/get",
	}
}

func (p *MyMemoryProvider) Translate(ctx context.Context, request *TranslationRequest) (*TranslationResult, error) {
	return nil, fmt.Errorf("%w: MyMemory", ErrTranslationProviderNotImplemented)
}

func (p *MyMemoryProvider) GetName() string {
	return "mymemory"
}

func (p *MyMemoryProvider) GetSupportedLanguages() []string {
	return []string{"en", "es", "fr", "de", "it", "pt", "ru"}
}

func (p *MyMemoryProvider) IsAvailable() bool {
	return false // no HTTP client implemented
}

// ErrTranslationProviderNotImplemented marks built-in translation providers
// that have no real API client. They replaced fabricated "[GT] text" output.
var ErrTranslationProviderNotImplemented = errors.New("translation provider client not implemented")
