import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const memoryTrend = new Trend('memory_usage', true);

// Soak test: sustained moderate load for 30 minutes to detect memory leaks
export const options = {
  stages: [
    { duration: '1m', target: 20 },    // Ramp up
    { duration: '28m', target: 20 },   // Sustained load
    { duration: '1m', target: 0 },     // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],   // Latency must stay stable
    errors: ['rate<0.02'],               // Max 2% error rate over soak period
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

  // Cycle through all major API endpoints
  const iteration = __ITER;
  const endpoints = [
    '/health',
    '/api/v1/storage-roots',
    '/api/v1/media/stats',
    '/api/v1/browse',
    '/api/v1/media/search?q=movie&limit=10',
    '/api/v1/entities',
    '/api/v1/challenges',
    '/api/v1/configuration',
  ];

  const endpoint = endpoints[iteration % endpoints.length];
  const res = http.get(`${BASE_URL}${endpoint}`, { headers });

  check(res, {
    'status 2xx': (r) => r.status >= 200 && r.status < 300,
    'response time < 500ms': (r) => r.timings.duration < 500,
  });
  errorRate.add(res.status >= 400);

  // Periodically check metrics endpoint for memory usage
  if (iteration % 50 === 0) {
    const metricsRes = http.get(`${BASE_URL}/metrics`, { timeout: '5s' });
    if (metricsRes.status === 200 && metricsRes.body) {
      const match = metricsRes.body.match(/go_memstats_alloc_bytes\s+(\d+)/);
      if (match) {
        memoryTrend.add(parseInt(match[1]) / (1024 * 1024)); // MB
      }
    }
  }

  sleep(1 + Math.random());
}
