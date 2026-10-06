# T050 round 4 notes (evrec / verify), response to WF3-REVIEW-wp05-evrec

| Field | Value |
|---|---|
| Date | 2026-10-06 |
| Scope | `tools/evidence/**` and `evidence/wp05` only (no contract, tasks.md or governance edit, nothing staged or committed) |
| Baseline (round 3) | `T050r4-baseline/` (evcore.py sha256 1bc9d547bf42be95, equal to the identity the reviewer hashed) |
| Evidence | `T050r4-red.txt`, `T050r4-green.txt` (x3), `T050r4-mutations.txt`, `T050r4-container.txt`; every file starts with an identity header (sha256 of tools, tests, schema) and ends with `# DONE` |
| Owner scope decision | fix every real-defect finding (N1-N4, real-defect minors), mutation-adequacy for the guard-removing mutants, record the rest as owed |

## 1. Findings and state

| Finding | State | How |
|---|---|---|
| N1 rerecord writes into the shared store | FIXED | `--name` must match `[A-Za-z0-9][A-Za-z0-9._-]*` and may not be `ledger-seq-map.json` or end in `.lock`, `.lock.guard`, `.runs`, `.stalled`, `.go` (`name_not_plain_token`, 64). `--out` is refused (`out_is_shared_store`, 70) when its realpath is inside the ledger directory, `$EV` or the blob store at ANY depth (symlinks resolved), not only equal to them. Test: `test_evrec_r4.sh` N1 block (shared flake ledger, deferrals and ledger byte-compared, evidence tree listing compared before and after). |
| N2 redaction | FIXED for the forms listed, limits below | JSON `"key":"value"` (double and single quotes, value taken to its closing quote), `--flag VALUE` pairs in argv (`redact_argv`) and in stream text, `Authorization: <scheme> X`, `Cookie`/`Set-Cookie`, three-part JWT, Slack, Stripe and Google API key shapes. Fixtures are assembled from pieces at run time (no credential-shaped literal committed). Benign JSON and flags stay unchanged (no false positive). |
| N3 rule 11 bypass | FIXED, two layers | The launcher list is extended (run_pinned.sh, kcov, podman, docker, nerdctl, ssh, adb, pytest, bats, vitest, jest, xvfb-run, strace, uv, poetry, bundle, rake, flutter, ctest and others, see `INTERPRETERS`): a RED/GREEN/MUTATION through any of them needs `--test-source` (69). For a command outside every list, every argv operand that is a readable regular file is hashed into `test_fingerprint`, so different test bytes cannot share a fingerprint behind an unknown launcher. |
| N4 reconcile false PASS | FIXED, owner question open | `evrec run` records each stored entry in a run index `<ledger>.runs` when `EVREC_RUN_TOKEN` is set (token `[A-Za-z0-9][A-Za-z0-9._-]{0,63}`, validated before the command runs). `reconcile --run TOKEN --runner-count N` (or the env token) counts only index rows whose seq and entry_hash match the ledger. An unscoped reconcile on a ledger that holds tokened entries is refused (`reconcile_unscoped_on_tokened_ledger`). The probe of the review (foreign entry, RED, failed GREEN retried, 2 more GREEN: 5 commands) now gives `count_mismatch` for 5 and passes for 4. |
| N5 mutation adequacy | FIXED for the guard-removing mutants | W3-1 to W3-10 are in `run_mutations.py` and each is killed by a named check (W3-6 by a deterministic unit check of the lock record and by the unreadable-marker semantics, see section 3). Further mutants for every new guard (N1 to N4, m1, m2, m4) are listed in `T050r4-mutations.txt`. |
| m1 `--ledger` anchor | FIXED | The anchor is looked up beside the ledger (`anchors.jsonl`) unless `EV_ANCHOR` is set. |
| m2 remap-refs | FIXED | Writes through a symlink (the link stays a link) and keeps the permission bits. |
| m3 golden test temp dirs | FIXED | `test_evcore_golden.py` removes its scratch directories at exit. |
| m4 deleted cwd / verify without schema | FIXED | `evrec run` in a deleted cwd is `usage_error` 64 with no traceback; `verify` without a readable schema is UNVERIFIED (3) instead of 67. |
| m5 stale notes | CORRECTED here | See section 4. The older notes are not edited. |
| m11 secret-shaped fixtures | FIXED for the new tests and for `test_evcore_golden.py` (pieces joined at run time). The round-3 RED capture text `T050r3-red.txt` still contains the fixture strings, it is a recorded artifact and is not rewritten. |

