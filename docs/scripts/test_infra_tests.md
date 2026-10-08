# tests/infra (test infrastructure tests) - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:40:16Z |
| Status | WF17 fix round 5 applied in the working tree (not committed); the independent review of that round is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/wf17/` |
| Source | `tests/infra/*.sh`; tests self |

## Tests

| Test | Task | What it proves (real rootless podman, no mocks) |
|---|---|---|
| `test_compose_files.sh` | T129 | no literal credential or host port, digest-pinned images equal to the lock, labels, rootless; `gen_env.sh` mode 0600, random per run; 8 compose mutations and a mode mutation |
| `test_seed_corpus.sh` | T130 | two runs byte-identical, digest recomputed independently, changed seed changes the digest, magic numbers, unicode and long names; 6 mutations |
| `test_up_down.sh` | T131 | lease per project (same refused, different admitted), label-only teardown (a foreign unlabelled container survives), release, idempotence; 4 mutations |
| `test_probes.sh` | T128 | protocol answers, wrong-credential negative controls, host / service-class refusals, TCP-only carrier FAILs, MinIO BLOCKED; mutations of the probes and the refusals |
| `test_roundtrip_<proto>.sh` | T132, T134 | x3 round trips recorded as `ev/1`, state delta, sabotaged-run control, mutations of the step runner |
| `test_concurrency.sh` | T133 | two stacks at once: distinct projects, ports, networks, data; both leases held, second owners refused; marker isolation; `concurrency.json` |
| `test_nfs_terminal_state.sh`, `test_nfs_fallback_state.sh` | T134, T134a | the terminal-state checks through `TIC tooling unit`, fixtures and mutations |
| `test_nas_readonly_leg.sh` | T132 | real-NAS leg: read-only scan, SKIP without env, credentials never in argv/env/logs, real two-host run |

Run a test with `bash tests/infra/test_<name>.sh` (mutations run by default; `*_NO_MUTATIONS=1` skips them). Tests must run one at a time: every `run_pinned` container trips the anti-mess sweep
(`AM-P1 container_without_op_label`, UNCONFIRMED root cause: the sweep reads the label `op_id`, run_pinned sets `catalogizer.op_id`), so a concurrent TIC call is refused until it ends.

## WF12 review fixes (revision 2)

- New: `test_sweep_leaks.sh` (F2 leak sweep, 7 paired mutations), `test_client_isolation.sh` (F3), `test_consumers.sh` (F1: container-build, setup-test-env, the Go helper, an enumeration of every tracked reference), `test_ftp_capability.sh` (F13 control pair).
- Changed: `test_compose_files.sh` (F4: literals found by structure in every string of a service; reviewer mutants RM1, RM2, RM3, RM5, RM7), `test_up_down.sh` (F2, F5, F6, F15, F16, F17; RM4), `test_concurrency.sh` (F5, F6, F11 measured evidence), `test_probes.sh` and `test_roundtrip.sh` (F8 specific refusal signals; RM6; F10), `test_nfs_terminal_state.sh` (F19, F20; RM8), `test_nfs_fallback_state.sh` / `test_blocked_external.sh` (F7), `test_nas_readonly_leg.sh` (F3, F14, F18), `test_seed_corpus.sh` (F14: per-run directory names).
- Mutant runs use `TI_FAILFAST=1`: the first violated check ends the run; the owner-aware `ti_down` cleans up.

## WF17 fix round 5 (revision 3)

- Stale statements corrected (TI-I5): the mutation counts are in the per-suite `*-mutations.txt` records of the evidence directory (they are not repeated here, they changed); `test_compose_files.sh` now renders the compose files with the real podman-compose under sentinel variables and allow-lists the rendered tree (`tests/infra/compose_scan.py`, mutants in `compose_mutants.py`).
- New suites (class closure): `test_sweep_contract.sh` (A: the real anti-mess sweep is clean at every lifecycle point), `test_ownership.sh` (B), `test_signals.sh` (C: INT/TERM/HUP at four observed stages of `up.sh`, plus `setup-test-env.sh`, `nfs_attempt.sh` and the test harness itself), `test_inputs.sh` (D: one row per input defect), `test_build_stack.sh` (C/F: `container-build.sh` with a fault-injecting compose wrapper), `test_nfs_attempt.sh` (E), `test_cache_install.sh` (the corpus-cache race), `test_evidence_binding.sh` with `check_evidence_binding.sh` (H: header and manifest hashes against the committed files). Shared helpers: `mutlib.sh` (paired mutations over a copy of `scripts/test-infra`; identity mutants must survive), `capture.sh` (requires `--` before the command).
- Harness: `TI_TEST_MODE=1` is exported by `lib.sh`; the EXIT trap waits (bounded) for background starts before it tears their ids down.
- Tests still run one at a time on this host. The anti-mess sweep wording above (`op_id` vs `catalogizer.op_id`) is outdated: `run_client.sh` replaces the label and the sweep reads `catalogizer.op_id` first.
