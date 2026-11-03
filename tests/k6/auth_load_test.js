import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const loginLatency = new Trend('login_latency', true);
const refreshLatency = new Trend('refresh_latency', true);
const authCheckLatency = new Trend('auth_check_latency', true);

// Authentication load test: 100 concurrent users exercising auth endpoints
export const options = {
  stages: [
    { duration: '15s', target: 20 },   // Ramp up to 20 users
    { duration: '30s', target: 50 },   // Ramp up to 50 users
    { duration: '1m', target: 100 },   // Ramp up to 100 users
    { duration: '1m', target: 100 },   // Hold at 100 users
    { duration: '15s', target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1500'],  // 95th < 500ms, 99th < 1.5s
    errors: ['rate<0.05'],                             // Error rate < 5%
    login_latency: ['p(95)<500'],                      // Login p95 < 500ms
    refresh_latency: ['p(95)<500'],                    // Token refresh p95 < 500ms
    auth_check_latency: ['p(95)<300'],                 // Auth check p95 < 300ms
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

export function setup() {
  // Perform initial login to verify credentials work
  const loginRes = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
    username: __ENV.API_USER || 'admin',
    password: __ENV.API_PASS || '',
  }), { headers: { 'Content-Type': 'application/json' } });

  check(loginRes, { 'setup login succeeded': (r) => r.status === 200 });

  if (loginRes.status !== 200) {
    console.error('Setup login failed, tests will use per-iteration login');
    return { setupToken: '' };
  }

  const body = JSON.parse(loginRes.body);
  return { setupToken: body.session_token || body.token || body.access_token || '' };
}

export default function (data) {
  const iteration = __ITER;
  const scenario = iteration % 5;

  switch (scenario) {
    case 0:
    case 1:
      // 40% — Login with valid credentials
      testLogin();
      break;
    case 2:
      // 20% — Login with invalid credentials (should fail gracefully)
      testInvalidLogin();
      break;
    case 3:
      // 20% — Token refresh / re-authentication
      testTokenRefresh();
      break;
    case 4:
      // 20% — Authenticated endpoint access
      testAuthenticatedAccess(data.setupToken);
      break;
  }

  sleep(0.3 + Math.random() * 0.5);
}

function testLogin() {
  const start = new Date();
  const res = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
    username: __ENV.API_USER || 'admin',
    password: __ENV.API_PASS || '',
  }), { headers: { 'Content-Type': 'application/json' } });

  loginLatency.add(new Date() - start);

  const success = check(res, {
    'login status 200': (r) => r.status === 200,
    'login returns token': (r) => {
      if (r.status !== 200) return false;
      const body = JSON.parse(r.body);
      return body.token !== undefined || body.access_token !== undefined;
    },
    'login response is JSON': (r) => {
      try { JSON.parse(r.body); return true; } catch (e) { return false; }
    },
  });

  errorRate.add(!success);
}

function testInvalidLogin() {
  const res = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
    username: 'nonexistent_user_' + __VU,
    password: 'wrongpassword',
  }), { headers: { 'Content-Type': 'application/json' } });

  const success = check(res, {
    'invalid login returns 401': (r) => r.status === 401 || r.status === 429,
    'invalid login response is JSON': (r) => {
      try { JSON.parse(r.body); return true; } catch (e) { return false; }
    },
  });

  // Invalid logins returning 401/429 are expected, not errors
  errorRate.add(res.status >= 500);
}

function testTokenRefresh() {
  // Login first to get a token
  const loginRes = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
    username: __ENV.API_USER || 'admin',
    password: __ENV.API_PASS || '',
  }), { headers: { 'Content-Type': 'application/json' } });

  if (loginRes.status !== 200) {
    errorRate.add(loginRes.status >= 500);
    return;
  }

  const body = JSON.parse(loginRes.body);
  const token = body.session_token || body.token || body.access_token || '';

  // Use the token to access a protected endpoint (simulates token refresh pattern)
  const start = new Date();
  const res = http.get(`${BASE_URL}/api/v1/auth/me`, {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${token}`,
    },
  });

  refreshLatency.add(new Date() - start);

  const success = check(res, {
    'auth me status 2xx': (r) => r.status >= 200 && r.status < 300,
    'auth me returns user data': (r) => {
      if (r.status >= 300) return false;
      try {
        const data = JSON.parse(r.body);
        return data.username !== undefined || data.user !== undefined || data.id !== undefined;
      } catch (e) {
        return false;
      }
    },
  });

  errorRate.add(!success);
}

function testAuthenticatedAccess(token) {
  if (!token) {
    // Re-authenticate if no setup token
    const loginRes = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
      username: __ENV.API_USER || 'admin',
      password: __ENV.API_PASS || '',
    }), { headers: { 'Content-Type': 'application/json' } });

    if (loginRes.status === 200) {
      const body = JSON.parse(loginRes.body);
      token = body.session_token || body.token || body.access_token || '';
    }
  }

  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${token}`,
  };

  // Access multiple authenticated endpoints in sequence
  const endpoints = [
    '/health',
    '/api/v1/storage-roots',
    '/api/v1/auth/me',
  ];

  for (const endpoint of endpoints) {
    const start = new Date();
    const res = http.get(`${BASE_URL}${endpoint}`, { headers });
    authCheckLatency.add(new Date() - start);

    check(res, {
      [`${endpoint} status 2xx`]: (r) => r.status >= 200 && r.status < 300,
    });

    errorRate.add(res.status >= 500);
  }
}
