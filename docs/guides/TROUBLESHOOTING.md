# Catalogizer Troubleshooting Guide

This guide helps you diagnose and resolve common issues with the Catalogizer system, including multi-protocol storage resilience, database problems, and performance issues.

## Table of Contents

- [Storage Connection Issues](#storage-connection-issues)
- [Database Problems](#database-problems)
- [Authentication Issues](#authentication-issues)
- [Performance Problems](#performance-problems)
- [Frontend Issues](#frontend-issues)
- [API Errors](#api-errors)
- [Deployment Issues](#deployment-issues)
- [Monitoring & Diagnostics](#monitoring--diagnostics)
- [WebSocket Connection Failures](#websocket-connection-failures)
- [SMB Authentication Issues](#smb-authentication-issues)
- [Subtitle Search Issues](#subtitle-search-issues)
- [Format Conversion Errors](#format-conversion-errors)
- [Android Connectivity Issues](#android-connectivity-issues)

## Storage Connection Issues

### Symptoms
- Media files not being detected
- "Storage source offline" notifications
- Connection timeouts in logs
- Circuit breaker activation

### Diagnostic Steps

#### 1. Check Storage Source Status
```bash
# API endpoint to check all storage sources
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/storage/sources/status
```

Expected response for healthy sources:
```json
{
  "sources": {
    "smb_123": {
      "name": "Media Server 1",
      "protocol": "smb",
      "path": "smb://server1/media",
      "state": "connected",
      "last_connected": "2024-01-15T10:30:00Z",
      "retry_attempts": 0,
      "is_enabled": true
    },
    "ftp_456": {
      "name": "FTP Server",
      "protocol": "ftp",
      "path": "ftp://ftp.example.com/media",
      "state": "connected",
      "last_connected": "2024-01-15T10:25:00Z",
      "retry_attempts": 0,
      "is_enabled": true
    }
  },
  "summary": {
    "total": 3,
    "connected": 3,
    "disconnected": 0,
    "offline": 0
  }
}
```

#### 2. Test SMB Connectivity Manually
```bash
# Test basic SMB connection
smbclient -L //your-server -U your-username

# Test file listing
smbclient //your-server/share -U your-username -c "ls"

# Test with different authentication
smbclient //your-server/share -U domain/username%password -c "ls"
```

#### 3. Check Network Connectivity
```bash
# Test network connectivity to SMB server
ping your-server

# Test specific SMB ports
nc -zv your-server 445  # SMB over TCP
nc -zv your-server 139  # NetBIOS

# Check firewall rules
sudo iptables -L | grep 445
```

### Common Solutions

#### 1. Authentication Issues
```env
# Update credentials in .env file
SMB_USERNAME=correct-username
SMB_PASSWORD=correct-password
SMB_DOMAIN=your-domain

# For workgroup environments, try without domain
SMB_DOMAIN=
```

#### 2. Network/Firewall Issues
```bash
# Allow SMB ports through firewall
sudo ufw allow 445
sudo ufw allow 139

# Check if SMB services are running on server
sudo systemctl status smbd
sudo systemctl status nmbd
```

#### 3. Circuit Breaker Reset
```bash
# Force reconnection via API
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/smb/sources/smb_123/reconnect
```

#### 4. SMB Protocol Version Issues
```bash
# Try different SMB protocol versions in smb.conf
echo "client min protocol = SMB2" >> /etc/samba/smb.conf
echo "client max protocol = SMB3" >> /etc/samba/smb.conf
```

### SMB Resilience Features

#### Offline Cache Behavior
When SMB sources become unavailable:
1. System switches to offline mode
2. Cached metadata serves user requests
3. Background reconnection attempts continue
4. Once reconnected, cached changes are synchronized

#### Circuit Breaker States
- **Closed**: Normal operation, all requests pass through
- **Open**: Too many failures, requests fail fast
- **Half-Open**: Testing if service has recovered

#### Recovery Process
1. Detect connection failure
2. Activate circuit breaker
3. Enable offline cache
4. Start exponential backoff retry
5. Test connection in half-open state
6. Resume normal operation when stable

## Database Problems

### Symptoms
- "Database locked" errors
- Slow query performance
- Connection pool exhausted
- Data corruption warnings

### Diagnostic Steps

#### 1. Check Database Health
```bash
# Test basic connectivity
sqlite3 /var/lib/catalogizer/catalogizer.db ".tables"

# Check database integrity
sqlite3 /var/lib/catalogizer/catalogizer.db "PRAGMA integrity_check;"

# Check database size and page count
sqlite3 /var/lib/catalogizer/catalogizer.db ".schema" | head -20
```

#### 2. Monitor Database Performance
```sql
-- Check slow queries
EXPLAIN QUERY PLAN SELECT * FROM media_items WHERE title LIKE '%matrix%';

-- Check index usage
.indices media_items

-- Analyze database statistics
ANALYZE;
```

### Common Solutions

#### 1. Database Locks
```bash
# Kill any processes holding locks
fuser /var/lib/catalogizer/catalogizer.db

# Check for long-running transactions
sqlite3 /var/lib/catalogizer/catalogizer.db "PRAGMA busy_timeout = 30000;"
```

#### 2. Performance Optimization
```sql
-- Rebuild indexes
REINDEX;

-- Update statistics
ANALYZE;

-- Vacuum database to reclaim space
VACUUM;
```

#### 3. Connection Pool Issues
```go
// Adjust connection pool settings in code
db.SetMaxOpenConns(25)
db.SetMaxIdleConns(5)
db.SetConnMaxLifetime(5 * time.Minute)
```

#### 4. Backup and Recovery
```bash
# Create backup before fixes
sqlite3 /var/lib/catalogizer/catalogizer.db ".backup backup.db"

# Restore from backup if needed
cp backup.db /var/lib/catalogizer/catalogizer.db
```

## Authentication Issues

### Symptoms
- "Invalid or expired token" errors
- Login failures with correct credentials
- Permission denied errors
- Session timeouts

### Diagnostic Steps

#### 1. Check JWT Token
```bash
# Decode JWT token (without verification)
echo "your-jwt-token" | cut -d. -f2 | base64 -d | jq .

# Check token expiration
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/auth/status
```

#### 2. Verify User Credentials
```bash
# Check user exists in database
sqlite3 /var/lib/catalogizer/catalogizer.db \
  "SELECT username, role, is_active FROM users WHERE username = 'admin';"
```

### Common Solutions

#### 1. Token Refresh
```bash
# Use refresh token to get new access token
curl -X POST http://localhost:8080/api/v1/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{"refresh_token": "your-refresh-token"}'
```

#### 2. Password Reset
```go
// Reset admin password via CLI tool
go run cmd/admin/main.go reset-password admin newpassword123
```

#### 3. Permission Issues
```sql
-- Check user permissions
SELECT u.username, u.role, GROUP_CONCAT(p.name) as permissions
FROM users u
LEFT JOIN user_permissions up ON u.id = up.user_id
LEFT JOIN permissions p ON up.permission_id = p.id
WHERE u.username = 'your-username';
```

## Performance Problems

### Symptoms
- Slow page loading
- High CPU/memory usage
- Database query timeouts
- Unresponsive UI

### Diagnostic Steps

#### 1. Monitor System Resources
```bash
# Check CPU and memory usage
top -p $(pgrep catalogizer)

# Monitor I/O usage
iotop

# Check disk space
df -h /var/lib/catalogizer
```

#### 2. Profile Application Performance
```bash
# Enable Go pprof endpoint
curl http://localhost:8080/debug/pprof/profile?seconds=30 > cpu.prof

# Analyze memory usage
curl http://localhost:8080/debug/pprof/heap > heap.prof

# View profiles
go tool pprof cpu.prof
go tool pprof heap.prof
```

#### 3. Database Performance Analysis
```sql
-- Find slow queries
.timer on
SELECT COUNT(*) FROM media_items;

-- Check query plans
EXPLAIN QUERY PLAN
SELECT * FROM media_items
WHERE media_type = 'movie'
ORDER BY updated_at DESC
LIMIT 20;
```

### Common Solutions

#### 1. Database Optimization
```sql
-- Add missing indexes
CREATE INDEX IF NOT EXISTS idx_media_type_updated
ON media_items(media_type, updated_at DESC);

-- Optimize queries
CREATE INDEX IF NOT EXISTS idx_media_search_fts
ON media_items(title, description);
```

#### 2. Caching Improvements
```go
// Implement query result caching
cache := make(map[string]interface{})
if result, exists := cache[key]; exists {
    return result
}
```

#### 3. Resource Limits
```bash
# Increase file descriptor limits
echo "catalogizer soft nofile 65536" >> /etc/security/limits.conf
echo "catalogizer hard nofile 65536" >> /etc/security/limits.conf

# Optimize Go garbage collector
export GOGC=100
export GOMEMLIMIT=2GiB
```

## Frontend Issues

### Symptoms
- White screen on load
- API call failures
- WebSocket disconnections
- UI components not updating

### Diagnostic Steps

#### 1. Check Browser Console
```javascript
// Check for JavaScript errors
console.log("Catalogizer debug info");

// Check API connectivity
fetch('/api/v1/health')
  .then(r => r.json())
  .then(console.log)
  .catch(console.error);
```

#### 2. Network Issues
```bash
# Check if API is accessible
curl http://localhost:8080/api/v1/health

# Test WebSocket connection
wscat -c ws://localhost:8080/ws
```

### Common Solutions

#### 1. API Configuration
```env
# Check environment variables
VITE_API_BASE_URL=http://localhost:8080
VITE_WS_URL=ws://localhost:8080/ws
```

#### 2. CORS Issues
```go
// Add CORS middleware in Go API
func CORSMiddleware() gin.HandlerFunc {
    return gin.HandlerFunc(func(c *gin.Context) {
        c.Header("Access-Control-Allow-Origin", "*")
        c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
        c.Header("Access-Control-Allow-Headers", "Authorization,Content-Type")

        if c.Request.Method == "OPTIONS" {
            c.AbortWithStatus(204)
            return
        }

        c.Next()
    })
}
```

#### 3. Build Issues
```bash
# Clear build cache
rm -rf node_modules/.vite
npm run build

# Check for dependency conflicts
npm audit fix
```

## API Errors

### Common Error Codes and Solutions

#### 401 Unauthorized
```json
{"error": "Invalid or expired token"}
```

**Solution:**
```bash
# Refresh token or re-login
# The admin account is created at first start from ADMIN_USERNAME / ADMIN_PASSWORD
# (set both in .env; known placeholder passwords are refused). There is no default login.
# Forgot it? Stop the server, set ADMIN_NEW_PASSWORD, run `catalog-api admin-reset-password`.
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d "{\"username\": \"$ADMIN_USERNAME\", \"password\": \"$ADMIN_PASSWORD\"}"
```

#### 403 Forbidden
```json
{"error": "Insufficient permissions"}
```

**Solution:**
```sql
-- Check user permissions
SELECT role, permissions FROM users WHERE username = 'your-user';

-- Update user role if needed
UPDATE users SET role = 'admin' WHERE username = 'your-user';
```

#### 500 Internal Server Error
```json
{"error": "Internal server error"}
```

**Solution:**
```bash
# Check server logs
journalctl -u catalogizer-api -f

# Check disk space
df -h

# Restart service if needed
sudo systemctl restart catalogizer-api
```

#### 503 Service Unavailable
```json
{"error": "Circuit breaker is open"}
```

**Solution:**
```bash
# Check service health
curl http://localhost:8080/health

# Reset circuit breaker
curl -X POST http://localhost:8080/api/v1/system/circuit-breaker/reset
```

## Deployment Issues

### Docker Deployment Problems

#### Container Won't Start
```bash
# Check container logs
docker logs catalogizer-api

# Check container status
docker ps -a

# Inspect container configuration
docker inspect catalogizer-api
```

#### Volume Mount Issues
```bash
# Check volume permissions
ls -la /path/to/volume

# Fix permissions
sudo chown -R 1000:1000 /path/to/volume
```

### Kubernetes Deployment Problems

#### Pod Crashes
```bash
# Check pod status
kubectl get pods -n catalogizer

# View pod logs
kubectl logs -f deployment/catalogizer-api -n catalogizer

# Describe pod for events
kubectl describe pod <pod-name> -n catalogizer
```

#### Service Discovery Issues
```bash
# Test service connectivity
kubectl exec -it <pod-name> -n catalogizer -- curl catalogizer-api:8080/health

# Check service endpoints
kubectl get endpoints -n catalogizer
```

## Monitoring & Diagnostics

### Health Check Endpoints

```bash
# Basic health check
curl http://localhost:8080/health

# Detailed status
curl http://localhost:8080/api/v1/status

# SMB sources health
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/smb/health

# Database health
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/db/health
```

### Log Analysis

#### Application Logs
```bash
# Follow application logs
tail -f /opt/catalogizer/logs/catalogizer.log

# Search for errors
grep -i error /opt/catalogizer/logs/catalogizer.log

# Filter by level
grep -i "level=error" /opt/catalogizer/logs/catalogizer.log
```

#### System Logs
```bash
# Service logs
journalctl -u catalogizer-api -f

# System messages
tail -f /var/log/syslog | grep catalogizer

# Nginx logs
tail -f /var/log/nginx/access.log
tail -f /var/log/nginx/error.log
```

### Performance Monitoring

#### Metrics Collection
```bash
# CPU and memory usage
ps aux | grep catalogizer

# Network connections
netstat -tulpn | grep :8080

# File descriptors
lsof -p $(pgrep catalogizer) | wc -l
```

#### Database Monitoring
```sql
-- Active connections
.show

-- Table sizes
SELECT name, COUNT(*) FROM sqlite_master
WHERE type='table'
GROUP BY name;

-- Index usage statistics
PRAGMA index_list('media_items');
```

### Emergency Recovery

#### Service Recovery
```bash
# Quick restart
sudo systemctl restart catalogizer-api

# Full system restart
sudo systemctl stop catalogizer-api
sudo systemctl start catalogizer-api

# Reset to defaults
rm /var/lib/catalogizer/catalogizer.db
go run cmd/migrate/main.go
```

#### Data Recovery
```bash
# Restore from backup
sudo systemctl stop catalogizer-api
cp /opt/catalogizer/backup/latest.db /var/lib/catalogizer/catalogizer.db
sudo systemctl start catalogizer-api

# Emergency admin user creation
go run cmd/admin/main.go create-admin emergency-admin secure-password-123
```

## Getting Help

### Debug Information Collection

When reporting issues, please collect the following information:

```bash
#!/bin/bash
# debug-info.sh - Collect debug information

echo "=== Catalogizer Debug Information ===" > debug-info.txt
echo "Date: $(date)" >> debug-info.txt
echo "Hostname: $(hostname)" >> debug-info.txt
echo "" >> debug-info.txt

echo "=== System Information ===" >> debug-info.txt
uname -a >> debug-info.txt
cat /etc/os-release >> debug-info.txt
echo "" >> debug-info.txt

echo "=== Service Status ===" >> debug-info.txt
systemctl status catalogizer-api >> debug-info.txt
echo "" >> debug-info.txt

echo "=== Recent Logs ===" >> debug-info.txt
tail -100 /opt/catalogizer/logs/catalogizer.log >> debug-info.txt
echo "" >> debug-info.txt

echo "=== SMB Status ===" >> debug-info.txt
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/smb/sources/status >> debug-info.txt
echo "" >> debug-info.txt

echo "=== Database Info ===" >> debug-info.txt
sqlite3 /var/lib/catalogizer/catalogizer.db ".schema" | head -50 >> debug-info.txt
echo "" >> debug-info.txt

echo "Debug information collected in debug-info.txt"
```

### Support Channels

- **GitHub Issues**: Report bugs and feature requests
- **Documentation**: Check README.md and ARCHITECTURE.md
- **Community**: Discord/Slack for community support
- **Enterprise**: Professional support options

### Useful Resources

- [Architecture Documentation](../architecture/ARCHITECTURE.md)
- [Deployment Guide](../deployment/DEPLOYMENT.md)

This troubleshooting guide covers the most common issues you may encounter. For specific problems not covered here, please check the logs and consider opening a GitHub issue with the debug information collected using the script above.

---

## WebSocket Connection Failures

WebSocket connections are used for real-time updates between the frontend and the catalog-api server. When WebSocket connections fail, you will see stale data or missing live updates in the web and desktop apps.

### Symptoms

- Real-time notifications not appearing
- Activity feed not updating
- "WebSocket disconnected" warnings in browser console
- Collection real-time collaboration not working

### Diagnostic Steps

```bash
# Test WebSocket connectivity
wscat -c ws://localhost:8080/ws

# Check if the WebSocket endpoint is accessible
curl -i -N -H "Connection: Upgrade" -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" \
  -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
  http://localhost:8080/ws

# Check if the API server is accepting WebSocket upgrades
curl -v http://localhost:8080/ws 2>&1 | grep -i upgrade
```

### Common Solutions

#### 1. Reverse Proxy Configuration

If you are using Nginx or another reverse proxy, ensure WebSocket upgrades are forwarded:

```nginx
# Nginx configuration for WebSocket support
location /ws {
    proxy_pass http://localhost:8080;
    proxy_http_version 1.1;
    proxy_set_header Upgrade $http_upgrade;
    proxy_set_header Connection "upgrade";
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_read_timeout 86400;  # Keep connection alive for 24 hours
}
```

For Apache:

```apache
# Enable required modules
LoadModule proxy_wstunnel_module modules/mod_proxy_wstunnel.so

ProxyPass /ws ws://localhost:8080/ws
ProxyPassReverse /ws ws://localhost:8080/ws
```

#### 2. Environment Variable Configuration

Ensure the frontend WebSocket URL matches the server:

```env
# In catalog-web .env
VITE_WS_URL=ws://localhost:8080/ws

# For HTTPS deployments
VITE_WS_URL=wss://catalogizer.example.com/ws
```

#### 3. Firewall and Network Issues

```bash
# Check if the WebSocket port is open
nc -zv localhost 8080

# On cloud deployments, ensure security groups allow WebSocket traffic
# WebSocket uses the same port as HTTP (typically 80 or 443)
```

#### 4. Connection Timeout Adjustments

If WebSocket connections drop frequently, adjust timeout settings:

```bash
# Increase proxy timeout in Nginx
proxy_read_timeout 3600;
proxy_send_timeout 3600;
```

The web app includes automatic reconnection logic with exponential backoff, but persistent failures indicate a configuration issue.

#### 5. CORS Issues

If the web app is served from a different origin than the API:

```go
// Ensure CORS middleware allows WebSocket origins
c.Header("Access-Control-Allow-Origin", "http://localhost:5173")
```

---

## SMB Authentication Issues

SMB (Server Message Block) is a primary storage protocol for Catalogizer. Authentication failures prevent media detection and browsing.

### Symptoms

- "Storage source offline" notifications
- "Access denied" or "Authentication failed" in server logs
- Circuit breaker activating for SMB sources
- Media files not being detected from network shares

### Diagnostic Steps

```bash
# Test SMB credentials manually
smbclient -L //server-hostname -U username

# Test access to a specific share
smbclient //server-hostname/share-name -U username -c "ls"

# Test with domain credentials
smbclient //server-hostname/share-name -U domain/username%password -c "ls"

# Check SMB port connectivity
nc -zv server-hostname 445
nc -zv server-hostname 139
```

### Common Solutions

#### 1. Credential Format Issues

Different SMB servers expect credentials in different formats:

```env
# Standard format
SMB_USERNAME=username
SMB_PASSWORD=password

# Domain format (Active Directory)
SMB_USERNAME=DOMAIN\username
SMB_PASSWORD=password
SMB_DOMAIN=DOMAIN

# UPN format
SMB_USERNAME=username@domain.com
SMB_PASSWORD=password
```

#### 2. SMB Protocol Version Mismatch

Modern servers may require SMB2 or SMB3:

```bash
# Check supported protocol versions
smbclient -L //server --option="client min protocol=SMB2" -U username

# Configure minimum protocol version
echo "client min protocol = SMB2" >> /etc/samba/smb.conf
echo "client max protocol = SMB3" >> /etc/samba/smb.conf
```

#### 3. Guest Authentication Disabled

Some NAS devices disable guest access by default:

```bash
# If guest access is needed
smbclient //server/share -N  # Test with no password

# If this fails, verify the share allows guest access on the server side
```

#### 4. Expired or Changed Passwords

When SMB passwords change:

1. Update credentials in the Catalogizer configuration.
2. Restart the catalog-api server or trigger a reconnection:

```bash
curl -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/smb/sources/{source_id}/reconnect
```

3. Monitor the circuit breaker state -- it should transition from Open to Half-Open to Closed.

#### 5. Permissions on the Share

Even with valid credentials, access may be denied due to share permissions:

- Ensure the user has read access to the share and its subdirectories.
- On Windows, check both Share Permissions and NTFS Permissions.
- On Linux Samba servers, check the `valid users` and `read list` directives in `smb.conf`.

---

## Subtitle Search Issues

The subtitle system integrates with external providers (OpenSubtitles, Subscene, etc.) and can encounter various issues.

### Symptoms

- "No subtitles found" for media that should have subtitles available
- Subtitle search returning errors or timing out
- Downloaded subtitles not syncing properly with media
- Subtitle upload failing

### Diagnostic Steps

```bash
# Test subtitle API endpoint
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/subtitles/search?query=movie-title&language=en

# Check supported providers
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/subtitles/providers

# Check supported languages
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/subtitles/languages
```

### Common Solutions

#### 1. Provider API Keys

Some subtitle providers require API keys:

```env
# Configure provider API keys
OPENSUBTITLES_API_KEY=your-api-key
OPENSUBTITLES_USERNAME=your-username
OPENSUBTITLES_PASSWORD=your-password
```

#### 2. Rate Limiting

External subtitle providers enforce rate limits:

- OpenSubtitles: typically 40 requests per 10 seconds for authenticated users.
- If you see HTTP 429 errors, wait before retrying.
- The system should respect rate limits automatically, but high concurrent usage may trigger limits.

#### 3. Search Query Too Specific

- Try broadening the search query (use the movie title without year).
- Remove special characters from the query.
- Try different language codes.

#### 4. Subtitle Sync Offset

If downloaded subtitles are out of sync:

1. In the web app Subtitle Manager, select the media item.
2. Click the **Verify Sync** button for the subtitle.
3. Use the Subtitle Sync Modal to adjust the timing offset (in milliseconds).
4. Positive values delay the subtitles; negative values advance them.

#### 5. Encoding Issues

If subtitles display garbled characters:

- Check the subtitle encoding (shown in the subtitle details: e.g. UTF-8, ISO-8859-1).
- The system attempts automatic encoding detection, but some edge cases may require manual re-encoding.
- Try downloading a different subtitle version from the search results.

---

## Format Conversion Errors

The format conversion system transforms media files between different formats (MP4, MKV, AVI, MOV, WebM, MP3, WAV, FLAC).

### Symptoms

- Conversion jobs stuck in "pending" status
- Jobs failing with error messages
- Converted files having no audio or video
- Conversion progress not updating

### Diagnostic Steps

```bash
# Check conversion job status
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/conversion/jobs

# Check a specific job
curl -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/conversion/jobs/{job_id}

# Verify FFmpeg is installed (required for conversion)
ffmpeg -version

# Check disk space (conversions need temporary space)
df -h
```

### Common Solutions

#### 1. FFmpeg Not Installed or Not in PATH

The conversion system requires FFmpeg:

```bash
# Install on Ubuntu/Debian
sudo apt install ffmpeg

# Install on macOS
brew install ffmpeg

# Install on Alpine (Docker)
apk add ffmpeg

# Verify installation
which ffmpeg
ffmpeg -version
```

#### 2. Insufficient Disk Space

Conversions require temporary disk space, often 1-3x the size of the source file:

```bash
# Check available space
df -h /tmp
df -h /var/lib/catalogizer

# Clean up old conversion artifacts
rm -rf /tmp/catalogizer-conversion-*
```

#### 3. Unsupported Codec

Some source files may use codecs not supported by your FFmpeg build:

```bash
# List supported codecs
ffmpeg -codecs

# List supported formats
ffmpeg -formats
```

If a codec is missing, install a full-featured FFmpeg build:

```bash
# Ubuntu - install with all codecs
sudo apt install ffmpeg libavcodec-extra
```

#### 4. Conversion Timeout

Very large files may exceed the conversion timeout:

- Check server logs for timeout errors.
- For files over 10 GB, consider adjusting the conversion timeout in the server configuration.
- Lower quality settings can speed up conversion.

#### 5. Job Queue Stalled

If jobs remain in "pending" state:

```bash
# Restart the conversion worker
sudo systemctl restart catalogizer-api

# Or check if the worker process is alive
ps aux | grep catalogizer
```

---

## Android Connectivity Issues

Android apps (both mobile and TV) connect to the Catalogizer server over the network. Various connectivity issues can prevent the apps from functioning.

### Symptoms

- Login fails with "Connection failed"
- Media list not loading
- Offline mode activating unexpectedly
- Sync operations failing

### Diagnostic Steps

On the Android device:

1. Open a browser and try navigating to your Catalogizer server URL.
2. Check Wi-Fi or cellular connection status.
3. In the app, check the Settings screen for any error messages.

### Common Solutions

#### 1. Server URL Issues

- Ensure the URL includes the protocol: `http://` or `https://`.
- If using a local IP address (e.g. `192.168.1.100`), ensure the Android device is on the same network.
- Do not use `localhost` -- this refers to the Android device itself, not your server.
- Try the IP address instead of a hostname if DNS resolution is unreliable.

#### 2. HTTPS Certificate Issues

If using self-signed certificates:

- Android rejects self-signed certificates by default.
- Install your CA certificate on the Android device: Settings > Security > Install from storage.
- For development, consider using HTTP instead of HTTPS.

#### 3. Cellular vs. Wi-Fi

- If the server is on a private network, cellular connections cannot reach it.
- Check if "Wi-Fi Only" is enabled in the app's offline settings.
- If using a VPN, ensure it is connected and routing traffic to your server's network.

#### 4. Background Sync Failures

Android's battery optimization may kill background sync processes:

- Go to device Settings > Apps > Catalogizer > Battery > Unrestricted.
- This allows the sync worker to run in the background without being killed.

#### 5. Token Expiration

JWT tokens have an expiration time:

- If the app suddenly cannot connect, try logging out and back in.
- The app should automatically refresh tokens, but edge cases (device clock skew, server restart) can cause token issues.
- Ensure your Android device's clock is accurate (Settings > Date & time > Automatic date & time).

#### 6. Large Library Loading

If you have a very large media library:

- The initial load may take longer on mobile connections.
- Enable offline mode to cache data locally for faster subsequent access.
- The app paginates requests, but the first page still requires a round trip.

#### 7. Android TV Specific Issues

- Ensure your TV has a stable Ethernet or Wi-Fi connection.
- Some Android TV devices have limited memory -- close other apps if Catalogizer crashes.
- Remote control input may be slow during network operations; wait for loading to complete before navigating.