package challenges

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shortCtx returns a context with a 3-second timeout for unreachable endpoint tests.
func shortCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 3*time.Second)
}

// setupCH051MockServer creates a mock API server for input validation tests
func setupCH051MockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"healthy"}`)
	})
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Sanitized response - no XSS reflection
		fmt.Fprint(w, `{"results":[],"query":"sanitized"}`)
	})
	mux.HandleFunc("/catalog/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"files":[],"total":0}`)
	})
	return httptest.NewServer(mux)
}

func TestInputValidationChallenge_Execute_MockServer(t *testing.T) {
	server := setupCH051MockServer()
	defer server.Close()

	ch := NewInputValidationChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  server.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestInputValidationChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewInputValidationChallenge()
	ch.config = &BrowsingConfig{BaseURL: "http://127.0.0.1:1"}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNewInputValidationChallenge(t *testing.T) {
	ch := NewInputValidationChallenge()
	assert.Equal(t, "input-validation", string(ch.ID()))
	assert.Equal(t, "Input Validation and Sanitization", ch.Name())
	assert.Equal(t, "security", ch.Category())
}

// CH-052 Tests

func setupCH052MockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"session_token":"test-jwt-token"}`)
	})
	mux.HandleFunc("/catalog/files", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"files":[],"total":0}`)
	})
	mux.HandleFunc("/entities", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[],"total":0}`)
	})
	mux.HandleFunc("/storage-roots", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[]}`)
	})
	return httptest.NewServer(mux)
}

func TestPaginationChallenge_Execute_MockServer(t *testing.T) {
	server := setupCH052MockServer()
	defer server.Close()

	ch := NewPaginationChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  server.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestPaginationChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewPaginationChallenge()
	ch.config = &BrowsingConfig{BaseURL: "http://127.0.0.1:1"}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNewPaginationChallenge(t *testing.T) {
	ch := NewPaginationChallenge()
	assert.Equal(t, "pagination", string(ch.ID()))
	assert.Equal(t, "api", ch.Category())
}

// CH-053 Tests

func setupCH053MockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"healthy"}`)
	})
	mux.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":"unauthorized"}`)
	})
	return httptest.NewServer(mux)
}

func TestContentTypesChallenge_Execute_MockServer(t *testing.T) {
	server := setupCH053MockServer()
	defer server.Close()

	ch := NewContentTypesChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  server.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestContentTypesChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewContentTypesChallenge()
	ch.config = &BrowsingConfig{BaseURL: "http://127.0.0.1:1"}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNewContentTypesChallenge(t *testing.T) {
	ch := NewContentTypesChallenge()
	assert.Equal(t, "content-types", string(ch.ID()))
	assert.Equal(t, "api", ch.Category())
}

// CH-054 Tests

func setupCH054MockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"session_token":"test-jwt-token"}`)
	})
	mux.HandleFunc("/users/me", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":1,"username":"admin","email":"admin@test.com"}`)
	})
	mux.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			w.WriteHeader(http.StatusUnauthorized)
			fmt.Fprint(w, `{"error":"unauthorized"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"users":[{"id":1,"username":"admin"}],"total":1}`)
	})
	mux.HandleFunc("/auth/init-status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"initialized":true}`)
	})
	return httptest.NewServer(mux)
}

