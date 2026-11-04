package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"catalogizer/database"
)

// ImageQualityAssessment mirrors a row of the image_quality_assessments table.
// It records the last outcome of a quality gate check for a specific cover
// variant, along with which provider supplied the bytes.
type ImageQualityAssessment struct {
	ID            int64
	EntityType    string
	EntityID      int64
	Variant       string
	Source        string
	ProviderRef   string
	Width         int
	Height        int
	BlurVar       float64
	BPP           float64
	AspectRatio   float64
	Verdict       string
	FailReason    string
	Format        string
	CachePath     string
	AttemptCount  int
	AssessedAt    time.Time
	LastCheckedAt time.Time
}

// ImageQualityRepository persists ImageQualityAssessment rows.
type ImageQualityRepository struct {
	db *database.DB
}

// NewImageQualityRepository wires the repository to a database handle.
func NewImageQualityRepository(db *database.DB) *ImageQualityRepository {
	return &ImageQualityRepository{db: db}
}

// ErrNotFound is returned by Find when no matching assessment exists.
var ErrNotFound = errors.New("image quality assessment not found")

// Find returns the most recent assessment for a (entity_type, entity_id, variant).
func (r *ImageQualityRepository) Find(ctx context.Context, entityType string, entityID int64, variant string) (*ImageQualityAssessment, error) {
	if r == nil || r.db == nil {
		return nil, ErrNotFound
	}

	row := r.db.QueryRowContext(ctx, `
		SELECT id, entity_type, entity_id, variant, source, COALESCE(provider_ref,''),
		       width, height, blur_var, bpp, COALESCE(aspect_ratio, 0),
		       verdict, COALESCE(fail_reason,''), COALESCE(format,''), COALESCE(cache_path,''),
		       attempt_count, assessed_at, last_checked_at
		FROM image_quality_assessments
		WHERE entity_type = ? AND entity_id = ? AND variant = ?
		LIMIT 1
	`, entityType, entityID, variant)

	a := &ImageQualityAssessment{}
	err := row.Scan(
		&a.ID, &a.EntityType, &a.EntityID, &a.Variant, &a.Source, &a.ProviderRef,
		&a.Width, &a.Height, &a.BlurVar, &a.BPP, &a.AspectRatio,
		&a.Verdict, &a.FailReason, &a.Format, &a.CachePath,
		&a.AttemptCount, &a.AssessedAt, &a.LastCheckedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan image quality assessment: %w", err)
	}
	return a, nil
}

// Upsert writes a new assessment or replaces the existing one for the same
// (entity_type, entity_id, variant), incrementing attempt_count on replace.
func (r *ImageQualityRepository) Upsert(ctx context.Context, a *ImageQualityAssessment) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("image quality repository not configured")
	}

	existing, err := r.Find(ctx, a.EntityType, a.EntityID, a.Variant)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return fmt.Errorf("lookup existing assessment: %w", err)
	}

	now := time.Now()
	if existing == nil {
		_, insErr := r.db.ExecContext(ctx, `
			INSERT INTO image_quality_assessments
			(entity_type, entity_id, variant, source, provider_ref, width, height,
			 blur_var, bpp, aspect_ratio, verdict, fail_reason, format, cache_path,
			 attempt_count, assessed_at, last_checked_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			a.EntityType, a.EntityID, a.Variant, a.Source, a.ProviderRef,
			a.Width, a.Height, a.BlurVar, a.BPP, a.AspectRatio,
			a.Verdict, a.FailReason, a.Format, a.CachePath,
			1, now, now,
		)
		if insErr != nil {
			return fmt.Errorf("insert image quality assessment: %w", insErr)
		}
		return nil
	}

	_, err = r.db.ExecContext(ctx, `
		UPDATE image_quality_assessments
		SET source = ?, provider_ref = ?, width = ?, height = ?,
		    blur_var = ?, bpp = ?, aspect_ratio = ?,
		    verdict = ?, fail_reason = ?, format = ?, cache_path = ?,
		    attempt_count = attempt_count + 1,
		    assessed_at = ?, last_checked_at = ?
		WHERE id = ?
	`,
		a.Source, a.ProviderRef, a.Width, a.Height,
		a.BlurVar, a.BPP, a.AspectRatio,
		a.Verdict, a.FailReason, a.Format, a.CachePath,
		now, now, existing.ID,
	)
	if err != nil {
		return fmt.Errorf("update image quality assessment: %w", err)
	}
	return nil
}

// TouchLastChecked updates last_checked_at for a row without altering verdict
// or measurements. Used by the background revalidation worker.
func (r *ImageQualityRepository) TouchLastChecked(ctx context.Context, id int64) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("image quality repository not configured")
	}
	_, err := r.db.ExecContext(ctx, `UPDATE image_quality_assessments SET last_checked_at = ? WHERE id = ?`, time.Now(), id)
	if err != nil {
		return fmt.Errorf("touch last_checked_at: %w", err)
	}
	return nil
}

// SampleForRevalidation returns up to limit rows whose last_checked_at is
// older than olderThan. Results are ordered oldest first.
func (r *ImageQualityRepository) SampleForRevalidation(ctx context.Context, olderThan time.Time, limit int) ([]ImageQualityAssessment, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("image quality repository not configured")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, entity_type, entity_id, variant, source, COALESCE(provider_ref,''),
		       width, height, blur_var, bpp, COALESCE(aspect_ratio, 0),
		       verdict, COALESCE(fail_reason,''), COALESCE(format,''), COALESCE(cache_path,''),
		       attempt_count, assessed_at, last_checked_at
		FROM image_quality_assessments
		WHERE last_checked_at < ?
		ORDER BY last_checked_at ASC
		LIMIT ?
	`, olderThan, limit)
	if err != nil {
		return nil, fmt.Errorf("sample for revalidation: %w", err)
	}
	defer rows.Close()

	out := []ImageQualityAssessment{}
	for rows.Next() {
		var a ImageQualityAssessment
		if err := rows.Scan(
			&a.ID, &a.EntityType, &a.EntityID, &a.Variant, &a.Source, &a.ProviderRef,
			&a.Width, &a.Height, &a.BlurVar, &a.BPP, &a.AspectRatio,
			&a.Verdict, &a.FailReason, &a.Format, &a.CachePath,
			&a.AttemptCount, &a.AssessedAt, &a.LastCheckedAt,
		); err != nil {
			return nil, fmt.Errorf("scan revalidation row: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
