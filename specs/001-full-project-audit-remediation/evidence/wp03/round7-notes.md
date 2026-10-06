# WP-03 / WP-02 round 7 notes: `verify_repos.sh` and `index_health.sh` fixes of the WF7 review of the round-6 verifier

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06 |
| Status | round 7 fix evidence; independent review of this change is owed (the author is not the reviewer, constitution 11.4.142) |
| Source | `wp04/wp04g-notes.md` (the full table, RED, GREEN and mutation records live in `$EV/wp04/wp04g-*.txt`); the WF7 review `WF7-REVIEW-wp04-verifier-r6` (its sections 3 and 4 still held placeholders when this round started, see `wp04g-notes.md`) |

`$EV` = `specs/001-full-project-audit-remediation/evidence`.

## What changed for the verifier (WP-03)

| Id | Change | Test |
|---|---|---|
| I-1 | every configured `filter.<n>.*` driver also gets `required=false` next to its EMPTY command (a REQUIRED driver without a command made git die "clean filter failed": a false exit 20 on a clean repository; also the global-config layout and dotted driver names) | `scripts/repo/tests/test_verify_repos_r7.sh` |
| I-2 | the tag OBJECT line of an annotated tag is decided by its peeled `^{}` line; a held blob or tree (a tag on a non-commit) cannot hold a pin: neither is "undecided" (14) any more, the definite REMOTE-BEHIND (11) stands; a branch tip or peeled commit not held locally stays undecided | same file |
| M-5 | `core.alternateRefsCommand=true` on every git command | same file |
| mutation adequacy | the reviewer mutants RM2, RM3, RM4 (all survived the round-6 verify tests) are now caught: RM3 by a process-filter fixture, RM4 by a pin held only by a tag, RM2 by a not-held branch tip golden-false | `scripts/repo/tests/run_wp04g_mutations.sh` J6, J7, J8 |

## What changed for the audit helpers (WP-02)

- `index_health.sh` P3: a case variant of an indexed third-party root FAILs `third_party_root_case_mismatch`; an NFD root that names an indexed NFD path is a hit (no longer `third_party_root_not_canonical`). `test_index_health_extra.sh` 54 checks (was 47), `run_wp04g_mutations.sh` N1 to N3.
- `mutate_index_health.sh`: m29 is an EQUIVALENT mutant (renamed `m29-eq-...`, printed EQUIVALENT, not counted): 31 mutants, 0 survived, 1 equivalent.

## Evidence

- Host: `$EV/wp04/wp04g-green.txt` (22 suites x 3 runs, all rc 0, plus `test_verify_repos.sh` 404/404 x 3), RED `wp04g-red.txt` (31 failing lines against the pre-fix helpers), mutations `wp04g-mutation.txt`.
- Container leg (IMG-TESTUTIL via `scripts/containers/run_pinned.sh --network=none`, once): `round7-green-container-test_verify_repos_r6.txt` (41/0), `round7-green-container-test_verify_repos_r7.txt` (22/0), `round7-green-container-test_verify_schema.txt` (14/0), `round7-green-container-test_verify_repos.txt` (404/0), `$EV/wp02/round7-green-container-test_index_health_extra.txt` (54/0). The WP-04 helper suites were NOT run in the container (UNCONFIRMED mawk behaviour of `scope_check`, `push_recursive`, `integrate_*`).
- `run_pinned.sh` writes disk-headroom records to `$EV/disk/` (shared with other runs; not edited or removed here).

## Owed / owner decisions

See `wp04g-notes.md`: main pushed past an unpublished submodule pin; P3 typo and homoglyph roots; the `--json` file of an earlier run after an exit-20 count failure (all recorded, none decided). The round-6 owner items F2 and W6-10 are unchanged.
