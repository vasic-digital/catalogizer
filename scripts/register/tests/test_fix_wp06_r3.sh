#!/usr/bin/env bash
# test_fix_wp06_r3.sh - WP-06 T068 re-review (evidence/reviews/WP-06-T068-r2.json, NO-GO) fix round 2: R2-01 .. R2-10.
# Written FIRST (11.4.224), RED against the round-1 scripts, then GREEN x3. Host side only: scratch DBs, stub LOCKED/BACKUP for replay.sh, no container.
# Sections (ONLY="R201 ..."): R201 replay never deletes what it did not create (every refusal path x a non-empty pre-existing --out); R202 every --prefix literal +
# one prefix grammar; R203 DDL-mutant fidelity of the R04 fixture; R204 raised sqlite_sequence; R205 trigger max leg + duplicate direction; R206 seq boundary;
# R207 input_args shape; R208 DDL header citation.
. "$(dirname "$0")/lib.sh"; . "$(dirname "$0")/clib.sh"
ident_header WP06-T068-R3
GATE=${GATE:-$REG_DIR/gate.sh}; RP=${REPLAY:-$REG_DIR/replay.sh}
ONLY=" ${ONLY:-R201 R202 R203 R204 R205 R206 R207 R208} "
want() { case "$ONLY" in *" $1 "*) return 0;; esac; return 1; }
D="$T_SCR/f3"; mkdir -p "$D"
DESC="scratch item for the T068 round 2 tests, long enough for the engine description floor"
run_gate() { REG_ROOT=${GATE_ROOT:-$ROOT} bash "$GATE" --db "$1" 2>&1; }
good_db() { fresh_ext_db "$1" || return 1; local id; id=$(mint "$1"); "$WI" add Task Low --db "$1" --id "$id" --prefix CAT --title "t" --description "$DESC" >/dev/null 2>&1; }
must_fail() { local out rc; out=$(run_gate "$2"); rc=$?
  if [ $rc -ne 0 ] && ! printf '%s' "$out" | grep -q 'GATE OK' && printf '%s' "$out" | grep -Eq "$3"; then ok "$1"; else bad "$1 :: rc=$rc out=[$(printf '%s' "$out" | head -4 | tr '\n' '|')] wanted /$3/"; fi; }
reapply() { "$REG_DIR/apply_ext.sh" --db "$1" >/dev/null 2>&1; }
mkpfx() { local r="$D/pfx-$1"; rm -rf "$r"; mkdir -p "$r/.helix" "$r/scripts/register" "$r/constitution/scripts/workable-items/bin"
  cp "$ROOT/.helix/reporting.yaml" "$r/.helix/"; cp "$REG_DIR/import_plan.py" "$REG_DIR/import_tickets.py" "$r/scripts/register/"; cp "${REG_EXT_SQL:-$REG_DIR/register_ext.sql}" "$r/scripts/register/register_ext.sql"
  cp "$ROOT/constitution/scripts/workable-items/bin/workable-items-linux" "$r/constitution/scripts/workable-items/bin/"; echo "$r"; }

mkrep() { local r; r=$(mkroot "rp-$1"); echo "stub db" >"$r/docs/workable_items.db"; mkdir -p "$r/.audit/register"; echo "$r"; }
BK="$T_SCR/stub_backup.sh"; printf '#!/usr/bin/env bash\necho "stub backup failure" >&2; exit 9\n' >"$BK"; chmod +x "$BK"
rp() { local r=$1; shift; LOCKED_TEST_MODE=1 LOCKED_ROOT="$r" LOCKED=/bin/false BACKUP="$BK" bash "$RP" "$@"; }
SINCE=$(printf 'a%.0s' $(seq 64)); SB=$(printf 'b%.0s' $(seq 64)); SC=$(printf 'c%.0s' $(seq 64))
jfull() { python3 -I - "$@" <<'PY'
import json,sys
seq,op,sb,sa=int(sys.argv[1]),sys.argv[2],sys.argv[3],sys.argv[4]
print(json.dumps({"seq":seq,"op_id":op,"mode":"register","argv":["sh","-c","x"],"stdin":None,"inputs":[],"input_args":{},"exit":0,"db":"docs/workable_items.db","db_sha_before":sb,"db_sha_after":sa,"wal_bytes_before":0,"wal_bytes_after":0,"ids_snapshot":"ok","ids_minted":[],"time":"2026-10-08T00:00:00.000000Z"},sort_keys=True))
PY
}
jwrite() { local f=$1; shift; : >"$f"; local l; for l in "$@"; do printf '%s\n' "$l" >>"$f"; done; }
mod() { python3 -I -c "import json,sys;r=json.loads(sys.stdin.read());$1;print(json.dumps(r,sort_keys=True))"; }   # mod 'r["seq"]=0' < row

