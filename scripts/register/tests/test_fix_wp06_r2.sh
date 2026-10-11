#!/usr/bin/env bash
# test_fix_wp06_r2.sh - WP-06 T068 independent review (verdict NO-GO, evidence/reviews/WP-06-T068.json) fix round: R-01 .. R-11.
# Written FIRST (11.4.224): every section reproduces one reviewed finding (or one member of its defect class) against the committed scripts (RED) and
# must pass on the fixed ones (GREEN). Host side only: real SQLite + the real engine binary on scratch DBs, stub RUNP / podman for locked.sh and stub
# LOCKED / BACKUP for replay.sh (the documented LOCKED_TEST_MODE hooks). No container is started, no image is built or pulled, and the real
# docs/workable_items.db is never created or touched.
# Classes: C1 "the id mint accepts an id the engine grammar or the monotone rule forbids" (R-01, R-03), C2 "an absent field reads as its safe value"
# (R-02, R-10 reads of a never-created dir), C3 "a check that does not compare the sources of one declared value" (R-04), C4 "a guard with no isolated test"
# (R-05, R-06), C5 "a side path skipped by mode" (R-11).
# Sections (ONLY="R01 R02 ..."): R01 R03 R04 R05 R06 R02 R10 R11.
# Env: GATE (gate.sh under test), REPLAY, LOCKED (mutant substitution, as the other suites).
. "$(dirname "$0")/lib.sh"; . "$(dirname "$0")/clib.sh"
ident_header WP06-T068-R2
GATE=${GATE:-$REG_DIR/gate.sh}
ONLY=" ${ONLY:-R01 R03 R04 R05 R06 R02 R10 R11} "
want() { case "$ONLY" in *" $1 "*) return 0;; esac; return 1; }
D="$T_SCR/f"; mkdir -p "$D"
DESC="scratch item for the T068 fix round tests, long enough for the engine description floor"
H() { printf '%s' "$1" | sha256sum | cut -d' ' -f1; }
run_gate() { REG_ROOT=${GATE_ROOT:-$ROOT} bash "$GATE" --db "$1" 2>&1; }
good_db() { fresh_ext_db "$1" || return 1; local id; id=$(mint "$1"); "$WI" add Task Low --db "$1" --id "$id" --prefix CAT --title "t" --description "$DESC" >/dev/null 2>&1; }
must_fail() {  # label db pattern
  local out rc; out=$(run_gate "$2"); rc=$?
  if [ $rc -ne 0 ] && ! printf '%s' "$out" | grep -q 'GATE OK' && printf '%s' "$out" | grep -Eq "$3"; then ok "$1"; else bad "$1 :: rc=$rc out=[$(printf '%s' "$out" | head -4 | tr '\n' '|')] wanted /$3/"; fi; }
# restore: re-apply the DDL so a dropped trigger is back (the forged rows stay, as the other golden-bad fixtures do)
reapply() { "$REG_DIR/apply_ext.sh" --db "$1" >/dev/null 2>&1; }

