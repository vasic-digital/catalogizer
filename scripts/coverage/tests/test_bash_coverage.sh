#!/usr/bin/env bash
# test_bash_coverage.sh - T199 (RED first). Oracle for scripts/bash-coverage.sh, the PS4 line-trace coverage harness (docs/05 7.1, constitution 11.4.224 E).
# Expected numbers are COUNTED BY HAND from the fixtures in scripts/coverage/tests/fixtures (the comments below list the lines), never taken from the harness:
#   cov_fixture.sh   executable lines 3 5 6 7 10 11 13 14 15 17 (10); run with no argument it executes 3 5 6 7 13 14 17 (7) = 70.00 %; uncovered 10 11 15; uncovered_fn is never called
#   cov_fixture2.sh  executable lines 3 4 5 6 8 11 (6) (case arms, a here-document body and a continuation line are not separate lines); it executes 3 4 5 8 11 (5) = 83.33 %
#   cov_excluded.sh  executable lines 3 4 5 (3); it executes 3 4 (2); the fence excludes it, so it must not enter the totals
#   cov_multiline.sh a 2-line assignment (counted line 2), a 3-line printf word (counted line 4), echo (7), an if (8) and a never-run 2-line assignment (9): executable 2 4 7 8 9, executed 2 4 7 8 = 80.00
#   cov_arrays.sh    arrays opened by local, declare, readonly and typeset: executable 3 7 10 12 15 17 (6), all executed
#   cov_arith.sh     `x=$(( 1 << y ))` in a function that is never called: executable 3 4 5 7 (4), executed 7 = 25.00
#   cov_hdoc2.sh     `<<END-DATA`, `<<WORD` in a comment, two here-documents on one line: executable 2 5 6 11 (4), all executed
#   cov_procsub.sh   `done < <(echo a; echo b)` is a command line: executable 2 3 4 5 6 (5), all executed
#   lib_trap.sh      one-line function (1), work (3 4), fail_it (7): executable 1 3 4 7; cov_trap.sh runs work (a USR1 trap) and fail_it (an ERR trap): executed 3 4 7 = 75.00
#   cov_case.sh      bare case labels `a)` `b)` are not lines: executable 2 3 5 8 11 (5); run with a it executes 2 3 5 11 = 80.00, uncovered 8
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
BCOV="$(dirname "$HARNESS")/coverage/bashcov.py"
run() { bash "$HARNESS" "$@" >"$T/out" 2>"$T/err"; RC=$?; }
jqf() { jq -r "$2" "$1"; }
J_() { jq -r "$2" "$T/$1/bash-coverage.json"; }   # J_ OUTDIR FILTER

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
for k in '${#BASH_SOURCE}' '${BASH_SOURCE}' '${LINENO}' '${PWD}' 'sha256sum' '$$'; do
  case "$(jqf $J .ps4)" in *"$k"*) ok "fixture 1: the PS4 in the record carries $k";; *) bad "fixture 1: the PS4 lacks $k: $(jqf $J .ps4)";; esac
done
check "fixture 1: the record carries the sha256 of the target and of the trace" "$(jqf $J '(.targets[0].sha256|length|tostring) + ":" + (.trace_sha256|length|tostring)')" "64:64"
check "fixture 1: a clean run has status ok (no classifier disagreement)" "$(jqf $J .status)" ok
jqf $J '.limits | join(" | ")' | grep -qi 'line, not branch' && ok "fixture 1: the honest limit (line, not branch) is written into the record" || bad "fixture 1: limit text missing"
jqf $J '.limits | join(" | ")' | grep -q 'set +x' && ok "fixture 1: the set +x limit is written into the record" || bad "fixture 1: set +x limit missing"
jqf $J '.limits | join(" | ")' | grep -q 'LINENO 1' && grep -q 'multi-line command substitution' <<<"$(jqf $J '.limits | join(" | ")')" && ok "fixture 1: the trap and the multi-line substitution limits are written into the record" || bad "fixture 1: trap / substitution limits missing"
# K12.6: the primary outputs are named so that .gitignore does not drop them
for f in trace.txt command.stdout.txt bash-coverage.json; do [ -e "$T/o1/$f" ] && ok "K12.6: $f exists in the output directory" || bad "K12.6: $f missing"; done
mkdir -p "$T/ign/run1"; cp "$REPO/.gitignore" "$T/ign/"; ( cd "$T/ign" && git init -q . ) 2>/dev/null
for f in trace.txt command.stdout.txt bash-coverage.json command.err; do : >"$T/ign/run1/$f"; ( cd "$T/ign" && git check-ignore -q "run1/$f" ); r=$?; [ "$r" = 1 ] && ok "K12.6: run1/$f is NOT ignored by the project's .gitignore (git check-ignore exit 1)" || bad "K12.6: run1/$f is ignored by .gitignore (exit $r)"; done
( cd "$T/ign" && git check-ignore -q run1/trace.log ) && ok "K12.6 control: the old name trace.log IS ignored (so the check can fail)" || bad "K12.6 control: trace.log is not ignored: the check cannot fail"
# the argument changes which branch runs: with `never` the branch executes (line 15) and the fixture covers 8 of 10
run --src-root "$FX" --out "$T/o1b" --target cov_fixture.sh -- bash "$FX/cov_fixture.sh" never
check "fixture 1 with the branch taken: executed 8 (line 15 now executes)" "$(J_ o1b '.targets[0].executed')" 8
# --- fixture 2: executable 6, executed 5, 83.33 %
run --src-root "$FX" --out "$T/o2" --target cov_fixture2.sh -- bash "$FX/cov_fixture2.sh"
J2="$T/o2/bash-coverage.json"
check "fixture 2: executable (hand count 6)" "$(jqf $J2 '.targets[0].executable')" 6
check "fixture 2: executed (hand count 5)" "$(jqf $J2 '.targets[0].executed')" 5
check "fixture 2: percent" "$(jqf $J2 '.targets[0].percent')" "83.33"
check "fixture 2: the uncovered line is the case arm 6" "$(jqf $J2 '.targets[0].uncovered_lines | map(tostring) | join(" ")')" "6"
check "fixture 2: no traced line outside the classifier's executable set" "$(jqf $J2 '.targets[0].traced_non_executable | length')" 0
# --- a driver that runs three scripts through CHILD bash processes; the fence excludes the vendored one
# (the fence legs run over a COPY of the fixtures outside any git work tree, so the root is enumerated by a walk and the vendored fixture's provenance files count whatever the commit state)
FXC="$T/fxc"; cp -r "$FX" "$FXC"; printf 'MIT License\n' >"$FXC/vendor/LICENSE"; printf 'upstream: https://example.org/vendored/cov_excluded (fixture)\n' >"$FXC/vendor/UPSTREAM"; mkdir -p "$T/fx"; cat >"$T/fx/fixtures.yaml" <<'Y'
schema: coverage-exclusions/1
application: fixtures
exclusions:
  - path: "vendor/cov_excluded.sh"
    class: vendored-third-party
    justification: "a vendored third-party script copied in unmodified, owned by its upstream"
