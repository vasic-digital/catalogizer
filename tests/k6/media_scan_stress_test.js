import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const scanErrorRate = new Rate('scan_errors');
const scanApiLatency = new Trend('scan_api_latency', true);
const apiDuringScans = new Trend('api_during_scan_latency', true);
const concurrentScans = new Counter('concurrent_scan_attempts');

// Media scan stress test: API responsiveness during scan operations
export const options = {
  scenarios: {
    // Background API traffic (simulates users browsing while scan runs)
    api_traffic: {
      executor: 'constant-vus',
      vus: 20,
      duration: '4m',
      exec: 'apiTraffic',
    },
    // Scan initiation attempts (should be serialized by the server)
    scan_attempts: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '30s', target: 5 },
        { duration: '2m', target: 10 },
        { duration: '1m', target: 10 },
        { duration: '30s', target: 0 },
      ],
      exec: 'scanOperations',
    },
  },
  thresholds: {
    scan_errors: ['rate<0.3'],                          // Scan errors < 30% (expected: scans may be rejected)
    api_during_scan_latency: ['p(95)<500', 'p(99)<1500'], // API still responsive during scans
    http_req_duration: ['p(95)<1000'],
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

// Simulates normal API usage during scan operations
export function apiTraffic(data) {
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${data.token}`,
  };

  group('Browse during scan', function () {
    const start = Date.now();
    const res = http.get(`${BASE_URL}/api/v1/entities?page=1&per_page=10`, { headers });
    apiDuringScans.add(Date.now() - start);

    check(res, {
      'browse responds during scan': (r) => r.status === 200,
    });
  });

  group('Search during scan', function () {
    const start = Date.now();
    const res = http.get(`${BASE_URL}/api/v1/media/search?q=movie&page=1`, { headers });
    apiDuringScans.add(Date.now() - start);

    check(res, {
      'search responds during scan': (r) => r.status === 200,
    });
  });

  group('Stats during scan', function () {
    const start = Date.now();
    const res = http.get(`${BASE_URL}/api/v1/stats`, { headers });
    apiDuringScans.add(Date.now() - start);

    check(res, {
      'stats responds during scan': (r) => r.status === 200,
    });
  });

  group('Health during scan', function () {
    const start = Date.now();
    const res = http.get(`${BASE_URL}/health`);
    apiDuringScans.add(Date.now() - start);

    check(res, {
      'health responds during scan': (r) => r.status === 200,
      'health under 50ms during scan': (r) => r.timings.duration < 50,
    });
  });

  sleep(1);
}

// Attempts to initiate and monitor scan operations
export function scanOperations(data) {
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${data.token}`,
  };

  group('Scan initiation', function () {
    concurrentScans.add(1);
    const start = Date.now();

    // Get storage roots first
    const rootsRes = http.get(`${BASE_URL}/api/v1/storage-roots`, { headers });
    scanApiLatency.add(Date.now() - start);

    const rootsOk = check(rootsRes, {
      'storage roots accessible': (r) => r.status === 200,
    });

    if (rootsOk) {
      try {
        const roots = JSON.parse(rootsRes.body);
        if (roots && roots.length > 0) {
          // Attempt scan on first root
          const scanStart = Date.now();
          const scanRes = http.post(`${BASE_URL}/api/v1/scan`, JSON.stringify({
            root_id: roots[0].id || 1,
          }), { headers });
          scanApiLatency.add(Date.now() - scanStart);

          check(scanRes, {
            'scan accepted or already running': (r) =>
              r.status === 200 || r.status === 202 || r.status === 409 || r.status === 429,
          });
        }
      } catch (e) {
        scanErrorRate.add(true);
      }
    } else {
      scanErrorRate.add(true);
    }
  });

  group('Scan status check', function () {
    const start = Date.now();
    const res = http.get(`${BASE_URL}/api/v1/scan/status`, { headers });
    scanApiLatency.add(Date.now() - start);

    check(res, {
      'scan status accessible': (r) => r.status === 200 || r.status === 404,
    });
  });

  sleep(3);
}
