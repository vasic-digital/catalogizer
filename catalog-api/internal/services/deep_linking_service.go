package services

import (
	"context"
	"errors"
	"fmt"
	neturl "net/url"
	"strings"
	"time"

	"catalogizer/internal/logging"
	"catalogizer/internal/models"
)

type DeepLinkingService struct {
	baseURL    string
	apiVersion string
}

type DeepLinkRequest struct {
	MediaID       string                `json:"media_id"`
	MediaMetadata *models.MediaMetadata `json:"media_metadata"`
	TargetApp     string                `json:"target_app,omitempty"`
	Action        string                `json:"action"` // detail, play, download, edit
	Context       *LinkContext          `json:"context,omitempty"`
}

type LinkContext struct {
	UserID       string            `json:"user_id,omitempty"`
	DeviceID     string            `json:"device_id,omitempty"`
	SessionID    string            `json:"session_id,omitempty"`
	ReferrerPage string            `json:"referrer_page,omitempty"`
	Platform     string            `json:"platform,omitempty"` // web, android, ios, desktop
	AppVersion   string            `json:"app_version,omitempty"`
	Preferences  map[string]string `json:"preferences,omitempty"`
	UTMParams    *UTMParameters    `json:"utm_params,omitempty"`
}

type UTMParameters struct {
	Source   string `json:"utm_source,omitempty"`
	Medium   string `json:"utm_medium,omitempty"`
	Campaign string `json:"utm_campaign,omitempty"`
	Term     string `json:"utm_term,omitempty"`
	Content  string `json:"utm_content,omitempty"`
}

type DeepLinkResponse struct {
	Links         map[string]*DeepLink `json:"links"` // Platform -> DeepLink
	UniversalLink string               `json:"universal_link"`
	QRCode        string               `json:"qr_code,omitempty"`
	ShareableLink string               `json:"shareable_link"`
	ExpiresAt     *time.Time           `json:"expires_at,omitempty"`
	TrackingID    string               `json:"tracking_id"`
	SupportedApps []string             `json:"supported_apps"`
	FallbackURL   string               `json:"fallback_url"`
}

type DeepLink struct {
	URL          string                 `json:"url"`
	Scheme       string                 `json:"scheme"`
	Package      string                 `json:"package,omitempty"`   // Android package name
	BundleID     string                 `json:"bundle_id,omitempty"` // iOS bundle ID
	StoreURL     string                 `json:"store_url,omitempty"` // App store download link
	Parameters   map[string]string      `json:"parameters"`
	Headers      map[string]string      `json:"headers,omitempty"`
	PostData     map[string]interface{} `json:"post_data,omitempty"`
	RequiresAuth bool                   `json:"requires_auth"`
	AppVersion   string                 `json:"min_app_version,omitempty"`
	Features     []string               `json:"required_features,omitempty"`
}

