package middleware

import (
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ipBucket struct {
	tokens    float64
	lastCheck time.Time
}

// RequestID adds a unique request ID to each request
func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}

		c.Header("X-Request-ID", requestID)
		c.Set("request_id", requestID)
		c.Next()
	}
}

// RateLimiter implements token-bucket rate limiting per client IP.
// The cleanup goroutine is stopped on server shutdown via middleware.StopAll().
func RateLimiter(requestsPerMinute int) gin.HandlerFunc {
	const maxBuckets = 10000

	var mu sync.Mutex
	buckets := make(map[string]*ipBucket)
	rate := float64(requestsPerMinute) / 60.0

	stopCh := make(chan struct{})
	var stopOnce sync.Once
	registerStop(func() {
		stopOnce.Do(func() { close(stopCh) })
	})

	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
			}

			mu.Lock()
			now := time.Now()
			for ip, b := range buckets {
				if now.Sub(b.lastCheck) > 10*time.Minute {
					delete(buckets, ip)
				}
			}
			// Evict oldest half if bucket map grows too large (e.g., under DDoS)
			if len(buckets) > maxBuckets {
				type ipTime struct {
					ip string
					t  time.Time
				}
				entries := make([]ipTime, 0, len(buckets))
				for ip, b := range buckets {
					entries = append(entries, ipTime{ip, b.lastCheck})
				}
				// Sort by lastCheck ascending (oldest first)
				sort.Slice(entries, func(i, j int) bool {
					return entries[i].t.Before(entries[j].t)
				})
				// Evict oldest half
				evictCount := len(entries) / 2
				for i := 0; i < evictCount; i++ {
					delete(buckets, entries[i].ip)
				}
			}
			mu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		b, exists := buckets[ip]
		if !exists {
			b = &ipBucket{
				tokens:    float64(requestsPerMinute),
				lastCheck: time.Now(),
			}
			buckets[ip] = b
		}

		now := time.Now()
		elapsed := now.Sub(b.lastCheck).Seconds()
		b.tokens += elapsed * rate
		if b.tokens > float64(requestsPerMinute) {
			b.tokens = float64(requestsPerMinute)
		}
		b.lastCheck = now

		if b.tokens < 1.0 {
			// Time until one whole token has refilled (RLW-FU2-F3, RFC 9110 §10.2.3).
			wait := time.Duration((1.0 - b.tokens) / rate * float64(time.Second))
			mu.Unlock()
			c.Header("Retry-After", retryAfterSeconds(wait))
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded",
			})
			return
		}

		b.tokens -= 1.0
		mu.Unlock()

		c.Next()
	}
}

// CORS handles Cross-Origin Resource Sharing
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
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}
