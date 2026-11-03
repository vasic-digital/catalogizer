> **SUPERSEDED** — This document was a planning artifact from an earlier phase. The work described here has been completed or superseded by the [2026-03-26 Project Completion Plan](superpowers/plans/2026-03-26-project-completion-plan.md). Retained for historical reference.

---

# CATALOGIZER PROJECT - COMPREHENSIVE IMPLEMENTATION PLAN
## Complete Phase-by-Phase Execution Roadmap
## Version: 1.0 | Date: March 22, 2026
## Estimated Duration: 6-8 Months | Estimated Effort: 3,200+ Hours

---

## TABLE OF CONTENTS

1. [Executive Summary](#executive-summary)
2. [Phase Overview](#phase-overview)
3. [Phase 1: Foundation & Safety](#phase-1-foundation--safety)
4. [Phase 2: Test Infrastructure](#phase-2-test-infrastructure)
5. [Phase 3: Coverage Expansion](#phase-3-coverage-expansion)
6. [Phase 4: Integration & Dead Code Removal](#phase-4-integration--dead-code-removal)
7. [Phase 5: Security & Scanning](#phase-5-security--scanning)
8. [Phase 6: Performance & Optimization](#phase-6-performance--optimization)
9. [Phase 7: Monitoring & Observability](#phase-7-monitoring--observability)
10. [Phase 8: Documentation & Training](#phase-8-documentation--training)
11. [Phase 9: Website & Content](#phase-9-website--content)
12. [Phase 10: Final Validation & Deployment](#phase-10-final-validation--deployment)
13. [Appendices](#appendices)

---

## EXECUTIVE SUMMARY

### Project Goal
Transform Catalogizer from 65% complete to 100% production-ready with:
- **95%+ test coverage** across all components
- **Zero dead code** - all features functional
- **Complete security posture** - all tools integrated
- **Full observability** - monitoring, tracing, alerting
- **Comprehensive documentation** - user guides, API docs, video courses
- **Optimized performance** - lazy loading, semaphores, non-blocking I/O
- **Safety guaranteed** - no memory leaks, deadlocks, or race conditions

### Success Criteria
| Metric | Current | Target | Status |
|--------|---------|--------|--------|
| Test Coverage | 35% | 95% | ⏳ IN PROGRESS |
| Dead Code | 40% | 0% | ⏳ IN PROGRESS |
| Documentation | 85% | 100% | ⏳ IN PROGRESS |
| Security Score | 70% | 95% | ⏳ IN PROGRESS |
| Performance | 65% | 90% | ⏳ IN PROGRESS |
| Overall | 65% | 95% | ⏳ IN PROGRESS |

### Execution Strategy
**Waterfall-Agile Hybrid:**
- Phases execute sequentially (dependencies)
- Within phases, tasks parallelize
- Daily standups, weekly demos
- Continuous integration testing
- No broken code committed

---

## PHASE OVERVIEW

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                        IMPLEMENTATION TIMELINE                               │
├─────┬───────────────────────────────────────────────────────────────────────┤
│ W1-2│ Phase 1: Foundation & Safety - Fix race conditions, memory leaks     │
│ W3-4│ Phase 2: Test Infrastructure - Framework, tools, baseline            │
│ W5-8│ Phase 3: Coverage Expansion - Backend 35% → 95%                      │
│ W9-12│ Phase 4: Integration - Wire submodules, remove dead code            │
│ W13-14│ Phase 5: Security - Snyk, SonarQube, all tools                      │
│ W15-18│ Phase 6: Performance - Optimization, lazy loading                   │
│ W19-20│ Phase 7: Monitoring - Observability stack                          │
│ W21-22│ Phase 8: Documentation - Complete documentation suite               │
│ W23-24│ Phase 9: Website - Content, video courses                           │
│ W25-26│ Phase 10: Final Validation - Full system validation                 │
└─────┴───────────────────────────────────────────────────────────────────────┘
```

---

## PHASE 1: FOUNDATION & SAFETY
**Duration:** Weeks 1-2 (80 hours)  
**Goal:** Fix all safety issues - memory leaks, deadlocks, race conditions  
**Success Criteria:** Zero race conditions detected with `-race`, zero memory leaks

### 1.1 Memory Leak Fixes

#### Task 1.1.1: SMB Connection Pool Cleanup
**File:** `catalog-api/internal/smb/resilience.go`  
**Issue:** Connection pool not releasing idle connections  
**Risk:** HIGH

**Implementation Steps:**
1. Add connection timeout configuration (30s default)
2. Implement idle connection cleanup goroutine
3. Add connection lifecycle tracking
4. Write unit tests for connection cleanup

```go
// TODO: Add to internal/smb/resilience.go
const (
    connectionIdleTimeout = 30 * time.Second
    connectionMaxLifetime = 5 * time.Minute
    cleanupInterval = 10 * time.Second
)

type ConnectionPool struct {
    connections map[string]*PooledConnection
    mu sync.RWMutex
    cleanupTicker *time.Ticker
    done chan struct{}
}

type PooledConnection struct {
    conn *ResilientSMBClient
    lastUsed time.Time
    createdAt time.Time
}

func (p *ConnectionPool) StartCleanup() {
    p.cleanupTicker = time.NewTicker(cleanupInterval)
    go func() {
        for {
            select {
            case <-p.cleanupTicker.C:
                p.cleanupIdleConnections()
            case <-p.done:
                p.cleanupTicker.Stop()
                return
            }
        }
    }()
}

func (p *ConnectionPool) cleanupIdleConnections() {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    now := time.Now()
    for id, pc := range p.connections {
        if now.Sub(pc.lastUsed) > connectionIdleTimeout ||
           now.Sub(pc.createdAt) > connectionMaxLifetime {
            pc.conn.Close()
            delete(p.connections, id)
        }
    }
}
```

**Tests Required:**
- Test connection cleanup after timeout
- Test connection lifecycle tracking
- Test cleanup goroutine shutdown
- Test concurrent access safety

**Estimated Time:** 8 hours

---

#### Task 1.1.2: File Handle Cleanup in Scan Service
**File:** `catalog-api/services/scan_service.go`  
**Issue:** File handles not closed in error paths  
**Risk:** MEDIUM

**Implementation Steps:**
1. Audit all file operations in scan service
2. Add defer Close() for all file handles
3. Use io.Closer interface consistently
4. Add file handle tracking for debugging

```go
// Pattern to apply throughout scan_service.go
func (s *ScanService) processFile(path string) error {
    file, err := os.Open(path)
    if err != nil {
        return fmt.Errorf("failed to open file: %w", err)
    }
    defer func() {
        if err := file.Close(); err != nil {
            s.logger.Error("failed to close file", zap.Error(err), zap.String("path", path))
        }
    }()
    
    // Process file...
    return nil
}
```

**Tests Required:**
- Test file handle cleanup on success
- Test file handle cleanup on error
- Test file handle cleanup on panic
- Test resource exhaustion scenarios

**Estimated Time:** 6 hours

---

#### Task 1.1.3: WebSocket Connection Cleanup
**File:** `catalog-api/handlers/websocket_handler.go`  
**Issue:** Client connections not properly cleaned up  
**Risk:** HIGH

**Implementation Steps:**
1. Add connection registry with cleanup
2. Implement heartbeat/ping-pong with timeout
3. Add connection limit (1000 concurrent)
4. Implement graceful shutdown

```go
type WebSocketManager struct {
    clients map[string]*Client
    mu sync.RWMutex
    maxConnections int
    cleanupInterval time.Duration
}

type Client struct {
    conn *websocket.Conn
    send chan []byte
    lastPing time.Time
    done chan struct{}
}

func (m *WebSocketManager) cleanupStaleConnections() {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    for id, client := range m.clients {
        if time.Since(client.lastPing) > 2*time.Minute {
            close(client.done)
            client.conn.Close()
            delete(m.clients, id)
        }
    }
}
```

**Tests Required:**
- Test connection cleanup on client disconnect
- Test connection cleanup on timeout
- Test max connection limit enforcement
- Test graceful shutdown

**Estimated Time:** 8 hours

---

#### Task 1.1.4: Cache TTL Implementation
**File:** `catalog-api/internal/cache/redis.go`  
**Issue:** Cache entries without TTL  
**Risk:** MEDIUM

**Implementation Steps:**
1. Add default TTL for all cache entries (1 hour)
2. Implement cache size limits with LRU eviction
3. Add cache metrics (hits, misses, evictions)
4. Write cleanup routine for expired entries

```go
const (
    defaultTTL = 1 * time.Hour
    maxCacheSize = 10000 // entries
)

func (c *RedisCache) SetWithTTL(key string, value interface{}, ttl time.Duration) error {
    if ttl == 0 {
        ttl = defaultTTL
    }
    // Implementation...
}
```

**Tests Required:**
- Test TTL expiration
- Test LRU eviction
- Test cache metrics

**Estimated Time:** 4 hours

---

#### Task 1.1.5: Buffer Pool Implementation
**File:** `catalog-api/internal/media/analyzer.go`  
**Issue:** Large file buffers not pooled  
**Risk:** MEDIUM

**Implementation Steps:**
1. Implement sync.Pool for buffers
2. Use buffer pool for file reads
3. Add buffer size limits
4. Track buffer pool metrics

```go
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 32*1024) // 32KB buffers
    },
}

func analyzeFile(path string) error {
    buf := bufferPool.Get().([]byte)
    defer bufferPool.Put(buf)
    
    // Use buffer...
}
```

**Tests Required:**
- Test buffer pool reuse
- Test buffer sizing
- Test concurrent access

**Estimated Time:** 4 hours

**Phase 1.1 Total: 30 hours**

---

### 1.2 Race Condition Fixes

#### Task 1.2.1: LazyBooter Thread Safety
**File:** `catalog-api/internal/concurrency/lazy_booter.go`  
**Issue:** `Started()` method has side effects  
**Risk:** MEDIUM

**Implementation Steps:**
1. Separate state check from state modification
2. Use atomic operations for state
3. Add proper locking around state changes
4. Document thread-safety guarantees

```go
type LazyBooter struct {
    state atomic.Int32 // 0=stopped, 1=starting, 2=started, 3=stopping
    mu sync.Mutex
    startFn func() error
    stopFn func() error
}

const (
    stateStopped int32 = iota
    stateStarting
    stateStarted
    stateStopping
)

func (lb *LazyBooter) IsStarted() bool {
    return lb.state.Load() == stateStarted
}

func (lb *LazyBooter) Start() error {
    if !lb.state.CompareAndSwap(stateStopped, stateStarting) {
        // Already started or starting
        return nil
    }
    
    lb.mu.Lock()
    defer lb.mu.Unlock()
    
    if err := lb.startFn(); err != nil {
        lb.state.Store(stateStopped)
        return err
    }
    
    lb.state.Store(stateStarted)
    return nil
}
```

**Tests Required:**
- Test concurrent Start() calls
- Test concurrent Stop() calls
- Test Start() during Stop()
- Race detector validation

**Estimated Time:** 6 hours

---

#### Task 1.2.2: Challenge Service Result Channel Safety
**File:** `catalog-api/services/challenge_service.go`  
**Issue:** Result channel draining race  
**Risk:** MEDIUM

**Implementation Steps:**
1. Use context cancellation for coordination
2. Implement proper channel closing
3. Add timeout for result collection
4. Use select with done channel

```go
func (s *ChallengeService) RunChallenge(ctx context.Context, challengeID string) (*ChallengeResult, error) {
    resultChan := make(chan ChallengeResult, 1)
    errChan := make(chan error, 1)
    
    go func() {
        result, err := s.executeChallenge(challengeID)
        if err != nil {
            errChan <- err
            return
        }
        resultChan <- result
    }()
    
    select {
    case result := <-resultChan:
        return &result, nil
    case err := <-errChan:
        return nil, err
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}
```

**Tests Required:**
- Test result channel race
- Test context cancellation
- Test timeout handling

**Estimated Time:** 4 hours

---

#### Task 1.2.3: SMB Resilience Mutex Fix
**File:** `catalog-api/internal/smb/resilience.go`  
**Issue:** Nested mutex locking  
**Risk:** MEDIUM

**Implementation Steps:**
1. Audit all mutex usage
2. Eliminate nested locking
3. Use RWMutex where appropriate
4. Document lock ordering

```go
// Lock ordering (must be consistent):
// 1. ConnectionPool.mu
// 2. ResilientSMBClient.mu

func (c *ResilientSMBClient) WithLock(fn func() error) error {
    c.mu.Lock()
    defer c.mu.Unlock()
    return fn()
}
```

**Tests Required:**
- Test deadlock scenarios
- Test concurrent operations
- Test lock ordering

**Estimated Time:** 6 hours

---

#### Task 1.2.4: WebSocket Concurrent Map Access
**File:** `catalog-api/handlers/websocket_handler.go`  
**Issue:** Concurrent map access  
**Risk:** HIGH

**Implementation Steps:**
1. Replace map with sync.Map or add mutex
2. Implement client registry with proper locking
3. Add broadcast with fan-out
4. Test concurrent access patterns

```go
type ClientRegistry struct {
    clients sync.Map // map[string]*Client
    mu sync.RWMutex
}

func (r *ClientRegistry) Register(id string, client *Client) {
    r.clients.Store(id, client)
}

func (r *ClientRegistry) Unregister(id string) {
    r.clients.Delete(id)
}

func (r *ClientRegistry) Broadcast(msg []byte) {
    r.clients.Range(func(key, value interface{}) bool {
        client := value.(*Client)
        select {
        case client.send <- msg:
        default:
            // Client send buffer full, skip
        }
        return true
    })
}
```

**Tests Required:**
- Test concurrent register/unregister
- Test concurrent broadcast
- Test race conditions

**Estimated Time:** 6 hours

**Phase 1.2 Total: 22 hours**

---

### 1.3 Deadlock Fixes

#### Task 1.3.1: Database Transaction Lock Ordering
**File:** `catalog-api/internal/database/transaction.go`  
**Issue:** Long-running transactions holding locks  
**Risk:** MEDIUM

**Implementation Steps:**
1. Add transaction timeout (30s default)
2. Implement query timeout
3. Add deadlock detection
4. Implement retry with exponential backoff

```go
const (
    transactionTimeout = 30 * time.Second
    queryTimeout = 5 * time.Second
    maxRetries = 3
)

func (db *DB) WithTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
    ctx, cancel := context.WithTimeout(ctx, transactionTimeout)
    defer cancel()
    
    tx, err := db.BeginTx(ctx, nil)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    if err := fn(tx); err != nil {
        return err
    }
    
    return tx.Commit()
}
```

**Tests Required:**
- Test transaction timeout
- Test deadlock detection
- Test retry logic

**Estimated Time:** 8 hours

---

#### Task 1.3.2: Sync Service Circular Dependency
**File:** `catalog-api/services/sync_service.go`  
**Issue:** Circular dependency in sync operations  
**Risk:** HIGH

**Implementation Steps:**
1. Break circular dependency with interface
2. Use dependency injection
3. Implement sync state machine
4. Add timeout and cancellation

```go
// Break circular dependency
type SyncOperation interface {
    Execute(ctx context.Context) error
}

type SyncService struct {
    operations map[string]SyncOperation
    mu sync.RWMutex
}

func (s *SyncService) RegisterOperation(name string, op SyncOperation) {
    s.mu.Lock()
    defer s.mu.Unlock()
    s.operations[name] = op
}
```

**Tests Required:**
- Test circular dependency resolution
- Test sync state machine
- Test timeout handling

**Estimated Time:** 10 hours

---

#### Task 1.3.3: Cache LRU Lock Ordering
**File:** `catalog-api/internal/cache/lru.go`  
**Issue:** Lock ordering inconsistencies  
**Risk:** LOW

**Implementation Steps:**
1. Document lock ordering
2. Ensure consistent lock acquisition order
3. Add lock hierarchy enforcement
4. Test edge cases

**Estimated Time:** 4 hours

**Phase 1.3 Total: 22 hours**

---

### 1.4 Goroutine Leak Fixes

#### Task 1.4.1: Scan Service Goroutine Cleanup
**File:** `catalog-api/services/scan_service.go`  
**Issue:** Fire-and-forget goroutines without cleanup  
**Risk:** MEDIUM

**Implementation Steps:**
1. Use errgroup for goroutine management
2. Add context cancellation
3. Implement worker pool with lifecycle
4. Track active goroutines

```go
import "golang.org/x/sync/errgroup"

func (s *ScanService) ScanDirectory(ctx context.Context, path string) error {
    g, ctx := errgroup.WithContext(ctx)
    g.SetLimit(10) // Max 10 concurrent scans
    
    files, err := s.listFiles(path)
    if err != nil {
        return err
    }
    
    for _, file := range files {
        file := file // capture range variable
        g.Go(func() error {
            return s.processFile(ctx, file)
        })
    }
    
    return g.Wait()
}
```

**Tests Required:**
- Test goroutine cleanup on success
- Test goroutine cleanup on error
- Test goroutine cleanup on cancellation
- Test max concurrency limit

**Estimated Time:** 6 hours

---

### 1.5 Race Detection Validation

#### Task 1.5.1: Comprehensive Race Testing
**Scope:** All Go modules  
**Goal:** Zero race conditions

**Implementation Steps:**
1. Run all tests with `-race` flag
2. Fix any detected races
3. Add race detection to CI
4. Document race-free guarantees

```bash
# Commands to run
GOMAXPROCS=3 go test ./... -race -count=1 -p 2 -parallel 2
```

**Estimated Time:** 8 hours

---

### Phase 1 Deliverables

1. ✅ All memory leaks fixed
2. ✅ All race conditions resolved
3. ✅ All deadlocks eliminated
4. ✅ All goroutine leaks fixed
5. ✅ Race detector passes on all tests
6. ✅ Performance benchmarks baseline
7. ✅ Safety documentation

**Phase 1 Total: 88 hours**

---

## PHASE 2: TEST INFRASTRUCTURE
**Duration:** Weeks 3-4 (80 hours)  
**Goal:** Establish comprehensive testing framework  
**Success Criteria:** All test types configured, baseline coverage established

### 2.1 Test Framework Enhancement

#### Task 2.1.1: HelixQA Test Bank Expansion
**Scope:** All supported test types  
**Files:** `challenges/helixqa-banks/`

**Implementation Steps:**

1. **Create test bank structure:**
```yaml
# challenges/helixqa-banks/catalogizer-complete.yaml
test_suites:
  - name: "Unit Tests"
    type: unit
    coverage_target: 95%
    
  - name: "Integration Tests"
    type: integration
    coverage_target: 90%
    
  - name: "E2E Tests"
    type: e2e
    coverage_target: 85%
    
  - name: "Security Tests"
    type: security
    coverage_target: 100%
    
  - name: "Stress Tests"
    type: stress
    coverage_target: 100%
    
  - name: "Load Tests"
    type: load
    coverage_target: 100%
    
  - name: "Contract Tests"
    type: contract
    coverage_target: 100%
    
  - name: "Mutation Tests"
    type: mutation
    coverage_target: 80%
```

2. **Create test bank for each component:**
   - `catalogizer-api-complete.yaml` - Backend tests
   - `catalogizer-web-complete.yaml` - Frontend tests
   - `catalogizer-desktop-complete.yaml` - Desktop tests
   - `catalogizer-android-complete.yaml` - Mobile tests
   - `catalogizer-integration-complete.yaml` - Integration tests
   - `catalogizer-security-complete.yaml` - Security tests
   - `catalogizer-performance-complete.yaml` - Performance tests

**Estimated Time:** 16 hours

---

#### Task 2.1.2: Test Utilities Library
**File:** `catalog-api/internal/tests/test_helper.go`  
**Goal:** Comprehensive test utilities

**Implementation Steps:**

1. **Database test utilities:**
```go
package tests

import (
    "testing"
    "catalog-api/database"
)

type TestDB struct {
    *database.DB
    t *testing.T
}

func NewTestDB(t *testing.T) *TestDB {
    db := database.WrapDB(sqlite.Open(":memory:"), database.DialectSQLite)
    if err := db.Migrate(); err != nil {
        t.Fatalf("failed to migrate: %v", err)
    }
    return &TestDB{DB: db, t: t}
}

func (tdb *TestDB) MustExec(query string, args ...interface{}) {
    tdb.t.Helper()
    if _, err := tdb.DB.Exec(query, args...); err != nil {
        tdb.t.Fatalf("failed to exec: %v", err)
    }
}

func (tdb *TestDB) MustQueryRow(query string, args ...interface{}) *sql.Row {
    tdb.t.Helper()
    return tdb.DB.QueryRow(query, args...)
}
```

2. **HTTP test utilities:**
```go
func NewTestServer(t *testing.T, handlers http.Handler) *httptest.Server {
    server := httptest.NewServer(handlers)
    t.Cleanup(func() { server.Close() })
    return server
}

func AssertStatusCode(t *testing.T, resp *http.Response, expected int) {
    t.Helper()
    if resp.StatusCode != expected {
        t.Errorf("expected status %d, got %d", expected, resp.StatusCode)
    }
}

func AssertJSONResponse(t *testing.T, resp *http.Response, v interface{}) {
    t.Helper()
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        t.Fatalf("failed to read body: %v", err)
    }
    if err := json.Unmarshal(body, v); err != nil {
        t.Fatalf("failed to unmarshal: %v", err)
    }
}
```

3. **Mock utilities:**
```go
type MockCache struct {
    mu sync.RWMutex
    data map[string]interface{}
}

func (m *MockCache) Get(key string) (interface{}, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    val, ok := m.data[key]
    return val, ok
}

func (m *MockCache) Set(key string, value interface{}, ttl time.Duration) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.data[key] = value
    return nil
}
```

**Estimated Time:** 12 hours

---

#### Task 2.1.3: Contract Testing Setup
**Scope:** API contracts  
**Tool:** Pact (consumer-driven contract testing)

**Implementation Steps:**

1. **Install Pact:**
```bash
# Add to docker-compose.test-infra.yml
services:
  pact-broker:
    image: pactfoundation/pact-broker
    ports:
      - "9292:9292"
    environment:
      - PACT_BROKER_DATABASE_URL=sqlite:////tmp/pact.db
```

2. **Create consumer contract tests:**
```go
// catalog-web/src/services/__tests__/api.contract.test.ts
import { Pact } from '@pact-foundation/pact';

describe('API Consumer Contract', () => {
  const provider = new Pact({
    consumer: 'catalog-web',
    provider: 'catalog-api',
    port: 1234,
  });

  beforeAll(() => provider.setup());
  afterEach(() => provider.verify());
  afterAll(() => provider.finalize());

  it('should return media list', async () => {
    await provider.addInteraction({
      state: 'media exists',
      uponReceiving: 'a request for media list',
      withRequest: {
        method: 'GET',
        path: '/api/v1/media',
        headers: { Authorization: 'Bearer token' },
      },
      willRespondWith: {
        status: 200,
        body: {
          items: Matchers.eachLike({
            id: Matchers.uuid(),
            title: Matchers.string(),
            type: Matchers.string(),
          }),
        },
      },
    });

    const result = await api.getMedia();
    expect(result.items).toHaveLength(1);
  });
});
```

3. **Create provider contract tests:**
```go
// catalog-api/contracts/media_contract_test.go
package contracts

import (
    "testing"
    "github.com/pact-foundation/pact-go/dsl"
)

func TestProvider(t *testing.T) {
    pact := &dsl.Pact{
        Provider: "catalog-api",
    }
    
    pact.VerifyProvider(t, dsl.VerifyRequest{
        ProviderBaseURL: "http://localhost:8080",
        PactURLs: []string{"../catalog-web/pacts/catalog-web-catalog-api.json"},
        StateHandlers: dsl.StateHandlers{
            "media exists": func() error {
                // Setup test data
                return nil
            },
        },
    })
}
```

**Estimated Time:** 16 hours

---

#### Task 2.1.4: Mutation Testing Setup
**Tool:** go-mutesting (Go), Stryker (TypeScript)

**Implementation Steps:**

1. **Go mutation testing:**
```bash
# Install
go install github.com/zimmski/go-mutesting/...@latest

# Run
go-mutesting ./...
```

2. **TypeScript mutation testing:**
```bash
# Install
npm install --save-dev @stryker-mutator/core @stryker-mutator/typescript

# Configure (stryker.config.js)
module.exports = {
  testRunner: 'jest',
  mutator: 'typescript',
  transpilers: ['typescript'],
  reporters: ['progress', 'clear-text', 'html'],
  testFramework: 'jest',
  coverageAnalysis: 'off',
  thresholds: {
    high: 80,
    low: 60,
    break: 50,
  },
};
```

**Estimated Time:** 8 hours

---

### 2.2 Test Coverage Tracking

#### Task 2.2.1: Coverage Reporting Infrastructure
**Goal:** Automated coverage tracking

**Implementation Steps:**

1. **Coverage configuration:**
```bash
# Add to Makefile
coverage:
	go test ./... -coverprofile=coverage.out -covermode=atomic
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out | tail -1

coverage-by-package:
	go test ./... -coverprofile=coverage.out
	@echo "Coverage by package:"
	@go tool cover -func=coverage.out | grep -E "^(catalog-api|digital\.vasic)" | grep -v "total:"
```

2. **Coverage badge generation:**
```bash
# scripts/generate-coverage-badge.sh
#!/bin/bash
coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')
color="red"
if (( $(echo "$coverage > 80" | bc -l) )); then color="green"; fi
echo "Coverage: ${coverage}%" > coverage-badge.txt
```

3. **Coverage threshold enforcement:**
```go
// Add to CI script
func main() {
    threshold := 95.0
    coverage := getCoverage()
    if coverage < threshold {
        log.Fatalf("Coverage %.2f%% below threshold %.2f%%", coverage, threshold)
    }
}
```

**Estimated Time:** 8 hours

---

### Phase 2 Deliverables

1. ✅ HelixQA test banks for all test types
2. ✅ Comprehensive test utilities library
3. ✅ Contract testing configured (Pact)
4. ✅ Mutation testing configured
5. ✅ Coverage tracking infrastructure
6. ✅ Coverage badges and reporting
7. ✅ Test documentation

**Phase 2 Total: 80 hours**

---

## PHASE 3: COVERAGE EXPANSION
**Duration:** Weeks 5-8 (160 hours)  
**Goal:** Backend test coverage 35% → 95%  
**Success Criteria:** All services >95% coverage

### 3.1 Critical Services (Coverage <30%)

#### Task 3.1.1: Auth Service Testing (26.7% → 95%)
**File:** `catalog-api/services/auth_service.go`  
**Current:** 26.7% | **Target:** 95%  
**Gap:** -68.3%

**Test Cases Required:**

1. **JWT Token Generation**
   - Valid credentials
   - Invalid credentials
   - Expired credentials
   - Empty credentials
   - Special characters in credentials
   - Unicode credentials

2. **Token Validation**
   - Valid token
   - Expired token
   - Invalid signature
   - Malformed token
   - Missing claims
   - Tampered token

3. **Password Operations**
   - Hash password
   - Verify correct password
   - Verify wrong password
   - Password strength validation
   - Password history check
   - Password reset flow

4. **Role-Based Access**
   - Admin access
   - User access
   - Guest access
   - Missing roles
   - Invalid roles
   - Role hierarchy

5. **Session Management**
   - Create session
   - Extend session
   - Invalidate session
   - Concurrent sessions
   - Session timeout
   - Session hijacking protection

**Implementation:**
```go
// services/auth_service_test.go
package services

import (
    "testing"
    "time"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "catalog-api/internal/tests"
)

func TestAuthService_GenerateToken(t *testing.T) {
    tdb := tests.NewTestDB(t)
    cache := tests.NewMockCache()
    service := NewAuthService(tdb.DB, cache, "test-secret")
    
    tests := []struct {
        name      string
        username  string
        password  string
        wantError bool
        errorMsg  string
    }{
        {
            name:      "valid credentials",
            username:  "admin",
            password:  "correct-password",
            wantError: false,
        },
        {
            name:      "invalid credentials",
            username:  "admin",
            password:  "wrong-password",
            wantError: true,
            errorMsg:  "invalid credentials",
        },
        {
            name:      "empty credentials",
            username:  "",
            password:  "",
            wantError: true,
            errorMsg:  "credentials required",
        },
        {
            name:      "special characters",
            username:  "user@example.com",
            password:  "p@ssw0rd!#$%",
            wantError: false,
        },
        {
            name:      "unicode credentials",
            username:  "用户",
            password:  "пароль",
            wantError: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            token, err := service.GenerateToken(tt.username, tt.password)
            if tt.wantError {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errorMsg)
                assert.Empty(t, token)
            } else {
                require.NoError(t, err)
                assert.NotEmpty(t, token)
            }
        })
    }
}

func TestAuthService_ValidateToken(t *testing.T) {
    tdb := tests.NewTestDB(t)
    cache := tests.NewMockCache()
    service := NewAuthService(tdb.DB, cache, "test-secret")
    
    // Generate valid token
    validToken, _ := service.GenerateToken("admin", "password")
    
    tests := []struct {
        name      string
        token     string
        wantValid bool
        wantError bool
    }{
        {
            name:      "valid token",
            token:     validToken,
            wantValid: true,
            wantError: false,
        },
        {
            name:      "invalid token",
            token:     "invalid-token",
            wantValid: false,
            wantError: true,
        },
        {
            name:      "empty token",
            token:     "",
            wantValid: false,
            wantError: true,
        },
        {
            name:      "malformed token",
            token:     "not.a.token",
            wantValid: false,
            wantError: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            claims, err := service.ValidateToken(tt.token)
            if tt.wantError {
                assert.Error(t, err)
            } else {
                require.NoError(t, err)
                assert.NotNil(t, claims)
                assert.True(t, tt.wantValid)
            }
        })
    }
}

func TestAuthService_PasswordHashing(t *testing.T) {
    service := &AuthService{}
    
    password := "my-secret-password"
    
    t.Run("hash password", func(t *testing.T) {
        hash, err := service.HashPassword(password)
        require.NoError(t, err)
        assert.NotEmpty(t, hash)
        assert.NotEqual(t, password, hash)
    })
    
    t.Run("verify correct password", func(t *testing.T) {
        hash, _ := service.HashPassword(password)
        valid := service.VerifyPassword(password, hash)
        assert.True(t, valid)
    })
    
    t.Run("verify wrong password", func(t *testing.T) {
        hash, _ := service.HashPassword(password)
        valid := service.VerifyPassword("wrong-password", hash)
        assert.False(t, valid)
    })
}

func TestAuthService_RoleBasedAccess(t *testing.T) {
    tdb := tests.NewTestDB(t)
    cache := tests.NewMockCache()
    service := NewAuthService(tdb.DB, cache, "test-secret")
    
    tests := []struct {
        name       string
        userRoles  []string
        required   []string
        wantAccess bool
    }{
        {
            name:       "admin accessing admin resource",
            userRoles:  []string{"admin"},
            required:   []string{"admin"},
            wantAccess: true,
        },
        {
            name:       "user accessing user resource",
            userRoles:  []string{"user"},
            required:   []string{"user"},
            wantAccess: true,
        },
        {
            name:       "user accessing admin resource",
            userRoles:  []string{"user"},
            required:   []string{"admin"},
            wantAccess: false,
        },
        {
            name:       "multiple roles",
            userRoles:  []string{"user", "moderator"},
            required:   []string{"moderator"},
            wantAccess: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            hasAccess := service.HasRole(tt.userRoles, tt.required)
            assert.Equal(t, tt.wantAccess, hasAccess)
        })
    }
}

func TestAuthService_SessionManagement(t *testing.T) {
    tdb := tests.NewTestDB(t)
    cache := tests.NewMockCache()
    service := NewAuthService(tdb.DB, cache, "test-secret")
    
    t.Run("create session", func(t *testing.T) {
        session, err := service.CreateSession("user-123", []string{"user"})
        require.NoError(t, err)
        assert.NotEmpty(t, session.ID)
        assert.Equal(t, "user-123", session.UserID)
        assert.WithinDuration(t, time.Now().Add(24*time.Hour), session.ExpiresAt, time.Minute)
    })
    
    t.Run("extend session", func(t *testing.T) {
        session, _ := service.CreateSession("user-123", []string{"user"})
        originalExpiry := session.ExpiresAt
        
        err := service.ExtendSession(session.ID)
        require.NoError(t, err)
        
        extended, _ := service.GetSession(session.ID)
        assert.True(t, extended.ExpiresAt.After(originalExpiry))
    })
    
    t.Run("invalidate session", func(t *testing.T) {
        session, _ := service.CreateSession("user-123", []string{"user"})
        
        err := service.InvalidateSession(session.ID)
        require.NoError(t, err)
        
        _, err = service.GetSession(session.ID)
        assert.Error(t, err)
    })
    
    t.Run("concurrent sessions", func(t *testing.T) {
        var sessions []string
        for i := 0; i < 5; i++ {
            session, _ := service.CreateSession("user-123", []string{"user"})
            sessions = append(sessions, session.ID)
        }
        
        // Verify all sessions exist
        for _, id := range sessions {
            _, err := service.GetSession(id)
            assert.NoError(t, err)
        }
    })
}
```

**Estimated Time:** 24 hours

---

#### Task 3.1.2: Conversion Service Testing (21.3% → 95%)
**File:** `catalog-api/services/conversion_service.go`  
**Current:** 21.3% | **Target:** 95%  
**Gap:** -73.7%

**Test Cases Required:**

1. **Format Detection**
   - Video formats (MP4, AVI, MKV, MOV)
   - Audio formats (MP3, AAC, FLAC, WAV)
   - Image formats (JPG, PNG, GIF, WebP)
   - Document formats (PDF, DOC, TXT)
   - Unknown formats
   - Corrupted files

2. **Video Conversion**
   - MP4 to AVI
   - Resolution changes
   - Bitrate adjustments
   - Codec selection
   - Failed conversions
   - Progress tracking

3. **Audio Conversion**
   - MP3 to FLAC
   - Sample rate changes
   - Channel configuration
   - Metadata preservation
   - Failed conversions

4. **Image Conversion**
   - Format conversion
   - Resize operations
   - Quality adjustments
   - Batch processing
   - Failed conversions

5. **Error Handling**
   - Invalid input
   - Disk space issues
   - Permission errors
   - Network timeouts
   - Process crashes

**Estimated Time:** 20 hours

---

#### Task 3.1.3: Favorites Service Testing (14.1% → 95%)
**File:** `catalog-api/services/favorites_service.go`  
**Current:** 14.1% | **Target:** 95%  
**Gap:** -80.9%

**Test Cases Required:**

1. **Add to Favorites**
   - Valid media
   - Already favorited
   - Invalid media ID
   - Duplicate prevention
   - Maximum limit (1000)

2. **Remove from Favorites**
   - Existing favorite
   - Non-existent favorite
   - Bulk removal
   - Clear all

3. **List Favorites**
   - Empty list
   - Paginated list
   - Sorted by date
   - Sorted by name
   - Filter by type

4. **Check Favorite Status**
   - Is favorited
   - Is not favorited
   - Batch check

5. **Watchlist Operations**
   - Add to watchlist
   - Mark as watched
   - Mark as unwatched
   - Progress tracking

**Estimated Time:** 16 hours

---

#### Task 3.1.4: Sync Service Testing (12.6% → 95%)
**File:** `catalog-api/services/sync_service.go`  
**Current:** 12.6% | **Target:** 95%  
**Gap:** -82.4%

**Test Cases Required:**

1. **Device Registration**
   - Register new device
   - Duplicate registration
   - Invalid device ID
   - Device metadata

2. **Sync Operations**
   - Full sync
   - Incremental sync
   - Bidirectional sync
   - Conflict resolution
   - Sync filtering

3. **Conflict Resolution**
   - Local wins
   - Remote wins
   - Merge strategy
   - Manual resolution
   - Timestamp-based

4. **Offline Support**
   - Queue operations
   - Replay on reconnect
   - Deduplication
   - Conflict detection

5. **Performance**
   - Large dataset sync
   - Concurrent syncs
   - Bandwidth limiting
   - Resume interrupted sync

**Estimated Time:** 20 hours

---

#### Task 3.1.5: WebDAV Client Testing (2.0% → 95%)
**File:** `catalog-api/internal/webdav/client.go`  
**Current:** 2.0% | **Target:** 95%  
**Gap:** -93.0%

**Test Cases Required:**

1. **Connection**
   - Valid connection
   - Invalid credentials
   - Connection timeout
   - SSL/TLS issues
   - Redirect handling

2. **Operations**
   - List directory
   - Download file
   - Upload file
   - Delete file
   - Create directory
   - Move/rename

3. **Error Handling**
   - 404 Not Found
   - 403 Forbidden
   - 500 Server Error
   - Network errors
   - Timeout errors

4. **Caching**
   - Cache hits
   - Cache misses
   - Cache invalidation
   - TTL expiration

5. **Resilience**
   - Retry on failure
   - Circuit breaker
   - Exponential backoff
   - Health checks

**Estimated Time:** 16 hours

**Phase 3.1 Total: 96 hours**

---

### 3.2 High-Priority Services (Coverage 30-60%)

#### Task 3.2.1: Analytics Service Testing (54.5% → 95%)
**File:** `catalog-api/services/analytics_service.go`  
**Current:** 54.5% | **Target:** 95%  
**Gap:** -40.5%

**Note:** This service is currently unconnected (dead code). Fix integration first.

**Estimated Time:** 12 hours

---

#### Task 3.2.2: Reporting Service Testing (30.5% → 95%)
**File:** `catalog-api/services/reporting_service.go`  
**Current:** 30.5% | **Target:** 95%  
**Gap:** -64.5%

**Note:** This service is currently unconnected (dead code). Fix integration first.

**Estimated Time:** 16 hours

---

#### Task 3.2.3: Configuration Service Testing (58.8% → 95%)
**File:** `catalog-api/services/configuration_service.go`  
**Current:** 58.8% | **Target:** 95%  
**Gap:** -36.2%

**Estimated Time:** 12 hours

---

#### Task 3.2.4: Challenge Service Testing (67.3% → 95%)
**File:** `catalog-api/services/challenge_service.go`  
**Current:** 67.3% | **Target:** 95%  
**Gap:** -27.7%

**Estimated Time:** 8 hours

**Phase 3.2 Total: 48 hours**

---

### 3.3 Repository Layer Testing

#### Task 3.3.1: Media Collection Repository Testing (30% → 95%)
**File:** `catalog-api/repository/media_collection_repository.go`  
**Current:** 30% | **Target:** 95%  
**Gap:** -65%

**Test Cases Required:**

1. **Create Collection**
2. **Update Collection**
3. **Delete Collection**
4. **Add Media to Collection**
5. **Remove Media from Collection**
6. **List Collections**
7. **Get Collection Details**
8. **Reorder Items**
9. **Bulk Operations**
10. **Permission Checks**

**Estimated Time:** 16 hours

---

### Phase 3 Deliverables

1. ✅ All critical services >95% coverage
2. ✅ All high-priority services >95% coverage
3. ✅ Repository layer >95% coverage
4. ✅ Handler layer >95% coverage
5. ✅ Integration tests for all services
6. ✅ Contract tests passing
7. ✅ Mutation testing passing

**Phase 3 Total: 160 hours**

---

## PHASE 4: INTEGRATION & DEAD CODE REMOVAL
**Duration:** Weeks 9-12 (160 hours)  
**Goal:** Wire submodules, remove dead code, integrate unconnected services  
**Success Criteria:** Zero dead code, all submodules integrated or removed

### 4.1 Dead Code Removal

#### Task 4.1.1: Remove Unused Recommendation Handler
**File:** `catalog-api/handlers/simple_recommendation_handler.go`  
**Size:** 156 lines  
**Action:** DELETE

**Steps:**
1. Remove file
2. Update any imports
3. Verify no references remain

**Estimated Time:** 1 hour

---

#### Task 4.1.2: Remove LLM Provider Stubs
**Files:**
- `junie_cli_stub.go` (89 lines)
- `gemini_cli_stub.go` (94 lines)

**Action:** DELETE

**Steps:**
1. Remove files
2. Update imports
3. Remove references from provider registry

**Estimated Time:** 1 hour

---

#### Task 4.1.3: Remove Vision Engine Stubs
**File:** `vision/stub.go` (234 lines)

**Action:** DELETE

**Steps:**
1. Remove file
2. Remove from build tags
3. Update vision engine factory

**Estimated Time:** 2 hours

---

#### Task 4.1.4: Remove Commented Code Blocks
**Scope:** All source files

**Action:** Clean up

**Files to clean:**
- `handlers/media_handler.go` (45-78)
- `services/scan_service.go` (234-289)
- `internal/media/detector/detector.go` (567-623)

**Estimated Time:** 4 hours

---

#### Task 4.1.5: Remove Unused Imports (Frontend)
**Scope:** catalog-web TypeScript files

**Action:** Automated cleanup

```bash
# Run ESLint with --fix
cd catalog-web
npm run lint:fix

# Remove unused imports
npx ts-prune
```

**Estimated Time:** 4 hours

---

#### Task 4.1.6: Remove Unused Functions
**Scope:** All source files

**Functions to remove:**
- `calculateAdvancedStats()` - analytics_service.go
- `generateCustomReport()` - reporting_service.go
- `bulkUpdateFavorites()` - favorites_service.go
- `validateFileChecksum()` - internal/utils/file_utils.go

**Estimated Time:** 4 hours

**Phase 4.1 Total: 16 hours**

---

### 4.2 Unconnected Services Integration

#### Task 4.2.1: Integrate Analytics Service
**Files:**
- `catalog-api/services/analytics_service.go`
- `catalog-api/handlers/analytics_handler.go`
- `catalog-api/main.go`

**Current State:** Service instantiated but never called  
**Action:** FULL INTEGRATION

**Implementation Steps:**

1. **Add API routes:**
```go
// main.go
analyticsHandler := handlers.NewAnalyticsHandler(analyticsService)
{
    analytics := api.Group("/analytics")
    analytics.Use(middleware.AuthRequired())
    {
        analytics.GET("/dashboard", analyticsHandler.GetDashboard)
        analytics.GET("/trends", analyticsHandler.GetTrends)
        analytics.GET("/media/:id/stats", analyticsHandler.GetMediaStats)
        analytics.POST("/track", analyticsHandler.TrackEvent)
    }
}
```

2. **Implement event tracking:**
```go
// handlers/analytics_handler.go
func (h *AnalyticsHandler) TrackEvent(c *gin.Context) {
    var event TrackEventRequest
    if err := c.ShouldBindJSON(&event); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    userID, _ := c.Get("userID")
    if err := h.service.TrackEvent(userID.(string), event); err != nil {
        c.JSON(500, gin.H{"error": err.Error()})
        return
    }
    
    c.JSON(200, gin.H{"status": "tracked"})
}
```

3. **Frontend integration:**
```typescript
// catalog-web/src/services/analytics.ts
export const analyticsService = {
  async trackEvent(event: AnalyticsEvent): Promise<void> {
    await api.post('/analytics/track', event);
  },
  
  async getDashboard(): Promise<DashboardData> {
    const response = await api.get('/analytics/dashboard');
    return response.data;
  },
  
  async getTrends(period: string): Promise<TrendData[]> {
    const response = await api.get('/analytics/trends', {
      params: { period }
    });
    return response.data;
  }
};
```

4. **Add analytics tracking to components:**
```typescript
// catalog-web/src/hooks/useAnalytics.ts
import { useEffect } from 'react';
import { analyticsService } from '@/services/analytics';

export const usePageView = (pageName: string) => {
  useEffect(() => {
    analyticsService.trackEvent({
      type: 'page_view',
      page: pageName,
      timestamp: new Date().toISOString()
    });
  }, [pageName]);
};
```

**Estimated Time:** 24 hours

---

#### Task 4.2.2: Integrate Reporting Service
**Files:**
- `catalog-api/services/reporting_service.go`
- `catalog-api/handlers/reporting_handler.go`
- `catalog-api/main.go`

**Current State:** Service instantiated but never called  
**Action:** FULL INTEGRATION

**Implementation Steps:**

1. **Add API routes:**
```go
// main.go
reportingHandler := handlers.NewReportingHandler(reportingService)
{
    reports := api.Group("/reports")
    reports.Use(middleware.AuthRequired())
    {
        reports.GET("/types", reportingHandler.GetReportTypes)
        reports.POST("/generate", reportingHandler.GenerateReport)
        reports.GET("/download/:id", reportingHandler.DownloadReport)
        reports.GET("/schedule", reportingHandler.GetScheduledReports)
        reports.POST("/schedule", reportingHandler.ScheduleReport)
    }
}
```

2. **Implement PDF generation:**
```go
// services/reporting_service.go
func (s *ReportingService) GeneratePDFReport(req ReportRequest) (*Report, error) {
    // Implementation...
}

func (s *ReportingService) ScheduleReport(req ScheduleRequest) error {
    // Implementation...
}
```

3. **Frontend integration:**
```typescript
// catalog-web/src/components/ReportingWizard.tsx
export const ReportingWizard: React.FC = () => {
  const [step, setStep] = useState(1);
  const [reportConfig, setReportConfig] = useState<ReportConfig>({});
  
  const handleGenerate = async () => {
    const report = await reportingService.generateReport(reportConfig);
    // Handle report generation
  };
  
  return (
    <Wizard steps={steps} currentStep={step}>
      {/* Wizard content */}
    </Wizard>
  );
};
```

**Estimated Time:** 28 hours

---

#### Task 4.2.3: Integrate Favorites Service
**Files:**
- `catalog-api/services/favorites_service.go`
- `catalog-api/handlers/favorites_handler.go`
- `catalog-api/main.go`

**Current State:** Service instantiated but never called  
**Action:** FULL INTEGRATION

**Implementation Steps:**

1. **Add API routes:**
```go
// main.go
favoritesHandler := handlers.NewFavoritesHandler(favoritesService)
{
    favorites := api.Group("/favorites")
    favorites.Use(middleware.AuthRequired())
    {
        favorites.GET("", favoritesHandler.GetFavorites)
        favorites.POST("/:mediaId", favoritesHandler.AddFavorite)
        favorites.DELETE("/:mediaId", favoritesHandler.RemoveFavorite)
        favorites.GET("/check/:mediaId", favoritesHandler.CheckFavorite)
        favorites.GET("/watchlist", favoritesHandler.GetWatchlist)
    }
}
```

2. **Frontend integration:**
```typescript
// catalog-web/src/components/FavoriteButton.tsx
export const FavoriteButton: React.FC<{ mediaId: string }> = ({ mediaId }) => {
  const { isFavorite, toggleFavorite } = useFavorite(mediaId);
  
  return (
    <Button
      variant={isFavorite ? 'filled' : 'outlined'}
      onClick={toggleFavorite}
      icon={isFavorite ? <HeartFilled /> : <HeartOutlined />}
    >
      {isFavorite ? 'Remove from Favorites' : 'Add to Favorites'}
    </Button>
  );
};
```

3. **Add Favorites page:**
```typescript
// catalog-web/src/pages/Favorites.tsx
export const FavoritesPage: React.FC = () => {
  const { favorites, isLoading } = useFavorites();
  
  return (
    <Page title="My Favorites">
      {isLoading ? (
        <LoadingSpinner />
      ) : (
        <MediaGrid items={favorites} />
      )}
    </Page>
  );
};
```

**Estimated Time:** 20 hours

**Phase 4.2 Total: 72 hours**

---

### 4.3 Submodule Integration

#### Task 4.3.1: Wire Database Submodule
**Module:** `Database/`  
**Purpose:** Database abstractions  
**Status:** NOT WIRED

**Integration Steps:**

1. **Add to go.mod:**
```go
replace digital.vasic/database => ./Database

require (
    digital.vasic/database v0.0.0
)
```

2. **Migrate existing code:**
```bash
# Find usages
grep -r "database\." --include="*.go" | grep -v "_test.go"

# Update imports
sed -i 's|"catalog-api/database"|"digital.vasic/database"|g'
```

3. **Update factory:**
```go
// database/factory.go
import "digital.vasic/database"

func NewDB(config Config) (*database.DB, error) {
    return database.New(config)
}
```

**Estimated Time:** 16 hours

---

#### Task 4.3.2: Wire Observability Submodule
**Module:** `Observability/`  
**Purpose:** Metrics and tracing  
**Status:** NOT WIRED

**Integration Steps:**

1. **Add to go.mod**
2. **Replace existing metrics:**
```go
// Replace custom metrics with observability module
import "digital.vasic/observability"

metrics := observability.NewMetrics()
metrics.Counter("api.requests").Inc()
```

3. **Add OpenTelemetry tracing:**
```go
import "digital.vasic/observability/tracing"

tracer := tracing.NewTracer("catalog-api")
ctx, span := tracer.Start(ctx, "operation")
defer span.End()
```

**Estimated Time:** 20 hours

---

#### Task 4.3.3: Wire Security Submodule
**Module:** `Security/`  
**Purpose:** Security utilities  
**Status:** NOT WIRED

**Integration Steps:**

1. **Add to go.mod**
2. **Replace security functions:**
```go
import "digital.vasic/security"

// Replace custom password hashing
hash := security.HashPassword(password)
valid := security.VerifyPassword(password, hash)

// Replace JWT handling
token := security.GenerateJWT(claims)
claims, err := security.ValidateJWT(token)
```

**Estimated Time:** 12 hours

---

#### Task 4.3.4: Evaluate Remaining Submodules

**Submodules to EVALUATE:**

| Submodule | Decision | Action |
|-----------|----------|--------|
| Discovery | Wire if needed | TBD |
| Media | Wire if needed | TBD |
| Middleware | Wire if needed | TBD |
| RateLimiter | Wire if needed | TBD |
| Storage | Wire if needed | TBD |
| Streaming | Wire if needed | TBD |
| Watcher | Wire if needed | TBD |
| Panoptic | Wire if needed | TBD |

**Decision Criteria:**
1. Does current implementation duplicate functionality?
2. Is the submodule mature and tested?
3. Does integration provide clear benefits?
4. Is effort justified?

**Estimated Time:** 16 hours

**Phase 4.3 Total: 64 hours**

---

### 4.4 Placeholder Implementations

#### Task 4.4.1: Implement Metadata Providers
**Scope:** 13 stubbed providers  
**Action:** Implement or remove

**Providers to implement:**

1. **TMDB Provider** (The Movie Database)
   - API integration
   - Rate limiting
   - Caching
   - Error handling

2. **IMDB Provider** (Internet Movie Database)
   - Web scraping or API
   - Data normalization

3. **TVDB Provider** (The TV Database)
   - API integration
   - Episode tracking

4. **Rotten Tomatoes Provider**
   - API or scraping
   - Rating aggregation

5. **Metacritic Provider**
   - API integration
   - Score normalization

6. **IGDB Provider** (Internet Game Database)
   - API integration
   - Game metadata

7. **Steam Provider**
   - Steam API
   - Game information

8. **GOG Provider**
   - GOG API
   - DRM-free games

9. **Epic Provider**
   - Epic Games Store API
   - Free games tracking

10. **MusicBrainz Provider**
    - Music metadata
    - Artist/album info

11. **Discogs Provider**
    - Music database
    - Release information

12. **ComicVine Provider**
    - Comic book data
    - Issue tracking

13. **Google Books Provider**
    - Book metadata
    - ISBN lookup

**Decision:**
- Implement top 5 providers (TMDB, IMDB, TVDB, IGDB, MusicBrainz)
- Remove or stub remaining 8 with clear documentation

**Estimated Time:** 40 hours (for top 5)

---

#### Task 4.4.2: Implement Media Detection
**Scope:** 10 placeholder detection methods  
**Action:** FULL IMPLEMENTATION

**Implementation:**

```go
// internal/media/detector/detector.go

func (d *Detector) detectMovie(path string) bool {
    // Check file extension
    ext := strings.ToLower(filepath.Ext(path))
    movieExts := map[string]bool{".mp4", ".mkv", ".avi", ".mov", ".wmv"}
    if !movieExts[ext] {
        return false
    }
    
    // Check filename patterns
    filename := filepath.Base(path)
    moviePatterns := []*regexp.Regexp{
        regexp.MustCompile(`(?i).*\(\d{4}\).*`),           // Movie (2020)
        regexp.MustCompile(`(?i).*[._-]\d{4}[._-].*`),     // Movie.2020.
        regexp.MustCompile(`(?i).*\d{4}p.*`),               // 1080p, 720p
    }
    
    for _, pattern := range moviePatterns {
        if pattern.MatchString(filename) {
            return true
        }
    }
    
    // Check metadata
    metadata := d.extractMetadata(path)
    return metadata.Duration > 45*time.Minute && metadata.Duration < 4*time.Hour
}

func (d *Detector) detectTVShow(path string) bool {
    // Check for season/episode patterns
    filename := filepath.Base(path)
    patterns := []*regexp.Regexp{
        regexp.MustCompile(`(?i)s\d{1,2}[._-]?e\d{1,2}`),   // S01E01
        regexp.MustCompile(`(?i)season[._-]?\d+`),          // Season 1
        regexp.MustCompile(`(?i)\d{1,2}x\d{1,2}`),         // 1x01
    }
    
    for _, pattern := range patterns {
        if pattern.MatchString(filename) {
            return true
        }
    }
    
    return false
}

// Implement remaining 8 detection methods similarly...
```

**Tests Required:**
- Test each detection method
- Test edge cases
- Test false positives
- Test performance

**Estimated Time:** 20 hours

**Phase 4.4 Total: 60 hours**

---

### Phase 4 Deliverables

1. ✅ Zero dead code
2. ✅ All unconnected services integrated
3. ✅ Submodules wired or removed
4. ✅ Metadata providers implemented
5. ✅ Media detection working
6. ✅ All integrations tested
7. ✅ Integration documentation

**Phase 4 Total: 212 hours** (Note: Extended from 160 due to complexity)

---

## PHASE 5: SECURITY & SCANNING
**Duration:** Weeks 13-14 (80 hours)  
**Goal:** Complete security posture with all tools  
**Success Criteria:** All security tools installed, zero critical vulnerabilities

### 5.1 Security Tool Installation

#### Task 5.1.1: Install Trivy
**Purpose:** Container vulnerability scanning  
**Status:** NOT INSTALLED

**Installation:**

```bash
# Install Trivy
curl -sfL https://raw.githubusercontent.com/aquasecurity/trivy/main/contrib/install.sh | sh

# Add to docker-compose.security.yml
services:
  trivy:
    image: aquasec/trivy:latest
    volumes:
      - .:/app
      - /var/run/docker.sock:/var/run/docker.sock
    command: ["trivy", "fs", "--severity", "HIGH,CRITICAL", "/app"]
```

**Scripts:**

```bash
#!/bin/bash
# scripts/security-scan-trivy.sh

echo "Running Trivy security scan..."

# Filesystem scan
trivy fs --severity HIGH,CRITICAL \
  --exit-code 1 \
  --format sarif \
  --output reports/trivy-results.sarif \
  .

# Container image scan
if [ -f "Dockerfile" ]; then
  podman build -t catalogizer:latest .
  trivy image --severity HIGH,CRITICAL \
    --exit-code 1 \
    catalogizer:latest
fi

echo "Trivy scan complete"
```

**Estimated Time:** 8 hours

---

#### Task 5.1.2: Install Gosec
**Purpose:** Go security checker  
**Status:** NOT INSTALLED

**Installation:**

```bash
# Install Gosec
go install github.com/securego/gosec/v2/cmd/gosec@latest

# Run scan
gosec -fmt sarif -out reports/gosec-results.sarif ./...
```

**Configuration:**

```yaml
# .gosec.config.yml
global:
  nosec: false
  show-ignored: false
  audit: true
  exclude:
    - G104 # Audit errors not checked
    - G304 # File path provided as taint input
```

**Script:**

```bash
#!/bin/bash
# scripts/security-scan-gosec.sh

echo "Running Gosec security scan..."

cd catalog-api

gosec -fmt sarif \
  -out ../reports/gosec-results.sarif \
  -exclude=G104,G304 \
  ./...

if [ $? -ne 0 ]; then
  echo "Gosec found security issues"
  exit 1
fi

echo "Gosec scan complete"
```

**Estimated Time:** 6 hours

---

#### Task 5.1.3: Install Nancy
**Purpose:** Go dependency vulnerability scanner  
**Status:** NOT INSTALLED

**Installation:**

```bash
# Install Nancy
go install github.com/sonatypecommunity/nancy@latest

# Generate dependencies and scan
cd catalog-api
go list -json -deps ./... | nancy sleuth
```

**Script:**

```bash
#!/bin/bash
# scripts/security-scan-nancy.sh

echo "Running Nancy dependency scan..."

cd catalog-api

# Generate go.sum if needed
go mod tidy

# Scan dependencies
go list -json -deps ./... | nancy sleuth \
  --output json \
  --outputfile ../reports/nancy-results.json

if [ $? -ne 0 ]; then
  echo "Nancy found vulnerable dependencies"
  exit 1
fi

echo "Nancy scan complete"
```

**Estimated Time:** 4 hours

---

#### Task 5.1.4: Install Semgrep
**Purpose:** Static analysis security testing  
**Status:** NOT INSTALLED

**Installation:**

```bash
# Install Semgrep
pip install semgrep

# Or use Docker
podman pull returntocorp/semgrep
```

**Configuration:**

```yaml
# .semgrep.yml
rules:
  - id: insecure-random
    patterns:
      - pattern: math_rand.$FUNC(...)
    message: "Insecure random number generator"
    languages: [go]
    severity: WARNING
    
  - id: sql-injection
    patterns:
      - pattern: db.Query($X + $Y)
    message: "Potential SQL injection"
    languages: [go]
    severity: ERROR
```

**Script:**

```bash
#!/bin/bash
# scripts/security-scan-semgrep.sh

echo "Running Semgrep SAST scan..."

semgrep --config=auto \
  --json \
  --output reports/semgrep-results.json \
  --severity=ERROR \
  .

if [ $? -ne 0 ]; then
  echo "Semgrep found security issues"
  exit 1
fi

echo "Semgrep scan complete"
```

**Estimated Time:** 6 hours

---

#### Task 5.1.5: Install Falco (Runtime Security)
**Purpose:** Runtime security monitoring  
**Status:** NOT INSTALLED

**Installation:**

```yaml
# docker-compose.security.yml
services:
  falco:
    image: falcosecurity/falco:latest
    privileged: true
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
      - /dev:/dev
      - /proc:/host/proc:ro
      - /boot:/host/boot:ro
      - /lib/modules:/host/lib/modules:ro
      - /usr:/host/usr:ro
      - ./falco-rules:/etc/falco/rules.d
```

**Configuration:**

```yaml
# falco-rules/catalogizer.yaml
- rule: Unauthorized API Access
  desc: Detect unauthorized access to API endpoints
  condition: spawned_process and proc.name=catalog-api and not user.name=appuser
  output: "Unauthorized API access (user=%user.name)"
  priority: WARNING
```

**Estimated Time:** 8 hours

---

#### Task 5.1.6: Configure Snyk
**Purpose:** Dependency and container scanning  
**Status:** CONFIGURED BUT VERIFY

**Verification Steps:**

```bash
# Verify Snyk configuration
snyk config get api
snyk auth

# Run tests
snyk test --all-projects
snyk container test catalogizer:latest
snyk iac test .
```

**Script:**

```bash
#!/bin/bash
# scripts/security-scan-snyk.sh

echo "Running Snyk security scan..."

# Test dependencies
snyk test --all-projects --json > reports/snyk-deps.json

# Test container
if [ -f "Dockerfile" ]; then
  snyk container test catalogizer:latest --json > reports/snyk-container.json
fi

# Test IaC
snyk iac test . --json > reports/snyk-iac.json

echo "Snyk scan complete"
```

**Estimated Time:** 4 hours

---

#### Task 5.1.7: Configure SonarQube
**Purpose:** Code quality and security analysis  
**Status:** CONFIGURED BUT VERIFY

**Verification:**

```bash
# Verify SonarQube configuration
if [ -f "sonar-project.properties" ]; then
  echo "SonarQube configuration exists"
  cat sonar-project.properties
fi

# Run SonarQube scanner
sonar-scanner \
  -Dsonar.projectKey=catalogizer \
  -Dsonar.sources=. \
  -Dsonar.host.url=$SONAR_HOST \
  -Dsonar.login=$SONAR_TOKEN
```

**Script:**

```bash
#!/bin/bash
# scripts/security-scan-sonarqube.sh

echo "Running SonarQube analysis..."

# Run tests with coverage
GOMAXPROCS=3 go test ./... -coverprofile=coverage.out -p 2 -parallel 2

# Run SonarQube scanner
sonar-scanner \
  -Dsonar.projectKey=catalogizer \
  -Dsonar.sources=catalog-api \
  -Dsonar.tests=catalog-api \
  -Dsonar.test.inclusions="**/*_test.go" \
  -Dsonar.go.coverage.reportPaths=coverage.out \
  -Dsonar.host.url=${SONAR_HOST:-http://localhost:9000} \
  -Dsonar.login=${SONAR_TOKEN}

echo "SonarQube analysis complete"
```

**Estimated Time:** 4 hours

**Phase 5.1 Total: 40 hours**

---

### 5.2 Vulnerability Remediation

#### Task 5.2.1: Critical Vulnerability Fix Process
**Goal:** Zero critical vulnerabilities

**Process:**

1. **Run all scans:**
```bash
./scripts/security-scan-all.sh
```

2. **Analyze results:**
- Parse SARIF/JSON outputs
- Categorize by severity
- Prioritize critical/high

3. **Create remediation tickets:**
```yaml
# Example ticket
title: "Fix SQL Injection in media_handler.go:145"
severity: critical
tool: gosec
file: catalog-api/handlers/media_handler.go
line: 145
rule: G202
description: "SQL query construction using string concatenation"
fix: "Use parameterized queries"
```

4. **Implement fixes:**
```go
// BEFORE (vulnerable)
query := "SELECT * FROM media WHERE title = '" + title + "'"

// AFTER (safe)
query := "SELECT * FROM media WHERE title = $1"
rows, err := db.Query(query, title)
```

5. **Verify fixes:**
```bash
# Re-run specific scan
gosec ./...

# Run tests
go test ./...
```

**Estimated Time:** 20 hours

---

#### Task 5.2.2: Dependency Updates
**Goal:** All dependencies up-to-date

**Process:**

```bash
# Check for updates
cd catalog-api
go list -u -m all

# Update dependencies
go get -u ./...
go mod tidy

# Test
GOMAXPROCS=3 go test ./... -p 2 -parallel 2

# Security scan
./scripts/security-scan-nancy.sh
```

**Estimated Time:** 8 hours

---

#### Task 5.2.3: Secret Scanning
**Goal:** No hardcoded secrets

**Tools:**
- GitLeaks
- TruffleHog
- Custom secret scanner

**Script:**

```bash
#!/bin/bash
# scripts/security-scan-secrets.sh

echo "Scanning for secrets..."

# Install GitLeaks
if ! command -v gitleaks &> /dev/null; then
  go install github.com/zricethezav/gitleaks/v8@latest
fi

# Scan repository
gitleaks detect --verbose --source . --report-format json --report-path reports/gitleaks-results.json

# Scan for common patterns
grep -r "AKIA[0-9A-Z]{16}" --include="*.go" --include="*.ts" . || true
grep -r "private_key" --include="*.go" --include="*.ts" . || true
grep -r "password.*=.*\"" --include="*.go" --include="*.ts" . || true

echo "Secret scan complete"
```

**Estimated Time:** 6 hours

**Phase 5.2 Total: 34 hours**

---

### 5.3 Security Enhancements

#### Task 5.3.1: Implement MFA/2FA
**Status:** NOT IMPLEMENTED  
**Priority:** HIGH

**Implementation:**

```go
// services/auth_service.go

func (s *AuthService) Enable2FA(userID string) (*TOTPConfig, error) {
    // Generate TOTP secret
    secret := make([]byte, 32)
    if _, err := rand.Read(secret); err != nil {
        return nil, err
    }
    
    // Store secret
    encryptedSecret, err := s.encryptSecret(secret)
    if err != nil {
        return nil, err
    }
    
    if err := s.db.Enable2FA(userID, encryptedSecret); err != nil {
        return nil, err
    }
    
    // Generate QR code
    qrCode, err := s.generateQRCode(userID, secret)
    if err != nil {
        return nil, err
    }
    
    return &TOTPConfig{
        Secret: base32.StdEncoding.EncodeToString(secret),
        QRCode: qrCode,
    }, nil
}

func (s *AuthService) Verify2FA(userID string, code string) error {
    secret, err := s.db.Get2FASecret(userID)
    if err != nil {
        return err
    }
    
    if !totp.Validate(code, secret) {
        return errors.New("invalid 2FA code")
    }
    
    return nil
}
```

**Frontend:**

```typescript
// components/TwoFactorSetup.tsx
export const TwoFactorSetup: React.FC = () => {
  const [qrCode, setQRCode] = useState<string>('');
  const [verificationCode, setVerificationCode] = useState('');
  
  const handleEnable = async () => {
    const config = await authService.enable2FA();
    setQRCode(config.qrCode);
  };
  
  const handleVerify = async () => {
    await authService.verify2FA(verificationCode);
    // Redirect to success
  };
  
  return (
    <div>
      <Button onClick={handleEnable}>Enable 2FA</Button>
      {qrCode && <img src={qrCode} alt="2FA QR Code" />}
      <Input 
        value={verificationCode}
        onChange={setVerificationCode}
        placeholder="Enter verification code"
      />
      <Button onClick={handleVerify}>Verify</Button>
    </div>
  );
};
```

**Estimated Time:** 16 hours

---

#### Task 5.3.2: Implement API Key Rotation
**Status:** NOT IMPLEMENTED  
**Priority:** MEDIUM

**Implementation:**

```go
// services/api_key_service.go

type APIKeyService struct {
    db *database.DB
    cache *cache.Cache
}

type APIKey struct {
    ID string
    UserID string
    Key string
    CreatedAt time.Time
    ExpiresAt time.Time
    LastUsedAt *time.Time
}

func (s *APIKeyService) CreateKey(userID string, expiresIn time.Duration) (*APIKey, error) {
    key := generateSecureKey(32)
    
    apiKey := &APIKey{
        ID: uuid.New().String(),
        UserID: userID,
        Key: hashKey(key),
        CreatedAt: time.Now(),
        ExpiresAt: time.Now().Add(expiresIn),
    }
    
    if err := s.db.CreateAPIKey(apiKey); err != nil {
        return nil, err
    }
    
    // Return unhashed key only once
    return &APIKey{
        ID: apiKey.ID,
        Key: key, // Unhashed for display
        ExpiresAt: apiKey.ExpiresAt,
    }, nil
}

func (s *APIKeyService) RotateKey(keyID string) (*APIKey, error) {
    // Invalidate old key
    if err := s.db.RevokeAPIKey(keyID); err != nil {
        return nil, err
    }
    
    // Get user ID
    userID, err := s.db.GetAPIKeyUser(keyID)
    if err != nil {
        return nil, err
    }
    
    // Create new key
    return s.CreateKey(userID, 90*24*time.Hour)
}
```

**Estimated Time:** 8 hours

---

#### Task 5.3.3: Implement Comprehensive Audit Logging
**Status:** PARTIAL  
**Priority:** HIGH

**Implementation:**

```go
// middleware/audit.go

type AuditMiddleware struct {
    logger *zap.Logger
    db *database.DB
}

func (m *AuditMiddleware) Log() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        // Capture request body
        var requestBody []byte
        if c.Request.Body != nil {
            requestBody, _ = io.ReadAll(c.Request.Body)
            c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
        }
        
        // Process request
        c.Next()
        
        // Log audit event
        event := &AuditEvent{
            ID: uuid.New().String(),
            Timestamp: time.Now(),
            UserID: c.GetString("userID"),
            IP: c.ClientIP(),
            Method: c.Request.Method,
            Path: c.Request.URL.Path,
            Status: c.Writer.Status(),
            Duration: time.Since(start),
            UserAgent: c.Request.UserAgent(),
            RequestBody: truncate(string(requestBody), 1000),
        }
        
        // Async write to database
        go m.db.SaveAuditEvent(event)
        
        // Structured logging
        m.logger.Info("api_request",
            zap.String("user_id", event.UserID),
            zap.String("method", event.Method),
            zap.String("path", event.Path),
            zap.Int("status", event.Status),
            zap.Duration("duration", event.Duration),
        )
    }
}
```

**Estimated Time:** 12 hours

---

#### Task 5.3.4: Implement Rate Limiting per User
**Status:** BASIC  
**Priority:** MEDIUM

**Implementation:**

```go
// middleware/rate_limit.go

type RateLimiter struct {
    store *cache.Cache
    limits map[string]RateLimit
}

type RateLimit struct {
    Requests int
    Window time.Duration
}

var defaultLimits = map[string]RateLimit{
    "anonymous": {Requests: 30, Window: time.Minute},
    "user": {Requests: 100, Window: time.Minute},
    "admin": {Requests: 1000, Window: time.Minute},
}

func (rl *RateLimiter) Limit() gin.HandlerFunc {
    return func(c *gin.Context) {
        userID := c.GetString("userID")
        if userID == "" {
            userID = c.ClientIP()
        }
        
        role := c.GetString("role")
        if role == "" {
            role = "anonymous"
        }
        
        limit := defaultLimits[role]
        key := fmt.Sprintf("rate_limit:%s:%s", userID, c.Request.URL.Path)
        
        // Check current count
        count, err := rl.store.Increment(key, 1, limit.Window)
        if err != nil {
            c.AbortWithStatus(500)
            return
        }
        
        if count > limit.Requests {
            c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit.Requests))
            c.Header("X-RateLimit-Remaining", "0")
            c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(limit.Window).Unix()))
            c.AbortWithStatusJSON(429, gin.H{"error": "rate limit exceeded"})
            return
        }
        
        c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit.Requests))
        c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", limit.Requests-int(count)))
        
        c.Next()
    }
}
```

**Estimated Time:** 8 hours

**Phase 5.3 Total: 44 hours**

---

### Phase 5 Deliverables

1. ✅ All security tools installed (Trivy, Gosec, Nancy, Semgrep, Falco)
2. ✅ Snyk and SonarQube verified
3. ✅ Zero critical vulnerabilities
4. ✅ MFA/2FA implemented
5. ✅ API key rotation implemented
6. ✅ Comprehensive audit logging
7. ✅ Granular rate limiting
8. ✅ Security documentation

**Phase 5 Total: 118 hours** (Note: Extended from 80 due to security enhancements)

---

## PHASE 6: PERFORMANCE & OPTIMIZATION
**Duration:** Weeks 15-18 (160 hours)  
**Goal:** Optimize performance with lazy loading, semaphores, non-blocking I/O  
**Success Criteria:** 50%+ performance improvement, zero blocking operations

### 6.1 Lazy Loading Implementation

#### Task 6.1.1: Lazy Database Connection
**File:** `catalog-api/database/connection.go`

**Implementation:**

```go
type LazyDB struct {
    config Config
    db *sql.DB
    once sync.Once
    mu sync.RWMutex
}

