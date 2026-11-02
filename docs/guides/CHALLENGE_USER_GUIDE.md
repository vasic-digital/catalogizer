# Challenge System User Guide

How to run challenges via the Catalogizer API. Challenges are automated verification tests that validate system functionality end-to-end.

## Prerequisites

- catalog-api running (either locally or in a container)
- Valid admin credentials (challenges require authentication)
- For NAS-related challenges: challenge endpoint config in `catalog-api/challenges/config/`

## Authentication

All challenge endpoints require a valid JWT token. Obtain one first:

```bash
# Login to get a JWT token
TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"<your-admin-password>"}' | jq -r '.token')
```

## API Endpoints

All endpoints are under `/api/v1/challenges`.

### List All Challenges

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/challenges | jq
```

Response:

```json
{
  "challenges": [
    {
      "id": "CH-001",
      "name": "SMB Connectivity",
      "description": "Verify SMB connection to NAS endpoint",
      "category": "connectivity",
      "dependencies": []
    }
  ],
  "total": 209
}
```

### Get a Single Challenge

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/challenges/CH-001 | jq
```

### Run a Single Challenge

```bash
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/challenges/CH-001/run | jq
```

Response:

```json
{
  "id": "CH-001",
  "name": "SMB Connectivity",
  "status": "passed",
  "duration": "1.234s",
  "assertions": [
    {"name": "connection established", "passed": true}
  ]
}
```

### Run All Challenges

**Warning**: `RunAll` is synchronous and blocking. No other challenge can run until it finishes. For large NAS scans, this can take 25+ minutes.

```bash
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/challenges/run | jq
```

### Run Challenges by Category

```bash
curl -s -X POST -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/challenges/run/category/connectivity | jq
```

### Get Past Results

```bash
curl -s -H "Authorization: Bearer $TOKEN" \
  http://localhost:8080/api/v1/challenges/results | jq
```

## Challenge Categories

| Category | ID Range | Count | Description |
|----------|----------|-------|-------------|
| connectivity | CH-001+ | 35 | SMB connectivity, scanning, media detection, entity pipeline |
| userflow-api | UF-API-* | 49 | HTTP API user flows |
| userflow-web | UF-WEB-* | 59 | Playwright browser user flows |
| userflow-desktop | UF-DESKTOP-* | 28 | Tauri desktop + installer wizard flows |
| userflow-mobile | UF-MOBILE-* | 38 | Android + Android TV flows |

## Timeouts and Liveness

- **Runner timeout**: 72 hours (hard upper bound)
- **Stale threshold**: 5 minutes -- if a challenge reports no progress for 5 minutes, the runner kills it with status `stuck`
- **Individual challenge timeout**: Defaults to 5 minutes via `challenge.NewConfig()`. Set `Timeout: 0` to use the runner's timeout instead.
- **HTTP client timeout**: 180 seconds for challenge HTTP requests

## Challenge Execution Rules

1. **Sequential execution only.** Challenges run one at a time. Never trigger multiple run endpoints simultaneously.
2. **Dependency ordering.** `RunAll` respects declared dependencies. If challenge B depends on A, A runs first.
3. **All operations go through the API.** Challenges interact with the running catalog-api service via HTTP, exactly as an end user would. Never use custom scripts or curl commands to bypass the service during challenge execution.
4. **Progress reporting.** Challenges that embed `BaseChallenge` automatically report progress. The runner monitors this to detect stuck challenges.

## Resource Limits

The host machine has strict resource limits (30-40% max). When running challenges:

```bash
# Monitor resource usage during challenge runs
podman stats --no-stream
cat /proc/loadavg
```

## Challenge Configuration

NAS endpoint challenges require a configuration file at `catalog-api/challenges/config/endpoints.json`:

```json
{
  "endpoints": [
    {
      "name": "synology",
      "host": "synology.local",
      "protocol": "smb",
      "username": "user",
      "password": "pass",
      "directories": [
        {"path": "/media/movies", "content_type": "movie"},
        {"path": "/media/music", "content_type": "music"}
      ]
    }
  ]
}
```

If this file is missing, NAS-specific challenges are silently skipped during registration.
