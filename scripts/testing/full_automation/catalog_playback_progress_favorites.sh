#!/usr/bin/env bash
# ==============================================================================
# catalog_playback_progress_favorites.sh
# ------------------------------------------------------------------------------
# Purpose:
#   Standing regression + full-automation API guard proving the catalog-api
#   PLAYBACK-PROGRESS / RESUME-WHERE-LEFT-OFF and FAVORITES endpoints really
#   work end-to-end for an end user (anti-bluff §11.4 / §11.4.69) — set a real
#   value, read it back, assert it is observably equal; then undo the change.
#
#     PPF-A (resume-where-left-off): drive the real playback-session lifecycle
#        POST /playback/sessions/start -> /progress -> /end on a real entity,
#        then GET /entities/{id}/progress and assert the rolled-up
#        `progress.last_position` equals the end_position we wrote — i.e. the
#        media_progress row records exactly where the user left off so a client
#        can RESUME there. POSITIVE: a non-null, equal last_position.
#
#     PPF-B (favorite round-trip add->list->present): POST /favorites
#        {entity_id, entity_type} for a real entity, then GET /favorites and
#        assert the just-added (entity_type, entity_id) pair APPEARS in the
#        list, AND GET /favorites/check/{type}/{id} reports is_favorite:true.
#        POSITIVE: the favorite is observably present after the add.
#
#     PPF-C (favorite remove->gone): DELETE /favorites/{type}/{id} for the
#        favorite added in PPF-B, then GET /favorites/check/{type}/{id} reports
#        is_favorite:false AND the pair is ABSENT from GET /favorites — the
#        remove is observable. NEGATIVE: the favorite is gone after the remove.
#
#     PPF-D (progress determinism §11.4.50): GET /entities/{id}/progress twice;
#        assert the observed last_position is identical across two independent
#        reads (the write in PPF-A produced a stable, deterministic value).
#
#   This is a §11.4.135 standing regression guard wired to run every cycle, and
#   a §11.4.146 extend-to-all-cases pass over the set/get/list/check/remove
#   surfaces. Each PASS cites a captured-evidence JSON path (the REAL HTTP
#   response body) under qa-results/playback_progress_favorites/<ts>.
#
# Self-cleaning (§11.4.14): every WRITE the suite makes it UNDOES on EXIT —
#   the favorite it adds it removes; the playback progress it writes it resets
#   to the pristine "not started" baseline (last_position=0) by ending one
#   final session at position 0 (the API exposes no DELETE for media_progress,
#   so reset-to-baseline is the honest quiescent state — documented, not faked).
#   The conductor OWNS the live DB (§11.4.119 single-resource-owner); this suite
#   NEVER triggers scans, clears, or aggregates — it touches ONLY its own
#   per-user playback_sessions/media_progress/favorites rows for one entity.
#
# Usage:
#   ./catalog_playback_progress_favorites.sh
#   CATALOGIZER_BASE_URL=http://127.0.0.1:8080 \
#   CATALOGIZER_ENV_FILE=/path/.api-env \
#   ./catalog_playback_progress_favorites.sh
#
# Inputs (env vars, all optional with defaults):
#   CATALOGIZER_BASE_URL    API base URL          (default http://127.0.0.1:8080)
#   CATALOGIZER_ENV_FILE    path to .api-env providing ADMIN_USERNAME/ADMIN_PASSWORD
#                           (and optionally QA_TOKEN). Sourced, never echoed
#                           (§11.4.10). Default: the most-recent qa-results
#                           catalogizer-qa-*/.api-env, else none.
#   CATALOGIZER_USER        login username        (default $ADMIN_USERNAME or admin)
#   CATALOGIZER_PASS        login password        (default $ADMIN_PASSWORD; NO
#                           hardcoded fallback secret — §11.4.10)
#   CATALOGIZER_TOKEN       pre-acquired session token (skips login if set; else
#                           QA_TOKEN from the env file, else a real login)
#   CATALOGIZER_RESULTS_DIR evidence output dir
#                           (default qa-results/playback_progress_favorites/<ts>)
#   CATALOGIZER_BROWSE_TYPE entity type to source a real fixture entity from
#                           (default movie). Must be one of the favorites
#                           closed set (movie tv_show tv_season tv_episode
#                           music_artist music_album song game software book
#                           comic).
#   CATALOGIZER_ENTITY_ID   explicit fixture entity id (skips browse discovery)
#   CATALOGIZER_RESUME_POS  end_position (seconds) to write + read back
#                           (default 873)
#
# Outputs:
#   - Per-assertion captured-evidence JSON under the results dir (the real HTTP
#     response body). A summary.txt and machine-readable summary.json.
#   - PASS/FAIL/SKIP lines on stdout, each citing its evidence path.
#   - Exit code 0 iff every non-SKIP assertion PASSed; 1 otherwise.
#
# Side-effects:
#   - Network calls to the configured API only. The ONLY writes are: one
#     playback session (start/progress/end) for the fixture entity + one
#     favorite add — BOTH undone in the EXIT trap (favorite removed; progress
#     reset to baseline 0). No scans, no aggregation, no destructive state.
#     Evidence files under the results dir. NEVER prints credentials (§11.4.10).
#
# Dependencies: bash, curl, python3 (JSON oracle). POSIX-portable beyond the
#   bash shebang; parses clean under both `sh -n` and `bash -n` (§11.4.67).
#
# Cross-references:
#   - docs/scripts/catalog_playback_progress_favorites.md  (companion, §11.4.18)
#   - submodules/helix_qa/banks/catalog_playback_progress_favorites.yaml (bank)
#   - scripts/testing/full_automation/catalog_episode_titles_dedup.sh (model)
#   - catalog-api/handlers/playback_handler.go (endpoints under guard)
#   - catalog-api/handlers/service_handlers.go (favorites endpoints under guard)
#
# Anti-bluff (§11.4 / §11.4.69 / §11.4.27): every PASS asserts a REAL observed
#   value from a captured response body (a read-back last_position EQUAL to the
#   value written; a favorite that APPEARS then DISAPPEARS), never a tautology,
#   and cites its evidence file. Real system, no mocks. Unreachable API / no
#   token / python3-absent => SKIP-with-reason (§11.4.3), never a faked PASS.
# ==============================================================================

