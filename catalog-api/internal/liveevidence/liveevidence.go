// Package liveevidence writes LIVE-EVIDENCE records (constitution 11.4.289(D)) for Go tests. Stdlib only.
// A test records WHAT ANSWERED (pid, executable path + sha256, or container image digest), the artifact fingerprint read from the target,
// request/response status histograms and the N-run signature; scripts/test-infra/verify_live_evidence.sh then reclassifies any record whose
// answering identity is a registered double, mismatched or missing as UNIT-CLASS. Schema: docs/qa/live-evidence.schema.json.
package liveevidence

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"sort"
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
	// EnvChecked: /proc/<pid>/environ was read for the closed key set InterpTable.EnvKeys; EnvInjected lists the keys that were non-empty (NODE_OPTIONS ...).
	EnvChecked  bool     `json:"env_checked,omitempty"`
	EnvInjected []string `json:"env_injected,omitempty"`
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
	if r.Expected == nil {
		r.Expected = &Expected{}
	}
	// merge, never replace: a script binding set earlier (SetExpectedScript) must survive a later SetExpected/ResolveAnswering
	if exeSHA256 != "" {
		r.Expected.ExeSHA256 = exeSHA256
	}
	if imageDigest != "" {
		r.Expected.ImageDigest = imageDigest
	}
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

// hostIPs resolves the host of the claimed endpoint. ok=false means the host is named but cannot be resolved: callers must FAIL CLOSED
// (match nothing), never treat it like "any address". An empty host is the explicit "any address" request.
func hostIPs(host string) (ips []net.IP, ok bool) {
	if host == "" {
		return nil, true
	}
	if ip := net.ParseIP(host); ip != nil {
		return []net.IP{ip}, true
	}
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return nil, false
	}
	return ips, true
}

// listenInodes returns the inodes of LISTEN sockets on port whose bound address covers host (a socket bound to a wildcard address covers every host;
// a socket bound to another address does NOT, so a listener on 10.0.0.5:port is never reported for 127.0.0.1:port).
func listenInodes(host string, port int) map[string]bool {
	in := map[string]bool{}
	want, ok := hostIPs(host)
	if !ok {
		return in // unresolvable host: nothing can be shown to answer it
	}
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
			covers := bound != nil && bound.IsUnspecified() || host == ""
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

// The ALLOW-LIST of interpreter command-line options is DATA shared with the verifier and the Node recorder (docs/qa/live-evidence.interpreter-options.json;
// interp_options.json here is its embedded copy, byte-parity tested). A "--name=value" token is split on the first "=" BEFORE any lookup; an option that is
// not listed as benign for the exe family is ambiguous: no script is recorded and the verifier reports UNIT. deno/bun are unsupported (no table).
// Limit: the same effect through the environment (NODE_OPTIONS, RUBYOPT, PYTHONSTARTUP, JAVA_TOOL_OPTIONS, PHP_INI_SCAN_DIR) is NOT visible in the command line.

//go:embed interp_options.json
var interpOptionsJSON []byte

type optEntry struct {
	Opt    string `json:"opt"`
	Val    string `json:"val"`
	Re     string `json:"re"`
	Ex     string `json:"ex"`
	Script bool   `json:"script"`
}
type famTable struct {
	Exe              string     `json:"exe"`
	Benign           []optEntry `json:"benign"`
	Dangerous        []string   `json:"dangerous"`
	PositionalScript *bool      `json:"positional_script"`
}

// InterpTable is the parsed option table.
type InterpTable struct {
	UnsupportedExe string              `json:"unsupported_exe"`
	EnvKeys        []string            `json:"env_keys"`
	EnvPrefixes    []string            `json:"env_prefixes"`
	EnvLoader      []string            `json:"env_loader"`
	Families       map[string]famTable `json:"families"`
}

// ParseInterpTable parses and shape-checks an option table (an empty or malformed table is an error, never "everything allowed").
func ParseInterpTable(b []byte) (*InterpTable, error) {
	var t InterpTable
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, err
	}
	if len(t.Families) == 0 || t.UnsupportedExe == "" {
		return nil, errors.New("interpreter option table empty or malformed")
	}
	for n, f := range t.Families {
		if _, err := regexp.Compile("(?i)" + f.Exe); err != nil || f.Exe == "" {
			return nil, fmt.Errorf("family %s: bad exe regex", n)
		}
		for _, e := range f.Benign {
			if e.Opt == "" || (e.Val != "none" && e.Val != "next" && e.Val != "eq" && e.Val != "attached") {
				return nil, fmt.Errorf("family %s: bad entry %q", n, e.Opt)
			}
			if e.Re != "" {
				if _, err := regexp.Compile(e.Re); err != nil {
					return nil, fmt.Errorf("family %s: bad re for %q", n, e.Opt)
				}
			}
			if e.Val == "attached" && e.Re == "" {
				return nil, fmt.Errorf("family %s: attached %q needs a value regex", n, e.Opt)
			}
		}
	}
	return &t, nil
}

func mustTable(b []byte) *InterpTable {
	t, err := ParseInterpTable(b)
	if err != nil {
		panic("liveevidence: embedded interpreter option table: " + err.Error())
	}
	return t
}

var defaultInterpTable = mustTable(interpOptionsJSON)

var ctlRE = regexp.MustCompile("[\\x00-\\x1f\\x7f]")

func (t *InterpTable) family(base string) *famTable {
	names := make([]string, 0, len(t.Families))
	for n := range t.Families {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		f := t.Families[n]
		if regexp.MustCompile("(?i)" + f.Exe).MatchString(base) {
			return &f
		}
	}
	return nil
}

