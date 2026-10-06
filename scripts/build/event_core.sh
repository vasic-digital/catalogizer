#!/usr/bin/env bash
# event_core.sh - T005b SLICE: verification, replay guard, ordering and exactly-once terminal claim for
# build-event/1 (contracts/build-event.schema.json). NOT the dispatcher, hub, runner or emitter (owed).
# Usage:
#   event_core.sh secret-init     <statedir> <checkout>     create the driver secret (0600), refuse inside the checkout
#   event_core.sh derive-key      <statedir> <build_id> <run_id>   (prints the key: hub-to-stdin use only, never log it)
#   event_core.sh consume         <builds_root> <statedir> <event.json>
#   event_core.sh cancel          <builds_root> <build_id>
#   event_core.sh resume-callback <builds_root> <build_id>   re-run a callback found claimed, running or failed
# Build dir: <builds_root>/<build_id>/{submit.json,callback.json,events.jsonl,consumed/<seq>,progress.json,tmp/,.lock,.cblock,
#   terminal/{state.json,callback.state,callback.reason},terminal.tmp-<pid>-<start-time>/ (T005b: prepared, then renamed onto terminal/),
#   effects/<key>,effects.log}. Exit 0 consumed|DUP|late_ignored|superseded, 20 refused (reason=...), 21 callback not done.
# The event file is read ONCE (by bev_crypto.py verify-event); every field acted on comes from the verified object.
# Sourcing this file (for tests) defines the functions and runs nothing.
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
CRYPTO="$here/lib/bev_crypto.py"
CHECKOUT=$(cd "$here/../.." && pwd)
refuse() { printf 'REFUSED reason=%s %s\n' "$1" "${2:-}" >&2; exit 20; }
refuse_l() { flock -u 9 2>/dev/null; refuse "$@"; }      # refuse while holding the per-build lock
now() { date -u +%Y-%m-%dT%H:%M:%SZ; }
starttime() { awk '{print $22}' /proc/$$/stat 2>/dev/null || echo 0; }
read_regular() { # file : print a regular, non-symlink file of at most 4096 bytes; anything else (FIFO, device, directory, symlink, dangling, oversize) fails. Opened O_NONBLOCK|O_NOFOLLOW and judged by fstat, so a swap after a check cannot block
  python3 -c 'import os, stat, sys
fd = os.open(sys.argv[1], os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)
st = os.fstat(fd)
if not stat.S_ISREG(st.st_mode) or st.st_size > 4096: sys.exit(1)
sys.stdout.buffer.write(os.read(fd, 4097))' "$1" 2>/dev/null; }
regular_or_absent() { # file : 0 when the path is absent or a regular non-symlink file; 1 for anything else (FIFO, device, directory, symlink, dangling link). Judged by fstat of an O_NONBLOCK|O_NOFOLLOW open, so it can never block
  python3 -c 'import errno, os, stat, sys
try: fd = os.open(sys.argv[1], os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)
except OSError as e: sys.exit(0 if e.errno == errno.ENOENT else 1)
sys.exit(0 if stat.S_ISREG(os.fstat(fd).st_mode) else 1)' "$1" 2>/dev/null; }
isint() { [[ ${1:-} =~ ^(0|[1-9][0-9]{0,15})$ ]]; }       # base-10, at most 2^53 in practice (the verifier bounds it)
# Real fsync(2) through python os.fsync: `sync FILE` on this host is uutils coreutils = a GLOBAL sync(2) that has no error to report.
FSYNC_PY='import errno, os, sys
d = sys.argv[1] == "dir"
try:
    fd = os.open(sys.argv[2], os.O_RDONLY | (os.O_DIRECTORY if d else 0))
    try: os.fsync(fd)
    finally: os.close(fd)
except OSError as e:
    sys.exit(0 if d and e.errno in (errno.EINVAL, errno.ENOTSUP) else 1)'
fsync_dir() { python3 -c "$FSYNC_PY" dir "$1" 2>/dev/null; }    # CHECKED; only EINVAL/ENOTSUP (a filesystem without directory sync) is not a failure
fsync_file() { python3 -c "$FSYNC_PY" file "$1" 2>/dev/null; }  # CHECKED: a failed one is a failed write
# Every writer below checks its exit status and fails closed: a temp file that was not completely written and synced is
# removed, never renamed over durable state, and the failure is returned to the caller (which refuses; nothing is acked).
journal() { # builddir json-line : ONE write(2) of line+newline (O_APPEND, never through a symlink), fsync of the file and the directory.
  # Opened O_NONBLOCK and judged by fstat: a FIFO (with or without a reader) or any non-regular file is a failed journal, never a blocked one (round 6, EC-2).
  # A short write or a failed fsync is ROLLED BACK (truncate to the previous size) so the journal stays one JSON object per line; a torn last
  # line left by a crash is repaired (cut back to the last newline) before the next append. The caller holds the per-build lock.
  printf '%s' "$2" | python3 -c 'import os, stat, sys
p = sys.argv[1]
d = sys.stdin.buffer.read() + b"\n"
fd = os.open(p, os.O_WRONLY | os.O_NONBLOCK | os.O_APPEND | os.O_CREAT | os.O_NOFOLLOW, 0o644)
st = os.fstat(fd)
if not stat.S_ISREG(st.st_mode): sys.exit(1)
size = st.st_size
if size:
    rfd = os.open(p, os.O_RDONLY | os.O_NONBLOCK | os.O_NOFOLLOW)
    if os.pread(rfd, 1, size - 1) != b"\n":
        size = os.pread(rfd, size, 0).rfind(b"\n") + 1
        os.ftruncate(fd, size)
    os.close(rfd)
try:
    ok = os.write(fd, d) == len(d)
    if ok:
        os.fsync(fd)
except OSError:
    ok = False
if not ok:
    try: os.ftruncate(fd, size)
    except OSError: pass
    sys.exit(1)' "$1/events.jsonl" 2>/dev/null || return 1
  fsync_dir "$1"; }
atomic_write() { # builddir file content   (temp lives in <builddir>/tmp, never beside the data)
  local d=$1 f=$2 t; mkdir -p "$d/tmp" 2>/dev/null || return 1; t="$d/tmp/w.$$.$RANDOM"
  { printf '%s\n' "$3" > "$t"; } 2>/dev/null && [ -s "$t" ] && fsync_file "$t" && mv -fT "$t" "$f" 2>/dev/null || { rm -f "$t"; return 1; }
  fsync_dir "$(dirname "$f")"; }
# lock fd opener: append-open (never truncates), refused when the path is not the regular file the fd points at (symlink swap)
lock_ok() { # fd path : after `exec N>>path` succeeded, the open file must be the inode that path names (lstat, no follow)
  [ "$(stat -L -c %i "/proc/$$/fd/$1" 2>/dev/null)" = "$(stat -c %i "$2" 2>/dev/null)" ]; }

cbstate() { read_regular "$1/terminal/callback.state" | head -c 64 | tr -d '\n'; }   # read_regular: a FIFO there is "no state", never a blocked head (round 6, EC-2)
set_cbstate() { # builddir state [reason] : callback.state (and callback.reason) are their own files inside terminal/
  local d=$1
  if [ -n "${3:-}" ]; then atomic_write "$d" "$d/terminal/callback.reason" "$3" || return 1; fi
  atomic_write "$d" "$d/terminal/callback.state" "$2"; }

cb_fail() { # builddir reason : record failed + reason, release the callback lock, report
  set_cbstate "$1" failed "$2"; flock -u 8 2>/dev/null; echo "callback failed: $2" >&2; return 1; }
run_callback() { # builddir [redo] : idempotent, durable state in terminal/callback.state, effect under a callback lock. 0 = done (redo: 0 also when nothing was run)
  local d=$1 key lrc
  [ "$(cbstate "$d")" = done ] && return 0
  { exec 8>>"$d/.cblock"; } 2>/dev/null && lock_ok 8 "$d/.cblock" || { echo "REFUSED reason=lock_unavailable callback" >&2; return 1; }
  flock -w 60 8 || { echo "REFUSED reason=lock_unavailable callback" >&2; return 1; }
  # re-check under the lock: another runner may have finished the callback while this one waited
  if [ "$(cbstate "$d")" = done ]; then flock -u 8; return 0; fi
  # round 5, N3: a redo (DUP / superseded redelivery) only re-runs a callback found claimed or running; one the lock holder marked failed meanwhile is left to resume-callback
  if [ "${2:-}" = redo ]; then case "$(cbstate "$d")" in claimed|running) ;; *) flock -u 8; return 0;; esac; fi
  set_cbstate "$d" running || { flock -u 8; return 1; }
  # the callback record must be readable JSON carrying a string effect_key: anything else is a failed callback (never "done" without an effect)
  # (an empty file is "unreadable" explicitly: jq 1.6 exits 0 for empty input even with -e, and a missing key must never fall through to the key checks)
  [ -s "$d/callback.json" ] && jq -e . "$d/callback.json" >/dev/null 2>&1 || { cb_fail "$d" effect_unreadable; return 1; }
  key=$(jq -er '.effect_key | select(type == "string")' "$d/callback.json" 2>/dev/null) || { cb_fail "$d" effect_key_missing; return 1; }
  [ -n "$key" ] || { cb_fail "$d" effect_key_missing; return 1; }
  {
    [[ $key =~ ^[A-Za-z0-9_-]{1,128}$ ]] || { cb_fail "$d" effect_key_invalid; return 1; }
    # the first creation of effects/ must itself be durable in the build directory (checked), else the proof's directory entry may be lost
    # (round 5, N1: the build-directory fsync runs on EVERY attempt, not only the one that created effects/: a retry after a failed fsync must succeed at it before "done")
    if [ ! -d "$d/effects" ]; then { mkdir "$d/effects" 2>/dev/null || [ -d "$d/effects" ]; } || { cb_fail "$d" effects_dir_unavailable; return 1; }; fi
    fsync_dir "$d" || { cb_fail "$d" effects_dir_not_durable; return 1; }
    mkdir -p "$d/tmp" 2>/dev/null || { cb_fail "$d" effects_dir_unavailable; return 1; }
    # round 6, EC-2: effects.log is read (awk), appended to and fsynced below, all under the callback lock: only an absent or regular file is ever opened
    regular_or_absent "$d/effects.log" || { cb_fail "$d" effects_log_not_regular; return 1; }
    if [ ! -e "$d/effects/$key" ]; then
      # audit line first (once per key, literal whole-key match), then the keyed effect: a crash between the two is repaired on resume
      if ! awk -v k="$key" '$1 == k { f = 1 } END { exit !f }' "$d/effects.log" 2>/dev/null; then
        { printf '%s %s\n' "$key" "$(now)" >> "$d/effects.log"; } 2>/dev/null && fsync_file "$d/effects.log" || { cb_fail "$d" effects_log_write_failed; return 1; }
      else
        # round 6, EC-1: the line is already there (an earlier attempt wrote it, perhaps failing the fsync): it is durable only once THIS attempt's checked fsync passes
        fsync_file "$d/effects.log" || { cb_fail "$d" effects_log_write_failed; return 1; }
      fi
      { printf '%s\n' "$(now)" > "$d/tmp/e.$$"; } 2>/dev/null || { cb_fail "$d" effect_temp_write_failed; return 1; }
      ln "$d/tmp/e.$$" "$d/effects/$key" 2>/dev/null; lrc=$?
      rm -f "$d/tmp/e.$$"
      # EEXIST (a concurrent writer of the same key) is success; any other failure leaves no effect and must not become "done"
      if [ $lrc -ne 0 ] && [ ! -e "$d/effects/$key" ]; then cb_fail "$d" effect_not_applied; return 1; fi
    fi
    # the effect file is the proof: its durability gates "done", on EVERY attempt (round 6, EC-1: a retry that finds the effect already created must still pass this fsync)
    fsync_dir "$d/effects" || { cb_fail "$d" effect_not_durable; return 1; }
  }
  set_cbstate "$d" done || { flock -u 8; return 1; }
  rm -f "$d/terminal/callback.reason"
  flock -u 8
}

