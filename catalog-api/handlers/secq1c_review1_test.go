package handlers

// CAT-SECQ1-C review round 1: route authz, unresolvable names, rebinding between the two classifications, and the per-user step-up throttle.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/internal/netguard"
	"catalogizer/models"
	"catalogizer/services"

	"github.com/stretchr/testify/require"
)

const secq1cUnresolvable = "nx-secq1c.invalid" // RFC 2606: never resolves

// freshThrottle swaps in a limiter with a controllable clock for the test.
func freshThrottle(t *testing.T) *time.Time {
	t.Helper()
	old := stepUpThrottle
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	l := newStepUpLimiter()
	l.now = func() time.Time { return now }
	stepUpThrottle = l
	t.Cleanup(func() { stepUpThrottle = old })
	return &now
}

// M1: the PUT class-change route is admin-only, through the REAL router and the REAL JWT middleware.
func TestSECQ1C_R1_ApprovedClassRouteIsAdminOnly(t *testing.T) {
	base := t.TempDir()
	a := newAccessHarness(t, base)
	admin, user := cat001Token(t, 1, "1"), cat001Token(t, 2, "2")
	code, body := a.do(admin, "POST", "/storage/roots", map[string]interface{}{"name": "loc", "protocol": "local", "path": base})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	path := "/storage/roots/" + idOf(body) + "/approved-class"

	code, out := a.do(user, "PUT", path, map[string]interface{}{"class": "public"})
	require.Equal(t, http.StatusForbidden, code, "a non-admin must not change the class: %v", out)
	var cls string
	require.NoError(t, a.h.db.QueryRow(`SELECT approved_class FROM storage_roots WHERE id = ?`, idOf(body)).Scan(&cls))
	require.Equal(t, "private", cls, "the refused request changed nothing")

	code, out = a.do(admin, "PUT", path, map[string]interface{}{"class": "public"})
	require.Equal(t, http.StatusOK, code, "%v", out) // narrowing needs no step-up
	require.NoError(t, a.h.db.QueryRow(`SELECT approved_class FROM storage_roots WHERE id = ?`, idOf(body)).Scan(&cls))
	require.Equal(t, "public", cls)
}

// M2: a destination that does not resolve is neither public nor free: it needs the operator's confirmation, then it is stored as private.
func TestSECQ1C_R1_UnresolvableRootNeedsConfirmation(t *testing.T) {
	h := secq1cHarness(t)
	req := map[string]interface{}{"name": "nx", "protocol": "webdav", "url": "http://" + secq1cUnresolvable + "/dav", "path": "/"}
	code, body := h.do("POST", "/storage/roots", req)
	require.Equal(t, http.StatusBadRequest, code, "%v", body)
	require.Equal(t, secq1cNoConfirm, body["code"])
	require.Equal(t, 0, rootCount(t, h.db))

	req["confirm_private_network"] = true
	code, body = h.do("POST", "/storage/roots", req)
	require.Equal(t, http.StatusCreated, code, "%v", body)
	cls, src := rootClass(t, h.db, "nx")
	require.Equal(t, "private", cls)
	require.Equal(t, "operator", src)
}

// M3: a name that classified public at the early check and re-resolves private at approval time (DNS rebinding) is refused with the
// row removed, even when the request carried confirm_private_network.
func TestSECQ1C_R1_RebindingBetweenClassificationsIsRefused(t *testing.T) {
	h := secq1cHarness(t)
	old := classifyDestination
	t.Cleanup(func() { classifyDestination = old })
	calls := 0
	classifyDestination = func(ctx context.Context, host, rawURL string) (netguard.Class, error) {
		calls++
		if calls == 1 {
			return netguard.ClassPublic, nil
		}
		return netguard.ClassPrivate, nil
	}
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "rb", "protocol": "webdav", "url": "https://rebind.example/dav", "path": "/", "confirm_private_network": true})
	require.Equal(t, http.StatusBadRequest, code, "%v", body)
	require.Equal(t, secq1cNoConfirm, body["code"])
	require.Equal(t, 0, rootCount(t, h.db), "the row is removed again")
	require.GreaterOrEqual(t, calls, 2, "control: the destination was classified twice")
}

