#!/usr/bin/env bash
# =============================================================================
# scripts/ledger/tests/test_project_gate_ledger.sh
# Identity: catalogizer / feature 001-full-project-audit-remediation / WP-08 T086
# Revision: 2 | Created: 2026-10-05 | Last modified: 2026-10-05 | Status: draft, UNREVIEWED (review: T094)
#   rev 2 = WF2 review fix round 3: fixtures for I-5 (a) carrier site, (b) baseline parse, (c) committed prev names,
#   (d) ratchet down, (e) hyphenated prose, (f) tracked-item rule; and for the survivors LM1-LM3 (empty removal
#   reason, trailing-dash ledger row, digit-bearing names). The assertion "count below baseline passes" of rev 1
#   is REPLACED by "count below baseline FAILs, lowered baseline passes": a deliberate change of the rule (I-5 d),
#   not a weakening - the old rule let freed slack be reused.
# NOTE: task text names scripts/gates/tests/; moved under scripts/ledger/ by the
#       caller's scope. RUNP/IMG-KCOV unavailable (T007 not done): host run.
# Executes the ratchet through its real invocation path; asserts exit + stderr.
# Env: RATCHET=<path> (default scripts/ledger/project_gate_ledger_ratchet.sh),
#      used by the paired mutation runner to aim the same test at a mutant.
# =============================================================================
set -u
HERE=$(cd "$(dirname "$0")" && pwd)
RATCHET=${RATCHET:-$HERE/../project_gate_ledger_ratchet.sh}
pass=0; failn=0
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT

mk() { # fresh golden-good fixture in $T/f
  rm -rf "$T/f"; mkdir -p "$T/f/docs" "$T/f/impl"
  printf 'See CM-AAA-ONE and CM-BBB-TWO.\n' >"$T/f/docs/a.md"
  printf '#!/bin/sh\necho CM-AAA-ONE gate\n' >"$T/f/impl/g.sh"   # rev 2: the token must sit on a non-comment line (I-5a)
  printf '# schema\nCM-AAA-ONE\tIMPLEMENTED\timpl/g.sh\nCM-BBB-TWO\tDEFERRED\tITEM-1\n' >"$T/f/ledger.tsv"
  echo 1 >"$T/f/baseline"; printf 'CM-AAA-ONE\nCM-BBB-TWO\n' >"$T/f/prev"; : >"$T/f/removals"
}
EXTRA=""
run() { bash "$RATCHET" $EXTRA --docs "$T/f/docs" --ledger "$T/f/ledger.tsv" --baseline "$T/f/baseline" \
  --prev-names "$T/f/prev" --removals "$T/f/removals" --root "$T/f" >"$T/out" 2>"$T/err"; echo $?; }
expect() { # name want-rc [stderr-pattern]
  local got; got=$(run)
  if [ "$got" = "$2" ] && { [ -z "${3:-}" ] || grep -q -- "$3" "$T/err"; }; then
    pass=$((pass+1)); echo "ok   $1 (rc=$got)"
  else failn=$((failn+1)); echo "FAIL $1 (rc=$got want $2, pat=${3:-})"; sed 's/^/     /' "$T/err" | head -3; fi
}

