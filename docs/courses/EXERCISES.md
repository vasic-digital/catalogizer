# Catalogizer Video Course - Hands-On Exercises

This document contains practical exercises for each of the 6 core modules in the Catalogizer video course. Each exercise reinforces key concepts from the corresponding module with guided, hands-on tasks.

**Completion Requirement**: Complete all exercises with at least 80% completion to qualify for certification.

---

## Module 1 Exercises: Introduction and Installation

### Exercise 1.1: Install Catalogizer Locally

**Objective**: Set up a complete Catalogizer development environment and verify all services start correctly.

**Prerequisites**: Git, Go 1.24+, Node.js 18+, a terminal

**Steps**:

1. Clone the repository and initialize submodules:

   ```bash
   git clone <repository-url> Catalogizer
   cd Catalogizer
   scripts/safe_submodule_init.sh   # never --recursive through constitution/
   ```

2. Set up the backend. Navigate to the catalog-api directory and create a `.env` file:

   ```bash
   cd catalog-api
   cat > .env << 'EOF'
   PORT=8080
   GIN_MODE=debug
   DB_TYPE=sqlite
   JWT_SECRET=exercise-dev-secret-key
   ADMIN_PASSWORD=admin123
   EOF
   ```

3. Install Go dependencies and start the backend:

   ```bash
   go mod tidy
   go run main.go
   ```

4. Open a second terminal. Set up and start the frontend:

   ```bash
   cd catalog-web
   npm install
   npm run dev
   ```

5. Open a browser and navigate to `http://localhost:3000`. Verify the login page loads.

6. Log in with the default admin credentials (admin / admin123).

**Expected Result**:

- The backend starts and writes a `.service-port` file in `catalog-api/`
- The frontend starts on port 3000 and successfully proxies API requests to the backend
- The browser shows the Catalogizer dashboard after login
- No errors appear in either terminal

**Bonus Challenge**: Check the SQLite database file (`catalogizer.db`) was created in the `catalog-api` directory. Use `sqlite3 catalogizer.db ".tables"` to list all tables and confirm the schema was initialized.

---

### Exercise 1.2: Container-Based Installation

**Objective**: Deploy Catalogizer using Podman Compose and verify all container services are healthy.

**Prerequisites**: Podman 4.0+ and podman-compose installed

**Steps**:

1. From the project root, start the development environment:

   ```bash
   podman-compose -f docker-compose.dev.yml up -d
   ```

2. Verify all containers are running:

   ```bash
   podman ps --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
   ```

3. Check container health:

   ```bash
   podman stats --no-stream
   ```

4. Open `http://localhost:3000` in a browser and verify the application loads.

5. Stop the environment:

   ```bash
   podman-compose -f docker-compose.dev.yml down
   ```

**Expected Result**:

- All containers start without errors
- `podman ps` shows healthy status for all services
- Resource usage stays within configured limits (max 4 CPUs, 8 GB RAM total)
- The web interface loads and connects to the backend

**Bonus Challenge**: Validate the production compose file without starting it: `podman-compose -f docker-compose.yml config --quiet`. Fix any validation errors you find.

---

## Module 2 Exercises: Getting Started with Media Management

### Exercise 2.1: Connect a Storage Source

**Objective**: Add a local filesystem storage root and trigger a scan of media files.

**Prerequisites**: Module 1 completed, Catalogizer running, a directory containing at least 10 media files (videos, music, images)

**Steps**:

1. Log in to Catalogizer at `http://localhost:3000`.

2. Navigate to the storage configuration page.

3. Add a new storage root:
   - Protocol: Local Filesystem
   - Path: Enter the absolute path to your media directory (e.g., `/home/user/Media`)
   - Name: Give it a descriptive label (e.g., "Local Media")

4. Save the storage root configuration.

5. Trigger a scan of the newly added storage root.

6. Monitor the scan progress. Observe the real-time updates as files are discovered and categorized.

