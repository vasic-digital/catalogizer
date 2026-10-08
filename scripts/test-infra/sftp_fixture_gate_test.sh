#!/usr/bin/env bash
# sftp_fixture_gate_test.sh - tests, without starting any container, two properties of sftp_fixture.sh added in fix-r2 (review SFTP-19):
#   1. SFTP_FIXTURE_SRC is refused unless SFTP_FIXTURE_ALLOW_SRC=1 (`REFUSED reason=src_override_not_declared`);
#   2. the "tested tree sha256" the script prints is the hash of exactly the tree it copied: it equals an independent computation over the
#      source directory and changes when one byte of one file changes.
#   3. (fix-r3, review WF24 S06) the stop hook SFTP_FIXTURE_STOP_AFTER_HASH=1 exists in selftest mode only and exits 3; in `run` mode it is
#      REFUSED before anything is created (exit 1, nothing on stdout, no log file), so an inherited variable can never turn a test run into a silent exit 0.
# Exit 0 = all checks passed; 1 = a check failed (each failure is printed). Uses the script's test hook SFTP_FIXTURE_STOP_AFTER_HASH=1.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
FIX="${SFTP_FIXTURE_UNDER_TEST:-$HERE/sftp_fixture.sh}"
TMP="$(mktemp -d "${TMPDIR:-/tmp}/sftp-gate-test.XXXXXX")" || exit 1
trap 'rm -rf -- "$TMP"' EXIT
FAILS=0; CHECKS=0
check() { CHECKS=$((CHECKS + 1)); if [ "$2" = ok ]; then echo "ok   $1"; else echo "FAIL $1 ($2)"; FAILS=$((FAILS + 1)); fi; }

mkdir -p "$TMP/src/sub" "$TMP/src/pkg/sftp"
printf 'alpha\n' >"$TMP/src/a.txt"; printf 'beta\n' >"$TMP/src/sub/b.txt"; printf 'package sftp\n' >"$TMP/src/pkg/sftp/x.go"
indep() { ( cd "$1" && find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum | awk '{print $1}' ); }

# 1. refused without the declaration
out="$(SFTP_FIXTURE_SRC="$TMP/src" "$FIX" selftest 2>&1 >/dev/null)"; rc=$?
[ "$rc" = 1 ] && printf '%s' "$out" | grep -q 'REFUSED reason=src_override_not_declared' && check "SFTP_FIXTURE_SRC without ALLOW is refused" ok || check "SFTP_FIXTURE_SRC without ALLOW is refused" "rc=$rc out=$out"
# control: the declaration passes that gate (a missing directory is then refused for ITS reason)
out="$(SFTP_FIXTURE_SRC="$TMP/missing" SFTP_FIXTURE_ALLOW_SRC=1 "$FIX" selftest 2>&1 >/dev/null)"; rc=$?
[ "$rc" = 1 ] && printf '%s' "$out" | grep -q 'REFUSED reason=src_module_missing' && check "with ALLOW the gate passes (next refusal is src_module_missing)" ok || check "with ALLOW the gate passes" "rc=$rc out=$out"
# an empty ALLOW or another value is not a declaration
out="$(SFTP_FIXTURE_SRC="$TMP/src" SFTP_FIXTURE_ALLOW_SRC=yes "$FIX" selftest 2>&1 >/dev/null)"; rc=$?
[ "$rc" = 1 ] && printf '%s' "$out" | grep -q 'src_override_not_declared' && check "ALLOW must be exactly 1" ok || check "ALLOW must be exactly 1" "rc=$rc out=$out"

