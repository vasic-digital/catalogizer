#!/usr/bin/env bash
# T040b test (TDD): the path-class loader scripts/repo/check_class.sh against the reviewed tables (plan owner's rule (V)).
# Usage   bash scripts/repo/tests/test_check_classes.sh      Env: H=<loader path> (the mutation driver points it at a copy;
#         a copy resolves the real tables through TBL=<dir>, default scripts/repo)
# Scope   class resolution (fixture root > exact path > glob rows > `source`), class_ambiguous / check_unknown refusals,
#         the per-class apply/skip/bound rows, git-wildmatch glob semantics. NOT here (blocked on scope_check.sh and
#         validate_cheap.sh, which are outside this slice): the table-change/held-table rules and the fixtures that run
#         through the real S2/S3 stages.
set -u
cd "$(git rev-parse --show-toplevel)" || exit 2
H="${H:-scripts/repo/check_class.sh}"; case "$H" in /*) ;; *) H="$(pwd)/$H" ;; esac
TBL="${TBL:-$(pwd)/scripts/repo}"
EV=specs/001-full-project-audit-remediation/evidence; AUD=specs/001-full-project-audit-remediation/audit
T="$(mktemp -d "${TMPDIR:-/tmp}/cc_test.XXXXXX")"; trap 'rm -rf "$T"' EXIT
PASSN=0; FAILN=0
ok()  { PASSN=$((PASSN+1)); echo "ok   $1"; }
bad() { FAILN=$((FAILN+1)); echo "FAIL $1"; }
eq()  { if [ "$2" = "$3" ]; then ok "$1 (=$2)"; else bad "$1 (got '$2', want '$3')"; fi; }
[ -x "$H" ] || echo "NOTE: $H is absent or not executable (RED state: every case must FAIL)"
CHECKS="secret_fold private_key merge_conflict trailing_whitespace end_of_file check_yaml check_json large_file shell_parse revision_header anti_bluff check_pins bank_validator go_fmt go_vet go_imports no_false_positive_log eslint prettier"
# ---- 1 resolution of paths to classes (real tables) ------------------------------------------------------------------------
for pair in "scripts/foo.sh:source" "docs/new/page.md:source" "tests/contracts/verifications/a.json:source" "scripts/release/test_path_patterns.tsv:source" \
  "docs/workable_items.db:generated" "docs/register/Issues.md:generated" "docs/register/sub/register.sql:generated" \
  "$EV/p0-exit.json:evidence" "$AUD/golden.json:evidence" "$AUD/lumen-verify/results.tsv:evidence" "$EV/blobs/0123abcd:evidence" "$EV/x/SHA256SUMS:evidence" \
  "$EV/notes/a.md:source" "$EV/x/other.jsonl:evidence" "$EV/ledger.jsonl:evidence-ledger" "$EV/flake_ledger.jsonl:evidence-ledger" \
  "$AUD/patches/a.patch:patches" "scripts/x.patch:source" \
  ".specify/memory/constitution-appendix.md:governance-carrier" "submodules/constitution/Constitution.md:governance-carrier" \
  "docs/issues/ISS-1.md:legacy-collection" "$EV/x/GoFile_test.go:source"; do
  out="$("$H" --exemptions "$TBL/check_exemptions.tsv" --classes "$TBL/check_classes.tsv" --fixture-roots /dev/null --check class "${pair%%:*}" 2>/dev/null)"
  eq "class of ${pair%%:*}" "$out" "${pair##*:}"; done
eq "ledger resolves to evidence-ledger, never class_ambiguous (exact path outranks the glob)" "$("$H" --exemptions "$TBL/check_exemptions.tsv" --classes "$TBL/check_classes.tsv" --fixture-roots /dev/null --check class $EV/ledger.jsonl >/dev/null 2>&1; echo $?)" 0
# ---- 2 per-class apply/skip/bound -----------------------------------------------------------------------------------------------
ap() { "$H" --exemptions "$TBL/check_exemptions.tsv" --classes "$TBL/check_classes.tsv" --fixture-roots "${FR:-/dev/null}" --check "$1" "$2" 2>/dev/null; }
for c in $CHECKS; do [ "$c" = large_file ] && continue; eq "source applies $c" "$(ap $c scripts/foo.sh)" apply; done
eq "source large_file bound" "$(ap large_file scripts/foo.sh)" 1024000
for c in trailing_whitespace end_of_file revision_header shell_parse anti_bluff check_pins bank_validator go_fmt eslint; do eq "generated skips $c" "$(ap $c docs/register/Issues.md)" skip; done
for c in secret_fold private_key merge_conflict check_yaml check_json; do eq "generated applies $c" "$(ap $c docs/register/Issues.md)" apply; done
eq "generated large_file is the 16 MiB register bound" "$(ap large_file docs/workable_items.db)" 16777216
for c in trailing_whitespace end_of_file merge_conflict revision_header shell_parse anti_bluff go_vet prettier; do eq "evidence skips $c" "$(ap $c $EV/p0-exit.json)" skip; done
for c in secret_fold private_key check_json check_yaml; do eq "evidence applies $c" "$(ap $c $EV/p0-exit.json)" apply; done
eq "evidence large_file" "$(ap large_file $EV/p0-exit.json)" 1024000
for c in check_json check_yaml trailing_whitespace end_of_file revision_header shell_parse; do eq "evidence-ledger skips $c" "$(ap $c $EV/ledger.jsonl)" skip; done
for c in secret_fold private_key merge_conflict; do eq "evidence-ledger applies $c" "$(ap $c $EV/ledger.jsonl)" apply; done
eq "evidence-ledger large_file is 32 MiB" "$(ap large_file $EV/ledger.jsonl)" 33554432
for c in trailing_whitespace end_of_file check_yaml check_json shell_parse revision_header; do eq "patches skips $c" "$(ap $c $AUD/patches/a.patch)" skip; done
for c in secret_fold private_key merge_conflict; do eq "patches applies $c" "$(ap $c $AUD/patches/a.patch)" apply; done
eq "patches large_file" "$(ap large_file $AUD/patches/a.patch)" 1024000
for c in secret_fold merge_conflict trailing_whitespace end_of_file revision_header private_key; do eq "governance-carrier applies $c" "$(ap $c submodules/constitution/Constitution.md)" apply; done
for c in check_yaml check_json shell_parse anti_bluff check_pins; do eq "governance-carrier skips $c" "$(ap $c submodules/constitution/Constitution.md)" skip; done
eq "governance-carrier large_file is 4 MiB" "$(ap large_file .specify/memory/constitution-appendix.md)" 4194304
eq "legacy-collection skips revision_header" "$(ap revision_header docs/issues/ISS-1.md)" skip
for c in trailing_whitespace end_of_file shell_parse secret_fold; do eq "legacy-collection applies $c" "$(ap $c docs/issues/ISS-1.md)" apply; done
eq "shell_parse skipped in generated" "$(ap shell_parse docs/register/x.md)" skip
eq "shell_parse applied in legacy-collection" "$(ap shell_parse docs/issues/a.md)" apply
# ---- 3 fixture roots (class fixtures; the root's row decides the skipped checks) -------------------------------------------------
printf 'tests/security/needles/\tsecret_fold,anti_bluff\tneedles\tT150\n' > "$T/fr.txt"; FR="$T/fr.txt"
eq "fixture root: class" "$(ap class tests/security/needles/n1.txt)" fixtures
eq "fixture root: named check is skipped (secret_fold)" "$(ap secret_fold tests/security/needles/n1.txt)" skip
eq "fixture root: named check is skipped (anti_bluff)" "$(ap anti_bluff tests/security/needles/sub/n2.go)" skip
eq "fixture root: a check its row does not name still applies (trailing_whitespace)" "$(ap trailing_whitespace tests/security/needles/n1.txt)" apply
eq "fixture root: large_file bound 1,024,000" "$(ap large_file tests/security/needles/n1.txt)" 1024000
eq "golden-false: one directory outside the root is source" "$(ap class tests/security/other/n1.txt)" source
eq "golden-false: sibling sharing the root's name prefix is source" "$(ap class tests/security/needles_x/n1.txt)" source
eq "golden-false: outside the root secret_fold applies" "$(ap secret_fold tests/security/other/n1.txt)" apply
FR=""
# ---- 4 glob semantics and ambiguity (mini tables) ------------------------------------------------------------------------------
mkdir -p "$T/m"; printf 'a/**/b.json\tevidence\tr\tT\nd/*.txt\tgenerated\tr\tT\nexact/file.txt\tpatches\tr\tT\nexact/*.txt\tgenerated\tr\tT\n' > "$T/m/ex.tsv"
mc() { "$H" --exemptions "$T/m/ex.tsv" --classes "$TBL/check_classes.tsv" --fixture-roots /dev/null --check class "$1" 2>"$T/err"; RC=$?; }
eq "** matches zero directories" "$(mc a/b.json)" evidence
eq "** matches several directories" "$(mc a/x/y/b.json)" evidence
eq "* stays inside one directory" "$(mc d/e/f.txt)" source
eq "* matches within a directory" "$(mc d/f.txt)" generated
eq "exact path outranks a glob row of another class" "$(mc exact/file.txt)" patches
printf 'g1/*\tevidence\tr\tT\ng1/f.*\tgenerated\tr\tT\ng2/*\tevidence\tr\tT\ng2/f.*\tevidence\tr\tT\n' > "$T/m/ex.tsv"
mc g1/f.x; eq "two glob rows of different classes: exit 20" "$RC" 20
eq "class_ambiguous named on stderr" "$(awk '{print $1}' "$T/err" | head -1)" class_ambiguous
mc g2/f.x; eq "two glob rows of the same class: not ambiguous" "$RC" 0
# ---- 5 check_unknown: a scope-files check with no class rows is refused, never given a default ---------------------------------
printf 'check\tcommand\timage\tmode\tbaseline\tscope\nshell_parse\tc\ti\tplain\t-\tfiles\ndetect_landmines\tc\ti\tplain\t-\tchangeset\nnew_files_check\tc\ti\tplain\t-\tfiles\n' > "$T/reg.tsv"
"$H" --exemptions "$TBL/check_exemptions.tsv" --classes "$TBL/check_classes.tsv" --fixture-roots /dev/null --checks-registry "$T/reg.tsv" --check class scripts/a.sh >/dev/null 2>"$T/err"; RC=$?
eq "scope-files check without class rows: exit 20" "$RC" 20
eq "check_unknown names the check" "$(head -1 "$T/err" | awk '{print $1" "$2}')" "check_unknown new_files_check"
printf 'check\tcommand\timage\tmode\tbaseline\tscope\nshell_parse\tc\ti\tplain\t-\tfiles\ndetect_landmines\tc\ti\tplain\t-\tchangeset\nold_deferred\tc\ti\tdeferred\t-\tfiles\n' > "$T/reg.tsv"
"$H" --exemptions "$TBL/check_exemptions.tsv" --classes "$TBL/check_classes.tsv" --fixture-roots /dev/null --checks-registry "$T/reg.tsv" --check class scripts/a.sh >/dev/null 2>&1; RC=$?
eq "golden-false: changeset-scope check and a deferred row are not refused" "$RC" 0
# ---- 6 the reviewed class table is complete: every class has a row for every check of the closed set ----------------------------
for k in source generated evidence patches governance-carrier fixtures evidence-ledger legacy-collection; do
  for c in $CHECKS; do n="$(awk -F'\t' -v k=$k -v c=$c '$1==k && $2==c' "$TBL/check_classes.tsv" | wc -l | tr -d ' ')"; [ "$n" = 1 ] || bad "class table row for $k/$c (found $n)"; done; done
ok "class table completeness scanned (any FAIL line above names a missing or duplicate row)"
eq "every row has a reason" "$(awk -F'\t' '!/^#/ && NF>=1 && ($4=="")' "$TBL/check_classes.tsv" | wc -l | tr -d ' ')" 0
eq "every exemption row has a reason and an owner" "$(awk -F'\t' '!/^#/ && NF>=1 && ($3==""||$4=="")' "$TBL/check_exemptions.tsv" | wc -l | tr -d ' ')" 0
echo "---- $PASSN ok, $FAILN failed"; [ "$FAILN" = 0 ] && [ -x "$H" ]
