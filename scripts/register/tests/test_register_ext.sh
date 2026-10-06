#!/usr/bin/env bash
# test_register_ext.sh - T060 (docs/04 §5, §14.3, §14.9, §14.10): the register extension DDL.
# Real SQLite + the real engine binary; scratch DBs only. RED before apply_ext.sh/register_ext.sql exist.
# Usage: bash scripts/register/tests/test_register_ext.sh   (rc 0 = all checks pass)
. "$(dirname "$0")/lib.sh"
ident_header T060
H() { printf '%s' "$1" | sha256sum | cut -d' ' -f1; }
D="$T_SCR/d"; mkdir -p "$D"
DESC="scratch item for the register extension tests, long enough for the engine description floor"

echo "== A. apply_ext.sh on a fresh DB, idempotence, counts =="
DB="$D/a.db"; rm -f "$DB"
out=$("$REG_DIR/apply_ext.sh" --db "$DB" 2>&1); rc=$?
assert_eq "A1 apply on absent DB exits 0" "$rc" 0
[ $rc -ne 0 ] && { echo "$out"; finish; exit 1; }
assert_eq "A2 25 reg_ tables" "$(sql "$DB" "select count(*) from sqlite_master where type='table' and name like 'reg_%'")" 25
assert_eq "A3 43 triggers (v5: 41 + reg_findings_no_delete + reg_findings_observation_no_update)" "$(sql "$DB" "select count(*) from sqlite_master where type='trigger'")" 43
assert_eq "A4 ext_schema_version (v5)" "$(sql "$DB" "select value from reg_meta where key='ext_schema_version'")" 5
assert_eq "A5 engine schema_version still 7" "$(sql "$DB" "select value from meta where key='schema_version'")" 7
"$REG_DIR/apply_ext.sh" --db "$DB" >/dev/null 2>&1; assert_eq "A6 second apply exits 0 (idempotent)" "$?" 0
assert_eq "A7 trigger count unchanged after re-apply" "$(sql "$DB" "select count(*) from sqlite_master where type='trigger'")" 43

echo "== B. DB already holding items =="
DB="$D/b.db"; rm -f "$DB"; "$WI" validate --db "$DB" >/dev/null 2>&1
"$WI" add Task Low --db "$DB" --id CAT-001 --prefix CAT --title "pre-existing" --description "$DESC" >/dev/null 2>&1
assert_eq "B1 engine accepted --id CAT-001 --prefix CAT" "$(sql "$DB" "select count(*) from items where atm_id='CAT-001'")" 1
"$REG_DIR/apply_ext.sh" --db "$DB" >/dev/null 2>&1; assert_eq "B2 apply on DB with items exits 0" "$?" 0
assert_eq "B3 item kept" "$(sql "$DB" "select count(*) from items where atm_id='CAT-001'")" 1
expect_reject "$DB" "B4 raw items insert without a mint refused" "no reg_ids row" \
  "INSERT INTO items SELECT 'CAT-777', current_location, representation, $(sql "$DB" "select group_concat(name) from pragma_table_info('items') where name not in ('atm_id','current_location','representation')") FROM items WHERE atm_id='CAT-001'"

echo "== C. engine version mismatch refused =="
DB="$D/c.db"; rm -f "$DB"; "$WI" validate --db "$DB" >/dev/null 2>&1
sql "$DB" "UPDATE meta SET value='6' WHERE key='schema_version'" >/dev/null
out=$("$REG_DIR/apply_ext.sh" --db "$DB" 2>&1); rc=$?
[ $rc -ne 0 ] && ok "C1 mismatching engine version exits non-zero" || bad "C1 rc=$rc"
printf '%s' "$out" | grep -q '6' && printf '%s' "$out" | grep -q '7' && ok "C2 message names both versions (6 and 7)" || bad "C2 [$out]"
assert_eq "C3 nothing applied" "$(sql "$DB" "select count(*) from sqlite_master where name like 'reg_%'")" 0
"$REG_DIR/apply_ext.sh" >/dev/null 2>&1; [ $? -ne 0 ] && ok "C4 missing --db refused" || bad "C4"

