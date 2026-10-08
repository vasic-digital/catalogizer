#!/usr/bin/env bash
# lib.sh - shared helpers of the long-operation registry (T089; constitution 11.4.232; docs/16 section 13). Sourced, never run.
#
# State      LD (default <repo>/.audit/longops, ignored by T004, durable on disk, never tmpfs) holds
#              ops/<op_id>.json            one record per operation (docs/16 13.1), every write temp-then-rename
#              claims/<purpose>/holder.json the atomic `mkdir` claim of a purpose and its holder record
#              <purpose>.lock              the flock file of every compare-and-swap of that purpose
#              events.jsonl                append-only event log
#              signals.log                 every signal this registry ever sent (audit trail; a test asserts it stays empty)
# Identity   a process is judged by /proc/<pid>/stat start time (field 22), the boot id and /proc/<pid>/cmdline, never by `pgrep` (11.4.196 D).
# Inputs     (11.4.276 round 5, class A) EVERY value read from a record, an argument, `stat` or podman is validated on its RAW type BEFORE it is used: LO_OP_SHAPE / LO_HOLDER_SHAPE are the ONE
#            definition of a readable op record / holder record; a record that fails its shape is `unreadable` (a refusal), never a defaulted fact (`// 0`, `// []`, `|| echo 0` are forbidden here).
# Test hooks LONGOPS_REPO, LONGOPS_DIR, LONGOPS_AUDIT, LONGOPS_BUILDS, LONGOPS_EV, LONGOPS_AUD relocate state into a fixture;
#            LONGOPS_NOW (epoch seconds) fixes the wall clock, LONGOPS_MONO (seconds) the monotonic clock (when only LONGOPS_NOW is set the monotonic clock follows it, so a fixture needs one knob);
#            LONGOPS_ALLOW_TMPFS=1 lets a fixture live on tmpfs;
#            LONGOPS_TEST_SLEEP_IN_CS (seconds) widens the compare-and-swap critical section so a missing flock is observable;
#            LONGOPS_TEST_SLEEP_AFTER_READ (seconds) pauses a holder reader between its snapshot and its judgement so a missing re-read is observable;
#            LONGOPS_TEST_SLEEP_BEFORE_LOCK (seconds) pauses reap.sh before it takes the purpose lock so a decision made outside the lock is observable (WF11 F2);
#            LONGOPS_TEST_SLEEP_BEFORE_SIGNAL (seconds) pauses reap.sh between its classification and the start-time re-check that precedes the signal (R2-9 is observable);
#            LONGOPS_DEFAULT_NO_PROGRESS_S (seconds, default 3600) is the no-progress budget of an op that declares none (WF11 F7: never "never hung"); validated at load: a positive integer (WF14 R2-6).
#            LONGOPS_LOCK_WAIT_S (seconds, default 15, positive) is how long any script waits for a purpose lock before it exits 70 (a test shortens it to make a lock held across waiting observable, WF14 R2-1).
#            LONGOPS_REAP_GRACE_S (seconds, default 15, a non-negative integer) is the FLOOR of how long reap.sh waits, WITHOUT holding the purpose lock, for a signalled owner to exit; an op that declares
#            budget.stop_grace_s (register.sh --stop-grace-s) is waited for max(grace, stop_grace_s + 10): the runner wrapper stops its container for up to STOP_GRACE_S + 20 before it releases the op (LO-D4).
#            LONGOPS_PODMAN_TIMEOUT_S (seconds, default 30, positive) bounds every podman call of reap.sh.
export LC_ALL=C
_LO_HERE=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=${LONGOPS_REPO:-$(cd "$_LO_HERE/../.." && pwd)}
AUDIT_DIR=${LONGOPS_AUDIT:-$ROOT/.audit}
LD=${LONGOPS_DIR:-$AUDIT_DIR/longops}
BUILDS_DIR=${LONGOPS_BUILDS:-$AUDIT_DIR/builds}
FEAT=specs/001-full-project-audit-remediation
EV=${LONGOPS_EV:-$ROOT/$FEAT/evidence}
AUD=${LONGOPS_AUD:-$ROOT/$FEAT/audit}
PODMAN=${LONGOPS_PODMAN:-podman}
# exit codes (documented in docs/scripts/longops.md)
RC_USAGE=2; RC_CONFLICT=3; RC_CAS=4; RC_LIVE=5; RC_IDENT=6; RC_UNSAFE=7; RC_SURVIVED=8; RC_CSURV=9; RC_REFUSE=20
LO_DEFAULT_NP=${LONGOPS_DEFAULT_NO_PROGRESS_S:-3600}
LO_LOCK_WAIT=${LONGOPS_LOCK_WAIT_S:-15}
LO_PODMAN_TIMEOUT=${LONGOPS_PODMAN_TIMEOUT_S:-30}

