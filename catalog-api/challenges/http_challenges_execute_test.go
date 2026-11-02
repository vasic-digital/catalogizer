package challenges

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"digital.vasic.challenges/pkg/challenge"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAPIServer creates a test server that simulates the Catalogizer API.
// It handles health, login, auth/me, and various other endpoints.
func mockAPIServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	mux.HandleFunc("/api/v1/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})

	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"session_token":"test-jwt-token"}`)
	})

	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"unauthorized"}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":1,"username":"admin","role":"admin"}`)
	})

	mux.HandleFunc("/api/v1/storage/roots", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"roots":[{"id":1,"path":"/media"}]}`)
	})

	mux.HandleFunc("/api/v1/entities", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"items":[]}`)
	})

	mux.HandleFunc("/api/v1/stats/overall", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"total_files":100}`)
	})

	// New endpoints added to match updated challenge definitions
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "go_goroutines 10\n")
	})

	mux.HandleFunc("/api/v1/storage-roots", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"roots":[]}`)
	})

	mux.HandleFunc("/api/v1/catalog", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"items":[]}`)
	})

	mux.HandleFunc("/api/v1/media/stats", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"total":0}`)
	})

	mux.HandleFunc("/api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"results":[]}`)
	})

	mux.HandleFunc("/api/v1/files", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"files":[]}`)
	})

	mux.HandleFunc("/api/v1/assets/request", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"asset_id":"test-asset-123"}`)
	})

	mux.HandleFunc("/api/v1/assets/test-asset-123", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("X-Asset-Status", "ready")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte{0x89, 0x50, 0x4E, 0x47}) // PNG magic bytes
	})

	mux.HandleFunc("/api/v1/assets/by-entity/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `[{"asset_id":"test-asset-123"}]`)
	})

	mux.HandleFunc("/api/v1/media/search", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"files":[{"id":1,"name":"test.mp4"}]}`)
	})

	mux.HandleFunc("/api/v1/collections", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"collections":[]}`)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Catch-all for unhandled endpoints: return error without sensitive data
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"not found"}`)
	})

	return httptest.NewServer(mux)
}

// --- CH-036: Auth Required Execute Test ---

func TestAuthRequiredChallenge_Execute_MockServer(t *testing.T) {
	srv := mockAPIServer(t)
	defer srv.Close()

	ch := &AuthRequiredChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"auth-required", "Auth Required on Protected Endpoints",
			"test", "security", []challenge.ID{"browsing-api-health"},
		),
		config: &BrowsingConfig{
			BaseURL:  srv.URL,
			Username: "admin",
			Password: testAdminPassword,
		},
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
	assert.GreaterOrEqual(t, len(result.Assertions), 7) // 2 public + 5 protected + login + auth access
}

// --- CH-039: CORS Headers Execute Test ---

func TestCORSHeadersChallenge_Execute_MockServer(t *testing.T) {
	srv := mockAPIServer(t)
	defer srv.Close()

	ch := &CORSHeadersChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"cors-headers", "CORS Headers",
			"test", "security", []challenge.ID{"browsing-api-health"},
		),
		config: &BrowsingConfig{BaseURL: srv.URL},
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
	assert.GreaterOrEqual(t, len(result.Assertions), 4)
}

func TestCORSHeadersChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := &CORSHeadersChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"cors-headers", "CORS Headers",
			"test", "security", nil,
		),
		config: &BrowsingConfig{BaseURL: "http://127.0.0.1:1"},
	}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
}

// --- CH-041: Health Latency Execute Test ---

func TestHealthLatencyChallenge_Execute_MockServer(t *testing.T) {
	srv := mockAPIServer(t)
	defer srv.Close()

	ch := &HealthLatencyChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"health-latency", "Health Endpoint Latency",
			"test", "performance", []challenge.ID{"browsing-api-health"},
		),
		config: &BrowsingConfig{BaseURL: srv.URL},
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
	assert.GreaterOrEqual(t, len(result.Assertions), 2)
	require.Contains(t, result.Metrics, "health_avg_latency")
}

// --- CH-012: Asset Serving Execute Test ---

func TestAssetServingChallenge_Execute_MockServer(t *testing.T) {
	srv := mockAPIServer(t)
	defer srv.Close()

	ch := &AssetServingChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"asset-serving", "Asset Serving",
			"test", "e2e", []challenge.ID{"browsing-api-health"},
		),
		config: &BrowsingConfig{
			BaseURL:  srv.URL,
			Username: "admin",
			Password: testAdminPassword,
		},
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
	assert.GreaterOrEqual(t, len(result.Assertions), 5)
}

// --- CH-013: Asset Lazy Loading Execute Test ---

func TestAssetLazyLoadingChallenge_Execute_MockServer(t *testing.T) {
	srv := mockAPIServer(t)
	defer srv.Close()

	ch := &AssetLazyLoadingChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"asset-lazy-loading", "Asset Lazy Loading",
			"test", "e2e", []challenge.ID{"asset-serving"},
		),
		config: &BrowsingConfig{
			BaseURL:  srv.URL,
			Username: "admin",
			Password: testAdminPassword,
		},
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
	assert.GreaterOrEqual(t, len(result.Assertions), 5)
}

// --- CH-040: No Sensitive Errors Execute Test ---

func TestNoSensitiveErrorsChallenge_Execute_MockServer(t *testing.T) {
	// Create a server that returns safe errors (no stack traces, no paths)
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"session_token":"test-jwt"}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"not found"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ch := NewNoSensitiveErrorsChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  srv.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	// Should pass because our mock doesn't leak sensitive data
	assert.GreaterOrEqual(t, len(result.Assertions), 1)
}

// --- Unreachable server tests for HTTP challenges ---

func TestAuthRequiredChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := &AuthRequiredChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"auth-required", "Auth Required",
			"test", "security", nil,
		),
		config: &BrowsingConfig{
			BaseURL:  "http://127.0.0.1:1",
			Username: "admin",
			Password: testAdminPassword,
		},
	}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
}

func TestHealthLatencyChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := &HealthLatencyChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"health-latency", "Health Latency",
			"test", "performance", nil,
		),
		config: &BrowsingConfig{BaseURL: "http://127.0.0.1:1"},
	}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
}

func TestAssetServingChallenge_Execute_LoginFails(t *testing.T) {
	// Server that rejects login
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"bad credentials"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ch := &AssetServingChallenge{
		BaseChallenge: challenge.NewBaseChallenge(
			"asset-serving", "Asset Serving",
			"test", "e2e", nil,
		),
		config: &BrowsingConfig{
			BaseURL:  srv.URL,
			Username: "bad",
			Password: "bad",
		},
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
}

// --- Mock server response validation ---

func TestMockAPIServer_Routes(t *testing.T) {
	srv := mockAPIServer(t)
	defer srv.Close()

	tests := []struct {
		method     string
		path       string
		auth       bool
		wantStatus int
	}{
		{"GET", "/health", false, 200},
		{"GET", "/api/v1/health", false, 200},
		{"POST", "/api/v1/auth/login", false, 200},
		{"GET", "/api/v1/auth/me", false, 401},
		{"GET", "/api/v1/auth/me", true, 200},
		{"GET", "/api/v1/storage/roots", false, 401},
		{"GET", "/api/v1/storage/roots", true, 200},
		{"GET", "/api/v1/entities", false, 401},
		{"GET", "/api/v1/entities", true, 200},
		{"GET", "/api/v1/stats/overall", false, 401},
		{"GET", "/api/v1/files", false, 401},
		{"GET", "/nonexistent", false, 404},
	}

	for _, tt := range tests {
		name := fmt.Sprintf("%s_%s_auth=%v", tt.method, tt.path, tt.auth)
		t.Run(name, func(t *testing.T) {
			req, _ := http.NewRequest(tt.method, srv.URL+tt.path, nil)
			if tt.auth {
				req.Header.Set("Authorization", "Bearer test-token")
			}
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			assert.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}

// --- Storage Roots API Execute Test ---

func TestStorageRootsAPIChallenge_Execute_MockServer(t *testing.T) {
	srv := mockAPIServer(t)
	defer srv.Close()

	ch := NewStorageRootsAPIChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  srv.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(result.Assertions), 1) // Login assertion at minimum
}

// --- Collections API Execute Test ---

func TestCollectionsAPIChallenge_Execute_MockServer(t *testing.T) {
	srv := mockAPIServer(t)
	defer srv.Close()

	ch := NewCollectionsAPIChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  srv.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(result.Assertions), 2)
}

// --- Helper function test: endpoint config JSON parsing ---

func TestEndpointConfig_JSON_MultipleEndpoints(t *testing.T) {
	data := `{
		"endpoints": [
			{"id":"nas1","name":"NAS1","host":"192.168.1.1","port":445,"share":"data","directories":[{"path":"Music","content_type":"music"}]},
			{"id":"nas2","name":"NAS2","host":"192.168.1.2","port":445,"share":"media","directories":[{"path":"Movies","content_type":"movie"},{"path":"TV","content_type":"tv_show"}]}
		]
	}`

	var cfg EndpointConfig
	err := json.Unmarshal([]byte(data), &cfg)
	require.NoError(t, err)
	assert.Len(t, cfg.Endpoints, 2)
	assert.Equal(t, "nas1", cfg.Endpoints[0].ID)
	assert.Len(t, cfg.Endpoints[0].Directories, 1)
	assert.Equal(t, "nas2", cfg.Endpoints[1].ID)
	assert.Len(t, cfg.Endpoints[1].Directories, 2)
}

func TestEndpointConfig_JSON_EmptyEndpoints(t *testing.T) {
	data := `{"endpoints":[]}`

	var cfg EndpointConfig
	err := json.Unmarshal([]byte(data), &cfg)
	require.NoError(t, err)
	assert.Len(t, cfg.Endpoints, 0)
}

func TestEndpointConfig_JSON_InvalidJSON(t *testing.T) {
	var cfg EndpointConfig
	err := json.Unmarshal([]byte(`{invalid}`), &cfg)
	assert.Error(t, err)
}

// --- CH-037: JWT Expiration Execute Test ---

func TestJWTExpirationChallenge_Execute_MockServer(t *testing.T) {
	// Custom mock that includes expires_at in login response
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"session_token":"test-jwt-token","expires_at":"2099-12-31T23:59:59Z"}`)
	})
	mux.HandleFunc("/api/v1/auth/me", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"unauthorized"}`)
			return
		}
		// Reject tampered or empty tokens
		token := strings.TrimPrefix(auth, "Bearer ")
		if token == "" || strings.HasSuffix(token, "tampered") {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"invalid token"}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"id":1,"username":"admin","role":"admin"}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"not found"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ch := NewJWTExpirationChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  srv.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
	// 5 assertions: login, expires_at_present, valid_token_access,
	// tampered_token_rejected, empty_token_rejected
	assert.GreaterOrEqual(t, len(result.Assertions), 5)
	for _, a := range result.Assertions {
		assert.True(t, a.Passed, "assertion %q failed: %s", a.Target, a.Message)
	}
	require.Contains(t, result.Metrics, "jwt_validation_time")
}

func TestJWTExpirationChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewJWTExpirationChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  "http://127.0.0.1:1",
		Username: "admin",
		Password: testAdminPassword,
	}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
}

// --- CH-038: Rate Limit Auth Execute Test ---

func TestRateLimitAuthChallenge_Execute_MockServer(t *testing.T) {
	// Custom mock: valid creds succeed, invalid creds return 401
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Read the body to check credentials
		var body map[string]string
		if decErr := json.NewDecoder(r.Body).Decode(&body); decErr != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"bad request"}`)
			return
		}
		if body["username"] == "admin" && body["password"] == testAdminPassword {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `{"session_token":"test-jwt-token"}`)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"invalid credentials"}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"not found"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ch := NewRateLimitAuthChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  srv.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
	// 4 assertions: auth_reachable, no_server_errors,
	// rate_limit_active, post_rate_limit_login
	assert.GreaterOrEqual(t, len(result.Assertions), 4)
	for _, a := range result.Assertions {
		assert.True(t, a.Passed, "assertion %q failed: %s", a.Target, a.Message)
	}
	require.Contains(t, result.Metrics, "rate_limited_count")
	require.Contains(t, result.Metrics, "total_rapid_requests")
}

func TestRateLimitAuthChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewRateLimitAuthChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  "http://127.0.0.1:1",
		Username: "admin",
		Password: testAdminPassword,
	}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
}

// --- CH-042: File Listing Latency Execute Test ---

func TestFileListingLatencyChallenge_Execute_MockServer(t *testing.T) {
	srv := mockAPIServer(t)
	defer srv.Close()

	ch := NewFileListingLatencyChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  srv.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
	// 2 assertions: file_listing_success_rate, file_listing_avg_latency
	assert.GreaterOrEqual(t, len(result.Assertions), 2)
	require.Contains(t, result.Metrics, "file_listing_avg_latency")
	require.Contains(t, result.Metrics, "file_listing_success_rate")
	require.Contains(t, result.Metrics, "file_listing_total_requests")
}

func TestFileListingLatencyChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewFileListingLatencyChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  "http://127.0.0.1:1",
		Username: "admin",
		Password: testAdminPassword,
	}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
}

// --- CH-043: Entity Search Latency Execute Test ---

func TestEntitySearchLatencyChallenge_Execute_MockServer(t *testing.T) {
	srv := mockAPIServer(t)
	defer srv.Close()

	ch := NewEntitySearchLatencyChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  srv.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
	// 2 assertions: search_success_rate, entity_search_avg_latency
	assert.GreaterOrEqual(t, len(result.Assertions), 2)
	require.Contains(t, result.Metrics, "entity_search_avg_latency")
	require.Contains(t, result.Metrics, "entity_search_max_latency")
}

func TestEntitySearchLatencyChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewEntitySearchLatencyChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  "http://127.0.0.1:1",
		Username: "admin",
		Password: testAdminPassword,
	}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
}

// --- CH-044: WebSocket Latency Execute Test (login-failure path only) ---

func TestWebSocketLatencyChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewWebSocketLatencyChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  "http://127.0.0.1:1",
		Username: "admin",
		Password: testAdminPassword,
	}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
}

// --- CH-048: DB Error Recovery Execute Test ---

func TestDBErrorRecoveryChallenge_Execute_MockServer(t *testing.T) {
	srv := mockAPIServer(t)
	defer srv.Close()

	ch := NewDBErrorRecoveryChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  srv.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
	// 4 assertions: pre_health_check, post_health_check,
	// post_stress_entities, post_stress_login
	assert.GreaterOrEqual(t, len(result.Assertions), 4)
	for _, a := range result.Assertions {
		assert.True(t, a.Passed, "assertion %q failed: %s", a.Target, a.Message)
	}
	require.Contains(t, result.Metrics, "recovery_test_time")
}

func TestDBErrorRecoveryChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewDBErrorRecoveryChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  "http://127.0.0.1:1",
		Username: "admin",
		Password: testAdminPassword,
	}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
}

