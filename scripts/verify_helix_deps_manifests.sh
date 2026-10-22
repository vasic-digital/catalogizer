#!/usr/bin/env bash
# verify_helix_deps_manifests.sh - thin wrapper over the canonical constitution 11.4.31 verifier.
# The engine lives in the constitution submodule (single source of truth, 11.4.28(C) / 11.4.177):
#   constitution/scripts/deps_manifest/verify_helix_deps_manifests.sh
# This wrapper only pins --root to this project's root and forwards every other argument.
# Companion guide: docs/scripts/verify_helix_deps_manifests.md
# Exit codes are the canonical ones, passed through unchanged:
#   0 PASS | 1 FAIL | 2 BLIND (also: canonical engine missing) | 64 usage error.
set -uo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]:-$0}")/.." && pwd)"
CANON="$ROOT/constitution/scripts/deps_manifest/verify_helix_deps_manifests.sh"
if [ ! -f "$CANON" ]; then
  {
    echo "BLIND: canonical verifier not found: $CANON"
    echo "  The constitution submodule is not checked out (or is too old to carry scripts/deps_manifest/)."
    echo "  Fix: git submodule update --init constitution   (never --recursive through constitution/), then fast-forward it to the latest pin."
  } >&2
  exit 2
fi
exec bash "$CANON" --root "$ROOT" "$@"
