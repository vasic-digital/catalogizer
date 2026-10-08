#!/usr/bin/env python3
"""mutate_w2.py - mutation sets of the W2 register tools (T166 recount, T174 root inventory, T170/T171 plan seeds, T172 ticket families, freeze listing).
Writes one mutant tree per id under <out>/<ID>/ (a byte copy of the tool's files with ONE edit of its target file) and prints the id list. Every edit must
match exactly once (a stale anchor is an error, never a silent no-op). The mutants are written by the task author (docs/04 13.2); N00 is the unmutated tree and
MUST pass its suite, C00 is a comment-only change and MUST survive (it proves the harness does not call every change a kill). A mutant that survives for a
reason the suite cannot see (an equivalent mutant) is listed with its reason in equivalent_w2_mutants.tsv, never silently skipped.

Usage: mutate_w2.py <tool> <out dir> [ID...]     mutate_w2.py --list <tool>     mutate_w2.py --tools     mutate_w2.py --field <tool> suite|item|sources
       mutate_w2.py --env <tool> <ID> <mutant dir>   -> the environment assignments that point the suite at the mutant (one NAME=VALUE per line)
"""
import os
import shutil
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
REG = os.path.dirname(HERE)

COMMON = ("check_freeze_listing.sh", "check_freeze_snapshot.sh", "snapshot_manifest.py")

# tool -> {files copied into every mutant dir (relative to scripts/register), target (the file that is edited), env (suite env var -> mutant file), suite}
TOOLS = {
    "freeze_listing": {"files": ["check_freeze_listing.sh"], "target": "check_freeze_listing.sh", "env": {"CHECK_FREEZE_LISTING": "check_freeze_listing.sh"},
                       "suite": "bash scripts/register/tests/test_freeze_listing.sh", "item": "AUD-W2-LISTING",
                       "sources": ["scripts/register/tests/test_freeze_listing.sh", "scripts/register/tests/w2_fixture.py", "scripts/register/tests/wp20_fixture.py"]},
    "recount": {"files": ["recount_sources.sh", "recount_sources.py"] + list(COMMON), "target": "recount_sources.py", "env": {"RECOUNT_SH": "recount_sources.sh"},
                "suite": "python3 -I scripts/register/tests/test_recount.py", "item": "AUD-T166",
                "sources": ["scripts/register/tests/test_recount.py", "scripts/register/tests/w2_fixture.py", "scripts/register/tests/wp20_fixture.py"]},
    "root_inventory": {"files": ["root_inventory.py", "root_dispositions.yaml", "check_freeze_listing.sh"], "target": "root_inventory.py", "env": {"ROOT_INVENTORY": "root_inventory.py"},
                       "suite": "python3 -I scripts/register/tests/test_root_inventory.py", "item": "AUD-T174",
                       "sources": ["scripts/register/tests/test_root_inventory.py", "scripts/register/tests/w2_fixture.py", "scripts/register/tests/wp20_fixture.py"]},
}

