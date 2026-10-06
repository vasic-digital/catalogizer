#!/usr/bin/env bash
# test_register_behaviour.sh - WF2 review B-1 (fix round 3): BEHAVIOURAL coverage of every trigger and every
# custody clause of scripts/register/register_ext.sql. It contains NO schema-object count assertion on purpose:
# a mutant that removes a guard must be killed because the guarded WRONG ACTION is no longer refused (or the
# guarded RIGHT ACTION no longer happens), never because a number changed (the §11.4.115(F) tautology class).
# Real SQLite + the real engine binary, scratch DBs only. Env REG_EXT_SQL aims the test at a mutant DDL.
# Last section (T-COVER) fails when a trigger of the DDL under test has no behavioural assertion registered
# through tg(); the mutation runner is the proof that each assertion really bites.
. "$(dirname "$0")/lib.sh"
ident_header T060b
H() { printf '%s' "$1" | sha256sum | cut -d' ' -f1; }
D="$T_SCR/b"; mkdir -p "$D"
DESC="scratch item for the register behaviour tests, long enough for the engine description floor"
NOW="'2026-10-05T00:00:00Z'"
EVI="INSERT INTO reg_evidence(atm_id,kind,evidence_class,path,sha256,size_bytes,target_fingerprint,polarity,exit_code,iterations,precondition_provenance,produced_by,captured_at)"
TRX="INSERT INTO reg_test_runs(atm_id,test_id,type_code,group_id,rep_index,polarity,verdict,target_fingerprint,evidence_id,started_at)"
RVI="INSERT INTO reg_reviews(atm_id,author,reviewer,model,effort,verdict,evidence_id,reviewed_at)"
CDX="INSERT INTO reg_closure_decisions(atm_id,to_status,decision,decision_json_evidence_id,decided_at)"
COV=""; tg() { COV="$COV $1 "; }          # register the trigger a following assertion is aimed at
fresh() { fresh_ext_db "$1" || { bad "fresh db $1"; finish; exit 1; }; }
addit() { "$WI" add Task Low --db "$1" --id "$2" --prefix CAT --title "t $2" --description "$DESC" >/dev/null 2>&1; }
inprog() { for st in "In progress" "Ready for testing" "In testing"; do "$WI" update --id "$2" --db "$1" --status "$st" >/dev/null 2>&1; done; }
# ---- chain helpers (globals DB X CYC): one item driven to In testing, evidence rows, runs, review, decision
CYC=1
setup_item() { DB=$1; fresh "$DB"; X=$(mint "$DB"); LAYER=${2:-source}
  sql "$DB" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity) VALUES ('$X','bug','$LAYER','low')" >/dev/null
  addit "$DB" "$X"; inprog "$DB" "$X"; CYC=1; }
# e KIND SALT FP POLARITY EXIT ITER PROVENANCE PRODUCER [CLASS]  -> evidence_id
e() { sql "$DB" "$EVI VALUES ('$X','$1','${9:-source}','p','$(H "$1$2$CYC")',10,${3:-NULL},${4:-NULL},${5:-NULL},${6:-NULL},${7:-NULL},'$8',$NOW); SELECT max(evidence_id) FROM reg_evidence" | tail -1; }
red1() { local r; r=$(e red_run 1 "'fpA$CYC'" "'RED'" 1 NULL "'${1:-observed}'" "${2:-dev}" "${3:-source}"); sql "$DB" "$TRX VALUES ('$X','T1','unit','r$CYC',1,'RED','FAIL','fpA$CYC',$r,$NOW)" >/dev/null; }
green() { local g n=${2:-3} i; for i in $(seq 1 "$n"); do g=$(e green_run $i "'$1'" "'GREEN'" 0 3 NULL dev "${3:-source}"); sql "$DB" "$TRX VALUES ('$X','T1','unit','g$CYC',$i,'GREEN','PASS','$1',$g,$NOW)" >/dev/null; done; }
mutn() { local m; m=$(e mutation_run 1 "'$1'" NULL 1 NULL NULL dev "${3:-source}"); sql "$DB" "$TRX VALUES ('$X','${2:-T1}','unit','m$CYC',1,'MUTATION','FAIL','$1',$m,$NOW)" >/dev/null; }
review() { local rv; rv=$(e review_verdict ${3:-1} NULL NULL NULL NULL NULL "$1"); sql "$DB" "$RVI VALUES ('$X','$2','$1','opus','xhigh','${4:-GO}',$rv,$NOW)" >/dev/null; }
# chain REDPROV REDPRODUCER GREENFP REVIEWER AUTHOR : RED on fpA<cyc>, GREENx3 on GREENFP, caught mutation, GO review
chain() { red1 "$1" "$2"; green "$3"; mutn "$3"; review "$4" "$5"; }
decide() { local cd_; cd_=$(e custody_decision 1 NULL NULL NULL NULL NULL dev); sql "$DB" "$CDX VALUES ('$X','Completed (→ Fixed.md)','ACCEPTED',$cd_,$NOW)" >/dev/null 2>&1 && echo ACCEPTED || echo REFUSED; }
reopen_cycle() { echo e1 > "$D/ev.txt"; "$WI" reopen --id "$X" --db "$DB" --why manual-testing-detected --who AI --when 2026-10-05T01:00:00Z --incident "$D/ev.txt" >/dev/null 2>&1; inprog "$DB" "$X"; }
close_it() { echo e1 > "$D/ev.txt"; "$WI" close "$X" --db "$DB" --status completed --evidence "$D/ev.txt" >/dev/null 2>&1; }

echo "== T1. identity: reg_ids =="
DB="$D/t1.db"; fresh "$DB"; A=$(mint "$DB"); B=$(mint "$DB")
tg reg_ids_no_update;  expect_reject "$DB" "T1a UPDATE reg_ids refused" "reg_ids is append-only \(§11.4.54\)" "UPDATE reg_ids SET minted_by='z' WHERE atm_id='$B'"
tg reg_ids_no_delete;  expect_reject "$DB" "T1b DELETE reg_ids refused (unreferenced id)" "reg_ids is append-only \(§11.4.54\)" "DELETE FROM reg_ids WHERE atm_id='$B'"
tg reg_ids_no_replace; expect_reject "$DB" "T1c INSERT OR REPLACE reg_ids refused" "key exists" "INSERT OR REPLACE INTO reg_ids(seq,minted_by,mint_basis) VALUES (1,'x','manual')"
assert_eq "T1d the refused calls left the ids as minted" "$(sql "$DB" "select group_concat(atm_id||':'||minted_by) from (select * from reg_ids order by seq)")" "CAT-001:tester,CAT-002:tester"

