package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

// REDFIX1: main_router.go:altSvcMiddleware is allow-listed as a pre-gate handler (preGateAllowedSymbols). The allow-list matches
// file and symbol only, so the body is content-pinned here: a different body under the same name must fail this test. The digest is
// over the printed declaration without comments (comment and whitespace edits do not move it).
const altSvcMiddlewareDeclSHA256 = "033942399f3e818e2a4a9c82ff1b11b3f46fa7c7aff87ad2e2e9380de956d621"

func altSvcDeclDigest(t *testing.T, src []byte) string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "main_router.go", src, 0)
	require.NoError(t, err)
	var found *ast.FuncDecl
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Name.Name == "altSvcMiddleware" {
			require.Nil(t, found, "altSvcMiddleware declared twice")
			found = fd
		}
	}
	require.NotNil(t, found, "altSvcMiddleware not declared in main_router.go")
	var b bytes.Buffer
	require.NoError(t, printer.Fprint(&b, token.NewFileSet(), found))
	sum := sha256.Sum256(b.Bytes())
	return hex.EncodeToString(sum[:])
}

func TestREDFIX1_AltSvcMiddlewareContentPin(t *testing.T) {
	src, err := os.ReadFile("main_router.go")
	require.NoError(t, err)
	require.Equal(t, altSvcMiddlewareDeclSHA256, altSvcDeclDigest(t, src), "altSvcMiddleware changed: re-review it as a pre-gate handler, then update the pin")
}

// Mutant and golden-FALSE for the pin.
func TestREDFIX1_AltSvcPinMutantsAndGoldenFalse(t *testing.T) {
	src, err := os.ReadFile("main_router.go")
	require.NoError(t, err)
	base := altSvcDeclDigest(t, src)
	// mutant: the middleware aborts before the gate (a real behaviour change in the same slot)
	mut := bytes.Replace(src, []byte("\tc.Next()\n}\n\n// wantGlobalChain"), []byte("\tc.AbortWithStatus(204)\n}\n\n// wantGlobalChain"), 1)
	require.NotEqual(t, src, mut, "mutation did not apply")
	require.NotEqual(t, base, altSvcDeclDigest(t, mut), "pin must catch a modified body")
	// mutant: header value changed
	mut2 := bytes.Replace(src, []byte(`ma=86400`), []byte(`ma=1`), 1)
	require.NotEqual(t, src, mut2)
	require.NotEqual(t, base, altSvcDeclDigest(t, mut2))
	// golden-FALSE: a comment-only and whitespace-only edit does not move the pin
	gf := bytes.Replace(src, []byte("// altSvcMiddleware advertises"), []byte("// altSvcMiddleware (edited comment) advertises"), 1)
	gf = bytes.Replace(gf, []byte("\tc.Next()\n}\n\n// wantGlobalChain"), []byte("\t// inline comment\n\tc.Next()\n}\n\n// wantGlobalChain"), 1)
	require.NotEqual(t, src, gf)
	require.Equal(t, base, altSvcDeclDigest(t, gf))
	// a differently named middleware in the slot is not allow-listed at all
	require.False(t, preGateAllowed(routeHandler{Name: "main.otherMiddleware", File: "/x/catalog-api/main_router.go"}))
	require.True(t, preGateAllowed(routeHandler{Name: "main.altSvcMiddleware", File: "/x/catalog-api/main_router.go"}))
}
