package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// HYG-R1-F3: buildRouter installs AccessLogRouteParams after the access logger, so a sensitive route parameter in a path
// position no secret-introducing segment precedes never reaches the access log of the production engine.
func TestBuildRouter_AccessLogRedactsRouteParam(t *testing.T) {
	const tok = "Qm4Zt8Wv1Nc6Hy3P" // synthetic marker
	var buf bytes.Buffer
	prev := gin.DefaultWriter
	gin.DefaultWriter = &buf
	defer func() { gin.DefaultWriter = prev }()
	e := buildRouter(routerTestDeps())
	e.GET("/zzhyg/items/:token/detail", func(c *gin.Context) { c.Status(http.StatusOK) })
	e.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/zzhyg/items/"+tok+"/detail", nil))
	out := buf.String()
	if !strings.Contains(out, "/zzhyg/items/") {
		t.Fatalf("control needle: request missing from access log: %q", out)
	}
	if strings.Contains(out, tok) {
		t.Fatalf("route parameter reached the production access log: %q", out)
	}
}