// Finding 5 (storage route): N failed step-ups per user then 429 with Retry-After, even for the right password; other users are unaffected;
// the window ends; a success clears the count. X-Forwarded-For is irrelevant (the key is the user).
func TestSECQ1C_R1_StorageStepUpIsThrottledPerUser(t *testing.T) {
	now := freshThrottle(t)
	t.Setenv(netguard.AllowCIDRsEnv, "")
	h := secq1cHarness(t)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "pub", "protocol": "webdav", "url": "https://" + secq1cPublicIP + "/dav", "path": "/"})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	path := "/storage/roots/" + idOf(body) + "/approved-class"

	for i := 0; i < stepUpMaxFailures; i++ {
		code, body = h.do("PUT", path, map[string]interface{}{"class": "private", "step_up_password": fmt.Sprintf("wrong-%d", i)})
		require.Equal(t, http.StatusForbidden, code, "failure %d: %v", i, body)
	}
	raw, _ := json.Marshal(map[string]interface{}{"class": "private", "step_up_password": secq1cStepPW})
	req := httptest.NewRequest("PUT", path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Forwarded-For", "203.0.113.9") // a different claimed client address changes nothing
	w := httptest.NewRecorder()
	h.r.ServeHTTP(w, req)
	require.Equal(t, http.StatusTooManyRequests, w.Code, "locked out even for the right password: %s", w.Body.String())
	require.Contains(t, w.Body.String(), "step_up_throttled")
	require.Equal(t, strconv.Itoa(int(stepUpWindow/time.Second)), w.Header().Get("Retry-After"), "the whole window, the clock being frozen")
	cls, _ := rootClass(t, h.db, "pub")
	require.Equal(t, "public", cls, "a throttled request changes nothing")

	*now = now.Add(stepUpWindow + time.Second) // the window ends
	code, body = h.do("PUT", path, map[string]interface{}{"class": "private", "step_up_password": secq1cStepPW})
	require.Equal(t, http.StatusOK, code, "%v", body)
	require.Equal(t, 0, stepUpThrottle.size(), "a success clears the user")
}

// failN makes n failed attempts of user through the limiter, the way the routes do.
func failN(l *stepUpLimiter, user, n int) {
	for i := 0; i < n; i++ {
		if _, ok := l.acquire(user); ok {
			l.release(user, false)
		}
	}
}

func TestSECQ1C_R1_ThrottleIsPerUserAndBounded(t *testing.T) {
	now := freshThrottle(t)
	l := stepUpThrottle
	failN(l, 1, stepUpMaxFailures)
	_, ok := l.acquire(1)
	require.False(t, ok)
	_, ok = l.acquire(2)
	require.True(t, ok, "another user is not locked out")
	l.release(2, true)

	for u := 100; u < 100+stepUpMaxUsers+500; u++ { // more users than the table may hold
		failN(l, u, 1)
		*now = now.Add(time.Millisecond)
	}
	require.LessOrEqual(t, l.size(), stepUpMaxUsers, "the limiter state is bounded")

	*now = now.Add(stepUpWindow + time.Second)
	failN(l, 5, 1) // purge on the next write
	require.LessOrEqual(t, l.size(), 2, "expired entries are dropped")
}

// Survivor A: a success clears the failures of the user BEFORE the window ends.
func TestSECQ1C_R1_SuccessClearsFailuresWithinWindow(t *testing.T) {
	freshThrottle(t)
	l := stepUpThrottle
	failN(l, 3, stepUpMaxFailures-1)
	_, ok := l.acquire(3)
	require.True(t, ok)
	l.release(3, true)
	failN(l, 3, stepUpMaxFailures-1) // four more failures are fine again: the count restarted
	_, ok = l.acquire(3)
	require.True(t, ok, "after a success the failure count starts again, inside the same window")
	l.release(3, true)
}

// F1: reservations count in-flight attempts, so concurrent attempts cannot all pass the limit.
func TestSECQ1C_R1_LimiterReservesInFlightAttempts(t *testing.T) {
	freshThrottle(t)
	l := stepUpThrottle
	admitted := 0
	for i := 0; i < 40; i++ {
		if _, ok := l.acquire(9); ok {
			admitted++
		}
	}
	require.Equal(t, stepUpMaxFailures, admitted, "no more attempts than the limit are admitted while others are still running")
	wait, ok := l.acquire(9)
	require.False(t, ok)
	require.Equal(t, time.Second, wait, "blocked only by running attempts: a short wait")
	for i := 0; i < admitted; i++ {
		l.release(9, false)
	}
	wait, ok = l.acquire(9)
	require.False(t, ok)
	require.Equal(t, stepUpWindow, wait, "blocked by failures: the rest of the window")
}

