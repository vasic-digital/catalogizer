#!/usr/bin/env bash
# require_verdicts.sh - refuse a gated step whose verdict file is missing or stale (T089; 11.4.135: absence of a verdict blocks exactly as a FAIL does).
#
# Usage   require_verdicts.sh --file <verdict.json> [--file ...] [--fingerprint <fp>] [--max-age-s <n>]
# A verdict file is JSON with `verdict` (must be PASS), `fingerprint` and `utc`. Refused (exit 1, one line per file with its reason):
#         missing | unreadable | not_pass (names the verdict) | stale_fingerprint (differs from --fingerprint, an absent field never matches) |
#         stale_age (utc older than --max-age-s, an absent or unparsable utc is stale, never fresh).
# Exits   0 every file passes; 1 at least one refused; 2 usage. Writes nothing. The clock is LONGOPS_NOW when set.
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
files=(); fp=""; maxage=""
while [ $# -gt 0 ]; do case "$1" in --file) files+=("${2:-}"); shift 2 ;; --fingerprint) fp=${2:-}; shift 2 ;; --max-age-s) maxage=${2:-}; shift 2 ;; *) lo_die usage_error "unknown argument $(printf '%q' "$1")" ;; esac; done
[ "${#files[@]}" -gt 0 ] || lo_die usage_error "at least one --file is required"
[ -z "$maxage" ] || [[ "$maxage" =~ ^[0-9]+$ ]] || lo_die usage_error "--max-age-s must be an integer"
rc=0
for f in "${files[@]}"; do
  [ -e "$f" ] || { printf 'refused\t%s\tmissing\n' "$f"; rc=1; continue; }
  v=$(jq -r '.verdict // ""' "$f" 2>/dev/null) || { printf 'refused\t%s\tunreadable\n' "$f"; rc=1; continue; }
  [ "$v" = PASS ] || { printf 'refused\t%s\tnot_pass (verdict=%s)\n' "$f" "${v:-absent}"; rc=1; continue; }
  if [ -n "$fp" ] && [ "$(jq -r '.fingerprint // ""' "$f")" != "$fp" ]; then printf 'refused\t%s\tstale_fingerprint\n' "$f"; rc=1; continue; fi
  if [ -n "$maxage" ]; then
    u=$(jq -r '.utc // ""' "$f"); t=$(date -u -d "$u" +%s 2>/dev/null) || t=""
    if [ -z "$t" ] || [ $(( $(lo_now) - t )) -gt "$maxage" ]; then printf 'refused\t%s\tstale_age (utc=%s)\n' "$f" "${u:-absent}"; rc=1; continue; fi
  fi
  printf 'ok\t%s\n' "$f"
done
exit $rc
