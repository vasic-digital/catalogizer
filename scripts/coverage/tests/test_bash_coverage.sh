#!/usr/bin/env bash
# test_bash_coverage.sh - T199 (RED first). Oracle for scripts/bash-coverage.sh, the PS4 line-trace coverage harness (docs/05 7.1, constitution 11.4.224 E).
# Expected numbers are COUNTED BY HAND from fixtures/cov_fixture.sh and cov_fixture2.sh (the file comments in this test list the lines), never taken from the harness:
#   cov_fixture.sh  executable lines 3 5 6 7 10 11 13 14 15 17 (10); run with no argument it executes 3 5 6 7 13 14 17 (7) = 70.00 %; uncovered 10 11 15; the function uncovered_fn is never called
#   cov_fixture2.sh executable lines 3 4 5 6 8 11 (6) (case arms, a here-document body and a continuation line are not separate lines); it executes 3 4 5 8 11 (5) = 83.33 %
#   cov_excluded.sh executable lines 3 4 5 (3); it executes 3 4 (2); the fence excludes it, so it must not enter the totals
# Run through `scripts/test-in-container.sh build-scripts unit -- bash scripts/coverage/tests/test_bash_coverage.sh` (IMG-KCOV: bash 5, python3, jq, kcov 43).
# The kcov cross-check leg SKIPs (named, never a pass) where kcov is absent. Paired mutations: copies of the harness with ONE guard removed must each make this body FAIL.
# Usage: test_bash_coverage.sh [--no-mutations]     Env: HARNESS (script under test), MUTATION_RECORD
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../../.." && pwd)"
HARNESS="${HARNESS:-$REPO/scripts/bash-coverage.sh}"
FX="$HERE/fixtures"
PASSES=0; FAILS=0; SKIPS=0
ok()  { PASSES=$((PASSES+1)); echo "PASS: $1"; }
bad() { FAILS=$((FAILS+1)); echo "FAIL: $1"; }
skip() { SKIPS=$((SKIPS+1)); echo "SKIP: $1"; }
check() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
T="$(mktemp -d "${TMPDIR:-/tmp}/bashcov-test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
[ -f "$HARNESS" ] || { bad "the harness scripts/bash-coverage.sh does not exist (T199)"; echo "Summary: PASS=$PASSES FAIL=$FAILS SKIP=$SKIPS"; exit 1; }
for d in jq python3; do command -v "$d" >/dev/null 2>&1 || { echo "FAIL: $d is required"; exit 2; }; done
run() { bash "$HARNESS" "$@" >"$T/out" 2>"$T/err"; RC=$?; }
jqf() { jq -r "$2" "$1"; }

