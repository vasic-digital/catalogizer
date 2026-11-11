#!/usr/bin/env bash
# podman_env.sh - SOURCE it. Makes rootless podman lifecycle commands work in agent/harness shells and gives a safe start/stop wrapper pair.
#
# ROOT CAUSE (measured 2026-11-11): agent shells export DBUS_SESSION_BUS_ADDRESS='unix:path=/run/user/1000/bus,guid=<hex>'. crun's sd-bus call
# then fails with "sd-bus call: Operation not permitted: Permission denied: OCI permission denied" for ANY `podman run/start`. The same address
# WITHOUT the ",guid=..." suffix works. qa_podman_env_normalize strips ONLY the "guid=" parameter (each ';'-separated address keeps its
# transport and every other ",key=val" parameter).
#
#   qa_podman_env_normalize         idempotent; also defaults XDG_RUNTIME_DIR=/run/user/<uid>.
#   qa_stack_start <container>      normalise env, start, wait healthy (QA_HEALTH_TIMEOUT s, default 120).
#   qa_stack_stop  <container>      normalise env, stop.
# Safety: <container> must match ^<QA_STACK_PREFIX>... (default "${STACK:-catqa3}-", WITH the hyphen, so catqa1/catqa2/cattsrv-* are refused);
# only the named container is ever addressed; no prune, no pod-wide or wildcard operation.
# Backend: the Containers submodule runtime abstraction (digital.vasic.containers pkg/runtime, via docs/qa/lib/ctrctl) when `go` and the
# submodule checkout are present; otherwise plain rootless podman with a logged DEVIATION line (see docs/CONTAINERS_SUBMODULE_PROPOSAL.md).
# Force with QA_STACK_BACKEND=podman|containers. The ctrctl binary lives in a per-checkout cache dir (<cache>/catalogizer-qa/<12-hex sha256 of the
# repo root>/ctrctl, override with QA_CTRCTL_CACHE); it is built to "<bin>.tmp.<pid>" in that dir and atomically renamed into place.
# Inspect calls are bounded by QA_INSPECT_TIMEOUT seconds (default 10); this needs coreutils `timeout`.

# Remove ONLY the "guid=" key from each ';'-separated D-Bus address, keeping every other parameter and the transport.
_qa_strip_guid() {
  local in="${1-}" out="" part tr kept p oldifs="$IFS" more=1 noglob=1
  case $- in *f*) ;; *) noglob=0; set -f;; esac   # params like "a*" must never glob against cwd files
  while [ "$more" = 1 ]; do
    part="${in%%;*}"; if [ "$in" = "$part" ]; then more=0; else in="${in#*;}"; fi
    tr="${part%%:*}"
    if [ "$tr" != "$part" ]; then
      kept=""; IFS=,; for p in ${part#*:}; do case "$p" in guid=*) ;; *) kept="${kept:+$kept,}$p";; esac; done; IFS="$oldifs"
      part="$tr:$kept"
    fi
    out="${out:+$out;}$part"
  done
  [ "$noglob" = 0 ] && set +f
  printf '%s' "$out"
}

qa_podman_env_normalize() {
  if [ -n "${DBUS_SESSION_BUS_ADDRESS-}" ]; then
    DBUS_SESSION_BUS_ADDRESS="$(_qa_strip_guid "$DBUS_SESSION_BUS_ADDRESS")"
    export DBUS_SESSION_BUS_ADDRESS
  fi
  : "${XDG_RUNTIME_DIR:=/run/user/$(id -u)}"; export XDG_RUNTIME_DIR
}

_qa_stack_log() { echo "qa_stack: $*" >&2; }

_qa_stack_check_name() {
  local c="${1-}" pfx="${QA_STACK_PREFIX:-${STACK:-catqa3}-}"
  [[ "$c" =~ ^[a-z0-9][a-z0-9_.-]*$ ]] || { _qa_stack_log "refused: bad container name '$c'"; return 64; }
  case "$c" in "$pfx"*) ;; *) _qa_stack_log "refused: '$c' is outside prefix '$pfx' (other stacks are never touched)"; return 64;; esac
}

