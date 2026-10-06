#!/usr/bin/env python3
"""conduit_to_ledger.py - T215 (doc12 8.2). Adapts a HelixQA conduit JSONL stream to `qa-verdict/1` ledger lines.

Usage: conduit_to_ledger.py --stream FILE --ledger FILE --run ID [--lane deterministic|exploratory] [--target-fingerprint VALUE]
                            [--advisory-store FILE] [--evidence-root DIR] [--check-only]
  --stream             the conduit.events.jsonl of a run (append-only, monotonic `seq`, closed event types, doc12 8.2)
  --ledger             the ledger the verdict lines are appended to (lines of other runs are kept)
  --run                the run id written into every line
  --lane               deterministic (default): SKIP makes the run INVALID; exploratory: SKIP is accepted and recorded as verdict `skip`
  --target-fingerprint the fingerprint read FROM the target at run time (11.4.115 F): stamped on every verdict unless the event carries its own
                       `fields.target_fingerprint`; a verdict without a fingerprint makes the run invalid
  --advisory-store     where llm_call / vision_call events go (default <ledger>.advisory.jsonl), each with role `advisory` (11.4.269): never in the ledger
  --evidence-root      directory the relative `evidence_path` of evidence_captured events resolve against (default: the stream's directory); the file
                       must exist and be non-empty (the stream's own anti-bluff rule)
  --check-only         validate and report, write nothing (the gate CM-QA-NO-SKIP-IN-DETERMINISTIC-LANE uses it)
Mapping: challenge_verdict PASS -> pass, FAIL -> fail, OPERATOR-BLOCKED + a closed-set `reason` -> blocked, SKIP -> invalid in the deterministic lane.
Invalid-run conditions (nothing is written; exit 3, the reason on stderr): SKIP in the deterministic lane; OPERATOR-BLOCKED without a closed-set reason; a
non-increasing `seq`; a line that is not a JSON object; an evidence file that is missing or empty; a verdict without a fingerprint; no challenge_verdict at all
(a stream with advisory events only never certifies anything); verdict count != ledger entry count after the write (recorder reconciliation: the file is
restored to its prior content). Exits: 0 ok, 2 usage / unreadable stream, 3 invalid run.
"""
import argparse
import json
import os
import sys

MAP = {"PASS": "pass", "FAIL": "fail", "OPERATOR-BLOCKED": "blocked"}
REASONS = {"service_unreachable", "credential_absent", "credential_rejected", "device_absent", "device_wrong_identity",
           "device_unauthorised", "geo_restricted", "quota_exhausted", "licence_absent", "host_resource_unavailable"}
ADVISORY_TYPES = ("llm_call", "vision_call")


class InvalidRun(Exception):
    pass


def reconcile(verdicts_in_stream, entries_in_ledger):
    """The recorder reconciliation rule: the number of verdicts the stream holds equals the number of ledger entries written for the run."""
    if verdicts_in_stream != entries_in_ledger:
        raise InvalidRun("recorder reconciliation failed: %d verdict(s) in the stream, %d ledger entr(ies) for the run" % (verdicts_in_stream, entries_in_ledger))


def count_run_entries(ledger, run):
    n = 0
    if os.path.exists(ledger):
        with open(ledger) as fh:
            for line in fh:
                try:
                    if json.loads(line).get("run") == run:
                        n += 1
                except ValueError:
                    continue
    return n


def run_exists(ledger, run):
    return _has_run(ledger, run)


def _has_run(ledger, run):
    if not os.path.exists(ledger):
        return False
    with open(ledger) as fh:
        for line in fh:
            try:
                if json.loads(line).get("run") == run:
                    return True
            except ValueError:
                continue
    return False


