#!/usr/bin/env bash
# Run sonar-scanner (container) over catalog-api (Go) and catalog-web (TS), wait for each quality gate, and export
# machine-readable results. Pure script: no LLM/model calls. Needs ./up.sh first.
#   RUN_ID=run1 ./scan.sh [--fail-on-gate]      results -> $OUT/$RUN_ID/
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; . "$HERE/lib.sh"
REPO="${REPO:-$(git -C "$HERE" rev-parse --show-toplevel)}"
OUT="${OUT:-$REPO/docs/qa/sonarqube-20261105}"; RUN_ID="${RUN_ID:-run1}"
FAIL_ON_GATE=0; [ "${1:-}" = "--fail-on-gate" ] && FAIL_ON_GATE=1
SQ_PROJECTS="${SQ_PROJECTS:-catalog-api catalog-web}"
[ -f "$SQ_DIR/sonar.env" ] || die "no $SQ_DIR/sonar.env - run ./up.sh first"
load_secrets; : "${SONAR_TOKEN:?run ./up.sh first (no scan token)}"
podman pod exists "$POD" || die "pod $POD not running - run ./up.sh first"
write_netrc admin "$SONAR_ADMIN_PASSWORD"
sq "$API_URL/api/authentication/validate" | jq -e .valid >/dev/null || die "admin login failed"
wait_for_load 900

R="$OUT/$RUN_ID"; mkdir -p "$R"; WORK="$(mktemp -d "$SQ_DIR/work.XXXXXX")"; chmod 700 "$WORK"
( umask 077; printf 'SONAR_TOKEN=%s\n' "$SONAR_TOKEN" > "$SQ_DIR/scanner.env" )
trap 'rm -rf "${WORK:?}"; rm -f "$SQ_DIR/scanner.env"' EXIT
GIT_SHA="$(git -C "$REPO" rev-parse HEAD)"
printf '{"run_id":"%s","started_utc":"%s","git_head":"%s","sonarqube_url":"%s"}\n' "$RUN_ID" "$(date -u +%FT%TZ)" "$GIT_SHA" "$API_URL" > "$R/run.json"
sq "$API_URL/api/system/status" > "$R/server_status.json"
podman image inspect "$SONAR_IMAGE" --format '{{.Digest}}' > "$R/sonarqube_image_digest.txt"
podman image inspect "$SCANNER_IMAGE" --format '{{.Digest}}' > "$R/scanner_image_digest.txt"
overall=0

