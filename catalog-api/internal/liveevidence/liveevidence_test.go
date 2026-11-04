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

const serverPy = `import http.server, sys, os
srv = http.server.HTTPServer(("127.0.0.1", int(sys.argv[1])), http.server.SimpleHTTPRequestHandler)
if len(sys.argv) > 2 and sys.argv[2] == "fork":
    if os.fork() == 0:
        pass  # child keeps the inherited LISTEN socket open: two pids hold it
srv.serve_forever()
`

// startRealServer starts a REAL separate process (python3 running a script file) on loopback.
func startRealServer(t *testing.T) (port int, exe string, cmd *exec.Cmd) {
	port, exe, _, cmd = startScriptServer(t, "")
	return
}

// startScriptServer writes serverPy to a temp dir and runs it; extra is an optional 2nd argument ("fork").
func startScriptServer(t *testing.T, extra string) (port int, exe, script string, cmd *exec.Cmd) {
	t.Helper()
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not present: hardware_not_present-class SKIP")
	}
	exe, _ = filepath.EvalSymlinks(py)
	port = freePort(t)
	cmd = exec.Command(py, "server.py", fmt.Sprint(port))
	if extra != "" {
		cmd.Args = append(cmd.Args, extra)
	}
	cmd.Dir = t.TempDir()
	script = filepath.Join(cmd.Dir, "server.py")
	if err := os.WriteFile(script, []byte(serverPy), 0o644); err != nil {
		t.Fatal(err)
	}
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
	port, exe, script, _ := startScriptServer(t, "")
	r := NewRecord("go-carrier", "go test ./internal/liveevidence")
	r.SetArtifact("sha256:"+fileSHA(t, script), "target")
	if err := r.ResolveAnswering(fmt.Sprintf("127.0.0.1:%d", port), fileSHA(t, exe)); err != nil {
		t.Fatal(err)
	}
	r.SetExpectedScript(fileSHA(t, script))
	if r.Answering.ScriptPath != script || r.Answering.ScriptSHA256 != fileSHA(t, script) {
		t.Fatalf("script not recorded: %+v", r.Answering)
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
	r.SetArtifact("sha256:"+strings.Repeat("a", 64), "target")
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
	r.SetArtifact("sha256:"+strings.Repeat("a", 64), "target")
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
	r.SetArtifact("sha256:"+strings.Repeat("a", 64), "target")
	if err := r.ResolveAnswering(fmt.Sprintf("127.0.0.1:%d", freePort(t)), strings.Repeat("a", 64)); err == nil {
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

// An interpreter-hosted server whose expected script is NOT named must not be LIVE (the exe hash only proves python).
func TestInterpreterWithoutExpectedScriptIsUnit(t *testing.T) {
	port, exe, script, _ := startScriptServer(t, "")
	r := NewRecord("go-interp-unbound", "x")
	r.SetArtifact("sha256:"+fileSHA(t, script), "target")
	if err := r.ResolveAnswering(fmt.Sprintf("127.0.0.1:%d", port), fileSHA(t, exe)); err != nil {
		t.Fatal(err)
	}
	r.Finish("PASS", []string{"a", "a", "a"})
	p, _ := r.Write(t.TempDir())
	rc, out := verify(t, p)
	if rc != 1 || !strings.Contains(out, "interpreter_script_unbound") {
		t.Fatalf("rc=%d %s", rc, out)
	}
}

// Fingerprint that is not the expected artifact's hash reclassifies UNIT.
func TestFingerprintMismatchIsUnit(t *testing.T) {
	port, exe, script, _ := startScriptServer(t, "")
	r := NewRecord("go-fp-mismatch", "x")
	r.SetArtifact("sha256:"+strings.Repeat("1", 64), "target")
	if err := r.ResolveAnswering(fmt.Sprintf("127.0.0.1:%d", port), fileSHA(t, exe)); err != nil {
		t.Fatal(err)
	}
	r.SetExpectedScript(fileSHA(t, script))
	r.Finish("PASS", []string{"a", "a", "a"})
	p, _ := r.Write(t.TempDir())
	rc, out := verify(t, p)
	if rc != 1 || !strings.Contains(out, "fingerprint_mismatch") {
		t.Fatalf("rc=%d %s", rc, out)
	}
}

// Two pids holding the same LISTEN socket: ambiguity is flagged, the first pid is never picked.
func TestAmbiguousListenerIsFlaggedNotPicked(t *testing.T) {
	port, _, _, _ := startScriptServer(t, "fork")
	time.Sleep(500 * time.Millisecond)
	a, err := ResolveListenerAddr("127.0.0.1", port)
	if err == nil || a != nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("want ambiguity error, got a=%+v err=%v", a, err)
	}
	r := NewRecord("go-ambiguous", "x")
	_ = r.ResolveAnswering(fmt.Sprintf("127.0.0.1:%d", port), "abc")
	if r.Answering != nil || r.ResolveError == "" {
		t.Fatalf("record must carry no answering and a resolve_error: %+v", r)
	}
}

// A socket bound to another address is not the listener of 127.0.0.1:port.
func TestBoundAddressMustCoverHost(t *testing.T) {
	port, _, _, _ := startScriptServer(t, "")
	if _, err := ResolveListenerAddr("127.0.0.1", port); err != nil {
		t.Fatalf("loopback bound listener must resolve: %v", err)
	}
	if _, err := ResolveListenerAddr("10.255.255.1", port); err == nil {
		t.Fatal("a listener bound to 127.0.0.1 must not satisfy 10.255.255.1:port")
	}
}

func TestScriptOfParsesKnownOptionsAndRefusesAmbiguity(t *testing.T) {
	d := t.TempDir()
	sc := filepath.Join(d, "real.py")
	if err := os.WriteFile(sc, []byte("print(1)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	want := fileSHA(t, sc)
	cases := []struct {
		name string
		args []string
		ok   bool
	}{
		{"plain", []string{"python3", sc}, true},
		{"-W takes a value", []string{"python3", "-W", "ignore", sc}, true},
		{"-W ignore must not be read as the script", []string{"python3", "-W", sc}, false}, // -W swallows the script path as its value => no positional
		{"-u flag", []string{"python3", "-u", sc}, true},
		{"unknown option is ambiguous", []string{"python3", "-Z", sc}, false},
		{"-m module", []string{"python3", "-m", "http.server"}, false},
		{"-c inline", []string{"python3", "-c", "print(1)"}, false},
		{"node (code-loading, must be unbound) --require=file real.js", []string{"node", "--require=./f.js", sc}, false},
		{"node (code-loading, must be unbound) --require f.js real.js", []string{"node", "--require", "./f.js", sc}, false},
		{"node unknown flag", []string{"node", "--frobnicate", sc}, false},
	}
	for _, c := range cases {
		p, h := scriptOf(0, "/usr/bin/"+c.args[0], c.args)
		if c.ok && (p != sc || h != want) {
			t.Errorf("%s: want script %s got %q %q", c.name, sc, p, h)
		}
		if !c.ok && (p != "" || h != "") {
			t.Errorf("%s: want no script, got %q", c.name, p)
		}
	}
	if p, _ := scriptOf(0, "/opt/app/server", []string{"server", sc}); p != "" {
		t.Error("non-interpreter basename must yield no script")
	}
}

func TestRuntimeHintSetForPythonServer(t *testing.T) {
	port, _, _, _ := startScriptServer(t, "")
	a, err := ResolveListenerAddr("127.0.0.1", port)
	if err != nil {
		t.Fatal(err)
	}
	if a.RuntimeHint == "" {
		// not a skip: a host that cannot produce the hint leaves the renamed-interpreter defence untested, which must be visible
		t.Fatalf("no runtime hint obtainable on this host for a running python server: %+v", a)
	}
}

// wildcard bind: a server bound to 0.0.0.0 covers every concrete host of that port, and the empty host means "any address".
func TestWildcardBindCoversConcreteHosts(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not present")
	}
	port := freePort(t)
	d := t.TempDir()
	sc := filepath.Join(d, "w.py")
	code := "import socket,sys,time\ns=socket.socket();s.setsockopt(socket.SOL_SOCKET,socket.SO_REUSEADDR,1);s.bind(('0.0.0.0',int(sys.argv[1])));s.listen(5);time.sleep(60)\n"
	if err := os.WriteFile(sc, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(py, sc, fmt.Sprint(port))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _, _ = cmd.Process.Wait() })
	var got *Answering
	for i := 0; i < 50 && got == nil; i++ {
		got, _ = ResolveListenerAddr("127.0.0.1", port)
		time.Sleep(100 * time.Millisecond)
	}
	if got == nil || got.PID != cmd.Process.Pid {
		t.Fatalf("0.0.0.0 bind must cover 127.0.0.1: %+v", got)
	}
	if a, err := ResolveListenerAddr("", port); err != nil || a.PID != cmd.Process.Pid {
		t.Fatalf("empty host means any address: %+v %v", a, err)
	}
	if a, err := ResolveListenerAddr("0.0.0.0", port); err != nil || a.PID != cmd.Process.Pid {
		t.Fatalf("wildcard host: %+v %v", a, err)
	}
}

// a host that cannot be resolved fails closed: no answering identity, never "any address"
func TestUnresolvableHostFailsClosed(t *testing.T) {
	port, _, _, _ := startScriptServer(t, "")
	if in := listenInodes("no-such-host.invalid", port); len(in) != 0 {
		t.Fatalf("unresolvable host must match nothing, got %v", in)
	}
	if a, err := ResolveListenerAddr("no-such-host.invalid", port); err == nil || a != nil {
		t.Fatalf("unresolvable host must not resolve an answering identity: %+v %v", a, err)
	}
	if in := listenInodes("", port); len(in) == 0 {
		t.Fatal("control needle: the empty host (any address) must still find the real listener")
	}
}

func TestScriptOfJarAndEqualsOptions(t *testing.T) {
	d := t.TempDir()
	jar := filepath.Join(d, "app.jar")
	js := filepath.Join(d, "s.js")
	py := filepath.Join(d, "s.py")
	for _, p := range []string{jar, js, py} {
		if err := os.WriteFile(p, []byte("x\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	sum := func(p string) string { return fileSHA(t, p) }
	type c struct {
		name, exe string
		args      []string
		path, sha string
	}
	cases := []c{
		{"java -jar", "/usr/bin/java", []string{"java", "-jar", jar}, jar, sum(jar)},
		{"java flags then -jar", "/usr/bin/java", []string{"java", "-Xmx1g", "-Dx=y", "-jar", jar, "arg"}, jar, sum(jar)},
		{"java -jar without argument", "/usr/bin/java", []string{"java", "-jar"}, "", ""},
		{"java main class is no file", "/usr/bin/java", []string{"java", "-cp", d, "com.example.Main"}, "", ""},
		{"node --opt=value script", "/usr/bin/node", []string{"node", "--max-old-space-size=4096", js}, js, sum(js)},
		{"python --opt=value script", "/usr/bin/python3", []string{"python3", "--check-hash-based-pycs=never", py}, py, sum(py)},
		{"python unknown short option", "/usr/bin/python3", []string{"python3", "-Z", py}, "", ""},
		{"python -m module", "/usr/bin/python3", []string{"python3", "-m", "http.server"}, "", ""},
	}
	for _, k := range cases {
		p, h := scriptOf(0, k.exe, k.args)
		if p != k.path || h != k.sha {
			t.Errorf("%s: got (%q,%q) want (%q,%q)", k.name, p, h, k.path, k.sha)
		}
	}
}

// the script binding must survive ResolveAnswering(.. expectedExe ..): SetExpected merges
func TestExpectedScriptSurvivesLaterSetExpected(t *testing.T) {
	r := NewRecord("merge", "x")
	r.SetExpectedScript("abc")
	r.SetExpected("def", "")
	if r.Expected == nil || r.Expected.ScriptSHA256 != "abc" || r.Expected.ExeSHA256 != "def" {
		t.Fatalf("expected must merge: %+v", r.Expected)
	}
	r2 := NewRecord("merge2", "x")
	r2.SetExpectedScript("abc")
	_ = r2.ResolveAnswering("127.0.0.1:1", "def")
	if r2.Expected == nil || r2.Expected.ScriptSHA256 != "abc" {
		t.Fatalf("ResolveAnswering dropped the script binding: %+v", r2.Expected)
	}
}

// code-loading options inject code the script hash does not cover: the script must stay unbound (UNIT), whatever form the option takes
func TestCodeLoadingOptionsLeaveScriptUnbound(t *testing.T) {
	d := t.TempDir()
	sc := filepath.Join(d, "server.js")
	if err := os.WriteFile(sc, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		exe  string
		args []string
	}{
		{"/usr/bin/node", []string{"node", "-r", "./canned.js", sc}},
		{"/usr/bin/node", []string{"node", "--require", "./canned.js", sc}},
		{"/usr/bin/node", []string{"node", "--require=./canned.js", sc}},
		{"/usr/bin/node", []string{"node", "--import", "./canned.mjs", sc}},
		{"/usr/bin/node", []string{"node", "--loader=./l.mjs", sc}},
		{"/usr/bin/node", []string{"node", "--experimental-loader", "./l.mjs", sc}},
		{"/usr/bin/ruby", []string{"ruby", "-rcanned", sc}},
		{"/usr/bin/ruby", []string{"ruby", "-r", "canned", sc}},
		{"/usr/bin/php", []string{"php", "-d", "auto_prepend_file=/x.php", sc}},
		{"/usr/bin/php", []string{"php", "-dauto_prepend_file=/x.php", sc}},
		{"/usr/bin/perl", []string{"perl", "-MCanned", sc}},
		{"/usr/bin/java", []string{"java", "-javaagent:/a.jar", "-jar", sc}},
	}
	for _, c := range cases {
		if p, h := scriptOf(0, c.exe, c.args); p != "" || h != "" {
			t.Errorf("%v: code-loading option must leave the script unbound, got (%q,%q)", c.args, p, h)
		}
	}
	// control needle: the same node call without the preload binds the script
	if p, _ := scriptOf(0, "/usr/bin/node", []string{"node", sc}); p != sc {
		t.Fatalf("control: plain node call must bind the script, got %q", p)
	}
}
