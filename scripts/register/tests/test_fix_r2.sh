#!/usr/bin/env bash
# test_fix_r2.sh - WF13 review fix round (11.4.276 round 3, the STRUCTURAL round) of the register operations: N1..N9, T1..T5, P3 of
# WF13-REVIEW-register-ops-r2.md. Written FIRST: every section reproduces one reviewed defect (or one member of its defect class) against the committed
# scripts (RED) and must pass on the fixed ones (GREEN). Defect classes (each member is a case below):
#   C1 "a journal row that changed the register is dropped by replay with an OK verdict": failed row with a changed hash / minted ids / pending -wal bytes, an
#      interrupted row, and the substring skips (regenerate marker, --install) that also dropped writes which merely MENTION the marker  -> N1*
#   C2 "the register lock is released before the writer's write has landed": signals TERM INT HUP USR1 USR2 ALRM QUIT to locked.sh, SIGPIPE (the podman client
#      leaves first), a client that dies early, an exit path other than the normal one, SIGKILL of locked.sh (fenced by the NEXT locked.sh)    -> SIG, DRAIN, FENCE, REAL
#   C3 "the lock-claim proof checks the wrong condition": fd 9 open but never locked (nobody / another process holds), pid not an ancestor, fd 9 closed, fd 9 on
#      another file                                                                                                                           -> LC
#   C4 "the writer's exit status is lost in a race": a fast-exiting child reaped before the first wait; the status of a container that outlived its client -> RACE, DRAIN
#   C5 "a bound that does not bound": the reaper without a KILL escalation                                                                      -> RP
#   C6 "a recorded identity or header that was not checked against the thing it names": backup source_sha256 under a busy checkpoint, backup file left by die,
#      reconcile header of an empty view, zero-byte id snapshot, the replay fallback base with a pending -wal                                     -> B, RC, T4, T5
# Env: SUT_DIR (directory holding the six scripts under test; the RED run points it at a copy of commit 846d441f with LOCKED, BACKUP, DUMP, EXPORT, REPLAY,
# RECONCILE), ONLY="N1S DRAIN" (sections to run), NO_E2E=1 (skip the sections that need real containers; the mutation runner sets it for follow-up runs).
. "$(dirname "$0")/lib.sh"; . "$(dirname "$0")/clib.sh"; . "$(dirname "$0")/mutscore.sh"
evhead FIX-R2
SUT_DIR=${SUT_DIR:-$REG_DIR}
RECONCILE=${RECONCILE:-$SUT_DIR/reconcile.sh}
ONLY=" ${ONLY:-N1S N1E DRAIN SIG RACE FENCE LC RP B RC T3 T4 T5 REAL} "
want() { case "$ONLY" in *" $1 "*) return 0;; esac; return 1; }
e2e() { [ -z "${NO_E2E:-}" ]; }
echo "# sut_dir=$SUT_DIR sha256 locked=$(fsha "$LOCKED" | cut -c1-16) backup=$(fsha "$BACKUP" | cut -c1-16) reconcile=$(fsha "$RECONCILE" | cut -c1-16) replay=$(fsha "$REPLAY" | cut -c1-16)"

# ---------- stub machinery (no container): a stub RUNP, a stub podman holding one fake container per op id ----------
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
  *) [ -x "$STUB_DIR/main.sh" ] && exec "$STUB_DIR/main.sh" "${a[@]}"; exit 0;;
esac
S
chmod +x "$STUBRUNP"
STUBPODMAN="$T_SCR/stub_podman.sh"
cat >"$STUBPODMAN" <<'S'
#!/usr/bin/env bash
# stub podman (the documented LOCKED_PODMAN test hook): a fake container of op X is the pid in $STUB_DIR/ctr.X.pid; its exit status is $STUB_DIR/ctr.X.rc
echo "$(date +%s.%N) $*" >>"$STUB_DIR/podman.log"
[ -f "$STUB_DIR/podman.fail" ] && exit 125
case "$1" in
  ps) op=""; for a in "$@"; do case "$a" in label=catalogizer.op_id=*) op="${a#label=catalogizer.op_id=}";; esac; done
      p=$(cat "$STUB_DIR/ctr.$op.pid" 2>/dev/null); if [ -n "$p" ] && kill -0 "$p" 2>/dev/null; then echo "ctr-$op"; fi; exit 0;;
  wait) op="${*: -1}"; op="${op#ctr-}"; p=$(cat "$STUB_DIR/ctr.$op.pid" 2>/dev/null); while [ -n "$p" ] && kill -0 "$p" 2>/dev/null; do sleep 0.1; done; cat "$STUB_DIR/ctr.$op.rc" 2>/dev/null || echo 0; exit 0;;
  inspect) id="${*: -1}"; op="${id#ctr-}"; cat "$STUB_DIR/ctr.$op.src" 2>/dev/null; exit 0;;
  events) op=""; for a in "$@"; do case "$a" in label=catalogizer.op_id=*) op="${a#label=catalogizer.op_id=}";; esac; done   # WF15: the died event of an ended container (the exit status source since round 4)
          p=$(cat "$STUB_DIR/ctr.$op.pid" 2>/dev/null); if [ -f "$STUB_DIR/ctr.$op.rc" ] && ! { [ -n "$p" ] && kill -0 "$p" 2>/dev/null; }; then echo "ctr-$op $(cat "$STUB_DIR/ctr.$op.rc")"; fi; exit 0;;
  rm) id="${*: -1}"; op="${id#ctr-}"; p=$(cat "$STUB_DIR/ctr.$op.pid" 2>/dev/null); if [ -n "$p" ] && [ "$p" -gt 1 ]; then kill -9 "$p" 2>/dev/null; fi; echo killed >"$STUB_DIR/ctr.$op.killed"; exit 0;;
esac
exit 0
S
chmod +x "$STUBPODMAN"
sreset() { rm -rf "$STUB_DIR"; mkdir -p "$STUB_DIR"; }
sroot() { local r; r=$(mkroot "$1"); echo "stub db" >"$r/docs/workable_items.db"; echo "$r"; }
slk() { LK_RUNP="$STUBRUNP" LOCKED_PODMAN="$STUBPODMAN" lk "$@"; }
jrow() { python3 -I -c 'import json,sys
rows=[json.loads(l) for l in open(sys.argv[1]) if l.strip()]
r=[x for x in rows if x["op_id"]==sys.argv[2]]
print("none" if not r else json.dumps(r[0].get(sys.argv[3])))' "$@"; }
jn() { python3 -I -c 'import sys;print(sum(1 for l in open(sys.argv[1]) if l.strip()))' "$1" 2>/dev/null || echo 0; }
killpid() { case "${1:-}" in ''|*[!0-9]*) return 0;; esac; [ "$1" -gt 1 ] && kill "$1" 2>/dev/null; return 0; }
fakectr() {  # fakectr <op> <seconds> [<rc>]: a fake detached container of op <op> that logs s/e lines and ends with status <rc>
  ( echo "s $(date +%s%N) ctr-$1" >>"$STUB_DIR/ov.log"; sleep "$2"; echo "e $(date +%s%N) ctr-$1" >>"$STUB_DIR/ov.log"; echo "${3:-0}" >"$STUB_DIR/ctr.$1.rc" ) >/dev/null 2>&1 </dev/null &
  echo $! >"$STUB_DIR/ctr.$1.pid"; disown 2>/dev/null; }