set -u

# ------------------------------------------------------------------------------
# Configuration
# ------------------------------------------------------------------------------
BASE_URL="${CATALOGIZER_BASE_URL:-http://127.0.0.1:8080}"
BROWSE_TYPE="${CATALOGIZER_BROWSE_TYPE:-movie}"
RESUME_POS="${CATALOGIZER_RESUME_POS:-873}"

# Resolve an env file (credentials single-source, §11.4.10). Explicit override
# wins; otherwise pick the most-recent qa-results catalogizer-qa-*/.api-env.
ENV_FILE="${CATALOGIZER_ENV_FILE:-}"
if [ -z "${ENV_FILE}" ]; then
  for d in $(ls -dt qa-results/catalogizer-qa-*/.api-env 2>/dev/null); do
    ENV_FILE="${d}"; break
  done
fi
# Source it WITHOUT echoing any value. `set +x` guards against a caller's xtrace.
if [ -n "${ENV_FILE}" ] && [ -f "${ENV_FILE}" ]; then
  set +x
  # shellcheck disable=SC1090
  . "${ENV_FILE}"
fi

USER="${CATALOGIZER_USER:-${ADMIN_USERNAME:-admin}}"
# No hardcoded default password (§11.4.10): fall back ONLY to an env-provided
# ADMIN_PASSWORD; if absent, login simply fails and the suite SKIPs honestly.
PASS="${CATALOGIZER_PASS:-${ADMIN_PASSWORD:-}}"
# Token precedence: explicit env var, else QA_TOKEN from the env file, else login.
TOKEN="${CATALOGIZER_TOKEN:-${QA_TOKEN:-}}"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
RESULTS_DIR="${CATALOGIZER_RESULTS_DIR:-qa-results/playback_progress_favorites/${TS}}"
mkdir -p "${RESULTS_DIR}"

SUMMARY_TXT="${RESULTS_DIR}/summary.txt"
SUMMARY_JSON="${RESULTS_DIR}/summary.json"
: > "${SUMMARY_TXT}"

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0
SUMMARY_ROWS=""