func (f *famTable) danger(tok string) bool {
	n := tok
	if strings.HasPrefix(tok, "--") && strings.Contains(tok, "=") {
		n = strings.SplitN(tok, "=", 2)[0]
	}
	for _, d := range f.Dangerous {
		if strings.HasPrefix(d, "prefix:") {
			if strings.HasPrefix(tok, strings.TrimPrefix(d, "prefix:")) {
				return true
			}
		} else if d == n {
			return true
		}
	}
	return false
}

func reOK(e optEntry, v string) bool { return e.Re == "" || regexp.MustCompile(e.Re).MatchString(v) }

// ParseInterpArgs applies the allow-list to an interpreter's arguments (args[0] is argv0). st is "ok", "codeload", "unknown" or "unsupported";
// script is the file the interpreter runs ("" when none could be determined: a main class, a subcommand, or an ambiguity).
func ParseInterpArgs(t *InterpTable, exe string, args []string) (st, script string) {
	base := filepath.Base(exe)
	f := t.family(base)
	if f == nil && regexp.MustCompile("(?i)"+t.UnsupportedExe).MatchString(base) {
		return "unsupported", ""
	}
	fam := &famTable{}
	if f != nil {
		fam = f
	}
	var toks []string
	if len(args) > 1 {
		toks = args[1:]
	}
	for i := 0; i < len(toks); {
		x := toks[i]
		if ctlRE.MatchString(x) {
			return "unknown", ""
		}
		if !strings.HasPrefix(x, "-") && !strings.HasPrefix(x, "+") {
			if fam.PositionalScript != nil && !*fam.PositionalScript {
				return "ok", ""
			}
			return "ok", x
		}
		if fam.danger(x) {
			return "codeload", ""
		}
		has := strings.HasPrefix(x, "--") && strings.Contains(x, "=")
		n, v := x, ""
		if has {
			p := strings.SplitN(x, "=", 2)
			n, v = p[0], p[1]
		}
		var e *optEntry
		for k := range fam.Benign {
			if fam.Benign[k].Opt == n {
				e = &fam.Benign[k]
				break
			}
		}
		if e == nil {
			ok := false
			for _, b := range fam.Benign {
				if b.Val == "attached" && strings.HasPrefix(x, b.Opt) && regexp.MustCompile(b.Re).MatchString(strings.TrimPrefix(x, b.Opt)) {
					ok = true
					break
				}
			}
			if !ok {
				return "unknown", ""
			}
			i++
			continue
		}
		switch e.Val {
		case "none":
			if has {
				return "unknown", ""
			}
			i++
		case "eq":
			if !has || !reOK(*e, v) {
				return "unknown", ""
			}
			i++
		case "next":
			if has {
				if !reOK(*e, v) || e.Script {
					return "unknown", ""
				}
				i++
			} else if i+1 < len(toks) && !strings.HasPrefix(toks[i+1], "-") && reOK(*e, toks[i+1]) {
				if e.Script {
					return "ok", toks[i+1]
				}
				i += 2
			} else {
				return "unknown", ""
			}
		default:
			return "unknown", ""
		}
	}
	return "ok", ""
}

// scriptOf returns the script file an interpreter process runs, or "" when the allow-list cannot vouch for the command line (unknown option, inline code,
// code-loading option, main class, unsupported or renamed interpreter). Relative paths are resolved against the process cwd.
func scriptOf(pid int, exe string, args []string) (string, string) {
	if defaultInterpTable.family(filepath.Base(exe)) == nil {
		return "", "" // renamed / unknown binary: nothing is guessed
	}
	st, cand := ParseInterpArgs(defaultInterpTable, exe, args)
	if st != "ok" || cand == "" {
		return "", ""
	}
	if !filepath.IsAbs(cand) {
		return "", "" // cooperative-producer binding: only an ABSOLUTE script token binds; a relative one is never resolved against a guessed cwd
	}
	sum, err := FileSHA256(cand)
	if err != nil {
		return "", ""
	}
	return cand, sum
}

// hasEnvPrefix: k starts with one of the closed prefixes and is longer than it (the bare prefix is not a key).
func hasEnvPrefix(prefixes []string, k string) bool {
	for _, p := range prefixes {
		if p != "" && len(k) > len(p) && strings.HasPrefix(k, p) {
			return true
		}
	}
	return false
}

// EnvOf reads the closed key set t.EnvKeys from environ (the content of /proc/<pid>/environ). checked is false when environ is nil (unreadable): never guessed clean.
func EnvOf(t *InterpTable, environ []byte, readErr error) (checked bool, injected []string) {
	if readErr != nil {
		return false, nil
	}
	keys := map[string]bool{}
	for _, k := range t.EnvKeys {
		keys[k] = true
	}
	seen := map[string]bool{}
	for _, kv := range strings.Split(string(environ), "\x00") {
		i := strings.Index(kv, "=")
		if i > 0 && kv[i+1:] != "" && !seen[kv[:i]] && (keys[kv[:i]] || hasEnvPrefix(t.EnvPrefixes, kv[:i])) {
			seen[kv[:i]] = true
			injected = append(injected, kv[:i])
		}
	}
	sort.Strings(injected)
	return true, injected
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
	env, envErr := os.ReadFile(fmt.Sprintf("/proc/%d/environ", pid))
	ec, ei := EnvOf(defaultInterpTable, env, envErr)
	return &Answering{EnvChecked: ec, EnvInjected: ei, Kind: "process", PID: pid, ExePath: exe, ExeSHA256: sum, Cmdline: strings.Join(args, " "), ScriptPath: sp, ScriptSHA256: ss, RuntimeHint: runtimeHint(pid)}, nil
}
