package logredact

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// rawPathRe finds the places a raw request path/target can be read in server code.
var rawPathRe = regexp.MustCompile(`(Request|\br)\.(URL\.(Path|RawPath|String\(\))|RequestURI)`)

// rawPathAllowed: the line is a routing/parsing/response use, not a log field, or already redacted.
var rawPathAllowed = regexp.MustCompile(`logredact\.|HasPrefix|HasSuffix|== "|!= "|strings\.Split|URL\.Path\)? *$|SecretPath`)

// scanRawPathLogs returns the file:line of every raw-path read that is neither redacted nor on the reviewed allow list.
// reviewed lists "file-suffix:substring" entries examined by hand (non-logging uses).
func scanRawPathLogs(root string, reviewed []string) ([]string, error) {
	var bad []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "vendor" || d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "mocks" || d.Name() == "tests") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, rerr := os.ReadFile(p)
		if rerr != nil {
			return rerr
		}
		for i, line := range strings.Split(string(b), "\n") {
			if !rawPathRe.MatchString(line) || rawPathAllowed.MatchString(line) || strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			ok := false
			for _, r := range reviewed {
				suffix, sub, _ := strings.Cut(r, ":")
				if strings.HasSuffix(p, suffix) && strings.Contains(line, sub) {
					ok = true
				}
			}
			if !ok {
				bad = append(bad, p+":"+itoa(i+1)+": "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	return bad, err
}

func itoa(n int) string { return fmtInt(n) }

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	var d []byte
	for ; n > 0; n /= 10 {
		d = append([]byte{byte('0' + n%10)}, d...)
	}
	return string(d)
}

// Class closure (PX-F17 review F1): every raw request-path read in server code is redacted (SecretPath/Path) or a reviewed
// non-logging use. A route like GET /logs/share/:token puts a bearer secret in the path, so a raw-path log field is a leak.
func TestNoRawRequestPathInServerCode(t *testing.T) {
	reviewed := []string{
		"handlers/path_params.go:r.URL.Path",                    // parses ids out of the path, not logged
		"internal/metrics/middleware.go:c.Request.URL.Path",     // /metrics skip decision
		"middleware/pprof_guard.go:p := c.Request.URL.Path",     // routing decision
		"internal/handlers/smb_discovery.go:c.Request.URL.Path", // echoed in a 410 response, not logged
	}
	bad, err := scanRawPathLogs("../..", reviewed)
	if err != nil {
		t.Fatal(err)
	}
	if len(bad) > 0 {
		t.Fatalf("raw request path reads not redacted:\n%s", strings.Join(bad, "\n"))
	}
}

// Control needle: the scanner sees a synthetic raw-path log line (an instrument that cannot see proves nothing).
func TestRawPathScannerControlNeedle(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.go"), []byte("package x\nfunc f(){ log(c.Request.URL.Path) }\nfunc g(){ log(logredact.SecretPath(c.Request.URL.Path)) }\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	bad, err := scanRawPathLogs(dir, nil)
	if err != nil || len(bad) != 1 || !strings.Contains(bad[0], "x.go:2") {
		t.Fatalf("scanner blind or wrong: %v %v", bad, err)
	}
}
