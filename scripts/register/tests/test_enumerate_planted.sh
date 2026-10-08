#!/usr/bin/env bash
# test_enumerate_planted.sh - T162 (WP-20): planted-entry completeness test of the Stage 0 enumerator
# (scripts/register/enumerate_sources.py, T165). doc03 section 10 step 9 and section 15 item 5.
#   * the T162 scenario: one checkbox line + one docs/issues ticket + one bank case planted into a copy -> exactly 3
#     new reg_source_entries rows; removing the planted entries gives back the baseline SQL byte for byte
#   * class-exhaustive control needle: one planted entry per source class S-01..S-25 -> that class and no other grows by
#     exactly 1 and exactly 1 new row lands in the scratch DB; every class reads > 0 on the baseline (a class that reads
#     zero is a blind instrument, not an empty source)
#   * decoy carriers (non-matching lines, .txt next to tickets, a rust std::process::exit, TODOS, s.Skip) add no row
#   * the enumerator's own run leaves a scratch DB's bytes unchanged; the SQL import is idempotent
#   * a snapshot copy changed after its manifest is refused `freeze_snapshot_moved` with no output written
#   * REAL_SNAPSHOT=<dir of a frozen snapshot>: the 3-way scenario also runs on a COPY of that snapshot (the real sources, T176), with
#     REAL_REMOTES (comma list of remote names, default origin) as the freeze json remotes; ABSOLUTE contents are asserted by
#     test_enumerate_absolute.py, this file asserts the deltas and the removal round trip
# Env: ENUMERATE_SOURCES (enumerator under test; mutation runs point it at a mutated copy), SQLITE3, WI, REG_SCRATCH.
# Runs in IMG-TESTUTIL via `scripts/test-in-container.sh tooling unit -- bash scripts/register/tests/test_enumerate_planted.sh`
# or on the host (identity header says which). Exit 0 only when every check passed.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
source "$(dirname "${BASH_SOURCE[0]}")/wp20_ident.sh"
ENUM=${ENUMERATE_SOURCES:-$REG_DIR/enumerate_sources.py}
FIX="$REG_DIR/tests/wp20_fixture.py"
ident_header_wp20 T162
echo "# enumerator=$ENUM sha256=$(sha256sum "$ENUM" 2>/dev/null | cut -c1-64)"
if [ ! -f "$ENUM" ]; then bad "enumerator absent: $ENUM (RED: T165 not implemented)"; echo "RESULT: pass=$PASS fail=$FAIL"; exit 1; fi

enum() {  # enum TREE OUTDIR -> runs freeze + enumerator; prints rc
  python3 "$FIX" freeze "$1" "$1.freeze.json" || return 99
  python3 "$ENUM" --freeze-json "$1.freeze.json" --out "$2" >"$2.log" 2>&1; echo $?
}
cls_entries() { python3 -c "import json,sys;print(json.load(open(sys.argv[1]))['classes'][sys.argv[2]]['entries'])" "$1/enumeration-stats.json" "$2"; }
total() { python3 -c "import json,sys;print(json.load(open(sys.argv[1]))['total_entries'])" "$1/enumeration-stats.json"; }
apply_sql() { "$SQLITE3" "$1" ".read $2" 2>&1; }
nrows() { "$SQLITE3" "$1" "SELECT count(*) FROM reg_source_entries"; }
dbhash() { "$SQLITE3" "$1" "SELECT group_concat(s.locator||'|'||e.locator||'|'||e.entry_sha256, char(10)) FROM reg_source_entries e JOIN reg_sources s USING(source_id) ORDER BY 1" | sha256sum | cut -c1-64; }

