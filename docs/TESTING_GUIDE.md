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