func TestUserManagementChallenge_Execute_MockServer(t *testing.T) {
	server := setupCH054MockServer()
	defer server.Close()

	ch := NewUserManagementChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  server.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestUserManagementChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewUserManagementChallenge()
	ch.config = &BrowsingConfig{BaseURL: "http://127.0.0.1:1"}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNewUserManagementChallenge(t *testing.T) {
	ch := NewUserManagementChallenge()
	assert.Equal(t, "user-management", string(ch.ID()))
	assert.Equal(t, "api", ch.Category())
}

// CH-055 Tests

func setupCH055MockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"session_token":"test-jwt-token"}`)
	})
	mux.HandleFunc("/stats/overall", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"total_files":100,"total_size":1024}`)
	})
	mux.HandleFunc("/stats/duplicates", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"duplicate_count":5}`)
	})
	mux.HandleFunc("/stats/media-types", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"types":{"movie":10,"tv_show":20}}`)
	})
	mux.HandleFunc("/stats/scan-history", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"scans":[]}`)
	})
	return httptest.NewServer(mux)
}

func TestAnalyticsAPIChallenge_Execute_MockServer(t *testing.T) {
	server := setupCH055MockServer()
	defer server.Close()

	ch := NewAnalyticsAPIChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  server.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestAnalyticsAPIChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewAnalyticsAPIChallenge()
	ch.config = &BrowsingConfig{BaseURL: "http://127.0.0.1:1"}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNewAnalyticsAPIChallenge(t *testing.T) {
	ch := NewAnalyticsAPIChallenge()
	assert.Equal(t, "analytics-api", string(ch.ID()))
	assert.Equal(t, "api", ch.Category())
}

// CH-056 Tests

func setupCH056MockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"session_token":"test-jwt-token"}`)
	})
	mux.HandleFunc("/entities", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"items":[{"id":1,"title":"Test Movie"}],"total":1}`)
	})
	mux.HandleFunc("/entities/99999999", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"not found"}`)
	})
	mux.HandleFunc("/media-types", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"types":["movie","tv_show","music_album"]}`)
	})
	return httptest.NewServer(mux)
}

func TestEntityCRUDChallenge_Execute_MockServer(t *testing.T) {
	server := setupCH056MockServer()
	defer server.Close()

	ch := NewEntityCRUDChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  server.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestEntityCRUDChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewEntityCRUDChallenge()
	ch.config = &BrowsingConfig{BaseURL: "http://127.0.0.1:1"}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNewEntityCRUDChallenge(t *testing.T) {
	ch := NewEntityCRUDChallenge()
	assert.Equal(t, "entity-crud", string(ch.ID()))
	assert.Equal(t, "api", ch.Category())
}

// CH-057 Tests

func setupCH057MockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"session_token":"test-jwt-token"}`)
	})
	mux.HandleFunc("/sync/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"status":"idle","last_sync":null}`)
	})
	mux.HandleFunc("/sync/history", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"sessions":[]}`)
	})
	mux.HandleFunc("/sync/devices", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"devices":[]}`)
	})
	mux.HandleFunc("/sync/conflicts", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"conflicts":[]}`)
	})
	return httptest.NewServer(mux)
}

func TestSyncAPIChallenge_Execute_MockServer(t *testing.T) {
	server := setupCH057MockServer()
	defer server.Close()

	ch := NewSyncAPIChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  server.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestSyncAPIChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewSyncAPIChallenge()
	ch.config = &BrowsingConfig{BaseURL: "http://127.0.0.1:1"}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNewSyncAPIChallenge(t *testing.T) {
	ch := NewSyncAPIChallenge()
	assert.Equal(t, "sync-api", string(ch.ID()))
	assert.Equal(t, "api", ch.Category())
}

// CH-058 Tests

func setupCH058MockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"session_token":"test-jwt-token"}`)
	})
	mux.HandleFunc("/subtitles", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"subtitles":[]}`)
	})
	mux.HandleFunc("/subtitles/languages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"languages":["en","es","fr","de"]}`)
	})
	mux.HandleFunc("/subtitles/99999999", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"not found"}`)
	})
	return httptest.NewServer(mux)
}

func TestSubtitleAPIChallenge_Execute_MockServer(t *testing.T) {
	server := setupCH058MockServer()
	defer server.Close()

	ch := NewSubtitleAPIChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  server.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestSubtitleAPIChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewSubtitleAPIChallenge()
	ch.config = &BrowsingConfig{BaseURL: "http://127.0.0.1:1"}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNewSubtitleAPIChallenge(t *testing.T) {
	ch := NewSubtitleAPIChallenge()
	assert.Equal(t, "subtitle-api", string(ch.ID()))
	assert.Equal(t, "api", ch.Category())
}

// CH-059 Tests

func setupCH059MockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"session_token":"test-jwt-token"}`)
	})
	mux.HandleFunc("/recommendations", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"recommendations":[]}`)
	})
	mux.HandleFunc("/entities/99999999/similar", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":"entity not found"}`)
	})
	return httptest.NewServer(mux)
}

func TestRecommendationAPIChallenge_Execute_MockServer(t *testing.T) {
	server := setupCH059MockServer()
	defer server.Close()

	ch := NewRecommendationAPIChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  server.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestRecommendationAPIChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewRecommendationAPIChallenge()
	ch.config = &BrowsingConfig{BaseURL: "http://127.0.0.1:1"}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNewRecommendationAPIChallenge(t *testing.T) {
	ch := NewRecommendationAPIChallenge()
	assert.Equal(t, "recommendation-api", string(ch.ID()))
	assert.Equal(t, "api", ch.Category())
}

// CH-060 Tests

func setupCH060MockServer() *httptest.Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/auth/login", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"session_token":"test-jwt-token"}`)
	})
	mux.HandleFunc("/localization/languages", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"languages":["en","es","fr","de","ru","sr"]}`)
	})
	mux.HandleFunc("/localization/translations", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"locale":"en","translations":{"hello":"Hello"}}`)
	})
	mux.HandleFunc("/localization/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"total_languages":6,"total_keys":100}`)
	})
	return httptest.NewServer(mux)
}

func TestLocalizationAPIChallenge_Execute_MockServer(t *testing.T) {
	server := setupCH060MockServer()
	defer server.Close()

	ch := NewLocalizationAPIChallenge()
	ch.config = &BrowsingConfig{
		BaseURL:  server.URL,
		Username: "admin",
		Password: testAdminPassword,
	}

	result, err := ch.Execute(context.Background())
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Assertions)
}

func TestLocalizationAPIChallenge_Execute_Unreachable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping unreachable endpoint test in short mode")
	}
	ch := NewLocalizationAPIChallenge()
	ch.config = &BrowsingConfig{BaseURL: "http://127.0.0.1:1"}

	ctx, cancel := shortCtx()
	defer cancel()
	result, err := ch.Execute(ctx)
	require.NoError(t, err)
	assert.NotNil(t, result)
}

func TestNewLocalizationAPIChallenge(t *testing.T) {
	ch := NewLocalizationAPIChallenge()
	assert.Equal(t, "localization-api", string(ch.ID()))
	assert.Equal(t, "api", ch.Category())
}

// Test helper function
func TestGetMapKeys(t *testing.T) {
	m := map[string]interface{}{
		"key1": "val1",
		"key2": "val2",
		"key3": "val3",
	}
	keys := getMapKeys(m)
	assert.Len(t, keys, 3)
	assert.Contains(t, keys, "key1")
	assert.Contains(t, keys, "key2")
	assert.Contains(t, keys, "key3")
}
