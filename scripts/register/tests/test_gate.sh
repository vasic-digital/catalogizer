#!/usr/bin/env bash
# test_gate.sh - T063 (docs/04 §12.3 step 2): scripts/register/gate.sh. Golden-good must print GATE OK;
# every golden-bad fixture DB must FAIL (rc 1, a FAIL line naming the check). RED before gate.sh exists.
# Env GATE (default scripts/register/gate.sh) lets the mutation runner substitute a mutant; REG_ROOT is passed
# so a mutant copy outside the tree still finds register_ext.sql.
. "$(dirname "$0")/lib.sh"
ident_header T063
GATE=${GATE:-$REG_DIR/gate.sh}
D="$T_SCR/g"; mkdir -p "$D"
DESC="scratch item for the register gate tests, long enough for the engine description floor"
H() { printf '%s' "$1" | sha256sum | cut -d' ' -f1; }
run_gate() { REG_ROOT=$ROOT bash "$GATE" --db "$1" 2>&1; }
good_db() { fresh_ext_db "$1" || return 1; local id; id=$(mint "$1"); "$WI" add Task Low --db "$1" --id "$id" --prefix CAT --title "t" --description "$DESC" >/dev/null 2>&1; }
must_fail() {  # label db pattern
  local out rc; out=$(run_gate "$2"); rc=$?
  if [ $rc -ne 0 ] && ! printf '%s' "$out" | grep -q 'GATE OK' && printf '%s' "$out" | grep -Eq "$3"; then ok "$1"; else bad "$1 :: rc=$rc out=[$(printf '%s' "$out" | head -4 | tr '\n' '|')] wanted /$3/"; fi; }

