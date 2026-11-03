import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const errorRate = new Rate('errors');

// Stress test: ramp beyond normal capacity to find breaking point
export const options = {
  stages: [
    { duration: '30s', target: 50 },   // Normal load
    { duration: '1m', target: 100 },   // Above normal
    { duration: '1m', target: 200 },   // Stress level
    { duration: '30s', target: 300 },  // Breaking point
    { duration: '30s', target: 0 },    // Recovery
  ],
  thresholds: {
    http_req_duration: ['p(99)<2000'],  // 99th < 2s even under stress
    errors: ['rate<0.15'],               // Allow up to 15% errors under stress
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
  ];

  const endpoint = endpoints[Math.floor(Math.random() * endpoints.length)];
  const res = http.get(`${BASE_URL}${endpoint}`, { headers, timeout: '10s' });

  check(res, { 'status not 5xx': (r) => r.status < 500 });
  errorRate.add(res.status >= 500);

  sleep(0.1 + Math.random() * 0.4);
}
