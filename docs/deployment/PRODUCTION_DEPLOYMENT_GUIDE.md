# Production Deployment Guide

**Version:** 1.0.0
**Last Updated:** 2026-02-10
**Target Environments:** Linux (Ubuntu/Debian/RHEL), Docker, Kubernetes

---

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Pre-Deployment Checklist](#pre-deployment-checklist)
3. [Deployment Methods](#deployment-methods)
   - [Linux Native Deployment](#linux-native-deployment)
   - [Docker Deployment](#docker-deployment)
   - [Kubernetes Deployment](#kubernetes-deployment)
4. [Configuration Management](#configuration-management)
5. [Database Setup](#database-setup)
6. [SSL/TLS Configuration](#ssltls-configuration)
7. [Reverse Proxy Setup](#reverse-proxy-setup-nginx)
8. [Service Management](#service-management)
9. [Monitoring & Health Checks](#monitoring--health-checks)
10. [Backup & Recovery](#backup--recovery)
11. [Rollback Procedures](#rollback-procedures)
12. [Post-Deployment Validation](#post-deployment-validation)
13. [Troubleshooting](#troubleshooting)

---

## Prerequisites

### System Requirements

**Minimum Requirements:**
- **CPU:** 2 cores
- **RAM:** 4 GB
- **Disk:** 20 GB (+ storage for media files)
- **OS:** Linux (Ubuntu 20.04+, Debian 11+, RHEL 8+, Rocky Linux 8+)

**Recommended for Production:**
- **CPU:** 4+ cores
- **RAM:** 8+ GB
- **Disk:** 50 GB SSD (+ storage for media files)
- **Network:** 1 Gbps

### Software Dependencies

**Backend (catalog-api):**
- Go 1.21+ (for native deployment)
- PostgreSQL 13+ OR SQLite 3.35+ (with cipher support)
- Redis 6+ (optional, for rate limiting)

**Frontend (catalog-web):**
- Node.js 18+ (for building)
- Nginx or Apache (for serving static files)

**Container Deployments:**
- Docker 20.10+ OR Podman 4.0+
- Docker Compose 2.0+ OR Podman Compose

**Kubernetes Deployments:**
- Kubernetes 1.24+
- Helm 3.10+ (optional, recommended)
- kubectl configured for target cluster

### Network Requirements

**Required Ports:**
- **8080** - API server (internal)
- **80** - HTTP (redirect to HTTPS)
- **443** - HTTPS (public)
- **5432** - PostgreSQL (if external database)
- **6379** - Redis (if external cache)

**Firewall Rules:**
- Allow incoming traffic on ports 80, 443
- Allow outgoing traffic for external APIs (TMDB, OMDB, IMDB)
- Allow database connections (PostgreSQL/Redis) from application servers

---

## Pre-Deployment Checklist

### Security

- [ ] SSL/TLS certificates obtained (Let's Encrypt or commercial CA)
- [ ] Strong passwords generated for all services
- [ ] JWT secret generated (256-bit minimum)
- [ ] Admin password changed from default
- [ ] Database credentials secured
- [ ] API keys for external services obtained (TMDB, OMDB)
- [ ] Firewall rules configured
- [ ] Security scanning completed (zero HIGH severity vulnerabilities)
- [ ] Rate limiting configured

### Configuration

- [ ] Environment variables reviewed and set
- [ ] Database connection settings configured
- [ ] Redis connection settings configured (if using)
- [ ] CORS origins configured for frontend domain
- [ ] File upload limits set appropriately
- [ ] Session timeout configured
- [ ] Logging levels set (INFO or WARN for production)
- [ ] Backup schedule configured

### Infrastructure

- [ ] DNS records configured (A/AAAA for domain)
- [ ] Load balancer configured (if multi-instance)
- [ ] Reverse proxy configured (nginx/Apache)
- [ ] CDN configured (optional, for static assets)
- [ ] Storage volumes mounted (for media files)
- [ ] Backup storage configured
- [ ] Monitoring system configured

### Testing

- [ ] All tests passing (go test ./...)
- [ ] Integration tests validated
- [ ] Stress tests validated
- [ ] Security scan completed
- [ ] Staging environment validated
- [ ] Rollback procedure tested

---

## Deployment Methods

### Linux Native Deployment

#### Step 1: Prepare the Server

```bash
# Update system
sudo apt update && sudo apt upgrade -y  # Ubuntu/Debian
# OR
sudo dnf update -y  # RHEL/Rocky Linux

# Install dependencies
sudo apt install -y build-essential git postgresql postgresql-contrib redis-server nginx
# OR
sudo dnf install -y gcc git postgresql postgresql-server redis nginx

# Create application user
sudo useradd -r -s /bin/bash -d /opt/catalogizer -m catalogizer

# Create required directories
sudo mkdir -p /opt/catalogizer/{api,web,logs,data}
sudo chown -R catalogizer:catalogizer /opt/catalogizer
```

#### Step 2: Build and Deploy Backend

```bash
# Clone repository (or upload build artifacts)
cd /tmp
git clone https://github.com/your-org/catalogizer.git
cd catalogizer

# Build API server
cd catalog-api
go build -o catalog-api -ldflags="-s -w" main.go

# Copy binary to production location
sudo cp catalog-api /opt/catalogizer/api/
sudo chown catalogizer:catalogizer /opt/catalogizer/api/catalog-api
sudo chmod +x /opt/catalogizer/api/catalog-api

# Copy configuration
sudo cp config/production.env /opt/catalogizer/api/.env
sudo chown catalogizer:catalogizer /opt/catalogizer/api/.env
sudo chmod 600 /opt/catalogizer/api/.env
```

#### Step 3: Build and Deploy Frontend

```bash
# Build frontend
cd /tmp/catalogizer/catalog-web
npm ci --production
npm run build

# Copy to web root
sudo cp -r dist/* /opt/catalogizer/web/
sudo chown -R catalogizer:www-data /opt/catalogizer/web
sudo chmod -R 755 /opt/catalogizer/web
```

#### Step 4: Configure Systemd Services

Create `/etc/systemd/system/catalogizer-api.service`:

```ini
[Unit]
Description=Catalogizer API Server
After=network.target postgresql.service redis.service
Wants=postgresql.service redis.service

[Service]
Type=simple
User=catalogizer
Group=catalogizer
WorkingDirectory=/opt/catalogizer/api
EnvironmentFile=/opt/catalogizer/api/.env
ExecStart=/opt/catalogizer/api/catalog-api
Restart=always
RestartSec=10
StandardOutput=append:/opt/catalogizer/logs/api.log
StandardError=append:/opt/catalogizer/logs/api-error.log

# Security hardening
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/catalogizer/data /opt/catalogizer/logs

# Resource limits
LimitNOFILE=65536
LimitNPROC=4096

[Install]
WantedBy=multi-user.target
```

Enable and start services:

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable services
sudo systemctl enable catalogizer-api

# Start services
sudo systemctl start catalogizer-api

# Check status
sudo systemctl status catalogizer-api
```

---

### Docker Deployment

#### Step 1: Prepare Docker Environment

```bash
# Install Docker (if not already installed)
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER

# OR use Podman
sudo apt install -y podman podman-compose
```

#### Step 2: Create Production Docker Compose

Create `docker-compose.prod.yml`:

```yaml
version: '3.8'

services:
  # PostgreSQL Database
  postgres:
    image: postgres:15-alpine
    container_name: catalogizer-postgres
    restart: unless-stopped
    environment:
      POSTGRES_DB: catalogizer
      POSTGRES_USER: catalogizer
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./database/migrations:/docker-entrypoint-initdb.d:ro
    networks:
      - catalogizer-network
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U catalogizer"]
      interval: 10s
      timeout: 5s
      retries: 5

  # Redis Cache
  redis:
    image: redis:7-alpine
    container_name: catalogizer-redis
    restart: unless-stopped
    command: redis-server --requirepass ${REDIS_PASSWORD}
    volumes:
      - redis_data:/data
    networks:
      - catalogizer-network
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 5

  # Catalogizer API
  api:
    build:
      context: ./catalog-api
      dockerfile: Dockerfile.prod
    container_name: catalogizer-api
    restart: unless-stopped
    environment:
      PORT: 8080
      GIN_MODE: release
      DB_TYPE: postgres
      DB_HOST: postgres
      DB_PORT: 5432
      DB_NAME: catalogizer
      DB_USER: catalogizer
      DB_PASSWORD: ${DB_PASSWORD}
      REDIS_HOST: redis
      REDIS_PORT: 6379
      REDIS_PASSWORD: ${REDIS_PASSWORD}
      JWT_SECRET: ${JWT_SECRET}
      ADMIN_PASSWORD: ${ADMIN_PASSWORD}
      TMDB_API_KEY: ${TMDB_API_KEY}
      OMDB_API_KEY: ${OMDB_API_KEY}
    volumes:
      - api_logs:/app/logs
      - media_storage:/media
    depends_on:
      postgres:
        condition: service_healthy
      redis:
        condition: service_healthy
    networks:
      - catalogizer-network
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  # Nginx Reverse Proxy
  nginx:
    image: nginx:alpine
    container_name: catalogizer-nginx
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./config/nginx.prod.conf:/etc/nginx/nginx.conf:ro
      - ./catalog-web/dist:/usr/share/nginx/html:ro
      - ./ssl:/etc/nginx/ssl:ro
      - nginx_logs:/var/log/nginx
    depends_on:
      - api
    networks:
      - catalogizer-network
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost/health"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  postgres_data:
    driver: local
  redis_data:
    driver: local
  api_logs:
    driver: local
  nginx_logs:
    driver: local
  media_storage:
    driver: local
    driver_opts:
      type: none
      o: bind
      device: /mnt/media  # Mount point for media storage

networks:
  catalogizer-network:
    driver: bridge
```

#### Step 3: Create Production Dockerfile

Create `catalog-api/Dockerfile.prod`:

```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build with optimizations
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo \
    -ldflags="-s -w -X main.version=$(git describe --tags --always)" \
    -o catalog-api main.go

# Production stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata wget

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/catalog-api .
COPY --from=builder /build/database/migrations ./database/migrations

# Create non-root user
RUN addgroup -g 1000 catalogizer && \
    adduser -D -u 1000 -G catalogizer catalogizer && \
    chown -R catalogizer:catalogizer /app

USER catalogizer

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

CMD ["./catalog-api"]
```

#### Step 4: Deploy with Docker Compose

```bash
# Create .env file for secrets
cat > .env <<EOF
DB_PASSWORD=$(openssl rand -base64 32)
REDIS_PASSWORD=$(openssl rand -base64 32)
JWT_SECRET=$(openssl rand -base64 64)
ADMIN_PASSWORD=$(openssl rand -base64 16)
TMDB_API_KEY=your_tmdb_api_key
OMDB_API_KEY=your_omdb_api_key
EOF

# Secure the .env file
chmod 600 .env

# Build and start services
docker-compose -f docker-compose.prod.yml up -d

# Check logs
docker-compose -f docker-compose.prod.yml logs -f

# Check health
docker-compose -f docker-compose.prod.yml ps
```

---

### Kubernetes Deployment

#### Step 1: Create Namespace

```bash
kubectl create namespace catalogizer
```

#### Step 2: Create Secrets

```bash
# Create database secret
kubectl create secret generic catalogizer-db-secret \
  --from-literal=password=$(openssl rand -base64 32) \
  -n catalogizer

# Create JWT secret
kubectl create secret generic catalogizer-jwt-secret \
  --from-literal=secret=$(openssl rand -base64 64) \
  -n catalogizer

# Create API keys secret
kubectl create secret generic catalogizer-api-keys \
  --from-literal=tmdb_key=your_tmdb_key \
  --from-literal=omdb_key=your_omdb_key \
  -n catalogizer
```

#### Step 3: Deploy PostgreSQL

Create `k8s/postgres-deployment.yaml`:

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: postgres-pvc
  namespace: catalogizer
spec:
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 10Gi
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
  namespace: catalogizer
spec:
  replicas: 1
  selector:
    matchLabels:
      app: postgres
  template:
    metadata:
      labels:
        app: postgres
    spec:
      containers:
      - name: postgres
        image: postgres:15-alpine
        env:
        - name: POSTGRES_DB
          value: catalogizer
        - name: POSTGRES_USER
          value: catalogizer
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: catalogizer-db-secret
              key: password
        ports:
        - containerPort: 5432
        volumeMounts:
        - name: postgres-storage
          mountPath: /var/lib/postgresql/data
        livenessProbe:
          exec:
            command:
            - pg_isready
            - -U
            - catalogizer
          initialDelaySeconds: 30
          periodSeconds: 10
      volumes:
      - name: postgres-storage
        persistentVolumeClaim:
          claimName: postgres-pvc
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: catalogizer
spec:
  selector:
    app: postgres
  ports:
  - port: 5432
    targetPort: 5432
```

#### Step 4: Deploy API Server

Create `k8s/api-deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalogizer-api
  namespace: catalogizer
spec:
  replicas: 3
  selector:
    matchLabels:
      app: catalogizer-api
  template:
    metadata:
      labels:
        app: catalogizer-api
    spec:
      containers:
      - name: api
        image: your-registry/catalogizer-api:latest
        env:
        - name: PORT
          value: "8080"
        - name: GIN_MODE
          value: "release"
        - name: DB_TYPE
          value: "postgres"
        - name: DB_HOST
          value: "postgres"
        - name: DB_PORT
          value: "5432"
        - name: DB_NAME
          value: "catalogizer"
        - name: DB_USER
          value: "catalogizer"
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: catalogizer-db-secret
              key: password
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: catalogizer-jwt-secret
              key: secret
        - name: TMDB_API_KEY
          valueFrom:
            secretKeyRef:
              name: catalogizer-api-keys
              key: tmdb_key
        ports:
        - containerPort: 8080
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "2000m"
---
apiVersion: v1
kind: Service
metadata:
  name: catalogizer-api
  namespace: catalogizer
spec:
  type: ClusterIP
  selector:
    app: catalogizer-api
  ports:
  - port: 8080
    targetPort: 8080
```

#### Step 5: Deploy Ingress

Create `k8s/ingress.yaml`:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: catalogizer-ingress
  namespace: catalogizer
  annotations:
    cert-manager.io/cluster-issuer: "letsencrypt-prod"
    nginx.ingress.kubernetes.io/proxy-body-size: "100m"
    nginx.ingress.kubernetes.io/rate-limit: "100"
spec:
  ingressClassName: nginx
  tls:
  - hosts:
    - catalogizer.yourdomain.com
    secretName: catalogizer-tls
  rules:
  - host: catalogizer.yourdomain.com
    http:
      paths:
      - path: /api/v1
        pathType: Prefix
        backend:
          service:
            name: catalogizer-api
            port:
              number: 8080
      - path: /
        pathType: Prefix
        backend:
          service:
            name: catalogizer-web
            port:
              number: 80
```

#### Step 6: Apply Kubernetes Configurations

```bash
# Apply all configurations
kubectl apply -f k8s/postgres-deployment.yaml
kubectl apply -f k8s/api-deployment.yaml
kubectl apply -f k8s/ingress.yaml

# Wait for rollout
kubectl rollout status deployment/catalogizer-api -n catalogizer

# Check pods
kubectl get pods -n catalogizer

# Check services
kubectl get svc -n catalogizer

# Check ingress
kubectl get ingress -n catalogizer
```

---

## Configuration Management

### Environment Variables

Create `/opt/catalogizer/api/.env` (Linux) or `.env` file (Docker):

```bash
# Server Configuration
PORT=8080
GIN_MODE=release
ALLOWED_ORIGINS=https://catalogizer.yourdomain.com

# Database Configuration (PostgreSQL)
DB_TYPE=postgres
DB_HOST=localhost
DB_PORT=5432
DB_NAME=catalogizer
DB_USER=catalogizer
DB_PASSWORD=your_secure_password_here
DB_SSL_MODE=require
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5
DB_CONN_MAX_LIFETIME=5m

# OR SQLite Configuration
# DB_TYPE=sqlite
# DB_PATH=/opt/catalogizer/data/catalogizer.db

# Redis Configuration (Optional)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=your_redis_password
REDIS_DB=0

# Authentication
JWT_SECRET=your_256_bit_secret_here
JWT_EXPIRATION=24h
ADMIN_PASSWORD=<your-admin-password>

# External APIs
TMDB_API_KEY=your_tmdb_api_key
OMDB_API_KEY=your_omdb_api_key
IMDB_API_KEY=your_imdb_api_key

# File Upload
MAX_UPLOAD_SIZE=104857600  # 100MB
ALLOWED_FILE_TYPES=.mp4,.mkv,.avi,.mov,.mp3,.flac,.jpg,.png

# Rate Limiting
RATE_LIMIT_ENABLED=true
RATE_LIMIT_REQUESTS=100
RATE_LIMIT_WINDOW=1m

# Logging
LOG_LEVEL=info
LOG_FORMAT=json
LOG_FILE=/opt/catalogizer/logs/api.log

# Session
SESSION_TIMEOUT=30m
SESSION_COOKIE_SECURE=true
SESSION_COOKIE_HTTPONLY=true
SESSION_COOKIE_SAMESITE=strict

# CORS
CORS_ENABLED=true
CORS_MAX_AGE=86400
```

### Configuration Validation

```bash
# Validate configuration
/opt/catalogizer/api/catalog-api --validate-config

# Check environment variables
/opt/catalogizer/api/catalog-api --check-env
```

---

## Database Setup

### PostgreSQL Production Setup

```bash
# Install PostgreSQL
sudo apt install -y postgresql postgresql-contrib

# Create database and user
sudo -u postgres psql <<EOF
CREATE DATABASE catalogizer;
CREATE USER catalogizer WITH ENCRYPTED PASSWORD 'your_secure_password';
GRANT ALL PRIVILEGES ON DATABASE catalogizer TO catalogizer;
\c catalogizer
GRANT ALL ON SCHEMA public TO catalogizer;
EOF

# Run migrations
cd /opt/catalogizer/api
./catalog-api migrate up

# Verify migrations
sudo -u postgres psql -d catalogizer -c "\dt"
```

### SQLite Production Setup (Small Deployments)

```bash
# Create data directory
sudo mkdir -p /opt/catalogizer/data
sudo chown catalogizer:catalogizer /opt/catalogizer/data

# Database will be created automatically on first run
# Ensure environment variable is set:
# DB_TYPE=sqlite
# DB_PATH=/opt/catalogizer/data/catalogizer.db
```

### Database Tuning (PostgreSQL)

Edit `/etc/postgresql/15/main/postgresql.conf`:

```ini
# Connection settings
max_connections = 100
shared_buffers = 256MB
effective_cache_size = 1GB
maintenance_work_mem = 64MB
checkpoint_completion_target = 0.9
wal_buffers = 16MB
default_statistics_target = 100
random_page_cost = 1.1
effective_io_concurrency = 200
work_mem = 2621kB
min_wal_size = 1GB
max_wal_size = 4GB

# Enable connection pooling
ssl = on
ssl_cert_file = '/etc/ssl/certs/ssl-cert-snakeoil.pem'
ssl_key_file = '/etc/ssl/private/ssl-cert-snakeoil.key'
```

Restart PostgreSQL:

```bash
sudo systemctl restart postgresql
```

---

## SSL/TLS Configuration

### Option 1: Let's Encrypt (Recommended)

```bash
# Install Certbot
sudo apt install -y certbot python3-certbot-nginx

# Obtain certificate
sudo certbot --nginx -d catalogizer.yourdomain.com

# Auto-renewal is configured by default
# Test renewal
sudo certbot renew --dry-run
```

### Option 2: Commercial Certificate

```bash
# Generate CSR
openssl req -new -newkey rsa:2048 -nodes \
  -keyout catalogizer.key \
  -out catalogizer.csr \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=catalogizer.yourdomain.com"

# Submit CSR to CA and receive certificate
# Copy certificate files to /etc/nginx/ssl/

sudo mkdir -p /etc/nginx/ssl
sudo cp catalogizer.crt /etc/nginx/ssl/
sudo cp catalogizer.key /etc/nginx/ssl/
sudo cp ca-bundle.crt /etc/nginx/ssl/
sudo chmod 600 /etc/nginx/ssl/catalogizer.key
```

---

## Reverse Proxy Setup (Nginx)

Create `/etc/nginx/sites-available/catalogizer`:

```nginx
# Rate limiting
limit_req_zone $binary_remote_addr zone=api_limit:10m rate=10r/s;
limit_req_zone $binary_remote_addr zone=auth_limit:10m rate=5r/s;

# Upstream API servers
upstream catalogizer_api {
    least_conn;
    server 127.0.0.1:8080 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

# HTTP -> HTTPS redirect
server {
    listen 80;
    listen [::]:80;
    server_name catalogizer.yourdomain.com;

    location /.well-known/acme-challenge/ {
        root /var/www/html;
    }

    location / {
        return 301 https://$server_name$request_uri;
    }
}

# HTTPS server
server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name catalogizer.yourdomain.com;

    # SSL configuration
    ssl_certificate /etc/nginx/ssl/catalogizer.crt;
    ssl_certificate_key /etc/nginx/ssl/catalogizer.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;
    ssl_session_cache shared:SSL:10m;
    ssl_session_timeout 10m;

    # Security headers
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    # Client body size
    client_max_body_size 100M;

    # Frontend static files
    root /opt/catalogizer/web;
    index index.html;

    # API proxy
    location /api/v1 {
        limit_req zone=api_limit burst=20 nodelay;

        proxy_pass http://catalogizer_api;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header X-Request-ID $request_id;

        proxy_connect_timeout 60s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;

        proxy_buffering off;
        proxy_cache_bypass $http_upgrade;
    }

    # Auth endpoints (stricter rate limiting)
    location ~ ^/api/v1/(auth|signup|login) {
        limit_req zone=auth_limit burst=5 nodelay;

        proxy_pass http://catalogizer_api;
        proxy_http_version 1.1;
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }

    # WebSocket support
    location /ws {
        proxy_pass http://catalogizer_api;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host $http_host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;

        proxy_connect_timeout 7d;
        proxy_send_timeout 7d;
        proxy_read_timeout 7d;
    }

    # Health check endpoint
    location /health {
        proxy_pass http://catalogizer_api;
        access_log off;
    }

    # Frontend routes (SPA)
    location / {
        try_files $uri $uri/ /index.html;
    }

    # Static assets caching
    location ~* \.(jpg|jpeg|png|gif|ico|css|js|svg|woff|woff2|ttf|eot)$ {
        expires 1y;
        add_header Cache-Control "public, immutable";
    }

    # Logging
    access_log /var/log/nginx/catalogizer-access.log combined;
    error_log /var/log/nginx/catalogizer-error.log warn;
}
```

Enable the site:

```bash
# Create symlink
sudo ln -s /etc/nginx/sites-available/catalogizer /etc/nginx/sites-enabled/

# Test configuration
sudo nginx -t

# Reload nginx
sudo systemctl reload nginx
```

---

## Service Management

### Systemd Commands

```bash
# Start service
sudo systemctl start catalogizer-api

# Stop service
sudo systemctl stop catalogizer-api

# Restart service
sudo systemctl restart catalogizer-api

# Reload configuration
sudo systemctl reload catalogizer-api

# Enable on boot
sudo systemctl enable catalogizer-api

# Disable on boot
sudo systemctl disable catalogizer-api

# Check status
sudo systemctl status catalogizer-api

# View logs
sudo journalctl -u catalogizer-api -f

# View recent logs
sudo journalctl -u catalogizer-api --since "1 hour ago"
```

### Docker Commands

```bash
# Start services
docker-compose -f docker-compose.prod.yml up -d

# Stop services
docker-compose -f docker-compose.prod.yml down

# Restart service
docker-compose -f docker-compose.prod.yml restart api

# View logs
docker-compose -f docker-compose.prod.yml logs -f api

# Check health
docker-compose -f docker-compose.prod.yml ps
```

### Kubernetes Commands

```bash
# Scale deployment
kubectl scale deployment catalogizer-api --replicas=5 -n catalogizer

# Rolling update
kubectl set image deployment/catalogizer-api api=your-registry/catalogizer-api:v1.1.0 -n catalogizer

# Rollback
kubectl rollout undo deployment/catalogizer-api -n catalogizer

# View logs
kubectl logs -f deployment/catalogizer-api -n catalogizer

# Execute command in pod
kubectl exec -it deployment/catalogizer-api -n catalogizer -- /bin/sh
```

---

## Monitoring & Health Checks

### Health Check Endpoint

**Endpoint:** `GET /health`

**Expected Response:**
```json
{
  "status": "ok",
  "timestamp": "2026-02-10T10:00:00Z",
  "version": "1.0.0",
  "database": "connected",
  "redis": "connected"
}
```

### Monitoring Setup

#### Prometheus Metrics

Prometheus metrics are fully implemented and exposed at the `/metrics` endpoint. The API server registers metrics via `promhttp.Handler()` in `main.go`:

```
GET /metrics
```

**Available metric families:**

| Metric | Type | Description |
|--------|------|-------------|
| `catalogizer_http_request_duration_seconds` | Histogram | HTTP request latency by method, path, status |
| `catalogizer_http_requests_total` | Counter | Total HTTP requests by method, path, status |
| `catalogizer_http_request_size_bytes` | Summary | Request body sizes |
| `catalogizer_http_response_size_bytes` | Summary | Response body sizes |
| `catalogizer_active_connections` | Gauge | Current active HTTP connections |
| `catalogizer_db_queries_total` | Counter | Database queries by operation and table |
| `catalogizer_db_query_duration_seconds` | Histogram | Database query latency |
| `catalogizer_db_connections_active` | Gauge | Active database connections |
| `catalogizer_db_connections_idle` | Gauge | Idle database connections |
| `catalogizer_cache_hits_total` | Counter | Cache hits by cache name |
| `catalogizer_cache_misses_total` | Counter | Cache misses by cache name |
| `catalogizer_media_files_scanned_total` | Counter | Total media files scanned |
| `catalogizer_media_files_analyzed_total` | Counter | Total media files analyzed |
| `catalogizer_media_analysis_duration_seconds` | Histogram | Media analysis duration |
| `catalogizer_media_by_type` | Gauge | Media count by type |
| `catalogizer_memory_usage_bytes` | Gauge | Process memory usage |
| `catalogizer_goroutines` | Gauge | Current goroutine count |

**Prometheus scrape config example:**

```yaml
scrape_configs:
  - job_name: 'catalogizer-api'
    scrape_interval: 15s
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: /metrics
```

#### Simple Monitoring Script

Create `/opt/catalogizer/scripts/health-check.sh`:

```bash
#!/bin/bash

API_URL="http://localhost:8080/health"
ALERT_EMAIL="admin@yourdomain.com"

response=$(curl -s -o /dev/null -w "%{http_code}" $API_URL)

if [ "$response" != "200" ]; then
    echo "API health check failed: HTTP $response" | \
        mail -s "Catalogizer API Health Check Failed" $ALERT_EMAIL
    exit 1
fi

echo "Health check passed"
exit 0
```

Add to crontab:

```bash
# Check every 5 minutes
*/5 * * * * /opt/catalogizer/scripts/health-check.sh
```

---

## Backup & Recovery

### Database Backup

#### PostgreSQL Backup Script

Create `/opt/catalogizer/scripts/backup-db.sh`:

```bash
#!/bin/bash

BACKUP_DIR="/opt/catalogizer/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
DB_NAME="catalogizer"
RETENTION_DAYS=30

# Create backup directory
mkdir -p $BACKUP_DIR

# Backup database
pg_dump -U catalogizer -h localhost $DB_NAME | \
    gzip > $BACKUP_DIR/catalogizer_db_$TIMESTAMP.sql.gz

# Remove old backups
find $BACKUP_DIR -name "catalogizer_db_*.sql.gz" -mtime +$RETENTION_DAYS -delete

echo "Backup completed: catalogizer_db_$TIMESTAMP.sql.gz"
```

#### Automated Backup

```bash
# Daily backup at 2 AM
0 2 * * * /opt/catalogizer/scripts/backup-db.sh
```

### Restore from Backup

```bash
# Stop API server
sudo systemctl stop catalogizer-api

# Restore database
gunzip < /opt/catalogizer/backups/catalogizer_db_20260210_020000.sql.gz | \
    psql -U catalogizer -h localhost catalogizer

# Start API server
sudo systemctl start catalogizer-api
```

### Full System Backup

```bash
# Backup script
#!/bin/bash

BACKUP_ROOT="/mnt/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)

# Create backup directory
mkdir -p $BACKUP_ROOT/$TIMESTAMP

# Backup database
pg_dump -U catalogizer catalogizer | gzip > $BACKUP_ROOT/$TIMESTAMP/database.sql.gz

# Backup configuration
tar -czf $BACKUP_ROOT/$TIMESTAMP/config.tar.gz \
    /opt/catalogizer/api/.env \
    /etc/nginx/sites-available/catalogizer \
    /etc/systemd/system/catalogizer-api.service

# Backup application
tar -czf $BACKUP_ROOT/$TIMESTAMP/application.tar.gz \
    /opt/catalogizer/api \
    /opt/catalogizer/web

# Backup logs (last 7 days)
tar -czf $BACKUP_ROOT/$TIMESTAMP/logs.tar.gz \
    --newer-mtime="7 days ago" \
    /opt/catalogizer/logs

echo "Full backup completed: $BACKUP_ROOT/$TIMESTAMP"
```

---

## Rollback Procedures

### Application Rollback

#### Systemd Deployment

```bash
# Stop current version
sudo systemctl stop catalogizer-api

# Restore previous version
sudo cp /opt/catalogizer/backups/catalog-api.v1.0.0 /opt/catalogizer/api/catalog-api

# Start service
sudo systemctl start catalogizer-api

# Verify
sudo systemctl status catalogizer-api
curl http://localhost:8080/health
```

#### Docker Deployment

```bash
# Rollback to previous image
docker-compose -f docker-compose.prod.yml down
docker tag your-registry/catalogizer-api:v1.0.0 your-registry/catalogizer-api:latest
docker-compose -f docker-compose.prod.yml up -d

# Verify
docker-compose -f docker-compose.prod.yml ps
docker-compose -f docker-compose.prod.yml logs api
```

#### Kubernetes Deployment

```bash
# Rollback to previous revision
kubectl rollout undo deployment/catalogizer-api -n catalogizer

# Rollback to specific revision
kubectl rollout undo deployment/catalogizer-api --to-revision=2 -n catalogizer

# Check rollout status
kubectl rollout status deployment/catalogizer-api -n catalogizer

# Verify
kubectl get pods -n catalogizer
```

### Database Rollback

```bash
# Stop API
sudo systemctl stop catalogizer-api

# Restore from backup
gunzip < /opt/catalogizer/backups/catalogizer_db_before_migration.sql.gz | \
    psql -U catalogizer catalogizer

# Start API
sudo systemctl start catalogizer-api
```

---

## Post-Deployment Validation

### Validation Checklist

```bash
# 1. Check service status
sudo systemctl status catalogizer-api
# Expected: Active (running)

# 2. Check API health
curl https://catalogizer.yourdomain.com/health
# Expected: {"status":"ok", ...}

# 3. Check frontend loads
curl -I https://catalogizer.yourdomain.com
# Expected: HTTP/2 200

# 4. Test authentication
curl -X POST https://catalogizer.yourdomain.com/api/v1/auth/login \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"your_password"}'
# Expected: {"token":"..."}

# 5. Check database connectivity
sudo -u postgres psql -d catalogizer -c "SELECT COUNT(*) FROM users;"
# Expected: Numeric result

# 6. Check logs for errors
sudo journalctl -u catalogizer-api --since "10 minutes ago" | grep -i error
# Expected: No critical errors

# 7. Check SSL certificate
echo | openssl s_client -connect catalogizer.yourdomain.com:443 2>/dev/null | \
    openssl x509 -noout -dates
# Expected: Valid dates

# 8. Test CORS
curl -H "Origin: https://catalogizer.yourdomain.com" \
    -H "Access-Control-Request-Method: POST" \
    -X OPTIONS https://catalogizer.yourdomain.com/api/v1/auth/login -v
# Expected: CORS headers present

# 9. Check reverse proxy
curl -I https://catalogizer.yourdomain.com/api/v1/health
# Expected: HTTP/2 200

# 10. Test WebSocket connection (if applicable)
# Use browser developer tools to verify WebSocket connection
```

### Performance Validation

```bash
# Run Apache Bench test
ab -n 1000 -c 10 https://catalogizer.yourdomain.com/api/v1/health

# Expected results:
# - Requests per second > 100
# - 99% percentile < 200ms
# - 0% failed requests
```

---

## Troubleshooting

### Common Issues

#### 1. API Won't Start

**Symptoms:** Service fails to start, exits immediately

**Diagnosis:**
```bash
sudo journalctl -u catalogizer-api -n 50
```

**Common Causes:**
- Database connection failure
- Port already in use
- Missing environment variables
- Invalid configuration

**Solutions:**
```bash
# Check database connectivity
psql -U catalogizer -h localhost -d catalogizer

# Check port availability
sudo netstat -tlnp | grep 8080

# Verify environment variables
sudo -u catalogizer /opt/catalogizer/api/catalog-api --check-env

# Validate configuration
sudo -u catalogizer /opt/catalogizer/api/catalog-api --validate-config
```

#### 2. Database Connection Errors

**Symptoms:** "connection refused" or "authentication failed"

**Solutions:**
```bash
# Check PostgreSQL is running
sudo systemctl status postgresql

# Verify database exists
sudo -u postgres psql -l | grep catalogizer

# Test connection
psql -U catalogizer -h localhost -d catalogizer

# Check pg_hba.conf
sudo cat /etc/postgresql/15/main/pg_hba.conf | grep catalogizer
```

#### 3. High Memory Usage

**Symptoms:** OOM killer terminates process

**Diagnosis:**
```bash
# Check memory usage
free -h
ps aux | grep catalog-api

# Check for memory leaks
sudo journalctl -u catalogizer-api | grep -i "out of memory"
```

**Solutions:**
```bash
# Adjust database connection pool
# In .env file:
DB_MAX_OPEN_CONNS=10  # Reduce from 25
DB_MAX_IDLE_CONNS=2   # Reduce from 5

# Add memory limits (systemd)
sudo systemctl edit catalogizer-api

# Add:
[Service]
MemoryLimit=2G
MemoryHigh=1.5G

# Restart
sudo systemctl daemon-reload
sudo systemctl restart catalogizer-api
```

#### 4. Slow Response Times

**Symptoms:** API requests taking > 1s

**Diagnosis:**
```bash
# Check database query performance
sudo -u postgres psql -d catalogizer -c "SELECT query, mean_exec_time, calls FROM pg_stat_statements ORDER BY mean_exec_time DESC LIMIT 10;"

# Check system load
uptime
iostat -x 1 5
```

**Solutions:**
```bash
# Enable query logging
# In /etc/postgresql/15/main/postgresql.conf:
log_min_duration_statement = 1000  # Log queries > 1s

# Analyze slow queries
sudo -u postgres psql -d catalogizer -c "EXPLAIN ANALYZE SELECT ..."

# Add indexes if needed
sudo -u postgres psql -d catalogizer -c "CREATE INDEX idx_files_path ON files(path);"
```

#### 5. SSL Certificate Issues

**Symptoms:** Browser shows "certificate invalid"

**Solutions:**
```bash
# Check certificate expiry
echo | openssl s_client -connect catalogizer.yourdomain.com:443 2>/dev/null | \
    openssl x509 -noout -dates

# Renew Let's Encrypt certificate
sudo certbot renew

# Check nginx SSL configuration
sudo nginx -t

# Reload nginx
sudo systemctl reload nginx
```

---

## Security Best Practices

### Production Security Checklist

- [ ] Change all default passwords
- [ ] Use strong JWT secret (256-bit minimum)
- [ ] Enable HTTPS only (redirect HTTP to HTTPS)
- [ ] Configure firewall (ufw/iptables/firewalld)
- [ ] Enable rate limiting
- [ ] Set secure cookie flags (Secure, HttpOnly, SameSite)
- [ ] Disable unnecessary services
- [ ] Keep system updated (apt/dnf update)
- [ ] Enable automatic security updates
- [ ] Configure fail2ban for brute force protection
- [ ] Use non-root user for application
- [ ] Configure database connection encryption (SSL mode for PostgreSQL)
- [ ] Regular security audits
- [ ] Monitor logs for suspicious activity
- [ ] Set up intrusion detection (OSSEC/Wazuh)

### Firewall Configuration

```bash
# UFW (Ubuntu)
sudo ufw allow 80/tcp
sudo ufw allow 443/tcp
sudo ufw allow 22/tcp
sudo ufw enable

# Firewalld (RHEL)
sudo firewall-cmd --permanent --add-service=http
sudo firewall-cmd --permanent --add-service=https
sudo firewall-cmd --permanent --add-service=ssh
sudo firewall-cmd --reload
```

---

## Support & Additional Resources

### Documentation
- [API Documentation](../api/API_DOCUMENTATION.md)
- [Configuration Reference](../guides/CONFIGURATION_REFERENCE.md)
- [Development Setup](../guides/DEVELOPMENT_SETUP.md)
- [Test Validation Summary](../testing/TEST_VALIDATION_SUMMARY.md)

### Getting Help
- GitHub Issues: https://github.com/your-org/catalogizer/issues
- Documentation: https://docs.catalogizer.com
- Community Forum: https://community.catalogizer.com

---

**Document Version:** 1.0.0
**Last Updated:** 2026-02-10
**Maintained By:** Catalogizer Development Team
