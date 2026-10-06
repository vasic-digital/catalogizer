#!/usr/bin/env bash
# test_backup_db.sh - T064a (docs/04 section 12.2): scripts/register/backup_db.sh, the pre-op backup helper. RED while the helper is absent.
# A hardlinked copy is not a backup (SQLite writes pages in place): the test has the control that shows the failure it guards against.
# Env BACKUP substitutes a mutant copy (mutate_backup_db.sh). Container leg: see clib.sh.
. "$(dirname "$0")/lib.sh"; . "$(dirname "$0")/clib.sh"
evhead T064a
if [ ! -f "$BACKUP" ]; then bad "T0 helper absent: $BACKUP"; finish; exit 1; fi
R=$(mkroot a); cdb "$R" workable_items.db || { bad setup; finish; exit 1; }
for i in 1 2 3; do lk "$R" -- sh -c "$(mint_cmd workable_items.db)" >/dev/null 2>&1 || { bad "setup mint $i"; finish; exit 1; }; done
hb() { env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R" LOCKED_RUNP="$RUNP" LOCKED="$LOCKED" SHIM_LOG="$SHIM_LOG" PATH="$SHIM_DIR:$PATH" bash "$BACKUP" "$@"; }
rows_of() { python3 -I -c 'import json,sys;print(json.load(open(sys.argv[1]))[sys.argv[2]])' "$@"; }
# control first: a hardlink "backup" of the live file changes with the source (the failure the test must be able to see)
cp -al "$R/docs/workable_items.db" "$T_SCR/ctl.db"; ctl0=$(fsha "$T_SCR/ctl.db")
echo "== helper =="
out=$(hb 2>&1); rc=$?; [ $rc -eq 2 ] && printf '%s' "$out" | grep -qi usage && ok "B1 no --record: exit 2 and a usage text" || bad "B1 rc=$rc [$out]"
REC="$T_SCR/rec.json"; : >"$SHIM_LOG"
out=$(hb --record "$REC" 2>&1); rc=$?
[ $rc -eq 0 ] && [ -s "$REC" ] && ok "B2 the helper succeeds and writes the record" || { bad "B2 rc=$rc [$out]"; finish; exit 1; }
BAK=$(rows_of "$REC" backup_path); [ -f "$R/$BAK" ] && case "$BAK" in docs/workable_items.db.bak-2*) ok "B3 the backup is docs/workable_items.db.bak-<UTC>";; *) bad "B3 name [$BAK]";; esac || bad "B3 absent [$BAK]"
for f in source_sha256 backup_sha256 source_rows backup_rows integrity restore_probe image_digest; do python3 -I -c 'import json,sys;sys.exit(0 if json.load(open(sys.argv[1])).get(sys.argv[2]) not in (None,"") else 1)' "$REC" $f && ok "B4 record field $f" || bad "B4 record field $f missing"; done
assert_eq "B5 integrity_check result is ok" "$(rows_of "$REC" integrity)" ok
assert_eq "B6 restore probe: the restored backup's canonical dump equals the source dump" "$(rows_of "$REC" restore_probe)" equal
[ "$(rows_of "$REC" source_rows)" = "$(rows_of "$REC" backup_rows)" ] && [ "$(rows_of "$REC" source_rows)" -gt 0 ] && ok "B7 both row counts are equal and non-zero ($(rows_of "$REC" source_rows))" || bad "B7"
assert_eq "B8 the record's backup sha256 is the file's" "$(rows_of "$REC" backup_sha256)" "$(fsha "$R/$BAK")"
assert_eq "B9 the helper's sqlite work ran in the image: the run calls name the pinned image" "$(grep -E '^run ' "$SHIM_LOG" | grep -vc 'localhost/catalogizer-testutil@sha256:')" 0
[ ! -e "$R/$BAK-wal" ] && [ ! -e "$R/$BAK-shm" ] && ok "B10 no -wal/-shm side file beside the backup (immutable reads)" || bad "B10 side files"
bsha=$(fsha "$R/$BAK"); brows=$(rows_of "$REC" backup_rows)
echo "== the backup stays what it was; the hardlink control changes =="
lk "$R" -- sh -c "$(mint_cmd workable_items.db); sqlite3 /src/docs/workable_items.db 'PRAGMA wal_checkpoint(TRUNCATE)' >/dev/null" >/dev/null 2>&1
assert_eq "B11 after one more row in the source (checkpointed) the backup's sha256 is unchanged" "$(fsha "$R/$BAK")" "$bsha"
n=$(lk "$R" --out "$T_SCR/o3" -- sqlite3 -readonly "file:/src/$BAK?immutable=1" "select count(*) from items" 2>&1); n0=$(lk "$R" --out "$T_SCR/o4" -- sqlite3 -readonly "file:/src/docs/workable_items.db?immutable=1" "select count(*) from items" 2>&1)
[ "$n" = 3 ] && [ "$n0" = 4 ] && ok "B12 the backup still holds 3 items, the source 4" || bad "B12 backup=[$n] source=[$n0]"
ctl1=$(fsha "$T_SCR/ctl.db"); [ "$ctl1" != "$ctl0" ] && ok "B13 CONTROL: the cp -al copy changed with the source (the test can see the failure it guards against)" || bad "B13 control did not change"
echo "== a failed check means no backup =="
R2=$(mkroot b); cdb "$R2" workable_items.db; REC2="$T_SCR/rec2.json"
out=$(env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R2" LOCKED_RUNP="$RUNP" LOCKED="$LOCKED" BACKUP_FAULT=truncate SHIM_LOG="$SHIM_LOG" PATH="$SHIM_DIR:$PATH" bash "$BACKUP" --record "$REC2" 2>&1); rc=$?
[ $rc -ne 0 ] && [ ! -e "$REC2" ] && ok "B14 a backup that fails the integrity check exits non-zero and writes no record" || bad "B14 rc=$rc rec=$([ -e "$REC2" ] && echo present) [$out]"
[ -z "$(ls "$R2"/docs/workable_items.db.bak-* 2>/dev/null)" ] && ok "B15 the failed backup file is removed (never reported as a backup)" || bad "B15 left: $(ls "$R2"/docs/)"
out=$(hb --record "$T_SCR/rec3.json" 2>&1); rc=$?; [ $rc -eq 0 ] && ok "B16 a normal run with the fault hook unset succeeds again" || bad "B16"
R4=$(mkroot d); cdb "$R4" workable_items.db
out=$(env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R4" LOCKED_RUNP="$RUNP" LOCKED="$LOCKED" BACKUP_FAULT=dumpdiff SHIM_LOG="$SHIM_LOG" PATH="$SHIM_DIR:$PATH" bash "$BACKUP" --record "$T_SCR/rec5.json" 2>&1); rc=$?
[ $rc -ne 0 ] && [ ! -e "$T_SCR/rec5.json" ] && [ -z "$(ls "$R4"/docs/workable_items.db.bak-* 2>/dev/null)" ] && printf '%s' "$out" | grep -q 'restore probe' && ok "B18 a restore probe whose dump differs from the source dump fails: non-zero, no record, no backup file" || bad "B18 rc=$rc [$out]"
echo "== a missing source database =="
R3=$(mkroot c); out=$(env LOCKED_TEST_MODE=1 LOCKED_ROOT="$R3" LOCKED_RUNP="$RUNP" LOCKED="$LOCKED" SHIM_LOG="$SHIM_LOG" PATH="$SHIM_DIR:$PATH" bash "$BACKUP" --record "$T_SCR/rec4.json" 2>&1); rc=$?
[ $rc -ne 0 ] && [ ! -e "$T_SCR/rec4.json" ] && ok "B17 no source database: refused, no record" || bad "B17 rc=$rc [$out]"
finish