# Mutable fixture state (resolved at runtime). Tracked so the EXIT trap can undo
# exactly what was created and nothing else.
ENTITY_ID="${CATALOGIZER_ENTITY_ID:-}"
ENTITY_TYPE=""
FAV_ADDED=0
PROG_WRITTEN=0

# python3 is the JSON oracle. Refuse to run a tautology if it is absent.
HAVE_PY=0
if command -v python3 >/dev/null 2>&1; then
  HAVE_PY=1
fi

# ------------------------------------------------------------------------------
# Anti-bluff helpers (§11.4.69)
# ------------------------------------------------------------------------------

# ab_pass_with_evidence <description> <evidence_path>
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

ab_skip_with_reason() {
  as_desc="$1"; as_reason="$2"
  printf 'SKIP: %s [reason: %s]\n' "${as_desc}" "${as_reason}"
  SKIP_COUNT=$((SKIP_COUNT + 1))
  SUMMARY_ROWS="${SUMMARY_ROWS}SKIP\t${as_desc}\t${as_reason}\n"
}

# http_get <path> <auth:yes|no> <evidence_basename>  -> echoes HTTP status code
http_get() {
  hg_path="$1"; hg_auth="$2"; hg_name="$3"
  hg_body_file="${RESULTS_DIR}/${hg_name}.json"
  hg_status_file="${RESULTS_DIR}/${hg_name}.status"

  set -- -sS -o "${hg_body_file}" -w '%{http_code}' -X GET -H 'Accept: application/json'
  if [ "${hg_auth}" = "yes" ] && [ -n "${TOKEN}" ]; then
    set -- "$@" -H "Authorization: Bearer ${TOKEN}"
  fi
  set -- "$@" "${BASE_URL}${hg_path}"

  hg_code="$(curl "$@" 2>"${RESULTS_DIR}/${hg_name}.curlerr")"
  hg_rc=$?
  if [ "${hg_rc}" -ne 0 ]; then
    printf '000' > "${hg_status_file}"
    printf '000'
    return 0
  fi
  printf '%s' "${hg_code}" > "${hg_status_file}"
  printf '%s' "${hg_code}"
}

# http_body <method> <path> <json-body> <evidence_basename> -> echoes status code
# Writes the response body to <name>.json and the status to <name>.status. Used
# for the authenticated POST/DELETE writes; ALWAYS sends the bearer token.
http_body() {
  hb_method="$1"; hb_path="$2"; hb_body="$3"; hb_name="$4"
  hb_body_file="${RESULTS_DIR}/${hb_name}.json"
  hb_status_file="${RESULTS_DIR}/${hb_name}.status"

  set -- -sS -o "${hb_body_file}" -w '%{http_code}' -X "${hb_method}" \
    -H 'Accept: application/json' -H 'Content-Type: application/json'
  if [ -n "${TOKEN}" ]; then
    set -- "$@" -H "Authorization: Bearer ${TOKEN}"
  fi
  if [ -n "${hb_body}" ]; then
    set -- "$@" --data "${hb_body}"
  fi
  set -- "$@" "${BASE_URL}${hb_path}"

  hb_code="$(curl "$@" 2>"${RESULTS_DIR}/${hb_name}.curlerr")"
  hb_rc=$?
  if [ "${hb_rc}" -ne 0 ]; then
    printf '000' > "${hb_status_file}"
    printf '000'
    return 0
  fi
  printf '%s' "${hb_code}" > "${hb_status_file}"
  printf '%s' "${hb_code}"
}

# json_field <file> <dotted-key> -> the scalar value (or empty)
json_field() {
  jf_file="$1"; jf_key="$2"
  python3 - "${jf_file}" "${jf_key}" <<'PY' 2>/dev/null
import sys, json
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    sys.exit(0)
cur = d
for part in sys.argv[2].split("."):
    if isinstance(cur, dict) and part in cur:
        cur = cur[part]
    else:
        sys.exit(0)
if cur is not None:
    print(cur)
PY
}

# first_browse_id <file> -> the numeric id of the first browse/list item, or empty
first_browse_id() {
  fb_file="$1"
  python3 - "${fb_file}" <<'PY' 2>/dev/null
import sys, json
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    sys.exit(0)
items = d.get("items") if isinstance(d, dict) else d
if isinstance(items, list):
    for it in items:
        if isinstance(it, dict) and it.get("id") is not None:
            print(it["id"]); break
PY
}

