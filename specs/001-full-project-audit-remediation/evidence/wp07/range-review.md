# T074 steps 2 to 4: remote tips and read-only review of the incoming range
Identity: catalogizer / 001-full-project-audit-remediation / WP-07 T074 | Revision 1 | 2026-10-06 | draft, UNREVIEWED | agent wp07 | git HEAD 87909608416f9c13b6b9e116866b6efbdc827cad | pin still 10b7a06c4a2ec3f06b4cde9b1611a622a79320ad
Nothing was checked out, merged or reset. The only write to the submodule: one plain read-only `git fetch --no-tags origin main` (objects and the remote-tracking ref refs/remotes/origin/main; refs/heads and the work tree unchanged, verified before/after).

## 1. Tip table (IC-13), `git ls-remote <remote> refs/heads/main`, 2026-10-06 17:05Z
| remote | url (fetch) | tip of main |
|---|---|---|
| gitflic | git@gitflic.ru:helixdevelopment/helixconstitution.git | a71b1767b40229c8e88140924352fbe990b3049e |
| github | git@github.com:HelixDevelopment/HelixConstitution.git | a71b1767b40229c8e88140924352fbe990b3049e |
| gitlab | git@gitlab.com:helixdevelopment1/helixconstitution.git | a71b1767b40229c8e88140924352fbe990b3049e |
| gitverse | git@gitverse.ru:helixdevelopment/HelixConstitution.git | a71b1767b40229c8e88140924352fbe990b3049e |
| origin | git@github.com:HelixDevelopment/HelixConstitution.git | a71b1767b40229c8e88140924352fbe990b3049e |
| upstream | git@gitflic.ru:helixdevelopment/helixconstitution.git | a71b1767b40229c8e88140924352fbe990b3049e |
| vasic_digital_github | git@github.com:vasic-digital/HelixConstitution.git | a71b1767b40229c8e88140924352fbe990b3049e |
| vasic_digital_gitlab | git@gitlab.com:vasic-digital/HelixConstitution.git | a71b1767b40229c8e88140924352fbe990b3049e |

All 8 remotes agree: unique maximum = **a71b1767** (the 4 of them read via a bounded ls-remote with BatchMode ssh; gitflic and upstream print a post-quantum key-exchange warning on stderr, the tip line was read with stderr dropped).
**Neither e44f22f (named by tasks.md T074, T077, docs/12 14.1) nor be06384 (named by T080) is the live tip any more.** Ancestry, read after the fetch: 10b7a06 is an ancestor of e44f22f, e44f22f is an ancestor of be06384, be06384 is an ancestor of a71b1767: `merge-base --is-ancestor` true for 10b7a06→a71b1767, e44f22f→a71b1767 and be06384→a71b1767. So a71b1767 is a fast-forward of the current pin.
Commits: 10b7a06..e44f22f = 25; e44f22f..a71b1767 = 30; 10b7a06..a71b1767 = 55.
Object availability before the fetch: e44f22f local, be06384 and a71b1767 NOT local (be06384 was reported by the previous read-only analysis as absent; now present).
Not done: `fetch --all --prune` of all 8 remotes (T074 step 2 as written): only origin was fetched, because every remote's tip was read by ls-remote and all equal a71b1767; the other remote-tracking refs (vasic_digital_*, upstream) remain stale locally (UNCONFIRMED whether a later fetch of them changes anything but those refs).

