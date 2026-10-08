#!/usr/bin/env bash
# lib.sh - shared helpers of scripts/test-infra (T129-T134). Sourced, never run. Documented in docs/scripts/test_infra_lib.md.
# HERE is the directory of the CALLING script (siblings are found there: a mutation copy of one script keeps using the real siblings through TI_SCRIPT_DIR);
# ROOT is the repository root (TI_ROOT overrides it for a fixture).
#
# This file is the ONE place of five class mechanisms (WF17 fix round 5, constitution 11.4.276):
#   INPUT      LC_ALL=C, ti_optval (a valued option needs a value), ti_uint, ti_abs, ti_safe_token/ti_safe_word, the env scrub of every compose / run_pinned call, test hooks only with TI_TEST_MODE=1
#   OWNERSHIP  ti_scan / ti_rm_resources: a resource is this checkout's only when project=catalogizer AND catalogizer.test_project=<P> AND catalogizer.test_root=<hash of this root> AND
#              its catalogizer.op_id names an op of THIS registry; anything else is `foreign_owner` and is never removed. ti_ld: the registry path comes from scripts/longops/lib.sh, never re-derived.
#              ti_lock: one per-user lock per project (flock on fd 9; every child is started with `9>&-` so a daemon never inherits it); a podman query that FAILS is `unknown`, never "nothing there".
#   STACK OP   ti_op_budget / ti_labels_check / ti_keeper_start: the registered operation of a stack has an explicit no-progress budget and a keeper that heartbeats ONLY while a container
#              carrying its catalogizer.op_id label is running, so a live stack is never `hung` and a dead one turns `hung` honestly.
export LC_ALL=C
# test hooks are honoured only in a test (TI_TEST_MODE=1), as scripts/containers/run_pinned.sh does for its own: a stray variable of a developer's shell must not redirect the stack
for _ti_h in TI_COMPOSE_FILE TI_CORPUS_CACHE_DIR TI_LOCK TI_SCRIPT_DIR TI_PROBE_CLIENT_DIR TI_RT_CLIENT_DIR TI_TEST_SLEEP_BEFORE_RELEASE TI_TEST_SLEEP_CACHE_INSTALL TI_TEST_SLEEP_AFTER_LOCK; do
  if [ -n "${!_ti_h:-}" ] && [ "${TI_TEST_MODE:-}" != 1 ]; then echo "test-infra: REFUSED reason=test_hook_outside_test_mode $_ti_h is a test hook (set TI_TEST_MODE=1 in a test)" >&2; exit 2; fi
