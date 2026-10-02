# Scaling Guide

This guide covers horizontal and vertical scaling strategies for Catalogizer, including load balancing, database scaling, WebSocket considerations, and CDN setup for static assets.

## Table of Contents

1. [Scaling Overview](#scaling-overview)
2. [Vertical Scaling](#vertical-scaling)
3. [Horizontal Scaling with Nginx Load Balancer](#horizontal-scaling-with-nginx-load-balancer)
4. [Redis Scaling](#redis-scaling)
5. [Database Scaling](#database-scaling)
6. [WebSocket Scaling with Sticky Sessions](#websocket-scaling-with-sticky-sessions)
7. [CDN for Static Assets](#cdn-for-static-assets)
8. [Performance Benchmarking](#performance-benchmarking)
9. [Capacity Planning](#capacity-planning)

---

## Scaling Overview

### Architecture at Scale

```
                    [CDN]
                      |
                   [Nginx LB]
                  /    |    \
            [API-1] [API-2] [API-3]
              |        |       |
         [Redis Cluster / Sentinel]
              |        |       |
         [PostgreSQL Primary]
              |
         [PostgreSQL Replica(s)]
```

### When to Scale

| Indicator | Threshold | Action |
|-----------|-----------|--------|
| CPU usage sustained | > 70% | Scale horizontally (add instances) |
| Memory usage sustained | > 80% | Scale vertically or horizontally |
| API p95 latency | > 2 seconds | Scale horizontally, optimize queries |
| Active connections | > 500 per instance | Add instances behind load balancer |
| WebSocket connections | > 200 per instance | Add instances with sticky sessions |
| Database connections | > 80% of max | Increase pool or add read replicas |
| Disk I/O wait | > 20% | Upgrade to NVMe SSD |

---

## Vertical Scaling

Before adding complexity with horizontal scaling, consider vertical scaling first.

### API Server

Increase resource limits in `docker-compose.yml`:

```yaml
api:
  deploy:
    resources:
      limits:
        cpus: '4'       # Increase from 2
        memory: 4G      # Increase from 2G
      reservations:
        cpus: '2'
        memory: 1G
```

### PostgreSQL

```yaml
postgres:
  deploy:
    resources:
      limits:
        cpus: '4'       # Increase from 2
        memory: 4G      # Increase from 2G
  command: >
    postgres
    -c shared_buffers=1GB
    -c effective_cache_size=3GB
    -c work_mem=16MB
    -c maintenance_work_mem=256MB
    -c max_connections=200
    -c checkpoint_completion_target=0.9
    -c wal_buffers=64MB
    -c random_page_cost=1.1
    -c effective_io_concurrency=200
```

### Redis

```yaml
redis:
  deploy:
    resources:
      limits:
        cpus: '2'
        memory: 1G
```

Update `config/redis.conf`:
```
maxmemory 768mb
```

---

## Horizontal Scaling with Nginx Load Balancer

### Step 1: Scale API Instances

Create a `docker-compose.scale.yml` override:

```yaml
version: '3.8'

services:
  api-1:
    build:
      context: ./catalog-api
      dockerfile: Dockerfile
    container_name: catalogizer-api-1
    environment:
      DATABASE_TYPE: ${DATABASE_TYPE:-postgres}
      DATABASE_HOST: postgres
      DATABASE_PORT: 5432
      DATABASE_USER: ${POSTGRES_USER:-catalogizer}
      DATABASE_PASSWORD: ${POSTGRES_PASSWORD}
      DATABASE_NAME: ${POSTGRES_DB:-catalogizer}
      REDIS_HOST: redis
      REDIS_PORT: 6379
      REDIS_PASSWORD: ${REDIS_PASSWORD:-}
      APP_ENV: ${APP_ENV:-production}
      API_PORT: 8080
      LOG_LEVEL: ${LOG_LEVEL:-info}
      JWT_SECRET: ${JWT_SECRET}
      CORS_ENABLED: ${CORS_ENABLED:-false}
      INSTANCE_ID: api-1
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    restart: unless-stopped
    networks:
      - catalogizer-network

  api-2:
    build:
      context: ./catalog-api
      dockerfile: Dockerfile
    container_name: catalogizer-api-2
    environment:
      DATABASE_TYPE: ${DATABASE_TYPE:-postgres}
      DATABASE_HOST: postgres
      DATABASE_PORT: 5432
      DATABASE_USER: ${POSTGRES_USER:-catalogizer}
      DATABASE_PASSWORD: ${POSTGRES_PASSWORD}
      DATABASE_NAME: ${POSTGRES_DB:-catalogizer}
      REDIS_HOST: redis
      REDIS_PORT: 6379
      REDIS_PASSWORD: ${REDIS_PASSWORD:-}
      APP_ENV: ${APP_ENV:-production}
      API_PORT: 8080
      LOG_LEVEL: ${LOG_LEVEL:-info}
      JWT_SECRET: ${JWT_SECRET}
      CORS_ENABLED: ${CORS_ENABLED:-false}
      INSTANCE_ID: api-2
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s
    restart: unless-stopped
    networks:
      - catalogizer-network
```

### Step 2: Update Nginx Configuration

Edit `config/nginx.conf` to add upstream servers:

```nginx
upstream catalogizer_api {
    least_conn;
    server api-1:8080 max_fails=3 fail_timeout=30s;
    server api-2:8080 max_fails=3 fail_timeout=30s;
    # Add more as needed:
    # server api-3:8080 max_fails=3 fail_timeout=30s;
    keepalive 32;
}
```

The existing `config/nginx.conf` already has the `upstream catalogizer_api` block configured with `least_conn` balancing and `keepalive 32`. Simply add additional server lines for new instances.

### Step 3: Deploy

> Uses rootless Podman Compose; never use sudo. Release artifacts are built only inside rootless build containers (constitution §11.4.173).

```bash
# Start with multiple API instances
podman compose -f docker-compose.yml -f docker-compose.scale.yml \
  --profile production up -d

# Verify all instances are healthy
podman compose -f docker-compose.yml -f docker-compose.scale.yml ps

# Test load balancing
for i in $(seq 1 10); do
  curl -s http://localhost/health | jq -r '.time'
done
```

### Load Balancing Strategies

The nginx configuration supports several strategies. Edit the `upstream` block:

```nginx
# Least connections (recommended for mixed workloads)
upstream catalogizer_api {
    least_conn;
    server api-1:8080;
    server api-2:8080;
}

# Round robin (default, good for uniform requests)
upstream catalogizer_api {
    server api-1:8080;
    server api-2:8080;
}

# IP hash (for session affinity without cookies)
upstream catalogizer_api {
    ip_hash;
    server api-1:8080;
    server api-2:8080;
}

# Weighted (for heterogeneous servers)
upstream catalogizer_api {
    server api-1:8080 weight=3;  # Gets 3x more traffic
    server api-2:8080 weight=1;
}
```

---

## Redis Scaling

### Redis Sentinel (High Availability)

For production environments requiring Redis HA:

```yaml
# docker-compose.redis-ha.yml
version: '3.8'

services:
  redis-master:
    image: redis:7-alpine
    container_name: catalogizer-redis-master
    command: redis-server /usr/local/etc/redis/redis.conf
    volumes:
      - redis_master_data:/data
      - ./config/redis.conf:/usr/local/etc/redis/redis.conf:ro
    networks:
      - catalogizer-network

  redis-replica-1:
    image: redis:7-alpine
    container_name: catalogizer-redis-replica-1
    command: redis-server --replicaof redis-master 6379 --appendonly yes
    volumes:
      - redis_replica1_data:/data
    depends_on:
      - redis-master
    networks:
      - catalogizer-network

  redis-sentinel-1:
    image: redis:7-alpine
    container_name: catalogizer-redis-sentinel-1
    command: >
      sh -c "echo 'sentinel monitor mymaster redis-master 6379 2
      sentinel down-after-milliseconds mymaster 5000
      sentinel failover-timeout mymaster 10000
      sentinel parallel-syncs mymaster 1' > /tmp/sentinel.conf &&
      redis-sentinel /tmp/sentinel.conf"
    depends_on:
      - redis-master
      - redis-replica-1
    networks:
      - catalogizer-network

volumes:
  redis_master_data:
  redis_replica1_data:
```

### Redis Cluster (Horizontal Scaling)

For workloads exceeding single-instance Redis capacity:

```bash
# Create a 6-node Redis Cluster (3 masters + 3 replicas)
docker network create redis-cluster

for i in $(seq 1 6); do
  docker run -d --name redis-$i \
    --net redis-cluster \
    -p $((7000+$i)):6379 \
    redis:7-alpine \
    redis-server --cluster-enabled yes --cluster-config-file nodes.conf \
    --cluster-node-timeout 5000 --appendonly yes
done

# Create the cluster
docker exec redis-1 redis-cli --cluster create \
  redis-1:6379 redis-2:6379 redis-3:6379 \
  redis-4:6379 redis-5:6379 redis-6:6379 \
  --cluster-replicas 1 --cluster-yes
```

Update the API configuration to use Redis Cluster by setting the `REDIS_ADDR` environment variable to point to one of the cluster nodes.

---

## Database Scaling

### SQLite Limitations

SQLite is suitable for:
- Development environments
- Single-server deployments with low to moderate traffic
- Up to approximately 100 concurrent users

SQLite limitations that trigger migration:
- Single writer at a time (readers can be concurrent in WAL mode)
- No built-in replication
- File-based -- cannot be shared across servers
- Limited to ~100 concurrent connections in practice

### Migrating from SQLite to PostgreSQL

The Catalogizer docker-compose already uses PostgreSQL in production. If you started with SQLite, migrate as follows:

```bash
# Step 1: Export data from SQLite
sqlite3 /path/to/catalogizer.db .dump > sqlite_data.sql

# Step 2: Start PostgreSQL
podman compose up -d postgres
sleep 10

# Step 3: The migration files are automatically applied
# (placed in /docker-entrypoint-initdb.d via volume mount)

# Step 4: Transform and import the data
# Note: SQLite SQL syntax may differ from PostgreSQL
# You may need to adjust data types and syntax
sed -e 's/INTEGER PRIMARY KEY AUTOINCREMENT/SERIAL PRIMARY KEY/' \
    -e 's/BOOLEAN/BOOLEAN/' \
    sqlite_data.sql > postgres_data.sql

podman compose exec -T postgres psql -U catalogizer catalogizer < postgres_data.sql

# Step 5: Update environment variables
# Set DATABASE_TYPE=postgres in .env
# Update DATABASE_HOST, DATABASE_USER, DATABASE_PASSWORD, DATABASE_NAME
```

### PostgreSQL Read Replicas

For read-heavy workloads, add PostgreSQL streaming replicas:

```yaml
# docker-compose.db-replica.yml
version: '3.8'

services:
  postgres-replica:
    image: postgres:15-alpine
    container_name: catalogizer-postgres-replica
    environment:
      POSTGRES_USER: ${POSTGRES_USER:-catalogizer}
      POSTGRES_PASSWORD: ${POSTGRES_PASSWORD}
      PGDATA: /var/lib/postgresql/data/pgdata
    command: >
      bash -c "
        until pg_basebackup -h postgres -D /var/lib/postgresql/data/pgdata -U catalogizer -X stream -P; do
          echo 'Waiting for primary...'
          sleep 5
        done
        echo 'primary_conninfo = host=postgres port=5432 user=catalogizer password=${POSTGRES_PASSWORD}' >> /var/lib/postgresql/data/pgdata/postgresql.auto.conf
        touch /var/lib/postgresql/data/pgdata/standby.signal
        exec postgres
      "
    depends_on:
      - postgres
    networks:
      - catalogizer-network
```

### PostgreSQL Connection Pooling

For high-connection scenarios, add PgBouncer:

```yaml
  pgbouncer:
    image: edoburu/pgbouncer:latest
    container_name: catalogizer-pgbouncer
    environment:
      DATABASE_URL: postgres://catalogizer:${POSTGRES_PASSWORD}@postgres:5432/catalogizer
      POOL_MODE: transaction
      MAX_CLIENT_CONN: 1000
      DEFAULT_POOL_SIZE: 20
      MIN_POOL_SIZE: 5
    ports:
      - "6432:6432"
    depends_on:
      - postgres
    networks:
      - catalogizer-network
```

Then point your API instances to PgBouncer instead of PostgreSQL directly:
```
DATABASE_HOST=pgbouncer
DATABASE_PORT=6432
```

---

## WebSocket Scaling with Sticky Sessions

Catalogizer uses WebSockets for real-time event updates (media detection events, scan progress). WebSocket connections are stateful and require sticky sessions when scaling horizontally.

### Nginx Sticky Sessions Configuration

Update the `upstream` block in `config/nginx.conf`:

```nginx
# For WebSocket connections, use IP hash for session affinity
upstream catalogizer_ws {
    ip_hash;
    server api-1:8080 max_fails=3 fail_timeout=30s;
    server api-2:8080 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

# For regular API calls, use least_conn
upstream catalogizer_api {
    least_conn;
    server api-1:8080 max_fails=3 fail_timeout=30s;
    server api-2:8080 max_fails=3 fail_timeout=30s;
    keepalive 32;
}
```

Update the WebSocket location block to use the sticky upstream:

```nginx
# WebSocket support (sticky sessions)
location /ws {
    proxy_pass http://catalogizer_ws;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

    # WebSocket timeouts (1 hour)
    proxy_read_timeout 3600s;
    proxy_send_timeout 3600s;
}

# Regular API (load balanced)
location /api/ {
    proxy_pass http://catalogizer_api;
    # ... existing proxy settings ...
}
```

### Redis Pub/Sub for Cross-Instance Events

When running multiple API instances, WebSocket events need to be broadcast across all instances. Use Redis Pub/Sub:

The API already connects to Redis (via `REDIS_ADDR` environment variable). When events occur on one instance (e.g., media detection), they should be published to a Redis channel and each instance subscribes to broadcast to its connected WebSocket clients.

Ensure all API instances share the same Redis connection:

```bash
REDIS_ADDR=redis:6379
REDIS_PASSWORD=your_redis_password
```

---

## CDN for Static Assets

### Setting Up CloudFront (AWS)

```bash
# Create CloudFront distribution pointing to your nginx origin
aws cloudfront create-distribution \
  --distribution-config '{
    "CallerReference": "catalogizer-cdn",
    "Origins": {
      "Quantity": 1,
      "Items": [{
        "Id": "catalogizer-origin",
        "DomainName": "your-server.example.com",
        "CustomOriginConfig": {
          "HTTPPort": 80,
          "HTTPSPort": 443,
          "OriginProtocolPolicy": "https-only"
        }
      }]
    },
    "DefaultCacheBehavior": {
      "TargetOriginId": "catalogizer-origin",
      "ViewerProtocolPolicy": "redirect-to-https",
      "CachePolicyId": "658327ea-f89d-4fab-a63d-7e88639e58f6",
      "Compress": true
    },
    "Enabled": true
  }'
```

### Nginx Cache Headers for Static Assets

The existing `config/nginx.conf` serves static files from `/usr/share/nginx/html`. Add caching headers:

```nginx
# Static files with long cache
location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
    root /usr/share/nginx/html;
    expires 1y;
    add_header Cache-Control "public, immutable";
    add_header Vary "Accept-Encoding";
    access_log off;
}

# HTML files with short cache (for SPA routing)
location ~* \.html$ {
    root /usr/share/nginx/html;
    expires 5m;
    add_header Cache-Control "public, must-revalidate";
}
```

### CDN Configuration for API Responses

For cacheable API responses (e.g., media metadata that rarely changes):

```nginx
location /api/v1/media/ {
    proxy_pass http://catalogizer_api;
    proxy_cache_valid 200 10m;
    add_header X-Cache-Status $upstream_cache_status;
}
```

---

## Performance Benchmarking

### Load Testing with wrk

```bash
# Install wrk
sudo apt install wrk

# Benchmark health endpoint (baseline)
wrk -t4 -c100 -d30s http://localhost:8080/health

# Benchmark API endpoint with auth
wrk -t4 -c100 -d30s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/catalog

# Benchmark with higher concurrency
wrk -t8 -c500 -d60s http://localhost:8080/health
```

### Load Testing with hey

```bash
# Install hey
go install github.com/rakyll/hey@latest

# 10000 requests, 200 concurrent
hey -n 10000 -c 200 http://localhost:8080/health

# With authentication
hey -n 10000 -c 200 -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/catalog
```

### Interpreting Results

| Metric | Good | Acceptable | Needs Scaling |
|--------|------|------------|--------------|
| p50 latency | < 50ms | < 200ms | > 500ms |
| p95 latency | < 200ms | < 1s | > 2s |
| p99 latency | < 500ms | < 2s | > 5s |
| Requests/sec | > 1000 | > 500 | < 200 |
| Error rate | 0% | < 0.1% | > 1% |

---

## Capacity Planning

### Estimated Capacity Per API Instance

| Resource | Single Instance Capacity |
|----------|------------------------|
| Concurrent HTTP connections | ~500 |
| WebSocket connections | ~200 |
| Requests per second | ~1000 (health), ~200 (catalog) |
| Memory usage | 256 MB - 2 GB |
| CPU usage | 0.5 - 2 cores |

### Scaling Formula

```
Required API instances = ceil(peak_concurrent_users / 300)
Required WebSocket instances = ceil(peak_websocket_users / 150)
```

### Instance Recommendations by User Count

| Users | API Instances | PostgreSQL | Redis | Nginx |
|-------|---------------|------------|-------|-------|
| < 100 | 1 | Single | Single | Single |
| 100 - 500 | 2 | Single + PgBouncer | Single | Single |
| 500 - 2000 | 3-4 | Primary + 1 Replica | Sentinel (3 nodes) | Single |
| 2000 - 10000 | 5-10 | Primary + 2 Replicas | Cluster (6 nodes) | 2 (HA) |
| > 10000 | 10+ | Managed (RDS/Cloud SQL) | Managed (ElastiCache) | Cloud LB |