cb_redo() { # builddir : after a DUP or superseded ack of a completed event, a callback left claimed or running (a refusal after a won claim) is re-run; failed is left to resume-callback (retry decision owed)
  case "$(cbstate "$1")" in claimed|running) run_callback "$1" redo || true;; esac; }
terminal_ok() { # builddir : a terminal exists and its record parses
  [ -s "$1/terminal/state.json" ] && jq -e '.kind' "$1/terminal/state.json" >/dev/null 2>&1 && [ -s "$1/terminal/callback.state" ]; }
claim_terminal() { # builddir kind seq exit_class [digest] -> 0 this caller won, 1 lost the race (a good terminal exists), 2 failed, 3 won but the journal write failed
  local d=$1 kind=$2 seq=$3 ec=$4 dig=${5:-} tmpd
  tmpd="$d/terminal.tmp-$$-$(starttime)"
  rm -rf "$tmpd"; mkdir "$tmpd" 2>/dev/null || return 2
  { jq -n --arg k "$kind" --argjson s "${seq:-0}" --arg e "$ec" --arg t "$(now)" --arg g "$dig" \
      '{kind:$k,seq:$s,exit_class:$e,claimed_at:$t,digest:$g}' > "$tmpd/state.json" \
    && [ -s "$tmpd/state.json" ] && jq -e '.kind' "$tmpd/state.json" >/dev/null \
    && printf 'claimed\n' > "$tmpd/callback.state" && [ -s "$tmpd/callback.state" ] \
    && fsync_file "$tmpd/state.json" && fsync_file "$tmpd/callback.state"; } 2>/dev/null || { rm -rf "$tmpd"; return 2; }
  fsync_dir "$tmpd" || { rm -rf "$tmpd"; return 2; }
  if mv -T "$tmpd" "$d/terminal" 2>/dev/null; then
    fsync_dir "$d"
    journal "$d" "$(jq -nc --arg k "$kind" --argjson s "${seq:-0}" '{event:"terminal_claimed",kind:$k,seq:$s}')" || return 3
    return 0
  fi
  rm -rf "$tmpd"
  # the rename failed: that is a lost race ONLY if a whole terminal is there; any other cause is an error, never "superseded"
  if terminal_ok "$d"; then return 1; fi
  return 2
}

