package tests

import (
	"context"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"catalogizer/internal/models"
	"catalogizer/internal/services"
)

func TestDeepLinkingService_GenerateDeepLinks(t *testing.T) {
	ctx := context.Background()
	deepLinkingService := services.NewDeepLinkingService("https://catalogizer.app", "v1")

	t.Run("basic deep link generation", func(t *testing.T) {
		req := &services.DeepLinkRequest{
			MediaID: "test_movie_123",
			Action:  "detail",
			MediaMetadata: &models.MediaMetadata{
				Title:     "Test Movie",
				MediaType: models.MediaTypeVideo,
			},
		}

		response, err := deepLinkingService.GenerateDeepLinks(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, response)

		// Verify response structure
		assert.NotEmpty(t, response.Links)
		assert.NotEmpty(t, response.UniversalLink)
		assert.NotEmpty(t, response.ShareableLink)
		assert.NotEmpty(t, response.TrackingID)
		assert.NotEmpty(t, response.SupportedApps)
		assert.NotEmpty(t, response.FallbackURL)

		// Verify all platforms are covered
		expectedPlatforms := []string{"web", "android", "ios", "desktop"}
		for _, platform := range expectedPlatforms {
			assert.Contains(t, response.Links, platform, "Should have link for platform: %s", platform)
		}

		// Verify tracking ID is included in all links
		for platform, link := range response.Links {
			assert.Contains(t, link.URL, response.TrackingID, "Platform %s link should contain tracking ID", platform)
		}
	})

	t.Run("different actions", func(t *testing.T) {
		actions := []string{"detail", "play", "download", "edit"}
		mediaID := "action_test_123"

		for _, action := range actions {
			req := &services.DeepLinkRequest{
				MediaID: mediaID,
				Action:  action,
				MediaMetadata: &models.MediaMetadata{
					Title:     "Action Test Movie",
					MediaType: models.MediaTypeVideo,
				},
			}

			response, err := deepLinkingService.GenerateDeepLinks(ctx, req)
			require.NoError(t, err, "Action %s should work", action)

			// Verify action is reflected in links
			for platform, link := range response.Links {
				assert.Contains(t, link.URL, action, "Platform %s link should contain action %s", platform, action)
			}

			// Verify auth requirements for sensitive actions
			if action == "edit" || action == "download" {
				for _, link := range response.Links {
					assert.True(t, link.RequiresAuth, "Action %s should require auth", action)
				}
			}
		}
	})

	t.Run("platform-specific links", func(t *testing.T) {
		req := &services.DeepLinkRequest{
			MediaID: "platform_test_123",
			Action:  "detail",
			MediaMetadata: &models.MediaMetadata{
				Title:     "Platform Test",
				MediaType: models.MediaTypeVideo,
			},
		}

		response, err := deepLinkingService.GenerateDeepLinks(ctx, req)
		require.NoError(t, err)

		// Test web link
		webLink := response.Links["web"]
		require.NotNil(t, webLink)
		assert.Equal(t, "https", webLink.Scheme)
		assert.Contains(t, webLink.URL, "catalogizer.app")
		assert.Contains(t, webLink.URL, req.MediaID)

		// Test Android link
		androidLink := response.Links["android"]
		require.NotNil(t, androidLink)
		assert.Equal(t, "catalogizer", androidLink.Scheme)
		assert.Equal(t, "com.catalogizer.app", androidLink.Package)
		assert.Contains(t, androidLink.StoreURL, "play.google.com")

		// Test iOS link
		iosLink := response.Links["ios"]
		require.NotNil(t, iosLink)
		assert.Equal(t, "catalogizer", iosLink.Scheme)
		assert.Equal(t, "com.catalogizer.app", iosLink.BundleID)
		assert.Contains(t, iosLink.StoreURL, "apps.apple.com")

		// Test desktop link
		desktopLink := response.Links["desktop"]
		require.NotNil(t, desktopLink)
		assert.Equal(t, "catalogizer-desktop", desktopLink.Scheme)
		assert.Contains(t, desktopLink.StoreURL, "github.com")
	})

	t.Run("context and UTM parameters", func(t *testing.T) {
		context := &services.LinkContext{
			UserID:       "user123",
			DeviceID:     "device456",
			SessionID:    "session789",
			ReferrerPage: "homepage",
			Platform:     "web",
			UTMParams: &services.UTMParameters{
				Source:   "newsletter",
				Medium:   "email",
				Campaign: "march_promo",
				Term:     "similar_movies",
				Content:  "recommendation",
			},
		}

		req := &services.DeepLinkRequest{
			MediaID: "utm_test_123",
			Action:  "detail",
			Context: context,
			MediaMetadata: &models.MediaMetadata{
				Title:     "UTM Test Movie",
				MediaType: models.MediaTypeVideo,
			},
		}

		response, err := deepLinkingService.GenerateDeepLinks(ctx, req)
		require.NoError(t, err)

		// Verify UTM parameters are included in web link
		webLink := response.Links["web"]
		parsedURL, err := url.Parse(webLink.URL)
		require.NoError(t, err)

		query := parsedURL.Query()
		assert.Equal(t, "user123", query.Get("user_id"))
		assert.Equal(t, "session789", query.Get("session_id"))
		assert.Equal(t, "homepage", query.Get("ref"))
		assert.Equal(t, "newsletter", query.Get("utm_source"))
		assert.Equal(t, "email", query.Get("utm_medium"))
		assert.Equal(t, "march_promo", query.Get("utm_campaign"))
		assert.Equal(t, "similar_movies", query.Get("utm_term"))
		assert.Equal(t, "recommendation", query.Get("utm_content"))

		// Verify context is included in mobile links
		androidLink := response.Links["android"]
		assert.Contains(t, androidLink.Parameters, "user_id")
		assert.Equal(t, "user123", androidLink.Parameters["user_id"])
	})

	t.Run("media type specific features", func(t *testing.T) {
		testCases := []struct {
			mediaType        string
			action           string
			expectedFeatures []string
		}{
			{
				mediaType:        models.MediaTypeVideo,
				action:           "play",
				expectedFeatures: []string{"video_playback", "fullscreen_video"},
			},
			{
				mediaType:        models.MediaTypeAudio,
				action:           "play",
				expectedFeatures: []string{"audio_playback", "background_audio"},
			},
			{
				mediaType:        models.MediaTypeBook,
				action:           "detail",
				expectedFeatures: []string{"pdf_reader", "epub_reader"},
			},
			{
				mediaType:        models.MediaTypeGame,
				action:           "detail",
				expectedFeatures: []string{"external_app_launch"},
			},
		}

		for _, tc := range testCases {
			req := &services.DeepLinkRequest{
				MediaID: "feature_test_" + tc.mediaType,
				Action:  tc.action,
				MediaMetadata: &models.MediaMetadata{
					Title:     "Feature Test",
					MediaType: tc.mediaType,
				},
			}

			response, err := deepLinkingService.GenerateDeepLinks(ctx, req)
			require.NoError(t, err)

			// Check that mobile links have required features
			for _, platform := range []string{"android", "ios"} {
				link := response.Links[platform]
				for _, expectedFeature := range tc.expectedFeatures {
					assert.Contains(t, link.Features, expectedFeature,
						"Platform %s should have feature %s for media type %s", platform, expectedFeature, tc.mediaType)
				}
			}
		}
	})

	t.Run("expiration for temporary actions", func(t *testing.T) {
		temporaryActions := []string{"play", "download"}

		for _, action := range temporaryActions {
			req := &services.DeepLinkRequest{
				MediaID: "temp_test_123",
				Action:  action,
				MediaMetadata: &models.MediaMetadata{
					Title:     "Temporary Action Test",
					MediaType: models.MediaTypeVideo,
				},
			}

			response, err := deepLinkingService.GenerateDeepLinks(ctx, req)
			require.NoError(t, err)

			assert.NotNil(t, response.ExpiresAt, "Action %s should have expiration", action)
			assert.True(t, response.ExpiresAt.After(time.Now()), "Expiration should be in the future")
		}

		// Detail action should not have expiration
		req := &services.DeepLinkRequest{
			MediaID: "detail_test_123",
			Action:  "detail",
			MediaMetadata: &models.MediaMetadata{
				Title:     "Detail Test",
				MediaType: models.MediaTypeVideo,
			},
		}

		response, err := deepLinkingService.GenerateDeepLinks(ctx, req)
		require.NoError(t, err)
		assert.Nil(t, response.ExpiresAt, "Detail action should not have expiration")
	})
}

