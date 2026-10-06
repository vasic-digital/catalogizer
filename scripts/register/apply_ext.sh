#!/usr/bin/env bash
# apply_ext.sh - apply scripts/register/register_ext.sql (docs/04 §5 + the v5 additions owed to docs/04, ext_schema_version=5) to a register DB.
# Usage: apply_ext.sh --db <db> [--sql <file>]
# 1. when <db> holds no engine schema it is bootstrapped through `$WI validate --db` (creates the engine schema);
# 2. meta.schema_version must equal the engine version the DDL requires (reg_meta engine_schema_required, 7):
#    a different version is REFUSED (exit 3) naming both versions, nothing is applied;
# 3. the DDL is applied in ONE transaction (all or nothing; the file is idempotent, IF NOT EXISTS throughout).
# Never run against docs/workable_items.db before the T068 G-DATA GO: the real DB is refused unless
# REGISTER_GO_LIVE=1 (set only by the committed go-live step, T069).
# Env: WI (engine binary), SQLITE3 (sqlite3 binary; the container image's one once RUNP exists, T008),
#      REG_REAL_DB (the real register path the guard protects, default docs/workable_items.db; tests only).
# Exit: 0 ok, 2 usage, 3 version mismatch, 4 refused path, 5 bootstrap/apply failure.
set -u
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
WI=${WI:-$ROOT/submodules/constitution/scripts/workable-items/bin/workable-items-linux}
SQLITE3=${SQLITE3:-sqlite3}
DB=""; SQL=${REG_EXT_SQL:-$ROOT/scripts/register/register_ext.sql}   # REG_EXT_SQL: mutation runs (T062)
while [ $# -gt 0 ]; do case "$1" in
  --db|--sql) [ $# -ge 2 ] || { echo "apply_ext: $1 needs a value" >&2; exit 2; }   # a missing value must not loop (WF2 m-1)
              if [ "$1" = --db ]; then DB=$2; else SQL=$2; fi; shift 2;;
  *) echo "apply_ext: unknown argument: $1" >&2; exit 2;; esac; done
[ -n "$DB" ] || { echo "usage: apply_ext.sh --db <db> [--sql <file>]" >&2; exit 2; }
[ -r "$SQL" ] || { echo "apply_ext: DDL file not readable: $SQL" >&2; exit 2; }
# the DDL path is interpolated into a sqlite3 dot-command: refuse any character that could end the argument
case "$SQL" in *'"'*|*"'"*|*\\*|*$'\n'*|*$'\r'*) echo "apply_ext: DDL path holds a quote, backslash or newline: refused" >&2; exit 2;; esac
# real-register guard (WF2 m-6, WF3 m3): SQLite decodes %XX in a URI filename and honours ?query parameters, so a
# guard that compares the raw string cannot be correct; EVERY file: URI is refused (exit 4). A symlink and a
# hardlink of the real file are the same register and are refused by path identity.
REAL=${REG_REAL_DB:-$ROOT/docs/workable_items.db}
case "$DB" in [Ff][Ii][Ll][Ee]:*) echo "apply_ext: a file: URI is refused (SQLite decodes %XX and query parameters, the real-register guard cannot compare it); give a plain path" >&2; exit 4;; esac
if { [ "$(realpath -m "$DB")" = "$(realpath -m "$REAL")" ] || { [ -e "$DB" ] && [ -e "$REAL" ] && [ "$DB" -ef "$REAL" ]; }; } && [ "${REGISTER_GO_LIVE:-}" != 1 ]; then
  echo "apply_ext: refusing the real register $REAL before the G-DATA GO (T068/T069); use a scratch DB" >&2; exit 4
fi
need=$(sed -n "s/.*'engine_schema_required','\([0-9][0-9]*\)'.*/\1/p" "$SQL" | head -1)
[ -n "$need" ] || { echo "apply_ext: cannot read engine_schema_required from $SQL" >&2; exit 5; }
have_meta() { "$SQLITE3" "$DB" "select 1 from sqlite_master where name='meta' and type='table'" 2>/dev/null | grep -q 1; }
if [ ! -s "$DB" ] || ! have_meta; then
  "$WI" validate --db "$DB" >/dev/null 2>&1 || { echo "apply_ext: engine bootstrap failed for $DB" >&2; exit 5; }
fi
have=$("$SQLITE3" "$DB" "select value from meta where key='schema_version'" 2>/dev/null | head -1)
if [ "$have" != "$need" ]; then
  echo "apply_ext: engine schema_version mismatch: database has [${have:-none}], the extension requires [$need]; nothing applied" >&2; exit 3
fi
ERR=$(mktemp "${TMPDIR:-/tmp}/apply_ext.XXXXXX") || exit 5; trap 'rm -f "$ERR"' EXIT
# PRAGMA foreign_keys is a no-op inside a transaction, so it is set BEFORE BEGIN (WF2 m-6)
printf 'PRAGMA foreign_keys=ON;\nBEGIN;\n.read "%s"\nCOMMIT;\n' "$SQL" | "$SQLITE3" -bail "$DB" >/dev/null 2>"$ERR"
rc=$?
if [ $rc -ne 0 ]; then echo "apply_ext: applying $SQL failed (rolled back): $(cat "$ERR")" >&2; exit 5; fi
echo "apply_ext: applied ext_schema_version=$("$SQLITE3" "$DB" "select value from reg_meta where key='ext_schema_version'") to $DB (engine schema_version $have)"