7. Verify the scan results by checking the dashboard for updated media counts.

**Expected Result**:

- The storage root appears in the storage configuration list
- Scanning begins and progress is reported in real time
- The dashboard shows updated counts for total media items
- Scanned files appear in the media browser with detected types (movie, music, etc.)

**Bonus Challenge**: Use the API directly to add a storage root. Send a POST request to `/api/v1/storage-roots` with JSON body containing the path and protocol. Compare the API response with what you see in the web interface.

---

### Exercise 2.2: Browse and Search Media

**Objective**: Navigate the media browser, apply filters, and use the search functionality to find specific content.

**Prerequisites**: Exercise 2.1 completed, media has been scanned

**Steps**:

1. Open the Media Browser from the main navigation.

2. Switch between Grid view and List view. Note the differences in information density.

3. Apply a type filter to show only video files. Then clear it and filter for audio files.

4. Sort the results by file size (largest first), then by date (newest first).

5. Click on any media item to open the detail modal. Review the detected metadata: file type, size, resolution (for video), codec information, and any external metadata from providers.

6. Navigate to the Search page. Enter a search query for a known media title in your collection.

7. Use the advanced search filters:
   - Filter by media type (e.g., movies only)
   - Filter by file size range
   - Combine multiple filters

8. Review the search results and verify they match your query.

**Expected Result**:

- Grid and List views both render correctly with proper media information
- Type filters correctly narrow the displayed results
- Sort ordering changes the display order as expected
- Media detail modal shows file metadata and any enriched external data
- Search returns relevant results matching the query
- Advanced filters narrow results appropriately

**Bonus Challenge**: Open the browser developer tools (F12) and watch the Network tab while performing searches. Identify the API endpoints being called (`/api/v1/media/search`), the query parameters, and the response structure.

---

## Module 3 Exercises: Advanced Media Features

### Exercise 3.1: Create a Collection

**Objective**: Create a manual collection, add media items to it, and configure collection settings.

**Prerequisites**: Module 2 completed, media items exist in the catalog

**Steps**:

1. Navigate to the Collections page from the main navigation.

2. Click "Create Collection" and fill in the details:
   - Name: "Favorite Movies"
   - Type: Manual
   - Visibility: Private

3. Save the collection.

4. Navigate to the Media Browser. Select several media items using the checkbox selection.

5. Use the "Add to Collection" action to add the selected items to your "Favorite Movies" collection.

6. Return to the Collections page and open your new collection. Verify all added items appear.

7. Try reordering items within the collection using drag-and-drop (if supported in your view mode).

**Expected Result**:

- The collection is created and appears in the Collections list
- Selected media items are successfully added to the collection
- Opening the collection shows all added items
- Collection metadata (name, type, visibility) displays correctly

**Bonus Challenge**: Create a Smart Collection with automatic filter rules. For example, create a Smart Collection that automatically includes all video files larger than 1 GB. Verify that matching media items are automatically included.

---

### Exercise 3.2: Manage Favorites and Create a Playlist

**Objective**: Add items to favorites, create a playlist, and organize media for playback.

**Prerequisites**: Exercise 3.1 completed

**Steps**:

1. Navigate to the Media Browser. Click the favorite (heart/star) icon on at least 5 different media items.

2. Navigate to the Favorites page. Verify all favorited items appear.

3. Export your favorites:
   - Click the Export button
   - Choose JSON format
   - Save the exported file

4. Open the exported JSON file and examine its structure. Note the metadata included for each item.

5. Navigate to the Playlists page. Create a new playlist:
   - Name: "Evening Playlist"
   - Add at least 3 audio or video items

6. Reorder the playlist items using drag-and-drop.

7. If the built-in media player is available, play the first item from your playlist.

**Expected Result**:

- Favorited items show the active favorite indicator
- The Favorites page lists all items marked as favorites
- Exported JSON contains valid data with item metadata
- The playlist is created with items in the specified order
- Drag-and-drop reordering updates the playlist order
- Media player opens and plays the selected item (if available)

