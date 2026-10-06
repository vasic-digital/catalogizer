# verdict, evverdict.py: verdict deriver - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T00:00:00Z |
| Status | tracked from WP-05 T054 and T051a; independent review owed (constitution 11.4.142, T058); the real-defect polarity-switch leg is OWED (see below) |
| Source | `tools/evidence/verdict`, `tools/evidence/evverdict.py` |

## Purpose
`verdict ITEM [--ledger FILE] [--blobs DIR] [--chain-only] [--state-delta]` DERIVES the verdict of a register item from the ledger by program (DR-E4, constitution 11.4.240, 11.4.249): never typed by an author. The ledger is walked first (chain, canonical form, ev/1 schema); a ledger that does not verify is UNVERIFIED (exit 3, named reason) and no verdict is derived. Output: one JSON line (`item`, `cycle_after_seq`, `reopens_counted`, `reopens_ignored`, the checks, `verdict`). Exit: 0 PASS, 1 FAIL, 3 UNVERIFIED, 64 usage.

## Rules (docs/06 section 4.2 step 7, 10, 13.1, 13.4)
`red_ok`, `green_ok` (three GREEN with three distinct iterations), `green_identical`, `same_test` (one argv, cwd, target class, target locator and test fingerprint), `red_before_green`, `fingerprints_differ`, `fingerprints_new`, and the cycle rule (a REOPEN cuts only a same-test genuine failure of a cycle that itself derived PASS; any other REOPEN is listed in `reopens_ignored`). The `--run-token VALUE` pair is removed from argv before the same-test comparison and masked in stdout before the identical-runs comparison (it differs per run by design).
`token_ok` (T051a): present only when the rule applies, which is with `--state-delta` OR whenever any RED/GREEN entry of the cycle carries `--run-token` (a caller cannot opt out by omitting the flag). Every GREEN must carry a token, the token must appear as a whole token in its OWN captured stdout, and no other entry may carry it. A post-state that carries the token of an earlier run is the stale-state bluff and fails here (`token_notes` says which entry).
`--chain-only` skips the per-entry ev/1 schema walk. It exists for the golden-bad fixtures the strict recorder REFUSES to write (a RED that passed, a GREEN that failed, a REOPEN that passed); a production caller never passes it, and the strict mode refuses such a ledger (`reason=schema_invalid`).

## Tests
`tools/evidence/tests/test_verdict.sh` with `verdict_cases/scenario.sh` (the docs/06 section 13.1 scenario ported to the real recorder; three cases are built with `tests/forge.py`, a test-only forger) and `verdict_cases/expected.tsv` (the 18 expected verdicts from docs/06 section 13.4: `good` and `second_cycle` PASS, the other 16 FAIL naming the failing check). The token cases are in `test_evrec.sh` (T051a).

## Stated limits and owed (11.4.6)
A MUTATION entry is not part of this derivation (the register seam of docs/06 section 17 step 5 checks the closing mutation); the oracle strength and the evidence class are not judged here. OWED: the polarity switch on one real register defect (docs/06 section 17 step 3, T054): it needs the findings register (T069) and the T038/T031 stale-tracking-ref case; if no P0 defect reproduces it is recorded in `$EV/p0-exit.json` and closed by D-04 in T109.
