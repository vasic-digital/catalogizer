#!/usr/bin/env bash
# =============================================================================
# verify_constitution_inheritance.sh — Constitution-inheritance VERIFICATION GATE
# -----------------------------------------------------------------------------
# Purpose:
#   Assert that the `constitution` submodule is really present and
#   genuinely inherited by this project, per Constitution §1.1 (paired-mutation
#   requirement) and §11.4.35 (canonical-root inheritance clarity). This is a
#   hard gate: ANY broken invariant exits non-zero with a directed message so
#   the failure cannot ship green.
#
#   The gate has TEETH: its paired meta-test
#   (scripts/meta_test_constitution_inheritance.sh) drives the constitution-side
#   helper `constitution/meta_test_inheritance.sh`, which mutates
#   Constitution.md (strips the §11.4 anchor line), runs THIS gate, and asserts
#   it FAILs — proving the gate is not a bluff gate (Constitution §1.1).
#
# Usage:
#   bash scripts/verify_constitution_inheritance.sh
#     (run from the project root: /Volumes/T7/Projects/catalogizer)
#
# Inputs:
#   - $PROJECT_ROOT (optional) — overrides the auto-resolved project root.
#   - The repository tree (constitution/** + parent governance docs).
#
# Outputs:
#   - Human-readable PASS/FAIL lines per invariant on stdout/stderr.
#   - Exit 0 ONLY when ALL invariants hold; non-zero (1) on ANY failure.
#
# Side-effects:
#   - NONE. Read-only: it only greps + stats files. It never mutates the tree.
#
# Dependencies:
#   - bash, grep (with -q / -F / -i support — GNU or BSD grep both work).
#
# Cross-references:
#   - Constitution §1.1 (false-positive immunity / paired mutation).
#   - Constitution §11.4 (anti-bluff covenant — END-USER QUALITY GUARANTEE).
#   - Constitution §11.4.18 (this in-source doc block).
#   - Constitution §11.4.35 (canonical-root inheritance clarity).
#   - constitution/meta_test_inheritance.sh (the paired helper).
#   - scripts/meta_test_constitution_inheritance.sh (the paired meta-test).
#   - tests/test_constitution_inheritance.sh (host-side runner).
# =============================================================================

set -euo pipefail

# --- Resolve the project root (parent of scripts/) ---------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="${PROJECT_ROOT:-$(cd "${SCRIPT_DIR}/.." && pwd)}"
CONST_DIR="${PROJECT_ROOT}/constitution"

FAILURES=0

fail() {
    # $1 = invariant id, $2 = directed message
    echo "FAIL[$1]: $2" >&2
    FAILURES=$((FAILURES + 1))
}

ok() {
    # $1 = invariant id, $2 = message
    echo "ok[$1]: $2"
}

# --- INV1: constitution/ exists -----------------------------------
if [[ -d "${CONST_DIR}" ]]; then
    ok INV1 "constitution/ exists"
else
    fail INV1 "constitution/ is MISSING at ${CONST_DIR} — run \`git submodule update --init --recursive\` to materialise the constitution submodule"
fi

# --- INV2: Constitution.md exists AND contains the verified §11.4 anchor ------
CONST_MD="${CONST_DIR}/Constitution.md"
ANCHOR_INV2='§11.4 End-user quality guarantee — forensic anchor'
if [[ -f "${CONST_MD}" ]]; then
    # The anchor literal appears BOTH as a Markdown SECTION HEADING (the
    # authoritative source-of-truth line) AND as a table-of-contents entry.
    # The constitution-side meta-test mutation strips ONLY the heading line, so
    # a plain `grep -qF` for the literal would still match the surviving ToC
    # entry and the gate would PASS on the mutated file — a BLUFF GATE.
    # We therefore require the literal on a Markdown HEADING line (begins with
    # one or more '#' then whitespace) so the ToC entry cannot satisfy INV2.
    # The verified literal itself is still matched fixed-string (no regex
    # metacharacters interpreted) by pre-filtering heading lines with grep, then
    # fixed-string matching the anchor with grep -qF.
    # NOTE: avoid `grep -E ... | grep -qF ...` here — under `set -o pipefail`
    # the `-q` consumer closes the pipe on first match, sending SIGPIPE to the
    # upstream grep and making the whole pipeline exit non-zero (false FAIL).
    # Match the heading lines into a variable, then fixed-string test that.
    INV2_HEADINGS="$(grep -E '^#{1,6}[[:space:]]' "${CONST_MD}" || true)"
    if printf "%s
