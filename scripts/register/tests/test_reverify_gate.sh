#!/usr/bin/env bash
# test_reverify_gate.sh - T179 (docs/04 §12.3 step 3, §13.3): scripts/register/gate.sh --completion, the feature
# completion gate. Every row of every reg_gate_checks view_not_done view counts as NOT done: a line
# `NOT DONE <view> rows=N` per view and a non-zero exit (4). The register gate of step 2 keeps its own result.
# RED before the --completion mode exists. Env GATE (default scripts/register/gate.sh) lets the mutation runner
# (mutate_reverify_gate.sh) substitute a mutant of gate.sh; REG_ROOT is passed so a mutant copy outside the tree still
# finds register_ext.sql. Real SQLite, real engine binary, scratch DBs only (never docs/workable_items.db).
. "$(dirname "$0")/lib.sh"
ident_header T179
GATE=${GATE:-$REG_DIR/gate.sh}
EXTSRC=${REG_EXT_SQL:-$REG_DIR/register_ext.sql}
D="$T_SCR/rv"; mkdir -p "$D"
DESC="scratch item for the completion gate tests, long enough for the engine description floor"
run_c() { local db=$1; shift; REG_ROOT=$ROOT bash "$GATE" --db "$db" --completion "$@" 2>&1; }
good_db() { fresh_ext_db "$1" || return 1; local id; id=$(mint "$1"); "$WI" add Task Low --db "$1" --id "$id" --prefix CAT --title "t" --description "$DESC" >/dev/null 2>&1; }
# rv_db db n [sev...]: a good DB plus n further items whose extension row says reverify_required=1 (a legacy re-verification entry)
rv_db() { local db=$1 n=$2 i id; good_db "$db" || return 1
  for i in $(seq 1 "$n"); do
    id=$(mint "$db"); "$WI" add Bug High --db "$db" --id "$id" --prefix CAT --title "rv$i" --description "$DESC" >/dev/null 2>&1
    sql "$db" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,reverify_required) VALUES ('$id','bug','source','high','machine_evidence',1)" >/dev/null
  done; }
n_lines() { printf '%s\n' "$1" | grep -c -- "$2"; }
# mkext name sed-expr : a copy of register_ext.sql with one edit (mutated reference), must differ
mkext() { sed -E "$2" "$EXTSRC" > "$D/$1.sql"; cmp -s "$EXTSRC" "$D/$1.sql" && { bad "setup: ext variant $1 identical to the source"; return 1; }; return 0; }

echo "== completion gate: the reverify queue =="
rv_db "$D/r1.db" 1 || { bad "setup"; finish; exit 1; }
n=$("$SQLITE3" -readonly "$D/r1.db" "SELECT count(*) FROM v_reverify_queue"); assert_eq "R0 fixture: v_reverify_queue holds exactly one row" "$n" 1
out=$(run_c "$D/r1.db"); rc=$?
[ $rc -ne 0 ] && printf '%s\n' "$out" | grep -qx 'NOT DONE v_reverify_queue rows=1' && ok "R1 one reverify_required=1 row prints exactly 'NOT DONE v_reverify_queue rows=1' and exits non-zero (rc $rc)" || bad "R1 rc=$rc [$(printf '%s' "$out" | head -4 | tr '\n' '|')]"
assert_eq "R1b the exit status is 4 (NOT DONE), distinct from gate failure 1 and partial 3" "$rc" 4
! printf '%s\n' "$out" | grep -Eq '^(GATE OK|COMPLETION OK)' && ok "R1c a NOT DONE run never prints GATE OK or COMPLETION OK" || bad "R1c [$out]"
printf '%s\n' "$out" | grep -qx 'COMPLETION NOT DONE views=1 rows=1' && ok "R1d the run ends with a one-line total 'COMPLETION NOT DONE views=1 rows=1'" || bad "R1d [$out]"
rv_db "$D/r2.db" 2; out=$(run_c "$D/r2.db"); rc=$?
printf '%s\n' "$out" | grep -qx 'NOT DONE v_reverify_queue rows=2' && [ $rc -eq 4 ] && ok "R2 two rows print rows=2 (the count is the view's row count)" || bad "R2 rc=$rc [$out]"
rv_db "$D/r3.db" 5; out=$(run_c "$D/r3.db"); rc=$?
printf '%s\n' "$out" | grep -qx 'NOT DONE v_reverify_queue rows=5' && printf '%s\n' "$out" | grep -qx 'COMPLETION NOT DONE views=1 rows=5' && ok "R3 five rows print rows=5 and the total counts rows" || bad "R3 rc=$rc [$out]"
before=$(sha256sum "$D/r1.db" | cut -d' ' -f1); run_c "$D/r1.db" >/dev/null; assert_eq "R4 the completion run does not modify the DB under test" "$(sha256sum "$D/r1.db" | cut -d' ' -f1)" "$before"

