package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

const lanSecret = "this-secret-is-at-least-16-bytes-long"

// lanRouter builds a real gin router with the real CSRF middleware wired
// exactly the way main.go wires it (options resolved from the env toggle).
func lanRouter(t *testing.T, toggle, appEnv string) (*gin.Engine, []string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	var warns []string
	opts := CSRFOptionsFromEnv(toggle, appEnv, func(f string, a ...any) { warns = append(warns, f) })
	guard, err := NewCSRF([]byte(lanSecret), opts...)
	if err != nil {
		t.Fatal(err)
	}
	r := gin.New()
	g := r.Group("/admin", guard.Handler())
	g.GET("/probe", func(c *gin.Context) { c.Status(http.StatusOK) })
	g.PUT("/users/1", func(c *gin.Context) { c.Status(http.StatusOK) })
	return r, warns
}

func mint(t *testing.T, r *gin.Engine) (*http.Cookie, string) {
	t.Helper()
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodGet, "/admin/probe", nil)
	r.ServeHTTP(w, req)
	cs := w.Result().Cookies()
	if len(cs) != 1 {
		t.Fatalf("want 1 cookie, got %d", len(cs))
	}
	return cs[0], w.Header().Get("X-CSRF-Token")
}

func put(r *gin.Engine, ck *http.Cookie, tok string) int {
	w := httptest.NewRecorder()
	req, _ := http.NewRequest(http.MethodPut, "/admin/users/1", strings.NewReader("{}"))
	if ck != nil {
		req.AddCookie(&http.Cookie{Name: ck.Name, Value: ck.Value})
	}
	if tok != "" {
		req.Header.Set("X-CSRF-Token", tok)
	}
	r.ServeHTTP(w, req)
	return w.Code
}

func TestCSRFLAN_DefaultIsSecureHostPrefixed(t *testing.T) {
	for _, env := range []string{"production", "development", ""} {
		r, _ := lanRouter(t, "", env)
		ck, tok := mint(t, r)
		if ck.Name != "__Host-csrf" || !ck.Secure {
			t.Fatalf("env=%q default must be __Host-csrf+Secure, got %s secure=%v", env, ck.Name, ck.Secure)
		}
		if got := put(r, ck, tok); got != http.StatusOK {
			t.Fatalf("default success path got %d", got)
		}
	}
}

func TestCSRFLAN_EnabledInDevelopmentWorksOverPlainHTTP(t *testing.T) {
	for _, v := range []string{"true", "1", "TRUE"} {
		r, _ := lanRouter(t, v, "development")
		ck, tok := mint(t, r)
		if strings.HasPrefix(ck.Name, "__Host-") || ck.Secure {
			t.Fatalf("toggle=%q: cookie must lose __Host- and Secure, got %s secure=%v", v, ck.Name, ck.Secure)
		}
		if got := put(r, ck, tok); got != http.StatusOK {
			t.Fatalf("success path got %d", got)
		}
		if got := put(r, ck, tok+"x"); got != http.StatusForbidden {
			t.Fatalf("mismatch must be 403, got %d", got)
		}
		if got := put(r, ck, ""); got != http.StatusForbidden {
			t.Fatalf("missing header must be 403, got %d", got)
		}
		if got := put(r, nil, tok); got != http.StatusForbidden {
			t.Fatalf("missing cookie must be 403, got %d", got)
		}
	}
}

func TestCSRFLAN_EnabledInProductionIsRefused(t *testing.T) {
	for _, env := range []string{"production", "Production", ""} {
		r, warns := lanRouter(t, "true", env)
		ck, _ := mint(t, r)
		if ck.Name != "__Host-csrf" || !ck.Secure {
			t.Fatalf("env=%q: production must ignore toggle, got %s secure=%v", env, ck.Name, ck.Secure)
		}
		if len(warns) == 0 {
			t.Fatalf("env=%q: refusal must be logged loudly", env)
		}
	}
}

func TestCSRFLAN_NonTruthyToggleStaysSecure(t *testing.T) {
	for _, v := range []string{"", "false", "0", "yes-please"} {
		r, _ := lanRouter(t, v, "development")
		if ck, _ := mint(t, r); !ck.Secure {
			t.Fatalf("toggle=%q must stay secure", v)
		}
	}
}
