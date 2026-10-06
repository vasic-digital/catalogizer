#!/usr/bin/env bash
# =============================================================================
# scripts/ledger/tests/test_project_gate_ledger_mutations.sh
# Identity: catalogizer / feature 001 / WP-08 T087 paired mutations. Revision 2,
# 2026-10-05, draft, UNREVIEWED. Each mutant of the ratchet must make
# test_project_gate_ledger.sh FAIL; the unmutated copy must pass (control).
# rev 2 (WF2 fix round 3): exact-text mutants (a mutant whose text is not found exactly once is an ERROR,
# never silently skipped), the reviewer's survivors LM1-LM3 and one mutant per rule added in rev 2 of the ratchet.
# Usage: bash test_project_gate_ledger_mutations.sh [out.tsv]   (out.tsv: machine-written result table)
# =============================================================================
set -u
HERE=$(cd "$(dirname "$0")" && pwd); SRC="$HERE/../project_gate_ledger_ratchet.sh"
OUT=${1:-}
T=$(mktemp -d); trap 'rm -rf "$T"' EXIT; bad=0; killed=0; total=0
[ -z "$OUT" ] || { echo "# identity: task=T087 utc=$(date -u +%Y-%m-%dT%H:%M:%SZ) host=$(hostname -s) uid=$(id -u) git_head=$(git -C "$HERE" rev-parse HEAD 2>/dev/null)
# ratchet_sha256=$(sha256sum "$SRC" | cut -d' ' -f1) test_sha256=$(sha256sum "$HERE/test_project_gate_ledger.sh" | cut -d' ' -f1) runner_sha256=$(sha256sum "${BASH_SOURCE[0]}" | cut -d' ' -f1)
# columns: mutant<TAB>result(caught|SURVIVED|ERROR)<TAB>first failing check of the test" >"$OUT"; }
record() { total=$((total+1)); [ -z "$OUT" ] || printf '%s\t%s\t%s\n' "$1" "$2" "${3:-}" >>"$OUT"; }
run_mutant() { # name file
  if RATCHET="$2" bash "$HERE/test_project_gate_ledger.sh" >"$T/$1.log" 2>&1; then
    echo "SURVIVED $1 (test did not catch it)"; bad=1; record "$1" SURVIVED
  else echo "caught  $1"; killed=$((killed+1)); record "$1" caught "$(grep -m1 '^FAIL' "$T/$1.log" | cut -c1-110)"; fi
}
mut() { # name sed-expr   (rev 1 style: sed; a no-op is an ERROR)
  sed -E "$2" "$SRC" >"$T/$1.sh"; cmp -s "$SRC" "$T/$1.sh" && { echo "NOOP-MUTANT $1"; bad=1; record "$1" ERROR "no-op mutant"; return; }
  run_mutant "$1" "$T/$1.sh"
}
pm() { # name old new  (exact text, must occur exactly once)
  python3 - "$SRC" "$T/$1.sh" "$2" "$3" <<'PY' || { echo "TEXT-NOT-UNIQUE $1"; bad=1; record "$1" ERROR "mutant text not found exactly once"; return; }
import sys
src, dst, old, new = sys.argv[1:5]
s = open(src, encoding='utf-8').read()
if s.count(old) != 1: sys.exit(1)
open(dst, 'w', encoding='utf-8').write(s.replace(old, new))
PY
  run_mutant "$1" "$T/$1.sh"
}
cp "$SRC" "$T/control.sh"
if RATCHET="$T/control.sh" bash "$HERE/test_project_gate_ledger.sh" >"$T/control.log" 2>&1; then echo "control ok"; else echo "control FAILED"; bad=1; fi
mut no_F1 's/^say\(\) \{ echo/say() { case "$1" in F1*) return;; esac; echo/'
mut no_F2 's/^say\(\) \{ echo/say() { case "$1" in F2*) return;; esac; echo/'
mut no_F3 's/^say\(\) \{ echo/say() { case "$1" in F3*) return;; esac; echo/'
mut allow_md_site 's/\*\.md\) say .*; continue;;/*.md) :;;/'
mut allow_wildcard 's/say "F1b wildcard or truncated ledger name: \$n"/:/'
mut blind_ok 's/zero names extracted[^"]*"; exit 1;/zero names"; exit 0;/'
# reviewer survivors of WF2 and the rules added in rev 2
pm LM1_removal_reason_ignored '&& length($2)>=8{f=1}' '{f=1}'
pm LM1b_removal_reason_any_length 'length($2)>=8' 'length($2)>=0'
pm LM2_trailing_dash_ledger_row_ok "case \"\$n\" in *'*'*|*-) say \"F1b" "case \"\$n\" in *'*'*) say \"F1b"
pm LM3_extraction_drops_digits "'CM-[A-Z0-9*]+(-[A-Z0-9*]+)*(-[a-z]|-)?'" "'CM-[A-Z*]+(-[A-Z*]+)*(-[a-z]|-)?'"
pm I5a_non_code_site_accepted "      case \"\$ref\" in *.sh|*.bash|*.py|*.go|*.js|*.ts|*.rb|*.pl) ;; *) say \"F1 \$n implementation site \$ref is not a code file (data and name lists are carriers, not gates)\"; continue;; esac
" ""
pm I5a_comment_line_counts "grep -Ev '^[[:space:]]*(#|//|--)' \"\$f\" | grep -qF" "cat \"\$f\" | grep -qF"
pm I5a_own_inputs_allowed 'if grep -qxF -- "$(realpath -m "$f")" "$tmp/inputs"; then' 'if false; then'
pm I5a_site_outside_root_allowed "case \"\$ref\" in *..*|'') say" "case \"\$ref\" in 'never_matches_xx') say"
pm I5b_baseline_digits_concatenated 'if [ "${blines:-0}" != 1 ] || ! [[ ${base:-x} =~ ^[0-9]+$ ]]; then' 'base=$(tr -dc "0-9" <"$baseline"); if false; then'
pm I5b_baseline_first_line_wins '"${blines:-0}" != 1 ]' '"${blines:-0}" = zz ]'
pm I5b_baseline_non_integer_ok '|| ! [[ ${base:-x} =~ ^[0-9]+$ ]]; then' '|| false; then'
pm I5c_prev_working_file_only 'cat "$tmp/prev.head" >>"$tmp/prev"; prev_source=working+HEAD' 'prev_source=working+HEAD'
pm I5d_no_ratchet_down '[ "$cnt" -ge "$base" ] || say "F3 unimplemented count $cnt is below baseline $base: lower the baseline to $cnt in the same change (a ratchet only moves down)"' ':'
pm I5e_hyphen_prose_not_stripped "| sed -E 's/-[a-z]\$//' |" "| cat |"
pm I5f_any_nonempty_ref_is_tracked 'if [[ $ref =~ ^[A-Z][A-Z0-9]*-[0-9]+$ ]]; then :' 'if [ -n "$ref" ]; then :'
pm I5f_pending_without_flag 'if [ "$allow_pending" = 1 ]; then pending=$((pending+1));' 'if true; then pending=$((pending+1));'
pm I5f_pending_reported_as_plain_pass '[ "$pending" -eq 0 ] || verdict=PASS-INTERIM' '[ "$pending" -eq 0 ] || verdict=PASS'
pm I5_duplicate_ledger_name_ok '[ -z "$dups" ] || say "F1c duplicate ledger name(s): $dups"' ':'
pm I5c_prev_source_not_reported 'prev_source=working+HEAD
fi' 'prev_source=working-only
fi'
pm usage_missing_value_ignored '[ $# -ge 2 ] || { echo "usage_error: $1 needs a value" >&2; exit 2; }' ':'
# rev 3 (WF3 round 4): I3 routes 1-3 and the reviewer's W6, W7
pm I3_F2b_unregistered_name_ok '    grep -qxF -- "$n" "$tmp/prevall" || say "F2b $n is in the documents but not registered in --prev-names (add it there, in the same change)"' '    :'
pm I3_prev_names_absent_ok '  say "F2 prev-names file absent: $prev (an unresolvable input is a refusal, never a pass)"' '  :'
pm I3_F1d_orphan_row_ok '  grep -qxF -- "$n" "$tmp/names" || say "F1d $n ledger row names no gate found in the documents (an orphan row keeps freed slack and an unvalidated reference)"' '  :'
pm W6_item_id_regex_unanchored_at_end 'if [[ $ref =~ ^[A-Z][A-Z0-9]*-[0-9]+$ ]]; then :' 'if [[ $ref =~ ^[A-Z][A-Z0-9]*-[0-9]+ ]]; then :'
pm W7_slash_comment_counts_as_code "grep -Ev '^[[:space:]]*(#|//|--)' \"\$f\"" "grep -Ev '^[[:space:]]*(#)' \"\$f\""
pm I3_F2b_applied_to_family_only_names 'case "$n" in *'"'"'*'"'"'*|*-) continue;; esac
    grep -qxF -- "$n" "$tmp/prevall"' 'continue
    grep -qxF -- "$n" "$tmp/prevall"'
# WF6 (OWED-WP06-8): exact-name membership weakened to substring
pm R1_F1d_substring_match 'grep -qxF -- "$n" "$tmp/names" || say "F1d' 'grep -qF -- "$n" "$tmp/names" || say "F1d'
pm R2_F2b_substring_match 'grep -qxF -- "$n" "$tmp/prevall" || say "F2b' 'grep -qF -- "$n" "$tmp/prevall" || say "F2b'
echo "mutants=$total caught=$killed survived_or_error=$((total-killed))"
[ "$bad" -eq 0 ] && echo "ALL MUTANTS CAUGHT" || echo "MUTATION GAP"
exit "$bad"
