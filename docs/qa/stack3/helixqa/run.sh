#!/usr/bin/env bash
# HelixQA smoke against QA stack 3 (live). No stubs (11.4.289).
# EXIT MAP (closed): 0 all PASS incl. UI vision | 1 FAIL | 2 PARTIAL (API green,
# UI vision unverified; this is the normal result until a validated vision
# provider exists, so exit 0 is currently unreachable by design) | 3 SKIP
# (binary or credentials absent) | 4 tool/usage error | 5 precheck BLOCKED.
# Secrets: only ADMIN_USERNAME/ADMIN_PASSWORD are parsed (never sourced/executed)
# from QA_ENV; login uses curl -K - (stdin config, nothing in argv); the bearer
# goes to helixqa via a 0600 temp bank, deleted on exit. Nothing secret is printed.
# Test hooks: QA_ENV, HELIXQA_BIN, BANK (alternate bank), API_URL, WEB_URL, OUT.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/../../../.." && pwd)"
QA_ENV="${QA_ENV:-/home/milosvasic/.cache/claude-tmp/catqa3/qa.env}"
API_URL="${API_URL:-http://127.0.0.1:38082}"
. "$REPO/docs/qa/lib/lan_ip.sh"
if [ -z "${WEB_URL:-}" ]; then _lan="$(qa_lan_ip)" || { echo "[helixqa-stack3] cannot detect the LAN address; set WEB_URL or QA_LAN_IP" >&2; exit 4; }; WEB_URL="http://${_lan}:33080"; fi
BANK="${BANK:-$HERE/smoke_bank.yaml}"
EXPECTED_IDS="${EXPECTED_IDS:-S3-SMOKE-001,S3-SMOKE-002,S3-SMOKE-003}"
HELIXQA_BIN="${HELIXQA_BIN:-}"
OUT="${OUT:-$HERE/evidence/$(date +%Y%m%d_%H%M%S)}"
mkdir -p "$OUT" || exit 4
TMPB=""; TOKEN=""; CHILD=""; DONE=0
logout(){ [ -n "$TOKEN" ] && printf 'header = "Authorization: Bearer %s"\nurl = "%s/api/v1/auth/logout"\nrequest = "POST"\n' "$TOKEN" "$API_URL" | curl -s -m 10 -K - -o /dev/null; TOKEN=""; }
cleanup(){ [ -n "$TMPB" ] && rm -f -- "$TMPB"; logout; }
# F3: on TERM/INT stop only the recorded child pid (never a process group), clean up, write overall:
on_sig(){ [ -n "$CHILD" ] && kill "$CHILD" 2>/dev/null; if [ "$DONE" -eq 0 ]; then DONE=1; { echo "overall: ERROR interrupted by signal"; echo "exit: 4"; } >> "$OUT/verdict.txt"; fi; cleanup; exit 4; }
trap cleanup EXIT; trap on_sig TERM INT
log(){ printf '[helixqa-stack3] %s\n' "$*" | tee -a "$OUT/run.log"; }
finish(){ # code overall-text
  DONE=1; { echo "overall: $2"; echo "exit: $1"; } >> "$OUT/verdict.txt"; log "overall: $2 (exit $1)"; exit "$1"; }

# --- credentials: parse two KEY= lines, never execute the file (S2)
[ -r "$QA_ENV" ] || finish 3 "SKIP credentials_absent (QA_ENV unreadable)"
getkey(){ sed -n "s/^$1=//p" "$QA_ENV" | head -n1 | sed -e "s/^['\"]//" -e "s/['\"]\$//"; }
U="$(getkey ADMIN_USERNAME)"; P="$(getkey ADMIN_PASSWORD)"
[ -n "$U" ] && [ -n "$P" ] || finish 3 "SKIP credentials_absent (ADMIN_USERNAME/ADMIN_PASSWORD missing)"

if [ -z "$HELIXQA_BIN" ]; then
  for c in "$HERE/bin/helixqa" "$REPO/submodules/helix_qa/bin/helixqa"; do [ -x "$c" ] && HELIXQA_BIN="$c" && break; done
fi
[ -n "$HELIXQA_BIN" ] && [ -x "$HELIXQA_BIN" ] || finish 3 "SKIP tool_absent (helixqa binary; build per README)"
[ -r "$BANK" ] || finish 4 "ERROR bank unreadable"