mk; expect "golden-good passes" 0
mk; printf 'CM-CCC-THREE\n' >>"$T/f/docs/a.md"; expect "name with no row FAILs" 1 "F1 CM-CCC-THREE"
mk; printf 'CM-OLD-GONE\n' >>"$T/f/prev"; expect "vanished name without citation FAILs" 1 "F2 CM-OLD-GONE"
mk; printf 'CM-OLD-GONE\n' >>"$T/f/prev"; printf 'CM-OLD-GONE\trepealed, tracked ITEM-9\n' >"$T/f/removals"
expect "vanished name WITH citation passes (golden-false)" 0
mk; echo 0 >"$T/f/baseline"; expect "unimplemented count above baseline FAILs" 1 "F3"
mk; echo 2 >"$T/f/baseline"; expect "count below baseline FAILs: slack must be lowered (ratchet only moves down, I-5d)" 1 "below baseline 2"
mk; expect "count equal to the baseline passes" 0
mk; : >"$T/f/impl/g.sh"; expect "implementation site without token FAILs" 1 "F1 CM-AAA-ONE"
mk; mv "$T/f/impl/g.sh" "$T/f/impl/g.md"; sed -i 's#impl/g.sh#impl/g.md#' "$T/f/ledger.tsv"
printf '# CM-AAA-ONE\n' >"$T/f/impl/g.md"; expect "markdown carrier as site FAILs" 1 "markdown"
mk; printf 'CM-QA-*\n' >>"$T/f/docs/a.md"; expect "generic family reference only warns (golden-false)" 0 "WARN"
mk; printf 'CM-QA-*\tDEFERRED\tITEM-2\n' >>"$T/f/ledger.tsv"; expect "wildcard ledger row FAILs" 1 "F1b"
mk; printf 'no names here\n' >"$T/f/docs/a.md"; expect "zero names is BLIND not clean" 1 "BLIND"
mk; sed -i 's/ITEM-1//' "$T/f/ledger.tsv"; expect "deferral without tracked item FAILs" 1 "CM-BBB-TWO"
# ---- WF2 fix round 3 -------------------------------------------------------------------------
mk; printf 'CM-AAA-ONE\tIMPLEMENTED\tprev.txt\n' >"$T/f/ledger.tsv.new"; printf 'CM-BBB-TWO\tDEFERRED\tITEM-1\n' >>"$T/f/ledger.tsv.new"; mv "$T/f/ledger.tsv.new" "$T/f/ledger.tsv"; printf 'CM-AAA-ONE\n' >"$T/f/prev.txt"
expect "I-5a a name list as IMPLEMENTED site (carrier) FAILs" 1 "not a code file"
mk; printf '#!/bin/sh\n# CM-AAA-ONE only mentioned in a comment\n' >"$T/f/impl/g.sh"; expect "I-5a a code file that mentions the token only in a comment FAILs" 1 "non-comment line"
mk; cp "$T/f/prev" "$T/f/prevfile.sh"; sed -i 's#impl/g.sh#prevfile.sh#' "$T/f/ledger.tsv"; run() { bash "$RATCHET" $EXTRA --docs "$T/f/docs" --ledger "$T/f/ledger.tsv" --baseline "$T/f/baseline" --prev-names "$T/f/prevfile.sh" --removals "$T/f/removals" --root "$T/f" >"$T/out" 2>"$T/err"; echo $?; }
printf 'CM-AAA-ONE\nCM-BBB-TWO\n' >"$T/f/prevfile.sh"
expect "I-5a the ratchet's own prev-names input as IMPLEMENTED site FAILs" 1 "own input files"
run() { bash "$RATCHET" $EXTRA --docs "$T/f/docs" --ledger "$T/f/ledger.tsv" --baseline "$T/f/baseline" --prev-names "$T/f/prev" --removals "$T/f/removals" --root "$T/f" >"$T/out" 2>"$T/err"; echo $?; }
mk; sed -i 's#impl/g.sh#../g.sh#' "$T/f/ledger.tsv"; expect "I-5a an implementation site path leaving the root FAILs" 1 "leaves the root"
mk; printf 'CM-NEW-ONE\nCM-NEW-TWO\n' >>"$T/f/docs/a.md"; printf 'CM-NEW-ONE\tDEFERRED\tITEM-5\nCM-NEW-TWO\tDEFERRED\tITEM-6\n' >>"$T/f/ledger.tsv"; printf '# baseline rev 2, 2026-10-05\n1\n' >"$T/f/baseline"
expect "I-5b a dated header in the baseline file is not read as part of the number: 3 deferred > 1 FAILs" 1 "exceeds baseline 1"
mk; printf '# baseline rev 2, 2026-10-05\n1\n' >"$T/f/baseline"; expect "I-5b a dated header with the right number passes (golden-false)" 0
mk; printf '1\n2\n' >"$T/f/baseline"; expect "I-5b two integers in the baseline file FAIL (ambiguous)" 1 "ambiguous"
mk; printf 'one\n' >"$T/f/baseline"; expect "I-5b a non-integer baseline FAILs" 1 "unreadable or ambiguous"
mk; printf '' >"$T/f/baseline"; expect "I-5b an empty baseline FAILs" 1 "unreadable or ambiguous"
mk; sed -i 's/ITEM-1/soon/' "$T/f/ledger.tsv"; expect "I-5f a deferral reference that is not a tracked item id FAILs" 1 "not a tracked item id"
mk; sed -i 's/ITEM-1/PENDING-REGISTER-ITEM(after T069)/' "$T/f/ledger.tsv"; expect "I-5f the interim placeholder FAILs without --allow-pending" 1 "needs --allow-pending"
mk; sed -i 's/ITEM-1/PENDING-REGISTER-ITEM(after T069)/' "$T/f/ledger.tsv"; EXTRA="--allow-pending"; expect "I-5f the interim placeholder with --allow-pending passes but is reported PASS-INTERIM, never PASS" 0
grep -q 'PASS-INTERIM .*pending_untracked=1' "$T/out" && { pass=$((pass+1)); echo "ok   the interim result line names PASS-INTERIM and pending_untracked=1"; } || { failn=$((failn+1)); echo "FAIL interim result line: $(cat "$T/out")"; }
mk; EXTRA="--allow-pending"; expect "I-5f with --allow-pending and a real tracked id the result is a plain PASS (golden-false)" 0
grep -qE 'ratchet PASS names|RATCHET PASS names' "$T/out" && ! grep -q INTERIM "$T/out" && { pass=$((pass+1)); echo "ok   a ledger with only tracked deferrals reports plain PASS"; } || { failn=$((failn+1)); echo "FAIL plain PASS line: $(cat "$T/out")"; }
EXTRA=""
mk; printf 'Use a CM-BRAND-NEW-GATE-style check here.\n' >>"$T/f/docs/a.md"; expect "I-5e a name followed by hyphenated prose is a NAME with no row: FAILs (was a silent warn)" 1 "F1 CM-BRAND-NEW-GATE has neither"
mk; printf 'Use a CM-BRAND-NEW-GATE check here.\n' >>"$T/f/docs/a.md"; expect "I-5e the same name without the prose suffix FAILs identically (control)" 1 "F1 CM-BRAND-NEW-GATE has neither"
mk; printf 'See CM-QA2-GATE9 now.\n' >>"$T/f/docs/a.md"; expect "LM3 a digit-bearing name with no row FAILs by its full name" 1 "F1 CM-QA2-GATE9 has neither"
mk; printf 'See CM-QA2-GATE9 now.\n' >>"$T/f/docs/a.md"; printf 'CM-QA2-GATE9\tDEFERRED\tITEM-8\n' >>"$T/f/ledger.tsv"; echo 2 >"$T/f/baseline"; printf 'CM-QA2-GATE9\n' >>"$T/f/prev"; expect "LM3 a digit-bearing name with a row and a prev-names entry passes (golden-false; rev 3: a new name must be registered in prev-names, F2b)" 0
mk; printf 'CM-OLD-GONE\n' >>"$T/f/prev"; printf 'CM-OLD-GONE\t\n' >"$T/f/removals"; expect "LM1 a removal row with an empty reason FAILs" 1 "F2 CM-OLD-GONE"
mk; printf 'CM-OLD-GONE\n' >>"$T/f/prev"; printf 'CM-OLD-GONE\tx\n' >"$T/f/removals"; expect "LM1 a removal row with a one-character reason FAILs" 1 "F2 CM-OLD-GONE"
mk; printf 'CM-OLD-GONE\n' >>"$T/f/prev"; printf 'CM-OLD-GONE\trepealed in ITEM-9\n' >"$T/f/removals"; expect "LM1 a removal row with a real reason passes (golden-false)" 0
mk; printf 'CM-DASH-\tDEFERRED\tITEM-3\n' >>"$T/f/ledger.tsv"; expect "LM2 a ledger row whose name ends in a dash FAILs" 1 "F1b"
mk; printf 'CM-AAA-ONE\tDEFERRED\tITEM-3\n' >>"$T/f/ledger.tsv"; echo 2 >"$T/f/baseline"; expect "I-5 a duplicate ledger name FAILs" 1 "F1c"
mk; EXTRA=""; out=$(bash "$RATCHET" --docs 2>&1); [ $? -eq 2 ] && { pass=$((pass+1)); echo "ok   an option without its value exits 2"; } || { failn=$((failn+1)); echo "FAIL option without value"; }