# --- fixture 1: executable 10, executed 7, 70.00 %, uncovered 10 11 15, uncovered_fn never called
run --src-root "$FX" --out "$T/o1" --target cov_fixture.sh -- bash "$FX/cov_fixture.sh"
check "fixture 1: exit 0" "$RC" 0
J="$T/o1/bash-coverage.json"
check "fixture 1: schema" "$(jqf $J .schema)" "bash-coverage/1"
check "fixture 1: executable lines (hand count 10)" "$(jqf $J '.targets[0].executable')" 10
check "fixture 1: executed lines (hand count 7)" "$(jqf $J '.targets[0].executed')" 7
check "fixture 1: percent" "$(jqf $J '.targets[0].percent')" "70.00"
check "fixture 1: the uncovered lines are 10 11 15" "$(jqf $J '.targets[0].uncovered_lines | map(tostring) | join(" ")')" "10 11 15"
check "fixture 1: the never-called function is REPORTED" "$(jqf $J '.targets[0].uncovered_functions | join(" ")')" "uncovered_fn"
check "fixture 1: the called function is not reported uncovered" "$(jqf $J '[.targets[0].uncovered_functions[] | select(.=="covered_fn")] | length')" 0
check "fixture 1: no traced line was classified non-executable (the classifier agrees with the tracer)" "$(jqf $J '.targets[0].traced_non_executable | length')" 0
check "fixture 1: totals equal the single target" "$(jqf $J '.totals.executed')/$(jqf $J '.totals.executable')" "7/10"
check "fixture 1: the record states the PS4 that was used" "$(jqf $J .ps4)" '+COV:${BASH_SOURCE##*/}:${LINENO}:'
jqf $J '.limits | join(" | ")' | grep -qi 'line, not branch' && ok "fixture 1: the honest limit (line, not branch) is written into the record" || bad "fixture 1: limit text missing"
jqf $J '.limits | join(" | ")' | grep -q 'set +x' && ok "fixture 1: the set +x / trap limit is written into the record" || bad "fixture 1: set +x limit missing"
# the argument changes which branch runs: with `never` the branch executes (line 15) and the fixture covers 8 of 10
run --src-root "$FX" --out "$T/o1b" --target cov_fixture.sh -- bash "$FX/cov_fixture.sh" never
check "fixture 1 with the branch taken: executed 8 (line 15 now executes)" "$(jqf $T/o1b/bash-coverage.json '.targets[0].executed')" 8
# --- fixture 2: executable 6, executed 5, 83.33 %
run --src-root "$FX" --out "$T/o2" --target cov_fixture2.sh -- bash "$FX/cov_fixture2.sh"
J2="$T/o2/bash-coverage.json"
check "fixture 2: executable (hand count 6)" "$(jqf $J2 '.targets[0].executable')" 6
check "fixture 2: executed (hand count 5)" "$(jqf $J2 '.targets[0].executed')" 5
check "fixture 2: percent" "$(jqf $J2 '.targets[0].percent')" "83.33"
check "fixture 2: the uncovered line is the case arm 6" "$(jqf $J2 '.targets[0].uncovered_lines | map(tostring) | join(" ")')" "6"
check "fixture 2: no traced line outside the classifier's executable set" "$(jqf $J2 '.targets[0].traced_non_executable | length')" 0
# --- a driver that runs three scripts through CHILD bash processes; the fence excludes the vendored one
mkdir -p "$T/fx"; cat >"$T/fx/fixtures.yaml" <<'Y'
schema: coverage-exclusions/1
application: fixtures
exclusions:
  - path: "cov_excluded.sh"
    class: vendored-third-party
    justification: "a vendored third-party script copied in unmodified, owned by its upstream"
Y
run --src-root "$FX" --out "$T/o3" --target cov_fixture.sh --target cov_fixture2.sh --target cov_excluded.sh --exclusions "$T/fx/fixtures.yaml" -- bash "$FX/cov_driver.sh"
check "driver with a fence: exit 0" "$RC" 0
J3="$T/o3/bash-coverage.json"
check "driver: child processes are traced (totals 12 executed of 16 executable)" "$(jqf $J3 '.totals.executed')/$(jqf $J3 '.totals.executable')" "12/16"
check "driver: the excluded file is not in the targets" "$(jqf $J3 '[.targets[].file] | join(" ")')" "cov_fixture.sh cov_fixture2.sh"
check "driver: the excluded file is listed with its class" "$(jqf $J3 '.excluded[0].file + ":" + .excluded[0].class')" "cov_excluded.sh:vendored-third-party"
check "driver: the totals percent is 75.00" "$(jqf $J3 '.totals.percent')" "75.00"
run --src-root "$FX" --out "$T/o4" --target cov_fixture.sh --target cov_fixture2.sh --target cov_excluded.sh -- bash "$FX/cov_driver.sh"
check "driver without a fence: all three count (14 executed of 19 executable)" "$(jqf $T/o4/bash-coverage.json '.totals.executed')/$(jqf $T/o4/bash-coverage.json '.totals.executable')" "14/19"
# an unjustified / first-party-without-item fence is refused by the T200 gate BEFORE any measuring
cat >"$T/fx/fixtures-bad.yaml" <<'Y'
schema: coverage-exclusions/1
application: fixtures-bad
exclusions:
  - path: "cov_excluded.sh"
    class: first-party
    justification: "excluded because it is inconvenient to cover"
