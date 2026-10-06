#!/usr/bin/env python3
"""manifest_check.py - T213 helper (doc12 6.7). Checks challenges/helixqa-banks/MANIFEST.yaml against the bank directory and the scripts.

Usage: manifest_check.py --root REPO_ROOT [--legacy FILE]
  1. every *.yaml bank under challenges/helixqa-banks/ (MANIFEST.yaml excluded) is listed in the manifest and every listed file exists;
     each listed case_floor is <= the number of cases the bank holds (a floor);
  2. no file of scripts/, challenges/ (banks and MANIFEST.yaml themselves excluded) or tests/ names a bank file: neither a bank basename nor a
     `<banks dir>/<file>.yaml|.yml|.json` path literal (the manifest itself excepted) (QF-04). --legacy FILE lists transitional offenders (one repo-relative path per line,
     # comments) that are tolerated while the legacy drivers exist; a listed file that no longer offends is itself reported (stale list).
Exits 0 clean, 1 findings (printed `FINDING <kind> <detail>`), 2 usage.
"""
import argparse
import os
import re
import sys

import yaml

SKIP_DIRS = {".git", ".audit", "node_modules", "__pycache__", ".pytest_cache"}
PATH_RE = re.compile(r"helixqa-banks/(?!MANIFEST\.yaml)[A-Za-z0-9_.-]+\.(?:ya?ml|json)")


def bank_files(root):
    d = os.path.join(root, "challenges", "helixqa-banks")
    return sorted(n for n in os.listdir(d) if n.endswith((".yaml", ".yml")) and n != "MANIFEST.yaml")


def check_manifest(root):
    out = []
    d = os.path.join(root, "challenges", "helixqa-banks")
    try:
        m = yaml.safe_load(open(os.path.join(d, "MANIFEST.yaml")))
    except Exception as e:
        return ["FINDING manifest_unreadable %s" % e]
    if not isinstance(m, dict) or m.get("schema") != "qa-manifest/1" or not isinstance(m.get("banks"), list):
        return ["FINDING manifest_malformed schema/banks"]
    listed = {}
    for b in m["banks"]:
        if not isinstance(b, dict) or "file" not in b:
            out.append("FINDING manifest_row_malformed %r" % (b,))
            continue
        listed[b["file"]] = b
    actual = bank_files(root)
    for n in actual:
        if n not in listed:
            out.append("FINDING unlisted_bank %s" % n)
    for n, b in sorted(listed.items()):
        p = os.path.join(d, n)
        if not os.path.isfile(p):
            out.append("FINDING listed_bank_missing %s" % n)
            continue
        cases = (yaml.safe_load(open(p)) or {}).get("test_cases") or []
        floor = b.get("case_floor")
        if not isinstance(floor, int) or floor < 1:
            out.append("FINDING floor_missing %s" % n)
        elif len(cases) < floor:
            out.append("FINDING below_floor %s holds %d < floor %d" % (n, len(cases), floor))
        if not b.get("profiles") or not b.get("platforms"):
            out.append("FINDING profile_or_platform_missing %s" % n)
    return out


def scan_literals(root, names):
    hits = {}
    for top in ("scripts", "challenges", "tests"):
        base = os.path.join(root, top)
        for dp, dns, fns in os.walk(base):
            dns[:] = [x for x in dns if x not in SKIP_DIRS]
            for fn in fns:
                rel = os.path.relpath(os.path.join(dp, fn), root)
                if rel.startswith("challenges/helixqa-banks/") and "/" not in rel[len("challenges/helixqa-banks/"):]:
                    continue  # the bank files and the manifest themselves
                try:
                    txt = open(os.path.join(dp, fn), errors="strict").read()
                except (UnicodeDecodeError, OSError):
                    continue
                found = sorted({n for n in names if n in txt} | set(PATH_RE.findall(txt)))
                if found:
                    hits[rel] = found
    return hits


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--root", required=True)
    ap.add_argument("--legacy")
    ns = ap.parse_args()
    out = check_manifest(ns.root)
    legacy = set()
    if ns.legacy:
        legacy = {ln.strip() for ln in open(ns.legacy) if ln.strip() and not ln.startswith("#")}
    hits = scan_literals(ns.root, bank_files(ns.root))
    for rel, found in sorted(hits.items()):
        if rel in legacy:
            continue
        out.append("FINDING bank_path_literal %s names %s" % (rel, ", ".join(found)))
    for rel in sorted(legacy - set(hits)):
        out.append("FINDING stale_legacy_entry %s no longer names a bank file" % rel)
    for line in out:
        print(line)
    sys.exit(1 if out else 0)


if __name__ == "__main__":
    main()
