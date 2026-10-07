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
# Signals and the writer container (WF10 F3, WF13 N2/N3, rebuilt by WF15 I1-I3 M1): the writer is the script, the podman CLIENT (RUNP's last process) and the CONTAINER (held by
# conmon, it outlives a client that dies of SIGPIPE, OOM or a crash); every decision uses an identity covering all three. CLIENT: the pending marker is opened on fd 8 and flocked before
# RUNP is forked and RUNP inherits fd 8 (conmon and the container never do), so the marker is locked while this script or an orphaned client lives. CONTAINER: `podman ps` filtered by the
# label catalogizer.op_id=<id> and by the /src mount source of THIS root (another checkout's container with the same id is not ours). EXIT STATUS: the `died` event podman recorded for the
# container (podman wait can lose a --rm container between two polls); the row says container_exit_source container_event | container_wait | client | client_unverified | unknown (unknown: exit 22, marker kept).
# TERM INT HUP USR1 USR2 ALRM QUIT received while the RUNP call runs are forwarded to that ONE call's pid (a child of this shell, proven ours through /proc before every kill, never a pid <= 1,
# never a process group); the handlers stay installed to the LAST line (a signal during the drain, the after-snapshot or the journal append is remembered, never fatal between a landed write and
# its row) and the after-snapshot and the journal writer ignore a terminal Ctrl-C; SIGPIPE is ignored by this script (set after the fork). After the client is gone the container is waited for up
# to LOCKED_CONTAINER_WAIT seconds (1800), then removed (container_drain=killed, exit 137); podman unable to answer = container_drain=unverified. The operation is journaled only after the
# container is gone, with "container_drain" none|waited|killed|unverified and "interrupted":"<SIG>" when a signal arrived; the exit status is the writer's real one (128+n when the writer ended 0
# after a signal). The writer does not inherit fd 9 (the lock). A SIGKILL of this script cannot be trapped: the NEXT call fences it (every pending marker: wait until its flock can be taken AND
# no container of that op and root exists, else 20 writer_still_running / writer_state_unverifiable; no process name is trusted); the killed write is not journaled and its marker stays (OWED-WP06-14).
# Journal safety (WF10 F5): the journal must be writable BEFORE the writer starts (else 20 journal_unwritable, nothing written); a register or scratch write
# first creates .audit/register/pending/<op_id> (O_EXCL), removed only after its journal row is on disk (replay.sh refuses while one exists); a journal append
# that fails after the write exits 21 (REFUSED reason=journal_append_failed) and keeps the marker. An argument file that cannot be copied is 20 input_capture_failed.
# LOCKED_LOCK_HELD (WF10 F6, WF13 N4) is a PROOF, not a switch: its value is the pid of the ancestor locked.sh that holds the lock; it is honoured only when that pid
# is an ancestor of this process, fd 9 is open on this root's docs/.register.lock, the lock cannot be taken by a fresh open (someone holds it) and `flock -n 9` succeeds
# (fd 9's own open file description holds it); else 20 lock_claim_invalid.
# The reaper call (commit-turn freeze) and the id snapshots run with fd 9 closed; the reaper is bounded by `timeout -k 2` (60 s then KILL, LOCKED_REAPER_TIMEOUT in test mode).
# Id snapshots (WF10 F4): ids_snapshot is "ok" only when BOTH snapshots answered; "failed" when a snapshot call failed or answered nothing; "unavailable" for
# a pending -wal file or an --out run. replay.sh refuses a planned register row whose snapshot is not "ok" (replay_ids_unknown).
# Journal row fields added: seq (strictly increasing under the lock), wal_bytes_before, interrupted (only when a signal arrived).
# Exit: the command's status (the container's own, from its died event); 2 usage; 20 refusal (REFUSED reason=<code>, also writer_still_running / writer_state_unverifiable from the fence); 21 journal append failed; 22 the container's exit status could not be read (container_exit_source=unknown); other statuses of RUNP are passed through.
# Test hooks (refused unless LOCKED_TEST_MODE=1): LOCKED_ROOT (scratch repository root), LOCKED_RUNP, LOCKED_BACKUP, LOCKED_REAPER_TIMEOUT, LOCKED_PODMAN, LOCKED_CONTAINER_WAIT, LOCKED_EVENT_WAIT (seconds to wait for the died event).
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
if [ -n "${LOCKED_ROOT+x}${LOCKED_RUNP+x}${LOCKED_BACKUP+x}${LOCKED_REAPER_TIMEOUT+x}${LOCKED_PODMAN+x}${LOCKED_CONTAINER_WAIT+x}${LOCKED_EVENT_WAIT+x}" ] && [ "${LOCKED_TEST_MODE:-}" != 1 ]; then   # MUT:test-hooks
  refuse test_hook_outside_test_mode "LOCKED_ROOT / LOCKED_RUNP / LOCKED_BACKUP / LOCKED_REAPER_TIMEOUT / LOCKED_PODMAN / LOCKED_CONTAINER_WAIT / LOCKED_EVENT_WAIT are test hooks and need LOCKED_TEST_MODE=1"
