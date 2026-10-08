#!/usr/bin/env bash
# test_registry.sh - T088: the long-op registry (register, acquire, release, heartbeat, holder, classify, reap,
# check_no_build_writing_tracked, require_verdicts). Real processes, real flock, real /proc. RED before scripts/longops exists
# (run with LONGOPS_SCRIPTS=<empty dir>), GREEN after; env LONGOPS_SCRIPTS lets the mutation runner substitute a mutated copy.
. "$(dirname "$0")/lib.sh"
ident_header T088
export LC_ALL=C
JT() { jq -r "$1" <<<"$2"; }

echo "== register / claim =="
newfx r1; mkll; A=$LL
id=$("$S/register.sh" --purpose go-test:catalog-api --owner tester --pid "$A" --write-path .audit/out/x --no-progress-s 30 --log "$FXN/x.log"); rc=$?
assert_rc "R1 register exits 0 and prints an op id" $rc 0
rec=$LONGOPS_DIR/ops/$id.json
assert_eq "R1b record state is registered before any start" "$(jq -r .state "$rec" 2>/dev/null)" registered
assert_eq "R1c record carries the declared write paths" "$(jq -c .write_paths "$rec" 2>/dev/null)" '[".audit/out/x"]'
assert_eq "R1d record carries the owner's /proc start time" "$(jq -r .start_time "$rec" 2>/dev/null)" "$(pstart "$A")"
mkll; B=$LL
"$S/register.sh" --purpose go-test:catalog-api --owner other --pid "$B" >"$FXN/o" 2>"$FXN/e"; rc=$?
assert_rc "R2 a duplicate purpose_key is refused (atomic mkdir claim, live holder)" $rc 3
grep -q purpose_conflict "$FXN/e" && ok "R2b refusal names purpose_conflict and the holder" || bad "R2b [$(cat "$FXN/e")]"
n=$(ls "$LONGOPS_DIR/ops" | grep -c json); assert_eq "R2c the refused register left exactly ONE more record, FAILED with the refusal as its verdict (11.4.276 G1: the record is written first, a lost claim fails it)" "$n:$(for f in "$LONGOPS_DIR"/ops/*.json; do jq -r '.state+"/"+.verdict' "$f"; done | sort | tr '\n' ' ')" "2:failed/claim_refused:3 registered/ "
newfx r3; mkll; D=$LL
"$S/register.sh" --purpose p3 --owner t --pid "$D" >/dev/null; kill "$D"; wait "$D" 2>/dev/null
"$S/register.sh" --purpose p3 --owner t --pid "$$" >/dev/null 2>"$FXN/e"; rc=$?
assert_rc "R3 a stale claim (dead holder) is refused, never taken over silently" $rc 4
grep -q stale_claim "$FXN/e" && ok "R3b refusal names stale_claim" || bad "R3b"
"$S/reap.sh" --purpose p3 >/dev/null 2>&1; rc=$?; assert_rc "R4 reap.sh --purpose releases the stale claim" $rc 0
"$S/register.sh" --purpose p3 --owner t --pid "$$" >/dev/null 2>&1; assert_rc "R4b the purpose can be registered after the reap" $? 0
newfx r5; mkll; "$S/register.sh" --purpose p5 --owner t --pid "$LL" >/dev/null
[ -z "$(find "$LONGOPS_DIR" -name '.tmp.*')" ] && ok "R5 no temp file is left behind (temp-then-rename)" || bad "R5 leftovers"
if [ "$(stat -f -c %T /dev/shm 2>/dev/null)" = tmpfs ]; then
  out=$(LONGOPS_ALLOW_TMPFS=0 LONGOPS_DIR=/dev/shm/longops-test-$$ "$S/register.sh" --purpose p5 --owner t --pid "$$" 2>&1); rc=$?
  assert_rc "R5b state on tmpfs is refused (not tmpfs, 11.4.232 A)" $rc 20; rm -rf /dev/shm/longops-test-$$
else ok "R5b skipped: /dev/shm is not tmpfs on this host (UNCONFIRMED)"; fi
out=$("$S/register.sh" --purpose '../evil' --owner t 2>&1); assert_rc "R6 a purpose with a slash is refused" $? 2

echo "== heartbeat / hung =="
newfx h1; mkll; H=$LL; export LONGOPS_NOW=1000
id=$("$S/register.sh" --purpose build:x --owner t --pid "$H" --no-progress-s 60 --log "$FXN/l.log")
printf 'aaaa' >"$FXN/l.log"; "$S/heartbeat.sh" --op-id "$id" --sample-log; rc=$?
assert_rc "H1 heartbeat --sample-log exits 0" $rc 0
assert_eq "H1b first heartbeat moves registered -> running" "$(jq -r .state "$LONGOPS_DIR/ops/$id.json")" running
assert_eq "H1c offset sampled from the log size" "$(jq -r .progress_offset "$LONGOPS_DIR/ops/$id.json")" 4
export LONGOPS_NOW=1050; assert_eq "H2 advancing within budget" "$("$S/classify.sh" --op-id "$id" | cut -f2)" advancing
export LONGOPS_NOW=1100; "$S/heartbeat.sh" --op-id "$id" --sample-log   # log unchanged: flat offset
assert_eq "H3 a heartbeat on a flat offset is not progress" "$(jq -r .last_progress_epoch "$LONGOPS_DIR/ops/$id.json")" 1000
export LONGOPS_NOW=1101; r=$("$S/classify.sh" --op-id "$id"); assert_eq "H4 flat progress offset past no_progress_s is HUNG" "$(cut -f2 <<<"$r")" hung
printf 'aaaabbbb' >"$FXN/l.log"; "$S/heartbeat.sh" --op-id "$id" --sample-log; assert_eq "H5 a grown offset is advancing again" "$("$S/classify.sh" --op-id "$id" | cut -f2)" advancing
export LONGOPS_NOW=1300; "$S/heartbeat.sh" --op-id "$id"; assert_eq "H6 an op-written heartbeat (no offset) counts as progress" "$("$S/classify.sh" --op-id "$id" | cut -f2)" advancing
assert_eq "H7 heartbeat_seq is monotone" "$(jq -r .heartbeat_seq "$LONGOPS_DIR/ops/$id.json")" 4
"$S/release.sh" --op-id "$id" --state complete --verdict PASS; assert_rc "H8 release exits 0" $? 0
"$S/heartbeat.sh" --op-id "$id" 2>/dev/null; assert_rc "H9 a terminal op takes no heartbeat" $? 4
assert_eq "H10 release removed the claim" "$([ -d "$LONGOPS_DIR/claims/build:x" ] && echo present || echo gone)" gone
# readers never see a torn record
newfx h2; mkll; id=$("$S/register.sh" --purpose torn --owner t --pid "$LL"); : >"$FXN/stop"; torn=0
( while [ -e "$FXN/stop" ]; do jq -e . "$LONGOPS_DIR/ops/$id.json" >/dev/null 2>&1 || echo T >>"$FXN/torn"; done ) & RD=$!
for i in $(seq 1 60); do "$S/heartbeat.sh" --op-id "$id" >/dev/null; done; rm -f "$FXN/stop"; wait "$RD" 2>/dev/null
assert_eq "H11 a reader looping over 60 record rewrites never reads a torn file" "$(cat "$FXN/torn" 2>/dev/null | wc -l)" 0

