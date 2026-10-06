#!/usr/bin/env bash
# locked.sh - T064 (docs/04 section 12.2): the single-writer wrapper of the register. A host control-plane launcher like RUNP.
# Usage:  locked.sh [--out <dir>] [--op-id <id>] -- <command word>...        run one command inside IMG-TESTUTIL under the register lock
#         locked.sh [--op-id <id>] import-sql <file.sql>                      import a SQL file (T165): register DB, or LOCKED_SCRATCH_DB
#         locked.sh -h
# What it does, in order, with the lock `flock docs/.register.lock` (never tracked, T004) held on the HOST around exactly one RUNP call:
#   1. commit-turn freeze (register writes only): .audit/commit_turn.json naming a run_id other than EVREC_TURN_RUN_ID is first reaped once
#      through `"${CPA_HOST_ENTRY:-$HOME/.local/bin/cpa-host}" --exec-approved scripts/release/commit_turn_check.sh --reap`, re-read, and if
#      still held (or unreadable, or not JSON) the call is REFUSED commit_turn_held (20): no podman call, no journal row.
#   2. one `scripts/containers/run_pinned.sh --rw docs IMG-TESTUTIL -- <command>` call (RUNP): the register is written by the image's
#      `sqlite3` and the engine binary, never by a host sqlite3. With --out <dir> the call is `RUNP --out <dir> IMG-TESTUTIL -- <command>`:
#      no --rw docs, so the command can write only the /out directory. A scratch import composes `--rw .audit/scratch` and never `--rw docs`.
#   3. one row appended to the ignored journal .audit/register/journal.jsonl: time, op_id, mode, argv, exit status, database sha256 before
#      and after, ids minted (reg_ids diff), and every argument that names an existing file copied to .audit/register/inputs/<sha256>
#      (a path relative to the repository root on the host, or a container path /src/<p> read as <p>; `.read <path>` arguments too;
#      database files (*.db) are never copied; the row also records input_args, argument index -> sha256, so a replay can re-bind them). This journal is the input of the register replay (scripts/register/replay.sh, T067a).
# The exit status of the wrapped command is passed through. RUNP passes no stdin into the container: stdin redirection into locked.sh is
# UNSUPPORTED (nothing is read from it; a fixture pipes SQL and sees no row written).
# import-sql <file>: <file> is a repository-relative path under .audit/out/; its sibling `<dir>/<stem>.sha256` (stem = file name without .sql) is
# ONE line of 64 lowercase hex digits and no file name; sha256 of <file> is compared to it as two strings (import_sha256_mismatch /
# import_sha256_malformed), then `sqlite3 /src/<DB> ".read /src/<file>"` runs. <DB> is docs/workable_items.db, or LOCKED_SCRATCH_DB (honoured
# only by import-sql, else 20 scratch_db_subcommand_invalid) whose canonical form (realpath -m on the HOST) must lie under
# realpath -m <toplevel>/.audit/scratch (else 20 scratch_db_path_invalid; the file itself must not be a symlink). A register import takes the
# pre-op backup first through scripts/register/backup_db.sh --record <out>/backup.json (T064a) while this lock is held (LOCKED_LOCK_HELD
# is exported so the helper's own locked.sh calls do not wait on the lock they inherit); a scratch import takes none and sets
# DISK_HEADROOM_OUT_DIR=.audit/out/<op_id>/disk/ so the disk-headroom writer never refuses commit_turn_held for it.
# Signals (WF10 F3): SIGTERM, SIGINT and SIGHUP received while the RUNP call runs are forwarded to that ONE call's pid (a child of this shell, proven ours
# through /proc before every kill, never a pid <= 1, never a process group) and the register lock stays held until that process is gone; the operation is then
# journaled with "interrupted":"<SIG>" and the writer's real exit status, and this script exits with that status (128+n when the writer ended 0). The writer
# does not inherit fd 9 (the lock); a SIGKILL of this script cannot be trapped, so the pending marker below is the only trace it leaves.
# Journal safety (WF10 F5): the journal must be writable BEFORE the writer starts (else 20 journal_unwritable, nothing written); a register or scratch write
# first creates .audit/register/pending/<op_id> (O_EXCL), removed only after its journal row is on disk (replay.sh refuses while one exists); a journal append
# that fails after the write exits 21 (REFUSED reason=journal_append_failed) and keeps the marker. An argument file that cannot be copied is 20 input_capture_failed.
# LOCKED_LOCK_HELD (WF10 F6) is a PROOF, not a switch: its value is the pid of the ancestor locked.sh that holds the lock; it is honoured only when that pid
# is an ancestor of this process, fd 9 is open on this root's docs/.register.lock, and the lock cannot be taken by a fresh open (else 20 lock_claim_invalid).
# The reaper call (commit-turn freeze) and the id snapshots run with fd 9 closed; the reaper is bounded by `timeout` (60 s, LOCKED_REAPER_TIMEOUT in test mode).
# Id snapshots (WF10 F4): ids_snapshot is "ok" only when BOTH snapshots answered; "failed" when a snapshot call failed or answered nothing; "unavailable" for
# a pending -wal file or an --out run. replay.sh refuses a planned register row whose snapshot is not "ok" (replay_ids_unknown).
# Journal row fields added: seq (strictly increasing under the lock), wal_bytes_before, interrupted (only when a signal arrived).
# Exit: the command's status; 2 usage; 20 refusal (REFUSED reason=<code>); 21 journal append failed; other statuses of RUNP are passed through.
# Test hooks (refused unless LOCKED_TEST_MODE=1): LOCKED_ROOT (scratch repository root), LOCKED_RUNP, LOCKED_BACKUP, LOCKED_REAPER_TIMEOUT.
set -u
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REAL_ROOT="$(cd "$SELF_DIR/../.." && pwd)"
refuse() { echo "locked: REFUSED reason=$1 ${2:-}" >&2; exit 20; }
usage() { cat >&2 <<'U'
locked: usage: locked.sh [--out <dir>] [--op-id <id>] -- <command word>...
locked:        locked.sh [--op-id <id>] import-sql <file.sql under .audit/out/>
locked: the command runs inside IMG-TESTUTIL via run_pinned.sh under the register lock; stdin is unsupported (nothing is read from it)
U
exit 2; }
if [ -n "${LOCKED_ROOT+x}${LOCKED_RUNP+x}${LOCKED_BACKUP+x}${LOCKED_REAPER_TIMEOUT+x}" ] && [ "${LOCKED_TEST_MODE:-}" != 1 ]; then   # MUT:test-hooks
  refuse test_hook_outside_test_mode "LOCKED_ROOT / LOCKED_RUNP / LOCKED_BACKUP / LOCKED_REAPER_TIMEOUT are test hooks and need LOCKED_TEST_MODE=1"
