#!/usr/bin/env bash
# dispatch.sh - T005b SLICE (owner decision C1): event-driven build dispatcher. Reuses event_core.sh (verify/replay/order/terminal claim/
# callback) and lib/bev_crypto.py; adds submit, status, wait, resume, cancel, snapshot, the per-build pump and the remote emitter wiring.
#   dispatch.sh snapshot <dir>                                   content digest of a source tree (sha256 over sorted path+sha256)
#   dispatch.sh submit --purpose KEY --callback ID --image IMG-ID --src DIR [--variant primary|repro-cold] [--need BYTES]
#                      [--wallclock-cap S] [--no-progress-budget S] [--heartbeat S] [--network-none] -- <argv...>
#       prints the build_id at once and never blocks. Exit 20 + `REFUSED reason=` for caller defects and refusals.
#   dispatch.sh status <build_id>        JSON: state, terminal kind/reason/exit class, callback state, consumed seqs
#   dispatch.sh wait   <build_id> [S]    block (inotify, no polling) until the callback is done/failed; exit 0 done, 1 failed, 3 timeout
#   dispatch.sh resume <build_id>        restart the pump of an open build, or drain the journal of a terminal one (late events)
#   dispatch.sh cancel <build_id>        remote cancel (best effort), then the exactly-once terminal claim `cancelled`
# Environment (all documented in docs/scripts/dispatch.md): DISPATCH_BUILDS_ROOT, DISPATCH_STATE_DIR, DISPATCH_HOSTS_FILE,
#   DISPATCH_CALLBACKS_TSV, DISPATCH_TRANSPORT (ssh | local), DISPATCH_ALLOW_LOCAL, DISPATCH_SSH, DISPATCH_JOBS, DISPATCH_EMIT_DIR,
#   DISPATCH_EVWAIT, DISPATCH_REMOTE_RUNP, DISPATCH_RECONNECTS, DISPATCH_RECONNECT_DELAY, DISPATCH_DISK_OUT.
set -u
here=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
ROOT=$(cd "$here/../.." && pwd)
EC="$here/event_core.sh"; CRYPTO="$here/lib/bev_crypto.py"; SNAP_PY="$here/lib/snapshot.py"; TC_PY="$here/lib/treecache.py"
EMIT_DIR="${DISPATCH_EMIT_DIR:-$here}"           # directory holding remote/emit.sh and lib/evwait.py
BUILDS="${DISPATCH_BUILDS_ROOT:-$ROOT/.audit/builds}"
CHECKOUT_ID=$(printf '%s' "$ROOT" | sha256sum | cut -d' ' -f1)
STATE="${DISPATCH_STATE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/catalogizer/$CHECKOUT_ID}"
HOSTS_FILE="${DISPATCH_HOSTS_FILE:-$ROOT/build/hosts.env}"
CALLBACKS="${DISPATCH_CALLBACKS_TSV:-$here/callbacks.tsv}"
TRANSPORT="${DISPATCH_TRANSPORT:-ssh}"
JOBS="${DISPATCH_JOBS:-1}"
REMOTE_RUNP="${DISPATCH_REMOTE_RUNP:-$ROOT/scripts/containers/run_pinned.sh}"
. "$here/lib/longops_env.sh"; bind_longops_env "$BUILDS" "$ROOT"
LO="${LONGOPS_SCRIPTS:-$ROOT/scripts/longops}"
# the event hub (one per checkout) is ensured by submit and resume. Default on for the real builds root (<checkout>/.audit/builds); a fixture that moves the
# builds root runs hub-less unless DISPATCH_HUB=1 (so a test never leaves a hub behind). DISPATCH_HUB_SCRIPT overrides the hub script (a mutation run).
case "$BUILDS" in "$ROOT"/*) HUB_DEFAULT=1;; *) HUB_DEFAULT=0;; esac
ensure_hub() { [ "${DISPATCH_HUB:-$HUB_DEFAULT}" = 1 ] || return 0; bash "${DISPATCH_HUB_SCRIPT:-$here/event_hub.sh}" start >/dev/null 2>&1 9>&- 6>&-; return 0; }
BLOCKED_REASONS="host_unreachable no_qualified_host build_liveness_lost build_progress_flat build_wallclock_exceeded driver_secret_lost artifact_unavailable signing_key_not_provisioned"

refuse() { printf 'REFUSED reason=%s %s\n' "$1" "${2:-}" >&2; exit 20; }
now() { date -u +%Y-%m-%dT%H:%M:%SZ; }
starttime() { awk '{print $22}' "/proc/$1/stat" 2>/dev/null; }
isint() { [[ ${1:-} =~ ^(0|[1-9][0-9]{0,15})$ ]]; }
need() { local t; for t in "$@"; do command -v "$t" >/dev/null 2>&1 || refuse dependency_missing "$t"; done; }
atomic() { local f=$1 t; t="$f.tmp-$$"; printf '%s\n' "$2" > "$t" && mv -f "$t" "$f"; }
valid_id() { [[ ${1:-} =~ ^[A-Za-z0-9_-]{1,128}$ ]]; }

# The snapshot of a git checkout (T005b): the sha256 of a manifest of (path, tree id) for the root and every initialised submodule at every depth, each tree
# written by git plumbing from that repository's own temporary index (lib/snapshot.py; no `git add`, no filter, no attribute). A plain directory (no .git, the
# proof sources of test_dispatch.sh) keeps the content digest below.
EXCLUDES="${DISPATCH_SNAPSHOT_EXCLUDES:-specs/001-full-project-audit-remediation/evidence specs/001-full-project-audit-remediation/audit}"
SNAP_MODE=tic; SNAP_DECL=(); SNAP_COMP=""; SNAP_CF=(); SNAP_CTX=""; SNAP_TMPFILES=""
is_git_src() { [ -e "$1/.git" ]; }
snap_args() { local x; for x in $EXCLUDES; do printf '%s\0' --exclude "$x"; done; printf '%s\0' --mode "$SNAP_MODE"
  for x in ${SNAP_DECL[@]+"${SNAP_DECL[@]}"}; do printf '%s\0' --declare "$x"; done; [ -z "$SNAP_COMP" ] || printf '%s\0' --component "$SNAP_COMP"
  for x in ${SNAP_CF[@]+"${SNAP_CF[@]}"}; do printf '%s\0' --containerfile "$x"; done; [ -z "$SNAP_CTX" ] || printf '%s\0' --context "$SNAP_CTX"; }
snap_py() { # subcommand root [extra...] : run snapshot.py with the snapshot options; stdout passes through, a REFUSED line is re-raised with its reason
  local sub=$1 r=$2 err rc args=(); shift 2
  mkdir -p "$BUILDS/.snap" 2>/dev/null; mapfile -d '' -t args < <(snap_args)
  err=$(mktemp "$BUILDS/.snap/err.XXXXXX") || refuse state_write_failed "$BUILDS/.snap"
  python3 -I "$SNAP_PY" "$sub" --root "$r" --tmp "$BUILDS/.snap" "${args[@]}" "$@" 2>"$err"; rc=$?
  if [ $rc -ne 0 ]; then
    local why; why=$(grep -o 'reason=[A-Za-z0-9_]*' "$err" | head -1 | cut -d= -f2); local det; det=$(sed -n 's/^REFUSED reason=[A-Za-z0-9_]* //p' "$err" | head -1)
    rm -f "$err"; refuse "${why:-snapshot_failed}" "$det"
  fi
  rm -f "$err"; }
snapshot_digest() { # dir : see above. The git-plumbing snapshot is used when the directory is a git checkout.
  if is_git_src "$1"; then snap_py digest "$1"
  else ( cd "$1" 2>/dev/null && LC_ALL=C find . -path ./.git -prune -o -type f -print0 | LC_ALL=C sort -z | xargs -0 -r sha256sum | sha256sum | cut -d' ' -f1 ); fi; }
argv_digest() { local a; for a in "$@"; do printf '%s\0' "$a"; done | sha256sum | cut -d' ' -f1; }

# Budgets of a build class (T089a): scripts/longops/purposes.tsv, rows `class TAB no_progress_s TAB wall_clock_s TAB basis`, class a glob over build:<component>:<lane>
# (first match wins). A value `UNKNOWN` (not yet measured, T115) falls back to the dispatcher's defaults (60 s no-progress, 3600 s wall-clock cap).
PURPOSES_TSV="${DISPATCH_PURPOSES_TSV:-$LO/purposes.tsv}"
purpose_budget() { # component lane : prints "no_progress_s wall_clock_s source"
  local key="build:$1:$2" cls np wc basis rest np_o=60 wc_o=3600 src=default
  if [ -r "$PURPOSES_TSV" ]; then
    while IFS=$'\t' read -r cls np wc basis; do
      case $cls in ''|'#'*) continue;; esac
      # shellcheck disable=SC2053
      if [[ $key == $cls ]]; then
        src="default:UNKNOWN"
        if isint "$np" && [ "$np" -gt 0 ]; then np_o=$np; src="purposes.tsv:$cls"; fi
        if isint "$wc" && [ "$wc" -gt 0 ]; then wc_o=$wc; src="purposes.tsv:$cls"; fi
        break
      fi
    done < "$PURPOSES_TSV"
  fi
  printf '%s %s %s\n' "$np_o" "$wc_o" "$src"; }

# ---------------------------------------------------------------- transport
SSHC=()
ssh_setup() {
  if [ -n "${DISPATCH_SSH:-}" ]; then read -r -a SSHC <<<"$DISPATCH_SSH"
  else
    SSHC=(ssh -o BatchMode=yes -o ConnectTimeout=10)
    local id; id=$(hosts_value BUILD_SSH_IDENTITY); [ -z "$id" ] || SSHC+=(-i "$id" -o IdentitiesOnly=yes)
  fi
}
hosts_value() { [ -r "$HOSTS_FILE" ] && sed -n "s/^$1=//p" "$HOSTS_FILE" | head -1 | tr -d '"'"'"; }
hosts_list() { [ -r "$HOSTS_FILE" ] && grep -E '^BUILD_HOST_[0-9]+=' "$HOSTS_FILE" | sort -t_ -k3 -n | sed 's/^[^=]*=//; s/"//g'; }
# Host-key pinning (T005b): a host is trusted ONLY by the fingerprint pinned for it in build/hosts.env (BUILD_HOST_<n>_FP, from docs/infrastructure/build-hosts.md, T100),
# never on first use. The host's keys are scanned (ssh-keyscan), the one whose fingerprint equals the pin is written to a known-hosts file of its own, and ssh then runs with
# StrictHostKeyChecking=yes against that file alone (GlobalKnownHostsFile=/dev/null): a host whose key does not match is never connected to. With the test ssh shim
# (DISPATCH_SSH) pinning is off unless DISPATCH_PIN=1 (the shim then stands in for a host whose scanned key is the shim's: DISPATCH_KEYSCAN).
KEYSCAN="${DISPATCH_KEYSCAN:-ssh-keyscan}"
PIN_ON=1; [ -z "${DISPATCH_SSH:-}" ] || [ "${DISPATCH_PIN:-}" = 1 ] || PIN_ON=0
host_fp() { # host : the pinned fingerprint of the BUILD_HOST_<n> equal to host
  local k v; [ -r "$HOSTS_FILE" ] || return 0
  while IFS='=' read -r k v; do v=${v//\"/}; v=${v//\'/}; [ "$v" = "$1" ] || continue; hosts_value "BUILD_HOST_${k#BUILD_HOST_}_FP"; return 0; done < <(grep -E '^BUILD_HOST_[0-9]+=' "$HOSTS_FILE")
}
pin_host() { # host : prints the known-hosts file holding the pinned key. 0 ok; 11 no pin; 12 host unreachable (no key scanned); 13 no scanned key matches the pin; 14 ssh-keygen absent
  local h=$1 fp name kh scan line got
  command -v ssh-keygen >/dev/null 2>&1 || return 14
  fp=$(host_fp "$h"); [ -n "$fp" ] || return 11
  name=${h#*@}; kh="$STATE/known_hosts/$(printf '%s' "$h" | sha256sum | cut -c1-24)"
  # an existing file is re-verified against the pin every time, never trusted
  if [ -s "$kh" ] && [ "$(ssh-keygen -lf "$kh" < /dev/null 2>/dev/null | awk '{print $2}' | head -1)" = "$fp" ]; then printf '%s\n' "$kh"; return 0; fi
  scan=$("$KEYSCAN" -T 10 "$name" 2>/dev/null < /dev/null) || return 12
  [ -n "$scan" ] || return 12
  while IFS= read -r line; do
    case $line in ''|'#'*) continue;; esac
    got=$(printf '%s\n' "$line" | ssh-keygen -lf - 2>/dev/null | awk '{print $2}')
    if [ "$got" = "$fp" ]; then
      mkdir -p "$STATE/known_hosts" && chmod 700 "$STATE/known_hosts" && printf '%s\n' "$line" > "$kh.tmp-$$" && mv -f "$kh.tmp-$$" "$kh" || return 12
      printf '%s\n' "$kh"; return 0
    fi
  done <<<"$scan"
  return 13; }
rsh() { local h=$1 kh rc; shift       # one remote command string: ssh joins its arguments with spaces
  if [ "$PIN_ON" = 1 ]; then
    kh=$(pin_host "$h" < /dev/null); rc=$?; [ $rc -eq 0 ] || return 255
    "${SSHC[@]}" -o StrictHostKeyChecking=yes -o UserKnownHostsFile="$kh" -o GlobalKnownHostsFile=/dev/null -o UpdateHostKeys=no "$h" "$@"
  else "${SSHC[@]}" "$h" "$@"; fi; }
quote() { printf '%q ' "$@"; }

emit_path() { # remote emitter script path of a build record
  jq -r '.emit_path' "$1/submit.json"; }
remote_emit() { # builddir sub args... : run the emitter of the build on its host (stdin and stdout pass through)
  local d=$1 sub=$2; shift 2
  local tr h ep; tr=$(jq -r .transport "$d/submit.json"); h=$(jq -r .host "$d/submit.json"); ep=$(emit_path "$d")
  if [ "$tr" = local ]; then bash "$ep" "$sub" "$@"; else ssh_setup; rsh "$h" "bash $(quote "$ep" "$sub" "$@")"; fi
}

qualify_hosts() { # outfile : prints the first qualified host; writes every attempt (json array) to outfile. 1 none qualified
  local h out of=$1 A='[]'; ssh_setup; printf '[]\n' > "$of"
  while IFS= read -r h; do
    [ -n "$h" ] || continue
    if [ "$PIN_ON" = 1 ]; then
      pin_host "$h" > /dev/null < /dev/null; local prc=$?
      if [ $prc -ne 0 ]; then
        case $prc in 11) out=host_key_not_pinned;; 12) out=host_unreachable;; 13) out=host_key_mismatch;; *) out=ssh_keygen_absent;; esac
        if [ "$out" = host_unreachable ]; then A=$(jq -c --arg h "$h" '. + [{host:$h,result:"host_unreachable"}]' <<<"$A")
        else A=$(jq -c --arg h "$h" --arg d "$out" '. + [{host:$h,result:"not_qualified",detail:$d}]' <<<"$A"); fi
        printf '%s\n' "$A" > "$of"; continue
      fi
    fi
    if ! rsh "$h" true >/dev/null 2>&1; then A=$(jq -c --arg h "$h" '. + [{host:$h,result:"host_unreachable"}]' <<<"$A"); printf '%s\n' "$A" > "$of"; continue; fi
    out=$(rsh "$h" "podman info --format '{{.Host.Security.Rootless}}'" 2>/dev/null | tr -d '[:space:]')
    if [ "$out" != true ]; then A=$(jq -c --arg h "$h" '. + [{host:$h,result:"not_qualified",detail:"runtime_not_rootless"}]' <<<"$A"); printf '%s\n' "$A" > "$of"; continue; fi
    A=$(jq -c --arg h "$h" '. + [{host:$h,result:"qualified"}]' <<<"$A"); printf '%s\n' "$A" > "$of"; printf '%s\n' "$h"; return 0
  done < <(hosts_list)
  return 1
}

ship_emitter() { # host : copy emit.sh and evwait.py to <host>:~/.cache/catalogizer/emit/<sha>/ ; prints the remote emit.sh path
  local h=$1 sha rd
  sha=$(cat "$EMIT_DIR/remote/emit.sh" "$EMIT_DIR/lib/evwait.py" | sha256sum | cut -d' ' -f1)
  rd=".cache/catalogizer/emit/$sha"
  tar -C "$EMIT_DIR" -hcf - remote/emit.sh lib/evwait.py | rsh "$h" "mkdir -p $rd && tar -C $rd -xf - && chmod +x $rd/remote/emit.sh" || return 1
  printf '%s %s\n' "$sha" "$rd/remote/emit.sh"
}
ship_tree() { # host dir snapshot : content-addressed source tree cache on the host (a tree already there is not sent again)
  local h=$1 src=$2 sn=$3 rd
  rd=".cache/catalogizer/trees/$sn"
  rsh "$h" "test -d $rd" 2>/dev/null || tar -C "$src" --exclude=.git -cf - . | rsh "$h" "mkdir -p $rd.tmp && tar -C $rd.tmp -xf - && mv -T $rd.tmp $rd" || return 1
  printf '%s\n' "$rd"
}

hx() { # host cmd... : run a command on the build host (ssh), or locally for the declared proof transport ("local-proof"); stdin and stdout pass through
  local h=$1; shift
  if [ "$h" = local-proof ]; then bash -c "$*"; else rsh "$h" "$*"; fi; }
ship_git() { # host src manifest repos snapshot component builddir : ship the input closure of a git snapshot into the object cache of the host and materialise it.
  # Prints the working directory on the host. Exit 20 = snapshot_mismatch (an object or a tree that does not hash to its id), 1 = transport failure.
  local h=$1 src=$2 m=$3 repos=$4 psnap=$5 comp=$6 d=$7 cache tcsha tc key dest miss ids nmiss bytes rc
  if [ "$h" = local-proof ]; then cache="$BUILDS/.hostcache"; else cache=".cache/catalogizer"; fi
  tcsha=$(sha256sum "$TC_PY" | cut -d' ' -f1); tc="$cache/tc/$tcsha/treecache.py"
  hx "$h" "mkdir -p $cache/tc/$tcsha $cache/manifests $cache/trees" < /dev/null || return 1
  hx "$h" "cat > $tc.tmp && mv -f $tc.tmp $tc" < "$TC_PY" || return 1
  key=$(printf '%s\n%s\n%s\n' "$psnap" "$repos" "$comp" | sha256sum | cut -d' ' -f1); dest="$cache/trees/$key"
  ids="$d/tmp/ids.$$"; miss="$d/tmp/miss.$$"
  python3 -I "$SNAP_PY" plan --manifest "$m" --repos "$repos" --root "$src" > "$ids" || { rm -f "$ids"; return 1; }
  hx "$h" "python3 -I $tc missing $cache" < "$ids" > "$miss" || { rm -f "$ids" "$miss"; return 1; }
  nmiss=$(grep -c . "$miss"); bytes=0
  if [ "$nmiss" -gt 0 ]; then
    python3 -I "$SNAP_PY" pack --manifest "$m" --repos "$repos" --root "$src" < "$miss" > "$d/tmp/pack.$$.tar" || { rm -f "$ids" "$miss" "$d/tmp/pack.$$.tar"; return 1; }
    bytes=$(stat -c %s "$d/tmp/pack.$$.tar")
    hx "$h" "python3 -I $tc store $cache" < "$d/tmp/pack.$$.tar" >/dev/null 2>&1; rc=$?
    rm -f "$d/tmp/pack.$$.tar"; [ $rc -eq 20 ] && { rm -f "$ids" "$miss"; return 20; }; [ $rc -eq 0 ] || { rm -f "$ids" "$miss"; return 1; }
  fi
  rm -f "$ids" "$miss"
  jq -nc --argjson o "$nmiss" --argjson b "$bytes" '{objects:$o,bytes:$b}' > "$d/transfer.json"
  hx "$h" "cat > $cache/manifests/$key.json" < "$m" || return 1
  hx "$h" "python3 -I $tc materialize $cache $cache/manifests/$key.json --repos $repos --dest $dest" >/dev/null 2>&1; rc=$?
  [ $rc -eq 20 ] && return 20; [ $rc -eq 0 ] || return 1
  printf '%s\n' "$dest"; }

# ---------------------------------------------------------------- build groups (T005a (v))
# A group (builds/group/<id>/, mode 0700) closes only through `group-seal`; it is terminal once it is sealed and every member is terminal, claimed by the
# rename of a prepared terminal directory exactly like a build. Member callbacks are record-only; the group has ONE callback (registered at the first submit).
# A member ending blocked-unavailable, cancelled or infra_failed ends the group early (with --cancel-on-fail also test_failed and build_failed): the open members
# are marked cancelled_by_group and cancelled, which never triggers anything and never sets the kind. The kind is decided by precedence (never arrival order).
valid_group() { [[ ${1:-} =~ ^[A-Za-z0-9_-]{1,100}$ ]]; }
gdir() { echo "$BUILDS/group/$1"; }
group_validate() { # gid cb cof : a submit joining an existing group must agree with it and the group must be open (nothing is created here)
  local g=$(gdir "$1")
  [ -f "$g/group.json" ] || return 0
  [ "$(jq -r .callback_id "$g/group.json")" = "$2" ] || refuse group_conflict "the group callback is registered once ($(jq -r .callback_id "$g/group.json"))"
  [ "$(jq -r .cancel_on_fail "$g/group.json")" = "$([ "$3" = 1 ] && echo true || echo false)" ] || refuse group_conflict "--cancel-on-fail differs from the group's"
  [ "$(jq -r .sealed "$g/group.json")" = false ] || refuse group_sealed "$1"
  [ "$(jq -r .ended_early "$g/group.json")" = false ] || refuse group_ended "$1"; }
group_add() { # gid cb cof budget build_id : create the group at its first member, append the member (under the group lock); 1 = conflict
  local g; g=$(gdir "$1")
  mkdir -p "$BUILDS/group" && chmod 0700 "$BUILDS/group" && mkdir -p "$g/tmp" || return 1
  (
    exec 5>>"$g/.lock"; flock 5
    if [ ! -f "$g/group.json" ]; then
      local srow k; srow=$(awk -F'\t' -v i="$2" '!/^#/ && $1 == i { print $2; exit }' "$CALLBACKS")
      if [ -n "$srow" ] && [ "$srow" != - ]; then
        jq -n --arg k "cbcore-$2-group-$1" --arg sk "cb-$2-group-$1" --arg cb "$2" '{callback_id:$cb,effect_key:$k,script_effect_key:$sk,args:{}}' > "$g/callback.tmp"
      else jq -n --arg k "cb-$2-group-$1" --arg cb "$2" '{callback_id:$cb,effect_key:$k,args:{}}' > "$g/callback.tmp"; fi && mv -f "$g/callback.tmp" "$g/callback.json"
      jq -n --arg g "$1" --arg cb "$2" --argjson c "$([ "$3" = 1 ] && echo true || echo false)" --argjson b "$4" --argjson t "$(date +%s)" \
        '{group_id:$g,callback_id:$cb,cancel_on_fail:$c,budget_s:$b,created_epoch:$t,sealed:false,ended_early:false,orphaned:false,members:[]}' > "$g/group.tmp" && mv -f "$g/group.tmp" "$g/group.json"
    fi
    [ "$(jq -r .callback_id "$g/group.json")" = "$2" ] && [ "$(jq -r .sealed "$g/group.json")" = false ] && [ "$(jq -r .ended_early "$g/group.json")" = false ] || exit 1
    jq --arg m "$5" 'if (.members | index($m)) then . else .members += [$m] end' "$g/group.json" > "$g/group.tmp" && mv -f "$g/group.tmp" "$g/group.json"
  ) 5>&-; }
member_kind() { # builddir -> effective kind: succeeded|test_failed|build_failed|infra_failed|blocked-unavailable|cancelled|cancelled_by_group|open
  local d=$1 k ec
  [ -d "$d/terminal" ] || { echo open; return; }
  k=$(jq -r .kind "$d/terminal/state.json" 2>/dev/null); ec=$(jq -r '.exit_class // ""' "$d/terminal/state.json" 2>/dev/null)
  if [ -e "$d/cancelled_by_group" ] && { [ "$k" = cancelled ] || [ "$ec" = cancelled ]; }; then echo cancelled_by_group
  elif [ "$k" = completed ]; then echo "${ec:-succeeded}"; else echo "$k"; fi; }
group_check() { # gid [orphan] : evaluate the group: early end, then the terminal claim once sealed and every member is terminal
  local gid=$1 g; g=$(gdir "$1"); [ -f "$g/group.json" ] || return 0
  local tocancel=() out
  out=$(
    exec 5>>"$g/.lock"; flock 5
    [ -d "$g/terminal" ] && { echo done; exit 0; }
    gj=$(cat "$g/group.json"); sealed=$(jq -r .sealed <<<"$gj"); early=$(jq -r .ended_early <<<"$gj"); cof=$(jq -r .cancel_on_fail <<<"$gj"); orphaned=$(jq -r .orphaned <<<"$gj")
    mapfile -t members < <(jq -r '.members[]' <<<"$gj")
    # an unsealed group past its budget, or a sealed one with no member, is orphaned: made terminal cancelled
    if [ "$orphaned" = false ]; then
      if { [ "$sealed" = false ] && [ $(( $(date +%s) - $(jq -r .created_epoch <<<"$gj") )) -gt "$(jq -r .budget_s <<<"$gj")" ]; } || { [ "$sealed" = true ] && [ ${#members[@]} -eq 0 ]; } || [ "${2:-}" = orphan ]; then
        orphaned=true; sealed=true; early=true
        gj=$(jq '.orphaned = true | .sealed = true | .ended_early = true' <<<"$gj"); printf '%s\n' "$gj" > "$g/group.tmp" && mv -f "$g/group.tmp" "$g/group.json"
      fi
    fi
    kinds=(); trig=""; trigk=""; trigr=""; allt=1; i=0
    for m in "${members[@]}"; do
      md="$BUILDS/$m"; k=$(member_kind "$md"); kinds+=("$k")
      [ "$k" = open ] && allt=0
      if [ -z "$trig" ]; then case $k in blocked-unavailable|cancelled|infra_failed) trig=$m; trigk=$k;; test_failed|build_failed) [ "$cof" = true ] && { trig=$m; trigk=$k; };; esac
        [ -z "$trig" ] || trigr=$(jq -r '.digest // ""' "$md/terminal/state.json" 2>/dev/null); fi
    done
    if [ "$early" = false ] && [ -n "$trig" ]; then
      early=true; gj=$(jq --arg m "$trig" --arg k "$trigk" --arg r "$trigr" '.ended_early = true | .trigger = {build_id:$m,kind:$k,reason:$r}' <<<"$gj"); printf '%s\n' "$gj" > "$g/group.tmp" && mv -f "$g/group.tmp" "$g/group.json"
    fi
    if [ "$early" = true ]; then
      for m in "${members[@]}"; do [ -d "$BUILDS/$m/terminal" ] || { : > "$BUILDS/$m/cancelled_by_group"; echo "cancel $m"; }; done
    fi
    if [ "$sealed" = true ] || [ "$orphaned" = true ]; then :; else echo wait; exit 0; fi
    [ $allt = 1 ] || { echo wait; exit 0; }
    # every member is terminal: the group kind by precedence over the members that were not cancelled by the group, in member order
    gk=""; for pass in "test_failed build_failed" infra_failed blocked-unavailable cancelled; do
      for k in "${kinds[@]}"; do case " $pass " in *" $k "*) gk=$k; break 2;; esac; done; done
    [ -n "$gk" ] || { [ "$orphaned" = true ] && gk=cancelled || gk=completed; }
    [ "$orphaned" = true ] && gk=cancelled
    gkind=$gk; gec=$gk; [ "$gk" = completed ] && gec=succeeded
    mj=$(jq -n --argjson o "$orphaned" --argjson trig "$(jq '.trigger // null' <<<"$gj")" --arg gk "$gk" \
        --argjson k "$(for i in "${!members[@]}"; do jq -nc --arg b "${members[$i]}" --arg k "${kinds[$i]}" --arg r "$(jq -r '.digest // ""' "$BUILDS/${members[$i]}/terminal/state.json" 2>/dev/null)" '{build_id:$b,kind:$k,reason:(if $k == "blocked-unavailable" or $k == "infra_failed" then $r else "" end)}'; done | jq -sc .)" \
        '{group_kind:$gk,orphaned:$o,trigger:$trig,members:$k}')
    st=$(awk '{print $22}' /proc/$$/stat); tmpd="$g/terminal.tmp-$$-$st"
    rm -rf "$tmpd"; mkdir "$tmpd" && printf '%s\n' "$mj" > "$tmpd/members.json" \
      && jq -n --arg k "$gkind" --arg e "$gec" --arg t "$(now)" --arg r "$(jq -r '.trigger.reason // ""' <<<"$gj")" '{kind:$k,seq:0,exit_class:$e,claimed_at:$t,digest:$r}' > "$tmpd/state.json" \
      && printf 'claimed\n' > "$tmpd/callback.state" || { rm -rf "$tmpd"; echo error; exit 0; }
    sync "$tmpd/state.json" 2>/dev/null
    if mv -T "$tmpd" "$g/terminal" 2>/dev/null; then echo claimed; else rm -rf "$tmpd"; echo lost; fi
  ) 5>&-
  local line claimed=0
  while IFS= read -r line; do case $line in "cancel "*) tocancel+=("${line#cancel }");; claimed) claimed=1;; esac; done <<<"$out"
  if [ $claimed = 1 ] || { [ -d "$g/terminal" ] && [ -s "$g/callback.json" ] && [ "$(cat "$g/terminal/callback.state" 2>/dev/null)" = claimed ]; }; then
    bash -c '. "$1"; run_callback "$2"' _ "$EC" "$g" >> "$g/callback.log" 2>&1
    script_phase "$g"
  fi
  local m; for m in ${tocancel[@]+"${tocancel[@]}"}; do cmd_cancel "$m" >/dev/null 2>&1; done
  return 0; }
group_hook() { # builddir : a member reached a terminal state: evaluate its group
  [ -f "$1/submit.json" ] || return 0; local g; g=$(jq -r '.group // empty' "$1/submit.json" 2>/dev/null); [ -z "$g" ] || group_check "$g"; return 0; }
cmd_group_seal() {
  local g; valid_group "${1:-}" && g=$(gdir "$1") && [ -f "$g/group.json" ] || refuse group_unknown "${1:-}"
  ( exec 5>>"$g/.lock"; flock 5; jq '.sealed = true' "$g/group.json" > "$g/group.tmp" && mv -f "$g/group.tmp" "$g/group.json" ) 5>&-
  group_check "$1"; echo sealed; }
cmd_group_sweep() { local g; for g in "$BUILDS"/group/*/; do [ -f "${g}group.json" ] || continue; group_check "$(basename "$g")"; done; }
cmd_group_status() { local g; valid_group "${1:-}" && g=$(gdir "$1") && [ -f "$g/group.json" ] || refuse group_unknown "${1:-}"
  jq -n --slurpfile gj "$g/group.json" --argjson t "$(cat "$g/terminal/state.json" 2>/dev/null || echo null)" --argjson m "$(cat "$g/terminal/members.json" 2>/dev/null || echo null)" --arg cb "$(cat "$g/terminal/callback.state" 2>/dev/null)" \
    '{group:$gj[0],terminal:$t,members:$m,callback_state:$cb}'; }
