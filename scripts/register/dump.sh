#!/usr/bin/env bash
# dump.sh - T066 (docs/04 section 12.1 R-6): the deterministic text dump docs/register/register.sql of the register database.
# Usage: dump.sh [--db docs/<name>.db] [--out docs/<dir>/<name>.sql]          register mode (defaults docs/workable_items.db, docs/register/register.sql)
#        dump.sh --out-dir <absolute dir> [--db-file <name>] [--out <name>]   out mode: the database and the dump both live in <dir> (replay.sh)
# One scripts/register/locked.sh call (the register lock, the image's sqlite3): PRAGMA wal_checkpoint(TRUNCATE) first (R-2), then
# `sqlite3 'file:<db>?immutable=1' .dump` with the `PRAGMA` lines removed, written to a temporary file and renamed into place. The immutable read
# is exact because it runs right after the checkpoint with no other writer (the lock); the dump is derived, never authoritative, and never edited
# by hand. Two consecutive dumps of an unchanged database are byte-identical (tests/test_dump.sh).
# Commit procedure (written down here, run in T069 and by scripts/commit-push-all.sh, docs/04 R-2, R-3): (1) scripts/register/export.sh (checkpoint
# first, engine export, reconcile.sh, reg_export_* rows, engine diff = in sync); (2) this script (checkpoint first; it runs AFTER the export because the
# export adds its own rows to the database and the dump must equal the database that is committed); (3) the DATABASE, the dump and the
# regenerated documents are committed together in ONE commit through scripts/commit-push-all.sh (never git commit by hand): a database
# without its dump and exports, or exports without their database, is the drift this procedure exists to prevent.
# Checkpoint (WF10 F10, docs/04 12.2): the result row of PRAGMA wal_checkpoint(TRUNCATE) is checked (busy must be 0) and the -wal file must be empty afterwards, else
# the call is REFUSED checkpoint_incomplete (20) and no dump is written: any other open connection (a reader outside the lock) blocks the TRUNCATE, and an immutable
# read would then miss the committed rows still held in the -wal file.
# Output check (WF10 F8): after the wrapper exits 0 the dump must exist on the host, be newer than the start of this run, and end with COMMIT; (else exit 1,
# FAILED reason=output_missing / output_incomplete); the LOCKED hook is honoured only with LOCKED_TEST_MODE=1.
# Exit: 0 ok; 1 the wrapper reported success but produced no complete dump; 2 usage; 20 refused (path, or checkpoint_incomplete); otherwise the wrapper's status.
set -u
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REAL_ROOT="$(cd "$SELF_DIR/../.." && pwd)"
if [ "${LOCKED_TEST_MODE:-}" = 1 ]; then LOCKED="${LOCKED:-$SELF_DIR/locked.sh}"; ROOT="${LOCKED_ROOT:-$REAL_ROOT}"   # MUT:hook-gate
else unset LOCKED; LOCKED="$SELF_DIR/locked.sh"; ROOT="$REAL_ROOT"; fi
DB=docs/workable_items.db; OUT=docs/register/register.sql; OUTDIR=""; DBFILE=workable_items.db; OUTNAME=register.sql
usage() { echo "usage: dump.sh [--db docs/<name>.db] [--out docs/<dir>/<name>.sql] | dump.sh --out-dir <abs dir> [--db-file <name>] [--out <name>]" >&2; exit 2; }
refuse() { echo "dump: REFUSED reason=$1 ${2:-}" >&2; exit 20; }
while [ $# -gt 0 ]; do case "$1" in
  --db) [ $# -ge 2 ] || usage; DB="$2"; shift 2;;
  --out) [ $# -ge 2 ] || usage; OUT="$2"; OUTNAME="$2"; shift 2;;
  --out-dir) [ $# -ge 2 ] || usage; OUTDIR="$2"; shift 2;;
  --db-file) [ $# -ge 2 ] || usage; DBFILE="$2"; shift 2;;
  *) usage;; esac; done
safe() { case "$1" in ""|*[!A-Za-z0-9_./-]*|/*|*..*|-*) return 1;; esac; return 0; }
if [ -n "$OUTDIR" ]; then
  case "$OUTDIR" in /*) ;; *) usage;; esac
  for n in "$DBFILE" "$OUTNAME"; do case "$n" in ""|*[!A-Za-z0-9_.-]*|-*|.*) refuse path_invalid "--db-file/--out must be plain file names";; esac; done
  CDB="/out/$DBFILE"; COUT="/out/$OUTNAME"; WRAP=(--out "$OUTDIR"); HOSTOUT="$OUTDIR/$OUTNAME"
else
  safe "$DB" && safe "$OUT" || refuse path_invalid "--db and --out must be repository-relative paths of [A-Za-z0-9_./-]"
  case "$DB" in docs/*.db) ;; *) refuse path_invalid "--db must be docs/<name>.db";; esac
  case "$OUT" in docs/*.sql) ;; *) refuse path_invalid "--out must be docs/<path>.sql";; esac
  CDB="/src/$DB"; COUT="/src/$OUT"; WRAP=(); HOSTOUT="$ROOT/$OUT"
fi
SCRIPT='# register-regenerate:dump
set -eo pipefail; db='"$CDB"'; out='"$COUT"'
r=$(sqlite3 "$db" "PRAGMA wal_checkpoint(TRUNCATE);") || { echo "dump: REFUSED reason=checkpoint_incomplete the checkpoint statement failed" >&2; exit 20; }
case "$r" in 0\|*) ;; *) echo "dump: REFUSED reason=checkpoint_incomplete wal_checkpoint(TRUNCATE) returned [$r], another connection holds the database" >&2; exit 20;; esac   # MUT:checkpoint-result
[ ! -s "$db-wal" ] || { echo "dump: REFUSED reason=checkpoint_incomplete the -wal file is not empty after the checkpoint" >&2; exit 20; }
mkdir -p "$(dirname "$out")"
sqlite3 -readonly "file:$db?immutable=1" .dump | grep -v "^PRAGMA" > "$out.tmp"   # MUT:pragma-filter
mv -f "$out.tmp" "$out"'
STARTM="$(mktemp "${TMPDIR:-/tmp}/dump-start.XXXXXX")" || { echo "dump: cannot create a temporary marker" >&2; exit 1; }; trap 'rm -f "$STARTM"' EXIT; sleep 0.01
"$LOCKED" "${WRAP[@]}" -- bash -c "$SCRIPT"
rc=$?
if [ $rc -eq 0 ]; then
  [ -s "$HOSTOUT" ] && [ "$HOSTOUT" -nt "$STARTM" ] || { echo "dump: FAILED reason=output_missing the wrapper exited 0 but $HOSTOUT was not (re)written by this run" >&2; exit 1; }   # MUT:output-check
  [ "$(tail -n1 "$HOSTOUT")" = "COMMIT;" ] || { echo "dump: FAILED reason=output_incomplete $HOSTOUT does not end with COMMIT;" >&2; exit 1; }
fi
if [ $rc -eq 0 ]; then if [ -n "$OUTDIR" ]; then echo "dump: OK $OUTDIR/$DBFILE -> $OUTDIR/$OUTNAME"; else echo "dump: OK $DB -> $OUT"; fi; fi
exit $rc
