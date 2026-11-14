package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
	_ "github.com/mutecomm/go-sqlcipher"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-LOGIN-LOCK-1: behavioural tests through the REAL router + login handler + AuthService + repository + migrated SQLite
// (production migration chain, real bcrypt). Unit-class evidence (constitution 11.4.289): no PostgreSQL instance and no
// deployed binary is exercised here. Throwaway credentials only.
const (
	llPass  = "Vict1m!Throwaway#9"
	llOther = "0ther!Throwaway#4"
)

type llEnv struct {
	r    *gin.Engine
	db   *database.DB
	raw  *sql.DB
	repo *repository.UserRepository
	svc  *services.AuthService
	uid  int
	oid  int
	nowN atomic.Int64 // unix ms of the injected clock
}

func (e *llEnv) clock() time.Time        { return time.UnixMilli(e.nowN.Load()) }
func (e *llEnv) advance(d time.Duration) { e.nowN.Add(d.Milliseconds()) }

func newLLEnv(t *testing.T) *llEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	raw, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	raw.SetMaxOpenConns(1)
	db := database.WrapDB(raw, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	e := &llEnv{db: db, raw: raw}
	e.nowN.Store(time.Date(2026, 11, 13, 12, 0, 0, 0, time.UTC).UnixMilli())
	e.wire(t)
	hash, salt, err := e.svc.HashPasswordForUser(llPass)
	require.NoError(t, err)
	id, err := e.repo.Create(&models.User{Username: "victim", Email: "victim@example.test", PasswordHash: hash, Salt: salt, RoleID: 2, IsActive: true})
	require.NoError(t, err)
	e.uid = int(id)
	hash2, salt2, err := e.svc.HashPasswordForUser(llOther)
	require.NoError(t, err)
	id2, err := e.repo.Create(&models.User{Username: "bystander", Email: "bystander@example.test", PasswordHash: hash2, Salt: salt2, RoleID: 2, IsActive: true})
	require.NoError(t, err)
	e.oid = int(id2)
	return e
}

// wire (re)builds repository, service and router over the same database: a "restart" of the process.
func (e *llEnv) wire(t *testing.T) {
	e.repo = repository.NewUserRepository(e.db)
	e.repo.LoginThrottle().SetClock(e.clock)
	e.svc = services.NewAuthService(e.repo, "test-jwt-secret-throwaway")
	e.r = gin.New()
	require.NoError(t, e.r.SetTrustedProxies(nil))
	e.r.POST("/api/v1/auth/login", NewAuthHandler(e.svc).LoginGin)
}

func (e *llEnv) loginFrom(ip, user, pass string) *httptest.ResponseRecorder {
	return e.loginHdr(ip, user, pass, nil)
}

