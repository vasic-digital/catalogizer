#!/usr/bin/env bash
# lib.sh - shared helpers of the scripts/longops tests (WP-08, T088/T089). Real processes, real flock, real /proc, scratch state only:
# no test writes the real .audit/. HOST-SIDE RUN NOTICE (UNCONFIRMED container leg): RUNP/IMG-TESTUTIL (T007/T008) do not exist yet,
# so the tests run on the host with coreutils, jq, flock and git; the identity header says so.
TROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
S=${LONGOPS_SCRIPTS:-$TROOT/scripts/longops}
PASS=0; FAIL=0; KILLME=(); KILL9ME=()   # KILL9ME: a process a test made ignore TERM on purpose (killed with KILL, one pid, never a group)
FX=$(mktemp -d "${TMPDIR:-/tmp}/longops-test.XXXXXX")
cleanup() { local p; for p in "${KILLME[@]:-}"; do [[ "$p" =~ ^[0-9]+$ && "$p" -gt 1 ]] && kill "$p" 2>/dev/null; done; for p in "${KILL9ME[@]:-}"; do [[ "$p" =~ ^[0-9]+$ && "$p" -gt 1 ]] && kill -9 "$p" 2>/dev/null; done; rm -rf "$FX"; }
trap cleanup EXIT
ok()  { PASS=$((PASS+1)); echo "ok   $*"; }
bad() { FAIL=$((FAIL+1)); echo "FAIL $*"; }
assert_eq() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 :: got [$2] want [$3]"; fi; }
assert_rc() { if [ "$2" -eq "$3" ]; then ok "$1"; else bad "$1 :: rc=$2 want $3"; fi; }
finish() { echo "RESULT pass=$PASS fail=$FAIL"; [ "$FAIL" -eq 0 ]; }
ident_header() {
  echo "# identity: task=${1:-?} utc=$(date -u +%Y-%m-%dT%H:%M:%SZ) host=$(hostname -s) uid=$(id -u) scripts=$S"
  echo "# git_head=$(git -C "$TROOT" rev-parse HEAD 2>/dev/null) (scripts and tests are uncommitted: content hashes below identify them)"
  local f; for f in lib register acquire release heartbeat holder classify reap check_no_build_writing_tracked require_verdicts; do
    printf '# sha256 %s.sh=%s\n' "$f" "$(sha256sum "$S/$f.sh" 2>/dev/null | cut -c1-64)"; done
  echo "# jq=$(jq --version) flock=$(flock --version 2>&1 | head -1) container_image_digest=UNCONFIRMED (RUNP/IMG-TESTUTIL absent: host-side run, T007/T008 pending)"
}
# fixture: new isolated state; sets LONGOPS_* for the process
newfx() {
  FXN=$FX/$1; rm -rf "$FXN"; mkdir -p "$FXN/repo/.audit" "$FXN/repo/specs/001-full-project-audit-remediation/evidence" "$FXN/repo/specs/001-full-project-audit-remediation/audit" "$FXN/repo/tracked"
  export LONGOPS_REPO=$FXN/repo LONGOPS_DIR=$FXN/repo/.audit/longops LONGOPS_AUDIT=$FXN/repo/.audit LONGOPS_ALLOW_TMPFS=1
  unset LONGOPS_NOW LONGOPS_TEST_SLEEP_IN_CS CPA_APPROVED_DIR CPA_RUN CPA_RUN_ID LONGOPS_PODMAN LONGOPS_BUILDS LONGOPS_EV LONGOPS_AUD LONGOPS_RESUME_TTL
  git -C "$FXN/repo" init -q 2>/dev/null; echo t >"$FXN/repo/tracked/a.txt"
  git -C "$FXN/repo" -c user.email=t@t -c user.name=t add tracked/a.txt; git -C "$FXN/repo" -c user.email=t@t -c user.name=t commit -qm fx
}
# mkll: start a sleeper, set LL to its pid (fds redirected: never inside a $(...) capture, the 11.4.201(12) pipe footgun)
mkll() { sleep 600 >/dev/null 2>&1 & LL=$!; KILLME+=("$LL"); }
pstart() { sed 's/^.*) //' "/proc/$1/stat" | cut -d' ' -f20; }
