#!/usr/bin/env bash
# install_env.sh - generate the installer's .env with RANDOM secrets.
#   install_write_env <file>
# Writes <file> (mode 0600, umask 077) containing freshly generated random
# values for every secret docker-compose.yml requires (POSTGRES_PASSWORD /
# DATABASE_PASSWORD, JWT_SECRET, ADMIN_PASSWORD, GRAFANA_PASSWORD,
# MINIO_ROOT_PASSWORD). Never overwrites an existing file; never prints a value.

# gen_secret <bytes>  -> 2*<bytes> hex characters from the OS CSPRNG.
gen_secret() {
    local n="${1:-24}"
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -hex "$n"
    else
        od -An -N"$n" -tx1 /dev/urandom | tr -d ' \n'
    fi
}

install_write_env() {
    local target="${1:?install_write_env: target file required}"
    if [ -e "$target" ] || [ -L "$target" ]; then
        echo "refusing: $target already exists; left untouched" >&2
        return 2
    fi
    local _db_pw _jwt _admin_pw _grafana_pw _minio_pw _minio_user _pgadmin_pw _sonar_pw
    _db_pw="$(gen_secret 24)" || return 1
    _jwt="$(gen_secret 32)" || return 1
    _admin_pw="$(gen_secret 16)" || return 1
    _grafana_pw="$(gen_secret 16)" || return 1
    _minio_pw="$(gen_secret 16)" || return 1
    _minio_user="cat$(gen_secret 6)" || return 1
    _pgadmin_pw="$(gen_secret 16)" || return 1
    _sonar_pw="$(gen_secret 16)" || return 1
    (
        umask 077
        set -o noclobber   # O_EXCL create: never follows a dangling symlink, never truncates
        cat > "$target" << EOF
# Catalogizer Environment Configuration
# Copy this file and modify as needed for your environment

#==============================================================================
# GENERAL SETTINGS
#==============================================================================

# Environment mode (development, staging, production)
CATALOGIZER_ENV=production

# Application version
CATALOGIZER_VERSION=1.0.0

# Installation directory
INSTALL_DIR=/opt/catalogizer

# Data directory for media files and databases
DATA_DIR=/var/lib/catalogizer

# Logs directory
LOGS_DIR=/var/log/catalogizer

#==============================================================================
# SERVER CONFIGURATION
#==============================================================================

# Server hostname/IP
CATALOGIZER_HOST=localhost

# API server port
CATALOGIZER_PORT=8080

# WebSocket port
CATALOGIZER_WS_PORT=8081

# Admin interface port
CATALOGIZER_ADMIN_PORT=9090

# Database configuration
DATABASE_TYPE=postgres  # postgres, mysql, sqlite
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_NAME=catalogizer
DATABASE_USER=catalogizer
DATABASE_PASSWORD=${_db_pw}
POSTGRES_PASSWORD=${_db_pw}

# Redis configuration (for caching and sessions)
REDIS_HOST=localhost
REDIS_PORT=6379
REDIS_PASSWORD=

#==============================================================================
# SECURITY SETTINGS
#==============================================================================

# JWT Secret for authentication (generated randomly by the installer)
JWT_SECRET=${_jwt}

# Admin user credentials (password generated randomly by the installer)
ADMIN_USERNAME=admin
ADMIN_PASSWORD=${_admin_pw}
ADMIN_EMAIL=admin@example.com

# Monitoring / object-storage secrets required by docker-compose.yml (generated)
GRAFANA_PASSWORD=${_grafana_pw}
MINIO_ROOT_USER=${_minio_user}
MINIO_ROOT_PASSWORD=${_minio_pw}
STORAGE_ACCESS_KEY=${_minio_user}
STORAGE_SECRET_KEY=${_minio_pw}
PGADMIN_DEFAULT_PASSWORD=${_pgadmin_pw}
SONAR_DB_PASSWORD=${_sonar_pw}

# SSL/TLS Configuration
SSL_ENABLED=false
SSL_CERT_PATH=/etc/catalogizer/ssl/cert.pem
SSL_KEY_PATH=/etc/catalogizer/ssl/key.pem

#==============================================================================
# MEDIA CONFIGURATION
#==============================================================================

# Default media directories (comma-separated)
MEDIA_DIRECTORIES=/mnt/media,/storage/media

# SMB/CIFS default configuration
SMB_WORKGROUP=WORKGROUP
SMB_DEFAULT_PORT=445

# Media processing
TRANSCODE_ENABLED=true
TRANSCODE_QUALITY=medium  # low, medium, high
THUMBNAIL_GENERATION=true

#==============================================================================
# DOCKER CONFIGURATION
#==============================================================================

# Docker network name
DOCKER_NETWORK=catalogizer-network

# Docker volumes
DOCKER_DATA_VOLUME=catalogizer-data
DOCKER_MEDIA_VOLUME=catalogizer-media
DOCKER_CONFIG_VOLUME=catalogizer-config

# Container restart policy
RESTART_POLICY=unless-stopped

#==============================================================================
# CLIENT BUILD CONFIGURATION
#==============================================================================

# Android build configuration
ANDROID_COMPILE_SDK=34
ANDROID_MIN_SDK=26
ANDROID_TARGET_SDK=34

# Desktop build targets (comma-separated)
DESKTOP_TARGETS=windows,macos,linux

# Build output directory
BUILD_OUTPUT_DIR=./releases

#==============================================================================
# DEPLOYMENT CONFIGURATION
#==============================================================================

# Git repository for updates
GIT_REPOSITORY=https://github.com/catalogizer/catalogizer.git
GIT_BRANCH=main

# Backup configuration
BACKUP_ENABLED=true
BACKUP_SCHEDULE="0 2 * * *"  # Daily at 2 AM
BACKUP_RETENTION_DAYS=30

# Monitoring
MONITORING_ENABLED=false
METRICS_PORT=9091

# Update check
AUTO_UPDATE_CHECK=true
UPDATE_CHANNEL=stable  # stable, beta, nightly

EOF
    ) || return 1
    chmod 600 "$target"
    echo "Created $target with randomly generated secrets (mode 0600; values not shown)"
}
