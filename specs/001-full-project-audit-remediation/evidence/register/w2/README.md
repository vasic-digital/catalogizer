# W2 register evidence (T166, T170, T171, T172, T174)

# identity: task=W2 utc=2026-10-12T11:54:04Z uid=1000 git_head=ff62c1dfc18cf1415d501e105ec8d2ac808d1ed0 (the tools and tests of W2 are uncommitted working-tree files at this head)

FIXTURE evidence: every record here comes from the fixture trees of `scripts/register/tests/` (never the real register `docs/workable_items.db`, never the real frozen snapshot).
The real-snapshot records (`root-items.json`, `families.json`, `platform_inference.json`, the real `recount.json`, the S-26 / S-27 reconciliations) wait for the T161 freeze
(conductor-owned) and are UNCONFIRMED until then.

| File | What it is |
|---|---|
| `ledger-<task>-red1.txt`, `ledger-<task>-green1..3.txt` | ledger entries (`tools/evidence/evrec`, items AUD-T166 / T170 / T171 / T172 / T174 / AUD-W2-LISTING) with the stored stdout of each IMG-TESTUTIL container run: RED = the suite against a SUT directory without the tool (T166: the pre-fix `recount_sources.py` of git HEAD), GREEN x3 on a different target fingerprint; each transcript starts with an identity header |
| `mutation-<tool>.tsv` | mutation results per tool: N00 PASSED, C00 SURVIVED (comment-only), every mutant KILLED except the argued equivalent in `scripts/register/tests/equivalent_w2_mutants.tsv` (seeds_import M17); host-side runs (the mutation container leg is UNCONFIRMED) |
| `samples/` | the records the tools write, on the fixture: `t172-qa/qa_causes.json`, `t172-families/{families,platform_inference}.json`, the entry-set before/after exports and verdicts, `root-items/root-items.json`, `recount/recount.json`, the S-26 / S-27 reconciliations |
| `../t172-entryset-red.txt`, `../t172-entryset.json`, `../t172-entryset-mutation.txt`, `../recount-red.txt` | the files named by the T172 and T166 task text |

Verify: `cd specs/001-full-project-audit-remediation/evidence/register/w2 && sha256sum -c SHA256SUMS` (paths relative to this directory).

## Review fix round 2 (independent review of 2026-10-12, NO-GO 0/6/3, findings F1-F9) - `fix-r2-` files

Resumed after a forced host power-off killed the first fixer; the state was rebuilt from disk, every finding re-verified against the tree, and the open parts closed (F3 oracle, F1 text, F7 S-26/S-27 records). Identity of these runs: git head `ff62c1df` plus the uncommitted W2 working-tree files.

| File | What it is |
|---|---|
| `fix-r2-convergence.md` | the 11.4.276 convergence assessment: defect classes, control-needled member inventories, honest boundary |
| `ledger-fix-r2-<task>-red1.txt`, `ledger-fix-r2-<task>-green1..3.txt` | ledger entries with the stored stdout of each IMG-TESTUTIL container run (`run_pinned.sh`, one container at a time): RED = the NEW suite against the PRE-FIX tool set (a byte copy kept before the fix), GREEN x3 = the same command after the fix |
| `fix-r2-mutation-<tool>.tsv` | mutation results per tool, one rootless IMG-TESTUTIL container per mutant, N00 PASSED, C00 SURVIVED, every other mutant KILLED, no equivalent rows left (M17 of seeds_import is killed by a docs/21 whose stated item bound is below the plan) |
| `fix-r2-qa-class-inventory.txt` | member inventory of the QA-instrument classifier over the LIVE `docs/issues` (UNCONFIRMED for the T161 freeze) with control needles |
| `fix-r2-samples/` | the records the tools write on the fixture; every record carries `freeze` and `fixture: true` in-file (host-side `python3 -I w2_samples.py`) |

The earlier files of this directory (`ledger-<task>-*`, `mutation-*.tsv`, `samples/`) are the round-1 evidence, kept unchanged, and are superseded by the `fix-r2-` files for the tools changed in round 2.
