#!/usr/bin/env python3
"""scope_to_lumen_json.py - derive the gen_lumenignore.py input JSON from config/index/scope.yaml (T018).

Purpose: the structural index scope (scope.yaml, rendered by scope_render.py) and the semantic index scope
  (.lumenignore, rendered by gen_lumenignore.py) must come from ONE class source (constitution 11.4.275(B)). This tool
  turns scope.yaml into the JSON form gen_lumenignore.py reads: {"allow": [...], "deny": [...], "root_files": bool}.
Usage:   scope_to_lumen_json.py --scope scope.yaml --submodules-tsv FILE --out FILE      derive
         scope_to_lumen_json.py --scope scope.yaml --submodules-tsv FILE --check FILE    prove no class was lost
Inputs:  scope.yaml (keys baseline_excludes{class: [patterns]}, project_excludes, pathological_excludes, optional
         lumen_allow_roots [rel dirs]); the submodules TSV written by scripts/audit/derive_scope.sh (columns path,
         class, URL, alt_class, flag).
Outputs: JSON, sort_keys, indent 2, final newline, no timestamp, keys: allow, deny, root_files (true), classes
         ({class: [deny patterns]}), dropped_negations. gen_lumenignore.py reads only allow/deny/root_files.
  allow = own rows of the TSV + lumen_allow_roots; deny = every class pattern + every third_party row nested under an
  allowed root. A pattern that does not start with "/" or "**" is written as "**/<pattern>" (gen_lumenignore.py would
  anchor it at the root); a nested third_party path is written literally (glob metacharacters backslash-escaped); a negation ("!x") is dropped and listed under dropped_negations (the generator cannot carry
  it, so a directory such as secretmgr/ loses semantic coverage: recorded, not hidden).
  --check is an EXACT re-derive-and-compare: allow, deny, root_files, classes and dropped_negations of FILE must equal what the
  scope and the TSV derive, so a missing entry AND an extra one (a third-party root in allow, an extra deny pattern, a
  flipped root_files) are both found.
Exit:    0 ok; 1 --check found a difference; 2 usage; 3 fail closed (empty allow, unreadable or malformed input, a TSV row whose
         class is not own|third_party or that lacks a path or a tab, a path classed both ways, a --check list with a
         non-string element, a lumen_allow_roots entry equal to or inside a third_party
         row (11.4.79(6): scope.yaml can not let third-party code into the semantic index), an unwritable --out).
Side effects: writes --out only on success.
"""
import argparse
import json
import sys

import yaml


def fail(code, msg):
    print(msg, file=sys.stderr)
    sys.exit(code)


def conv(p):
    return p if p.startswith(("/", "**")) else "**/" + p


def lit(path):
    """A path that is a FILE NAME in a deny pattern, not a glob: gitignore-style backslash escapes for the metacharacters (and a leading
    '#' or '!'), so `x[1]` denies x[1] only and never its sibling `x1`."""
    out = "".join("\\" + c if c in "\\*?[]" else c for c in path)
    return ("\\" + out) if out[:1] in ("#", "!") else out


def validate(scope):
    """Fail closed (exit 3) on a scope.yaml shape the derivation cannot read faithfully."""
    if not isinstance(scope, dict):
        fail(3, "malformed scope: the top level must be a mapping, got %s" % type(scope).__name__)
    be = scope.get("baseline_excludes")
    if be is not None and (not isinstance(be, dict) or any(not isinstance(v, (list, type(None))) for v in be.values())):
        fail(3, "malformed scope: baseline_excludes must map class names to lists of patterns")
    if be is not None and any(not isinstance(k, str) for k in be):   # a YAML key `1:` or `null:` is no class name (N6-6): rc 3, not a TypeError in sorted()
        fail(3, "malformed scope: every class name of baseline_excludes must be a string, got %r" % [k for k in be if not isinstance(k, str)][:3])
    for k in ("project_excludes", "pathological_excludes", "lumen_allow_roots"):
        if scope.get(k) is not None and not isinstance(scope[k], list):
            fail(3, "malformed scope: %s must be a list" % k)


def parse_rows(tsv):
    """The submodules TSV rows (path, class, ...): every non-blank row must carry a path and a class of own|third_party, else exit 3."""
    rows = []
    for n, line in enumerate(tsv, 1):
        if not line.strip():
            continue
        r = line.rstrip("\n").split("\t")
        if len(r) < 2 or not r[0].strip() or r[1] not in ("own", "third_party"):
            fail(3, "malformed submodules TSV row %d (want path<TAB>own|third_party<TAB>...): %r" % (n, line.rstrip("\n")[:120]))
        rows.append(r)
    seen = {}
    for r in rows:   # a path classed BOTH own and third_party would be allowed and not denied: refuse, never pick one
        if seen.setdefault(r[0], r[1]) != r[1]:
            fail(3, "malformed submodules TSV: %r is classed both own and third_party" % r[0])
    return rows