# ---- WF3 round 4 (I3 routes 1-3, minors W6 W7) -----------------------------------------------------
# I3 route 1: a name that is in the documents but was never listed in prev-names could later vanish with no citation
mk; printf 'CM-NEW-GATE\n' >>"$T/f/docs/a.md"; printf 'CM-NEW-GATE\tDEFERRED\tITEM-5\n' >>"$T/f/ledger.tsv"; echo 2 >"$T/f/baseline"
expect "I3-1 a document name that is not registered in prev-names FAILs (F2b: it could later vanish uncited)" 1 "F2b CM-NEW-GATE"
mk; printf 'CM-NEW-GATE\n' >>"$T/f/docs/a.md"; printf 'CM-NEW-GATE\tDEFERRED\tITEM-5\n' >>"$T/f/ledger.tsv"; echo 2 >"$T/f/baseline"; printf 'CM-NEW-GATE\n' >>"$T/f/prev"
expect "I3-1b the same name registered in prev-names passes (golden-false)" 0
mk; printf 'See CM-QA-* and CM-ZZ-.\n' >>"$T/f/docs/a.md"; expect "I3-1c a generic family reference needs no prev-names entry (golden-false)" 0 "WARN"
# I3 route 1 end to end in a real git repo: the whole add-then-remove sequence of the reviewer's A1-A3
G2="$T/g2"; rm -rf "$G2"; mkdir -p "$G2/docs"; ( cd "$G2" && git init -q . && git config user.email t@t && git config user.name t )
printf 'CM-AAA-ONE CM-NEW-GATE\n' >"$G2/docs/a.md"; printf '#!/bin/sh\necho CM-AAA-ONE\n' >"$G2/g.sh"
printf 'CM-AAA-ONE\tIMPLEMENTED\tg.sh\nCM-NEW-GATE\tDEFERRED\tITEM-4\n' >"$G2/ledger.tsv"; echo 1 >"$G2/baseline"; printf 'CM-AAA-ONE\nCM-NEW-GATE\n' >"$G2/prev"; : >"$G2/removals"
( cd "$G2" && git add -A && git commit -q -m init )
g2r() { bash "$RATCHET" --docs "$G2/docs" --ledger "$G2/ledger.tsv" --baseline "$G2/baseline" --prev-names "$G2/prev" --removals "$G2/removals" --root "$G2" >"$T/out" 2>"$T/err"; echo $?; }
got=$(g2r); [ "$got" = 0 ] && { pass=$((pass+1)); echo "ok   I3-1d control: the registered name passes in a committed repo"; } || { failn=$((failn+1)); echo "FAIL I3-1d control rc=$got [$(cat "$T/err")]"; }
printf 'CM-AAA-ONE\n' >"$G2/docs/a.md"; printf 'CM-AAA-ONE\tIMPLEMENTED\tg.sh\n' >"$G2/ledger.tsv"; echo 0 >"$G2/baseline"; printf 'CM-AAA-ONE\n' >"$G2/prev"
got=$(g2r); [ "$got" = 1 ] && grep -q 'F2 CM-NEW-GATE' "$T/err" && { pass=$((pass+1)); echo "ok   I3-1e removing a registered name from docs, ledger, baseline and the working prev-names without citation FAILs (HEAD prev-names)"; } || { failn=$((failn+1)); echo "FAIL I3-1e rc=$got [$(cat "$T/err")]"; }
# I3 route 2: a prev-names file that does not exist must refuse, not silently disable F2
mk; run() { bash "$RATCHET" $EXTRA --docs "$T/f/docs" --ledger "$T/f/ledger.tsv" --baseline "$T/f/baseline" --prev-names "$T/f/no_such_prev" --removals "$T/f/removals" --root "$T/f" >"$T/out" 2>"$T/err"; echo $?; }
expect "I3-2 a missing --prev-names file FAILs (an unresolvable input is a refusal, never a pass)" 1 "prev-names file absent"
run() { bash "$RATCHET" $EXTRA --docs "$T/f/docs" --ledger "$T/f/ledger.tsv" --baseline "$T/f/baseline" --prev-names "$T/f/prev" --removals "$T/f/removals" --root "$T/f" >"$T/out" 2>"$T/err"; echo $?; }
# I3 route 3: an orphan ledger row (its name is in no document) keeps slack and hides an unvalidated reference
mk; printf 'CM-ORPHAN-PAD\tDEFERRED\tjunk\n' >>"$T/f/ledger.tsv"; echo 2 >"$T/f/baseline"
expect "I3-3 an orphan ledger row (name in no document) FAILs (F1d)" 1 "F1d CM-ORPHAN-PAD"
mk; sed -i 's/CM-BBB-TWO\tDEFERRED\tITEM-1/CM-BBB-TWO\tIMPLEMENTED\timpl\/h.sh/' "$T/f/ledger.tsv"; printf '#!/bin/sh\necho CM-BBB-TWO\n' >"$T/f/impl/h.sh"; echo 0 >"$T/f/baseline"; printf 'CM-ORPHAN-PAD\tDEFERRED\tITEM-77\n' >>"$T/f/ledger.tsv"
expect "I3-3b after a gate is implemented an orphan DEFERRED row cannot keep the freed slack: still FAILs" 1 "F1d CM-ORPHAN-PAD"
mk; sed -i 's/CM-BBB-TWO\tDEFERRED\tITEM-1/CM-BBB-TWO\tIMPLEMENTED\timpl\/h.sh/' "$T/f/ledger.tsv"; printf '#!/bin/sh\necho CM-BBB-TWO\n' >"$T/f/impl/h.sh"; echo 0 >"$T/f/baseline"
expect "I3-3c control: the same implemented ledger with a baseline lowered to 0 and no orphan passes (golden-false)" 0
# W6: the tracked-item id is anchored at both ends
mk; sed -i 's/ITEM-1/CAT-12-later/' "$T/f/ledger.tsv"; expect "W6 a deferral reference with a suffix (CAT-12-later) FAILs: the item-id pattern is anchored at the end" 1 "not a tracked item id"
# W7: a // comment is a comment
mk; printf 'package g\n// CM-AAA-ONE only mentioned in a Go line comment\n' >"$T/f/impl/g.go"; sed -i 's#impl/g.sh#impl/g.go#' "$T/f/ledger.tsv"
expect "W7 a .go site whose token sits only in a // comment FAILs" 1 "non-comment line"
mk; printf 'package g\nvar n = "CM-AAA-ONE"\n' >"$T/f/impl/g.go"; sed -i 's#impl/g.sh#impl/g.go#' "$T/f/ledger.tsv"
expect "W7b a .go site carrying the token in code passes (golden-false)" 0

