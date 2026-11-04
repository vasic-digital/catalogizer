# Testing Guide

## Overview

Catalogizer employs a comprehensive testing strategy mandated by **Constitution Article V**, which requires **100% coverage across all 10 test categories** for every component. No category may be skipped, deferred, or partially covered. Shipping is prohibited while any category is incomplete.

## Constitution Article V -- 10 Mandatory Test Categories

### 1. Unit Tests

Pure logic, individual functions, and classes. Every branch (happy, error, edge, adversarial) of every public function must be exercised.

| Component | Framework | Command | Target |
|-----------|-----------|---------|--------|
| catalog-api | Go testing + testify | `GOMAXPROCS=3 go test ./... -p 2 -parallel 2` | 100% package coverage |
| catalog-web | Vitest + RTL | `npm run test` | 100% component coverage |
| catalogizer-desktop | Vitest (React) + Rust `#[test]` | `npm run test` / `cargo test` | 100% both layers |
| installer-wizard | Vitest (React) + Rust `#[test]` | `npm run test` / `cargo test` | 100% both layers |
| catalogizer-android | JUnit 4 + MockK | `./gradlew test` | 100% class coverage |
| catalogizer-androidtv | JUnit 4 + MockK | `./gradlew test` | 100% class coverage |
| catalogizer-api-client | Vitest | `npm run test` | 100% export coverage |
| Go submodules (22) | Go testing | `go test ./...` per module | 100% exported functions |
| TS submodules (9) | Vitest | `npm run test` per module | 100% component coverage |

### 2. Integration Tests

Cross-module interactions, database, cache, queues, filesystems.

- **catalog-api**: `catalog-api/tests/integration/` -- user flows, API round-trips, database operations
- **catalog-web**: `src/components/**/integration.test.tsx` -- auth flow, protected routes, context integration
- **Protocol tests**: SMB, FTP, NFS, WebDAV integration with real or mocked servers

### 3. End-to-End (E2E) Tests

Full user journeys through the live system.

- **catalog-web**: Playwright (`npm run test:e2e`) -- 5 spec files: auth, browse, collections, favorites, accessibility
- **HelixQA**: Autonomous LLM-driven E2E across all platforms
- **Challenges**: User flow challenges (174 registered) exercise full journeys

### 4. Full Automation Tests

Unattended, reproducible, CI-runnable E2E.

- **Challenge runner**: `Challenges/cmd/userflow-runner/` with `--platform`, `--compose`, `--timeout` flags
- **Container test stack**: `docker-compose.test.yml` with profiles per platform
- **HelixQA orchestrator**: `./scripts/helixqa-orchestrator.sh [platforms]`

### 5. Stress Tests

Saturation, concurrency, large payloads, long sessions.

- **k6 scripts** (`tests/k6/`): load_test.js, stress_test.js, soak_test.js, spike_test.js, endurance_test.js, breakpoint_test.js
- **Go stress tests** (`catalog-api/tests/stress/`): concurrent handlers, API load, rate limiter stress, responsiveness
- **Run**: `podman run --rm --network host -v $(pwd)/tests/k6:/scripts docker.io/grafana/k6:latest run /scripts/stress_test.js`

### 6. Security Tests

Authentication/authorization, injection, SSRF, secrets, CVE scans.

- **govulncheck**: `cd catalog-api && govulncheck ./...`
- **npm audit**: `cd catalog-web && npm audit --audit-level=high`
- **Semgrep**: `podman-compose -f docker-compose.security.yml --profile semgrep-scan run --rm semgrep-scanner`
- **Gosec**: `./scripts/gosec-scan.sh`
- **Trivy**: `podman-compose -f docker-compose.security.yml --profile trivy-scan run --rm trivy-scanner`
- **Snyk**: `podman-compose -f docker-compose.security.yml --profile snyk-scan run --rm snyk-cli`
- **Consolidated**: `./scripts/security-scan.sh`
- **HelixQA bank**: `HelixQA/banks/security-comprehensive.yaml` (30 entries)

### 7. DDoS / Rate-Limit Tests

Floods, bursts, slowloris, connection exhaustion, rejection + recovery verification.

- **k6**: `tests/k6/ddos_ratelimit_test.js` -- rate limit verification, burst patterns, auth brute force
- **Go tests**: `catalog-api/tests/stress/rate_limiter_stress_test.go` -- concurrent rate limiter validation
- **HelixQA bank**: `HelixQA/banks/ddos-ratelimit-comprehensive.yaml` (20 entries)
- **Recovery criterion**: System must recover within 30s after attack stops

### 8. Benchmarking Tests

Latency, throughput, memory baselines with regression detection.

