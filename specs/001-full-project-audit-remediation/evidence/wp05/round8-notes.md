# WP-05 evidence tooling after the recorder: T048, T048a, T051, T051a, T052, T053, T054, T055, T056, T057 (partial) - state, deviations, owed

| Field | Value |
|---|---|
| Date | 2026-10-07 |
| Scope | `tools/evidence/**`, `docs/scripts/evidence-*.md`, the eleven suites under `scripts/testing/full_automation/` (TS-02 migration) with their `docs/scripts/catalog_*.md` guides, `evidence/wp05` (nothing staged or committed; other streams' uncommitted work was not touched) |
| Not done | T058 (independent review, never self-review), the README row of `docs/scripts/README.md` (not edited, listed owed), tasks.md (not edited) |
| Evidence | every capture starts with an identity header (sha256 of tools, tests, schema, fixture tree, image digests for container legs) and ends with `# DONE` |

## 1. Per task

| Task | State | Evidence |
|---|---|---|
| T048 | DONE (reading task): `DR-E1.md` records what continuum offers (read at `4639347`, its own suite RUN `ok`), and that it settles NONE of the four open points for this repository (a real ev/1 ledger is REFUSED by `continuum-integrity`, exit 4). Adapter seams named. Nothing was invented, nothing built. | `DR-E1.md`, `t048-continuum-probe.txt` |
| T048a | already implemented by an earlier round (schema revision 7 carries `pre_release` / `test_state_gos`); re-verified here, GREEN x3 | `t048a-green-x3.txt` (RED/GREEN of the original work: `T048a-red.txt`, `T048a-green*.txt`) |
| T051 | DONE: `wrap-go.sh`, `wrap-bash.sh`, `lib/wrap_common.sh`, `evparse.py` | RED `wrappers-red.txt` (both suites fail, rc=1), GREEN x3 `t051-green-x3.txt` (host, fixtures), live legs in IMG-GO and IMG-KCOV `rollout/step2-wrappers.txt` |
| T051a | DONE: `evrec token`, `--run-token` (wrapper, recorded argv, deriver rule `token_ok`, `verify` `run_token_reused`) | RED `t051a-red.txt`, GREEN x3 `t051a-green-x3.txt`, mutants K1..K9 in `mutations.txt` |
| T052 | DONE: structural census `census_ab_pass.py`, ONE shared definition `lib/ab_pass_with_evidence.sh` that records an ev/1 entry before printing PASS, eleven suites migrated (11 definitions before, 0 after, none left behind) | RED `t052-red.txt`, GREEN x3 `t052-green-x3.txt`, `ts02-census.json`, `ts02-mutation.txt` |
| T053 | DONE for the parser legs: `wrap-vitest.sh`, `wrap-gradle.sh`, `wrap-cargo.sh`; live legs `blocked-unavailable` | RED `wrappers2-red.txt`, GREEN x3 `t053-green-x3.txt`, `rollout/step2-wrappers.txt` |
| T054 | DONE except the real-defect polarity leg (OWED, below): `tools/evidence/verdict`, the 18 cases in `tests/verdict_cases/` | RED `t054-red.txt`, GREEN x3 `t054-green-x3.txt`, mutants D1..D11 |
| T055 | DONE: `evrec anchor`, anchor comparison in `verify` (exit 2), strength probe, anchor leg of `evrec rerecord` | RED `t055-red.txt` (against the chain-only verifier of round 7), GREEN x3 `t055-green-x3.txt`, mutants A1..A13 |
| T056 | DONE: `mutations.txt` (+ `mutations-addendum.txt`) | the anchor-comparison mutant (A1) and the GREEN-without-RED mutant (D1) are the named T056 pair; the commit-turn mutants of T056 are the T050 mutants C1..C10 and RV-X4 re-run (mapping by my reading of each id) |
| T057 | PARTIAL: steps 1 to 4 transcripts in the pinned images with image digests, `docs/scripts/evidence-recorder.md` (+ three more guides) written; the polarity switch (step 3) and the `docs/scripts/README.md` row are OWED | `rollout/step1..4*.txt` |

## 2. Deviations from the task text (each recorded, none decided for the owner)

1. T051 asks for `test_wrap_go.sh` as a remote lane of IMG-GO through `scripts/build/dispatch.sh`. `dispatch.sh` is another stream's uncommitted work and no build host was used: the live leg ran LOCALLY in IMG-GO through `scripts/containers/run_pinned.sh` (11.4.173: the Go compile happens in the pinned container, not on the bare host). The remote-lane run is OWED. `run_go.sh` itself refused during this work (anti-mess sweep drift: another run's container without an op label), so `run_pinned.sh` was called directly, as the T050 captures do.
2. T054: three of the 18 scenario cases (`blind_red`, `launder`, `reopen_pass`) contain an entry the strict recorder REFUSES to write (a RED that passed, a GREEN that failed, a REOPEN that passed). They are built with the test-only forger `tests/forge.py` (record as PROBE, retag, rechain) and derived with `verdict --chain-only`; the strict mode refuses the same ledger (`schema_invalid`, tested). The other 15 are built with the real recorder. `--test-source ./check.sh` names the test bytes (the POC hashed argv[0] only; with operand hashing the target would have changed the test fingerprint).
3. T051a needed three changes to existing recorder code (additive): `--run-token <32 hex>` is exempt from the credential-flag redaction of argv (the flag name contains `token`), the run's own token stays readable in the stored stdout/stderr blobs (placeholder swap around `redact_bytes`), and the deriver removes the `--run-token` pair from argv before the same-test comparison and masks the token in stdout before the identical-runs comparison (both differ per run by design). The token rule applies whenever a RED/GREEN entry carries a token, not only with `--state-delta`.
4. `evrec remap-refs` ignores seq-map entries of kind `anchor` (T055 lists the replacement anchor in the seq map); `verify_main` prints `FAIL` for exit 2.
5. Anchors: `strength` is `policy|mechanism`; continuum's is `policy|mechanism|unknown` and an unreachable probe remote is `unknown` there but `policy` here (the task text: "downgrades to policy"). Recorded in `DR-E1.md` point 4.
6. REMOVED: 11.4.113 absolute. The strength probe no longer performs any push: the scratch-repository forced rewrite, the `--probe-scratch-ok` flag and the `evrec-scratch-remote` marker are gone (11.4.113 forbids every forced, plus-ref or lease-guarded push and every history rewrite on ANY repository with or without approval; there is no scratch exception, so there is no owner call to make). The probe is now a read-only inspection of a LOCAL bare repository's own configuration (`probe_kind=config`: `git config --show-origin --get-all` of `receive.denyNonFastForwards` and `receive.denyDeletes`, the config scopes of the probing environment included); `rewrite_rejected` is true only when BOTH are effectively true, executable hooks are recorded as evidence only, a network remote is never contacted, and the strength stays `policy` when unproven. Evidence: `t055b-red.txt`, `t055b-green-x3.txt`, `t055b-mutations.txt`, `t055b-evidence-suites.txt`, `t055b-scan.txt`.
7. `wrap-*.sh` exit 126 for every "no test outcome" (including usage errors) instead of 64/127, because the recorder maps 1..125 to `fail` (a usage error must not be a genuine RED).
8. Existing test files edited additively: `test_evrec.sh` (T051a section appended), `test_evrec_hermetic.sh` (the scratch checkout now also carries the new tools), `run_mutations.py` (scratch tree now carries the new tools; without it a survivor would read as CAUGHT-OTHER-CHECKS-ONLY). `run_mutations_wp05b.py` was corrected once AFTER `mutations.txt` (one expectation string, A12): the corrected re-run is `mutations-addendum.txt`, which also re-runs the commit-turn mutants with the corrected runner.
9. Gradle and cargo fixtures are AUTHORED from the documented formats, NOT recorded (no Gradle or cargo run exists here): UNCONFIRMED against a live run. Go, bash and vitest fixtures are REAL recordings (IMG-GO go1.25.14, IMG-KCOV, vitest 1.6.1 in a scratch install in IMG-NODE).