func (ldb *LazyDB) DB() (*sql.DB, error) {
    ldb.once.Do(func() {
        db, err := sql.Open(ldb.config.Driver, ldb.config.DSN)
        if err != nil {
            return
        }
        
        db.SetMaxOpenConns(ldb.config.MaxOpenConns)
        db.SetMaxIdleConns(ldb.config.MaxIdleConns)
        db.SetConnMaxLifetime(ldb.config.ConnMaxLifetime)
        
        ldb.mu.Lock()
        ldb.db = db
        ldb.mu.Unlock()
    })
    
    ldb.mu.RLock()
    defer ldb.mu.RUnlock()
    
    if ldb.db == nil {
        return nil, errors.New("failed to initialize database")
    }
    
    return ldb.db, nil
}
```

**Estimated Time:** 8 hours

---

#### Task 6.1.2: Lazy Cache Initialization
**File:** `catalog-api/internal/cache/redis.go`

**Implementation:**

```go
type LazyCache struct {
    config RedisConfig
    client *redis.Client
    once sync.Once
    mu sync.RWMutex
}

func (lc *LazyCache) Client() (*redis.Client, error) {
    lc.once.Do(func() {
        client := redis.NewClient(&redis.Options{
            Addr: lc.config.Addr,
            Password: lc.config.Password,
            DB: lc.config.DB,
        })
        
        // Test connection
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        
        if err := client.Ping(ctx).Err(); err != nil {
            return
        }
        
        lc.mu.Lock()
        lc.client = client
        lc.mu.Unlock()
    })
    
    lc.mu.RLock()
    defer lc.mu.RUnlock()
    
    if lc.client == nil {
        return nil, errors.New("failed to initialize cache")
    }
    
    return lc.client, nil
}
```

**Estimated Time:** 6 hours

---

#### Task 6.1.3: Lazy Media Metadata Loading
**File:** `catalog-api/services/media_service.go`

**Implementation:**

```go
type LazyMediaItem struct {
    id string
    mu sync.RWMutex
    loaded bool
    
    // Lazy-loaded fields
    _metadata *MediaMetadata
    _thumbnails []Thumbnail
    _subtitles []Subtitle
}

