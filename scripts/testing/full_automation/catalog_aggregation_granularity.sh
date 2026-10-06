#!/usr/bin/env bash
# ==============================================================================
# catalog_aggregation_granularity.sh
# ------------------------------------------------------------------------------
# Purpose:
#   Standing regression + full-automation API guard for DEFECT-E
#   (title-granular SMB-scan aggregation). DEFECT-E was: the SMB scan produced
#   only 3 directory-level entities ("movies"/"music"/"software") instead of one
#   entity PER TITLE. The fix (catalog-api/internal/services/aggregation_service.go,
#   title-granular leaf walk, commit e748bba5) makes the scan emit one entity per
#   title. This suite drives the LIVE catalog-api over real HTTP and asserts the
#   per-title aggregation is genuinely present, with captured-evidence JSON for
#   every assertion (anti-bluff §11.4 / §11.4.69 — never a tautology).
#
#   POSITIVE assertions (the fixed behaviour MUST be observable):
#     A. distinct movie entities titled exactly "Inception", "Interstellar",
#        "The Matrix", each resolving to media_type == "movie" (NOT one "movies"
#        folder entity). Driven via GET /api/v1/entities/browse/movie + a
#        per-entity detail fetch that exposes the media_type string.
#     B. a tv_show entity "Breaking Bad" that is browsable and whose children are
#        tv_season entities "Season 1" + "Season 2", each having tv_episode
#        descendants (via GET /api/v1/entities/{id}/children).
#     C. a music_album entity "The Dark Side of the Moon"
#        (GET /api/v1/entities/browse/music_album + detail media_type check).
#     D. NEGATIVE — the bug's signature is ABSENT: NO entity is titled
#        "movies" / "music" / "series" / "software" / "tv" / "shows" (those were
#        the directory-level folder entities the bug emitted).
#
#   This is a §11.4.135 standing regression guard wired to run every cycle, and a
#   §11.4.146 extend-to-all-cases pass (every title, every level of the tv tree,
#   the music album, plus the negative folder-signature sweep).
#
# Usage:
#   ./catalog_aggregation_granularity.sh
#   CATALOGIZER_BASE_URL=http://127.0.0.1:8080 \
#   CATALOGIZER_ENV_FILE=/path/.api-env \
#   ./catalog_aggregation_granularity.sh
#
# Inputs (env vars, all optional with defaults):
#   CATALOGIZER_BASE_URL    API base URL          (default http://127.0.0.1:8080)
#   CATALOGIZER_ENV_FILE    path to .api-env providing ADMIN_USERNAME/ADMIN_PASSWORD
#                           (and optionally QA_TOKEN). Sourced, never echoed
#                           (§11.4.10). Default: the most-recent qa-results
#                           catalogizer-qa-*/.api-env, else none.
#   CATALOGIZER_USER        login username        (default $ADMIN_USERNAME or admin)
#   CATALOGIZER_PASS        login password        (default $ADMIN_PASSWORD)
#   CATALOGIZER_TOKEN       pre-acquired session token (skips login if set; else
#                           QA_TOKEN from the env file, else a real login)
#   CATALOGIZER_RESULTS_DIR evidence output dir
#                           (default qa-results/aggregation_granularity/<ts>)
#
# Outputs:
#   - Per-assertion captured-evidence JSON under the results dir (the real HTTP
#     response body). A summary.txt and machine-readable summary.json.
#   - PASS/FAIL/SKIP lines on stdout, each citing its evidence path.
#   - Exit code 0 iff every non-SKIP assertion PASSed; 1 otherwise.
#
# Side-effects:
#   - Network calls (read-only GETs) to the configured API only. No writes, no
#     destructive state. Evidence files written under the results dir. The
#     sourced env file is read-only. NEVER prints credentials (§11.4.10).
#
# Dependencies: bash, curl, python3 (JSON parsing — sqlite3 not required since the
#   guard is API-level, not DB-level). POSIX-portable beyond the bash shebang;
#   parses clean under both `sh -n` and `bash -n` (§11.4.67).
#
# Cross-references:
#   - docs/scripts/catalog_aggregation_granularity.md  (companion guide, §11.4.18)
#   - submodules/helix_qa/banks/catalog_aggregation_granularity.yaml (HelixQA bank)
#   - catalog-api/internal/services/aggregation_service.go (the fixed code under guard)
#   - scripts/testing/full_automation/catalog_functional_matrix.sh (pattern model)
#
# Anti-bluff (§11.4 / §11.4.69 / §11.4.27): every PASS asserts a REAL observed
#   title / media_type / hierarchy value from a captured response body, never a
#   tautology, and cites its evidence file. Real system, no mocks. An unreachable
#   API => SKIP-with-reason (§11.4.3), never a fabricated PASS.
# ==============================================================================

