# Catalogizer v3.0 - Troubleshooting and Maintenance Guide

## Table of Contents
1. [Overview](#overview)
2. [Quick Diagnostics](#quick-diagnostics)
3. [Common Issues](#common-issues)
4. [Service Management](#service-management)
5. [Database Issues](#database-issues)
6. [Storage and Media Issues](#storage-and-media-issues)
7. [Performance Issues](#performance-issues)
8. [Network and Connectivity](#network-and-connectivity)
9. [Authentication and Authorization](#authentication-and-authorization)
10. [Configuration Problems](#configuration-problems)
11. [Log Analysis](#log-analysis)
12. [Debug Tools and Utilities](#debug-tools-and-utilities)
13. [Maintenance Procedures](#maintenance-procedures)
14. [Recovery Procedures](#recovery-procedures)
15. [Prevention and Monitoring](#prevention-and-monitoring)

## Overview

This guide provides comprehensive troubleshooting procedures and maintenance tasks for Catalogizer v3.0. It covers common issues, diagnostic procedures, and preventive maintenance to ensure optimal system operation.

### Support Channels

- **Bug Reports & Issues**: https://github.com/vasic-digital/Catalogizer/issues
- **Documentation**: https://catalogizer.dev/documentation
- **Community Discussions**: https://github.com/vasic-digital/Catalogizer/discussions
- **Project Website**: https://catalogizer.dev

## Quick Diagnostics

### Health Check Commands

```bash
# Quick system health check
curl -s http://localhost:8080/health | jq

# Service status
sudo systemctl status catalogizer

# Resource usage
top -p $(pgrep catalogizer)
free -h
df -h

# Log tail for immediate issues
sudo tail -f /opt/catalogizer/logs/catalogizer.log

# Database connectivity
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "SELECT 1;"
```

### System Information Script

```bash
#!/bin/bash
# system_info.sh - Quick system diagnostics

echo "=== Catalogizer System Information ==="
echo "Date: $(date)"
echo "Hostname: $(hostname)"
echo "Uptime: $(uptime)"
echo

echo "=== Service Status ==="
sudo systemctl status catalogizer --no-pager
echo

echo "=== Resource Usage ==="
echo "Memory:"
free -h
echo
echo "Disk:"
df -h /opt/catalogizer
echo
echo "CPU Load:"
cat /proc/loadavg
echo

echo "=== Network Status ==="
netstat -tulpn | grep :8080
echo

echo "=== Recent Errors ==="
sudo tail -20 /opt/catalogizer/logs/catalogizer.log | grep -i error
echo

echo "=== Database Status ==="
if command -v psql >/dev/null 2>&1; then
    PGPASSWORD=$DB_PASSWORD psql -h ${DB_HOST:-localhost} -U ${DB_USER:-catalogizer_user} -d ${DB_NAME:-catalogizer} -c "SELECT count(*) as total_users FROM users;" 2>/dev/null || echo "Database connection failed"
else
    echo "PostgreSQL client not installed"
fi
```

### Quick Fix Commands

```bash
# Restart service
sudo systemctl restart catalogizer

# Clear logs (if disk space issue)
sudo truncate -s 0 /opt/catalogizer/logs/catalogizer.log

# Check and fix permissions
sudo chown -R catalogizer:catalogizer /opt/catalogizer
sudo chmod 755 /opt/catalogizer/bin/catalogizer

# Reload configuration
curl -X POST http://localhost:8080/api/config/reload

# Clear cache (if using Redis)
redis-cli FLUSHDB
```

## Common Issues

### Issue: Service Won't Start

**Symptoms:**
- Service fails to start
- Error: "Job for catalogizer.service failed"
- Application exits immediately

**Diagnosis:**
```bash
# Check service status
sudo systemctl status catalogizer -l

# Check logs
sudo journalctl -u catalogizer -f

# Test configuration
/opt/catalogizer/bin/catalogizer --config /opt/catalogizer/config/config.json --validate

# Check file permissions
ls -la /opt/catalogizer/bin/catalogizer
ls -la /opt/catalogizer/config/config.json
```

**Solutions:**

1. **Configuration Issues:**
```bash
# Validate configuration
/opt/catalogizer/bin/catalogizer --validate-config

# Check for syntax errors
cat /opt/catalogizer/config/config.json | jq
```

2. **Permission Issues:**
```bash
sudo chown catalogizer:catalogizer /opt/catalogizer/bin/catalogizer
sudo chmod +x /opt/catalogizer/bin/catalogizer
sudo chown -R catalogizer:catalogizer /opt/catalogizer/config
```

3. **Port Already in Use:**
```bash
# Check what's using the port
sudo netstat -tulpn | grep :8080
sudo lsof -i :8080

# Kill conflicting process
sudo kill $(sudo lsof -t -i:8080)
```

4. **Missing Dependencies:**
```bash
# Check Go installation
go version

# Check required libraries
ldd /opt/catalogizer/bin/catalogizer
```

### Issue: Database Connection Failed

**Symptoms:**
- "Connection refused" errors
- "Authentication failed" messages
- Timeout errors

**Diagnosis:**
```bash
# Test database connectivity
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c "SELECT version();"

# Check database service status
sudo systemctl status postgresql

# Check database logs
sudo tail -f /var/log/postgresql/postgresql-14-main.log

# Verify connection parameters
echo "Host: $DB_HOST"
echo "Port: $DB_PORT"
echo "User: $DB_USER"
echo "Database: $DB_NAME"
```

**Solutions:**

1. **Database Not Running:**
```bash
sudo systemctl start postgresql
sudo systemctl enable postgresql
```

2. **Incorrect Credentials:**
```bash
# Reset password
sudo -u postgres psql -c "ALTER USER catalogizer_user PASSWORD 'new_password';"

# Update configuration
sudo nano /opt/catalogizer/config/config.json
```

3. **Connection Limit Reached:**
```bash
# Check active connections
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "
SELECT count(*) as active_connections, setting as max_connections
FROM pg_stat_activity, pg_settings
WHERE name='max_connections';"

# Kill idle connections
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE state = 'idle' AND state_change < now() - interval '1 hour';"
```

4. **Network Issues:**
```bash
# Check firewall rules
sudo ufw status
sudo iptables -L

# Test network connectivity
telnet $DB_HOST $DB_PORT
```

### Issue: High Memory Usage

**Symptoms:**
- System becomes unresponsive
- Out of memory errors
- Swap usage is high

**Diagnosis:**
```bash
# Check memory usage
free -h
ps aux --sort=-%mem | head -10

# Check Catalogizer memory usage
ps aux | grep catalogizer

# Check for memory leaks
cat /proc/$(pgrep catalogizer)/status | grep -i vm

# Generate memory profile
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof
```

**Solutions:**

1. **Increase System Memory:**
```bash
# Add swap space (temporary solution)
sudo fallocate -l 2G /swapfile
sudo chmod 600 /swapfile
sudo mkswap /swapfile
sudo swapon /swapfile
```

2. **Optimize Application:**
```bash
# Restart service to clear memory
sudo systemctl restart catalogizer

# Adjust memory limits
sudo systemctl edit catalogizer
# Add:
# [Service]
# MemoryLimit=2G
```

3. **Configuration Tuning:**
```json
{
  "performance": {
    "worker_count": 2,
    "cache_size": "256MB",
    "queue_size": 500
  }
}
```

### Issue: Slow Performance

**Symptoms:**
- High response times
- Timeouts
- Users complaining about slowness

**Diagnosis:**
```bash
# Check CPU usage
top -p $(pgrep catalogizer)

# Check I/O wait
iostat -x 1 5

# Check network latency
ping -c 5 $DB_HOST

# Generate CPU profile
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Check database performance
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "
SELECT query, mean_time, calls
FROM pg_stat_statements
ORDER BY mean_time DESC
LIMIT 10;"
```

**Solutions:**

1. **Database Optimization:**
```sql
-- Update statistics
ANALYZE;

-- Check for missing indexes
SELECT schemaname, tablename, attname, n_distinct, correlation
FROM pg_stats
WHERE schemaname = 'public'
ORDER BY n_distinct DESC;

-- Add indexes for slow queries
CREATE INDEX CONCURRENTLY idx_media_items_user_created
ON media_items(user_id, created_at);
```

2. **Application Tuning:**
```json
{
  "database": {
    "max_connections": 20,
    "max_idle_connections": 10,
    "connection_lifetime": "1h"
  },
  "performance": {
    "cache": {
      "enabled": true,
      "size": "512MB",
      "ttl": "1h"
    }
  }
}
```

3. **System Optimization:**
```bash
# Increase file descriptors
echo "catalogizer soft nofile 65536" | sudo tee -a /etc/security/limits.conf
echo "catalogizer hard nofile 65536" | sudo tee -a /etc/security/limits.conf

# Optimize kernel parameters
echo "net.core.somaxconn = 65535" | sudo tee -a /etc/sysctl.conf
sudo sysctl -p
```

### Issue: SSL/TLS Certificate Problems

**Symptoms:**
- Browser warnings about insecure connections
- Certificate expired errors
- SSL handshake failures

**Diagnosis:**
```bash
# Check certificate validity
openssl x509 -in /etc/ssl/certs/catalogizer.crt -text -noout

# Test SSL connection
openssl s_client -connect yourdomain.com:443 -servername yourdomain.com

# Check certificate expiry
echo | openssl s_client -connect yourdomain.com:443 2>/dev/null | openssl x509 -noout -dates

# Verify certificate chain
curl -I https://yourdomain.com
```

**Solutions:**

1. **Renew Let's Encrypt Certificate:**
```bash
sudo certbot renew --dry-run
sudo certbot renew
sudo systemctl reload nginx
```

2. **Update Certificate Files:**
```bash
# Copy new certificate
sudo cp new_certificate.crt /etc/ssl/certs/catalogizer.crt
sudo cp new_private.key /etc/ssl/private/catalogizer.key

# Set permissions
sudo chmod 644 /etc/ssl/certs/catalogizer.crt
sudo chmod 600 /etc/ssl/private/catalogizer.key

# Restart web server
sudo systemctl restart nginx
```

3. **Fix Certificate Chain:**
```bash
# Concatenate intermediate certificates
cat domain.crt intermediate.crt > /etc/ssl/certs/catalogizer.crt
```

## Service Management

### Systemd Service Operations

```bash
# Service status and control
sudo systemctl status catalogizer
sudo systemctl start catalogizer
sudo systemctl stop catalogizer
sudo systemctl restart catalogizer
sudo systemctl reload catalogizer

# Enable/disable service
sudo systemctl enable catalogizer
sudo systemctl disable catalogizer

# View service logs
sudo journalctl -u catalogizer -f
sudo journalctl -u catalogizer --since "1 hour ago"
sudo journalctl -u catalogizer --since "2024-01-01"

# Check service configuration
sudo systemctl cat catalogizer
sudo systemctl show catalogizer
```

### Process Management

```bash
# Find Catalogizer processes
ps aux | grep catalogizer
pgrep -f catalogizer

# Kill processes
sudo pkill -f catalogizer
sudo kill -TERM $(pgrep catalogizer)
sudo kill -KILL $(pgrep catalogizer)

# Check process tree
pstree -p $(pgrep catalogizer)

# Monitor process resources
watch -n 1 'ps aux | grep catalogizer'
```

### Configuration Reload

```bash
# Graceful configuration reload
curl -X POST http://localhost:8080/api/config/reload
kill -HUP $(pgrep catalogizer)

# Full restart with new configuration
sudo systemctl restart catalogizer

# Validate configuration before reload
/opt/catalogizer/bin/catalogizer --validate-config
```

## Database Issues

### Connection Pool Management

```bash
# Monitor connection pool
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "
SELECT count(*) filter (where state = 'active') as active,
       count(*) filter (where state = 'idle') as idle,
       count(*) as total
FROM pg_stat_activity
WHERE usename = 'catalogizer_user';"

# Kill idle connections
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE state = 'idle'
  AND state_change < now() - interval '1 hour'
  AND usename = 'catalogizer_user';"
```

### Database Maintenance

```bash
# Vacuum and analyze tables
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "
VACUUM ANALYZE users;
VACUUM ANALYZE media_items;
VACUUM ANALYZE analytics_events;
VACUUM ANALYZE log_entries;"

# Check table sizes
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "
SELECT schemaname, tablename,
       pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) as size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;"

# Check index usage
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "
SELECT schemaname, tablename, indexname, idx_tup_read, idx_tup_fetch
FROM pg_stat_user_indexes
ORDER BY idx_tup_read DESC;"
```

### Database Performance Issues

```sql
-- Find slow queries
SELECT query, mean_time, calls, total_time
FROM pg_stat_statements
ORDER BY mean_time DESC
LIMIT 10;

-- Find blocking queries
SELECT blocked_locks.pid AS blocked_pid,
       blocked_activity.usename AS blocked_user,
       blocking_locks.pid AS blocking_pid,
       blocking_activity.usename AS blocking_user,
       blocked_activity.query AS blocked_statement,
       blocking_activity.query AS current_statement_in_blocking_process
FROM pg_catalog.pg_locks blocked_locks
JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
JOIN pg_catalog.pg_locks blocking_locks ON blocking_locks.locktype = blocked_locks.locktype
    AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
    AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
    AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
    AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
    AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
    AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
    AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
    AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
    AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
    AND blocking_locks.pid != blocked_locks.pid
JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
WHERE NOT blocked_locks.granted;

-- Check for table bloat
SELECT schemaname, tablename,
       round(100 * pg_relation_size(schemaname||'.'||tablename) / pg_total_relation_size(schemaname||'.'||tablename)) AS table_pct,
       round(100 * (pg_total_relation_size(schemaname||'.'||tablename) - pg_relation_size(schemaname||'.'||tablename)) / pg_total_relation_size(schemaname||'.'||tablename)) AS index_pct
FROM pg_tables
WHERE schemaname = 'public';
```

## Storage and Media Issues

### Disk Space Management

```bash
# Check disk usage
df -h /opt/catalogizer/media
du -sh /opt/catalogizer/media/*

# Find large files
find /opt/catalogizer/media -type f -size +100M -exec ls -lh {} \; | sort -k5 -hr

# Clean up temporary files
find /opt/catalogizer/media/temp -type f -mtime +7 -delete

# Check inode usage
df -i /opt/catalogizer/media
```

### Media File Issues

```bash
# Check for corrupted files
find /opt/catalogizer/media -type f -name "*.jpg" -exec jpeginfo -c {} \; | grep -v "OK"
find /opt/catalogizer/media -type f -name "*.png" -exec pngcheck {} \; | grep -v "OK"

# Fix permissions
sudo chown -R catalogizer:catalogizer /opt/catalogizer/media
find /opt/catalogizer/media -type f -exec chmod 644 {} \;
find /opt/catalogizer/media -type d -exec chmod 755 {} \;

# Rebuild media index
curl -X POST http://localhost:8080/api/admin/rebuild-index \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Check for orphaned files
curl -X GET http://localhost:8080/api/admin/orphaned-files \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

### Storage Configuration Issues

```bash
# Test storage connectivity (S3)
aws s3 ls s3://your-bucket-name/ --profile catalogizer

# Test WebDAV connectivity
curl -X PROPFIND http://webdav.example.com/catalogizer/ \
  -u username:password \
  -H "Depth: 1"

# Check local storage permissions
namei -l /opt/catalogizer/media/uploads/
```

## Performance Issues

### CPU Performance

```bash
# Monitor CPU usage
top -p $(pgrep catalogizer)
htop -p $(pgrep catalogizer)

# Check CPU-intensive processes
ps aux --sort=-%cpu | head -10

# Generate CPU profile
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof catalogizer cpu.prof

# Check system load
uptime
cat /proc/loadavg
```

### Memory Performance

```bash
# Check memory usage patterns
vmstat 1 10
sar -r 1 10

# Check for memory leaks
valgrind --tool=memcheck --leak-check=full /opt/catalogizer/bin/catalogizer

# Monitor garbage collection
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof catalogizer heap.prof
```

### I/O Performance

```bash
# Monitor disk I/O
iostat -x 1 5
iotop -o

# Check disk performance
dd if=/dev/zero of=/opt/catalogizer/test_file bs=1M count=1000 oflag=direct
rm /opt/catalogizer/test_file

# Monitor file descriptor usage
lsof -p $(pgrep catalogizer) | wc -l
cat /proc/$(pgrep catalogizer)/limits | grep "Max open files"
```

## Network and Connectivity

### Network Diagnostics

```bash
# Check listening ports
netstat -tulpn | grep catalogizer
ss -tulpn | grep catalogizer

# Test connectivity
curl -v http://localhost:8080/health
wget --spider -S http://localhost:8080/health

# Check DNS resolution
nslookup yourdomain.com
dig yourdomain.com

# Test external connectivity
curl -I https://api.github.com
```

### Load Balancer Issues

```bash
# Check Nginx status
sudo systemctl status nginx
sudo nginx -t

# Check Nginx logs
sudo tail -f /var/log/nginx/access.log
sudo tail -f /var/log/nginx/error.log

# Test backend connectivity
curl -H "Host: yourdomain.com" http://localhost:8080/health

# Check upstream status
curl http://localhost/nginx_status
```

### Firewall Issues

```bash
# Check firewall rules
sudo ufw status verbose
sudo iptables -L -n

# Test port accessibility
nc -zv localhost 8080
telnet localhost 8080

# Check SELinux (if applicable)
getenforce
sudo sealert -a /var/log/audit/audit.log
```

## Authentication and Authorization

### JWT Token Issues

```bash
# Validate JWT token
curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/user/profile

# Check token expiration
echo $TOKEN | cut -d. -f2 | base64 -d | jq .exp

# Generate new token
# The admin account is created at first start from ADMIN_USERNAME / ADMIN_PASSWORD
# (known placeholder passwords are refused). docker compose reads them from .env;
# a binary started directly needs them as real environment variables. There is no default login.
# Forgot the password? Stop the server, set ADMIN_NEW_PASSWORD, run
# `catalog-api admin-reset-password <username>` (the no-argument form only rotates
# admin accounts that still carry the published default password).
# The JSON body is built from the environment and piped in, so the password never
# appears on a command line (ps, shell history).
jq -n '{username: env.ADMIN_USERNAME, password: env.ADMIN_PASSWORD}' | \
  curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d @-
```

### User Access Issues

```bash
# Check user status
curl -X GET http://localhost:8080/api/admin/users/123 \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Reset user password
curl -X POST http://localhost:8080/api/admin/users/123/reset-password \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Check user permissions
curl -X GET http://localhost:8080/api/admin/users/123/permissions \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

## Configuration Problems

### Configuration Validation

```bash
# Validate configuration syntax
cat /opt/catalogizer/config/config.json | jq .

# Test configuration
/opt/catalogizer/bin/catalogizer --config /opt/catalogizer/config/config.json --validate

# Check environment variables
env | grep CATALOGIZER

# Compare with default configuration
diff /opt/catalogizer/config/config.json /opt/catalogizer/config/config.json.example
```

### Configuration Recovery

```bash
# Backup current configuration
cp /opt/catalogizer/config/config.json /opt/catalogizer/config/config.json.backup.$(date +%Y%m%d_%H%M%S)

# Restore from backup
curl -X GET http://localhost:8080/api/config/backups \
  -H "Authorization: Bearer $ADMIN_TOKEN"

curl -X POST http://localhost:8080/api/config/backups/123/restore \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# Reset to defaults
cp /opt/catalogizer/config/config.json.default /opt/catalogizer/config/config.json
```

## Log Analysis

### Log File Locations

```bash
# Application logs
/opt/catalogizer/logs/catalogizer.log
/opt/catalogizer/logs/error.log
/opt/catalogizer/logs/access.log

# System logs
/var/log/syslog
/var/log/messages
/var/log/daemon.log

# Service logs
sudo journalctl -u catalogizer
sudo journalctl -u nginx
sudo journalctl -u postgresql
```

### Log Analysis Commands

```bash
# Search for errors
grep -i error /opt/catalogizer/logs/catalogizer.log | tail -20

# Count error types
grep -i error /opt/catalogizer/logs/catalogizer.log | \
  awk -F'"' '{print $4}' | sort | uniq -c | sort -nr

# Analyze response times
awk '/response_time/ {
  sum += $NF; count++
} END {
  print "Average response time:", sum/count "ms"
}' /opt/catalogizer/logs/catalogizer.log

# Find slow requests
awk '/response_time/ && $NF > 1000 {print}' /opt/catalogizer/logs/catalogizer.log

# Check for memory issues
grep -i "out of memory\|memory\|gc" /opt/catalogizer/logs/catalogizer.log

# Monitor logs in real-time
tail -f /opt/catalogizer/logs/catalogizer.log | grep -i error
```

### Log Rotation Issues

```bash
# Check logrotate configuration
sudo cat /etc/logrotate.d/catalogizer

# Test logrotate
sudo logrotate -d /etc/logrotate.d/catalogizer
sudo logrotate -f /etc/logrotate.d/catalogizer

# Manual log rotation
sudo service catalogizer stop
sudo mv /opt/catalogizer/logs/catalogizer.log /opt/catalogizer/logs/catalogizer.log.$(date +%Y%m%d)
sudo service catalogizer start
```

## Debug Tools and Utilities

### Built-in Debug Endpoints

```bash
# Health check
curl http://localhost:8080/health

# Runtime statistics
curl http://localhost:8080/debug/vars | jq

# Memory profiling
curl http://localhost:8080/debug/pprof/heap > heap.prof
go tool pprof heap.prof

# CPU profiling
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof cpu.prof

# Goroutine analysis
curl http://localhost:8080/debug/pprof/goroutine > goroutine.prof
go tool pprof goroutine.prof

# Full goroutine dump
curl http://localhost:8080/debug/pprof/goroutine?debug=2
```

### System Debugging Tools

```bash
# Process monitoring
strace -p $(pgrep catalogizer)
ltrace -p $(pgrep catalogizer)

# Network monitoring
tcpdump -i any port 8080
netstat -continuous

# File system monitoring
inotify-tools
watch -n 1 'lsof -p $(pgrep catalogizer) | wc -l'

# System call analysis
perf record -p $(pgrep catalogizer) sleep 10
perf report
```

### Custom Debug Scripts

```bash
#!/bin/bash
# debug_collector.sh - Collect debugging information

DEBUG_DIR="/tmp/catalogizer_debug_$(date +%Y%m%d_%H%M%S)"
mkdir -p $DEBUG_DIR

echo "Collecting debug information..."

# System information
uname -a > $DEBUG_DIR/system_info.txt
uptime >> $DEBUG_DIR/system_info.txt
free -h >> $DEBUG_DIR/system_info.txt
df -h >> $DEBUG_DIR/system_info.txt

# Service status
systemctl status catalogizer > $DEBUG_DIR/service_status.txt

# Configuration
cp /opt/catalogizer/config/config.json $DEBUG_DIR/

# Logs
tail -1000 /opt/catalogizer/logs/catalogizer.log > $DEBUG_DIR/app_logs.txt
journalctl -u catalogizer --since "1 hour ago" > $DEBUG_DIR/systemd_logs.txt

# Network status
netstat -tulpn > $DEBUG_DIR/network_status.txt
ss -tulpn >> $DEBUG_DIR/network_status.txt

# Process information
ps aux | grep catalogizer > $DEBUG_DIR/process_info.txt
pmap $(pgrep catalogizer) > $DEBUG_DIR/memory_map.txt

# Create archive
tar -czf catalogizer_debug_$(date +%Y%m%d_%H%M%S).tar.gz -C /tmp $(basename $DEBUG_DIR)
echo "Debug information collected: catalogizer_debug_$(date +%Y%m%d_%H%M%S).tar.gz"
```

## Maintenance Procedures

### Regular Maintenance Tasks

#### Daily Tasks

```bash
#!/bin/bash
# daily_maintenance.sh

# Check service status
systemctl is-active catalogizer || echo "ALERT: Catalogizer service is down"

# Check disk space
DISK_USAGE=$(df /opt/catalogizer | tail -1 | awk '{print $5}' | sed 's/%//')
if [ $DISK_USAGE -gt 80 ]; then
    echo "WARNING: Disk usage is ${DISK_USAGE}%"
fi

# Check memory usage
MEMORY_USAGE=$(free | grep Mem | awk '{printf "%.0f", $3/$2 * 100.0}')
if [ $MEMORY_USAGE -gt 80 ]; then
    echo "WARNING: Memory usage is ${MEMORY_USAGE}%"
fi

# Check for errors in logs
ERROR_COUNT=$(grep -c "ERROR" /opt/catalogizer/logs/catalogizer.log)
if [ $ERROR_COUNT -gt 10 ]; then
    echo "WARNING: ${ERROR_COUNT} errors found in logs"
fi

# Check database connectivity
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "SELECT 1;" >/dev/null 2>&1 || echo "ALERT: Database connection failed"
```

#### Weekly Tasks

```bash
#!/bin/bash
# weekly_maintenance.sh

# Database maintenance
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "VACUUM ANALYZE;"

# Clean up old logs
find /opt/catalogizer/logs -name "*.log.*" -mtime +7 -delete

# Check certificate expiry
if [ -f /etc/ssl/certs/catalogizer.crt ]; then
    CERT_EXPIRY=$(openssl x509 -in /etc/ssl/certs/catalogizer.crt -noout -enddate | cut -d= -f2)
    CERT_EXPIRY_EPOCH=$(date -d "$CERT_EXPIRY" +%s)
    CURRENT_EPOCH=$(date +%s)
    DAYS_TO_EXPIRY=$(( ($CERT_EXPIRY_EPOCH - $CURRENT_EPOCH) / 86400 ))

    if [ $DAYS_TO_EXPIRY -lt 30 ]; then
        echo "WARNING: SSL certificate expires in $DAYS_TO_EXPIRY days"
    fi
fi

# Update system packages
sudo apt update && sudo apt list --upgradable
```

#### Monthly Tasks

```bash
#!/bin/bash
# monthly_maintenance.sh

# Full database backup
pg_dump -h $DB_HOST -U $DB_USER -d $DB_NAME | gzip > /opt/catalogizer/backups/monthly_backup_$(date +%Y%m).sql.gz

# System security updates
sudo apt update && sudo apt upgrade -y

# Clean up old backups
find /opt/catalogizer/backups -name "*.sql.gz" -mtime +90 -delete

# Performance analysis
curl http://localhost:8080/debug/pprof/profile?seconds=60 > /tmp/monthly_profile_$(date +%Y%m).prof

# Generate monthly report
echo "Monthly System Report - $(date +%Y-%m)" > /tmp/monthly_report.txt
echo "======================================" >> /tmp/monthly_report.txt
echo "Uptime: $(uptime)" >> /tmp/monthly_report.txt
echo "Disk Usage: $(df -h /opt/catalogizer)" >> /tmp/monthly_report.txt
echo "Memory Usage: $(free -h)" >> /tmp/monthly_report.txt
echo "Top Processes: $(ps aux --sort=-%cpu | head -5)" >> /tmp/monthly_report.txt
```

### Automated Monitoring

```bash
#!/bin/bash
# monitoring_script.sh

ALERT_EMAIL="admin@yourcompany.com"
LOG_FILE="/var/log/catalogizer_monitoring.log"

log_message() {
    echo "$(date): $1" >> $LOG_FILE
}

send_alert() {
    echo "$1" | mail -s "Catalogizer Alert" $ALERT_EMAIL
    log_message "ALERT: $1"
}

# Check service status
if ! systemctl is-active --quiet catalogizer; then
    send_alert "Catalogizer service is not running"
fi

# Check response time
RESPONSE_TIME=$(curl -o /dev/null -s -w '%{time_total}' http://localhost:8080/health)
if (( $(echo "$RESPONSE_TIME > 5.0" | bc -l) )); then
    send_alert "High response time: ${RESPONSE_TIME}s"
fi

# Check disk space
DISK_USAGE=$(df /opt/catalogizer | tail -1 | awk '{print $5}' | sed 's/%//')
if [ $DISK_USAGE -gt 90 ]; then
    send_alert "Critical disk usage: ${DISK_USAGE}%"
fi

# Check memory usage
MEMORY_USAGE=$(free | grep Mem | awk '{printf "%.0f", $3/$2 * 100.0}')
if [ $MEMORY_USAGE -gt 90 ]; then
    send_alert "Critical memory usage: ${MEMORY_USAGE}%"
fi

log_message "Monitoring check completed"
```

## Recovery Procedures

### Service Recovery

```bash
#!/bin/bash
# service_recovery.sh

echo "Starting Catalogizer service recovery..."

# Stop all related services
sudo systemctl stop catalogizer
sudo systemctl stop nginx

# Check for hung processes
if pgrep catalogizer; then
    echo "Killing hung processes..."
    sudo pkill -9 catalogizer
fi

# Check disk space
DISK_USAGE=$(df /opt/catalogizer | tail -1 | awk '{print $5}' | sed 's/%//')
if [ $DISK_USAGE -gt 95 ]; then
    echo "Critical disk space - cleaning logs..."
    sudo find /opt/catalogizer/logs -name "*.log.*" -delete
    sudo truncate -s 100M /opt/catalogizer/logs/catalogizer.log
fi

# Verify configuration
if ! /opt/catalogizer/bin/catalogizer --validate-config; then
    echo "Configuration invalid - restoring backup..."
    sudo cp /opt/catalogizer/config/config.json.backup /opt/catalogizer/config/config.json
fi

# Check database connectivity
if ! PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -d $DB_NAME -c "SELECT 1;" >/dev/null 2>&1; then
    echo "Database connection failed - checking database service..."
    sudo systemctl restart postgresql
    sleep 10
fi

# Start services
sudo systemctl start catalogizer
sleep 5

# Verify service is running
if systemctl is-active --quiet catalogizer; then
    echo "Service recovery successful"
    sudo systemctl start nginx
else
    echo "Service recovery failed - manual intervention required"
    exit 1
fi
```

### Database Recovery

```bash
#!/bin/bash
# database_recovery.sh

BACKUP_FILE=$1

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup_file>"
    exit 1
fi

echo "Starting database recovery from $BACKUP_FILE..."

# Stop application
sudo systemctl stop catalogizer

# Create recovery database
PGPASSWORD=$DB_PASSWORD createdb -h $DB_HOST -U $DB_USER catalogizer_recovery

# Restore from backup
if [[ $BACKUP_FILE == *.gz ]]; then
    gunzip -c $BACKUP_FILE | PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER catalogizer_recovery
else
    PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER catalogizer_recovery < $BACKUP_FILE
fi

# Verify restoration
ROW_COUNT=$(PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER catalogizer_recovery -t -c "SELECT count(*) FROM users;")
echo "Restored database contains $ROW_COUNT users"

# Swap databases
PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -c "
ALTER DATABASE catalogizer RENAME TO catalogizer_old;
ALTER DATABASE catalogizer_recovery RENAME TO catalogizer;
"

# Start application
sudo systemctl start catalogizer

# Test application
sleep 10
if curl -f http://localhost:8080/health; then
    echo "Database recovery successful"
    # Clean up old database after verification
    # PGPASSWORD=$DB_PASSWORD dropdb -h $DB_HOST -U $DB_USER catalogizer_old
else
    echo "Application failed to start - rolling back..."
    sudo systemctl stop catalogizer
    PGPASSWORD=$DB_PASSWORD psql -h $DB_HOST -U $DB_USER -c "
    ALTER DATABASE catalogizer RENAME TO catalogizer_failed;
    ALTER DATABASE catalogizer_old RENAME TO catalogizer;
    "
    sudo systemctl start catalogizer
fi
```

## Prevention and Monitoring

### Proactive Monitoring Setup

```bash
# Install monitoring tools
sudo apt install prometheus-node-exporter
sudo systemctl enable prometheus-node-exporter
sudo systemctl start prometheus-node-exporter

# Configure log monitoring
sudo apt install logwatch
echo "logwatch --detail high --mailto admin@yourcompany.com --service catalogizer" | sudo tee /etc/cron.daily/catalogizer-logwatch
```

### Health Check Monitoring

```bash
#!/bin/bash
# health_monitor.sh

ENDPOINTS=(
    "http://localhost:8080/health"
    "http://localhost:8080/api/health"
    "http://localhost:8080/metrics"
)

for endpoint in "${ENDPOINTS[@]}"; do
    if ! curl -f -s "$endpoint" >/dev/null; then
        echo "ALERT: $endpoint is not responding"
        # Send alert notification
    fi
done
```

### Preventive Measures

1. **Regular Backups**: Automated daily backups with verification
2. **Monitoring**: Comprehensive monitoring of all system metrics
3. **Updates**: Regular security updates and patches
4. **Testing**: Regular disaster recovery testing
5. **Documentation**: Keep troubleshooting procedures updated
6. **Training**: Ensure team is familiar with recovery procedures

### Alerting Configuration

```yaml
# alertmanager.yml
global:
  smtp_smarthost: 'localhost:587'
  smtp_from: 'alerts@yourcompany.com'

route:
  group_by: ['alertname']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'web.hook'

receivers:
- name: 'web.hook'
  email_configs:
  - to: 'admin@yourcompany.com'
    subject: 'Catalogizer Alert: {{ .GroupLabels.alertname }}'
    body: |
      {{ range .Alerts }}
      Alert: {{ .Annotations.summary }}
      Description: {{ .Annotations.description }}
      {{ end }}
```

## Goroutine Leak Detection and Resolution

### Symptoms

- Steadily increasing memory usage over time
- Prometheus metric `go_goroutines` climbing without plateau
- Application becomes unresponsive under load

### Detection

```bash
# Check current goroutine count via Prometheus
curl -s http://localhost:8080/metrics | grep go_goroutines

# Get a goroutine dump via pprof
curl -s http://localhost:8080/debug/pprof/goroutine?debug=1 > goroutines.txt
wc -l goroutines.txt

# Get a full goroutine profile for analysis
curl -s http://localhost:8080/debug/pprof/goroutine?debug=2 > goroutines_full.txt

# Analyze with go tool pprof
go tool pprof http://localhost:8080/debug/pprof/goroutine
```

### Common Causes in Catalogizer

1. **Unclosed WebSocket connections**: The WebSocket handler (`handlers/websocket_handler.go`) manages client connections. If a client disconnects abnormally, the cleanup goroutine may not fire.

```bash
# Check active WebSocket connections
curl -s http://localhost:8080/metrics | grep websocket
```

2. **Scanner goroutines not cleaned up**: The `UniversalScanner` spawns goroutines per storage root. If a scan hangs on an unreachable network share, goroutines accumulate.

```bash
# Check active scans
TOKEN="your_jwt_token"
curl -s -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/scans | jq '.[] | select(.status == "running")'
```

3. **CacheService goroutine leak**: The `CacheService` in `internal/services/cache_service.go` spawns background cleanup goroutines. The `Close()` method must be called during shutdown.

### Resolution

```bash
# Restart the service to clear leaked goroutines
sudo systemctl restart catalogizer

# For persistent issues, enable the Memory leak detection module
# (digital.vasic.memory) which provides runtime leak tracking
```

### Prevention

- The `CacheService.Close()` pattern is called in `main.go` deferred cleanup. Verify this is in place.
- Scanner goroutines use `context.WithCancel` for cancellation. Ensure all contexts are properly cancelled.
- The `ConcurrencyLimiter(100)` middleware caps concurrent request handling.
- The `Recovery` module (`digital.vasic.recovery`) provides circuit breaker patterns that prevent goroutine accumulation on failing network calls.

---

## Database Lock Issues

### SQLite WAL Mode Locks

SQLite in WAL (Write-Ahead Logging) mode allows concurrent reads but serializes writes. Lock contention is the most common SQLite issue in production.

**Symptoms:**
- "database is locked" errors in logs
- Write operations timing out
- Scan operations stalling

**Diagnosis:**

```bash
# Check WAL mode is active
sqlite3 catalog-api/data/catalogizer.db "PRAGMA journal_mode;"
# Should return: wal

# Check WAL file size (large WAL = checkpoint needed)
ls -lh catalog-api/data/catalogizer.db-wal

# Check busy timeout
sqlite3 catalog-api/data/catalogizer.db "PRAGMA busy_timeout;"
# Should return: 30000 (30 seconds, set in connection.go)
```

**Resolution:**

```bash
# Force a WAL checkpoint
sqlite3 catalog-api/data/catalogizer.db "PRAGMA wal_checkpoint(TRUNCATE);"

# If the database is stuck, stop the service and checkpoint
sudo systemctl stop catalogizer
sqlite3 catalog-api/data/catalogizer.db "PRAGMA wal_checkpoint(TRUNCATE);"
sudo systemctl start catalogizer
```

**Prevention:**
- The connection string in `database/connection.go` sets `_busy_timeout=30000` and `_journal_mode=WAL`
- WAL auto-checkpoint is configured: `_wal_autocheckpoint=1000`
- Explicit `PRAGMA journal_mode=WAL` is executed after connection because go-sqlcipher ignores connection string pragmas

### PostgreSQL Lock Issues

**Symptoms:**
- Queries hanging indefinitely
- "deadlock detected" errors
- Connection pool exhaustion

**Diagnosis:**

```sql
-- Find blocking queries
SELECT blocked_locks.pid AS blocked_pid,
       blocked_activity.usename AS blocked_user,
       blocking_locks.pid AS blocking_pid,
       blocking_activity.usename AS blocking_user,
       blocked_activity.query AS blocked_statement,
       blocking_activity.query AS blocking_statement
FROM pg_catalog.pg_locks blocked_locks
JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
JOIN pg_catalog.pg_locks blocking_locks
  ON blocking_locks.locktype = blocked_locks.locktype
  AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
  AND blocking_locks.pid != blocked_locks.pid
JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
WHERE NOT blocked_locks.granted;

-- Check for long-running transactions
SELECT pid, now() - xact_start AS duration, query, state
FROM pg_stat_activity
WHERE state != 'idle'
  AND xact_start IS NOT NULL
ORDER BY duration DESC;

-- Check connection pool usage
SELECT count(*) FILTER (WHERE state = 'active') AS active,
       count(*) FILTER (WHERE state = 'idle') AS idle,
       count(*) AS total
FROM pg_stat_activity
WHERE usename = 'catalogizer_user';
```

**Resolution:**

```sql
-- Kill a blocking query
SELECT pg_terminate_backend(<blocking_pid>);

-- Kill all idle connections older than 1 hour
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE state = 'idle'
  AND state_change < now() - interval '1 hour'
  AND usename = 'catalogizer_user';
```

**Prevention:**
- Connection pool defaults in `database/connection.go`: MaxOpen=25, MaxIdle=10, ConnMaxLifetime=5min
- The `RequestTimeout(60s)` middleware ensures queries do not run indefinitely
- Use the `config.json` `write_timeout` setting (should be 900 for long-running challenge RunAll)

---

## WebSocket Connection Problems

### Symptoms

- Frontend shows "Disconnected" status
- Real-time scan progress not updating
- Browser console shows WebSocket errors

### Diagnosis

```bash
# Test WebSocket connectivity
# Use websocat or wscat
wscat -c ws://localhost:8080/ws

# Check if the WebSocket handler is registered
curl -s http://localhost:8080/metrics | grep websocket

# Check for proxy issues (nginx must be configured for WebSocket)
grep -i "upgrade" /etc/nginx/conf.d/catalogizer.conf
```

### Common Causes

1. **Nginx proxy not configured for WebSocket upgrade:**

```nginx
# Required nginx configuration for WebSocket
location /ws {
    proxy_pass http://localhost:8080/ws;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_read_timeout 86400;
}
```

2. **Authentication via query parameter**: The WebSocket endpoint authenticates via query parameter, not the `Authorization` header. Ensure the frontend passes `?token=<jwt>` in the WebSocket URL.

3. **Connection timeout**: The WebSocket connection may be closed by intermediate proxies. Configure `proxy_read_timeout` in nginx to a high value (86400 seconds = 24 hours).

4. **HTTP/3 (QUIC) and WebSocket**: WebSocket over HTTP/3 uses WebTransport. If the client does not support WebTransport, it falls back to HTTP/2 or HTTP/1.1 for WebSocket connections.

### Resolution

```bash
# Restart the service to reset all WebSocket connections
sudo systemctl restart catalogizer

# Clear browser state (frontend reconnects automatically)
# The @vasic-digital/websocket-client package handles reconnection with backoff
```

---

## Cache Invalidation Issues

### Symptoms

- Stale data shown in the UI after updates
- Media entity metadata not reflecting recent scans
- Statistics showing outdated counts

### Diagnosis

```bash
# Check Redis connectivity (if Redis is configured)
redis-cli ping
redis-cli info memory

# Check cache headers on API responses
curl -I -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/entities/stats
# Look for Cache-Control headers

# Check entity cache age
curl -I -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/stats/overall
# Statistics are cached for 60 seconds (CacheHeaders middleware)
# Entity browsing is cached for 300 seconds (5 minutes)
```

### Common Causes

1. **Browser cache**: The frontend may serve cached responses. Entity endpoints have 5-minute cache headers; statistics have 1-minute cache.

2. **Redis stale data**: If Redis is enabled, cached data may persist after database changes.

3. **CDN or proxy cache**: Intermediate caches may hold stale responses.

### Resolution

```bash
# Clear Redis cache
redis-cli FLUSHDB

# Force cache bypass from the client
# Add Cache-Control: no-cache header to requests

# Clear browser cache
# Ctrl+Shift+Delete in the browser, or hard refresh with Ctrl+Shift+R

# For persistent issues, restart the service
sudo systemctl restart catalogizer
```

### Prevention

- The `CacheHeaders` middleware sets appropriate `Cache-Control` headers
- The `StaticCacheHeaders` middleware is only used for asset serving
- WebSocket events broadcast changes in real-time, triggering React Query invalidation in the frontend
- The asset event bus (`event.AssetReady`, `event.AssetFailed`) pushes updates to WebSocket clients

---

## Container Networking Issues

### Symptoms

- Backend cannot reach PostgreSQL or Redis in containers
- SSL errors during container builds
- DNS resolution failures inside containers
- `crun exec.fifo` errors with podman-compose

### Diagnosis

```bash
# Check container status
podman ps -a

# Check container logs
podman logs catalogizer-api
podman logs catalogizer-db

# Check container networking
podman network ls
podman inspect catalogizer-api | jq '.[0].NetworkSettings'

# Test connectivity from inside a container
podman exec catalogizer-api ping -c 1 catalogizer-db
podman exec catalogizer-api curl -s http://localhost:8080/health
```

### Common Causes and Solutions

1. **SSL errors during build**: Podman's default container networking has issues with some SSL endpoints (dl.google.com, crates.io). Always use `--network host` for builds:

```bash
podman build --network host -t catalogizer-api .
```

2. **crun exec.fifo errors**: Running PostgreSQL and Redis via podman-compose can cause `crun exec.fifo` errors. Use `podman run --network host` instead of compose for builds:

```bash
podman run --network host --cpus=2 --memory=4g catalogizer-api
```

3. **Short image names failing**: Podman without a TTY cannot prompt for registry selection. Use fully qualified names:

```bash
# Wrong:
podman pull golang:1.24

# Correct:
podman pull docker.io/library/golang:1.24
```

4. **Container resource limits**: The host has a 30-40% resource budget. Exceeding limits freezes the system:

```bash
# Check container resource usage
podman stats --no-stream

# Recommended limits:
# PostgreSQL: --cpus=1 --memory=2g
# API: --cpus=2 --memory=4g
# Web: --cpus=1 --memory=2g
# Builder: --cpus=3 --memory=8g
# Total budget: max 4 CPUs, 8 GB RAM across all containers
```

5. **NAS access from containers**: The API container needs explicit host mapping for NAS devices:

```bash
podman run --add-host=synology.local:192.168.0.241 catalogizer-api
```

6. **GOTOOLCHAIN auto-download**: Inside containers, Go may try to download a newer toolchain, causing build failures. Set:

```bash
export GOTOOLCHAIN=local
```

7. **Tauri AppImage in containers**: Containers lack FUSE support. Set:

```bash
export APPIMAGE_EXTRACT_AND_RUN=1
```

---

For additional help with specific issues not covered in this guide, please:

1. Check the [Configuration Guide](CONFIGURATION_GUIDE.md) for setup issues
2. Review the [Deployment Guide](DEPLOYMENT_GUIDE.md) for infrastructure problems
3. Consult the [API Documentation](api/API_DOCUMENTATION.md) for integration issues
4. See the [SQL Migrations](architecture/SQL_MIGRATIONS.md) guide for database migration issues
5. Review the [Monitoring Guide](deployment/MONITORING_GUIDE.md) for metrics and alerting
6. Contact support with the debug information collected using the scripts in this guide