#!/usr/bin/env bash
# check_licence_swallow.sh - owner decision 2026-10-07 (1): a licence-acceptance step must FAIL LOUDLY, never be swallowed (constitution 11.4.201, 11.4.1).
# Fails (exit 1) when a Dockerfile / Containerfile / compose file / shell script runs a licence acceptance (`sdkmanager --licenses`, or any command line
# naming `licenses` / `licences`) and then swallows its failure with `|| true` / `|| :` on the SAME logical command (backslash continuations are joined).
# Full-line comments are ignored, so text that merely MENTIONS the pattern does not fire (11.4.201 carrier rule). A `|| true` on an unrelated line does not fire.
# Usage: check_licence_swallow.sh [--root DIR] [--list]
#   --root DIR  tree to scan (default: the repository this script lives in). Pruned: .git, node_modules, submodules, .audit, scripts/containers/tests, scripts/tests
#   --list      print the files that were scanned (used by the test as a control needle: the instrument must see the files it is meant to judge)
# Exit: 0 clean; 1 at least one swallow (each reported `file:line: text`); 2 usage; 3 nothing scanned (blind instrument, a zero would be a lie).
# Contract: docs/scripts/check_licence_swallow.md. Test: scripts/tests/test_check_licence_swallow.sh.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/.." && pwd)"; LIST=0
while [ $# -gt 0 ]; do
  case "$1" in
    --root) [ $# -ge 2 ] || { echo "check_licence_swallow: --root needs a value" >&2; exit 2; }; ROOT="$2"; shift 2 ;;
    --list) LIST=1; shift ;;
    *) echo "check_licence_swallow: unknown argument $1" >&2; exit 2 ;;
  esac
done
[ -d "$ROOT" ] || { echo "check_licence_swallow: root is not a directory: $ROOT" >&2; exit 2; }
ROOT="$(cd "$ROOT" && pwd)"
mapfile -d '' FILES < <(cd "$ROOT" && find . \( -name .git -o -name node_modules -o -name submodules -o -name .audit \) -prune -o \
  -type f \( -name 'Dockerfile*' -o -name 'Containerfile*' -o -name 'docker-compose*.yml' -o -name 'docker-compose*.yaml' -o -name 'compose*.yml' -o -name '*.sh' \) \
  ! -path './scripts/containers/tests/*' ! -path './scripts/tests/*' -print0 | sort -z)
[ "${#FILES[@]}" -gt 0 ] || { echo "check_licence_swallow: REFUSED reason=nothing_scanned root=$ROOT" >&2; exit 3; }
if [ "$LIST" = 1 ]; then printf '%s\n' "${FILES[@]#./}"; exit 0; fi
HITS=0
for f in "${FILES[@]}"; do
  out="$(awk -v F="${f#./}" '
    function flush() {
      if (cur != "" && cur ~ /licen[cs]es/ && cur ~ /\|\|[ \t]*(true|:)([ \t;&)|"\047]|$)/) { printf "%s:%d: %s\n", F, start, cur; n++ }
      cur = ""
    }
    { line = $0
      if (cur == "") { if (line ~ /^[ \t]*#/) next; start = NR }
      cont = (line ~ /\\[ \t]*$/)
      sub(/\\[ \t]*$/, "", line)
      cur = cur (cur == "" ? "" : " ") line
      if (!cont) flush()
    }
    END { flush(); exit (n > 0 ? 1 : 0) }' "$ROOT/$f")"
  rc=$?
  if [ "$rc" = 1 ]; then printf '%s\n' "$out"; HITS=$((HITS + $(printf '%s\n' "$out" | wc -l))); fi
done
if [ "$HITS" -gt 0 ]; then echo "check_licence_swallow: FAIL $HITS licence-acceptance step(s) swallow their failure (remove the \`|| true\`)" >&2; exit 1; fi
echo "check_licence_swallow: OK ${#FILES[@]} files scanned, no licence-acceptance swallow"
exit 0
