package liveevidence

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type optCase struct {
	Member string   `json:"member"`
	Exe    string   `json:"exe"`
	Args   []string `json:"args"`
	Want   string   `json:"want"`
	Script *string  `json:"script"`
}

func loadCases(t *testing.T) []optCase {
	t.Helper()
	b, err := os.ReadFile("../../../tests/live-evidence/option_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cs []optCase
	if err := json.Unmarshal(b, &cs); err != nil {
		t.Fatal(err)
	}
	if len(cs) < 100 {
		t.Fatalf("frozen case set suspiciously small: %d", len(cs))
	}
	return cs
}

// failures runs every frozen case against a table; returns the failing case members.
func failures(tbl *InterpTable, cs []optCase, only func(string) bool) []string {
	var bad []string
	for _, c := range cs {
		if only != nil && !only(c.Member) {
			continue
		}
		st, sc := ParseInterpArgs(tbl, c.Exe, c.Args)
		wantSc := ""
		if c.Script != nil {
			wantSc = *c.Script
		}
		if st != c.Want || (c.Want == "ok" && sc != wantSc) {
			bad = append(bad, c.Member)
		}
	}
	return bad
}

// the embedded table must be byte-identical to the canonical docs/qa table the verifier and the Node recorder read
func TestInterpTableParityWithDocsCopy(t *testing.T) {
	d, err := os.ReadFile("../../../docs/qa/live-evidence.interpreter-options.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(d, interpOptionsJSON) {
		t.Fatal("catalog-api/internal/liveevidence/interp_options.json differs from docs/qa/live-evidence.interpreter-options.json")
	}
}

func TestFrozenOptionCases(t *testing.T) {
	cs := loadCases(t)
	if bad := failures(defaultInterpTable, cs, nil); len(bad) > 0 {
		t.Fatalf("%d frozen option cases fail: %v", len(bad), bad)
	}
}

// every member of the real table is pinned by at least one frozen case (adding a member without a case must fail)
func TestEveryTableMemberHasACase(t *testing.T) {
	cs := loadCases(t)
	pinned := func(name string) bool {
		for _, c := range cs {
			if c.Member == name || strings.HasPrefix(c.Member, name+":") {
				return true
			}
		}
		return false
	}
	for fam, f := range defaultInterpTable.Families {
		for _, e := range f.Benign {
			if !pinned(fmt.Sprintf("%s:benign:%s", fam, e.Opt)) {
				t.Errorf("no case pins %s benign %s", fam, e.Opt)
			}
		}
		for _, d := range f.Dangerous {
			if !pinned(fmt.Sprintf("%s:dangerous:%s", fam, d)) {
				t.Errorf("no case pins %s dangerous %s", fam, d)
			}
		}
	}
}

func cloneTable(t *testing.T) *InterpTable {
	t.Helper()
	c, err := ParseInterpTable(interpOptionsJSON)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// reviewer-style mutants of the table, ONE MEMBER AT A TIME: removing a benign member, removing a dangerous member, or moving a dangerous member into
// the benign list must each be caught by the frozen cases of exactly that member. A member whose mutant survives is not pinned.
func TestEveryTableMemberMutantIsKilled(t *testing.T) {
	cs := loadCases(t)
	mutants, killed := 0, 0
	check := func(name string, tbl *InterpTable, member string) {
		mutants++
		bad := failures(tbl, cs, func(m string) bool { return m == member || strings.HasPrefix(m, member+":") })
		if len(bad) == 0 {
			t.Errorf("mutant survived: %s", name)
			return
		}
		killed++
	}
	for fam, f := range defaultInterpTable.Families {
		for i, e := range f.Benign {
			m := cloneTable(t)
			ff := m.Families[fam]
			ff.Benign = append(append([]optEntry{}, ff.Benign[:i]...), ff.Benign[i+1:]...)
			m.Families[fam] = ff
			check("remove benign "+fam+" "+e.Opt, m, fmt.Sprintf("%s:benign:%s", fam, e.Opt))
		}
		for i, d := range f.Dangerous {
			m := cloneTable(t)
			ff := m.Families[fam]
			ff.Dangerous = append(append([]string{}, ff.Dangerous[:i]...), ff.Dangerous[i+1:]...)
			m.Families[fam] = ff
			check("remove dangerous "+fam+" "+d, m, fmt.Sprintf("%s:dangerous:%s", fam, d))
			if !strings.HasPrefix(d, "prefix:") {
				m2 := cloneTable(t)
				ff2 := m2.Families[fam]
				ff2.Dangerous = append(append([]string{}, ff2.Dangerous[:i]...), ff2.Dangerous[i+1:]...)
				ff2.Benign = append(append([]optEntry{}, ff2.Benign...), optEntry{Opt: d, Val: "none"})
				m2.Families[fam] = ff2
				check("move dangerous to benign "+fam+" "+d, m2, fmt.Sprintf("%s:dangerous:%s", fam, d))
			}
		}
	}
	t.Logf("table-member mutants: %d, killed %d", mutants, killed)
	if mutants < 150 || killed != mutants {
		t.Fatalf("mutants=%d killed=%d", mutants, killed)
	}
}

// control needles: plain interpreter calls stay bound, the reviewer's canned-server shapes do not
func TestReviewShapesAreNotBound(t *testing.T) {
	if st, sc := ParseInterpArgs(defaultInterpTable, "/usr/bin/node", []string{"node", "server.js"}); st != "ok" || sc != "server.js" {
		t.Fatalf("control: plain node call, got %s %q", st, sc)
	}
	for _, a := range [][]string{
		{"node", "--eval=require('http').createServer((a, b) => {})", "real.js"},
		{"node", "--env-file=c2.env", "real.js"},
		{"node", "--env-file", "c2.env", "real.js"},
		{"node", "-e", "x", "real.js"},
	} {
		if st, _ := ParseInterpArgs(defaultInterpTable, "/usr/bin/node", a); st == "ok" {
			t.Errorf("%v must not be bound", a)
		}
	}
	for _, c := range []struct {
		exe  string
		args []string
	}{
		{"/usr/bin/java", []string{"java", "--patch-module=java.base=/tmp/c", "-jar", "real.jar"}},
		{"/usr/bin/php", []string{"php", "-c", "canned.ini", "real.php"}},
		{"/usr/bin/python3", []string{"python3", "-c", "x", "real.py"}},
		{"/usr/bin/python3", []string{"python3", "-m", "http.server"}},
		{"/usr/bin/bun", []string{"bun", "--preload", "c.ts", "real.ts"}},
		{"/usr/bin/deno", []string{"deno", "run", "--preload=.", "real.ts"}},
	} {
		if st, _ := ParseInterpArgs(defaultInterpTable, c.exe, c.args); st == "ok" {
			t.Errorf("%v must not be bound", c.args)
		}
	}
}

func TestMalformedTableIsRefused(t *testing.T) {
	for _, b := range []string{`{}`, `{"unsupported_exe":"x","families":{}}`, `not json`, `{"unsupported_exe":"x","families":{"a":{"exe":"(","benign":[],"dangerous":[]}}}`,
		`{"unsupported_exe":"x","families":{"a":{"exe":"a","benign":[{"opt":"-q","val":"weird"}],"dangerous":[]}}}`,
		`{"unsupported_exe":"x","families":{"a":{"exe":"a","benign":[{"opt":"-Xq","val":"attached"}],"dangerous":[]}}}`} {
		if _, err := ParseInterpTable([]byte(b)); err == nil {
			t.Errorf("table %q must be refused", b)
		}
	}
}

// synthetic tables: entry shapes the real table does not (yet) exercise must still be refused by the code itself, not by luck of the data
func TestSyntheticTable(t *testing.T) {
	tbl, err := ParseInterpTable([]byte(`{"unsupported_exe":"^zz$","families":{"x":{"exe":"^x$","benign":[
	 {"opt":"--eqnore","val":"eq"},{"opt":"--nx","val":"next"},{"opt":"--jar","val":"next","script":true},{"opt":"-n","val":"none"}],"dangerous":[]}}}`))
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) (string, string) {
		return ParseInterpArgs(tbl, "/usr/bin/x", append([]string{"x"}, args...))
	}
	for _, c := range []struct {
		args     []string
		st, scr  string
		describe string
	}{
		{[]string{"--eqnore=5", "s"}, "ok", "s", "eq with =value and no regex is accepted"},
		{[]string{"--eqnore", "5", "s"}, "unknown", "", "eq without =value is refused even without a regex"},
		{[]string{"--nx=v", "s"}, "ok", "s", "next accepts the = form"},
		{[]string{"--nx", "v", "s"}, "ok", "s", "next accepts the space form"},
		{[]string{"--nx", "-v", "s"}, "unknown", "", "next refuses a dash-led value even without a regex"},
		{[]string{"--jar=a.jar"}, "unknown", "", "a script-binding option never takes the = form"},
		{[]string{"--jar", "a.jar"}, "ok", "a.jar", "a script-binding option binds its value"},
		{[]string{"-n", "s"}, "ok", "s", "control: plain none option"},
	} {
		if st, scr := run(c.args...); st != c.st || scr != c.scr {
			t.Errorf("%s: %v got (%s,%q) want (%s,%q)", c.describe, c.args, st, scr, c.st, c.scr)
		}
	}
}

// scriptOf binds a plain interpreter call to the real file, never a renamed / unsupported binary, never an unlisted option
func TestScriptOfBinding(t *testing.T) {
	d := t.TempDir()
	sc := d + "/server.js"
	if err := os.WriteFile(sc, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if p, h := scriptOf(0, "/usr/bin/node", []string{"node", sc}); p != sc || h == "" {
		t.Fatalf("control: plain node call must bind, got %q %q", p, h)
	}
	for _, c := range []struct {
		exe  string
		args []string
	}{
		{"/opt/app/server", []string{"server", sc}},
		{"/usr/bin/deno", []string{"deno", "run", sc}},
		{"/usr/bin/node", []string{"node", "--env-file=c2.env", sc}},
		{"/usr/bin/node", []string{"node", "--eval=x", sc}},
	} {
		if p, h := scriptOf(0, c.exe, c.args); p != "" || h != "" {
			t.Errorf("%v must stay unbound, got %q %q", c.args, p, h)
		}
	}
}

func TestEnvOfEveryKey(t *testing.T) {
	want := []string{"NODE_OPTIONS", "NODE_PATH", "JDK_JAVA_OPTIONS", "JAVA_TOOL_OPTIONS", "_JAVA_OPTIONS", "RUBYOPT", "RUBYLIB", "GEM_HOME", "GEM_PATH", "BUNDLE_GEMFILE", "BUNDLE_PATH", "PERL5OPT", "PERL5LIB", "PERLLIB", "PYTHONPATH", "PYTHONSTARTUP", "PYTHONHOME", "PYTHONUSERBASE", "PYTHONBREAKPOINT", "PYTHONPLATLIBDIR", "PYTHONINSPECT", "PYTHONWARNINGS", "LUA_INIT", "LUA_PATH", "LUA_CPATH", "BASH_ENV", "ENV", "SHELLOPTS", "BASHOPTS", "ZDOTDIR", "PHPRC", "PHP_INI_SCAN_DIR", "OPENSSL_CONF", "OPENSSL_MODULES", "OPENSSL_ENGINES", "LD_PRELOAD", "LD_AUDIT", "LD_LIBRARY_PATH", "GCONV_PATH"}
	got := append([]string(nil), defaultInterpTable.EnvKeys...)
	sort.Strings(got)
	w := append([]string(nil), want...)
	sort.Strings(w)
	if strings.Join(got, ",") != strings.Join(w, ",") {
		t.Fatalf("env key set %v != mandated %v", got, w)
	}
	for _, k := range want {
		if ok, inj := EnvOf(defaultInterpTable, []byte("A=b\x00"+k+"=x\x00"), nil); !ok || len(inj) != 1 || inj[0] != k {
			t.Errorf("%s not detected: %v %v", k, ok, inj)
		}
		if ok, inj := EnvOf(defaultInterpTable, []byte(k+"=--require=/x.js=y\x00"), nil); !ok || len(inj) != 1 || inj[0] != k {
			t.Errorf("value containing = must still detect %s: %v %v", k, ok, inj)
		}
		if ok, inj := EnvOf(defaultInterpTable, []byte(k+"=\x00"), nil); !ok || len(inj) != 0 {
			t.Errorf("empty %s must not count: %v %v", k, ok, inj)
		}
		if ok, inj := EnvOf(defaultInterpTable, []byte("X"+k+"=x\x00"+k+"X=x\x00"), nil); !ok || len(inj) != 0 {
			t.Errorf("lookalike of %s must not count: %v %v", k, ok, inj)
		}
	}
	for _, pf := range []string{"BASH_FUNC_", "LUA_INIT_", "LUA_PATH_", "LUA_CPATH_"} {
		k := pf + "5_4"
		if ok, inj := EnvOf(defaultInterpTable, []byte("A=b\x00"+k+"=x\x00"), nil); !ok || len(inj) != 1 || inj[0] != k {
			t.Errorf("prefix key %s not detected: %v %v", k, ok, inj)
		}
		if _, inj := EnvOf(defaultInterpTable, []byte(k+"=\x00X"+k+"=x\x00"+pf+"=x\x00"), nil); len(inj) != 0 {
			t.Errorf("empty / lookalike / bare prefix %s must not count: %v", pf, inj)
		}
	}
	if strings.Join(defaultInterpTable.EnvPrefixes, ",") != "BASH_FUNC_,LUA_INIT_,LUA_PATH_,LUA_CPATH_" || strings.Join(defaultInterpTable.EnvLoader, ",") != "LD_PRELOAD,LD_AUDIT,LD_LIBRARY_PATH,GCONV_PATH" {
		t.Errorf("prefix/loader sets: %v %v", defaultInterpTable.EnvPrefixes, defaultInterpTable.EnvLoader)
	}
	ordinary := "PATH=/usr/bin\x00HOME=/h\x00LANG=C.UTF-8\x00USER=u\x00SHELL=/bin/bash\x00PWD=/w\x00TERM=xterm\x00XDG_CONFIG_HOME=/h/.config\x00XDG_DATA_DIRS=/usr/share\x00GOPATH=/h/go\x00GOFLAGS=-mod=mod\x00JAVA_HOME=/jdk\x00CLASSPATH=/x.jar\x00PYTHONUNBUFFERED=1\x00PYTHONDONTWRITEBYTECODE=1\x00NODE_ENV=production\x00LD_ASSUME_KERNEL=\x00RUBY_GC_HEAP_INIT_SLOTS=1\x00BUNDLE_APP_CONFIG=/c\x00SSL_CERT_FILE=/c.pem\x00OPENSSL_FOO=1\x00GEM_HOMEX=1\x00"
	if ok, inj := EnvOf(defaultInterpTable, []byte(ordinary), nil); !ok || len(inj) != 0 {
		t.Errorf("an ordinary environment must stay clean: %v %v", ok, inj)
	}
	if ok, _ := EnvOf(defaultInterpTable, nil, os.ErrPermission); ok {
		t.Error("unreadable environ must be env_checked=false")
	}
	if _, inj := EnvOf(defaultInterpTable, []byte("NODE_OPTIONS=a\x00LD_PRELOAD=b\x00NODE_OPTIONS=c\x00"), nil); strings.Join(inj, ",") != "LD_PRELOAD,NODE_OPTIONS" {
		t.Errorf("dedup+sort: %v", inj)
	}
}

func TestRelativeScriptTokenNeverBinds(t *testing.T) {
	d := t.TempDir()
	sc := filepath.Join(d, "server.js")
	if err := os.WriteFile(sc, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if p, h := scriptOf(os.Getpid(), "/usr/bin/node", []string{"node", sc}); p != sc || h == "" {
		t.Fatalf("control: the absolute form must bind, got %q %q", p, h)
	}
	old, _ := os.Getwd()
	if err := os.Chdir(d); err != nil { // a REAL file at the relative path: only the absolute-only rule keeps it unbound
		t.Fatal(err)
	}
	defer os.Chdir(old) //nolint:errcheck
	if p, h := scriptOf(os.Getpid(), "/usr/bin/node", []string{"node", "server.js"}); p != "" || h != "" {
		t.Errorf("relative token bound: %q %q", p, h)
	}
}

// F5: self-contained (the mutation rung does not run liveevidence_test.go). An unresolvable named host matches nothing (fail closed), the empty host still finds
// a real listener (control needle), and an absolute script token whose file cannot be hashed never yields a bound script.
func TestUnresolvableHostFailsClosedSelfContained(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("no loopback listener available: %v", err)
	}
	defer ln.Close()
	port := ln.Addr().(*net.TCPAddr).Port
	if in := listenInodes("", port); len(in) == 0 {
		t.Fatal("control needle: the empty host (any address) must find the real listener")
	}
	if in := listenInodes("no-such-host.invalid", port); len(in) != 0 {
		t.Fatalf("unresolvable host must match nothing, got %v", in)
	}
	if a, err := ResolveListenerAddr("no-such-host.invalid", port); err == nil || a != nil {
		t.Fatalf("unresolvable host must not resolve an answering identity: %+v %v", a, err)
	}
	// a WILDCARD bind covers every resolvable host, so only the explicit unresolvable-host refusal keeps it from matching a name nobody can resolve
	wl, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Skipf("no wildcard listener available: %v", err)
	}
	defer wl.Close()
	wport := wl.Addr().(*net.TCPAddr).Port
	if in := listenInodes("127.0.0.1", wport); len(in) == 0 {
		t.Fatal("control needle: a wildcard bind covers a resolvable host")
	}
	if in := listenInodes("no-such-host.invalid", wport); len(in) != 0 {
		t.Fatalf("unresolvable host must match nothing even for a wildcard bind, got %v", in)
	}
}

func TestAbsoluteScriptThatCannotBeHashedStaysUnbound(t *testing.T) {
	d := t.TempDir()
	good := filepath.Join(d, "real.js")
	if err := os.WriteFile(good, []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if p, h := scriptOf(os.Getpid(), "/usr/bin/node", []string{"node", good}); p != good || h == "" {
		t.Fatalf("control: an existing absolute script must bind: %q %q", p, h)
	}
	for name, tok := range map[string]string{"missing": filepath.Join(d, "nope.js"), "directory": d} {
		if p, h := scriptOf(os.Getpid(), "/usr/bin/node", []string{"node", tok}); p != "" || h != "" {
			t.Errorf("%s: an unhashable script must stay unbound, got %q %q", name, p, h)
		}
	}
}