echo "== T2. reg_item_ext custody =="
DB="$D/t2.db"; fresh "$DB"; A=$(mint "$DB"); B=$(mint "$DB"); I1=$(mint "$DB")
sql "$DB" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('t','import'),('t','import')" >/dev/null
IMP1=CAT-004; IMP2=CAT-005
expect_ok "$DB" "T2.0 control: ext rows for A (artifact) and B (reverify 1) accepted" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity) VALUES ('$A','bug','artifact','low'); INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,reverify_required) VALUES ('$B','bug','source','low',1)"
tg reg_item_ext_no_delete;  expect_reject "$DB" "T2a DELETE reg_item_ext refused" "never deleted" "DELETE FROM reg_item_ext WHERE atm_id='$A'"
tg reg_item_ext_no_replace; expect_reject "$DB" "T2b INSERT OR REPLACE reg_item_ext refused" "row exists" "INSERT OR REPLACE INTO reg_item_ext(atm_id,category,defect_layer,severity) VALUES ('$A','bug','source','low')"
assert_eq "T2c the refused REPLACE kept the artifact layer" "$(sql "$DB" "select defect_layer from reg_item_ext where atm_id='$A'")" artifact
tg trg_item_ext_legacy_insert
expect_reject "$DB" "T2d legacy_import for a non-import mint refused" "mint_basis=import" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,legacy_status) VALUES ('$I1','bug','source','low','legacy_import','closed')"
expect_reject "$DB" "T2e legacy_import for an OPEN legacy_status refused (§14.10 m-a, R10)" "closed-class" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,legacy_status) VALUES ('$IMP1','bug','source','low','legacy_import','open')"
expect_reject "$DB" "T2f legacy_import with a NULL legacy_status refused" "closed-class" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,legacy_status) VALUES ('$IMP1','bug','source','low','legacy_import',NULL)"
addit "$DB" "$IMP2"
expect_reject "$DB" "T2g legacy_import after the id had an items row refused" "before the id ever had an items row" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,legacy_status) VALUES ('$IMP2','bug','source','low','legacy_import','closed')"
expect_ok "$DB" "T2h control: legacy_import for an import mint, closed-class status, no items row accepted" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,legacy_status,reverify_required) VALUES ('$IMP1','bug','source','low','legacy_import',' Closed ',1)"
tg trg_item_ext_custody_update
expect_reject "$DB" "T2i atm_id update refused" "immutable" "UPDATE reg_item_ext SET atm_id='$I1' WHERE atm_id='$A'"
expect_reject "$DB" "T2j defect_layer lowered refused" "can only be raised" "UPDATE reg_item_ext SET defect_layer='source' WHERE atm_id='$A'"
expect_ok "$DB" "T2k control: defect_layer raised accepted" "UPDATE reg_item_ext SET defect_layer='runtime' WHERE atm_id='$A'"
expect_reject "$DB" "T2l custody_basis changed TO legacy_import refused" "can never be changed TO legacy_import" "UPDATE reg_item_ext SET custody_basis='legacy_import' WHERE atm_id='$A'"
expect_reject "$DB" "T2m reverify_required set 0 -> 1 after insert refused (R1)" "can only be cleared" "UPDATE reg_item_ext SET reverify_required=1 WHERE atm_id='$A'"
expect_ok "$DB" "T2n control: reverify_required cleared 1 -> 0 accepted" "UPDATE reg_item_ext SET reverify_required=0 WHERE atm_id='$B'"
assert_eq "T2o reverify flag stayed cleared and the layer raised" "$(sql "$DB" "select group_concat(x) from (select atm_id||':'||reverify_required||':'||defect_layer as x from reg_item_ext where atm_id in ('$A','$B') order by atm_id)")" "$A:0:runtime,$B:0:source"

echo "== T3. sources, entries, map =="
DB="$D/t3.db"; fresh "$DB"; A=$(mint "$DB"); B=$(mint "$DB")
sql "$DB" "INSERT INTO reg_sources(kind,locator,parser) VALUES ('md_tracker','t.md','p'); INSERT INTO reg_source_entries(source_id,locator,title,entry_sha256) VALUES (1,'e1','orig','$(H e1)'),(1,'e2','two','$(H e2)')" >/dev/null
tg reg_source_entries_no_delete; expect_reject "$DB" "T3a DELETE of a scanned entry refused (FR-002)" "never deleted" "DELETE FROM reg_source_entries WHERE entry_id=2"
tg reg_source_entries_no_replace
sql "$DB" "INSERT OR REPLACE INTO reg_source_entries(entry_id,source_id,locator,title,entry_sha256) VALUES (1,1,'e1','CHANGED','$(H e1b)')" >/dev/null 2>&1
assert_eq "T3b INSERT OR REPLACE by entry_id keeps the scanned row unchanged" "$(sql "$DB" "select title from reg_source_entries where entry_id=1")" orig
sql "$DB" "INSERT OR REPLACE INTO reg_source_entries(entry_id,source_id,locator,title,entry_sha256) VALUES (99,1,'e1','CHANGED2','$(H e1c)')" >/dev/null 2>&1
assert_eq "T3c INSERT OR REPLACE by (source_id, locator) keeps the scanned row and adds none" "$(sql "$DB" "select group_concat(x) from (select entry_id||':'||title as x from reg_source_entries order by entry_id)")" "1:orig,2:two"
expect_ok "$DB" "T3d control: a new entry is accepted" "INSERT INTO reg_source_entries(source_id,locator,title,entry_sha256) VALUES (1,'e3','three','$(H e3)')"
SM="INSERT INTO reg_source_map(entry_id,atm_id,relation,match_basis,severity_governs,mapped_by)"
expect_ok "$DB" "T3e control: two mappings (entry 1, 2) of A, one severity-governing" "$SM VALUES (1,'$A','primary','ticket',1,'t'),(2,'$A','sibling','ticket',0,'t')"
tg reg_source_map_no_delete; expect_reject "$DB" "T3f DELETE of a mapping refused (FR-002)" "never deleted" "DELETE FROM reg_source_map WHERE entry_id=2"
expect_reject "$DB" "T3g a second severity-governing mapping of the same id refused (uq_source_map_severity)" "UNIQUE" "$SM VALUES (3,'$A','sibling','ticket',1,'t')"
expect_ok "$DB" "T3h control: the mapping may be corrected by UPDATE" "UPDATE reg_source_map SET relation='refers_to' WHERE entry_id=2"