set -u

# ------------------------------------------------------------------------------
# Configuration
# ------------------------------------------------------------------------------
BASE_URL="${CATALOGIZER_BASE_URL:-http://127.0.0.1:8080}"

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
PASS="${CATALOGIZER_PASS:-${ADMIN_PASSWORD:-admin}}"
# Token precedence: explicit env var, else QA_TOKEN from the env file, else login.
TOKEN="${CATALOGIZER_TOKEN:-${QA_TOKEN:-}}"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
RESULTS_DIR="${CATALOGIZER_RESULTS_DIR:-qa-results/aggregation_granularity/${TS}}"
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

# id_for_title <file> <exact-title> -> the numeric entity id of the item whose
# title matches exactly (first match), or empty.
id_for_title() {
  it_file="$1"; it_title="$2"
  python3 - "${it_file}" "${it_title}" <<'PY' 2>/dev/null
import sys, json
try:
    d = json.load(open(sys.argv[1]))
except Exception:
    sys.exit(0)
want = sys.argv[2]
items = d.get("items") if isinstance(d, dict) else d
if isinstance(items, list):
    for it in items:
        if isinstance(it, dict) and it.get("title") == want and it.get("id") is not None:
            print(it["id"]); break
PY
}

# titles_contains_exact <file> <exact-title> -> exit 0 if present
titles_contains_exact() {
  tc_file="$1"; tc_title="$2"
  json_titles "${tc_file}" | grep -Fxq -- "${tc_title}"
}

echo "=============================================================="
echo " Catalogizer aggregation-granularity guard (DEFECT-E regression)"
echo " base_url   : ${BASE_URL}"
echo " results    : ${RESULTS_DIR}"
echo " env_file   : ${ENV_FILE:-<none>}"
echo " python3    : ${HAVE_PY}"
echo " started    : ${TS}"
echo "=============================================================="

if [ "${HAVE_PY}" -eq 0 ]; then
  echo
  echo "python3 is REQUIRED as the JSON oracle (no tautology fallback)."
  ab_skip_with_reason "ALL aggregation-granularity assertions" "python3 absent: cannot parse JSON evidence without it (§11.4.6)"
  printf 'PASS=%s FAIL=%s SKIP=%s\n' "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" > "${SUMMARY_TXT}"
  printf '%b' "${SUMMARY_ROWS}" >> "${SUMMARY_TXT}"
  printf '{"pass":%s,"fail":%s,"skip":%s,"reason":"python3_absent"}\n' \
    "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" > "${SUMMARY_JSON}"
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
  for t in \
    "AGG-A movie-titles" "AGG-A-media-type movie" \
    "AGG-B tv_show-Breaking-Bad" "AGG-B-seasons" "AGG-B-episodes" \
    "AGG-C music_album" "AGG-D negative-folder-signature"; do
    ab_skip_with_reason "${t}" "network_unreachable: ${BASE_URL} not reachable"
  done
  printf 'PASS=%s FAIL=%s SKIP=%s\n' "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" > "${SUMMARY_TXT}"
  printf '%b' "${SUMMARY_ROWS}" >> "${SUMMARY_TXT}"
  printf '{"pass":%s,"fail":%s,"skip":%s,"base_url":"%s","unreachable":true}\n' \
    "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" "${BASE_URL}" > "${SUMMARY_JSON}"
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
  for t in \
    "AGG-A movie-titles" "AGG-A-media-type movie" \
    "AGG-B tv_show-Breaking-Bad" "AGG-B-seasons" "AGG-B-episodes" \
    "AGG-C music_album" "AGG-D negative-folder-signature"; do
    ab_skip_with_reason "${t}" "no_token: login did not yield a session_token"
  done
  printf 'PASS=%s FAIL=%s SKIP=%s\n' "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" > "${SUMMARY_TXT}"
  printf '%b' "${SUMMARY_ROWS}" >> "${SUMMARY_TXT}"
  printf '{"pass":%s,"fail":%s,"skip":%s,"base_url":"%s","no_token":true}\n' \
    "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" "${BASE_URL}" > "${SUMMARY_JSON}"
  echo "Summary: PASS=${PASS_COUNT} FAIL=${FAIL_COUNT} SKIP=${SKIP_COUNT}"
  exit 0
