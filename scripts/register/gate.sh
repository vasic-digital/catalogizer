#!/usr/bin/env bash
# gate.sh - register gate, docs/04 §12.3 step 2, and with --completion the completion gate of step 3.
# Usage: gate.sh --db <db> [--ext <register_ext.sql>] [--allow-url-evidence] [--completion]
# Steps (each prints FAIL <what> and sets rc=1; the run continues so one report lists every failure):
#   engine validate | PRAGMA integrity_check = ok | PRAGMA foreign_key_check empty |
#   v_gate_missing_objects empty | every reg_gate_checks view_empty view returns 0 rows |
#   schema and seed equal a fresh reference built from register_ext.sql, over EVERY schema object (an added
#   trigger, view, index or table of any name is an unexpected object, WF2 I-1) | engine schema_version equals
#   reg_meta.engine_schema_required (m-4) | every cited reg_evidence file re-hashes to its recorded sha256 (I-3).
# The DB under test is never modified: every read goes through a read-only copy made with the SQLite online
# backup taken from a read-only connection (a WAL register keeps its -wal untouched, m-2); the engine's own
# `validate` may migrate, so it runs on the copy too. Prints `GATE OK` only when every step passed.
# Evidence paths (WF3 I2): only a tracker_receipt may cite a non-file (URL-shaped) path; a URL-shaped path on any
# other kind is a FAIL (a producer cannot opt out of the re-hash by choosing a path shape). A URL-shaped
# tracker_receipt is counted, never claimed verified: the run then ends `GATE PASS-PARTIAL url_skipped=N` (docs/04
# section 7.1(a), K-11: the re-hash is the compensating control, so a run that skipped paths is partial) with exit 3,
# or exit 0 (still PASS-PARTIAL, never a bare GATE OK) when --allow-url-evidence is given.
# --completion (T180, docs/04 §12.3 step 3, §13.3): after the register steps, the FEATURE COMPLETION gate: every row of every
#   reg_gate_checks view_not_done view is open work and counts as NOT done. The list of views is read from the trusted reference
#   (like view_empty), so a row deleted from the DB under test cannot hide the queue (the seed diff FAILs it as well). Prints one
#   `NOT DONE <view> rows=N` line per populated view (name order), then `COMPLETION NOT DONE views=V rows=R`, exit 4. A clean run
#   prints `COMPLETION OK`. A reference with no view_not_done row, a name that is not a view, or a count that is not a number is a
#   FAIL. Without --completion the register gate is unchanged (view_not_done rows are not counted).
# Exit 0 ok (or PASS-PARTIAL allowed), 1 gate failure, 2 usage, 3 PASS-PARTIAL not allowed, 4 NOT DONE (--completion only; a
#   register FAIL (1) outranks it, it outranks PASS-PARTIAL (3)).
# Env: WI, SQLITE3 (the container image's binaries once RUNP exists), REG_ROOT (tree root, for tests; also the
#      base of a relative reg_evidence.path), REG_EXT_SQL (DDL reference, for the mutation runner).
set -u
export LC_ALL=C   # sort and comm must agree on the collation
ROOT=${REG_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}
WI=${WI:-$ROOT/submodules/constitution/scripts/workable-items/bin/workable-items-linux}
SQLITE3=${SQLITE3:-sqlite3}
DB=""; EXT=${REG_EXT_SQL:-$ROOT/scripts/register/register_ext.sql}; ALLOW_URL=0; COMPLETION=0
while [ $# -gt 0 ]; do case "$1" in
  --allow-url-evidence) ALLOW_URL=1; shift;;
  --completion) COMPLETION=1; shift;;
  --db|--ext) [ $# -ge 2 ] || { echo "gate: $1 needs a value" >&2; exit 2; }   # a missing value must not loop (m-1)
              if [ "$1" = --db ]; then DB=$2; else EXT=$2; fi; shift 2;;
  *) echo "gate: unknown argument $1" >&2; exit 2;; esac; done
