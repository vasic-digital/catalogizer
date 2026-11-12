package middleware

// CAT-PXF23-R5: the validation_failed response must expose only client-correctable text.
import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func r5Do(t *testing.T, cfg InputValidationConfig, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(InputValidation(cfg))
	r.POST("/x", func(c *gin.Context) { c.String(200, "ok") })
	req := httptest.NewRequest(http.MethodPost, "/x", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestPXF23R5_InternalValidationErrorTextNotReturned(t *testing.T) {
	cfg := DefaultInputValidationConfig()
	cfg.CustomRules = map[string]string{"code": "([unclosed"} // server-side misconfiguration, not client error
	w := r5Do(t, cfg, `{"code":"abc"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.NotContains(t, w.Body.String(), "invalid validation pattern", "server config error text must not reach the client")
	assert.Contains(t, w.Body.String(), "validation_failed")
}

func TestPXF23R5_ClientCorrectableValidationTextPreserved(t *testing.T) {
	cfg := DefaultInputValidationConfig()
	w := r5Do(t, cfg, `{"q":"1' OR '1'='1"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "potential SQL injection detected in field: q")
	w = r5Do(t, cfg, `{bad json`)
	assert.Contains(t, w.Body.String(), "invalid JSON format")
}