M = {}   # tool -> [(id, what, anchor, replacement)]
M["freeze_listing"] = [
    ("N00", "NEGATIVE CONTROL: the unmutated tree must pass", None, None),
    ("C00", "CONTROL: a comment-only change must SURVIVE", "# Refusals (stderr", "# (comment only) Refusals (stderr"),
    ("M01", "skips the listing hash comparison", "    if h != fj[\"listing_sha256\"]:                      # MUT:listing-hash", "    if False:"),
    ("M02", "accepts a listing that is not NUL-terminated", "    if not data.endswith(b\"\\0\"):", "    if False:"),
    ("M03", "accepts an empty listing", "    if not data:", "    if False:"),
    ("M04", "accepts a duplicate path", "    if len(set(recs)) != len(recs):", "    if False:"),
    ("M05", "accepts a freeze.json without listing_sha256", "not isinstance(fj.get(\"listing_sha256\"), str)", "False"),
    ("M06", "two-argument form ignores added paths", "sorted(\"added \" + p for p in listed - want) + ", ""),
    ("M07", "two-argument form ignores missing paths", " + sorted(\"missing \" + p for p in want - listed)", ""),
    ("M08", "accepts an empty path record", "    if any(r == b\"\" for r in recs):", "    if False:"),
    ("M09", "hash taken over the sorted record set, not the file bytes (a reordering passes)", "    h = hashlib.sha256(data).hexdigest()", "    h = hashlib.sha256(b\"\\0\".join(sorted(_recs)) + b\"\\0\").hexdigest()"),
    ("M10", "refusal exit code 0 instead of 20", "    sys.exit(20)", "    sys.exit(0)"),
]
M["recount"] = [
    ("N00", "NEGATIVE CONTROL: the unmutated tree must pass", None, None),
    ("C00", "CONTROL: a comment-only change must SURVIVE", "ENV = dict(os.environ, LC_ALL=\"C\")", "ENV = dict(os.environ, LC_ALL=\"C\")  # comment only"),
    ("M01", "paths passed to grep without `--` (a file named -n is read as an option)", "+ list(gargs) + [\"--\"]    # MUT:dashdash", "+ list(gargs)"),
    ("M02", "grep without -Z (file and count split on a colon)", "\"-a\", \"-H\", \"-Z\"]", "\"-a\", \"-H\"]"),
    ("M03", "skips the listing check", "[\"bash\", lchk, a.freeze_json]", "[\"bash\", \"-c\", \"true\"]"),
    ("M04", "skips the snapshot re-hash", "[\"bash\", schk, snapdir, manifest]", "[\"bash\", \"-c\", \"true\"]"),
    ("M05", "checkbox pattern loses its trailing whitespace requirement", "CHECKBOX = r\"^[[:space:]]*[-*+][[:space:]]+\\[( |x|X)\\][[:space:]]+\"", "CHECKBOX = r\"^[[:space:]]*[-*+][[:space:]]+\\[( |x|X)\\]\""),
    ("M06", "S-12 forgets that S-25 claimed SECURITY_AUDIT_REPORT.md", "\"SECURITY_KEY_ROTATION_REQUIRED.md\", \"SECURITY_AUDIT_REPORT.md\")", "\"SECURITY_KEY_ROTATION_REQUIRED.md\")"),
    ("M07", "ids home: S-15 checked before S-14", "for cls, fs in ((\"S-14\", s14), (\"S-15\", s15)):", "for cls, fs in ((\"S-15\", s15), (\"S-14\", s14)):"),
    ("M08", "any explanation excuses any difference", "rec[\"status\"] = \"match\" if (not diffs and total == sc) else (\"explained\" if loc in explanations else \"diff\")",
     "rec[\"status\"] = \"match\" if (not diffs and total == sc) else (\"explained\" if explanations else \"diff\")"),
    ("M09", "exit code 0 when a source differs", "    return 1 if failing else 0", "    return 0"),
    ("M10", "only a recount BELOW the register is a difference", "if by.get(k, 0) != db_by.get(k, 0):", "if by.get(k, 0) < db_by.get(k, 0):"),
    ("M11", "key mapping stops at `:` only (a `#` case locator is not attributed to its file)", "(i == len(locator) or locator[i] in \":#\")", "(i == len(locator) or locator[i] == \":\")"),
    ("M12", "provider table: separator rows are counted", "if ($0 !~ /^\\\\|[[:space:]:|-]+\\\\|?[[:space:]]*$/) n++;", "n++;"),
    ("M13", "remotes are not counted", "return {\"remotes\": len(pop.fj.get(\"remotes\") or [])}, True", "return {\"remotes\": 0}, True"),
    ("M14", "one named service too few", "return {\"services\": len(SERVICES)}, True", "return {\"services\": len(SERVICES) - 1}, True"),
    ("M15", "marker pattern without word boundaries (TODOS counts)", "[\"-I\", \"-w\", \"-E\", \"-e\", \"TODO|FIXME|HACK|XXX\"]", "[\"-I\", \"-E\", \"-e\", \"TODO|FIXME|HACK|XXX\"]"),
    ("M16", "markers count binary files", "[\"-I\", \"-w\", \"-E\", \"-e\", \"TODO|FIXME|HACK|XXX\"]", "[\"-w\", \"-E\", \"-e\", \"TODO|FIXME|HACK|XXX\"]"),
    ("M17", "the Go skip pattern is dropped", "SKIP_PCRE = (r\"\\b(?:t|b|tb)\\.(?:Skip|SkipNow|Skipf)\\(\",", "SKIP_PCRE = (r\"\\bnever-matches\\(\","),
    ("M18", "the Kotlin @Ignore pattern is dropped", "r\"@Ignore\\b\", r\"#\\[ignore\\b\"", "r\"#\\[ignore\\b\""),
    ("M19", "docs/*AUDIT*.md matches nested documents", "or re.match(r\"^docs/[^/]*AUDIT[^/]*\\.md$\", p))", "or re.match(r\"^docs/.*AUDIT.*\\.md$\", p))"),
    ("M26", "the S-14 audits method matches nested documents", "\"docs/*AUDIT*.md\": m_s14_audits(r\"^docs/[^/]*AUDIT[^/]*\\.md$\"),", "\"docs/*AUDIT*.md\": m_s14_audits(r\"^docs/.*AUDIT.*\\.md$\"),"),
    ("M20", "the specs/ exclusion of the id census is dropped", "ID_EXCLUDE = (\"specs/\", \".audit/\", \"submodules/\", \"scripts/register/tests/\")", "ID_EXCLUDE = (\".audit/\", \"submodules/\", \"scripts/register/tests/\")"),
    ("M21", "bank cases: the DEEPEST indentation counts (nested step ids)", "if (min==0 || m<min) min=m;", "if (min==0 || m>min) min=m;"),
    ("M22", "doc03 delta always reported equal", "rec[\"status\"] = \"equal\" if measured == val else \"delta\"", "rec[\"status\"] = \"equal\""),
    ("M23", "scanned_entry_count vs rows is not itemised", "            if sc != len(rows):", "            if False:"),
    ("M24", "gosec records counted by line (a one-line JSON file reads 1)", "n = len(xargs_grep(pop, [p], [\"-E\", \"-o\", \"-e\", r'\"rule_id\"'], count=False))", "n = gcount(pop, [p], r'\"rule_id\"').get(p, 0)"),
    ("M25", "a source without a method passes as unrecounted", "failing = [s for s in out_src if s[\"status\"] in (\"diff\", \"no_recount_method\")]", "failing = [s for s in out_src if s[\"status\"] in (\"diff\",)]"),
]
M["root_inventory"] = [
    ("N00", "NEGATIVE CONTROL: the unmutated tree must pass", None, None),
    ("C00", "CONTROL: a comment-only change must SURVIVE", "DISPOSITIONS = (\"problem_source\"", "DISPOSITIONS = (\"problem_source\""),
    ("M01", "skips the listing check", "[\"bash\", chk, fjp]", "[\"bash\", \"-c\", \"true\"]"),
    ("M02", ".git is kept as a root entry", "        if name == \".git\":", "        if False:"),
    ("M03", "untracked root entries of freeze.json are ignored", "    for u in untracked:", "    for u in []:"),
    ("M04", "gitlinks of freeze.json are ignored", "    for g in gitlink_paths:", "    for g in []:"),
    ("M05", "the LAST matching rule wins", "    for r in rules:\n        if name in (r.get(\"names\") or [])", "    for r in reversed(rules):\n        if name in (r.get(\"names\") or [])"),
    ("M06", "an entry no rule decides is silently excluded", "row.update(disposition=None, rule=None)", "row.update(disposition=\"excluded\", rule=None, reason=\"default\")"),
    ("M07", "the generating run exits 0 with problems", "    return 1 if problems else 0\n\n\ndef check(a):", "    return 0\n\n\ndef check(a):"),
    ("M08", "the check mode exits 0 with problems", "    print(\"root_inventory: check %s rows=%d\" % (\"FAIL\" if problems else \"PASS\", len(rows)))\n    return 1 if problems else 0", "    print(\"root_inventory: check %s rows=%d\" % (\"FAIL\" if problems else \"PASS\", len(rows)))\n    return 0"),
    ("M09", "documentation rows get a doc_class (WP-37 assigns it)", "row[\"doc_class\"] = None            # assigned by WP-37 (T282), never here", "row[\"doc_class\"] = \"pending\""),
    ("M10", "rows are not sorted", "for n in sorted(ent, key=lambda x: x.encode(\"utf-8\"))]", "for n in ent]"),
    ("M11", "the docs/01 tables of 3.1-3.6 count as root rows", "if sec in (\"3.7\", \"3.8\") and l.startswith(\"|\")", "if sec and l.startswith(\"|\")"),
    ("M12", "a sub-path token is read as a root path row", "if name and \"/\" not in name:\n                    idx.setdefault(name, [])", "if name:\n                    idx.setdefault(name, [])"),
    ("M13", "leading dots are stripped from a token (.claude -> claude)", "name = re.sub(r\"^\\./\", \"\", tok.strip()).rstrip(\"/\")", "name = tok.strip().lstrip(\"./\").rstrip(\"/\")"),
    ("M14", "excluded_files are always reported as in the frozen listing", "\"in_frozen_listing\": f[\"path\"] in listing_set", "\"in_frozen_listing\": True"),
    ("M15", "--check does not compare the rows with the freeze", "    if a.freeze_json:\n        fj, paths, glinks, untracked = read_freeze(a.freeze_json)\n        ent, forbidden = root_entries(paths, glinks, untracked)\n        have", "    if False:\n        fj, paths, glinks, untracked = read_freeze(a.freeze_json)\n        ent, forbidden = root_entries(paths, glinks, untracked)\n        have"),
    ("M16", "--check accepts a .git row", "        if e == \".git\":", "        if False:"),
    ("M17", "--check accepts a duplicate row", "        if e in seen:", "        if False:"),
    ("M18", "--check accepts unsorted rows", "    if [r.get(\"entry\") for r in rows] != sorted((r.get(\"entry\") for r in rows), key=lambda x: str(x).encode(\"utf-8\")):", "    if False:"),
    ("M19", "the rule table may name a disposition outside the closed set", "r.get(\"disposition\") not in DISPOSITIONS or ", ""),
    ("M20", "the rule table may omit the field a disposition needs", "        if need and not r.get(need):", "        if False:"),
    ("M21", "an untracked entry is reported as tracked", "ent[u] = {\"kind\": \"untracked\", \"tracked\": False, \"tracked_files\": 0}", "ent[u] = {\"kind\": \"untracked\", \"tracked\": True, \"tracked_files\": 0}"),
    ("M22", "a governance file is counted as a source class", "k = \"governance\" if r[\"entry\"] in GOVERNANCE_ROOT else r.get(\"source_class\", \"none\")", "k = r.get(\"source_class\", \"none\")"),
    ("M23", "a problem_source row without a source class passes --check", "        if d == \"problem_source\" and not r.get(\"source_class\"):", "        if False:"),
    ("M24", "an excluded row without a reason passes --check", "        if d == \"excluded\" and not r.get(\"reason\"):", "        if False:"),
]


