# Module 17: Load Testing with k6 -- Video Script

**Duration**: 45 minutes
**Prerequisites**: Module 8 (HTTP/3 and Performance), familiarity with HTTP load testing concepts

---

## Video 17.1: k6 Setup and Configuration (10 min)

### Opening

Welcome to Module 17. Performance under load is a requirement, not a nice-to-have. This module shows how to use k6 to load test Catalogizer, interpret the results, and connect them to Grafana dashboards for continuous performance monitoring.

### What is k6?

k6 is a modern load testing tool written in Go that uses JavaScript for test scripts. It produces metrics compatible with Prometheus and Grafana, making it a natural fit for Catalogizer's monitoring stack.

### Installing k6

```bash
# Linux (Debian/Ubuntu)
sudo gpg -k
sudo gpg --no-default-keyring --keyring /usr/share/keyrings/k6-archive-keyring.gpg \
  --keyserver hkp://keyserver.ubuntu.com:80 --recv-keys C5AD17C747E3415A3642D57D77C6C491D6AC1D69
echo "deb [signed-by=/usr/share/keyrings/k6-archive-keyring.gpg] https://dl.k6.io/deb stable main" | \
  sudo tee /etc/apt/sources.list.d/k6.list
sudo apt-get update && sudo apt-get install k6

# macOS
brew install k6

# Verify installation
k6 version
```

### Project Structure

Catalogizer's load tests are located in `tests/k6/`:

```
tests/k6/
  load-test.js          # Standard load test (gradual ramp)
  stress-test.js        # Stress test (find breaking point)
  soak-test.js          # Soak test (sustained load over time)
  spike-test.js         # Spike test (sudden load increase)
  helpers/
    auth.js             # Authentication helper (login, get token)
    config.js           # Base URL, thresholds, common options
```

### Configuration

The `helpers/config.js` file defines shared configuration:

```javascript
export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';

export const THRESHOLDS = {
  http_req_duration: ['p(95)<500', 'p(99)<1000'],  // 95th < 500ms, 99th < 1s
  http_req_failed: ['rate<0.01'],                    // < 1% failure rate
  http_reqs: ['rate>100'],                           // > 100 requests/sec
};
```

### Authentication Helper

Since all API endpoints require JWT authentication, the auth helper logs in and caches the token:

```javascript
// helpers/auth.js
import http from 'k6/http';

export function login(baseUrl, username, password) {
  const res = http.post(`${baseUrl}/api/v1/auth/login`, JSON.stringify({
    username: username,
    password: password,
  }), { headers: { 'Content-Type': 'application/json' } });

  if (res.status !== 200) {
    throw new Error(`Login failed: ${res.status} ${res.body}`);
  }

  return JSON.parse(res.body).token;
}
```

---

## Video 17.2: Load Test Scenarios (15 min)

### Standard Load Test

The load test simulates normal production traffic with a gradual ramp-up:

```javascript
// tests/k6/load-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { login } from './helpers/auth.js';
import { BASE_URL, THRESHOLDS } from './helpers/config.js';

export const options = {
  stages: [
    { duration: '2m', target: 10 },   // Ramp up to 10 users
    { duration: '5m', target: 10 },   // Stay at 10 users
    { duration: '2m', target: 20 },   // Ramp up to 20 users
    { duration: '5m', target: 20 },   // Stay at 20 users
    { duration: '2m', target: 0 },    // Ramp down
  ],
  thresholds: THRESHOLDS,
};

let token;

export function setup() {
  token = login(BASE_URL, 'admin', '<your-admin-password>');
  return { token };
}

export default function(data) {
  const headers = {
    'Authorization': `Bearer ${data.token}`,
    'Content-Type': 'application/json',
  };

  // Health check (no auth)
  const healthRes = http.get(`${BASE_URL}/health`);
  check(healthRes, { 'health status 200': (r) => r.status === 200 });

  // Storage roots listing
  const rootsRes = http.get(`${BASE_URL}/api/v1/storage-roots`, { headers });
  check(rootsRes, { 'roots status 200': (r) => r.status === 200 });

  // Media search
  const searchRes = http.get(`${BASE_URL}/api/v1/media/search?q=test&limit=20`, { headers });
  check(searchRes, { 'search status 200': (r) => r.status === 200 });

  // Entity listing
  const entitiesRes = http.get(`${BASE_URL}/api/v1/entities?limit=20`, { headers });
  check(entitiesRes, { 'entities status 200': (r) => r.status === 200 });

  // Statistics
  const statsRes = http.get(`${BASE_URL}/api/v1/stats/overall`, { headers });
  check(statsRes, { 'stats status 200': (r) => r.status === 200 });

  sleep(1); // Simulate think time between requests
}
```

