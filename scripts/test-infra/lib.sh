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