BASE="$T_SCR/base"; python3 "$FIX" build "$BASE"
OUT0="$T_SCR/out0"; mkdir -p "$OUT0"; rc=$(enum "$BASE" "$OUT0")
assert_eq "baseline enumeration exits 0" "$rc" "0"
[ "$rc" = 0 ] || { echo "RESULT: pass=$PASS fail=$FAIL"; exit 1; }
N0=$(total "$OUT0")
# control needle: every class reads > 0 entries on the baseline (the instrument can see each class)
for n in $(seq 1 25); do c=$(printf 'S-%02d' $n); v=$(cls_entries "$OUT0" "$c" 2>&1); if [ "$v" -gt 0 ] 2>/dev/null; then ok "baseline class $c reads $v entries"; else bad "baseline class $c reads [$v] (blind or empty)"; fi; done
kinds=$(python3 -c "import json;d=json.load(open('$OUT0/source-class-kinds.json'));print(len(d['classes']),d['classes_missing'])")
assert_eq "source-class-kinds.json maps all 25 classes plus XID, EXT and the plan-document classes S-26 and S-27" "$kinds" "29 []"
ALLC=$(python3 -c "import json;print(' '.join(sorted(json.load(open('$OUT0/enumeration-stats.json'))['classes'])))")
for c in XID EXT; do v=$(cls_entries "$OUT0" "$c" 2>&1); if [ "$v" -gt 0 ] 2>/dev/null; then ok "baseline class $c reads $v entries"; else bad "baseline class $c reads [$v]"; fi; done
assert_eq "sql sha256 sibling (the one locked.sh import-sql reads) equals the .sql.sha256 file" "$(cat "$OUT0/source_entries.sha256")" "$(cat "$OUT0/source_entries.sql.sha256")"
assert_eq "sql sha256 file is the bare hash of the sql" "$(cat "$OUT0/source_entries.sql.sha256")" "$(sha256sum "$OUT0/source_entries.sql" | cut -d' ' -f1)"
# determinism
OUT0b="$T_SCR/out0b"; mkdir -p "$OUT0b"; enum "$BASE" "$OUT0b" >/dev/null
assert_eq "two runs over the same snapshot give a byte-identical sql" "$(sha256sum <"$OUT0b/source_entries.sql" | cut -c1-64)" "$(sha256sum <"$OUT0/source_entries.sql" | cut -c1-64)"

# scratch DB: the enumerator's own run does not touch it; the import changes it; a second import changes nothing
DB0="$T_SCR/base.db"; fresh_ext_db "$DB0" || { bad "scratch db"; echo "RESULT: pass=$PASS fail=$FAIL"; exit 1; }
EMPTYDB="$T_SCR/empty.db"; cp "$DB0" "$EMPTYDB"     # a pristine extension DB: a planted tree is imported into a COPY of it (an import over the baseline would be refused by the stale-entry gate, a planted file changing its sha at its locator)
H_DB=$(sha256sum <"$DB0" | cut -c1-64); OUTx="$T_SCR/outx"; mkdir -p "$OUTx"; enum "$BASE" "$OUTx" >/dev/null
assert_eq "enumerator run leaves the scratch DB bytes unchanged" "$(sha256sum <"$DB0" | cut -c1-64)" "$H_DB"
r=$(apply_sql "$DB0" "$OUT0/source_entries.sql"); assert_eq "sql import applies cleanly" "$r" ""
assert_eq "import creates exactly total_entries rows" "$(nrows "$DB0")" "$N0"
assert_eq "import changes the DB bytes" "$([ "$(sha256sum <"$DB0" | cut -c1-64)" != "$H_DB" ] && echo changed)" "changed"
G1=$(dbhash "$DB0"); r2=$(apply_sql "$DB0" "$OUT0/source_entries.sql")
assert_eq "second import prints no error (INSERT OR IGNORE, never a UNIQUE failure)" "$r2" ""
assert_eq "second import adds no row" "$(nrows "$DB0")" "$N0"
assert_eq "second import changes no entry" "$(dbhash "$DB0")" "$G1"
assert_eq "scanned_entry_count equals the rows of every source" "$("$SQLITE3" "$DB0" "SELECT count(*) FROM reg_sources s WHERE scanned_entry_count IS NOT (SELECT count(*) FROM reg_source_entries e WHERE e.source_id=s.source_id)")" "0"

