# WP-04 fix round 3: notes

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-05 |
| Last modified | 2026-10-05 |
| Status | evidence note of the fix round for WF2-REVIEW-wp04-helpers (nothing staged or committed) |

## Evidence files (all sha256-headed)

- `wp04c-red.txt`: first run of `test_wp04c.sh` against the unfixed helpers, 74 ok and 123 FAIL (exit 1).
- `wp04c-green.txt`: `test_wp04c.sh` three times (230 ok, 0 FAIL, exit 0 each) and the 11 earlier WP-04 tests once (all exit 0).
- `wp04c-mutation.txt`: 64 mutants, 58 CAUGHT, 2 EQUIVALENT (EQ-C6, EQ-F13: a second layer refuses the same input), 3 SURVIVED (F14, F15, P5), 1 driver ERROR (S4).
- `wp04c-mutation-rerun.txt`: S4, F14, F15, P5 re-run after the test was strengthened: all CAUGHT.
- `wp04c-real-s3.txt`: the shipped S3 and S2 over the 35 WP-04 files of the real repository, exit 0 (before this round the S3 exited 20 on the real repository).

## Honest limits

- Several of the 230 cases (gitlink commit, `--ev` spellings, validate_cheap gitlink, ff_only unknown tip without a local commit, uppercase `--owned-orgs`,
  the moved-remote-under-hold case, the untracked-directory backup, the change-set spelling and adoption cases) were added after the first RED run; their
  RED is the mutation that removes the guard they test, not a failing run against the unfixed helpers.
- The full mutation run decided 60 mutants against the test before F14/F15/P5/S4 were strengthened; the strengthening only added cases (P5's case also
  fetches the diverged tip), it removed none.
- Not done: review minors 4 (trial merge in the shared tree before the backup), 6 (dirty-submodule exception can pin no content), 9 (`verdict_go` has no
  schema validation), 10 (`.gitea/workflows/*` form, existing test needs `noext` to fire), 11 (`cr_msg.*` temp leak on kill, `CPA_HELPER_PID` export), 12
  (the RED file names), 13 (`docs/scripts/README.md` row), 14 (gitlink pin proof binding in `commit_recursive`).
- UNCONFIRMED: `integrate_ff_only` main fetch recursion into submodules was not reproduced by the ff fixture (the merge fixture reproduces it and is
  fixed); `--no-recurse-submodules` there is kept as defence. When HEAD holds no class table the helper's own table is the HEAD table; run from the real
  repository, whose tables are untracked, that is the working-tree file until the tables are tracked.
- `scripts/detect-landmines.sh` is still mode 100644 (no tracked file mode was changed); the registry row now names `bash`.
