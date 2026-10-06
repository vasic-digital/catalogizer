# WP-04 stage helpers: T040 (integrate_ff_only slice), T040a (reader slice), T040b (loader slice), T041 (record_deferral slice)

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-05 |
| Last modified | 2026-10-06 |
| Status | evidence index of the WP-04 stage helpers; the section 11.4.44 header was added in fix round 4 (WF3 review m9) |

Host run (bash, git, python3, jq). Not through the pinned containers or `cpa-host`: `scripts/containers` and `cpa-host` (T042) are
outside this slice. Tests use throwaway repositories and local bare remotes only; no force operation, no real remote, no credential.

Files: `helpers-red.txt`, `helpers-green.txt` (three runs of each of the four tests), `helpers-mutation.txt`,
`recursive-red.txt` (record_deferral), `recursive-mutation.txt`, `check-classes-red.txt`, `check-classes-mutation.txt`,
`fixture-roots-red.txt`, `fixture-roots-mutation.txt`. Mutation driver: `scripts/repo/tests/run_wp04_mutations.sh`.

Not done in this slice (named precisely):
- T040: `integrate_merge.sh`, `scope_check.sh`, `validate_cheap.sh` + `validate_checks.tsv`, `check_no_ci.sh`, `check_revision_headers.sh`,
  `anti-bluff-scan.sh --files-from`, `validate_baselines/*`, `hook-filters.json`; inside `integrate_ff_only.sh`: the Foreign-Commit listing,
  the adoption-anchor read of `adoption.json`, the admission-table (T040b tables) routing rule, the "GO committed in HEAD lists it"
  clause of the CPA predicate (T042). Decisions UNCONFIRMED: see the header of `integrate_ff_only.sh`.
- T040a: `.secrets.baseline` and the S2 secret-fold fixtures: `detect-secrets` is absent on the host and belongs to IMG-TESTUTIL (T006),
  which is built through `scripts/containers` (not touched); `scope_check.sh` (S2) is absent; `private_key_carriers.tsv` not created.
- T040b: held-table rule, `class_table_unreviewed`, `table_admits_unheld_path`, `legacy_row_not_dropped`, moved-legacy-file rule, the
  OD-76 blob rule (all need the S2/S3 stages and a diff against HEAD); `docs/scripts/check_classes.md`.
- T041: `commit_recursive.sh`, `push_recursive.sh`, `$EV/wp04/recursive-red.txt` for them (the file holds only the record_deferral RED).
