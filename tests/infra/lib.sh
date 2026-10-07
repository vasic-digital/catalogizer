#!/usr/bin/env bash
# tests/infra/lib.sh - shared helpers of the WP-13 test-infrastructure tests (T128-T134). Sourced, never run.
# Oracle strategy (11.4.245): SPECIFIED (the protocol answers a client defines: SELECT 1 -> 1, PING -> PONG, a stored file reads back with
# the sha256 recorded in the seeded corpus manifest) and INVARIANT (two seeder runs are byte-identical). No test asserts "no error".
# Every test runs REAL services (rootless podman, digest-pinned images); there are no mocks (11.4.27).
# shellcheck disable=SC2034
set -u
TI_TEST_HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TI_REPO="$(cd "$TI_TEST_HERE/../.." && pwd)"
FAILS=0; PASSES=0; BLOCKED=0
ok()      { PASSES=$((PASSES+1)); [ "${QUIET:-0}" = 1 ] || echo "PASS: $1"; }
bad()     { FAILS=$((FAILS+1)); echo "FAIL: $1"; [ "${TI_FAILFAST:-0}" != 1 ] || exit 1; }   # TI_FAILFAST=1 (mutant runs only): the first violated check ends the run, the EXIT trap tears down
blocked() { BLOCKED=$((BLOCKED+1)); echo "BLOCKED: $1"; }
check()   { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 (got '$2' want '$3')"; fi; }
# identity header: every captured run starts with it (head, sha256 of each file under test, run_at)
ti_identity() { # ti_identity <title> <file>...
  echo "# identity: $1"; echo "# head: $(git -C "$TI_REPO" rev-parse HEAD 2>/dev/null)"; echo "# run_at: $(date -u +%Y-%m-%dT%H:%M:%SZ)"; shift
  local f; for f in "$@"; do if [ -f "$TI_REPO/$f" ]; then echo "# sha256 $f $(sha256sum "$TI_REPO/$f" | cut -d' ' -f1)"; else echo "# sha256 $f ABSENT"; fi; done
}
# a build id that is unique per run and per call: lowercase letters, digits, dash
ti_new_id() { local i; i="$(printf 't%s%04d' "$(date +%s | tail -c 7)" "$((RANDOM % 10000))")"; [ -z "${TI_IDS_LOG:-}" ] || echo "$i" >>"$TI_IDS_LOG"; echo "$i"; }
TI_SCRATCH="$(mktemp -d "$TI_REPO/.audit/scratch/ti-test.XXXXXX" 2>/dev/null || { mkdir -p "$TI_REPO/.audit/scratch"; mktemp -d "$TI_REPO/.audit/scratch/ti-test.XXXXXX"; })"
TI_IDS=()      # build ids this test started; the EXIT trap downs each of them by project label (never by name)
TI_FOREIGN=()  # container ids of foreign fixtures created by the test (removed by exact id)
TI_FOREIGN_PODS=()  # pod names of foreign fixtures created by the test (removed by exact name)
TI_FOREIGN_DIRS=()  # output directories of foreign fixtures created by the test (removed by exact path)
# ti_down <id> [extra down.sh args]: tears the project down AS ITS OWNER: the test started it, so it names the operation of that start (the env file of the run records it).
# A project whose env file is already gone has no live holder to name (down.sh then needs no op id).
ti_down() {
  local id=$1 op rc i hp; shift; op="$(ti_val "$id" TI_OP_ID 2>/dev/null)"
  bash "${TI_DOWN:-$TI_REPO/scripts/test-infra/down.sh}" --build-id "$id" ${op:+--op-id "$op"} "$@"; rc=$?
  if [ "$rc" = 5 ]; then   # a start of a MUTANT copy that failed after it took the lease: the live holder is a keeper this test started and the env file names an older operation; the keeper leaves when its file goes (no signal), then the holder is provably dead and the real down.sh reaps it
    rm -f "$(ti_state "$id")"/lease.keep.* 2>/dev/null
    for i in $(seq 1 20); do   # wait (bounded) until the recorded holder pid is no longer a live process: /proc only, no signal of any kind
      hp="$(jq -r '.pid // 0' "$TI_REPO/.audit/longops/claims/$(ti_project "$id")/holder.json" 2>/dev/null)"
      { [[ "$hp" =~ ^[0-9]+$ ]] && [ "$hp" -gt 1 ] && [ -d "/proc/$hp" ]; } || break; sleep 1
    done
    bash "$TI_REPO/scripts/test-infra/down.sh" --build-id "$id" "$@"; rc=$?
  fi
  return "$rc"
}
ti_cleanup() {
  local i c
  for i in "${TI_IDS[@]:-}"; do [ -n "$i" ] && { ti_down "$i" >/dev/null 2>&1; rm -rf -- "$TI_REPO/.audit/out/$(ti_project "$i")-logs"; }; done   # the -logs directory of a failed start (up.sh --keep-logs) belongs to the test that provoked it
  for c in "${TI_FOREIGN[@]:-}"; do [ -n "$c" ] && podman rm -f "$c" >/dev/null 2>&1; done
  for c in "${TI_FOREIGN_PODS[@]:-}"; do [ -n "$c" ] && podman pod rm -f "$c" >/dev/null 2>&1; done
  for c in "${TI_FOREIGN_DIRS[@]:-}"; do [ -n "$c" ] && rm -rf -- "$c"; done
  rm -rf -- "${TI_SCRATCH:?}"
}
# ti_lease_state <build id> <expected op id>: `ok` only when the claim of the project names that operation AND its holder is a LIVE keeper process of that very start
# (pid > 1, /proc entry present and not a zombie, command line = the lease keeper watching `lease.keep.<op id>`); else free | wrong_run:<run> | bad_pid | dead | wrong_identity.
# The check of the lease is never "the claim directory exists" (WF12 F5): a directory without a live holder is the stale state, not a lease.
ti_lease_state() {
  local f="$TI_REPO/.audit/longops/claims/$(ti_project "$1")/holder.json" run pid cmd st
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
ti_envfile()  { echo "$TI_REPO/.audit/test-infra/$(ti_project "$1")/env"; }
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
ti_state()    { echo "$TI_REPO/.audit/test-infra/$(ti_project "$1")"; }
