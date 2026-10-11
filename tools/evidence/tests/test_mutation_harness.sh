#!/usr/bin/env bash
# WP-05 fix round 8, finding R6 (independent review WP-05): the mutation harness scratch tree did not hold docs/scripts/evidence-recorder.md, so the
# UNMUTATED test_anchors.sh FAILed 'no-force-push scan found usage' there, and mutants A17, A18 and A18b were recorded CAUGHT on exactly that FAIL line
# (a false match, 11.4.201(6)). Fix: the tree holds the guide, and every run starts with the unmutated-baseline control (run_mutations_wp05b.py baseline(),
# run_mutations.py main()). This test proves (1) the tree is complete: A17, A18, A18b are re-run and are CAUGHT with the baseline green first;
# (2) the control SEES a damaged tree (the guide removed): FAIL lines on the unmutated tree; (3) a harness run on a damaged tree stops with exit 2
# 'BASELINE NOT GREEN' and issues no mutant verdict. Constitution 11.4.224 (test first), 11.4.201(6)-(7), 11.4.194(6)(d).
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd)
H=${HARNESS:-$here/run_mutations_wp05b.py}     # HARNESS: only to run this test against an older harness (the RED run)
fails=0; n=0
ok()  { n=$((n+1)); if [ -f "$H" ]; then echo "ok   $1"; else fails=$((fails+1)); echo "FAIL $1 (vacuous: harness absent)"; fi; }
bad() { n=$((n+1)); fails=$((fails+1)); echo "FAIL $1"; }
S=$(mktemp -d); trap 'rm -rf "$S"' EXIT
python3 -B "$H" -j 1 A17-forced A18-plus A18b-plain >"$S/run.out" 2>&1; r=$?
grep -q '^baseline green: test_anchors.sh' "$S/run.out" && ok "R6: the run starts with the baseline control and test_anchors.sh is green on the UNMUTATED scratch tree" || bad "R6: no green baseline line: $(head -c 400 "$S/run.out")"
c=$(grep -c ' CAUGHT ' "$S/run.out"); { [ "$r" = 0 ] && [ "$c" = 3 ]; } && ok "R6: A17, A18 and A18b are re-run on the complete tree and are CAUGHT (3 of 3, exit 0)" || bad "R6: A17/A18/A18b: exit $r caught=$c: $(head -c 600 "$S/run.out")"
grep ' CAUGHT ' "$S/run.out" | grep -q 'no-force-push scan' && ok "R6: each catch cites the no-force-push scan FAIL line of a suite that is green without the mutation" || bad "R6: catch lines do not cite the scan"
python3 -B - "$H" "$S" <<'P'
import os, sys, importlib.util
spec = importlib.util.spec_from_file_location("rm", sys.argv[1]); rm = importlib.util.module_from_spec(spec); sys.dont_write_bytecode = True; spec.loader.exec_module(rm)
guide = lambda tree: os.remove(os.path.join(tree, "docs/scripts/evidence-recorder.md"))
fails = rm.baseline(["test_anchors.sh"], prep=guide)
open(os.path.join(sys.argv[2], "damaged.txt"), "w").write("\n".join(fails))
orig = rm.build_tree
def damaged(dst):
    orig(dst); guide(dst)
rm.build_tree = damaged
import io, contextlib
buf = io.StringIO()
with contextlib.redirect_stdout(buf):
    rc = rm.main(["-j", "1", "A17-forced"])
open(os.path.join(sys.argv[2], "damaged_run.txt"), "w").write("rc=%s\n%s" % (rc, buf.getvalue()))
P
grep -q 'no-force-push scan' "$S/damaged.txt" && ok "R6 control needle: baseline() on a tree WITHOUT the guide reports the no-force-push FAIL line (the control can see the damage)" || bad "R6: baseline() did not see the missing guide: $(head -c 300 "$S/damaged.txt")"
{ grep -q '^rc=2' "$S/damaged_run.txt" && grep -q 'BASELINE NOT GREEN' "$S/damaged_run.txt" && ! grep -q ' CAUGHT ' "$S/damaged_run.txt"; } && ok "R6: a harness run on the damaged tree stops with exit 2 BASELINE NOT GREEN and issues no CAUGHT verdict" || bad "R6: damaged-tree run: $(head -c 400 "$S/damaged_run.txt")"
python3 -B "$H" --check A17-forced A18-plus A18b-plain >"$S/chk.out" 2>&1; [ "$(grep -c ': *ok\| ok$' "$S/chk.out")" -ge 3 ] && ok "R6: the three edits apply to the current sources (--check)" || bad "R6: --check: $(head -c 300 "$S/chk.out")"
echo "checks=$n failures=$fails"; [ "$fails" -eq 0 ]