done
unset _ti_h
TI_HERE="${TI_SCRIPT_DIR:-$(cd "$(dirname "${BASH_SOURCE[1]:-${BASH_SOURCE[0]}}")" && pwd)}"
TI_ROOT="${TI_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
TI_STATE_DIR="${TI_STATE_DIR:-$TI_ROOT/.audit/test-infra}"
TI_COMPOSE_FILE="${TI_COMPOSE_FILE:-$TI_ROOT/docker-compose.test-infra.yml}"
TI_LOCK="${TI_LOCK:-$TI_ROOT/build/containers/images.lock.yaml}"
ti_die()  { echo "test-infra: $1" >&2; exit "${2:-1}"; }
ti_refuse() { echo "test-infra: REFUSED reason=$1 ${2:-}" >&2; exit "${3:-1}"; }
# every child of a script that holds the project lock is started with fd 9 closed: a daemonised container monitor must never inherit the lock (podman ps/rm/inspect/... are plain children)
podman() { command podman "$@" 9>&-; }
# ---- INPUT layer ----
# ti_optval <option> <argc> <value>: the parser of a valued option calls it BEFORE `shift 2` (`--x) ti_optval "$1" $# "${2:-}"; X=$2; shift 2;;`). With the option LAST, `shift 2` fails without shifting and the loop never ends (WF17 TI-D1).
ti_optval() { [ "$2" -ge 2 ] || ti_die "$1 needs a value" 2; [ -n "$3" ] || ti_die "$1 needs a non-empty value" 2; }
# ti_uint <value> <max digits> <what>: a positive decimal without a leading zero (08 is not octal-safe, 010 is 8); ti_uint0 also admits 0. Arithmetic on the result is always $((10#$v)).
ti_uint()  { [[ "$1" =~ ^[1-9][0-9]*$ ]] && [ "${#1}" -le "$2" ] || ti_die "$3 must be a positive integer of at most $2 digits (no leading zero), got '${1:0:24}'" 2; }
ti_uint0() { [[ "$1" =~ ^(0|[1-9][0-9]*)$ ]] && [ "${#1}" -le "$2" ] || ti_die "$3 must be a non-negative integer of at most $2 digits (no leading zero), got '${1:0:24}'" 2; }
# ti_abs <path>: the absolute, normalised form against the CALLER's cwd (a relative path option never resolves against another base; the symlink-free canonical form is realpath -m)
ti_abs() { realpath -m -- "$1" 2>/dev/null || ti_die "cannot resolve the path '${1:0:80}'" 2; }
# ti_safe_token <what> <value>: an op id / label token: the longops safe-name grammar. ti_safe_word <what> <value> <max>: printable ASCII only (no newline, no control, no byte >= 0x80), not starting with a dash.
ti_safe_token() { [[ "$2" =~ ^[A-Za-z0-9_@+:][A-Za-z0-9._@+:=-]{0,199}$ ]] || ti_die "$1 must match ^[A-Za-z0-9_@+:][A-Za-z0-9._@+:=-]{0,199}\$" 2; }
ti_safe_word() { [[ "$2" =~ ^[\ -~]+$ ]] && [ "${#2}" -le "$3" ] && [ "${2:0:1}" != "-" ] || ti_die "$1 must be 1..$3 printable ASCII characters and not start with '-' (no newline, control or non-ASCII byte)" 2; }
# a state path becomes TI_DATA_DIR in a dotenv file and a volume source: ':' ' #' '$' a quote or a newline would break one of them
ti_safe_path() { local nl=$'\n'; case "$1" in *:*|*" #"*|*'$'*|*\'*|*\"*|*"$nl"*|*\\*) ti_die "the path '${1:0:80}' holds one of : ' #' \$ a quote a newline or a backslash, which the env file and the volume syntax cannot carry" 2;; esac; }
# ti_hash_of <string>: the first 16 hex characters of its sha256
ti_hash_of() { printf '%s' "$1" | sha256sum | cut -c1-16; }
# the checkout identity every resource of a stack carries as the label catalogizer.test_root (realpath, so a symlinked checkout is the same root)
ti_root_hash() { ti_hash_of "$(realpath -e -- "$TI_ROOT" 2>/dev/null || echo "$TI_ROOT")"; }
# ti_envscrub: fills TI_ESC with `env -u NAME ...` for every inherited variable that podman-compose or run_pinned would read INSTEAD of the per-run env file (shell env beats --env-file in podman-compose 1.5.0)
ti_envscrub() { local n; TI_ESC=(env); while IFS= read -r n; do TI_ESC+=(-u "$n"); done < <(compgen -e | grep -E '^(TI_|COMPOSE_|RUNP_)'); }
# ti_compose <podman-compose args...>: podman-compose under the scrubbed environment (fd 9 closed)
ti_compose() { ti_envscrub; "${TI_ESC[@]}" podman-compose "$@" 9>&-; }
# ti_runpinned <args...>: scripts/containers/run_pinned.sh with every inherited RUNP_* removed (run_pinned scrubs its own; the direct callers must too); RUNP_PRINT_ARGV=1 when TI_RUNP_PRINT=1
ti_runpinned() { local n; TI_ESC=(env); while IFS= read -r n; do TI_ESC+=(-u "$n"); done < <(compgen -e | grep -E '^RUNP_'); "${TI_ESC[@]}" ${TI_RUNP_PRINT:+RUNP_PRINT_ARGV=1} bash "$TI_ROOT/scripts/containers/run_pinned.sh" "$@" 9>&-; }
# build ids: lowercase letters, digits and dash, 1..31 chars, so the compose project name stays a valid, short, label-safe name (ASCII only: LC_ALL=C makes the class bytewise)
ti_valid_id() { [[ "$1" =~ ^[a-z0-9][a-z0-9-]{0,30}$ ]]; }
ti_project() { echo "catalogizer-test-$1"; }
ti_state() { echo "$TI_STATE_DIR/$(ti_project "$1")"; }
ti_network() { echo "$(ti_project "$1")_test-network"; }
ti_need() { local c; for c in "$@"; do command -v "$c" >/dev/null 2>&1 || ti_die "dependency missing: $c" 2; done; }
# ti_env_get <envfile> <VAR>: value of VAR (no export, no shell evaluation of the file: credentials are plain tokens, the file is data)
ti_env_get() { sed -n "s/^$2=//p" "$1" | head -1; }
# the long-op registry (11.4.232) is the scripts/longops CLI of this repository; a fixture may relocate its state with the LONGOPS_* variables
ti_lo() { local s=$1; shift; bash "$TI_ROOT/scripts/longops/$s.sh" "$@" 9>&-; }
# ti_ld: the registry directory AS scripts/longops/lib.sh derives it (LONGOPS_DIR, else LONGOPS_AUDIT/longops, else LONGOPS_REPO/.audit/longops); never re-derived here (WF17 TI-B4)
ti_ld_init() { [ -n "${TI_LD_CACHE:-}" ] || TI_LD_CACHE="$( LONGOPS_REPO="${LONGOPS_REPO:-$TI_ROOT}"; . "$TI_ROOT/scripts/longops/lib.sh" && printf '%s\n' "$LD" )"; [ -n "$TI_LD_CACHE" ] || ti_die "cannot resolve the long-op registry directory" 1; }   # sets TI_LD_CACHE in the CALLING shell
ti_ld() { ti_ld_init; printf '%s\n' "$TI_LD_CACHE"; }
# ---- per-user project lock (WF17 TI-B2/B3) ----
# ti_lock <project>: flock on fd 9 (bounded wait = the registry's lock wait). The lock is per USER (XDG_RUNTIME_DIR), so it spans checkouts. A caller whose parent holds it (TI_LOCK_HELD=<project>) does not take it again.
# ti_exit_on_signals: an entry point that arms an EXIT trap calls this FIRST. Measured (WF17 round 5): a non-interactive bash killed by SIGHUP runs NO EXIT trap, so an up.sh hung up at any stage left its
# claim and its lease keeper behind while INT and TERM cleaned up; a trap on the signal that exits turns every one of the three into a normal exit, and the EXIT trap then runs (the conventional 128+n statuses).
ti_exit_on_signals() { trap 'exit 130' INT; trap 'exit 143' TERM; trap 'exit 129' HUP; }
ti_lock() {
  [ "${TI_LOCK_HELD:-}" != "$1" ] || return 0
  ti_need flock
  local d f; d="${XDG_RUNTIME_DIR:-/tmp}/catalogizer-test-infra-$(id -u)"; f="$d/$1.lock"
  mkdir -p -m 700 "$d" 2>/dev/null; { exec 9>>"$f"; } 2>/dev/null || ti_die "cannot open the project lock $f" 1
  [ -z "${TI_TEST_SLEEP_AFTER_LOCK:-}" ] || sleep "$TI_TEST_SLEEP_AFTER_LOCK"   # test hook (TI_TEST_MODE=1): widens the window in which the lock is held
  flock -w "${LONGOPS_LOCK_WAIT_S:-15}" 9 || { { exec 9>&-; } 2>/dev/null; ti_refuse project_lock_busy "another process holds the lock of $1 for more than ${LONGOPS_LOCK_WAIT_S:-15}s ($f)" 5; }
  export TI_LOCK_HELD="$1"
}
ti_unlock() { { exec 9>&-; } 2>/dev/null; unset TI_LOCK_HELD; }
# ---- OWNERSHIP ----
ti_pod() { echo "pod_$1"; }   # the name of the pod podman-compose 1.5.0 creates for a project unless `--in-pod false` (its default `in_pod` is True, podman_compose.py:2081); unlabelled, so a LABEL filter never finds it
# ti_scan <project>: classifies what exists under the project's namespace. Sets TI_OWN_C (container ids owned), TI_OWN_V (volumes), TI_OWN_N (1 when the network is owned),
# TI_FOREIGN (a description of the first resource that is not this checkout's, else empty) and TI_UNKNOWN (the query that failed, else empty). Returns 0 when every query ran, 3 when one failed.
# A query that fails is UNKNOWN, never "nothing there" (WF17 TI-B5). One selector for the guard and the remover (WF17 TI-B6): catalogizer.test_project=<P>.
ti_scan() {
  local P=$1 NET root ids c lbl pj tp tr op v pod cnt
  ti_ld_init; NET="${P}_test-network"; root="$(ti_root_hash)"; TI_OWN_C=(); TI_OWN_V=(); TI_OWN_N=0; TI_FOREIGN=""; TI_UNKNOWN=""
  ids="$(podman ps -a -q --filter "label=catalogizer.test_project=$P" 2>/dev/null)" || { TI_UNKNOWN="podman ps"; return 3; }
  for c in $ids; do
    lbl="$(podman inspect --format '{{index .Config.Labels "project"}}|{{index .Config.Labels "catalogizer.test_project"}}|{{index .Config.Labels "catalogizer.test_root"}}|{{index .Config.Labels "catalogizer.op_id"}}' "$c" 2>/dev/null)" || {
      podman container exists "$c" 2>/dev/null && { TI_UNKNOWN="podman inspect $c"; return 3; }; continue; }   # a container that vanished between ps and inspect is gone, not unknown
    IFS='|' read -r pj tp tr op <<<"$lbl"
    if [ "$pj" != catalogizer ] || [ "$tp" != "$P" ]; then [ -n "$TI_FOREIGN" ] || TI_FOREIGN="container $c lacks the label project=catalogizer (has '$pj') or catalogizer.test_project=$P (has '$tp')"
    elif [ "$tr" != "$root" ]; then [ -n "$TI_FOREIGN" ] || TI_FOREIGN="container $c carries catalogizer.test_root='$tr' (this checkout is $root): another checkout owns it"
    elif [ -n "${TI_SCAN_ONLY_OP:-}" ] && [ "$op" != "$TI_SCAN_ONLY_OP" ]; then [ -n "$TI_FOREIGN" ] || TI_FOREIGN="container $c belongs to operation '$op', not to '$TI_SCAN_ONLY_OP'"
    elif [ -z "$op" ] || [ ! -e "$TI_LD_CACHE/ops/$op.json" ]; then [ -n "$TI_FOREIGN" ] || TI_FOREIGN="container $c carries catalogizer.op_id='$op', which is no operation of this checkout's registry"
    else TI_OWN_C+=("$c"); fi
  done
  ids="$(podman volume ls -q --filter "label=catalogizer.test_project=$P" 2>/dev/null)" || { TI_UNKNOWN="podman volume ls"; return 3; }
  for v in $ids; do
    tr="$(podman volume inspect "$v" --format '{{index .Labels "catalogizer.test_root"}}' 2>/dev/null)" || { TI_UNKNOWN="podman volume inspect $v"; return 3; }
    if [ "$tr" = "$root" ]; then TI_OWN_V+=("$v"); else [ -n "$TI_FOREIGN" ] || TI_FOREIGN="volume $v carries catalogizer.test_root='$tr' (this checkout is $root)"; fi
  done
  if podman network exists "$NET" 2>/dev/null; then
    tp="$(podman network inspect "$NET" --format '{{index .Labels "catalogizer.test_project"}}|{{index .Labels "catalogizer.test_root"}}' 2>/dev/null)" || { TI_UNKNOWN="podman network inspect $NET"; return 3; }
    if [ "$tp" = "$P|$root" ]; then TI_OWN_N=1; else echo "test-infra: network $NET is not labelled for $P in this checkout ('$tp'), left alone" >&2; fi
  else
    podman network exists "$NET" >/dev/null 2>&1; [ "$?" -le 1 ] || { TI_UNKNOWN="podman network exists $NET"; return 3; }
  fi
  pod="$(ti_pod "$P")"
  if podman pod exists "$pod" 2>/dev/null; then
    cnt="$(podman ps -a -q --filter "pod=$pod" 2>/dev/null)" || { TI_UNKNOWN="podman ps --filter pod"; return 3; }
    TI_POD_CNT="$(printf '%s' "$cnt" | grep -c .)"
  else TI_POD_CNT=-1; fi
  return 0
}
# ti_rm_resources <project>: removes this checkout's containers, volumes, network and the EMPTY podman-compose pod of the project. Returns 0 removed (sets TI_NC / TI_NN / TI_NV / TI_NP), 1 a resource
# could not be removed, 3 a podman query failed (unknown: nothing was removed), 5 a resource of the project is not this checkout's (nothing was removed; TI_FOREIGN names it).
ti_rm_resources() {
  local P=$1 NET c v pod rc=0; NET="${P}_test-network"; TI_NC=0; TI_NN=0; TI_NV=0; TI_NP=0
  ti_scan "$P" || { echo "test-infra: unknown state of $P: $TI_UNKNOWN failed" >&2; return 3; }
  if [ -n "$TI_FOREIGN" ]; then echo "test-infra: REFUSED reason=foreign_owner $TI_FOREIGN; nothing of $P was removed" >&2; return 5; fi
  for c in "${TI_OWN_C[@]}"; do
    if podman rm -f -v "$c" >/dev/null 2>&1; then TI_NC=$((TI_NC+1)); else echo "test-infra: cannot remove container $c" >&2; rc=1; fi
  done
  for v in "${TI_OWN_V[@]}"; do if podman volume rm "$v" >/dev/null 2>&1; then TI_NV=$((TI_NV+1)); else rc=1; fi; done
  if [ "$TI_OWN_N" = 1 ]; then if podman network rm "$NET" >/dev/null 2>&1; then TI_NN=1; else echo "test-infra: cannot remove network $NET" >&2; rc=1; fi; fi
  pod="$(ti_pod "$P")"
  if [ "$TI_POD_CNT" = 0 ]; then if podman pod rm "$pod" >/dev/null 2>&1; then TI_NP=1; else echo "test-infra: cannot remove pod $pod" >&2; rc=1; fi
  elif [ "$TI_POD_CNT" -gt 0 ]; then echo "test-infra: pod $pod still holds $TI_POD_CNT container(s) that do not carry the label of $P, left alone" >&2; fi
  return "$rc"
}
# ti_rm_out_dirs <project>: the per-project output directories the lifecycle scripts create under <repo>/.audit/out: `<project>-client` (run_client.sh default) and `<project>-seed`
# (a failed seeding). Exactly those two names for exactly this project; `<project>-logs` is the deliberate --keep-logs result and stays. Returns 1 when one cannot be removed.
ti_rm_out_dirs() {
  local P=$1 d rc=0
  ti_valid_project "$P" || return 1
  for d in "$TI_ROOT/.audit/out/$P-client" "$TI_ROOT/.audit/out/$P-seed"; do
    [ -e "$d" ] || continue
    podman unshare rm -rf -- "${d:?}" 2>/dev/null || rm -rf -- "$d" 2>/dev/null || { echo "test-infra: cannot remove $d" >&2; rc=1; }
  done
  return "$rc"
}
# a project name of this stack: catalogizer-test-<valid build id>
ti_valid_project() { [[ "$1" =~ ^catalogizer-test-[a-z0-9][a-z0-9-]{0,30}$ ]]; }
# ti_view_dir <dest> <repo-relative dir>...: builds the /src tree of a client container (WF12 F3): a scratch directory holding ONLY the named repository directories (their files, not subtrees).
# A path is refused unless, after normalisation, it is a plain relative path with no empty, `.` or `..` COMPONENT, no symlink component and (WF17 TI-F1) it resolves under scripts/test-infra/
# or .audit/scratch/ of this checkout; files named `.env*` or `*.env` are never copied.
ti_path_in_view() {   # ti_path_in_view <repo-relative path>: plain relative, no empty/./.. component, under scripts/test-infra/ or .audit/scratch/, and no symlink component (its realpath is the normalised path)
  local d=$1 rp
  case "$d" in ''|/*) return 1;; esac
  case "/$d/" in */./*|*//*|*/../*) return 1;; esac
  case "$d" in scripts/test-infra/*|.audit/scratch/*) ;; *) return 1;; esac
  rp="$(realpath -e -- "$TI_ROOT/$d" 2>/dev/null)" || return 1
  [ "$rp" = "$(realpath -m -- "$TI_ROOT")/$d" ]
}
ti_view_dir() {
  local dest=$1 d rp; shift
  for d in "$@"; do
    ti_path_in_view "$d" || return 1
    rp="$(realpath -e -- "$TI_ROOT/$d")"
    mkdir -p "$dest/$d" && { [ -z "$(find "$rp" -maxdepth 1 -type f ! -name '.env*' ! -name '*.env' -print -quit)" ] || find "$rp" -maxdepth 1 -type f ! -name '.env*' ! -name '*.env' -exec cp -p -t "$dest/$d" {} +; } || return 1
  done
}
# ---- STACK OP contract (WF17 TI-A1..A5) ----
# ti_op_budget: the no-progress budget (seconds) of the registered operation of a stack, TI_OP_BUDGET_S, default 3600. The keeper heartbeats every budget/6 seconds (at least 1).
ti_op_budget() { local b="${TI_OP_BUDGET_S:-3600}"; ti_uint "$b" 6 TI_OP_BUDGET_S; printf '%s\n' "$((10#$b))"; }
# ti_keeper_start <keep file> <op id> : starts the lease keeper (own session, fd 9 closed), sets KPID. It runs until the keep file goes (no signal is ever sent) and heartbeats the op ONLY when a running
# container carrying catalogizer.op_id=<op id> exists and the query succeeded: a dead stack then turns `hung` honestly (WF17 TI-A1).
ti_keeper_start() {
  local budget every; budget="$(ti_op_budget)" || exit 2; every=$((budget/6)); [ "$every" -ge 1 ] || every=1
  setsid bash -c 'keep=$1; op=$2; every=$3; root=$4; n=0
    while [ -e "$keep" ]; do sleep 1; n=$((n+1))
      if [ "$n" -ge "$every" ]; then n=0
        ids=$(podman ps -q --filter "label=catalogizer.op_id=$op" --filter status=running 2>/dev/null) && [ -n "$ids" ] && bash "$root/scripts/longops/heartbeat.sh" --op-id "$op" >/dev/null 2>&1
      fi
    done' ti-lease-keeper "$1" "$2" "$every" "$TI_ROOT" </dev/null >/dev/null 2>&1 9>&- &
  KPID=$!
  local _; for _ in 1 2 3 4 5 6 7 8 9 10; do [ -r "/proc/$KPID/stat" ] && break; sleep 0.1; done
  return 0
}
# ti_holder_ok <project> <op id> <keeper pid>: the claim of the project names that operation, its holder pid is the keeper and /proc says the keeper is alive (not a zombie) and is the keeper of that very start
ti_holder_ok() {
  local f run pid st cmd; ti_ld_init; f="$TI_LD_CACHE/claims/$1/holder.json"
  [ -f "$f" ] || return 1
  run="$(jq -r '.run_id // ""' "$f" 2>/dev/null)"; pid="$(jq -r '.pid // ""' "$f" 2>/dev/null)"
  [ "$run" = "$2" ] && [ "$pid" = "$3" ] || return 1
  [ -r "/proc/$pid/cmdline" ] || return 1
  st="$(sed 's/^.*) //' "/proc/$pid/stat" 2>/dev/null | cut -c1)"; { [ -n "$st" ] && [ "$st" != Z ]; } || return 1
  cmd="$(tr '\0' ' ' <"/proc/$pid/cmdline" 2>/dev/null)"
  case "$cmd" in *ti-lease-keeper*"lease.keep.$2"*) return 0;; esac; return 1
}
# ti_live_keeper_in <state dir>: succeeds (prints the keep file) when a lease.keep.* file of that state directory names a LIVE keeper process, judged by /proc/<pid>/cmdline (never by a pgrep substring)
ti_live_keeper_in() {
  local kf p c
  for kf in "$1"/lease.keep.*; do [ -e "$kf" ] || continue
    for p in /proc/[0-9]*; do
      [ -r "$p/cmdline" ] || continue
      c="$(tr '\0' ' ' <"$p/cmdline" 2>/dev/null)"
      case "$c" in *ti-lease-keeper*" $kf "*) printf '%s\n' "$kf"; return 0;; esac
    done
  done
  return 1
}
