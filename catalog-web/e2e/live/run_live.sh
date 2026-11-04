#!/usr/bin/env bash
# Run the LIVE catalog-web specs N times for one project and build the evidence records.
# usage: run_live.sh <project: chromium-desktop|chromium-390|firefox-desktop|firefox-390> [runs=3]
# env (required): ADMIN_USERNAME ADMIN_PASSWORD (never printed), LIVE_CORPUS_MANIFEST (host path of the corpus manifest.json)
# env (defaults): PLAYWRIGHT_BASE_URL=http://127.0.0.1:33020  LIVE_API_URL=http://127.0.0.1:38082  LIVE_EVIDENCE_OUT=<repo>/docs/qa/web-live-<date>
#                 LIVE_EXPECTED_WEB_IMAGE_DIGEST (pinned digest of the web image the claim names)
set -u
project=${1:?project}; runs=${2:-3}
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd); web=$(cd "$here/../.." && pwd)
: "${ADMIN_USERNAME:?}" "${ADMIN_PASSWORD:?}" "${LIVE_CORPUS_MANIFEST:?}"
export PLAYWRIGHT_BASE_URL=${PLAYWRIGHT_BASE_URL:-http://127.0.0.1:33020} LIVE_API_URL=${LIVE_API_URL:-http://127.0.0.1:38082}
out=${LIVE_EVIDENCE_OUT:-$web/../docs/qa/web-live-$(date +%Y%m%d)}; mkdir -p "${out:?}/$project"
cd "$web" || exit 2
for i in $(seq 1 "$runs"); do
  d="$out/$project/run$i"; mkdir -p "${d:?}/obs"
  LIVE_OBS_DIR="$d/obs" LIVE_JSON_REPORT="$d/report.json" LIVE_OUTPUT_DIR="$d/artifacts" \
    npx playwright test -c playwright.live.config.ts --project="$project" ${LIVE_SPEC_ARGS:-} > "$d/console.log" 2>&1
  echo "run$i exit=$?" | tee -a "$out/$project/runs.log"
done
node e2e/live/make_evidence.mjs "$out/$project" "$out/$project/evidence" "$PLAYWRIGHT_BASE_URL" "$LIVE_API_URL" "$project"
