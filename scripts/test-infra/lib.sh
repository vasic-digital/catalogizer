#!/usr/bin/env bash
# lib.sh - shared helpers of scripts/test-infra (T129-T134). Sourced, never run. Documented in docs/scripts/test_infra_lib.md.
# HERE is the directory of the CALLING script (siblings are found there: a mutation copy of one script keeps using the real siblings through TI_SCRIPT_DIR);
# ROOT is the repository root (TI_ROOT overrides it for a fixture).
TI_HERE="${TI_SCRIPT_DIR:-$(cd "$(dirname "${BASH_SOURCE[1]:-${BASH_SOURCE[0]}}")" && pwd)}"
TI_ROOT="${TI_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
TI_STATE_DIR="${TI_STATE_DIR:-$TI_ROOT/.audit/test-infra}"
TI_COMPOSE_FILE="${TI_COMPOSE_FILE:-$TI_ROOT/docker-compose.test-infra.yml}"
TI_LOCK="${TI_LOCK:-$TI_ROOT/build/containers/images.lock.yaml}"
ti_die()  { echo "test-infra: $1" >&2; exit "${2:-1}"; }
ti_refuse() { echo "test-infra: REFUSED reason=$1 ${2:-}" >&2; exit "${3:-1}"; }
# build ids: lowercase letters, digits and dash, 1..31 chars, so the compose project name stays a valid, short, label-safe name
ti_valid_id() { [[ "$1" =~ ^[a-z0-9][a-z0-9-]{0,30}$ ]]; }
ti_project() { echo "catalogizer-test-$1"; }
ti_state() { echo "$TI_STATE_DIR/$(ti_project "$1")"; }
ti_network() { echo "$(ti_project "$1")_test-network"; }
ti_need() { local c; for c in "$@"; do command -v "$c" >/dev/null 2>&1 || ti_die "dependency missing: $c" 2; done; }
# ti_env_get <envfile> <VAR>: value of VAR (no export, no shell evaluation of the file: credentials are plain tokens, the file is data)
ti_env_get() { sed -n "s/^$2=//p" "$1" | head -1; }
# the long-op registry (11.4.232) is the scripts/longops CLI of this repository; a fixture may relocate its state with the LONGOPS_* variables
ti_lo() { local s=$1; shift; bash "$TI_ROOT/scripts/longops/$s.sh" "$@"; }
# ti_pod <project>: the name of the pod podman-compose (< 1.5.0 default `in_pod`) creates for a compose project: unlabelled, so a LABEL filter never finds it.
ti_pod() { echo "pod_$1"; }
# ti_rm_resources <project>: removes this project's containers (label catalogizer.test_project=<project> AND project=catalogizer, label re-read per container), volumes and network
# (labelled), and the unlabelled podman-compose pod `pod_<project>` when no container is left in it. Nothing is selected by a name pattern. Sets TI_NC / TI_NN / TI_NV / TI_NP (counts
# removed); returns 1 when a resource of the project could not be removed. (WF12 F2: the pod and the out directories used to leak on every cycle.)
ti_rm_resources() {
  local P=$1 NET c v lab pod rc=0 cnt; NET="${P}_test-network"; TI_NC=0; TI_NN=0; TI_NV=0; TI_NP=0
  for c in $(podman ps -a -q --filter "label=catalogizer.test_project=$P" --filter "label=project=catalogizer" 2>/dev/null); do
    lab="$(podman inspect --format '{{index .Config.Labels "catalogizer.test_project"}}' "$c" 2>/dev/null)"
    [ "$lab" = "$P" ] || { echo "test-infra: skipping $c: label '$lab' is not $P" >&2; continue; }
    if podman rm -f -v "$c" >/dev/null 2>&1; then TI_NC=$((TI_NC+1)); else echo "test-infra: cannot remove container $c" >&2; rc=1; fi
  done
  for v in $(podman volume ls -q --filter "label=catalogizer.test_project=$P" 2>/dev/null); do
    if podman volume rm "$v" >/dev/null 2>&1; then TI_NV=$((TI_NV+1)); else rc=1; fi
  done
  if podman network exists "$NET" 2>/dev/null; then
    lab="$(podman network inspect "$NET" --format '{{index .Labels "catalogizer.test_project"}}' 2>/dev/null)"
    if [ "$lab" = "$P" ]; then if podman network rm "$NET" >/dev/null 2>&1; then TI_NN=$((TI_NN+1)); else echo "test-infra: cannot remove network $NET" >&2; rc=1; fi
    else echo "test-infra: network $NET is not labelled for $P, left alone" >&2; fi
  fi
  pod="$(ti_pod "$P")"
  if podman pod exists "$pod" 2>/dev/null; then
    cnt="$(podman ps -a -q --filter "pod=$pod" 2>/dev/null | grep -c .)"
    if [ "$cnt" = 0 ]; then if podman pod rm "$pod" >/dev/null 2>&1; then TI_NP=$((TI_NP+1)); else echo "test-infra: cannot remove pod $pod" >&2; rc=1; fi
    else echo "test-infra: pod $pod still holds $cnt container(s) that do not carry the label of $P, left alone" >&2; fi
  fi
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
# A path that is empty, absolute, `.`, contains `..`, or names `.env` / `.git` / `.audit/test-infra` / `.audit/out` is refused: the repository root and its secrets are never a view.
ti_view_dir() {
  local dest=$1 d; shift
  for d in "$@"; do
    case "$d" in ''|.|/*|*..*|.env*|*/.env*|.git|.git/*|.audit/test-infra*|.audit/out*) return 1;; esac
    mkdir -p "$dest/$d" && { [ -z "$(find "$TI_ROOT/$d" -maxdepth 1 -type f -print -quit)" ] || find "$TI_ROOT/$d" -maxdepth 1 -type f -exec cp -p -t "$dest/$d" {} +; } || return 1
  done
}
