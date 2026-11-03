// Package liveevidence writes LIVE-EVIDENCE records (constitution 11.4.289(D)) for Go tests. Stdlib only.
// A test records WHAT ANSWERED (pid, executable path + sha256, or container image digest), the artifact fingerprint read from the target,
// request/response status histograms and the N-run signature; scripts/test-infra/verify_live_evidence.sh then reclassifies any record whose
// answering identity is a registered double, mismatched or missing as UNIT-CLASS. Schema: docs/qa/live-evidence.schema.json.
package liveevidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const Schema = "catalogizer.live-evidence/1"

// SkipReasons is the closed set of honest SKIP reasons (11.4.69).
var SkipReasons = []string{"hardware_not_present", "credentials_absent", "topology_unsupported", "operator_attended", "structurally_impossible", "feature_disabled_by_config", "artifact_not_yet_built"}

type Artifact struct {
	Fingerprint string `json:"fingerprint,omitempty"`
	Source      string `json:"source,omitempty"`
}
type Endpoint struct {
	Host       string `json:"host,omitempty"`
	Port       int    `json:"port,omitempty"`
	ResolvedIP string `json:"resolved_ip,omitempty"`
	URL        string `json:"url,omitempty"`
}
type Answering struct {
	Kind        string `json:"kind"`
	PID         int    `json:"pid,omitempty"`
	ExePath     string `json:"exe_path,omitempty"`
	ExeSHA256   string `json:"exe_sha256,omitempty"`
	Cmdline     string `json:"cmdline,omitempty"`
	Image       string `json:"image,omitempty"`
	ImageDigest string `json:"image_digest,omitempty"`
	ContainerID string `json:"container_id,omitempty"`
	// ScriptPath/ScriptSHA256: for interpreter-hosted servers (python, node, java ...) the executable hash only proves the interpreter;
	// the script that actually answers is recorded here (heuristic: first non-option argument; never for -c/-e/-m code, which stays unbound).
	ScriptPath   string `json:"script_path,omitempty"`
	ScriptSHA256 string `json:"script_sha256,omitempty"`
	// RuntimeHint: a language-runtime library mapped into the process (libpython, libjvm ...): marks an interpreter even when the binary was renamed.
	// Statically linked runtimes (node, deno, bun) expose none: a renamed copy of those is not detected (documented limit).
	RuntimeHint string `json:"runtime_hint,omitempty"`
}
type Expected struct {
	ExeSHA256    string `json:"exe_sha256,omitempty"`
	ImageDigest  string `json:"image_digest,omitempty"`
	ScriptSHA256 string `json:"script_sha256,omitempty"`
}
type Requests struct {
	Total   int            `json:"total"`
	Methods map[string]int `json:"methods"`
}
type Responses struct {
	StatusHistogram map[string]int `json:"status_histogram"`
}
type Runs struct {
	N         int    `json:"n"`
	Identical bool   `json:"identical"`
	Signature string `json:"signature"`
}

type Record struct {
	Schema     string     `json:"schema"`
	TestID     string     `json:"test_id"`
	Class      string     `json:"class"`
	Verdict    string     `json:"verdict"`
	SkipReason string     `json:"skip_reason,omitempty"`
	Timestamp  string     `json:"timestamp"`
	Command    string     `json:"command"`
	Artifact   Artifact   `json:"artifact"`
	Endpoint   Endpoint   `json:"endpoint"`
	Answering  *Answering `json:"answering"`
	Expected   *Expected  `json:"expected"`
	Requests   Requests   `json:"requests"`
	Responses  Responses  `json:"responses"`
	Runs       Runs       `json:"runs"`
	// ResolveError is set when the listener could not be resolved or was ambiguous (several pids hold the same LISTEN socket); the verifier then reclassifies UNIT.
	ResolveError string `json:"resolve_error,omitempty"`
}

// NewRecord starts a record claiming class LIVE with verdict FAIL until Finish/Skip says otherwise.
func NewRecord(testID, command string) *Record {
	return &Record{Schema: Schema, TestID: testID, Class: "LIVE", Verdict: "FAIL", Timestamp: time.Now().UTC().Format(time.RFC3339), Command: command,
		Requests: Requests{Methods: map[string]int{}}, Responses: Responses{StatusHistogram: map[string]int{}}}
}

