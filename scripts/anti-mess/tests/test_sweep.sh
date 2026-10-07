#!/usr/bin/env bash
# test_sweep.sh - T090: the anti-mess invariant sweep. Real git repositories, real processes, real /proc, real flock; the only stand-ins are a
# unit-level `podman` shell script (container listing/stop) and a fake `cpa-host` that refuses (neither exists in the fixture); the sweep
# itself runs against the REAL podman on the real state (T091). RED before scripts/anti-mess exists, GREEN after. Env SWEEP lets
# mutate_sweep.sh substitute a mutated copy of the whole tree (scripts/anti-mess + scripts/longops + scripts/repo).
. "$(dirname "$0")/../../longops/tests/lib.sh"
SW=${SWEEP:-$TROOT/scripts/anti-mess/sweep.sh}
ident_header T090
echo "# sha256 sweep.sh=$(sha256sum "$SW" 2>/dev/null | cut -c1-64) catalogue.yaml=$(sha256sum "$(dirname "$SW")/catalogue.yaml" 2>/dev/null | cut -c1-64)"
export LC_ALL=C GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=protocol.file.allow GIT_CONFIG_VALUE_0=always
mkrepo() { git init -q "$1" 2>/dev/null; git -C "$1" config user.email t@t; git -C "$1" config user.name t; git -C "$1" config commit.gpgsign false; }
cmt() { git -C "$1" add -A && git -C "$1" -c core.hooksPath=/dev/null commit -qm "${2:-c}" 2>/dev/null; }
swfx() {  # swfx <name>: a clean fixture repository with an ignored .audit, an empty exception list, a fake podman
  newfx "$1"; R=$FXN/root; rm -rf "$R"; mkrepo "$R"; echo 1 >"$R/a.txt"; printf '/.audit/\n' >"$R/.gitignore"; cmt "$R" init; mkdir -p "$R/.audit"
  printf '# path\tkind\tfile\twt\tblob\treason\n' >"$FXN/exc.tsv"
  cat >"$FXN/podman" <<'PE'
#!/usr/bin/env bash
echo "$*" >>"${PODLOG:-/dev/null}"
case "$1" in ps) cat "${PODJSON:-/dev/null}" 2>/dev/null || echo '[]' ;; *) ;; esac
exit 0
PE
  chmod +x "$FXN/podman"; echo '[]' >"$FXN/pods.json"
  export LONGOPS_REPO=$R LONGOPS_DIR=$R/.audit/longops LONGOPS_AUDIT=$R/.audit LONGOPS_PODMAN=$FXN/podman PODJSON=$FXN/pods.json PODLOG=$FXN/podlog
  export ANTIMESS_ROOT=$R ANTIMESS_OWNED_ORGS=fixorg AM_EXC=$FXN/exc.tsv ANTIMESS_LOCK_MIN_AGE=60 ANTIMESS_ORPHAN_AGE_S=300
  J=$FXN/out.json
  mkdir -p "$FXN/ap/scripts/repo"; printf 'resume_ttl=60\n' >"$FXN/ap/scripts/repo/commit_push.conf"; export CPA_APPROVED_DIR=$FXN/ap   # the approved copy of the CPA run the sweep serves
}
sw() { bash "$SW" --json "$J" "$@" >"$FXN/sw.out" 2>"$FXN/sw.err"; SWRC=$?; }
st() { jq -r --arg i "$1" '.invariants[]|select(.id==$i)|.status' "$J" 2>/dev/null; }
cls() { jq -r --arg i "$1" '.invariants[]|select(.id==$i)|.findings[]|select(.severity=="drift")|.class' "$J" 2>/dev/null | sort | tr '\n' ' ' | sed 's/ $//'; }
infocls() { jq -r --arg i "$1" '.invariants[]|select(.id==$i)|.findings[]|select(.severity=="info")|.class' "$J" 2>/dev/null | sort | tr '\n' ' ' | sed 's/ $//'; }
needle() { jq -r --arg i "$1" '.invariants[]|select(.id==$i)|.control_needle' "$J" 2>/dev/null; }
ALL=AM-R1,AM-R2,AM-R5,AM-G2,AM-P1,AM-P2,AM-P3,AM-P4,INV-9
tree_hash() { ( cd "$1" && find . -path ./.git -prune -o -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum | cut -c1-64; git -C "$1" status --porcelain | sha256sum | cut -c1-64 ); }

echo "== clean state, control needles =="
swfx c1; before=$(tree_hash "$R"); sw --only "$ALL"
assert_rc "C1 clean state exits 0" $SWRC 0
for i in AM-R1 AM-R2 AM-R5 AM-G2 AM-P1 AM-P2 AM-P3 AM-P4 INV-9; do assert_eq "C1 $i clean on the clean state" "$(st $i)" clean; assert_eq "C1 $i control needle seen on the same path" "$(needle $i)" seen; done
assert_eq "C1b the sweep wrote nothing into the swept tree (read-only)" "$(tree_hash "$R")" "$before"
assert_eq "C1c report schema" "$(jq -r .schema "$J")" anti-mess-sweep/1
sw; assert_rc "C2 the full cadence sweep exits 0 on the clean fixture" $SWRC 0
for i in AM-R3 AM-R4 AM-G1 AM-S1 INV-6 INV-7 INV-8; do assert_eq "C2 $i is not_evaluated, never clean" "$(st $i)" not_evaluated; done
[ -n "$(jq -r '.invariants[]|select(.id=="AM-R3")|.reason' "$J")" ] && ok "C2b a not_evaluated invariant carries its reason" || bad "C2b"
assert_eq "C2c summary counts" "$(jq -r '"\(.summary.clean) \(.summary.not_evaluated) \(.summary.drift) \(.summary.blind)"' "$J")" "9 7 0 0"
sw --stage S0 --only AM-R2; assert_eq "C3 a cadence-only invariant is skipped_stage at S0" "$(st AM-R2)" skipped_stage
bash "$SW" --stage bogus >/dev/null 2>&1; assert_rc "C4 an unknown stage is a usage refusal (20)" $? 20