func (e *llEnv) loginHdr(ip, user, pass string, hdr map[string]string) *httptest.ResponseRecorder {
	body, err := json.Marshal(map[string]string{"username": user, "password": pass}) // %q is not JSON (NUL, \x escapes)
	if err != nil {
		panic(err)
	}
	req := httptest.NewRequest("POST", "/api/v1/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	req.RemoteAddr = ip + ":40000"
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

func (e *llEnv) failedColumn(t *testing.T, uid int) int {
	var n int
	require.NoError(t, e.db.QueryRow(`SELECT failed_login_attempts FROM users WHERE id = ?`, uid).Scan(&n))
	return n
}

func distinctIP(i int) string { return fmt.Sprintf("10.%d.%d.%d", 1+i/62500, (i/250)%250, 1+i%250) }

func wrong(i int) string { return fmt.Sprintf("Wr0ng!guess#%d", i) }

// attackAccount makes n wrong guesses at "victim" from n distinct addresses and returns how many were evaluated (401).
func (e *llEnv) attackAccount(n, offset int) (processed, refused int) {
	for i := 0; i < n; i++ {
		switch e.loginFrom(distinctIP(offset+i), "victim", wrong(i)).Code {
		case http.StatusUnauthorized:
			processed++
		case http.StatusTooManyRequests:
			refused++
		}
	}
	return
}

// Reproduction + fix: 120 wrong passwords for ONE account from 120 distinct addresses. On the base commit every one was
// evaluated (401, failed_login_attempts = 120, no 429); now exactly AccountFree (20) are evaluated and the rest refused.
func TestLoginLock_DistributedGuessingOfOneAccountIsBounded(t *testing.T) {
	e := newLLEnv(t)
	processed, refused := e.attackAccount(120, 0)
	t.Logf("processed(401)=%d refused(429)=%d failed_login_attempts column=%d", processed, refused, e.failedColumn(t, e.uid))
	assert.Equal(t, 20, processed)
	assert.Equal(t, 100, refused)
	assert.Equal(t, 20, e.failedColumn(t, e.uid), "refused attempts are not evaluated")
}

func TestLoginLock_ThrottleIsTemporaryVictimLogsInAfterBackoff(t *testing.T) {
	e := newLLEnv(t)
	e.attackAccount(30, 0)
	w := e.loginFrom("192.0.2.77", "victim", llPass)
	require.Equal(t, http.StatusTooManyRequests, w.Code, "right password from a new address is also held while the account is throttled")
	assert.NotEmpty(t, w.Header().Get("Retry-After"))
	e.advance(31 * time.Second)
	w = e.loginFrom("192.0.2.77", "victim", llPass)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
}

func TestLoginLock_BackoffDoublesAndIsCappedNeverPermanent(t *testing.T) {
	e := newLLEnv(t)
	var delays []int
	// Keep guessing from fresh addresses; each time the block ends, one more charged attempt doubles the next one.
	for i := 0; i < 20; i++ {
		e.loginFrom(distinctIP(i), "victim", wrong(i)) // reaches AccountFree at the 20th
	}
	for round := 0; round < 14; round++ {
		w := e.loginFrom(distinctIP(100+round), "victim", wrong(round))
		require.Equal(t, http.StatusTooManyRequests, w.Code)
		ra, err := strconv.Atoi(w.Header().Get("Retry-After"))
		require.NoError(t, err)
		delays = append(delays, ra)
		e.advance(time.Duration(ra)*time.Second + time.Second)
		w = e.loginFrom(distinctIP(200+round), "victim", wrong(round)) // evaluated again, charged, new longer block
		require.Equal(t, http.StatusUnauthorized, w.Code)
	}
	assert.Equal(t, 30, delays[0], "first block after AccountFree: Base")
	assert.Equal(t, 60, delays[1]) // 30 s block, then the next charge doubles
	assert.Equal(t, 120, delays[2])
	for _, d := range delays {
		assert.LessOrEqual(t, d, 900, "never longer than Cap (15 min)")
	}
	assert.Equal(t, 900, delays[len(delays)-1], "capped")
	// permanent? no: after the cap elapses the owner logs in.
	e.advance(16 * time.Minute)
	assert.Equal(t, http.StatusOK, e.loginFrom("192.0.2.9", "victim", llPass).Code)
}

func TestLoginLock_KnownGoodAddressBypassesAccountThrottle(t *testing.T) {
	e := newLLEnv(t)
	require.Equal(t, http.StatusOK, e.loginFrom("198.51.100.5", "victim", llPass).Code) // owner's usual address
	e.attackAccount(40, 0)
	assert.Equal(t, http.StatusTooManyRequests, e.loginFrom("203.0.113.9", "victim", llPass).Code, "new address is held")
	w := e.loginFrom("198.51.100.5", "victim", llPass)
	assert.Equal(t, http.StatusOK, w.Code, "the owner's known address still logs in during an attack: "+w.Body.String())
	// the owner's success must NOT end the account block: that would tell a real account from a nonexistent name (S1)
	assert.Equal(t, http.StatusTooManyRequests, e.loginFrom("203.0.113.9", "victim", llPass).Code)
}

func TestLoginLock_SpoofedKnownAddressBuysOnlyItsOwnBudget(t *testing.T) {
	e := newLLEnv(t)
	require.Equal(t, http.StatusOK, e.loginFrom("198.51.100.5", "victim", llPass).Code)
	e.attackAccount(40, 0)
	processed := 0
	for i := 0; i < 50; i++ { // attacker sits on the known address and guesses
		if e.loginFrom("198.51.100.5", "victim", wrong(i)).Code == http.StatusUnauthorized {
			processed++
		}
	}
	assert.Equal(t, 5, processed, "a known address is still bounded by its per-address budget (BucketFree)")
}

func TestLoginLock_PerAddressBucketStopsOneClientButNotOthers(t *testing.T) {
	e := newLLEnv(t)
	for i := 0; i < 5; i++ {
		require.Equal(t, http.StatusUnauthorized, e.loginFrom("203.0.113.1", "victim", wrong(i)).Code)
	}
	assert.Equal(t, http.StatusTooManyRequests, e.loginFrom("203.0.113.1", "victim", llPass).Code, "6th attempt from the same address is held")
	assert.Equal(t, http.StatusOK, e.loginFrom("203.0.113.2", "victim", llPass).Code, "another address is not affected (account budget not spent)")
}

func TestLoginLock_PerAccountIsolation(t *testing.T) {
	e := newLLEnv(t)
	e.attackAccount(40, 0)
	assert.Equal(t, http.StatusOK, e.loginFrom("203.0.113.50", "bystander", llOther).Code, "another account is unaffected")
}

func TestLoginLock_ResponsesIndistinguishableUnknownKnownAndThrottled(t *testing.T) {
	e := newLLEnv(t)
	type shape struct {
		code      int
		body      string
		retry     string
		ctype     string
		hasCookie bool
	}
	get := func(w *httptest.ResponseRecorder) shape {
		return shape{w.Code, w.Body.String(), w.Header().Get("Retry-After"), w.Header().Get("Content-Type"), w.Header().Get("Set-Cookie") != ""}
	}
	for i := 0; i < 8; i++ {
		known := get(e.loginFrom("203.0.113.1", "victim", wrong(i)))
		unknown := get(e.loginFrom("203.0.113.1", "no-such-account", wrong(i)))
		assert.Equal(t, unknown, known, "attempt %d: known vs unknown name", i+1)
		if i < 5 {
			assert.Equal(t, 401, known.code)
		} else {
			assert.Equal(t, 429, known.code, "attempt %d is throttled for both", i+1)
			assert.NotEmpty(t, known.retry)
		}
	}
	// the throttled answer is the same for an account-level block as for an address-level one
	e2 := newLLEnv(t)
	e2.attackAccount(25, 0)
	a := get(e2.loginFrom("192.0.2.200", "victim", wrong(1)))
	f := newLLEnv(t)
	for i := 0; i < 25; i++ {
		f.loginFrom(distinctIP(i), "no-such-account", wrong(i))
	}
	b := get(f.loginFrom("192.0.2.200", "no-such-account", wrong(1)))
	assert.Equal(t, b, a, "account-level throttle: known vs unknown name")
}

func TestLoginLock_ThrottledAnswerDoesNotRevealSuspensionOrExistenceByTiming(t *testing.T) {
	e := newLLEnv(t)
	_, err := e.db.Exec(`UPDATE users SET is_locked = 1, locked_until = NULL WHERE id = ?`, e.oid) // suspended bystander
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		assert.Equal(t, http.StatusUnauthorized, e.loginFrom("203.0.113.1", "bystander", wrong(i)).Code)
	}
	assert.Equal(t, http.StatusTooManyRequests, e.loginFrom("203.0.113.1", "bystander", llOther).Code, "even the right password gets the generic throttle answer")
}

func TestLoginLock_SuccessResetsCharge(t *testing.T) {
	e := newLLEnv(t)
	for round := 0; round < 3; round++ {
		for i := 0; i < 4; i++ {
			require.Equal(t, http.StatusUnauthorized, e.loginFrom("203.0.113.1", "victim", wrong(i)).Code)
		}
		require.Equal(t, http.StatusOK, e.loginFrom("203.0.113.1", "victim", llPass).Code, "round %d", round)
	}
}

func TestLoginLock_StatePersistsAcrossRestart(t *testing.T) {
	e := newLLEnv(t)
	e.attackAccount(20, 0)
	e.wire(t) // new repository, service, router over the same database
	assert.Equal(t, http.StatusTooManyRequests, e.loginFrom("192.0.2.1", "victim", llPass).Code)
	e.advance(31 * time.Second)
	assert.Equal(t, http.StatusOK, e.loginFrom("192.0.2.1", "victim", llPass).Code)
}

func TestLoginLock_ConcurrentAttemptsAreChargedExactly(t *testing.T) {
	t.Run("one address, 5 free", func(t *testing.T) {
		e := newLLEnv(t)
		// the per-address bucket is charged before the check and is exact; lift the in-flight bound so it is what is measured
		e.repo.LoginThrottle().SetPolicy(repository.LoginThrottlePolicy{MaxInflight: 1000})
		var wg sync.WaitGroup
		var ok401, ok429 atomic.Int32
		for i := 0; i < 40; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				switch e.loginFrom("203.0.113.1", "victim", wrong(i)).Code {
				case http.StatusUnauthorized:
					ok401.Add(1)
				case http.StatusTooManyRequests:
					ok429.Add(1)
				}
			}(i)
		}
		wg.Wait()
		assert.EqualValues(t, 5, ok401.Load(), "exactly BucketFree evaluations")
		assert.EqualValues(t, 35, ok429.Load())
		assert.Equal(t, 5, e.failedColumn(t, e.uid))
	})
	t.Run("many addresses, 20 free plus the in-flight slack", func(t *testing.T) {
		e := newLLEnv(t)
		var wg sync.WaitGroup
		var ok401, ok429 atomic.Int32
		for i := 0; i < 60; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				switch e.loginFrom(distinctIP(i), "victim", wrong(i)).Code {
				case http.StatusUnauthorized:
					ok401.Add(1)
				case http.StatusTooManyRequests:
					ok429.Add(1)
				}
			}(i)
		}
		wg.Wait()
		// CAT-LOGIN-LOCK-2: parallel guesses are bounded by in-flight tokens (4), so a burst can evaluate at most
		// AccountFree + MaxInflight - 1 guesses; a serial guesser still gets exactly AccountFree (see the distributed test).
		assert.LessOrEqual(t, int(ok401.Load()), 20+3, "AccountFree + MaxInflight - 1 evaluations at most")
		assert.GreaterOrEqual(t, int(ok401.Load()), 4, "the burst is not refused wholesale")
		assert.EqualValues(t, 60, ok401.Load()+ok429.Load())
	})
}