# fav_pair_present <file> <entity_type> <entity_id> -> exit 0 iff the favorites
# list body contains an item with that (entity_type, entity_id) pair.
fav_pair_present() {
  fp_file="$1"; fp_type="$2"; fp_id="$3"
  python3 - "${fp_file}" "${fp_type}" "${fp_id}" <<'PY'
import sys, json
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    sys.exit(1)
want_type = sys.argv[2]
try:
    want_id = int(sys.argv[3])
except ValueError:
    sys.exit(1)
items = (d.get("favorites") if isinstance(d, dict) else d) or []
if isinstance(items, list):
    for it in items:
        if not isinstance(it, dict):
            continue
        if it.get("entity_type") == want_type and it.get("entity_id") == want_id:
            sys.exit(0)
sys.exit(1)
PY
}

echo "=============================================================="
echo " Catalogizer playback-progress + favorites guard (resume + fav)"
echo " base_url    : ${BASE_URL}"
echo " browse_type : ${BROWSE_TYPE}"
echo " resume_pos  : ${RESUME_POS}"
echo " results     : ${RESULTS_DIR}"
echo " env_file    : ${ENV_FILE:-<none>}"
echo " python3     : ${HAVE_PY}"
echo " started     : ${TS}"
echo "=============================================================="

emit_summary() {
  es_extra_json="$1"
  {
    printf 'PASS=%s FAIL=%s SKIP=%s\n' "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}"
    printf '%b' "${SUMMARY_ROWS}"
  } > "${SUMMARY_TXT}"
  printf '{"pass":%s,"fail":%s,"skip":%s,"base_url":"%s","results_dir":"%s","ts":"%s","guard":"playback-progress-resume+favorites"%s}\n' \
    "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" "${BASE_URL}" "${RESULTS_DIR}" "${TS}" "${es_extra_json}" \
    > "${SUMMARY_JSON}"
}

# skip_all <reason> — SKIP every assertion in the current shell (counters
# increment here, never in a subshell — mirrors the reference for-loop style).
skip_all() {
  sa_reason="$1"
  for lbl in \
    "PPF-A resume-where-left-off (progress.last_position == written end_position)" \
    "PPF-B favorite-add-then-present (appears in list + is_favorite:true)" \
    "PPF-C favorite-remove-then-gone (absent from list + is_favorite:false)" \
    "PPF-D progress-determinism (last_position stable across two reads)"; do
    ab_skip_with_reason "${lbl}" "${sa_reason}"
  done
}

# ------------------------------------------------------------------------------
# Cleanup (§11.4.14) — undo EVERY write the suite made, leave the target
# quiescent. The favorite is removed; the playback progress is reset to the
# pristine "not started" baseline (last_position=0) by ending one final session
# at position 0 (the API exposes no DELETE for media_progress, so reset-to-
# baseline is the honest quiescent state). Cleanup output goes to a log file so
# it never pollutes the PASS/FAIL stream; it best-effort and never FAILs the run.
# ------------------------------------------------------------------------------
CLEANUP_LOG="${RESULTS_DIR}/_cleanup.log"
cleanup() {
  {
    if [ -n "${TOKEN}" ]; then
      if [ "${FAV_ADDED}" -eq 1 ] && [ -n "${ENTITY_ID}" ] && [ -n "${ENTITY_TYPE}" ]; then
        http_body DELETE "/api/v1/favorites/${ENTITY_TYPE}/${ENTITY_ID}" "" _cleanup_fav_remove >/dev/null 2>&1 || true
      fi
      if [ "${PROG_WRITTEN}" -eq 1 ] && [ -n "${ENTITY_ID}" ]; then
        # Reset progress to baseline: open+end a session at position 0 so the
        # rolled-up media_progress.last_position returns to the not-started value.
        rcode="$(http_body POST /api/v1/playback/sessions/start \
          "{\"media_item_id\":${ENTITY_ID},\"position_unit\":\"seconds\",\"start_position\":0}" \
          _cleanup_sess_start 2>/dev/null)"
        if [ "${rcode}" = "200" ]; then
          rsid="$(json_field "${RESULTS_DIR}/_cleanup_sess_start.json" session_id)"
          if [ -n "${rsid}" ]; then
            http_body POST /api/v1/playback/sessions/end \
              "{\"session_id\":${rsid},\"end_position\":0,\"total_amount\":0,\"completed\":false}" \
              _cleanup_sess_end >/dev/null 2>&1 || true
          fi
        fi
      fi
    fi
    rm -f "${RESULTS_DIR}"/.scratch.* 2>/dev/null || true
  } > "${CLEANUP_LOG}" 2>&1 || true
}
trap cleanup EXIT INT TERM

