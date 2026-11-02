import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const browseOps = new Counter('browse_operations');
const searchOps = new Counter('search_operations');
const detailOps = new Counter('detail_operations');
const adminOps = new Counter('admin_operations');
const opLatency = new Trend('operation_latency', true);

// Realistic mixed workload test:
//   40% browse, 30% search, 20% media details, 10% admin operations
//   50 concurrent users, 5 minute duration
export const options = {
  stages: [
    { duration: '30s', target: 15 },   // Ramp up to 15 users
    { duration: '30s', target: 30 },   // Ramp up to 30 users
    { duration: '30s', target: 50 },   // Ramp up to 50 users
    { duration: '3m', target: 50 },    // Hold at 50 users for 3 minutes
    { duration: '30s', target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1500'],  // 95th < 500ms
    errors: ['rate<0.05'],                             // Error rate < 5%
    operation_latency: ['p(95)<500'],                  // Operation p95 < 500ms
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

// Search terms for realistic queries
const SEARCH_TERMS = [
  'movie', 'series', 'music', 'video', 'album', 'documentary',
  'action', 'drama', 'comedy', 'thriller', 'rock', 'jazz',
  'classical', 'pop', 'electronic', 'inception', 'matrix', 'star',
];

export function setup() {
  const loginRes = http.post(`${BASE_URL}/api/v1/auth/login`, JSON.stringify({
    username: __ENV.API_USER || 'admin',
    password: __ENV.API_PASS || '',
  }), { headers: { 'Content-Type': 'application/json' } });

  if (loginRes.status === 200) {
    const body = JSON.parse(loginRes.body);
    return { token: body.session_token || body.token || body.access_token || '' };
  }

  console.warn('Setup login failed, proceeding without token');
  return { token: '' };
}

export default function (data) {
  const headers = {
    'Content-Type': 'application/json',
    'Authorization': `Bearer ${data.token}`,
  };

  // Weighted random selection: 40% browse, 30% search, 20% details, 10% admin
  const roll = Math.random() * 100;

  if (roll < 40) {
    doBrowseOperation(headers);
  } else if (roll < 70) {
    doSearchOperation(headers);
  } else if (roll < 90) {
    doMediaDetailOperation(headers);
  } else {
    doAdminOperation(headers);
  }

  // Realistic think time between operations
  sleep(0.5 + Math.random() * 1.5);
}

// --- Browse operations (40%) ---
function doBrowseOperation(headers) {
  browseOps.add(1);

  const browseEndpoints = [
    '/api/v1/browse',
    '/api/v1/media/recent?limit=10',
    '/api/v1/media/popular?limit=10',
    '/api/v1/storage-roots',
    '/api/v1/entities?page=1&limit=20',
  ];

  const endpoint = browseEndpoints[Math.floor(Math.random() * browseEndpoints.length)];

  const start = new Date();
  const res = http.get(`${BASE_URL}${endpoint}`, { headers });
  opLatency.add(new Date() - start);

  const success = check(res, {
    'browse status 2xx': (r) => r.status >= 200 && r.status < 300,
    'browse returns JSON': (r) => {
      try { JSON.parse(r.body); return true; } catch (e) { return false; }
    },
  });

  errorRate.add(res.status >= 400);

  // Simulate user scrolling / pagination
  if (Math.random() < 0.3) {
    sleep(0.2);
    const page = Math.floor(Math.random() * 3) + 2;
    const pageRes = http.get(`${BASE_URL}/api/v1/browse?page=${page}&limit=20`, { headers });
    opLatency.add(pageRes.timings.duration);
    errorRate.add(pageRes.status >= 400);
  }
}

// --- Search operations (30%) ---
function doSearchOperation(headers) {
  searchOps.add(1);

  const query = SEARCH_TERMS[Math.floor(Math.random() * SEARCH_TERMS.length)];
  const limit = [5, 10, 20][Math.floor(Math.random() * 3)];

  const start = new Date();
  const res = http.get(`${BASE_URL}/api/v1/media/search?q=${query}&limit=${limit}`, { headers });
  opLatency.add(new Date() - start);

  const success = check(res, {
    'search status 2xx': (r) => r.status >= 200 && r.status < 300,
    'search returns JSON': (r) => {
      try { JSON.parse(r.body); return true; } catch (e) { return false; }
    },
  });

  errorRate.add(res.status >= 400);

  // Simulate user refining search
  if (Math.random() < 0.2) {
    sleep(0.3);
    const refinedQuery = query + '+' + SEARCH_TERMS[Math.floor(Math.random() * SEARCH_TERMS.length)];
    const refineRes = http.get(`${BASE_URL}/api/v1/media/search?q=${refinedQuery}&limit=10`, { headers });
    opLatency.add(refineRes.timings.duration);
    errorRate.add(refineRes.status >= 400);
  }
}

// --- Media detail operations (20%) ---
function doMediaDetailOperation(headers) {
  detailOps.add(1);

  const detailEndpoints = [
    '/api/v1/media/stats',
    '/health',
    '/api/v1/entities?page=1&limit=5',
  ];

  const endpoint = detailEndpoints[Math.floor(Math.random() * detailEndpoints.length)];

  const start = new Date();
  const res = http.get(`${BASE_URL}${endpoint}`, { headers });
  opLatency.add(new Date() - start);

  const success = check(res, {
    'detail status 2xx': (r) => r.status >= 200 && r.status < 300,
    'detail returns JSON': (r) => {
      try { JSON.parse(r.body); return true; } catch (e) { return false; }
    },
  });

  errorRate.add(res.status >= 400);

  // Simulate checking related data
  if (Math.random() < 0.4) {
    sleep(0.2);
    const statsRes = http.get(`${BASE_URL}/api/v1/media/stats`, { headers });
    opLatency.add(statsRes.timings.duration);
    errorRate.add(statsRes.status >= 400);
  }
}

// --- Admin operations (10%) ---
function doAdminOperation(headers) {
  adminOps.add(1);

  const adminEndpoints = [
    '/health',
    '/api/v1/configuration',
    '/api/v1/challenges',
  ];

  const endpoint = adminEndpoints[Math.floor(Math.random() * adminEndpoints.length)];

  const start = new Date();
  const res = http.get(`${BASE_URL}${endpoint}`, { headers });
  opLatency.add(new Date() - start);

  const success = check(res, {
    'admin status 2xx': (r) => r.status >= 200 && r.status < 300,
    'admin returns JSON': (r) => {
      try { JSON.parse(r.body); return true; } catch (e) { return false; }
    },
  });

  errorRate.add(res.status >= 400);
}
