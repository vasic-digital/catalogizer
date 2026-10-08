#!/usr/bin/env bash
# usage: rt.sh <logname> '<go test args as one string>'   (runs in IMG-GO, tees to evidence dir)
cd /home/milosvasic/Projects/catalogizer
export TMPDIR=/dev/shm DISK_HEADROOM_REPO_ROOT=$PWD LONGOPS_ALLOW_TMPFS=1
log=$1; args=$2
EV=specs/001-full-project-audit-remediation/evidence/wp12/scanner
bash scripts/containers/run_pinned.sh IMG-GO -- bash -c "cd catalog-api && set -o pipefail; env GOMAXPROCS=3 GOTOOLCHAIN=local CGO_ENABLED=1 go test $args 2>&1 | grep -v '^go: downloading'; echo go_test_rc=\${PIPESTATUS[0]}" > $EV/$log 2>&1
tail -${3:-6} $EV/$log