**Bonus Challenge**: Export your favorites in CSV format as well. Compare the JSON and CSV exports. Then clear all favorites and re-import them from the JSON export file.

---

## Module 4 Exercises: Multi-Platform Experience

### Exercise 4.1: Install and Test the Mobile App

**Objective**: Build the Android app, install it on a device or emulator, and verify it connects to the Catalogizer backend.

**Prerequisites**: Module 2 completed, Android Studio installed, Android SDK 34, JDK 17

**Steps**:

1. Open the Android project in Android Studio:

   ```bash
   cd catalogizer-android
   ```

2. Wait for Gradle sync to complete. Check that all dependencies resolve.

3. Build the debug APK:

   ```bash
   ./gradlew assembleDebug
   ```

4. Install on a connected device or emulator:

   ```bash
   ./gradlew installDebug
   ```

5. Launch the Catalogizer app on the device.

6. Configure the server URL to point to your running Catalogizer backend:
   - If using an emulator: `http://10.0.2.2:8080`
   - If using a physical device on the same network: `http://<your-ip>:8080`

7. Log in with your admin credentials.

8. Browse the catalog, search for media, and add items to favorites from the mobile app.

**Expected Result**:

- The APK builds without errors
- The app installs and launches on the device/emulator
- Server connection is established successfully
- The catalog displays the same media as the web interface
- Search and favorites work correctly from the mobile app

**Bonus Challenge**: Build and test the Android TV app (`catalogizer-androidtv`) on an Android TV emulator. Compare the TV interface with the mobile interface and note the differences in navigation and layout.

---

### Exercise 4.2: Set Up the Desktop App and Test the API Client

**Objective**: Run the Tauri desktop application in development mode and execute API client library tests.

**Prerequisites**: Module 2 completed, Rust toolchain installed, Node.js 18+

**Steps**:

1. Set up and launch the desktop app:

   ```bash
   cd catalogizer-desktop
   npm install
   npm run tauri:dev
   ```

2. Wait for the Tauri application window to open. Verify the desktop app loads the Catalogizer interface.

3. Test basic functionality: login, browse media, search.

4. Close the desktop app and navigate to the API client:

   ```bash
   cd catalogizer-api-client
   npm install
   ```

5. Build the API client library:

   ```bash
   npm run build
   ```

6. Run the client test suite:

   ```bash
   npm run test
   ```

7. Examine the test output and verify all tests pass.

**Expected Result**:

- The desktop app window opens with the Catalogizer interface
- Login and browsing work in the desktop app
- The API client builds without TypeScript errors
- All API client tests pass
- Test output shows the number of passing tests and coverage information

**Bonus Challenge**: Write a small script using the API client to authenticate and list all media items programmatically:

```typescript
import { CatalogizerClient } from '@vasic-digital/catalogizer-api-client';

const client = new CatalogizerClient({ baseUrl: 'http://localhost:8080' });
await client.auth.login('admin', 'admin123');
const media = await client.media.list();
console.log(`Total media items: ${media.length}`);
```

---

## Module 5 Exercises: Administration and Configuration

### Exercise 5.1: Manage Users and Configure Security

**Objective**: Create user accounts with different roles, configure JWT settings, and run a security scan.

**Prerequisites**: Module 2 completed, admin access

**Steps**:

1. Log in to Catalogizer as admin.

2. Navigate to the Admin Panel.

3. Create two new user accounts:
   - User 1: username "viewer", role "viewer" (read-only access)
   - User 2: username "editor", role "editor" (can create collections and manage favorites)

4. Log out and log in as the "viewer" user. Verify:
   - Can browse media and search
   - Cannot create collections or modify settings

5. Log out and log in as the "editor" user. Verify:
   - Can browse media, search, create collections, and add favorites
   - Cannot access the Admin Panel