fi
REAPER_TIMEOUT=60; if [ "${LOCKED_TEST_MODE:-}" = 1 ]; then case "${LOCKED_REAPER_TIMEOUT:-60}" in ''|*[!0-9]*|0|0*) refuse test_hook_invalid "LOCKED_REAPER_TIMEOUT must be a positive integer";; *) REAPER_TIMEOUT="${LOCKED_REAPER_TIMEOUT:-60}";; esac; fi
ROOT="${LOCKED_ROOT:-$REAL_ROOT}"
RUNP="${LOCKED_RUNP:-$REAL_ROOT/scripts/containers/run_pinned.sh}"
BACKUP="${LOCKED_BACKUP:-$SELF_DIR/backup_db.sh}"
IMG=IMG-TESTUTIL
OUT=""; OP_ID=""; SUB=""
while [ $# -gt 0 ]; do case "$1" in
  --) shift; break;;
  -h|--help) usage;;
  --out) [ $# -ge 2 ] || usage; [ -z "$OUT" ] || usage; OUT="$2"; shift 2;;
  --op-id) [ $# -ge 2 ] || usage; OP_ID="$2"; shift 2;;
  import-sql) SUB=import-sql; shift; break;;
  -*) echo "locked: unknown option '$1'" >&2; usage;;
  *) echo "locked: unexpected argument '$1' before --" >&2; usage;;
