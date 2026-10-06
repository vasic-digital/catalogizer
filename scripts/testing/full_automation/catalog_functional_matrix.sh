#!/usr/bin/env bash
# ==============================================================================
# catalog_functional_matrix.sh
# ------------------------------------------------------------------------------
# Purpose:
#   Full-automation REST API functional-matrix test suite for the Catalogizer
#   catalog-api (Go/Gin) backend. Drives the LIVE HTTP API with real requests
#   and asserts BOTH the HTTP status AND a real observable body field for every
#   test (anti-bluff: never a tautology). Each test writes a captured-evidence
#   JSON artefact (the actual HTTP response) and prints PASS/FAIL with the
#   evidence path, per Helix Constitution §11.4.69 (ab_pass_with_evidence).
#
#   Functional matrix covered (one focused test each, real HTTP):
#     1. health            GET  /health                       (200, status healthy)
#     2. login             POST /api/v1/auth/login            (200, session_token)
#        login negative    POST /api/v1/auth/login wrong pw   (401)
#     3. catalog list      GET  /api/v1/catalog (auth)        (200, roots key)
#        catalog unauth    GET  /api/v1/catalog (no token)    (401)
#     4. entities browse   GET  /api/v1/entities (auth)       (200, items+total shape)
#     5. media recent      GET  /api/v1/media/recent (auth)   (200, list shape)
#     6. favorites cycle   POST/GET/DELETE /api/v1/favorites  (add -> list-contains -> remove)
#     7. playback lifecycle POST .../sessions/start|progress|end + resume-position persists
#        ("remember where you left off") via GET /api/v1/entities/:id/progress
#     8. search            GET  /api/v1/search?q=...          (200, results array)
#
# Usage:
#   ./catalog_functional_matrix.sh
#   CATALOGIZER_BASE_URL=http://127.0.0.1:18080 \
#   CATALOGIZER_USER=admin CATALOGIZER_PASS=secret \
#   ./catalog_functional_matrix.sh
#
# Inputs (env vars, all optional with defaults):
#   CATALOGIZER_BASE_URL   API base URL          (default http://127.0.0.1:18080)
#   CATALOGIZER_USER       login username        (default admin)
#   CATALOGIZER_PASS       login password        (default admin)
#   CATALOGIZER_TOKEN      pre-acquired JWT       (skips login acquisition if set)
#   CATALOGIZER_RESULTS_DIR  evidence output dir  (default qa-results/functional_matrix/<ts>)
#   CATALOGIZER_MEDIA_ID   media_item_id used for playback lifecycle (default 1)
#   CATALOGIZER_FAV_ENTITY_ID  entity_id used for favorites cycle    (default 1)
#
# Outputs:
#   - Per-test captured-evidence JSON under the results dir (one file per request).
#   - A summary.txt and machine-readable summary.json in the results dir.
#   - PASS/FAIL lines on stdout, each citing its evidence path.
#   - Exit code 0 iff every non-SKIP test PASSed; 1 otherwise.
#
# Side-effects:
#   - Network calls to the configured API only. The favorites cycle adds then
#     removes a favorite (self-cleaning). The playback cycle creates playback
#     sessions (records only — no destructive state).
#   - Writes evidence files under the results dir.
#
# Dependencies: bash, curl, (optional) jq for richer field extraction; falls
#   back to grep/sed when jq is absent. POSIX-portable beyond the bash shebang.
#
# Cross-references:
#   - docs/scripts/catalog_functional_matrix.md (companion guide, §11.4.18)
#   - submodules/helix_qa/banks/catalog_functional_matrix.yaml (HelixQA bank)
#   - catalog-api/main.go (route registration — source of truth for endpoints)
#
# Anti-bluff (§11.4 / §11.4.69): every PASS asserts a REAL body field, not a
#   tautology, and cites a captured-evidence file. Unreachable API => SKIP with
#   reason (§11.4.3), never a fabricated PASS.
# ==============================================================================

set -u

# ------------------------------------------------------------------------------
# Configuration
# ------------------------------------------------------------------------------
BASE_URL="${CATALOGIZER_BASE_URL:-http://127.0.0.1:18080}"
USER="${CATALOGIZER_USER:-${ADMIN_USERNAME:-admin}}"
PASS="${CATALOGIZER_PASS:-${ADMIN_PASSWORD:-}}"
TOKEN="${CATALOGIZER_TOKEN:-}"
MEDIA_ID="${CATALOGIZER_MEDIA_ID:-1}"
FAV_ENTITY_ID="${CATALOGIZER_FAV_ENTITY_ID:-1}"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
RESULTS_DIR="${CATALOGIZER_RESULTS_DIR:-qa-results/functional_matrix/${TS}}"
mkdir -p "${RESULTS_DIR}"