def build(tool, out, ids):
    spec = TOOLS[tool]
    tpath = os.path.join(REG, spec["target"])
    src = open(tpath, encoding="utf-8").read()
    made = []
    for mid, what, old, new in M[tool]:
        if ids and mid not in ids:
            continue
        d = os.path.join(out, mid)
        shutil.rmtree(d, ignore_errors=True)
        os.makedirs(d)
        for f in spec["files"]:
            shutil.copy2(os.path.join(REG, f), os.path.join(d, f))
        if old is not None:
            n = src.count(old)
            if n != 1:
                sys.exit("mutate_w2: %s mutant %s anchor occurs %d times (want exactly 1): %r" % (tool, mid, n, old[:90]))
            open(os.path.join(d, spec["target"]), "w", encoding="utf-8").write(src.replace(old, new))
        open(os.path.join(d, "WHAT"), "w").write(what + "\n")
        made.append(mid)
    return made


if __name__ == "__main__":
    a = sys.argv[1:]
    if a[:1] == ["--tools"]:
        print("\n".join(sorted(TOOLS)))
    elif a[:1] == ["--list"] and len(a) == 2:
        for mid, what, _o, _n in M[a[1]]:
            print("%s\t%s" % (mid, what))
    elif a[:1] == ["--field"] and len(a) == 3:
        v = TOOLS[a[1]][a[2]]
        print("\n".join(v) if isinstance(v, list) else v)
    elif a[:1] == ["--env"] and len(a) == 4:
        for var, f in TOOLS[a[1]]["env"].items():
            print("%s=%s" % (var, os.path.join(a[3], f)))
    elif len(a) >= 2 and a[0] in TOOLS:
        print("\n".join(build(a[0], a[1], a[2:])))
    else:
        sys.exit(__doc__)