echo "== D. CHECK constraints: rejected inserts incl. NULL inputs (§14.3) =="
DB="$D/d.db"; fresh_ext_db "$DB" || { bad "D0 fresh db"; finish; exit 1; }
A=$(mint "$DB"); B=$(mint "$DB")
EV="INSERT INTO reg_evidence(atm_id,kind,evidence_class,path,sha256,size_bytes,target_fingerprint,polarity,exit_code,iterations,precondition_provenance,produced_by,captured_at)"
S=$(H s0); NOW="'2026-10-05T00:00:00Z'"
CK="CHECK constraint failed"
expect_reject "$DB" "D1 red_run exit_code 0" "$CK" "$EV VALUES ('$A','red_run','source','p','$S',1,NULL,'RED',0,NULL,'observed','dev',$NOW)"
expect_reject "$DB" "D2 red_run exit_code NULL" "$CK" "$EV VALUES ('$A','red_run','source','p','$S',1,NULL,'RED',NULL,NULL,'observed','dev',$NOW)"
expect_reject "$DB" "D3 red_run exit_code 126" "$CK" "$EV VALUES ('$A','red_run','source','p','$S',1,NULL,'RED',126,NULL,'observed','dev',$NOW)"
expect_reject "$DB" "D4 red_run polarity NULL" "$CK" "$EV VALUES ('$A','red_run','source','p','$S',1,NULL,NULL,1,NULL,'observed','dev',$NOW)"
expect_reject "$DB" "D5 green_run iterations 1" "$CK" "$EV VALUES ('$A','green_run','source','p','$S',1,NULL,'GREEN',0,1,NULL,'dev',$NOW)"
expect_reject "$DB" "D6 green_run iterations NULL" "$CK" "$EV VALUES ('$A','green_run','source','p','$S',1,NULL,'GREEN',0,NULL,NULL,'dev',$NOW)"
expect_reject "$DB" "D7 green_run exit_code NULL" "$CK" "$EV VALUES ('$A','green_run','source','p','$S',1,NULL,'GREEN',NULL,3,NULL,'dev',$NOW)"
expect_reject "$DB" "D8 green_run exit_code 1" "$CK" "$EV VALUES ('$A','green_run','source','p','$S',1,NULL,'GREEN',1,3,NULL,'dev',$NOW)"
expect_reject "$DB" "D9 runtime evidence without fingerprint (NULL)" "$CK" "$EV VALUES ('$A','log','runtime','p','$S',1,NULL,NULL,NULL,NULL,NULL,'dev',$NOW)"
expect_reject "$DB" "D10 runtime evidence with empty fingerprint" "$CK" "$EV VALUES ('$A','log','runtime','p','$S',1,'',NULL,NULL,NULL,NULL,'dev',$NOW)"
expect_reject "$DB" "D11 mutation_run exit_code 0" "$CK" "$EV VALUES ('$A','mutation_run','source','p','$S',1,NULL,NULL,0,NULL,NULL,'dev',$NOW)"
expect_reject "$DB" "D12 mutation_run exit_code NULL" "$CK" "$EV VALUES ('$A','mutation_run','source','p','$S',1,NULL,NULL,NULL,NULL,NULL,'dev',$NOW)"
expect_reject "$DB" "D13 polarity on a non run kind" "$CK" "$EV VALUES ('$A','log','source','p','$S',1,NULL,'RED',NULL,NULL,NULL,'dev',$NOW)"
expect_reject "$DB" "D14 sha256 uppercase" "$CK" "$EV VALUES ('$A','log','source','p','$(H x | tr a-f A-F)',1,NULL,NULL,NULL,NULL,NULL,'dev',$NOW)"
expect_reject "$DB" "D15 sha256 short" "$CK" "$EV VALUES ('$A','log','source','p','abc',1,NULL,NULL,NULL,NULL,NULL,'dev',$NOW)"
expect_reject "$DB" "D16 sha256 NULL" "NOT NULL" "$EV VALUES ('$A','log','source','p',NULL,1,NULL,NULL,NULL,NULL,NULL,'dev',$NOW)"
expect_reject "$DB" "D17 size_bytes 0" "$CK" "$EV VALUES ('$A','log','source','p','$S',0,NULL,NULL,NULL,NULL,NULL,'dev',$NOW)"
expect_reject "$DB" "D18 unknown kind" "$CK" "$EV VALUES ('$A','nope','source','p','$S',1,NULL,NULL,NULL,NULL,NULL,'dev',$NOW)"
expect_reject "$DB" "D19 evidence for an unminted id (FK)" "FOREIGN KEY" "$EV VALUES ('CAT-999','log','source','p','$S',1,NULL,NULL,NULL,NULL,NULL,'dev',$NOW)"
expect_ok "$DB" "D20 positive control: a valid red_run is accepted" "$EV VALUES ('$A','red_run','source','p','$(H red)',1,NULL,'RED',1,NULL,'observed','dev',$NOW)"
expect_reject "$DB" "D21 UPDATE reg_evidence" "append-only" "UPDATE reg_evidence SET path='x'"
expect_reject "$DB" "D22 DELETE reg_evidence" "append-only" "DELETE FROM reg_evidence"
expect_reject "$DB" "D23 INSERT OR REPLACE reg_evidence" "append-only" "INSERT OR REPLACE INTO reg_evidence(evidence_id,atm_id,kind,evidence_class,path,sha256,size_bytes,produced_by,captured_at) VALUES (1,'$A','log','source','p','$S',1,'dev',$NOW)"
expect_reject "$DB" "D24 UPDATE reg_ids" "append-only" "UPDATE reg_ids SET minted_by='z'"
expect_reject "$DB" "D25 DELETE reg_ids" "append-only" "DELETE FROM reg_ids"
expect_reject "$DB" "D26 reg_ids minted_by empty" "$CK" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('','manual')"
expect_reject "$DB" "D27 reg_ids unknown mint_basis" "$CK" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('t','guess')"
expect_reject "$DB" "D28 reg_ids mint_basis NULL" "NOT NULL" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('t',NULL)"
assert_eq "D29 minted ids follow the CAT-NNN form" "$A" "CAT-001"

# reg_item_ext / reg_components
expect_reject "$DB" "D30 reg_item_ext bad category" "$CK" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity) VALUES ('$A','nope','source','low')"
expect_reject "$DB" "D31 reg_item_ext severity NULL" "NOT NULL" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity) VALUES ('$A','bug','source',NULL)"
expect_reject "$DB" "D32 reg_item_ext defect_layer NULL" "NOT NULL" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity) VALUES ('$A','bug',NULL,'low')"
expect_reject "$DB" "D33 reg_components id with uppercase" "$CK" "INSERT INTO reg_components(component_id,kind,path_root) VALUES ('Web','web','x')"
expect_ok "$DB" "D34 positive control: reg_item_ext row accepted" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity) VALUES ('$A','bug','artifact','low')"
expect_reject "$DB" "D35 defect_layer lowered" "can only be raised" "UPDATE reg_item_ext SET defect_layer='source' WHERE atm_id='$A'"
expect_reject "$DB" "D36 reg_item_ext deleted" "never deleted" "DELETE FROM reg_item_ext WHERE atm_id='$A'"
expect_reject "$DB" "D37 custody_basis changed to legacy_import" "legacy_import" "UPDATE reg_item_ext SET custody_basis='legacy_import' WHERE atm_id='$A'"
expect_reject "$DB" "D38 reg_item_ext.atm_id changed" "immutable" "UPDATE reg_item_ext SET atm_id='$B' WHERE atm_id='$A'"
expect_reject "$DB" "D39 legacy_import for a non-import mint" "mint_basis=import" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,legacy_status) VALUES ('$B','bug','source','low','legacy_import','closed')"

