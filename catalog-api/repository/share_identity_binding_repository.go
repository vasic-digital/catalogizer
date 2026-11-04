package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"catalogizer/database"
	"catalogizer/models"
)

// ShareIdentityBindingRepository persists the remembered working (host, share,
// identity) bindings discovered during SMB probing (§ identity-share-discovery
// epic, pillar 4). It is the persistence core of "we remember the combinations
// of shares and identity which work together".
//
// SECURITY (§11.4.10): the repository stores only identity_index and
// identity_label. It MUST NEVER receive or store a password/token/secret.
type ShareIdentityBindingRepository struct {
	db *database.DB
}

// NewShareIdentityBindingRepository creates a new binding repository.
func NewShareIdentityBindingRepository(db *database.DB) *ShareIdentityBindingRepository {
	return &ShareIdentityBindingRepository{db: db}
}

// Upsert records a working binding, creating it if absent or updating
// identity fields + last_ok_at + updated_at if a row for (host, share_name,
// protocol) already exists. Idempotent on the unique key.
//
// Uses INSERT OR IGNORE + UPDATE rather than ON CONFLICT ... DO UPDATE
// because the go-sqlcipher SQLite backend does not support the standard
// UPSERT syntax (SQLite < 3.24.0 compatibility).
func (r *ShareIdentityBindingRepository) Upsert(ctx context.Context, b *models.ShareIdentityBinding) error {
	now := time.Now().UTC()

	// Step 1: INSERT OR IGNORE — no-op if the unique key already exists.
	insertQuery := `INSERT OR IGNORE INTO share_identity_bindings
		(host, share_name, protocol, identity_index, identity_label, last_ok_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`

	res, err := r.db.ExecContext(ctx, insertQuery,
		b.Host, b.ShareName, b.Protocol,
		b.IdentityIndex, b.IdentityLabel,
		now, now, now,
	)
	if err != nil {
		return fmt.Errorf("upsert-insert share_identity_binding (%s/%s/%s): %w",
			b.Host, b.ShareName, b.Protocol, err)
	}

	rows, _ := res.RowsAffected()
	if rows > 0 {
		// INSERT succeeded (new row) — done.
		b.UpdatedAt = now
		return nil
	}

	// Step 2: row already exists — UPDATE the identity and timestamp fields.
	updateQuery := `UPDATE share_identity_bindings SET
		identity_index = ?,
		identity_label = ?,
		last_ok_at = ?,
		updated_at = ?
		WHERE host = ? AND share_name = ? AND protocol = ?`

	_, err = r.db.ExecContext(ctx, updateQuery,
		b.IdentityIndex, b.IdentityLabel, now, now,
		b.Host, b.ShareName, b.Protocol,
	)
	if err != nil {
		return fmt.Errorf("upsert-update share_identity_binding (%s/%s/%s): %w",
			b.Host, b.ShareName, b.Protocol, err)
	}

	b.UpdatedAt = now
	return nil
}

// GetWorkingForHost returns all bindings for a given host, ordered by
// last_ok_at descending (most-recently-confirmed first).
func (r *ShareIdentityBindingRepository) GetWorkingForHost(ctx context.Context, host string) ([]models.ShareIdentityBinding, error) {
	query := `SELECT id, host, share_name, protocol, identity_index, identity_label,
		last_ok_at, created_at, updated_at
		FROM share_identity_bindings
		WHERE host = ?
		ORDER BY last_ok_at DESC`

	rows, err := r.db.QueryContext(ctx, query, host)
	if err != nil {
		return nil, fmt.Errorf("query bindings for host %s: %w", host, err)
	}
	defer rows.Close()

	return scanBindings(rows)
}

// List returns all bindings ordered by last_ok_at descending.
func (r *ShareIdentityBindingRepository) List(ctx context.Context) ([]models.ShareIdentityBinding, error) {
	query := `SELECT id, host, share_name, protocol, identity_index, identity_label,
		last_ok_at, created_at, updated_at
		FROM share_identity_bindings
		ORDER BY last_ok_at DESC`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list bindings: %w", err)
	}
	defer rows.Close()

	return scanBindings(rows)
}

// GetByID returns a single binding by its primary key.
func (r *ShareIdentityBindingRepository) GetByID(ctx context.Context, id int64) (*models.ShareIdentityBinding, error) {
	query := `SELECT id, host, share_name, protocol, identity_index, identity_label,
		last_ok_at, created_at, updated_at
		FROM share_identity_bindings
		WHERE id = ?`

	row := r.db.QueryRowContext(ctx, query, id)
	b, err := scanBinding(row)
	if err != nil {
		return nil, fmt.Errorf("get binding %d: %w", id, err)
	}
	return b, nil
}

// Delete removes a binding by id. It is a no-op if the id does not exist.
func (r *ShareIdentityBindingRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM share_identity_bindings WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete binding %d: %w", id, err)
	}
	return nil
}

// scanBindings scans all rows from a binding query into a slice.
func scanBindings(rows *sql.Rows) ([]models.ShareIdentityBinding, error) {
	out := []models.ShareIdentityBinding{}
	for rows.Next() {
		var b models.ShareIdentityBinding
		if err := rows.Scan(
			&b.ID, &b.Host, &b.ShareName, &b.Protocol,
			&b.IdentityIndex, &b.IdentityLabel,
			&b.LastOKAt, &b.CreatedAt, &b.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan binding row: %w", err)
		}
		out = append(out, b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate binding rows: %w", err)
	}
	return out, nil
}

// scanBinding scans a single row into a models.ShareIdentityBinding.
func scanBinding(row *sql.Row) (*models.ShareIdentityBinding, error) {
	var b models.ShareIdentityBinding
	if err := row.Scan(
		&b.ID, &b.Host, &b.ShareName, &b.Protocol,
		&b.IdentityIndex, &b.IdentityLabel,
		&b.LastOKAt, &b.CreatedAt, &b.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &b, nil
}