6. Log back in as admin. Run the security scanning tools:

   ```bash
   cd catalog-api
   # Run Go vulnerability check
   govulncheck ./...

   cd ../catalog-web
   # Run npm audit
   npm audit --production
   ```

7. Review the scan results and note any findings.

**Expected Result**:

- Both user accounts are created successfully
- Role-based access control enforces the correct permissions for each role
- The "viewer" cannot perform write operations
- The "editor" can create collections but cannot access admin functions
- `govulncheck` reports 0 vulnerabilities
- `npm audit` reports 0 critical production vulnerabilities

**Bonus Challenge**: Modify the JWT expiry configuration in the `.env` file. Set `JWT_EXPIRY_HOURS=1` and `REFRESH_TOKEN_EXPIRY_HOURS=24`. Restart the backend and verify tokens expire after 1 hour by checking the token claims.

---

### Exercise 5.2: Set Up Monitoring

**Objective**: Configure Prometheus metrics collection and verify the metrics endpoint.

**Prerequisites**: Exercise 5.1 completed, Catalogizer running

**Steps**:

1. Verify the metrics endpoint is exposed by the backend:

   ```bash
   curl -s http://localhost:8080/metrics | head -20
   ```

2. Examine the available metrics. Look for:
   - HTTP request duration metrics
   - Request count metrics
   - Go runtime metrics (goroutines, memory)

3. Review the Prometheus configuration:

   ```bash
   cat monitoring/prometheus.yml
   ```

4. If Podman is available, start the monitoring stack:

   ```bash
   podman run -d --name prometheus \
     --network host \
     -v $(pwd)/monitoring/prometheus.yml:/etc/prometheus/prometheus.yml:ro \
     --cpus=1 --memory=2g \
     docker.io/prom/prometheus:latest
   ```

5. Access Prometheus at `http://localhost:9090` and run a query:
   - Query: `up` (shows which targets are being scraped)
   - Query: `go_goroutines` (shows active goroutines in the API)

6. Stop the monitoring container:

   ```bash
   podman stop prometheus && podman rm prometheus
   ```

**Expected Result**:

- The `/metrics` endpoint returns Prometheus-format metrics
- Metrics include HTTP request data, Go runtime stats, and custom application metrics
- Prometheus successfully scrapes the Catalogizer backend
- Queries return valid data in the Prometheus UI

**Bonus Challenge**: Create a simple Grafana dashboard. Run Grafana in a container, connect it to your Prometheus instance, and create a panel showing HTTP request rate over time.

---

## Module 6 Exercises: Developer Guide and API

### Exercise 6.1: Set Up the Development Environment

**Objective**: Configure a complete development environment with backend and frontend running simultaneously, verify the test suite passes.

**Prerequisites**: Go 1.24+, Node.js 18+, Git

**Steps**:

1. Clone and set up the repository (if not already done):

   ```bash
   cd Catalogizer
   scripts/safe_submodule_init.sh   # never --recursive through constitution/
   ```

2. Start the backend in development mode:

   ```bash
   cd catalog-api
   go mod tidy
   go run main.go
   ```

3. In a second terminal, start the frontend:

   ```bash
   cd catalog-web
   npm install
   npm run dev
   ```

4. Verify the frontend proxies to the backend by opening `http://localhost:3000` and checking that API calls succeed (check the browser Network tab).

5. Run the backend test suite with resource limits:

   ```bash
   cd catalog-api
   GOMAXPROCS=3 go test ./... -p 2 -parallel 2
   ```

6. Run the frontend test suite:

   ```bash
   cd catalog-web
   npm run test
   ```

7. Run linting and type checking on the frontend:

   ```bash
   cd catalog-web
   npm run lint
   npm run type-check
   ```

**Expected Result**:

- Backend starts and writes `.service-port` file
- Frontend reads `.service-port` and proxies API requests correctly
- All Go tests pass (38 packages, 0 failures, 0 race conditions)
- All frontend tests pass (101 files, 1623 tests)
- Linting reports no errors
- Type checking reports no errors

