package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"catalogizer/middleware"
	"catalogizer/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// stubAuth stands in for RequireAuth: it sets user_id (int) from X-Test-User.
func stubAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := strconv.Atoi(c.GetHeader("X-Test-User"))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Set("user_id", id)
		c.Next()
	}
}

func passthrough() gin.HandlerFunc { return func(c *gin.Context) { c.Next() } }

type chpwEnv struct {
	r   *gin.Engine
	ids [2]int
}

// newChpwEnv builds the REAL ChangePassword handler on a real sqlite DB with two
// users (password "oldpassword123"), wired through the production
// RegisterChangePassword when withLimiter, else the pre-fix chain.
func newChpwEnv(t *testing.T, withLimiter bool, max int) *chpwEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db, cleanup := setupMediaQueryTestDB(t)
	t.Cleanup(cleanup)
	h, _, userRepo := setupMediaQueryHandler(t, db)
	env := &chpwEnv{}
	salt := "testsalt1234567890abcdef01234567"
	hash, err := bcrypt.GenerateFromPassword([]byte("oldpassword123"+salt), bcrypt.MinCost)
	require.NoError(t, err)
	for i, name := range []string{"alice", "bob"} {
		id, err := userRepo.Create(&models.User{Username: name, Email: name + "@example.com",
			PasswordHash: string(hash), Salt: salt, RoleID: 1, IsActive: true})
		require.NoError(t, err)
		env.ids[i] = id
	}
	env.r = gin.New()
	g := env.r.Group("/api/v1/auth")
	if withLimiter {
		middleware.RegisterChangePassword(g, "/change-password", passthrough(), stubAuth(), h.ChangePassword,
			middleware.NewFailedAttemptLimiter(max, middleware.CredentialFailureWindow))
	} else {
		g.POST("/change-password", passthrough(), stubAuth(), h.ChangePassword)
	}
	return env
}

func (e *chpwEnv) call(user int, current string) int {
	body, _ := json.Marshal(map[string]string{"current_password": current, "new_password": "brandnewpass1"})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-User", strconv.Itoa(user))
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w.Code
}

func TestChpwLimiter_ExactBoundary(t *testing.T) {
	e := newChpwEnv(t, true, 5)
	for i := 0; i < 5; i++ {
		require.Equal(t, http.StatusUnauthorized, e.call(e.ids[0], "bad"), "failure %d is answered", i+1)
	}
	require.Equal(t, http.StatusTooManyRequests, e.call(e.ids[0], "bad"), "6th is refused")
}

func TestChpwLimiter_LockedUserRefusedEvenWithCorrectPassword(t *testing.T) {
	e := newChpwEnv(t, true, 5)
	for i := 0; i < 5; i++ {
		e.call(e.ids[0], "bad")
	}
	require.Equal(t, http.StatusTooManyRequests, e.call(e.ids[0], "oldpassword123"))
}

func TestChpwLimiter_PerUserIsolation(t *testing.T) {
	e := newChpwEnv(t, true, 5)
	for i := 0; i < 20; i++ {
		e.call(e.ids[0], "bad")
	}
	require.Equal(t, http.StatusTooManyRequests, e.call(e.ids[0], "bad"))
	// bob is untouched by alice's failures and still succeeds.
	require.Equal(t, http.StatusOK, e.call(e.ids[1], "oldpassword123"))
}

func TestChpwLimiter_SuccessResetsOwnCounter(t *testing.T) {
	e := newChpwEnv(t, true, 5)
	for i := 0; i < 4; i++ {
		require.Equal(t, http.StatusUnauthorized, e.call(e.ids[0], "bad"))
	}
	require.Equal(t, http.StatusOK, e.call(e.ids[0], "oldpassword123")) // password is now brandnewpass1
	for i := 0; i < 5; i++ {                                            // fresh budget of 5
		require.Equal(t, http.StatusUnauthorized, e.call(e.ids[0], "bad"), "attempt %d", i+1)
	}
	require.Equal(t, http.StatusTooManyRequests, e.call(e.ids[0], "bad"))
}

func TestChpwLimiter_UnknownUserAndMissingIdentityNotCounted(t *testing.T) {
	e := newChpwEnv(t, true, 2)
	for i := 0; i < 10; i++ {
		require.Equal(t, http.StatusNotFound, e.call(9999, "bad")) // 404, not a verification failure
	}
	require.Equal(t, http.StatusNotFound, e.call(9999, "bad"))
	require.Equal(t, http.StatusUnauthorized, e.call(e.ids[0], "bad")) // alice still has her budget
}

func TestChpwLimiter_ConcurrentGuessesStopAtLimit(t *testing.T) {
	e := newChpwEnv(t, true, 5)
	var wg sync.WaitGroup
	var mu sync.Mutex
	counts := map[int]int{}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := e.call(e.ids[0], "bad")
			mu.Lock()
			counts[c]++
			mu.Unlock()
		}()
	}
	wg.Wait()
	// In-flight attempts hold budget: exactly max are evaluated, the rest refused.
	require.Equal(t, 5, counts[http.StatusUnauthorized])
	require.Equal(t, 95, counts[http.StatusTooManyRequests])
	require.Equal(t, http.StatusTooManyRequests, e.call(e.ids[0], "bad"))
}
