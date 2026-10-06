#!/usr/bin/env bash
# =============================================================================
# scripts/ledger/project_gate_ledger_ratchet.sh
# Identity: catalogizer / feature 001-full-project-audit-remediation / WP-08 T087
# Revision: 3 | Created: 2026-10-05 | Last modified: 2026-10-06 | Status: draft, UNREVIEWED (review: T094)
#   rev 3 = WF3 review fix round 4 (I3): F1d orphan ledger rows refuse, F2b every document name must be registered in
#   prev-names (a name never listed could vanish uncited), an absent --prev-names file refuses (was a silent pass).
#   rev 2 = WF2 review fix round 3 (I-5 a-f, I-6): executable-site rule, strict baseline parse, committed
#   prev-names union, ratchet-down, tracked-item rule with an explicit interim class, hyphenated-prose
#   extraction, duplicate and empty-reason refusals.
# NOTE: task text names scripts/gates/project_gate_ledger_ratchet.sh; this copy
#       lives under scripts/ledger/ by the caller's scope. Relocation = owed (tasks.md amendment owed, see
#       specs/001-full-project-audit-remediation/evidence/wp08/README.md).
# Purpose: project-level named-gate ledger ratchet (constitution 11.4.227(A)).
#   F1 every CM-* name in the feature documents has a ledger row: IMPLEMENTED
#      (a code file under --root, never data or markdown, carrying the token on a non-comment line) or DEFERRED
#      (row with a tracked item id). A wildcard or truncated ledger row is refused (F1b), a duplicate ledger
#      name is refused (F1c); a bare family reference such as CM-* in a document only warns.
#   F2 a name present in --prev-names (the working file UNION the version committed at HEAD, so a name cannot
#      leave by editing the working file) but absent from the documents needs a row in --removals
#      (name<TAB>reason of >= 8 characters), else FAIL.
#   F1d a ledger row whose name occurs in no document FAILs (an orphan row would keep freed baseline slack and its
#      reference would never be validated).
#   F2b every non-family name found in the documents must appear in --prev-names (the working file UNION the
#      HEAD version), so a name cannot enter and later vanish without ever having been registered; an absent
#      --prev-names file is a FAIL (an unresolvable input never reads as PASS).
#   F3 the DEFERRED count must EQUAL the single integer in --baseline: above it FAILs, below it FAILs too
#      (lower the baseline in the same change: a ratchet only moves down and slack must not be reusable).
# Usage: project_gate_ledger_ratchet.sh --docs DIR[,DIR] --ledger FILE
#        --baseline FILE --prev-names FILE --removals FILE [--root DIR] [--allow-pending]
#   --allow-pending  accepts the interim deferral reference PENDING-REGISTER-ITEM(...) (the register is not
#                    live, T069, so no real item id exists yet). The result line then reads PASS-INTERIM and
#                    names the pending count: an interim deferral is never reported as a plain PASS (I-6).
# Ledger schema (TAB): name  IMPLEMENTED|DEFERRED  site-path|tracked-item-id
# Exit: 0 PASS / PASS-INTERIM, 1 FAIL (reasons on stderr), 2 usage. Read-only, no network.
# =============================================================================
set -u
export LC_ALL=C
docs=""; ledger=""; baseline=""; prev=""; removals=""; root="."; allow_pending=0
while [ $# -gt 0 ]; do
  case "$1" in
    --allow-pending) allow_pending=1; shift;;
    --docs|--ledger|--baseline|--prev-names|--removals|--root)
      [ $# -ge 2 ] || { echo "usage_error: $1 needs a value" >&2; exit 2; }
      case "$1" in --docs) docs=$2;; --ledger) ledger=$2;; --baseline) baseline=$2;; --prev-names) prev=$2;; --removals) removals=$2;; --root) root=$2;; esac
      shift 2;;
    *) echo "usage_error: $1" >&2; exit 2;;
  esac
done
for v in docs ledger baseline prev removals; do
  [ -n "${!v}" ] || { echo "usage_error: --$v required" >&2; exit 2; }
done
fail=0
say() { echo "FAIL: $*" >&2; fail=1; }
tmp=$(mktemp -d) || exit 2; trap 'rm -rf "$tmp"' EXIT

# names in the documents. A token ending in '-' or holding '*' is a family reference (warn only); a token that
# ends in '-<lowercase letter>' is a name followed by prose ("CM-FOO-style"): the hyphen and the letter go.
IFS=, read -r -a dirs <<<"$docs"
grep -rhoE 'CM-[A-Z0-9*]+(-[A-Z0-9*]+)*(-[a-z]|-)?' --include='*.md' "${dirs[@]}" 2>/dev/null \
  | sed -E 's/-[a-z]$//' | sort -u >"$tmp/names"
[ -s "$tmp/names" ] || { say "zero names extracted from documents (BLIND, not clean)"; exit 1; }
awk -F'\t' '!/^#/ && NF>=3' "$ledger" >"$tmp/rows"

# every file that feeds the ratchet is data, never an implementation site
for f in "$ledger" "$baseline" "$prev" "$removals"; do realpath -m "$f"; done >"$tmp/inputs"

