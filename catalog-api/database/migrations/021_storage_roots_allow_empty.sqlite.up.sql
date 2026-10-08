-- SQLite: storage_roots.allow_empty (see 021_storage_roots_allow_empty.up.sql). SQLite has no ADD COLUMN IF NOT EXISTS; the Go migration v21
-- probes PRAGMA table_info first. Running this file twice fails with "duplicate column name", which is the expected CLI behaviour.

ALTER TABLE storage_roots ADD COLUMN allow_empty BOOLEAN NOT NULL DEFAULT 0;
