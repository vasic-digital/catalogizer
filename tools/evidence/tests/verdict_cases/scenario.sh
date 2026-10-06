#!/usr/bin/env bash
# scenario.sh CASE DIR - the docs/06 s13.1 scenario, ported from the proof of concept to the REAL recorder (tools/evidence/evrec).
# Builds one ledger for one case under DIR: the target adder.sh holds first the broken artifact (computes 2 - 3), then the fixed one
# (2 + 3); the test check.sh exits 0 when the target prints 5. `good` is the golden-good, every other case changes exactly one thing.
# Environment: EVREC (the recorder), FORGE (tests/forge.py). The test and the hermetic environment are the caller's.
#
# Deviations from the proof of concept (stated, 11.4.6):
#  * item id CAT-001 (the schema accepts canonical ids only); --test-source ./check.sh names the test bytes (rule 11), so the
#    test fingerprint is argv[0] plus check.sh and does not change when the TARGET changes (the POC hashed argv[0] only).
#  * three cases hold an entry that the strict recorder REFUSES to write: `blind_red` (a RED that passed), `launder` (a GREEN that
#    failed, a REOPEN that passed) and `reopen_pass` (a REOPEN that passed). They are recorded with polarity PROBE and re-tagged
#    with FORGE retag (a delete-and-recompute style forgery), and are derived with `verdict --chain-only`: the golden-bad then
#    tests the deriver's own rule for an entry the recorder would have refused.
set -euo pipefail
case=$1; d=$2; : "${EVREC:?}" "${FORGE:?}"
rm -rf -- "${d:?}"; mkdir -p "$d"; cd "$d"                          # a re-run starts from an empty ledger
export EV_LEDGER=$d/ledger.jsonl EV_ANCHOR=$d/anchors.jsonl EV_BLOBS=$d/blobs
printf '#!/usr/bin/env bash\necho $(( 2 - 3 ))\n' > broken.sh
printf '#!/usr/bin/env bash\necho $(( 2 + 3 ))\n' > fixed.sh
printf '#!/usr/bin/env bash\necho $(( 3 + 2 ))\n' > fixed2.sh      # the cycle-2 fix: a new build, a new fingerprint
printf '#!/usr/bin/env bash\necho $(( 2 * 3 ))\n' > broken2.sh     # the defect returns after the closure
printf '#!/usr/bin/env bash\n[ "$("$1")" = 5 ]\n' > check.sh
cp check.sh check_other.sh; chmod +x ./*.sh
red_argv=(./check.sh ./adder.sh); red_ref=adder.sh; red_src=(--test-source ./check.sh); iters=(1 2 3)
dep() { cp "$1" adder.sh; chmod +x adder.sh; }
dep broken.sh
OPTS=(--oracle specified --oracle-independent --evidence-class runtime)   # the oracle of the scenario is specified: 2 + 3 must be 5
rec() { local a=$1 b=$2 c=$3 r=$4; shift 4; "$EVREC" run CAT-001 "$a" "$b" "$c" "$r" "${OPTS[@]}" "$@" >/dev/null; }   # rec POL ITER CLASS REF [--test-source ..] -- argv
red() { rec RED 1 shell_script "$red_ref" "${red_src[@]}" -- "${red_argv[@]}"
        [ -e check.sh.away ] && mv -f check.sh.away check.sh; [ -e check.sh.orig ] && mv -f check.sh.orig check.sh
        chmod +x check.sh; return 0; }                              # restore the test for GREEN
green() { dep "${1:-fixed.sh}"; local i; for i in "${iters[@]}"; do rec GREEN "$i" shell_script adder.sh --test-source ./check.sh -- ./check.sh ./adder.sh; done; }
run1() { rec "$1" "${2:-1}" shell_script adder.sh --test-source ./check.sh -- ./check.sh ./adder.sh; }
forged() { # forged POL ITER: run as PROBE, then re-tag the new last entry (an entry the recorder refuses to write under POL)
  rec PROBE "$2" shell_script adder.sh --test-source ./check.sh -- ./check.sh ./adder.sh
  "$FORGE" retag "$EV_LEDGER" "$(wc -l <"$EV_LEDGER")" "$1"; }
case $case in
  good|second_cycle|reopen_no_new|green_on_reopen_fp|green_old_fp|reopen_after_fail|green_first|launder|reopen_pass) : ;;
  blind_red)   dep fixed.sh ;;                                      # RED run on an already-correct artifact (exit 0)
  exit127)     mv check.sh check.sh.away; red_src=() ;;             # same argv, test script absent: exit 127
  exit126)     chmod -x check.sh ;;                                 # same argv, test script not executable: exit 126
  signal)      mv check.sh check.sh.away; printf '#!/usr/bin/env bash\nkill -9 $$\n' > check.sh; chmod +x check.sh ;;
  dash_argv)   red_argv=(bash -c '[ "$("$0")" = 5 ]' ./adder.sh) ;; # argv with a leading-dash element, another test
  other_argv)  red_argv=(./check_other.sh ./adder.sh) ;;            # a different test
  other_target) cp broken.sh adder_old.sh; chmod +x adder_old.sh; red_argv=(./check.sh ./adder_old.sh); red_ref=adder_old.sh ;;
  typo_red)    cp check.sh check.sh.orig                            # RED-only typo ($l for $1), argv unchanged: exit 1, wrong reason
               printf '#!/usr/bin/env bash\n[ "$("$l")" = 5 ]\n' > check.sh; chmod +x check.sh ;;
  green_dup_iter) iters=(1 1 1) ;;                                  # three GREEN entries, one iteration value
  *) echo "scenario.sh: unknown case $case" >&2; exit 64 ;;
esac
[ "$case" = reopen_after_fail ] && iters=(1 2)                      # cycle 1 incomplete: two GREEN only
if [ "$case" = green_first ]; then green; dep broken.sh; red
elif [ "$case" = blind_red ]; then
  rec PROBE 1 shell_script adder.sh --test-source ./check.sh -- ./check.sh ./adder.sh; "$FORGE" retag "$EV_LEDGER" 1 RED
  green
elif [ "$case" = launder ]; then                                    # cycle 1 with a FAILING GREEN (it 2), then a PASSING REOPEN
  red; dep fixed.sh; run1 GREEN 1; dep broken.sh; forged GREEN 2; dep fixed.sh; run1 GREEN 3
  forged REOPEN 1; dep broken.sh; run1 RED; iters=(1 2 3); green     # that would discard it, then a clean cycle
else red; green; fi
case $case in
  reopen_no_new)      dep broken2.sh; run1 REOPEN ;;                                   # recurrence, nothing new
  second_cycle)       dep broken2.sh; run1 REOPEN; run1 RED; iters=(1 2 3); green fixed2.sh ;;  # honest cycle 2
  reopen_after_fail)  dep broken2.sh; run1 REOPEN; run1 RED; iters=(1 2 3); green fixed2.sh ;;  # cut after a FAIL cycle
  reopen_pass)        forged REOPEN 1; dep broken2.sh; run1 RED; iters=(1 2 3); green fixed2.sh ;;  # REOPEN that passed (exit 0)
  green_on_reopen_fp) dep fixed2.sh; chmod -x adder.sh; run1 REOPEN                    # fixed2 deployed and failing
                      dep broken2.sh; run1 RED; iters=(1 2 3); green fixed2.sh ;;      # GREEN on that same artifact
  green_old_fp)       dep broken2.sh; run1 REOPEN; run1 RED; iters=(1 2 3); green fixed.sh ;;  # GREEN on the cycle-1 artifact
esac
