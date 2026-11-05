#!/usr/bin/env bash
# run.sh - live S3 evidence runner. Reads credentials from ${S3_ENV:?} (0600) in this process only; never prints them.
set -uo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"; REPO="$(cd "$HERE/../../.." && pwd)"
S3_ENV="${S3_ENV:-$HOME/.cache/claude-tmp/catqa-s3live/s3.env}"; : "${S3_ENV:?}"
OUT="${OUT:-$HERE}"; : "${OUT:?}"
GC="${GOCACHE_DIR:?set GOCACHE_DIR to a scratch dir}"
t() { ( set -a; . "$S3_ENV"; set +a; cd "$REPO/catalog-api" && GOMAXPROCS=3 GOCACHE="$GC" systemd-run --user --scope -q -p MemoryMax=3G -p MemorySwapMax=0 go test -tags s3live -run TestS3Live -count=1 -v ./filesystem/ ) ; }
mode="${1:-green}"
case "$mode" in
 green) for i in 1 2 3; do t > "$OUT/green_run$i.txt" 2>&1; echo "run$i exit=$?" >> "$OUT/green_exits.txt"; done ;;
 skip) ( env -u S3_LIVE_ENDPOINT -u S3_LIVE_ACCESS_KEY -u S3_LIVE_SECRET_KEY -u S3_LIVE_BUCKET bash -c "cd '$REPO/catalog-api' && GOCACHE='$GC' go test -tags s3live -run TestS3Live -count=1 -v ./filesystem/" ) > "$OUT/skip_control.txt" 2>&1; echo "exit=$?" >> "$OUT/skip_control.txt" ;;
 red) t > "$OUT/red_run.txt" 2>&1; echo "exit=$?" >> "$OUT/red_run.txt" ;;
esac