cmd_group_next_expiry() { # seconds until the earliest unsealed group's budget runs out (empty when none): the hub's only timer
  local g now=$(date +%s) best="" e c b; for g in "$BUILDS"/group/*/; do [ -f "${g}group.json" ] && [ ! -d "${g}terminal" ] || continue
    [ "$(jq -r .sealed "${g}group.json")" = false ] || continue
    c=$(jq -r .created_epoch "${g}group.json"); b=$(jq -r .budget_s "${g}group.json"); e=$(( c + b + 1 - now )); [ $e -ge 1 ] || e=1
    { [ -z "$best" ] || [ "$e" -lt "$best" ]; } && best=$e; done; echo "$best"; }

# ---------------------------------------------------------------- terminal transitions (shares event_core's exactly-once rename)
# ---------------------------------------------------------------- long-op registry binding (T089a, 11.4.232)
# Every dispatched build is a registered long-op: registered by its pump BEFORE the remote start (purpose key of T005b, owner dispatch, the pump as the live owner), its
# liveness is PROVEN by progress (each consumed heartbeat feeds scripts/longops/heartbeat.sh; scripts/longops/classify.sh is the one owner of the HUNG decision), it
# ends in a terminal registry state with the verdict, and a driver stop hands it over (`handoff`) for a restarted driver to re-adopt through resume.
# Registry progress offset = progress_offset + stage of the heartbeat (both are non-decreasing, so the sum grows exactly when either grows).
OPID=""
reg_adopt() { # builddir : register (or re-adopt) the build; sets OPID. 0 ok, 3 purpose_conflict, 1 failed
  local d=$1 id=${1##*/} purpose np wall n=1 opid cls rc=0
  purpose=$(jq -r .purpose "$d/submit.json"); np=$(jq -r .no_progress_budget_s "$d/submit.json"); wall=$(jq -r .wallclock_cap_s "$d/submit.json")
  opid=$id
  while [ -e "$LONGOPS_DIR/ops/$opid.json" ]; do
    cls=$(bash "$LO/classify.sh" --op-id "$opid" 2>/dev/null | cut -f2)
    [ "$cls" != dead_owner ] || bash "$LO/reap.sh" --op-id "$opid" >/dev/null 2>&1     # a pump killed outright: proven dead from /proc, reaped without any signal
    n=$((n+1)); opid="$id-a$n"
  done
  bash "$LO/register.sh" --grammar build --purpose "$purpose" --owner dispatch --op-id "$opid" --pid $$ --no-progress-s "$np" --wall-s "$wall" \
      --container-label "catalogizer.op_id=dispatch-$id" >/dev/null 2>"$d/tmp/reg.err"; rc=$?
  if [ $rc -eq 4 ]; then bash "$LO/reap.sh" --purpose "$purpose" >/dev/null 2>&1; bash "$LO/register.sh" --grammar build --purpose "$purpose" --owner dispatch --op-id "$opid" --pid $$ \
      --no-progress-s "$np" --wall-s "$wall" --container-label "catalogizer.op_id=dispatch-$id" >/dev/null 2>"$d/tmp/reg.err"; rc=$?; fi
  [ $rc -eq 0 ] || { pump_log "registry refused rc=$rc $(head -c 200 "$d/tmp/reg.err" 2>/dev/null)"; [ $rc -eq 3 ] && return 3; return 1; }
  OPID=$opid; printf '%s\n' "$opid" > "$d/op_id.tmp-$$" && mv -f "$d/op_id.tmp-$$" "$d/op_id"; return 0; }   # `registered` until the first consumed heartbeat moves it to running
