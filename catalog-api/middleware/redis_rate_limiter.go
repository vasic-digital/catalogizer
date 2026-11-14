package middleware

import (
	"catalogizer/internal/logredact"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"catalogizer/internal/logging"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RedisRateLimiterConfig holds configuration for Redis-based rate limiting
type RedisRateLimiterConfig struct {
	// Requests per window
	Requests int
	// Window duration (e.g., time.Minute, time.Hour)
	Window time.Duration
	// Key generator function (e.g., by IP, user ID, etc.)
	KeyGenerator func(*gin.Context) string
	// Message to return when rate limited
	Message string
	// Redis client
	Client *redis.Client
	// Tier names the key namespace of this limiter ("auth", "default", ...).
	// Two limiters with different tiers never share a counter, even for the
	// same client IP (PKGC-FU4). Empty means "untiered".
	Tier string
	// Fallback, when set, handles the request instead of answering 503 when
	// Redis errors after startup (operator decision 2026-11-12, CAT-OPDEC-A).
	// It is the in-memory limiter of the same tier. It is consulted per
	// request, so Redis takes over again as soon as it answers.
	Fallback gin.HandlerFunc
	// OutageBackoff is how long a limiter that has fallen back stops asking
	// Redis (circuit breaker, CAT-OPA-3). Zero means redisOutageBackoffDefault;
	// negative disables the breaker (every request tries Redis).
	OutageBackoff time.Duration
	// Clock is the breaker/log-throttle clock; nil means time.Now (tests inject
	// a fake clock instead of sleeping).
	Clock func() time.Time
}

// tierKey builds "<prefix>:<tier>:<client key>". An empty tier is "untiered",
// never a bare prefix, so it cannot collide with a named tier.
func tierKey(prefix, tier, clientKey string) string {
	if tier == "" {
		tier = "untiered"
	}
	return prefix + ":" + tier + ":" + clientKey
}

// redisFallbackLogEvery bounds the outage log line to one per interval.
const redisFallbackLogEvery = 30 * time.Second

// redisOutageBackoffDefault is how long a limiter that has fallen back stops
// asking Redis (circuit breaker, CAT-OPA-3). Without it every request during
// an outage waits for the 500 ms Redis timeout before the fallback answers.
const redisOutageBackoffDefault = 5 * time.Second

// outageState is the per-limiter outage state: the log throttle (CAT-OPA-5:
// one per limiter, so the auth tier does not silence the default tier) and the
// circuit breaker. One instance is created per RedisRateLimit /
// SlidingWindowRedisRateLimit call.
//
// Breaker states: closed (openUntil zero), open (now < openUntil: everything
// goes to the fallback), half-open (backoff expired: exactly ONE request, the
// holder of the probing slot, asks Redis; the rest keep using the fallback
// until the probe settles, F2).
type outageState struct {
	mu        sync.Mutex
	logLast   time.Time
	openUntil time.Time
	probing   bool
	backoff   time.Duration
	now       func() time.Time // injectable clock (F4); never nil
}

func newOutageState(config RedisRateLimiterConfig) *outageState {
	b := config.OutageBackoff
	if b == 0 {
		b = redisOutageBackoffDefault
	}
	now := config.Clock
	if now == nil {
		now = time.Now
	}
	return &outageState{backoff: b, now: now}
}

// admit decides where the request goes. useFallbackNow is true when the
// request must go straight to the fallback. probe is true when this request
// holds the single half-open probe slot and MUST call settle.
func (st *outageState) admit(config RedisRateLimiterConfig) (useFallbackNow, probe bool) {
	if config.Fallback == nil || st.backoff < 0 {
		return false, false
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.openUntil.IsZero() {
		return false, false // closed
	}
	if st.now().Before(st.openUntil) {
		return true, false // open
	}
	if st.probing {
		return true, false // half-open, someone else is probing
	}
	st.probing = true
	return false, true
}

type breakerOutcome int

const (
	outcomeNeutral breakerOutcome = iota // probe slot released, state unchanged (still half-open)
	outcomeOK                            // Redis answered: close the breaker
)

// tripped reports whether the breaker is open or half-open (tests).
func (st *outageState) tripped() bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	return !st.openUntil.IsZero()
}

// settle ends a half-open probe. A tripping failure is recorded by trip()
// before settle is reached; settle only clears the slot (and closes the
// breaker on outcomeOK).
func (st *outageState) settle(probe bool, o breakerOutcome) {
	if !probe {
		return
	}
	st.mu.Lock()
	st.probing = false
	if o == outcomeOK {
		st.openUntil = time.Time{}
	}
	st.mu.Unlock()
}

// Error classes for the breaker (F1, constitution 11.4.276(C): the class is
// named, every member enumerated, the class closed). The breaker trips ONLY
// for a genuine Redis outage, and only while the request itself is alive.
//
//	class                                              trips?  why
//	-------------------------------------------------- ------- -------------------------------
//	request context already done (client went away)    no      says nothing about Redis
//	context.Canceled                                   no      client cancel
//	context.DeadlineExceeded, request ctx still alive  YES     call deadline or pool wait (N2/N3)
//	net.Error (dial, refused, reset, i/o timeout)      YES     transport outage
//	io.EOF / io.ErrUnexpectedEOF                       YES     connection dropped
//	redis.ErrClosed / redis.ErrPoolTimeout             YES     client closed / pool exhausted
//	reply error LOADING / MASTERDOWN / CLUSTERDOWN     YES     server up but not serving
//	any other reply error (WRONGTYPE, NOSCRIPT, ERR    no      Redis is answering; this key or
//	  script errors, NOAUTH, ...)                              script is the problem
//	"unexpected reply" shape error (no Go error)       no      Redis is answering
//	anything unclassified                              no      do not switch the limiter off on
//	                                                           a guess (11.4.6)
//
// N2: the 500 ms context in the handlers is NOT what bounds a black-holed
// Redis call in production unless the client enables ContextTimeoutEnabled;
// otherwise go-redis ReadTimeout (3 s) governs, so each in-flight request or
// probe can take about 3 s before DeadlineExceeded/i-o timeout.
// N3: DeadlineExceeded can also come from waiting for a pool connection under
// heavy load; that trips the breaker too. Accepted, bounded risk: the breaker
// only diverts to the in-memory fallback for the backoff and re-probes.
//
// A request that does not trip the breaker still gets the fallback for itself
// (or 503 without a fallback), exactly as before the breaker existed.
func isRedisOutage(reqCtx context.Context, err error) bool {
	if err == nil || reqCtx.Err() != nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if errors.Is(err, redis.ErrClosed) || errors.Is(err, redis.ErrPoolTimeout) {
		return true
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	var re redis.Error
	if errors.As(err, &re) {
		msg := re.Error()
		for _, p := range []string{"LOADING", "MASTERDOWN", "CLUSTERDOWN"} {
			if strings.HasPrefix(msg, p) {
				return true
			}
		}
		return false // reply error: Redis is answering
	}
	var ne net.Error
	return errors.As(err, &ne)
}

// useFallback hands the request to config.Fallback when one is configured and
// returns true. It opens the breaker only when err is a genuine outage
// (isRedisOutage) and writes the throttled log line. The log line carries the
// Redis error text VERBATIM: that text can include the Redis host:port (never
// credentials), so it is not address-free.
func useFallback(c *gin.Context, config RedisRateLimiterConfig, st *outageState, err error, release func()) bool {
	if config.Fallback == nil {
		return false
	}
	trip := isRedisOutage(c.Request.Context(), err)
	st.mu.Lock()
	if trip && st.backoff > 0 {
		st.openUntil = st.now().Add(st.backoff)
	}
	log := st.now().Sub(st.logLast) >= redisFallbackLogEvery
	if log {
		st.logLast = st.now()
	}
	st.mu.Unlock()
	if log && logging.Logger != nil {
		logging.With(logging.ErrorField(err)).Warn("Redis rate limiter unavailable; using in-memory limiter fallback")
	}
	// N1: the half-open probe slot is released BEFORE the downstream chain
	// runs. Fallback may call c.Next(), so a slow or panicking downstream
	// handler must neither hold the slot for its whole duration nor leak it.
	if release != nil {
		release()
	}
	config.Fallback(c)
	return true
}

// DefaultRedisRateLimiterConfig returns a secure default configuration
func DefaultRedisRateLimiterConfig(client *redis.Client) RedisRateLimiterConfig {
	return RedisRateLimiterConfig{
		Tier:     "default",
		Requests: 2000, // matches the in-memory defaultRateLimiter (main.go: RateLimitByUser(2000, "1m"))
		Window:   time.Minute,
		Message:  "Rate limit exceeded. Please try again later.",
		Client:   client,
		KeyGenerator: func(c *gin.Context) string {
			return c.ClientIP()
		},
	}
}

// StrictRedisRateLimiterConfig returns a stricter configuration for sensitive endpoints
func StrictRedisRateLimiterConfig(client *redis.Client) RedisRateLimiterConfig {
	return RedisRateLimiterConfig{
		Tier:     "strict",
		Requests: 10, // 10 requests per window
		Window:   time.Minute,
		Message:  "Too many requests. Please wait before trying again.",
		Client:   client,
		KeyGenerator: func(c *gin.Context) string {
			return c.ClientIP()
		},
	}
}

// AuthRedisRateLimiterConfig returns a rate limiter specifically for auth endpoints
func AuthRedisRateLimiterConfig(client *redis.Client) RedisRateLimiterConfig {
	return RedisRateLimiterConfig{
		Tier: "auth",
		// Matches the in-memory authRateLimiter (main.go: RateLimitByUser(600, "1m")).
		// This tier guards token ops (/refresh, /logout, /me, /status,
		// /permissions, /profile), NOT /login or /register: those use the
		// in-memory loginRateLimiter (30/min) and never reach Redis.
		Requests: 600,
		Window:   time.Minute,
		Message:  "Too many authentication requests. Please try again later.",
		Client:   client,
		KeyGenerator: func(c *gin.Context) string {
			// Use both IP and username for auth endpoints if available
			key := c.ClientIP()
			if username, exists := c.Get("username"); exists {
				if userStr, ok := username.(string); ok {
					key = userStr + ":" + key
				}
			}
			return key
		},
	}
}

// UserRedisRateLimiterConfig returns user-based rate limiting
func UserRedisRateLimiterConfig(client *redis.Client) RedisRateLimiterConfig {
	return RedisRateLimiterConfig{
		Tier:     "user",
		Requests: 200, // 200 requests per minute for authenticated users
		Window:   time.Minute,
		Message:  "Rate limit exceeded. Please try again later.",
		Client:   client,
		KeyGenerator: func(c *gin.Context) string {
			// Try to get user ID from context (set by auth middleware)
			if userID, exists := c.Get("user_id"); exists {
				if id, ok := userID.(string); ok {
					return "user:" + id
				}
			}
			// Fallback to IP for unauthenticated requests
			return "ip:" + c.ClientIP()
		},
	}
}

// fixedWindowScript increments the window counter and sets its expiry only
// when the window starts. Returns {count, pttl_ms}.
var fixedWindowScript = redis.NewScript(`
local count = redis.call('INCR', KEYS[1])
if count == 1 or redis.call('PTTL', KEYS[1]) == -1 then
  redis.call('PEXPIRE', KEYS[1], ARGV[1])
end
return {count, redis.call('PTTL', KEYS[1])}
`)

// RedisRateLimit creates a Gin middleware for Redis-based rate limiting.
//
// Fixed window: the window starts at the first request for a key and ends
// Window later; requests inside the window do not extend it. On any Redis
// error the middleware fails closed (HTTP 503) unless config.Fallback is set,
// in which case the in-memory fallback answers (CAT-OPDEC-A). After a
// fallback the limiter stops asking Redis for OutageBackoff (default 5 s) so
// an outage does not add the Redis timeout to every request (CAT-OPA-3).
//
// Wiring (catalog-api/main.go): this limiter replaces the in-memory one only
// when REDIS_RATE_LIMIT=true AND the startup Redis PING succeeds; otherwise
// each process uses per-process in-memory limiting.
func RedisRateLimit(config RedisRateLimiterConfig) gin.HandlerFunc {
	h, _ := redisRateLimitWithState(config)
	return h
}

// redisRateLimitWithState is RedisRateLimit plus its breaker state (tests).
func redisRateLimitWithState(config RedisRateLimiterConfig) (gin.HandlerFunc, *outageState) {
	st := newOutageState(config)
	return func(c *gin.Context) {
		toFallback, probe := st.admit(config)
		if toFallback {
			config.Fallback(c)
			return
		}
		// Idempotent release of the probe slot (N1/N4): runs at most once,
		// from the fallback path (before downstream), the normal paths, or
		// this defer (panic in KeyGenerator, script call or anywhere later).
		released := false
		release := func(o breakerOutcome) {
			if released {
				return
			}
			released = true
			st.settle(probe, o)
		}
		defer release(outcomeNeutral)
		key := tierKey("rate_limit", config.Tier, config.KeyGenerator(c))
		ctx, cancel := context.WithTimeout(c.Request.Context(), 500*time.Millisecond)
		defer cancel()

		// Atomic fixed window: INCR, and set the expiry only when the window
		// starts (count == 1), so later requests inside the window never
		// extend it. A key left without a TTL (PTTL == -1) is repaired too.
		res, err := fixedWindowScript.Run(ctx, config.Client, []string{key},
			config.Window.Milliseconds()).Int64Slice()

		// Fail closed: when Redis is unavailable, reject instead of letting
		// requests bypass the limiter.
		if err != nil || len(res) != 2 {
			if err == nil {
				err = errors.New("unexpected fixed-window script reply")
			}
			if useFallback(c, config, st, err, func() { release(outcomeNeutral) }) {
				return
			}
			release(outcomeNeutral)
			if logging.Logger != nil {
				logging.With(logging.ErrorField(err)).Warn("Redis rate limiter error")
			}
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "Rate limiting service temporarily unavailable",
				"message": "Please try again later",
			})
			c.Abort()
			return
		}

		release(outcomeOK)
		currentCount := res[0]
		ttl := time.Duration(res[1]) * time.Millisecond
		if ttl <= 0 {
			ttl = config.Window
		}

		// Check if rate limit exceeded
		if currentCount > int64(config.Requests) {
			// Log rate limit attempt
			if logging.Logger != nil {
				logging.With(
					logging.String("key", key),
					logging.String("ip", c.ClientIP()),
					logging.String("path", logredact.SecretPath(c.Request.URL.Path)),
				).Warn("Rate limit exceeded")
			}

			c.Header("Retry-After", retryAfterSeconds(ttl))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     config.Message,
				"retry_after": ttl.String(),
			})
			c.Abort()
			return
		}

		// Add rate limit headers
		remaining := int64(config.Requests) - currentCount
		c.Header("X-RateLimit-Limit", strconv.Itoa(config.Requests))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(int(remaining)))
		c.Header("X-RateLimit-Reset", time.Now().Add(ttl).Format(time.RFC3339))
		c.Header("X-RateLimit-Window", config.Window.String())

		c.Next()
	}, st
}