Run the load test:

```bash
k6 run tests/k6/load-test.js
```

### Stress Test

The stress test pushes beyond normal capacity to find the breaking point:

```javascript
// tests/k6/stress-test.js
export const options = {
  stages: [
    { duration: '2m', target: 10 },   // Below normal load
    { duration: '5m', target: 50 },   // Normal load
    { duration: '2m', target: 100 },  // Around breaking point
    { duration: '5m', target: 100 },  // Stay at breaking point
    { duration: '2m', target: 150 },  // Beyond breaking point
    { duration: '5m', target: 150 },  // Stay beyond
    { duration: '5m', target: 0 },    // Recovery
  ],
  thresholds: {
    http_req_duration: ['p(95)<2000'],  // Relaxed: 95th < 2s
    http_req_failed: ['rate<0.10'],      // Allow up to 10% failures
  },
};
```

The stress test helps identify:
- The maximum concurrent user count before degradation
- How the `ConcurrencyLimiter(100)` middleware behaves at capacity
- Whether the system recovers after load decreases
- Database connection pool saturation points

### Soak Test

The soak test runs sustained moderate load for an extended period to detect memory leaks and resource exhaustion:

```javascript
// tests/k6/soak-test.js
export const options = {
  stages: [
    { duration: '5m', target: 20 },    // Ramp up
    { duration: '4h', target: 20 },    // Sustained load for 4 hours
    { duration: '5m', target: 0 },     // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],
    http_req_failed: ['rate<0.01'],
  },
};
```

During the soak test, monitor:
- `go_goroutines` Prometheus metric (should be stable, not climbing)
- `go_memstats_alloc_bytes` (should show a sawtooth pattern from GC, not steady growth)
- Response latency (should remain consistent, not degrading over time)

### Running with Resource Limits

Remember Catalogizer's host resource constraint (30-40% maximum). Limit k6 accordingly:

```bash
# Limit k6 to 2 CPU cores
taskset -c 0,1 k6 run tests/k6/load-test.js

# Or use cgroups
systemd-run --scope -p CPUQuota=200% k6 run tests/k6/load-test.js
```

---

## Video 17.3: Interpreting Results (10 min)

### k6 Output

After a test run, k6 prints a summary:

```
          /\      |------|  /------/
     /\  /  \     |  |---|/  /-----/
    /  \/    \    |  ||   /  /-----/
   /          \   |  | | / /-/
  / __________ \  |__|/ |/___/

  execution: local
     script: tests/k6/load-test.js
     output: -

  scenarios: (100.00%) 1 scenario, 20 max VUs, 16m30s max duration

     data_received..................: 15 MB  16 kB/s
     data_sent......................: 1.2 MB 1.3 kB/s
     http_req_blocked...............: avg=1.2ms   p(95)=3.5ms
     http_req_connecting............: avg=0.8ms   p(95)=2.1ms
     http_req_duration..............: avg=45ms    p(95)=120ms   p(99)=250ms
     http_req_failed................: 0.05%  5 out of 10000
     http_req_receiving.............: avg=0.3ms   p(95)=1.0ms
     http_req_sending...............: avg=0.1ms   p(95)=0.3ms
     http_req_waiting...............: avg=44ms    p(95)=118ms
     http_reqs......................: 10000  17.2/s
     iteration_duration.............: avg=1.05s   p(95)=1.12s
     vus............................: 20     min=0  max=20
     vus_max........................: 20     min=20 max=20
```

