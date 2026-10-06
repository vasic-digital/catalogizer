#!/usr/bin/env bash
# export.sh - T067 (docs/04 section 11.2, 11.3): export and reconciliation of the register, and the drift check.
# Usage: export.sh [--db docs/<name>.db] [--out-dir docs/<dir>]            register mode (defaults docs/workable_items.db, docs/register)
#        export.sh --check [--db ...] [--out-dir ...]                      read-only drift check (STALE / OK)
#        export.sh --out-mode <absolute dir> [--db-file <name>] [--check]  out mode: the database is <dir>/<name>, outputs <dir>/export (replay.sh)
# Export (one scripts/register/locked.sh call, the image's sqlite3 and engine): PRAGMA wal_checkpoint(TRUNCATE); the engine's
# `export --db <db> --out-dir <dir>` (Issues.md, Fixed.md, Issues_Summary.md, Fixed_Summary.md; `--no-formats` while the image has no pandoc: in P0
# the HTML, PDF and DOCX siblings are produced by the remote `docs render` lane of IMG-DOCS from P2 on, T121a, T176); scripts/register/reconcile.sh
# (the CSV files and Reconciliation.md); `export-manifest.sha256` (sha256 of every Markdown and CSV output); the engine's
# `diff --db <db> --issues <dir>/Issues.md --fixed <dir>/Fixed.md`, which must print "in sync" (else exit 5, no row recorded); then ONE
# reg_export_runs row (db fingerprint, engine version = the engine's .source.sha256, container image digest, verdict OK) and one reg_export_files
# row per Markdown output ('written') and per HTML, PDF and DOCX sibling ('written' with its sha256 when the file exists, else
# 'skipped_tool_absent' with the sentinel sha256 of 64 zeros: the schema requires 64 characters, and no sibling file is ever faked).
# db_fingerprint: DEVIATION from docs/04 section 11.3 ("sha256 of the checkpointed .db"): the recorded value is the sha256 of the canonical
# `.dump` WITHOUT the reg_export_runs / reg_export_files rows and their sqlite_sequence rows, because the file hash changes with the run's own
# bookkeeping rows, which would make every later check read STALE (owed to the docs/04 owner). The CSV outputs have no reg_export_files row:
# the schema's `format` enum is md/html/pdf/docx; their sha256 are held by export-manifest.sha256 (owed to docs/04).
# --check: refuses (20 register_not_checkpointed) while <db>-wal is non-empty (never checkpoints); copies the database file to /tmp inside the
# image and reports STALE (exit 1, the reasons named) when the fingerprint differs from the last export run, a recorded 'written' file or a
# manifest line no longer matches the file on disk, or the engine diff does not say "in sync"; OK (exit 0) otherwise. No row is written.
# Exit: 0 ok; 1 STALE; 2 usage; 5 engine diff not in sync; 20 refused. Needs: locked.sh (+ RUNP, podman, IMG-TESTUTIL).
set -u
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REAL_ROOT="$(cd "$SELF_DIR/../.." && pwd)"
LOCKED="${LOCKED:-$SELF_DIR/locked.sh}"
DB=docs/workable_items.db; OUTD=docs/register; OUTMODE=""; DBFILE=workable_items.db; CHECK=0
usage() { echo "usage: export.sh [--check] [--db docs/<name>.db] [--out-dir docs/<dir>] | export.sh --out-mode <abs dir> [--db-file <name>] [--check]" >&2; exit 2; }
refuse() { echo "export: REFUSED reason=$1 ${2:-}" >&2; exit 20; }
while [ $# -gt 0 ]; do case "$1" in
  --check) CHECK=1; shift;;
  --db) [ $# -ge 2 ] || usage; DB="$2"; shift 2;;
  --out-dir) [ $# -ge 2 ] || usage; OUTD="$2"; shift 2;;
  --out-mode) [ $# -ge 2 ] || usage; OUTMODE="$2"; shift 2;;
  --db-file) [ $# -ge 2 ] || usage; DBFILE="$2"; shift 2;;
  *) usage;; esac; done
