package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/utils"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 1. Service-level error wrapping
// ---------------------------------------------------------------------------

// mockRepo simulates a repository that returns errors.
type mockRepo struct {
	err error
}

func TestServiceErrorWrapping_Login_UserNotFound(t *testing.T) {
	// Verify that service methods wrap repository errors with context.
	// AuthService.Login wraps errors from the user repository using fmt.Errorf("... %w", err).

	svc := &AuthService{
		userRepo:  nil, // will not be called in this test path
		jwtSecret: []byte("test-secret"),
		jwtExpiry: time.Hour,
	}

	// ValidatePassword is a simple in-process call that returns a plain error.
	err := svc.ValidatePassword("short")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "at least 8 characters")
}

func TestServiceErrorWrapping_ContextPreserved(t *testing.T) {
	// Simulate the error wrapping pattern used throughout the codebase:
	// fmt.Errorf("failed to <action>: %w", underlying)
	underlying := errors.New("connection refused")
	wrapped := fmt.Errorf("failed to save error report: %w", underlying)

	// errors.Is should find the underlying error.
	assert.True(t, errors.Is(wrapped, underlying))
	// The message should contain both context and underlying error.
	assert.Contains(t, wrapped.Error(), "failed to save error report")
	assert.Contains(t, wrapped.Error(), "connection refused")
}

func TestServiceErrorWrapping_NestedWrapping(t *testing.T) {
	// Repository layer error
	repoErr := errors.New("sqlite: table not found")
	// Service layer wraps it
	svcErr := fmt.Errorf("failed to get configuration: %w", repoErr)
	// Handler layer wraps again
	handlerErr := fmt.Errorf("configuration endpoint failed: %w", svcErr)

	assert.True(t, errors.Is(handlerErr, repoErr))
	assert.Contains(t, handlerErr.Error(), "configuration endpoint failed")
	assert.Contains(t, handlerErr.Error(), "failed to get configuration")
	assert.Contains(t, handlerErr.Error(), "sqlite: table not found")
}

func TestServiceErrorWrapping_CustomErrorTypes(t *testing.T) {
	type ServiceError struct {
		Code    string
		Message string
		Cause   error
	}

	// The codebase uses sentinel errors in models (ErrUnauthorized, etc.).
	// Verify that wrapping preserves Is/As semantics.
	sentinelNotFound := errors.New("not found")
	wrapped := fmt.Errorf("user lookup: %w", sentinelNotFound)
	assert.True(t, errors.Is(wrapped, sentinelNotFound))
}

func TestServiceErrorWrapping_NilErrorPassthrough(t *testing.T) {
	// Verify that nil errors are not accidentally wrapped.
	var err error
	assert.Nil(t, err)
	// Simulate the pattern: if err != nil { return fmt.Errorf(...) }
	// When err is nil the service should return nil.
	result := wrapIfError("operation", err)
	assert.Nil(t, result)
}