# recurrence links
sql "$DB" "INSERT INTO reg_sources(kind,locator,parser) VALUES ('md_tracker','t.md','p'); INSERT INTO reg_source_entries(source_id,locator,entry_sha256) VALUES (1,'e1','$(H e1)');" >/dev/null
LK="INSERT INTO reg_recurrence_links(head_atm_id,reported_entry_id,intake_path,verdict,match_basis,new_atm_id,head_log_id,decided_by)"
expect_reject "$DB" "D40 SAME_DEFECT with a minted new id" "$CK" "$LK VALUES ('$A',1,'import','SAME_DEFECT','ticket','$B',0,'t')"
expect_reject "$DB" "D41 UNDECIDED without a new id (NULL)" "$CK" "$LK VALUES ('$A',1,'import','UNDECIDED','ticket',NULL,0,'t')"
expect_reject "$DB" "D42 new id equals the head" "$CK" "$LK VALUES ('$A',1,'import','DISTINCT','ticket','$A',0,'t')"
expect_reject "$DB" "D43 no reported entry and no finding" "$CK" "INSERT INTO reg_recurrence_links(head_atm_id,intake_path,verdict,match_basis,head_log_id,decided_by) VALUES ('$A','import','SAME_DEFECT','ticket',0,'t')"
expect_reject "$DB" "D44 head_log_id not the head's last log id" "head_log_id" "$LK VALUES ('$A',1,'import','SAME_DEFECT','ticket',NULL,99,'t')"
expect_ok "$DB" "D45 positive control: valid SAME_DEFECT link" "$LK VALUES ('$A',1,'import','SAME_DEFECT','ticket',NULL,0,'t')"
expect_reject "$DB" "D46 UPDATE reg_recurrence_links" "append-only" "UPDATE reg_recurrence_links SET decided_by='x'"
expect_reject "$DB" "D47 DELETE reg_recurrence_links" "append-only" "DELETE FROM reg_recurrence_links"

# tracker sync
sql "$DB" "INSERT INTO reg_trackers(tracker_id,kind) VALUES ('github','github')" >/dev/null
TK="INSERT INTO reg_tracker_sync_log(tracker_id,atm_id,status,skip_reason,missing_env_names,exit_code,remote_ref,evidence_id,item_revision,attempted_at)"
expect_reject "$DB" "D50 SYNCED without exit/ref/evidence" "$CK" "$TK VALUES ('github','$A','SYNCED',NULL,NULL,NULL,NULL,NULL,'r',$NOW)"
expect_reject "$DB" "D51 SYNCED exit 0 but no remote_ref" "$CK" "$TK VALUES ('github','$A','SYNCED',NULL,NULL,0,NULL,1,'r',$NOW)"
expect_reject "$DB" "D52 SYNCED exit 0 ref but no evidence" "$CK" "$TK VALUES ('github','$A','SYNCED',NULL,NULL,0,'#1',NULL,'r',$NOW)"
expect_reject "$DB" "D53 SYNCED with exit 1" "$CK" "$TK VALUES ('github','$A','SYNCED',NULL,NULL,1,'#1',1,'r',$NOW)"
expect_reject "$DB" "D54 SKIPPED without a reason" "$CK" "$TK VALUES ('github','$A','SKIPPED',NULL,NULL,NULL,NULL,NULL,'r',$NOW)"
expect_reject "$DB" "D55 a reason on a non-SKIPPED row" "$CK" "$TK VALUES ('github','$A','FAILED','credentials_absent',NULL,1,NULL,NULL,'r',$NOW)"
expect_reject "$DB" "D56 FAILED with exit 0" "$CK" "$TK VALUES ('github','$A','FAILED',NULL,NULL,0,NULL,NULL,'r',$NOW)"
expect_reject "$DB" "D57 FAILED with exit NULL" "$CK" "$TK VALUES ('github','$A','FAILED',NULL,NULL,NULL,NULL,NULL,'r',$NOW)"
expect_ok "$DB" "D58 positive control: SKIPPED credentials_absent accepted" "$TK VALUES ('github','$A','SKIPPED','credentials_absent','GH_TOKEN',NULL,NULL,NULL,'r',$NOW)"

