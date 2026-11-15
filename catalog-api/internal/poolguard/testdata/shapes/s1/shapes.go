package pgshapes

import (
	"context"
	"database/sql"

	"catalogizer/internal/poolguard/testdata/shapes/s1/ext"
)

type RepoR struct{ db *sql.DB }

func (r *RepoR) Check(ctx context.Context, id int) bool {
	var ok bool
	_ = r.db.QueryRowContext(ctx, "SELECT 1", id).Scan(&ok)
	return ok
}

type PureR []string

func (p PureR) Check(ctx context.Context, id int) bool { return len(p) > id }

type Checker interface {
	Check(ctx context.Context, id int) bool
}
type Checker2 interface{ Checker } // embedded interface
type EmbedSvc struct{ *RepoR }
type Wrap struct{ *RepoR }
type AliasR = RepoR
type Holder struct{ repo *RepoR }

type G[T any] struct{ db *sql.DB }

func (g *G[T]) Check(ctx context.Context, id int) bool {
	_, _ = g.db.ExecContext(ctx, "UPDATE x")
	return true
}

type GG[K comparable, V any] struct{ db *sql.DB }

func (g *GG[K, V]) Check(ctx context.Context, id int) bool {
	_, _ = g.db.ExecContext(ctx, "UPDATE x")
	return true
}

var pkgRepo = &RepoR{}
var p = &RepoR{} // package-level, shadowed only in a sibling block below

func cur(ctx context.Context, db *sql.DB) *sql.Rows { r, _ := db.QueryContext(ctx, "SELECT"); return r }
func getRepo() *RepoR                               { return pkgRepo }

func S01RangeShadow(ctx context.Context, db *sql.DB, rs []*RepoR) {
	var x PureR
	_ = x
	rows := cur(ctx, db)
	for rows.Next() {
		for _, x := range rs {
			x.Check(ctx, 1)
		}
	}
}

func S02IfaceParam(ctx context.Context, db *sql.DB, x Checker) {
	var y PureR
	_ = y
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

func S03Promoted(ctx context.Context, db *sql.DB) {
	x := &EmbedSvc{}
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

func S04EmbeddedLocalLit(ctx context.Context, db *sql.DB) {
	x := Wrap{RepoR: pkgRepo}
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

func S05Alias(ctx context.Context, db *sql.DB) {
	x := &AliasR{}
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

func S06ClosureShadow(ctx context.Context, db *sql.DB) {
	x := PureR{}
	_ = x
	rows := cur(ctx, db)
	for rows.Next() {
		func(x *RepoR) { x.Check(ctx, 1) }(pkgRepo)
	}
}

func S07FieldRecv(ctx context.Context, db *sql.DB, h Holder) {
	rows := cur(ctx, db)
	for rows.Next() {
		h.repo.Check(ctx, 1)
	}
}

func S08CallResultRecv(ctx context.Context, db *sql.DB) {
	rows := cur(ctx, db)
	for rows.Next() {
		getRepo().Check(ctx, 1)
	}
}

func S09TypeSwitch(ctx context.Context, db *sql.DB, v any) {
	x := PureR{}
	_ = x
	rows := cur(ctx, db)
	for rows.Next() {
		switch x := v.(type) {
		case *RepoR:
			x.Check(ctx, 1)
		}
	}
}

func S10MethodExpr(ctx context.Context, db *sql.DB) {
	rows := cur(ctx, db)
	for rows.Next() {
		(*RepoR).Check(pkgRepo, ctx, 1)
	}
}

func S11MethodValue(ctx context.Context, db *sql.DB) {
	f := pkgRepo.Check
	rows := cur(ctx, db)
	for rows.Next() {
		f(ctx, 1)
	}
}

func S12Generic1(ctx context.Context, db *sql.DB) {
	x := &G[int]{db: db}
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

func S13Generic2(ctx context.Context, db *sql.DB) {
	x := &GG[int, string]{db: db}
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

func S14EmbeddedIface(ctx context.Context, db *sql.DB, x Checker2) {
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

// external-package interface whose dynamic value is the same-package *RepoR.
func S15ExternalIface(ctx context.Context, db *sql.DB, x ext.Checker) {
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

func S16ExternalIfaceVar(ctx context.Context, db *sql.DB) {
	var x ext.Checker = pkgRepo
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

// package-level p used in the loop; p is a PureR only inside an unrelated sibling block.
func S17PkgVarSiblingShadow(ctx context.Context, db *sql.DB) {
	{
		var p PureR
		_ = p
	}
	rows := cur(ctx, db)
	for rows.Next() {
		p.Check(ctx, 1)
	}
}

func S18IfInitShadow(ctx context.Context, db *sql.DB) {
	x := PureR{}
	_ = x
	rows := cur(ctx, db)
	for rows.Next() {
		if x := getRepo(); x != nil {
			x.Check(ctx, 1)
		}
	}
}

func S19SelectShadow(ctx context.Context, db *sql.DB, ch chan *RepoR) {
	x := PureR{}
	_ = x
	rows := cur(ctx, db)
	for rows.Next() {
		select {
		case x := <-ch:
			x.Check(ctx, 1)
		}
	}
}

func S20TypeParamRecv[C Checker](ctx context.Context, db *sql.DB, x C) {
	rows := cur(ctx, db)
	for rows.Next() {
		x.Check(ctx, 1)
	}
}

// shadowed package name: local named like the imported package ext.
func S21ShadowedPkgName(ctx context.Context, db *sql.DB) {
	ext := pkgRepo
	rows := cur(ctx, db)
	for rows.Next() {
		ext.Check(ctx, 1)
	}
}

// closure-local declaration (different scope) of a pure type under the name used for a package var.
func S22ClosureLocalShadowsPkgVar(ctx context.Context, db *sql.DB) {
	_ = func() { p := PureR{}; _ = p }
	rows := cur(ctx, db)
	for rows.Next() {
		p.Check(ctx, 1)
	}
}

// external-typed helper param reached indirectly (depth 1).
func helperExt(ctx context.Context, c ext.Checker) { c.Check(ctx, 1) }

func S23ExternalIfaceViaHelper(ctx context.Context, db *sql.DB) {
	rows := cur(ctx, db)
	for rows.Next() {
		helperExt(ctx, pkgRepo)
	}
}

var _ = ext.Pure{}