fi
REAPER_TIMEOUT=60; if [ "${LOCKED_TEST_MODE:-}" = 1 ]; then case "${LOCKED_REAPER_TIMEOUT:-60}" in ''|*[!0-9]*|0|0*) refuse test_hook_invalid "LOCKED_REAPER_TIMEOUT must be a positive integer";; *) REAPER_TIMEOUT="${LOCKED_REAPER_TIMEOUT:-60}";; esac; fi
CWAIT=1800; PODMAN=podman; EVWAIT=20   # the bound on waiting for a writer container (seconds), the podman binary, the bound on waiting for the container's died event (seconds)
if [ "${LOCKED_TEST_MODE:-}" = 1 ]; then
  case "${LOCKED_CONTAINER_WAIT:-1800}" in ''|*[!0-9]*|0|0*) refuse test_hook_invalid "LOCKED_CONTAINER_WAIT must be a positive integer";; *) CWAIT="${LOCKED_CONTAINER_WAIT:-1800}";; esac
  PODMAN="${LOCKED_PODMAN:-podman}"
  case "${LOCKED_EVENT_WAIT:-20}" in ''|*[!0-9]*|0|0*) refuse test_hook_invalid "LOCKED_EVENT_WAIT must be a positive integer";; *) EVWAIT="${LOCKED_EVENT_WAIT:-20}";; esac
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
# ---------- writer lifecycle: the helpers of the lock, the fence and the drain (WF15 round 4: defined BEFORE the lock so a mutant cannot move the fence above it by anything but its call) ----------
# The WRITER is three things with ONE identity (the op id): the shell script, the podman CLIENT (RUNP's last process) and the CONTAINER (held by conmon, it outlives the client).
# Every decision below derives from an identity that covers the writer's whole life, never from one of the three alone (class C2, WF15 I1/I3):
#   - the client is proven alive or gone by the flock on its pending marker (fd 8, inherited by RUNP, never by conmon),
#   - the container is proven present or gone by `podman ps` filtered by its label AND by the /src mount of THIS repository root (WF15 M1),
#   - the container's exit status is the `died` event podman recorded (WF15 I2): `podman wait` cannot be used, a `--rm` container can be removed between two polls.
shield() { ( trap '' TERM INT HUP QUIT; exec "$@" ); }   # a terminal Ctrl-C reaches the whole foreground group: the helpers that must finish (after-snapshot, journal append) ignore it
ctr_ids() {  # ctr_ids <op_id>: prints the ids of the containers of THIS root labelled with the op id; status 1 when podman could not answer
  local o id src keep=""
  o="$(timeout 30 "$PODMAN" ps -a -q --no-trunc --filter "label=catalogizer.op_id=$1" 2>/dev/null </dev/null 9>&- 8>&-)" || return 1
  for id in $o; do
    src="$(timeout 30 "$PODMAN" inspect --format '{{range .Mounts}}{{if eq .Destination "/src"}}{{.Source}}{{end}}{{end}}' "$id" 2>/dev/null </dev/null 9>&- 8>&-)" || src=""   # unanswerable: conservatively ours (the next poll converges)
    if [ -n "$src" ] && [ "$(realpath -m -- "$src" 2>/dev/null)" != "$ROOT_REAL" ]; then continue; fi   # MUT:root-scope  # another checkout's container carrying the same op id
    keep="$keep$id"$'\n'
  done
  printf '%s' "$keep"
}
DRAIN=none; CRC=""; SEEN=""; WCODE=""; EXSRC=client; T0=""; EV_OK=0
ev_status() {  # ev_status: sets EV_N (died events of this op since T0, restricted to the containers seen when any was seen) and EV_CODE; status 1 when podman could not answer
  local o id code; EV_N=0; EV_CODE=""
  o="$(timeout 30 "$PODMAN" events --since "$T0" --until "$(( $(date +%s) + 5 ))" --stream=false --filter "label=catalogizer.op_id=$OP_ID" --filter event=died --format '{{.ID}} {{.ContainerExitCode}}' 2>/dev/null </dev/null 9>&- 8>&-)" || return 1
  while read -r id code; do
    case "$code" in ''|*[!0-9]*) continue;; esac
    if [ -n "${SEEN// /}" ]; then case " $SEEN " in *" $id "*) ;; *) continue;; esac; fi
    EV_N=$((EV_N+1)); EV_CODE="$code"
  done <<<"$o"
  return 0
}
ev_resolve() {  # ev_resolve <need 0|1> [<max seconds>]: ev_status, repeated for up to EVWAIT (or the given) seconds while a status is NEEDED (a container was seen or the client died of a signal) and no died event is readable yet:
  # the journald events backend can lag behind a busy host (observed: seconds); sets EV_OK (1 = the last query answered), EV_N, EV_CODE
  local mx="${2:-$EVWAIT}" end; [ "$mx" -le "$EVWAIT" ] || mx="$EVWAIT"; end=$(( $(date +%s) + mx ))
  while :; do
    if ev_status; then EV_OK=1; else EV_OK=0; EV_N=0; EV_CODE=""; fi
    [ "$1" = 1 ] || return 0
    [ "$EV_OK" = 1 ] && [ "$EV_N" -ge 1 ] && return 0   # one event = the status, several = ambiguous: waiting longer does not change that
    [ "$(date +%s)" -lt "$end" ] || return 0
    sleep 0.5
  done
}
drain_containers() {  # drain_containers <op_id>: returns when no container of the op exists. Sets DRAIN (none|waited|killed|unverified), SEEN (ids seen)
  local op="$1" ids end k=0 id w rem
  DRAIN=none; CRC=""; SEEN=""; WCODE=""; end=$(( $(date +%s) + CWAIT ))
  while :; do
    if ! ids="$(ctr_ids "$op")"; then
      k=$((k+1)); [ "$k" -lt 3 ] || { DRAIN=unverified; echo "locked: WARNING cannot ask podman whether a container of $op is still running; the write is not proven finished (container_drain=unverified)" >&2; return 0; }
      sleep 1; continue
    fi
    k=0; ids="$(printf '%s' "$ids" | tr -s '\n' ' ')"
    [ -n "${ids// /}" ] || return 0
    SEEN="$SEEN $ids"
    [ "$DRAIN" = killed ] || DRAIN=waited
    if [ "$(date +%s)" -ge "$end" ] && [ "$DRAIN" != killed ]; then
      echo "locked: container of $op still running after ${CWAIT}s: removing it (container_drain=killed)" >&2
      # shellcheck disable=SC2086
      timeout 30 "$PODMAN" rm -f -t 0 $ids >/dev/null 2>&1 </dev/null 9>&- 8>&-; DRAIN=killed; CRC=137; end=$(( $(date +%s) + 30 ))
    elif [ "$DRAIN" = killed ] && [ "$(date +%s)" -ge "$end" ]; then DRAIN=unverified; echo "locked: WARNING the container of $op could not be removed" >&2; return 0
    fi
    # a second source of the status, valid only when it answers: ONE wait for the whole remaining budget (not 5 s slices: a slice that ends exactly when the container ends loses the status,
    # WF15 I2), with a short poll interval because a --rm container is removed shortly after it exits
    rem=$(( end - $(date +%s) )); [ "$rem" -ge 1 ] || rem=1
    for id in $ids; do w="$(timeout -k 2 "$rem" "$PODMAN" wait --interval 20ms "$id" 2>/dev/null </dev/null 9>&- 8>&-)" && case "$w" in ''|*[!0-9]*) ;; *) [ "$DRAIN" = killed ] || WCODE="$w";; esac; done
    sleep 0.2
  done
}
fence_orphans() {  # a pending marker is an orphan write when its client is gone or its container still runs: wait until BOTH are gone before this call writes
  local m op end ids held
  for m in "$AUD"/pending/*; do
    [ -f "$m" ] || continue
    op="${m##*/}"; end=$(( $(date +%s) + CWAIT ))
    while :; do
      if ! ( exec 7<"$m" ) 2>/dev/null; then [ -e "$m" ] || break; refuse writer_state_unverifiable "pending marker $m cannot be opened"; fi
      held=0; ( exec 7<"$m"; flock -n 7 ) 2>/dev/null || held=1   # MUT:marker-lock  # the lock on the marker is held by every process that holds the writer's client fd 8: a live locked.sh or an orphaned podman client
      ids="$(ctr_ids "$op")" || refuse writer_state_unverifiable "pending marker $m names an op whose writer container could not be looked up (podman did not answer)"
      [ "$held" = 0 ] && [ -z "${ids//[[:space:]]/}" ] && break
      [ "$(date +%s)" -lt "$end" ] || refuse writer_still_running "the writer of pending op $op (its locked.sh is gone; client alive=$held, container ids=[${ids//[[:space:]]/ }]) is still running after ${CWAIT}s; a second writer would overlap it"
      sleep 0.5
    done
  done
}
# ---------- lock ----------
proc_ppid() { local st; st="$(cat "/proc/$1/stat" 2>/dev/null)" || return 1; st="${st##*) }"; set -- $st; printf '%s' "$2"; }
lock_claim_proven() {  # F6: LOCKED_LOCK_HELD names an ancestor pid that really holds docs/.register.lock through the fd 9 this process inherited
  local h="${LOCKED_LOCK_HELD:-}" p="$$" i=0
  case "$h" in ''|*[!0-9]*) return 1;; esac
  [ "$h" -gt 1 ] || return 1
  [ -e "/proc/$$/fd/9" ] && [ "$(stat -L -c %d:%i "/proc/$$/fd/9" 2>/dev/null)" = "$(stat -c %d:%i docs/.register.lock 2>/dev/null)" ] || return 1
  while [ "$i" -lt 64 ]; do p="$(proc_ppid "$p")" || return 1; case "$p" in ''|*[!0-9]*) return 1;; esac; [ "$p" -gt 1 ] || return 1; [ "$p" = "$h" ] && break; i=$((i+1)); done
  [ "$p" = "$h" ] || return 1
  ( exec 8>>docs/.register.lock && ! flock -n 8 ) || return 1   # the lock is held by SOMEONE: a fresh open file description cannot take it
  flock -n 9   # ... and by US: fd 9's own open file description holds it (re-locking a lock the description already holds succeeds; if another description holds it this fails; if nobody did, the line above already refused)
}
NESTED=0
if [ -n "${LOCKED_LOCK_HELD:-}" ]; then
  NESTED=1
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
    timeout -k 2 "$REAPER_TIMEOUT" "${CPA_HOST_ENTRY:-$HOME/.local/bin/cpa-host}" --exec-approved scripts/release/commit_turn_check.sh --reap >/dev/null 2>&1 </dev/null 9>&- || true   # MUT:reap-direct
    if held; then refuse commit_turn_held "a commit turn is held by another run (reap attempted once)"; fi
  fi
