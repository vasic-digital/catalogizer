package database

// CAT-PG-TX-SWEEP-1 guard: a statement run on a raw *sql.Tx bypasses the dialect rewrite and sends `?` / sqlite-only syntax to
// PostgreSQL. DB.Begin/BeginTx therefore return *Tx (tx.go). This guard fails when non-test code outside this package (and
// outside the allowlist) names the raw type, unwraps the wrapper to reach a raw transaction, or calls SQLTx().
//
// Allowlist (each entry justified): internal/media/database is the separate SQLite/SQLCipher-only media store, opened as a
// raw *sql.DB with no PostgreSQL mode, so there is no dialect to rewrite for.

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var rawTxAllowlist = []string{
	"internal/media/database/", // SQLite/SQLCipher-only media store (raw *sql.DB by design)
}

// rawTxSites returns one finding per raw-transaction reference in the parsed file.
func rawTxSites(fset *token.FileSet, f *ast.File) []string {
	var out []string
	ast.Inspect(f, func(n ast.Node) bool {
		se, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pos := fset.Position(se.Pos())
		if id, ok := se.X.(*ast.Ident); ok && id.Name == "sql" && se.Sel.Name == "Tx" {
			out = append(out, pos.String()+": raw sql.Tx type")
		}
		switch se.Sel.Name {
		case "SQLTx":
			out = append(out, pos.String()+": Tx.SQLTx() unwrap")
		case "Begin", "BeginTx":
			if inner, ok := se.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "DB" {
				out = append(out, pos.String()+": Begin on the unwrapped *sql.DB (rewrite bypass)")
			}
		}
		return true
	})
	return out
}

func TestNoRawSQLTxOutsideDatabasePackage(t *testing.T) {
	root, err := filepath.Abs("..")
	require.NoError(t, err)
	var findings []string
	scanned := 0
	err = filepath.Walk(root, func(p string, info os.FileInfo, werr error) error {
		if werr != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if info.IsDir() {
			if info.Name() == "vendor" || info.Name() == ".git" || info.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") || strings.HasPrefix(rel, "database/") {
			return nil
		}
		for _, a := range rawTxAllowlist {
			if strings.HasPrefix(rel, a) {
				return nil
			}
		}
		fset := token.NewFileSet()
		f, perr := parser.ParseFile(fset, p, nil, 0)
		require.NoError(t, perr, rel)
		scanned++
		for _, s := range rawTxSites(fset, f) {
			findings = append(findings, strings.Replace(s, root+"/", "", 1))
		}
		return nil
	})
	require.NoError(t, err)
	// Control needle (§11.4.273): an empty result is meaningful only if the walk actually saw the code it is about.
	assert.Greater(t, scanned, 200, "guard walked too few files: instrument is blind")
	assert.Empty(t, findings, "raw *sql.Tx statement sites bypass the dialect rewrite; use database.Tx (DB.Begin/BeginTx)")
}

func TestRawTxSiteScannerFindsPlantedSites(t *testing.T) {
	cases := map[string]struct {
		src  string
		want int
	}{
		"param type":       {`package x; import "database/sql"; func f(tx *sql.Tx) {}`, 1},
		"raw unwrap":       {`package x; func f(db *D) { tx, _ := db.DB.Begin(); _ = tx }`, 1},
		"raw unwrap ctx":   {`package x; func f(db *D) { tx, _ := db.DB.BeginTx(nil, nil); _ = tx }`, 1},
		"Raw call":         {`package x; func f(tx *T) { _ = tx.SQLTx() }`, 1},
		"clean wrapper":    {`package x; func f(db *D) { tx, _ := db.BeginTx(nil, nil); tx.Exec("x"); _ = tx }`, 0},
		"sql.DB is fine":   {`package x; import "database/sql"; func f(db *sql.DB) {}`, 0},
		"sql.Rows is fine": {`package x; import "database/sql"; func f(r *sql.Rows) {}`, 0},
	}
	for name, c := range cases {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name+".go", c.src, 0)
		require.NoError(t, err, name)
		assert.Len(t, rawTxSites(fset, f), c.want, name)
	}
}
