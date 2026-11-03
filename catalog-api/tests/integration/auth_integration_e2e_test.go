//go:build e2e_binary

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// E2E TEST: Auth flow against real spawned binary
// =============================================================================

func TestAuthIntegration_LoginFlowViaHTTP(t *testing.T) {
	sb := spawnBinary(t, "REGISTRATION_MODE=open")
	defer sb.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	t.Run("RegisterLoginAccessProtectedLogout", func(t *testing.T) {
		timestamp := time.Now().UnixNano()
		username := fmt.Sprintf("auth_flow_%d", timestamp)

		// Step 1: Register
		registerData := map[string]interface{}{
			"username":   username,
			"email":      fmt.Sprintf("%s@example.com", username),
			"password":   "Str0ngP@ssword!",
			"first_name": "Test",
			"last_name":  "User",
		}
		body, _ := json.Marshal(registerData)
		resp, err := client.Post(sb.baseURL+"/api/v1/auth/register", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		resp.Body.Close()
		require.Equal(t, http.StatusCreated, resp.StatusCode,
			"Registration should succeed")

		// Step 2: Login with new credentials
		loginData := map[string]interface{}{
			"username": username,
			"password": "Str0ngP@ssword!",
		}
		body, _ = json.Marshal(loginData)
		resp, err = client.Post(sb.baseURL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
		require.NoError(t, err)

		var loginResult map[string]interface{}
		decodeJSONE2E(t, resp, &loginResult)
		require.NotEmpty(t, loginResult["session_token"],
			"Login should return a session_token")

		token := loginResult["session_token"].(string)

		// Step 3: Access protected resource
		req, err := http.NewRequest(http.MethodGet, sb.baseURL+"/api/v1/auth/me", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err = client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"Should access protected endpoint with valid token")

		// Step 4: Logout
		req, err = http.NewRequest(http.MethodPost, sb.baseURL+"/api/v1/auth/logout", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err = client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"Logout should succeed")
	})

	t.Run("InvalidCredentialsRejected", func(t *testing.T) {
		loginData := map[string]interface{}{
			"username": "nonexistent_user",
			"password": "wrong_password",
		}
		body, _ := json.Marshal(loginData)
		resp, err := client.Post(sb.baseURL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("EmptyTokenRejected", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, sb.baseURL+"/api/v1/auth/me", nil)
		require.NoError(t, err)

		resp, err := client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("InvalidTokenRejected", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodGet, sb.baseURL+"/api/v1/auth/me", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer invalid-token-value-that-does-not-exist")

		resp, err := client.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

func TestAuthIntegration_TokenUniqueness(t *testing.T) {
	sb := spawnBinary(t)
	defer sb.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	tokens := make(map[string]bool)
	var mu sync.Mutex

	loginCount := 20
	for i := 0; i < loginCount; i++ {
		loginData := map[string]interface{}{
			"username": "admin",
			"password": testAdminPassword,
		}
		body, _ := json.Marshal(loginData)
		resp, err := client.Post(sb.baseURL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
		require.NoError(t, err)

		var result map[string]interface{}
		decodeJSONE2E(t, resp, &result)
		token := result["session_token"].(string)

		mu.Lock()
		assert.False(t, tokens[token],
			"Token should be unique, got duplicate: %s", token)
		tokens[token] = true
		mu.Unlock()
	}
	assert.Equal(t, loginCount, len(tokens),
		"Each login should produce a unique token")
}

func TestAuthIntegration_PasswordHashNotExposed(t *testing.T) {
	sb := spawnBinary(t)
	defer sb.Close()

	client := &http.Client{Timeout: 10 * time.Second}
	token := sb.login()

	req, err := http.NewRequest(http.MethodGet, sb.baseURL+"/api/v1/auth/me", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	require.NoError(t, err)

	var result map[string]interface{}
	decodeJSONE2E(t, resp, &result)

	data, ok := result["data"].(map[string]interface{})
	if !ok {
		data = result
	}

	_, hasPassword := data["password"]
	_, hasPasswordHash := data["password_hash"]
	_, hasSalt := data["salt"]

	assert.False(t, hasPassword,
		"Password should not be in API response")
	assert.False(t, hasPasswordHash,
		"Password hash should not be in API response")
	assert.False(t, hasSalt,
		"Salt should not be in API response")
}

func TestAuthIntegration_ConcurrentLogins(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping concurrent login integration test in short mode") 
	}

	sb := spawnBinary(t)
	defer sb.Close()

	client := &http.Client{Timeout: 10 * time.Second}

	concurrentLogins := 20
	var wg sync.WaitGroup
	var successCount atomic.Int64
	var mu sync.Mutex
	tokens := make([]string, 0, concurrentLogins)

	for i := 0; i < concurrentLogins; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loginData := map[string]interface{}{
				"username": "admin",
				"password": testAdminPassword,
			}
			body, _ := json.Marshal(loginData)
			resp, err := client.Post(sb.baseURL+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
			if err != nil {
				return
			}

			if resp.StatusCode == http.StatusOK {
				successCount.Add(1)
				var result map[string]interface{}
				if decodeJSONSafeE2E(resp, &result) == nil {
					if token, ok := result["session_token"].(string); ok {
						mu.Lock()
						tokens = append(tokens, token)
						mu.Unlock()
					}
				}
			} else {
				resp.Body.Close()
			}
		}()
	}

	wg.Wait()
	assert.Equal(t, int64(concurrentLogins), successCount.Load(),
		"All concurrent logins should succeed")
	assert.Equal(t, concurrentLogins, len(tokens),
		"Each login should produce a token")
}

// =============================================================================
// E2E helpers (local to this file to avoid polluting the integration package)
// =============================================================================

// decodeJSONE2E reads the response body and decodes it into dest, closing the body.
func decodeJSONE2E(t *testing.T, resp *http.Response, dest interface{}) {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(body, dest))
}

// decodeJSONSafeE2E decodes resp body into dest without requiring *testing.T.
func decodeJSONSafeE2E(resp *http.Response, dest interface{}) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dest)
}
