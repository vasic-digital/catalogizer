package middleware

import (
	"catalogizer/internal/logredact"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func CORS() gin.HandlerFunc {
	allowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "http://localhost:5173,http://localhost:3000"
	}
	origins := strings.Split(allowedOrigins, ",")
	for i := range origins {
		origins[i] = strings.TrimSpace(origins[i])
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		for _, o := range origins {
			if o == origin && origin != "" {
				c.Header("Access-Control-Allow-Origin", origin)
				c.Header("Access-Control-Allow-Credentials", "true")
				break
			}
		}
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func Logger(logger *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := loggedPath(c.Request)
		raw := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		clientIP := c.ClientIP()
		method := c.Request.Method
		statusCode := c.Writer.Status()

		path = logredact.PathKnown(path, raw, requestKnownSecrets(c)) // never log ticket / access_token values or route-param secrets

		logger.Info("HTTP Request",
			zap.String("method", method),
			zap.String("path", path),
			zap.Int("status", statusCode),
			zap.String("ip", clientIP),
			zap.Duration("latency", latency),
			zap.String("user_agent", c.Request.UserAgent()),
		)
	}
}

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) > 0 {
			err := c.Errors.Last()
			// The text of an error (binder internals, driver text, paths) never goes to the client; the detail
			// is logged server-side under the request id (CAT-WP02-ERRLEAK).
			// production registers catalogizer/middleware.RequestID (key "request_id"); this package's own
			// RequestID sets "RequestID". The id may be client-supplied (X-Request-ID), so it is quoted (%q).
			reqID, ok := c.Get("request_id")
			if !ok {
				reqID, _ = c.Get("RequestID")
			}
			log.Printf("middleware: request %q %s %q failed (type %d, detail not returned to client): %v",
				fmt.Sprint(reqID), c.Request.Method, logredact.PathKnown(loggedPath(c.Request), "", requestKnownSecrets(c)), err.Type, requestErrorText(c, err.Err))
			switch err.Type {
			case gin.ErrorTypeBind:
				c.JSON(http.StatusBadRequest, gin.H{
					"error":   "Invalid request format",
					"details": "request could not be parsed",
				})
			default:
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": "Internal server error",
				})
			}
		}
	}
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		c.Header("X-Request-ID", requestID)
		c.Set("RequestID", requestID)
		c.Next()
	}
}

type RateLimiter struct {
	mu       sync.RWMutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		rl.mu.Lock()
		defer rl.mu.Unlock()

		clientIP := c.ClientIP()
		now := time.Now()

		// Clean old requests
		if requests, exists := rl.requests[clientIP]; exists {
			var validRequests []time.Time
			for _, req := range requests {
				if now.Sub(req) < rl.window {
					validRequests = append(validRequests, req)
				}
			}
			rl.requests[clientIP] = validRequests
		}

		// Check if limit exceeded
		if len(rl.requests[clientIP]) >= rl.limit {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error": "Rate limit exceeded",
			})
			c.Abort()
			return
		}

		// Add current request
		rl.requests[clientIP] = append(rl.requests[clientIP], now)
		c.Next()
	}
}

// requestErrorText is the loggable text of an error raised while serving c: the request's own credentials are removed first
// (a bare share token cannot be recognised by shape), then the free-text redactor and cap run.
func requestErrorText(c *gin.Context, err error) string {
	return logredact.ErrorTextKnown(fmt.Sprint(err), requestKnownSecrets(c))
}

// accessLogParamsKey is the gin context key under which AccessLogRouteParams stores the matched route parameters.
const accessLogParamsKey = "catalogizer.accesslog.route_params"

// AccessLogRouteParams hands the matched route parameters to AccessLogFormatter: gin.LogFormatterParams carries no route
// parameters, but it carries c.Keys. Register it AFTER the logger (the parameters exist once the route matched; the formatter
// runs after the handlers). HYG-F3.
func AccessLogRouteParams() gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(c.Params) > 0 {
			params := make(gin.Params, len(c.Params))
			copy(params, c.Params)
			c.Set(accessLogParamsKey, params)
		}
		c.Next()
	}
}

// loggedPath is the request path as it is logged, in CANONICAL escaped form: the raw path is split on "/" and every segment is
// unescaped and escaped again. A %2F inside a malformed token stays inside its segment (SecretPath redacts the whole segment,
// HYG-F4), while a non-canonical escape (%73hare, %54OKEN) is normalised so secret-segment keys and known secrets still match
// (HYG-R1-F1). A segment that does not unescape is kept as is.
func loggedPath(r *http.Request) string {
	segs := strings.Split(r.URL.EscapedPath(), "/")
	for i, sg := range segs {
		if u, err := url.PathUnescape(sg); err == nil {
			segs[i] = url.PathEscape(u)
		}
	}
	return strings.Join(segs, "/")
}

// AccessLogFormatter is the gin access-log line format (gin.LoggerWithFormatter): the path+query and the error text are redacted.
func AccessLogFormatter(p gin.LogFormatterParams) string {
	var params map[string]string
	if rp, ok := p.Keys[accessLogParamsKey].(gin.Params); ok {
		params = make(map[string]string, len(rp))
		for _, kv := range rp {
			params[kv.Key] = kv.Value
		}
	}
	path := loggedPath(p.Request)
	known := logredact.KnownSecrets(path, p.Request.URL.RawQuery, p.Request.Header, params)
	// also the DECODED path, as requestKnownSecrets does for the other sinks: a %2F-encoded key shape ("share%2FS") is one escaped
	// segment in loggedPath but "share/S" decoded (HYG-R3-F1 round 4, F4-1).
	known = append(known, logredact.KnownSecrets(p.Request.URL.Path, "", nil, nil)...)
	return fmt.Sprintf("[GIN] %s | %3d | %13v | %15s | %-7s %#v\n%s",
		p.TimeStamp.Format("2006/01/02 - 15:04:05"), p.StatusCode, p.Latency, p.ClientIP, p.Method,
		logredact.PathKnown(path, p.Request.URL.RawQuery, known), logredact.ErrorTextKnown(p.ErrorMessage, known))
}

// requestKnownSecrets are the credential values carried by c's request (path secrets, sensitive route parameters and query
// values, credential headers).
func requestKnownSecrets(c *gin.Context) []string {
	params := make(map[string]string, len(c.Params))
	for _, p := range c.Params {
		params[p.Key] = p.Value
	}
	return logredact.KnownSecrets(c.Request.URL.Path, c.Request.URL.RawQuery, c.Request.Header, params)
}
