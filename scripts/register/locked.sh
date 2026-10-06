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
# Exit: the command's status; 2 usage; 20 refusal (REFUSED reason=<code>); other statuses of RUNP are passed through.
# Test hooks (refused unless LOCKED_TEST_MODE=1): LOCKED_ROOT (scratch repository root), LOCKED_RUNP, LOCKED_BACKUP.
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
if [ -n "${LOCKED_ROOT+x}${LOCKED_RUNP+x}${LOCKED_BACKUP+x}" ] && [ "${LOCKED_TEST_MODE:-}" != 1 ]; then   # MUT:test-hooks
  refuse test_hook_outside_test_mode "LOCKED_ROOT / LOCKED_RUNP / LOCKED_BACKUP are test hooks and need LOCKED_TEST_MODE=1"
fi
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
if [ -z "${LOCKED_LOCK_HELD:-}" ]; then
  exec 9>>docs/.register.lock || refuse lock_unopenable "docs/.register.lock"
  flock 9   # MUT:flock
fi
export LOCKED_LOCK_HELD=1
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
    "${CPA_HOST_ENTRY:-$HOME/.local/bin/cpa-host}" --exec-approved scripts/release/commit_turn_check.sh --reap >/dev/null 2>&1 || true   # MUT:reap-direct
    if held; then refuse commit_turn_held "a commit turn is held by another run (reap attempted once)"; fi
  fi
fi
# ---------- database snapshots ----------
dbsha() { [ -f "$1" ] && sha256sum -- "$1" | cut -d' ' -f1 || echo null; }
snap_ids() {  # ids of reg_ids, read-only through the immutable URI inside the image; empty when unavailable
  [ -f "$1" ] || return 0
  [ ! -s "$1-wal" ] || { echo "?wal"; return 0; }
  env DISK_HEADROOM_OUT_DIR=".audit/out/$OP_ID/disk/" "$RUNP" --op-id "$OP_ID-snap" "$IMG" -- sqlite3 -readonly "file:/src/$1?immutable=1" \
    "select group_concat(atm_id,' ') from (select atm_id from reg_ids order by seq)" 2>/dev/null </dev/null | tr -d '\r'
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
mkdir -p -- "$AUD/inputs"
declare -a INPUTS=()
capture() {
  local a="$1" idx="${2:-}" p cand
  case "$a" in ".read "*) a="${a#.read }";; esac
  case "$a" in /src/*) p="${a#/src/}";; /*|""|-*) return 0;; *) p="$a";; esac   # MUT:capture-rel
  case "$p" in *.db|*.db-wal|*.db-shm|*.sqlite|*.sqlite3) return 0;; esac
  cand="$(realpath -m -- "$ROOT_REAL/$p")" || return 0
  case "$cand" in "$ROOT_REAL"/*) ;; *) return 0;; esac
  [ -f "$cand" ] || return 0
  local h; h="$(sha256sum -- "$cand" | cut -d' ' -f1)"
  [ -f "$AUD/inputs/$h" ] || { cp -- "$cand" "$AUD/inputs/.tmp.$$" && mv -f -- "$AUD/inputs/.tmp.$$" "$AUD/inputs/$h"; }   # MUT:capture-src
  INPUTS+=("$h"); [ -z "$idx" ] || INPUTARGS+=("$idx:$h")
}
INPUTARGS=(); ai=0
for a in "${CMD[@]}"; do capture "$a" "$ai"; ai=$((ai+1)); done
[ "$SUB" != import-sql ] || capture "$IMPORT"
# ---------- the one RUNP call ----------
RC=0
env ${DISK_HEADROOM_OUT_DIR:+DISK_HEADROOM_OUT_DIR="$DISK_HEADROOM_OUT_DIR"} "$RUNP" "${RARGS[@]}" --op-id "$OP_ID" "$IMG" -- "${CMD[@]}" </dev/null 9>&-
RC=$?
if [ "$MODE" = out ]; then AFTER_SHA="$(dbsha "$DB")"; AFTER_IDS="?out"; else AFTER_SHA="$(dbsha "$DB")"; AFTER_IDS="$(snap_ids "$DB")"; fi
# ---------- journal ----------
WALB=0; [ -f "$DB-wal" ] && WALB="$(stat -c %s -- "$DB-wal")"
python3 -I - "$JOURNAL" "$OP_ID" "$MODE" "$RC" "$BEFORE_SHA" "$AFTER_SHA" "$BEFORE_IDS" "$AFTER_IDS" "$DB" "$WALB" "${INPUTS[*]:-}" "${INPUTARGS[*]:-}" -- "${CMD[@]}" <<'PY'
import json, sys, datetime, os
a = sys.argv[1:]; i = a.index("--")
jr, op, mode, rc, bs, as_, bi, ai, db, walb, inputs, inargs = a[:12]; argv = a[i+1:]
bset = bi.split() if not bi.startswith("?") else None; aset = ai.split() if not ai.startswith("?") else None
minted = sorted(set(aset) - set(bset)) if bset is not None and aset is not None else []
row = {"time": datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%S.%fZ"), "op_id": op, "mode": mode, "argv": argv,
       "stdin": None, "inputs": inputs.split(), "input_args": dict(x.split(":", 1) for x in inargs.split()), "exit": int(rc), "db": db, "db_sha_before": None if bs == "null" else bs,
       "db_sha_after": None if as_ == "null" else as_, "wal_bytes_after": int(walb), "ids_snapshot": "ok" if (bset is not None and aset is not None) else "unavailable",
       "ids_minted": minted}
os.makedirs(os.path.dirname(jr), exist_ok=True)
fd = os.open(jr, os.O_WRONLY | os.O_APPEND | os.O_CREAT, 0o600)
os.write(fd, (json.dumps(row, sort_keys=True) + "\n").encode()); os.close(fd)
PY
exit "$RC"   # MUT:exit