// SetArtifact records the fingerprint READ FROM THE TARGET at run time.
func (r *Record) SetArtifact(fingerprint, source string) {
	r.Artifact = Artifact{Fingerprint: fingerprint, Source: source}
}

// Observe counts one request/response pair.
func (r *Record) Observe(method string, status int) {
	r.Requests.Total++
	r.Requests.Methods[method]++
	r.Responses.StatusHistogram[strconv.Itoa(status)]++
}

// SetAnsweringContainer records a container as the answering identity (image digest read from the runtime).
func (r *Record) SetAnsweringContainer(image, imageDigest, containerID string) {
	r.Answering = &Answering{Kind: "container", Image: image, ImageDigest: imageDigest, ContainerID: containerID}
}

// SetExpected names the real artifact the claim is about (executable sha256 or image digest).
func (r *Record) SetExpected(exeSHA256, imageDigest string) {
	r.Expected = &Expected{ExeSHA256: exeSHA256, ImageDigest: imageDigest}
}

// SetExpectedScript names the real script an interpreter-hosted server must be running (sha256 hex of the script file).
func (r *Record) SetExpectedScript(scriptSHA256 string) {
	if r.Expected == nil {
		r.Expected = &Expected{}
	}
	r.Expected.ScriptSHA256 = scriptSHA256
}

// ResolveAnswering resolves host:port to the LISTENing process. On failure Answering stays nil (the verifier then reclassifies the record UNIT).
func (r *Record) ResolveAnswering(hostport, expectedExeSHA256 string) error {
	host, ps, err := net.SplitHostPort(hostport)
	if err != nil {
		return err
	}
	port, err := strconv.Atoi(ps)
	if err != nil {
		return err
	}
	r.Endpoint = Endpoint{Host: host, Port: port, ResolvedIP: host}
	if expectedExeSHA256 != "" {
		r.SetExpected(expectedExeSHA256, "")
	}
	a, err := ResolveListenerAddr(host, port)
	if err != nil {
		r.Answering = nil
		r.ResolveError = err.Error()
		return err
	}
	r.Answering = a
	return nil
}

// Finish sets the verdict and the N-run signature: one canonical verdict string per run, identical iff all runs agree (11.4.50).
func (r *Record) Finish(verdict string, signatures []string) {
	r.Verdict = verdict
	ident := len(signatures) > 0
	for _, s := range signatures {
		if s != signatures[0] {
			ident = false
		}
	}
	h := sha256.Sum256([]byte(strings.Join(signatures, "\n")))
	r.Runs = Runs{N: len(signatures), Identical: ident, Signature: "sha256:" + hex.EncodeToString(h[:])}
}

// Skip records an honest SKIP with a reason from the closed set.
func (r *Record) Skip(reason string) error {
	for _, ok := range SkipReasons {
		if ok == reason {
			r.Verdict, r.SkipReason, r.Answering, r.Expected = "SKIP", reason, nil, nil
			return nil
		}
	}
	return fmt.Errorf("skip reason %q not in closed set", reason)
}

var unsafeName = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// Write stores the record as <dir>/<test_id>.json and returns the path.
func (r *Record) Write(dir string) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	p := filepath.Join(dir, unsafeName.ReplaceAllString(r.TestID, "_")+".json")
	return p, os.WriteFile(p, append(b, '\n'), 0o644)
}

// FileSHA256 returns the hex sha256 of a file.
func FileSHA256(p string) (string, error) {
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), nil
}

// procAddr decodes a /proc/net/tcp{,6} hex local address ("0100007F" or 32 hex chars, each 32-bit word little-endian).
func procAddr(h string) net.IP {
	b, err := hex.DecodeString(h)
	if err != nil || (len(b) != 4 && len(b) != 16) {
		return nil
	}
	for w := 0; w+4 <= len(b); w += 4 {
		b[w], b[w+1], b[w+2], b[w+3] = b[w+3], b[w+2], b[w+1], b[w]
	}
	return net.IP(b)
}

// hostIPs resolves the host of the claimed endpoint; "" or a wildcard host matches every bound address.
func hostIPs(host string) []net.IP {
	if host == "" {
		return nil
	}
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}
	}
	ips, _ := net.LookupIP(host)
	return ips
}

