#!/usr/bin/env bash
# sync_common.sh - helpers shared by test_sync_driver.sh (T188) and test_sync_fix_r2.sh (WF23 fix round 2) for scripts/register/sync_trackers.sh.
# Source AFTER lib.sh and clib.sh, with SYNC set to the driver under test.
# 1. Host-pressure retry (WF23 E9): every container call of the suites goes through envretry. When the wrapper REFUSED to start the container because the host has too
#    little memory/CPU/disk (run_pinned REFUSED reason=memory_budget_unavailable ...: nothing ran), the call is repeated after a pause, a bounded number of times
#    (ENV_TRIES x 15 s). The budget is NEVER overridden; a refusal that persists past the bound is reported as the failure it is.
ENV_RX="disk_below_headroom|disk_free_unreadable|lock_unreadable|memory_budget_unavailable|image_not_present_locally|cpu_budget_unavailable"
eval "$(declare -f lk | sed '1s/^lk /clib_lk /')"
eval "$(declare -f tool | sed '1s/^tool /clib_tool /')"
envretry() {
  local n=0 out rc ef="$T_SCR/envretry.$$.err"
  while :; do
    out=$("$@" 2>"$ef"); rc=$?
    if [ $rc -ne 0 ] && grep -Eq "$ENV_RX" "$ef" && [ $n -lt "${ENV_TRIES:-120}" ]; then n=$((n+1)); ENV_RETRIES=$(( ${ENV_RETRIES:-0} + 1 )); sleep 15; continue; fi
    [ -z "$out" ] || printf '%s\n' "$out"; cat "$ef" >&2; return $rc
  done
}
lk() { envretry clib_lk "$@"; }
tool() { envretry clib_tool "$@"; }

# 2. register fixture helpers
DB=docs/workable_items.db
pq() { python3 -I - "$1/$DB" "$2" <<'PY'
import sqlite3,sys,urllib.parse
c=sqlite3.connect("file:"+urllib.parse.quote(sys.argv[1])+"?mode=ro",uri=True)
for r in c.execute(sys.argv[2]): print("|".join("" if v is None else str(v) for v in r))
PY
}
ADAPTER='#!/usr/bin/env bash
R=$(cd "$(dirname "$0")" && pwd)
in=$(cat)
id=$(printf "%s" "$in" | python3 -I -c "import json,sys;print(json.load(sys.stdin)[\"atm_id\"])")
evd=$(printf "%s" "$in" | python3 -I -c "import json,sys;print(json.load(sys.stdin)[\"evidence_dir\"])")
printf "%s argv=[%s] envnames=%s\n" "$id" "$*" "$(env | cut -d= -f1 | sort | tr "\n" ",")" >>"$R/adapter.log"
printf "%s" "$in" >"$R/adapter.stdin.last"
case "$(cat "$R/adapter.mode" 2>/dev/null || echo ok)" in
 ok) mkdir -p "$R/$evd"; printf "{\"receipt\":\"%s\",\"remote_ref\":\"stub#%s\"}\n" "$id" "$id" >"$R/$evd/$id.json"; printf "{\"status\":\"SYNCED\",\"remote_ref\":\"stub#%s\",\"evidence_path\":\"%s/%s.json\",\"exit_code\":0}\n" "$id" "$evd" "$id";;
 slow) sleep 6; mkdir -p "$R/$evd"; printf "{\"receipt\":\"%s\",\"remote_ref\":\"stub#%s\"}\n" "$id" "$id" >"$R/$evd/$id.json"; printf "{\"status\":\"SYNCED\",\"remote_ref\":\"stub#%s\",\"evidence_path\":\"%s/%s.json\",\"exit_code\":0}\n" "$id" "$evd" "$id";;
 fail) echo "{\"status\":\"FAILED\"}"; exit 3;;
 unreachable) echo "{\"status\":\"SKIPPED\",\"skip_reason\":\"unreachable\"}";;
 garbage) echo "this is not json";;
 noref) mkdir -p "$R/$evd"; echo x >"$R/$evd/$id.json"; printf "{\"status\":\"SYNCED\",\"evidence_path\":\"%s/%s.json\"}\n" "$evd" "$id";;
 exit1) mkdir -p "$R/$evd"; printf "{\"receipt\":\"%s\",\"remote_ref\":\"stub#%s\"}\n" "$id" "$id" >"$R/$evd/$id.json"; printf "{\"status\":\"SYNCED\",\"remote_ref\":\"stub#%s\",\"evidence_path\":\"%s/%s.json\"}\n" "$id" "$evd" "$id"; exit 1;;
 outside) printf "{\"status\":\"SYNCED\",\"remote_ref\":\"stub#%s\",\"evidence_path\":\"../outside-evidence.json\"}\n" "$id";;
 badskip) echo "{\"status\":\"SKIPPED\",\"skip_reason\":\"not_configured\"}";;
 badnames) echo "{\"status\":\"SKIPPED\",\"skip_reason\":\"credentials_absent\",\"missing_env\":[\"not a name\"]}";;
 skipcreds) echo "{\"status\":\"SKIPPED\",\"skip_reason\":\"credentials_absent\",\"missing_env\":[\"REMOTE_TOKEN\"]}";;
 hang) exec sleep 4;;
 noevidence) printf "{\"status\":\"SYNCED\",\"remote_ref\":\"stub#%s\",\"evidence_path\":\"%s/none.json\"}\n" "$id" "$evd";;
