#!/usr/bin/env bash
# test_import.sh - T163 (WP-20): integration tests of the Stage 1/2 importer scripts/register/import.sh (T168), real SQLite,
# the real $WI engine binary, a scratch DB only (never docs/workable_items.db).
#   step 0  the engine binary is the one the T064 real leg recorded (`$EV/wp06/register-binaries.json`): `$WI --version`
#           exits 0 and the sha256 of $WI appears in that verdict; a failing run or a differing sha256 stops RED with
#           engine_binary_mismatch (both values recorded); a missing verdict is `engine_binary_verdict_missing`.
#   (a) idempotency   two importer runs change no row count and no table hash
#   (b) completeness  deleting one reg_source_map row makes v_unmapped_entries report exactly that entry
#   (c) legacy order  reg_item_ext custody_basis='legacy_import' before `add` is accepted, after `add` refused (doc04 9.3, 13.2, 14.8 L1)
#   (d) description cap  a ticket over 2048 bytes is imported with a description <= 2048 bytes (Sources block included), cut on a
#       UTF-8 boundary and marked as cut, the Sources block naming the source file and its sha256 (which matches the file); a
#       shorter ticket keeps its whole text
# Importer interface assumed here (UNCONFIRMED until T168 lands; the test is the specification):
#   import.sh --db <scratch db> --freeze-json <freeze.json> --engine <$WI>   exit 0 on success, reads the ticket text and its sha256
#   from the frozen snapshot named by --freeze-json, writes reg_ids/reg_item_ext/items/reg_source_map, prints nothing on stdout that
#   the test depends on.
# Env: EV (evidence root), TIC_IMAGE_DIGEST (digest of the TIC run record when run in IMG-TESTUTIL), IMPORT_SH, ENUMERATE_SOURCES.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
source "$(dirname "${BASH_SOURCE[0]}")/wp20_ident.sh"
EV=${EV:-$ROOT/specs/001-full-project-audit-remediation/evidence}
IMP=${IMPORT_SH:-$REG_DIR/import.sh}; ENUM=${ENUMERATE_SOURCES:-$REG_DIR/enumerate_sources.py}; FIX="$REG_DIR/tests/wp20_fixture.py"
ident_header_wp20 T163
# ---- step 0: engine binary identity
WIV=$("$WI" --version 2>&1 | head -1); wirc=${PIPESTATUS[0]}; WISHA=$(sha256sum "$WI" | cut -d' ' -f1)
# FINDING (measured 2026-10-07): the committed engine binary has no --version subcommand (exit 1, "unknown subcommand: --version" and its
# usage text). The task text asks for `$WI --version`; the probe that stands in for it is `$WI validate --db <fresh scratch db>` (the command
# T064's real leg runs), and the deviation is recorded here rather than silently passing.
case "$WIV" in "unknown subcommand: --version"*) WIVNOTE="--version unsupported by the binary (UNCONFIRMED vs the T163 text); probe=validate"; fresh_ext_db "$T_SCR/probe.db" 2>/dev/null; "$WI" validate --db "$T_SCR/probe.db" >/dev/null 2>&1; wirc=$?; WIV="validate rc=$wirc";; *) WIVNOTE="";; esac
echo "# engine probe: $WIV ${WIVNOTE:+($WIVNOTE)}"
VERD="$EV/wp06/register-binaries.json"
if [ ! -f "$VERD" ]; then bad "engine_binary_verdict_missing: $VERD is not recorded (T064 real leg), engine sha256=$WISHA version=[$WIV]"
elif [ $wirc -ne 0 ]; then bad "engine_binary_mismatch: '\$WI --version' exit $wirc [$WIV]"
elif ! grep -q "$WISHA" "$VERD"; then bad "engine_binary_mismatch: sha256 of \$WI=$WISHA not in the verdict $VERD"
else ok "engine binary sha256 $WISHA is the recorded verdict"; fi
if [ -n "${TIC_IMAGE_DIGEST:-}" ]; then
  grep -q "$TIC_IMAGE_DIGEST" "$VERD" 2>/dev/null && ok "TIC image digest matches the verdict" || bad "engine_binary_mismatch: image digest $TIC_IMAGE_DIGEST not in the verdict"
