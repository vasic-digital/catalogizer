package middleware

// CAT-PXF23-R5: the validation_failed response must expose only client-correctable text.
import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"digital.vasic.security/pkg/guardrails"
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
	// R5-F5: a server-side misconfiguration is a server error, not a client error.
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, w.Body.String(), "invalid validation pattern", "server config error text must not reach the client")
	assert.NotContains(t, w.Body.String(), "unclosed", "pattern text must not reach the client")
	assert.NotContains(t, w.Body.String(), "validation_failed")
	assert.Contains(t, w.Body.String(), "internal_error")
}

func TestPXF23R5_ClientCorrectableValidationTextPreserved(t *testing.T) {
	cfg := DefaultInputValidationConfig()
	w := r5Do(t, cfg, `{"q":"1' OR '1'='1"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "potential SQL injection detected in field: q")
	w = r5Do(t, cfg, `{bad json`)
	assert.Contains(t, w.Body.String(), "invalid JSON format")
}

// R5-F2: a real guardrail engine whose rule error text carries an internal marker; the marker
// (and the rule's error text) must not reach the response, while the field name stays visible.
type r5MarkerRule struct{}

func (r5MarkerRule) Name() string { return "r5_marker" }
func (r5MarkerRule) Check(content string) error {
	if strings.Contains(content, "trigger") {
		return errors.New("rule backend pq: connection refused host=db")
	}
	return nil
}

func TestPXF23R5_GuardrailInternalErrorTextNotReturned(t *testing.T) {
	cfg := DefaultInputValidationConfig()
	eng := guardrails.NewEngine(guardrails.DefaultConfig())
	eng.AddRule(r5MarkerRule{})
	cfg.GuardrailEngine = eng
	w := r5Do(t, cfg, `{"note":"please trigger it"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	all := w.Body.String()
	for k, v := range w.Header() {
		all += "\n" + k + ": " + strings.Join(v, ",")
	}
	assert.NotContains(t, all, "connection refused")
	assert.NotContains(t, all, "host=db")
	assert.NotContains(t, all, "rule backend")
	assert.Contains(t, w.Body.String(), "guardrail violation in field note")
	// A clean value passes the same engine.
	w = r5Do(t, cfg, `{"note":"fine"}`)
	assert.Equal(t, http.StatusOK, w.Code)
}
