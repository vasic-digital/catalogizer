# Backup and Disaster Recovery Guide

This guide covers backup procedures, disaster recovery planning, and data retention policies for Catalogizer production deployments.

## Table of Contents

1. [Backup Strategy Overview](#backup-strategy-overview)
2. [Database Backup (PostgreSQL)](#database-backup-postgresql)
3. [Database Backup (SQLite)](#database-backup-sqlite)
4. [Redis Data Persistence](#redis-data-persistence)
5. [Configuration Backup](#configuration-backup)
6. [Full System Backup](#full-system-backup)
7. [Disaster Recovery Procedures](#disaster-recovery-procedures)
8. [Data Retention Policies](#data-retention-policies)
9. [Backup Verification](#backup-verification)
10. [Automated Backup Setup](#automated-backup-setup)

---

## Backup Strategy Overview

Catalogizer stores data across several components:

| Component | Storage Type | Backup Priority | Method |
|-----------|-------------|-----------------|--------|
| PostgreSQL | Relational database | Critical | pg_dump |
| SQLite | File-based database | Critical | File copy / `.backup` command |
| Redis | In-memory cache + AOF/RDB | Medium | RDB snapshot + AOF file copy |
| Configuration | Files (.env, config.json, nginx.conf) | High | File copy |
| Media files | Filesystem / SMB mounts | Low (read-only source) | Not typically backed up (source of truth is SMB) |
| Docker volumes | Container state | Medium | Volume export |

### Recommended Backup Schedule

| Backup Type | Frequency | Retention |
|-------------|-----------|-----------|
| Database full backup | Daily at 2:00 AM | 30 days |
| Configuration backup | After every change | 90 days |
| Redis RDB snapshot | Every 6 hours | 7 days |
| Full system backup | Weekly | 12 weeks |

---

## Database Backup (PostgreSQL)

PostgreSQL is the primary production database. Use `pg_dump` for logical backups.

### Manual Backup

> Uses rootless Podman Compose; never use sudo. Release artifacts are built only inside rootless build containers (constitution §11.4.173).

```bash
# Full database dump (plain SQL format)
podman compose exec postgres pg_dump \
  -U ${POSTGRES_USER:-catalogizer} \
  ${POSTGRES_DB:-catalogizer} \
  > backup_$(date +%Y%m%d_%H%M%S).sql

# Compressed backup (recommended for large databases)
podman compose exec postgres pg_dump \
  -U ${POSTGRES_USER:-catalogizer} \
  --format=custom \
  --compress=9 \
  ${POSTGRES_DB:-catalogizer} \
  > backup_$(date +%Y%m%d_%H%M%S).dump

# Schema-only backup (useful for migration reference)
podman compose exec postgres pg_dump \
  -U ${POSTGRES_USER:-catalogizer} \
  --schema-only \
  ${POSTGRES_DB:-catalogizer} \
  > schema_$(date +%Y%m%d_%H%M%S).sql
```

### Restore from PostgreSQL Backup

```bash
# Step 1: Stop the API to prevent writes
podman compose stop api

# Step 2: Restore from plain SQL backup
podman compose exec -T postgres psql \
  -U ${POSTGRES_USER:-catalogizer} \
  ${POSTGRES_DB:-catalogizer} \
  < backup_20260201_020000.sql

# OR restore from custom-format backup
podman compose exec -T postgres pg_restore \
  -U ${POSTGRES_USER:-catalogizer} \
  -d ${POSTGRES_DB:-catalogizer} \
  --clean \
  --if-exists \
  < backup_20260201_020000.dump

# Step 3: Restart the API
podman compose start api

# Step 4: Verify
curl -sf http://localhost:8080/health
```

### Database Tables Overview

The database contains the following key tables (from migration `000001_initial_schema.up.sql`):

- `storage_roots` -- Configured storage sources (SMB, FTP, NFS, WebDAV, local)
- `files` -- Cataloged file entries with metadata
- `users` -- User accounts (from migration 000003)
- `conversion_jobs` -- Media conversion job queue (from migration 000002)
- `subtitles` -- Subtitle data (from migrations 014-015)

### Point-in-Time Recovery

For continuous backup with point-in-time recovery, configure PostgreSQL WAL archiving:

```bash
# Enable WAL archiving in PostgreSQL configuration
podman compose exec postgres psql -U catalogizer -c "
ALTER SYSTEM SET wal_level = 'replica';
ALTER SYSTEM SET archive_mode = 'on';
ALTER SYSTEM SET archive_command = 'cp %p /backups/wal/%f';
"

# Create WAL archive directory
podman compose exec postgres mkdir -p /backups/wal

# Restart PostgreSQL to apply changes
podman compose restart postgres
```

---

## Database Backup (SQLite)

SQLite is used in development and single-server deployments. The database file is configured via the `DB_PATH` config setting (default: `./data/catalogizer.db`).

### Manual Backup

```bash
# Option 1: Using SQLite backup command (safe, handles WAL)
sqlite3 /path/to/catalogizer.db ".backup /path/to/backup/catalogizer_$(date +%Y%m%d_%H%M%S).db"

# Option 2: Copy with WAL checkpoint (ensures all data is flushed)
sqlite3 /path/to/catalogizer.db "PRAGMA wal_checkpoint(TRUNCATE);"
cp /path/to/catalogizer.db /path/to/backup/catalogizer_$(date +%Y%m%d_%H%M%S).db

# Option 3: Compressed backup
sqlite3 /path/to/catalogizer.db ".backup /tmp/catalogizer_backup.db"
gzip -c /tmp/catalogizer_backup.db > /path/to/backup/catalogizer_$(date +%Y%m%d_%H%M%S).db.gz
rm /tmp/catalogizer_backup.db
```

### Restore from SQLite Backup

```bash
# Step 1: Stop the API
sudo systemctl stop catalogizer-api
# or: podman compose stop api

# Step 2: Backup the current database (just in case)
cp /path/to/catalogizer.db /path/to/catalogizer.db.pre-restore

# Step 3: Restore
# From uncompressed backup:
cp /path/to/backup/catalogizer_20260201_020000.db /path/to/catalogizer.db

# From compressed backup:
gunzip -c /path/to/backup/catalogizer_20260201_020000.db.gz > /path/to/catalogizer.db

# Step 4: Fix permissions
chown catalogizer:catalogizer /path/to/catalogizer.db

# Step 5: Restart the API
sudo systemctl start catalogizer-api
# or: podman compose start api
```

### SQLite WAL Mode Considerations

The Catalogizer database is configured with WAL (Write-Ahead Logging) mode via the connection string: `_journal_mode=WAL`. When backing up, ensure you also back up the associated WAL and SHM files if they exist:

```bash
# Checkpoint WAL before backup to ensure consistency
sqlite3 /path/to/catalogizer.db "PRAGMA wal_checkpoint(TRUNCATE);"

# Now these files can be safely copied
ls -la /path/to/catalogizer.db*
# catalogizer.db       -- main database
# catalogizer.db-wal   -- WAL file (may exist)
# catalogizer.db-shm   -- shared memory file (may exist)
```

---

## Redis Data Persistence

Redis is used for distributed rate limiting and caching. It is configured with both RDB snapshots and AOF persistence via `config/redis.conf`.

### Current Redis Persistence Settings

From `config/redis.conf`:

```
# RDB snapshots
save 900 1      # Save after 900s if at least 1 key changed
save 300 10     # Save after 300s if at least 10 keys changed
save 60 10000   # Save after 60s if at least 10000 keys changed

# AOF persistence
appendonly yes
appendfilename "appendonly.aof"
appendfsync everysec
```

### Manual Redis Backup

```bash
# Trigger an RDB snapshot
podman compose exec redis redis-cli BGSAVE
# Wait for completion
podman compose exec redis redis-cli LASTSAVE

# Copy the RDB file from the Docker volume
podman compose exec redis cat /data/dump.rdb > redis_backup_$(date +%Y%m%d_%H%M%S).rdb

# Copy the AOF file
podman compose exec redis cat /data/appendonly.aof > redis_aof_backup_$(date +%Y%m%d_%H%M%S).aof
```

### Restore Redis Data

```bash
# Step 1: Stop Redis
podman compose stop redis

# Step 2: Copy backup files into the Redis data volume
podman compose run --rm -v $(pwd):/backup redis sh -c "cp /backup/redis_backup.rdb /data/dump.rdb"

# Step 3: Start Redis
podman compose start redis

# Step 4: Verify
podman compose exec redis redis-cli DBSIZE
```

### Note on Redis Data Loss

Redis data loss is non-critical for Catalogizer. The API gracefully falls back to in-memory rate limiting if Redis is unavailable. Cache data will be rebuilt automatically. If Redis data is lost, the only impact is:
- Rate limiting counters reset (temporary)
- Cached API responses are invalidated (temporary performance impact)

---

## Configuration Backup

### Files to Back Up

```bash
# Critical configuration files
/opt/catalogizer/.env                              # Environment variables
/opt/catalogizer/config/nginx.conf                 # Nginx configuration
/opt/catalogizer/config/redis.conf                 # Redis configuration
/opt/catalogizer/monitoring/prometheus.yml          # Prometheus configuration
/opt/catalogizer/monitoring/grafana/                # Grafana provisioning
/opt/catalogizer/ssl/                              # SSL certificates
/opt/catalogizer/catalog-api/config.json           # API configuration (if used)
```

### Configuration Backup Script

```bash
#!/bin/bash
# Save as: /opt/catalogizer/scripts/backup_config.sh

set -euo pipefail

BACKUP_DIR="/opt/catalogizer/backups/config"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
ARCHIVE="$BACKUP_DIR/config_backup_$TIMESTAMP.tar.gz"

mkdir -p "$BACKUP_DIR"

cd /opt/catalogizer

tar -czf "$ARCHIVE" \
  --exclude='*.log' \
  --exclude='node_modules' \
  .env \
  config/ \
  monitoring/ \
  ssl/ \
  docker-compose.yml \
  docker-compose.dev.yml \
  2>/dev/null || true

echo "Configuration backup created: $ARCHIVE"
echo "Size: $(du -h "$ARCHIVE" | cut -f1)"

# Retain last 90 days
find "$BACKUP_DIR" -name "config_backup_*.tar.gz" -mtime +90 -delete
```

```bash
chmod +x /opt/catalogizer/scripts/backup_config.sh
```

---

## Full System Backup

### Comprehensive Backup Script

Save as `/opt/catalogizer/scripts/full_backup.sh`:

```bash
#!/bin/bash
set -euo pipefail

BACKUP_ROOT="/opt/catalogizer/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="$BACKUP_ROOT/$TIMESTAMP"

echo "=== Catalogizer Full Backup: $TIMESTAMP ==="

mkdir -p "$BACKUP_DIR"

# 1. Database backup
echo "Backing up PostgreSQL database..."
podman compose -f /opt/catalogizer/docker-compose.yml exec -T postgres pg_dump \
  -U ${POSTGRES_USER:-catalogizer} \
  --format=custom \
  --compress=9 \
  ${POSTGRES_DB:-catalogizer} \
  > "$BACKUP_DIR/database.dump" 2>/dev/null
echo "  Database: $(du -h "$BACKUP_DIR/database.dump" | cut -f1)"

# 2. Redis backup
echo "Backing up Redis data..."
podman compose -f /opt/catalogizer/docker-compose.yml exec -T redis redis-cli BGSAVE > /dev/null 2>&1
sleep 2
podman compose -f /opt/catalogizer/docker-compose.yml exec -T redis cat /data/dump.rdb \
  > "$BACKUP_DIR/redis.rdb" 2>/dev/null || echo "  Redis backup skipped (no data)"

# 3. Configuration backup
echo "Backing up configuration..."
cd /opt/catalogizer
tar -czf "$BACKUP_DIR/config.tar.gz" \
  --exclude='*.log' \
  --exclude='node_modules' \
  --exclude='backups' \
  .env config/ monitoring/ ssl/ docker-compose*.yml \
  2>/dev/null || true

# 4. Docker volume list (for reference)
echo "Recording Docker volume state..."
docker volume ls --filter name=catalogizer > "$BACKUP_DIR/volumes.txt"

# 5. Create manifest
cat > "$BACKUP_DIR/manifest.txt" <<EOF
Catalogizer Full Backup
========================
Created: $(date)
Host: $(hostname)
Docker version: $(docker --version)

Components backed up:
- PostgreSQL database (custom format, compressed)
- Redis RDB snapshot
- Configuration files (env, nginx, redis, prometheus, grafana, ssl)
- Docker volume listing

Restore instructions: See docs/deployment/BACKUP_AND_RECOVERY.md
EOF

# 6. Create single archive
echo "Creating archive..."
tar -czf "$BACKUP_ROOT/full_backup_$TIMESTAMP.tar.gz" -C "$BACKUP_ROOT" "$TIMESTAMP"
rm -rf "$BACKUP_DIR"

FINAL_SIZE=$(du -h "$BACKUP_ROOT/full_backup_$TIMESTAMP.tar.gz" | cut -f1)
echo "Backup completed: full_backup_$TIMESTAMP.tar.gz ($FINAL_SIZE)"

# 7. Upload to remote storage (optional)
if [ -n "${S3_BACKUP_BUCKET:-}" ]; then
  echo "Uploading to S3..."
  aws s3 cp "$BACKUP_ROOT/full_backup_$TIMESTAMP.tar.gz" \
    "s3://$S3_BACKUP_BUCKET/catalogizer/full_backup_$TIMESTAMP.tar.gz"
  echo "Upload completed."
fi

# 8. Cleanup old backups
echo "Cleaning up backups older than ${BACKUP_RETENTION_DAYS:-30} days..."
find "$BACKUP_ROOT" -name "full_backup_*.tar.gz" -mtime +${BACKUP_RETENTION_DAYS:-30} -delete

echo "=== Backup complete ==="
```

```bash
chmod +x /opt/catalogizer/scripts/full_backup.sh
```

---

## Disaster Recovery Procedures

### Scenario 1: API Container Failure

**Impact**: API is down, database and cache are intact.

```bash
# Step 1: Restart the API container
podman compose restart api

# Step 2: If restart fails, rebuild
podman compose up -d --build --no-deps api

# Step 3: Verify
sleep 10
curl -sf http://localhost:8080/health
```

### Scenario 2: Database Corruption

**Impact**: API cannot serve requests, data may be lost.

```bash
# Step 1: Stop the API
podman compose stop api

# Step 2: Check database health
podman compose exec postgres pg_isready -U catalogizer

# Step 3: If database is running but corrupted, restore from backup
# Find the latest backup
ls -la /opt/catalogizer/backups/full_backup_*.tar.gz | tail -1

# Step 4: Extract the backup
LATEST_BACKUP=$(ls -t /opt/catalogizer/backups/full_backup_*.tar.gz | head -1)
RESTORE_DIR=/tmp/catalogizer_restore
mkdir -p $RESTORE_DIR
tar -xzf $LATEST_BACKUP -C $RESTORE_DIR

# Step 5: Restore the database
BACKUP_TIMESTAMP=$(ls $RESTORE_DIR | head -1)
podman compose exec -T postgres psql -U catalogizer -c "DROP DATABASE IF EXISTS catalogizer;"
podman compose exec -T postgres psql -U catalogizer -c "CREATE DATABASE catalogizer;"
podman compose exec -T postgres pg_restore \
  -U catalogizer \
  -d catalogizer \
  "$RESTORE_DIR/$BACKUP_TIMESTAMP/database.dump"

# Step 6: Restart the API
podman compose start api

# Step 7: Verify
sleep 10
curl -sf http://localhost:8080/health

# Step 8: Clean up
rm -rf $RESTORE_DIR
```

### Scenario 3: Complete Server Loss

**Impact**: Everything is gone. Full recovery needed on a new server.

```bash
# Step 1: Provision a new server (see PRODUCTION_RUNBOOK.md Prerequisites)

# Step 2: Install Docker
curl -fsSL https://get.docker.com | sudo sh

# Step 3: Clone the repository
git clone <repository-url> /opt/catalogizer
cd /opt/catalogizer

# Step 4: Retrieve the latest backup from remote storage
aws s3 cp s3://$S3_BACKUP_BUCKET/catalogizer/ . --recursive --exclude "*" \
  --include "full_backup_*.tar.gz" | tail -1
# Or copy from your backup location

# Step 5: Extract backup
LATEST_BACKUP=$(ls -t full_backup_*.tar.gz | head -1)
mkdir -p /tmp/restore
tar -xzf $LATEST_BACKUP -C /tmp/restore

# Step 6: Restore configuration
BACKUP_DIR=$(ls /tmp/restore | head -1)
tar -xzf /tmp/restore/$BACKUP_DIR/config.tar.gz -C /opt/catalogizer/

# Step 7: Start infrastructure services
podman compose up -d postgres redis
echo "Waiting for databases..."
sleep 20

# Step 8: Restore database
podman compose exec -T postgres pg_restore \
  -U catalogizer \
  -d catalogizer \
  --clean --if-exists \
  /tmp/restore/$BACKUP_DIR/database.dump

# Step 9: Restore Redis (optional)
podman compose exec -T redis sh -c "cat > /data/dump.rdb" < /tmp/restore/$BACKUP_DIR/redis.rdb
podman compose restart redis

# Step 10: Start the API
podman compose up -d api

# Step 11: Start nginx (if using production profile)
podman compose --profile production up -d nginx

# Step 12: Verify
sleep 10
curl -sf http://localhost:8080/health && echo "Recovery successful"

# Step 13: Clean up
rm -rf /tmp/restore
```

### Scenario 4: Redis Data Loss

**Impact**: Rate limiting counters reset. Minimal operational impact.

```bash
# Redis data loss is non-critical. Simply restart Redis.
podman compose restart redis

# The API will automatically reconnect and rebuild cache.
# If Redis was unavailable, restart the API to re-establish connection.
podman compose restart api
```

---

## Data Retention Policies

### Recommended Retention

| Data Type | Retention Period | Storage Location |
|-----------|-----------------|-----------------|
| Daily database backups | 30 days | Local + remote (S3) |
| Weekly full system backups | 12 weeks | Local + remote (S3) |
| Monthly archival backups | 12 months | Remote (S3/Glacier) |
| Configuration backups | 90 days | Local + remote |
| Redis snapshots | 7 days | Local only |
| Application logs | 30 days | Local |
| Prometheus metrics | 200 hours (~8 days) | Docker volume |
| Grafana data | Indefinite | Docker volume |

### Cleanup Commands

```bash
# Remove database backups older than 30 days
find /opt/catalogizer/backups -name "*.sql" -mtime +30 -delete
find /opt/catalogizer/backups -name "*.dump" -mtime +30 -delete

# Remove full system backups older than 84 days (12 weeks)
find /opt/catalogizer/backups -name "full_backup_*.tar.gz" -mtime +84 -delete

# Remove config backups older than 90 days
find /opt/catalogizer/backups/config -name "config_backup_*.tar.gz" -mtime +90 -delete

# Remove Redis snapshots older than 7 days
find /opt/catalogizer/backups -name "redis_*.rdb" -mtime +7 -delete

# Remove old application logs
find /opt/catalogizer/logs -name "*.log" -mtime +30 -delete

# Clean Docker resources
docker system prune -f --filter "until=720h"
```

---

## Backup Verification

### Automated Verification Script

Save as `/opt/catalogizer/scripts/verify_backup.sh`:

```bash
#!/bin/bash
set -euo pipefail

BACKUP_FILE=$1

if [ -z "${BACKUP_FILE:-}" ]; then
    echo "Usage: $0 <backup_file.tar.gz>"
    exit 1
fi

echo "=== Verifying backup: $BACKUP_FILE ==="

# Check archive integrity
echo "Checking archive integrity..."
tar -tzf "$BACKUP_FILE" > /dev/null && echo "  Archive: OK" || { echo "  Archive: CORRUPTED"; exit 1; }

# Extract to temp directory
VERIFY_DIR=$(mktemp -d)
tar -xzf "$BACKUP_FILE" -C "$VERIFY_DIR"
BACKUP_TIMESTAMP=$(ls "$VERIFY_DIR" | head -1)

# Check database dump
if [ -f "$VERIFY_DIR/$BACKUP_TIMESTAMP/database.dump" ]; then
    SIZE=$(du -h "$VERIFY_DIR/$BACKUP_TIMESTAMP/database.dump" | cut -f1)
    echo "  Database dump: present ($SIZE)"

    # Test restore to a temporary database
    echo "  Testing database restore..."
    podman compose exec -T postgres psql -U catalogizer -c "CREATE DATABASE verify_test;" 2>/dev/null || true
    if podman compose exec -T postgres pg_restore \
        -U catalogizer -d verify_test --clean --if-exists \
        "$VERIFY_DIR/$BACKUP_TIMESTAMP/database.dump" 2>/dev/null; then
        echo "  Database restore test: PASSED"
    else
        echo "  Database restore test: FAILED (may be OK if tables don't exist yet)"
    fi
    podman compose exec -T postgres psql -U catalogizer -c "DROP DATABASE IF EXISTS verify_test;" 2>/dev/null
else
    echo "  Database dump: MISSING"
fi

# Check config backup
if [ -f "$VERIFY_DIR/$BACKUP_TIMESTAMP/config.tar.gz" ]; then
    echo "  Configuration: present"
    tar -tzf "$VERIFY_DIR/$BACKUP_TIMESTAMP/config.tar.gz" | head -5
else
    echo "  Configuration: MISSING"
fi

# Check Redis backup
if [ -f "$VERIFY_DIR/$BACKUP_TIMESTAMP/redis.rdb" ]; then
    SIZE=$(du -h "$VERIFY_DIR/$BACKUP_TIMESTAMP/redis.rdb" | cut -f1)
    echo "  Redis snapshot: present ($SIZE)"
else
    echo "  Redis snapshot: not present (non-critical)"
fi

# Check manifest
if [ -f "$VERIFY_DIR/$BACKUP_TIMESTAMP/manifest.txt" ]; then
    echo "  Manifest:"
    cat "$VERIFY_DIR/$BACKUP_TIMESTAMP/manifest.txt" | sed 's/^/    /'
fi

# Cleanup
rm -rf "$VERIFY_DIR"

echo "=== Verification complete ==="
```

```bash
chmod +x /opt/catalogizer/scripts/verify_backup.sh
```

---

## Automated Backup Setup

### Cron Configuration

```bash
# Edit crontab
crontab -e

# Add the following lines:

# Daily full backup at 2:00 AM
0 2 * * * /opt/catalogizer/scripts/full_backup.sh >> /opt/catalogizer/logs/backup.log 2>&1

# Configuration backup after business hours (6:00 PM)
0 18 * * 1-5 /opt/catalogizer/scripts/backup_config.sh >> /opt/catalogizer/logs/backup.log 2>&1

# Weekly backup verification (Sunday at 4:00 AM)
0 4 * * 0 LATEST=$(ls -t /opt/catalogizer/backups/full_backup_*.tar.gz | head -1) && /opt/catalogizer/scripts/verify_backup.sh "$LATEST" >> /opt/catalogizer/logs/backup.log 2>&1

# Monthly cleanup of old backups
0 3 1 * * find /opt/catalogizer/backups -name "full_backup_*.tar.gz" -mtime +84 -delete >> /opt/catalogizer/logs/backup.log 2>&1
```

### Docker Compose Backup Service (Alternative)

The `deployment/docker-compose.yml` includes an optional backup service profile:

```bash
# Start the backup service
podman compose -f deployment/docker-compose.yml --profile backup up -d backup
```

This service supports the following environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `BACKUP_SCHEDULE` | `0 2 * * *` | Cron schedule for automated backups |
| `BACKUP_RETENTION_DAYS` | `30` | Days to retain local backups |
| `S3_BACKUP_ENABLED` | `false` | Enable S3 remote backup |
| `S3_BACKUP_BUCKET` | (empty) | S3 bucket name |
| `S3_ACCESS_KEY` | (empty) | AWS access key |
| `S3_SECRET_KEY` | (empty) | AWS secret key |