### Key Metrics to Watch

| Metric | Good | Warning | Critical |
|--------|------|---------|----------|
| `http_req_duration` p(95) | < 200ms | 200-500ms | > 500ms |
| `http_req_duration` p(99) | < 500ms | 500ms-1s | > 1s |
| `http_req_failed` rate | < 0.1% | 0.1-1% | > 1% |
| `http_reqs` rate | > 100/s | 50-100/s | < 50/s |

### Sending Results to Grafana

k6 can output metrics to Prometheus, which Grafana then visualizes:

```bash
# Output to Prometheus remote write
k6 run --out experimental-prometheus-rw tests/k6/load-test.js

# Or output to InfluxDB
k6 run --out influxdb=http://localhost:8086/k6 tests/k6/load-test.js

# Or output to JSON for post-processing
k6 run --out json=results.json tests/k6/load-test.js
```

### Analyzing Trends

Compare results across runs to detect performance regressions:

```bash
# Save results with timestamps
k6 run --out json=results/load-$(date +%Y%m%d_%H%M%S).json tests/k6/load-test.js
```

Look for:
- **Latency creep**: p(95) increasing across runs with the same load profile
- **Error rate changes**: New failure modes appearing
- **Throughput degradation**: Fewer requests/second for the same virtual user count

---

## Video 17.4: Grafana Dashboard for Load Testing (10 min)

### Pre-Built Dashboard

Catalogizer includes a Grafana dashboard configuration in `monitoring/` that visualizes both runtime metrics and load test results.

### Key Panels

1. **Request Rate**: `rate(http_requests_total[5m])` -- shows requests per second
2. **Latency Distribution**: `histogram_quantile(0.95, rate(http_request_duration_seconds_bucket[5m]))` -- 95th percentile latency
3. **Error Rate**: `rate(http_requests_total{status=~"5.."}[5m])` -- server error rate
4. **Goroutine Count**: `go_goroutines` -- should be stable during sustained load
5. **Memory Usage**: `go_memstats_alloc_bytes` -- heap allocation over time
6. **Connection Pool**: Active vs idle database connections
7. **Cache Hit Rate**: Cache hits vs misses over time

### Setting Up the Dashboard

```bash
# Start the monitoring stack
podman-compose -f docker-compose.dev.yml up prometheus grafana

# Import the dashboard
# Navigate to http://localhost:3001 (Grafana)
# Import from monitoring/grafana/dashboards/catalogizer.json
```

### Correlating Load Test with Application Metrics

Run the load test while watching Grafana:

1. Start the monitoring stack
2. Start the Catalogizer API
3. Open Grafana at `http://localhost:3001`
4. Run `k6 run tests/k6/stress-test.js`
5. Watch the dashboards in real-time to see how metrics respond to load

The most valuable insight is the correlation between concurrent users and latency. You should see latency remain flat until a threshold, then increase sharply -- that threshold is your system's capacity.

---

## Exercises

1. Write a k6 test that exercises the subtitle search endpoint and measures latency
2. Run the stress test and identify the breaking point for your deployment
3. Run the soak test for 1 hour and check `go_goroutines` for leak indicators
4. Create a Grafana panel that shows k6 test results alongside application metrics

---

## Key Files Referenced

- `tests/k6/load-test.js` -- Standard load test scenario
- `tests/k6/stress-test.js` -- Stress test to find breaking points
- `tests/k6/soak-test.js` -- Sustained load for leak detection
- `monitoring/` -- Prometheus and Grafana configurations
- `catalog-api/middleware/concurrency.go` -- ConcurrencyLimiter(100)
- `catalog-api/internal/metrics/` -- Prometheus metrics registration
- `catalog-api/database/connection.go` -- Connection pool configuration