def derive(scope, tsv):
    validate(scope)
    classes, dropped = {}, []

    def add(name, pats):
        for p in pats or []:
            p = str(p).strip()
            if not p:
                continue
            if p.startswith("!"):
                dropped.append(p)
                continue
            classes.setdefault(name, set()).add(conv(p))

    for name, pats in (scope.get("baseline_excludes") or {}).items():
        add(name, pats)
    add("project_excludes", scope.get("project_excludes"))
    add("pathological_excludes", scope.get("pathological_excludes"))
    rows = parse_rows(tsv)
    allow = {r[0] for r in rows if r[1] == "own"}
    roots = {str(a).strip().strip("/") for a in (scope.get("lumen_allow_roots") or []) if str(a).strip().strip("/")}
    third = {r[0] for r in rows if r[1] == "third_party"}
    for a in sorted(roots):   # 11.4.79(6): third-party code never enters the semantic index, not even by an explicit root
        hit = sorted(t for t in third if a == t or a.startswith(t + "/"))
        if hit:
            fail(3, "lumen_allow_roots entry %r equals or lies inside the third_party submodule %r: refusing (11.4.79(6))" % (a, hit[0]))
    allow |= roots
    if not allow:
        fail(3, "empty allow-list: no own submodule rows and no lumen_allow_roots; refusing")
    nested = {"/" + lit(r[0]) + "/" for r in rows if r[1] == "third_party"
              and any(r[0].startswith(a + "/") for a in allow)}
    if nested:
        classes["third_party_nested"] = nested
    deny = sorted(set().union(*classes.values())) if classes else []
    return {
        "allow": sorted(allow),
        "classes": {k: sorted(v) for k, v in sorted(classes.items())},
        "deny": deny,
        "dropped_negations": sorted(set(dropped)),
        "root_files": True,
    }


def main():
    ap = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    ap.add_argument("--scope", required=True)
    ap.add_argument("--submodules-tsv", required=True)
    g = ap.add_mutually_exclusive_group(required=True)
    g.add_argument("--out")
    g.add_argument("--check")
    try:
        a = ap.parse_args()
    except SystemExit as e:
        sys.exit(2 if e.code else 0)
    try:
        with open(a.scope, encoding="utf-8") as fh:
            scope = yaml.safe_load(fh)
        scope = {} if scope is None else scope
        with open(a.submodules_tsv, encoding="utf-8") as fh:
            tsv = fh.readlines()
    except (OSError, yaml.YAMLError, UnicodeDecodeError) as e:   # input that is not UTF-8 is malformed input: rc 3, never a traceback (N6-6)
        fail(3, "unreadable input: %s" % e)
    want = derive(scope, tsv)
    if a.out:
        text = json.dumps(want, indent=2, sort_keys=True) + "\n"
        try:
            with open(a.out, "w") as f:
                f.write(text)
        except OSError as e:
            fail(3, "cannot write --out: %s" % e)
        return
    try:
        with open(a.check, encoding="utf-8") as fh:
            have = json.load(fh)
    except (OSError, ValueError) as e:   # UnicodeDecodeError is a ValueError
        fail(3, "unreadable --check file: %s" % e)
    if not isinstance(have, dict):
        fail(3, "malformed --check file: the top level must be an object")
    for key in ("allow", "deny", "dropped_negations"):
        if key in have and not isinstance(have[key], list):
            fail(3, "malformed --check file: %s must be a list" % key)
        if key in have and any(not isinstance(x, str) for x in have[key]):
            fail(3, "malformed --check file: %s must be a list of strings" % key)
    if "classes" in have and (not isinstance(have["classes"], dict)
                              or any(not isinstance(v, list) for v in have["classes"].values())):
        fail(3, "malformed --check file: classes must map class names to lists")
    if "classes" in have and any(not isinstance(x, str) for v in have["classes"].values() for x in v):
        fail(3, "malformed --check file: every classes list must hold strings only")
    diffs = []
    for key in ("allow", "deny", "classes", "dropped_negations", "root_files"):
        if key not in have:
            diffs.append("key missing: " + key)
            continue
        # root_files is compared with its TYPE: Python's 1 == True would let `"root_files": 1` pass an "exact" check (N6-6)
        if have[key] == want[key] and (key != "root_files" or type(have[key]) is type(want[key])):
            continue
        if isinstance(want[key], dict):
            for c in sorted(set(want[key]) | set(have[key])):
                if c not in have[key]:
                    diffs.append("class missing: " + c)
                elif c not in want[key]:
                    diffs.append("class not in the scope (extra): " + c)
                else:
                    for pat in sorted(set(want[key][c]) - set(have[key][c])):
                        diffs.append("pattern missing: %s in class %s" % (pat, c))
                    for pat in sorted(set(have[key][c]) - set(want[key][c])):
                        diffs.append("pattern not in the scope (extra): %s in class %s" % (pat, c))
        elif isinstance(want[key], list):
            for x in sorted(set(want[key]) - set(have[key])):
                diffs.append("%s entry missing: %s" % (key, x))
            for x in sorted(set(have[key]) - set(want[key])):
                diffs.append("%s entry not derived (extra): %s" % (key, x))
            if set(have[key]) == set(want[key]):   # same members, different list: duplicates (the comparison is EXACT, not set-wise)
                diffs.append("%s has duplicate or misplaced entries (have %d, derived %d)" % (key, len(have[key]), len(want[key])))
        else:
            diffs.append("%s differs: have %r, derived %r" % (key, have[key], want[key]))
    missing = diffs
    if missing:
        print("\n".join(missing), file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