lo_die() { echo "longops: $1: $2" >&2; exit "${3:-$RC_USAGE}"; }
# lo_uint <s>: a canonical non-negative integer of at most 15 digits (fits int64 with room for the arithmetic below): no sign, no space and NO LEADING ZERO (bash `$(( ))` reads 08 as an
# invalid octal and 010 as 8: LO-A7). Every operand of `$(( ))` or `[ -gt ]` that comes from a record, an argument, `stat` or podman passes it first.
lo_uint() { [[ "$1" =~ ^(0|[1-9][0-9]{0,14})$ ]]; }
# lo_need "$@": call it as the first command of a value-taking `--flag)` branch; a flag given as the LAST token has no value and is a usage error (LO-K1: `shift 2` fails on one argument and the loop spun).
lo_need() { [ "$#" -ge 2 ] || lo_die usage_error "$1 requires a value"; }
# lo_pid_ok <pid>: a pid a record may name: an integer > 1 whose /proc entry exists now, that is not a zombie and whose process group is > 1 (pid 0, 1, junk, a pid that does not exist,
# a zombie and a kernel thread are refused: none of them can own an operation, WF14 R2-5; the same test lo_alive and lo_signal apply).
lo_pid_ok() { local g; lo_uint "${1:-}" && [ "$1" -gt 1 ] && [ -r "/proc/$1/stat" ] && [ "$(lo_pstate "$1")" != Z ] && g=$(lo_ppgrp "$1") && [[ "$g" =~ ^[0-9]+$ && "$g" -gt 1 ]]; }
lo_uint "$LO_LOCK_WAIT" && [ "$LO_LOCK_WAIT" -gt 0 ] || lo_die bad_lock_wait "LONGOPS_LOCK_WAIT_S must be a positive integer" "$RC_USAGE"
lo_uint "$LO_DEFAULT_NP" && [ "$LO_DEFAULT_NP" -gt 0 ] || lo_die bad_default_budget "LONGOPS_DEFAULT_NO_PROGRESS_S must be a positive integer of at most 15 digits (an unvalidated 0 or junk would bring back \"never hung\", WF14 R2-6)" "$RC_USAGE"
lo_uint "$LO_PODMAN_TIMEOUT" && [ "$LO_PODMAN_TIMEOUT" -gt 0 ] || lo_die bad_podman_timeout "LONGOPS_PODMAN_TIMEOUT_S must be a positive integer" "$RC_USAGE"
[ -z "${LONGOPS_NOW:-}" ] || lo_uint "$LONGOPS_NOW" || lo_die bad_clock "LONGOPS_NOW is not a non-negative integer" "$RC_USAGE"
[ -z "${LONGOPS_MONO:-}" ] || lo_uint "$LONGOPS_MONO" || lo_die bad_clock "LONGOPS_MONO is not a non-negative integer" "$RC_USAGE"
lo_now() { echo "${LONGOPS_NOW:-$(date +%s)}"; }
# lo_mono: the MONOTONIC clock in whole seconds (/proc/uptime). Liveness is judged on it, never on the wall clock: a forward NTP step or a suspend/resume does not make every op hung (LO-D5). A fixture that sets only
# LONGOPS_NOW gets a monotonic clock that follows it; LONGOPS_MONO overrides it.
lo_mono() { local u; if [ -n "${LONGOPS_MONO:-}" ]; then echo "$LONGOPS_MONO"; elif [ -n "${LONGOPS_NOW:-}" ]; then echo "$LONGOPS_NOW"; else IFS=. read -r u _ </proc/uptime; echo "$u"; fi; }
# lo_boot_id: the identity of THIS boot. A (pid, start ticks) pair is only meaningful inside one boot: after a reboot a record can match a NEW process (LO-D6).
lo_boot_id() { local b; IFS= read -r b </proc/sys/kernel/random/boot_id 2>/dev/null; echo "$b"; }
lo_test_pause() { [ -z "${LONGOPS_TEST_SLEEP_BEFORE_LOCK:-}" ] || sleep "$LONGOPS_TEST_SLEEP_BEFORE_LOCK"; }
lo_utc() { date -u -d "@$1" +%Y-%m-%dT%H:%M:%SZ; }

# lo_safe_name <s>: purpose keys and op ids become file names: no slash, no leading dot or dash, no control character.
lo_safe_name() { [[ "$1" =~ ^[A-Za-z0-9_@+:][A-Za-z0-9._@+:=-]{0,199}$ ]]; }

