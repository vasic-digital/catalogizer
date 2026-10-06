#!/usr/bin/env bash
# ==============================================================================
# catalog_details_assets_caching.sh
# ------------------------------------------------------------------------------
# Purpose:
#   Standing full-automation API guard for the catalog-api media-DETAILS,
#   COVER/ASSET, and CACHING surfaces. Proves — over the LIVE catalog-api on real
#   HTTP — that each surface genuinely works for an end user, with captured
#   evidence for every assertion (anti-bluff §11.4 / §11.4.69 — never a tautology).
#
#   DETAILS surface (GET /api/v1/entities/:id — JSON entity detail):
#     a single media item resolves to a detail body with a NON-EMPTY title, a
#     numeric id, and a media_type field. A 404 / empty-title detail is a real
#     defect; this guard asserts the populated detail an end user would see.
#
#   COVER / ASSET surface (GET /api/v1/cover/:id, /cover/url/:id,
#   /cover/placeholder/:type — image bytes + cover-url JSON):
#     the cover endpoint returns IMAGE bytes with an image/* content-type and a
#     NON-DEGENERATE size (NOT 0 bytes, NOT a 1x1 stub) per §11.4.38
#     installable-asset evidence. A present-but-degenerate cover is a FAIL, never
#     a PASS. The placeholder SVG endpoint and the cover-url JSON endpoint are
#     asserted as the always-available fallback surfaces.
#
#   CACHING surface (HTTP cache headers on the entities group +
#   conditional-request 304):
#     the entities group is wrapped in CacheHeaders(300), so a GET detail carries
#     an ETag + Cache-Control: public, max-age=300. A SECOND fetch sending
#     If-None-Match:<that ETag> MUST be served from cache as 304 Not Modified
#     (the body is not re-sent). The ETag MUST be STABLE across two reads of the
#     same unchanged entity (§11.4.50 determinism). This is the directly-
#     observable HTTP caching behaviour backed by the media_metadata_cache layer.
#
#   POSITIVE / NEGATIVE assertions:
#     DAC-A (detail populated, POSITIVE): GET /entities/:id resolves to a detail
#        body whose title is non-empty, id is numeric, and media_type is present.
#     DAC-B (cover asset non-degenerate, POSITIVE + §11.4.38): GET /cover/:id
#        returns image/* bytes whose size exceeds a non-degenerate floor; a
#        0-byte / 1x1 cover FAILs.
#     DAC-C (cover-url JSON, POSITIVE): GET /cover/url/:id returns JSON carrying a
#        cover_url field for the same media id.
#     DAC-D (ETag present + stable, CACHING POSITIVE + §11.4.50): GET /entities/:id
#        carries an ETag + Cache-Control header, and the ETag is identical across
#        two independent reads of the unchanged entity.
#     DAC-E (conditional 304 served-from-cache, CACHING NEGATIVE-of-200): a second
#        GET /entities/:id with If-None-Match:<ETag> returns 304 Not Modified (the
#        cache short-circuit fires; the body is NOT re-served).
#     DAC-F (placeholder asset non-degenerate, POSITIVE + §11.4.38): GET
#        /cover/placeholder/<type> returns image/svg+xml bytes above the
#        non-degenerate floor — the always-available fallback asset.
#
#   This is a §11.4.135 standing regression guard wired to run every cycle, and a
#   §11.4.146 extend-to-all-cases pass (detail fields, the asset byte/content-type
#   census, the cover-url JSON, the ETag stability + 304 short-circuit, and the
#   placeholder fallback).
#
# Usage:
#   ./catalog_details_assets_caching.sh
#   CATALOGIZER_BASE_URL=http://127.0.0.1:8080 \
#   CATALOGIZER_ENV_FILE=/path/.api-env \
#   ./catalog_details_assets_caching.sh
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
#                           (default qa-results/details_assets_caching/<ts>)
#   CATALOGIZER_ENTITY_TYPE entity type to browse for a detail subject
#                           (default "movie"; falls back to tv_show then any)
#   CATALOGIZER_COVER_FLOOR non-degenerate cover-byte floor (default 64 bytes —
#                           a 1x1 PNG is ~67-100B; a placeholder SVG is >300B; a
#                           0-byte / truncated cover sits below the floor)
#
# Outputs:
#   - Per-assertion captured-evidence files under the results dir (the real HTTP
#     response body for JSON; for binary cover/asset bytes a sidecar
#     <name>.meta describing observed size + content-type, NOT the raw bytes if
#     large). A summary.txt and machine-readable summary.json.
#   - PASS/FAIL/SKIP lines on stdout, each citing its evidence path.
#   - Exit code 0 iff every non-SKIP assertion PASSed; 1 otherwise.
#
# Side-effects:
#   - Network calls (read-only GETs) to the configured API only. No writes, no
#     destructive state, no scans/clear/aggregate. Evidence files written under
#     the results dir. The sourced env file is read-only. NEVER prints
#     credentials (§11.4.10). Cleanup on every exit path (§11.4.14).
#
# Dependencies: bash, curl, python3 (JSON parsing). POSIX-portable beyond the
#   bash shebang; parses clean under both `sh -n` and `bash -n` (§11.4.67).
#
# Cross-references:
#   - docs/scripts/catalog_details_assets_caching.md  (companion guide, §11.4.18)
#   - submodules/helix_qa/banks/catalog_details_assets_caching.yaml (HelixQA bank)
#   - scripts/testing/full_automation/catalog_episode_titles_dedup.sh (model)
#
# Anti-bluff (§11.4 / §11.4.38 / §11.4.69 / §11.4.27): every PASS asserts a REAL
#   observed value from a captured response (title / id / image content-type +
#   byte size / ETag / 304 status), never a tautology, and cites its evidence
#   file. Real system, no mocks. An unreachable API => SKIP-with-reason
#   (§11.4.3), never a fabricated PASS. A present-but-degenerate asset (0 bytes /
#   1x1) is a FAIL, never a PASS (§11.4.38).
# ==============================================================================