reg_beat() { [ -n "$OPID" ] && bash "$LO/heartbeat.sh" --op-id "$OPID" --progress-offset "$(( $1 + $2 ))" --elapsed-ms "$3" >/dev/null 2>&1; }
reg_class() { [ -n "$OPID" ] && bash "$LO/classify.sh" --op-id "$OPID" 2>/dev/null | cut -f2; }
reg_handoff() { [ -n "$OPID" ] && bash "$LO/release.sh" --op-id "$OPID" --state handoff --verdict driver_stop >/dev/null 2>&1; return 0; }
reg_finish() { # builddir : the build is terminal: end its registry op in the matching state (success is the verdict, never a process exit code)
  local d=$1 opid k ec why st
  [ -s "$d/op_id" ] || return 0; opid=$(cat "$d/op_id"); [ -e "$LONGOPS_DIR/ops/$opid.json" ] || return 0
  k=$(jq -r .kind "$d/terminal/state.json" 2>/dev/null); ec=$(jq -r '.exit_class // ""' "$d/terminal/state.json" 2>/dev/null); why=$(jq -r '.digest // ""' "$d/terminal/state.json" 2>/dev/null)
  case "$k:$ec" in
    completed:succeeded) st=complete; why=succeeded;;
    blocked-unavailable:*) case $why in build_liveness_lost|build_progress_flat|build_wallclock_exceeded) st=reaped;; *) st=blocked-escape;; esac;;
    completed:*) st=failed; why=$ec;;
    *) st=failed; [ -n "$why" ] || why=$k;;
  esac
  bash "$LO/release.sh" --op-id "$opid" --state "$st" --verdict "$why" >/dev/null 2>&1; return 0; }