Y
printf 'schema: coverage-tools/1\napps:\n  fixtures: {kind: fence, root: "%s", note: "the harness applies the fence file itself"}\n  fixtures-bad: {kind: fence, root: "%s", note: "the harness applies the fence file itself"}\n  fixtures-false-class: {kind: fence, root: "%s", note: "the harness applies the fence file itself"}\n' "$FXC" "$FXC" "$FXC" >"$T/fx/tools.yaml"
TOOLS="$T/fx/tools.yaml"
run --src-root "$FXC" --out "$T/o3" --target cov_fixture.sh --target cov_fixture2.sh --target vendor/cov_excluded.sh --exclusions "$T/fx/fixtures.yaml" --tools "$TOOLS" -- bash "$FXC/cov_driver.sh"
check "driver with a fence: exit 0" "$RC" 0
J3="$T/o3/bash-coverage.json"
check "driver: child processes are traced (totals 12 executed of 16 executable)" "$(jqf $J3 '.totals.executed')/$(jqf $J3 '.totals.executable')" "12/16"
check "driver: the excluded file is not in the targets" "$(jqf $J3 '[.targets[].file] | join(" ")')" "cov_fixture.sh cov_fixture2.sh"
check "driver: the excluded file is listed with its class" "$(jqf $J3 '.excluded[0].file + ":" + .excluded[0].class')" "vendor/cov_excluded.sh:vendored-third-party"
check "driver: the totals percent is 75.00" "$(jqf $J3 '.totals.percent')" "75.00"
check "K8.1: the applied fence is the SNAPSHOT in the run directory and its sha256 is in the record" "$(jqf $J3 '(.exclusions_file | endswith("/fence/fixtures.yaml")|tostring) + ":" + (.exclusions_sha256 | length | tostring)')" "true:64"
check "K8.1: the record's fence sha256 is the sha256 of the original fence file" "$(jqf $J3 .exclusions_sha256)" "$(sha256sum <"$T/fx/fixtures.yaml" | cut -d' ' -f1)"
run --src-root "$FX" --out "$T/o4" --target cov_fixture.sh --target cov_fixture2.sh --target vendor/cov_excluded.sh -- bash "$FX/cov_driver.sh"
check "driver without a fence: all three count (14 executed of 19 executable)" "$(J_ o4 '.totals.executed')/$(J_ o4 '.totals.executable')" "14/19"
# an unjustified / first-party-without-item fence is refused by the T200 gate BEFORE any measuring
cat >"$T/fx/fixtures-bad.yaml" <<'Y'
schema: coverage-exclusions/1
application: fixtures-bad
exclusions:
  - path: "vendor/cov_excluded.sh"
    class: first-party
    justification: "excluded because it is inconvenient to cover"
