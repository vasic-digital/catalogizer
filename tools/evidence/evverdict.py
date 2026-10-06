"""evverdict - the verdict deriver behind tools/evidence/verdict (tasks.md T054, docs/06 s4.2 step 7, s10, s13.1, s13.4, DR-E4).

    verdict ITEM [--ledger FILE] [--blobs DIR] [--chain-only] [--state-delta]

The verdict of an item is DERIVED by program from the ledger, never typed by an author (DR-E4, 11.4.240, 11.4.249). The ledger is
walked first: a ledger whose chain, canonical form or ev/1 schema fails is UNVERIFIED (exit 3, named reason), no verdict is derived
from it. `--chain-only` walks the chain without the per-entry ev/1 schema check; the golden-bad fixtures of docs/06 s13.4 that the
strict recorder REFUSES to write (a RED that passed, a GREEN that failed, a REOPEN that passed) are derived that way, and a
production caller never passes it.

The rules are the ones of the proof of concept (docs/06 s13, revision 5), ported to ev/1 entries:
  red_ok            at least one RED, every RED a genuine test failure (exit 1..125, verdict fail; 126/127/signal are not)
  green_ok          at least three GREEN, three distinct iteration values, every GREEN exit 0 and verdict pass
  green_identical   one stdout digest and one target fingerprint across the GREEN entries (an entry's own run token masked in its stdout)
  same_test         one argv (the `--run-token VALUE` pair removed: it differs per run by design, T051a), one cwd, one target class, one
                    target locator and one test_fingerprint across RED and GREEN, none of the last three missing
  red_before_green  every RED precedes every GREEN in the ledger (max RED seq < min GREEN seq)
  fingerprints_differ  no GREEN target fingerprint equals a RED one
  fingerprints_new  no GREEN ran on a fingerprint that a cutting REOPEN showed failing or that was GREEN in an earlier cycle
  cycle             a REOPEN ends the previous cycle only when it CUTS: a genuine failure of the same test as the cycle it ends, and
                    that cycle itself derived PASS; any other REOPEN is ignored and listed in reopens_ignored (it cannot discard a
                    failing GREEN). Only entries after the last cutting REOPEN count.
  token_ok          (T051a, constitution 7.1) present only when the rule applies: with --state-delta, or when any RED/GREEN entry of the
                    cycle carries `--run-token` in its argv (a caller cannot opt out by omitting the flag). Every GREEN must then carry
                    a token in its argv, the token must appear as a whole token in ITS OWN captured stdout (the test wrote it into the
                    state it changed and read it back), and no other entry of the item may carry the same token. A post-state that
                    carries the token of an earlier run is the stale-state bluff and fails here.
Exit status: 0 PASS, 1 FAIL, 3 UNVERIFIED (the ledger cannot be trusted or read), 64 usage.
Stated limits (11.4.6): a MUTATION entry is not part of this derivation (the register seam of docs/06 s17 step 5 checks the closing
mutation); the oracle strength and the evidence class are not judged here (the reviewer's table, docs/06 s18); the token rule proves
the token was in the captured stdout of its own entry, not that the state was changed correctly.
"""
import json
import os
import re
import sys

import evcore

CHECKS = ("red_ok", "green_ok", "green_identical", "same_test", "red_before_green", "fingerprints_differ", "fingerprints_new")
TOKEN = re.compile(r"[0-9a-f]{32}")


argv_token = evcore.argv_token        # one definition of the token shape (evcore), shared with `verify` and `redact_argv`


def norm_argv(argv):
    if not isinstance(argv, list):
        return argv
    out, i = [], 0
    while i < len(argv):
        if argv[i] == "--run-token" and i + 1 < len(argv) and isinstance(argv[i + 1], str) and TOKEN.fullmatch(argv[i + 1]):
            i += 2
            continue
        out.append(argv[i])
        i += 1
    return out


def _j(v):
    return json.dumps(v, sort_keys=True)


def genuine(e):
    x = e.get("exit_status")
    return isinstance(x, int) and 1 <= x <= 125 and e.get("verdict") == "fail"


def same(a, b):
    return (_j(norm_argv(a.get("argv"))) == _j(norm_argv(b.get("argv"))) and all(a.get(k) == b.get(k) for k in
            ("cwd", "target_class", "target_ref", "test_fingerprint")))


def _unique(vals):
    return len({_j(v) for v in vals})


def _blob_has(bdir, digest, tok):
    if not bdir or not isinstance(digest, str):
        return False
    try:
        with open(os.path.join(bdir, digest), "rb") as f:
            data = f.read()
    except OSError:
        return False
    return re.search(rb"(?<![0-9a-f])" + tok.encode() + rb"(?![0-9a-f])", data) is not None


def stdout_identity(e, bdir):
    """The stdout digest used for the three-identical-runs comparison: the raw digest, except that an entry that carries a run token has
    that token MASKED in its stored stdout first (the token differs per run by design, T051a; everything else must still be identical)."""
    t = argv_token(e.get("argv"))
    if t and bdir and isinstance(e.get("stdout_sha256"), str):
        try:
            with open(os.path.join(bdir, e["stdout_sha256"]), "rb") as f:
                data = f.read()
            return evcore.sha_bytes(re.sub(rb"(?<![0-9a-f])" + t.encode() + rb"(?![0-9a-f])", b"<EVREC_RUN_TOKEN>", data))
        except OSError:
            pass
    return e.get("stdout_sha256")


