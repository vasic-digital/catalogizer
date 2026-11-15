package handlers

// CAT-SECQ1-C follow-ups (SECQ1C-M1, SECQ1C-M4): the ownership half of the update authorisation, and the limiter purge never dropping an
// attempt that is still running. Real SQLite, real AuthService/SyncService/SyncRepository, the real handler; the limiter tests drive the
// real limiter (unit class).

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"catalogizer/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// secq1cFUUser adds a user with its own role (permissions JSON) and an active session, and returns a token for it.
func (s *SyncHandlerTestSuite) secq1cFUUser(id int, perms string) string {
	_, err := s.db.Exec(`INSERT INTO roles (id, name, description, permissions, is_system) VALUES (?, ?, 'fu', ?, 0)`, id+100, fmt.Sprintf("fu-role-%d", id), perms)
	require.NoError(s.T(), err)
	_, err = s.db.Exec(`INSERT INTO users (id, username, email, password_hash, salt, role_id) VALUES (?, ?, ?, ?, ?, ?)`,
		id, fmt.Sprintf("fu%d", id), fmt.Sprintf("fu%d@example.test", id), "$2a$10$fakehashfortest", "fakesalt", id+100)
	require.NoError(s.T(), err)
	_, err = s.db.Exec(`INSERT INTO user_sessions (id, user_id, session_token, refresh_token, device_info, is_active, expires_at) VALUES (?, ?, ?, ?, '{}', 1, ?)`,
		id+100, id, fmt.Sprintf("t%d", id), fmt.Sprintf("r%d", id), time.Now().Add(time.Hour))
	require.NoError(s.T(), err)
	return s.generateTestToken(id, fmt.Sprintf("fu%d", id), id+100, fmt.Sprint(id+100))
}

func (s *SyncHandlerTestSuite) putAs(token, path string, body map[string]interface{}) *httptest.ResponseRecorder {
	old := s.authToken
	s.authToken = token
	defer func() { s.authToken = old }()
	return s.doRequest("PUT", path, body, true)
}

// SECQ1C-M1: a user who neither owns the endpoint nor holds share.edit is refused (403 "unauthorized") BEFORE any password is checked:
// no throttle slot is reserved and the answer is not step_up_failed. A user holding share.edit (control) is admitted to the password check.
// SECQ1CFU-N1: the suite router has no adminGate (production main.go mounts one); this test covers handler-level ownership only.
func (s *SyncHandlerTestSuite) TestSECQ1CFU_M1_ForeignEndpointCostsNoPasswordCheck() {
	freshThrottle(s.T())
	foreign := s.secq1cFUUser(52, `[]`)
	editor := s.secq1cFUUser(53, fmt.Sprintf(`["%s"]`, models.PermissionEditShares))
	id := s.createTestEndpointInDB(s.testUserID, "owned-by-1", "local", "active")
	path := fmt.Sprintf("/sync/endpoints/%d", id)
	body := map[string]interface{}{"name": "x", "step_up_password": "wrong"}

	for i := 0; i < stepUpMaxFailures+3; i++ {
		w := s.putAs(foreign, path, body)
		require.Equal(s.T(), http.StatusForbidden, w.Code, "%s", w.Body.String())
		require.Contains(s.T(), w.Body.String(), "not permitted")
		require.NotContains(s.T(), w.Body.String(), "step_up_failed", "no password check ran")
		require.NotContains(s.T(), w.Body.String(), "step_up_throttled", "no slot was used")
	}
	require.Equal(s.T(), 0, stepUpThrottle.size(), "no attempt was reserved for the foreign user")

	// control: share.edit holder passes the authorisation and reaches the password check (wrong password -> step_up_failed, a slot is used)
	w := s.putAs(editor, path, body)
	require.Equal(s.T(), http.StatusForbidden, w.Code, "%s", w.Body.String())
	require.Contains(s.T(), w.Body.String(), "step_up_failed")
	require.Equal(s.T(), 1, stepUpThrottle.size())
}

// SECQ1C-M4: purge never drops an entry whose attempt is still running, whether it expired or the table is full.
func TestSECQ1CFU_M4_PurgeKeepsInFlightEntries(t *testing.T) {
	now := freshThrottle(t)
	l := stepUpThrottle
	_, ok := l.acquire(1) // running, never released
	require.True(t, ok)
	*now = now.Add(stepUpWindow + time.Second) // its window is over
	_, ok = l.acquire(2)                       // a new user triggers the purge
	require.True(t, ok)
	require.Contains(t, l.m, 1, "an expired entry with a running attempt survives the purge")
	l.release(1, false)
	require.Equal(t, 1, l.m[1].inflight+l.m[1].count, "its release still finds the entry")
	l.release(2, true)
}

func TestSECQ1CFU_M4_FullTableNeverEvictsInFlightEntries(t *testing.T) {
	now := freshThrottle(t)
	l := stepUpThrottle
	for u := 1; u <= stepUpMaxUsers; u++ {
		_, ok := l.acquire(u)
		require.True(t, ok)
		*now = now.Add(time.Millisecond)
	}
	_, ok := l.acquire(stepUpMaxUsers + 1) // table full, every entry running: nothing may be evicted
	require.True(t, ok)
	for u := 1; u <= stepUpMaxUsers; u++ {
		require.Contains(t, l.m, u, "running attempt of user %d was evicted", u)
	}
}

// reviewer probe TestRV3_PanicAndWindow, re-built: a panic in the check counts as a failure and releases its slot; a window end resets.
func TestSECQ1CFU_M4_PanicCountsAsFailureAndReleasesSlot(t *testing.T) {
	now := freshThrottle(t)
	l := stepUpThrottle
	for i := 0; i < stepUpMaxFailures; i++ {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		func() {
			defer func() { require.NotNil(t, recover(), "the panic propagates") }()
			stepUpAttempt(c, 7, true, func() bool { panic("boom") })
		}()
	}
	require.Equal(t, 0, l.m[7].inflight, "every panicking attempt released its slot")
	require.Equal(t, stepUpMaxFailures, l.m[7].count, "every panic counted as a failure")
	_, ok := l.acquire(7)
	require.False(t, ok)
	*now = now.Add(stepUpWindow) // exactly the window edge: a new window starts
	_, ok = l.acquire(7)
	require.True(t, ok, "at the window edge the count restarts")
}