Y
run --src-root "$FXC" --out "$T/o5" --target cov_fixture.sh --exclusions "$T/fx/fixtures-bad.yaml" --tools "$TOOLS" -- bash "$FXC/cov_fixture.sh"
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
run --src-root "$FXC" --out "$T/o5b" --target cov_fixture.sh --target cov_fixture2.sh --exclusions "$T/fx/fixtures-false-class.yaml" --tools "$TOOLS" -- bash "$FXC/cov_fixture.sh"
[ "$RC" = 3 ] && grep -q 'class generated-code is not true' "$T/err" && ok "I5: a fence whose class is false of the file it excludes is refused by the harness (exit 3, content-checked through --root)" || bad "I5: false class accepted by the harness ($RC): $(cat "$T/err")"
# K8.1: the fence is gated and APPLIED as one snapshot. A command that rewrites the original fence mid-run (adding the file under test as generated-code) cannot change the applied scope
mkdir -p "$T/toc/src"; cp "$FX/cov_fixture.sh" "$T/toc/src/t.sh"; cp "$FX/cov_fixture.sh" "$T/toc/src/u.sh"
printf 'schema: coverage-exclusions/1\napplication: toc\nexclusions: []\n' >"$T/toc/toc.yaml"
printf 'schema: coverage-tools/1\napps:\n  toc: {kind: fence, root: "%s", note: "the harness applies the fence file itself"}\n' "$T/toc/src" >"$T/toc/tools.yaml"
cat >"$T/toc/rewrite.sh" <<'F'
#!/usr/bin/env bash
bash "$1/t.sh" >/dev/null
printf 'schema: coverage-exclusions/1\napplication: toc\nexclusions:\n  - path: "u.sh"\n    class: vendored-third-party\n    justification: "swapped in while the run is in progress, never gated"\n' >"$2"
F
run --src-root "$T/toc/src" --out "$T/o-toc" --target t.sh --target u.sh --exclusions "$T/toc/toc.yaml" --tools "$T/toc/tools.yaml" -- bash "$T/toc/rewrite.sh" "$T/toc/src" "$T/toc/toc.yaml"
check "K8.1: a fence rewritten during the run does not change the applied scope: u.sh stays a target (both counted: 7 of 20 = 35.00)" "$(J_ o-toc '.totals.executed')/$(J_ o-toc '.totals.executable')" "7/20"
check "K8.1: nothing is excluded" "$(J_ o-toc '.excluded | length')" 0
# the command's failure is never swallowed
cat >"$T/failing.sh" <<'F'
#!/usr/bin/env bash
bash "$1/cov_fixture.sh" >/dev/null
exit 7
F
run --src-root "$FX" --out "$T/o6" --target cov_fixture.sh -- bash "$T/failing.sh" "$FX"
check "a failing command: the harness exits with the command's status" "$RC" 7
check "a failing command: the record says command_failed with the status" "$(J_ o6 '.status + ":" + (.command_rc|tostring)')" "command_failed:7"
# the instrument must see: a command that runs no bash at all leaves an empty trace and is refused (control needle), never reported as 0 %
run --src-root "$FX" --out "$T/o7" --target cov_fixture.sh -- true
[ "$RC" = 3 ] && grep -q 'trace_empty' "$T/err" && ok "an empty trace is refused as trace_empty (exit 3), never reported as 0 percent" || bad "empty trace not refused ($RC): $(cat "$T/err")"
# a target that the command never runs, while the trace is not empty, is a legitimate 0 percent
run --src-root "$FX" --out "$T/o8" --target cov_fixture.sh --target cov_fixture2.sh -- bash "$FX/cov_fixture2.sh"
check "a target the command never ran is reported 0.00 (the trace is not empty)" "$(J_ o8 '.targets[] | select(.file=="cov_fixture.sh") | .percent')" "0.00"
# refusals
run --src-root "$FX" --out "$T/o9" --target no_such_file.sh -- bash "$FX/cov_fixture.sh"
[ "$RC" = 3 ] && grep -q 'target_missing' "$T/err" && ok "a missing target is refused (exit 3 target_missing)" || bad "missing target not refused ($RC)"
# --- review I3: attribution by PATH. Two targets with one base name are legal; a same-basename file that is not the target earns it nothing.
mkdir -p "$T/col/a" "$T/col/b" "$T/col/other/b"; cp "$FX/cov_fixture.sh" "$T/col/a/x.sh"; cp "$FX/cov_fixture.sh" "$T/col/b/x.sh"; cp "$FX/cov_fixture.sh" "$T/col/other/b/x.sh"
run --src-root "$T/col" --out "$T/o10" --target a/x.sh --target b/x.sh -- bash "$T/col/b/x.sh"
check "I3: two targets sharing a base name are accepted (exit 0)" "$RC" 0
check "I3: only the run one earns its lines (b/x.sh 70.00)" "$(J_ o10 '.targets[] | select(.file=="b/x.sh") | .percent')" "70.00"
check "I3: the sibling with the same base name that never ran stays 0.00 (control: the collision no longer shares credit)" "$(J_ o10 '.targets[] | select(.file=="a/x.sh") | .percent')" "0.00"
run --src-root "$T/col" --out "$T/o10b" --target a/x.sh -- bash "$T/col/other/b/x.sh"
check "I3 probe: running a DIFFERENT x.sh does not credit the target a/x.sh (0.00, was 30.00 on the basename rule)" "$(J_ o10b '.targets[0].percent')" "0.00"
check "I3 probe: the foreign same-basename file is NAMED in the record" "$(J_ o10b '.targets[0].attribution.foreign_same_basename | map(endswith("other/b/x.sh")) | any')" true
check "I3 probe: no traced line is credited to the non-target" "$(J_ o10b '.targets[0].traced_non_executable | length')" 0
( cd "$T/col" && bash "$HARNESS" --src-root "$T/col" --out "$T/o10c" --target a/x.sh -- bash other/b/x.sh ) >/dev/null 2>&1
check "I3 probe: a RELATIVE path that resolves to a different file is not credited" "$(J_ o10c '.targets[0].percent')" "0.00"
( cd "$T/col" && bash "$HARNESS" --src-root "$T/col" --out "$T/o10d" --target a/x.sh -- bash a/x.sh ) >/dev/null 2>&1
check "I3 golden-false: a RELATIVE path that resolves to the target is credited (70.00)" "$(J_ o10d '.targets[0].percent')" "70.00"
# a copy of the tree that keeps the layout (the way tests/test_build_system.sh copies Build/) is credited; a root-level target has no layout and is not
mkdir -p "$T/copy/a" "$T/copy/b"; cp "$FX/cov_fixture.sh" "$T/copy/a/x.sh"
run --src-root "$T/col" --out "$T/o10e" --target a/x.sh -- bash "$T/copy/a/x.sh"
check "I3: a copy that keeps the target's relative layout (.../a/x.sh) is credited (70.00)" "$(J_ o10e '.targets[0].percent')" "70.00"
check "I3: the copy path is recorded as a copy" "$(J_ o10e '.targets[0].attribution.copy_paths | length')" 1
mkdir -p "$T/rootsrc"; cp "$FX/cov_fixture.sh" "$T/rootsrc/x.sh"; cp "$FX/cov_fixture.sh" "$T/copy/b/x.sh"
run --src-root "$T/rootsrc" --out "$T/o10f" --target x.sh -- bash "$T/copy/b/x.sh"
check "I3: a root-level target is not credited by any same-basename file elsewhere (0.00)" "$(J_ o10f '.targets[0].percent')" "0.00"
run --src-root "$T/col" --out "$T/o10g" --target a/x.sh --target ./a/x.sh -- bash "$T/col/a/x.sh"
[ "$RC" = 3 ] && grep -q 'duplicate_target' "$T/err" && ok "I3: two --target arguments naming ONE file are refused (3 duplicate_target)" || bad "duplicate target not refused ($RC): $(cat "$T/err")"
# --- K3.1: a relative path is resolved against the $PWD the TRACED shell had, never against the directory the harness was started in
mkdir -p "$T/cd/sub"; cp "$FX/cov_fixture.sh" "$T/cd/x.sh"; cp "$FX/cov_fixture.sh" "$T/cd/sub/x.sh"
( cd "$T/cd" && bash "$HARNESS" --src-root "$T/cd" --out "$T/o-cd1" --target x.sh -- bash -c 'cd sub && bash x.sh' ) >/dev/null 2>&1
check "K3.1: a never-run x.sh is NOT credited by the x.sh a child ran after `cd sub` (0.00, was 100.00)" "$(J_ o-cd1 '.targets[0].percent')" "0.00"
( cd "$T/cd" && bash "$HARNESS" --src-root "$T/cd" --out "$T/o-cd2" --target sub/x.sh -- bash -c 'cd sub && bash x.sh' ) >/dev/null 2>&1
check "K3.1 golden-false: the x.sh it really ran (sub/x.sh) is credited (70.00)" "$(J_ o-cd2 '.targets[0].percent')" "70.00"
( cd "$T/cd/sub" && bash "$HARNESS" --src-root "$T/cd" --out "$T/o-cd3" --target x.sh -- bash x.sh ) >/dev/null 2>&1
check "K3.1 (RM-C): the harness started in sub/ runs sub/x.sh; the root target x.sh is not credited (0.00)" "$(J_ o-cd3 '.targets[0].percent')" "0.00"
# K4.4: a directory name that contains `:12:`, and one that contains the 0x1f delimiter byte itself, cannot split the record
mkdir -p "$T/col2/a:12:b"; cp "$FX/cov_fixture.sh" "$T/col2/a:12:b/x.sh"
run --src-root "$T/col2" --out "$T/o-colon" --target "a:12:b/x.sh" -- bash "$T/col2/a:12:b/x.sh"
check "K4.4: a path with :12: in a directory name is credited (70.00, was 0.00 with exit 0)" "$(J_ o-colon '.targets[0].percent')" "70.00"
US="$(printf '\037')"; mkdir -p "$T/col3/d${US}e"; cp "$FX/cov_fixture.sh" "$T/col3/d${US}e/x.sh"
run --src-root "$T/col3" --out "$T/o-us" --target "d${US}e/x.sh" -- bash "$T/col3/d${US}e/x.sh"
check "K4.4: a path that contains the delimiter byte itself is credited (length-prefixed, 70.00)" "$(J_ o-us '.targets[0].percent')" "70.00"
# --- K3.2: copy credit needs content identity captured in the run
mkdir -p "$T/ci/src/scripts" "$T/ci/other/vendor-project/scripts"; cp "$FX/cov_fixture.sh" "$T/ci/src/scripts/lib.sh"; { cat "$FX/cov_fixture.sh"; echo "# a different file"; } >"$T/ci/other/vendor-project/scripts/lib.sh"
run --src-root "$T/ci/src" --out "$T/o-ci1" --target scripts/lib.sh -- bash "$T/ci/other/vendor-project/scripts/lib.sh"
check "K3.2: an unrelated file with the same path suffix and DIFFERENT content earns nothing (0.00, was 70.00)" "$(J_ o-ci1 '.targets[0].percent')" "0.00"
check "K3.2: and it is listed as copy_unverified" "$(J_ o-ci1 '.targets[0].attribution.copy_unverified | length')" 1
cat >"$T/ci/mimic.sh" <<'F'
#!/usr/bin/env bash
# mimics tests/test_build_system.sh: copy the tree to a temp dir, run the copy, delete the copy BEFORE the report
d="$(mktemp -d)"; mkdir -p "$d/scripts"; cp "$1/scripts/lib.sh" "$d/scripts/lib.sh"; bash "$d/scripts/lib.sh" >/dev/null; rm -rf "$d"
F
run --src-root "$T/ci/src" --out "$T/o-ci2" --target scripts/lib.sh -- bash "$T/ci/mimic.sh" "$T/ci/src"
check "K3.2: an IDENTICAL copy that the command deletes before it returns is credited (70.00)" "$(J_ o-ci2 '.targets[0].percent')" "70.00"
check "K3.2: and recorded as a copy" "$(J_ o-ci2 '.targets[0].attribution.copy_paths | length')" 1
# --- K3.3: the action of a trap is reported by bash at LINENO 1 of whatever file runs: it is not credited to line 1
run --src-root "$FX" --out "$T/o-trap" --target lib_trap.sh -- bash "$FX/cov_trap.sh"
check "K3.3: lib_trap.sh executable (hand count 4: 1 3 4 7)" "$(J_ o-trap '.targets[0].executable')" 4
check "K3.3: executed 3 4 7 = 75.00 (a USR1 trap and an ERR trap did not credit line 1; was 100.00)" "$(J_ o-trap '.targets[0].percent')" "75.00"
check "K3.3: the uncovered line is 1" "$(J_ o-trap '.targets[0].uncovered_lines | map(tostring) | join(" ")')" "1"
check "K3.3: no classifier disagreement (the line-1 trap records are listed as ignored, not as disagreement)" "$(J_ o-trap '.status')" ok
# --- K3.4: a target spelled ./x or lib/../x is the file the fence names; a target outside --src-root is refused
mkdir -p "$T/ts/vendor" "$T/ts/lib"; cp "$FX/cov_fixture.sh" "$T/ts/vendor/v.sh"; cp "$FX/cov_fixture.sh" "$T/ts/lib/l.sh"; printf 'MIT License\n' >"$T/ts/vendor/LICENSE"; printf 'upstream: https://example.org/v\n' >"$T/ts/vendor/UPSTREAM"
printf 'schema: coverage-exclusions/1\napplication: ts\nexclusions:\n  - path: "vendor/**"\n    class: vendored-third-party\n    justification: "a vendored third-party tree copied in unmodified"\n' >"$T/ts/ts.yaml"
printf 'schema: coverage-tools/1\napps:\n  ts: {kind: fence, root: "%s", note: "the harness applies the fence file itself"}\n' "$T/ts" >"$T/ts/tools.yaml"
for sp in "vendor/v.sh" "./vendor/v.sh" "lib/../vendor/v.sh"; do
  run --src-root "$T/ts" --out "$T/o-ts-$(echo "$sp" | tr -c 'a-z\n' '_')" --target "$sp" --target lib/l.sh --exclusions "$T/ts/ts.yaml" --tools "$T/ts/tools.yaml" -- bash "$T/ts/lib/l.sh"
  check "K3.4: the target spelled '$sp' is excluded by the fence (one target left)" "$(jq -r '[.targets[].file] | join(" ")' "$T/o-ts-$(echo "$sp" | tr -c 'a-z\n' '_')/bash-coverage.json")" "lib/l.sh"
