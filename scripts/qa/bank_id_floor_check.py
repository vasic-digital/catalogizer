#!/usr/bin/env python3
"""bank_id_floor_check.py - T214 helper. The directory scan of the bank-id floor (doc12 7.7): every case id recorded in <dir>/.bank-id-floor.txt must still be held
by a bank file of the directory. The same rule as the HelixQA loader's checkBankIDFloor (submodules/helix_qa/pkg/testbank/loader.go, HXC-305), implemented here so
a check can run without building helixqa; helixqa itself (`helixqa banks regen-floor`) remains the tool that writes the floor.
Usage: bank_id_floor_check.py --banks DIR      Exits 0 (floor holds, or no floor file: printed as `NOTE: floor not enforced`), 1 (ids missing: printed `FINDING missing_id <id>`),
2 usage / unreadable input / a floor file that records no id (a header-only floor protects nothing: refused like the loader's RegenerateBankIDFloor).
Floor format: one case id per line; blank lines and lines starting with # are ignored. ADDING a case never trips the floor; only a removal does."""
import argparse
import os
import sys

import yaml


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--banks", required=True)
    try:
        ns = ap.parse_args()
    except SystemExit as e:
        sys.exit(2 if e.code not in (0, None) else 0)
    d = ns.banks
    if not os.path.isdir(d):
        sys.stderr.write("bank_id_floor_check: %s is not a directory\n" % d)
        sys.exit(2)
    present = set()
    for n in sorted(os.listdir(d)):
        if not n.endswith((".yaml", ".yml")) or n == "MANIFEST.yaml":
            continue
        try:
            doc = yaml.safe_load(open(os.path.join(d, n))) or {}
        except Exception as e:
            sys.stderr.write("bank_id_floor_check: %s does not parse: %s\n" % (n, str(e).splitlines()[0]))
            sys.exit(2)
        for c in doc.get("test_cases") or []:
            if isinstance(c, dict) and c.get("id"):
                present.add(str(c["id"]))
    fp = os.path.join(d, ".bank-id-floor.txt")
    if not os.path.exists(fp):
        print("NOTE: floor not enforced (no .bank-id-floor.txt in %s)" % d)
        sys.exit(0)
    ids = [ln.strip() for ln in open(fp) if ln.strip() and not ln.strip().startswith("#")]
    if not ids:
        sys.stderr.write("bank_id_floor_check: the floor file records no case id\n")
        sys.exit(2)
    missing = sorted(i for i in ids if i not in present)
    for i in missing:
        print("FINDING missing_id %s" % i)
    if missing:
        print("FINDING floor_violated %d id(s) recorded in .bank-id-floor.txt are absent from the banks" % len(missing))
        sys.exit(1)
    print("floor holds: %d ids, %d held" % (len(ids), len(present)))


if __name__ == "__main__":
    main()