[ -n "$DB" ] || { echo "usage: gate.sh --db <db> [--ext <ddl>] [--allow-url-evidence] [--completion]" >&2; exit 2; }
[ -f "$DB" ] || { echo "FAIL database file absent: $DB" >&2; exit 1; }
rc=0
# a message may carry text read from the database under test: newlines and carriage returns are flattened so no
# data can start a line of its own, in particular a line that reads GATE OK (m-3)
flat() { printf '%s' "$*" | tr '\r\n' '  '; }
fail() { printf 'FAIL %s\n' "$(flat "$*")"; rc=1; }
W=$(mktemp -d "${TMPDIR:-/tmp}/regate.XXXXXX"); trap 'rm -rf "$W"' EXIT
COPY="$W/copy.db"
if ! "$SQLITE3" -readonly "$DB" ".backup '$COPY'" >"$W/bk.out" 2>&1 || [ ! -s "$COPY" ]; then
  fail "cannot read the database (online backup failed): $(head -c 200 "$W/bk.out")"; echo "GATE FAILED"; exit 1
fi
q() { "$SQLITE3" -readonly "$COPY" "$@" 2>&1; }
sane() { printf '%s' "$1" | tr -cd '[:alnum:]_ .,;()*-' | cut -c1-60; }
sane_full() { printf '%s' "$1" | LC_ALL=C tr -cd '[:print:]' | cut -c1-600; }   # printable ASCII only (quotes, brackets and backticks kept so a quoting-only forgery stays visible, WF7-4), 600-char cap (WF6-4); control characters and newlines are still dropped

step_engine_validate() { "$WI" validate --db "$COPY" >"$W/val.out" 2>&1 || fail "engine validate: $(head -c 300 "$W/val.out")"; }
step_integrity()       { r=$(q 'PRAGMA integrity_check;'); [ "$r" = ok ] || fail "integrity_check: $(printf '%s' "$r" | head -c 300)"; }
step_fk()              { r=$(q 'PRAGMA foreign_key_check;'); [ -z "$r" ] || fail "foreign_key_check: $(printf '%s' "$r" | head -3 | tr '\n' ';')"; }
step_missing_objects() { m=$(q "SELECT group_concat(name) FROM v_gate_missing_objects;"); [ -z "$m" ] || fail "v_gate_missing_objects: $(sane "$m")"; }
# Untrusted-name rule: a name read from the database under test is DATA, never SQL. The view_empty list comes
# from the TRUSTED reference DB built from register_ext.sql; each name must match ^v_[a-z0-9_]+$, must be a
# VIEW in sqlite_master of the DB under test, and is quoted as an identifier. Registry rows of the DB under
# test are only cross-checked (printed sanitised, never executed).
qr() { "$SQLITE3" -readonly "$REF" "$@" 2>&1; }
step_view_empty() {
  [ -s "${REF:-}" ] || { fail "view_empty not run: no trusted reference database"; return; }
  views=$(qr "SELECT name FROM reg_gate_checks WHERE kind='view_empty'")  # LIST_SOURCE reference, not the DB under test
  [ -n "$views" ] || { fail "reference gate registry holds no view_empty row"; return; }
  checked=0
  for v in $views; do
    [[ $v =~ ^v_[a-z0-9_]+$ ]] || { fail "invalid view name in the reference registry: $(sane "$v")"; continue; }   # GUARD_REGEX
    k=$(q "SELECT count(*) FROM sqlite_master WHERE type='view' AND name='$v'")   # name already matched the regex
    [ "$k" = 1 ] || { fail "$v is not a view in the database under test"; continue; }   # GUARD_ISVIEW
    n=$(q "SELECT count(*) FROM \"$v\""); [ "$n" = 0 ] || fail "$v rows=$n"
    checked=$((checked+1))
  done
  echo "INFO view_empty checked=$checked"
  # cross-check the registry stored in the database under test (data only)
  while IFS= read -r row; do
    [[ $row =~ ^v_[a-z0-9_]+$ ]] || { fail "invalid registry name in the database under test: $(sane "$row")"; continue; }   # GUARD_DBROW_REGEX
    printf '%s\n' "$views" | grep -qx -- "$row" || fail "registry row not in the reference: $row"
  done < <(q "SELECT name FROM reg_gate_checks WHERE kind='view_empty'")
}
# One record per schema object, sql hex-encoded so a multi-line definition stays one line.
# WF5-1: nothing is hidden by name prefix. SQLite reserves sqlite_ (any case) only while writable_schema is off
# or the connection is defensive; a non-defensive connection creates triggers / views / indexes / tables with
# such names and they are active on every later connection. The ONLY objects left out of the comparison are
# the tables SQLite itself creates on demand and the reference never holds: sqlite_stat1..4 (ANALYZE). They
# are not hidden either: part 0 of step_reference_diff below pins their exact definition. sqlite_sequence and
# sqlite_autoindex_* are deterministic for the same DDL, so they stay in the comparison.
STAT_FILTER="NOT (type='table' AND name GLOB 'sqlite_stat[1-4]')"   # GLOB: case-sensitive, _ is literal (LIKE's _ is a wildcard, WF3 I1)
OBJ_ALL="SELECT type||'|'||name||'|'||tbl_name||'|'||ifnull(hex(sql),'') FROM sqlite_master WHERE $STAT_FILTER ORDER BY type,name"
STAT_ALL="SELECT name||'|'||tbl_name||'|'||ifnull(sql,'') FROM sqlite_master WHERE type='table' AND name GLOB 'sqlite_stat[1-4]' ORDER BY name"
SEEDS="SELECT 'T|'||from_status||'|'||to_status FROM reg_status_transitions ORDER BY 1;
       SELECT 'G|'||name||'|'||kind FROM reg_gate_checks ORDER BY 1;
       SELECT 'Y|'||type_code||'|'||label FROM reg_test_types ORDER BY 1;
       SELECT 'M|'||key||'|'||value FROM reg_meta ORDER BY 1;"
