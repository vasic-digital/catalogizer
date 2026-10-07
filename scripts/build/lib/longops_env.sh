#!/usr/bin/env bash
# longops_env.sh - sourced by dispatch.sh and event_hub.sh: where the long-op registry of this driver lives (T089a, 11.4.232 A).
# bind_longops_env <builds_root> <checkout_root>: exports LONGOPS_DIR, LONGOPS_AUDIT, LONGOPS_BUILDS and LONGOPS_REPO for the scripts/longops/*.sh calls.
#   The registry sits next to the builds root (`<audit>/longops` for the default `<checkout>/.audit/builds`), so a test fixture that moves the builds root moves
#   the registry with it and never touches the real one. DISPATCH_LONGOPS_DIR overrides it. A registry outside the checkout (a fixture) may live on tmpfs.
bind_longops_env() {
  local b=$1 r=$2 d
  d="${DISPATCH_LONGOPS_DIR:-$(dirname "$b")/longops}"
  export LONGOPS_DIR="$d" LONGOPS_AUDIT="$(dirname "$d")" LONGOPS_BUILDS="$b" LONGOPS_REPO="$r"
  case "$d" in "$r"/*) ;; *) export LONGOPS_ALLOW_TMPFS=1;; esac
}
