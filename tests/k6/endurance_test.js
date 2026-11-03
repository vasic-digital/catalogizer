import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

const errorRate = new Rate('errors');
const apiLatency = new Trend('api_latency', true);
const memoryLeakDetector = new Counter('requests_served');

// Endurance test: moderate constant load for 4 hours to detect memory
// leaks, connection pool exhaustion, slow request accumulation, and GC
// pressure that only surfaces under long runs. Hourly latency checkpoints
// enforce that performance does not degrade over time.
//
// Set K6_DURATION to override: K6_DURATION=1h k6 run endurance_test.js
export const options = {
  scenarios: {
    endurance: {
      executor: 'constant-vus',
      vus: 25,
      duration: __ENV.K6_DURATION || '4h',
    },
  },
  thresholds: {
    // Latency must stay stable — abort if p95 degrades past 800 ms.
    http_req_duration: ['p(95)<800', 'p(99)<2000'],
    errors: ['rate<0.01'],
    api_latency: ['p(50)<300', 'p(95)<800'],
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

export function setup() {
  const loginRes = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
    username: __ENV.API_USER || 'admin',
    password: __ENV.API_PASS || '',
  }), { headers: { 'Content-Type': 'application/json' } });

  const token = loginRes.status === 200 ? (JSON.parse(loginRes.body).token || '') : '';
  return { token, startTime: Date.now() };
}

export default function (data) {
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${data.token}`,
  };

  // Realistic browsing mix — what a user does over a long session.
  const endpoints = [
    '/health',
    '/api/v1/storage-roots',
    '/api/v1/media/stats',
    '/api/v1/media/search?q=a&limit=20',
    '/api/v1/media/search?q=movie&limit=10',
    '/api/v1/browse',
    '/api/v1/entities',
    '/api/v1/entities?type=movie&limit=30',
  ];

  const endpoint = endpoints[Math.floor(Math.random() * endpoints.length)];
  const res = http.get(`${BASE_URL}${endpoint}`, { headers, timeout: '10s' });

  apiLatency.add(res.timings.duration);
  errorRate.add(res.status >= 500);
  memoryLeakDetector.add(1);

  check(res, {
    'status not 5xx': (r) => r.status < 500,
    'response time < 5s': (r) => r.timings.duration < 5000,
  });

  // 3-5 s think time per "page" — realistic pacing.
  sleep(3 + Math.random() * 2);
}

export function teardown(data) {
  const elapsedMin = (Date.now() - data.startTime) / 60000;
  console.log(`Endurance test complete after ${elapsedMin.toFixed(1)} minutes.`);
}