if want R01; then echo "== R-01: reg_ids accepts only the id the engine would mint next; malformed / gapped ids fail the gate =="
  DB="$D/r01.db"; fresh_ext_db "$DB" || { bad "R01 setup"; finish; exit 1; }
  for n in 1 2 3; do mint "$DB" >/dev/null; done
  assert_eq "R01-0 control: three engine-assigned mints give CAT-001..003" "$(sql "$DB" "select group_concat(atm_id) from (select atm_id from reg_ids order by seq)")" "CAT-001,CAT-002,CAT-003"
  M="INSERT INTO reg_ids(seq,minted_by,mint_basis) VALUES"
  expect_reject "$DB" "R01-1 explicit seq 9 (a gap after CAT-003) refused" "reg_ids" "$M (9,'x','manual')"
  expect_reject "$DB" "R01-2 explicit seq 0 (CAT-000) refused" "reg_ids" "$M (0,'x','manual')"
  expect_reject "$DB" "R01-3 explicit seq -3 (CAT--03, outside the engine heading grammar) refused" "reg_ids" "$M (-3,'x','manual')"
  expect_reject "$DB" "R01-4 explicit seq 2 (below the maximum, out of order) refused" "reg_ids" "$M (2,'x','manual')"
  expect_reject "$DB" "R01-4b explicit seq -1 (the value SQLite shows an auto-assigned key as inside a BEFORE trigger) refused" "reg_ids" "$M (-1,'x','manual')"
  expect_reject "$DB" "R01-4c INSERT OR REPLACE with an explicit seq refused" "reg_ids" "INSERT OR REPLACE INTO reg_ids(seq,minted_by,mint_basis) VALUES (9,'x','manual')"
  assert_eq "R01-5 none of the refused inserts left a row (still CAT-001..003)" "$(sql "$DB" "select group_concat(atm_id) from (select atm_id from reg_ids order by seq)")" "CAT-001,CAT-002,CAT-003"
  expect_ok "$DB" "R01-6 control: the next engine-assigned mint is accepted" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('y','manual')"
  assert_eq "R01-6b ... and is CAT-004" "$(sql "$DB" "select atm_id from reg_ids order by seq desc limit 1")" "CAT-004"
  # sqlite_sequence is an ordinary writable table: raising it makes the next AUTOINCREMENT mint skip numbers (a gap)
  sql "$DB" "UPDATE sqlite_sequence SET seq=100 WHERE name='reg_ids'" >/dev/null
  expect_reject "$DB" "R01-7 a mint after sqlite_sequence was raised (would be CAT-101, a gap) refused" "reg_ids" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('z','manual')"
  # the dump / restore path: a restored dump loads its rows (explicit seq) BEFORE the triggers exist, and must still load
  RD="$D/r01-dump.sql"; "$SQLITE3" "$DB" .dump | grep -v '^PRAGMA' >"$RD"
  rm -f "$D/r01-restored.db"; "$SQLITE3" "$D/r01-restored.db" ".read $RD" >/dev/null 2>"$D/r01-restore.err"
  assert_eq "R01-8 a dump of a clean register restores (explicit seq loads before the triggers) and holds CAT-001..004" "$(sql "$D/r01-restored.db" "select group_concat(atm_id) from (select atm_id from reg_ids order by seq)")" "CAT-001,CAT-002,CAT-003,CAT-004"
  # gate: forged DBs (the trigger dropped, the rows planted, the trigger restored by re-applying the DDL, as fixture B2 does)
  good_db "$D/r01g1.db"; sql "$D/r01g1.db" "DROP TRIGGER IF EXISTS reg_ids_mint_monotone; INSERT INTO reg_ids(seq,minted_by,mint_basis) VALUES (9,'f','manual'); INSERT INTO reg_ids(seq,minted_by,mint_basis) VALUES (5,'f','manual'); INSERT INTO reg_ids(seq,minted_by,mint_basis) VALUES (0,'f','manual'); INSERT INTO reg_ids(seq,minted_by,mint_basis) VALUES (-3,'f','manual');" >/dev/null; reapply "$D/r01g1.db"
  must_fail "R01-G1 rows CAT--03, CAT-000, CAT-005, CAT-009 (out of order, gaps, malformed) fail the gate naming v_ids_malformed" "$D/r01g1.db" 'FAIL v_ids_malformed rows='
  good_db "$D/r01g2.db"; sql "$D/r01g2.db" "DROP TRIGGER IF EXISTS reg_ids_mint_monotone; UPDATE sqlite_sequence SET seq=100 WHERE name='reg_ids'; INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('f','manual');" >/dev/null; reapply "$D/r01g2.db"
  must_fail "R01-G2 a gap made through sqlite_sequence (CAT-001 then CAT-101) fails the gate naming v_ids_malformed" "$D/r01g2.db" 'FAIL v_ids_malformed rows='
  good_db "$D/r01g3.db"; sql "$D/r01g3.db" "DROP TRIGGER reg_ids_mint_monotone" >/dev/null 2>&1;
  must_fail "R01-G3 the monotone trigger dropped (not restored) fails the gate: missing object / schema differs" "$D/r01g3.db" 'FAIL .*(reg_ids_mint_monotone|v_gate_missing_objects|unexpected schema object|schema object of the reference missing)'
  good_db "$D/r01g4.db"; out=$(run_gate "$D/r01g4.db"); printf '%s' "$out" | grep -q '^GATE OK' && ok "R01-G4 golden-FALSE: a clean one-item register passes" || bad "R01-G4 [$out]"
  # class members that were already closed (recorded so the class inventory is complete): an item with a grammar-bad id and no mint
  good_db "$D/r01c.db"
  expect_reject "$D/r01c.db" "R01-C1 raw items insert of CAT-000 without a mint refused (trg_items_require_mint)" "no reg_ids row" "INSERT INTO items SELECT 'CAT-000', current_location, representation, $(sql "$D/r01c.db" "select group_concat(name) from pragma_table_info('items') where name not in ('atm_id','current_location','representation')") FROM items LIMIT 1"
  out=$("$WI" add Task Low --db "$D/r01c.db" --prefix CAT --title "auto" --description "$DESC" 2>&1); [ $? -ne 0 ] && ok "R01-C3 the engine add with an auto-generated id and no mint behind it is refused (the engine's own id allocation is not a mint path)" || bad "R01-C3 [$out]"
  out=$("$WI" add Task Low --db "$D/r01c.db" --id CAT--03 --prefix CAT --title "t" --description "$DESC" 2>&1); [ $? -ne 0 ] && ok "R01-C2 the engine add with a grammar-bad, never-minted id CAT--03 is refused" || bad "R01-C2 [$out]"