# per-class planted entry
for c in $(for n in $(seq 1 25); do printf 'S-%02d ' $n; done) XID; do
  T="$T_SCR/p_$c"; cp -a "$BASE" "$T"; python3 "$FIX" plant "$T" "$c"; O="$T_SCR/o_$c"; mkdir -p "$O"; rc=$(enum "$T" "$O")
  if [ "$rc" != 0 ]; then bad "planted $c: enumerator rc=$rc $(head -c 200 "$O.log")"; continue; fi
  d=$(( $(cls_entries "$O" "$c") - $(cls_entries "$OUT0" "$c") )); dt=$(( $(total "$O") - N0 ))
  others=0; for k in $ALLC; do [ "$k" = "$c" ] && continue; [ "$(cls_entries "$O" "$k")" = "$(cls_entries "$OUT0" "$k")" ] || others=$((others+1)); done
  DBc="$T_SCR/db_$c.db"; cp "$EMPTYDB" "$DBc"; apply_sql "$DBc" "$O/source_entries.sql" >/dev/null
  assert_eq "planted $c: class +1, total +1, other classes unchanged, DB +1 row" "$d $dt $others $(( $(nrows "$DBc") - N0 ))" "1 1 0 1"
done

# T162 scenario: checkbox + ticket + bank case -> exactly 3 new rows, removal restores the baseline
T3="$T_SCR/p_3way"; cp -a "$BASE" "$T3"; python3 "$FIX" plant "$T3" 3way; O3="$T_SCR/o_3way"; mkdir -p "$O3"; rc=$(enum "$T3" "$O3")
assert_eq "3-way plant: enumerator exits 0" "$rc" "0"
DB3="$T_SCR/db_3way.db"; cp "$EMPTYDB" "$DB3"; apply_sql "$DB3" "$O3/source_entries.sql" >/dev/null
assert_eq "3-way plant: exactly 3 new reg_source_entries rows" "$(( $(nrows "$DB3") - N0 ))" "3"
DBB="$T_SCR/db_basefresh.db"; cp "$EMPTYDB" "$DBB"; apply_sql "$DBB" "$OUT0/source_entries.sql" >/dev/null
assert_eq "3-way plant: the new locators (planted DB minus baseline DB, by natural key) are the checkbox line, the ticket file and the bank case" "$("$SQLITE3" "$DB3" "ATTACH '$DBB' AS b; SELECT group_concat(locator, ' ') FROM (SELECT e.locator FROM reg_source_entries e WHERE NOT EXISTS (SELECT 1 FROM b.reg_source_entries f WHERE f.locator=e.locator AND f.entry_sha256=e.entry_sha256) AND e.locator IN ('MASTER_EXECUTION_CHECKLIST.md:L5','challenges/helixqa-banks/b1.yaml#planted-case','docs/issues/HELIX-901-planted.md') ORDER BY e.locator)")" "MASTER_EXECUTION_CHECKLIST.md:L5 challenges/helixqa-banks/b1.yaml#planted-case docs/issues/HELIX-901-planted.md"
# remove the planted entries FROM THE PLANTED COPY (not a fresh build): restore the two appended files, delete the planted ticket
RM="$T_SCR/p_removed"; cp -a "$T3" "$RM"; cp "$BASE/MASTER_EXECUTION_CHECKLIST.md" "$RM/MASTER_EXECUTION_CHECKLIST.md"; cp "$BASE/challenges/helixqa-banks/b1.yaml" "$RM/challenges/helixqa-banks/b1.yaml"; rm "$RM/docs/issues/HELIX-901-planted.md"
O4="$T_SCR/o_removed"; mkdir -p "$O4"; enum "$RM" "$O4" >/dev/null
assert_eq "planted entries removed from the planted copy: the sql equals the baseline byte for byte" "$(sha256sum <"$O4/source_entries.sql" | cut -c1-64)" "$(sha256sum <"$OUT0/source_entries.sql" | cut -c1-64)"