// SlidingWindowRedisRateLimit implements sliding window rate limiting using Redis.
// On any Redis error it hands the request to config.Fallback when one is set
// (CAT-OPDEC-A), otherwise it fails closed (HTTP 503), like RedisRateLimit.
func SlidingWindowRedisRateLimit(config RedisRateLimiterConfig) gin.HandlerFunc {
	h, _ := slidingWindowWithState(config)
	return h
}

// slidingWindowWithState is SlidingWindowRedisRateLimit plus its breaker state (tests).
func slidingWindowWithState(config RedisRateLimiterConfig) (gin.HandlerFunc, *outageState) {
	st := newOutageState(config)
	return func(c *gin.Context) {
		toFallback, probe := st.admit(config)
		if toFallback {
			config.Fallback(c)
			return
		}
		// Idempotent release of the probe slot (N1/N4): runs at most once,
		// from the fallback path (before downstream), the normal paths, or
		// this defer (panic in KeyGenerator, script call or anywhere later).
		released := false
		release := func(o breakerOutcome) {
			if released {
				return
			}
			released = true
			st.settle(probe, o)
		}
		defer release(outcomeNeutral)
		key := tierKey("sliding_rate_limit", config.Tier, config.KeyGenerator(c))
		ctx, cancel := context.WithTimeout(c.Request.Context(), 500*time.Millisecond)
		defer cancel()
		now := time.Now().UnixNano()
		windowStart := now - config.Window.Nanoseconds()

		// Clean up old entries and add current request timestamp
		pipe := config.Client.Pipeline()

		// Remove old entries outside the window
		pipe.ZRemRangeByScore(ctx, key, "0", strconv.FormatInt(windowStart, 10))

		// Add current request
		pipe.ZAdd(ctx, key, redis.Z{
			Score:  float64(now),
			Member: now,
		})

		// Count current requests in window
		countCmd := pipe.ZCard(ctx, key)

		// Set expiration for the key
		pipe.Expire(ctx, key, config.Window*2)

		// Execute pipeline
		_, err := pipe.Exec(ctx)

		// Handle Redis errors gracefully
		if err != nil {
			if useFallback(c, config, st, err, func() { release(outcomeNeutral) }) {
				return
			}
			release(outcomeNeutral)
			if logging.Logger != nil {
				logging.With(logging.ErrorField(err)).Warn("Redis sliding window rate limiter error")
			}
			// No fallback configured: fail closed, consistent with RedisRateLimit.
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "Rate limiting service temporarily unavailable",
				"message": "Please try again later",
			})
			c.Abort()
			return
		}

		release(outcomeOK)
		// Get current count
		count, err := countCmd.Result()
		if err != nil {
			count = 1
		}

		// Check if rate limit exceeded
		if count > int64(config.Requests) {
			// Get TTL for retry-after header
			ttl, _ := config.Client.TTL(ctx, key).Result()
			if ttl < 0 {
				ttl = config.Window
			}

			// Log rate limit attempt
			if logging.Logger != nil {
				logging.With(
					logging.String("key", key),
					logging.String("ip", c.ClientIP()),
					logging.String("path", logredact.SecretPath(c.Request.URL.Path)),
				).Warn("Sliding window rate limit exceeded")
			}

			c.Header("Retry-After", retryAfterSeconds(min(ttl, config.Window)))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "rate_limit_exceeded",
				"message":     config.Message,
				"retry_after": ttl.String(),
			})
			c.Abort()
			return
		}

		// Add rate limit headers
		remaining := int64(config.Requests) - count
		c.Header("X-RateLimit-Limit", strconv.Itoa(config.Requests))
		c.Header("X-RateLimit-Remaining", strconv.Itoa(int(remaining)))
		c.Header("X-RateLimit-Reset", time.Now().Add(config.Window).Format(time.RFC3339))
		c.Header("X-RateLimit-Window", config.Window.String())
		c.Header("X-RateLimit-Algorithm", "sliding-window")

		c.Next()
	}, st
}