## 3. OWED / not decided

1. T058 independent review (reviewer-authored tamper case) of recorder, verifier, deriver, wrappers, anchors, `rerecord`/`remap-refs`.
2. `docs/scripts/README.md` rows for `evidence-recorder.md`, `evidence-wrappers.md`, `evidence-verdict.md`, `evidence-ts02.md` (the file was not edited).
3. T054 polarity switch on one real register defect (needs T069, T038, T031; else `$EV/p0-exit.json` and D-04 in T109). T052 finding filing (T069): nothing to file, no copy left behind.
4. Live legs of `wrap-vitest.sh` (IMG-NODE with catalog-web, WP-11), `wrap-gradle.sh`, `wrap-cargo.sh` (IMG-ANDROID, IMG-RUST, WP-14), and the dispatch remote lane of `test_wrap_go.sh`.
5. The four continuum points (hash construction, record schema, exit codes, anchor format) and the anchor location / `mechanism` claim (OD-76); owner decisions R6-3, F4 and the Python minimum (W7-9) are unchanged and NOT mine.
6. The suites of `scripts/testing/full_automation/` were not run against a live catalog API (none available): migration verified by the census, `bash -n` on all eleven, and the helper behaviour tests.
7. Not authored as mutants (stated): the second detection path of the Go parser (the `--- FAIL:` text scan duplicates the failure events) and of the bash/vitest parsers (`_finish` repeats the exit-status and header rules); a mutant that removes only one of two redundant paths is observationally equivalent, so the P-bash2 and P-vt3 mutants remove both.

## 4. UNCONFIRMED
Behaviour on Python older than 3.11 (the IMG-TESTUTIL / IMG-GO interpreters are 3.11.2, the host 3.14.4; every suite ran on both); the wrappers against live Gradle and cargo output; whether continuum's `pkg/store` and `pkg/lock` fit the single-writer ledger; behaviour of the probe against a real hosting platform's branch protection.
