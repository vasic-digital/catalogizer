package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CAT-UI-FAV-1b: /media/search returns FILE rows (files.id). The UI stores
// favourites under (entity_type, entity_id) of media_items and opens
// /entity/:id, so each row must also carry the linked media entity id + type.
func TestSearchMedia_CarriesLinkedEntityIDAndType(t *testing.T) {
	db, cleanup := setupEntityTestDB(t)
	defer cleanup()
	ctx := context.Background()
	exec := func(q string, a ...interface{}) int64 {
		r, err := db.ExecContext(ctx, q, a...)
		require.NoError(t, err)
		id, _ := r.LastInsertId()
		return id
	}
	root := exec(`INSERT INTO storage_roots (name, protocol, path, enabled) VALUES ('r','local','/x',1)`)
	mkFile := func(name string) int64 {
		return exec(`INSERT INTO files (storage_root_id, path, name, size, modified_at, file_type) VALUES (?,?,?,?,CURRENT_TIMESTAMP,'video')`,
			root, "/x/"+name, name, 10)
	}
	linked := mkFile("a.mkv")
	multi := mkFile("b.mkv")
	orphan := mkFile("c.mkv")
	var songType, movieType int64
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM media_types WHERE name='song'`).Scan(&songType))
	require.NoError(t, db.QueryRowContext(ctx, `SELECT id FROM media_types WHERE name='movie'`).Scan(&movieType))
	// entity ids deliberately differ from file ids
	exec(`INSERT INTO media_items (id, media_type_id, title, status) VALUES (900, ?, 'S', 'detected')`, songType)
	exec(`INSERT INTO media_items (id, media_type_id, title, status) VALUES (901, ?, 'M1', 'detected')`, movieType)
	exec(`INSERT INTO media_items (id, media_type_id, title, status) VALUES (902, ?, 'M2', 'detected')`, movieType)
	exec(`INSERT INTO media_files (media_item_id, file_id, is_primary) VALUES (900, ?, 1)`, linked)
	exec(`INSERT INTO media_files (media_item_id, file_id, is_primary) VALUES (901, ?, 0)`, multi)
	exec(`INSERT INTO media_files (media_item_id, file_id, is_primary) VALUES (902, ?, 1)`, multi)

	h := NewMediaBrowseHandler(repository.NewFileRepository(db), repository.NewStatsRepository(db), db)
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/media/search", nil)
	h.SearchMedia(c)
	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Items []map[string]interface{} `json:"items"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	byID := map[int64]map[string]interface{}{}
	for _, it := range resp.Items {
		byID[int64(it["id"].(float64))] = it
	}
	require.Len(t, byID, 3)

	assert.EqualValues(t, 900, byID[linked]["entity_id"])
	assert.Equal(t, "song", byID[linked]["entity_type"])
	// primary link (902) wins over a lower-id non-primary one (901)
	assert.EqualValues(t, 902, byID[multi]["entity_id"])
	assert.Equal(t, "movie", byID[multi]["entity_type"])
	// unlinked file: no entity fields at all (never a guessed id)
	_, hasID := byID[orphan]["entity_id"]
	_, hasType := byID[orphan]["entity_type"]
	assert.False(t, hasID)
	assert.False(t, hasType)
	// file id is still the row id
	assert.EqualValues(t, orphan, byID[orphan]["id"])
}