# decoys add no row
TD="$T_SCR/p_decoy"; cp -a "$BASE" "$TD"; python3 "$FIX" decoy "$TD"; OD="$T_SCR/o_decoy"; mkdir -p "$OD"; rc=$(enum "$TD" "$OD")
assert_eq "decoy tree: enumerator exits 0" "$rc" "0"
assert_eq "decoy carriers add no row (total unchanged)" "$(total "$OD")" "$N0"
assert_eq "decoy carriers appear in no locator" "$(grep -c -e 'process::exit' -e 'other\.go' -e 'notes\.txt' "$OD/source_entries.sql")" "0"

# a snapshot copy changed after its manifest -> freeze_snapshot_moved, nothing written
TM="$T_SCR/p_moved"; cp -a "$BASE" "$TM"; python3 "$FIX" freeze "$TM" "$TM.freeze.json"; echo "- [ ] sneaked in" >>"$TM/MASTER_EXECUTION_CHECKLIST.md"
OM="$T_SCR/o_moved"; mkdir -p "$OM"; python3 "$ENUM" --freeze-json "$TM.freeze.json" --out "$OM" >"$OM.log" 2>&1; rc=$?
assert_eq "moved snapshot: refused with exit 20" "$rc" "20"
assert_eq "moved snapshot: stderr names freeze_snapshot_moved and the file" "$(grep -c 'freeze_snapshot_moved' "$OM.log") $(grep -c 'MASTER_EXECUTION_CHECKLIST.md' "$OM.log")" "2 1"
assert_eq "moved snapshot: no source_entries.sql written" "$(ls "$OM" | wc -l)" "0"

# the real sources (T176): the 3-way scenario on a copy of a real frozen snapshot
if [ -n "${REAL_SNAPSHOT:-}" ]; then
  mkreal() {  # mkreal <tree> <freeze.json>: manifest + freeze json of the tree with the SAME remotes both times (the remote rows are part of the totals)
    python3 - "$1" "$2" "${REAL_REMOTES:-origin}" "$REG_DIR" <<'PY'
import json, subprocess, sys, os
root, fj, rem, reg = sys.argv[1:5]
mf = fj + ".manifest.json"
subprocess.run([sys.executable, os.path.join(reg, "snapshot_manifest.py"), root], stdout=open(mf, "w"), check=True)
json.dump({"snapshot": root, "manifest": mf, "frozen_at": "2026-10-08T00:00:00Z", "head": "real-copy", "remotes": rem.split(",")}, open(fj, "w"))
PY
  }
  RS="$T_SCR/real"; cp -a "$REAL_SNAPSHOT" "$RS"; mkreal "$RS" "$T_SCR/real.json"
  OR0="$T_SCR/real_o0"; mkdir -p "$OR0"; python3 "$ENUM" --freeze-json "$T_SCR/real.json" --out "$OR0" >"$OR0.log" 2>&1; assert_eq "real sources: baseline enumeration exits 0" "$?" "0"
  NR0=$(total "$OR0")
  python3 "$FIX" plant "$RS" 3wayreal; mkreal "$RS" "$T_SCR/real2.json"; OR1="$T_SCR/real_o1"; mkdir -p "$OR1"; python3 "$ENUM" --freeze-json "$T_SCR/real2.json" --out "$OR1" >"$OR1.log" 2>&1
  assert_eq "real sources: the 3-way plant adds exactly 3 entries" "$(( $(total "$OR1") - NR0 ))" "3"
fi

echo "RESULT: pass=$PASS fail=$FAIL"
[ "$FAIL" -eq 0 ]
