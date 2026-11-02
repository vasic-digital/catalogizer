# User Flow Testing Troubleshooting

## Common Issues

### Container Not Starting

**Symptom:** `podman-compose -f docker-compose.test.yml up` fails or containers exit immediately.

**Diagnosis:**

```bash
# Check container status
podman-compose -f docker-compose.test.yml ps

# Check container logs
podman-compose -f docker-compose.test.yml logs catalog-api
podman-compose -f docker-compose.test.yml logs catalog-web
podman-compose -f docker-compose.test.yml logs playwright

# Check if ports are already in use
ss -tlnp | grep :8080
ss -tlnp | grep :3000
ss -tlnp | grep :9222
```

**Solutions:**

- **Port conflict:** Kill the process occupying the port. Common culprit: Bear Messenger on port 3000. Use `ss -tlnp | grep :3000` to identify and kill it.
- **Image build failure:** Rebuild with `podman build --network host` (SSL issues with default networking).
- **Resource exhaustion:** Check `podman stats --no-stream` and `cat /proc/loadavg`. Stop other containers if exceeding the 4 CPU / 8 GB budget.
- **Stale containers:** Run `podman-compose -f docker-compose.test.yml down -v` to clean up, then start again.

### CDP Connection Failed (Web Challenges)

**Symptom:** Web challenges fail with "Could not connect to CDP endpoint" or "WebSocket connection to ws://localhost:9222 failed".

**Diagnosis:**

```bash
# Verify Playwright container is running
podman ps | grep playwright

# Test CDP connectivity
curl -s http://localhost:9222/json/version

# Check Playwright container logs
podman logs $(podman ps -q --filter name=playwright)
```

**Solutions:**

- **Playwright container not running:** Start it with `podman-compose -f docker-compose.test.yml up -d playwright`.
- **Wrong CDP port:** Verify the container exposes port 9222. The `PlaywrightCLIAdapter` connects to `ws://localhost:9222`.
- **Firewall blocking:** Since all services use `network_mode: host`, ensure the local firewall allows connections on port 9222.
- **Container image not installed:** Pull the image manually: `podman pull mcr.microsoft.com/playwright:v1.40.0-jammy`.

### ADB Not Found (Android Challenges)

**Symptom:** Android challenges fail with "adb: command not found" or "no devices/emulators found".

**Diagnosis:**

```bash
# Check ADB is installed
which adb
adb version

# List connected devices
adb devices

# Check ANDROID_HOME
echo $ANDROID_HOME
```

**Solutions:**

- **ADB not installed:** Install Android SDK platform-tools. On Fedora/ALT: `sudo apt install android-tools` or download from developer.android.com.
- **No device connected:** Start an emulator (`emulator -avd <name>`) or connect a physical device with USB debugging enabled.
- **Wrong device serial:** Set the correct serial via environment variable: `ANDROID_DEVICE_SERIAL=emulator-5554`.
- **ADB server not running:** Start it with `adb start-server`.
- **Permission denied:** Add your user to the `plugdev` group for USB device access.

### Desktop App Not Launching (Desktop/Wizard Challenges)

**Symptom:** Desktop challenges fail with "binary not found" or "failed to launch application".

**Diagnosis:**

```bash
# Check if the binary exists
ls -la ../catalogizer-desktop/src-tauri/target/debug/catalogizer-desktop
ls -la ../installer-wizard/src-tauri/target/debug/installer-wizard

# Try launching manually
../catalogizer-desktop/src-tauri/target/debug/catalogizer-desktop
```

**Solutions:**

- **Binary not built:** Build first with `cd catalogizer-desktop && npm run tauri:build` or `cd installer-wizard && npm run tauri:build`.
- **Wrong binary path:** Set via environment variables: `DESKTOP_BINARY_PATH` or `WIZARD_BINARY_PATH`.
- **Missing shared libraries:** Install required system libraries for Tauri (webkit2gtk, etc.).
- **Display not available:** Desktop challenges require a display (X11 or Wayland). In headless environments, use `xvfb-run` or set `DISPLAY=:99` with Xvfb running.

### Port Conflicts

**Symptom:** Services fail to start because ports are already bound.

**Diagnosis:**

```bash
# Check all relevant ports
ss -tlnp | grep -E ':(8080|3000|9222|5432) '
```

**Solutions:**

| Port | Service | Resolution |
|------|---------|------------|
| 8080 | catalog-api | Kill existing API instance or change port via `PORT` env var |
| 3000 | catalog-web | Kill Bear Messenger or other process: `kill $(lsof -t -i:3000)` |
| 9222 | Playwright CDP | Kill existing Playwright container: `podman stop playwright` |
| 5432 | PostgreSQL | Stop the PostgreSQL container if running in dev mode |

### Challenge Stuck (No Progress)

