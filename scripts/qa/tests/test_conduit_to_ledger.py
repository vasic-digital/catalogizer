"""T215 - tests for scripts/qa/conduit_to_ledger.py (doc12 8.2). Run through `TIC tooling unit` (IMG-TESTUTIL).

The adapter reads a HelixQA conduit JSONL stream and writes one `qa-verdict/1` ledger line per challenge_verdict; the deterministic lane refuses SKIP,
OPERATOR-BLOCKED needs a closed-set reason, llm_call/vision_call are advisory and stay out of the verdict chain (11.4.269), and the number of verdicts
the stream holds must equal the number of ledger entries written (recorder reconciliation).
"""
import importlib.util
import json
import os
import subprocess
import sys

import pytest

HERE = os.path.dirname(os.path.abspath(__file__))
QA = os.path.dirname(HERE)
ADAPTER = os.path.join(QA, "conduit_to_ledger.py")
FP = "sha256:" + "ab" * 32


def ev(seq, typ, **kw):
    d = {"seq": seq, "type": typ, "session": "s1"}
    d.update(kw)
    return d


def write_stream(path, events):
    with open(path, "w") as fh:
        for e in events:
            fh.write((e if isinstance(e, str) else json.dumps(e)) + "\n")


def run(stream, ledger, *extra):
    return subprocess.run([sys.executable, "-I", ADAPTER, "--stream", str(stream), "--ledger", str(ledger), "--run", "r1",
                           "--target-fingerprint", FP, *extra], capture_output=True, text=True)


def ledger_lines(p):
    return [json.loads(x) for x in open(p).read().splitlines()] if os.path.exists(p) else []


def good_events():
    return [ev(1, "challenge_start", challenge="c1"),
            ev(2, "challenge_verdict", challenge="c1", verdict="PASS"),
            ev(3, "challenge_verdict", challenge="c2", verdict="FAIL")]


def test_adapter_exists():
    assert os.path.isfile(ADAPTER), "scripts/qa/conduit_to_ledger.py is absent (T215 not implemented)"


def test_valid_stream_one_ledger_line_per_verdict(tmp_path):
    s, l = tmp_path / "c.jsonl", tmp_path / "ledger.jsonl"
    write_stream(s, good_events())
    p = run(s, l)
    assert p.returncode == 0, p.stderr
    rows = ledger_lines(l)
    assert [(r["challenge"], r["verdict"], r["conduit_seq"]) for r in rows] == [("c1", "pass", 2), ("c2", "fail", 3)]
    assert all(r["schema"] == "qa-verdict/1" and r["run"] == "r1" and r["target_fingerprint"] == FP for r in rows)


def test_skip_in_deterministic_lane_makes_run_invalid_and_writes_nothing(tmp_path):
    s, l = tmp_path / "c.jsonl", tmp_path / "ledger.jsonl"
    write_stream(s, good_events() + [ev(4, "challenge_verdict", challenge="c3", verdict="SKIP", reason="not applicable")])
    p = run(s, l)
    assert p.returncode == 3, p.stderr
    assert "SKIP is not accepted in the deterministic lane" in p.stderr and "c3" in p.stderr
    assert ledger_lines(l) == []


def test_skip_accepted_in_exploratory_lane(tmp_path):
    s, l = tmp_path / "c.jsonl", tmp_path / "ledger.jsonl"
    write_stream(s, [ev(1, "challenge_verdict", challenge="c3", verdict="SKIP", reason="x")])
    p = run(s, l, "--lane", "exploratory")
    assert p.returncode == 0, p.stderr
    assert ledger_lines(l)[0]["verdict"] == "skip" and ledger_lines(l)[0]["lane"] == "exploratory"


@pytest.mark.parametrize("reason", [None, "", "because", "SERVICE_UNREACHABLE"])
def test_blocked_without_closed_set_reason_fails(tmp_path, reason):
    s, l = tmp_path / "c.jsonl", tmp_path / "ledger.jsonl"
    e = ev(1, "challenge_verdict", challenge="c1", verdict="OPERATOR-BLOCKED")
    if reason is not None:
        e["reason"] = reason
    write_stream(s, [e])
    p = run(s, l)
    assert p.returncode == 3 and "closed-set reason" in p.stderr
    assert ledger_lines(l) == []


def test_blocked_with_closed_set_reason_is_blocked(tmp_path):
    s, l = tmp_path / "c.jsonl", tmp_path / "ledger.jsonl"
    write_stream(s, [ev(1, "challenge_verdict", challenge="c1", verdict="OPERATOR-BLOCKED", reason="credential_absent")])
    p = run(s, l)
    assert p.returncode == 0, p.stderr
    r = ledger_lines(l)[0]
    assert r["verdict"] == "blocked" and r["reason"] == "credential_absent"


