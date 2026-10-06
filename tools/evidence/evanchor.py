"""evanchor - anchors for the evidence ledger (tasks.md T055; docs/06 s7, s8, s13.3; constitution 11.4.268 B/C/D).

An anchor is an append-only row of `$EV/anchors.jsonl`: {"schema":"ev-anchor/1","head":<entry_hash of entry COUNT>,"count":N,
"at":<UTC>,"strength":"policy"|"mechanism"[,"probe":{...}]}. It is the only thing that catches a delete-and-recompute forgery and a
tail truncation (the chain walk alone is satisfied by both, docs/06 s7).

  verify_anchors(path, entries)   every row is compared with the chain: the entry at seq COUNT must carry the anchored head, and the
                                  chain must hold at least COUNT entries. A chain LONGER than the newest anchor with an intact prefix is valid
                                  growth (a lagging anchor, the continuum negative control). Exit statuses through Refuse: 2 anchor
                                  disagreement / inconsistent rows, 3 UNVERIFIED (unreadable, malformed, `mechanism` without probe evidence).
  cmd_anchor(args)                `evrec anchor [--strength policy|mechanism] [--probe-remote DIR]`
  rerecord_anchor_leg(...)        the anchor leg of `evrec rerecord`

Strength (docs/06 s8, DR-E2): `policy` by default. `mechanism` is written only with evidence in the row (probe_kind=config): the
probe READS the configuration of a local bare repository (`git config --show-origin`) and requires receive.denyNonFastForwards AND
receive.denyDeletes both effectively true; executable pre-receive/update hooks are recorded as evidence only. No push of any kind is ever
made (11.4.113: no forced, plus-ref or lease-guarded push, no rewrite, scratch repositories included), and a network remote is never contacted. The
anchor location of the real feature (a remote with enforced fast-forward-only branches) is BLOCKED-ON OD-76: until it is answered the
anchors stay in `$EV/anchors.jsonl` and are `policy`. Stated limits (11.4.6): an anchor in the same directory as the ledger is rewritable
by whoever can rewrite the ledger (docs/06 s7 last row); verify says `policy` and nothing stronger.
"""
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import time

import evcore
from evcore import Refuse, canon

SCHEMA = "ev-anchor/1"
HEX64 = re.compile(r"[0-9a-f]{64}")
STRENGTHS = ("policy", "mechanism")
_KEYS = {"schema", "head", "count", "at", "strength", "probe"}


def now():
    return time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime())


def read_rows(path):
    """Rows of an anchors file. An unreadable file is UNVERIFIED `anchor_unreadable` (3); a row that is not a valid ev-anchor/1 row
    is UNVERIFIED `anchor_not_compared` (3); a `mechanism` row without probe evidence is UNVERIFIED `strength_unproven` (3)."""
    try:
        with open(path, "rb") as f:
            raw = f.read()
    except OSError as e:
        raise Refuse("anchor_unreadable", 3, "anchor %s cannot be read: %s: UNVERIFIED" % (path, e.strerror))
    rows = []
    for n, line in enumerate(raw.split(b"\n"), 1):
        if not line:
            continue
        try:
            r = json.loads(line.decode("utf-8"))
            ok = (isinstance(r, dict) and set(r) <= _KEYS and r.get("schema") == SCHEMA and isinstance(r.get("head"), str)
                  and HEX64.fullmatch(r["head"]) and isinstance(r.get("count"), int) and not isinstance(r["count"], bool)
                  and r["count"] >= 1 and isinstance(r.get("at"), str) and r["at"].endswith("Z") and r.get("strength") in STRENGTHS
                  and (r.get("probe") is None or isinstance(r["probe"], dict)))
        except (ValueError, UnicodeDecodeError):
            ok = False
        if not ok:
            raise Refuse("anchor_not_compared", 3, "anchor %s line %d is not a valid ev-anchor/1 row: the chain was verified but the anchor "
                         "was NOT compared: UNVERIFIED against the anchor" % (path, n))
        if r["strength"] == "mechanism" and not (isinstance(r.get("probe"), dict) and r["probe"].get("rewrite_rejected") is True
                                                 and r["probe"].get("probe_kind") == "config" and isinstance(r["probe"].get("remote"), str)):
            raise Refuse("strength_unproven", 3, "anchor %s line %d claims strength mechanism without config-probe evidence (probe_kind=config) that a rewrite is denied: "
                         "UNVERIFIED (the claim is not accepted; policy is the strength that can be shown)" % (path, n))
        rows.append(r)
    return rows