// two administrators with passwords, through the real router and the real JWT middleware
func twoAdminsHarness(t *testing.T) (*accessHarness, string, string, string) {
	t.Helper()
	base := t.TempDir()
	a := newAccessHarness(t, base)
	for _, id := range []int{11, 12} {
		salt := "00112233445566778899aabbccddee0" + fmt.Sprint(id%10)
		hash, err := services.HashPasswordWithSalt(secq1cStepPW, salt)
		require.NoError(t, err)
		_, err = a.h.db.Exec(`INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (?, ?, ?, ?, ?, 1)`, id, fmt.Sprint("adm", id), fmt.Sprint("adm", id, "@example.test"), hash, salt)
		require.NoError(t, err)
	}
	t1, t2 := cat001Token(t, 1, "11"), cat001Token(t, 1, "12")
	code, body := a.do(t1, "POST", "/storage/roots", map[string]interface{}{"name": "pub", "protocol": "webdav", "url": "https://" + secq1cPublicIP + "/dav", "path": "/"})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	return a, t1, t2, "/storage/roots/" + idOf(body) + "/approved-class"
}

// Survivors D/D2: the limiter is keyed per user; user A's failures never lock user B (real routes, real tokens).
func TestSECQ1C_R1_FailuresOfOneUserDoNotLockAnother(t *testing.T) {
	freshThrottle(t)
	t.Setenv(netguard.AllowCIDRsEnv, "")
	a, tokA, tokB, path := twoAdminsHarness(t)
	for i := 0; i < stepUpMaxFailures; i++ {
		code, _ := a.do(tokA, "PUT", path, map[string]interface{}{"class": "private", "step_up_password": "wrong"})
		require.Equal(t, http.StatusForbidden, code)
	}
	code, body := a.do(tokA, "PUT", path, map[string]interface{}{"class": "private", "step_up_password": secq1cStepPW})
	require.Equal(t, http.StatusTooManyRequests, code, "user A is locked: %v", body)

	code, body = a.do(tokB, "PUT", path, map[string]interface{}{"class": "private", "step_up_password": "wrong"})
	require.Equal(t, http.StatusForbidden, code, "user B is not locked by user A's failures: %v", body)
	code, body = a.do(tokB, "PUT", path, map[string]interface{}{"class": "private", "step_up_password": secq1cStepPW})
	require.Equal(t, http.StatusOK, code, "%v", body)
	require.Contains(t, stepUpThrottle.m, 11)
	require.NotContains(t, stepUpThrottle.m, 0, "keyed by the acting user, never a constant")
}

// F1 (storage route): a concurrent burst of wrong step-ups runs at most the limit of password checks, the rest get 429, and every 429 says
// when to come back.
func TestSECQ1C_R1_StorageConcurrentBurstRunsAtMostLimitChecks(t *testing.T) {
	now := freshThrottle(t)
	t.Setenv(netguard.AllowCIDRsEnv, "")
	h := secq1cHarness(t)
	code, body := h.do("POST", "/storage/roots", map[string]interface{}{"name": "pub", "protocol": "webdav", "url": "https://" + secq1cPublicIP + "/dav", "path": "/"})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	path := "/storage/roots/" + idOf(body) + "/approved-class"
	const n = 40
	var wg sync.WaitGroup
	var mu sync.Mutex
	counts := map[int]int{}
	retry := map[string]int{}
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			raw, _ := json.Marshal(map[string]interface{}{"class": "private", "step_up_password": fmt.Sprintf("wrong-%d", i)})
			req := httptest.NewRequest("PUT", path, bytes.NewReader(raw))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			<-start
			h.r.ServeHTTP(w, req)
			mu.Lock()
			counts[w.Code]++
			if w.Code == http.StatusTooManyRequests {
				retry[w.Header().Get("Retry-After")]++
			}
			mu.Unlock()
		}(i)
	}
	close(start)
	wg.Wait()
	require.Equal(t, stepUpMaxFailures, counts[http.StatusForbidden], "exactly the limit of password checks ran: %v", counts)
	require.Equal(t, n-stepUpMaxFailures, counts[http.StatusTooManyRequests], "%v", counts)
	for v := range retry {
		require.Contains(t, []string{"1", strconv.Itoa(int(stepUpWindow / time.Second))}, v, "Retry-After is a wait the limiter computed: %v", retry)
	}
	// survivor C: the value is the rest of the window, not a constant
	*now = now.Add(100 * time.Second)
	raw, _ := json.Marshal(map[string]interface{}{"class": "private", "step_up_password": secq1cStepPW})
	req := httptest.NewRequest("PUT", path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	h.r.ServeHTTP(w, req)
	require.Equal(t, http.StatusTooManyRequests, w.Code)
	require.Equal(t, strconv.Itoa(int((stepUpWindow-100*time.Second)/time.Second)), w.Header().Get("Retry-After"))
}

