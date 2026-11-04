Monitoring deploy live evidence (AZ2-R5 / AZ2-R6 / review G3), captured 2026-11-04 on a scratch rootless podman project
"catmon-test" (torn down afterwards; see 08_teardown.txt). No token or password values appear in these files.

What was run: a REAL catalog-api binary (built in golang:1.25 from main d4a3dbd6 sources) as a plain container on the
project network; the real prom/prometheus, grafana/grafana and otel/opentelemetry-collector-contrib:0.164.0 images, started
from services copied out of the repo docker-compose.yml (prometheus, grafana, otel-collector; depends_on and deploy stripped
because the full stack - postgres, redis, minio, api image build - is not started). Prometheus was started with
PROMETHEUS_EXTRA_FLAGS=--web.enable-remote-write-receiver (opt-in, only for the otel profile).

Limitation (honest): the podman network DNS (aardvark-dns) on this host refused queries on the new network, so the names
api / catalogizer-api / prometheus were injected with extra_hosts in the scratch compose file. Therefore:
 - VERIFIED (files 01-07): token file written in-container, scrape with Bearer token (up=1), otel collector validate + start +
   scrape + remote write (up{job="catalogizer-api"}=1), Grafana datasource + dashboards provisioned, ports bound to 127.0.0.1,
   remote-write receiver closed by default (404) and opened only by the flag (400), regression test + 22 mutants killed.
 - NOT VERIFIED (PROVISIONAL): resolution of the `api` network alias in deployment/docker-compose.yml through compose DNS
   (only the static test checks the alias exists), and the full deployment/docker-compose.yml stack.