fi

if want R03; then echo "== R-03: a retried import mint cannot create a second id; orphan import ids fail the gate =="
  DB="$D/r03.db"; fresh_ext_db "$DB" || { bad "R03 setup"; finish; exit 1; }
  expect_ok "$DB" "R03-1 control: the first import mint for a marker is accepted" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('importer:e1','import')"
  expect_reject "$DB" "R03-2 the same marker minted again (a retry after an unknown-status write) refused by the database" "UNIQUE|uq_reg_ids_import_marker|import marker" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('importer:e1','import')"
  expect_ok "$DB" "R03-3 control: the same minted_by text is allowed for a non-import basis (the guard is scoped to import)" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('importer:e1','manual')"
  assert_eq "R03-4 only the legitimate rows exist (2)" "$(sql "$DB" "select count(*) from reg_ids")" 2
  good_db "$D/r03g1.db"; sql "$D/r03g1.db" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('importer:e9','import')" >/dev/null
  must_fail "R03-G1 an import-minted id with no item and no source mapping (an orphan) fails the gate naming v_import_mint_orphans" "$D/r03g1.db" 'FAIL v_import_mint_orphans rows='
  good_db "$D/r03g2.db"; sql "$D/r03g2.db" "DROP INDEX IF EXISTS uq_reg_ids_import_marker; INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('importer:e7','import'); INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('importer:e7','import');" >/dev/null
  must_fail "R03-G2 two ids for one import marker (the index dropped, a retry duplicated the mint) fail the gate" "$D/r03g2.db" 'FAIL v_import_mint_orphans rows=|FAIL .*uq_reg_ids_import_marker|schema object of the reference missing'
  good_db "$D/r03g3.db"; out=$(run_gate "$D/r03g3.db"); printf '%s' "$out" | grep -q '^GATE OK' && ok "R03-G3 golden-FALSE: a manual mint with an item passes" || bad "R03-G3 [$out]"
  good_db "$D/r03g4.db"; sql "$D/r03g4.db" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('importer:e5','import')" >/dev/null; "$WI" add Task Low --db "$D/r03g4.db" --id CAT-002 --prefix CAT --title "t2" --description "$DESC" >/dev/null 2>&1
  out=$(run_gate "$D/r03g4.db"); printf '%s' "$out" | grep -q '^GATE OK' && ok "R03-G4 golden-FALSE: an import-minted id that has its item passes" || bad "R03-G4 [$out]"
fi

