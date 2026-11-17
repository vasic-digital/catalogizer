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

// sqlAliases returns the local names under which "database/sql" is imported in f (default "sql"; an aliased import such as
// `stdsql "database/sql"` is followed; blank and dot imports bind no usable name and are ignored).
func sqlAliases(f *ast.File) map[string]bool {
	out := map[string]bool{}
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, "\"`") != "database/sql" {
			continue
		}
		name := "sql"
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name != "_" && name != "." {
			out[name] = true
		}
	}
	return out
}

// isSQLSel reports whether e is `<sqlalias>.<sel>` (optionally behind a pointer).
func isSQLSel(e ast.Expr, aliases map[string]bool, sel string) bool {
	if st, ok := e.(*ast.StarExpr); ok {
		e = st.X
	}
	se, ok := e.(*ast.SelectorExpr)
	if !ok || se.Sel.Name != sel {
		return false
	}
	id, ok := se.X.(*ast.Ident)
	return ok && aliases[id.Name]
}

// rawTxSites returns one finding per raw-transaction reference in the parsed file. The scan is AST based with three bits of
// declared-type tracking (an AST walk, NOT go/types; the limits are listed in the SUMMARY): the (possibly aliased) database/sql
// import, names declared as *sql.DB (parameters, vars, struct fields), and local names assigned from an unwrap of the
// wrapper's embedded `.DB`. Findings:
//   - the raw sql.Tx type (any import alias);
//   - Tx.SQLTx() and the embedded-field unwrap `<x>.Tx.Tx` (reaching the raw *sql.Tx of a *database.Tx / *database.Transaction);
//   - Begin/BeginTx on `<x>.DB` directly, on a local assigned from `<x>.DB` (`raw := db.DB; raw.BeginTx(...)`), or on a name
//     declared as *sql.DB.
func rawTxSites(fset *token.FileSet, f *ast.File) []string {
	var out []string
	aliases := sqlAliases(f)

	// Pass 1: names that hold a raw *sql.DB.
	rawDB := map[string]bool{}
	markNames := func(names []*ast.Ident, typ ast.Expr) {
		if typ != nil && isSQLSel(typ, aliases, "DB") {
			for _, n := range names {
				rawDB[n.Name] = true
			}
		}
	}
	isDotDB := func(e ast.Expr) bool {
		se, ok := e.(*ast.SelectorExpr)
		return ok && se.Sel.Name == "DB"
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.Field: // parameters, results, struct fields
			markNames(v.Names, v.Type)
		case *ast.ValueSpec:
			markNames(v.Names, v.Type)
			for i, val := range v.Values {
				if i < len(v.Names) && isDotDB(val) {
					rawDB[v.Names[i].Name] = true
				}
			}
		case *ast.AssignStmt:
			for i, rhs := range v.Rhs {
				if i < len(v.Lhs) && isDotDB(rhs) {
					switch l := v.Lhs[i].(type) {
					case *ast.Ident:
						rawDB[l.Name] = true
					case *ast.SelectorExpr:
						rawDB[l.Sel.Name] = true
					}
				}
			}
		}
		return true
	})

	// Pass 2: findings.
	ast.Inspect(f, func(n ast.Node) bool {
		se, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pos := fset.Position(se.Pos())
		if id, ok := se.X.(*ast.Ident); ok && aliases[id.Name] && se.Sel.Name == "Tx" {
			out = append(out, pos.String()+": raw sql.Tx type")
		}
		if inner, ok := se.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "Tx" && se.Sel.Name == "Tx" {
			out = append(out, pos.String()+": embedded raw *sql.Tx unwrap (.Tx.Tx)")
		}
		switch se.Sel.Name {
		case "SQLTx":
			out = append(out, pos.String()+": Tx.SQLTx() unwrap")
		case "Begin", "BeginTx":
			switch x := se.X.(type) {
			case *ast.SelectorExpr:
				if x.Sel.Name == "DB" || rawDB[x.Sel.Name] {
					out = append(out, pos.String()+": Begin on a raw *sql.DB (rewrite bypass)")
				}
			case *ast.Ident:
				if rawDB[x.Name] {
					out = append(out, pos.String()+": Begin on a raw *sql.DB (rewrite bypass)")
				}
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
		// PGTX1-R-F1 planted shapes the first scanner missed:
		"aliased import":           {`package x; import stdsql "database/sql"; func f(tx *stdsql.Tx) {}`, 1},
		"aliased sql.DB ok":        {`package x; import stdsql "database/sql"; func f(db *stdsql.DB) { _ = db }`, 0},
		"local unwrap":             {`package x; func f(db *D) { raw := db.DB; tx, _ := raw.BeginTx(nil, nil); _ = tx }`, 1},
		"local unwrap var":         {`package x; func f(db *D) { var raw = db.DB; tx, _ := raw.Begin(); _ = tx }`, 1},
		"field unwrap":             {`package x; type S struct{ raw *D2 }; func (s *S) f(db *D) { s.raw = db.DB; tx, _ := s.raw.Begin(); _ = tx }`, 1},
		"declared *sql.DB":         {`package x; import "database/sql"; func f(raw *sql.DB) { tx, _ := raw.Begin(); _ = tx }`, 1},
		"declared sql.DB fld":      {`package x; import "database/sql"; type S struct{ raw *sql.DB }; func (s *S) f() { tx, _ := s.raw.BeginTx(nil, nil); _ = tx }`, 1},
		"embedded tx unwrap":       {`package x; func f(tx *database.Transaction) { _, _ = tx.Tx.Tx.Exec("x") }`, 1},
		"Transaction wrapped fine": {`package x; func f(tx *database.Transaction) { _, _ = tx.Exec("x"); _, _ = tx.Tx.Exec("x") }`, 0},
		"non-sql alias named sql":  {`package x; func f(sql *Other) { _ = sql.Tx }`, 0},
	}
	for name, c := range cases {
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, name+".go", c.src, 0)
		require.NoError(t, err, name)
		assert.Len(t, rawTxSites(fset, f), c.want, name)
	}
}
