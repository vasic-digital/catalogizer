package handlers

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"catalogizer/database"
	"catalogizer/models"
	"catalogizer/repository"
	"catalogizer/services"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// CAT-LOGSHARE-TOK-1, behavioural: the real gin router, the real handler, the real service and repository on a fully migrated
// SQLite database (unit-class, §11.4.289: no live server; the auth service is the only stand-in).
func newShareRoutedFixture(t *testing.T, perms []string) (*gin.Engine, *database.DB, string, int) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	raw, err := sql.Open("sqlite3", ":memory:")
	require.NoError(t, err)
	raw.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = raw.Close() })
	db := database.WrapDB(raw, database.DialectSQLite)
	require.NoError(t, db.RunMigrations(context.Background()))
	svc := services.NewLogManagementService(repository.NewLogManagementRepository(db))
	auth := new(MockLogManagementAuthService)
	auth.On("CheckPermission", mock.Anything, models.PermissionSystemAdmin).Return(true, nil)
	h := NewLogManagementHandler(&LogManagementServiceAdapter{Inner: svc}, auth)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Request = c.Request.WithContext(context.WithValue(c.Request.Context(), "user_id", 1))
	})
	g := r.Group("/api/v1/logs")
	g.POST("/share", WrapHTTPHandler(h.CreateLogShare))
	g.GET("/share/:token", WrapHTTPHandler(h.GetLogShare))

	col, err := svc.CollectLogs(1, &models.LogCollectionRequest{Name: "c", Components: []string{"api"}, LogLevel: "info"})
	require.NoError(t, err)
	body, _ := json.Marshal(models.LogShareRequest{CollectionID: col.ID, ShareType: "link", Permissions: perms})
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest("POST", "/api/v1/logs/share", bytes.NewReader(body)))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var created models.LogShare
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
	return r, db, created.ShareToken, created.ID
}

func getShare(r *gin.Engine, token string) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest("GET", "/api/v1/logs/share/"+token, nil))
	return rr
}

func TestCATLOGSHARETOK1_RoutedVerifyAndNoOracle(t *testing.T) {
	r, db, token, id := newShareRoutedFixture(t, []string{"read"})
	assert.True(t, strings.HasPrefix(token, "log_share_") && len(token) == 53, "token %q", token)

	ok := getShare(r, token)
	require.Equal(t, http.StatusOK, ok.Code, ok.Body.String())
	assert.NotContains(t, ok.Body.String(), `"share_token":"`+token, "the token is not echoed on verify")
	var out struct {
		Share struct {
			ShareToken string `json:"share_token"`
		} `json:"share"`
	}
	require.NoError(t, json.Unmarshal(ok.Body.Bytes(), &out))
	assert.Empty(t, out.Share.ShareToken, "neither plaintext nor the stored hash is returned")

	// legacy token still in a pre-v28 row, expired token, revoked token, unknown, malformed: one answer
	_, err := db.Exec(`INSERT INTO log_shares (collection_id, user_id, share_token, is_active, expires_at, permissions, recipients) SELECT collection_id, 1, 'log_share_1700000000_42', 1, ?, '["read"]', '[]' FROM log_shares WHERE id = ?`, time.Now().Add(time.Hour), id)
	require.NoError(t, err)
	want := getShare(r, "log_share_1700000000_42")
	require.Equal(t, http.StatusNotFound, want.Code)
	for name, tok := range map[string]string{
		"unknown": "log_share_" + strings.Repeat("A", 43), "malformed": "nope", "stored hash": hashOf(t, db, id),
	} {
		got := getShare(r, tok)
		assert.Equal(t, want.Code, got.Code, name)
		assert.Equal(t, want.Body.String(), got.Body.String(), name)
	}
	_, err = db.Exec(`UPDATE log_shares SET expires_at = ? WHERE id = ?`, time.Now().Add(-time.Minute), id)
	require.NoError(t, err)
	exp := getShare(r, token)
	assert.Equal(t, want.Code, exp.Code)
	assert.Equal(t, want.Body.String(), exp.Body.String(), "an expired token answers like an unknown one")
}

func hashOf(t *testing.T, db *database.DB, id int) string {
	t.Helper()
	var h string
	require.NoError(t, db.QueryRow(`SELECT share_token FROM log_shares WHERE id = ?`, id).Scan(&h))
	return h
}

// a valid token whose share lacks the read permission answers like an unknown one (it used to be a distinguishable 403)
func TestCATLOGSHARETOK1_NoReadPermissionIsIndistinguishable(t *testing.T) {
	r, _, token, _ := newShareRoutedFixture(t, []string{"write"})
	noRead := getShare(r, token)
	unknown := getShare(r, "log_share_"+strings.Repeat("B", 43))
	assert.Equal(t, http.StatusNotFound, noRead.Code)
	assert.Equal(t, unknown.Code, noRead.Code)
	assert.Equal(t, unknown.Body.String(), noRead.Body.String())
}
