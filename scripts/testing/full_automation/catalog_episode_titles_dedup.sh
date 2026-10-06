#!/usr/bin/env bash
# ==============================================================================
# catalog_episode_titles_dedup.sh
# ------------------------------------------------------------------------------
# Purpose:
#   Standing regression + full-automation API guard for DEFECT-H + DEFECT-I in
#   the TV-episode aggregation path.
#
#     DEFECT-H (no duplicate episodes): the scan emitted two (or more) episode
#       entities sharing the SAME episode_number under the SAME season — a
#       duplicate-episode bug. The fix de-duplicates by (season, episode_number)
#       so each (season_id, episode_number) pair is unique.
#
#     DEFECT-I (real human episode titles): episodes whose source filename
#       carries a human title (e.g. "Breaking Bad S01E01.Pilot") were titled
#       generically ("Episode 1") instead of "Pilot". The fix carries the human
#       title from the filename into the entity title.
#
#   This suite drives the LIVE catalog-api over real HTTP and asserts the fixed
#   behaviour (de-duplicated episodes + real human titles where present), with
#   captured-evidence JSON for every assertion (anti-bluff §11.4 / §11.4.69 —
#   never a tautology).
#
#   POSITIVE / NEGATIVE assertions:
#     ETD-A (no-dup, DEFECT-H, NEGATIVE): for "Breaking Bad", walk seasons via
#        GET /entities/{id}/children then each season's episodes via children;
#        collect (season_id, episode_number) pairs; assert NO duplicate pair.
#     ETD-B (real titles, DEFECT-I, POSITIVE): assert at least one episode whose
#        filename carries a human title (e.g. S01E01 "Pilot") has an entity
#        title that is the human title — i.e. NOT matching `^Episode [0-9]+$`.
#     ETD-C (fallback honesty): an episode with NO human title in its filename
#        may legitimately be titled "Episode N" — assert that the "Episode N"
#        pattern is ACCEPTED (the guard does NOT false-FAIL genuinely-untitled
#        episodes); the suite PASSes whether or not such an episode exists.
#     ETD-D (count sanity + determinism): assert the show's episodes are present
#        and the de-duplicated tv_episode count under "Breaking Bad" is stable
#        across two independent reads (§11.4.50 determinism).
#
#   This is a §11.4.135 standing regression guard wired to run every cycle, and a
#   §11.4.146 extend-to-all-cases pass (every season, every episode, the
#   per-episode title check, the duplicate-pair sweep, and the determinism
#   re-read).
#
# Usage:
#   ./catalog_episode_titles_dedup.sh
#   CATALOGIZER_BASE_URL=http://127.0.0.1:8080 \
#   CATALOGIZER_ENV_FILE=/path/.api-env \
#   ./catalog_episode_titles_dedup.sh
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
#                           (default qa-results/episode_titles_dedup/<ts>)
#   CATALOGIZER_SHOW_TITLE  TV show under guard    (default "Breaking Bad")
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
#   - docs/scripts/catalog_episode_titles_dedup.md  (companion guide, §11.4.18)
#   - submodules/helix_qa/banks/catalog_episode_titles_dedup.yaml (HelixQA bank)
#   - scripts/testing/full_automation/catalog_aggregation_granularity.sh (model)
#
# Anti-bluff (§11.4 / §11.4.69 / §11.4.27): every PASS asserts a REAL observed
#   episode_number / title / hierarchy value from a captured response body, never
#   a tautology, and cites its evidence file. Real system, no mocks. An
#   unreachable API => SKIP-with-reason (§11.4.3), never a fabricated PASS.
# ==============================================================================

set -u

# ------------------------------------------------------------------------------
# Configuration
# ------------------------------------------------------------------------------
BASE_URL="${CATALOGIZER_BASE_URL:-http://127.0.0.1:8080}"
SHOW_TITLE="${CATALOGIZER_SHOW_TITLE:-Breaking Bad}"

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
RESULTS_DIR="${CATALOGIZER_RESULTS_DIR:-qa-results/episode_titles_dedup/${TS}}"
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

