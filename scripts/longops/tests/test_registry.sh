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
n=$(ls "$LONGOPS_DIR/ops" | grep -c json); assert_eq "R2c the refused register wrote no op record" "$n" 1
newfx r3; kill -0 1 2>/dev/null; mkll; D=$LL
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
"$S/reap.sh" --op-id "$id" >/dev/null 2>&1; assert_rc "P5 a recycled pid (start time differs) is dead_owner: reaped" $? 0
kill -0 "$P" 2>/dev/null && ok "P5b the recycled-pid process is NOT signalled" || bad "P5b it was killed"
[ ! -s "$LONGOPS_DIR/signals.log" ] && ok "P5c no signal was recorded" || bad "P5c"
# pgid/pid <= 1
( . "$S/lib.sh"; LD=$FXN/sig; mkdir -p "$LD"; rcs=""; for t in 1 0 -1 abc ""; do lo_signal TERM "$t" >/dev/null 2>&1; rcs="$rcs$? "; done; echo "$rcs" ) >"$FXN/o"
assert_eq "P6 lo_signal refuses pid 1, 0, -1, junk and empty (rc 7 each)" "$(cat "$FXN/o")" "7 7 7 7 7 "
[ ! -e "$FXN/sig/signals.log" ] && ok "P6b no signal audit entry exists for any refused target" || bad "P6b"
newfx p4; id=$("$S/register.sh" --purpose p4 --owner t --pid 1 --no-progress-s 1 2>/dev/null); export LONGOPS_NOW=9999
r=$("$S/classify.sh" --op-id "$id" | cut -f2); assert_eq "P7 an op naming pid 1 is never live: dead_owner" "$r" dead_owner
"$S/reap.sh" --op-id "$id" >/dev/null 2>&1; assert_rc "P7b reaping it sends nothing and succeeds as dead_owner" $? 0
[ ! -s "$LONGOPS_DIR/signals.log" ] && ok "P7c no signal sent to pid 1" || bad "P7c"
# container label resolution (a unit-level stand-in for podman; the sweep runs against the real one)
newfx p5; mkll; P=$LL; export LONGOPS_NOW=4000
cat >"$FXN/podman" <<'PE'
#!/usr/bin/env bash
echo "$*" >>"$PODLOG"
[ "$1" = ps ] && echo cid1234
exit 0
PE
chmod +x "$FXN/podman"; export LONGOPS_PODMAN=$FXN/podman PODLOG=$FXN/podlog
id=$("$S/register.sh" --purpose p5 --owner t --pid "$P" --no-progress-s 5); export LONGOPS_NOW=4100
"$S/reap.sh" --op-id "$id" >/dev/null 2>&1; assert_rc "P8 hung op with a labelled container is reaped" $? 0
grep -q "ps --filter label=op_id=$id" "$FXN/podlog" && ok "P8b the container is resolved by label op_id=<id>" || bad "P8b [$(cat "$FXN/podlog" 2>/dev/null)]"
grep -q 'stop -t 5 cid1234' "$FXN/podlog" && ok "P8c the resolved container is stopped" || bad "P8c"
unset LONGOPS_PODMAN
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
kill "$C"; wait "$C" 2>/dev/null; "$S/check_no_build_writing_tracked.sh" >"$FXN/o"; assert_rc "K8 a dead-owner row is not a writer (it never wedges the window)" $? 0
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

finish