# a stub main: the "client". With CTR_SLEEP set it starts a fake container (which writes the database file late) and leaves at once with STUB_CLIENT_RC.
cat >"$STUB_DIR/../main_ctr.sh" <<'S'
#!/usr/bin/env bash
op=""; while [ $# -gt 0 ]; do case "$1" in --op-id) op="$2"; shift 2;; --) shift; break;; *) shift;; esac; done
echo "w $(date +%s%N) $op" >>"$STUB_DIR/ov.log"
[ -z "${STUB_PRINT:-}" ] || echo first-line
if [ -n "${CTR_SLEEP:-}" ]; then
  ( echo "s $(date +%s%N) ctr-$op" >>"$STUB_DIR/ov.log"; sleep "$CTR_SLEEP"; echo "late write" >>docs/workable_items.db; echo "e $(date +%s%N) ctr-$op" >>"$STUB_DIR/ov.log"; echo "${CTR_RC:-5}" >"$STUB_DIR/ctr.$op.rc" ) >/dev/null 2>&1 </dev/null &
  echo $! >"$STUB_DIR/ctr.$op.pid"
fi
exit "${STUB_CLIENT_RC:-141}"
S
chmod +x "$STUB_DIR/../main_ctr.sh"
usemain() { cp "$T_SCR/main_ctr.sh" "$STUB_DIR/main.sh"; }
lockfree() { flock -n "$1" true 2>/dev/null; }

# ================= N1: replay never drops a row that changed the register =================
if want N1S; then echo "== N1 replay planning of failed / interrupted / marker-mentioning rows (stub tools, no container) =="
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
  mkj() { mkdir -p "$H/j/$1"; python3 -I -c '
import json,sys
a,b,c=sys.argv[2:5]
def row(op,mode,db,bs,as_,minted,argv,ex=0,snap="ok",wal=0,seq=None,**kw):
    r={"op_id":op,"mode":mode,"db":db,"db_sha_before":bs,"db_sha_after":as_,"ids_minted":minted,"argv":argv,"exit":ex,"input_args":{},"inputs":[],"wal_bytes_after":wal,"ids_snapshot":snap}
    if seq is not None: r["seq"]=seq
    r.update(kw); return r
REG="docs/workable_items.db"; M=["sqlite3","/src/"+REG,"MINT"]
rows=eval(sys.stdin.read())
open(sys.argv[1]+"/journal.jsonl","w").write("".join((l if isinstance(l,str) else json.dumps(l))+"\n" for l in rows))' "$H/j/$1" "$A" "$B" "$C"; }
  rp() { local n=$1 ids=$2 fin="${3:-CAT-001 CAT-002}"; printf '%s\n' "$ids" >"$H/ids_$n.txt"; printf '%s\n' "$fin" >"$H/ids_$n.txt.final"; rm -f "$H/ids_$n.txt.n"; rm -rf "$H/out_$n"
    local o; o=$(env LOCKED_TEST_MODE=1 LOCKED_ROOT="$H/root" LOCKED="$H/stubs/locked.sh" BACKUP="$H/stubs/backup.sh" DUMP="$H/stubs/noop.sh" EXPORT="$H/stubs/noop.sh" HARNESS_LOG="$H/log_$n" HARNESS_IDS="$H/ids_$n.txt" bash "$REPLAY" --onto docs/workable_items.db --since "$A" --out "$H/out_$n" --journal "$H/j/$n/journal.jsonl" 2>&1); echo "rc=$? $o" | cut -c1-400; }
  chk() { local name=$1 r=$2 want=$3 label=$4; case "$r" in $want) ok "$name $label";; *) bad "$name $label :: [$r]";; esac; }
  mkj ctl <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("W2","register",REG,a,b,["CAT-002"],M)]
E
  chk N1-0 "$(rp ctl 'CAT-001')" 'rc=0 replay: OK replayed=1 *' "control: an ok register row is replayed"
  mkj fch <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("EDIT","register",REG,a,b,[],["sqlite3","/src/"+REG,"UPDATE items SET title='x'","SELECT * FROM nosuch"],ex=1)]
E
  chk N1-1 "$(rp fch 'CAT-001')" 'rc=20 *reason=replay_failed_row_changed_register*EDIT*' "a failed row that CHANGED the database (hash differs) is refused, never skipped (the local edit must not vanish with an OK verdict)"
  [ ! -e "$H/out_fch" ] && ok "N1-1b the refusal removed the output directory" || bad "N1-1b output directory left"
  mkj fmint <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("MINTFAIL","register",REG,b,b,["CAT-002"],M,ex=7)]
E
  chk N1-2 "$(rp fmint 'CAT-001')" 'rc=20 *reason=replay_failed_row_changed_register*MINTFAIL*' "a failed row that minted ids is refused"
  mkj fwal <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("WALFAIL","register",REG,b,b,[],M,ex=1,wal=4152)]
E
  chk N1-3 "$(rp fwal 'CAT-001')" 'rc=20 *reason=replay_failed_row_changed_register*WALFAIL*' "a failed row whose change is pending in the -wal file (main-file hash unchanged) is refused"
  mkj fint <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("INTR","register",REG,a,b,[],M,ex=143,interrupted="TERM")]
E
  chk N1-4 "$(rp fint 'CAT-001')" 'rc=20 *reason=replay_failed_row_changed_register*INTR*' "an interrupted writer (exit 143) that changed the database is refused"
  mkj fsnap <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("UNK","register",REG,a,b,[],M,ex=1,snap="unavailable")]
E
  chk N1-5 "$(rp fsnap 'CAT-001')" 'rc=20 *reason=replay_failed_row_changed_register*UNK*' "a failed row with an unknown id snapshot that changed the database is refused as a failed row (never skipped)"
  mkj fnone <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("FAILRO","register",REG,b,b,[],["sqlite3","/src/"+REG,"SELECT * FROM nosuch"],ex=1), row("W2","register",REG,b,c,["CAT-002"],M)]
E
  chk N1-6 "$(rp fnone 'CAT-001')" 'rc=0 replay: OK replayed=1 skipped=1 *' "golden-FALSE: a failed row that changed nothing (hash equal, no ids, no -wal) is skipped and listed, the next row is replayed"
  [ "$(python3 -I -c 'import json,sys;print([s["reason"] for s in json.load(open(sys.argv[1]))["rows_skipped"]])' "$H/out_fnone/replay-report.json" 2>/dev/null)" = "['command_failed']" ] && ok "N1-6b the report lists the skipped row with reason command_failed" || bad "N1-6b report: $(cat "$H/out_fnone/replay-report.json" 2>/dev/null | head -c 300)"
  mkj mention <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("MENT","register",REG,a,b,[],["sqlite3","/src/"+REG,"UPDATE items SET title='# register-regenerate:fake' WHERE 1"]), row("MENT2","register",REG,b,c,[],["sqlite3","/src/"+REG,"UPDATE items SET title='export.sh --install' WHERE 1"]), row("MENT3","register",REG,c,b,[],["env","--install","sqlite3","/src/"+REG,"UPDATE items SET title='see export.sh' WHERE 1"])]