cmd_secret_init() {
  local sd checkout
  [[ $1 == /* ]] || refuse secret_dir_not_absolute "$1"                # an option-like or relative argument never names a state dir
  sd=$(realpath -m -- "$1"); checkout=$(realpath -m -- "${2:-$CHECKOUT}")
  case "$sd/" in "$checkout"/*) refuse secret_dir_inside_checkout "$sd";; esac
  case "$sd/" in "$CHECKOUT"/*) refuse secret_dir_inside_checkout "$sd";; esac   # this script's OWN checkout, whatever the argument says
  if [ -e "$sd/build_hmac.key" ] || [ -L "$sd/build_hmac.key" ]; then
    # an existing key is kept only if it is a usable one (regular file, ours, 0600, matching meta, 32 bytes)
    BEV_LIB="$here/lib" python3 - "$sd" "$CHECKOUT" <<'PY' || refuse secret_unusable "existing key"
import os, sys
sys.path.insert(0, os.environ["BEV_LIB"])
import bev_crypto
try: bev_crypto.secret(sys.argv[1], sys.argv[2])
except bev_crypto.SecretError: sys.exit(1)
PY
    echo "secret exists (kept)"; return 0
  fi
  ( umask 077
    mkdir -p "$sd" && chmod 700 "$sd" || exit 1
    k="$sd/build_hmac.key"; m="$k.meta"; t="$k.tmp-$$"; mt="$m.tmp-$$"
    trap 'rm -f "$t" "$mt"' EXIT
    openssl rand -hex 32 > "$t" || exit 1
    [[ "$(cat "$t")" =~ ^[0-9a-f]{64}$ ]] || exit 1
    fsync_file "$t" || exit 1
    mv -f "$t" "$k" || exit 1
    chmod 600 "$k" || exit 1
    h=$(sha256sum "$k" | cut -d' ' -f1); [[ $h =~ ^[0-9a-f]{64}$ ]] || exit 1
    printf 'created=%s\nsha256=%s\n' "$(now)" "$h" > "$mt" || exit 1
    fsync_file "$mt" || exit 1
    mv -f "$mt" "$m" || exit 1
    fsync_dir "$sd" || exit 1 ) || { rm -f "$sd/build_hmac.key" "$sd/build_hmac.key.meta" 2>/dev/null; refuse secret_create_failed "$sd"; }
  # report success only when the key we just wrote reads back as a usable 32-byte secret
  BEV_LIB="$here/lib" python3 - "$sd" "$CHECKOUT" <<'PY' || { rm -f "$sd/build_hmac.key" "$sd/build_hmac.key.meta" 2>/dev/null; refuse secret_create_failed "read-back"; }
import os, sys
sys.path.insert(0, os.environ["BEV_LIB"])
import bev_crypto
try: bev_crypto.secret(sys.argv[1], sys.argv[2])
except bev_crypto.SecretError: sys.exit(1)
PY
  echo "secret created"
}

cmd_consume() {
  local root=$1 sd=$2 ef=$3 out status ev dig b r seq kind ec po st d last lastprog laststage n f old crc pj
  [ -e "$ef" ] || refuse event_malformed "event file absent"
  out=$(python3 "$CRYPTO" verify-event "$sd" "$root" "$CHECKOUT" < "$ef") || refuse event_malformed "verifier failed"
  status=${out%%$'\n'*}
  case "$status" in
    ok) ;;
    other_run) refuse event_replayed "an event of another run of that build";;
    unknown_build) refuse event_unknown_build "";;
    malformed:*) refuse event_malformed "${status#malformed:}";;
    secret:*) refuse secret_unusable "${status#secret:}";;
    *) refuse event_unauthenticated "";;
  esac
  { read -r _; read -r ev; read -r dig; } <<<"$out"
  # every value below comes from the VERIFIED object, never from the file again
  IFS=$'\t' read -r b r seq kind ec po st < <(jq -r '[.build_id,.run_id,.seq,.kind,(.exit_class // "-"),(.progress_offset // -1),(.stage // -1)] | @tsv' <<<"$ev")
  isint "$seq" || refuse event_malformed "seq"
  # the build directory acted on must be the verified build_id byte for byte: a plain token (jq @tsv escapes LF/TAB/backslash, so a token
  # without those characters is exactly the verified string)
  [[ $b =~ ^[A-Za-z0-9_-]{1,128}$ ]] || refuse event_malformed "build_id"
  d="$root/$b"
  mkdir -p "$d/consumed" "$d/tmp"
  # ---- sequence section, serialised per build, held through the terminal claim ----
  { exec 9>>"$d/.lock"; } 2>/dev/null && lock_ok 9 "$d/.lock" || refuse lock_unavailable "$b"
  flock -w 60 9 || refuse lock_unavailable "$b"
  if [ -e "$d/consumed/$seq" ]; then
    old=$(read_regular "$d/consumed/$seq") || refuse_l state_corrupt "consumed/$seq"   # round 6, EC-2: a FIFO or oversize file there is damage, never a blocked cut under the lock
    old=${old%% *}
    if [ "$old" = "$dig" ]; then flock -u 9; echo "DUP seq=$seq acknowledged and dropped"; [ "$kind" = completed ] && cb_redo "$d"; return 0; fi
    refuse_l seq_conflict "seq=$seq already consumed with different content"
  fi
  if [ "$kind" = completed ] && [ -f "$d/terminal/state.json" ] && [ "$(jq -r '.kind' "$d/terminal/state.json")" = completed ] \
     && [ "$(jq -r '.seq' "$d/terminal/state.json")" = "$seq" ]; then
    if [ "$(jq -r '.digest // empty' "$d/terminal/state.json")" = "$dig" ]; then flock -u 9; echo "DUP seq=$seq acknowledged and dropped"; cb_redo "$d"; return 0; fi
    refuse_l seq_conflict "seq=$seq claimed with different content"
  fi
  if [ -d "$d/terminal" ]; then
    # a terminal directory that exists but is damaged is not a decided build: refuse (as cancel does), never ack the event as late
    terminal_ok "$d" || refuse_l state_corrupt "terminal"
    journal "$d" "$(jq -nc --argjson s "$seq" --arg k "$kind" '{event:"late_ignored",seq:$s,kind:$k}')" || refuse_l journal_failed "late_ignored"
    flock -u 9; echo "late_ignored seq=$seq"; return 0
  fi
  last=0
  for f in "$d/consumed"/*; do   # only names that are plain decimal sequence numbers count; crash leftovers are ignored
    n=${f##*/}; [[ $n =~ ^[1-9][0-9]{0,15}$ ]] || continue
    [ "$n" -gt "$last" ] && last=$n
  done
  if [ "$seq" -le "$last" ]; then refuse_l illegal_transition "seq=$seq not above last consumed $last"; fi
  if [ "$last" = 0 ] && [ "$kind" != accepted ]; then refuse_l illegal_transition "$kind before accepted"; fi
  if [ "$last" = 0 ] && [ "$seq" != 1 ]; then refuse_l illegal_transition "first event has seq $seq, not 1"; fi
  if [ "$last" != 0 ] && [ "$kind" = accepted ]; then refuse_l illegal_transition "second accepted"; fi
  if [ "$kind" = heartbeat ]; then
    isint "$po" && isint "$st" || refuse_l event_malformed "progress fields"
    lastprog=0; laststage=0
    if [ -e "$d/progress.json" ] || [ -L "$d/progress.json" ]; then   # present in any form: a directory or dangling link there is state damage, not "no progress yet"
      pj=$(read_regular "$d/progress.json") || refuse_l state_corrupt "progress.json"
      lastprog=$(jq -r '.progress_offset' <<<"$pj" 2>/dev/null); laststage=$(jq -r '.stage' <<<"$pj" 2>/dev/null)
      isint "$lastprog" && isint "$laststage" || refuse_l state_corrupt "progress.json"
    fi
    if [ "$po" -lt "$lastprog" ] || [ "$st" -lt "$laststage" ]; then refuse_l illegal_transition "progress went backwards"; fi
    atomic_write "$d" "$d/progress.json" "$(jq -nc --argjson p "$po" --argjson s "$st" '{progress_offset:$p,stage:$s}')" || refuse_l state_write_failed "progress.json"
  fi
  journal "$d" "$ev" || refuse_l journal_failed "event"
  if [ "$kind" != completed ]; then
    atomic_write "$d" "$d/consumed/$seq" "$dig $(now)" || refuse_l state_write_failed "consumed/$seq"
    flock -u 9; echo "consumed seq=$seq kind=$kind"; return 0
  fi
  # ---- terminal claim, still under the lock: the rename of a prepared directory is the single serialisation point ----
  claim_terminal "$d" completed "$seq" "$ec" "$dig"; crc=$?
  case $crc in
    0) atomic_write "$d" "$d/consumed/$seq" "$dig $(now)" || refuse_l state_write_failed "consumed/$seq"
       flock -u 9
       run_callback "$d"
       echo "consumed seq=$seq kind=completed exit_class=$ec callback_state=$(cbstate "$d")";;
    1) journal "$d" "$(jq -nc --argjson s "$seq" '{event:"superseded",seq:$s,kind:"completed"}')" || refuse_l journal_failed "superseded"
       atomic_write "$d" "$d/consumed/$seq" "$dig $(now)" || refuse_l state_write_failed "consumed/$seq"
       flock -u 9
       echo "superseded seq=$seq (terminal already claimed)"; cb_redo "$d";;
    3) refuse_l journal_failed "terminal_claimed (the terminal exists; redelivery is a DUP, resume-callback owns the callback)";;
    *) refuse_l terminal_claim_failed "build=$b seq=$seq (nothing consumed, redeliver)";;
  esac
  return 0
}

