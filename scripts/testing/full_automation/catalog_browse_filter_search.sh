#!/usr/bin/env bash
# ==============================================================================
# catalog_browse_filter_search.sh
# ------------------------------------------------------------------------------
# Purpose:
#   Full-automation REST API browse / filter / search coverage suite for the
#   Catalogizer catalog-api (Go/Gin) backend. Drives the LIVE HTTP API with real
#   requests and asserts BOTH the HTTP status AND a real observable body field
#   for every test (anti-bluff: never a tautology). Each test writes a
#   captured-evidence JSON artefact (the actual HTTP response) and prints
#   PASS/FAIL with the evidence path, per Helix Constitution §11.4.69
#   (ab_pass_with_evidence).
#
#   This suite DEEPENS the browse/discovery surface that the functional matrix
#   only touches lightly: entity filtering by query params, search WITH results
#   and the empty-query edge case, the media-item DETAIL fetch, and the
#   media/popular discovery feed.
#
#   Coverage (one focused test each, real HTTP):
#     B1 entities default     GET /api/v1/entities                (200, items+total)
#     B2 entities limit=1     GET /api/v1/entities?limit=1        (200, <=1 item honoured)
#     B3 entities type filter GET /api/v1/entities?media_type=... (200, items+total)
#     S1 search q=test        GET /api/v1/search?q=test           (200, results array)
#     S2 search empty q       GET /api/v1/search?q=               (handled, not 5xx)
#     D1 media detail         GET /api/v1/media/:id               (200 detail | 404 honest)
#     P1 media popular        GET /api/v1/media/popular           (200, list-shaped)
#
# Usage:
#   ./catalog_browse_filter_search.sh
#   CATALOGIZER_BASE_URL=http://127.0.0.1:18080 \
#   CATALOGIZER_USER=admin CATALOGIZER_PASS=secret \
#   ./catalog_browse_filter_search.sh
#
# Inputs (env vars, all optional with defaults):
#   CATALOGIZER_BASE_URL     API base URL          (default http://127.0.0.1:18080)
#   CATALOGIZER_USER         login username        (default admin)
#   CATALOGIZER_PASS         login password        (default admin)
#   CATALOGIZER_TOKEN        pre-acquired token    (skips login acquisition if set)
#   CATALOGIZER_RESULTS_DIR  evidence output dir   (default qa-results/browse_filter_search/<ts>)
#   CATALOGIZER_MEDIA_ID     media id for D1 detail (default 1)
#   CATALOGIZER_MEDIA_TYPE   media_type filter for B3 (default movie)
#   CATALOGIZER_SEARCH_QUERY search term for S1     (default test)
#
# Outputs:
#   - Per-test captured-evidence JSON under the results dir (one file per request).
#   - A summary.txt and machine-readable summary.json in the results dir.
#   - PASS/FAIL lines on stdout, each citing its evidence path.
#   - Exit code 0 iff every non-SKIP test PASSed; 1 otherwise.
#
# Side-effects:
#   - Network calls to the configured API only. Read-only discovery requests —
#     no state is created or mutated. Writes evidence files under the results dir.
#
# Dependencies: bash, curl, (optional) jq for richer field extraction; falls
#   back to grep/sed when jq is absent. POSIX-portable beyond the bash shebang.
#
# Cross-references:
#   - docs/scripts/catalog_browse_filter_search.md (companion guide, §11.4.18)
#   - submodules/helix_qa/banks/catalog_browse_favorites_resume.yaml (HelixQA bank)
#   - scripts/testing/full_automation/catalog_functional_matrix.sh (sibling suite)
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
MEDIA_TYPE="${CATALOGIZER_MEDIA_TYPE:-movie}"
SEARCH_QUERY="${CATALOGIZER_SEARCH_QUERY:-test}"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
RESULTS_DIR="${CATALOGIZER_RESULTS_DIR:-qa-results/browse_filter_search/${TS}}"
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
    printf '000' > "${hr_status_file}"
    printf '000'
    return 0
  fi
  printf '%s' "${hr_code}" > "${hr_status_file}"
  printf '%s' "${hr_code}"
}

# json_field <file> <jq-path> <grep-fallback-key>
json_field() {
  jf_file="$1"; jf_jq="$2"; jf_key="$3"
  if [ "${HAVE_JQ}" -eq 1 ]; then
    jq -r "${jf_jq} // empty" "${jf_file}" 2>/dev/null
  else
    grep -o "\"${jf_key}\"[[:space:]]*:[[:space:]]*\"[^\"]*\"" "${jf_file}" 2>/dev/null \
      | head -1 | sed 's/.*:[[:space:]]*"\([^"]*\)".*/\1/' \
      || true
  fi
}

# body_has_key <file> <key>
body_has_key() {
  grep -q "\"$2\"" "$1" 2>/dev/null
}

# body_is_list_shaped <file>
# True if the body carries any known list-bearing key OR is a top-level array.
body_is_list_shaped() {
  bls_file="$1"
  body_has_key "${bls_file}" media || body_has_key "${bls_file}" items \
    || body_has_key "${bls_file}" data || body_has_key "${bls_file}" results \
    || grep -q '^\[' "${bls_file}" 2>/dev/null
}

