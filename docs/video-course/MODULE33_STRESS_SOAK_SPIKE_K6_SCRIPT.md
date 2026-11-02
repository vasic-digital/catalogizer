# Module 33 — Stress, Soak & Spike Testing with k6

**Duration:** 20 minutes
**Prerequisites:** Module 17 (Load Testing)

## Learning objectives

1. Tell the difference between load, stress, soak, spike, breakpoint, and endurance tests.
2. Write a k6 scenario using the `constant-vus`, `ramping-vus`, and `ramping-arrival-rate` executors.
3. Define thresholds that abort the run (`abortOnFail: true`) vs soft thresholds.
4. Run k6 inside a rootless Podman container without interactive prompts.

## Segment 1 — Taxonomy of performance tests (0:00 – 4:00)

| Test | Question it answers | Script |
|---|---|---|
| Load | Does the system handle the expected daily peak? | `load_test.js` |
| Stress | At what concurrency does it degrade? | `stress_test.js` |
| Soak | Does performance drift under long-running load? | `soak_test.js` |
| Spike | Can it absorb a sudden traffic surge? | `spike_test.js` |
| Breakpoint | What RPS ceiling can the server sustain? | `breakpoint_test.js` |
| Endurance | 4-hour run to detect leaks / GC pressure. | `endurance_test.js` |
| Concurrent writers | Read/write contention on shared rows. | `concurrent_writers_test.js` |

## Segment 2 — Executors (4:00 – 9:00)

**`constant-vus`** — N virtual users running as fast as they can for a duration. Best for load/soak.

```js
scenarios: {
  load: { executor: 'constant-vus', vus: 50, duration: '10m' },
}
```

**`ramping-vus`** — N → M users over time. Best for stress.

```js
scenarios: {
  stress: {
    executor: 'ramping-vus',
    stages: [
      { duration: '1m',  target: 50 },
      { duration: '2m',  target: 200 },
      { duration: '30s', target: 0 },
    ],
  },
}
```

**`ramping-arrival-rate`** — N rps → M rps over time. Best for breakpoint (measures throughput directly, not indirectly via VUs).

```js
scenarios: {
  breakpoint: {
    executor: 'ramping-arrival-rate',
    startRate: 10,
    timeUnit: '1s',
    preAllocatedVUs: 50,
    maxVUs: 500,
    stages: [
      { duration: '1m', target: 100 },
      { duration: '1m', target: 400 },
      { duration: '1m', target: 1000 },
    ],
  },
}
```

## Segment 3 — Thresholds: hard vs soft (9:00 – 13:00)

**Soft threshold** — test logs a warning but continues:

```js
thresholds: {
  http_req_duration: ['p(95)<800'],
}
```

**Hard threshold** — test aborts immediately with `abortOnFail`:

```js
thresholds: {
  http_req_duration: [
    { threshold: 'p(95)<1500', abortOnFail: true, delayAbortEval: '30s' },
  ],
  errors: [
    { threshold: 'rate<0.10', abortOnFail: true, delayAbortEval: '30s' },
  ],
}
```

Use `delayAbortEval` to give the test time to warm up before thresholds matter.

## Segment 4 — Running in a container (13:00 – 17:00)

```bash
podman run --rm --network host \
  -v "$PWD/tests/k6:/scripts" \
  docker.io/grafana/k6:latest run /scripts/spike_test.js
```

**Why `--network host`**: k6 needs to reach `localhost:8080` where catalog-api listens. Inside a bridged network it would see a different loopback.

**Why `--rm`**: never persist k6 containers. Each run is ephemeral.

**Env vars**: override the base URL / credentials per run:
```bash
podman run --rm --network host \
  -e API_URL=http://catalog-api:8080 \
  -e API_USER=admin -e API_PASS=<ADMIN_PASSWORD> \
  -v "$PWD/tests/k6:/scripts" \
  docker.io/grafana/k6:latest run /scripts/endurance_test.js
```

## Segment 5 — Reading results (17:00 – 20:00)

k6 output:
```
✓ status not 5xx
✗ response time < 5s
 ↳ 98% — ✓ 98123 / ✗ 2001
```

Cross-check with Grafana (`Catalogizer Runtime & Latency` dashboard) to see if k6's client-side latency matches the server's histogram. Divergence often indicates network saturation or a slow proxy.

## Exercise

1. Write a k6 script targeting `/api/v1/media/search?q=…` that ramps 5→100 VUs over 5 minutes.
2. Set a hard threshold at `p(95) < 500ms` with `abortOnFail: true`.
3. Run inside the compose network and capture the result.

## Assessment

1. What's the difference between a VU and an RPS? Answer: a VU is a concurrent caller, RPS is a throughput metric — a VU with 100 ms think time produces ~10 RPS.
2. Why is `constant-arrival-rate` better than `constant-vus` for breakpoint tests? Answer: it targets throughput directly instead of indirectly via concurrency.
