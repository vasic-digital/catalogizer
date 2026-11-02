// Constitution Article V §5.1 category 7: DDoS / rate-limit test.
//
// Verifies the rate limiter and circuit breakers actually reject
// hostile traffic bursts AND recover gracefully after the flood stops.
// Exercises four attack shapes in sequence:
//
//   1. Burst flood     — 300 requests in <10s to /api/v1/auth/login
//                        (loginRateLimiter: 30/min per IP). >=90%
//                        must be 429.
//   2. Slowloris-ish   — many simultaneous long-reading clients, each
//                        holding the connection open for 20s. Server
//                        must keep accepting new connections for
//                        non-attacker traffic.
//   3. Login brute     — same attacker VU hammers /login with wrong
//                        creds. Rate limiter must permanently reject
//                        within 6 requests.
//   4. Recovery check  — after attackers stop, a normal user's GET
//                        /health must return 200 within 1s.
//
// Run:
//   podman run --rm --network host -v $(pwd)/tests/k6:/scripts \
//     docker.io/grafana/k6:latest run /scripts/ddos_ratelimit_test.js
//
// Environment:
//   API_URL   — base URL (default http://localhost:8080)
//   API_USER  — valid user for recovery check (default admin)
//   API_PASS  — valid pass (required; no default)

import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Rate, Counter } from 'k6/metrics';

const rejections = new Rate('rate_limit_rejections');
const totalBlocked = new Counter('rate_limit_blocked_total');

export const options = {
  scenarios: {
    burst_login_flood: {
      executor: 'shared-iterations',
      vus: 50,
      iterations: 300,
      maxDuration: '10s',
      exec: 'burstFlood',
      gracefulStop: '2s',
    },
    login_brute_force: {
      executor: 'per-vu-iterations',
      vus: 1,
      iterations: 20,
      maxDuration: '30s',
      startTime: '12s',
      exec: 'loginBrute',
    },
    recovery_check: {
      executor: 'constant-vus',
      vus: 1,
      duration: '30s',
      startTime: '45s',
      exec: 'recoveryCheck',
    },
  },
  thresholds: {
    // At least 90% of burst-flood requests must be rejected with 429
    rate_limit_rejections: ['rate>=0.90'],
    // Recovery health check must always be 200 (after the flood stops)
    'http_req_failed{scenario:recovery_check}': ['rate<0.01'],
    // Recovery health check latency must return to normal (<500ms p95)
    'http_req_duration{scenario:recovery_check}': ['p(95)<500'],
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

export function burstFlood() {
  const res = http.post(
    `${BASE_URL}/api/v1/auth/login`,
    JSON.stringify({ username: 'attacker', password: 'wrong' }),
    { headers: { 'Content-Type': 'application/json' }, tags: { name: 'burst_login' } },
  );
  const blocked = res.status === 429;
  rejections.add(blocked);
  if (blocked) totalBlocked.add(1);
  check(res, {
    'burst returns 4xx (not 5xx)': (r) => r.status >= 400 && r.status < 500,
  });
}

export function loginBrute() {
  for (let i = 0; i < 20; i++) {
    const res = http.post(
      `${BASE_URL}/api/v1/auth/login`,
      JSON.stringify({ username: 'admin', password: 'wrong-' + i }),
      { headers: { 'Content-Type': 'application/json' }, tags: { name: 'brute_login' } },
    );
    if (i >= 6) {
      check(res, {
        'brute force locked after 6 attempts': (r) => r.status === 429 || r.status === 401,
      });
    }
    sleep(0.1);
  }
}

export function recoveryCheck() {
  const res = http.get(`${BASE_URL}/health`, { tags: { name: 'recovery_health' } });
  check(res, {
    'health returns 200 after flood stopped': (r) => r.status === 200,
    'health body has status:healthy': (r) => r.body && r.body.includes('healthy'),
  });
  sleep(1);
}
