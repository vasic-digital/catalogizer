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
# Usage: bash scripts/monitoring/test_prometheus_token_start.sh            (exit 0 all PASS, 1 any FAIL)
#        bash scripts/monitoring/test_prometheus_token_start.sh --mutations (each mutant must make the test FAIL)
# Env  : COMPOSE_FILES (space separated, default: docker-compose.yml deployment/docker-compose.yml),
#        PROM_CONFIG (default monitoring/prometheus.yml), PROM_IMAGE (default docker.io/prom/prometheus:latest).
set -u
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_FILES="${COMPOSE_FILES:-docker-compose.yml deployment/docker-compose.yml}"
PROM_CONFIG="${PROM_CONFIG:-monitoring/prometheus.yml}"
PROM_IMAGE="${PROM_IMAGE:-docker.io/prom/prometheus:latest}"
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
  # live part
  command -v podman >/dev/null || { SKIP=$((SKIP+1)); echo "SKIP $label live: podman absent (hardware_not_present-class, not a PASS)"; return; }
  podman image exists "$PROM_IMAGE" || { SKIP=$((SKIP+1)); echo "SKIP $label live: image $PROM_IMAGE absent (not a PASS)"; return; }
  local sh_script; sh_script="$(printf '%s' "$j" | python3 -c 'import json,sys;e=json.load(sys.stdin)["entrypoint"];print(e[2].replace("$$","$"))')"
  local cmdargs; mapfile -t cmdargs < <(printf '%s' "$j" | python3 -c 'import json,sys;[print(c) for c in json.load(sys.stdin).get("command",[])]')
  local name="catmon-tokentest-$$-${RANDOM}" out rc
  # (a) unset token
  out="$(timeout 25 podman run --rm --name "$name-a" --entrypoint /bin/sh -v "$pc:/etc/prometheus/prometheus.yml:ro" "$PROM_IMAGE" -c "$sh_script" prometheus "${cmdargs[@]}" 2>&1)"; rc=$?
  if [ "$rc" -ne 0 ] && printf '%s' "$out" | grep -q 'METRICS_SCRAPE_TOKEN must be set'; then ok "$label live: unset token -> exit $rc with clear message"; else bad "$label live: unset token did not fail fast (rc=$rc)"; fi
  podman rm -f "$name-a" >/dev/null 2>&1
  # (b) short token
  out="$(timeout 25 podman run --rm --name "$name-b" -e METRICS_SCRAPE_TOKEN=short --entrypoint /bin/sh -v "$pc:/etc/prometheus/prometheus.yml:ro" "$PROM_IMAGE" -c "$sh_script" prometheus "${cmdargs[@]}" 2>&1)"; rc=$?
  if [ "$rc" -ne 0 ] && printf '%s' "$out" | grep -q 'METRICS_SCRAPE_TOKEN must be set'; then ok "$label live: short token -> exit $rc with clear message"; else bad "$label live: short token did not fail fast (rc=$rc)"; fi
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
  mutant() { # mutant <name> <python-expr transforming text s> [compose|config]
    local name="$1" expr="$2" kind="${3:-compose}" cf="$T/compose.yml" pc="$T/prometheus.yml"
    cp "$ROOT/docker-compose.yml" "$cf"; cp "$ROOT/$PROM_CONFIG" "$pc"
    python3 - "$kind" "$cf" "$pc" "$expr" <<'PY'
import sys
kind, cf, pc, expr = sys.argv[1:5]
p = cf if kind == "compose" else pc
s = open(p).read(); n = eval(expr); assert n != s, "mutation did not change the file"; open(p, "w").write(n)
PY
    M=$((M+1))
    if COMPOSE_FILES="$cf" PROM_CONFIG="$pc" bash "${BASH_SOURCE[0]}" >/dev/null 2>&1; then echo "SURVIVED $name"; else K=$((K+1)); echo "KILLED   $name"; fi
  }
  # PROM_CONFIG is resolved relative to ROOT in normal mode; make absolute-safe here
  mutant m1_readd_host_token_file_mount "s.replace('      - prometheus_data:/prometheus','      - ./monitoring/secrets/metrics_scrape_token:/etc/prometheus/secrets/metrics_scrape_token:ro\n      - prometheus_data:/prometheus',1)"
  mutant m2_drop_length_guard "s.replace('[ \"\$\${#METRICS_SCRAPE_TOKEN}\" -ge 32 ] || { echo \"METRICS_SCRAPE_TOKEN must be set (>= 32 chars, same value as the API); see docs/deployment/MONITORING_GUIDE.md\" >&2; exit 1; }; ','',1)"
  mutant m3_credentials_path_drift "s.replace('/tmp/metrics_scrape_token','/etc/prometheus/secrets/metrics_scrape_token')" config
  echo "mutations: $K/$M killed"; [ "$K" -eq "$M" ] && exit 0 || exit 1
fi

for cf in $COMPOSE_FILES; do
  case "$cf" in /*) p="$cf" ;; *) p="$ROOT/$cf" ;; esac
  case "$PROM_CONFIG" in /*) pc="$PROM_CONFIG" ;; *) pc="$ROOT/$PROM_CONFIG" ;; esac
  run_one "$p" "$pc" "$cf"
done
echo "SUMMARY pass=$PASS fail=$FAIL skip=$SKIP"
[ "$FAIL" -eq 0 ] && [ "$PASS" -gt 0 ]