cmd_cancel() {
  local d="$1/$2" crc
  [[ $2 =~ ^[A-Za-z0-9_-]{1,128}$ ]] || refuse event_malformed "build_id"
  [ -f "$d/submit.json" ] || refuse event_unknown_build "$2"
  mkdir -p "$d/tmp" 2>/dev/null || refuse lock_unavailable "$2"
  # same per-build lock as consume: the claim and every journal line are serialised with the consumers
  { exec 9>>"$d/.lock"; } 2>/dev/null && lock_ok 9 "$d/.lock" || refuse lock_unavailable "$2"
  flock -w 60 9 || refuse lock_unavailable "$2"
  if [ -d "$d/terminal" ]; then if terminal_ok "$d"; then crc=1; else crc=2; fi; else claim_terminal "$d" cancelled 0 cancelled; crc=$?; fi
  case $crc in
    0) flock -u 9; run_callback "$d"; echo "cancelled";;
    1) journal "$d" "$(jq -nc '{event:"superseded",kind:"cancel"}')" || refuse_l journal_failed "superseded"
       flock -u 9; echo "superseded (terminal already claimed)";;
    3) refuse_l journal_failed "terminal_claimed (the terminal exists)";;
    *) refuse_l terminal_claim_failed "build=$2 (nothing claimed, retry)";;
  esac
}

