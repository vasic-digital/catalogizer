package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type binEnv struct {
	t    *testing.T
	bin  string
	dir  string
	port int
}

func (b *binEnv) env(extra ...string) []string {
	e := []string{
		"PATH=" + os.Getenv("PATH"), "HOME=" + b.dir,
		"JWT_SECRET=Zq7-test-only-jwt-secret-0123456789abcdef",
		"ADMIN_USERNAME=admin", "HOST=127.0.0.1", fmt.Sprintf("SERVER_PORT=%d", b.port),
		"DATABASE_TYPE=sqlite", "DATABASE_ENCRYPTION_KEY_FILE=" + filepath.Join(b.dir, "keys", "db.key"),
	}
	return append(e, extra...)
}

// start launches the server and returns (cmd, exited-channel). Only the child
// pid is signalled (never a process group, §11.4.263).
func (b *binEnv) start(extra ...string) (*exec.Cmd, chan error, *bytes.Buffer) {
	cmd := exec.Command(b.bin)
	cmd.Dir = b.dir
	cmd.Env = b.env(extra...)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	require.NoError(b.t, cmd.Start())
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	return cmd, done, &out
}

func (b *binEnv) waitHealthy(done chan error) bool {
	for i := 0; i < 120; i++ {
		select {
		case <-done:
			return false
		default:
		}
		if r, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/health", b.port)); err == nil {
			r.Body.Close()
			return true
		}
		time.Sleep(500 * time.Millisecond)
	}
	return false
}

func (b *binEnv) stop(cmd *exec.Cmd, done chan error) {
	_ = cmd.Process.Signal(os.Interrupt)
	select {
	case <-done:
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		<-done
	}
}

func (b *binEnv) login(pw string) (int, map[string]interface{}) {
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": pw})
	r, err := http.Post(fmt.Sprintf("http://127.0.0.1:%d/api/v1/auth/login", b.port), "application/json", bytes.NewReader(body))
	require.NoError(b.t, err)
	defer r.Body.Close()
	raw, _ := io.ReadAll(r.Body)
	var m map[string]interface{}
	_ = json.Unmarshal(raw, &m)
	return r.StatusCode, m
}

func (b *binEnv) get(path, token string) int {
	req, _ := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d%s", b.port, path), nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r, err := http.DefaultClient.Do(req)
	require.NoError(b.t, err)
	r.Body.Close()
	return r.StatusCode
}

// Real-binary lifecycle (CAT-021 I1/I3): a compromised install refuses to start,
// the offline subcommand rotates it, the old credentials and sessions die, the
// new password works. Kills reviewer mutants on main.go wiring (Fatal->Warn,
// dispatch disabled).
func TestAdminReset_RealBinaryLifecycle(t *testing.T) {
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
	const goodPw = "Op3rator-Supplied-Value-9x"
	const newPw = "Rotated-Op3rator-Value-7q"

	// 1. dev-mode first start seeds the compromised default; capture a session.
	cmd, done, out := b.start("CATALOGIZER_ALLOW_INSECURE_DEFAULTS=true", "GIN_MODE=debug", "ADMIN_PASSWORD="+seedCompromised)
	require.True(t, b.waitHealthy(done), "dev start must come up:\n%s", out.String())
	code, login := b.login(seedCompromised)
	require.Equal(t, 200, code, "%v", login)
	oldToken, _ := login["session_token"].(string)
	require.NotEmpty(t, oldToken)
	require.Equal(t, 200, b.get("/api/v1/auth/me", oldToken))
	// JWT-only route (guarded by middleware.RequireAuth alone, no session lookup in the handler).
	require.Equal(t, 200, b.get("/api/v1/catalog", oldToken))
	b.stop(cmd, done)

	// 2. production-mode start must REFUSE (Fatal) and name the recovery command.
	cmd, done, out = b.start("GIN_MODE=release", "ADMIN_PASSWORD="+goodPw)
	select {
	case err := <-done:
		require.Error(t, err, "server must exit non-zero")
	case <-time.After(60 * time.Second):
		b.stop(cmd, done)
		t.Fatalf("server did not refuse to start:\n%s", out.String())
	}
	assert.Contains(t, out.String(), "admin-reset-password")

	// 3. offline rotation (real binary, no password echoed).
	rc := exec.Command(bin, "admin-reset-password")
	rc.Dir = dir
	rc.Env = b.env("GIN_MODE=release", "ADMIN_PASSWORD="+goodPw, "ADMIN_NEW_PASSWORD="+newPw)
	rout, err := rc.CombinedOutput()
	require.NoError(t, err, "%s", rout)
	assert.Contains(t, string(rout), "rotated admin account: admin")
	assert.NotContains(t, string(rout), newPw)

	// 4. now it starts; new password works, old does not, old session is dead.
	cmd, done, out = b.start("GIN_MODE=release", "ADMIN_PASSWORD="+goodPw)
	require.True(t, b.waitHealthy(done), "start after rotation:\n%s", out.String())
	defer b.stop(cmd, done)
	code, nl := b.login(newPw)
	assert.Equal(t, 200, code, "%v", nl)
	oc, _ := b.login(seedCompromised)
	assert.Equal(t, 401, oc)
	assert.Equal(t, 401, b.get("/api/v1/auth/me", oldToken), "pre-reset session must be rejected")
	// F1 (round 4): stateless access JWTs on JWT-only routes must die with the session.
	assert.Equal(t, 401, b.get("/api/v1/catalog", oldToken), "pre-reset access token must be rejected on JWT-only routes")
	if nt, _ := nl["session_token"].(string); nt != "" {
		assert.Equal(t, 200, b.get("/api/v1/auth/me", nt))
		assert.Equal(t, 200, b.get("/api/v1/catalog", nt), "post-reset token must work on JWT-only routes")
	}
	assert.False(t, strings.Contains(out.String(), seedCompromised))
}

// The real binary refuses an empty replacement password before any DB work.
func TestAdminReset_RealBinaryRefusesEmptyNewPassword(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary")
	}
	dir := t.TempDir()
	bin := filepath.Join(dir, "catalog-api")
	if out, err := exec.Command("go", "build", "-buildvcs=false", "-o", bin, ".").CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	cmd := exec.Command(bin, "admin-reset-password")
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "ADMIN_NEW_PASSWORD=",
		"JWT_SECRET=Zq7-test-only-jwt-secret-0123456789abcdef", "ADMIN_USERNAME=admin", "ADMIN_PASSWORD=Op3rator-Supplied-Value-9x"}
	out, err := cmd.CombinedOutput()
	require.Error(t, err)
	assert.Contains(t, string(out), "ADMIN_NEW_PASSWORD")
}
