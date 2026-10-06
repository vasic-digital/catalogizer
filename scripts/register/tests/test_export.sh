#!/usr/bin/env bash
# test_export.sh - T067 (docs/04 section 11.2, 11.3): scripts/register/export.sh and reconcile.sh on a scratch DB holding items. RED while absent.
# Env EXPORT / RECONCILE substitute mutant copies (mutate_register_ops.sh). Container leg: see clib.sh.
. "$(dirname "$0")/lib.sh"; . "$(dirname "$0")/clib.sh"
evhead T067
if [ ! -f "$EXPORT" ]; then bad "T0 export script absent: $EXPORT"; finish; exit 1; fi
R=$(mkroot a); cdb "$R" s.db || { bad setup; finish; exit 1; }
for i in 1 2 3; do lk "$R" -- sh -c "$(mint_cmd s.db)" >/dev/null 2>&1 || { bad "setup mint"; finish; exit 1; }; done
# reconciliation data: two entries of one legacy id (collision), one unmapped, one mapped; a re-verify row; a finding is not seeded (no audit run)
lk "$R" -- sh -c "sqlite3 /src/docs/s.db \"INSERT INTO reg_sources(kind,locator,parser) VALUES ('issue_file','docs/issues/A.md','p'); INSERT INTO reg_source_entries(source_id,locator,legacy_id,title,entry_sha256) VALUES (1,'a1','LEG-1','same','$(printf 'a%.0s' $(seq 64))'),(1,'a2','LEG-1','other','$(printf 'b%.0s' $(seq 64))'),(1,'a3','LEG-2','third','$(printf 'c%.0s' $(seq 64))'); INSERT INTO reg_source_map(entry_id,atm_id,relation,match_basis,mapped_by) VALUES (1,'CAT-001','primary','ticket','t');\"" >/dev/null 2>&1 || { bad "seed reconciliation rows"; }
ex() { tool "$R" "$EXPORT" "$@"; }
D="$R/docs/register"
echo "== export =="
out=$(ex --db docs/s.db --out-dir docs/register 2>&1); rc=$?
[ $rc -eq 0 ] && printf '%s' "$out" | grep -q 'in sync' && ok "E1 export succeeds and the engine diff prints in sync" || { bad "E1 rc=$rc [$out]"; finish; exit 1; }
miss=""; for f in Issues.md Fixed.md Issues_Summary.md Fixed_Summary.md Reconciliation.md export-manifest.sha256 reconciliation.csv unmapped_entries.csv legacy_id_collisions.csv reverify_queue.csv reopen_counts.csv stale_tracker_sync.csv findings.csv; do [ -f "$D/$f" ] || miss="$miss $f"; done
[ -z "$miss" ] && ok "E2 every output exists (4 engine files, Reconciliation.md, 7 CSV, manifest)" || bad "E2 missing:$miss"
snap() { (cd "$D" && sha256sum *.md *.csv | sort -k2); }
s1=$(snap); ex --db docs/s.db --out-dir docs/register >/dev/null 2>&1; s2=$(snap)
assert_eq "E3 two runs on an unchanged DB give identical sha256 for every Markdown and CSV output" "$s2" "$s1"
echo "== reconcile content (golden) =="
grep -q 'LEG-1' "$D/legacy_id_collisions.csv" && ok "E4 the collision CSV names legacy id LEG-1" || bad "E4 $(cat "$D/legacy_id_collisions.csv")"
n=$(($(wc -l <"$D/unmapped_entries.csv")-1)); assert_eq "E5 the unmapped CSV holds the two unmapped entries" "$n" 2
grep -q 'CAT-001' "$D/reconciliation.csv" && ok "E6 the reconciliation CSV holds the mapped entry's item" || bad "E6"
grep -q '^| Revision | 1 |$' "$D/Reconciliation.md" && grep -q '^| Last modified |' "$D/Reconciliation.md" && grep -q '^## Unmapped source entries' "$D/Reconciliation.md" && ok "E7 Reconciliation.md carries its header table and a section per view" || bad "E7"
echo "== recorded run =="
SQ() { lk "$R" --out "$T_SCR/o$RANDOM" -- sqlite3 -readonly "file:/src/docs/s.db?immutable=1" "$1" 2>&1; }
assert_eq "E8 two runs recorded two reg_export_runs rows, both OK" "$(SQ "select count(*)||':'||group_concat(distinct verdict) from reg_export_runs")" "2:OK"
fps=$(SQ "select count(distinct db_fingerprint) from reg_export_runs"); assert_eq "E9 both runs recorded the same fingerprint (unchanged DB; the run's own rows are excluded)" "$fps" 1
row=$(SQ "select length(db_fingerprint)||' '||engine_version||' '||container_image_digest from reg_export_runs limit 1")
assert_eq "E10 fingerprint is 64 hex, engine version is the engine's .source.sha256, the image digest is recorded" "$row" "64 $(tr -d '\n' <"$ROOT/submodules/constitution/scripts/workable-items/bin/.source.sha256") $(grep -A4 '^- id: IMG-TESTUTIL$' "$RUNP_LOCK" | grep -o 'sha256:[0-9a-f]*' | head -1)"
assert_eq "E11 each run has 5 written Markdown rows and 15 skipped sibling rows" "$(SQ "select status||':'||count(*) from reg_export_files where export_id=1 group by status order by status" | tr '\n' ' ')" "skipped_tool_absent:15 written:5 "
assert_eq "E12 skipped rows: html/pdf/docx, the sentinel sha256 of zeros" "$(SQ "select count(*) from reg_export_files where status='skipped_tool_absent' and sha256=printf('%064d',0) and format in ('html','pdf','docx')")" 30
ls "$D"/*.html "$D"/*.pdf "$D"/*.docx >/dev/null 2>&1 && bad "E13 a sibling file exists" || ok "E13 no HTML, PDF or DOCX file was faked"
echo "== drift check =="
out=$(ex --check --db docs/s.db --out-dir docs/register 2>&1); rc=$?
[ $rc -eq 0 ] && printf '%s' "$out" | grep -q '^export-check: OK' && ok "E14 --check on an unchanged export: OK, exit 0" || bad "E14 rc=$rc [$out]"
cp "$D/Issues.md" "$T_SCR/Issues.md.orig"; echo "hand edit" >>"$D/Issues.md"
out=$(ex --check --db docs/s.db --out-dir docs/register 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'STALE.*docs/register/Issues.md no longer matches its recorded sha256' && ok "E15 a twin edited after export: STALE, exit non-zero, the file named" || bad "E15 rc=$rc [$out]"
cp "$T_SCR/Issues.md.orig" "$D/Issues.md"; echo "x" >>"$D/legacy_id_collisions.csv"
out=$(ex --check --db docs/s.db --out-dir docs/register 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'legacy_id_collisions.csv no longer matches export-manifest.sha256' && ok "E16 a CSV edited after export: STALE through the manifest" || bad "E16 rc=$rc [$out]"
ex --db docs/s.db --out-dir docs/register >/dev/null 2>&1
lk "$R" -- sh -c "$(mint_cmd s.db)" >/dev/null 2>&1
out=$(ex --check --db docs/s.db --out-dir docs/register 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'STALE' && ok "E17 a database change after export: STALE (fingerprint and engine diff)" || bad "E17 rc=$rc [$out]"
ex --db docs/s.db --out-dir docs/register >/dev/null 2>&1
# a database change the engine's Markdown diff cannot see (a minted id with no item): only the fingerprint catches it
lk "$R" -- sqlite3 /src/docs/s.db "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('fp','manual')" >/dev/null 2>&1
out=$(ex --check --db docs/s.db --out-dir docs/register 2>&1); rc=$?
[ $rc -eq 1 ] && printf '%s' "$out" | grep -q 'the database content differs from the last export run (fingerprint)' && ok "E17b a change invisible to the engine diff (a minted id without an item): STALE through the fingerprint" || bad "E17b rc=$rc [$out]"
ex --db docs/s.db --out-dir docs/register >/dev/null 2>&1; out=$(ex --check --db docs/s.db --out-dir docs/register 2>&1); [ $? -eq 0 ] && ok "E18 after a re-export the check is OK again (golden-true)" || bad "E18 [$out]"
sha_before=$(fsha "$R/docs/s.db"); ex --check --db docs/s.db --out-dir docs/register >/dev/null 2>&1; assert_eq "E19 --check writes nothing: the database keeps its sha256" "$(fsha "$R/docs/s.db")" "$sha_before"
echo "== diff gate: an engine diff that is not in sync records nothing =="
R2=$(mkroot b); cdb "$R2" s.db; lk "$R2" -- sh -c "$(mint_cmd s.db)" >/dev/null 2>&1
B2="$R2/submodules/constitution/scripts/workable-items/bin"; mv "$B2/workable-items-linux" "$B2/workable-items-real"
printf '#!/bin/sh\ncase "$1" in diff) echo "diff: DB and Markdown DIFFER (golden-bad engine stub)"; exit 1;; esac\nexec "$(dirname "$0")/workable-items-real" "$@"\n' >"$B2/workable-items-linux"; chmod +x "$B2/workable-items-linux"
out=$(tool "$R2" "$EXPORT" --db docs/s.db --out-dir docs/register 2>&1); rc=$?
[ $rc -eq 5 ] && printf '%s' "$out" | grep -q 'not in sync' && ok "E20 an engine diff that is not in sync: exit 5" || bad "E20 rc=$rc [$out]"
assert_eq "E20b ... no reg_export_runs row recorded and no manifest written" "$(lk "$R2" --out "$T_SCR/o20" -- sqlite3 -readonly file:/src/docs/s.db?immutable=1 'select count(*) from reg_export_runs' 2>&1)$([ -e "$R2/docs/register/export-manifest.sha256" ] && echo M)" "0"
mv "$B2/workable-items-real" "$B2/workable-items-linux"; out=$(tool "$R2" "$EXPORT" --db docs/s.db --out-dir docs/register 2>&1); [ $? -eq 0 ] && ok "E20c golden-true: the same DB with the real engine exports" || bad "E20c [$out]"
echo "== refusals =="
for a in "--db ../x.db" "--db docs/x.txt" "--out-dir /abs" "--out-dir outside" "--out-dir docs/../x"; do
  out=$(ex $a 2>&1); rc=$?; [ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'REFUSED reason=path_invalid' && ok "E21 refused: $a" || bad "E21 $a rc=$rc [$out]"
done
ex --bogus >/dev/null 2>&1; assert_eq "E22 unknown argument: exit 2" "$?" 2
echo "== --check never checkpoints: a pending -wal file is refused =="
RW=$(mkroot w); cdb "$RW" s.db; lk "$RW" -- sh -c "$(mint_cmd s.db)" >/dev/null 2>&1; tool "$RW" "$EXPORT" --db docs/s.db --out-dir docs/register >/dev/null 2>&1
lk "$RW" -- bash -c 'sqlite3 /src/docs/s.db "select count(*) from reg_ids" ".shell sleep 8" >/dev/null 2>&1 & sleep 2; sqlite3 /src/docs/s.db "INSERT INTO reg_ids(minted_by,mint_basis) VALUES (\"walrow\",\"manual\")"; exit 0' >/dev/null 2>&1
if [ -s "$RW/docs/s.db-wal" ]; then ok "E27a fixture: a committed row is pending in docs/s.db-wal"; else bad "E27a fixture: no pending WAL"; fi
wsha=$(fsha "$RW/docs/s.db"); out=$(tool "$RW" "$EXPORT" --check --db docs/s.db --out-dir docs/register 2>&1); rc=$?
[ $rc -eq 20 ] && printf '%s' "$out" | grep -q 'register_not_checkpointed' && [ "$(fsha "$RW/docs/s.db")" = "$wsha" ] && ok "E27 --check on a database with a pending -wal file is refused register_not_checkpointed (20) and does not checkpoint it" || bad "E27 rc=$rc [$out]"
echo "== out mode (replay.sh) =="
mkdir -p "$T_SCR/om"; cp "$R/docs/s.db" "$T_SCR/om/copy.db"
out=$(ex --out-mode "$T_SCR/om" --db-file copy.db 2>&1); rc=$?
[ $rc -eq 0 ] && [ -f "$T_SCR/om/export/Issues.md" ] && cmp -s "$T_SCR/om/export/Issues.md" "$D/Issues.md" && ok "E23 out mode exports <dir>/export with the same Issues.md as register mode" || bad "E23 rc=$rc [$out]"
out=$(ex --out-mode "$T_SCR/om" --db-file copy.db --check 2>&1); [ $? -eq 0 ] && ok "E24 out mode --check is OK" || bad "E24 [$out]"
echo "== export header (information for docs/04 section 11, never a gate) =="
python3 -I - "${EV:-}" "$D/Issues.md" "$D/Issues_Summary.md" "$D/Reconciliation.md" <<'PY'
import json, sys
ev, *files = sys.argv[1:]
res = {}
for f in files:
    t = open(f, encoding="utf-8").read()
    res[f.rsplit("/", 1)[-1]] = {"has_revision": "**Revision:**" in t, "has_last_modified": "**Last modified:**" in t, "has_header_table": "| Revision |" in t}
out = {"information_only": True, "note": "export_revision.go replays the header of the last sync md-to-db import; a DB built only by add/export carries none; Reconciliation.md carries its own header table", "files": res}
if ev: json.dump(out, open(ev + "/export-header.json", "w"), indent=1, sort_keys=True)
print(json.dumps(res, sort_keys=True))
PY
ok "E26 header presence recorded as information (export-header.json), not asserted as a gate"
echo "== size (db-size.json) =="
if [ -n "${EV:-}" ]; then
  python3 -I - "$EV/db-size.json" "$R/docs/s.db" "$D" <<'PY'
import json,os,sys
out,db,d=sys.argv[1:]
sz=lambda p: os.path.getsize(p) if os.path.exists(p) else None
json.dump({"scratch_db_bytes":sz(db),"items":4,"export_files":{f:sz(os.path.join(d,f)) for f in sorted(os.listdir(d))},
 "note":"scratch measurement of a 4-item fixture only; the 2,010-item projection of T067 is owed to the T168 importer (UNCONFIRMED)"},open(out,"w"),indent=1,sort_keys=True)
PY
  [ -s "$EV/db-size.json" ] && ok "E25 db-size.json written (fixture measurement; projection owed)" || bad "E25"
fi
finish
