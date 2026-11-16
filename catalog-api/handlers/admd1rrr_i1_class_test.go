package handlers

// ADMD1RRR-I1 class sweep (§11.4.276(C)): the other write handlers whose check-then-insert can lose a race and reach the
// client as 500 instead of 409: self-registration (users.username/email unique) and storage-root creation
// (storage_roots.name unique). Real routers, real migrations, real repositories; N goroutines release together on a
// barrier so the existence check passes for several of them and the loser hits the database constraint. The PostgreSQL
// leg needs CAT_TEST_PG_DSN (honest SKIP otherwise). The outcome is race-dependent for the unfixed code; with the fix it
// is deterministic: exactly one 201/200-class winner, every other answer 409, never a 5xx.

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func raceCodes(n int, do func(i int) int) map[int]int {
	codes := map[int]int{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			c := do(i)
			mu.Lock()
			codes[c]++
			mu.Unlock()
		}(i)
	}
	close(start)
	wg.Wait()
	return codes
}

func registerRaceChecks(t *testing.T, e *rtEnv) {
	t.Setenv("REGISTRATION_MODE", "open")
	ah := NewAuthHandler(e.authSvc)
	r := gin.New()
	r.POST("/api/v1/auth/register", func(c *gin.Context) { ah.RegisterGin(c, e.repo) })
	for round := 0; round < 4; round++ {
		name := fmt.Sprintf("racer%d", round)
		body := fmt.Sprintf(`{"username":%q,"email":%q,"password":"Pw!Throwaway#9x","first_name":"R","last_name":"R"}`, name, name+"@example.test")
		codes := raceCodes(24, func(int) int {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader([]byte(body)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			return rec.Code
		})
		assert.Equal(t, 1, codes[http.StatusCreated], "round %d: exactly one registration wins: %v", round, codes)
		assert.Equal(t, 23, codes[http.StatusConflict], "round %d: every loser sees 409: %v", round, codes)
		assert.Zero(t, codes[http.StatusInternalServerError], "round %d: a duplicate must never be a 500: %v", round, codes)
	}
}

func TestADMD1RRRi1_ConcurrentRegistrationIs409Not500_SQLite(t *testing.T) {
	registerRaceChecks(t, newRTEnv(t))
}

func TestADMD1RRRi1_ConcurrentRegistrationIs409Not500_PostgresLive(t *testing.T) {
	registerRaceChecks(t, newRTEnvPG(t))
}

func TestADMD1RRRi1_ConcurrentStorageRootCreateIs409Not500_SQLite(t *testing.T) {
	base := t.TempDir()
	a := newAccessHarness(t, base)
	admin := cat001Token(t, 1, "1")
	for round := 0; round < 4; round++ {
		name := fmt.Sprintf("raceroot%d", round)
		dir := base
		codes := raceCodes(16, func(int) int {
			code, _ := a.do(admin, "POST", "/storage/roots", map[string]interface{}{
				"name": name, "protocol": "local", "path": dir, "confirm_private_network": true})
			return code
		})
		assert.Equal(t, 1, codes[http.StatusCreated], "round %d: exactly one create wins: %v", round, codes)
		assert.Equal(t, 15, codes[http.StatusConflict], "round %d: every loser sees 409: %v", round, codes)
		assert.Zero(t, codes[http.StatusInternalServerError], "round %d: %v", round, codes)
	}
}
