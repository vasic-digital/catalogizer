#!/usr/bin/env bash
# lib.sh - shared helpers of the long-operation registry (T089; constitution 11.4.232; docs/16 section 13). Sourced, never run.
#
# State      LD (default <repo>/.audit/longops, ignored by T004, durable on disk, never tmpfs) holds
#              ops/<op_id>.json            one record per operation (docs/16 13.1), every write temp-then-rename
#              claims/<purpose>/holder.json the atomic `mkdir` claim of a purpose and its holder record
#              <purpose>.lock              the flock file of every compare-and-swap of that purpose
#              events.jsonl                append-only event log
#              signals.log                 every signal this registry ever sent (audit trail; a test asserts it stays empty)
# Identity   a process is judged by /proc/<pid>/stat start time (field 22) and /proc/<pid>/cmdline, never by `pgrep` (11.4.196 D).
# Test hooks LONGOPS_REPO, LONGOPS_DIR, LONGOPS_AUDIT, LONGOPS_BUILDS, LONGOPS_EV, LONGOPS_AUD relocate state into a fixture;
#            LONGOPS_NOW (epoch seconds) fixes the clock; LONGOPS_ALLOW_TMPFS=1 lets a fixture live on tmpfs;
#            LONGOPS_TEST_SLEEP_IN_CS (seconds) widens the compare-and-swap critical section so a missing flock is observable;
#            LONGOPS_TEST_SLEEP_AFTER_READ (seconds) pauses a holder reader between its snapshot and its judgement so a missing re-read is observable;
#            LONGOPS_TEST_SLEEP_BEFORE_LOCK (seconds) pauses reap.sh before it takes the purpose lock so a decision made outside the lock is observable (WF11 F2);
#            LONGOPS_DEFAULT_NO_PROGRESS_S (seconds, default 3600) is the no-progress budget of an op that declares none (WF11 F7: never "never hung").
LC_ALL=C
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
RC_USAGE=2; RC_CONFLICT=3; RC_CAS=4; RC_LIVE=5; RC_IDENT=6; RC_UNSAFE=7; RC_SURVIVED=8; RC_REFUSE=20
LO_DEFAULT_NP=${LONGOPS_DEFAULT_NO_PROGRESS_S:-3600}

lo_die() { echo "longops: $1: $2" >&2; exit "${3:-$RC_USAGE}"; }
# lo_uint <s>: a non-negative integer of at most 15 digits (fits int64 with room for the arithmetic below); every number read from an argument or a record goes through it.
lo_uint() { [[ "$1" =~ ^[0-9]{1,15}$ ]]; }
# lo_pid_ok <pid>: a pid a record may name: an integer > 1 whose /proc entry exists now (pid 0, 1, junk and a pid that does not exist are refused).
lo_pid_ok() { lo_uint "${1:-}" && [ "$1" -gt 1 ] && [ -r "/proc/$1/stat" ]; }
[ -z "${LONGOPS_NOW:-}" ] || lo_uint "$LONGOPS_NOW" || lo_die bad_clock "LONGOPS_NOW is not a non-negative integer" "$RC_USAGE"
lo_now() { echo "${LONGOPS_NOW:-$(date +%s)}"; }
lo_test_pause() { [ -z "${LONGOPS_TEST_SLEEP_BEFORE_LOCK:-}" ] || sleep "$LONGOPS_TEST_SLEEP_BEFORE_LOCK"; }
lo_utc() { date -u -d "@$1" +%Y-%m-%dT%H:%M:%SZ; }

# lo_safe_name <s>: purpose keys and op ids become file names: no slash, no leading dot or dash, no control character.
lo_safe_name() { [[ "$1" =~ ^[A-Za-z0-9_@+:][A-Za-z0-9._@+:=-]{0,199}$ ]]; }

# lo_init: create the state directories; refuses a tmpfs location (11.4.232 A: durable, never tmpfs).
lo_init() {
  mkdir -p "$LD/ops" "$LD/claims" || lo_die state "cannot create $LD" "$RC_REFUSE"
  local t; t=$(stat -f -c %T "$LD" 2>/dev/null)
  case "$t" in tmpfs|ramfs) [ "${LONGOPS_ALLOW_TMPFS:-0}" = 1 ] || lo_die tmpfs_state "$LD is on $t; the registry must be durable (set LONGOPS_ALLOW_TMPFS=1 only for a test fixture)" "$RC_REFUSE" ;; esac
}

