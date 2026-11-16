# Module 10: Testing and Quality - Script

**Duration**: 45 minutes
**Module**: 10 - Testing and Quality

---

## Scene 1: Unit Testing Patterns (0:00 - 15:00)

**[Visual: Test coverage summary: Go 38/38 packages, Frontend 101 files/1623 tests, Installer 19 files/178 tests]**

**Narrator**: Welcome to Module 10. Catalogizer maintains zero test failures across all platforms. Go backend: 38 packages with zero race conditions. Frontend: 101 test files with 1623 tests. Installer wizard: 19 files with 178 tests. Let us examine how this level of quality is achieved.

**[Visual: Open a Go table-driven test file]**

**Narrator**: Go tests follow the table-driven pattern. Each test case is a struct with a name, inputs, and expected outputs. A single test function iterates over all cases, running each as a subtest. This pattern is used throughout the codebase.

```go
// catalog-api/database/dialect_test.go
func TestRewritePlaceholders(t *testing.T) {
    tests := []struct {
        name     string
        dialect  DialectType
        input    string
        expected string
    }{
        {
            name:     "sqlite passthrough",
            dialect:  DialectSQLite,
            input:    "SELECT * FROM files WHERE id = ?",
            expected: "SELECT * FROM files WHERE id = ?",
        },
        {
            name:     "postgres single placeholder",
            dialect:  DialectPostgres,
            input:    "SELECT * FROM files WHERE id = ?",
            expected: "SELECT * FROM files WHERE id = $1",
        },
        {
            name:     "postgres multiple placeholders",
            dialect:  DialectPostgres,
            input:    "INSERT INTO files (name, size) VALUES (?, ?)",
            expected: "INSERT INTO files (name, size) VALUES ($1, $2)",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            d := &Dialect{Type: tt.dialect}
            result := d.RewritePlaceholders(tt.input)
            assert.Equal(t, tt.expected, result)
        })
    }
}
```

**[Visual: Show test helper for database setup]**

**Narrator**: The test helper in `internal/tests/test_helper.go` provides a `SetupTestDB` function that creates an in-memory SQLite database wrapped with the dialect-aware `database.DB`. This gives every test a clean, isolated database with zero external dependencies.

```go
// catalog-api/internal/tests/test_helper.go
func SetupTestDB(t *testing.T) *database.DB {
    sqlDB, err := sql.Open("sqlite3", ":memory:")
    require.NoError(t, err)
    db := database.WrapDB(sqlDB, database.DialectSQLite)
    // Run migrations
    require.NoError(t, db.RunMigrations(context.Background()))
    return db
}
```

**[Visual: Show mock vs real database in tests]**

**Narrator**: Catalogizer prefers real in-memory SQLite over mocks for database tests. The dialect abstraction means the same SQL runs against both SQLite and PostgreSQL. In-memory SQLite tests run in milliseconds and catch real SQL bugs that mocks would miss.

**[Visual: Show fuzz tests]**

**Narrator**: The codebase also includes fuzz tests. `dialect_fuzz_test.go` fuzzes the SQL rewriter with random inputs to find edge cases. `title_parser_fuzz_test.go` fuzzes the media title parser. `factory_fuzz_test.go` fuzzes the protocol factory. Fuzz tests catch inputs that unit tests miss.

```go
// catalog-api/database/dialect_fuzz_test.go
func FuzzRewritePlaceholders(f *testing.F) {
    f.Add("SELECT * FROM ? WHERE id = ?")
    f.Add("")
    f.Add("'don''t touch this ?'")
    f.Fuzz(func(t *testing.T, query string) {
        d := &Dialect{Type: DialectPostgres}
        result := d.RewritePlaceholders(query)
        // Should not panic, should not produce empty output for non-empty input
        if query != "" && result == "" {
            t.Errorf("empty result for non-empty input: %q", query)
        }
    })
}
```

**[Visual: Show resource-limited test execution]**

**Narrator**: Tests run with strict resource limits. Go tests use `GOMAXPROCS=3`, 2 parallel packages, and 2 parallel tests per package. This ensures the test suite does not consume more than 30-40% of host resources.

```bash
GOMAXPROCS=3 go test ./... -p 2 -parallel 2
```

---

## Scene 2: Integration Testing (15:00 - 30:00)

**[Visual: Show `docker-compose.test.yml` configuration]**

**Narrator**: Integration tests use a container stack defined in `docker-compose.test.yml`. Three containers run on the host network: the catalog-api, catalog-web, and a Playwright container for browser automation.

**[Visual: Show API contract testing pattern]**

