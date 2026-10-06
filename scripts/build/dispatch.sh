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
EC="$here/event_core.sh"; CRYPTO="$here/lib/bev_crypto.py"
EMIT_DIR="${DISPATCH_EMIT_DIR:-$here}"           # directory holding remote/emit.sh and lib/evwait.py
BUILDS="${DISPATCH_BUILDS_ROOT:-$ROOT/.audit/builds}"
CHECKOUT_ID=$(printf '%s' "$ROOT" | sha256sum | cut -d' ' -f1)
STATE="${DISPATCH_STATE_DIR:-${XDG_STATE_HOME:-$HOME/.local/state}/catalogizer/$CHECKOUT_ID}"
HOSTS_FILE="${DISPATCH_HOSTS_FILE:-$ROOT/build/hosts.env}"
CALLBACKS="${DISPATCH_CALLBACKS_TSV:-$here/callbacks.tsv}"
TRANSPORT="${DISPATCH_TRANSPORT:-ssh}"
JOBS="${DISPATCH_JOBS:-1}"
REMOTE_RUNP="${DISPATCH_REMOTE_RUNP:-$ROOT/scripts/containers/run_pinned.sh}"
BLOCKED_REASONS="host_unreachable no_qualified_host build_liveness_lost build_progress_flat build_wallclock_exceeded driver_secret_lost artifact_unavailable signing_key_not_provisioned"

refuse() { printf 'REFUSED reason=%s %s\n' "$1" "${2:-}" >&2; exit 20; }
now() { date -u +%Y-%m-%dT%H:%M:%SZ; }
starttime() { awk '{print $22}' "/proc/$1/stat" 2>/dev/null; }
isint() { [[ ${1:-} =~ ^(0|[1-9][0-9]{0,15})$ ]]; }
need() { local t; for t in "$@"; do command -v "$t" >/dev/null 2>&1 || refuse dependency_missing "$t"; done; }
atomic() { local f=$1 t; t="$f.tmp-$$"; printf '%s\n' "$2" > "$t" && mv -f "$t" "$f"; }
valid_id() { [[ ${1:-} =~ ^[A-Za-z0-9_-]{1,128}$ ]]; }

snapshot_digest() { # dir : sha256 over the sorted "sha256  relpath" lines of every regular file (the .git directory excluded). The git-plumbing snapshot of T005b is OWED.
  ( cd "$1" 2>/dev/null && LC_ALL=C find . -path ./.git -prune -o -type f -print0 | LC_ALL=C sort -z | xargs -0 -r sha256sum | sha256sum | cut -d' ' -f1 ); }
argv_digest() { local a; for a in "$@"; do printf '%s\0' "$a"; done | sha256sum | cut -d' ' -f1; }

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
rsh() { local h=$1; shift; "${SSHC[@]}" "$h" "$@"; }       # one remote command string: ssh joins its arguments with spaces
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