## 2. Range 10b7a06..e44f22f (the range tasks.md T074 names), 25 commits, 28 files, +7320 -485
```
e44f22f 2026-10-03 feat(governance): §11.4.235 extended with clause (D) FINDING-LAYER CLASSIFIER
4bd12c6 2026-10-03 fix(fastcycle/T177-r30): close independent-review Important -- a present-but-CORRUPT (not merely missing) base-commit object in an attacker-writable de
eb790ce 2026-10-03 fix(fastcycle/T177-r29): close R28 NO-GO -- fc_is_partial_clone() missed every promisor boolean spelling but 'true' plus partialclonefilter-only config
07beade 2026-10-03 fix(fastcycle/CA-026-round2): close independent-review BLOCKING-1 -- git-dir isolation alone does not stop argv option injection via a dash-leading rep
36a351a 2026-10-03 docs(fastcycle): T048 Round 24 review M1 fix -- drop the hand-maintained line-number citation that drifted twice in two rounds
38f8c3f 2026-10-03 fix(fastcycle/CA-026): isolate remote-contact git-dir in verify_remote/_remote_head_tip -- untrusted repo-local core.sshCommand/credential.helper/remot
9d5068a 2026-10-03 fix(fastcycle/T177-r28): close R27 NO-GO -- a thin-pack destination without a partial-clone check could lazy-fetch an attacker promisor remote (Importa
9b1470f 2026-10-03 docs(fastcycle): T048 Round 23 review M2/N1 fixes -- known_flaky_gates.tsv deletion comment + r22 regression line-citation drift
660d2f3 2026-10-03 fix(fastcycle/T177-r27): close R26 NO-GO -- unbounded object transfer could write ~71GiB per migration (I-1), `=` in a .gitmodules section name silentl
fae49d8 2026-10-03 fix(fastcycle/T177-r26): close R25 NO-GO -- push-into-untrusted-repo spawned receive-side hooks (B1), submodule config pins keyed by path not section n
afe8fe5 2026-10-03 fix(fastcycle/T177-r25): close R24 NO-GO -- protocol.allow=never was blocking ssh/https (B1), insteadOf still redirected hop-2/sync-fetch (B2), submodu
b209fd9 2026-10-03 feat(fastcycle/T081): wire render_keys.py into sync_all_markdown_exports.sh's freshness gate (US2)
dc972ea 2026-10-03 fix(fastcycle/T177-r24): close R23 NO-GO -- full-scope filter neutralisation (git-lfs gateway), transport-executable closures, header correction, I23-1
7b573e2 2026-10-03 fix(fastcycle/T085-r6): close R5-B1 BLOCKING D-Bus/systemd-run + USB-umount escapes, R5-I1 immediate-detach race, R5-I2 sibling-search field-presence b
07db39b 2026-10-03 fix(fastcycle/T177-r23): close R22 NO-GO -- process-wide, full-effective-config filter-driver neutralisation; withdraw the Round 22 "CLOSED" overclaim
36d2a9a 2026-10-03 fix(fastcycle/review): tower_detector Round 3 independent review -- verdict GO; close the 1 new MINOR + 2 cosmetic notes; 102 -> 103 assertions
1dc5872 2026-10-03 fix(fastcycle/review): tower_detector Round 2 independent review -- verdict GO; remediate 5 new MINOR findings (unpinned MINOR-5/NEW-5 fixes, stale doc
071f325 2026-10-03 fix(fastcycle/review): tower_detector Round 1 independent review remediation -- 2 IMPORTANT (symbol-attribution false-positives, untested headline outp
9073277 2026-10-03 fix(fastcycle/T177-r22): close R21-I1 filter-driver RCE + J19/J28 disposition-record misattribution
204aa7d 2026-10-03 fix(hooks/test_credential_scan_lib): remediate 2 minor review notes, comment-only
b48b2c2 2026-10-03 fix(fastcycle/review): tower_detector test suite -- shellcheck SC2016 hygiene (info-level), zero behavior change
1edb7c5 2026-10-03 fix(hooks/test_credential_scan_lib): shellcheck cleanup, zero behavior change
f662b4d 2026-10-03 feat(fastcycle/review): git-history "patch-tower" (S11.4.250) detector -- new standalone, zero-overlap tooling
1dea38d 2026-10-03 fix(hooks/test_credential_scan_lib): restructure GOLDEN-BAD fixtures so this test's own source bytes never trip precheck_pack_run.py's check_secret_sca
68ddb80 2026-10-03 fix(fastcycle/T085-r5): shellcheck hygiene -- SC2181+SC2329, zero behavior change
```
Diffstat classes: canon text 5 files (Constitution.md, CLAUDE.md, AGENTS.md, QWEN.md, GEMINI.md: +4 -2 each, one clause); scripts/fastcycle/** tooling and its tests (consumers/migrate.sh +1296, verify/repo_verify.py +426, lib/fc_common.py, the new review/tower_detector.* with TOWER_DETECTOR.md); scripts/hooks/test_credential_scan_lib.sh; scripts/gates/gate_ledger_deferrals.tsv +1 row. No .gitmodules change, no nested-gitlink change, scripts/post_update_hook.sh and helix-deps.yaml unchanged (0 diff lines, read from `git diff 10b7a06 e44f22f -- <path>`).
Canon change: §11.4.235 gains clause (D) FINDING-LAYER CLASSIFIER (Constitution.md and the four mirrors identical text); the gate ledger gains CM-REVIEW-FINDING-CLASS-RECORDED as deferral OWED-GATE-106. No anchor added or removed (heading count of `### §`: 256 at both).

## 3. Range e44f22f..a71b1767 (NEW since the plan was written), 30 commits, 80 files, +21488 -7501
```
a71b176 2026-10-05 fix(hooks): carrier-strip #35 — bounded-3-pass iterative entity-decode
a398270 2026-10-05 docs: regenerate remaining doc twins (README/CHANGELOG/AGENT_GUARDRAILS/codegraph-Status/owed-gates)
7a422e0 2026-10-05 fix(hooks): carrier-strip #34 extended to detector-1 — double-escaped HTML tag FP
60e1f42 2026-10-05 fix(hooks): carrier-strips #32-#34 — AKIA doc-placeholder + HTML-rendering FPs
ac5d70d 2026-10-05 cascade: docs(mistiq-vader): close ATM-1117 round-4 scope + file ATM-1121 follow-up
a174007 2026-10-05 fix(hooks): carrier-strip #31 — ssh:// URL-form git remote email-adjacency FP
4d0172c 2026-10-05 docs(constitution): add helixconstitution-v69 CHANGELOG entry
f9d1cde 2026-10-05 docs(constitution): regenerate html/pdf/docx twins — §11.4.276 + §11.4.272-274 mirror-gap fix
1901233 2026-10-05 fix(governance): close §11.4.157 lockstep gap — §11.4.272-274 missing from all 4 mirrors
8671bac 2026-10-05 docs: pre-release currency pass — README Rev 5, 3 doc spot-checks
b0cff47 2026-10-05 feat(governance): add §11.4.276 review-round budget (5-7 max, root-cause-driven)
52da419 2026-10-04 feat(§11.4.209/§11.4.211): add genuine-unavailability Sonnet review fallback; fix carrier-strip #28/#29 false-positive test fixtures
e971a01 2026-10-04 fix(release_prefix): S8/S9 round-7 fix -- I-1 FULL remediation: table-driven near-miss basename sweep closes mC (PREFIX) + mD (CASE-FOLD) gaps, replaces D10
00697b5 2026-10-04 fix(governance): §11.4.230(D) round-3 precision fix -- coalescing failed/refused-build handling + per-branch multi-coexistence clause + ledger row 107/108 
2da2ed4 2026-10-04 fix(release_prefix): S8/S9 round-6 fix -- I-1 residual (D11 line-preserving basename-insert mutation coverage) + M-1 residual (defaulted count in diagnostic
567581b 2026-10-04 fix(governance): §11.4.230(D) round-2 fix-forward on Opus-xhigh review NO-GO (2 Important, 6 Minor)
5fd59fa 2026-10-04 feat(governance): §11.4.230 extended with clause (D) INCREMENTAL DELIVERY GRANULARITY + DECLARED QA-BUILD CADENCE
b195981 2026-10-04 fix(release_prefix): S8/S9 round-5 fix -- I-1 (D10 basename-substring mutation coverage) + M-1 (grep -c doubled-output footgun) + M-4 (4 comment-accuracy co
518d38d 2026-10-04 test(transcript_ingest): track the extra_prefix_attribution fixture (T048 S8/S9 round-5 B-1)
0c1c588 2026-10-04 fix(release_prefix): S8/S9 round-4 -- I-A (4 reviewer mutations) + M-a..M-d closed
be06384 2026-10-03 fix(cycle_report): S12 round-6 GO -- REVIEW_SPAN_INVERTED/REVIEW_ELAPSED_NEGATIVE data-quality flags, proof-of-correctness gaps closed
3e9389a 2026-10-03 fix(release_prefix): S8/S9 round-3 -- B1/I1/I2/I3/M1 fixed, source build-eligible per S11.4.235(D)
e1c6c3a 2026-10-03 fix(independence_tier): T048 round-37 review B3 -- absolute-path stat/id
03c73c6 2026-10-03 fix(cycle_report): S12 F5/F6/F13 follow-ups, round 3 GO -- summed_review_duration_ms, --review-records-dir refusal, inverted-span exclusion
49006bf 2026-10-03 fix(execution_record): T048 round-36 review R36-I1 -- absolute-path grep in exec_record_lookup()
72f5d61 2026-10-03 fix(cycle_report): ATM-1055 status-desync false positive + composition defects (T048 S12)
97fd777 2026-10-03 fix(transcript_ingest): decouple hardcoded ATM- item-tag prefix (T048 S9)
54010ed 2026-10-03 fix(release_prefix): CWD-independent + standalone/embedded dual-layout project-root resolution (T048 S8)
77db11f 2026-10-03 chore: bump 4 nested-submodule gitlinks + add stray TOWER_DETECTOR export siblings
f41a83f 2026-10-03 feat(review_record): add optional finding_layer field per §11.4.235(D)
```
Canon text (Constitution.md +99, each mirror +46/-?; read from `git diff e44f22f a71b1767 -- Constitution.md CLAUDE.md AGENTS.md QWEN.md GEMINI.md`):
1. **NEW anchor §11.4.276** — review-round budget: every independently-reviewed work item converges within a declared budget of at most 7 rounds (5 to 7, consumer-declared), root-cause-driven; it bounds the 're-run until a clean GO' obligation of §11.4.134 (an EXTENSION paragraph is added to that clause). Heading count of `### §` 257 (was 256).
2. **§11.4.209 and §11.4.211 AMENDED (2026-10-04, CRITICAL operator mandate)**: the review and merge-conflict model pins gain a GENUINE-UNAVAILABILITY SONNET FALLBACK: Opus at xhigh stays the PRIMARY substrate, but when Opus is genuinely unavailable (a captured FACT, never a guess) the work may run on Sonnet instead of blocking; Fable still not permitted. The headings change ("... with a genuine-unavailability Sonnet fallback — no Fable"), the escape-hatch list now says 'beyond the sanctioned (A) Sonnet fallback'. This CONTRADICTS the project's own Spec Kit text (constitution.md Principle III and appendix 11.4.209/11.4.211 digests state 'no fallback model, a genuine Opus-unavailability BLOCKS') and the docs that quote it: the Spec Kit layer must be updated by T081 and the owner ratification T081a must see it. The verbatim operator mandate of 2026-10-04 was read in full (Constitution.md lines 10578 and 10612 of the a71b1767 tree: 'If we do not have access to Opus model for some reason but only to Sonnet, we MUST USE what we have! Reviews MUST BE done. Ideally with Opus model, however Sonet MUST BE used if Opus for any reason cannot be used!'). UNCONFIRMED: the exact operative wording of the amended clause (A) (read only as heading plus mandate).
3. **§11.4.230 extended with clause (D)** INCREMENTAL DELIVERY GRANULARITY + DECLARED QA-BUILD CADENCE (D.1 to D.6): delivery unit is the increment, a declared QA-build cadence (per-increment, periodic, ...), continuous background hardening that never blocks the next increment, never-stale tracking as a hand-off precondition, QA findings on early increments still coverage escapes; the heading and forensic paragraph are extended; a process note reserves clause letter (E).
4. **§11.4.235 / §11.4.231 / others**: clause (D) finding-layer classifier (already in the e44f22f range) plus minor in-place edits; the status summary header of Constitution.md moves Revision 68 -> 72 (Last modified 2026-10-05T07:49:21Z).
5. **§11.4.272, §11.4.273, §11.4.274 mirror-gap fix** (commit 1901233 'close §11.4.157 lockstep gap — §11.4.272-274 missing from all 4 mirrors'): these three anchors existed in Constitution.md at the pin but were MISSING from CLAUDE/AGENTS/QWEN/GEMINI; the new tip carries them in all four. (So the lockstep census of lockstep.txt section C: 74 -> 78 fully-lockstep anchors.)
6. CHANGELOG entry helixconstitution-v69 (4d0172c) and regenerated .html/.pdf/.docx twins; README Rev 5; docs/owed_gate_implementations.md, docs/AGENT_GUARDRAILS.md, docs/codegraph/Status*.md updated.
Other tooling: scripts/fastcycle/cycle/cycle_report.py (+534), review_record.py (+136, optional finding_layer field per §11.4.235(D)), tests, fixtures; scripts/release_prefix.sh and scripts/hooks carrier-strip fixes #28 to #35 (false-positive fixes of the credential scanner). 4 nested gitlinks bumped (77db11f): claude-video, design-toolkit, polyscreen-mcp, verification (new nested commits NOT local, content UNCONFIRMED).
Not changed in e44f22f..a71b1767: constitution_index.yaml (still source_sha256 d915a5c1..., 283 anchors, generated_at 2026-09-26: STALE, see upstream-report.md), scripts/post_update_hook.sh, helix-deps.yaml, .gitmodules (diff stat names none of them).

## 4. What this means for the plan (findings)
F1. The plan's target tip is out of date: tasks.md T080 says be06384 and docs/12 14.3 step 6 says e44f22f; the live tip of all 8 remotes is a71b1767 (30 commits further). T080 must be re-read against a fresh ls-remote at execution time (the plan already says so: unique maximum of step 3). T077 therefore ran on BOTH e44f22f and a71b1767.
F2. §11.4.209/§11.4.211 now allow a Sonnet fallback, which this project's constitution.md and appendix deny. A bump to a71b1767 changes a binding review rule: owner ratification (T081a) must see it; the T084 reviewer substrate rule of the project (plan: Opus xhigh, unavailable -> blocked) is not changed by the bump by itself but would be by the Spec Kit update.
F3. The index lags the canon at BOTH tips (new anchor 11.4.276 absent from constitution_index.yaml at a71b1767; the index hash is the 10b7a06 hash). regen_speckit_catalogue.py states this in the files it writes.
F4. §11.4.276 introduces a review round budget that interacts with this project's own T011a / review-iteration practice: UNCONFIRMED how the plan's 'iterate to GO' tasks map to a declared budget; a consumer-declared value is DATA the owner would have to give (T081a).

## 5. Method and limits
* Read with `git -C submodules/constitution log --oneline/--format`, `diff --stat`, `diff <a> <b> -- <file>`, `ls-remote`, `merge-base --is-ancestor`; all on objects.
* The diffs of Constitution.md and the four mirrors were read as head-of-diff excerpts and by statistics, not line by line over the 99-line a71b1767 delta: statements about exact clause wording in section 3 are labelled UNCONFIRMED where made from an excerpt.
* Not read: the 80-file tooling delta beyond the stat (fastcycle tests, cycle_report.py, release_prefix.sh); whether any call site of this project uses them is UNCONFIRMED (the project calls none of scripts/fastcycle today, per the earlier readiness analysis).