**Narrator**: API contract tests verify that every endpoint the frontend calls exists, returns valid responses, and matches the expected shape. This is the Zero Warning / Zero Error policy: no console errors, no failed network requests, no missing endpoints.

**[Visual: Show integration test files]**

**Narrator**: Integration tests live in `services/services_integration_test.go` and `internal/services/services_integration_test.go`. These tests spin up the full service stack with a real database and test complete workflows: create a storage root, trigger a scan, wait for completion, and verify entities were created.

**[Visual: Show database fixtures]**

**Narrator**: Test fixtures seed the database with known data. A fixture might create a storage root, insert specific files, and then verify that the aggregation service produces the correct entities. Fixtures are defined inline in the test, not in separate files, keeping the test self-contained.

**[Visual: Show end-to-end flow test]**

**Narrator**: End-to-end tests exercise the full stack. A typical test: POST to `/api/v1/auth/login`, save the token, POST to create a storage root, POST to start a scan, poll until complete, GET the file list, GET the entity list, and verify counts and types.

**[Visual: Show Playwright E2E tests]**

**Narrator**: Frontend E2E tests use Playwright to automate a real browser. They click through the UI, fill forms, trigger actions, and verify visual state. The test stack runs in containers with `network_mode: host` so the browser can reach both the API and the web server.

---

## Scene 3: Challenge System (30:00 - 45:00)

**[Visual: Challenge system architecture diagram]**

**Narrator**: The challenge system is Catalogizer's unique quality assurance framework. 209 challenges -- 35 original challenges plus 174 user flow challenges -- verify the entire system end-to-end with 406 assertions. All 209 pass.

**[Visual: Show challenge registration in `catalog-api/challenges/register.go`]**

**Narrator**: Challenges are registered in `register.go`. The `RegisterAll` function loads endpoint configuration, creates challenge instances for each configured storage root, and registers browsing, asset, and populate challenges.

```go
// catalog-api/challenges/register.go
func RegisterAll(svc *services.ChallengeService) error {
    cfg, err := LoadEndpointConfig(DefaultConfigPath())
    if err != nil {
        if os.IsNotExist(err) { return nil }
        return nil
    }

    for _, ep := range cfg.Endpoints {
        svc.Register(NewSMBConnectivityChallenge(&endpoint))
        svc.Register(NewDirectoryDiscoveryChallenge(&endpoint))
        // ... per-directory content type challenges
    }

    svc.Register(NewFirstCatalogPopulateChallenge())
    svc.Register(NewBrowsingAPIHealthChallenge())
    svc.Register(NewBrowsingAPICatalogChallenge())
    // ...
}
```

**[Visual: Show challenge struct pattern]**

**Narrator**: Each challenge is a Go struct embedding `challenge.BaseChallenge` from the Challenges submodule. The `Execute()` method contains the test logic. Challenges interact with the running API through HTTP calls -- never through direct database access or internal function calls.

**[Visual: Show the Challenges submodule structure]**

**Narrator**: The `Challenges/` submodule provides the generic framework. The `pkg/userflow/` package defines multi-platform automation with adapter interfaces for Browser, Mobile, Desktop, API, Build, and Process operations.

**[Visual: Show userflow challenge distribution]**

**Narrator**: 174 user flow challenges span four platforms:

- 49 API challenges in `userflow_api.go` -- HTTP endpoint verification
- 59 Web challenges in `userflow_web.go` -- Playwright browser automation
- 28 Desktop challenges in `userflow_desktop.go` -- Tauri desktop + wizard
- 38 Mobile challenges in `userflow_mobile.go` -- Android + Android TV

**[Visual: Show challenge execution constraints]**

**Narrator**: Critical constraint: `RunAll` is synchronous and blocking. No other challenge can run until it finishes. A 5-minute stale threshold kills stuck challenges. Progress reporting happens every 5 seconds. The entire suite must be executed through the running catalog-api service -- never through scripts or curl commands.

**[Visual: Show challenge API endpoints]**

**Narrator**: Challenges are exposed via REST at `/api/v1/challenges`. You can list all challenges, run a specific one by ID, run all challenges, and retrieve results. The challenge bank definitions live in `challenges/config/`.

**[Visual: Show the CLI runner]**

**Narrator**: The CLI runner at `Challenges/cmd/userflow-runner/` provides command-line execution with flags for platform, report format, compose file, root directory, timeout, and output directory.

```bash
# Run all API challenges
cd Challenges && go run cmd/userflow-runner/main.go --platform api --root /path/to/catalogizer

# Run web challenges with Playwright
go run cmd/userflow-runner/main.go --platform web --compose docker-compose.test.yml
```

**[Visual: Course title card]**

