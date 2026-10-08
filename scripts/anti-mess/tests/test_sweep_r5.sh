#!/usr/bin/env bash
# test_sweep_r5.sh - 11.4.276 round 5: the tests of the classes the round-3 review found STILL OPEN in scripts/anti-mess/sweep.sh and in the registry readers it uses. Every test drives the REAL sweep (and the real
# reap.sh / release.sh / register.sh it delegates to) on a fixture; none re-implements the guarded logic. Each test was observed RED against 350372a8 (evidence/wp08/fix-r5-RED-*.txt) and GREEN x3 on the fix tree.
# Env SWEEP substitutes the sweep (the RED run points at the extracted 350372a8 tree and the mutation runner at a mutated copy).
# Classes: A (raw-typed inputs), B (the driver cannot report clean without having evaluated), C (reconcile bound to its invariant, canonical paths, parse-safe stream), E (label -> lineage, writers, adoption),
# G (AM-P4 claim without op), H (identity, canonical root), I (git semantics), J (scale).
. "$(dirname "$0")/../../longops/tests/lib.sh"
. "$(dirname "$0")/sweep_lib.sh"
ident_header WP08-R5-sweep
echo "# sha256 sweep.sh=$(sha256sum "$SW" 2>/dev/null | cut -c1-64) catalogue.yaml=$(sha256sum "$(dirname "$SW")/catalogue.yaml" 2>/dev/null | cut -c1-64)"
sha() { sha256sum "$1" 2>/dev/null | cut -c1-64; }
# want <section>: the mutation runner runs only the sections that can kill a mutant (S5_SECTIONS=B,C ...); an unset S5_SECTIONS runs everything
want() { [ -z "${S5_SECTIONS:-}" ] || case ",$S5_SECTIONS," in *",$1,"*) return 0 ;; *) return 1 ;; esac; }
NINV=$(python3 -I -c 'import yaml,sys; print(len(yaml.safe_load(open(sys.argv[1]))["invariants"]))' "$(dirname "$SW")/catalogue.yaml")
# a test catalogue + test-only detectors (ANTIMESS_TEST_MODE=1 only): swcat <file> <id>:<detector>:<reconcile>:<actions,..>...
swcat() {
  local out=$1; shift; { printf 'schema: anti-mess-catalogue/1\ninvariants:\n'; local spec id det rec act
    for spec in "$@"; do IFS=: read -r id det rec act <<<"$spec"
      printf '  - id: %s\n    plane: runtime\n    desired: "test"\n    detector: %s\n    stages: [cadence]\n    reconcile: %s\n    needle: no\n    actions: [%s]\n' "$id" "$det" "$rec" "$act"; done; } >"$out"
}