- **Go benchmarks**: `go test -bench=. -benchmem ./...` -- WorkerPool, JWT, TitleParser, RateLimiter
- **k6 baselines**: `tests/k6/monitoring_test.js` -- response time SLAs per endpoint
- **HelixQA bank**: `HelixQA/banks/benchmarking-baselines.yaml` (15 entries)
- **Regression detection**: Compare against stored baselines; fail if p99 latency increases >10%

### 9. Challenges

Registered `digital.vasic.challenges` entry per feature.

- **507+ challenges** registered in catalog-api
- **Categories**: 50 original (CH-001..050) + 174 userflow (UF-*) + 15 module verification (MOD-*)
- **Run single**: `curl -X POST http://localhost:8080/api/v1/challenges/<id>/run`
- **Run all**: `curl -X POST http://localhost:8080/api/v1/challenges/run-all`
- **CLI runner**: `cd Challenges/cmd/userflow-runner && go run . --platform api --timeout 30m`
- **Bank files**: `challenges/config/`, `challenges/helixqa-banks/`

### 10. HelixQA

Autonomous bank + session entry per screen, flow, and adversarial case.

- **Orchestrator**: `./scripts/helixqa-orchestrator.sh [platforms]`
- **Banks**: 1,600+ test cases across 25 YAML files
- **Pipeline**: Learn -> Plan -> Execute -> Curiosity -> Analyze
- **Platforms**: androidtv, android, web, desktop
- **Vision**: LLM-driven screenshot analysis for every action
- **Output**: `qa-results/session-<timestamp>/`

## Mandatory Retesting Loop

After any change, the full loop must execute until clean:

1. **Rebuild** affected binaries, containers, deployments
2. **Execute** all tests (all 10 categories)
3. **Analyze** results, videos, screenshots, logs
4. **Create tickets** for every defect
5. **Fix** root causes + add regression test to fixes-validation bank
6. **Repeat** from step 1

## Platform Coverage Order

Per Constitution Article V, coverage is achieved sequentially:

1. catalog-api
2. catalog-web
3. catalogizer-desktop
4. installer-wizard
5. catalogizer-android
6. catalogizer-androidtv
7. catalogizer-api-client
8. Go submodules
9. TS submodules
10. HelixQA

---

## Security Testing Detail

### 3. Security Testing (Mandatory)

#### SonarQube Code Quality Analysis
**Purpose**: Static code analysis for bugs, vulnerabilities, and code smells

**Setup**:
```bash
# Set environment variable
export SONAR_TOKEN=your_sonar_token_here

# Run analysis
./scripts/sonarqube-scan.sh
```

**Requirements**:
- Quality gate must pass
- No critical or blocker issues
- Coverage minimum 80%
- Code smell density < 5%

**Reports**: `reports/sonarqube-report.json`

#### Snyk Security Scanning (Freemium)
**Purpose**: Dependency vulnerability scanning and Static Application Security Testing (SAST)

**Freemium Benefits**:
- Unlimited private repositories
- Unlimited developers
- 200 tests per month for public repos
- Basic vulnerability remediation guidance

**Setup**:
```bash
# 1. Sign up for free account at https://snyk.io
# 2. Get your token from https://snyk.io/account
# 3. Set environment variables
export SNYK_TOKEN=your_snyk_token_here
export SNYK_ORG=your_org_name  # Optional

# 4. Run scanning
./scripts/snyk-scan.sh
```

**Requirements**:
- No high or critical severity vulnerabilities
- Dependencies must be regularly updated
- Security policies must be enforced

**Reports**: `reports/snyk-*-results.json`

#### Additional Security Tools

**Trivy Vulnerability Scanner**:
```bash
# Scan filesystem
docker-compose -f docker-compose.security.yml run --rm trivy-scanner
```

**OWASP Dependency Check**:
```bash
# Check dependencies
docker-compose -f docker-compose.security.yml run --rm dependency-check
```

## Running All Tests

#### Initial Setup (Freemium Accounts)
```bash
# Setup your freemium security testing accounts
./scripts/setup-freemium-tokens.sh
```

#### Full Test Suite (Including Security)
```bash
# Run all tests including security scans
./scripts/security-test.sh

# Or run individual security scans
./scripts/sonarqube-scan.sh  # Requires SONAR_TOKEN
./scripts/snyk-scan.sh       # Requires SNYK_TOKEN
```

This script will:
1. Start security services (SonarQube, etc.)
2. Run all unit and integration tests
3. Perform security scans
4. Generate comprehensive reports
5. Stop security services

### Prerequisites
- Docker and Docker Compose installed
- Environment variables set for security tools
- All dependencies installed in project modules

## Test Reports

All test results are stored in the `reports/` directory:

- `comprehensive-security-report.html` - Main security report
- `sonarqube-report.json` - Code quality analysis
- `snyk-*-results.json` - Vulnerability scans per module
- `trivy-results.json` - Container vulnerability scan
- `dependency-check/` - OWASP dependency analysis

