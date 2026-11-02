package services

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"catalogizer/internal/models"
)

// ---------------------------------------------------------------------------
// Constructor
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// GenerateDeepLinks
// ---------------------------------------------------------------------------

func TestGenerateDeepLinks_EmptyMediaID(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	resp, err := svc.GenerateDeepLinks(context.Background(), &DeepLinkRequest{
		MediaID: "",
		Action:  "detail",
	})
	if err == nil {
		t.Fatal("expected error for empty media ID")
	}
	if resp != nil {
		t.Fatal("expected nil response for empty media ID")
	}
	if !strings.Contains(err.Error(), "media ID is required") {
		t.Errorf("error = %q, want it to contain 'media ID is required'", err.Error())
	}
}

func TestGenerateDeepLinks_AllActions(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")
	ctx := context.Background()

	tests := []struct {
		action         string
		expectExpiry   bool
		expectAuth     map[string]bool
		expectAutoplay bool
	}{
		{
			action:       "detail",
			expectExpiry: false,
			expectAuth: map[string]bool{
				"web": false, "android": false, "ios": false, "desktop": false,
			},
			expectAutoplay: false,
		},
		{
			action:       "play",
			expectExpiry: true,
			expectAuth: map[string]bool{
				"web": false, "android": false, "ios": false, "desktop": false,
			},
			expectAutoplay: true,
		},
		{
			action:       "download",
			expectExpiry: true,
			expectAuth: map[string]bool{
				"web": true, "android": true, "ios": true, "desktop": true,
			},
			expectAutoplay: false,
		},
		{
			action:       "edit",
			expectExpiry: false,
			expectAuth: map[string]bool{
				"web": true, "android": true, "ios": true, "desktop": true,
			},
			expectAutoplay: false,
		},
	}

	for _, tt := range tests {
		t.Run("action_"+tt.action, func(t *testing.T) {
			resp, err := svc.GenerateDeepLinks(ctx, &DeepLinkRequest{
				MediaID: "media-123",
				Action:  tt.action,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp == nil {
				t.Fatal("expected non-nil response")
			}

			// Four platforms
			for _, platform := range []string{"web", "android", "ios", "desktop"} {
				link, ok := resp.Links[platform]
				if !ok {
					t.Errorf("missing link for platform %q", platform)
					continue
				}
				if link.URL == "" {
					t.Errorf("empty URL for platform %q", platform)
				}
				if link.RequiresAuth != tt.expectAuth[platform] {
					t.Errorf("platform %q RequiresAuth = %v, want %v",
						platform, link.RequiresAuth, tt.expectAuth[platform])
				}
				_, hasAutoplay := link.Parameters["autoplay"]
				if hasAutoplay != tt.expectAutoplay {
					t.Errorf("platform %q autoplay present = %v, want %v",
						platform, hasAutoplay, tt.expectAutoplay)
				}
			}

			// Expiration
			if tt.expectExpiry && resp.ExpiresAt == nil {
				t.Error("expected ExpiresAt to be set")
			}
			if !tt.expectExpiry && resp.ExpiresAt != nil {
				t.Error("expected ExpiresAt to be nil")
			}

			// Universal / shareable / tracking / QR / fallback / apps
			if resp.UniversalLink == "" {
				t.Error("expected non-empty universal link")
			}
			if !strings.Contains(resp.UniversalLink, "media-123") {
				t.Error("universal link should contain media ID")
			}
			if !strings.Contains(resp.UniversalLink, tt.action) {
				t.Errorf("universal link should contain action %q", tt.action)
			}
			if resp.ShareableLink != resp.UniversalLink {
				t.Error("shareable link should equal universal link")
			}
			if resp.TrackingID == "" || !strings.HasPrefix(resp.TrackingID, "track_") {
				t.Errorf("bad tracking ID: %q", resp.TrackingID)
			}
			if resp.QRCode == "" {
				t.Error("expected non-empty QR code URL")
			}
			if resp.FallbackURL == "" {
				t.Error("expected non-empty fallback URL")
			}
			if len(resp.SupportedApps) == 0 {
				t.Error("expected supported apps list")
			}
		})
	}
}

func TestGenerateDeepLinks_DefaultAction(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	resp, err := svc.GenerateDeepLinks(context.Background(), &DeepLinkRequest{
		MediaID: "media-456",
		Action:  "unknown_action",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back to detail path
	webLink := resp.Links["web"]
	if webLink == nil {
		t.Fatal("expected web link")
	}
	if !strings.Contains(webLink.URL, "/detail/media-456") {
		t.Errorf("default action should produce detail path, got URL: %s", webLink.URL)
	}
	if resp.ExpiresAt != nil {
		t.Error("expected no expiration for unknown action")
	}
}

func TestGenerateDeepLinks_ExpirationForPlayAndDownloadOnly(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")
	ctx := context.Background()

	tests := []struct {
		action       string
		shouldExpire bool
	}{
		{"detail", false},
		{"play", true},
		{"download", true},
		{"edit", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			resp, err := svc.GenerateDeepLinks(ctx, &DeepLinkRequest{
				MediaID: "exp-test",
				Action:  tt.action,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.shouldExpire && resp.ExpiresAt == nil {
				t.Errorf("action %q should set ExpiresAt", tt.action)
			}
			if !tt.shouldExpire && resp.ExpiresAt != nil {
				t.Errorf("action %q should not set ExpiresAt", tt.action)
			}
		})
	}
}

func TestGenerateDeepLinks_WithContext(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	resp, err := svc.GenerateDeepLinks(context.Background(), &DeepLinkRequest{
		MediaID: "media-789",
		Action:  "detail",
		Context: &LinkContext{
			UserID:       "user-1",
			DeviceID:     "device-1",
			SessionID:    "session-1",
			ReferrerPage: "/library",
			Platform:     "web",
			AppVersion:   "2.0.0",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Web link context params
	web := resp.Links["web"]
	if web == nil {
		t.Fatal("expected web link")
	}
	if web.Parameters["user_id"] != "user-1" {
		t.Errorf("user_id = %q, want 'user-1'", web.Parameters["user_id"])
	}
	if web.Parameters["session_id"] != "session-1" {
		t.Errorf("session_id = %q, want 'session-1'", web.Parameters["session_id"])
	}
	if web.Parameters["ref"] != "/library" {
		t.Errorf("ref = %q, want '/library'", web.Parameters["ref"])
	}

	// Android link context
	android := resp.Links["android"]
	if android == nil {
		t.Fatal("expected android link")
	}
	if android.Parameters["user_id"] != "user-1" {
		t.Errorf("android user_id = %q, want 'user-1'", android.Parameters["user_id"])
	}
	if android.Parameters["session_id"] != "session-1" {
		t.Errorf("android session_id = %q, want 'session-1'", android.Parameters["session_id"])
	}
}

func TestGenerateDeepLinks_WithFullUTMParams(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	resp, err := svc.GenerateDeepLinks(context.Background(), &DeepLinkRequest{
		MediaID: "media-utm",
		Action:  "detail",
		Context: &LinkContext{
			UTMParams: &UTMParameters{
				Source:   "newsletter",
				Medium:   "email",
				Campaign: "summer2025",
				Term:     "media+manager",
				Content:  "cta_button",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	web := resp.Links["web"]
	if web == nil {
		t.Fatal("expected web link")
	}

	want := map[string]string{
		"utm_source":   "newsletter",
		"utm_medium":   "email",
		"utm_campaign": "summer2025",
		"utm_term":     "media+manager",
		"utm_content":  "cta_button",
	}
	for k, v := range want {
		if web.Parameters[k] != v {
			t.Errorf("web %s = %q, want %q", k, web.Parameters[k], v)
		}
	}

	// They should also appear in the URL query string
	for _, p := range []string{"utm_source=", "utm_medium=", "utm_campaign=", "utm_term=", "utm_content="} {
		if !strings.Contains(web.URL, p) {
			t.Errorf("web URL should contain %q", p)
		}
	}
}

func TestGenerateDeepLinks_PartialUTMParams(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	resp, err := svc.GenerateDeepLinks(context.Background(), &DeepLinkRequest{
		MediaID: "media-partial-utm",
		Action:  "detail",
		Context: &LinkContext{
			UTMParams: &UTMParameters{
				Source: "google",
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	web := resp.Links["web"]
	if web == nil {
		t.Fatal("expected web link")
	}
	if web.Parameters["utm_source"] != "google" {
		t.Errorf("utm_source = %q, want 'google'", web.Parameters["utm_source"])
	}
	for _, key := range []string{"utm_medium", "utm_campaign", "utm_term", "utm_content"} {
		if _, exists := web.Parameters[key]; exists {
			t.Errorf("empty UTM param %q should not be in parameters", key)
		}
	}
}

func TestGenerateDeepLinks_NilContext(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	resp, err := svc.GenerateDeepLinks(context.Background(), &DeepLinkRequest{
		MediaID: "media-nocontext",
		Action:  "detail",
		Context: nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Links) != 4 {
		t.Errorf("expected 4 platform links, got %d", len(resp.Links))
	}

	web := resp.Links["web"]
	if web == nil {
		t.Fatal("expected web link")
	}
	if _, exists := web.Parameters["user_id"]; exists {
		t.Error("user_id should not be present without context")
	}
	if _, exists := web.Parameters["session_id"]; exists {
		t.Error("session_id should not be present without context")
	}
	if web.Parameters["track"] == "" {
		t.Error("track parameter should always be present")
	}
}

func TestGenerateDeepLinks_ContextWithNilUTMParams(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	resp, err := svc.GenerateDeepLinks(context.Background(), &DeepLinkRequest{
		MediaID: "ctx-nil-utm",
		Action:  "detail",
		Context: &LinkContext{
			UserID:    "user-1",
			UTMParams: nil,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	web := resp.Links["web"]
	if web == nil {
		t.Fatal("expected web link")
	}
	for _, key := range []string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content"} {
		if _, exists := web.Parameters[key]; exists {
			t.Errorf("nil UTM should not produce %q", key)
		}
	}
}

func TestGenerateDeepLinks_ContextWithAllEmptyFields(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	resp, err := svc.GenerateDeepLinks(context.Background(), &DeepLinkRequest{
		MediaID: "ctx-empty",
		Action:  "detail",
		Context: &LinkContext{
			UserID:       "",
			DeviceID:     "",
			SessionID:    "",
			ReferrerPage: "",
			Platform:     "",
			AppVersion:   "",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	web := resp.Links["web"]
	if web == nil {
		t.Fatal("expected web link")
	}
	for _, key := range []string{"user_id", "session_id", "ref"} {
		if _, exists := web.Parameters[key]; exists {
			t.Errorf("empty field %q should not be in parameters", key)
		}
	}
}

func TestGenerateDeepLinks_EmptyBaseURLFallsBack(t *testing.T) {
	svc := NewDeepLinkingService("", "v1")

	resp, err := svc.GenerateDeepLinks(context.Background(), &DeepLinkRequest{
		MediaID: "media-fb",
		Action:  "detail",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	web := resp.Links["web"]
	if web == nil {
		t.Fatal("expected web link")
	}
	if !strings.HasPrefix(web.URL, "https://catalogizer.app") {
		t.Errorf("expected default base URL, got: %s", web.URL)
	}
	if !strings.HasPrefix(resp.UniversalLink, "https://catalogizer.app") {
		t.Errorf("universal link should use default, got: %s", resp.UniversalLink)
	}
	if !strings.HasPrefix(resp.FallbackURL, "https://catalogizer.app") {
		t.Errorf("fallback should use default, got: %s", resp.FallbackURL)
	}
}

func TestGenerateDeepLinks_CustomBaseURL(t *testing.T) {
	custom := "https://my-catalog.example.com"
	svc := NewDeepLinkingService(custom, "v2")

	resp, err := svc.GenerateDeepLinks(context.Background(), &DeepLinkRequest{
		MediaID: "custom-1",
		Action:  "detail",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.HasPrefix(resp.Links["web"].URL, custom) {
		t.Errorf("web link should use custom URL, got: %s", resp.Links["web"].URL)
	}
	if !strings.HasPrefix(resp.UniversalLink, custom) {
		t.Errorf("universal link should use custom URL, got: %s", resp.UniversalLink)
	}
	if !strings.HasPrefix(resp.FallbackURL, custom) {
		t.Errorf("fallback URL should use custom URL, got: %s", resp.FallbackURL)
	}

	// Native links must NOT use the custom web URL
	android := resp.Links["android"]
	if android != nil && strings.Contains(android.URL, custom) {
		t.Error("android link should not contain custom web base URL")
	}
}

func TestGenerateDeepLinks_TrackingInAllPlatformLinks(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	resp, err := svc.GenerateDeepLinks(context.Background(), &DeepLinkRequest{
		MediaID: "track-all",
		Action:  "detail",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for platform, link := range resp.Links {
		if link.Parameters["track"] == "" {
			t.Errorf("platform %q: missing track parameter", platform)
		}
	}
}

func TestGenerateDeepLinks_AllLinksHaveInitializedParameters(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	resp, err := svc.GenerateDeepLinks(context.Background(), &DeepLinkRequest{
		MediaID: "param-check",
		Action:  "detail",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for platform, link := range resp.Links {
		if link.Parameters == nil {
			t.Errorf("platform %q: parameters map should not be nil", platform)
		}
	}
}

func TestGenerateDeepLinks_ResponseFieldsNotEmpty(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	resp, err := svc.GenerateDeepLinks(context.Background(), &DeepLinkRequest{
		MediaID: "fields-test",
		Action:  "detail",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	checks := map[string]string{
		"Links": func() string {
			if resp.Links == nil {
				return ""
			}
			return "ok"
		}(),
		"UniversalLink": resp.UniversalLink,
		"ShareableLink": resp.ShareableLink,
		"QRCode":        resp.QRCode,
		"TrackingID":    resp.TrackingID,
		"FallbackURL":   resp.FallbackURL,
	}
	for field, val := range checks {
		if val == "" {
			t.Errorf("%s should not be empty", field)
		}
	}
	if len(resp.SupportedApps) == 0 {
		t.Error("SupportedApps should not be empty")
	}
}

func TestGenerateDeepLinks_SpecialCharactersInMediaID(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")
	ctx := context.Background()

	ids := []string{
		"media-with-dashes",
		"media_with_underscores",
		"media.with.dots",
		"media/with/slashes",
		"12345",
		"uuid-550e8400-e29b-41d4-a716-446655440000",
	}

	for _, id := range ids {
		t.Run("id_"+id, func(t *testing.T) {
			resp, err := svc.GenerateDeepLinks(ctx, &DeepLinkRequest{
				MediaID: id,
				Action:  "detail",
			})
			if err != nil {
				t.Fatalf("unexpected error for ID %q: %v", id, err)
			}
			if len(resp.Links) == 0 {
				t.Error("expected at least one platform link")
			}
		})
	}
}

func TestGenerateDeepLinks_WithMediaMetadata(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")
	ctx := context.Background()

	tests := []struct {
		name        string
		mediaType   string
		action      string
		expectFeats []string
	}{
		{"video play", "video", "play",
			[]string{"media_playback", "video_playback", "fullscreen_video"}},
		{"audio play", "audio", "play",
			[]string{"media_playback", "audio_playback", "background_audio"}},
		{"book detail", "book", "detail",
			[]string{"pdf_reader", "epub_reader"}},
		{"game detail", "game", "detail",
			[]string{"external_app_launch"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.GenerateDeepLinks(ctx, &DeepLinkRequest{
				MediaID:       "media-feat",
				Action:        tt.action,
				MediaMetadata: &models.MediaMetadata{MediaType: tt.mediaType},
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			for _, platform := range []string{"android", "ios", "desktop"} {
				link := resp.Links[platform]
				if link == nil {
					t.Errorf("missing %s link", platform)
					continue
				}
				set := make(map[string]bool)
				for _, f := range link.Features {
					set[f] = true
				}
				for _, want := range tt.expectFeats {
					if !set[want] {
						t.Errorf("platform %q: expected feature %q, got %v",
							platform, want, link.Features)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// generatePlatformLink
// ---------------------------------------------------------------------------

func TestGeneratePlatformLink_AllPlatforms(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")
	ctx := context.Background()

	tests := []struct {
		platform      string
		expectScheme  string
		expectPackage string
		expectBundle  string
		storeContains string
	}{
		{"web", "https", "", "", ""},
		{"android", "catalogizer", "com.catalogizer.app", "", "play.google.com"},
		{"ios", "catalogizer", "", "com.catalogizer.app", "apps.apple.com"},
		{"desktop", "catalogizer-desktop", "", "", "github.com"},
	}

	for _, tt := range tests {
		t.Run(tt.platform, func(t *testing.T) {
			link, err := svc.generatePlatformLink(ctx, &DeepLinkRequest{
				MediaID: "test-media",
				Action:  "detail",
			}, tt.platform, "track_test")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if link.Scheme != tt.expectScheme {
				t.Errorf("scheme = %q, want %q", link.Scheme, tt.expectScheme)
			}
			if tt.expectPackage != "" && link.Package != tt.expectPackage {
				t.Errorf("package = %q, want %q", link.Package, tt.expectPackage)
			}
			if tt.expectBundle != "" && link.BundleID != tt.expectBundle {
				t.Errorf("bundleID = %q, want %q", link.BundleID, tt.expectBundle)
			}
			if tt.storeContains != "" && !strings.Contains(link.StoreURL, tt.storeContains) {
				t.Errorf("storeURL %q should contain %q", link.StoreURL, tt.storeContains)
			}
		})
	}
}

func TestGeneratePlatformLink_UnsupportedPlatform(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	link, err := svc.generatePlatformLink(context.Background(), &DeepLinkRequest{
		MediaID: "test",
		Action:  "detail",
	}, "roku", "track_test")

	if err == nil {
		t.Fatal("expected error for unsupported platform")
	}
	if link != nil {
		t.Fatal("expected nil link")
	}
	if !strings.Contains(err.Error(), "unsupported platform") {
		t.Errorf("error = %q, want 'unsupported platform'", err.Error())
	}
}

// ---------------------------------------------------------------------------
// generateWebLink
// ---------------------------------------------------------------------------

func TestGenerateWebLink_Actions(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	tests := []struct {
		action     string
		expectPath string
		expectAuth bool
		autoplay   bool
	}{
		{"detail", "/detail/m1", false, false},
		{"play", "/play/m1", false, true},
		{"download", "/download/m1", true, false},
		{"edit", "/edit/m1", true, false},
		{"unknown", "/detail/m1", false, false},
		{"", "/detail/m1", false, false},
	}

	for _, tt := range tests {
		t.Run("action_"+tt.action, func(t *testing.T) {
			link, err := svc.generateWebLink(&DeepLinkRequest{
				MediaID: "m1",
				Action:  tt.action,
			}, "track_001")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(link.URL, tt.expectPath) {
				t.Errorf("URL %q should contain %q", link.URL, tt.expectPath)
			}
			if link.RequiresAuth != tt.expectAuth {
				t.Errorf("RequiresAuth = %v, want %v", link.RequiresAuth, tt.expectAuth)
			}
			if link.Parameters["track"] != "track_001" {
				t.Errorf("track = %q, want 'track_001'", link.Parameters["track"])
			}
			_, hasAP := link.Parameters["autoplay"]
			if hasAP != tt.autoplay {
				t.Errorf("autoplay present = %v, want %v", hasAP, tt.autoplay)
			}
		})
	}
}

func TestGenerateWebLink_EmptyBaseURLFallback(t *testing.T) {
	svc := NewDeepLinkingService("", "v1")

	link, err := svc.generateWebLink(&DeepLinkRequest{
		MediaID: "fb",
		Action:  "detail",
	}, "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(link.URL, "https://catalogizer.app") {
		t.Errorf("expected default base URL, got: %s", link.URL)
	}
}

func TestGenerateWebLink_ContextParams(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	link, err := svc.generateWebLink(&DeepLinkRequest{
		MediaID: "m1",
		Action:  "detail",
		Context: &LinkContext{
			UserID:       "u1",
			SessionID:    "s1",
			ReferrerPage: "/home",
		},
	}, "t1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link.Parameters["user_id"] != "u1" {
		t.Errorf("user_id = %q", link.Parameters["user_id"])
	}
	if link.Parameters["session_id"] != "s1" {
		t.Errorf("session_id = %q", link.Parameters["session_id"])
	}
	if link.Parameters["ref"] != "/home" {
		t.Errorf("ref = %q", link.Parameters["ref"])
	}
}

func TestGenerateWebLink_AllUTMFieldsInURL(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	link, err := svc.generateWebLink(&DeepLinkRequest{
		MediaID: "utm-full",
		Action:  "detail",
		Context: &LinkContext{
			UTMParams: &UTMParameters{
				Source: "src", Medium: "med", Campaign: "camp", Term: "trm", Content: "cnt",
			},
		},
	}, "track_utm")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, p := range []string{"utm_source=", "utm_medium=", "utm_campaign=", "utm_term=", "utm_content=", "track="} {
		if !strings.Contains(link.URL, p) {
			t.Errorf("URL should contain %q, got: %s", p, link.URL)
		}
	}
}

// ---------------------------------------------------------------------------
// generateAndroidLink
// ---------------------------------------------------------------------------

func TestGenerateAndroidLink_AllActions(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	tests := []struct {
		action     string
		expectPath string
		expectAuth bool
		autoplay   bool
	}{
		{"detail", "detail/m1", false, false},
		{"play", "play/m1", false, true},
		{"download", "download/m1", true, false},
		{"edit", "edit/m1", true, false},
		{"unknown", "detail/m1", false, false},
	}

	for _, tt := range tests {
		t.Run("action_"+tt.action, func(t *testing.T) {
			link, err := svc.generateAndroidLink(&DeepLinkRequest{
				MediaID: "m1",
				Action:  tt.action,
			}, "track_a")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasPrefix(link.URL, "catalogizer://") {
				t.Errorf("URL should start with 'catalogizer://', got %q", link.URL)
			}
			if !strings.Contains(link.URL, tt.expectPath) {
				t.Errorf("URL %q should contain %q", link.URL, tt.expectPath)
			}
			if link.Package != "com.catalogizer.app" {
				t.Errorf("package = %q", link.Package)
			}
			if link.AppVersion != "1.0.0" {
				t.Errorf("app version = %q", link.AppVersion)
			}
			if link.RequiresAuth != tt.expectAuth {
				t.Errorf("RequiresAuth = %v, want %v", link.RequiresAuth, tt.expectAuth)
			}
			if !strings.Contains(link.StoreURL, "play.google.com") {
				t.Errorf("store URL = %q", link.StoreURL)
			}
			_, hasAP := link.Parameters["autoplay"]
			if hasAP != tt.autoplay {
				t.Errorf("autoplay present = %v, want %v", hasAP, tt.autoplay)
			}
		})
	}
}

func TestGenerateAndroidLink_WithContext(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	link, err := svc.generateAndroidLink(&DeepLinkRequest{
		MediaID: "m1",
		Action:  "detail",
		Context: &LinkContext{UserID: "u1", SessionID: "s1"},
	}, "track_ctx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link.Parameters["user_id"] != "u1" {
		t.Errorf("user_id = %q", link.Parameters["user_id"])
	}
	if link.Parameters["session_id"] != "s1" {
		t.Errorf("session_id = %q", link.Parameters["session_id"])
	}
	if !strings.Contains(link.URL, "user_id=u1") {
		t.Errorf("URL should contain user_id param: %s", link.URL)
	}
}

func TestGenerateAndroidLink_NilContext(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	link, err := svc.generateAndroidLink(&DeepLinkRequest{
		MediaID: "m1",
		Action:  "detail",
		Context: nil,
	}, "track_nil")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link.Parameters["track"] != "track_nil" {
		t.Errorf("track = %q", link.Parameters["track"])
	}
	if _, exists := link.Parameters["user_id"]; exists {
		t.Error("user_id should not be present with nil context")
	}
}

func TestGenerateAndroidLink_Features(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	link, err := svc.generateAndroidLink(&DeepLinkRequest{
		MediaID:       "m1",
		Action:        "play",
		MediaMetadata: &models.MediaMetadata{MediaType: "video"},
	}, "t")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	set := make(map[string]bool)
	for _, f := range link.Features {
		set[f] = true
	}
	for _, want := range []string{"media_playback", "video_playback", "fullscreen_video"} {
		if !set[want] {
			t.Errorf("expected feature %q in %v", want, link.Features)
		}
	}
}

// ---------------------------------------------------------------------------
// generateIOSLink
// ---------------------------------------------------------------------------

func TestGenerateIOSLink_AllActions(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	tests := []struct {
		action     string
		expectPath string
		expectAuth bool
		autoplay   bool
	}{
		{"detail", "detail/m1", false, false},
		{"play", "play/m1", false, true},
		{"download", "download/m1", true, false},
		{"edit", "edit/m1", true, false},
		{"unknown", "detail/m1", false, false},
	}

	for _, tt := range tests {
		t.Run("action_"+tt.action, func(t *testing.T) {
			link, err := svc.generateIOSLink(&DeepLinkRequest{
				MediaID: "m1",
				Action:  tt.action,
			}, "track_ios")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasPrefix(link.URL, "catalogizer://") {
				t.Errorf("URL should start with 'catalogizer://', got %q", link.URL)
			}
			if !strings.Contains(link.URL, tt.expectPath) {
				t.Errorf("URL %q should contain %q", link.URL, tt.expectPath)
			}
			if link.BundleID != "com.catalogizer.app" {
				t.Errorf("bundleID = %q", link.BundleID)
			}
			if link.AppVersion != "1.0.0" {
				t.Errorf("app version = %q", link.AppVersion)
			}
			if link.RequiresAuth != tt.expectAuth {
				t.Errorf("RequiresAuth = %v, want %v", link.RequiresAuth, tt.expectAuth)
			}
			if !strings.Contains(link.StoreURL, "apps.apple.com") {
				t.Errorf("store URL = %q", link.StoreURL)
			}
			_, hasAP := link.Parameters["autoplay"]
			if hasAP != tt.autoplay {
				t.Errorf("autoplay present = %v, want %v", hasAP, tt.autoplay)
			}
		})
	}
}

func TestGenerateIOSLink_WithContext(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	link, err := svc.generateIOSLink(&DeepLinkRequest{
		MediaID: "m1",
		Action:  "detail",
		Context: &LinkContext{UserID: "ios-user", SessionID: "ios-session"},
	}, "track_ios_ctx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link.Parameters["user_id"] != "ios-user" {
		t.Errorf("user_id = %q", link.Parameters["user_id"])
	}
	if link.Parameters["session_id"] != "ios-session" {
		t.Errorf("session_id = %q", link.Parameters["session_id"])
	}
}

func TestGenerateIOSLink_NilContext(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	link, err := svc.generateIOSLink(&DeepLinkRequest{
		MediaID: "m1",
		Action:  "detail",
		Context: nil,
	}, "track_nil")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := link.Parameters["user_id"]; exists {
		t.Error("user_id should not be present with nil context")
	}
}

// ---------------------------------------------------------------------------
// generateDesktopLink
// ---------------------------------------------------------------------------

func TestGenerateDesktopLink_AllActions(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	tests := []struct {
		action     string
		expectPath string
		expectAuth bool
		autoplay   bool
	}{
		{"detail", "detail/m1", false, false},
		{"play", "play/m1", false, true},
		{"download", "download/m1", true, false},
		{"edit", "edit/m1", true, false},
		{"unknown", "detail/m1", false, false},
	}

	for _, tt := range tests {
		t.Run("action_"+tt.action, func(t *testing.T) {
			link, err := svc.generateDesktopLink(&DeepLinkRequest{
				MediaID: "m1",
				Action:  tt.action,
			}, "track_desk")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasPrefix(link.URL, "catalogizer-desktop://") {
				t.Errorf("URL should start with 'catalogizer-desktop://', got %q", link.URL)
			}
			if !strings.Contains(link.URL, tt.expectPath) {
				t.Errorf("URL %q should contain %q", link.URL, tt.expectPath)
			}
			if link.Scheme != "catalogizer-desktop" {
				t.Errorf("scheme = %q", link.Scheme)
			}
			if link.AppVersion != "1.0.0" {
				t.Errorf("app version = %q", link.AppVersion)
			}
			if link.RequiresAuth != tt.expectAuth {
				t.Errorf("RequiresAuth = %v, want %v", link.RequiresAuth, tt.expectAuth)
			}
			if !strings.Contains(link.StoreURL, "github.com") {
				t.Errorf("store URL = %q", link.StoreURL)
			}
			_, hasAP := link.Parameters["autoplay"]
			if hasAP != tt.autoplay {
				t.Errorf("autoplay present = %v, want %v", hasAP, tt.autoplay)
			}
		})
	}
}

func TestGenerateDesktopLink_WithContext(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	link, err := svc.generateDesktopLink(&DeepLinkRequest{
		MediaID: "m1",
		Action:  "detail",
		Context: &LinkContext{UserID: "desk-user", SessionID: "desk-session"},
	}, "track_desk_ctx")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if link.Parameters["user_id"] != "desk-user" {
		t.Errorf("user_id = %q", link.Parameters["user_id"])
	}
	if link.Parameters["session_id"] != "desk-session" {
		t.Errorf("session_id = %q", link.Parameters["session_id"])
	}
}

func TestGenerateDesktopLink_NilContext(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	link, err := svc.generateDesktopLink(&DeepLinkRequest{
		MediaID: "m1",
		Action:  "detail",
		Context: nil,
	}, "track_nil")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := link.Parameters["user_id"]; exists {
		t.Error("user_id should not be present with nil context")
	}
}

// ---------------------------------------------------------------------------
// generateUniversalLink
// ---------------------------------------------------------------------------

func TestGenerateUniversalLink(t *testing.T) {
	tests := []struct {
		name       string
		baseURL    string
		action     string
		mediaID    string
		expectBase string
		expectPath string
	}{
		{"detail custom base", "https://example.com", "detail", "abc",
			"https://example.com", "/link/detail/abc"},
		{"play empty base", "", "play", "xyz",
			"https://catalogizer.app", "/link/play/xyz"},
		{"download", "https://catalogizer.app", "download", "dl-001",
			"https://catalogizer.app", "/link/download/dl-001"},
		{"edit", "https://catalogizer.app", "edit", "e-100",
			"https://catalogizer.app", "/link/edit/e-100"},
		{"unknown defaults to detail", "https://catalogizer.app", "share", "s-200",
			"https://catalogizer.app", "/link/detail/s-200"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewDeepLinkingService(tt.baseURL, "v1")
			link := svc.generateUniversalLink(&DeepLinkRequest{
				MediaID: tt.mediaID,
				Action:  tt.action,
			}, "track_test")
			if !strings.HasPrefix(link, tt.expectBase) {
				t.Errorf("link %q should start with %q", link, tt.expectBase)
			}
			if !strings.Contains(link, tt.expectPath) {
				t.Errorf("link %q should contain %q", link, tt.expectPath)
			}
			if !strings.Contains(link, "track=track_test") {
				t.Error("link should contain tracking param")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// generateFallbackURL
// ---------------------------------------------------------------------------

func TestGenerateFallbackURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		mediaID string
		want    string
	}{
		{"custom base", "https://custom.app", "fb-1", "https://custom.app/detail/fb-1"},
		{"empty base", "", "fb-2", "https://catalogizer.app/detail/fb-2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := NewDeepLinkingService(tt.baseURL, "v1")
			got := svc.generateFallbackURL(&DeepLinkRequest{MediaID: tt.mediaID})
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// generateQRCodeURL
// ---------------------------------------------------------------------------

func TestGenerateQRCodeURL(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	qr := svc.generateQRCodeURL("https://catalogizer.app/link/detail/m1?track=abc")

	if !strings.HasPrefix(qr, "https://api.qrserver.com/v1/create-qr-code/") {
		t.Errorf("QR URL should use qrserver.com, got %q", qr)
	}
	if !strings.Contains(qr, "size=200x200") {
		t.Error("QR URL should specify size")
	}
	if !strings.Contains(qr, "data=") {
		t.Error("QR URL should contain data parameter")
	}
}

// ---------------------------------------------------------------------------
// generateTrackingID
// ---------------------------------------------------------------------------

func TestGenerateTrackingID(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	id1 := svc.generateTrackingID()
	id2 := svc.generateTrackingID()

	if !strings.HasPrefix(id1, "track_") {
		t.Errorf("id1 = %q, want prefix 'track_'", id1)
	}
	if !strings.HasPrefix(id2, "track_") {
		t.Errorf("id2 = %q, want prefix 'track_'", id2)
	}
	// Very likely different in sequential calls (UnixNano)
	if id1 == id2 {
		t.Log("warning: two consecutive tracking IDs are identical")
	}
}

// ---------------------------------------------------------------------------
// getSupportedApps
// ---------------------------------------------------------------------------

func TestGetSupportedApps(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")
	apps := svc.getSupportedApps()

	want := []string{
		"catalogizer-web",
		"catalogizer-android",
		"catalogizer-ios",
		"catalogizer-desktop",
		"catalogizer-tv",
	}
	if len(apps) != len(want) {
		t.Fatalf("len = %d, want %d", len(apps), len(want))
	}
	set := make(map[string]bool)
	for _, a := range apps {
		set[a] = true
	}
	for _, w := range want {
		if !set[w] {
			t.Errorf("missing app %q", w)
		}
	}
}

// ---------------------------------------------------------------------------
// getRequiredFeatures
// ---------------------------------------------------------------------------

func TestGetRequiredFeatures(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	tests := []struct {
		name      string
		action    string
		mediaType string
		wantFeats []string
		noFeats   []string
	}{
		{"detail no media", "detail", "", nil, nil},
		{"play no media", "play", "", []string{"media_playback"}, nil},
		{"download", "download", "", []string{"file_download"}, nil},
		{"edit", "edit", "", []string{"file_edit", "user_authentication"}, nil},
		{"play video", "play", "video",
			[]string{"media_playback", "video_playback", "fullscreen_video"}, nil},
		{"detail video", "detail", "video",
			nil, []string{"video_playback", "fullscreen_video"}},
		{"play audio", "play", "audio",
			[]string{"media_playback", "audio_playback", "background_audio"}, nil},
		{"detail audio", "detail", "audio",
			nil, []string{"audio_playback", "background_audio"}},
		{"book detail", "detail", "book",
			[]string{"pdf_reader", "epub_reader"}, nil},
		{"book play", "play", "book",
			[]string{"media_playback", "pdf_reader", "epub_reader"}, nil},
		{"game detail", "detail", "game",
			[]string{"external_app_launch"}, nil},
		{"game play", "play", "game",
			[]string{"media_playback", "external_app_launch"}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &DeepLinkRequest{MediaID: "test", Action: tt.action}
			if tt.mediaType != "" {
				req.MediaMetadata = &models.MediaMetadata{MediaType: tt.mediaType}
			}

			feats := svc.getRequiredFeatures(req)
			set := make(map[string]bool)
			for _, f := range feats {
				set[f] = true
			}
			for _, w := range tt.wantFeats {
				if !set[w] {
					t.Errorf("expected feature %q in %v", w, feats)
				}
			}
			for _, nw := range tt.noFeats {
				if set[nw] {
					t.Errorf("unexpected feature %q in %v", nw, feats)
				}
			}
		})
	}
}

func TestGetRequiredFeatures_NilMediaMetadata(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	feats := svc.getRequiredFeatures(&DeepLinkRequest{
		MediaID:       "test",
		Action:        "detail",
		MediaMetadata: nil,
	})
	if len(feats) != 0 {
		t.Errorf("expected empty features for detail with nil metadata, got %v", feats)
	}
}

func TestGetRequiredFeatures_EmptyMediaType(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	feats := svc.getRequiredFeatures(&DeepLinkRequest{
		MediaID:       "test",
		Action:        "play",
		MediaMetadata: &models.MediaMetadata{MediaType: ""},
	})
	// Should only have base "play" feature, no media-type-specific features
	if len(feats) != 1 || feats[0] != "media_playback" {
		t.Errorf("expected [media_playback], got %v", feats)
	}
}

// ---------------------------------------------------------------------------
// TrackLinkEvent
// ---------------------------------------------------------------------------

func TestTrackLinkEvent(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")
	ctx := context.Background()

	tests := []struct {
		name  string
		event *LinkTrackingEvent
	}{
		{"click success", &LinkTrackingEvent{
			TrackingID: "t1", EventType: "click", Platform: "web", Success: true, AppOpened: true,
		}},
		{"fallback", &LinkTrackingEvent{
			TrackingID: "t2", EventType: "fallback", Platform: "android",
			Success: false, FallbackUsed: true, ErrorMessage: "app not installed",
		}},
		{"error", &LinkTrackingEvent{
			TrackingID: "t3", EventType: "error", Platform: "ios",
			Success: false, ErrorMessage: "link expired",
		}},
		{"open with metadata", &LinkTrackingEvent{
			TrackingID: "t4", EventType: "open", Platform: "desktop",
			Success: true, AppOpened: true,
			Metadata: map[string]interface{}{"app_version": "2.1.0", "os": "macOS"},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := svc.TrackLinkEvent(ctx, tt.event); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetLinkAnalytics
// ---------------------------------------------------------------------------

func TestGetLinkAnalytics_NotFabricated(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")
	for _, id := range []string{"track_123", "track_abc"} {
		a, err := svc.GetLinkAnalytics(context.Background(), id)
		if !errors.Is(err, ErrLinkAnalyticsNotImplemented) {
			t.Fatalf("err = %v, want ErrLinkAnalyticsNotImplemented", err)
		}
		if a != nil {
			t.Fatalf("analytics must not be fabricated, got %+v", a)
		}
	}
}

// ---------------------------------------------------------------------------
// RegisterApp
// ---------------------------------------------------------------------------

func TestRegisterApp(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")
	ctx := context.Background()

	tests := []struct {
		name      string
		config    *AppConfiguration
		expectErr string
	}{
		{"valid", &AppConfiguration{
			AppID: "app-1", Name: "Catalogizer", Platforms: []string{"web", "android", "ios"},
		}, ""},
		{"missing app_id", &AppConfiguration{
			AppID: "", Name: "App", Platforms: []string{"web"},
		}, "app_id is required"},
		{"missing name", &AppConfiguration{
			AppID: "app-2", Name: "", Platforms: []string{"web"},
		}, "app name is required"},
		{"empty platforms", &AppConfiguration{
			AppID: "app-3", Name: "App", Platforms: []string{},
		}, "at least one platform is required"},
		{"nil platforms", &AppConfiguration{
			AppID: "app-4", Name: "App", Platforms: nil,
		}, "at least one platform is required"},
		{"single platform", &AppConfiguration{
			AppID: "web-only", Name: "Web Only", Platforms: []string{"web"},
		}, ""},
		{"full config", &AppConfiguration{
			AppID:     "full-app",
			Name:      "Full",
			Platforms: []string{"web", "android", "ios", "desktop"},
			Schemes:   map[string]string{"web": "https", "android": "catalogizer"},
			Packages:  map[string]string{"android": "com.catalogizer.app"},
			StoreURLs: map[string]string{
				"android": "https://play.google.com/store/apps/details?id=com.catalogizer.app",
			},
			MinVersions:   map[string]string{"android": "1.0.0"},
			Features:      []string{"video_playback"},
			PreferredApps: true,
			Active:        true,
		}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := svc.RegisterApp(ctx, tt.config)
			if tt.expectErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.expectErr) {
					t.Errorf("error = %q, want %q", err.Error(), tt.expectErr)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GetAppConfiguration
// ---------------------------------------------------------------------------

func TestGetAppConfiguration(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")
	ctx := context.Background()

	tests := []struct {
		name  string
		appID string
	}{
		{"standard", "catalogizer-main"},
		{"custom", "my-app"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := svc.GetAppConfiguration(ctx, tt.appID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg == nil {
				t.Fatal("expected non-nil config")
			}
			if cfg.AppID != tt.appID {
				t.Errorf("AppID = %q, want %q", cfg.AppID, tt.appID)
			}
			if cfg.Name == "" {
				t.Error("expected non-empty Name")
			}
			if len(cfg.Platforms) == 0 {
				t.Error("expected non-empty Platforms")
			}
			if len(cfg.Schemes) == 0 {
				t.Error("expected non-empty Schemes")
			}
			if len(cfg.Packages) == 0 {
				t.Error("expected non-empty Packages")
			}
			if len(cfg.StoreURLs) == 0 {
				t.Error("expected non-empty StoreURLs")
			}
			if len(cfg.MinVersions) == 0 {
				t.Error("expected non-empty MinVersions")
			}
			if len(cfg.Features) == 0 {
				t.Error("expected non-empty Features")
			}
			if !cfg.Active {
				t.Error("expected Active = true")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// GenerateSmartLink / determineRoutingStrategy
// ---------------------------------------------------------------------------

func TestDetermineRoutingStrategy(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	tests := []struct {
		name     string
		context  *LinkContext
		expected string
	}{
		{"nil context", nil, "universal"},
		{"android", &LinkContext{Platform: "android"}, "native_preferred"},
		{"ios", &LinkContext{Platform: "ios"}, "native_preferred"},
		{"web", &LinkContext{Platform: "web"}, "web_only"},
		{"desktop", &LinkContext{Platform: "desktop"}, "universal"},
		{"empty platform", &LinkContext{Platform: ""}, "universal"},
		{"unknown platform", &LinkContext{Platform: "vr"}, "universal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.determineRoutingStrategy(tt.context)
			if got != tt.expected {
				t.Errorf("strategy = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestGenerateSmartLink_RoutingStrategies(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")
	ctx := context.Background()

	tests := []struct {
		name             string
		context          *LinkContext
		expectedStrategy string
		expectFallbacks  bool
	}{
		{"nil context universal", nil, "universal", false},
		{"android native", &LinkContext{Platform: "android"}, "native_preferred", true},
		{"ios native", &LinkContext{Platform: "ios"}, "native_preferred", true},
		{"web only", &LinkContext{Platform: "web"}, "web_only", false},
		{"desktop universal", &LinkContext{Platform: "desktop"}, "universal", false},
		{"unknown universal", &LinkContext{Platform: "roku"}, "universal", false},
		{"empty universal", &LinkContext{Platform: ""}, "universal", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := svc.GenerateSmartLink(ctx, &DeepLinkRequest{
				MediaID: "smart-m1",
				Action:  "detail",
				Context: tt.context,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Strategy != tt.expectedStrategy {
				t.Errorf("strategy = %q, want %q", resp.Strategy, tt.expectedStrategy)
			}
			if resp.PrimaryLink == "" {
				t.Error("expected non-empty primary link")
			}
			if tt.expectFallbacks && len(resp.FallbackLinks) == 0 {
				t.Error("expected fallback links")
			}
			if resp.Instructions == nil {
				t.Error("expected non-nil instructions")
			}
			if resp.Instructions["primary"] == "" {
				t.Error("missing primary instruction")
			}
			if resp.Instructions["fallback"] == "" {
				t.Error("missing fallback instruction")
			}
		})
	}
}

func TestGenerateSmartLink_AndroidNativePreferred(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	resp, err := svc.GenerateSmartLink(context.Background(), &DeepLinkRequest{
		MediaID: "android-smart",
		Action:  "play",
		Context: &LinkContext{Platform: "android"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp.PrimaryLink, "catalogizer://") {
		t.Errorf("primary link should be android deep link, got %q", resp.PrimaryLink)
	}
	if len(resp.FallbackLinks) == 0 {
		t.Fatal("expected fallback links")
	}
	if !strings.HasPrefix(resp.FallbackLinks[0], "https://") {
		t.Errorf("fallback should be HTTPS, got %q", resp.FallbackLinks[0])
	}
}

func TestGenerateSmartLink_IOSNativePreferred(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	resp, err := svc.GenerateSmartLink(context.Background(), &DeepLinkRequest{
		MediaID: "ios-smart",
		Action:  "detail",
		Context: &LinkContext{Platform: "ios"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp.PrimaryLink, "catalogizer://") {
		t.Errorf("primary link should be iOS deep link, got %q", resp.PrimaryLink)
	}
	if len(resp.FallbackLinks) == 0 {
		t.Fatal("expected fallback links")
	}
	if !strings.HasPrefix(resp.FallbackLinks[0], "https://") {
		t.Errorf("fallback should be HTTPS, got %q", resp.FallbackLinks[0])
	}
}

func TestGenerateSmartLink_WebOnly(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	resp, err := svc.GenerateSmartLink(context.Background(), &DeepLinkRequest{
		MediaID: "web-smart",
		Action:  "detail",
		Context: &LinkContext{Platform: "web"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(resp.PrimaryLink, "https://") {
		t.Errorf("primary should be HTTPS, got %q", resp.PrimaryLink)
	}
	if len(resp.FallbackLinks) != 0 {
		t.Errorf("web_only should have no fallbacks, got %d", len(resp.FallbackLinks))
	}
}

func TestGenerateSmartLink_UniversalLink(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	resp, err := svc.GenerateSmartLink(context.Background(), &DeepLinkRequest{
		MediaID: "universal-test",
		Action:  "detail",
		Context: nil,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Strategy != "universal" {
		t.Errorf("strategy = %q, want 'universal'", resp.Strategy)
	}
	if !strings.Contains(resp.PrimaryLink, "/link/") {
		t.Errorf("primary link should contain '/link/', got %q", resp.PrimaryLink)
	}
	if !strings.Contains(resp.PrimaryLink, "universal-test") {
		t.Errorf("primary link should contain media ID, got %q", resp.PrimaryLink)
	}
}

// ---------------------------------------------------------------------------
// GenerateBatchLinks
// ---------------------------------------------------------------------------

func TestGenerateBatchLinks_MultipleValid(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	responses, err := svc.GenerateBatchLinks(context.Background(), []*DeepLinkRequest{
		{MediaID: "batch-1", Action: "detail"},
		{MediaID: "batch-2", Action: "play"},
		{MediaID: "batch-3", Action: "download"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 3 {
		t.Fatalf("expected 3, got %d", len(responses))
	}

	ids := make(map[string]bool)
	for _, r := range responses {
		if r.TrackingID == "" {
			t.Error("expected non-empty tracking ID")
		}
		ids[r.TrackingID] = true
	}
	if len(ids) != 3 {
		t.Error("expected unique tracking IDs")
	}
}

func TestGenerateBatchLinks_EmptyBatch(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	responses, err := svc.GenerateBatchLinks(context.Background(), []*DeepLinkRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 0 {
		t.Errorf("expected 0, got %d", len(responses))
	}
}

func TestGenerateBatchLinks_MixedValidInvalid(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	responses, err := svc.GenerateBatchLinks(context.Background(), []*DeepLinkRequest{
		{MediaID: "valid-1", Action: "detail"},
		{MediaID: "", Action: "detail"},
		{MediaID: "valid-2", Action: "play"},
		{MediaID: "", Action: "play"},
		{MediaID: "valid-3", Action: "download"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 3 {
		t.Errorf("expected 3 valid responses, got %d", len(responses))
	}
}

func TestGenerateBatchLinks_AllInvalid(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	responses, err := svc.GenerateBatchLinks(context.Background(), []*DeepLinkRequest{
		{MediaID: "", Action: "detail"},
		{MediaID: "", Action: "play"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 0 {
		t.Errorf("expected 0, got %d", len(responses))
	}
}

func TestGenerateBatchLinks_SingleItem(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	responses, err := svc.GenerateBatchLinks(context.Background(), []*DeepLinkRequest{
		{MediaID: "single", Action: "edit"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 1 {
		t.Fatalf("expected 1, got %d", len(responses))
	}
}

func TestGenerateBatchLinks_LargeBatch(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	requests := make([]*DeepLinkRequest, 50)
	for i := range requests {
		requests[i] = &DeepLinkRequest{
			MediaID: fmt.Sprintf("batch-large-%d", i),
			Action:  "detail",
		}
	}

	responses, err := svc.GenerateBatchLinks(context.Background(), requests)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(responses) != 50 {
		t.Errorf("expected 50, got %d", len(responses))
	}
}

func TestGenerateBatchLinks_PreservesOrder(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	responses, err := svc.GenerateBatchLinks(context.Background(), []*DeepLinkRequest{
		{MediaID: "first", Action: "detail"},
		{MediaID: "second", Action: "play"},
		{MediaID: "third", Action: "download"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []string{"first", "second", "third"}
	for i, resp := range responses {
		if !strings.Contains(resp.FallbackURL, expected[i]) {
			t.Errorf("response[%d] fallback %q should contain %q",
				i, resp.FallbackURL, expected[i])
		}
	}
}

// ---------------------------------------------------------------------------
// ValidateLinks / validateLink
// ---------------------------------------------------------------------------

func TestValidateLink(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	tests := []struct {
		name   string
		link   *DeepLink
		expect bool
	}{
		{"valid https", &DeepLink{URL: "https://example.com", Scheme: "https"}, true},
		{"valid custom scheme", &DeepLink{URL: "catalogizer://detail/m1", Scheme: "catalogizer"}, true},
		{"valid desktop scheme", &DeepLink{URL: "catalogizer-desktop://play/m1", Scheme: "catalogizer-desktop"}, true},
		{"empty URL", &DeepLink{URL: "", Scheme: "https"}, false},
		{"empty scheme", &DeepLink{URL: "https://example.com", Scheme: ""}, false},
		{"both empty", &DeepLink{URL: "", Scheme: ""}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := svc.validateLink(tt.link)
			if got != tt.expect {
				t.Errorf("validateLink() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestValidateLinks_AllValid(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	result, err := svc.ValidateLinks(context.Background(), []*DeepLink{
		{URL: "https://catalogizer.app/detail/m1", Scheme: "https"},
		{URL: "catalogizer://detail/m2", Scheme: "catalogizer"},
		{URL: "catalogizer-desktop://play/m3", Scheme: "catalogizer-desktop"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalLinks != 3 {
		t.Errorf("TotalLinks = %d, want 3", result.TotalLinks)
	}
	if result.ValidLinks != 3 {
		t.Errorf("ValidLinks = %d, want 3", result.ValidLinks)
	}
	if result.InvalidLinks != 0 {
		t.Errorf("InvalidLinks = %d, want 0", result.InvalidLinks)
	}
	if len(result.Errors) != 0 {
		t.Errorf("expected no errors, got %v", result.Errors)
	}
}

func TestValidateLinks_AllInvalid(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	result, err := svc.ValidateLinks(context.Background(), []*DeepLink{
		{URL: "", Scheme: "https"},
		{URL: "https://example.com", Scheme: ""},
		{URL: "", Scheme: ""},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalLinks != 3 {
		t.Errorf("TotalLinks = %d, want 3", result.TotalLinks)
	}
	if result.ValidLinks != 0 {
		t.Errorf("ValidLinks = %d, want 0", result.ValidLinks)
	}
	if result.InvalidLinks != 3 {
		t.Errorf("InvalidLinks = %d, want 3", result.InvalidLinks)
	}
	if len(result.Errors) != 3 {
		t.Errorf("expected 3 errors, got %d", len(result.Errors))
	}
}

func TestValidateLinks_Mixed(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	result, err := svc.ValidateLinks(context.Background(), []*DeepLink{
		{URL: "https://catalogizer.app/detail/m1", Scheme: "https"},
		{URL: "", Scheme: "https"},
		{URL: "catalogizer://detail/m2", Scheme: "catalogizer"},
		{URL: "https://example.com", Scheme: ""},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalLinks != 4 {
		t.Errorf("TotalLinks = %d, want 4", result.TotalLinks)
	}
	if result.ValidLinks != 2 {
		t.Errorf("ValidLinks = %d, want 2", result.ValidLinks)
	}
	if result.InvalidLinks != 2 {
		t.Errorf("InvalidLinks = %d, want 2", result.InvalidLinks)
	}
	if len(result.Errors) != 2 {
		t.Errorf("expected 2 errors, got %d", len(result.Errors))
	}
}

func TestValidateLinks_EmptySlice(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	result, err := svc.ValidateLinks(context.Background(), []*DeepLink{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalLinks != 0 {
		t.Errorf("TotalLinks = %d, want 0", result.TotalLinks)
	}
	if result.ValidLinks != 0 {
		t.Errorf("ValidLinks = %d, want 0", result.ValidLinks)
	}
	if result.InvalidLinks != 0 {
		t.Errorf("InvalidLinks = %d, want 0", result.InvalidLinks)
	}
}

func TestValidateLinks_ErrorMessageContent(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	result, err := svc.ValidateLinks(context.Background(), []*DeepLink{
		{URL: "", Scheme: "https"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d", len(result.Errors))
	}
	if !strings.Contains(result.Errors[0], "Invalid link") {
		t.Errorf("error = %q, should contain 'Invalid link'", result.Errors[0])
	}
}

func TestValidateLinks_WarningsAndErrorsInitialized(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	result, err := svc.ValidateLinks(context.Background(), []*DeepLink{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Warnings == nil {
		t.Error("Warnings should be initialized, not nil")
	}
	if result.Errors == nil {
		t.Error("Errors should be initialized, not nil")
	}
}

// ---------------------------------------------------------------------------
// Cross-platform consistency: autoplay and nil-context across generators
// ---------------------------------------------------------------------------

func TestNativePlatformLinks_PlayAutoplay(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	generators := []struct {
		name     string
		generate func(*DeepLinkRequest, string) (*DeepLink, error)
	}{
		{"android", svc.generateAndroidLink},
		{"ios", svc.generateIOSLink},
		{"desktop", svc.generateDesktopLink},
	}

	for _, g := range generators {
		t.Run(g.name+"_play", func(t *testing.T) {
			link, err := g.generate(&DeepLinkRequest{
				MediaID: "ap-test", Action: "play",
			}, "t")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if link.Parameters["autoplay"] != "true" {
				t.Error("play action should have autoplay=true")
			}
		})

		t.Run(g.name+"_detail_no_autoplay", func(t *testing.T) {
			link, err := g.generate(&DeepLinkRequest{
				MediaID: "noap", Action: "detail",
			}, "t")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if _, exists := link.Parameters["autoplay"]; exists {
				t.Error("detail should not have autoplay")
			}
		})
	}
}

func TestNativePlatformLinks_NilContextHandling(t *testing.T) {
	svc := NewDeepLinkingService("https://catalogizer.app", "v1")

	generators := []struct {
		name     string
		generate func(*DeepLinkRequest, string) (*DeepLink, error)
	}{
		{"android", svc.generateAndroidLink},
		{"ios", svc.generateIOSLink},
		{"desktop", svc.generateDesktopLink},
	}

	for _, g := range generators {
		t.Run(g.name+"_nil_context", func(t *testing.T) {
			link, err := g.generate(&DeepLinkRequest{
				MediaID: "nil-ctx", Action: "detail", Context: nil,
			}, "track_nil")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if link.Parameters["track"] != "track_nil" {
				t.Errorf("track = %q", link.Parameters["track"])
			}
			if _, exists := link.Parameters["user_id"]; exists {
				t.Error("user_id should not be present")
			}
			if _, exists := link.Parameters["session_id"]; exists {
				t.Error("session_id should not be present")
			}
		})
	}
}