after_terminal() { # builddir : every terminal site: registry op ended, callback script started, group evaluated
  reg_finish "$1"; script_phase "$1"; group_hook "$1"; }
cmd_pair() { # build_id : the partner of a build for the double build of T443, T447a and T566: every field of the purpose key but the variant matches, the iteration is ignored
  valid_id "${1:-}" && [ -f "$BUILDS/$1/submit.json" ] || refuse event_unknown_build "${1:-}"
  local re='^build:([^:]+):([^:]+):([0-9a-f]{64}):([0-9a-f]{64}):(repro-cold|primary)(:[^:]+)?$' mine want x p cand="" first="" k
  p=$(jq -r .purpose "$BUILDS/$1/submit.json"); [[ $p =~ $re ]] || return 0
  mine="${BASH_REMATCH[1]}:${BASH_REMATCH[2]}:${BASH_REMATCH[3]}:${BASH_REMATCH[4]}"; want=primary; [ "${BASH_REMATCH[5]}" = primary ] && want=repro-cold
  for x in $(ls -tr "$BUILDS"/b-*/submit.json 2>/dev/null); do
    p=$(jq -r .purpose "$x"); [[ $p =~ $re ]] || continue
    [ "${BASH_REMATCH[1]}:${BASH_REMATCH[2]}:${BASH_REMATCH[3]}:${BASH_REMATCH[4]}" = "$mine" ] && [ "${BASH_REMATCH[5]}" = "$want" ] || continue
    k=$(basename "$(dirname "$x")"); [ -n "$first" ] || first=$k
    # the deliverable: the primary record whose build completed and succeeded (a failed iteration is never the artifact the cold build is compared with)
    if [ "$(jq -r .kind "$BUILDS/$k/terminal/state.json" 2>/dev/null)" = completed ] && [ "$(jq -r .exit_class "$BUILDS/$k/terminal/state.json" 2>/dev/null)" = succeeded ]; then cand=$k; break; fi
  done
  [ -n "$cand" ] || cand=$first
  [ -z "$cand" ] || echo "$cand"; }
