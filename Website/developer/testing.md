---
title: Testing Guide
description: Running and writing tests for Catalogizer - unit, integration, E2E, stress, fuzz, and challenge tests
---

# Testing Guide

Catalogizer has a comprehensive test suite spanning unit tests, integration tests, end-to-end tests, security tests, and a challenge framework. This guide covers how to run existing tests and write new ones.

---

## Test Commands Quick Reference

```bash
# Backend (Go)
cd catalog-api
GOMAXPROCS=3 go test ./... -p 2 -parallel 2      # all tests
go test -v -run TestName ./path/to/pkg/            # single test

# Frontend (React/TypeScript)
cd catalog-web
npm run test                                        # single run
npm run test:watch                                  # watch mode
npm run test:coverage                               # with coverage

# E2E
cd catalog-web
npm run test:e2e                                    # Playwright

# Android
cd catalogizer-android
./gradlew test                                      # unit tests

# All components
./scripts/run-all-tests.sh                          # everything
```

---

## Resource Limits

The host machine has resource constraints. Always limit test parallelism:

- **Go tests**: Use `GOMAXPROCS=3 -p 2 -parallel 2` to cap CPU and parallel packages
- **Node tests**: Vitest runs in a single process by default, which is fine
- **Total budget**: Max 4 CPUs, 8 GB RAM across all running processes

---

## Backend Tests (Go)

### Unit Tests

Unit test files sit alongside their source files with a `_test.go` suffix. Tests use the standard `testing` package with table-driven patterns.

```go
func TestServiceMethod(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        expected string
    }{
        {"valid input", "test", "result"},
        {"empty input", "", "default"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := service.Method(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

### Test Database

Use `database.WrapDB()` for in-memory SQLite test databases:

```go
func TestRepository(t *testing.T) {
    sqlDB, err := sql.Open("sqlite3", ":memory:")
    require.NoError(t, err)
    db := database.WrapDB(sqlDB, database.DialectSQLite)
    // run migrations, then test
}
```

The test helper at `internal/tests/test_helper.go` provides convenience functions for setting up test databases with migrations.

### Running Specific Tests

```bash
# Run a single test function
go test -v -run TestMediaDetection ./internal/media/detector/

# Run tests matching a pattern
go test -v -run "TestSMB.*" ./internal/services/

# Run with race detection
go test -race ./internal/services/
```

---

## Frontend Tests (Vitest)

Frontend tests use Vitest with React Testing Library.

```bash
cd catalog-web
npm run test              # single run
npm run test:watch        # re-run on file changes
npm run test:coverage     # generate coverage report
```

Test files use the `.test.ts` or `.test.tsx` extension and sit next to their source files or in `__tests__/` directories.

---

## End-to-End Tests (Playwright)

E2E tests verify the full stack from the browser through the API to the database.

```bash
cd catalog-web
npm run test:e2e
```

The test stack is defined in `docker-compose.test.yml` with three services:
- `catalog-api` -- backend
- `catalog-web` -- frontend
- `playwright` -- test runner

All services use `network_mode: host` for simplified connectivity.

---

## Security Tests

```bash
# Go dependency vulnerability check
cd catalog-api && govulncheck ./...

# Frontend dependency audit
cd catalog-web && npm audit --production

# Full security scan suite
./scripts/security-test.sh

# Containerized security tools
podman-compose -f docker-compose.security.yml up
```

---

## Challenge Framework

Catalogizer includes a challenge framework that validates the entire system end-to-end. Challenges are registered in `catalog-api/challenges/register.go` and exposed via `/api/v1/challenges`.

### Running Challenges

Challenges run through the API, never via external scripts:

```bash
# Run all challenges (blocking, sequential)
curl -X POST http://localhost:8080/api/v1/challenges/run-all \
  -H "Authorization: Bearer <token>"

# Run a specific challenge
curl -X POST http://localhost:8080/api/v1/challenges/run/CH-001 \
  -H "Authorization: Bearer <token>"

# Check status
curl http://localhost:8080/api/v1/challenges/status \
  -H "Authorization: Bearer <token>"
```

### Challenge Categories

| Range | Category | Description |
|-------|----------|-------------|
| CH-001 to CH-050+ | Core system challenges | Connectivity, scanning, security, performance, resilience, stress, integration |
| UF-API-* | API user flow challenges | HTTP endpoint validation across all API routes |
| UF-WEB-* | Web user flow challenges | Browser-based flows via Playwright |
| UF-DESKTOP-* | Desktop user flow challenges | Tauri desktop and installer wizard flows |
| UF-MOBILE-* | Mobile user flow challenges | Android and Android TV flows via ADB |
| MOD-* | Module verification challenges | Per-module functional validation |
| **Total** | **557+ challenges** | Stress, integration, security, documentation completeness |

### Writing a Challenge

Challenges embed `challenge.BaseChallenge` and implement an `Execute()` method:

```go
type MyChallenge struct {
    challenge.BaseChallenge
}

func (c *MyChallenge) Execute(ctx context.Context) error {
    // test logic with assertions
    return nil
}
```

Register challenges in `challenges/register.go` via the `RegisterAll()` function.

### Key Constraints

- `RunAll` is synchronous and blocking -- no other challenge can execute concurrently
- Progress-based liveness detection kills stuck challenges after 5 minutes of no progress
- `challenge.NewConfig()` defaults to a 5-minute timeout -- set it to zero to use the runner's timeout
- `config.json` `write_timeout` must be `900` for long-running challenge sets

---

## Stress Tests (k6)

Catalogizer includes k6 load testing scripts in `tests/k6/` for performance validation.

| Script | Scenario | Description |
|--------|----------|-------------|
| `load_test.js` | Load test | Ramp to 50 virtual users, verify p95 < 500ms |
| `stress_test.js` | Stress test | Ramp to 300 virtual users, find the breaking point |
| `soak_test.js` | Soak test | 20 users for 30 minutes, detect memory leaks |
| `spike_test.js` | Spike test | Sudden traffic burst to test recovery |

Run via containers:

```bash
podman run --rm --network host \
  -v $(pwd)/tests/k6:/scripts \
  docker.io/grafana/k6:latest run /scripts/load_test.js
```

---

## Writing Tests

### Conventions

- Place test files next to the code they test
- Use table-driven tests in Go
- Use descriptive test names: `TestService_Method_WhenCondition_ExpectedOutcome`
- Mock external dependencies (network, filesystem, providers)
- Clean up test resources (databases, temp files) in `t.Cleanup()` or `defer`

### What to Test

- Service layer business logic
- Repository SQL queries (using in-memory SQLite)
- Handler request/response formatting
- Middleware behavior (auth, rate limiting, CORS)
- Detection pipeline accuracy
- Entity hierarchy construction
