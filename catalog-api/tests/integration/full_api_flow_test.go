//go:build e2e_binary

package integration

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFullAPIFlow exercises real API endpoints end-to-end.
func TestFullAPIFlow(t *testing.T) {
	sb := spawnBinary(t)
	defer sb.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	// --- Step 1: Health check (no auth required) ---
	t.Run("HealthCheck", func(t *testing.T) {
		resp, err := client.Get(sb.baseURL + "/api/v1/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assertJSONContentType(t, resp)

		body := readBody(t, resp)
		data := parseJSON(t, body)

		assert.Equal(t, "healthy", data["status"])
		assert.NotNil(t, data["time"], "health response should include time")
		assert.NotNil(t, data["version"], "health response should include version")
	})

	// --- Step 2: Login ---
	var authToken string
	t.Run("Login", func(t *testing.T) {
		loginBody := strings.NewReader(`{"username":"admin","password":"` + testAdminPassword + `"}`)
		resp, err := client.Post(sb.baseURL+"/api/v1/auth/login", "application/json", loginBody)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assertJSONContentType(t, resp)

		body := readBody(t, resp)
		data := parseJSON(t, body)

		token, ok := data["session_token"].(string)
		require.True(t, ok, "login response should contain a session_token string")
		assert.NotEmpty(t, token, "session_token should not be empty")
		authToken = token

		user, ok := data["user"].(map[string]interface{})
		assert.True(t, ok, "login response should contain a user object")
		assert.NotNil(t, user["username"], "user should have a username")
	})

	// Build auth headers for subsequent requests
	authHeaders := func(req *http.Request) {
		req.Header.Set("Authorization", "Bearer "+authToken)
		req.Header.Set("Content-Type", "application/json")
	}

	// --- Steps 3-8: Table-driven authenticated endpoint tests ---
	type endpointTest struct {
		name           string
		method         string
		path           string
		expectedStatus int
		expectedFields []string
	}

	tests := []endpointTest{
		{
			name:           "ListStorageRoots",
			method:         "GET",
			path:           "/api/v1/storage-roots",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"roots"},
		},
		{
			name:           "BrowseRecentMedia",
			method:         "GET",
			path:           "/api/v1/media/recent",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"items", "total"},
		},
		{
			name:           "BrowsePopularMedia",
			method:         "GET",
			path:           "/api/v1/media/popular",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"items", "total"},
		},
		{
			name:           "SearchMedia",
			method:         "GET",
			path:           "/api/v1/media/search?q=movie",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"results", "total", "query"},
		},
		{
			name:           "GetMediaStats",
			method:         "GET",
			path:           "/api/v1/media/stats",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"total_items", "total_size", "by_type"},
		},
		{
			name:           "GetAuthMe",
			method:         "GET",
			path:           "/api/v1/auth/me",
			expectedStatus: http.StatusOK,
			expectedFields: []string{"id", "username", "role"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(tt.method, sb.baseURL+tt.path, nil)
			require.NoError(t, err)
			authHeaders(req)

			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.expectedStatus, resp.StatusCode,
				"endpoint %s should return status %d", tt.path, tt.expectedStatus)
			assertJSONContentType(t, resp)

			body := readBody(t, resp)
			data := parseJSON(t, body)

			for _, field := range tt.expectedFields {
				assert.NotNil(t, data[field],
					"response from %s should contain field %q", tt.path, field)
			}
		})
	}
}

// TestFullAPIFlow_InvalidAuth verifies that protected endpoints reject
// unauthenticated requests.
func TestFullAPIFlow_InvalidAuth(t *testing.T) {
	sb := spawnBinary(t)
	defer sb.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	protectedEndpoints := []string{
		"/api/v1/auth/me",
		"/api/v1/storage-roots",
	}

	for _, endpoint := range protectedEndpoints {
		t.Run("Unauthorized_"+endpoint, func(t *testing.T) {
			resp, err := client.Get(sb.baseURL + endpoint)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"endpoint %s should reject unauthenticated requests", endpoint)
			assertJSONContentType(t, resp)

			body := readBody(t, resp)
			data := parseJSON(t, body)
			assert.NotNil(t, data["error"], "error response should contain 'error' field")
		})
	}
}

