#!/usr/bin/env bash
# podman_env.sh - SOURCE it. Makes rootless podman lifecycle commands work in agent/harness shells and gives a safe start/stop wrapper pair.
#
# ROOT CAUSE (measured 2026-11-11): agent shells export DBUS_SESSION_BUS_ADDRESS='unix:path=/run/user/1000/bus,guid=<hex>'. crun's sd-bus call
# then fails with "sd-bus call: Operation not permitted: Permission denied: OCI permission denied" for ANY `podman run/start`. The same address
# WITHOUT the ",guid=..." suffix works. qa_podman_env_normalize strips every ",key=val" parameter after the path.
#
#   qa_podman_env_normalize         idempotent; also defaults XDG_RUNTIME_DIR=/run/user/<uid>.
#   qa_stack_start <container>      normalise env, start, wait healthy (QA_HEALTH_TIMEOUT s, default 120).
#   qa_stack_stop  <container>      normalise env, stop.
# Safety: <container> must match ^<QA_STACK_PREFIX>... (default "${STACK:-catqa3}-", WITH the hyphen, so catqa1/catqa2/cattsrv-* are refused);
# only the named container is ever addressed; no prune, no pod-wide or wildcard operation.
# Backend: the Containers submodule runtime abstraction (digital.vasic.containers pkg/runtime, via docs/qa/lib/ctrctl) when `go` and the
# submodule checkout are present; otherwise plain rootless podman with a logged DEVIATION line (see docs/CONTAINERS_SUBMODULE_PROPOSAL.md).
# Force with QA_STACK_BACKEND=podman|containers.

qa_podman_env_normalize() {
  if [ -n "${DBUS_SESSION_BUS_ADDRESS-}" ]; then
    DBUS_SESSION_BUS_ADDRESS="${DBUS_SESSION_BUS_ADDRESS%%,*}"
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

_qa_stack_backend() { # prints "containers <bin>" or "podman"
  local here root bin
  here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"; root="$here/../../.."
  case "${QA_STACK_BACKEND:-auto}" in podman) echo podman; return;; esac
  if command -v go >/dev/null 2>&1 && [ -f "$root/submodules/containers/pkg/runtime/podman.go" ]; then
    bin="${QA_CTRCTL_BIN:-${XDG_CACHE_HOME:-$HOME/.cache}/catalogizer-qa/ctrctl}"
    if [ ! -x "$bin" ] || [ -n "$(find "$here/ctrctl/main.go" -newer "$bin" 2>/dev/null)" ]; then
      mkdir -p "$(dirname "$bin")" && ( cd "$here/ctrctl" && GOFLAGS=-mod=mod go build -o "$bin" . ) >&2 || { echo podman; return; }
    fi
    echo "containers $bin"; return
  fi
  [ "${QA_STACK_BACKEND:-auto}" = containers ] && { _qa_stack_log "containers backend requested but unavailable"; echo none; return; }
  echo podman
}

_qa_stack_run() { # <start|stop|status> <container>
  local b; b="$(_qa_stack_backend)"
  case "$b" in
    containers\ *) "${b#containers }" "$1" "$2";;
    podman) _qa_stack_log "DEVIATION 11.4.76/11.4.173: using rootless podman directly (Containers submodule runtime unavailable here)"; podman "$1" "$2" >/dev/null;;
    *) return 69;;
  esac
}

_qa_stack_state() { # prints "<running|other> <health>"
  local s h
  s="$(podman inspect "$1" --format '{{.State.Status}}' 2>/dev/null)" || return 1
  h="$(podman inspect "$1" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}none{{end}}' 2>/dev/null)"
  echo "$s ${h:-none}"
}

qa_stack_wait_healthy() { # <container> [timeout]
  local c="$1" t="${2:-${QA_HEALTH_TIMEOUT:-120}}" i=0 st
  while [ "$i" -lt "$t" ]; do
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
  _qa_stack_run start "$1" || { _qa_stack_log "start $1 failed"; return 1; }
  qa_stack_wait_healthy "$1"
}

qa_stack_stop() {
  _qa_stack_check_name "${1-}" || return $?
  qa_podman_env_normalize
  _qa_stack_run stop "$1" || { _qa_stack_log "stop $1 failed"; return 1; }
}
