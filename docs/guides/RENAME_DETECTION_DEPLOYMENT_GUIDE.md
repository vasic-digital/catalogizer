# Catalogizer Deployment Guide - Rename Detection System

## Quick Start

The rename detection system is now fully integrated into Catalogizer. Here's how to deploy and configure it:

### 1. System Requirements

```bash
# Minimum Requirements
- Go 1.21+
- SQLite 3.35+ (with encryption support)
- 4GB RAM (recommended 8GB for large catalogs)
- 10GB disk space (database and logs)

# Network Requirements (for remote protocols)
- SMB: Port 445, 139
- FTP: Port 21 (+ data ports)
- NFS: Port 2049, 111
- WebDAV: Port 80/443
```

### 2. Installation

```bash
# Clone and build
git clone https://github.com/your-org/catalogizer.git
cd catalogizer/catalog-api

# Install dependencies
go mod download

# Build the application
./scripts/build.sh

# Or build manually
go build -o bin/catalog-api .
```

### 3. Configuration

Create `config.json`:

```json
{
  "database": {
    "path": "catalog.db",
    "encryption_key": "your-secure-key-here"
  },
  "rename_detection": {
    "enabled": true,
    "workers": 4,
    "queue_size": 10000,
    "cleanup_interval": "30s",
    "protocols": {
      "local": {
        "move_window": "2s",
        "batch_size": 1000
      },
      "smb": {
        "move_window": "10s",
        "batch_size": 500
      },
      "ftp": {
        "move_window": "30s",
        "batch_size": 100
      },
      "nfs": {
        "move_window": "5s",
        "batch_size": 800
      },
      "webdav": {
        "move_window": "15s",
        "batch_size": 200
      }
    }
  },
  "storage_roots": [
    {
      "name": "local_media",
      "protocol": "local",
      "enabled": true,
      "max_depth": 10,
      "settings": {
        "base_path": "/media/storage"
      }
    },
    {
      "name": "nas_smb",
      "protocol": "smb",
      "enabled": true,
      "max_depth": 8,
      "settings": {
        "host": "nas.local",
        "port": 445,
        "share": "media",
        "username": "catalogizer",
        "password": "secure-password",
        "domain": "WORKGROUP"
      }
    }
  ]
}
```

### 4. Running the Service

```bash
# Start the service
./bin/catalog-api

# With custom config
CONFIG_PATH=/etc/catalogizer/config.json ./bin/catalog-api

# As systemd service (see systemd section below)
systemctl start catalogizer
```

## Production Deployment

### Systemd Service

Create `/etc/systemd/system/catalogizer.service`:

```ini
[Unit]
Description=Catalogizer Media Catalog API
After=network.target
Wants=network.target

[Service]
Type=simple
User=catalogizer
Group=catalogizer
ExecStart=/opt/catalogizer/bin/catalog-api
Environment=CONFIG_PATH=/etc/catalogizer/config.json
Environment=LOG_LEVEL=info
WorkingDirectory=/opt/catalogizer
Restart=always
RestartSec=5s
StandardOutput=journal
StandardError=journal

# Security settings
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/catalogizer /var/log/catalogizer

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable catalogizer
sudo systemctl start catalogizer
sudo systemctl status catalogizer
```

### Docker Deployment

Create `Dockerfile`:

```dockerfile
FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -o catalog-api .

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /root/

COPY --from=builder /app/catalog-api .
COPY config.json .

CMD ["./catalog-api"]
```

Create `docker-compose.yml`:

```yaml
version: '3.8'

services:
  catalogizer:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - /media:/media:ro  # Mount read-only media
      - catalogizer_data:/data
      - ./config.json:/root/config.json:ro
    environment:
      - LOG_LEVEL=info
      - CONFIG_PATH=/root/config.json
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

volumes:
  catalogizer_data:
```

Run with Docker:

> Uses rootless Podman Compose; never use sudo. Release artifacts are built only inside rootless build containers (constitution §11.4.173).

```bash
podman compose up -d
```

### Kubernetes Deployment

Create `k8s-deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalogizer
  labels:
    app: catalogizer
spec:
  replicas: 2
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
        image: catalogizer:latest
        ports:
        - containerPort: 8080
        env:
        - name: CONFIG_PATH
          value: "/config/config.json"
        - name: LOG_LEVEL
          value: "info"
        volumeMounts:
        - name: config
          mountPath: /config
        - name: data
          mountPath: /data
        - name: media
          mountPath: /media
          readOnly: true
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "2Gi"
            cpu: "1000m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 30
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: config
        configMap:
          name: catalogizer-config
      - name: data
        persistentVolumeClaim:
          claimName: catalogizer-data
      - name: media
        hostPath:
          path: /media
          type: Directory

---
apiVersion: v1
kind: Service
metadata:
  name: catalogizer-service
spec:
  selector:
    app: catalogizer
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer

---
apiVersion: v1
kind: ConfigMap
metadata:
  name: catalogizer-config
data:
  config.json: |
    {
      "database": {
        "path": "/data/catalog.db",
        "encryption_key": "$DATABASE_ENCRYPTION_KEY"
      },
      "rename_detection": {
        "enabled": true,
        "workers": 4
      }
    }

---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: catalogizer-data
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 50Gi
```