WIIN=/src/submodules/constitution/scripts/workable-items/bin/workable-items-linux
if [ -n "$OUTMODE" ]; then
  case "$OUTMODE" in /*) ;; *) usage;; esac
  case "$DBFILE" in ""|*[!A-Za-z0-9_.-]*|-*|.*) refuse path_invalid "--db-file must be a plain file name";; esac
  CDB="/out/$DBFILE"; COUT=/out/export; PFX=export; MNT=/out; WRAP=(--out "$OUTMODE"); DBLABEL="$DBFILE"
else
  for p in "$DB" "$OUTD"; do case "$p" in ""|*[!A-Za-z0-9_./-]*|/*|*..*|-*) refuse path_invalid "$p";; esac; done
  case "$DB" in docs/*.db) ;; *) refuse path_invalid "--db must be docs/<name>.db";; esac
  case "$OUTD" in docs/*) ;; *) refuse path_invalid "--out-dir must be under docs/";; esac
  CDB="/src/$DB"; COUT="/src/$OUTD"; PFX="$OUTD"; MNT=/src; WRAP=(); DBLABEL="$DB"
fi
DIGEST="$(grep -A4 '^- id: IMG-TESTUTIL$' "${RUNP_LOCK:-$REAL_ROOT/build/containers/images.lock.yaml}" | grep -o 'sha256:[0-9a-f]*' | head -1)"
MDS="Issues Fixed Issues_Summary Fixed_Summary Reconciliation"
CSVS="reconciliation unmapped_entries legacy_id_collisions reverify_queue reopen_counts stale_tracker_sync findings"
FPCMD="grep -Ev \"^INSERT INTO \\\"?reg_export_|^INSERT INTO \\\"?sqlite_sequence\\\"? VALUES\\(.reg_export_\" | sha256sum | cut -d' ' -f1"
sub() { local t="$1"; t="${t//@DB@/$CDB}"; t="${t//@D@/$COUT}"; t="${t//@WI@/$WIIN}"; t="${t//@MNT@/$MNT}"; t="${t//@PFX@/$PFX}"; t="${t//@DBLABEL@/$DBLABEL}"
  t="${t//@DIGEST@/$DIGEST}"; t="${t//@MDS@/$MDS}"; t="${t//@CSVS@/$CSVS}"; t="${t//@FPCMD@/$FPCMD}"; printf '%s' "$t"; }
read -r -d '' CHECK_TPL <<'EOS'
# register-regenerate:export-check
set -uo pipefail; db=@DB@; d=@D@; WI=@WI@; mnt=@MNT@
[ ! -s "$db-wal" ] || { echo "export-check: REFUSED reason=register_not_checkpointed (the register is not checkpointed: a check never checkpoints)"; exit 20; }
cp "$db" /tmp/chk.db || { echo "export-check: cannot read $db"; exit 20; }
P=""
fp=$(sqlite3 /tmp/chk.db .dump | @FPCMD@)
row=$(sqlite3 /tmp/chk.db "select export_id||'|'||db_fingerprint from reg_export_runs order by export_id desc limit 1" 2>&1)
if [ -z "$row" ]; then echo "export-check: STALE: no export run is recorded"; exit 1; fi
eid=${row%%|*}; rfp=${row#*|}
[ "$fp" = "$rfp" ] || P="$P; the database content differs from the last export run (fingerprint)"   # MUT:check-fingerprint
while IFS='|' read -r path sha st; do
  [ "$st" = written ] || continue
  f="$mnt/$path"
  if [ ! -f "$f" ]; then P="$P; recorded file $path is missing"; continue; fi
  [ "$(sha256sum "$f" | cut -d' ' -f1)" = "$sha" ] || P="$P; $path no longer matches its recorded sha256"   # MUT:check-file-hash
done < <(sqlite3 /tmp/chk.db "select path,sha256,status from reg_export_files where export_id=$eid order by path")
if [ -f "$d/export-manifest.sha256" ]; then
  while read -r h n; do [ "$(sha256sum "$d/$n" 2>/dev/null | cut -d' ' -f1)" = "$h" ] || P="$P; $n no longer matches export-manifest.sha256"; done < "$d/export-manifest.sha256"   # MUT:check-manifest
else P="$P; export-manifest.sha256 is missing"; fi
dv=$("$WI" diff --db /tmp/chk.db --issues "$d/Issues.md" --fixed "$d/Fixed.md" 2>&1)
case "$dv" in *"in sync"*) ;; *) P="$P; engine diff: $(printf '%s' "$dv" | head -1 | cut -c1-160)";; esac
if [ -n "$P" ]; then echo "export-check: STALE${P}"; exit 1; fi
echo "export-check: OK (export_id $eid, fingerprint $fp; $dv)"
EOS
read -r -d '' EXPORT_TPL <<'EOS'
# register-regenerate:export
set -eo pipefail; db=@DB@; d=@D@; WI=@WI@
sqlite3 "$db" "PRAGMA wal_checkpoint(TRUNCATE);" >/dev/null
mkdir -p "$d"
fp=$(sqlite3 "$db" .dump | @FPCMD@)
if command -v pandoc >/dev/null 2>&1; then FMT=""; else FMT=--no-formats; fi
"$WI" export --db "$db" --out-dir "$d" $FMT >/dev/null
bash /src/scripts/register/reconcile.sh --db "$db" --out-dir "$d"
dv=$("$WI" diff --db "$db" --issues "$d/Issues.md" --fixed "$d/Fixed.md" 2>&1) || true
echo "$dv"
case "$dv" in *"in sync"*) ;; *) echo "export: engine diff is not in sync: no export recorded" >&2; exit 5;; esac   # MUT:diff-gate
: > "$d/.manifest.tmp"
for n in @MDS@; do h=$(sha256sum "$d/$n.md" | cut -d' ' -f1); echo "$h $n.md" >> "$d/.manifest.tmp"; done
for n in @CSVS@; do h=$(sha256sum "$d/$n.csv" | cut -d' ' -f1); echo "$h $n.csv" >> "$d/.manifest.tmp"; done
mv -f "$d/.manifest.tmp" "$d/export-manifest.sha256"
ev=$(tr -d '\n' < /src/submodules/constitution/scripts/workable-items/bin/.source.sha256)
S="BEGIN; INSERT INTO reg_export_runs(db_fingerprint,engine_version,container_image_digest,started_at,verdict) VALUES ('$fp','$ev','@DIGEST@',strftime('%Y-%m-%dT%H:%M:%SZ','now'),'OK');"
Z=0000000000000000000000000000000000000000000000000000000000000000
for n in @MDS@; do
  h=$(sha256sum "$d/$n.md" | cut -d' ' -f1)
  S="$S INSERT INTO reg_export_files VALUES ((SELECT max(export_id) FROM reg_export_runs),'@PFX@/$n.md','md','$h','@DBLABEL@','written');"
  for ext in html pdf docx; do
    if [ -f "$d/$n.$ext" ]; then s2=$(sha256sum "$d/$n.$ext" | cut -d' ' -f1); st=written; else s2=$Z; st=skipped_tool_absent; fi
    S="$S INSERT INTO reg_export_files VALUES ((SELECT max(export_id) FROM reg_export_runs),'@PFX@/$n.$ext','$ext','$s2','@PFX@/$n.md','$st');"
  done
done
sqlite3 "$db" "$S COMMIT;"
echo "export: OK fingerprint=$fp files=$(ls "$d" | wc -l)"
EOS
if [ "$CHECK" = 1 ]; then
  CO="${OUTMODE:-$REAL_ROOT/.audit/out/export-check-$$}"; mkdir -p -- "$CO"
  "$LOCKED" --out "$CO" -- bash -c "$(sub "$CHECK_TPL")"
  exit $?
fi
"$LOCKED" "${WRAP[@]}" -- bash -c "$(sub "$EXPORT_TPL")"
exit $?
