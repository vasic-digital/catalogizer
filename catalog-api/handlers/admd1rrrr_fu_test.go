package handlers

// ADMD1RRRR follow-ups (m2, m3, m4, n1, n2): non-unique negative controls that run on SQLite WITHOUT the PostgreSQL DSN, a
// live-PostgreSQL leg of the storage-root race, and the 409 body of a racing storage-root create.
//
// The non-unique failure is a real database-level abort raised by a trigger created on the REAL migrated database
// ("RAISE(ABORT, ...)", a constraint-class failure whose text is not a unique violation). The handler, repository and SQL
// under test are the production ones; only the failing condition is armed in the database. The live PostgreSQL leg needs
// CAT_TEST_PG_DSN (honest SKIP otherwise).

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"catalogizer/database"
	"catalogizer/filesystem"
)

const forcedFailText = "forced non-unique failure"

// armFailure makes the database refuse (RAISE ABORT, a non-unique reason) the given write when cond holds.
func armFailure(t *testing.T, db *database.DB, name, event, table, cond string) {
	t.Helper()
	_, err := db.Exec(fmt.Sprintf(`CREATE TRIGGER %s BEFORE %s ON %s WHEN %s BEGIN SELECT RAISE(ABORT, '%s'); END`,
		name, event, table, cond, forcedFailText))
	require.NoError(t, err)
}

func assertNot409Is500(t *testing.T, label string, code int, body string) {
	t.Helper()
	assert.NotEqual(t, http.StatusConflict, code, "%s: non-unique failure classified as a duplicate: %s", label, body)
	assert.NotContains(t, body, "already exists", label)
	assert.Equal(t, http.StatusInternalServerError, code, "%s: a non-unique write failure is a plain 500: %s", label, body)
}

// m3 + m2 (register): the loser path of RegisterGin with a failing non-unique insert stays 500.
func TestADMD1RRRRfu_RegisterNonUniqueInsertFailureIs500_SQLite(t *testing.T) {
	e := newRTEnv(t)
	t.Setenv("REGISTRATION_MODE", "open")
	armFailure(t, e.db, "fail_reg", "INSERT", "users", "NEW.username = 'forcefail'")
	ah := NewAuthHandler(e.authSvc)
	r := gin.New()
	r.POST("/api/v1/auth/register", func(c *gin.Context) { ah.RegisterGin(c, e.repo) })
	post := func(user string) (int, string) {
		body := fmt.Sprintf(`{"username":%q,"email":%q,"password":"Pw!Throwaway#9x","first_name":"R","last_name":"R"}`, user, user+"@example.test")
		req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}
	code, body := post("forcefail")
	assertNot409Is500(t, "register", code, body)
	// control needles: the trigger is not armed for other names (201), and a genuine duplicate is still 409
	code, body = post("okname")
	assert.Equal(t, http.StatusCreated, code, body)
	code, body = post("okname")
	assert.Equal(t, http.StatusConflict, code, body)
}

// m3 + m2 (storage root create).
func TestADMD1RRRRfu_StorageRootNonUniqueInsertFailureIs500_SQLite(t *testing.T) {
	base := t.TempDir()
	a := newAccessHarness(t, base)
	admin := cat001Token(t, 1, "1")
	armFailure(t, a.h.db, "fail_root", "INSERT", "storage_roots", "NEW.name = 'forcefail'")
	code, body := a.do(admin, "POST", "/storage/roots", map[string]interface{}{
		"name": "forcefail", "protocol": "local", "path": base, "confirm_private_network": true})
	raw, _ := json.Marshal(body)
	assertNot409Is500(t, "storage root", code, string(raw))
	// control needles: another name is created, and its duplicate is a 409 carrying existing_id
	code, body = a.do(admin, "POST", "/storage/roots", map[string]interface{}{
		"name": "okroot", "protocol": "local", "path": base, "confirm_private_network": true})
	require.Equal(t, http.StatusCreated, code, "%v", body)
	code, body = a.do(admin, "POST", "/storage/roots", map[string]interface{}{
		"name": "okroot", "protocol": "local", "path": base, "confirm_private_network": true})
	assert.Equal(t, http.StatusConflict, code, "%v", body)
}