else echo "# TIC_IMAGE_DIGEST unset: image digest check UNCONFIRMED (host-side run)"; fi

# ---- (c) legacy order: independent of the importer (DDL + engine)
DBC="$T_SCR/c.db"; fresh_ext_db "$DBC" || { bad "(c) scratch db"; }
sql "$DBC" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('t','import')" >/dev/null; CX=$(sql "$DBC" "SELECT atm_id FROM reg_ids ORDER BY seq DESC LIMIT 1" | tail -1)
expect_ok "$DBC" "(c) legacy_import row written BEFORE add is accepted" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,legacy_status,reverify_required) VALUES ('$CX','bug','source','low','legacy_import','closed',1)"
"$WI" add Task Low --db "$DBC" --id "$CX" --prefix CAT --title "legacy order" --description "legacy order leg: an item added after its legacy extension row, long enough for the floor" >/dev/null 2>&1
sql "$DBC" "INSERT INTO reg_ids(minted_by,mint_basis) VALUES ('t','import')" >/dev/null; CY=$(sql "$DBC" "SELECT atm_id FROM reg_ids ORDER BY seq DESC LIMIT 1" | tail -1)
"$WI" add Task Low --db "$DBC" --id "$CY" --prefix CAT --title "legacy order 2" --description "legacy order leg: add first then the legacy row, long enough for the floor" >/dev/null 2>&1
expect_reject "$DBC" "(c) legacy_import row written AFTER add is refused" "before the id ever had an items row" "INSERT INTO reg_item_ext(atm_id,category,defect_layer,severity,custody_basis,legacy_status,reverify_required) VALUES ('$CY','bug','source','low','legacy_import','closed',1)"

# ---- importer legs
if [ ! -f "$IMP" ]; then bad "(a)(b)(d) importer absent: $IMP (RED: T168 not implemented)"; echo "RESULT: pass=$PASS fail=$FAIL"; exit 1; fi
TREE="$T_SCR/tree"; python3 "$FIX" build "$TREE"; python3 "$FIX" tickets "$TREE"; python3 "$FIX" freeze "$TREE" "$T_SCR/freeze.json"
python3 "$ENUM" --freeze-json "$T_SCR/freeze.json" --out "$T_SCR/enum" >/dev/null 2>&1 || bad "enumerator failed: $(python3 "$ENUM" --freeze-json "$T_SCR/freeze.json" --out "$T_SCR/enum" 2>&1 | head -c 200)"
DB="$T_SCR/imp.db"; fresh_ext_db "$DB"; "$SQLITE3" "$DB" ".read $T_SCR/enum/source_entries.sql" || bad "enumerator SQL did not apply"
run_imp() { "$IMP" --db "$DB" --freeze-json "$T_SCR/freeze.json" --engine "$WI" >"$T_SCR/imp.log" 2>&1; }
snap() { for t in reg_ids reg_item_ext items reg_source_map reg_status_log; do printf '%s=%s ' "$t" "$("$SQLITE3" "$DB" "SELECT count(*) FROM $t")"; done; "$SQLITE3" "$DB" .dump | grep -v '^PRAGMA' | sha256sum | cut -c1-64; }
run_imp; assert_eq "importer first run exits 0" "$?" "0"
S1=$(snap); run_imp; assert_eq "importer second run exits 0" "$?" "0"; S2=$(snap)
assert_eq "(a) idempotency: second run changes no row count and no hash" "$S2" "$S1"
UNM=$("$SQLITE3" "$DB" "SELECT count(*) FROM v_unmapped_entries"); echo "# unmapped after import (S-01 tickets are mapped, other classes may remain): $UNM"
TICKETS=$("$SQLITE3" "$DB" "SELECT count(*) FROM reg_source_entries e JOIN reg_sources s USING(source_id) WHERE s.kind='issue_file'")
MAPPED=$("$SQLITE3" "$DB" "SELECT count(*) FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) JOIN reg_sources s USING(source_id) WHERE s.kind='issue_file'")
assert_eq "every issue_file entry is mapped to an item" "$MAPPED" "$TICKETS"
# (b)
VICTIM=$("$SQLITE3" "$DB" "SELECT entry_id FROM reg_source_map ORDER BY entry_id LIMIT 1"); BEFORE=$("$SQLITE3" "$DB" "SELECT group_concat(entry_id) FROM v_unmapped_entries")
"$SQLITE3" "$DB" "DROP TRIGGER reg_source_map_no_delete; DELETE FROM reg_source_map WHERE entry_id=$VICTIM;" || bad "(b) could not delete the map row in the scratch DB"
AFTER=$("$SQLITE3" "$DB" "SELECT group_concat(entry_id) FROM v_unmapped_entries ORDER BY 1")
python3 - "$BEFORE" "$AFTER" "$VICTIM" <<'PY' && ok "(b) completeness: v_unmapped_entries grew by exactly entry $VICTIM" || bad "(b) completeness: before=[$BEFORE] after=[$AFTER] victim=$VICTIM"
import sys
b = set(x for x in sys.argv[1].split(",") if x); a = set(x for x in sys.argv[2].split(",") if x)
sys.exit(0 if a - b == {sys.argv[3]} and b <= a else 1)
PY
# (d) description cap, on a fresh import (the victim row above was only for leg b)
DB="$T_SCR/imp2.db"; fresh_ext_db "$DB"; "$SQLITE3" "$DB" ".read $T_SCR/enum/source_entries.sql"; run_imp
python3 - "$DB" "$TREE" <<'PY'
import hashlib, re, sqlite3, sys
db, tree = sys.argv[1:3]
c = sqlite3.connect(db)
def item_for(path):
    r = c.execute("SELECT i.description FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) JOIN items i ON i.atm_id=m.atm_id WHERE e.locator=?", (path,)).fetchone()
    return r[0] if r else None
