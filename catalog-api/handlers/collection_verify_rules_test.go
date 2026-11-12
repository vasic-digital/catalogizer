package handlers

// CAT-PXF23-R R1: verifyRules answers a client-correctable ValidationError with its text in the 400 message.

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"catalogizer/config"
	"catalogizer/database"
	"catalogizer/internal/smartrules"
	"catalogizer/repository"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newVerifyRulesHandler(t *testing.T) (*CollectionHandler, *database.DB) {
	t.Helper()
	dir := t.TempDir()
	db, err := database.NewConnection(&config.DatabaseConfig{Type: "sqlite", Path: filepath.Join(dir, "c.db"),
		EncryptionKeyFile: filepath.Join(dir, "k", "db.key"), MaxOpenConnections: 2})
	require.NoError(t, err)
	require.NoError(t, db.RunMigrations(context.Background()))
	t.Cleanup(func() { db.Close() })
	return NewCollectionHandler(repository.NewMediaCollectionRepository(db)), db
}

func TestVerifyRules_ValidationErrorTextReachesClient(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, _ := newVerifyRulesHandler(t)
	rs, err := smartrules.Parse([]json.RawMessage{json.RawMessage(`{"field":"favorite","operator":"equals","value":true}`)})
	require.NoError(t, err)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/c", nil)
	_, ok := h.verifyRules(c, rs, 0) // uid 0: user-scoped field -> ValidationError at compile time
	require.False(t, ok)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), `needs an authenticated user`)
	assert.Contains(t, w.Body.String(), `favorite`)
	assert.NotContains(t, w.Body.String(), `"details"`)
}

// Non-validation failures stay generic: the driver text must not reach the client.
func TestVerifyRules_NonValidationErrorStaysGeneric(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h, db := newVerifyRulesHandler(t)
	rs, err := smartrules.Parse([]json.RawMessage{json.RawMessage(`{"field":"year","operator":"greater_than","value":0}`)})
	require.NoError(t, err)
	require.NoError(t, db.Close()) // evaluation now fails with a driver error

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/c", nil)
	_, ok := h.verifyRules(c, rs, 1)
	require.False(t, ok)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"success":false,"error":"Smart collection rules could not be evaluated"}`, w.Body.String())
}
