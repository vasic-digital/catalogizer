// Phase 15 smoke load test — 30-second sanity check against a local
// catalog-api to validate the k6 pipeline works end-to-end.
// Ramps up to 10 concurrent users for 20s, holds for 5s, ramps down.
// Not a performance certification run; that's load_test.js (6 min, 50
// users) which operators run against staging.

import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const errorRate = new Rate('errors');

export const options = {
  stages: [
    { duration: '10s', target: 5 },
    { duration: '10s', target: 10 },
    { duration: '5s', target: 10 },
    { duration: '5s', target: 0 },
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],
    errors: ['rate<0.05'],
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

let sessionToken = '';

export function setup() {
  const res = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({ username: 'admin', password: __ENV.API_PASS || '' }),
    { headers: { 'Content-Type': 'application/json' } }
  );
  if (res.status !== 200) {
    throw new Error(`login failed: ${res.status} ${res.body}`);
  }
  const body = JSON.parse(res.body);
  return { token: body.session_token };
}

export default function (data) {
  const headers = { Authorization: `Bearer ${data.token}` };

  const health = http.get(`${BASE_URL}/api/v1/health`);
  check(health, { 'health 200': (r) => r.status === 200 });
  errorRate.add(health.status !== 200);

  const browse = http.get(
    `${BASE_URL}/api/v1/entities/browse/movie?limit=20`,
    { headers }
  );
  check(browse, {
    'browse 200': (r) => r.status === 200,
    'browse has items': (r) => {
      try {
        const b = JSON.parse(r.body);
        return Array.isArray(b.items) && b.items.length > 0;
      } catch (_) {
        return false;
      }
    },
  });
  errorRate.add(browse.status !== 200);

  sleep(1);
}
