#!/usr/bin/env bash
# ==============================================================================
# catalog_music_album_tracks.sh
# ------------------------------------------------------------------------------
# Purpose:
#   §11.4 full-automation API guard proving the catalog-api MUSIC surfaces work
#   end-to-end against the LIVE catalog-api over real HTTP:
#
#     (1) album browse + resolve  — GET /api/v1/entities/browse/music_album
#         lists music_album entities; each resolves to a music_album via
#         GET /api/v1/entities/{id} (media_type == "music_album").
#     (2) album -> track listing   — GET /api/v1/entities/{album_id}/children
#         returns the album's tracks (container children). The track list shape
#         is asserted HONESTLY: count >= 0. Where tracks ARE seeded, each track's
#         track_number ordering is captured as positive evidence; where the
#         catalog seeds an album with NO track children (the current corpus seeds
#         music_album entities but no track children), the children-list-shape is
#         asserted (a 200 body carrying an `items` array, count >= 0) rather than
#         a track being fabricated (§11.4.6 no-guessing, §11.4.3 honesty).
#     (3) per-track playback progress — the SAME mechanism every media card uses:
#         POST /playback/sessions/start -> /progress -> /end, then
#         GET /entities/{id}/progress reads progress.last_position back. A chosen
#         playable media item (a track when seeded, else the album entity, which
#         is itself a valid media_item_id for the playback lifecycle) has its
#         position set and read back EQUAL (resume-per-track). Determinism
#         (§11.4.50): the read-back last_position is identical across two
#         independent start->progress->end->read cycles for the same position.
#
#   Every PASS cites a REAL observed value (album title / media_type / child
#   track_number / read-back last_position) from a captured response body, never
#   a tautology, and cites its evidence file (§11.4 / §11.4.69 / §11.4.27).
#   An unreachable API / no album / no token => SKIP-with-reason (§11.4.3),
#   never a fabricated PASS.
#
#   This suite is a §11.4.135 standing regression guard wired to run every cycle,
#   and a §11.4.146 extend-to-all-cases pass (browse, every album resolve, the
#   per-album children-list-shape sweep, the playback round-trip, and the
#   determinism re-read).
#
# Usage:
#   ./catalog_music_album_tracks.sh
#   CATALOGIZER_BASE_URL=http://127.0.0.1:8080 \
#   CATALOGIZER_ENV_FILE=/path/.api-env \
#   ./catalog_music_album_tracks.sh
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
#                           (default qa-results/music_album_tracks/<ts>)
#   CATALOGIZER_TRACK_POSITION  playback position (seconds) to set + read back
#                           (default 137 — an arbitrary stable non-zero value)
#
# Outputs:
#   - Per-assertion captured-evidence JSON under the results dir (the real HTTP
#     response body). A summary.txt and machine-readable summary.json.
#   - PASS/FAIL/SKIP lines on stdout, each citing its evidence path.
#   - Exit code 0 iff every non-SKIP assertion PASSed; 1 otherwise.
#
# Side-effects:
#   - Network calls to the configured API only. Read-only GET probes for browse /
#     children / detail. The playback assertion makes a bounded WRITE
#     (start->progress->end) against a chosen media item; this is per-user
#     progress state, NOT catalog state, and is intrinsically idempotent at the
#     read-back layer (last_position is set to the SAME value each run). No scan,
#     no clear, no aggregate, no destructive catalog mutation (§11.4.119 — the
#     conductor owns the catalog/DB; this suite never touches catalog rows).
#   - Evidence files written under the results dir. The sourced env file is
#     read-only. NEVER prints credentials (§11.4.10).
#
# Dependencies: bash, curl, python3 (JSON oracle). POSIX-portable beyond the bash
#   shebang; parses clean under both `sh -n` and `bash -n` (§11.4.67).
#
# Cross-references:
#   - docs/scripts/catalog_music_album_tracks.md          (companion guide §11.4.18)
#   - submodules/helix_qa/banks/catalog_music_album_tracks.yaml (HelixQA bank)
#   - scripts/testing/full_automation/catalog_episode_titles_dedup.sh (model)
#
# Anti-bluff (§11.4 / §11.4.69 / §11.4.27): every PASS asserts a REAL observed
#   value from a captured response body, never a tautology, and cites its
#   evidence file. Real system, no mocks. Unreachable/no-album/no-token => SKIP.
# ==============================================================================

set -u

