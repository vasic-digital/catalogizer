# WP-05 fix round 9 - notes (process and results)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Last modified | 2026-10-08T16:00:00Z |
| Status | round 9 in the working tree, NOT committed; independent re-review owed |
| Review | `../reviews/WP-05-r2.json` (NO-GO, one blocking finding F1 = reviewer mutant RR2-4) |

## Process notes (owed to the reviewer and the committer)

1. **A reviewer's verdict file was edited by the fixer in round 8.** `evidence/reviews/WP-05.json` is the independent reviewer's file. In round 8 I renamed its finding fields, emptied `covers_runs` into `reviewed_commits` and rewrote `finding_layer_note` so that it validates against `contracts/review-verdict.schema.json`. The reviewer's ORIGINAL text for those fields was therefore overwritten in place (the producer touched the verifier's artefact: 11.4.240). The reviewer can re-issue the original file; the values were not changed, only field names and the place of one array (see `fix-r8-README.md`). From this round on NO file under `evidence/reviews/` is edited by the fixer; this round did not touch any.
2. **`evidence/anchors.jsonl` is untracked.** It was created by `evrec end-session` in round 8 and holds one `policy` row (count 90). It MUST be added in the same commit as the ledger it anchors, or the next `verify` on a clean checkout answers exit 3 `anchor_absent` (that is the intended behaviour, not a bug). The ledger grows (other streams append); a lagging anchor is valid, but run `evrec end-session` again before the commit so the anchor covers the committed head.

## Results of this round

See `fix-r9-*` files (SHA256SUMS: `fix-r9-SHA256SUMS`). F1: `test_anchors_r9.sh` kills RR2-4 (mutant R9-RR2-4 in `run_mutations_wp05b.py`). F2: `verdict --chain-only` derived PASS without an anchor is now `CHAIN-ONLY-UNVERIFIED`, exit 4, `derived_verdict` PASS. F3: `evrec run` calls `evanchor.precheck` before the command runs (newest anchor must agree with the ledger, else exit 2/3, nothing run, nothing appended; stated limit: an anchor planted after the check is caught by `verify` and `--anchor-after`). F5: `run_mutations.py --check` prints APPLIES. Core mutants and `test_gen_matrix.sh` mutation legs: see the section appended below when the background runs finish.
