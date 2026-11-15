package zr2

import (
	"context"
	"database/sql"
	"fmt"

	"catalogizer/internal/poolguard/testdata/zr2/bad"
	"catalogizer/internal/poolguard/testdata/zr2/win"
	"vx/p"
)

type Repo struct{ db *sql.DB }

func (r *Repo) String() string {
	_, _ = r.db.ExecContext(context.Background(), "UPDATE s")
	return ""
}
func (r *Repo) Load() error {
	_, err := r.db.ExecContext(context.Background(), "UPDATE l")
	return err
}

func cur(ctx context.Context, db *sql.DB) *sql.Rows { r, _ := db.QueryContext(ctx, "SELECT"); return r }

type Deep struct{}

func (Deep) String() string { return "pure" }
func (Deep) Load() error    { return nil }

type Mid struct{ Deep }

// N01 alias chain to an interface literal embedding an unresolved interface.
type A2 = interface{ fmt.Stringer }
type A1 = A2
type O1 struct {
	A1
	Mid
}

func N01AliasChain(ctx context.Context, db *sql.DB, o O1) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = o.String()
	}
}

// N02 generic struct instance embedding a generic that embeds the incomplete interface.
type IS interface{ fmt.Stringer }
type Inner[T any] struct {
	IS
	v T
}
type G[T any] struct {
	Inner[T]
	Mid2
}
type Mid2 struct{ Mid }

func N02GenericEmbedding(ctx context.Context, db *sql.DB, g G[int]) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = g.String()
	}
}

// N03 recursive type with an incomplete embedded interface.
type Rec struct {
	*Rec
	IS
	Mid
}

func N03Recursive(ctx context.Context, db *sql.DB, r Rec) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = r.String()
	}
}

// N04 embedded pointer to a struct that embeds a defined type over an unresolved interface.
type DS fmt.Stringer
type Holder struct{ DS }
type O4 struct {
	*Holder
	Mid2
}

func N04PtrToHolder(ctx context.Context, db *sql.DB, o O4) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = o.String()
	}
}

// N05 method expression on a type with an unresolved embedding.
func N05MethodExpr(ctx context.Context, db *sql.DB, o O1) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = O1.String(o)
	}
}

// N06 struct embedding a type from a GOOS-suffixed-only package declaration.
func N06WindowsSuffix(ctx context.Context, db *sql.DB, w win.W) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = w.Load()
	}
}

// N07 embedded interface from a package that is unreliable (ignored file).
type O7 struct {
	bad.I
	Mid2
}

func N07UnreliablePkgIface(ctx context.Context, db *sql.DB, o O7) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = o.String()
	}
}

// N08 embedded type from a vendored package (vendor is skipped).
type O8 struct {
	p.I
	Mid2
}

func N08Vendored(ctx context.Context, db *sql.DB, o O8) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = o.String()
	}
}

// N09 init-time global func var bound to a pure method value.
var hook func() error

func init() { hook = Deep{}.Load }
func N09InitHook(ctx context.Context, db *sql.DB) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = hook()
	}
}

// N10 generic receiver instantiated with an unresolved type argument; method set from Mid only.
type GX[T any] struct {
	Mid
	v T
}

func N10GenericUnresolvedArg(ctx context.Context, db *sql.DB, g GX[fmt.Stringer]) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = g.String()
	}
}