fi
echo "Auth token: acquired (value not printed, §11.4.10)."

# ------------------------------------------------------------------------------
# (A) Per-title MOVIE entities — Inception, Interstellar, The Matrix — each with
#     media_type == "movie" (NOT one collapsed "movies" folder entity).
# ------------------------------------------------------------------------------
echo
echo "--- AGG-A: GET /api/v1/entities/browse/movie -> per-title movie entities ---"
CODE="$(http_get /api/v1/entities/browse/movie yes agg_a_browse_movie)"
EV_MV="${RESULTS_DIR}/agg_a_browse_movie.json"
if [ "${CODE}" != "200" ]; then
  ab_fail "AGG-A movie-browse: expected 200, got ${CODE}" "${EV_MV}"
else
  MISSING_MV=""
  for title in "Inception" "Interstellar" "The Matrix"; do
    if ! titles_contains_exact "${EV_MV}" "${title}"; then
      MISSING_MV="${MISSING_MV} [${title}]"
    fi
  done
  if [ -n "${MISSING_MV}" ]; then
    ab_fail "AGG-A movie-titles: missing distinct per-title movie entity(ies):${MISSING_MV}" "${EV_MV}"
  else
    ab_pass_with_evidence "AGG-A movie-titles: distinct 'Inception' + 'Interstellar' + 'The Matrix' present (per-title, not a 'movies' folder)" "${EV_MV}"
  fi

  # (A2) media_type == "movie" on the per-entity detail for each title. The list
  # endpoint exposes only media_type_id; the detail endpoint resolves the string.
  echo
  echo "--- AGG-A2: per-title detail media_type == 'movie' ---"
  MT_FAIL=""
  i=0
  for title in "Inception" "Interstellar" "The Matrix"; do
    i=$((i + 1))
    MID="$(id_for_title "${EV_MV}" "${title}")"
    if [ -z "${MID}" ]; then
      MT_FAIL="${MT_FAIL} [${title}:no-id]"
      continue
    fi
    CODE="$(http_get "/api/v1/entities/${MID}" yes "agg_a2_detail_${i}")"
    EV_D="${RESULTS_DIR}/agg_a2_detail_${i}.json"
    if [ "${CODE}" != "200" ]; then
      MT_FAIL="${MT_FAIL} [${title}:http=${CODE}]"
      continue
    fi
    MT="$(json_field "${EV_D}" media_type)"
    DT="$(json_field "${EV_D}" title)"
    if [ "${MT}" != "movie" ] || [ "${DT}" != "${title}" ]; then
      MT_FAIL="${MT_FAIL} [${title}:media_type='${MT}',title='${DT}']"
    fi
  done
  if [ -n "${MT_FAIL}" ]; then
    ab_fail "AGG-A2 movie-media-type: not all titles resolve to media_type 'movie':${MT_FAIL}" "${RESULTS_DIR}/agg_a2_detail_1.json"
  else
    ab_pass_with_evidence "AGG-A2 movie-media-type: Inception/Interstellar/The Matrix each detail media_type='movie'" "${RESULTS_DIR}/agg_a2_detail_1.json"
  fi
fi

# ------------------------------------------------------------------------------
# (B) tv_show "Breaking Bad" browsable -> tv_season children "Season 1"+"Season 2"
#     -> each season has tv_episode descendants.
# ------------------------------------------------------------------------------
echo
echo "--- AGG-B: GET /api/v1/entities/browse/tv_show -> 'Breaking Bad' ---"
CODE="$(http_get /api/v1/entities/browse/tv_show yes agg_b_browse_tv)"
EV_TV="${RESULTS_DIR}/agg_b_browse_tv.json"
BB_ID=""
if [ "${CODE}" != "200" ]; then
  ab_fail "AGG-B tv-browse: expected 200, got ${CODE}" "${EV_TV}"
