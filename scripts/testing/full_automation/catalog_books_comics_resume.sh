#!/usr/bin/env bash
# ==============================================================================
# catalog_books_comics_resume.sh
# ------------------------------------------------------------------------------
# Purpose:
#   §11.4 full-automation API guard for the catalog-api BOOK + COMIC reading
#   surfaces. Proves, against the LIVE catalog-api over real HTTP, that:
#
#     (1) a BOOK and/or COMIC entity resolves via the type-browse endpoint;
#     (2) its chapters / pages list via the container-children endpoint (≥0,
#         honest if none seeded — a comic with a single embedded file may have
#         zero child entities, which is reported truthfully, never faked);
#     (3) reading progress (resume-where-left-off) works through the SAME
#         playback-session mechanism episodes and movies use — a reading
#         position is set (POST /playback/sessions/start + /progress, with
#         position_unit "pages") and then read back EQUAL via
#         GET /entities/{id}/progress -> progress.last_position.
#
#   Book/comic reading progress is NOT a separate subsystem: the playback
#   handler explicitly records "video, audio, book, comic, game" sessions, and
#   the per-entity rollup at GET /api/v1/entities/{id}/progress carries
#   progress.last_position + progress.position_unit. position_unit "pages" is
#   the reading-position dimension (vs "seconds" for AV). This suite drives that
#   exact path so a PASS proves the book/comic resume surface really works for
#   an end user, with captured-evidence JSON per assertion (anti-bluff §11.4 /
#   §11.4.69 — never a tautology).
#
#   POSITIVE / NEGATIVE assertions:
#     BCR-API-001 (entity resolves, POSITIVE): browse book and comic types; assert at
#        least one of them resolves ≥1 entity with a numeric id + a media_type
#        in {book, comic, comic_book}. If NEITHER type is seeded, SKIP-with-
#        reason (§11.4.3) — never a fake PASS, never a FAIL.
#     BCR-API-002 (chapters/pages via children, POSITIVE-or-honest-zero): for the
#        resolved entity, GET /entities/{id}/children returns 200 and a JSON
#        list; assert the list parses (a real array, ≥0 children). A non-200 or
#        non-array is a FAIL; a genuine empty list PASSes with the captured body
#        as evidence (honest zero, §11.4.6).
#     BCR-API-003 (resume round-trip, POSITIVE): read the entity's progress baseline,
#        set a reading position via a real session (start@page0 -> progress@pageN
#        with position_unit=pages), then read GET /entities/{id}/progress back
#        and assert progress.last_position EQUALS the position we wrote AND
#        progress.position_unit == "pages". The write-leg is opt-in
#        (CATALOGIZER_ALLOW_PLAYBACK_WRITE=1, conductor-gated — §11.4.119 the
#        conductor OWNS the live API+DB) since it appends a per-user session
#        row; when not enabled, the suite still PASSes the READ-only half by
#        asserting the progress endpoint returns a well-formed
#        {"progress": ...} envelope (200), and SKIPs the write round-trip with a
#        clear reason.
#     BCR-API-004 (determinism §11.4.50): the resolved-entity's children-count is
#        stable across two independent reads.
#
#   This is a §11.4.135 standing regression guard (book/comic reading surface)
#   and a §11.4.146 extend-to-all-cases pass (both book AND comic types, the
#   children sweep, the progress round-trip, the determinism re-read).
#
# Usage:
#   ./catalog_books_comics_resume.sh
#   CATALOGIZER_BASE_URL=http://127.0.0.1:8080 \
#   CATALOGIZER_ENV_FILE=/path/.api-env \
#   CATALOGIZER_ALLOW_PLAYBACK_WRITE=1 \
#   ./catalog_books_comics_resume.sh
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
#                           (default qa-results/books_comics_resume/<ts>)
#   CATALOGIZER_ALLOW_PLAYBACK_WRITE
#                           "1" enables the resume WRITE round-trip (a real
#                           per-user playback session is created — read-only by
#                           default per §11.4.119). Default: unset (read-only).
#   CATALOGIZER_READ_POSITION
#                           page number to write+read-back in BCR-API-003 (default 7).
#
# Outputs:
#   - Per-assertion captured-evidence JSON under the results dir (the real HTTP
#     response body). A summary.txt and machine-readable summary.json.
#   - PASS/FAIL/SKIP lines on stdout, each citing its evidence path.
#   - Exit code 0 iff every non-SKIP assertion PASSed; 1 otherwise.
#
# Side-effects:
#   - Network calls. By default ONLY read-only GETs (book/comic resolve, children,
#     progress read). The resume WRITE round-trip (BCR-API-003 write-leg) runs ONLY
#     when CATALOGIZER_ALLOW_PLAYBACK_WRITE=1; it appends a per-user playback
#     session row (NOT a scan / clear / aggregate — those are never invoked).
#     Evidence files written under the results dir. The sourced env file is
#     read-only. NEVER prints credentials (§11.4.10).
#
# Dependencies: bash, curl, python3 (JSON oracle). POSIX-portable beyond the bash
#   shebang; parses clean under both `sh -n` and `bash -n` (§11.4.67).
#
# Cross-references:
#   - docs/scripts/catalog_books_comics_resume.md              (companion, §11.4.18)
#   - submodules/helix_qa/banks/catalog_books_comics_resume.yaml (HelixQA bank)
#   - scripts/testing/full_automation/catalog_episode_titles_dedup.sh (model)
#
# Anti-bluff (§11.4 / §11.4.69 / §11.4.27): every PASS asserts a REAL observed
#   id / media_type / children-count / last_position value from a captured
#   response body, never a tautology, and cites its evidence file. Real system,
#   no mocks. Unreachable API / no auth / no book-or-comic seeded => honest
#   SKIP-with-reason (§11.4.3), never a fabricated PASS.
# ==============================================================================