func (lmi *LazyMediaItem) Metadata() (*MediaMetadata, error) {
    lmi.mu.RLock()
    if lmi.loaded && lmi._metadata != nil {
        defer lmi.mu.RUnlock()
        return lmi._metadata, nil
    }
    lmi.mu.RUnlock()
    
    lmi.mu.Lock()
    defer lmi.mu.Unlock()
    
    if lmi._metadata != nil {
        return lmi._metadata, nil
    }
    
    // Load from database
    metadata, err := loadMetadata(lmi.id)
    if err != nil {
        return nil, err
    }
    
    lmi._metadata = metadata
    return metadata, nil
}
```

**Estimated Time:** 12 hours

---

#### Task 6.1.4: Frontend Lazy Loading
**Scope:** catalog-web React components

**Implementation:**

```typescript
// Lazy load components
const MediaGrid = lazy(() => import('./components/MediaGrid'));
const AnalyticsDashboard = lazy(() => import('./components/AnalyticsDashboard'));
const ReportingWizard = lazy(() => import('./components/ReportingWizard'));

// Lazy load routes
const routes = [
  {
    path: '/media',
    component: lazy(() => import('./pages/MediaPage')),
  },
  {
    path: '/analytics',
    component: lazy(() => import('./pages/AnalyticsPage')),
  },
];

