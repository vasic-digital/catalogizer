# Owed contract items of the WP-02 / WP-03 scripts

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-05 |
| Last modified | 2026-10-06 (round 4: N-I6 resolved) |
| Status | draft, untracked work product (review fix of WF-REVIEW-wp02-wp03, finding I11); a tracked item in the register is owed |
| Source | `scripts/repo/verify_repos.sh`, `scripts/audit/derive_scope.sh`, `contracts/repo-verification-report.schema.json` |

Each item below is something the scripts need from a contract that this work product may not edit (contracts are outside its scope). Until the contract owner decides, the scripts do the conservative thing and say so; none of these is silently invented.

## IC-30: "emit both classifications" for an ODG-15 pending account (T032)

- Need: for a repository whose organisation is `helixdevelopment1` or `milos85vasic` (BLOCKED-ON ODG-15, `scripts/audit/own_orgs_pending.txt`) docs/21 IC-30 asks the verifier to emit BOTH classifications (third-party under the conservative reading, own under the alternative one).
- State: `scripts/audit/derive_scope.sh` does carry both (`class`, `alt_class`, `flag` ODG-15 in its TSV). `verify_repos.sh` cannot: `repo-verification-report/1` has `additionalProperties: false` on the report and on every row, so an extra field would make the report schema-invalid. The verifier counts a pending account third-party and prints a one-line note on stderr naming the accounts and this file.
- Owed: a contracts amendment adding an OPTIONAL row field (suggested name `owned_alt`, boolean, present only on rows whose two readings differ) and the matching `summary` count, with the consumer (`scripts/repo/commit_push.sh` stage S7) told that the field is advisory. Owner: the contract owner; decision input: ODG-15.
- Visible in: `verify_repos.sh` stderr note, `docs/scripts/verify_repos.md` (Limits), `evidence/wp03/baseline-summary.md`.

## B3: the hash evidence of an excepted row has no report field

- Need: T032 asks that a `dirty` row carry as evidence the sha256 of the excepted working-tree file and of its committed blob.
- State: the hashes live in the exception row (`scripts/repo/exceptions.tsv`, columns 4 and 5) and are checked on every run: a stale hash makes the repository an unexcepted dirty one (exit 13). The report row only has `excepted` and `exception_reason` (schema-closed), so the hashes are not repeated in the report.
- Owed: an optional row field (suggested `exception_evidence`: list of `{file, wt_sha256, blob_sha256}`) in the contract, if the register import wants the hashes in the report itself.

## A repository with no remote at all

- Reported as `unproven: ["NO-REMOTE-BRANCH"]` with `remotes: []` (the schema forbids remotes on a not-owned row, and a repository with no remote has no organisation). The `summary.classes` counts remote comparisons only, so such a row raises `summary.unproven` (exit 14) but no class count. Owed: the contract owner either accepts this reading of `NO-REMOTE-BRANCH` or adds a dedicated unproven member (which would change the exit-code table).

## Exit code 12 for a `behind` row under `--strict`

- See `evidence/wp03/exit-code-decisions.md` decision (b): UNREVIEWED by the owner. A fast-forwardable `LOCAL-BEHIND` shares code 12 with a true divergence only because the closed exit set has no other fitting code; the `problems` list (`behind` versus `diverged`) is unambiguous.

## WF2-REVIEW round 3: items owed, not closed

- N-I6 (I10 remainder, T031/T032, section 11.4.224): RESOLVED in round 4 (2026-10-06): the images exist; the suites ran in `IMG-TESTUTIL` (375/0, 14/0, 9/0, 39/0), shellcheck 0.11.0 at -S warning is clean, a PS4 line-trace coverage figure is recorded (kcov cannot attach in the sandboxed image). See `evidence/wp03/round4-notes.md`. Open remainder: kcov branch coverage UNCONFIRMED.
- m-d: three organisation classifiers (`org_of.py`, `scope_render.py`, the structural scope renderer) can differ on a non-github host or a nested group path; today 97/97 real URLs are github.com with 0 disagreements.
- m-f, m-k, m-l, m-m, m-n: see `docs/scripts/verify_repos.md` (Round 3 residuals) and `docs/scripts/derive_scope.md`; m-n (IC-30 as a tracked register item) is still not a register item.
- m-i: `golden.json` / `golden.sha256` were rewritten together after the G-CG-2 fix with no record of the prior hash; all 60 Lumen entries are `in_tierA: true`, so the SKIP path is never exercised. Owner decision whether to keep a hash history; not changed here (audit goldens were not touched in round 3).
- m-j: `docs/scripts/README.md` (the scripts index) is still not created (outside this round's touch list).
