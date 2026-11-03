package handlers

// canEdit for a legacy ownerless collection (owner_user_id NULL): readable by everyone, editable and
// deletable by an administrator only. Real migrated SQLite, row seeded directly with a NULL owner.
// Kills reviewer mutant A (`coll.OwnerUserID == nil ||` added to canEdit).

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCollectionHandler_OwnerlessIsAdminEditOnly(t *testing.T) {
	gin.SetMode(gin.TestMode)
	dir := t.TempDir()
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: filepath.Join(dir, "c.db"),
		EncryptionKeyFile: filepath.Join(dir, "k", "db.key"), MaxOpenConnections: 2})
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	require.NoError(t, db.RunMigrations(context.Background()))

	res, err := db.Exec(`INSERT INTO media_collections (name, collection_type, total_items, external_ids, created_at, updated_at, is_smart, is_public, owner_user_id) VALUES ('legacy','manual',0,'{}',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,0,0,NULL)`)
	require.NoError(t, err)
	id, err := res.LastInsertId()
	require.NoError(t, err)
	var owner interface{}
	require.NoError(t, db.QueryRow(`SELECT owner_user_id FROM media_collections WHERE id = ?`, id).Scan(&owner))
	require.Nil(t, owner, "fixture must be ownerless")

	h := NewCollectionHandler(repository.NewMediaCollectionRepository(db))
	as := func(uid, role int) *gin.Engine {
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("user_id", uid); c.Set("role_id", role); c.Next() })
		r.GET("/c/:id", h.GetCollection)
		r.PUT("/c/:id", h.UpdateCollection)
		r.DELETE("/c/:id", h.DeleteCollection)
		return r
	}
	do := func(r *gin.Engine, method, body string) int {
		req := httptest.NewRequest(method, "/c/"+strconv.FormatInt(id, 10), bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Code
	}
	user, admin := as(7, 2), as(1, 1)

	require.Equal(t, http.StatusOK, do(user, http.MethodGet, ""), "non-admin may read a legacy shared collection")
	require.Equal(t, http.StatusForbidden, do(user, http.MethodPut, `{"name":"hijack"}`))
	require.Equal(t, http.StatusForbidden, do(user, http.MethodDelete, ""))
	var name string
	require.NoError(t, db.QueryRow(`SELECT name FROM media_collections WHERE id = ?`, id).Scan(&name))
	require.Equal(t, "legacy", name, "forbidden update must not change the row")

	require.Equal(t, http.StatusOK, do(admin, http.MethodPut, `{"name":"renamed"}`))
	require.NoError(t, db.QueryRow(`SELECT name FROM media_collections WHERE id = ?`, id).Scan(&name))
	require.Equal(t, "renamed", name)
	require.Equal(t, http.StatusOK, do(admin, http.MethodDelete, ""))
	var n int
	require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM media_collections WHERE id = ?`, id).Scan(&n))
	require.Equal(t, 0, n)
}