elif ! titles_contains_exact "${EV_TV}" "Breaking Bad"; then
  ab_fail "AGG-B tv_show-Breaking-Bad: no tv_show entity titled 'Breaking Bad'" "${EV_TV}"
else
  BB_ID="$(id_for_title "${EV_TV}" "Breaking Bad")"
  ab_pass_with_evidence "AGG-B tv_show-Breaking-Bad: browsable tv_show 'Breaking Bad' present (id=${BB_ID})" "${EV_TV}"
fi

# (B2) Seasons as tv_season children of Breaking Bad.
echo
echo "--- AGG-B2: children of Breaking Bad -> 'Season 1' + 'Season 2' (tv_season) ---"
S1_ID=""; S2_ID=""
if [ -z "${BB_ID}" ]; then
  ab_skip_with_reason "AGG-B2 seasons" "no_breaking_bad_id: parent tv_show not resolved"
else
  CODE="$(http_get "/api/v1/entities/${BB_ID}/children" yes agg_b2_seasons)"
  EV_S="${RESULTS_DIR}/agg_b2_seasons.json"
  if [ "${CODE}" != "200" ]; then
    ab_fail "AGG-B2 seasons: expected 200 from children, got ${CODE}" "${EV_S}"
  else
    SEASON_MISSING=""
    for s in "Season 1" "Season 2"; do
      titles_contains_exact "${EV_S}" "${s}" || SEASON_MISSING="${SEASON_MISSING} [${s}]"
    done
    if [ -n "${SEASON_MISSING}" ]; then
      ab_fail "AGG-B2 seasons: missing season child(ren):${SEASON_MISSING}" "${EV_S}"
    else
      S1_ID="$(id_for_title "${EV_S}" "Season 1")"
      S2_ID="$(id_for_title "${EV_S}" "Season 2")"
      # Verify the season child resolves to media_type tv_season on its detail.
      CODE="$(http_get "/api/v1/entities/${S1_ID}" yes agg_b2_season1_detail)"
      S1_MT="$(json_field "${RESULTS_DIR}/agg_b2_season1_detail.json" media_type)"
      if [ "${S1_MT}" = "tv_season" ]; then
        ab_pass_with_evidence "AGG-B2 seasons: 'Season 1'+'Season 2' are tv_season children of Breaking Bad (Season 1 media_type=tv_season)" "${EV_S}"
      else
        ab_fail "AGG-B2 seasons: season present but Season 1 media_type='${S1_MT}' (expected tv_season)" "${RESULTS_DIR}/agg_b2_season1_detail.json"
      fi
    fi
  fi
fi

# (B3) Episodes as tv_episode descendants under a season.
echo
echo "--- AGG-B3: children of a season -> tv_episode descendants ---"
if [ -z "${S1_ID}" ] && [ -z "${S2_ID}" ]; then
  ab_skip_with_reason "AGG-B3 episodes" "no_season_id: no season resolved to descend into"
else
  SEASON_FOR_EP="${S1_ID:-${S2_ID}}"
  CODE="$(http_get "/api/v1/entities/${SEASON_FOR_EP}/children" yes agg_b3_episodes)"
  EV_EP="${RESULTS_DIR}/agg_b3_episodes.json"
  if [ "${CODE}" != "200" ]; then
    ab_fail "AGG-B3 episodes: expected 200 from season children, got ${CODE}" "${EV_EP}"
  else
    EP_COUNT="$(json_titles "${EV_EP}" | grep -c .)"
    # Confirm the first episode child resolves to media_type tv_episode.
    EP1_ID="$(python3 - "${EV_EP}" <<'PY' 2>/dev/null
import sys, json
d = json.load(open(sys.argv[1]))
items = d.get("items") if isinstance(d, dict) else d
if isinstance(items, list) and items and isinstance(items[0], dict):
    print(items[0].get("id", ""))