Y
run --src-root "$FX" --out "$T/o5" --target cov_fixture.sh --exclusions "$T/fx/fixtures-bad.yaml" -- bash "$FX/cov_fixture.sh"
[ "$RC" = 3 ] && grep -q 'exclusions_gate_failed' "$T/err" && ok "a fence the gate refuses is refused by the harness (exit 3 exclusions_gate_failed)" || bad "bad fence accepted or wrong exit ($RC): $(cat "$T/err")"
[ ! -e "$T/o5/bash-coverage.json" ] && ok "a refused fence writes no coverage record" || bad "a record was written for a refused fence"
# the command's failure is never swallowed
cat >"$T/failing.sh" <<'F'
#!/usr/bin/env bash
bash "$1/cov_fixture.sh" >/dev/null
exit 7
F
run --src-root "$FX" --out "$T/o6" --target cov_fixture.sh -- bash "$T/failing.sh" "$FX"
check "a failing command: the harness exits with the command's status" "$RC" 7
check "a failing command: the record says command_failed with the status" "$(jqf $T/o6/bash-coverage.json '.status + ":" + (.command_rc|tostring)')" "command_failed:7"
# the instrument must see: a command that runs no bash at all leaves an empty trace and is refused (control needle), never reported as 0 %
run --src-root "$FX" --out "$T/o7" --target cov_fixture.sh -- true
[ "$RC" = 3 ] && grep -q 'trace_empty' "$T/err" && ok "an empty trace is refused as trace_empty (exit 3), never reported as 0 percent" || bad "empty trace not refused ($RC): $(cat "$T/err")"
# a target that the command never runs, while the trace is not empty, is a legitimate 0 percent
run --src-root "$FX" --out "$T/o8" --target cov_fixture.sh --target cov_fixture2.sh -- bash "$FX/cov_fixture2.sh"
check "a target the command never ran is reported 0.00 (the trace is not empty)" "$(jqf $T/o8/bash-coverage.json '.targets[] | select(.file=="cov_fixture.sh") | .percent')" "0.00"
# refusals
run --src-root "$FX" --out "$T/o9" --target no_such_file.sh -- bash "$FX/cov_fixture.sh"
[ "$RC" = 3 ] && grep -q 'target_missing' "$T/err" && ok "a missing target is refused (exit 3 target_missing)" || bad "missing target not refused ($RC)"
mkdir -p "$T/col/a" "$T/col/b"; cp "$FX/cov_fixture.sh" "$T/col/a/x.sh"; cp "$FX/cov_fixture.sh" "$T/col/b/x.sh"
run --src-root "$T/col" --out "$T/o10" --target a/x.sh --target b/x.sh -- bash "$T/col/a/x.sh"
[ "$RC" = 3 ] && grep -q 'basename_collision' "$T/err" && ok "two targets with one basename are refused (the PS4 names the base name only)" || bad "basename collision not refused ($RC)"
run --src-root "$FX" --out "$T/o11"; check "no -- command is a usage error" "$RC" 2
run --src-root "$FX" --out "$T/o12" -- bash "$FX/cov_fixture.sh"; check "no target is a usage error" "$RC" 2
# the harness's own state does not leak: BASH_ENV of the caller is restored for the command's parent shell (the harness exports it only to the run)
check "the caller's BASH_ENV is untouched after the run" "${BASH_ENV:-unset}" "unset"
# determinism: a second run over the same fixture gives the same numbers (the record's executed lines are equal)
run --src-root "$FX" --out "$T/o13" --target cov_fixture.sh -- bash "$FX/cov_fixture.sh"
check "determinism: two runs give identical executed lines" "$(jqf $T/o13/bash-coverage.json '.targets[0].executed_lines | map(tostring) | join(",")')" "$(jqf $T/o1/bash-coverage.json '.targets[0].executed_lines | map(tostring) | join(",")')"

