#!/usr/bin/env bash
# webdav_yandex_live.sh - LIVE suite of the real WebDAVClient against the real Yandex WebDAV (build tag live), N identical runs, redacted output.
# Constitution section 11.4.289: real service only; credentials or service absent = honest SKIP, never a stand-in.
# Environment (names only):
#   YANDEX_WEBDAV_ENV_FILE  path of a 0600 file that defines WEBDAV_URL, WEBDAV_USER, WEBDAV_PASSWORD, WEBDAV_TEST_DIR (sourced inside this process only)
#   YANDEX_RUNS             number of identical runs (default 3)
#   YANDEX_EVIDENCE_DIR     where the JSON evidence of each run is written (default: a mktemp dir, printed)
# Credentials never appear on a command line: they travel as environment variables and the output is redacted by awk reading ENVIRON.
# Everything remote stays inside /Testing (guard in the test); heavy runs are limited with systemd-run --user --scope when available.
# Exit: 0 VERDICT=PASS or PASS_WITH_ANOMALIES (all runs passed, signatures identical, guard_violations==2, no lost acked write, no idle cancel; ANOMALIES = transient retries, always printed); 1 FAIL; 2 usage/env error (VERDICT=ERROR); 3 SKIPPED (no env file / credentials / service).
set -u -o pipefail
ENVF="${YANDEX_WEBDAV_ENV_FILE:-}"
[ -n "$ENVF" ] && [ -r "$ENVF" ] || { echo "VERDICT=SKIPPED reason=credentials_absent (YANDEX_WEBDAV_ENV_FILE unset or unreadable)"; exit 3; }
mode=$(stat -c %a "$ENVF" 2>/dev/null || echo 000)
[ "$mode" = "600" ] || { echo "VERDICT=ERROR reason=env_file_mode_$mode (must be 0600)"; exit 2; }
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUNS="${YANDEX_RUNS:-3}"
EVD="${YANDEX_EVIDENCE_DIR:-$(mktemp -d "${TMPDIR:-/tmp}/yandex-webdav-evidence.XXXXXX")}"
mkdir -p "$EVD"
set +x
set -a; . "$ENVF"; set +a
export LIVE_WEBDAV_URL="${WEBDAV_URL:-}" LIVE_WEBDAV_USER="${WEBDAV_USER:-}" LIVE_WEBDAV_PASSWORD="${WEBDAV_PASSWORD:-}" LIVE_WEBDAV_SANDBOX="${WEBDAV_TEST_DIR:-}"
unset WEBDAV_URL WEBDAV_USER WEBDAV_PASSWORD WEBDAV_TEST_DIR
for v in LIVE_WEBDAV_URL LIVE_WEBDAV_USER LIVE_WEBDAV_PASSWORD LIVE_WEBDAV_SANDBOX; do
  [ -n "${!v:-}" ] || { echo "VERDICT=SKIPPED reason=credentials_absent missing=$v"; exit 3; }
done
redact() { awk 'BEGIN{n=split(ENVIRON["LIVE_WEBDAV_USER"] "\n" ENVIRON["LIVE_WEBDAV_PASSWORD"],s,"\n")} {for(i=1;i<=n;i++) if(length(s[i])>0){while((j=index($0,s[i]))>0) $0=substr($0,1,j-1) "<redacted>" substr($0,j+length(s[i]))} print}'; }
wrap=(); command -v systemd-run >/dev/null 2>&1 && wrap=(systemd-run --user --scope --quiet -p MemoryMax=5G -p MemorySwapMax=0)
pass=0; skip=0; fail=0
for i in $(seq 1 "$RUNS"); do
  export LIVE_YANDEX_EVIDENCE="$EVD/run$i.json"
  out=$(cd "$ROOT/catalog-api" && "${wrap[@]}" go test -tags live -count=1 -v -timeout 28m -run '^TestLiveYandex' ./filesystem/ 2>&1); rc=$?
  printf '%s\n' "$out" | redact | grep -E -- '^(=== RUN|--- |\s+--- |PASS|FAIL|ok|SKIP|panic|\s+live_webdav_yandex)' | grep -v '=== RUN' | sed -n '1,200p'
  if [ $rc -ne 0 ]; then fail=$((fail+1)); elif printf '%s\n' "$out" | grep -q -- '--- SKIP: TestLiveYandex_Suite'; then skip=$((skip+1)); else pass=$((pass+1)); fi
  echo "RUN $i GO_RC=$rc"
done
echo "RUNS=$RUNS PASSED=$pass SKIPPED=$skip FAILED=$fail EVIDENCE_DIR=$EVD"
if [ "$fail" -gt 0 ]; then echo VERDICT=FAIL; exit 1; fi
if [ "$pass" -eq 0 ]; then echo "VERDICT=SKIPPED (no live run executed)"; exit 3; fi
if [ "$pass" -ne "$RUNS" ]; then echo "VERDICT=PARTIAL"; exit 3; fi
python3 -I - "$EVD" "$RUNS" <<'PY' || { echo "VERDICT=FAIL reason=signature_or_integrity_check_failed"; exit 1; }
import json,sys
d,n=sys.argv[1],int(sys.argv[2]); sigs=[]; bad=[]; anomalies=0
for i in range(1,n+1):
    e=json.load(open(f"{d}/run{i}.json"))
    notes={s["name"]:{k:v for k,v in (s.get("notes") or {}).items() if k.startswith("sha256_") or k in("entries","files","range_supported","move_status","protocol")} for s in e["steps"]}
    sigs.append(json.dumps({"sig":e["signature"],"n":notes},sort_keys=True))
    print(f"run{i}: requests={e['requests_total']} p50={e['latency_ms_p50']}ms p95={e['latency_ms_p95']}ms max={e['latency_ms_max']}ms up={e['bytes_uploaded']}B status={e['requests_by_status']}")
    print(f"run{i}: guard_violations={e['guard_violations']} transient_retries={e['transient_retries']} retry_classes={e['transient_retry_classes']} lost_acked_writes={e['lost_acked_writes']} idle_cancel_errors={e['idle_cancel_errors']}")
    if e["guard_violations"]!=2: bad.append(f"run{i}: guard_violations={e['guard_violations']} (expected exactly 2 from step 17)")
    if e["lost_acked_writes"]>0: bad.append(f"run{i}: lost_acked_writes={e['lost_acked_writes']} (server data-integrity finding)")
    if e["idle_cancel_errors"]>0: bad.append(f"run{i}: idle_cancel_errors={e['idle_cancel_errors']}")
    anomalies+=e["transient_retries"]
if bad:
    print("INTEGRITY FAILURES: "+"; ".join(bad)); sys.exit(1)
assert len(set(sigs))==1, "signatures differ"
print("SIGNATURES IDENTICAL across",n,"runs")
print("ANOMALIES=%d transient retries across %d runs (service connection resets / 5xx / stalls; see retry_classes)"%(anomalies,n))
sys.exit(0)
PY
if [ $? -ne 0 ]; then exit 1; fi
tot=$(python3 -I - "$EVD" "$RUNS" <<'PY'
import json,sys
print(sum(json.load(open(f"{sys.argv[1]}/run{i}.json"))["transient_retries"] for i in range(1,int(sys.argv[2])+1)))
PY
)
if [ "${tot:-0}" -gt 0 ]; then echo "VERDICT=PASS_WITH_ANOMALIES transient_retries=$tot"; exit 0; fi
echo VERDICT=PASS