# json_child_ids <file> -> newline-separated list of item ids in a children body
json_child_ids() {
  jc_file="$1"
  python3 - "${jc_file}" <<'PY' 2>/dev/null
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

# titles_contains_exact <file> <exact-title> -> exit 0 if present
titles_contains_exact() {
  tc_file="$1"; tc_title="$2"
  json_titles "${tc_file}" | grep -Fxq -- "${tc_title}"
}

echo "=============================================================="
echo " Catalogizer episode-titles + dedup guard (DEFECT-H + DEFECT-I)"
echo " base_url   : ${BASE_URL}"
echo " show       : ${SHOW_TITLE}"
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
  printf '{"pass":%s,"fail":%s,"skip":%s,"base_url":"%s","results_dir":"%s","ts":"%s","guard":"DEFECT-H+DEFECT-I-episode-titles-dedup"%s}\n' \
    "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" "${BASE_URL}" "${RESULTS_DIR}" "${TS}" "${es_extra_json}" \
    > "${SUMMARY_JSON}"
}

# skip_all <reason> — SKIP every assertion in the current shell (counters
# increment here, never in a subshell — mirrors the reference for-loop style).
skip_all() {
  sa_reason="$1"
  for lbl in \
    "ETD-A no-duplicate-episodes" \
    "ETD-B real-human-episode-title" \
    "ETD-C episode-N-fallback-accepted" \
    "ETD-D episode-count-determinism"; do
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
# Resolve the show -> its season children -> each season's episode children.
# Shared by ETD-A / ETD-B / ETD-C / ETD-D.
# ------------------------------------------------------------------------------
echo
echo "--- Resolving '${SHOW_TITLE}' via GET /api/v1/entities/browse/tv_show ---"
CODE="$(http_get /api/v1/entities/browse/tv_show yes etd_browse_tv)"
EV_TV="${RESULTS_DIR}/etd_browse_tv.json"
SHOW_ID=""
if [ "${CODE}" != "200" ]; then
  ab_fail "ETD resolve tv_show-browse: expected 200, got ${CODE}" "${EV_TV}"
elif ! titles_contains_exact "${EV_TV}" "${SHOW_TITLE}"; then
  ab_fail "ETD resolve: no tv_show entity titled '${SHOW_TITLE}'" "${EV_TV}"
else
  SHOW_ID="$(id_for_title "${EV_TV}" "${SHOW_TITLE}")"
  echo "Resolved '${SHOW_TITLE}' -> entity id=${SHOW_ID}"
fi

# Resolve season children of the show.
SEASON_IDS=""
if [ -n "${SHOW_ID}" ]; then
  echo
  echo "--- Resolving season children of '${SHOW_TITLE}' ---"
  CODE="$(http_get "/api/v1/entities/${SHOW_ID}/children" yes etd_seasons)"
  EV_SEASONS="${RESULTS_DIR}/etd_seasons.json"
  if [ "${CODE}" != "200" ]; then
    ab_fail "ETD resolve seasons: expected 200 from children, got ${CODE}" "${EV_SEASONS}"
  else
    SEASON_IDS="$(json_child_ids "${EV_SEASONS}")"
  fi
fi

# For each season, fetch its episode children and append per-episode detail
# (id, title, episode_number) into a single newline-delimited evidence file.
# Lines are TAB-separated: <season_id>\t<episode_id>\t<episode_number>\t<title>
EP_FACTS="${RESULTS_DIR}/etd_episode_facts.tsv"
: > "${EP_FACTS}"
EP_DETAIL_IDX=0
if [ -n "${SEASON_IDS}" ]; then
  echo
  echo "--- Walking episodes under each season (capturing episode_number + title) ---"
  for sid in ${SEASON_IDS}; do
    CODE="$(http_get "/api/v1/entities/${sid}/children" yes "etd_eps_season_${sid}")"
    EV_EPS="${RESULTS_DIR}/etd_eps_season_${sid}.json"
    [ "${CODE}" = "200" ] || continue
    for eid in $(json_child_ids "${EV_EPS}"); do
      EP_DETAIL_IDX=$((EP_DETAIL_IDX + 1))
      http_get "/api/v1/entities/${eid}" yes "etd_ep_detail_${EP_DETAIL_IDX}" >/dev/null
      EV_DET="${RESULTS_DIR}/etd_ep_detail_${EP_DETAIL_IDX}.json"
      EP_MT="$(json_field "${EV_DET}" media_type)"
      # Only consider entities that genuinely resolve to tv_episode.
      [ "${EP_MT}" = "tv_episode" ] || continue
      EP_NUM="$(json_field "${EV_DET}" episode_number)"
      EP_TITLE="$(json_field "${EV_DET}" title)"
      printf '%s\t%s\t%s\t%s\n' "${sid}" "${eid}" "${EP_NUM}" "${EP_TITLE}" >> "${EP_FACTS}"
    done
  done
fi

# ------------------------------------------------------------------------------
# (A) ETD-A — DEFECT-H: NO duplicate (season_id, episode_number) pair.
# ------------------------------------------------------------------------------
echo
echo "--- ETD-A (DEFECT-H): no two episodes share (season, episode_number) ---"
if [ -z "${SHOW_ID}" ]; then
  ab_skip_with_reason "ETD-A no-duplicate-episodes" "no_show_id: '${SHOW_TITLE}' not resolved"
elif [ ! -s "${EP_FACTS}" ]; then
  ab_fail "ETD-A no-duplicate-episodes: no tv_episode entities found under '${SHOW_TITLE}'" "${EP_FACTS}"
else
  DUP_PAIRS="$(python3 - "${EP_FACTS}" <<'PY' 2>/dev/null
import sys
from collections import Counter
pairs = []
for line in open(sys.argv[1], encoding="utf-8"):
    parts = line.rstrip("\n").split("\t")
    if len(parts) < 3:
        continue
    season_id, _episode_id, episode_number = parts[0], parts[1], parts[2]
    # An empty / non-numeric episode_number is itself a defect-class signal.
    pairs.append((season_id, episode_number))
counts = Counter(pairs)
dups = [f"{s}:#{n}(x{c})" for (s, n), c in counts.items() if c > 1]
print(",".join(dups))
PY
)"
  if [ -n "${DUP_PAIRS}" ]; then
    ab_fail "ETD-A no-duplicate-episodes: DEFECT-H duplicate (season,episode_number) pair(s) present: ${DUP_PAIRS}" "${EP_FACTS}"
  else
    EP_TOTAL="$(grep -c . "${EP_FACTS}")"
    ab_pass_with_evidence "ETD-A no-duplicate-episodes: every (season,episode_number) pair unique across ${EP_TOTAL} episodes (DEFECT-H signature absent)" "${EP_FACTS}"
  fi
