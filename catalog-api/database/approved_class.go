package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"catalogizer/internal/netguard"
)

// Tables that carry approved_class / approved_class_source (migration v27). The accessors below only accept these names:
// the table name is interpolated into the statement, so it is never taken from input.
const (
	TableStorageRoots  = "storage_roots"
	TableSyncEndpoints = "sync_endpoints"
)

var approvedClassTables = []string{TableStorageRoots, TableSyncEndpoints}

// Sources of a stored approval.
const (
	ApprovedSourceDefault  = "default"
	ApprovedSourceAuto     = "auto"
	ApprovedSourceOperator = "operator"
)

// ErrApprovedClassInvalid is returned for an unknown table, class or source.
var ErrApprovedClassInvalid = errors.New("invalid approved class request")

// ErrEndpointNotFound is returned when no row has the given id.
var ErrEndpointNotFound = errors.New("endpoint not found")

func checkApprovedTable(table string) error {
	for _, t := range approvedClassTables {
		if t == table {
			return nil
		}
	}
	return fmt.Errorf("%w: table %q", ErrApprovedClassInvalid, table)
}

// GetApprovedClass returns the stored class and its source for row id of table (migration v27).
func (db *DB) GetApprovedClass(ctx context.Context, table string, id int64) (netguard.Class, string, error) {
	if err := checkApprovedTable(table); err != nil {
		return netguard.ClassNever, "", err
	}
	var cls, src string
	err := db.QueryRowContext(ctx, `SELECT approved_class, approved_class_source FROM `+table+` WHERE id = ?`, id).Scan(&cls, &src)
	if errors.Is(err, sql.ErrNoRows) {
		return netguard.ClassNever, "", fmt.Errorf("%w: %s id %d", ErrEndpointNotFound, table, id)
	}
	if err != nil {
		return netguard.ClassNever, "", err
	}
	c, ok := netguard.ParseClass(cls)
	if !ok || c == netguard.ClassNever {
		return netguard.ClassNever, "", fmt.Errorf("%w: stored class %q of %s id %d", ErrApprovedClassInvalid, cls, table, id)
	}
	return c, src, nil
}

// SetApprovedClass stores an approval. Only private and public are storable (never approves nothing), and source must be
// one of default, auto, operator.
func (db *DB) SetApprovedClass(ctx context.Context, table string, id int64, class netguard.Class, source string) error {
	if err := checkApprovedTable(table); err != nil {
		return err
	}
	if class != netguard.ClassPrivate && class != netguard.ClassPublic {
		return fmt.Errorf("%w: class %s is not storable", ErrApprovedClassInvalid, class)
	}
	switch source {
	case ApprovedSourceDefault, ApprovedSourceAuto, ApprovedSourceOperator:
	default:
		return fmt.Errorf("%w: source %q", ErrApprovedClassInvalid, source)
	}
	res, err := db.ExecContext(ctx, `UPDATE `+table+` SET approved_class = ?, approved_class_source = ? WHERE id = ?`, class.String(), source, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("%w: %s id %d", ErrEndpointNotFound, table, id)
	}
	return nil
}

// AutoClassifyApprovedClass classifies the endpoint (host, or the host of rawURL when host is empty) with
// netguard.ClassifyEndpoint and stores the result with source 'auto'. A row whose source is 'operator' is never changed:
// the stored class is returned. When classification fails (never-class address, unresolvable name, bad spelling) nothing
// is written and the error is returned: the row keeps its stored class.
func (db *DB) AutoClassifyApprovedClass(ctx context.Context, table string, id int64, host, rawURL string) (netguard.Class, error) {
	cur, src, err := db.GetApprovedClass(ctx, table, id)
	if err != nil {
		return netguard.ClassNever, err
	}
	if src == ApprovedSourceOperator {
		return cur, nil
	}
	c, err := netguard.ClassifyEndpoint(ctx, host, rawURL)
	if err != nil {
		return cur, err
	}
	if err := db.SetApprovedClass(ctx, table, id, c, ApprovedSourceAuto); err != nil {
		return cur, err
	}
	return c, nil
}