fail = 0
def check(label, cond, info=""):
    global fail
    print(("ok   " if cond else "FAIL ") + label + ("" if cond else " :: " + info)); fail += 0 if cond else 1
long_p, short_p = "docs/issues/HELIX-950-long.md", "docs/issues/HELIX-951-short.md"
d = item_for(long_p)
check("(d) long ticket has an item", d is not None)
if d is not None:
    raw = c.execute("SELECT CAST(i.description AS BLOB) FROM reg_source_map m JOIN reg_source_entries e USING(entry_id) JOIN items i ON i.atm_id=m.atm_id WHERE e.locator=?", (long_p,)).fetchone()[0]
    check("(d) long description is <= 2048 bytes, Sources block included (%d bytes)" % len(raw), len(raw) <= 2048, str(len(raw)))
    try: raw.decode("utf-8"); strict_ok = True        # the STORED BYTES decoded strictly: a cut inside a two-byte character is invalid UTF-8
    except UnicodeDecodeError: strict_ok = False
    check("(d) cut on a UTF-8 character boundary: the stored bytes decode strictly and hold no U+FFFD replacement character", strict_ok and "\ufffd" not in raw.decode("utf-8", "replace"))
    body = open(tree + "/" + long_p, "rb").read().decode("utf-8").split("\n\n# Long ticket\n\n", 1)[1]
    run = re.search("\u00e9+", d)
    check("(d) the stored text is a PREFIX of the ticket body (%d of %d characters kept)" % (len(run.group(0)) if run else 0, len(body.strip())), bool(run) and body.startswith(run.group(0)) and len(run.group(0)) < len(body.strip()))
    check("(d) marked as cut", re.search(r"(?i)\bcut\b|truncat", d) is not None)
    sha = hashlib.sha256(open(tree + "/" + long_p, "rb").read()).hexdigest()
    check("(d) Sources block names the source file and its sha256", long_p in d and sha in d, sha)
s = item_for(short_p)
check("(d) short ticket has an item", s is not None)
if s is not None:
    body = "Short body text that stays whole."
    check("(d) short ticket keeps its whole text", body in s and not re.search(r"(?i)\bcut\b|truncat", s.replace(body, "")), s[:120])
    sha = hashlib.sha256(open(tree + "/" + short_p, "rb").read()).hexdigest()
    check("(d) short ticket Sources block names file and sha256", short_p in s and sha in s)
sys.exit(1 if fail else 0)
PY
[ $? -eq 0 ] || FAIL=$((FAIL+1))
echo "RESULT: pass=$PASS fail=$FAIL"
[ "$FAIL" -eq 0 ]