# reg_test_runs / reg_reviews
sql "$DB" "$EV VALUES ('$A','log','source','p','$(H log1)',1,NULL,NULL,NULL,NULL,NULL,'dev',$NOW)" >/dev/null
TR="INSERT INTO reg_test_runs(atm_id,test_id,type_code,group_id,rep_index,polarity,verdict,blocked_reason,target_fingerprint,evidence_id,started_at)"
EVID=$(sql "$DB" "select max(evidence_id) from reg_evidence")
expect_reject "$DB" "D60 GREEN row that is BLOCKED" "$CK" "$TR VALUES ('$A','t','unit','g1',1,'GREEN','BLOCKED','service_unreachable','fp',$EVID,$NOW)"
expect_reject "$DB" "D61 BLOCKED without a reason" "$CK" "$TR VALUES ('$A','t','unit','g1',1,'RED','BLOCKED',NULL,'fp',$EVID,$NOW)"
expect_reject "$DB" "D62 PASS carrying a blocked_reason" "$CK" "$TR VALUES ('$A','t','unit','g1',1,'GREEN','PASS','service_unreachable','fp',$EVID,$NOW)"
expect_reject "$DB" "D63 RED row that PASSes" "$CK" "$TR VALUES ('$A','t','unit','g1',1,'RED','PASS',NULL,'fp',$EVID,$NOW)"
expect_reject "$DB" "D64 rep_index 0" "$CK" "$TR VALUES ('$A','t','unit','g1',0,'GREEN','PASS',NULL,'fp',$EVID,$NOW)"
expect_reject "$DB" "D65 empty target_fingerprint" "$CK" "$TR VALUES ('$A','t','unit','g1',1,'GREEN','PASS',NULL,'',$EVID,$NOW)"
expect_reject "$DB" "D66 unknown test type (FK)" "FOREIGN KEY" "$TR VALUES ('$A','t','nonsense','g1',1,'GREEN','PASS',NULL,'fp',$EVID,$NOW)"
expect_ok "$DB" "D67 positive control: GREEN PASS row accepted" "$TR VALUES ('$A','t','unit','g1',1,'GREEN','PASS',NULL,'fp',$EVID,$NOW)"
expect_reject "$DB" "D68 duplicate (group_id, rep_index)" "append-only|UNIQUE" "$TR VALUES ('$A','t','unit','g1',1,'GREEN','PASS',NULL,'fp',$EVID,$NOW)"
expect_reject "$DB" "D69 UPDATE reg_test_runs" "append-only" "UPDATE reg_test_runs SET test_id='x'"
RV="INSERT INTO reg_reviews(atm_id,author,reviewer,model,effort,verdict,evidence_id,reviewed_at)"
expect_reject "$DB" "D70 author equals reviewer" "$CK" "$RV VALUES ('$A','alice','alice','m','e','GO',$EVID,$NOW)"
expect_reject "$DB" "D71 author equals reviewer (case, space)" "$CK" "$RV VALUES ('$A','alice',' Alice','m','e','GO',$EVID,$NOW)"
expect_reject "$DB" "D72 verdict neither GO nor NO-GO" "$CK" "$RV VALUES ('$A','a','b','m','e','MAYBE',$EVID,$NOW)"
expect_reject "$DB" "D73 reviewer NULL" "NOT NULL" "$RV VALUES ('$A','a',NULL,'m','e','GO',$EVID,$NOW)"
expect_reject "$DB" "D74 status log forged by a raw insert" "reg_status_log" "INSERT INTO reg_status_log(atm_id,from_status,to_status) VALUES ('$A',NULL,'Queued')"
expect_reject "$DB" "D75 gate registry check kind unknown" "$CK" "INSERT INTO reg_gate_checks VALUES ('x','nope')"
expect_reject "$DB" "D76 reg_export_runs verdict unknown" "$CK" "INSERT INTO reg_export_runs(db_fingerprint,engine_version,started_at,verdict) VALUES ('f','v',$NOW,'MAYBE')"
expect_reject "$DB" "D77 reg_discovery recorded_by equals producer" "FOREIGN KEY|$CK" "INSERT INTO reg_discovery VALUES ('FND-0001','c','manual_qa','x',NULL,'a','a')"

echo "== E. custody chain through the engine: close needs an ACCEPTED, single-use decision =="
DB="$D/e.db"; fresh_ext_db "$DB" || { bad "E0"; finish; exit 1; }
X=$(mint "$DB"); Y=$(mint "$DB")
sql "$DB" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity) VALUES ('$X','bug','source','low'),('$Y','bug','source','low')" >/dev/null
for id in $X $Y; do
  "$WI" add Task Low --db "$DB" --id "$id" --prefix CAT --title "t $id" --description "$DESC" >/dev/null 2>&1
  for st in "In progress" "Ready for testing" "In testing"; do "$WI" update --id "$id" --db "$DB" --status "$st" >/dev/null 2>&1; done
done
assert_eq "E1 both items reached In testing" "$(sql "$DB" "select group_concat(status) from items order by atm_id")" "In testing,In testing"
echo e1 > "$D/ev.txt"
out=$("$WI" close "$X" --db "$DB" --status completed --evidence "$D/ev.txt" 2>&1); rc=$?
[ $rc -ne 0 ] && printf '%s' "$out" | grep -q 'custody' && ok "E2 close without a decision refused (custody)" || bad "E2 rc=$rc [$out]"
assert_eq "E3 item unchanged after the refused close" "$(sql "$DB" "select status||'/'||current_location from items where atm_id='$X'")" "In testing/Issues"
expect_reject "$DB" "E4 raw UPDATE to a terminal status refused" "custody" "UPDATE items SET status='Completed (→ Fixed.md)' WHERE atm_id='$Y'"
expect_reject "$DB" "E5 raw status edge not in the graph refused" "transition" "UPDATE items SET status='Queued' WHERE atm_id='$Y'"
expect_reject "$DB" "E6 DELETE+INSERT as terminal refused (bypass)" "custody|transition" "BEGIN; CREATE TEMP TABLE c AS SELECT * FROM items WHERE atm_id='$Y'; DELETE FROM items WHERE atm_id='$Y'; UPDATE c SET status='Completed (→ Fixed.md)', current_location='Fixed'; INSERT INTO items SELECT * FROM c;"
assert_eq "E7 item still present after the refused bypass" "$(sql "$DB" "select count(*) from items where atm_id='$Y'")" 1
expect_reject "$DB" "E8 hand-made ACCEPTED decision on a non-decision evidence row" "custody_decision|chain incomplete" \
  "$EV VALUES ('$X','log','source','p','$(H hl)',1,NULL,NULL,NULL,NULL,NULL,'dev',$NOW); INSERT INTO reg_closure_decisions(atm_id,to_status,decision,decision_json_evidence_id,decided_at) VALUES ('$X','Completed (→ Fixed.md)','ACCEPTED',(SELECT max(evidence_id) FROM reg_evidence),$NOW)"
