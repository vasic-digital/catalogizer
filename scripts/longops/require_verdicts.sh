#!/usr/bin/env bash
# require_verdicts.sh - refuse a gated step whose verdict file is missing or stale (T089; 11.4.135: absence of a verdict blocks exactly as a FAIL does).
#
# Usage   require_verdicts.sh --file <verdict.json> [--file ...] [--fingerprint <fp>] [--max-age-s <n>]
# A verdict file is JSON with `verdict` (must be the string PASS), `fingerprint` and `utc`. Refused (exit 1, one line per file with its reason):
#         missing | unreadable | not_pass (names the verdict) | stale_fingerprint (differs from --fingerprint, an absent field never matches) |
#         stale_age (utc older than --max-age-s; an absent, null, empty, non-ISO or unparsable utc is stale, never fresh; a utc more than LONGOPS_CLOCK_SKEW_S (default 300) in the FUTURE is
#         `stale_age (future_utc)` too: a verdict dated next year never expires).
#         A utc must match YYYY-MM-DDTHH:MM:SSZ and is NEVER passed raw to `date -d` (which reads "", "now" and "next year" as a time, LO-A9).
#         An option given with an EMPTY value (an unset caller variable) is a usage error (2): an empty --fingerprint or --max-age-s never means "no check".
# Exits   0 every file passes; 1 at least one refused; 2 usage. Writes nothing. The clock is LONGOPS_NOW when set.
set -u
. "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
files=(); fp=""; maxage=""; have_fp=0; have_age=0
while [ $# -gt 0 ]; do case "$1" in --file) lo_need "$@"; files+=("$2"); shift 2 ;; --fingerprint) lo_need "$@"; fp=$2; have_fp=1; shift 2 ;; --max-age-s) lo_need "$@"; maxage=$2; have_age=1; shift 2 ;; *) lo_die usage_error "unknown argument $(printf '%q' "$1")" ;; esac; done
[ "${#files[@]}" -gt 0 ] || lo_die usage_error "at least one --file is required"
[ "$have_fp" = 0 ] || [ -n "$fp" ] || lo_die usage_error "--fingerprint was given an empty value: an empty value never means \"no check\" (an unset caller variable?)"
[ "$have_age" = 0 ] || lo_uint "$maxage" || lo_die usage_error "--max-age-s must be a canonical non-negative integer (an empty value never means \"no check\")"
SKEW=${LONGOPS_CLOCK_SKEW_S:-300}; lo_uint "$SKEW" || lo_die usage_error "LONGOPS_CLOCK_SKEW_S must be a canonical non-negative integer"
rc=0
for f in "${files[@]}"; do
  [ -e "$f" ] || { printf 'refused\t%s\tmissing\n' "$f"; rc=1; continue; }
  v=$(jq -r 'if type=="object" then (.verdict // "") else "" end' "$f" 2>/dev/null) || { printf 'refused\t%s\tunreadable\n' "$f"; rc=1; continue; }
  [ "$v" = PASS ] || { printf 'refused\t%s\tnot_pass (verdict=%s)\n' "$f" "${v:-absent}"; rc=1; continue; }
  if [ "$have_fp" = 1 ] && [ "$(jq -r '.fingerprint // ""' "$f")" != "$fp" ]; then printf 'refused\t%s\tstale_fingerprint\n' "$f"; rc=1; continue; fi
  if [ "$have_age" = 1 ]; then
    u=$(jq -r '.utc | if type=="string" then . else "" end' "$f")
    t=""
    if [[ "$u" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$ ]]; then t=$(date -u -d "$u" +%s 2>/dev/null) || t=""; fi
    if [ -z "$t" ]; then printf 'refused\t%s\tstale_age (utc=%s)\n' "$f" "${u:-absent}"; rc=1; continue; fi
    if [ "$t" -gt $(( $(lo_now) + SKEW )) ]; then printf 'refused\t%s\tstale_age (future_utc=%s)\n' "$f" "$u"; rc=1; continue; fi
    if [ $(( $(lo_now) - t )) -gt "$maxage" ]; then printf 'refused\t%s\tstale_age (utc=%s)\n' "$f" "${u:-absent}"; rc=1; continue; fi
  fi
  printf 'ok\t%s\n' "$f"
done
exit $rc
