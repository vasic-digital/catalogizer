package middleware

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TIERTEST-M1. Unit-class: the production guard through gin, no doubles. Competitors ask for a 10 s
// window (not 1 s), so a Retry-After read from an unrecorded window end (0 -> "1") or from a previous
// window is far outside [ceil(10-elapsed), 10] and the test fails WITHOUT -race. The window is held
// by a handler blocked on a channel, so exactly one request owns the slot while the others are refused.
func TestPprofSamplingGuard_BusyRetryAfterBracketedWithoutRace(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const iterations = 1500
	competitors := 6
	for it := 0; it < iterations; it++ {
		done := make(chan struct{})
		r := gin.New()
		r.GET("/debug/pprof/profile", PprofSamplingGuard(30), func(c *gin.Context) {
			<-done
			c.Status(http.StatusOK)
		})
		type res struct {
			code int
			ra   string
		}
		results := make(chan res, competitors)
		start := make(chan struct{})
		var wg sync.WaitGroup
		for i := 0; i < competitors; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				w := httptest.NewRecorder()
				r.ServeHTTP(w, httptest.NewRequest("GET", "/debug/pprof/profile?seconds=10", nil))
				results <- res{w.Code, w.Header().Get("Retry-After")}
			}()
		}
		t0 := time.Now()
		close(start)
		// Release the held window as soon as the first refusal arrives, so later competitors race
		// the release (and the reset of the recorded end) against their own read of that end.
		var all []res
		for len(all) < competitors {
			x := <-results
			all = append(all, x)
			if len(all) == 1 {
				close(done)
			}
		}
		wg.Wait()
		t1 := time.Now()
		refused := all[:0:0]
		for _, x := range all {
			if x.code == http.StatusTooManyRequests {
				refused = append(refused, x)
			}
		}
		require.NotEmpty(t, refused, "iteration %d: nobody was refused", it)
		lo := int(math.Ceil(10 - t1.Sub(t0).Seconds()))
		if lo < 1 {
			lo = 1
		}
		for _, x := range refused {
			n, err := strconv.Atoi(x.ra)
			require.NoError(t, err)
			require.GreaterOrEqual(t, n, lo, "iteration %d: Retry-After %d below the bracket", it, n)
			require.LessOrEqual(t, n, 10, "iteration %d: Retry-After %d above the 10 s window", it, n)
		}
	}
}

// TIERTEST-M2. After a window is released a new one must report ITS OWN end, never the finished
// window's: 30 s window released, then a 2 s window held; a refusal must read [1,2], not ~30.
func TestPprofSamplingGuard_ReleaseThenNewWindowReportsOwnEnd(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hold := make(chan struct{})
	entered := make(chan struct{}, 1)
	r := gin.New()
	r.GET("/debug/pprof/profile", PprofSamplingGuard(30), func(c *gin.Context) {
		if c.Query("seconds") == "2" {
			entered <- struct{}{}
			<-hold
		}
		c.Status(http.StatusOK)
	})
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/debug/pprof/profile?seconds=30", nil))
	go r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/debug/pprof/profile?seconds=2", nil))
	<-entered
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/debug/pprof/profile?seconds=30", nil))
	close(hold)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	n, err := strconv.Atoi(w.Header().Get("Retry-After"))
	require.NoError(t, err)
	require.GreaterOrEqual(t, n, 1)
	require.LessOrEqual(t, n, 2, "refusal read the finished 30 s window's end")
}

// TIERFU-M1. Competitors ask a DIFFERENT window (3 s) than the holder (10 s): a Retry-After taken from the
// refused request's own seconds (<=3) can never satisfy the bracket [ceil(10-elapsed), 10].
func TestPprofSamplingGuard_RefusalReportsHolderWindowNotOwnSeconds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hold := make(chan struct{})
	defer close(hold)
	entered := make(chan struct{}, 1)
	r := gin.New()
	r.GET("/debug/pprof/profile", PprofSamplingGuard(30), func(c *gin.Context) {
		entered <- struct{}{}
		<-hold
		c.Status(http.StatusOK)
	})
	t0 := time.Now()
	go r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/debug/pprof/profile?seconds=10", nil))
	<-entered
	for i := 0; i < 5; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", "/debug/pprof/profile?seconds=3", nil))
		require.Equal(t, http.StatusTooManyRequests, w.Code)
		n, err := strconv.Atoi(w.Header().Get("Retry-After"))
		require.NoError(t, err)
		lo := int(math.Ceil(10 - time.Since(t0).Seconds()))
		require.GreaterOrEqual(t, n, lo, "Retry-After %d below the holder window bracket (own seconds used?)", n)
		require.LessOrEqual(t, n, 10)
	}
}