" "${INV2_HEADINGS}" | grep -F "${ANCHOR_INV2}" >/dev/null; then
        ok INV2 "Constitution.md present + contains the §11.4 forensic-anchor HEADING"
    else
        fail INV2 "Constitution.md is present but the verified anchor HEADING '${ANCHOR_INV2}' is MISSING — the §11.4 End-user quality guarantee section was stripped or the file is not the canonical Constitution"
    fi
else
    fail INV2 "Constitution.md is MISSING at ${CONST_MD}"
fi

# --- INV3: CLAUDE.md exists AND contains the anti-bluff covenant heading ------
CONST_CLAUDE="${CONST_DIR}/CLAUDE.md"
ANCHOR_INV3='MANDATORY ANTI-BLUFF COVENANT'
if [[ -f "${CONST_CLAUDE}" ]]; then
    if grep -qF "${ANCHOR_INV3}" "${CONST_CLAUDE}"; then
        ok INV3 "constitution/CLAUDE.md present + contains '${ANCHOR_INV3}'"
    else
        fail INV3 "constitution/CLAUDE.md is present but '${ANCHOR_INV3}' is MISSING — the anti-bluff covenant is not inherited"
    fi
else
    fail INV3 "constitution/CLAUDE.md is MISSING at ${CONST_CLAUDE}"
fi

# --- INV4: AGENTS.md exists AND references the anti-bluff covenant ------------
CONST_AGENTS="${CONST_DIR}/AGENTS.md"
ANCHOR_INV4='Anti-bluff covenant'
if [[ -f "${CONST_AGENTS}" ]]; then
    if grep -qiF "${ANCHOR_INV4}" "${CONST_AGENTS}"; then
        ok INV4 "constitution/AGENTS.md present + references the anti-bluff covenant"
    else
        fail INV4 "constitution/AGENTS.md is present but '${ANCHOR_INV4}' (case-insensitive) is MISSING — the agent-facing covenant pointer is not inherited"
    fi
else
    fail INV4 "constitution/AGENTS.md is MISSING at ${CONST_AGENTS}"
fi

# --- INV5: parent governance docs reference constitution ----------
# NOTE: another subagent adds these pointers in parallel. If INV5 fails right
# now that is EXPECTED until the parent pointers land; the main stream re-runs
# this gate afterward.
PARENT_POINTER='constitution'
for parent in CLAUDE.md AGENTS.md CONSTITUTION.md; do
    parent_path="${PROJECT_ROOT}/${parent}"
    if [[ -f "${parent_path}" ]]; then
        if grep -qF "${PARENT_POINTER}" "${parent_path}"; then
            ok "INV5:${parent}" "${parent} references ${PARENT_POINTER}"
        else
            fail "INV5:${parent}" "${parent} does NOT reference '${PARENT_POINTER}' — add the inheritance pointer (parallel subagent may still be landing this; re-run after pointers land)"
        fi
    else
        fail "INV5:${parent}" "parent governance doc ${parent} is MISSING at ${parent_path}"
    fi
done

# --- Verdict -----------------------------------------------------------------
if [[ "${FAILURES}" -eq 0 ]]; then
    echo "PASS: constitution inheritance verified"
    exit 0
else
    echo "GATE FAILED: ${FAILURES} constitution-inheritance invariant(s) broken — see FAIL[...] lines above" >&2
    exit 1
fi