echo "== T4. findings =="
DB="$D/t4.db"; fresh "$DB"; A=$(mint "$DB"); B=$(mint "$DB"); G40=$(printf 'a%.0s' $(seq 40))
sql "$DB" "INSERT INTO reg_components(component_id,kind,path_root) VALUES ('web','web','w'),('api','backend','a'); INSERT INTO reg_audit_runs VALUES ('RUN-1',$NOW,'$G40','p','t'),('RUN-2',$NOW,'$G40','p','t');
 $EVI VALUES ('$A','log','source','p','$(H f1)',1,NULL,NULL,NULL,NULL,NULL,'d',$NOW);" >/dev/null
FN="INSERT INTO reg_findings(unit_alias,atm_id,run_id,component_id,location_path,category,severity,detector,fingerprint,evidence_id)"
expect_ok "$DB" "T4.0 control: a finding is accepted" "$FN VALUES ('F-web-001','$A','RUN-1','web','a.go','bug','critical','det','$(H fp1)',1)"
tg reg_findings_id_no_update
expect_reject "$DB" "T4a unit_alias update refused" "ids are immutable" "UPDATE reg_findings SET unit_alias='F-web-002' WHERE finding_seq=1"
expect_reject "$DB" "T4b finding_seq update refused" "ids are immutable" "UPDATE reg_findings SET finding_seq=7 WHERE finding_seq=1"
tg reg_findings_no_replace
expect_reject "$DB" "T4c INSERT OR REPLACE of the same unit_alias refused" "key exists" "INSERT OR REPLACE INTO reg_findings(unit_alias,atm_id,run_id,component_id,location_path,category,severity,detector,fingerprint,evidence_id) VALUES ('F-web-001','$A','RUN-1','web','x','bug','low','d','$(H fp9)',1)"
expect_reject "$DB" "T4d the same fingerprint in the same run refused as a duplicate key" "key exists" "$FN VALUES ('F-web-002','$A','RUN-1','web','a.go','bug','critical','det','$(H fp1)',1)"
tg reg_findings_no_delete;           expect_reject "$DB" "T4e DELETE of a finding refused (I-4)" "never deleted" "DELETE FROM reg_findings WHERE finding_seq=1"
tg reg_findings_observation_no_update
expect_reject "$DB" "T4f severity lowered critical -> cosmetic refused (I-4)" "never edited" "UPDATE reg_findings SET severity='cosmetic' WHERE finding_seq=1"
expect_reject "$DB" "T4g atm_id re-pointed to another item refused (I-4)" "never edited" "UPDATE reg_findings SET atm_id='$B' WHERE finding_seq=1"
expect_reject "$DB" "T4h fingerprint rewritten refused (I-4)" "never edited" "UPDATE reg_findings SET fingerprint='$(H fpz)' WHERE finding_seq=1"
expect_reject "$DB" "T4i run_id edit refused (I-4)" "never edited" "UPDATE reg_findings SET run_id='RUN-2' WHERE finding_seq=1"
expect_reject "$DB" "T4j location_path edit refused (I-4)" "never edited" "UPDATE reg_findings SET location_path='z' WHERE finding_seq=1"
expect_reject "$DB" "T4k detector edit refused (I-4)" "never edited" "UPDATE reg_findings SET detector='z' WHERE finding_seq=1"
sql "$DB" "$EVI VALUES ('$A','log','source','p2','$(H f2)',1,NULL,NULL,NULL,NULL,NULL,'d',$NOW);" >/dev/null   # a second evidence row so an evidence_id edit is FK-valid (WF3 I4)
expect_reject "$DB" "T4m category edit refused (WF3 I4, every observation column)" "never edited" "UPDATE reg_findings SET category='gap' WHERE finding_seq=1"
expect_reject "$DB" "T4n evidence_id re-pointed to another evidence row refused (WF3 I4)" "never edited" "UPDATE reg_findings SET evidence_id=2 WHERE finding_seq=1"
expect_reject "$DB" "T4o location_line edit refused (WF3 I4)" "never edited" "UPDATE reg_findings SET location_line=77 WHERE finding_seq=1"
assert_eq "T4l the finding is exactly as recorded after every refused edit" "$(sql "$DB" "select atm_id||'|'||severity||'|'||run_id||'|'||location_path||'|'||detector from reg_findings")" "$A|critical|RUN-1|a.go|det"