# ---------------------------------------------------------------------------------------------------------------------------------------------------------------
# THE record shapes (11.4.276 round 5 class A, A-S2/A-S3). One definition each; every reader of an op record or a holder record goes through it.
# LO_OP_SHAPE   a jq program: true only for a record whose fields have their RAW types. Every record: op_id and purpose_key safe names, run_id a non-empty string, state IN the closed set, container_label a string
#               without TAB/LF/CR, write_paths an array of strings, started_utc ISO-8601 Z, attached_to/superseded_by/adopted_by absent or non-empty strings. A NON-TERMINAL record (registered|running) also names its
#               owner: pid a number > 1, start_time a canonical integer string, boot_id, cmdline, the progress fields and a budget of numbers (elapsed_ms and budget.stop_grace_s absent or numbers).
# LO_HOLDER_SHAPE  kind process: run_id, pid > 1, start_time, boot_id. kind suspended-run: builds PRESENT and an array of strings, state IN {suspended, ready_to_resume}, callback_state absent or IN the closed set,
#               ready_at a number when ready_to_resume, resume_ttl absent or a number. Any other kind is unreadable.
LO_OP_SHAPE='
def u: type=="number" and .==floor and .>=0 and .<1e15;
def sname: type=="string" and test("^[A-Za-z0-9_@+:][A-Za-z0-9._@+:=-]{0,199}$");
def optstr(k): (has(k)|not) or (.[k]|type=="string" and length>0);
type=="object"
and (.op_id|sname) and (.purpose_key|sname)
and (.state|type=="string" and IN("registered","running","complete","failed","reaped","handoff","blocked-escape"))
and (.run_id|type=="string" and length>0)
and (.container_label|type=="string" and (test("[\t\n\r]")|not))
and (.write_paths|type=="array" and all(.[]; type=="string"))
and (.started_utc|type=="string" and test("^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$"))
and optstr("attached_to") and optstr("superseded_by") and optstr("adopted_by")
and (if (.state|IN("registered","running")) then (
      (.pid|type=="number" and .==floor and .>1 and .<1e15)
      and (.start_time|type=="string" and test("^(0|[1-9][0-9]*)$"))
      and (.boot_id|type=="string" and length>0)
      and (.cmdline|type=="string")
      and (.last_progress_epoch|u) and (.last_progress_mono|u) and (.progress_offset|u)
      and (.budget|type=="object") and ([.budget.no_progress_s,.budget.wall_clock_s]|all(u))
      and ((.budget|has("stop_grace_s")|not) or (.budget.stop_grace_s|u))
      and ((has("elapsed_ms")|not) or (.elapsed_ms|u))
    ) else true end)'
LO_HOLDER_SHAPE='
def u: type=="number" and .==floor and .>=0 and .<1e15;
type=="object" and (.kind|type=="string") and (
  if .kind=="process" then
    (.run_id|type=="string" and length>0) and (.pid|type=="number" and .==floor and .>1 and .<1e15)
    and (.start_time|type=="string" and test("^(0|[1-9][0-9]*)$")) and (.boot_id|type=="string" and length>0)
  elif .kind=="suspended-run" then
    (.run_id|type=="string" and length>0) and (.builds|type=="array" and all(.[]; type=="string"))
    and (.state|type=="string" and IN("suspended","ready_to_resume"))
    and ((has("callback_state")|not) or (.callback_state|type=="string" and IN("none","claimed","running","done")))
    and ((.state!="ready_to_resume") or (.ready_at|u)) and ((has("resume_ttl")|not) or (.resume_ttl|u))
  else false end)'

# lo_init: create the state directories; refuses a tmpfs location (11.4.232 A: durable, never tmpfs).
lo_init() {
  mkdir -p "$LD/ops" "$LD/claims" || lo_die state "cannot create $LD" "$RC_REFUSE"
  local t; t=$(stat -f -c %T "$LD" 2>/dev/null)
  case "$t" in tmpfs|ramfs) [ "${LONGOPS_ALLOW_TMPFS:-0}" = 1 ] || lo_die tmpfs_state "$LD is on $t; the registry must be durable (set LONGOPS_ALLOW_TMPFS=1 only for a test fixture)" "$RC_REFUSE" ;; esac
}

