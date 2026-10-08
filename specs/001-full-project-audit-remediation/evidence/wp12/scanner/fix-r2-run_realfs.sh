#!/usr/bin/env bash
# usage: [WORK=<repo-relative catalog-api dir>] [RACE=-race] fix-r2-run_realfs.sh <logname> [test flags...]      (WF22 fix round 2)
# builds the realfs test binary in IMG-GO (rootless, caches under the scratch out dir) and runs it on the wf22fix stack (ftp + webdav containers)
# prerequisite: scripts/test-infra/up.sh --build-id wf22fix --services ftp,webdav
# WORK defaults to catalog-api; the RED runs set it to the pre-fix snapshot under .audit/scratch/wf22fix/prefix/catalog-api
cd /home/milosvasic/Projects/catalogizer || exit 9
export TMPDIR=/dev/shm DISK_HEADROOM_REPO_ROOT=$PWD LONGOPS_ALLOW_TMPFS=1
EV=specs/001-full-project-audit-remediation/evidence/wp12/scanner
OUT=$PWD/.audit/scratch/wf22fix/out
log=$1; shift
rm -f .audit/scratch/wf22fix/realfs.test
bash scripts/containers/run_pinned.sh --rw .audit/scratch --out "$OUT" IMG-GO -- bash -c "cd ${WORK:-catalog-api} && export GOTOOLCHAIN=local HOME=/out/home GOCACHE=/out/gocache GOMODCACHE=/out/gomod GOMAXPROCS=2 CGO_ENABLED=1 CGO_CFLAGS=-w && go test -tags realfs ${RACE:-} -c -o /src/.audit/scratch/wf22fix/realfs.test ./tests/realfs/ 2>&1 | grep -v '^go: downloading'"
[ -x .audit/scratch/wf22fix/realfs.test ] || { echo BUILD_FAILED; exit 9; }
bash scripts/test-infra/run_client.sh --build-id wf22fix -- /src/.audit/scratch/wf22fix/realfs.test -test.v -test.count=1 "$@" > "$EV/$log" 2>&1
rc=$?
# the per-run credentials of the throw-away stack are printed by tests that show a leaked reason: never keep them in evidence
sed -i -E 's#(https?://)[^@/[:space:]"\\]+@#\1REDACTED-IN-EVIDENCE@#g' "$EV/$log"
echo "realfs_rc=$rc" >> "$EV/$log"
grep -E "^(--- |FAIL|PASS|ok|realfs_rc)|realfs |realdb " "$EV/$log"