echo "== T5. evidence, test runs, reviews: append-only ledgers =="
DB="$D/t5.db"; fresh "$DB"; A=$(mint "$DB")
sql "$DB" "$EVI VALUES ('$A','log','source','p','$(H l1)',1,NULL,NULL,NULL,NULL,NULL,'dev',$NOW)" >/dev/null
tg reg_evidence_no_update;  expect_reject "$DB" "T5a UPDATE reg_evidence refused" "append-only" "UPDATE reg_evidence SET path='x'"
tg reg_evidence_no_delete;  expect_reject "$DB" "T5b DELETE reg_evidence refused" "append-only" "DELETE FROM reg_evidence"
tg reg_evidence_no_replace; expect_reject "$DB" "T5c INSERT OR REPLACE reg_evidence refused" "key exists" "INSERT OR REPLACE INTO reg_evidence(evidence_id,atm_id,kind,evidence_class,path,sha256,size_bytes,produced_by,captured_at) VALUES (1,'$A','log','source','p','$(H l2)',1,'dev',$NOW)"
TR="INSERT INTO reg_test_runs(atm_id,test_id,type_code,group_id,rep_index,polarity,verdict,blocked_reason,target_fingerprint,evidence_id,started_at)"
expect_ok "$DB" "T5.0 control: a GREEN row and a review are accepted" "$TR VALUES ('$A','t','unit','g1',1,'GREEN','PASS',NULL,'fp',1,$NOW); $RVI VALUES ('$A','alice','bob','m','e','GO',1,$NOW)"
tg reg_test_runs_no_update;  expect_reject "$DB" "T5d UPDATE reg_test_runs refused" "append-only" "UPDATE reg_test_runs SET test_id='x'"
tg reg_test_runs_no_delete;  expect_reject "$DB" "T5e DELETE reg_test_runs refused" "append-only" "DELETE FROM reg_test_runs"
tg reg_test_runs_no_replace; expect_reject "$DB" "T5f INSERT OR REPLACE reg_test_runs (same run_row) refused" "key exists" "INSERT OR REPLACE INTO reg_test_runs(run_row,atm_id,test_id,type_code,group_id,rep_index,polarity,verdict,target_fingerprint,evidence_id,started_at) VALUES (1,'$A','t2','unit','g2',1,'GREEN','PASS','fp',1,$NOW)"
expect_reject "$DB" "T5g INSERT OR REPLACE reg_test_runs (same group_id, rep_index) refused" "key exists" "INSERT OR REPLACE INTO reg_test_runs(atm_id,test_id,type_code,group_id,rep_index,polarity,verdict,target_fingerprint,evidence_id,started_at) VALUES ('$A','t2','unit','g1',1,'GREEN','PASS','fp',1,$NOW)"
tg reg_reviews_no_update;  expect_reject "$DB" "T5h UPDATE reg_reviews refused" "append-only" "UPDATE reg_reviews SET verdict='NO-GO'"
tg reg_reviews_no_delete;  expect_reject "$DB" "T5i DELETE reg_reviews refused" "append-only" "DELETE FROM reg_reviews"
tg reg_reviews_no_replace; expect_reject "$DB" "T5j INSERT OR REPLACE reg_reviews refused" "key exists" "INSERT OR REPLACE INTO reg_reviews(review_id,atm_id,author,reviewer,model,effort,verdict,evidence_id,reviewed_at) VALUES (1,'$A','a','b','m','e','NO-GO',1,$NOW)"
assert_eq "T5k every ledger kept its single original row" "$(sql "$DB" "select (select count(*) from reg_evidence)||'/'||(select count(*) from reg_test_runs)||'/'||(select count(*) from reg_reviews)||'/'||(select verdict from reg_reviews)")" "1/1/1/GO"

echo "== T6. recurrence links =="
DB="$D/t6.db"; fresh "$DB"; A=$(mint "$DB")
sql "$DB" "INSERT INTO reg_sources(kind,locator,parser) VALUES ('md_tracker','t.md','p'); INSERT INTO reg_source_entries(source_id,locator,entry_sha256) VALUES (1,'e1','$(H e1)')" >/dev/null
LK="INSERT INTO reg_recurrence_links(head_atm_id,reported_entry_id,intake_path,verdict,match_basis,new_atm_id,head_log_id,decided_by)"
tg reg_recurrence_links_head_guard; expect_reject "$DB" "T6a head_log_id that is not the head's last log id refused" "head_log_id" "$LK VALUES ('$A',1,'import','SAME_DEFECT','ticket',NULL,99,'t')"
expect_ok "$DB" "T6.0 control: a link with the right decision point is accepted" "$LK VALUES ('$A',1,'import','SAME_DEFECT','ticket',NULL,0,'t')"
tg reg_recurrence_links_no_update;  expect_reject "$DB" "T6b UPDATE refused" "append-only" "UPDATE reg_recurrence_links SET decided_by='x'"
tg reg_recurrence_links_no_delete;  expect_reject "$DB" "T6c DELETE refused" "append-only" "DELETE FROM reg_recurrence_links"
tg reg_recurrence_links_no_replace; expect_reject "$DB" "T6d INSERT OR REPLACE refused" "key exists" "INSERT OR REPLACE INTO reg_recurrence_links(link_id,head_atm_id,reported_entry_id,intake_path,verdict,match_basis,head_log_id,decided_by) VALUES (1,'$A',1,'import','SAME_DEFECT','ticket',0,'x')"

echo "== T7. status log: insert guard, append-only, high-water marks (item that pre-dates the extension) =="
DB="$D/t7.db"; rm -f "$DB"; "$WI" validate --db "$DB" >/dev/null 2>&1; "$WI" add Task Low --db "$DB" --id CAT-001 --prefix CAT --title pre --description "$DESC" >/dev/null 2>&1
"$REG_DIR/apply_ext.sh" --db "$DB" >/dev/null 2>&1
assert_eq "T7.0 the pre-existing item has no status-log row yet" "$(sql "$DB" "select count(*) from reg_status_log where atm_id='CAT-001'")" 0
LG="INSERT INTO reg_status_log(atm_id,from_status,to_status,ev_hwm,run_hwm,rev_hwm)"
tg reg_status_log_insert_guard
expect_reject "$DB" "T7a to_status that is not the current items status refused" "to_status must equal the current items status" "$LG VALUES ('CAT-001',NULL,'In progress',0,0,0)"
expect_reject "$DB" "T7b from_status equal to to_status refused" "from_status must equal the last logged" "$LG VALUES ('CAT-001','Queued','Queued',0,0,0)"
expect_reject "$DB" "T7c wrong evidence high-water mark refused (R6)" "ledger maxima" "$LG VALUES ('CAT-001',NULL,'Queued',5,0,0)"
expect_reject "$DB" "T7d wrong run high-water mark refused" "ledger maxima" "$LG VALUES ('CAT-001',NULL,'Queued',0,7,0)"
expect_reject "$DB" "T7e wrong review high-water mark refused" "ledger maxima" "$LG VALUES ('CAT-001',NULL,'Queued',0,0,3)"
expect_ok "$DB" "T7f control: the correct first row is accepted" "$LG VALUES ('CAT-001',NULL,'Queued',0,0,0)"
tg reg_status_log_no_update;  expect_reject "$DB" "T7g UPDATE reg_status_log refused" "append-only" "UPDATE reg_status_log SET session_actor='x'"
tg reg_status_log_no_delete;  expect_reject "$DB" "T7h DELETE reg_status_log refused" "append-only" "DELETE FROM reg_status_log"
# no_replace is not redundant: after a tamper left the log lagging behind items (trigger dropped, status moved),
# a REPLACE of log row 1 can satisfy insert_guard and would rewrite history; only no_replace refuses it
DB="$D/t7b.db"; fresh "$DB"; A7=$(mint "$DB"); addit "$DB" "$A7"
sql "$DB" "DROP TRIGGER trg_items_status_log; UPDATE items SET status='In progress' WHERE atm_id='$A7'" >/dev/null
tg reg_status_log_no_replace; expect_reject "$DB" "T7i INSERT OR REPLACE of log row 1 that satisfies the insert guard refused" "key exists" "INSERT OR REPLACE INTO reg_status_log(log_id,atm_id,from_status,to_status,ev_hwm,run_hwm,rev_hwm) VALUES (1,'$A7','Queued','In progress',0,0,0)"
assert_eq "T7j log row 1 is still the original first row" "$(sql "$DB" "select (from_status is null)||'/'||to_status from reg_status_log where log_id=1")" "1/Queued"

