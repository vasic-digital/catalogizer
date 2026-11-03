import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

const errorRate = new Rate('errors');
const apiLatency = new Trend('api_latency', true);
const requestsServed = new Counter('requests_served');

// Breakpoint test: constant-rate ramp to find the RPS ceiling where p95
// latency or error rate collapses. Unlike stress_test.js (which ramps
// concurrent users), this ramps target throughput with a constant-arrival
// executor so we measure the true request-rate limit of the server.
export const options = {
  scenarios: {
    breakpoint: {
      executor: 'ramping-arrival-rate',
      startRate: 10,
      timeUnit: '1s',
      preAllocatedVUs: 50,
      maxVUs: 500,
      stages: [
        { duration: '30s', target: 50 },    // Warm up to 50 rps
        { duration: '1m',  target: 100 },   // 100 rps baseline
        { duration: '1m',  target: 200 },   // 200 rps
        { duration: '1m',  target: 400 },   // 400 rps
        { duration: '1m',  target: 600 },   // 600 rps — typical ceiling
        { duration: '1m',  target: 1000 },  // 1000 rps — search for break
        { duration: '30s', target: 0 },     // Ramp down
      ],
    },
  },
  thresholds: {
    // Abort the test if p95 exceeds 1500 ms or error rate exceeds 10% —
    // that's the breakpoint. Thresholds that abort must use abortOnFail.
    http_req_duration: [
      { threshold: 'p(95)<1500', abortOnFail: true, delayAbortEval: '30s' },
    ],
    errors: [
      { threshold: 'rate<0.10', abortOnFail: true, delayAbortEval: '30s' },
    ],
    api_latency: ['p(95)<1500'],
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

export function setup() {
  const loginRes = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
    username: __ENV.API_USER || 'admin',
    password: __ENV.API_PASS || '',
  }), { headers: { 'Content-Type': 'application/json' } });

  const token = loginRes.status === 200 ? (JSON.parse(loginRes.body).token || '') : '';
  return { token };
}

export default function (data) {
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${data.token}`,
  };

  // Lightweight read-path mix to maximize observable throughput.
  const endpoints = [
    '/health',
    '/api/v1/media/stats',
    '/api/v1/storage-roots',
  ];
  const endpoint = endpoints[Math.floor(Math.random() * endpoints.length)];
  const res = http.get(`${BASE_URL}${endpoint}`, { headers, timeout: '5s' });

  apiLatency.add(res.timings.duration);
  errorRate.add(res.status >= 500);
  requestsServed.add(1);

  check(res, {
    'status not 5xx': (r) => r.status < 500,
  });
}

export function teardown(data) {
  console.log('Breakpoint test complete.');
}
