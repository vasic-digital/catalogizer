#!/usr/bin/env bash
# backup_db.sh - T064a (docs/04 section 12.2 bulk-write discipline): the ONE pre-op backup helper of docs/workable_items.db.
# Usage: backup_db.sh --record <file>
# A hardlinked copy is not a backup: SQLite writes its pages in place, so `cp -al` shares the inode and the "backup" follows every later
# write. The backup is taken with the SQLite online backup (`.backup`) by the image's sqlite3 (IMG-TESTUTIL), never with cp -al (reserved for .git).
#   1. ONE scripts/register/locked.sh call (the register lock taken once, or inherited when LOCKED_LOCK_HELD is set by an import-sql run) runs ONE
#      sqlite3 script inside the image: PRAGMA wal_checkpoint(TRUNCATE); .backup docs/workable_items.db.bak-<UTC>; and a canonical `.dump` of
#      the source into the op's /out (the source state under the lock).
#   2. afterwards, read-only through the immutable=1 URI (creates no -shm/-wal beside the backup): PRAGMA integrity_check must print ok on the
#      backup, and a restore probe (the backup restored into a scratch database under /out) must dump to the same bytes as the source dump of 1.
#   3. writes the record --record <file> (backup_path, utc, both sha256 values, both row counts (INSERT rows of the canonical dumps), the
#      integrity result, the restore-probe result, the image digest, op ids); it exits non-zero, removes the backup file and writes NO record
#      when any check fails: a backup that fails a check is not a backup and the bulk step does not start.
# Name and hash (WF10 F7, F14): the backup is docs/workable_items.db.bak-<UTC>-<pid>-<ns>, created with O_EXCL (noclobber) before anything runs, so two backups started
# in the same second never share a file and fail() removes only the file this run created; source_sha256 is computed INSIDE step 1 (under the lock, right after the
# online backup, by the image's sha256sum) and read back from the op's /out, never from the source after the lock was released.
# Checkpoint (WF13 N7): the result row of PRAGMA wal_checkpoint(TRUNCATE) must be 0|...; a busy checkpoint (a reader outside the lock) is REFUSED checkpoint_incomplete, because source_sha256 is the
# main-file hash and would name a state without the WAL pages the backup holds. Every failure after the O_EXCL pre-creation removes the file (die as well as fail, WF13 N6).
# Exit: 0 ok; 1 a check failed or the wrapper failed; 2 usage. Test hooks (need LOCKED_TEST_MODE=1, ignored otherwise): LOCKED_ROOT, LOCKED_RUNP, LOCKED, BACKUP_FAULT=truncate|dumpdiff.
set -u
SELF_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REAL_ROOT="$(cd "$SELF_DIR/../.." && pwd)"
if [ "${LOCKED_TEST_MODE:-}" = 1 ]; then LOCKED="${LOCKED:-$SELF_DIR/locked.sh}"; RUNP="${LOCKED_RUNP:-$REAL_ROOT/scripts/containers/run_pinned.sh}"   # MUT:hook-gate
else unset LOCKED LOCKED_RUNP LOCKED_ROOT BACKUP_FAULT; LOCKED="$SELF_DIR/locked.sh"; RUNP="$REAL_ROOT/scripts/containers/run_pinned.sh"; fi
OWN=0
die() { [ "$OWN" = 1 ] && rm -f -- "$BAKREL"; echo "backup_db: $*" >&2; exit 1; }   # a failure after the O_EXCL pre-creation must not leave the 0-byte file (WF13 N6)
REC=""
while [ $# -gt 0 ]; do case "$1" in
  --record) [ $# -ge 2 ] || { echo "backup_db: --record needs a value" >&2; exit 2; }; REC="$2"; shift 2;;
  *) echo "backup_db: unknown argument '$1'" >&2; echo "usage: backup_db.sh --record <file>" >&2; exit 2;; esac; done
[ -n "$REC" ] || { echo "usage: backup_db.sh --record <file>" >&2; exit 2; }
if [ "${LOCKED_TEST_MODE:-}" = 1 ]; then ROOT="${LOCKED_ROOT:-$REAL_ROOT}"; else ROOT="$REAL_ROOT"; fi
cd "$ROOT" || die "cannot enter $ROOT"
DBREL=docs/workable_items.db
[ -f "$DBREL" ] || die "REFUSED no source database $DBREL"
UTC="$(date -u +%Y%m%dT%H%M%SZ)"; NS="$(date -u +%N)"
BAKREL="$DBREL.bak-$UTC-$$-$NS"
OP="backup-$UTC-$$-$NS"; OPV="$OP-verify"
( set -o noclobber; : >"$BAKREL" ) 2>/dev/null || die "REFUSED backup $BAKREL already exists or cannot be created"   # MUT:backup-excl
OWN=1
OUT1=".audit/out/$OP"; mkdir -p "$OUT1" || die "cannot create $OUT1"
fail() { [ "$OWN" = 1 ] && rm -f -- "$BAKREL"   # MUT:fail-keeps-file
  echo "backup_db: BACKUP FAILED: $*" >&2; exit 1; }
