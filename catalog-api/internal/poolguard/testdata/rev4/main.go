package rev4

import (
	"context"
	"database/sql"

	bu "catalogizer/internal/poolguard/testdata/rev4/b/util"
	"catalogizer/internal/poolguard/testdata/rev4/conc"
	"catalogizer/internal/poolguard/testdata/rev4/emb"
	"catalogizer/internal/poolguard/testdata/rev4/fx"
	g "catalogizer/internal/poolguard/testdata/rev4/grp"
	u "catalogizer/internal/poolguard/testdata/rev4/ifc"
	"catalogizer/internal/poolguard/testdata/rev4/tagged"
)

type Repo struct{ db *sql.DB }

func (r *Repo) Save(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, "UPDATE x")
	return err
}
func (r *Repo) Foo(ctx context.Context, id int) error {
	_, err := r.db.ExecContext(ctx, "UPDATE y")
	return err
}
func (r *Repo) Do(ctx context.Context, id int) error   { return r.Foo(ctx, id) }
func (r *Repo) Load(ctx context.Context, id int) error { return r.Foo(ctx, id) }
func LoadTags(ctx context.Context, id int) []string {
	rows, _ := (&Repo{}).db.QueryContext(ctx, "SELECT")
	_ = rows
	return nil
}

func cur(ctx context.Context, db *sql.DB) *sql.Rows { r, _ := db.QueryContext(ctx, "SELECT"); return r }

// R01: func-typed field of an in-tree concrete struct; h.Save may hold (*Repo).Save.
func R01FuncFieldMethodValue(ctx context.Context, db *sql.DB, h fx.Hooks) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = h.Save(ctx, 1)
	}
}

// R20: func-typed field named like a same-package pooled function.
func R20FuncFieldPlainFunc(ctx context.Context, db *sql.DB, h fx.Hooks) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = h.LoadTags(ctx, 1)
	}
}

// R01b: func field via composite literal local.
func R01bFuncFieldLocal(ctx context.Context, db *sql.DB, r *Repo) {
	h := &fx.Hooks{Save: r.Save}
	rows := cur(ctx, db)
	for rows.Next() {
		_ = h.Save(ctx, 1)
	}
}

// R09: named func type method (method body in fx calls the func value).
func R09NamedFuncTypeMethod(ctx context.Context, db *sql.DB, f fx.Fn) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = f.Do(ctx, 1)
	}
}

// R10: slice type method in fx (pure).
func R10SliceTypeMethod(ctx context.Context, db *sql.DB, l fx.L) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = l.Load(ctx, 1)
	}
}

// R06: interface declared in a grouped decl in a second file of the imported package (renamed import).
func R06GroupedIfaceSecondFile(ctx context.Context, db *sql.DB, x g.I) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = x.Foo(ctx, 1)
	}
}

// R07: same package name "util" in two dirs; b/util.I is an interface (a/util.I a struct).
func R07SamePkgNameOtherDir(ctx context.Context, db *sql.DB, x bu.I) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = x.Foo(ctx, 1)
	}
}

// R04: build-tagged files: T is a struct on linux, an interface elsewhere.
func R04BuildTaggedKinds(ctx context.Context, db *sql.DB, x tagged.T) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = callFoo(ctx, x)
	}
}

func callFoo(ctx context.Context, x interface{}) error { return nil }

// R04b: direct use of the tagged type as receiver (non-linux build: interface).
func R04bBuildTaggedRecv(ctx context.Context, db *sql.DB, x tagged.T) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = any(x)
	}
}

// R08: struct embedding a pointer to a struct that embeds an interface.
func R08EmbedPtrToEmbedIface(ctx context.Context, db *sql.DB, w emb.W) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = w.Foo(ctx, 1)
	}
}

// R14: interface satisfied by *Repo, held in an interface-typed local from another package.
func R14IfaceHoldsPtrRepo(ctx context.Context, db *sql.DB, r *Repo) {
	var i u.T = r
	rows := cur(ctx, db)
	for rows.Next() {
		_ = i.Foo(ctx, 1)
	}
}

// R12: param whose name equals an import name ("u" shadows import u inside the body).
func R12ParamShadowsImport(ctx context.Context, db *sql.DB, u conc.T, z interface {
	Foo(context.Context, int) error
}) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = z.Foo(ctx, 1)
	}
}

// R13: method value of an interface bound before the loop.
func R13MethodValue(ctx context.Context, db *sql.DB, x u.T) {
	f := x.Foo
	rows := cur(ctx, db)
	for rows.Next() {
		_ = f(ctx, 1)
	}
}

// R15: generic with concrete-ish constraint.
func R15GenericConstraint[T interface {
	Foo(context.Context, int) error
}](ctx context.Context, db *sql.DB, x T) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = x.Foo(ctx, 1)
	}
}

// R28: selector chain receiver (field of interface type).
func R28FieldChain(ctx context.Context, db *sql.DB, n emb.N) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = n.X.Foo(ctx, 1)
	}
}

// R29: local shadow in nested block: concrete outer, interface inner.
func R29BlockShadow(ctx context.Context, db *sql.DB, y u.T) {
	x := conc.T{}
	_ = x
	rows := cur(ctx, db)
	for rows.Next() {
		x := y
		_ = x.Foo(ctx, 1)
	}
}

// R30: type switch binding.
func R30TypeSwitch(ctx context.Context, db *sql.DB, v any) {
	rows := cur(ctx, db)
	for rows.Next() {
		switch x := v.(type) {
		case u.T:
			_ = x.Foo(ctx, 1)
		}
	}
}

// R31: x declared concrete via var, then reassigned through an interface field elsewhere: same type only.
// Helper indirection: concrete-typed local in a helper that itself forwards to a pooled call.
func R31HelperForwards(ctx context.Context, db *sql.DB, r *Repo) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = fwd(ctx, r)
	}
}

func fwd(ctx context.Context, r *Repo) error {
	h := fx.Hooks{Save: r.Save}
	return h.Save(ctx, 2)
}

// R11: loop in main.go on u.T (interface here) must stay flagged.
func R11CollideIface(ctx context.Context, db *sql.DB, x u.T) {
	rows := cur(ctx, db)
	for rows.Next() {
		_ = x.Foo(ctx, 1)
	}
}
