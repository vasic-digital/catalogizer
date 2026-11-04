#!/usr/bin/env bash
# AZ2-R5 regression test (constitution 11.4.292 / 11.4.289): the Prometheus service must not depend on a host
# token FILE that may be absent (docker: container start error; podman-compose: silently creates a DIRECTORY there
# and every scrape then fails with "is a directory"), and it must start with a real scrape-credential file written
# inside the container from METRICS_SCRAPE_TOKEN, and fail fast with a clear message when the token is unset/short.
#
# Static part : for each compose file, the prometheus service has NO bind mount whose source is a token/secrets path,
#               uses an entrypoint that writes the file, and monitoring/prometheus.yml reads that same path.
# Live part   : runs the REAL prom/prometheus image (rootless podman) with the service's own entrypoint/command:
#               (a) token unset -> exits non-zero with the message; (b) short token -> same; (c) valid token ->
#               Prometheus becomes ready and /tmp/metrics_scrape_token holds the token (length + mode 600 only
#               are reported, never the value). The image absent -> live part is an honest SKIP, not a PASS.
# Also checked: METRICS_SCRAPE_TOKEN env passthrough, every relative mount source of prometheus/grafana/otel-collector
# exists relative to its compose file, the `api` network alias that monitoring/prometheus.yml targets exists, and the
# pinned otel-collector image validates the repo collector config (`otelcol-contrib validate`).
# Usage: bash scripts/monitoring/test_prometheus_token_start.sh            (exit 0 all PASS, 1 any FAIL)
#        bash scripts/monitoring/test_prometheus_token_start.sh --mutations (each mutant must make the test FAIL)
# Env  : COMPOSE_FILES (space separated, default: docker-compose.yml deployment/docker-compose.yml),
#        PROM_CONFIG (default monitoring/prometheus.yml), PROM_IMAGE (default docker.io/prom/prometheus:latest).
set -u
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_FILES="${COMPOSE_FILES:-docker-compose.yml deployment/docker-compose.yml}"
PROM_CONFIG="${PROM_CONFIG:-monitoring/prometheus.yml}"
PROM_IMAGE="${PROM_IMAGE:-docker.io/prom/prometheus:latest}"
LIVE="${LIVE:-1}"
PASS=0; FAIL=0; SKIP=0
ok() { PASS=$((PASS+1)); echo "PASS $*"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL $*"; }

# extract <compose> <field>: prints json of the prometheus service (or exits 3)
svc_json() { python3 - "$1" <<'PY'
import json, sys, yaml
d = yaml.safe_load(open(sys.argv[1]))
s = d.get("services", {}).get("prometheus")
if s is None: sys.exit(3)
print(json.dumps(s))
PY
}

run_one() { # run_one <compose-file-path> <prom-config-path> <label>
  local cf="$1" pc="$2" label="$3" j
  j="$(svc_json "$cf")" || { bad "$label: no prometheus service in $cf"; return; }
  # static 1: no host token/secrets file bind mount
  local mounts; mounts="$(printf '%s' "$j" | python3 -c 'import json,sys;[print(v) for v in json.load(sys.stdin).get("volumes",[]) if isinstance(v,str)]')"
  if printf '%s\n' "$mounts" | grep -Eqi 'secrets|scrape_token'; then bad "$label: prometheus bind-mounts a host token/secrets path"; else ok "$label: no host token-file bind mount"; fi
  # static 2: entrypoint writes the credential file; length guard present
  local ep; ep="$(printf '%s' "$j" | python3 -c 'import json,sys;print(" ".join(json.load(sys.stdin).get("entrypoint",[])))')"
  local tokpath; tokpath="$(printf '%s' "$ep" | grep -Eo '> /[^ ;]*metrics_scrape_token' | head -1 | sed 's/^> //')"
  if [ -n "$tokpath" ]; then ok "$label: entrypoint writes $tokpath from the environment"; else bad "$label: entrypoint does not write a token file"; fi
  if printf '%s' "$ep" | grep -q '\-ge 32'; then ok "$label: entrypoint has the length guard"; else bad "$label: entrypoint has no length guard"; fi
  # static 3: prometheus.yml reads the same path
  local cred; cred="$(python3 -c 'import sys,yaml;d=yaml.safe_load(open(sys.argv[1]));print([j["authorization"]["credentials_file"] for j in d["scrape_configs"] if "authorization" in j][0])' "$pc" 2>/dev/null)"
  if [ -n "$tokpath" ] && [ "$cred" = "$tokpath" ]; then ok "$label: prometheus.yml credentials_file == $tokpath"; else bad "$label: prometheus.yml credentials_file ($cred) != entrypoint path ($tokpath)"; fi
  # static 4: environment passthrough of the token into the prometheus service
  if printf '%s' "$j" | python3 -c 'import json,sys;e=json.load(sys.stdin).get("environment",{});sys.exit(0 if ("METRICS_SCRAPE_TOKEN" in e or any(str(x).startswith("METRICS_SCRAPE_TOKEN") for x in e)) else 1)'; then ok "$label: environment passes METRICS_SCRAPE_TOKEN to prometheus"; else bad "$label: prometheus has no METRICS_SCRAPE_TOKEN environment passthrough"; fi
  # static 5: relative mount sources of the monitoring services exist (relative to the compose file)
  local missing; missing="$(python3 - "$cf" <<'PY'
import os, sys, yaml
cf = sys.argv[1]; d = yaml.safe_load(open(cf)); base = os.path.dirname(cf)
for name in ("prometheus", "grafana", "otel-collector"):
    for v in d.get("services", {}).get(name, {}).get("volumes", []):
        if isinstance(v, str) and v.startswith("."):
            src = v.split(":")[0]
            if not os.path.exists(os.path.normpath(os.path.join(base, src))): print(name + ":" + src)
PY
)"
  if [ -z "$missing" ]; then ok "$label: every relative monitoring mount source exists"; else bad "$label: mount source(s) missing relative to the compose file: $(echo $missing)"; fi
  # static 6: the scrape target host of prometheus.yml is reachable by name (service name or network alias)
  local tgt; tgt="$(python3 -c 'import sys,yaml;d=yaml.safe_load(open(sys.argv[1]));print([j["static_configs"][0]["targets"][0].split(":")[0] for j in d["scrape_configs"] if "authorization" in j][0])' "$pc" 2>/dev/null)"
  if python3 - "$cf" "$tgt" <<'PY'
import sys, yaml
d = yaml.safe_load(open(sys.argv[1])); t = sys.argv[2]; sv = d.get("services", {})
ok = t in sv
for s in sv.values():
    for n in (s.get("networks") or {}).values() if isinstance(s.get("networks"), dict) else []:
        if n and t in (n.get("aliases") or []): ok = True
sys.exit(0 if ok else 1)
PY
  then ok "$label: scrape target host '$tgt' is a service name or network alias"; else bad "$label: scrape target host '$tgt' is neither a service nor a network alias"; fi
  # static 7 + validate: otel-collector (only where the compose file defines it)
  if python3 -c 'import sys,yaml;sys.exit(0 if "otel-collector" in yaml.safe_load(open(sys.argv[1]))["services"] else 1)' "$cf"; then
    local oimg ocfg; oimg="$(python3 -c 'import sys,yaml;print(yaml.safe_load(open(sys.argv[1]))["services"]["otel-collector"]["image"])' "$cf")"
    ocfg="$(python3 - "$cf" <<'PY'
import os, sys, yaml
cf = sys.argv[1]; s = yaml.safe_load(open(cf))["services"]["otel-collector"]
print(os.path.normpath(os.path.join(os.path.dirname(cf), [v for v in s["volumes"] if "otel" in v][0].split(":")[0])))
PY
)"
    if printf '%s' "$oimg" | grep -Eq ':[0-9]+\.[0-9]+'; then ok "$label: otel-collector image is version-pinned ($oimg)"; else bad "$label: otel-collector image is not version-pinned ($oimg)"; fi
    if [ "$LIVE" = 1 ] && command -v podman >/dev/null && podman image exists "$oimg"; then
      local oout orc; oout="$(timeout 60 podman run --rm -e METRICS_SCRAPE_TOKEN=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa -v "$ocfg:/c.yml:ro" "$oimg" validate --config=/c.yml 2>&1)"; orc=$?
      if [ "$orc" -eq 0 ]; then ok "$label live: otelcol-contrib validate accepts the collector config"; else bad "$label live: otelcol-contrib validate rc=$orc: $(printf '%s' "$oout" | tail -1 | cut -c1-160)"; fi
    else SKIP=$((SKIP+1)); echo "SKIP $label otel validate: LIVE=0, podman or image $oimg absent (not a PASS)"; fi
  fi
  [ "$LIVE" = 1 ] || { SKIP=$((SKIP+1)); echo "SKIP $label live fail-fast/start: LIVE=0"; return; }
  # live part
  command -v podman >/dev/null || { SKIP=$((SKIP+1)); echo "SKIP $label live: podman absent (hardware_not_present-class, not a PASS)"; return; }
  podman image exists "$PROM_IMAGE" || { SKIP=$((SKIP+1)); echo "SKIP $label live: image $PROM_IMAGE absent (not a PASS)"; return; }
  local sh_script; sh_script="$(printf '%s' "$j" | python3 -c 'import json,sys;e=json.load(sys.stdin)["entrypoint"];print(e[2].replace("$$","$"))')"
  local cmdargs; mapfile -t cmdargs < <(printf '%s' "$j" | python3 -c 'import json,sys;[print(c) for c in json.load(sys.stdin).get("command",[])]')
  local name="catmon-tokentest-$$-${RANDOM}" out rc
  # (a) unset token
  out="$(timeout 25 podman run --rm --name "$name-a" --entrypoint /bin/sh -v "$pc:/etc/prometheus/prometheus.yml:ro" "$PROM_IMAGE" -c "$sh_script" prometheus "${cmdargs[@]}" 2>&1)"; rc=$?
  if [ "$rc" -eq 1 ] && printf '%s' "$out" | grep -q 'METRICS_SCRAPE_TOKEN must be set' && ! podman container exists "$name-a"; then ok "$label live: unset token -> container exited rc=1 with clear message"; else bad "$label live: unset token did not fail fast with rc=1 (rc=$rc; 124 = timeout, it kept running)"; fi
  podman rm -f "$name-a" >/dev/null 2>&1
  # (b) short token
  out="$(timeout 25 podman run --rm --name "$name-b" -e METRICS_SCRAPE_TOKEN=short --entrypoint /bin/sh -v "$pc:/etc/prometheus/prometheus.yml:ro" "$PROM_IMAGE" -c "$sh_script" prometheus "${cmdargs[@]}" 2>&1)"; rc=$?
  if [ "$rc" -eq 1 ] && printf '%s' "$out" | grep -q 'METRICS_SCRAPE_TOKEN must be set' && ! podman container exists "$name-b"; then ok "$label live: short token -> container exited rc=1 with clear message"; else bad "$label live: short token did not fail fast with rc=1 (rc=$rc; 124 = timeout, it kept running)"; fi
  podman rm -f "$name-b" >/dev/null 2>&1
  # (c) valid token: no host file exists anywhere, container still starts and the file is written
  local tok; tok="$(head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"
  podman run -d --name "$name-c" -e METRICS_SCRAPE_TOKEN="$tok" --entrypoint /bin/sh -v "$pc:/etc/prometheus/prometheus.yml:ro" "$PROM_IMAGE" -c "$sh_script" prometheus "${cmdargs[@]}" >/dev/null 2>&1
  local i ready=1; for i in $(seq 1 20); do podman exec "$name-c" wget -q -O /dev/null http://127.0.0.1:9090/-/ready 2>/dev/null && { ready=0; break; }; sleep 1; done
  local len mode same
  len="$(podman exec "$name-c" sh -c "wc -c < $tokpath" 2>/dev/null | tr -d ' ')"; mode="$(podman exec "$name-c" stat -c %a "$tokpath" 2>/dev/null)"
  same="$(podman exec "$name-c" sh -c "[ \"\$(cat $tokpath)\" = \"\$METRICS_SCRAPE_TOKEN\" ] && echo yes || echo no" 2>/dev/null)"
  podman rm -f "$name-c" >/dev/null 2>&1
  if [ "$ready" -eq 0 ] && [ "${len:-0}" = "${#tok}" ] && [ "$mode" = "600" ] && [ "$same" = "yes" ]; then ok "$label live: valid token -> Prometheus ready, token file len=$len mode=$mode equals env"; else bad "$label live: valid token start failed (ready=$ready len=${len:-?} mode=${mode:-?} same=${same:-?})"; fi
}

