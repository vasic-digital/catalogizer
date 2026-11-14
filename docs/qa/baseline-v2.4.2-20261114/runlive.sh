#!/bin/bash
W=/home/milosvasic/.cache/claude-tmp/qabase.1mSVyT
set -a; . /home/milosvasic/.cache/claude-tmp/catqab/qa.env; set +a
unset JWT_SECRET POSTGRES_PASSWORD METRICS_SCRAPE_TOKEN
export PLAYWRIGHT_BASE_URL=http://127.0.0.1:33090 LIVE_API_URL=http://127.0.0.1:38090 LIVE_WEB_CONTAINER=catqab-web LIVE_API_CONTAINER=catqab-api
export LIVE_CORPUS_MANIFEST=/home/milosvasic/.cache/claude-tmp/media-corpus/manifest.json
export LIVE_EXPECTED_WEB_IMAGE_DIGEST=sha256:001b83754767da9b802d851627a1ea1f721b38964bc48d478d5dc33003fd706a
export LIVE_CORPUS_ROOT_PATH=/media LIVE_EVIDENCE_OUT=$W/evidence/web-live GOMAXPROCS=2
cd $W/repo/catalog-web
exec nice -n 10 systemd-run --user --scope -q -p MemoryMax=6G -p MemorySwapMax=0 --setenv=ADMIN_USERNAME --setenv=ADMIN_PASSWORD --setenv=PLAYWRIGHT_BASE_URL --setenv=LIVE_API_URL --setenv=LIVE_WEB_CONTAINER --setenv=LIVE_API_CONTAINER --setenv=LIVE_CORPUS_MANIFEST --setenv=LIVE_EXPECTED_WEB_IMAGE_DIGEST --setenv=LIVE_EVIDENCE_OUT --setenv=LIVE_CORPUS_ROOT_PATH --setenv=PATH="$PATH" --setenv=HOME="$HOME" bash e2e/live/run_live.sh "$@"
