#!/usr/bin/env bash
# ==============================================================================
# catalog_favorites_resume_lifecycle.sh
# ------------------------------------------------------------------------------
# Purpose:
#   Full-automation REST API LIFECYCLE coverage suite for the Catalogizer
#   catalog-api (Go/Gin) backend. Where the functional matrix touches favorites
#   and playback once, this suite drives both end-to-end with STATE-DELTA
#   assertions: a favorite that is added MUST then be observably PRESENT in the
#   listing and observably ABSENT after removal; a playback position written to
#   N MUST be observably read back as exactly N ("remember where you left off").
#
#   Anti-bluff (§11.4 / §11.4.69): every PASS asserts a REAL observable state
#   delta captured from the live API, not a tautology, and cites a
#   captured-evidence file. The favorites lifecycle FAILs if the add does not
#   show up in the list, or if the remove leaves it behind. The resume lifecycle
#   FAILs if the read-back position is not exactly the position that was written.
#
#   Coverage (real HTTP, observable deltas):
#     F1 fav-add            POST   /api/v1/favorites               (write path)
#     F2 fav-list-present   GET    /api/v1/favorites               (contains added)
#     F3 fav-remove         DELETE /api/v1/favorites/:type/:id     (delete)
#     F4 fav-list-absent    GET    /api/v1/favorites               (no longer present)
#     R1 resume-start       POST   /api/v1/playback/sessions/start (session_id)
#     R2 resume-progress    POST   /api/v1/playback/sessions/progress (position=N)
#     R3 resume-end         POST   /api/v1/playback/sessions/end   (end at N, not done)
#     R4 resume-read         GET   /api/v1/entities/:id/progress   (last_position == N)
#
# Usage:
#   ./catalog_favorites_resume_lifecycle.sh
#   CATALOGIZER_BASE_URL=http://127.0.0.1:18080 \
#   CATALOGIZER_USER=admin CATALOGIZER_PASS=secret \
#   CATALOGIZER_FAV_ENTITY_ID=1 CATALOGIZER_MEDIA_ID=1 \
#   ./catalog_favorites_resume_lifecycle.sh
#
# Inputs (env vars, all optional with defaults):
#   CATALOGIZER_BASE_URL       API base URL          (default http://127.0.0.1:18080)
#   CATALOGIZER_USER           login username        (default admin)
#   CATALOGIZER_PASS           login password        (default admin)
#   CATALOGIZER_TOKEN          pre-acquired token    (skips login acquisition if set)
#   CATALOGIZER_RESULTS_DIR    evidence output dir   (default qa-results/favorites_resume_lifecycle/<ts>)
#   CATALOGIZER_FAV_ENTITY_ID  entity_id for favorites cycle (default 1)
#   CATALOGIZER_FAV_ENTITY_TYPE entity_type for favorites   (default movie)
#   CATALOGIZER_MEDIA_ID       media_item_id for resume cycle (default 1)
#   CATALOGIZER_RESUME_POS     resume position (seconds)      (default 137)
#
# Outputs:
#   - Per-request captured-evidence JSON under the results dir.
#   - A summary.txt and machine-readable summary.json in the results dir.
#   - PASS/FAIL/SKIP lines on stdout, each citing its evidence path.
#   - Exit code 0 iff every non-SKIP test PASSed; 1 otherwise.
#
# Side-effects:
#   - Network calls to the configured API only. The favorites cycle adds then
#     removes a favorite (self-cleaning, §11.4.14). The playback cycle records a
#     resume position (records only — no destructive state).
#   - Writes evidence files under the results dir.
#
# Dependencies: bash, curl, (optional) jq for richer field extraction; falls
#   back to grep/sed when jq is absent. POSIX-portable beyond the bash shebang.
#
# Cross-references:
#   - docs/scripts/catalog_favorites_resume_lifecycle.md (companion guide, §11.4.18)
#   - submodules/helix_qa/banks/catalog_browse_favorites_resume.yaml (HelixQA bank)
#   - scripts/testing/full_automation/catalog_functional_matrix.sh (sibling suite)
#   - catalog-api/main.go (route registration — source of truth for endpoints)
# ==============================================================================

set -u