Apply to Kubernetes:

```bash
kubectl apply -f k8s-deployment.yaml
```

## Protocol-Specific Setup

### SMB Configuration

For SMB connections, ensure proper authentication:

```json
{
  "name": "windows_share",
  "protocol": "smb",
  "settings": {
    "host": "windows-server.domain.com",
    "port": 445,
    "share": "MediaShare",
    "username": "serviceaccount",
    "password": "SecurePassword123!",
    "domain": "DOMAIN"
  }
}
```

Test SMB connectivity:

```bash
# Install smbclient for testing
sudo apt-get install smbclient

# Test connection
smbclient //windows-server.domain.com/MediaShare -U serviceaccount
```

### NFS Configuration

For NFS mounts, configure proper exports:

```bash
# On NFS server (/etc/exports)
/export/media *(rw,sync,no_subtree_check,no_root_squash)

# Restart NFS server
sudo systemctl restart nfs-kernel-server
```

Client configuration:

```json
{
  "name": "nfs_storage",
  "protocol": "nfs",
  "settings": {
    "host": "nfs-server.local",
    "export_path": "/export/media",
    "mount_point": "/mnt/nfs",
    "options": "rw,hard,intr"
  }
}
```

### FTP Configuration

For FTP connections with SSL/TLS:

```json
{
  "name": "ftp_archive",
  "protocol": "ftp",
  "settings": {
    "host": "ftp.example.com",
    "port": 21,
    "username": "ftpuser",
    "password": "ftppass",
    "use_tls": true,
    "passive_mode": true
  }
}
```

### WebDAV Configuration

For WebDAV connections:

```json
{
  "name": "webdav_cloud",
  "protocol": "webdav",
  "settings": {
    "url": "https://webdav.example.com/dav",
    "username": "webdavuser",
    "password": "webdavpass",
    "verify_ssl": true
  }
}
```

## Monitoring and Logging

### Prometheus Metrics

Add to your monitoring stack:

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'catalogizer'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: '/metrics'
    scrape_interval: 30s