def verify_anchors(path, entries):
    """Compare every row with the chain. Returns (rows, newest_row). Raises Refuse 2 on disagreement, 3 on unverifiable."""
    rows = read_rows(path)
    if not rows:
        return rows, None
    prev, heads = 0, {}
    for i, r in enumerate(rows, 1):
        if r["count"] < prev:
            raise Refuse("anchor_inconsistent", 2, "anchor row %d has count %d below the previous row's %d: anchor rows are append-only history"
                         % (i, r["count"], prev))
        if heads.get(r["count"], r["head"]) != r["head"]:
            raise Refuse("anchor_inconsistent", 2, "two anchor rows hold count %d with different heads" % r["count"])
        heads[r["count"]], prev = r["head"], r["count"]
    n = len(entries)
    for r in reversed(rows):                                  # newest first: that is the row a reader needs named
        c = r["count"]
        if c > n:
            raise Refuse("anchor_disagrees", 2, "anchored count=%d head=%s but the ledger has actual count=%d head=%s: entries are missing "
                         "(deleted tail or truncation)" % (c, r["head"][:12], n, entries[-1]["entry_hash"][:12] if n else "-"))
        if entries[c - 1]["entry_hash"] != r["head"]:
            raise Refuse("anchor_disagrees", 2, "anchored count=%d head=%s but entry %d of the ledger has hash %s: the history before the "
                         "anchor was rewritten (edit, delete-and-recompute or reorder with the chain recomputed)"
                         % (c, r["head"][:12], c, entries[c - 1]["entry_hash"][:12]))
    return rows, rows[-1]


def describe(rows, newest):
    return "anchor agrees: %d row(s), newest count=%d head=%s strength=%s" % (len(rows), newest["count"], newest["head"][:12], newest["strength"])


# ----------------------------------------------------------------------------- the strength probe
# Constitution 11.4.113 forbids every forced, plus-ref or lease-guarded push and every history rewrite on ANY repository, scratch ones included:
# the probe therefore pushes NOTHING. It reads the local bare repository's own server-side configuration (read-only `git config`) and
# claims that a rewrite is rejected only when receive.denyNonFastForwards AND receive.denyDeletes are both effectively true. Hooks are
# recorded as evidence only: an executable hook proves nothing about what it does.

PROBE_KIND = "config"
DENY_KEYS = ("receive.denyNonFastForwards", "receive.denyDeletes")
HOOKS = ("pre-receive", "update")


def _git(args, cwd, env):
    return subprocess.run(["git"] + args, cwd=cwd, env=env, stdin=subprocess.DEVNULL, stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=60)


def _read_key(remote, key, env):
    """The effective boolean value (git's own resolution: last value wins, every config scope of the environment) and every value with its
    origin (`--show-origin --get-all`). Read-only."""
    allv = _git(["--git-dir=" + remote, "config", "--show-origin", "--get-all", key], remote, env)
    values = []
    if allv.returncode == 0:
        for line in allv.stdout.decode("utf-8", "replace").splitlines():
            origin, _, value = line.partition("\t")
            values.append({"origin": origin, "value": value})
    eff = _git(["--git-dir=" + remote, "config", "--type=bool", "--get", key], remote, env)
    effective = None
    if eff.returncode == 0:
        effective = eff.stdout.decode("utf-8", "replace").strip() == "true"
    elif eff.returncode == 1:
        effective = False                                    # key unset: git's default for both keys is false
    return {"effective": effective, "values": values}


def probe_rewrite(remote):
    """Read-only inspection of a LOCAL bare repository. Returns the probe record for the row (probe_kind=config). Never raises on an
    unreachable remote: nothing was shown to reject a rewrite, so the strength stays policy."""
    rec = {"remote": os.path.abspath(remote), "probe_kind": PROBE_KIND, "rewrite_rejected": None, "result": None, "probed_at": now()}
    if shutil.which("git") is None:
        rec["result"] = "git_absent"
        return rec
    if not os.path.isdir(remote):
        rec["result"] = "unreachable"
        return rec
    remote = os.path.abspath(remote)
    env = dict(os.environ)
    env["GIT_TERMINAL_PROMPT"] = "0"
    for k in [k for k in env if k in ("GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE")]:
        del env[k]
    try:
        bare = _git(["-C", remote, "rev-parse", "--is-bare-repository"], remote, env)
        if bare.returncode != 0 or bare.stdout.strip() != b"true":
            rec["result"] = "not_a_bare_repository"
            return rec
        cfg = {k: _read_key(remote, k, env) for k in DENY_KEYS}
        hooks = {}
        for h in HOOKS:
            hp = os.path.join(remote, "hooks", h)
            hooks[h] = "executable" if (os.path.isfile(hp) and os.access(hp, os.X_OK)) else ("present_not_executable" if os.path.isfile(hp) else "absent")
        rec["config"], rec["hooks"] = cfg, hooks
        if any(c["effective"] is None for c in cfg.values()):
            rec["result"] = "config_unreadable"
            return rec
        rejected = all(c["effective"] is True for c in cfg.values())
        rec["rewrite_rejected"] = rejected
        rec["result"] = "rewrite_rejected_by_config" if rejected else "rewrite_not_denied_by_config"
        return rec
    except (OSError, subprocess.SubprocessError) as e:
        rec["result"] = "probe_error:%s" % type(e).__name__
        return rec