## Quality Gates

### Mandatory Requirements
- ✅ All unit tests pass (100% success rate)
- ✅ All integration tests pass
- ✅ SonarQube quality gate passes
- ✅ No high/critical Snyk vulnerabilities
- ✅ Minimum 80% test coverage
- ✅ No broken modules or features

### Zero-Defect Policy
Catalogizer follows a zero-defect policy where:
- All tests must pass before deployment
- Security scans must pass with no critical issues
- Code quality metrics must meet standards
- No module can be left broken or disabled

## CI/CD Integration

### Local CI/CD

> **Note:** GitHub Actions are permanently disabled for this project. All testing and security scanning runs locally.

Run security scans locally:
```bash
# SonarQube analysis (set SONAR_TOKEN env var first)
export SONAR_TOKEN=your_sonar_token
./scripts/sonarqube-scan.sh

# Snyk security analysis (set SNYK_TOKEN env var first)
export SNYK_TOKEN=your_snyk_token
./scripts/snyk-scan.sh

# Full test suite including security
./scripts/run-all-tests.sh
```

### Required Environment Variables
- `SONAR_TOKEN`: SonarQube authentication token
- `SNYK_TOKEN`: Snyk API token
- `SNYK_ORG`: Snyk organization name (optional)

## Troubleshooting

### Common Issues

#### SonarQube Connection Failed
```bash
# Check if SonarQube is running
curl -f http://localhost:9000/api/system/status

# Restart services
docker-compose -f docker-compose.security.yml down
docker-compose -f docker-compose.security.yml up -d sonarqube
```

#### Snyk Authentication Failed
```bash
# Verify token
snyk auth $SNYK_TOKEN

# Check organization
snyk orgs
```

#### Docker Issues
```bash
# Clean up containers
docker system prune -a

# Restart Docker service
sudo systemctl restart docker
```

### Test Failures

#### Integration Tests Failing
- Check database connection
- Verify API server is running
- Check environment configuration

#### Security Scans Failing
- Update dependencies to latest secure versions
- Review and fix code quality issues
- Address security hotspots

## Development Testing

### Pre-commit Testing
```bash
# Run tests before committing
./scripts/run-all-tests.sh

# Run security checks
./scripts/security-test.sh
```

### Module-specific Testing

#### Go Backend
```bash
cd catalog-api
go test -v -race -cover ./...
```

#### React Frontend
```bash
cd catalog-web
npm test -- --coverage
```

#### Android Apps
```bash
cd catalogizer-android
./gradlew testDebugUnitTest
```

## Performance Testing

### Load Testing
```bash
# Using Artillery (if configured)
cd qa-ai-system/scripts/ci-cd
artillery run performance-test.yml
```

### Memory and CPU Profiling
```bash
# Go profiling
cd catalog-api
go test -bench=. -benchmem -cpuprofile=cpu.prof -memprofile=mem.prof ./...
```

## Security Testing Best Practices

### Code Review Checklist
- [ ] Security-sensitive functions reviewed
- [ ] Input validation implemented
- [ ] Authentication/authorization checked
- [ ] SQL injection prevention verified
- [ ] XSS prevention implemented
- [ ] CSRF protection in place

### Dependency Management
- [ ] Dependencies regularly updated
- [ ] Security advisories monitored
- [ ] License compliance checked
- [ ] Unused dependencies removed

### Infrastructure Security
- [ ] Container images scanned
- [ ] Secrets properly managed
- [ ] Network security configured
- [ ] Access controls implemented

## Reporting Issues

### Test Failures
1. Check test logs in `reports/` directory
2. Identify root cause
3. Fix the issue
4. Re-run tests
5. Update documentation if needed

### Security Vulnerabilities
1. Assess severity and impact
2. Implement fix or mitigation
3. Update dependencies if applicable
4. Re-scan to verify resolution
5. Document the resolution

## Continuous Improvement

### Metrics Tracking
- Test coverage trends
- Security scan results over time
- Performance benchmarks
- Code quality metrics

### Regular Updates
- Security tools updated quarterly
- Dependencies reviewed monthly
- Test suites expanded with new features
- Documentation kept current

## Support

For testing-related issues:
- Check this guide first
- Review test logs in `reports/`
- Check GitHub Issues for similar problems
- Contact the development team

## Appendix

### Test Scripts Reference

| Script | Purpose | Requirements |
|--------|---------|--------------|
| `security-test.sh` | Full security testing | Docker, tokens |
| `sonarqube-scan.sh` | Code quality analysis | SONAR_TOKEN |
| `snyk-scan.sh` | Vulnerability scanning | SNYK_TOKEN |
| `run-all-tests.sh` | Unit and integration tests | None |

