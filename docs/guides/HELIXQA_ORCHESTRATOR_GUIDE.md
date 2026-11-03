# HelixQA Orchestrator Guide

## Overview

The **HelixQA Orchestrator** is a master automation script that wires together the entire QA testing workflow. With a single command, it validates the environment, connects devices, installs APKs, runs HelixQA autonomous testing, and generates consolidated reports.

## Quick Start

### One Command to Test Everything

```bash
./scripts/helixqa-orchestrator.sh
```

That's it! The orchestrator handles everything automatically.

## What Does It Do?

The orchestrator runs 6 phases automatically:

```
┌─────────────────────────────────────────────────────────────┐
│  Phase 1: Environment Validation                             │
│  ├── Checks API is running on localhost:8080                 │
│  ├── Starts API if not running                               │
│  ├── Checks Web is accessible                                │
│  └── Validates HelixQA binary                                │
├─────────────────────────────────────────────────────────────┤
│  Phase 2: Device Connection                                  │
│  ├── Reads .devconnect file                                  │
│  ├── Runs devconnect.sh                                      │
│  ├── Validates device reachability                           │
│  └── Connects all listed devices                             │
├─────────────────────────────────────────────────────────────┤
│  Phase 3: APK Installation                                   │
│  ├── Checks for Android TV APK                               │
│  ├── Builds APK if missing                                   │
│  ├── Uninstalls old versions                                 │
│  └── Installs fresh APKs on all devices                      │
├─────────────────────────────────────────────────────────────┤
│  Phase 4: Monitoring                                         │
│  ├── Starts background monitor                               │
│  ├── Monitors API health                                     │
│  ├── Tracks device connectivity                              │
│  └── Logs HelixQA status                                     │
├─────────────────────────────────────────────────────────────┤
│  Phase 5: HelixQA Execution                                  │
│  ├── Runs autonomous testing                                 │
│  ├── Tests all specified platforms                           │
│  ├── Captures screenshots and logs                           │
│  └── Detects issues automatically                            │
├─────────────────────────────────────────────────────────────┤
│  Phase 6: Report Generation                                  │
│  ├── Consolidates results                                    │
│  ├── Creates summary report                                  │
│  └── Outputs session directory                               │
└─────────────────────────────────────────────────────────────┘
```

## Usage

### Basic Usage

```bash
# Test all platforms (default)
./scripts/helixqa-orchestrator.sh

# Test Android TV only
./scripts/helixqa-orchestrator.sh android

# Test Web only
./scripts/helixqa-orchestrator.sh web

# Test Desktop only
./scripts/helixqa-orchestrator.sh desktop
```

### Command Reference

```bash
./scripts/helixqa-orchestrator.sh [platform]

Arguments:
  platform    Platform to test (optional, default: all)
              Options: all, android, web, desktop

Examples:
  ./scripts/helixqa-orchestrator.sh           # All platforms
  ./scripts/helixqa-orchestrator.sh android   # Android TV only
  ./scripts/helixqa-orchestrator.sh web       # Web only
  ./scripts/helixqa-orchestrator.sh --help    # Show help
```

## Requirements

### Prerequisites

Before running the orchestrator, ensure you have:

1. **ADB (Android Debug Bridge)**
   ```bash
   which adb
   ```

2. **HelixQA Binary**
   ```bash
   ls -la HelixQA/bin/helixqa
   ```

3. **Podman** (for APK building)
   ```bash
   which podman
   ```

4. **Device Configuration**
   - Create `.devconnect` file with Android TV IPs
   - Ensure devices are NOT in `.devignore`

### File Structure

```
Catalogizer/
├── .devconnect              # Device IP list
├── .devignore               # Device exclusions
├── HelixQA/
│   └── bin/
│       └── helixqa          # HelixQA binary
├── scripts/
│   ├── devconnect.sh        # Device connection
│   └── helixqa-orchestrator.sh  # This script
├── catalog-api/             # API source
├── catalog-web/             # Web source
└── catalogizer-androidtv/   # Android TV source
```

## Output

### Session Directory

Each run creates a timestamped session directory:

```
qa-results/
└── session-20260407_195305/
    ├── helixqa/             # HelixQA results
    │   └── session-*/
    │       ├── pipeline-report.json
    │       └── ...
    ├── logs/
    │   ├── orchestrator.log
    │   └── monitor.log
    ├── reports/
    │   └── report.md
    ├── screenshots/
    └── monitor.sh
```

### Report Contents

The generated report includes:

- Session timestamp
- Tested platforms
- Device list
- HelixQA results summary
- Links to detailed logs

### Real-time Output

The orchestrator provides color-coded real-time output:

```
╔════════════════════════════════════════════════════════════╗
║        HelixQA Master Orchestrator                         ║
╚════════════════════════════════════════════════════════════╝

[STEP] === Phase 1: Environment Validation ===
[INFO] Checking API (localhost:8080)...
[INFO] ✓ API is healthy
[INFO] ✓ Environment validation complete

[STEP] === Phase 2: Device Connection ===
[INFO] Running devconnect script...
[INFO] ✓ Device 192.168.0.214:5555 is reachable
[INFO] ✓ Successfully connected to 192.168.0.214:5555

[STEP] === Phase 3: APK Installation ===
[INFO] Installing APKs on device: 192.168.0.214:5555
[INFO]   ✓ Android TV APK installed

[STEP] === Phase 4: Starting Monitoring ===
[INFO] ✓ Monitor started (PID: 12345)

[STEP] === Phase 5: Running HelixQA ===
[INFO] Platforms: android,web
[INFO] Starting HelixQA autonomous session...
[... HelixQA output ...]
[INFO] ✓ HelixQA completed successfully

[STEP] === Phase 6: Generating Report ===
[INFO] ✓ Report generated

[STEP] === Orchestration Complete ===
[INFO] Session directory: qa-results/session-20260407_195305
[INFO] All phases completed successfully!
```