// Lazy load images
const LazyImage: React.FC<{ src: string; alt: string }> = ({ src, alt }) => {
  const [isLoaded, setIsLoaded] = useState(false);
  
  return (
    <img
      src={src}
      alt={alt}
      loading="lazy"
      onLoad={() => setIsLoaded(true)}
      style={{ opacity: isLoaded ? 1 : 0, transition: 'opacity 0.3s' }}
    />
  );
};

// Intersection Observer for lazy loading
const useLazyLoad = (options?: IntersectionObserverInit) => {
  const ref = useRef<HTMLDivElement>(null);
  const [isVisible, setIsVisible] = useState(false);
  
  useEffect(() => {
    const observer = new IntersectionObserver(([entry]) => {
      if (entry.isIntersecting) {
        setIsVisible(true);
        observer.disconnect();
      }
    }, options);
    
    if (ref.current) {
      observer.observe(ref.current);
    }
    
    return () => observer.disconnect();
  }, [options]);
  
  return { ref, isVisible };
};
```

**Estimated Time:** 16 hours

**Phase 6.1 Total: 42 hours**

---

### 6.2 Semaphore Implementation

#### Task 6.2.1: Global Semaphore Manager
**File:** `catalog-api/internal/concurrency/semaphore.go`

**Implementation:**

```go
package concurrency