# build the full chain for X: RED, GREEN x3 on another fingerprint, caught mutation, independent GO review, decision
e() { sql "$DB" "$EV VALUES ('$X','$1','source','$D/$1','$(H "$1$2")',10,${3:-NULL},${4:-NULL},${5:-NULL},${6:-NULL},${7:-NULL},'$8',$NOW); SELECT max(evidence_id) FROM reg_evidence" | tail -1; }
TRX="INSERT INTO reg_test_runs(atm_id,test_id,type_code,group_id,rep_index,polarity,verdict,target_fingerprint,evidence_id,started_at)"
r=$(e red_run 1 "'fpA'" "'RED'" 1 NULL "'observed'" dev);  sql "$DB" "$TRX VALUES ('$X','T1','unit','$X-red',1,'RED','FAIL','fpA',$r,$NOW)" >/dev/null
out=$("$WI" close "$X" --db "$DB" --status completed --evidence "$D/ev.txt" 2>&1); [ $? -ne 0 ] && ok "E9 RED alone: close still refused" || bad "E9"
for i in 1 2 3; do g=$(e green_run $i "'fpB'" "'GREEN'" 0 3 NULL dev); sql "$DB" "$TRX VALUES ('$X','T1','unit','$X-green',$i,'GREEN','PASS','fpB',$g,$NOW)" >/dev/null; done
m=$(e mutation_run 1 "'fpB'" NULL 1 NULL NULL dev); sql "$DB" "$TRX VALUES ('$X','T1','unit','$X-mut',1,'MUTATION','FAIL','fpB',$m,$NOW)" >/dev/null
rv=$(e review_verdict 1 NULL NULL NULL NULL NULL rev); sql "$DB" "$RV VALUES ('$X','dev','rev','opus','xhigh','GO',$rv,$NOW)" >/dev/null
expect_reject "$DB" "E10 ACCEPTED decision without a custody_decision evidence row" "custody_decision" "INSERT INTO reg_closure_decisions(atm_id,to_status,decision,decision_json_evidence_id,decided_at) VALUES ('$X','Completed (→ Fixed.md)','ACCEPTED',$rv,$NOW)"
cd_=$(e custody_decision 1 NULL NULL NULL NULL NULL dev)
expect_ok "$DB" "E11 ACCEPTED decision on a complete chain accepted" "INSERT INTO reg_closure_decisions(atm_id,to_status,decision,decision_json_evidence_id,decided_at) VALUES ('$X','Completed (→ Fixed.md)','ACCEPTED',$cd_,$NOW)"
out=$("$WI" close "$X" --db "$DB" --status completed --evidence "$D/ev.txt" 2>&1); rc=$?
[ $rc -eq 0 ] && ok "E12 close with an ACCEPTED decision succeeds" || bad "E12 rc=$rc [$out]"
assert_eq "E13 item is Fixed" "$(sql "$DB" "select status||'/'||current_location from items where atm_id='$X'")" "Completed (→ Fixed.md)/Fixed"
assert_eq "E14 decision consumed (single use)" "$(sql "$DB" "select consumed_at is not null from reg_closure_decisions where atm_id='$X'")" 1
expect_reject "$DB" "E15 a consumed decision cannot be revived" "consume|single" "UPDATE reg_closure_decisions SET consumed_at=NULL WHERE atm_id='$X'"
expect_reject "$DB" "E16 decision DELETE refused" "append-only" "DELETE FROM reg_closure_decisions"
assert_eq "E17 v_custody_violations empty" "$(sql "$DB" "select count(*) from v_custody_violations")" 0
assert_eq "E18 status log recorded the whole path" "$(sql "$DB" "select count(*) from reg_status_log where atm_id='$X'")" 5
expect_reject "$DB" "E19 raw edit of the closed status refused" "transition|custody" "UPDATE items SET status='Queued' WHERE atm_id='$X'"
# reopen then close again: the old chain must not count
"$WI" reopen --id "$X" --db "$DB" --why manual-testing-detected --who AI --when 2026-10-05T01:00:00Z --incident "$D/ev.txt" >/dev/null 2>&1
assert_eq "E20 reopen succeeded" "$(sql "$DB" "select status from items where atm_id='$X'")" "Reopened"
for st in "In progress" "Ready for testing" "In testing"; do "$WI" update --id "$X" --db "$DB" --status "$st" >/dev/null 2>&1; done
expect_reject "$DB" "E21 a new decision on cycle-1 evidence refused" "custody_decision|chain incomplete|replay" \
  "$EV VALUES ('$X','custody_decision','source','p','$(H custody_decision1)',10,NULL,NULL,NULL,NULL,NULL,'dev',$NOW)"