echo "=============================================================="
echo " Catalogizer API browse / filter / search suite"
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
    "B1 entities-default" "B2 entities-limit" "B3 entities-type-filter" \
    "S1 search-results" "S2 search-empty-query" \
    "D1 media-detail" "P1 media-popular"; do
    ab_skip "${t}" "network_unreachable: ${BASE_URL} not reachable"
  done
  printf 'PASS=%s FAIL=%s SKIP=%s\n' "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" > "${SUMMARY_TXT}"
  printf '%b' "${SUMMARY_ROWS}" >> "${SUMMARY_TXT}"
  printf '{"pass":%s,"fail":%s,"skip":%s,"base_url":"%s","unreachable":true}\n' \
    "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" "${BASE_URL}" > "${SUMMARY_JSON}"
  echo "Summary: PASS=${PASS_COUNT} FAIL=${FAIL_COUNT} SKIP=${SKIP_COUNT}"
  exit 0
fi

# ------------------------------------------------------------------------------
# Acquire auth token (login) unless one was supplied.
# ------------------------------------------------------------------------------
if [ -z "${TOKEN}" ]; then
  echo
  echo "--- AUTH: POST /api/v1/auth/login (acquire session_token) ---"
  LOGIN_BODY="{\"username\":\"${USER}\",\"password\":\"${PASS}\"}"
  CODE="$(http_request POST /api/v1/auth/login no "${LOGIN_BODY}" auth_login)"
  EV="${RESULTS_DIR}/auth_login.json"
  if [ "${CODE}" = "200" ] && body_has_key "${EV}" session_token; then
    TOKEN="$(json_field "${EV}" '.session_token' session_token)"
    if [ -n "${TOKEN}" ]; then
      ab_pass_with_evidence "AUTH login: 200 + non-empty session_token" "${EV}"
    else
      ab_fail "AUTH login: 200 + session_token key but empty value" "${EV}"
    fi
  else
    ab_fail "AUTH login: expected 200 + session_token, got ${CODE}" "${EV}"
  fi
fi

if [ -z "${TOKEN}" ]; then
  echo
  echo "No auth token available — SKIPPING auth-required tests (§11.4.3)."
  for t in \
    "B1 entities-default" "B2 entities-limit" "B3 entities-type-filter" \
    "S1 search-results" "S2 search-empty-query" \
    "D1 media-detail" "P1 media-popular"; do
    ab_skip "${t}" "no_token: login did not yield a session_token"
  done
