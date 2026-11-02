package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var adminPwAssign = regexp.MustCompile(`ADMIN_(?:NEW_)?PASSWORD["']?\s*[=:]\s*["']?([^"'\s]+)`)

// censusAdminPasswordValues extracts the literal values assigned to
// ADMIN_PASSWORD in the given text files.
func censusAdminPasswordValues(t *testing.T, files []string) map[string][]string {
	t.Helper()
	out := map[string][]string{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for i, line := range strings.Split(string(b), "\n") {
			for _, m := range adminPwAssign.FindAllStringSubmatch(line, -1) {
				out[m[1]] = append(out[m[1]], f+":"+string(rune('0'+i%10)))
			}
		}
	}
	return out
}

func repoFiles(t *testing.T) []string {
	t.Helper()
	root := filepath.Join("..", "..")
	var files []string
	skip := []string{"/submodules/", "/constitution/", "/node_modules/", "/.git/", "/docs/reports/", "/docs/audits/", "/docs/qa/", "/specs/", "/docs/plans/", "/docs/superpowers/"}
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		sp := filepath.ToSlash(p)
		for _, s := range skip {
			if strings.Contains(sp, s) {
				return nil
			}
		}
		if strings.HasSuffix(sp, ".go") || info.Size() > 2<<20 || strings.Contains(sp, "/scripts/") ||
			strings.HasSuffix(sp, "FOLLOW_UPS_NEXT_PHASE.md") || strings.HasSuffix(sp, "/.sh") || strings.HasSuffix(sp, ".sh") {
			return nil // scripts/tests carry deliberate fixtures; the prose follow-up log quotes them
		}
		for _, ext := range []string{".md", ".yml", ".yaml", ".example", ".html"} {
			if strings.HasSuffix(sp, ext) || strings.Contains(filepath.Base(sp), ".env") {
				files = append(files, p)
				break
			}
		}
		return nil
	})
	return files
}

// D1 census: every published example ADMIN_PASSWORD value must be rejected by
// the denylist, a ${VAR}/$VAR reference or a <template>; a copy-pasteable
// example must never be accepted.
func TestDocsAdminPasswordExamplesAreAllRejected(t *testing.T) {
	// control needle 1 (instrument can see): a known-present value is extracted
	tmp := filepath.Join(t.TempDir(), "x.md")
	require.NoError(t, os.WriteFile(tmp, []byte("export ADMIN_PASSWORD=\"needle-present-value\"\n"), 0o600))
	seen := censusAdminPasswordValues(t, []string{tmp})
	require.Contains(t, seen, "needle-present-value")
	// control needle 2: a fabricated absent token is not found in the repo census
	files := repoFiles(t)
	require.Greater(t, len(files), 50, "census must see the repository")
	vals := censusAdminPasswordValues(t, files)
	require.NotContains(t, vals, "zz-never-written-9f3")
	require.NotEmpty(t, vals, "census must find documented ADMIN_PASSWORD values")

	for v, where := range vals {
		v = strings.TrimRight(v, ",.;)`")
		if strings.HasPrefix(v, "$") || strings.HasPrefix(v, "<") || strings.HasPrefix(v, "`") || strings.HasPrefix(v, "\\$") {
			continue // variable reference
		}
		assert.True(t, IsPlaceholderPassword(v), "documented example %q would be ACCEPTED as a real password (first at %v)", v, where[0])
	}
}

func TestIsPlaceholderPassword_ExampleShapes(t *testing.T) {
	for _, v := range []string{"your-admin-password", "your_admin_password_here", "secure-admin-password",
		"your-secure-admin-password", "your-admin-password-here", "your-secure-password", "strong-admin-password",
		"SecureAdminPassword123!", "your-api-key", "your_token_value", "db-secret-here", "my-password-here", "SecurePass123!", "SecurePassword123", "<generate-your-own>"} {
		assert.True(t, IsPlaceholderPassword(v), v)
	}
	// real-looking values stay accepted (no false positive)
	for _, v := range []string{"Op3rator-Supplied-Value-9x", "k8#Qm2!vLp0zR", "correct-horse-battery-staple-42"} {
		assert.False(t, IsPlaceholderPassword(v), v)
	}
}