func TestLoginLock_AdminUnlockAndPasswordChangeClearBackoff(t *testing.T) {
	e := newLLEnv(t)
	e.attackAccount(25, 0)
	require.Equal(t, http.StatusTooManyRequests, e.loginFrom("192.0.2.1", "victim", llPass).Code)
	require.NoError(t, e.svc.UnlockAccount(e.uid))
	assert.Equal(t, http.StatusOK, e.loginFrom("192.0.2.1", "victim", llPass).Code, "admin unlock ends the back-off")

	e.attackAccount(25, 1000)
	require.Equal(t, http.StatusTooManyRequests, e.loginFrom("192.0.2.2", "victim", llPass).Code)
	h, s, err := e.svc.HashPasswordForUser("N3w!Throwaway#5")
	require.NoError(t, err)
	require.NoError(t, e.repo.UpdatePassword(e.uid, h, s))
	assert.Equal(t, http.StatusOK, e.loginFrom("192.0.2.2", "victim", "N3w!Throwaway#5").Code, "a new password ends the back-off")
}

func TestLoginLock_AttackDoesNotSuspendAccountOrRevokeSessions(t *testing.T) {
	e := newLLEnv(t)
	w := e.loginFrom("198.51.100.5", "victim", llPass)
	require.Equal(t, http.StatusOK, w.Code)
	e.attackAccount(100, 0)
	var locked bool
	var until sql.NullString
	require.NoError(t, e.db.QueryRow(`SELECT is_locked, locked_until FROM users WHERE id = ?`, e.uid).Scan(&locked, &until))
	assert.False(t, locked, "the throttle never writes users.is_locked")
	assert.False(t, until.Valid)
	var active int
	require.NoError(t, e.db.QueryRow(`SELECT COUNT(*) FROM user_sessions WHERE user_id = ? AND is_active = ?`, e.uid, true).Scan(&active))
	assert.Equal(t, 1, active, "the victim's existing session survives the attack")
}

