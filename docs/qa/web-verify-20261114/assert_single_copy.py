#!/usr/bin/env python3
"""PW-F4: assert exactly ONE copy of each deduped package is in a production build.
usage: assert_single_copy.py <dist-dir> [--expect-multiple PKG]
Reads the source maps (sources[] lists every bundled module path), groups by the
node_modules package root. Control needles: 'node_modules/react' known-present
(must be found) and 'node_modules/definitely-not-a-pkg-zq' known-absent (must not).
Exit 0 = PASS, 1 = FAIL, 2 = BLIND (instrument cannot see)."""
import json, glob, re, sys, collections, os
dist = sys.argv[1]
PKGS = ['react', 'react-dom', '@tanstack/react-query', 'axios']
maps = sorted(glob.glob(os.path.join(dist, 'assets', '*.map')))
if not maps: print('BLIND: no source maps'); sys.exit(2)
roots = collections.defaultdict(set)   # pkg -> set of normalised package roots
allsrc = []
for f in maps:
    allsrc += json.load(open(f))['sources']
for s in allsrc:
    n = re.sub(r'^(\.\./)+', '', s)
    for p in PKGS:
        m = re.match(r'^(.*node_modules/' + re.escape(p) + r')/', n)
        if m: roots[p].add(m.group(1))
needle_pos = any('node_modules/react/' in s for s in allsrc)
needle_neg = any('node_modules/definitely-not-a-pkg-zq/' in s for s in allsrc)
print('sourcemaps=%d sources=%d needle_present(react)=%s needle_absent(fake)=%s' % (len(maps), len(allsrc), needle_pos, needle_neg))
if not needle_pos or needle_neg: print('BLIND: control needle failed'); sys.exit(2)
bad = 0
for p in PKGS:
    print('%-24s copies=%d %s' % (p, len(roots[p]), sorted(roots[p])))
    if len(roots[p]) != 1: bad += 1
print('RESULT', 'FAIL' if bad else 'PASS'); sys.exit(1 if bad else 0)