import (
    "context"
    "sync"
)

type SemaphoreManager struct {
    semaphores map[string]*WeightedSemaphore
    mu sync.RWMutex
}

type WeightedSemaphore struct {
    name string
    size int64
    ch chan struct{}
    mu sync.Mutex
    current int64
}

func NewSemaphoreManager() *SemaphoreManager {
    return &SemaphoreManager{
        semaphores: make(map[string]*WeightedSemaphore),
    }
}

func (sm *SemaphoreManager) Register(name string, size int64) *WeightedSemaphore {
    sm.mu.Lock()
    defer sm.mu.Unlock()
    
    if _, exists := sm.semaphores[name]; exists {
        return sm.semaphores[name]
    }
    
    sem := &WeightedSemaphore{
        name: name,
        size: size,
        ch: make(chan struct{}, size),
    }
    
    // Pre-fill semaphore
    for i := int64(0); i < size; i++ {
        sem.ch <- struct{}{}
    }
    
    sm.semaphores[name] = sem
    return sem
}

func (sm *SemaphoreManager) Get(name string) (*WeightedSemaphore, bool) {
    sm.mu.RLock()
    defer sm.mu.RUnlock()
    
    sem, exists := sm.semaphores[name]
    return sem, exists
}

func (ws *WeightedSemaphore) Acquire(ctx context.Context) error {
    select {
    case <-ctx.Done():
        return ctx.Err()
    case <-ws.ch:
        ws.mu.Lock()
        ws.current--
        ws.mu.Unlock()
        return nil
    }
}

func (ws *WeightedSemaphore) Release() {
    ws.mu.Lock()
    ws.current++
    ws.mu.Unlock()
    
    select {
    case ws.ch <- struct{}{}:
    default:
        // Semaphore full, shouldn't happen
    }
}

func (ws *WeightedSemaphore) Stats() (available, total int64) {
    ws.mu.Lock()
    defer ws.mu.Unlock()
    return ws.current, ws.size
}
```

**Estimated Time:** 10 hours

---

#### Task 6.2.2: Scan Operation Semaphore
**File:** `catalog-api/services/scan_service.go`

**Implementation:**

```go
func (s *ScanService) ScanDirectory(ctx context.Context, path string) error {
    // Use semaphore to limit concurrent scans
    sem := s.semaphoreManager.Get("scan_operations")
    if sem == nil {
        sem = s.semaphoreManager.Register("scan_operations", 5)
    }
    
    if err := sem.Acquire(ctx); err != nil {
        return fmt.Errorf("failed to acquire scan semaphore: %w", err)
    }
    defer sem.Release()
    
    // Perform scan...
    return s.performScan(ctx, path)
}
```

**Estimated Time:** 6 hours

---

#### Task 6.2.3: API Request Semaphore
**File:** `catalog-api/middleware/semaphore.go`

**Implementation:**

```go
func SemaphoreMiddleware(sem *concurrency.WeightedSemaphore) gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx, cancel := context.WithTimeout(c.Request.Context(), 30*time.Second)
        defer cancel()
        
        if err := sem.Acquire(ctx); err != nil {
            c.AbortWithStatusJSON(503, gin.H{
                "error": "server too busy",
                "retry_after": 5,
            })
            return
        }
        defer sem.Release()
        
        c.Next()
    }
}

// In main.go
apiSem := semaphoreManager.Register("api_requests", 100)
router.Use(middleware.SemaphoreMiddleware(apiSem))
```

**Estimated Time:** 6 hours

**Phase 6.2 Total: 22 hours**

---

### 6.3 Non-Blocking Operations

#### Task 6.3.1: Non-Blocking Cache Operations
**File:** `catalog-api/internal/cache/redis.go`

**Implementation:**

```go
func (c *RedisCache) GetAsync(key string) <-chan CacheResult {
    resultChan := make(chan CacheResult, 1)
    
    go func() {
        defer close(resultChan)
        
        value, err := c.Get(key)
        resultChan <- CacheResult{Value: value, Error: err}
    }()
    
    return resultChan
}

func (c *RedisCache) SetAsync(key string, value interface{}, ttl time.Duration) <-chan error {
    errChan := make(chan error, 1)
    
    go func() {
        defer close(errChan)
        errChan <- c.Set(key, value, ttl)
    }()
    
    return errChan
}

// Usage
select {
case result := <-cache.GetAsync("key"):
    if result.Error != nil {
        // Handle error
    }
    // Use result.Value
case <-time.After(100 * time.Millisecond):
    // Timeout, use default
}
```

**Estimated Time:** 8 hours

---

#### Task 6.3.2: Non-Blocking Database Queries
**File:** `catalog-api/database/async.go`

**Implementation:**

```go
func (db *DB) QueryAsync(ctx context.Context, query string, args ...interface{}) <-chan QueryResult {
    resultChan := make(chan QueryResult, 1)
    
    go func() {
        defer close(resultChan)
        
        rows, err := db.QueryContext(ctx, query, args...)
        resultChan <- QueryResult{Rows: rows, Error: err}
    }()
    
    return resultChan
}

func (db *DB) QueryRowAsync(ctx context.Context, query string, args ...interface{}) <-chan RowResult {
    resultChan := make(chan RowResult, 1)
    
    go func() {
        defer close(resultChan)
        
        row := db.QueryRowContext(ctx, query, args...)
        resultChan <- RowResult{Row: row}
    }()
    
    return resultChan
}
```

**Estimated Time:** 8 hours

---

#### Task 6.3.3: Async Media Processing
**File:** `catalog-api/services/media_processor.go`

**Implementation:**

```go
type AsyncMediaProcessor struct {
    workerPool *WorkerPool
    resultCache *cache.Cache
}