func TestDeepLinkingService_SmartLinking(t *testing.T) {
	ctx := context.Background()
	deepLinkingService := services.NewDeepLinkingService("https://catalogizer.app", "v1")

	t.Run("native app preferred strategy", func(t *testing.T) {
		contexts := []*services.LinkContext{
			{Platform: "android"},
			{Platform: "ios"},
		}

		for _, linkContext := range contexts {
			req := &services.DeepLinkRequest{
				MediaID: "smart_test_123",
				Action:  "detail",
				Context: linkContext,
				MediaMetadata: &models.MediaMetadata{
					Title:     "Smart Link Test",
					MediaType: models.MediaTypeVideo,
				},
			}

			response, err := deepLinkingService.GenerateSmartLink(ctx, req)
			require.NoError(t, err)

			assert.Equal(t, "native_preferred", response.Strategy)
			assert.NotEmpty(t, response.PrimaryLink)
			assert.NotEmpty(t, response.FallbackLinks)

			// Primary link should be native app link
			if linkContext.Platform == "android" {
				assert.Contains(t, response.PrimaryLink, "catalogizer://")
			} else if linkContext.Platform == "ios" {
				assert.Contains(t, response.PrimaryLink, "catalogizer://")
			}

			// Fallback should be web link
			assert.True(t, len(response.FallbackLinks) > 0)
			assert.Contains(t, response.FallbackLinks[0], "https://")
		}
	})

	t.Run("web-only strategy", func(t *testing.T) {
		req := &services.DeepLinkRequest{
			MediaID: "web_test_123",
			Action:  "detail",
			Context: &services.LinkContext{
				Platform: "web",
			},
			MediaMetadata: &models.MediaMetadata{
				Title:     "Web Test",
				MediaType: models.MediaTypeVideo,
			},
		}

		response, err := deepLinkingService.GenerateSmartLink(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, "web_only", response.Strategy)
		assert.NotEmpty(t, response.PrimaryLink)
		assert.Contains(t, response.PrimaryLink, "https://")
		assert.Empty(t, response.FallbackLinks)
	})

	t.Run("universal strategy", func(t *testing.T) {
		req := &services.DeepLinkRequest{
			MediaID: "universal_test_123",
			Action:  "detail",
			Context: nil, // No context
			MediaMetadata: &models.MediaMetadata{
				Title:     "Universal Test",
				MediaType: models.MediaTypeVideo,
			},
		}

		response, err := deepLinkingService.GenerateSmartLink(ctx, req)
		require.NoError(t, err)

		assert.Equal(t, "universal", response.Strategy)
		assert.NotEmpty(t, response.PrimaryLink)
		assert.Contains(t, response.PrimaryLink, "/link/")
	})
}