# lo_wjson <file> <json>: durable atomic write (temp in the same directory, fsync, rename). REFUSES empty or invalid JSON (WF11 F5: an empty
# string passed `jq -c .` with exit 0 and wrote a 0-byte record, so one bad argument erased an op record or a holder and still reported success).
lo_wjson() {
  local f=$1 tmp; [ -n "${2:-}" ] || return 1
  tmp=$(mktemp "$(dirname "$f")/.tmp.XXXXXX") || return 1
  { printf '%s\n' "$2" | jq -ce . >"$tmp" 2>/dev/null && [ -s "$tmp" ]; } || { rm -f "$tmp"; return 1; }
  sync "$tmp" 2>/dev/null; mv -f "$tmp" "$f"
}
# lo_wjson_new <file> <json>: the same write, but EXCLUSIVE: it fails when <file> exists (hard link of the temp file: atomic, never overwrites).
lo_wjson_new() {
  local f=$1 tmp; [ -n "${2:-}" ] || return 1
  tmp=$(mktemp "$(dirname "$f")/.tmp.XXXXXX") || return 1
  { printf '%s\n' "$2" | jq -ce . >"$tmp" 2>/dev/null && [ -s "$tmp" ]; } || { rm -f "$tmp"; return 1; }
  sync "$tmp" 2>/dev/null; ln -- "$tmp" "$f" 2>/dev/null; local r=$?; rm -f "$tmp"; return $r
}
lo_event() {  # lo_event <kind> <jq-object-args...>: one append (O_APPEND, below PIPE_BUF: atomic)
  local k=$1; shift
  jq -nc --arg k "$k" --argjson t "$(lo_now)" "$@" '{ev:$k,t:$t}+$ARGS.named' >>"$LD/events.jsonl" 2>/dev/null || true
}

# --- process identity (never pgrep) ---
lo_stat_tail() { [ -r "/proc/$1/stat" ] && sed 's/^.*) //' "/proc/$1/stat" 2>/dev/null; }   # fields 3.. of /proc/<pid>/stat
lo_pstart() { lo_stat_tail "$1" | cut -d' ' -f20; }                                         # field 22: start time in ticks
lo_pstate() { lo_stat_tail "$1" | cut -d' ' -f1; }
lo_ppgrp()  { lo_stat_tail "$1" | cut -d' ' -f3; }                                           # field 5
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
  ( flock -w 15 9 || exit 70; "$@" ) 9>"$LD/$p.lock"
}
lo_cs_pause() { [ -z "${LONGOPS_TEST_SLEEP_IN_CS:-}" ] || sleep "$LONGOPS_TEST_SLEEP_IN_CS"; }

lo_holder_file() { echo "$LD/claims/$1/holder.json"; }
lo_op_file() { echo "$LD/ops/$1.json"; }
# lo_load_op <op_id>: sets `f` (the record file) and `purpose`. 4 unknown_op when no record exists; 20 op_record_unreadable when it is empty, unparsable or lacks a
# string state/purpose_key: a corrupt record is never read as unknown, clean or dead (WF11 class 2). Call in the main shell: it exits through lo_die.
lo_load_op() {
  f=$(lo_op_file "$1"); [ -e "$f" ] || lo_die unknown_op "$1" "$RC_CAS"
  jq -e 'type=="object" and (.purpose_key|type=="string") and (.state|type=="string")' "$f" >/dev/null 2>&1 || lo_die op_record_unreadable "$f is empty, unparsable or has no state/purpose_key" "$RC_REFUSE"
  purpose=$(jq -r .purpose_key "$f"); lo_safe_name "$purpose" || lo_die op_record_unreadable "$f names an unsafe purpose key" "$RC_REFUSE"
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

# lo_holder_live <purpose>: print `live`, `expired` or `dead` for the holder record of the purpose; non-zero when no record.
lo_holder_status() {  # re-reads the record after judging it and retries on change: a reader never decides on a snapshot the hub has already moved past
  local p=$1 hf n r s2
  hf=$(lo_holder_file "$p")
  for n in 1 2 3 4 5 6; do
    s2=$(cat "$hf" 2>/dev/null)
    [ -z "${LONGOPS_TEST_SLEEP_AFTER_READ:-}" ] || sleep "$LONGOPS_TEST_SLEEP_AFTER_READ"   # test hook: lets a test move the state between the snapshot and the judgement
    r=$(_lo_holder_status_of "$p" "$s2"); local rc=$?
    [ "$(cat "$hf" 2>/dev/null)" = "$s2" ] && { [ -n "$r" ] && echo "$r"; return $rc; }
  done
  [ -n "$r" ] && echo "$r"; return $rc
}
_lo_holder_status_of() {   # prints live|expired|dead|unreadable; return 1 = no record; RC_REFUSE = a lazy conf read failed. An unparsable or unknown-kind record is `unreadable`, NEVER dead (WF11 F3).
  local p=$1 h=$2 kind pid st
  [ -n "$h" ] || return 1
  jq -e 'type=="object" and (.kind|type=="string")' >/dev/null 2>&1 <<<"$h" || { echo unreadable; return 0; }
  kind=$(jq -r '.kind' <<<"$h")
  case "$kind" in
    process) pid=$(jq -r '.pid' <<<"$h"); st=$(jq -r '.start_time' <<<"$h")
             lo_uint "$pid" || { echo unreadable; return 0; }
             if lo_alive "$pid" "$st"; then echo live; else echo dead; fi ;;
    suspended-run)
      local b st2 cb rdy ttl now; now=$(lo_now)
      while IFS= read -r b; do [ -n "$b" ] || continue; [ -d "$BUILDS_DIR/$b/terminal" ] || { echo live; return 0; }; done < <(jq -r '.builds[]?' <<<"$h")
      cb=$(jq -r '.callback_state // "none"' <<<"$h"); case "$cb" in claimed|running) echo live; return 0 ;; esac
      st2=$(jq -r '.state' <<<"$h")
      if [ "$st2" = ready_to_resume ]; then
        rdy=$(jq -r '.ready_at // 0' <<<"$h"); lo_uint "$rdy" || { echo unreadable; return 0; }
        ttl=$(lo_resume_ttl "$p" "$h") || return "$RC_REFUSE"
        if [ "$now" -lt $((rdy + ttl)) ]; then echo live; else echo expired; fi
      else echo dead; fi ;;
    *) echo unreadable ;;
  esac
}