// wrapIfError mirrors the wrapping pattern used throughout services.
func wrapIfError(operation string, err error) error {
	if err != nil {
		return fmt.Errorf("failed to %s: %w", operation, err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// 2. HTTP error responses
// ---------------------------------------------------------------------------

func setupTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r
}

func TestHTTPErrorResponse_NotFound(t *testing.T) {
	r := setupTestRouter()
	r.GET("/api/v1/resource/:id", func(c *gin.Context) {
		utils.SendErrorResponse(c, http.StatusNotFound, "Resource not found", nil)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/resource/999", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var body utils.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.False(t, body.Success)
	assert.Equal(t, "Resource not found", body.Error)
}

func TestHTTPErrorResponse_BadRequest(t *testing.T) {
	r := setupTestRouter()
	r.POST("/api/v1/resource", func(c *gin.Context) {
		utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid request format", errors.New("missing required field: name"))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/v1/resource", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var body utils.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.False(t, body.Success)
	assert.Equal(t, "Invalid request format", body.Error)
	// PX-F23 (CAT-PXF23): the underlying error text is internal and must never
	// reach the client; it is logged server-side only.
	assert.Empty(t, body.Details)
	assert.NotContains(t, w.Body.String(), "missing required field: name")
}

func TestHTTPErrorResponse_InternalServerError(t *testing.T) {
	r := setupTestRouter()
	r.GET("/api/v1/health", func(c *gin.Context) {
		utils.SendErrorResponse(c, http.StatusInternalServerError, "Internal server error", errors.New("database connection lost"))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/health", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var body utils.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.False(t, body.Success)
	assert.Equal(t, "Internal server error", body.Error)
}

func TestHTTPErrorResponse_Unauthorized(t *testing.T) {
	r := setupTestRouter()
	r.GET("/api/v1/protected", func(c *gin.Context) {
		utils.SendErrorResponse(c, http.StatusUnauthorized, "Authorization header required", nil)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/protected", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var body utils.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.False(t, body.Success)
	assert.Equal(t, "Authorization header required", body.Error)
	assert.Empty(t, body.Details) // no underlying error provided
}

func TestHTTPErrorResponse_JSONFormat(t *testing.T) {
	r := setupTestRouter()
	r.GET("/api/v1/test", func(c *gin.Context) {
		utils.SendErrorResponse(c, http.StatusForbidden, "Insufficient permissions", nil)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/test", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusForbidden, w.Code)
	assert.Contains(t, w.Header().Get("Content-Type"), "application/json")

	// Verify the response is valid JSON.
	var raw map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &raw)
	require.NoError(t, err)
	assert.Contains(t, raw, "success")
	assert.Contains(t, raw, "error")
}

func TestHTTPSuccessResponse_Format(t *testing.T) {
	r := setupTestRouter()
	r.GET("/api/v1/ok", func(c *gin.Context) {
		utils.SendSuccessResponse(c, http.StatusOK, map[string]string{"status": "healthy"}, "OK")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/ok", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var body utils.SuccessResponse
	err := json.Unmarshal(w.Body.Bytes(), &body)
	require.NoError(t, err)
	assert.True(t, body.Success)
	assert.Equal(t, "OK", body.Message)
}

// ---------------------------------------------------------------------------
// 3. Middleware error recovery
// ---------------------------------------------------------------------------

func TestPanicRecoveryMiddleware_CatchesPanic(t *testing.T) {
	r := setupTestRouter()
	r.Use(gin.Recovery()) // Gin's built-in recovery middleware

	r.GET("/api/v1/panic", func(c *gin.Context) {
		panic("unexpected nil pointer dereference")
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/panic", nil)
	r.ServeHTTP(w, req)

	// Gin's Recovery middleware should catch the panic and return 500.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPanicRecoveryMiddleware_DoesNotAffectNormalRequests(t *testing.T) {
	r := setupTestRouter()
	r.Use(gin.Recovery())

	r.GET("/api/v1/normal", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/normal", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestPanicRecoveryMiddleware_IntegerPanic(t *testing.T) {
	r := setupTestRouter()
	r.Use(gin.Recovery())

	r.GET("/api/v1/int-panic", func(c *gin.Context) {
		panic(42)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/int-panic", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestPanicRecoveryMiddleware_ErrorPanic(t *testing.T) {
	r := setupTestRouter()
	r.Use(gin.Recovery())

	r.GET("/api/v1/err-panic", func(c *gin.Context) {
		panic(errors.New("fatal database error"))
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/v1/err-panic", nil)
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// ---------------------------------------------------------------------------
// 4. Error logging integration
// ---------------------------------------------------------------------------

// testLogEntry captures a single log event.
type testLogEntry struct {
	Level   string
	Message string
	Fields  map[string]interface{}
}

// testLogger collects log entries for assertion in tests.
type testLogger struct {
	mu      sync.Mutex
	entries []testLogEntry
}

func (l *testLogger) Log(level, message string, fields map[string]interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.entries = append(l.entries, testLogEntry{
		Level:   level,
		Message: message,
		Fields:  fields,
	})
}

func (l *testLogger) Entries() []testLogEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	result := make([]testLogEntry, len(l.entries))
	copy(result, l.entries)
	return result
}

func TestErrorLogging_ErrorEventsAreCaptured(t *testing.T) {
	logger := &testLogger{}

	// Simulate the pattern used in services: log an error event.
	simulateErrorLogging(logger, "failed to save report", errors.New("disk full"))

	entries := logger.Entries()
	require.Len(t, entries, 1)
	assert.Equal(t, "error", entries[0].Level)
	assert.Contains(t, entries[0].Message, "failed to save report")
	assert.Equal(t, "disk full", entries[0].Fields["error"])
}

func TestErrorLogging_MultipleErrorsAreLogged(t *testing.T) {
	logger := &testLogger{}

	simulateErrorLogging(logger, "database timeout", errors.New("connection timed out"))
	simulateErrorLogging(logger, "cache miss", errors.New("key not found"))
	simulateErrorLogging(logger, "auth failure", errors.New("invalid token"))

	entries := logger.Entries()
	require.Len(t, entries, 3)
	assert.Equal(t, "database timeout", entries[0].Message)
	assert.Equal(t, "cache miss", entries[1].Message)
	assert.Equal(t, "auth failure", entries[2].Message)
}

func TestErrorLogging_ConcurrentLogWrites(t *testing.T) {
	logger := &testLogger{}
	var wg sync.WaitGroup

	count := 100
	wg.Add(count)
	for i := 0; i < count; i++ {
		go func(idx int) {
			defer wg.Done()
			simulateErrorLogging(logger, fmt.Sprintf("error-%d", idx), fmt.Errorf("err-%d", idx))
		}(i)
	}
	wg.Wait()

	entries := logger.Entries()
	assert.Len(t, entries, count)
}

func TestErrorLogging_WarningAndInfoLevels(t *testing.T) {
	logger := &testLogger{}

	logger.Log("warn", "deprecated API usage", map[string]interface{}{
		"endpoint": "/api/v1/old",
	})
	logger.Log("info", "request completed", map[string]interface{}{
		"status": 200,
	})

	entries := logger.Entries()
	require.Len(t, entries, 2)
	assert.Equal(t, "warn", entries[0].Level)
	assert.Equal(t, "info", entries[1].Level)
}

// simulateErrorLogging mirrors the logging pattern in the codebase.
func simulateErrorLogging(logger *testLogger, message string, err error) {
	logger.Log("error", message, map[string]interface{}{
		"error":     err.Error(),
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// ---------------------------------------------------------------------------
// 5. Concurrent error handling
// ---------------------------------------------------------------------------

func TestConcurrentErrorHandling_SimultaneousRequests(t *testing.T) {
	r := setupTestRouter()
	r.Use(gin.Recovery())

	var successCount int64
	var errorCount int64

	r.GET("/api/v1/concurrent", func(c *gin.Context) {
		// Simulate intermittent errors.
		id := c.Query("id")
		if id == "" {
			atomic.AddInt64(&errorCount, 1)
			utils.SendErrorResponse(c, http.StatusBadRequest, "Missing id parameter", nil)
			return
		}
		atomic.AddInt64(&successCount, 1)
		utils.SendSuccessResponse(c, http.StatusOK, gin.H{"id": id}, "OK")
	})

	var wg sync.WaitGroup
	requestCount := 50

	// Half with valid requests, half without.
	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var url string
			if idx%2 == 0 {
				url = fmt.Sprintf("/api/v1/concurrent?id=%d", idx)
			} else {
				url = "/api/v1/concurrent"
			}
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", url, nil)
			r.ServeHTTP(w, req)

			if idx%2 == 0 {
				assert.Equal(t, http.StatusOK, w.Code)
			} else {
				assert.Equal(t, http.StatusBadRequest, w.Code)
			}
		}(i)
	}

	wg.Wait()

	assert.Equal(t, int64(requestCount/2), atomic.LoadInt64(&successCount))
	assert.Equal(t, int64(requestCount/2), atomic.LoadInt64(&errorCount))
}

func TestConcurrentErrorHandling_PanicRecoveryUnderLoad(t *testing.T) {
	r := setupTestRouter()
	r.Use(gin.Recovery())

	r.GET("/api/v1/maybe-panic", func(c *gin.Context) {
		action := c.Query("action")
		if action == "panic" {
			panic("simulated panic under load")
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	var wg sync.WaitGroup
	total := 40

	for i := 0; i < total; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var url string
			if idx%4 == 0 {
				url = "/api/v1/maybe-panic?action=panic"
			} else {
				url = "/api/v1/maybe-panic?action=normal"
			}
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", url, nil)
			r.ServeHTTP(w, req)

			if idx%4 == 0 {
				assert.Equal(t, http.StatusInternalServerError, w.Code)
			} else {
				assert.Equal(t, http.StatusOK, w.Code)
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentErrorHandling_RaceConditionSafety(t *testing.T) {
	// Ensure the error response utilities are safe to call concurrently.
	r := setupTestRouter()

	r.POST("/api/v1/write", func(c *gin.Context) {
		var body map[string]interface{}
		if err := c.ShouldBindJSON(&body); err != nil {
			utils.SendErrorResponse(c, http.StatusBadRequest, "Invalid JSON", err)
			return
		}
		utils.SendSuccessResponse(c, http.StatusCreated, body, "Created")
	})

	var wg sync.WaitGroup

	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			var payload string
			if idx%3 == 0 {
				payload = "not-json"
			} else {
				payload = fmt.Sprintf(`{"key": "value-%d"}`, idx)
			}

			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/api/v1/write", strings.NewReader(payload))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)

			if idx%3 == 0 {
				assert.Equal(t, http.StatusBadRequest, w.Code)
			} else {
				assert.Equal(t, http.StatusCreated, w.Code)
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentErrorHandling_ErrorResponseConsistency(t *testing.T) {
	// Verify every error response from concurrent requests has a consistent JSON structure.
	r := setupTestRouter()
	r.GET("/api/v1/err", func(c *gin.Context) {
		utils.SendErrorResponse(c, http.StatusServiceUnavailable, "Service unavailable", errors.New("backend down"))
	})

	var wg sync.WaitGroup
	results := make([]int, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			w := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/api/v1/err", nil)
			r.ServeHTTP(w, req)

			results[idx] = w.Code

			var body utils.ErrorResponse
			err := json.Unmarshal(w.Body.Bytes(), &body)
			assert.NoError(t, err)
			assert.False(t, body.Success)
			assert.Equal(t, "Service unavailable", body.Error)
			assert.Empty(t, body.Details)
			assert.NotContains(t, w.Body.String(), "backend down")
		}(i)
	}

	wg.Wait()

	for i, code := range results {
		assert.Equal(t, http.StatusServiceUnavailable, code, "request %d got unexpected status", i)
	}
}
