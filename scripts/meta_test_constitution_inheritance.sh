#!/usr/bin/env bash
# =============================================================================
# meta_test_constitution_inheritance.sh — paired §1.1 meta-test for the
#   constitution-inheritance gate
# -----------------------------------------------------------------------------
# Purpose:
#   Prove that scripts/verify_constitution_inheritance.sh is NOT a bluff gate,
#   per Constitution §1.1 (false-positive immunity is an invariant). It does so
#   by invoking the constitution-side helper
#   `constitution/meta_test_inheritance.sh` with THIS project's gate
#   as the argument. That helper:
#     1. snapshots Constitution.md,
#     2. mutates it (strips the §11.4 anchor line the gate's INV2 asserts),
#     3. runs our gate and asserts the gate now FAILs,
#     4. restores Constitution.md,
#     5. exits 0 only if the gate correctly FAILed on the mutated constitution.
#   A gate that still PASSed on the mutated constitution would be a BLUFF GATE
#   and the helper exits non-zero — which this wrapper propagates.
#
# Usage:
#   bash scripts/meta_test_constitution_inheritance.sh
#     (run from the project root: /Volumes/T7/Projects/catalogizer)
#
# Inputs:
#   - $PROJECT_ROOT (optional) — overrides the auto-resolved project root.
#
# Outputs:
#   - The constitution-side helper's full output (mutation + gate result +
#     META-TEST PASS/FAIL verdict) on stdout/stderr.
#   - Exit status == the helper's exit status (0 = gate has teeth).
#
# Side-effects:
#   - The constitution-side helper temporarily mutates + restores
#     constitution/Constitution.md (snapshot/restore via a tmp file
#     and an EXIT trap). This wrapper performs NO direct mutation itself.
#
# Dependencies:
#   - bash, the constitution submodule present with meta_test_inheritance.sh,
#     scripts/verify_constitution_inheritance.sh.
#
# Cross-references:
#   - Constitution §1.1 (paired mutation / false-positive immunity).
#   - Constitution §11.4 (anti-bluff covenant).
#   - Constitution §11.4.18 (this in-source doc block).
#   - constitution/meta_test_inheritance.sh (the helper).
#   - scripts/verify_constitution_inheritance.sh (the gate under test).
# =============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="${PROJECT_ROOT:-$(cd "${SCRIPT_DIR}/.." && pwd)}"

HELPER="${PROJECT_ROOT}/constitution/meta_test_inheritance.sh"
GATE="${PROJECT_ROOT}/scripts/verify_constitution_inheritance.sh"

if [[ ! -f "${HELPER}" ]]; then
    echo "FAIL: constitution-side meta-test helper not found at ${HELPER}" >&2
    echo "      Run \`scripts/safe_submodule_init.sh\` first (never \`--recursive\` through constitution/; see docs/GOVERNANCE.md section 5)." >&2
    exit 2
fi
if [[ ! -f "${GATE}" ]]; then
    echo "FAIL: inheritance gate not found at ${GATE}" >&2
    exit 2
fi

# Run the helper FROM the project root so the gate resolves PROJECT_ROOT and
# the relative `constitution/...` paths identically to a normal run.
cd "${PROJECT_ROOT}"

echo "=== Paired §1.1 meta-test: proving the inheritance gate has teeth ==="
echo "Helper: ${HELPER}"
echo "Gate  : bash scripts/verify_constitution_inheritance.sh"
echo

set +e
bash "${HELPER}" "bash scripts/verify_constitution_inheritance.sh"
RC=$?
set -e

echo
if [[ "${RC}" -eq 0 ]]; then
    echo "META-TEST PASS: the gate correctly FAILs on a mutated Constitution (not a bluff gate)."
else
    echo "META-TEST FAIL (rc=${RC}): the gate did NOT behave correctly under mutation — see helper output above." >&2
fi

exit "${RC}"
