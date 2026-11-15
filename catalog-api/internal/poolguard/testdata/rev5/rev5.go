package rev5

import (
	"context"
	"database/sql"

	"catalogizer/internal/poolguard/testdata/rev5/dup"
	"catalogizer/internal/poolguard/testdata/rev5/emb2"
	"catalogizer/internal/poolguard/testdata/rev5/fx2"
	"catalogizer/internal/poolguard/testdata/rev5/tag3"
)

type Repo struct{ db *sql.DB }

func (r *Repo) Foo(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, "UPDATE x")
	return err
}

func (r *Repo) Load(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, "UPDATE l")
	return err
}

func (r *Repo) Close() error {
	_, err := r.db.Exec("UPDATE c")
	return err
}

func cur(ctx context.Context, db *sql.DB) *sql.Rows { r, _ := db.QueryContext(ctx, "SELECT"); return r }

// R35: build-tagged package whose FIRST file (linux) declares a concrete T with a pure Foo; on other
// platforms T is an interface. The package is not reliable for typing: must stay flagged.
func R35TaggedStructFirst(ctx context.Context, db *sql.DB, x tag3.T) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = x.Foo(ctx, 1)
	}
}

// Wrap is an in-tree wrapper whose ExecContext is pure (closed); the scanner's direct detection is by
// method name, so a call to it counts as pooled.
type Wrap struct{}

func (Wrap) ExecContext(ctx context.Context, q string) error { return nil }

// Checker is a same-package concrete type whose closed Check reaches the pooled-named Wrap.ExecContext.
type Checker struct{ w Wrap }

func (c Checker) Check(ctx context.Context) error { return c.w.ExecContext(ctx, "UPDATE") }

// R36: a closed SAME-package method is followed exactly; it reaches a pooled-named call: flagged.
func R36SamePkgExactReachesPooled(ctx context.Context, db *sql.DB, c Checker) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = c.Check(ctx)
	}
}

// R37: promoted method through an unresolvable embedded interface: the scanner must not trust the
// depth-two concrete Inner.Close it can see. Must stay flagged (Repo.Close is pooled).
func R37InvalidEmbeddingShadow(ctx context.Context, db *sql.DB, s emb2.S) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = s.Close()
	}
}

// R38: other-package concrete method that ranges over a func-typed field (callback): stays flagged.
func R38RangeOverFuncCallback(ctx context.Context, db *sql.DB, it fx2.Iter) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = it.Load(ctx, 1)
	}
}

// R39: other-package concrete method whose helper calls a func value (open one level down): flagged.
func R39OpenOneLevelDown(ctx context.Context, db *sql.DB, b fx2.Box) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = b.Load(ctx, 1)
	}
}

// R39b: other-package concrete method that is closed through a static helper: sanctioned removal.
func R39bClosedThroughHelper(ctx context.Context, db *sql.DB, p fx2.Pure) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = p.Load(ctx, 1)
	}
}

func h0(ctx context.Context) error { return h1(ctx) }
func h1(ctx context.Context) error { return h2(ctx) }
func h2(ctx context.Context) error { return h3(ctx) }
func h3(ctx context.Context) error {
	_, err := (&Repo{}).db.ExecContext(ctx, "UPDATE h")
	return err
}
func g0(ctx context.Context) error { return h0(ctx) }

// R40: pooled call exactly four helper levels down (the deepest the bound allows): flagged.
func R40DepthBound(ctx context.Context, db *sql.DB) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = h0(ctx)
	}
}

// R41: one level deeper than the bound: flagged by neither scanner.
func R41BeyondBound(ctx context.Context, db *sql.DB) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = g0(ctx)
	}
}

// R42: a package with a redeclared type (broken, untagged) is not trusted for typing: stays flagged.
func R42RedeclaredType(ctx context.Context, db *sql.DB, x dup.T) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = x.Foo(ctx, 1)
	}
}