**Narrator**: Testing at this scale requires discipline: table-driven unit tests, in-memory database fixtures, container-based integration tests, Playwright E2E automation, and 209 challenge verifications. The Zero Warning / Zero Error policy ensures no regression goes undetected. In Module 11, we harden the system with security tools and monitoring.

---

## Key Code Examples

### Running All Tests
```bash
# Backend (resource-limited)
cd catalog-api
GOMAXPROCS=3 go test ./... -p 2 -parallel 2

# Frontend
cd catalog-web
npm run test           # 1623 tests, single run
npm run test:coverage  # with coverage report

# Installer wizard
cd installer-wizard
npm run test           # 178 tests

# E2E
cd catalog-web
npm run test:e2e       # Playwright
```

### Challenge Configuration
```json
// challenges/config/endpoints.json
{
  "endpoints": [
    {
      "name": "NAS",
      "protocol": "smb",
      "host": "nas.local",
      "directories": [
        { "path": "/music", "content_type": "music" },
        { "path": "/movies", "content_type": "movie" },
        { "path": "/tv", "content_type": "tv_show" }
      ]
    }
  ]
}
```

### Test Summary
```
Go:        38/38 packages, 0 race conditions
Frontend:  101/101 files, 1623 tests, 0 failures
Installer: 19/19 files, 178 tests, 0 failures
Challenges: 209/209 PASSED, 406/406 assertions
Security:  govulncheck 0 vulns, npm audit 0 critical
```

---

## Quiz Questions

1. Why does Catalogizer prefer in-memory SQLite over mocks for database tests?
   **Answer**: In-memory SQLite runs real SQL queries through the dialect abstraction layer, catching actual SQL bugs (syntax errors, incorrect joins, missing columns) that mocks would miss. Tests run in milliseconds because SQLite in-memory is fast. The dialect abstraction ensures the same SQL works on PostgreSQL, so SQLite tests are a valid proxy for production behavior.

2. What is the Zero Warning / Zero Error policy?
   **Answer**: All components must run with zero console warnings, zero console errors, and zero failed network requests in every environment. Every API endpoint the frontend calls must exist and return valid responses. No framework deprecation warnings. No WebSocket connection failures. If a feature is not implemented, stub endpoints return valid empty responses. The 209-challenge suite enforces this end-to-end.

3. How does the challenge system differ from traditional integration tests?
   **Answer**: Challenges execute against the running system through HTTP API calls and browser automation -- exactly as an end user would. They never access databases directly or call internal functions. They verify the complete stack: API, frontend, desktop, and mobile. Traditional integration tests often bypass the HTTP layer. Challenges also have progress reporting, timeout handling, and a bank/registry pattern.

4. What are the resource constraints for running the Go test suite?
   **Answer**: `GOMAXPROCS=3` limits Go to 3 OS threads. `-p 2` runs at most 2 packages in parallel. `-parallel 2` limits per-package test parallelism to 2. This ensures tests use no more than 30-40% of host resources, preventing system freezes on the development machine.

---

## Addendum: Stress Tests, Integration Tests, and the 517-Case Test Bank

**[Visual: Test pyramid diagram with unit tests at the base, integration in the middle, stress/load at the top, and the 517-case test bank spanning all levels]**

**Narrator**: Beyond the unit tests and challenge system covered earlier, Catalogizer includes dedicated stress tests, comprehensive integration tests, and a structured test bank containing 517 test case definitions across multiple domains. Let us examine each layer.

### Stress Tests

**[Visual: Terminal output from a k6 stress test showing ramp-up to 300 virtual users]**

**Narrator**: Stress tests live in `tests/k6/` and use the k6 load testing framework. Three test scripts target different scenarios:

```bash
# Load test: ramp to 50 users, verify p95 < 500ms
podman run --rm --network host \
  -v $(pwd)/tests/k6:/scripts docker.io/grafana/k6:latest \
  run /scripts/load_test.js

# Stress test: ramp to 300 users, find breaking point
podman run --rm --network host \
  -v $(pwd)/tests/k6:/scripts docker.io/grafana/k6:latest \
  run /scripts/stress_test.js

# Soak test: 20 users for 30 minutes, detect memory leaks
podman run --rm --network host \
  -v $(pwd)/tests/k6:/scripts docker.io/grafana/k6:latest \
  run /scripts/soak_test.js
```

**Narrator**: The load test verifies that the API meets its SLO: p95 response time under 500ms with 50 concurrent users. The stress test ramps to 300 users to find the breaking point -- where error rates exceed 1%. The soak test runs for 30 minutes at moderate load to detect memory leaks and goroutine leaks by comparing memory metrics at start and end.

