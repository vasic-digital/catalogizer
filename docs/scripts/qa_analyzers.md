# vision analyzers - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:45:00Z |
| Status | new in the working tree (WP-24), not yet committed; independent review owed (constitution 11.4.142, T219); its row in `docs/scripts/README.md` is owed |
| Source | `tools/evidence/analyzers/` (T218): `pngio.py`, `frame_analyzer.py`, `ocr_analyzer.py`, `selftest.py`, `make_fixtures.py`, `fixtures/`; tests `tools/evidence/analyzers/tests/test_analyzers.py` |

## Purpose

Deterministic analyzers for recorded frames (doc12 section 9.3): a standard-library frame analyzer (blank frame, error overlay, frozen sequence, motion, colour oracle, blob counter) and an OCR text analyzer over the `tesseract` CLI. They are self-validated (11.4.107 (10)): trusted only after they FAIL the golden-bad recording and still PASS the golden-good and the negative control. The colour oracle is calibrated on `submodules/helix_qa/data/vision_gt/` (red circle seen, three blue circles counted, no red in the blue image). No model call: a verdict never depends on an LLM.

## Usage

```bash
python3 tools/evidence/analyzers/selftest.py --out evidence/qa/analyzers_selftest.json
scripts/test-in-container.sh tooling unit -- python3 /src/tools/evidence/analyzers/selftest.py --out /out/analyzers_selftest.json
```

Exit 0 every leg trusted; 4 frame and calibration trusted but the OCR leg BLOCKED (no tesseract: only IMG-QA carries it); 1 a leg untrusted; 2 fixtures missing.

## Honest boundary

The OCR leg cannot run until IMG-QA exists; it is reported blocked, never passed. The fixtures are PNG frame sequences made by `make_fixtures.py` (Pillow, a regeneration aid only); real recordings (video through ffmpeg) are exercised in IMG-QA. Fixture-root registration of `tools/evidence/analyzers/fixtures/` and the `local_allowlist.tsv` row of the self-test command are owed to the CPA change set (T040b, T121b).