# lo_fsync_dir <dir>: flush a directory entry (a rename, a link, a mkdir, an rmdir) to disk: GNU `sync` with a directory argument fsyncs it. Durability stops at the FILE otherwise: a power loss can undo a terminal
# write or a claim (11.4.205(6), LO-G3). The cost is one fsync per transition, documented in docs/scripts/longops.md.
lo_fsync_dir() { sync -- "$1" 2>/dev/null; return 0; }
# lo_wjson <file> <json>: durable atomic write (temp in the same directory, fsync, rename, directory fsync). REFUSES empty or invalid JSON (WF11 F5: an empty
# string passed `jq -c .` with exit 0 and wrote a 0-byte record, so one bad argument erased an op record or a holder and still reported success).
lo_wjson() {
  local f=$1 tmp; [ -n "${2:-}" ] || return 1
  tmp=$(mktemp "$(dirname "$f")/.tmp.XXXXXX") || return 1
  { printf '%s\n' "$2" | jq -ce . >"$tmp" 2>/dev/null && [ -s "$tmp" ]; } || { rm -f "$tmp"; return 1; }
  sync "$tmp" 2>/dev/null; mv -f "$tmp" "$f" || { rm -f "$tmp"; return 1; }
  lo_fsync_dir "$(dirname "$f")"
}
# lo_wjson_new <file> <json>: the same write, but EXCLUSIVE: it fails when <file> exists (hard link of the temp file: atomic, never overwrites).
lo_wjson_new() {
  local f=$1 tmp; [ -n "${2:-}" ] || return 1
  tmp=$(mktemp "$(dirname "$f")/.tmp.XXXXXX") || return 1
  { printf '%s\n' "$2" | jq -ce . >"$tmp" 2>/dev/null && [ -s "$tmp" ]; } || { rm -f "$tmp"; return 1; }
  sync "$tmp" 2>/dev/null; ln -- "$tmp" "$f" 2>/dev/null; local r=$?; rm -f "$tmp"; [ "$r" -ne 0 ] || lo_fsync_dir "$(dirname "$f")"; return $r
}
lo_event() {  # lo_event <kind> <jq-object-args...>: one append (O_APPEND, below PIPE_BUF: atomic)
  local k=$1; shift
  jq -nc --arg k "$k" --argjson t "$(lo_now)" "$@" '{ev:$k,t:$t}+$ARGS.named' >>"$LD/events.jsonl" 2>/dev/null || true
}