**Bonus Challenge**: Run the Go tests with the race detector enabled: `GOMAXPROCS=3 go test -race ./... -p 2 -parallel 2`. Verify 0 race conditions are detected.

---

### Exercise 6.2: Add a New API Endpoint

**Objective**: Create a new REST API endpoint following the Handler-Service-Repository pattern used throughout the codebase.

**Prerequisites**: Exercise 6.1 completed, familiarity with Go

**Steps**:

1. Study the existing pattern. Open and read these files:

   ```bash
   # Handler example
   cat catalog-api/handlers/media_entity_handler.go | head -50

   # Service example
   cat catalog-api/services/auth_service.go | head -50

   # Route registration
   cat catalog-api/main.go | grep -A 5 "api/v1"
   ```

2. Create a new handler file `catalog-api/handlers/health_check_handler.go`:

   ```go
   package handlers

   import (
       "net/http"
       "runtime"
       "time"

       "github.com/gin-gonic/gin"
   )

   type HealthCheckHandler struct {
       startTime time.Time
   }

   func NewHealthCheckHandler() *HealthCheckHandler {
       return &HealthCheckHandler{
           startTime: time.Now(),
       }
   }

   func (h *HealthCheckHandler) DetailedHealth(c *gin.Context) {
       var memStats runtime.MemStats
       runtime.ReadMemStats(&memStats)

       c.JSON(http.StatusOK, gin.H{
           "status":      "healthy",
           "uptime":      time.Since(h.startTime).String(),
           "goroutines":  runtime.NumGoroutine(),
           "memory_mb":   memStats.Alloc / 1024 / 1024,
           "go_version":  runtime.Version(),
           "num_cpu":     runtime.NumCPU(),
       })
   }
   ```

3. Register the route in `main.go` under the `/api/v1` group (study where existing routes are registered and add yours nearby).

4. Write a test file `catalog-api/handlers/health_check_handler_test.go`:

   ```go
   package handlers

   import (
       "encoding/json"
       "net/http"
       "net/http/httptest"
       "testing"

       "github.com/gin-gonic/gin"
   )

   func TestDetailedHealth(t *testing.T) {
       gin.SetMode(gin.TestMode)
       handler := NewHealthCheckHandler()

       router := gin.New()
       router.GET("/api/v1/health/detailed", handler.DetailedHealth)

       req := httptest.NewRequest(http.MethodGet, "/api/v1/health/detailed", nil)
       w := httptest.NewRecorder()
       router.ServeHTTP(w, req)

       if w.Code != http.StatusOK {
           t.Errorf("expected status 200, got %d", w.Code)
       }

       var response map[string]interface{}
       if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
           t.Fatalf("failed to unmarshal response: %v", err)
       }

       if response["status"] != "healthy" {
           t.Errorf("expected status 'healthy', got '%v'", response["status"])
       }

       if _, ok := response["uptime"]; !ok {
           t.Error("expected 'uptime' field in response")
       }

       if _, ok := response["goroutines"]; !ok {
           t.Error("expected 'goroutines' field in response")
       }
   }
   ```

5. Run your test:

   ```bash
   cd catalog-api
   go test -v -run TestDetailedHealth ./handlers/
   ```

6. Start the server and test the endpoint manually:

   ```bash
   curl -s http://localhost:8080/api/v1/health/detailed | python3 -m json.tool
   ```

**Expected Result**:

- The handler compiles without errors
- The test passes with `ok` status
- The endpoint returns a JSON response with status, uptime, goroutines, memory, Go version, and CPU count:

  ```json
  {
      "status": "healthy",
      "uptime": "2m30.123456s",
      "goroutines": 12,
      "memory_mb": 15,
      "go_version": "go1.24",
      "num_cpu": 8
  }
  ```

**Bonus Challenge**: Extend the health check handler to include database connectivity status. Add a method that pings the database and reports whether it is reachable. Write a test for the new method using the test helper's `database.WrapDB()` for an in-memory SQLite database.

