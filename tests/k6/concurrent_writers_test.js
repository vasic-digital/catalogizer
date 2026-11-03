import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

const readErrorRate = new Rate('read_errors');
const writeErrorRate = new Rate('write_errors');
const readLatency = new Trend('read_latency', true);
const writeLatency = new Trend('write_latency', true);
const readOps = new Counter('read_operations');
const writeOps = new Counter('write_operations');

// Concurrent readers/writers test: N writers mutating playback positions
// and user metadata simultaneously with M readers fetching the same data.
// Exercises the optimistic-lock / UPSERT paths and the cache-invalidation
// semantics under contention.
export const options = {
  scenarios: {
    writers: {
      executor: 'constant-vus',
      exec: 'writer',
      vus: 15,
      duration: '3m',
    },
    readers: {
      executor: 'constant-vus',
      exec: 'reader',
      vus: 35,
      duration: '3m',
    },
  },
  thresholds: {
    http_req_duration: ['p(95)<1000'],
    read_errors: ['rate<0.02'],
    write_errors: ['rate<0.05'], // Writers may see more contention
    read_latency: ['p(95)<600'],
    write_latency: ['p(95)<1200'],
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

function login() {
  const loginRes = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
    username: __ENV.API_USER || 'admin',
    password: __ENV.API_PASS || '',
  }), { headers: { 'Content-Type': 'application/json' } });
  return loginRes.status === 200 ? (JSON.parse(loginRes.body).token || '') : '';
}

export function setup() {
  return { token: login() };
}

function headers(data) {
  return {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${data.token}`,
  };
}

// Writer: post playback-position updates to a small set of shared media IDs
// so readers and writers contend on the same rows.
export function writer(data) {
  const mediaID = 100 + Math.floor(Math.random() * 10); // 10 shared IDs
  const body = JSON.stringify({
    media_id: mediaID,
    position: Math.floor(Math.random() * 60000),
    duration: 120000,
    timestamp: Date.now(),
  });

  const res = http.put(
    `${BASE_URL}/api/v1/media/${mediaID}/position`,
    body,
    { headers: headers(data), timeout: '5s' },
  );

  writeLatency.add(res.timings.duration);
  writeErrorRate.add(res.status >= 500 || res.status === 0);
  writeOps.add(1);

  check(res, { 'write status not 5xx': (r) => r.status < 500 });
  sleep(0.1 + Math.random() * 0.2);
}

// Reader: fetch the same small set of shared media IDs continuously.
export function reader(data) {
  const mediaID = 100 + Math.floor(Math.random() * 10);
  const res = http.get(
    `${BASE_URL}/api/v1/media/${mediaID}/position`,
    { headers: headers(data), timeout: '5s' },
  );

  readLatency.add(res.timings.duration);
  readErrorRate.add(res.status >= 500 || res.status === 0);
  readOps.add(1);

  check(res, { 'read status not 5xx': (r) => r.status < 500 });
  sleep(0.05 + Math.random() * 0.1);
}

export function teardown() {
  console.log('Concurrent writers test complete.');
}