# --- process identity (never pgrep) ---
# lo_stat_tail <pid>: fields 3.. of /proc/<pid>/stat on ONE line. The record is read WHOLE (a process-controlled comm may hold a newline or an invalid UTF-8 byte), everything through the LAST ") " is dropped
# and any remaining newline becomes a space (LO-H1: the line-wise `sed 's/^.*) //'` read the second line of a record with a newline in its comm and judged a LIVE process dead).
lo_stat_tail() { local s=""; { IFS= read -r -d '' s; } 2>/dev/null <"/proc/$1/stat"; [ -n "$s" ] || return 1; s=${s##*) }; printf '%s\n' "${s//$'\n'/ }"; }
lo_pstart() { local t f; t=$(lo_stat_tail "$1") || return 1; read -r -a f <<<"$t"; echo "${f[19]:-}"; }   # field 22: start time in ticks
lo_pstate() { local t f; t=$(lo_stat_tail "$1") || return 1; read -r -a f <<<"$t"; echo "${f[0]:-}"; }
lo_ppgrp()  { local t f; t=$(lo_stat_tail "$1") || return 1; read -r -a f <<<"$t"; echo "${f[2]:-}"; }    # field 5
lo_cmdline() { tr '\0' ' ' <"/proc/$1/cmdline" 2>/dev/null | sed 's/ $//'; }
# lo_alive <pid> <start>: the process this record names is still the one running: same start time, not a zombie, kill -0 pre-filter.
lo_alive() {
  local pid=$1 st=$2
  { lo_uint "$pid" && [ "$pid" -gt 1 ] && [ -n "$st" ]; } || return 1
  [ -r "/proc/$pid/stat" ] || return 1
  [ "$(lo_pstart "$pid")" = "$st" ] || return 1
  [ "$(lo_pstate "$pid")" != Z ] || return 1
  kill -0 "$pid" 2>/dev/null || [ ! -O "/proc/$pid" ]   # EPERM on a foreign uid's live process is alive, not dead
}

# lo_signal <sig> <pid>: the ONLY place a signal is sent. Refuses pid or pgid <= 1 (11.4.263), never signals a group, writes the audit trail.
lo_signal() {
  local sig=$1 pid=$2 pg
  [[ "$pid" =~ ^[0-9]+$ && "$pid" -gt 1 ]] || return "$RC_UNSAFE"
  pg=$(lo_ppgrp "$pid"); [[ "$pg" =~ ^[0-9]+$ && "$pg" -gt 1 ]] || return "$RC_UNSAFE"
  printf '%s %s pid=%s pgid=%s\n' "$(lo_utc "$(lo_now)")" "$sig" "$pid" "$pg" >>"$LD/signals.log"
  kill -s "$sig" -- "$pid" 2>/dev/null
}
# lo_kill_child <pid>: TERM to a sleeper THIS process started (a needle's `$!`). Same guards as lo_signal (pid > 1, pgid > 1, one pid, never a group), no audit
# entry (a needle's sleeper is not a registry signal). The only other kill site of the scope besides lo_signal and the `kill -0` of lo_alive (a test asserts the set).
lo_kill_child() {
  local pid=$1 pg
  [[ "$pid" =~ ^[0-9]+$ && "$pid" -gt 1 ]] || return "$RC_UNSAFE"
  pg=$(lo_ppgrp "$pid"); [[ "$pg" =~ ^[0-9]+$ && "$pg" -gt 1 ]] || return "$RC_UNSAFE"
  kill -s TERM -- "$pid" 2>/dev/null
}

# lo_with_lock <purpose> <fn> [args]: run <fn> in a subshell holding the purpose's flock (every lock transition is a CAS under it).
lo_with_lock() {
  local p=$1; shift
  lo_safe_name "$p" || lo_die usage_error "unsafe purpose $(printf '%q' "$p")"
  ( flock -w "$LO_LOCK_WAIT" 9 || exit 70; "$@" ) 9>"$LD/$p.lock"
}
lo_cs_pause() { [ -z "${LONGOPS_TEST_SLEEP_IN_CS:-}" ] || sleep "$LONGOPS_TEST_SLEEP_IN_CS"; }

lo_holder_file() { echo "$LD/claims/$1/holder.json"; }
lo_op_file() { echo "$LD/ops/$1.json"; }
# lo_load_op <op_id>: sets `f` (the record file) and `purpose`. 4 unknown_op when no record exists; 20 op_record_unreadable when the record fails LO_OP_SHAPE (empty, unparsable, mistyped, an unknown
# state ...): a corrupt record is never read as unknown, clean or dead (WF11 class 2). Call in the main shell: it exits through lo_die.
lo_load_op() {
  f=$(lo_op_file "$1"); [ -e "$f" ] || lo_die unknown_op "$1" "$RC_CAS"
  jq -e -s "length==1 and (.[0] | ($LO_OP_SHAPE))" "$f" >/dev/null 2>&1 || lo_die op_record_unreadable "$f fails the op record shape (docs/scripts/longops.md FIELDS): empty, unparsable, mistyped or an unknown state" "$RC_REFUSE"
  purpose=$(jq -r .purpose_key "$f")
}

# lo_ops_snapshot: ONE pass over ops/*.json: one python3 reads every file, ONE jq applies LO_OP_SHAPE to all of them (LO-J1: the registry is never rescanned record by record, about 1000 records a day).
# Output, one line per file in name order: `ok TAB <file> TAB <state> TAB <op_id> TAB <purpose_key> TAB <compact json>` for a record that passes the shape, `bad TAB <file>` for every other file (empty, unparsable, mistyped,
# unreadable, a directory). A caller classifies only the NON-terminal records (lo_alive needs /proc) and treats a `bad` line as unreadable.
LO_SNAP_JQ='(fromjson) as $e | ($e.f | gsub("[[:cntrl:]]"; "?")) as $f
  | if $e.e then "bad\t\($f)" else
      ($e.t | try fromjson catch null) as $r
      | if ($r != null and ($r | ('"$LO_OP_SHAPE"'))) then "ok\t\($f)\t\($r.state)\t\($r.op_id)\t\($r.purpose_key)\t\($r | tojson)" else "bad\t\($f)" end
    end'
lo_ops_snapshot() {
  [ -d "$LD/ops" ] || return 0
  local raw
  raw=$(python3 -I -c '
import os, sys, json
d = sys.argv[1]
for n in sorted(os.listdir(d)):
    if not n.endswith(".json") or n.startswith("."):
        continue
    p = os.path.join(d, n)
    try:
        with open(p, "rb") as fh:
            t = fh.read().decode("utf-8")
        print(json.dumps({"f": p, "t": t}))
    except Exception:
        print(json.dumps({"f": p, "e": 1}))
' "$LD/ops" 2>/dev/null) || { printf 'bad\t%s\n' "$LD/ops"; return 0; }       # the reader itself failed: the registry is UNREAD, never empty
  [ -n "$raw" ] || return 0
  jq -rR "$LO_SNAP_JQ" <<<"$raw" 2>/dev/null || printf 'bad\t%s\n' "$LD/ops"
}

# lo_resume_ttl <purpose> <holder-json>: seconds. commit_push reads the reviewed conf of the approved copy (lazy: only called when a
# ready_to_resume holder exists); every other purpose uses the holder's own resume_ttl or LONGOPS_RESUME_TTL.
lo_resume_ttl() {
  local p=$1 h=$2 v
  if [ "$p" = commit_push ]; then
    v=$(sed -n 's/^[[:space:]]*resume_ttl[[:space:]]*[=:][[:space:]]*\([0-9][0-9]*\).*/\1/p' "${CPA_APPROVED_DIR:?}/scripts/repo/commit_push.conf" 2>/dev/null | head -1)
    [ -n "$v" ] || lo_die conf_unreadable "resume_ttl missing in $CPA_APPROVED_DIR/scripts/repo/commit_push.conf" "$RC_REFUSE"
  else
    v=$(jq -r '.resume_ttl // empty' <<<"$h" 2>/dev/null); v=${v:-${LONGOPS_RESUME_TTL:-3600}}
  fi
  lo_uint "$v" || lo_die conf_unreadable "resume_ttl '$v' is not a non-negative integer (purpose $p)" "$RC_REFUSE"
  echo "$v"
}
# lo_require_approved <purpose>: purpose commit_push needs CPA_APPROVED_DIR, checked first (CENTRAL C2).
lo_require_approved() {
  [ "$1" != commit_push ] || [ -n "${CPA_APPROVED_DIR:-}" ] || lo_die helper_not_approved "CPA_APPROVED_DIR is unset; run through cpa-host --exec-approved (CENTRAL C2)" "$RC_REFUSE"
}

# _lo_read_holder <purpose>: print the holder record text. rc 0 read; 1 there is NO holder record (the file does not exist); 2 it EXISTS but cannot be read (empty, mode 000, a directory ...): absent and
# unreadable are different facts and the second is never a stale claim (LO-A6).
_lo_read_holder() {
  local hf s=""; hf=$(lo_holder_file "$1")
  [ -e "$hf" ] || [ -L "$hf" ] || return 1
  [ -f "$hf" ] || return 2
  { IFS= read -r -d '' s; } 2>/dev/null <"$hf"
  [ -n "$s" ] || return 2
  printf '%s' "$s"
}
# lo_holder_status <purpose>: print `live`, `expired`, `dead` or `unreadable` for the holder record of the purpose; non-zero when there is no record.
lo_holder_status() {  # re-reads the record after judging it and retries on change: a reader never decides on a snapshot the hub has already moved past
  local p=$1 n r="" s2 s3 rc=0 rr
  for n in 1 2 3 4 5 6; do
    s2=$(_lo_read_holder "$p"); rr=$?
    case $rr in 1) return 1 ;; 2) echo unreadable; return 0 ;; esac
    [ -z "${LONGOPS_TEST_SLEEP_AFTER_READ:-}" ] || sleep "$LONGOPS_TEST_SLEEP_AFTER_READ"   # test hook: lets a test move the state between the snapshot and the judgement
    r=$(_lo_holder_status_of "$p" "$s2"); rc=$?
    s3=$(_lo_read_holder "$p")
    [ "$s3" = "$s2" ] && { [ -n "$r" ] && echo "$r"; return $rc; }
  done
  [ -n "$r" ] && echo "$r"; return $rc
}
_lo_holder_status_of() {   # prints live|expired|dead|unreadable; return 1 = no record; RC_REFUSE = a lazy conf read failed. A record that fails LO_HOLDER_SHAPE is `unreadable`, NEVER dead (WF11 F3).
  local p=$1 h=$2 kind f
  [ -n "$h" ] || return 1
  jq -e "$LO_HOLDER_SHAPE" >/dev/null 2>&1 <<<"$h" || { echo unreadable; return 0; }
  kind=$(jq -r '.kind' <<<"$h")
  case "$kind" in
    process)
      local pid st bid
      f=$(jq -r '[(.pid|tostring), .start_time, .boot_id]|@tsv' <<<"$h"); IFS=$'\t' read -r pid st bid <<<"$f"
      lo_uint "$pid" || { echo unreadable; return 0; }
      [ "$bid" = "$(lo_boot_id)" ] || { echo dead; return 0; }          # a record of ANOTHER boot names a process that is gone, whatever its pid and start ticks say now (LO-D6)
      if lo_alive "$pid" "$st"; then echo live; else echo dead; fi ;;
    suspended-run)
      local b st2 cb rdy ttl now
      now=$(lo_now)
      while IFS= read -r b; do [ -n "$b" ] || continue; [ -d "$BUILDS_DIR/$b/terminal" ] || { echo live; return 0; }; done < <(jq -r '.builds[]' <<<"$h")
      cb=$(jq -r '.callback_state // "none"' <<<"$h"); case "$cb" in claimed|running) echo live; return 0 ;; esac
      st2=$(jq -r '.state' <<<"$h")
      if [ "$st2" = ready_to_resume ]; then
        rdy=$(jq -r '.ready_at' <<<"$h"); lo_uint "$rdy" || { echo unreadable; return 0; }
        ttl=$(lo_resume_ttl "$p" "$h") || return "$RC_REFUSE"
        if [ "$now" -lt $((rdy + ttl)) ]; then echo live; else echo expired; fi
      else echo dead; fi ;;
    *) echo unreadable ;;
  esac
}

