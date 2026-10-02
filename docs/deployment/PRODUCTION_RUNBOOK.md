# Production Deployment Runbook

This runbook provides step-by-step procedures for deploying, maintaining, and troubleshooting Catalogizer in production environments.

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Pre-Deployment Checklist](#pre-deployment-checklist)
3. [Step-by-Step Deployment](#step-by-step-deployment)
4. [Health Checks](#health-checks)
5. [Rolling Updates](#rolling-updates)
6. [Rollback Procedure](#rollback-procedure)
7. [Common Issues and Remediation](#common-issues-and-remediation)
8. [Maintenance Windows](#maintenance-windows)

---

## Prerequisites

### System Requirements

| Component | Minimum | Recommended |
|-----------|---------|-------------|
| CPU | 2 cores | 8+ cores |
| RAM | 4 GB | 16 GB |
| Storage | 20 GB SSD | 100 GB NVMe SSD |
| Network | 100 Mbps | 1 Gbps |
| OS | Ubuntu 20.04+ / RHEL 8+ | Ubuntu 22.04 LTS |

### Software Dependencies

| Software | Version | Purpose |
|----------|---------|---------|
| Docker | 20.10+ | Container runtime |
| Docker Compose | 2.0+ | Service orchestration |
| Go | 1.21+ | Backend build (if building from source) |
| Node.js | 18+ | Frontend build (if building from source) |
| PostgreSQL client | 15+ | Database management |
| curl | any | Health checks |
| nginx | alpine (Docker) | Reverse proxy / load balancer |

### Required Credentials and Secrets

Before deployment, ensure you have:

- A strong JWT secret (minimum 32 characters)
- PostgreSQL database password
- Admin username and password for the API
- SSL certificates (for HTTPS deployments)
- Grafana admin password (if using monitoring)
- Redis password (optional but recommended for production)

---

## Pre-Deployment Checklist

Run through this checklist before every production deployment:

> Uses rootless Podman Compose; never use sudo. Release artifacts are built only inside rootless build containers (constitution §11.4.173).

```bash
# 1. Verify Docker is running
docker info > /dev/null 2>&1 && echo "OK: Docker running" || echo "FAIL: Docker not running"

# 2. Verify Docker Compose is available
podman compose version && echo "OK" || echo "FAIL"

# 3. Check available disk space (need at least 10GB free)
df -h / | awk 'NR==2 {print "Available:", $4}'

# 4. Check that required ports are free
for port in 80 443 8080 5432 6379; do
  ss -tlnp | grep -q ":${port} " && echo "WARN: Port $port in use" || echo "OK: Port $port free"
done

# 5. Verify .env file exists and has required variables
cd /opt/catalogizer
test -f .env && echo "OK: .env exists" || echo "FAIL: .env missing"
grep -q "POSTGRES_PASSWORD" .env && echo "OK: DB password set" || echo "FAIL: DB password missing"
grep -q "JWT_SECRET" .env && echo "OK: JWT secret set" || echo "FAIL: JWT secret missing"

# 6. Verify SSL certificates (if using HTTPS)
test -f ssl/cert.pem && echo "OK: SSL cert exists" || echo "WARN: No SSL cert"
test -f ssl/key.pem && echo "OK: SSL key exists" || echo "WARN: No SSL key"
```

---

## Step-by-Step Deployment

### First-Time Deployment

#### Step 1: Prepare the Server

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install Docker (if not installed)
curl -fsSL https://get.docker.com | sudo sh
sudo usermod -aG docker $USER

# Create application directory
sudo mkdir -p /opt/catalogizer
sudo chown $USER:$USER /opt/catalogizer
cd /opt/catalogizer
```

#### Step 2: Clone and Configure

```bash
# Clone the repository
git clone <repository-url> /opt/catalogizer
cd /opt/catalogizer

# Create environment file from example
cp .env.example .env
```

#### Step 3: Edit Environment Configuration

Edit `/opt/catalogizer/.env` with production values:

```bash
# Application Environment
APP_ENV=production
LOG_LEVEL=info

# API Configuration
API_PORT=8080

# Database Configuration (PostgreSQL)
DATABASE_TYPE=postgres
POSTGRES_USER=catalogizer
POSTGRES_PASSWORD=<STRONG_PASSWORD_HERE>
POSTGRES_DB=catalogizer
POSTGRES_PORT=5432

# Redis Configuration
REDIS_PORT=6379
REDIS_PASSWORD=<REDIS_PASSWORD_HERE>

# Security
JWT_SECRET=<MINIMUM_32_CHAR_SECRET_HERE>

# CORS Configuration
CORS_ENABLED=false

# SMB/File System Configuration
SMB_ENABLED=true
MEDIA_ROOT_PATH=/media

# Docker Configuration
GO_VERSION=1.21
HTTP_PORT=80
HTTPS_PORT=443
```

#### Step 4: Create SSL Directory (if using HTTPS)

```bash
mkdir -p /opt/catalogizer/ssl
# Place your cert.pem and key.pem files in /opt/catalogizer/ssl/
```

#### Step 5: Build and Start Services

```bash
cd /opt/catalogizer

# Pull base images
podman compose pull

# Build the API image
podman compose build api

# Start core services (PostgreSQL, Redis, API)
podman compose up -d postgres redis
echo "Waiting for databases to initialize..."
sleep 15

# Verify databases are healthy
podman compose ps postgres redis

# Start the API
podman compose up -d api
echo "Waiting for API to start..."
sleep 10

# Start nginx reverse proxy (production profile)
podman compose --profile production up -d nginx
```

#### Step 6: Verify Deployment

```bash
# Check all container statuses
podman compose ps

# Test health endpoint
curl -f http://localhost:8080/health
# Expected: {"status":"healthy","time":"..."}

# Test through nginx (if enabled)
curl -f http://localhost/health

# Check logs for errors
podman compose logs --tail=50 api | grep -i "error"
```

### Subsequent Deployments

```bash
cd /opt/catalogizer

# Pull latest changes
git pull origin main

# Rebuild only the API image
podman compose build api

# Restart the API service with zero downtime
podman compose up -d --no-deps api

# Verify health
sleep 10
curl -f http://localhost:8080/health
```

---

## Health Checks

### Automated Health Check Script

Save as `/opt/catalogizer/scripts/health_check.sh`:

```bash
#!/bin/bash
set -euo pipefail

HEALTH_URL="http://localhost:8080/health"
TIMEOUT=10
MAX_RETRIES=3
ALERT_EMAIL="${ALERT_EMAIL:-ops@yourcompany.com}"

check_service() {
    local service=$1
    local status
    status=$(podman compose -f /opt/catalogizer/docker-compose.yml ps --format json "$service" 2>/dev/null | jq -r '.Health // .State' 2>/dev/null)
    echo "$service: $status"
}

check_api_health() {
    local response
    response=$(curl -s -o /dev/null -w "%{http_code}" --max-time "$TIMEOUT" "$HEALTH_URL" 2>/dev/null || echo "000")
    if [ "$response" = "200" ]; then
        echo "API: healthy (HTTP $response)"
        return 0
    else
        echo "API: unhealthy (HTTP $response)"
        return 1
    fi
}

echo "=== Catalogizer Health Check $(date) ==="

# Check individual services
check_service postgres
check_service redis
check_service api

# Check API health endpoint
for i in $(seq 1 "$MAX_RETRIES"); do
    if check_api_health; then
        exit 0
    fi
    echo "Retry $i/$MAX_RETRIES..."
    sleep 5
done

echo "CRITICAL: API health check failed after $MAX_RETRIES attempts"
exit 1
```

```bash
chmod +x /opt/catalogizer/scripts/health_check.sh
```

### Manual Health Checks

```bash
# API health
curl -s http://localhost:8080/health | jq .

# PostgreSQL health
podman compose exec postgres pg_isready -U catalogizer
# Expected: /var/run/postgresql:5432 - accepting connections

# Redis health
podman compose exec redis redis-cli ping
# Expected: PONG

# Container resource usage
docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}\t{{.NetIO}}"

# Check container logs for recent errors
podman compose logs --since 10m api 2>&1 | grep -ci "error"
```

### Cron-Based Health Monitoring

```bash
# Add to crontab: crontab -e
*/5 * * * * /opt/catalogizer/scripts/health_check.sh >> /opt/catalogizer/logs/health.log 2>&1
```

---

## Rolling Updates

### Docker Compose Rolling Update

```bash
cd /opt/catalogizer

# Step 1: Create a pre-deployment backup
podman compose exec postgres pg_dump -U catalogizer catalogizer > backup_$(date +%Y%m%d_%H%M%S).sql

# Step 2: Pull latest code
git pull origin main

# Step 3: Build new image
podman compose build api

# Step 4: Rolling restart of the API
# Docker Compose will stop the old container and start the new one
podman compose up -d --no-deps --build api

# Step 5: Wait for the new container to become healthy
echo "Waiting for health check..."
for i in $(seq 1 30); do
    if curl -sf http://localhost:8080/health > /dev/null 2>&1; then
        echo "API is healthy after ${i}s"
        break
    fi
    sleep 1
done

# Step 6: Verify
curl -s http://localhost:8080/health | jq .
podman compose logs --tail=20 api
```

### Multi-Instance Rolling Update (with nginx load balancer)

If running multiple API instances behind nginx:

```bash
# Step 1: Scale up with new version
podman compose up -d --scale api=2 --no-recreate

# Step 2: Wait for new instance to be healthy
sleep 30

# Step 3: Stop old instance
docker stop catalogizer-api

# Step 4: Rename new instance
# (Or configure nginx upstream dynamically)

# Step 5: Verify
curl -sf http://localhost/health
```

---

## Rollback Procedure

### Quick Rollback (Docker)

```bash
cd /opt/catalogizer

# Step 1: Stop the current API
podman compose stop api

# Step 2: Revert to previous Git commit
git log --oneline -5  # Identify the previous good commit
git checkout <PREVIOUS_COMMIT_HASH>

# Step 3: Rebuild and restart
podman compose build api
podman compose up -d api

# Step 4: Verify
sleep 10
curl -sf http://localhost:8080/health && echo "Rollback successful" || echo "Rollback FAILED"
```

### Rollback with Database Restore

If the deployment included database schema changes that need reverting:

```bash
cd /opt/catalogizer

# Step 1: Stop the API
podman compose stop api

# Step 2: Restore database from backup
podman compose exec -T postgres psql -U catalogizer catalogizer < backup_YYYYMMDD_HHMMSS.sql

# Step 3: Revert code
git checkout <PREVIOUS_COMMIT_HASH>

# Step 4: Rebuild and restart
podman compose build api
podman compose up -d api

# Step 5: Verify
sleep 10
curl -sf http://localhost:8080/health && echo "Rollback successful" || echo "Rollback FAILED"
podman compose logs --tail=20 api
```

### Emergency Rollback

If the system is completely unresponsive:

```bash
# Force stop all containers
podman compose down

# Revert to known-good commit
git checkout <KNOWN_GOOD_COMMIT>

# Clean rebuild everything
podman compose build --no-cache api

# Start fresh
podman compose up -d postgres redis
sleep 15
podman compose up -d api
sleep 10

# Restore database if needed
podman compose exec -T postgres psql -U catalogizer catalogizer < backup_YYYYMMDD_HHMMSS.sql

# Restart API to pick up restored data
podman compose restart api

# Verify
curl -sf http://localhost:8080/health
```

---

## Common Issues and Remediation

### Issue: API Container Fails to Start

**Symptoms**: Container exits immediately or restarts in a loop.

```bash
# Check logs
podman compose logs --tail=100 api

# Common causes:
# 1. Database not ready yet
podman compose ps postgres  # Should show "healthy"
podman compose restart api

# 2. Missing environment variables
podman compose exec api env | grep -E "JWT_SECRET|ADMIN"

# 3. Port conflict
ss -tlnp | grep 8080
```

### Issue: Database Connection Refused

**Symptoms**: API logs show "connection refused" to PostgreSQL.

```bash
# Check if PostgreSQL is running
podman compose ps postgres

# Check PostgreSQL logs
podman compose logs --tail=50 postgres

# Test connectivity from API container
podman compose exec api sh -c "nc -zv postgres 5432"

# Restart PostgreSQL if needed
podman compose restart postgres
sleep 10
podman compose restart api
```

### Issue: Redis Connection Failed

**Symptoms**: API warns about Redis fallback to in-memory rate limiting.

```bash
# Check Redis status
podman compose ps redis
podman compose exec redis redis-cli ping

# Check Redis logs
podman compose logs redis

# Restart Redis
podman compose restart redis
```

**Note**: The API gracefully falls back to in-memory rate limiting if Redis is unavailable. This is non-critical but reduces distributed rate limiting capability.

### Issue: High Memory Usage

```bash
# Check container resource usage
docker stats --no-stream

# Check Go heap profile (if debug endpoints enabled)
curl http://localhost:8080/debug/pprof/heap > heap.prof

# Restart with memory limits (already configured in docker-compose.yml)
# API: 2G limit, PostgreSQL: 2G limit, Redis: 512M limit
podman compose restart api
```

### Issue: Slow API Responses

```bash
# Check database query performance
podman compose exec postgres psql -U catalogizer -c "
SELECT query, calls, mean_exec_time, total_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;"

# Check for connection pool exhaustion
podman compose exec postgres psql -U catalogizer -c "
SELECT count(*) as active FROM pg_stat_activity WHERE state = 'active';"

# Check Redis for cache hit rates
podman compose exec redis redis-cli info stats | grep keyspace
```

### Issue: SSL/TLS Certificate Expiry

```bash
# Check certificate expiration
openssl x509 -enddate -noout -in /opt/catalogizer/ssl/cert.pem

# Renew with Let's Encrypt (if applicable)
sudo certbot renew --quiet

# Restart nginx to pick up new certs
podman compose restart nginx
```

### Issue: Disk Space Full

```bash
# Check disk usage
df -h /

# Clean up Docker resources
docker system prune -f
docker volume prune -f

# Clean up old logs
find /opt/catalogizer/logs -name "*.log" -mtime +30 -delete

# Rotate PostgreSQL WAL files
podman compose exec postgres psql -U catalogizer -c "SELECT pg_switch_wal();"
```

---

## Maintenance Windows

### Planned Maintenance Procedure

```bash
# 1. Notify users (via your notification system)

# 2. Create backup
podman compose exec postgres pg_dump -U catalogizer catalogizer > \
  /opt/catalogizer/backups/pre_maintenance_$(date +%Y%m%d_%H%M%S).sql

# 3. Perform maintenance (e.g., upgrade, config change)
# ...

# 4. Verify all services
podman compose ps
curl -sf http://localhost:8080/health

# 5. Monitor logs for 10 minutes
podman compose logs -f --tail=0 api &
LOG_PID=$!
sleep 600
kill $LOG_PID

# 6. Notify users maintenance is complete
```

### Log Rotation

Add to `/etc/logrotate.d/catalogizer`:

```
/opt/catalogizer/logs/*.log {
    daily
    missingok
    rotate 30
    compress
    delaycompress
    notifempty
    create 644 root root
}
```

### Periodic Maintenance Tasks

| Task | Frequency | Command |
|------|-----------|---------|
| Database backup | Daily (2 AM) | `podman compose exec postgres pg_dump -U catalogizer catalogizer` |
| Log rotation | Daily | Handled by logrotate |
| Docker image cleanup | Weekly | `docker image prune -f --filter "until=168h"` |
| SSL cert check | Weekly | `openssl x509 -enddate -noout -in ssl/cert.pem` |
| Disk usage check | Daily | `df -h /` |
| Security updates | Monthly | `sudo apt update && sudo apt upgrade -y` |