E
  chk N1-7 "$(rp mention 'CAT-001')" 'rc=0 replay: OK replayed=3 skipped=0 *' "a register write whose SQL merely MENTIONS the regenerate marker or 'export.sh --install' is replayed, not dropped (the skips key on the program, not on a substring)"
  mkj realskip <<'E'
[row("W1","register",REG,None,a,["CAT-001"],M), row("REGEN","register",REG,a,b,[],["bash","-c","# register-regenerate:dump\nset -eo pipefail"]), row("INST","register",REG,b,b,[],["bash","/src/scripts/register/export.sh","--install"])]
E
  chk N1-8 "$(rp realskip 'CAT-001')" 'rc=0 replay: OK replayed=0 skipped=2 *' "golden-true: the real regeneration row (exactly bash -c <script starting with the marker>) and an UNCHANGED export.sh --install row are still skipped (WF15 M6: a changed install row is refused, see test_fix_r4.sh M6-3)"
fi

if want N1E && e2e; then echo "== N1 end to end (real containers): a failed local write that committed =="
  BASE=$(mkroot n1base); cdb "$BASE" workable_items.db || bad "N1E setup"
  for i in 1 2; do lk "$BASE" -- sh -c "$(mint_cmd workable_items.db)" >/dev/null 2>&1 || { bad "N1E setup mint"; }; done
  SINCE=$(tail -n1 "$BASE/.audit/register/journal.jsonl" | python3 -I -c 'import json,sys;print(json.load(sys.stdin)["db_sha_after"])')
  LOCAL="$T_SCR/n1local"; REMOTE="$T_SCR/n1remote"; cp -a "$BASE" "$LOCAL"; cp -a "$BASE" "$REMOTE"
  lk "$LOCAL" --op-id local-edit -- sqlite3 /src/docs/workable_items.db "UPDATE items SET title='LOCAL-EDIT committed' WHERE atm_id='CAT-001';" "SELECT * FROM no_such_table;" >/dev/null 2>&1; lrc=$?
  [ "$lrc" -ne 0 ] && ok "N1E-0 fixture: the local write exited $lrc after its UPDATE committed" || bad "N1E-0 fixture: the local write exited 0"
  O="$T_SCR/n1out"; out=$(tool "$REMOTE" "$REPLAY" --onto docs/workable_items.db --since "$SINCE" --out "$O" --journal "$LOCAL/.audit/register/journal.jsonl" 2>&1); rc=$?
  echo "# N1E replay rc=$rc :: $(printf '%s' "$out" | cut -c1-260)"
  [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'reason=replay_failed_row_changed_register' && [ ! -e "$O" ] && ok "N1E-1 the replay of a journal holding a failed-but-committed local edit is refused (20), nothing is written" || bad "N1E-1 rc=$rc [$out]"
fi

# ================= DRAIN: the lock is kept until the container is gone (stub podman) =================
if want DRAIN; then echo "== DRAIN client leaves before its container (stub podman, no real container) =="
  sreset; usemain; R=$(sroot d1); LCK="$R/docs/.register.lock"; B0=$(sha256sum "$R/docs/workable_items.db" | cut -d' ' -f1)
  ( CTR_SLEEP=4 STUB_CLIENT_RC=141 slk "$R" --op-id d1 -- sh -c 'w' >"$T_SCR/d1.out" 2>&1; echo $? >"$T_SCR/d1.rc" ) &
  for i in $(seq 1 100); do [ -s "$STUB_DIR/ctr.d1.pid" ] && break; sleep 0.1; done
  W=$(cat "$STUB_DIR/ctr.d1.pid" 2>/dev/null)
  ( CTR_SLEEP=1 STUB_CLIENT_RC=0 slk "$R" --op-id d2 -- sh -c 'second' >"$T_SCR/d2.out" 2>&1; echo $? >"$T_SCR/d2.rc" ) &
  free_while_alive=0; seen=0
  for i in $(seq 1 150); do kill -0 "$W" 2>/dev/null || break; if lockfree "$LCK"; then free_while_alive=$((free_while_alive+1)); else seen=1; fi; sleep 0.1; done
  wait
  assert_eq "DRAIN-1 the register lock was never free while the container of the call whose client had already left was still running" "$free_while_alive" 0
  assert_eq "DRAIN-2 locked.sh exit status = the container's real exit status (5), not the dead client's (141)" "$(cat "$T_SCR/d1.rc")" 5
  J="$R/.audit/register/journal.jsonl"
  A1=$(sha256sum "$R/docs/workable_items.db" | cut -d' ' -f1)
  assert_eq "DRAIN-3 the journal row of the first call is written AFTER its container's write: db_sha_before is the old state" "$(jrow "$J" d1 db_sha_before)" "\"$B0\""
  if [ "$(jrow "$J" d1 db_sha_after)" != "\"$B0\"" ] && [ "$(jrow "$J" d1 db_sha_after)" != null ]; then ok "DRAIN-4 db_sha_after differs from before: the row records the state AFTER the late write"; else bad "DRAIN-4 db_sha_after=$(jrow "$J" d1 db_sha_after) (the write had not landed when the row was taken)"; fi
  assert_eq "DRAIN-5 the row says how the lock was held: container_drain=waited" "$(jrow "$J" d1 container_drain)" '"waited"'
  assert_eq "DRAIN-6 the row records the client's own status: client_exit=141 (exit differs)" "$(jrow "$J" d1 client_exit)" 141
  [ -e "$R/.audit/register/pending/d1" ] && bad "DRAIN-7 a pending marker is left for a drained, journaled op" || ok "DRAIN-7 no pending marker is left once the drained op is journaled"
  python3 -I - "$STUB_DIR/ov.log" <<'PY' && ok "DRAIN-8 the second writer did not start before the first container ended (w of d2 after e of ctr-d1)" || bad "DRAIN-8 overlap: $(tr '\n' ' ' <"$STUB_DIR/ov.log")"
import sys
ev=[l.split() for l in open(sys.argv[1]) if l.strip()]
e1=[int(e[1]) for e in ev if e[0]=="e" and e[2]=="ctr-d1"][0]
w2=[int(e[1]) for e in ev if e[0]=="w" and e[2]=="d2"][0]
assert w2>e1, (w2,e1)
PY
  # the same call with a normal client: nothing to drain
  sreset; usemain; R=$(sroot d4); STUB_CLIENT_RC=0 slk "$R" --op-id d4 -- sh -c 'w' >/dev/null 2>&1; rc=$?
  assert_eq "DRAIN-9 golden-FALSE: a call whose container is gone when the client exits drains nothing: container_drain=none, exit 0" "$rc/$(jrow "$R/.audit/register/journal.jsonl" d4 container_drain)" '0/"none"'
  # a bounded wait: a container that never ends is killed after LOCKED_CONTAINER_WAIT and the row says so
  sreset; usemain; R=$(sroot d5); t0=$(date +%s)
  CTR_SLEEP=60 STUB_CLIENT_RC=141 LOCKED_CONTAINER_WAIT=2 slk "$R" --op-id d5 -- sh -c 'w' >"$T_SCR/d5.out" 2>&1; rc=$?; dt=$(( $(date +%s)-t0 ))
  [ $dt -lt 30 ] && [ -e "$STUB_DIR/ctr.d5.killed" ] && [ "$(jrow "$R/.audit/register/journal.jsonl" d5 container_drain)" = '"killed"' ] && ok "DRAIN-10 a container that outlives LOCKED_CONTAINER_WAIT is removed through podman and the row says container_drain=killed (${dt}s, exit $rc)" || bad "DRAIN-10 dt=${dt}s rc=$rc killed=$([ -e "$STUB_DIR/ctr.d5.killed" ] && echo y || echo n) row=$(jrow "$R/.audit/register/journal.jsonl" d5 container_drain)"
  [ -e "$R/.audit/register/pending/d5" ] && ok "DRAIN-11 a killed writer leaves its pending marker (replay refuses: the write is partial)" || bad "DRAIN-11 no pending marker after a killed container"
  killpid "$(cat "$STUB_DIR/ctr.d5.pid" 2>/dev/null)"
  # SIGPIPE to locked.sh itself: its own message to a closed pipe (`2>&1 | head -n1`) must not end it while the container is being drained
  sreset; usemain; R=$(sroot d7)
  ( STUB_PRINT=1 CTR_SLEEP=30 STUB_CLIENT_RC=141 LOCKED_CONTAINER_WAIT=2 slk "$R" --op-id d7 -- sh -c 'w' 2>&1; echo $? >"$T_SCR/d7.rc" ) | head -n1 >"$T_SCR/d7.head"
  for i in $(seq 1 100); do [ -s "$T_SCR/d7.rc" ] && break; sleep 0.2; done
  [ "$(cat "$T_SCR/d7.head")" = first-line ] && [ "$(jrow "$R/.audit/register/journal.jsonl" d7 container_drain)" = '"killed"' ] && [ -e "$STUB_DIR/ctr.d7.killed" ] && ok "DRAIN-13 locked.sh | head -n1 (its own later warning goes to the closed pipe): SIGPIPE does not end locked.sh, the container is still drained and the row is written (container_drain=killed)" || bad "DRAIN-13 head=[$(cat "$T_SCR/d7.head")] rc=$(cat "$T_SCR/d7.rc" 2>/dev/null) row=$(jrow "$R/.audit/register/journal.jsonl" d7 container_drain)"
  killpid "$(cat "$STUB_DIR/ctr.d7.pid" 2>/dev/null)"
  # podman cannot answer: the claim of a released container is not made
  sreset; usemain; R=$(sroot d6); : >"$STUB_DIR/podman.fail"
  STUB_CLIENT_RC=3 slk "$R" --op-id d6 -- sh -c 'w' >"$T_SCR/d6.out" 2>&1; rc=$?
  [ "$(jrow "$R/.audit/register/journal.jsonl" d6 container_drain)" = '"unverified"' ] && [ $rc -eq 3 ] && [ -e "$R/.audit/register/pending/d6" ] && ok "DRAIN-12 podman unavailable: the row says container_drain=unverified, the client's status 3 is passed through, the pending marker is kept" || bad "DRAIN-12 rc=$rc row=$(jrow "$R/.audit/register/journal.jsonl" d6 container_drain) marker=$([ -e "$R/.audit/register/pending/d6" ] && echo y || echo n)"
fi

# ================= SIG: every catchable signal keeps the lock until the writer is gone =================
if want SIG; then echo "== SIG signals to locked.sh while the writer runs (stub RUNP) =="
  cat >"$T_SCR/main_sig.sh" <<'S'
#!/usr/bin/env bash
echo $$ > "$STUB_DIR/main.pid"
echo "s $(date +%s%N) $$" >> "$STUB_DIR/ov.log"
sleep 5 & SL=$!
h() { kill $SL 2>/dev/null; sleep 1; echo "e $(date +%s%N) $$ sig" >> "$STUB_DIR/ov.log"; exit 143; }
trap h TERM HUP USR1 USR2 ALRM
wait $SL
echo "e $(date +%s%N) $$ done" >> "$STUB_DIR/ov.log"
S
  chmod +x "$T_SCR/main_sig.sh"
  for S in TERM INT HUP USR1 USR2 ALRM QUIT; do
    sreset; cp "$T_SCR/main_sig.sh" "$STUB_DIR/main.sh"; R=$(sroot "sig$S"); LCK="$R/docs/.register.lock"; N=$(kill -l "$S")
    # started through python (which restores SIGINT/SIGQUIT to their defaults and execs: the pid stays) so the call sees the dispositions of a foreground command
    python3 -I -c 'import os,signal,sys;signal.signal(signal.SIGINT,signal.SIG_DFL);signal.signal(signal.SIGQUIT,signal.SIG_DFL);os.execvp("env",sys.argv[1:])' env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED_RUNP="$STUBRUNP" LOCKED_PODMAN="$STUBPODMAN" bash "$LOCKED" --op-id "sig-$S" -- sh -c 'w' >"$T_SCR/sig.out" 2>&1 &
    P=$!
    for i in $(seq 1 100); do [ -s "$STUB_DIR/main.pid" ] && break; sleep 0.1; done
    W=$(cat "$STUB_DIR/main.pid" 2>/dev/null)
    IGN=0; if [ "$P" -gt 1 ] && tr '\0' ' ' </proc/$P/cmdline 2>/dev/null | grep -q 'locked.sh' && [ -n "$W" ] && [ "$W" -gt 1 ]; then python3 -I -c 'import sys;m=int(sys.argv[1],16);print(1 if m>>(int(sys.argv[2])-1)&1 else 0)' "$(awk '/^SigIgn/{print $2}' /proc/$P/status)" "$N" >"$T_SCR/ign" 2>/dev/null; IGN=$(cat "$T_SCR/ign"); kill -s "$S" "$P" 2>/dev/null; else bad "SIG-$S fixture: pid $P is not our locked.sh (writer $W)"; fi
    free_while_alive=0
    for i in $(seq 1 120); do kill -0 "$W" 2>/dev/null || break; lockfree "$LCK" && free_while_alive=$((free_while_alive+1)); sleep 0.1; done
    wait "$P"; arc=$?
    JR="$R/.audit/register/journal.jsonl"
    # a background job of a NON-interactive shell starts with SIGINT and SIGQUIT ignored (POSIX): the signal is then ignored by locked.sh itself, which is harmless
    # (the lock stays held, nothing is forwarded, no row says interrupted); the proof of that disposition is /proc/<pid>/status SigIgn, read before the signal
    if [ "$IGN" = 1 ]; then
      if [ "$free_while_alive" = 0 ] && [ "$arc" = 0 ] && [ "$(jrow "$JR" "sig-$S" interrupted)" = null ]; then ok "SIG-$S SIG$S was ignored at entry (SigIgn bit set: background job of a non-interactive shell): the call ran to its end, the lock was never free, exit 0"
      else bad "SIG-$S (ignored) free_while_alive=$free_while_alive exit=$arc interrupted=$(jrow "$JR" "sig-$S" interrupted)"; fi
    elif [ "$free_while_alive" = 0 ] && { [ "$arc" = 143 ] || [ "$arc" = $((128+N)) ]; } && [ "$(jrow "$JR" "sig-$S" interrupted)" = "\"$S\"" ]; then ok "SIG-$S SIG$S to locked.sh: the lock was never free while the writer lived, exit $arc, journal interrupted=$S"
    else bad "SIG-$S free_while_alive=$free_while_alive exit=$arc interrupted=$(jrow "$JR" "sig-$S" interrupted) out=[$(head -c 200 "$T_SCR/sig.out")]"; fi
  done
fi

# ================= RACE: the writer's exit status is never lost =================
if want RACE; then echo "== RACE fast-exiting writer: exit status 7 =="
  sreset; printf '#!/bin/sh\nexit 7\n' >"$STUB_DIR/main.sh"; chmod +x "$STUB_DIR/main.sh"; R=$(sroot race); lost=0
  # deterministic leg: a slow `cat` (proc_ppid reads /proc through it) on PATH makes the child be reaped by bash BEFORE the first look at /proc/<pid>, the exact window of the race
  SC="$T_SCR/slowcat"; mkdir -p "$SC"; printf '#!/bin/sh\nsleep 0.3\nexec /usr/bin/cat "$@"\n' >"$SC/cat"; chmod +x "$SC/cat"
  for i in $(seq 1 12); do PATH="$SC:$PATH" slk "$R" --op-id "racedet-$i" -- sh -c 'w' >/dev/null 2>&1; rc=$?; [ $rc -eq 7 ] || lost=$((lost+1)); done
  [ "$lost" = 0 ] && ok "RACE-1 deterministic: with the child reaped before the first /proc read, 12 of 12 calls still exit 7 (status lost: $lost)" || bad "RACE-1 the exit status was lost in $lost of 12 calls (the reaped child's status was never read)"
  lost=0; for i in $(seq 1 60); do slk "$R" --op-id "race-$i" -- sh -c 'w' >/dev/null 2>&1; rc=$?; [ $rc -eq 7 ] || lost=$((lost+1)); done
  jbad=$(python3 -I -c 'import json,sys
n=0
for l in open(sys.argv[1]):
    if l.strip() and json.loads(l)["exit"]!=7: n+=1
print(n)' "$R/.audit/register/journal.jsonl")
  [ "$lost" = 0 ] && [ "$jbad" = 0 ] && ok "RACE-2 natural timing: 60 of 60 calls exited 7, and no journal row of the 72 calls says another status (lost $lost, wrong rows $jbad)" || bad "RACE-2 lost $lost of 60 calls; journal rows with another status: $jbad"
fi

# ================= FENCE: a SIGKILLed locked.sh does not let a second writer overlap its container =================
if want FENCE; then echo "== FENCE the next locked.sh waits for the container of a dead predecessor (stub podman) =="
  mkdead() {  # mkdead <root> <op>: a pending marker of a writer whose locked.sh is gone
    mkdir -p "$1/.audit/register/pending"; printf '{"op_id":"%s","pid":4194000}\n' "$2" >"$1/.audit/register/pending/$2"; [ ! -e /proc/4194000 ] || bad "FENCE fixture: pid 4194000 exists"; }
  sreset; usemain; R=$(sroot fe1); mkdead "$R" dead1; fakectr dead1 3 0
  t0=$(date +%s%N); STUB_CLIENT_RC=0 slk "$R" --op-id fe1 -- sh -c 'w' >"$T_SCR/fe1.out" 2>&1; rc=$?
  python3 -I - "$STUB_DIR/ov.log" <<'PY' && ok "FENCE-1 the new writer's call starts only after the container of the dead predecessor ended (w of fe1 after e of ctr-dead1), exit $rc" || bad "FENCE-1 overlap: $(tr '\n' ' ' <"$STUB_DIR/ov.log") rc=$rc [$(head -c 200 "$T_SCR/fe1.out")]"
import sys
ev=[l.split() for l in open(sys.argv[1]) if l.strip()]
e1=[int(e[1]) for e in ev if e[0]=="e" and e[2]=="ctr-dead1"][0]
w2=[int(e[1]) for e in ev if e[0]=="w" and e[2]=="fe1"][0]
assert w2>e1, (w2,e1)
PY
  [ -e "$R/.audit/register/pending/dead1" ] && ok "FENCE-2 the dead predecessor's pending marker is left (its write was never journaled: replay refuses)" || bad "FENCE-2 the marker of the dead writer was removed"
  sreset; usemain; R=$(sroot fe2); mkdead "$R" dead2; fakectr dead2 40 0
  out=$(STUB_CLIENT_RC=0 LOCKED_CONTAINER_WAIT=2 slk "$R" --op-id fe2 -- sh -c 'w' 2>&1); rc=$?
  [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'reason=writer_still_running' && ! grep -q ' fe2$' "$STUB_DIR/ov.log" && ok "FENCE-3 a predecessor container that outlives the bound refuses the new writer (20 writer_still_running), no writer call was made" || bad "FENCE-3 rc=$rc [$out]"
  killpid "$(cat "$STUB_DIR/ctr.dead2.pid" 2>/dev/null)"
  sreset; usemain; R=$(sroot fe3); mkdead "$R" dead3
  STUB_CLIENT_RC=0 slk "$R" --op-id fe3 -- sh -c 'w' >/dev/null 2>&1; rc=$?
  assert_eq "FENCE-4 golden-FALSE: a dead predecessor whose container is gone does not delay or refuse the new writer (exit 0)" "$rc" 0
  sreset; usemain; R=$(sroot fe4); mkdir -p "$R/.audit/register/pending"; bash -c 'exec -a locked.sh sleep 20' & OWN=$!; sleep 0.3
  printf '{"op_id":"live4","pid":%s}\n' "$OWN" >"$R/.audit/register/pending/live4"; fakectr live4 6 0
  t0=$(date +%s); STUB_CLIENT_RC=0 slk "$R" --op-id fe4 -- sh -c 'w' >/dev/null 2>&1; rc=$?; dt=$(( $(date +%s)-t0 ))
  [ $rc -eq 0 ] && [ $dt -ge 4 ] && ok "FENCE-5 (WF15 M2) a marker whose pid is a live process with 'locked.sh' in its cmdline is NOT an owner (no cmdline proxy): its running container is still waited for (${dt}s)" || bad "FENCE-5 rc=$rc dt=${dt}s"
  killpid "$OWN"; killpid "$(cat "$STUB_DIR/ctr.live4.pid" 2>/dev/null)"; wait 2>/dev/null
fi

# ================= LC: the lock-claim proof =================
if want LC; then echo "== LC LOCKED_LOCK_HELD is accepted only for the ancestor whose fd 9 HOLDS the lock =="
  sreset; R=$(sroot lc); LCK="$R/docs/.register.lock"; : >"$LCK"
  lcrun() { env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED_RUNP="$STUBRUNP" LOCKED_PODMAN="$STUBPODMAN" LCK="$LCK" LK="$LOCKED" OTHER="${OTHER:-}" OTHERF="$T_SCR/otherfile" bash -c "$1" 2>&1; }
  lcexp() { local name=$1 body=$2 want=$3 label=$4 out; out=$(lcrun "$body"); case "$want" in refused) if printf '%s' "$out" | grep -q 'reason=lock_claim_invalid' && ! printf '%s' "$out" | grep -q '^rc=0'; then ok "$name $label"; else bad "$name $label :: [$out]"; fi;; ok) if printf '%s' "$out" | grep -q '^rc=0'; then ok "$name $label"; else bad "$name $label :: [$out]"; fi;; esac; }
  RUN='LOCKED_LOCK_HELD=$$ bash "$LK" --op-id lc -- true; echo rc=$?'
  lcexp LC-1 "exec 9>>\"\$LCK\"; $RUN" refused "fd 9 is open on the lock file but NOBODY holds the lock: refused (an unlocked fd 9 proves nothing)"
  flock "$LCK" sleep 12 & HOLDER=$!; sleep 0.4
  flock -n "$LCK" true 2>/dev/null && bad "LC-0 control needle: the lock is NOT held by the other process" || ok "LC-0 control needle: another process holds the lock"
  lcexp LC-2 "exec 9>>\"\$LCK\"; $RUN" refused "fd 9 open on the lock file (never locked) while ANOTHER process holds it: refused (N4: the proof must show fd 9's own file description holds it)"
  lcexp LC-3 "exec 9>&-; $RUN" refused "an ancestor with fd 9 closed while another process holds the lock: refused"
  lcexp LC-4 "exec 9>>\"\$OTHERF\"; flock 9; $RUN" refused "fd 9 open and locked on ANOTHER file while another process holds the register lock: refused (identity of fd 9's file)"
  kill "$HOLDER" 2>/dev/null; wait "$HOLDER" 2>/dev/null
  sleep 300 & OTHER_PID=$!; sleep 0.2
  OTHER=$OTHER_PID lcexp LC-5 'exec 9>>"$LCK"; flock 9; LOCKED_LOCK_HELD=$OTHER bash "$LK" --op-id lc -- true; echo rc=$?' refused "fd 9 really holds the lock but the claimed pid is a live process that is NOT an ancestor: refused"
  killpid "$OTHER_PID"; wait "$OTHER_PID" 2>/dev/null
  lcexp LC-6 "exec 9>>\"\$LCK\"; flock 9; $RUN" ok "golden-true: the ancestor whose fd 9 really holds the lock is accepted"
  [ "$(jn "$R/.audit/register/journal.jsonl")" = 1 ] && ok "LC-7 only the golden-true call was journaled (the five refusals wrote no row)" || bad "LC-7 journal rows: $(jn "$R/.audit/register/journal.jsonl")"
fi

# ================= RP: the reaper bound escalates to KILL =================
if want RP; then echo "== RP a reaper that ignores SIGTERM is still bounded =="
  sreset; R=$(sroot rp); echo "{\"run_id\":\"OTHER\",\"pid\":4194000}" >"$R/.audit/commit_turn.json"
  printf '#!/bin/sh\ntrap "" TERM\necho $$ >"%s/ign.pid"\nwhile :; do sleep 1; done\n' "$STUB_DIR" >"$STUB_DIR/reaper.sh"; chmod +x "$STUB_DIR/reaper.sh"
  t0=$(date +%s); out=$(CPA_HOST_ENTRY="$STUB_DIR/reaper.sh" LOCKED_REAPER_TIMEOUT=2 timeout 40 env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED_RUNP="$STUBRUNP" LOCKED_PODMAN="$STUBPODMAN" bash "$LOCKED" --op-id rp -- true 2>&1); rc=$?; dt=$(( $(date +%s)-t0 ))
  [ $rc -eq 20 ] && [ $dt -lt 25 ] && ok "RP-1 a reaper that ignores SIGTERM is killed after the grace (refused commit_turn_held after ${dt}s, not hung)" || bad "RP-1 rc=$rc dt=${dt}s [$out]"
  killpid "$(cat "$STUB_DIR/ign.pid" 2>/dev/null)"; wait 2>/dev/null
fi

# ================= B: backup_db.sh =================
if want B; then echo "== B backup_db.sh: die leaves no file; a busy checkpoint is refused =="
  R=$(mkroot b6); echo "x" >"$R/docs/workable_items.db"; : >"$R/.audit/out"
  out=$(env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" bash "$BACKUP" --record "$T_SCR/b6.rec" 2>&1); rc=$?
  left=$(ls "$R"/docs/workable_items.db.bak-* 2>/dev/null | wc -l)
  [ $rc -ne 0 ] && [ "$left" = 0 ] && ok "B-1 a failure after the backup file was pre-created (output directory not creatable) removes the 0-byte file (rc $rc, files left $left)" || bad "B-1 rc=$rc left=$left [$out]"
  if e2e; then
    R=$(mkroot b7); cdb "$R" workable_items.db || bad "B-2 setup"; lk "$R" -- sh -c "$(mint_cmd workable_items.db)" >/dev/null 2>&1
    python3 -I - "$R/docs/workable_items.db" >"$T_SCR/b7.reader" <<'P' &
import sqlite3,sys,time
c=sqlite3.connect("file:"+sys.argv[1]+"?mode=ro",uri=True,isolation_level=None); c.execute("BEGIN"); n=c.execute("select count(*) from reg_ids").fetchone()[0]
print("reader holds %d reg_ids rows"%n, flush=True); time.sleep(40); c.execute("COMMIT"); c.close()
P
    RP=$!; sleep 2
    lk "$R" -- sh -c "$(mint_cmd workable_items.db)" >/dev/null 2>&1   # a committed write the reader's snapshot blocks from being checkpointed
    out=$(tool "$R" "$BACKUP" --record "$T_SCR/b7.rec" 2>&1); rc=$?
    left=$(ls "$R"/docs/workable_items.db.bak-* 2>/dev/null | wc -l)
    [ $rc -ne 0 ] && printf '%s' "$out" | grep -q 'checkpoint' && [ ! -e "$T_SCR/b7.rec" ] && [ "$left" = 0 ] && ok "B-2 a checkpoint that cannot complete (reader open) refuses the backup: its source_sha256 would name a state without the WAL pages (no record, no file)" || bad "B-2 rc=$rc rec=$([ -e "$T_SCR/b7.rec" ] && echo y || echo n) left=$left [$out]"
    kill "$RP" 2>/dev/null; wait "$RP" 2>/dev/null
    out=$(tool "$R" "$BACKUP" --record "$T_SCR/b7b.rec" 2>&1); rc=$?
    [ $rc -eq 0 ] && [ -s "$T_SCR/b7b.rec" ] && ok "B-3 golden-true: with no reader the same register backs up" || bad "B-3 rc=$rc [$out]"
  fi
fi

# ================= RC: reconcile.sh checks the header of an EMPTY view too =================
if want RC; then echo "== RC reconcile.sh empty view header =="
  DBF="$T_SCR/rc.db"; fresh_ext_db "$DBF" || bad "RC setup"
  OUTD="$T_SCR/rc_out"; out=$(bash "$RECONCILE" --db "$DBF" --out-dir "$OUTD" 2>&1); rc=$?
  [ $rc -eq 0 ] && [ "$(head -n1 "$OUTD/unmapped_entries.csv")" = "entry_id,source,locator" ] && ok "RC-1 golden-true: a register whose views are all empty writes the hand-kept headers (they match the views)" || bad "RC-1 rc=$rc [$out]"
  sed 's/\[reopen_counts\]="atm_id,reopens"/[reopen_counts]="atm_id,reopen_count"/' "$RECONCILE" >"$T_SCR/reconcile_drift.sh"
  cmp -s "$RECONCILE" "$T_SCR/reconcile_drift.sh" && bad "RC-2 fixture: the drifted copy equals the script"
  rm -rf "$OUTD"; out=$(bash "$T_SCR/reconcile_drift.sh" --db "$DBF" --out-dir "$OUTD" 2>&1); rc=$?
  [ $rc -eq 4 ] && printf '%s' "$out" | grep -q 'header of view reopen_counts' && [ ! -e "$OUTD/reopen_counts.csv" ] && ok "RC-2 an EMPTY view whose hand-kept header drifted from the view's columns is exit 4 naming the header, no stale CSV shipped" || bad "RC-2 rc=$rc csv=$(cat "$OUTD/reopen_counts.csv" 2>/dev/null | head -1) [$out]"
fi

# ================= T3: the mutation runner scores an environmental failure as NOT killed =================
if want T3; then echo "== T3 mut_score =="
  assert_eq "T3-1 a passing run is survived" "$(mut_score 0 '')" survived
  assert_eq "T3-2 a failing run with a test FAIL line is killed" "$(mut_score 1 'FAIL F3-1 the lock was free')" killed
  assert_eq "T3-3 a failing run whose FAIL line is a disk headroom refusal is env, never killed" "$(mut_score 1 'FAIL x run_pinned: disk_below_headroom')" env
  assert_eq "T3-4 a failing run with no FAIL line at all (crash, kill) is env, never killed" "$(mut_score 2 '')" env
  assert_eq "T3-5 image lock unreadable is env" "$(mut_score 1 'FAIL lock_unreadable')" env
fi

# ================= T4: a zero-byte id snapshot is a failure, not an empty table =================
if want T4; then echo "== T4 zero-byte id snapshot =="
  sreset; R=$(sroot t4); : >"$STUB_DIR/snap.1"; : >"$STUB_DIR/snap.2"
  slk "$R" --op-id t4 -- sh -c 'true' >/dev/null 2>&1
  assert_eq "T4-1 both snapshots answer with zero bytes and exit 0: ids_snapshot is failed (sqlite3 always prints a row terminator)" "$(jrow "$R/.audit/register/journal.jsonl" t4 ids_snapshot)" '"failed"'
fi

# ================= T5: the replay fallback base is not taken while a -wal was pending before the first row =================
if want T5; then echo "== T5 replay fallback base with wal_bytes_before =="
  H2="$T_SCR/rh2"; mkdir -p "$H2/root/docs" "$H2/stubs" "$H2/j"; echo "x" >"$H2/root/docs/workable_items.db"
  cat >"$H2/stubs/backup.sh" <<'S'
#!/usr/bin/env bash
cp docs/workable_items.db docs/bak; s=$(sha256sum docs/bak | cut -d' ' -f1); printf '{"backup_path":"docs/bak","backup_sha256":"%s","source_sha256":"%s"}' "$s" "$s" > "$2"
S
  cat >"$H2/stubs/locked.sh" <<'S'
#!/usr/bin/env bash
case "$*" in *"select atm_id from reg_ids"*) echo "CAT-001";; *gate.sh*) echo "GATE OK";; esac; exit 0
S
  printf '#!/bin/sh\nexit 0\n' >"$H2/stubs/noop.sh"; chmod +x "$H2/stubs/"*.sh
  A=$(printf 'a%.0s' $(seq 64)); B=$(printf 'b%.0s' $(seq 64))
  t5() { python3 -I -c '
import json,sys
a,b,w=sys.argv[2:5]
r={"op_id":"FIRST","mode":"register","db":"docs/workable_items.db","db_sha_before":a,"db_sha_after":b,"ids_minted":[],"argv":["sqlite3","/src/docs/workable_items.db","X"],"exit":0,"input_args":{},"inputs":[],"wal_bytes_after":0,"wal_bytes_before":int(w),"ids_snapshot":"ok"}
open(sys.argv[1],"w").write(json.dumps(r)+"\n")' "$H2/j/journal.jsonl" "$A" "$B" "$1"
    rm -rf "$H2/out"; env LOCKED_TEST_MODE=1 LOCKED_ROOT="$H2/root" LOCKED="$H2/stubs/locked.sh" BACKUP="$H2/stubs/backup.sh" DUMP="$H2/stubs/noop.sh" EXPORT="$H2/stubs/noop.sh" bash "$REPLAY" --onto docs/workable_items.db --since "$A" --out "$H2/out" --journal "$H2/j/journal.jsonl" 2>&1 | cut -c1-300; }
  r=$(t5 0); case "$r" in "replay: OK replayed=1 "*) ok "T5-1 control: the first register row has db_sha_before = --since and an empty -wal before it: it is the fallback base's successor and is replayed";; *) bad "T5-1 [$r]";; esac
  r=$(t5 4152); case "$r" in *"reason=since_not_found"*) ok "T5-2 the same row with wal_bytes_before=4152 (a write pending in the -wal file before it) is NOT a base: since_not_found";; *) bad "T5-2 [$r]";; esac
