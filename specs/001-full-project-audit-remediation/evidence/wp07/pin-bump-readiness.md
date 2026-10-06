# WP-07 pin-bump readiness (read-only analysis)

Identity: catalogizer / 001-full-project-audit-remediation / WP-07 (T074 to T085) | Revision 1 | 2026-10-05 | draft, UNREVIEWED
Nothing was pulled, fetched, checked out or committed. Evidence below comes only from objects and refs already in `submodules/constitution/.git`.

## 1. Facts (commands read-only, output as observed)

| Fact | Evidence |
|---|---|
| Recorded gitlink and checked-out HEAD | `git ls-tree HEAD submodules/constitution` = `10b7a06c4a2ec3f06b4cde9b1611a622a79320ad`; submodule `git status --short` empty |
| Target object is already local | `git -C submodules/constitution cat-file -t e44f22f` = `commit` |
| Target is a fast-forward | `merge-base --is-ancestor 10b7a06 e44f22f` = true (ANCESTOR_OK); 25 commits in `10b7a06..e44f22f` |
| Tip of e44f22f | `e44f22fc54685bc026d9c3a8ce3daff015798063`, 2026-10-03 15:52 +0500, "§11.4.235 extended with clause (D) FINDING-LAYER CLASSIFIER" |
| Tracking refs (local, NOT live) | origin, github, gitlab, gitverse, gitflic `main` = `e44f22f`; `vasic_digital_github/main` and `vasic_digital_gitlab/main` = `10b7a06` (stale ref, would need a fetch to know); `upstream/main` = `c9ac78e` (known stale, docs/11 F-4) |
| Newer tip named in tasks.md | `be06384` (T080, seen by `git ls-remote origin` on 2026-10-03): `cat-file -t be06384` = fatal, not a valid object, so it is NOT in the local clone. UNCONFIRMED: its content, and whether it is still the live tip. Only a network read (`ls-remote`, T080) can say. |
| Diff size | 28 files, +7320 -485 (`diff --stat 10b7a06 e44f22f`): 8 added, 20 modified |
| Canon text change | `Constitution.md`, `CLAUDE.md`, `AGENTS.md`, `QWEN.md`, `GEMINI.md` each +4 -2: the single change is a new clause (D) FINDING-LAYER CLASSIFIER inside §11.4.235. No new anchor number. |
| Gate ledger | `scripts/gates/gate_ledger_deferrals.tsv` +1 row: `CM-REVIEW-FINDING-CLASS-RECORDED` `OWED-GATE-106` |
| Hook and index | `scripts/post_update_hook.sh`, `helix-deps.yaml` unchanged in the range (0 diff lines). `constitution_index.yaml` unchanged and STALE at e44f22f: its `source_sha256` is `d915a5c1...` (the 10b7a06 `Constitution.md`), while `Constitution.md` at e44f22f hashes `8cc29e0d71989fb7de2c42d1e116a4bf8a2f4bb24fd0aa6f488ca9a6700ccb39`, and the index lists 5 mentions of 11.4.235 but was generated 2026-09-26. This is the T078 upstream report. |
| Nested engines | range changes no `.gitmodules` and no nested gitlink (diff name list holds none), so `submodule update --init --recursive` should change nothing for e44f22f (UNCONFIRMED until run; T080 records before/after) |
| Rest of the range | `scripts/fastcycle/**` (migrate.sh +1296, fc_common.py, repo_verify.py, tower_detector, many tests) and `scripts/hooks/test_credential_scan_lib.sh`: tooling the project does not call today (UNCONFIRMED: no caller search made beyond the post-update hook) |

## 2. What the bump would change in this repository

1. One gitlink line: `submodules/constitution` `10b7a06 -> e44f22f` (fast-forward, `git merge --ff-only`, ORIG_HEAD kept, T080).
2. `.specify/memory/constitution.md` and `.specify/memory/constitution-appendix.md` still cite pin `10b7a06...` and `Constitution.md` sha256 `d915a5c1...` (grep confirmed, the only tracked files outside specs/ that mention it). T081 regenerates them with `scripts/governance/regen_speckit_catalogue.py` (T075/T076, absent: `scripts/governance/` holds only `tests/`): new pin, new sha256 `8cc29e0d...`, §11.4.235 (D) added at appendix line ~3492 block; version line MINOR bump (pin adds a clause only) and `Last Amended`; revision header added to `constitution.md`.
3. Spec documents citing `10b7a06` (docs 02, 04, 11, 12, 16): descriptive, to be amended in the WP-07 change set where they state the current pin.
4. If the carrier-row change applies: `scripts/repo/private_key_carriers.tsv` (T040a, not yet present).
5. Hook outputs (`skills/`, `.mcp.json`, `.gemini/`, `.qwen/`, `prompts/`, `.claude/*`, `.git/hooks`) only in T082, which is blocked on ODG-12.

