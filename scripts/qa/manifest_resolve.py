#!/usr/bin/env python3
"""manifest_resolve.py - T217 helper (doc12 6.7, 8.1 step 1). Resolves one run profile from MANIFEST.yaml for scripts/qa/run_profile.sh.
Usage: manifest_resolve.py --manifest FILE --profile P [--banks-dir DIR]
Prints TAB separated lines the wrapper reads (no shell evaluation of manifest text):
  IMAGE<TAB>id<TAB>digest-or-empty
  PLATFORM<TAB>value                the `helixqa run --platform` value of the profile (api and installer: all)
  BANK<TAB>file<TAB>floor<TAB>held  one per bank listed for the profile (held = the number of cases the bank file holds today)
  PROBE<TAB>name<TAB>json-args      the profile's probe plan, `$NAME` words expanded from the environment
  PROBEENV<TAB>name<TAB>VARNAME     a plan whose `$NAME` variable is unset (the wrapper reports the probe blocked: credential_absent)
or `ERR<TAB>code<TAB>detail` with exit 3 (manifest_unreadable, manifest_malformed, profile_unknown, no_bank_for_profile, bank_missing).
"""
import argparse
import json
import os
import re
import sys

import yaml

PLATFORM = {"api": "all", "web": "web", "desktop": "desktop", "android": "android", "androidtv": "android", "installer": "all"}


def err(code, detail):
    print("ERR\t%s\t%s" % (code, detail))
    sys.exit(3)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--manifest", required=True)
    ap.add_argument("--profile", required=True)
    ap.add_argument("--banks-dir")
    ns = ap.parse_args()
    try:
        m = yaml.safe_load(open(ns.manifest))
    except Exception as e:
        err("manifest_unreadable", str(e).splitlines()[0])
    if not isinstance(m, dict) or m.get("schema") != "qa-manifest/1" or not isinstance(m.get("banks"), list) or not isinstance(m.get("profiles"), list):
        err("manifest_malformed", "schema qa-manifest/1, profiles and banks are required")
    if ns.profile not in m["profiles"]:
        err("profile_unknown", "%s is not one of %s" % (ns.profile, ",".join(map(str, m["profiles"]))))
    bdir = ns.banks_dir or os.path.dirname(os.path.abspath(ns.manifest))
    img = m.get("image") if isinstance(m.get("image"), dict) else {}
    print("IMAGE\t%s\t%s" % (img.get("id") or "", img.get("digest") or ""))
    print("PLATFORM\t%s" % PLATFORM.get(ns.profile, "all"))
    n = 0
    for b in m["banks"]:
        if not isinstance(b, dict) or ns.profile not in (b.get("profiles") or []):
            continue
        n += 1
        path = os.path.join(bdir, b["file"])
        if not os.path.isfile(path):
            err("bank_missing", b["file"])
        doc = yaml.safe_load(open(path)) or {}
        held = len(doc.get("test_cases") or [])
        print("BANK\t%s\t%s\t%d" % (b["file"], b.get("case_floor", 0), held))
    if n == 0:
        err("no_bank_for_profile", ns.profile)
    for pr in (m.get("profile_probes") or {}).get(ns.profile) or []:
        args, missing = [], None
        for a in pr.get("args") or []:
            a = str(a)
            mm = re.fullmatch(r"\$([A-Za-z_][A-Za-z0-9_]*)", a)
            if mm:
                if mm.group(1) not in os.environ or not os.environ[mm.group(1)]:
                    missing = mm.group(1)
                    break
                a = os.environ[mm.group(1)]
            args.append(a)
        if missing:
            print("PROBEENV\t%s\t%s" % (pr["probe"], missing))
        else:
            print("PROBE\t%s\t%s" % (pr["probe"], json.dumps(args)))


if __name__ == "__main__":
    main()