echo "== AM-P1 registry vs reality: hung, dead owner, orphan container =="
swfx p1; mkll; P=$LL; mkll; Q=$LL
id=$("$S/register.sh" --purpose go-test:x --owner t --pid "$P" --no-progress-s 5 --log "$FXN/l.log"); printf 'abc' >"$FXN/l.log"
LONGOPS_NOW=$(( $(date +%s) - 100 )) "$S/heartbeat.sh" --op-id "$id" --sample-log
id2=$("$S/register.sh" --purpose go-test:y --owner t --pid "$Q" --no-progress-s 5000)
sw --only AM-P1; assert_rc "P1 a hung op (live owner, flat offset past budget) is drift (10)" $SWRC 10
assert_eq "P1b exactly the hung op is reported" "$(cls AM-P1)" hung_op
jq -r '.invariants[]|select(.id=="AM-P1")|.findings[]|.subject' "$J" | grep -qx "$id" && ok "P1c the finding names the op id" || bad "P1c"
kill "$Q"; wait "$Q" 2>/dev/null; sw --only AM-P1; assert_eq "P2 a dead-owner registry row is reported too" "$(cls AM-P1)" "hung_op registry_row_dead_owner"
"$S/release.sh" --op-id "$id" --state complete >/dev/null; "$S/release.sh" --op-id "$id2" --state failed >/dev/null; sw --only AM-P1; assert_eq "P3 once both ops are terminal the sweep is clean" "$(st AM-P1)" clean
printf '[{"Id":"ghostghostghostghost","Labels":{"op_id":"ghost-op","project":"catalogizer"},"Created":100}]' >"$PODJSON"
sw --only AM-P1; assert_eq "P4 an orphan labelled container (no registry row, older than budget) is reported" "$(cls AM-P1)" orphan_container
grep -q 'ghostghostgh' "$J" && ok "P4b the finding names the container id" || bad "P4b"
printf '[{"Id":"youngyoungyoungyoung","Labels":{"op_id":"fresh","project":"catalogizer"},"Created":%s}]' "$(date +%s)" >"$PODJSON"
sw --only AM-P1; assert_eq "P5 golden-false carrier: a container younger than the budget is informational only" "$(st AM-P1)" clean
assert_eq "P5b it is listed as orphan_container_young" "$(infocls AM-P1)" orphan_container_young
mkll; R3=$LL; id3=$("$S/register.sh" --purpose live:z --owner t --pid "$R3" --no-progress-s 5000); printf '[{"Id":"regregregregregregreg","Labels":{"op_id":"%s","project":"catalogizer"},"Created":100}]' "$id3" >"$PODJSON"
sw --only AM-P1; assert_eq "P6 golden-false carrier: a container whose op has a live registry row is not reported" "$(st AM-P1)" clean
"$S/release.sh" --op-id "$id3" --state complete >/dev/null; sw --only AM-P1; assert_eq "P7 a container of a TERMINAL op is reported" "$(cls AM-P1)" container_of_terminal_op
echo '[]' >"$PODJSON"
export LONGOPS_PODMAN=$FXN/does-not-exist; sw --only AM-P1; assert_eq "P8 F8 an unreadable container runtime makes AM-P1 unread, NEVER clean" "$(st AM-P1)" unread; assert_rc "P8b the sweep exits 11 (a source was not read), not 0" $SWRC 11
jq -r '.invariants[]|select(.id=="AM-P1")|.findings[]|select(.severity=="unread")|.class' "$J" | grep -qx containers_unread && ok "P8c the finding is containers_unread" || bad "P8c"; export LONGOPS_PODMAN=$FXN/podman

echo "== AM-P2 duplicate owner =="
swfx p2; mkll; A=$LL; "$S/register.sh" --purpose dup:k --owner a --pid "$A" >/dev/null
jq -nc '{op_id:"manual-dup",purpose_key:"dup:k",run_id:"manual-dup",pid:0,start_time:"",state:"running",last_progress_epoch:0,progress_offset:0,budget:{no_progress_s:0}}' >"$LONGOPS_DIR/ops/manual-dup.json"
sw --only AM-P2; assert_rc "D1 two non-terminal ops with one purpose_key: drift" $SWRC 10; assert_eq "D1b class" "$(cls AM-P2)" duplicate_owner
jq -c '.attached_to="x"' "$LONGOPS_DIR/ops/manual-dup.json" >"$FXN/t" && mv "$FXN/t" "$LONGOPS_DIR/ops/manual-dup.json"; sw --only AM-P2; assert_eq "D2 golden-false: the second op is attached to the first" "$(st AM-P2)" clean

echo "== the heartbeat check is load-bearing (mutation inside the sweep tests) =="
swfx m1; mkdir -p "$FXN/mt/scripts/anti-mess"; cp "$TROOT/scripts/anti-mess/catalogue.yaml" "$FXN/mt/scripts/anti-mess/"; for d in repo longops; do cp -r "$TROOT/scripts/$d" "$FXN/mt/scripts/$d"; done
python3 - "$FXN/mt/scripts/longops/lib.sh" <<'PY'
import sys; p=sys.argv[1]; s=open(p).read(); s=s.replace('[ "$np" -gt 0 ] && [ $((now - lp)) -gt "$np" ]','false',1); open(p,'w').write(s)
PY
cp "$SW" "$FXN/mt/scripts/anti-mess/sweep.sh"
mkll; P=$LL; id=$("$S/register.sh" --purpose hb:x --owner t --pid "$P" --no-progress-s 5); LONGOPS_NOW=$(( $(date +%s) - 100 )) "$S/heartbeat.sh" --op-id "$id" --progress-offset 1
bash "$FXN/mt/scripts/anti-mess/sweep.sh" --only AM-P1 --json "$FXN/mt.json" >"$FXN/mt.out" 2>&1; rc=$?
[ $rc -eq 20 ] && [ "$(jq -r '.invariants[]|select(.id=="AM-P1")|.status' "$FXN/mt.json")" = blind ] && ok "H1 with the heartbeat check disabled the sweep fails its own control needle (blind, exit 20), never prints clean" || bad "H1 rc=$rc [$(cat "$FXN/mt.out")]"

