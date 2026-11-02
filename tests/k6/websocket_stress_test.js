import http from 'k6/http';
import { check, sleep } from 'k6';
import ws from 'k6/ws';
import { Rate, Trend, Counter } from 'k6/metrics';

// Custom metrics
const wsConnectErrors = new Rate('ws_connect_errors');
const wsMessageLatency = new Trend('ws_message_latency', true);
const wsMessagesReceived = new Counter('ws_messages_received');
const wsReconnections = new Counter('ws_reconnections');

// WebSocket stress test: 100 concurrent connections
export const options = {
  stages: [
    { duration: '20s', target: 20 },   // Ramp to 20 connections
    { duration: '30s', target: 50 },   // Ramp to 50 connections
    { duration: '1m', target: 100 },   // Ramp to 100 connections
    { duration: '2m', target: 100 },   // Hold at 100 connections
    { duration: '30s', target: 0 },    // Ramp down
  ],
  thresholds: {
    ws_connect_errors: ['rate<0.1'],          // Connect error rate < 10%
    ws_message_latency: ['p(95)<200'],        // Message latency p95 < 200ms
    http_req_duration: ['p(95)<500'],         // Auth latency p95 < 500ms
  },
};

const BASE_URL = __ENV.API_URL || 'http://localhost:8080';
const WS_URL = __ENV.WS_URL || 'ws://localhost:8080/ws';

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
  const url = `${WS_URL}?token=${data.token}`;

  const res = ws.connect(url, {}, function (socket) {
    socket.on('open', function () {
      // Subscribe to channels
      socket.send(JSON.stringify({ type: 'subscribe', channel: 'media_updates' }));
      socket.send(JSON.stringify({ type: 'subscribe', channel: 'system_updates' }));

      // Send periodic pings to keep connection alive
      socket.setInterval(function () {
        socket.send(JSON.stringify({ type: 'ping', timestamp: Date.now() }));
      }, 5000);
    });

    socket.on('message', function (msg) {
      wsMessagesReceived.add(1);
      try {
        const data = JSON.parse(msg);
        if (data.type === 'pong' && data.timestamp) {
          wsMessageLatency.add(Date.now() - data.timestamp);
        }
      } catch (e) {
        // Non-JSON message, still count it
      }
    });

    socket.on('error', function (e) {
      wsConnectErrors.add(true);
    });

    socket.on('close', function () {
      // Connection closed
    });

    // Keep connection open for 30 seconds then close
    socket.setTimeout(function () {
      socket.close();
    }, 30000);
  });

  check(res, {
    'WebSocket connection established': (r) => r && r.status === 101,
  });

  if (!res || res.status !== 101) {
    wsConnectErrors.add(true);
  }

  sleep(1);
}