echo "== T8. engine items table: mint, identity, transitions, log writers =="
DB="$D/t8.db"; fresh "$DB"; A=$(mint "$DB"); B=$(mint "$DB")
tg trg_items_insert_log; addit "$DB" "$A"
assert_eq "T8a an engine insert wrote the first log row (Queued)" "$(sql "$DB" "select (from_status is null)||'/'||to_status from reg_status_log where atm_id='$A'")" "1/Queued"
COLS=$(sql "$DB" "select group_concat(name) from pragma_table_info('items') where name not in ('atm_id','current_location','representation')")
tg trg_items_require_mint
expect_reject "$DB" "T8b raw items insert without a mint refused" "no reg_ids row" "INSERT INTO items(atm_id,current_location,representation,$COLS) SELECT 'CAT-777', current_location, representation, $COLS FROM items WHERE atm_id='$A'"
expect_reject "$DB" "T8c second items row for a minted id refused" "already has an items row" "INSERT INTO items(atm_id,current_location,representation,$COLS) SELECT '$A', 'Fixed', representation, $COLS FROM items WHERE atm_id='$A'"
tg trg_items_identity_update
expect_reject "$DB" "T8d items.atm_id re-pointed to an unminted id refused" "no reg_ids row" "UPDATE items SET atm_id='CAT-888' WHERE atm_id='$A'"
addit "$DB" "$B"
expect_reject "$DB" "T8e items.atm_id re-pointed to another item's id refused" "already has another items row" "UPDATE items SET atm_id='$B' WHERE atm_id='$A'"
tg trg_items_transition
expect_reject "$DB" "T8f an edge outside the graph refused (Queued -> In testing)" "not in reg_status_transitions" "UPDATE items SET status='In testing' WHERE atm_id='$A'"
assert_eq "T8g the refused statement left only the first log row" "$(sql "$DB" "select group_concat(to_status,'>') from reg_status_log where atm_id='$A'")" "Queued"
tg trg_items_status_log
sql "$DB" "UPDATE items SET status='In progress' WHERE atm_id='$A'" >/dev/null
assert_eq "T8h an UPDATE of status wrote a log row" "$(sql "$DB" "select group_concat(x,';') from (select ifnull(from_status,'-')||'>'||to_status as x from reg_status_log where atm_id='$A' order by log_id)")" "->Queued;Queued>In progress"
expect_reject "$DB" "T8i a terminal UPDATE without a live decision refused (item B driven to In testing)" "custody" "$(inprog "$DB" "$B"; echo "UPDATE items SET status='Completed (→ Fixed.md)' WHERE atm_id='$B'")"
tg trg_items_insert_guard
expect_reject "$DB" "T8j DELETE + INSERT as a status not reachable from the last logged one refused" "not reachable from the last logged status" "BEGIN; CREATE TEMP TABLE c AS SELECT * FROM items WHERE atm_id='$A'; DELETE FROM items WHERE atm_id='$A'; UPDATE c SET status='In testing'; INSERT INTO items SELECT * FROM c;"
expect_reject "$DB" "T8k DELETE + INSERT as a terminal status without a live decision refused" "custody: terminal insert" "BEGIN; CREATE TEMP TABLE c2 AS SELECT * FROM items WHERE atm_id='$B'; DELETE FROM items WHERE atm_id='$B'; UPDATE c2 SET status='Completed (→ Fixed.md)', current_location='Fixed'; INSERT INTO items SELECT * FROM c2;"
assert_eq "T8l both items survived the bypass attempts unchanged" "$(sql "$DB" "select group_concat(x) from (select atm_id||':'||status as x from items order by atm_id)")" "$A:In progress,$B:In testing"

echo "== T9. closure decisions: guard, append-only, single use =="
setup_item "$D/t9.db"
tg trg_closure_decision_guard
expect_reject "$DB" "T9a ACCEPTED on a non-decision evidence row refused" "custody_decision evidence row" "$CDX VALUES ('$X','Completed (→ Fixed.md)','ACCEPTED',(SELECT max(evidence_id) FROM reg_evidence),$NOW)"
l=$(e log 1 NULL NULL NULL NULL NULL dev); cdv=$(e custody_decision 1 NULL NULL NULL NULL NULL dev)
expect_reject "$DB" "T9a2 ACCEPTED on a non-decision (log) evidence row refused" "custody_decision evidence row" "$CDX VALUES ('$X','Completed (→ Fixed.md)','ACCEPTED',$l,$NOW)"
expect_reject "$DB" "T9b ACCEPTED on a custody_decision row with no chain refused" "chain incomplete" "$CDX VALUES ('$X','Completed (→ Fixed.md)','ACCEPTED',$cdv,$NOW)"
expect_ok "$DB" "T9c control: a REFUSED decision needs no chain" "$CDX VALUES ('$X','Completed (→ Fixed.md)','REFUSED',$cdv,$NOW)"
tg reg_closure_decisions_no_delete;  expect_reject "$DB" "T9d DELETE reg_closure_decisions refused" "append-only" "DELETE FROM reg_closure_decisions"
tg reg_closure_decisions_no_replace; expect_reject "$DB" "T9e INSERT OR REPLACE reg_closure_decisions refused" "key exists" "INSERT OR REPLACE INTO reg_closure_decisions(decision_id,atm_id,to_status,decision,decision_json_evidence_id,decided_at) VALUES (1,'$X','Completed (→ Fixed.md)','REFUSED',$cdv,$NOW)"
tg reg_closure_decisions_consume_only
expect_reject "$DB" "T9f editing the decision column refused" "only a single consumption" "UPDATE reg_closure_decisions SET decision='ACCEPTED' WHERE decision_id=1"
expect_reject "$DB" "T9g editing to_status refused" "only a single consumption" "UPDATE reg_closure_decisions SET to_status='Fixed (→ Fixed.md)' WHERE decision_id=1"
expect_ok "$DB" "T9h control: a single consumption (NULL -> set) is allowed" "UPDATE reg_closure_decisions SET consumed_at=$NOW WHERE decision_id=1"
expect_reject "$DB" "T9i a second consumption refused" "only a single consumption" "UPDATE reg_closure_decisions SET consumed_at='2099-01-01T00:00:00Z' WHERE decision_id=1"