cmd_resume_callback() { # builds_root build_id : re-run a callback found claimed, running or failed; non-zero when it is not done
  local d="$1/$2" rc=0; [[ $2 =~ ^[A-Za-z0-9_-]{1,128}$ ]] || refuse event_malformed "build_id"
  [ -f "$d/terminal/state.json" ] || refuse no_terminal "$2"
  run_callback "$d" || rc=21
  echo "callback_state=$(cbstate "$d")"; return $rc
}

main() {
  case "${1:-}" in
    resume-callback) [ $# -eq 3 ] || exit 2; cmd_resume_callback "$2" "$3";;
    secret-init) [ $# -eq 3 ] || exit 2; cmd_secret_init "$2" "$3";;
    derive-key)  [ $# -eq 4 ] || exit 2; python3 "$CRYPTO" derive "$2" "$3" "$4" "$CHECKOUT";;
    consume)     [ $# -eq 4 ] || exit 2; cmd_consume "$2" "$3" "$4";;
    cancel)      [ $# -eq 3 ] || exit 2; cmd_cancel "$2" "$3";;
    *) echo "usage: $0 secret-init|derive-key|consume|cancel|resume-callback ..." >&2; exit 2;;
  esac
}
if [ "${BASH_SOURCE[0]}" = "$0" ]; then main "$@"; fi