step_reference_diff() {
  REF="$W/ref.db"; ENG="$W/eng.db"
  "$WI" validate --db "$ENG" >/dev/null 2>&1 && cp "$ENG" "$REF" && "$SQLITE3" "$REF" < "$EXT" >/dev/null 2>&1 || { fail "reference build from $EXT failed"; return; }
  "$SQLITE3" -readonly "$COPY" "$OBJ_ALL" > "$W/a.all" 2>&1; "$SQLITE3" -readonly "$REF" "$OBJ_ALL" > "$W/b.all" 2>&1
  "$SQLITE3" -readonly "$ENG" "$OBJ_ALL" > "$W/e.all" 2>&1
  [ -s "$W/a.all" ] || { fail "schema unreadable in the database under test"; return; }
  # 0. SQLite's own statistics tables are the only objects not compared: each present one must carry exactly
  #    the definition SQLite writes (tbl_name = itself), anything else is a forged object using the name.
  #    SQLite writes the column lists in LOWERCASE (WF6-1: vendored 3.53.3 source has "tbl,idx,stat" and
  #    "tbl,idx,neq,nlt,ndlt,sample"); stat2 is the documented text; stat3 (legacy, not written by 3.53.3) follows
  #    stat4's list - UNCONFIRMED offline for pre-3.30 builds.
  while IFS= read -r r; do
    case "$r" in
      'sqlite_stat1|sqlite_stat1|CREATE TABLE sqlite_stat1(tbl,idx,stat)'|\
      'sqlite_stat2|sqlite_stat2|CREATE TABLE sqlite_stat2(tbl,idx,sampleno,sample)'|\
      'sqlite_stat3|sqlite_stat3|CREATE TABLE sqlite_stat3(tbl,idx,neq,nlt,ndlt,sample)'|\
      'sqlite_stat4|sqlite_stat4|CREATE TABLE sqlite_stat4(tbl,idx,neq,nlt,ndlt,sample)') ;;
      *) fail "unexpected schema object in the database under test, not in the reference: table|$(sane "${r%%|*}") (definition is not SQLite's own; tbl_name and sql, first 600 characters: $(sane_full "${r#*|}"))" ;;
    esac
  done < <("$SQLITE3" -readonly "$COPY" "$STAT_ALL" 2>&1)
  # 1. the set of objects: every object of the database under test must exist in the reference (and the other
  #    way round), whatever its name. An unexpected trigger / view / index / table is the way a guard is silenced.
  cut -d'|' -f1-3 "$W/a.all" | sort > "$W/a.names"; cut -d'|' -f1-3 "$W/b.all" | sort > "$W/b.names"
  while IFS= read -r o; do fail "unexpected schema object in the database under test, not in the reference: $(sane "$o")"; done < <(comm -23 "$W/a.names" "$W/b.names")
  while IFS= read -r o; do fail "schema object of the reference missing in the database under test: $(sane "$o")"; done < <(comm -13 "$W/a.names" "$W/b.names")
  # 2. definitions: every extension object, and every engine trigger and view, must equal the reference text
  cut -d'|' -f1-3 "$W/e.all" | sort > "$W/e.names"
  comm -13 "$W/e.names" "$W/b.names" | cut -d'|' -f2 | sort -u > "$W/ext.set"               # names created by the extension
  awk -F'|' 'NR==FNR{x[$1]=1;next} ($2 in x) || ($1=="trigger") || ($1=="view") {print}' "$W/ext.set" "$W/a.all" | sort > "$W/a.def"
  awk -F'|' 'NR==FNR{x[$1]=1;next} ($2 in x) || ($1=="trigger") || ($1=="view") {print}' "$W/ext.set" "$W/b.all" | sort > "$W/b.def"
  if ! diff "$W/a.def" "$W/b.def" >"$W/d.def" 2>&1; then
    fail "schema differs from $EXT ($(grep -c '^[<>]' "$W/d.def") differing definition lines; first: $(grep -m1 '^[<>]' "$W/d.def" | cut -d'|' -f1-3 | cut -c1-120))"; fi
  # 3. seed rows
  "$SQLITE3" -readonly "$COPY" "$SEEDS" > "$W/a.seed" 2>&1; "$SQLITE3" -readonly "$REF" "$SEEDS" > "$W/b.seed" 2>&1
  diff "$W/a.seed" "$W/b.seed" >"$W/d.seed" 2>&1 || fail "seed differs from $EXT ($(grep -c '^[<>]' "$W/d.seed") differing lines; first: $(grep -m1 '^[<>]' "$W/d.seed" | cut -c1-120))"
}
step_engine_version() {   # m-4: the engine schema the database holds is the one the extension requires
  have=$(q "SELECT value FROM meta WHERE key='schema_version'"); need=$(qr "SELECT value FROM reg_meta WHERE key='engine_schema_required'")
  [ -n "$need" ] && [ "$have" = "$need" ] || fail "engine schema_version [$(sane "$have")] != engine_schema_required [$(sane "$need")]"
}
# I-3 (docs/04 §5 limitation 3(a), §7.1(a); FR-008, SC-003): every cited evidence file must exist and re-hash to the
# sha256 and size recorded when it was captured. A row naming a file that is absent, or whose bytes changed,
# is no evidence. A path is DATA: never executed, passed after `--`, tab/newline paths refused. URL-shaped
# paths are not files and are only counted (honest gap, printed).
step_evidence_rehash() {
  bad=$(q "SELECT count(*) FROM reg_evidence WHERE instr(path,char(9))>0 OR instr(path,char(10))>0 OR instr(path,char(13))>0")
  [ "$bad" = 0 ] || fail "evidence_rehash: $bad evidence path(s) hold a tab or newline"
  n=0; skipped=0
  while IFS=$'\t' read -r id path sha size kind; do
    [[ $id =~ ^[0-9]+$ ]] || { fail "evidence_rehash: unreadable evidence row"; continue; }
    case "$path" in
      [A-Za-z]*://*)   # URL-shaped: not a file. Only a tracker receipt may be one (WF3 I2); counted, never verified
        if [ "$kind" = tracker_receipt ]; then skipped=$((skipped+1)); else fail "evidence $id: kind $(sane "$kind") must cite a file, URL-shaped path refused: $(sane "$path")"; fi
        continue;;
      /*) f=$path;; *) f="$ROOT/$path";; esac
    n=$((n+1))
    if [ ! -f "$f" ]; then fail "evidence $id: cited file absent: $(sane "$path")"; continue; fi
    h=$(sha256sum -- "$f" | cut -d' ' -f1); [ "$h" = "$sha" ] || { fail "evidence $id: sha256 mismatch for $(sane "$path") (recorded $(sane "${sha:0:12}"), file $(sane "${h:0:12}"))"; continue; }
    sz=$(stat -c %s -- "$f"); [ "$sz" = "$size" ] || fail "evidence $id: size mismatch for $(sane "$path") (recorded $(sane "$size"), file $sz)"
  done < <("$SQLITE3" -readonly -separator $'\t' "$COPY" "SELECT evidence_id,path,sha256,size_bytes,kind FROM reg_evidence WHERE instr(path,char(9))=0 AND instr(path,char(10))=0 AND instr(path,char(13))=0 ORDER BY evidence_id" 2>&1)
  echo "INFO evidence_rehash files=$n url_skipped=$skipped"
  URL_SKIPPED=$skipped
}
# T180 (docs/04 §12.3 step 3): every row of a view_not_done view is open work. Same untrusted-name rule as step_view_empty: the
# list comes from the TRUSTED reference, each name must match ^v_[a-z0-9_]+$ and be a VIEW of the database under test, and the
# registry of the database under test is only cross-checked. A count that is not a number (the view errors) is a FAIL, never zero.
ND_VIEWS=0; ND_ROWS=0
step_not_done() {
  [ -s "${REF:-}" ] || { fail "not_done not run: no trusted reference database"; return; }
  views=$(qr "SELECT name FROM reg_gate_checks WHERE kind='view_not_done' ORDER BY name")
  [ -n "$views" ] || { fail "reference gate registry holds no view_not_done row"; return; }
  checked=0
  for v in $views; do
    [[ $v =~ ^v_[a-z0-9_]+$ ]] || { fail "invalid view name in the reference registry: $(sane "$v")"; continue; }   # GUARD_ND_REGEX
    k=$(q "SELECT count(*) FROM sqlite_master WHERE type='view' AND name='$v'")
    [ "$k" = 1 ] || { fail "$v is not a view in the database under test"; continue; }   # GUARD_ND_ISVIEW
    n=$(q "SELECT count(*) FROM \"$v\"")
    [[ $n =~ ^[0-9]+$ ]] || { fail "$v not counted: $(sane "$n")"; continue; }   # GUARD_ND_NUMERIC
    checked=$((checked+1))
    if [ "$n" != 0 ]; then echo "NOT DONE $v rows=$n"; ND_VIEWS=$((ND_VIEWS+1)); ND_ROWS=$((ND_ROWS+n)); fi
  done
  echo "INFO not_done checked=$checked"
  while IFS= read -r row; do
    [[ $row =~ ^v_[a-z0-9_]+$ ]] || { fail "invalid registry name in the database under test: $(sane "$row")"; continue; }   # GUARD_ND_DBROW_REGEX
    printf '%s\n' "$views" | grep -qx -- "$row" || fail "registry row not in the reference: $row"
  done < <(q "SELECT name FROM reg_gate_checks WHERE kind='view_not_done'")
}

URL_SKIPPED=0
step_reference_diff   # first: builds the trusted reference and compares schema + seed before any query uses DB contents
step_engine_version   # before the engine's validate: validate migrates the copy, which would hide a stale schema_version
step_engine_validate
step_integrity
step_fk
step_missing_objects
step_view_empty
step_evidence_rehash
[ $COMPLETION = 1 ] && step_not_done
if [ $rc != 0 ]; then echo "GATE FAILED"; exit 1; fi
if [ $COMPLETION = 1 ] && [ "$ND_VIEWS" -gt 0 ]; then echo "COMPLETION NOT DONE views=$ND_VIEWS rows=$ND_ROWS"; exit 4; fi   # GUARD_ND_EXIT
PFX=GATE; [ $COMPLETION = 1 ] && PFX=COMPLETION
if [ "${URL_SKIPPED:-0}" -gt 0 ]; then   # a run that skipped evidence paths is never a bare GATE OK
  if [ $ALLOW_URL = 1 ]; then echo "$PFX PASS-PARTIAL url_skipped=$URL_SKIPPED (allowed by --allow-url-evidence; those receipts are NOT re-hashed)"; exit 0; fi
  echo "$PFX PASS-PARTIAL url_skipped=$URL_SKIPPED (refused: URL-shaped evidence is not re-hashed; pass --allow-url-evidence to accept the partial result)"; exit 3
fi
if [ $COMPLETION = 1 ]; then echo "COMPLETION OK"; exit 0; fi
echo "GATE OK"; exit 0