hub_live() { local p s; [ -s "$BUILDS/hub.pid" ] || return 1; read -r p s < "$BUILDS/hub.pid"; [ "${p:-0}" -gt 1 ] 2>/dev/null && [ "$(starttime "$p")" = "${s:-x}" ] && tr '\0' ' ' < "/proc/$p/cmdline" 2>/dev/null | grep -q 'event_hub.sh'; }
script_phase() { # builddir : after a terminal exists, start the callback runner for a script row (detached: the caller never waits for a callback script)
  local d=$1; [ -s "$d/callback.json" ] && [ -n "$(jq -r '.script_effect_key // empty' "$d/callback.json" 2>/dev/null)" ] || return 0
  hub_live && return 0       # a live hub runs the callback itself, from its own export of the callbacks table (it sees the terminal directory appear)
  setsid -f bash "$here/run_callback.sh" "$BUILDS" "${d#$BUILDS/}" >> "$d/script.log" 2>&1 < /dev/null 8>&- 6>&-; }
terminate() { terminate_core "$@"; local rc=$?; [ ! -d "$1/terminal" ] || after_terminal "$1"; return $rc; }
terminate_core() { # builddir kind exit_class reason : claim a terminal state and run the callback once (superseded when a terminal exists)
  # a separate process that sources event_core.sh: its lock_ok() reads /proc/$$/fd, which needs the locking shell to be the process itself
  bash -c '. "$1"; d=$2; kind=$3; ec=$4; reason=$5
    mkdir -p "$d/tmp"
    { exec 9>>"$d/.lock"; } 2>/dev/null && lock_ok 9 "$d/.lock" || { echo "REFUSED reason=lock_unavailable" >&2; exit 20; }
    flock -w 60 9 || { echo "REFUSED reason=lock_unavailable" >&2; exit 20; }
    if [ -d "$d/terminal" ]; then journal "$d" "$(jq -nc --arg k "$kind" "{event:\"superseded\",kind:\$k}")"; flock -u 9; echo superseded; exit 0; fi
    claim_terminal "$d" "$kind" 0 "$ec" "$reason"; crc=$?
    flock -u 9
    case $crc in 0) run_callback "$d"; echo "terminal $kind $reason";; 1) echo superseded;; *) echo "REFUSED reason=terminal_claim_failed" >&2; exit 20;; esac' _ "$EC" "$1" "$2" "$3" "${4:-}"
}

open_state() { # builddir -> terminal|queued|open
  local d=$1; if [ -d "$d/terminal" ]; then echo terminal; elif [ -e "$d/queued" ]; then echo queued; else echo open; fi; }
pump_alive() { local p s; [ -s "$1/pump.pid" ] || return 1; read -r p s < "$1/pump.pid"; [ -n "$p" ] && [ "$(starttime "$p")" = "$s" ] && tr '\0' ' ' < "/proc/$p/cmdline" 2>/dev/null | grep -q '_pump'; }
last_consumed() { local f n l=0; for f in "$1/consumed"/*; do n=${f##*/}; [[ $n =~ ^[1-9][0-9]{0,15}$ ]] || continue; [ "$n" -gt "$l" ] && l=$n; done; echo "$l"; }
spawn_pump() { local d=$1; setsid -f bash "$0" _pump "${d##*/}" >> "$d/pump.log" 2>&1 < /dev/null 8>&-; }   # fd 8 (the slots lock) is not inherited

