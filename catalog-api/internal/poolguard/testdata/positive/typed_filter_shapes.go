package fixture

import (
	"context"
	"database/sql"
)

// RepoR owns the DB-backed Check (a pooled query); PureR owns a pure same-named Check.
type RepoR struct{ db *sql.DB }

func (r *RepoR) Check(ctx context.Context, id int) bool {
	var ok bool
	_ = r.db.QueryRowContext(ctx, "SELECT 1 FROM p WHERE id = ?", id).Scan(&ok)
	return ok
}

type PureR []string

func (p PureR) Check(ctx context.Context, id int) bool { return len(p) > id }

// Checker is a same-package interface: it owns no method declarations.
type Checker interface {
	Check(ctx context.Context, id int) bool
}

// Embeds *RepoR: Check is promoted.
type EmbedSvc struct {
	*RepoR
	db *sql.DB
}

type Wrap struct{ *RepoR }

type AliasR = RepoR

func openCursor(ctx context.Context, db *sql.DB) (*sql.Rows, error) {
	return db.QueryContext(ctx, "SELECT id FROM p")
}

// (1) range var shadows a pure-typed p of the enclosing scope.
func RangeShadow(ctx context.Context, db *sql.DB, repos []*RepoR) error {
	var p PureR
	_ = p
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		for _, p := range repos {
			p.Check(ctx, id)
		}
	}
	return nil
}

// (2) parameter of a same-package interface type.
func IfaceParam(ctx context.Context, db *sql.DB, c Checker) error {
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		c.Check(ctx, id)
	}
	return nil
}

// (3) receiver embeds *RepoR: promoted method.
func (s *EmbedSvc) PromotedRecv(ctx context.Context) error {
	rows, err := openCursor(ctx, s.db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		s.Check(ctx, id)
	}
	return nil
}

// (4) local composite literal of a type embedding RepoR.
func EmbeddedLocal(ctx context.Context, db *sql.DB, r *RepoR) error {
	w := &Wrap{RepoR: r}
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		w.Check(ctx, id)
	}
	return nil
}

// (5) alias type.
func AliasLocal(ctx context.Context, db *sql.DB, r *RepoR) error {
	a := &AliasR{db: db}
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		a.Check(ctx, id)
	}
	return nil
}

// (6) closure parameter shadows an outer pure-typed variable.
func ClosureShadow(ctx context.Context, db *sql.DB, r *RepoR) error {
	var p PureR
	_ = p
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		func(p *RepoR) { p.Check(ctx, id) }(r)
	}
	return nil
}

// (7) one identifier bound to two types: pure first, then DB-backed (kills "keep first type").
func TwoTypes(ctx context.Context, db *sql.DB, r *RepoR) error {
	x := PureR{}
	_ = x
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		_ = rows.Scan(&id)
		x := &RepoR{db: db}
		x.Check(ctx, id)
	}
	return nil
}
