package main

import (
	"bytes"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

// HYG-R3-F2: buildRouter-level tests do not see what main() registers AFTER buildRouter (the /api/v1 group, aliases, WebSocket
// ... and the late Alt-Svc Use) nor the two verifyAuthRouteTable calls main() makes at startup, which logger.Fatal on a mismatch.
// This test runs the REAL main() (the test binary re-executes itself into main, so the whole registration code path runs, not a
// copy of it) against a throw-away SQLite directory and requires: no "Auth route table check failed" Fatal, the server reached
// "Starting catalog API server" (past BOTH verify calls) and /health answers 200. A global-chain change that breaks startup, or
// a route the pinned table does not know, therefore fails a test instead of the first production start.
const runMainEnv = "CATALOGIZER_TEST_RUN_MAIN"

// TestHelperRunMain is the child entry point: it runs main() and only when the parent asked for it.
func TestHelperRunMain(t *testing.T) {
	if os.Getenv(runMainEnv) != "1" {
		t.Skip("child-process entry point; run by TestMainFullRouteRegistrationPassesVerifyAuthRouteTable")
	}
	main()
}

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string             { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

func r3f2FreePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func TestMainFullRouteRegistrationPassesVerifyAuthRouteTable(t *testing.T) {
	if os.Getenv(runMainEnv) == "1" {
		t.Skip("running inside the child")
	}
	dir := t.TempDir()
	port := r3f2FreePort(t)
	// Throw-away random secrets: generated here, passed to the child only, never printed.
	rnd := func() string { return fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), os.Getpid(), port) + strings.Repeat("x", 24) }
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperRunMain$", "-test.v=false")
	cmd.Dir = dir
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"), "HOME=" + dir, runMainEnv + "=1", "APP_ENV=development", "GIN_MODE=release",
		"JWT_SECRET=" + rnd(), "ADMIN_USERNAME=admin", "ADMIN_PASSWORD=" + rnd(),
		"DATABASE_TYPE=sqlite", fmt.Sprintf("SERVER_PORT=%d", port), "HOST=127.0.0.1",
	}
	out := &syncBuf{}
	cmd.Stdout, cmd.Stderr = out, out
	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	stop := func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case <-exited:
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			<-exited
		}
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/health", port)
	deadline := time.Now().Add(90 * time.Second)
	var healthy bool
	for time.Now().Before(deadline) {
		select {
		case err := <-exited:
			t.Fatalf("main() exited during startup (%v), the auth route table check or another startup step failed:\n%s", err, tail(out.String(), 3000))
		default:
		}
		if resp, err := http.Get(url); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK && strings.Contains(out.String(), "Starting catalog API server") {
				healthy = true
				break
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	got := out.String()
	stop()
	if !healthy {
		t.Fatalf("main() did not reach a serving state within 90s:\n%s", tail(got, 3000))
	}
	if strings.Contains(got, "Auth route table check failed") {
		t.Fatalf("auth route table check failed at startup:\n%s", tail(got, 3000))
	}
	// Control needle: the log is the child's real output (it contains the startup line the success condition required).
	if !strings.Contains(got, "Starting catalog API server") {
		t.Fatalf("control needle: startup line missing")
	}
}

func tail(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}