# F1
pending=0
while read -r n; do
  case "$n" in *'*'*|*-) echo "WARN: generic family reference in documents, not a gate name: $n" >&2; continue;; esac
  row=$(awk -F'\t' -v n="$n" '$1==n{print; exit}' "$tmp/rows")
  if [ -z "$row" ]; then say "F1 $n has neither implementation nor deferral row"; continue; fi
  kind=$(printf '%s' "$row" | cut -f2); ref=$(printf '%s' "$row" | cut -f3)
  case "$kind" in
    IMPLEMENTED)
      f="$root/$ref"
      case "$ref" in *.md) say "F1 $n implementation site is markdown (carrier, not a gate)"; continue;; esac
      case "$ref" in *..*|'') say "F1 $n implementation site path is empty or leaves the root: $ref"; continue;; esac
      case "$ref" in *.sh|*.bash|*.py|*.go|*.js|*.ts|*.rb|*.pl) ;; *) say "F1 $n implementation site $ref is not a code file (data and name lists are carriers, not gates)"; continue;; esac
      if grep -qxF -- "$(realpath -m "$f")" "$tmp/inputs"; then say "F1 $n implementation site $ref is one of the ratchet's own input files"; continue; fi
      if [ ! -f "$f" ] || ! grep -Ev '^[[:space:]]*(#|//|--)' "$f" | grep -qF -- "$n"; then say "F1 $n implementation site $ref missing or lacks the token on a non-comment line"; fi;;
    DEFERRED)
      if [[ $ref =~ ^[A-Z][A-Z0-9]*-[0-9]+$ ]]; then :
      elif [[ $ref == PENDING-REGISTER-ITEM\(* ]]; then
        if [ "$allow_pending" = 1 ]; then pending=$((pending+1)); else say "F1 $n deferral references no tracked item (interim placeholder $ref needs --allow-pending)"; fi
      else say "F1 $n deferral reference '$ref' is not a tracked item id"; fi;;
    *) say "F1 $n unknown ledger kind '$kind'";;
  esac
done <"$tmp/names"

# F1b / F1c: a ledger row must spell the gate out, never a wildcard, and appear once
while IFS=$'\t' read -r n _; do
  case "$n" in *'*'*|*-) say "F1b wildcard or truncated ledger name: $n";; esac
done <"$tmp/rows"
dups=$(cut -f1 "$tmp/rows" | sort | uniq -d | head -3 | tr '\n' ' ')
[ -z "$dups" ] || say "F1c duplicate ledger name(s): $dups"

# F1d: every ledger row names a gate that the documents name
while IFS=$'\t' read -r n _; do
  case "$n" in *'*'*|*-) continue;; esac   # already refused by F1b
  grep -qxF -- "$n" "$tmp/names" || say "F1d $n ledger row names no gate found in the documents (an orphan row keeps freed slack and an unvalidated reference)"
done <"$tmp/rows"

# F2: the previous names are the working file UNION the version committed at HEAD
prev_source=working-only
: >"$tmp/prev"
if [ -f "$prev" ]; then
  cp "$prev" "$tmp/prev"
else
  say "F2 prev-names file absent: $prev (an unresolvable input is a refusal, never a pass)"
fi
pdir=$(dirname "$prev"); pbase=$(basename "$prev")
if git -C "$pdir" ls-files --error-unmatch -- "$pbase" >/dev/null 2>&1 && git -C "$pdir" show "HEAD:./$pbase" >"$tmp/prev.head" 2>/dev/null; then
  cat "$tmp/prev.head" >>"$tmp/prev"; prev_source=working+HEAD
fi
if [ -f "$prev" ]; then
  grep -vE '^[[:space:]]*(#|$)' "$tmp/prev" | sort -u >"$tmp/prevall"
  while read -r p; do
    [ -n "$p" ] || continue
    grep -qxF -- "$p" "$tmp/names" && continue
    if ! awk -F'\t' -v n="$p" '!/^#/ && $1==n && length($2)>=8{f=1} END{exit !f}' "$removals" 2>/dev/null; then
      say "F2 $p vanished without a removal citation (a reason of at least 8 characters)"
    fi
  done <"$tmp/prevall"
  # F2b: a name found in the documents but never registered could later vanish without a citation
  while read -r n; do
    case "$n" in *'*'*|*-) continue;; esac
    grep -qxF -- "$n" "$tmp/prevall" || say "F2b $n is in the documents but not registered in --prev-names (add it there, in the same change)"
  done <"$tmp/names"
fi

# F3: strict baseline, equality (ratchet down)
cnt=$(awk -F'\t' '$2=="DEFERRED"' "$tmp/rows" | wc -l)
blines=$(grep -vcE '^[[:space:]]*(#|$)' "$baseline" 2>/dev/null)
base=$(grep -vE '^[[:space:]]*(#|$)' "$baseline" 2>/dev/null | head -1 | tr -d '[:space:]')
if [ "${blines:-0}" != 1 ] || ! [[ ${base:-x} =~ ^[0-9]+$ ]]; then
  say "F3 baseline unreadable or ambiguous (want exactly one non-comment line holding one integer; found ${blines:-0} line(s))"; base=$cnt
fi
[ "$cnt" -le "$base" ] || say "F3 unimplemented count $cnt exceeds baseline $base"
[ "$cnt" -ge "$base" ] || say "F3 unimplemented count $cnt is below baseline $base: lower the baseline to $cnt in the same change (a ratchet only moves down)"

if [ "$fail" -eq 0 ]; then
  verdict=PASS; [ "$pending" -eq 0 ] || verdict=PASS-INTERIM
  echo "CM-PROJECT-GATE-LEDGER-RATCHET $verdict names=$(wc -l <"$tmp/names") deferred=$cnt baseline=$base pending_untracked=$pending prev_source=$prev_source"
fi
exit "$fail"