set -u

# ------------------------------------------------------------------------------
# Configuration
# ------------------------------------------------------------------------------
BASE_URL="${CATALOGIZER_BASE_URL:-http://127.0.0.1:8080}"
ALLOW_WRITE="${CATALOGIZER_ALLOW_PLAYBACK_WRITE:-}"
READ_POSITION="${CATALOGIZER_READ_POSITION:-7}"

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
RESULTS_DIR="${CATALOGIZER_RESULTS_DIR:-qa-results/books_comics_resume/${TS}}"
mkdir -p "${RESULTS_DIR}"

SUMMARY_TXT="${RESULTS_DIR}/summary.txt"
SUMMARY_JSON="${RESULTS_DIR}/summary.json"
: > "${SUMMARY_TXT}"

PASS_COUNT=0
FAIL_COUNT=0
SKIP_COUNT=0
SUMMARY_ROWS=""

# python3 is the JSON oracle. Refuse to run a tautology if it is absent.
HAVE_PY=0
if command -v python3 >/dev/null 2>&1; then
  HAVE_PY=1
fi

# ------------------------------------------------------------------------------
# Cleanup (§11.4.14) — leave the working tree quiescent. No DESTRUCTIVE
# server-side state is created; the optional resume write-leg appends a per-user
# playback session row only (conductor-gated). We remove our own scratch files.
# ------------------------------------------------------------------------------
cleanup() {
  rm -f "${RESULTS_DIR}"/.scratch.* 2>/dev/null || true
}
trap cleanup EXIT INT TERM

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