# 2. the printed hash is the hash of the tree
printed() { printf '%s' "$1" | sed -n 's/^sftp-fixture: tested tree sha256=\([0-9a-f]\{64\}\) pkg\/sftp sha256=[0-9a-f]\{64\} .*/\1/p'; }
printedpkg() { printf '%s' "$1" | sed -n 's/^sftp-fixture: tested tree sha256=[0-9a-f]\{64\} pkg\/sftp sha256=\([0-9a-f]\{64\}\) .*/\1/p'; }
out="$(SFTP_FIXTURE_SRC="$TMP/src" SFTP_FIXTURE_ALLOW_SRC=1 SFTP_FIXTURE_STOP_AFTER_HASH=1 "$FIX" selftest 2>&1 >/dev/null)"; rc=$?
h1="$(printed "$out")"; e1="$(indep "$TMP/src")"; p1="$(printedpkg "$out")"; pe1="$(indep "$TMP/src/pkg/sftp")"
[ "$rc" = 3 ] && [ -n "$h1" ] && [ "$h1" = "$e1" ] && check "printed hash equals the independent hash of the source tree" ok || check "printed hash equals the independent hash" "rc=$rc printed=$h1 expected=$e1 out=$out"
[ -n "$p1" ] && [ "$p1" = "$pe1" ] && check "printed pkg/sftp hash equals the independent hash of the package directory" ok || check "printed pkg/sftp hash" "printed=$p1 expected=$pe1"
printf 'alphb\n' >"$TMP/src/a.txt"
out="$(SFTP_FIXTURE_SRC="$TMP/src" SFTP_FIXTURE_ALLOW_SRC=1 SFTP_FIXTURE_STOP_AFTER_HASH=1 "$FIX" selftest 2>&1 >/dev/null)"
h2="$(printed "$out")"; e2="$(indep "$TMP/src")"
[ -n "$h2" ] && [ "$h2" = "$e2" ] && [ "$h2" != "$h1" ] && check "one changed byte changes the printed hash (and it still equals the independent one)" ok || check "one changed byte changes the hash" "h1=$h1 h2=$h2 e2=$e2"
# the checked-in module path (no override): the printed hashes equal the documented recomputation commands
out="$(SFTP_FIXTURE_STOP_AFTER_HASH=1 "$FIX" selftest 2>&1 >/dev/null)"; rc=$?
h3="$(printed "$out")"; e3="$(cd "$HERE/../../submodules/filesystem" && find . -path ./.git -prune -o -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum | awk '{print $1}')"
p3="$(printedpkg "$out")"; pe3="$(cd "$HERE/../../submodules/filesystem/pkg/sftp" && find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum | sha256sum | awk '{print $1}')"
[ "$rc" = 3 ] && [ -n "$p3" ] && [ "$p3" = "$pe3" ] && check "checked-in module: printed pkg/sftp hash equals the documented command" ok || check "checked-in module pkg/sftp hash" "rc=$rc printed=$p3 expected=$pe3"
# the whole-tree hash of the checked-in module is informational only: other work edits other packages of the module while this runs
if [ "$h3" = "$e3" ]; then echo "info: printed whole-tree hash of the checked-in module equals the documented command"; else echo "info: whole-tree hash differs (the module is shared with other work and changed while the check ran); only the pkg/sftp hash is a check"; fi
# 3. the stop hook is a selftest-only hook: in `run` mode it is refused, with no output on stdout, no log file and no scratch directory
out="$(SFTP_FIXTURE_SRC="$TMP/src" SFTP_FIXTURE_ALLOW_SRC=1 SFTP_FIXTURE_STOP_AFTER_HASH=1 "$FIX" run --log "$TMP/run.log" -- -race -run Integration -v ./pkg/sftp/ 2>"$TMP/run.err" )"; rc=$?
[ "$rc" = 1 ] && [ -z "$out" ] && [ ! -e "$TMP/run.log" ] && grep -q 'REFUSED reason=stop_hook_only_in_selftest' "$TMP/run.err" && check "the stop hook is refused in run mode (exit 1, no stdout, no log)" ok || check "the stop hook is refused in run mode" "rc=$rc out=$out err=$(cat "$TMP/run.err")"
# control: the same run without the hook gets past that gate (its next refusal or work is not the hook's)
out="$(SFTP_FIXTURE_SRC="$TMP/missing" SFTP_FIXTURE_ALLOW_SRC=1 "$FIX" run -- -v ./pkg/sftp/ 2>&1 >/dev/null)"
if printf '%s' "$out" | grep -q 'stop_hook_only_in_selftest'; then check "without the hook the run mode does not refuse for it" "$out"; else check "without the hook the run mode does not refuse for it" ok; fi
# selftest exit code of the hook is the distinct 3, never 0
SFTP_FIXTURE_SRC="$TMP/src" SFTP_FIXTURE_ALLOW_SRC=1 SFTP_FIXTURE_STOP_AFTER_HASH=1 "$FIX" selftest >/dev/null 2>&1; rc=$?
[ "$rc" = 3 ] && check "selftest stopped by the hook exits 3" ok || check "selftest stopped by the hook exits 3" "rc=$rc"
# nothing was left behind (the script removes its scratch directory on every exit)
left="$(ls -d "$HERE/../../.audit/scratch"/catalogizer-sftp-* 2>/dev/null | wc -l)"
[ "$left" = 0 ] && check "no scratch directory left behind" ok || check "no scratch directory left behind" "left=$left"
echo "sftp_fixture_gate_test: $CHECKS checks, $FAILS failed"
[ "$FAILS" = 0 ]
