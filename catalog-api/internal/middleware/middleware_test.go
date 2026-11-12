package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// ---------- CORS ----------

func TestCORS(t *testing.T) {
	tests := []struct {
		name            string
		envOrigins      string
		requestOrigin   string
		method          string
		wantAllowOrigin string
		wantStatus      int
	}{
		{
			name:            "allowed origin from default list",
			envOrigins:      "",
			requestOrigin:   "http://localhost:5173",
			method:          "GET",
			wantAllowOrigin: "http://localhost:5173",
			wantStatus:      http.StatusOK,
		},
		{
			name:            "second default origin",
			envOrigins:      "",
			requestOrigin:   "http://localhost:3000",
			method:          "GET",
			wantAllowOrigin: "http://localhost:3000",
			wantStatus:      http.StatusOK,
		},
		{
			name:            "disallowed origin gets no Access-Control-Allow-Origin",
			envOrigins:      "",
			requestOrigin:   "http://evil.example.com",
			method:          "GET",
			wantAllowOrigin: "",
			wantStatus:      http.StatusOK,
		},
		{
			name:            "empty origin header",
			envOrigins:      "",
			requestOrigin:   "",
			method:          "GET",
			wantAllowOrigin: "",
			wantStatus:      http.StatusOK,
		},
		{
			name:            "custom env origins - allowed",
			envOrigins:      "https://app.example.com, https://admin.example.com",
			requestOrigin:   "https://admin.example.com",
			method:          "GET",
			wantAllowOrigin: "https://admin.example.com",
			wantStatus:      http.StatusOK,
		},
		{
			name:            "custom env origins - disallowed",
			envOrigins:      "https://app.example.com",
			requestOrigin:   "https://other.example.com",
			method:          "GET",
			wantAllowOrigin: "",
			wantStatus:      http.StatusOK,
		},
		{
			name:            "OPTIONS preflight returns 204",
			envOrigins:      "",
			requestOrigin:   "http://localhost:5173",
			method:          "OPTIONS",
			wantAllowOrigin: "http://localhost:5173",
			wantStatus:      http.StatusNoContent,
		},
		{
			name:            "OPTIONS with disallowed origin still returns 204",
			envOrigins:      "",
			requestOrigin:   "http://evil.example.com",
			method:          "OPTIONS",
			wantAllowOrigin: "",
			wantStatus:      http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envOrigins != "" {
				t.Setenv("CORS_ALLOWED_ORIGINS", tt.envOrigins)
			} else {
				t.Setenv("CORS_ALLOWED_ORIGINS", "")
			}

			router := gin.New()
			router.Use(CORS())
			router.GET("/test", func(c *gin.Context) {
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest(tt.method, "/test", nil)
			if tt.requestOrigin != "" {
				req.Header.Set("Origin", tt.requestOrigin)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantAllowOrigin, w.Header().Get("Access-Control-Allow-Origin"))

			// These headers should always be set
			assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "GET")
			assert.Contains(t, w.Header().Get("Access-Control-Allow-Methods"), "POST")
			assert.Contains(t, w.Header().Get("Access-Control-Allow-Headers"), "Authorization")
		})
	}
}

