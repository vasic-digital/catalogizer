# Round 25 review residue (tasks.md rev 25, commit 2e1fd314)

Revision 2, 2026-10-04 (revision 1 text unchanged; section "Rev 26 disposition" appended). Stop rule decided by the owner: two more rounds, then bound it. Round 25 was the last review round. Findings below are OPEN and tracked; none is fixed in rev 25. Source: five independent read-only reviews. Verdict per slice: NO-GO. Blocking 3, important 25, minor 17.

## Blocking (3)
- B-A1 (T040/T042 S1, T041 S6, T095): a forged `CPA-Run:` trailer passes every check except the S0 launcher; foreign G-GATE change with a copied trailer is fast-forwarded unheld and pushed to other remotes. Fix: one "CPA commit" predicate (local run record lists the sha, or a committed GO lists the run) used by S1 routing, unrecorded_local_commit, Foreign-Commit naming, S6 withhold, T095; fixtures for forged-trailer fast-forward and hand commit with copied trailer, each with a mutation.
- B-C1 (T264/T300, block 3): frozen worktree scan set includes ignored files that change during a run (.codegraph/codegraph.db, .remember/*, .claude/), so AUD-002 always stops `worktree_scan_set_changed`; SC-002 unreachable. Fix: reviewed exclusion list of plan-written state with reasons and a needle.
- B-C2 (block 8, T225/T255/T271/T286/T299): own-org `submodules/security` derives unit id `security`, colliding with the cross-cutting `security` unit; T225 fails `unit_id_collision`. Fix: rename cross-cutting unit (e.g. `security-xcut`).

## Important (25)
Phase 0: (A-I1) T039/T040 baseline bootstrap expects 0/10 for unheld validate_checks.tsv edit but S2 refuses first with 20; (A-I2) `released_code_unreviewed` remediation via self-release does not clear (add reviewed_files clause for self-release verdict); (A-I3) waiver authorisation is a self-asserted string, authoriser_is_producer compares unlike namespaces (need owner attestation + single-uid boundary note).
Phases 1-2: (B-I1) T212 split rows lack run_image; libc check should apply only to native binaries; (B-I2) libc field has no backfill owner, IMG-CLEAN has no shell; (B-I3) T164 and T174 read live tree not frozen snapshot; (B-I4) T165/T167 import command hides .sql path from the T064 journal; (B-I5) T203/T204 use false reason codes, should use artifact_not_yet_built/image_not_built.
Phases 3-4: (C-I1) T244 lacks IMG-RUST guard; (C-I2) block (1) has no rows for tools/**, build/**, codegraph.json, .mcp.json, .lumenignore, .secrets.baseline, .helix/**; (C-I3) T235 does not update fixture_roots.txt review header; (C-I4) no review releases T325 release_seam_files.txt edit; (C-I5) T233/T240a/T277a missing from nondeterministic_detectors.tsv; (C-I6) T232 detectors and T267 lack author-side paired mutations.
Phases 5-7: (E-I1) T440a class (c) count 17 vs 16 (MVT/js_mse_eme is class d); (E-I2) stale "re-cut never mints a second increment" bullet; (E-I3) release-seam rule over-reaches (register/ledgers/targets.yaml); (E-I4) constitution survivor route cycle T572<->T580b and no re-run after T580e (UNCONFIRMED likelihood); (E-I5) reviews of change sets with blocked legs not covered by owner-blocked rule (T359, T395, T411, T428); (E-I6) WP-73 main-mode CPA runs can hit the S0 sweep with uncommitted records.
Docs (D-I1..D-I5): all docs follow rev 24 (670/75, no T134a); README and docs/21 call rev 24 uncommitted; docs/16 12.2.8(4) contradicts T042 on materialisation and lacks B1/I1 text; path-gate table lacks release_seam and guard_registry rows; P6-P7 increment and pre-qa semantics stale in docs 21, 06, 04.

## Minor (17)
A: M1 merge-commit path set in S5; M2 commit_push.conf, review-verdict.schema.json and T040b tables have no gate row; M3 T039 deletion fixture does not say held. B: T121b default classes for IMG-DOCS/scanner images; T134a terminal state on T134 pass; T142 readelf/binutils unconfirmed. C: T245 stale ref; T240a fixture ordering; "no class exempts secret fold" bullet; T277b missing T236 edge; two ambiguous unit-location cases; R08/R09 lock sha unconfirmed; vague checks T251, T260, T261, T277. E: T447a/T447b gate not named (T451); unreachable "(c) re-cut with no finding" branch; T581(0) omits earlier re-cut fingerprint records and T580e evrec. Docs: WP-13 ODG-08 input; P1 terminal-state rule, IMG-INFRA-CLIENT/IMG-SIGVERIFY, run_image, libc, waiver roster absent from docs.

## Also owed
- contracts revision: finding/1 unit and reg_components must accept every id in $AUD/units.json (no task defines reg_components).
- Mermaid rendering unconfirmed in the round-25 docs review (browser launch failed there; earlier docs agent rendered all blocks).

## Rev 26 disposition

Evidence: the tasks.md rev 26 Status row (673 tasks, 78 suffix ids) and the task bodies it names, read on 2026-10-04 by a docs-sweep agent. "Fixed in rev 26" means the task text now states the fix; nothing is implemented or run yet, and rev 26 itself is pending re-review. No finding above is deleted.

- B-A1: fixed in rev 26 (one CPA-commit predicate, T042 S0, used by S1/S6/T041/T095/T090; forged- and copied-trailer fixtures with mutations, T039/T043).
- B-C1: fixed in rev 26 (`$AUD/worktree-scan-exclusions.txt`, T225/T264; `scan_exclusion_unreasoned`, `scan_exclusion_covers_input`).
- B-C2: fixed in rev 26 (cross-cutting unit `security-xcut`, T225 block (8); T225d, T225e).
- A-I1, A-I2, A-I3: fixed in rev 26 (P0 I1 T039/T040; I2 T042 launcher clause (3); I3 waiver attestation, `attestation_invalid`).
- B-I1 to B-I5: fixed in rev 26 (T212 `run_image`/`artifact_kind`; `scripts/containers/read_libc.sh` and libc backfill; T164/T166/T174 frozen listing and `freeze.json`; T165 `locked.sh import-sql`; T203/T204/T220 reasons).
- C-I1 to C-I6: fixed in rev 26 (T244 IMG-RUST guard; unit-map rows; T235 header; T325 hold; nondeterministic rows; T232/T267 mutations).
- E-I1: fixed in rev 26 (T440a, 16 class (c) by the recording parent's `.gitmodules`).
- E-I2, E-I3, E-I6: fixed in rev 26 (each QA deploy mints its own increment, T580e (2); release-seam list limited to tracked policy/threshold inputs; T580e commit window).
- E-I4: fixed in rev 26 as to route (further T580b (3c) iteration consuming the T440a `owed_to_T580b` rows); whether the T572/T580b cycle is fully broken stays UNCONFIRMED (not re-verified here).
- E-I5: fixed in rev 26 (T359, T395, T411, T428 added to T457; `scripts/release/check_review_edges.sh`).
- D-I1: partly fixed. This sweep aligned docs 02, 03, 05, 09, 14 and contracts/README.md with rev 26; docs 01, 07, 08, 10, 13, 15, 19, 20 hold no rev-26 contradiction found. README and docs 04, 06, 11, 12, 16, 17, 18, 21 are outside this sweep (being edited concurrently by other agents) and are not judged here; treat as open until their own sweep reports.
- D-I2, D-I3, D-I4: still open (README, docs/16, docs/21; outside this sweep).
- D-I5: partly fixed (docs/05 section 13.5, revision 10); still open in docs 21, 06, 04.
- Minor A M1-M3, B (T121b classes, T134a terminal state, T142 ELF reader), E (T447a/T447b on T451; no re-cut without a finding; T581 (0) lists earlier re-cut records and evrec): fixed in rev 26.
- Minor C: T277b now depends on T236 and the secret-fold bullet names the fixture-root exception (fixed); rev 26 states P3-P4 M1-M6 applied, but T245 stale ref, T240a fixture ordering, the two ambiguous unit-location cases, R08/R09 lock sha and the vague checks of T251/T260/T261/T277 were not re-verified here (UNCONFIRMED).
- Minor Docs: P1 terminal-state rule now stated in docs/09 for T244; WP-13 ODG-08 input, IMG-INFRA-CLIENT/IMG-SIGVERIFY, `run_image`, libc and the waiver roster still open (docs 16, 21; outside this sweep).
- Also owed, contracts: still open as a schema change; contracts/README.md revision 17 records the required `finding/1` `unit` description (prose only, schema JSON unchanged).
- Also owed, Mermaid rendering: still open.
