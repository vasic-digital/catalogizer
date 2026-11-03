# Catalogizer Testing Guide

## Overview

This guide covers all testing workflows for the Catalogizer project, from unit tests to full HelixQA autonomous testing.

## Testing Pyramid

```
                    ┌─────────────────┐
                    │   HelixQA       │  ← E2E Autonomous
                    │  (LLM-driven)   │    Testing
                    └────────┬────────┘
                             │
                    ┌────────▼────────┐
                    │  Integration    │  ← API + UI
                    │     Tests       │    Integration
                    └────────┬────────┘
                             │
            ┌────────────────┼────────────────┐
            │                │                │
    ┌───────▼──────┐ ┌──────▼──────┐ ┌──────▼──────┐
    │  Unit Tests  │ │  Unit Tests │ │  Unit Tests │
    │    (Go)      │ │   (React)   │ │  (Kotlin)   │
    └──────────────┘ └─────────────┘ └─────────────┘
```

## Quick Reference

| Test Type | Command | Duration |
|-----------|---------|----------|
| **Unit Tests (Go)** | `cd catalog-api && go test ./...` | 2-5 min |
| **Unit Tests (Web)** | `cd catalog-web && npm test` | 2-3 min |
| **HelixQA (Full)** | `./scripts/helixqa-orchestrator.sh` | 30-60 min |
| **HelixQA (Android)** | `./scripts/helixqa-orchestrator.sh android` | 20-40 min |
| **HelixQA (Web)** | `./scripts/helixqa-orchestrator.sh web` | 15-25 min |

## Unit Testing

### Backend (Go)

```bash
cd catalog-api

# Run all tests
GOMAXPROCS=3 go test ./... -p 2 -parallel 2

# Run specific package tests
go test ./handlers/... -v
go test ./services/... -v

# Run with coverage
go test -cover ./...

# Run benchmarks
go test -bench=. ./...
```

### Frontend (React/Web)

```bash
cd catalog-web

# Run tests
npm test

# Run with coverage
npm run test:coverage

# Run E2E tests
npm run test:e2e
```

### Android (Kotlin)

```bash
cd catalogizer-android

# Run unit tests
./gradlew test

# Run specific test
./gradlew test --tests "*LoginViewModelTest"
```

## Integration Testing

### API Testing

```bash
# Start API first
cd catalog-api && go run main.go &

# Test endpoints
curl http://localhost:8080/api/v1/health

# With authentication
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"<your-admin-password>"}' | jq -r '.session_token')

curl -H "Authorization: Bearer $TOKEN" http://localhost:8080/api/v1/media/recent
```

## HelixQA Autonomous Testing

### Prerequisites

1. **Devices Configured**
   ```bash
   # Create .devconnect with Android TV IPs
   echo "192.168.0.214" > .devconnect
   
   # Verify device not in .devignore
   grep -i "mibox" .devignore  # Should be empty
   ```

2. **Services Running**
   ```bash
   # API should be accessible
   curl http://localhost:8080/health
   
   # Web should be accessible
   curl http://localhost:3000
   ```

3. **HelixQA Binary**
   ```bash
   ls -la HelixQA/bin/helixqa
   ```

### Full Automation (Recommended)

Use the orchestrator for one-command testing:

```bash
# Test everything
./scripts/helixqa-orchestrator.sh

# Test specific platform
./scripts/helixqa-orchestrator.sh android
./scripts/helixqa-orchestrator.sh web
```

### Manual HelixQA Execution

If you need more control:

```bash
# 1. Connect devices
./scripts/devconnect.sh

# 2. Verify devices
adb devices

# 3. Run HelixQA
cd HelixQA
./bin/helixqa autonomous \
  -platforms android \
  -project /run/media/milosvasic/DATA4TB/Projects/Catalogizer \
  -output qa-results/manual \
  -timeout 2h
```

## Testing Workflows

### Pre-Commit Testing

Before committing code:

```bash
#!/bin/bash
# pre-commit-test.sh

set -e

echo "=== Running Pre-Commit Tests ==="

# 1. Go unit tests
echo "1. Go Unit Tests..."
cd catalog-api
go test ./... -p 2 -parallel 2
cd ..

# 2. Web unit tests
echo "2. Web Unit Tests..."
cd catalog-web
npm test
cd ..

# 3. Linting
echo "3. Linting..."
cd catalog-api
go vet ./...
cd ../catalog-web
npm run lint
cd ..

echo "=== All Pre-Commit Tests Passed ==="
```

### Nightly Full QA

```bash
#!/bin/bash
# nightly-qa.sh

# Run full HelixQA suite
./scripts/helixqa-orchestrator.sh all

# Upload results (example)
aws s3 sync qa-results/session-$(date +%Y%m%d) s3://qa-reports/

# Send notification (example)
curl -X POST https://slack.com/api/chat.postMessage \
  -H "Authorization: Bearer $SLACK_TOKEN" \
  -d "channel=#qa" \
  -d "text=Nightly QA complete: $(ls qa-results/session-*/report.md)"
```

### Release Testing

Before releasing:

```bash
# 1. Full unit test suite
./scripts/run-all-tests.sh

# 2. Full HelixQA suite
./scripts/helixqa-orchestrator.sh

# 3. Security scans
./scripts/security-scan.sh

# 4. Performance tests
./scripts/performance-test.sh
```

## Device Management

### .devconnect Workflow

```bash
# 1. Add device IPs to .devconnect
cat > .devconnect << 'EOF'
# Android TV devices
192.168.0.214    # Mi Box 4 - Living Room
192.168.0.215    # Shield TV - Bedroom
EOF

# 2. Auto-connect devices
./scripts/devconnect.sh

# 3. Verify
adb devices
```