func TestLoginLock_UsernameAndEmailShareOneBudget(t *testing.T) {
	e := newLLEnv(t)
	processed := 0
	for i := 0; i < 40; i++ {
		name := "victim"
		if i%2 == 1 {
			name = "victim@example.test"
		}
		if e.loginFrom(distinctIP(i), name, wrong(i)).Code == http.StatusUnauthorized {
			processed++
		}
	}
	assert.Equal(t, 20, processed)
}

func TestLoginLock_ForgedForwardedForDoesNotMintAddresses(t *testing.T) {
	e := newLLEnv(t) // no trusted proxy: X-Forwarded-For is ignored, the peer address is used
	processed := 0
	for i := 0; i < 30; i++ {
		w := e.loginHdr("203.0.113.1", "victim", wrong(i), map[string]string{"X-Forwarded-For": distinctIP(i), "X-Real-IP": distinctIP(i + 500)})
		if w.Code == http.StatusUnauthorized {
			processed++
		}
	}
	assert.Equal(t, 5, processed, "spoofed forwarding headers do not create new per-address budgets")
}

func TestLoginLock_ThrottleStoreFailureFailsClosed(t *testing.T) {
	e := newLLEnv(t)
	_, err := e.db.Exec(`DROP TABLE login_throttle`)
	require.NoError(t, err)
	w := e.loginFrom("203.0.113.1", "victim", llPass)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code, "no silent fail-open: even the right password is not accepted without the back-off store")
}