echo "== completion gate: empty queue and the plain register gate =="
good_db "$D/c1.db"; out=$(run_c "$D/c1.db"); rc=$?
[ $rc -eq 0 ] && printf '%s\n' "$out" | grep -qx 'COMPLETION OK' && ! printf '%s' "$out" | grep -q 'NOT DONE' && ok "C1 a clean register with an empty queue prints COMPLETION OK (rc 0)" || bad "C1 rc=$rc [$out]"
printf '%s\n' "$out" | grep -q 'INFO not_done checked=1' && ok "C1b the run reports how many view_not_done views it checked (checked=1: the count is evidence the loop ran)" || bad "C1b [$out]"
fresh_ext_db "$D/c2.db"; out=$(run_c "$D/c2.db"); rc=$?; [ $rc -eq 0 ] && printf '%s\n' "$out" | grep -qx 'COMPLETION OK' && ok "C2 a freshly applied empty register completes" || bad "C2 rc=$rc [$out]"
out=$(REG_ROOT=$ROOT bash "$GATE" --db "$D/r1.db" 2>&1); rc=$?
[ $rc -eq 0 ] && printf '%s\n' "$out" | grep -qx 'GATE OK' && ! printf '%s' "$out" | grep -q 'NOT DONE' && ok "C3 WITHOUT --completion the register gate still prints GATE OK with a populated reverify queue (step 2 unchanged, rc 0)" || bad "C3 rc=$rc [$out]"
out=$(REG_ROOT=$ROOT bash "$GATE" --completion 2>&1); rc=$?; [ $rc -eq 2 ] && printf '%s' "$out" | grep -q 'usage' && ok "C4 --completion without --db exits 2 with the usage message" || bad "C4 rc=$rc [$out]"
out=$(run_c "$D/does-not-exist.db"); rc=$?; [ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'database file absent' && ok "C5 --completion on an absent DB exits 1 naming the absence" || bad "C5 rc=$rc [$out]"
out=$(REG_ROOT=$ROOT bash "$GATE" --db "$D/r1.db" --completion --completion 2>&1); rc=$?; [ $rc -eq 4 ] && ok "C6 --completion given twice is the same as once" || bad "C6 rc=$rc [$out]"
out=$(REG_ROOT=$ROOT bash "$GATE" --completion --db "$D/r1.db" 2>&1); rc=$?; [ $rc -eq 4 ] && ok "C7 argument order does not matter (--completion before --db)" || bad "C7 rc=$rc"

echo "== completion gate: the register gate result is not hidden, the count is not suppressed =="
rv_db "$D/f1.db" 1; sql "$D/f1.db" "DROP TRIGGER reg_ids_no_delete" >/dev/null
out=$(run_c "$D/f1.db"); rc=$?
[ $rc -eq 1 ] && printf '%s\n' "$out" | grep -q 'FAIL.*v_gate_missing_objects.*reg_ids_no_delete' && printf '%s\n' "$out" | grep -qx 'NOT DONE v_reverify_queue rows=1' && printf '%s\n' "$out" | grep -qx 'GATE FAILED' && ! printf '%s' "$out" | grep -Eq '^(GATE OK|COMPLETION OK)' && ok "F1 a register gate failure plus a queued row: rc 1, the FAIL line AND the NOT DONE line both printed, GATE FAILED" || bad "F1 rc=$rc [$out]"
# F2 the gate registry of the DB under test is data: a view_not_done row deleted from it cannot hide the queue (the trusted reference holds the list)
rv_db "$D/f2.db" 1; sql "$D/f2.db" "DELETE FROM reg_gate_checks WHERE name='v_reverify_queue'" >/dev/null
out=$(run_c "$D/f2.db"); rc=$?
[ $rc -eq 1 ] && printf '%s\n' "$out" | grep -qx 'NOT DONE v_reverify_queue rows=1' && printf '%s\n' "$out" | grep -q 'FAIL.*\(seed\|differs\)' && ok "F2 the view_not_done row deleted from the DB's registry: the queue is STILL counted (list read from the trusted reference), seed drift FAILs (rc 1)" || bad "F2 rc=$rc [$out]"
# F3 an injected registry name of kind view_not_done is DATA, never SQL
rv_db "$D/f3.db" 1; sql "$D/f3.db" "INSERT INTO reg_gate_checks VALUES ('v_x; DROP TABLE reg_meta','view_not_done')" >/dev/null
out=$(run_c "$D/f3.db"); rc=$?
[ $rc -eq 1 ] && printf '%s\n' "$out" | grep -q 'FAIL.*invalid registry name' && ok "F3 an injected view_not_done registry name fails closed (invalid registry name)" || bad "F3 rc=$rc [$out]"
assert_eq "F3b the injected DROP did not run (reg_meta exists)" "$(sql "$D/f3.db" "select count(*) from sqlite_master where name='reg_meta'")" 1
# F4 the view is replaced by a table of the same name: not a view, counted nowhere, fails closed
rv_db "$D/f4.db" 1; sql "$D/f4.db" "DROP VIEW v_reverify_queue; CREATE TABLE v_reverify_queue(a)" >/dev/null
out=$(run_c "$D/f4.db"); rc=$?
[ $rc -eq 1 ] && printf '%s\n' "$out" | grep -q 'FAIL v_reverify_queue is not a view' && ! printf '%s' "$out" | grep -qx 'COMPLETION OK' && ok "F4 a view_not_done name that is a TABLE in the DB fails closed (not a view), never COMPLETION OK" || bad "F4 rc=$rc [$out]"
# F5 the view cannot be counted (redefined onto an absent table): a count that is not a number is a FAIL, not zero rows
rv_db "$D/f5.db" 1; sql "$D/f5.db" "DROP VIEW v_reverify_queue; CREATE VIEW v_reverify_queue AS SELECT atm_id FROM no_such_table_xyz" >/dev/null 2>&1
out=$(run_c "$D/f5.db"); rc=$?
[ $rc -eq 1 ] && printf '%s\n' "$out" | grep -q '^FAIL v_reverify_queue not counted' && ! printf '%s' "$out" | grep -Eq '^(GATE OK|COMPLETION OK|NOT DONE)' && ok "F5 a view_not_done view that errors on count is a FAIL 'not counted' (never zero rows, never a bogus rows= value; rc 1, no COMPLETION OK)" || bad "F5 rc=$rc [$out]"
# F6 evidence re-hash still runs in completion mode
rv_db "$D/f6.db" 0; sql "$D/f6.db" "INSERT INTO reg_evidence(atm_id,kind,evidence_class,path,sha256,size_bytes,produced_by,captured_at) VALUES ('CAT-001','log','source','absent/file.txt','$(printf 'a%.0s' $(seq 64))',1,'d','2026-10-05T00:00:00Z')" >/dev/null
out=$(run_c "$D/f6.db"); rc=$?; [ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'FAIL evidence 1: cited file absent' && ok "F6 the evidence re-hash runs in completion mode (absent cited file: rc 1)" || bad "F6 rc=$rc [$out]"

echo "== completion gate: PASS-PARTIAL =="
good_db "$D/p1.db"; sql "$D/p1.db" "INSERT INTO reg_evidence(atm_id,kind,evidence_class,path,sha256,size_bytes,produced_by,captured_at) VALUES ('CAT-001','tracker_receipt','source','https://example.invalid/r/1','$(printf 'a%.0s' $(seq 64))',1,'d','2026-10-05T00:00:00Z')" >/dev/null
out=$(run_c "$D/p1.db"); rc=$?
[ $rc -eq 3 ] && printf '%s' "$out" | grep -q 'PASS-PARTIAL' && ! printf '%s' "$out" | grep -Eq '^(GATE OK|COMPLETION OK)' && ok "P1 an empty queue with URL-skipped evidence is PASS-PARTIAL (rc 3), never COMPLETION OK" || bad "P1 rc=$rc [$out]"
out=$(run_c "$D/p1.db" --allow-url-evidence); rc=$?
[ $rc -eq 0 ] && printf '%s' "$out" | grep -q 'PASS-PARTIAL' && printf '%s' "$out" | grep -q 'COMPLETION' && ! printf '%s\n' "$out" | grep -qx 'COMPLETION OK' && ok "P2 --allow-url-evidence: rc 0 but still a PARTIAL line, never a bare COMPLETION OK" || bad "P2 rc=$rc [$out]"
rv_db "$D/p3.db" 1; sql "$D/p3.db" "INSERT INTO reg_evidence(atm_id,kind,evidence_class,path,sha256,size_bytes,produced_by,captured_at) VALUES ('CAT-001','tracker_receipt','source','https://example.invalid/r/1','$(printf 'a%.0s' $(seq 64))',1,'d','2026-10-05T00:00:00Z')" >/dev/null
out=$(run_c "$D/p3.db"); rc=$?; [ $rc -eq 4 ] && printf '%s\n' "$out" | grep -qx 'NOT DONE v_reverify_queue rows=1' && ok "P3 NOT DONE (4) takes priority over PASS-PARTIAL (3)" || bad "P3 rc=$rc [$out]"

echo "== completion gate: the reference list is data (mutated reference) =="
# E1 the reference registers no view_not_done at all: fail closed (a gate that checks nothing must not complete)
mkext e1 "/\('v_reverify_queue','view_not_done'\),/s/\('v_reverify_queue','view_not_done'\),//" && {
  REG_EXT_SQL="$D/e1.sql" fresh_ext_db "$D/e1.db"; id=$(mint "$D/e1.db"); "$WI" add Bug High --db "$D/e1.db" --id "$id" --prefix CAT --title x --description "$DESC" >/dev/null 2>&1
  sql "$D/e1.db" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,reverify_required) VALUES ('$id','bug','source','high','machine_evidence',1)" >/dev/null
  out=$(REG_ROOT=$ROOT bash "$GATE" --db "$D/e1.db" --ext "$D/e1.sql" --completion 2>&1); rc=$?
  [ $rc -ne 0 ] && printf '%s' "$out" | grep -q 'FAIL.*no view_not_done row' && ! printf '%s' "$out" | grep -Eq '^COMPLETION OK' && ok "E1 a reference registry with no view_not_done row fails closed (nothing is checked, nothing completes)" || bad "E1 rc=$rc [$out]"; }
# E2 two view_not_done views: one line per view, in name order, total counts both
mkext e2 "s/\('v_ids_without_item','view_report'\)/('v_ids_without_item','view_not_done')/" && {
  REG_EXT_SQL="$D/e2.sql" fresh_ext_db "$D/e2.db"; id=$(mint "$D/e2.db")
  out=$(REG_ROOT=$ROOT bash "$GATE" --db "$D/e2.db" --ext "$D/e2.sql" --completion 2>&1); rc=$?
  [ $rc -eq 4 ] && printf '%s\n' "$out" | grep -qx 'NOT DONE v_ids_without_item rows=1' && ! printf '%s' "$out" | grep -q 'NOT DONE v_reverify_queue' && printf '%s\n' "$out" | grep -qx 'COMPLETION NOT DONE views=1 rows=1' && ok "E2 a second view_not_done view is read from the reference: only the populated view is reported (views=1)" || bad "E2 rc=$rc [$out]"
  id2=$(mint "$D/e2.db"); "$WI" add Bug High --db "$D/e2.db" --id "$id2" --prefix CAT --title y --description "$DESC" >/dev/null 2>&1
  sql "$D/e2.db" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,reverify_required) VALUES ('$id2','bug','source','high','machine_evidence',1)" >/dev/null
  id3=$(mint "$D/e2.db")
  out=$(REG_ROOT=$ROOT bash "$GATE" --db "$D/e2.db" --ext "$D/e2.sql" --completion 2>&1); rc=$?
  want=$'NOT DONE v_ids_without_item rows=2\nNOT DONE v_reverify_queue rows=1'
  [ $rc -eq 4 ] && [ "$(printf '%s\n' "$out" | grep '^NOT DONE')" = "$want" ] && printf '%s\n' "$out" | grep -qx 'COMPLETION NOT DONE views=2 rows=3' && ok "E3 both views populated: one line per view in name order, total views=2 rows=3" || bad "E3 rc=$rc [$out]"; }
# E4 a malformed name in the REFERENCE registry (a reviewed-DDL edit gone wrong) is refused before it reaches a query
mkext e4 "s/\\('v_reverify_queue','view_not_done'\\)/('v_Bad Name','view_not_done')/" && {
  REG_EXT_SQL="$D/e4.sql" fresh_ext_db "$D/e4.db"
  out=$(REG_ROOT=$ROOT bash "$GATE" --db "$D/e4.db" --ext "$D/e4.sql" --completion 2>&1); rc=$?
  [ $rc -ne 0 ] && printf '%s' "$out" | grep -q 'FAIL invalid view name in the reference registry' && ok "E4 a malformed view_not_done name in the reference registry is refused (invalid view name)" || bad "E4 rc=$rc [$out]"; }
finish