echo "== C. custody chain: one scenario per clause; the complete chain is the only ACCEPTED one =="
setup_item "$D/c0.db"; chain observed dev fpB rev dev
assert_eq "C0 control: complete chain (RED observed, GREENx3 on another fingerprint, caught mutation, independent GO) ACCEPTED" "$(decide)" ACCEPTED
close_it
assert_eq "C0b the engine close then succeeds and the decision is consumed" "$(sql "$DB" "select status||'/'||(select consumed_at is not null from reg_closure_decisions) from items where atm_id='$X'")" "Completed (→ Fixed.md)/1"
assert_eq "C0c status log of the closed item is the full path" "$(sql "$DB" "select group_concat(to_status,'>') from (select to_status from reg_status_log where atm_id='$X' order by log_id)")" "Queued>In progress>Ready for testing>In testing>Completed (→ Fixed.md)"
setup_item "$D/c1.db"; assert_eq "C1 no evidence at all REFUSED" "$(decide)" REFUSED
setup_item "$D/c2.db"; chain constructed dev fpB rev dev
assert_eq "C2 RED with precondition_provenance=constructed REFUSED (R7, §11.4.115(G))" "$(decide)" REFUSED
setup_item "$D/c3.db"; chain observed rev fpB rev dev
assert_eq "C3 reviewer who produced the RED evidence REFUSED (R8, §11.4.240)" "$(decide)" REFUSED
setup_item "$D/c4.db"; chain observed dev fpB rev dev; review rev2 dev 2 NO-GO
assert_eq "C4 a later NO-GO review blocks (R9)" "$(decide)" REFUSED
setup_item "$D/c5.db"; chain observed dev fpA1 rev dev
assert_eq "C5 GREEN on the RED fingerprint (fix never deployed) REFUSED" "$(decide)" REFUSED
setup_item "$D/c6.db"; red1; green fpB 2; mutn fpB; review rev dev
assert_eq "C6 only two GREEN repetitions REFUSED" "$(decide)" REFUSED
setup_item "$D/c9.db"; red1; green fpB; review rev dev
assert_eq "C9 no caught mutation REFUSED" "$(decide)" REFUSED
setup_item "$D/c10.db"; chain observed dev fpB rev dev; sql "$DB" "INSERT INTO reg_test_runs(atm_id,test_id,type_code,group_id,rep_index,polarity,verdict,target_fingerprint,evidence_id,started_at) VALUES ('$X','T2','unit','m-other',1,'MUTATION','FAIL','fpB',(SELECT max(evidence_id) FROM reg_evidence WHERE kind='mutation_run'),$NOW)" >/dev/null
assert_eq "C10 control: an extra mutation row for another test leaves an otherwise complete chain ACCEPTED" "$(decide)" ACCEPTED
setup_item "$D/c11.db"; red1; green fpB; mutn fpB T9; review rev dev
assert_eq "C11 the caught mutation belongs to a DIFFERENT test_id than the RED/GREEN pair REFUSED" "$(decide)" REFUSED
setup_item "$D/c12.db"; red1; green fpB; mutn fpB
assert_eq "C12 no review at all REFUSED" "$(decide)" REFUSED
setup_item "$D/c13.db" runtime; chain observed dev fpB rev dev
assert_eq "C13 source-class evidence for a runtime-layer item REFUSED (class floor §11.4.226)" "$(decide)" REFUSED
setup_item "$D/c14.db" runtime; red1 observed dev runtime; green fpB 3 source; mutn fpB T1 runtime; review rev dev
assert_eq "C14 runtime RED and mutation but source-class GREEN for a runtime-layer item REFUSED" "$(decide)" REFUSED
setup_item "$D/c15.db"; chain observed dev fpB rev dev; r2=$(e red_run 9 "'fpQ'" "'RED'" 1 NULL "'observed'" dev); sql "$DB" "$TRX VALUES ('$X','T1','unit','rq',1,'RED','FAIL','fpQ',$r2,$NOW)" >/dev/null
assert_eq "C15 control: an additional RED on another fingerprint leaves the chain complete" "$(decide)" ACCEPTED
setup_item "$D/c16.db"; red1; green fpB; mutn fpB; review rev dev 1 NO-GO
assert_eq "C16 a NO-GO as the only review REFUSED" "$(decide)" REFUSED
setup_item "$D/c17.db"; red1 observed dev; green fpB; mutn fpB; review dev dev
[ "$(sql "$DB" "select count(*) from reg_reviews")" = 0 ] && ok "C17 the author=reviewer CHECK refused a self-review row" || bad "C17 a self-review row was stored"
assert_eq "C17b and the chain without an independent review REFUSED" "$(decide)" REFUSED