// m3 (admin update, guarded + plain branch): a failing non-unique UPDATE stays 500 without the PG DSN.
func TestADMD1RRRRfu_AdminUpdateNonUniqueFailureIs500_SQLite(t *testing.T) {
	e := newRTEnv(t)
	w := e.do(e.adminTok, "POST", "/api/v1/users", `{"username":"admfu","email":"admfu@example.test","password":"Pw!Throwaway#9x","role_id":2}`)
	require.Equal(t, 201, w.Code, w.Body.String())
	var u struct {
		ID int `json:"id"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &u))
	url := "/api/v1/admin/users/" + rtItoa(u.ID)
	armFailure(t, e.db, "fail_upd", "UPDATE", "users", "NEW.email = 'forcefail@example.test'")
	body := `{"email":"forcefail@example.test"}`
	plain := gin.New()
	plain.PUT("/api/v1/admin/users/:id", NewAdminHandler(&AuthServiceAdapter{Inner: e.authSvc},
		struct{ AdminUserRepoInterface }{e.repo}, e.db, "test").UpdateUser)
	req := httptest.NewRequest(http.MethodPut, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+e.adminTok)
	rec := httptest.NewRecorder()
	plain.ServeHTTP(rec, req)
	g := e.do(e.adminTok, "PUT", url, body)
	assertNot409Is500(t, "guarded", g.Code, g.Body.String())
	assertNot409Is500(t, "plain", rec.Code, rec.Body.String())
	// control needle: another email still updates (200), a duplicate email is still 409
	w = e.do(e.adminTok, "PUT", url, `{"email":"admfu2@example.test"}`)
	assert.Equal(t, 200, w.Code, w.Body.String())
	w = e.do(e.adminTok, "PUT", url, `{"email":"rtadmin@example.test"}`)
	assert.Equal(t, 409, w.Code, w.Body.String())
}

// raceBodies runs n goroutines released together and returns code -> bodies.
func raceBodies(n int, do func(i int) (int, map[string]interface{})) map[int][]map[string]interface{} {
	out := map[int][]map[string]interface{}{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			c, b := do(i)
			mu.Lock()
			out[c] = append(out[c], b)
			mu.Unlock()
		}(i)
	}
	close(start)
	wg.Wait()
	return out
}

// checkRootConflictBody: every 409 names the existing root (existing_id equals the winner's id) and its hint does not point at
// an endpoint that does not exist (n1, n2).
func checkRootConflictBody(t *testing.T, label string, winnerID int64, losers []map[string]interface{}) {
	t.Helper()
	for _, b := range losers {
		assert.Equal(t, float64(winnerID), b["existing_id"], "%s: 409 body must name the existing root: %v", label, b)
		hint, _ := b["hint"].(string)
		assert.NotContains(t, hint, "PUT /api/v1/storage/roots/{id}", "%s: hint points at a nonexistent endpoint: %v", label, b)
		assert.NotEmpty(t, hint, label)
	}
}

func rootRace(t *testing.T, h *ScanHandler, base string, rounds, n int) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("role_id", 1) })
	r.POST("/storage/roots", h.CreateStorageRoot)
	for round := 0; round < rounds; round++ {
		name := fmt.Sprintf("fuhrace%d", round)
		res := raceBodies(n, func(int) (int, map[string]interface{}) {
			buf, _ := json.Marshal(map[string]interface{}{"name": name, "protocol": "local", "path": base, "confirm_private_network": true})
			req := httptest.NewRequest(http.MethodPost, "/storage/roots", bytes.NewReader(buf))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			var b map[string]interface{}
			_ = json.Unmarshal(rec.Body.Bytes(), &b)
			return rec.Code, b
		})
		require.Len(t, res[http.StatusCreated], 1, "round %d: exactly one create wins: %v", round, res)
		assert.Len(t, res[http.StatusConflict], n-1, "round %d: every loser sees 409: %v", round, res)
		assert.Empty(t, res[http.StatusInternalServerError], "round %d", round)
		winner := int64(res[http.StatusCreated][0]["id"].(float64))
		checkRootConflictBody(t, "round "+strconv.Itoa(round), winner, res[http.StatusConflict])
	}
}

func TestADMD1RRRRfu_RacingStorageRootConflictBody_SQLite(t *testing.T) {
	base := t.TempDir()
	a := newAccessHarness(t, base)
	rootRace(t, a.h.handle, base, 6, 16)
}

// m4: the same race on live PostgreSQL (InsertReturningID + SQLSTATE 23505).
func TestADMD1RRRRfu_RacingStorageRootConflictBody_PostgresLive(t *testing.T) {
	e := newRTEnvPG(t)
	base := t.TempDir()
	t.Setenv(filesystem.AllowedLocalBasesEnv, base)
	rootRace(t, NewScanHandler(newMockUniversalScanner(), e.db), base, 6, 16)
}

// n1/n2 sequential path: same body shape as the racing one.
func TestADMD1RRRRfu_SequentialStorageRootConflictBody_SQLite(t *testing.T) {
	base := t.TempDir()
	a := newAccessHarness(t, base)
	admin := cat001Token(t, 1, "1")
	code, first := a.do(admin, "POST", "/storage/roots", map[string]interface{}{
		"name": "seqroot", "protocol": "local", "path": base, "confirm_private_network": true})
	require.Equal(t, http.StatusCreated, code, "%v", first)
	code, dup := a.do(admin, "POST", "/storage/roots", map[string]interface{}{
		"name": "seqroot", "protocol": "local", "path": base, "confirm_private_network": true})
	require.Equal(t, http.StatusConflict, code, "%v", dup)
	checkRootConflictBody(t, "sequential", int64(first["id"].(float64)), []map[string]interface{}{dup})
}

// ADMD1RRRRR-m1: deterministic insert-path 409. The test seam inserts the winner row AFTER the duplicate-name pre-check
// and BEFORE the handler's insert, so the request provably misses the pre-check and the 409 can only come from the
// insert-time unique-violation branch. The hook receives the pre-check outcome (sql.ErrNoRows) and the test asserts it,
// which proves the pre-check ran before the hook (ADMD1RRRRRR-m1).
func TestADMD1RRRRRfu_InsertPathConflictBodyDeterministic_SQLite(t *testing.T) {
	base := t.TempDir()
	a := newAccessHarness(t, base)
	admin := cat001Token(t, 1, "1")
	var winnerID int64
	calls := 0
	var preErr error
	storageRootBeforeInsertHook = func(ctx context.Context, name string, preCheckErr error) {
		calls++
		preErr = preCheckErr
		id, err := a.h.db.InsertReturningID(ctx,
			`INSERT INTO storage_roots (name, protocol, path, enabled, max_depth) VALUES (?, 'local', ?, ?, 10)`, name, base, true)
		require.NoError(t, err)
		winnerID = id
	}
	t.Cleanup(func() { storageRootBeforeInsertHook = nil })
	code, body := a.do(admin, "POST", "/storage/roots", map[string]interface{}{
		"name": "seamroot", "protocol": "local", "path": base, "confirm_private_network": true})
	require.Equal(t, http.StatusConflict, code, "%v", body)
	require.Equal(t, 1, calls, "the request must have passed the pre-check and reached the insert")
	require.ErrorIs(t, preErr, sql.ErrNoRows, "the hook must run after the pre-check, which found no row")
	require.NotZero(t, winnerID)
	checkRootConflictBody(t, "insert path", winnerID, []map[string]interface{}{body})
	assert.Equal(t, "seamroot", body["name"])
	assert.Equal(t, storageRootConflictHint, body["hint"])
	// control needle: a normal create (no competing winner) with the hook disabled still yields 201
	storageRootBeforeInsertHook = nil
	code, body = a.do(admin, "POST", "/storage/roots", map[string]interface{}{
		"name": "freeroot", "protocol": "local", "path": base, "confirm_private_network": true})
	assert.Equal(t, http.StatusCreated, code, "%v", body)
}

// ADMD1RRRRR-n2: without existing_id (winner lookup failed) the body omits it and the hint no longer refers to it.
func TestADMD1RRRRRfu_ConflictBodyWithoutExistingIDHint(t *testing.T) {
	b := storageRootConflictBody("x", nil)
	_, has := b["existing_id"]
	assert.False(t, has)
	assert.NotContains(t, b["hint"], "existing_id", "hint must not refer to an absent field: %v", b)
	assert.NotEmpty(t, b["hint"])
	id := int64(7)
	w := storageRootConflictBody("x", &id)
	assert.Equal(t, int64(7), w["existing_id"])
	assert.Contains(t, w["hint"], "existing_id")
}

// ADMD1RRRRRR-m2: the failed-winner-lookup branch through the real handler. The hook makes the request miss the
// pre-check (winner inserted), then the after-conflict hook deletes the winner so the handler's real lookup finds no
// row: the 409 must omit existing_id and use the no-id hint.
func TestADMD1RRRRRRfu_FailedWinnerLookupOmitsExistingID_SQLite(t *testing.T) {
	base := t.TempDir()
	a := newAccessHarness(t, base)
	admin := cat001Token(t, 1, "1")
	storageRootBeforeInsertHook = func(ctx context.Context, name string, _ error) {
		_, err := a.h.db.InsertReturningID(ctx,
			`INSERT INTO storage_roots (name, protocol, path, enabled, max_depth) VALUES (?, 'local', ?, ?, 10)`, name, base, true)
		require.NoError(t, err)
	}
	afterCalls := 0
	storageRootAfterConflictHook = func(ctx context.Context, name string) {
		afterCalls++
		_, err := a.h.db.ExecContext(ctx, "DELETE FROM storage_roots WHERE name = ?", name)
		require.NoError(t, err)
	}
	t.Cleanup(func() { storageRootBeforeInsertHook = nil; storageRootAfterConflictHook = nil })
	code, body := a.do(admin, "POST", "/storage/roots", map[string]interface{}{
		"name": "lostwinner", "protocol": "local", "path": base, "confirm_private_network": true})
	require.Equal(t, http.StatusConflict, code, "%v", body)
	require.Equal(t, 1, afterCalls, "the conflict must have come from the insert-time unique violation")
	_, has := body["existing_id"]
	assert.False(t, has, "existing_id must be omitted when the winner lookup failed: %v", body)
	assert.Equal(t, storageRootConflictHintNoID, body["hint"])
	assert.Equal(t, "lostwinner", body["name"])
	// control needle: with the lookup succeeding (no deletion) existing_id is present and the normal hint is used
	storageRootAfterConflictHook = nil
	code, body = a.do(admin, "POST", "/storage/roots", map[string]interface{}{
		"name": "winnerkept", "protocol": "local", "path": base, "confirm_private_network": true})
	require.Equal(t, http.StatusConflict, code, "%v", body)
	assert.NotNil(t, body["existing_id"], "%v", body)
	assert.Equal(t, storageRootConflictHint, body["hint"])
}
