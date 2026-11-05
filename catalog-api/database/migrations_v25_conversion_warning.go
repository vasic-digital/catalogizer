package database

import (
	"context"
	"fmt"
)

// addConversionJobWarning (migration v25) adds conversion_jobs.warning: a job can end `completed`
// and still carry a client-safe warning (for example "the output is much shorter than the source
// declares: the source may be truncated"). Existing rows keep NULL.
func (db *DB) addConversionJobWarning(ctx context.Context) error {
	if db.dialect.IsPostgres() {
		if _, err := db.ExecContext(ctx, `ALTER TABLE conversion_jobs ADD COLUMN IF NOT EXISTS warning TEXT`); err != nil {
			return fmt.Errorf("postgres: conversion_jobs.warning: %w", err)
		}
		return nil
	}
	has, err := db.columnExistsSQLite(ctx, "conversion_jobs", "warning")
	if err != nil {
		return fmt.Errorf("sqlite: probe conversion_jobs.warning: %w", err)
	}
	if has {
		return nil
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE conversion_jobs ADD COLUMN warning TEXT`); err != nil {
		return fmt.Errorf("sqlite: add conversion_jobs.warning: %w", err)
	}
	return nil
}