echo "== AM-R2 stale git lock (dead holder) and its carriers =="
swfx l1; : >"$R/.git/index.lock"; touch -d '10 minutes ago' "$R/.git/index.lock"
sw --only AM-R2; assert_eq "L1 a stale lock (no process has it open, no git running in the repo, old enough) is drift" "$(cls AM-R2)" stale_git_lock
exec 8<"$R/.git/index.lock"; sw --only AM-R2; exec 8<&-; assert_eq "L2 golden-false carrier: the same lock held open by a live process is not stale" "$(st AM-R2)" clean
assert_eq "L2b it is listed as held open" "$(infocls AM-R2)" lock_held_open
touch "$R/.git/index.lock"; sw --only AM-R2; assert_eq "L3 golden-false carrier: a young lock is not stale" "$(st AM-R2)" clean
rm -f "$R/.git/index.lock"; : >"$R/decoy.lock"; touch -d '10 minutes ago' "$R/decoy.lock"; sw --only AM-R2; assert_eq "L4 golden-false carrier: a .lock file outside .git is not a git lock" "$(st AM-R2)" clean
mkdir -p "$R/.git/modules/m"; : >"$R/.git/modules/m/HEAD.lock"; touch -d '10 minutes ago' "$R/.git/modules/m/HEAD.lock"; sw --only AM-R2; assert_eq "L5 a stale lock in a submodule gitdir under .git/modules is found" "$(cls AM-R2)" stale_git_lock

echo "== AM-R1: change set, exceptions, pending pins =="
swfx r1; echo changed >>"$R/a.txt"; mkdir -p "$R/specs/f/evidence"; echo "blob" >"$R/specs/f/evidence/recorder.blob"; echo '{"row":1}' >"$R/specs/f/evidence/deferrals.jsonl"
sw --stage S7 --only AM-R1; assert_eq "R1 at S7 nothing is excluded: the dirty tree is drift" "$(cls AM-R1)" dirty_repository
printf 'a.txt\n' >"$FXN/cs"; sw --stage S0 --paths-from "$FXN/cs" --only AM-R1; assert_eq "R2 S0: an undeclared change and an untracked evidence file of another stream are reported" "$(cls AM-R1)" dirty_outside_change_set
jq -r '.invariants[]|select(.id=="AM-R1")|.findings[0].evidence' "$J" | grep -q 'specs/f/evidence' && ok "R2b the evidence names the undeclared files" || bad "R2b"
printf 'a.txt\nspecs/f/evidence/recorder.blob\tspecs/f/evidence/reviews/v.json\nspecs/f/evidence/deferrals.jsonl\n' >"$FXN/cs"; sw --stage S0 --paths-from "$FXN/cs" --only AM-R1
assert_eq "R3 S0: a tree whose only changes are the declared change set (with a declared recorder blob and a declared row appended to deferrals.jsonl) passes" "$(st AM-R1)" clean
assert_eq "R3b it is recorded as declared_change_set_only" "$(infocls AM-R1)" declared_change_set_only
swfx r2; mkrepo "$FXN/dl"; printf 'line\r\n' >"$FXN/dl/data.txt"; cmt "$FXN/dl"; git -C "$R" submodule add -q "$FXN/dl" third/dl 2>/dev/null; cmt "$R" sub
printf 'line\n' >"$R/third/dl/data.txt"
sw --only AM-R1; assert_eq "R4 a dirty submodule WITHOUT an exceptions.tsv row is reported" "$(cls AM-R1)" dirty_repository
jq -r '.invariants[]|select(.id=="AM-R1")|.findings[]|select(.severity=="drift")|.subject' "$J" | grep -qx 'third/dl' && ok "R4b it names third/dl" || bad "R4b"
wt=$(sha256sum "$R/third/dl/data.txt" | cut -c1-64); bl=$(git -C "$R/third/dl" show HEAD:data.txt | sha256sum | cut -c1-64)
printf 'third/dl\tdirty\tdata.txt\t%s\t%s\tCRLF test data\n' "$wt" "$bl" >>"$FXN/exc.tsv"
sw --only AM-R1; assert_eq "R5 the same repository WITH its reviewed exceptions.tsv row passes (control needle R4)" "$(st AM-R1)" clean
printf '# header\n' >"$FXN/exc.tsv"
git -C "$R/third/dl" checkout -q -- data.txt; echo n >"$R/third/dl/new.txt"; git -C "$R/third/dl" add new.txt; git -C "$R/third/dl" -c core.hooksPath=/dev/null commit -qm adv
printf 'third/dl\n' >"$R/.audit/pending_pins.tsv"; sw --only AM-R1
assert_eq "R6 a pointer drift equal to a row of .audit/pending_pins.tsv is a pending pin move, never dirt" "$(st AM-R1):$(infocls AM-R1)" "clean:pending_pin_move"
rm -f "$R/.audit/pending_pins.tsv"; sw --only AM-R1; assert_eq "R7 the same drift without the row is not dirt either (pin rules own it)" "$(st AM-R1)" clean
echo dirty >>"$R/a.txt"; echo d >>"$R/third/dl/new.txt"; sw --only AM-R1 --repo third/dl; assert_eq "R8 --repo scopes AM-R1 to that repository only" "$(jq -r '.invariants[]|select(.id=="AM-R1")|.findings[]|select(.severity=="drift")|.subject' "$J" | tr '\n' ' ')" "third/dl "