fi

# ------------------------------------------------------------------------------
# (B) ETD-B — DEFECT-I: at least one episode carries a real human title (NOT
#     matching `^Episode [0-9]+$`). Breaking Bad S01E01 "Pilot" is the fixture.
# ------------------------------------------------------------------------------
echo
echo "--- ETD-B (DEFECT-I): at least one real human episode title present ---"
if [ ! -s "${EP_FACTS}" ]; then
  ab_skip_with_reason "ETD-B real-human-episode-title" "no_episodes: no tv_episode entities resolved"
else
  HUMAN_TITLES="$(python3 - "${EP_FACTS}" <<'PY' 2>/dev/null
import sys, re
generic = re.compile(r'^Episode\s+[0-9]+$')
human = []
for line in open(sys.argv[1], encoding="utf-8"):
    parts = line.rstrip("\n").split("\t")
    if len(parts) < 4:
        continue
    title = parts[3]
    if title and not generic.match(title.strip()):
        human.append(title)
# Emit the distinct human titles (capped) as positive evidence.
seen = []
for t in human:
    if t not in seen:
        seen.append(t)
print(" | ".join(seen[:10]))
PY
)"
  if [ -n "${HUMAN_TITLES}" ]; then
    ab_pass_with_evidence "ETD-B real-human-episode-title: at least one episode has a human title (not '^Episode N$'): ${HUMAN_TITLES}" "${EP_FACTS}"
  else
    ab_fail "ETD-B real-human-episode-title: every episode title matched '^Episode N$' — DEFECT-I (human title from filename, e.g. 'Pilot', not carried through)" "${EP_FACTS}"
  fi
fi

# ------------------------------------------------------------------------------
# (C) ETD-C — fallback honesty: an episode with NO human title in its filename
#     may legitimately be titled "Episode N". Assert that pattern is ACCEPTED
#     (the guard does not false-FAIL genuinely-untitled episodes). This PASSes
#     whether or not such an episode exists — it asserts the classifier is
#     tolerant of the legitimate fallback, never that a fallback MUST exist.
# ------------------------------------------------------------------------------
echo
echo "--- ETD-C (fallback honesty): 'Episode N' titles are an ACCEPTED fallback ---"
if [ ! -s "${EP_FACTS}" ]; then
  ab_skip_with_reason "ETD-C episode-N-fallback-accepted" "no_episodes: no tv_episode entities resolved"
else
  FALLBACK_REPORT="$(python3 - "${EP_FACTS}" <<'PY' 2>/dev/null
