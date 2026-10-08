#!/usr/bin/env bash
# check_freeze_listing.sh - re-check the tracked-file listing of a frozen commit (WP-20, T161 rule; consumers T166, T174, T167).
#   check_freeze_listing.sh <freeze.json>              the listing named by freeze.json (`listing`) must hash to the sha256 recorded in
#                                                      freeze.json (`listing_sha256`) and must be well formed (NUL-terminated records)
#   check_freeze_listing.sh <listing> <manifest>       the two-argument form lead_scan.py calls: the listing's path SET must equal the
#                                                      snapshot manifest's path set (a dropped or extra path is detected, not assumed absent)
# STAND-IN NOTICE (UNCONFIRMED vs T161): T161 (freeze) owns this file; written by the W2 register worker because T166 and T174 name it as their
# one listing check. The freeze.json field names `listing` and `listing_sha256` are the stand-in shape (UNCONFIRMED vs the T161 owner). The
# check reads the listing and freeze.json only: no git, no ls, no snapshot file (T174 reads no snapshot file at all).
# Refusals (stderr `check_freeze_listing: REFUSED reason=<code> ...`, exit 20): freeze_listing_moved (hash or path set differs),
# freeze_listing_unrecorded (freeze.json names no listing or no listing_sha256), freeze_listing_invalid (unreadable, empty, a record that is not
# NUL-terminated, an empty path, a duplicate path). Exit 2 usage; 0 = the listing is the recorded one.
set -u
{ [ $# -eq 1 ] && [ -f "$1" ]; } || { [ $# -eq 2 ] && [ -f "$1" ] && [ -f "$2" ]; } || { echo "usage: check_freeze_listing.sh <freeze.json> | <listing> <manifest>" >&2; exit 2; }
exec python3 -I - "$@" <<'PY'
import hashlib, json, sys

def refuse(reason, detail=""):
    sys.stderr.write("check_freeze_listing: REFUSED reason=%s %s\n" % (reason, detail))
    sys.exit(20)

def read_listing(path):
    try:
        data = open(path, "rb").read()
    except OSError as e:
        refuse("freeze_listing_invalid", "%s: %s" % (path, type(e).__name__))
    if not data:
        refuse("freeze_listing_invalid", "%s is empty" % path)
    if not data.endswith(b"\0"):
        refuse("freeze_listing_invalid", "%s: the last record is not NUL-terminated (a newline-separated listing is refused)" % path)
    recs = data.split(b"\0")[:-1]
    if any(r == b"" for r in recs):
        refuse("freeze_listing_invalid", "%s holds an empty path record" % path)
    if len(set(recs)) != len(recs):
        dup = sorted({r for r in recs if recs.count(r) > 1})[:3]
        refuse("freeze_listing_invalid", "%s holds a duplicate path: %r" % (path, dup))
    return data, recs

a = sys.argv[1:]
if len(a) == 1:
    try:
        fj = json.load(open(a[0], encoding="utf-8"))
    except (OSError, ValueError) as e:
        refuse("freeze_listing_unrecorded", "%s: %s" % (a[0], type(e).__name__))
    if not isinstance(fj, dict) or not isinstance(fj.get("listing"), str) or not isinstance(fj.get("listing_sha256"), str):
        refuse("freeze_listing_unrecorded", "%s names no `listing` path and `listing_sha256`" % a[0])
    data, _recs = read_listing(fj["listing"])
    h = hashlib.sha256(data).hexdigest()
    if h != fj["listing_sha256"]:                      # MUT:listing-hash
        refuse("freeze_listing_moved", "the listing %s hashes to %s, freeze.json records %s" % (fj["listing"], h, fj["listing_sha256"]))
else:
    _data, recs = read_listing(a[0])
    try:
        want = {e["path"] for e in json.load(open(a[1], encoding="utf-8"))["entries"]}
    except (OSError, ValueError, KeyError, TypeError) as e:
        refuse("freeze_manifest_invalid", "%s: %s" % (a[1], type(e).__name__))
    listed = set()
    for r in recs:
        try:
            listed.add(r.decode("utf-8"))
        except UnicodeDecodeError:
            refuse("freeze_path_unsafe", repr(r))
    bad = sorted("added " + p for p in listed - want) + sorted("missing " + p for p in want - listed)
    if bad:
        refuse("freeze_listing_moved", "; ".join(bad)[:400])
PY