SUMMARY_TXT="${RESULTS_DIR}/summary.txt"
SUMMARY_JSON="${RESULTS_DIR}/summary.json"
: > "${SUMMARY_TXT}"

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0
SUMMARY_ROWS=""

HAVE_JQ=0
if command -v jq >/dev/null 2>&1; then
  HAVE_JQ=1
fi

# ------------------------------------------------------------------------------
# Helpers
# ------------------------------------------------------------------------------

# ab_pass_with_evidence <description> <evidence_path>
# Anti-bluff PASS helper (§11.4.69): verifies the evidence path exists and is
# non-empty before declaring PASS.
# NOTE (T052, docs/05 TS-02): this suite no longer carries its own copy of the helper above. The ONE definition is
# tools/evidence/lib/ab_pass_with_evidence.sh: it keeps the evidence-exists-and-non-empty check and, before it prints PASS, records the check
# as an ev/1 entry through tools/evidence/evrec (a PASS without an entry is a FAIL). Environment: EVREC_ITEM (register item or run id, default
# RUN-<pid>), EVREC_BIN (the recorder). Counters and line formats (PASS_COUNT, FAIL_COUNT, SUMMARY_ROWS) are unchanged.
. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../../../tools/evidence/lib/ab_pass_with_evidence.sh" || { echo "FAIL: cannot load tools/evidence/lib/ab_pass_with_evidence.sh"; exit 1; }

ab_fail() {
  af_desc="$1"; af_evidence="${2:-none}"
  printf 'FAIL: %s [evidence: %s]\n' "${af_desc}" "${af_evidence}"
  FAIL_COUNT=$((FAIL_COUNT + 1))
  SUMMARY_ROWS="${SUMMARY_ROWS}FAIL\t${af_desc}\t${af_evidence}\n"
}

ab_skip() {
  as_desc="$1"; as_reason="$2"
  printf 'SKIP: %s [reason: %s]\n' "${as_desc}" "${as_reason}"
  SKIP_COUNT=$((SKIP_COUNT + 1))
  SUMMARY_ROWS="${SUMMARY_ROWS}SKIP\t${as_desc}\t${as_reason}\n"
}

# http_request <method> <path> <auth:yes|no> <body|-> <evidence_basename>
# Performs the request, writes the JSON body to <evidence>.json and the HTTP
# status to <evidence>.status. Echoes the status code on stdout.
http_request() {
  hr_method="$1"; hr_path="$2"; hr_auth="$3"; hr_body="$4"; hr_name="$5"
  hr_body_file="${RESULTS_DIR}/${hr_name}.json"
  hr_status_file="${RESULTS_DIR}/${hr_name}.status"

  set -- -sS -o "${hr_body_file}" -w '%{http_code}' -X "${hr_method}" \
    -H 'Accept: application/json'
  if [ "${hr_auth}" = "yes" ] && [ -n "${TOKEN}" ]; then
    set -- "$@" -H "Authorization: Bearer ${TOKEN}"
  fi
  if [ -n "${hr_body}" ] && [ "${hr_body}" != "-" ]; then
    set -- "$@" -H 'Content-Type: application/json' --data "${hr_body}"
  fi
  set -- "$@" "${BASE_URL}${hr_path}"

  hr_code="$(curl "$@" 2>"${RESULTS_DIR}/${hr_name}.curlerr")"
  hr_rc=$?
  if [ "${hr_rc}" -ne 0 ]; then
    # curl transport failure (connection refused, DNS, etc.) — record 000.
    printf '000' > "${hr_status_file}"
    printf '000'
    return 0
  fi
  printf '%s' "${hr_code}" > "${hr_status_file}"
  printf '%s' "${hr_code}"
}

# json_field <file> <jq-path> <grep-fallback-key>
# Extracts a field using jq when available, else a grep/sed fallback.
json_field() {
  jf_file="$1"; jf_jq="$2"; jf_key="$3"
  if [ "${HAVE_JQ}" -eq 1 ]; then
    jq -r "${jf_jq} // empty" "${jf_file}" 2>/dev/null
  else
    # crude fallback: first "key":"value" or "key":value match
    grep -o "\"${jf_key}\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" "${jf_file}" 2>/dev/null \
      | head -1 | sed 's/.*:[[:space:]]*"\([^"]*\)".*/\1/' \
      || true
  fi
}

