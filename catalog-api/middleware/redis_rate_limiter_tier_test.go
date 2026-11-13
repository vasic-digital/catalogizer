package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Unit-class (miniredis is a test double, §11.4.289). PKGC-FU4: each Redis
// tier owns its key namespace and its budget matches the in-memory tier
// (main.go: authRateLimiter 600/min, defaultRateLimiter 2000/min).

func tierRouter(auth, def gin.HandlerFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.GET("/default", def, ok)
	r.GET("/auth/me", auth, ok)
	return r
}

func tierGet(r *gin.Engine, path, ip string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", path, nil)
	req.RemoteAddr = ip + ":1234"
	r.ServeHTTP(w, req)
	return w
}

func tierClient(t *testing.T) (*redis.Client, func() []string) {
	mr, client := setupMiniredis(t)
	return client, mr.Keys
}

func TestTierBudgetsMatchInMemory(t *testing.T) {
	_, c := setupMiniredis(t)
	assert.Equal(t, 600, AuthRedisRateLimiterConfig(c).Requests)
	assert.Equal(t, 2000, DefaultRedisRateLimiterConfig(c).Requests)
	assert.Equal(t, "auth", AuthRedisRateLimiterConfig(c).Tier)
	assert.Equal(t, "default", DefaultRedisRateLimiterConfig(c).Tier)
}

// RED on main: both tiers INCR rate_limit:<ip>.
func TestTierKeysAreDistinct(t *testing.T) {
	client, keys := tierClient(t)
	auth, def := NewRedisTierLimiters(client, nil, nil)
	r := tierRouter(auth, def)
	require.Equal(t, 200, tierGet(r, "/default", "10.0.0.1").Code)
	require.Equal(t, 200, tierGet(r, "/auth/me", "10.0.0.1").Code)
	assert.ElementsMatch(t, []string{"rate_limit:auth:10.0.0.1", "rate_limit:default:10.0.0.1"}, keys())
}

// RED on main: 5 default-tier requests then /auth/me got 429 (auth limit 5, shared key).
func TestDefaultTierTrafficDoesNotExhaustAuthTier(t *testing.T) {
	client, _ := tierClient(t)
	auth, def := NewRedisTierLimiters(client, nil, nil)
	r := tierRouter(auth, def)
	for i := 0; i < 50; i++ {
		require.Equal(t, 200, tierGet(r, "/default", "10.0.0.1").Code)
	}
	for i := 0; i < 50; i++ {
		require.Equal(t, 200, tierGet(r, "/auth/me", "10.0.0.1").Code, "auth tier request %d", i)
	}
}

func TestAuthTierAllows600ThenBlocks601(t *testing.T) {
	client, _ := tierClient(t)
	auth, def := NewRedisTierLimiters(client, nil, nil)
	r := tierRouter(auth, def)
	for i := 1; i <= 600; i++ {
		w := tierGet(r, "/auth/me", "10.0.0.2")
		require.Equal(t, 200, w.Code, "request %d", i)
		if i == 1 {
			assert.Equal(t, "600", w.Header().Get("X-RateLimit-Limit"))
			assert.Equal(t, "599", w.Header().Get("X-RateLimit-Remaining"))
		}
	}
	w := tierGet(r, "/auth/me", "10.0.0.2")
	assert.Equal(t, 429, w.Code)
	assert.Contains(t, w.Body.String(), "rate_limit_exceeded")
	// other IP unaffected
	assert.Equal(t, 200, tierGet(r, "/auth/me", "10.0.0.3").Code)
}

func TestDefaultTierAllows2000ThenBlocks2001(t *testing.T) {
	client, _ := tierClient(t)
	auth, def := NewRedisTierLimiters(client, nil, nil)
	r := tierRouter(auth, def)
	for i := 1; i <= 2000; i++ {
		w := tierGet(r, "/default", "10.0.0.4")
		require.Equal(t, 200, w.Code, "request %d", i)
		if i == 1 {
			assert.Equal(t, "2000", w.Header().Get("X-RateLimit-Limit"))
		}
	}
	assert.Equal(t, 429, tierGet(r, "/default", "10.0.0.4").Code)
	// exhausting default does not touch auth
	assert.Equal(t, 200, tierGet(r, "/auth/me", "10.0.0.4").Code)
}

func TestSlidingWindowKeyIsTierScoped(t *testing.T) {
	client, keys := tierClient(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ok := func(c *gin.Context) { c.Status(http.StatusOK) }
	r.GET("/a", SlidingWindowRedisRateLimit(AuthRedisRateLimiterConfig(client)), ok)
	r.GET("/d", SlidingWindowRedisRateLimit(DefaultRedisRateLimiterConfig(client)), ok)
	tierGet(r, "/a", "10.0.0.5")
	tierGet(r, "/d", "10.0.0.5")
	assert.ElementsMatch(t, []string{"sliding_rate_limit:auth:10.0.0.5", "sliding_rate_limit:default:10.0.0.5"}, keys())
}

func TestUntieredConfigGetsOwnNamespace(t *testing.T) {
	client, keys := tierClient(t)
	cfg := DefaultRedisRateLimiterConfig(client)
	cfg.Tier = ""
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/x", RedisRateLimit(cfg), func(c *gin.Context) { c.Status(200) })
	tierGet(r, "/x", "10.0.0.6")
	assert.Equal(t, []string{"rate_limit:untiered:10.0.0.6"}, keys())
}

func TestTierKeysExpireWithinWindow(t *testing.T) {
	mr, client := setupMiniredis(t)
	auth, def := NewRedisTierLimiters(client, nil, nil)
	r := tierRouter(auth, def)
	tierGet(r, "/auth/me", "10.0.0.7")
	tierGet(r, "/default", "10.0.0.7")
	for _, k := range mr.Keys() {
		assert.Equal(t, 60_000_000_000, int(mr.TTL(k)), "key %s", k) // 1 minute
	}
}

// Credential brute-force routes are unchanged: /login and /register use the
// in-memory loginRateLimiter (30/min, main.go:973,1211-1212), never a Redis
// tier. Documented here as a route-class table; the wiring is in main.go.
func TestLoginRegisterAreNotRedisTiers(t *testing.T) {
	src, err := mainGoSource()
	require.NoError(t, err)
	assert.Contains(t, src, `authGroup.POST("/login", loginRateLimiter,`)
	assert.Contains(t, src, `authGroup.POST("/register", loginRateLimiter,`)
	assert.Contains(t, src, `loginRateLimiter := authMiddleware.RateLimitByUser(30, "1m")`)
	assert.Contains(t, src, `NewRedisTierLimiters(redisClient, authRateLimiter, defaultRateLimiter)`)
	assert.NotContains(t, src, "NewRedisTierLimiters(redisClient, loginRateLimiter")
}

func TestConfigTierNamesAreUnique(t *testing.T) {
	_, c := setupMiniredis(t)
	names := []string{
		AuthRedisRateLimiterConfig(c).Tier, DefaultRedisRateLimiterConfig(c).Tier,
		StrictRedisRateLimiterConfig(c).Tier, UserRedisRateLimiterConfig(c).Tier,
	}
	assert.Equal(t, []string{"auth", "default", "strict", "user"}, names)
}