**[Visual: k6 results dashboard showing thresholds]**

**Narrator**: Key thresholds enforced by the tests:

| Test | Metric | Threshold |
|------|--------|-----------|
| Load | p95 response time | < 500ms |
| Load | Error rate | < 1% |
| Stress | Requests/second at peak | > 200 |
| Soak | Memory growth over 30 min | < 50MB |
| Soak | Goroutine count delta | < 10 |

### Integration Tests

**[Visual: Show `services_integration_test.go` test file structure]**

**Narrator**: Integration tests verify complete service workflows with a real database. They live in `services/services_integration_test.go` and `internal/services/services_integration_test.go`. Each test creates a fresh in-memory SQLite database, seeds it with known data, and exercises the full service stack.

```go
func TestScanAndAggregateWorkflow(t *testing.T) {
    db := tests.SetupTestDB(t)
    fileRepo := repository.NewFileRepository(db)
    entityRepo := repository.NewMediaItemRepository(db)
    scanSvc := services.NewScanService(fileRepo, db)
    aggSvc := services.NewAggregationService(entityRepo, fileRepo, db)

    // 1. Create a storage root
    root, err := scanSvc.CreateStorageRoot(ctx, rootRequest)
    require.NoError(t, err)

    // 2. Insert files simulating a scan
    for _, file := range testFiles {
        _, err := fileRepo.InsertFile(ctx, root.ID, file)
        require.NoError(t, err)
    }

    // 3. Run aggregation
    err = aggSvc.AggregateAfterScan(ctx, root.ID)
    require.NoError(t, err)

    // 4. Verify entities were created
    entities, err := entityRepo.GetAll(ctx, 100, 0)
    require.NoError(t, err)
    assert.GreaterOrEqual(t, len(entities), 5)
}
```

**Narrator**: Integration tests cover workflows that span multiple services: scan-then-aggregate, create-user-then-authenticate, convert-then-download, and admin-backup-then-restore. They catch inter-service contract violations that unit tests miss.

### The 517-Case Test Bank

**[Visual: Table showing test bank files and case counts]**

**Narrator**: The test bank is a structured collection of 517 test case definitions organized by domain. These are not executable tests -- they are YAML/JSON specifications that define inputs, expected outputs, and validation criteria. The challenge system and integration tests consume these definitions.

| Domain | File | Cases | Description |
|--------|------|-------|-------------|
| Entity detection | `testbank_entity.yaml` | 127 | Title parsing, media type detection, hierarchy building |
| Admin operations | `testbank_admin.yaml` | 68 | User management, backup/restore, system info |
| Security | `testbank_security.yaml` | 94 | Auth flows, permission checks, input validation, injection |
| Performance | `testbank_performance.yaml` | 52 | Response time, throughput, connection pool behavior |
| Conversion | `testbank_conversion.yaml` | 43 | Format support, quality presets, error handling |
| API contracts | `testbank_api.yaml` | 89 | Endpoint existence, response shapes, status codes |
| Search | `testbank_search.yaml` | 44 | Full-text search, filters, pagination, sorting |

```yaml
# Example test case from testbank_entity.yaml
- id: ENT-042
  name: "TV show hierarchy detection"
  input:
    filename: "Breaking Bad S01E01 Pilot.mkv"
    path: "/tv/Breaking Bad/Season 1/"
  expected:
    media_type: tv_episode
    title: "Pilot"
    series: "Breaking Bad"
    season: 1
    episode: 1
    hierarchy:
      - type: tv_show
        title: "Breaking Bad"
      - type: tv_season
        title: "Season 1"
        parent: "Breaking Bad"
```

**Narrator**: The test bank serves multiple purposes. First, it documents expected behavior in a human-readable format. Second, the challenge system's assertion engine reads these definitions to verify API responses. Third, the title parser's table-driven tests are generated from the entity test bank, ensuring consistency between the test bank specification and the actual unit tests.

**[Visual: Diagram showing test bank -> challenge assertions -> integration tests -> unit tests]**

**Narrator**: The 517 cases are not a replacement for the 1623 frontend tests or the 38 Go test packages. They are a specification layer that sits above executable tests. When a new media format or edge case is discovered, it is added to the test bank first, then the appropriate unit test and challenge are updated to cover it. This top-down approach ensures no test case exists without a specification, and no specification exists without a test.

**Key takeaways:**
- Stress tests run in containers via k6 with strict SLO thresholds.
- Integration tests use real in-memory databases for full workflow verification.
- The 517-case test bank is a specification layer consumed by challenges and unit tests.
- Test bank cases span 7 domains: entity, admin, security, performance, conversion, API contracts, and search.
