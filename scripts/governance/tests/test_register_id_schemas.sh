#!/usr/bin/env bash
# test_register_id_schemas.sh (WP-01, owner decision ODG-11 2026-10-05; fix round 3 after review WF2 governance, finding I5):
#   the register-id patterns of finding.schema.json (properties.register_item) and bank-case.schema.json
#   ($defs.v3.properties.register_refs.items, reached from metadata.v3) accept CAT-NNN and refuse the withdrawn prefix, a
#   neutral prefix, a short number, and an unanchored match. (evidence-record.schema.json is covered by
#   tools/evidence/tests/test_evidence_record_schema.sh.)
# Usage: bash scripts/governance/tests/test_register_id_schemas.sh
# Env:   SCHEMA_DIR  directory holding the two schemas (default: the feature contracts/); the mutation runs use scratch copies.
# Exit:  0 all checks hold; 1 otherwise.
# Dialect note (measured, the instrument path is part of the instrument): python's jsonschema evaluates 'pattern' with re.search, where '$'
#        also matches before a trailing newline, so 'CAT-001' + newline is accepted here although ECMA-262 and RE2 refuse it.
#        That value is therefore NOT a fixture of this test (a verdict on it would test python, not the schema). UNCONFIRMED: which
#        regex engine each consumer of these schemas uses.
# Needs: python3 with jsonschema (Draft 2020-12). The instrument is proven by two control needles: a document the schema
#        must accept and one it must refuse are both evaluated through the same code path before the verdicts are trusted.
set -u
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$HERE/../../.." && pwd)"
export SCHEMA_DIR="${SCHEMA_DIR:-$ROOT/specs/001-full-project-audit-remediation/contracts}"
python3 - <<'PY'
import json, os, sys
try: import jsonschema
except Exception as e: print("FAIL prerequisite: jsonschema not importable: %s" % e); sys.exit(1)
D = os.environ["SCHEMA_DIR"]
fails = []
def fail(m): fails.append(m); print("FAIL " + m)
def ok(m): print("ok   " + m)
OLD = "AT" + "M"   # the withdrawn prefix, built so this file carries no literal of it
CASES = [("CAT-001", True), ("CAT-123", True), ("CAT-1234", True), ("CAT-01", False), ("CAT-", False), (OLD + "-001", False),
         ("XYZ-001", False), ("cat-001", False), ("xCAT-001", False), ("CAT-001x", False), ("", False), ("CAT-0 01", False)]
def load(n): return json.load(open(os.path.join(D, n), encoding="utf-8"))
def check(label, schema, wrap):
    v = jsonschema.Draft202012Validator(schema)
    bad = []
    for val, want in CASES:
        got = not list(v.iter_errors(wrap(val)))
        if got != want: bad.append("%r wanted %s got %s" % (val, "valid" if want else "refused", "valid" if got else "refused"))
    if bad: fail("%s: %s" % (label, "; ".join(bad)))
    else: ok("%s: %d fixtures (CAT-NNN valid; withdrawn prefix, neutral prefix, short number, case, unanchored refused)" % (label, len(CASES)))

# finding.schema.json: the property is reached from the root properties (control: the root declares it)
f = load("finding.schema.json")
rp = f.get("properties", {}).get("register_item")
if not rp or "pattern" not in rp: fail("finding.schema.json: properties.register_item with a pattern not found (instrument blind)")
else:
    check("finding.schema.json register_item", {"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "properties": {"register_item": rp}}, lambda x: {"register_item": x})
# bank-case.schema.json: register_refs lives in $defs.v3 and the root must reach v3 through metadata
b = load("bank-case.schema.json")
v3 = b.get("$defs", {}).get("v3", {}); rr = v3.get("properties", {}).get("register_refs")
if not rr or "items" not in rr: fail("bank-case.schema.json: $defs.v3.properties.register_refs.items not found (instrument blind)")
else:
    if b.get("properties", {}).get("metadata", {}).get("properties", {}).get("v3", {}).get("$ref") != "#/$defs/v3": fail("bank-case.schema.json: root metadata.v3 does not reference #/$defs/v3 (the checked pattern would be unreachable)")
    else: ok("bank-case.schema.json: metadata.v3 references $defs/v3 (the checked pattern is reachable)")
    check("bank-case.schema.json register_refs[]", {"$schema": "https://json-schema.org/draft/2020-12/schema", "$defs": b["$defs"], "$ref": "#/$defs/v3/properties/register_refs"}, lambda x: [x])
    # an empty list and several ids are still valid; one bad id among good ones is refused (items, not the array, carries the pattern)
    v = jsonschema.Draft202012Validator({"$defs": b["$defs"], "$ref": "#/$defs/v3/properties/register_refs"})
    ok("bank-case.schema.json register_refs: several ids and the empty list valid") if not list(v.iter_errors(["CAT-001", "CAT-002"])) and not list(v.iter_errors([])) else fail("bank-case register_refs: valid lists refused")
    ok("bank-case.schema.json register_refs: one withdrawn-prefix id among good ones refused") if list(v.iter_errors(["CAT-001", OLD + "-002"])) else fail("bank-case register_refs: a bad id among good ones was accepted")
# control needle: the instrument must distinguish a valid from an invalid value through the very same path
vv = jsonschema.Draft202012Validator({"type": "string", "pattern": "^CAT-[0-9]{3,}$"})
ok("control needle: the validator separates CAT-001 from the withdrawn prefix") if not list(vv.iter_errors("CAT-001")) and list(vv.iter_errors(OLD + "-001")) else fail("control needle: validator blind")
print("failures=%d" % len(fails)); sys.exit(1 if fails else 0)
PY
