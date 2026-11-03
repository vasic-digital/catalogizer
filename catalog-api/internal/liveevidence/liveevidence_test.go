package liveevidence

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func repoRoot(t *testing.T) string {
	t.Helper()
	d, _ := os.Getwd()
	for i := 0; i < 6; i++ {
		if _, err := os.Stat(filepath.Join(d, "scripts/test-infra/verify_live_evidence.sh")); err == nil {
			return d
		}
		d = filepath.Dir(d)
	}
	t.Fatal("repo root not found")
	return ""
}

func verify(t *testing.T, path string) (int, string) {
	t.Helper()
	cmd := exec.Command(filepath.Join(repoRoot(t), "scripts/test-infra/verify_live_evidence.sh"), path)
	out, err := cmd.CombinedOutput()
	rc := 0
	if ee, ok := err.(*exec.ExitError); ok {
		rc = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return rc, string(out)
}

func freePort(t *testing.T) int {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return p
}

// startRealServer starts a REAL separate process (python3 http.server) on loopback.
func startRealServer(t *testing.T) (port int, exe string, cmd *exec.Cmd) {
	t.Helper()
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not present: hardware_not_present-class SKIP")
	}
	exe, _ = filepath.EvalSymlinks(py)
	port = freePort(t)
	cmd = exec.Command(py, "-m", "http.server", fmt.Sprint(port), "--bind", "127.0.0.1")
	cmd.Dir = t.TempDir()
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	for i := 0; i < 50; i++ {
		if r, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port)); err == nil {
			r.Body.Close()
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatal("server did not start")
	return
}

func fileSHA(t *testing.T, p string) string {
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

func TestResolveListenerFindsRealProcess(t *testing.T) {
	port, exe, cmd := startRealServer(t)
	a, err := ResolveListener(port)
	if err != nil {
		t.Fatal(err)
	}
	if a.PID != cmd.Process.Pid || a.ExePath != exe || a.ExeSHA256 != fileSHA(t, exe) {
		t.Fatalf("got %+v want pid=%d exe=%s", a, cmd.Process.Pid, exe)
	}
}

func TestResolveListenerControlNeedleNoListener(t *testing.T) {
	if _, err := ResolveListener(freePort(t)); err == nil {
		t.Fatal("expected error for a port nobody listens on")
	}
}

func TestCarrierRealLoopbackServiceStaysLive(t *testing.T) {
	port, exe, _ := startRealServer(t)
	r := NewRecord("go-carrier", "go test ./internal/liveevidence")
	r.SetArtifact("sha256:"+fileSHA(t, exe), "target")
	if err := r.ResolveAnswering(fmt.Sprintf("127.0.0.1:%d", port), fileSHA(t, exe)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		r.Observe("GET", resp.StatusCode)
	}
	r.Finish("PASS", []string{"ok", "ok", "ok"})
	p, err := r.Write(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	rc, out := verify(t, p)
	if rc != 0 || !strings.Contains(out, "class=LIVE") {
		t.Fatalf("rc=%d %s", rc, out)
	}
	if r.Responses.StatusHistogram["200"] != 3 {
		t.Fatalf("histogram %+v", r.Responses)
	}
}

func TestDoubleCmdlineIsReclassifiedUnit(t *testing.T) {
	port, exe, _ := startRealServer(t)
	r := NewRecord("go-double", "x")
	r.SetArtifact("sha256:abc", "target")
	if err := r.ResolveAnswering(fmt.Sprintf("127.0.0.1:%d", port), fileSHA(t, exe)); err != nil {
		t.Fatal(err)
	}
	r.Answering.Cmdline += " tests/jev/fixtures/jev_server.py"
	r.Finish("PASS", []string{"a", "a", "a"})
	p, _ := r.Write(t.TempDir())
	rc, out := verify(t, p)
	if rc != 1 || !strings.Contains(out, "registered_double") {
		t.Fatalf("rc=%d %s", rc, out)
	}
}

func TestMismatchedExpectedIsUnit(t *testing.T) {
	port, _, _ := startRealServer(t)
	r := NewRecord("go-mismatch", "x")
	r.SetArtifact("sha256:abc", "target")
	if err := r.ResolveAnswering(fmt.Sprintf("127.0.0.1:%d", port), strings.Repeat("0", 64)); err != nil {
		t.Fatal(err)
	}
	r.Finish("PASS", []string{"a", "a", "a"})
	p, _ := r.Write(t.TempDir())
	rc, out := verify(t, p)
	if rc != 1 || !strings.Contains(out, "identity_mismatch") {
		t.Fatalf("rc=%d %s", rc, out)
	}
}

func TestUnresolvedAnsweringIsUnit(t *testing.T) {
	r := NewRecord("go-nobody", "x")
	r.SetArtifact("sha256:abc", "target")
	if err := r.ResolveAnswering(fmt.Sprintf("127.0.0.1:%d", freePort(t)), "abc"); err == nil {
		t.Fatal("expected resolve error")
	}
	r.Finish("PASS", []string{"a", "a", "a"})
	p, _ := r.Write(t.TempDir())
	rc, out := verify(t, p)
	if rc != 1 || !strings.Contains(out, "answering_identity_missing") {
		t.Fatalf("rc=%d %s", rc, out)
	}
}

func TestSkipClosedSet(t *testing.T) {
	r := NewRecord("go-skip", "x")
	if err := r.Skip("lazy"); err == nil {
		t.Fatal("unknown skip reason must be rejected")
	}
	if err := r.Skip("credentials_absent"); err != nil {
		t.Fatal(err)
	}
	r.Runs = Runs{N: 1, Identical: true, Signature: "sha256:0"}
	p, _ := r.Write(t.TempDir())
	rc, out := verify(t, p)
	if rc != 0 || !strings.Contains(out, "class=SKIP") {
		t.Fatalf("rc=%d %s", rc, out)
	}
}

func TestFinishIdenticalOnlyWhenAllEqual(t *testing.T) {
	r := NewRecord("go-sig", "x")
	r.Finish("PASS", []string{"a", "a", "b"})
	if r.Runs.Identical || r.Runs.N != 3 {
		t.Fatalf("%+v", r.Runs)
	}
	r.Finish("PASS", nil)
	if r.Runs.Identical {
		t.Fatal("zero runs must not be identical")
	}
}
