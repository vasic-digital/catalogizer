// Package rev6 holds the class-wide fixtures for an incomplete method set (round-1 finding F1): every
// way an unresolved type can contribute methods to a receiver. fmt and bufio are not resolved by the
// scanner's importer (only the importing module's own packages are), so their types are invalid here.
package rev6

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
)

type Repo struct{ db *sql.DB }

// String is DB-backed (pooled): by name, every String call below reaches it.
func (r *Repo) String() string {
	_, _ = r.db.ExecContext(context.Background(), "UPDATE s")
	return ""
}

func cur(ctx context.Context, db *sql.DB) *sql.Rows { r, _ := db.QueryContext(ctx, "SELECT"); return r }

type Deep struct{}

func (Deep) String() string { return "pure" }

type Mid struct{ Deep }

// R44: an alias of an unresolved interface, embedded beside Mid. A real build promotes the alias'
// String (depth 1), which shadows Mid's Deep.String (depth 2).
type A = fmt.Stringer
type Outer44 struct {
	A
	Mid
}

func R44AliasOfUnresolved(ctx context.Context, db *sql.DB, o Outer44) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = o.String()
	}
}

// R45: a generic struct instantiated over int that embeds an unresolved interface.
type GW[T any] struct {
	fmt.Stringer
	Mid
	v T
}

func R45GenericInstantiation(ctx context.Context, db *sql.DB, o GW[int]) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = o.String()
	}
}

// R46: a pointer to a defined type whose underlying type is unresolved (invalid underlying).
type Bad bufio.ReadWriter
type Outer46 struct {
	*Bad
	Mid
}

func R46PtrDefinedOverUnresolved(ctx context.Context, db *sql.DB, o Outer46) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = o.String()
	}
}

// R47: a defined type over a struct whose embedded field is unresolved.
type Inc struct {
	fmt.Stringer
	Mid
}
type N47 Inc

func R47NamedOverIncompleteStruct(ctx context.Context, db *sql.DB, o N47) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = o.String()
	}
}

// R48: a constraint whose type set embeds an unresolved interface and a union term.
type J interface {
	fmt.Stringer
	~struct{ Mid } | ~struct{}
}

// R48: a type parameter constrained by an interface over an unresolved one: always by name.
func R48ConstraintOverUnresolved[T J](ctx context.Context, db *sql.DB, v T) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = v.String()
	}
}

// R49 (control, sanctioned): a non-embedded field of an unresolved type does not contribute methods,
// and an in-tree, fully resolved embedded interface without String does not shadow Mid's String.
// The method set is complete, so the static Deep.String is trusted.
type Named interface{ Name() string }
type Outer49 struct {
	Named
	Mid
	w *bufio.Writer
}

func R49CompleteDespiteUnresolvedField(ctx context.Context, db *sql.DB, o Outer49) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = o.String()
	}
}