set -u

# ------------------------------------------------------------------------------
# Configuration
# ------------------------------------------------------------------------------
BASE_URL="${CATALOGIZER_BASE_URL:-http://127.0.0.1:8080}"
ENTITY_TYPE="${CATALOGIZER_ENTITY_TYPE:-movie}"
# Non-degenerate cover-byte floor (§11.4.38). 64 bytes rejects a 0-byte/truncated
# cover while accepting a real raster (a 1x1 PNG ~67B is itself near-degenerate,
# but the floor is the byte gate; the content-type gate rejects non-image bodies).
COVER_FLOOR="${CATALOGIZER_COVER_FLOOR:-64}"

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
RESULTS_DIR="${CATALOGIZER_RESULTS_DIR:-qa-results/details_assets_caching/${TS}}"
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
# Cleanup (§11.4.14) — leave the working tree quiescent. No server-side state is
# created (read-only suite); we only remove our own transient scratch files.
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
# Writes the response BODY to <name>.json and the status to <name>.status.
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

# http_get_binary <path> <auth:yes|no> <evidence_basename> -> echoes
# "<http_code>|<size_bytes>|<content_type>". Writes the raw body to
# <name>.bin and a human-readable <name>.meta sidecar describing the observed
# size + content-type (the captured evidence for an asset assertion — §11.4.38).
http_get_binary() {
  hb_path="$1"; hb_auth="$2"; hb_name="$3"
  hb_body_file="${RESULTS_DIR}/${hb_name}.bin"
  hb_meta_file="${RESULTS_DIR}/${hb_name}.meta"

  set -- -sS -o "${hb_body_file}" \
    -w '%{http_code}|%{size_download}|%{content_type}' \
    -X GET -H 'Accept: image/*'
  if [ "${hb_auth}" = "yes" ] && [ -n "${TOKEN}" ]; then
    set -- "$@" -H "Authorization: Bearer ${TOKEN}"
  fi
  set -- "$@" "${BASE_URL}${hb_path}"

  hb_out="$(curl "$@" 2>"${RESULTS_DIR}/${hb_name}.curlerr")"
  hb_rc=$?
  if [ "${hb_rc}" -ne 0 ]; then
    printf 'request=%s\nresult=transport_failure\n' "${hb_path}" > "${hb_meta_file}"
    printf '000|0|none'
    return 0
  fi
  hb_code="$(printf '%s' "${hb_out}" | cut -d'|' -f1)"
  hb_size="$(printf '%s' "${hb_out}" | cut -d'|' -f2)"
  hb_ctype="$(printf '%s' "${hb_out}" | cut -d'|' -f3)"
  {
    printf 'request=%s\n' "${hb_path}"
    printf 'http_code=%s\n' "${hb_code}"
    printf 'size_bytes=%s\n' "${hb_size}"
    printf 'content_type=%s\n' "${hb_ctype}"
  } > "${hb_meta_file}"
  printf '%s|%s|%s' "${hb_code}" "${hb_size}" "${hb_ctype}"
}

