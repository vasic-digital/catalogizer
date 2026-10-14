#!/usr/bin/env bash
# usage: run_go.sh <logfile-abs> '<go args>' [tail-lines]   env WORK = catalog-api copy relative to the repo root (default catalog-api)
# Runs `go <args>` inside the pinned rootless IMG-GO container (scripts/containers/run_pinned.sh), one container, GOMAXPROCS=1.
cd "$(dirname "$0")/../.." || exit 9
export TMPDIR=/dev/shm DISK_HEADROOM_REPO_ROOT=$PWD LONGOPS_ALLOW_TMPFS=1 RUNP_MEMORY=${RUNP_MEMORY:-3221225472}
log=$1; args=$2
OUT=$PWD/.audit/scratch/wf22fix/out
bash scripts/containers/run_pinned.sh --out "$OUT" IMG-GO -- bash -c "cd ${WORK:-catalog-api} && export GOTOOLCHAIN=local HOME=/out/home GOCACHE=/out/gocache GOMODCACHE=/out/gomod GOMAXPROCS=1 CGO_ENABLED=1 CGO_CFLAGS=-w; set -o pipefail; go $args 2>&1 | grep -v '^go: downloading'; echo go_rc=\${PIPESTATUS[0]}" > "$log" 2>&1
tail -"${3:-8}" "$log"