fi

# ================= REAL: the F3 class on real podman and the real locked.sh (T1) =================
if want REAL && e2e; then echo "== REAL signals / SIGPIPE / SIGKILL with a real container =="
  realroot() { local r; r=$(mkroot "$1"); cdb "$r" workable_items.db || return 1; lk "$r" -- sh -c "$(mint_cmd workable_items.db)" >/dev/null 2>&1 || return 1; echo "$r"; }
  title_sql() { printf '%s' "sqlite3 /src/docs/workable_items.db \"UPDATE items SET title='$1' WHERE atm_id='CAT-001'\""; }
  ctrs() { podman ps -a -q --filter "label=catalogizer.op_id=$1" 2>/dev/null | wc -l; }
  wait_ctr_gone() { local i; for i in $(seq 1 200); do [ "$(ctrs "$1")" = 0 ] && return 0; sleep 0.2; done; return 1; }
  # ---- REAL-TERM ----
  R=$(realroot rterm) || { bad "REAL-TERM setup"; R=""; }
  if [ -n "$R" ]; then LCK="$R/docs/.register.lock"
    W1="touch /src/docs/first.started; sleep 7; $(title_sql FIRST-LATE); touch /src/docs/first.finished"
    lk "$R" --op-id first-term -- sh -c "$W1" >"$T_SCR/rt.out" 2>&1 & P=$!
    for i in $(seq 1 300); do [ -e "$R/docs/first.started" ] && break; sleep 0.1; done
    if [ -e "$R/docs/first.started" ] && [ "$P" -gt 1 ] && tr '\0' ' ' </proc/$P/cmdline 2>/dev/null | grep -q locked.sh && [ "$(awk '{print $4}' /proc/$P/stat 2>/dev/null)" = "$$" ]; then kill -s TERM "$P"; else bad "REAL-TERM fixture: writer not started or pid $P is not our locked.sh"; fi
    ( lk "$R" --op-id second-term -- sh -c "$(title_sql SECOND); touch /src/docs/second.done" >"$T_SCR/rt2.out" 2>&1; echo $? >"$T_SCR/rt2.rc" ) &
    free=0; for i in $(seq 1 400); do [ -e "$R/docs/first.finished" ] && break; lockfree "$LCK" && free=$((free+1)); sleep 0.05; done
    wait
    T=$(lk "$R" --out "$T_SCR/q1" -- sqlite3 -readonly "file:/src/docs/workable_items.db?immutable=1" "select title from items where atm_id='CAT-001'" 2>/dev/null)
    J="$R/.audit/register/journal.jsonl"
    [ "$free" = 0 ] && [ "$T" = SECOND ] && [ "$(jrow "$J" first-term interrupted)" = '"TERM"' ] && ok "REAL-TERM SIGTERM to locked.sh with a real container: lock never free while the first container ran ($free free samples), the second writer ran after it (final title $T), journal interrupted=TERM" || bad "REAL-TERM free=$free title=$T interrupted=$(jrow "$J" first-term interrupted)"
    [ "$(jrow "$J" second-term db_sha_before)" = "$(jrow "$J" first-term db_sha_after)" ] && ok "REAL-TERM-b the journal chains: the second row's db_sha_before equals the first row's db_sha_after (the first row was taken after its container's write)" || bad "REAL-TERM-b first after=$(jrow "$J" first-term db_sha_after) second before=$(jrow "$J" second-term db_sha_before)"
  fi
  # ---- REAL-PIPE ----
  R=$(realroot rpipe) || { bad "REAL-PIPE setup"; R=""; }
  if [ -n "$R" ]; then LCK="$R/docs/.register.lock"
    WR="echo first-line; sleep 2; i=0; while [ \$i -lt 20000 ]; do echo filler-\$i; i=\$((i+1)); done; sleep 6; $(title_sql PIPE-LATE); touch /src/docs/pipe.finished"
    ( lk "$R" --op-id pipe-op -- sh -c "$WR" 2>"$T_SCR/pipe.err"; echo "$? $(date +%s.%N)" >"$T_SCR/pipe.rc" ) | head -n1 >"$T_SCR/pipe.head" &
    for i in $(seq 1 600); do [ -s "$T_SCR/pipe.rc" ] && break; [ -e "$R/docs/pipe.finished" ] && break; sleep 0.05; done
    freebefore=no; [ ! -e "$R/docs/pipe.finished" ] && lockfree "$LCK" && freebefore=yes
    for i in $(seq 1 400); do [ -e "$R/docs/pipe.finished" ] && [ -s "$T_SCR/pipe.rc" ] && break; sleep 0.1; done; wait
    J="$R/.audit/register/journal.jsonl"; FSHA=$(sha256sum "$R/docs/workable_items.db" | cut -d' ' -f1)
    # locked.sh must not have ended before the write landed: its end time is after the marker mtime
    python3 -I - "$R/docs/pipe.finished" "$T_SCR/pipe.rc" <<'PY' && lend=ok || lend=early
