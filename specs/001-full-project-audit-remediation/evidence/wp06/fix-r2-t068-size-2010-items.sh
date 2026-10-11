#!/usr/bin/env bash
# T068 R-08: size of a 2,010-item register fixture (scratch DB only, never docs/workable_items.db)
set -u
R=/home/milosvasic/Projects/catalogizer; WI=$R/submodules/constitution/scripts/workable-items/bin/workable-items-linux
D=/tmp/claude-1000/size/work; rm -rf $D; mkdir -p $D; DB=$D/size.db; N=${1:-2010}
$R/scripts/register/apply_ext.sh --db $DB >/dev/null || exit 1
sqlite3 $DB "WITH RECURSIVE c(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM c WHERE i<$N) INSERT INTO reg_ids(minted_by,mint_basis) SELECT 'size:e'||i,'import' FROM c;"
DESC=$(python3 -c "print(('word%04d ' * 205)[:2048] % tuple(range(205)))" 2>/dev/null || python3 -c "print(('lorem ipsum '*200)[:2048])")
DESC=$(python3 -c "print(('lorem ipsum '*200)[:2040])")
echo "desc_bytes=${#DESC}"
for i in $(seq 1 $N); do id=$(printf 'CAT-%03d' $i); "$WI" add Task Low --db $DB --id $id --prefix CAT --title "scratch size item $i with a title of ordinary length for the measurement" --description "$DESC" >/dev/null || { echo "add failed at $id"; exit 2; }; done
sqlite3 $DB "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,severity_source,custody_basis,reverify_required,legacy_status) SELECT atm_id,'bug','source','low','size','machine_evidence',0,NULL FROM items;"
sqlite3 $DB "PRAGMA wal_checkpoint(TRUNCATE)" >/dev/null
echo "items=$(sqlite3 $DB 'select count(*) from items') reg_ids=$(sqlite3 $DB 'select count(*) from reg_ids') ext=$(sqlite3 $DB 'select count(*) from reg_item_ext')"
echo "db_bytes=$(stat -c %s $DB)"
sqlite3 $DB .dump | grep -v '^PRAGMA' > $D/register.sql; echo "register_sql_bytes=$(stat -c %s $D/register.sql)"
"$WI" sync db-to-md --db $DB --out-issues $D/Issues.md --out-fixed $D/Fixed.md >/dev/null 2>&1; echo "Issues_md_bytes=$(stat -c %s $D/Issues.md 2>/dev/null) Fixed_md_bytes=$(stat -c %s $D/Fixed.md 2>/dev/null)"
echo "gate: $(REG_ROOT=$R bash $R/scripts/register/gate.sh --db $DB 2>&1 | tail -1)"