## Configuration

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HELIXQA_BIN` | Path to HelixQA binary | `HelixQA/bin/helixqa` |
| `DEVCONNECT_SCRIPT` | Path to devconnect script | `scripts/devconnect.sh` |
| `QA_RESULTS_DIR` | Output directory | `qa-results` |

### Custom Configuration

```bash
# Use custom paths
export HELIXQA_BIN=/opt/helixqa/bin/helixqa
export QA_RESULTS_DIR=/mnt/storage/qa-results

./scripts/helixqa-orchestrator.sh
```

## Monitoring

### Background Monitor

During execution, a background monitor:

- Checks API health every 30 seconds
- Tracks device connectivity
- Monitors HelixQA process status
- Logs to `session-*/logs/monitor.log`

### Viewing Monitor Logs

```bash
# Tail monitor log in real-time
tail -f qa-results/session-*/logs/monitor.log

# View full monitor log
cat qa-results/session-*/logs/monitor.log
```

## Troubleshooting

### API Not Starting

```
[ERROR] ✗ API not responding on localhost:8080
[INFO] Starting API...
[ERROR] ✗ Failed to start API
```

**Solutions:**
- Check port 8080 is not in use: `lsof -i :8080`
- Verify Go is installed: `go version`
- Check API logs: `cat /tmp/api.log`

### No Devices Connected

```
[WARN] No .devconnect file, skipping device connection
```

**Solution:**
```bash
echo "192.168.0.214" > .devconnect
```

### APK Build Failure

```
[ERROR] ✗ Android TV APK not found
[INFO] Building Android TV APK...
```

**Solutions:**
- Ensure Podman is running
- Check builder image exists: `podman images | grep catalogizer-builder`
- Build manually if needed: See [Build container auto-dispatch](../BUILD_CONTAINER_AUTO_DISPATCH.md)

### HelixQA Fails

```
[ERROR] ✗ HelixQA failed or timed out
```

**Solutions:**
- Check HelixQA logs: `cat qa-results/session-*/logs/orchestrator.log`
- Verify devices are still connected: `adb devices`
- Check API is still running: `curl http://localhost:8080/health`

## Advanced Usage

### Continuous Testing

Run orchestrator in a loop for continuous monitoring:

```bash
#!/bin/bash
while true; do
    ./scripts/helixqa-orchestrator.sh
    echo "Waiting 1 hour before next run..."
    sleep 3600
done
```

### CI/CD Integration

```yaml
# .gitlab-ci.yml
qa:nightly:
  script:
    - ./scripts/helixqa-orchestrator.sh android
  artifacts:
    paths:
      - qa-results/session-*/
    expire_in: 7 days
```

### Selective Testing

Test only specific components:

```bash
# Test only Web (faster)
./scripts/helixqa-orchestrator.sh web

# Test only Android TV
./scripts/helixqa-orchestrator.sh android
```

## Performance

### Execution Time

Typical execution times:

| Phase | Duration | Notes |
|-------|----------|-------|
| Environment Validation | 5-10s | API startup if needed |
| Device Connection | 5-15s | Per device |
| APK Installation | 30-60s | Build + install |
| HelixQA Execution | 20-60min | Depends on coverage |
| Report Generation | 5-10s | |
| **Total** | **25-70min** | |

### Resource Usage

- **CPU**: Moderate (API + HelixQA + monitoring)
- **Memory**: 2-4 GB (depending on media library size)
- **Network**: Continuous (ADB + API calls)

## Comparison: Manual vs Orchestrated

### Manual Workflow (Before)

```bash
# Step 1: Check API
curl http://localhost:8080/health || go run catalog-api/main.go &
sleep 10

# Step 2: Connect devices
adb connect 192.168.0.214
adb devices

# Step 3: Install APKs
adb -s 192.168.0.214:5555 uninstall com.catalogizer.androidtv
adb -s 192.168.0.214:5555 install catalogizer-androidtv/app/build/outputs/apk/debug/app-debug.apk

# Step 4: Run HelixQA
./HelixQA/bin/helixqa autonomous -platforms android -project . -output qa-results/test

# Step 5: Check results
ls qa-results/test/
```

**Time: 10-15 minutes of manual work**

### Orchestrated Workflow (After)

```bash
./scripts/helixqa-orchestrator.sh
```

**Time: 1 command, fully automated**

## Best Practices

### 1. Always Use .devconnect

Ensure devices are listed in `.devconnect`:

```bash
# Check before running
cat .devconnect | grep -v "^#" | grep -v "^$"
```

### 2. Verify .devignore

Make sure test devices are NOT excluded:

```bash
# Should return nothing for your device
grep "MiBox\|mibox\|192.168.0.214" .devignore
```

### 3. Monitor Disk Space

QA sessions can consume disk space:

```bash
# Check space
du -sh qa-results/

# Clean old sessions
find qa-results/ -type d -mtime +7 -exec rm -rf {} +
```

### 4. Review Reports

Always review the generated report:

```bash
cat qa-results/session-*/report.md
```

## See Also

- [Device Connect Guide](./DEVCONNECT_GUIDE.md) - `.devconnect` documentation
- [Testing Guide](./TESTING_GUIDE.md) - General testing procedures
- [HelixQA Documentation](../../HelixQA/README.md) - HelixQA reference
- [AGENTS.md](../../AGENTS.md) - Agent constraints and guidelines

## Support

For issues or questions:

1. Check logs: `qa-results/session-*/logs/`
2. Review troubleshooting section above
3. Consult [Troubleshooting Guide](./TROUBLESHOOTING.md)
