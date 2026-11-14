#!/usr/bin/env bash
# Tests of the permanent-operator-accepted risk class (kind permanent-risk) of check_release_consistency.sh.
# Unit-class evidence: fixtures are input data for the checker (11.4.289(C)(3)). Also run by
# test_check_release_consistency.sh. CHECKER=<path> selects the checker under test.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHECKER="${CHECKER:-$HERE/check_release_consistency.sh}"
# shellcheck source=scripts/release/release_fixture.sh
. "$HERE/release_fixture.sh"
BASE="${RELCHK_TMP:-${XDG_CACHE_HOME:-${HOME:?}/.cache}/relchk-tmp}"
mkdir -p "$BASE" || exit 2
ROOT="$(mktemp -d "$BASE/prtest.XXXXXX")" || exit 2
case "$ROOT" in "$BASE"/prtest.*) ;; *) echo "refusing odd ROOT $ROOT" >&2; exit 2 ;; esac
trap 'rm -rf "${ROOT:?}"' EXIT
PASS=0; FAIL=0
ok()  { PASS=$((PASS+1)); }
bad() { FAIL=$((FAIL+1)); echo "  FAIL: $1"; }
GOLD="$ROOT/gold"; make_fixture "$GOLD" || { echo "fixture build failed" >&2; exit 2; }
fresh() { local t; t="$(mktemp -d "$ROOT/c.XXXXXX")" && cp -a "$GOLD/." "$t/" && echo "$t"; }
wv() { mkdir -p "$1/docs/qa"; printf '%s\n' "$2" > "$1/docs/qa/EVIDENCE_WAIVERS.txt"; (cd "$1" && git add -A && git commit -q -m waivers); }
# expect <label> <rc> <needle> <dir> [args]: checker on the committed state; asserts rc, needle and a VERDICT line
expect() {
  local label="$1" rc="$2" needle="$3" dir="$4"; shift 4
  local out got
  out="$("$CHECKER" --repo "$dir" "$@" 2>&1)"; got=$?
  if [ "$got" != "$rc" ]; then bad "$label: rc=$got want $rc"; return; fi
  if [ -n "$needle" ] && ! printf '%s\n' "$out" | grep -qF -- "$needle"; then bad "$label: output lacks '$needle'"; return; fi
  printf '%s\n' "$out" | grep -q '^VERDICT: ' || { bad "$label: no VERDICT line"; return; }
  ok
}
PR='permanent-risk|keyrot|CAT-KEYROT-WAIVER-1|OPDEC-2026-11-14-1|2026-11-14|operator re-affirms this accepted risk each release review|key rotation is deferred by operator decision'
echo "== permanent-risk class"
d=$(fresh); wv "$d" "$PR"; expect "PR1 permanent risk is accepted and listed visibly" 0 "PERMANENT-RISK  keyrot: key rotation is deferred by operator decision [item CAT-KEYROT-WAIVER-1, operator decision OPDEC-2026-11-14-1 of 2026-11-14, no expiry; re-affirmation: operator re-affirms this accepted risk each release review]" "$d" --check changelog
d=$(fresh); wv "$d" "$PR"; expect "PR2 permanent risk is counted in the summary" 0 "permanent_risks=1" "$d" --check changelog
d=$(fresh); wv "$d" "$PR"; expect "PR3 permanent risk is no evidence gap: verdict word stays PASS" 0 "VERDICT: PASS section" "$d" --check changelog
d=$(fresh); wv "$d" "$PR"; RELCHK_TODAY=2099-01-01 expect "PR4 permanent risk never expires (today 2099)" 0 "PERMANENT-RISK  keyrot" "$d" --check changelog
d=$(fresh); wv "$d" 'permanent-risk|keyrot|CAT-KEYROT-WAIVER-1||2026-11-14|note|reason'; expect "PR5 permanent risk without decision id -> BADWAIVER" 1 "operator decision id" "$d" --check changelog
d=$(fresh); wv "$d" 'permanent-risk|keyrot|CAT-KEYROT-WAIVER-1|not an id|2026-11-14|note|reason'; expect "PR5b permanent risk with malformed decision id -> BADWAIVER" 1 "operator decision id" "$d" --check changelog
d=$(fresh); wv "$d" 'permanent-risk|keyrot|CAT-KEYROT-WAIVER-1|OPDEC-1||note|reason'; expect "PR6 permanent risk without decision date -> BADWAIVER" 1 "decision date" "$d" --check changelog
d=$(fresh); wv "$d" 'permanent-risk|keyrot|CAT-KEYROT-WAIVER-1|OPDEC-1|2099-12-31|note|reason'; expect "PR7 permanent risk with future decision date -> BADWAIVER" 1 "decision date" "$d" --check changelog
d=$(fresh); wv "$d" 'permanent-risk|keyrot|CAT-KEYROT-WAIVER-1|OPDEC-1|2026-11-14||reason'; expect "PR8 permanent risk without re-affirmation note -> BADWAIVER" 1 "re-affirmation" "$d" --check changelog
d=$(fresh); wv "$d" 'permanent-risk|keyrot|CAT-KEYROT-WAIVER-1|OPDEC-1|2026-11-14|note|  '; expect "PR9 permanent risk without reason -> BADWAIVER" 1 "missing reason" "$d" --check changelog
d=$(fresh); wv "$d" 'permanent-risk|keyrot||OPDEC-1|2026-11-14|note|reason'; expect "PR10 permanent risk without tracked item -> BADWAIVER" 1 "tracked item id" "$d" --check changelog
d=$(fresh); wv "$d" "$PR
$PR"; expect "PR11 duplicate permanent risk -> BADWAIVER" 1 "duplicate" "$d" --check changelog
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; wv "$d" "$PR
unlisted|docs/qa/run2|extra.txt|CAT-T-1|2020-01-01|r"; expect "PR12 a permanent risk does not rescue an expired ordinary waiver" 1 "expired on 2020-01-01" "$d" --check changelog
d=$(fresh); echo extra > "$d/docs/qa/run2/extra.txt"; wv "$d" "$PR"; expect "PR13 a permanent risk does not waive an evidence gap" 1 "UNLISTED  docs/qa/run2/extra.txt" "$d" --check changelog
d=$(fresh); wv "$d" 'unlisted|docs/qa/run2|nothing.txt|CAT-T-1|2020-01-01|r'; expect "PR14 an expired ordinary waiver alone still fails" 1 "expired on 2020-01-01" "$d" --check changelog
d=$(fresh); wv "$d" "# comment
$PR"; expect "PR15 permanent risk with leading comment line is read" 0 "permanent_risks=1" "$d" --check changelog
# M-1: a register item marked PERMANENT must have a permanent-risk record
REG='| CAT-KEYROT-WAIVER-1 | Task | Open risk, operator-accepted (PERMANENT) | text | next | owner |'
regcommit() { mkdir -p "$1/docs"; printf '%s\n' "$2" > "$1/docs/WORKABLE_ITEMS_REGISTER.md"; (cd "$1" && git add -A && git commit -q -m reg); }
d=$(fresh); regcommit "$d" "$REG"; expect "PR16 register item marked PERMANENT without any record -> FAIL" 1 "marked PERMANENT in docs/WORKABLE_ITEMS_REGISTER.md" "$d" --check changelog
d=$(fresh); regcommit "$d" "$REG"; wv "$d" "# $PR"; expect "PR17 commented-out record does not satisfy the register -> FAIL" 1 "CAT-KEYROT-WAIVER-1" "$d" --check changelog
d=$(fresh); regcommit "$d" "$REG"; wv "$d" "$PR"; expect "PR18 register PERMANENT item with its record passes" 0 "permanent_risks=1" "$d" --check changelog
d=$(fresh); regcommit "$d" "$REG"; wv "$d" 'permanent-risk|other|CAT-OTHER-1|OPDEC-1|2026-11-14|note|reason'; expect "PR19 record for another item does not satisfy the register -> FAIL" 1 "marked PERMANENT" "$d" --check changelog
d=$(fresh); regcommit "$d" '| CAT-X-1 | Task | Open risk | not permanent | n | o |'; expect "PR20 register row without the PERMANENT mark is not required to have a record" 0 "VERDICT: PASS" "$d" --check changelog
# M-4: reviewer mutants
d=$(fresh); wv "$d" "${PR/permanent-risk|/permanent-riskX|}"; expect "PR21 kind prefix permanent-riskX is an unknown kind (R1)" 1 "unknown kind 'permanent-riskX'" "$d" --check changelog
d=$(fresh); wv "$d" 'permanent-risk|keyrot|CAT-KEYROT-WAIVER-1|OPDEC-1|2026-02-30|note|reason'; expect "PR22 impossible decision date 2026-02-30 -> BADWAIVER (R2)" 1 "decision date" "$d" --check changelog
d=$(fresh); wv "$d" 'permanent-risk|keyrot|CAT-KEYROT-WAIVER-1|OPDEC-1|2026-11-14|note|part one|part two'; expect "PR23 reason containing a pipe is kept whole (R3)" 0 "part one|part two [item" "$d" --check changelog
d=$(fresh); wv "$d" "$PR"; so="$("$CHECKER" --repo "$d" --check changelog 2>/dev/null)"; if printf '%s\n' "$so" | grep -q '^PERMANENT-RISK  keyrot'; then ok; else bad "PR24 PERMANENT-RISK line must be on stdout (R4)"; fi
echo "== result: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