## 2. Found while running inside IMG-TESTUTIL (new, real defect, fixed)

The suites run in `scripts/containers/run_pinned.sh IMG-TESTUTIL` (uid 1000 mapped, repo at `/src`, python 3.11.2, jq 1.6, jsonschema 4.10.3). With the round-3 tools the first container run failed 124 checks. Root cause, measured: the ev/1 contract declares a relative `$id` (`ev/1`); jsonschema 4.10.3 resolves the local `#/$defs/sha256` references against it as a remote document (`RefResolutionError: unknown url type: 'ev/ev/1'`), so every record was refused (`schema_unavailable`/crash); 4.19.2 on the host does not. Fix inside `tools/evidence`: `evcore.portable_schema` validates against an in-memory copy with an absolute URN `$id` (the contract file is untouched, every `$ref` is local). The two test helpers that call jsonschema directly (`test_evrec.sh`, `test_evidence_record_schema.sh`) apply the same in-memory change. Result: all seven test files pass in the container (`T050r4-container.txt`).

## 3. Design notes

* W3-6 was a real defect: a lock taken while `/proc` was unreadable recorded `cmdline: ""`, which a reader with a readable `/proc` treats as pid reuse, so a live lock was reaped and an entry lost. A holder that cannot read its own cmdline now records `cmdline: null, cmdline_unreadable: true`; such a holder is never reaped while alive, and is reaped when its pid is dead. An empty string written by an older tool is treated as the same unreadable identity.
* `reconcile`: untokened runs keep the old behaviour (whole ledger) so existing runners keep working; it is exact only when the runner is the sole writer. The refusal on tokened ledgers makes the protection switch on as soon as any runner adopts a token.

## 4. Corrections to earlier notes (review m5)

