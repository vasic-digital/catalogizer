#!/usr/bin/env bash
# lib.sh - shared helpers of the scripts/register tests (WP-06, tasks T060-T063).
# Real SQLite, real engine binary, scratch DBs only: no test writes docs/workable_items.db.
# Env: SQLITE3 (default: sqlite3 on PATH), WI (engine binary), REG_SCRATCH (scratch base dir).
# HOST-SIDE RUN NOTICE (UNCONFIRMED container leg): RUNP/IMG-TESTUTIL (T007/T008) do not exist yet, so the
# tests run with the host sqlite3 and the committed engine binary. The identity header says so.
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)
REG_DIR="$ROOT/scripts/register"
WI=${WI:-$ROOT/submodules/constitution/scripts/workable-items/bin/workable-items-linux}
SQLITE3=${SQLITE3:-sqlite3}
PASS=0; FAIL=0
T_SCR=$(mktemp -d "${REG_SCRATCH:-${TMPDIR:-/tmp}}/regtest.XXXXXX")
trap 'rm -rf "$T_SCR"' EXIT

ident_header() {  # identity header for evidence transcripts (never a credential, no env dump)
  echo "# identity: task=${1:-?} utc=$(date -u +%Y-%m-%dT%H:%M:%SZ) host=$(hostname -s) uid=$(id -u)"
  echo "# git_head=$(git -C "$ROOT" rev-parse HEAD 2>/dev/null) tree_dirty_files=$(git -C "$ROOT" status --porcelain 2>/dev/null | wc -l)"
  echo "# sqlite3=$($SQLITE3 --version 2>&1 | cut -d' ' -f1,2) path=$(command -v "$SQLITE3")"
  echo "# engine=$WI sha256=$(sha256sum "$WI" 2>/dev/null | cut -d' ' -f1)"
  # the scripts and the test are uncommitted: HEAD does not identify them, their content hashes do (WF2 m-8)
  echo "# sha256 ddl=$(sha256sum "${REG_EXT_SQL:-$REG_DIR/register_ext.sql}" 2>/dev/null | cut -c1-64) apply_ext=$(sha256sum "$REG_DIR/apply_ext.sh" 2>/dev/null | cut -c1-64) gate=$(sha256sum "${GATE:-$REG_DIR/gate.sh}" 2>/dev/null | cut -c1-64) test=$(sha256sum "${BASH_SOURCE[1]:-$0}" 2>/dev/null | cut -c1-64)"
  echo "# container_image_digest=UNCONFIRMED (RUNP/IMG-TESTUTIL absent: host-side run, T007/T008 pending)"
}
ok()   { PASS=$((PASS+1)); echo "ok   $*"; }
bad()  { FAIL=$((FAIL+1)); echo "FAIL $*"; }
# sql DB "stmt..." -> stdout; foreign keys ON per connection (docs/04 §5 limitation 7); stderr merged
sql()  { local db=$1; shift; "$SQLITE3" -bail "$db" "PRAGMA foreign_keys=ON; $*" 2>&1; }
sqlq() { local db=$1; shift; "$SQLITE3" -bail "$db" "PRAGMA foreign_keys=ON;" "$*" 2>&1 | grep -v '^$'; }
# expect_ok DB LABEL STMT
expect_ok() { local out; out=$(sql "$1" "$3"); if [ $? -eq 0 ]; then ok "$2"; else bad "$2 :: $out"; fi; }
# expect_reject DB LABEL PATTERN STMT : statement must fail AND the message must match PATTERN (so the
# rejection is the intended one, not an unrelated error)
expect_reject() { local out rc; out=$(sql "$1" "$4"); rc=$?
  if [ $rc -ne 0 ] && printf '%s' "$out" | grep -Eq "$3"; then ok "$2"; 
  else bad "$2 :: rc=$rc out=[$out] wanted /$3/"; fi; }
assert_eq() { if [ "$2" = "$3" ]; then ok "$1"; else bad "$1 :: got [$2] want [$3]"; fi; }
fresh_ext_db() {  # fresh engine DB + ext applied through apply_ext.sh
  local db=$1; rm -f "$db" "$db"-wal "$db"-shm; "$REG_DIR/apply_ext.sh" --db "$db" >"$T_SCR/apply.out" 2>&1 || { echo "fresh_ext_db failed: $(cat "$T_SCR/apply.out")"; return 1; }
}
mint() { sql "$1" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('${2:-tester}','manual'); SELECT atm_id FROM reg_ids ORDER BY seq DESC LIMIT 1;" | tail -1; }
finish() { echo "RESULT pass=$PASS fail=$FAIL"; [ "$FAIL" -eq 0 ]; }
