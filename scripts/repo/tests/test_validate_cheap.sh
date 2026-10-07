#!/usr/bin/env bash
# T040/T040b test (TDD): scripts/repo/validate_cheap.sh with scripts/repo/validate_checks.tsv - CPA stage S3 (the cheap checks), the
# class table application, the held-table rules (class_table_unreviewed, table_admits_unheld_path, legacy_row_not_dropped) and the
# pending-row exit precedence. Throwaway repositories; every check is check-only (no tracked byte changes).
# Usage: bash scripts/repo/tests/test_validate_cheap.sh   Env: H=<helper>
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
D0="$(pwd)"; H="${H:-scripts/repo/validate_cheap.sh}"; case "$H" in /*) ;; *) H="$D0/$H" ;; esac
. "$D0/scripts/repo/tests/lib_wp04b.sh"
T="$(mktemp -d "${TMPDIR:-/tmp}/vc_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
[ -x "$H" ] || echo "NOTE: $H is absent or not executable (RED state: every case must FAIL)"
# code root: the helper scripts and the registry as a consumer sees them (the commands of the registry are relative to it)
CODE="$T/code"; mkdir -p "$CODE/scripts/repo" "$CODE/scripts/hooks"
cp "$D0"/scripts/repo/*.sh "$D0"/scripts/repo/validate_checks.tsv "$CODE/scripts/repo/" 2>/dev/null
printf '#!/usr/bin/env bash\n[ -e ./LANDMINE ] && { echo landmine; exit 1; }\nexit 0\n' > "$CODE/scripts/detect-landmines.sh"; chmod +x "$CODE/scripts/detect-landmines.sh"
cp "$D0/scripts/hooks/no-false-positive-log.sh" "$CODE/scripts/hooks/"
REG="$CODE/scripts/repo/validate_checks.tsv"
R=""; VERD=specs/ev/reviews/WP.json
fresh() {
  rm -rf "$T/r"; R="$T/r"; mkrepo "$R"; mkdir -p "$R/scripts/repo" "$R/src" "$R/specs/ev/reviews"
  cp "$D0"/scripts/repo/check_classes.tsv "$D0"/scripts/repo/check_exemptions.tsv "$D0"/scripts/repo/fixture_roots.txt "$R/scripts/repo/"
  sed -i "s#^\# review: .*#\# review: $VERD   (admission table)#" "$R/scripts/repo/check_classes.tsv" "$R/scripts/repo/check_exemptions.tsv" "$R/scripts/repo/fixture_roots.txt"
  echo ready > "$R/src/keep.txt"; commit_all "$R" init
}
w() { mkdir -p "$(dirname "$R/$1")"; printf '%b' "$2" > "$R/$1"; }
lst() { printf '%s\n' "$@" > "$T/cs.lst"; }
run() { ( cd "$R" && "$H" --root "$R" --adopt-working-tables --code-root "$CODE" --registry "${REGF:-$REG}" ${APPR:+--approved-registry "$APPR"} ${HELD:+--held-from "$HELD"} --files-from "$T/cs.lst" "$@" ) >"$T/out" 2>"$T/err"; RC=$?; }
tree_sum() { ( cd "$R" && find . -path ./.git -prune -o -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum | cut -d' ' -f1 ); }
one() { # one <label> <path> <content> <want rc> <want substring of the report>
  w "$2" "$3"; lst "$2"; local b; b="$(tree_sum)"; run; eq "$1: exit" "$RC" "$4"; [ -z "${5-}" ] || has "$1: report" "$(cat "$T/out")" "$5"; eq "$1: no byte of the tree changed" "$(tree_sum)" "$b"
}
fresh
# ---- 1 every check: a violating declared file gives 10, its clean twin 0 (the control needle) ------------------------------------
one "shell_parse clean"   good.sh '#!/bin/bash\necho hi\n' 0
one "shell_parse bad"     bad.sh  'if then\n' 10 "shell_parse"
one "merge_conflict clean" mc_ok.txt 'a\nb\n' 0
one "merge_conflict bad"  mc.txt  'a\n<<<<<<< HEAD\nb\n=======\nc\n>>>>>>> other\n' 10 "merge_conflict"
one "trailing_whitespace clean" tw_ok.txt 'a\n' 0
one "trailing_whitespace bad" tw.txt 'a \nb\n' 10 "trailing_whitespace"
one "end_of_file clean"   eof_ok.txt 'a\n' 0
one "end_of_file missing newline" eof1.txt 'a' 10 "end_of_file"
one "end_of_file extra blank lines" eof2.txt 'a\n\n' 10 "end_of_file"
one "check_yaml clean"    ok.yaml 'a: 1\nb: [1, 2]\n' 0
one "check_yaml bad"      bad.yaml 'a: [1, 2\nb: : :\n' 10 "check_yaml"
one "check_json clean"    ok.json '{"a": 1}\n' 0
one "check_json bad"      bad.json '{"a": 1,}\n' 10 "check_json"
one "revision_header clean (new md file)" ok.md '# T\n\n| Revision | 1 |\n| Last modified | 2026-10-05 |\n' 0
one "revision_header bad (new md file)" nohdr.md '# T\n\nbody\n' 10 "revision_header"
# large file: bound from the class (source 1,024,000 B); above 75% is a size_alarm, never a refusal
w big.txt "$(head -c 1030000 /dev/zero | tr '\0' a)\n"; lst big.txt; run; eq "large_file above the bound: exit" "$RC" 10; has "large_file named" "$(cat "$T/out")" "large_file"
w alarm.txt "$(head -c 800000 /dev/zero | tr '\0' a)\n"; lst alarm.txt; run; eq "size_alarm above 75% of the bound: exit 0" "$RC" 0; has "size_alarm reported" "$(cat "$T/out")" "size_alarm"
# binary files are never given to the text checks (listed as left out), but never refused for it
printf 'a \0b \n' > "$R/bin.dat"; lst bin.dat; run; eq "binary file with trailing blanks: 0" "$RC" 0; has "left out by the filter" "$(cat "$T/out")" "left_out"
# a binary-looking file that a language check selects by its suffix was NOT judged by it (`not_judged`, WF14 N4); the generic text checks leaving a binary file out are `left_out` only
hasnot "a generic left_out is not a not_judged" "$(cat "$T/out")" "not_judged"
printf 'if then\n\0\n' > "$R/bin.sh"; lst bin.sh; run; eq "binary-looking .sh: 0 (reported, never refused)" "$RC" 0
has "shell_parse left it out" "$(cat "$T/out")" "left_out	shell_parse	bin.sh"; has "and says it was not judged" "$(cat "$T/out")" "not_judged	shell_parse	bin.sh"
# no tracked or declared byte changes by any run
b="$(tree_sum)"; lst tw.txt eof1.txt bad.sh; run; eq "bytes unchanged after a failing run" "$(tree_sum)" "$b"
# ---- 2 class table application ------------------------------------------------------------------------------------------------
fresh; w docs/register/x.md 'a \n\nno header, trailing blank\n\n'; w docs/y.md 'a \n'
lst docs/register/x.md; run; eq "class generated: trailing space, extra blank, no header all skipped: 0" "$RC" 0
lst docs/y.md; run; eq "same content outside docs/register (source): 10" "$RC" 10
# ---- 3 changeset-scope checks ---------------------------------------------------------------------------------------------------
fresh; mkdir -p "$R/.github/workflows"; echo x > "$R/.github/workflows/ci.yml"; lst src/keep.txt; run
eq "no_ci: planted workflow file, unrelated declared file: 10" "$RC" 10; has "no_ci named" "$(cat "$T/out")" "no_ci"
rm -rf "$R/.github"; run; eq "no_ci clean: 0" "$RC" 0
touch "$R/LANDMINE"; run; eq "detect_landmines (changeset scope) fails: 10" "$RC" 10; has "detect_landmines named" "$(cat "$T/out")" "detect_landmines"; rm -f "$R/LANDMINE"
# ---- 4 registry rules ----------------------------------------------------------------------------------------------------------
fresh; lst src/keep.txt; run; has "deferred rows are listed, never run" "$(cat "$T/out")" "deferred	go_vet"; eq "deferred rows: exit 0" "$RC" 0
sed 's/^go_vet\t\([^\t]*\)\t\([^\t]*\)\tdeferred/go_vet\t\1\t\2\tratchet/' "$REG" > "$T/reg_ratchet.tsv"
REGF="$T/reg_ratchet.tsv" run; eq "a ratchet row (not implemented in this slice) refuses: 20" "$RC" 20; has "reason named" "$(cat "$T/err")$(cat "$T/out")" "ratchet_not_implemented"
{ cat "$REG"; printf 'newcheck\tbuiltin:newcheck\t-\tplain\t-\tfiles\ttest\n'; } > "$T/reg_new.tsv"
REGF="$T/reg_new.tsv" run; eq "a files-scope row with no class rows: 20 (check_unknown)" "$RC" 20; has "check_unknown named" "$(cat "$T/err")$(cat "$T/out")" "check_unknown"
{ cat "$REG"; printf 'evil\t../code/scripts/detect-landmines.sh\t-\tplain\t-\tchangeset\ttest\n'; } > "$T/reg_evil.tsv"
REGF="$T/reg_evil.tsv" run; eq "a registry command outside the code root (..): 20" "$RC" 20
{ cat "$REG"; printf 'gone\tscripts/nonexistent.sh\t-\tplain\t-\tchangeset\ttest\n'; } > "$T/reg_gone.tsv"
REGF="$T/reg_gone.tsv" run; eq "a registry command that does not exist: 20" "$RC" 20; has "check_command_absent" "$(cat "$T/err")$(cat "$T/out")" "check_command_absent"
REGF="$T/none.tsv" run; eq "registry unreadable: 20" "$RC" 20
# ---- 5 pending rows and the exit precedence (CENTRAL C1 (c)) ---------------------------------------------------------------------
fresh; cp "$REG" "$T/approved.tsv"; sed 's#^shell_parse\tbuiltin:shell_parse#shell_parse\tbuiltin:shell_parse_v2#' "$REG" > "$T/reg_changed.tsv"
w clean.txt 'a\n'; lst clean.txt
APPR="$T/approved.tsv" REGF="$T/reg_changed.tsv" run; eq "a differing unapproved row, nothing else failing: 14" "$RC" 14; has "check_pending_release named" "$(cat "$T/out")" "check_pending_release	shell_parse"
w mc.txt '<<<<<<< x\n=======\n>>>>>>> y\n'; lst clean.txt mc.txt
APPR="$T/approved.tsv" REGF="$T/reg_changed.tsv" run; eq "a failing check and a pending row in one run: 10 (never 14)" "$RC" 10
has "both named: the refusal" "$(cat "$T/out")" "merge_conflict"; has "both named: the pending row" "$(cat "$T/out")" "check_pending_release	shell_parse"
w bad.sh 'if then\n'; lst clean.txt bad.sh
APPR="$T/approved.tsv" REGF="$T/reg_changed.tsv" run; eq "the pending row is not run (shell error in a declared file): 14" "$RC" 14
APPR="$T/approved.tsv" run; eq "no row differs: the check runs and fails: 10" "$RC" 10
grep -v '^go_vet' "$REG" > "$T/approved_nogovet.tsv"; lst clean.txt
APPR="$T/approved_nogovet.tsv" run; eq "a row the approved copy lacks is pending: 14" "$RC" 14; has "named" "$(cat "$T/out")" "check_pending_release	go_vet"
APPR="$T/approved_nogovet.tsv" run --run-declared go_vet; eq "that row declared by the change set runs (deferred row: nothing to run): 0" "$RC" 0
# ---- 6 held-table rules --------------------------------------------------------------------------------------------------------
GO='{"schema":"review-verdict/1","verdict":"GO","covers_runs":[],"model":"m","effort":"?","blocking_findings":0}\n'
NOGO='{"schema":"review-verdict/1","verdict":"NO-GO","covers_runs":[],"model":"m","effort":"?","blocking_findings":1}\n'
addrow() { printf 'gen/**\tgenerated\ttest row\ttest\nother2/**\tgenerated\ttest row\ttest\n' >> "$R/scripts/repo/check_exemptions.tsv"; }
TBL=scripts/repo/check_exemptions.tsv
# a. table change with no review form: judged with HEAD tables, and the change itself gives 10 class_table_unreviewed
fresh; addrow; w gen/x.txt 'a \n'; lst "$TBL" gen/x.txt; run
eq "unreviewed table change: 10" "$RC" 10; has "class_table_unreviewed" "$(cat "$T/out")" "class_table_unreviewed"; has "gen/x.txt judged with the HEAD tables" "$(cat "$T/out")" "trailing_whitespace"
# b. GO declared in the same change set: the declared table decides
fresh; addrow; w gen/x.txt 'a \n'; w "$VERD" "$GO"; lst "$TBL" gen/x.txt "$VERD"; run; eq "table change with GO declared in the change set: 0" "$RC" 0
# c. GO already committed in HEAD
fresh; w "$VERD" "$GO"; commit_all "$R" go; addrow; w gen/x.txt 'a \n'; lst "$TBL" gen/x.txt; run; eq "table change with GO in HEAD: 0" "$RC" 0
# c2. a NO-GO verdict (or GO with blocking findings) is no review form
fresh; w "$VERD" "$NOGO"; commit_all "$R" nogo; addrow; w gen/x.txt 'a \n'; lst "$TBL" gen/x.txt; run; eq "verdict NO-GO in HEAD: 10" "$RC" 10; has "unreviewed" "$(cat "$T/out")" "class_table_unreviewed"
fresh; GOB='{"schema":"review-verdict/1","verdict":"GO","covers_runs":[],"model":"m","effort":"?","blocking_findings":2}\n'; w "$VERD" "$GOB"; commit_all "$R" gob; addrow; w gen/x.txt 'a \n'; lst "$TBL" gen/x.txt; run
eq "verdict GO with blocking findings in HEAD is no review form: 10" "$RC" 10; has "unreviewed" "$(cat "$T/out")" "class_table_unreviewed"
fresh; NG0='{"schema":"review-verdict/1","verdict":"NO-GO","covers_runs":[],"model":"m","effort":"?","blocking_findings":0}\n'; w "$VERD" "$NG0"; commit_all "$R" ng0; addrow; w gen/x.txt 'a \n'; lst "$TBL" gen/x.txt; run
eq "verdict NO-GO (even with 0 blocking findings) in HEAD is no review form: 10" "$RC" 10; has "unreviewed" "$(cat "$T/out")" "class_table_unreviewed"
# d. held on the verdict (not yet GO): held paths use the declared tables, unheld paths the HEAD tables
fresh; addrow; w "$VERD" "$NOGO"; w gen/x.txt 'a \n'; w other2/y.txt 'a \n'; w src/z.txt 'a \n'; w src/ok.txt 'a\n'
printf 'gen/x.txt\t%s\n' "$VERD" > "$T/held.tsv"
HELD="$T/held.tsv" lst "$TBL" "$VERD" gen/x.txt; HELD="$T/held.tsv" run; eq "held path under a held table change: 0" "$RC" 0; hasnot "no class_table_unreviewed when held" "$(cat "$T/out")" "class_table_unreviewed"
HELD="$T/held.tsv" lst "$TBL" "$VERD" gen/x.txt other2/y.txt; HELD="$T/held.tsv" run; eq "unheld path the held table would admit: 20" "$RC" 20; has "table_admits_unheld_path" "$(cat "$T/out")$(cat "$T/err")" "table_admits_unheld_path"; has "names the path and the check" "$(cat "$T/out")$(cat "$T/err")" "other2/y.txt"
HELD="$T/held.tsv" lst "$TBL" "$VERD" gen/x.txt src/z.txt; HELD="$T/held.tsv" run; eq "unheld path failing under both tables: 10" "$RC" 10; hasnot "not table_admits" "$(cat "$T/out")" "table_admits_unheld_path"
HELD="$T/held.tsv" lst "$TBL" "$VERD" gen/x.txt src/ok.txt; HELD="$T/held.tsv" run; eq "unheld path that passes under the HEAD tables is never refused: 0" "$RC" 0
# e. legacy_row_not_dropped
fresh; printf 'LEGACY.md\tlegacy-collection\tlegacy root report\tT\n' >> "$R/scripts/repo/check_exemptions.tsv"; w LEGACY.md 'legacy text\n'; w docs/issues/T-9.md 'ticket\n'; commit_all "$R" legacy
w LEGACY.md 'legacy text edited\n'; lst LEGACY.md; run; eq "edited legacy root report, row kept: 10" "$RC" 10; has "legacy_row_not_dropped names the row" "$(cat "$T/out")" "legacy_row_not_dropped	LEGACY.md"
lst LEGACY.md; git -C "$R" checkout -q -- LEGACY.md; run; eq "legacy root report declared unchanged: 0" "$RC" 0
w docs/issues/T-9.md 'ticket edited\n'; lst docs/issues/T-9.md; run; eq "an edited docs/issues ticket (glob row): 0" "$RC" 0
sed -i '/^LEGACY.md/d' "$R/scripts/repo/check_exemptions.tsv"; w LEGACY.md '# L\n\n| Revision | 2 |\n| Last modified | 2026-10-05 |\n\nedited with header\n'; w "$VERD" "$GO"
lst LEGACY.md scripts/repo/check_exemptions.tsv "$VERD"; run; eq "row dropped by a reviewed table change, header added: 0" "$RC" 0
w LEGACY.md 'edited, no header\n'; run; eq "row dropped but no header: 10 (class source now)" "$RC" 10; has "revision_header" "$(cat "$T/out")" "revision_header"
# ---- 7 call refusals (20): argument injection and misuse -------------------------------------------------------------------------
fresh
for bad in '../x' '/abs' '-n' 'a/../b'; do lst "$bad"; run; eq "unsafe declared path '$bad': 20" "$RC" 20; done
lst src/keep.txt; ( cd "$R" && "$H" --root "-x" --code-root "$CODE" --files-from "$T/cs.lst" ) >/dev/null 2>&1; eq "root starting with a dash: 20" "$?" 20
mkdir -p "$T/cwd"; mkrepo "$T/cwd/-x"; ( cd "$T/cwd" && "$H" --root -x --code-root "$CODE" --files-from "$T/cs.lst" ) >/dev/null 2>&1; eq "an existing repository named -x is refused as an option-like value: 20" "$?" 20
( cd "$R" && "$H" --root "$R" --code-root "$CODE" ) >/dev/null 2>&1; eq "no --files-from: 20" "$?" 20
run --bogus; eq "unknown option: 20" "$RC" 20
fin
