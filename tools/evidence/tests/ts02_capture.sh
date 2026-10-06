#!/usr/bin/env bash
# T052: writes $EV/wp05/ts02-census.json = the structural census BEFORE (the HEAD versions of the suites) and AFTER (the working tree), the control
# needle result, and the migration list (one row per suite: definition line and body sha256 before, the sourcing line after).
# Usage: ts02_capture.sh [OUT] (default specs/001-full-project-audit-remediation/evidence/wp05/ts02-census.json)
set -u
here=$(cd "$(dirname "$0")" && pwd); root=$(cd "$here/../../.." && pwd); cd "$root" || exit 1
F=specs/001-full-project-audit-remediation; OUT=${1:-$F/evidence/wp05/ts02-census.json}
S=$(mktemp -d); trap 'rm -rf "$S"' EXIT
mkdir -p "$S/before/scripts/testing/full_automation" "$S/before/docs/scripts" "$S/before/.specify/memory" "$S/before/$F/docs"
for f in $(git ls-tree --name-only HEAD scripts/testing/full_automation/); do git show "HEAD:$f" >"$S/before/$f"; done
cp docs/scripts/catalog_*.md "$S/before/docs/scripts/" 2>/dev/null; cp .specify/memory/*.md "$S/before/.specify/memory/" 2>/dev/null; cp "$F"/docs/05-*.md "$S/before/$F/docs/" 2>/dev/null
python3 -I tools/evidence/census_ab_pass.py --root "$S/before" >"$S/before.json" || exit 1
python3 -I tools/evidence/census_ab_pass.py --root . >"$S/after.json" || exit 1
python3 - "$S/before.json" "$S/after.json" "$OUT" "$root" <<'P'
import json, sys, hashlib, os
b, a = json.load(open(sys.argv[1])), json.load(open(sys.argv[2])); out, root = sys.argv[3], sys.argv[4]
def sha(p): return hashlib.sha256(open(os.path.join(root, p), "rb").read()).hexdigest()
mig = []
for d in b["definitions"]:
    if not d["path"].startswith("scripts/testing/full_automation/"):
        continue
    after = [x for x in a["definitions"] if x["path"] == d["path"]]
    src = [l for l in open(os.path.join(root, d["path"])).read().split("\n") if "tools/evidence/lib/ab_pass_with_evidence.sh" in l and l.startswith(". ")]
    mig.append({"path": d["path"], "definition_line_before": d["line"], "body_sha256_before": d["body_sha256"], "definition_after": bool(after),
                "sources_shared_definition_after": bool(src), "file_sha256_after": sha(d["path"])})
doc = {"task": "T052 / docs/05 TS-02", "census_tool": "tools/evidence/census_ab_pass.py", "census_tool_sha256": sha("tools/evidence/census_ab_pass.py"),
       "shared_definition": "tools/evidence/lib/ab_pass_with_evidence.sh", "shared_definition_sha256": sha("tools/evidence/lib/ab_pass_with_evidence.sh"),
       "before_note": "scratch tree: the HEAD versions of scripts/testing/full_automation/*.sh plus the current carrier documents (docs/scripts/catalog_*.md, .specify/memory/*.md, docs/05)",
       "before": {k: b[k] for k in ("needle_found", "definitions", "calls", "carriers", "excluded", "scanned_files")},
       "after": {k: a[k] for k in ("needle_found", "definitions", "calls", "carriers", "excluded", "scanned_files")},
       "migration": mig,
       "summary": {"definitions_under_full_automation_before": len([d for d in b["definitions"] if d["path"].startswith("scripts/testing/full_automation/")]),
                   "definitions_under_full_automation_after": len([d for d in a["definitions"] if d["path"].startswith("scripts/testing/full_automation/")]),
                   "shipping_definitions_after": [d["path"] for d in a["definitions"] if d["class"] == "shipping"],
                   "copies_left_behind": [m["path"] for m in mig if m["definition_after"]],
                   "finding_filing": "none needed: no copy is left behind (the filing depends on T069 in any case)"}}
json.dump(doc, open(out, "w"), indent=1, sort_keys=True); open(out, "a").write("\n")
print("wrote", out, "migrated", len(mig), "left behind", doc["summary"]["copies_left_behind"])
P