if want R04; then echo "== R-04: the CAT prefix must agree across .helix/reporting.yaml, the DDL and the import scripts =="
  PR="$D/pfx"; mkdir -p "$PR/.helix" "$PR/scripts/register"
  mkpfx() {  # mkpfx NAME -> a REG_ROOT that holds the three sources of the prefix, copied from the repository
    local r="$D/pfx-$1"; rm -rf "$r"; mkdir -p "$r/.helix" "$r/scripts/register" "$r/constitution/scripts/workable-items/bin"
    cp "$ROOT/.helix/reporting.yaml" "$r/.helix/"; cp "$REG_DIR/import_plan.py" "$REG_DIR/import_tickets.py" "$r/scripts/register/"; cp "${REG_EXT_SQL:-$REG_DIR/register_ext.sql}" "$r/scripts/register/register_ext.sql"   # the DDL UNDER TEST (R2-03: a mutant DDL must not fail by artifact)
    cp "$ROOT/constitution/scripts/workable-items/bin/workable-items-linux" "$r/constitution/scripts/workable-items/bin/"; echo "$r"; }
  good_db "$D/r04.db"
  r=$(mkpfx ok); out=$(REG_ROOT=$r REG_EXT_SQL=$r/scripts/register/register_ext.sql bash "$GATE" --db "$D/r04.db" 2>&1); printf '%s' "$out" | grep -q '^GATE OK' && printf '%s' "$out" | grep -q 'INFO prefix_agreement' && ok "R04-0 control: the three sources agree on CAT, the gate passes and reports the check" || bad "R04-0 [$out]"
  r=$(mkpfx yaml); sed -i 's/^\( *id_prefix: *\)CAT/\1DOG/' "$r/.helix/reporting.yaml"; out=$(REG_ROOT=$r bash "$GATE" --db "$D/r04.db" 2>&1)
  printf '%s' "$out" | grep -q 'FAIL prefix_agreement' && ! printf '%s' "$out" | grep -q 'GATE OK' && ok "R04-1 reporting.yaml id_prefix DOG against CAT in the DDL fails the gate" || bad "R04-1 [$out]"
  r=$(mkpfx import1); sed -i 's/"--prefix", "CAT"/"--prefix", "DOG"/' "$r/scripts/register/import_plan.py"; out=$(REG_ROOT=$r bash "$GATE" --db "$D/r04.db" 2>&1)
  printf '%s' "$out" | grep -q 'FAIL prefix_agreement' && ok "R04-2 import_plan.py --prefix DOG fails the gate" || bad "R04-2 [$out]"
  r=$(mkpfx import2); sed -i 's/"--prefix", "CAT"/"--prefix", "DOG"/' "$r/scripts/register/import_tickets.py"; out=$(REG_ROOT=$r bash "$GATE" --db "$D/r04.db" 2>&1)
  printf '%s' "$out" | grep -q 'FAIL prefix_agreement' && ok "R04-3 import_tickets.py --prefix DOG fails the gate" || bad "R04-3 [$out]"
  r=$(mkpfx ddl); sed -i "s/'CAT-' || printf/'DOG-' || printf/" "$r/scripts/register/register_ext.sql"; out=$(REG_ROOT=$r REG_EXT_SQL=$r/scripts/register/register_ext.sql bash "$GATE" --db "$D/r04.db" 2>&1)
  printf '%s' "$out" | grep -q 'FAIL prefix_agreement' && ok "R04-4 the DDL literal DOG- against reporting.yaml CAT fails the gate" || bad "R04-4 [$out]"
  r=$(mkpfx noyaml); rm -f "$r/.helix/reporting.yaml"; out=$(REG_ROOT=$r bash "$GATE" --db "$D/r04.db" 2>&1)
  printf '%s' "$out" | grep -q 'FAIL prefix_agreement' && ok "R04-5 an absent reporting.yaml fails closed (a missing source is not agreement)" || bad "R04-5 [$out]"
  r=$(mkpfx noimport); rm -f "$r/scripts/register/import_plan.py"; out=$(REG_ROOT=$r bash "$GATE" --db "$D/r04.db" 2>&1)
  printf '%s' "$out" | grep -q 'FAIL prefix_agreement' && ok "R04-6 an absent import script fails closed" || bad "R04-6 [$out]"
  r=$(mkpfx noprefixfound); sed -i 's/"--prefix", "CAT"/"--prefix", PREFIX_VAR/' "$r/scripts/register/import_tickets.py"; out=$(REG_ROOT=$r bash "$GATE" --db "$D/r04.db" 2>&1)
  printf '%s' "$out" | grep -q 'FAIL prefix_agreement' && ok "R04-7 an import script whose --prefix is not a literal fails closed (zero matches is blindness, not agreement)" || bad "R04-7 [$out]"
  # the column name stays: the engine owns the atm_id -> ticket_id rename
  assert_eq "R04-8 the engine column atm_id is still the key of items and reg_ids (no local rename)" "$(sql "$D/r04.db" "select count(*) from pragma_table_xinfo('reg_ids') where name='atm_id'")/$(sql "$D/r04.db" "select count(*) from pragma_table_xinfo('items') where name='atm_id'")" "1/1"
