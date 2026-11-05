#!/usr/bin/env bash
# HelixQA smoke against QA stack 3 (live). Never stubs (11.4.289): a missing
# real dependency is an honest SKIP with a reason and a distinct exit code.
# Exit: 0 all PASS (API + UI vision) | 1 FAIL | 2 BLOCKED/partial | 3 SKIP (nothing could run)
# Credentials: read by VARIABLE NAME from QA_ENV (default below), never printed.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"
REPO="$(cd "$HERE/../../../.." && pwd)"
QA_ENV="${QA_ENV:-/home/milosvasic/.cache/claude-tmp/catqa3/qa.env}"
API_URL="${API_URL:-http://127.0.0.1:38082}"
WEB_URL="${WEB_URL:-http://192.168.1.115:33080}"
HELIXQA_BIN="${HELIXQA_BIN:-}"
OUT="${OUT:-$HERE/evidence/$(date +%Y%m%d_%H%M%S)}"
mkdir -p "$OUT"
log(){ printf '[helixqa-stack3] %s\n' "$*" | tee -a "$OUT/run.log"; }
skip(){ log "SKIP: $*"; echo "SKIP: $*" > "$OUT/verdict.txt"; exit 3; }

[ -r "$QA_ENV" ] || skip "credentials file $QA_ENV not readable (credentials_absent)"
if [ -z "$HELIXQA_BIN" ]; then
  for c in "$HERE/bin/helixqa" "$REPO/submodules/helix_qa/bin/helixqa"; do [ -x "$c" ] && HELIXQA_BIN="$c" && break; done
fi
[ -n "$HELIXQA_BIN" ] && [ -x "$HELIXQA_BIN" ] || skip "helixqa binary absent; build it in the golang container (see README) and set HELIXQA_BIN (tool_absent)"

curl -sf -m 10 "$API_URL/health" >/dev/null || { log "BLOCKED: API $API_URL not healthy"; echo "BLOCKED: api unreachable" > "$OUT/verdict.txt"; exit 2; }
curl -sf -m 10 -o /dev/null "$WEB_URL/" || { log "BLOCKED: web $WEB_URL unreachable"; echo "BLOCKED: web unreachable" > "$OUT/verdict.txt"; exit 2; }

# Credentials in a subshell; passed to the binary by flag only (not echoed).
(
  set -a; . "$QA_ENV"; set +a
  : "${ADMIN_USERNAME:?}" "${ADMIN_PASSWORD:?}"
  "$HELIXQA_BIN" http --bank "$HERE/smoke_bank.yaml" --base-url "$API_URL" \
     --admin-user "$ADMIN_USERNAME" --admin-pass "$ADMIN_PASSWORD" --verbose --json
) > "$OUT/http_api.json" 2> "$OUT/http_api.err"
rc=$?
log "api bank exit=$rc evidence=$OUT/http_api.json"

# Vision UI leg: requires a validated vision endpoint (11.4.277/278). Never faked.
if [ -z "${HELIX_VISION_ENDPOINT:-}" ]; then
  log "SKIP(UI vision leg): HELIX_VISION_ENDPOINT unset - no validated vision provider (llmctl vision profile does_not_fit under current host load; see README). UI screens NOT verified."
  echo "ui_vision: SKIP vision_provider_absent" >> "$OUT/verdict.txt"
elif ! llmctl require-validated "${HELIXQA_LLMCTL_PROFILE:-vision}" >>"$OUT/run.log" 2>&1; then
  log "SKIP(UI vision leg): llmctl require-validated failed (not_validated)"
  echo "ui_vision: SKIP not_validated" >> "$OUT/verdict.txt"
else
  log "BLOCKED(UI vision leg): endpoint validated but no web vision bank/CDP browser wired yet (owed work)"
  echo "ui_vision: BLOCKED bank_owed" >> "$OUT/verdict.txt"
fi
echo "api_http_exit=$rc" >> "$OUT/verdict.txt"
# API leg green but UI leg not verified => honest partial (2), never 0.
if [ "$rc" -eq 0 ] && ! grep -q "ui_vision: PASS" "$OUT/verdict.txt"; then log "PARTIAL: API PASS, UI vision not verified -> exit 2"; exit 2; fi
exit "$rc"
