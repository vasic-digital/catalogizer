#!/usr/bin/env bash
# test_run_mutations.sh - the mutation runner is itself a gate (WF2 I-7): it must classify killed, equivalent,
# surviving and count-only mutants correctly, refuse stale equivalence rows, and write a summary whose numbers add up.
# A fake test (grep on the mutant DDL, no SQLite) stands in for the real tests, so the whole file runs in seconds.
. "$(dirname "$0")/lib.sh"
ident_header T062r
RUNNER="$REG_DIR/run_mutations.sh"; W="$T_SCR/rm"; mkdir -p "$W"
REF_DDL="$REG_DIR/register_ext.sql"
cat > "$W/fake_test.sh" <<'EOF'
#!/usr/bin/env bash
# fails when a trigger of the reference DDL is missing from the mutant DDL, except the names in FAKE_IGNORE
ref="${FAKE_REF:?}"; bad=0
for t in $(sed -n 's/^CREATE TRIGGER IF NOT EXISTS \([A-Za-z0-9_]*\).*/\1/p' "$ref"); do
  case " ${FAKE_IGNORE:-} " in *" $t "*) continue;; esac
  grep -q "CREATE TRIGGER IF NOT EXISTS $t\b" "${REG_EXT_SQL:?}" || { echo "FAIL ${FAKE_LABEL:-T} trigger $t missing"; bad=1; break; }
done
exit $bad
EOF
export FAKE_REF="$REF_DDL"
run() { bash "$RUNNER" --test "$W/fake_test.sh" --kinds trigger --jobs 4 --out "$W/out.tsv" "$@" >"$W/run.out" 2>"$W/run.err"; echo $?; }
cnt() { grep -v '^#' "$W/out.tsv" | awk -F'\t' -v s="$1" '$4==s' | wc -l; }