# Prints "containers <bin>", "podman", or "none". The ctrctl binary is rebuilt on EVERY call (go build is incremental, so this tracks main.go, go.mod and
# the whole Containers submodule source) into a per-checkout cache dir. Forced QA_STACK_BACKEND=containers never falls back to podman.
_qa_stack_backend() {
  local root bin tmpb forced="${QA_STACK_BACKEND:-auto}" cdir
  [ "$forced" = podman ] && { echo podman; return; }
  root="${QA_REPO_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)}"
  if command -v go >/dev/null 2>&1 && [ -f "$root/submodules/containers/pkg/runtime/podman.go" ] && [ -f "$root/docs/qa/lib/ctrctl/main.go" ]; then
    cdir="${QA_CTRCTL_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/catalogizer-qa/$(printf %s "$root" | sha256sum | cut -c1-12)}"
    bin="$cdir/ctrctl"; tmpb="$bin.tmp.${BASHPID:-$$}"   # BASHPID: $$ is identical in every subshell of one shell
    if mkdir -p "$cdir" && ( cd "$root/docs/qa/lib/ctrctl" && GOFLAGS=-mod=mod go build -o "$tmpb" . ) >&2 && [ -x "$tmpb" ] && mv -f "$tmpb" "$bin"; then
      echo "containers $bin"; return
    fi
    rm -f -- "${tmpb:?}"
    _qa_stack_log "ctrctl build FAILED"
  fi
  if [ "$forced" = containers ]; then _qa_stack_log "ERROR: QA_STACK_BACKEND=containers but the Containers-submodule backend is unavailable (no fallback to podman)"; echo none; return; fi
  echo podman
}

# One-line backend identity for logs/evidence: "backend=containers ctrctl_sha256=<hex> path=<bin>" or "backend=podman".
qa_stack_backend_info() {
  local b; b="$(_qa_stack_backend)"
  case "$b" in containers\ *) echo "backend=containers ctrctl_sha256=$(sha256sum "${b#containers }" | cut -d' ' -f1) path=${b#containers }";; *) echo "backend=$b";; esac
}

_qa_stack_run() { # <start|stop|status> <container>
  local b; b="$(_qa_stack_backend)"
  case "$b" in
    containers\ *) _qa_stack_log "backend=containers ctrctl_sha256=$(sha256sum "${b#containers }" | cut -d' ' -f1)"; "${b#containers }" "$1" "$2";;
    podman)
      if [ "${QA_STACK_BACKEND:-auto}" = podman ]; then _qa_stack_log "backend=podman FORCED by QA_STACK_BACKEND=podman DEVIATION 11.4.76/11.4.173: using rootless podman directly (Containers submodule runtime not used)"
      else _qa_stack_log "backend=podman DEVIATION 11.4.76/11.4.173: using rootless podman directly (Containers submodule runtime unavailable here)"; fi
      podman "$1" "$2" >/dev/null;;
    *) return 69;;
  esac
}

_qa_stack_state() { # prints "<running|other> <health>"
  local s h
  s="$(timeout "${QA_INSPECT_TIMEOUT:-10}" podman inspect "$1" --format '{{.State.Status}}' 2>/dev/null)" || return 1
  h="$(timeout "${QA_INSPECT_TIMEOUT:-10}" podman inspect "$1" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' 2>/dev/null)"
  echo "$s ${h:-none}"
}

qa_stack_wait_healthy() { # <container> [timeout]
  command -v timeout >/dev/null 2>&1 || { _qa_stack_log "ERROR: 'timeout' command not found (coreutils required to bound podman inspect)"; return 69; }
  local c="$1" t="${2:-${QA_HEALTH_TIMEOUT:-120}}" i=0 st end=$((SECONDS + ${2:-${QA_HEALTH_TIMEOUT:-120}}))
  while [ "$i" -lt "$t" ] && [ "$SECONDS" -lt "$end" ]; do
    st="$(_qa_stack_state "$c")" || { _qa_stack_log "$c not found"; return 1; }
    case "$st" in
      "running healthy"|"running none") _qa_stack_log "$c $st after ${i}s"; return 0;;
      "exited "*|"stopped "*|"dead "*) _qa_stack_log "$c is $st"; return 1;;
    esac
    sleep 1; i=$((i+1))
  done
  _qa_stack_log "$c not healthy after ${t}s"; return 1
}

qa_stack_start() {
  _qa_stack_check_name "${1-}" || return $?
  qa_podman_env_normalize
  local rc; _qa_stack_run start "$1"; rc=$?; [ "$rc" = 69 ] && return 69; [ "$rc" = 0 ] || { _qa_stack_log "start $1 failed"; return 1; }
  qa_stack_wait_healthy "$1"
}

qa_stack_stop() {
  _qa_stack_check_name "${1-}" || return $?
  qa_podman_env_normalize
  local rc; _qa_stack_run stop "$1"; rc=$?; [ "$rc" = 69 ] && return 69; [ "$rc" = 0 ] || { _qa_stack_log "stop $1 failed"; return 1; }
}