fi
# a SIGKILLed predecessor leaves a pending marker and possibly a running container: wait for it (after the commit-turn freeze, which makes no podman call)
if [ "$MODE" != out ] && [ "$NESTED" = 0 ]; then fence_orphans; fi   # MUT:fence
# ---------- database snapshots ----------
dbsha() { [ -f "$1" ] && sha256sum -- "$1" | cut -d' ' -f1 || echo null; }
snap_ids() {  # ids of reg_ids, read-only through the immutable URI inside the image; "" = an empty table; ?wal = pending -wal; ?fail = the call failed or answered nothing
  [ -f "$1" ] || return 0
  [ ! -s "$1-wal" ] || { echo "?wal"; return 0; }
  local tmp="$AUD/.snap.$$" rc
  shield env DISK_HEADROOM_OUT_DIR=".audit/out/$OP_ID/disk/" "$RUNP" --op-id "$OP_ID-snap" "$IMG" -- sqlite3 -readonly "file:/src/$1?immutable=1" \
    "select group_concat(atm_id,' ') from (select atm_id from reg_ids order by seq)" >"$tmp" 2>/dev/null </dev/null 9>&- 8>&-
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
RC=0; RUNP_PID=""; INTR=""; CLIENT_RC=""
our_child() {  # our_child <pid>: a positive pid > 1 that /proc shows is a direct child of this shell
  case "${1:-}" in ''|*[!0-9]*) return 1;; esac
  [ "$1" -gt 1 ] || return 1
  [ "$(proc_ppid "$1")" = "$$" ]
}
forward_signal() {  # forward_signal <SIG>: remember the signal, hand it to the ONE RUNP call (proven ours); this script never dies of it
  [ -n "$INTR" ] || INTR="$1"
  if our_child "$RUNP_PID"; then kill -s "$1" "$RUNP_PID" 2>/dev/null; fi   # MUT:signal-forward
}
for sg in TERM INT HUP USR1 USR2 ALRM QUIT; do trap "forward_signal $sg" "$sg"; done   # MUT:signal-set
sigstatus() { printf '%s' "$((128 + $(kill -l "$1")))"; }
PEND=""
if [ "$MODE" != out ]; then
  PEND="$AUD/pending/$OP_ID"
  ( set -o noclobber; printf '{"op_id":"%s","pid":%s}\n' "$OP_ID" "$$" >"$PEND" ) 2>/dev/null || refuse pending_marker_failed "cannot create $PEND (an operation with this id is pending or the directory is not writable)"   # MUT:pending-marker
  # the client's liveness (WF15 I1): fd 8 is an open file description of the marker with the flock held; RUNP inherits it (the writer's podman CLIENT keeps it until it ends, conmon and the
  # container never receive it), so the NEXT call's fence finds the marker locked while this script is alive OR its orphaned client still runs, and free only when both are gone
  { exec 8<"$PEND"; } 2>/dev/null && flock -n 8 || { rm -f -- "$PEND"; refuse pending_marker_failed "cannot lock $PEND"; }   # MUT:marker-fd