# ---------------------------------------------------------------- submit
cmd_submit() {
  need jq python3 flock sha256sum openssl
  local purpose="" cb="" image="" src="" variant="" needb=0 wall="" budget="" hb=2 netnone=0 argv=() cbargs='{}' comp="" smode=tic decl=() cfs=() ctx="" progress=log+cpu grp="" cof=0 gbud=${DISPATCH_GROUP_BUDGET:-3600}
  while [ $# -gt 0 ]; do
    case $1 in
      --purpose) purpose=$2; shift 2;; --callback) cb=$2; shift 2;; --callback-args) cbargs=$2; shift 2;; --image) image=$2; shift 2;;
      --src) src=$2; shift 2;; --variant) variant=$2; shift 2;; --need) needb=$2; shift 2;; --wallclock-cap) wall=$2; shift 2;;
      --no-progress-budget) budget=$2; shift 2;; --heartbeat) hb=$2; shift 2;; --network-none) netnone=1; shift;;
      --component) comp=$2; shift 2;; --containerfile) cfs+=("$2"); shift 2;; --context) ctx=$2; shift 2;;
      --snapshot-mode) smode=$2; shift 2;; --declare) decl+=("$2"); shift 2;;
      --progress) progress=$2; shift 2;; --group) grp=$2; shift 2;; --cancel-on-fail) cof=1; shift;; --group-budget) gbud=$2; shift 2;;
      --) shift; argv=("$@"); break;; *) refuse usage "unknown option $1";;
    esac
  done
  local re='^build:([A-Za-z0-9_.-]{1,16}):([A-Za-z0-9_.-]{1,16}):([0-9a-f]{64}):([0-9a-f]{64}):(primary|repro-cold)(:([A-Za-z0-9_-]{1,16}))?$'
  [[ $purpose =~ $re ]] || refuse purpose_malformed "$purpose"
  local pv=${BASH_REMATCH[5]} psnap=${BASH_REMATCH[3]} pargv=${BASH_REMATCH[4]} pcomp=${BASH_REMATCH[1]} plane=${BASH_REMATCH[2]} piter=${BASH_REMATCH[7]}
  # the registered purpose key is rebuilt from its parts (T089a): every part decides identity, so two lanes, two iterations or the two variants never share a build
  purpose="build:$pcomp:$plane:$psnap:$pargv:$pv${piter:+:$piter}"
  [ -z "$variant" ] || [ "$variant" = "$pv" ] || refuse purpose_malformed "variant differs from the purpose key"
  variant=$pv
  local bsrc=cli
  if [ -z "$budget" ] || [ -z "$wall" ]; then
    local b_np b_wc; read -r b_np b_wc bsrc < <(purpose_budget "$pcomp" "$plane")
    [ -n "$budget" ] || budget=$b_np; [ -n "$wall" ] || wall=$b_wc
  fi
  [ -n "$cb" ] && [ -n "$image" ] && [ -n "$src" ] && [ ${#argv[@]} -gt 0 ] || refuse usage "--callback --image --src and a build argv are required"
  isint "$needb" && isint "$wall" && isint "$budget" && isint "$hb" && [ "$wall" -gt 0 ] && [ "$budget" -gt 0 ] && [ "$hb" -gt 0 ] || refuse usage "numeric options"
  jq -e 'type == "object"' <<<"$cbargs" >/dev/null 2>&1 || refuse usage "--callback-args must be a JSON object"
  [ -d "$src" ] || refuse usage "--src is not a directory"
  # callback: only an id of the closed registry (a TSV: id, script, effect key prefix, long-op purpose, no-progress budget)
  [ -r "$CALLBACKS" ] && awk -F'\t' -v i="$cb" '!/^#/ && $1 == i { f = 1 } END { exit !f }' "$CALLBACKS" || refuse callback_not_registered "$cb"
  # build groups: with --group the --callback names the GROUP callback (registered once); a member's own callback is the record-only no-op
  local gcb=""
  if [ -n "$grp" ]; then
    valid_group "$grp" || refuse usage "--group id"
    isint "$gbud" && [ "$gbud" -gt 0 ] || refuse usage "--group-budget"
    awk -F'\t' '!/^#/ && $1 == "record-only" { f = 1 } END { exit !f }' "$CALLBACKS" 2>/dev/null || refuse callback_not_registered "record-only (the member callback of a group)"
    group_validate "$grp" "$cb" "$cof"
    gcb=$cb; cb=record-only
  elif [ $cof = 1 ]; then refuse usage "--cancel-on-fail needs --group"; fi
  # the purpose key binds the snapshot and the argv: a key that does not match what is submitted is refused
  local gitsrc=0 msnap="" closure="" crepos="."
  case $smode in tic|cpa) ;; *) refuse usage "--snapshot-mode is tic or cpa";; esac
  case $progress in log|log+cpu) ;; *) refuse usage "--progress is log or log+cpu";; esac
  if is_git_src "$src"; then
    gitsrc=1
    [ -n "$comp" ] || refuse usage "a git source needs --component (the input closure is the unit that ships)"
    SNAP_MODE=$smode; SNAP_DECL=(${decl[@]+"${decl[@]}"}); SNAP_COMP=$comp; SNAP_CF=(${cfs[@]+"${cfs[@]}"}); SNAP_CTX=$ctx
    mkdir -p "$BUILDS/.snap" 2>/dev/null || refuse state_write_failed "$BUILDS/.snap"
    msnap="$BUILDS/.snap/manifest.$$.json"; closure="$BUILDS/.snap/closure.$$.json"; SNAP_TMPFILES="$msnap $closure"
    trap 'rm -f $SNAP_TMPFILES' EXIT
    snap_py manifest "$src" > "$msnap"                       # the refusals of the snapshot (undeclared_dirty_input, ...) surface here, in this shell
    [ "$(jq -r .digest "$msnap")" = "$psnap" ] || refuse purpose_digest_mismatch "source snapshot digest"
    snap_py closure "$src" > "$closure"                      # submodule_uninitialised surfaces here
    crepos=$(jq -r '.repos | join(",")' "$closure")
  else
    [ "$(snapshot_digest "$src")" = "$psnap" ] || refuse purpose_digest_mismatch "source snapshot digest"
  fi
  [ "$(argv_digest "${argv[@]}")" = "$pargv" ] || refuse purpose_digest_mismatch "argv digest"
  local id d
  id="b-$(printf '%s' "$purpose" | sha256sum | cut -c1-24)"; d="$BUILDS/$id"
  # single owner per purpose (11.4.232(B)): a live build is attached to, a completed one is reused (build once), a failed one needs a fresh iteration
  if [ -f "$d/submit.json" ]; then
    case $(open_state "$d") in
      terminal) if [ "$(jq -r .kind "$d/terminal/state.json" 2>/dev/null)" = completed ] && [ "$(jq -r .exit_class "$d/terminal/state.json")" = succeeded ] && artifact_ok "$d"; then echo "$id"; echo "reused (build once)" >&2; return 0; fi
                refuse purpose_already_terminal "$id (submit a fresh iteration)";;
      *) echo "$id"; echo "attached (open build of this purpose)" >&2; return 0;;
    esac
  fi
  # a purpose held in the long-op registry by something other than this build is a conflict, never started twice (T089a)
  local hj hr; hj=$(bash "$LO/holder.sh" "$purpose" 2>/dev/null)
  if [ -n "$hj" ] && [ "$hj" != none ]; then hr=$(jq -r '.run_id // ""' <<<"$hj" 2>/dev/null)
    case $hr in "$id"|"$id"-a*) ;; *) refuse purpose_conflict "$purpose is held by $hr";; esac; fi
  # disk gate first, memory at run time through run_pinned (60% ceiling), both before anything is created
  mkdir -p "$BUILDS" || refuse state_write_failed "$BUILDS"
  local dh_err dh_out="${DISPATCH_DISK_OUT:-$BUILDS/.disk}"
  dh_err=$(DISK_HEADROOM_OUT_DIR="$dh_out" bash "$ROOT/scripts/containers/disk_headroom.sh" --need "$needb" --op-id "submit-$id" 2>&1 >/dev/null) \
    || refuse "$(printf '%s' "$dh_err" | grep -o 'reason=[A-Za-z0-9_]*' | head -1 | cut -d= -f2 | sed 's/^$/disk_headroom_failed/')" "disk gate"
  # host selection (failover at submit time only)
  local host="" attempts='[]' transport=$TRANSPORT emit_sha="" emit_path="" tree="$src" mismatch=0
  if [ "$TRANSPORT" = local ]; then
    [ "${DISPATCH_ALLOW_LOCAL:-}" = 1 ] || refuse local_transport_forbidden "builds run on a qualified build host (owner decision C1); the local transport is a declared test/proof exception (DISPATCH_ALLOW_LOCAL=1)"
    host=local-proof; emit_path="$EMIT_DIR/remote/emit.sh"; emit_sha=$(cat "$EMIT_DIR/remote/emit.sh" "$EMIT_DIR/lib/evwait.py" | sha256sum | cut -d' ' -f1)
  else
    [ -r "$HOSTS_FILE" ] && [ -n "$(hosts_list)" ] || refuse no_qualified_host "host list $HOSTS_FILE absent or names no host"
  fi
  # secret
  bash "$EC" secret-init "$STATE" "$ROOT" >/dev/null || refuse secret_unusable "driver secret"
  local run="r-$(date -u +%Y%m%dT%H%M%SZ)-$(openssl rand -hex 6)" blocked=""
  mkdir -p "$d/tmp" "$d/consumed" || refuse state_write_failed "$d"
  if [ "$TRANSPORT" != local ]; then
    ssh_setup
    if host=$(qualify_hosts "$d/attempts.json"); then
      read -r emit_sha emit_path < <(ship_emitter "$host"); [ -n "$emit_path" ] || { host=""; blocked=host_unreachable; }
      if [ -n "$host" ] && [ $gitsrc = 1 ]; then ship_git "$host" "$src" "$msnap" "$crepos" "$psnap" "$comp" "$d" > "$d/tree.path"; rc=$?
        case $rc in 0) tree=$(cat "$d/tree.path");; 20) tree=""; mismatch=1;; *) host=""; blocked=host_unreachable;; esac; rm -f "$d/tree.path"
      elif [ -n "$host" ]; then tree=$(ship_tree "$host" "$src" "$psnap") || { host=""; blocked=host_unreachable; }; fi
    else blocked=no_qualified_host; fi
    attempts=$(cat "$d/attempts.json" 2>/dev/null || echo '[]'); rm -f "$d/attempts.json"
  fi
  if [ "$TRANSPORT" = local ] && [ $gitsrc = 1 ]; then
    ship_git local-proof "$src" "$msnap" "$crepos" "$psnap" "$comp" "$d" > "$d/tree.path"; rc=$?
    case $rc in 0) tree=$(cat "$d/tree.path");; 20) tree=""; mismatch=1;; *) blocked=host_unreachable;; esac; rm -f "$d/tree.path"
  fi
  local rdir; if [ "$TRANSPORT" = local ]; then rdir="$d/remote"; else rdir=".cache/catalogizer/builds/$id"; fi
  jq -n --arg id "$id" --arg run "$run" --arg v "$variant" --arg p "$purpose" --arg cb "$cb" --arg img "$image" --arg host "${host:-none}" --arg tr "$transport" \
    --arg src "$tree" --arg es "$emit_sha" --arg ep "$emit_path" --arg rd "$rdir" --arg sn "$psnap" --arg ad "$pargv" --arg t "$(now)" \
    --argjson argv "$(printf '%s\n' "${argv[@]}" | jq -R . | jq -sc .)" --argjson need "$needb" --argjson wall "$wall" --argjson bud "$budget" --argjson hb "$hb" \
    --argjson nn "$netnone" --argjson att "$attempts" --arg runp "$REMOTE_RUNP" --argjson tr_ "$(cat "$d/transfer.json" 2>/dev/null || echo 'null')" --arg comp "$comp" --arg crepos "$crepos" --arg bsrc "$bsrc" --arg prog "$progress" \
    '{schema:"dispatch-submit/1",build_id:$id,run_id:$run,variant:$v,purpose:$p,callback_id:$cb,image:$img,host:$host,transport:$tr,src:$src,emit_sha256:$es,emit_path:$ep,
      remote_dir:$rd,snapshot_digest:$sn,argv_digest:$ad,argv:$argv,need_bytes:$need,wallclock_cap_s:$wall,no_progress_budget_s:$bud,heartbeat_s:$hb,network_none:($nn==1),
      host_attempts:$att,runp:$runp,submitted_at:$t,started:false,transfer:$tr_,component:$comp,closure_repos:$crepos,budget_source:$bsrc,progress:$prog}' > "$d/submit.tmp" && mv -f "$d/submit.tmp" "$d/submit.json" || refuse state_write_failed "submit.json"
  # a script row of the callback table (column 2 is not `-`): the core's keyed-effect callback keeps its own bookkeeping key, the script (run_callback.sh) its own
  local srow; srow=$(awk -F'\t' -v i="$cb" '!/^#/ && $1 == i { print $2; exit }' "$CALLBACKS")
  if [ -n "$srow" ] && [ "$srow" != - ]; then
    jq -n --arg k "cbcore-$cb-$id" --arg sk "cb-$cb-$id" --arg cb "$cb" --argjson a "$cbargs" '{callback_id:$cb,effect_key:$k,script_effect_key:$sk,args:$a}' > "$d/callback.tmp"
  else
    jq -n --arg k "cb-$cb-$id" --arg cb "$cb" --argjson a "$cbargs" '{callback_id:$cb,effect_key:$k,args:$a}' > "$d/callback.tmp"
  fi && mv -f "$d/callback.tmp" "$d/callback.json"
  if [ -n "$grp" ]; then group_add "$grp" "$gcb" "$cof" "$gbud" "$id" || { rm -rf "$d"; refuse group_conflict "$grp"; }; jq --arg g "$grp" '.group = $g' "$d/submit.json" > "$d/submit.tmp" && mv -f "$d/submit.tmp" "$d/submit.json"; fi
  if [ $mismatch = 1 ]; then terminate "$d" infra_failed infra_failed snapshot_mismatch >&2; echo "$id"; return 0; fi
  if [ -n "$blocked" ]; then terminate "$d" blocked-unavailable blocked "$blocked" >&2; echo "$id"; return 0; fi
  # concurrency: more open builds than JOBS are queued, never started (started by the drain when a build ends)
  ( flock 8
    if [ "$(count_running "$d")" -ge "$JOBS" ]; then : > "$d/queued"; else spawn_pump "$d"; fi ) 8>>"$BUILDS/.slots.lock"
  ensure_hub
  echo "$id"
}