# ----------------------------------------------------------------------------- evrec anchor

def cmd_anchor(args):
    usage = "evrec anchor [--strength policy|mechanism] [--probe-remote DIR]"
    want, remote, i = None, None, 0
    while i < len(args):
        a = args[i]
        if a == "--strength" and i + 1 < len(args) and want is None:
            want = args[i + 1]; i += 2
        elif a == "--probe-remote" and i + 1 < len(args) and remote is None:
            remote = args[i + 1]; i += 2
        else:
            raise Refuse("usage_error", 64, usage)
    if want is not None and want not in STRENGTHS:
        raise Refuse("usage_error", 64, "--strength must be policy or mechanism")
    probe = None
    if remote is not None:
        if re.match(r"^[A-Za-z][A-Za-z0-9+.-]*://", remote) or re.match(r"^[^/]+@[^/]+:", remote):
            raise Refuse("probe_remote_not_local", 64, "%s is a network remote: the probe never contacts one (it only reads the configuration of a local "
                         "bare repository, and no push is ever made, 11.4.113); name a local bare repository" % remote)
        probe = probe_rewrite(remote)
    strength = "mechanism" if (probe and probe["rewrite_rejected"] is True and want != "policy") else "policy"   # an explicit --strength policy is honoured
    if want == "mechanism" and strength != "mechanism":
        raise Refuse("strength_unproven", 71, "--strength mechanism needs a config probe that read both receive.denyNonFastForwards and "
                     "receive.denyDeletes as true (--probe-remote DIR); %s. Nothing written; the achievable strength here is policy (docs/06 s8, DR-E2)"
                     % ("the probe result was %s" % probe["result"] if probe else "no probe was given"))
    led, apath = evcore.ledger_path(), evcore.anchor_path()
    lock = evcore.LedgerLock(led, evcore.lock_timeout())
    lock.acquire()
    try:
        entries, head = evcore.chain_walk(led)                 # a ledger that does not verify is never anchored (raises 1 or 3)
        old = b""
        if os.path.exists(apath) and os.path.getsize(apath) > 0:
            verify_anchors(apath, entries)                      # the existing history must agree with the chain, else nothing is added
            with open(apath, "rb") as f:
                old = f.read()
        row = {"schema": SCHEMA, "head": head, "count": len(entries), "at": now(), "strength": strength}
        if probe:
            row["probe"] = probe
        rows = [json.loads(l) for l in old.split(b"\n") if l]
        if rows and rows[-1]["count"] == row["count"] and rows[-1]["head"] == row["head"]:
            print("anchor unchanged: count=%d head=%s already anchored (strength=%s)" % (row["count"], head[:12], rows[-1]["strength"]))
            return 0
        evcore.atomic_write(apath, old + canon(row) + b"\n")
    finally:
        lock.release()
    print("anchored count=%d head=%s strength=%s -> %s" % (row["count"], head[:12], strength, apath))
    return 0


# ----------------------------------------------------------------------------- rerecord anchor leg

def rerecord_anchor_leg(onto_path, local_path, onto_anchors, local_anchors, cp, new_entries_count, new_head, old_suffix_seqs):
    """Returns (anchors_bytes, seqmap_entry). Both input anchor files must verify against their own ledger side (else side_unverifiable 68).
    Output rows: the remote side's rows verbatim, then the local rows that the remote rows do not already pin (remote newest count < count
    <= common prefix), then ONE anchor over the re-chained head when the local side had anchors after the common prefix."""
    sides = {}
    for tag, lp, ap in (("onto", onto_path, onto_anchors), ("local", local_path, local_anchors)):
        try:
            entries, _ = evcore.chain_walk(lp)
            rows, _ = verify_anchors(ap, entries)
        except Refuse as e:
            raise Refuse("side_unverifiable", 68, "%s side anchors rejected: %s %s" % (tag, e.reason, e.detail))
        sides[tag] = rows
    remote_rows, local_rows = sides["onto"], sides["local"]
    rmax = max([r["count"] for r in remote_rows], default=0)
    kept = [r for r in local_rows if rmax < r["count"] <= cp]
    replaced = [r for r in local_rows if r["count"] > cp]
    out = list(remote_rows) + kept
    seqentry = None
    if replaced:
        row = {"schema": SCHEMA, "head": new_head, "count": new_entries_count, "at": now(), "strength": "policy"}
        out.append(row)
        seqentry = {"kind": "anchor", "old_counts": [r["count"] for r in replaced], "new_count": new_entries_count, "digest": new_head}
    return b"".join(canon(r) + b"\n" for r in out), seqentry
