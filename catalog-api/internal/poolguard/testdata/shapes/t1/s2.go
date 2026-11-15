package zzshapes2

import (
	"context"
	"database/sql"
	"net/http"

	. "catalogizer/internal/poolguard/testdata/shapes/t1/ext2"
	e2 "catalogizer/internal/poolguard/testdata/shapes/t1/ext2"
)

type RepoR struct{ db *sql.DB }

func (r *RepoR) Check(ctx context.Context, id int) bool {
	var ok bool
	_ = r.db.QueryRowContext(ctx, "SELECT 1", id).Scan(&ok)
	return ok
}

func (r *RepoR) ServeHTTP(w http.ResponseWriter, q *http.Request) {
	_, _ = r.db.ExecContext(q.Context(), "UPDATE x")
}

type PureR []string

func (p PureR) Check(ctx context.Context, id int) bool { return len(p) > id }

type Srv struct{}

func (Srv) ServeHTTP(w http.ResponseWriter, q *http.Request) {}

var pkgRepo = &RepoR{}

func cur(ctx context.Context, db *sql.DB) *sql.Rows { r, _ := db.QueryContext(ctx, "SELECT"); return r }

// T01: stdlib interface http.Handler; tree has a struct named Handler (ext2.Handler).
func T01StdlibIfaceNameCollides(ctx context.Context, db *sql.DB, h http.Handler, w http.ResponseWriter, q *http.Request) {
	rows := cur(ctx, db)
	for rows.Next() {
		h.ServeHTTP(w, q)
	}
}

// T02: ext alias to interface.
func T02ExtAliasIface(ctx context.Context, db *sql.DB, x e2.CheckerAlias) {
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

// T03: ext defined type with interface underlying.
func T03ExtDefinedIface(ctx context.Context, db *sql.DB, x e2.CheckerDef) {
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

// T04: ext struct embedding an interface (promoted Check from same-package dynamic value).
func T04ExtStructEmbedsIface(ctx context.Context, db *sql.DB) {
	x := e2.Wrapper{Checker: pkgRepo}
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

// T05: ext generic wrapper forwarding to same-package value.
func T05ExtGenericWrapper(ctx context.Context, db *sql.DB) {
	x := &e2.Box[*RepoR]{V: pkgRepo}
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

// T06: type parameter named like a package type with a pure method.
func T06TypeParamShadowsPkgType[PureR Checker](ctx context.Context, db *sql.DB, x PureR) {
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

// T07: function-local type declaration shadows the package type name.
func T07LocalTypeShadow(ctx context.Context, db *sql.DB) {
	type PureR = RepoR
	x := &PureR{}
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

// T08: dot-imported interface used unqualified.
func T08DotImportIface(ctx context.Context, db *sql.DB, x Checker) {
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

// T09: renamed import alias of interface package.
func T09RenamedImport(ctx context.Context, db *sql.DB, x e2.Checker) {
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

// T10: grouped var block declaring an interface-typed local.
func T10GroupedVar(ctx context.Context, db *sql.DB) {
	var (
		y         = 1
		x Checker = pkgRepo
	)
	_ = y
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

type IfHolder struct{ c Checker }

// T11: struct field typed as interface.
func T11FieldIface(ctx context.Context, db *sql.DB, h IfHolder) {
	rows := cur(ctx, db)
	for rows.Next() {
		h.c.Check(ctx, 1)
	}
}

// T12: local interface conversion via parenthesised type (unevident).
func T12ConvIface(ctx context.Context, db *sql.DB) {
	x := Checker(pkgRepo)
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

// T13: interface declared in another package via embedding only (interface{ e2.Checker }).
type Emb interface{ e2.Checker }

func T13EmbeddedExtIface(ctx context.Context, db *sql.DB, x Emb) {
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

// T14: sql.Tx-like external concrete type: a TRUE negative for the external skip (no same-package Check).
func T14ExtConcreteHandlerStruct(ctx context.Context, db *sql.DB, h e2.Handler) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = h
	}
}

// T15: shadow via labelled for-init in loop body using a typed pure local first.
func T15ForInitShadow(ctx context.Context, db *sql.DB) {
	x := PureR{}
	_ = x
	rows := cur(ctx, db)
	for rows.Next() {
		for x := pkgRepo; x != nil; x = nil {
			x.Check(ctx, 1)
		}
	}
}

var _ = Srv{}