# I-5c: the previous names are the committed file UNION the working file (rename/delete gaming)
G="$T/g"; rm -rf "$G"; mkdir -p "$G/docs"; ( cd "$G" && git init -q . && git config user.email t@t && git config user.name t )
printf 'CM-AAA-ONE CM-GONE-NAME\n' >"$G/docs/a.md"; printf '#!/bin/sh\n# x\necho CM-AAA-ONE\n' >"$G/g.sh"
printf 'CM-AAA-ONE\tIMPLEMENTED\tg.sh\nCM-GONE-NAME\tDEFERRED\tITEM-4\n' >"$G/ledger.tsv"; echo 1 >"$G/baseline"; printf 'CM-AAA-ONE\nCM-GONE-NAME\n' >"$G/prev"; : >"$G/removals"
( cd "$G" && git add -A && git commit -q -m init )
gr() { bash "$RATCHET" --docs "$G/docs" --ledger "$G/ledger.tsv" --baseline "$G/baseline" --prev-names "$G/prev" --removals "$G/removals" --root "$G" >"$T/out" 2>"$T/err"; echo $?; }
got=$(gr); [ "$got" = 0 ] && grep -q 'prev_source=working+HEAD' "$T/out" && { pass=$((pass+1)); echo "ok   I-5c control: committed repo, nothing removed, passes and names prev_source=working+HEAD"; } || { failn=$((failn+1)); echo "FAIL I-5c control rc=$got [$(cat "$T/out") $(cat "$T/err")]"; }
printf 'CM-AAA-ONE\n' >"$G/docs/a.md"; printf 'CM-AAA-ONE\tIMPLEMENTED\tg.sh\n' >"$G/ledger.tsv"; echo 0 >"$G/baseline"; printf 'CM-AAA-ONE\n' >"$G/prev"
got=$(gr); [ "$got" = 1 ] && grep -q 'F2 CM-GONE-NAME' "$T/err" && { pass=$((pass+1)); echo "ok   I-5c a name removed from docs, ledger AND the working prev-names is still caught against the committed prev-names"; } || { failn=$((failn+1)); echo "FAIL I-5c gaming rc=$got [$(cat "$T/err")]"; }
printf 'CM-GONE-NAME\tretired, ITEM-9 closed it\n' >"$G/removals"
got=$(gr); [ "$got" = 0 ] && { pass=$((pass+1)); echo "ok   I-5c with a removal citation the same change passes (golden-false)"; } || { failn=$((failn+1)); echo "FAIL I-5c cited removal rc=$got [$(cat "$T/err")]"; }
U="$T/u"; rm -rf "$U"; mkdir -p "$U/docs"; printf 'CM-AAA-ONE\n' >"$U/docs/a.md"; printf '#!/bin/sh\necho CM-AAA-ONE\n' >"$U/g.sh"; printf 'CM-AAA-ONE\tIMPLEMENTED\tg.sh\n' >"$U/ledger.tsv"; echo 0 >"$U/baseline"; printf 'CM-AAA-ONE\n' >"$U/prev"; : >"$U/removals"
bash "$RATCHET" --docs "$U/docs" --ledger "$U/ledger.tsv" --baseline "$U/baseline" --prev-names "$U/prev" --removals "$U/removals" --root "$U" >"$T/out" 2>"$T/err"
grep -q 'prev_source=working-only' "$T/out" && { pass=$((pass+1)); echo "ok   I-5c an untracked prev-names file is reported prev_source=working-only (the guard is honestly inactive)"; } || { failn=$((failn+1)); echo "FAIL I-5c untracked: $(cat "$T/out")"; }
# WF6 (OWED-WP06-8): prefix-collision fixtures. A name that is only a PREFIX of another name must not satisfy the
# exact-name membership guards F1d and F2b (grep -qxF weakened to grep -qF would accept it).
# F1d: the orphan row CM-ALPHA while the documents name only CM-ALPHA-ONE
mk; printf 'CM-ALPHA-ONE\n' >>"$T/f/docs/a.md"; printf 'CM-ALPHA-ONE\tDEFERRED\tCAT-6\nCM-ALPHA\tDEFERRED\tCAT-5\n' >>"$T/f/ledger.tsv"; echo 3 >"$T/f/baseline"; printf 'CM-ALPHA-ONE\nCM-ALPHA\n' >>"$T/f/prev"
expect "WF6-R1 an orphan row CM-ALPHA whose name is only a prefix of the document name CM-ALPHA-ONE FAILs (F1d is exact-name)" 1 "F1d CM-ALPHA ledger row"
# F2b: the document name CM-ALPHA while prev-names holds only CM-ALPHA-ONE
mk; printf 'CM-ALPHA-ONE\nCM-ALPHA\n' >>"$T/f/docs/a.md"; printf 'CM-ALPHA-ONE\tDEFERRED\tCAT-6\nCM-ALPHA\tDEFERRED\tCAT-5\n' >>"$T/f/ledger.tsv"; echo 3 >"$T/f/baseline"; printf 'CM-ALPHA-ONE\n' >>"$T/f/prev"
expect "WF6-R2 a document name CM-ALPHA registered only as a prefix of CM-ALPHA-ONE in prev-names FAILs (F2b is exact-name)" 1 "F2b CM-ALPHA is in the documents"
# golden-false: both names exactly present and registered passes
mk; printf 'CM-ALPHA-ONE\nCM-ALPHA\n' >>"$T/f/docs/a.md"; printf 'CM-ALPHA-ONE\tDEFERRED\tCAT-6\nCM-ALPHA\tDEFERRED\tCAT-5\n' >>"$T/f/ledger.tsv"; echo 3 >"$T/f/baseline"; printf 'CM-ALPHA-ONE\nCM-ALPHA\n' >>"$T/f/prev"
expect "WF6-R3 golden-false: both CM-ALPHA and CM-ALPHA-ONE exactly present and registered passes" 0
# WF7-1: IMPLEMENTED site match is a whole-token match (boundaries on identifier characters), never a substring
wf7() { # $1 = content of impl/one.sh body line; the document names CM-ALPHA, ledger says IMPLEMENTED in impl/one.sh
  mk; printf 'CM-ALPHA\n' >>"$T/f/docs/a.md"; printf '#!/bin/sh\n%s\n' "$1" >"$T/f/impl/one.sh"
  printf 'CM-ALPHA\tIMPLEMENTED\timpl/one.sh\n' >>"$T/f/ledger.tsv"; printf 'CM-ALPHA\n' >>"$T/f/prev"; }
