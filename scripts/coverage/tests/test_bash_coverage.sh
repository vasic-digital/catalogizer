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
check "fixture 1: the record states the PS4 that was used (the FULL path, review I3)" "$(jqf $J .ps4)" '+COV:${BASH_SOURCE}:${LINENO}:'
check "fixture 1: a clean run has status ok (no classifier disagreement)" "$(jqf $J .status)" ok
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
  - path: "vendor/cov_excluded.sh"
    class: vendored-third-party
    justification: "a vendored third-party script copied in unmodified, owned by its upstream"
Y
run --src-root "$FX" --out "$T/o3" --target cov_fixture.sh --target cov_fixture2.sh --target vendor/cov_excluded.sh --exclusions "$T/fx/fixtures.yaml" -- bash "$FX/cov_driver.sh"
check "driver with a fence: exit 0" "$RC" 0
J3="$T/o3/bash-coverage.json"
check "driver: child processes are traced (totals 12 executed of 16 executable)" "$(jqf $J3 '.totals.executed')/$(jqf $J3 '.totals.executable')" "12/16"
check "driver: the excluded file is not in the targets" "$(jqf $J3 '[.targets[].file] | join(" ")')" "cov_fixture.sh cov_fixture2.sh"
check "driver: the excluded file is listed with its class" "$(jqf $J3 '.excluded[0].file + ":" + .excluded[0].class')" "vendor/cov_excluded.sh:vendored-third-party"
check "driver: the totals percent is 75.00" "$(jqf $J3 '.totals.percent')" "75.00"
run --src-root "$FX" --out "$T/o4" --target cov_fixture.sh --target cov_fixture2.sh --target vendor/cov_excluded.sh -- bash "$FX/cov_driver.sh"
check "driver without a fence: all three count (14 executed of 19 executable)" "$(jqf $T/o4/bash-coverage.json '.totals.executed')/$(jqf $T/o4/bash-coverage.json '.totals.executable')" "14/19"
# an unjustified / first-party-without-item fence is refused by the T200 gate BEFORE any measuring
cat >"$T/fx/fixtures-bad.yaml" <<'Y'
schema: coverage-exclusions/1
application: fixtures-bad
exclusions:
  - path: "vendor/cov_excluded.sh"
    class: first-party
    justification: "excluded because it is inconvenient to cover"
Y
run --src-root "$FX" --out "$T/o5" --target cov_fixture.sh --exclusions "$T/fx/fixtures-bad.yaml" -- bash "$FX/cov_fixture.sh"
[ "$RC" = 3 ] && grep -q 'exclusions_gate_failed' "$T/err" && ok "a fence the gate refuses is refused by the harness (exit 3 exclusions_gate_failed)" || bad "bad fence accepted or wrong exit ($RC): $(cat "$T/err")"
[ ! -e "$T/o5/bash-coverage.json" ] && ok "a refused fence writes no coverage record" || bad "a record was written for a refused fence"
# review I5 at the harness: the gate runs WITH --root, so a class claim that the files contradict is refused before any measuring
cat >"$T/fx/fixtures-false-class.yaml" <<'Y'
schema: coverage-exclusions/1
application: fixtures-false-class
exclusions:
  - path: "cov_fixture.sh"
    class: generated-code
    justification: "claimed generated, but it is a hand-written script with no generated marker at all"