### Environment Variables

| Variable | Purpose | Required |
|----------|---------|----------|
| `SONAR_TOKEN` | SonarQube authentication | Yes for SonarQube |
| `SNYK_TOKEN` | Snyk authentication | Yes for Snyk |
| `SNYK_ORG` | Snyk organization | Optional |
| `SONAR_HOST_URL` | SonarQube server URL | Optional (defaults to localhost) |

### File Structure

```
reports/
├── comprehensive-security-report.html
├── sonarqube-report.json
├── snyk-api-results.json
├── snyk-web-results.json
├── trivy-results.json
└── dependency-check/
    ├── dependency-check-report.html
    ├── dependency-check-report.json
    └── dependency-check-report.xml
```

## Live WebDAV test (real server only)

`catalog-api/filesystem/live_webdav_test.go` (build tag `live`) drives the real `WebDAVClient` against a real WebDAV server: connect, list, write a uniquely named file inside the sandbox, read back and compare sha256, HTTP 404 on a missing file (asserted as a status), rejection of a deliberately wrong password, delete only its own file. Environment: `LIVE_WEBDAV_URL`, `LIVE_WEBDAV_USER`, `LIVE_WEBDAV_PASSWORD`, `LIVE_WEBDAV_SANDBOX` (at least share/folder deep), optional `LIVE_WEBDAV_INSECURE=1` for a self-signed certificate. Without the tag the test is not compiled; with the tag and no environment it SKIPs `credentials_absent`; a dial/TLS failure or a non-WebDAV answer (405, 501, or 200 without a DAV header) SKIPs `service_unavailable`, while a WebDAV server answering 401/403/404 FAILs with the status; the sandbox is cleaned and must be a rooted path of at least share/folder with no `.`/`..` segments. Pure decision logic is unit-tested in `live_webdav_logic_test.go` (default `go test`) (never a stand-in, section 11.4.289). Run: `go test -tags live -run TestLiveWebDAV -v ./filesystem/` from `catalog-api`, or `scripts/test-infra/webdav_live.sh` for a redacted run with a machine-readable verdict (PASS only when both live tests passed and none skipped; exit 3 for SKIPPED or PARTIAL).

## Live Yandex WebDAV suite (real service, `/Testing` only)

`catalog-api/filesystem/live_webdav_yandex_test.go` (build tag `live`) drives Catalogizer's own `WebDAVClient` against the real Yandex WebDAV service (`https://webdav.yandex.ru`), and `catalog-api/internal/services/live_webdav_scan_test.go` runs the real WebDAV `GenericScanner` over a remote tree. Constitution section 11.4.289: real service, no fixture server; no credentials or an unreachable service is a `SKIP` (`credentials_absent` / `service_unavailable`), never a stand-in.