# http_post_json <path> <json-body> <evidence_basename> -> echoes HTTP status
# (auth always sent when a TOKEN is present). Used ONLY by the gated write-leg.
http_post_json() {
  hp_path="$1"; hp_body="$2"; hp_name="$3"
  hp_body_file="${RESULTS_DIR}/${hp_name}.json"
  hp_status_file="${RESULTS_DIR}/${hp_name}.status"

  set -- -sS -o "${hp_body_file}" -w '%{http_code}' -X POST \
    -H 'Content-Type: application/json' -H 'Accept: application/json'
  if [ -n "${TOKEN}" ]; then
    set -- "$@" -H "Authorization: Bearer ${TOKEN}"
  fi
  set -- "$@" --data "${hp_body}" "${BASE_URL}${hp_path}"

  hp_code="$(curl "$@" 2>"${RESULTS_DIR}/${hp_name}.curlerr")"
  hp_rc=$?
  if [ "${hp_rc}" -ne 0 ]; then
    printf '000' > "${hp_status_file}"
    printf '000'
    return 0
  fi
  printf '%s' "${hp_code}" > "${hp_status_file}"
  printf '%s' "${hp_code}"
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

# first_entity <file> <comma-separated-acceptable-media-types>
#   -> "<id>\t<media_type>\t<title>" of the first item whose media_type is one
#      of the acceptable set AND that has a numeric id; empty if none.
first_entity() {
  fe_file="$1"; fe_types="$2"
  python3 - "${fe_file}" "${fe_types}" <<'PY' 2>/dev/null
import sys, json
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    sys.exit(0)
accept = set(t.strip() for t in sys.argv[2].split(",") if t.strip())
items = d.get("items") if isinstance(d, dict) else d
if isinstance(items, list):
    for it in items:
        if not isinstance(it, dict):
            continue
        mt = it.get("media_type")
        iid = it.get("id")
        if iid is None:
            continue
        # accept either an explicit acceptable media_type, or (empty accept set
        # never happens) any item — but here we require the type to match so the
        # PASS asserts a genuine book/comic, never an arbitrary entity.
        if mt in accept:
            print("%s\t%s\t%s" % (iid, mt, it.get("title", "")))
            break
PY
}

# children_count <file> -> integer count of array items (or -1 if not a list)
children_count() {
  cc_file="$1"
  python3 - "${cc_file}" <<'PY' 2>/dev/null
import sys, json
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    print(-1); sys.exit(0)
items = d.get("items") if isinstance(d, dict) else d
if isinstance(items, list):
    print(len(items))
else:
    print(-1)
PY
}

# progress_is_object <file> -> prints "object" if body is {"progress": {...}},
#   "null" if {"progress": null}, "absent" if the progress key is missing/other.
progress_shape() {
  ps_file="$1"
  python3 - "${ps_file}" <<'PY' 2>/dev/null
import sys, json
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    print("absent"); sys.exit(0)
if not isinstance(d, dict) or "progress" not in d:
    print("absent"); sys.exit(0)
p = d["progress"]
if p is None:
    print("null")
elif isinstance(p, dict):
    print("object")
else:
    print("absent")
PY
}

echo "=============================================================="
echo " Catalogizer BOOK + COMIC reading surface guard (resume + children)"
echo " base_url     : ${BASE_URL}"
echo " results      : ${RESULTS_DIR}"
echo " env_file     : ${ENV_FILE:-<none>}"
echo " python3      : ${HAVE_PY}"
echo " write-leg    : $( [ "${ALLOW_WRITE}" = "1" ] && echo ENABLED || echo "read-only (gated)")"
echo " read-position: ${READ_POSITION}"
echo " started      : ${TS}"
echo "=============================================================="

emit_summary() {
  es_extra_json="$1"
  {
    printf 'PASS=%s FAIL=%s SKIP=%s\n' "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}"
    printf '%b' "${SUMMARY_ROWS}"
  } > "${SUMMARY_TXT}"
  printf '{"pass":%s,"fail":%s,"skip":%s,"base_url":"%s","results_dir":"%s","ts":"%s","guard":"books-comics-reading-resume"%s}\n' \
    "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" "${BASE_URL}" "${RESULTS_DIR}" "${TS}" "${es_extra_json}" \
    > "${SUMMARY_JSON}"
}

# skip_all <reason> — SKIP every assertion in the current shell (counters
# increment here, never in a subshell — mirrors the reference for-loop style).
skip_all() {
  sa_reason="$1"
  for lbl in \
    "BCR-API-001 book-or-comic-entity-resolves" \
    "BCR-API-002 chapters-pages-via-children" \
    "BCR-API-003 reading-position-resume-roundtrip" \
    "BCR-API-004 children-count-determinism"; do
    ab_skip_with_reason "${lbl}" "${sa_reason}"
  done
}

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
# Resolve a BOOK or COMIC entity. Probe both the 'book' and 'comic' type-browse
# endpoints. A 400 means the type name is not provisioned in media_types (not a
# product defect of the reading surface) — treated as "type absent", not FAIL.
# We accept media_type in {book, comic, comic_book} on the resolved item.
# Shared by BCR-API-001 / BCR-API-002 / BCR-API-003 / BCR-API-004.
# ------------------------------------------------------------------------------
echo
echo "--- Resolving a book/comic entity via GET /api/v1/entities/browse/{book,comic} ---"

ENTITY_ID=""
ENTITY_MT=""
ENTITY_TITLE=""
RESOLVE_EVIDENCE=""
RESOLVE_NOTE=""

for typ in book comic; do
  CODE="$(http_get "/api/v1/entities/browse/${typ}" yes "bcr_browse_${typ}")"
  EV="${RESULTS_DIR}/bcr_browse_${typ}.json"
  if [ "${CODE}" = "400" ]; then
    RESOLVE_NOTE="${RESOLVE_NOTE}${typ}=type-not-provisioned(400); "
    continue
  fi
  if [ "${CODE}" != "200" ]; then
    RESOLVE_NOTE="${RESOLVE_NOTE}${typ}=http-${CODE}; "
    continue
  fi
  # Accept book OR comic OR comic_book media_type on the item.
  HIT="$(first_entity "${EV}" "book,comic,comic_book")"
  if [ -n "${HIT}" ]; then
    ENTITY_ID="$(printf '%s' "${HIT}" | cut -f1)"
    ENTITY_MT="$(printf '%s' "${HIT}" | cut -f2)"
    ENTITY_TITLE="$(printf '%s' "${HIT}" | cut -f3)"
    RESOLVE_EVIDENCE="${EV}"
    echo "Resolved ${typ} entity: id=${ENTITY_ID} media_type=${ENTITY_MT} title='${ENTITY_TITLE}'"
    break
  fi
  RESOLVE_NOTE="${RESOLVE_NOTE}${typ}=no-entities-seeded; "
done

# ------------------------------------------------------------------------------
# (A) BCR-API-001 — a book or comic entity resolves with a numeric id + media_type.
# ------------------------------------------------------------------------------
echo
echo "--- BCR-API-001: a book/comic entity resolves ---"
if [ -z "${ENTITY_ID}" ]; then
  ab_skip_with_reason "BCR-API-001 book-or-comic-entity-resolves" \
    "no_book_or_comic_seeded: ${RESOLVE_NOTE:-no entities}"
else
  ab_pass_with_evidence \
    "BCR-API-001 book-or-comic-entity-resolves: entity id=${ENTITY_ID} media_type=${ENTITY_MT} title='${ENTITY_TITLE}'" \
    "${RESOLVE_EVIDENCE}"
fi

# ------------------------------------------------------------------------------
# (B) BCR-API-002 — chapters / pages list via container children.
# ------------------------------------------------------------------------------
echo
echo "--- BCR-API-002: chapters/pages list via GET /api/v1/entities/{id}/children ---"
CHILDREN_COUNT=""
if [ -z "${ENTITY_ID}" ]; then
  ab_skip_with_reason "BCR-API-002 chapters-pages-via-children" \
    "no_entity_id: no book/comic resolved"
else
  CODE="$(http_get "/api/v1/entities/${ENTITY_ID}/children" yes bcr_children)"
  EV_CH="${RESULTS_DIR}/bcr_children.json"
  if [ "${CODE}" != "200" ]; then
    ab_fail "BCR-API-002 chapters-pages-via-children: expected 200 from children, got ${CODE}" "${EV_CH}"
  else
    CHILDREN_COUNT="$(children_count "${EV_CH}")"
    if [ "${CHILDREN_COUNT}" = "-1" ]; then
      ab_fail "BCR-API-002 chapters-pages-via-children: children body is not a JSON list" "${EV_CH}"
    else
      ab_pass_with_evidence \
        "BCR-API-002 chapters-pages-via-children: children endpoint returned a JSON list of ${CHILDREN_COUNT} chapter/page entit$( [ "${CHILDREN_COUNT}" = "1" ] && echo y || echo ies) (honest zero accepted for single-file books/comics)" \
        "${EV_CH}"
    fi
  fi
fi

# ------------------------------------------------------------------------------
# (C) BCR-API-003 — reading-position resume round-trip (the core surface).
#   Read baseline -> (gated) set position via real session -> read back EQUAL.
# ------------------------------------------------------------------------------
echo
echo "--- BCR-API-003: reading-position resume round-trip (set -> read back equal) ---"
if [ -z "${ENTITY_ID}" ]; then
  ab_skip_with_reason "BCR-API-003 reading-position-resume-roundtrip" \
    "no_entity_id: no book/comic resolved"
else
  # Baseline read of the progress envelope (always read-only).
  CODE="$(http_get "/api/v1/entities/${ENTITY_ID}/progress" yes bcr_progress_baseline)"
  EV_PB="${RESULTS_DIR}/bcr_progress_baseline.json"
  if [ "${CODE}" != "200" ]; then
    ab_fail "BCR-API-003 reading-position-resume-roundtrip: progress endpoint expected 200, got ${CODE}" "${EV_PB}"
  else
    BASE_SHAPE="$(progress_shape "${EV_PB}")"
    if [ "${BASE_SHAPE}" = "absent" ]; then
      ab_fail "BCR-API-003 reading-position-resume-roundtrip: progress body malformed (no {\"progress\": object|null} envelope)" "${EV_PB}"
    elif [ "${ALLOW_WRITE}" != "1" ]; then
      # Read-only mode (§11.4.119 conductor owns the DB): validate the progress
      # READ surface is well-formed, SKIP the write round-trip honestly.
      ab_pass_with_evidence \
        "BCR-API-003 reading-position-resume-roundtrip [read-only]: GET /entities/${ENTITY_ID}/progress returns a well-formed {\"progress\": ${BASE_SHAPE}} envelope (write round-trip gated behind CATALOGIZER_ALLOW_PLAYBACK_WRITE=1)" \
        "${EV_PB}"
      ab_skip_with_reason "BCR-API-003 reading-position-resume-roundtrip [write-leg]" \
        "playback_write_disabled: set CATALOGIZER_ALLOW_PLAYBACK_WRITE=1 (conductor-gated, §11.4.119) to exercise the set->read-back round-trip"
    else
      # Write round-trip: start a reading session, advance to READ_POSITION
      # pages, then read progress.last_position back and assert equality.
      START_BODY="{\"media_item_id\":${ENTITY_ID},\"position_unit\":\"pages\",\"start_position\":0}"
      SCODE="$(http_post_json /api/v1/playback/sessions/start "${START_BODY}" bcr_session_start)"
      EV_SS="${RESULTS_DIR}/bcr_session_start.json"
      SESSION_ID="$(json_field "${EV_SS}" session_id)"
      if [ "${SCODE}" != "200" ] || [ -z "${SESSION_ID}" ]; then
        ab_fail "BCR-API-003 reading-position-resume-roundtrip: session start failed (http ${SCODE}, session_id='${SESSION_ID}')" "${EV_SS}"
      else
        PROG_BODY="{\"session_id\":${SESSION_ID},\"end_position\":${READ_POSITION},\"total_amount\":${READ_POSITION}}"
        PCODE="$(http_post_json /api/v1/playback/sessions/progress "${PROG_BODY}" bcr_session_progress)"
        EV_SP="${RESULTS_DIR}/bcr_session_progress.json"
        if [ "${PCODE}" != "200" ]; then
          ab_fail "BCR-API-003 reading-position-resume-roundtrip: session progress write failed (http ${PCODE})" "${EV_SP}"
        else
          # Read the per-entity rollup back and assert last_position equals what
          # we wrote AND position_unit is "pages".
          http_get "/api/v1/entities/${ENTITY_ID}/progress" yes bcr_progress_after >/dev/null
          EV_PA="${RESULTS_DIR}/bcr_progress_after.json"
          GOT_POS="$(json_field "${EV_PA}" progress.last_position)"
          GOT_UNIT="$(json_field "${EV_PA}" progress.position_unit)"
          if [ "${GOT_POS}" = "${READ_POSITION}" ] && [ "${GOT_UNIT}" = "pages" ]; then
            ab_pass_with_evidence \
              "BCR-API-003 reading-position-resume-roundtrip: wrote page ${READ_POSITION} via a real session and read it back EQUAL (progress.last_position=${GOT_POS}, progress.position_unit=${GOT_UNIT}) — resume-where-left-off works" \
              "${EV_PA}"
          else
            ab_fail \
              "BCR-API-003 reading-position-resume-roundtrip: resume mismatch — wrote page ${READ_POSITION} (pages) but read back last_position='${GOT_POS}', position_unit='${GOT_UNIT}'" \
              "${EV_PA}"
          fi
        fi
      fi
    fi
  fi
fi

# ------------------------------------------------------------------------------
# (D) BCR-API-004 — children-count determinism (§11.4.50): stable across two reads.
# ------------------------------------------------------------------------------
echo
echo "--- BCR-API-004 (determinism §11.4.50): children-count stable across two reads ---"
if [ -z "${ENTITY_ID}" ]; then
  ab_skip_with_reason "BCR-API-004 children-count-determinism" \
    "no_entity_id: no book/comic resolved"
elif [ -z "${CHILDREN_COUNT}" ] || [ "${CHILDREN_COUNT}" = "-1" ]; then
  ab_skip_with_reason "BCR-API-004 children-count-determinism" \
    "no_valid_children_read: BCR-API-002 did not produce a parseable children count"
else
  http_get "/api/v1/entities/${ENTITY_ID}/children" yes bcr_children_read2 >/dev/null
  EV_CH2="${RESULTS_DIR}/bcr_children_read2.json"
  COUNT2="$(children_count "${EV_CH2}")"
  if [ "${CHILDREN_COUNT}" = "${COUNT2}" ]; then
    ab_pass_with_evidence \
      "BCR-API-004 children-count-determinism: children count stable across two reads (read1=${CHILDREN_COUNT}, read2=${COUNT2})" \
      "${EV_CH2}"
  else
    ab_fail \
      "BCR-API-004 children-count-determinism: children count NOT stable (read1=${CHILDREN_COUNT}, read2=${COUNT2})" \
      "${EV_CH2}"
  fi
fi

# ------------------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------------------
{
  printf 'PASS=%s FAIL=%s SKIP=%s\n' "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}"
  printf '%b' "${SUMMARY_ROWS}"
} > "${SUMMARY_TXT}"

printf '{"pass":%s,"fail":%s,"skip":%s,"base_url":"%s","results_dir":"%s","ts":"%s","entity_id":"%s","entity_media_type":"%s","guard":"books-comics-reading-resume"}\n' \
  "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" "${BASE_URL}" "${RESULTS_DIR}" "${TS}" "${ENTITY_ID}" "${ENTITY_MT}" \
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
