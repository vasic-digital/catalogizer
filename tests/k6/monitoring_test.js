import http from 'k6/http';
import { check, sleep, group } from 'k6';
import { Rate, Trend } from 'k6/metrics';

// Custom metrics
const metricsErrorRate = new Rate('metrics_errors');
const metricsLatency = new Trend('metrics_endpoint_latency', true);
const healthLatency = new Trend('health_endpoint_latency', true);

// Monitoring test: verify metrics and health endpoints under load
export const options = {
  stages: [
    { duration: '15s', target: 20 },
    { duration: '1m', target: 50 },
    { duration: '2m', target: 100 },
    { duration: '1m', target: 100 },
    { duration: '15s', target: 0 },
  ],
  thresholds: {
    metrics_errors: ['rate<0.01'],                    // Metrics error rate < 1%
    metrics_endpoint_latency: ['p(95)<200'],          // Metrics p95 < 200ms
    health_endpoint_latency: ['p(95)<50', 'p(99)<100'], // Health p95 < 50ms
    http_req_duration: ['p(95)<500'],
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';

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
  // Get initial metrics for baseline
  const metricsRes = http.get(`${BASE_URL}/metrics`);
  return { token, hasMetrics: metricsRes.status === 200 };
}

export default function (data) {
  const headers = {
    'Authorization': `Bearer ${data.token}`,
  };

  group('Health check responsiveness', function () {
    const start = Date.now();
    const res = http.get(`${BASE_URL}/health`);
    healthLatency.add(Date.now() - start);

    const success = check(res, {
      'health returns 200': (r) => r.status === 200,
      'health responds under 10ms target': (r) => r.timings.duration < 10,
    });
    if (!success) metricsErrorRate.add(true);
    else metricsErrorRate.add(false);
  });

  group('Prometheus metrics endpoint', function () {
    const start = Date.now();
    const res = http.get(`${BASE_URL}/metrics`);
    metricsLatency.add(Date.now() - start);

    const success = check(res, {
      'metrics returns 200': (r) => r.status === 200,
      'contains go runtime metrics': (r) => r.body && r.body.includes('go_goroutines'),
      'contains http metrics': (r) => r.body && r.body.includes('http_request'),
      'contains custom catalogizer metrics': (r) => r.body && (
        r.body.includes('catalogizer_') || r.body.includes('gin_')
      ),
      'no metric cardinality explosion': (r) => {
        // Check that metrics output is under 1MB (sign of cardinality issues)
        return r.body && r.body.length < 1024 * 1024;
      },
    });
    if (!success) metricsErrorRate.add(true);
    else metricsErrorRate.add(false);
  });

  group('API endpoints under monitoring load', function () {
    // Hit various API endpoints to generate metrics
    const endpoints = [
      '/api/v1/stats',
      '/api/v1/entities?page=1&per_page=5',
      '/api/v1/media/recent?limit=5',
    ];

    for (const endpoint of endpoints) {
      const res = http.get(`${BASE_URL}${endpoint}`, { headers });
      check(res, {
        [`${endpoint} responds`]: (r) => r.status === 200 || r.status === 404,
      });
    }
  });

  sleep(0.5);
}