type LinkTrackingEvent struct {
	TrackingID   string                 `json:"tracking_id"`
	EventType    string                 `json:"event_type"` // click, open, fallback, error
	Platform     string                 `json:"platform"`
	UserAgent    string                 `json:"user_agent,omitempty"`
	IPAddress    string                 `json:"ip_address,omitempty"`
	Timestamp    time.Time              `json:"timestamp"`
	Success      bool                   `json:"success"`
	ErrorMessage string                 `json:"error_message,omitempty"`
	AppOpened    bool                   `json:"app_opened"`
	FallbackUsed bool                   `json:"fallback_used"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

type AppConfiguration struct {
	AppID         string            `json:"app_id"`
	Name          string            `json:"name"`
	Platforms     []string          `json:"platforms"`
	Schemes       map[string]string `json:"schemes"`      // Platform -> URL scheme
	Packages      map[string]string `json:"packages"`     // Platform -> package/bundle ID
	StoreURLs     map[string]string `json:"store_urls"`   // Platform -> store download URL
	MinVersions   map[string]string `json:"min_versions"` // Platform -> minimum version
	Features      []string          `json:"supported_features"`
	PreferredApps bool              `json:"is_preferred"`
	Active        bool              `json:"is_active"`
}

func NewDeepLinkingService(baseURL, apiVersion string) *DeepLinkingService {
	return &DeepLinkingService{
		baseURL:    baseURL,
		apiVersion: apiVersion,
	}
}

func (dls *DeepLinkingService) GenerateDeepLinks(ctx context.Context, req *DeepLinkRequest) (*DeepLinkResponse, error) {
	// Validate request
	if req.MediaID == "" {
		return nil, fmt.Errorf("media ID is required")
	}

	trackingID := dls.generateTrackingID()

	response := &DeepLinkResponse{
		Links:         make(map[string]*DeepLink),
		TrackingID:    trackingID,
		SupportedApps: dls.getSupportedApps(),
		FallbackURL:   dls.generateFallbackURL(req),
	}

	// Generate universal link
	response.UniversalLink = dls.generateUniversalLink(req, trackingID)
	response.ShareableLink = response.UniversalLink

	// Generate platform-specific links
	platforms := []string{"web", "android", "ios", "desktop"}

	for _, platform := range platforms {
		link, err := dls.generatePlatformLink(ctx, req, platform, trackingID)
		if err != nil {
			continue // Skip platforms that fail
		}
		response.Links[platform] = link
	}

	// Generate QR code for easy sharing
	response.QRCode = dls.generateQRCodeURL(response.UniversalLink)

	// Set expiration for temporary links
	if req.Action == "play" || req.Action == "download" {
		expiresAt := time.Now().Add(24 * time.Hour)
		response.ExpiresAt = &expiresAt
	}

	return response, nil
}

func (dls *DeepLinkingService) generatePlatformLink(ctx context.Context, req *DeepLinkRequest, platform, trackingID string) (*DeepLink, error) {
	switch platform {
	case "web":
		return dls.generateWebLink(req, trackingID)
	case "android":
		return dls.generateAndroidLink(req, trackingID)
	case "ios":
		return dls.generateIOSLink(req, trackingID)
	case "desktop":
		return dls.generateDesktopLink(req, trackingID)
	default:
		return nil, fmt.Errorf("unsupported platform: %s", platform)
	}
}

func (dls *DeepLinkingService) generateWebLink(req *DeepLinkRequest, trackingID string) (*DeepLink, error) {
	baseURL := dls.baseURL
	if baseURL == "" {
		baseURL = "https://catalogizer.app"
	}

	var path string
	parameters := make(map[string]string)

	switch req.Action {
	case "detail":
		path = fmt.Sprintf("/detail/%s", req.MediaID)
	case "play":
		path = fmt.Sprintf("/play/%s", req.MediaID)
		parameters["autoplay"] = "true"
	case "download":
		path = fmt.Sprintf("/download/%s", req.MediaID)
	case "edit":
		path = fmt.Sprintf("/edit/%s", req.MediaID)
	default:
		path = fmt.Sprintf("/detail/%s", req.MediaID)
	}

	// Add context parameters
	if req.Context != nil {
		if req.Context.UserID != "" {
			parameters["user_id"] = req.Context.UserID
		}
		if req.Context.SessionID != "" {
			parameters["session_id"] = req.Context.SessionID
		}
		if req.Context.ReferrerPage != "" {
			parameters["ref"] = req.Context.ReferrerPage
		}

		// Add UTM parameters
		if req.Context.UTMParams != nil {
			if req.Context.UTMParams.Source != "" {
				parameters["utm_source"] = req.Context.UTMParams.Source
			}
			if req.Context.UTMParams.Medium != "" {
				parameters["utm_medium"] = req.Context.UTMParams.Medium
			}
			if req.Context.UTMParams.Campaign != "" {
				parameters["utm_campaign"] = req.Context.UTMParams.Campaign
			}
			if req.Context.UTMParams.Term != "" {
				parameters["utm_term"] = req.Context.UTMParams.Term
			}
			if req.Context.UTMParams.Content != "" {
				parameters["utm_content"] = req.Context.UTMParams.Content
			}
		}
	}

	// Add tracking
	parameters["track"] = trackingID

	// Build URL with parameters
	u, err := neturl.Parse(baseURL + path)
	if err != nil {
		return nil, err
	}

	query := u.Query()
	for key, value := range parameters {
		query.Set(key, value)
	}
	u.RawQuery = query.Encode()

	return &DeepLink{
		URL:          u.String(),
		Scheme:       "https",
		Parameters:   parameters,
		RequiresAuth: req.Action == "edit" || req.Action == "download",
	}, nil
}

func (dls *DeepLinkingService) generateAndroidLink(req *DeepLinkRequest, trackingID string) (*DeepLink, error) {
	scheme := "catalogizer"
	packageName := "com.catalogizer.app"

	var path string
	parameters := make(map[string]string)

	switch req.Action {
	case "detail":
		path = fmt.Sprintf("detail/%s", req.MediaID)
	case "play":
		path = fmt.Sprintf("play/%s", req.MediaID)
		parameters["autoplay"] = "true"
	case "download":
		path = fmt.Sprintf("download/%s", req.MediaID)
	case "edit":
		path = fmt.Sprintf("edit/%s", req.MediaID)
	default:
		path = fmt.Sprintf("detail/%s", req.MediaID)
	}

	// Add context
	if req.Context != nil {
		if req.Context.UserID != "" {
			parameters["user_id"] = req.Context.UserID
		}
		if req.Context.SessionID != "" {
			parameters["session_id"] = req.Context.SessionID
		}
	}

	// Add tracking
	parameters["track"] = trackingID

	// Build Android deep link
	url := fmt.Sprintf("%s://%s", scheme, path)
	if len(parameters) > 0 {
		query := make([]string, 0, len(parameters))
		for key, value := range parameters {
			query = append(query, fmt.Sprintf("%s=%s", key, neturl.QueryEscape(value)))
		}
		url += "?" + strings.Join(query, "&")
	}

	return &DeepLink{
		URL:          url,
		Scheme:       scheme,
		Package:      packageName,
		StoreURL:     "https://play.google.com/store/apps/details?id=" + packageName,
		Parameters:   parameters,
		RequiresAuth: req.Action == "edit" || req.Action == "download",
		AppVersion:   "1.0.0",
		Features:     dls.getRequiredFeatures(req),
	}, nil
}

func (dls *DeepLinkingService) generateIOSLink(req *DeepLinkRequest, trackingID string) (*DeepLink, error) {
	scheme := "catalogizer"
	bundleID := "com.catalogizer.app"

	var path string
	parameters := make(map[string]string)

	switch req.Action {
	case "detail":
		path = fmt.Sprintf("detail/%s", req.MediaID)
	case "play":
		path = fmt.Sprintf("play/%s", req.MediaID)
		parameters["autoplay"] = "true"
	case "download":
		path = fmt.Sprintf("download/%s", req.MediaID)
	case "edit":
		path = fmt.Sprintf("edit/%s", req.MediaID)
	default:
		path = fmt.Sprintf("detail/%s", req.MediaID)
	}

	// Add context
	if req.Context != nil {
		if req.Context.UserID != "" {
			parameters["user_id"] = req.Context.UserID
		}
		if req.Context.SessionID != "" {
			parameters["session_id"] = req.Context.SessionID
		}
	}

	// Add tracking
	parameters["track"] = trackingID

	// Build iOS deep link
	url := fmt.Sprintf("%s://%s", scheme, path)
	if len(parameters) > 0 {
		query := make([]string, 0, len(parameters))
		for key, value := range parameters {
			query = append(query, fmt.Sprintf("%s=%s", key, neturl.QueryEscape(value)))
		}
		url += "?" + strings.Join(query, "&")
	}

	return &DeepLink{
		URL:          url,
		Scheme:       scheme,
		BundleID:     bundleID,
		StoreURL:     "https://apps.apple.com/app/id123456789", // Would be actual App Store ID
		Parameters:   parameters,
		RequiresAuth: req.Action == "edit" || req.Action == "download",
		AppVersion:   "1.0.0",
		Features:     dls.getRequiredFeatures(req),
	}, nil
}

func (dls *DeepLinkingService) generateDesktopLink(req *DeepLinkRequest, trackingID string) (*DeepLink, error) {
	scheme := "catalogizer-desktop"

	var path string
	parameters := make(map[string]string)

	switch req.Action {
	case "detail":
		path = fmt.Sprintf("detail/%s", req.MediaID)
	case "play":
		path = fmt.Sprintf("play/%s", req.MediaID)
		parameters["autoplay"] = "true"
	case "download":
		path = fmt.Sprintf("download/%s", req.MediaID)
	case "edit":
		path = fmt.Sprintf("edit/%s", req.MediaID)
	default:
		path = fmt.Sprintf("detail/%s", req.MediaID)
	}

	// Add context
	if req.Context != nil {
		if req.Context.UserID != "" {
			parameters["user_id"] = req.Context.UserID
		}
		if req.Context.SessionID != "" {
			parameters["session_id"] = req.Context.SessionID
		}
	}

	// Add tracking
	parameters["track"] = trackingID

	// Build desktop deep link
	url := fmt.Sprintf("%s://%s", scheme, path)
	if len(parameters) > 0 {
		query := make([]string, 0, len(parameters))
		for key, value := range parameters {
			query = append(query, fmt.Sprintf("%s=%s", key, neturl.QueryEscape(value)))
		}
		url += "?" + strings.Join(query, "&")
	}

	return &DeepLink{
		URL:          url,
		Scheme:       scheme,
		StoreURL:     "https://github.com/catalogizer/desktop/releases", // GitHub releases
		Parameters:   parameters,
		RequiresAuth: req.Action == "edit" || req.Action == "download",
		AppVersion:   "1.0.0",
		Features:     dls.getRequiredFeatures(req),
	}, nil
}

func (dls *DeepLinkingService) generateUniversalLink(req *DeepLinkRequest, trackingID string) string {
	baseURL := dls.baseURL
	if baseURL == "" {
		baseURL = "https://catalogizer.app"
	}

	switch req.Action {
	case "detail":
		return fmt.Sprintf("%s/link/detail/%s?track=%s", baseURL, req.MediaID, trackingID)
	case "play":
		return fmt.Sprintf("%s/link/play/%s?track=%s", baseURL, req.MediaID, trackingID)
	case "download":
		return fmt.Sprintf("%s/link/download/%s?track=%s", baseURL, req.MediaID, trackingID)
	case "edit":
		return fmt.Sprintf("%s/link/edit/%s?track=%s", baseURL, req.MediaID, trackingID)
	default:
		return fmt.Sprintf("%s/link/detail/%s?track=%s", baseURL, req.MediaID, trackingID)
	}
}

func (dls *DeepLinkingService) generateFallbackURL(req *DeepLinkRequest) string {
	baseURL := dls.baseURL
	if baseURL == "" {
		baseURL = "https://catalogizer.app"
	}

	return fmt.Sprintf("%s/detail/%s", baseURL, req.MediaID)
}

func (dls *DeepLinkingService) generateQRCodeURL(link string) string {
	// Use a QR code generation service
	return fmt.Sprintf("https://api.qrserver.com/v1/create-qr-code/?size=200x200&data=%s", neturl.QueryEscape(link))
}

func (dls *DeepLinkingService) generateTrackingID() string {
	// Generate unique tracking ID
	return fmt.Sprintf("track_%d", time.Now().UnixNano())
}

func (dls *DeepLinkingService) getSupportedApps() []string {
	return []string{
		"catalogizer-web",
		"catalogizer-android",
		"catalogizer-ios",
		"catalogizer-desktop",
		"catalogizer-tv",
	}
}

func (dls *DeepLinkingService) getRequiredFeatures(req *DeepLinkRequest) []string {
	var features []string

	// Base features based on action
	if req.Action == "play" {
		features = append(features, "media_playback")
	}

	if req.Action == "download" {
		features = append(features, "file_download")
	}

	if req.Action == "edit" {
		features = append(features, "file_edit", "user_authentication")
	}

	// Media type specific features
	if req.MediaMetadata != nil && req.MediaMetadata.MediaType != "" {
		switch req.MediaMetadata.MediaType {
		case "video":
			if req.Action == "play" {
				features = append(features, "video_playback", "fullscreen_video")
			}
		case "audio":
			if req.Action == "play" {
				features = append(features, "audio_playback", "background_audio")
			}
		case "book":
			features = append(features, "pdf_reader", "epub_reader")
		case "game":
			features = append(features, "external_app_launch")
		}
	}

	return features
}

// Link tracking methods
func (dls *DeepLinkingService) TrackLinkEvent(ctx context.Context, event *LinkTrackingEvent) error {
	// This would normally store the event in a database or analytics service
	// For now, we'll just log it
	if logging.Logger != nil {
		logging.With(logging.Any("event", event)).Debug("Link tracking event")
	}
	return nil
}

// ErrLinkAnalyticsNotImplemented is returned because TrackLinkEvent does not
// persist events, so there is nothing real to aggregate (CAT-001: the former
// implementation returned invented click counts).
var ErrLinkAnalyticsNotImplemented = errors.New("link analytics not implemented: link events are not stored")

func (dls *DeepLinkingService) GetLinkAnalytics(ctx context.Context, trackingID string) (*LinkAnalytics, error) {
	return nil, ErrLinkAnalyticsNotImplemented
}

type LinkAnalytics struct {
	TrackingID        string         `json:"tracking_id"`
	TotalClicks       int            `json:"total_clicks"`
	UniqueClicks      int            `json:"unique_clicks"`
	PlatformBreakdown map[string]int `json:"platform_breakdown"`
	ConversionRate    float64        `json:"conversion_rate"`
	FirstClickAt      time.Time      `json:"first_click_at"`
	LastClickAt       time.Time      `json:"last_click_at"`
}

// App registration and configuration
func (dls *DeepLinkingService) RegisterApp(ctx context.Context, config *AppConfiguration) error {
	// This would normally store the app configuration
	// For now, we'll just validate it
	if config.AppID == "" {
		return fmt.Errorf("app_id is required")
	}
	if config.Name == "" {
		return fmt.Errorf("app name is required")
	}
	if len(config.Platforms) == 0 {
		return fmt.Errorf("at least one platform is required")
	}

	return nil
}

func (dls *DeepLinkingService) GetAppConfiguration(ctx context.Context, appID string) (*AppConfiguration, error) {
	// Mock implementation - would normally retrieve from storage
	return &AppConfiguration{
		AppID:     appID,
		Name:      "Catalogizer",
		Platforms: []string{"web", "android", "ios", "desktop"},
		Schemes: map[string]string{
			"web":     "https",
			"android": "catalogizer",
			"ios":     "catalogizer",
			"desktop": "catalogizer-desktop",
		},
		Packages: map[string]string{
			"android": "com.catalogizer.app",
			"ios":     "com.catalogizer.app",
		},
		StoreURLs: map[string]string{
			"android": "https://play.google.com/store/apps/details?id=com.catalogizer.app",
			"ios":     "https://apps.apple.com/app/id123456789",
			"desktop": "https://github.com/catalogizer/desktop/releases",
		},
		MinVersions: map[string]string{
			"android": "1.0.0",
			"ios":     "1.0.0",
			"desktop": "1.0.0",
		},
		Features:      []string{"video_playback", "audio_playback", "pdf_reader", "file_download"},
		PreferredApps: true,
		Active:        true,
	}, nil
}

// Smart link routing based on user context
func (dls *DeepLinkingService) GenerateSmartLink(ctx context.Context, req *DeepLinkRequest) (*SmartLinkResponse, error) {
	// Analyze user context to determine best link strategy
	strategy := dls.determineRoutingStrategy(req.Context)

	response := &SmartLinkResponse{
		Strategy:      strategy,
		PrimaryLink:   "",
		FallbackLinks: make([]string, 0),
		Instructions:  make(map[string]string),
	}

	switch strategy {
	case "native_preferred":
		// Try native app first, fallback to web
		if req.Context.Platform == "android" {
			androidLink, _ := dls.generateAndroidLink(req, dls.generateTrackingID())
			response.PrimaryLink = androidLink.URL
			webLink, _ := dls.generateWebLink(req, dls.generateTrackingID())
			response.FallbackLinks = append(response.FallbackLinks, webLink.URL)
		} else if req.Context.Platform == "ios" {
			iosLink, _ := dls.generateIOSLink(req, dls.generateTrackingID())
			response.PrimaryLink = iosLink.URL
			webLink, _ := dls.generateWebLink(req, dls.generateTrackingID())
			response.FallbackLinks = append(response.FallbackLinks, webLink.URL)
		}

	case "web_only":
		// Web-only strategy
		webLink, _ := dls.generateWebLink(req, dls.generateTrackingID())
		response.PrimaryLink = webLink.URL

	case "universal":
		// Universal link that detects platform
		response.PrimaryLink = dls.generateUniversalLink(req, dls.generateTrackingID())
	}

	// Add usage instructions
	response.Instructions["primary"] = "Tap to open in your preferred app"
	response.Instructions["fallback"] = "If the app doesn't open, use the web version"

	return response, nil
}

type SmartLinkResponse struct {
	Strategy      string            `json:"strategy"`
	PrimaryLink   string            `json:"primary_link"`
	FallbackLinks []string          `json:"fallback_links"`
	Instructions  map[string]string `json:"instructions"`
}

func (dls *DeepLinkingService) determineRoutingStrategy(context *LinkContext) string {
	if context == nil {
		return "universal"
	}

	// Determine strategy based on context
	switch context.Platform {
	case "android", "ios":
		return "native_preferred"
	case "web":
		return "web_only"
	default:
		return "universal"
	}
}

// Batch link generation for multiple items
func (dls *DeepLinkingService) GenerateBatchLinks(ctx context.Context, requests []*DeepLinkRequest) ([]*DeepLinkResponse, error) {
	responses := make([]*DeepLinkResponse, 0, len(requests))

	for _, req := range requests {
		response, err := dls.GenerateDeepLinks(ctx, req)
		if err != nil {
			// Continue with others even if one fails
			continue
		}
		responses = append(responses, response)
	}

	return responses, nil
}

// Link validation and testing
func (dls *DeepLinkingService) ValidateLinks(ctx context.Context, links []*DeepLink) (*ValidationResult, error) {
	result := &ValidationResult{
		TotalLinks:   len(links),
		ValidLinks:   0,
		InvalidLinks: 0,
		Warnings:     make([]string, 0),
		Errors:       make([]string, 0),
	}

	for _, link := range links {
		if dls.validateLink(link) {
			result.ValidLinks++
		} else {
			result.InvalidLinks++
			result.Errors = append(result.Errors, fmt.Sprintf("Invalid link: %s", link.URL))
		}
	}

	return result, nil
}

type ValidationResult struct {
	TotalLinks   int      `json:"total_links"`
	ValidLinks   int      `json:"valid_links"`
	InvalidLinks int      `json:"invalid_links"`
	Warnings     []string `json:"warnings"`
	Errors       []string `json:"errors"`
}

func (dls *DeepLinkingService) validateLink(link *DeepLink) bool {
	if link.URL == "" {
		return false
	}
	if link.Scheme == "" {
		return false
	}
	// Add more validation logic as needed
	return true
}
