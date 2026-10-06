# run_profile.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:45:00Z |
| Status | new in the working tree (WP-24), not yet committed; independent review owed (constitution 11.4.142, T219); its row in `docs/scripts/README.md` is owed |
| Source | `scripts/qa/run_profile.sh` (T217) with `manifest_resolve.py` and `canonical_result.py`; tests `scripts/qa/tests/test_run_profile.sh`; gate `scripts/qa/gates/qa_blocked_not_pass.sh` |

## Purpose

The one QA execution wrapper (doc12 section 8.1), run inside IMG-QA from the read-only source mount: resolves a profile from `challenges/helixqa-banks/MANIFEST.yaml`, refuses below the floor, runs the availability probes first (a blocked probe means the runner never starts), runs `helixqa run` three times, adapts the conduit events (T215), computes the canonical result hash per run (doc12 section 8.3) and exits non-zero on any fail or blocked. The runner exit status is captured in a variable, never through a pipe (QF-05).

## Usage

```bash
scripts/qa/run_profile.sh --profile api|web|desktop|android|androidtv|installer [--manifest F] [--banks-dir D] [--out DIR] [--helixqa CMD] [--probes-dir D] [--runs N] [--fingerprint V]
```

Exit 0 all pass and one canonical hash; 1 a `fail` verdict; 2 usage; 3 REFUSED before running (`profile_unknown`, `manifest_image_unset`, `below_floor`, `bank_missing`, `no_bank_for_profile`); 4 blocked (a probe or a verdict); 5 invalid run (SKIP in the deterministic lane, no conduit stream, missing evidence or fingerprint); 6 nondeterministic (canonical hashes differ); 7 the runner crashed. Outputs: `probes.jsonl` first, `run-<i>/`, `ledger-<i>.jsonl`, `canonical-run-<i>.json`, `summary.json`.

## Honest boundary

The manifest records no IMG-QA digest yet (T211 blocked), so every real profile is refused `manifest_image_unset`. The pinned `helixqa run` does not wire the conduit writer (`cmdAutonomous` only): a run with no stream is exit 5, never a fabricated verdict; the upstream change belongs to WP-Q3. Paired mutations observed failing: runner status through a pipe, blocked treated as pass, floor check removed, probes not first, hash comparison removed, deterministic lane switched to exploratory.