import os,sys
fin=os.stat(sys.argv[1]).st_mtime; end=float(open(sys.argv[2]).read().split()[1])
sys.exit(0 if end>=fin else 1)
PY
    [ "$(cat "$T_SCR/pipe.head")" = first-line ] && [ "$freebefore" = no ] && [ "$lend" = ok ] && [ "$(jrow "$J" pipe-op db_sha_after)" = "\"$FSHA\"" ] && ok "REAL-PIPE 'locked.sh | head -n1' with a real container: head read first-line, the lock was held until the write landed, locked.sh ended after it, the journal row's db_sha_after is the final state (drain=$(jrow "$J" pipe-op container_drain))" || bad "REAL-PIPE head=[$(cat "$T_SCR/pipe.head")] lock_free_before_write=$freebefore locked_end=$lend row_after=$(jrow "$J" pipe-op db_sha_after) final=$FSHA rc=$(cat "$T_SCR/pipe.rc" 2>/dev/null)"
    [ ! -e "$R/.audit/register/pending/pipe-op" ] && ok "REAL-PIPE-b no pending marker is left (the op was journaled after the container was gone)" || bad "REAL-PIPE-b pending marker left"
  fi
  # ---- REAL-KILL ----
  R=$(realroot rkill) || { bad "REAL-KILL setup"; R=""; }
  if [ -n "$R" ]; then
    W1="touch /src/docs/first.started; sleep 7; $(title_sql FIRST-LATE); touch /src/docs/first.finished"
    lk "$R" --op-id first-kill -- sh -c "$W1" >"$T_SCR/rk.out" 2>&1 & P=$!
    for i in $(seq 1 300); do [ -e "$R/docs/first.started" ] && break; sleep 0.1; done
    if [ -e "$R/docs/first.started" ] && [ "$P" -gt 1 ] && tr '\0' ' ' </proc/$P/cmdline 2>/dev/null | grep -q locked.sh && [ "$(awk '{print $4}' /proc/$P/stat 2>/dev/null)" = "$$" ]; then kill -s KILL "$P"; else bad "REAL-KILL fixture: writer not started or pid $P is not our locked.sh"; fi
    wait "$P" 2>/dev/null
    ( lk "$R" --op-id second-kill -- sh -c "$(title_sql SECOND); touch /src/docs/second.done" >"$T_SCR/rk2.out" 2>&1; echo $? >"$T_SCR/rk2.rc" ) &
    wait
    wait_ctr_gone first-kill
    T=$(lk "$R" --out "$T_SCR/q3" -- sqlite3 -readonly "file:/src/docs/workable_items.db?immutable=1" "select title from items where atm_id='CAT-001'" 2>/dev/null)
    python3 -I - "$R/docs/first.finished" "$R/docs/second.done" <<'PY' && ord=ok || ord=overlap
import os,sys
a=os.stat(sys.argv[1]).st_mtime; b=os.stat(sys.argv[2]).st_mtime
sys.exit(0 if b>a else 1)
PY
    [ "$ord" = ok ] && [ "$T" = SECOND ] && [ "$(cat "$T_SCR/rk2.rc")" = 0 ] && ok "REAL-KILL SIGKILL of locked.sh: the NEXT locked.sh waited for the dead writer's container (second write after the first, final title $T): no overlap" || bad "REAL-KILL order=$ord title=$T second rc=$(cat "$T_SCR/rk2.rc" 2>/dev/null) out=[$(head -c 200 "$T_SCR/rk2.out")]"
    [ -e "$R/.audit/register/pending/first-kill" ] && ok "REAL-KILL-b the killed writer's pending marker is left (its write is not journaled: the documented residual limit, replay refuses)" || bad "REAL-KILL-b no pending marker for the killed writer"
  fi
fi

finish