echo "== AM-R5 uninitialised submodule =="
swfx u1; mkrepo "$FXN/sm"; echo 1 >"$FXN/sm/f"; cmt "$FXN/sm"; git -C "$R" submodule add -q "$FXN/sm" vendor/sm 2>/dev/null; cmt "$R" sub; git clone -q "$R" "$FXN/clone" 2>/dev/null
ANTIMESS_ROOT=$FXN/clone sw --only AM-R5; assert_eq "U1 a declared but uninitialised submodule is drift" "$(cls AM-R5)" uninitialised_submodule
sw --only AM-R5; assert_eq "U2 golden-false: the initialised submodule is clean" "$(st AM-R5)" clean

echo "== AM-G2 CI pipelines and core.hooksPath =="
swfx g1; mkdir -p "$R/.github/workflows"; echo 'name: x' >"$R/.github/workflows/ci.yml"; cmt "$R" ci
sw --only AM-G2; assert_eq "G1 a planted workflow file is reported on the cadence (through scripts/repo/check_no_ci.sh)" "$(cls AM-G2)" ci_pipeline_definition
sw --stage S0 --only AM-G2; assert_eq "G1b at S0 only the hooksPath half runs: the workflow file is not reported" "$(st AM-G2)" clean
swfx g2; mkdir -p "$R/blk"; printf '#!/bin/sh\nexit 1\n' >"$R/blk/pre-commit"; chmod +x "$R/blk/pre-commit"; git -C "$R" config core.hooksPath blk
sw --only AM-G2; assert_eq "G2 a blocking core.hooksPath is reported on the cadence" "$(cls AM-G2)" blocking_hooks_path; assert_rc "G2b cadence reports it (10), does not refuse" $SWRC 10
sw --stage S0 --only AM-G2; assert_rc "G3 at S0 a blocking core.hooksPath is REFUSED with 20" $SWRC 20
sw --stage S7 --only AM-G2; assert_rc "G3b at S7 it is reported (10), not refused" $SWRC 10
mkdir -p "$R/empty"; git -C "$R" config core.hooksPath empty; sw --only AM-G2; assert_eq "G4 golden-false: core.hooksPath pointing at a non-blocking (empty) directory is clean" "$(st AM-G2)" clean
git -C "$R" config --unset core.hooksPath; sw --only AM-G2; assert_eq "G5 golden-false: core.hooksPath unset is clean" "$(st AM-G2)" clean

echo "== INV-9 commit-push run directories =="
swfx i1; B=$R/.audit/commit-push; mkdir -p "$B"
mkdir -p "$B/20260101T000000Z-999999-ab12"; sw --only INV-9; assert_eq "I1 a directory without report.json whose process is gone is an interrupted run" "$(cls INV-9)" interrupted_run
sw --only INV-9 --reconcile; [ -d "$B/20260101T000000Z-999999-ab12" ] && ok "I1b an interrupted run is never removed, not even by --reconcile" || bad "I1b removed"
printf 'sleep 600\n' >"$FXN/commit-push-all.sh"; bash "$FXN/commit-push-all.sh" >/dev/null 2>&1 & CP=$!; KILLME+=("$CP")
mkdir -p "$B/20260102T000000Z-$CP-ef56"; sw --only INV-9; assert_eq "I2 a live run (pid from the run id, cmdline contains commit-push-all) is not interrupted" "$(cls INV-9)" interrupted_run
assert_eq "I2b the live one is listed live_run" "$(infocls INV-9)" live_run
rm -rf "${B:?}/20260101T000000Z-999999-ab12" "${B:?}/20260102T000000Z-$CP-ef56"
mkdir -p "$B/mrg1"; echo '{"repo":".","pid":999997,"start_time":"1"}' >"$B/mrg1/merge.json"; git -C "$R" rev-parse HEAD >"$R/.git/MERGE_HEAD"
sw --only INV-9; assert_eq "I3 merge.json, no report.json, MERGE_HEAD, process gone: interrupted merge" "$(cls INV-9)" interrupted_merge
jq -r '.invariants[]|select(.id=="INV-9")|.findings[]|.evidence' "$J" | grep -q 'git merge --abort' && ok "I3b with the T042 S0 remediation" || bad "I3b"
mkll; M=$LL; echo "{\"repo\":\".\",\"pid\":$M,\"start_time\":\"$(pstart "$M")\"}" >"$B/mrg1/merge.json"
sw --only INV-9; assert_eq "I4 golden-false: the merge process lives (paused between merge --no-commit and commit): the live holder, no remediation" "$(st INV-9):$(infocls INV-9)" "clean:live_merge_holder"
jq -r '.invariants[]|select(.id=="INV-9")|.findings[]|.evidence' "$J" | grep -q 'merge --abort' && bad "I4b remediation offered for a live holder" || ok "I4b no remediation offered while the holder lives"
rm -f "$R/.git/MERGE_HEAD"
# retention + remote check
swfx i2; B=$R/.audit/commit-push; mkdir -p "$B" "$R/scripts/repo"; printf 'retain_runs=1\nretain_days=1\n' >"$R/scripts/repo/commit_push.conf"; cmt "$R" conf
git init -q --bare "$FXN/origin.git" 2>/dev/null; git -C "$R" branch -M main; git -C "$R" remote add origin "$FXN/origin.git"; git -C "$R" push -q origin main 2>/dev/null
for r in old1 old2 new1; do mkdir -p "$B/$r"; echo '{"status":"complete"}' >"$B/$r/report.json"; done; touch -d '5 days ago' "$B/old1" "$B/old1/report.json"; touch -d '4 days ago' "$B/old2" "$B/old2/report.json"
echo x >"$R/held.txt"; cmt "$R" held; HELD=$(git -C "$R" rev-parse HEAD); printf '.\t%s\n' "$HELD" >"$B/old1/commits.tsv"
sw --only INV-9 --reconcile
[ -d "$B/old1" ] && ok "I5 a finished directory past both bounds whose held commit no remote holds is KEPT" || bad "I5 removed"
[ ! -d "$B/old2" ] && ok "I5b a finished directory past both bounds with no held commit is removed (auto-safe)" || bad "I5b not removed"
[ -d "$B/new1" ] && ok "I5c a directory inside the newest retain_runs is kept" || bad "I5c"
assert_eq "I5d the kept held record is reported kept_held_commit" "$(infocls INV-9)" kept_held_commit
git -C "$R" push -q origin main 2>/dev/null; sw --only INV-9 --reconcile
[ ! -d "$B/old1" ] && ok "I6 the first sweep after every reachable remote holds the commit removes the directory" || bad "I6"
swfx i3; B=$R/.audit/commit-push; mkdir -p "$B/susp" "$B/ready" "$B/exp"
mkll; A=$LL; export LONGOPS_BUILDS=$FXN/bld; mkdir -p "$FXN/bld/b1"
"$S/acquire.sh" --purpose commit_push --run-id susp --pid "$A" >/dev/null; "$S/acquire.sh" --suspend susp --builds b1 >/dev/null
rm -rf "${B:?}/ready" "${B:?}/exp"; echo '{"status":"awaiting_remote_checks"}' >"$B/susp/report.json"; touch -d '9 days ago' "$B/susp" "$B/susp/report.json"
sw --only INV-9 --reconcile; assert_eq "I7 a run awaiting_remote_checks with a live suspended-run holder is suspended, never interrupted, never reaped" "$(st INV-9):$(infocls INV-9)" "clean:suspended_run"
[ -d "$B/susp" ] && ok "I7b not removed" || bad "I7b"
mkdir -p "$FXN/bld/b1/terminal"; "$S/acquire.sh" --update susp --callback-state done --state ready_to_resume >/dev/null; rm -rf "${B:?}/susp"; mkdir "$B/susp"
sw --only INV-9; assert_eq "I8 a run in state ready_to_resume is its own class, never interrupted, suspended or stale" "$(st INV-9):$(infocls INV-9)" "clean:ready_to_resume"
jq -c '.ready_at=1' "$R/.audit/longops/claims/commit_push/holder.json" >"$FXN/t" && mv "$FXN/t" "$R/.audit/longops/claims/commit_push/holder.json"
sw --only INV-9 --reconcile; assert_eq "I9 an expired ready_to_resume holder is REPORTED" "$(cls INV-9)" ready_to_resume_expired
[ -d "$R/.audit/longops/claims/commit_push" ] && ok "I9b and never released by the sweep (release is acquire.sh --expire)" || bad "I9b released"
unset CPA_APPROVED_DIR; sw --only INV-9; assert_eq "I10 with CPA_APPROVED_DIR unset and no cpa-host the commit_push holder is reported unread, never a sweep failure" "$(infocls INV-9 | tr ' ' '\n' | grep -c commit_push_holder_unread)" 1
mkdir -p "$FXN/bin"; printf '#!/bin/sh\necho project_not_trusted >&2\nexit 20\n' >"$FXN/bin/cpa-host"; chmod +x "$FXN/bin/cpa-host"
PATH=$FXN/bin:$PATH sw --only INV-9; jq -r '.invariants[]|select(.id=="INV-9")|.findings[]|.evidence' "$J" | grep -q project_not_trusted && ok "I11 a project_not_trusted refusal of cpa-host is reported as the purpose unread" || bad "I11"
unset LONGOPS_BUILDS

