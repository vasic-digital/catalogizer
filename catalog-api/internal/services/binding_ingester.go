// SPDX-FileCopyrightText: 2026 Milos Vasic
// SPDX-License-Identifier: Apache-2.0

package services

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"catalogizer/database"
	"catalogizer/internal/netguard"
	"catalogizer/models"
	"catalogizer/repository"

	"go.uber.org/zap"
)

// BindingIngester takes an SMBProbeResult and automatically creates storage_root
// entries for each working share, persisting the identity binding so future
// discovery can skip re-probing every configured identity against the same
// (host, share) pair.
//
// SECURITY (§11.4.10): the ingester stores only identity_index (an integer index
// into the gitignored CATALOGIZER_IDENTITY_N_* env vars) in the storage_root's
// Options JSON column — NEVER a password, token, or other secret. The
// identity_index is the lookup key back into the credential configuration, not
// the credential itself.
type BindingIngester struct {
	db          *database.DB
	logger      *zap.Logger
	bindingRepo *repository.ShareIdentityBindingRepository
}

// NewBindingIngester creates a new BindingIngester.
func NewBindingIngester(db *database.DB, logger *zap.Logger) *BindingIngester {
	return &BindingIngester{
		db:          db,
		logger:      logger,
		bindingRepo: repository.NewShareIdentityBindingRepository(db),
	}
}

// IngestionResult summarises what the ingester did in a single call.
type IngestionResult struct {
	BoundShares int `json:"bound_shares"` // number of share_identity_binding rows upserted
	NewRoots    int `json:"new_roots"`    // number of storage_roots inserted (0 if all already existed)
}

// IngestProbeResult accepts a successful SMBProbeResult and creates/updates the
// corresponding storage_root + share_identity_binding records. It is idempotent:
// repeated calls with the same (host, share_name) do not create duplicate rows.
//
// The probe result MUST be authenticated (Authenticated == true) — an
// unauthenticated result has no valid identity binding to persist and will be
// rejected.
func (bi *BindingIngester) IngestProbeResult(ctx context.Context, result *SMBProbeResult) (*IngestionResult, error) {
	return bi.ingest(ctx, result, nil)
}

// ApprovedClassAssignment is the network class (and who decided it) stored on a storage root this ingester creates.
type ApprovedClassAssignment struct {
	Class  netguard.Class
	Source string // database.ApprovedSource*
}

// IngestProbeResultWithClass is IngestProbeResult that also stores the approved network class on every storage root it CREATES (CPN1-M1).
// A root that already exists is left untouched: re-ingesting must neither widen nor narrow an existing approval. If the class cannot be
// stored the new row is removed again and the error returned, so no root is left behind with an approval nobody made.
func (bi *BindingIngester) IngestProbeResultWithClass(ctx context.Context, result *SMBProbeResult, a ApprovedClassAssignment) (*IngestionResult, error) {
	return bi.ingest(ctx, result, &a)
}

func (bi *BindingIngester) ingest(ctx context.Context, result *SMBProbeResult, assign *ApprovedClassAssignment) (*IngestionResult, error) {
	if result == nil {
		return nil, fmt.Errorf("ingester: nil probe result")
	}
	if !result.Authenticated {
		return nil, fmt.Errorf("ingester: probe result for %s is not authenticated — no binding to persist", result.Host)
	}

	var out IngestionResult

	for _, share := range result.Shares {
		bi.logger.Info("Ingesting SMB probe result",
			zap.String("host", result.Host),
			zap.String("share", share.ShareName),
			zap.Int("identity_index", result.IdentityIndex),
		)

		// Step 1 — persist the remembered binding (idempotent via Upsert).
		binding := &models.ShareIdentityBinding{
			Host:          result.Host,
			ShareName:     share.ShareName,
			Protocol:      "smb",
			IdentityIndex: result.IdentityIndex,
			IdentityLabel: result.IdentityLabel,
		}
		if err := bi.bindingRepo.Upsert(ctx, binding); err != nil {
			return nil, fmt.Errorf("ingester: upsert binding for %s/%s: %w",
				result.Host, share.ShareName, err)
		}
		out.BoundShares++

		// Step 2 — insert a storage_root entry for this share if one does not
		// already exist (idempotent by UNIQUE(name)).
		//
		// SECURITY (§11.4.10): the storage_root stores identity_index in the
		// Options JSON column, NEVER in the password (or username) column. The
		// password, username, and domain columns are left NULL so that no code
		// path can accidentally read a credential from the database.
		rootName := rootNameFor(result.Host, share.ShareName)

		created, err := bi.insertStorageRootOnce(ctx, rootName, result.Host, share.ShareName, result.IdentityIndex, assign)
		if err != nil {
			return nil, err
		}
		if created {
			out.NewRoots++
		}
	}

	return &out, nil
}

// insertStorageRootOnce inserts a storage_root iff it does not already exist.
// Returns created=true when a new row was inserted, created=false when the row
// already exists (idempotent). The row stores identity_index in the Options JSON
// column and leaves Username/Password NULL (§11.4.10).
func (bi *BindingIngester) insertStorageRootOnce(ctx context.Context, name, host, sharePath string, identityIdx int, assign *ApprovedClassAssignment) (bool, error) {
	// Check existence first (reliable idempotency: InsertReturningID uses
	// LastInsertId which does NOT return 0 for INSERT OR IGNORE no-ops).
	var existing int64
	err := bi.db.QueryRowContext(ctx,
		"SELECT id FROM storage_roots WHERE name = ? LIMIT 1", name,
	).Scan(&existing)
	if err == nil {
		bi.logger.Debug("Storage root already exists, skipping",
			zap.String("name", name), zap.Int64("id", existing))
		return false, nil
	}

	identityMeta, _ := json.Marshal(map[string]int{"identity_index": identityIdx})
	optionsStr := string(identityMeta)
	now := time.Now().UTC()

	newID, insertErr := bi.db.InsertReturningID(ctx,
		`INSERT INTO storage_roots
			(name, protocol, host, path, domain, options, enabled, max_depth, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, 10, ?, ?)`,
		name, "smb", host, sharePath,
		"WORKGROUP", optionsStr, true, now, now,
	)
	if insertErr != nil {
		return false, fmt.Errorf("ingester: insert storage_root %s: %w", name, insertErr)
	}
	if assign != nil {
		if err := bi.db.SetApprovedClass(ctx, database.TableStorageRoots, newID, assign.Class, assign.Source); err != nil {
			cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			_, _ = bi.db.ExecContext(cctx, `DELETE FROM storage_roots WHERE id = ?`, newID)
			return false, fmt.Errorf("ingester: store approved class for %s: %w", name, err)
		}
	}

	bi.logger.Info("Storage root created from probe result",
		zap.String("name", name),
		zap.String("host", host),
		zap.String("share", sharePath),
		zap.Int("identity_index", identityIdx),
	)
	return true, nil
}

// rootNameFor returns a deterministic human-readable storage-root name for a
// (host, shareName) pair. It is safe for use as a UNIQUE storage_roots.name.
func rootNameFor(host, shareName string) string {
	return fmt.Sprintf("%s:%s", host, shareName)
}