// TokenBucketRedisRateLimit implements token bucket algorithm using Redis
func TokenBucketRedisRateLimit(
	redisClient *redis.Client,
	keyGenerator func(*gin.Context) string,
	tokens int, // Maximum tokens in bucket
	refillRate int, // Tokens to add per second
	refillInterval time.Duration, // How often to add tokens
) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := "token_bucket:" + keyGenerator(c)
		ctx, cancel := context.WithTimeout(c.Request.Context(), 500*time.Millisecond)
		defer cancel()

		// Use Lua script for atomic token bucket operations
		luaScript := `
			local key = KEYS[1]
			local current_time = tonumber(ARGV[1])
			local tokens = tonumber(ARGV[2])
			local refill_rate = tonumber(ARGV[3])
			local refill_interval = tonumber(ARGV[4])
			local requested = tonumber(ARGV[5])
			
			local bucket = redis.call('HMGET', key, 'tokens', 'last_refill', 'max_tokens')
			local current_tokens = tonumber(bucket[1]) or tokens
			local last_refill = tonumber(bucket[2]) or current_time
			local max_tokens = tonumber(bucket[3]) or tokens
			
			-- Calculate tokens to add based on elapsed time
			local elapsed = current_time - last_refill
			local tokens_to_add = math.floor(elapsed / refill_interval) * refill_rate
			current_tokens = math.min(max_tokens, current_tokens + tokens_to_add)
			
			-- Check if enough tokens are available
			if current_tokens >= requested then
				-- Consume tokens
				current_tokens = current_tokens - requested
				redis.call('HMSET', key, 'tokens', current_tokens, 'last_refill', current_time, 'max_tokens', max_tokens)
				redis.call('EXPIRE', key, refill_interval * 2)
				return {1, current_tokens, max_tokens}
			else
				-- Not enough tokens, update last_refill but don't consume
				redis.call('HMSET', key, 'tokens', current_tokens, 'last_refill', current_time, 'max_tokens', max_tokens)
				redis.call('EXPIRE', key, refill_interval * 2)
				return {0, current_tokens, max_tokens}
			end
		`

		now := time.Now().Unix()
		result, err := redisClient.Eval(ctx, luaScript, []string{key},
			now, tokens, refillRate, refillInterval.Seconds(), 1).Result()

		if err != nil {
			if logging.Logger != nil {
				logging.With(logging.ErrorField(err)).Warn("Redis token bucket rate limiter error")
			}
			c.Next()
			return
		}

		// Parse Lua script result
		if arr, ok := result.([]interface{}); ok && len(arr) >= 3 {
			allowed := arr[0].(int64)
			currentTokens := arr[1].(int64)
			maxTokens := arr[2].(int64)

			if allowed == 0 {
				// Rate limit exceeded
				c.Header("Retry-After", retryAfterSeconds(refillInterval))
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error":       "rate_limit_exceeded",
					"message":     "Rate limit exceeded. Please try again later.",
					"retry_after": refillInterval.String(),
				})
				c.Abort()
				return
			}

			// Add rate limit headers
			c.Header("X-RateLimit-Limit", strconv.Itoa(int(maxTokens)))
			c.Header("X-RateLimit-Remaining", strconv.Itoa(int(currentTokens)))
			c.Header("X-RateLimit-Reset", time.Now().Add(refillInterval).Format(time.RFC3339))
			c.Header("X-RateLimit-Algorithm", "token-bucket")
		}

		c.Next()
	}
}

// NewRedisTierLimiters builds the two Redis-backed tiers used by main.go: the
// auth tier (AuthRedisRateLimiterConfig) and the default tier
// (DefaultRedisRateLimiterConfig), each falling back to ITS OWN in-memory
// limiter when Redis errors after startup (CAT-OPDEC-A, CAT-OPA-1).
func NewRedisTierLimiters(client *redis.Client, authFallback, defaultFallback gin.HandlerFunc) (auth, def gin.HandlerFunc) {
	authCfg := AuthRedisRateLimiterConfig(client)
	authCfg.Fallback = authFallback
	defCfg := DefaultRedisRateLimiterConfig(client)
	defCfg.Fallback = defaultFallback
	return RedisRateLimit(authCfg), RedisRateLimit(defCfg)
}

// retryAfterSeconds renders a back-off as the whole-second integer value of the
// Retry-After header (RFC 9110 §10.2.3), rounded UP and never below 1 so a
// client that waits it out is not refused again.
func retryAfterSeconds(d time.Duration) string {
	secs := int64((d + time.Second - 1) / time.Second)
	if secs < 1 {
		secs = 1
	}
	return strconv.FormatInt(secs, 10)
}