setup_item "$D/c18.db"; red1; for i in 1 2 3; do fp=fpB; [ $i -gt 1 ] && fp=fpC; g=$(e green_run $i "'$fp'" "'GREEN'" 0 3 NULL dev); sql "$DB" "$TRX VALUES ('$X','T1','unit','g1',$i,'GREEN','PASS','$fp',$g,$NOW)" >/dev/null; done; mutn fpB; review rev dev
assert_eq "C18 one GREEN group spread over two fingerprints REFUSED" "$(decide)" REFUSED
setup_item "$D/c19.db"; red1; green fpB; mutn fpB; rv=$(e review_verdict 1 NULL NULL NULL NULL NULL dev); sql "$DB" "$RVI VALUES ('$X','dev','rev','opus','xhigh','GO',$rv,$NOW)" >/dev/null
assert_eq "C19 a review whose verdict evidence was produced by someone other than the reviewer REFUSED" "$(decide)" REFUSED
setup_item "$D/c20.db" runtime; red1 observed dev runtime; green fpB 3 runtime; mutn fpB T1 source; review rev dev
assert_eq "C20 runtime-layer item with a source-class caught mutation REFUSED (class floor on the mutation)" "$(decide)" REFUSED
setup_item "$D/c20b.db" runtime; red1 observed dev runtime; green fpB 3 runtime; mutn fpB T1 runtime; review rev dev
assert_eq "C20b control: the same item with a runtime-class mutation ACCEPTED" "$(decide)" ACCEPTED
setup_item "$D/c21.db"; r=$(e red_run 1 "'fpZ'" "'RED'" 1 NULL "'observed'" dev); sql "$DB" "$TRX VALUES ('$X','T1','unit','r1',1,'RED','FAIL','fpA1',$r,$NOW)" >/dev/null; green fpB; mutn fpB; review rev dev
assert_eq "C21 RED whose evidence fingerprint differs from its test-run fingerprint REFUSED" "$(decide)" REFUSED
setup_item "$D/c22.db"; red1; green fpB; m=$(e mutation_run 1 "'fpB'" NULL 1 NULL NULL dev); sql "$DB" "$TRX VALUES ('$X','T1','unit','m1',1,'MUTATION','PASS','fpB',$m,$NOW)" >/dev/null; review rev dev
assert_eq "C22 a mutation that PASSed (survived) REFUSED" "$(decide)" REFUSED
setup_item "$D/c23.db"; red1; green fpB; m=$(e log 1 "'fpB'" NULL 1 NULL NULL dev); sql "$DB" "$TRX VALUES ('$X','T1','unit','m1',1,'MUTATION','FAIL','fpB',$m,$NOW)" >/dev/null; review rev dev
# (the mutation row above is backed by a plain log row that carries exit_code 1: only the kind check can refuse it)
assert_eq "C23 a mutation row backed by a plain log evidence row REFUSED" "$(decide)" REFUSED
setup_item "$D/c24.db"; Y=$(mint "$DB"); red1; green fpB; mutn fpB; ey=$(sql "$DB" "$EVI VALUES ('$Y','review_verdict','source','p','$(H foreign)',10,NULL,NULL,NULL,NULL,NULL,'rev',$NOW); SELECT max(evidence_id) FROM reg_evidence" | tail -1)
sql "$DB" "$RVI VALUES ('$X','dev','rev','opus','xhigh','GO',$ey,$NOW)" >/dev/null
assert_eq "C24 a review whose evidence row belongs to ANOTHER item REFUSED" "$(decide)" REFUSED
setup_item "$D/c25.db"; red1; green fpB; mutn fpB; el=$(e log 1 NULL NULL NULL NULL NULL rev); sql "$DB" "$RVI VALUES ('$X','dev','rev','opus','xhigh','GO',$el,$NOW)" >/dev/null
assert_eq "C25 a review whose evidence row is not a review_verdict REFUSED" "$(decide)" REFUSED
# obsolete path: custody_basis other than machine_evidence closes only through proof + independent review
decide_obs() { local cd_; cd_=$(e custody_decision 1 NULL NULL NULL NULL NULL dev); sql "$DB" "$CDX VALUES ('$X','Obsolete (→ Fixed.md)','ACCEPTED',$cd_,$NOW)" >/dev/null 2>&1 && echo ACCEPTED || echo REFUSED; }
setup_item "$D/c26.db"; sql "$DB" "DELETE FROM reg_item_ext WHERE 0" >/dev/null; "$SQLITE3" "$DB" "PRAGMA foreign_keys=ON; DROP TRIGGER reg_item_ext_no_delete; DELETE FROM reg_item_ext WHERE atm_id='$X'; INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis) VALUES ('$X','bug','source','low','false_positive_evidence')" >/dev/null 2>&1
e false_positive_proof 1 NULL NULL NULL NULL NULL dev >/dev/null; review rev dev
assert_eq "C26 false_positive_evidence item with a proof row and an independent GO closes as Obsolete (ACCEPTED)" "$(decide_obs)" ACCEPTED
setup_item "$D/c27.db"; "$SQLITE3" "$DB" "DROP TRIGGER reg_item_ext_no_delete; DELETE FROM reg_item_ext WHERE atm_id='$X'; INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis) VALUES ('$X','bug','source','low','false_positive_evidence')" >/dev/null 2>&1; review rev dev
assert_eq "C27 the same item without a proof row REFUSED" "$(decide_obs)" REFUSED
setup_item "$D/c28.db"; e false_positive_proof 1 NULL NULL NULL NULL NULL dev >/dev/null; review rev dev
assert_eq "C28 a machine_evidence item cannot close as Obsolete on a proof row (basis does not allow it) REFUSED" "$(decide_obs)" REFUSED
setup_item "$D/c29.db"; "$SQLITE3" "$DB" "DROP TRIGGER reg_item_ext_no_delete; DELETE FROM reg_item_ext WHERE atm_id='$X'; INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis) VALUES ('$X','bug','source','low','false_positive_evidence')" >/dev/null 2>&1; chain observed dev fpB rev dev
assert_eq "C29 a false_positive_evidence item with a complete FIX chain cannot close as Fixed/Completed REFUSED" "$(decide)" REFUSED

setup_item "$D/c30.db" runtime; red1 observed dev source; green fpB 3 runtime; mutn fpB T1 runtime; review rev dev
assert_eq "C30 runtime-layer item whose only RED is source-class REFUSED (class floor on the RED)" "$(decide)" REFUSED

