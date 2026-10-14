-- Rollback of 021 (PostgreSQL): drop the column. SQLite has no "DROP COLUMN IF EXISTS" (it is a syntax error there): the SQLite rollback is
-- 021_storage_roots_allow_empty.sqlite.down.sql.
ALTER TABLE storage_roots DROP COLUMN IF EXISTS allow_empty;