PY
)"
    EP1_MT=""
    if [ -n "${EP1_ID}" ]; then
      http_get "/api/v1/entities/${EP1_ID}" yes agg_b3_episode1_detail >/dev/null
      EP1_MT="$(json_field "${RESULTS_DIR}/agg_b3_episode1_detail.json" media_type)"
    fi
    if [ "${EP_COUNT}" -ge 1 ] && [ "${EP1_MT}" = "tv_episode" ]; then
      ab_pass_with_evidence "AGG-B3 episodes: season has ${EP_COUNT} tv_episode descendant(s) (first media_type=tv_episode)" "${EV_EP}"
    else
      ab_fail "AGG-B3 episodes: expected >=1 tv_episode descendant (count=${EP_COUNT}, first media_type='${EP1_MT}')" "${EV_EP}"
    fi
  fi
fi

# ------------------------------------------------------------------------------
# (C) music_album "The Dark Side of the Moon".
# ------------------------------------------------------------------------------
echo
echo "--- AGG-C: GET /api/v1/entities/browse/music_album -> 'The Dark Side of the Moon' ---"
CODE="$(http_get /api/v1/entities/browse/music_album yes agg_c_browse_album)"
EV_AL="${RESULTS_DIR}/agg_c_browse_album.json"
if [ "${CODE}" != "200" ]; then
  ab_fail "AGG-C music_album: expected 200 from browse/music_album, got ${CODE}" "${EV_AL}"
elif ! titles_contains_exact "${EV_AL}" "The Dark Side of the Moon"; then
  ab_fail "AGG-C music_album: no music_album titled 'The Dark Side of the Moon'" "${EV_AL}"
else
  AL_ID="$(id_for_title "${EV_AL}" "The Dark Side of the Moon")"
  http_get "/api/v1/entities/${AL_ID}" yes agg_c_album_detail >/dev/null
  AL_MT="$(json_field "${RESULTS_DIR}/agg_c_album_detail.json" media_type)"
  if [ "${AL_MT}" = "music_album" ]; then
    ab_pass_with_evidence "AGG-C music_album: 'The Dark Side of the Moon' present + detail media_type='music_album'" "${EV_AL}"
  else
    ab_fail "AGG-C music_album: present but detail media_type='${AL_MT}' (expected music_album)" "${RESULTS_DIR}/agg_c_album_detail.json"
  fi
fi

# ------------------------------------------------------------------------------
# (D) NEGATIVE — the bug's signature is ABSENT. The pre-fix scan emitted
#     directory-level folder entities ("movies"/"music"/"series"/"software"/...).
#     The fixed per-title scan emits NONE of these. Sweep ALL entities.
# ------------------------------------------------------------------------------
echo
echo "--- AGG-D (NEGATIVE): no folder-level entity titled movies/music/series/software/tv/shows ---"
CODE="$(http_get '/api/v1/entities?limit=500&offset=0' yes agg_d_all_entities)"
EV_ALL="${RESULTS_DIR}/agg_d_all_entities.json"
if [ "${CODE}" != "200" ]; then
  ab_fail "AGG-D negative: expected 200 from /entities, got ${CODE}" "${EV_ALL}"
else
  FOLDERY="$(python3 - "${EV_ALL}" <<'PY' 2>/dev/null
import sys, json
d = json.load(open(sys.argv[1]))
items = d.get("items") if isinstance(d, dict) else d
bad_set = {"movies", "music", "series", "software", "tv", "shows", "tv_shows", "movie", "videos"}
bad = []
if isinstance(items, list):
    for it in items:
        if isinstance(it, dict):
            t = str(it.get("title", "")).strip().lower()
            if t in bad_set:
                bad.append(it.get("title"))
print(",".join(repr(b) for b in bad))
PY
)"
  if [ -n "${FOLDERY}" ]; then
    ab_fail "AGG-D negative: DEFECT-E folder-signature present — entity(ies) titled: ${FOLDERY}" "${EV_ALL}"
  else
    TOTAL="$(json_field "${EV_ALL}" total)"
    ab_pass_with_evidence "AGG-D negative: NO directory-level folder entity present (swept ${TOTAL} entities; bug signature absent)" "${EV_ALL}"
  fi
fi

# ------------------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------------------
{
  printf 'PASS=%s FAIL=%s SKIP=%s\n' "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}"
  printf '%b' "${SUMMARY_ROWS}"
} > "${SUMMARY_TXT}"

printf '{"pass":%s,"fail":%s,"skip":%s,"base_url":"%s","results_dir":"%s","ts":"%s","guard":"DEFECT-E-aggregation-granularity"}\n' \
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
