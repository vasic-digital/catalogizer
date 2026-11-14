package handlers

// CAT-SECQ1-C review round 1: route authz, unresolvable names, rebinding between the two classifications, and the per-user step-up throttle.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/internal/netguard"

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
	require.Equal(t, http.StatusConflict, code, "%v", body)
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
	require.Equal(t, http.StatusConflict, code, "%v", body)
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
	require.NotEmpty(t, w.Header().Get("Retry-After"))
	cls, _ := rootClass(t, h.db, "pub")
	require.Equal(t, "public", cls, "a throttled request changes nothing")

	*now = now.Add(stepUpWindow + time.Second) // the window ends
	code, body = h.do("PUT", path, map[string]interface{}{"class": "private", "step_up_password": secq1cStepPW})
	require.Equal(t, http.StatusOK, code, "%v", body)
	require.Equal(t, 0, stepUpThrottle.size(), "a success clears the user")
}

func TestSECQ1C_R1_ThrottleIsPerUserAndBounded(t *testing.T) {
	now := freshThrottle(t)
	l := stepUpThrottle
	for i := 0; i < stepUpMaxFailures; i++ {
		l.fail(1)
	}
	_, blocked := l.blocked(1)
	require.True(t, blocked)
	_, blocked = l.blocked(2)
	require.False(t, blocked, "another user is not locked out")

	for u := 100; u < 100+stepUpMaxUsers+500; u++ { // more users than the table may hold
		l.fail(u)
		*now = now.Add(time.Millisecond)
	}
	require.LessOrEqual(t, l.size(), stepUpMaxUsers, "the limiter state is bounded")

	*now = now.Add(stepUpWindow + time.Second)
	l.fail(5) // purge on the next write
	require.LessOrEqual(t, l.size(), 2, "expired entries are dropped")
}

// Finding 5 (sync route).
func (s *SyncHandlerTestSuite) TestSECQ1C_StepUpIsThrottled() {
	freshThrottle(s.T())
	s.giveUserPassword()
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
	require.NotEmpty(s.T(), w.Header().Get("Retry-After"))
}

var _ = database.TableStorageRoots