---

### Exercise 6.3: Run the Complete Test Suite

**Objective**: Execute the full test suite across all components and verify the zero-error policy.

**Prerequisites**: Exercise 6.1 completed

**Steps**:

1. Run the complete Go backend tests with resource limits:

   ```bash
   cd catalog-api
   GOMAXPROCS=3 go test ./... -p 2 -parallel 2 -count=1
   ```

2. Run the frontend tests:

   ```bash
   cd catalog-web
   npm run test
   ```

3. Run the frontend linter and type checker:

   ```bash
   cd catalog-web
   npm run lint && npm run type-check
   ```

4. Run Go security vulnerability check:

   ```bash
   cd catalog-api
   govulncheck ./...
   ```

5. Run npm production audit:

   ```bash
   cd catalog-web
   npm audit --production
   ```

6. Document the results in a summary:
   - Number of Go test packages passed
   - Number of frontend test files and individual tests passed
   - Number of security vulnerabilities found
   - Any warnings or errors encountered

**Expected Result**:

- Go: 38/38 packages pass, 0 failures, 0 race conditions
- Frontend: 101 test files, 1623 tests pass, 0 failures
- govulncheck: 0 vulnerabilities
- npm audit: 0 critical/production vulnerabilities
- Zero warnings, zero errors across all components

**Bonus Challenge**: Run the challenge system. Start the Catalogizer backend, then use the challenge API to execute the original 35 challenges (CH-001 through CH-035). Verify all 209 challenges pass with 406/406 assertions.

---

## Advanced Exercises

### Exercise 7.1: Create and Restore a Database Backup

**Objective**: Use the Backup Management API to create a database backup, list available backups, and restore from one.

**Prerequisites**: Module 5 completed, Catalogizer running, admin access

**Steps**:

1. Log in as admin and obtain a JWT token:

   ```bash
   TOKEN=$(curl -s -X POST http://localhost:8080/api/v1/auth/login \
     -H "Content-Type: application/json" \
     -d '{"username":"admin","password":"admin123"}' | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")
   ```

2. Create a database backup:

   ```bash
   curl -s -X POST http://localhost:8080/api/v1/admin/backup \
     -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
   ```

3. List all available backups:

   ```bash
   curl -s http://localhost:8080/api/v1/admin/backups \
     -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
   ```

4. Note the backup filename from the list. Restore from it:

   ```bash
   curl -s -X POST http://localhost:8080/api/v1/admin/backup/restore \
     -H "Authorization: Bearer $TOKEN" \
     -H "Content-Type: application/json" \
     -d '{"filename":"<backup-filename>"}' | python3 -m json.tool
   ```

5. Verify the restore succeeded by checking the dashboard data is intact.

**Expected Result**:

- Backup creation returns a success response with the backup filename
- Listing backups shows the newly created backup with timestamp and size
- Restore completes without errors and data remains consistent
- Attempting a second concurrent backup is blocked by the backup semaphore

**Bonus Challenge**: Try restoring with a path traversal filename (e.g., `../../etc/passwd`). Verify the API rejects it with a security error.

---

### Exercise 7.2: Run Security Scans

**Objective**: Execute security scans across backend and frontend components and interpret the results.

**Prerequisites**: Go toolchain, Node.js, Catalogizer source code

**Steps**:

1. Run Go vulnerability scanning:

   ```bash
   cd catalog-api
   govulncheck ./...
   ```

2. Run npm dependency audit on the frontend:

   ```bash
   cd catalog-web
   npm audit --production
   ```

3. If Podman is available, run Semgrep SAST scanning:

   ```bash
   podman-compose -f docker-compose.security.yml --profile semgrep-scan run --rm semgrep-scanner
   ```

4. Run the consolidated security scan script:

   ```bash
   ./scripts/security-scan.sh
   ```