if [ "${HAVE_PY}" -eq 0 ]; then
  echo
  echo "python3 is REQUIRED as the JSON oracle (no tautology fallback)."
  skip_all "python3 absent: cannot parse JSON evidence without it (§11.4.6)"
  emit_summary ',"reason":"python3_absent"'
  echo "Summary: PASS=${PASS_COUNT} FAIL=${FAIL_COUNT} SKIP=${SKIP_COUNT}"
  exit 0
fi

# ------------------------------------------------------------------------------
# Pre-flight reachability probe (§11.4.3 topology dispatch)
# ------------------------------------------------------------------------------
PROBE_CODE="$(http_get /health no _preflight_health)"
if [ "${PROBE_CODE}" = "000" ]; then
  echo
  echo "API at ${BASE_URL} is UNREACHABLE (curl transport failure)."
  echo "Marking every assertion SKIP-with-reason (§11.4.3)."
  skip_all "network_unreachable: ${BASE_URL} not reachable"
  emit_summary ',"unreachable":true'
  echo "Summary: PASS=${PASS_COUNT} FAIL=${FAIL_COUNT} SKIP=${SKIP_COUNT}"
  exit 0
fi

# ------------------------------------------------------------------------------
# Acquire a real session token if one was not supplied (§11.4.10: never echoed).
# ------------------------------------------------------------------------------
if [ -z "${TOKEN}" ]; then
  echo
  echo "--- Acquiring real session token via POST /api/v1/auth/login ---"
  LOGIN_BODY="{\"username\":\"${USER}\",\"password\":\"${PASS}\"}"
  curl -sS -o "${RESULTS_DIR}/_login.json" -w '%{http_code}' \
    -X POST -H 'Content-Type: application/json' -H 'Accept: application/json' \
    --data "${LOGIN_BODY}" "${BASE_URL}/api/v1/auth/login" \
    > "${RESULTS_DIR}/_login.status" 2>"${RESULTS_DIR}/_login.curlerr" || true
  LOGIN_CODE="$(cat "${RESULTS_DIR}/_login.status" 2>/dev/null)"
  if [ "${LOGIN_CODE}" = "200" ]; then
    TOKEN="$(json_field "${RESULTS_DIR}/_login.json" session_token)"
  fi
fi

if [ -z "${TOKEN}" ]; then
  echo
  echo "No auth token available — SKIPPING all auth-required assertions (§11.4.3)."
  skip_all "no_token: login did not yield a session_token"
  emit_summary ',"no_token":true'
  echo "Summary: PASS=${PASS_COUNT} FAIL=${FAIL_COUNT} SKIP=${SKIP_COUNT}"
  exit 0
fi
echo "Auth token: acquired (value not printed, §11.4.10)."

# ------------------------------------------------------------------------------
# Resolve a real fixture entity: its id (favorite entity_id + playback
# media_item_id) and its media_type (favorite entity_type). Shared by all four
# assertions. An explicit CATALOGIZER_ENTITY_ID skips browse discovery.
# ------------------------------------------------------------------------------
echo
if [ -z "${ENTITY_ID}" ]; then
  echo "--- Resolving a fixture entity via GET /api/v1/entities/browse/${BROWSE_TYPE} ---"
  CODE="$(http_get "/api/v1/entities/browse/${BROWSE_TYPE}?limit=1" yes ppf_browse)"
  EV_BROWSE="${RESULTS_DIR}/ppf_browse.json"
  if [ "${CODE}" != "200" ]; then
    ab_fail "PPF resolve: browse/${BROWSE_TYPE} expected 200, got ${CODE}" "${EV_BROWSE}"
  else
    ENTITY_ID="$(first_browse_id "${EV_BROWSE}")"
  fi