LO_CLASSIFY_JQ='if ('"$LO_OP_SHAPE"') then (if (.state|IN("registered","running")) then [.state,(.pid|tostring),.start_time,.boot_id,(.last_progress_mono|tostring),(.budget.no_progress_s|tostring),(.budget.wall_clock_s|tostring),((.elapsed_ms // 0)|tostring),(.progress_offset|tostring)] else [.state] end)|@tsv else empty end'
# lo_classify_op <op-json>: terminal | advancing | hung | dead_owner | unreadable  (+ evidence on stdout line 2). The record is judged against LO_OP_SHAPE FIRST (ONE jq call that also extracts every field the
# judgement reads, with no defaulting): `unreadable` = a record that fails the shape (empty, unparsable, a mistyped or out-of-set field): it is NEVER read as dead_owner, advancing or clean (WF11 class 2, 11.4.276 A).
lo_classify_op() {
  local j=$1 fl st pid pst bid lp np wc el off now i
  fl=$(jq -r "$LO_CLASSIFY_JQ" 2>/dev/null <<<"$j")
  [ -n "$fl" ] || { echo unreadable; echo "op record fails the record shape (docs/scripts/longops.md FIELDS): empty, unparsable, mistyped or an unknown state"; return 0; }
  IFS=$'\t' read -r st pid pst bid lp np wc el off <<<"$fl"
  case "$st" in complete|failed|reaped|handoff|blocked-escape) echo terminal; return 0 ;; esac
  for i in "$pid" "$lp" "$np" "$wc" "$el" "$off"; do lo_uint "$i" || { echo unreadable; echo "a numeric field of the record is not a canonical non-negative integer of at most 15 digits"; return 0; }; done
  [ "$bid" = "$(lo_boot_id)" ] || { echo dead_owner; echo "boot_id of the record differs from this boot: pid=$pid start_time=$pst belongs to a previous boot (resolved from /proc/sys/kernel/random/boot_id)"; return 0; }
  if ! lo_alive "$pid" "$pst"; then echo dead_owner; echo "pid=$pid start_time=$pst not running (resolved from /proc)"; return 0; fi
  now=$(lo_mono)
  [ "$np" -gt 0 ] || np=$LO_DEFAULT_NP   # an op with no no-progress budget is never "never hung": the declared default applies (WF11 F7)
  # an advancing but over-long op is hung too (T089a): its own elapsed monotonic time (heartbeat.sh --elapsed-ms) passed the wall-clock cap recorded at registration
  if [ "$wc" -gt 0 ] && [ "$el" -gt $((wc * 1000)) ]; then echo hung; echo "wall_clock: elapsed ${el}ms > wall_clock_s=${wc}"; return 0; fi
  if [ "$np" -gt 0 ] && [ $((now - lp)) -gt "$np" ]; then echo hung; echo "offset flat for $((now - lp))s > no_progress_s=$np (progress_offset=$off)"; return 0; fi
  echo advancing; echo "last progress $((now - lp))s ago, budget ${np}s"
}