func TestDeepLinkingService_TrackingAndAnalytics(t *testing.T) {
	ctx := context.Background()
	deepLinkingService := services.NewDeepLinkingService("https://catalogizer.app", "v1")

	t.Run("track link events", func(t *testing.T) {
		event := &services.LinkTrackingEvent{
			TrackingID:   "track_123",
			EventType:    "click",
			Platform:     "android",
			UserAgent:    "Mozilla/5.0 (Android)",
			IPAddress:    "192.168.1.100",
			Timestamp:    time.Now(),
			Success:      true,
			AppOpened:    true,
			FallbackUsed: false,
		}

		err := deepLinkingService.TrackLinkEvent(ctx, event)
		assert.NoError(t, err)
	})

	t.Run("get link analytics is honestly unavailable", func(t *testing.T) {
		analytics, err := deepLinkingService.GetLinkAnalytics(ctx, "analytics_test_123")
		assert.ErrorIs(t, err, services.ErrLinkAnalyticsNotImplemented)
		assert.Nil(t, analytics)
	})
}

func TestDeepLinkingService_BatchOperations(t *testing.T) {
	ctx := context.Background()
	deepLinkingService := services.NewDeepLinkingService("https://catalogizer.app", "v1")

	t.Run("batch link generation", func(t *testing.T) {
		requests := []*services.DeepLinkRequest{
			{
				MediaID: "batch_1",
				Action:  "detail",
				MediaMetadata: &models.MediaMetadata{
					Title:     "Batch Test 1",
					MediaType: models.MediaTypeVideo,
				},
			},
			{
				MediaID: "batch_2",
				Action:  "play",
				MediaMetadata: &models.MediaMetadata{
					Title:     "Batch Test 2",
					MediaType: models.MediaTypeAudio,
				},
			},
			{
				MediaID: "batch_3",
				Action:  "download",
				MediaMetadata: &models.MediaMetadata{
					Title:     "Batch Test 3",
					MediaType: models.MediaTypeBook,
				},
			},
		}

		responses, err := deepLinkingService.GenerateBatchLinks(ctx, requests)
		require.NoError(t, err)

		assert.Len(t, responses, len(requests))

		for i, response := range responses {
			assert.NotEmpty(t, response.Links)
			assert.NotEmpty(t, response.TrackingID)
			assert.Contains(t, response.UniversalLink, requests[i].MediaID)
		}
	})

	t.Run("batch with partial failures", func(t *testing.T) {
		requests := []*services.DeepLinkRequest{
			{
				MediaID: "valid_1",
				Action:  "detail",
				MediaMetadata: &models.MediaMetadata{
					Title:     "Valid Test 1",
					MediaType: models.MediaTypeVideo,
				},
			},
			{
				MediaID: "", // Invalid - empty media ID
				Action:  "detail",
				MediaMetadata: &models.MediaMetadata{
					Title:     "Invalid Test",
					MediaType: models.MediaTypeVideo,
				},
			},
			{
				MediaID: "valid_2",
				Action:  "detail",
				MediaMetadata: &models.MediaMetadata{
					Title:     "Valid Test 2",
					MediaType: models.MediaTypeAudio,
				},
			},
		}

		responses, err := deepLinkingService.GenerateBatchLinks(ctx, requests)
		require.NoError(t, err)

		// Should have responses for valid requests only
		assert.True(t, len(responses) < len(requests))

		for _, response := range responses {
			assert.NotEmpty(t, response.Links)
			assert.NotEmpty(t, response.TrackingID)
		}
	})
}