artifact_ok() { # builddir : the brought-back artifact tree still has the digest the completed event recorded
  local d=$1 want got
  [ -d "$d/artifacts" ] || return 1
  want=$(jq -r 'select(.event == null and .kind == "completed") | .artifact_manifest_sha256' "$d/events.jsonl" 2>/dev/null | tail -1)
  got=$(tree_manifest "$d/artifacts")
  [ -n "$want" ] && [ "$want" = "$got" ]
}
tree_manifest() { ( cd "$1" 2>/dev/null && LC_ALL=C find . -type f -print0 | LC_ALL=C sort -z | xargs -0 -r sha256sum | sed 's#  \./#  #' | sha256sum | cut -d' ' -f1 ); }
count_running() { local n=0 x; for x in "$BUILDS"/b-*/; do [ "${x%/}" != "${1:-}" ] || continue; [ -f "$x/submit.json" ] && [ ! -d "$x/terminal" ] && [ ! -e "$x/queued" ] && n=$((n+1)); done; echo "$n"; }

# ---------------------------------------------------------------- pump: one per open build, blocking on the event stream
pump_log() { printf '%s %s\n' "$(now)" "$*" >> "$PD/pump.log"; }
pump_end() { # after a terminal state: start the next queued build if a slot is free
  ( flock 8
    local x; for x in $(ls -tr "$BUILDS"/b-*/queued 2>/dev/null); do
      [ "$(count_running)" -lt "$JOBS" ] || break
      rm -f "$x"; spawn_pump "$(dirname "$x")"; break
    done ) 8>>"$BUILDS/.slots.lock"
}
cmd_pump() {
  local id=$1 d="$BUILDS/$1" drain=${2:-} run host hb budget wall key line rc po st el why try=0 maxtry=${DISPATCH_RECONNECTS:-3} sub status kind ec evok
  PD=$d
  valid_id "$id" && [ -f "$d/submit.json" ] || exit 2
  # one pump per build: a lock held for the pump's life (a second pump, from the hub or a resume racing a submit, leaves at once)
  { exec 6>>"$d/.pumplock"; } 2>/dev/null && flock -w 3 6 || exit 0   # a dying pump releases within moments; a live one holds it
  printf '%s %s\n' "$$" "$(starttime $$)" > "$d/pump.pid.tmp-$$" && mv -f "$d/pump.pid.tmp-$$" "$d/pump.pid"     # temp-then-rename: the hub reads it on the rename event
  run=$(jq -r .run_id "$d/submit.json"); host=$(jq -r .host "$d/submit.json"); hb=$(jq -r .heartbeat_s "$d/submit.json")
  budget=$(jq -r .no_progress_budget_s "$d/submit.json"); wall=$(jq -r .wallclock_cap_s "$d/submit.json")
  # a driver stop hands the build over to the registry (`handoff`), unless it already ended
  trap '[ -d "$d/terminal" ] || reg_handoff; exit 0' TERM
  key=$(bash "$EC" derive-key "$STATE" "$id" "$run" 2>/dev/null) || key=""
  if ! [[ $key =~ ^[0-9a-f]{64}$ ]]; then
    # the secret is lost or altered: every open build ends blocked-unavailable (driver_secret_lost), none is resubmitted
    [ -d "$d/terminal" ] || terminate "$d" blocked-unavailable blocked driver_secret_lost >> "$d/pump.log" 2>&1
    pump_end; exit 0
  fi
  if [ -z "$drain" ]; then   # registered BEFORE the remote start: the build is a long-op from here on
    reg_adopt "$d"; rc=$?
    if [ $rc -ne 0 ]; then terminate "$d" infra_failed infra_failed purpose_conflict >> "$d/pump.log" 2>&1; pump_end; exit 0; fi
  fi
  jq '.started = true' "$d/submit.json" > "$d/submit.tmp" && mv -f "$d/submit.tmp" "$d/submit.json"   # the submit record precedes the remote start
  while :; do
    local from; from=$(last_consumed "$d")
    local args=(--dir "$(jq -r .remote_dir "$d/submit.json")" --build-id "$id" --run-id "$run" --variant "$(jq -r .variant "$d/submit.json")" --host "$host"
                --image "$(jq -r .image "$d/submit.json")" --src "$(jq -r .src "$d/submit.json")" --runp "$(jq -r .runp "$d/submit.json")" --heartbeat "$hb"
                --op-id "dispatch-$id" --need "$(jq -r .need_bytes "$d/submit.json")" --from "$from" --progress "$(jq -r '.progress // "log+cpu"' "$d/submit.json")")
    [ "$(jq -r .network_none "$d/submit.json")" != true ] || args+=(--network-none)
    mapfile -t _argv < <(jq -r '.argv[]' "$d/submit.json")
    # a process substitution, not a coproc: bash closes a coproc's descriptors when it exits and would drop events still buffered in the pipe
    local emfd empid got=0
    exec {emfd}< <({ printf '%s\n' "$key" | remote_emit "$d" run "${args[@]}" -- "${_argv[@]}"; } 6>&-)   # the key reaches the emitter over its stdin only; the pump lock is not inherited by the reader's subshell or its children
    empid=$!
    while :; do
      line=""
      IFS= read -r -t "$((budget + 1))" -u "$emfd" line; rc=$?
      if [ $rc -gt 128 ]; then # silence past the no-progress budget: HUNG, proven from the stream, never from a living process; the registry judges it
        [ -n "$drain" ] && { kill "$empid" 2>/dev/null; break 2; }
        [ "$(reg_class)" = hung ] || continue
        cancel_remote "$d"; terminate "$d" blocked-unavailable blocked build_liveness_lost >> "$d/pump.log" 2>&1; kill "$empid" 2>/dev/null; pump_end; exit 0
      fi
      [ $rc -eq 0 ] || break                     # EOF of the stream
      got=1
      printf '%s' "$line" > "$d/tmp/ev.$$.json"
      kind=$(jq -r '.kind // empty' <<<"$line" 2>/dev/null)
      if [ "$kind" = completed ] && [ "$(jq -r '.exit_class // empty' <<<"$line" 2>/dev/null)" = succeeded ] && [ ! -d "$d/terminal" ]; then
        evok=$(python3 "$CRYPTO" verify-event "$STATE" "$BUILDS" "$ROOT" < "$d/tmp/ev.$$.json" 2>/dev/null | head -1)
        if [ "$evok" = ok ] && ! bring_back "$d" "$(jq -r .artifact_manifest_sha256 <<<"$line")"; then
          rm -f "$d/tmp/ev.$$.json"; kill "$empid" 2>/dev/null; pump_end; exit 0     # bring_back already claimed the terminal (infra_failed / artifact_unavailable)
        fi
      fi
      status=$(bash "$EC" consume "$BUILDS" "$STATE" "$d/tmp/ev.$$.json" 2>&1); rc=$?
      rm -f "$d/tmp/ev.$$.json"
      pump_log "consume rc=$rc $(printf '%s' "$status" | head -c 200 | tr '\n' ' ')"
      [ $rc -eq 0 ] || continue                  # refused events are never consumed and never advance the liveness view
      if [ "$kind" = completed ]; then kill "$empid" 2>/dev/null; [ ! -d "$d/terminal" ] || after_terminal "$d"; pump_end; exit 0; fi
      if [ "$kind" = heartbeat ] && [ -z "$drain" ]; then
        po=$(jq -r .progress_offset <<<"$line"); st=$(jq -r .stage <<<"$line"); el=$(jq -r .elapsed_monotonic_ms <<<"$line")
        reg_beat "$po" "$st" "$el"                       # the heartbeat feeds the registry; the registry decides HUNG (progress flat past no_progress_s, or the build host's own elapsed time past the cap)
        if [ "$(reg_class)" = hung ]; then
          if [ "$el" -gt $(( wall * 1000 )) ]; then why=build_wallclock_exceeded; else why=build_progress_flat; fi
          cancel_remote "$d"; terminate "$d" blocked-unavailable blocked "$why" >> "$d/pump.log" 2>&1; kill "$empid" 2>/dev/null; pump_end; exit 0
        fi
      fi
    done
    exec {emfd}<&-; wait "$empid" 2>/dev/null; rc=$?
    [ -d "$d/terminal" ] && [ -n "$drain" ] && exit 0
    [ -d "$d/terminal" ] && { pump_end; exit 0; }
    # the stream ended with no completed event: reconnect from the last acknowledged seq a bounded number of times
    try=$((try+1))
    if [ $try -gt "$maxtry" ]; then
      [ -n "$drain" ] && exit 0
      cancel_remote "$d"
      terminate "$d" blocked-unavailable blocked "$( [ "$rc" = 255 ] || [ "$got" = 0 ] && echo host_unreachable || echo build_liveness_lost )" >> "$d/pump.log" 2>&1
      pump_end; exit 0
    fi
    sleep "${DISPATCH_RECONNECT_DELAY:-1}"
  done
  exit 0
}