// listenInodes returns the inodes of LISTEN sockets on port whose bound address covers host (a socket bound to a wildcard address covers every host;
// a socket bound to another address does NOT, so a listener on 10.0.0.5:port is never reported for 127.0.0.1:port).
func listenInodes(host string, port int) map[string]bool {
	in := map[string]bool{}
	want := hostIPs(host)
	for _, f := range []string{"/proc/net/tcp", "/proc/net/tcp6"} {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, line := range strings.Split(string(b), "\n")[1:] {
			c := strings.Fields(line)
			if len(c) < 10 || c[3] != "0A" {
				continue
			}
			lp := strings.SplitN(c[1], ":", 2)
			if len(lp) != 2 {
				continue
			}
			p, err := strconv.ParseInt(lp[1], 16, 32)
			if err != nil || int(p) != port {
				continue
			}
			bound := procAddr(lp[0])
			covers := bound != nil && bound.IsUnspecified() || want == nil
			for _, w := range want {
				if bound != nil && bound.Equal(w) {
					covers = true
				}
			}
			if covers {
				in[c[9]] = true
			}
		}
	}
	return in
}

var interpreterRE = regexp.MustCompile(`(?i)^(python[0-9.]*|pypy[0-9.]*|node|nodejs|deno|bun|java|ruby[0-9.]*|perl[0-9.]*|php[0-9.]*|lua[0-9.]*|bash|sh|dash|zsh|ksh|fish)$`)
var runtimeLibRE = regexp.MustCompile(`(libpython[0-9.]*|libjvm|libruby[0-9.]*|libperl|libphp[0-9.]*|liblua[0-9.]*|libnode|libv8|/lib-dynload/|/site-packages/|/node_modules/)`)

// interpOpts: per interpreter family, options that take no value and options that take the NEXT argument as value, plus inline-code options.
// An option that is in none of these tables makes the script ambiguous: the record then carries no script and the verifier reports UNIT.
type interpOpts struct{ noval, val, inline map[string]bool }

func set(xs ...string) map[string]bool {
	m := map[string]bool{}
	for _, x := range xs {
		m[x] = true
	}
	return m
}

var interpTable = map[string]interpOpts{
	"python": {set("-u", "-B", "-E", "-s", "-S", "-I", "-O", "-OO", "-v", "-vv", "-b", "-bb", "-q", "-d", "-R", "-x", "-P"), set("-W", "-X"), set("-c", "-m")},
	"node":   {set("--no-warnings", "--trace-warnings", "--expose-gc", "--enable-source-maps", "--experimental-vm-modules", "--harmony", "--use-strict", "--preserve-symlinks"), set("-r", "--require", "--import", "--loader", "--experimental-loader", "--conditions", "-C", "--input-type", "--title"), set("-e", "-p", "--eval", "--print")},
	"java":   {set("-server", "-ea", "-da", "-verbose"), set("-cp", "-classpath", "--class-path", "-p", "--module-path"), set()},
	"ruby":   {set("-w", "-W0", "-W1", "-W2", "-v"), set("-I", "-r"), set("-e")},
	"perl":   {set("-w", "-W", "-t", "-T", "-l"), set("-I"), set("-e", "-E")},
	"php":    {set("-q", "-n"), set("-d", "-c"), set("-r")},
	"lua":    {set("-W", "-i"), set("-l"), set("-e")},
	"shell":  {set("-x", "-e", "-u", "-v", "-eu", "-ex", "-eux", "-l"), set(), set("-c")},
}

func interpFamily(base string) string {
	b := strings.ToLower(base)
	switch {
	case strings.HasPrefix(b, "python"), strings.HasPrefix(b, "pypy"):
		return "python"
	case b == "node", b == "nodejs":
		return "node"
	case b == "bash", b == "sh", b == "dash", b == "zsh", b == "ksh", b == "fish":
		return "shell"
	}
	for _, f := range []string{"java", "ruby", "perl", "php", "lua"} {
		if strings.HasPrefix(b, f) {
			return f
		}
	}
	return "" // deno, bun and unknown: no table => nothing is guessed
}