type ProcessingJob struct {
    ID string
    MediaID string
    Operation string
    Params map[string]interface{}
}

type ProcessingResult struct {
    JobID string
    Success bool
    Result interface{}
    Error error
    CompletedAt time.Time
}

func (amp *AsyncMediaProcessor) ProcessAsync(job ProcessingJob) (string, error) {
    // Queue job
    if err := amp.workerPool.Submit(job); err != nil {
        return "", err
    }
    
    // Return job ID for status checking
    return job.ID, nil
}

func (amp *AsyncMediaProcessor) GetResult(jobID string) (*ProcessingResult, error) {
    // Check cache for result
    result, err := amp.resultCache.Get("job:" + jobID)
    if err != nil {
        return nil, fmt.Errorf("job not found or not completed: %w", err)
    }
    
    return result.(*ProcessingResult), nil
}

func (amp *AsyncMediaProcessor) WaitForResult(ctx context.Context, jobID string, timeout time.Duration) (*ProcessingResult, error) {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()
    
    ticker := time.NewTicker(100 * time.Millisecond)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-ticker.C:
            result, err := amp.GetResult(jobID)
            if err == nil {
                return result, nil
            }
        }
    }
}
```

**Estimated Time:** 12 hours

**Phase 6.3 Total: 28 hours**

---

### 6.4 Database Optimization

#### Task 6.4.1: Fix N+1 Queries
**File:** `catalog-api/repository/media_repository.go`

**Before (N+1):**
```go
// BAD: N+1 query problem
mediaItems, _ := db.Query("SELECT * FROM media_items")
for _, item := range mediaItems {
    // This query runs for each item!
    files, _ := db.Query("SELECT * FROM files WHERE media_id = ?", item.ID)
    item.Files = files
}
```

**After (Single Query):**
```go
// GOOD: Single query with JOIN
rows, err := db.Query(`
    SELECT 
        mi.*,
        f.id as file_id,
        f.path as file_path,
        f.size as file_size
    FROM media_items mi
    LEFT JOIN files f ON f.media_id = mi.id
    WHERE mi.id IN (?)
`, mediaIDs)

// Process with map to avoid duplicates
mediaMap := make(map[string]*MediaItem)
for rows.Next() {
    var item MediaItem
    var file File
    // Scan into item and file
    
    if existing, ok := mediaMap[item.ID]; ok {
        existing.Files = append(existing.Files, file)
    } else {
        item.Files = []File{file}
        mediaMap[item.ID] = &item
    }
}
```

**Estimated Time:** 16 hours

---

#### Task 6.4.2: Add Missing Indexes
**File:** `catalog-api/database/migrations/`

**Implementation:**

```sql
-- Migration: add_performance_indexes.sql
-- Add indexes for common queries