# ---------------------------------------------------------------- terminal transitions (shares event_core's exactly-once rename)
terminate() { # builddir kind exit_class reason : claim a terminal state and run the callback once (superseded when a terminal exists)
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
  local purpose="" cb="" image="" src="" variant="" needb=0 wall=3600 budget=60 hb=2 netnone=0 argv=() cbargs='{}'
  while [ $# -gt 0 ]; do
    case $1 in
      --purpose) purpose=$2; shift 2;; --callback) cb=$2; shift 2;; --callback-args) cbargs=$2; shift 2;; --image) image=$2; shift 2;;
      --src) src=$2; shift 2;; --variant) variant=$2; shift 2;; --need) needb=$2; shift 2;; --wallclock-cap) wall=$2; shift 2;;
      --no-progress-budget) budget=$2; shift 2;; --heartbeat) hb=$2; shift 2;; --network-none) netnone=1; shift;;
      --) shift; argv=("$@"); break;; *) refuse usage "unknown option $1";;
    esac
  done
  local re='^build:([A-Za-z0-9_.-]+):([A-Za-z0-9_.-]+):([0-9a-f]{64}):([0-9a-f]{64}):(primary|repro-cold)(:([A-Za-z0-9_-]+))?$'
  [[ $purpose =~ $re ]] || refuse purpose_malformed "$purpose"
  local pv=${BASH_REMATCH[5]} psnap=${BASH_REMATCH[3]} pargv=${BASH_REMATCH[4]}
  [ -z "$variant" ] || [ "$variant" = "$pv" ] || refuse purpose_malformed "variant differs from the purpose key"
  variant=$pv
  [ -n "$cb" ] && [ -n "$image" ] && [ -n "$src" ] && [ ${#argv[@]} -gt 0 ] || refuse usage "--callback --image --src and a build argv are required"
  isint "$needb" && isint "$wall" && isint "$budget" && isint "$hb" && [ "$wall" -gt 0 ] && [ "$budget" -gt 0 ] && [ "$hb" -gt 0 ] || refuse usage "numeric options"
  jq -e 'type == "object"' <<<"$cbargs" >/dev/null 2>&1 || refuse usage "--callback-args must be a JSON object"
  [ -d "$src" ] || refuse usage "--src is not a directory"
  # callback: only an id of the closed registry (a TSV: id, script, effect key prefix, long-op purpose, no-progress budget)
  [ -r "$CALLBACKS" ] && awk -F'\t' -v i="$cb" '!/^#/ && $1 == i { f = 1 } END { exit !f }' "$CALLBACKS" || refuse callback_not_registered "$cb"
  # the purpose key binds the snapshot and the argv: a key that does not match what is submitted is refused
  [ "$(snapshot_digest "$src")" = "$psnap" ] || refuse purpose_digest_mismatch "source snapshot digest"
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
  # disk gate first, memory at run time through run_pinned (60% ceiling), both before anything is created
  mkdir -p "$BUILDS" || refuse state_write_failed "$BUILDS"
  local dh_err dh_out="${DISPATCH_DISK_OUT:-$BUILDS/.disk}"
  dh_err=$(DISK_HEADROOM_OUT_DIR="$dh_out" bash "$ROOT/scripts/containers/disk_headroom.sh" --need "$needb" --op-id "submit-$id" 2>&1 >/dev/null) \
    || refuse "$(printf '%s' "$dh_err" | grep -o 'reason=[A-Za-z0-9_]*' | head -1 | cut -d= -f2 | sed 's/^$/disk_headroom_failed/')" "disk gate"
  # host selection (failover at submit time only)
  local host="" attempts='[]' transport=$TRANSPORT emit_sha="" emit_path="" tree="$src"
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
      [ -z "$host" ] || { tree=$(ship_tree "$host" "$src" "$psnap") || { host=""; blocked=host_unreachable; }; }
    else blocked=no_qualified_host; fi
    attempts=$(cat "$d/attempts.json" 2>/dev/null || echo '[]'); rm -f "$d/attempts.json"
  fi
  local rdir; if [ "$TRANSPORT" = local ]; then rdir="$d/remote"; else rdir=".cache/catalogizer/builds/$id"; fi
  jq -n --arg id "$id" --arg run "$run" --arg v "$variant" --arg p "$purpose" --arg cb "$cb" --arg img "$image" --arg host "${host:-none}" --arg tr "$transport" \
    --arg src "$tree" --arg es "$emit_sha" --arg ep "$emit_path" --arg rd "$rdir" --arg sn "$psnap" --arg ad "$pargv" --arg t "$(now)" \
    --argjson argv "$(printf '%s\n' "${argv[@]}" | jq -R . | jq -sc .)" --argjson need "$needb" --argjson wall "$wall" --argjson bud "$budget" --argjson hb "$hb" \
    --argjson nn "$netnone" --argjson att "$attempts" --arg runp "$REMOTE_RUNP" \
    '{schema:"dispatch-submit/1",build_id:$id,run_id:$run,variant:$v,purpose:$p,callback_id:$cb,image:$img,host:$host,transport:$tr,src:$src,emit_sha256:$es,emit_path:$ep,
      remote_dir:$rd,snapshot_digest:$sn,argv_digest:$ad,argv:$argv,need_bytes:$need,wallclock_cap_s:$wall,no_progress_budget_s:$bud,heartbeat_s:$hb,network_none:($nn==1),
      host_attempts:$att,runp:$runp,submitted_at:$t,started:false}' > "$d/submit.tmp" && mv -f "$d/submit.tmp" "$d/submit.json" || refuse state_write_failed "submit.json"
  jq -n --arg k "cb-$cb-$id" --arg cb "$cb" --argjson a "$cbargs" '{callback_id:$cb,effect_key:$k,args:$a}' > "$d/callback.tmp" && mv -f "$d/callback.tmp" "$d/callback.json"
  if [ -n "$blocked" ]; then terminate "$d" blocked-unavailable blocked "$blocked" >&2; echo "$id"; return 0; fi
  # concurrency: more open builds than JOBS are queued, never started (started by the drain when a build ends)
  ( flock 8
    if [ "$(count_running "$d")" -ge "$JOBS" ]; then : > "$d/queued"; else spawn_pump "$d"; fi ) 8>>"$BUILDS/.slots.lock"
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
  local id=$1 d="$BUILDS/$1" drain=${2:-} run host hb budget wall key line rc po st el lastpo=-1 lastst=-1 lastadv=0 try=0 maxtry=${DISPATCH_RECONNECTS:-3} sub status kind ec evok
  PD=$d
  valid_id "$id" && [ -f "$d/submit.json" ] || exit 2
  printf '%s %s\n' "$$" "$(starttime $$)" > "$d/pump.pid"
  run=$(jq -r .run_id "$d/submit.json"); host=$(jq -r .host "$d/submit.json"); hb=$(jq -r .heartbeat_s "$d/submit.json")
  budget=$(jq -r .no_progress_budget_s "$d/submit.json"); wall=$(jq -r .wallclock_cap_s "$d/submit.json")
  trap 'exit 0' TERM
  key=$(bash "$EC" derive-key "$STATE" "$id" "$run" 2>/dev/null) || key=""
  if ! [[ $key =~ ^[0-9a-f]{64}$ ]]; then
    # the secret is lost or altered: every open build ends blocked-unavailable (driver_secret_lost), none is resubmitted
    [ -d "$d/terminal" ] || terminate "$d" blocked-unavailable blocked driver_secret_lost >> "$d/pump.log" 2>&1
    pump_end; exit 0
  fi
  jq '.started = true' "$d/submit.json" > "$d/submit.tmp" && mv -f "$d/submit.tmp" "$d/submit.json"   # the submit record precedes the remote start
  while :; do
    local from; from=$(last_consumed "$d")
    local args=(--dir "$(jq -r .remote_dir "$d/submit.json")" --build-id "$id" --run-id "$run" --variant "$(jq -r .variant "$d/submit.json")" --host "$host"
                --image "$(jq -r .image "$d/submit.json")" --src "$(jq -r .src "$d/submit.json")" --runp "$(jq -r .runp "$d/submit.json")" --heartbeat "$hb"
                --op-id "dispatch-$id" --need "$(jq -r .need_bytes "$d/submit.json")" --from "$from")
    [ "$(jq -r .network_none "$d/submit.json")" != true ] || args+=(--network-none)
    mapfile -t _argv < <(jq -r '.argv[]' "$d/submit.json")
    # a process substitution, not a coproc: bash closes a coproc's descriptors when it exits and would drop events still buffered in the pipe
    local emfd empid got=0
    exec {emfd}< <(printf '%s\n' "$key" | remote_emit "$d" run "${args[@]}" -- "${_argv[@]}")   # the key reaches the emitter over its stdin only
    empid=$!
    while :; do
      line=""
      IFS= read -r -t "$budget" -u "$emfd" line; rc=$?
      if [ $rc -gt 128 ]; then # silence past the no-progress budget: HUNG, proven from the stream, never from a living process
        [ -n "$drain" ] && { kill "$empid" 2>/dev/null; break 2; }
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
      if [ "$kind" = completed ]; then kill "$empid" 2>/dev/null; pump_end; exit 0; fi
      if [ "$kind" = heartbeat ] && [ -z "$drain" ]; then
        po=$(jq -r .progress_offset <<<"$line"); st=$(jq -r .stage <<<"$line"); el=$(jq -r .elapsed_monotonic_ms <<<"$line")
        if [ "$po" != "$lastpo" ] || [ "$st" != "$lastst" ]; then lastpo=$po; lastst=$st; lastadv=$el; fi
        if [ "$el" -gt $(( wall * 1000 )) ]; then cancel_remote "$d"; terminate "$d" blocked-unavailable blocked build_wallclock_exceeded >> "$d/pump.log" 2>&1; kill "$empid" 2>/dev/null; pump_end; exit 0; fi
        if [ $(( el - lastadv )) -gt $(( budget * 1000 )) ]; then cancel_remote "$d"; terminate "$d" blocked-unavailable blocked build_progress_flat >> "$d/pump.log" 2>&1; kill "$empid" 2>/dev/null; pump_end; exit 0; fi
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
cmd_wait() { valid_id "$1" && [ -f "$BUILDS/$1/submit.json" ] || refuse event_unknown_build "$1"; exec python3 -I "${DISPATCH_EVWAIT:-$here/lib/evwait.py}" wait "$BUILDS/$1" ${2:+"$2"}; }
cmd_resume() {
  local d="$BUILDS/$1"; valid_id "$1" && [ -f "$d/submit.json" ] || refuse event_unknown_build "$1"
  if pump_alive "$d"; then echo "pump running"; return 0; fi
  if [ -d "$d/terminal" ]; then
    # callback left claimed/running is re-run; remaining journal events are consumed (late ones are recorded late_ignored, the verdict never changes)
    bash "$EC" resume-callback "$BUILDS" "$1" >/dev/null 2>&1
    bash "$0" _pump "$1" drain >> "$d/pump.log" 2>&1; echo "drained"; return 0
  fi
  rm -f "$d/queued"; spawn_pump "$d"; echo "pump started"
}
cmd_cancel() {
  local d="$BUILDS/$1" p s; valid_id "$1" && [ -f "$d/submit.json" ] || refuse event_unknown_build "$1"
  cancel_remote "$d"
  bash "$EC" cancel "$BUILDS" "$1"
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
    snapshot) [ $# -eq 1 ] || exit 2; snapshot_digest "$1";;
    argv-digest) argv_digest "$@";;
    _pump)  cmd_pump "$@";;
    *) echo "usage: $0 submit|status|wait|resume|cancel|snapshot|argv-digest ..." >&2; exit 2;;
  esac
}
if [ "${BASH_SOURCE[0]}" = "$0" ]; then main "$@"; fi
