package middleware

import (
	"net/http"
	"net/http/httptest"
	"net/http/pprof"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SECQ-3 / AZ2-5: the REAL net/http/pprof handlers behind the guard, served by a real http.Server whose WriteTimeout is the
// shipped default (900 s), so the stdlib's own "seconds exceeds WriteTimeout" check does not hide the hold.
func pprofGuardServer(t *testing.T, maxSeconds int) *httptest.Server {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	g := r.Group("/debug/pprof")
	g.Use(PprofSamplingGuard(maxSeconds))
	g.GET("/profile", gin.WrapF(pprof.Profile))
	g.GET("/trace", gin.WrapF(pprof.Trace))
	g.GET("/heap", gin.WrapH(pprof.Handler("heap")))
	srv := httptest.NewUnstartedServer(r)
	srv.Config.WriteTimeout = 900 * time.Second
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

func getWithin(t *testing.T, url string, d time.Duration) (int, bool) {
	t.Helper()
	cl := &http.Client{Timeout: d}
	resp, err := cl.Get(url)
	if err != nil {
		return 0, false
	}
	defer resp.Body.Close()
	return resp.StatusCode, true
}

func TestPprofGuard_RejectsOversizedSamplingWindowImmediately(t *testing.T) {
	srv := pprofGuardServer(t, 5)
	for _, p := range []string{"/debug/pprof/profile?seconds=600", "/debug/pprof/trace?seconds=600"} {
		code, answered := getWithin(t, srv.URL+p, 2*time.Second)
		require.True(t, answered, p+": the request was held instead of refused")
		assert.Equal(t, http.StatusBadRequest, code, p)
	}
}

func TestPprofGuard_RejectsMalformedAndNonPositiveSeconds(t *testing.T) {
	srv := pprofGuardServer(t, 5)
	for _, q := range []string{"seconds=abc", "seconds=-1", "seconds=0", "seconds=1e9"} {
		code, answered := getWithin(t, srv.URL+"/debug/pprof/profile?"+q, 2*time.Second)
		require.True(t, answered, q)
		assert.Equal(t, http.StatusBadRequest, code, q)
	}
}

func TestPprofGuard_AllowsInCapSampleAndSnapshots(t *testing.T) {
	srv := pprofGuardServer(t, 5)
	code, answered := getWithin(t, srv.URL+"/debug/pprof/profile?seconds=1", 10*time.Second)
	require.True(t, answered)
	assert.Equal(t, http.StatusOK, code, "an in-cap profile must still be served")
	code, answered = getWithin(t, srv.URL+"/debug/pprof/heap", 5*time.Second)
	require.True(t, answered)
	assert.Equal(t, http.StatusOK, code, "snapshot endpoints are not sampling windows and stay untouched")
}

func TestPprofGuard_OnlyOneSamplingWindowAtATime(t *testing.T) {
	srv := pprofGuardServer(t, 5)
	var wg sync.WaitGroup
	first := make(chan int, 1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		code, _ := getWithin(t, srv.URL+"/debug/pprof/profile?seconds=2", 10*time.Second)
		first <- code
	}()
	time.Sleep(600 * time.Millisecond)
	code, answered := getWithin(t, srv.URL+"/debug/pprof/trace?seconds=1", 3*time.Second)
	require.True(t, answered)
	assert.Equal(t, http.StatusTooManyRequests, code, "a second concurrent sampling window must be refused")
	wg.Wait()
	assert.Equal(t, http.StatusOK, <-first)
	code, _ = getWithin(t, srv.URL+"/debug/pprof/trace?seconds=1", 5*time.Second)
	assert.Equal(t, http.StatusOK, code, "the slot is released after the first window ends")
}