// TestFullAPIFlow_InvalidLogin verifies that login rejects bad credentials.
func TestFullAPIFlow_InvalidLogin(t *testing.T) {
	sb := spawnBinary(t)
	defer sb.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	tests := []struct {
		name     string
		username string
		password string
	}{
		{"WrongPassword", "admin", "wrongpassword"},
		{"WrongUsername", "nonexistent", testAdminPassword},
		{"BothWrong", "nobody", "nothing"},
		{"EmptyCredentials", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loginBody := fmt.Sprintf(`{"username":%q,"password":%q}`, tt.username, tt.password)
			resp, err := client.Post(
				sb.baseURL+"/api/v1/auth/login",
				"application/json",
				strings.NewReader(loginBody),
			)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
				"login with invalid credentials should return 401")
			assertJSONContentType(t, resp)

			body := readBody(t, resp)
			data := parseJSON(t, body)
			assert.NotNil(t, data["error"], "error response should contain 'error' field")
		})
	}
}

// TestFullAPIFlow_SearchValidation verifies search endpoint input validation.
func TestFullAPIFlow_SearchValidation(t *testing.T) {
	sb := spawnBinary(t)
	defer sb.Close()

	token := sb.login()
	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("MissingQuery", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, sb.baseURL+"/api/v1/media/search", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// The real API accepts empty queries and returns all results.
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("EmptyQuery", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, sb.baseURL+"/api/v1/media/search?q=", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		// The real API accepts empty queries and returns all results.
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("ValidQuery", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, sb.baseURL+"/api/v1/media/search?q=test", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body := readBody(t, resp)
		data := parseJSON(t, body)
		assert.NotNil(t, data["results"], "response should contain results")
	})
}

// TestFullAPIFlow_NonExistentEndpoint verifies 404 handling.
func TestFullAPIFlow_NonExistentEndpoint(t *testing.T) {
	sb := spawnBinary(t)
	defer sb.Close()

	token := sb.login()
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest(http.MethodGet, sb.baseURL+"/api/v1/does-not-exist", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestFullAPIFlow_ResponseTimes verifies that all endpoints respond within
// a reasonable timeframe.
func TestFullAPIFlow_ResponseTimes(t *testing.T) {
	sb := spawnBinary(t)
	defer sb.Close()

	token := sb.login()
	client := &http.Client{Timeout: 10 * time.Second}

	endpoints := []string{
		"/api/v1/health",
		"/api/v1/storage-roots",
		"/api/v1/media/recent",
		"/api/v1/media/popular",
		"/api/v1/media/search?q=test",
		"/api/v1/media/stats",
	}

	for _, endpoint := range endpoints {
		t.Run(endpoint, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, sb.baseURL+endpoint, nil)
			require.NoError(t, err)
			if endpoint != "/api/v1/health" {
				req.Header.Set("Authorization", "Bearer "+token)
			}

			start := time.Now()
			resp, err := client.Do(req)
			duration := time.Since(start)

			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Less(t, duration, 5*time.Second,
				"endpoint %s should respond within 5 seconds", endpoint)
			assert.Less(t, resp.StatusCode, 500,
				"endpoint %s should not return a server error", endpoint)
		})
	}
}

// --- Helper functions ---

// assertJSONContentType verifies the response Content-Type contains application/json.
func assertJSONContentType(t *testing.T, resp *http.Response) {
	t.Helper()
	contentType := resp.Header.Get("Content-Type")
	assert.Contains(t, contentType, "application/json",
		"Content-Type should be application/json, got: %s", contentType)
}

// readBody reads and returns the full response body. Fails the test on error.
func readBody(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err, "failed to read response body")
	return body
}

// parseJSON unmarshals a JSON byte slice into a map. Fails the test on error.
func parseJSON(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var data map[string]interface{}
	err := json.Unmarshal(body, &data)
	require.NoError(t, err, "response body should be valid JSON: %s", string(body))
	return data
}
