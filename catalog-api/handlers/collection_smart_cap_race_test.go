package handlers

// SECQ-4b: the per-user smart collection cap must hold under concurrent creates. Real router, real handler,
// real migrated encrypted SQLite, 16 parallel POSTs behind a start barrier from cap-1.

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCollectionHandler_SmartCapHoldsUnderConcurrentCreates(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for round := 0; round < 5; round++ {
		dir := t.TempDir()
		db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: filepath.Join(dir, "c.db"),
			EncryptionKeyFile: filepath.Join(dir, "k", "db.key"), MaxOpenConnections: 8})
		require.NoError(t, err)
		require.NoError(t, db.RunMigrations(context.Background()))
		_, err = db.Exec(`INSERT INTO users (username, email, password_hash, salt, role_id, is_active) VALUES ('u1','u1@x.invalid','h','s',2,1)`)
		require.NoError(t, err)
		var uid int
		require.NoError(t, db.QueryRow(`SELECT id FROM users WHERE username='u1'`).Scan(&uid))
		for i := 0; i < MaxSmartCollectionsPerUser-1; i++ {
			_, err = db.Exec(`INSERT INTO media_collections (name, collection_type, total_items, external_ids, created_at, updated_at, is_smart, is_public, owner_user_id) VALUES ('s','smart',0,'{}',CURRENT_TIMESTAMP,CURRENT_TIMESTAMP,1,0,?)`, uid)
			require.NoError(t, err)
		}
		h := NewCollectionHandler(repository.NewMediaCollectionRepository(db))
		r := gin.New()
		r.Use(func(c *gin.Context) { c.Set("user_id", uid); c.Set("role_id", 2); c.Next() })
		r.POST("/c", h.CreateCollection)

		const workers = 16
		var wg sync.WaitGroup
		start := make(chan struct{})
		codes := make([]int, workers)
		for i := 0; i < workers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				req := httptest.NewRequest(http.MethodPost, "/c", bytes.NewBufferString(
					`{"name":"race","is_smart":true,"smart_rules":[{"field":"year","operator":"greater_than","value":0}]}`))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				codes[i] = w.Code
			}(i)
		}
		close(start)
		wg.Wait()

		created, refused := 0, 0
		for _, c := range codes {
			switch c {
			case http.StatusCreated:
				created++
			case http.StatusConflict:
				refused++
			}
		}
		var stored int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM media_collections WHERE is_smart = 1 AND owner_user_id = ?`, uid).Scan(&stored))
		db.Close()
		require.LessOrEqual(t, stored, MaxSmartCollectionsPerUser, "round %d: cap exceeded (codes %v)", round, codes)
		require.Equal(t, 1, created, "round %d: exactly one create may succeed from cap-1 (codes %v)", round, codes)
		require.Equal(t, workers-1, refused, "round %d: the rest are refused with 409 (codes %v)", round, codes)
	}
}
