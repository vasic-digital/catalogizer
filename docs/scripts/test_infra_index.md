# Test infrastructure scripts (index) - Companion Guide

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T16:40:48Z |
| Status | new in WF17 fix round 5, in the working tree (not committed); independent review owed (constitution 11.4.142 / 11.4.209) |
| Source | `scripts/test-infra/`, `tests/infra/`, `scripts/container-build.sh`, `scripts/setup-test-env.sh` |

## Purpose

One page that links every companion guide of the real-service test infrastructure (WF17 TI-I4: none of them was reachable from another document). The stack itself is described in [docs/testing/real-service-stack.md](../testing/real-service-stack.md). `docs/scripts/README.md` is owned by another agent in this round: the row that links THIS page from it is owed to that owner (UNCONFIRMED until added); this page is linked from `docs/testing/real-service-stack.md` meanwhile.

## Guides

- Lifecycle: [up.sh](test_infra_up.md), [down.sh](test_infra_down.md), [sweep_leaks.sh](test_infra_sweep_leaks.md), [lib.sh](test_infra_lib.md), [gen_env.sh](gen_env.md), [setup-test-env.sh](setup_test_env.md), [container-build.sh](container_build.md)
- Probes and clients: [probe.sh](test_infra_probe.md), [roundtrip.sh](test_infra_roundtrip.md), [run_client.sh](test_infra_run_client.md)
- NFS: [nfs_build.sh](nfs_build.md), [nfs_attempt.sh](nfs_attempt.md), [nfs_terminal_state.sh](nfs_terminal_state.md), [nfs_fallback_state.sh](nfs_fallback_state.md)
- External legs: [nas_readonly_leg.sh](nas_readonly_leg.md), [blocked_external.sh](blocked_external.md), [dotenv_get.py](dotenv_get.md)
- Tests and evidence: [tests/infra](test_infra_tests.md), [check_evidence_binding.sh](check_evidence_binding.md)