### .devignore Workflow

```bash
# Exclude problematic devices
cat > .devignore << 'EOF'
# Devices excluded from testing
ATMOSphere       # Development boards - unstable
Pixel_3          # Personal phone - do not test
EOF
```

## Test Categories

### 1. Functional Testing

HelixQA automatically tests:
- Login/logout flows
- Navigation and browsing
- Media playback
- Search functionality
- Settings management

### 2. Performance Testing

```bash
# Load testing
cd catalog-api
go test ./tests/performance/... -v

# Stress testing
./scripts/stress-test.sh
```

### 3. Security Testing

```bash
# Security scan
./scripts/security-scan.sh

# Vulnerability check
govulncheck ./...
```

### 4. Accessibility Testing

HelixQA includes accessibility checks:
- Screen reader compatibility
- Keyboard navigation
- Color contrast
- Font sizes

## Monitoring Tests

### Real-time Monitoring

During HelixQA execution:

```bash
# Monitor API logs
tail -f /tmp/api.log

# Monitor device logs
adb -s 192.168.0.214:5555 logcat | grep catalogizer

# Monitor HelixQA output
tail -f qa-results/session-*/logs/orchestrator.log
```

### Post-Test Analysis

```bash
# View report
cat qa-results/session-*/report.md

# Count issues
find qa-results/session-*/helixqa -name "*.json" | xargs jq '.issues_found'

# View screenshots
ls qa-results/session-*/helixqa/session-*/screenshots/
```

## Troubleshooting

### Common Issues

#### Device Not Found

```bash
# Check ADB
adb devices

# Reconnect
./scripts/devconnect.sh

# Restart ADB server
adb kill-server
adb start-server
```

#### API Connection Failed

```bash
# Check API status
curl http://localhost:8080/health

# Restart API
cd catalog-api
pkill -f "go run main.go"
HOST=0.0.0.0 go run main.go &
```

#### APK Install Failed

```bash
# Uninstall first
adb uninstall com.catalogizer.androidtv

# Reinstall
adb install catalogizer-androidtv/app/build/outputs/apk/debug/app-debug.apk
```

## Best Practices

### 1. Always Validate Environment

```bash
# Before running tests
./scripts/services-status.sh
./scripts/devconnect.sh
```

### 2. Use Session-Based Results

Each test run creates a timestamped session:

```
qa-results/
├── session-20260407_120000/   # Old session
├── session-20260407_180000/   # Old session
└── session-20260408_090000/   # Current session
```

### 3. Clean Up Regularly

```bash
# Remove sessions older than 7 days
find qa-results/session-* -mtime +7 -type d -exec rm -rf {} +
```

### 4. Monitor Resource Usage

```bash
# Check disk space
df -h

# Check memory
free -h

# Check running processes
ps aux | grep -E "go run|helixqa|adb"
```

## CI/CD Integration

### GitLab CI Example

```yaml
stages:
  - unit-test
  - integration-test
  - qa

unit-tests:
  stage: unit-test
  script:
    - cd catalog-api && go test ./...
    - cd ../catalog-web && npm test

helixqa:
  stage: qa
  script:
    - ./scripts/helixqa-orchestrator.sh android
  artifacts:
    paths:
      - qa-results/session-*/
    expire_in: 1 week
  only:
    - main
```

### GitHub Actions Example

```yaml
name: HelixQA
on:
  schedule:
    - cron: '0 2 * * *'  # Nightly at 2 AM

jobs:
  qa:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v2
      
      - name: Setup Environment
        run: |
          echo "192.168.0.214" > .devconnect
          ./scripts/devconnect.sh
      
      - name: Run HelixQA
        run: ./scripts/helixqa-orchestrator.sh
      
      - name: Upload Results
        uses: actions/upload-artifact@v2
        with:
          name: qa-results
          path: qa-results/session-*/
```

## Reference

### Scripts

| Script | Purpose |
|--------|---------|
| `helixqa-orchestrator.sh` | Master automation script |
| `devconnect.sh` | Device connection management |
| `run-all-tests.sh` | Full test suite |
| `security-scan.sh` | Security scanning |
| `performance-test.sh` | Performance testing |

### Documentation

| Document | Description |
|----------|-------------|
| [DEVCONNECT_GUIDE.md](./DEVCONNECT_GUIDE.md) | Device connection guide |
| [HELIXQA_ORCHESTRATOR_GUIDE.md](./HELIXQA_ORCHESTRATOR_GUIDE.md) | Orchestrator documentation |
| [AGENTS.md](../../AGENTS.md) | Agent constraints |

## See Also

- [API Documentation](../api/API_DOCUMENTATION.md)
- [Architecture Guide](../architecture/ARCHITECTURE.md)
- [Deployment Guide](../DEPLOYMENT_GUIDE.md)
- [Troubleshooting Guide](./TROUBLESHOOTING.md)


## QA notes: self-registration mode

The API decides what happens to `POST /api/v1/auth/register` with `REGISTRATION_MODE`:
`approval` (default; the account is created inactive and pending until an administrator approves it
through `PUT /api/v1/admin/users/:id` with `{"status":"active"}`), `open` (active at once) or `disabled`.
`docker-compose.test.yml` sets `open` by default (override with the `REGISTRATION_MODE` variable)
because `tests/test_subtitles.sh` and similar scripts register a user and log in immediately.
`scripts/run-full-qa.sh` only checks the register response (201/409/400/429) and does not log in as
the new user, so it works in every mode. A QA flow that needs a working second account on a stack in
`approval` mode must call the admin approve step after registering.
