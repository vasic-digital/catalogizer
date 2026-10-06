#!/usr/bin/env python3
"""forge.py - TEST-ONLY ledger forger (T054, T055, T056). Never installed, never called by a tool: it builds the attacked and the
impossible-to-record ledgers that the golden-bad fixtures need. Every mode re-chains the whole ledger afterwards (the attacker who
recomputes, docs/06 s13.3), unless the mode says otherwise.

  forge.py rechain LEDGER                       recompute prev_hash / entry_hash of every line, keep the seq values as they are
  forge.py renumber LEDGER                      rechain AND renumber seq 1..n (the careful forger)
  forge.py delete LEDGER SEQ [renumber]         delete the entry with this seq, then rechain (delete-and-recompute)
  forge.py truncate LEDGER N                    keep the first N lines (no rechain needed)
  forge.py retag LEDGER SEQ POLARITY            change the polarity of one entry, then rechain (a record the strict recorder refuses to write)
  forge.py set LEDGER SEQ KEY JSON              set one field of one entry, then rechain
  forge.py swap LEDGER SEQ1 SEQ2                swap two lines, then rechain, keeping the swapped entries' own seq values
"""
import json, os, sys
sys.path.insert(0, os.path.join(os.path.dirname(os.path.realpath(__file__)), ".."))
sys.dont_write_bytecode = True
import evcore


def load(p):
    return [json.loads(l) for l in open(p, "rb").read().split(b"\n") if l]


def save(p, recs, rechain=True, renumber=False):
    prev, out = evcore.ZERO, []
    for i, r in enumerate(recs, 1):
        r = dict(r)
        if renumber:
            r["seq"] = i
        if rechain:
            r["prev_hash"] = prev
            r.pop("entry_hash", None)
            r["entry_hash"] = evcore.compute_entry_hash(r)
            prev = r["entry_hash"]
        out.append(evcore.canon(r) + b"\n")
    with open(p, "wb") as f:
        f.write(b"".join(out))


def main(a):
    mode, p = a[0], a[1]
    recs = load(p)
    if mode == "rechain":
        save(p, recs)
    elif mode == "renumber":
        save(p, recs, renumber=True)
    elif mode == "delete":
        recs = [r for r in recs if r["seq"] != int(a[2])]
        save(p, recs, renumber=len(a) > 3 and a[3] == "renumber")
    elif mode == "truncate":
        save(p, recs[:int(a[2])], rechain=False)
    elif mode == "retag":
        for r in recs:
            if r["seq"] == int(a[2]):
                r["polarity"] = a[3]
        save(p, recs)
    elif mode == "set":
        for r in recs:
            if r["seq"] == int(a[2]):
                r[a[3]] = json.loads(a[4])
        save(p, recs)
    elif mode == "swap":
        i = [r["seq"] for r in recs].index(int(a[2])); j = [r["seq"] for r in recs].index(int(a[3]))
        recs[i], recs[j] = recs[j], recs[i]
        save(p, recs)
    else:
        sys.exit("forge.py: unknown mode %s" % mode)


main(sys.argv[1:])