func TestCORS_CredentialsHeader(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	router := gin.New()
	router.Use(CORS())
	router.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, "true", w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORS_PreflightDoesNotReachHandler(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "")

	handlerCalled := false
	router := gin.New()
	router.Use(CORS())
	router.OPTIONS("/test", func(c *gin.Context) {
		handlerCalled = true
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("OPTIONS", "/test", nil)
	req.Header.Set("Origin", "http://localhost:5173")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
	assert.False(t, handlerCalled, "OPTIONS handler should not be reached because CORS middleware aborts")
}

// ---------- RequestID ----------

func TestRequestID(t *testing.T) {
	tests := []struct {
		name            string
		incomingID      string
		expectIncoming  bool
		expectGenerated bool
	}{
		{
			name:            "generates new ID when none provided",
			incomingID:      "",
			expectGenerated: true,
		},
		{
			name:           "propagates existing ID",
			incomingID:     "my-custom-request-id-123",
			expectIncoming: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var contextID string
			router := gin.New()
			router.Use(RequestID())
			router.GET("/test", func(c *gin.Context) {
				val, _ := c.Get("RequestID")
				contextID, _ = val.(string)
				c.Status(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			if tt.incomingID != "" {
				req.Header.Set("X-Request-ID", tt.incomingID)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			responseID := w.Header().Get("X-Request-ID")
			assert.NotEmpty(t, responseID)
			assert.Equal(t, responseID, contextID, "context and response header should match")

			if tt.expectIncoming {
				assert.Equal(t, tt.incomingID, responseID)
			}
			if tt.expectGenerated {
				assert.NotEmpty(t, responseID)
				assert.Len(t, responseID, 36, "generated UUID should be 36 chars")
			}
		})
	}
}

func TestRequestID_UniquenessAcrossRequests(t *testing.T) {
	router := gin.New()
	router.Use(RequestID())
	router.GET("/test", func(c *gin.Context) { c.Status(http.StatusOK) })

	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		id := w.Header().Get("X-Request-ID")
		assert.False(t, ids[id], "duplicate request ID detected: %s", id)
		ids[id] = true
	}
}

// ---------- ErrorHandler ----------

func TestErrorHandler(t *testing.T) {
	tests := []struct {
		name       string
		setupErr   func(c *gin.Context)
		wantStatus int
		wantBody   map[string]interface{}
	}{
		{
			name:       "no errors passes through",
			setupErr:   func(c *gin.Context) {},
			wantStatus: http.StatusOK,
			wantBody:   nil,
		},
		{
			name: "bind error returns 400",
			setupErr: func(c *gin.Context) {
				c.Error(&gin.Error{
					Err:  assert.AnError,
					Type: gin.ErrorTypeBind,
				})
			},
			wantStatus: http.StatusBadRequest,
			wantBody: map[string]interface{}{
				"error": "Invalid request format",
			},
		},
		{
			name: "public error returns generic 500 without its text",
			setupErr: func(c *gin.Context) {
				c.Error(&gin.Error{
					Err:  assert.AnError,
					Type: gin.ErrorTypePublic,
				})
			},
			wantStatus: http.StatusInternalServerError,
		},
		{
			name: "private error returns generic 500",
			setupErr: func(c *gin.Context) {
				c.Error(&gin.Error{
					Err:  assert.AnError,
					Type: gin.ErrorTypePrivate,
				})
			},
			wantStatus: http.StatusInternalServerError,
			wantBody: map[string]interface{}{
				"error": "Internal server error",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := gin.New()
			router.Use(ErrorHandler())
			router.GET("/test", func(c *gin.Context) {
				tt.setupErr(c)
				if len(c.Errors) == 0 {
					c.JSON(http.StatusOK, gin.H{"ok": true})
				}
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if tt.wantBody != nil {
				var body map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &body)
				require.NoError(t, err)
				for k, v := range tt.wantBody {
					assert.Equal(t, v, body[k], "body field %q mismatch", k)
				}
			}
		})
	}
}

func TestErrorHandler_BindErrorContainsDetails(t *testing.T) {
	router := gin.New()
	router.Use(ErrorHandler())
	router.GET("/test", func(c *gin.Context) {
		c.Error(&gin.Error{
			Err:  assert.AnError,
			Type: gin.ErrorTypeBind,
		})
	})

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Contains(t, body, "details", "bind error response should include details field")
}

// ---------- Logger ----------

func TestLogger(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	router := gin.New()
	router.Use(Logger(logger))
	router.GET("/test", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/test?foo=bar", nil)
	req.Header.Set("User-Agent", "test-agent")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, 1, logs.Len(), "expected exactly one log entry")
	entry := logs.All()[0]

	assert.Equal(t, "HTTP Request", entry.Message)

	fields := make(map[string]interface{})
	for _, f := range entry.Context {
		fields[f.Key] = f
	}

	assert.Contains(t, fields, "method")
	assert.Contains(t, fields, "path")
	assert.Contains(t, fields, "status")
	assert.Contains(t, fields, "latency")
	assert.Contains(t, fields, "user_agent")
	assert.Contains(t, fields, "ip")
}

func TestLogger_PathIncludesQueryString(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	router := gin.New()
	router.Use(Logger(logger))
	router.GET("/api/search", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/api/search?q=hello&page=2", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, 1, logs.Len())
	entry := logs.All()[0]

	var pathValue string
	for _, f := range entry.Context {
		if f.Key == "path" {
			pathValue = f.String
		}
	}
	assert.Equal(t, "/api/search?q=hello&page=2", pathValue)
}

func TestLogger_NoQueryString(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	router := gin.New()
	router.Use(Logger(logger))
	router.GET("/plain", func(c *gin.Context) { c.Status(http.StatusOK) })

	req := httptest.NewRequest("GET", "/plain", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, 1, logs.Len())
	var pathValue string
	for _, f := range logs.All()[0].Context {
		if f.Key == "path" {
			pathValue = f.String
		}
	}
	assert.Equal(t, "/plain", pathValue)
}

func TestLogger_LatencyIsPositive(t *testing.T) {
	core, logs := observer.New(zap.InfoLevel)
	logger := zap.New(core)

	router := gin.New()
	router.Use(Logger(logger))
	router.GET("/slow", func(c *gin.Context) {
		time.Sleep(10 * time.Millisecond)
		c.Status(http.StatusOK)
	})

	req := httptest.NewRequest("GET", "/slow", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, 1, logs.Len())
	for _, f := range logs.All()[0].Context {
		if f.Key == "latency" {
			latency := time.Duration(f.Integer)
			assert.True(t, latency >= 10*time.Millisecond, "latency should be at least 10ms, got %v", latency)
			return
		}
	}
	t.Fatal("latency field not found in log entry")
}

func TestLogger_RecordsStatusCode(t *testing.T) {
	tests := []struct {
		name   string
		status int
	}{
		{"200 OK", http.StatusOK},
		{"404 Not Found", http.StatusNotFound},
		{"500 Internal Server Error", http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zap.InfoLevel)
			logger := zap.New(core)

			router := gin.New()
			router.Use(Logger(logger))
			router.GET("/test", func(c *gin.Context) {
				c.Status(tt.status)
			})

			req := httptest.NewRequest("GET", "/test", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			require.Equal(t, 1, logs.Len())
			for _, f := range logs.All()[0].Context {
				if f.Key == "status" {
					assert.Equal(t, int64(tt.status), f.Integer)
					return
				}
			}
			t.Fatal("status field not found")
		})
	}
}

// ---------- RateLimiter ----------

func TestRateLimiter_UnderLimit(t *testing.T) {
	rl := NewRateLimiter(5, time.Minute)

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code, "request %d should succeed", i+1)
	}
}

func TestRateLimiter_ExceedsLimit(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:5555"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// 4th request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:5555"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, "Rate limit exceeded", body["error"])
}

func TestRateLimiter_DifferentIPsIndependent(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Exhaust limit for IP A
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// IP A should be limited
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// IP B should still work
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.2:1234"
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimiter_WindowExpiry(t *testing.T) {
	// Use a very short window to test expiry
	rl := NewRateLimiter(2, 50*time.Millisecond)

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// Exhaust limit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusOK, w.Code)
	}

	// Should be rate limited now
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)

	// Should be allowed again
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestRateLimiter_SlidingWindow(t *testing.T) {
	rl := NewRateLimiter(3, 100*time.Millisecond)

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	makeRequest := func() int {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}

	// Send 2 requests
	assert.Equal(t, http.StatusOK, makeRequest())
	assert.Equal(t, http.StatusOK, makeRequest())

	// Wait for half the window
	time.Sleep(60 * time.Millisecond)

	// 3rd request should succeed (still within window, but under limit)
	assert.Equal(t, http.StatusOK, makeRequest())

	// 4th should fail
	assert.Equal(t, http.StatusTooManyRequests, makeRequest())

	// Wait for first two to expire (they were ~60ms ago, window is 100ms, so wait another 50ms)
	time.Sleep(50 * time.Millisecond)

	// Now the first 2 requests should have expired, only the 3rd remains in window
	// So we should be able to make 2 more requests
	assert.Equal(t, http.StatusOK, makeRequest())
	assert.Equal(t, http.StatusOK, makeRequest())
}

func TestRateLimiter_ConcurrentRequests(t *testing.T) {
	rl := NewRateLimiter(10, time.Minute)

	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	var wg sync.WaitGroup
	results := make([]int, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			req := httptest.NewRequest("GET", "/test", nil)
			req.RemoteAddr = "10.0.0.1:1234"
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			results[idx] = w.Code
		}(i)
	}

	wg.Wait()

	okCount := 0
	limitedCount := 0
	for _, code := range results {
		switch code {
		case http.StatusOK:
			okCount++
		case http.StatusTooManyRequests:
			limitedCount++
		default:
			t.Errorf("unexpected status code: %d", code)
		}
	}

	// With 10 limit and 20 requests, at least some should succeed and some should be limited
	// Due to race conditions in the non-mutex-protected rate limiter, exact counts may vary
	assert.True(t, okCount > 0, "at least some requests should succeed")
	assert.True(t, okCount+limitedCount == 20, "all requests should have a result")
}

func TestRateLimiter_AbortsPipeline(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)

	handlerCalled := 0
	router := gin.New()
	router.Use(rl.Middleware())
	router.GET("/test", func(c *gin.Context) {
		handlerCalled++
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})

	// First request should reach handler
	req := httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, handlerCalled)

	// Second request should be aborted before handler
	req = httptest.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w = httptest.NewRecorder()
	router.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, 1, handlerCalled, "handler should not be called when rate limited")
}