fi

if want R05; then echo "== R-05: a tracker_receipt that cites a FILE is re-hashed =="
  RR="$D/r05root"; mkdir -p "$RR/ev"; printf 'receipt one\n' >"$RR/ev/rcpt1.txt"
  good_db "$D/r05.db"; ID=$(sql "$D/r05.db" "select atm_id from items limit 1")
  RH=$(sha256sum "$RR/ev/rcpt1.txt" | cut -d' ' -f1); RS=$(stat -c %s "$RR/ev/rcpt1.txt")
  sql "$D/r05.db" "INSERT INTO reg_evidence(atm_id,kind,evidence_class,path,sha256,size_bytes,produced_by,captured_at) VALUES ('$ID','tracker_receipt','source','ev/rcpt1.txt','$RH',$RS,'dev','2026-10-05T00:00:00Z')" >/dev/null
  GATE_ROOT="$RR"; cp -a "$ROOT/.helix" "$RR/" 2>/dev/null; mkdir -p "$RR/scripts/register" "$RR/constitution/scripts/workable-items/bin"; cp "$REG_DIR/import_plan.py" "$REG_DIR/import_tickets.py" "$REG_DIR/register_ext.sql" "$RR/scripts/register/"; cp "$ROOT/constitution/scripts/workable-items/bin/workable-items-linux" "$RR/constitution/scripts/workable-items/bin/"
  out=$(REG_ROOT=$RR bash "$GATE" --db "$D/r05.db" 2>&1); printf '%s' "$out" | grep -q '^GATE OK' && printf '%s' "$out" | grep -q 'evidence_rehash files=1' && ok "R05-0 control: a tracker_receipt file that re-hashes passes and is counted" || bad "R05-0 [$out]"
  printf 'receipt one, forged\n' >"$RR/ev/rcpt1.txt"
  out=$(REG_ROOT=$RR bash "$GATE" --db "$D/r05.db" 2>&1); rc=$?
  [ $rc -ne 0 ] && printf '%s' "$out" | grep -Eq 'FAIL evidence [0-9]+: sha256 mismatch for ev.?rcpt1.txt' && ok "R05-1 a changed tracker_receipt file fails the gate (sha256 mismatch)" || bad "R05-1 rc=$rc [$out]"
  rm -f "$RR/ev/rcpt1.txt"; out=$(REG_ROOT=$RR bash "$GATE" --db "$D/r05.db" 2>&1); rc=$?
  [ $rc -ne 0 ] && printf '%s' "$out" | grep -Eq 'FAIL evidence [0-9]+: cited file absent: ev.?rcpt1.txt' && ok "R05-2 an absent tracker_receipt file fails the gate" || bad "R05-2 rc=$rc [$out]"
  GATE_ROOT=""
fi

if want R06; then echo "== R-06: the registry cross-check of step_view_empty has an isolated fixture =="
  good_db "$D/r06.db"
  sql "$D/r06.db" "INSERT INTO reg_gate_checks VALUES ('v_duplicate_item','view_empty')" >/dev/null
  out=$(run_gate "$D/r06.db"); printf '%s' "$out" | grep -q 'FAIL registry row not in the reference: v_duplicate_item$' && ok "R06-1 a forged registry row that is only a SUBSTRING of a reference view name is reported by the cross-check (exact-line match)" || bad "R06-1 [$out]"
fi

if want R02 || want R10; then
  RP=${REPLAY:-$REG_DIR/replay.sh}
  mkrep() {  # mkrep NAME -> scratch root holding a stub register DB; BACKUP stub fails after leaving a foreign file in the output dir
    local r; r=$(mkroot "rp-$1"); echo "stub db" >"$r/docs/workable_items.db"; mkdir -p "$r/.audit/register"; echo "$r"; }
  BK="$T_SCR/stub_backup.sh"
  cat >"$BK" <<'S'