esac'
mkfix() {  # mkfix NAME N -> scratch root with a register database holding N items (written through the real wrapper)
  local r; r=$(mkroot "$1"); cdb "$r" workable_items.db || return 1
  local i; for i in $(seq 1 "$2"); do lk "$r" -- sh -c "$(mint_cmd workable_items.db)" >/dev/null 2>&1 || { echo "mint $i failed" >&2; return 1; }; done
  printf '%s\n' "$ADAPTER" >"$r/adapter.sh"; : >"$r/adapter.log"; mkdir -p "$r/.helix"; echo "$r"; }
# cfg ROOT <<EOF : write the config; the two first-sync keys are added (approver "owner", a pilot size large enough for the fixture)
# (A driver without the first-sync gate rejects those keys as unknown, so the keys are written only for a driver that has the gate: that lets the new suite run against the
# PRE-FIX driver and fail on the merits of each check, which is the RED evidence.)
cfg() { if grep -q first_sync_approvers "$SYNC"; then sed '/^id_prefix: /a first_sync_approvers: [owner]\nfirst_sync_pilot_max: 1000' >"$1/.helix/reporting.yaml"; else cat >"$1/.helix/reporting.yaml"; fi; }

# 3. driver calls. pflags prints the first-sync approval flags for the plan that a dry run of the same arguments prints (nothing when no tracker needs approval).
pflags() {  # pflags ROOT args...
  local r=$1; shift; local -a keep=(); local haslimit=0 out h
  while [ $# -gt 0 ]; do case "$1" in --json) shift; [ $# -gt 0 ] && shift;; --dry-run) shift;; --limit) haslimit=1; keep+=("$1"); shift; if [ $# -gt 0 ]; then keep+=("$1"); shift; fi;; *) keep+=("$1"); shift;; esac; done
  out=$(tool "$r" "$SYNC" "${keep[@]}" --dry-run 2>/dev/null)
  h=$(printf '%s\n' "$out" | sed -n 's/.*first_sync_gate trackers=[^ ]* plan_hash=\([0-9a-f]\{64\}\).*/\1/p' | head -1)
  [ -n "$h" ] || return 0
  printf '%s %s %s %s' --approved-by owner --approve-first-sync "$h"; [ "$haslimit" = 1 ] || printf ' --limit 1000'
}
drv() {  # drv ROOT args... : the driver against a scratch root, real wrapper; a non-dry run carries the first-sync approval of its own plan. The whole output is also appended to $LEAK.
  local r=$1; shift; local o rc f
  case " $* " in *" --dry-run "*) o=$(tool "$r" "$SYNC" "$@" 2>&1); rc=$?;; *) f=$(pflags "$r" "$@"); o=$(tool "$r" "$SYNC" "$@" $f 2>&1); rc=$?;; esac
  [ -z "${LEAK:-}" ] || printf '%s\n' "$o" >>"$LEAK"
  printf '%s\n' "$o"; return $rc
}