# lo_classify_op <op-json>: terminal | advancing | hung | dead_owner | unreadable  (+ evidence on stdout line 2). `unreadable` = an empty, unparsable or
# non-numeric record: it is NEVER read as dead_owner, advancing or clean (WF11 class 2); callers refuse or report it.
lo_classify_op() {
  local j=$1 st pid pst now np lp v
  jq -e 'type=="object" and (.state|type=="string")' >/dev/null 2>&1 <<<"$j" || { echo unreadable; echo "op record is empty, unparsable or has no string state"; return 0; }
  st=$(jq -r '.state' <<<"$j")
  case "$st" in complete|failed|reaped|handoff|blocked-escape) echo terminal; return 0 ;; esac
  pid=$(jq -r '.pid // 0' <<<"$j"); pst=$(jq -r '.start_time // ""' <<<"$j")
  if ! lo_alive "$pid" "$pst"; then echo dead_owner; echo "pid=$pid start_time=$pst not running (resolved from /proc)"; return 0; fi
  now=$(lo_now); np=$(jq -r '.budget.no_progress_s // 0' <<<"$j"); lp=$(jq -r '.last_progress_epoch // 0' <<<"$j")
  # an advancing but over-long op is hung too (T089a): its own elapsed monotonic time (heartbeat.sh --elapsed-ms) passed the wall-clock cap recorded at registration
  local wc el; wc=$(jq -r '.budget.wall_clock_s // 0' <<<"$j"); el=$(jq -r '.elapsed_ms // 0' <<<"$j")
  for v in "$np" "$lp" "$wc" "$el"; do lo_uint "$v" || { echo unreadable; echo "a numeric field of the record is not a non-negative integer of at most 15 digits"; return 0; }; done
  [ "$np" -gt 0 ] || np=$LO_DEFAULT_NP   # an op with no no-progress budget is never "never hung": the declared default applies (WF11 F7)
  if [ "$wc" -gt 0 ] && [ "$el" -gt $((wc * 1000)) ]; then echo hung; echo "wall_clock: elapsed ${el}ms > wall_clock_s=${wc}"; return 0; fi
  if [ "$np" -gt 0 ] && [ $((now - lp)) -gt "$np" ]; then echo hung; echo "offset flat for $((now - lp))s > no_progress_s=$np (progress_offset=$(jq -r '.progress_offset' <<<"$j"))"; return 0; fi
  echo advancing; echo "last progress $((now - lp))s ago, budget ${np}s"
}

lo_live_ops() {  # one op id per line: every non-terminal op record (an unreadable record prints its file name: it is never skipped)
  local f j c; for f in "$LD"/ops/*.json; do [ -e "$f" ] || continue; j=$(cat "$f" 2>/dev/null) || { echo "unreadable:$f"; continue; }
    c=$(lo_classify_op "$j" | head -1); case "$c" in terminal) ;; unreadable) echo "unreadable:$f" ;; *) jq -r '.op_id' <<<"$j" ;; esac; done
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
  lo_wjson "$(lo_holder_file "$p")" "$h" || { rmdir "$LD/claims/$p" 2>/dev/null; return 1; }
}
# lo_unclaim <purpose> <expect-run-id>: CAS release of the claim; 4 when the holder is not the expected run.
lo_unclaim() {
  local p=$1 want=$2 cur
  [ -d "$LD/claims/$p" ] || return 0
  cur=$(jq -r '.run_id // ""' "$(lo_holder_file "$p")" 2>/dev/null)
  [ "$cur" = "$want" ] || { echo "cas_mismatch: $p held by run '$cur', expected '$want'" >&2; return "$RC_CAS"; }
  rm -rf -- "$LD/claims/$p"
}
lo_holder_json() {  # lo_holder_json <purpose> <run_id> <pid>
  local pid=$3 st; st=$(lo_pstart "$pid")
  jq -nc --arg p "$1" --arg r "$2" --argjson pid "$pid" --arg st "$st" --arg c "$(lo_cmdline "$pid")" --argjson t "$(lo_now)" \
    '{kind:"process",purpose:$p,run_id:$r,pid:$pid,start_time:$st,cmdline:$c,state:"running",since:$t}'
}
