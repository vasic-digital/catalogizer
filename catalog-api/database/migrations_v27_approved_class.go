package database

import (
	"context"
	"fmt"
)

// addApprovedClassColumns (migration v27, CAT-SECQ1-B / W3) adds the per-endpoint address-class approval to the two tables
// that hold operator-chosen outbound destinations: storage_roots and sync_endpoints.
//
//	approved_class         'private' (default) or 'public': the most internal class the endpoint's transport may reach
//	                       (netguard.Class: private permits private and public addresses, public permits public only;
//	                       'never' is not storable, it approves nothing).
//	approved_class_source  'default' (never classified; every row that existed before this migration), 'auto'
//	                       (derived by netguard.ClassifyEndpoint) or 'operator' (set or confirmed by an operator; an
//	                       automatic classification never overwrites it).
//
// Existing rows keep the behaviour they had: private/default. The migration does not resolve DNS names or read the host's
// interface list (the result would depend on the machine and the moment the migration runs); classification of existing rows
// is an explicit step (AutoClassifyApprovedClass) so that it is visible and repeatable.
func (db *DB) addApprovedClassColumns(ctx context.Context) error {
	for _, table := range approvedClassTables {
		if err := db.addApprovedClassColumnsTo(ctx, table); err != nil {
			return err
		}
	}
	return nil
}

const (
	approvedClassColumnDDL  = `approved_class TEXT NOT NULL DEFAULT 'private' CHECK (approved_class IN ('private', 'public'))`
	approvedSourceColumnDDL = `approved_class_source TEXT NOT NULL DEFAULT 'default' CHECK (approved_class_source IN ('default', 'auto', 'operator'))`
)

func (db *DB) addApprovedClassColumnsTo(ctx context.Context, table string) error {
	if db.dialect.IsPostgres() {
		for _, ddl := range []string{approvedClassColumnDDL, approvedSourceColumnDDL} {
			if _, err := db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN IF NOT EXISTS `+ddl); err != nil {
				return fmt.Errorf("postgres: %s approved class columns: %w", table, err)
			}
		}
		return nil
	}
	for _, c := range []struct{ name, ddl string }{
		{"approved_class", approvedClassColumnDDL},
		{"approved_class_source", approvedSourceColumnDDL},
	} {
		has, err := db.columnExistsSQLite(ctx, table, c.name)
		if err != nil {
			return fmt.Errorf("sqlite: probe %s.%s: %w", table, c.name, err)
		}
		if has {
			continue
		}
		if _, err := db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+c.ddl); err != nil {
			return fmt.Errorf("sqlite: add %s.%s: %w", table, c.name, err)
		}
	}
	return nil
}
