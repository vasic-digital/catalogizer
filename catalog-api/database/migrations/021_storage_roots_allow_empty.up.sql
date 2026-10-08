-- PostgreSQL: storage_roots.allow_empty - a root that is EXPECTED to be empty may complete a scan with 0 files (default FALSE: an empty root fails).
--
-- NOTE: the authoritative runtime migration is the Go migration v21
-- (database/migrations_v21_storage_root_allow_empty.go, registered in
-- database/migrations.go and applied by DB.RunMigrations). This SQL file
-- mirrors it for the golang-migrate CLI path documented in this directory's
-- README.

ALTER TABLE storage_roots ADD COLUMN IF NOT EXISTS allow_empty BOOLEAN NOT NULL DEFAULT FALSE;