Y
run --src-root "$FX" --out "$T/o5b" --target cov_fixture.sh --target cov_fixture2.sh --exclusions "$T/fx/fixtures-false-class.yaml" -- bash "$FX/cov_fixture.sh"
[ "$RC" = 3 ] && grep -q 'class generated-code is not true' "$T/err" && ok "I5: a fence whose class is false of the file it excludes is refused by the harness (exit 3, content-checked through --root)" || bad "I5: false class accepted by the harness ($RC): $(cat "$T/err")"
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
# --- review I3: attribution by PATH. Two targets with one base name are legal; a same-basename file that is not the target earns it nothing.
mkdir -p "$T/col/a" "$T/col/b" "$T/col/other/b"; cp "$FX/cov_fixture.sh" "$T/col/a/x.sh"; cp "$FX/cov_fixture.sh" "$T/col/b/x.sh"; cp "$FX/cov_fixture.sh" "$T/col/other/b/x.sh"
run --src-root "$T/col" --out "$T/o10" --target a/x.sh --target b/x.sh -- bash "$T/col/b/x.sh"
check "I3: two targets sharing a base name are accepted (exit 0)" "$RC" 0
check "I3: only the run one earns its lines (b/x.sh 70.00)" "$(jqf $T/o10/bash-coverage.json '.targets[] | select(.file=="b/x.sh") | .percent')" "70.00"
check "I3: the sibling with the same base name that never ran stays 0.00 (control: the collision no longer shares credit)" "$(jqf $T/o10/bash-coverage.json '.targets[] | select(.file=="a/x.sh") | .percent')" "0.00"
run --src-root "$T/col" --out "$T/o10b" --target a/x.sh -- bash "$T/col/other/b/x.sh"
check "I3 probe: running a DIFFERENT x.sh does not credit the target a/x.sh (0.00, was 30.00 on the basename rule)" "$(jqf $T/o10b/bash-coverage.json '.targets[0].percent')" "0.00"
check "I3 probe: the foreign same-basename file is NAMED in the record" "$(jqf $T/o10b/bash-coverage.json '.targets[0].attribution.foreign_same_basename | map(endswith("other/b/x.sh")) | any')" true
check "I3 probe: no traced line is credited to the non-target" "$(jqf $T/o10b/bash-coverage.json '.targets[0].traced_non_executable | length')" 0
( cd "$T/col" && bash "$HARNESS" --src-root "$T/col" --out "$T/o10c" --target a/x.sh -- bash other/b/x.sh ) >/dev/null 2>&1
check "I3 probe: a RELATIVE path that resolves to a different file is not credited" "$(jqf $T/o10c/bash-coverage.json '.targets[0].percent')" "0.00"
( cd "$T/col" && bash "$HARNESS" --src-root "$T/col" --out "$T/o10d" --target a/x.sh -- bash a/x.sh ) >/dev/null 2>&1
check "I3 golden-false: a RELATIVE path that resolves to the target is credited (70.00)" "$(jqf $T/o10d/bash-coverage.json '.targets[0].percent')" "70.00"
# a copy of the tree that keeps the layout (the way tests/test_build_system.sh copies Build/) is credited; a root-level target has no layout and is not
mkdir -p "$T/copy/a" "$T/copy/b"; cp "$FX/cov_fixture.sh" "$T/copy/a/x.sh"
run --src-root "$T/col" --out "$T/o10e" --target a/x.sh -- bash "$T/copy/a/x.sh"
check "I3: a copy that keeps the target's relative layout (.../a/x.sh) is credited (70.00)" "$(jqf $T/o10e/bash-coverage.json '.targets[0].percent')" "70.00"
check "I3: the copy path is recorded as a copy" "$(jqf $T/o10e/bash-coverage.json '.targets[0].attribution.copy_paths | length')" 1
mkdir -p "$T/rootsrc"; cp "$FX/cov_fixture.sh" "$T/rootsrc/x.sh"; cp "$FX/cov_fixture.sh" "$T/copy/b/x.sh"
run --src-root "$T/rootsrc" --out "$T/o10f" --target x.sh -- bash "$T/copy/b/x.sh"
check "I3: a root-level target is not credited by any same-basename file elsewhere (0.00)" "$(jqf $T/o10f/bash-coverage.json '.targets[0].percent')" "0.00"
run --src-root "$T/col" --out "$T/o10g" --target a/x.sh --target ./a/x.sh -- bash "$T/col/a/x.sh"
[ "$RC" = 3 ] && grep -q 'duplicate_target' "$T/err" && ok "I3: two --target arguments naming ONE file are refused (3 duplicate_target)" || bad "duplicate target not refused ($RC): $(cat "$T/err")"
# --- review I4: a relative --out must not lose the children's trace after a cd
cat >"$T/cdrun.sh" <<'F'
#!/usr/bin/env bash
cd "$1" || exit 9
bash "$2/cov_fixture.sh" >/dev/null
F
mkdir -p "$T/elsewhere"
( cd "$T" && bash "$HARNESS" --src-root "$FX" --out rel-out --target cov_fixture.sh -- bash "$T/cdrun.sh" "$T/elsewhere" "$FX" ) >"$T/out" 2>"$T/err"; RC=$?
check "I4: a RELATIVE --out with a child that cd's: exit 0" "$RC" 0
check "I4: the child's lines are traced (70.00, was 0.00 with exit 0)" "$(jqf $T/rel-out/bash-coverage.json '.targets[0].percent')" "70.00"
( cd "$T" && bash "$HARNESS" --src-root "$FX" --out abs-out --target cov_fixture.sh --exclusions fx/fixtures.yaml -- bash "$T/cdrun.sh" "$T/elsewhere" "$FX" ) >/dev/null 2>&1
check "I4: a RELATIVE --exclusions file is resolved before any cd (the fence still applies)" "$(jqf $T/abs-out/bash-coverage.json '.exclusions_file | startswith("/")')" true
# --- review i1: the classifier counts the elements of a pipeline / and-or continuation (the tracer reports each one's own line)
run --src-root "$FX" --out "$T/o14" --target cov_pipe.sh -- bash "$FX/cov_pipe.sh"
check "i1: pipeline fixture executable lines (hand count 8: 3 4 5 6 7 8 9 10)" "$(jqf $T/o14/bash-coverage.json '.targets[0].executable')" 8
check "i1: pipeline fixture executed (8)" "$(jqf $T/o14/bash-coverage.json '.targets[0].executed')" 8
check "i1: no traced line is classified non-executable (the classifier agrees with the tracer)" "$(jqf $T/o14/bash-coverage.json '.targets[0].traced_non_executable | length')" 0
# measured on Build/lib/hash.sh (a 12-line array literal at 31-42 traced as line 42 alone): hand count for the fixture = lines 6 and 7
run --src-root "$FX" --out "$T/o15" --target cov_array.sh -- bash "$FX/cov_array.sh"
check "i1: a multi-line array assignment counts ONE executable line, its closing line (fixture hand count 2: lines 6 7)" "$(jqf $T/o15/bash-coverage.json '.targets[0].executable')" 2
check "i1: both are executed (the assignment is traced at line 6)" "$(jqf $T/o15/bash-coverage.json '.targets[0].executed_lines | map(tostring) | join(",")')" "6,7"
check "i1: no classifier disagreement on the array fixture (status ok)" "$(jqf $T/o15/bash-coverage.json .status)" ok
# the tracer's line for a command that spans lines depends on the bash VERSION (measured in IMG-KCOV: bash 5.2.15 reports `echo \` + `"arg"` at its LAST line 11, bash 5.3.9 at its first
# line 10). The report folds either onto the command's counted line, proven here on SYNTHETIC traces so the leg does not depend on the bash that runs the suite.
syn() { # syn NAME FILE line... : a hand-made trace of FILE (absolute path) with the given lines, reported through bashcov.py directly
  local n="$1" f="$2"; shift 2; : >"$T/syn-$n.trace"; for l in "$@"; do printf '+COV:%s:%s:cmd\n' "$FX/$f" "$l" >>"$T/syn-$n.trace"; done
  python3 -I "$(dirname "$HARNESS")/coverage/bashcov.py" report --src-root "$FX" --trace "$T/syn-$n.trace" --out "$T/syn-$n.json" --cwd / --command syn --command-rc 0 --bash-version synthetic -- "$f" >/dev/null 2>&1; }