if want R201; then echo "== R2-01: replay.sh removes nothing it did not create: every refusal path against a NON-EMPTY pre-existing --out =="
  R=$(mkrep r201); J="$R/.audit/register/journal.jsonl"
  mkuser() {  # a directory holding user files AND files with replay's own output names
    local u=$1; rm -rf "$u"; mkdir -p "$u/export" "$u/inputs"; echo n >"$u/notes.txt"; echo c >"$u/export/report.csv"; echo i >"$u/inputs/x"; echo s >"$u/register.sql"; echo b >"$u/backup.json"; echo d >"$u/replay.db"; echo r >"$u/replay-report.json"; }
  snap() { (cd "$1" && find . -type f -print0 | sort -z | xargs -0 sha256sum | sha256sum | cut -d' ' -f1; find "$1" | wc -l); }
  chk() {  # chk LABEL REASON args... (the --out is $U)
    local lab=$1 reason=$2; shift 2; mkuser "$U"; local before after out rc; before=$(snap "$U"); out=$(rp "$R" "$@" 2>&1); rc=$?; after=$(snap "$U")
    if [ $rc -eq 20 ] && printf '%s' "$out" | grep -q "reason=$reason" && [ "$before" = "$after" ]; then ok "$lab refused $reason, every file of the pre-existing directory kept"
    else bad "$lab rc=$rc [$(printf '%s' "$out" | head -c 160)] kept=$([ "$before" = "$after" ] && echo yes || echo NO)"; fi; }
  U="$T_SCR/r201u"
  jwrite "$J" "$(jfull 1 base $SB $SINCE)" "$(jfull 2 next $SINCE $SC)"
  chk "R201-1" since_malformed --onto docs/workable_items.db --since notahash --out "$U" --journal "$J"
  chk "R201-2" onto_not_register_database --onto docs/other.db --since "$SINCE" --out "$U" --journal "$J"
  chk "R201-3" journal_missing --onto docs/workable_items.db --since "$SINCE" --out "$U" --journal "$T_SCR/nojournal.jsonl"
  printf 'not json\n' >"$T_SCR/corrupt.jsonl"
  chk "R201-4" journal_corrupt --onto docs/workable_items.db --since "$SINCE" --out "$U" --journal "$T_SCR/corrupt.jsonl"
  mkdir -p "$R/.audit/register/pending"; echo p >"$R/.audit/register/pending/op1"
  chk "R201-5" replay_pending_ops --onto docs/workable_items.db --since "$SINCE" --out "$U" --journal "$J"
  rm -rf "$R/.audit/register/pending"
  jwrite "$T_SCR/inc.jsonl" "$(jfull 1 base $SB $SINCE | mod 'r.pop("wal_bytes_after")')"
  chk "R201-6" journal_row_incomplete --onto docs/workable_items.db --since "$SINCE" --out "$U" --journal "$T_SCR/inc.jsonl"
  jwrite "$T_SCR/ord.jsonl" "$(jfull 2 base $SB $SINCE)" "$(jfull 2 next $SINCE $SC)"
  chk "R201-7" journal_order_invalid --onto docs/workable_items.db --since "$SINCE" --out "$U" --journal "$T_SCR/ord.jsonl"
  chk "R201-8" since_not_found --onto docs/workable_items.db --since "$(printf 'e%.0s' $(seq 64))" --out "$U" --journal "$J"
  chk "R201-9" out_dir_not_empty --onto docs/workable_items.db --since "$SINCE" --out "$U" --journal "$J"
  mkuser "$U"; before=$(snap "$U"); rp "$R" --onto docs/workable_items.db >/dev/null 2>&1; rc=$?; after=$(snap "$U")
  [ $rc -eq 2 ] && [ "$before" = "$after" ] && ok "R201-10 a usage error (exit 2) touches nothing" || bad "R201-10 rc=$rc"
  # a pre-existing EMPTY directory that the stub BACKUP fails inside: a file of ANOTHER process stays, nothing of the run's own is left (R-10 kept)
  mkdir -p "$T_SCR/r201e"; out=$(rp "$R" --onto docs/workable_items.db --since "$SINCE" --out "$T_SCR/r201e" --journal "$J" 2>&1); rc=$?
  [ $rc -eq 20 ] && [ -d "$T_SCR/r201e" ] && [ -z "$(ls -A "$T_SCR/r201e")" ] && ok "R201-11 golden-FALSE: an empty pre-existing directory survives a refusal after the run started and holds nothing of the run" || bad "R201-11 rc=$rc"