def parse(stream, lane, fingerprint, evidence_root):
    """Validate the stream; returns (ledger_records, advisory_records). Raises InvalidRun."""
    records, advisory, last_seq = [], [], None
    try:
        fh = open(stream)
    except OSError as e:
        raise OSError("stream unreadable: %s" % e)
    with fh:
        for lineno, line in enumerate(fh, 1):
            if not line.strip():
                continue
            try:
                e = json.loads(line)
            except ValueError:
                raise InvalidRun("line %d is not JSON" % lineno)
            if not isinstance(e, dict):
                raise InvalidRun("line %d is not a JSON object" % lineno)
            seq = e.get("seq")
            if not isinstance(seq, int) or (last_seq is not None and seq <= last_seq):
                raise InvalidRun("line %d: seq %r is not strictly increasing (previous %r)" % (lineno, seq, last_seq))
            last_seq = seq
            typ = e.get("type")
            if typ in ADVISORY_TYPES:
                advisory.append({"schema": "qa-advisory/1", "role": "advisory", "type": typ, "challenge": e.get("challenge"),
                                 "conduit_seq": seq, "fields": e.get("fields") or {}})
                continue
            if typ == "evidence_captured":
                path = e.get("evidence_path") or ""
                full = path if os.path.isabs(path) else os.path.join(evidence_root, path)
                if not path or not os.path.isfile(full) or os.path.getsize(full) == 0:
                    raise InvalidRun("evidence_captured line %d: evidence file %r is missing or empty" % (lineno, path))
                continue
            if typ != "challenge_verdict":
                continue
            ch = e.get("challenge") or "?"
            v = e.get("verdict")
            if v == "SKIP":
                if lane == "deterministic":
                    raise InvalidRun("invalid run: SKIP is not accepted in the deterministic lane: %s" % ch)
                verdict = "skip"
            elif v == "OPERATOR-BLOCKED":
                if e.get("reason") not in REASONS:
                    raise InvalidRun("blocked without a closed-set reason: %s (reason %r)" % (ch, e.get("reason")))
                verdict = "blocked"
            elif v in MAP:
                verdict = MAP[v]
            else:
                raise InvalidRun("challenge %s has an unknown verdict %r" % (ch, v))
            fp = (e.get("fields") or {}).get("target_fingerprint") or fingerprint
            if not fp:
                raise InvalidRun("verdict for %s carries no target fingerprint (11.4.115 F)" % ch)
            records.append({"schema": "qa-verdict/1", "lane": lane, "challenge": ch, "verdict": verdict, "reason": e.get("reason"),
                            "conduit_seq": seq, "target_fingerprint": fp})
    if not records:
        raise InvalidRun("no challenge_verdict event in the stream (zero verdicts is a blind run, never a pass)")
    return records, advisory


def adapt(stream, ledger, run, lane, fingerprint, advisory_store, evidence_root, check_only=False):
    records, advisory = parse(stream, lane, fingerprint, evidence_root)
    for r in records:
        r["run"] = run
    for a in advisory:
        a["run"] = run
    if check_only:
        return records, advisory
    prior = open(ledger).read() if os.path.exists(ledger) else None
    if run_exists(ledger, run):
        raise InvalidRun("run id %r already has entries in the ledger (a run id is used once)" % run)
    os.makedirs(os.path.dirname(os.path.abspath(ledger)), exist_ok=True)
    with open(ledger, "a") as out:
        for r in records:
            out.write(json.dumps(r, sort_keys=True) + "\n")
    try:
        reconcile(len(records), count_run_entries(ledger, run))
    except InvalidRun:
        if prior is None:
            os.remove(ledger)
        else:
            with open(ledger, "w") as out:
                out.write(prior)
        raise
    if advisory:
        store = advisory_store or ledger + ".advisory.jsonl"
        with open(store, "a") as out:
            for a in advisory:
                out.write(json.dumps(a, sort_keys=True) + "\n")
    return records, advisory


def main(argv=None):
    ap = argparse.ArgumentParser(description="conduit stream to qa-verdict ledger adapter")
    ap.add_argument("--stream", required=True)
    ap.add_argument("--ledger", required=True)
    ap.add_argument("--run", required=True)
    ap.add_argument("--lane", default="deterministic", choices=("deterministic", "exploratory"))
    ap.add_argument("--target-fingerprint")
    ap.add_argument("--advisory-store")
    ap.add_argument("--evidence-root")
    ap.add_argument("--check-only", action="store_true")
    try:
        ns = ap.parse_args(argv)
    except SystemExit as e:
        sys.exit(2 if e.code not in (0, None) else 0)
    root = ns.evidence_root or os.path.dirname(os.path.abspath(ns.stream))
    try:
        records, advisory = adapt(ns.stream, ns.ledger, ns.run, ns.lane, ns.target_fingerprint, ns.advisory_store, root, ns.check_only)
    except InvalidRun as e:
        sys.stderr.write("conduit_to_ledger: %s\n" % e)
        sys.exit(3)
    except OSError as e:
        sys.stderr.write("conduit_to_ledger: %s\n" % e)
        sys.exit(2)
    print("conduit_to_ledger: %d verdict(s)%s, %d advisory event(s) kept out of the chain" % (len(records), " (check only)" if ns.check_only else " written", len(advisory)))


if __name__ == "__main__":
    main()
