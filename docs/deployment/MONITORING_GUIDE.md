# Monitoring Setup Guide

This guide covers setting up and configuring monitoring for Catalogizer using Prometheus and Grafana, understanding available metrics, configuring alerts, and creating custom dashboards.

## Table of Contents

1. [Starting Monitoring Services](#starting-monitoring-services)
2. [Prometheus Configuration](#prometheus-configuration)
3. [Available Metrics](#available-metrics)
4. [Grafana Setup](#grafana-setup)
5. [Dashboard Overview](#dashboard-overview)
6. [Alert Configuration](#alert-configuration)
7. [Custom Dashboards](#custom-dashboards)
8. [Troubleshooting Monitoring](#troubleshooting-monitoring)

---

## Starting Monitoring Services

Monitoring is provided as an optional Docker Compose profile. Prometheus and Grafana run alongside the core services.

### Start Monitoring Stack

```bash
cd /opt/catalogizer

# Start core services plus monitoring
docker compose --profile monitoring up -d

# Or if core services are already running, add monitoring
docker compose --profile monitoring up -d prometheus grafana
```

### Verify Monitoring Services

```bash
# Check all services are running
docker compose --profile monitoring ps

# Test Prometheus
curl -sf http://localhost:9090/-/healthy && echo "Prometheus: healthy"

# Test Grafana
curl -sf http://localhost:3001/api/health && echo "Grafana: healthy"

# Test that Prometheus is scraping the API
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health: .health}'
```

### Access Points

| Service | URL | Default Credentials |
|---------|-----|---------------------|
| Prometheus | http://localhost:9090 | No auth |
| Grafana | http://localhost:3001 | admin / admin (set via `GRAFANA_PASSWORD`) |
| API Metrics | http://localhost:8080/metrics | Requires API access |

### Environment Variables for Monitoring

Configure these in your `.env` file:

```bash
# Prometheus (published on loopback by default; PROMETHEUS_BIND=0.0.0.0 exposes it, see "Exposure" below)
PROMETHEUS_PORT=9090
PROMETHEUS_BIND=127.0.0.1

# Grafana
GRAFANA_PORT=3001
GRAFANA_BIND=127.0.0.1
GRAFANA_USER=admin
GRAFANA_PASSWORD=<YOUR_SECURE_PASSWORD>
```

### Resource Allocation

The monitoring stack has the following resource limits configured in `docker-compose.yml`:

| Service | CPU Limit | Memory Limit | CPU Reservation | Memory Reservation |
|---------|-----------|--------------|-----------------|-------------------|
| Prometheus | 1 CPU | 1 GB | 0.5 CPU | 256 MB |
| Grafana | 1 CPU | 512 MB | 0.5 CPU | 256 MB |

---

## Prometheus Configuration

### Current Configuration

The Prometheus configuration is located at `monitoring/prometheus.yml`:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'catalog-api'
    metrics_path: /metrics
    authorization:
      type: Bearer
      credentials_file: /etc/prometheus/secrets/metrics_scrape_token
    static_configs:
      - targets: ['api:8080']
        labels:
          service: 'catalog-api'
          environment: 'production'

  - job_name: 'prometheus'
    static_configs:
      - targets: ['localhost:9090']
```

### /metrics Authentication

`/metrics` (and `/debug/pprof/*`) are admin-only. Scrapers use a dedicated credential. Generate it
once and put the SAME value in `.env` (the API and Prometheus both read `METRICS_SCRAPE_TOKEN`):

```bash
grep -q '^METRICS_SCRAPE_TOKEN=.' .env 2>/dev/null || \
  { sed -i '/^METRICS_SCRAPE_TOKEN=/d' .env 2>/dev/null; printf 'METRICS_SCRAPE_TOKEN=%s\n' "$(openssl rand -hex 32)" >> .env; }   # .env is gitignored, keep it mode 0600
```

Prometheus does NOT mount a token file from the host. Its compose entrypoint writes the value of
`METRICS_SCRAPE_TOKEN` to `/tmp/metrics_scrape_token` inside the container (mode 0600) before starting,
and `monitoring/prometheus.yml` reads that path (`authorization.credentials_file`). If the variable is
unset or shorter than 32 characters the Prometheus container exits at once with
`METRICS_SCRAPE_TOKEN must be set (>= 32 chars ...)` (previously a missing host file broke the start, or,
under podman-compose, was silently created as a directory so every scrape failed with "is a directory").
Note: `deployment/docker-compose.yml` reads `deployment/.env` (compose resolves `.env` next to the
project directory), not the repository-root `.env`; put the variable in the file your compose command uses.

Security note: the token lives in the container environment (visible to anyone who can run
`podman inspect` / `docker inspect` on the host) and may briefly appear on the host's `podman create`
command line while the container is created. Treat the host account as trusted; the credential grants
read access to `GET /metrics` only. Rotate by changing `.env` and recreating the `prometheus` container (and the API).

The API accepts it as `Authorization: Bearer <token>` on `GET /metrics` only (min 32 chars;
constant-time compare; any other route answers 401 with it). Without the variable, only an
admin JWT can read `/metrics`. The OpenTelemetry collector (compose profile `otel`, root
`docker-compose.yml`) is started with `METRICS_SCRAPE_TOKEN` and reads `${env:METRICS_SCRAPE_TOKEN}`;
the collector image is version-pinned, its config exports metrics to Prometheus' remote-write receiver
(opt-in: start Prometheus with `PROMETHEUS_EXTRA_FLAGS=--web.enable-remote-write-receiver`, only together with the
`otel` profile), and traces/logs are only logged (no backend in this compose file; the former jaeger/loki exporters
are invalid in current releases).
VERIFIED, with committed raw evidence in `docs/qa/monitoring-deploy-20261104/` (files 01-05): `otelcol-contrib validate`
accepts the config (the pre-fix config is rejected), the collector starts (0 restarts, 0 error lines), scrapes `/metrics`
with the token and Prometheus holds the series (`up{job="catalogizer-api"} = 1`, 79 series); Grafana provisioned its
Prometheus datasource and both dashboards. Not verified: see the PROVISIONAL note below.

#### Exposure (read before changing `*_BIND`)

- Prometheus and Grafana are published on `127.0.0.1` by default (`PROMETHEUS_BIND`, `GRAFANA_BIND`); a remote
  scraper or browser needs an explicit `0.0.0.0` (or a reverse proxy with authentication in front).
- Prometheus runs with `--web.enable-lifecycle`: `POST /-/reload` and `POST /-/quit` are UNAUTHENTICATED. Anyone who
  can reach the port can stop Prometheus. Keep it on loopback or behind an authenticating proxy. (Lifecycle is used
  by the reload command below; drop the flag in the compose file if you do not need hot reload.)
- `--web.enable-remote-write-receiver` opens an UNAUTHENTICATED write endpoint (`/api/v1/write`) that can inject
  arbitrary series. It is therefore off by default (verified: 404 without the flag, 400 = handler reached with it)
  and must only be enabled with Prometheus bound to loopback or an internal-only network.

Regression test (real `prom/prometheus` image under rootless podman, with a paired mutation run):
`bash scripts/monitoring/test_prometheus_token_start.sh` and `... --mutations`.

### Starting the monitoring stack (QA host / any host)

```bash
# root docker-compose.yml: monitoring is the `monitoring` profile
export METRICS_SCRAPE_TOKEN=<same value as in .env> GRAFANA_PASSWORD=<set>
PROMETHEUS_PORT=19090 GRAFANA_PORT=13001 \
  podman-compose -p <project> --profile monitoring up -d prometheus grafana
# Prometheus scrapes api:8080 on the compose network; start `api` from the same compose project
# (or give your API container the network alias `api`). Check:
curl -s localhost:19090/api/v1/targets      # catalog-api health "up"
# teardown by project name: podman-compose -p <project> --profile monitoring down -v
```

`deployment/docker-compose.yml` mounts `../monitoring/...` (paths are relative to that file) and its
`catalogizer-server` service carries the network alias `api`, which `monitoring/prometheus.yml` targets.
PROVISIONAL (not live-verified): resolution of the `api` alias through the compose network DNS and the full
`deployment/docker-compose.yml` stack (the test host's podman network DNS refused queries on the scratch project
network, so names were injected with `extra_hosts`; see `docs/qa/monitoring-deploy-20261104/00_README.txt`). The
static test checks that the alias and the mount sources exist. Grafana datasource/dashboard provisioning IS
verified (file 02).

Ports (override per host; default / suggested QA-host value, pick free ports, never reuse a running stack's):

| Service | Env var | Default | Suggested on a shared QA host |
|---------|---------|---------|-------------------------------|
| Prometheus | `PROMETHEUS_PORT` (bind: `PROMETHEUS_BIND`, default 127.0.0.1) | 9090 | 19090 |
| Grafana | `GRAFANA_PORT` (bind: `GRAFANA_BIND`, default 127.0.0.1) | 3001 | 13001 |
| OTel collector (profile `otel`) | none published | - | - |

### Adding Additional Scrape Targets

To monitor additional services, edit `monitoring/prometheus.yml` and add new jobs:

```yaml
scrape_configs:
  # Existing catalog-api job
  - job_name: 'catalog-api'
    metrics_path: /metrics
    static_configs:
      - targets: ['api:8080']
        labels:
          service: 'catalog-api'
          environment: 'production'

  # Node Exporter (system metrics) - add if running node-exporter
  - job_name: 'node-exporter'
    static_configs:
      - targets: ['node-exporter:9100']

  # PostgreSQL Exporter - add if running postgres-exporter
  - job_name: 'postgres'
    static_configs:
      - targets: ['postgres-exporter:9187']

  # Redis Exporter - add if running redis-exporter
  - job_name: 'redis'
    static_configs:
      - targets: ['redis-exporter:9121']

  # Nginx Exporter - add if running nginx-exporter
  - job_name: 'nginx'
    static_configs:
      - targets: ['nginx-exporter:9113']
```

After editing, reload Prometheus configuration:

```bash
# Hot reload (--web.enable-lifecycle is enabled: unauthenticated, loopback-only by default, see Exposure)
curl -X POST http://localhost:9090/-/reload

# Or restart the container
docker compose --profile monitoring restart prometheus
```

### Data Retention

Prometheus is configured with a 200-hour (approximately 8 days) retention period via the `--storage.tsdb.retention.time=200h` flag. To change this, edit the Prometheus command in `docker-compose.yml`.

---

## Available Metrics

The Catalogizer API exposes metrics at the `/metrics` endpoint in Prometheus format.

### HTTP Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `http_requests_total` | Counter | Total HTTP requests by method, path, and status code |
| `http_request_duration_seconds` | Histogram | Request duration in seconds (with buckets) |
| `http_active_connections` | Gauge | Number of currently active HTTP connections |

### WebSocket Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `websocket_connections_active` | Gauge | Number of active WebSocket connections |

### SMB / Storage Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `smb_connection_healthy` | Gauge | SMB connection health status (1 = healthy, 0 = down) |
| `smb_circuit_breaker_state` | Gauge | Circuit breaker state per SMB host |

### Go Runtime Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `go_goroutines` | Gauge | Number of active goroutines |
| `go_memstats_alloc_bytes` | Gauge | Bytes of allocated heap memory |
| `go_memstats_sys_bytes` | Gauge | Total bytes of memory obtained from the OS |
| `go_memstats_heap_inuse_bytes` | Gauge | Bytes in in-use heap spans |
| `go_gc_duration_seconds` | Summary | GC invocation duration |
| `go_threads` | Gauge | Number of OS threads |

### Process Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `process_cpu_seconds_total` | Counter | Total CPU time in seconds |
| `process_resident_memory_bytes` | Gauge | Resident memory size in bytes |
| `process_open_fds` | Gauge | Number of open file descriptors |

### Querying Metrics

Test metrics directly from the API:

```bash
# View all metrics
curl -s http://localhost:8080/metrics

# Check specific metric via Prometheus
curl -s 'http://localhost:9090/api/v1/query?query=http_requests_total' | jq .

# Check request rate over 5 minutes
curl -s 'http://localhost:9090/api/v1/query?query=rate(http_requests_total[5m])' | jq .

# Check 95th percentile latency
curl -s 'http://localhost:9090/api/v1/query?query=histogram_quantile(0.95,rate(http_request_duration_seconds_bucket[5m]))' | jq .
```

---

## Grafana Setup

### Initial Configuration

Grafana is pre-configured with:

1. **Prometheus datasource** -- auto-provisioned via `monitoring/grafana/provisioning/datasources/prometheus.yml`
2. **Dashboard provisioning** -- auto-provisioned via `monitoring/grafana/provisioning/dashboards/dashboard.yml`
3. **Catalogizer Overview dashboard** -- pre-built at `monitoring/grafana/dashboards/catalogizer-overview.json`

### First Login

1. Open http://localhost:3001 in your browser
2. Log in with the credentials from your `.env` file (default: `admin` / value of `GRAFANA_PASSWORD`)
3. You will be prompted to change the password on first login

### Verify Datasource

```bash
# Check that Prometheus datasource is configured
curl -s -u admin:$GRAFANA_PASSWORD http://localhost:3001/api/datasources | jq '.[].name'
# Expected: "Prometheus"
```

---

## Dashboard Overview

### Catalogizer Overview Dashboard

The pre-built dashboard (`monitoring/grafana/dashboards/catalogizer-overview.json`) includes the following panels:

#### Row 1: Request Metrics
- **HTTP Request Rate** -- Time series showing requests per second, broken down by HTTP method and path. Uses `sum(rate(http_requests_total[5m])) by (method, path)`.
- **HTTP Request Duration** -- Time series showing p50, p95, and p99 latency percentiles. Alerts you to performance degradation.

#### Row 2: Connection Status
- **Active Connections** -- Single stat showing current active HTTP connections. Thresholds: green (< 100), yellow (100-500), red (> 500).
- **WebSocket Connections** -- Single stat showing active WebSocket connections. Thresholds: green (< 50), yellow (50-200), red (> 200).
- **Error Rate** -- Time series showing 4xx and 5xx error rates over time.

#### Row 3: System Health
- **Goroutines** -- Time series tracking Go goroutine count. A steadily increasing count may indicate a goroutine leak.
- **Memory Usage** -- Time series showing allocated, system, and heap-in-use memory.
- **SMB Health Status** -- Stat panels showing SMB connection health and circuit breaker state per host.

### Accessing the Dashboard

1. Navigate to http://localhost:3001
2. Click **Dashboards** in the left sidebar
3. Select **Catalogizer Overview**

---

## Alert Configuration

### Setting Up Alerting in Grafana

#### Step 1: Configure Notification Channel

Navigate to **Alerting > Contact points** in Grafana and add a notification channel:

**Email notification**:
1. Click **Add contact point**
2. Name: `ops-email`
3. Type: Email
4. Addresses: `ops@yourcompany.com`

**Slack notification**:
1. Click **Add contact point**
2. Name: `ops-slack`
3. Type: Slack
4. Webhook URL: `https://hooks.slack.com/services/...`

#### Step 2: Create Alert Rules

Navigate to **Alerting > Alert rules** and create rules:

**High Error Rate Alert**:
```
Rule name: High API Error Rate
Query: sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) > 0.05
Duration: 5m
Severity: critical
Message: API 5xx error rate exceeds 5% for the last 5 minutes
```

**High Latency Alert**:
```
Rule name: High API Latency
Query: histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le)) > 2
Duration: 5m
Severity: warning
Message: API p95 latency exceeds 2 seconds
```

**Service Down Alert**:
```
Rule name: API Health Check Failed
Query: up{job="catalog-api"} == 0
Duration: 1m
Severity: critical
Message: Catalogizer API is not responding to Prometheus scrapes
```

**High Memory Usage Alert**:
```
Rule name: High Memory Usage
Query: go_memstats_alloc_bytes{job="catalog-api"} > 1.5e9
Duration: 10m
Severity: warning
Message: API memory usage exceeds 1.5GB for 10 minutes
```

**SMB Connection Down Alert**:
```
Rule name: SMB Connection Down
Query: smb_connection_healthy == 0
Duration: 2m
Severity: critical
Message: SMB connection is unhealthy
```

### Prometheus-Native Alerting (Alternative)

Create a file `monitoring/alert_rules.yml`:

```yaml
groups:
  - name: catalogizer-alerts
    rules:
      - alert: CatalogizerDown
        expr: up{job="catalog-api"} == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Catalogizer API is down"
          description: "The catalog-api target has been unreachable for 1 minute."

      - alert: HighErrorRate
        expr: sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m])) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High 5xx error rate"
          description: "Error rate is {{ $value | humanizePercentage }} over the last 5 minutes."

      - alert: HighLatency
        expr: histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (le)) > 2
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High API latency"
          description: "p95 latency is {{ $value }}s over the last 5 minutes."

      - alert: HighMemoryUsage
        expr: go_memstats_alloc_bytes{job="catalog-api"} > 1.5e9
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "High memory usage"
          description: "API memory usage is {{ $value | humanize1024 }}B."

      - alert: GoroutineLeak
        expr: go_goroutines{job="catalog-api"} > 10000
        for: 15m
        labels:
          severity: warning
        annotations:
          summary: "Possible goroutine leak"
          description: "Goroutine count is {{ $value }}."

      - alert: SMBConnectionUnhealthy
        expr: smb_connection_healthy == 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "SMB connection unhealthy"
          description: "SMB connection to {{ $labels.host }} is down."
```

Add to `monitoring/prometheus.yml`:

```yaml
rule_files:
  - "/etc/prometheus/alert_rules.yml"
```

Mount the file in `docker-compose.yml` by adding to the prometheus volumes:

```yaml
- ./monitoring/alert_rules.yml:/etc/prometheus/alert_rules.yml:ro
```

---

## Custom Dashboards

### Creating a Database Performance Dashboard

In Grafana, create a new dashboard with these panels (requires PostgreSQL exporter):

**Panel: Active Database Connections**
```promql
pg_stat_activity_count{state="active"}
```

**Panel: Database Size**
```promql
pg_database_size_bytes{datname="catalogizer"}
```

### Creating an API Performance Dashboard

**Panel: Requests Per Second by Endpoint**
```promql
sum(rate(http_requests_total[5m])) by (path)
```

**Panel: Top 10 Slowest Endpoints**
```promql
topk(10, avg(rate(http_request_duration_seconds_sum[5m])) by (path) / avg(rate(http_request_duration_seconds_count[5m])) by (path))
```

**Panel: Error Rate by Status Code**
```promql
sum(rate(http_requests_total{status=~"[45].."}[5m])) by (status)
```

**Panel: Concurrent Connections Over Time**
```promql
http_active_connections
```

### Exporting and Importing Dashboards

```bash
# Export a dashboard
curl -s -u admin:$GRAFANA_PASSWORD \
  http://localhost:3001/api/dashboards/uid/catalogizer-overview | jq '.dashboard' > dashboard_export.json

# Import a dashboard
curl -s -u admin:$GRAFANA_PASSWORD \
  -H "Content-Type: application/json" \
  -d @dashboard_export.json \
  http://localhost:3001/api/dashboards/db
```

### Adding Dashboards to Auto-Provisioning

Place JSON dashboard files in `monitoring/grafana/dashboards/`. They will be automatically loaded by Grafana on startup via the provisioning configuration at `monitoring/grafana/provisioning/dashboards/dashboard.yml`.

---

## Troubleshooting Monitoring

### Prometheus Cannot Scrape API

```bash
# Check if the API metrics endpoint is accessible from within Docker network
docker compose exec prometheus wget -qO- http://api:8080/metrics | head -20

# Check Prometheus targets page
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[] | {job: .labels.job, health: .health, lastError: .lastError}'

# Verify the network connectivity
docker compose exec prometheus nslookup api
```

### Grafana Shows "No Data"

```bash
# Verify Prometheus datasource from Grafana
curl -s -u admin:$GRAFANA_PASSWORD http://localhost:3001/api/datasources/proxy/1/api/v1/query?query=up | jq .

# Check that the datasource URL is correct
curl -s -u admin:$GRAFANA_PASSWORD http://localhost:3001/api/datasources | jq '.[].url'
# Expected: "http://prometheus:9090"

# Test a simple query directly
curl -s 'http://localhost:9090/api/v1/query?query=up' | jq '.data.result'
```

### High Prometheus Disk Usage

```bash
# Check Prometheus data directory size
docker compose exec prometheus du -sh /prometheus

# Reduce retention time (edit docker-compose.yml prometheus command)
# Change: '--storage.tsdb.retention.time=200h' to a lower value

# Force garbage collection
curl -X POST http://localhost:9090/api/v1/admin/tsdb/clean_tombstones
```

### Grafana Provisioning Errors

```bash
# Check Grafana logs for provisioning errors
docker compose logs grafana | grep -i "provisioning\|error"

# Validate dashboard JSON
python3 -m json.tool monitoring/grafana/dashboards/catalogizer-overview.json > /dev/null && echo "Valid JSON" || echo "Invalid JSON"
```
