#!/usr/bin/env bash
# test_fix_r1.sh - WF10 review fix round 1 of the register operations (F1..F14 of WF10-REVIEW-register-ops.md). Written FIRST: every section
# reproduces one reviewed defect against the committed scripts (RED) and must pass on the fixed ones (GREEN).
# Env: SUT_DIR (directory holding the six scripts under test, default scripts/register; the RED run points it at a copy of commit e7a6a9b9, together
# with LOCKED, BACKUP, DUMP, EXPORT, REPLAY, RECONCILE), ONLY="F3 F4" (sections to run). Container sections use the real RUNP + IMG-TESTUTIL (clib.sh);
# the sections that need no database engine use a stub RUNP (the documented LOCKED_RUNP test hook) and fake repository trees.
. "$(dirname "$0")/lib.sh"; . "$(dirname "$0")/clib.sh"
evhead FIX-R1
SUT_DIR=${SUT_DIR:-$REG_DIR}
RECONCILE=${RECONCILE:-$SUT_DIR/reconcile.sh}
ONLY=" ${ONLY:-F1 F2 F3 F4 F5 F6 F7 F8 F9 F10 F11 F12 F13 F14 M6} "
want() { case "$ONLY" in *" $1 "*) return 0;; esac; return 1; }
echo "# sut_dir=$SUT_DIR sha256 locked=$(fsha "$LOCKED" | cut -c1-16) backup=$(fsha "$BACKUP" | cut -c1-16) dump=$(fsha "$DUMP" | cut -c1-16) export=$(fsha "$EXPORT" | cut -c1-16) reconcile=$(fsha "$RECONCILE" | cut -c1-16) replay=$(fsha "$REPLAY" | cut -c1-16)"

# ---------- stub machinery (no container) ----------
STUB_DIR="$T_SCR/stub"; export STUB_DIR; mkdir -p "$STUB_DIR"
STUBRUNP="$T_SCR/stub_runp.sh"
cat >"$STUBRUNP" <<'S'
#!/usr/bin/env bash
# stub RUNP: logs argv; snapshot calls (op-id *-snap) answer from $STUB_DIR/snap.<n> (.fail = exit 1); the main call execs $STUB_DIR/main.sh
op=""; a=("$@"); while [ $# -gt 0 ]; do case "$1" in --op-id) op="$2"; shift 2;; --) shift; break;; *) shift;; esac; done
echo "$(date +%s.%N) pid=$$ op=$op argv=${a[*]}" >> "$STUB_DIR/runp.log"
case "$op" in
  *-snap) n=$(cat "$STUB_DIR/snapn" 2>/dev/null || echo 0); n=$((n+1)); echo $n >"$STUB_DIR/snapn"; f="$STUB_DIR/snap.$n"
          [ -f "$f.fail" ] && exit 1; [ -f "$f" ] && cat "$f"; exit 0;;
  *) [ -x "$STUB_DIR/main.sh" ] && exec "$STUB_DIR/main.sh" "$@"; exit 0;;
esac
S
chmod +x "$STUBRUNP"
sreset() { rm -rf "$STUB_DIR"; mkdir -p "$STUB_DIR"; }
sroot() { local r; r=$(mkroot "$1"); echo "stub db" >"$r/docs/workable_items.db"; echo "$r"; }
slk() { LK_RUNP="$STUBRUNP" lk "$@"; }
jfield() { python3 -I -c 'import json,sys
rows=[json.loads(l) for l in open(sys.argv[1]) if l.strip()]
print(len(rows)) if sys.argv[2]=="n" else print(json.dumps(rows[int(sys.argv[2])].get(sys.argv[3])))' "$@"; }
ftree() {  # fake repository tree: the scripts under test copied under its scripts/register, a stub RUNP, NO test mode (REAL_ROOT is the tree)
  local t="$T_SCR/ft_$1"; mkdir -p "$t/scripts/register" "$t/scripts/containers" "$t/docs" "$t/.audit" "$t/build/containers"
  # the scripts under test come from the substitution variables (the mutation runner swaps one of them); the rest from SUT_DIR
  cp "$LOCKED" "$t/scripts/register/locked.sh"; cp "$BACKUP" "$t/scripts/register/backup_db.sh"; cp "$DUMP" "$t/scripts/register/dump.sh"; cp "$EXPORT" "$t/scripts/register/export.sh"; cp "$RECONCILE" "$t/scripts/register/reconcile.sh"; cp "$REPLAY" "$t/scripts/register/replay.sh"
  cp "$STUBRUNP" "$t/scripts/containers/run_pinned.sh"; cp "$ROOT/build/containers/images.lock.yaml" "$t/build/containers/"; echo "stub db" >"$t/docs/workable_items.db"; echo "$t"; }
killpid() { case "${1:-}" in ''|*[!0-9]*) return 0;; esac; [ "$1" -gt 1 ] && kill "$1" 2>/dev/null; return 0; }

# ================= F3: SIGTERM keeps the register lock until the writer is gone =================
if want F3; then echo "== F3 signal handling of locked.sh =="
  sreset; R=$(sroot f3); LCK="$R/docs/.register.lock"
  cat >"$STUB_DIR/main.sh" <<'S'
#!/usr/bin/env bash
echo $$ > "$STUB_DIR/main.pid"
echo "s $(date +%s%N) $$" >> "$STUB_DIR/ov.log"
sleep "${MAIN_SLEEP:-6}" & SL=$!
trap 'kill $SL 2>/dev/null; sleep 1; echo "e $(date +%s%N) $$ term" >> "$STUB_DIR/ov.log"; exit 143' TERM
wait $SL
echo "e $(date +%s%N) $$ done" >> "$STUB_DIR/ov.log"
S
  chmod +x "$STUB_DIR/main.sh"
  env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED_RUNP="$STUBRUNP" bash "$LOCKED" --op-id f3-a -- sh -c 'echo write' >"$T_SCR/f3a.out" 2>&1 &
  A=$!
  for i in $(seq 1 100); do [ -s "$STUB_DIR/main.pid" ] && break; sleep 0.1; done
  W=$(cat "$STUB_DIR/main.pid" 2>/dev/null)
  if tr '\0' ' ' </proc/$A/cmdline 2>/dev/null | grep -q 'locked.sh' && [ "$A" -gt 1 ] && [ -n "$W" ] && [ "$W" -gt 1 ]; then ok "F3-0 fixture: exact locked.sh pid $A proven ours via /proc, writer pid $W running"; else bad "F3-0 fixture (pid=$A writer=$W)"; fi
  kill -TERM "$A"
  # second caller right after the signal: it must not enter while the first writer is alive
  ( env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED_RUNP="$STUBRUNP" MAIN_SLEEP=1 bash "$LOCKED" --op-id f3-b -- sh -c 'echo second' >"$T_SCR/f3b.out" 2>&1; echo $? >"$T_SCR/f3b.rc" ) &
  free_while_alive=0; seen_held=0
  for i in $(seq 1 120); do
    kill -0 "$W" 2>/dev/null || break      # sample the lock for as long as the writer lives, whether or not locked.sh is still there
    if flock -n "$LCK" true 2>/dev/null; then free_while_alive=1; else seen_held=1; fi
    sleep 0.1
  done
  wait "$A"; arc=$?; wait
  assert_eq "F3-1 the register lock was never free while the killed call's writer was still running" "$free_while_alive" 0
  kill -0 "$W" 2>/dev/null && bad "F3-2 the writer is still running after locked.sh exited" || ok "F3-2 the writer is gone when locked.sh has exited"
  assert_eq "F3-3 locked.sh reports the interruption (exit 143 = 128+SIGTERM)" "$arc" 143
  n=$(jfield "$R/.audit/register/journal.jsonl" n 2>/dev/null)
  python3 -I - "$R/.audit/register/journal.jsonl" <<'PY' && ok "F3-4 the interrupted write has its journal row: op f3-a, interrupted=TERM, the writer's real exit 143" || bad "F3-4 journal: $(cat "$R/.audit/register/journal.jsonl" 2>/dev/null | head -c 500)"