# ------------------------------------------------------------------------------
# Configuration
# ------------------------------------------------------------------------------
BASE_URL="${CATALOGIZER_BASE_URL:-http://127.0.0.1:8080}"
TRACK_POSITION="${CATALOGIZER_TRACK_POSITION:-137}"

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
RESULTS_DIR="${CATALOGIZER_RESULTS_DIR:-qa-results/music_album_tracks/${TS}}"
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
# Cleanup (§11.4.14) — leave the working tree quiescent. The only server-side
# state created is per-user playback progress (start->progress->end); it is NOT
# catalog state and is intrinsically idempotent at the read-back layer. We remove
# our own transient scratch files on every exit path.
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

# http_post_json <path> <json-body> <evidence_basename> -> echoes HTTP status code
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

# json_first_id <file> -> the id of the first item in an items[] body (or empty)
json_first_id() {
  jf_file="$1"
  python3 - "${jf_file}" <<'PY' 2>/dev/null
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

# json_item_ids <file> -> newline-separated ids of all items in an items[] body
json_item_ids() {
  jf_file="$1"
  python3 - "${jf_file}" <<'PY' 2>/dev/null
import sys, json
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    sys.exit(0)
items = d.get("items") if isinstance(d, dict) else d
if isinstance(items, list):
    for it in items:
        if isinstance(it, dict) and it.get("id") is not None:
            print(it["id"])
PY
}

# json_items_count <file> -> the integer count of items[] (0 if absent/empty)
json_items_count() {
  jf_file="$1"
  python3 - "${jf_file}" <<'PY' 2>/dev/null
import sys, json
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    print(0); sys.exit(0)
items = d.get("items") if isinstance(d, dict) else d
print(len(items) if isinstance(items, list) else 0)
PY
}

# json_has_items_array <file> -> exit 0 if the body has an `items` array (shape)
json_has_items_array() {
  jf_file="$1"
  python3 - "${jf_file}" <<'PY' 2>/dev/null
import sys, json
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    sys.exit(1)
sys.exit(0 if isinstance(d, dict) and isinstance(d.get("items"), list) else 1)
PY
}

# track_order_report <file> -> "ORDERED|N|<list>" or "UNORDERED|N|<list>" or
# "NOTRACKS|0|" — captures the track_number ordering of an album's children.
track_order_report() {
  tr_file="$1"
  python3 - "${tr_file}" <<'PY' 2>/dev/null
import sys, json
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    print("NOTRACKS|0|"); sys.exit(0)
items = d.get("items") if isinstance(d, dict) else d
if not isinstance(items, list) or not items:
    print("NOTRACKS|0|"); sys.exit(0)
nums = []
for it in items:
    if isinstance(it, dict) and it.get("track_number") is not None:
        try:
            nums.append(int(it["track_number"]))
        except (TypeError, ValueError):
            pass
if not nums:
    # children present but no track_number metadata — honest, not a failure.
    print("NOTRACKNUM|%d|" % len(items)); sys.exit(0)
ordered = all(nums[i] <= nums[i + 1] for i in range(len(nums) - 1))
print("%s|%d|%s" % ("ORDERED" if ordered else "UNORDERED",
                    len(nums), ",".join(str(n) for n in nums[:20])))
PY
}

echo "=============================================================="
echo " Catalogizer MUSIC album -> tracks + per-track playback guard"
echo " base_url   : ${BASE_URL}"
echo " position   : ${TRACK_POSITION}"
echo " results    : ${RESULTS_DIR}"
echo " env_file   : ${ENV_FILE:-<none>}"
echo " python3    : ${HAVE_PY}"
echo " started    : ${TS}"
echo "=============================================================="

emit_summary() {
  es_extra_json="$1"
  {
    printf 'PASS=%s FAIL=%s SKIP=%s\n' "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}"
    printf '%b' "${SUMMARY_ROWS}"
  } > "${SUMMARY_TXT}"
  printf '{"pass":%s,"fail":%s,"skip":%s,"base_url":"%s","results_dir":"%s","ts":"%s","guard":"music-album-tracks-playback"%s}\n' \
    "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" "${BASE_URL}" "${RESULTS_DIR}" "${TS}" "${es_extra_json}" \
    > "${SUMMARY_JSON}"
}

# skip_all <reason> — SKIP every assertion in the current shell (counters
# increment here, never in a subshell — mirrors the reference for-loop style).
skip_all() {
  sa_reason="$1"
  for lbl in \
    "MAT-API-001 music-album-browse-resolves" \
    "MAT-API-002 album-children-track-listing-shape" \
    "MAT-API-003 album-resolves-to-music-album" \
    "MAT-API-004 per-track-playback-progress-resume" \
    "MAT-API-005 playback-progress-determinism"; do
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
# (MAT-API-001) Music-album browse resolves to >=1 music_album entity.
#   GET /api/v1/entities/browse/music_album
# ------------------------------------------------------------------------------
echo
echo "--- MAT-API-001: browse music_album via GET /entities/browse/music_album ---"
CODE="$(http_get "/api/v1/entities/browse/music_album?limit=50" yes mat_browse_album)"
EV_BROWSE="${RESULTS_DIR}/mat_browse_album.json"
ALBUM_IDS=""
FIRST_ALBUM_ID=""
if [ "${CODE}" != "200" ]; then
  ab_fail "MAT-API-001 music-album-browse-resolves: expected 200, got ${CODE}" "${EV_BROWSE}"
else
  N_ALBUMS="$(json_items_count "${EV_BROWSE}")"
  if [ "${N_ALBUMS}" -ge 1 ]; then
    ALBUM_IDS="$(json_item_ids "${EV_BROWSE}")"
    FIRST_ALBUM_ID="$(json_first_id "${EV_BROWSE}")"
    ab_pass_with_evidence "MAT-API-001 music-album-browse-resolves: ${N_ALBUMS} music_album entit(y/ies) listed (first id=${FIRST_ALBUM_ID})" "${EV_BROWSE}"
  else
    # Honest: zero seeded albums is a topology gap, not a product defect.
    ab_skip_with_reason "MAT-API-001 music-album-browse-resolves" "no_music_album_seeded: browse returned 200 with 0 items (§11.4.3 topology)"
  fi
fi

# ------------------------------------------------------------------------------
# (MAT-API-003) Each browsed album resolves via GET /entities/{id} to a
#   music_album (media_type == "music_album") with a children_count field.
# ------------------------------------------------------------------------------
echo
echo "--- MAT-API-003: each album resolves to media_type=music_album ---"
if [ -z "${ALBUM_IDS}" ]; then
  ab_skip_with_reason "MAT-API-003 album-resolves-to-music-album" "no_album_ids: nothing to resolve"
else
  MAT3_BAD=""
  MAT3_OK_REPORT=""
  MAT3_IDX=0
  for aid in ${ALBUM_IDS}; do
    MAT3_IDX=$((MAT3_IDX + 1))
    http_get "/api/v1/entities/${aid}" yes "mat_album_detail_${aid}" >/dev/null
    EV_DET="${RESULTS_DIR}/mat_album_detail_${aid}.json"
    MT="$(json_field "${EV_DET}" media_type)"
    ATITLE="$(json_field "${EV_DET}" title)"
    if [ "${MT}" = "music_album" ]; then
      MAT3_OK_REPORT="${MAT3_OK_REPORT}[${aid}:${ATITLE}] "
    else
      MAT3_BAD="${MAT3_BAD}[${aid}:media_type='${MT}'] "
    fi
  done
  EV_DET_FIRST="${RESULTS_DIR}/mat_album_detail_${FIRST_ALBUM_ID}.json"
  if [ -n "${MAT3_BAD}" ]; then
    ab_fail "MAT-API-003 album-resolves-to-music-album: ${MAT3_IDX} album(s) checked, non-music_album media_type for: ${MAT3_BAD}" "${EV_DET_FIRST}"
  else
    ab_pass_with_evidence "MAT-API-003 album-resolves-to-music-album: all ${MAT3_IDX} album(s) resolve media_type=music_album: ${MAT3_OK_REPORT}" "${EV_DET_FIRST}"
  fi
fi

# ------------------------------------------------------------------------------
# (MAT-API-002) Album -> track listing (container children) shape is correct.
#   GET /api/v1/entities/{album_id}/children
#   The body MUST carry an `items` array (count >= 0 — honest). Where tracks ARE
#   present, the track_number ordering is captured as positive evidence. Where
#   NO track children are seeded (the current corpus), the children-list-shape
#   (200 + items[] + count >= 0) is the asserted fact — NOT a fabricated track
#   (§11.4.6 / §11.4.3).
# ------------------------------------------------------------------------------
echo
echo "--- MAT-API-002: album children = track listing (GET /entities/{id}/children) ---"
TRACK_PLAYABLE_ID=""
if [ -z "${ALBUM_IDS}" ]; then
  ab_skip_with_reason "MAT-API-002 album-children-track-listing-shape" "no_album_ids: nothing to list children for"
else
  MAT2_BAD=""
  MAT2_REPORT=""
  MAT2_TOTAL_TRACKS=0
  for aid in ${ALBUM_IDS}; do
    CCODE="$(http_get "/api/v1/entities/${aid}/children" yes "mat_album_children_${aid}")"
    EV_CH="${RESULTS_DIR}/mat_album_children_${aid}.json"
    if [ "${CCODE}" != "200" ]; then
      MAT2_BAD="${MAT2_BAD}[${aid}:status=${CCODE}] "
      continue
    fi
    if ! json_has_items_array "${EV_CH}"; then
      MAT2_BAD="${MAT2_BAD}[${aid}:no-items-array] "
      continue
    fi
    NCHILD="$(json_items_count "${EV_CH}")"
    ORDER_REPORT="$(track_order_report "${EV_CH}")"
    ORDER_KIND="$(printf '%s' "${ORDER_REPORT}" | cut -d'|' -f1)"
    if [ "${ORDER_KIND}" = "UNORDERED" ]; then
      MAT2_BAD="${MAT2_BAD}[${aid}:track_numbers-not-ascending:${ORDER_REPORT}] "
      continue
    fi
    MAT2_TOTAL_TRACKS=$((MAT2_TOTAL_TRACKS + NCHILD))
    MAT2_REPORT="${MAT2_REPORT}[${aid}:children=${NCHILD};${ORDER_REPORT}] "
    # Remember the first real track id (if any) for the playback assertion.
    if [ -z "${TRACK_PLAYABLE_ID}" ] && [ "${NCHILD}" -ge 1 ]; then
      TRACK_PLAYABLE_ID="$(json_first_id "${EV_CH}")"
    fi
  done
  EV_CH_FIRST="${RESULTS_DIR}/mat_album_children_${FIRST_ALBUM_ID}.json"
  if [ -n "${MAT2_BAD}" ]; then
    ab_fail "MAT-API-002 album-children-track-listing-shape: shape/ordering defect(s): ${MAT2_BAD}" "${EV_CH_FIRST}"
  else
    ab_pass_with_evidence "MAT-API-002 album-children-track-listing-shape: every album returns a valid items[] track listing (count>=0 honest); total tracks=${MAT2_TOTAL_TRACKS}; ${MAT2_REPORT}" "${EV_CH_FIRST}"
  fi
fi

# ------------------------------------------------------------------------------
# Choose the playable media item for the per-track playback assertions.
#   Prefer a real seeded track (album child); else fall back to the album entity
#   itself, which is a valid media_item_id for the playback lifecycle (the
#   mechanism is entity-agnostic). The fallback is the honest path when the
#   corpus seeds albums with no track children (§11.4.6 — assert the real
#   mechanism on a real entity, never fabricate a track row).
# ------------------------------------------------------------------------------
PLAY_ID="${TRACK_PLAYABLE_ID}"
PLAY_KIND="track"
if [ -z "${PLAY_ID}" ]; then
  PLAY_ID="${FIRST_ALBUM_ID}"
  PLAY_KIND="album"
fi

# ------------------------------------------------------------------------------
# (MAT-API-004) Per-track playback progress + resume.
#   start -> progress -> end, then read GET /entities/{id}/progress and assert
#   progress.last_position == TRACK_POSITION (resume-per-track).
# ------------------------------------------------------------------------------
echo
echo "--- MAT-API-004: per-track playback progress + resume (set -> read back) ---"
LAST1=""
if [ -z "${PLAY_ID}" ]; then
  ab_skip_with_reason "MAT-API-004 per-track-playback-progress-resume" "no_playable_id: no track or album to drive playback"
else
  echo "Driving playback on ${PLAY_KIND} entity id=${PLAY_ID}, position=${TRACK_POSITION}"
  SCODE="$(http_post_json /api/v1/playback/sessions/start \
    "{\"media_item_id\":${PLAY_ID},\"position_unit\":\"seconds\",\"start_position\":0}" \
    mat_play_start)"
  EV_START="${RESULTS_DIR}/mat_play_start.json"
  SID="$(json_field "${EV_START}" session_id)"
  if [ "${SCODE}" != "200" ] || [ -z "${SID}" ]; then
    ab_fail "MAT-API-004 per-track-playback-progress-resume: start failed (status=${SCODE}, session_id='${SID}')" "${EV_START}"
  else
    http_post_json /api/v1/playback/sessions/progress \
      "{\"session_id\":${SID},\"end_position\":${TRACK_POSITION},\"total_amount\":${TRACK_POSITION}}" \
      mat_play_progress >/dev/null
    http_post_json /api/v1/playback/sessions/end \
      "{\"session_id\":${SID},\"end_position\":${TRACK_POSITION},\"total_amount\":${TRACK_POSITION},\"completed\":false}" \
      mat_play_end >/dev/null
    http_get "/api/v1/entities/${PLAY_ID}/progress" yes mat_play_read1 >/dev/null
    EV_READ1="${RESULTS_DIR}/mat_play_read1.json"
    LAST1="$(json_field "${EV_READ1}" progress.last_position)"
    if [ "${LAST1}" = "${TRACK_POSITION}" ]; then
      ab_pass_with_evidence "MAT-API-004 per-track-playback-progress-resume: ${PLAY_KIND} ${PLAY_ID} progress set to ${TRACK_POSITION}s and read back EQUAL (progress.last_position=${LAST1})" "${EV_READ1}"
    else
      ab_fail "MAT-API-004 per-track-playback-progress-resume: read-back last_position='${LAST1}' != set position '${TRACK_POSITION}'" "${EV_READ1}"
    fi
  fi
fi

# ------------------------------------------------------------------------------
# (MAT-API-005) Determinism (§11.4.50): a second independent
#   start->progress->end->read cycle for the SAME position reads back the SAME
#   last_position. last_position is set-state (overwrite), so two cycles with the
#   same position yield identical last_position read-backs.
# ------------------------------------------------------------------------------
echo
echo "--- MAT-API-005: playback progress determinism (§11.4.50, two cycles) ---"
if [ -z "${PLAY_ID}" ] || [ -z "${LAST1}" ]; then
  ab_skip_with_reason "MAT-API-005 playback-progress-determinism" "no_first_read: MAT-API-004 did not produce a read-back to compare"
else
  SCODE2="$(http_post_json /api/v1/playback/sessions/start \
    "{\"media_item_id\":${PLAY_ID},\"position_unit\":\"seconds\",\"start_position\":0}" \
    mat_play_start2)"
  SID2="$(json_field "${RESULTS_DIR}/mat_play_start2.json" session_id)"
  if [ "${SCODE2}" != "200" ] || [ -z "${SID2}" ]; then
    ab_fail "MAT-API-005 playback-progress-determinism: second start failed (status=${SCODE2})" "${RESULTS_DIR}/mat_play_start2.json"
  else
    http_post_json /api/v1/playback/sessions/progress \
      "{\"session_id\":${SID2},\"end_position\":${TRACK_POSITION},\"total_amount\":${TRACK_POSITION}}" \
      mat_play_progress2 >/dev/null
    http_post_json /api/v1/playback/sessions/end \
      "{\"session_id\":${SID2},\"end_position\":${TRACK_POSITION},\"total_amount\":${TRACK_POSITION},\"completed\":false}" \
      mat_play_end2 >/dev/null
    http_get "/api/v1/entities/${PLAY_ID}/progress" yes mat_play_read2 >/dev/null
    EV_READ2="${RESULTS_DIR}/mat_play_read2.json"
    LAST2="$(json_field "${EV_READ2}" progress.last_position)"
    if [ "${LAST2}" = "${TRACK_POSITION}" ] && [ "${LAST2}" = "${LAST1}" ]; then
      ab_pass_with_evidence "MAT-API-005 playback-progress-determinism: last_position stable across two cycles (read1=${LAST1}, read2=${LAST2}, position=${TRACK_POSITION})" "${EV_READ2}"
    else
      ab_fail "MAT-API-005 playback-progress-determinism: last_position not stable (read1=${LAST1}, read2=${LAST2}, expected=${TRACK_POSITION})" "${EV_READ2}"
    fi
  fi
fi

# ------------------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------------------
{
  printf 'PASS=%s FAIL=%s SKIP=%s\n' "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}"
  printf '%b' "${SUMMARY_ROWS}"
} > "${SUMMARY_TXT}"

printf '{"pass":%s,"fail":%s,"skip":%s,"base_url":"%s","results_dir":"%s","ts":"%s","guard":"music-album-tracks-playback"}\n' \
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
