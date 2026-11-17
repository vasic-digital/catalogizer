package netguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// AZ2-1 class guard. Every construction of an outbound HTTP client in non-test code under catalog-api (an
// http.Client or http.Transport literal, http.DefaultClient, the package-level http.Get/Post/Head/PostForm) must be
// either inside internal/netguard (the one place that builds them) or listed in az21Scoped with the reason it is
// not a fixed-host provider client. A new provider client that uses a plain http.Client therefore fails this test.
// The allow-list is keyed by file and enclosing function, never by line.
var az21Scoped = map[string]string{
	// Other netguard client classes, each already dial-time checked with its own redirect policy.
	"filesystem/s3_client.go|":     "operator storage class (OperatorCheckRedirectFor + class transport)",
	"filesystem/webdav_client.go|": "operator storage class (OperatorCheckRedirectFor + class transport)",
	"services/webdav_client.go|":   "operator storage class (StrictClassTransport / OperatorCheckRedirect)",
	"internal/httpclient/pool.go|": "generic pool primitive; callers pick the transport (no provider caller; grep-checked)",
	// Image proxy and DNS bootstrap in main.go.
	"main.go|buildImageProxyClient": "public image proxy: netguard dial-time check, host allow-list (AZ2-4) and a redirect policy in newImageProxyClient",
	// Cover art download with an operator proxy keeps its own guardedRedirect (string-level guard per hop).
	"internal/services/cover_art_service.go|downloadImage": "operator-proxy variant: guardedRedirect re-checks each hop; the default client is netguard.StrictClient",
	// Integration-test and challenge harnesses talk to the local API under test, not to providers.
	"challenges/": "challenge harness: clients of the local API under test",
}

func az21IsScoped(rel, fn string) (string, bool) {
	if strings.HasPrefix(rel, "internal/netguard/") {
		return "netguard builds the clients", true
	}
	for k, why := range az21Scoped {
		if strings.HasSuffix(k, "/") && strings.HasPrefix(rel, k) {
			return why, true
		}
		if f, g, ok := strings.Cut(k, "|"); ok && f == rel && (g == "" || g == fn) {
			return why, true
		}
	}
	return "", false
}

func az21Offenders(t testing.TB, root string) []string {
	var out []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", "testdata", ".git", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, p, nil, 0)
		if perr != nil {
			return nil
		}
		hasHTTP := false
		for _, im := range f.Imports {
			if im.Path.Value == `"net/http"` {
				hasHTTP = true
			}
		}
		if !hasHTTP {
			return nil
		}
		check := func(fn string, n ast.Node) {
			var what string
			switch x := n.(type) {
			case *ast.CompositeLit:
				if sel, ok := x.Type.(*ast.SelectorExpr); ok && isIdent(sel.X, "http") && (sel.Sel.Name == "Client" || sel.Sel.Name == "Transport") {
					what = "http." + sel.Sel.Name + " literal"
				}
			case *ast.SelectorExpr:
				if isIdent(x.X, "http") && x.Sel.Name == "DefaultClient" {
					what = "http.DefaultClient"
				}
			case *ast.CallExpr:
				if sel, ok := x.Fun.(*ast.SelectorExpr); ok && isIdent(sel.X, "http") {
					switch sel.Sel.Name {
					case "Get", "Post", "Head", "PostForm":
						what = "http." + sel.Sel.Name + "(...) on the default client"
					}
				}
				if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "new" && len(x.Args) == 1 {
					if sel, ok := x.Args[0].(*ast.SelectorExpr); ok && isIdent(sel.X, "http") && (sel.Sel.Name == "Client" || sel.Sel.Name == "Transport") {
						what = "new(http." + sel.Sel.Name + ")"
					}
				}
			}
			if what == "" {
				return
			}
			if _, ok := az21IsScoped(rel, fn); ok {
				return
			}
			out = append(out, rel+"|"+fn+": "+what)
		}
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				name := d.Name.Name
				ast.Inspect(d, func(n ast.Node) bool { check(name, n); return true })
			case *ast.GenDecl:
				ast.Inspect(d, func(n ast.Node) bool { check("init", n); return true })
			}
		}
		return nil
	})
	require.NoError(t, err)
	sort.Strings(out)
	return out
}

func isIdent(e ast.Expr, name string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == name
}

func TestAZ21_NoPlainHTTPClientOutsideNetguard(t *testing.T) {
	require.Empty(t, az21Offenders(t, "../.."), "build outbound clients with netguard.NewProviderClient (or scope them in az21Scoped with a reason)")
}

// Control needle + polarity: the instrument flags a plain client in a scratch tree and passes a netguard one.
func TestAZ21_GuardInstrumentSeesPlainClients(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644))
	}
	write("p/bad_literal.go", "package p\nimport \"net/http\"\nfunc F() *http.Client { return &http.Client{} }\n")
	write("p/bad_default.go", "package p\nimport \"net/http\"\nfunc G() { _, _ = http.Get(\"x\"); _ = http.DefaultClient }\n")
	write("p/bad_new.go", "package p\nimport \"net/http\"\nvar c = new(http.Client)\n")
	write("p/good.go", "package p\nimport \"net/http\"\nfunc H(c *http.Client) { _, _ = c.Get(\"x\") }\n")
	write("p/bad_test.go", "package p\nimport \"net/http\"\nvar _ = &http.Client{}\n")
	got := az21Offenders(t, dir)
	require.Len(t, got, 4, "%v", got) // literal, DefaultClient, Get call, new()
	for _, g := range got {
		require.NotContains(t, g, "good.go")
		require.NotContains(t, g, "bad_test.go")
	}
}

// M3: netguardtest is a test support package; only _test.go files may import it.
func TestAZ21_NetguardtestImportedByTestsOnly(t *testing.T) {
	root := "../.."
	var bad []string
	_ = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case "vendor", ".git", "node_modules", "netguardtest":
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(token.NewFileSet(), p, nil, parser.ImportsOnly)
		if perr != nil {
			return nil
		}
		for _, im := range f.Imports {
			if strings.HasSuffix(strings.Trim(im.Path.Value, `"`), "/netguard/netguardtest") {
				bad = append(bad, p)
			}
		}
		return nil
	})
	require.Empty(t, bad, "non-test files must not import netguardtest")
}