func TestDeepLinkingService_Validation(t *testing.T) {
	ctx := context.Background()
	deepLinkingService := services.NewDeepLinkingService("https://catalogizer.app", "v1")

	t.Run("validate links", func(t *testing.T) {
		links := []*services.DeepLink{
			{
				URL:    "https://catalogizer.app/detail/123",
				Scheme: "https",
				Parameters: map[string]string{
					"media_id": "123",
				},
			},
			{
				URL:    "catalogizer://detail/456",
				Scheme: "catalogizer",
				Parameters: map[string]string{
					"media_id": "456",
				},
			},
			{
				URL:    "", // Invalid - empty URL
				Scheme: "https",
			},
			{
				URL:    "invalid-url",
				Scheme: "", // Invalid - empty scheme
			},
		}

		result, err := deepLinkingService.ValidateLinks(ctx, links)
		require.NoError(t, err)
		require.NotNil(t, result)

		assert.Equal(t, len(links), result.TotalLinks)
		assert.Equal(t, 2, result.ValidLinks)   // First two are valid
		assert.Equal(t, 2, result.InvalidLinks) // Last two are invalid
		assert.Len(t, result.Errors, 2)
	})
}

func TestDeepLinkingService_AppConfiguration(t *testing.T) {
	ctx := context.Background()
	deepLinkingService := services.NewDeepLinkingService("https://catalogizer.app", "v1")

	t.Run("register app configuration", func(t *testing.T) {
		config := &services.AppConfiguration{
			AppID:     "test_app",
			Name:      "Test Catalogizer App",
			Platforms: []string{"android", "ios"},
			Schemes: map[string]string{
				"android": "test-catalogizer",
				"ios":     "test-catalogizer",
			},
			Packages: map[string]string{
				"android": "com.test.catalogizer",
				"ios":     "com.test.catalogizer",
			},
			StoreURLs: map[string]string{
				"android": "https://play.google.com/store/apps/details?id=com.test.catalogizer",
				"ios":     "https://apps.apple.com/app/id999999999",
			},
			Features:      []string{"video_playback", "audio_playback"},
			PreferredApps: false,
			Active:        true,
		}

		err := deepLinkingService.RegisterApp(ctx, config)
		assert.NoError(t, err)
	})

	t.Run("register app validation", func(t *testing.T) {
		invalidConfigs := []*services.AppConfiguration{
			{
				// Missing AppID
				Name:      "Test App",
				Platforms: []string{"android"},
			},
			{
				AppID: "test_app",
				// Missing Name
				Platforms: []string{"android"},
			},
			{
				AppID: "test_app",
				Name:  "Test App",
				// Missing Platforms
			},
		}

		for _, config := range invalidConfigs {
			err := deepLinkingService.RegisterApp(ctx, config)
			assert.Error(t, err, "Invalid config should return error")
		}
	})

	t.Run("get app configuration", func(t *testing.T) {
		config, err := deepLinkingService.GetAppConfiguration(ctx, "catalogizer")
		require.NoError(t, err)
		require.NotNil(t, config)

		assert.Equal(t, "catalogizer", config.AppID)
		assert.Equal(t, "Catalogizer", config.Name)
		assert.NotEmpty(t, config.Platforms)
		assert.NotEmpty(t, config.Schemes)
		assert.True(t, config.Active)
	})
}