fi

if want R202; then echo "== R2-02 / R2-09: every --prefix literal of an importer is checked; one prefix grammar (the engine's 3 capitals) =="
  good_db "$D/r202.db"
  r=$(mkpfx a); out=$(REG_ROOT=$r bash "$GATE" --db "$D/r202.db" 2>&1); printf '%s' "$out" | grep -q '^GATE OK' && ok "R202-0 control: the unmodified tree passes" || bad "R202-0 [$out]"
  for lit in CATS cat C ""; do r=$(mkpfx "x$lit"); printf '\nALT = ["add", "--prefix", "%s"]\n' "$lit" >>"$r/scripts/register/import_tickets.py"
    out=$(REG_ROOT=$r bash "$GATE" --db "$D/r202.db" 2>&1); printf '%s' "$out" | grep -q 'FAIL prefix_agreement' && ! printf '%s' "$out" | grep -q 'GATE OK' && ok "R202-1 a second \"--prefix\" literal [$lit] in an importer fails the gate" || bad "R202-1 [$lit] [$out]"; done
  r=$(mkpfx y); printf '\nALT = ["add", "--prefix", "CAT"]\n' >>"$r/scripts/register/import_plan.py"; out=$(REG_ROOT=$r bash "$GATE" --db "$D/r202.db" 2>&1)
  printf '%s' "$out" | grep -q '^GATE OK' && ok "R202-2 golden-FALSE: a second literal with the SAME value CAT is not a disagreement" || bad "R202-2 [$out]"
  # one grammar: sync_trackers.sh takes the same 3 capitals as the gate and the engine heading grammar
  SR="$T_SCR/st"; mkdir -p "$SR/scripts/register" "$SR/docs" "$SR/.helix"; cp "$REG_DIR/sync_trackers.sh" "$SR/scripts/register/"; fresh_ext_db "$SR/docs/w.db"
  for p in CATS CA; do sed "s/^id_prefix: CAT/id_prefix: $p/" "$ROOT/.helix/reporting.yaml" >"$SR/c.yaml"; out=$(cd "$SR" && bash scripts/register/sync_trackers.sh --config c.yaml --db docs/w.db --dry-run 2>&1)
    printf '%s' "$out" | grep -q 'id_prefix must be 3 capital letters' && ok "R202-3 sync_trackers refuses id_prefix $p (3 capitals, the engine grammar)" || bad "R202-3 $p [$(printf '%s' "$out" | head -c 200)]"; done
  cp "$ROOT/.helix/reporting.yaml" "$SR/c.yaml"; out=$(cd "$SR" && bash scripts/register/sync_trackers.sh --config c.yaml --db docs/w.db --dry-run 2>&1)
  printf '%s' "$out" | grep -q 'id_prefix must be' && bad "R202-4 id_prefix CAT refused [$out]" || ok "R202-4 golden-FALSE: id_prefix CAT passes the config check"
fi

if want R203; then echo "== R2-03: the R04 fixture follows the DDL under test (a mutant DDL is not failed by artifact) =="
  r=$(mkpfx m); cmp -s "$r/scripts/register/register_ext.sql" "${REG_EXT_SQL:-$REG_DIR/register_ext.sql}" && ok "R203-1 the fixture root holds the DDL under test byte for byte" || bad "R203-1 the fixture DDL differs from the DDL under test"
fi

if want R204; then echo "== R2-04: a raised sqlite_sequence is reported by the gate =="
  good_db "$D/r204.db"; sql "$D/r204.db" "UPDATE sqlite_sequence SET seq=100 WHERE name='reg_ids'" >/dev/null
  must_fail "R204-1 sqlite_sequence 100 above max(seq) 1 fails the gate naming v_ids_malformed" "$D/r204.db" 'FAIL v_ids_malformed rows='
  good_db "$D/r204b.db"; out=$(run_gate "$D/r204b.db"); printf '%s' "$out" | grep -q '^GATE OK' && ok "R204-2 golden-FALSE: sqlite_sequence equal to max(seq) passes" || bad "R204-2 [$out]"
  fresh_ext_db "$D/r204c.db"; out=$(run_gate "$D/r204c.db"); printf '%s' "$out" | grep -q '^GATE OK' && ok "R204-3 golden-FALSE: an empty register (no sqlite_sequence row) passes" || bad "R204-3 [$out]"