syn p53 cov_pipe.sh 3 4 5 6 7 8 9 10
check "i1 synthetic (bash 5.3 style, the continuation traced at its first line 10): all 8 executed, none foreign" "$(jqf $T/syn-p53.json '(.targets[0].executed|tostring) + "/" + (.targets[0].traced_non_executable|length|tostring)')" "8/0"
syn p52 cov_pipe.sh 3 4 5 6 7 8 9 11
check "i1 synthetic (bash 5.2 style, the continuation traced at its LAST line 11): the same 8 executed, none foreign" "$(jqf $T/syn-p52.json '(.targets[0].executed|tostring) + "/" + (.targets[0].traced_non_executable|length|tostring)')" "8/0"
check "i1 synthetic: the status is ok for both styles" "$(jqf $T/syn-p52.json .status)" ok
syn p00 cov_pipe.sh 3 4 5 6 7 8 9
check "i1 synthetic control: with the continuation line never traced the command is NOT executed (7 of 8)" "$(jqf $T/syn-p00.json '(.targets[0].executed|tostring) + "/" + (.targets[0].executable|tostring)')" "7/8"
syn a3 cov_array.sh 3 7
check "i1 synthetic: a multi-line array reported at its OPENING line (an older bash) is folded onto its closing line: 2 of 2" "$(jqf $T/syn-a3.json '(.targets[0].executed|tostring) + "/" + (.targets[0].executable|tostring)')" "2/2"
check "i1: an argument continuation (line 11) is not counted" "$(jqf $T/o14/bash-coverage.json '[.targets[0].executed_lines[] | select(.==11)] | length')" 0
run --src-root "$FX" --out "$T/o11"; check "no -- command is a usage error" "$RC" 2
run --src-root "$FX" --out "$T/o12" -- bash "$FX/cov_fixture.sh"; check "no target is a usage error" "$RC" 2
# the harness's own state does not leak: BASH_ENV of the caller is restored for the command's parent shell (the harness exports it only to the run)
# (review m9: the old leg asserted the CALLER's BASH_ENV after the run, which no child process can change - a tautology.) The real property: the command receives BASH_ENV
# and COV_TRACE_FILE pointing into ITS OWN --out directory, and a command run WITHOUT the harness sees neither.
printf '#!/usr/bin/env bash\necho "BASHENV=${BASH_ENV:-unset}"\necho "TRACEFILE=${COV_TRACE_FILE:-unset}"\n' >"$T/env-probe.sh"
run --src-root "$FX" --out "$T/o12b" --target cov_fixture.sh -- bash "$T/env-probe.sh"
grep -qx "BASHENV=$T/o12b/.cov-bashenv.sh" "$T/out" && grep -qx "TRACEFILE=$T/o12b/trace.log" "$T/out" && ok "m9: the command is handed BASH_ENV and COV_TRACE_FILE inside its own --out directory" || bad "m9: env handed to the command: $(grep -E 'BASHENV|TRACEFILE' "$T/out" | tr '\n' ' ')"
env -u BASH_ENV -u COV_TRACE_FILE bash "$T/env-probe.sh" >"$T/env-bare.txt" 2>&1
grep -qx 'BASHENV=unset' "$T/env-bare.txt" && grep -qx 'TRACEFILE=unset' "$T/env-bare.txt" && ok "m9 control: the same probe run without the harness sees neither variable (so the leg above can fail)" || bad "m9 control: probe sees $(cat "$T/env-bare.txt" | tr '\n' ' ')"
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
    cp "$REPO/scripts/bash-coverage.sh" "$d/scripts/"; cp "$REPO/scripts/coverage/bashcov.py" "$REPO/scripts/coverage/fence_lib.py" "$REPO/scripts/coverage/check_exclusions.sh" "$REPO/scripts/coverage/check_exclusions.py" "$d/scripts/coverage/"
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
  # SANDBOX CONTROL (review round 1): an identity mutation must leave this body GREEN, or every "caught" below could be the sandbox failing (a missing import) and not the mutation
  ctl() { local d="$T/mut-ctl"; rm -rf "$d"; mkdir -p "$d/scripts/coverage"; cp "$REPO/scripts/bash-coverage.sh" "$d/scripts/"
    cp "$REPO/scripts/coverage/bashcov.py" "$REPO/scripts/coverage/fence_lib.py" "$REPO/scripts/coverage/check_exclusions.sh" "$REPO/scripts/coverage/check_exclusions.py" "$d/scripts/coverage/"
    if HARNESS="$d/scripts/bash-coverage.sh" HARNESS_MUTANT=1 bash "${BASH_SOURCE[0]}" --no-mutations >"$T/mut-ctl.out" 2>&1; then ok "mutation sandbox control: an UNMUTATED copy of the harness passes this body (so a CAUGHT below is the mutation)"; echo "CONTROL PASS" >>"$REC"
    else bad "mutation sandbox control FAILED: the unmutated copy does not pass ($(grep -c '^FAIL:' "$T/mut-ctl.out") failing legs), every CAUGHT below is suspect"; echo "CONTROL FAIL" >>"$REC"; fi; }
  ctl
  mut no-xtrace-fd bash-coverage.sh 'BASH_XTRACEFD=$COV_FD' ':'
  mut structural-counted bashcov.py 'if STRUCT_RE.match(s):' 'if False:'
  mut exclusions-ignored bashcov.py 'if e:   # MUT:excl' 'if False:'
  mut command-rc-swallowed bash-coverage.sh 'exit "$CMD_RC"' 'exit 0'
  mut empty-trace-accepted bashcov.py 'if trace_lines == 0:' 'if False:'
  mut fence-root-dropped bash-coverage.sh '"$GATE" "$EXCL" --root "$SRC"' '"$GATE" "$EXCL"'
  mut fence-gate-skipped bash-coverage.sh 'bash "$GATE" "$EXCL"' 'true'
  mut duplicate-target-unchecked bashcov.py 'if len(set(os.path.realpath(os.path.join(src, t)) for t in targets)) != len(targets):' 'if False:'
  # review round 1: the attribution and path rules, and the classifier rule for pipelines
  mut RM_basename_credit bashcov.py 'if os.path.realpath(p) == absT:   # MUT:direct_realpath' 'if os.path.basename(p) == os.path.basename(absT):   # MUT:direct_realpath'
  mut copy-suffix-dropped bashcov.py 'elif "/" in relT and p.endswith("/" + relT):   # MUT:copy_suffix' 'elif False:   # MUT:copy_suffix'
  mut root-level-copy-credit bashcov.py 'elif "/" in relT and p.endswith("/" + relT):   # MUT:copy_suffix' 'elif p.endswith("/" + relT):   # MUT:copy_suffix'
  mut relative-out-kept bash-coverage.sh 'OUT="$(cd "$OUT" && pwd)"   # MUT:abs_out' ':'
  mut continuation-alias-dropped bashcov.py 'elif cont_start:' 'elif False:'
  mut array-alias-dropped bashcov.py 'alias[al] = i' 'pass'
  mut array-lines-counted bashcov.py 'if in_array:   # MUT:array' 'if False:   # MUT:array'
  mut pipe-continuation-dropped bashcov.py 'if prev_pipe and s and not s.startswith("#"):   # MUT:pipe_cont' 'if False:'
  mut function-report-dropped bashcov.py 'uncovered_functions = [f["name"] for f in funcs if not (set(f["lines"]) & executed)]' 'uncovered_functions = []'
fi
echo "Summary: PASS=$PASSES FAIL=$FAILS SKIP=$SKIPS"
[ "$FAILS" = 0 ]