export FAKE_IGNORE="reg_ids_no_update reg_ids_no_delete"
rc=$(run); assert_eq "R1 two mutants the fake test cannot see survive: exit 1" "$rc" 1
assert_eq "R2 summary: 41 killed, 0 equivalent, 2 survived of 43" "$(sed 's/ count_only_in_survived=.*//' "$W/out.tsv.summary")" "mutants=43 killed=41 equivalent_reviewed=0 survived_unreviewed=2"
assert_eq "R3 the table agrees with the summary (41 yes, 2 NO)" "$(cnt yes)/$(cnt NO)" "41/2"
printf 'trigger\treg_ids_no_update\t\tfake reason one\tauthor\ntrigger\treg_ids_no_delete\t\tfake reason two\tauthor\n' > "$W/eq.tsv"
rc=$(run --equivalent "$W/eq.tsv"); assert_eq "R4 both survivors listed as reviewed-equivalent: exit 0" "$rc" 0
assert_eq "R5 summary counts them as equivalent_reviewed, not killed" "$(sed 's/ count_only_in_survived=.*//' "$W/out.tsv.summary")" "mutants=43 killed=41 equivalent_reviewed=2 survived_unreviewed=0"
assert_eq "R6 the equivalence reason is printed in the table" "$(grep -c 'fake reason one' "$W/out.tsv")" 1
printf 'trigger\treg_ids_no_update\t\tr\tauthor\ntrigger\treg_ids_no_delete\t\tr\tauthor\ntrigger\treg_ids_no_replace\t\ta killed mutant listed as equivalent\tauthor\n' > "$W/eq2.tsv"
rc=$(run --equivalent "$W/eq2.tsv"); assert_eq "R7 a row naming a mutant that a test KILLS is stale: exit 4" "$rc" 4
grep -q 'STALE equivalence entry (a test kills it): trigger reg_ids_no_replace' "$W/run.err" && ok "R8 the stale row is named on stderr" || bad "R8 [$(cat "$W/run.err")]"
printf 'trigger\treg_ids_no_update\t\tr\tauthor\ntrigger\treg_ids_no_delete\t\tr\tauthor\ntrigger\tno_such_trigger\t\tr\tauthor\n' > "$W/eq3.tsv"
rc=$(run --equivalent "$W/eq3.tsv"); assert_eq "R9 a row naming no mutant is stale: exit 4" "$rc" 4
grep -q 'STALE equivalence entry (no such mutant): trigger no_such_trigger' "$W/run.err" && ok "R10 named on stderr" || bad "R10 [$(cat "$W/run.err")]"
printf 'check\treg_evidence\tCHECK (x)\tr\tauthor\ntrigger\treg_ids_no_update\t\tr\tauthor\ntrigger\treg_ids_no_delete\t\tr\tauthor\n' > "$W/eq4.tsv"
rc=$(run --equivalent "$W/eq4.tsv"); assert_eq "R11 a row of a kind this run did not select is not stale: exit 0" "$rc" 0
printf 'trigger\treg_ids_no_update\tnot-the-prefix\tr\tauthor\ntrigger\treg_ids_no_delete\t\tr\tauthor\n' > "$W/eq5.tsv"
rc=$(run --equivalent "$W/eq5.tsv"); assert_eq "R12 a detail prefix that matches nothing leaves the mutant a survivor (and the row stale): exit 4" "$rc" 4
# count-only kills are not kills
cat > "$W/count_test.sh" <<'EOF'
#!/usr/bin/env bash
n=$(grep -c '^CREATE TRIGGER IF NOT EXISTS' "${REG_EXT_SQL:?}"); [ "$n" = "$(grep -c '^CREATE TRIGGER IF NOT EXISTS' "${FAKE_REF:?}")" ] || { echo "FAIL A3 trigger count :: got [$n]"; exit 1; }
EOF
unset FAKE_IGNORE
rc=$(bash "$RUNNER" --test "$W/count_test.sh" --kinds trigger --count-checks '^FAIL A3 ' --out "$W/out.tsv" >"$W/run.out" 2>&1; echo $?)
assert_eq "R13 a test that fails only on a count kills nothing: every mutant is COUNT_ONLY, exit 1" "$rc/$(cnt COUNT_ONLY)/$(cnt yes)" "1/43/0"
assert_eq "R14 COUNT_ONLY rows are counted as unreviewed survivors in the summary" "$(sed 's/ count_only_in_survived=.*//' "$W/out.tsv.summary")" "mutants=43 killed=0 equivalent_reviewed=0 survived_unreviewed=43"
rc=$(bash "$RUNNER" --test "$W/count_test.sh" --kinds trigger --out "$W/out.tsv" >"$W/run.out" 2>&1; echo $?)
assert_eq "R15 the same test without --count-checks kills them all (the option is what separates the two)" "$rc/$(cnt yes)" "0/43"
# control run, usage, options
printf '#!/usr/bin/env bash\nexit 1\n' > "$W/bad_control.sh"
rc=$(bash "$RUNNER" --test "$W/bad_control.sh" --kinds trigger --out "$W/out.tsv" >"$W/run.out" 2>&1; echo $?)
assert_eq "R16 a test that fails on the unmutated DDL aborts the run (exit 3)" "$rc" 3
rc=$(bash "$RUNNER" --test 2>/dev/null; echo $?); assert_eq "R17 an option without a value exits 2 (no hang)" "$rc" 2
rc=$(bash "$RUNNER" --kinds trigger 2>/dev/null; echo $?); assert_eq "R18 no --test/--out exits 2" "$rc" 2
# the stale check looks at the mutants a row NAMES (kind, name, detail prefix), not at every mutant of that table
cat > "$W/check_test.sh" <<'EOF'
#!/usr/bin/env bash
grep -q "CHECK (polarity IN ('RED','GREEN'))" "${REG_EXT_SQL:?}" || { echo "FAIL polarity check missing"; exit 1; }
EOF
printf "check\treg_evidence\tCHECK (size_bytes>0)\ta survivor of the same table\tauthor\n" > "$W/eq6.tsv"
bash "$RUNNER" --test "$W/check_test.sh" --kinds check --equivalent "$W/eq6.tsv" --out "$W/out.tsv" >"$W/run.out" 2>"$W/run.err"
grep -q STALE "$W/run.err" && bad "R21 a row for a SURVIVING check mutant is called stale because another mutant of the same table is killed [$(cat "$W/run.err")]" || ok "R21 a row for a surviving mutant of a table is not stale just because a sibling mutant of that table is killed"
assert_eq "R21b the survivor is reported EQUIV" "$(grep -c "a survivor of the same table" "$W/out.tsv")/$(grep -v '^#' "$W/out.tsv" | awk -F'\t' '$4=="EQUIV"' | wc -l)" "1/1"
printf "check\treg_evidence\tCHECK (polarity IN\tthe killed one\tauthor\n" > "$W/eq7.tsv"
bash "$RUNNER" --test "$W/check_test.sh" --kinds check --equivalent "$W/eq7.tsv" --out "$W/out.tsv" >"$W/run.out" 2>"$W/run.err"; rc=$?
assert_eq "R22 a row naming the one mutant a test KILLS is stale: exit 4" "$rc" 4
grep -q 'STALE equivalence entry (a test kills it): check reg_evidence' "$W/run.err" && ok "R22b named on stderr" || bad "R22b [$(cat "$W/run.err")]"
# a clause mutant whose text is not in the DDL is an error, never silently skipped
sed 's/AND p.evidence_id <= (SELECT ev_hwm/AND p.evidence_id <=  (SELECT ev_hwm/' "$REF_DDL" > "$W/changed.sql"
rc=$(bash "$RUNNER" --test "$W/fake_test.sh" --ddl "$W/changed.sql" --kinds trigger --out "$W/out.tsv" >"$W/run.out" 2>"$W/run.err"; echo $?)
assert_eq "R19 a DDL in which a clause mutant's text no longer occurs exactly once aborts (exit 2)" "$rc" 2
grep -q 'update CLAUSE_MUTANTS' "$W/run.err" && ok "R20 the message names the remedy" || bad "R20 [$(cat "$W/run.err")]"
finish
