import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const apiLatency = new Trend('api_latency', true);

// Spike test: sudden traffic surge from normal to extreme load, then recovery
export const options = {
  stages: [
    { duration: '10s', target: 5 },    // Normal load
    { duration: '5s', target: 200 },   // Spike!
    { duration: '30s', target: 200 },  // Stay at spike
    { duration: '5s', target: 5 },     // Drop back to normal
    { duration: '30s', target: 5 },    // Verify recovery
    { duration: '5s', target: 0 },     // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<2000', 'p(99)<5000'], // Relaxed: 95th < 2s, 99th < 5s
    errors: ['rate<0.20'],                             // Allow up to 20% errors during spike
    api_latency: ['p(95)<2000'],                       // API latency 95th < 2s
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

  // Mix of read operations to simulate real traffic
  const endpoints = [
    '/health',
    '/api/v1/storage-roots',
    '/api/v1/media/stats',
    '/api/v1/browse',
    '/api/v1/media/search?q=video&limit=5',
    '/api/v1/entities',
  ];

  const endpoint = endpoints[Math.floor(Math.random() * endpoints.length)];
  const res = http.get(`${BASE_URL}${endpoint}`, { headers, timeout: '10s' });

  apiLatency.add(res.timings.duration);
  errorRate.add(res.status >= 500);

  check(res, {
    'status not 5xx': (r) => r.status < 500,
    'response time < 5s': (r) => r.timings.duration < 5000,
  });

  sleep(0.1 + Math.random() * 0.4);
}