echo "== AM-P3 leftover build temp dirs, open builds =="
swfx b1; export LONGOPS_BUILDS=$R/.audit/builds; mkdir -p "$LONGOPS_BUILDS/x1" "$LONGOPS_BUILDS/x2/terminal" "$LONGOPS_BUILDS/x3"; mkdir "$LONGOPS_BUILDS/x3/job.tmp-999999-1"; mkdir "$LONGOPS_BUILDS/x3/live.tmp-$$-$(pstart $$)"
sw --only AM-P3; assert_eq "B1 a .tmp-<pid>-<start> directory whose process is gone is drift; open builds with no hub are reported" "$(cls AM-P3)" "build_without_registry_row build_without_registry_row open_builds_without_hub open_builds_without_hub stale_build_tmp"
sw --only AM-P3 --reconcile; [ ! -d "$LONGOPS_BUILDS/x3/job.tmp-999999-1" ] && ok "B2 --reconcile reaps the dead-owner temp dir (auto-safe)" || bad "B2"
[ -d "$LONGOPS_BUILDS/x3/live.tmp-$$-$(pstart $$)" ] && ok "B2b the temp dir of a live process is untouched" || bad "B2b"
[ -d "$LONGOPS_BUILDS/x2/terminal" ] && ok "B2c a terminal build is untouched" || bad "B2c"
mkll; H=$LL; echo "{\"pid\":$H,\"start_time\":\"$(pstart "$H")\"}" >"$LONGOPS_BUILDS/hub.json"; sw --only AM-P3; assert_eq "B3 with a live hub record open_builds_without_hub is no longer reported" "$(cls AM-P3 | tr ' ' '\n' | grep -c open_builds_without_hub)" 0
unset LONGOPS_BUILDS

