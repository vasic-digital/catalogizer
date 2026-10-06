#!/usr/bin/env bash
# test_dump.sh - T066 (docs/04 section 12.1 R-6): the deterministic dump scripts/register/dump.sh, proven on a scratch DB. RED while absent.
# Env DUMP substitutes a mutant copy (mutate_register_ops.sh).
. "$(dirname "$0")/lib.sh"; . "$(dirname "$0")/clib.sh"
evhead T066
if [ ! -f "$DUMP" ]; then bad "T0 dump script absent: $DUMP"; finish; exit 1; fi
R=$(mkroot a); cdb "$R" s.db || { bad setup; finish; exit 1; }
for i in 1 2; do lk "$R" -- sh -c "$(mint_cmd s.db)" >/dev/null 2>&1 || { bad "setup mint"; finish; exit 1; }; done
dump() { tool "$R" "$DUMP" "$@"; }
echo "== determinism =="
out=$(dump --db docs/s.db --out docs/register/s.sql 2>&1); rc=$?
[ $rc -eq 0 ] && [ -s "$R/docs/register/s.sql" ] && ok "D1 the dump is written (register mode, docs/register/s.sql)" || { bad "D1 rc=$rc [$out]"; finish; exit 1; }
h1=$(fsha "$R/docs/register/s.sql"); cp "$R/docs/register/s.sql" "$T_SCR/first.sql"
dump --db docs/s.db --out docs/register/s.sql >/dev/null 2>&1; h2=$(fsha "$R/docs/register/s.sql")
assert_eq "D2 two consecutive dumps of an unchanged database hash identically (recorded: $h1)" "$h2" "$h1"
dump --db docs/s.db --out docs/register/s.sql >/dev/null 2>&1; assert_eq "D3 a third dump hashes identically" "$(fsha "$R/docs/register/s.sql")" "$h1"
grep -q '^PRAGMA' "$R/docs/register/s.sql" && bad "D4 PRAGMA lines present" || ok "D4 no PRAGMA line in the dump"
grep -q '^INSERT INTO' "$R/docs/register/s.sql" && grep -q 'CAT-001' "$R/docs/register/s.sql" && ok "D5 the dump holds the register's rows (CAT-001)" || bad "D5"
echo "== the dump follows the database =="
lk "$R" -- sh -c "$(mint_cmd s.db)" >/dev/null 2>&1; dump --db docs/s.db --out docs/register/s.sql >/dev/null 2>&1
h3=$(fsha "$R/docs/register/s.sql"); [ "$h3" != "$h1" ] && grep -q 'CAT-003' "$R/docs/register/s.sql" && ok "D6 a database change changes the dump (the new item CAT-003 is in it)" || bad "D6"
echo "== a restored dump dumps to the same bytes =="
lk "$R" --out "$T_SCR/o1" -- sh -c 'sqlite3 /out/r.db ".read /src/docs/register/s.sql" && sqlite3 /out/r.db ".dump" | grep -v "^PRAGMA" > /out/r.sql' >/dev/null 2>&1
assert_eq "D7 dump -> restore -> dump is byte-identical" "$(fsha "$T_SCR/o1/r.sql")" "$h3"
echo "== the dump does not alter the register =="
before=$(fsha "$R/docs/s.db"); dump --db docs/s.db --out docs/register/s.sql >/dev/null 2>&1; assert_eq "D8 the database file keeps its sha256 (the checkpoint finds nothing to move)" "$(fsha "$R/docs/s.db")" "$before"
echo "== out mode =="
mkdir -p "$T_SCR/om"; cp "$R/docs/s.db" "$T_SCR/om/copy.db"
out=$(dump --out-dir "$T_SCR/om" --db-file copy.db --out copy.sql 2>&1); rc=$?
[ $rc -eq 0 ] && [ "$(fsha "$T_SCR/om/copy.sql")" = "$h3" ] && ok "D9 out mode dumps <dir>/<db-file> to <dir>/<out> with the same bytes as register mode" || bad "D9 rc=$rc [$out]"
echo "== a row still in the -wal file is in the dump (the checkpoint comes first) =="
RW=$(mkroot w); cdb "$RW" s.db
# a second connection holds the database open (a query that reads a table, so the file is really opened) while the row is committed: the last
# connection never closes cleanly, the row stays in the -wal file when the container is gone
lk "$RW" -- bash -c 'sqlite3 /src/docs/s.db "select count(*) from reg_ids" ".shell sleep 8" >/dev/null 2>&1 & sleep 2; sqlite3 /src/docs/s.db "INSERT INTO reg_ids(minted_by,mint_basis) VALUES (\"walrow\",\"manual\")"; exit 0' >/dev/null 2>&1
if [ -s "$RW/docs/s.db-wal" ]; then ok "D14a fixture: a committed row is pending in docs/s.db-wal ($(stat -c %s "$RW/docs/s.db-wal") bytes)"; else bad "D14a fixture: no pending WAL content"; fi
tool "$RW" "$DUMP" --db docs/s.db --out docs/register/s.sql >/dev/null 2>&1
grep -q "walrow" "$RW/docs/register/s.sql" && ok "D14 the pending row is in the dump (checkpoint, then dump)" || bad "D14 the dump misses the row that was only in the -wal file"
echo "== refusals =="
for a in "--db ../x.db" "--db docs/a_b.db/../../x.db" "--db /abs.db" "--out docs/../x.sql" "--out outside/x.sql" "--out docs/x.txt" "--db docs/x.txt"; do
  out=$(dump $a 2>&1); rc=$?; [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'REFUSED reason=path_invalid' && ok "D10 refused: $a" || bad "D10 $a rc=$rc [$out]"
done
dump --bogus >/dev/null 2>&1; assert_eq "D11 unknown argument: exit 2" "$?" 2
[ -f "$R/docs/register/x.sql" ] && bad "D12 a refused call wrote a file" || ok "D12 refused calls wrote nothing"
echo "== the commit procedure is written down =="
DOC="$ROOT/docs/scripts/register_dump.md"
if [ -f "$DOC" ]; then grep -q 'wal_checkpoint(TRUNCATE)' "$DOC" && grep -q 'commit-push-all.sh' "$DOC" && grep -q 'export.sh' "$DOC" && ok "D13 docs/scripts/register_dump.md writes the commit procedure down (checkpoint, export, one commit through commit-push-all.sh)" || bad "D13 procedure text incomplete"; else bad "D13 $DOC absent"; fi
finish