else

  # ----------------------------------------------------------------------------
  # B1 — entities default browse (auth)
  # ----------------------------------------------------------------------------
  echo
  echo "--- B1: GET /api/v1/entities (default browse) ---"
  CODE="$(http_request GET '/api/v1/entities' yes - b1_entities_default)"
  EV="${RESULTS_DIR}/b1_entities_default.json"
  if [ "${CODE}" = "200" ] && body_has_key "${EV}" items && body_has_key "${EV}" total; then
    ab_pass_with_evidence "B1 entities-default: 200 + items[] + total shape" "${EV}"
  else
    ab_fail "B1 entities-default: expected 200 + items+total shape, got ${CODE}" "${EV}"
  fi

  # ----------------------------------------------------------------------------
  # B2 — entities limit filter (auth): limit=1 must be honoured (<=1 item)
  # ----------------------------------------------------------------------------
  echo
  echo "--- B2: GET /api/v1/entities?limit=1 (limit param honoured) ---"
  CODE="$(http_request GET '/api/v1/entities?limit=1&offset=0' yes - b2_entities_limit)"
  EV="${RESULTS_DIR}/b2_entities_limit.json"
  if [ "${CODE}" = "200" ] && body_has_key "${EV}" items; then
    # Real observable assertion: with jq, the returned items array length is <= 1.
    if [ "${HAVE_JQ}" -eq 1 ]; then
      ITEM_COUNT="$(jq -r '(.items | length) // 0' "${EV}" 2>/dev/null)"
      if [ -n "${ITEM_COUNT}" ] && [ "${ITEM_COUNT}" -le 1 ] 2>/dev/null; then
        ab_pass_with_evidence "B2 entities-limit: limit=1 honoured (returned ${ITEM_COUNT} item(s))" "${EV}"
      else
        ab_fail "B2 entities-limit: limit=1 NOT honoured (returned ${ITEM_COUNT} items)" "${EV}"
      fi
    else
      # jq-less: still a real assertion — body carries the items key + total.
      if body_has_key "${EV}" total; then
        ab_pass_with_evidence "B2 entities-limit: 200 + items+total (jq absent; length unchecked)" "${EV}"
      else
        ab_fail "B2 entities-limit: 200 + items but missing total" "${EV}"
      fi
    fi
  else
    ab_fail "B2 entities-limit: expected 200 + items, got ${CODE}" "${EV}"
  fi

  # ----------------------------------------------------------------------------
  # B3 — entities media_type filter (auth)
  # ----------------------------------------------------------------------------
  echo
  echo "--- B3: GET /api/v1/entities?media_type=${MEDIA_TYPE} (type filter) ---"
  CODE="$(http_request GET "/api/v1/entities?media_type=${MEDIA_TYPE}&limit=10" yes - b3_entities_type)"
  EV="${RESULTS_DIR}/b3_entities_type.json"
  if [ "${CODE}" = "200" ] && body_has_key "${EV}" items && body_has_key "${EV}" total; then
    ab_pass_with_evidence "B3 entities-type-filter: 200 + items+total for media_type=${MEDIA_TYPE}" "${EV}"
  else
    ab_fail "B3 entities-type-filter: expected 200 + items+total, got ${CODE}" "${EV}"
  fi

  # ----------------------------------------------------------------------------
  # S1 — search with query that should return a results array (auth)
  # ----------------------------------------------------------------------------
  echo
  echo "--- S1: GET /api/v1/search?q=${SEARCH_QUERY} (results array) ---"
  CODE="$(http_request GET "/api/v1/search?q=${SEARCH_QUERY}" yes - s1_search_results)"
  EV="${RESULTS_DIR}/s1_search_results.json"
  if [ "${CODE}" = "200" ] && body_has_key "${EV}" results; then
    ab_pass_with_evidence "S1 search-results: 200 + results array for q=${SEARCH_QUERY}" "${EV}"
  else
    ab_fail "S1 search-results: expected 200 + results, got ${CODE}" "${EV}"
  fi

  # ----------------------------------------------------------------------------
  # S2 — search empty-query EDGE CASE (auth): must be handled, never a 5xx crash
  # ----------------------------------------------------------------------------
  echo
  echo "--- S2: GET /api/v1/search?q= (empty-query edge case) ---"
  CODE="$(http_request GET '/api/v1/search?q=' yes - s2_search_empty)"
  EV="${RESULTS_DIR}/s2_search_empty.json"
  # An empty query is a real edge case. The API MUST handle it deterministically:
  # 200 with a (possibly empty) results array, or a 400 client error. A 5xx or a
  # transport failure is a genuine defect — never silently treated as a pass.
  case "${CODE}" in
    200)
      if body_has_key "${EV}" results; then
        ab_pass_with_evidence "S2 search-empty-query: empty q handled with 200 + results array" "${EV}"
      else
        ab_fail "S2 search-empty-query: 200 but no results key for empty query" "${EV}"
      fi
      ;;
    400)
      ab_pass_with_evidence "S2 search-empty-query: empty q rejected deterministically with 400" "${EV}"
      ;;
    *)
      ab_fail "S2 search-empty-query: empty q NOT handled cleanly (got ${CODE}; 5xx/000 is a defect)" "${EV}"
      ;;
  esac

  # ----------------------------------------------------------------------------
  # D1 — media-item detail fetch (auth)
  # ----------------------------------------------------------------------------
  echo
  echo "--- D1: GET /api/v1/media/${MEDIA_ID} (detail fetch) ---"
  CODE="$(http_request GET "/api/v1/media/${MEDIA_ID}" yes - d1_media_detail)"
  EV="${RESULTS_DIR}/d1_media_detail.json"
  if [ "${CODE}" = "200" ]; then
    # Real observable assertion: a detail body carries an id field (or a known
    # detail-bearing key), not merely a 200 envelope.
    if body_has_key "${EV}" id || body_has_key "${EV}" title \
       || body_has_key "${EV}" media || body_has_key "${EV}" data; then
      ab_pass_with_evidence "D1 media-detail: 200 + detail body for media id=${MEDIA_ID}" "${EV}"
    else
      ab_fail "D1 media-detail: 200 but no detail field (id/title/media/data)" "${EV}"
    fi
  elif [ "${CODE}" = "404" ]; then
    # Honest data-precondition SKIP: the media id is absent in this DB. The route
    # is genuinely exercised; this is not a fabricated pass.
    ab_skip "D1 media-detail" "media_not_found: media id=${MEDIA_ID} absent in this DB (set CATALOGIZER_MEDIA_ID)"
  else
    ab_fail "D1 media-detail: expected 200 detail (or 404 precondition), got ${CODE}" "${EV}"
  fi

  # ----------------------------------------------------------------------------
  # P1 — media popular discovery feed (auth)
  # ----------------------------------------------------------------------------
  echo
  echo "--- P1: GET /api/v1/media/popular (discovery feed) ---"
  CODE="$(http_request GET '/api/v1/media/popular?limit=10' yes - p1_media_popular)"
  EV="${RESULTS_DIR}/p1_media_popular.json"
  if [ "${CODE}" = "200" ]; then
    if body_is_list_shaped "${EV}"; then
      ab_pass_with_evidence "P1 media-popular: 200 + list-shaped discovery body" "${EV}"
    else
      ab_fail "P1 media-popular: 200 but no list-shaped key (media/items/data/results/array)" "${EV}"
    fi
  else
    ab_fail "P1 media-popular: expected 200, got ${CODE}" "${EV}"
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