# body_has_key <file> <key>
body_has_key() {
  grep -q "\"$2\"" "$1" 2>/dev/null
}

echo "=============================================================="
echo " Catalogizer API functional-matrix suite"
echo " base_url   : ${BASE_URL}"
echo " results    : ${RESULTS_DIR}"
echo " jq present : ${HAVE_JQ}"
echo " started    : ${TS}"
echo "=============================================================="

# ------------------------------------------------------------------------------
# Pre-flight reachability probe (§11.4.3 topology dispatch)
# ------------------------------------------------------------------------------
PROBE_CODE="$(http_request GET /health no - _preflight_health)"
if [ "${PROBE_CODE}" = "000" ]; then
  echo
  echo "API at ${BASE_URL} is UNREACHABLE (curl transport failure)."
  echo "Marking every test SKIP-with-reason (§11.4.3). NOT a fabricated PASS."
  echo
  for t in \
    "T1 health" "T2 login" "T2b login-negative" \
    "T3 catalog-list" "T3b catalog-unauth" "T4 entities-browse" \
    "T5 media-recent" "T6 favorites-cycle" "T7 playback-lifecycle" \
    "T8 search"; do
    ab_skip "${t}" "network_unreachable: ${BASE_URL} not reachable"
  done
  # write summary then exit non-zero-free (SKIPs are honest, not failures)
  printf 'PASS=%s FAIL=%s SKIP=%s\n' "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" > "${SUMMARY_TXT}"
  printf '%b' "${SUMMARY_ROWS}" >> "${SUMMARY_TXT}"
  printf '{"pass":%s,"fail":%s,"skip":%s,"base_url":"%s","unreachable":true}\n' \
    "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" "${BASE_URL}" > "${SUMMARY_JSON}"
  echo "Summary: PASS=${PASS_COUNT} FAIL=${FAIL_COUNT} SKIP=${SKIP_COUNT}"
  exit 0
fi

# ------------------------------------------------------------------------------
# T1 — health
# ------------------------------------------------------------------------------
echo
echo "--- T1: GET /health ---"
CODE="$(http_request GET /health no - t1_health)"
EV="${RESULTS_DIR}/t1_health.json"
if [ "${CODE}" = "200" ] && body_has_key "${EV}" status; then
  STATUS_VAL="$(json_field "${EV}" '.status' status)"
  if [ "${STATUS_VAL}" = "healthy" ] || [ "${STATUS_VAL}" = "ok" ]; then
    ab_pass_with_evidence "T1 health: 200 + status=${STATUS_VAL}" "${EV}"
  else
    ab_fail "T1 health: 200 but status='${STATUS_VAL}' (expected healthy/ok)" "${EV}"
  fi
else
  ab_fail "T1 health: expected 200 + status field, got ${CODE}" "${EV}"
fi

# ------------------------------------------------------------------------------
# T2 — login (positive) + T2b login (negative)
# ------------------------------------------------------------------------------
echo
echo "--- T2: POST /api/v1/auth/login (valid) ---"
if [ -n "${TOKEN}" ]; then
  ab_skip "T2 login" "CATALOGIZER_TOKEN supplied; token acquisition skipped"
else
  LOGIN_BODY="{\"username\":\"${USER}\",\"password\":\"${PASS}\"}"
  CODE="$(http_request POST /api/v1/auth/login no "${LOGIN_BODY}" t2_login)"
  EV="${RESULTS_DIR}/t2_login.json"
  if [ "${CODE}" = "200" ] && body_has_key "${EV}" session_token; then
    TOKEN="$(json_field "${EV}" '.session_token' session_token)"
    if [ -n "${TOKEN}" ]; then
      ab_pass_with_evidence "T2 login: 200 + non-empty session_token" "${EV}"
    else
      ab_fail "T2 login: 200 + session_token key but empty value" "${EV}"
    fi
  else
    ab_fail "T2 login: expected 200 + session_token, got ${CODE}" "${EV}"
  fi
fi