**Symptom:** Challenge status shows "running" but no progress updates for extended period, eventually transitions to "stuck".

**Diagnosis:**

```bash
# Check challenge progress
curl http://localhost:8080/api/v1/challenges/progress

# Check API server logs
podman logs catalogizer-api 2>&1 | tail -50
```

**Solutions:**

- **Stale threshold hit:** After 5 minutes of no progress, the runner kills the challenge automatically. The challenge will show status "stuck".
- **Long-running scan:** The populate challenge can take up to 25 minutes for large NAS scans. This is expected. The stale threshold resets each time progress is reported.
- **Deadlock in challenge:** If a challenge is truly stuck, restart the API server. RunAll is synchronous/blocking, so no other challenge can run until the stuck one is resolved.
- **Network timeout:** API challenges use a 180-second HTTP client timeout. If the API server is slow, increase the timeout or investigate the slow endpoint.

### Authentication Failures

**Symptom:** Challenges that depend on `UF-API-AUTH-LOGIN` fail with 401 errors.

**Diagnosis:**

```bash
# Test login manually
curl -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"<ADMIN_PASSWORD>"}'

# Check environment variables
echo $ADMIN_USERNAME
echo $ADMIN_PASSWORD
```

**Solutions:**

- **Wrong credentials:** Set `ADMIN_PASSWORD=<ADMIN_PASSWORD>` (or whatever is configured) via environment variable or `.env` file.
- **JWT secret mismatch:** Ensure `JWT_SECRET` is consistent between the API server and the challenge environment.
- **User not created:** The admin user is created on first startup. If the database was reset, restart the API server to recreate it.

### Web App Not Loading (Web Challenges)

**Symptom:** Browser challenges fail with "page did not load" or selector timeouts.

**Diagnosis:**

```bash
# Check if web dev server is running
curl -s http://localhost:3000 | head -5

# Check if API proxy is working
curl -s http://localhost:3000/api/v1/health

# Check .service-port file exists
cat ../catalog-api/.service-port
```

**Solutions:**

- **Dev server not running:** Start with `cd catalog-web && npm run dev`.
- **API proxy misconfigured:** The web dev server reads `../catalog-api/.service-port` to discover the API port. If the file is missing or stale, restart the API server.
- **Node modules missing:** Run `cd catalog-web && npm install`.
- **Build errors:** Run `cd catalog-web && npm run type-check && npm run lint` to identify issues.

### Screenshot Assertion Failures

**Symptom:** Challenges with `screenshot_exists` assertions fail even though the app is running.

**Solutions:**

- **Headless mode issue:** Ensure the browser is running with `--headless` flag in the Playwright container.
- **Display not available:** For mobile screenshots, ensure the emulator is fully booted: `adb shell getprop sys.boot_completed` should return `1`.
- **Empty screenshot data:** This can happen if the app crashed before the screenshot was taken. Check the preceding steps for failures.

### Resource Exhaustion

**Symptom:** System becomes unresponsive, containers killed by OOM, or tests fail with timeout errors.

**Diagnosis:**

```bash
# Check container resource usage
podman stats --no-stream

# Check host load
cat /proc/loadavg

# Check memory
free -h

# Check for OOM kills
dmesg | grep -i oom | tail -10
```

**Solutions:**

- **Reduce parallelism:** Only run one platform group at a time. Platform groups are designed to fit within the 4 CPU / 8 GB budget individually.
- **Stop unnecessary services:** Only run the containers needed for the current platform group.
- **Increase swap:** If memory is tight, add swap space.
- **Kill background processes:** Check for other heavy processes consuming resources.
- **Use Go resource limits:** Always run Go tests with `GOMAXPROCS=3 go test ./... -p 2 -parallel 2`.

## Diagnostic Commands

Quick reference for diagnostic commands:

```bash
# Container status
podman-compose -f docker-compose.test.yml ps
podman stats --no-stream

# Service health
curl -s http://localhost:8080/health | jq .
curl -s http://localhost:3000 | head -1
curl -s http://localhost:9222/json/version | jq .

# Challenge status
curl -s http://localhost:8080/api/v1/challenges | jq .
curl -s http://localhost:8080/api/v1/challenges/progress | jq .
curl -s http://localhost:8080/api/v1/challenges/results | jq .

# Logs
podman-compose -f docker-compose.test.yml logs --tail=50 catalog-api
podman-compose -f docker-compose.test.yml logs --tail=50 catalog-web
podman-compose -f docker-compose.test.yml logs --tail=50 playwright

# Network
ss -tlnp | grep -E ':(8080|3000|9222) '
curl -v http://localhost:8080/health 2>&1 | head -20

# ADB
adb devices
adb shell getprop sys.boot_completed

# Host resources
cat /proc/loadavg
free -h
```