fi
if [ -n "$INTR" ]; then [ -z "$PEND" ] || rm -f -- "$PEND"; echo "locked: interrupted by SIG$INTR before the writer started; nothing was written" >&2; exit "$(sigstatus "$INTR")"; fi
T0="$(date +%s.%N)"   # the `died` event of the writer's container is looked up from here (before the fork: no earlier event of the same op id can match)
env ${DISK_HEADROOM_OUT_DIR:+DISK_HEADROOM_OUT_DIR="$DISK_HEADROOM_OUT_DIR"} "$RUNP" "${RARGS[@]}" --op-id "$OP_ID" "$IMG" -- "${CMD[@]}" </dev/null 9>&- &
RUNP_PID=$!
trap '' PIPE   # set AFTER the fork: the client keeps its default SIGPIPE; this script must not die of a closed pipe (a reader such as `| head -n1`) while the writer's container still runs
[ -z "$INTR" ] || forward_signal "$INTR"   # a signal that arrived between the fork and the pid assignment is forwarded now
# wait for the client; `wait` returns early when a trapped signal arrives (the child is then still ours: wait again). The status is read from `wait` itself, never
# preset: bash may already have reaped a fast-exiting child before the first test, and `wait` still reports its status (WF13 N5).
while :; do wait "$RUNP_PID"; RC=$?; our_child "$RUNP_PID" || break; done   # MUT:wait-loop
CLIENT_RC=$RC; RUNP_PID=""   # the pid is not ours any more: a signal from here on is only remembered (INTR), never forwarded to a reused pid
# the client is gone, the writer may not be: keep the lock until no container of this op exists
drain_containers "$OP_ID"   # MUT:drain
# the writer's real exit status is the `died` event podman recorded for its container (WF15 I2); the client's own status stands only when no container ever died under this label
# (RUNP refused or failed before the container, or it ended normally with nothing outliving it) and, when neither the event nor a `podman wait` answer can be read, is NOT guessed:
# exit 22, container_exit_source=unknown, the pending marker is kept
EXSRC=client
if [ "$DRAIN" != killed ] && [ "$DRAIN" != unverified ]; then
  NEED=0; [ "$DRAIN" = waited ] && NEED=1; [ "$CLIENT_RC" -ge 128 ] 2>/dev/null && NEED=1   # a container outlived the client, or the client died of a signal: its own status is not the writer's
  if [ -n "$WCODE" ]; then ev_resolve 0; elif [ "$DRAIN" = none ]; then ev_resolve "$NEED" 3; else ev_resolve "$NEED"; fi   # a status that podman wait already gave is not waited for twice; with no container ever seen only a short look (the client may simply have been signalled)
  if [ "$EV_OK" = 1 ] && [ "$EV_N" = 1 ]; then CRC="$EV_CODE"; EXSRC=container_event
  elif [ -n "$WCODE" ]; then CRC="$WCODE"; EXSRC=container_wait
  elif [ "$NEED" = 0 ]; then EXSRC=client
  elif [ "$EV_OK" = 1 ] && [ "$EV_N" -gt 1 ]; then if [ "$CLIENT_RC" = 0 ]; then EXSRC=client; else EXSRC=unknown; fi   # several containers carry this op id (another checkout): a client that ended 0 saw its container end 0, any other status is not attributable
  elif [ "$DRAIN" = none ]; then EXSRC=client_unverified   # the client died of a signal, no container was ever seen and no died event exists: its own status stands, flagged as unproven
  else EXSRC=unknown; fi
