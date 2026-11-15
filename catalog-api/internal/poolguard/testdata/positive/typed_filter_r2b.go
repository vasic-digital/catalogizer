package fixture

import (
	"context"
	"database/sql"

	"catalogizer/internal/poolguard/testdata/positive/ext"
)

var pkgRepoB = &RepoR{}

// package-level var sharedP; PureR-typed only in an unrelated sibling block.
var sharedP = &RepoR{}

// S15: parameter typed with an interface declared in another package.
func ExtIfaceParam(ctx context.Context, db *sql.DB, x ext.Checker) error {
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

// S16
func ExtIfaceVar(ctx context.Context, db *sql.DB) error {
	var x ext.Checker = pkgRepoB
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

func helperExt(ctx context.Context, c ext.Checker) { c.Check(ctx, 1) }

// S23: reached through a helper.
func ExtIfaceViaHelper(ctx context.Context, db *sql.DB) error {
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		helperExt(ctx, pkgRepoB)
	}
	return nil
}

// S17: package-level p used in the loop; PureR p only in a sibling block.
func PkgVarSiblingShadow(ctx context.Context, db *sql.DB) error {
	{
		var sharedP PureR
		_ = sharedP
	}
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		sharedP.Check(ctx, 1)
	}
	return nil
}

// S22: shadowing p only inside a never-called closure.
func ClosureLocalShadowsPkgVar(ctx context.Context, db *sql.DB) error {
	_ = func() { sharedP := PureR{}; _ = sharedP }
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		sharedP.Check(ctx, 1)
	}
	return nil
}

// M6: pooled binding first, pure same-name binding later (conflict must stay unknown).
func PooledFirstPureLater(ctx context.Context, db *sql.DB) error {
	z := &RepoR{db: db}
	rows, err := openCursor(ctx, db)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		z.Check(ctx, 1)
	}
	{
		z := PureR{}
		_ = z
	}
	return nil
}
