package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"catalogizer/internal/pooltest"
	"catalogizer/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// Pool-deadlock class (see internal/poolguard): GetStorageInfo ran a per-root COUNT query while its
// storage_roots cursor was still open. Real SQLite file, production migrations, pool of
// pooltest.PoolSize connections, more concurrent callers than connections. Only the auth stub
// is a double (the subject is the database access).
func TestAdminHandler_GetStorageInfo_ConcurrentCallsExceedingPoolDoNotDeadlock(t *testing.T) {
	db, sqlDB := pooltest.NewPooledDB(t)
	// roots a..e with 1..5 files; root "a" never scanned (NULL last_scan_at).
	for i, n := range []string{"c", "a", "e", "b", "d"} { // inserted out of order: ORDER BY name must sort
		_, err := sqlDB.Exec(`INSERT INTO storage_roots (name, protocol, path, enabled, last_scan_at) VALUES (?, 'smb', ?, 1, ?)`,
			n, "/share/"+n, map[bool]any{true: nil, false: "2026-02-03 04:05:06"}[n == "a"])
		require.NoError(t, err)
		_ = i
	}
	counts := map[string]int{"a": 1, "b": 2, "c": 3, "d": 4, "e": 5}
	for n, c := range counts {
		var id int
		require.NoError(t, sqlDB.QueryRow(`SELECT id FROM storage_roots WHERE name = ?`, n).Scan(&id))
		for k := 0; k < c; k++ {
			_, err := sqlDB.Exec(`INSERT INTO files (storage_root_id, path, name, extension, mime_type, file_type, size, is_directory, modified_at)
				VALUES (?, ?, ?, 'mp4', 'video/mp4', 'video', 1, 0, '2026-01-01 00:00:00')`, id, fmt.Sprintf("/%s/f%d", n, k), fmt.Sprintf("f%d", k))
			require.NoError(t, err)
		}
	}

	auth := &mockAdminAuthService{
		getCurrentUserFn:  func(string) (*models.User, error) { return &models.User{ID: 1}, nil },
		checkPermissionFn: func(int, string) (bool, error) { return true, nil },
	}
	h := NewAdminHandler(auth, &mockAdminUserRepo{}, db, "test")
	call := func() ([]map[string]any, error) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", "/admin/storage", nil)
		c.Request.Header.Set("Authorization", "Bearer token")
		h.GetStorageInfo(c)
		if w.Code != http.StatusOK {
			return nil, fmt.Errorf("status %d: %s", w.Code, w.Body.String())
		}
		var out []map[string]any
		return out, json.Unmarshal(w.Body.Bytes(), &out)
	}

	// Exact output on a seeded dataset (sorted by name, per-root counts, lastScan omitted when NULL).
	want, err := call()
	require.NoError(t, err)
	require.Len(t, want, 5)
	for i, n := range []string{"a", "b", "c", "d", "e"} {
		require.Equal(t, "smb:///share/"+n, want[i]["path"])
		require.EqualValues(t, counts[n], want[i]["mediaCount"], n)
	}
	require.NotContains(t, want[0], "lastScan")
	require.Equal(t, "2026-02-03T04:05:06Z", want[1]["lastScan"])

	pooltest.RunConcurrent(t, 12, 15*time.Second, func(int) error {
		got, e := call()
		if e != nil {
			return e
		}
		if len(got) != 5 {
			return fmt.Errorf("got %d roots, want 5", len(got))
		}
		for i := range got {
			if got[i]["mediaCount"] != want[i]["mediaCount"] || got[i]["path"] != want[i]["path"] {
				return fmt.Errorf("root %d differs: %v vs %v", i, got[i], want[i])
			}
		}
		return nil
	})
}