fi

if want R205; then echo "== R2-05: the trigger's max leg; the duplicate direction of v_import_mint_orphans =="
  good_db "$D/r205.db"; mint "$D/r205.db" >/dev/null; mint "$D/r205.db" >/dev/null
  sql "$D/r205.db" "DROP TRIGGER reg_ids_mint_monotone; INSERT INTO reg_ids(seq,minted_by,mint_basis) VALUES (9,'legacy','manual')" >/dev/null; reapply "$D/r205.db"
  expect_reject "$D/r205.db" "R205-1 in a gapped legacy register (1,2,3,9) an explicit seq equal to the row count (5) is refused by the max leg" "next gapless number" "INSERT INTO reg_ids(seq,minted_by,mint_basis) VALUES (5,'x','manual')"
  # the later duplicate is the one flagged
  good_db "$D/r205b.db"; sql "$D/r205b.db" "DROP INDEX uq_reg_ids_import_marker; INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('m1','import'); INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('m1','import')" >/dev/null
  "$WI" add Task Low --db "$D/r205b.db" --id CAT-002 --prefix CAT --title t2 --description "$DESC" >/dev/null 2>&1; "$WI" add Task Low --db "$D/r205b.db" --id CAT-003 --prefix CAT --title t3 --description "$DESC" >/dev/null 2>&1
  assert_eq "R205-2 with both duplicates having items, exactly the LATER id (CAT-003) is flagged duplicate_marker" "$(sql "$D/r205b.db" "select group_concat(atm_id||':'||why) from v_import_mint_orphans")" "CAT-003:duplicate_marker"
fi

if want R206 || want R207; then
  R=$(mkrep r206); J="$R/.audit/register/journal.jsonl"
  base() { jwrite "$J" "$(jfull 1 base $SB $SINCE)" "$1"; }
  tryrow() {  # tryrow LABEL pyexpr -> must be refused journal_row_incomplete
    base "$(jfull 2 next $SINCE $SC | mod "$2")"; out=$(rp "$R" --onto docs/workable_items.db --since "$SINCE" --out "$T_SCR/o-$1" --journal "$J" 2>&1); rc=$?
    [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'reason=journal_row_incomplete' && ok "$1 refused journal_row_incomplete" || bad "$1 rc=$rc [$(printf '%s' "$out" | head -c 200)]"; }
  trypass() { base "$(jfull 2 next $SINCE $SC | mod "$2")"; out=$(rp "$R" --onto docs/workable_items.db --since "$SINCE" --out "$T_SCR/o-$1" --journal "$J" 2>&1)
    printf '%s' "$out" | grep -q 'reason=journal_row_incomplete' && bad "$1 golden-FALSE refused [$out]" || ok "$1 golden-FALSE accepted by the shape check"; }
fi
if want R206; then echo "== R2-06: seq boundary =="
  tryrow R206-1 'r["seq"]=0'; tryrow R206-2 'r["seq"]=-1'; trypass R206-3 'r["seq"]=2'
fi
if want R207; then echo "== R2-07: input_args shape =="
  tryrow R207-1 'r["input_args"]={"x":"%s"}' ; tryrow R207-2 'r["input_args"]={"99":"'$SC'"}'; tryrow R207-3 'r["input_args"]={"1":5}'; tryrow R207-4 'r["input_args"]={"1":"nothex"}'; tryrow R207-5 'r["input_args"]={"-1":"'$SC'"}'
  trypass R207-6 'r["input_args"]={"2":"'$SC'"}; r["argv"]=["a","b","c"]'
fi

if want R208; then echo "== R2-08: every evidence file the DDL header cites exists =="
  EVD="$ROOT/specs/001-full-project-audit-remediation/evidence/wp06"; miss=""
  for f in $(sed -n '1,12p' "$REG_DIR/register_ext.sql" | grep -o '[A-Za-z0-9_.-]*owed-docs04[A-Za-z0-9_.-]*\.diff'); do [ -f "$EVD/$f" ] || miss="$miss $f"; done
  [ -z "$miss" ] && ok "R208-1 every cited owed-docs04 diff exists" || bad "R208-1 cited but absent:$miss"
fi
finish
