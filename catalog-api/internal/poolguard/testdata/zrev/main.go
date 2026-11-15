package zrev

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"

	"catalogizer/internal/poolguard/testdata/zrev/gn"
	"catalogizer/internal/poolguard/testdata/zrev/lib"
)

type Repo struct{ db *sql.DB }

// String, Read and Load are DB-backed (pooled).
func (r *Repo) String() string {
	_, _ = r.db.ExecContext(context.Background(), "UPDATE s")
	return ""
}
func (r *Repo) Read(p []byte) (int, error) {
	_, err := r.db.ExecContext(context.Background(), "UPDATE r")
	return 0, err
}
func (r *Repo) Load() error {
	_, err := r.db.ExecContext(context.Background(), "UPDATE l")
	return err
}

func cur(ctx context.Context, db *sql.DB) *sql.Rows { r, _ := db.QueryContext(ctx, "SELECT"); return r }

// Z01: cross-package, struct embeds an in-tree interface that embeds an unresolved interface.
func Z01IfaceEmbedsUnresolved(ctx context.Context, db *sql.DB, o lib.Outer) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = o.String()
	}
}

// Same-package variants.
type I interface{ fmt.Stringer }
type Deep struct{}

func (Deep) String() string             { return "pure" }
func (Deep) Read(p []byte) (int, error) { return 0, nil }

type Mid struct{ Deep }
type Outer struct {
	I
	Mid
}

// Z01b: same package; real dispatch is I.String (e.g. a *Repo held in I) -> pooled.
func Z01bSamePkg(ctx context.Context, db *sql.DB, o Outer) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = o.String()
	}
}

// Z05: defined type over an unresolved struct type; it inherits promoted methods (Read via *bufio.Reader,
// which calls the wrapped io.Reader, e.g. *Repo) that the checker cannot see.
type Bad bufio.ReadWriter
type Mid2 struct{ Mid }
type Outer5 struct {
	Bad
	Mid2
}

func Z05DefinedOverUnresolved(ctx context.Context, db *sql.DB, o Outer5) {
	rows := cur(ctx, db)
	for rows.Next() {
		_, _ = o.Read(nil)
	}
}

// Z06: in-tree interface declared via a defined type over an unresolved interface.
type J fmt.Stringer
type Outer6 struct {
	J
	Mid
}

func Z06DefinedIfaceOverUnresolved(ctx context.Context, db *sql.DB, o Outer6) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = o.String()
	}
}

// Z07: generic instantiation with a pure method; static, closed (expected sanctioned-class removal).
func Z07GenericPure(ctx context.Context, db *sql.DB, b gn.Box[int]) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = b.Load()
	}
}

// Z08: generic method whose body calls the type argument's method (open): must stay flagged.
func Z08GenericOpen(ctx context.Context, db *sql.DB, b gn.Call[*Repo]) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = b.Load()
	}
}

// Z09: method value bound and passed to a same-package helper that calls it.
func run(f func() error) error { return f() }
func Z09BoundMethodValueArg(ctx context.Context, db *sql.DB, d Deep, r *Repo) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = run(r.Load)
	}
}

// Z10: closure stored in a struct field then invoked.
type holder struct{ f func() error }

func Z10ClosureInField(ctx context.Context, db *sql.DB, r *Repo) {
	h := holder{f: r.Load}
	rows := cur(ctx, db)
	for rows.Next() {
		_ = h.f()
	}
}

// Z11: method on a named func type that calls the receiver.
type Fn func() error

func (f Fn) Load() error { return f() }
func Z11NamedFuncType(ctx context.Context, db *sql.DB, r *Repo) {
	f := Fn(r.Load)
	rows := cur(ctx, db)
	for rows.Next() {
		_ = f.Load()
	}
}

// Z12: deferred / goroutine call of a pure static method shadowing a pooled name (expected removal).
func Z12DeferGo(ctx context.Context, db *sql.DB, d Deep) {
	rows := cur(ctx, db)
	for rows.Next() {
		go func() { _ = d.String() }()
		defer d.String()
	}
}