for P in $SQ_PROJECTS; do
  case "$P" in catalog-api) PROPS=sonar-catalog-api.properties;; catalog-web) PROPS=sonar-catalog-web.properties;; *) die "unknown project $P";; esac
  KEY="$(sed -n 's/^sonar.projectKey=//p' "$HERE/$PROPS")"; mkdir -p "$WORK/$P"
  echo "== scanning $P (key $KEY) =="
  podman run --rm --pod "$POD" --user 0:0 --memory "$SCANNER_MEM" --memory-swap "$SCANNER_MEM" --cpus "$SCANNER_CPUS" \
    --env-file "$SQ_DIR/scanner.env" -e SONAR_SCANNER_OPTS="-Xmx${SCANNER_XMX}" \
    -v "$REPO:/usr/src:ro" -v "$WORK/$P:/work" -w /usr/src "$SCANNER_IMAGE" \
    sonar-scanner -Dproject.settings="/usr/src/docs/qa/stack3/sonarqube/$PROPS" \
      -Dsonar.host.url=http://127.0.0.1:9000 -Dsonar.working.directory=/work \
      -Dsonar.projectVersion="$GIT_SHA" -Dsonar.qualitygate.wait=false > "$WORK/$P.scanner.log" 2>&1 \
    || { cp "$WORK/$P.scanner.log" "$R/$P.scanner.FAILED.log"; scrub_file "$R/$P.scanner.FAILED.log"; grep -nE 'ERROR|WARN|Caused|Exception' "$R/$P.scanner.FAILED.log" | head -15 >&2; die "scanner failed for $P"; }
  cp "$WORK/$P.scanner.log" "$R/$P.scanner.log"
  RT="$WORK/$P/report-task.txt"; [ -f "$RT" ] || die "no report-task.txt for $P"
  CE="$(sed -n 's/^ceTaskId=//p' "$RT")"
  for i in $(seq 1 180); do                       # wait for the background compute-engine task (<=15 min)
    sq "$API_URL/api/ce/task?id=$CE" > "$R/$P.ce_task.json"
    ST="$(jq -r .task.status "$R/$P.ce_task.json")"
    case "$ST" in SUCCESS) break;; FAILED|CANCELED) die "CE task $CE $ST for $P";; esac; sleep 5
  done
  [ "$ST" = SUCCESS ] || die "CE task for $P not finished"
  AID="$(jq -r .task.analysisId "$R/$P.ce_task.json")"
  sq "$API_URL/api/qualitygates/project_status?analysisId=$AID" > "$R/$P.quality_gate.json"
  GATE="$(jq -r .projectStatus.status "$R/$P.quality_gate.json")"
  # Issues: page size 500, API caps at 10000 per query; split by severity if the total is larger.
  TOTAL="$(sq "$API_URL/api/issues/search?componentKeys=$KEY&ps=1" | jq .total)"
  : > "$WORK/$P.issues.ndjson"
  if [ "$TOTAL" -le 10000 ]; then SPLITS="ALL"; else SPLITS="INFO MINOR MAJOR CRITICAL BLOCKER"; fi
  for S in $SPLITS; do
    F="&resolved=false"; [ "$S" = ALL ] || F="$F&severities=$S"
    PG=1; while :; do
      sq "$API_URL/api/issues/search?componentKeys=$KEY&ps=500&p=$PG$F" > "$WORK/pg.json"
      jq -c '.issues[]' "$WORK/pg.json" >> "$WORK/$P.issues.ndjson"
      N="$(jq '.paging.total' "$WORK/pg.json")"; [ $((PG*500)) -ge "$N" ] && break; PG=$((PG+1)); [ "$PG" -le 20 ] || break
    done
  done
  jq -s --arg key "$KEY" --argjson reported "$TOTAL" '{project:$key,reported_total:$reported,exported:length,issues:.}' "$WORK/$P.issues.ndjson" > "$R/$P.issues.json"
  sq "$API_URL/api/measures/component?component=$KEY&metricKeys=ncloc,bugs,vulnerabilities,code_smells,security_hotspots,duplicated_lines_density,reliability_rating,security_rating,sqale_rating,violations" > "$R/$P.measures.json"
  sq "$API_URL/api/issues/search?componentKeys=$KEY&resolved=false&ps=1&facets=severities,types,rules" | jq '{total,facets}' > "$R/$P.issue_facets.json"
  sq "$API_URL/api/hotspots/search?projectKey=$KEY&ps=500" > "$R/$P.hotspots.json"
  printf '%s %s\n' "$P" "$GATE" >> "$R/gate_status.txt"
  echo "$P: gate=$GATE issues=$TOTAL (exported $(jq .exported "$R/$P.issues.json"))"
  [ "$GATE" = OK ] || overall=1
done

# Scrub secrets from every exported file, then prove it with a control needle (planted secret must vanish; clean file must stay clean).
CN="$WORK/needle.txt"; printf 'x %s y\n' "$SONAR_TOKEN" > "$CN"
scrub_file "$CN"; no_secret_in "$CN" || die "scrub control needle: secret survived scrub (scrubber blind)"
grep -q '\[REDACTED\]' "$CN" || die "scrub control needle: replacement marker missing"
printf 'x y\n' > "$CN.clean"; no_secret_in "$CN.clean" || die "negative control tripped"
for f in "$R"/*; do scrub_file "$f"; no_secret_in "$f" || die "secret value present in $f after scrub"; done
echo "scrub control needle: PASS (planted token redacted, clean file clean, $(ls "$R" | wc -l) files checked)" | tee "$R/scrub_control.txt"
( cd "$R" && sha256sum $(ls | grep -v '^SHA256SUMS$') > SHA256SUMS )
echo "results: $R (SHA256SUMS written); quality-gate overall: $([ $overall = 0 ] && echo OK || echo NOT-OK)"
[ "$FAIL_ON_GATE" = 1 ] && exit "$overall"; exit 0