out=$("$WI" close "$X" --db "$DB" --status completed --evidence "$D/ev.txt" 2>&1); [ $? -ne 0 ] && ok "E22 second close without a new chain refused" || bad "E22"
expect_reject "$DB" "E23 replay of a cycle-1 evidence file refused" "replay" "$EV VALUES ('$X','red_run','source','p','$(H red_run1)',10,'fpA','RED',1,NULL,'observed','dev',$NOW)"
echo "== F. trigger set equals the DDL's own trigger names; the real register is refused =="
DB="$D/f.db"; fresh_ext_db "$DB"
EXT=${REG_EXT_SQL:-$REG_DIR/register_ext.sql}
sed -n 's/^CREATE TRIGGER IF NOT EXISTS \([A-Za-z0-9_]*\).*/\1/p' "$EXT" | sort > "$D/ddl_triggers.txt"
sql "$DB" "select name from sqlite_master where type='trigger' order by name" | sort > "$D/db_triggers.txt"
assert_eq "F1 trigger names in the DB equal the names in the DDL (diff empty)" "$(diff "$D/ddl_triggers.txt" "$D/db_triggers.txt" | wc -l)" 0
assert_eq "F2 43 trigger names in the DDL" "$(wc -l < "$D/ddl_triggers.txt")" 43
[ -n "${TRIGGERS_OUT:-}" ] && { { ident_header T061; echo "ddl_triggers=$(wc -l < "$D/ddl_triggers.txt") db_triggers=$(wc -l < "$D/db_triggers.txt") diff_lines=$(diff "$D/ddl_triggers.txt" "$D/db_triggers.txt" | wc -l)"; echo "--- DDL names"; cat "$D/ddl_triggers.txt"; echo "--- DB names"; cat "$D/db_triggers.txt"; echo "--- diff"; diff "$D/ddl_triggers.txt" "$D/db_triggers.txt"; } > "$TRIGGERS_OUT"; }
# m-5: neutralise a go-live environment; F4 compares existence and checksum before/after, so it stays valid after T069
had=0; [ -e "$ROOT/docs/workable_items.db" ] && had=1; sum0=$(sha256sum "$ROOT/docs/workable_items.db" 2>/dev/null | cut -d' ' -f1)
out=$(env -u REGISTER_GO_LIVE "$REG_DIR/apply_ext.sh" --db "$ROOT/docs/workable_items.db" 2>&1); rc=$?
assert_eq "F3 the real docs/workable_items.db is refused (rc 4)" "$rc" 4
has=0; [ -e "$ROOT/docs/workable_items.db" ] && has=1; sum1=$(sha256sum "$ROOT/docs/workable_items.db" 2>/dev/null | cut -d' ' -f1)
[ "$had/$sum0" = "$has/$sum1" ] && ok "F4 the refused call neither created nor changed the real DB file" || bad "F4 docs/workable_items.db changed ($had/$sum0 -> $has/$sum1)"
# WF6: a "file:" argument is a RELATIVE path (directory "file:") for any program that does not treat it as a URI. Every call below
# that passes a file: URI therefore runs with its working directory in the scratch tree, so a guard that failed to refuse
# can only write under the scratch dir and never leave directories "file:" / "FILE:" in the repository root.
CW="$D/cwd"; mkdir -p "$CW"; inCW() { ( cd "$CW" && "$@" ); }
# m-6: the real-DB guard also refuses a file: URI and a hardlink of the real file, and a path that needs shell quoting
out=$(inCW env -u REGISTER_GO_LIVE "$REG_DIR/apply_ext.sh" --db "file:$ROOT/docs/workable_items.db?mode=rwc" 2>&1); rc=$?
assert_eq "F5 a file: URI naming the real register is refused (rc 4)" "$rc" 4
out=$(timeout 10 "$REG_DIR/apply_ext.sh" --db 2>&1); rc=$?
assert_eq "F6 --db with no value exits 2 (usage), it does not hang (124 = hung)" "$rc" 2
SP="$D/a dir with spaces"; mkdir -p "$SP"; "$REG_DIR/apply_ext.sh" --db "$SP/x.db" >/dev/null 2>&1; assert_eq "F7 a DB path with spaces is applied" "$(sql "$SP/x.db" "select value from reg_meta where key='ext_schema_version'")" 5
cp "$EXT" "$SP/ddl with space.sql"; "$REG_DIR/apply_ext.sh" --db "$D/f2.db" --sql "$SP/ddl with space.sql" >/dev/null 2>&1; assert_eq "F8 a DDL path with spaces is applied" "$(sql "$D/f2.db" "select count(*) from sqlite_master where name='reg_meta'")" 1
QS="$D/dd\"q.sql"; cp "$EXT" "$QS"; out=$("$REG_DIR/apply_ext.sh" --db "$D/f3.db" --sql "$QS" 2>&1); rc=$?
[ $rc -eq 2 ] && ok "F9 a DDL path holding a double quote is refused (rc 2), not interpolated into .read" || bad "F9 rc=$rc [$out]"
RR="$D/realish.db"; "$WI" validate --db "$RR" >/dev/null 2>&1; ln "$RR" "$D/realish.hard.db"; ln -s "$RR" "$D/realish.sym.db"
for v in "$D/realish.db" "$D/realish.hard.db" "$D/realish.sym.db" "file:$RR?mode=rwc" "file://$RR"; do
  out=$(inCW env -u REGISTER_GO_LIVE REG_REAL_DB="$RR" "$REG_DIR/apply_ext.sh" --db "$v" 2>&1); rc=$?
  assert_eq "F10 the real register reached through [$(basename "${v%%\?*}")] is refused (rc 4)" "$rc" 4