# --- kcov cross-check (IMG-KCOV): kcov is an independent instrument; on fixture 1 it must report the same executable and executed line counts
#     (measured 2026-10-06 in IMG-KCOV: kcov 7 of 10 lines = 70.00 %, equal to this harness). A disagreement is a FAIL, kcov absent is a named SKIP.
if command -v kcov >/dev/null 2>&1; then
  K="$T/kcov"; mkdir -p "$K"
  kcov --bash-method=DEBUG "$K" "$FX/cov_fixture.sh" >/dev/null 2>&1
  KJ="$(find "$K" -name coverage.json | head -1)"
  if [ -n "$KJ" ]; then
    KC="$(jq -r '[.files[] | select(.file | endswith("cov_fixture.sh"))][0] | (.covered_lines|tostring) + "/" + (.total_lines|tostring)' "$KJ" 2>/dev/null)"
    OURS="$(jqf $J '.targets[0].executed|tostring')/$(jqf $J '.targets[0].executable|tostring')"
    check "kcov cross-check: kcov and this harness agree on executed/executable lines of fixture 1" "$KC" "$OURS"
  else skip "kcov cross-check: kcov ran but wrote no coverage.json (UNCONFIRMED)"; fi
else skip "kcov cross-check: kcov is not installed on this host (it is in IMG-KCOV: run through TIC build-scripts unit)"; fi

# --- paired mutations ---
if [ "${1:-}" != --no-mutations ] && [ -z "${HARNESS_MUTANT:-}" ]; then
  REC="${MUTATION_RECORD:-$T/mutations.txt}"; : >"$REC"
  mut() { # mut NAME FILE OLD NEW   (FILE is bash-coverage.sh or bashcov.py, both copied)
    local name="$1" f="$2" old="$3" new="$4" d="$T/mut-$1"; rm -rf "$d"; mkdir -p "$d/scripts/coverage"
    cp "$REPO/scripts/bash-coverage.sh" "$d/scripts/"; cp "$REPO/scripts/coverage/bashcov.py" "$REPO/scripts/coverage/check_exclusions.sh" "$REPO/scripts/coverage/check_exclusions.py" "$d/scripts/coverage/"
    local tgt="$d/scripts/$f"; [ "$f" = bashcov.py ] && tgt="$d/scripts/coverage/bashcov.py"
    python3 -I - "$tgt" "$old" "$new" <<'PY' || { bad "mutation $name: anchor not unique"; return; }
import sys
s=open(sys.argv[1]).read()
if s.count(sys.argv[2])!=1: sys.exit(1)
open(sys.argv[1],"w").write(s.replace(sys.argv[2],sys.argv[3]))
PY
    if HARNESS="$d/scripts/bash-coverage.sh" HARNESS_MUTANT=1 bash "${BASH_SOURCE[0]}" --no-mutations >"$T/mut-$name.out" 2>&1; then bad "mutation $name SURVIVED"; echo "SURVIVED $name" >>"$REC"
    else ok "mutation $name caught ($(grep -c '^FAIL:' "$T/mut-$name.out") failing legs)"; echo "CAUGHT $name: $(grep '^FAIL:' "$T/mut-$name.out" | head -2 | cut -c1-110 | tr '\n' '|')" >>"$REC"; fi
  }
  mut no-xtrace-fd bash-coverage.sh 'BASH_XTRACEFD=$COV_FD' ':'
  mut structural-counted bashcov.py 'if STRUCT_RE.match(s):' 'if False:'
  mut exclusions-ignored bashcov.py 'if e:   # MUT:excl' 'if False:'
  mut command-rc-swallowed bash-coverage.sh 'exit "$CMD_RC"' 'exit 0'
  mut empty-trace-accepted bashcov.py 'if trace_lines == 0:' 'if False:'
  mut fence-gate-skipped bash-coverage.sh 'bash "$GATE" "$EXCL"' 'true'
  mut collision-unchecked bashcov.py 'if len(bases) != len(set(bases)):' 'if False:'
  mut function-report-dropped bashcov.py 'uncovered_functions = [f["name"] for f in funcs if not (set(f["lines"]) & executed)]' 'uncovered_functions = []'
fi
echo "Summary: PASS=$PASSES FAIL=$FAILS SKIP=$SKIPS"
[ "$FAILS" = 0 ]
