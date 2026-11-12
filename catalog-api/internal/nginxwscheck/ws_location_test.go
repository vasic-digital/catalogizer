package nginxwscheck

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// SEC-WS-1R3-1/R3-2/F6: every nginx location that proxies the WebSocket (a
// `location ... /ws` block, or any location that forwards `Upgrade
// $http_upgrade`) must forward the client's Host WITH its port
// (`$http_host`); `$host` strips the port and the server-side Origin
// host:port check then answers 403. Server-level headers are NOT inherited
// once a location sets any proxy_set_header of its own, so the assertion is
// per location. Scans every nginx conf/template under the repo and every
// fenced snippet in the Markdown docs. Config-level only (PROVISIONAL); no
// nginx process is started here.

var (
	wsLocRe    = regexp.MustCompile(`location\s+(?:[=~*^]+\s*)?\S*/ws\S*\s*\{`)
	anyLocRe   = regexp.MustCompile(`location\s+[^{;]*\{`)
	hostGoodRe = regexp.MustCompile(`proxy_set_header\s+Host\s+\\?\$http_host\s*;`)
	hostAnyRe  = regexp.MustCompile(`proxy_set_header\s+Host\s`)
	upgradeRe  = regexp.MustCompile(`proxy_set_header\s+Upgrade\s+\\?\$http_upgrade\s*;`)
	connRe     = regexp.MustCompile(`proxy_set_header\s+Connection\s+["']?(?i:upgrade)["']?\s*;|proxy_set_header\s+Connection\s+\\?\$connection_upgrade\s*;`)
)

// stripNginxComments removes `#` comments (to end of line) so a commented-out
// directive cannot satisfy, or hide, a match (SEC-WS-1R3FU-1).
func stripNginxComments(body string) string {
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		if k := strings.Index(l, "#"); k >= 0 {
			lines[i] = l[:k]
		}
	}
	return strings.Join(lines, "\n")
}

// wsLocationViolations returns every rule a WebSocket location body breaks.
func wsLocationViolations(body string) []string {
	b := stripNginxComments(body)
	var v []string
	if !hostGoodRe.MatchString(b) {
		v = append(v, "missing `proxy_set_header Host $http_host;`")
	}
	if n := len(hostAnyRe.FindAllString(b, -1)); n > 1 {
		v = append(v, "more than one `proxy_set_header Host`")
	}
	if !upgradeRe.MatchString(b) {
		v = append(v, "missing `proxy_set_header Upgrade $http_upgrade;`")
	}
	if !connRe.MatchString(b) {
		v = append(v, "missing `proxy_set_header Connection` upgrade line")
	}
	return v
}

type locBlock struct {
	file string
	line int
	body string
}

func extractLocations(file, src string) []locBlock {
	var out []locBlock
	for _, m := range anyLocRe.FindAllStringIndex(src, -1) {
		depth, end := 1, -1
		for i := m[1]; i < len(src); i++ {
			switch src[i] {
			case '{':
				depth++
			case '}':
				depth--
			}
			if depth == 0 {
				end = i
				break
			}
		}
		if end < 0 {
			continue
		}
		out = append(out, locBlock{file, 1 + strings.Count(src[:m[0]], "\n"), src[m[0]:end]})
	}
	return out
}

func isWSLocation(b locBlock) bool {
	head := b.body[:strings.Index(b.body, "{")+1]
	return wsLocRe.MatchString(head) || strings.Contains(stripNginxComments(b.body), "$http_upgrade")
}

func repoNginxSources(t *testing.T) map[string]string {
	t.Helper()
	root := "../../.."
	srcs := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case ".git", "node_modules", "constitution", "vendor", "submodules", "dist", "build":
				return filepath.SkipDir
			}
			return nil
		}
		base := d.Name()
		isConf := strings.HasSuffix(base, ".conf") || strings.Contains(base, "nginx.conf") ||
			(strings.Contains(p, "nginx") && strings.HasSuffix(base, ".template"))
		isDoc := strings.HasSuffix(base, ".md") && strings.Contains(filepath.ToSlash(p), "docs")
		if !isConf && !isDoc {
			return nil
		}
		sp := filepath.ToSlash(p)
		if strings.Contains(sp, "FOLLOW_UPS") || strings.Contains(sp, "docs/qa/sec-ws-1") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err == nil {
			srcs[p] = string(b)
		}
		return nil
	})
	require.NoError(t, err)
	return srcs
}

func TestEveryNginxWSLocationForwardsClientHostWithPort(t *testing.T) {
	srcs := repoNginxSources(t)
	seen := 0
	for f, s := range srcs {
		for _, b := range extractLocations(f, s) {
			if !isWSLocation(b) {
				continue
			}
			seen++
			assert.Emptyf(t, wsLocationViolations(b.body),
				"%s:%d: WebSocket location violations", b.file, b.line)
		}
	}
	// control needle: the scan must actually have seen the known shipped /ws locations
	assert.GreaterOrEqual(t, seen, 5, "scan blind: too few WebSocket locations found")
	var stack3 bool
	for f := range srcs {
		if strings.HasSuffix(f, "docs/qa/stack3/monitoring/nginx/nginx.conf") {
			stack3 = true
		}
	}
	assert.True(t, stack3, "scan blind: stack3 nginx.conf not scanned")
}

func TestNginxWSLocationScannerSelfCheck(t *testing.T) {
	bad := "server { location /ws { proxy_pass http://a; proxy_set_header Upgrade $http_upgrade; } }"
	good := "server { location /ws { proxy_pass http://a; proxy_set_header Host $http_host; } }"
	hostv := "server { location /api/ { proxy_set_header Host $host; proxy_set_header Upgrade $http_upgrade; } }"
	for src, want := range map[string]bool{bad: false, good: true, hostv: false} {
		bs := extractLocations("x", src)
		require.Len(t, bs, 1)
		require.True(t, isWSLocation(bs[0]))
		assert.Equal(t, want, hostGoodRe.MatchString(bs[0].body), src)
	}
}

const wsGoodLoc = `location /ws {
    proxy_pass http://a;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $http_host;
}`

// SEC-WS-1R3FU-1/2: reviewer mutants Ma/Md/Me must now be rejected; the
// good location must pass (golden-FALSE guard).
func TestWSLocationMutantsAreRejected(t *testing.T) {
	assert.Empty(t, wsLocationViolations(wsGoodLoc), "golden-good must pass")
	mut := map[string]string{
		"Ma commented Host": strings.Replace(wsGoodLoc, "proxy_set_header Host $http_host;", "# proxy_set_header Host $http_host;", 1),
		"Md duplicate Host": strings.Replace(wsGoodLoc, "proxy_set_header Host $http_host;", "proxy_set_header Host $http_host;\n    proxy_set_header Host $host;", 1),
		"Me no Upgrade":     strings.Replace(wsGoodLoc, "    proxy_set_header Upgrade $http_upgrade;\n", "", 1),
		"Me no Connection":  strings.Replace(wsGoodLoc, "    proxy_set_header Connection \"upgrade\";\n", "", 1),
		"Me commented Conn": strings.Replace(wsGoodLoc, "proxy_set_header Connection", "# proxy_set_header Connection", 1),
	}
	for name, body := range mut {
		assert.NotEmptyf(t, wsLocationViolations(body), "mutant %s survived", name)
	}
	// a trailing comment after a valid directive must not break a good location
	assert.Empty(t, wsLocationViolations(strings.Replace(wsGoodLoc, "$http_host;", "$http_host; # keep port", 1)))
}
