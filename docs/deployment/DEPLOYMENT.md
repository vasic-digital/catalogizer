# Catalogizer Deployment Guide

This guide covers various deployment scenarios for Catalogizer, from development setups to production environments with high availability.

## Table of Contents

- [Quick Start](#quick-start)
- [Development Deployment](#development-deployment)
- [Production Deployment](#production-deployment)
- [Docker Deployment](#docker-deployment)
- [Kubernetes Deployment](#kubernetes-deployment)
- [Environment Configuration](#environment-configuration)
- [Database Setup](#database-setup)
- [SMB Configuration](#smb-configuration)
- [SSL/TLS Setup](#ssltls-setup)
- [Monitoring & Logging](#monitoring--logging)
- [Backup & Recovery](#backup--recovery)
- [Troubleshooting](#troubleshooting)

## Quick Start

For a quick development setup:

> Uses rootless Podman Compose; never use sudo. Release artifacts are built only inside rootless build containers (constitution §11.4.173).

```bash
# Clone the repository
git clone <repository-url>
cd Catalogizer

# Start with Docker Compose
podman compose up -d

# Access the application
open http://localhost:3000
```

## Development Deployment

### Prerequisites

- Go 1.21+
- Node.js 18+
- SQLCipher
- Git

### Backend Setup

```bash
# Navigate to backend directory
cd catalog-api

# Install dependencies
go mod tidy

# Create environment file
cp .env.example .env

# Edit configuration
nano .env

# Run database migrations
go run cmd/migrate/main.go

# Start the API server
go run cmd/server/main.go
```

### Frontend Setup

```bash
# Navigate to frontend directory
cd catalog-web

# Install dependencies
npm install

# Create environment file
cp .env.example .env.local

# Start development server
npm run dev
```

### Development Environment Variables

#### Backend (.env)
```env
# Development Configuration
DB_PATH=./data/catalogizer.db
DB_ENCRYPTION_KEY=dev-key-change-in-production-32char
PORT=8080
HOST=127.0.0.1
GIN_MODE=debug

# JWT Configuration
JWT_SECRET=dev-jwt-secret-change-in-production
JWT_EXPIRY_HOURS=24

# SMB Configuration (update with your SMB sources)
SMB_SOURCES=smb://your-server/media
SMB_USERNAME=your-username
SMB_PASSWORD=your-password

# External API Keys (optional for development)
TMDB_API_KEY=your-tmdb-key
SPOTIFY_CLIENT_ID=your-spotify-id
SPOTIFY_CLIENT_SECRET=your-spotify-secret

# Logging
LOG_LEVEL=debug
LOG_FILE=./logs/catalogizer.log
```

#### Frontend (.env.local)
```env
VITE_API_BASE_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080/ws
VITE_ENABLE_ANALYTICS=true
VITE_ENABLE_REALTIME=true
```

## Production Deployment

### System Requirements

**Minimum Requirements:**
- CPU: 2 cores
- RAM: 4GB
- Storage: 20GB SSD
- Network: 100 Mbps

**Recommended for High Load:**
- CPU: 8 cores
- RAM: 16GB
- Storage: 100GB NVMe SSD
- Network: 1 Gbps

### Production Setup

#### 1. System Preparation

```bash
# Update system
sudo apt update && sudo apt upgrade -y

# Install required packages
sudo apt install -y nginx postgresql-client sqlite3 certbot

# Create application user
sudo useradd -m -s /bin/bash catalogizer
sudo usermod -aG docker catalogizer

# Create application directories
sudo mkdir -p /opt/catalogizer/{api,web,data,logs,backup}
sudo chown -R catalogizer:catalogizer /opt/catalogizer
```

#### 2. Database Setup

```bash
# Install SQLCipher
sudo apt install -y sqlcipher

# Create secure database directory
sudo mkdir -p /var/lib/catalogizer
sudo chown catalogizer:catalogizer /var/lib/catalogizer
sudo chmod 750 /var/lib/catalogizer

# Initialize database
sudo -u catalogizer sqlcipher /var/lib/catalogizer/catalogizer.db
```

#### 3. Backend Deployment

```bash
# Build the application
cd catalog-api
CGO_ENABLED=1 go build -o /opt/catalogizer/api/catalogizer cmd/server/main.go

# Create systemd service
sudo tee /etc/systemd/system/catalogizer-api.service > /dev/null <<EOF
[Unit]
Description=Catalogizer API Server
After=network.target

[Service]
Type=simple
User=catalogizer
Group=catalogizer
WorkingDirectory=/opt/catalogizer/api
ExecStart=/opt/catalogizer/api/catalogizer
Restart=always
RestartSec=5
Environment=PATH=/usr/local/bin:/usr/bin:/bin

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/catalogizer /var/lib/catalogizer

[Install]
WantedBy=multi-user.target
EOF

# Enable and start service
sudo systemctl enable catalogizer-api
sudo systemctl start catalogizer-api
```

#### 4. Frontend Deployment

```bash
# Build frontend
cd catalog-web
npm run build

# Copy built files to web directory
sudo cp -r dist/* /opt/catalogizer/web/

# Set permissions
sudo chown -R www-data:www-data /opt/catalogizer/web
```

#### 5. Nginx Configuration

```bash
# Create Nginx configuration
sudo tee /etc/nginx/sites-available/catalogizer > /dev/null <<EOF
server {
    listen 80;
    server_name your-domain.com;

    # Redirect HTTP to HTTPS
    return 301 https://\$server_name\$request_uri;
}

server {
    listen 443 ssl http2;
    server_name your-domain.com;

    # SSL Configuration
    ssl_certificate /etc/letsencrypt/live/your-domain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/your-domain.com/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES256-GCM-SHA512:DHE-RSA-AES256-GCM-SHA512;
    ssl_prefer_server_ciphers off;

    # Security headers
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    # Frontend
    location / {
        root /opt/catalogizer/web;
        try_files \$uri \$uri/ /index.html;

        # Cache static assets
        location ~* \.(js|css|png|jpg|jpeg|gif|ico|svg|woff|woff2|ttf|eot)$ {
            expires 1y;
            add_header Cache-Control "public, immutable";
        }
    }

    # API Proxy
    location /api/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # WebSocket endpoint
    location /ws {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    # Gzip compression
    gzip on;
    gzip_vary on;
    gzip_min_length 1024;
    gzip_proxied any;
    gzip_comp_level 6;
    gzip_types
        text/plain
        text/css
        text/xml
        text/javascript
        application/json
        application/javascript
        application/xml+rss
        application/atom+xml
        image/svg+xml;
}
EOF

# Enable site
sudo ln -s /etc/nginx/sites-available/catalogizer /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

### Production Environment Variables

#### Backend (.env)
```env
# Production Configuration
DB_PATH=/var/lib/catalogizer/catalogizer.db
DB_ENCRYPTION_KEY=your-32-character-encryption-key-here
PORT=8080
HOST=127.0.0.1
GIN_MODE=release

# JWT Configuration
JWT_SECRET=your-production-jwt-secret-key-here
JWT_EXPIRY_HOURS=24
REFRESH_TOKEN_EXPIRY_HOURS=168

# SMB Configuration
SMB_SOURCES=smb://server1/media,smb://server2/backup
SMB_USERNAME=media_user
SMB_PASSWORD=secure_password
SMB_DOMAIN=your-domain
SMB_RETRY_ATTEMPTS=5
SMB_RETRY_DELAY_SECONDS=30
SMB_CONNECTION_TIMEOUT=30
SMB_OFFLINE_CACHE_SIZE=10000

# External API Keys
TMDB_API_KEY=your-production-tmdb-key
SPOTIFY_CLIENT_ID=your-production-spotify-id
SPOTIFY_CLIENT_SECRET=your-production-spotify-secret
STEAM_API_KEY=your-steam-api-key

# Monitoring Configuration
WATCH_INTERVAL_SECONDS=30
MAX_CONCURRENT_ANALYSIS=10
ANALYSIS_TIMEOUT_MINUTES=15

# Logging
LOG_LEVEL=info
LOG_FILE=/opt/catalogizer/logs/catalogizer.log
```

## Docker Deployment

### Docker Compose for Production

```yaml
# docker-compose.prod.yml
version: '3.8'

services:
  catalogizer-api:
    build:
      context: ./catalog-api
      dockerfile: Dockerfile.prod
    restart: unless-stopped
    environment:
      - DB_PATH=/data/catalogizer.db
      - DB_ENCRYPTION_KEY=${DB_ENCRYPTION_KEY}
      - JWT_SECRET=${JWT_SECRET}
      - SMB_SOURCES=${SMB_SOURCES}
      - SMB_USERNAME=${SMB_USERNAME}
      - SMB_PASSWORD=${SMB_PASSWORD}
      - TMDB_API_KEY=${TMDB_API_KEY}
    volumes:
      - catalogizer_data:/data
      - catalogizer_logs:/logs
      - ${SMB_MOUNT_PATH}:/media:ro
    networks:
      - catalogizer_network
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  catalogizer-web:
    build:
      context: ./catalog-web
      dockerfile: Dockerfile.prod
    restart: unless-stopped
    depends_on:
      - catalogizer-api
    networks:
      - catalogizer_network

  nginx:
    image: nginx:alpine
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./config/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./ssl:/etc/ssl/certs:ro
      - catalogizer_logs:/var/log/nginx
    depends_on:
      - catalogizer-web
      - catalogizer-api
    networks:
      - catalogizer_network

  prometheus:
    image: prom/prometheus:latest
    restart: unless-stopped
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus_data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
    networks:
      - catalogizer_network

  grafana:
    image: grafana/grafana:latest
    restart: unless-stopped
    environment:
      - GF_SECURITY_ADMIN_PASSWORD=${GRAFANA_PASSWORD}
    volumes:
      - grafana_data:/var/lib/grafana
    networks:
      - catalogizer_network

volumes:
  catalogizer_data:
    driver: local
  catalogizer_logs:
    driver: local
  prometheus_data:
    driver: local
  grafana_data:
    driver: local

networks:
  catalogizer_network:
    driver: bridge
```

### Production Dockerfile (Backend)

```dockerfile
# catalog-api/Dockerfile.prod
FROM golang:1.21-alpine AS builder

# Install build dependencies
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o catalogizer cmd/server/main.go

# Final stage
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates sqlite tzdata curl

WORKDIR /root/

# Copy the binary
COPY --from=builder /app/catalogizer .

# Create directories
RUN mkdir -p /data /logs

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080/health || exit 1

# Run the binary
CMD ["./catalogizer"]
```

### Production Dockerfile (Frontend)

```dockerfile
# catalog-web/Dockerfile.prod
FROM node:18-alpine AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./
RUN npm ci --only=production

# Copy source code
COPY . .

# Build the application
RUN npm run build

# Final stage - nginx
FROM nginx:alpine

# Copy built assets
COPY --from=builder /app/dist /usr/share/nginx/html

# Copy nginx configuration
COPY nginx.conf /etc/nginx/nginx.conf

# Expose port
EXPOSE 80

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD curl -f http://localhost || exit 1

CMD ["nginx", "-g", "daemon off;"]
```

## Kubernetes Deployment

### Namespace and ConfigMap

```yaml
# k8s/namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: catalogizer

---
# k8s/configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: catalogizer-config
  namespace: catalogizer
data:
  DB_PATH: "/data/catalogizer.db"
  PORT: "8080"
  HOST: "0.0.0.0"
  GIN_MODE: "release"
  LOG_LEVEL: "info"
  WATCH_INTERVAL_SECONDS: "30"
  MAX_CONCURRENT_ANALYSIS: "10"
```

### Secrets

```yaml
# k8s/secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: catalogizer-secrets
  namespace: catalogizer
type: Opaque
data:
  DB_ENCRYPTION_KEY: <base64-encoded-key>
  JWT_SECRET: <base64-encoded-secret>
  SMB_USERNAME: <base64-encoded-username>
  SMB_PASSWORD: <base64-encoded-password>
  TMDB_API_KEY: <base64-encoded-api-key>
```

### Deployment

```yaml
# k8s/deployment.yaml
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
      - name: catalogizer-api
        image: catalogizer/api:latest
        ports:
        - containerPort: 8080
        env:
        - name: DB_ENCRYPTION_KEY
          valueFrom:
            secretKeyRef:
              name: catalogizer-secrets
              key: DB_ENCRYPTION_KEY
        - name: JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: catalogizer-secrets
              key: JWT_SECRET
        envFrom:
        - configMapRef:
            name: catalogizer-config
        volumeMounts:
        - name: data-volume
          mountPath: /data
        - name: logs-volume
          mountPath: /logs
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
      volumes:
      - name: data-volume
        persistentVolumeClaim:
          claimName: catalogizer-data-pvc
      - name: logs-volume
        persistentVolumeClaim:
          claimName: catalogizer-logs-pvc

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalogizer-web
  namespace: catalogizer
spec:
  replicas: 2
  selector:
    matchLabels:
      app: catalogizer-web
  template:
    metadata:
      labels:
        app: catalogizer-web
    spec:
      containers:
      - name: catalogizer-web
        image: catalogizer/web:latest
        ports:
        - containerPort: 80
        resources:
          requests:
            memory: "64Mi"
            cpu: "100m"
          limits:
            memory: "128Mi"
            cpu: "200m"
```

### Services

```yaml
# k8s/services.yaml
apiVersion: v1
kind: Service
metadata:
  name: catalogizer-api-service
  namespace: catalogizer
spec:
  selector:
    app: catalogizer-api
  ports:
  - protocol: TCP
    port: 8080
    targetPort: 8080
  type: ClusterIP

---
apiVersion: v1
kind: Service
metadata:
  name: catalogizer-web-service
  namespace: catalogizer
spec:
  selector:
    app: catalogizer-web
  ports:
  - protocol: TCP
    port: 80
    targetPort: 80
  type: ClusterIP
```

### Ingress

```yaml
# k8s/ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: catalogizer-ingress
  namespace: catalogizer
  annotations:
    nginx.ingress.kubernetes.io/rewrite-target: /
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/proxy-read-timeout: "3600"
    nginx.ingress.kubernetes.io/proxy-send-timeout: "3600"
spec:
  tls:
  - hosts:
    - catalogizer.yourdomain.com
    secretName: catalogizer-tls
  rules:
  - host: catalogizer.yourdomain.com
    http:
      paths:
      - path: /api
        pathType: Prefix
        backend:
          service:
            name: catalogizer-api-service
            port:
              number: 8080
      - path: /ws
        pathType: Prefix
        backend:
          service:
            name: catalogizer-api-service
            port:
              number: 8080
      - path: /
        pathType: Prefix
        backend:
          service:
            name: catalogizer-web-service
            port:
              number: 80
```

## SSL/TLS Setup

### Let's Encrypt with Certbot

```bash
# Install Certbot
sudo apt install certbot python3-certbot-nginx

# Obtain SSL certificate
sudo certbot --nginx -d your-domain.com

# Auto-renewal
sudo crontab -e
# Add: 0 12 * * * /usr/bin/certbot renew --quiet
```

### Manual SSL Certificate

```bash
# Generate private key
openssl genrsa -out catalogizer.key 2048

# Generate certificate signing request
openssl req -new -key catalogizer.key -out catalogizer.csr

# Generate self-signed certificate (for development)
openssl x509 -req -days 365 -in catalogizer.csr -signkey catalogizer.key -out catalogizer.crt
```

## Monitoring & Logging

### Prometheus Configuration

```yaml
# prometheus.yml
global:
  scrape_interval: 15s

scrape_configs:
  - job_name: 'catalogizer-api'
    static_configs:
      - targets: ['catalogizer-api:8080']
    metrics_path: /metrics
    scrape_interval: 10s

  - job_name: 'nginx'
    static_configs:
      - targets: ['nginx:9113']
```

### Grafana Dashboards

Key metrics to monitor:
- API response times
- Database connection pool usage
- SMB source availability
- Memory and CPU usage
- Error rates
- Active user sessions

### Log Management

```bash
# Logrotate configuration
sudo tee /etc/logrotate.d/catalogizer > /dev/null <<EOF
/opt/catalogizer/logs/*.log {
    daily
    missingok
    rotate 30
    compress
    delaycompress
    notifempty
    create 644 catalogizer catalogizer
    postrotate
        systemctl reload catalogizer-api
    endscript
}
EOF
```

## Backup & Recovery

### Database Backup

```bash
#!/bin/bash
# backup.sh

BACKUP_DIR="/opt/catalogizer/backup"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
DB_PATH="/var/lib/catalogizer/catalogizer.db"

# Create backup directory
mkdir -p $BACKUP_DIR

# Backup database
sqlite3 $DB_PATH ".backup $BACKUP_DIR/catalogizer_$TIMESTAMP.db"

# Compress backup
gzip "$BACKUP_DIR/catalogizer_$TIMESTAMP.db"

# Remove backups older than 30 days
find $BACKUP_DIR -name "*.gz" -mtime +30 -delete

echo "Backup completed: catalogizer_$TIMESTAMP.db.gz"
```

### Automated Backup

```bash
# Add to crontab
0 2 * * * /opt/catalogizer/scripts/backup.sh
```

### Recovery Procedure

```bash
# Stop the service
sudo systemctl stop catalogizer-api

# Restore from backup
gunzip -c /opt/catalogizer/backup/catalogizer_20240115_020000.db.gz > /var/lib/catalogizer/catalogizer.db

# Set permissions
sudo chown catalogizer:catalogizer /var/lib/catalogizer/catalogizer.db

# Start the service
sudo systemctl start catalogizer-api
```

## Troubleshooting

### Common Issues

#### 1. Database Connection Issues

```bash
# Check database permissions
ls -la /var/lib/catalogizer/

# Test database connectivity
sqlite3 /var/lib/catalogizer/catalogizer.db ".tables"

# Check logs for encryption key issues
journalctl -u catalogizer-api -f
```

#### 2. SMB Connection Problems

```bash
# Test SMB connectivity
smbclient -L //your-server -U your-username

# Check SMB service logs
tail -f /opt/catalogizer/logs/catalogizer.log | grep SMB
```

#### 3. High Memory Usage

```bash
# Monitor memory usage
top -p $(pgrep catalogizer)

# Check for memory leaks
valgrind --tool=memcheck ./catalogizer
```

#### 4. Performance Issues

```bash
# Check database performance
sqlite3 /var/lib/catalogizer/catalogizer.db "EXPLAIN QUERY PLAN SELECT * FROM media_items WHERE media_type = 'movie';"

# Monitor system resources
htop
iotop
```

### Debug Mode

```bash
# Enable debug logging
export LOG_LEVEL=debug
export GIN_MODE=debug

# Restart service with debug enabled
sudo systemctl restart catalogizer-api
```

### Health Check Endpoints

```bash
# Check API health
curl http://localhost:8080/health

# Check detailed status
curl http://localhost:8080/api/v1/status

# Check SMB source status
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/smb/sources/status
```

This deployment guide provides comprehensive instructions for setting up Catalogizer in various environments, from development to production-ready deployments with high availability and monitoring.