#!/usr/bin/env bash
# tests/infra/lib.sh - shared helpers of the WP-13 test-infrastructure tests (T128-T134). Sourced, never run.
# Oracle strategy (11.4.245): SPECIFIED (the protocol answers a client defines: SELECT 1 -> 1, PING -> PONG, a stored file reads back with
# the sha256 recorded in the seeded corpus manifest) and INVARIANT (two seeder runs are byte-identical). No test asserts "no error".
# Every test runs REAL services (rootless podman, digest-pinned images); there are no mocks (11.4.27).
# shellcheck disable=SC2034
set -u
TI_TEST_HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TI_REPO="$(cd "$TI_TEST_HERE/../.." && pwd)"
# the scripts under test honour their test hooks only in a test (WF17 TI-D6); a test that proves the refusal runs the script with `env -u TI_TEST_MODE`
export TI_TEST_MODE=1
FAILS=0; PASSES=0; BLOCKED=0
ok()      { PASSES=$((PASSES+1)); [ "${QUIET:-0}" = 1 ] || echo "PASS: $1"; }
bad()     { FAILS=$((FAILS+1)); echo "FAIL: $1"; [ "${TI_FAILFAST:-0}" != 1 ] || exit 1; }   # TI_FAILFAST=1 (mutant runs only): the first violated check ends the run, the EXIT trap tears down
blocked() { BLOCKED=$((BLOCKED+1)); echo "BLOCKED: $1"; }
check()   { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
# identity header: every captured run starts with it (head, sha256 of each file under test, run_at)
ti_identity() { # ti_identity <title> <file>...
  echo "# identity: $1"; echo "# head: ${TI_IDENTITY_HEAD:-$(git -C "$TI_REPO" rev-parse HEAD 2>/dev/null)}"; echo "# run_at: $(date -u +%Y-%m-%dT%H:%M:%SZ)"; shift
  local f; for f in "$@"; do if [ -f "$TI_REPO/$f" ]; then echo "# sha256 $f $(sha256sum "$TI_REPO/$f" | cut -d' ' -f1)"; else echo "# sha256 $f ABSENT"; fi; done
}
# ti_regdir: the long-op registry directory AS scripts/longops/lib.sh derives it (LONGOPS_DIR, LONGOPS_AUDIT, LONGOPS_REPO), never a hard-coded `.audit/longops` (WF17 TI-B4, TI-G12)
ti_regdir() { ( LONGOPS_REPO="${LONGOPS_REPO:-$TI_REPO}"; . "$TI_REPO/scripts/longops/lib.sh" && printf '%s\n' "$LD" ); }
# a build id that is unique per run and per call: lowercase letters, digits, dash
ti_new_id() { local i; i="$(printf 't%s%04d' "$(date +%s | tail -c 7)" "$((RANDOM % 10000))")"; [ -z "${TI_IDS_LOG:-}" ] || echo "$i" >>"$TI_IDS_LOG"; echo "$i"; }
TI_SCRATCH="$(mktemp -d "$TI_REPO/.audit/scratch/ti-test.XXXXXX" 2>/dev/null || { mkdir -p "$TI_REPO/.audit/scratch"; mktemp -d "$TI_REPO/.audit/scratch/ti-test.XXXXXX"; })"
TI_IDS=()      # build ids this test started; the EXIT trap downs each of them by project label (never by name)
TI_BG_PIDS=()  # background starts of the test: the EXIT trap WAITS for them (bounded) before it downs the ids, so a start that finishes after the trap began is still torn down (WF17 TI-C7)
TI_FOREIGN=()  # container ids of foreign fixtures created by the test (removed by exact id)
TI_FOREIGN_PODS=()  # pod names of foreign fixtures created by the test (removed by exact name)
TI_FOREIGN_NETS=()  # network names of foreign fixtures created by the test (removed by exact name)
TI_FOREIGN_DIRS=()  # output directories of foreign fixtures created by the test (removed by exact path)
# ti_down <id> [extra down.sh args]: tears the project down AS ITS OWNER: the test started it, so it names the operation of that start (the env file of the run records it).
# A project whose env file is already gone has no live holder to name (down.sh then needs no op id).
ti_down() {
  local id=$1 op rc i hp; shift; op="$(ti_val "$id" TI_OP_ID 2>/dev/null)"
  bash "${TI_DOWN:-$TI_REPO/scripts/test-infra/down.sh}" --build-id "$id" ${op:+--op-id "$op"} "$@"; rc=$?
  if [ "$rc" = 5 ]; then   # a start of a MUTANT copy that failed after it took the lease: the live holder is a keeper this test started and the env file names an older operation; the keeper leaves when its file goes (no signal), then the holder is provably dead and the real down.sh reaps it
    rm -f "$(ti_state "$id")"/lease.keep.* 2>/dev/null
    for i in $(seq 1 20); do   # wait (bounded) until the recorded holder pid is no longer a live process: /proc only, no signal of any kind
      hp="$(jq -r '.pid // 0' "$(ti_regdir)/claims/$(ti_project "$id")/holder.json" 2>/dev/null)"
      { [[ "$hp" =~ ^[0-9]+$ ]] && [ "$hp" -gt 1 ] && [ -d "/proc/$hp" ]; } || break; sleep 1
    done
    bash "$TI_REPO/scripts/test-infra/down.sh" --build-id "$id" "$@"; rc=$?
  fi
  return "$rc"
}
# ti_test_root: the value of the catalogizer.test_root label every resource of THIS checkout carries (first 16 hex of the sha256 of the real path of the checkout, no newline)
ti_test_root() { printf '%s' "$(realpath -e -- "$TI_REPO")" | sha256sum | cut -c1-16; }
# TI_LABEL_PROJECTS: compose project names a test EXPECTS to stay empty (a hostile-environment fixture such as the exported "victim" project) but which a broken script could populate: removed by exact label / name at exit
TI_LABEL_PROJECTS=()
# TI_ROOT_HASHES: catalogizer.test_root values of scratch checkouts the test created and ran a stack from (container-build.sh in a copy): everything carrying such a root label is removed at exit, whatever a (mutated) script left behind
TI_ROOT_HASHES=()
ti_cleanup() {
  local i c p w
  for p in "${TI_BG_PIDS[@]:-}"; do [ -n "$p" ] || continue; w=0; while kill -0 "$p" 2>/dev/null && [ "$w" -lt 120 ]; do sleep 1; w=$((w+1)); done; done   # no signal is ever sent: a start is waited for, not killed
  for c in "${TI_FOREIGN[@]:-}"; do [ -n "$c" ] && podman rm -f "$c" >/dev/null 2>&1; done
  for c in "${TI_FOREIGN_PODS[@]:-}"; do [ -n "$c" ] && podman pod rm -f "$c" >/dev/null 2>&1; done
  for c in "${TI_FOREIGN_NETS[@]:-}"; do [ -n "$c" ] && podman network rm "$c" >/dev/null 2>&1; done
  for p in "${TI_ROOT_HASHES[@]:-}"; do [ -n "$p" ] || continue; for c in $(podman ps -a -q --filter "label=catalogizer.test_root=$p" 2>/dev/null); do podman rm -f "$c" >/dev/null 2>&1; done
    for c in $(podman pod ls -q --filter "label=catalogizer.test_root=$p" 2>/dev/null); do podman pod rm -f "$c" >/dev/null 2>&1; done
    for c in $(podman network ls -q --filter "label=catalogizer.test_root=$p" 2>/dev/null); do podman network rm "$c" >/dev/null 2>&1; done
    for c in $(podman volume ls -q --filter "label=catalogizer.test_root=$p" 2>/dev/null); do podman volume rm -f "$c" >/dev/null 2>&1; done; done
  for p in "${TI_LABEL_PROJECTS[@]:-}"; do [ -n "$p" ] || continue; for c in $(podman ps -a -q --filter "label=catalogizer.test_project=$p" 2>/dev/null); do podman rm -f "$c" >/dev/null 2>&1; done; podman pod rm -f "pod_$p" >/dev/null 2>&1; podman network rm "${p}_test-network" >/dev/null 2>&1; done
  # the foreign fixtures are removed BEFORE the ids are downed: down.sh REFUSES a project that holds a resource that is not its own (foreign_owner), which would leave the whole stack behind
  for i in "${TI_IDS[@]:-}"; do [ -n "$i" ] && { ti_down "$i" >/dev/null 2>&1; rm -rf -- "$TI_REPO/.audit/out/$(ti_project "$i")-logs"; }; done   # the -logs directory of a failed start (up.sh --keep-logs) belongs to the test that provoked it
  for c in "${TI_FOREIGN_DIRS[@]:-}"; do [ -n "$c" ] && rm -rf -- "$c"; done
  rm -rf -- "${TI_SCRATCH:?}"
}
# ti_scrap <id>: removes what a MUTANT start of project <id> left behind when the real down.sh (correctly) REFUSES it: a mutant that skips the registration starts containers labelled with an operation that is in no registry,
# which is `foreign_owner` by design. Exact project label / name only (the id is this test's own), then the registry rows of the project through the registry's own reap; used on test-owned ids only, after ti_down failed.
ti_scrap() {
  local id=$1 P c; P="$(ti_project "$id")"
  for c in $(podman ps -a -q --filter "label=catalogizer.test_project=$P" 2>/dev/null); do podman rm -f "$c" >/dev/null 2>&1; done
  podman pod rm -f "pod_$P" >/dev/null 2>&1; podman network rm "${P}_test-network" >/dev/null 2>&1
  for c in $(podman volume ls -q --filter "label=catalogizer.test_project=$P" 2>/dev/null); do podman volume rm -f "$c" >/dev/null 2>&1; done
  [ ! -e "$(ti_state "$id")" ] || podman unshare rm -rf -- "$(ti_state "$id")" >/dev/null 2>&1 || rm -rf -- "$(ti_state "$id")"
  bash "$TI_REPO/scripts/longops/reap.sh" --purpose "$P" >/dev/null 2>&1; rm -rf -- "$TI_REPO/.audit/out/$P-client" "$TI_REPO/.audit/out/$P-seed"
  local f o   # a mutant start leaves non-terminal operation rows of the project; each owner is dead by now (the keeper left with its file), so the registry's own reap records them `reaped`
  for f in "$(ti_regdir)"/ops/"$P"-up-*.json; do [ -e "$f" ] || continue
    o="$(jq -r 'select(.state|IN("complete","failed","reaped","handoff","blocked-escape")|not)|.op_id' "$f" 2>/dev/null)"; [ -z "$o" ] || bash "$TI_REPO/scripts/longops/reap.sh" --op-id "$o" >/dev/null 2>&1; done
}
# ti_lease_state <build id> <expected op id>: `ok` only when the claim of the project names that operation AND its holder is a LIVE keeper process of that very start
# (pid > 1, /proc entry present and not a zombie, command line = the lease keeper watching `lease.keep.<op id>`); else free | wrong_run:<run> | bad_pid | dead | wrong_identity.
# The check of the lease is never "the claim directory exists" (WF12 F5): a directory without a live holder is the stale state, not a lease.
ti_lease_state() {
  local f; f="$(ti_regdir)/claims/$(ti_project "$1")/holder.json"; local run pid cmd st
  [ -f "$f" ] || { echo free; return; }
  run="$(jq -r '.run_id // ""' "$f" 2>/dev/null)"; pid="$(jq -r '.pid // ""' "$f" 2>/dev/null)"
  [ "$run" = "$2" ] || { echo "wrong_run:$run"; return; }
  { [[ "$pid" =~ ^[0-9]+$ ]] && [ "$pid" -gt 1 ]; } || { echo bad_pid; return; }
  [ -r "/proc/$pid/cmdline" ] || { echo dead; return; }
  st="$(sed 's/^.*) //' "/proc/$pid/stat" 2>/dev/null | cut -c1)"; { [ -n "$st" ] && [ "$st" != Z ]; } || { echo dead; return; }
  cmd="$(tr '\0' ' ' <"/proc/$pid/cmdline" 2>/dev/null)"
  case "$cmd" in *ti-lease-keeper*"lease.keep.$2"*) echo ok;; *) echo wrong_identity;; esac
}
trap ti_cleanup EXIT
ti_summary() { echo "SUMMARY pass=$PASSES fail=$FAILS blocked=$BLOCKED"; [ "$FAILS" -eq 0 ]; }
# require a script to exist (RED while it does not): the failing check names the missing file
need_script() { if [ -f "$TI_REPO/$1" ]; then ok "script present: $1"; else bad "script absent: $1"; return 1; fi; }
# state of a build id
ti_project()  { echo "catalogizer-test-$1"; }
ti_envfile()  { echo "$(ti_state "$1")/env"; }
ti_val()      { sed -n "s/^$2=//p" "$(ti_envfile "$1")" | head -1; }   # ti_val <id> <VAR>
# ti_tic <tic args...>: scripts/test-in-container.sh with a bounded retry on the two TRANSIENT refusals measured on this shared host
# (another stream's container briefly trips the anti-mess sweep, `anti_mess_drift`; the memory envelope moves between its computation and
# its check, `limit_exceeds_envelope`). Any other refusal or exit is returned at once; the retry count is printed to stderr (never silent).
ti_tic() {
  local n=0 rc errf="$TI_SCRATCH/tic.err.$$"
  while :; do
    bash "$TI_REPO/scripts/test-in-container.sh" "$@" 2>"$errf"; rc=$?
    if [ "$rc" -ne 0 ] && grep -q 'reason=\(anti_mess_drift\|limit_exceeds_envelope\)' "$errf" && [ "$n" -lt "${TI_TIC_RETRIES:-400}" ]; then n=$((n+1)); sleep 5; continue; fi
    cat "$errf" >&2; [ "$n" -eq 0 ] || echo "ti_tic: retried $n time(s) on a transient refusal" >&2; rm -f "$errf"; return "$rc"
  done
}
ti_network()  { echo "$(ti_project "$1")_test-network"; }
ti_state()    { echo "${TI_STATE_DIR:-$TI_REPO/.audit/test-infra}/$(ti_project "$1")"; }
# ---- fault injection at ONE edge (11.4.85): a PATH wrapper that DELEGATES every call to the real tool and injects exactly one fault, switched by the existence of a control file ----
# ti_wrap_podman <dir>: <dir>/podman runs the real podman, except `ps` exits 125 "database is locked" while <dir>/fault-ps exists (a transient this host produces)
ti_wrap_podman() {
  local d=$1 real; real="$(command -v podman)"; mkdir -p "$d"
  cat >"$d/podman" <<WRAP
#!/bin/bash
if [ "\${1:-}" = ps ] && [ -e "$d/fault-ps" ]; then echo "Error: database is locked" >&2; exit 125; fi
exec "$real" "\$@"
WRAP
  chmod +x "$d/podman"
}
# ti_wrap_compose <dir>: <dir>/podman-compose runs the real podman-compose for every subcommand except `up`: `up` runs the REAL `up -d postgres redis` of the same project and file (real containers), holds for
# <dir>/hold seconds (default 3), then exits with the code in <dir>/up-exit (default 0). Everything before the word `up` (-p, --in-pod, --env-file, -f, --profile) is passed through unchanged.
ti_wrap_compose() {
  local d=$1 real; real="$(command -v podman-compose)"; mkdir -p "$d"
  cat >"$d/podman-compose" <<WRAP
#!/bin/bash
g=(); for a in "\$@"; do [ "\$a" = up ] && break; g+=("\$a"); done
if [ "\${#g[@]}" -eq "\$#" ]; then exec "$real" "\$@"; fi
"$real" "\${g[@]}" up -d postgres redis || exit 99
touch "$d/started"
h=3; [ -r "$d/hold" ] && h=\$(cat "$d/hold"); sleep "\$h"
e=0; [ -r "$d/up-exit" ] && e=\$(cat "$d/up-exit"); exit "\$e"
WRAP
  chmod +x "$d/podman-compose"
}
# ti_wait_for <seconds> <command...>: polls (1 s) until the command succeeds; returns its last status (no chained sleeps in the callers)
ti_wait_for() { local n=$1 i; shift; for i in $(seq 1 "$n"); do "$@" && return 0; sleep 1; done; "$@"; }
# ti_proc_by_arg <needle>: prints the pids whose /proc cmdline holds the needle as an argument (never pgrep: a carrier would match)
ti_proc_by_arg() { local p c; for p in /proc/[0-9]*; do [ -r "$p/cmdline" ] || continue; c="$(tr '\0' ' ' <"$p/cmdline" 2>/dev/null)"; case "$c" in *"$1"*) [ "${p#/proc/}" = "$$" ] || echo "${p#/proc/}";; esac; done; }
