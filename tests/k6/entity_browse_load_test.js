import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const errorRate = new Rate('errors');
const browseLatency = new Trend('browse_latency', true);
const searchLatency = new Trend('search_latency', true);
const detailLatency = new Trend('detail_latency', true);

// Entity browsing load test: media browsing, search, and entity retrieval
export const options = {
  stages: [
    { duration: '15s', target: 10 },   // Ramp up to 10 users
    { duration: '30s', target: 25 },   // Ramp up to 25 users
    { duration: '1m', target: 50 },    // Ramp up to 50 users
    { duration: '2m', target: 50 },    // Hold at 50 users
    { duration: '15s', target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1500'],  // 95th < 500ms, 99th < 1.5s
    errors: ['rate<0.05'],                             // Error rate < 5%
    browse_latency: ['p(95)<500'],                     // Browse p95 < 500ms
    search_latency: ['p(95)<500'],                     // Search p95 < 500ms
    detail_latency: ['p(95)<500'],                     // Detail p95 < 500ms
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

// Search terms for realistic query simulation
const SEARCH_TERMS = [
  'movie', 'series', 'music', 'video', 'album', 'documentary',
  'action', 'drama', 'comedy', 'thriller', 'horror', 'sci-fi',
  'rock', 'jazz', 'classical', 'pop', 'electronic', 'soundtrack',
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

  const iteration = __ITER;
  const scenario = iteration % 10;

  switch (scenario) {
    case 0:
    case 1:
    case 2:
      // 30% — Browse media items (paginated)
      testBrowseMedia(headers);
      break;
    case 3:
    case 4:
    case 5:
      // 30% — Search media
      testSearchMedia(headers);
      break;
    case 6:
    case 7:
      // 20% — Get recent and popular media
      testRecentAndPopular(headers);
      break;
    case 8:
      // 10% — Get entity stats
      testEntityStats(headers);
      break;
    case 9:
      // 10% — Get media details
      testMediaDetails(headers);
      break;
  }

  sleep(0.5 + Math.random() * 0.5);
}

function testBrowseMedia(headers) {
  const page = Math.floor(Math.random() * 5) + 1;
  const limit = [10, 20, 50][Math.floor(Math.random() * 3)];

  const start = new Date();
  const res = http.get(`${BASE_URL}/api/v1/browse?page=${page}&limit=${limit}`, { headers });
  browseLatency.add(new Date() - start);

  const success = check(res, {
    'browse status 2xx': (r) => r.status >= 200 && r.status < 300,
    'browse returns JSON': (r) => {
      try { JSON.parse(r.body); return true; } catch (e) { return false; }
    },
  });

  errorRate.add(res.status >= 400);
}

function testSearchMedia(headers) {
  const query = SEARCH_TERMS[Math.floor(Math.random() * SEARCH_TERMS.length)];
  const limit = [5, 10, 20][Math.floor(Math.random() * 3)];

  const start = new Date();
  const res = http.get(`${BASE_URL}/api/v1/media/search?q=${query}&limit=${limit}`, { headers });
  searchLatency.add(new Date() - start);

  const success = check(res, {
    'search status 2xx': (r) => r.status >= 200 && r.status < 300,
    'search returns JSON': (r) => {
      try { JSON.parse(r.body); return true; } catch (e) { return false; }
    },
  });

  errorRate.add(res.status >= 400);
}

function testRecentAndPopular(headers) {
  // Recent media
  const startRecent = new Date();
  const recentRes = http.get(`${BASE_URL}/api/v1/media/recent?limit=10`, { headers });
  browseLatency.add(new Date() - startRecent);

  check(recentRes, {
    'recent media status 2xx': (r) => r.status >= 200 && r.status < 300,
    'recent media returns JSON': (r) => {
      try { JSON.parse(r.body); return true; } catch (e) { return false; }
    },
  });
  errorRate.add(recentRes.status >= 400);

  sleep(0.2);

  // Popular media
  const startPopular = new Date();
  const popularRes = http.get(`${BASE_URL}/api/v1/media/popular?limit=10`, { headers });
  browseLatency.add(new Date() - startPopular);

  check(popularRes, {
    'popular media status 2xx': (r) => r.status >= 200 && r.status < 300,
    'popular media returns JSON': (r) => {
      try { JSON.parse(r.body); return true; } catch (e) { return false; }
    },
  });
  errorRate.add(popularRes.status >= 400);
}

function testEntityStats(headers) {
  const start = new Date();
  const res = http.get(`${BASE_URL}/api/v1/media/stats`, { headers });
  detailLatency.add(new Date() - start);

  const success = check(res, {
    'stats status 2xx': (r) => r.status >= 200 && r.status < 300,
    'stats returns JSON': (r) => {
      try { JSON.parse(r.body); return true; } catch (e) { return false; }
    },
  });

  errorRate.add(res.status >= 400);
}

function testMediaDetails(headers) {
  // Get entities endpoint
  const start = new Date();
  const res = http.get(`${BASE_URL}/api/v1/entities?page=1&limit=5`, { headers });
  detailLatency.add(new Date() - start);

  check(res, {
    'entities status 2xx': (r) => r.status >= 200 && r.status < 300,
    'entities returns JSON': (r) => {
      try { JSON.parse(r.body); return true; } catch (e) { return false; }
    },
  });

  errorRate.add(res.status >= 400);
}
