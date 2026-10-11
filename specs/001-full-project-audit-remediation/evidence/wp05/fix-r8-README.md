# WP-05 fix round 8 - evidence for the independent review NO-GO (WP-05.json, R1 to R9)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Last modified | 2026-10-08T14:30:00Z |
| Status | fix round complete in the working tree, NOT committed; independent re-review owed (constitution 11.4.142, 11.4.134) |
| Review | `../reviews/WP-05.json` (NO-GO, 6 blocking, model opus, effort ?) |

Method (11.4.224): failing test and captured RED first (`fix-r8-red-*.txt`), then the fix, then three GREEN runs (`fix-r8-green-*-run{1,2,3}.txt`), then the paired mutations observed failing (`fix-r8-mutations-*.txt`). Every file is listed in `SHA256SUMS`.

| Finding | Fix (files) | Test (RED then GREEN) | Mutant observed failing |
|---|---|---|---|
| R1 verify exit 0 without an anchor | `evcore.py` `anchor_gate` (3 `anchor_absent` for absent/empty anchors; `verify --chain-only` is the explicit opt-in, prints CHAIN-ONLY, never OK, never switches off a present anchor); `test_anchors.sh` lines that asserted exit 0 now assert 3 | `test_anchors_r8.sh` R1, `test_anchors.sh` | R8-A24, R8-A25, T056-A1 |
| R2 live ledger unanchored | `evrec end-session`, `evrec run --anchor-after`; the live ledger was anchored through the tool (`fix-r8-live-anchor.txt`: count 90, policy, verify exit 0) | `test_anchors_r8.sh` R2 | R8-A27, R8-A28 |
| R3 mechanism for a producer-reconfigurable remote | `evanchor.py`: strength is always `policy`; `--strength mechanism` refused 71; probe row records `producer_can_reconfigure` and `anchor_location=local_file` | `test_anchors_r8.sh` R3, `test_anchors.sh` probe cases | R8-A29, A10, A11 |
| R4 verify trusts a mechanism row | `evanchor.read_rows` rejects every `mechanism` row (3 `strength_unproven`) | `test_anchors_r8.sh` R4 | R8-A26 |
| R5 deriver ignores anchors | `evverdict.py` calls `evcore.anchor_gate`; output carries `anchor`; same class: `matrix/gen_matrix.py` `ledger_context` (exit 3 `ledger_anchor_invalid`) | `test_anchors_r8.sh` R5, `test_gen_matrix.sh` (2 new cases) | R8-D13, R8-D14 |
| R6 harness vacuous catches | `run_mutations_wp05b.py` tree holds the guide, the verdict schema and WP-05.json; both harnesses start with the unmutated-baseline control (exit 2 BASELINE NOT GREEN); A17, A18, A18b re-run CAUGHT; A5, A13, A19 (retired claims) removed, A10, A11 rewritten | `test_mutation_harness.sh` | A17, A18, A18b CAUGHT with the baseline green (`fix-r8-mutations-wp05b.txt`); the control also stopped the first full run of this round with BASELINE NOT GREEN because the tree lacked the verdict file, which was then added |
| R7 five surviving mutants | tests in `test_anchors_r8.sh`: REV-1, REV-3, REV-4, REV-5, REV-6 | same | R8-A20, A21, D12, A22, A23 all CAUGHT |
| R8 schema wording and constraint | `contracts/evidence-record.schema.json` (`test_state_gos` items pattern, wording), `contracts/README.md` revision 30; `evidence/reviews/WP-05.json` now validates against `contracts/review-verdict.schema.json` (`finding_layer` = source/artifact/runtime, `finding_class` = 11.4.235(D) class, `covers_runs` moved to `reviewed_commits` because the commit has no CPA-Run trailer) | `test_anchors_r8.sh` R8 | n/a (contract) |
| R9 lead | not a code defect: 5 of 8 live items derive FAIL (AUD-T162, T168, T169, T179, T180); their owners must not cite the ledger as PASS | | |

Docs touched (revision headers bumped): `docs/scripts/evidence-recorder.md` (rev 3), `docs/scripts/evidence-verdict.md` (rev 2), `specs/001-full-project-audit-remediation/docs/06-determinism-and-evidence-framework.md` (rev 30, section 8), `contracts/README.md` (rev 30).

Stated limits (11.4.6): the first RED of `test_mutation_harness.sh` ran the OLD harness against the NEW sources (the old mutant text no longer matched), so its INVALID lines are an artefact; the reviewer's own run shows the original false baseline. `fix-r8-red-test_gen_matrix.txt` was stopped before its mutation legs finished. `run_mutations.py` (the core harness) was run on the seven mutants that touch verify only (`fix-r8-mutations-core-subset.txt`, 7 of 7 caught, baseline green); its other 234 mutants were not re-run, and 4 pre-existing INVALID mutants (N-I4g, N-I7e, N-I7f, W3-7) stay INVALID. Anchor strength `mechanism` stays BLOCKED-ON OD-76.