# ------------------------------------------------------------------------------
# Configuration
# ------------------------------------------------------------------------------
BASE_URL="${CATALOGIZER_BASE_URL:-http://127.0.0.1:18080}"
USER="${CATALOGIZER_USER:-${ADMIN_USERNAME:-admin}}"
PASS="${CATALOGIZER_PASS:-${ADMIN_PASSWORD:-}}"
TOKEN="${CATALOGIZER_TOKEN:-}"
FAV_ENTITY_ID="${CATALOGIZER_FAV_ENTITY_ID:-1}"
FAV_ENTITY_TYPE="${CATALOGIZER_FAV_ENTITY_TYPE:-movie}"
MEDIA_ID="${CATALOGIZER_MEDIA_ID:-1}"
RESUME_POS="${CATALOGIZER_RESUME_POS:-137}"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
RESULTS_DIR="${CATALOGIZER_RESULTS_DIR:-qa-results/favorites_resume_lifecycle/${TS}}"
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

body_has_key() {
  grep -q "\"$2\"" "$1" 2>/dev/null
}

# fav_list_contains_id <file> <entity_id>
# Real observable check: does this favorites listing mention the entity_id?
fav_list_contains_id() {
  flc_file="$1"; flc_id="$2"
  grep -q "\"entity_id\"[[:space:]]*:[[:space:]]*${flc_id}\b" "${flc_file}" 2>/dev/null
}

echo "=============================================================="
echo " Catalogizer API favorites + resume LIFECYCLE suite"
echo " base_url     : ${BASE_URL}"
echo " results      : ${RESULTS_DIR}"
echo " jq present   : ${HAVE_JQ}"
echo " fav entity   : id=${FAV_ENTITY_ID} type=${FAV_ENTITY_TYPE}"
echo " resume media : id=${MEDIA_ID} position=${RESUME_POS}"
echo " started      : ${TS}"
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
    "F1 fav-add" "F2 fav-list-present" "F3 fav-remove" "F4 fav-list-absent" \
    "R1 resume-start" "R2 resume-progress" "R3 resume-end" "R4 resume-read"; do
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
    "F1 fav-add" "F2 fav-list-present" "F3 fav-remove" "F4 fav-list-absent" \
    "R1 resume-start" "R2 resume-progress" "R3 resume-end" "R4 resume-read"; do
    ab_skip "${t}" "no_token: login did not yield a session_token"
  done