wf7 'echo CM-ALPHA-ONE'
expect "WF7-P1 a site that carries only the longer token CM-ALPHA-ONE does not implement CM-ALPHA (prefix collision)" 1 "F1 CM-ALPHA implementation site"
wf7 'echo CM-ALPHAX_HELPER'
expect "WF7-P1b a site that carries only the unrelated identifier CM-ALPHAX_HELPER does not implement CM-ALPHA (embedded suffix)" 1 "F1 CM-ALPHA implementation site"
wf7 'echo XCM-ALPHA'
expect "WF7-P1c a site that carries only XCM-ALPHA does not implement CM-ALPHA (embedded prefix)" 1 "F1 CM-ALPHA implementation site"
wf7 'echo CM-ALPHA_2'
expect "WF7-P1d a site that carries only CM-ALPHA_2 does not implement CM-ALPHA (underscore continues the identifier)" 1 "F1 CM-ALPHA implementation site"
wf7 'echo CM-ALPHA-ONE; echo CM-ALPHAX'
expect "WF7-P1e a site holding several look-alikes and no exact token still FAILs" 1 "F1 CM-ALPHA implementation site"
for body in 'echo CM-ALPHA' 'echo "CM-ALPHA"' "case \$1 in 'CM-ALPHA') ;; esac" 'run(CM-ALPHA);' 'x=CM-ALPHA,y' 'CM-ALPHA'; do
  wf7 "$body"; echo 1 >"$T/f/baseline"; : # one DEFERRED row from mk (CM-BBB-TWO)
  expect "WF7-P1g golden-false: a real implementation spelling the exact token ($body) passes" 0