echo "== golden-good =="
good_db "$D/ok0.db" || { bad "setup"; finish; exit 1; }
out=$(run_gate "$D/ok0.db"); rc=$?
[ $rc -eq 0 ] && printf '%s' "$out" | grep -q '^GATE OK' && ok "G1 clean DB with one item prints GATE OK (rc 0)" || bad "G1 rc=$rc [$out]"
fresh_ext_db "$D/ok1.db"; out=$(run_gate "$D/ok1.db"); [ $? -eq 0 ] && ok "G2 freshly applied empty register passes" || bad "G2 [$out]"
before=$(sha256sum "$D/ok0.db" | cut -d' ' -f1); run_gate "$D/ok0.db" >/dev/null; assert_eq "G3 the gate does not modify the DB under test" "$(sha256sum "$D/ok0.db" | cut -d' ' -f1)" "$before"
out=$(REG_ROOT=$ROOT bash "$GATE" 2>&1); rc=$?; [ $rc -eq 2 ] && printf '%s' "$out" | grep -q 'usage' && ok "G4 missing --db exits 2 with a usage message" || bad "G4 rc=$rc [$out]"
out=$(run_gate "$D/does-not-exist.db"); rc=$?; [ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'database file absent' && ok "G5 absent DB file exits 1 naming the absence (and not created)" || bad "G5 rc=$rc [$out]"
[ ! -e "$D/does-not-exist.db" ] && ok "G6 absent DB file was not created" || bad "G6 created"

echo "== golden-bad fixtures =="
# B1 orphan finding: a finding whose item row does not exist
good_db "$D/b1.db"; X=$(mint "$D/b1.db")
sql "$D/b1.db" "INSERT INTO reg_components(component_id,kind,path_root) VALUES ('web','web','x'); INSERT INTO reg_audit_runs VALUES ('RUN-1','2026-10-05T00:00:00Z','$(printf 'a%.0s' $(seq 40))','p','t');
 INSERT INTO reg_evidence(atm_id,kind,evidence_class,path,sha256,size_bytes,produced_by,captured_at) VALUES ('$X','log','source','p','$(H f1)',1,'d','2026-10-05T00:00:00Z');
 INSERT INTO reg_findings(unit_alias,atm_id,run_id,component_id,location_path,category,severity,detector,fingerprint,evidence_id) VALUES ('F-web-001','$X','RUN-1','web','a.go','bug','low','det','$(H fp)',(SELECT max(evidence_id) FROM reg_evidence));" >/dev/null
must_fail "B1 orphan finding (no item row) fails" "$D/b1.db" 'FAIL.*v_findings_without_item'
# B2 illegal logged edge (guard dropped, forged log row, guard recreated by re-applying the DDL)
good_db "$D/b2.db"; Y=$(sql "$D/b2.db" "select atm_id from items"); sql "$D/b2.db" "DROP TRIGGER reg_status_log_insert_guard; INSERT INTO reg_status_log(atm_id,from_status,to_status,ev_hwm,run_hwm,rev_hwm) VALUES ('$Y','Queued','Completed (→ Fixed.md)',0,0,0);" >/dev/null
"$REG_DIR/apply_ext.sh" --db "$D/b2.db" >/dev/null 2>&1
must_fail "B2 illegal logged status edge fails" "$D/b2.db" 'FAIL.*v_illegal_logged_edges'
# B3 dropped trigger
good_db "$D/b3.db"; sql "$D/b3.db" "DROP TRIGGER reg_ids_no_delete" >/dev/null
must_fail "B3 dropped trigger fails (v_gate_missing_objects)" "$D/b3.db" 'FAIL.*v_gate_missing_objects.*reg_ids_no_delete'
# B4 same-name view redefinition
good_db "$D/b4.db"; sql "$D/b4.db" "DROP VIEW v_cycle_start; CREATE VIEW v_cycle_start AS SELECT atm_id, 0 AS log_id, 0 AS ev_hwm, 0 AS run_hwm, 0 AS rev_hwm FROM reg_ids" >/dev/null
must_fail "B4 same-name view redefinition fails (schema differs from the reference)" "$D/b4.db" 'FAIL.*(schema|differs)'
# B5 seed drift: an edited edge graph
good_db "$D/b5.db"; sql "$D/b5.db" "DELETE FROM reg_status_transitions WHERE from_status='Queued' AND to_status='In progress'" >/dev/null
must_fail "B5 edited edge graph fails (seed differs)" "$D/b5.db" 'FAIL.*(seed|differs|schema)'
# B6 foreign key violation written with foreign keys OFF
good_db "$D/b6.db"; "$SQLITE3" "$D/b6.db" "PRAGMA foreign_keys=OFF; INSERT INTO reg_evidence(atm_id,kind,evidence_class,path,sha256,size_bytes,produced_by,captured_at) VALUES ('CAT-999','log','source','p','$(H f2)',1,'d','2026-10-05T00:00:00Z');"
must_fail "B6 foreign-key violation fails (foreign_key_check)" "$D/b6.db" 'FAIL.*foreign_key_check'
# B7 item without a mint (trigger dropped, re-applied)
good_db "$D/b7.db"; sql "$D/b7.db" "DROP TRIGGER trg_items_require_mint" >/dev/null
"$WI" add Task Low --db "$D/b7.db" --id CAT-900 --prefix CAT --title "t" --description "$DESC" >/dev/null 2>&1
"$REG_DIR/apply_ext.sh" --db "$D/b7.db" >/dev/null 2>&1
must_fail "B7 item without a mint fails (v_items_without_mint)" "$D/b7.db" 'FAIL.*v_items_without_mint'
# B8 a view_empty registry row removed (the registry is data: edited seed)
good_db "$D/b8.db"; sql "$D/b8.db" "DELETE FROM reg_gate_checks WHERE name='v_custody_violations'" >/dev/null
must_fail "B8 edited gate registry fails (seed differs)" "$D/b8.db" 'FAIL.*(seed|differs|schema)'
# B9 not a database
echo "this is not a sqlite database" > "$D/b9.db"
must_fail "B9 a file that is not a database fails" "$D/b9.db" 'FAIL'
# B10 page corruption: overwrite bytes in the middle of a populated DB
good_db "$D/b10.db"; sql "$D/b10.db" "CREATE TABLE junk(a); WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i<3000) INSERT INTO junk SELECT randomblob(100) FROM n" >/dev/null
sz=$(stat -c %s "$D/b10.db"); dd if=/dev/urandom of="$D/b10.db" bs=1 count=3000 seek=$((sz-60000)) conv=notrunc 2>/dev/null
must_fail "B10 page corruption in a data page fails (integrity_check)" "$D/b10.db" "FAIL integrity_check"
# B11 a gate step run only for its view: a populated closed item without a decision chain
good_db "$D/b11.db"; sql "$D/b11.db" "DROP TRIGGER trg_items_transition; DROP TRIGGER trg_items_status_log" >/dev/null
Z=$(sql "$D/b11.db" "select atm_id from items"); sql "$D/b11.db" "UPDATE items SET status='Completed (→ Fixed.md)' WHERE atm_id='$Z'" >/dev/null
"$REG_DIR/apply_ext.sh" --db "$D/b11.db" >/dev/null 2>&1
must_fail "B11 terminal item with no custody chain fails (v_custody_violations)" "$D/b11.db" 'FAIL.*v_custody_violations'
# ---- untrusted-name fixtures (coordinator fix, automated security review): reg_gate_checks of the DB under
# test is DATA. Each fixture would make an unguarded gate PASS (the injected name selects zero rows).
n_ref=$(sql "$D/ok0.db" "select count(*) from reg_gate_checks where kind='view_empty'")
good_db "$D/b12.db"; sql "$D/b12.db" "INSERT INTO reg_gate_checks VALUES ('v_open_items WHERE 0','view_empty')" >/dev/null
must_fail "B12 injected registry name 'v_open_items WHERE 0' fails closed (invalid registry name)" "$D/b12.db" 'FAIL.*invalid registry name'
good_db "$D/b13.db"; sql "$D/b13.db" "INSERT INTO reg_gate_checks VALUES ('v_x; DROP TABLE reg_meta','view_empty')" >/dev/null
must_fail "B13 injected registry name with a statement separator fails closed" "$D/b13.db" 'FAIL.*invalid registry name'
assert_eq "B13b the injected DROP did not run (reg_meta exists)" "$(sql "$D/b13.db" "select count(*) from sqlite_master where name='reg_meta'")" 1
good_db "$D/b14.db"; sql "$D/b14.db" "DROP VIEW v_duplicate_item_ids; CREATE TABLE v_duplicate_item_ids(a)" >/dev/null
must_fail "B14 a registered name that is a TABLE, not a view, fails (not a view)" "$D/b14.db" 'FAIL.*v_duplicate_item_ids is not a view'
good_db "$D/b15.db"; sql "$D/b15.db" "INSERT INTO reg_gate_checks VALUES ('sqlite_master','view_empty')" >/dev/null
must_fail "B15 registry row naming sqlite_master fails closed" "$D/b15.db" 'FAIL.*invalid registry name'
good_db "$D/b16.db"; sql "$D/b16.db" "DELETE FROM reg_gate_checks WHERE name='v_custody_violations'" >/dev/null
out=$(run_gate "$D/b16.db"); printf '%s' "$out" | grep -q "INFO view_empty checked=$n_ref\$" && ok "B16 the view list comes from the reference: all $n_ref views checked though the DB registry lost a row" || bad "B16 [$(printf '%s' "$out" | grep INFO)] wanted checked=$n_ref"
before=$(sha256sum "$D/b12.db" | cut -d' ' -f1); run_gate "$D/b12.db" >/dev/null; assert_eq "B17 tampered DB left unmodified by the gate" "$(sha256sum "$D/b12.db" | cut -d' ' -f1)" "$before"
# B18 a hostile name in the TRUSTED reference registry itself (DDL under test): refused by the name guard
sed "s/('v_replayed_evidence','view_empty')/('v_replayed_evidence','view_empty'),('v_x; select 0','view_empty')/" "${REG_EXT_SQL:-$REG_DIR/register_ext.sql}" > "$D/hostile.sql"
out=$(REG_ROOT=$ROOT bash "$GATE" --db "$D/ok0.db" --ext "$D/hostile.sql" 2>&1); rc=$?
[ $rc -ne 0 ] && printf '%s' "$out" | grep -q 'FAIL invalid view name in the reference registry' && ok "B18 hostile name in the reference registry fails the gate" || bad "B18 rc=$rc [$(printf '%s' "$out" | head -3 | tr '\n' '|')]"
# B19 an invariant only the engine's validate checks (description floor): the engine step must carry it
good_db "$D/b19.db"; sql "$D/b19.db" "UPDATE items SET description='x'" >/dev/null
must_fail "B19 engine-only violation fails (engine validate step)" "$D/b19.db" 'FAIL engine validate'

echo "== WF2 fix round 3: fail-open gaps (I-1 I-2 I-3 I-8), minors m-1 m-2 m-3 m-4 =="
OBJ_ATTACK() { :; }
# m-1: an option without its value must exit 2, never loop
out=$(REG_ROOT=$ROOT timeout 10 bash "$GATE" --db 2>&1); rc=$?; [ $rc -eq 2 ] && ok "G7 --db without a value exits 2 (no hang)" || bad "G7 rc=$rc"
out=$(REG_ROOT=$ROOT timeout 10 bash "$GATE" --db "$D/ok0.db" --ext 2>&1); rc=$?; [ $rc -eq 2 ] && ok "G7b --ext without a value exits 2 (no hang)" || bad "G7b rc=$rc"
# m-2: a WAL register is not modified (db bytes and -wal bytes), the -wal file survives
good_db "$D/wal.db"; "$SQLITE3" "$D/wal.db" "PRAGMA journal_mode=WAL;" >/dev/null
python3 - "$D/wal.db" <<'PY'
import sqlite3, sys, os
c = sqlite3.connect(sys.argv[1], isolation_level=None); c.execute("PRAGMA wal_autocheckpoint=0")
c.execute("INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('wal','manual')"); os._exit(0)   # no close: the -wal keeps the row
PY
[ -s "$D/wal.db-wal" ] && ok "G8.0 fixture: an uncheckpointed -wal exists" || bad "G8.0 no -wal fixture"
h1=$(sha256sum "$D/wal.db" | cut -d' ' -f1); w1=$(sha256sum "$D/wal.db-wal" 2>/dev/null | cut -d' ' -f1)
out=$(run_gate "$D/wal.db"); rc=$?
h2=$(sha256sum "$D/wal.db" | cut -d' ' -f1); w2=$(sha256sum "$D/wal.db-wal" 2>/dev/null | cut -d' ' -f1)
assert_eq "G8 the gate leaves a WAL database file byte-identical" "$h2" "$h1"
assert_eq "G8b the gate leaves the -wal file byte-identical and present" "$w2" "$w1"
# I-1: an object of ANY name outside the gate's old glob list is an unexpected object
good_db "$D/x1.db"; sql "$D/x1.db" "CREATE TRIGGER audit_silent BEFORE INSERT ON reg_status_log BEGIN SELECT RAISE(IGNORE); END;" >/dev/null
must_fail "B20 an added trigger named outside the old globs fails (unexpected object)" "$D/x1.db" 'FAIL unexpected schema object.*audit_silent'
good_db "$D/x2.db"; sql "$D/x2.db" "CREATE VIEW audit_view AS SELECT 1 AS a" >/dev/null
must_fail "B21 an added view of an unrelated name fails" "$D/x2.db" 'FAIL unexpected schema object.*audit_view'
good_db "$D/x3.db"; sql "$D/x3.db" "CREATE INDEX audit_idx ON reg_evidence(path)" >/dev/null
must_fail "B22 an added index of an unrelated name fails" "$D/x3.db" 'FAIL unexpected schema object.*audit_idx'
good_db "$D/x4.db"; sql "$D/x4.db" "CREATE TABLE audit_tbl(a)" >/dev/null
must_fail "B23 an added table of an unrelated name fails" "$D/x4.db" 'FAIL unexpected schema object.*audit_tbl'
# WF3 I1: LIKE treats _ as a wildcard, so an object named sqliteX... was invisible to every comparison
good_db "$D/x20.db"; sql "$D/x20.db" "CREATE TRIGGER sqliteXswallow BEFORE INSERT ON reg_status_log BEGIN SELECT RAISE(IGNORE); END;" >/dev/null
must_fail "B35 an added trigger named sqliteX... (LIKE-wildcard boundary of the object filter) fails" "$D/x20.db" 'FAIL unexpected schema object.*sqliteXswallow'
good_db "$D/x21.db"; sql "$D/x21.db" "CREATE VIEW sqlite1view AS SELECT 1 AS a" >/dev/null
must_fail "B36 an added view named sqlite1... fails" "$D/x21.db" 'FAIL unexpected schema object.*sqlite1view'
good_db "$D/x22.db"; sql "$D/x22.db" "CREATE TABLE sqlitezz(a)" >/dev/null
must_fail "B37 an added table named sqlitezz (no underscore) fails" "$D/x22.db" 'FAIL unexpected schema object.*sqlitezz'
# WF5-1: SQLite refuses a reserved sqlite_ prefix (any case) only while writable_schema is off or the connection is
# defensive; a non-defensive connection (python sqlite3 default) creates such objects, and they persist and are
# active. The gate must never hide a trigger / view / index / table by prefix. Probed 2026-10-06: lowercase,
# UPPERCASE and Mixed-case sqlite_ names are all creatable that way; only SQLite's own sqlite_stat1..4 tables
# (created by ANALYZE) may be absent from the comparison.
wsx() {  # db "stmt" : run one statement on a non-defensive connection with writable_schema=ON
  python3 - "$1" "$2" <<'PY'
import sqlite3, sys
c = sqlite3.connect(sys.argv[1], isolation_level=None); c.execute("PRAGMA writable_schema=ON"); c.execute(sys.argv[2]); c.close()
PY
}
n=0
for spec in "trigger|sqlite_swallow|CREATE TRIGGER sqlite_swallow BEFORE INSERT ON reg_status_log BEGIN SELECT RAISE(IGNORE); END" \
            "trigger|SQLITE_SWALLOW|CREATE TRIGGER SQLITE_SWALLOW BEFORE INSERT ON reg_status_log BEGIN SELECT RAISE(IGNORE); END" \
            "trigger|Sqlite_Swallow|CREATE TRIGGER Sqlite_Swallow BEFORE INSERT ON reg_status_log BEGIN SELECT RAISE(IGNORE); END" \
            "view|sqlite_hidden_v|CREATE VIEW sqlite_hidden_v AS SELECT 1 AS a" \
            "view|SQLite_Hidden_V|CREATE VIEW SQLite_Hidden_V AS SELECT 1 AS a" \
            "index|sqlite_hidden_i|CREATE INDEX sqlite_hidden_i ON reg_evidence(path)" \
            "index|SQLITE_HIDDEN_I|CREATE INDEX SQLITE_HIDDEN_I ON reg_evidence(path)" \
            "table|sqlite_hidden_t|CREATE TABLE sqlite_hidden_t(a)" \
            "table|Sqlite_Hidden_T|CREATE TABLE Sqlite_Hidden_T(a)" \
            "trigger|sqlite_stat1|CREATE TRIGGER sqlite_stat1 BEFORE INSERT ON reg_status_log BEGIN SELECT RAISE(IGNORE); END" \
            "view|sqlite_stat2|CREATE VIEW sqlite_stat2 AS SELECT 1 AS a" \
            "index|sqlite_stat3|CREATE INDEX sqlite_stat3 ON reg_evidence(path)" \
            "table|sqlite_sequence_x|CREATE TABLE sqlite_sequence_x(a)" \
            "table|sqlite_stat9|CREATE TABLE sqlite_stat9(a)" \
            "table|sqlite_autoindex_fake|CREATE TABLE sqlite_autoindex_fake(a)" \
            "index|sqlite_autoindex_fake_1|CREATE INDEX sqlite_autoindex_fake_1 ON reg_evidence(path)"; do
  n=$((n+1)); IFS='|' read -r kind nm ddl <<<"$spec"; db="$D/ws$n.db"
  good_db "$db"; wsx "$db" "$ddl"
  [ "$(sql "$db" "SELECT count(*) FROM sqlite_master WHERE name='$nm'" | tail -1)" = 1 ] && ok "B40.$n fixture: $kind $nm exists in the database under test" || bad "B40.$n fixture $kind $nm not created"
  must_fail "B40.$n an added $kind named $nm (reserved-prefix name, writable_schema) fails, not hidden" "$db" "FAIL unexpected schema object.*$nm"
done
# the swallow trigger is also effective (WF6-3b): proven, not asserted. Control: an illegal status-log insert is REFUSED by the
# register's own guard. With the stat-named trigger present the SAME insert is silently swallowed (rc 0, no row, no error).
good_db "$D/ws_ctl.db"; YC=$(sql "$D/ws_ctl.db" "select atm_id from items" | tail -1)
SWI="INSERT INTO reg_status_log(atm_id,from_status,to_status,ev_hwm,run_hwm,rev_hwm) VALUES ('$YC','Queued','Completed (→ Fixed.md)',0,0,0)"
sql "$D/ws_ctl.db" "$SWI" >/dev/null 2>&1; rcc=$?
[ $rcc -ne 0 ] && ok "B41.0 control: without the trigger the illegal status-log insert is refused (rc $rcc)" || bad "B41.0 control insert was accepted"
good_db "$D/ws_eff.db"; wsx "$D/ws_eff.db" "CREATE TRIGGER sqlite_swallow BEFORE INSERT ON reg_status_log BEGIN SELECT RAISE(IGNORE); END"
[ "$(sql "$D/ws_eff.db" "SELECT count(*) FROM sqlite_master WHERE type='trigger' AND name='sqlite_swallow'" | tail -1)" = 1 ] && ok "B41 the sqlite_swallow trigger persists in the file (new connection sees it)" || bad "B41"
nb=$(sql "$D/ws_eff.db" "SELECT count(*) FROM reg_status_log" | tail -1); YE=$(sql "$D/ws_eff.db" "select atm_id from items" | tail -1)
sql "$D/ws_eff.db" "${SWI//$YC/$YE}" >/dev/null 2>&1; rce=$?; na=$(sql "$D/ws_eff.db" "SELECT count(*) FROM reg_status_log" | tail -1)
[ $rce -eq 0 ] && [ "$na" = "$nb" ] && ok "B41.1 the trigger is effective: the illegal insert is swallowed (rc 0, rows $nb -> $na), the guard never runs" || bad "B41.1 rc=$rce rows $nb -> $na"
# no false refusal: SQLite's own sqlite_stat1 (ANALYZE) and sqlite_sequence must not fail the gate
good_db "$D/an.db"; sql "$D/an.db" "ANALYZE" >/dev/null
[ "$(sql "$D/an.db" "SELECT count(*) FROM sqlite_master WHERE name='sqlite_stat1'" | tail -1)" = 1 ] && ok "G9.0 fixture: ANALYZE created sqlite_stat1" || bad "G9.0 no sqlite_stat1"
out=$(run_gate "$D/an.db"); rc=$?; [ $rc -eq 0 ] && printf '%s' "$out" | grep -q '^GATE OK' && ok "G9 a register after ANALYZE (sqlite_stat1 present) still prints GATE OK" || bad "G9 rc=$rc [$(printf '%s' "$out" | head -3 | tr '\n' '|')]"
# a forged sqlite_stat1 (wrong definition) is not SQLite's own: it must fail
good_db "$D/stf.db"; wsx "$D/stf.db" "CREATE TABLE sqlite_stat1(tbl,idx,stat,extra_forged)"
must_fail "B42 a forged sqlite_stat1 (definition differs from SQLite's own) fails" "$D/stf.db" 'FAIL unexpected schema object.*sqlite_stat1'
# WF6-1/WF6-2: SQLite's own statistics tables. SQLite writes the column lists in lowercase: the vendored 3.53.3 source
# (mattn/go-sqlite3 v1.14.48, sqlite3-binding.c:123799/123801) has "tbl,idx,stat" and "tbl,idx,neq,nlt,ndlt,sample", so
# the stored definition is CREATE TABLE sqlite_stat4(tbl,idx,neq,nlt,ndlt,sample). A real ANALYZE on a STAT4 build was
# captured by the WF6 reviewer (probe/stat4_run.out). sqlite_stat2/3 are legacy (not written by 3.53.3): their exact
# text is UNCONFIRMED offline; stat2 is the documented text, stat3 follows stat4's lowercase list.
# These golden-good tables are created by the author through writable_schema with SQLite's exact text (an author-written
# oracle for stat2/3; for stat4 the real-ANALYZE capture is the independent confirmation).
n=0
for spec in "sqlite_stat2|CREATE TABLE sqlite_stat2(tbl,idx,sampleno,sample)" \
            "sqlite_stat3|CREATE TABLE sqlite_stat3(tbl,idx,neq,nlt,ndlt,sample)" \
            "sqlite_stat4|CREATE TABLE sqlite_stat4(tbl,idx,neq,nlt,ndlt,sample)"; do
  n=$((n+1)); IFS='|' read -r nm ddl <<<"$spec"; db="$D/gs$n.db"; good_db "$db"; wsx "$db" "$ddl"
  [ "$(sql "$db" "SELECT sql FROM sqlite_master WHERE name='$nm'" | tail -1)" = "$ddl" ] && ok "G10.$n fixture: $nm exists with SQLite's exact definition" || bad "G10.$n fixture $nm"
  out=$(run_gate "$db"); rc=$?; [ $rc -eq 0 ] && printf '%s' "$out" | grep -q '^GATE OK' && ok "G10.$n a register holding SQLite's exact $nm still prints GATE OK (no false refusal)" || bad "G10.$n rc=$rc [$(printf '%s' "$out" | head -3 | tr '\n' '|')]"
done
# golden-bad: forged definitions under the same names (and a stat4 with a trailing extra column)
n=0
for spec in "sqlite_stat2|CREATE TABLE sqlite_stat2(a)" \
            "sqlite_stat3|CREATE TABLE sqlite_stat3(x,y)" \
            "sqlite_stat4|CREATE TABLE sqlite_stat4(a)" \
            "sqlite_stat4|CREATE TABLE sqlite_stat4(tbl,idx,neq,nlt,ndlt,sample,extra_forged_tail)"; do
  n=$((n+1)); IFS='|' read -r nm ddl <<<"$spec"; db="$D/gf$n.db"; good_db "$db"; wsx "$db" "$ddl"
  must_fail "B43.$n a forged $nm ($ddl) fails, not hidden" "$db" "FAIL unexpected schema object.*$nm"
done
# WF6-4(1): the refusal shows the differing definition tail untruncated (diagnosability, section 11.4.201(5))
must_fail "B43.5 the stat-mismatch refusal prints the full definition tail (not cut at 60 characters)" "$D/gf4.db" 'extra_forged_tail'
# WF7-4: the refusal must not strip quote characters, so a quoting-only forgery is visible in the printed definition
good_db "$D/gq1.db"; wsx "$D/gq1.db" 'CREATE TABLE sqlite_stat4(tbl,idx,neq,nlt,ndlt,"sample")'
must_fail "B43.6 a quoting-only forgery of sqlite_stat4 is refused and the printed definition shows the double quotes" "$D/gq1.db" 'ndlt,"sample"\)'
good_db "$D/gq2.db"; wsx "$D/gq2.db" "CREATE TABLE sqlite_stat4(tbl,idx,neq,nlt,ndlt,'sample')"
must_fail "B43.7 a single-quote forgery of sqlite_stat4 is refused and the printed definition shows the single quotes" "$D/gq2.db" "ndlt,'sample'\\)"
# I-8: weakened table definition and dropped index (the rest of step_reference_diff)
good_db "$D/x5.db"; sql "$D/x5.db" "PRAGMA foreign_keys=OFF; DROP TABLE reg_reviews; CREATE TABLE reg_reviews (review_id INTEGER PRIMARY KEY AUTOINCREMENT, atm_id TEXT NOT NULL REFERENCES reg_ids(atm_id), author TEXT NOT NULL, reviewer TEXT NOT NULL, model TEXT NOT NULL, effort TEXT NOT NULL, verdict TEXT NOT NULL CHECK (verdict IN ('GO','NO-GO')), evidence_id INTEGER NOT NULL REFERENCES reg_evidence(evidence_id), reviewed_at TEXT NOT NULL);" >/dev/null
"$REG_DIR/apply_ext.sh" --db "$D/x5.db" >/dev/null 2>&1   # the table drop removed its triggers; re-apply recreates them, the CHECK stays gone
must_fail "B24 reg_reviews rebuilt without the author != reviewer CHECK (triggers intact) fails" "$D/x5.db" 'FAIL schema differs'
good_db "$D/x6.db"; sql "$D/x6.db" "DROP INDEX uq_source_map_severity" >/dev/null
must_fail "B25 a dropped index fails (missing object)" "$D/x6.db" 'FAIL schema object of the reference missing.*uq_source_map_severity'
good_db "$D/x7.db"; sql "$D/x7.db" "DROP VIEW test_diary_summary" >/dev/null
must_fail "B26 a dropped ENGINE view fails (missing object)" "$D/x7.db" 'FAIL schema object of the reference missing.*test_diary_summary'
good_db "$D/x8.db"; sql "$D/x8.db" "DROP INDEX idx_findings_atm" >/dev/null
must_fail "B26b a dropped idx_ index fails" "$D/x8.db" 'FAIL schema object of the reference missing.*idx_findings_atm'
good_db "$D/x14.db"; sql "$D/x14.db" "DROP VIEW test_diary_summary; CREATE VIEW test_diary_summary AS SELECT 1 AS a" >/dev/null
must_fail "B26c a redefined ENGINE view (same name, other body) fails (schema differs)" "$D/x14.db" 'FAIL schema differs'
good_db "$D/x15.db"; sql "$D/x15.db" "DROP INDEX uq_source_map_severity; CREATE UNIQUE INDEX uq_source_map_severity ON reg_source_map(atm_id)" >/dev/null
must_fail "B26d an index redefined under the same name (WHERE clause dropped) fails (schema differs)" "$D/x15.db" 'FAIL schema differs'
# m-4: engine version and reg_meta
good_db "$D/x9.db"; sql "$D/x9.db" "UPDATE meta SET value='6' WHERE key='schema_version'" >/dev/null
must_fail "B27 engine schema_version other than engine_schema_required fails" "$D/x9.db" 'FAIL engine schema_version \[6\]|FAIL engine schema_version.*!= engine_schema_required'
good_db "$D/x10.db"; sql "$D/x10.db" "UPDATE reg_meta SET value='4' WHERE key='ext_schema_version'" >/dev/null
must_fail "B28 an edited reg_meta row fails (seed differs)" "$D/x10.db" 'FAIL seed differs'
# I-1 / I-2: the engine-recorded reopen the register log lacks, and a deleted item
good_db "$D/x11.db"; Q=$(sql "$D/x11.db" "select atm_id from items")
sql "$D/x11.db" "INSERT INTO item_history(atm_id,event_type,on_date) VALUES ('$Q','Reopened','2026-10-05')" >/dev/null
must_fail "B29 an engine Reopened event the status log lacks fails (v_reopen_unlogged)" "$D/x11.db" 'FAIL v_reopen_unlogged rows=1'
good_db "$D/x12.db"; Q=$(sql "$D/x12.db" "select atm_id from items")
sql "$D/x12.db" "DELETE FROM items WHERE atm_id='$Q'; DELETE FROM doc_segments WHERE atm_id='$Q'" >/dev/null
must_fail "B30 a raw DELETE of an item (and its doc_segments) fails (v_deleted_items)" "$D/x12.db" 'FAIL v_deleted_items rows=1'
# the full attack of WF2 X9 on a legacy-import item: silent trigger, engine reopen, drop the trigger again
good_db "$D/x13.db"; sql "$D/x13.db" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('t','import'); INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,legacy_status,reverify_required) VALUES ('CAT-002','bug','source','low','legacy_import','closed',1); INSERT INTO reg_sources(kind,locator,parser) VALUES ('md_tracker','legacy.md','p'); INSERT INTO reg_source_entries(source_id,locator,entry_sha256) VALUES (1,'e1','$(H e1)'); INSERT INTO reg_source_map(entry_id,atm_id,relation,match_basis,mapped_by) VALUES (1,'CAT-002','primary','import_1to1','t')" >/dev/null
"$WI" add Task Low --db "$D/x13.db" --id CAT-002 --prefix CAT --title leg --description "$DESC" >/dev/null 2>&1; echo e1 > "$D/ev13.txt"
"$WI" close CAT-002 --db "$D/x13.db" --status completed --evidence "$D/ev13.txt" >/dev/null 2>&1
assert_eq "X9.0 fixture: the legacy item is closed through the import-time exemption" "$(sql "$D/x13.db" "select status from items where atm_id='CAT-002'")" "Completed (→ Fixed.md)"
out=$(run_gate "$D/x13.db"); printf '%s' "$out" | grep -q '^GATE OK' && ok "X9.1 control: before the attack the register passes" || bad "X9.1 [$out]"
sql "$D/x13.db" "CREATE TRIGGER audit_silent BEFORE INSERT ON reg_status_log WHEN NEW.atm_id='CAT-002' BEGIN SELECT RAISE(IGNORE); END;" >/dev/null
"$WI" reopen --id CAT-002 --db "$D/x13.db" --why manual-testing-detected --who AI --when 2026-10-05T01:00:00Z --incident "$D/ev13.txt" >/dev/null 2>&1
must_fail "X9.2 with the silent trigger present the gate fails (unexpected object)" "$D/x13.db" 'FAIL unexpected schema object.*audit_silent'
sql "$D/x13.db" "DROP TRIGGER audit_silent" >/dev/null
must_fail "X9.3 after the silent trigger is dropped again the reopen it hid is still detected (v_reopen_unlogged)" "$D/x13.db" 'FAIL v_reopen_unlogged rows=1'
# I-3: evidence re-hash
EVF="$D/evfile.txt"; printf 'captured output\n' > "$EVF"; EH=$(sha256sum "$EVF" | cut -d' ' -f1); ES=$(stat -c %s "$EVF")
EVR="INSERT INTO reg_evidence(atm_id,kind,evidence_class,path,sha256,size_bytes,produced_by,captured_at)"
EVRF="INSERT INTO reg_evidence(atm_id,kind,evidence_class,path,sha256,size_bytes,target_fingerprint,polarity,exit_code,iterations,precondition_provenance,produced_by,captured_at)"
good_db "$D/e1.db"; E1=$(sql "$D/e1.db" "select atm_id from items")
sql "$D/e1.db" "$EVR VALUES ('$E1','log','source','$EVF','$EH',$ES,'dev','2026-10-05T00:00:00Z')" >/dev/null
out=$(run_gate "$D/e1.db"); printf '%s' "$out" | grep -q '^GATE OK' && printf '%s' "$out" | grep -q 'INFO evidence_rehash files=1' && ok "E1 an evidence row whose file re-hashes to its sha256 passes (control)" || bad "E1 [$out]"
good_db "$D/e2.db"; sql "$D/e2.db" "$EVR VALUES ('$E1','log','source','/nonexistent/forged.txt','$EH',$ES,'dev','2026-10-05T00:00:00Z')" >/dev/null
must_fail "E2 an evidence row citing a file that does not exist fails (WF2 X1)" "$D/e2.db" 'FAIL evidence 1: cited file absent'
good_db "$D/e3.db"; sql "$D/e3.db" "$EVR VALUES ('$E1','log','source','$EVF','$(printf 'f%.0s' $(seq 64))',$ES,'dev','2026-10-05T00:00:00Z')" >/dev/null
must_fail "E3 an evidence row whose recorded sha256 differs from the file fails" "$D/e3.db" 'FAIL evidence 1: sha256 mismatch'
good_db "$D/e4.db"; sql "$D/e4.db" "$EVR VALUES ('$E1','log','source','$EVF','$EH',$((ES+5)),'dev','2026-10-05T00:00:00Z')" >/dev/null
must_fail "E4 an evidence row whose recorded size differs from the file fails" "$D/e4.db" 'FAIL evidence 1: size mismatch'
good_db "$D/e5.db"; sql "$D/e5.db" "$EVR VALUES ('$E1','log','source','a'||char(9)||'b','$EH',$ES,'dev','2026-10-05T00:00:00Z')" >/dev/null
must_fail "E5 an evidence path holding a tab fails closed" "$D/e5.db" 'FAIL evidence_rehash: 1 evidence path'
# WF3 I2 (docs/04 section 7.1(a), K-11): the re-hash is the compensating control for producer = verifier, so a
# producer must not be able to opt out by choosing a URL-shaped path. Only a tracker receipt is not a file.
good_db "$D/e6.db"; sql "$D/e6.db" "$EVR VALUES ('$E1','tracker_receipt','source','https://example.invalid/r/1','$EH',$ES,'dev','2026-10-05T00:00:00Z')" >/dev/null
out=$(run_gate "$D/e6.db"); rc=$?
[ $rc -eq 3 ] && ! printf '%s' "$out" | grep -q '^GATE OK' && printf '%s' "$out" | grep -q '^GATE PASS-PARTIAL.*url_skipped=1.*refused' && printf '%s' "$out" | grep -q 'url_skipped=1' && ok "E6 a tracker_receipt URL is counted as skipped, the run is PASS-PARTIAL (rc 3), never GATE OK" || bad "E6 rc=$rc [$out]"
out=$(REG_ROOT=$ROOT bash "$GATE" --db "$D/e6.db" --allow-url-evidence 2>&1); rc=$?
[ $rc -eq 0 ] && ! printf '%s' "$out" | grep -q '^GATE OK' && printf '%s' "$out" | grep -q '^GATE PASS-PARTIAL.*url_skipped=1.*allowed' && ok "E6a with --allow-url-evidence the partial run exits 0 but still reads PASS-PARTIAL with the count, never GATE OK" || bad "E6a rc=$rc [$out]"
good_db "$D/e6b.db"; sql "$D/e6b.db" "$EVR VALUES ('$E1','log','source','forged://made/up','$EH',$ES,'dev','2026-10-05T00:00:00Z')" >/dev/null
must_fail "E6b a URL-shaped path on a custody-bearing kind (log) fails: only a tracker_receipt may be a non-file (WF3 I2)" "$D/e6b.db" 'FAIL evidence 1: kind log must cite a file'
good_db "$D/e6c.db"; sql "$D/e6c.db" "$EVRF VALUES ('$E1','red_run','source','forged://made/up','$EH',$ES,NULL,'RED',1,NULL,'observed','dev','2026-10-05T00:00:00Z')" >/dev/null
out=$(REG_ROOT=$ROOT bash "$GATE" --db "$D/e6c.db" --allow-url-evidence 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'FAIL evidence 1: kind red_run must cite a file' && ok "E6c the allow flag does not legalise a URL path on a red_run (rc 1)" || bad "E6c rc=$rc [$out]"
good_db "$D/e8.db"; mkdir -p "$D/co:lon"; printf 'x\n' > "$D/co:lon/f.txt"
sql "$D/e8.db" "$EVR VALUES ('$E1','log','source','$D/co:lon/f.txt','$(printf 'f%.0s' $(seq 64))',2,'dev','2026-10-05T00:00:00Z')" >/dev/null
must_fail "E8 a real file path that contains a colon is re-hashed, not skipped as URL-shaped (WF3 W5)" "$D/e8.db" 'FAIL evidence 1: sha256 mismatch'
RELDIR="$ROOT/specs/001-full-project-audit-remediation/evidence/wp06"; mkdir -p "$D/rel"; printf 'rel\n' > "$D/rel/f.txt"
good_db "$D/e7.db"; sql "$D/e7.db" "$EVR VALUES ('$E1','log','source','rel/f.txt','$(sha256sum "$D/rel/f.txt" | cut -d' ' -f1)',4,'dev','2026-10-05T00:00:00Z')" >/dev/null
out=$(WI="$WI" REG_ROOT="$D" bash "$GATE" --db "$D/e7.db" --ext "${REG_EXT_SQL:-$REG_DIR/register_ext.sql}" 2>&1); printf '%s' "$out" | grep -q '^GATE OK' && ok "E7 a relative evidence path resolves under the tree root (control)" || bad "E7 [$out]"
# m-3: a registry name carrying a newline cannot print a line of its own that reads GATE OK
good_db "$D/s1.db"; sql "$D/s1.db" "INSERT INTO reg_gate_checks VALUES ('zz'||char(10)||'GATE OK','view_empty')" >/dev/null
out=$(run_gate "$D/s1.db"); rc=$?
assert_eq "S1 a failing run never prints a bare GATE OK line even when database text tries to (rc 1)" "$(printf '%s\n' "$out" | grep -c '^GATE OK$')/$rc" "0/1"
good_db "$D/s2.db"; sql "$D/s2.db" "DROP TRIGGER reg_ids_no_update; INSERT INTO reg_gate_checks VALUES ('zz'||char(10)||'GATE OK','trigger_present')" >/dev/null
out=$(run_gate "$D/s2.db"); assert_eq "S2 the missing-objects report cannot spoof GATE OK either" "$(printf '%s\n' "$out" | grep -c '^GATE OK$')" 0
echo "== WF3 I4: golden-good boundary fixtures of the I-1/I-2 views (a gate that refuses a clean register is a FAIL-bluff, 11.4.201(1)) =="
# GF1 a legitimately reopened item: the engine's reopen is logged by the extension, so engine and register agree (kills W1: > turned into >=)
good_db "$D/gf1.db"; sql "$D/gf1.db" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('t','import'); INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,legacy_status,reverify_required) VALUES ('CAT-002','bug','source','low','legacy_import','closed',1); INSERT INTO reg_sources(kind,locator,parser) VALUES ('md_tracker','legacy.md','p'); INSERT INTO reg_source_entries(source_id,locator,entry_sha256) VALUES (1,'e1','$(H gf1)'); INSERT INTO reg_source_map(entry_id,atm_id,relation,match_basis,mapped_by) VALUES (1,'CAT-002','primary','import_1to1','t')" >/dev/null
"$WI" add Task Low --db "$D/gf1.db" --id CAT-002 --prefix CAT --title leg --description "$DESC" >/dev/null 2>&1; echo gf1 > "$D/evgf1.txt"
"$WI" close CAT-002 --db "$D/gf1.db" --status completed --evidence "$D/evgf1.txt" >/dev/null 2>&1
"$WI" reopen --id CAT-002 --db "$D/gf1.db" --why manual-testing-detected --who AI --when 2026-10-05T01:00:00Z --incident "$D/evgf1.txt" >/dev/null 2>&1
assert_eq "GF1.0 fixture: the engine recorded one Reopened event and the register logged it" "$(sql "$D/gf1.db" "select (select count(*) from item_history where atm_id='CAT-002' and event_type='Reopened')||'/'||(select count(*) from reg_status_log where atm_id='CAT-002' and to_status='Reopened')")" "1/1"
out=$(run_gate "$D/gf1.db"); printf '%s' "$out" | grep -q '^GATE OK' && ok "GF1 a legitimately reopened item (logged reopen == engine reopen) passes; v_reopen_unlogged is empty" || bad "GF1 [$out]"
# GF2 a minted id that has no item yet (the gap between mint and add, T064): not a deletion (kills W2: v_deleted_items reading reg_ids)
good_db "$D/gf2.db"; mint "$D/gf2.db" >/dev/null
out=$(run_gate "$D/gf2.db"); printf '%s' "$out" | grep -q '^GATE OK' && ok "GF2 a minted id with no item row yet passes; v_deleted_items is empty" || bad "GF2 [$out]"
# GF3 brownfield: engine reopen history that pre-dates the extension's first log row is not a violation (kills W3: the time window dropped)
good_db "$D/gf3.db"; Q3=$(sql "$D/gf3.db" "select atm_id from items")
sql "$D/gf3.db" "INSERT INTO item_history(atm_id,event_type,on_date,created_at) VALUES ('$Q3','Reopened','2020-01-01','2020-01-01 00:00:00')" >/dev/null
out=$(run_gate "$D/gf3.db"); printf '%s' "$out" | grep -q '^GATE OK' && ok "GF3 a brownfield reopen older than the id's first log row passes (the T069 go-live case)" || bad "GF3 [$out]"
# S3 the engine-printing path: PRAGMA integrity_check prints raw index names, a corrupted index named with a newline must not spoof GATE OK (kills output_not_flattened, WF3 m2)
good_db "$D/s3.db"
python3 - "$D/s3.db" <<'PYS3'
import sqlite3, sys
c = sqlite3.connect(sys.argv[1], isolation_level=None)   # python, not the sqlite3 shell: the shell is defensive and refuses to edit sqlite_master
c.executescript('CREATE TABLE zt(a,b); INSERT INTO zt VALUES (1,2),(3,4); CREATE INDEX "ix\nGATE OK" ON zt(a);')
c.execute('PRAGMA writable_schema=ON')
c.execute("UPDATE sqlite_master SET sql=replace(sql,'ON zt(a)','ON zt(b)') WHERE type='index' AND name LIKE 'ix%'")
c.close()
PYS3
out=$(run_gate "$D/s3.db"); rc=$?
assert_eq "S3.0 fixture: integrity_check reports the corrupted newline-named index" "$("$SQLITE3" -readonly "$D/s3.db" 'PRAGMA integrity_check' 2>&1 | grep -c 'missing from index')" "2"
assert_eq "S3 a corrupted index named ix<LF>GATE OK never prints a bare GATE OK line (rc 1)" "$(printf '%s\n' "$out" | grep -c '^GATE OK')/$rc" "0/1"
echo "== view_empty views each have a golden-bad fixture =="
EVRF="INSERT INTO reg_evidence(atm_id,kind,evidence_class,path,sha256,size_bytes,target_fingerprint,polarity,exit_code,iterations,precondition_provenance,produced_by,captured_at)"
good_db "$D/v1.db"; sql "$D/v1.db" "INSERT INTO reg_sources(kind,locator,parser) VALUES ('md_tracker','t.md','p'); INSERT INTO reg_source_entries(source_id,locator,entry_sha256) VALUES (1,'e1','$(H ve1)')" >/dev/null
must_fail "B31 a scanned source entry with no mapping fails (v_unmapped_entries)" "$D/v1.db" 'FAIL v_unmapped_entries rows=1'
good_db "$D/v2.db"; sql "$D/v2.db" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('t','import'); INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,legacy_status,reverify_required) VALUES ('CAT-002','bug','source','low','legacy_import','closed',1)" >/dev/null
must_fail "B32 a legacy_import row with no primary source mapping fails (v_legacy_import_unbacked)" "$D/v2.db" 'FAIL v_legacy_import_unbacked rows=1'
good_db "$D/v3.db"; sql "$D/v3.db" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('t','import'); INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,legacy_status,reverify_required) VALUES ('CAT-002','bug','source','low','legacy_import','closed',1); INSERT INTO reg_sources(kind,locator,parser) VALUES ('md_tracker','legacy.md','p'); INSERT INTO reg_source_entries(source_id,locator,entry_sha256) VALUES (1,'e1','$(H ve3)'); INSERT INTO reg_source_map(entry_id,atm_id,relation,match_basis,mapped_by) VALUES (1,'CAT-002','primary','import_1to1','t')" >/dev/null
"$WI" add Task Low --db "$D/v3.db" --id CAT-002 --prefix CAT --title leg --description "$DESC" >/dev/null 2>&1; echo e1 > "$D/ev3.txt"; "$WI" close CAT-002 --db "$D/v3.db" --status completed --evidence "$D/ev3.txt" >/dev/null 2>&1
H3=$(sql "$D/v3.db" "select max(log_id) from reg_status_log where atm_id='CAT-002'")
sql "$D/v3.db" "INSERT INTO reg_recurrence_links(head_atm_id,reported_entry_id,intake_path,verdict,match_basis,new_atm_id,head_log_id,decided_by) VALUES ('CAT-002',1,'import','SAME_DEFECT','ticket',NULL,$H3,'t')" >/dev/null
must_fail "B33 a SAME_DEFECT link on a terminal head that was never reopened fails (v_recurrence_violations)" "$D/v3.db" 'FAIL v_recurrence_violations rows=1'
good_db "$D/v4.db"; sql "$D/v4.db" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('t','import'); INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,legacy_status,reverify_required) VALUES ('CAT-002','bug','source','low','legacy_import','closed',1); INSERT INTO reg_sources(kind,locator,parser) VALUES ('md_tracker','legacy.md','p'); INSERT INTO reg_source_entries(source_id,locator,entry_sha256) VALUES (1,'e1','$(H ve4)'); INSERT INTO reg_source_map(entry_id,atm_id,relation,match_basis,mapped_by) VALUES (1,'CAT-002','primary','import_1to1','t')" >/dev/null
"$WI" add Task Low --db "$D/v4.db" --id CAT-002 --prefix CAT --title leg --description "$DESC" >/dev/null 2>&1; echo e1 > "$D/ev4.txt"; "$WI" close CAT-002 --db "$D/v4.db" --status completed --evidence "$D/ev4.txt" >/dev/null 2>&1
sql "$D/v4.db" "$EVRF VALUES ('CAT-002','red_run','source','$EVF','$EH',$ES,NULL,'RED',1,NULL,'observed','dev','2026-10-05T00:00:00Z')" >/dev/null
"$WI" reopen --id CAT-002 --db "$D/v4.db" --why manual-testing-detected --who AI --when 2026-10-05T01:00:00Z --incident "$D/ev4.txt" >/dev/null 2>&1
sql "$D/v4.db" "DROP TRIGGER reg_evidence_no_replay; $EVRF VALUES ('CAT-002','red_run','source','$EVF','$EH',$ES,NULL,'RED',1,NULL,'observed','dev','2026-10-05T00:00:00Z')" >/dev/null
"$REG_DIR/apply_ext.sh" --db "$D/v4.db" >/dev/null 2>&1   # the replay trigger is back; the replayed row it would have refused stays
must_fail "B34 a custody-bearing evidence file recorded again after the reopen fails (v_replayed_evidence)" "$D/v4.db" 'FAIL v_replayed_evidence rows=1'
finish