esac; done
if [ "$SUB" = import-sql ]; then [ $# -eq 1 ] || usage; IMPORT="$1"; else [ $# -ge 1 ] || usage; fi
[ -n "$OP_ID" ] || OP_ID="locked-$(date -u +%Y%m%dT%H%M%S)-$$-$RANDOM"
case "$OP_ID" in ''|*[!A-Za-z0-9._-]*|.*) usage;; esac
if [ -n "${LOCKED_SCRATCH_DB:-}" ] && [ "$SUB" != import-sql ]; then refuse scratch_db_subcommand_invalid "LOCKED_SCRATCH_DB is honoured only by import-sql"; fi   # MUT:scratch-subcommand
[ -d "$ROOT" ] || refuse root_missing "$ROOT"
cd "$ROOT" || refuse root_missing "$ROOT"
ROOT_REAL="$(realpath -- "$ROOT")"
mkdir -p -- docs .audit 2>/dev/null
AUD=".audit/register"; JOURNAL="$AUD/journal.jsonl"
MODE=register; [ -z "$OUT" ] || MODE=out
REGDB=docs/workable_items.db; DB="$REGDB"; SCRATCH=0
if [ "$SUB" = import-sql ]; then
  MODE=register
  case "$IMPORT" in /*|*..*|-*|"") refuse import_path_invalid "$IMPORT";; esac
  case "$IMPORT" in .audit/out/*.sql) ;; *) refuse import_path_invalid "$IMPORT is not a .sql file under .audit/out/";; esac
  [ -f "$IMPORT" ] && [ ! -L "$IMPORT" ] || refuse import_path_invalid "$IMPORT is not a regular file"
  if [ -n "${LOCKED_SCRATCH_DB:-}" ]; then
    SCRATCH=1; MODE=scratch
    base="$(realpath -m -- "$ROOT_REAL/.audit/scratch")"
    canon="$(realpath -m -- "$LOCKED_SCRATCH_DB")"
    [ -L "$LOCKED_SCRATCH_DB" ] && refuse scratch_db_path_invalid "the scratch database is a symlink"
    case "$canon" in "$base"/*) ;; *) refuse scratch_db_path_invalid "$canon is not under $base";; esac   # MUT:scratch-canonical
    DB="${canon#"$ROOT_REAL"/}"
  fi
fi
# ---------- lock ----------
proc_ppid() { local st; st="$(cat "/proc/$1/stat" 2>/dev/null)" || return 1; st="${st##*) }"; set -- $st; printf '%s' "$2"; }
lock_claim_proven() {  # F6: LOCKED_LOCK_HELD names an ancestor pid that really holds docs/.register.lock through the fd 9 this process inherited
  local h="${LOCKED_LOCK_HELD:-}" p="$$" i=0
  case "$h" in ''|*[!0-9]*) return 1;; esac
  [ "$h" -gt 1 ] || return 1
  [ -e "/proc/$$/fd/9" ] && [ "$(stat -L -c %d:%i "/proc/$$/fd/9" 2>/dev/null)" = "$(stat -c %d:%i docs/.register.lock 2>/dev/null)" ] || return 1
  while [ "$i" -lt 64 ]; do p="$(proc_ppid "$p")" || return 1; case "$p" in ''|*[!0-9]*) return 1;; esac; [ "$p" -gt 1 ] || return 1; [ "$p" = "$h" ] && break; i=$((i+1)); done
  [ "$p" = "$h" ] || return 1
  ( exec 8>>docs/.register.lock && ! flock -n 8 )   # the lock is really held: a fresh open file description cannot take it
}
if [ -n "${LOCKED_LOCK_HELD:-}" ]; then
  [ -f docs/.register.lock ] || refuse lock_claim_invalid "docs/.register.lock does not exist"
  lock_claim_proven || refuse lock_claim_invalid "LOCKED_LOCK_HELD=$LOCKED_LOCK_HELD is not an ancestor locked.sh holding docs/.register.lock on fd 9"   # MUT:lock-claim
else
  exec 9>>docs/.register.lock || refuse lock_unopenable "docs/.register.lock"
  flock 9   # MUT:flock
fi
export LOCKED_LOCK_HELD=$$
# journal safety (F5): the journal and its directories must be writable before anything runs
mkdir -p -- "$AUD/inputs" "$AUD/pending" 2>/dev/null || refuse journal_unwritable "$AUD is not creatable"
if [ -e "$JOURNAL" ] || [ -L "$JOURNAL" ]; then { [ -f "$JOURNAL" ] && [ ! -L "$JOURNAL" ] && [ -w "$JOURNAL" ]; } || refuse journal_unwritable "$JOURNAL is not a writable regular file"; else [ -w "$AUD" ] || refuse journal_unwritable "$AUD is not writable"; fi   # MUT:journal-precheck
# ---------- commit-turn freeze (register writes only; never a scratch import, never a --out run) ----------
if [ "$MODE" = register ]; then
  grant_state() {  # sets GRANT_RUN to the held run_id, "" when no grant is held; returns 1 when the grant is unreadable
    GRANT_RUN=""; local g=.audit/commit_turn.json
    if [ ! -e "$g" ] && [ ! -L "$g" ]; then [ -d .audit ] && [ -x .audit ] && return 0; return 1; fi
    [ -r "$g" ] && [ -f "$g" ] || return 1
    local j; j="$(python3 -I -c 'import json,sys
d=json.load(open(sys.argv[1]))
r=d.get("run_id") if isinstance(d,dict) else None
sys.exit(3) if not isinstance(r,str) or not r else print(r)' "$g" 2>/dev/null)" || return 1
    GRANT_RUN="$j"; return 0; }
  held() { grant_state || return 0; [ -n "$GRANT_RUN" ] && [ "$GRANT_RUN" != "${EVREC_TURN_RUN_ID:-}" ]; }   # MUT:grant-ignored
  if held; then
    timeout "$REAPER_TIMEOUT" "${CPA_HOST_ENTRY:-$HOME/.local/bin/cpa-host}" --exec-approved scripts/release/commit_turn_check.sh --reap >/dev/null 2>&1 </dev/null 9>&- || true   # MUT:reap-direct
    if held; then refuse commit_turn_held "a commit turn is held by another run (reap attempted once)"; fi
  fi
fi
# ---------- database snapshots ----------
dbsha() { [ -f "$1" ] && sha256sum -- "$1" | cut -d' ' -f1 || echo null; }
snap_ids() {  # ids of reg_ids, read-only through the immutable URI inside the image; "" = an empty table; ?wal = pending -wal; ?fail = the call failed or answered nothing
  [ -f "$1" ] || return 0
  [ ! -s "$1-wal" ] || { echo "?wal"; return 0; }
  local tmp="$AUD/.snap.$$" rc
  env DISK_HEADROOM_OUT_DIR=".audit/out/$OP_ID/disk/" "$RUNP" --op-id "$OP_ID-snap" "$IMG" -- sqlite3 -readonly "file:/src/$1?immutable=1" \
    "select group_concat(atm_id,' ') from (select atm_id from reg_ids order by seq)" >"$tmp" 2>/dev/null </dev/null 9>&-
  rc=$?   # the status of the call itself (a pipeline would report its last stage, the footgun of docs/guides/shell_instrument_footguns.md I5)
  if [ "$rc" -ne 0 ] || [ ! -s "$tmp" ]; then rm -f "$tmp"; echo "?fail"; return 0; fi   # sqlite3 always prints a row terminator: a zero-byte answer is a failure, not an empty table
  tr -d '\r' <"$tmp" | tr -d '\n'; echo; rm -f "$tmp"
}
# ---------- compose ----------
if [ "$SUB" = import-sql ]; then
  stem="${IMPORT%.sql}"; sib="$stem.sha256"
  [ -f "$sib" ] || refuse import_sha256_malformed "sibling $sib is missing"
  [ "$(wc -l <"$sib")" -le 1 ] || refuse import_sha256_malformed "sibling $sib has more than one line"
  want="$(head -n1 "$sib")"
  case "$want" in *[!0-9a-f]*|"") refuse import_sha256_malformed "sibling $sib is not one bare lowercase hex hash";; esac
  [ "${#want}" -eq 64 ] || refuse import_sha256_malformed "sibling $sib is not 64 hex digits"
  have="$(sha256sum <"$IMPORT" | cut -d' ' -f1)"
  [ "$have" = "$want" ] || refuse import_sha256_mismatch "sha256 of $IMPORT is $have, its sibling holds $want"   # MUT:import-sha
  if [ "$SCRATCH" = 1 ]; then
    mkdir -p -- .audit/scratch
    export DISK_HEADROOM_OUT_DIR=".audit/out/$OP_ID/disk/"   # MUT:scratch-headroom
  else
    mkdir -p -- ".audit/out/$OP_ID"
    "$BACKUP" --record ".audit/out/$OP_ID/backup.json" || { echo "locked: backup helper failed; no import" >&2; exit 20; }   # MUT:import-backup
  fi
  CMD=(sqlite3 "/src/$DB" ".read /src/$IMPORT")
  RARGS=(); if [ "$SCRATCH" = 1 ]; then RARGS=(--rw .audit/scratch); else RARGS=(--rw docs); fi   # MUT:scratch-bind
else
  CMD=("$@")
  if [ "$MODE" = out ]; then RARGS=(--out "$OUT"); else RARGS=(--rw docs); fi   # MUT:out-bind
fi
if [ "$MODE" = out ]; then BEFORE_SHA="$(dbsha "$DB")"; BEFORE_IDS="?out"; else BEFORE_SHA="$(dbsha "$DB")"; BEFORE_IDS="$(snap_ids "$DB")"; fi   # an --out run cannot write docs/: no id snapshot
# argument capture: whole-argument paths (host-relative or /src/<p>) and `.read <path>`; databases are never copied
declare -a INPUTS=()
capture() {
  local a="$1" idx="${2:-}" p cand
  case "$a" in ".read "*) a="${a#.read }";; esac
  case "$a" in /src/*) p="${a#/src/}";; /*|""|-*) return 0;; *) p="$a";; esac   # MUT:capture-rel
  case "$p" in *.db|*.db-wal|*.db-shm|*.sqlite|*.sqlite3) return 0;; esac   # MUT:capture-db-skip
  cand="$(realpath -m -- "$ROOT_REAL/$p")" || return 0
  case "$cand" in "$ROOT_REAL"/*) ;; *) return 0;; esac
  [ -f "$cand" ] || return 0
  local h; h="$(sha256sum -- "$cand" | cut -d' ' -f1)"
  [ -f "$AUD/inputs/$h" ] || { cp -- "$cand" "$AUD/inputs/.tmp.$$" && mv -f -- "$AUD/inputs/.tmp.$$" "$AUD/inputs/$h"; } || refuse input_capture_failed "cannot copy $cand to $AUD/inputs/$h"   # MUT:capture-src
  INPUTS+=("$h"); [ -z "$idx" ] || INPUTARGS+=("$idx:$h")
}
INPUTARGS=(); ai=0
for a in "${CMD[@]}"; do capture "$a" "$ai"; ai=$((ai+1)); done
[ "$SUB" != import-sql ] || capture "$IMPORT"
WALB_BEFORE=0; [ -f "$DB-wal" ] && WALB_BEFORE="$(stat -c %s -- "$DB-wal")"
# ---------- the one RUNP call ----------
# A register or scratch write leaves a pending marker until its journal row is on disk (replay.sh refuses while one exists). The call runs as a child of
# this shell with fd 9 closed; TERM/INT/HUP are forwarded to that child (proven ours through /proc) and the lock is kept until it is gone.
RC=0; RUNP_PID=""; INTR=""
our_child() {  # our_child <pid>: a positive pid > 1 that /proc shows is a direct child of this shell
  case "${1:-}" in ''|*[!0-9]*) return 1;; esac
  [ "$1" -gt 1 ] || return 1
  [ "$(proc_ppid "$1")" = "$$" ]
}
forward_signal() {  # forward_signal <TERM|INT|HUP>
  [ -n "$INTR" ] || INTR="$1"
  if our_child "$RUNP_PID"; then kill -s "$1" "$RUNP_PID" 2>/dev/null; fi   # MUT:signal-forward
}
trap 'forward_signal TERM' TERM
trap 'forward_signal INT' INT
trap 'forward_signal HUP' HUP
PEND=""
if [ "$MODE" != out ]; then
  PEND="$AUD/pending/$OP_ID"
  ( set -o noclobber; printf '{"op_id":"%s","pid":%s}\n' "$OP_ID" "$$" >"$PEND" ) 2>/dev/null || refuse pending_marker_failed "cannot create $PEND (an operation with this id is pending or the directory is not writable)"   # MUT:pending-marker
fi
if [ -n "$INTR" ]; then [ -z "$PEND" ] || rm -f -- "$PEND"; echo "locked: interrupted by SIG$INTR before the writer started; nothing was written" >&2; case "$INTR" in TERM) exit 143;; INT) exit 130;; *) exit 129;; esac; fi
env ${DISK_HEADROOM_OUT_DIR:+DISK_HEADROOM_OUT_DIR="$DISK_HEADROOM_OUT_DIR"} "$RUNP" "${RARGS[@]}" --op-id "$OP_ID" "$IMG" -- "${CMD[@]}" </dev/null 9>&- &
RUNP_PID=$!
[ -z "$INTR" ] || forward_signal "$INTR"   # a signal that arrived between the fork and the pid assignment is forwarded now
while our_child "$RUNP_PID"; do wait "$RUNP_PID"; RC=$?; done   # a trapped signal ends `wait` early with the child alive: the loop waits again
trap - TERM INT HUP
if [ "$MODE" = out ]; then AFTER_SHA="$(dbsha "$DB")"; AFTER_IDS="?out"; else AFTER_SHA="$(dbsha "$DB")"; AFTER_IDS="$(snap_ids "$DB")"; fi
# ---------- journal ----------
WALB=0; [ -f "$DB-wal" ] && WALB="$(stat -c %s -- "$DB-wal")"
python3 -I - "$JOURNAL" "$OP_ID" "$MODE" "$RC" "$BEFORE_SHA" "$AFTER_SHA" "$BEFORE_IDS" "$AFTER_IDS" "$DB" "$WALB" "$WALB_BEFORE" "$INTR" "${INPUTS[*]:-}" "${INPUTARGS[*]:-}" -- "${CMD[@]}" <<'PY'
import json, sys, datetime, os
a = sys.argv[1:]; i = a.index("--")
jr, op, mode, rc, bs, as_, bi, ai, db, walb, walb0, intr, inputs, inargs = a[:14]; argv = a[i+1:]
def ids(x): return None if x.startswith("?") else x.split()
bset, aset = ids(bi), ids(ai)
minted = sorted(set(aset) - set(bset)) if bset is not None and aset is not None else []
if bset is not None and aset is not None: snap = "ok"
elif bi == "?fail" or ai == "?fail": snap = "failed"
else: snap = "unavailable"
try:
    seq = 1
    if os.path.exists(jr):
        with open(jr, "rb") as f: seq = sum(1 for _ in f) + 1
    row = {"seq": seq, "time": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%fZ"), "op_id": op, "mode": mode, "argv": argv,
           "stdin": None, "inputs": inputs.split(), "input_args": dict(x.split(":", 1) for x in inargs.split()), "exit": int(rc), "db": db, "db_sha_before": None if bs == "null" else bs,
           "db_sha_after": None if as_ == "null" else as_, "wal_bytes_before": int(walb0), "wal_bytes_after": int(walb), "ids_snapshot": snap, "ids_minted": minted}
    if intr: row["interrupted"] = intr
    os.makedirs(os.path.dirname(jr), exist_ok=True)
    data = (json.dumps(row, sort_keys=True) + "\n").encode()
    fd = os.open(jr, os.O_WRONLY | os.O_APPEND | os.O_CREAT, 0o600)
    try:
        n = os.write(fd, data)
        if n != len(data): raise OSError("short write %d of %d" % (n, len(data)))
        os.fsync(fd)
    finally: os.close(fd)
except Exception as e:
    print("locked: JOURNAL_APPEND_FAILED %s: %s" % (type(e).__name__, e), file=sys.stderr); sys.exit(3)
PY
JRC=$?
if [ "$JRC" -ne 0 ]; then echo "locked: REFUSED reason=journal_append_failed the write of $OP_ID ran (exit $RC) but its journal row could not be written; its pending marker $PEND is left" >&2; exit 21; fi
[ -z "$PEND" ] || rm -f -- "$PEND"
if [ -n "$INTR" ] && [ "$RC" -eq 0 ]; then case "$INTR" in TERM) RC=143;; INT) RC=130;; *) RC=129;; esac; fi
exit "$RC"   # MUT:exit