fi

if [ -n "${ENTITY_ID}" ]; then
  echo "--- Resolving entity ${ENTITY_ID} detail (media_type) ---"
  CODE="$(http_get "/api/v1/entities/${ENTITY_ID}" yes ppf_entity_detail)"
  EV_DET="${RESULTS_DIR}/ppf_entity_detail.json"
  if [ "${CODE}" = "200" ]; then
    ENTITY_TYPE="$(json_field "${EV_DET}" media_type)"
  fi
  # Fall back to the browse type if detail did not surface a media_type.
  if [ -z "${ENTITY_TYPE}" ]; then
    ENTITY_TYPE="${BROWSE_TYPE}"
  fi
  echo "Fixture: entity id=${ENTITY_ID}, entity_type=${ENTITY_TYPE}"
fi

NO_FIXTURE=0
if [ -z "${ENTITY_ID}" ] || [ -z "${ENTITY_TYPE}" ]; then
  NO_FIXTURE=1
fi

# ------------------------------------------------------------------------------
# (A) PPF-A — resume-where-left-off: run start->progress->end on a real entity,
#     then read GET /entities/{id}/progress and assert last_position == RESUME_POS.
# ------------------------------------------------------------------------------
echo
echo "--- PPF-A (resume): write a playback session, read back last_position ---"
if [ "${NO_FIXTURE}" -eq 1 ]; then
  ab_skip_with_reason "PPF-A resume-where-left-off (progress.last_position == written end_position)" \
    "no_fixture: could not resolve a ${BROWSE_TYPE} entity id+type to drive"
else
  SCODE="$(http_body POST /api/v1/playback/sessions/start \
    "{\"media_item_id\":${ENTITY_ID},\"position_unit\":\"seconds\",\"start_position\":0}" \
    ppf_a_start)"
  EV_START="${RESULTS_DIR}/ppf_a_start.json"
  SESSION_ID=""
  if [ "${SCODE}" = "200" ]; then
    SESSION_ID="$(json_field "${EV_START}" session_id)"
  fi
  if [ -z "${SESSION_ID}" ]; then
    ab_fail "PPF-A resume: start session did not yield a session_id (status ${SCODE})" "${EV_START}"
  else
    PROG_WRITTEN=1
    # Bump progress, then end at RESUME_POS so media_progress.last_position is set.
    http_body POST /api/v1/playback/sessions/progress \
      "{\"session_id\":${SESSION_ID},\"end_position\":${RESUME_POS},\"total_amount\":${RESUME_POS}}" \
      ppf_a_progress >/dev/null
    ECODE="$(http_body POST /api/v1/playback/sessions/end \
      "{\"session_id\":${SESSION_ID},\"end_position\":${RESUME_POS},\"total_amount\":${RESUME_POS},\"completed\":false}" \
      ppf_a_end)"
    if [ "${ECODE}" != "200" ]; then
      ab_fail "PPF-A resume: end session expected 200, got ${ECODE}" "${RESULTS_DIR}/ppf_a_end.json"
    else
      RCODE="$(http_get "/api/v1/entities/${ENTITY_ID}/progress" yes ppf_a_progress_read)"
      EV_PROG="${RESULTS_DIR}/ppf_a_progress_read.json"
      if [ "${RCODE}" != "200" ]; then
        ab_fail "PPF-A resume: GET progress expected 200, got ${RCODE}" "${EV_PROG}"
      else
        LASTPOS="$(json_field "${EV_PROG}" progress.last_position)"
        if [ -z "${LASTPOS}" ]; then
          ab_fail "PPF-A resume: progress.last_position is null/absent after a real end() at ${RESUME_POS} — resume position NOT recorded" "${EV_PROG}"
        elif [ "${LASTPOS}" = "${RESUME_POS}" ]; then
          ab_pass_with_evidence "PPF-A resume-where-left-off: media_progress.last_position == ${RESUME_POS} (exactly the end_position written) — client can resume there" "${EV_PROG}"
        else
          ab_fail "PPF-A resume: read-back last_position=${LASTPOS} != written end_position=${RESUME_POS} — resume position WRONG" "${EV_PROG}"
        fi
      fi
    fi
  fi