#!/usr/bin/env bash
# stub BACKUP: proves the run got past journal validation; leaves a foreign file in the output dir, then fails (backup_failed)
[ -n "${REPLAY_FOREIGN_DIR:-}" ] && [ -d "$REPLAY_FOREIGN_DIR" ] && echo foreign >"$REPLAY_FOREIGN_DIR/foreign.txt"
echo "stub backup failure" >&2; exit 9
S
  chmod +x "$BK"
  rp() { local r=$1; shift; LOCKED_TEST_MODE=1 LOCKED_ROOT="$r" LOCKED=/bin/false BACKUP="$BK" bash "$RP" "$@"; }
  SINCE=$(printf 'a%.0s' $(seq 64)); SB=$(printf 'b%.0s' $(seq 64)); SC=$(printf 'c%.0s' $(seq 64))
  # a complete register row (every field locked.sh writes); the base row, then a changing row
  jfull() { python3 -I - "$@" <<'PY'
import json,sys
seq,op,sb,sa=int(sys.argv[1]),sys.argv[2],sys.argv[3],sys.argv[4]
print(json.dumps({"seq":seq,"op_id":op,"mode":"register","argv":["sh","-c","x"],"stdin":None,"inputs":[],"input_args":{},"exit":0,"db":"docs/workable_items.db","db_sha_before":sb,"db_sha_after":sa,"wal_bytes_before":0,"wal_bytes_after":0,"ids_snapshot":"ok","ids_minted":[],"time":"2026-10-08T00:00:00.000000Z","container_drain":"none","container_exit_source":"container_event"},sort_keys=True))
PY
  }
  jwrite() { local f=$1; shift; : >"$f"; local l; for l in "$@"; do printf '%s\n' "$l" >>"$f"; done; }