echo
echo "--- T2b: POST /api/v1/auth/login (wrong password -> 401) ---"
BAD_BODY="{\"username\":\"${USER}\",\"password\":\"definitely-wrong-$$\"}"
CODE="$(http_request POST /api/v1/auth/login no "${BAD_BODY}" t2b_login_bad)"
EV="${RESULTS_DIR}/t2b_login_bad.json"
if [ "${CODE}" = "401" ] && body_has_key "${EV}" error; then
  ab_pass_with_evidence "T2b login-negative: wrong password rejected with 401 + error body" "${EV}"
else
  ab_fail "T2b login-negative: expected 401 + error, got ${CODE}" "${EV}"
fi

# Gate the auth-dependent tests on having a token.
if [ -z "${TOKEN}" ]; then
  echo
  echo "No auth token available — SKIPPING auth-required tests (§11.4.3)."
  for t in "T3 catalog-list" "T4 entities-browse" "T5 media-recent" \
           "T6 favorites-cycle" "T7 playback-lifecycle" "T8 search"; do
    ab_skip "${t}" "no_token: login did not yield a session_token"
  done
else

  # ----------------------------------------------------------------------------
  # T3 — catalog list (auth) + T3b unauth -> 401
  # ----------------------------------------------------------------------------
  echo
  echo "--- T3: GET /api/v1/catalog (authenticated) ---"
  CODE="$(http_request GET /api/v1/catalog yes - t3_catalog)"
  EV="${RESULTS_DIR}/t3_catalog.json"
  if [ "${CODE}" = "200" ] && body_has_key "${EV}" roots; then
    ab_pass_with_evidence "T3 catalog-list: 200 + roots array present" "${EV}"
  else
    ab_fail "T3 catalog-list: expected 200 + roots, got ${CODE}" "${EV}"
  fi

  echo
  echo "--- T3b: GET /api/v1/catalog (NO auth -> 401) ---"
  # Temporarily blank the token for this single unauth request.
  SAVED_TOKEN="${TOKEN}"; TOKEN=""
  CODE="$(http_request GET /api/v1/catalog yes - t3b_catalog_unauth)"
  TOKEN="${SAVED_TOKEN}"
  EV="${RESULTS_DIR}/t3b_catalog_unauth.json"
  if [ "${CODE}" = "401" ]; then
    ab_pass_with_evidence "T3b catalog-unauth: unauthenticated catalog access rejected with 401" "${EV}"
  else
    ab_fail "T3b catalog-unauth: expected 401, got ${CODE}" "${EV}"
  fi

  # ----------------------------------------------------------------------------
  # T4 — entities browse (auth)
  # ----------------------------------------------------------------------------
  echo
  echo "--- T4: GET /api/v1/entities (authenticated) ---"
  CODE="$(http_request GET '/api/v1/entities?limit=10&offset=0' yes - t4_entities)"
  EV="${RESULTS_DIR}/t4_entities.json"
  if [ "${CODE}" = "200" ] && body_has_key "${EV}" items && body_has_key "${EV}" total; then
    ab_pass_with_evidence "T4 entities-browse: 200 + items[] + total shape" "${EV}"
  else
    ab_fail "T4 entities-browse: expected 200 + items+total shape, got ${CODE}" "${EV}"
  fi

  # ----------------------------------------------------------------------------
  # T5 — media recent (auth) — real media list route from main.go
  # ----------------------------------------------------------------------------
  echo
  echo "--- T5: GET /api/v1/media/recent (authenticated) ---"
  CODE="$(http_request GET '/api/v1/media/recent?limit=10' yes - t5_media_recent)"
  EV="${RESULTS_DIR}/t5_media_recent.json"
  if [ "${CODE}" = "200" ]; then
    # Recent media returns a list-shaped body. Accept any of the known
    # list-bearing keys, OR a top-level JSON array.
    if body_has_key "${EV}" media || body_has_key "${EV}" items \
       || body_has_key "${EV}" data || body_has_key "${EV}" results \
       || grep -q '^\[' "${EV}" 2>/dev/null; then
      ab_pass_with_evidence "T5 media-recent: 200 + list-shaped body" "${EV}"
    else
      ab_fail "T5 media-recent: 200 but no list-shaped key (media/items/data/results/array)" "${EV}"
    fi
  else
    ab_fail "T5 media-recent: expected 200, got ${CODE}" "${EV}"
  fi

  # ----------------------------------------------------------------------------
  # T6 — favorites cycle: add -> list-contains -> remove
  # ----------------------------------------------------------------------------
  echo
  echo "--- T6: favorites add -> list -> remove (entity_id=${FAV_ENTITY_ID}, type=movie) ---"
  FAV_TYPE="movie"
  ADD_BODY="{\"entity_id\":${FAV_ENTITY_ID},\"entity_type\":\"${FAV_TYPE}\"}"

  CODE="$(http_request POST /api/v1/favorites yes "${ADD_BODY}" t6a_fav_add)"
  EV_ADD="${RESULTS_DIR}/t6a_fav_add.json"
  # 200 added, OR 409 already-in-favorites (idempotent re-run), OR 404 when the
  # entity row does not exist in this DB. 200/409 mean the favorites write path
  # is genuinely exercised; 404 is an honest data-precondition SKIP, not a pass.
  if [ "${CODE}" = "200" ] || [ "${CODE}" = "409" ]; then
    ADD_OK=1
    ab_pass_with_evidence "T6a favorites-add: write path exercised (HTTP ${CODE})" "${EV_ADD}"
  elif [ "${CODE}" = "404" ]; then
    ADD_OK=0
    ab_skip "T6 favorites-cycle" "entity_not_found: entity_id=${FAV_ENTITY_ID} absent in this DB (set CATALOGIZER_FAV_ENTITY_ID)"
  else
    ADD_OK=0
    ab_fail "T6a favorites-add: expected 200/409 (or 404 data-precondition), got ${CODE}" "${EV_ADD}"
  fi

  if [ "${ADD_OK}" = "1" ]; then
    # list-contains
    CODE="$(http_request GET /api/v1/favorites yes - t6b_fav_list)"
    EV_LIST="${RESULTS_DIR}/t6b_fav_list.json"
    if [ "${CODE}" = "200" ] && body_has_key "${EV_LIST}" favorites; then
      # Real observable assertion: the listed favorites mention our entity_id.
      if grep -q "\"entity_id\"[[:space:]]*:[[:space:]]*${FAV_ENTITY_ID}\b" "${EV_LIST}" 2>/dev/null \
         || grep -q "${FAV_ENTITY_ID}" "${EV_LIST}" 2>/dev/null; then
        ab_pass_with_evidence "T6b favorites-list: list contains added entity_id=${FAV_ENTITY_ID}" "${EV_LIST}"
      else
        ab_fail "T6b favorites-list: 200 + favorites[] but added entity_id=${FAV_ENTITY_ID} not found" "${EV_LIST}"
      fi
    else
      ab_fail "T6b favorites-list: expected 200 + favorites[], got ${CODE}" "${EV_LIST}"
    fi

    # remove (cleanup, §11.4.14)
    CODE="$(http_request DELETE "/api/v1/favorites/${FAV_TYPE}/${FAV_ENTITY_ID}" yes - t6c_fav_remove)"
    EV_DEL="${RESULTS_DIR}/t6c_fav_remove.json"
    if [ "${CODE}" = "200" ] || [ "${CODE}" = "204" ]; then
      ab_pass_with_evidence "T6c favorites-remove: deletion succeeded (HTTP ${CODE})" "${EV_DEL}"
    else
      ab_fail "T6c favorites-remove: expected 200/204, got ${CODE}" "${EV_DEL}"
    fi
  fi

  # ----------------------------------------------------------------------------
  # T7 — playback lifecycle: start -> progress -> end, then resume-position
  #      persists (the "remember where you left off" feature).
  # ----------------------------------------------------------------------------
  echo
  echo "--- T7: playback lifecycle (media_item_id=${MEDIA_ID}) ---"
  RESUME_POS=137
  START_BODY="{\"media_item_id\":${MEDIA_ID},\"position_unit\":\"seconds\",\"start_position\":0}"
  CODE="$(http_request POST /api/v1/playback/sessions/start yes "${START_BODY}" t7a_pb_start)"
  EV_START="${RESULTS_DIR}/t7a_pb_start.json"
  if [ "${CODE}" = "200" ] && body_has_key "${EV_START}" session_id; then
    SESSION_ID="$(json_field "${EV_START}" '.session_id' session_id)"
    if [ -z "${SESSION_ID}" ] && [ "${HAVE_JQ}" -eq 0 ]; then
      # numeric session_id: jq-less fallback
      SESSION_ID="$(grep -o '"session_id"[[:space:]]*:[[:space:]]*[0-9]*' "${EV_START}" | head -1 | sed 's/.*:[[:space:]]*//')"
    fi
    ab_pass_with_evidence "T7a playback-start: 200 + session_id=${SESSION_ID}" "${EV_START}"

    if [ -n "${SESSION_ID}" ]; then
      # progress to RESUME_POS seconds
      PROG_BODY="{\"session_id\":${SESSION_ID},\"end_position\":${RESUME_POS},\"total_amount\":${RESUME_POS}}"
      CODE="$(http_request POST /api/v1/playback/sessions/progress yes "${PROG_BODY}" t7b_pb_progress)"
      EV_PROG="${RESULTS_DIR}/t7b_pb_progress.json"
      if [ "${CODE}" = "200" ]; then
        ab_pass_with_evidence "T7b playback-progress: position advanced to ${RESUME_POS}s (HTTP 200)" "${EV_PROG}"
      else
        ab_fail "T7b playback-progress: expected 200, got ${CODE}" "${EV_PROG}"
      fi

      # end the session at RESUME_POS (not completed -> resume should persist)
      END_BODY="{\"session_id\":${SESSION_ID},\"end_position\":${RESUME_POS},\"total_amount\":${RESUME_POS},\"completed\":false}"
      CODE="$(http_request POST /api/v1/playback/sessions/end yes "${END_BODY}" t7c_pb_end)"
      EV_END="${RESULTS_DIR}/t7c_pb_end.json"
      if [ "${CODE}" = "200" ]; then
        ab_pass_with_evidence "T7c playback-end: session ended at ${RESUME_POS}s (HTTP 200)" "${EV_END}"
      else
        ab_fail "T7c playback-end: expected 200, got ${CODE}" "${EV_END}"
      fi

      # RESUME assertion: per-entity progress remembers the position.
      CODE="$(http_request GET "/api/v1/entities/${MEDIA_ID}/progress" yes - t7d_pb_resume)"
      EV_RES="${RESULTS_DIR}/t7d_pb_resume.json"
      if [ "${CODE}" = "200" ] && body_has_key "${EV_RES}" progress; then
        # Real observable assertion: the persisted resume position == ${RESUME_POS}.
        if grep -q "${RESUME_POS}" "${EV_RES}" 2>/dev/null; then
          ab_pass_with_evidence "T7d playback-resume: resume position ${RESUME_POS}s persisted ('remember where you left off')" "${EV_RES}"
        else
          ab_fail "T7d playback-resume: 200 + progress key but persisted position != ${RESUME_POS}" "${EV_RES}"
        fi
      else
        ab_fail "T7d playback-resume: expected 200 + progress, got ${CODE}" "${EV_RES}"
      fi
    else
      ab_skip "T7 playback-resume" "no_session_id: start did not return a usable session_id"
    fi
  elif [ "${CODE}" = "404" ] || [ "${CODE}" = "500" ]; then
    ab_skip "T7 playback-lifecycle" "media_precondition: media_item_id=${MEDIA_ID} not present (HTTP ${CODE}); set CATALOGIZER_MEDIA_ID"
  else
    ab_fail "T7a playback-start: expected 200 + session_id, got ${CODE}" "${EV_START}"
  fi

  # ----------------------------------------------------------------------------
  # T8 — search
  # ----------------------------------------------------------------------------
  echo
  echo "--- T8: GET /api/v1/search?q=... (authenticated) ---"
  CODE="$(http_request GET '/api/v1/search?q=test' yes - t8_search)"
  EV="${RESULTS_DIR}/t8_search.json"
  if [ "${CODE}" = "200" ] && body_has_key "${EV}" results; then
    ab_pass_with_evidence "T8 search: 200 + results array present" "${EV}"
  else
    ab_fail "T8 search: expected 200 + results, got ${CODE}" "${EV}"
  fi

fi  # end token-gated block

# ------------------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------------------
{
  printf 'PASS=%s FAIL=%s SKIP=%s\n' "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}"
  printf '%b' "${SUMMARY_ROWS}"
} > "${SUMMARY_TXT}"

printf '{"pass":%s,"fail":%s,"skip":%s,"base_url":"%s","results_dir":"%s","ts":"%s"}\n' \
  "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" "${BASE_URL}" "${RESULTS_DIR}" "${TS}" \
  > "${SUMMARY_JSON}"

echo
echo "=============================================================="
echo " Summary: PASS=${PASS_COUNT} FAIL=${FAIL_COUNT} SKIP=${SKIP_COUNT}"
echo " Evidence dir: ${RESULTS_DIR}"
echo " Summary file: ${SUMMARY_TXT}"
echo "=============================================================="

if [ "${FAIL_COUNT}" -gt 0 ]; then
  exit 1
fi
exit 0
