# WP-21 evidence: T179, T180, T185

Identity: utc=2026-10-07T16:48:27Z host=anton uid=1000 git_head=65e5f3391acadf08bbae583e2f371fbb068b9d5d (working tree dirty: the scripts below are uncommitted, their content hashes are in SHA256SUMS and in each transcript header). All suite runs: `scripts/containers/run_pinned.sh IMG-TESTUTIL -- bash <test>` (rootless podman, scratch on /dev/shm).

| File | What |
|---|---|
| RED-test_reverify_gate.txt | T179 RED before `--completion` existed: pass=5 fail=24 (`gate: unknown argument --completion`), a host transcript. **Correction (WF23 review E2):** the earlier statement here that the recorder cannot run a RED once the implementation exists was FALSE: the test takes `GATE=<path>`, so the RED is reproducible today against the gate of the parent commit (`git show 203d83f0^:scripts/register/gate.sh`, kept as `fix-r2-gate_pre_t180.sh`, sha256 784173e6...). It is now in the ledger: seq 25 (item AUD-T179, polarity RED, exit 1, `[gate: unknown argument --completion]`, pass=5 fail=25) |
| GREEN-test_reverify_gate-run1..3.txt | T179 GREEN x3: pass=30 fail=0 each; also recorded as ledger seq 1..3 (item AUD-T179, polarity GREEN) |
| GREEN-test_gate-regression.txt | T063 suite after the change: pass=126 fail=0 |
| mut2/mut.tsv, mut2/mut.tsv.summary | T180 mutation runner: 27 mutants (22 gate.sh, 5 register_ext.sql) killed=27 survived=0, controls (unmutated, comment-only) both NOT killed. A first run had 2 survivors (guard_nd_numeric, guard_nd_regex); the test was strengthened (F5, E4) and the runner re-run |
| mutant_ext_row_removed.sql | the G-GATE mutant of T180 (the `('v_reverify_queue','view_not_done')` row removed); the test run against it FAILs, recorded as ledger seq 4 (AUD-T180, polarity MUTATION, exit 1) |
| ticket_delta.json (also ../register/ticket_delta.json) | T185 FACT / UNCONFIRMED statement with the commit list |

Ledger: `../ledger.jsonl` seq 1..4 (verify with `tools/evidence/verify --ledger ../ledger.jsonl`). Item ids use the AUD- prefix because the schema accepts no task ids.

## Round 2 (WF23 fix pass, 2026-10-08): ledger entries that fingerprint the GATE, not its test (review E1, E2)

Seq 1..4 above (AUD-T179 GREEN x3, AUD-T180 MUTATION) carry `target_ref = scripts/register/tests/test_reverify_gate.sh`: the fingerprint is the TEST's, so a later change of `gate.sh` would not stale them and a RED and a GREEN would share a fingerprint. The ledger is append-only, so they stay as history and are superseded by seq 25..32, recorded by `scripts/register/tests/record_wp20_evidence.py` through `tools/evidence/evrec` (rootless `scripts/containers/run_pinned.sh IMG-TESTUTIL`, `--network=none`, test sources `test_reverify_gate.sh` + `lib.sh`):

| seq | item | polarity | target_ref | exit | what |
|---|---|---|---|---|---|
| 25 | AUD-T179 | RED 1 | `.audit/scratch/wp21-r2/gate.sh` (the pre-T180 gate) | 1 | `GATE=<that file>`; `FAIL R1 rc=2 [gate: unknown argument --completion]` |
| 26, 27, 28 | AUD-T179 | GREEN 1..3 | the same path, now a byte copy of `scripts/register/gate.sh` (sha256 65e0124b...) | 0 | pass=30 fail=0; the RED/GREEN pair shares argv, cwd, target_ref and test fingerprint and differs in the target fingerprint |
| 29, 30, 31 | AUD-T180 | GREEN 1..3 | `scripts/register/gate.sh` | 0 | the gate itself is the target |
| 32 | AUD-T180 | MUTATION 1 | `scripts/register/gate.sh` | 1 | `REG_EXT_SQL=<mutant_ext_row_removed.sql>`: `FAIL reference gate registry holds no view_not_done row` |

Per-run transcripts: `fix-r2-ledger-t179-*.txt`, `fix-r2-ledger-t180-*.txt` in this folder (the ledger blobs of those entries, with the ledger header). Reviewer mutants of `gate.sh` GM01..GM05 are run by `scripts/register/tests/mutate_wf23.py` (GM02-GM04 killed, GM05 equivalent by exit code, GM01 survives until `test_reverify_gate.sh`, a file outside this pass's edit scope, pins the numeric guard's anchors: request R7 in `../wp20/fix-r2-open-requests.md`). Verify: `tools/evidence/verify --ledger ../ledger.jsonl`.
