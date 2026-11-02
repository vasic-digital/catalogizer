import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const apiLatency = new Trend('api_latency', true);

// Load test configuration: ramp up to 50 concurrent users
export const options = {
  stages: [
    { duration: '30s', target: 10 },  // Ramp up to 10 users
    { duration: '1m', target: 25 },   // Ramp up to 25 users
    { duration: '2m', target: 50 },   // Ramp up to 50 users
    { duration: '2m', target: 50 },   // Hold at 50 users
    { duration: '30s', target: 0 },   // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'], // 95th < 500ms, 99th < 1s
    errors: ['rate<0.05'],                           // Error rate < 5%
    api_latency: ['p(95)<300'],                      // API latency 95th < 300ms
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

// Authenticate and get token
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

export default function (data) {
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${data.token}`,
  };

  // Test 1: Health check
  const healthRes = http.get(`${BASE_URL}/health`, { headers });
  check(healthRes, { 'health status 200': (r) => r.status === 200 });
  errorRate.add(healthRes.status !== 200);
  apiLatency.add(healthRes.timings.duration);

  sleep(0.5);

  // Test 2: List storage roots
  const rootsRes = http.get(`${BASE_URL}/api/v1/storage-roots`, { headers });
  check(rootsRes, { 'roots status 200': (r) => r.status === 200 });
  errorRate.add(rootsRes.status !== 200);
  apiLatency.add(rootsRes.timings.duration);

  sleep(0.5);

  // Test 3: Search media
  const searchRes = http.get(`${BASE_URL}/api/v1/media/search?q=test&limit=10`, { headers });
  check(searchRes, { 'search status 2xx': (r) => r.status >= 200 && r.status < 300 });
  errorRate.add(searchRes.status >= 400);
  apiLatency.add(searchRes.timings.duration);

  sleep(0.5);

  // Test 4: Get media stats
  const statsRes = http.get(`${BASE_URL}/api/v1/media/stats`, { headers });
  check(statsRes, { 'stats status 2xx': (r) => r.status >= 200 && r.status < 300 });
  errorRate.add(statsRes.status >= 400);
  apiLatency.add(statsRes.timings.duration);

  sleep(0.5);

  // Test 5: Browse files
  const browseRes = http.get(`${BASE_URL}/api/v1/browse`, { headers });
  check(browseRes, { 'browse status 2xx': (r) => r.status >= 200 && r.status < 300 });
  errorRate.add(browseRes.status >= 400);
  apiLatency.add(browseRes.timings.duration);

  sleep(1);
}
