#!/usr/bin/env bash
# Test of the log-check logic of nfs_emulator_live.sh (NFS3W-FU-m2) against sample go test -v logs. No container needed.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "${CHECK_LIB:-$HERE/nfs_emulator_check.sh}"
T="$(mktemp -d)"; trap 'rm -rf "${T:?}"' EXIT
fail=0
ok() { if [ "$1" = "$2" ]; then echo "PASS $3"; else echo "FAIL $3 (got $1 want $2)"; fail=1; fi; }
{ for t in $(nfs_emulator_required_tests); do echo "=== RUN   $t"; echo "--- PASS: $t (0.01s)"; done; echo PASS; } >"$T/pass.log"
{ for t in $(nfs_emulator_required_tests | head -4); do echo "--- PASS: $t (0.01s)"; done
  echo "=== RUN   TestNFS3FactoryLifecycleLive"; echo "--- SKIP: TestNFS3FactoryLifecycleLive (0.00s)"; } >"$T/skip.log"
{ for t in $(nfs_emulator_required_tests | head -4); do echo "--- PASS: $t (0.01s)"; done
  echo "--- SKIP: TestNFS3FactoryLifecycleLive (0.00s)"; echo "    --- PASS: TestNFS3FactoryLifecycleLive/sub (0.00s)"; } >"$T/subonly.log"
{ for t in $(nfs_emulator_required_tests | tail -n +2); do echo "--- PASS: $t (0.01s)"; done; } >"$T/missing_first.log"
printf 'ok  \tcatalogizer/tests/nfsemu\t0.1s [no tests to run]\n' >"$T/zero.log"
{ for t in $(nfs_emulator_required_tests | head -4); do echo "--- PASS: $t (0.01s)"; done
  echo "    --- PASS: TestNFS3FactoryLifecycleLive (0.00s)"; } >"$T/indented.log"
{ for t in $(nfs_emulator_required_tests | head -4); do echo "--- PASS: $t (0.01s)"; done
  echo "--- PASS: TestNFS3FactoryLifecycleLiveExtra (0.00s)"; } >"$T/prefix.log"
{ cat "$T/pass.log"; echo "--- FAIL: TestSomethingElse (0.01s)"; } >"$T/failalso.log"
{ cat "$T/pass.log"; echo "panic: runtime error: index out of range"; } >"$T/panic.log"
{ for t in $(nfs_emulator_required_tests | head -4); do echo "--- PASS: $t (0.01s)"; done
  echo "--- PASS: TestNFS3FactoryLifecycleLive (fake"; } >"$T/fakecarrier.log"
{ for t in $(nfs_emulator_required_tests | head -4); do echo "--- PASS: $t (0.01s)"; done
  echo "--- PASS: TestNFS3FactoryLifecycleLive (0.00s) trailing junk"; } >"$T/trailing.log"
{ for t in $(nfs_emulator_required_tests | head -4); do echo "--- PASS: $t (0.01s)"; done
  echo "--- PASS: TestNFS3FactoryLifecycleLive (s)"; } >"$T/nodigits.log"
: >"$T/empty.log"
# NFS3FU2-M2: a CR-terminated final PASS line (CRLF log) must be refused whatever `grep` resolves to; the check must not depend on a shell wrapper function.
{ for t in $(nfs_emulator_required_tests | head -4); do echo "--- PASS: $t (0.01s)"; done
  printf -- '--- PASS: TestNFS3FactoryLifecycleLive (0.00s)\r\n'; } >"$T/crlf.log"
# NFS3FU2-N1: duration needs digits on both sides of the dot; N2: a column-0 subtest-looking name is not a required test.
{ for t in $(nfs_emulator_required_tests | head -4); do echo "--- PASS: $t (0.01s)"; done
  echo "--- PASS: TestNFS3FactoryLifecycleLive (0.s)"; } >"$T/fracnodigit.log"
{ for t in $(nfs_emulator_required_tests | head -4); do echo "--- PASS: $t (0.01s)"; done
  echo "--- PASS: TestNFS3FactoryLifecycleLive/sub (0.00s)"; } >"$T/col0sub.log"
nfs_emulator_missing_pass "$T/pass.log" >/dev/null; ok $? 0 "full PASS log accepted"
nfs_emulator_missing_pass "$T/skip.log" >/dev/null; ok $? 1 "lifecycle SKIP refused"
out="$(nfs_emulator_missing_pass "$T/skip.log")"; ok "$out" TestNFS3FactoryLifecycleLive "SKIP log names the missing test"
nfs_emulator_missing_pass "$T/subonly.log" >/dev/null; ok $? 1 "subtest-only PASS refused"
nfs_emulator_missing_pass "$T/missing_first.log" >/dev/null; ok $? 1 "a missing nfsemu test refused"
nfs_emulator_missing_pass "$T/indented.log" >/dev/null; ok $? 1 "indented (non top-level) PASS line refused"
nfs_emulator_missing_pass "$T/prefix.log" >/dev/null; ok $? 1 "PASS line of a longer test name refused"
nfs_emulator_missing_pass "$T/failalso.log" >/dev/null; ok $? 1 "all PASS lines plus an anchored FAIL refused"
nfs_emulator_missing_pass "$T/panic.log" >/dev/null; ok $? 1 "all PASS lines plus a panic refused"
nfs_emulator_missing_pass "$T/zero.log" >/dev/null; ok $? 1 "zero-test run refused"
nfs_emulator_missing_pass "$T/fakecarrier.log" >/dev/null; ok $? 1 "unterminated fake PASS line refused"
nfs_emulator_missing_pass "$T/trailing.log" >/dev/null; ok $? 1 "PASS line with trailing junk refused"
nfs_emulator_missing_pass "$T/nodigits.log" >/dev/null; ok $? 1 "PASS line with no duration digits refused"
nfs_emulator_missing_pass "$T/crlf.log" >/dev/null; ok $? 1 "CR-terminated PASS line refused"
nfs_emulator_missing_pass "$T/fracnodigit.log" >/dev/null; ok $? 1 "duration with no digit before the dot refused"
nfs_emulator_missing_pass "$T/col0sub.log" >/dev/null; ok $? 1 "column-0 subtest PASS line refused"
# the check must not depend on `grep` resolving to a CR-tolerant wrapper function (the harness defines one that is inherited by `bash script`):
# define a deterministic CR-tolerant stand-in (`$` -> optional CR then `$`) and re-run; the check must still refuse.
grep() { local a=() x; for x in "$@"; do a+=("${x//\$/$'\r'?\$}"); done; command grep "${a[@]}"; }
printf -- '--- PASS: X (0.00s)\r\n' | grep -Eq -- '^--- PASS: X \([0-9.]+s\)$' || { echo "FAIL stand-in wrapper does not tolerate CR (test self-check)"; fail=1; }
nfs_emulator_missing_pass "$T/crlf.log" >/dev/null; ok $? 1 "CR-terminated PASS line refused even with a CR-tolerant grep function in scope"
unset -f grep
nfs_emulator_missing_pass "$T/empty.log" >/dev/null; ok $? 1 "empty log refused"
nfs_emulator_missing_pass "$T/nonexistent.log" >/dev/null; ok $? 1 "missing log refused"
exit $fail