echo "== reconcile only the catalogued auto-safe classes =="
swfx x1; : >"$R/.git/index.lock"; touch -d '10 minutes ago' "$R/.git/index.lock"; echo dirty >>"$R/a.txt"; mkdir -p "$R/.audit/commit-push/20260101T000000Z-999999-zz"
h0=$(sha256sum "$R/a.txt" | cut -c1-64)
sw --only AM-R1,AM-R2,INV-9 --reconcile; assert_rc "X1 drift remains after a reconcile that could not fix everything (10)" $SWRC 10
[ ! -e "$R/.git/index.lock" ] && ok "X1b the auto-safe class (stale git lock) was removed" || bad "X1b"
assert_eq "X1c the operator-gated dirty file is byte-unchanged" "$(sha256sum "$R/a.txt" | cut -c1-64)" "$h0"
[ -d "$R/.audit/commit-push/20260101T000000Z-999999-zz" ] && ok "X1d the interrupted run was not touched" || bad "X1d"
assert_eq "X1e the report records the reconcile result" "$(jq -r '.invariants[]|select(.id=="AM-R2")|.reconciled[0].result' "$J")" removed
swfx x2; mkll; A=$LL; id=$("$S/register.sh" --purpose dead:op --owner t --pid "$A"); kill "$A"; wait "$A" 2>/dev/null; sw --only AM-P1 --reconcile
assert_eq "X2 a dead-owner registry row is reaped by --reconcile" "$(jq -r .state "$LONGOPS_DIR/ops/$id.json")" reaped
printf '[{"Id":"ghostghostghostghost","Labels":{"op_id":"ghost-op","project":"catalogizer"},"Created":100}]' >"$PODJSON"; sw --only AM-P1 --reconcile
grep -q 'stop -t 5 ghostghostgh' "$PODLOG" && ok "X3 an orphan labelled container is stopped by --reconcile (podman stop)" || bad "X3 [$(cat "$PODLOG" 2>/dev/null)]"
swfx x3; mkrepo "$FXN/sm"; echo 1 >"$FXN/sm/f"; cmt "$FXN/sm"; git -C "$R" submodule add -q "$FXN/sm" vendor/sm 2>/dev/null; cmt "$R" sub; git clone -q "$R" "$FXN/clone" 2>/dev/null
ANTIMESS_ROOT=$FXN/clone sw --only AM-R5 --reconcile; [ -e "$FXN/clone/vendor/sm/f" ] && ok "X4 --reconcile initialises an uninitialised submodule (auto-safe)" || bad "X4"


echo "== WF11 class 2: a corrupt op record, an unreadable source: reported unread, never clean, and never hiding the rest (F4, F8) =="
swfx n1; mkll; P=$LL; mkll; Q=$LL
id=$("$S/register.sh" --purpose n1:hung --owner t --pid "$P" --no-progress-s 5 --log "$FXN/l.log"); printf 'abc' >"$FXN/l.log"; LONGOPS_NOW=$(( $(date +%s) - 100 )) "$S/heartbeat.sh" --op-id "$id" --sample-log
"$S/register.sh" --purpose n1:dup --owner a --pid "$Q" >/dev/null; jq -nc '{op_id:"manual-dup",purpose_key:"n1:dup",run_id:"manual-dup",pid:0,start_time:"",state:"running",last_progress_epoch:0,progress_offset:0,budget:{no_progress_s:0}}' >"$LONGOPS_DIR/ops/manual-dup.json"
sw --only AM-P1,AM-P2; assert_rc "N0 control: without the corrupt record the hung op and the duplicate owner are reported (10)" $SWRC 10
assert_eq "N0b control: AM-P1 hung_op (+ the hand-written pid-0 row), AM-P2 duplicate_owner" "$(cls AM-P1)/$(cls AM-P2)" "hung_op registry_row_dead_owner/duplicate_owner"
printf '{"op_id":"trunc",' >"$LONGOPS_DIR/ops/00corrupt.json"
sw --only AM-P1,AM-P2; assert_rc "N1 F4 one unparsable record sorted first does not blind AM-P1 / AM-P2: still drift (10)" $SWRC 10
assert_eq "N1b the hung op is still reported" "$(cls AM-P1)" "hung_op registry_row_dead_owner"
assert_eq "N1c the duplicate owner is still reported" "$(cls AM-P2)" duplicate_owner
for i in AM-P1 AM-P2; do assert_eq "N1d $i also reports the corrupt record as an unread finding" "$(jq -r --arg i $i '.invariants[]|select(.id==$i)|.findings[]|select(.severity=="unread")|.class' "$J")" corrupt_op_record; done
swfx n2; printf '{"op_id":"trunc",' >"$LONGOPS_DIR/ops/00corrupt.json" 2>/dev/null || { mkdir -p "$LONGOPS_DIR/ops"; printf '{"op_id":"trunc",' >"$LONGOPS_DIR/ops/00corrupt.json"; }; : >"$LONGOPS_DIR/ops/01empty.json"
sw --only AM-P1,AM-P2; assert_rc "N2 only corrupt records (and a 0-byte one): exit 11, never 0" $SWRC 11
assert_eq "N2b AM-P1 and AM-P2 are unread, not clean" "$(st AM-P1)/$(st AM-P2)" unread/unread
assert_eq "N2c both files are named" "$(jq -r '.invariants[]|select(.id=="AM-P1")|.findings[]|select(.severity=="unread")|.subject' "$J" | sed 's#.*/##' | tr '\n' ' ')" "00corrupt.json 01empty.json "
echo "-- the label the launcher really sets (catalogizer.op_id, scripts/containers/run_pinned.sh) --"
swfx n3; mkll; P=$LL; id=$("$S/register.sh" --purpose n3:x --owner t --pid "$P" --no-progress-s 5000)
printf '[{"Id":"labelllabelllabelll","Labels":{"catalogizer.op_id":"%s","project":"catalogizer"},"Created":100}]' "$id" >"$PODJSON"
sw --only AM-P1; assert_eq "N3 F-label a container labelled catalogizer.op_id of a LIVE op is not container_without_op_label (and not reported at all)" "$(st AM-P1)" clean
printf '[{"Id":"nolabelnolabelnolabe","Labels":{"project":"catalogizer"},"Created":100}]' >"$PODJSON"; sw --only AM-P1; assert_eq "N3b a container with no op label at all is still reported" "$(cls AM-P1)" container_without_op_label
printf '[{"Id":"orphanorphanorphano","Labels":{"catalogizer.op_id":"ghost-op","project":"catalogizer"},"Created":100}]' >"$PODJSON"; sw --only AM-P1; assert_eq "N3c catalogizer.op_id of an unknown op older than the budget is an orphan_container" "$(cls AM-P1)" orphan_container
"$S/release.sh" --op-id "$id" --state complete --verdict PASS >/dev/null; printf '[{"Id":"termtermtermtermterm","Labels":{"catalogizer.op_id":"%s","project":"catalogizer"},"Created":100}]' "$id" >"$PODJSON"
sw --only AM-P1 --reconcile; assert_eq "N3d a container (catalogizer.op_id) of a TERMINAL op is reported and stopped by --reconcile" "$(cls AM-P1):$(grep -c 'stop -t 5 termtermterm' "$PODLOG")" "container_of_terminal_op:1"