fi
if want R02; then echo "== R-02: replay.sh refuses a journal row with a missing field (journal_row_incomplete) instead of reading it as its safe value =="
  R=$(mkrep r02); J="$R/.audit/register/journal.jsonl"
  jwrite "$J" "$(jfull 1 base $SB $SINCE)" "$(jfull 2 next $SINCE $SC)"
  out=$(rp "$R" --onto docs/workable_items.db --since "$SINCE" --out "$T_SCR/r02ctl" --journal "$J" 2>&1); rc=$?
  [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'reason=backup_failed' && ok "R02-0 control: a complete journal passes validation and reaches the backup step (stub failure)" || bad "R02-0 rc=$rc [$out]"
  for f in seq op_id mode argv db db_sha_before db_sha_after ids_minted ids_snapshot wal_bytes_before wal_bytes_after exit input_args; do
    L2=$(jfull 2 next $SINCE $SC | python3 -I -c "import json,sys;r=json.loads(sys.stdin.read());r.pop('$f');print(json.dumps(r,sort_keys=True))")
    jwrite "$J" "$(jfull 1 base $SB $SINCE)" "$L2"
    out=$(rp "$R" --onto docs/workable_items.db --since "$SINCE" --out "$T_SCR/r02-$f" --journal "$J" 2>&1); rc=$?
    [ $rc -eq 20 ] && printf '%s' "$out" | grep -q "reason=journal_row_incomplete" && printf '%s' "$out" | grep -q "$f" && [ ! -e "$T_SCR/r02-$f" ] && ok "R02-$f a row without $f is refused journal_row_incomplete naming the field, nothing written" || bad "R02-$f rc=$rc [$out]"
  done
  # the base row itself (the row whose db_sha_after is --since) is validated too
  L1=$(jfull 1 base $SB $SINCE | python3 -I -c "import json,sys;r=json.loads(sys.stdin.read());r.pop('wal_bytes_after');print(json.dumps(r,sort_keys=True))")
  jwrite "$J" "$L1" "$(jfull 2 next $SINCE $SC)"
  out=$(rp "$R" --onto docs/workable_items.db --since "$SINCE" --out "$T_SCR/r02-base" --journal "$J" 2>&1); rc=$?
  [ $rc -eq 20 ] && printf '%s' "$out" | grep -q "reason=journal_row_incomplete" && ok "R02-base the base row without wal_bytes_after is refused (absent was read as 0 = an empty -wal)" || bad "R02-base rc=$rc [$out]"
  # malformed values, not only absent ones
  vi=0; for spec in "wal_bytes_after=\"0\"" "wal_bytes_after=true" "wal_bytes_before=-1" "ids_minted=\"CAT-001\"" "ids_snapshot=\"maybe\"" "exit=\"0\"" "seq=\"2\"" "mode=\"bogus\"" "db_sha_after=\"xyz\"" "db_sha_before=\"A$(printf 'b%.0s' $(seq 63))\"" "argv=\"sh\""; do
    k=${spec%%=*}; v=${spec#*=}; vi=$((vi+1))
    L2=$(jfull 2 next $SINCE $SC | python3 -I -c "import json,sys;r=json.loads(sys.stdin.read());r['$k']=json.loads('''$v''');print(json.dumps(r,sort_keys=True))")
    jwrite "$J" "$(jfull 1 base $SB $SINCE)" "$L2"
    out=$(rp "$R" --onto docs/workable_items.db --since "$SINCE" --out "$T_SCR/r02v-$vi" --journal "$J" 2>&1); rc=$?
    [ $rc -eq 20 ] && printf '%s' "$out" | grep -q "reason=journal_row_incomplete" && ok "R02-v$vi-$k a $k of the wrong type or shape ($v) is refused journal_row_incomplete" || bad "R02-v$vi-$k rc=$rc [$out]"
  done
  # a null hash is legitimate only as a recorded value (locked.sh writes null when the file did not exist): present-and-null passes
  L2=$(jfull 2 next $SINCE $SC | python3 -I -c "import json,sys;r=json.loads(sys.stdin.read());r['db_sha_before']=None;print(json.dumps(r,sort_keys=True))")
  jwrite "$J" "$(jfull 1 base $SB $SINCE)" "$L2"
  out=$(rp "$R" --onto docs/workable_items.db --since "$SINCE" --out "$T_SCR/r02-null" --journal "$J" 2>&1); rc=$?
  printf '%s' "$out" | grep -q 'reason=journal_row_incomplete' && bad "R02-null golden-FALSE: a recorded null db_sha_before was refused as incomplete [$out]" || ok "R02-null golden-FALSE: a field that is present with the value null is a recorded fact, not an absent field"
fi

if want R10; then echo "== R-10: replay.sh never deletes an output directory (or a file in it) it did not create =="
  R=$(mkrep r10); J="$R/.audit/register/journal.jsonl"; jwrite "$J" "$(jfull 1 base $SB $SINCE)" "$(jfull 2 next $SINCE $SC)"
  mkdir -p "$T_SCR/r10pre"
  out=$(REPLAY_FOREIGN_DIR="$T_SCR/r10pre" rp "$R" --onto docs/workable_items.db --since "$SINCE" --out "$T_SCR/r10pre" --journal "$J" 2>&1); rc=$?
  [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'reason=backup_failed' && [ -d "$T_SCR/r10pre" ] && ok "R10-1 a pre-existing EMPTY output directory still exists after a refusal (the run did not create it)" || bad "R10-1 rc=$rc dir=$([ -d "$T_SCR/r10pre" ] && echo kept || echo DELETED) [$out]"
  [ -f "$T_SCR/r10pre/foreign.txt" ] && ok "R10-2 a file another process wrote into the directory survives the refusal" || bad "R10-2 the foreign file was deleted"
  out=$(REPLAY_FOREIGN_DIR="$T_SCR/r10new" rp "$R" --onto docs/workable_items.db --since "$SINCE" --out "$T_SCR/r10new" --journal "$J" 2>&1); rc=$?
  [ $rc -eq 20 ] && [ ! -e "$T_SCR/r10new" ] && ok "R10-3 golden-FALSE: a directory the run created itself is removed on refusal (nothing left behind)" || bad "R10-3 rc=$rc [$out] exists=$([ -e "$T_SCR/r10new" ] && echo yes || echo no)"
  mkdir -p "$T_SCR/r10own"
  # the run's own outputs inside a pre-existing directory are cleaned: the stub writes backup.json like the real helper does
  cat >"$T_SCR/stub_backup2.sh" <<'S'
#!/usr/bin/env bash
# stub BACKUP that behaves like the helper up to the point of failure: it writes its record file into the output dir (the run's OWN output), then fails
echo '{}' >"$REPLAY_OUTDIR/backup.json"; exit 9
S
  chmod +x "$T_SCR/stub_backup2.sh"
  out=$(REPLAY_OUTDIR="$T_SCR/r10own" LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED=/bin/false BACKUP="$T_SCR/stub_backup2.sh" bash "$RP" --onto docs/workable_items.db --since "$SINCE" --out "$T_SCR/r10own" --journal "$J" 2>&1); rc=$?
  [ $rc -eq 20 ] && [ -d "$T_SCR/r10own" ] && [ ! -e "$T_SCR/r10own/backup.json" ] && ok "R10-4 in a pre-existing directory the run removes the outputs it wrote itself (backup.json) and keeps the directory" || bad "R10-4 rc=$rc [$out] ls=$(ls "$T_SCR/r10own" 2>&1 | tr '\n' ' ')"
fi

if want R11; then echo "== R-11: locked.sh --out runs fence an orphan writer too =="
  STUB_DIR="$T_SCR/stub11"; export STUB_DIR; mkdir -p "$STUB_DIR"
  cat >"$T_SCR/stub11_runp.sh" <<'S'
#!/usr/bin/env bash
echo "$* " >>"$STUB_DIR/runp.log"; case "$*" in *-snap*) echo "CAT-001";; esac; exit 0
S
  chmod +x "$T_SCR/stub11_runp.sh"
  cat >"$T_SCR/stub11_podman.sh" <<'S'
#!/usr/bin/env bash
echo "$*" >>"$STUB_DIR/podman.log"
[ -f "$STUB_DIR/podman.fail" ] && exit 125
exit 0
S
  chmod +x "$T_SCR/stub11_podman.sh"
  mkmarker() { mkdir -p "$1/.audit/register/pending"; printf '{"op_id":"%s","pid":4194000}\n' "$2" >"$1/.audit/register/pending/$2"; }
  olk() { env LOCKED_TEST_MODE=1 LOCKED_ROOT="$1" LOCKED_RUNP="$T_SCR/stub11_runp.sh" LOCKED_PODMAN="$T_SCR/stub11_podman.sh" LOCKED_CONTAINER_WAIT=2 STUB_DIR="$STUB_DIR" bash "${LOCKED:-$REG_DIR/locked.sh}" "${@:2}"; }
  R=$(mkroot r11a); echo "stub db" >"$R/docs/workable_items.db"; mkmarker "$R" dead11; : >"$STUB_DIR/podman.fail"; mkdir -p "$T_SCR/r11o"
  out=$(olk "$R" --out "$T_SCR/r11o" --op-id r11a -- sh -c 'w' 2>&1); rc=$?
  [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'reason=writer_state_unverifiable' && ! grep -q 'r11a ' "$STUB_DIR/runp.log" 2>/dev/null && ok "R11-1 an --out run with a pending marker whose writer state cannot be verified is refused (20 writer_state_unverifiable), the command is not run" || bad "R11-1 rc=$rc [$out] runp=$(cat "$STUB_DIR/runp.log" 2>/dev/null | head -2 | tr '\n' '|')"
  rm -f "$STUB_DIR/podman.fail"
  R=$(mkroot r11b); echo "stub db" >"$R/docs/workable_items.db"; mkdir -p "$T_SCR/r11o2"
  out=$(olk "$R" --out "$T_SCR/r11o2" --op-id r11b -- sh -c 'w' 2>&1); rc=$?
  [ $rc -eq 0 ] && ok "R11-2 golden-FALSE: an --out run with NO pending marker is not delayed or refused (rc 0)" || bad "R11-2 rc=$rc [$out]"
  R=$(mkroot r11c); echo "stub db" >"$R/docs/workable_items.db"; mkmarker "$R" gone11; mkdir -p "$T_SCR/r11o3"
  out=$(olk "$R" --out "$T_SCR/r11o3" --op-id r11c -- sh -c 'w' 2>&1); rc=$?
  [ $rc -eq 0 ] && ok "R11-3 golden-FALSE: an --out run with a pending marker whose client is gone and whose container does not exist proceeds (the fence proved the writer finished)" || bad "R11-3 rc=$rc [$out]"
fi
finish