fi
if [ "$EXSRC" = unknown ]; then echo "locked: WARNING the exit status of the writer container of $OP_ID could not be read (container_exit_source=unknown): exit 22, the write is not proven finished" >&2; CRC=22; fi   # MUT:status-unknown
if [ -n "$CRC" ] && [ "$CRC" != "$RC" ]; then RC="$CRC"; fi   # MUT:container-status
if [ "$MODE" = out ]; then AFTER_SHA="$(dbsha "$DB")"; AFTER_IDS="?out"; else AFTER_SHA="$(dbsha "$DB")"; AFTER_IDS="$(snap_ids "$DB")"; fi
# ---------- journal ----------
WALB=0; [ -f "$DB-wal" ] && WALB="$(stat -c %s -- "$DB-wal")"
shield python3 -I - "$JOURNAL" "$OP_ID" "$MODE" "$RC" "$BEFORE_SHA" "$AFTER_SHA" "$BEFORE_IDS" "$AFTER_IDS" "$DB" "$WALB" "$WALB_BEFORE" "$INTR" "${INPUTS[*]:-}" "${INPUTARGS[*]:-}" "$DRAIN" "$CLIENT_RC" "$EXSRC" -- "${CMD[@]}" 8>&- <<'PY'
import json, sys, datetime, os
a = sys.argv[1:]; i = a.index("--")
jr, op, mode, rc, bs, as_, bi, ai, db, walb, walb0, intr, inputs, inargs, drain, crc, exsrc = a[:17]; argv = a[i+1:]
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
    row["container_drain"] = drain
    row["container_exit_source"] = exsrc
    if crc != "" and int(crc) != int(rc): row["client_exit"] = int(crc)
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
if [ "$EXSRC" != unknown ]; then case "$DRAIN" in none|waited) [ -z "$PEND" ] || rm -f -- "$PEND";; *) echo "locked: the pending marker $PEND is kept: the write of $OP_ID is not proven finished (container_drain=$DRAIN)" >&2;; esac   # a killed or unverified writer may have written part of its work: replay refuses
else echo "locked: the pending marker $PEND is kept: the write of $OP_ID is not proven finished (container_exit_source=unknown)" >&2; fi
# the signal handlers stay installed up to HERE (WF15 I3): a signal during the after-snapshot or the journal append is remembered (INTR), never fatal between the landed write and its row
trap - TERM INT HUP USR1 USR2 ALRM QUIT
if [ -n "$INTR" ] && [ "$RC" -eq 0 ]; then RC="$(sigstatus "$INTR")"; fi
exit "$RC"   # MUT:exit
