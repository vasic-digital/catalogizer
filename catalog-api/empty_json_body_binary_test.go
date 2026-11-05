package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// LIVE-CLASS (§11.4.289): the real catalog-api binary with its real router and middleware chain.
// D7/D8 remainder (docs/qa/live-conv-docs-subs-r1-20261104.md): the input-validation middleware
// runs before authentication and rejected an EMPTY body sent with Content-Type: application/json
// (the default of fetch-style clients on bodiless POSTs) with 400 "invalid JSON format", so an
// anonymous caller got 400 instead of 401 and an authenticated caller got 400 on endpoints that
// take no body.

func rawRequest(t *testing.T, method, url, token, contentType, body string) (int, string) {
	t.Helper()
	var rd io.Reader = http.NoBody
	if body != "" {
		rd = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, url, rd)
	require.NoError(t, err)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	r, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer r.Body.Close()
	out, _ := io.ReadAll(r.Body)
	return r.StatusCode, string(out)
}

func TestEmptyJSONBody_AuthDecidesBeforeBodyValidation_RealBinary(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "catalog-api")
	if out, err := exec.Command("go", "build", "-buildvcs=false", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	b := &binEnv{t: t, bin: bin, dir: dir, port: port}
	const adminPw = "Op3rator-Supplied-Value-9x"
	cmd, done, out := b.start("GIN_MODE=release", "ADMIN_PASSWORD="+adminPw)
	require.True(t, b.waitHealthy(done), "server must come up:\n%s", out.String())
	defer b.stop(cmd, done)
	base := fmt.Sprintf("http://127.0.0.1:%d", port)
	c := &apiClient{t: t, base: base}
	tok := c.login("admin", adminPw)
	const js = "application/json"

	// anonymous + empty JSON body: authentication answers first -> 401 (was 400)
	for _, p := range []string{"/api/v1/conversion/jobs/1/cancel", "/api/v1/conversion/jobs/1/retry", "/api/v1/conversion/jobs", "/api/v1/auth/logout"} {
		code, body := rawRequest(t, "POST", base+p, "", js, "")
		assert.Equal(t, 401, code, "anonymous empty JSON body on %s: %s", p, body)
	}
	// authenticated + empty JSON body on a bodiless action: the action runs (unknown job -> 404, not 400 invalid JSON)
	code, body := rawRequest(t, "POST", base+"/api/v1/conversion/jobs/999999/cancel", tok, js, "")
	assert.Equal(t, 404, code, "authenticated empty body on a bodiless action: %s", body)
	code, body = rawRequest(t, "POST", base+"/api/v1/conversion/jobs/999999/retry", tok, js, "")
	assert.Equal(t, 404, code, "%s", body)
	// authenticated + empty JSON body on an endpoint that REQUIRES a body: still 400, now from the handler
	code, body = rawRequest(t, "POST", base+"/api/v1/conversion/jobs", tok, js, "")
	assert.Equal(t, 400, code, "a body-requiring endpoint must still refuse an empty body: %s", body)
	assert.NotContains(t, body, "validation_failed", "the refusal must come from the handler, not the pre-auth middleware: %s", body)
	// whitespace-only is empty too
	code, body = rawRequest(t, "POST", base+"/api/v1/conversion/jobs/999999/cancel", tok, js, " \n")
	assert.Equal(t, 404, code, "%s", body)

	// controls: the middleware still validates NON-empty bodies (malformed JSON, injection)
	code, body = rawRequest(t, "POST", base+"/api/v1/conversion/jobs/999999/cancel", tok, js, "{not json")
	assert.Equal(t, 400, code, "malformed non-empty JSON must still be refused: %s", body)
	assert.Contains(t, body, "validation_failed")
	code, body = rawRequest(t, "POST", base+"/api/v1/auth/login", "", js, `{"username":"<script>alert(1)</script>","password":"x"}`)
	assert.Equal(t, 400, code, "an XSS payload must still be refused by the middleware: %s", body)
}