fi

# ------------------------------------------------------------------------------
# (B) PPF-B — favorite add -> present: POST /favorites, then it appears in the
#     list AND check reports is_favorite:true.
# ------------------------------------------------------------------------------
echo
echo "--- PPF-B (favorite add): add a favorite, assert it is observably present ---"
if [ "${NO_FIXTURE}" -eq 1 ]; then
  ab_skip_with_reason "PPF-B favorite-add-then-present (appears in list + is_favorite:true)" \
    "no_fixture: could not resolve a ${BROWSE_TYPE} entity id+type to favorite"
else
  ACODE="$(http_body POST /api/v1/favorites \
    "{\"entity_id\":${ENTITY_ID},\"entity_type\":\"${ENTITY_TYPE}\"}" ppf_b_add)"
  EV_ADD="${RESULTS_DIR}/ppf_b_add.json"
  # 200 (added) or 409 (already present from a prior interrupted run) both mean
  # the favorite is now present; either way we own the row and must remove it.
  if [ "${ACODE}" = "200" ] || [ "${ACODE}" = "409" ]; then
    FAV_ADDED=1
    LCODE="$(http_get "/api/v1/favorites?limit=200" yes ppf_b_list)"
    EV_LIST="${RESULTS_DIR}/ppf_b_list.json"
    CCODE="$(http_get "/api/v1/favorites/check/${ENTITY_TYPE}/${ENTITY_ID}" yes ppf_b_check)"
    EV_CHECK="${RESULTS_DIR}/ppf_b_check.json"
    IS_FAV="$(json_field "${EV_CHECK}" is_favorite)"
    IN_LIST=no
    if fav_pair_present "${EV_LIST}" "${ENTITY_TYPE}" "${ENTITY_ID}"; then
      IN_LIST=yes
    fi
    IS_FAV_TRUE=no
    if [ "${IS_FAV}" = "True" ] || [ "${IS_FAV}" = "true" ]; then
      IS_FAV_TRUE=yes
    fi
    if [ "${LCODE}" != "200" ] || [ "${CCODE}" != "200" ]; then
      ab_fail "PPF-B favorite-add: list/check non-200 (list=${LCODE} check=${CCODE})" "${EV_LIST}"
    elif [ "${IN_LIST}" = "yes" ] && [ "${IS_FAV_TRUE}" = "yes" ]; then
      ab_pass_with_evidence "PPF-B favorite-add-then-present: (${ENTITY_TYPE},${ENTITY_ID}) appears in GET /favorites AND is_favorite=true" "${EV_LIST}"
    else
      ab_fail "PPF-B favorite-add: added (${ENTITY_TYPE},${ENTITY_ID}) NOT observably present (in_list=${IN_LIST} is_favorite=${IS_FAV})" "${EV_LIST}"
    fi
  else
    ab_fail "PPF-B favorite-add: POST /favorites expected 200/409, got ${ACODE}" "${EV_ADD}"
  fi
fi

# ------------------------------------------------------------------------------
# (C) PPF-C — favorite remove -> gone: DELETE the favorite, then it is absent
#     from the list AND check reports is_favorite:false.
# ------------------------------------------------------------------------------
echo
echo "--- PPF-C (favorite remove): remove the favorite, assert it is gone ---"
if [ "${NO_FIXTURE}" -eq 1 ]; then
  ab_skip_with_reason "PPF-C favorite-remove-then-gone (absent from list + is_favorite:false)" \
    "no_fixture: could not resolve a ${BROWSE_TYPE} entity id+type to remove"
elif [ "${FAV_ADDED}" -ne 1 ]; then
  ab_skip_with_reason "PPF-C favorite-remove-then-gone (absent from list + is_favorite:false)" \
    "no_favorite_added: PPF-B did not add a favorite to remove"
