#!/usr/bin/env bash
# QA-PROM-AUTH wiring test (11.4.289: real prometheus + grafana images). Throwaway pod catqa-promauth-test-gpod (127.0.0.1 only): Prometheus with web.config
# basic auth, Grafana with the REAL provisioned datasource file and the password as a podman secret target=/run/secrets/prom_pw uid=472 mode=0400.
# Asserts datasource health OK with the secret. PROMAUTH_MUTATE=nosecret omits the secret mount: the assertion must then FAIL.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$HERE/lib.sh"; export PROM_AUTH_USER
PN=catqa-promauth-test-gpod; SN=catqa-promauth-test-gpw; T="$(mktemp -d)"; : "${T:?}"
cleanup() { podman pod rm -f "$PN" >/dev/null 2>&1 || true; podman secret rm "$SN" >/dev/null 2>&1 || true; rm -rf -- "${T:?}"; }; trap cleanup EXIT
cleanup_pre() { podman pod rm -f "$PN" >/dev/null 2>&1 || true; podman secret rm "$SN" >/dev/null 2>&1 || true; }; cleanup_pre
PORT=39891; while ss -ltnH "sport = :$PORT" | grep -q .; do PORT=$((PORT+1)); done
PW="$(head -c 18 /dev/urandom | od -An -vtx1 | tr -d ' \n')"
mkdir -p "$T/cfg" "$T/prov"; cp -r "$HERE/grafana/provisioning/datasources" "$T/prov/"
printf '%s' "$PW" | prom_web_config "$PROM_AUTH_USER" > "$T/cfg/web.yml" || { echo "FAIL setup.web_config"; exit 1; }
printf 'global: {scrape_interval: 15s}\n' > "$T/cfg/prometheus.yml"; chmod -R a+rX "$T/cfg" "$T/prov"
printf '%s' "$PW" | podman secret create "$SN" - >/dev/null
podman pod create --name "$PN" -p "127.0.0.1:$PORT:3000" >/dev/null || { echo "FAIL setup.pod"; exit 1; }
podman create --pod "$PN" --name catqa-promauth-test-prom "$PROM_IMAGE" --config.file=/cfg/prometheus.yml --web.config.file=/cfg/web.yml --storage.tsdb.path=/prometheus >/dev/null \
  && podman cp "$T/cfg" catqa-promauth-test-prom:/cfg && podman start catqa-promauth-test-prom >/dev/null || { echo "FAIL setup.prom"; exit 1; }
SEC=(--secret "$SN,target=/run/secrets/prom_pw,uid=472,mode=0400"); [ "${PROMAUTH_MUTATE:-}" = nosecret ] && SEC=()
podman create --pod "$PN" --name catqa-promauth-test-graf -e PROM_AUTH_USER -e GF_SECURITY_ADMIN_PASSWORD__FILE=/run/secrets/gfpw --secret "$SN,target=/run/secrets/gfpw,uid=472,mode=0400" \
  -e GF_PATHS_PROVISIONING=/prov "${SEC[@]}" "$GRAFANA_IMAGE" >/dev/null && podman cp "$T/prov" catqa-promauth-test-graf:/prov && podman start catqa-promauth-test-graf >/dev/null || { echo "FAIL setup.grafana"; exit 1; }
for i in $(seq 1 60); do curl -s -m 3 -o /dev/null "http://127.0.0.1:$PORT/api/health" && break; sleep 2; done
gh() { printf 'user = "admin:%s"\n' "$PW" | curl -s -m 20 -K - "http://127.0.0.1:$PORT/api/datasources/uid/prometheus-qa/health"; }
out=""; for i in $(seq 1 20); do out="$(gh)"; case "$out" in *'"status":"OK"'*|*Unauthorized*) break;; esac; sleep 2; done
fail=0
case "$out" in *'"status":"OK"'*) echo "PASS datasource_health_ok_with_secret";; *) echo "FAIL datasource_health_ok_with_secret ${out:0:120}"; fail=1;; esac
exit "$fail"
