# WP-03 round 8 notes (verify_repos / scope_to_lumen_json part of the WF7 round-6 review fixes)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06 |
| Status | round 8 evidence index; independent review owed (11.4.142) |

The full account is `$EV/wp04/wp04h-notes.md` (`$EV` = `specs/001-full-project-audit-remediation/evidence`). The parts in WP-03 scope:

- **M-6** `verify_repos.sh` removes a stale `--json FILE` when the run exits 20 (test: `scripts/repo/tests/test_wp04h.sh` section M-6; record `$EV/wp04/wp04h-green.txt`).
- **M-7, M-8** `scripts/audit/scope_to_lumen_json.py` (reserved class name `third_party_nested` exit 3; `--check` exact for `classes` lists, exit 1). RED `round8-red-scope-to-lumen-json.txt` (`pass=55 fail=5`), GREEN x3 `round8-green-scope-to-lumen-json.txt` (`pass=60 fail=0`).
- **Container leg** `round8-green-container-test_wp04h.txt` (IMG-TESTUTIL, git 2.39.5, mawk): 75/0; test_scope_to_lumen_json 60/0 and test_push_recursive 153/0 in the same image (outputs summarised in `$EV/wp04/wp04h-green.txt`).
- **E-1 repoint** `$EV/wp03/round6-notes.md` never existed; the round-6 N6 fixes are described in `docs/scripts/verify_repos.md` section "Round 6", `docs/scripts/index_health.md`, `docs/scripts/scope_to_lumen_json.md` and recorded in `$EV/wp03/round6-*.txt`.
- **E-2** the r6 mutation records ran against `test_verify_repos_r6.sh` sha `53b76cb3` (committed file `d934db91`): re-established by the WF7 reviewer (V1..V15 caught) and by the full f-driver run of this round (`$EV/wp04/wp04h-mutation.txt`).
- Round 7 had no WP-03 owner decision for M-6; the stale-file question (round 7 owner decision 3) is answered by removal on exit 20.