else
  DCODE="$(http_body DELETE "/api/v1/favorites/${ENTITY_TYPE}/${ENTITY_ID}" "" ppf_c_remove)"
  EV_REMOVE="${RESULTS_DIR}/ppf_c_remove.json"
  if [ "${DCODE}" != "200" ]; then
    ab_fail "PPF-C favorite-remove: DELETE expected 200, got ${DCODE}" "${EV_REMOVE}"
  else
    FAV_ADDED=0
    LCODE="$(http_get "/api/v1/favorites?limit=200" yes ppf_c_list)"
    EV_LIST2="${RESULTS_DIR}/ppf_c_list.json"
    CCODE="$(http_get "/api/v1/favorites/check/${ENTITY_TYPE}/${ENTITY_ID}" yes ppf_c_check)"
    EV_CHECK2="${RESULTS_DIR}/ppf_c_check.json"
    IS_FAV2="$(json_field "${EV_CHECK2}" is_favorite)"
    if [ "${LCODE}" != "200" ] || [ "${CCODE}" != "200" ]; then
      ab_fail "PPF-C favorite-remove: list/check non-200 (list=${LCODE} check=${CCODE})" "${EV_LIST2}"
    elif fav_pair_present "${EV_LIST2}" "${ENTITY_TYPE}" "${ENTITY_ID}"; then
      ab_fail "PPF-C favorite-remove: (${ENTITY_TYPE},${ENTITY_ID}) STILL present in GET /favorites after DELETE" "${EV_LIST2}"
    elif [ "${IS_FAV2}" = "False" ] || [ "${IS_FAV2}" = "false" ]; then
      ab_pass_with_evidence "PPF-C favorite-remove-then-gone: (${ENTITY_TYPE},${ENTITY_ID}) absent from GET /favorites AND is_favorite=false" "${EV_CHECK2}"
    else
      ab_fail "PPF-C favorite-remove: is_favorite still reports '${IS_FAV2}' after DELETE" "${EV_CHECK2}"
    fi
  fi
fi

# ------------------------------------------------------------------------------
# (D) PPF-D — progress determinism (§11.4.50): two independent reads of
#     GET /entities/{id}/progress observe the SAME last_position.
# ------------------------------------------------------------------------------
echo
echo "--- PPF-D (determinism §11.4.50): last_position stable across two reads ---"
if [ "${NO_FIXTURE}" -eq 1 ]; then
  ab_skip_with_reason "PPF-D progress-determinism (last_position stable across two reads)" \
    "no_fixture: could not resolve a ${BROWSE_TYPE} entity id"
elif [ "${PROG_WRITTEN}" -ne 1 ]; then
  ab_skip_with_reason "PPF-D progress-determinism (last_position stable across two reads)" \
    "no_progress_written: PPF-A did not write a playback session"
else
  http_get "/api/v1/entities/${ENTITY_ID}/progress" yes ppf_d_read1 >/dev/null
  http_get "/api/v1/entities/${ENTITY_ID}/progress" yes ppf_d_read2 >/dev/null
  EV_R1="${RESULTS_DIR}/ppf_d_read1.json"
  EV_R2="${RESULTS_DIR}/ppf_d_read2.json"
  POS1="$(json_field "${EV_R1}" progress.last_position)"
  POS2="$(json_field "${EV_R2}" progress.last_position)"
  if [ -z "${POS1}" ] || [ -z "${POS2}" ]; then
    ab_fail "PPF-D determinism: a read returned null last_position (read1='${POS1}' read2='${POS2}')" "${EV_R1}"
  elif [ "${POS1}" = "${POS2}" ]; then
    ab_pass_with_evidence "PPF-D progress-determinism: last_position identical across two reads (read1=${POS1}, read2=${POS2})" "${EV_R1}"
  else
    ab_fail "PPF-D determinism: last_position not stable (read1=${POS1}, read2=${POS2})" "${EV_R1}"
  fi
fi

# ------------------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------------------
{
  printf 'PASS=%s FAIL=%s SKIP=%s\n' "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}"
  printf '%b' "${SUMMARY_ROWS}"
} > "${SUMMARY_TXT}"

printf '{"pass":%s,"fail":%s,"skip":%s,"base_url":"%s","results_dir":"%s","ts":"%s","entity_id":"%s","entity_type":"%s","resume_pos":"%s","guard":"playback-progress-resume+favorites"}\n' \
  "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" "${BASE_URL}" "${RESULTS_DIR}" "${TS}" "${ENTITY_ID}" "${ENTITY_TYPE}" "${RESUME_POS}" \
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