def token_rule(red, grn, all_entries, state_delta, bdir):
    """None when the rule does not apply, else (ok, notes)."""
    if not (state_delta or any(argv_token(e.get("argv")) for e in red + grn)):
        return None
    notes = []
    count = {}
    for e in all_entries:
        t = argv_token(e.get("argv"))
        if t:
            count[t] = count.get(t, 0) + 1
    for e in grn:
        t = argv_token(e.get("argv"))
        if not t:
            notes.append("seq %s: no run token in argv" % e.get("seq"))
        elif count.get(t, 0) > 1:
            notes.append("seq %s: token shared with %d other entr%s" % (e.get("seq"), count[t] - 1, "y" if count[t] == 2 else "ies"))
        elif not _blob_has(bdir, e.get("stdout_sha256"), t):
            notes.append("seq %s: the captured post-state does not carry the token of its own entry" % e.get("seq"))
    if not grn:
        notes.append("no GREEN entry to carry a token")
    return (not notes), notes


def checks(window, banned, all_entries, state_delta, bdir):
    red = [e for e in window if e.get("polarity") == "RED"]
    grn = [e for e in window if e.get("polarity") == "GREEN"]
    both = red + grn
    o = {}
    o["red_ok"] = len(red) >= 1 and all(genuine(e) for e in red)
    its = {e.get("iteration") for e in grn}
    o["green_ok"] = len(grn) >= 3 and len(its) >= 3 and all(e.get("exit_status") == 0 and e.get("verdict") == "pass" for e in grn)
    o["green_identical"] = _unique([stdout_identity(e, bdir) for e in grn]) == 1 and _unique([e.get("target_fingerprint") for e in grn]) == 1
    o["same_test"] = (_unique([norm_argv(e.get("argv")) for e in both]) == 1 and all(_unique([e.get(k) for e in both]) == 1
                      for k in ("cwd", "target_class", "target_ref", "test_fingerprint"))
                      and all(e.get("target_class") is not None and e.get("target_ref") is not None and e.get("test_fingerprint") is not None
                              for e in both))
    o["red_before_green"] = len(red) >= 1 and len(grn) >= 1 and max(e["seq"] for e in red) < min(e["seq"] for e in grn)
    rf, gf = [e.get("target_fingerprint") for e in red], [e.get("target_fingerprint") for e in grn]
    o["fingerprints_differ"] = len([f for f in rf if f not in gf]) == len(rf) and len(red) >= 1 and len(grn) >= 1
    o["fingerprints_new"] = len([f for f in gf if f not in banned]) == len(grn)
    tr = token_rule(red, grn, all_entries, state_delta, bdir)
    notes = []
    if tr is not None:
        o["token_ok"], notes = tr
    o["pass"] = all(o[k] for k in o)
    return o, notes


def derive(entries, item, state_delta=False, bdir=None):
    allm = [e for e in entries if e.get("item") == item]
    cut, banned, counted, ignored = 0, [], [], []
    for o in [e for e in allm if e.get("polarity") == "REOPEN"]:
        w = [e for e in allm if e["seq"] > cut and e["seq"] < o["seq"] and e.get("polarity") != "REOPEN"]
        ref = next((e for e in w if e.get("polarity") in ("RED", "GREEN")), None)
        ck, _ = checks(w, banned, allm, state_delta, bdir)
        if ck["pass"] and genuine(o) and ref is not None and same(o, ref) and o.get("target_fingerprint") is not None:
            cut = o["seq"]
            counted.append(o["seq"])
            banned = banned + [e.get("target_fingerprint") for e in w if e.get("polarity") == "GREEN"] + [o["target_fingerprint"]]
        else:
            ignored.append(o["seq"])
    final = [e for e in allm if e["seq"] > cut and e.get("polarity") != "REOPEN"]
    ck, notes = checks(final, banned, allm, state_delta, bdir)
    out = {"item": item, "cycle_after_seq": cut, "reopens_counted": len(counted), "reopens_ignored": ignored}
    for k in CHECKS:
        out[k] = ck[k]
    if "token_ok" in ck:
        out["token_ok"] = ck["token_ok"]
        if notes:
            out["token_notes"] = notes
    out["verdict"] = "PASS" if ck["pass"] else "FAIL"
    return out


def main(argv):
    usage = "verdict ITEM [--ledger FILE] [--blobs DIR] [--chain-only] [--state-delta]"
    item = led = blobs = None
    chain_only = state_delta = False
    i = 0
    try:
        while i < len(argv):
            a = argv[i]
            if a == "--ledger" and led is None:
                led = argv[i + 1]; i += 2
            elif a == "--blobs" and blobs is None:
                blobs = argv[i + 1]; i += 2
            elif a == "--chain-only":
                chain_only = True; i += 1
            elif a == "--state-delta":
                state_delta = True; i += 1
            elif not a.startswith("--") and item is None:
                item = a; i += 1
            else:
                raise ValueError(a)
        if item is None:
            raise ValueError("ITEM")
    except (ValueError, IndexError) as e:
        sys.stderr.write("verdict: refused reason=usage_error %s; %s\n" % (e, usage))
        return 64
    default = led is None
    led = led or evcore.ledger_path()
    bdir = blobs or os.environ.get("EV_BLOBS") or (evcore.blobs_path() if default else os.path.join(os.path.dirname(os.path.abspath(led)), "blobs"))
    try:
        entries, _ = evcore.chain_walk(led, deep=not chain_only)
    except evcore.Refuse as r:
        sys.stderr.write("verdict: refused reason=%s %s (no verdict is derived from a ledger that does not verify)\n" % (r.reason, r.detail))
        return 3
    out = derive(entries, item, state_delta, bdir)
    print(json.dumps(out))
    return 0 if out["verdict"] == "PASS" else 1
