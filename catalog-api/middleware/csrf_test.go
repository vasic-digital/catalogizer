package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func buildCSRFRouter(t *testing.T, secret []byte) *gin.Engine {
	t.Helper()
	c, err := NewCSRF(secret)
	if err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(c.Handler())
	r.GET("/ping", func(c *gin.Context) { c.String(http.StatusOK, "pong") })
	r.POST("/write", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

func TestCSRF_GETMintsToken(t *testing.T) {
	r := buildCSRFRouter(t, bytes.Repeat([]byte{0xab}, 32))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/ping", nil))
	if w.Header().Get("X-CSRF-Token") == "" {
		t.Error("GET must mint a token header")
	}
	if !strings.Contains(w.Header().Get("Set-Cookie"), "__Host-csrf=") {
		t.Errorf("cookie not set: %q", w.Header().Get("Set-Cookie"))
	}
}

func TestCSRF_POSTWithoutTokenForbidden(t *testing.T) {
	r := buildCSRFRouter(t, bytes.Repeat([]byte{0xab}, 32))
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/write", nil))
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestCSRF_POSTWithMatchingTokenAccepted(t *testing.T) {
	r := buildCSRFRouter(t, bytes.Repeat([]byte{0xab}, 32))

	// First, mint a token via GET.
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, httptest.NewRequest("GET", "/ping", nil))
	token := getRec.Header().Get("X-CSRF-Token")
	cookie := parseSetCookie(getRec.Header().Get("Set-Cookie"), "__Host-csrf")
	if token == "" || cookie == "" {
		t.Fatalf("mint failed: token=%q cookie=%q", token, cookie)
	}

	// Now POST with the matching header + cookie.
	req := httptest.NewRequest("POST", "/write", nil)
	req.AddCookie(&http.Cookie{Name: "__Host-csrf", Value: cookie})
	req.Header.Set("X-CSRF-Token", token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("valid request = %d %s, want 200", w.Code, w.Body.String())
	}
}

func TestCSRF_POSTWithWrongHeaderForbidden(t *testing.T) {
	r := buildCSRFRouter(t, bytes.Repeat([]byte{0xab}, 32))
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, httptest.NewRequest("GET", "/ping", nil))
	cookie := parseSetCookie(getRec.Header().Get("Set-Cookie"), "__Host-csrf")

	req := httptest.NewRequest("POST", "/write", nil)
	req.AddCookie(&http.Cookie{Name: "__Host-csrf", Value: cookie})
	req.Header.Set("X-CSRF-Token", "not-the-right-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestCSRF_POSTWithoutCookieForbidden(t *testing.T) {
	r := buildCSRFRouter(t, bytes.Repeat([]byte{0xab}, 32))
	getRec := httptest.NewRecorder()
	r.ServeHTTP(getRec, httptest.NewRequest("GET", "/ping", nil))
	token := getRec.Header().Get("X-CSRF-Token")

	req := httptest.NewRequest("POST", "/write", nil)
	req.Header.Set("X-CSRF-Token", token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

func TestSubtleCompare(t *testing.T) {
	if subtleCompare("abc", "abc") != 1 {
		t.Error("equal strings must compare 1")
	}
	if subtleCompare("abc", "abd") != 0 {
		t.Error("unequal strings must compare 0")
	}
	if subtleCompare("ab", "abc") != 0 {
		t.Error("differing lengths must compare 0")
	}
}

// parseSetCookie extracts the value of the named cookie from a raw
// Set-Cookie header. Gin writes one cookie per header; this helper
// keeps the test file short.
func parseSetCookie(header, name string) string {
	parts := strings.Split(header, ";")
	if len(parts) == 0 {
		return ""
	}
	kv := strings.SplitN(parts[0], "=", 2)
	if len(kv) != 2 || kv[0] != name {
		return ""
	}
	return kv[1]
}

// Found by the LIVE admin spec (intermittent 403 on Approve): a browser fires several GETs in parallel; every GET without the header
// minted a NEW token+cookie, so the cookie the browser kept and the token the page kept could belong to different mints.
func TestCSRF_GETWithValidCookieReusesToken(t *testing.T) {
	r := buildCSRFRouter(t, bytes.Repeat([]byte{0xab}, 32))
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest("GET", "/ping", nil))
	tok := w1.Header().Get("X-CSRF-Token")
	cookie := strings.SplitN(strings.SplitN(w1.Header().Get("Set-Cookie"), ";", 2)[0], "=", 2)[1]
	req := httptest.NewRequest("GET", "/ping", nil)
	req.AddCookie(&http.Cookie{Name: "__Host-csrf", Value: cookie})
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if got := w2.Header().Get("X-CSRF-Token"); got != tok {
		t.Errorf("second GET with a valid cookie must echo the same token: got %q want %q", got, tok)
	}
	if sc := w2.Header().Get("Set-Cookie"); sc != "" {
		t.Errorf("no new cookie may be minted while a valid one is presented, got %q", sc)
	}
}

func TestCSRF_RejectionReissuesUsableToken(t *testing.T) {
	r := buildCSRFRouter(t, bytes.Repeat([]byte{0xab}, 32))
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, httptest.NewRequest("GET", "/ping", nil))
	staleCookie := strings.SplitN(strings.SplitN(w1.Header().Get("Set-Cookie"), ";", 2)[0], "=", 2)[1]

	req := httptest.NewRequest("POST", "/write", nil)
	req.AddCookie(&http.Cookie{Name: "__Host-csrf", Value: staleCookie})
	req.Header.Set("X-CSRF-Token", "not-the-token")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != http.StatusForbidden {
		t.Fatalf("mismatch must be 403, got %d", w2.Code)
	}
	fresh := w2.Header().Get("X-CSRF-Token")
	freshCookie := ""
	if sc := w2.Header().Get("Set-Cookie"); strings.Contains(sc, "__Host-csrf=") {
		freshCookie = strings.SplitN(strings.SplitN(sc, ";", 2)[0], "=", 2)[1]
	}
	if fresh == "" || freshCookie == "" {
		t.Fatalf("a csrf rejection must hand back a fresh token+cookie so one retry succeeds (header=%q cookie=%q)", fresh, freshCookie)
	}
	retry := httptest.NewRequest("POST", "/write", nil)
	retry.AddCookie(&http.Cookie{Name: "__Host-csrf", Value: freshCookie})
	retry.Header.Set("X-CSRF-Token", fresh)
	w3 := httptest.NewRecorder()
	r.ServeHTTP(w3, retry)
	if w3.Code != http.StatusOK {
		t.Errorf("retry with the reissued pair must pass, got %d", w3.Code)
	}
}