```

Key metrics to monitor:

```
catalogizer_rename_detection_pending_moves{protocol="smb"}
catalogizer_rename_detection_success_rate{protocol="local"}
catalogizer_watcher_queue_length
catalogizer_watcher_events_processed_total
catalogizer_database_operations_duration_seconds
```

### Grafana Dashboard

Create a dashboard with these panels:

1. **Rename Detection Overview**
   - Total pending moves by protocol
   - Success rate trends
   - Processing time histograms

2. **File System Activity**
   - Events per second by operation type
   - Queue length over time
   - Worker utilization

3. **Protocol Performance**
   - Response times by protocol
   - Error rates
   - Connection status

### Log Configuration

Set up structured logging:

```json
{
  "logging": {
    "level": "info",
    "format": "json",
    "output": "/var/log/catalogizer/app.log",
    "max_size": "100MB",
    "max_backups": 5,
    "max_age": 30,
    "compress": true
  }
}
```

Log aggregation with ELK stack:

```yaml
# filebeat.yml
filebeat.inputs:
- type: log
  paths:
    - /var/log/catalogizer/*.log
  json.keys_under_root: true
  json.add_error_key: true

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
  index: "catalogizer-logs-%{+yyyy.MM.dd}"
```

## Security Hardening

### Network Security

1. **Firewall Configuration**:
```bash
# Allow only necessary ports
sudo ufw allow 8080/tcp  # API port
sudo ufw allow 445/tcp   # SMB (if needed)
sudo ufw allow 2049/tcp  # NFS (if needed)
```

2. **TLS Configuration**:
```json
{
  "tls": {
    "enabled": true,
    "cert_file": "/etc/ssl/certs/catalogizer.crt",
    "key_file": "/etc/ssl/private/catalogizer.key",
    "min_version": "1.2"
  }
}
```

### Authentication

1. **API Authentication**:
```json
{
  "auth": {
    "enabled": true,
    "jwt_secret": "your-jwt-secret-key",
    "token_expiry": "24h",
    "require_https": true
  }
}
```

2. **Database Encryption**:
```json
{
  "database": {
    "encryption_enabled": true,
    "encryption_key": "32-character-encryption-key-here",
    "backup_encryption": true
  }
}
```

### Access Control

Set up proper file permissions:

```bash
# Create catalogizer user
sudo useradd -r -s /bin/false catalogizer

# Set directory permissions
sudo mkdir -p /var/lib/catalogizer /var/log/catalogizer
sudo chown catalogizer:catalogizer /var/lib/catalogizer /var/log/catalogizer
sudo chmod 750 /var/lib/catalogizer /var/log/catalogizer

# Set binary permissions
sudo chown root:catalogizer /opt/catalogizer/bin/catalog-api
sudo chmod 750 /opt/catalogizer/bin/catalog-api
```

## Performance Tuning

### Database Optimization

1. **SQLite Settings**:
```json
{
  "database": {
    "pragma": {
      "journal_mode": "WAL",
      "synchronous": "NORMAL",
      "cache_size": "-64000",
      "temp_store": "MEMORY",
      "mmap_size": "268435456"
    }
  }
}
```

2. **Connection Pooling**:
```json
{
  "database": {
    "max_open_connections": 10,
    "max_idle_connections": 5,
    "connection_max_lifetime": "1h"
  }
}
```

### Memory Management

1. **Go Runtime Tuning**:
```bash
export GOGC=100
export GOMAXPROCS=4
export GOMEMLIMIT=2GiB
```

2. **System Tuning**:
```bash
# Increase file descriptor limits
echo "catalogizer soft nofile 65536" >> /etc/security/limits.conf
echo "catalogizer hard nofile 65536" >> /etc/security/limits.conf

# Optimize kernel parameters
echo "fs.file-max = 2097152" >> /etc/sysctl.conf
echo "vm.swappiness = 10" >> /etc/sysctl.conf
sysctl -p
```

## Backup and Recovery

### Database Backup

```bash
#!/bin/bash
# backup-catalogizer.sh

BACKUP_DIR="/backups/catalogizer"
DATE=$(date +%Y%m%d_%H%M%S)
DB_PATH="/var/lib/catalogizer/catalog.db"

# Create backup directory
mkdir -p "$BACKUP_DIR"

# Stop service for consistent backup
systemctl stop catalogizer

# Backup database
cp "$DB_PATH" "$BACKUP_DIR/catalog_${DATE}.db"

# Backup configuration
cp /etc/catalogizer/config.json "$BACKUP_DIR/config_${DATE}.json"

# Start service
systemctl start catalogizer

# Compress old backups
find "$BACKUP_DIR" -name "*.db" -mtime +7 -exec gzip {} \;

# Remove very old backups
find "$BACKUP_DIR" -name "*.gz" -mtime +30 -delete

echo "Backup completed: catalog_${DATE}.db"
```

### Disaster Recovery

1. **Recovery Process**:
```bash
# Stop service
systemctl stop catalogizer

# Restore database
cp /backups/catalogizer/catalog_YYYYMMDD_HHMMSS.db /var/lib/catalogizer/catalog.db

# Restore configuration
cp /backups/catalogizer/config_YYYYMMDD_HHMMSS.json /etc/catalogizer/config.json

# Set permissions
chown catalogizer:catalogizer /var/lib/catalogizer/catalog.db
chmod 640 /var/lib/catalogizer/catalog.db

# Start service
systemctl start catalogizer
```

2. **Data Validation**:
```bash
# Verify database integrity
sqlite3 /var/lib/catalogizer/catalog.db "PRAGMA integrity_check;"

# Check service status
systemctl status catalogizer
curl -f http://localhost:8080/health
```

## Troubleshooting

### Common Issues

1. **High Memory Usage**:
```bash
# Check memory usage
ps aux | grep catalog-api
cat /proc/$(pgrep catalog-api)/status | grep VmRSS

# Reduce batch sizes in config
"batch_size": 250  # Instead of 1000
```

2. **Slow Performance**:
```bash
# Check database locks
sqlite3 catalog.db ".timeout 1000"

# Monitor file system I/O
iotop -p $(pgrep catalog-api)

# Check network latency for remote protocols
ping nas.local
```

3. **Connection Errors**:
```bash
# Test SMB connectivity
smbclient -L //nas.local -U username

# Test NFS connectivity
showmount -e nfs-server.local

# Test FTP connectivity
telnet ftp.example.com 21
```

### Debug Mode

Enable debug logging:

```json
{
  "logging": {
    "level": "debug",
    "enable_trace": true
  },
  "rename_detection": {
    "debug_tracking": true,
    "log_all_events": true
  }
}
```

## Maintenance

### Regular Tasks

1. **Database Maintenance**:
```bash
# Weekly vacuum
sqlite3 /var/lib/catalogizer/catalog.db "VACUUM;"

# Monthly analyze
sqlite3 /var/lib/catalogizer/catalog.db "ANALYZE;"

# Check database size
ls -lh /var/lib/catalogizer/catalog.db
```

2. **Log Rotation**:
```bash
# Setup logrotate
cat > /etc/logrotate.d/catalogizer << EOF
/var/log/catalogizer/*.log {
    daily
    missingok
    rotate 30
    compress
    delaycompress
    notifempty
    create 644 catalogizer catalogizer
    postrotate
        systemctl reload catalogizer
    endscript
}
EOF
```

3. **Health Checks**:
```bash
# Daily health check script
#!/bin/bash
HEALTH_URL="http://localhost:8080/health"
RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" "$HEALTH_URL")

if [ "$RESPONSE" != "200" ]; then
    echo "Health check failed: $RESPONSE"
    systemctl restart catalogizer
fi
```

This deployment guide covers all aspects of running the Catalogizer system with rename detection in production environments. Adjust configurations based on your specific requirements and environment.