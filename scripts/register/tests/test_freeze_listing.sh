#!/usr/bin/env bash
# test_freeze_listing.sh - W2 (T161 stand-in check used by T166 and T174): scripts/register/check_freeze_listing.sh.
#   * the T161 fixture: a scratch listing with one NUL-terminated path appended is refused `freeze_listing_moved`
#   * a recorded, untouched listing passes (negative control of the refusal: a check that refuses everything is no check)
#   * a truncated listing, a reordered listing, a listing whose last record is not NUL-terminated, an empty listing, a duplicate path,
#     freeze.json without the hash field, a missing freeze.json
#   * the two-argument form lead_scan.py calls: path-set equality with the manifest (added / missing path refused, equal passes)
# Env: CHECK_FREEZE_LISTING (script under test; mutation runs point it at a mutated copy). Real files only, scratch directory.
# Exit 0 only when every check passed.
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"
source "$(dirname "${BASH_SOURCE[0]}")/wp20_ident.sh"
CHK=${CHECK_FREEZE_LISTING:-$REG_DIR/check_freeze_listing.sh}
FIX="$REG_DIR/tests/w2_fixture.py"
ident_header_wp20 W2-freeze-listing
echo "# check=$CHK sha256=$(sha256sum "$CHK" 2>/dev/null | cut -c1-64)"
if [ ! -f "$CHK" ]; then bad "check_freeze_listing.sh absent: $CHK (RED: the listing check is not implemented)"; echo "RESULT: pass=$PASS fail=$FAIL"; exit 1; fi
run() { bash "$CHK" "$@" >"$T_SCR/o.txt" 2>"$T_SCR/e.txt"; echo $?; }
T="$T_SCR/tree"; python3 "$(dirname "$FIX")/wp20_fixture.py" build "$T"; python3 "$FIX" freeze2 "$T" "$T_SCR/f.json"
LST=$(python3 -c "import json,sys;print(json.load(open(sys.argv[1]))['listing'])" "$T_SCR/f.json")
assert_eq "recorded untouched listing passes" "$(run "$T_SCR/f.json")" "0"
# the T161 fixture: one NUL-terminated path appended
cp "$LST" "$LST.orig"; printf 'sneaked/in.md\0' >>"$LST"
assert_eq "appended NUL-terminated path: refused with exit 20" "$(run "$T_SCR/f.json")" "20"
assert_eq "appended path: reason freeze_listing_moved" "$(grep -c 'REFUSED reason=freeze_listing_moved' "$T_SCR/e.txt")" "1"
cp "$LST.orig" "$LST"; assert_eq "restored listing passes again" "$(run "$T_SCR/f.json")" "0"
# truncated by one record
python3 - "$LST" <<'PY'
import sys
d = open(sys.argv[1], "rb").read().split(b"\0")[:-2]
open(sys.argv[1], "wb").write(b"\0".join(d) + b"\0")
PY
assert_eq "one record dropped: refused freeze_listing_moved" "$(run "$T_SCR/f.json") $(grep -c 'reason=freeze_listing_moved' "$T_SCR/e.txt")" "20 1"
cp "$LST.orig" "$LST"
# reordered (same set, different bytes)
python3 - "$LST" <<'PY'
import sys
d = open(sys.argv[1], "rb").read().split(b"\0")[:-1]
d[0], d[1] = d[1], d[0]
open(sys.argv[1], "wb").write(b"\0".join(d) + b"\0")
PY
assert_eq "two records swapped (same path set): refused freeze_listing_moved" "$(run "$T_SCR/f.json") $(grep -c 'reason=freeze_listing_moved' "$T_SCR/e.txt")" "20 1"
cp "$LST.orig" "$LST"
# not NUL-terminated at the end (newline separated, the failure the NUL rule exists for)
tr '\0' '\n' <"$LST.orig" >"$LST"
assert_eq "newline-separated listing: refused freeze_listing_invalid" "$(run "$T_SCR/f.json") $(grep -c 'reason=freeze_listing_invalid' "$T_SCR/e.txt")" "20 1"
cp "$LST.orig" "$LST"
: >"$LST"; assert_eq "empty listing: refused freeze_listing_invalid, the detail says it is empty" "$(run "$T_SCR/f.json") $(grep -c 'reason=freeze_listing_invalid.* is empty' "$T_SCR/e.txt")" "20 1"
cp "$LST.orig" "$LST"
printf 'a.md\0a.md\0' >"$LST"; python3 - "$T_SCR/f.json" "$LST" <<'PY'
import hashlib, json, sys
fj = json.load(open(sys.argv[1])); fj["listing_sha256"] = hashlib.sha256(open(sys.argv[2], "rb").read()).hexdigest()
json.dump(fj, open(sys.argv[1] + ".dup", "w"))
PY
assert_eq "duplicate path (hash recorded for it): refused freeze_listing_invalid" "$(run "$T_SCR/f.json.dup") $(grep -c 'reason=freeze_listing_invalid' "$T_SCR/e.txt")" "20 1"
cp "$LST.orig" "$LST"
printf 'a.md\0\0b.md\0' >"$LST"; python3 - "$T_SCR/f.json" "$LST" <<'PY'
import hashlib, json, sys
fj = json.load(open(sys.argv[1])); fj["listing_sha256"] = hashlib.sha256(open(sys.argv[2], "rb").read()).hexdigest()
json.dump(fj, open(sys.argv[1] + ".emptyrec", "w"))
PY
assert_eq "an empty path record (hash recorded for it): refused freeze_listing_invalid naming it" "$(run "$T_SCR/f.json.emptyrec") $(grep -c 'reason=freeze_listing_invalid.*empty path record' "$T_SCR/e.txt")" "20 1"
cp "$LST.orig" "$LST"
# freeze.json without the hash / without the listing / missing
python3 - "$T_SCR/f.json" <<'PY'
import json, sys
fj = json.load(open(sys.argv[1])); del fj["listing_sha256"]; json.dump(fj, open(sys.argv[1] + ".nohash", "w"))
fj2 = json.load(open(sys.argv[1])); del fj2["listing"]; json.dump(fj2, open(sys.argv[1] + ".nolist", "w"))
PY
assert_eq "freeze.json without listing_sha256: refused freeze_listing_unrecorded" "$(run "$T_SCR/f.json.nohash") $(grep -c 'reason=freeze_listing_unrecorded' "$T_SCR/e.txt")" "20 1"
assert_eq "freeze.json without listing: refused freeze_listing_unrecorded" "$(run "$T_SCR/f.json.nolist") $(grep -c 'reason=freeze_listing_unrecorded' "$T_SCR/e.txt")" "20 1"
assert_eq "no argument: usage exit 2" "$(run)" "2"
assert_eq "a freeze.json that does not exist: usage exit 2" "$(run "$T_SCR/none.json")" "2"
# the two-argument form (lead_scan.py): path-set equality with the manifest
MF=$(python3 -c "import json,sys;print(json.load(open(sys.argv[1]))['manifest'])" "$T_SCR/f.json")
assert_eq "two-argument form: listing path set equals the manifest path set" "$(run "$LST" "$MF")" "0"
printf 'extra/file.md\0' >>"$LST"
assert_eq "two-argument form: an added path is refused freeze_listing_moved naming it" "$(run "$LST" "$MF") $(grep -c 'added extra/file.md' "$T_SCR/e.txt")" "20 1"
cp "$LST.orig" "$LST"
python3 - "$LST" <<'PY'
import sys
d = open(sys.argv[1], "rb").read().split(b"\0")[:-2]
open(sys.argv[1], "wb").write(b"\0".join(d) + b"\0")
PY
assert_eq "two-argument form: a missing path is refused freeze_listing_moved" "$(run "$LST" "$MF") $(grep -c 'REFUSED reason=freeze_listing_moved.*missing ' "$T_SCR/e.txt")" "20 1"
cp "$LST.orig" "$LST"
echo "RESULT: pass=$PASS fail=$FAIL"
[ "$FAIL" -eq 0 ]