if want B; then
echo "== class B: the driver cannot print clean without having evaluated =="
swfx b1; mkll; P=$LL; id=$("$S/register.sh" --purpose b1:x --owner t --pid "$P" --no-progress-s 600); kill "$P"; wait "$P" 2>/dev/null
: >"$R/.git/index.lock"; touch -d '10 minutes ago' "$R/.git/index.lock"
mkdir -p "$FXN/shim"; real_find=$(command -v find)
cat >"$FXN/shim/find" <<EOF
#!/bin/bash
# lists like find, but removes every listed git lock BEFORE the caller can stat it (the ordinary \`git status\` index.lock race)
case " \$* " in *" -name *.lock "*) case "\$1" in "$R"/*) ;; *) exec "$real_find" "\$@" ;; esac; mapfile -d '' -t L < <("$real_find" "\$@"); for f in "\${L[@]}"; do rm -f "\$f"; done; for f in "\${L[@]}"; do printf '%s\\0' "\$f"; done; exit 0 ;; esac
exec "$real_find" "\$@"
EOF
chmod +x "$FXN/shim/find"
PATH=$FXN/shim:$PATH sw
assert_eq "B1 a lock that vanishes between find and stat must not abandon the driver: ONE row per catalogued invariant ($NINV)" "$(jq -r '.invariants|length' "$J" 2>/dev/null)" "$NINV"
assert_eq "B1b the seeded dead-owner drift is still reported (exit 10, never 0)" "$SWRC:$(st AM-P1)" "10:drift"
assert_eq "B1c the vanished lock is skipped, AM-R2 clean (released meanwhile)" "$(st AM-R2)" clean
swfx b2; mkdir -p "$R/scripts/repo" "$R/.audit/commit-push/old1"; printf 'retain_runs=0\nretain_days=0\n' >"$R/scripts/repo/commit_push.conf"; cmt "$R" conf; echo '{"status":"complete"}' >"$R/.audit/commit-push/old1/report.json"; touch -d '5 days ago' "$R/.audit/commit-push/old1" "$R/.audit/commit-push/old1/report.json"
mkdir -p "$FXN/shim"; real_stat=$(command -v stat)
cat >"$FXN/shim/stat" <<EOF
#!/bin/bash
case " \$* " in *"$R"/.audit/commit-push/*report.json*) exit 1 ;; esac
exec "$real_stat" "\$@"
EOF
chmod +x "$FXN/shim/stat"
PATH=$FXN/shim:$PATH sw --only INV-9,AM-P4
assert_eq "B2 a report.json that vanishes before stat (INV-9) preserves completeness: both rows present, no arithmetic abort" "$(jq -r '.invariants|length' "$J" 2>/dev/null):$SWRC" "2:0"
swfx b3; echo '{"x":1}' >/dev/null
cat >"$FXN/dets.sh" <<'EOF'
det_TEST_EXIT3() { exit 3; }
det_TEST_UNBOUND() { echo "$undefined_variable_of_the_test"; }
det_TEST_ARITH() { local x=; echo $(( 5 - x_ )) $(( 5 - )); }
det_TEST_SEES() { emit drift test_seen subj "the detector after the broken ones still ran"; }
EOF
swcat "$FXN/cat.yaml" TEST-EXIT3:det_TEST_EXIT3:operator-gated: TEST-UNBOUND:det_TEST_UNBOUND:operator-gated: TEST-ARITH:det_TEST_ARITH:operator-gated: TEST-SEES:det_TEST_SEES:operator-gated:
ANTIMESS_TEST_MODE=1 ANTIMESS_CATALOGUE=$FXN/cat.yaml ANTIMESS_TEST_DETECTORS=$FXN/dets.sh sw
assert_eq "B3 a detector that exits, one with an unbound variable and one with a broken arithmetic expansion are BLIND (exit 20), never clean" "$SWRC:$(st TEST-EXIT3):$(st TEST-UNBOUND):$(st TEST-ARITH)" "20:blind:blind:blind"
assert_eq "B3b every other invariant still has its row and ran" "$(st TEST-SEES):$(jq -r '.invariants|length' "$J")" "drift:4"
jq -r '.invariants[]|select(.id=="TEST-EXIT3")|.reason' "$J" | grep -q "abnormally" && ok "B3c the reason says the detector ended abnormally" || bad "B3c [$(jq -r '.invariants[]|select(.id=="TEST-EXIT3")|.reason' "$J")]"
swfx b5
cat >"$FXN/dets.sh" <<'EOF'
det_TEST_WIPE() { rm -rf "$W"; }
det_TEST_AFTER() { emit info after x "this row cannot be written"; }
EOF
swcat "$FXN/cat.yaml" TEST-WIPE:det_TEST_WIPE:operator-gated: TEST-AFTER:det_TEST_AFTER:operator-gated:
ANTIMESS_TEST_MODE=1 ANTIMESS_CATALOGUE=$FXN/cat.yaml ANTIMESS_TEST_DETECTORS=$FXN/dets.sh sw
assert_eq "B5 a run that did not produce ONE result row per selected invariant exits 20 driver_incomplete, never 0" "$SWRC:$(grep -c driver_incomplete "$FXN/sw.err")" "20:1"
echo "-- two passes: the final status and the exit code come from the state AFTER the reconcile --"
swfx b4; mkdir -p "$FXN/pod"; export PODDIR=$FXN/pod LONGOPS_PODMAN=$FX/podman-state; : >"$PODDIR/calls"
mkll; Q=$LL; mkop bld1 b4:p complete 999999 0
mkop bld1-a2 b4:p running "$Q" 0; mkholder b4:p bld1-a2 "$Q"; kill "$Q"; wait "$Q" 2>/dev/null; jq -c '.container_label="catalogizer.op_id=dispatch-bld1"|.started_utc="2026-01-01T00:00:05Z"' "$LONGOPS_DIR/ops/bld1-a2.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/bld1-a2.json"
jq -c '.container_label="catalogizer.op_id=dispatch-bld1"' "$LONGOPS_DIR/ops/bld1.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/bld1.json"
printf '[{"Id":"twopasstwopasstwopass","Labels":{"op_id":"bld1","project":"catalogizer"},"Created":100}]' >"$PODDIR/ps.json"
sw --only AM-P1 --reconcile
assert_eq "B4 pass 1 reaps the dead-owner op; pass 2 sees its container as the container of a TERMINAL op and stops it: the container is gone" "$(jq -r 'length' "$PODDIR/ps.json")" 0
assert_eq "B4b the FINAL state is clean and the exit is 0 (it was computed after the reconcile)" "$(st AM-P1):$SWRC" "clean:0"
assert_eq "B4c the report keeps what pass 1 detected" "$(jq -r '.invariants[0].detected.status' "$J")" drift
assert_eq "B4d two reconcile actions ran (reapop, stopcontainer), neither repeated" "$(jq -r '.invariants[0].reconciled|map(.action|split(":")[0])|sort|join(",")' "$J")" "reapop,stopcontainer"
unset PODDIR; export LONGOPS_PODMAN=$FXN/podman

fi
if want C; then
echo "== class C: reconcile bound to its invariant, canonical sinks, parse-safe stream =="
swfx c1; V=$FXN/victim.txt; echo precious >"$V"; touch -d '10 minutes ago' "$V"; mkll; Z=$LL
mkop evil AM-X running 999999 0; jq -c --arg id "$(printf 'x\ty\trmlock:%s\nz' "$V")" '.op_id=$id' "$LONGOPS_DIR/ops/evil.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/evil.json"
sw --only AM-P1 --reconcile
assert_eq "C1 an op_id carrying TAB + LF + an rmlock action forges no row: the file outside any .git is intact" "$([ -e "$V" ] && echo present || echo GONE)" present
assert_eq "C1b the hostile record is reported unread (it fails the record shape), no action ran" "$(st AM-P1):$(jq -r '.invariants[0].reconciled|length' "$J")" "unread:0"
O=$FXN/outside_dir; mkdir -p "$O" "$R/.audit/commit-push"; echo '{"status":"complete"}' >"$O/report.json"; echo keep >"$O/data.txt"
mkop evil2 AM-X running 999999 0; jq -c --arg id "$(printf 'x\ty\trmdir:finished_run:%s/commit-push/../../../outside_dir\nz' "$R/.audit")" '.op_id=$id' "$LONGOPS_DIR/ops/evil2.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/evil2.json"
sw --only AM-P1 --reconcile
assert_eq "C1c a forged rmdir:finished_run with a climbing path removes nothing outside commit-push" "$([ -d "$O" ] && echo present || echo GONE)" present
rm -f "$LONGOPS_DIR/ops/evil.json" "$LONGOPS_DIR/ops/evil2.json"
echo "-- every sink on canonical paths, bound to its invariant (test detectors, ANTIMESS_TEST_MODE=1) --"
swfx c2; V=$FXN/victim.txt; echo precious >"$V"; touch -d '10 minutes ago' "$V"; SYMDIR=$FXN/sym-outside; mkdir -p "$SYMDIR"; echo '{"status":"complete"}' >"$SYMDIR/report.json"
mkdir -p "$R/.audit/commit-push" "$R/.audit/builds/b"; ln -s "$V" "$R/.git/evil.lock"; ln -s "$SYMDIR" "$R/.audit/commit-push/symrun"; ln -s "$SYMDIR" "$R/.audit/builds/b/x.tmp-999999-1"
: >"$R/.git/real.lock"; touch -d '10 minutes ago' "$R/.git/real.lock"; mkdir -p "$R/.audit/commit-push/realrun" "$R/.audit/builds/b/y.tmp-999998-1"; echo '{"status":"complete"}' >"$R/.audit/commit-push/realrun/report.json"
cat >"$FXN/dets.sh" <<EOF
det_T_RMLOCK() { emit drift t1 s1 e1 "rmlock:$V"; emit drift t2 s2 e2 "rmlock:$R/.git/evil.lock"; emit drift t3 s3 e3 "rmlock:$R/.git/real.lock"; }
det_T_RMDIR() { emit drift t4 s4 e4 "rmdir:finished_run:$R/.audit/commit-push/../../../sym-outside"; emit drift t5 s5 e5 "rmdir:finished_run:$R/.audit/commit-push/symrun"; emit drift t6 s6 e6 "rmdir:finished_run:$R/.audit/commit-push/realrun"
  emit drift t7 s7 e7 "rmdir:build_tmp:$R/.audit/builds/b/x.tmp-999999-1"; emit drift t8 s8 e8 "rmdir:build_tmp:$SYMDIR"; emit drift t9 s9 e9 "rmdir:build_tmp:$R/.audit/builds/b/y.tmp-999998-1"; }
det_T_FOREIGN() { emit drift t10 s10 e10 "rmlock:$R/.git/real2.lock"; emit drift t11 s11 e11 "reapop:nosuchop"; }
EOF
: >"$R/.git/real2.lock"; touch -d '10 minutes ago' "$R/.git/real2.lock"
swcat "$FXN/cat.yaml" T-RMLOCK:det_T_RMLOCK:auto-safe:rmlock T-RMDIR:det_T_RMDIR:auto-safe:rmdir:finished_run,rmdir:build_tmp T-FOREIGN:det_T_FOREIGN:auto-safe:reapop
ANTIMESS_TEST_MODE=1 ANTIMESS_CATALOGUE=$FXN/cat.yaml ANTIMESS_TEST_DETECTORS=$FXN/dets.sh sw --reconcile
res() { jq -r --arg i "$1" --arg a "$2" '.invariants[]|select(.id==$i)|.reconciled[]|select(.action==$a)|.result' "$J" 2>/dev/null; }
assert_eq "C2 rmlock of a file that is not a *.lock under the git dir is refused" "$(res T-RMLOCK "rmlock:$V")" refused_not_a_git_lock
assert_eq "C2b rmlock of a SYMLINK named *.lock inside .git (pointing outside) is refused" "$(res T-RMLOCK "rmlock:$R/.git/evil.lock")" refused_not_a_git_lock
assert_eq "C2c golden-false: a real stale index-style lock is removed" "$(res T-RMLOCK "rmlock:$R/.git/real.lock")" removed
assert_eq "C2d the victim and the symlink target are intact" "$([ -e "$V" ] && echo present || echo GONE)" present
assert_eq "C2e rmdir:finished_run with a climbing path is refused" "$(res T-RMDIR "rmdir:finished_run:$R/.audit/commit-push/../../../sym-outside")" refused_outside_commit_push
assert_eq "C2f rmdir:finished_run on a SYMLINK run dir pointing outside is refused" "$(res T-RMDIR "rmdir:finished_run:$R/.audit/commit-push/symrun")" refused_symlink
assert_eq "C2g golden-false: a real finished run directory (canonical parent commit-push) is removed" "$(res T-RMDIR "rmdir:finished_run:$R/.audit/commit-push/realrun")" removed
assert_eq "C2h the outside directory with its report.json is intact" "$([ -d "$SYMDIR" ] && echo present || echo GONE)" present
assert_eq "C2i rmdir:build_tmp on a symlink named *.tmp-<pid>-<start> is refused" "$(res T-RMDIR "rmdir:build_tmp:$R/.audit/builds/b/x.tmp-999999-1")" refused_symlink
assert_eq "C2j rmdir:build_tmp outside the builds dir is refused" "$(res T-RMDIR "rmdir:build_tmp:$SYMDIR")" refused_outside_builds
assert_eq "C2k golden-false: a real dead-owner build temp dir is removed" "$(res T-RMDIR "rmdir:build_tmp:$R/.audit/builds/b/y.tmp-999998-1")" removed
assert_eq "C3 an rmlock action emitted by an invariant whose catalogue entry does not list rmlock is refused (not run)" "$(res T-FOREIGN "rmlock:$R/.git/real2.lock")" refused_action_not_of_invariant
[ -e "$R/.git/real2.lock" ] && ok "C3b the lock of the foreign row is intact" || bad "C3b it was removed"
assert_eq "C3c an action of its own invariant still runs (reapop of an unknown op is 'refused' by reap.sh, not by the binding)" "$(res T-FOREIGN "reapop:nosuchop")" refused
swfx c6; : >"$R/.git/$(printf 'a\ndrift\tforged\tsubj\tev\treapop:nosuchop\nz.lock')"; touch -d '10 minutes ago' "$R/.git/"a*z.lock
sw --only AM-R2 --reconcile
assert_eq "C6 a git lock whose NAME holds LF and TAB forges no row (the stream is escaped): no finding of the forged class, the odd name is reported unread" "$(jq -r '.invariants[0].findings[].class' "$J" | sort | tr '\n' ' '):$(jq -r '.invariants[0].reconciled|length' "$J")" "git_lock_name :0"
echo "-- LO-C2: the action re-verifies the class that was DETECTED, not 'reap.sh would reap' --"
swfx c4; mkll; P=$LL; mkll; Z=$LL; export LONGOPS_NOW=2000; mkop dead4 c4:x running 999999 1000; mkholder c4:x dead4 999999
cat >"$FXN/hook.sh" <<EOF
#!/bin/bash
# between detection and action the dead-owner row is rebound to a LIVE process whose progress is stale (hung): a reap.sh that only asks 'would I reap?' would send it TERM
st=\$(sed 's/^.*) //' /proc/$P/stat | cut -d' ' -f20); cmd=\$(tr '\\0' ' ' </proc/$P/cmdline | sed 's/ \$//')
jq -c --argjson p $P --arg st "\$st" --arg c "\$cmd" '.pid=\$p|.start_time=\$st|.cmdline=\$c' "$LONGOPS_DIR/ops/dead4.json" >"$FXN/hk.json" && cp "$FXN/hk.json" "$LONGOPS_DIR/ops/dead4.json"
EOF
ANTIMESS_TEST_MODE=1 ANTIMESS_TEST_BEFORE_ACTION=$FXN/hook.sh sw --only AM-P1 --reconcile
assert_eq "C4 a dead-owner row rebound to a live pid before the action is NOT signalled: the owner lives" "$(kill -0 "$P" 2>/dev/null && echo alive || echo DEAD)" alive
assert_eq "C4b no signal was ever recorded" "$([ -s "$LONGOPS_DIR/signals.log" ] && echo signalled || echo none)" none
assert_eq "C4c the report says the action was refused" "$(jq -r '.invariants[0].reconciled[0].result' "$J")" refused
assert_eq "C4d the record was not reaped" "$(jq -r .state "$LONGOPS_DIR/ops/dead4.json")" running
unset LONGOPS_NOW
swfx c5; mkll; P=$LL; export LONGOPS_NOW=5000; mkop hung5 c5:x running "$P" 100; mkholder c5:x hung5 "$P"
jq -c --arg c "$(tr '\0' ' ' </proc/$P/cmdline | sed 's/ $//')" '.cmdline=$c' "$LONGOPS_DIR/ops/hung5.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/hung5.json"
sw --only AM-P1 --reconcile
assert_eq "C5 --reconcile does NOT reap a HUNG op (live owner): reported with NO action, the owner lives, nothing signalled, the op not reaped" "$(cls AM-P1):$(kill -0 "$P" 2>/dev/null && echo alive):$(jq -r .state "$LONGOPS_DIR/ops/hung5.json"):$([ -s "$LONGOPS_DIR/signals.log" ] && echo sig || echo none):$(jq -r '.invariants[0].reconciled|length' "$J")" "hung_op:alive:running:none:0"
unset LONGOPS_NOW

fi
if want A; then
echo "== class A on the sweep consumers: a bad field is unread, never clean, never reaped =="
SMUT=('del(.pid)' '.pid=null' '.pid=1' '.start_time=""' '.last_progress_epoch="08"' '.budget.no_progress_s="x"' '.state="bogus"' 'del(.run_id)' '.container_label=7' '.attached_to=0' '.superseded_by=[]' 'del(.boot_id)')
si=0; for m in "${SMUT[@]}"; do si=$((si+1)); swfx "sa$si"; mkop badop "sa:$si" running 999999 0; jq -c "$m" "$LONGOPS_DIR/ops/badop.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/badop.json"; h0=$(sha "$LONGOPS_DIR/ops/badop.json")
  sw --only AM-P1,AM-P2,AM-P4 --reconcile
  assert_eq "A1s.$si [$m]: AM-P1, AM-P2 and AM-P4 are all unread (exit 11), none clean" "$(st AM-P1) $(st AM-P2) $SWRC" "unread unread 11"
  assert_eq "A1s.$si [$m]: the record was NOT reaped by --reconcile (byte-identical)" "$(sha "$LONGOPS_DIR/ops/badop.json")" "$h0"
done
swfx a3; mkll; P=$LL; export LONGOPS_NOW=5000; mkop live1 a3:dup running "$P" 5000; mkop live2 a3:dup running "$P" 5000
for att in 'ok:.attached_to="live1"' 'ghost:.attached_to="ghost"' 'zero:.attached_to=0' 'empty:.superseded_by=[]' 'sup:.superseded_by="live1"'; do nm=${att%%:*}; fl=${att#*:}
  cp "$LONGOPS_DIR/ops/live2.json" "$FXN/live2.good"; jq -c "$fl" "$FXN/live2.good" >"$LONGOPS_DIR/ops/live2.json"; sw --only AM-P2
  case "$nm" in ok|sup) assert_eq "A3 attached_to/superseded_by naming a LIVE op of the same purpose is excused (clean)" "$(st AM-P2)" clean ;;
    ghost) assert_eq "A3b attached_to naming no op hides nothing: duplicate_owner" "$(cls AM-P2)" duplicate_owner ;;
    zero|empty) assert_eq "A3c a mistyped attached_to/superseded_by ($nm) is unread, not a hidden duplicate" "$(st AM-P2)" unread ;; esac
  cp "$FXN/live2.good" "$LONGOPS_DIR/ops/live2.json"
done
unset LONGOPS_NOW
echo "-- INV-9: commits.tsv that cannot be read as <repo> TAB <sha> rows is never 'no held commit' (LO-A10) --"
swfx a6; B=$R/.audit/commit-push; mkdir -p "$B" "$R/scripts/repo"; printf 'retain_runs=0\nretain_days=0\n' >"$R/scripts/repo/commit_push.conf"; cmt "$R" conf
git init -q --bare "$FXN/origin.git" 2>/dev/null; git -C "$R" branch -M main; git -C "$R" remote add origin "$FXN/origin.git"; git -C "$R" push -q origin main 2>/dev/null; HEADSHA=$(git -C "$R" rev-parse HEAD)
for r in sp rd ok; do mkdir -p "$B/$r"; echo '{"status":"complete"}' >"$B/$r/report.json"; touch -d '5 days ago' "$B/$r" "$B/$r/report.json"; done
printf '.  %s\n' "$HEADSHA" >"$B/sp/commits.tsv"; printf '.\t%s\n' "$HEADSHA" >"$B/rd/commits.tsv"; printf '.\t%s\n' "$HEADSHA" >"$B/ok/commits.tsv"
if [ "$(id -u)" = 0 ]; then ok "A6 skipped: running as root, mode 000 is readable (UNCONFIRMED here, 11.4.3)"; else chmod 000 "$B/rd/commits.tsv"
  sw --only INV-9 --reconcile
  [ -d "$B/sp" ] && ok "A6 a SPACE-delimited commits.tsv (no sha column) keeps the run directory" || bad "A6 the run with an unparsable commits.tsv was removed"
  [ -d "$B/rd" ] && ok "A6b an UNREADABLE commits.tsv keeps the run directory" || bad "A6b the run with an unreadable commits.tsv was removed"
  [ ! -d "$B/ok" ] && ok "A6c golden-false: a TAB-delimited file whose sha is on the remote tip is removable (removed)" || bad "A6c the proven run was kept"
  chmod 644 "$B/rd/commits.tsv"
fi
echo "-- AM-P4: a claim removed while it is being judged is not a drift of age 'now' (LO-A11) --"
swfx a7; mkdir -p "$LONGOPS_DIR/claims/vanish"; touch -d '10 minutes ago' "$LONGOPS_DIR/claims/vanish"
mkdir -p "$FXN/shim"; real_stat=$(command -v stat)
cat >"$FXN/shim/stat" <<EOF
#!/bin/bash
# the claim directory is released between the holder read and the stat
case " \$* " in *claims/vanish*) rmdir "$LONGOPS_DIR/claims/vanish" 2>/dev/null ;; esac
exec "$real_stat" "\$@"
EOF
chmod +x "$FXN/shim/stat"
PATH=$FXN/shim:$PATH sw --only AM-P4
assert_eq "A7 a claim directory released during the read yields no claim_without_holder drift" "$(st AM-P4):$(cls AM-P4)" "clean:"

fi
if want E; then
echo "== class E: label -> lineage (the REAL reg_adopt records), the writers of every op, adoption =="
BDS=$FXN/bd; AD() { :; }
H64=$(printf '%064d' 1); A64=$(printf '%064d' 2)
awk '/^reg_adopt\(\)/{p=1} /^reg_beat\(\)/{p=0} p' "$TROOT/scripts/build/dispatch.sh" >"$FX/reg_adopt.fn"
grep -q 'register.sh' "$FX/reg_adopt.fn" && ok "E0 control needle: the extraction holds the real reg_adopt" || bad "E0 the extraction is empty or blind"
drv() { LO=$S bash -c 'pump_log() { :; }; . "$1"; reg_adopt "$2"; echo "$OPID" >"$3"; exec sleep 600' _ "$FX/reg_adopt.fn" "$BDS" "$1" >/dev/null 2>&1 &
  KILLME+=("$!"); local i; for i in $(seq 1 80); do [ -s "$1" ] && break; sleep 0.1; done; }
for restarts in 1 2 10; do for final in live complete handoff; do
  swfx "e1-$restarts-$final"; BDS=$FXN/bld1; mkdir -p "$BDS/tmp"; jq -nc --arg p "build:app:lane:$H64:$A64:primary" '{purpose:$p,no_progress_budget_s:600,wallclock_cap_s:3600}' >"$BDS/submit.json"
  export LONGOPS_PODMAN=$FXN/podman; : >"$PODLOG"
  k=0; while [ "$k" -le "$restarts" ]; do rm -f "$FXN/op.$k"; drv "$FXN/op.$k"; last=$(cat "$FXN/op.$k" 2>/dev/null)
    if [ "$k" -lt "$restarts" ]; then bash "$S/release.sh" --op-id "$last" --state handoff --verdict driver_stop >/dev/null 2>&1; fi; k=$((k+1)); done
  case "$final" in complete) bash "$S/release.sh" --op-id "$last" --state complete --verdict PASS >/dev/null 2>&1 ;; handoff) bash "$S/release.sh" --op-id "$last" --state handoff --verdict driver_stop >/dev/null 2>&1 ;; esac
  printf '[{"Id":"lineageaaaaaaaaaaaa","Labels":{"catalogizer.op_id":"dispatch-bld1","project":"catalogizer"},"Created":100}]' >"$PODJSON"
  sw --only AM-P1 --reconcile; stops=$(grep -c '^stop' "$PODLOG" 2>/dev/null)
  case "$final" in
    live) assert_eq "E1 [$restarts restart(s), final attempt LIVE] the container of the running build is not reported and not stopped" "$(cls AM-P1)|$stops" "|0" ;;
    complete) assert_eq "E1 [$restarts restart(s), final attempt COMPLETE] the container of the finished build is reported and stopped once" "$(cls AM-P1)|$stops" "container_of_terminal_op|1" ;;
    handoff) assert_eq "E1 [$restarts restart(s), final attempt HANDOFF] the container is informational, never stopped" "$(cls AM-P1)|$(infocls AM-P1)|$stops" "|container_of_handoff_op|0" ;;
  esac
done; done
swfx y1; mkll; P=$LL; export LONGOPS_NOW=5000; mkop yop y1:p running "$P" 5000 '.container_label="op_id=ylabel"'
printf '[{"Id":"ylabylabylabylab","Labels":{"op_id":"ylabel","project":"catalogizer"},"Created":100}]' >"$PODJSON"
sw --only AM-P1; assert_eq "Y1 a record container_label written as op_id=<value> matches the container labelled <value>: clean" "$(st AM-P1)" clean
mkop ybare y1:q running "$P" 5000 '.container_label="ybarelabel"'; printf '[{"Id":"ybareybareybareyba","Labels":{"catalogizer.op_id":"ybarelabel","project":"catalogizer"},"Created":100}]' >"$PODJSON"
sw --only AM-P1; assert_eq "C10 a BARE container_label equal to the container's label value matches: clean" "$(st AM-P1)" clean
unset LONGOPS_NOW
swfx t9; printf '#!/bin/sh\nsleep 600\n' >"$FXN/podman-wedged"; chmod +x "$FXN/podman-wedged"; t0=$(date +%s)
LONGOPS_PODMAN=$FXN/podman-wedged LONGOPS_PODMAN_TIMEOUT_S=2 timeout 40 bash "$SW" --only AM-P1 --json "$J" >"$FXN/sw.out" 2>"$FXN/sw.err"; rc=$?; t1=$(date +%s)
assert_eq "C09 a wedged container runtime is bounded by LONGOPS_PODMAN_TIMEOUT_S: the sweep returns 11 (containers_unread), never hangs" "$rc:$(st AM-P1)" "11:unread"
[ $((t1-t0)) -lt 30 ] && ok "C09b it returned in $((t1-t0)) s" || bad "C09b it took $((t1-t0)) s"
echo "-- LO-E3: a container that carries BOTH label keys with different values (the real test-infra client) --"
swfx e3; mkll; P=$LL; export LONGOPS_NOW=5000; mkop stackop e3:stack running "$P" 5000
printf '[{"Id":"clientclientclient","Labels":{"catalogizer.op_id":"e3-client-123-4567","op_id":"stackop","project":"catalogizer"},"Created":100}]' >"$PODJSON"
sw --only AM-P1; assert_eq "E3 the container whose op_id label names a LIVE op is clean (400 s old, the other label names an unregistered op)" "$(st AM-P1)" clean
printf '[{"Id":"clientclientclient","Labels":{"catalogizer.op_id":"e3-client-123-4567","op_id":"nobody","project":"catalogizer"},"Created":100}]' >"$PODJSON"
sw --only AM-P1; assert_eq "E3b control: when BOTH labels name no op the container IS an orphan" "$(cls AM-P1)" orphan_container
unset LONGOPS_NOW
echo "-- LO-E4: a handoff is adopted by its next attempt, not by any later op of the purpose --"
swfx e4; mkll; P=$LL
mkop h1 e4:a handoff "$P" 1; mkop h1-a2 e4:a complete "$P" 1; jq -c '.started_utc="2026-01-01T00:00:00Z"' "$LONGOPS_DIR/ops/h1-a2.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/h1-a2.json"
mkop h2 e4:b handoff "$P" 1; mkop later2 e4:b complete "$P" 1; jq -c '.started_utc="2026-02-01T00:00:00Z"' "$LONGOPS_DIR/ops/later2.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/later2.json"
mkop h3 e4:c handoff "$P" 1; mkop old3 e4:c complete "$P" 1; jq -c '.started_utc="2025-12-01T00:00:00Z"' "$LONGOPS_DIR/ops/old3.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/old3.json"
sw --only AM-P4; assert_eq "E4 a SAME-SECOND successor (-a2, started_utc equal) adopts; an unrelated later op and an earlier op do not" "$(cls AM-P4)" "handoff_unadopted handoff_unadopted"
jq -r '.invariants[]|select(.id=="AM-P4")|.findings[]|select(.severity=="drift")|.subject' "$J" | sort | tr '\n' ' ' | grep -qx "h2 h3 " && ok "E4b the two un-adopted handoffs are h2 and h3" || bad "E4b [$(jq -r '.invariants[]|select(.id=="AM-P4")|.findings[]|.subject' "$J" | tr '\n' ' ')]"
mkop h4 e4:d handoff "$P" 1; jq -c '.started_utc=""' "$LONGOPS_DIR/ops/h4.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/h4.json"; sw --only AM-P4
assert_eq "E4c a handoff with started_utc '' is unread, not 'adopted by an earlier op'" "$(jq -r '.invariants[]|select(.id=="AM-P4")|.findings[]|select(.severity=="unread")|.class' "$J" | sort -u | tr '\n' ' ')" "corrupt_op_record "
echo "-- AM-P4: a live claim that no op accounts for, and the claim of a terminal op --"
swfx g4; mkll; P=$LL; mkholder g4:orphan nobody "$P"; mkop t4 g4:term complete "$P" 1; mkholder g4:term t4 "$P"; mkholder commit_push cpa-run "$P"
sw --only AM-P4; assert_eq "G4 claim_without_op and claim_of_terminal_op are drift; the acquire.sh purpose commit_push (no op record) is not" "$(cls AM-P4)" "claim_of_terminal_op claim_without_op"

fi
if want HI; then
echo "== class A (merge.json): the INV-9 merge record is judged by its fields, never by a default =="
swfx a9; B=$R/.audit/commit-push; mkdir -p "$B/mrgA"; git -C "$R" rev-parse HEAD >"$R/.git/MERGE_HEAD"
for spec in '{"repo":"."}' '{"repo":".","pid":"abc","start_time":"1"}' '{"repo":".","pid":0,"start_time":"1"}' '{"repo":".","pid":012,"start_time":"1"}' '{"repo":".","pid":999997}' 'not json'; do
  printf '%s\n' "$spec" >"$B/mrgA/merge.json"; sw --only INV-9
  assert_eq "A9 a merge.json with absent/mistyped/non-canonical pid or start_time [$spec] is UNREAD, never 'the merge process is gone' (no interrupted_merge)" "$(st INV-9):$(grep -c interrupted_merge <<<"$(jq -r '.invariants[0].findings[].class' "$J")")" "unread:0"
done
echo '{"repo":".","pid":999997,"start_time":"1"}' >"$B/mrgA/merge.json"; sw --only INV-9
assert_eq "A9b control: a well-formed record of a gone process is still interrupted_merge" "$(cls INV-9)" interrupted_merge
rm -f "$R/.git/MERGE_HEAD"
echo "== class H / I: process identity under a symlinked root; git semantics =="
swfx h2; mkfifo "$FXN/ff"; ( cd "$R" && exec git cat-file --batch <"$FXN/ff" >/dev/null 2>&1 ) & GP=$!; KILLME+=("$GP"); exec 9>"$FXN/ff"
for _ in $(seq 1 30); do grep -qx git "/proc/$GP/comm" 2>/dev/null && break; sleep 0.1; done
: >"$R/.git/index.lock"; touch -d '10 minutes ago' "$R/.git/index.lock"; ln -s "$R" "$FXN/rootlink"
ANTIMESS_ROOT=$FXN/rootlink sw --only AM-R2 --reconcile
assert_eq "H2 a live git process whose cwd is the PHYSICAL root is seen when the swept root is a SYMLINK: the lock is not stale" "$(cls AM-R2):$(infocls AM-R2)" ":lock_with_git_activity"
[ -e "$R/.git/index.lock" ] && ok "H2b --reconcile left the lock a live git holds" || bad "H2b the lock was removed"
exec 9>&-; kill "$GP" 2>/dev/null; wait "$GP" 2>/dev/null
swfx i1; mkdir -p "$R/cs"; echo p >"$R/protected.txt"; cmt "$R" p; git -C "$R" mv protected.txt cs/new.txt; printf 'cs/new.txt\n' >"$FXN/cs"
sw --stage S0 --paths-from "$FXN/cs" --only AM-R1; assert_eq "I1 a staged rename whose NEW path is declared but whose SOURCE is not is drift (the source is deleted)" "$(cls AM-R1)" dirty_outside_change_set
printf 'cs/new.txt\nprotected.txt\n' >"$FXN/cs"; sw --stage S0 --paths-from "$FXN/cs" --only AM-R1; assert_eq "I1b golden-false: both paths declared" "$(st AM-R1)" clean
swfx i2; mkdir -p "$FXN/home/hk"; printf '#!/bin/sh\nexit 1\n' >"$FXN/home/hk/pre-commit"; chmod +x "$FXN/home/hk/pre-commit"; git -C "$R" config core.hooksPath '~/hk'
HOME=$FXN/home sw --only AM-G2; assert_eq "I2 core.hooksPath=~/hk holding a blocking pre-commit (git expands the tilde and runs it) is drift" "$(cls AM-G2)" blocking_hooks_path
HOME=$FXN/home git -C "$R" commit --allow-empty -qm x >/dev/null 2>&1; assert_rc "I2b control: that hook really blocks a commit" $(( $? != 0 )) 1
git -C "$R" config core.hooksPath rt; mkdir -p "$R/rt"; printf '#!/bin/sh\nexit 1\n' >"$R/rt/reference-transaction"; chmod +x "$R/rt/reference-transaction"
sw --only AM-G2; assert_eq "I2c a blocking reference-transaction hook (aborts every ref update) is drift" "$(cls AM-G2)" blocking_hooks_path
rm -f "$R/rt/reference-transaction"; printf '#!/bin/sh\nexit 1\n' >"$R/rt/post-commit"; chmod +x "$R/rt/post-commit"; sw --only AM-G2; assert_eq "I2d golden-false: only a post-commit hook (never aborts) is clean" "$(st AM-G2)" clean

fi
if want J; then
echo "== class J: the all-time registry (1017 records) must not make the pre-start gate take minutes =="
swfx j1; mkdir -p "$LONGOPS_DIR/ops"
python3 -I - "$LONGOPS_DIR/ops" <<'PY'
import json, sys
d = sys.argv[1]
for i in range(1017):
    r = {"op_id": "old-%04d" % i, "purpose_key": "build:t:l:%064d:%064d:primary" % (i, i), "owner": "x", "run_id": "old-%04d" % i, "container_label": "catalogizer.op_id=dispatch-old-%04d" % i, "write_paths": [], "state": ["complete", "failed", "reaped"][i % 3],
         "started_utc": "2026-01-01T00:00:00Z", "verdict": "x", "evidence_path": ""}
    open("%s/old-%04d.json" % (d, i), "w").write(json.dumps(r))
PY
mkll; P=$LL; "$S/register.sh" --purpose j1:a --owner t --pid "$P" --no-progress-s 600 >/dev/null
printf '[{"Id":"jjjjjjjjjjjjjjjjjjjj","Labels":{"catalogizer.op_id":"dispatch-old-0001","project":"catalogizer"},"Created":100},{"Id":"kkkkkkkkkkkkkkkkkkkk","Labels":{"catalogizer.op_id":"dispatch-old-0002","project":"catalogizer"},"Created":100},{"Id":"llllllllllllllllllll","Labels":{"catalogizer.op_id":"dispatch-old-0003","project":"catalogizer"},"Created":100}]' >"$PODJSON"
mkdir -p "$FXN/shim"; for tool in jq python3; do real=$(command -v "$tool"); printf '#!/bin/sh\necho x >>"%s/spawn.count"\nexec %s "$@"\n' "$FXN" "$real" >"$FXN/shim/$tool"; chmod +x "$FXN/shim/$tool"; done
: >"$FXN/spawn.count"; t0=$(date +%s.%N); PATH=$FXN/shim:$PATH timeout 120 bash "$SW" --only AM-P1,AM-P2,AM-P3 --json "$J" >/dev/null 2>&1; rc=$?; t1=$(date +%s.%N); n=$(wc -l <"$FXN/spawn.count")
[ "$rc" -eq 10 ] && [ "$(jq -r '.invariants|length' "$J" 2>/dev/null)" = 3 ] && ok "J1 the runner pre-start gate (AM-P1,AM-P2,AM-P3) finishes over 1017 terminal records with its three rows (exit $rc: the three containers of terminal ops are reported)" || bad "J1 rc=$rc"
[ "$(echo "$t1 - $t0 < 8" | bc)" = 1 ] && ok "J1b it took $(printf '%.1f' "$(echo "$t1 - $t0" | bc)") s" || bad "J1b it took $(echo "$t1 - $t0" | bc) s"
[ "$n" -le 120 ] && ok "J1c it spawned $n jq/python3 processes (bound: 120; O(1 + live + containers), not O(records))" || bad "J1c it spawned $n jq/python3 processes"

fi
if want N; then
echo "== round 5 survivors of the first mutation run: each field, each guard, tested ALONE (a record that is wrong in exactly one way) =="
# S39 / NM-A12: the record shape is judged field by field. A record valid in every OTHER respect but missing op_id (or purpose_key) must be unread; the old BF2/BF3 records lacked pid and more, so dropping one predicate survived.
swfx n1; mkll; P=$LL
for del in op_id purpose_key run_id container_label; do
  rm -f "$LONGOPS_DIR"/ops/*.json; mkop n1ok n1:p running "$P" 1000; jq -c --arg d "$del" 'del(.[$d])' "$LONGOPS_DIR/ops/n1ok.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/n1ok.json"
  sw --only AM-P1; assert_eq "N1 a running record valid in every respect but the missing field '$del' is unread, never judged" "$(st AM-P1):$(infocls AM-P1)" "unread:"
done
rm -f "$LONGOPS_DIR"/ops/*.json; mkop n1t n1:q complete "$P" 1000; jq -c 'del(.op_id)' "$LONGOPS_DIR/ops/n1t.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/n1t.json"
sw --only AM-P2; assert_eq "N1b a TERMINAL record with only op_id missing is unread too (the shape applies to every state)" "$(st AM-P2)" unread
# S40: an op that carries the successor id but started EARLIER than the handoff is not its adopter
swfx n2; mkll; P=$LL
mkop h5 n2:a handoff "$P" 1; jq -c '.started_utc="2026-03-01T00:00:00Z"' "$LONGOPS_DIR/ops/h5.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/h5.json"
mkop h5-a2 n2:a complete "$P" 1; jq -c '.started_utc="2026-01-01T00:00:00Z"' "$LONGOPS_DIR/ops/h5-a2.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/h5-a2.json"
sw --only AM-P4; assert_eq "N2 a successor-id op that started EARLIER than the handoff is not its adopter: handoff_unadopted" "$(cls AM-P4)" handoff_unadopted
# NM-A11: with an unreadable record present, a live holder whose run names no READABLE op is not asserted to be 'a claim nothing accounts for' (the op may be the unreadable record)
swfx n3; mkll; P=$LL; mkholder n3:p ghostrun "$P"; mkdir -p "$LONGOPS_DIR/ops"
sw --only AM-P4; assert_eq "N3 control: a live holder of a non-acquire purpose whose run names no op is claim_without_op" "$(cls AM-P4)" claim_without_op
printf 'not json' >"$LONGOPS_DIR/ops/junk.json"; sw --only AM-P4
assert_eq "N3b with an UNREADABLE op record present the same claim is not asserted orphaned (the op may be the unreadable record): unread, no claim_without_op" "$(st AM-P4):$(cls AM-P4)" "unread:"
# NM-E1: one label that belongs to TWO lineages resolves to neither: unreadable, never 'terminal' (its container is never stopped)
swfx n4; mkll; P=$LL
mkop la n4:a complete "$P" 1; jq -c '.container_label="shared"' "$LONGOPS_DIR/ops/la.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/la.json"
mkop lb n4:b complete "$P" 1; jq -c '.container_label="shared"' "$LONGOPS_DIR/ops/lb.json" >"$FXN/m.json" && cp "$FXN/m.json" "$LONGOPS_DIR/ops/lb.json"
printf '[{"Id":"sharedsharedshare","Labels":{"catalogizer.op_id":"shared","project":"catalogizer"},"Created":100}]' >"$PODJSON"
sw --only AM-P1 --reconcile; assert_eq "N4 a container whose label belongs to two lineages is unread and is NOT stopped by --reconcile" "$(st AM-P1):$(jq -r '.invariants[0].reconciled|length' "$J")" "unread:0"
# NM-C3 / NM-I1: the action-time guards, tested with forged actions of test detectors (the emit side is not the only gate)
swfx n5; mkdir -p "$R/.audit/commit-push"
cat >"$FXN/dets.sh" <<'DETS'
det_T_BADID() { emit drift t1 s1 e1 "stopcontainer:--all:terminal"; }
det_T_BADSUB() { emit drift t2 s2 e2 "initsub:../outside"; emit drift t3 s3 e3 "initsub:-x"; }
DETS
swcat "$FXN/cat.yaml" T-BADID:det_T_BADID:auto-safe:stopcontainer T-BADSUB:det_T_BADSUB:auto-safe:initsub
ANTIMESS_TEST_MODE=1 ANTIMESS_CATALOGUE=$FXN/cat.yaml ANTIMESS_TEST_DETECTORS=$FXN/dets.sh sw --reconcile
resn() { jq -r --arg i "$1" --arg a "$2" '.invariants[]|select(.id==$i)|.reconciled[]|select(.action==$a)|.result' "$J" 2>/dev/null; }
assert_eq "N5 stopcontainer with an unsafe container id (--all) is refused before any podman call" "$(resn T-BADID 'stopcontainer:--all:terminal'):$(cat "$PODLOG" 2>/dev/null | grep -c stop)" "refused_unsafe_container_id:0"
assert_eq "N5b initsub with a .. component is refused" "$(resn T-BADSUB 'initsub:../outside')" refused_unsafe_path
assert_eq "N5c initsub with a leading dash is refused" "$(resn T-BADSUB 'initsub:-x')" refused_unsafe_path
# NM-C4: a backslash in a name round-trips: a stale git lock whose name contains a LITERAL backslash-t is removed as itself (without escaping the backslash the unescape turns it into a TAB and the action misses the file)
swfx n6; : >"$R/.git/x\\ty.lock"; touch -d '10 minutes ago' "$R/.git/x\\ty.lock"
sw --only AM-R2 --reconcile
assert_eq "N6 a stale lock named with a literal backslash-t is removed as itself (the stream escapes the backslash)" "$([ -e "$R/.git/x\\ty.lock" ] && echo present || echo removed):$(jq -r '.invariants[0].reconciled[0].result' "$J")" "removed:removed"

fi
finish