# http_get_header <path> <header-name> <evidence_basename> [if-none-match]
# -> echoes "<http_code>|<header_value>". Writes a <name>.hdr sidecar with the
# full response headers (captured evidence for the caching assertions).
http_get_header() {
  gh_path="$1"; gh_header="$2"; gh_name="$3"; gh_inm="${4:-}"
  gh_hdr_file="${RESULTS_DIR}/${gh_name}.hdr"

  set -- -sS -D "${gh_hdr_file}" -o /dev/null -w '%{http_code}' \
    -X GET -H 'Accept: application/json'
  if [ -n "${TOKEN}" ]; then
    set -- "$@" -H "Authorization: Bearer ${TOKEN}"
  fi
  if [ -n "${gh_inm}" ]; then
    set -- "$@" -H "If-None-Match: ${gh_inm}"
  fi
  set -- "$@" "${BASE_URL}${gh_path}"

  gh_code="$(curl "$@" 2>"${RESULTS_DIR}/${gh_name}.curlerr")"
  gh_rc=$?
  if [ "${gh_rc}" -ne 0 ]; then
    printf '000|'
    return 0
  fi
  # Extract the requested header value (case-insensitive), last occurrence wins.
  gh_val="$(grep -i "^${gh_header}:" "${gh_hdr_file}" 2>/dev/null \
    | tail -n 1 | sed -E "s/^[^:]*:[[:space:]]*//; s/[[:space:]]*\r?$//")"
  printf '%s|%s' "${gh_code}" "${gh_val}"
}

# json_titles <file> -> newline-separated list of item titles in a browse/list body
json_titles() {
  jt_file="$1"
  python3 - "${jt_file}" <<'PY' 2>/dev/null
import sys, json
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    sys.exit(0)
items = d.get("items") if isinstance(d, dict) else d
if isinstance(items, list):
    for it in items:
        if isinstance(it, dict):
            t = it.get("title")
            if t is not None:
                print(t)
PY
}

# first_entity_id <file> -> the numeric id of the first item carrying a non-empty
# title (or the first with an id), or empty.
first_entity_id() {
  fe_file="$1"
  python3 - "${fe_file}" <<'PY' 2>/dev/null
import sys, json
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    sys.exit(0)
items = d.get("items") if isinstance(d, dict) else d
if isinstance(items, list):
    # Prefer an item with a non-empty title; else first item with an id.
    for it in items:
        if isinstance(it, dict) and it.get("id") is not None and (it.get("title") or "").strip():
            print(it["id"]); sys.exit(0)
    for it in items:
        if isinstance(it, dict) and it.get("id") is not None:
            print(it["id"]); sys.exit(0)
PY
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

# json_has_key <file> <key> -> exit 0 if the top-level object has the key
json_has_key() {
  jh_file="$1"; jh_key="$2"
  python3 - "${jh_file}" "${jh_key}" <<'PY' 2>/dev/null
import sys, json
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    sys.exit(1)
sys.exit(0 if isinstance(d, dict) and sys.argv[2] in d else 1)
PY
}

echo "=============================================================="
echo " Catalogizer details + cover/asset + caching guard"
echo " base_url    : ${BASE_URL}"
echo " entity_type : ${ENTITY_TYPE}"
echo " cover_floor : ${COVER_FLOOR} bytes (§11.4.38 non-degenerate)"
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
  printf '{"pass":%s,"fail":%s,"skip":%s,"base_url":"%s","results_dir":"%s","ts":"%s","guard":"details-assets-caching"%s}\n' \
    "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" "${BASE_URL}" "${RESULTS_DIR}" "${TS}" "${es_extra_json}" \
    > "${SUMMARY_JSON}"
}

