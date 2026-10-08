#!/usr/bin/env bash
# usage: realfs.sh <logname> [extra test flags]   builds the realfs test binary (IMG-GO) and runs it on the wp12scan stack
cd /home/milosvasic/Projects/catalogizer
export TMPDIR=/dev/shm DISK_HEADROOM_REPO_ROOT=$PWD LONGOPS_ALLOW_TMPFS=1
EV=specs/001-full-project-audit-remediation/evidence/wp12/scanner
log=$1; shift
rm -f .audit/scratch/wp12scanner/realfs.test; bash scripts/containers/run_pinned.sh --rw .audit/scratch IMG-GO -- bash -c 'cd catalog-api && env GOMAXPROCS=3 GOTOOLCHAIN=local CGO_ENABLED=1 go test -tags realfs '"${RACE:-}"' -c -o /src/.audit/scratch/wp12scanner/realfs.test ./tests/realfs/ 2>&1 | grep -v "^go: downloading\|sqlite3\|^ *[0-9|]\|\^~\|go-sqlcipher"'
[ -x .audit/scratch/wp12scanner/realfs.test ] || { echo BUILD_FAILED; exit 9; }
bash scripts/test-infra/run_client.sh --build-id wp12scan -- /src/.audit/scratch/wp12scanner/realfs.test -test.v -test.count=1 "$@" > $EV/$log 2>&1
echo "realfs_rc=$?" >> $EV/$log
grep -E "^(--- |FAIL|PASS|ok|realfs_rc)|realfs (ftp|webdav)" $EV/$log