def test_llm_and_vision_calls_are_advisory_and_outside_the_chain(tmp_path):
    s, l, a = tmp_path / "c.jsonl", tmp_path / "ledger.jsonl", tmp_path / "adv.jsonl"
    write_stream(s, [ev(1, "llm_call", challenge="c1", fields={"model": "m", "tokens": 5}),
                     ev(2, "vision_call", challenge="c1", fields={"model": "v"}),
                     ev(3, "challenge_verdict", challenge="c1", verdict="PASS")])
    p = run(s, l, "--advisory-store", str(a))
    assert p.returncode == 0, p.stderr
    assert len(ledger_lines(l)) == 1 and "advisory" not in json.dumps(ledger_lines(l))
    adv = ledger_lines(a)
    assert [x["type"] for x in adv] == ["llm_call", "vision_call"] and all(x["role"] == "advisory" for x in adv)


def test_pass_never_derived_from_advisory_only(tmp_path):
    """An llm_call that says PASS must not produce a verdict: with no challenge_verdict event the run is invalid (zero verdicts)."""
    s, l = tmp_path / "c.jsonl", tmp_path / "ledger.jsonl"
    write_stream(s, [ev(1, "llm_call", challenge="c1", verdict="PASS", fields={"model": "m"})])
    p = run(s, l)
    assert p.returncode == 3 and "no challenge_verdict" in p.stderr
    assert ledger_lines(l) == []


def test_reconciliation_mismatch_is_invalid():
    spec = importlib.util.spec_from_file_location("c2l", ADAPTER)
    m = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(m)
    m.reconcile(2, 2)
    with pytest.raises(m.InvalidRun):
        m.reconcile(3, 2)


def test_reconciliation_counts_what_was_written(tmp_path, monkeypatch):
    """Drop a line from the ledger between write and re-read: the adapter must notice (verdict count != ledger entry count)."""
    spec = importlib.util.spec_from_file_location("c2l_b", ADAPTER)
    m = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(m)
    s, l = tmp_path / "c.jsonl", tmp_path / "ledger.jsonl"
    write_stream(s, good_events())
    real = m.count_run_entries
    monkeypatch.setattr(m, "count_run_entries", lambda path, run: real(path, run) - 1)
    with pytest.raises(m.InvalidRun):
        m.adapt(str(s), str(l), run="r1", lane="deterministic", fingerprint=FP, advisory_store=None, evidence_root=str(tmp_path))


def test_non_monotonic_seq_is_invalid(tmp_path):
    s, l = tmp_path / "c.jsonl", tmp_path / "ledger.jsonl"
    write_stream(s, [ev(2, "challenge_verdict", challenge="c1", verdict="PASS"), ev(1, "challenge_verdict", challenge="c2", verdict="PASS")])
    p = run(s, l)
    assert p.returncode == 3 and "seq" in p.stderr


def test_missing_or_empty_evidence_file_is_invalid(tmp_path):
    s, l = tmp_path / "c.jsonl", tmp_path / "ledger.jsonl"
    write_stream(s, [ev(1, "evidence_captured", challenge="c1", evidence_path="shot.png", evidence_kind="screenshot"),
                     ev(2, "challenge_verdict", challenge="c1", verdict="PASS")])
    assert run(s, l).returncode == 3
    (tmp_path / "shot.png").write_bytes(b"")
    assert run(s, l).returncode == 3
    (tmp_path / "shot.png").write_bytes(b"x")
    assert run(s, l).returncode == 0


def test_missing_fingerprint_is_invalid(tmp_path):
    s, l = tmp_path / "c.jsonl", tmp_path / "ledger.jsonl"
    write_stream(s, [ev(1, "challenge_verdict", challenge="c1", verdict="PASS")])
    p = subprocess.run([sys.executable, "-I", ADAPTER, "--stream", str(s), "--ledger", str(l), "--run", "r1"], capture_output=True, text=True)
    assert p.returncode == 3 and "fingerprint" in p.stderr


def test_malformed_line_is_invalid(tmp_path):
    s, l = tmp_path / "c.jsonl", tmp_path / "ledger.jsonl"
    write_stream(s, ["{not json", ev(2, "challenge_verdict", challenge="c1", verdict="PASS")])
    assert run(s, l).returncode == 3


def test_check_only_writes_nothing(tmp_path):
    s, l = tmp_path / "c.jsonl", tmp_path / "ledger.jsonl"
    write_stream(s, good_events())
    p = run(s, l, "--check-only")
    assert p.returncode == 0 and not os.path.exists(l)


def test_existing_ledger_lines_of_other_runs_are_kept(tmp_path):
    s, l = tmp_path / "c.jsonl", tmp_path / "ledger.jsonl"
    l.write_text(json.dumps({"schema": "qa-verdict/1", "run": "other", "challenge": "z", "verdict": "pass"}) + "\n")
    write_stream(s, good_events())
    assert run(s, l).returncode == 0
    rows = ledger_lines(l)
    assert len(rows) == 3 and rows[0]["run"] == "other"
