# Catalogizer Migration Guide

This guide covers database migration procedures, environment variable changes, and version upgrade paths for Catalogizer deployments.

---

## Table of Contents

1. [SQLite to PostgreSQL Migration](#sqlite-to-postgresql-migration)
2. [Database Schema Upgrades](#database-schema-upgrades)
3. [Environment Variable Changes](#environment-variable-changes)
4. [Version Upgrade Procedures](#version-upgrade-procedures)

---

## SQLite to PostgreSQL Migration

Catalogizer supports both SQLite (development/small deployments) and PostgreSQL (production). The dual-dialect abstraction in `catalog-api/database/dialect.go` rewrites SQL transparently, but the data must be migrated manually.

### Prerequisites

- PostgreSQL 15+ installed and running
- A dedicated database and user created for Catalogizer
- Podman or direct access to the PostgreSQL server
- A backup of the current SQLite database

### Step 1: Create the PostgreSQL Database

```bash
# Connect to PostgreSQL
sudo -u postgres psql

# Create database and user
CREATE USER catalogizer_user WITH PASSWORD 'your_secure_password';
CREATE DATABASE catalogizer OWNER catalogizer_user;
GRANT ALL PRIVILEGES ON DATABASE catalogizer TO catalogizer_user;

\q
```

### Step 2: Back Up the SQLite Database

```bash
# Stop the Catalogizer service
sudo systemctl stop catalogizer

# Back up the SQLite database
cp catalog-api/data/catalogizer.db catalog-api/data/catalogizer.db.backup
```

### Step 3: Update Environment Variables

Create or update the `.env` file in `catalog-api/`:

```env
DATABASE_TYPE=postgres
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_NAME=catalogizer
DATABASE_USER=catalogizer_user
DATABASE_PASSWORD=your_secure_password
DATABASE_SSL_MODE=disable
```

For containerized PostgreSQL with port mapping 5432->5433:

```env
DATABASE_HOST=localhost
DATABASE_PORT=5433
```

### Step 4: Start Catalogizer to Create Schema

Start the Catalogizer backend. It will detect PostgreSQL and run all migrations (v1 through v10) to create the schema:

```bash
cd catalog-api
go run main.go
```

Verify migrations applied:

```bash
PGPASSWORD=$DATABASE_PASSWORD psql -h $DATABASE_HOST -p $DATABASE_PORT -U $DATABASE_USER -d $DATABASE_NAME -c \
  "SELECT version, name, applied_at FROM migrations ORDER BY version;"
```

Stop the service after migrations complete.

### Step 5: Export Data from SQLite

Use the `sqlite3` CLI to export each table as CSV:

```bash
DB=catalog-api/data/catalogizer.db

# Export storage_roots
sqlite3 -header -csv "$DB" "SELECT * FROM storage_roots;" > /tmp/storage_roots.csv

# Export files
sqlite3 -header -csv "$DB" "SELECT * FROM files;" > /tmp/files.csv

# Export file_metadata
sqlite3 -header -csv "$DB" "SELECT * FROM file_metadata;" > /tmp/file_metadata.csv

# Export duplicate_groups
sqlite3 -header -csv "$DB" "SELECT * FROM duplicate_groups;" > /tmp/duplicate_groups.csv

# Export scan_history
sqlite3 -header -csv "$DB" "SELECT * FROM scan_history;" > /tmp/scan_history.csv

# Export users
sqlite3 -header -csv "$DB" "SELECT * FROM users;" > /tmp/users.csv

# Export roles
sqlite3 -header -csv "$DB" "SELECT * FROM roles;" > /tmp/roles.csv

# Export media_types
sqlite3 -header -csv "$DB" "SELECT * FROM media_types;" > /tmp/media_types.csv

# Export media_items
sqlite3 -header -csv "$DB" "SELECT * FROM media_items;" > /tmp/media_items.csv

# Export media_files
sqlite3 -header -csv "$DB" "SELECT * FROM media_files;" > /tmp/media_files.csv

# Continue for all remaining tables...
```

### Step 6: Import Data into PostgreSQL

```bash
# Disable foreign key checks during import
PGPASSWORD=$DATABASE_PASSWORD psql -h $DATABASE_HOST -p $DATABASE_PORT -U $DATABASE_USER -d $DATABASE_NAME -c \
  "SET session_replication_role = 'replica';"

# Import tables in dependency order
for table in roles storage_roots duplicate_groups users permissions files file_metadata \
             virtual_paths scan_history user_sessions user_permissions auth_audit_log \
             conversion_jobs media_types media_items media_files media_collections \
             media_collection_items external_metadata user_metadata directory_analyses \
             detection_rules assets subtitle_tracks subtitle_sync_status subtitle_cache \
             subtitle_downloads media_subtitles sync_endpoints sync_sessions sync_schedules; do

  PGPASSWORD=$DATABASE_PASSWORD psql -h $DATABASE_HOST -p $DATABASE_PORT -U $DATABASE_USER -d $DATABASE_NAME -c \
    "\COPY $table FROM '/tmp/${table}.csv' WITH CSV HEADER;"
done

# Reset sequences to match imported data
PGPASSWORD=$DATABASE_PASSWORD psql -h $DATABASE_HOST -p $DATABASE_PORT -U $DATABASE_USER -d $DATABASE_NAME -c "
SELECT setval(pg_get_serial_sequence(t.table_name, 'id'),
              COALESCE((SELECT MAX(id) FROM information_schema.tables st
                        WHERE st.table_name = t.table_name), 1))
FROM information_schema.tables t
WHERE t.table_schema = 'public' AND t.table_type = 'BASE TABLE';
"

# Re-enable foreign key checks
PGPASSWORD=$DATABASE_PASSWORD psql -h $DATABASE_HOST -p $DATABASE_PORT -U $DATABASE_USER -d $DATABASE_NAME -c \
  "SET session_replication_role = 'origin';"
```

### Step 7: Reset PostgreSQL Sequences

After importing data with explicit IDs, sequences must be updated to avoid conflicts:

```sql
-- Run in psql
SELECT setval('storage_roots_id_seq', COALESCE((SELECT MAX(id) FROM storage_roots), 1));
SELECT setval('files_id_seq', COALESCE((SELECT MAX(id) FROM files), 1));
SELECT setval('users_id_seq', COALESCE((SELECT MAX(id) FROM users), 1));
SELECT setval('roles_id_seq', COALESCE((SELECT MAX(id) FROM roles), 1));
SELECT setval('media_items_id_seq', COALESCE((SELECT MAX(id) FROM media_items), 1));
SELECT setval('media_files_id_seq', COALESCE((SELECT MAX(id) FROM media_files), 1));
-- Repeat for all SERIAL columns...
```

### Step 8: Verify the Migration

```bash
# Start the service
cd catalog-api && go run main.go

# Test the health endpoint
curl -s http://localhost:8080/health | jq

# Compare row counts
echo "SQLite counts:"
sqlite3 "$DB" "SELECT 'storage_roots', COUNT(*) FROM storage_roots UNION ALL SELECT 'files', COUNT(*) FROM files UNION ALL SELECT 'users', COUNT(*) FROM users UNION ALL SELECT 'media_items', COUNT(*) FROM media_items;"

echo "PostgreSQL counts:"
PGPASSWORD=$DATABASE_PASSWORD psql -h $DATABASE_HOST -p $DATABASE_PORT -U $DATABASE_USER -d $DATABASE_NAME -c "
SELECT 'storage_roots', COUNT(*) FROM storage_roots UNION ALL
SELECT 'files', COUNT(*) FROM files UNION ALL
SELECT 'users', COUNT(*) FROM users UNION ALL
SELECT 'media_items', COUNT(*) FROM media_items;"
```

### Boolean Value Differences

SQLite uses `0`/`1` for booleans; PostgreSQL uses `TRUE`/`FALSE`. The Catalogizer dialect layer (`database.DB`) automatically rewrites boolean literals in queries at runtime. No manual conversion is needed in application code, but if you are importing data via CSV, ensure that boolean columns contain `t`/`f` or `true`/`false` values for PostgreSQL.

### Known Considerations

- **SQLCipher encryption**: SQLite databases encrypted with SQLCipher must be decrypted before export. Export from the running application or use the SQLCipher CLI with the encryption key.
- **BIGINT columns**: PostgreSQL uses `BIGINT` for `files.size`, `duplicate_groups.total_size`, `assets.size`, and `directory_analyses.total_size`. SQLite `INTEGER` values import directly without issues.
- **Triggers**: SQLite triggers are not exported. PostgreSQL trigger functions are created automatically during migration.

---

## Database Schema Upgrades

Catalogizer runs migrations automatically on startup. When you update the application binary, new migrations are applied the next time the service starts.

### How Migrations Work

1. A `migrations` table tracks which versions have been applied.
2. On startup, `database.RunMigrations()` iterates versions 1 through 10.
3. Each migration checks if its version exists in `migrations`; if not, it runs.
4. Migrations are idempotent -- they use `CREATE TABLE IF NOT EXISTS` and `CREATE INDEX IF NOT EXISTS`.

### Migration Versions

| Version | Tables/Changes |
|---------|----------------|
| 1 | storage_roots, files, file_metadata, duplicate_groups, virtual_paths, scan_history |
| 2 | Migrates legacy smb_roots data to storage_roots |
| 3 | users, roles, user_sessions, permissions, user_permissions, auth_audit_log |
| 4 | conversion_jobs |
| 5 | subtitle_tracks, subtitle_sync_status, subtitle_cache, subtitle_downloads, media_subtitles |
| 6 | Fixes subtitle table foreign keys (SQLite backup/recreate; PostgreSQL no-op) |
| 7 | assets |
| 8 | media_types, media_items, media_files, media_collections, media_collection_items, external_metadata, user_metadata, directory_analyses, detection_rules |
| 9 | Performance indexes on files, media_items, user_metadata; unique index on media_files with deduplication |
| 10 | sync_endpoints, sync_sessions, sync_schedules |

### Manual Schema Verification

```sql
-- List all tables
-- SQLite:
SELECT name FROM sqlite_master WHERE type='table' ORDER BY name;

-- PostgreSQL:
SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename;

-- Check migration status
SELECT version, name, applied_at FROM migrations ORDER BY version;
```

### Rolling Back a Migration

There is no built-in rollback mechanism. If a migration fails:

1. Investigate the error in the application logs.
2. Fix the underlying issue (e.g., data conflict, missing prerequisite table).
3. Restart the application; the migration will re-attempt.
4. If necessary, manually delete the migration record from the `migrations` table and let it re-run:

```sql
DELETE FROM migrations WHERE version = 9;
```

---

## Environment Variable Changes

### Database Configuration

| Variable | v1.0 Name | v2.0 Name | Notes |
|----------|-----------|-----------|-------|
| Database type | `DB_TYPE` | `DATABASE_TYPE` | Both accepted; `DATABASE_TYPE` takes precedence |
| Database host | `DB_HOST` | `DATABASE_HOST` | |
| Database port | `DB_PORT` | `DATABASE_PORT` | |
| Database name | `DB_NAME` | `DATABASE_NAME` | |
| Database user | `DB_USER` | `DATABASE_USER` | |
| Database password | `DB_PASSWORD` | `DATABASE_PASSWORD` | |
| SSL mode | -- | `DATABASE_SSL_MODE` | New in v2.0; default: `disable` |

### Server Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP server port | 8080 |
| `HOST` | Bind address | 0.0.0.0 |
| `GIN_MODE` | Gin framework mode | release |

### Authentication

| Variable | Description | Default |
|----------|-------------|---------|
| `JWT_SECRET` | JWT signing key | Auto-generated (ephemeral) |
| `ADMIN_USERNAME` | Default admin username | admin |
| `ADMIN_PASSWORD` | Admin password (no default; placeholders refused) | _required_ |

### Redis

| Variable | Description | Default |
|----------|-------------|---------|
| `REDIS_ADDR` | Redis address (host:port) | empty (disabled) |
| `REDIS_PASSWORD` | Redis password | empty |

### Precedence

Environment variables always override values from `config.json` and `.env` files. The precedence order is:

1. Environment variables (highest)
2. `.env` file
3. `config.json`
4. Application defaults (lowest)

---

## Version Upgrade Procedures

### Pre-Upgrade Checklist

1. Back up the database (see [Disaster Recovery](DISASTER_RECOVERY.md))
2. Note the current version: `curl -s http://localhost:8080/health | jq .version`
3. Review the [API Changelog](api/CHANGELOG.md) for breaking changes
4. Test the upgrade in a staging environment

### Upgrade Steps

```bash
# 1. Stop the service
sudo systemctl stop catalogizer

# 2. Back up the database
# SQLite:
cp catalog-api/data/catalogizer.db catalog-api/data/catalogizer.db.pre-upgrade

# PostgreSQL:
pg_dump -Fc -h $DATABASE_HOST -U $DATABASE_USER -d $DATABASE_NAME > catalogizer_pre_upgrade.dump

# 3. Update the binary
cd /opt/catalogizer
git pull origin main
cd catalog-api && go build -o /opt/catalogizer/bin/catalogizer

# 4. Start the service (migrations run automatically)
sudo systemctl start catalogizer

# 5. Verify
curl -s http://localhost:8080/health | jq
```

### Container Upgrade

```bash
# 1. Back up
podman exec catalogizer-db pg_dump -U catalogizer_user -d catalogizer > backup.sql

# 2. Pull and rebuild
podman-compose -f docker-compose.yml down
podman-compose -f docker-compose.yml build
podman-compose -f docker-compose.yml up -d

# 3. Verify
curl -s http://localhost:8080/health | jq
```

### Post-Upgrade Verification

```bash
# Check migration status
PGPASSWORD=$DATABASE_PASSWORD psql -h $DATABASE_HOST -U $DATABASE_USER -d $DATABASE_NAME -c \
  "SELECT version, name, applied_at FROM migrations ORDER BY version;"

# Run the challenge suite
curl -X POST http://localhost:8080/api/v1/challenges/run \
  -H "Authorization: Bearer $TOKEN"

# Check metrics
curl -s http://localhost:8080/metrics | grep catalogizer
```