import sys, re
generic = re.compile(r'^Episode\s+[0-9]+$')
fallback = []
malformed = []
for line in open(sys.argv[1], encoding="utf-8"):
    parts = line.rstrip("\n").split("\t")
    if len(parts) < 4:
        continue
    title = (parts[3] or "").strip()
    if not title:
        # An empty title is neither a human title nor the accepted fallback.
        malformed.append("<empty>")
    elif generic.match(title):
        fallback.append(title)
# The guard ACCEPTS the 'Episode N' fallback (it is legitimate). Only a
# genuinely malformed/empty title is reportable here.
print("ACCEPTED=%d|MALFORMED=%d|%s" % (
    len(fallback), len(malformed), ",".join(sorted(set(fallback))[:10])))
PY
)"
  MALFORMED_N="$(printf '%s' "${FALLBACK_REPORT}" | sed -n 's/.*MALFORMED=\([0-9]*\).*/\1/p')"
  if [ -n "${MALFORMED_N}" ] && [ "${MALFORMED_N}" -gt 0 ]; then
    ab_fail "ETD-C episode-N-fallback-accepted: ${MALFORMED_N} episode(s) have an empty/malformed title (neither human nor 'Episode N')" "${EP_FACTS}"
  else
    ab_pass_with_evidence "ETD-C episode-N-fallback-accepted: 'Episode N' is an accepted fallback; no malformed/empty titles (${FALLBACK_REPORT})" "${EP_FACTS}"
  fi
fi

# ------------------------------------------------------------------------------
# (D) ETD-D — count sanity + determinism (§11.4.50): episodes present, and the
#     de-duplicated tv_episode count under the show is stable across two reads.
# ------------------------------------------------------------------------------
echo
echo "--- ETD-D (determinism §11.4.50): episode count stable across two reads ---"
if [ -z "${SHOW_ID}" ]; then
  ab_skip_with_reason "ETD-D episode-count-determinism" "no_show_id: '${SHOW_TITLE}' not resolved"
else
  COUNT1="$(grep -c . "${EP_FACTS}" 2>/dev/null || printf '0')"
  # Second independent read of the same hierarchy.
  EP_FACTS2="${RESULTS_DIR}/etd_episode_facts_read2.tsv"
  : > "${EP_FACTS2}"
  http_get "/api/v1/entities/${SHOW_ID}/children" yes etd_seasons_read2 >/dev/null
  EV_SEASONS2="${RESULTS_DIR}/etd_seasons_read2.json"
  EP_DETAIL2_IDX=0
  for sid in $(json_child_ids "${EV_SEASONS2}"); do
    http_get "/api/v1/entities/${sid}/children" yes "etd_eps2_season_${sid}" >/dev/null
    for eid in $(json_child_ids "${RESULTS_DIR}/etd_eps2_season_${sid}.json"); do
      EP_DETAIL2_IDX=$((EP_DETAIL2_IDX + 1))
      http_get "/api/v1/entities/${eid}" yes "etd_ep2_detail_${EP_DETAIL2_IDX}" >/dev/null
      MT2="$(json_field "${RESULTS_DIR}/etd_ep2_detail_${EP_DETAIL2_IDX}.json" media_type)"
      [ "${MT2}" = "tv_episode" ] || continue
      printf 'x\n' >> "${EP_FACTS2}"
    done
  done
  COUNT2="$(grep -c . "${EP_FACTS2}" 2>/dev/null || printf '0')"
  if [ "${COUNT1}" -ge 1 ] && [ "${COUNT1}" = "${COUNT2}" ]; then
    ab_pass_with_evidence "ETD-D episode-count-determinism: ${COUNT1} tv_episode entities present + count stable across two reads (read1=${COUNT1}, read2=${COUNT2})" "${EP_FACTS}"
  else
    ab_fail "ETD-D episode-count-determinism: count not stable/>=1 (read1=${COUNT1}, read2=${COUNT2})" "${EP_FACTS}"
  fi
fi

# ------------------------------------------------------------------------------
# Summary
# ------------------------------------------------------------------------------
{
  printf 'PASS=%s FAIL=%s SKIP=%s\n' "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}"
  printf '%b' "${SUMMARY_ROWS}"
} > "${SUMMARY_TXT}"

printf '{"pass":%s,"fail":%s,"skip":%s,"base_url":"%s","results_dir":"%s","ts":"%s","show":"%s","guard":"DEFECT-H+DEFECT-I-episode-titles-dedup"}\n' \
  "${PASS_COUNT}" "${FAIL_COUNT}" "${SKIP_COUNT}" "${BASE_URL}" "${RESULTS_DIR}" "${TS}" "${SHOW_TITLE}" \
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