# skip_all <reason> — SKIP every assertion in the current shell (counters
# increment here, never in a subshell — mirrors the reference for-loop style).
skip_all() {
  sa_reason="$1"
  for lbl in \
    "DAC-A entity-detail-populated" \
    "DAC-B cover-asset-non-degenerate" \
    "DAC-C cover-url-json" \
    "DAC-D etag-present-and-stable" \
    "DAC-E conditional-304-served-from-cache" \
    "DAC-F placeholder-asset-non-degenerate"; do
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
# Note: the /cover/* endpoints are public, but /entities/* requires auth.
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
# Resolve a real entity id to drive the detail + cover + caching surfaces.
# Browse the requested type; fall back to tv_show, then list-any. Shared subject.
# ------------------------------------------------------------------------------
echo
echo "--- Resolving a detail subject via GET /api/v1/entities/browse/${ENTITY_TYPE} ---"
SUBJECT_ID=""
SUBJECT_SRC=""
for try_type in "${ENTITY_TYPE}" tv_show movie; do
  CODE="$(http_get "/api/v1/entities/browse/${try_type}" yes "dac_browse_${try_type}")"
  EV_BR="${RESULTS_DIR}/dac_browse_${try_type}.json"
  [ "${CODE}" = "200" ] || continue
  CAND="$(first_entity_id "${EV_BR}")"
  if [ -n "${CAND}" ]; then
    SUBJECT_ID="${CAND}"
    SUBJECT_SRC="browse/${try_type}"
    break
  fi
done
# Last-resort: the unfiltered entities list.
if [ -z "${SUBJECT_ID}" ]; then
  CODE="$(http_get "/api/v1/entities?limit=25" yes dac_list_any)"
  EV_ANY="${RESULTS_DIR}/dac_list_any.json"
  if [ "${CODE}" = "200" ]; then
    CAND="$(first_entity_id "${EV_ANY}")"
    if [ -n "${CAND}" ]; then
      SUBJECT_ID="${CAND}"
      SUBJECT_SRC="entities-list"
    fi
  fi
fi

if [ -n "${SUBJECT_ID}" ]; then
  echo "Resolved detail subject -> entity id=${SUBJECT_ID} (via ${SUBJECT_SRC})"
else
  echo "No entity could be resolved from any browse/list endpoint."
fi

# ------------------------------------------------------------------------------
# (A) DAC-A — DETAILS: GET /entities/:id resolves to a populated detail body.
# ------------------------------------------------------------------------------
echo
echo "--- DAC-A (details): GET /api/v1/entities/:id -> non-empty title + id + media_type ---"
DETAIL_OK=0
if [ -z "${SUBJECT_ID}" ]; then
  ab_skip_with_reason "DAC-A entity-detail-populated" "no_entity: no entity resolved from any browse/list endpoint"
else
  CODE="$(http_get "/api/v1/entities/${SUBJECT_ID}" yes dac_detail)"
  EV_DET="${RESULTS_DIR}/dac_detail.json"
  if [ "${CODE}" != "200" ]; then
    ab_fail "DAC-A entity-detail-populated: expected 200 from /entities/${SUBJECT_ID}, got ${CODE}" "${EV_DET}"
  else
    D_ID="$(json_field "${EV_DET}" id)"
    D_TITLE="$(json_field "${EV_DET}" title)"
    D_MTYPE="$(json_field "${EV_DET}" media_type)"
    # A populated detail: numeric id present, non-empty title, media_type key present.
    if [ -z "${D_ID}" ]; then
      ab_fail "DAC-A entity-detail-populated: detail body carried no id" "${EV_DET}"
    elif [ -z "${D_TITLE}" ]; then
      ab_fail "DAC-A entity-detail-populated: detail title is EMPTY (real defect — an end user would see a blank title)" "${EV_DET}"
    elif ! json_has_key "${EV_DET}" media_type; then
      ab_fail "DAC-A entity-detail-populated: detail body has no media_type field" "${EV_DET}"
    else
      DETAIL_OK=1
      ab_pass_with_evidence "DAC-A entity-detail-populated: id=${D_ID} title='${D_TITLE}' media_type='${D_MTYPE}' (populated detail)" "${EV_DET}"
    fi
  fi
fi

# ------------------------------------------------------------------------------
# (B) DAC-B — COVER/ASSET: GET /cover/:id returns image bytes, image/* content-
#     type, NON-DEGENERATE size (§11.4.38). 0-byte / 1x1 / non-image => FAIL.
# ------------------------------------------------------------------------------
echo
echo "--- DAC-B (cover asset): GET /api/v1/cover/:id -> image bytes, non-degenerate (§11.4.38) ---"
if [ -z "${SUBJECT_ID}" ]; then
  ab_skip_with_reason "DAC-B cover-asset-non-degenerate" "no_entity: no entity resolved to request a cover for"
else
  COVER_OUT="$(http_get_binary "/api/v1/cover/${SUBJECT_ID}" no dac_cover)"
  EV_COVER_META="${RESULTS_DIR}/dac_cover.meta"
  C_CODE="$(printf '%s' "${COVER_OUT}" | cut -d'|' -f1)"
  C_SIZE="$(printf '%s' "${COVER_OUT}" | cut -d'|' -f2)"
  C_CTYPE="$(printf '%s' "${COVER_OUT}" | cut -d'|' -f3)"
  if [ "${C_CODE}" = "000" ]; then
    ab_fail "DAC-B cover-asset-non-degenerate: transport failure fetching /cover/${SUBJECT_ID}" "${EV_COVER_META}"
  elif [ "${C_CODE}" != "200" ]; then
    ab_fail "DAC-B cover-asset-non-degenerate: expected 200 from /cover/${SUBJECT_ID}, got ${C_CODE}" "${EV_COVER_META}"
  else
    # content-type MUST be an image/* type (raster jpeg/png/webp OR svg placeholder).
    case "${C_CTYPE}" in
      image/*)
        if [ -z "${C_SIZE}" ] || [ "${C_SIZE}" -le "${COVER_FLOOR}" ] 2>/dev/null; then
          ab_fail "DAC-B cover-asset-non-degenerate: DEGENERATE cover — size ${C_SIZE}B <= floor ${COVER_FLOOR}B (0-byte/1x1/truncated asset, §11.4.38)" "${EV_COVER_META}"
        else
          ab_pass_with_evidence "DAC-B cover-asset-non-degenerate: content_type='${C_CTYPE}' size=${C_SIZE}B (> ${COVER_FLOOR}B floor) — non-degenerate image asset (§11.4.38)" "${EV_COVER_META}"
        fi
        ;;
      *)
        ab_fail "DAC-B cover-asset-non-degenerate: content_type='${C_CTYPE}' is NOT an image/* type (asset endpoint returned a non-image body)" "${EV_COVER_META}"
        ;;
    esac
  fi
fi

# ------------------------------------------------------------------------------
# (C) DAC-C — COVER-URL: GET /cover/url/:id returns JSON carrying a cover_url
#     for the same media id (the metadata companion to the byte stream).
# ------------------------------------------------------------------------------
echo
echo "--- DAC-C (cover-url): GET /api/v1/cover/url/:id -> JSON cover_url field ---"
if [ -z "${SUBJECT_ID}" ]; then
  ab_skip_with_reason "DAC-C cover-url-json" "no_entity: no entity resolved to request a cover-url for"
else
  CODE="$(http_get "/api/v1/cover/url/${SUBJECT_ID}" no dac_cover_url)"
  EV_CURL="${RESULTS_DIR}/dac_cover_url.json"
  if [ "${CODE}" != "200" ]; then
    ab_fail "DAC-C cover-url-json: expected 200 from /cover/url/${SUBJECT_ID}, got ${CODE}" "${EV_CURL}"
  elif ! json_has_key "${EV_CURL}" cover_url; then
    ab_fail "DAC-C cover-url-json: response has no cover_url field" "${EV_CURL}"
  else
    CU_VAL="$(json_field "${EV_CURL}" cover_url)"
    CU_MID="$(json_field "${EV_CURL}" media_item_id)"
    ab_pass_with_evidence "DAC-C cover-url-json: media_item_id=${CU_MID} cover_url='${CU_VAL}' (cover-url metadata present)" "${EV_CURL}"
  fi
fi

# ------------------------------------------------------------------------------
# (D) DAC-D — CACHING: GET /entities/:id carries an ETag + Cache-Control, and the
#     ETag is STABLE across two independent reads (§11.4.50 determinism).
# ------------------------------------------------------------------------------
echo
echo "--- DAC-D (caching): GET /api/v1/entities/:id -> ETag present + stable across two reads (§11.4.50) ---"
SUBJECT_ETAG=""
if [ -z "${SUBJECT_ID}" ]; then
  ab_skip_with_reason "DAC-D etag-present-and-stable" "no_entity: no entity resolved to probe caching headers for"
else
  ETAG_OUT1="$(http_get_header "/api/v1/entities/${SUBJECT_ID}" "ETag" dac_etag_read1)"
  E1_CODE="$(printf '%s' "${ETAG_OUT1}" | cut -d'|' -f1)"
  E1_ETAG="$(printf '%s' "${ETAG_OUT1}" | cut -d'|' -f2-)"
  CC_OUT1="$(http_get_header "/api/v1/entities/${SUBJECT_ID}" "Cache-Control" dac_cachectl_read1)"
  E1_CC="$(printf '%s' "${CC_OUT1}" | cut -d'|' -f2-)"
  ETAG_OUT2="$(http_get_header "/api/v1/entities/${SUBJECT_ID}" "ETag" dac_etag_read2)"
  E2_ETAG="$(printf '%s' "${ETAG_OUT2}" | cut -d'|' -f2-)"
  EV_ETAG="${RESULTS_DIR}/dac_etag_read1.hdr"
  if [ "${E1_CODE}" != "200" ]; then
    ab_fail "DAC-D etag-present-and-stable: expected 200 from /entities/${SUBJECT_ID}, got ${E1_CODE}" "${EV_ETAG}"
  elif [ -z "${E1_ETAG}" ]; then
    ab_fail "DAC-D etag-present-and-stable: no ETag header on the entity detail response (caching surface absent)" "${EV_ETAG}"
  elif [ -z "${E1_CC}" ]; then
    ab_fail "DAC-D etag-present-and-stable: ETag present but no Cache-Control header" "${EV_ETAG}"
  elif [ "${E1_ETAG}" != "${E2_ETAG}" ]; then
    ab_fail "DAC-D etag-present-and-stable: ETag NOT stable across two reads (read1=${E1_ETAG} read2=${E2_ETAG}) — non-deterministic cache key (§11.4.50)" "${EV_ETAG}"
  else
    SUBJECT_ETAG="${E1_ETAG}"
    ab_pass_with_evidence "DAC-D etag-present-and-stable: ETag=${E1_ETAG} Cache-Control='${E1_CC}' stable across two reads" "${EV_ETAG}"
  fi
fi

# ------------------------------------------------------------------------------
# (E) DAC-E — CACHING: a second GET /entities/:id with If-None-Match:<ETag> is
#     served from cache as 304 Not Modified (the cache short-circuit fires; the
#     body is NOT re-served).
# ------------------------------------------------------------------------------
echo
echo "--- DAC-E (caching): If-None-Match -> 304 Not Modified served from cache ---"
if [ -z "${SUBJECT_ID}" ]; then
  ab_skip_with_reason "DAC-E conditional-304-served-from-cache" "no_entity: no entity resolved to probe conditional caching for"
elif [ -z "${SUBJECT_ETAG}" ]; then
  ab_skip_with_reason "DAC-E conditional-304-served-from-cache" "no_etag: DAC-D did not yield a stable ETag to revalidate with"
else
  COND_OUT="$(http_get_header "/api/v1/entities/${SUBJECT_ID}" "ETag" dac_conditional_304 "${SUBJECT_ETAG}")"
  COND_CODE="$(printf '%s' "${COND_OUT}" | cut -d'|' -f1)"
  EV_COND="${RESULTS_DIR}/dac_conditional_304.hdr"
  if [ "${COND_CODE}" = "304" ]; then
    ab_pass_with_evidence "DAC-E conditional-304-served-from-cache: If-None-Match:${SUBJECT_ETAG} -> HTTP 304 Not Modified (served from cache, body not re-sent)" "${EV_COND}"
  else
    ab_fail "DAC-E conditional-304-served-from-cache: expected 304 with matching If-None-Match, got ${COND_CODE} (cache revalidation did not short-circuit)" "${EV_COND}"
  fi
fi

# ------------------------------------------------------------------------------
# (F) DAC-F — ASSET fallback: GET /cover/placeholder/<type> returns image/svg+xml
#     bytes above the non-degenerate floor (§11.4.38) — the always-available
#     placeholder asset every entity can fall back to.
# ------------------------------------------------------------------------------
echo
echo "--- DAC-F (placeholder asset): GET /api/v1/cover/placeholder/<type> -> non-degenerate svg (§11.4.38) ---"
PH_OUT="$(http_get_binary "/api/v1/cover/placeholder/${ENTITY_TYPE}" no dac_placeholder)"
EV_PH_META="${RESULTS_DIR}/dac_placeholder.meta"
PH_CODE="$(printf '%s' "${PH_OUT}" | cut -d'|' -f1)"
PH_SIZE="$(printf '%s' "${PH_OUT}" | cut -d'|' -f2)"
PH_CTYPE="$(printf '%s' "${PH_OUT}" | cut -d'|' -f3)"
if [ "${PH_CODE}" = "000" ]; then
  ab_fail "DAC-F placeholder-asset-non-degenerate: transport failure fetching placeholder" "${EV_PH_META}"
elif [ "${PH_CODE}" != "200" ]; then
  ab_fail "DAC-F placeholder-asset-non-degenerate: expected 200 from /cover/placeholder/${ENTITY_TYPE}, got ${PH_CODE}" "${EV_PH_META}"
else
  case "${PH_CTYPE}" in
    image/*)
      if [ -z "${PH_SIZE}" ] || [ "${PH_SIZE}" -le "${COVER_FLOOR}" ] 2>/dev/null; then
        ab_fail "DAC-F placeholder-asset-non-degenerate: DEGENERATE placeholder — size ${PH_SIZE}B <= floor ${COVER_FLOOR}B (§11.4.38)" "${EV_PH_META}"
      else
        ab_pass_with_evidence "DAC-F placeholder-asset-non-degenerate: content_type='${PH_CTYPE}' size=${PH_SIZE}B (> ${COVER_FLOOR}B floor) — non-degenerate placeholder asset (§11.4.38)" "${EV_PH_META}"
      fi
      ;;
    *)
      ab_fail "DAC-F placeholder-asset-non-degenerate: content_type='${PH_CTYPE}' is NOT an image/* type" "${EV_PH_META}"
      ;;
  esac
fi

# ------------------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------------------
{
  printf 'PASS=%s FAIL=%s SKIP=%s\n' "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}"
  printf '%b' "${SUMMARY_ROWS}"
} > "${SUMMARY_TXT}"

printf '{"pass":%s,"fail":%s,"skip":%s,"base_url":"%s","results_dir":"%s","ts":"%s","entity_type":"%s","subject_id":"%s","guard":"details-assets-caching"}\n' \
  "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" "${BASE_URL}" "${RESULTS_DIR}" "${TS}" "${ENTITY_TYPE}" "${SUBJECT_ID}" \
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