done
# WF3 m3: SQLite decodes %XX in a URI filename, so the guard must not compare the raw string; every file: URI is refused
h0=$(sha256sum "$RR" | cut -d' ' -f1)
out=$(inCW env -u REGISTER_GO_LIVE REG_REAL_DB="$RR" "$REG_DIR/apply_ext.sh" --db "file:$D/re%61lish.db" 2>&1); rc=$?
assert_eq "F12 a percent-encoded file: URI that SQLite decodes to the real register is refused (rc 4)" "$rc" 4
assert_eq "F12b the protected file is byte-identical after the refused percent-encoded call" "$(sha256sum "$RR" | cut -d' ' -f1)" "$h0"
out=$(inCW env -u REGISTER_GO_LIVE REG_REAL_DB="$RR" "$REG_DIR/apply_ext.sh" --db "FILE:$D/re%61lish.db" 2>&1); rc=$?
assert_eq "F12c the scheme is matched case-insensitively (FILE:) and refused (rc 4)" "$rc" 4
out=$(inCW env -u REGISTER_GO_LIVE REG_REAL_DB="$RR" "$REG_DIR/apply_ext.sh" --db "file:$D/notreal2.db" 2>&1); rc=$?
assert_eq "F13 any file: URI is refused, even for a scratch path (rc 4): the guard does not try to out-parse SQLite" "$rc" 4
env -u REGISTER_GO_LIVE REG_REAL_DB="$RR" "$REG_DIR/apply_ext.sh" --db "$D/notreal.db" >/dev/null 2>&1; assert_eq "F11 control: a different scratch DB is applied (rc 0)" "$?" 0
# WF6 stray-directory proof. (1) control needle: with the file: guard REMOVED, the same call does create the stray path - under the scratch
# working directory (so the instrument can see a stray) and not in the repository root; (2) with the real guard nothing is created at all.
NG="$T_SCR/noguard"; mkdir -p "$NG/scripts/register"; cp "$REG_DIR/register_ext.sql" "$NG/scripts/register/"
grep -v '^case "\$DB" in \[Ff\]\[Ii\]\[Ll\]\[Ee\]:\*)' "$REG_DIR/apply_ext.sh" > "$NG/scripts/register/apply_ext.sh"; chmod +x "$NG/scripts/register/apply_ext.sh"
cmp -s "$REG_DIR/apply_ext.sh" "$NG/scripts/register/apply_ext.sh" && bad "F14.0 the guard-less copy is identical to apply_ext.sh (the needle is vacuous)" || ok "F14.0 the guard-less copy differs from apply_ext.sh (the file: guard line is gone)"
inCW env -u REGISTER_GO_LIVE WI="$WI" "$NG/scripts/register/apply_ext.sh" --db "file:$D/stray_needle.db" >/dev/null 2>&1
[ -e "$CW/file:$D/stray_needle.db" ] && ok "F14 control needle: without the guard the file: argument creates the stray path, under the scratch cwd" || bad "F14 the instrument cannot see a stray path"
rm -rf "$CW/file:" "$CW/FILE:"
inCW env -u REGISTER_GO_LIVE "$REG_DIR/apply_ext.sh" --db "file:$D/stray2.db" >/dev/null 2>&1; rcs=$?
[ "$rcs" = 4 ] && [ ! -e "$CW/file:" ] && ok "F15 with the real guard the file: argument is refused (rc 4) and creates nothing even under the scratch cwd" || bad "F15 rc=$rcs"
if [ ! -e "$ROOT/file:" ] && [ ! -e "$ROOT/FILE:" ]; then ok "F16 no stray 'file:' or 'FILE:' directory exists in the repository root after the file: URI tests"; else bad "F16 stray directory in the repository root: $(ls -d "$ROOT"/file: "$ROOT"/FILE: 2>/dev/null | tr '\n' ' ')"; fi
echo "== H. remaining CHECK constraints: one rejected insert per CHECK (value out of range, and NULL where a closed set applies) =="
DB="$D/h.db"; fresh_ext_db "$DB" || { bad "H0"; finish; exit 1; }
HA=$(mint "$DB"); T0="'2026-10-05T00:00:00Z'"; G40=$(printf 'a%.0s' $(seq 40))
sql "$DB" "INSERT INTO reg_components(component_id,kind,path_root) VALUES ('web','web','w');
 INSERT INTO reg_audit_runs VALUES ('RUN-1',$T0,'$G40','p','t');
 INSERT INTO reg_sources(kind,locator,parser) VALUES ('md_tracker','t.md','p');
 INSERT INTO reg_source_entries(source_id,locator,entry_sha256) VALUES (1,'e1','$(H e1)');
 INSERT INTO reg_trackers(tracker_id,kind) VALUES ('gh','github'); INSERT INTO reg_cycle VALUES ('c1',1);
 INSERT INTO reg_evidence(atm_id,kind,evidence_class,path,sha256,size_bytes,produced_by,captured_at) VALUES ('$HA','log','source','p','$(H h1)',1,'d',$T0);
 INSERT INTO reg_closure_decisions(atm_id,to_status,decision,decision_json_evidence_id,decided_at) VALUES ('$HA','Fixed (→ Fixed.md)','REFUSED',1,$T0);" >/dev/null
