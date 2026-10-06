# WP-04 round 7 (wp04g) notes: fixes for the WF7 review of the round-6 WP-04 helpers (`WF7-REVIEW-wp04-verifier-r6`)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06 |
| Status | round 7 fix evidence; independent review of this change is owed (the author is not the reviewer, constitution 11.4.142) |
| Source | the WF7 review `WF7-REVIEW-wp04-verifier-r6` (scratchpad copy) and its probes `wf7/wp04-verifier-r6/probes` (p5 to p16, r1 to r5, mutants RM1 to RM5) |

`$EV` = `specs/001-full-project-audit-remediation/evidence`.

**Honest limit of this round's input (11.4.6).** The review file I was given has its sections 3 (mutation table) and 4 (findings) still holding the
placeholders `PENDING_MUTATION_TABLE` / `PENDING_FINDINGS`. The ids I-1 to I-4 and M-1 to M-9 are therefore NOT all recoverable. I worked from section 1
(which names I-1, I-2, I-3, M-2, M-3, M-5, M-6 and the three "info" items), the probe logs and the reviewer mutants. UNCONFIRMED: the review's own wording
of I-4 (I took the probe `p7` "a broken submodule is skipped, main is pushed, exit 0" as I-4, it is the only important item left over) and of M-1, M-4, M-7,
M-8, M-9 (no probe or text points at them). If the final review text names a defect that is not in the table below, it is NOT fixed by this round.

## Fixed (test first: RED against the pre-fix helpers, then GREEN three times)

| Id | Class | Change | Test (file, case) |
|---|---|---|---|
| I-1 (important, regression of round 6 N6-4) | false exit 20 | `verify_repos.sh`: the round-6 override of `filter.<n>.clean/smudge/process` with an EMPTY command makes a REQUIRED driver (`filter.<n>.required=true`, the git-lfs layout) a hard git error ("clean filter 'x' failed") for every touched tracked file, so a clean repository read exit 20. Every driver name now also gets `filter.<n>.required=false` in the same `GIT_CONFIG_COUNT` block (the name may contain dots; a driver set only in the GLOBAL config is covered, `config --name-only --get-regexp` reads all scopes). A really modified file is still dirty (13). | `test_verify_repos_r7.sh` I-1: repository config, global config only, dotted driver name `a.b`, golden-false (modified file under the required filter still exit 13) |
| I-2 (important, regression of round 6 N6-2) | over-reach: UNKNOWN-DIFFERENT (14) where the definite answer is REMOTE-BEHIND (11) | `verify_repos.sh` held-pin check: the tag OBJECT line of an annotated tag (advertised beside its peeled `^{}` line) is decided by the peeled commit line, and a HELD blob or tree (a tag on a non-commit) can never descend from a pin: neither makes the answer "undecided" any more. A branch tip or a peeled commit that this clone does not hold stays undecided (14). | `test_verify_repos_r7.sh` I-2: annotated tag, tag on blob, tag on tree, `--fetch`; golden-false: a not-held branch tip (14), an annotated tag over a not-held commit (14), a tag whose commit is the pin (held, 0) |
| I-3 (important) | W6-6 closed only for a `.git` FILE | `scope_check.sh` walk: git walks UP to the parent when the `.git` entry is a directory that is no valid git dir (truncated HEAD) or a dangling symlink, so `rev-parse --git-dir` succeeded and the toplevel test `continue`d (a dirty submodule read clean, exit 0). The toplevel mismatch of a submodule that HAS a `.git` entry is now `submodule_git_unreadable`, exit 20. | `test_wp04g.sh` I-3 (truncated HEAD, dangling symlink, `.git` file to a missing dir, restored = dirty again 13, uninitialised = 0) |
| I-4 (important, UNCONFIRMED id, see above) | broken submodule skipped, main pushed | `push_recursive.sh discover`: a gitlink whose directory has a `.git` entry (file, directory or dangling symlink) that is no repository of its own is `submodule_git_unreadable`, exit 20, nothing pushed; only an ABSENT `.git` is an uninitialised submodule and skipped. | `test_wp04g.sh` I-4 (moved git dir: main NOT pushed; truncated legacy HEAD; dangling symlink; golden-false: healthy submodule pushed, uninitialised skipped and main pushed) |
| M-2 | W6-7 ref-corruption variant | `scope_check.sh head_mode`: "no commit yet" is no longer "`rev-parse -q --verify HEAD` failed". A zero-length or garbage branch ref, an emptied packed-refs or an unresolvable detached HEAD in a repository that HAS history (loose ref file present or non-empty HEAD reflog) is exit 20 `git_tree_unreadable`; a rewritten append-only store no longer reads as a new one. A repository without a commit still holds no store (0). | `test_wp04g.sh` M-2 (zero-length ref, garbage ref, emptied packed-refs, restored = 13, no-commit repository = 0) |
| M-3 | W6-12 one pipe-status site left | `integrate_merge.sh`: `g remote | sort` inside the loop made a failing `git remote` "no remotes" (nothing_to_merge, exit 0). Read with its status first: `git_listing_failed`, exit 20. The same class in `integrate_ff_only.sh` (`REMS="$(g remote)"`: a failing listing was zero remotes) got the same fix (`finish 20 refused git_listing_failed`). | `test_integrate_merge.sh` and `test_integrate_ff_only.sh` (shim model; control with real git) |
| M-5 | N6-4 one more surface | `verify_repos.sh`: `core.alternateRefsCommand` (a repository-configured program run by `git fetch` for a repository with an alternate object store) is set to `true` on every git command. | `test_verify_repos_r7.sh` M-5 (fixture with a control that the program runs under a plain fetch) |
| M-6 | N6-5 residual | `index_health.sh` P3: (a) a CASE VARIANT of a real third-party root (`Third` for an indexed `third/`) now FAILs `third_party_root_case_mismatch`; (b) an NFD root that names an NFD path git really indexed is a normal hit, no longer refused `third_party_root_not_canonical` (a false refusal of round 6); an NFD root with only the NFC path indexed stays refused. | `test_index_health_extra.sh` M-6a, M-6b |
| F7-7 (disk/docs review, same area) | mutation adequacy | `mutate_index_health.sh` m29 (drop the explicit control-character test) survives because every control character is a Unicode `C*` category character that the next loop refuses: an EQUIVALENT mutant. Renamed `m29-eq-...`, the harness prints `EQUIVALENT` for `-eq-` ids and does not count them. | `mutate_index_health.sh` |

