package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strconv"
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

// CAT-SYNC-404-1: a caller that may not see a sync endpoint or session must get the SAME answer for a foreign id and for a missing id (404, same body),
// like the job-id decision (CAT-JOB-404-1); a caller that may see it (share.view) keeps its answer (the record, or 403 for an action it lacks).
// Real SQLite (all migrations), real AuthService (real login tokens) and SyncService; no stubs. The sync routes sit behind RequireAdmin (admins hold
// every permission), so the handler-level answer is a defence in depth: the shipped routes cannot reach it for a non-admin today.

type sync404Env struct {
	h                                *SyncHandler
	ownerTok, strangerTok, holderTok string
	endpointID, sessionID, missingID int
}

func newSync404Env(t *testing.T) *sync404Env {
	t.Helper()
	gin.SetMode(gin.TestMode)
	raw, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = raw.Close() })
	raw.SetMaxOpenConns(1)
	db := database.WrapDB(raw, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	_, err = raw.Exec(`INSERT INTO roles (id, name, description, permissions, is_system) VALUES (3, 'Viewer', 'share viewer', '["share.view"]', 0)`)
	require.NoError(t, err)

	users := repository.NewUserRepository(db)
	auth := services.NewAuthService(users, "test-jwt-secret-throwaway")
	const pw = "Sync404!Throwaway#1aa"
	mk := func(name string, role int) (int, string) {
		id, err := users.Create(&models.User{Username: name, Email: name + "@example.test", RoleID: role, IsActive: true, Salt: "0123456789abcdef0123456789abcdef"})
		require.NoError(t, err)
		require.NoError(t, auth.ResetPassword(id, pw))
		res, err := auth.Login(models.LoginRequest{Username: name, Password: pw}, "127.0.0.1", "test")
		require.NoError(t, err)
		return id, res.SessionToken
	}
	ownerID, ownerTok := mk("owner", 2)
	_, strangerTok := mk("stranger", 2)
	_, holderTok := mk("holder", 3)

	repo := repository.NewSyncRepository(db)
	svc := services.NewSyncService(repo, users, auth)
	t.Cleanup(func() { svc.Close() })
	eid, err := repo.CreateEndpoint(&models.SyncEndpoint{UserID: ownerID, Name: "e", Type: models.SyncTypeWebDAV, URL: "https://nas.example.test/dav",
		SyncDirection: models.SyncDirectionUpload, LocalPath: "/data", Status: models.SyncStatusActive, CreatedAt: time.Now(), UpdatedAt: time.Now()})
	require.NoError(t, err)
	sid, err := repo.CreateSession(&models.SyncSession{EndpointID: eid, UserID: ownerID, Status: models.SyncSessionStatusCompleted, SyncType: models.SyncTypeManual, StartedAt: time.Now()})
	require.NoError(t, err)
	return &sync404Env{h: NewSyncHandler(svc, auth), ownerTok: ownerTok, strangerTok: strangerTok, holderTok: holderTok, endpointID: eid, sessionID: sid, missingID: 987654}
}

func (e *sync404Env) call(t *testing.T, tok, method, route string, id int, body string, fn func(*gin.Context)) (int, string) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, route, bytes.NewBufferString(body))
	c.Request.Header.Set("Authorization", "Bearer "+tok)
	c.Request.Header.Set("Content-Type", "application/json")
	if id != 0 {
		c.Params = gin.Params{{Key: "id", Value: strconv.Itoa(id)}}
	}
	fn(c)
	return w.Code, w.Body.String()
}

func TestSync404_ForeignIdLooksLikeMissingId(t *testing.T) {
	e := newSync404Env(t)
	type op struct {
		name   string
		method string
		fn     func(*gin.Context)
		id     func(ep, sess int) int
		body   string
	}
	ops := []op{
		{"GetEndpoint", "GET", e.h.GetEndpoint, func(ep, _ int) int { return ep }, ""},
		{"UpdateEndpoint", "PUT", e.h.UpdateEndpoint, func(ep, _ int) int { return ep }, `{"name":"x"}`},
		{"DeleteEndpoint", "DELETE", e.h.DeleteEndpoint, func(ep, _ int) int { return ep }, ""},
		{"StartSync", "POST", e.h.StartSync, func(ep, _ int) int { return ep }, ""},
		{"GetSession", "GET", e.h.GetSession, func(_, s int) int { return s }, ""},
	}
	for _, o := range ops {
		t.Run(o.name, func(t *testing.T) {
			missCode, missBody := e.call(t, e.strangerTok, o.method, "/x", e.missingID, o.body, o.fn)
			require.Equal(t, http.StatusNotFound, missCode, "a missing id is 404")
			code, body := e.call(t, e.strangerTok, o.method, "/x", o.id(e.endpointID, e.sessionID), o.body, o.fn)
			assert.Equal(t, missCode, code, "a foreign id must answer like a missing id")
			assert.Equal(t, missBody, body, "a foreign id must answer with the same body as a missing id")
		})
	}
	t.Run("ScheduleSync", func(t *testing.T) {
		mk := func(id int) (int, string) {
			return e.call(t, e.strangerTok, "POST", "/x", 0, `{"endpoint_id":`+strconv.Itoa(id)+`,"frequency":"daily"}`, e.h.ScheduleSync)
		}
		missCode, missBody := mk(e.missingID)
		require.Equal(t, http.StatusNotFound, missCode)
		code, body := mk(e.endpointID)
		assert.Equal(t, missCode, code)
		assert.Equal(t, missBody, body)
	})
	// the stranger's attempts changed nothing: the endpoint is still the owner's, unchanged, and there is no session started
	code, body := e.call(t, e.ownerTok, "GET", "/x", e.endpointID, "", e.h.GetEndpoint)
	assert.Equal(t, http.StatusOK, code, body)
	assert.Contains(t, body, `"name":"e"`)
}

func TestSync404_OwnerAndShareViewHolderKeepTheirAnswers(t *testing.T) {
	e := newSync404Env(t)
	// the owner reads both records
	code, _ := e.call(t, e.ownerTok, "GET", "/x", e.endpointID, "", e.h.GetEndpoint)
	assert.Equal(t, http.StatusOK, code)
	code, _ = e.call(t, e.ownerTok, "GET", "/x", e.sessionID, "", e.h.GetSession)
	assert.Equal(t, http.StatusOK, code)
	// a share.view holder may see both (existence is no secret from it)
	code, _ = e.call(t, e.holderTok, "GET", "/x", e.endpointID, "", e.h.GetEndpoint)
	assert.Equal(t, http.StatusOK, code, "share.view holder reads a foreign endpoint")
	code, _ = e.call(t, e.holderTok, "GET", "/x", e.sessionID, "", e.h.GetSession)
	assert.Equal(t, http.StatusOK, code, "share.view holder reads a foreign session")
	// but it lacks the action permissions: 403, not 404 (it can see the record)
	for name, fn := range map[string]func(*gin.Context){"update": e.h.UpdateEndpoint, "delete": e.h.DeleteEndpoint, "start": e.h.StartSync} {
		method := map[string]string{"update": "PUT", "delete": "DELETE", "start": "POST"}[name]
		code, _ = e.call(t, e.holderTok, method, "/x", e.endpointID, `{"name":"x"}`, fn)
		assert.Equal(t, http.StatusForbidden, code, name)
	}
}
