package handlers

// CAT-001: GET /admin/system-info reported cpuUsage 0, diskUsage {0,0,0}, totalRequests 0 as if measured ("return 0 for now"), and
// GET /admin/storage reported totalSpace/usedSpace/availableSpace 0. Real measurements (/proc/stat, statfs on the real file system)
// where the host can provide them; JSON null plus an "unavailable" list where it cannot - never a fabricated 0.

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"

	"catalogizer/models"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func adminCall(t *testing.T, h *AdminHandler, fn func(*gin.Context)) (int, map[string]interface{}, []interface{}) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/admin/x", nil)
	c.Request.Header.Set("Authorization", "Bearer token")
	fn(c)
	var obj map[string]interface{}
	var arr []interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &obj); err != nil {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &arr))
	}
	return w.Code, obj, arr
}

func adminAuthOK() *mockAdminAuthService {
	return &mockAdminAuthService{
		getCurrentUserFn:  func(string) (*models.User, error) { return &models.User{ID: 1}, nil },
		checkPermissionFn: func(int, string) (bool, error) { return true, nil },
	}
}

func TestCAT001_SystemInfo_ReportsMeasuredDiskAndHonestUnknowns(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("topology_unsupported: /proc/stat and statfs measurements are asserted on Linux only")
	}
	h := newTestAdminHandler(adminAuthOK(), &mockAdminUserRepo{})
	code, resp, _ := adminCall(t, h, h.GetSystemInfo)
	require.Equal(t, http.StatusOK, code)

	disk, _ := resp["diskUsage"].(map[string]interface{})
	require.NotNil(t, disk, "%v", resp)
	total, used, free := disk["total"].(float64), disk["used"].(float64), disk["free"].(float64)
	assert.Greater(t, total, 0.0, "the file system has a size")
	assert.Greater(t, free, 0.0)
	assert.LessOrEqual(t, used, total)

	cpu, ok := resp["cpuUsage"].(float64)
	require.True(t, ok, "cpuUsage is a measured number on Linux: %v", resp["cpuUsage"])
	assert.GreaterOrEqual(t, cpu, 0.0)
	assert.LessOrEqual(t, cpu, 100.0)

	// no request counter exists: unknown, not 0
	assert.Nil(t, resp["totalRequests"])
	un, _ := resp["unavailable"].([]interface{})
	assert.Contains(t, un, "totalRequests")
}

func TestCAT001_StorageInfo_LocalRootHasRealSpaceOtherProtocolsAreNull(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("topology_unsupported")
	}
	db := openProductionFileDB(t)
	dir := t.TempDir()
	// An smb root whose path happens to exist locally: a measurement of it would
	// be a statfs of the wrong machine, so it must still be null.
	remDir := t.TempDir()
	_, err := db.Exec(`INSERT INTO storage_roots (name, protocol, path, enabled, max_depth) VALUES ('loc','local',?,1,3)`, dir)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO storage_roots (name, protocol, host, path, enabled, max_depth) VALUES ('rem','smb','nas.invalid',?,1,3)`, remDir)
	require.NoError(t, err)
	h := NewAdminHandler(adminAuthOK(), &mockAdminUserRepo{}, db, "t")

	code, _, arr := adminCall(t, h, h.GetStorageInfo)
	require.Equal(t, http.StatusOK, code)
	require.Len(t, arr, 2)
	byPath := map[string]map[string]interface{}{}
	for _, e := range arr {
		m := e.(map[string]interface{})
		byPath[m["path"].(string)] = m
	}
	loc := byPath["local://"+dir]
	require.NotNil(t, loc, "%v", byPath)
	assert.Greater(t, loc["totalSpace"].(float64), 0.0)
	assert.Greater(t, loc["availableSpace"].(float64), 0.0)
	rem := byPath["smb://"+remDir]
	require.NotNil(t, rem, "%v", byPath)
	assert.Nil(t, rem["totalSpace"], "space of a remote share is unknown, not 0")
	assert.Nil(t, rem["availableSpace"])
}