func TestLoginLock_StaleRowsArePurgedAndBounded(t *testing.T) {
	e := newLLEnv(t)
	for i := 0; i < 30; i++ {
		e.loginFrom(distinctIP(i), fmt.Sprintf("ghost-%d", i), "x")
	}
	var n int
	require.NoError(t, e.db.QueryRow(`SELECT COUNT(*) FROM login_throttle`).Scan(&n))
	assert.Equal(t, 90, n, "three rows (account, address, per-subject slots lock row) per distinct unknown name")
	e.advance(25 * time.Hour)
	e.loginFrom("192.0.2.1", "victim", llPass)
	require.NoError(t, e.db.QueryRow(`SELECT COUNT(*) FROM login_throttle WHERE subject LIKE 'n:%'`).Scan(&n))
	assert.Equal(t, 0, n, "rows that can no longer matter are purged after StaleAfter")
}

func TestLoginLock_DecayRestartsCountAfterQuietPeriod(t *testing.T) {
	e := newLLEnv(t)
	for i := 0; i < 4; i++ {
		e.loginFrom("203.0.113.1", "victim", wrong(i))
	}
	e.advance(2 * time.Hour) // longer than BucketDecay
	for i := 0; i < 4; i++ {
		assert.Equal(t, http.StatusUnauthorized, e.loginFrom("203.0.113.1", "victim", wrong(i)).Code, "count restarted")
	}
}