func TestDeepLinkingService_EdgeCases(t *testing.T) {
	ctx := context.Background()
	deepLinkingService := services.NewDeepLinkingService("", "") // Empty base URL

	t.Run("empty base URL", func(t *testing.T) {
		req := &services.DeepLinkRequest{
			MediaID: "empty_url_test",
			Action:  "detail",
			MediaMetadata: &models.MediaMetadata{
				Title:     "Empty URL Test",
				MediaType: models.MediaTypeVideo,
			},
		}

		response, err := deepLinkingService.GenerateDeepLinks(ctx, req)
		require.NoError(t, err)

		// Should use default base URL
		webLink := response.Links["web"]
		assert.Contains(t, webLink.URL, "catalogizer.app")
	})

	t.Run("special characters in media ID", func(t *testing.T) {
		specialMediaIDs := []string{
			"media with spaces",
			"media-with-dashes",
			"media_with_underscores",
			"media.with.dots",
			"media/with/slashes",
			"媒体中文",
			"médiä-spéciål",
		}

		for _, mediaID := range specialMediaIDs {
			req := &services.DeepLinkRequest{
				MediaID: mediaID,
				Action:  "detail",
				MediaMetadata: &models.MediaMetadata{
					Title:     "Special Chars Test",
					MediaType: models.MediaTypeVideo,
				},
			}

			response, err := deepLinkingService.GenerateDeepLinks(ctx, req)
			require.NoError(t, err, "Should handle special characters in media ID: %s", mediaID)

			// Links should be properly encoded
			for platform, link := range response.Links {
				_, parseErr := url.Parse(link.URL)
				assert.NoError(t, parseErr, "Platform %s link should be valid URL for media ID: %s", platform, mediaID)
			}
		}
	})

	t.Run("long media ID", func(t *testing.T) {
		longMediaID := strings.Repeat("a", 1000) // Very long ID

		req := &services.DeepLinkRequest{
			MediaID: longMediaID,
			Action:  "detail",
			MediaMetadata: &models.MediaMetadata{
				Title:     "Long ID Test",
				MediaType: models.MediaTypeVideo,
			},
		}

		response, err := deepLinkingService.GenerateDeepLinks(ctx, req)
		require.NoError(t, err)

		// Should handle long IDs gracefully
		assert.NotEmpty(t, response.Links)
		assert.NotEmpty(t, response.UniversalLink)
	})

	t.Run("invalid action", func(t *testing.T) {
		req := &services.DeepLinkRequest{
			MediaID: "invalid_action_test",
			Action:  "invalid_action",
			MediaMetadata: &models.MediaMetadata{
				Title:     "Invalid Action Test",
				MediaType: models.MediaTypeVideo,
			},
		}

		response, err := deepLinkingService.GenerateDeepLinks(ctx, req)
		require.NoError(t, err)

		// Should default to "detail" action
		for _, link := range response.Links {
			assert.Contains(t, link.URL, "detail")
		}
	})

	t.Run("nil context", func(t *testing.T) {
		req := &services.DeepLinkRequest{
			MediaID: "nil_context_test",
			Action:  "detail",
			Context: nil, // Nil context
			MediaMetadata: &models.MediaMetadata{
				Title:     "Nil Context Test",
				MediaType: models.MediaTypeVideo,
			},
		}

		response, err := deepLinkingService.GenerateDeepLinks(ctx, req)
		require.NoError(t, err)

		// Should handle nil context gracefully
		assert.NotEmpty(t, response.Links)
		assert.NotEmpty(t, response.UniversalLink)
	})
}

func BenchmarkDeepLinkingService(b *testing.B) {
	ctx := context.Background()
	deepLinkingService := services.NewDeepLinkingService("https://catalogizer.app", "v1")

	req := &services.DeepLinkRequest{
		MediaID: "benchmark_test",
		Action:  "detail",
		MediaMetadata: &models.MediaMetadata{
			Title:     "Benchmark Test",
			MediaType: models.MediaTypeVideo,
		},
		Context: &services.LinkContext{
			UserID:   "user123",
			Platform: "android",
			UTMParams: &services.UTMParameters{
				Source:   "test",
				Medium:   "benchmark",
				Campaign: "performance",
			},
		},
	}

	b.Run("generate deep links", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := deepLinkingService.GenerateDeepLinks(ctx, req)
			if err != nil {
				b.Fatalf("Benchmark failed: %v", err)
			}
		}
	})

	b.Run("generate smart link", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := deepLinkingService.GenerateSmartLink(ctx, req)
			if err != nil {
				b.Fatalf("Benchmark failed: %v", err)
			}
		}
	})
}
