#!/usr/bin/env bash
#
# normalize_submodule_pointers.sh — Collapse the accumulated stack of
# constitution-inheritance pointer blocks in each owned submodule's CLAUDE.md /
# AGENTS.md down to exactly ONE project-agnostic pointer, removing other-project
# leaks (ATMOSphere / Lava / hardcoded constitution/ paths) per §11.4.28(B).
#
# Mechanism: every affected file has a clean original H1 (`# CLAUDE.md ...` /
# `# AGENTS.md ...`). Everything ABOVE that H1 is accumulated pointer junk and is
# replaced with a single canonical block. All original content (H1 onward) is
# preserved byte-for-byte.
#
# SAFETY: DRY_RUN=1 (default) reports per-file (junk lines stripped, H1 line) and
# changes nothing. Files without a clean H1 are SKIPPED (never guessed). No git
# ops. Reversible via git checkout (files are tracked inside each submodule).
#
# Usage: scripts/normalize_submodule_pointers.sh           # preview
#        DRY_RUN=0 scripts/normalize_submodule_pointers.sh # execute

set -uo pipefail
DRY_RUN="${DRY_RUN:-1}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

c_reset=$'\033[0m'; c_blue=$'\033[34m'; c_green=$'\033[32m'; c_yellow=$'\033[33m'; c_red=$'\033[31m'
log()  { printf '%s[norm]%s %s\n' "${c_blue}"  "${c_reset}" "$*"; }
ok()   { printf '%s[ ok ]%s %s\n' "${c_green}" "${c_reset}" "$*"; }
warn() { printf '%s[warn]%s %s\n' "${c_yellow}""${c_reset}" "$*"; }

CLAUDE_BLOCK='## INHERITED FROM the Helix Constitution

This module is governed by the Helix Constitution. All rules in the
constitution'\''s `CLAUDE.md` and the `Constitution.md` it references apply
unconditionally. Locate the constitution from any nested depth via its
`find_constitution.sh` helper — do NOT hardcode a path (this module stays
fully decoupled and project-agnostic per §11.4.28).

Canonical reference: https://github.com/HelixDevelopment/HelixConstitution
'

AGENTS_BLOCK='> ## INHERITED FROM the Helix Constitution
> This module is governed by the Helix Constitution — its `AGENTS.md` and
> the `Constitution.md` it references are authoritative. Locate the
> constitution from any nested depth via `find_constitution.sh`; never
> hardcode a path (full decoupling per §11.4.28).
> Canonical reference: https://github.com/HelixDevelopment/HelixConstitution
'

normalize() {
  local f="$1" kind="$2" block="$3"
  [ -f "${f}" ] || return 0
  # first original H1 line (single # + CLAUDE/AGENTS), NOT a "## INHERITED" or "> ##" pointer line
  local hl
  hl="$(awk '/^# (CLAUDE|AGENTS)/{print NR; exit}' "${f}")"
  if [ -z "${hl}" ]; then warn "no clean H1, SKIP (manual): ${f}"; return 0; fi
  local total junk before_inherit
  total="$(wc -l < "${f}" | tr -d ' ')"
  junk=$((hl - 1))
  before_inherit="$(grep -cF 'INHERITED FROM' "${f}")"
  if [ "${DRY_RUN}" = "1" ]; then
    printf '%s[plan]%s %-46s H1@%-4s strip %-3s pre-H1 lines, %s→1 INHERITED block\n' \
      "${c_yellow}" "${c_reset}" "${f}" "${hl}" "${junk}" "${before_inherit}"
    return 0
  fi
  { printf '%s\n' "${block}"; tail -n +"${hl}" "${f}"; } > "${f}.norm"
  mv "${f}.norm" "${f}"
  local after
  after="$(grep -cF 'INHERITED FROM' "${f}")"
  if [ "${after}" = "1" ] && grep -qE '^# (CLAUDE|AGENTS)' "${f}"; then
    ok "normalized ${f} (1 pointer, H1 preserved)"
  else
    warn "post-check anomaly ${f}: INHERITED=${after}; inspect"
  fi
}

log "Repo root: ${REPO_ROOT}"
[ "${DRY_RUN}" = "1" ] && warn "DRY_RUN=1 (default) — preview only." || warn "DRY_RUN=0 — files WILL be rewritten."

for p in $(git config -f .gitmodules --get-regexp '^submodule\..*\.path$' | awk '{print $2}' | grep '^submodules/' | grep -v '^constitution$' | sort); do
  normalize "${p}/CLAUDE.md" CLAUDE "${CLAUDE_BLOCK}"
  normalize "${p}/AGENTS.md" AGENTS "${AGENTS_BLOCK}"
done
ok "done"