echo "== WF11 F9 AM-P4: stale claims, holderless claims, unreadable holders and un-adopted handoffs are visible =="
swfx q1; sw --only AM-P4; assert_eq "Q0 control: a clean registry is clean" "$(st AM-P4)" clean
mkll; A=$LL; id=$("$S/register.sh" --purpose q1:stale --owner t --pid "$A"); kill "$A"; wait "$A" 2>/dev/null; rm -f "$LONGOPS_DIR/ops/$id.json"
sw --only AM-P4; assert_eq "Q1 a dead holder with no op record (register now exits 4) is drift stale_claim" "$(cls AM-P4)" stale_claim; assert_rc "Q1b exit 10" $SWRC 10
mkdir "$LONGOPS_DIR/claims/q1:nohold"; sw --only AM-P4; assert_eq "Q2 a YOUNG claim directory with no holder record is informational (a registration in progress)" "$(cls AM-P4):$(infocls AM-P4)" "stale_claim:claim_young"
touch -d '10 minutes ago' "$LONGOPS_DIR/claims/q1:nohold"; sw --only AM-P4; assert_eq "Q2b the same directory older than the minimum age is claim_without_holder" "$(cls AM-P4)" "claim_without_holder stale_claim"
mkdir "$LONGOPS_DIR/claims/q1:bad"; printf '{"kind":"proc' >"$LONGOPS_DIR/claims/q1:bad/holder.json"; sw --only AM-P4; assert_eq "Q3 an unreadable holder record is drift claim_unreadable" "$(cls AM-P4)" "claim_unreadable claim_without_holder stale_claim"
swfx q2; mkll; B=$LL; id=$("$S/register.sh" --purpose q2:live --owner t --pid "$B" --no-progress-s 5000)
sw --only AM-P4; assert_eq "Q4 golden-false: a live holder with its op is clean" "$(st AM-P4)" clean
mkll; C=$LL; idh=$("$S/register.sh" --purpose q2:ho --owner t --pid "$C" --no-progress-s 5000); "$S/release.sh" --op-id "$idh" --state handoff --verdict driver_stop >/dev/null
sw --only AM-P4; assert_eq "Q5 an op in the re-adoptable handoff state that nothing supersedes is drift handoff_unadopted" "$(cls AM-P4)" handoff_unadopted
jq -c '.superseded_by="newop"' "$LONGOPS_DIR/ops/$idh.json" >"$FXN/t" && mv "$FXN/t" "$LONGOPS_DIR/ops/$idh.json"; sw --only AM-P4; assert_eq "Q5b golden-false: a superseded handoff is clean" "$(st AM-P4)" clean
jq -c 'del(.superseded_by)' "$LONGOPS_DIR/ops/$idh.json" >"$FXN/t" && mv "$FXN/t" "$LONGOPS_DIR/ops/$idh.json"; "$S/release.sh" --op-id "$idh" --state complete --verdict adopted >/dev/null; sw --only AM-P4; assert_eq "Q5c golden-false: once the handoff is resolved into a terminal state it is clean" "$(st AM-P4)" clean
swfx q3; rm -f "$FXN/ap/scripts/repo/commit_push.conf"; mkll; D=$LL
"$S/acquire.sh" --purpose commit_push --run-id cpq --pid "$D" >/dev/null; mkdir -p "$FXN/bld/b1"; "$S/acquire.sh" --suspend cpq --builds b1 >/dev/null; "$S/acquire.sh" --update cpq --callback-state done --state ready_to_resume >/dev/null; mkdir -p "$FXN/bld/b1/terminal"
LONGOPS_BUILDS=$FXN/bld sw --only AM-P4; assert_eq "Q6 a holder that cannot be judged (ready_to_resume, no conf) is unread, never clean" "$(st AM-P4)" unread