echo "== R. reopen cycles: the old chain never counts; replay and GREEN-fingerprint reuse refused =="
setup_item "$D/r1.db"; chain observed dev fpB rev dev; [ "$(decide)" = ACCEPTED ] && close_it
assert_eq "R0 cycle 1 closed" "$(sql "$DB" "select status from items where atm_id='$X'")" "Completed (→ Fixed.md)"
reopen_cycle
assert_eq "R1 the engine reopen was recorded as a Reopened log row" "$(sql "$DB" "select count(*) from reg_status_log where atm_id='$X' and to_status='Reopened'")" 1
assert_eq "R2 a second close with no new chain is REFUSED (cycle boundary)" "$(decide)" REFUSED
expect_reject "$DB" "R3 replay of the last cycle-1 evidence file (custody_decision) refused (boundary, R2)" "replay" "$EVI VALUES ('$X','custody_decision','source','p','$(H custody_decision11)',10,NULL,NULL,NULL,NULL,NULL,'dev',$NOW)"
tg reg_evidence_no_replay
expect_reject "$DB" "R4 replay of a cycle-1 RED file refused" "replay" "$EVI VALUES ('$X','red_run','source','p','$(H red_run11)',10,'fpR','RED',1,NULL,'observed','dev',$NOW)"
CYC=2; chain observed dev fpB rev dev
assert_eq "R5 cycle 2 GREEN on the fingerprint that was GREEN in cycle 1 REFUSED (R5, §14.10 I1)" "$(decide)" REFUSED
CYC=3; chain observed dev fpC rev dev
assert_eq "R6 cycle 2 with a new GREEN fingerprint and a fresh review ACCEPTED" "$(decide)" ACCEPTED

cycle1_closed() { setup_item "$1"; chain observed dev fpB rev dev; [ "$(decide)" = ACCEPTED ] && close_it
  [ "$(sql "$DB" "select status from items where atm_id='$X'")" = "Completed (→ Fixed.md)" ] || bad "setup $1: cycle 1 did not close (the cycle-2 assertions below would be vacuous)"
  reopen_cycle
  [ "$(sql "$DB" "select status from items where atm_id='$X'")" = "In testing" ] || bad "setup $1: cycle 2 is not In testing after the reopen"; }
cycle1_closed "$D/r2.db"; CYC=2; green fpD; mutn fpD; review rev dev 1
assert_eq "R7 cycle 2 with a new GREEN, mutation and review but NO new RED REFUSED (the old RED does not count)" "$(decide)" REFUSED
red1; assert_eq "R7b control: adding the cycle-2 RED makes the chain ACCEPTED" "$(decide)" ACCEPTED
cycle1_closed "$D/r3.db"; CYC=2; red1; green fpD; review rev dev 1
assert_eq "R8 cycle 2 with a new RED, GREEN and review but NO new caught mutation REFUSED (the old mutation does not count)" "$(decide)" REFUSED
mutn fpD; assert_eq "R8b control: adding the cycle-2 mutation makes the chain ACCEPTED" "$(decide)" ACCEPTED
cycle1_closed "$D/r4.db"; CYC=2; red1; green fpD; mutn fpD
assert_eq "R9 cycle 2 without a cycle-2 review REFUSED (the old GO review does not count)" "$(decide)" REFUSED

# GREEN rows of a later cycle that cite GREEN evidence recorded in an EARLIER cycle (evidence nobody used before the reopen)
setup_item "$D/r5.db"; chain observed dev fpB rev dev; for i in 1 2 3; do e green_run "stale$i" "'fpE'" "'GREEN'" 0 3 NULL dev >/dev/null; done
STALE1=$(sql "$DB" "select min(evidence_id) from reg_evidence where kind='green_run' and target_fingerprint='fpE'")
[ "$(decide)" = ACCEPTED ] && close_it
assert_eq "R10.0 setup: cycle 1 closed" "$(sql "$DB" "select status from items where atm_id='$X'")" "Completed (→ Fixed.md)"
reopen_cycle; CYC=2; red1
for i in 1 2 3; do sql "$DB" "$TRX VALUES ('$X','T1','unit','g-stale',$i,'GREEN','PASS','fpE',$((STALE1+i-1)),$NOW)" >/dev/null; done
mutn fpE; review rev dev 1
assert_eq "R10 cycle-2 GREEN rows that cite GREEN evidence recorded before the reopen REFUSED (evidence bound to the cycle)" "$(decide)" REFUSED

echo "== L. legacy import: exemption, reopen clears it =="
DB="$D/l1.db"; fresh "$DB"; sql "$DB" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('t','import')" >/dev/null; X=CAT-001
sql "$DB" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,legacy_status,reverify_required) VALUES ('$X','bug','source','low','legacy_import','closed',1)" >/dev/null
addit "$DB" "$X"; close_it
assert_eq "L1 an import-time legacy closure is accepted without a chain (exemption, Queued -> terminal)" "$(sql "$DB" "select status from items where atm_id='$X'")" "Completed (→ Fixed.md)"
assert_eq "L2 the exemption view reports the id while it awaits re-verification" "$(sql "$DB" "select count(*) from v_legacy_exempt where atm_id='$X'")" 1
tg trg_status_log_reopen_legacy
reopen_cycle
assert_eq "L3 the reopen turned the legacy row into an ordinary machine_evidence item" "$(sql "$DB" "select custody_basis||'/'||reverify_required from reg_item_ext where atm_id='$X'")" "machine_evidence/0"
assert_eq "L4 and the exemption view no longer reports it" "$(sql "$DB" "select count(*) from v_legacy_exempt where atm_id='$X'")" 0
DB="$D/l2.db"; fresh "$DB"; sql "$DB" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('t','import')" >/dev/null; X=CAT-001
sql "$DB" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,legacy_status,reverify_required) VALUES ('$X','bug','source','low','legacy_import','resolved',1)" >/dev/null
addit "$DB" "$X"; inprog "$DB" "$X"
out=$("$WI" close "$X" --db "$DB" --status completed --evidence "$D/ev.txt" 2>&1); rc=$?
[ $rc -ne 0 ] && ok "L5 a legacy item that was worked on (In testing) loses the exemption: the close is REFUSED" || bad "L5 rc=$rc [$out]"

echo "== T-COVER. every trigger of the DDL under test has a behavioural assertion registered through tg() =="
DBC="$D/cov.db"; fresh "$DBC"
missing=""; while read -r t; do case "$COV" in *" $t "*) ;; *) missing="$missing $t";; esac; done < <(sql "$DBC" "select name from sqlite_master where type='trigger' order by name")
[ -z "$missing" ] && ok "T-COVER no trigger without a behavioural assertion" || bad "T-COVER triggers with no behavioural assertion:$missing"
finish