## 3. Policy as stated (owner, recorded in tasks.md/research.md; not re-decided here)

* FR-017 answer (T012a, OD-38): go-ahead to move the pin; target is the LATEST REVIEWED constitution tip ("always latest"). If the live tip is beyond e44f22f, the target moves to it only after the T074 range review and the T077 gates are extended to it and pass; until then the target stays e44f22f.
* ODG-12 hook variant: research OD-31 recommends variant B (project-only, `PATH` without the `claude` binary, docs/11 section 7.4 item 3). It does not block T080/T081/T083/T084/T085; it blocks only T082 and T084a. The hook is run only after T085 has pushed the pin.
* No force push, no history rewrite, fast-forward only (constitution 11.4.113); pre-op backup before the move (9.2).
* Observation, not a decision: the local tracking refs show 5 remotes at e44f22f and 2 vasic-digital remotes at 10b7a06; "every remote holds the target" (T080 `pin-held.txt`) is therefore UNCONFIRMED and must be proven live at run time.

## 4. Exact validation sweep, in order (all UNRUN)

Preconditions: T069 register live, T008/T007 (images and RUNP) done, T014 HC-0 recorded, T012a recorded; WP-03 and WP-04 done.
1. T074: backup of the working tree to `$EV/wp07/backup.txt`; `git -C submodules/constitution fetch --all --prune`; `ls-remote` tip table; `log --oneline`, `diff --stat`, diffs of `Constitution.md` and the four mirrors into `$EV/wp07/range-review.md` (the table above is the offline pre-read of it).
2. T075 then T076: failing test, then `regen_speckit_catalogue.py` and drift check; GREEN x3; mutation (drift check that ignores edits) in `$EV/wp07/drift-mutation.txt`.
3. T077: `git -C submodules/constitution archive e44f22f` into a scratch dir (submodule stays at 10b7a06); run the lockstep and anchor-block integrity gates read from `scripts/gates/` (names to confirm; present candidate: `cm_covenant_114_235_propagation.sh` and its mutation test; `cm_gate_ledger_ratchet.sh` for the new deferral row), in a container; output `$EV/wp07/lockstep.txt`.
4. T078: `$EV/wp07/upstream-report.md` (stale `constitution_index.yaml` hash, see section 1), register item after T069.
5. T080: `git ls-remote` every remote into `tips-before-merge.txt`; `merge-base --is-ancestor e44f22f <tip>` per remote into `pin-held.txt`; `git -C submodules/constitution merge --ff-only e44f22f`; `submodule update --init --recursive` with `submodule status --recursive` before and after into `nested-update.txt`; local commit of the gitlink held for review.
6. T081: regenerate `.specify/memory/` with T076; clean regeneration diff; version and header edits.
7. T083 (run A, no hook needed): `scripts/verify_constitution_inheritance.sh`, `scripts/audit/anti-bluff-scan.sh` (both present), plus constitution gates for the changed anchor, in containers; transcript `sweep-pin.json`; GREEN means no FAIL beyond the T040 ratchet baseline measured at the old pin; each FAIL a `Queued`/`Bug` register item.
8. T084 G-PIN review (verdict `$EV/reviews/WP-07.json`, `accepted_pins` entry, `finding_layer` per finding), T081a owner ratification, T085 CPA push (fast-forward, S6 pushes the held pin commit).
9. After T085 and ODG-12: T082 hook run (variant B), before/after hashes of `.claude/` and `.git/hooks`, `hook-outputs.json` decisions, run B sweep, T084a review.

## 5. Risks and gaps stated

* Blocked here: T074 (needs `fetch`), T077 (container), T080 and later (pin move, owner flow, commit). Not attempted.
* `scripts/governance/regen_speckit_catalogue.py` does not exist; T075 and T076 are the only WP-07 tasks with host-side test-first work, and T075 needs IMG-TESTUTIL (T008).
* Possible newer live tip `be06384` unknown locally; the range review would have to be re-run to it.