- **Confinement (operator rule).** A guard `http.RoundTripper` refuses every request whose path (or `Destination` header) is not `/Testing` or below, plus non-https and foreign hosts, before a byte leaves the process. Work happens in a unique `Testing/catalogizer-live-<epoch>` directory and only that directory is deleted. Decision logic is unit-tested in the default build (`TestYandexAllowedPath`, `TestYandexAllowedURL`, `TestDetBytesDeterministic`); the live test also proves the guard refuses a raw `PROPFIND /../Other` and a `MOVE` with an outside `Destination`.
- **Steps (sequential sub-tests, each recorded in the evidence):** connect/auth success; auth failure with a deliberately wrong password constant (clean `401`, client not connected, no secret in the error); mkdir (existing `405`, missing parent `409`); upload of deterministic payloads of known sha256 (4 KiB, 1 MiB, 2 MiB); list with sizes and mtimes (plausible against the local clock); download plus sha256 equality plus stat; ranged read (probed with a raw guarded `GET`: the client has no ranged read); `COPY` (client), overwrite-by-copy, `MOVE` (raw: the client has no rename); overwrite; nonexistent-path errors for read, stat, list, delete, write; unicode and special names (Cyrillic, CJK, accents, emoji, space, `&`, `+`, `=`, `#`, `%`, `?`, nested unicode directory); a 200-file listing (uploaded with 4 workers); 8 concurrent uploads (at most 4 in flight) with per-file sha256; an upload from an unsized reader (chunked); timeout and cancel (deadline far below the transfer time returns `context.DeadlineExceeded` promptly and leaves no truncated file; an already-cancelled context; a cancel in the middle of a body; the client stays usable); delete (file and recursive directory); the guard check.
- **Scan leg.** `TestLiveYandexScan_RemoteTreeIsCatalogued` uploads a 3-level tree with unicode names, scans it with the real WebDAV scanner (only the record seam is a collector), asserts every catalogued path, size and directory flag, downloads each catalogued file and compares its sha256, and checks that the public https host passes `GuardProviderURL` while loopback is refused.
- **Determinism.** `scripts/test-infra/webdav_yandex_live.sh` runs the suite `YANDEX_RUNS` times (default 3) in fresh processes and compares the `signature` (step results) and the content hashes: `SIGNATURES IDENTICAL`. Evidence JSON per run (request counts by method and status, bytes, latency p50/p95/max, guard violations, per-step notes) under `docs/qa/webdav-yandex-live-20261103/`; it holds no credential. Companion guide: `docs/scripts/webdav_yandex_live.md`.
- **Service facts observed (2026-11-03, one account, one network path; not constants).** Uploads are throttled to about 16 KiB/s (earlier uncommitted measurements: 1 MiB about 67 s, 2 MiB about 129 s; the committed `runN.json` step timings are authoritative), so the suite takes about 6 minutes a run; run it in the background. Downloads are fast. A transient `connection reset by peer` and a 30 s PROPFIND timeout were each seen once; the bootstrap probe retries 3 times before it reports `service_unavailable`.
- **Defect found and fixed by this suite.** The client's `http.Client{Timeout: 30s}` covers the whole request including the body, so a progressing transfer longer than 30 s was cut (live RED, committed as `docs/qa/webdav-yandex-live-20261103/run_RED_pre_fix_client_base05709505.log`: the pre-fix client failed the 1 MiB upload with `Client.Timeout exceeded` while the service was still accepting it; the same cap would have cut every large `ReadFile` stream). Now metadata operations are bounded as a whole (`webdavOpTimeout`, 30 s), and transfers by the caller's context plus an idle timeout (`webdavIdleTimeout`, 60 s without a byte moving; `webdavResponseWait`, 10 min for the answer after the last upload byte). Unit guards in `webdav_transfer_timeout_test.go` (slow-but-progressing upload and download succeed, a stalled body is cut, a hung HEAD is bounded, a sized body stays sized, the default client has no whole-request `Timeout`); the mutation that restores `Timeout` makes three of them fail.
- Run: `YANDEX_WEBDAV_ENV_FILE=<0600 file> scripts/test-infra/webdav_yandex_live.sh`.

## Scaling and chaos (live)

LIVE-class suite (section 11.4.289) for `catalog-api`: `catalog-api/scaling_live_test.go` (build tag `live`, excluded from `go test ./...`) with pure helpers in `scalinglive_helpers_test.go` and their unit tests in `scalinglive_helpers_unit_test.go` (default `go test`). Runner: `scripts/test-infra/scaling_live.sh`.

What is real: the `catalog-api` binary built by `go build` from the tree under test, running against a real SQLCipher SQLite file created by the production migrations, on `127.0.0.1` with an ephemeral port (re-checked against the forbidden ports 38080/33000/33001/38081/33010/33011), real HTTP, real JWT login through the seeded-admin path (random secrets generated at runtime, never printed). The only fixture inputs are 20 000 synthetic `files` rows inserted through the real database layer (`database.NewConnection`) and a noise PNG for the real ImageMagick converter. No mock, stub, fixture server or recorder is in the path. Clients use distinct loopback source IPs (`127.0.x.y`) because the API rate limiter keys on client IP (a single IP is capped at 2000 requests/min, 30 logins/min).

Run: `scripts/test-infra/scaling_live.sh` (all four live tests, about 3 minutes plus any wait for host load). Exit codes: `0` PASS, `1` FAIL (a test failed, `go test` exited non-zero, nothing ran, or the tree under test does not build: a failed `go build` is a FAIL, never a SKIP), `77` SKIP (no go toolchain, no `systemd-run`, ImageMagick missing so every live test skipped, or host load stays above `SCALING_LIVE_MAX_LOAD`, default 14), `78` PARTIAL (at least one live test passed and at least one skipped, none failed: not a full PASS, the skipped legs are unproven); a SKIP or PARTIAL is never a PASS. `scaling_live.sh --classify <log> <rc>` prints the verdict for an existing log (tested by `scripts/test-infra/test_scaling_live_classify.sh`). A round also fails when more than `SCALING_MAX_CLIENT_ERR_SHARE` (default 0) of its responses are 401/403/404, or when a status key is malformed or any 5xx (500 to 599) appears. It runs under `systemd-run --user --scope -p MemoryMax=4G -p MemorySwapMax=0`. `SCALING_LIVE_RUN=TestLiveStress` selects one test. Evidence (JSON, one file per phase, plus the binary, per-run DB and server log) goes to `SCALING_LIVE_ARTIFACT_DIR` (default `.scaling-live-artifacts/<utc>/`, gitignored). Every evidence file carries the binary sha256, the endpoint (`host:port`), pid, executable path and `/health` build info, request counts, status histograms, p50/p95/p99, RSS/fds/threads/goroutines.