if [ "${1:-}" = "--mutations" ]; then
  T="$(mktemp -d "${TMPDIR:-/tmp}/promtok_mut.XXXXXX")" || exit 2
  trap 'rm -rf "${T:?}"' EXIT
  M=0; K=0
  # mutant <name> <root|deploy|config|otel> <old text> <new text> <live 0|1>   (old -> new, first occurrence; the
  # replacement must change the file). Each run gets a fresh mirror tree $T/{docker-compose.yml,
  # deployment/docker-compose.yml,monitoring/} so relative mount sources resolve as in the repository.
  mutant() {
    local name="$1" kind="$2" old="$3" new="$4" live="$5" target
    rm -rf "${T:?}"/*; mkdir -p "$T/deployment"
    cp "$ROOT/docker-compose.yml" "$T/docker-compose.yml"; cp "$ROOT/deployment/docker-compose.yml" "$T/deployment/docker-compose.yml"
    cp -r "$ROOT/monitoring" "$T/monitoring"
    case "$kind" in root) target="$T/docker-compose.yml" ;; deploy) target="$T/deployment/docker-compose.yml" ;;
      config) target="$T/monitoring/prometheus.yml" ;; otel) target="$T/monitoring/opentelemetry/otel-collector.yml" ;; esac
    python3 - "$target" "$old" "$new" <<'PY' || { echo "BADMUTANT $name (old text not found)"; M=$((M+1)); return; }
import sys
p, old, new = sys.argv[1:4]
s = open(p).read(); assert old in s, "old text absent"; open(p, "w").write(s.replace(old, new, 1))
PY
    M=$((M+1))
    if LIVE="$live" COMPOSE_FILES="$T/docker-compose.yml $T/deployment/docker-compose.yml" PROM_CONFIG="$T/monitoring/prometheus.yml" bash "${BASH_SOURCE[0]}" >/dev/null 2>&1; then echo "SURVIVED $name"; else K=$((K+1)); echo "KILLED   $name"; fi
  }
  GUARD='[ "$${#METRICS_SCRAPE_TOKEN}" -ge 32 ] || { echo "METRICS_SCRAPE_TOKEN must be set (>= 32 chars, same value as the API); see docs/deployment/MONITORING_GUIDE.md" >&2; exit 1; }; '
  PASSTHRU=$'    environment:\n      METRICS_SCRAPE_TOKEN: ${METRICS_SCRAPE_TOKEN:-}\n    entrypoint:'
  NOPASS=$'    environment:\n      X_UNRELATED: "1"\n    entrypoint:'
  mutant m1_root_readd_host_token_file_mount root '      - prometheus_data:/prometheus' $'      - ./monitoring/secrets/metrics_scrape_token:/etc/prometheus/secrets/metrics_scrape_token:ro\n      - prometheus_data:/prometheus' 0
  mutant m1_deploy_readd_host_token_file_mount deploy '      - prometheus_data:/prometheus' $'      - ../monitoring/secrets/metrics_scrape_token:/etc/prometheus/secrets/metrics_scrape_token:ro\n      - prometheus_data:/prometheus' 0
  mutant m2_root_drop_length_guard root "$GUARD" '' 1
  mutant m2_deploy_drop_length_guard deploy "$GUARD" '' 1
  mutant m2b_deploy_guard_exit_to_true deploy 'exit 1; }; umask' 'true; }; umask' 1
  mutant m3_credentials_path_drift config '/tmp/metrics_scrape_token' '/etc/prometheus/secrets/metrics_scrape_token' 0
  mutant m4_deploy_revert_prometheus_mount_path deploy '../monitoring/prometheus.yml:' './monitoring/prometheus.yml:' 0
  mutant m5_deploy_remove_api_alias deploy $'        aliases:\n          - api  # monitoring/prometheus.yml scrapes api:8080\n' '' 0
  mutant m6_root_remove_token_passthrough root "$PASSTHRU" "$NOPASS" 0
  mutant m7_deploy_remove_token_passthrough deploy "$PASSTHRU" "$NOPASS" 0
  mutant m8_otel_invalid_exporter otel $'exporters:\n' $'exporters:\n  loki:\n    endpoint: http://loki:3100/loki/api/v1/push\n' 1
  mutant m9_otel_image_latest root 'opentelemetry-collector-contrib:0.164.0' 'opentelemetry-collector-contrib:latest' 0
  echo "mutations: $K/$M killed"; [ "$K" -eq "$M" ] && exit 0 || exit 1
fi

for cf in $COMPOSE_FILES; do
  case "$cf" in /*) p="$cf" ;; *) p="$ROOT/$cf" ;; esac
  case "$PROM_CONFIG" in /*) pc="$PROM_CONFIG" ;; *) pc="$ROOT/$PROM_CONFIG" ;; esac
  run_one "$p" "$pc" "$cf"
done
echo "SUMMARY pass=$PASS fail=$FAIL skip=$SKIP"
[ "$FAIL" -eq 0 ] && [ "$PASS" -gt 0 ]