HEV=1; rej() { expect_reject "$DB" "H $1" "$CK" "$2"; }
EVC="INSERT INTO reg_evidence(atm_id,kind,evidence_class,path,sha256,size_bytes,polarity,iterations,precondition_provenance,produced_by,captured_at)"
rej "components.kind" "INSERT INTO reg_components(component_id,kind,path_root) VALUES ('c2','nope','x')"
rej "components.own_repo" "INSERT INTO reg_components(component_id,kind,path_root,own_repo) VALUES ('c2','web','x',2)"
IX="INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,reverify_required)"
rej "item_ext.defect_layer" "$IX VALUES ('$HA','bug','nope','low','machine_evidence',0)"
rej "item_ext.severity" "$IX VALUES ('$HA','bug','source','nope','machine_evidence',0)"
rej "item_ext.custody_basis" "$IX VALUES ('$HA','bug','source','low','nope',0)"
rej "item_ext.reverify_required" "$IX VALUES ('$HA','bug','source','low','machine_evidence',2)"
rej "sources.kind" "INSERT INTO reg_sources(kind,locator,parser) VALUES ('nope','l2','p')"
rej "source_entries.entry_sha256 length" "INSERT INTO reg_source_entries(source_id,locator,entry_sha256) VALUES (1,'e2','abc')"
SM="INSERT INTO reg_source_map(entry_id,atm_id,relation,match_basis,severity_governs,mapped_by)"
rej "source_map.relation" "$SM VALUES (1,'$HA','nope','ticket',0,'t')"
rej "source_map.match_basis" "$SM VALUES (1,'$HA','primary','nope',0,'t')"
rej "source_map.severity_governs" "$SM VALUES (1,'$HA','primary','ticket',2,'t')"
rej "audit_runs.run_id pattern" "INSERT INTO reg_audit_runs VALUES ('BAD-1',$T0,'$G40','p','t')"
rej "audit_runs.git_head length" "INSERT INTO reg_audit_runs VALUES ('RUN-2',$T0,'abc','p','t')"
FN="INSERT INTO reg_findings(unit_alias,atm_id,run_id,component_id,location_path,category,severity,detector,fingerprint,evidence_id)"
F64=$(H fpx)
rej "findings.location_path empty" "$FN VALUES ('F-web-001','$HA','RUN-1','web','','bug','low','d','$F64',$HEV)"
rej "findings.category" "$FN VALUES ('F-web-001','$HA','RUN-1','web','a','nope','low','d','$F64',$HEV)"
rej "findings.severity" "$FN VALUES ('F-web-001','$HA','RUN-1','web','a','bug','nope','d','$F64',$HEV)"
rej "findings.fingerprint length" "$FN VALUES ('F-web-001','$HA','RUN-1','web','a','bug','low','d','abc',$HEV)"
rej "findings.unit_alias for another component" "$FN VALUES ('F-api-001','$HA','RUN-1','web','a','bug','low','d','$F64',$HEV)"
rej "findings.unit_alias non-digit suffix" "$FN VALUES ('F-web-001x','$HA','RUN-1','web','a','bug','low','d','$F64',$HEV)"
rej "evidence.evidence_class" "$EVC VALUES ('$HA','log','nope','p','$(H h2)',1,NULL,NULL,NULL,'d',$T0)"
rej "evidence.path empty" "$EVC VALUES ('$HA','log','source','','$(H h2)',1,NULL,NULL,NULL,'d',$T0)"
rej "evidence.polarity value" "$EVC VALUES ('$HA','red_run','source','p','$(H h2)',1,'MAYBE',NULL,'observed','d',$T0)"
rej "evidence.iterations 0" "$EVC VALUES ('$HA','log','source','p','$(H h2)',1,NULL,0,NULL,'d',$T0)"
rej "evidence.precondition_provenance" "$EVC VALUES ('$HA','log','source','p','$(H h2)',1,NULL,NULL,'nope','d',$T0)"
TH="INSERT INTO reg_test_runs(atm_id,test_id,type_code,group_id,rep_index,polarity,verdict,blocked_reason,target_fingerprint,evidence_id,started_at)"
rej "test_runs.polarity" "$TH VALUES ('$HA','t','unit','hg',1,'NOPE','PASS',NULL,'fp',$HEV,$T0)"
rej "test_runs.verdict" "$TH VALUES ('$HA','t','unit','hg',1,'GREEN','NOPE',NULL,'fp',$HEV,$T0)"
rej "test_runs.verdict (MUTATION row, the only path no other CHECK covers)" "$TH VALUES ('$HA','t','unit','hg',1,'MUTATION','NOPE',NULL,'fp',$HEV,$T0)"
rej "test_runs.blocked_reason" "$TH VALUES ('$HA','t','unit','hg',1,'RED','BLOCKED','nope','fp',$HEV,$T0)"
RL="INSERT INTO reg_recurrence_links(head_atm_id,reported_entry_id,intake_path,verdict,match_basis,new_atm_id,reopened,head_log_id,decided_by)"
rej "recurrence.intake_path" "$RL VALUES ('$HA',1,'nope','SAME_DEFECT','ticket',NULL,0,0,'t')"
rej "recurrence.verdict" "$RL VALUES ('$HA',1,'import','nope','ticket',NULL,0,0,'t')"
rej "recurrence.match_basis" "$RL VALUES ('$HA',1,'import','SAME_DEFECT','nope',NULL,0,0,'t')"
rej "recurrence.reopened" "$RL VALUES ('$HA',1,'import','SAME_DEFECT','ticket',NULL,2,0,'t')"
CD="INSERT INTO reg_closure_decisions(atm_id,to_status,decision,decision_json_evidence_id,decided_at)"
rej "closure_decisions.to_status" "$CD VALUES ('$HA','Queued','REFUSED',$HEV,$T0)"
rej "closure_decisions.decision" "$CD VALUES ('$HA','Fixed (→ Fixed.md)','MAYBE',$HEV,$T0)"
rej "trackers.tracker_id lowercase" "INSERT INTO reg_trackers(tracker_id,kind) VALUES ('GH2','github')"
rej "trackers.enabled" "INSERT INTO reg_trackers(tracker_id,kind,enabled) VALUES ('gh2','github',2)"
TS="INSERT INTO reg_tracker_sync_log(tracker_id,atm_id,status,skip_reason,exit_code,item_revision,attempted_at)"
rej "tracker_sync.status" "$TS VALUES ('gh','$HA','NOPE',NULL,NULL,'r',$T0)"
rej "tracker_sync.skip_reason" "$TS VALUES ('gh','$HA','SKIPPED','nope',NULL,'r',$T0)"
sql "$DB" "INSERT INTO reg_export_runs(db_fingerprint,engine_version,started_at,verdict) VALUES ('f','v',$T0,'OK')" >/dev/null
XF="INSERT INTO reg_export_files(export_id,path,format,sha256,source_path,status)"
rej "export_files.format" "$XF VALUES (1,'p','nope','$(H x1)','s','written')"
rej "export_files.sha256 length" "$XF VALUES (1,'p','md','abc','s','written')"
rej "export_files.status" "$XF VALUES (1,'p','md','$(H x1)','s','nope')"
rej "cycle.manual_qa_ran" "INSERT INTO reg_cycle VALUES ('c2',2)"
sql "$DB" "INSERT INTO reg_findings(unit_alias,atm_id,run_id,component_id,location_path,category,severity,detector,fingerprint,evidence_id) VALUES ('F-web-001','$HA','RUN-1','web','a','bug','low','d','$F64',$HEV)" >/dev/null
DS="INSERT INTO reg_discovery(finding_id,cycle_id,channel,should_have_been_caught_by,none_justification,recorded_by,producer)"
rej "discovery.channel" "$DS VALUES ('FND-0001','c1','nope','seam',NULL,'a','b')"
rej "discovery.should_have_been_caught_by empty" "$DS VALUES ('FND-0001','c1','manual_qa','',NULL,'a','b')"
rej "discovery.recorded_by equals producer" "$DS VALUES ('FND-0001','c1','manual_qa','seam',NULL,' A','a')"
rej "discovery.none needs a 20-char justification" "$DS VALUES ('FND-0001','c1','manual_qa','none','too short','a','b')"
rej "discovery.none with NULL justification" "$DS VALUES ('FND-0001','c1','manual_qa','none',NULL,'a','b')"
expect_ok "$DB" "H positive control: discovery with a real justification accepted" "$DS VALUES ('FND-0001','c1','manual_qa','none','the seam did not exist when this shipped','a','b')"
rej "escape_baseline.escapes negative" "INSERT INTO reg_escape_baseline VALUES ('c1',-1)"
finish