cancel_remote() { remote_emit "$1" cancel --dir "$(jq -r .remote_dir "$1/submit.json")" >/dev/null 2>&1 < /dev/null || true; }

bring_back() { # builddir manifest_sha : fetch the artifact tree, verify it against the verified event before the terminal claim. 1 = a terminal was claimed instead
  local d=$1 want=$2 got tmp="$1/artifacts.tmp-$$"
  rm -rf "$tmp"; mkdir -p "$tmp"
  if ! remote_emit "$d" fetch --dir "$(jq -r .remote_dir "$d/submit.json")" < /dev/null | tar -x -C "$tmp" 2>/dev/null; then
    rm -rf "$tmp"; terminate "$d" blocked-unavailable blocked artifact_unavailable >> "$d/pump.log" 2>&1; return 1
  fi
  got=$(tree_manifest "$tmp")
  if [ "$got" != "$want" ]; then rm -rf "$tmp"; terminate "$d" infra_failed infra_failed artifact_mismatch >> "$d/pump.log" 2>&1; return 1; fi
  rm -rf "$d/artifacts"; mv -T "$tmp" "$d/artifacts"
  return 0
}

# ---------------------------------------------------------------- status / wait / resume / cancel
cmd_status() {
  local d="$BUILDS/$1" st=open
  valid_id "$1" && [ -f "$d/submit.json" ] || refuse event_unknown_build "$1"
  st=$(open_state "$d")
  jq -n --arg id "$1" --arg st "$st" --slurpfile sub "$d/submit.json" --argjson cons "$(ls "$d/consumed" 2>/dev/null | jq -R . | jq -sc .)" \
    --arg cbs "$(cat "$d/terminal/callback.state" 2>/dev/null)" --arg cbr "$(cat "$d/terminal/callback.reason" 2>/dev/null)" \
    --argjson term "$(cat "$d/terminal/state.json" 2>/dev/null || echo null)" --argjson pa "$(pump_alive "$d" && echo true || echo false)" \
    '{build_id:$id,state:$st,purpose:$sub[0].purpose,host:$sub[0].host,run_id:$sub[0].run_id,consumed:$cons,pump_alive:$pa,
      terminal:(if $term == null then null else {kind:$term.kind,exit_class:$term.exit_class,reason:(if $term.kind == "blocked-unavailable" or $term.exit_class == "infra_failed" then $term.digest else null end)} end),
      callback_state:$cbs,callback_reason:$cbr}'
}
cmd_wait() { # build_id [S]: the core callback first; for a script row, then the script phase (waitstate on terminal/script.state)
  valid_id "$1" && [ -f "$BUILDS/$1/submit.json" ] || refuse event_unknown_build "$1"
  local ev="${DISPATCH_EVWAIT:-$here/lib/evwait.py}" d="$BUILDS/$1" rc out t0=$SECONDS left
  python3 -I "$ev" wait "$d" ${2:+"$2"}; rc=$?
  [ $rc -eq 0 ] || [ $rc -eq 1 ] || return $rc
  if [ -n "$(jq -r '.script_effect_key // empty' "$d/callback.json" 2>/dev/null)" ]; then
    left=""; [ -z "${2:-}" ] || { left=$(( $2 - (SECONDS - t0) )); [ "$left" -ge 1 ] || left=1; }
    out=$(python3 -I "$ev" waitstate "$d/terminal/script.state" done,failed $left); rc=$?
    [ $rc -eq 0 ] || { echo "$out"; return $rc; }
    echo "script_state=$out"; [ "$out" = done ] && return 0 || return 1
  fi
  return $rc; }
cmd_resume() {
  local d="$BUILDS/$1"; valid_id "$1" && [ -f "$d/submit.json" ] || refuse event_unknown_build "$1"
  if [ ! -d "$d/terminal" ] && pump_alive "$d"; then echo "pump running"; return 0; fi
  if [ -d "$d/terminal" ]; then
    # callback left claimed/running is re-run; remaining journal events are consumed (late ones are recorded late_ignored, the verdict never changes).
    # A pump that is still finishing its terminal work (registry release, callback script, group evaluation) does not block this: the drain pump takes the pump lock when it frees.
    bash "$EC" resume-callback "$BUILDS" "$1" >/dev/null 2>&1
    after_terminal "$d"
    bash "$0" _pump "$1" drain >> "$d/pump.log" 2>&1; echo "drained"; return 0
  fi
  rm -f "$d/queued"; spawn_pump "$d"; ensure_hub; echo "pump started"
}
cmd_cancel() {
  local d="$BUILDS/$1" p s; valid_id "$1" && [ -f "$d/submit.json" ] || refuse event_unknown_build "$1"
  cancel_remote "$d"
  bash "$EC" cancel "$BUILDS" "$1"
  [ ! -d "$d/terminal" ] || after_terminal "$d"
  if [ -s "$d/pump.pid" ]; then read -r p s < "$d/pump.pid"
    # exact pid > 1, proven ours by start time and cmdline; never a process-group signal
    if [ "${p:-0}" -gt 1 ] && [ "$(starttime "$p")" = "$s" ] && tr '\0' ' ' < "/proc/$p/cmdline" 2>/dev/null | grep -q '_pump'; then kill -TERM "$p" 2>/dev/null; fi
  fi
  rm -f "$d/queued"
}

main() {
  local c=${1:-}; shift || true
  case $c in
    submit) cmd_submit "$@";;
    status) [ $# -eq 1 ] || exit 2; cmd_status "$1";;
    wait)   [ $# -ge 1 ] || exit 2; cmd_wait "$@";;
    resume) [ $# -eq 1 ] || exit 2; cmd_resume "$1";;
    cancel) [ $# -eq 1 ] || exit 2; cmd_cancel "$1";;
    drain) pump_end;;
    pair) [ $# -eq 1 ] || exit 2; cmd_pair "$1";;
    group-seal) [ $# -eq 1 ] || exit 2; cmd_group_seal "$1";;
    group-sweep) cmd_group_sweep;;
    group-status) [ $# -eq 1 ] || exit 2; cmd_group_status "$1";;
    group-next-expiry) cmd_group_next_expiry;;
    snapshot) [ $# -eq 1 ] || exit 2; snapshot_digest "$1";;
    argv-digest) argv_digest "$@";;
    _pump)  cmd_pump "$@";;
    *) echo "usage: $0 submit|status|wait|resume|cancel|snapshot|argv-digest ..." >&2; exit 2;;
  esac
}
if [ "${BASH_SOURCE[0]}" = "$0" ]; then main "$@"; fi