done
run --src-root "$T/ts" --out "$T/o-ts-out" --target ../outside/o.sh -- bash "$T/ts/lib/l.sh"
[ "$RC" = 3 ] && grep -q 'target_outside_src' "$T/err" && ok "K3.4: a target outside --src-root is refused (3 target_outside_src)" || bad "K3.4: outside target not refused ($RC): $(cat "$T/err")"
# --- K4.1: lines inside multi-line words and arrays are not lines of their own
run --src-root "$FX" --out "$T/o-ml" --target cov_multiline.sh -- bash "$FX/cov_multiline.sh"
check "K4.1: cov_multiline.sh executable (hand count 5: 2 4 7 8 9)" "$(J_ o-ml '.targets[0].executable')" 5
check "K4.1: executed 4 = 80.00 (a fully run multi-line word is covered, was 42.86 for the unaliased lines)" "$(J_ o-ml '.targets[0].percent')" "80.00"
check "K4.1: the only uncovered line is 9 (the never-run 2-line assignment counts ONCE)" "$(J_ o-ml '.targets[0].uncovered_lines | map(tostring) | join(" ")')" "9"
check "K4.1: no classifier disagreement" "$(J_ o-ml .status)" ok
run --src-root "$FX" --out "$T/o-arr" --target cov_arrays.sh -- bash "$FX/cov_arrays.sh"
check "K4.1: arrays opened by local / declare / readonly / typeset count ONE line each (executable 6)" "$(J_ o-arr '.targets[0].executable')" 6
check "K4.1: all six are executed, no element line is uncovered" "$(J_ o-arr '.targets[0].executed|tostring') $(J_ o-arr '.targets[0].uncovered_lines|length|tostring')" "6 0"
# the real Build/lib/version.sh: its multi-line words (46-51, 74-82, 90-107) must never be reported uncovered
cat >"$T/vdrive.sh" <<'F'
#!/usr/bin/env bash
export BUILD_PROJECT_ROOT="$1/proj" BUILD_VERSIONS_FILE="$1/proj/versions.json"; mkdir -p "$BUILD_PROJECT_ROOT"
. "$2/Build/lib/version.sh"
init_versions >/dev/null 2>&1 || true
_json_write "global.major" 3 >/dev/null 2>&1 || true
_json_read "global.major" >/dev/null 2>&1 || true
F
run --src-root "$REPO" --out "$T/o-ver" --target Build/lib/version.sh -- bash "$T/vdrive.sh" "$T" "$REPO"
check "K4.1: version.sh driven through init_versions, _json_write, _json_read: exit 0 and status ok" "$RC $(J_ o-ver .status)" "0 ok"
check "K4.1: none of version.sh 46-51 / 74-82 / 90-107 is reported uncovered or counted on its own (the committed record counted 33 of them)" "$(J_ o-ver '[.targets[0].uncovered_lines[] | select((. >= 47 and . <= 51) or (. >= 75 and . <= 82) or (. >= 91 and . <= 107))] | length')" 0
# --- K4.2: the here-document rules
run --src-root "$FX" --out "$T/o-ar" --target cov_arith.sh -- bash "$FX/cov_arith.sh"
check "K4.2: an arithmetic << is not a here-document: executable 4 (3 4 5 7), executed 1 = 25.00 (was 66.67)" "$(J_ o-ar '.targets[0].executable')/$(J_ o-ar '.targets[0].percent')" "4/25.00"
run --src-root "$FX" --out "$T/o-hd" --target cov_hdoc2.sh -- bash "$FX/cov_hdoc2.sh"
check "K4.2: <<END-DATA is a here-document delimited by END-DATA, <<WORD in a comment is not one, two here-documents on one line both end: executable 4, 100.00" "$(J_ o-hd '.targets[0].executable')/$(J_ o-hd '.targets[0].percent')/$(J_ o-hd '.targets[0].traced_non_executable|length')" "4/100.00/0"
# --- K4.3: process substitution on a done line is a command line
run --src-root "$FX" --out "$T/o-ps" --target cov_procsub.sh -- bash "$FX/cov_procsub.sh"
check "K4.3: done < <(echo a; echo b) is executable: executable 5, executed 5, no disagreement" "$(J_ o-ps '.targets[0].executable')/$(J_ o-ps '.targets[0].executed')/$(J_ o-ps .status)" "5/5/ok"
# --- CM19 (adopted): a bare case label is not a line
run --src-root "$FX" --out "$T/o-case" --target cov_case.sh -- bash "$FX/cov_case.sh"
check "CM19: bare case labels a) and b) are not lines: executable 5 (2 3 5 8 11), executed 4 = 80.00, uncovered 8" "$(J_ o-case '.targets[0].executable')/$(J_ o-case '.targets[0].percent')/$(J_ o-case '.targets[0].uncovered_lines|map(tostring)|join(",")')" "5/80.00/8"
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
( cd "$T" && bash "$HARNESS" --src-root "$FXC" --out abs-out --target cov_fixture.sh --exclusions fx/fixtures.yaml --tools fx/tools.yaml -- bash "$T/cdrun.sh" "$T/elsewhere" "$FXC" ) >/dev/null 2>&1
check "I4: a RELATIVE --exclusions file is resolved before any cd (the fence still applies)" "$(jqf $T/abs-out/bash-coverage.json '.exclusions_file | startswith("/")')" true
# --- review i1: the classifier counts the elements of a pipeline / and-or continuation (the tracer reports each one's own line)
run --src-root "$FX" --out "$T/o14" --target cov_pipe.sh -- bash "$FX/cov_pipe.sh"
check "i1: pipeline fixture executable lines (hand count 8: 3 4 5 6 7 8 9 10)" "$(J_ o14 '.targets[0].executable')" 8
check "i1: pipeline fixture executed (8)" "$(J_ o14 '.targets[0].executed')" 8
check "i1: no traced line is classified non-executable (the classifier agrees with the tracer)" "$(J_ o14 '.targets[0].traced_non_executable | length')" 0
# measured on Build/lib/hash.sh (a 12-line array literal at 31-42 traced as line 42 alone): the array counts ONE line, now its first
run --src-root "$FX" --out "$T/o15" --target cov_array.sh -- bash "$FX/cov_array.sh"
check "i1: a multi-line array assignment counts ONE executable line (fixture hand count 2: lines 3 and 7)" "$(J_ o15 '.targets[0].executable')" 2
check "i1: both are executed (the assignment is traced at its closing line 6 by bash 5.3, folded onto the group's counted line 3)" "$(J_ o15 '.targets[0].executed_lines | map(tostring) | join(",")')" "3,7"
check "i1: no classifier disagreement on the array fixture (status ok)" "$(J_ o15 .status)" ok
# the tracer's line for a command that spans lines depends on the bash VERSION and on the construct: the report folds any line of a group onto the counted line,
# proven here on SYNTHETIC traces (new record format) so the leg does not depend on the bash that runs the suite.
syn() { # syn NAME FILE line... : a hand-made trace of FILE (absolute path) with the given lines, reported through bashcov.py directly
  local n="$1" f="$2"; shift 2; : >"$T/syn-$n.trace"; local p="$FX/$f"
  for l in "$@"; do printf '+COV\037%d\037%s\037%s\037%s\037%s\037%s\037cmd\n' "${#p}" "$p" "$l" 1 / "$(sha256sum <"$p" | cut -d' ' -f1)" >>"$T/syn-$n.trace"; done
  printf '%s  %s\n' "$(sha256sum <"$FX/$f" | cut -d' ' -f1)" "$f" >"$T/syn-$n.sha"
  python3 -I "$BCOV" report --src-root "$FX" --trace "$T/syn-$n.trace" --out "$T/syn-$n.json" --pre-sha "$T/syn-$n.sha" --cwd / --command syn --command-rc 0 --bash-version synthetic -- "$f" >/dev/null 2>"$T/syn-$n.err"; SYNRC=$?; }
