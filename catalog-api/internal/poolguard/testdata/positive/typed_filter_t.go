package fixture

import (
	"context"
	"database/sql"
	"net/http"

	e2 "catalogizer/internal/poolguard/testdata/positive/ext2"
)

// ServeHTTP is DB-backed, so a loop calling an http.Handler may reach it.
func (r *RepoR) ServeHTTP(w http.ResponseWriter, q *http.Request) {
	_, _ = r.db.ExecContext(q.Context(), "UPDATE x")
}

// T01: stdlib interface; the tree has a struct also named Handler (ext2.Handler).
func T01StdlibIfaceNameCollides(ctx context.Context, db *sql.DB, h http.Handler, w http.ResponseWriter, q *http.Request) error {
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		h.ServeHTTP(w, q)
	}
	return nil
}

// T02: alias to an interface of another in-tree package.
func T02ExtAliasIface(ctx context.Context, db *sql.DB, x e2.CheckerAlias) error {
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		x.Check(ctx, 1)
	}
	return nil
}

// T03: defined type over an interface.
func T03ExtDefinedIface(ctx context.Context, db *sql.DB, x e2.CheckerDef) error {
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		x.Check(ctx, 1)
	}
	return nil
}

// T04: struct embedding an interface (promoted method).
func T04ExtStructEmbedsIface(ctx context.Context, db *sql.DB) error {
	x := e2.Wrapper{Checker: pkgRepoB}
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		x.Check(ctx, 1)
	}
	return nil
}

// T05: generic wrapper forwarding to a same-package value.
func T05ExtGenericWrapper(ctx context.Context, db *sql.DB) error {
	x := &e2.Box[*RepoR]{V: pkgRepoB}
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		x.Check(ctx, 1)
	}
	return nil
}

// T06: a type parameter named like a package type that owns a pure Check.
func T06TypeParamShadowsPkgType[PureR Checker](ctx context.Context, db *sql.DB, x PureR) error {
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		x.Check(ctx, 1)
	}
	return nil
}

// T07: a function-local alias shadows the package type name.
func T07LocalTypeShadow(ctx context.Context, db *sql.DB) error {
	type PureR = RepoR
	x := &PureR{}
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		x.Check(ctx, 1)
	}
	return nil
}

type twoRecv struct{}

// callsBoth calls the same bare method on a pure and a DB-backed receiver, in that order: the
// seen-set must key on the receiver type or the second call is never examined.
func (twoRecv) callsBoth(ctx context.Context, a PureR, b *RepoR) {
	a.Check(ctx, 1)
	b.Check(ctx, 1)
}

// SeenKeyedByRecvType kills the "seen keyed by bare name" mutant.
func SeenKeyedByRecvType(ctx context.Context, db *sql.DB, r *RepoR) error {
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		twoRecv{}.callsBoth(ctx, nil, r)
	}
	return nil
}
