# Catalogizer - Installation & Setup Guide

![Installation Wizard](screenshots/wizard/welcome-step.png)
*Welcome screen of the Catalogizer installation wizard*

## 📋 Table of Contents

1. [System Requirements](#system-requirements)
2. [Pre-Installation Checklist](#pre-installation-checklist)
3. [Installation Methods](#installation-methods)
4. [Database Setup](#database-setup)
5. [Configuration](#configuration)
6. [First-Time Setup Wizard](#first-time-setup-wizard)
7. [Advanced Configuration](#advanced-configuration)
8. [Troubleshooting](#troubleshooting)
9. [Upgrade Instructions](#upgrade-instructions)

---

## 🖥️ System Requirements

### Minimum Requirements
- **CPU**: 2 cores, 2.0 GHz
- **RAM**: 4 GB
- **Storage**: 10 GB free space
- **Operating System**:
  - Linux (Ubuntu 20.04+, CentOS 8+, Debian 11+)
  - macOS 11.0+
  - Windows 10/11
- **Go Runtime**: Version 1.21 or later

### Recommended Requirements
- **CPU**: 4+ cores, 2.5+ GHz
- **RAM**: 8+ GB
- **Storage**: 50+ GB SSD
- **Database**: Dedicated database server
- **Network**: Gigabit connection for media processing

### Software Dependencies
- **Required**:
  - Go 1.21+
  - SQLite (included) OR MySQL 8.0+ OR PostgreSQL 13+
- **Optional**:
  - FFmpeg (for video conversion)
  - ImageMagick (for image processing)
  - Calibre (for document conversion)
  - Docker (for containerized deployment)

---

## ✅ Pre-Installation Checklist

![Pre-Installation Check](screenshots/wizard/requirements-check.png)
*System requirements verification in the wizard*

### Before You Begin
- [ ] Verify system meets minimum requirements
- [ ] Download latest Catalogizer v3.0 release
- [ ] Prepare database (if using MySQL/PostgreSQL)
- [ ] Plan storage locations for media files
- [ ] Configure firewall rules (if needed)
- [ ] Backup existing data (if upgrading)

### Required Permissions
- [ ] Administrative/sudo access for installation
- [ ] Write permissions for installation directory
- [ ] Database creation privileges
- [ ] Network port access (default: 8080)

---

## 📦 Installation Methods

### Method 1: Binary Installation (Recommended)

#### Linux/macOS
```bash
# Download the latest release
wget https://github.com/catalogizer/releases/download/v3.0.0/catalogizer-v3.0.0-linux-amd64.tar.gz

# Extract the archive
tar -xzf catalogizer-v3.0.0-linux-amd64.tar.gz

# Move to installation directory
sudo mv catalogizer /usr/local/bin/

# Make executable
sudo chmod +x /usr/local/bin/catalogizer

# Verify installation
catalogizer --version
```

#### Windows
1. Download `catalogizer-v3.0.0-windows-amd64.zip`
2. Extract to `C:\Program Files\Catalogizer\`
3. Add to PATH environment variable
4. Open Command Prompt and run `catalogizer --version`

### Method 2: Docker Installation

![Docker Setup](screenshots/docker/docker-setup.png)
*Docker container configuration interface*

> Uses rootless Podman Compose; never use sudo. Release artifacts are built only inside rootless build containers (constitution §11.4.173).

```bash
# Pull the official image
docker pull catalogizer/catalogizer:v3.0.0

# Create data directories
mkdir -p ./data/media ./data/db ./data/logs

# Run with Docker Compose
curl -o docker-compose.yml https://raw.githubusercontent.com/catalogizer/catalogizer/main/docker-compose.yml

# Start the services
podman compose up -d

# Check status
podman compose ps
```

**docker-compose.yml example:**
```yaml
version: '3.8'

services:
  catalogizer:
    image: catalogizer/catalogizer:v3.0.0
    ports:
      - "8080:8080"
    volumes:
      - ./data/media:/data/media
      - ./data/db:/data/db
      - ./data/logs:/data/logs
      - ./config:/config
    environment:
      - CATALOGIZER_DATABASE_TYPE=sqlite
      - CATALOGIZER_DATABASE_PATH=/data/db/catalogizer.db
      - CATALOGIZER_MEDIA_DIR=/data/media
    restart: unless-stopped

  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: catalogizer
      POSTGRES_USER: catalogizer
      POSTGRES_PASSWORD: secure_password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    restart: unless-stopped

volumes:
  postgres_data:
```

### Method 3: Source Code Installation

```bash
# Prerequisites: Go 1.21+
go version

# Clone the repository
git clone https://github.com/catalogizer/catalogizer.git
cd catalogizer

# Build from source
make build

# Install
sudo make install

# Verify
catalogizer --version
```

---

## 🗄️ Database Setup

### SQLite (Default - No Setup Required)
SQLite is the default database and requires no additional setup. The database file will be created automatically.

![SQLite Setup](screenshots/wizard/database-sqlite.png)
*SQLite configuration in the setup wizard*

### MySQL Setup

![MySQL Setup](screenshots/wizard/database-mysql.png)
*MySQL database configuration interface*

#### 1. Install MySQL Server
```bash
# Ubuntu/Debian
sudo apt update
sudo apt install mysql-server

# CentOS/RHEL
sudo yum install mysql-server

# macOS (with Homebrew)
brew install mysql
```

#### 2. Create Database and User
```sql
-- Connect to MySQL as root
mysql -u root -p

-- Create database
CREATE DATABASE catalogizer CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- Create user
CREATE USER 'catalogizer'@'localhost' IDENTIFIED BY 'secure_password';

-- Grant permissions
GRANT ALL PRIVILEGES ON catalogizer.* TO 'catalogizer'@'localhost';
FLUSH PRIVILEGES;

-- Exit
EXIT;
```

#### 3. Test Connection
```bash
mysql -u catalogizer -p catalogizer
```

### PostgreSQL Setup

![PostgreSQL Setup](screenshots/wizard/database-postgresql.png)
*PostgreSQL configuration options*

#### 1. Install PostgreSQL
```bash
# Ubuntu/Debian
sudo apt update
sudo apt install postgresql postgresql-contrib

# CentOS/RHEL
sudo yum install postgresql-server postgresql-contrib

# macOS (with Homebrew)
brew install postgresql
```

#### 2. Initialize and Start Service
```bash
# Linux
sudo systemctl start postgresql
sudo systemctl enable postgresql

# macOS
brew services start postgresql
```

#### 3. Create Database and User
```bash
# Switch to postgres user
sudo -u postgres psql

# Create user
CREATE USER catalogizer WITH PASSWORD 'secure_password';

# Create database
CREATE DATABASE catalogizer OWNER catalogizer;

# Grant permissions
GRANT ALL PRIVILEGES ON DATABASE catalogizer TO catalogizer;

# Exit
\q
```

---

## ⚙️ Configuration

### Configuration File Location
- **Linux**: `/etc/catalogizer/config.json`
- **macOS**: `/usr/local/etc/catalogizer/config.json`
- **Windows**: `C:\ProgramData\Catalogizer\config.json`
- **Docker**: `/config/config.json` (mounted volume)

### Basic Configuration (config.json)

![Configuration File](screenshots/admin/config-file-edit.png)
*Configuration file editor interface*

```json
{
  "version": "3.0.0",
  "database": {
    "type": "sqlite",
    "host": "localhost",
    "port": 5432,
    "name": "catalogizer",
    "username": "catalogizer",
    "password": "secure_password",
    "ssl_mode": "disable"
  },
  "storage": {
    "media_directory": "/var/lib/catalogizer/media",
    "thumbnail_directory": "/var/lib/catalogizer/thumbnails",
    "temp_directory": "/tmp/catalogizer",
    "max_file_size": 1073741824,
    "storage_quota": 0
  },
  "network": {
    "host": "0.0.0.0",
    "port": 8080,
    "https": {
      "enabled": false,
      "cert_path": "/etc/ssl/certs/catalogizer.crt",
      "key_path": "/etc/ssl/private/catalogizer.key"
    },
    "cors": {
      "allowed_origins": ["*"],
      "allowed_methods": ["GET", "POST", "PUT", "DELETE"],
      "allowed_headers": ["*"]
    }
  },
  "authentication": {
    "jwt_secret": "your-secret-key-here",
    "session_timeout": "24h",
    "enable_registration": true,
    "require_email_verification": false,
    "admin_email": "admin@yourdomain.com"
  },
  "features": {
    "media_conversion": true,
    "webdav_sync": false,
    "stress_testing": false,
    "error_reporting": true,
    "log_management": true
  },
  "external_services": {
    "smtp": {
      "host": "smtp.gmail.com",
      "port": 587,
      "username": "your-email@gmail.com",
      "password": "app-password"
    },
    "slack": {
      "webhook_url": "https://hooks.slack.com/..."
    },
    "analytics": true
  }
}
```

### Environment Variables
You can also configure Catalogizer using environment variables:

```bash
# Database
export CATALOGIZER_DATABASE_TYPE=postgresql
export CATALOGIZER_DATABASE_HOST=localhost
export CATALOGIZER_DATABASE_PORT=5432
export CATALOGIZER_DATABASE_NAME=catalogizer
export CATALOGIZER_DATABASE_USERNAME=catalogizer
export CATALOGIZER_DATABASE_PASSWORD=secure_password

# Storage
export CATALOGIZER_MEDIA_DIR=/data/media
export CATALOGIZER_THUMBNAIL_DIR=/data/thumbnails

# Network
export CATALOGIZER_HOST=0.0.0.0
export CATALOGIZER_PORT=8080

# Security
export CATALOGIZER_JWT_SECRET=your-jwt-secret
```

---

## 🧙‍♂️ First-Time Setup Wizard

![Setup Wizard Overview](screenshots/wizard/wizard-overview.png)
*Complete setup wizard flow*

When you first start Catalogizer, you'll be guided through an interactive setup wizard.

### Step 1: Welcome & Requirements
![Welcome Step](screenshots/wizard/welcome-step.png)
*Welcome screen with system requirements check*

The wizard will:
- Check system requirements
- Verify dependencies
- Display license information

### Step 2: Database Configuration
![Database Step](screenshots/wizard/database-step.png)
*Database configuration with connection testing*

Choose your database type and provide connection details:
- **SQLite**: No additional configuration needed
- **MySQL**: Host, port, database name, credentials
- **PostgreSQL**: Host, port, database name, credentials

The wizard will test the connection before proceeding.

### Step 3: Storage Configuration
![Storage Step](screenshots/wizard/storage-step.png)
*Storage location and capacity settings*

Configure storage locations:
- **Media Directory**: Where uploaded files are stored
- **Thumbnail Directory**: Generated thumbnails location
- **Temporary Directory**: Processing workspace
- **Storage Limits**: Optional storage quotas

### Step 4: Network Configuration
![Network Step](screenshots/wizard/network-step.png)
*Network and security settings configuration*

Set up network access:
- **Server Address**: IP address to bind to
- **Port**: HTTP port (default: 8080)
- **HTTPS**: SSL certificate configuration
- **CORS**: Cross-origin request settings

### Step 5: Authentication Setup
![Authentication Step](screenshots/wizard/authentication-step.png)
*User authentication and security configuration*

Configure security settings:
- **JWT Secret**: Secure token signing key
- **Session Duration**: How long users stay logged in
- **Registration**: Allow new user registration
- **Admin Account**: Create the first administrator

### Step 6: Feature Selection
![Features Step](screenshots/wizard/features-step.png)
*Advanced feature enablement interface*

Enable optional features:
- **Media Conversion**: Video/audio format conversion
- **WebDAV Sync**: Cloud storage synchronization
- **Stress Testing**: Performance testing tools
- **Error Reporting**: Crash and error tracking
- **Log Management**: Centralized logging

### Step 7: External Services
![External Services Step](screenshots/wizard/external-services-step.png)
*Third-party service integration setup*

Configure integrations:
- **Email (SMTP)**: For notifications and verification
- **Slack**: Real-time notifications
- **Analytics**: Usage tracking

### Step 8: Configuration Summary
![Summary Step](screenshots/wizard/summary-step.png)
*Configuration review before finalization*

Review all settings before applying:
- Configuration summary
- Test connections
- Preview generated config file

### Step 9: Installation Complete
![Complete Step](screenshots/wizard/complete-step.png)
*Installation completion with next steps*

Final steps:
- Configuration applied
- Database initialized
- First admin user created
- Access URLs provided

---

## 🔧 Advanced Configuration

### SSL/HTTPS Setup

![HTTPS Configuration](screenshots/admin/https-setup.png)
*SSL certificate configuration interface*

#### Using Let's Encrypt
```bash
# Install certbot
sudo apt install certbot

# Generate certificate
sudo certbot certonly --standalone -d yourdomain.com

# Update config.json
{
  "network": {
    "https": {
      "enabled": true,
      "cert_path": "/etc/letsencrypt/live/yourdomain.com/fullchain.pem",
      "key_path": "/etc/letsencrypt/live/yourdomain.com/privkey.pem"
    }
  }
}

# Restart Catalogizer
sudo systemctl restart catalogizer
```

#### Using Custom Certificates
```bash
# Place your certificates
sudo cp your-cert.crt /etc/ssl/certs/catalogizer.crt
sudo cp your-key.key /etc/ssl/private/catalogizer.key

# Set permissions
sudo chmod 644 /etc/ssl/certs/catalogizer.crt
sudo chmod 600 /etc/ssl/private/catalogizer.key
```

### Reverse Proxy Setup (Nginx)

![Nginx Configuration](screenshots/admin/nginx-setup.png)
*Nginx reverse proxy configuration*

Create `/etc/nginx/sites-available/catalogizer`:
```nginx
server {
    listen 80;
    server_name yourdomain.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # WebSocket support
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
    }

    # Media files direct serving
    location /media/ {
        alias /var/lib/catalogizer/media/;
        expires 1y;
        add_header Cache-Control "public, immutable";
    }
}
```

Enable the site:
```bash
sudo ln -s /etc/nginx/sites-available/catalogizer /etc/nginx/sites-enabled/
sudo nginx -t
sudo systemctl reload nginx
```

### Systemd Service Setup

![Service Configuration](screenshots/admin/service-setup.png)
*System service configuration interface*

Create `/etc/systemd/system/catalogizer.service`:
```ini
[Unit]
Description=Catalogizer Media Management System
After=network.target
Requires=network.target

[Service]
Type=simple
User=catalogizer
Group=catalogizer
WorkingDirectory=/var/lib/catalogizer
ExecStart=/usr/local/bin/catalogizer --config /etc/catalogizer/config.json
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal

# Environment
Environment=CATALOGIZER_ENV=production

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ReadWritePaths=/var/lib/catalogizer /var/log/catalogizer

[Install]
WantedBy=multi-user.target
```

Enable and start the service:
```bash
sudo systemctl daemon-reload
sudo systemctl enable catalogizer
sudo systemctl start catalogizer
sudo systemctl status catalogizer
```

---

## 🔍 Troubleshooting

### Common Installation Issues

#### Issue: "Command not found"
![Command Not Found](screenshots/troubleshooting/command-not-found.png)

**Solution:**
```bash
# Check if binary is in PATH
echo $PATH

# Add to PATH (Linux/macOS)
export PATH=$PATH:/usr/local/bin

# Make permanent (add to ~/.bashrc or ~/.zshrc)
echo 'export PATH=$PATH:/usr/local/bin' >> ~/.bashrc
```

#### Issue: Database Connection Failed
![Database Error](screenshots/troubleshooting/database-error.png)

**Solutions:**
1. **Check database service status:**
   ```bash
   # MySQL
   sudo systemctl status mysql

   # PostgreSQL
   sudo systemctl status postgresql
   ```

2. **Verify credentials:**
   ```bash
   # Test MySQL connection
   mysql -u catalogizer -p catalogizer

   # Test PostgreSQL connection
   psql -U catalogizer -d catalogizer -h localhost
   ```

3. **Check firewall rules:**
   ```bash
   # Allow MySQL port
   sudo ufw allow 3306

   # Allow PostgreSQL port
   sudo ufw allow 5432
   ```

#### Issue: Permission Denied
![Permission Error](screenshots/troubleshooting/permission-error.png)

**Solutions:**
```bash
# Create catalogizer user
sudo useradd -r -s /bin/false catalogizer

# Set directory ownership
sudo chown -R catalogizer:catalogizer /var/lib/catalogizer

# Set correct permissions
sudo chmod 755 /var/lib/catalogizer
sudo chmod -R 644 /var/lib/catalogizer/*
```

#### Issue: Port Already in Use
![Port Conflict](screenshots/troubleshooting/port-conflict.png)

**Solutions:**
```bash
# Check what's using the port
sudo netstat -tlnp | grep :8080
sudo lsof -i :8080

# Change port in config.json or kill conflicting process
sudo kill -9 <PID>
```

### Log File Locations
- **Application Logs**: `/var/log/catalogizer/app.log`
- **Error Logs**: `/var/log/catalogizer/error.log`
- **Access Logs**: `/var/log/catalogizer/access.log`
- **System Logs**: `sudo journalctl -u catalogizer`

### Getting Help

![Support Interface](screenshots/troubleshooting/support-interface.png)
*Built-in support and diagnostics interface*

1. **Built-in Diagnostics**: `/admin/diagnostics`
2. **Log Viewer**: `/admin/logs`
3. **System Health**: `/admin/health`
4. **Community Forum**: https://community.catalogizer.com
5. **GitHub Issues**: https://github.com/catalogizer/catalogizer/issues

---

## 📈 Upgrade Instructions

### Backup Before Upgrading

![Backup Interface](screenshots/admin/backup-interface.png)
*System backup and restore interface*

```bash
# Stop Catalogizer service
sudo systemctl stop catalogizer

# Backup database
sudo -u catalogizer catalogizer backup --output /backup/catalogizer-$(date +%Y%m%d).sql

# Backup media files
sudo tar -czf /backup/media-$(date +%Y%m%d).tar.gz /var/lib/catalogizer/media/

# Backup configuration
sudo cp -r /etc/catalogizer /backup/config-$(date +%Y%m%d)/
```

### Upgrade Process

#### Binary Upgrade
```bash
# Download new version
wget https://github.com/catalogizer/releases/download/v3.1.0/catalogizer-v3.1.0-linux-amd64.tar.gz

# Backup current binary
sudo cp /usr/local/bin/catalogizer /usr/local/bin/catalogizer.backup

# Install new binary
tar -xzf catalogizer-v3.1.0-linux-amd64.tar.gz
sudo mv catalogizer /usr/local/bin/

# Run database migrations
sudo -u catalogizer catalogizer migrate

# Start service
sudo systemctl start catalogizer
```

#### Docker Upgrade
```bash
# Pull new image
docker pull catalogizer/catalogizer:v3.1.0

# Update docker-compose.yml
sed -i 's/v3.0.0/v3.1.0/g' docker-compose.yml

# Restart containers
podman compose down
podman compose up -d
```

### Post-Upgrade Verification

![Upgrade Verification](screenshots/admin/upgrade-verification.png)
*Post-upgrade system verification*

1. **Check service status:**
   ```bash
   sudo systemctl status catalogizer
   ```

2. **Verify web interface:**
   - Open browser to `http://localhost:8080`
   - Login with admin credentials
   - Check version in admin panel

3. **Test core functionality:**
   - Upload a test file
   - Create a collection
   - Verify API endpoints

4. **Check logs for errors:**
   ```bash
   sudo journalctl -u catalogizer -f
   ```

---

## 🎉 Next Steps

After successful installation:

1. **Complete the setup wizard**
2. **Create your first user accounts**
3. **Upload some test media**
4. **Explore the features**
5. **Configure integrations**
6. **Set up monitoring**
7. **Schedule regular backups**

![Getting Started](screenshots/dashboard/getting-started.png)
*Getting started guide in the main dashboard*

**Welcome to Catalogizer v3.0!** 🚀

For additional help, consult the [User Guide](USER_GUIDE.md) and [Configuration Guide](CONFIGURATION_GUIDE.md).