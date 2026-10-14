-- Rollback of 021 (SQLite): drop the column. ALTER TABLE ... DROP COLUMN needs SQLite >= 3.35 and has NO "IF EXISTS" form.
ALTER TABLE storage_roots DROP COLUMN allow_empty;