## Mutation adequacy (cheap paired mutants for the NEW code, plus the reviewer's surviving mutants)

- `scripts/repo/tests/run_wp04g_mutations.sh`: 20 mutants of the round-7 code (J1-J8 verify_repos, K1-K5 scope_check, L1-L2 push_recursive, M1-M2 integrate, N1-N3 index_health), results in `wp04g-mutation.txt`. The reviewer's mutants RM2, RM3 and RM4, which survived the round-6 verify tests, are J6, J7 and J8 here and are caught by `test_verify_repos_r7.sh` (RM3 needed a new process-filter fixture, RM4 a pin held only by a tag).
- `run_wp04f_mutations.sh`: the two mutants whose `old` text the round-7 edit moved (G3 and W4) were updated to the same logical mutation of the new text; they are not weakened. The whole f-driver is re-run (`wp04g-mutation.txt`).
- Not re-run (hours, unchanged by round 7): `run_wp04_mutations.sh`, `run_wp04b_mutations.sh`, `run_wp04c_mutations.sh`, `run_wp04d_mutations.sh`. UNCONFIRMED that every older mutant still dies; the round-7 edits touched only lines no older driver names (`grep` of their `old` texts against the changed lines found none).

## Owner decisions (recorded, NOT decided here)

1. **Main pushed past an unpublished submodule pin (sibling of I-4).** Probe p7b: with an INTACT submodule the helper refuses or holds the submodule's unrecorded hand commit (20 `unrecorded_local_commit`) but still pushes the main repository whose CPA commit moves the gitlink to that unpublished commit; a fresh `clone --recurse-submodules` of the published main then fails `not our ref` (`verify_repos.sh` reports it afterwards as DIVERGED/REMOTE-BEHIND of the pin). Options: (a) S6 refuses to push a repository whose gitlink points at a commit that the submodule's remotes do not hold after this run (cross-repository dependency check, exit 20 or 11 `pin_unpublished`); (b) keep deepest-first independent pushes and rely on the verifier; (c) push main only when every submodule row is PUSHED or SAME. Not changed.
2. **P3 typo and homoglyph roots (M-6 residual).** A root that matches no indexed path in any case (`thrid`, a Cyrillic letter in `third`) is legitimate when the tree is simply not indexed, so it cannot be refused mechanically. Options: require each third-party root to match at least one TRACKED path (the tracked list is a P2 input), or keep PASS and list the roots that matched nothing in the row. Not changed.
3. **N6-3 pre-existing `--json` file.** After an exit-20 count failure no new report is written, but a report file left by an EARLIER run stays and contradicts the exit status (probe p16). Options: remove the target at start, remove it on exit 20, or refuse to overwrite. Not changed.
4. F2 (S1 writes no tracking ref) and W6-10 (the conservative 11) are unchanged from `wp04f-notes.md`.

## Owed / UNCONFIRMED

- Other `git remote` and `git ... | ...` sites: `integrate_ff_only.sh` `vet_remotes` (`done < <(git -C "$d" remote)`) and the `for r in $(git -C "$d" remote)` loops (lines 119, 150, 184, 185, 205) still read a failing `git remote` as "no remotes" (UNCONFIRMED consequence; no trigger shown).
- SIGINT legs of W6-5 are void in the review (a background job of a non-interactive shell ignores SIGINT); an interactive-terminal SIGINT run is owed.
- `derive_scope.sh` keeps a duplicate entry of `classes.build` while `deny` is deduplicated (probe p16): UNCONFIRMED whether a consumer depends on either.
- mawk-only image behaviour of the changed helpers (the container leg below ran the verify and audit suites, not the WP-04 helper suites), exactly as in round 6.
- The round-6 status `wp04f-green.txt` header hashes are stale by design (it is a record of its own run).
- No `pgrep -f` or `pkill` was used. No git stage, commit or push, no `tasks.md` edit, no credential.
