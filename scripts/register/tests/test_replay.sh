#!/usr/bin/env bash
# test_replay.sh - T067a (docs/04 section 12.2 single writer across clones): scripts/register/replay.sh. RED while absent.
# Fixtures: two scratch clones diverged from one base (a local one holding the journal, a remote-side one holding the remote database).
# Env REPLAY substitutes a mutant copy (mutate_register_ops.sh). Container leg: see clib.sh.
. "$(dirname "$0")/lib.sh"; . "$(dirname "$0")/clib.sh"
evhead T067a
if [ ! -f "$REPLAY" ]; then bad "T0 replay script absent: $REPLAY"; finish; exit 1; fi
BASE=$(mkroot base); cdb "$BASE" workable_items.db || { bad setup; finish; exit 1; }
for i in 1 2; do lk "$BASE" -- sh -c "$(mint_cmd workable_items.db)" >/dev/null 2>&1 || { bad "setup mint"; finish; exit 1; }; done
SINCE=$(tail -n1 "$BASE/.audit/register/journal.jsonl" | python3 -I -c 'import json,sys;print(json.load(sys.stdin)["db_sha_after"])')
[ "$SINCE" = "$(fsha "$BASE/docs/workable_items.db")" ] && ok "P0 the base state hash is the last journal row's db_sha_after ($SINCE)" || bad "P0 journal hash $SINCE != $(fsha "$BASE/docs/workable_items.db")"
LOCAL="$T_SCR/local"; REMOTE="$T_SCR/remote"; REMOTE2="$T_SCR/remote2"; cp -a "$BASE" "$LOCAL"; cp -a "$BASE" "$REMOTE"; cp -a "$BASE" "$REMOTE2"
DB=docs/workable_items.db
# local side: a mint, a raw edit through a recorded input file, a failing command, a read, an --out run, an export (regeneration)
lk "$LOCAL" -- sh -c "$(mint_cmd workable_items.db)" >/dev/null 2>&1
echo "UPDATE items SET title='from-file' WHERE atm_id='CAT-001';" >"$LOCAL/.audit/upd.sql"
lk "$LOCAL" -- sqlite3 /src/$DB ".read /src/.audit/upd.sql" >/dev/null 2>&1; assert_eq "P1 local file-input edit applied" "$(lk "$LOCAL" --out "$T_SCR/q1" -- sqlite3 -readonly "file:/src/$DB?immutable=1" "select title from items where atm_id='CAT-001'" 2>&1)" "from-file"
lk "$LOCAL" -- sh -c 'exit 7' >/dev/null 2>&1
lk "$LOCAL" -- sqlite3 -readonly "file:/src/$DB?immutable=1" "select 1" >/dev/null 2>&1
tool "$LOCAL" "$EXPORT" --db $DB --out-dir docs/register >/dev/null 2>&1
# remote side: a different edit of CAT-002 (no mint)
lk "$REMOTE" -- sh -c "$WI_IN update --id CAT-002 --db /src/$DB --title remote-edit" >/dev/null 2>&1
# remote2: the remote side minted its own CAT-003 for another item (an id collision with the local mint)
lk "$REMOTE2" -- sh -c "$(mint_cmd workable_items.db)" >/dev/null 2>&1
JL="$LOCAL/.audit/register/journal.jsonl"
rp() { local r=$1; shift; tool "$r" "$REPLAY" "$@"; }
echo "== the replay of local mints, edits and closures onto the remote side =="
O1="$T_SCR/out1"; out=$(rp "$REMOTE" --onto $DB --since "$SINCE" --out "$O1" --journal "$JL" 2>&1); rc=$?
[ $rc -eq 0 ] && printf '%s' "$out" | grep -q 'gate=GATE OK' && ok "P2 replay exits 0 and the gate on the replayed database prints GATE OK" || { bad "P2 rc=$rc [$out]"; finish; exit 1; }
q() { lk "$REMOTE" --out "$O1" -- sqlite3 -readonly "file:/out/replay.db?immutable=1" "$1" 2>&1; }
assert_eq "P3 the replayed database holds every reg_ids id of both sides (CAT-001..003)" "$(q "select group_concat(atm_id) from (select atm_id from reg_ids order by seq)")" "CAT-001,CAT-002,CAT-003"
assert_eq "P4 the local mint was replayed: CAT-003 is an item" "$(q "select count(*) from items where atm_id='CAT-003'")" 1
assert_eq "P5 the local file-input edit was replayed (CAT-001 title)" "$(q "select title from items where atm_id='CAT-001'")" "from-file"
assert_eq "P6 the remote side's own edit is kept (CAT-002 title)" "$(q "select title from items where atm_id='CAT-002'")" "remote-edit"
python3 -I - "$O1/replay-report.json" <<'PY' && ok "P7 the report lists 2 rows replayed and the skipped rows with their reasons" || bad "P7 report: $(cat "$O1/replay-report.json" | head -c 600)"
import json,sys
r=json.load(open(sys.argv[1])); reasons=sorted(s["reason"] for s in r["rows_skipped"])
assert len(r["rows_replayed"])==2, r["rows_replayed"]
for want in ("command_failed","not_a_register_write","regenerated_by_replay"): assert want in reasons, (want,reasons)
assert r["ids_local_kept"]==["CAT-003"] and r["gate"]=="GATE OK"
PY
[ "$(fsha "$REMOTE/$DB")" != "$(fsha "$O1/replay.db")" ] && ok "P8 the remote checkout's own database was not rewritten (the result is only in <dir>)" || bad "P8"
tool "$REMOTE" "$DUMP" --out-dir "$O1" --db-file replay.db --out again.sql >/dev/null 2>&1; assert_eq "P9 the dump of the replayed database equals a regenerated dump (register.sql)" "$(fsha "$O1/again.sql")" "$(fsha "$O1/register.sql")"
out=$(tool "$REMOTE" "$EXPORT" --out-mode "$O1" --db-file replay.db --check 2>&1); [ $? -eq 0 ] && ok "P10 the exports in <dir>/export equal the regenerated ones (export --check OK)" || bad "P10 [$out]"
[ ! -s "$REMOTE/docs/register/register.sql" ] && ok "P11 nothing was written into the remote checkout's docs/register" || bad "P11"
echo "== refusals =="
O2="$T_SCR/out2"; sha_r2=$(fsha "$REMOTE2/$DB"); out=$(rp "$REMOTE2" --onto $DB --since "$SINCE" --out "$O2" --journal "$JL" 2>&1); rc=$?
[ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'reason=replay_id_collision' && printf '%s' "$out" | grep -q 'CAT-003' && ok "P12 a local mint whose id the remote side holds for another item is refused replay_id_collision, the id named" || bad "P12 rc=$rc [$out]"
[ ! -e "$O2" ] && [ "$(fsha "$REMOTE2/$DB")" = "$sha_r2" ] && ok "P13 nothing written: no output directory, the remote database unchanged" || bad "P13"
O3="$T_SCR/out3"; out=$(rp "$REMOTE" --onto $DB --since "$(printf '0%.0s' $(seq 64))" --out "$O3" --journal "$JL" 2>&1); rc=$?
[ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'reason=since_not_found' && [ ! -e "$O3" ] && ok "P14 a --since that matches no journal row is refused since_not_found" || bad "P14 rc=$rc [$out]"
out=$(rp "$REMOTE" --onto $DB --since "not-a-hash" --out "$T_SCR/out4" --journal "$JL" 2>&1); [ $? -eq 20 ] && printf '%s' "$out" | grep -q since_malformed && ok "P15 a malformed --since is refused" || bad "P15 [$out]"
mkdir -p "$T_SCR/jd"; cp -a "$LOCAL/.audit/register/journal.jsonl" "$T_SCR/jd/"; mkdir -p "$T_SCR/jd/inputs"
out=$(rp "$REMOTE" --onto $DB --since "$SINCE" --out "$T_SCR/out5" --journal "$T_SCR/jd/journal.jsonl" 2>&1); rc=$?
[ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'reason=input_missing' && [ ! -e "$T_SCR/out5" ] && ok "P16 a recorded input that is missing is refused input_missing" || bad "P16 rc=$rc [$out]"
mkdir -p "$T_SCR/out6"; echo x >"$T_SCR/out6/f"; out=$(rp "$REMOTE" --onto $DB --since "$SINCE" --out "$T_SCR/out6" --journal "$JL" 2>&1); [ $? -eq 20 ] && printf '%s' "$out" | grep -q out_dir_not_empty && [ -f "$T_SCR/out6/f" ] && ok "P17 a non-empty output directory is refused and left untouched" || bad "P17 [$out]"
out=$(rp "$REMOTE" --onto docs/other.db --since "$SINCE" --out "$T_SCR/out7" --journal "$JL" 2>&1); [ $? -eq 20 ] && printf '%s' "$out" | grep -q onto_not_register_database && ok "P18 --onto that is not this checkout's register database is refused" || bad "P18 [$out]"
rp "$REMOTE" --bogus >/dev/null 2>&1; assert_eq "P19 unknown argument: exit 2" "$?" 2
echo "== export.sh --install rows (T175a) are listed, not replayed =="
mkdir -p "$T_SCR/jd2/inputs"; python3 -I - "$JL" "$T_SCR/jd2/journal.jsonl" <<'PY'
import json,sys
rows=[json.loads(l) for l in open(sys.argv[1]) if l.strip()]
# SYNTHETIC row (the installer of T175a does not exist yet): a register row whose argv names export.sh --install
rows.append({"time":"2026-10-06T00:00:00.000000Z","op_id":"synthetic-install","mode":"register","argv":["bash","scripts/register/export.sh","--install","b1"],"exit":0,
 "db_sha_before":"a"*64,"db_sha_after":"a"*64,"ids_minted":[],"input_args":{},"inputs":[],"wal_bytes_after":0,"stdin":None,"db":"docs/workable_items.db","ids_snapshot":"ok"})   # UNCHANGED (WF15 M6: an install row that changed the register is refused)
open(sys.argv[2],"w").write("".join(json.dumps(r)+"\n" for r in rows))
PY
cp -a "$LOCAL/.audit/register/inputs/." "$T_SCR/jd2/inputs/" 2>/dev/null
O8="$T_SCR/out8"; out=$(rp "$REMOTE" --onto $DB --since "$SINCE" --out "$O8" --journal "$T_SCR/jd2/journal.jsonl" 2>&1); rc=$?
[ $rc -eq 0 ] && grep -q '"install_replay_owed_T175a"' "$O8/replay-report.json" && ok "P20 an export.sh --install row is listed as skipped install_replay_owed_T175a (UNCONFIRMED: the installer is not written)" || bad "P20 rc=$rc [$out]"
echo "== a replayed row that breaks the register is refused by the gate =="
mkdir -p "$T_SCR/jd3/inputs"; python3 -I - "$JL" "$T_SCR/jd3/journal.jsonl" <<'PY'
import json,sys
rows=[json.loads(l) for l in open(sys.argv[1]) if l.strip()]
# SYNTHETIC golden-bad row: drops a guard trigger of the register (the same fault as B3 of test_gate.sh)
rows.append({"time":"2026-10-06T00:00:00.000000Z","op_id":"synthetic-drop-trigger","mode":"register","argv":["sqlite3","/src/docs/workable_items.db","DROP TRIGGER reg_ids_no_delete"],"exit":0,
 "db_sha_before":"c"*64,"db_sha_after":"d"*64,"ids_minted":[],"input_args":{},"inputs":[],"wal_bytes_after":0,"stdin":None,"db":"docs/workable_items.db","ids_snapshot":"ok"})
open(sys.argv[2],"w").write("".join(json.dumps(r)+"\n" for r in rows))
PY
cp -a "$LOCAL/.audit/register/inputs/." "$T_SCR/jd3/inputs/" 2>/dev/null; O9="$T_SCR/out9"
out=$(rp "$REMOTE" --onto $DB --since "$SINCE" --out "$O9" --journal "$T_SCR/jd3/journal.jsonl" 2>&1); rc=$?
[ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'reason=replay_gate_failed' && [ ! -e "$O9" ] && ok "P21 a replayed row that drops a register guard is refused replay_gate_failed (no output directory)" || bad "P21 rc=$rc [$out]"
finish
