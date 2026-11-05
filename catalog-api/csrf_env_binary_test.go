package main

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CSRF-R2-1: the main() call adminCSRFOptions(os.Getenv, ...) is covered only by unit tests of
// the helper, so a main() that passed a blank getenv survived. This starts the REAL binary with
// real environment variables and reads the CSRF cookie the admin group mints on a GET.
func TestAdminCSRFCookie_RealBinary_EnvToggleReachesMiddleware(t *testing.T) {
	if testing.Short() {
		t.Skip("builds and runs the server binary")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "catalog-api")
	if out, err := exec.Command("go", "build", "-buildvcs=false", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	const adminPw = "Op3rator-Supplied-Value-9x"
	cases := []struct {
		name       string
		env        []string
		wantCookie string
		wantSecure bool
	}{
		{"default is the secure __Host- cookie", []string{"APP_ENV=development"}, "__Host-csrf", true},
		{"toggle in a dev-like env reaches the middleware", []string{"APP_ENV=development", "CSRF_INSECURE_DEV=true"}, "csrf-dev", false},
		{"toggle is refused outside a dev-like env", []string{"APP_ENV=production", "CSRF_INSECURE_DEV=true"}, "__Host-csrf", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ln, err := net.Listen("tcp", "127.0.0.1:0")
			require.NoError(t, err)
			port := ln.Addr().(*net.TCPAddr).Port
			ln.Close()
			b := &binEnv{t: t, bin: bin, dir: t.TempDir(), port: port}
			cmd, done, out := b.start(append([]string{"GIN_MODE=release", "ADMIN_PASSWORD=" + adminPw}, tc.env...)...)
			require.True(t, b.waitHealthy(done), "server must come up:\n%s", out.String())
			defer b.stop(cmd, done)
			c := &apiClient{t: t, base: fmt.Sprintf("http://127.0.0.1:%d", port)}
			tok := c.login("admin", adminPw)
			req, _ := http.NewRequest("GET", c.base+"/api/v1/admin/health", nil)
			req.Header.Set("Authorization", "Bearer "+tok)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			resp.Body.Close()
			require.Equal(t, 200, resp.StatusCode, "control: the admin GET itself works")
			var got *http.Cookie
			for _, ck := range resp.Cookies() {
				if strings.Contains(ck.Name, "csrf") {
					got = ck
				}
			}
			require.NotNil(t, got, "the admin group must mint a CSRF cookie")
			assert.Equal(t, tc.wantCookie, got.Name)
			assert.Equal(t, tc.wantSecure, got.Secure)
		})
	}
}
