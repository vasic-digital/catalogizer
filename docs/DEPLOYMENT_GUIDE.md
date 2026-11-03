# Catalogizer v2.4.0 - Deployment and Operations Guide

## Table of Contents
1. [Overview](#overview)
2. [System Requirements](#system-requirements)
3. [Pre-deployment Checklist](#pre-deployment-checklist)
4. [Local Development Deployment](#local-development-deployment)
5. [Production Deployment](#production-deployment)
6. [Docker Deployment](#docker-deployment)
7. [Kubernetes Deployment](#kubernetes-deployment)
8. [Cloud Platform Deployments](#cloud-platform-deployments)
9. [Load Balancing and High Availability](#load-balancing-and-high-availability)
10. [Database Setup and Migration](#database-setup-and-migration)
11. [SSL/TLS Configuration](#ssltls-configuration)
12. [Monitoring and Observability](#monitoring-and-observability)
13. [Backup and Disaster Recovery](#backup-and-disaster-recovery)
14. [Security Hardening](#security-hardening)
15. [Performance Optimization](#performance-optimization)
16. [Operational Procedures](#operational-procedures)
17. [Troubleshooting](#troubleshooting)

## Overview

This guide provides comprehensive instructions for deploying and operating Catalogizer v2.4.0 in various environments, from local development to large-scale production deployments.

### Deployment Options

- **Standalone Server**: Single server deployment for small to medium workloads
- **Docker Containers**: Containerized deployment for consistency and scalability
- **Kubernetes**: Orchestrated deployment for high availability and auto-scaling
- **Cloud Native**: Leveraging cloud platform services (AWS, GCP, Azure)

## System Requirements

### Minimum Requirements

| Component | Requirement |
|-----------|-------------|
| Operating System | Linux (Ubuntu 20.04+, CentOS 8+), macOS 10.15+, Windows Server 2019+ |
| CPU | 2 cores, 2.0 GHz |
| Memory | 4 GB RAM |
| Storage | 20 GB available space |
| Network | 1 Gbps network interface |
| Go Version | 1.21 or later |

### Recommended Requirements

| Component | Requirement |
|-----------|-------------|
| Operating System | Linux (Ubuntu 22.04 LTS, RHEL 9) |
| CPU | 8 cores, 3.0 GHz |
| Memory | 16 GB RAM |
| Storage | 100 GB SSD |
| Network | 10 Gbps network interface |
| Go Version | 1.21.5 or later |

### Production Requirements

| Component | Requirement |
|-----------|-------------|
| Operating System | Linux (Ubuntu 22.04 LTS) |
| CPU | 16+ cores, 3.2 GHz |
| Memory | 32+ GB RAM |
| Storage | 500+ GB NVMe SSD |
| Network | 25+ Gbps network interface |
| Load Balancer | Nginx, HAProxy, or cloud LB |
| Database | PostgreSQL 14+, MySQL 8.0+ |

## Pre-deployment Checklist

### Infrastructure Preparation

- [ ] Provision required servers/instances
- [ ] Configure network security groups/firewalls
- [ ] Set up DNS records
- [ ] Obtain SSL/TLS certificates
- [ ] Configure load balancers
- [ ] Set up monitoring and logging infrastructure
- [ ] Prepare backup storage

### Software Dependencies

- [ ] Install Go 1.21+
- [ ] Install database server (PostgreSQL/MySQL) or configure managed database
- [ ] Install web server (Nginx) for reverse proxy
- [ ] Install monitoring tools (Prometheus, Grafana)
- [ ] Install log aggregation tools (ELK Stack, Loki)
- [ ] Configure firewall rules

### Security Preparation

- [ ] Generate secure JWT secrets
- [ ] Configure database credentials
- [ ] Set up SSL certificates
- [ ] Configure access control lists
- [ ] Prepare security scanning tools
- [ ] Set up intrusion detection

## Local Development Deployment

### Quick Start

```bash
# Clone the repository
git clone https://github.com/your-org/catalogizer.git
cd catalogizer/catalog-api

# Install dependencies
go mod download

# Run configuration wizard
go run main.go --wizard --config-type development

# Start the development server
go run main.go --dev
```

### Manual Development Setup

```bash
# Set environment variables
export CATALOGIZER_ENV=development
export CATALOGIZER_PORT=8080
export CATALOGIZER_DB_TYPE=sqlite
export CATALOGIZER_DB_CONNECTION="./dev.db"
export CATALOGIZER_LOG_LEVEL=debug
export CATALOGIZER_MEDIA_PATH="./media"

# Create required directories
mkdir -p ./logs ./media ./config

# Generate development configuration
cat > config/config.json << EOF
{
  "version": "2.4.0",
  "configuration": {
    "server": {
      "port": 8080,
      "host": "localhost",
      "cors_enabled": true,
      "cors_origins": ["http://localhost:3000"]
    },
    "database": {
      "type": "sqlite",
      "connection_string": "./dev.db",
      "auto_migrate": true
    },
    "storage": {
      "type": "local",
      "path": "./media"
    },
    "logging": {
      "level": "debug",
      "output": "console"
    }
  }
}
EOF

# Start the server
go run main.go
```

### Development with Docker

```bash
# Build development image
docker build -t catalogizer:dev .

# Run with docker-compose
cat > docker-compose.dev.yml << EOF
version: '3.8'
services:
  catalogizer:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./media:/app/media
      - ./logs:/app/logs
      - ./config:/app/config
    environment:
      - CATALOGIZER_ENV=development
      - CATALOGIZER_LOG_LEVEL=debug
    depends_on:
      - db

  db:
    image: postgres:14
    environment:
      POSTGRES_DB: catalogizer_dev
      POSTGRES_USER: dev_user
      POSTGRES_PASSWORD: dev_password
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

volumes:
  postgres_data:
EOF

docker-compose -f docker-compose.dev.yml up -d
```

## Production Deployment

### Server Preparation

```bash
# Update system packages
sudo apt update && sudo apt upgrade -y

# Install required packages
sudo apt install -y curl wget git nginx postgresql-client

# Create catalogizer user
sudo useradd -m -s /bin/bash catalogizer
sudo usermod -aG sudo catalogizer

# Create application directories
sudo mkdir -p /opt/catalogizer/{bin,config,logs,media,backups}
sudo chown -R catalogizer:catalogizer /opt/catalogizer
```

### Application Installation

```bash
# Switch to catalogizer user
sudo su - catalogizer

# Download latest release
curl -L https://github.com/your-org/catalogizer/releases/download/v2.4.0/catalogizer-linux-amd64.tar.gz | tar xz -C /opt/catalogizer/bin

# Make binary executable
chmod +x /opt/catalogizer/bin/catalogizer

# Create production configuration
cat > /opt/catalogizer/config/config.json << EOF
{
  "version": "2.4.0",
  "configuration": {
    "server": {
      "port": 8080,
      "host": "0.0.0.0",
      "ssl_enabled": false,
      "cors_enabled": true,
      "cors_origins": ["https://yourdomain.com"]
    },
    "database": {
      "type": "postgresql",
      "host": "localhost",
      "port": 5432,
      "database": "catalogizer",
      "username": "catalogizer_user",
      "password": "secure_password",
      "ssl_mode": "require",
      "max_connections": 20
    },
    "storage": {
      "type": "local",
      "path": "/opt/catalogizer/media",
      "max_file_size": "500MB"
    },
    "security": {
      "jwt_secret": "your-256-bit-secret-key-here",
      "jwt_expiration": "24h",
      "password_min_length": 8
    },
    "logging": {
      "level": "info",
      "format": "json",
      "output": "file",
      "file_path": "/opt/catalogizer/logs/catalogizer.log",
      "max_size": "100MB",
      "max_age": "30d"
    }
  }
}
EOF
```

### Database Setup

```bash
# Install PostgreSQL
sudo apt install -y postgresql postgresql-contrib

# Start and enable PostgreSQL
sudo systemctl start postgresql
sudo systemctl enable postgresql

# Create database and user
sudo -u postgres psql << EOF
CREATE DATABASE catalogizer;
CREATE USER catalogizer_user WITH ENCRYPTED PASSWORD 'secure_password';
GRANT ALL PRIVILEGES ON DATABASE catalogizer TO catalogizer_user;
ALTER USER catalogizer_user CREATEDB;
EOF

# Test database connection
PGPASSWORD=secure_password psql -h localhost -U catalogizer_user -d catalogizer -c "SELECT version();"
```

### Systemd Service Configuration

```bash
# Create systemd service file
sudo cat > /etc/systemd/system/catalogizer.service << EOF
[Unit]
Description=Catalogizer Media Management System
After=network.target postgresql.service
Wants=postgresql.service

[Service]
Type=simple
User=catalogizer
Group=catalogizer
WorkingDirectory=/opt/catalogizer
ExecStart=/opt/catalogizer/bin/catalogizer --config /opt/catalogizer/config/config.json
ExecReload=/bin/kill -HUP \$MAINPID
Restart=always
RestartSec=10
StandardOutput=journal
StandardError=journal
SyslogIdentifier=catalogizer

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/opt/catalogizer

# Resource limits
LimitNOFILE=65536
LimitNPROC=32768

[Install]
WantedBy=multi-user.target
EOF

# Reload systemd and start service
sudo systemctl daemon-reload
sudo systemctl enable catalogizer
sudo systemctl start catalogizer

# Check service status
sudo systemctl status catalogizer
```

### Nginx Reverse Proxy

```bash
# Create Nginx configuration
sudo cat > /etc/nginx/sites-available/catalogizer << EOF
upstream catalogizer_backend {
    server 127.0.0.1:8080;
    keepalive 32;
}

server {
    listen 80;
    server_name yourdomain.com www.yourdomain.com;
    return 301 https://\$server_name\$request_uri;
}

server {
    listen 443 ssl http2;
    server_name yourdomain.com www.yourdomain.com;

    # SSL Configuration
    ssl_certificate /etc/ssl/certs/yourdomain.com.pem;
    ssl_certificate_key /etc/ssl/private/yourdomain.com.key;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers ECDHE-RSA-AES128-GCM-SHA256:ECDHE-RSA-AES256-GCM-SHA384;
    ssl_prefer_server_ciphers off;

    # Security Headers
    add_header X-Frame-Options DENY;
    add_header X-Content-Type-Options nosniff;
    add_header X-XSS-Protection "1; mode=block";
    add_header Strict-Transport-Security "max-age=31536000; includeSubDomains" always;

    # Proxy Configuration
    location / {
        proxy_pass http://catalogizer_backend;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_cache_bypass \$http_upgrade;
        proxy_read_timeout 300s;
        proxy_connect_timeout 75s;
    }

    # Media files handling
    location /media/ {
        alias /opt/catalogizer/media/;
        expires 1y;
        add_header Cache-Control "public, immutable";
        access_log off;
    }

    # API rate limiting
    location /api/ {
        limit_req zone=api burst=10 nodelay;
        proxy_pass http://catalogizer_backend;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    # Health check endpoint
    location /health {
        proxy_pass http://catalogizer_backend;
        access_log off;
    }
}

# Rate limiting zone
http {
    limit_req_zone \$binary_remote_addr zone=api:10m rate=10r/s;
}
EOF

# Enable site and restart Nginx
sudo ln -s /etc/nginx/sites-available/catalogizer /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl restart nginx
```

## Docker Deployment

### Dockerfile

```dockerfile
# Multi-stage build
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o catalogizer main.go

# Production image
FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

# Copy binary
COPY --from=builder /app/catalogizer .

# Create directories
RUN mkdir -p /app/{config,logs,media,backups}

# Create non-root user
RUN adduser -D -g '' catalogizer
RUN chown -R catalogizer:catalogizer /app
USER catalogizer

EXPOSE 8080

CMD ["./catalogizer"]
```

### Converter tools in the runtime image

The `catalog-api` runtime image (`catalog-api/Dockerfile` and `catalog-api/Dockerfile.runtime`, Debian
trixie-slim base, non-root `appuser`) installs the external tools the conversion worker executes:

| Tool | Package | Used for |
|---|---|---|
| `ffmpeg` | `ffmpeg` | audio and video conversion |
| `convert` | `imagemagick` | image conversion, PDF rasterising fallback |
| `pdftotext` | `poppler-utils` | PDF text extraction (primary engine) |

`GET /health/deep` reports which of them the running server can find (`"converters":
{"ffmpeg":true,"convert":true,"pdftotext":true}`), from real `PATH` lookups on every request. If a tool is
missing, a conversion job that needs it ends `failed` within seconds with the reason
`converter tool "ffmpeg" is not installed on this server (not found on PATH)`, it is not left `pending`.
Bare-metal installs need the same three packages on the service user's `PATH`. Build the image with
`podman build --network host -f catalog-api/Dockerfile -t catalogizer-api .` from the repository root.

### Docker Compose Production

```yaml
version: '3.8'

services:
  catalogizer:
    image: catalogizer:2.4.0
    container_name: catalogizer_app
    restart: unless-stopped
    ports:
      - "8080:8080"
    volumes:
      - ./config:/app/config:ro
      - ./media:/app/media
      - ./logs:/app/logs
      - ./backups:/app/backups
    environment:
      - CATALOGIZER_ENV=production
      - CATALOGIZER_CONFIG_PATH=/app/config/config.json
    depends_on:
      - db
      - redis
    networks:
      - catalogizer_network
    healthcheck:
      test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

  db:
    image: postgres:14-alpine
    container_name: catalogizer_db
    restart: unless-stopped
    environment:
      POSTGRES_DB: catalogizer
      POSTGRES_USER: catalogizer_user
      POSTGRES_PASSWORD: ${DB_PASSWORD}
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./scripts/init.sql:/docker-entrypoint-initdb.d/init.sql:ro
    networks:
      - catalogizer_network
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U catalogizer_user -d catalogizer"]
      interval: 10s
      timeout: 5s
      retries: 5

  redis:
    image: redis:7-alpine
    container_name: catalogizer_redis
    restart: unless-stopped
    command: redis-server --appendonly yes
    volumes:
      - redis_data:/data
    networks:
      - catalogizer_network
    healthcheck:
      test: ["CMD", "redis-cli", "ping"]
      interval: 10s
      timeout: 3s
      retries: 5

  nginx:
    image: nginx:alpine
    container_name: catalogizer_nginx
    restart: unless-stopped
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./nginx/nginx.conf:/etc/nginx/nginx.conf:ro
      - ./nginx/ssl:/etc/nginx/ssl:ro
      - ./media:/var/www/media:ro
    depends_on:
      - catalogizer
    networks:
      - catalogizer_network

volumes:
  postgres_data:
  redis_data:

networks:
  catalogizer_network:
    driver: bridge
```

### Container Management Scripts

```bash
#!/bin/bash
# deploy.sh - Production deployment script

set -e

# Configuration
COMPOSE_FILE="docker-compose.prod.yml"
ENV_FILE=".env.prod"
IMAGE_TAG="catalogizer:2.4.0"

# Functions
log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $1"
}

check_requirements() {
    log "Checking requirements..."
    command -v docker >/dev/null 2>&1 || { echo "Docker is required but not installed." >&2; exit 1; }
    command -v docker-compose >/dev/null 2>&1 || { echo "Docker Compose is required but not installed." >&2; exit 1; }
}

backup_data() {
    log "Creating backup..."
    docker-compose -f $COMPOSE_FILE exec -T db pg_dump -U catalogizer_user catalogizer > backup_$(date +%Y%m%d_%H%M%S).sql
}

deploy() {
    log "Starting deployment..."

    # Pull latest images
    docker-compose -f $COMPOSE_FILE pull

    # Stop services gracefully
    docker-compose -f $COMPOSE_FILE down --timeout 30

    # Start services
    docker-compose -f $COMPOSE_FILE up -d

    # Wait for services to be healthy
    log "Waiting for services to be healthy..."
    sleep 30

    # Check health
    docker-compose -f $COMPOSE_FILE ps
}

rollback() {
    log "Rolling back deployment..."
    # Implementation depends on your rollback strategy
    git checkout HEAD~1
    docker-compose -f $COMPOSE_FILE down
    docker-compose -f $COMPOSE_FILE up -d
}

# Main execution
case "${1:-deploy}" in
    deploy)
        check_requirements
        backup_data
        deploy
        ;;
    rollback)
        rollback
        ;;
    *)
        echo "Usage: $0 {deploy|rollback}"
        exit 1
        ;;
esac
```

### catalog-web container: API upstream

The `catalog-web` image (nginx serving the SPA) proxies `/api` and `/ws` to the
catalog-api. The upstream is **configurable at run time** through the
`API_UPSTREAM` environment variable (`host:port`, no scheme). The nginx image
renders `catalog-web/nginx.conf.template` with `envsubst` on start; only `API_*`
variables are substituted.

| Situation | `API_UPSTREAM` |
|---|---|
| Default (unset) | `host.containers.internal:8080` (API on the container host, rootless podman) |
| Compose stack, API service named `catalog-api` | `catalog-api:8080` |
| Host networking (`network_mode: host`) | `127.0.0.1:8080` |

```bash
podman run -d -p 3000:3000 -e API_UPSTREAM=catalog-api:8080 --network catalogizer localhost/catalogizer-web
```

nginx resolves the upstream name when it starts, so the API container must be
resolvable (same network) at that moment.

## Kubernetes Deployment

### Namespace and ConfigMap

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: catalogizer
  labels:
    name: catalogizer

---
# configmap.yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: catalogizer-config
  namespace: catalogizer
data:
  config.json: |
    {
      "version": "2.4.0",
      "configuration": {
        "server": {
          "port": 8080,
          "host": "0.0.0.0"
        },
        "database": {
          "type": "postgresql",
          "host": "catalogizer-postgres",
          "port": 5432,
          "database": "catalogizer"
        },
        "storage": {
          "type": "s3",
          "bucket": "catalogizer-media"
        }
      }
    }
```

### Secrets

```yaml
# secrets.yaml
apiVersion: v1
kind: Secret
metadata:
  name: catalogizer-secrets
  namespace: catalogizer
type: Opaque
data:
  db-password: <base64-encoded-password>
  jwt-secret: <base64-encoded-jwt-secret>
  aws-access-key-id: <base64-encoded-access-key>
  aws-secret-access-key: <base64-encoded-secret-key>
```

### Deployment

```yaml
# deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalogizer
  namespace: catalogizer
  labels:
    app: catalogizer
spec:
  replicas: 3
  selector:
    matchLabels:
      app: catalogizer
  template:
    metadata:
      labels:
        app: catalogizer
    spec:
      containers:
      - name: catalogizer
        image: catalogizer:2.4.0
        ports:
        - containerPort: 8080
        env:
        - name: CATALOGIZER_CONFIG_PATH
          value: "/app/config/config.json"
        - name: CATALOGIZER_DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: catalogizer-secrets
              key: db-password
        - name: CATALOGIZER_JWT_SECRET
          valueFrom:
            secretKeyRef:
              name: catalogizer-secrets
              key: jwt-secret
        - name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: catalogizer-secrets
              key: aws-access-key-id
        - name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: catalogizer-secrets
              key: aws-secret-access-key
        volumeMounts:
        - name: config-volume
          mountPath: /app/config
        - name: logs-volume
          mountPath: /app/logs
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "500m"
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
      volumes:
      - name: config-volume
        configMap:
          name: catalogizer-config
      - name: logs-volume
        emptyDir: {}
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: app
                  operator: In
                  values:
                  - catalogizer
              topologyKey: kubernetes.io/hostname
```

### Service and Ingress

```yaml
# service.yaml
apiVersion: v1
kind: Service
metadata:
  name: catalogizer-service
  namespace: catalogizer
spec:
  selector:
    app: catalogizer
  ports:
    - protocol: TCP
      port: 80
      targetPort: 8080
  type: ClusterIP

---
# ingress.yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: catalogizer-ingress
  namespace: catalogizer
  annotations:
    kubernetes.io/ingress.class: nginx
    cert-manager.io/cluster-issuer: letsencrypt-prod
    nginx.ingress.kubernetes.io/proxy-body-size: 500m
    nginx.ingress.kubernetes.io/rate-limit: "100"
spec:
  tls:
  - hosts:
    - catalogizer.yourdomain.com
    secretName: catalogizer-tls
  rules:
  - host: catalogizer.yourdomain.com
    http:
      paths:
      - path: /
        pathType: Prefix
        backend:
          service:
            name: catalogizer-service
            port:
              number: 80
```

### PostgreSQL StatefulSet

```yaml
# postgres.yaml
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: catalogizer-postgres
  namespace: catalogizer
spec:
  serviceName: catalogizer-postgres
  replicas: 1
  selector:
    matchLabels:
      app: catalogizer-postgres
  template:
    metadata:
      labels:
        app: catalogizer-postgres
    spec:
      containers:
      - name: postgres
        image: postgres:14
        env:
        - name: POSTGRES_DB
          value: catalogizer
        - name: POSTGRES_USER
          value: catalogizer_user
        - name: POSTGRES_PASSWORD
          valueFrom:
            secretKeyRef:
              name: catalogizer-secrets
              key: db-password
        - name: PGDATA
          value: /var/lib/postgresql/data/pgdata
        ports:
        - containerPort: 5432
        volumeMounts:
        - name: postgres-storage
          mountPath: /var/lib/postgresql/data
        resources:
          requests:
            memory: "1Gi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "1000m"
  volumeClaimTemplates:
  - metadata:
      name: postgres-storage
    spec:
      accessModes: ["ReadWriteOnce"]
      resources:
        requests:
          storage: 20Gi
      storageClassName: fast-ssd

---
apiVersion: v1
kind: Service
metadata:
  name: catalogizer-postgres
  namespace: catalogizer
spec:
  selector:
    app: catalogizer-postgres
  ports:
    - port: 5432
  clusterIP: None
```

### Deployment Script

```bash
#!/bin/bash
# k8s-deploy.sh

set -e

NAMESPACE="catalogizer"
KUSTOMIZE_DIR="k8s"

# Create namespace
kubectl create namespace $NAMESPACE --dry-run=client -o yaml | kubectl apply -f -

# Apply configurations
kubectl apply -f $KUSTOMIZE_DIR/namespace.yaml
kubectl apply -f $KUSTOMIZE_DIR/configmap.yaml
kubectl apply -f $KUSTOMIZE_DIR/secrets.yaml
kubectl apply -f $KUSTOMIZE_DIR/postgres.yaml
kubectl apply -f $KUSTOMIZE_DIR/deployment.yaml
kubectl apply -f $KUSTOMIZE_DIR/service.yaml
kubectl apply -f $KUSTOMIZE_DIR/ingress.yaml

# Wait for deployment
kubectl rollout status deployment/catalogizer -n $NAMESPACE --timeout=300s

# Check pods
kubectl get pods -n $NAMESPACE

echo "Deployment completed successfully!"
```

## Cloud Platform Deployments

### AWS Deployment

#### Using AWS ECS

```json
{
  "family": "catalogizer",
  "networkMode": "awsvpc",
  "requiresCompatibilities": ["FARGATE"],
  "cpu": "1024",
  "memory": "2048",
  "executionRoleArn": "arn:aws:iam::account:role/ecsTaskExecutionRole",
  "taskRoleArn": "arn:aws:iam::account:role/catalogizerTaskRole",
  "containerDefinitions": [
    {
      "name": "catalogizer",
      "image": "your-account.dkr.ecr.region.amazonaws.com/catalogizer:2.4.0",
      "portMappings": [
        {
          "containerPort": 8080,
          "protocol": "tcp"
        }
      ],
      "environment": [
        {
          "name": "CATALOGIZER_ENV",
          "value": "production"
        }
      ],
      "secrets": [
        {
          "name": "CATALOGIZER_DB_PASSWORD",
          "valueFrom": "arn:aws:secretsmanager:region:account:secret:catalogizer/db-password"
        }
      ],
      "logConfiguration": {
        "logDriver": "awslogs",
        "options": {
          "awslogs-group": "/ecs/catalogizer",
          "awslogs-region": "us-east-1",
          "awslogs-stream-prefix": "ecs"
        }
      },
      "healthCheck": {
        "command": ["CMD-SHELL", "curl -f http://localhost:8080/health || exit 1"],
        "interval": 30,
        "timeout": 5,
        "retries": 3
      }
    }
  ]
}
```

#### AWS CloudFormation Template

```yaml
AWSTemplateFormatVersion: '2010-09-09'
Description: 'Catalogizer v2.4.0 Infrastructure'

Parameters:
  VpcId:
    Type: AWS::EC2::VPC::Id
    Description: VPC ID for the deployment

  PrivateSubnetIds:
    Type: List<AWS::EC2::Subnet::Id>
    Description: Private subnet IDs

  PublicSubnetIds:
    Type: List<AWS::EC2::Subnet::Id>
    Description: Public subnet IDs

Resources:
  # RDS Database
  CatalogizerDB:
    Type: AWS::RDS::DBInstance
    Properties:
      DBInstanceIdentifier: catalogizer-db
      DBInstanceClass: db.t3.medium
      Engine: postgres
      EngineVersion: '14.9'
      MasterUsername: catalogizer_user
      MasterUserPassword: !Ref DBPassword
      AllocatedStorage: 100
      StorageType: gp2
      VPCSecurityGroups:
        - !Ref DBSecurityGroup
      DBSubnetGroupName: !Ref DBSubnetGroup
      BackupRetentionPeriod: 7
      DeletePolicy: Snapshot

  # ECS Cluster
  CatalogizerCluster:
    Type: AWS::ECS::Cluster
    Properties:
      ClusterName: catalogizer-cluster
      CapacityProviders:
        - FARGATE
        - FARGATE_SPOT

  # Application Load Balancer
  ApplicationLoadBalancer:
    Type: AWS::ElasticLoadBalancingV2::LoadBalancer
    Properties:
      Name: catalogizer-alb
      Scheme: internet-facing
      Type: application
      Subnets: !Ref PublicSubnetIds
      SecurityGroups:
        - !Ref ALBSecurityGroup

  # S3 Bucket for media storage
  MediaBucket:
    Type: AWS::S3::Bucket
    Properties:
      BucketName: !Sub 'catalogizer-media-${AWS::AccountId}'
      VersioningConfiguration:
        Status: Enabled
      PublicAccessBlockConfiguration:
        BlockPublicAcls: true
        BlockPublicPolicy: true
        IgnorePublicAcls: true
        RestrictPublicBuckets: true
      BucketEncryption:
        ServerSideEncryptionConfiguration:
          - ServerSideEncryptionByDefault:
              SSEAlgorithm: AES256
```

### Google Cloud Platform Deployment

#### Using Cloud Run

```yaml
apiVersion: serving.knative.dev/v1
kind: Service
metadata:
  name: catalogizer
  namespace: default
  annotations:
    run.googleapis.com/ingress: all
    run.googleapis.com/execution-environment: gen2
spec:
  template:
    metadata:
      annotations:
        autoscaling.knative.dev/maxScale: '10'
        autoscaling.knative.dev/minScale: '1'
        run.googleapis.com/cpu-throttling: 'false'
    spec:
      containerConcurrency: 80
      timeoutSeconds: 300
      containers:
      - image: gcr.io/your-project/catalogizer:2.4.0
        ports:
        - containerPort: 8080
        env:
        - name: CATALOGIZER_ENV
          value: production
        - name: CATALOGIZER_DB_HOST
          value: /cloudsql/your-project:region:catalogizer-db
        resources:
          limits:
            cpu: '2'
            memory: 4Gi
          requests:
            cpu: '1'
            memory: 2Gi
```

### Azure Deployment

#### Using Azure Container Instances

```json
{
  "apiVersion": "2021-09-01",
  "type": "Microsoft.ContainerInstance/containerGroups",
  "name": "catalogizer",
  "location": "East US",
  "properties": {
    "containers": [
      {
        "name": "catalogizer",
        "properties": {
          "image": "catalogizer:2.4.0",
          "ports": [
            {
              "port": 8080,
              "protocol": "TCP"
            }
          ],
          "environmentVariables": [
            {
              "name": "CATALOGIZER_ENV",
              "value": "production"
            }
          ],
          "resources": {
            "requests": {
              "cpu": 2,
              "memoryInGB": 4
            }
          }
        }
      }
    ],
    "osType": "Linux",
    "ipAddress": {
      "type": "Public",
      "ports": [
        {
          "port": 8080,
          "protocol": "TCP"
        }
      ]
    },
    "restartPolicy": "Always"
  }
}
```

## Load Balancing and High Availability

### HAProxy Configuration

```
global
    daemon
    maxconn 4096
    log stdout local0

defaults
    mode http
    timeout connect 5000ms
    timeout client 50000ms
    timeout server 50000ms
    option httplog
    option redispatch
    retries 3

frontend catalogizer_frontend
    bind *:80
    bind *:443 ssl crt /etc/ssl/certs/catalogizer.pem
    redirect scheme https if !{ ssl_fc }

    # Rate limiting
    stick-table type ip size 100k expire 30s store http_req_rate(10s)
    http-request track-sc0 src
    http-request reject if { sc_http_req_rate(0) gt 10 }

    default_backend catalogizer_backend

backend catalogizer_backend
    balance roundrobin
    option httpchk GET /health

    server app1 10.0.1.10:8080 check
    server app2 10.0.1.11:8080 check
    server app3 10.0.1.12:8080 check

listen stats
    bind *:8404
    stats enable
    stats uri /stats
    stats refresh 30s
```

### Nginx Load Balancer

```nginx
upstream catalogizer_backend {
    least_conn;
    server 10.0.1.10:8080 max_fails=3 fail_timeout=30s;
    server 10.0.1.11:8080 max_fails=3 fail_timeout=30s;
    server 10.0.1.12:8080 max_fails=3 fail_timeout=30s;
    keepalive 32;
}

server {
    listen 443 ssl http2;
    server_name catalogizer.example.com;

    # SSL configuration
    ssl_certificate /etc/ssl/certs/catalogizer.crt;
    ssl_certificate_key /etc/ssl/private/catalogizer.key;

    # Health check endpoint
    location /health {
        proxy_pass http://catalogizer_backend;
        proxy_set_header Host $host;
        access_log off;
    }

    # Main application
    location / {
        proxy_pass http://catalogizer_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # Connection pooling
        proxy_http_version 1.1;
        proxy_set_header Connection "";

        # Timeouts
        proxy_connect_timeout 5s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
}
```

## Database Setup and Migration

### PostgreSQL Setup

```sql
-- Create database and user
CREATE DATABASE catalogizer;
CREATE USER catalogizer_user WITH ENCRYPTED PASSWORD 'secure_password';
GRANT ALL PRIVILEGES ON DATABASE catalogizer TO catalogizer_user;

-- Connect to catalogizer database
\c catalogizer;

-- Create extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
CREATE EXTENSION IF NOT EXISTS "btree_gin";

-- Grant schema permissions
GRANT ALL ON SCHEMA public TO catalogizer_user;
```

### Database Migration Script

```bash
#!/bin/bash
# migrate.sh

set -e

DB_HOST=${DB_HOST:-localhost}
DB_PORT=${DB_PORT:-5432}
DB_NAME=${DB_NAME:-catalogizer}
DB_USER=${DB_USER:-catalogizer_user}
DB_PASSWORD=${DB_PASSWORD}

if [ -z "$DB_PASSWORD" ]; then
    echo "DB_PASSWORD environment variable is required"
    exit 1
fi

# Test database connection
echo "Testing database connection..."
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c "SELECT version();" || {
    echo "Database connection failed"
    exit 1
}

# Run migrations
echo "Running database migrations..."
/opt/catalogizer/bin/catalogizer --migrate --config /opt/catalogizer/config/config.json

echo "Database migration completed successfully"
```

### Backup Script

```bash
#!/bin/bash
# backup.sh

set -e

BACKUP_DIR="/opt/catalogizer/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="catalogizer_backup_$TIMESTAMP.sql"

# Create backup directory
mkdir -p $BACKUP_DIR

# Create database backup
PGPASSWORD=$DB_PASSWORD pg_dump \
    -h $DB_HOST \
    -p $DB_PORT \
    -U $DB_USER \
    -d $DB_NAME \
    --no-owner \
    --no-privileges \
    --clean \
    --if-exists \
    > "$BACKUP_DIR/$BACKUP_FILE"

# Compress backup
gzip "$BACKUP_DIR/$BACKUP_FILE"

# Remove old backups (keep last 30 days)
find $BACKUP_DIR -name "catalogizer_backup_*.sql.gz" -mtime +30 -delete

echo "Backup completed: $BACKUP_DIR/$BACKUP_FILE.gz"
```

## SSL/TLS Configuration

### Let's Encrypt with Certbot

```bash
# Install Certbot
sudo apt install certbot python3-certbot-nginx

# Obtain certificate
sudo certbot --nginx -d catalogizer.example.com

# Set up automatic renewal
sudo crontab -e
# Add line: 0 12 * * * /usr/bin/certbot renew --quiet
```

### Manual SSL Certificate Configuration

```bash
# Generate private key
openssl genrsa -out catalogizer.key 2048

# Generate certificate signing request
openssl req -new -key catalogizer.key -out catalogizer.csr

# Generate self-signed certificate (for testing)
openssl req -x509 -newkey rsa:2048 -keyout catalogizer.key -out catalogizer.crt -days 365 -nodes

# Set proper permissions
chmod 600 catalogizer.key
chmod 644 catalogizer.crt
```

## Monitoring and Observability

### Prometheus Configuration

```yaml
# prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

rule_files:
  - "catalogizer_rules.yml"

scrape_configs:
  - job_name: 'catalogizer'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 15s

  - job_name: 'node-exporter'
    static_configs:
      - targets: ['localhost:9100']

alerting:
  alertmanagers:
    - static_configs:
        - targets:
          - alertmanager:9093
```

### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "Catalogizer v2.4.0 Dashboard",
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(http_requests_total[5m])",
            "legendFormat": "{{method}} {{status}}"
          }
        ]
      },
      {
        "title": "Response Time",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))",
            "legendFormat": "95th percentile"
          }
        ]
      },
      {
        "title": "Error Rate",
        "type": "singlestat",
        "targets": [
          {
            "expr": "rate(http_requests_total{status=~\"5..\"}[5m]) / rate(http_requests_total[5m])",
            "legendFormat": "Error Rate"
          }
        ]
      }
    ]
  }
}
```

### Health Check Script

```bash
#!/bin/bash
# health_check.sh

SERVICE_URL="http://localhost:8080/health"
TIMEOUT=10
MAX_RETRIES=3

check_health() {
    local url=$1
    local timeout=$2

    response=$(curl -s -o /dev/null -w "%{http_code}" --max-time $timeout $url)

    if [ "$response" = "200" ]; then
        return 0
    else
        return 1
    fi
}

# Perform health check with retries
for i in $(seq 1 $MAX_RETRIES); do
    if check_health $SERVICE_URL $TIMEOUT; then
        echo "Health check passed"
        exit 0
    else
        echo "Health check failed (attempt $i/$MAX_RETRIES)"
        if [ $i -lt $MAX_RETRIES ]; then
            sleep 5
        fi
    fi
done

echo "Health check failed after $MAX_RETRIES attempts"
exit 1
```

## Backup and Disaster Recovery

### Automated Backup Script

```bash
#!/bin/bash
# backup_system.sh

set -e

BACKUP_ROOT="/opt/catalogizer/backups"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
RETENTION_DAYS=30

# Create timestamped backup directory
BACKUP_DIR="$BACKUP_ROOT/$TIMESTAMP"
mkdir -p $BACKUP_DIR

echo "Starting backup at $(date)"

# Database backup
echo "Backing up database..."
PGPASSWORD=$DB_PASSWORD pg_dump \
    -h $DB_HOST \
    -p $DB_PORT \
    -U $DB_USER \
    -d $DB_NAME \
    --clean \
    --if-exists \
    > "$BACKUP_DIR/database.sql"

# Configuration backup
echo "Backing up configuration..."
cp -r /opt/catalogizer/config "$BACKUP_DIR/"

# Media files backup (if local storage)
if [ "$STORAGE_TYPE" = "local" ]; then
    echo "Backing up media files..."
    tar -czf "$BACKUP_DIR/media.tar.gz" -C /opt/catalogizer media/
fi

# Logs backup
echo "Backing up logs..."
tar -czf "$BACKUP_DIR/logs.tar.gz" -C /opt/catalogizer logs/

# Create backup manifest
cat > "$BACKUP_DIR/manifest.txt" << EOF
Backup created: $(date)
Database: included
Configuration: included
Media files: $([ "$STORAGE_TYPE" = "local" ] && echo "included" || echo "skipped (external storage)")
Logs: included
EOF

# Compress entire backup
echo "Compressing backup..."
tar -czf "$BACKUP_ROOT/catalogizer_backup_$TIMESTAMP.tar.gz" -C $BACKUP_ROOT $TIMESTAMP

# Remove temporary directory
rm -rf $BACKUP_DIR

# Upload to remote storage (if configured)
if [ -n "$BACKUP_S3_BUCKET" ]; then
    echo "Uploading to S3..."
    aws s3 cp "$BACKUP_ROOT/catalogizer_backup_$TIMESTAMP.tar.gz" \
        "s3://$BACKUP_S3_BUCKET/backups/"
fi

# Clean up old local backups
echo "Cleaning up old backups..."
find $BACKUP_ROOT -name "catalogizer_backup_*.tar.gz" -mtime +$RETENTION_DAYS -delete

echo "Backup completed: catalogizer_backup_$TIMESTAMP.tar.gz"
```

### Disaster Recovery Procedure

```bash
#!/bin/bash
# disaster_recovery.sh

set -e

BACKUP_FILE=$1
RECOVERY_DIR="/opt/catalogizer/recovery"

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup_file>"
    exit 1
fi

echo "Starting disaster recovery from $BACKUP_FILE"

# Create recovery directory
mkdir -p $RECOVERY_DIR
cd $RECOVERY_DIR

# Extract backup
echo "Extracting backup..."
tar -xzf $BACKUP_FILE

# Find backup directory
BACKUP_DIR=$(find . -maxdepth 1 -type d -name "20*" | head -1)

if [ -z "$BACKUP_DIR" ]; then
    echo "No backup directory found"
    exit 1
fi

# Stop services
echo "Stopping services..."
sudo systemctl stop catalogizer
sudo systemctl stop nginx

# Restore database
echo "Restoring database..."
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME < "$BACKUP_DIR/database.sql"

# Restore configuration
echo "Restoring configuration..."
sudo cp -r "$BACKUP_DIR/config/"* /opt/catalogizer/config/

# Restore media files (if included)
if [ -f "$BACKUP_DIR/media.tar.gz" ]; then
    echo "Restoring media files..."
    sudo tar -xzf "$BACKUP_DIR/media.tar.gz" -C /opt/catalogizer/
fi

# Restore logs
echo "Restoring logs..."
sudo tar -xzf "$BACKUP_DIR/logs.tar.gz" -C /opt/catalogizer/

# Fix permissions
sudo chown -R catalogizer:catalogizer /opt/catalogizer

# Start services
echo "Starting services..."
sudo systemctl start catalogizer
sudo systemctl start nginx

# Verify recovery
echo "Verifying recovery..."
sleep 10
curl -f http://localhost:8080/health || {
    echo "Health check failed after recovery"
    exit 1
}

echo "Disaster recovery completed successfully"
```

## Security Hardening

### Firewall Configuration

```bash
#!/bin/bash
# firewall_setup.sh

# Reset firewall rules
ufw --force reset

# Default policies
ufw default deny incoming
ufw default allow outgoing

# SSH access (adjust port as needed)
ufw allow 22/tcp

# HTTP/HTTPS
ufw allow 80/tcp
ufw allow 443/tcp

# Database (only from application servers)
ufw allow from 10.0.1.0/24 to any port 5432

# Monitoring
ufw allow from 10.0.1.0/24 to any port 9090  # Prometheus
ufw allow from 10.0.1.0/24 to any port 3000  # Grafana

# Enable firewall
ufw --force enable

# Show status
ufw status verbose
```

### Fail2ban Configuration

```ini
# /etc/fail2ban/jail.d/catalogizer.conf
[catalogizer]
enabled = true
port = 80,443
filter = catalogizer
logpath = /opt/catalogizer/logs/catalogizer.log
maxretry = 5
bantime = 3600
findtime = 600

[catalogizer-auth]
enabled = true
port = 80,443
filter = catalogizer-auth
logpath = /opt/catalogizer/logs/catalogizer.log
maxretry = 3
bantime = 7200
findtime = 600
```

```
# /etc/fail2ban/filter.d/catalogizer.conf
[Definition]
failregex = ^.*\[ERROR\].*Authentication failed.*remote_addr":"<HOST>".*$
            ^.*\[ERROR\].*Invalid credentials.*remote_addr":"<HOST>".*$

ignoreregex =
```

### Security Scan Script

```bash
#!/bin/bash
# security_scan.sh

echo "Starting security scan..."

# Check for common vulnerabilities
echo "Checking for outdated packages..."
apt list --upgradable 2>/dev/null | grep -v "Listing..." || echo "All packages up to date"

# Check file permissions
echo "Checking file permissions..."
find /opt/catalogizer -type f -perm /o+w -exec ls -la {} \; | head -10

# Check for exposed services
echo "Checking exposed services..."
netstat -tulpn | grep LISTEN

# Check for weak passwords (if local users exist)
echo "Checking password policy compliance..."
chage -l catalogizer 2>/dev/null || echo "No local catalogizer user found"

# Check SSL certificate expiry
echo "Checking SSL certificate expiry..."
if [ -f /etc/ssl/certs/catalogizer.crt ]; then
    openssl x509 -in /etc/ssl/certs/catalogizer.crt -noout -dates
fi

# Check for suspicious processes
echo "Checking running processes..."
ps aux | grep -E "(catalogizer|postgres|nginx)" | grep -v grep

echo "Security scan completed"
```

## Performance Optimization

### Database Optimization

```sql
-- PostgreSQL performance tuning
-- Add to postgresql.conf

# Memory settings
shared_buffers = 256MB
effective_cache_size = 1GB
work_mem = 4MB
maintenance_work_mem = 64MB

# Connection settings
max_connections = 100
shared_preload_libraries = 'pg_stat_statements'

# Checkpoint settings
checkpoint_completion_target = 0.9
wal_buffers = 16MB
checkpoint_segments = 32

# Query tuning
default_statistics_target = 100
random_page_cost = 1.1
effective_io_concurrency = 200

# Create indexes for better performance
CREATE INDEX CONCURRENTLY idx_users_email ON users(email);
CREATE INDEX CONCURRENTLY idx_media_user_id ON media_items(user_id);
CREATE INDEX CONCURRENTLY idx_analytics_timestamp ON analytics_events(timestamp);
CREATE INDEX CONCURRENTLY idx_logs_timestamp ON log_entries(timestamp);
```

### Application Performance Tuning

```bash
# /etc/security/limits.conf
catalogizer soft nofile 65536
catalogizer hard nofile 65536
catalogizer soft nproc 32768
catalogizer hard nproc 32768

# Kernel parameters for high performance
# /etc/sysctl.d/99-catalogizer.conf
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.ipv4.tcp_rmem = 4096 12582912 16777216
net.ipv4.tcp_wmem = 4096 12582912 16777216
net.core.netdev_max_backlog = 5000
net.ipv4.tcp_congestion_control = bbr
vm.swappiness = 10
vm.dirty_ratio = 15
vm.dirty_background_ratio = 5
```

### Caching Configuration

```json
{
  "performance": {
    "cache": {
      "enabled": true,
      "type": "redis",
      "redis": {
        "host": "localhost",
        "port": 6379,
        "db": 0,
        "password": "",
        "max_idle": 10,
        "max_active": 100,
        "idle_timeout": "240s"
      },
      "default_ttl": "1h",
      "cache_sizes": {
        "user_sessions": "10MB",
        "media_metadata": "50MB",
        "thumbnails": "100MB",
        "search_results": "25MB"
      }
    }
  }
}
```

## Operational Procedures

### Rolling Update Procedure

```bash
#!/bin/bash
# rolling_update.sh

set -e

NEW_VERSION=$1
SERVERS=("server1.example.com" "server2.example.com" "server3.example.com")
HEALTH_CHECK_URL="/health"
TIMEOUT=300

if [ -z "$NEW_VERSION" ]; then
    echo "Usage: $0 <new_version>"
    exit 1
fi

update_server() {
    local server=$1
    echo "Updating $server to version $NEW_VERSION..."

    # Remove server from load balancer
    echo "Removing $server from load balancer..."
    # Implementation depends on your load balancer

    # Wait for connections to drain
    sleep 30

    # Update application
    ssh $server "
        sudo systemctl stop catalogizer
        sudo wget -O /opt/catalogizer/bin/catalogizer https://releases.example.com/catalogizer/$NEW_VERSION/catalogizer
        sudo chmod +x /opt/catalogizer/bin/catalogizer
        sudo systemctl start catalogizer
    "

    # Wait for service to start
    sleep 10

    # Health check
    for i in {1..10}; do
        if curl -f "http://$server:8080$HEALTH_CHECK_URL"; then
            echo "$server is healthy"
            break
        fi
        if [ $i -eq 10 ]; then
            echo "$server failed health check"
            exit 1
        fi
        sleep 5
    done

    # Add server back to load balancer
    echo "Adding $server back to load balancer..."
    # Implementation depends on your load balancer

    echo "$server updated successfully"
}

# Update servers one by one
for server in "${SERVERS[@]}"; do
    update_server $server
    sleep 60  # Wait between servers
done

echo "Rolling update completed successfully"
```

### Scaling Procedure

```bash
#!/bin/bash
# scale_out.sh

DESIRED_INSTANCES=$1
CURRENT_INSTANCES=$(kubectl get pods -l app=catalogizer --no-headers | wc -l)

if [ -z "$DESIRED_INSTANCES" ]; then
    echo "Usage: $0 <desired_instances>"
    exit 1
fi

echo "Current instances: $CURRENT_INSTANCES"
echo "Desired instances: $DESIRED_INSTANCES"

if [ $DESIRED_INSTANCES -gt $CURRENT_INSTANCES ]; then
    echo "Scaling out..."
    kubectl scale deployment catalogizer --replicas=$DESIRED_INSTANCES
elif [ $DESIRED_INSTANCES -lt $CURRENT_INSTANCES ]; then
    echo "Scaling in..."
    kubectl scale deployment catalogizer --replicas=$DESIRED_INSTANCES
else
    echo "No scaling needed"
    exit 0
fi

# Wait for scaling to complete
echo "Waiting for scaling to complete..."
kubectl rollout status deployment/catalogizer --timeout=300s

echo "Scaling completed successfully"
```

## Troubleshooting

### Common Issues and Solutions

#### Service Won't Start

```bash
# Check service status
sudo systemctl status catalogizer

# Check logs
sudo journalctl -u catalogizer -f

# Check configuration
/opt/catalogizer/bin/catalogizer --config /opt/catalogizer/config/config.json --validate

# Check file permissions
ls -la /opt/catalogizer/
sudo chown -R catalogizer:catalogizer /opt/catalogizer
```

#### Database Connection Issues

```bash
# Test database connectivity
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c "SELECT 1;"

# Check database logs
sudo tail -f /var/log/postgresql/postgresql-14-main.log

# Check connection limits
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c "
SELECT count(*) as active_connections,
       setting as max_connections
FROM pg_stat_activity, pg_settings
WHERE name='max_connections';"
```

#### High Memory Usage

```bash
# Check memory usage
free -h
ps aux --sort=-%mem | head -10

# Check Go memory stats
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof

# Adjust memory limits in systemd
sudo systemctl edit catalogizer
# Add:
# [Service]
# MemoryLimit=2G
```

#### Performance Issues

```bash
# Check CPU usage
top -p $(pgrep catalogizer)

# Check I/O usage
iotop -o

# Check network connections
netstat -an | grep :8080

# Profile application
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof
```

### Log Analysis

```bash
# Search for errors
grep -i error /opt/catalogizer/logs/catalogizer.log | tail -20

# Search for specific patterns
grep "authentication failed" /opt/catalogizer/logs/catalogizer.log

# Analyze response times
awk '/response_time/ {print $NF}' /opt/catalogizer/logs/catalogizer.log | sort -n | tail -10

# Check error rates
grep -c "ERROR" /opt/catalogizer/logs/catalogizer.log
```

### Debugging Tools

```bash
# Debug API endpoints
curl -v http://localhost:8080/api/health

# Check runtime statistics
curl http://localhost:8080/debug/vars | jq

# Generate debug bundle
tar -czf debug_bundle_$(date +%Y%m%d_%H%M%S).tar.gz \
    /opt/catalogizer/logs/ \
    /opt/catalogizer/config/ \
    /var/log/nginx/ \
    /var/log/postgresql/
```

For additional troubleshooting guidance, see the [Troubleshooting Guide](TROUBLESHOOTING_GUIDE.md).

## Related Documentation

- [Architecture Overview](architecture/ARCHITECTURE.md) - System design and component interactions
- [Monitoring Guide](deployment/MONITORING_GUIDE.md) - Metrics, alerts, and observability setup
- [Production Runbook](deployment/PRODUCTION_RUNBOOK.md) - Operational procedures and incident response
- [Backup and Recovery](deployment/BACKUP_AND_RECOVERY.md) - Data protection and disaster recovery
- [Scaling Guide](deployment/SCALING_GUIDE.md) - Horizontal and vertical scaling strategies
- [Environment Variables](deployment/ENVIRONMENT_VARIABLES.md) - Complete environment configuration reference
- [Docker Setup](deployment/DOCKER_SETUP.md) - Container-based deployment details
- [Configuration Guide](CONFIGURATION_GUIDE.md) - Application configuration options
- [SQL Migrations](architecture/SQL_MIGRATIONS.md) - Database migration procedures