done
# WF7-3 (LR1): F2 presence is exact-name, not substring
mk; printf 'CM-ALPHA-ONE\n' >>"$T/f/docs/a.md"; printf 'CM-ALPHA-ONE\tDEFERRED\tCAT-6\n' >>"$T/f/ledger.tsv"; echo 2 >"$T/f/baseline"; printf 'CM-ALPHA-ONE\nCM-ALPHA\n' >>"$T/f/prev"
expect "WF7-LR1 a prev name CM-ALPHA that is only a prefix of the document name CM-ALPHA-ONE vanished uncited FAILs (F2 is exact-name)" 1 "F2 CM-ALPHA vanished"
# WF7-3 (LR2): F1d is case-sensitive: an orphan row that differs only by case keeps freed slack
mk; printf 'cm-bbb-two\tDEFERRED\tCAT-7\n' >>"$T/f/ledger.tsv"; echo 2 >"$T/f/baseline"
expect "WF7-LR2 an orphan row cm-bbb-two differing only by case from the document name CM-BBB-TWO FAILs (F1d is case-sensitive)" 1 "F1d cm-bbb-two ledger row"
# WF7-3 (LR3): F2b is case-sensitive: a document name registered only in lower case is not registered
mk; printf 'CM-NEW-GATE\n' >>"$T/f/docs/a.md"; printf 'CM-NEW-GATE\tDEFERRED\tCAT-8\n' >>"$T/f/ledger.tsv"; echo 2 >"$T/f/baseline"; printf 'cm-new-gate\n' >>"$T/f/prev"
expect "WF7-LR3 a document name CM-NEW-GATE registered in prev-names only as cm-new-gate FAILs (F2b is case-sensitive)" 1 "F2b CM-NEW-GATE is in the documents"
mk; # control needle: instrument sees a planted name through the same path
printf 'CM-NEEDLE-PLANT\n' >>"$T/f/docs/a.md"; run >/dev/null
if grep -q "CM-NEEDLE-PLANT" "$T/err"; then pass=$((pass+1)); echo "ok   control needle seen"; else failn=$((failn+1)); echo "FAIL control needle not seen"; fi

echo "RESULT pass=$pass fail=$failn"
[ "$failn" -eq 0 ]