syn p53 cov_pipe.sh 3 4 5 6 7 8 9 10
check "i1 synthetic (bash 5.3 style, the continuation traced at its first line 10): all 8 executed, none foreign" "$(jqf $T/syn-p53.json '(.targets[0].executed|tostring) + "/" + (.targets[0].traced_non_executable|length|tostring)')" "8/0"
syn p52 cov_pipe.sh 3 4 5 6 7 8 9 11
check "i1 synthetic (bash 5.2 style, the continuation traced at its LAST line 11): the same 8 executed, none foreign" "$(jqf $T/syn-p52.json '(.targets[0].executed|tostring) + "/" + (.targets[0].traced_non_executable|length|tostring)')" "8/0"
check "i1 synthetic: the status is ok for both styles" "$(jqf $T/syn-p52.json .status)" ok
syn p00 cov_pipe.sh 3 4 5 6 7 8 9
check "i1 synthetic control: with the continuation line never traced the command is NOT executed (7 of 8)" "$(jqf $T/syn-p00.json '(.targets[0].executed|tostring) + "/" + (.targets[0].executable|tostring)')" "7/8"
syn a3 cov_array.sh 3 7
check "i1 synthetic: a multi-line array reported at its OPENING line is folded onto its counted line: 2 of 2" "$(jqf $T/syn-a3.json '(.targets[0].executed|tostring) + "/" + (.targets[0].executable|tostring)')" "2/2"
syn a6 cov_array.sh 6 7
check "i1 synthetic: the same array reported at its CLOSING line (bash 5.3 style) is folded onto its counted line: 2 of 2" "$(jqf $T/syn-a6.json '(.targets[0].executed|tostring) + "/" + (.targets[0].executable|tostring)')" "2/2"
syn m3 cov_multiline.sh 2 3 4 7 8
check "K4.1 synthetic: an assignment traced at its LAST line (3) and a word traced at its first line (4): both folded, 4 of 5" "$(jqf $T/syn-m3.json '(.targets[0].executed|tostring) + "/" + (.targets[0].executable|tostring)')" "4/5"
check "i1: an argument continuation (line 11) is not counted" "$(J_ o14 '[.targets[0].executed_lines[] | select(.==11)] | length')" 0
# K4.5 (RM-A): a trace line the classifier calls non-executable is a REFUSAL (exit 3, status classifier_disagreement, no figure), never a number
syn dis cov_fixture.sh 3 5 6 7 8 13 14 17
check "K4.5 (RM-A): a traced structural line (8, a closing brace) makes the report exit 3" "$SYNRC" 3
check "K4.5: the record says classifier_disagreement and carries NO percent" "$(jqf $T/syn-dis.json '.status + ":" + (.totals.percent|tostring)')" "classifier_disagreement:null"
grep -q 'classifier_disagreement' "$T/syn-dis.err" && ok "K4.5: the refusal is on stderr" || bad "K4.5: no refusal text: $(cat "$T/syn-dis.err")"
syn ok1 cov_fixture.sh 3 5 6 7 13 14 17
check "K4.5 control: the same synthetic trace without the stray line reports 70.00 and exits 0" "$SYNRC $(jqf $T/syn-ok1.json '.totals.percent')" "0 70.00"
# a trace record past the end of the file (a bash defect inside multi-line substitutions) is listed, not credited and not a disagreement
syn oor cov_fixture.sh 3 5 6 7 13 14 17 40
check "K4.5: a record at line 40 of a 17-line file is listed in trace_out_of_range and does not refuse" "$SYNRC $(jqf $T/syn-oor.json '(.targets[0].trace_out_of_range|map(tostring)|join(","))')" "0 40"
# K6.5: --out with $( ) must not run code (BASH_ENV is expanded by bash)
mkdir -p "$T/inj"; run --src-root "$FX" --out "$T/inj/"'o$(touch '"$T/inj/EXPANDED"')' --target cov_fixture.sh -- bash "$FX/cov_fixture.sh"
[ "$RC" = 3 ] && grep -q 'out_unsafe_path' "$T/err" && [ ! -e "$T/inj/EXPANDED" ] && ok "K6.5: an --out path with \$( ) is refused (3 out_unsafe_path) and no code ran" || bad "K6.5: rc=$RC EXPANDED=$([ -e "$T/inj/EXPANDED" ] && echo yes || echo no): $(cat "$T/err")"
# K6.6: no --out writes under $PWD/.audit/out when /out is not writable
mkdir -p "$T/cwd6"
if [ -d /out ] && [ -w /out ]; then skip "K6.6: /out is writable on this host, so the default is /out (the \$PWD/.audit/out leg is not applicable here)"
else ( cd "$T/cwd6" && bash "$HARNESS" --src-root "$FX" --target cov_fixture.sh -- bash "$FX/cov_fixture.sh" ) >/dev/null 2>&1
  check "K6.6: the default --out is created under \$PWD/.audit/out/ (one record)" "$(ls "$T"/cwd6/.audit/out/bash-coverage-*/bash-coverage.json 2>/dev/null | wc -l | tr -d ' ')" 1; fi