* `T050r3-DR-E1-open-deviation.md` says the owner question is NOT in owner-request-list and the I13 row of `T050r3-implementation.md` says "Not decided". Both are superseded: `decisions/owner-request-list.md` item 11 and `owner-decisions.yaml` (`owner_answers_2026-10-05_batch3.evidence_chain`) record the owner's answer, SWITCH to the continuum chain, with four points OPEN; the python docs/06 chain is interim.
* The statement that the RED of the rev-7 withdrawn-prefix checks "cannot be re-created after the fact" is wrong: `EV_SCHEMA=<git show HEAD:specs/001-full-project-audit-remediation/contracts/evidence-record.schema.json>` run through `test_evidence_record_schema.sh` reproduces it while the rev-6 schema is at HEAD (reviewer's measurement; not re-run in this round).

## 5. Residual limits of redaction (declared, never "complete")

Pattern scanning cannot see: a secret split across argv elements or across stream chunks, encoded or encrypted secrets (base64 of a password outside an `Authorization` header, URL-encoded values), secrets without a recognisable key name or shape, short flags such as `-p VALUE` (ambiguous with non-credential flags), values shorter than the pattern minimum (4 to 6 characters), and credentials in file contents the command never prints. Named variables (`EVREC_REDACT_VARS`) remain the exact mechanism for known values.

## 6. OWED items (not tracked by me: tasks.md and the owner list are outside this slice)

1. Owner question (docs/06): rule 10 (a GREEN is always `pass`) against s4.2 / s13.4 `launder` (a failing GREEN iteration must be recorded so the deriver can refuse it). As built, a failing GREEN is refused after the run and leaves no ledger trace; `reconcile --run` is the only backstop. How a failed GREEN attempt is to be recorded needs an owner decision (schema, `contracts/`, is out of this slice).
2. Gitignore entries for `ledger.jsonl.lock`, `.lock.guard`, `.tmp.<pid>`, `.stalled` and the new `ledger.jsonl.runs` run index (decide: tracked or ignored), owed since round 2 (m7).
3. 11.4.18 companion docs for evrec and verify; the torn-line owner question (s16 against T049) (m7).
4. m6: no RED/GREEN/BASELINE/MUTATION can be recorded for the schema's `remote_service` class or a `container_image` given by digest (only files and directories resolve); needs a fingerprint reader per class or a declared limitation.
5. m8: the commit-turn grant writer (T580e) takes no `ledger.jsonl.lock`, so a grant made between the in-lock check and the atomic write does not stop that write; cross-slice contract UNCONFIRMED.
6. m9: lock liveness is local to one PID namespace (host vs container writer on a shared bind mount); UNCONFIRMED whether evrec ever runs both ways at once. Note the container now maps uid 1000, so this is no longer hypothetical for a shared ledger.
7. m10: `EV_TEST_SOURCES` is still split on whitespace (a source path with a space breaks).
8. The `$id` of `contracts/evidence-record.schema.json` is relative; fixing it at the source (an absolute URN) is a contract change for its owner (tools now tolerate it).
9. Mutants not individually authored for the round-4 code: each reserved name suffix of `--name`, the `.tmp.` name rule, each added launcher name beyond kcov/podman/docker/ssh/run_pinned, the Slack/Stripe/Google/`access_key` redaction patterns individually, `operand_files` edge cases (directories, flags), run-index rows with a non-dict JSON value. None of these is a guard whose removal loses an entry or leaks a documented credential form, but each is open.
10. The reviewer's non-guard mutants beyond W3-1..W3-10 were not enumerated in the review text; none is recorded as owed beyond item 9.

## 7. UNCONFIRMED

* Whether concurrent UNTOKENED runners exist in practice (they would share the legacy unscoped reconcile).
* Behaviour of the tools under jsonschema versions other than 4.10.3 and 4.19.2.
* The mutation sweep covers evcore.py edits; edits of `evrec`/`verify` wrappers and of `hermetic.sh` beyond the round-3 set are not mutated.

## 8. Results (identity header of every T050r4-*.txt: evcore.py 4af4288875d0ad93, test_evrec_r4.sh 8ad0bb43224659ac, run_mutations.py 028303758e568084; all equal the current files)

* RED (`T050r4-red.txt`, round-3 tools from `T050r4-baseline/`): test_evrec_r4.sh 105 checks, 66 FAIL; the golden suite stops at the first round-4 symbol (`redact_argv` missing, rc 1); test_evrec_hermetic.sh 1 FAIL; the three older shell suites 0 FAIL (unchanged behaviour).
* GREEN x3 (`T050r4-green.txt`): 55 / 24 / 140 / 105 / 73 / 6 checks, 0 failures, rc 0, in all three runs.
* Container (`T050r4-container.txt`, IMG-TESTUTIL via run_pinned.sh, once): the same six suites plus test_evidence_record_schema.sh, 0 failures.
* Mutations (`T050r4-mutations.txt`): 178 mutants, 178 caught by a check that names the cause, rc 0. A first sweep of this round had 10 survivors (five N2 pattern mutants that did not really disable their pattern, N1 ev-dir, N3 operands-always-hashed, m2 x2 expect text, N2 redact-everything expect text); the mutants were corrected, the missing checks added (fingerprint formula pinned, `$EV` protected independently of the ledger directory), and the full sweep was re-run.
* Side effect: the first r4 capture attempt also ran the `t048a` step, which re-captured `T048a-green-r3.txt` (same test script, one-line portable-`$id` helper change); the earlier content of that file is replaced.