5. Document findings: number of vulnerabilities by severity, affected packages, and recommended actions.

**Expected Result**:

- `govulncheck` reports 0 known vulnerabilities in Go dependencies
- `npm audit --production` reports 0 critical vulnerabilities in production dependencies
- Semgrep reports findings categorized by severity (error, warning, info)
- The consolidated scan script produces a summary report

**Bonus Challenge**: Run SonarQube analysis with `./scripts/run-sonarqube-scan.sh` and review the quality gate results for code smells, duplications, and coverage metrics.

---

### Exercise 7.3: Load Test the API with k6

**Objective**: Use k6 to run load tests against the Catalogizer API and verify performance under concurrent load.

**Prerequisites**: Podman installed, Catalogizer backend running

**Steps**:

1. Review the available k6 test scripts:

   ```bash
   ls tests/k6/
   ```

2. Run the standard load test (ramps to 50 virtual users):

   ```bash
   podman run --rm --network host \
     -v $(pwd)/tests/k6:/scripts:ro \
     --cpus=1 --memory=2g \
     docker.io/grafana/k6:latest run /scripts/load_test.js
   ```

3. Review the output. Check the `http_req_duration` p95 metric (target: under 500ms).

4. Run the stress test to find the breaking point:

   ```bash
   podman run --rm --network host \
     -v $(pwd)/tests/k6:/scripts:ro \
     --cpus=1 --memory=2g \
     docker.io/grafana/k6:latest run /scripts/stress_test.js
   ```

5. Monitor the backend during the test with `podman stats --no-stream` or `curl localhost:8080/metrics`.

**Expected Result**:

- The load test completes with p95 latency under 500ms for 50 users
- The stress test identifies the breaking point (where error rate exceeds acceptable threshold)
- Backend remains responsive and recovers after test completion
- No goroutine leaks visible in `/metrics` after test ends

**Bonus Challenge**: Run the soak test (`soak_test.js`) for 30 minutes with 20 users to check for memory leaks. Compare `go_memstats_alloc_bytes` at the start and end of the test.

---

### Exercise 7.4: Monitor Memory Usage with Prometheus Metrics

**Objective**: Set up memory monitoring and interpret Go runtime metrics to detect potential memory issues.

**Prerequisites**: Catalogizer running, Podman available

**Steps**:

1. Query current memory metrics from the API:

   ```bash
   curl -s http://localhost:8080/metrics | grep go_memstats
   ```

2. Note the key metrics:
   - `go_memstats_alloc_bytes` -- currently allocated heap memory
   - `go_memstats_sys_bytes` -- total memory obtained from the OS
   - `go_goroutines` -- number of active goroutines

3. Start Prometheus to collect metrics over time:

   ```bash
   podman run -d --name prom-memory \
     --network host \
     -v $(pwd)/monitoring/prometheus.yml:/etc/prometheus/prometheus.yml:ro \
     --cpus=1 --memory=2g \
     docker.io/prom/prometheus:latest
   ```

4. Trigger some load (browse the catalog, run a scan, search).

5. Query memory trends in Prometheus at `http://localhost:9090`:
   - `rate(go_memstats_alloc_bytes_total[5m])` -- allocation rate
   - `go_goroutines` -- goroutine count over time
   - `go_gc_duration_seconds` -- GC pause durations

6. Clean up:

   ```bash
   podman stop prom-memory && podman rm prom-memory
   ```

**Expected Result**:

- Memory metrics are exposed and scrapable by Prometheus
- Heap allocation stabilizes after initial load (no unbounded growth)
- Goroutine count remains stable (no goroutine leaks)
- GC pauses stay under 10ms for typical workloads

**Bonus Challenge**: Run `scripts/memory-leak-check.sh` and compare its heap snapshot analysis with the Prometheus metrics you collected.

---

### Exercise 7.5: Create a Custom Challenge

**Objective**: Write a custom challenge that validates a specific aspect of the Catalogizer system using the challenge framework.