import json,sys
rows=[json.loads(l) for l in open(sys.argv[1]) if l.strip()]
r=[x for x in rows if x["op_id"]=="f3-a"]; assert len(r)==1, rows
assert r[0].get("interrupted")=="TERM" and r[0]["exit"]==143, r[0]
PY
  python3 -I - "$STUB_DIR/ov.log" <<'PY' && ok "F3-5 the two writers did not overlap (first ended before the second started)" || bad "F3-5 overlap: $(tr '\n' ' ' <"$STUB_DIR/ov.log")"
import sys
ev=[l.split() for l in open(sys.argv[1]) if l.strip()]
assert [e[0] for e in ev]==['s','e','s','e'], [e[0] for e in ev]
PY
  [ -e "$R/.audit/register/pending/f3-a" ] && bad "F3-6 a pending marker is left for a journaled op" || ok "F3-6 no pending marker is left once the interrupted op is journaled"
  assert_eq "F3-7 the second caller completed normally" "$(cat "$T_SCR/f3b.rc" 2>/dev/null)" 0
fi

# ================= F4: snapshot failure is not an empty snapshot =================
if want F4; then echo "== F4 id snapshot pipeline status =="
  snapcase() {  # snapcase NAME <before-file-content|FAIL> <after-file-content|FAIL>
    sreset; local r; r=$(sroot "$1"); local b="$2" a="$3"
    if [ "$b" = FAIL ]; then : >"$STUB_DIR/snap.1.fail"; else printf '%b' "$b" >"$STUB_DIR/snap.1"; fi
    if [ "$a" = FAIL ]; then : >"$STUB_DIR/snap.2.fail"; else printf '%b' "$a" >"$STUB_DIR/snap.2"; fi
    slk "$r" --op-id "$1" -- sh -c 'true' >/dev/null 2>&1; echo "$r"; }
  r=$(snapcase f4c 'CAT-001 CAT-002\n' 'CAT-001 CAT-002 CAT-003\n')
  assert_eq "F4-1 control: both snapshots answer: ids_snapshot ok, CAT-003 minted" "$(jfield "$r/.audit/register/journal.jsonl" 0 ids_snapshot)$(jfield "$r/.audit/register/journal.jsonl" 0 ids_minted)" '"ok"["CAT-003"]'
  r=$(snapcase f4a 'CAT-001 CAT-002\n' FAIL)
  assert_eq "F4-2 the AFTER snapshot fails: ids_snapshot is failed (never ok with no mint)" "$(jfield "$r/.audit/register/journal.jsonl" 0 ids_snapshot)" '"failed"'
  r=$(snapcase f4b FAIL 'CAT-001 CAT-002 CAT-003\n')
  assert_eq "F4-3 the BEFORE snapshot fails: ids_snapshot is failed (no false mint of every id)" "$(jfield "$r/.audit/register/journal.jsonl" 0 ids_snapshot)$(jfield "$r/.audit/register/journal.jsonl" 0 ids_minted)" '"failed"[]'
  r=$(snapcase f4d '\n' '\n')
  assert_eq "F4-4 golden-FALSE: an empty table (both snapshots answer an empty row) is ok with no mint" "$(jfield "$r/.audit/register/journal.jsonl" 0 ids_snapshot)$(jfield "$r/.audit/register/journal.jsonl" 0 ids_minted)" '"ok"[]'
fi

# ================= F5: a journal append failure is loud =================
if want F5; then echo "== F5 journal append failure =="
  sreset; R=$(sroot f5a); mkdir -p "$R/.audit/register/journal.jsonl"
  printf '#!/bin/sh\ntouch docs/wrote\n' >"$STUB_DIR/main.sh"; chmod +x "$STUB_DIR/main.sh"
  out=$(slk "$R" --op-id f5a -- sh -c 'w' 2>&1); rc=$?
  [ $rc -ne 0 ] && [ ! -e "$R/docs/wrote" ] && printf '%s' "$out" | grep -q 'reason=journal_unwritable' && ok "F5-1 an unwritable journal refuses the call before the writer runs (journal_unwritable)" || bad "F5-1 rc=$rc wrote=$([ -e "$R/docs/wrote" ] && echo yes || echo no) [$out]"
  sreset; R=$(sroot f5b)
  printf '#!/bin/sh\ntouch docs/wrote\nrm -f .audit/register/journal.jsonl; mkdir .audit/register/journal.jsonl\n' >"$STUB_DIR/main.sh"; chmod +x "$STUB_DIR/main.sh"
  out=$(slk "$R" --op-id f5b -- sh -c 'w' 2>&1); rc=$?
  [ $rc -eq 21 ] && printf '%s' "$out" | grep -q 'reason=journal_append_failed' && ok "F5-2 a journal append that fails after the write exits 21 with reason journal_append_failed" || bad "F5-2 rc=$rc [$out]"
  [ -e "$R/.audit/register/pending/f5b" ] && ok "F5-3 the unjournaled write is detectable: its pending marker is left" || bad "F5-3 no pending marker"
  sreset; R=$(sroot f5c); printf '#!/bin/sh\ntrue\n' >"$STUB_DIR/main.sh"; chmod +x "$STUB_DIR/main.sh"
  out=$(slk "$R" --op-id f5c -- sh -c 'w' 2>&1); rc=$?
  [ $rc -eq 0 ] && [ ! -e "$R/.audit/register/pending/f5c" ] && [ "$(jfield "$R/.audit/register/journal.jsonl" n)" = 1 ] && ok "F5-4 golden-true: a normal call exits 0, journals one row and leaves no pending marker" || bad "F5-4 rc=$rc [$out]"