# K7.1: a child that outlives the command is a refusal, whatever the timing
cat >"$T/bg.sh" <<'F'
#!/usr/bin/env bash
bash "$1/cov_fixture.sh" >/dev/null
( sleep "$2"; bash "$1/cov_fixture2.sh" >/dev/null ) &
F
run --src-root "$FX" --out "$T/o-bg1" --target cov_fixture.sh --target cov_fixture2.sh -- bash "$T/bg.sh" "$FX" 1
[ "$RC" = 3 ] && grep -q 'trace_writers_outlived_command' "$T/err" && ok "K7.1: a background child still running when the command returns is refused (3 trace_writers_outlived_command)" || bad "K7.1: rc=$RC: $(cat "$T/err")"
check "K7.1: the record carries status trace_writers_outlived_command and no figure" "$(J_ o-bg1 '.status + ":" + (.totals.percent|tostring)')" "trace_writers_outlived_command:null"
sleep 1.5; before="$(stat -c %s "$T/o-bg1/trace.txt")"; sleep 0.5
check "K7.1: the survivor was reaped: the trace does not grow after the run" "$(stat -c %s "$T/o-bg1/trace.txt")" "$before"
run --src-root "$FX" --out "$T/o-bg2" --target cov_fixture.sh -- bash "$FX/cov_fixture.sh"
check "K7.1 control: the same command without a background child is ok (exit 0)" "$RC" 0
# K7.3 / K7.2: INT and TERM sent to the harness's own process group
cat >"$T/sig-driver.sh" <<'D'
#!/usr/bin/env bash
# runs the harness as a job of its own process group (job control), sends a signal to that group after a delay, and records the harness's exit status
set -m
bash "$1" --src-root "$2" --out "$3" --target cov_fixture.sh --target cov_fixture2.sh -- bash "$4" "$2" &
H=$!
sleep "$5"
kill -s "$6" -- "-$H"
wait "$H"; echo "$?" >"$7"; echo "$SECONDS" >"$7.sec"
D
cat >"$T/absorb.sh" <<'F'
#!/usr/bin/env bash
trap : INT
bash "$1/cov_fixture.sh" >/dev/null
sleep 2
exit 0
F
bash "$T/sig-driver.sh" "$HARNESS" "$FX" "$T/o-int" "$T/absorb.sh" 1 INT "$T/int.rc" >/dev/null 2>&1
check "K7.3: INT reaches the harness while the command absorbs it: the harness exits 130 (was 0)" "$(cat "$T/int.rc" 2>/dev/null)" 130
check "K7.3: the record says interrupted and carries no figure" "$(J_ o-int '.status + ":" + (.totals.percent|tostring)')" "interrupted:null"
cat >"$T/orphan.sh" <<'F'
#!/usr/bin/env bash
bash "$1/cov_fixture.sh" >/dev/null
( sleep 1.5; bash "$1/cov_fixture2.sh" >/dev/null ) &
sleep 5
F
bash "$T/sig-driver.sh" "$HARNESS" "$FX" "$T/o-term" "$T/orphan.sh" 0.6 TERM "$T/term.rc" >/dev/null 2>&1
check "K7.2: TERM to the harness: it exits 143 and the record says interrupted" "$(cat "$T/term.rc" 2>/dev/null) $(J_ o-term .status)" "143 interrupted"
[ "$(cat "$T/term.rc.sec" 2>/dev/null || echo 99)" -le 3 ] && ok "K7.2: and it returns within 3 s of the signal: the TERM was forwarded to the command's process group (the command would otherwise run its 5 s)" || bad "K7.2: the harness took $(cat "$T/term.rc.sec" 2>/dev/null) s after TERM: the signal was not forwarded to the group"
sz="$(stat -c %s "$T/o-term/trace.txt")"; sleep 2.2
check "K7.2: no orphan of the TERMed run keeps writing to the trace (the group was signalled and reaped)" "$(stat -c %s "$T/o-term/trace.txt")" "$sz"
run --src-root "$FX" --out "$T/o-term" --target cov_fixture.sh -- bash "$FX/cov_fixture.sh"
[ "$RC" = 3 ] && grep -q 'out_not_empty' "$T/err" && ok "K7.2 / K8.5: a second run into the same --out is refused (3 out_not_empty): the orphan of the first can never be credited to the second" || bad "K7.2: reused --out not refused ($RC): $(cat "$T/err")"
# K8.2: a target rewritten during the run is refused (it would be credited for lines that never ran)
mkdir -p "$T/rw"; cp "$FX/cov_fixture.sh" "$T/rw/cov_fixture.sh"
cat >"$T/rewrite-target.sh" <<'F'
#!/usr/bin/env bash
bash "$1/cov_fixture.sh" >/dev/null
printf '#!/usr/bin/env bash\nn1=1\nn2=2\nn3=3\nn4=4\nn5=5\n' >"$1/cov_fixture.sh"
F
run --src-root "$T/rw" --out "$T/o-rw" --target cov_fixture.sh -- bash "$T/rewrite-target.sh" "$T/rw"
[ "$RC" = 3 ] && grep -q 'target_changed_during_run' "$T/err" && ok "K8.2: a target rewritten during the run is refused (3 target_changed_during_run)" || bad "K8.2: rc=$RC: $(cat "$T/err")"
run --src-root "$FX" --out "$T/o-rw2" --target cov_fixture.sh -- bash "$FX/cov_fixture.sh"
check "K8.2 control: an unchanged target is measured (70.00)" "$(J_ o-rw2 '.targets[0].percent')" "70.00"
run --src-root "$FX" --out "$T/o11"; check "no -- command is a usage error" "$RC" 2
run --src-root "$FX" --out "$T/o12" -- bash "$FX/cov_fixture.sh"; check "no target is a usage error" "$RC" 2
# the harness's own state does not leak: the command receives BASH_ENV and COV_TRACE_FILE pointing into ITS OWN --out directory, and a command run WITHOUT the harness sees neither.
printf '#!/usr/bin/env bash\necho "BASHENV=${BASH_ENV:-unset}"\necho "TRACEFILE=${COV_TRACE_FILE:-unset}"\n' >"$T/env-probe.sh"
run --src-root "$FX" --out "$T/o12b" --target cov_fixture.sh -- bash "$T/env-probe.sh"
grep -qx "BASHENV=$T/o12b/.cov-bashenv.sh" "$T/out" && grep -qx "TRACEFILE=$T/o12b/trace.txt" "$T/out" && ok "m9: the command is handed BASH_ENV and COV_TRACE_FILE inside its own --out directory" || bad "m9: env handed to the command: $(grep -E 'BASHENV|TRACEFILE' "$T/out" | tr '\n' ' ')"
env -u BASH_ENV -u COV_TRACE_FILE bash "$T/env-probe.sh" >"$T/env-bare.txt" 2>&1
grep -qx 'BASHENV=unset' "$T/env-bare.txt" && grep -qx 'TRACEFILE=unset' "$T/env-bare.txt" && ok "m9 control: the same probe run without the harness sees neither variable (so the leg above can fail)" || bad "m9 control: probe sees $(cat "$T/env-bare.txt" | tr '\n' ' ')"
# determinism: a second run over the same fixture gives the same numbers (the record's executed lines are equal)
run --src-root "$FX" --out "$T/o13" --target cov_fixture.sh -- bash "$FX/cov_fixture.sh"
check "determinism: two runs give identical executed lines" "$(J_ o13 '.targets[0].executed_lines | map(tostring) | join(",")')" "$(J_ o1 '.targets[0].executed_lines | map(tostring) | join(",")')"

