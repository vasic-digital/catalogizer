# WP-21 evidence: T179, T180, T185

Identity: utc=2026-10-07T16:48:27Z host=anton uid=1000 git_head=65e5f3391acadf08bbae583e2f371fbb068b9d5d (working tree dirty: the scripts below are uncommitted, their content hashes are in SHA256SUMS and in each transcript header). All suite runs: `scripts/containers/run_pinned.sh IMG-TESTUTIL -- bash <test>` (rootless podman, scratch on /dev/shm).

| File | What |
|---|---|
| RED-test_reverify_gate.txt | T179 RED before `--completion` existed: pass=5 fail=24 (`gate: unknown argument --completion`). NOT recorded in ledger.jsonl: the recorder cannot run a RED once the implementation exists; this transcript is the RED. |
| GREEN-test_reverify_gate-run1..3.txt | T179 GREEN x3: pass=30 fail=0 each; also recorded as ledger seq 1..3 (item AUD-T179, polarity GREEN) |
| GREEN-test_gate-regression.txt | T063 suite after the change: pass=126 fail=0 |
| mut2/mut.tsv, mut2/mut.tsv.summary | T180 mutation runner: 27 mutants (22 gate.sh, 5 register_ext.sql) killed=27 survived=0, controls (unmutated, comment-only) both NOT killed. A first run had 2 survivors (guard_nd_numeric, guard_nd_regex); the test was strengthened (F5, E4) and the runner re-run |
| mutant_ext_row_removed.sql | the G-GATE mutant of T180 (the `('v_reverify_queue','view_not_done')` row removed); the test run against it FAILs, recorded as ledger seq 4 (AUD-T180, polarity MUTATION, exit 1) |
| ticket_delta.json (also ../register/ticket_delta.json) | T185 FACT / UNCONFIRMED statement with the commit list |

Ledger: `../ledger.jsonl` seq 1..4 (verify with `tools/evidence/verify --ledger ../ledger.jsonl`). Item ids use the AUD- prefix because the schema accepts no task ids.