**Prerequisites**: Module 6 completed, familiarity with Go

**Steps**:

1. Study an existing challenge for reference:

   ```bash
   cat catalog-api/challenges/register.go | head -40
   ```

2. Create a new challenge file `catalog-api/challenges/custom_health_challenge.go`:

   ```go
   package challenges

   import (
       "context"
       "fmt"
       "net/http"
       "time"

       "digital.vasic.challenges/pkg/challenge"
   )

   type HealthEndpointChallenge struct {
       challenge.BaseChallenge
       baseURL string
   }

   func NewHealthEndpointChallenge(baseURL string) *HealthEndpointChallenge {
       return &HealthEndpointChallenge{
           BaseChallenge: challenge.BaseChallenge{
               ChallengeID:   "CH-CUSTOM-001",
               ChallengeName: "Health Endpoint Validation",
               Description:   "Verifies the health endpoint returns 200 OK within acceptable latency",
           },
           baseURL: baseURL,
       }
   }

   func (c *HealthEndpointChallenge) Execute(ctx context.Context) (*challenge.Result, error) {
       result := challenge.NewResult(c.ChallengeID, c.ChallengeName)

       start := time.Now()
       resp, err := http.Get(c.baseURL + "/api/v1/health")
       elapsed := time.Since(start)

       result.Assert("health endpoint reachable", err == nil,
           fmt.Sprintf("GET /api/v1/health returned error: %v", err))

       if resp != nil {
           defer resp.Body.Close()
           result.Assert("returns 200 OK", resp.StatusCode == 200,
               fmt.Sprintf("expected 200, got %d", resp.StatusCode))
       }

       result.Assert("responds within 500ms", elapsed < 500*time.Millisecond,
           fmt.Sprintf("took %v", elapsed))

       return result, nil
   }
   ```

3. Register the challenge in `register.go` (study existing registrations for the pattern).

4. Build and run the backend, then execute your challenge via the API:

   ```bash
   curl -s -X POST http://localhost:8080/api/v1/challenges/CH-CUSTOM-001/run \
     -H "Authorization: Bearer $TOKEN" | python3 -m json.tool
   ```

5. Review the challenge result: check assertion pass/fail status and any messages.

**Expected Result**:

- The challenge compiles and registers without errors
- Executing via the API returns a structured result with 3 assertions
- All 3 assertions pass (endpoint reachable, returns 200, responds within 500ms)
- The challenge appears in the challenge list at `GET /api/v1/challenges`

**Bonus Challenge**: Extend the challenge to also check that the response body contains valid JSON with a `"status": "healthy"` field. Add the assertion and re-run.

---

## Capstone Project

### Project: Build a Custom Media Dashboard

**Objective**: Demonstrate mastery of Catalogizer by building a custom integration that combines backend API knowledge, frontend development, and system administration.

**Prerequisites**: All 6 core module exercises completed

**Requirements**:

1. **Backend**: Create at least one new API endpoint that provides aggregated data not available from existing endpoints (e.g., media statistics by decade, storage protocol usage breakdown, most-accessed media items).

2. **Frontend**: Build a custom dashboard page that consumes your new API endpoint and displays the data using charts or tables. Use React Query for data fetching and Tailwind CSS for styling.

3. **Testing**: Write unit tests for your new backend handler and service. Write a frontend test for your dashboard component. All tests must pass.

4. **Administration**: Document the deployment steps for your feature, including any new environment variables, database migrations, or configuration changes required.

**Deliverables**:

- New Go handler, service, and test files in `catalog-api/`
- New React page component and test in `catalog-web/`
- A brief writeup (1 page) describing your feature, its architecture, and how to deploy it

**Evaluation Criteria**:

- Code follows existing project conventions (constructor injection, table-driven tests, PascalCase components)
- All tests pass
- The feature works end-to-end: backend serves data, frontend displays it
- No console errors, no failed network requests (zero warning/zero error policy)