# --- kcov cross-check (IMG-KCOV): kcov is an independent instrument; on fixture 1 it must report the same executable and executed line COUNTS
#     (measured 2026-10-06 in IMG-KCOV: kcov 7 of 10 lines = 70.00 %, equal to this harness). A disagreement is a FAIL, kcov absent is a named SKIP.
if command -v kcov >/dev/null 2>&1; then
  K="$T/kcov"; mkdir -p "$K"
  kcov --bash-method=DEBUG "$K" "$FX/cov_fixture.sh" >/dev/null 2>&1
  KJ="$(find "$K" -name coverage.json | head -1)"
  if [ -n "$KJ" ]; then
    KC="$(jq -r '[.files[] | select(.file | endswith("cov_fixture.sh"))][0] | (.covered_lines|tostring) + "/" + (.total_lines|tostring)' "$KJ" 2>/dev/null)"
    OURS="$(jqf $J '.targets[0].executed|tostring')/$(jqf $J '.targets[0].executable|tostring')"
    check "kcov cross-check: kcov and this harness agree on the executed/executable line COUNTS of fixture 1" "$KC" "$OURS"
  else skip "kcov cross-check: kcov ran but wrote no coverage.json (UNCONFIRMED)"; fi
else skip "kcov cross-check: kcov is not installed on this host (it is in IMG-KCOV: run through TIC build-scripts unit)"; fi