// scriptOf returns the script file an interpreter process runs. Options are parsed with the per-family tables above; an unknown option, inline code
// (-c, -e, -m ...) or a non-file first positional (module, main class, subcommand) yields "" so the verifier reports interpreter_script_missing.
// For java, -jar's argument is the artifact. Relative paths are resolved against the process cwd.
func scriptOf(pid int, exe string, args []string) (string, string) {
	fam := interpFamily(filepath.Base(exe))
	t, ok := interpTable[fam]
	if !ok {
		return "", ""
	}
	cand := ""
	for i := 1; i < len(args) && cand == ""; i++ {
		a := args[i]
		switch {
		case t.inline[a]:
			return "", ""
		case fam == "java" && a == "-jar":
			if i+1 >= len(args) {
				return "", ""
			}
			cand = args[i+1]
		case t.val[a]:
			i++
		case t.noval[a], strings.HasPrefix(a, "--") && strings.Contains(a, "="):
		case fam == "java" && (strings.HasPrefix(a, "-X") || strings.HasPrefix(a, "-D")):
		case strings.HasPrefix(a, "-"):
			return "", "" // unknown option: ambiguous, never guessed
		default:
			cand = a
		}
	}
	if cand == "" {
		return "", ""
	}
	if !filepath.IsAbs(cand) {
		if cwd, err := os.Readlink(fmt.Sprintf("/proc/%d/cwd", pid)); err == nil {
			cand = filepath.Join(cwd, cand)
		}
	}
	sum, err := FileSHA256(cand)
	if err != nil {
		return "", ""
	}
	return cand, sum
}

// runtimeHint names a language-runtime library mapped into the process (python, jvm, ...), which marks an interpreter even when the binary was renamed.
func runtimeHint(pid int) string {
	b, err := os.ReadFile(fmt.Sprintf("/proc/%d/maps", pid))
	if err != nil {
		return ""
	}
	return runtimeLibRE.FindString(string(b))
}

// ResolveListener finds the process holding the LISTEN socket on port on any address (same-user processes only; never guessed).
func ResolveListener(port int) (*Answering, error) { return ResolveListenerAddr("", port) }

// ResolveListenerAddr finds the process holding the LISTEN socket on host:port. If more than one distinct pid holds a matching socket the
// result is ambiguous: an error naming the pids is returned and NO answering identity (the first pid is never picked).
func ResolveListenerAddr(host string, port int) (*Answering, error) {
	inodes := listenInodes(host, port)
	if len(inodes) == 0 {
		return nil, fmt.Errorf("nothing listens on %s:%d", host, port)
	}
	procs, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	var holders []int
	for _, d := range procs {
		pid, err := strconv.Atoi(d.Name())
		if err != nil {
			continue
		}
		fds, err := os.ReadDir(fmt.Sprintf("/proc/%d/fd", pid))
		if err != nil {
			continue
		}
		for _, fd := range fds {
			l, err := os.Readlink(fmt.Sprintf("/proc/%d/fd/%s", pid, fd.Name()))
			if err != nil || !strings.HasPrefix(l, "socket:[") {
				continue
			}
			if inodes[strings.TrimSuffix(strings.TrimPrefix(l, "socket:["), "]")] {
				holders = append(holders, pid)
				break
			}
		}
	}
	switch len(holders) {
	case 0:
		return nil, errors.New("listener process not found among readable processes")
	case 1:
	default:
		return nil, fmt.Errorf("ambiguous listener: pids %v hold the same LISTEN socket (or several sockets match); not choosing one", holders)
	}
	pid := holders[0]
	exe, err := filepath.EvalSymlinks(fmt.Sprintf("/proc/%d/exe", pid))
	if err != nil {
		return nil, fmt.Errorf("pid %d exe unreadable: %w", pid, err)
	}
	sum, err := FileSHA256(exe)
	if err != nil {
		return nil, err
	}
	cl, _ := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	args := strings.FieldsFunc(string(cl), func(r rune) bool { return r == 0 })
	sp, ss := scriptOf(pid, exe, args)
	return &Answering{Kind: "process", PID: pid, ExePath: exe, ExeSHA256: sum, Cmdline: strings.Join(args, " "), ScriptPath: sp, ScriptSHA256: ss, RuntimeHint: runtimeHint(pid)}, nil
}