# --- prechecks + fingerprint (T3)
curl -sf -m 10 "$API_URL/health" -o "$OUT/health.json" || finish 5 "BLOCKED api unreachable"
curl -sf -m 10 -o /dev/null "$WEB_URL/" || finish 5 "BLOCKED web unreachable"
{
  echo "timestamp: $(date -Iseconds)"
  echo "helixqa_sha256: $(sha256sum "$HELIXQA_BIN" | cut -d' ' -f1)"
  echo "helix_qa_commit_at_build: $(cat "$HELIXQA_BIN.stamp" 2>/dev/null || echo UNKNOWN) (build stamp written by the README build command)"
  echo "api_url: $API_URL  web_url: $WEB_URL"
  python3 -I -c 'import json,sys;d=json.load(open(sys.argv[1]));print("target_build_number:",d.get("build_number"));print("target_version:",d.get("version"))' "$OUT/health.json"
  PORT="${API_URL##*:}"
  echo "answering_container: $(podman ps --format '{{.Names}} {{.Image}} {{.Ports}}' 2>/dev/null | grep -F ":${PORT}->" | awk '$1 ~ /-api$/ {print $1" "$2}' | tr '\n' ';') (API container of the pod that publishes :${PORT})"
} > "$OUT/verdict.txt"

# --- login with stdin curl config (no secret in argv, S1)
# F1: python builds the curl config; user/pass come via ENV (not argv), JSON and curl-config escaping handled.
TOKEN="$(U="$U" P="$P" python3 -I -c 'import os,json,sys
body=json.dumps({"username":os.environ["U"],"password":os.environ["P"]})
esc=lambda t:t.replace("\\","\\\\").replace("\"","\\\"")
sys.stdout.write("url = \"%s/api/v1/auth/login\"\nheader = \"Content-Type: application/json\"\ndata = \"%s\"\n"%(sys.argv[1],esc(body)))' "$API_URL" \
  | curl -s -m 15 -K - | python3 -I -c 'import sys,json
try: print(json.load(sys.stdin).get("session_token",""))
except Exception: print("")')"
unset P
[ -n "$TOKEN" ] || finish 5 "BLOCKED login failed (no session_token)"
TMPB="$(mktemp "${TMPDIR:-/tmp}/s3bank.XXXXXX")"; chmod 600 "$TMPB"
BANKTXT="$(<"$BANK")"; printf '%s\n' "${BANKTXT//__SESSION_TOKEN__/"$TOKEN"}" > "$TMPB"; unset BANKTXT   # builtins only: token never in argv

"$HELIXQA_BIN" http --bank "$TMPB" --base-url "$API_URL" --verbose --json > "$OUT/http_api.out" 2> "$OUT/http_api.err" &
CHILD=$!; wait "$CHILD"; rc=$?; CHILD=""
log "helixqa http exit=$rc"
# split text and JSON (valid JSON file)
python3 -I - "$OUT/http_api.out" "$OUT/http_api.json" "$OUT/http_api.txt" <<'PY'
import sys
s=open(sys.argv[1],errors="replace").read()
i=s.find('{\n  "base_url"')
open(sys.argv[3],"w").write(s[:i] if i>=0 else s)
open(sys.argv[2],"w").write(s[i:] if i>=0 else "")
PY
rm -f "$OUT/http_api.out"

# --- T1 strict report check
python3 -I - "$OUT/http_api.json" "$EXPECTED_IDS" > "$OUT/strict_check.txt" <<'PY'
import sys,json
try: r=json.load(open(sys.argv[1]))
except Exception as e: print("report invalid/empty:",e); sys.exit(1)
exp=sys.argv[2].split(",")
ids=[c["id"] for c in r.get("cases",[])]
ok = (sorted(ids)==sorted(exp) and r.get("total_cases")==len(exp)
      and r.get("passed_cases")==len(exp) and r.get("skipped_cases")==0
      and r.get("failed_cases")==0 and r.get("errored_cases")==0)
print("report check:", "OK" if ok else "MISMATCH", "ids=%s total=%s passed=%s skipped=%s failed=%s errored=%s"%(ids,r.get("total_cases"),r.get("passed_cases"),r.get("skipped_cases"),r.get("failed_cases"),r.get("errored_cases")))
sys.exit(0 if ok else 1)
PY
chk=$?
log "$(cat "$OUT/strict_check.txt")"; { echo "strict_check: $(cat "$OUT/strict_check.txt")"; } >> "$OUT/verdict.txt"
if [ ! -s "$OUT/http_api.json" ]; then
  finish 4 "ERROR helixqa produced no JSON report (exit $rc); see http_api.err"
fi
if [ "$rc" -ne 0 ] || [ "$chk" -ne 0 ]; then
  finish 1 "FAIL api smoke (helixqa exit $rc, report check $chk)"
fi

# --- UI vision leg (never faked)
if [ -z "${HELIX_VISION_ENDPOINT:-}" ]; then
  echo "ui_vision: SKIP vision_provider_absent" >> "$OUT/verdict.txt"
elif ! LLMCTL_VALIDATE_STRICT=1 llmctl require-validated "${HELIXQA_LLMCTL_PROFILE:-vision}" >>"$OUT/run.log" 2>&1; then
  echo "ui_vision: SKIP not_validated" >> "$OUT/verdict.txt"
else
  echo "ui_vision: BLOCKED web_vision_bank_owed" >> "$OUT/verdict.txt"
fi
finish 2 "PARTIAL (exit 2) - API 3/3 PASS, UI NOT verified"
