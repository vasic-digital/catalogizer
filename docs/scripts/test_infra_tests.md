# tests/infra (test infrastructure tests) - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T20:00:00Z |
| Status | new in the working tree (T128-T134a), not yet committed; independent review owed (constitution 11.4.142); its row in `docs/scripts/README.md` is owed (that file is being edited by another agent) |
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
