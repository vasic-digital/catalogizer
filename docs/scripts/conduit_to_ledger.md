# conduit_to_ledger.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:45:00Z |
| Status | new in the working tree (WP-24), not yet committed; independent review owed (constitution 11.4.142, T219); its row in `docs/scripts/README.md` is owed |
| Source | `scripts/qa/conduit_to_ledger.py` (T215); tests `scripts/qa/tests/test_conduit_to_ledger.py`; gate `scripts/qa/gates/no_skip_in_deterministic_lane.sh` |

## Purpose

Adapts a HelixQA conduit stream (`conduit.events.jsonl`, doc12 section 8.2) to `qa-verdict/1` ledger lines. SKIP makes a deterministic-lane run invalid; OPERATOR-BLOCKED needs a closed-set reason; `llm_call` / `vision_call` are kept out of the verdict chain with role `advisory` (11.4.269); the number of verdicts in the stream must equal the number of ledger entries written (recorder reconciliation: on a mismatch the ledger is restored and the run is invalid).

## Usage

```bash
python3 scripts/qa/conduit_to_ledger.py --stream FILE --ledger FILE --run ID [--lane deterministic|exploratory] [--target-fingerprint V] [--advisory-store FILE] [--evidence-root DIR] [--check-only]
scripts/qa/gates/no_skip_in_deterministic_lane.sh --stream FILE [--stream FILE ...]      # gate CM-QA-NO-SKIP-IN-DETERMINISTIC-LANE
```

Exit 0 ok, 2 usage / unreadable stream, 3 invalid run (nothing written). Invalid: SKIP in the deterministic lane, blocked without a closed-set reason, non-increasing `seq`, a line that is not JSON, an evidence file missing or empty, a verdict without a target fingerprint, a run id already in the ledger, a stream with no `challenge_verdict` (advisory events alone never certify anything).

## Honest boundary

The adapter feeds the document 06 recorder, it does not replace it. The pinned `helixqa run` does not itself wire the conduit writer (`cmd/helixqa/main.go` 884-905 is inside `cmdAutonomous`; `cmdRun` is 119-262; read statically, UNCONFIRMED at runtime): see `run_profile.md`.
