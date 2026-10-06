#!/usr/bin/env python3
"""canonical_result.py - T217 helper (doc12 8.3). The canonical result of a run: the sorted list of (challenge_id, step_index, verdict, assertion_digest)
read from a conduit stream, excluding timestamps, durations, tokens, sequence numbers and generated ids; three runs from the same state must give the same hash.
Usage: canonical_result.py --stream FILE [--out FILE]     prints the sha256 of the canonical list; --out writes {schema qa-canonical/1, entries, canonical_hash}.
challenge_step events contribute (challenge, step, verdict, fields.assertion_digest); challenge_verdict events contribute (challenge, "-1", verdict, "").
Verdicts are normalised (PASS -> pass, ...). Exits 0, 2 usage / unreadable stream, 3 malformed stream."""
import argparse
import hashlib
import json
import sys

NORM = {"PASS": "pass", "FAIL": "fail", "OPERATOR-BLOCKED": "blocked", "SKIP": "skip"}


def canonical(stream):
    entries = []
    with open(stream) as fh:
        for n, line in enumerate(fh, 1):
            if not line.strip():
                continue
            try:
                e = json.loads(line)
            except ValueError:
                raise ValueError("line %d is not JSON" % n)
            t = e.get("type")
            if t == "challenge_step":
                entries.append([str(e.get("challenge", "")), str(e.get("step", "")), NORM.get(e.get("verdict"), str(e.get("verdict"))), str((e.get("fields") or {}).get("assertion_digest", ""))])
            elif t == "challenge_verdict":
                entries.append([str(e.get("challenge", "")), "-1", NORM.get(e.get("verdict"), str(e.get("verdict"))), ""])
    entries.sort()
    h = hashlib.sha256(json.dumps(entries, sort_keys=True, separators=(",", ":")).encode()).hexdigest()
    return entries, h


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--stream", required=True)
    ap.add_argument("--out")
    try:
        ns = ap.parse_args()
    except SystemExit as e:
        sys.exit(2 if e.code not in (0, None) else 0)
    try:
        entries, h = canonical(ns.stream)
    except OSError as e:
        sys.stderr.write("canonical_result: %s\n" % e)
        sys.exit(2)
    except ValueError as e:
        sys.stderr.write("canonical_result: %s\n" % e)
        sys.exit(3)
    if ns.out:
        with open(ns.out, "w") as fh:
            json.dump({"schema": "qa-canonical/1", "entries": entries, "canonical_hash": h}, fh, sort_keys=True, indent=1)
            fh.write("\n")
    print(h)


if __name__ == "__main__":
    main()
