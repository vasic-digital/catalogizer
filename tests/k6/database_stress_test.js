import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const dbErrorRate = new Rate('db_errors');
const queryLatency = new Trend('query_latency', true);
const writeLatency = new Trend('write_latency', true);
const concurrentOps = new Counter('concurrent_operations');

// Database stress test: concurrent read/write operations
export const options = {
  scenarios: {
    readers: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '20s', target: 30 },
        { duration: '2m', target: 50 },
        { duration: '1m', target: 50 },
        { duration: '20s', target: 0 },
      ],
      exec: 'readOperations',
    },
    writers: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '20s', target: 10 },
        { duration: '2m', target: 20 },
        { duration: '1m', target: 20 },
        { duration: '20s', target: 0 },
      ],
      exec: 'writeOperations',
    },
  },
  thresholds: {
    db_errors: ['rate<0.05'],                 // Error rate < 5%
    query_latency: ['p(95)<300', 'p(99)<800'], // Read latency
    write_latency: ['p(95)<500', 'p(99)<1000'], // Write latency
    http_req_duration: ['p(95)<500'],
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

function authenticate() {
  const loginRes = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
    username: __ENV.API_USER || 'admin',
    password: __ENV.API_PASS || '',
  }), { headers: { 'Content-Type': 'application/json' } });

  if (loginRes.status === 200) {
    const body = JSON.parse(loginRes.body);
    return body.session_token || body.token || body.access_token || '';
  }
  return '';
}

export function setup() {
  const token = authenticate();
  return { token };
}

// Read-heavy operations: browse, search, stats
export function readOperations(data) {
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${data.token}`,
  };

  group('Entity browsing under load', function () {
    const start = Date.now();
    const res = http.get(`${BASE_URL}/api/v1/entities?page=1&per_page=20`, { headers });
    queryLatency.add(Date.now() - start);
    concurrentOps.add(1);

    const success = check(res, {
      'entity list status 200': (r) => r.status === 200,
      'response is JSON': (r) => {
        try { JSON.parse(r.body); return true; } catch { return false; }
      },
    });
    if (!success) dbErrorRate.add(true);
    else dbErrorRate.add(false);
  });

  group('Media search under load', function () {
    const start = Date.now();
    const res = http.get(`${BASE_URL}/api/v1/media/search?q=test&page=1`, { headers });
    queryLatency.add(Date.now() - start);
    concurrentOps.add(1);

    const success = check(res, {
      'search status 200': (r) => r.status === 200,
    });
    if (!success) dbErrorRate.add(true);
    else dbErrorRate.add(false);
  });

  group('Stats computation under load', function () {
    const start = Date.now();
    const res = http.get(`${BASE_URL}/api/v1/stats`, { headers });
    queryLatency.add(Date.now() - start);
    concurrentOps.add(1);

    const success = check(res, {
      'stats status 200': (r) => r.status === 200,
    });
    if (!success) dbErrorRate.add(true);
    else dbErrorRate.add(false);
  });

  sleep(0.5);
}

// Write operations: collections, favorites, user metadata
export function writeOperations(data) {
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${data.token}`,
  };

  group('Collection create/delete cycle', function () {
    const start = Date.now();
    const createRes = http.post(`${BASE_URL}/api/v1/collections`, JSON.stringify({
      name: `stress-test-${Date.now()}-${__VU}`,
      description: 'Stress test collection',
    }), { headers });
    writeLatency.add(Date.now() - start);
    concurrentOps.add(1);

    const success = check(createRes, {
      'collection created': (r) => r.status === 200 || r.status === 201,
    });

    if (success && createRes.status <= 201) {
      try {
        const body = JSON.parse(createRes.body);
        const id = body.id || body.collection_id;
        if (id) {
          sleep(0.2);
          const deleteStart = Date.now();
          http.del(`${BASE_URL}/api/v1/collections/${id}`, null, { headers });
          writeLatency.add(Date.now() - deleteStart);
        }
      } catch (e) {
        // Parse failure is acceptable
      }
    }
    if (!success) dbErrorRate.add(true);
    else dbErrorRate.add(false);
  });

  sleep(1);
}