# ---- 1. under the lock: checkpoint, online backup, canonical source dump (one sqlite3 script, one wrapper call) ----
"$LOCKED" --op-id "$OP" -- sqlite3 "/src/$DBREL" "PRAGMA wal_checkpoint(TRUNCATE);" ".backup /src/$BAKREL" ".output /out/source.dump.sql" ".dump" ".output stdout" ".shell sha256sum /src/$DBREL >/out/source.sha256" >"$OUT1/step1.out" 2>"$OUT1/step1.err"   # MUT:backup-method
rc=$?
[ $rc -eq 0 ] || fail "step 1 (checkpoint, backup, dump) exited $rc: $(head -c 300 "$OUT1/step1.err")"
[ -s "$BAKREL" ] || fail "step 1 left no backup file"
[ -s "$OUT1/source.dump.sql" ] || fail "step 1 wrote no source dump"
CKP="$(head -1 "$OUT1/step1.out" 2>/dev/null)"   # the result row of PRAGMA wal_checkpoint(TRUNCATE): busy|log|checkpointed (WF13 N7, as dump.sh F10)
case "$CKP" in 0\|*) ;; *) fail "REFUSED reason=checkpoint_incomplete wal_checkpoint(TRUNCATE) returned [$CKP]: another connection holds the database, so source_sha256 would name a state without the WAL pages the backup holds";; esac   # MUT:checkpoint-result
SSHA="$(head -1 "$OUT1/source.sha256" 2>/dev/null | cut -d' ' -f1)"
case "$SSHA" in *[!0-9a-f]*|"") fail "step 1 did not record the source sha256 under the lock";; esac; [ "${#SSHA}" -eq 64 ] || fail "step 1 recorded a malformed source sha256 [$SSHA]"
if [ "${BACKUP_FAULT:-}" = truncate ]; then : >"$BAKREL"; head -c 200 /dev/urandom >"$BAKREL"; fi
# ---- 2. read-only checks (immutable URI, no side files), restore probe into a scratch database under /out ----
OUT2=".audit/out/$OPV"; mkdir -p "$OUT2"
"$LOCKED" --op-id "$OPV" --out "$ROOT/$OUT2" -- sh -c '
  set -e
  b="file:/src/'"$BAKREL"'?immutable=1"
  ic=$(sqlite3 -readonly "$b" "PRAGMA integrity_check" 2>&1 | head -1); echo "$ic" > /out/integrity.txt
  [ "$ic" = ok ] || exit 0
  sqlite3 -readonly "$b" ".backup /out/restored.db" && sqlite3 /out/restored.db ".output /out/restored.dump.sql" ".dump"
' >"$OUT2/step2.out" 2>"$OUT2/step2.err"
rc=$?
[ $rc -eq 0 ] || fail "step 2 (integrity, restore probe) exited $rc: $(head -c 300 "$OUT2/step2.err")"
INTEG="$(head -1 "$OUT2/integrity.txt" 2>/dev/null)"
[ "$INTEG" = ok ] || fail "PRAGMA integrity_check on the backup printed [$INTEG], not ok"
[ -s "$OUT2/restored.dump.sql" ] || fail "restore probe produced no dump"
[ "${BACKUP_FAULT:-}" != dumpdiff ] || echo '-- injected difference' >>"$OUT2/restored.dump.sql"
cmp -s "$OUT1/source.dump.sql" "$OUT2/restored.dump.sql" || fail "restore probe: the restored backup's canonical dump differs from the source dump"
SROWS="$(grep -c '^INSERT INTO' "$OUT1/source.dump.sql")"; BROWS="$(grep -c '^INSERT INTO' "$OUT2/restored.dump.sql")"
[ "$SROWS" = "$BROWS" ] || fail "row counts differ: source $SROWS, backup $BROWS"
# ---- 3. the record ----
DIGEST="$(grep -A4 '^- id: IMG-TESTUTIL$' "${RUNP_LOCK:-$REAL_ROOT/build/containers/images.lock.yaml}" | grep -o 'sha256:[0-9a-f]*' | head -1)"
BSHA="$(sha256sum -- "$BAKREL" | cut -d' ' -f1)"
mkdir -p -- "$(dirname "$REC")"
TMP="$REC.tmp.$$"
python3 -I - "$TMP" "$BAKREL" "$UTC" "$SSHA" "$BSHA" "$SROWS" "$BROWS" "$INTEG" "$DIGEST" "$OP" "$OPV" <<'PY' || fail "cannot write the record"
import json, sys
t, bak, utc, ss, bs, sr, br, ig, dg, op, opv = sys.argv[1:]
json.dump({"backup_path": bak, "utc": utc, "source_sha256": ss, "backup_sha256": bs, "source_rows": int(sr), "backup_rows": int(br),
           "integrity": ig, "restore_probe": "equal", "image_digest": dg, "op_ids": [op, opv], "method": "sqlite3 .backup (online backup), image sqlite3"},
          open(t, "w"), indent=1, sort_keys=True)
PY
mv -f -- "$TMP" "$REC" || fail "cannot place the record"
echo "backup_db: OK $BAKREL sha256=$BSHA rows=$BROWS integrity=ok restore_probe=equal"