echo "== reap =="
unset LONGOPS_NOW
newfx p1; mkll; P=$LL; export LONGOPS_NOW=2000
id=$("$S/register.sh" --purpose p1 --owner t --pid "$P" --no-progress-s 10); "$S/heartbeat.sh" --op-id "$id" --progress-offset 5
export LONGOPS_NOW=2005; "$S/reap.sh" --op-id "$id" >/dev/null 2>"$FXN/e"; rc=$?
assert_rc "P1 a live advancing op is never reaped" $rc 5
kill -0 "$P" 2>/dev/null && ok "P1b the advancing process is still alive" || bad "P1b died"
export LONGOPS_NOW=2100; out=$("$S/reap.sh" --op-id "$id" --dry-run); assert_rc "P2 --dry-run on a hung op exits 0 and signals nothing" $? 0
kill -0 "$P" 2>/dev/null && ok "P2b dry-run left the process alive" || bad "P2b"
out=$("$S/reap.sh" --op-id "$id" 2>&1); rc=$?; assert_rc "P3 a hung op is reaped" $rc 0
sleep 0.5; kill -0 "$P" 2>/dev/null && bad "P3b process still alive" || ok "P3b the hung process was terminated"
assert_eq "P3c record state is reaped" "$(jq -r .state "$LONGOPS_DIR/ops/$id.json")" reaped
grep -q "TERM pid=$P" "$LONGOPS_DIR/signals.log" && ok "P3d signal audit trail names the single pid" || bad "P3d"
assert_eq "P3e claim released after the reap" "$([ -d "$LONGOPS_DIR/claims/p1" ] && echo present || echo gone)" gone
# identity mismatch
newfx p2; mkll; P=$LL; export LONGOPS_NOW=3000
id=$("$S/register.sh" --purpose p2 --owner t --pid "$P" --no-progress-s 10)
jq -c '.cmdline="/bin/some-other-program"' "$LONGOPS_DIR/ops/$id.json" >"$FXN/t" && mv "$FXN/t" "$LONGOPS_DIR/ops/$id.json"
export LONGOPS_NOW=3100; "$S/reap.sh" --op-id "$id" >/dev/null 2>"$FXN/e"; rc=$?
assert_rc "P4 an unresolvable identity (cmdline differs) is refused" $rc 6
kill -0 "$P" 2>/dev/null && ok "P4b the process with a different identity is untouched" || bad "P4b"
# dead owner: no signal at all; recycled pid
newfx p3; mkll; P=$LL; id=$("$S/register.sh" --purpose p3 --owner t --pid "$P"); jq -c '.start_time="1"' "$LONGOPS_DIR/ops/$id.json" >"$FXN/t" && mv "$FXN/t" "$LONGOPS_DIR/ops/$id.json"
jq -c '.start_time="1"' "$LONGOPS_DIR/claims/p3/holder.json" >"$FXN/t" && mv "$FXN/t" "$LONGOPS_DIR/claims/p3/holder.json"   # the recycled pid is recycled for the op AND for its claim holder (LO-F2: a LIVE own-run holder refuses the reap)
"$S/reap.sh" --op-id "$id" >/dev/null 2>&1; assert_rc "P5 a recycled pid (start time differs) is dead_owner: reaped" $? 0
kill -0 "$P" 2>/dev/null && ok "P5b the recycled-pid process is NOT signalled" || bad "P5b it was killed"
[ ! -s "$LONGOPS_DIR/signals.log" ] && ok "P5c no signal was recorded" || bad "P5c"
# pgid/pid <= 1
( . "$S/lib.sh"; LD=$FXN/sig; mkdir -p "$LD"; rcs=""; for t in 1 0 -1 abc ""; do lo_signal TERM "$t" >/dev/null 2>&1; rcs="$rcs$? "; done; echo "$rcs" ) >"$FXN/o"
assert_eq "P6 lo_signal refuses pid 1, 0, -1, junk and empty (rc 7 each)" "$(cat "$FXN/o")" "7 7 7 7 7 "
[ ! -e "$FXN/sig/signals.log" ] && ok "P6b no signal audit entry exists for any refused target" || bad "P6b"
newfx p4; mkll; id=$("$S/register.sh" --purpose p4 --owner t --pid "$LL" --no-progress-s 1); jq -c '.pid=1|.start_time="4"|.cmdline="init"' "$LONGOPS_DIR/ops/$id.json" >"$FXN/t" && mv "$FXN/t" "$LONGOPS_DIR/ops/$id.json"; export LONGOPS_NOW=9999
r=$("$S/classify.sh" --op-id "$id" | cut -f2); assert_eq "P7 a record naming pid 1 (hand-written: register refuses it, W16) is UNREADABLE (the record shape wants a pid > 1), never live and never dead_owner" "$r" unreadable
"$S/reap.sh" --op-id "$id" >/dev/null 2>&1; assert_rc "P7b reaping it is refused (20) and sends nothing" $? 20
[ ! -s "$LONGOPS_DIR/signals.log" ] && ok "P7c no signal sent to pid 1" || bad "P7c"
# container label resolution (a unit-level stand-in for podman; the sweep runs against the real one)
newfx p5; fpod; mkll; P=$LL; export LONGOPS_NOW=4000
id=$("$S/register.sh" --purpose p5 --owner t --pid "$P" --no-progress-s 5); fcont cid1234 "catalogizer.op_id=$id"; export LONGOPS_NOW=4100
"$S/reap.sh" --op-id "$id" >/dev/null 2>&1; assert_rc "P8 hung op with a labelled container is reaped" $? 0
grep -q "ps --filter label=op_id=$id" "$PODDIR/calls" && grep -q "ps --filter label=catalogizer.op_id=$id" "$PODDIR/calls" && ok "P8b the container is resolved by label op_id=<id> AND catalogizer.op_id=<id> (the one run_pinned.sh sets)" || bad "P8b [$(cat "$PODDIR/calls" 2>/dev/null)]"
grep -q 'stop -t 5 cid1234' "$PODDIR/calls" && ok "P8c the resolved container is stopped" || bad "P8c"
assert_eq "P8d it is gone from the runtime's list before the op is recorded reaped" "$(ls "$PODDIR" | grep -c '^c-'):$(jq -r .state "$LONGOPS_DIR/ops/$id.json")" "0:reaped"
export LONGOPS_PODMAN=$FXN/podman-null
bare=$(grep -h -E '^[^#]*\b(pgrep|pkill|killall)\b' "$S"/*.sh | wc -l); assert_eq "P9 no script of the registry uses pgrep, pkill or killall" "$bare" 0
unset LONGOPS_NOW

echo "== holder, suspend, adopt, expire =="
newfx l1; mkll; A=$LL
"$S/acquire.sh" --purpose lockp --run-id run-1 --pid "$A"; assert_rc "L1 acquire exits 0" $? 0
j=$("$S/holder.sh" lockp)
assert_eq "L1b holder prints the run id" "$(JT .run_id "$j")" run-1
assert_eq "L1c holder prints the pid" "$(JT .pid "$j")" "$A"
assert_eq "L1d holder prints the process start time from /proc/<pid>/stat" "$(JT .start_time "$j")" "$(pstart "$A")"
assert_eq "L1e status live" "$(JT .status "$j")" live
assert_eq "L2 no lock gives none" "$("$S/holder.sh" nolock)" none
kill "$A"; wait "$A" 2>/dev/null; assert_eq "L3 a dead holder gives none" "$("$S/holder.sh" lockp)" none
mkll; A=$LL; "$S/acquire.sh" --purpose lock2 --run-id r2 --pid "$A" >/dev/null; jq -c '.start_time="1"' "$LONGOPS_DIR/claims/lock2/holder.json" >"$FXN/t" && mv "$FXN/t" "$LONGOPS_DIR/claims/lock2/holder.json"
assert_eq "L4 a pid that now belongs to another process (start time differs) gives none" "$("$S/holder.sh" lock2)" none
"$S/acquire.sh" --purpose lock2 --run-id r9 --pid "$$" >/dev/null 2>&1; assert_rc "L4b acquiring over a dead holder is a stale_claim refusal" $? 4

echo "-- suspended-run --"
newfx s1; mkll; A=$LL; export LONGOPS_BUILDS=$FXN/builds; mkdir -p "$LONGOPS_BUILDS/b1" "$LONGOPS_BUILDS/b2"
"$S/acquire.sh" --purpose susp --run-id sr1 --pid "$A" >/dev/null
"$S/acquire.sh" --suspend sr1 --purpose susp --builds b1,b2 --resume-ttl 100; assert_rc "S1 --suspend swaps process -> suspended-run" $? 0
j=$("$S/holder.sh" susp); assert_eq "S1b a suspended-run holder with non-terminal builds is live" "$(JT .status "$j")" live
assert_eq "S1c kind is suspended-run and builds are listed" "$(JT '.kind+":"+(.builds|join(","))' "$j")" suspended-run:b1,b2
kill "$A"; wait "$A" 2>/dev/null
assert_eq "S1d the dead CPA process does not make the suspended run stale" "$(JT .status "$("$S/holder.sh" susp)")" live
mkdir "$LONGOPS_BUILDS/b1/terminal"; assert_eq "S2 live while ANY member build is non-terminal" "$(JT .status "$("$S/holder.sh" susp)")" live
mkdir "$LONGOPS_BUILDS/b2/terminal"; assert_eq "S3 all builds terminal, callback none, state suspended: none" "$("$S/holder.sh" susp)" none
"$S/acquire.sh" --update sr1 --purpose susp --callback-state running; assert_eq "S4 callback running keeps the holder live" "$(JT .status "$("$S/holder.sh" susp)")" live
export LONGOPS_NOW=5000
"$S/acquire.sh" --update sr1 --purpose susp --callback-state done --state ready_to_resume; assert_rc "S5 callback done + ready_to_resume in one swap" $? 0
assert_eq "S5b ready_to_resume within resume_ttl is live" "$(JT .status "$("$S/holder.sh" susp)")" live
export LONGOPS_NOW=5099; assert_eq "S5c one second before the ttl: live" "$(JT .status "$("$S/holder.sh" susp)")" live
export LONGOPS_NOW=5100; assert_eq "S6 ready_to_resume past resume_ttl is reported expired by the read-only holder" "$(JT .status "$("$S/holder.sh" susp)")" expired
[ -d "$LONGOPS_DIR/claims/susp" ] && ok "S6b holder.sh released nothing" || bad "S6b"
"$S/acquire.sh" --expire susp --op-id op1; assert_rc "S7 acquire --expire releases the expired holder" $? 0
assert_eq "S7b the claim is gone and the record names the expired run" "$([ -d "$LONGOPS_DIR/claims/susp" ] && echo present || echo gone):$(jq -r .expired_run "$LONGOPS_AUDIT/out/op1/resume_expired.json")" gone:sr1
"$S/acquire.sh" --expire susp --op-id op2 2>/dev/null; assert_rc "S7c expire with no holder is a cas_mismatch" $? 4
"$S/acquire.sh" --purpose susp --run-id sr2 --pid "$$" >/dev/null; "$S/acquire.sh" --suspend sr2 --purpose susp --builds b1 --resume-ttl 100 >/dev/null
"$S/acquire.sh" --update sr2 --purpose susp --callback-state done --state ready_to_resume >/dev/null
"$S/acquire.sh" --expire susp --op-id op3 2>/dev/null; assert_rc "S8 --expire refuses a holder still inside resume_ttl" $? 4
unset LONGOPS_NOW

echo "-- a reader never sees none across the swaps --"
newfx s2; mkll; A=$LL; export LONGOPS_BUILDS=$FXN/builds; mkdir -p "$LONGOPS_BUILDS/b1"
"$S/acquire.sh" --purpose swp --run-id w1 --pid "$A" >/dev/null; : >"$FXN/stop"
( while [ -e "$FXN/stop" ]; do "$S/holder.sh" swp >>"$FXN/reads"; done ) & RD=$!
for i in 1 2 3 4 5 6 7 8; do
  rm -rf "$LONGOPS_BUILDS/b1/terminal"
  "$S/acquire.sh" --suspend w1 --purpose swp --builds b1 --resume-ttl 100 >/dev/null || bad "S9 suspend $i"
  "$S/acquire.sh" --update w1 --purpose swp --callback-state running >/dev/null || bad "S9 callback $i"
  mkdir -p "$LONGOPS_BUILDS/b1/terminal"
  "$S/acquire.sh" --update w1 --purpose swp --callback-state done --state ready_to_resume >/dev/null || bad "S9 update $i"
  "$S/acquire.sh" --adopt w1 --purpose swp --pid "$A" >/dev/null || bad "S9 adopt $i"
done
rm -f "$FXN/stop"; wait "$RD" 2>/dev/null
nr=$(wc -l <"$FXN/reads"); nn=$(grep -c '^none$' "$FXN/reads")
[ "$nr" -gt 20 ] && ok "S9 reader made $nr reads while 8 suspend/update/adopt rounds ran" || bad "S9 reader made only $nr reads"
assert_eq "S9b the reader never read none across suspend, callback done and adopt" "$nn" 0

echo "-- a reader whose snapshot went stale retries (deterministic) --"
newfx s4; mkll; A=$LL; export LONGOPS_BUILDS=$FXN/builds; mkdir -p "$LONGOPS_BUILDS/b1"
"$S/acquire.sh" --purpose stl --run-id st1 --pid "$A" >/dev/null; "$S/acquire.sh" --suspend st1 --purpose stl --builds b1 --resume-ttl 100 >/dev/null
LONGOPS_TEST_SLEEP_AFTER_READ=0.6 "$S/holder.sh" stl >"$FXN/stale.out" & RDP=$!
sleep 0.2; "$S/acquire.sh" --update st1 --purpose stl --callback-state running >/dev/null; mkdir -p "$LONGOPS_BUILDS/b1/terminal"
wait "$RDP"; assert_eq "S13 a reader that read the holder before the hub moved it (callback running, build terminal) judges the CURRENT record: live, never none" "$(jq -r .status "$FXN/stale.out" 2>/dev/null)" live
unset LONGOPS_BUILDS

echo "-- adopt racing expire: exactly one winner --"
wins_bad=0
for t in 1 2 3 4 5 6; do
  newfx race$t; mkll; A=$LL; export LONGOPS_NOW=7000 LONGOPS_TEST_SLEEP_IN_CS=0.4 LONGOPS_BUILDS=$FXN/builds; mkdir -p "$LONGOPS_BUILDS/b1/terminal"
  "$S/acquire.sh" --purpose rc --run-id q1 --pid "$A" >/dev/null; "$S/acquire.sh" --suspend q1 --purpose rc --builds b1 --resume-ttl 10 >/dev/null
  "$S/acquire.sh" --update q1 --purpose rc --callback-state done --state ready_to_resume >/dev/null
  export LONGOPS_NOW=7100
  LONGOPS_TEST_SLEEP_IN_CS=0.3 "$S/acquire.sh" --adopt q1 --purpose rc --pid "$A" >/dev/null 2>&1 & W1=$!
  LONGOPS_TEST_SLEEP_IN_CS=0.7 "$S/acquire.sh" --expire rc --op-id race >/dev/null 2>&1 & W2=$!
  wait "$W1"; r1=$?; wait "$W2"; r2=$?
  [ $(( (r1 == 0) + (r2 == 0) )) -eq 1 ] || { wins_bad=$((wins_bad+1)); echo "  trial $t: adopt rc=$r1 expire rc=$r2"; }
done
assert_eq "S10 over 6 trials adopt vs expire has exactly one winner each time" "$wins_bad" 0
unset LONGOPS_NOW LONGOPS_TEST_SLEEP_IN_CS

newfx s3; mkll; "$S/acquire.sh" --adopt nobody --purpose none1 --pid "$LL" 2>/dev/null; assert_rc "S11 adopt with no holder is a cas_mismatch" $? 4
[ ! -d "$LONGOPS_DIR/claims/none1" ] && ok "S11b a failed adopt creates no claim" || bad "S11b"
"$S/acquire.sh" --purpose own1 --run-id o1 --pid "$LL" >/dev/null; "$S/acquire.sh" --adopt o1 --purpose own1 --pid "$LL" 2>/dev/null; assert_rc "S12 adopt of a run that is a live process holder (not suspended) is a cas_mismatch" $? 4
unset LONGOPS_NOW LONGOPS_TEST_SLEEP_IN_CS

echo "-- CENTRAL C2: commit_push needs CPA_APPROVED_DIR --"
newfx c2; mkll; A=$LL; unset CPA_APPROVED_DIR CPA_RUN CPA_RUN_ID
"$S/holder.sh" commit_push >"$FXN/o" 2>"$FXN/e"; rc=$?; assert_rc "C1 holder.sh commit_push with CPA_APPROVED_DIR unset gives 20" $rc 20
grep -q helper_not_approved "$FXN/e" && ok "C1b the refusal names helper_not_approved" || bad "C1b"
"$S/acquire.sh" --expire commit_push >/dev/null 2>"$FXN/e"; rc=$?; assert_rc "C2 --expire unset gives 20 helper_not_approved FIRST (before the --op-id usage check)" $rc 20
grep -q helper_not_approved "$FXN/e" && ok "C2b it is helper_not_approved, not usage_error" || bad "C2b [$(cat "$FXN/e")]"
[ -z "$(find "$LONGOPS_DIR" "$LONGOPS_AUDIT/out" "$LONGOPS_AUDIT/commit-push" -type f ! -name '*.lock' 2>/dev/null)" ] && ok "C2c nothing was written" || bad "C2c wrote files"
mkdir -p "$FXN/approved/scripts/repo"; export CPA_APPROVED_DIR=$FXN/approved
"$S/acquire.sh" --purpose commit_push --run-id cp1 --pid "$A" >/dev/null
j=$("$S/holder.sh" commit_push); assert_eq "C3 with it set, a live process holder reads without any conf (ttl is read lazily)" "$(JT .run_id "$j")" cp1
"$S/acquire.sh" --expire commit_push >/dev/null 2>"$FXN/e"; rc=$?; assert_rc "C4 --expire outside a run without --op-id gives 20 usage_error" $rc 20
grep -q usage_error "$FXN/e" && ok "C4b it names usage_error" || bad "C4b [$(cat "$FXN/e")]"
[ -z "$(find "$LONGOPS_AUDIT/out" -type f 2>/dev/null)" ] && ok "C4c nothing written" || bad "C4c"
mkdir -p "$LONGOPS_AUDIT/builds/b9/terminal"; "$S/acquire.sh" --suspend cp1 --builds b9 >/dev/null; "$S/acquire.sh" --update cp1 --callback-state done --state ready_to_resume >/dev/null
"$S/holder.sh" commit_push >/dev/null 2>"$FXN/e"; rc=$?; assert_rc "C5 a ready_to_resume holder with no commit_push.conf is refused (the lazy read fails loudly)" $rc 20
printf 'resume_ttl=60\n' >"$FXN/approved/scripts/repo/commit_push.conf"
LONGOPS_NOW=$(( $(date +%s) + 30 )) "$S/holder.sh" commit_push | jq -e '.status=="live"' >/dev/null && ok "C6 resume_ttl from the approved conf: inside the window is live" || bad "C6"
export LONGOPS_NOW=$(( $(date +%s) + 61 )); assert_eq "C6b past the conf's ttl: expired" "$(JT .status "$("$S/holder.sh" commit_push)")" expired
mkdir -p "$LONGOPS_AUDIT/commit-push/cp1"; echo '{}' >"$LONGOPS_AUDIT/commit-push/cp1/report.json"; h0=$(sha256sum "$LONGOPS_AUDIT/commit-push/cp1/report.json" | cut -c1-64)
mkdir -p "$FXN/callrun"; CPA_RUN_ID=cp2 CPA_RUN=$FXN/callrun "$S/acquire.sh" --expire commit_push; assert_rc "C7 --expire inside a run" $? 0
assert_eq "C7b resume_expired goes into the calling run's own report directory, naming the expired run" "$(jq -r .expired_run "$FXN/callrun/resume_expired.json")" cp1
assert_eq "C7c the expired run's directory is byte-unchanged" "$(sha256sum "$LONGOPS_AUDIT/commit-push/cp1/report.json" | cut -c1-64)$(ls "$LONGOPS_AUDIT/commit-push/cp1" | tr '\n' ,)" "$h0"report.json,
unset LONGOPS_NOW CPA_APPROVED_DIR

echo "== check_no_build_writing_tracked =="
newfx k1; mkll; A=$LL; "$S/check_no_build_writing_tracked.sh" >"$FXN/o"; assert_rc "K1 golden-true: no registered op, exit 0" $? 0
id=$("$S/register.sh" --purpose build:web:unit:abc --owner t --pid "$A" --write-path .audit/out/web)
"$S/check_no_build_writing_tracked.sh" >"$FXN/o"; assert_rc "K2 golden-true: a live build writing only an untracked, ignored path" $? 0
id2=$("$S/register.sh" --purpose build:api:unit:abc --owner t --pid "$A" --write-path tracked/a.txt); "$S/check_no_build_writing_tracked.sh" >"$FXN/o"; rc=$?
assert_rc "K3 golden-false: a live build writing a TRACKED path is refused" $rc 1
grep -q "$id2" "$FXN/o" && ok "K3b the refusal names the op" || bad "K3b [$(cat "$FXN/o")]"
"$S/check_no_build_writing_tracked.sh" --except-op-id "$id2" >"$FXN/o"; assert_rc "K4 the caller's own op can be excepted" $? 0
"$S/release.sh" --op-id "$id2" --state complete --verdict PASS; "$S/check_no_build_writing_tracked.sh" >"$FXN/o"; assert_rc "K5 a terminal op blocks nothing" $? 0
mkll; B=$LL; id3=$("$S/register.sh" --purpose go-test:x --owner stream-b --pid "$B" --write-path specs/001-full-project-audit-remediation/evidence/wp09/out.txt)
"$S/check_no_build_writing_tracked.sh" >"$FXN/o"; rc=$?; assert_rc "K6 any live op declaring a write path under \$EV is refused (not only builds)" $rc 1
grep -q "$id3" "$FXN/o" && ok "K6b names the op" || bad "K6b"
"$S/release.sh" --op-id "$id3" --state complete; mkll; C=$LL
id4=$("$S/register.sh" --purpose scan:y --owner t --pid "$C" --write-path specs/001-full-project-audit-remediation/audit/findings/FND-0001.json); "$S/check_no_build_writing_tracked.sh" >"$FXN/o"; assert_rc "K7 a write path under \$AUD is refused" $? 1
kill "$C"; wait "$C" 2>/dev/null; "$S/check_no_build_writing_tracked.sh" >"$FXN/o"; assert_rc "K8 a dead-owner row STILL blocks the window until it is reaped (owner gone is not workload gone, 11.4.276 LO-D3)" $? 1
"$S/reap.sh" --op-id "$id4" >/dev/null 2>&1; "$S/check_no_build_writing_tracked.sh" >"$FXN/o"; assert_rc "K8b once the dead-owner row is reaped it blocks nothing" $? 0
git -C "$LONGOPS_REPO" ls-files | grep -q . && ok "K9 fixture control: the fixture repository really tracks tracked/a.txt" || bad "K9"

echo "== require_verdicts =="
newfx v1; export LONGOPS_NOW=$(date +%s); now=$(date -u -d "@$LONGOPS_NOW" +%Y-%m-%dT%H:%M:%SZ); old=$(date -u -d "@$((LONGOPS_NOW-1000))" +%Y-%m-%dT%H:%M:%SZ)
echo "{\"verdict\":\"PASS\",\"fingerprint\":\"fp1\",\"utc\":\"$now\"}" >"$FXN/good.json"
echo "{\"verdict\":\"PASS\",\"fingerprint\":\"fp0\",\"utc\":\"$now\"}" >"$FXN/fp.json"
echo "{\"verdict\":\"PASS\",\"fingerprint\":\"fp1\",\"utc\":\"$old\"}" >"$FXN/old.json"
echo "{\"verdict\":\"FAIL\",\"fingerprint\":\"fp1\",\"utc\":\"$now\"}" >"$FXN/fail.json"
echo "{\"verdict\":\"PASS\",\"fingerprint\":\"fp1\"}" >"$FXN/nutc.json"
"$S/require_verdicts.sh" --file "$FXN/good.json" --fingerprint fp1 --max-age-s 60 >/dev/null; assert_rc "V1 golden-true: fresh PASS with the right fingerprint" $? 0
"$S/require_verdicts.sh" --file "$FXN/none.json" >"$FXN/o"; rc=$?; assert_rc "V2 golden-false: a missing verdict file is refused" $rc 1
grep -q missing "$FXN/o" && ok "V2b names missing" || bad "V2b"
"$S/require_verdicts.sh" --file "$FXN/fp.json" --fingerprint fp1 >"$FXN/o"; assert_rc "V3 a stale fingerprint is refused" $? 1
"$S/require_verdicts.sh" --file "$FXN/old.json" --max-age-s 60 >"$FXN/o"; assert_rc "V4 a verdict older than max-age is refused" $? 1
"$S/require_verdicts.sh" --file "$FXN/fail.json" >"$FXN/o"; assert_rc "V5 a FAIL verdict is refused" $? 1
"$S/require_verdicts.sh" --file "$FXN/nutc.json" --max-age-s 60 >"$FXN/o"; assert_rc "V6 an absent utc is stale, never fresh" $? 1
"$S/require_verdicts.sh" --file "$FXN/good.json" --file "$FXN/none.json" >"$FXN/o"; assert_rc "V7 one missing file among several refuses the lot" $? 1
echo '{bad' >"$FXN/bad.json"; "$S/require_verdicts.sh" --file "$FXN/bad.json" >"$FXN/o"; assert_rc "V8 an unparsable verdict is refused" $? 1

echo "== T089a: purpose-key grammar of a dispatched build, wall-clock cap =="
newfx g1; mkll; GP=$LL
H64=$(printf '%064d' 1); A64=$(printf '%064d' 2)
"$S/register.sh" --grammar build --purpose "build:catalog-api:unit:$H64:$A64:primary" --owner dispatch --pid "$GP" >"$FXN/o" 2>"$FXN/e"; assert_rc "G1 golden-true: a well-formed build purpose key is registered" $? 0
mkll; GP2=$LL; "$S/register.sh" --grammar build --purpose "build:catalog-api:integration:$H64:$A64:repro-cold:it7" --owner dispatch --pid "$GP2" >/dev/null 2>&1; assert_rc "G1b golden-true: variant repro-cold with an iteration id is registered" $? 0
for bad in "build:app:lane:abc:def:primary" "build:app:lane:${H64:1}:$A64:primary" "build:app:lane:$H64:$A64:fast" "build:app:lane:$H64:$A64:primary:bad iter" "build:app:$H64:$A64:primary" "build::lane:$H64:$A64:primary"; do
  mkll; X=$LL; "$S/register.sh" --grammar build --purpose "$bad" --owner dispatch --pid "$X" >/dev/null 2>"$FXN/e"; rc=$?
  assert_rc "G2 golden-false: a malformed build purpose key is refused ($bad)" $rc 2
done
grep -q purpose_key_malformed "$FXN/e" && ok "G2b the refusal names purpose_key_malformed" || bad "G2b [$(cat "$FXN/e")]"
mkll; X=$LL; "$S/register.sh" --purpose "build:x" --owner t --pid "$X" >/dev/null 2>&1; assert_rc "G3 without --grammar build a purpose keeps the older rule (any safe name)" $? 0
mkll; X=$LL; "$S/register.sh" --grammar nonsense --purpose "build:x" --owner t --pid "$X" >/dev/null 2>&1; assert_rc "G3b an unknown --grammar is a usage error" $? 2
newfx w1; mkll; W=$LL; export LONGOPS_NOW=$(date +%s)
id=$("$S/register.sh" --grammar build --purpose "build:app:lane:$H64:$A64:primary" --owner dispatch --pid "$W" --no-progress-s 600 --wall-s 5)
"$S/heartbeat.sh" --op-id "$id" --progress-offset 10 --elapsed-ms 3000 >/dev/null
assert_eq "W1 heartbeat records the build host's elapsed monotonic time" "$(jq -r .elapsed_ms "$LONGOPS_DIR/ops/$id.json")" 3000
assert_eq "W2 under the cap and advancing: advancing" "$("$S/classify.sh" --op-id "$id" | cut -f2)" advancing
"$S/heartbeat.sh" --op-id "$id" --progress-offset 20 --elapsed-ms 6000 >/dev/null
r=$("$S/classify.sh" --op-id "$id"); assert_eq "W3 an advancing but over-long build (elapsed > wall_clock_s) is hung" "$(cut -f2 <<<"$r")" hung
grep -q wall_clock <<<"$r" && ok "W3b the evidence names the wall-clock cap" || bad "W3b [$r]"
mkll; W2=$LL; id2=$("$S/register.sh" --purpose "build:app:lane2:$H64:$A64:primary" --owner dispatch --pid "$W2" --no-progress-s 600)
"$S/heartbeat.sh" --op-id "$id2" --progress-offset 5 --elapsed-ms 999999999 >/dev/null; assert_eq "W4 no cap recorded (wall_clock_s 0): never hung by elapsed time" "$("$S/classify.sh" --op-id "$id2" | cut -f2)" advancing
unset LONGOPS_NOW

echo "== WF11 class 1: every decision is made UNDER the lock and re-derived there (reap) =="
newfx w1; export LONGOPS_NOW=2000
bash -c 'trap "" TERM; while :; do sleep 1; done' >/dev/null 2>&1 & TP=$!; KILL9ME+=("$TP"); sleep 0.4
id=$("$S/register.sh" --purpose ta:x --owner t --pid "$TP" --no-progress-s 10); "$S/heartbeat.sh" --op-id "$id" --progress-offset 5; export LONGOPS_NOW=2100
LONGOPS_REAP_GRACE_S=2 "$S/reap.sh" --op-id "$id" >"$FXN/o" 2>"$FXN/e"; rc=$?
assert_rc "W1 F1 a process that IGNORES TERM is not reaped: exit 8 (survived), never 0" $rc 8
kill -0 "$TP" 2>/dev/null && ok "W1b control: the process really ignores TERM and is still alive" || bad "W1b it died"
assert_eq "W1c the record keeps its state (never reaped while the process lives)" "$(jq -r .state "$LONGOPS_DIR/ops/$id.json")" running
assert_eq "W1d the claim is kept: the purpose still has exactly one owner" "$([ -d "$LONGOPS_DIR/claims/ta:x" ] && echo present || echo gone)" present
mkll; "$S/register.sh" --purpose ta:x --owner other --pid "$LL" >/dev/null 2>&1; assert_rc "W1e a second registration of the purpose is refused (one owner)" $? 3
[ "$(jq -r '.reap_survived_utc // "null"' "$LONGOPS_DIR/ops/$id.json")" != null ] && ok "W1f the survival is recorded on the op (reap_survived_utc)" || bad "W1f"
"$S/release.sh" --op-id "$id" --state reaped --verdict operator >/dev/null 2>&1; assert_rc "W1g an operator decision can still end it (release --state reaped)" $? 0
unset LONGOPS_NOW
newfx w2; mkll; P=$LL; export LONGOPS_NOW=2000
id=$("$S/register.sh" --purpose tb:x --owner t --pid "$P" --no-progress-s 10); "$S/heartbeat.sh" --op-id "$id" --progress-offset 5; export LONGOPS_NOW=2100
LONGOPS_TEST_SLEEP_BEFORE_LOCK=1.5 "$S/reap.sh" --op-id "$id" >"$FXN/o" 2>"$FXN/e" & RP=$!
sleep 0.5; "$S/heartbeat.sh" --op-id "$id" --progress-offset 500; wait "$RP"; rc=$?
assert_rc "W2 F2 a heartbeat that lands before the reap takes the lock is SEEN: the op is advancing, refused (5)" $rc 5
kill -0 "$P" 2>/dev/null && ok "W2b the advancing process was never signalled" || bad "W2b it was terminated although it was advancing"
[ ! -s "$LONGOPS_DIR/signals.log" ] && ok "W2c no signal was recorded" || bad "W2c"
assert_eq "W2d the record is still running" "$(jq -r .state "$LONGOPS_DIR/ops/$id.json")" running
newfx w3; mkll; A=$LL; "$S/register.sh" --purpose tp:x --owner t --pid "$A" >/dev/null; kill "$A"; wait "$A" 2>/dev/null
LONGOPS_TEST_SLEEP_BEFORE_LOCK=8 "$S/reap.sh" --purpose tp:x >"$FXN/o" 2>"$FXN/e" & RP=$!
sleep 0.4; "$S/reap.sh" --purpose tp:x >/dev/null 2>&1; mkll; B=$LL; "$S/acquire.sh" --purpose tp:x --run-id newrun --pid "$B" >/dev/null 2>&1   # a holder with NO op record: only the holder re-read under the lock can refuse (an op-backed holder is also refused by the live-op guard, WF14)
wait "$RP"; rc=$?
assert_rc "W3 F2 reap --purpose re-reads the holder UNDER the lock: a live holder that took the purpose in the window is refused (5)" $rc 5
assert_eq "W3b the new live holder's claim is intact" "$(jq -r .pid "$LONGOPS_DIR/claims/tp:x/holder.json" 2>/dev/null)" "$B"
unset LONGOPS_NOW
newfx w4; mkll; A=$LL; mkll; B=$LL; export LONGOPS_TEST_SLEEP_IN_CS=1
"$S/register.sh" --purpose px:1 --owner a --op-id same-id --pid "$A" >"$FXN/oa" 2>"$FXN/ea" & W1=$!; sleep 0.3
unset LONGOPS_TEST_SLEEP_IN_CS; "$S/register.sh" --purpose py:1 --owner b --op-id same-id --pid "$B" >"$FXN/ob" 2>"$FXN/eb"; rb=$?; wait "$W1"; ra=$?
assert_eq "W4 F2 two registrations of ONE op id under different purposes: exactly one wins (record created exclusively)" "$(( (ra == 0) + (rb == 0) ))" 1
assert_eq "W4b the loser made no claim at all (the record is created FIRST, so the loser stops at the exclusive record write: one claim, the winner's)" "$(ls "$LONGOPS_DIR/claims" | tr '\n' ' ')" "px:1 "
assert_eq "W4c the record belongs to the winner (the first to create it)" "$(jq -r .purpose_key "$LONGOPS_DIR/ops/same-id.json")" px:1

echo "== WF11 class 2: refusal / unreadable input is a distinct status, never clean, stale or advancing =="
newfx w5; mkll; A=$LL; unset CPA_APPROVED_DIR
"$S/acquire.sh" --purpose commit_push --run-id cpx --pid "$A" >/dev/null 2>&1
"$S/reap.sh" --purpose commit_push >"$FXN/o" 2>"$FXN/e"; rc=$?; assert_rc "X1 F3 reap --purpose commit_push with CPA_APPROVED_DIR unset REFUSES (20), it is never read as a stale claim" $rc 20
assert_eq "X1b the claim of the live holder is still there" "$([ -d "$LONGOPS_DIR/claims/commit_push" ] && echo present || echo gone)" present
mkdir -p "$FXN/approved/scripts/repo"; export CPA_APPROVED_DIR=$FXN/approved
"$S/reap.sh" --purpose commit_push >"$FXN/o" 2>"$FXN/e"; rc=$?; assert_rc "X2 with it set, a live holder is refused (5)" $rc 5
printf '{"kind":"proc' >"$LONGOPS_DIR/claims/commit_push/holder.json"
"$S/holder.sh" commit_push >"$FXN/o" 2>"$FXN/e"; rc=$?; assert_rc "X3 holder.sh on a truncated holder record is 20, never none" $rc 20
grep -q holder_unreadable "$FXN/e" && ok "X3b names holder_unreadable" || bad "X3b [$(cat "$FXN/e")]"
"$S/reap.sh" --purpose commit_push >"$FXN/o" 2>"$FXN/e"; rc=$?; assert_rc "X4 reap --purpose on an unreadable holder record REFUSES (20): a live holder's claim is never released" $rc 20
assert_eq "X4b the claim survives" "$([ -d "$LONGOPS_DIR/claims/commit_push" ] && echo present || echo gone)" present
"$S/register.sh" --purpose commit_push --owner t --pid "$$" >/dev/null 2>"$FXN/e"; rc=$?; assert_rc "X4c register on an unreadable claim is refused (20), not stale_claim (4)" $rc 20
rm -rf "$LONGOPS_DIR/claims/commit_push"; mkdir "$LONGOPS_DIR/claims/commit_push"
"$S/reap.sh" --purpose commit_push >/dev/null 2>&1; rc=$?; assert_rc "X5 a claim directory with no holder record at all is stale: released (0)" $rc 0
unset CPA_APPROVED_DIR
# RM4: lo_claim must not swallow the conf-unreadable refusal
newfx w6; mkll; A=$LL; mkdir -p "$FXN/approved/scripts/repo" "$LONGOPS_AUDIT/builds/b9/terminal"; export CPA_APPROVED_DIR=$FXN/approved
"$S/acquire.sh" --purpose commit_push --run-id cp1 --pid "$A" >/dev/null; "$S/acquire.sh" --suspend cp1 --builds b9 >/dev/null; "$S/acquire.sh" --update cp1 --callback-state done --state ready_to_resume >/dev/null
"$S/acquire.sh" --purpose commit_push --run-id cp9 --pid "$$" >/dev/null 2>"$FXN/e"; rc=$?; assert_rc "X6 RM4 acquiring over a ready_to_resume holder whose conf is unreadable is 20, never a stale_claim takeover (4)" $rc 20
assert_eq "X6b the holder is still cp1" "$(jq -r .run_id "$LONGOPS_DIR/claims/commit_push/holder.json")" cp1
"$S/register.sh" --purpose commit_push --owner t --pid "$$" >/dev/null 2>&1; assert_rc "X6c register hits the same refusal (20)" $? 20
unset CPA_APPROVED_DIR
newfx w7; mkll; P=$LL; id=$("$S/register.sh" --purpose uo:x --owner t --pid "$P" --no-progress-s 30); : >"$LONGOPS_DIR/ops/$id.json"
assert_eq "X7 a 0-byte op record is classified unreadable (never dead_owner or advancing)" "$("$S/classify.sh" --op-id "$id" | cut -f2)" unreadable
"$S/reap.sh" --op-id "$id" >/dev/null 2>&1; assert_rc "X7b reap refuses it (20)" $? 20
"$S/heartbeat.sh" --op-id "$id" >/dev/null 2>&1; assert_rc "X7c heartbeat refuses it (20), not unknown_op" $? 20
"$S/release.sh" --op-id "$id" --state failed >/dev/null 2>&1; assert_rc "X7d release refuses it (20)" $? 20
"$S/check_no_build_writing_tracked.sh" >"$FXN/o" 2>/dev/null; rc=$?; assert_rc "X7e check_no_build_writing_tracked BLOCKS on an unreadable record (1), never reads it as no writer" $rc 1
printf '{"op_id":"trunc",' >"$LONGOPS_DIR/ops/trunc.json"; assert_eq "X7f a truncated record is unreadable too" "$("$S/classify.sh" --op-id trunc | cut -f2)" unreadable
newfx w8; mkll; P=$LL; id=$("$S/register.sh" --purpose ov:x --owner t --pid "$P" --no-progress-s 600 --wall-s 5)
jq -c '.elapsed_ms=99999999999999999999' "$LONGOPS_DIR/ops/$id.json" >"$FXN/t" && mv "$FXN/t" "$LONGOPS_DIR/ops/$id.json"
assert_eq "X8 F13 an elapsed_ms beyond 15 digits is unreadable, never read as advancing" "$("$S/classify.sh" --op-id "$id" | cut -f2)" unreadable
"$S/classify.sh" --op-id no-such-op >/dev/null 2>"$FXN/e"; rc=$?; assert_rc "X9 F14 classify --op-id of an op that does not exist is 4 (unknown_op), not an empty success" $rc 4
"$S/classify.sh" bogus-arg >/dev/null 2>&1; assert_rc "X9b classify with an unknown argument is a usage error (2)" $? 2
LONGOPS_NOW=abc "$S/classify.sh" >/dev/null 2>"$FXN/e"; assert_rc "X10 a non-numeric LONGOPS_NOW is refused (2) at load" $? 2

echo "== WF11 class 3: unvalidated input never writes an empty record or rebinds an op to init / a dead pid =="
newfx w9; mkll; P=$LL; id=$("$S/register.sh" --purpose va:x --owner t --pid "$P" --no-progress-s 30); h0=$(sha256sum "$LONGOPS_DIR/ops/$id.json" | cut -c1-64)
for a in "--pid abc" "--pid 1" "--pid 0" "--pid 99999999" "--elapsed-ms abc" "--elapsed-ms 1e20" "--elapsed-ms 99999999999999999999" "--progress-offset -5" "--progress-offset x"; do
  "$S/heartbeat.sh" --op-id "$id" $a >/dev/null 2>&1; rc=$?; assert_rc "V1 F5/F6 heartbeat $a is refused (2)" $rc 2
done
assert_eq "V1b none of them changed the record (no 0-byte write, no rebind)" "$(sha256sum "$LONGOPS_DIR/ops/$id.json" | cut -c1-64)" "$h0"
mkll; Q=$LL; "$S/heartbeat.sh" --op-id "$id" --pid "$Q" >/dev/null 2>&1; assert_rc "V2 a heartbeat may rebind to a live pid > 1" $? 0
assert_eq "V2b the rebind records pid, start time AND cmdline of the new process together" "$(jq -r '[.pid,.start_time,.cmdline]|@tsv' "$LONGOPS_DIR/ops/$id.json")" "$(printf '%s\t%s\t%s' "$Q" "$(pstart "$Q")" "$(tr '\0' ' ' </proc/$Q/cmdline | sed 's/ $//')")"
for pp in 1 0 abc 99999999 -1 ""; do "$S/register.sh" --purpose "vb:$pp" --owner t --pid "$pp" >/dev/null 2>&1; rc=$?; assert_rc "V3 F6 register --pid '$pp' is refused (2)" $rc 2; done
assert_eq "V3b no op record and no claim was created by them" "$(ls "$LONGOPS_DIR/ops" | wc -l):$(ls "$LONGOPS_DIR/claims" | wc -l)" "1:1"
"$S/acquire.sh" --purpose vc:x --run-id r1 --pid 1 >/dev/null 2>&1; assert_rc "V4 acquire --pid 1 is refused (2)" $? 2
"$S/acquire.sh" --purpose vd:x --run-id r1 --pid "$P" >/dev/null; "$S/acquire.sh" --suspend r1 --purpose vd:x --builds b1 --resume-ttl abc >/dev/null 2>&1; assert_rc "V5 F5 acquire --suspend --resume-ttl abc is refused (2)" $? 2
assert_eq "V5b the holder is untouched (still a process holder, not an empty file)" "$(jq -r .kind "$LONGOPS_DIR/claims/vd:x/holder.json")" process
"$S/acquire.sh" --update r1 --purpose vd:x --callback-state bogus >/dev/null 2>&1; assert_rc "V5c an unknown --callback-state is refused (2)" $? 2
( . "$S/lib.sh"; LD=$FXN/wj; mkdir -p "$LD"; lo_wjson "$LD/a.json" ""; r1=$?; lo_wjson "$LD/a.json" "abc"; r2=$?; lo_wjson "$LD/a.json" '{"a":1}'; r3=$?; lo_wjson "$LD/a.json" ""; r4=$?; echo "$r1 $r2 $r3 $r4 $(cat "$LD/a.json")" ) >"$FXN/o"
assert_eq "V6 F5 lo_wjson refuses empty and invalid JSON (1), writes valid JSON (0), and a refused write leaves the old file intact" "$(cat "$FXN/o")" '1 1 0 1 {"a":1}'
mkdir -p "$FXN/wj"; [ -z "$(find "$FXN/wj" -name '.tmp.*')" ] && ok "V6b no temp file is left behind by a refused write" || bad "V6b"
# a suspended holder whose resume_ttl is not a number: refused, never a silent empty swap
newfx w10; mkll; A=$LL; mkdir -p "$LONGOPS_AUDIT/builds/b1/terminal"; "$S/acquire.sh" --purpose rt:x --run-id q1 --pid "$A" >/dev/null; "$S/acquire.sh" --suspend q1 --purpose rt:x --builds b1 --resume-ttl 50 >/dev/null; "$S/acquire.sh" --update q1 --purpose rt:x --callback-state done --state ready_to_resume >/dev/null
jq -c '.resume_ttl="abc"' "$LONGOPS_DIR/claims/rt:x/holder.json" >"$FXN/t" && mv "$FXN/t" "$LONGOPS_DIR/claims/rt:x/holder.json"
"$S/holder.sh" rt:x >/dev/null 2>&1; assert_rc "V7 a holder with a non-numeric resume_ttl is refused (20), not judged" $? 20
"$S/acquire.sh" --expire rt:x --op-id e1 >/dev/null 2>&1; assert_rc "V7b --expire on it is refused (20) and the claim stays" $? 20
[ -d "$LONGOPS_DIR/claims/rt:x" ] && ok "V7c the claim is still there" || bad "V7c"

echo "== WF11 F7: no op is ever \"never hung\" =="
newfx w11; mkll; P=$LL; id=$("$S/register.sh" --purpose df:x --owner t --pid "$P"); mkll; P2=$LL; id2=$("$S/register.sh" --purpose df:y --owner t --pid "$P2" --no-progress-s 0)
assert_eq "D1 register with no budget records the declared default (3600), not 0" "$(jq -r .budget.no_progress_s "$LONGOPS_DIR/ops/$id.json")" 3600
assert_eq "D1b --no-progress-s 0 records the default too" "$(jq -r .budget.no_progress_s "$LONGOPS_DIR/ops/$id2.json")" 3600
export LONGOPS_NOW=$(( $(date +%s) + 86400 )); assert_eq "D2 a flat offset for a day is HUNG under the default budget" "$("$S/classify.sh" --op-id "$id" | cut -f2)" hung
jq -c '.budget.no_progress_s=0' "$LONGOPS_DIR/ops/$id2.json" >"$FXN/t" && mv "$FXN/t" "$LONGOPS_DIR/ops/$id2.json"; assert_eq "D3 a legacy record with budget 0 is judged by the default too" "$("$S/classify.sh" --op-id "$id2" | cut -f2)" hung
unset LONGOPS_NOW

echo "== WF11 class 4: terminal states are immutable; a stale release can never free another owner's claim =="
newfx w12; mkll; P=$LL; id=$("$S/register.sh" --purpose tm:x --owner t --pid "$P")
"$S/release.sh" --op-id "$id" --state complete --verdict PASS >/dev/null; assert_rc "T1 release complete" $? 0
"$S/release.sh" --op-id "$id" --state reaped --verdict x >/dev/null 2>"$FXN/e"; rc=$?; assert_rc "T2 F11 a second release into another state is refused (4 already_terminal)" $rc 4
assert_eq "T2b the record is still complete/PASS" "$(jq -r '.state+"/"+.verdict' "$LONGOPS_DIR/ops/$id.json")" complete/PASS
"$S/release.sh" --op-id "$id" --state complete --verdict PASS >/dev/null 2>&1; assert_rc "T3 the same state and verdict again is an idempotent no-op (0)" $? 0
"$S/release.sh" --op-id "$id" --state complete --verdict OTHER >/dev/null 2>&1; assert_rc "T3b the same state with another verdict is refused (4)" $? 4
mkll; P=$LL; id=$("$S/register.sh" --purpose tm:y --owner t --pid "$P"); "$S/release.sh" --op-id "$id" --state handoff --verdict driver_stop >/dev/null
"$S/release.sh" --op-id "$id" --state complete --verdict adopted >/dev/null 2>&1; assert_rc "T4 a handoff (re-adoptable) may be resolved into a terminal state" $? 0
newfx w13; mkll; A=$LL; ida=$("$S/register.sh" --purpose tz:x --owner t --pid "$A" --op-id opa); kill "$A"; wait "$A" 2>/dev/null; "$S/reap.sh" --op-id "$ida" >/dev/null 2>&1
mkll; B=$LL; idb=$("$S/register.sh" --purpose tz:x --owner t --pid "$B" --op-id opb)
"$S/release.sh" --op-id "$ida" --state failed >/dev/null 2>&1; rc=$?; assert_rc "T5 re-releasing a reaped op is refused (4)" $rc 4
assert_eq "T5b the NEW owner's claim is intact" "$(jq -r .run_id "$LONGOPS_DIR/claims/tz:x/holder.json")" opb
assert_eq "T5c and the reaped record is unchanged" "$(jq -r .state "$LONGOPS_DIR/ops/$ida.json")" reaped

echo "== WF11 F16 reviewer mutants: tests that fail on RM1-RM3 =="
kt=""; for d in /proc/[0-9]*; do n=${d#/proc/}; [ "$n" -gt 1 ] 2>/dev/null || continue; pg=$(sed 's/^.*) //' "$d/stat" 2>/dev/null | cut -d' ' -f3); [ "$pg" = 0 ] && { kt=$n; break; }; done
if [ -n "$kt" ]; then
  ( . "$S/lib.sh"; LD=$FXN/sig2; mkdir -p "$LD"; lo_signal TERM "$kt" >/dev/null 2>&1; a=$?; lo_kill_child "$kt" >/dev/null 2>&1; b=$?; echo "$a $b $(ls "$LD" | wc -l)" ) >"$FXN/o"
  assert_eq "M1 RM1 a pid > 1 whose process GROUP is <= 1 (kernel thread $kt, pgrp 0) is refused by lo_signal and lo_kill_child (7 7), no audit entry" "$(cat "$FXN/o")" "7 7 0"
else ok "M1 skipped: no kernel thread (pgrp 0) on this host; UNCONFIRMED here (11.4.3)"; fi
python3 -I -c 'import os,sys,time
pid=os.fork()
if pid==0: os._exit(0)
print(pid); sys.stdout.flush(); time.sleep(120)' >"$FXN/zpid" 2>/dev/null & ZP=$!; KILLME+=("$ZP"); sleep 0.6; Z=$(head -1 "$FXN/zpid")
newfx w14; zs=$(sed 's/^.*) //' "/proc/$Z/stat" 2>/dev/null | cut -d' ' -f1); assert_eq "M2a control: the target process really is a zombie (state Z)" "$zs" Z
"$S/register.sh" --purpose zb:y --owner t --pid "$Z" --no-progress-s 30 >/dev/null 2>&1; assert_rc "M2c WF14 R2-5 register --pid <zombie> is refused (2): a zombie is no owner" $? 2
mkll; zid=$("$S/register.sh" --purpose zb:x --owner t --pid "$LL" --no-progress-s 30 2>/dev/null); jq -c --argjson z "$Z" --arg st "$(pstart "$Z")" '.pid=$z|.start_time=$st' "$LONGOPS_DIR/ops/$zid.json" >"$FXN/t" && mv "$FXN/t" "$LONGOPS_DIR/ops/$zid.json"
assert_eq "M2 RM2 an op whose owner is a ZOMBIE (hand-written record: register refuses it) is dead_owner, not alive" "$("$S/classify.sh" --op-id "$zid" | cut -f2)" dead_owner
"$S/acquire.sh" --purpose zc:y --run-id zr --pid "$Z" >/dev/null 2>&1; assert_rc "M2d acquire --pid <zombie> is refused (2)" $? 2
mkll; "$S/acquire.sh" --purpose zc:x --run-id zr --pid "$LL" >/dev/null 2>&1; jq -c --argjson z "$Z" --arg st "$(pstart "$Z")" '.pid=$z|.start_time=$st' "$LONGOPS_DIR/claims/zc:x/holder.json" >"$FXN/t" && mv "$FXN/t" "$LONGOPS_DIR/claims/zc:x/holder.json"
assert_eq "M2b a holder that is a zombie (hand-written record) reads none" "$("$S/holder.sh" zc:x)" none
newfx w15; mkll
for bp in 'a/b' 'a/../../x' 'x/..' '..' '.hidden' '-x' 'a b' $'a\nb' 'a:b/c'; do "$S/register.sh" --purpose "$bp" --owner t --pid "$LL" >/dev/null 2>&1; rc=$?; assert_rc "M3 RM3 an unsafe purpose key is refused (2): $(printf '%q' "$bp")" $rc 2; done
[ -z "$(find "$FXN" -name 'x.lock' -o -name 'b.lock')" ] && ok "M3b nothing was created outside the state directory" || bad "M3b"

echo "== WF11: lo_signal and the guarded child kill are the ONLY kill sites of the scope (11.4.263 D) =="
ks=$(awk 'FNR==1{f=FILENAME} { l=$0; sub(/^[ \t]+/,"",l); if (l ~ /^#/) next; if (l ~ /(^|[^A-Za-z0-9_.-])kill([ \t]|$)/) { sub(/[ \t]+#.*$/,"",l); print l } }' "$S"/*.sh "$TROOT/scripts/anti-mess/sweep.sh" | sort)
assert_eq "K10 exactly three kill lines exist in the production scripts: lo_alive kill -0, lo_signal, lo_kill_child" "$(printf '%s\n' "$ks" | wc -l)" 3
printf '%s\n' "$ks" | grep -qx 'kill -s "$sig" -- "$pid" 2>/dev/null' && printf '%s\n' "$ks" | grep -qx 'kill -s TERM -- "$pid" 2>/dev/null' && printf '%s\n' "$ks" | grep -q 'kill -0 "$pid" 2>/dev/null' && ok "K10b they are the three expected lines (single pid after the double dash, never a group)" || bad "K10b [$ks]"

echo "== WF14 round 3: ground truth taken from the REAL producers (dispatch.sh pump trap, heartbeat rebind, runner/dispatch container labels) =="
# R2-1: the shape of the pump at scripts/build/dispatch.sh:546 -- the owner's TERM trap runs `release.sh --state handoff` (which takes the SAME purpose flock) and exits. The owner's lock wait is shortened to 1 s
# (LONGOPS_LOCK_WAIT_S) so that a reap holding the lock across its grace wait makes the owner's release fail (70) and the handoff is never written.
newfx n1; export LONGOPS_NOW=2000
cat >"$FXN/owner.sh" <<EOF
trap 'LONGOPS_LOCK_WAIT_S=1 "$S/release.sh" --op-id coop --state handoff --verdict driver_stop; exit 0' TERM
while :; do sleep 0.2; done
EOF
bash "$FXN/owner.sh" >/dev/null 2>&1 & CP=$!; KILLME+=("$CP"); sleep 0.4
id=$("$S/register.sh" --purpose co:x --owner t --op-id coop --pid "$CP" --no-progress-s 10); "$S/heartbeat.sh" --op-id "$id" --progress-offset 5; export LONGOPS_NOW=2100
"$S/reap.sh" --op-id "$id" >"$FXN/o" 2>"$FXN/e"; rc=$?
assert_rc "N1 R2-1 a COOPERATING owner (TERM trap releases the op itself, the dispatch pump shape) is reaped without a false survival: exit 0, not 8" $rc 0
assert_eq "N1b the owner's handoff answers OUR reap intent: the record ends reaped, verdict reaped_hung:owner_handoff, the owner's own verdict kept (a build judged HUNG is not re-adoptable, 11.4.276 LO-D2)" "$(jq -r '.state+"/"+.verdict+"/"+.owner_verdict' "$LONGOPS_DIR/ops/$id.json")" "reaped/reaped_hung:owner_handoff/driver_stop"
assert_eq "N1c no reap_survived marker was written for a cooperating owner" "$(jq -r '.reap_survived_utc // "none"' "$LONGOPS_DIR/ops/$id.json")" none
assert_eq "N1d the owner released the claim itself" "$([ -d "$LONGOPS_DIR/claims/co:x" ] && echo present || echo gone)" gone
grep -q "TERM pid=$CP" "$LONGOPS_DIR/signals.log" && ok "N1e CONTROL: the owner really was signalled (the audit trail names it)" || bad "N1e"
kill -0 "$CP" 2>/dev/null && bad "N1f the owner is still alive" || ok "N1f the owner exited after its trap"
unset LONGOPS_NOW
# R2-8: reap holds NO lock across podman: a slow container runtime never makes an owner's heartbeat time out (flock -w 15 -> exit 70)
newfx n2; mkll; P=$LL; export LONGOPS_NOW=2000
printf '#!/bin/sh\ncase "$1" in ps) sleep 3 ;; esac\nexit 0\n' >"$FXN/podman-slow"; chmod +x "$FXN/podman-slow"
id=$("$S/register.sh" --purpose sp:x --owner t --pid "$P" --no-progress-s 10); "$S/heartbeat.sh" --op-id "$id" --progress-offset 5; export LONGOPS_NOW=2100
LONGOPS_PODMAN=$FXN/podman-slow "$S/reap.sh" --op-id "$id" --dry-run >"$FXN/o" 2>&1 & RP=$!; sleep 0.8
t0=$(date +%s.%N); "$S/heartbeat.sh" --op-id "$id" --progress-offset 6 >/dev/null 2>&1; hrc=$?; t1=$(date +%s.%N); wait "$RP"
assert_rc "N2 R2-8 a heartbeat during a slow container listing is recorded (0)" $hrc 0
[ "$(echo "$t1 - $t0 < 2.0" | bc)" = 1 ] && ok "N2b it did not wait for the container runtime (the lock is not held across podman)" || bad "N2b waited $(echo "$t1 - $t0" | bc)s"
unset LONGOPS_NOW
# R2-3 / R2-5: a rebind moves the CLAIM HOLDER with the op owner; reap --purpose never frees the claim of a live op
newfx n3; mkll; A=$LL; mkll; B=$LL; export LONGOPS_NOW=3000
id=$("$S/register.sh" --purpose rb:x --owner t --pid "$A" --no-progress-s 600); "$S/heartbeat.sh" --op-id "$id" --pid "$B" >/dev/null
assert_eq "N3 R2-3 the rebind moved the claim holder to the new owner (pid, start time, cmdline)" "$(jq -r '[.pid,.start_time]|@tsv' "$LONGOPS_DIR/claims/rb:x/holder.json")" "$(printf '%s\t%s' "$B" "$(pstart "$B")")"
kill "$A"; wait "$A" 2>/dev/null
assert_eq "N3b after the launcher exits the holder still reads live (the worker owns the purpose)" "$(jq -r .status <<<"$("$S/holder.sh" rb:x)")" live
"$S/reap.sh" --purpose rb:x >/dev/null 2>&1; assert_rc "N3c reap --purpose refuses a live holder (5)" $? 5
mkll; C=$LL; "$S/register.sh" --purpose rb:x --owner second --pid "$C" >/dev/null 2>&1; assert_rc "N3d a second registration is refused (3): one owner" $? 3
# the guard itself: a holder record that names a dead pid while the op's owner lives (written by hand: the pre-fix rebind) must still not be released
newfx n4; mkll; A=$LL; mkll; B=$LL; export LONGOPS_NOW=3000
id=$("$S/register.sh" --purpose rg:x --owner t --pid "$A" --no-progress-s 600); jq -c --argjson b "$B" --arg st "$(pstart "$B")" '.pid=$b|.start_time=$st' "$LONGOPS_DIR/ops/$id.json" >"$FXN/t" && mv "$FXN/t" "$LONGOPS_DIR/ops/$id.json"; kill "$A"; wait "$A" 2>/dev/null
"$S/reap.sh" --purpose rg:x >"$FXN/o" 2>"$FXN/e"; rc=$?; assert_rc "N4 R2-3 reap --purpose of a DEAD holder refuses (5) while a non-terminal op of that purpose has a live owner" $rc 5
grep -q "$id" "$FXN/e" && ok "N4b the refusal names the op" || bad "N4b [$(cat "$FXN/e")]"
assert_eq "N4c the claim is intact" "$([ -d "$LONGOPS_DIR/claims/rg:x" ] && echo present || echo gone)" present
printf '{"op_id":"unr","purpose_key":"rg:x","state":"running","pid":"abc"}' >"$LONGOPS_DIR/ops/unr.json"
"$S/reap.sh" --purpose rg:x >/dev/null 2>&1; assert_rc "N4d a valid-JSON op record that cannot be judged (pid 'abc') refuses (20) BEFORE any purpose filter or live-op verdict: never read as dead, never 'another purpose's'" $? 20
kill "$B"; wait "$B" 2>/dev/null
unset LONGOPS_NOW
# R2-5: pid validation: a zombie and a kernel thread are no owner
newfx n5; mkll; A=$LL; id=$("$S/register.sh" --purpose zk:x --owner t --pid "$A" --no-progress-s 30); h0=$(sha256sum "$LONGOPS_DIR/ops/$id.json" | cut -c1-64)
"$S/heartbeat.sh" --op-id "$id" --pid "$Z" >/dev/null 2>&1; assert_rc "N5 R2-5 heartbeat --pid <zombie> is refused (2)" $? 2
if [ -n "$kt" ]; then "$S/heartbeat.sh" --op-id "$id" --pid "$kt" >/dev/null 2>&1; assert_rc "N5b heartbeat --pid <kernel thread $kt, pgrp 0> is refused (2)" $? 2
  "$S/register.sh" --purpose zk:y --owner t --pid "$kt" >/dev/null 2>&1; assert_rc "N5c register --pid <kernel thread> is refused (2)" $? 2; else ok "N5b skipped: no kernel thread on this host (UNCONFIRMED here, 11.4.3)"; fi
assert_eq "N5d none of them changed the record" "$(sha256sum "$LONGOPS_DIR/ops/$id.json" | cut -c1-64)" "$h0"
# R2-6: the default budget is validated at load
newfx n6; mkll
for v in 0 abc -5; do LONGOPS_LOCK_WAIT_S=$v "$S/classify.sh" >/dev/null 2>&1; assert_rc "N6d LONGOPS_LOCK_WAIT_S='$v' is refused (2) at load" $? 2; LONGOPS_DEFAULT_NO_PROGRESS_S=$v "$S/classify.sh" >/dev/null 2>"$FXN/e"; rc=$?; assert_rc "N6 R2-6 LONGOPS_DEFAULT_NO_PROGRESS_S='$v' is refused (2) at load, never 'never hung'" $rc 2; done
LONGOPS_DEFAULT_NO_PROGRESS_S=0 "$S/register.sh" --purpose df:z --owner t --pid "$LL" >/dev/null 2>&1; assert_rc "N6b register refuses it too (2)" $? 2
LONGOPS_DEFAULT_NO_PROGRESS_S=120 "$S/classify.sh" >/dev/null 2>&1; assert_rc "N6c control: a valid override loads (0)" $? 0
# R2-10: a record write that succeeded is never reported as a failed CAS
newfx n7; mkll; A=$LL; mkll; B=$LL
"$S/register.sh" --purpose nc:x --owner a --op-id opA --pid "$A" >/dev/null; "$S/register.sh" --purpose nc:x --owner b --op-id opB --no-claim --pid "$B" >/dev/null
kill "$B"; wait "$B" 2>/dev/null; "$S/reap.sh" --op-id opB >"$FXN/o" 2>"$FXN/e"; rc=$?
assert_rc "N7 R2-10 reaping a --no-claim op whose purpose is claimed by ANOTHER op succeeds (0): the record is written, the other claim untouched" $rc 0
assert_eq "N7b state reaped, the claim is still opA's" "$(jq -r .state "$LONGOPS_DIR/ops/opB.json"):$(jq -r .run_id "$LONGOPS_DIR/claims/nc:x/holder.json")" reaped:opA
newfx n8; mkll; A=$LL; mkll; B=$LL
"$S/register.sh" --purpose ho:x --owner a --op-id hoA --pid "$A" >/dev/null; "$S/release.sh" --op-id hoA --state handoff --verdict driver_stop >/dev/null
"$S/register.sh" --purpose ho:x --owner a --op-id hoA-a2 --pid "$B" >/dev/null
"$S/release.sh" --op-id hoA --state complete --verdict adopted >"$FXN/o" 2>"$FXN/e"; rc=$?
assert_rc "N8 R2-2/R2-10 resolving a HANDOFF op whose purpose was re-claimed by its successor succeeds (0), never a post-write failure (4)" $rc 0
assert_eq "N8b the handoff op is complete, the successor keeps its claim" "$(jq -r .state "$LONGOPS_DIR/ops/hoA.json"):$(jq -r .run_id "$LONGOPS_DIR/claims/ho:x/holder.json")" complete:hoA-a2
# R2-4: the container of a dispatched build is found by the label the record carries (dispatch.sh sets catalogizer.op_id=dispatch-<build id>, the op id is <build id>)
newfx n9; fpod; mkll; P=$LL; export LONGOPS_NOW=4000
id=$("$S/register.sh" --purpose dp:x --owner dispatch --op-id bld7 --pid "$P" --no-progress-s 5 --container-label "catalogizer.op_id=dispatch-bld7"); fcont cidD7 "catalogizer.op_id=dispatch-bld7"; export LONGOPS_NOW=4100
"$S/reap.sh" --op-id "$id" >/dev/null 2>&1; assert_rc "N9 R2-4 a hung dispatched build is reaped (0)" $? 0
grep -q 'stop -t 5 cidD7' "$PODDIR/calls" && ok "N9b its container is resolved by the record's own container_label and stopped" || bad "N9b [$(cat "$PODDIR/calls" 2>/dev/null)]"
export LONGOPS_PODMAN=$FXN/podman-null; unset LONGOPS_NOW
# the default fixture never reaches the real podman (R2-T2)
newfx n10; assert_eq "N10 R2-T2 the default LONGOPS_PODMAN of every fixture is the null stub, never /usr/bin/podman" "$LONGOPS_PODMAN" "$FXN/podman-null"
# class D (valid JSON, one bad field): every per-field branch of the classification (reviewer mutants RM5, RM11 and the pid/start_time class)
newfx n11; mkll; P=$LL; export LONGOPS_NOW=5000
id=$("$S/register.sh" --purpose bf:x --owner t --pid "$P" --no-progress-s 30); rec=$LONGOPS_DIR/ops/$id.json; cp "$rec" "$FXN/good.json"
for fld in '.last_progress_epoch="abc"' '.budget.no_progress_s="x"' '.budget.wall_clock_s=-1' '.elapsed_ms="1e3"' '.pid="abc"' '.pid=[1]' '.start_time={"a":1}' '.pid=1.5'; do
  jq -c "$fld" "$FXN/good.json" >"$rec"; r=$("$S/classify.sh" --op-id "$id" 2>"$FXN/e" | cut -f2)
  assert_eq "N11 a valid-JSON record with ONE bad field ($fld) is unreadable, never dead, advancing or an empty class" "$r" unreadable
  [ ! -s "$FXN/e" ] && ok "N11b ... and it raises no shell error ($fld)" || bad "N11b [$(head -c 160 "$FXN/e")]"
done
cp "$FXN/good.json" "$rec"; assert_eq "N11c control: the unmodified record is advancing" "$("$S/classify.sh" --op-id "$id" | cut -f2)" advancing
newfx n12; mkll; A=$LL; "$S/acquire.sh" --purpose hb:x --run-id hr --pid "$A" >/dev/null
for fld in '.pid="abc"' '.pid=null' '.pid=[1]'; do
  jq -c "$fld" "$LONGOPS_DIR/claims/hb:x/holder.json" >"$FXN/t" && cp "$LONGOPS_DIR/claims/hb:x/holder.json" "$FXN/hgood" && mv "$FXN/t" "$LONGOPS_DIR/claims/hb:x/holder.json"
  "$S/holder.sh" hb:x >/dev/null 2>&1; assert_rc "N12 a holder record with ONE bad field ($fld) is refused (20), never none" $? 20
  "$S/reap.sh" --purpose hb:x >/dev/null 2>&1; assert_rc "N12b reap --purpose does not release it (20)" $? 20
  assert_eq "N12c the claim survives ($fld)" "$([ -d "$LONGOPS_DIR/claims/hb:x" ] && echo present || echo gone)" present
  cp "$FXN/hgood" "$LONGOPS_DIR/claims/hb:x/holder.json"
done
# the lock wait budget is honoured (LONGOPS_LOCK_WAIT_S): a held purpose lock makes a script exit 70 after that budget, not after a fixed 15 s
newfx n14; mkll; P=$LL; id=$("$S/register.sh" --purpose lw:x --owner t --pid "$P" --no-progress-s 600)
( flock 9; sleep 5 ) 9>"$LONGOPS_DIR/lw:x.lock" & LH=$!; sleep 0.5
t0=$(date +%s.%N); LONGOPS_LOCK_WAIT_S=1 "$S/heartbeat.sh" --op-id "$id" >/dev/null 2>&1; rc=$?; t1=$(date +%s.%N); wait "$LH"
assert_rc "N14 a held purpose lock makes a script exit 70 after LONGOPS_LOCK_WAIT_S" $rc 70
[ "$(echo "$t1 - $t0 < 3.0" | bc)" = 1 ] && ok "N14b it waited the configured 1 s, not the default 15 s" || bad "N14b waited $(echo "$t1 - $t0" | bc)s"
# the identity and suspended-run branches of the holder judgement (valid JSON, one mistyped field: unreadable, never dead)
newfx n13; mkll; A=$LL; "$S/acquire.sh" --purpose hs:x --run-id hr --pid "$A" >/dev/null; cp "$LONGOPS_DIR/claims/hs:x/holder.json" "$FXN/hgood"
for fld in '.start_time={"a":1}' '.start_time=null' '.start_time=""'; do
  jq -c "$fld" "$FXN/hgood" >"$LONGOPS_DIR/claims/hs:x/holder.json"
  "$S/holder.sh" hs:x >/dev/null 2>&1; assert_rc "N13 a process holder with a start_time that cannot identify a process ($fld) is refused (20), never none" $? 20
  "$S/reap.sh" --purpose hs:x >/dev/null 2>&1; assert_rc "N13b reap --purpose does not release it (20)" $? 20
done
cp "$FXN/hgood" "$LONGOPS_DIR/claims/hs:x/holder.json"; assert_eq "N13c control: the unmodified holder is live" "$(jq -r .status <<<"$("$S/holder.sh" hs:x)")" live
"$S/acquire.sh" --suspend hr --purpose hs:x --builds b1 >/dev/null; cp "$LONGOPS_DIR/claims/hs:x/holder.json" "$FXN/sgood"
for fld in '.builds="abc"' '.builds=[1]' 'del(.state)' '.callback_state=7'; do
  jq -c "$fld" "$FXN/sgood" >"$LONGOPS_DIR/claims/hs:x/holder.json"
  "$S/holder.sh" hs:x >/dev/null 2>&1; assert_rc "N13d a suspended-run holder with ONE mistyped field ($fld) is refused (20), never none (dead)" $? 20
done
cp "$FXN/sgood" "$LONGOPS_DIR/claims/hs:x/holder.json"; assert_eq "N13e control: the unmodified suspended-run holder is live (build b1 not terminal)" "$(jq -r .status <<<"$("$S/holder.sh" hs:x)")" live
unset LONGOPS_NOW

finish
