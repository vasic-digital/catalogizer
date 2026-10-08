-- Rollback of 021: drop the column (PostgreSQL; SQLite >= 3.35 supports DROP COLUMN as well).
ALTER TABLE storage_roots DROP COLUMN IF EXISTS allow_empty;