fi

# ================= F6: LOCKED_LOCK_HELD is a proven claim, not a switch =================
if want F6; then echo "== F6 LOCKED_LOCK_HELD =="
  sreset; R=$(sroot f6); LCK="$R/docs/.register.lock"
  flock "$LCK" sleep 8 & HOLDER=$!; sleep 0.5
  flock -n "$LCK" true 2>/dev/null && bad "F6-0 control needle: the lock is NOT held by the other process" || ok "F6-0 control needle: another process holds the register lock"
  t0=$(date +%s%N); out=$(LOCKED_LOCK_HELD=1 slk "$R" --op-id f6a -- true 2>&1); rc=$?; ms=$(( ($(date +%s%N)-t0)/1000000 ))
  [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'reason=lock_claim_invalid' && [ "$(jfield "$R/.audit/register/journal.jsonl" n 2>/dev/null || echo 0)" = 0 ] && ok "F6-1 LOCKED_LOCK_HELD=1 without a verified fd 9 is refused lock_claim_invalid (${ms} ms), no journal row" || bad "F6-1 rc=$rc ${ms}ms rows=$(jfield "$R/.audit/register/journal.jsonl" n 2>/dev/null) [$out]"
  kill "$HOLDER" 2>/dev/null; wait "$HOLDER" 2>/dev/null
  FT=$(ftree f6); flock "$FT/docs/.register.lock" sleep 6 & HOLDER=$!; sleep 0.5
  out=$(LOCKED_LOCK_HELD=1 bash "$FT/scripts/register/locked.sh" --op-id f6b -- true 2>&1); rc=$?
  [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'reason=lock_claim_invalid' && ok "F6-2 outside test mode (real tree layout) the forged claim is refused too" || bad "F6-2 rc=$rc [$out]"
  kill "$HOLDER" 2>/dev/null; wait "$HOLDER" 2>/dev/null
  # golden-true: the production nesting (import-sql register -> backup helper -> locked.sh) inherits fd 9 and is accepted
  sreset; R=$(sroot f6n); mkdir -p "$R/.audit/out/op1"; printf 'select 1;\n' >"$R/.audit/out/op1/x.sql"; sha256sum <"$R/.audit/out/op1/x.sql" | cut -d' ' -f1 >"$R/.audit/out/op1/x.sha256"
  printf '#!/bin/sh\ntimeout 30 bash "%s" --op-id nested-f6 -- true || exit 9\n' "$LOCKED" >"$T_SCR/nested_backup.sh"; chmod +x "$T_SCR/nested_backup.sh"
  out=$(LK_BACKUP="$T_SCR/nested_backup.sh" slk "$R" --op-id f6parent import-sql .audit/out/op1/x.sql 2>&1); rc=$?
  [ $rc -eq 0 ] && [ "$(jfield "$R/.audit/register/journal.jsonl" n)" = 2 ] && ok "F6-3 golden-true: a locked.sh nested under a lock-holding locked.sh (fd 9 inherited) is accepted and journaled" || bad "F6-3 rc=$rc rows=$(jfield "$R/.audit/register/journal.jsonl" n 2>/dev/null) [$out]"
fi

# ================= F13: fd 9 is not inherited by the reaper; the reaper is bounded =================
if want F13; then echo "== F13 reaper =="
  sreset; R=$(sroot f13); LCK="$R/docs/.register.lock"
  sleep 300 & HOLD=$!; echo "{\"run_id\":\"OTHER\",\"pid\":$HOLD}" >"$R/.audit/commit_turn.json"
  printf '#!/bin/sh\nsleep 8 &\necho $! >"%s/child.pid"\nexit 0\n' "$STUB_DIR" >"$STUB_DIR/reaper.sh"; chmod +x "$STUB_DIR/reaper.sh"
  out=$(CPA_HOST_ENTRY="$STUB_DIR/reaper.sh" slk "$R" --op-id f13a -- true 2>&1); rc=$?
  flock -n "$LCK" true 2>/dev/null && free=1 || free=0
  [ $rc -eq 20 ] && printf '%s' "$out" | grep -q commit_turn_held && ok "F13-0 fixture: the grant stays held, locked.sh refuses commit_turn_held" || bad "F13-0 rc=$rc [$out]"
  assert_eq "F13-1 the register lock is free as soon as locked.sh exits (a child the reaper left behind does not hold it)" "$free" 1
  killpid "$(cat "$STUB_DIR/child.pid" 2>/dev/null)"
  printf '#!/bin/sh\necho $$ >"%s/hang.pid"\nexec sleep 300\n' "$STUB_DIR" >"$STUB_DIR/reaper.sh"
  t0=$(date +%s); out=$(CPA_HOST_ENTRY="$STUB_DIR/reaper.sh" LOCKED_REAPER_TIMEOUT=2 timeout 40 env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED_RUNP="$STUBRUNP" bash "$LOCKED" --op-id f13b -- true 2>&1); rc=$?; dt=$(( $(date +%s)-t0 ))
  [ $rc -eq 20 ] && [ $dt -lt 20 ] && ok "F13-2 a hanging reaper is bounded by LOCKED_REAPER_TIMEOUT (refused after ${dt}s, not hung)" || bad "F13-2 rc=$rc dt=${dt}s [$out]"
  killpid "$(cat "$STUB_DIR/hang.pid" 2>/dev/null)"; killpid "$HOLD"; wait 2>/dev/null
fi

# ================= F8: LOCKED hooks and output verification =================
if want F8; then echo "== F8 LOCKED hook outside test mode, output verification =="
  FT=$(ftree f8); FOREIGN="$T_SCR/foreign_locked.sh"; FL="$T_SCR/foreign.log"; printf '#!/bin/sh\necho "$*" >>"%s"\nexit 0\n' "$FL" >"$FOREIGN"; chmod +x "$FOREIGN"; : >"$FL"
  sreset
  env LOCKED="$FOREIGN" bash "$FT/scripts/register/backup_db.sh" --record "$T_SCR/f8.rec" >/dev/null 2>&1
  env LOCKED="$FOREIGN" bash "$FT/scripts/register/dump.sh" >"$T_SCR/f8.dump.out" 2>&1; drc=$?
  env LOCKED="$FOREIGN" bash "$FT/scripts/register/export.sh" >"$T_SCR/f8.exp.out" 2>&1
  env LOCKED="$FOREIGN" bash "$FT/scripts/register/export.sh" --check >/dev/null 2>&1
  assert_eq "F8-1 outside test mode the LOCKED override is ignored by backup_db.sh, dump.sh and export.sh (the foreign wrapper was called 0 times)" "$(wc -l <"$FL")" 0
  # test mode: a wrapper that exits 0 and produces nothing
  NOOP="$T_SCR/noop_locked.sh"; printf '#!/bin/sh\nexit 0\n' >"$NOOP"; chmod +x "$NOOP"; R=$(mkroot f8r)
  out=$( LOCKED="$NOOP"; tool "$R" "$DUMP" --db docs/s.db --out docs/register/s.sql 2>&1); rc=$?
  [ $rc -ne 0 ] && ! printf '%s' "$out" | grep -q '^dump: OK' && [ ! -e "$R/docs/register/s.sql" ] && ok "F8-2 dump.sh does not report OK when the wrapper produced no dump (rc $rc)" || bad "F8-2 rc=$rc [$out]"
  # a COMPLETE stale dump from an earlier run is not the output of this run: the freshness check (not only the content check) must refuse
  mkdir -p "$R/docs/register"; printf 'BEGIN TRANSACTION;\nCOMMIT;\n' >"$R/docs/register/stale.sql"; touch -d '2020-01-01' "$R/docs/register/stale.sql"
  out=$( LOCKED="$NOOP"; tool "$R" "$DUMP" --db docs/s.db --out docs/register/stale.sql 2>&1); rc=$?
  [ $rc -ne 0 ] && ! printf '%s' "$out" | grep -q '^dump: OK' && ok "F8-2b a stale complete dump left by an earlier run is not accepted as this run's output (rc $rc)" || bad "F8-2b rc=$rc [$out]"
  # same for export: every expected output and a manifest already on disk, wrapper produced nothing
  EDIR="$R/docs/register"; for n in Issues Fixed Issues_Summary Fixed_Summary Reconciliation; do echo "x" >"$EDIR/$n.md"; done; for n in reconciliation unmapped_entries legacy_id_collisions reverify_queue reopen_counts stale_tracker_sync findings; do echo "h" >"$EDIR/$n.csv"; done; echo "x" >"$EDIR/export-manifest.sha256"; touch -d '2020-01-01' "$EDIR"/*
  out=$( LOCKED="$NOOP"; tool "$R" "$EXPORT" --db docs/s.db --out-dir docs/register 2>&1); rc=$?
  [ $rc -ne 0 ] && printf '%s' "$out" | grep -q 'not written by this run' && ok "F8-3b stale outputs and manifest from an earlier run are not accepted (rc $rc)" || bad "F8-3b rc=$rc [$out]"
  out=$( LOCKED="$NOOP"; tool "$R" "$EXPORT" --db docs/s.db --out-dir docs/register 2>&1); rc=$?
  [ $rc -ne 0 ] && ! printf '%s' "$out" | grep -q '^export: OK' && ok "F8-3 export.sh does not succeed when the wrapper produced no export (rc $rc)" || bad "F8-3 rc=$rc [$out]"
  # replay.sh: outside test mode none of its LOCKED/BACKUP/DUMP/EXPORT overrides may be called (a valid journal takes it as far as the backup step)
  mkdir -p "$T_SCR/f8j"; python3 -I -c 'import json;a="a"*64;print(json.dumps({"op_id":"W1","mode":"register","db":"docs/workable_items.db","db_sha_before":None,"db_sha_after":a,"ids_minted":[],"argv":["true"],"exit":0,"input_args":{},"inputs":[],"wal_bytes_after":0,"ids_snapshot":"ok"}))' >"$T_SCR/f8j/journal.jsonl"
  : >"$FL"; env LOCKED="$FOREIGN" BACKUP="$FOREIGN" DUMP="$FOREIGN" EXPORT="$FOREIGN" bash "$FT/scripts/register/replay.sh" --onto docs/workable_items.db --since "$(printf 'a%.0s' $(seq 64))" --out "$T_SCR/f8out" --journal "$T_SCR/f8j/journal.jsonl" >"$T_SCR/f8.replay.out" 2>&1
  assert_eq "F8-5 outside test mode replay.sh ignores the LOCKED/BACKUP/DUMP/EXPORT overrides (the foreign script was called 0 times)" "$(wc -l <"$FL")" 0
  out=$( LOCKED="$NOOP"; tool "$R" "$EXPORT" --check --db docs/s.db --out-dir docs/register 2>&1); rc=$?
  [ $rc -ne 0 ] && ! printf '%s' "$out" | grep -q '^export-check: OK' && ok "F8-4 export.sh --check does not say OK when the wrapper printed no verdict (rc $rc)" || bad "F8-4 rc=$rc [$out]"
fi

# ================= F2 / F1 / journal safety: replay planning on synthetic journals (no container) =================
if want F1 || want F2; then echo "== F1/F2 replay planning (stub tools) =="
  H="$T_SCR/rh"; mkdir -p "$H/root/docs" "$H/stubs"; echo "x" >"$H/root/docs/workable_items.db"
  cat >"$H/stubs/backup.sh" <<'S'
#!/usr/bin/env bash
cp docs/workable_items.db docs/bak; s=$(sha256sum docs/bak | cut -d' ' -f1); printf '{"backup_path":"docs/bak","backup_sha256":"%s","source_sha256":"%s"}' "$s" "$s" > "$2"
S
  cat >"$H/stubs/locked.sh" <<'S'
#!/usr/bin/env bash
echo "$*" >> "$HARNESS_LOG"
case "$*" in *"select atm_id from reg_ids"*) n=$(cat "$HARNESS_IDS.n" 2>/dev/null || echo 0); echo $((n+1)) >"$HARNESS_IDS.n"; if [ "$n" = 0 ]; then cat "$HARNESS_IDS"; else cat "$HARNESS_IDS.final"; fi;; *gate.sh*) echo "GATE OK";; esac; exit 0
S
  printf '#!/bin/sh\nexit 0\n' >"$H/stubs/noop.sh"; chmod +x "$H/stubs/"*.sh
  A=$(printf 'a%.0s' $(seq 64)); B=$(printf 'b%.0s' $(seq 64)); C=$(printf 'c%.0s' $(seq 64))
  mkj() {  # mkj NAME : journals into $H/j/NAME/journal.jsonl from the python spec on stdin
    mkdir -p "$H/j/$1"; python3 -I -c '
import json,sys
a,b,c=sys.argv[2:5]
def row(op,mode,db,bs,as_,minted,argv,ex=0,snap="ok",wal=0,seq=None):
    r={"op_id":op,"mode":mode,"db":db,"db_sha_before":bs,"db_sha_after":as_,"ids_minted":minted,"argv":argv,"exit":ex,"input_args":{},"inputs":[],"wal_bytes_after":wal,"ids_snapshot":snap}
    if seq is not None: r["seq"]=seq
    return r
REG="docs/workable_items.db"; M=["sqlite3","/src/"+REG,"MINT"]
rows=eval(sys.stdin.read())
open(sys.argv[1]+"/journal.jsonl","w").write("".join((l if isinstance(l,str) else json.dumps(l))+"\n" for l in rows))' "$H/j/$1" "$A" "$B" "$C"; }
  rp() {  # rp NAME IDSFILE-CONTENT -> runs replay.sh against the stubs, prints "rc=.. <output>" ; out dir $H/out_NAME
    local n=$1 ids=$2 fin="${3:-CAT-001 CAT-002}"; printf '%s\n' "$ids" >"$H/ids_$n.txt"; printf '%s\n' "$fin" >"$H/ids_$n.txt.final"; rm -f "$H/ids_$n.txt.n"; rm -rf "$H/out_$n"
    local o; o=$(env LOCKED_TEST_MODE=1 LOCKED_ROOT="$H/root" LOCKED="$H/stubs/locked.sh" BACKUP="$H/stubs/backup.sh" DUMP="$H/stubs/noop.sh" EXPORT="$H/stubs/noop.sh" HARNESS_LOG="$H/log_$n" HARNESS_IDS="$H/ids_$n.txt" bash "$REPLAY" --onto docs/workable_items.db --since "$A" --out "$H/out_$n" --journal "$H/j/$n/journal.jsonl" 2>&1); echo "rc=$? $o" | cut -c1-300; }
  if want F2; then
    mkj ctl <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("W2","register",REG,a,b,["CAT-002"],M)]
E
    r=$(rp ctl "CAT-001"); case "$r" in "rc=0 replay: OK replayed=1 "*) ok "F2-0 control: base W1, the local mint W2 is replayed";; *) bad "F2-0 [$r]";; esac
    mkj scr <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("W2","register",REG,a,b,["CAT-002"],M), row("X","scratch",".audit/scratch/copy.db",a,a,[],["sqlite3","/src/.audit/scratch/copy.db","IMPORT"],ex=1)]
E
    r=$(rp scr "CAT-001"); case "$r" in "rc=0 replay: OK replayed=1 "*) ok "F2-1 a later scratch-mode row whose hash equals the base hash is not a base candidate: W2 is still replayed";; *) bad "F2-1 [$r]";; esac
    mkj scr2 <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("W2","register",REG,a,b,["CAT-002"],M), row("X2","out",REG,a,a,[],["sqlite3","/src/"+REG,"READ"])]
E
    r=$(rp scr2 "CAT-001"); case "$r" in "rc=0 replay: OK replayed=1 "*) ok "F2-2 a later --out-mode row with the base hash is not a base candidate either";; *) bad "F2-2 [$r]";; esac
    mkj walrow <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("WALW","register",REG,a,a,[],["sqlite3","/src/"+REG,"PRAGMA user_version=7"],wal=4152), row("W3","register",REG,a,b,["CAT-002"],M)]
E
    r=$(rp walrow "CAT-001"); case "$r" in "rc=0 replay: OK replayed=2 "*) ok "F2-3 a write left pending in the -wal file (main-file sha unchanged) is not the base: it is replayed with the rest (2 rows)";; *) bad "F2-3 [$r]";; esac
    HX=$(printf 'select 1;\n' | sha256sum | cut -d' ' -f1)
    mkj inp <<E
[row("W1","register",REG,None,a,["CAT-001"],M), dict(row("W2","register",REG,a,b,["CAT-002"],["sqlite3","/src/"+REG,".read /src/.audit/x.sql"]), input_args={"2":"$HX"}, inputs=["$HX"])]
E
    mkdir -p "$H/j/inp/inputs"; printf 'select 1;\n' >"$H/j/inp/inputs/$HX"
    r=$(rp inp "CAT-001"); case "$r" in "rc=0 replay: OK replayed=1 "*) ok "F2-7 golden-true: a recorded input whose content hashes to its name is re-bound and replayed";; *) bad "F2-7 [$r]";; esac
    echo tampered >"$H/j/inp/inputs/$HX"
    r=$(rp inp "CAT-001"); case "$r" in "rc=20 "*"reason=input_missing"*) ok "F2-8 a recorded input whose content no longer hashes to its name is refused input_missing";; *) bad "F2-8 [$r]";; esac
    mkj seq <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M,seq=1), row("W2","register",REG,a,b,["CAT-002"],M,seq=3), row("W3","register",REG,b,c,[],M,seq=2)]
E
    r=$(rp seq "CAT-001"); case "$r" in "rc=20 "*"reason=journal_order_invalid"*) ok "F2-4 journal rows out of seq order are refused journal_order_invalid";; *) bad "F2-4 [$r]";; esac
    mkj corrupt <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), "{this is not json"]
E
    r=$(rp corrupt "CAT-001"); case "$r" in "rc=20 "*"reason=journal_corrupt"*) ok "F2-5 a corrupt journal line is a named refusal journal_corrupt (no traceback)";; *) bad "F2-5 [$r]";; esac
    mkj pend <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("W2","register",REG,a,b,["CAT-002"],M)]
E
    mkdir -p "$H/j/pend/pending"; : >"$H/j/pend/pending/killed-op"
    r=$(rp pend "CAT-001"); case "$r" in "rc=20 "*"reason=replay_pending_ops"*) ok "F2-6 a pending marker (a write that never reached the journal) is refused replay_pending_ops";; *) bad "F2-6 [$r]";; esac
  fi
  if want F1; then
    mkj unav <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("U","register",REG,a,c,[],M,snap="unavailable")]
E
    r=$(rp unav "CAT-001 CAT-002"); case "$r" in "rc=20 "*"reason=replay_ids_unknown"*) ok "F1-1 a planned register row whose ids_snapshot is unavailable is refused replay_ids_unknown (never replayed blind)";; *) bad "F1-1 [$r]";; esac
    [ ! -e "$H/out_unav" ] && ok "F1-2 the refusal removed the output directory" || bad "F1-2"
    mkj fail <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("U","register",REG,a,c,[],M,snap="failed")]
E
    r=$(rp fail "CAT-001"); case "$r" in "rc=20 "*"reason=replay_ids_unknown"*) ok "F1-3 ids_snapshot=failed is refused too";; *) bad "F1-3 [$r]";; esac
    mkj ctl <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("W2","register",REG,a,b,["CAT-002"],M)]
E
    r=$(rp ctl "CAT-001" "CAT-001"); case "$r" in "rc=20 "*"reason=replay_id_lost"*) ok "F1-5 a replayed database that lacks a local mint's id is refused replay_id_lost";; *) bad "F1-5 [$r]";; esac
    mkj nochg <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("R","register",REG,a,a,[],["sqlite3","-readonly","/src/"+REG,"select 1"],snap="unavailable")]
E
    r=$(rp nochg "CAT-001"); case "$r" in "rc=0 replay: OK replayed=0 "*) ok "F1-4 golden-FALSE: an unchanged-database row with an unavailable snapshot is a no_database_change skip, not a refusal";; *) bad "F1-4 [$r]";; esac
  fi
fi

# ================= F11: test isolation (suite level) =================
if want F11; then echo "== F11 test isolation =="
  case "${DISK_HEADROOM_OUT_DIR:-UNSET}" in "$T_SCR"/*) ok "F11-1 clib.sh routes the disk-headroom records into the scratch root ($DISK_HEADROOM_OUT_DIR)";; *) bad "F11-1 DISK_HEADROOM_OUT_DIR=${DISK_HEADROOM_OUT_DIR:-UNSET}";; esac
  R=$(mkroot f11); cdb "$R" s.db && lk "$R" -- sh -c "$(mint_cmd s.db)" >/dev/null 2>&1
  op=$(python3 -I -c 'import json,sys;print([json.loads(l)["op_id"] for l in open(sys.argv[1]) if l.strip()][-1])' "$R/.audit/register/journal.jsonl")
  [ -f "$DISK_HEADROOM_OUT_DIR/$op.json" ] && ok "F11-2 the disk-headroom record of a register call ($op) is in the scratch root" || bad "F11-2 no record $DISK_HEADROOM_OUT_DIR/$op.json"
  [ ! -e "$ROOT/specs/001-full-project-audit-remediation/evidence/disk/$op.json" ] && ok "F11-3 and not in the real repository's evidence/disk" || bad "F11-3 leaked into the real repository"
  tool "$R" "$EXPORT" --db docs/s.db --out-dir docs/register >/dev/null 2>&1
  tool "$R" "$EXPORT" --check --db docs/s.db --out-dir docs/register >/dev/null 2>&1
  ls -d "$R"/.audit/out/export-check-* >/dev/null 2>&1 && ok "F11-4 export.sh --check works under the scratch root's .audit/out" || bad "F11-4 no export-check directory in the scratch root"
  n=$(find "$ROOT/.audit/out" -maxdepth 1 -name 'export-check-*' -newer "$RT_MARK" 2>/dev/null | wc -l)
  assert_eq "F11-5 no export-check directory was created under the real repository's .audit/out during this run" "$n" 0
fi

# ================= F12: reconcile on empty views =================
if want F12; then echo "== F12 reconcile empty views =="
  DBF="$T_SCR/f12.db"; fresh_ext_db "$DBF" || bad "F12 setup"
  OD="$T_SCR/f12out"; out=$(bash "$RECONCILE" --db "$DBF" --out-dir "$OD" 2>&1); rc=$?
  [ $rc -eq 0 ] && ok "F12-0 reconcile succeeds on a register database with empty views" || bad "F12-0 rc=$rc [$out]"
  bad_csv=0; for f in "$OD"/*.csv; do [ -s "$f" ] || bad_csv=$((bad_csv+1)); done
  assert_eq "F12-1 every CSV of an empty view carries its header line (no 0-byte CSV)" "$bad_csv" 0
  grep -q '^Rows: -1' "$OD/Reconciliation.md" && bad "F12-2 'Rows: -1' in Reconciliation.md" || ok "F12-2 no 'Rows: -1'"
  assert_eq "F12-3 every empty view reads Rows: 0 (7 sections)" "$(grep -c '^Rows: 0$' "$OD/Reconciliation.md")" 7
  grep -q '^| kind | source ' "$OD/Reconciliation.md" && ok "F12-4 an empty view still shows its Markdown table header" || bad "F12-4"
  head -1 "$OD/reverify_queue.csv" | grep -q '^atm_id,legacy_status,status,severity$' && ok "F12-5 the empty CSV header is the column list" || bad "F12-5 [$(head -1 "$OD/reverify_queue.csv")]"
fi

# ================= F10: a busy checkpoint is refused, never a stale dump =================
if want F10; then echo "== F10 dump with a busy checkpoint =="
  R=$(mkroot f10); cdb "$R" s.db || bad setup; lk "$R" -- sh -c "$(mint_cmd s.db)" >/dev/null 2>&1
  python3 -I - "$R/docs/s.db" >"$T_SCR/f10.reader" <<'P' &
import sqlite3,sys,time
c=sqlite3.connect("file:"+sys.argv[1]+"?mode=ro",uri=True,isolation_level=None); c.execute("BEGIN"); n=c.execute("select count(*) from reg_ids").fetchone()[0]
print("reader holds %d reg_ids rows"%n, flush=True); time.sleep(45); c.execute("COMMIT"); c.close()
P
  RP=$!; sleep 2
  NEW=$(lk "$R" -- sh -c "$(mint_cmd s.db)" 2>/dev/null | tail -1)
  rm -f "$R/docs/register/s.sql"
  out=$(tool "$R" "$DUMP" --db docs/s.db --out docs/register/s.sql 2>&1); rc=$?
  [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'reason=checkpoint_incomplete' && [ ! -s "$R/docs/register/s.sql" ] && ok "F10-1 a checkpoint that cannot complete (reader open) is refused checkpoint_incomplete (20), no dump written (minted $NEW)" || bad "F10-1 rc=$rc [$out] dump=$([ -s "$R/docs/register/s.sql" ] && grep -c "$NEW" "$R/docs/register/s.sql")"
  kill "$RP" 2>/dev/null; wait "$RP" 2>/dev/null
  out=$(tool "$R" "$DUMP" --db docs/s.db --out docs/register/s.sql 2>&1); rc=$?
  [ $rc -eq 0 ] && grep -q "$NEW" "$R/docs/register/s.sql" && ok "F10-2 golden-true: with no reader the dump is written and holds $NEW" || bad "F10-2 rc=$rc [$out]"
fi

# ================= F9: --check against a tampered manifest =================
if want F9; then echo "== F9 export --check manifest =="
  R=$(mkroot f9); cdb "$R" s.db || bad setup; lk "$R" -- sh -c "$(mint_cmd s.db)" >/dev/null 2>&1
  D="$R/docs/register"; ex() { tool "$R" "$EXPORT" "$@" --db docs/s.db --out-dir docs/register 2>&1; }
  ex >/dev/null || bad "F9 export failed"; rm -rf "$T_SCR/f9bak"; cp -a "$D" "$T_SCR/f9bak"
  restore() { rm -rf "$D"; cp -a "$T_SCR/f9bak" "$D"; }
  out=$(ex --check); [ $? -eq 0 ] && ok "F9-0 golden-true: an untouched export checks OK" || bad "F9-0 [$out]"
  echo "TAMPERED,x" >>"$D/findings.csv"; : >"$D/export-manifest.sha256"; out=$(ex --check); rc=$?
  [ $rc -eq 1 ] && printf '%s' "$out" | grep -q STALE && ok "F9-1 CSV edited and manifest emptied: STALE" || bad "F9-1 rc=$rc [$out]"
  restore; echo "TAMPERED,x" >>"$D/findings.csv"; grep -v ' findings.csv$' "$T_SCR/f9bak/export-manifest.sha256" >"$D/export-manifest.sha256"; out=$(ex --check); rc=$?
  [ $rc -eq 1 ] && printf '%s' "$out" | grep -q STALE && ok "F9-2 CSV edited and its manifest line removed: STALE" || bad "F9-2 rc=$rc [$out]"
  restore; rm -f "$D/findings.csv"; grep -v ' findings.csv$' "$T_SCR/f9bak/export-manifest.sha256" >"$D/export-manifest.sha256"; out=$(ex --check); rc=$?
  [ $rc -eq 1 ] && printf '%s' "$out" | grep -q STALE && ok "F9-3 CSV deleted and its manifest line removed: STALE" || bad "F9-3 rc=$rc [$out]"
  restore; rm -f "$D/export-manifest.sha256"; out=$(ex --check); rc=$?
  [ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'manifest' && ok "F9-4 manifest missing: STALE" || bad "F9-4 rc=$rc [$out]"
  restore; echo "TAMPERED,x" >>"$D/findings.csv"; h=$(sha256sum "$D/findings.csv" | cut -d' ' -f1); sed -i "s/^[0-9a-f]\{64\} findings.csv\$/$h findings.csv/" "$D/export-manifest.sha256"; out=$(ex --check); rc=$?
  [ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'findings.csv' && ok "F9-5 CSV edited and its manifest line rewritten to match: STALE (the CSV is regenerated from the database and compared)" || bad "F9-5 rc=$rc [$out]"
  restore; : >"$D/export-manifest.sha256"; out=$(ex --check); rc=$?
  [ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'does not list exactly' && ok "F9-7 only the manifest emptied (every output untouched): STALE, the missing names reported" || bad "F9-7 rc=$rc [$out]"
  restore; out=$(ex --check); [ $? -eq 0 ] && ok "F9-6 golden-true again after restoring the export" || bad "F9-6 [$out]"
fi

# ================= F7 / F14: backup uniqueness and the source hash under the lock =================
if want F7 || want F14; then echo "== F7/F14 backup_db.sh =="
  R=$(mkroot f7); cdb "$R" workable_items.db || bad setup; for i in 1 2; do lk "$R" -- sh -c "$(mint_cmd workable_items.db)" >/dev/null 2>&1; done
  hb() { env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED_RUNP="$RUNP" LOCKED="${HB_LOCKED:-$LOCKED}" SHIM_LOG="$SHIM_LOG" PATH="$SHIM_DIR:$PATH" "$@" bash "$BACKUP" "${HBARGS[@]}"; }
  rec() { python3 -I -c 'import json,sys;print(json.load(open(sys.argv[1]))[sys.argv[2]])' "$@"; }
  if want F7; then
    python3 -I -c 'import time;t=time.time();time.sleep(1.05-(t-int(t)))'
    HBARGS=(--record "$T_SCR/a.json"); ( hb >"$T_SCR/a.out" 2>&1; echo $? >"$T_SCR/a.rc" ) &
    HBARGS=(--record "$T_SCR/b.json"); ( hb >"$T_SCR/b.out" 2>&1; echo $? >"$T_SCR/b.rc" ) &
    wait
    pa=$(rec "$T_SCR/a.json" backup_path 2>/dev/null); pb=$(rec "$T_SCR/b.json" backup_path 2>/dev/null)
    [ "$(cat "$T_SCR/a.rc")$(cat "$T_SCR/b.rc")" = 00 ] && [ -n "$pa" ] && [ "$pa" != "$pb" ] && [ -f "$R/$pa" ] && [ -f "$R/$pb" ] && ok "F7-1 two backups started together get two distinct backup files ($pa, $pb)" || bad "F7-1 rc=$(cat "$T_SCR/a.rc")$(cat "$T_SCR/b.rc") a=[$pa] b=[$pb]"
    for x in a b; do p=$(rec "$T_SCR/$x.json" backup_path 2>/dev/null); [ -n "$p" ] && [ "$(rec "$T_SCR/$x.json" backup_sha256)" = "$(fsha "$R/$p")" ] || bad "F7-2 record $x names a file that does not match its sha256"; done; ok "F7-2 each record's backup_sha256 equals its own file"
    python3 -I -c 'import time;t=time.time();time.sleep(1.05-(t-int(t)))'
    HBARGS=(--record "$T_SCR/g.json"); ( hb >"$T_SCR/g.out" 2>&1; echo $? >"$T_SCR/g.rc" ) &
    HBARGS=(--record "$T_SCR/f.json"); ( BACKUP_FAULT=truncate hb >"$T_SCR/f.out" 2>&1; echo $? >"$T_SCR/f.rc" ) &
    wait; pg=$(rec "$T_SCR/g.json" backup_path 2>/dev/null)
    [ "$(cat "$T_SCR/g.rc")" = 0 ] && [ -n "$pg" ] && [ -f "$R/$pg" ] && [ "$(rec "$T_SCR/g.json" backup_sha256)" = "$(fsha "$R/$pg")" ] && [ "$(cat "$T_SCR/f.rc")" != 0 ] && ok "F7-3 a failing backup started in the same second does not delete the good backup (fail() removes only its own file)" || bad "F7-3 g.rc=$(cat "$T_SCR/g.rc") f.rc=$(cat "$T_SCR/f.rc") pg=[$pg] exists=$([ -f "$R/$pg" ] && echo y || echo n)"
  fi
  if want F14; then
    HOOK="$T_SCR/hook14.sh"; MINTTXT="$(mint_cmd workable_items.db)"
    cat >"$HOOK" <<S
#!/usr/bin/env bash
bash "$LOCKED" "\$@"; rc=\$?
case "\$*" in *.backup*) if [ ! -e "$T_SCR/hook14.done" ]; then : >"$T_SCR/hook14.done"; sha256sum docs/workable_items.db | cut -d' ' -f1 >"$T_SCR/hook14.sha"; bash "$LOCKED" --op-id hook14-mint -- sh -c '$MINTTXT' >/dev/null 2>&1; fi;; esac
exit \$rc
S
    chmod +x "$HOOK"; rm -f "$T_SCR/hook14.sha" "$T_SCR/hook14.done"; HBARGS=(--record "$T_SCR/h.json"); HB_LOCKED="$HOOK" hb >"$T_SCR/h.out" 2>&1; hrc=$?
    [ $hrc -eq 0 ] && [ -s "$T_SCR/hook14.sha" ] && [ "$(rec "$T_SCR/h.json" source_sha256)" = "$(cat "$T_SCR/hook14.sha")" ] && ok "F14-1 source_sha256 is the hash of the source under the backup's lock, not of a later state (another writer changed it in between)" || bad "F14-1 rc=$hrc rec=$(rec "$T_SCR/h.json" source_sha256 2>/dev/null | cut -c1-16) at-lock=$(cut -c1-16 "$T_SCR/hook14.sha" 2>/dev/null) now=$(fsha "$R/docs/workable_items.db" | cut -c1-16) [$(tail -2 "$T_SCR/h.out")]"
  fi
fi

# ================= F1 / F2 end to end with real containers (the reviewer's pending-WAL probe) =================
if want F1 && [ -z "${NO_E2E:-}" ]; then echo "== F1/F2 end to end (real containers, pending -wal) =="
  BASE=$(mkroot base); cdb "$BASE" workable_items.db || bad "e2e setup"
  for i in 1 2; do lk "$BASE" -- sh -c "$(mint_cmd workable_items.db)" >/dev/null 2>&1 || { bad "setup mint"; finish; exit 1; }; done
  SINCE=$(tail -n1 "$BASE/.audit/register/journal.jsonl" | python3 -I -c 'import json,sys;print(json.load(sys.stdin)["db_sha_after"])')
  LOCAL="$T_SCR/local"; REMOTE2="$T_SCR/remote2"; cp -a "$BASE" "$LOCAL"; cp -a "$BASE" "$REMOTE2"; DB=docs/workable_items.db
  lk "$LOCAL" -- bash -c 'sqlite3 /src/docs/workable_items.db "select count(*) from reg_ids" ".shell sleep 8" >/dev/null 2>&1 & sleep 2; sqlite3 /src/docs/workable_items.db "PRAGMA user_version=7"; exit 0' >/dev/null 2>&1
  [ -s "$LOCAL/docs/workable_items.db-wal" ] && ok "E2E-0 fixture: a committed write is pending in the -wal file ($(stat -c %s "$LOCAL/docs/workable_items.db-wal") bytes)" || bad "E2E-0 fixture: no pending WAL"
  LID=$(lk "$LOCAL" -- sh -c "$(mint_cmd workable_items.db)" 2>/dev/null | tail -1)
  RID=$(lk "$REMOTE2" -- sh -c "$(mint_cmd workable_items.db)" 2>/dev/null | tail -1)
  lk "$REMOTE2" -- sqlite3 /src/docs/workable_items.db "UPDATE items SET title='REMOTE-ITEM' WHERE atm_id='$RID'" >/dev/null 2>&1
  O="$T_SCR/e2e_out"; out=$(tool "$REMOTE2" "$REPLAY" --onto $DB --since "$SINCE" --out "$O" --journal "$LOCAL/.audit/register/journal.jsonl" 2>&1); rc=$?
  items=""; [ -f "$O/replay.db" ] && items=$(lk "$REMOTE2" --out "$O" -- sqlite3 -readonly "file:/out/replay.db?immutable=1" "select group_concat(atm_id||'='||title,' | ') from (select atm_id,title from items order by atm_id)" 2>&1)
  echo "# E2E replay rc=$rc (local mint printed $LID, remote mint $RID) :: $out :: items=[$items]" | cut -c1-500
  case "$items" in *"CAT-004=scratch item"*) bad "E2E-1 the local item was silently renumbered to CAT-004 with replay verdict rc=$rc";; *) [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'reason=replay_ids_unknown' && ok "E2E-1 the replay of a mint recorded under a pending -wal is refused replay_ids_unknown, nothing renumbered" || bad "E2E-1 rc=$rc [$out]";; esac
  [ ! -e "$O" ] && ok "E2E-2 no output directory after the refusal" || bad "E2E-2"
fi

# ================= M6: the backup integrity check refuses index damage that the canonical dump cannot see =================
if want M6; then echo "== M6 backup integrity check (index root pages swapped in the backup between step 1 and step 2) =="
  R=$(mkroot m6); cdb "$R" workable_items.db || bad setup; for i in 1 2 3; do lk "$R" -- sh -c "$(mint_cmd workable_items.db)" >/dev/null 2>&1; done
  W="$T_SCR/locked_wrap_m6.sh"; cat >"$W" <<'WRAP'
#!/usr/bin/env bash
# LOCKED wrapper: before the step-2 call (--op-id *-verify) the newest docs/*.bak-* of the scratch root gets two index root pages swapped
# (writable_schema; the table b-trees stay untouched, so the canonical .dump is unchanged and only integrity_check can see it)
V=0; for a in "$@"; do case "$a" in *-verify) V=1;; esac; done
if [ "$V" = 1 ]; then b=$(ls -t "$LOCKED_ROOT"/docs/workable_items.db.bak-* | head -1)
  python3 -I - "$b" <<'P'
import sqlite3,sys
c=sqlite3.connect(sys.argv[1]); c.execute("PRAGMA writable_schema=ON")
ix=c.execute("select name,rootpage from sqlite_master where name in ('idx_item_history_atm_id','idx_item_history_event_type') order by name").fetchall()
if len(ix)<2: ix=c.execute("select name,rootpage from sqlite_master where type='index' and sql is not null order by name limit 2").fetchall()
(a,ra),(b2,rb)=ix[0],ix[1]
c.execute("update sqlite_master set rootpage=? where name=?",(rb,a)); c.execute("update sqlite_master set rootpage=? where name=?",(ra,b2))
v=c.execute("pragma schema_version").fetchone()[0]; c.execute("pragma schema_version=%d"%(v+1)); c.commit(); c.close()
P
fi
exec bash "$REAL_LOCKED" "$@"
WRAP
  chmod +x "$W"
  hbm() { local bk=$1; shift; env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED_RUNP="$RUNP" LOCKED="$W" REAL_LOCKED="$LOCKED" SHIM_LOG="$SHIM_LOG" PATH="$SHIM_DIR:$PATH" bash "$bk" "$@"; }
  out=$(hbm "$BACKUP" --record "$T_SCR/m6a.json" 2>&1); rc=$?
  [ $rc -ne 0 ] && [ ! -e "$T_SCR/m6a.json" ] && [ -z "$(ls "$R"/docs/workable_items.db.bak-* 2>/dev/null)" ] && printf '%s' "$out" | grep -q 'integrity_check' && ok "M6-1 index damage invisible to the canonical dump: the backup is refused (integrity_check), no record, no file" || bad "M6-1 rc=$rc [$out]"
  sleep 1.1
  out=$(env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED_RUNP="$RUNP" LOCKED="$LOCKED" SHIM_LOG="$SHIM_LOG" PATH="$SHIM_DIR:$PATH" bash "$BACKUP" --record "$T_SCR/m6b.json" 2>&1); rc=$?
  [ $rc -eq 0 ] && ok "M6-2 golden-true: the same register without the damage backs up" || bad "M6-2 rc=$rc [$out]"
fi

finish
