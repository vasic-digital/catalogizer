package middleware

import (
	"net/http"
	"net/http/httptest"
	"runtime"
	"strconv"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TIERTEST-RLW-F3. A finished 30 s window must never leak into the Retry-After of a
// later 1 s window: between "slot taken" and "window end recorded" a competitor used to
// read the PREVIOUS window's end. Unit-class: the production guard through gin, no doubles.
func TestPprofSamplingGuard_Busy429NeverReadsPreviousWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const iterations = 3000
	competitors := runtime.GOMAXPROCS(0) + 3
	stale := 0
	for it := 0; it < iterations; it++ {
		r := gin.New()
		r.GET("/debug/pprof/profile", PprofSamplingGuard(30), func(c *gin.Context) { c.Status(http.StatusOK) })
		// Window 1: 30 s, released at once, leaving its end behind.
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/debug/pprof/profile?seconds=30", nil))

		start := make(chan struct{})
		var wg sync.WaitGroup
		var mu sync.Mutex
		for i := 0; i < competitors; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest("GET", "/debug/pprof/profile?seconds=1", nil))
				if w.Code == http.StatusTooManyRequests {
					n, err := strconv.Atoi(w.Header().Get("Retry-After"))
					if err != nil || n > 1 {
						mu.Lock()
						stale++
						mu.Unlock()
					}
				}
			}()
		}
		close(start)
		wg.Wait()
	}
	require.Zero(t, stale, "429 carried the previous window's Retry-After (>1 s) %d times", stale)
}
