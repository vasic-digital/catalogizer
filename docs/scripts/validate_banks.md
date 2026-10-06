# validate_banks.py - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:45:00Z |
| Status | new in the working tree (WP-24), not yet committed; independent review owed (constitution 11.4.142, T219); its row in `docs/scripts/README.md` is owed |
| Source | `scripts/qa/validate_banks.py` (T210); tests `scripts/qa/tests/test_validate_banks.py` and fixtures `scripts/qa/tests/fixtures/banks/` (T209) |

## Purpose

The bank validator of doc12 section 6.6: rules R-1..R-8 over the HelixQA bank files, run before any bank executes. R-1 every step has a machine assertion; R-2 no TODO/CONVERT/placeholder in an action; R-3 no step skipped for a bank reason; R-4 `skipped` is no allowed final status; R-5 credentials only as `${ENV_NAME}` (a literal under a credential key, a Bearer/JWT/AKIA literal, `password: x` text, a typed default account); R-6 `metadata.v3.needs` complete (every `${ENV}` and `requires_env` name in `needs.credential_env`, an http/navigate step needs `needs.service`, a tap/keypress/text/adb step needs `needs.device`); R-7 `metadata.v3.oracle` exists, is independent of the system under test and its strategy is in the closed set; R-8 a coordinate tap needs an `ocr_assert` or `pixel_assert` and is never the fixed centre 960,540.

## Usage

```bash
python3 scripts/qa/validate_banks.py --banks PATH [PATH ...] [--cases FILE] [--json FILE | --json-stdout] [--counts-tsv FILE]
scripts/test-in-container.sh tooling unit -- python3 /src/scripts/qa/validate_banks.py --banks /src/challenges/helixqa-banks
```

- `--banks` a bank file or a directory (`*.yaml`, `*.yml`; `MANIFEST.yaml` is not a bank; a `.json` twin is listed under `unscanned`, never merged).
- `--cases FILE` one case id per line: R-1..R-8 are evaluated only for the listed cases; a violation in an unlisted case is not reported; a listed id that no bank holds is refused (exit 2), never ignored (the WP-60 W1 acceptance is scoped with it).
- `--counts-tsv FILE` the `bank<TAB>rule<TAB>count` ratchet baseline format.
- Exit: 0 clean, 1 at least one violation (a bank that does not parse is rule `PARSE`: a blind bank is never clean), 2 usage / unreadable input / no bank / unknown listed case id.

## Honest boundary (11.4.6)

It reads structure, never behaviour: a bank that passes proves it is well formed under the rules, not that a case would catch a defect (that is the paired mutation of doc12 section 8.5). `vision_verify` is not a machine assertion (a model call is advisory, 11.4.269). R-5 treats a literal in a step marked `negative: true` as a deliberately wrong value (the doc12 section 6.5 pattern); a deliberately malformed Bearer value in an un-marked step is reported and may be a false positive to be marked `negative`.

Measured over `challenges/helixqa-banks/` at HEAD f04c555d: 15 banks, 1,269 cases, 5,011 findings: R-1 1906, R-2 1178, R-5 18, R-6 405, R-7 1269, R-8 235 (234 of them the fixed centre tap). Evidence: `specs/001-full-project-audit-remediation/evidence/qa/validator_baseline.json`.