-- Media search indexes
CREATE INDEX IF NOT EXISTS idx_media_items_title ON media_items(title);
CREATE INDEX IF NOT EXISTS idx_media_items_type ON media_items(type);
CREATE INDEX IF NOT EXISTS idx_media_items_created_at ON media_items(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_media_items_title_type ON media_items(title, type);

-- File indexes
CREATE INDEX IF NOT EXISTS idx_files_media_id ON files(media_id);
CREATE INDEX IF NOT EXISTS idx_files_path ON files(path);
CREATE INDEX IF NOT EXISTS idx_files_size ON files(size);

-- User indexes
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_email ON users(email);

-- Favorites indexes
CREATE INDEX IF NOT EXISTS idx_favorites_user_id ON favorites(user_id);
CREATE INDEX IF NOT EXISTS idx_favorites_media_id ON favorites(media_id);
CREATE INDEX IF NOT EXISTS idx_favorites_user_media ON favorites(user_id, media_id);

-- Audit log indexes
CREATE INDEX IF NOT EXISTS idx_audit_events_timestamp ON audit_events(timestamp DESC);
CREATE INDEX IF NOT EXISTS idx_audit_events_user_id ON audit_events(user_id);

-- Composite indexes for common query patterns
CREATE INDEX IF NOT EXISTS idx_media_items_search ON media_items(type, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_files_scan ON files(storage_id, scanned_at);
```

**Estimated Time:** 8 hours

---

#### Task 6.4.3: Batch Insert Implementation
**File:** `catalog-api/services/scan_service.go`

**Implementation:**

```go
func (s *ScanService) batchInsertFiles(files []File) error {
    if len(files) == 0 {
        return nil
    }
    
    const batchSize = 1000
    
    for i := 0; i < len(files); i += batchSize {
        end := i + batchSize
        if end > len(files) {
            end = len(files)
        }
        
        batch := files[i:end]
        if err := s.insertBatch(batch); err != nil {
            return fmt.Errorf("failed to insert batch: %w", err)
        }
    }
    
    return nil
}

func (s *ScanService) insertBatch(files []File) error {
    // Build bulk insert query
    valueStrings := make([]string, 0, len(files))
    valueArgs := make([]interface{}, 0, len(files)*5)
    
    for i, file := range files {
        valueStrings = append(valueStrings, fmt.Sprintf("($%d,$%d,$%d,$%d,$%d)", i*5+1, i*5+2, i*5+3, i*5+4, i*5+5))
        valueArgs = append(valueArgs, file.ID, file.Path, file.Size, file.MediaID, file.StorageID)
    }
    
    query := fmt.Sprintf("INSERT INTO files (id, path, size, media_id, storage_id) VALUES %s", 
        strings.Join(valueStrings, ","))
    
    _, err := s.db.Exec(query, valueArgs...)
    return err
}
```

**Estimated Time:** 10 hours

---

#### Task 6.4.4: Query Timeout Implementation
**File:** `catalog-api/database/query.go`

**Implementation:**

```go
func (db *DB) QueryWithTimeout(timeout time.Duration, query string, args ...interface{}) (*sql.Rows, error) {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    
    return db.QueryContext(ctx, query, args...)
}

func (db *DB) ExecWithTimeout(timeout time.Duration, query string, args ...interface{}) (sql.Result, error) {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel()
    
    return db.ExecContext(ctx, query, args...)
}

// Slow query logger
func (db *DB) QueryLogged(query string, args ...interface{}) (*sql.Rows, error) {
    start := time.Now()
    rows, err := db.Query(query, args...)
    duration := time.Since(start)
    
    if duration > 1*time.Second {
        db.logger.Warn("slow query detected",
            zap.String("query", query),
            zap.Duration("duration", duration),
        )
    }
    
    return rows, err
}
```

**Estimated Time:** 6 hours

**Phase 6.4 Total: 40 hours**

---

### 6.5 Caching Strategy

#### Task 6.5.1: Multi-Level Cache
**File:** `catalog-api/internal/cache/multi_level.go`

**Implementation:**

```go
type MultiLevelCache struct {
    l1 *LRUCache      // Local in-memory
    l2 *RedisCache    // Distributed Redis
    l3 *DiskCache     // Persistent disk cache
}

func (mc *MultiLevelCache) Get(key string) (interface{}, error) {
    // Try L1 first (fastest)
    if val, err := mc.l1.Get(key); err == nil {
        return val, nil
    }
    
    // Try L2 (Redis)
    if val, err := mc.l2.Get(key); err == nil {
        // Populate L1 for next time
        mc.l1.Set(key, val, 1*time.Minute)
        return val, nil
    }
    
    // Try L3 (Disk)
    if val, err := mc.l3.Get(key); err == nil {
        // Populate L1 and L2
        mc.l1.Set(key, val, 1*time.Minute)
        mc.l2.Set(key, val, 10*time.Minute)
        return val, nil
    }
    
    return nil, ErrCacheMiss
}

func (mc *MultiLevelCache) Set(key string, value interface{}, ttl time.Duration) error {
    // Set in all levels with appropriate TTLs
    mc.l1.Set(key, value, ttl/10)        // L1: 10% of TTL
    mc.l2.Set(key, value, ttl)            // L2: Full TTL
    mc.l3.Set(key, value, ttl*24*7)      // L3: 1 week
    
    return nil
}
```

**Estimated Time:** 10 hours

---

#### Task 6.5.2: Cache Warming
**File:** `catalog-api/services/cache_warmer.go`

**Implementation:**

```go
type CacheWarmer struct {
    cache *MultiLevelCache
    db *database.DB
}

func (cw *CacheWarmer) WarmPopularMedia() error {
    // Get most accessed media
    rows, err := cw.db.Query(`
        SELECT media_id, COUNT(*) as views
        FROM media_views
        WHERE viewed_at > NOW() - INTERVAL '7 days'
        GROUP BY media_id
        ORDER BY views DESC
        LIMIT 100
    `)
    if err != nil {
        return err
    }
    defer rows.Close()
    
    for rows.Next() {
        var mediaID string
        var views int
        rows.Scan(&mediaID, &views)
        
        // Warm cache for this media
        go cw.warmMediaCache(mediaID)
    }
    
    return nil
}

func (cw *CacheWarmer) warmMediaCache(mediaID string) {
    // Fetch and cache media metadata
    media, _ := cw.getMediaWithFiles(mediaID)
    if media != nil {
        cw.cache.Set("media:"+mediaID, media, 1*time.Hour)
    }
}
```

**Estimated Time:** 6 hours

---

#### Task 6.5.3: Cache Invalidation Strategy
**File:** `catalog-api/internal/cache/invalidation.go`

**Implementation:**

```go
type CacheInvalidator struct {
    cache *MultiLevelCache
    eventBus *eventbus.EventBus
}

func (ci *CacheInvalidator) Start() {
    // Subscribe to events
    ci.eventBus.Subscribe("media.updated", ci.handleMediaUpdate)
    ci.eventBus.Subscribe("media.deleted", ci.handleMediaDelete)
    ci.eventBus.Subscribe("storage.scan.completed", ci.handleScanComplete)
}

func (ci *CacheInvalidator) handleMediaUpdate(event Event) {
    mediaID := event.Payload["media_id"].(string)
    
    // Invalidate specific cache entries
    ci.cache.Delete("media:" + mediaID)
    ci.cache.Delete("media:" + mediaID + ":files")
    
    // Invalidate list caches
    ci.cache.DeletePattern("media:list:*")
}

func (ci *CacheInvalidator) handleScanComplete(event Event) {
    storageID := event.Payload["storage_id"].(string)
    
    // Invalidate storage-related caches
    ci.cache.Delete("storage:" + storageID)
    ci.cache.DeletePattern("files:storage:" + storageID + ":*")
    
    // Warm cache with new data
    go ci.warmStorageCache(storageID)
}
```

**Estimated Time:** 8 hours

**Phase 6.5 Total: 24 hours**

---

### 6.6 Memory Management

#### Task 6.6.1: Object Pooling
**File:** `catalog-api/internal/pool/object_pool.go`

**Implementation:**

```go
type ObjectPool struct {
    pool sync.Pool
    maxSize int
    currentSize int32
}

func NewObjectPool(newFunc func() interface{}) *ObjectPool {
    return &ObjectPool{
        pool: sync.Pool{
            New: newFunc,
        },
        maxSize: 10000,
    }
}

func (op *ObjectPool) Get() interface{} {
    atomic.AddInt32(&op.currentSize, 1)
    return op.pool.Get()
}

func (op *ObjectPool) Put(x interface{}) {
    atomic.AddInt32(&op.currentSize, -1)
    op.pool.Put(x)
}

func (op *ObjectPool) Stats() (current, max int) {
    return int(atomic.LoadInt32(&op.currentSize)), op.maxSize
}

// Specialized pools
var (
    bufferPool = NewObjectPool(func() interface{} {
        return make([]byte, 32*1024)
    })
    
    stringBuilderPool = NewObjectPool(func() interface{} {
        return new(strings.Builder)
    })
)
```

**Estimated Time:** 6 hours

---

#### Task 6.6.2: Memory Profiling Integration
**File:** `catalog-api/internal/profiler/profiler.go`

**Implementation:**

```go
type MemoryProfiler struct {
    enabled bool
    threshold uint64
    logger *zap.Logger
}

func (mp *MemoryProfiler) Start() {
    if !mp.enabled {
        return
    }
    
    go func() {
        ticker := time.NewTicker(30 * time.Second)
        defer ticker.Stop()
        
        var m runtime.MemStats
        
        for range ticker.C {
            runtime.ReadMemStats(&m)
            
            if m.Alloc > mp.threshold {
                mp.logger.Warn("high memory usage detected",
                    zap.Uint64("alloc_mb", m.Alloc/1024/1024),
                    zap.Uint64("sys_mb", m.Sys/1024/1024),
                    zap.Uint32("num_gc", m.NumGC),
                )
                
                // Write heap profile
                mp.writeHeapProfile()
            }
        }
    }()
}

func (mp *MemoryProfiler) writeHeapProfile() {
    f, err := os.Create(fmt.Sprintf("heap_%d.prof", time.Now().Unix()))
    if err != nil {
        return
    }
    defer f.Close()
    
    pprof.WriteHeapProfile(f)
}
```

**Estimated Time:** 4 hours

**Phase 6.6 Total: 10 hours**

---

### Phase 6 Deliverables

1. ✅ Lazy loading implemented (database, cache, media, frontend)
2. ✅ Semaphore mechanisms (global, scan, API)
3. ✅ Non-blocking operations (cache, database, media processing)
4. ✅ Database optimization (N+1 fixed, indexes, batch insert, timeouts)
5. ✅ Multi-level caching strategy
6. ✅ Object pooling
7. ✅ Memory profiling
8. ✅ Performance benchmarks
9. ✅ 50%+ performance improvement measured

**Phase 6 Total: 166 hours** (Note: Extended from 160)

---

## PHASE 7: MONITORING & OBSERVABILITY
**Duration:** Weeks 19-20 (80 hours)  
**Goal:** Complete observability stack with monitoring, tracing, alerting  
**Success Criteria:** Full visibility into system health, automated alerting

### 7.1 AlertManager Configuration

#### Task 7.1.1: Install AlertManager
**File:** `docker-compose.monitoring.yml`

**Implementation:**

```yaml
services:
  alertmanager:
    image: prom/alertmanager:latest
    volumes:
      - ./monitoring/alertmanager:/etc/alertmanager
      - alertmanager-data:/alertmanager
    command:
      - '--config.file=/etc/alertmanager/alertmanager.yml'
      - '--storage.path=/alertmanager'
    ports:
      - "9093:9093"
    
  prometheus:
    image: prom/prometheus:latest
    volumes:
      - ./monitoring/prometheus:/etc/prometheus
      - prometheus-data:/prometheus
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--alertmanager.url=http://alertmanager:9093'
    depends_on:
      - alertmanager
```

**Configuration:**

```yaml
# monitoring/alertmanager/alertmanager.yml
global:
  smtp_smarthost: 'localhost:587'
  smtp_from: 'alerts@catalogizer.local'

route:
  receiver: 'default'
  routes:
    - match:
        severity: critical
      receiver: 'critical-alerts'
      continue: true
    - match:
        severity: warning
      receiver: 'warning-alerts'

templates:
  - '/etc/alertmanager/templates/*.tmpl'

receivers:
  - name: 'default'
    email_configs:
      - to: 'admin@catalogizer.local'
        
  - name: 'critical-alerts'
    email_configs:
      - to: 'oncall@catalogizer.local'
        priority: high
    slack_configs:
      - api_url: '${SLACK_WEBHOOK_URL}'
        channel: '#alerts-critical'
        
  - name: 'warning-alerts'
    email_configs:
      - to: 'team@catalogizer.local'
    slack_configs:
      - api_url: '${SLACK_WEBHOOK_URL}'
        channel: '#alerts-warning'
```

**Estimated Time:** 8 hours

---

#### Task 7.1.2: Define Alert Rules
**File:** `monitoring/prometheus/alerts.yml`

**Implementation:**

```yaml
groups:
  - name: catalogizer-alerts
    rules:
      # High error rate
      - alert: HighErrorRate
        expr: rate(http_requests_total{status=~"5.."}[5m]) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value }} errors per second"
          
      # High latency
      - alert: HighLatency
        expr: histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m])) > 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency detected"
          description: "95th percentile latency is {{ $value }}s"
          
      # Low disk space
      - alert: LowDiskSpace
        expr: (node_filesystem_avail_bytes / node_filesystem_size_bytes) < 0.1
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Low disk space"
          description: "Disk space is below 10%"
          
      # High memory usage
      - alert: HighMemoryUsage
        expr: (process_resident_memory_bytes / process_virtual_memory_max_bytes) > 0.85
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage"
          description: "Memory usage is above 85%"
          
      # Database connection pool exhausted
      - alert: DatabasePoolExhausted
        expr: db_connections_in_use / db_connections_max > 0.9
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Database connection pool exhausted"
          description: "{{ $value }}% of connections in use"
          
      # Cache hit ratio low
      - alert: LowCacheHitRatio
        expr: rate(cache_hits_total[5m]) / (rate(cache_hits_total[5m]) + rate(cache_misses_total[5m])) < 0.5
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Low cache hit ratio"
          description: "Cache hit ratio is {{ $value }}"
          
      # Service down
      - alert: ServiceDown
        expr: up{job="catalog-api"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Catalog API is down"
          description: "Catalog API has been down for more than 1 minute"
```

**Estimated Time:** 6 hours

---

#### Task 7.1.3: Webhook Integration
**File:** `monitoring/alertmanager/webhooks.yml`

**Implementation:**

```yaml
# Custom webhook receiver
receivers:
  - name: 'catalogizer-webhook'
    webhook_configs:
      - url: 'http://catalog-api:8080/api/v1/internal/alerts'
        send_resolved: true
        http_config:
          bearer_token: '${WEBHOOK_TOKEN}'
```

**API Handler:**

```go
// handlers/alert_handler.go

func (h *AlertHandler) ReceiveAlert(c *gin.Context) {
    var alert AlertmanagerWebhook
    if err := c.ShouldBindJSON(&alert); err != nil {
        c.JSON(400, gin.H{"error": err.Error()})
        return
    }
    
    // Process alerts
    for _, a := range alert.Alerts {
        h.processAlert(a)
    }
    
    c.JSON(200, gin.H{"status": "received"})
}

func (h *AlertHandler) processAlert(alert Alert) {
    // Store alert
    h.db.SaveAlert(&AlertRecord{
        Name: alert.Labels["alertname"],
        Severity: alert.Labels["severity"],
        Status: alert.Status,
        Description: alert.Annotations["description"],
        Fingerprint: alert.Fingerprint,
        StartsAt: alert.StartsAt,
        EndsAt: alert.EndsAt,
    })
    
    // Trigger automated response if needed
    if alert.Labels["severity"] == "critical" {
        h.triggerEmergencyResponse(alert)
    }
}
```

**Estimated Time:** 6 hours

**Phase 7.1 Total: 20 hours**

---

### 7.2 OpenTelemetry Tracing

#### Task 7.2.1: Install OpenTelemetry
**File:** `catalog-api/go.mod`

**Dependencies:**

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
    "go.opentelemetry.io/otel/trace"
)
```

**Initialization:**

```go
// internal/tracing/tracing.go

func InitTracer(serviceName string) (*sdktrace.TracerProvider, error) {
    // Create Jaeger exporter
    exp, err := jaeger.New(jaeger.WithCollectorEndpoint(
        jaeger.WithEndpoint("http://jaeger:14268/api/traces"),
    ))
    if err != nil {
        return nil, err
    }
    
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exp),
        sdktrace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceName(serviceName),
            attribute.String("environment", os.Getenv("ENV")),
            attribute.String("version", Version),
        )),
    )
    
    otel.SetTracerProvider(tp)
    return tp, nil
}
```

**Estimated Time:** 8 hours

---

#### Task 7.2.2: Instrument Services
**File:** `catalog-api/services/*`

**Implementation:**

```go
// services/media_service.go

func (s *MediaService) GetMedia(ctx context.Context, id string) (*Media, error) {
    tracer := otel.Tracer("media-service")
    ctx, span := tracer.Start(ctx, "GetMedia",
        trace.WithAttributes(
            attribute.String("media.id", id),
        ),
    )
    defer span.End()
    
    // Add database span
    ctx, dbSpan := tracer.Start(ctx, "database.query")
    media, err := s.repo.GetByID(ctx, id)
    dbSpan.End()
    
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return nil, err
    }
    
    span.SetAttributes(
        attribute.String("media.title", media.Title),
        attribute.String("media.type", media.Type),
    )
    
    return media, nil
}
```

**Estimated Time:** 10 hours

---

#### Task 7.2.3: HTTP Middleware Tracing
**File:** `catalog-api/middleware/tracing.go`

**Implementation:**

```go
func TracingMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        tracer := otel.Tracer("http-server")
        
        // Extract context from request headers
        ctx := otel.GetTextMapPropagator().Extract(c.Request.Context(),
            propagation.HeaderCarrier(c.Request.Header),
        )
        
        // Start span
        ctx, span := tracer.Start(ctx, fmt.Sprintf("%s %s", c.Request.Method, c.FullPath()),
            trace.WithAttributes(
                semconv.HTTPMethodKey.String(c.Request.Method),
                semconv.HTTPURLKey.String(c.Request.URL.String()),
                semconv.HTTPClientIPKey.String(c.ClientIP()),
                semconv.HTTPUserAgentKey.String(c.Request.UserAgent()),
            ),
        )
        defer span.End()
        
        // Add trace context to request
        c.Request = c.Request.WithContext(ctx)
        
        // Continue processing
        c.Next()
        
        // Record response info
        span.SetAttributes(
            semconv.HTTPStatusCodeKey.Int(c.Writer.Status()),
        )
        
        if c.Writer.Status() >= 500 {
            span.SetStatus(codes.Error, "internal server error")
        }
    }
}
```

**Estimated Time:** 6 hours

---

#### Task 7.2.4: Database Tracing
**File:** `catalog-api/database/tracing.go`

**Implementation:**

```go
func (db *DB) QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error) {
    tracer := otel.Tracer("database")
    ctx, span := tracer.Start(ctx, "sql.query",
        trace.WithAttributes(
            attribute.String("db.system", db.dialect.String()),
            attribute.String("db.statement", query),
            attribute.Int("db.parameter_count", len(args)),
        ),
    )
    defer span.End()
    
    start := time.Now()
    rows, err := db.DB.QueryContext(ctx, query, args...)
    duration := time.Since(start)
    
    span.SetAttributes(
        attribute.Int64("db.duration_ms", duration.Milliseconds()),
    )
    
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }
    
    return rows, err
}
```

**Estimated Time:** 6 hours

**Phase 7.2 Total: 30 hours**

---

### 7.3 Log Aggregation

#### Task 7.3.1: Install Loki
**File:** `docker-compose.monitoring.yml`

**Implementation:**

```yaml
services:
  loki:
    image: grafana/loki:latest
    ports:
      - "3100:3100"
    volumes:
      - ./monitoring/loki:/etc/loki
      - loki-data:/loki
    command: -config.file=/etc/loki/loki.yml
    
  promtail:
    image: grafana/promtail:latest
    volumes:
      - ./monitoring/promtail:/etc/promtail
      - /var/log:/var/log:ro
      - ./catalog-api/logs:/app/logs:ro
    command: -config.file=/etc/promtail/promtail.yml
    depends_on:
      - loki
```

**Configuration:**

```yaml
# monitoring/loki/loki.yml
auth_enabled: false

server:
  http_listen_port: 3100

ingester:
  lifecycler:
    ring:
      kvstore:
        store: inmemory
      replication_factor: 1
      
schema_config:
  configs:
    - from: 2020-10-24
      store: boltdb
      object_store: filesystem
      schema: v11
      index:
        prefix: index_
        period: 168h

storage_config:
  boltdb:
    directory: /loki/index
  filesystem:
    directory: /loki/chunks
```

**Promtail Configuration:**

```yaml
# monitoring/promtail/promtail.yml
server:
  http_listen_port: 9080

positions:
  filename: /tmp/positions.yml

clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: catalog-api
    static_configs:
      - targets:
          - localhost
        labels:
          job: catalog-api
          __path__: /app/logs/*.log
          
  - job_name: system
    static_configs:
      - targets:
          - localhost
        labels:
          job: system
          __path__: /var/log/syslog
```

**Estimated Time:** 8 hours

---

#### Task 7.3.2: Structured Logging Enhancement
**File:** `catalog-api/internal/logger/logger.go`

**Implementation:**

```go
func NewLogger(service string) *zap.Logger {
    config := zap.NewProductionConfig()
    config.OutputPaths = []string{
        "stdout",
        "./logs/catalog-api.log",
    }
    
    // Loki-friendly format
    config.EncoderConfig.TimeKey = "timestamp"
    config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
    config.EncoderConfig.MessageKey = "message"
    config.EncoderConfig.LevelKey = "level"
    
    logger, _ := config.Build()
    
    return logger.With(
        zap.String("service", service),
        zap.String("version", Version),
    )
}

// Request-scoped logger
func WithRequestID(logger *zap.Logger, requestID string) *zap.Logger {
    return logger.With(zap.String("request_id", requestID))
}

// User-scoped logger
func WithUserID(logger *zap.Logger, userID string) *zap.Logger {
    return logger.With(zap.String("user_id", userID))
}
```

**Estimated Time:** 6 hours

**Phase 7.3 Total: 14 hours**

---

### 7.4 Grafana Dashboards

#### Task 7.4.1: Enhanced Dashboards
**File:** `monitoring/grafana/dashboards/`

**Dashboards to Create:**

1. **System Overview Dashboard**
   - CPU/Memory usage
   - Disk I/O
   - Network traffic
   - Up time

2. **API Performance Dashboard**
   - Request rate
   - Latency percentiles (p50, p95, p99)
   - Error rate
   - Endpoint breakdown

3. **Database Dashboard**
   - Query rate
   - Connection pool status
   - Slow queries
   - Lock waits

4. **Cache Dashboard**
   - Hit/miss ratio
   - Eviction rate
   - Size
   - Latency

5. **Business Metrics Dashboard**
   - Active users
   - Media items
   - Storage usage
   - Scan jobs

**Implementation:**

```json
{
  "dashboard": {
    "title": "Catalogizer - API Performance",
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(http_requests_total[5m])",
            "legendFormat": "{{method}} {{handler}}"
          }
        ]
      },
      {
        "title": "Latency (p95)",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))",
            "legendFormat": "{{handler}}"
          }
        ]
      },
      {
        "title": "Error Rate",
        "type": "singlestat",
        "targets": [
          {
            "expr": "rate(http_requests_total{status=~\"5..\"}[5m])"
          }
        ]
      }
    ]
  }
}
```

**Estimated Time:** 10 hours

---

#### Task 7.4.2: Dashboard Provisioning
**File:** `monitoring/grafana/provisioning/`

**Implementation:**

```yaml
# monitoring/grafana/provisioning/dashboards/dashboards.yml
apiVersion: 1

providers:
  - name: 'catalogizer'
    orgId: 1
    folder: 'Catalogizer'
    type: file
    disableDeletion: false
    updateIntervalSeconds: 10
    allowUiUpdates: true
    options:
      path: /etc/grafana/provisioning/dashboards
```

**Estimated Time:** 4 hours

**Phase 7.4 Total: 14 hours**

---

### Phase 7 Deliverables

1. ✅ AlertManager configured with alerts
2. ✅ OpenTelemetry tracing implemented
3. ✅ Loki log aggregation
4. ✅ Enhanced Grafana dashboards (50+ panels)
5. ✅ Structured logging
6. ✅ Webhook integrations
7. ✅ Automated alerting
8. ✅ Monitoring documentation

**Phase 7 Total: 88 hours** (Note: Extended from 80)

---

## PHASE 8: DOCUMENTATION & TRAINING
**Duration:** Weeks 21-22 (80 hours)  
**Goal:** Complete documentation suite with user manuals, API docs, training materials  
**Success Criteria:** All documentation 100% complete

### 8.1 User Documentation

#### Task 8.1.1: Complete User Guide
**File:** `docs/user/USER_GUIDE.md`

**Sections:**

1. **Getting Started**
   - Installation
   - First-time setup
   - Quick tour

2. **Media Management**
   - Adding storage
   - Scanning media
   - Organizing collections
   - Metadata editing

3. **User Features**
   - Favorites and watchlists
   - Analytics and reporting
   - Sharing
   - Search

4. **Advanced Topics**
   - API integration
   - Webhooks
   - Automation
   - Troubleshooting

**Estimated Time:** 16 hours

---

#### Task 8.1.2: API Documentation
**File:** `docs/api/API_REFERENCE.md`

**Format:** OpenAPI 3.0 + Markdown

**Implementation:**

```yaml
# docs/api/openapi.yml
openapi: 3.0.0
info:
  title: Catalogizer API
  version: 1.0.0
  description: Complete API reference for Catalogizer

paths:
  /api/v1/media:
    get:
      summary: List media items
      parameters:
        - name: type
          in: query
          schema:
            type: string
            enum: [movie, tv_show, music, game]
        - name: limit
          in: query
          schema:
            type: integer
            default: 20
      responses:
        '200':
          description: List of media items
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/MediaList'
```

**Estimated Time:** 12 hours

---

#### Task 8.1.3: Administrator Guide
**File:** `docs/admin/ADMINISTRATOR_GUIDE.md`

**Sections:**

1. **Installation & Deployment**
   - Docker/Podman deployment
   - Kubernetes deployment
   - Configuration management
   - Database setup

2. **Security**
   - Authentication setup
   - Authorization configuration
   - SSL/TLS certificates
   - Security scanning

3. **Monitoring**
   - Prometheus setup
   - Grafana dashboards
   - Alert configuration
   - Log aggregation

4. **Maintenance**
   - Backup procedures
   - Updates and upgrades
   - Performance tuning
   - Troubleshooting

**Estimated Time:** 14 hours

---

#### Task 8.1.4: Developer Guide
**File:** `docs/developer/DEVELOPER_GUIDE.md`

**Sections:**

1. **Development Setup**
   - Prerequisites
   - IDE configuration
   - Debugging

2. **Architecture**
   - System overview
   - Component diagrams
   - Data flow

3. **Contributing**
   - Code style
   - Testing
   - Pull request process
   - Code review

4. **Extending**
   - Plugin development
   - API clients
   - Custom providers

**Estimated Time:** 12 hours

---

#### Task 8.1.5: Troubleshooting Guide
**File:** `docs/troubleshooting/TROUBLESHOOTING.md`

**Implementation:**

```markdown
# Troubleshooting Guide

## Common Issues

### Service Won't Start

**Symptom:** Application fails to start with database connection error

**Solution:**
1. Check database is running: `podman ps | grep postgres`
2. Verify connection string in `.env`
3. Check database logs: `podman logs catalogizer-db`

### High Memory Usage

**Symptom:** Application uses excessive memory

**Solution:**
1. Check for memory leaks: Enable profiling
2. Reduce cache size: Update `CACHE_SIZE` in config
3. Limit concurrent operations: Adjust semaphore limits
```

**Estimated Time:** 8 hours

**Phase 8.1 Total: 62 hours**

---

### 8.2 Video Course Extension

#### Task 8.2.1: Advanced Video Modules
**File:** `docs/video-course/`

**Modules:**

1. **Module 6: Performance Optimization** (45 min)
   - Lazy loading strategies
   - Caching best practices
   - Database optimization
   - Profiling techniques

2. **Module 7: Security Implementation** (60 min)
   - Authentication patterns
   - Authorization strategies
   - Vulnerability scanning
   - Security monitoring

3. **Module 8: Monitoring & Observability** (45 min)
   - Prometheus metrics
   - Grafana dashboards
   - Alerting setup
   - Tracing with OpenTelemetry

4. **Module 9: Advanced Testing** (60 min)
   - Integration testing
   - Contract testing
   - Performance testing
   - Security testing

5. **Module 10: Production Deployment** (45 min)
   - Kubernetes deployment
   - Scaling strategies
   - Disaster recovery
   - Maintenance procedures

**Estimated Time:** 40 hours

---

#### Task 8.2.2: Video Course Scripts
**File:** `docs/video-course/scripts/`

**Script Template:**

```markdown
# Module 6: Performance Optimization

## Introduction (2 min)
[Scene: Presenter at desk]
"Welcome to Module 6 of the Catalogizer Advanced Course. Today we'll dive deep into performance optimization techniques that will make your application lightning fast."

## Section 1: Lazy Loading (10 min)
[Scene: Code editor]
"First, let's look at lazy loading. The key principle is simple: don't load what you don't need..."

[CODE EXAMPLE]
```go
// BEFORE: Eager loading
db.Query("SELECT * FROM media_items")

// AFTER: Lazy loading
type LazyMediaItem struct {
    metadata *MediaMetadata // Loaded on demand
}
```

## Section 2: Caching Strategies (15 min)
...
```

**Estimated Time:** 20 hours

**Phase 8.2 Total: 60 hours** (Note: Extended from original scope)

---

### 8.3 Architecture Documentation

#### Task 8.3.1: Complete Architecture Diagrams
**File:** `docs/architecture/diagrams/`

**Diagrams:**

1. **System Architecture Overview**
2. **Data Flow Diagrams**
3. **Component Interaction Diagrams**
4. **Database Schema Diagrams**
5. **Deployment Architecture**
6. **Security Architecture**
7. **Monitoring Architecture**

**Estimated Time:** 10 hours

---

#### Task 8.3.2: Architecture Decision Records (ADRs)
**File:** `docs/architecture/adrs/`

**ADRs to Complete:**

1. **ADR-001: Database Selection** - SQLite vs PostgreSQL
2. **ADR-002: API Framework** - Gin vs Echo vs Fiber
3. **ADR-003: Frontend Framework** - React vs Vue vs Angular
4. **ADR-004: Caching Strategy** - Redis vs Memcached
5. **ADR-005: Container Runtime** - Podman vs Docker
6. **ADR-006: Authentication** - JWT vs Session
7. **ADR-007: Testing Strategy** - Unit vs Integration vs E2E

**Estimated Time:** 8 hours

---

#### Task 8.3.3: Data Dictionary
**File:** `docs/database/DATA_DICTIONARY.md`

**Content:**

```markdown
# Data Dictionary

## media_items Table

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | UUID | NO | gen_random_uuid() | Primary key |
| title | VARCHAR(255) | NO | - | Media title |
| type | VARCHAR(50) | NO | - | Media type enum |
| created_at | TIMESTAMP | NO | now() | Creation timestamp |
| updated_at | TIMESTAMP | NO | now() | Last update timestamp |

## Indexes
- PRIMARY KEY (id)
- INDEX idx_media_type (type)
- INDEX idx_media_created (created_at)

## Relationships
- One-to-Many: media_items → files
- Many-to-Many: media_items ↔ collections
```

**Estimated Time:** 12 hours

**Phase 8.3 Total: 30 hours**

---

### Phase 8 Deliverables

1. ✅ Complete user guide
2. ✅ Complete API documentation
3. ✅ Administrator guide
4. ✅ Developer guide
5. ✅ Troubleshooting guide
6. ✅ 5 advanced video modules
7. ✅ Complete architecture diagrams
8. ✅ All ADRs documented
9. ✅ Data dictionary
10. ✅ All documentation 100% complete

**Phase 8 Total: 152 hours** (Note: Extended from 80 due to comprehensive requirements)

---

## PHASE 9: WEBSITE & CONTENT
**Duration:** Weeks 23-24 (80 hours)  
**Goal:** Update website with all new content  
**Success Criteria:** Website 100% complete with all documentation

### 9.1 Website Structure

#### Task 9.1.1: Documentation Site
**File:** `website/docs/`

**Structure:**

```
website/
├── docs/
│   ├── getting-started/
│   │   ├── installation.md
│   │   ├── quickstart.md
│   │   └── configuration.md
│   ├── user-guide/
│   │   ├── media-management.md
│   │   ├── collections.md
│   │   └── search.md
│   ├── admin-guide/
│   │   ├── deployment.md
│   │   ├── security.md
│   │   └── monitoring.md
│   ├── api/
│   │   ├── authentication.md
│   │   ├── endpoints.md
│   │   └── examples.md
│   └── developer/
│       ├── architecture.md
│       ├── contributing.md
│       └── extending.md
├── blog/
│   └── releases/
├── video-courses/
│   └── index.md
└── index.md
```

**Technology:** Docusaurus or MkDocs

**Estimated Time:** 20 hours

---

#### Task 9.1.2: Interactive API Explorer
**File:** `website/api/`

**Implementation:**

```javascript
// Swagger UI or Redoc integration
import SwaggerUI from 'swagger-ui-react';
import 'swagger-ui-react/swagger-ui.css';

function APIExplorer() {
  return (
    <SwaggerUI
      url="/api/openapi.yml"
      docExpansion="list"
      defaultModelsExpandDepth={-1}
    />
  );
}
```

**Estimated Time:** 10 hours

---

#### Task 9.1.3: Video Course Portal
**File:** `website/courses/`

**Features:**
- Course listing
- Video player
- Progress tracking
- Quizzes
- Certificates

**Estimated Time:** 16 hours

---

#### Task 9.1.4: Search Integration
**File:** `website/search/`

**Implementation:**

```javascript
// Algolia or Elasticsearch integration
import algoliasearch from 'algoliasearch/lite';

const searchClient = algoliasearch(
  process.env.ALGOLIA_APP_ID,
  process.env.ALGOLIA_API_KEY
);

function Search() {
  return (
    <InstantSearch searchClient={searchClient} indexName="catalogizer-docs">
      <SearchBox />
      <Hits hitComponent={Hit} />
    </InstantSearch>
  );
}
```

**Estimated Time:** 8 hours

**Phase 9.1 Total: 54 hours**

---

### 9.2 Content Creation

#### Task 9.2.1: Blog Posts
**File:** `website/blog/`

**Topics:**
1. "Introducing Catalogizer 1.0"
2. "Performance Optimization Techniques"
3. "Security Best Practices"
4. "Monitoring at Scale"
5. "Testing Strategy"

**Estimated Time:** 10 hours

---

#### Task 9.2.2: Tutorial Series
**File:** `website/tutorials/`

**Tutorials:**
1. "Getting Started with Catalogizer"
2. "Building Your First Collection"
3. "Advanced Search Techniques"
4. "API Integration Tutorial"
5. "Custom Provider Development"

**Estimated Time:** 12 hours

---

#### Task 9.2.3: FAQ Page
**File:** `website/faq.md`

**Content:** 50+ FAQs covering all aspects

**Estimated Time:** 6 hours

---

#### Task 9.2.4: Changelog
**File:** `website/changelog.md`

**Format:** Keep a Changelog standard

**Estimated Time:** 4 hours

**Phase 9.2 Total: 32 hours**

---

### Phase 9 Deliverables

1. ✅ Complete documentation website
2. ✅ Interactive API explorer
3. ✅ Video course portal
4. ✅ Search functionality
5. ✅ Blog posts
6. ✅ Tutorial series
7. ✅ FAQ page
8. ✅ Changelog

**Phase 9 Total: 86 hours** (Note: Extended from 80)

---

## PHASE 10: FINAL VALIDATION & DEPLOYMENT
**Duration:** Weeks 25-26 (80 hours)  
**Goal:** Complete system validation and deployment  
**Success Criteria:** All tests passing, production ready

### 10.1 Comprehensive Testing

#### Task 10.1.1: Full Test Suite Execution
**Scope:** All test types

**Execution:**

```bash
#!/bin/bash
# scripts/run-all-tests-comprehensive.sh

set -e

echo "=== COMPREHENSIVE TEST SUITE ==="

# 1. Unit Tests
echo "1. Running unit tests..."
GOMAXPROCS=3 go test ./... -race -coverprofile=coverage.out -p 2 -parallel 2

# 2. Integration Tests
echo "2. Running integration tests..."
go test ./... -tags=integration -v

# 3. Contract Tests
echo "3. Running contract tests..."
cd contracts && go test ./...

# 4. Security Tests
echo "4. Running security tests..."
./scripts/security-scan-all.sh

# 5. Performance Tests
echo "5. Running performance tests..."
./scripts/performance-tests.sh

# 6. E2E Tests
echo "6. Running E2E tests..."
cd catalog-web && npm run test:e2e

# 7. Load Tests
echo "7. Running load tests..."
./scripts/load-tests.sh

# 8. Chaos Tests
echo "8. Running chaos tests..."
./scripts/chaos-tests.sh

echo "=== ALL TESTS PASSED ==="
```

**Estimated Time:** 24 hours

---

#### Task 10.1.2: Coverage Validation
**Goal:** 95%+ coverage across all components

**Validation:**

```bash
#!/bin/bash
# scripts/validate-coverage.sh

THRESHOLD=95.0

echo "Validating test coverage..."

# Parse coverage output
coverage=$(go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//')

echo "Current coverage: ${coverage}%"

if (( $(echo "$coverage < $THRESHOLD" | bc -l) )); then
    echo "ERROR: Coverage ${coverage}% is below threshold ${THRESHOLD}%"
    
    # Show packages below threshold
    echo ""
    echo "Packages below threshold:"
    go tool cover -func=coverage.out | awk -v threshold=$THRESHOLD '
    {
        if (NF > 2 && $3+0 < threshold) {
            print $1 ": " $3
        }
    }'
    
    exit 1
fi

echo "✓ Coverage validation passed"
```

**Estimated Time:** 8 hours

---

#### Task 10.1.3: Stress Testing
**Goal:** System remains responsive under extreme load

**Implementation:**

```bash
#!/bin/bash
# scripts/stress-tests.sh

echo "Starting stress tests..."

# Install k6 if not present
if ! command -v k6 &> /dev/null; then
    go install go.k6.io/k6@latest
fi

# Run stress test
cat > stress-test.js << 'EOF'
import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    stages: [
        { duration: '2m', target: 100 },  // Ramp up
        { duration: '5m', target: 100 },  // Steady state
        { duration: '2m', target: 200 },  // Ramp up
        { duration: '5m', target: 200 },  // Steady state
        { duration: '2m', target: 400 },  // Peak load
        { duration: '5m', target: 400 },  // Sustained peak
        { duration: '5m', target: 0 },    // Ramp down
    ],
    thresholds: {
        http_req_duration: ['p(95)<500'],  // 95% under 500ms
        http_req_failed: ['rate<0.01'],    // <1% errors
    },
};

export default function () {
    const res = http.get('http://localhost:8080/api/v1/media');
    check(res, {
        'status is 200': (r) => r.status === 200,
        'response time < 500ms': (r) => r.timings.duration < 500,
    });
    sleep(1);
}
EOF

k6 run stress-test.js

echo "Stress tests complete"
```

**Estimated Time:** 12 hours

---

#### Task 10.1.4: Chaos Engineering
**Goal:** System resilience validation

**Implementation:**

```bash
#!/bin/bash
# scripts/chaos-tests.sh

echo "Starting chaos engineering tests..."

# Install Chaos Mesh or similar
if ! command -v chaosctl &> /dev/null; then
    # Install Chaos Mesh CLI
    curl -sSL https://mirrors.chaos-mesh.org/latest/install.sh | bash
fi

# Network latency
chaosctl create network-latency.yaml
echo "Injected network latency"
sleep 30

# Pod kill
chaosctl create pod-kill.yaml
echo "Killed random pods"
sleep 30

# Disk stress
chaosctl create disk-stress.yaml
echo "Applied disk stress"
sleep 30

# Verify system still functioning
curl -f http://localhost:8080/health || exit 1

echo "Chaos tests complete"
```

**Estimated Time:** 10 hours

**Phase 10.1 Total: 54 hours**

---

### 10.2 Deployment

#### Task 10.2.1: Production Deployment
**File:** `deployment/production/`

**Implementation:**

```yaml
# deployment/production/k8s/catalogizer-deployment.yml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: catalogizer-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: catalogizer-api
  template:
    metadata:
      labels:
        app: catalogizer-api
    spec:
      containers:
        - name: api
          image: catalogizer/api:latest
          ports:
            - containerPort: 8080
          resources:
            requests:
              memory: "512Mi"
              cpu: "500m"
            limits:
              memory: "2Gi"
              cpu: "2000m"
          livenessProbe:
            httpGet:
              path: /health/live
              port: 8080
            initialDelaySeconds: 30
            periodSeconds: 10
          readinessProbe:
            httpGet:
              path: /health/ready
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 5
```

**Estimated Time:** 16 hours

---

#### Task 10.2.2: Blue-Green Deployment
**File:** `scripts/deploy-blue-green.sh`

**Implementation:**

```bash
#!/bin/bash
# scripts/deploy-blue-green.sh

VERSION=$1
if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version>"
    exit 1
fi

echo "Deploying version $VERSION using blue-green strategy..."

# Determine current color
CURRENT_COLOR=$(kubectl get service catalogizer -o jsonpath='{.spec.selector.color}')
NEW_COLOR=$([ "$CURRENT_COLOR" = "blue" ] && echo "green" || echo "blue")

echo "Current color: $CURRENT_COLOR"
echo "New color: $NEW_COLOR"

# Deploy new version
kubectl set image deployment/catalogizer-$NEW_COLOR \
    api=catalogizer/api:$VERSION

# Wait for rollout
kubectl rollout status deployment/catalogizer-$NEW_COLOR

# Run smoke tests
./scripts/smoke-tests.sh

# Switch traffic
kubectl patch service catalogizer -p \
    '{"spec":{"selector":{"color":"'$NEW_COLOR'"}}}'

# Scale down old version (keep for rollback)
kubectl scale deployment catalogizer-$CURRENT_COLOR --replicas=1

echo "Deployment complete"
```

**Estimated Time:** 8 hours

**Phase 10.2 Total: 24 hours**

---

### 10.3 Documentation

#### Task 10.3.1: Final Summary Report
**File:** `docs/FINAL_PROJECT_REPORT.md`

**Content:**
- Project overview
- Completed work summary
- Metrics and achievements
- Lessons learned
- Future roadmap

**Estimated Time:** 8 hours

---

#### Task 10.3.2: Maintenance Procedures
**File:** `docs/operations/MAINTENANCE_PROCEDURES.md`

**Content:**
- Regular maintenance tasks
- Backup procedures
- Update procedures
- Rollback procedures
- Emergency procedures

**Estimated Time:** 6 hours

---

#### Task 10.3.3: Runbook
**File:** `docs/operations/RUNBOOK.md`

**Content:**
- Common incidents
- Resolution procedures
- Escalation paths
- Contact information

**Estimated Time:** 4 hours

**Phase 10.3 Total: 18 hours**

---

### Phase 10 Deliverables

1. ✅ All test suites passing
2. ✅ 95%+ coverage validated
3. ✅ Stress tests passed
4. ✅ Chaos tests passed
5. ✅ Production deployment complete
6. ✅ Blue-green deployment configured
7. ✅ Final summary report
8. ✅ Maintenance procedures
9. ✅ Runbook
10. ✅ Project 100% complete

**Phase 10 Total: 96 hours** (Note: Extended from 80)

---

## SUMMARY

### Total Estimated Effort

| Phase | Original Estimate | Revised Estimate | Change |
|-------|------------------|------------------|---------|
| Phase 1: Foundation & Safety | 80h | 88h | +10% |
| Phase 2: Test Infrastructure | 80h | 80h | 0% |
| Phase 3: Coverage Expansion | 160h | 160h | 0% |
| Phase 4: Integration & Dead Code | 160h | 212h | +33% |
| Phase 5: Security & Scanning | 80h | 118h | +48% |
| Phase 6: Performance & Optimization | 160h | 166h | +4% |
| Phase 7: Monitoring & Observability | 80h | 88h | +10% |
| Phase 8: Documentation & Training | 80h | 152h | +90% |
| Phase 9: Website & Content | 80h | 86h | +8% |
| Phase 10: Final Validation & Deployment | 80h | 96h | +20% |
| **TOTAL** | **1,040h** | **1,246h** | **+20%** |

### Timeline

- **Start Date:** March 23, 2026
- **End Date:** September 18, 2026 (26 weeks)
- **Team Size:** 3-5 engineers
- **Total Cost:** ~$187,000 (at $150/hour)

### Success Metrics

| Metric | Before | After | Improvement |
|--------|--------|-------|-------------|
| Test Coverage | 35% | 95%+ | +171% |
| Dead Code | 40% | 0% | -100% |
| Documentation | 85% | 100% | +18% |
| Security Score | 70% | 95% | +36% |
| Performance | 65% | 90% | +38% |
| Overall Health | 65% | 95% | +46% |

### Key Deliverables

1. ✅ Zero safety issues (races, leaks, deadlocks)
2. ✅ 95%+ test coverage
3. ✅ All security tools integrated
4. ✅ Complete observability stack
5. ✅ Comprehensive documentation
6. ✅ Complete video courses
7. ✅ Updated website
8. ✅ Production deployment ready
9. ✅ All tests passing
10. ✅ Zero unfinished functionality

---

**Document Version:** 1.0  
**Last Updated:** March 22, 2026  
**Status:** READY FOR EXECUTION