lo_live_ops() {  # one op id per line: every non-terminal op record (an unreadable record prints its file name: it is never skipped)
  local kind file state oid pk json c
  while IFS=$'\t' read -r kind file state oid pk json; do
    if [ "$kind" = bad ]; then echo "unreadable:$file"; continue; fi
    case "$state" in registered|running) ;; *) continue ;; esac
    c=$(lo_classify_op "$json" | head -1); case "$c" in terminal) ;; unreadable) echo "unreadable:$file" ;; *) echo "$oid" ;; esac
  done < <(lo_ops_snapshot)
}

# --- label -> lineage (LO-E1/LO-E3): which op owns a container label ---
# A dispatched build is re-adopted as <id>, <id>-a2, <id>-a3 ... ALL carrying catalogizer.op_id=dispatch-<id>. The ONE lineage resolver decides on the LATEST attempt by NUMERIC suffix (-a10 > -a2), never on glob order.
declare -gA LIN_BASES=() LIN_CLASS=(); LIN_BAD=0
# lo_lineage_build: (re)build the label -> lineage map from the CURRENT registry (one snapshot, one jq). LIN_BAD=1 when ANY record of ops/ fails the shape: no label can then be resolved with certainty.
lo_lineage_build() {
  LIN_BASES=(); LIN_CLASS=(); LIN_BAD=0
  local kind file state oid pk json recs="" t k v
  while IFS=$'\t' read -r kind file state oid pk json; do
    if [ "$kind" = bad ]; then LIN_BAD=1; continue; fi
    recs+="$json"$'\n'
  done < <(lo_ops_snapshot)
  [ -n "$recs" ] || return 0
  while IFS=$'\t' read -r t k v; do
    case "$t" in
      K) case " ${LIN_BASES[$k]:-} " in *" $v "*) ;; *) LIN_BASES[$k]="${LIN_BASES[$k]:-} $v" ;; esac ;;
      C) LIN_CLASS[$k]=$v ;;
    esac
  done < <(jq -rs '
    def base: .op_id | sub("-a[0-9]+$"; "");
    def attempt: (.op_id | ((capture("-a(?<n>[0-9]+)$") | .n | tonumber) // 1));
    def lab: .container_label | (if startswith("catalogizer.op_id=") then .[18:] elif startswith("op_id=") then .[6:] else . end);
    (group_by(base) | .[] | (max_by(attempt)) as $l
       | ("C\t" + ($l|base) + "\t" + (if ($l.state|IN("registered","running")) then "live" elif $l.state=="handoff" then "handoff" else "terminal" end))),
    (.[] | (base) as $b | ([.op_id, lab] | map(select(length>0)) | unique[] | "K\t\(.)\t\($b)"))' <<<"$recs")
}
# lo_lineage_class <label-value>: live | handoff | terminal | unreadable | orphan. `unreadable` when any record of ops/ fails the shape or when one label belongs to more than one lineage; `orphan` when no record
# names the label (absence from ONE registry is not proof of staleness, 11.4.232 E).
lo_lineage_class() {
  local v=$1 bases n
  [ "$LIN_BAD" = 0 ] || { echo unreadable; return; }
  bases=${LIN_BASES[$v]:-}
  [ -n "${bases// /}" ] || { echo orphan; return; }
  set -- $bases; n=$#
  [ "$n" -eq 1 ] || { echo unreadable; return; }
  echo "${LIN_CLASS[$1]:-unreadable}"
}

# lo_claim <purpose> <holder-json>: atomic claim. 0 claimed; 3 the purpose is held (live or expired holder: prints the holder);
# 4 a stale claim (dead holder, or a claim directory without a holder record): never taken over silently, `reap.sh --purpose` decides.
lo_claim() {
  local p=$1 h=$2 s
  if [ -d "$LD/claims/$p" ]; then
    s=$(lo_holder_status "$p"); local rc=$?; [ "$rc" -eq "$RC_REFUSE" ] && return "$RC_REFUSE"; [ "$rc" -eq 0 ] || s=nohold
    case "$s" in
      live|expired) echo "purpose_conflict: $p held ($s): $(cat "$(lo_holder_file "$p")")" >&2; return "$RC_CONFLICT" ;;
      unreadable) echo "claim_unreadable: $p has a holder record that cannot be read; never taken over, resolve it by hand" >&2; return "$RC_REFUSE" ;;
      *) echo "stale_claim: $p holder is $s (resolved from /proc); reap with scripts/longops/reap.sh --purpose $p" >&2; return "$RC_CAS" ;;
    esac
  fi
  mkdir "$LD/claims/$p" 2>/dev/null || { echo "purpose_conflict: $p claimed concurrently" >&2; return "$RC_CONFLICT"; }
  lo_fsync_dir "$LD/claims"
  lo_wjson "$(lo_holder_file "$p")" "$h" || { rmdir "$LD/claims/$p" 2>/dev/null; lo_fsync_dir "$LD/claims"; return 1; }
}
# lo_unclaim <purpose> <expect-run-id>: CAS release of the claim; 4 when the holder is not the expected run.
lo_unclaim() {
  local p=$1 want=$2 cur
  [ -d "$LD/claims/$p" ] || return 0
  cur=$(jq -r '.run_id // ""' "$(lo_holder_file "$p")" 2>/dev/null)
  [ "$cur" = "$want" ] || { echo "cas_mismatch: $p held by run '$cur', expected '$want'" >&2; return "$RC_CAS"; }
  rm -rf -- "$LD/claims/$p"; lo_fsync_dir "$LD/claims"
}
# lo_unclaim_own <purpose> <run-id>: release the claim ONLY when this run holds it. A claim that belongs to another run (a successor that re-adopted the purpose, another op of the purpose) is left
# untouched and that is NOT a failure: the caller's own record write already succeeded (WF14 R2-10: a post-write CAS failure reported a correct write as exit 4).
lo_unclaim_own() {
  local p=$1 run=$2 cur
  [ -d "$LD/claims/$p" ] || return 0
  cur=$(jq -r '.run_id // ""' "$(lo_holder_file "$p")" 2>/dev/null)
  if [ "$cur" = "$run" ]; then rm -rf -- "$LD/claims/$p"; lo_fsync_dir "$LD/claims"; else echo "claim_not_ours: $p is held by run '$cur', not '$run': left untouched" >&2; fi
  return 0
}
lo_holder_json() {  # lo_holder_json <purpose> <run_id> <pid>; empty output (rc 1) when the process vanished
  local pid=$3 st; st=$(lo_pstart "$pid") && [ -n "$st" ] || return 1
  jq -nc --arg p "$1" --arg r "$2" --argjson pid "$pid" --arg st "$st" --arg c "$(lo_cmdline "$pid")" --argjson t "$(lo_now)" --arg b "$(lo_boot_id)" \
    '{kind:"process",purpose:$p,run_id:$r,pid:$pid,start_time:$st,boot_id:$b,cmdline:$c,state:"running",since:$t}'
}