// M (ID 0): a not-yet-stored endpoint never gets a cached client.
// L: approveNewRoot refuses a destination that is public now when it was private (or unresolved) when the operator was asked and
// the operator has not confirmed (a branch the early check of the create route normally pre-empts).
func TestSECQ1C_R1_ApproveNewRootRefusesPublicAfterPrivateUnconfirmed(t *testing.T) {
	h := secq1cHarness(t)
	old := classifyDestination
	t.Cleanup(func() { classifyDestination = old })
	classifyDestination = func(context.Context, string, string) (netguard.Class, error) { return netguard.ClassPublic, nil }
	res, err := h.db.Exec(`INSERT INTO storage_roots (name, protocol, url, path, enabled, max_depth) VALUES ('lpub', 'webdav', 'https://x.example/dav', '/', 1, 10)`)
	require.NoError(t, err)
	id, _ := res.LastInsertId()
	u := "https://x.example/dav"
	root := &models.StorageRoot{Protocol: "webdav", URL: &u}
	_, aerr := h.handle.approveNewRoot(context.Background(), id, root, netguard.ClassPrivate, false)
	require.ErrorIs(t, aerr, errPrivateUnconfirmed)
	require.Equal(t, 0, rootCount(t, h.db), "the row is removed")
}

// Finding 5 (sync route).
func (s *SyncHandlerTestSuite) TestSECQ1C_StepUpIsThrottled() {
	freshThrottle(s.T())
	s.T().Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	s.giveUserPassword()
	srv := openDavSecq1c(s.T())
	cb := s.davBody(srv.URL)
	cb["confirm_private_network"] = true
	require.Equal(s.T(), http.StatusCreated, s.doRequest("POST", "/sync/endpoints", cb, true).Code)
	up := map[string]interface{}{"url": "http://127.0.0.1:1/x", "step_up_password": "wrong"}
	for i := 0; i < stepUpMaxFailures; i++ {
		w := s.doRequest("PUT", "/sync/endpoints/1", up, true)
		require.Equal(s.T(), http.StatusForbidden, w.Code, "failure %d: %s", i, w.Body.String())
		require.Contains(s.T(), w.Body.String(), "step_up_failed")
	}
	up["step_up_password"] = syncStepPW
	w := s.doRequest("PUT", "/sync/endpoints/1", up, true)
	require.Equal(s.T(), http.StatusTooManyRequests, w.Code, "%s", w.Body.String())
	require.Contains(s.T(), w.Body.String(), "step_up_throttled")
	require.Equal(s.T(), strconv.Itoa(int(stepUpWindow/time.Second)), w.Header().Get("Retry-After"))
}

// F1 (sync route): a concurrent burst of wrong step-ups runs at most the limit of password checks.
func (s *SyncHandlerTestSuite) TestSECQ1C_StepUpConcurrentBurst() {
	freshThrottle(s.T())
	s.T().Setenv(netguard.AllowCIDRsEnv, "127.0.0.0/8")
	s.giveUserPassword()
	srv := openDavSecq1c(s.T())
	body := s.davBody(srv.URL)
	body["confirm_private_network"] = true
	require.Equal(s.T(), http.StatusCreated, s.doRequest("POST", "/sync/endpoints", body, true).Code)
	const n = 40
	var wg sync.WaitGroup
	var mu sync.Mutex
	counts := map[int]int{}
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			w := s.doRequest("PUT", "/sync/endpoints/1", map[string]interface{}{"name": "x", "step_up_password": fmt.Sprintf("wrong-%d", i)}, true)
			mu.Lock()
			counts[w.Code]++
			mu.Unlock()
		}(i)
	}
	close(start)
	wg.Wait()
	require.Equal(s.T(), stepUpMaxFailures, counts[http.StatusForbidden], "exactly the limit of password checks ran: %v", counts)
	require.Equal(s.T(), n-stepUpMaxFailures, counts[http.StatusTooManyRequests], "%v", counts)
	require.Contains(s.T(), stepUpThrottle.m, s.testUserID, "keyed by the acting user")
}

// F1: the endpoint must exist (and be the user's) BEFORE a password is checked: no password oracle, no throttle slot used.
func (s *SyncHandlerTestSuite) TestSECQ1C_StepUpOnUnknownEndpointCostsNoCheck() {
	freshThrottle(s.T())
	s.giveUserPassword()
	for i := 0; i < stepUpMaxFailures+3; i++ {
		w := s.doRequest("PUT", "/sync/endpoints/4242", map[string]interface{}{"name": "x", "step_up_password": "wrong"}, true)
		require.Equal(s.T(), http.StatusNotFound, w.Code, "%s", w.Body.String())
	}
	require.Equal(s.T(), 0, stepUpThrottle.size(), "no attempt was reserved for an endpoint that does not exist")
}

var _ = database.TableStorageRoots