# --- paired mutations ---
if [ "${1:-}" != --no-mutations ] && [ -z "${HARNESS_MUTANT:-}" ]; then
  REC="${MUTATION_RECORD:-$T/mutations.txt}"; : >"$REC"
  copy_sut() { local d="$1"; rm -rf "$d"; mkdir -p "$d/scripts/coverage"; cp "$REPO/scripts/bash-coverage.sh" "$d/scripts/"; cp "$REPO"/scripts/coverage/*.py "$REPO"/scripts/coverage/*.sh "$d/scripts/coverage/"; mkdir -p "$d/coverage/exclusions"; cp "$REPO/coverage/exclusions/tools.yaml" "$d/coverage/exclusions/"; }
  mut() { # mut NAME FILE OLD NEW   (FILE is bash-coverage.sh, bashcov.py, fence_lib.py or check_exclusions.py)
    local name="$1" f="$2" old="$3" new="$4" d="$T/mut-$1"; copy_sut "$d"
    local tgt="$d/scripts/$f"; case "$f" in *.py) tgt="$d/scripts/coverage/$f";; esac
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
  ctl() { local d="$T/mut-ctl"; copy_sut "$d"
    if HARNESS="$d/scripts/bash-coverage.sh" HARNESS_MUTANT=1 bash "${BASH_SOURCE[0]}" --no-mutations >"$T/mut-ctl.out" 2>&1; then ok "mutation sandbox control: an UNMUTATED copy of the harness passes this body (so a CAUGHT below is the mutation)"; echo "CONTROL PASS" >>"$REC"
    else bad "mutation sandbox control FAILED: the unmutated copy does not pass ($(grep -c '^FAIL:' "$T/mut-ctl.out") failing legs), every CAUGHT below is suspect"; echo "CONTROL FAIL" >>"$REC"; fi; }
  ctl
  mut no-xtrace-fd bash-coverage.sh 'BASH_XTRACEFD=$COV_FD' ':'
  mut structural-counted bashcov.py 'elif sm:' 'elif False:'
  mut exclusions-ignored bashcov.py 'if e:   # MUT:excl' 'if False:'
  mut command-rc-swallowed bash-coverage.sh 'exit "$CMD_RC"' 'exit 0'
  mut empty-trace-accepted bashcov.py 'if trace_lines == 0:' 'if False:'
  mut fence-root-dropped bash-coverage.sh '--root "$SRC" --repo' '--repo'
  mut fence-gate-skipped bash-coverage.sh 'GOUT="$(bash "$GATE" "$SNAP"' 'GOUT="$(true "$SNAP"'
  mut fence-snapshot-ignored bash-coverage.sh '  EXCL="$SNAP"' '  EXCL="$EXCL"'
  mut duplicate-target-unchecked bashcov.py 'if len(set(os.path.realpath(os.path.join(src, t)) for t in targets)) != len(targets):' 'if False:'
  # review round 1 + 5: the attribution and path rules
  mut RM_basename_credit bashcov.py 'if os.path.realpath(full) == absT:   # MUT:direct_realpath' 'if os.path.basename(full) == os.path.basename(absT):   # MUT:direct_realpath'
  mut copy-suffix-dropped bashcov.py 'elif os.path.isabs(p) and "/" in relT and p.endswith("/" + relT):   # MUT:copy_suffix' 'elif False:   # MUT:copy_suffix'
  mut root-level-copy-credit bashcov.py 'elif os.path.isabs(p) and "/" in relT and p.endswith("/" + relT):   # MUT:copy_suffix' 'elif os.path.isabs(p) and p.endswith("/" + relT):   # MUT:copy_suffix'
  mut copy-hash-ignored bashcov.py 'if presha and shas and presha in shas and len(shas) == 1:   # MUT:copy_hash' 'if True:   # MUT:copy_hash'
  mut RM_C_resolve_against_src bashcov.py 'base = first_pwd.setdefault(key, r["pwd"])' 'base = src'
  mut resolve-against-harness-cwd bashcov.py 'base = first_pwd.setdefault(key, r["pwd"])' 'base = os.getcwd()'
  mut relative-out-kept bash-coverage.sh 'OUT="$(cd "$OUT" && pwd)"   # MUT:abs_out' ':'
  mut target-norm-dropped bashcov.py 'rel_ = posixpath.normpath(t)   # MUT:target_norm' 'rel_ = t'
  mut target-hash-ignored bashcov.py 'if pre.get(rel) != post:   # MUT:target_hash' 'if False:'
  mut line1-trap-credited bashcov.py 'if not w or w not in first:   # MUT:line1_trap' 'if False:'
  mut unsafe-out-accepted bash-coverage.sh 'refuse out_unsafe_path "the --out path contains a character that bash expands in BASH_ENV"' ':'
  mut out-not-empty-accepted bash-coverage.sh 'refuse out_not_empty "$OUT already holds files (use a new directory)"' ':'
  mut survivors-ignored bash-coverage.sh 'if [ "$CPID" -gt 1 ] && kill -0 -- "-$CPID" 2>/dev/null; then   # MUT:survivors' 'if false; then'
  mut late-writers-ignored bash-coverage.sh 'if [ "$SURV" = 1 ] || [ "$SIZE1" != "$SIZE0" ]; then   # MUT:late_writers' 'if false; then'
  mut interrupted-ignored bash-coverage.sh 'if [ -n "$INTR" ]; then   # MUT:interrupted' 'if false; then'
  mut group-forwarding-dropped bash-coverage.sh '  kill -s "$1" -- "-$CPID" 2>/dev/null' '  :'
  # the classifier
  mut group-fold-dropped bashcov.py 'kinds[i] = "cont"; alias[i] = group' 'kinds[i] = "cont"'
  mut pipe-continuation-dropped bashcov.py 'pipe_next = bool(re.search(r"(\|\||&&|\|)$", before))' 'pipe_next = False'
  mut array-context-dropped bashcov.py 'stack.append({"k": "array"}); j += 2; continue' 'j += 2; continue'
  mut arith-heredoc-detected bashcov.py 'if k == "arith":' 'if False:'
  mut heredoc-delimiter-first-identifier bashcov.py 'while m < L and raw[m] not in " \t;&|<>()":' 'while m < L and (raw[m].isalnum() or raw[m] == "_"):'
  mut procsub-done-structural bashcov.py '        if "\x02" in rest:' '        if False:'
  mut CM19_case_label_counted bashcov.py 'elif case_depth > 0 and ARM_RE.match(s) and not s.startswith("case "):' 'elif False:'
  mut RM_A_status_never_disagreement bashcov.py 'status = "command_failed" if rc != 0 and not disagree else "classifier_disagreement" if disagree else "ok"' 'status = "command_failed" if rc != 0 else "ok"'
  mut disagreement-exits-zero bashcov.py '    if disagree:
        bad = [' '    if False:
        bad = ['
  mut function-report-dropped bashcov.py 'uncovered_functions = [f["name"] for f in funcs if not (set(f["lines"]) & executed)]' 'uncovered_functions = []'
  mut length-prefix-dropped bashcov.py 'path_b = rest[:n]; after = rest[n + 1:]' 'path_b = rest.split(US, 1)[0]; after = rest.split(US, 1)[1]'
  mut out-of-range-credited bashcov.py 'seen = {alias.get(l, l) for l in seen if l <= nlines}' 'seen = {alias.get(l, l) for l in seen}'
fi
echo "Summary: PASS=$PASSES FAIL=$FAILS SKIP=$SKIPS"
[ "$FAILS" = 0 ]