// --- CH-049: Scanner Recovery Execute Test ---

func TestScannerRecoveryChallenge_Execute_MockServer(t *testing.T) {
	// Custom mock that handles POST to /api/v1/storage/roots
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	})
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"session_token":"test-jwt-token"}`)
	})
	mux.HandleFunc("/api/v1/storage/roots", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.Method == http.MethodPost {
			// Accept the bad root gracefully (400 or 200 — both non-5xx)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, `{"error":"path does not exist"}`)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"roots":[{"id":1,"path":"/media"}]}`)
	})
	mux.HandleFunc("/api/v1/entities", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"items":[]}`)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"not found"}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ch := NewScannerRecoveryChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  srv.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
	// 4 assertions: pre_health, create_bad_root, post_health, post_scan_entities
	assert.GreaterOrEqual(t, len(result.Assertions), 4)
	for _, a := range result.Assertions {
		assert.True(t, a.Passed, "assertion %q failed: %s", a.Target, a.Message)
	}
	require.Contains(t, result.Metrics, "scanner_recovery_time")
}

func TestScannerRecoveryChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewScannerRecoveryChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  "http://127.0.0.1:1",
		Username: "admin",
		Password: testAdminPassword,
	}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
}

// --- CH-050: Graceful Shutdown Execute Test ---

func TestGracefulShutdownChallenge_Execute_MockServer(t *testing.T) {
	srv := mockAPIServer(t)
	defer srv.Close()

	ch := NewGracefulShutdownChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  srv.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusPassed, result.Status)
	// 5 assertions: health_endpoint, http_protocol, content_type_header,
	// concurrent_connections, keep_alive_connections
	assert.GreaterOrEqual(t, len(result.Assertions), 5)
	for _, a := range result.Assertions {
		assert.True(t, a.Passed, "assertion %q failed: %s", a.Target, a.Message)
	}
	require.Contains(t, result.Metrics, "shutdown_readiness_time")
}

func TestGracefulShutdownChallenge_Execute_Unreachable(t *testing.T) {
	ch := NewGracefulShutdownChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  "http://127.0.0.1:1",
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.Equal(t, challenge.StatusFailed, result.Status)
}