| Test | What it does | Pass criteria |
|---|---|---|
| `TestLiveScaling` | 20k rows; 8 candidate read endpoints probed, those answering 200 are loaded; calibration 3 x 6 s at 8 clients, then 8/32/64 clients x 3 identical rounds x 10 s | zero 5xx, zero transport errors, at least 200 x 200 per round, p99 (of 200 responses) at most the derived threshold, RSS growth first-to-last round at most 128 MiB (`SCALING_RSS_GROWTH_KB`), pass/fail verdict identical across the 3 rounds |
| `TestLiveStress` | 320 wrong-password logins from one IP; two 128 MiB request bodies (login, authenticated `search/advanced`); 500 idle keep-alive connections then close | login: only 401/429, at least one 429, burst IP still 429 afterwards; bodies: no 5xx, RSS delta under 256 MiB; storm: fds rise by about 500 and return to baseline (+10) within 40 s; server alive after each |
| `TestLiveChaosKillRestart` | 40 real conversion jobs + 16 clients of read load, `SIGKILL` once the worker has a job in `running`, offline `PRAGMA integrity_check`, restart on the same DB | integrity `ok`, 40 rows (none lost/duplicated), healthy again within 60 s (`CHAOS_RESTART_BOUND_S`), restart logs the requeue, all 40 jobs reach `completed` with a non-empty output file |
| `TestLiveChaosGracefulShutdown` | a genuinely long in-flight request (`/debug/pprof/profile?seconds=4`), `SIGTERM` after 1.2 s | the in-flight response completes with 200 and a body, exit status 0, exit within 35 s (`CHAOS_SHUTDOWN_BOUND_S`; the code's own bound is 30 s) |

Threshold derivation (measure first, then bound): the p99 threshold is the largest p99 measured in the three calibration rounds at 8 clients times `SCALING_P99_FACTOR` (default 25), never below `SCALING_P99_FLOOR_MS` (default 250); `SCALING_P99_MS` overrides it. The factor is a deliberately generous host-dependent choice, not a service-level objective: it exists to catch collapse (the finding below), not 2x regressions. All numbers below are host-dependent.

Measured on this host (shared dev workstation, load average 9 to 19 during the runs), 2026-11-03, binary `scaling-live-52cd3174` plus the fix below, one full run (`scripts/test-infra/scaling_live.sh`, exit 0, 4 of 4 live tests passed, 0 skipped):

| Clients | Requests (3 rounds) | Status | p50 / p95 / p99 ms per round | RSS after rounds (MiB) |
|---|---|---|---|---|
| calibration 8 | 3 rounds | all 200 | baseline p99 447 ms (max of 3) | |
| 8 | 548 / 544 / 531 | all 200 | 158/346/376, 161/353/384, 161/348/417 | 145.6, 145.6, 154.5 |
| 32 | 785 / 725 / 677 | all 200 | 420/995/1218, 431/1097/1319, 467/1187/1380 | 202.2, 216.8, 233.2 |
| 64 | 762 / 782 / 571 | all 200 | 874/1881/2305, 839/1859/2244, 1149/2872/3316 | 242.8, 253.1, 257.3 |

Derived threshold in that run: 447 ms x 25 = 11 176 ms; 5 925 requests, all 200, zero 5xx, zero transport errors, verdicts identical across rounds at every level. Throughput stays near 55 to 80 requests/s at every level (the host and the single SQLite file are the limit), so latency rises about linearly with clients. Server threads 24 to 46, fds 25 to 41 after load, goroutines 30 after load.

Stress, same run: login burst 320 attempts from one IP gave 30 x 401 and 290 x 429, the correct password from the burst IP was 429, from a clean IP 200 (no lockout of the admin account). 128 MiB bodies: both answered 400, RSS delta 40 kB. Storm: 500 of 500 connections established, server fds 33 to 533 to 33 after close. Chaos kill: at `SIGKILL` the DB held `completed=3, pending=34, running=3`; integrity_check `ok`; restart healthy in 2.2 s; the log line `Conversion worker requeued interrupted jobs` (requeued 3); final `completed=40`, 40 non-empty output files. SIGTERM: the 4.0 s in-flight request returned 200 (584 bytes), process exit status 0, 3.2 s after the signal.

Defect found and fixed by this suite (RED first): before the fix, 32 clients got 19 x 200 in 10 s with 25 transport errors (client timeout 30 s), p99 about 29.9 s, and some authenticated requests answered 401 with latency exactly 3.000 s (the session checker times out against the DB and fails closed). Root cause: `FileRepository.SearchFiles` ran the per-row metadata query while its own result cursor was still open, so each in-flight search held one pooled connection and waited for a second; with more concurrent searches than the pool (25) every search waited on another (pool deadlock, ended only by the client giving up). Reproduced at repository level with a pool of 3 and 12 concurrent searches against a real SQLite file (`repository/file_repository_pool_test.go`: RED, `context deadline exceeded`, then GREEN after buffering the page and closing the cursor before the metadata queries). Mutation check of the unit helpers: forcing `evaluateRound` to return no violations makes 4 unit tests fail.

Honest limits: the workload is read-only browse/search; write paths, SMB/WebDAV backends, Postgres and TLS are not exercised. The suite never goes above 64 clients and ran on a shared host, so absolute numbers are not capacity claims. The 401s seen before the fix show that under DB starvation the session check fails closed after 3 s (by design); no 5xx was observed in any run. The RSS rise and the monitor alert seen in the 9-round run are explained by the soak below (S1).

### Memory soak (S1)

`TestLiveSoak` (`catalog-api/scaling_soak_live_test.go`, tag `live`, skipped unless `SCALING_SOAK_ROUNDS=N` is set) repeats ONE identical load round N times against one real server and records per round: RSS (`RssAnon`/`RssFile`), Go `HeapInuse`/`HeapSys`, post-GC `HeapAlloc` and `HeapObjects` (from `/debug/pprof/heap?debug=1&gc=1`), goroutines (after the round and peak during it), fds, threads, status histogram; heap profiles at selected rounds and `/proc/<pid>/smaps` at the last round. Env: `SCALING_SOAK_MODES` (`distinct32` one source IP per client, `fewips` 4 IPs shared, `manyips` a fresh IP set every round), `SCALING_SOAK_CLIENTS` (default 32), `SCALING_SOAK_ROUND_SECS` (10), `SCALING_SOAK_PROFILE_ROUNDS`, `SCALING_SOAK_SERVER_ENV` (comma list of `KEY=VAL` for the server, diagnostics only). Run: `SCALING_LIVE_RUN=TestLiveSoak SCALING_SOAK_ROUNDS=30 SCALING_SOAK_ROUND_SECS=6 scripts/test-infra/scaling_live.sh`.

What was measured (2026-11-03, shared host, host-dependent; 32 clients, 6 s rounds, 20k rows):

| Build / setting | Rounds | RSS round 1 -> last (MiB) | Notes |
|---|---|---|---|
| before the fix, default env | 30 | 183 -> 298 (+115) | 54 glibc malloc arenas hold about 190 MiB of anonymous memory in the process; Go `HeapSys` flat at 44 -> 52 MiB; Go heap objects flat at about 24k |
| before the fix, `MALLOC_ARENA_MAX=2` | 16 | 183 -> 195 (+12) | one arena of about 48 MiB; same Go heap |
| after the fix (arena cap at load), default env | 30 | 168 -> 196 (+28) | 6 arenas |
| after the fix (arena cap + limiter sweep), long run | 70 | 162 -> 167 (mean of rounds 1-10 158, 21-30 165, 51-60 173, 61-70 172) | plateau: slope about 0 MiB/round over the last 20 rounds (-0.11, -0.03); post-GC heap objects 22.5k -> 23.4k; goroutines 28, fds 40 |

Root cause of the RSS ramp: SQLCipher is linked through cgo; each OS thread that calls into C gets its own glibc malloc arena (cap 8 x cores) and freed C memory stays in the arenas. This is arena retention, not a Go-heap leak. Fix: `catalog-api/main_malloc_glibc.go` caps the arenas at 2 in a load-time C constructor (calling `mallopt` from a Go `init()` was measured to be too late: 54 arenas remained). An explicit `MALLOC_ARENA_MAX` in the environment wins. The steady-state bound to expect for this workload is about 170 to 200 MiB RSS (measured 172 MiB mean over rounds 51-70); without the cap about 300 MiB.

The `MEMORY ALERT: potential leak detected` line was a false positive of the goroutine criterion (`goroutines > 1.5 x start-up baseline`, the heap threshold of 10x was not reached: ratio 1.45). Goroutines are 29 to 30 when idle or between rounds and peak at 207 to 256 during 64 concurrent clients (one goroutine per live connection). The monitor now reports a goroutine-only trigger as `MEMORY NOTICE ... (load-correlated)` at info level with a `trigger` field; only a heap ratio above the threshold is an `ALERT`.

Rate limiter: `RateLimitByUser` kept one entry per distinct client forever. That is real unbounded growth with client cardinality (about 6 heap objects per client in the `manyips` run: 24.2k -> 28.2k objects over 640 IPs) but it is NOT the cause of the RSS ramp (the `fewips` run with 4 IPs ramps the same way). Fixed: idle keys are swept at most once per window (`internal/auth/ratelimit.go`).

Honest limits (UNCONFIRMED where stated): RSS rose monotonically across the 9 load rounds (from 146 to 257 MiB, about 111 MiB) and the process's own memory monitor logged `MEMORY ALERT: potential leak detected` (heap ratio 1.45, 205 goroutines) once at 64 clients; the 128 MiB per-level bound passes, but whether RSS plateaus is UNCONFIRMED (a longer soak is needed, see `docs/FOLLOW_UPS_NEXT_PHASE.md`). The workload is read-only browse/search; write paths, SMB/WebDAV backends, Postgres and TLS are not exercised. The suite never goes above 64 clients and ran on a shared host, so absolute numbers are not capacity claims. The 401s seen before the fix show that under DB starvation the session check fails closed after 3 s (by design); no 5xx was observed in any run.

### Pool-deadlock class guard (nested query while a rows cursor is open)

A function that runs a pooled query or exec inside a `for rows.Next()` loop holds one pooled connection for the cursor and asks for a second; with more concurrent callers than pool connections every caller waits for a connection only another waiter can free (a pool deadlock; queries without a deadline never end). Code that writes while its own SELECT cursor is open can also lose the write in rollback-journal mode (busy timeout, then "database is locked").

- Guard: `catalog-api/internal/poolguard` (`go test ./internal/poolguard`). A `go/parser` scan of the non-test Go files of `catalog-api` fails on a pooled call (`Query`, `QueryRow`, `Exec`, their `*Context` forms, `Prepare`, `InsertReturningID`) inside a `for X.Next()` body, directly or through same-package helpers up to three calls deep. Exceptions go in the justified allowlist in `guard_test.go` (today only `RefreshSmartPlaylist`, one `*sql.Tx` = one pinned connection); a stale allowlist entry also fails. Control needles: `testdata/positive` must be found (direct, indirect, exec-per-row), `testdata/negative` (buffered shape, non-cursor loop) must not.
- Fix pattern: scan the page into a slice, close the cursor (`rows.Close()`), then run the per-row queries.
- Per-member regression tests (real SQLite file, production migrations, pool of 3, 12 concurrent callers, shared helper `internal/pooltest`; RED signature `pool deadlock: 12 of 12 callers still blocked`): `repository/pool_class_test.go`, `handlers/pool_class_test.go`, `internal/services/pool_class_test.go`, `internal/media/realtime/pool_class_test.go`.
## Registration approval and converter tools (real binary)

Two real-binary tests in `catalog-api/` (they build and start the server on SQLite; skipped under `-short`):

```bash
cd catalog-api
go test -run TestRegistrationApproval_RealBinary -count=1 .   # register -> pending -> login 403 -> admin approves -> login 200; open/disabled modes
go test -run TestConversionToolsMissing_RealBinary -count=1 . # PATH without ffmpeg: job ends failed naming the tool; /health/deep converters all false
```

To verify the runtime image end to end: build it locally with rootless podman
(`podman build --network host -f catalog-api/Dockerfile -t localhost/<name>:test <context>`), run it on an
ephemeral loopback port with a throwaway `ADMIN_PASSWORD`/`JWT_SECRET`, check `GET /health/deep` shows
`ffmpeg`, `convert` and `pdftotext` as `true`, and submit a real conversion job through
`POST /api/v1/conversion/jobs` (source and target under the container's `/app/data`). Remove only the
container and image you created. Tests that register a user and log in straight away must run the server
with `REGISTRATION_MODE=open` (see `docs/UPGRADE_NOTES.md`).

## Live-evidence records: what the verifier does and does not defend against (section 11.4.289)

`scripts/test-infra/verify_live_evidence.sh` (doc: `docs/scripts/verify_live_evidence.md`) reclassifies a record that claims LIVE to UNIT when the answering process is a registered double, does not match the expected identity, loads code the script hash does not cover (interpreter option allow-list, `docs/qa/live-evidence.interpreter-options.json`), carries injecting environment (`interpreter_env_injection`, closed key set `env_keys`; an interpreter record without `env_checked: true` is `env_unrecorded`), or names its script by a relative path (only an absolute script token that equals `script_path` binds).

**Threat model: COOPERATIVE PRODUCER** (operator decision, constitution 11.4.289(J)). The verifier defends against accidental or careless stubs and mislabelled runs, not against a double that actively forges its own process state. Stated limits: (F3) a process can rewrite its own `/proc/<pid>/cmdline` (ctypes, `process.title`, `setproctitle`); (F4) a container is identified by its image digest only, so command / entrypoint / mount / environment overrides are not captured; (F5) a generic launcher script binds the launcher's hash, not what it finally runs; the script file is hashed when the record is written, not when it is loaded; environment variables outside the closed key set are not visible. A harness that itself exports one of the injecting keys (for example `LD_PRELOAD`) must spawn the server under test with that key removed.