echo "== WF11 F10 handoff containers are never stopped; stopcontainer re-verifies =="
swfx r1x; mkll; A=$LL; idh=$("$S/register.sh" --purpose r1x:h --owner t --pid "$A" --no-progress-s 5000); "$S/release.sh" --op-id "$idh" --state handoff --verdict driver_stop >/dev/null
printf '[{"Id":"handoffhandoffhand","Labels":{"catalogizer.op_id":"%s","project":"catalogizer"},"Created":100}]' "$idh" >"$PODJSON"
sw --only AM-P1 --reconcile; assert_eq "H1 a container of a HANDOFF op is informational only" "$(cls AM-P1):$(infocls AM-P1)" ":container_of_handoff_op"
assert_eq "H1b and --reconcile did not stop it" "$(grep -c 'stop' "$PODLOG" 2>/dev/null)" 0
swfx r2x; mkdir -p "$LONGOPS_DIR/ops"; printf '[{"Id":"racerackeracerackera","Labels":{"catalogizer.op_id":"ghost-op","project":"catalogizer"},"Created":100}]' >"$PODJSON"
printf '#!/bin/bash\njq -nc --arg id ghost-op --arg p r2x:p '"'"'{op_id:$id,purpose_key:$p,run_id:$id,pid:0,start_time:"",state:"running",last_progress_epoch:0,progress_offset:0,budget:{no_progress_s:0}}'"'"' >"%s/ops/ghost-op.json"\n' "$LONGOPS_DIR" >"$FXN/hook.sh"
ANTIMESS_TEST_MODE=1 ANTIMESS_TEST_BEFORE_ACTION=$FXN/hook.sh sw --only AM-P1 --reconcile
assert_eq "H2 an orphan container whose op got a registry row between detection and action is NOT stopped" "$(grep -c 'stop' "$PODLOG" 2>/dev/null)" 0
assert_eq "H2b the report records why" "$(jq -r '.invariants[]|select(.id=="AM-P1")|.reconciled[]|select(.action|startswith("stopcontainer"))|.result' "$J" | head -1)" skipped_precondition_changed_live
ANTIMESS_TEST_BEFORE_ACTION=/bin/true bash "$SW" --only AM-P1 >"$FXN/o" 2>&1; assert_rc "H3 a test hook outside ANTIMESS_TEST_MODE=1 is refused (20)" $? 20
swfx r3x; : >"$R/.git/index.lock"; touch -d '10 minutes ago' "$R/.git/index.lock"; printf '#!/bin/bash\ntouch "%s/.git/index.lock"\n' "$R" >"$FXN/hook.sh"
ANTIMESS_TEST_MODE=1 ANTIMESS_TEST_BEFORE_ACTION=$FXN/hook.sh sw --only AM-R2 --reconcile
assert_eq "H4 RMS1 a lock that became young (touched) between detection and action is NOT removed" "$([ -e "$R/.git/index.lock" ] && echo kept || echo removed)" kept
assert_eq "H4b the report says skipped_not_provably_stale" "$(jq -r '.invariants[]|select(.id=="AM-R2")|.reconciled[0].result' "$J")" skipped_not_provably_stale
swfx r4x; mkrepo "$FXN/sm"; echo 1 >"$FXN/sm/f"; cmt "$FXN/sm"; git -C "$R" submodule add -q "$FXN/sm" vendor/sm 2>/dev/null; cmt "$R" sub; git clone -q "$R" "$FXN/clone" 2>/dev/null
printf '#!/bin/bash\ngit -C "%s" submodule update --init -- vendor/sm >/dev/null 2>&1\n' "$FXN/clone" >"$FXN/hook.sh"
ANTIMESS_ROOT=$FXN/clone ANTIMESS_TEST_MODE=1 ANTIMESS_TEST_BEFORE_ACTION=$FXN/hook.sh sw --only AM-R5 --reconcile
assert_eq "H5 initsub re-verifies: a submodule initialised between detection and action is skipped_already_initialised" "$(jq -r '.invariants[]|select(.id=="AM-R5")|.reconciled[0].result' "$J")" skipped_already_initialised
swfx r5x; B=$R/.audit/commit-push; mkdir -p "$B" "$R/scripts/repo"; printf 'retain_runs=1\nretain_days=1\n' >"$R/scripts/repo/commit_push.conf"; cmt "$R" conf
git init -q --bare "$FXN/origin.git" 2>/dev/null; git -C "$R" branch -M main; git -C "$R" remote add origin "$FXN/origin.git"; git -C "$R" push -q origin main 2>/dev/null
for r in old2 new1; do mkdir -p "$B/$r"; echo '{"status":"complete"}' >"$B/$r/report.json"; done; touch -d '4 days ago' "$B/old2" "$B/old2/report.json"
echo x >"$R/held.txt"; cmt "$R" held; UNPUSHED=$(git -C "$R" rev-parse HEAD)
printf '#!/bin/bash\nprintf ".\\t%%s\\n" "%s" >"%s/old2/commits.tsv"\n' "$UNPUSHED" "$B" >"$FXN/hook.sh"
ANTIMESS_TEST_MODE=1 ANTIMESS_TEST_BEFORE_ACTION=$FXN/hook.sh sw --only INV-9 --reconcile
[ -d "$B/old2" ] && ok "H6 a finished run that gained an unpushed held commit between detection and action is KEPT (rmdir re-verifies)" || bad "H6 removed"
assert_eq "H6b the report says why" "$(jq -r '.invariants[]|select(.id=="INV-9")|.reconciled[0].result' "$J")" skipped_held_commit_not_on_remote

echo "== WF11 F12: an unknown --only id is a usage refusal =="
swfx o1
for o in NOPE am-p1 "AM-P1,NOPE" "" ","; do bash "$SW" --only "$o" >"$FXN/o" 2>"$FXN/e"; rc=$?; assert_rc "O1 --only '$o' is refused (20), never an empty clean sweep" $rc 20; done
grep -q 'unknown invariant id' "$FXN/e" && ok "O1b it names the unknown id" || bad "O1b [$(cat "$FXN/e")]"
bash "$SW" --only AM-P1,AM-P2 >"$FXN/o" 2>&1; assert_rc "O2 control: valid ids still run (0)" $? 0

echo "== a blind detector never prints clean =="
swfx z1; mkdir -p "$FXN/bt/scripts/anti-mess"; cp "$TROOT/scripts/anti-mess/catalogue.yaml" "$FXN/bt/scripts/anti-mess/"; for d in repo longops; do cp -r "$TROOT/scripts/$d" "$FXN/bt/scripts/$d"; done
python3 - "$SW" "$FXN/bt/scripts/anti-mess/sweep.sh" <<'PY'
import sys; s=open(sys.argv[1]).read()
a='''  done < <(find "$gd" -name '*.lock' -type f -print0 2>/dev/null)'''
assert a in s; s=s.replace(a,'''  done < <(true)''',1); open(sys.argv[2],'w').write(s)
PY
: >"$R/.git/index.lock"; touch -d '10 minutes ago' "$R/.git/index.lock"
bash "$FXN/bt/scripts/anti-mess/sweep.sh" --only AM-R2 --json "$FXN/bt.json" >"$FXN/bt.out" 2>&1; rc=$?
[ $rc -eq 20 ] && [ "$(jq -r '.invariants[0].status' "$FXN/bt.json")" = blind ] && ok "Z1 a detector that cannot see the seeded drift is BLIND: exit 20, not a clean report" || bad "Z1 rc=$rc [$(cat "$FXN/bt.out")]"
bash "$SW" --only AM-R2 --json "$FXN/ok.json" >/dev/null 2>&1; assert_eq "Z2 control: the unmutated sweep sees the same lock" "$(jq -r '.invariants[0].status' "$FXN/ok.json")" drift

finish