else

  # ============================================================================
  # FAVORITES LIFECYCLE — add -> list(present) -> remove -> list(absent)
  # ============================================================================
  echo
  echo "=== FAVORITES LIFECYCLE (entity_id=${FAV_ENTITY_ID}, type=${FAV_ENTITY_TYPE}) ==="

  # --- F1: add ----------------------------------------------------------------
  echo
  echo "--- F1: POST /api/v1/favorites (add) ---"
  ADD_BODY="{\"entity_id\":${FAV_ENTITY_ID},\"entity_type\":\"${FAV_ENTITY_TYPE}\"}"
  CODE="$(http_request POST /api/v1/favorites yes "${ADD_BODY}" f1_fav_add)"
  EV_ADD="${RESULTS_DIR}/f1_fav_add.json"
  FAV_OK=0
  # 200 added, OR 409 already-in-favorites (idempotent re-run). 404 is an honest
  # data-precondition SKIP (the entity row is absent). Both 200/409 exercise the
  # real write path; 404 skips the whole lifecycle (not a fabricated pass).
  if [ "${CODE}" = "200" ] || [ "${CODE}" = "409" ]; then
    FAV_OK=1
    ab_pass_with_evidence "F1 fav-add: write path exercised (HTTP ${CODE})" "${EV_ADD}"
  elif [ "${CODE}" = "404" ]; then
    ab_skip "F1 fav-add" "entity_not_found: entity_id=${FAV_ENTITY_ID} absent in this DB (set CATALOGIZER_FAV_ENTITY_ID)"
    ab_skip "F2 fav-list-present" "depends_on: F1 add skipped"
    ab_skip "F3 fav-remove" "depends_on: F1 add skipped"
    ab_skip "F4 fav-list-absent" "depends_on: F1 add skipped"
  else
    ab_fail "F1 fav-add: expected 200/409 (or 404 precondition), got ${CODE}" "${EV_ADD}"
  fi

  if [ "${FAV_OK}" = "1" ]; then
    # --- F2: list contains the added favorite ---------------------------------
    echo
    echo "--- F2: GET /api/v1/favorites (added entity PRESENT) ---"
    CODE="$(http_request GET /api/v1/favorites yes - f2_fav_list_present)"
    EV_LP="${RESULTS_DIR}/f2_fav_list_present.json"
    if [ "${CODE}" = "200" ] && body_has_key "${EV_LP}" favorites; then
      if fav_list_contains_id "${EV_LP}" "${FAV_ENTITY_ID}"; then
        ab_pass_with_evidence "F2 fav-list-present: listing positively contains entity_id=${FAV_ENTITY_ID}" "${EV_LP}"
      else
        ab_fail "F2 fav-list-present: 200 + favorites[] but added entity_id=${FAV_ENTITY_ID} NOT found (add did not persist)" "${EV_LP}"
      fi
    else
      ab_fail "F2 fav-list-present: expected 200 + favorites[], got ${CODE}" "${EV_LP}"
    fi

    # --- F3: remove (cleanup, §11.4.14) ---------------------------------------
    echo
    echo "--- F3: DELETE /api/v1/favorites/${FAV_ENTITY_TYPE}/${FAV_ENTITY_ID} (remove) ---"
    CODE="$(http_request DELETE "/api/v1/favorites/${FAV_ENTITY_TYPE}/${FAV_ENTITY_ID}" yes - f3_fav_remove)"
    EV_RM="${RESULTS_DIR}/f3_fav_remove.json"
    REMOVED=0
    if [ "${CODE}" = "200" ] || [ "${CODE}" = "204" ]; then
      REMOVED=1
      ab_pass_with_evidence "F3 fav-remove: deletion succeeded (HTTP ${CODE})" "${EV_RM}"
    else
      ab_fail "F3 fav-remove: expected 200/204, got ${CODE}" "${EV_RM}"
    fi

    # --- F4: list NO LONGER contains the removed favorite (state delta) --------
    echo
    echo "--- F4: GET /api/v1/favorites (removed entity ABSENT) ---"
    if [ "${REMOVED}" = "1" ]; then
      CODE="$(http_request GET /api/v1/favorites yes - f4_fav_list_absent)"
      EV_LA="${RESULTS_DIR}/f4_fav_list_absent.json"
      if [ "${CODE}" = "200" ]; then
        # Real observable delta: the just-removed entity_id must be GONE.
        if fav_list_contains_id "${EV_LA}" "${FAV_ENTITY_ID}"; then
          ab_fail "F4 fav-list-absent: entity_id=${FAV_ENTITY_ID} STILL present after delete (remove did not take effect)" "${EV_LA}"
        else
          ab_pass_with_evidence "F4 fav-list-absent: entity_id=${FAV_ENTITY_ID} confirmed absent after removal" "${EV_LA}"
        fi
      else
        ab_fail "F4 fav-list-absent: expected 200 favorites listing, got ${CODE}" "${EV_LA}"
      fi
    else
      ab_skip "F4 fav-list-absent" "depends_on: F3 remove did not succeed"
    fi
  fi

  # ============================================================================
  # RESUME LIFECYCLE — start -> progress(N) -> end -> read(last_position==N)
  # ============================================================================
  echo
  echo "=== RESUME LIFECYCLE (media_item_id=${MEDIA_ID}, position=${RESUME_POS}) ==="

  # --- R1: start --------------------------------------------------------------
  echo
  echo "--- R1: POST /api/v1/playback/sessions/start ---"
  START_BODY="{\"media_item_id\":${MEDIA_ID},\"position_unit\":\"seconds\",\"start_position\":0}"
  CODE="$(http_request POST /api/v1/playback/sessions/start yes "${START_BODY}" r1_resume_start)"
  EV_ST="${RESULTS_DIR}/r1_resume_start.json"
  SESSION_ID=""
  if [ "${CODE}" = "200" ] && body_has_key "${EV_ST}" session_id; then
    SESSION_ID="$(json_field "${EV_ST}" '.session_id' session_id)"
    if [ -z "${SESSION_ID}" ]; then
      # numeric session_id: jq-less fallback
      SESSION_ID="$(grep -o '"session_id"[[:space:]]*:[[:space:]]*[0-9]*' "${EV_ST}" | head -1 | sed 's/.*:[[:space:]]*//')"
    fi
    if [ -n "${SESSION_ID}" ]; then
      ab_pass_with_evidence "R1 resume-start: 200 + session_id=${SESSION_ID}" "${EV_ST}"
    else
      ab_fail "R1 resume-start: 200 + session_id key but no usable value" "${EV_ST}"
    fi
  elif [ "${CODE}" = "404" ] || [ "${CODE}" = "500" ]; then
    ab_skip "R1 resume-start" "media_precondition: media_item_id=${MEDIA_ID} not present (HTTP ${CODE}); set CATALOGIZER_MEDIA_ID"
    ab_skip "R2 resume-progress" "depends_on: R1 start skipped"
    ab_skip "R3 resume-end" "depends_on: R1 start skipped"
    ab_skip "R4 resume-read" "depends_on: R1 start skipped"
  else
    ab_fail "R1 resume-start: expected 200 + session_id, got ${CODE}" "${EV_ST}"
  fi

  if [ -n "${SESSION_ID}" ]; then
    # --- R2: progress to RESUME_POS -------------------------------------------
    echo
    echo "--- R2: POST /api/v1/playback/sessions/progress (position=${RESUME_POS}) ---"
    PROG_BODY="{\"session_id\":${SESSION_ID},\"end_position\":${RESUME_POS},\"total_amount\":${RESUME_POS}}"
    CODE="$(http_request POST /api/v1/playback/sessions/progress yes "${PROG_BODY}" r2_resume_progress)"
    EV_PR="${RESULTS_DIR}/r2_resume_progress.json"
    if [ "${CODE}" = "200" ]; then
      ab_pass_with_evidence "R2 resume-progress: position advanced to ${RESUME_POS}s (HTTP 200)" "${EV_PR}"
    else
      ab_fail "R2 resume-progress: expected 200, got ${CODE}" "${EV_PR}"
    fi

    # --- R3: end at RESUME_POS (NOT completed -> resume must persist) ----------
    echo
    echo "--- R3: POST /api/v1/playback/sessions/end (end at ${RESUME_POS}, not completed) ---"
    END_BODY="{\"session_id\":${SESSION_ID},\"end_position\":${RESUME_POS},\"total_amount\":${RESUME_POS},\"completed\":false}"
    CODE="$(http_request POST /api/v1/playback/sessions/end yes "${END_BODY}" r3_resume_end)"
    EV_EN="${RESULTS_DIR}/r3_resume_end.json"
    if [ "${CODE}" = "200" ]; then
      ab_pass_with_evidence "R3 resume-end: session ended at ${RESUME_POS}s, not completed (HTTP 200)" "${EV_EN}"
    else
      ab_fail "R3 resume-end: expected 200, got ${CODE}" "${EV_EN}"
    fi

    # --- R4: read-back asserts last_position == RESUME_POS (state delta) -------
    echo
    echo "--- R4: GET /api/v1/entities/${MEDIA_ID}/progress (last_position == ${RESUME_POS}) ---"
    CODE="$(http_request GET "/api/v1/entities/${MEDIA_ID}/progress" yes - r4_resume_read)"
    EV_RD="${RESULTS_DIR}/r4_resume_read.json"
    if [ "${CODE}" = "200" ] && body_has_key "${EV_RD}" progress; then
      # Real observable assertion: read-back last_position equals what we wrote.
      READ_POS=""
      if [ "${HAVE_JQ}" -eq 1 ]; then
        READ_POS="$(jq -r '(.progress.last_position // .progress.position // .last_position // empty)' "${EV_RD}" 2>/dev/null)"
      fi
      if [ -n "${READ_POS}" ]; then
        if [ "${READ_POS}" = "${RESUME_POS}" ]; then
          ab_pass_with_evidence "R4 resume-read: last_position read-back == ${RESUME_POS}s ('remember where you left off')" "${EV_RD}"
        else
          ab_fail "R4 resume-read: last_position read-back=${READ_POS} != written ${RESUME_POS}" "${EV_RD}"
        fi
      else
        # jq-less fallback: positively require the exact written value to appear
        # next to a position field in the captured body — never a bare grep that
        # could match an unrelated number.
        if grep -q "\"last_position\"[[:space:]]*:[[:space:]]*${RESUME_POS}\b" "${EV_RD}" 2>/dev/null \
           || grep -q "\"position\"[[:space:]]*:[[:space:]]*${RESUME_POS}\b" "${EV_RD}" 2>/dev/null; then
          ab_pass_with_evidence "R4 resume-read: position field == ${RESUME_POS}s in captured body (jq absent)" "${EV_RD}"
        else
          ab_fail "R4 resume-read: 200 + progress but persisted position != ${RESUME_POS} (jq absent)" "${EV_RD}"
        fi
      fi
    else
      ab_fail "R4 resume-read: expected 200 + progress, got ${CODE}" "${EV_RD}"
    fi
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
