# census_ab_pass.py and lib/ab_pass_with_evidence.sh: TS-02 adoption - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T00:00:00Z |
| Status | tracked from WP-05 T052 (docs/05 TS-02); independent review owed (constitution 11.4.142, T058); the finding filing for any copy left behind is OWED on T069 |
| Source | `tools/evidence/census_ab_pass.py`, `tools/evidence/lib/ab_pass_with_evidence.sh`, the eleven suites under `scripts/testing/full_automation/` |

## Purpose
Eleven identical per-script copies of `ab_pass_with_evidence` could print `PASS` without any ev/1 entry. `lib/ab_pass_with_evidence.sh` is now the ONE definition: it keeps the evidence-exists-and-non-empty check and, before it prints `PASS`, records the check with `evrec run ITEM PROBE N remote_service <evidence> -- test -s <evidence>`. A recorder that is absent, refuses or fails is a `FAIL`, never a `PASS`. Environment: `EVREC_ITEM` (default `RUN-<pid>`), `EVREC_BIN`; the counters and line formats are the legacy ones.
Every suite sources it by its own location: `. "$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)/../../../tools/evidence/lib/ab_pass_with_evidence.sh"`.

## Census
`python3 -I tools/evidence/census_ab_pass.py --root DIR [--out FILE]` counts the name STRUCTURALLY: definitions (function definitions at code positions), calls, and carrier mentions (comments, strings, heredoc bodies, documents). Its control needle (a function-keyword form, a spaced form and three decoys) is run before it reports; a blind lexer exits 2. Excluded trees are listed in the output. Definitions under `fixtures/` are class `fixture`.

## Tests
`tools/evidence/tests/test_ts02.sh`: census needle and decoys, repository census (zero definitions under `scripts/testing/full_automation`, exactly one shipping definition, every suite sources it and still calls it), behaviour (golden-bad: the legacy copy `fixtures/ab_pass_legacy.sh` prints PASS and writes no entry; golden-good: one entry with the evidence digest; negative control: empty evidence; recorder unavailable; recorder refusing). The paired mutation (a restored local copy) is `$EV/wp05/ts02-mutation.txt`; the census and migration list are `$EV/wp05/ts02-census.json`.

## Stated limits (11.4.6)
The suites were not executed against a live catalog API here (none is available); the migration is verified by the census, `bash -n`, and the helper's behaviour tests. The `submodules/` trees carry their own copies in other repositories and are outside the census.
