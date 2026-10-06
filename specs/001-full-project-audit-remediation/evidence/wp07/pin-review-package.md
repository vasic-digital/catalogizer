# WP-07 pin move: review package for the independent reviewer (T084 G-PIN review, input only)

Identity: catalogizer / 001-full-project-audit-remediation / WP-07 T080, T081, T083 (T082, T084, T084a, T085 not done) | Revision 1 | 2026-10-06 | draft, UNREVIEWED, NOT SELF-APPROVED | agent wp07-pin | repository HEAD 960c553a198d9c17218cb2ad4283a599bdc83b04, working tree uncommitted (these files were generated while HEAD was a91860162b5f; the commits a9186016..960c553a touch none of the WP-07 files, checked by the WF8 reviewer; identity re-stamped by the WF8 fix pass, F6) | pin 10b7a06c4a2ec3f06b4cde9b1611a622a79320ad -> a71b1767b40229c8e88140924352fbe990b3049e (fast-forward, 55 commits)

| Field | Value |
|---|---|
| Revision | 2 (WF9 pin-fix 2, finding G5: the body was stale after the WF8 fixes and the WF9 review; sections 1, 3 and 4 are rewritten below, section 7 point 2 and the G9 recovery statement are corrected; the Revision 1 text that contradicted the fixed files is replaced, not kept) |
| Last modified | 2026-10-06T19:37:16Z |
| Status | draft, UNREVIEWED; the constitution pin move itself is committed at 40a1f8e3 (the tree is no longer uncommitted for the pin files; the WF9 minor-finding fixes below are an uncommitted working-tree change on top of it) |

No commit, stage, push or force-push was made by this step (§11.4.113); history untouched. The verdict file `$EV/reviews/WP-07.json` does not exist and is not created here.

## Revision 2 note (read this first)

Revision 1 of this package described the tree as it was before the WF8 fix pass and its text for sections 1, 3 and 4 no longer matched the files
(it said the project rule is "stricter than canon" and "stands", gave the old hashes and "17 of 17"). The independent re-review WF9 (finding G5)
asked for the body to be refreshed. Revision 2 does that: section 1 lists the current files and hashes, section 3 the current results, section 4 the
corrected conflict wording. Where an older sentence below is kept, it is an unchanged fact (the pre-conditions of section 2, the not-done list of
section 5, the register text of section 6). Evidence of the WF9 minor-finding fixes: `pin-fix2-*` in this directory.

Recovery statement (WF9 G9, corrected): HEAD 40a1f8e3 plus the §9.2 backup `.audit/backups/wp07-pin-fix2-20261006T192945Z` (the committed versions of every file the
fix pass edits, `files.sha256` verified) plus git cover recovery of the committed state. The intermediate working-tree drafts that were reviewed
by WF8 (for example the appendix `73ee0bba`) were never committed and are in none of them; only their text in the review transcripts remains.

## 1. What changed (exact paths; revision 2, current hashes)

| Path | Change | sha256 now |
|---|---|---|
| `submodules/constitution` (gitlink) | `10b7a06` -> `a71b1767`, committed at 40a1f8e3 (unchanged by the fix pass) | gitlink `a71b1767b40229c8e88140924352fbe990b3049e`; `Constitution.md` 95117cf3d96e4bb144d4e4cfed4031392abd91e22d55c95e6eb5aed940238a4e |
| `.specify/memory/constitution.md` | Revision 8: owner-record citations corrected (ODG-19 is the plan's recommendation, OA-2026-10-05-23 the conductor-relayed answer, both with `owner-decisions.yaml` locators; no UNCONFIRMED locator), both Sonnet-fallback flags named, the Known Conflicts preamble covers items 17 and 18, the stale generated catalogue titles of 11.4.209 and 11.4.211 are stated as index lag, anchor counts stated truthfully (283 listed by the index, 284 in the canon); generated catalogue sentence regenerated | 0b541911102801caa996cef3825584b70c73cb5e1ba4d8394b3a43cefc8352fa |
| `.specify/memory/constitution-appendix.md` | Revision 4: digest wording restored from canon (11.4.230 four invariants, clause (D) Composes list, (D.2) MUST wording; 11.4.209 "undone / blocked / deferred" and the last sentence of the quoted mandate); owner-record citations in the 11.4.209 and 11.4.211 notes; the 276 Source line 11928-11965; header counts (283 canon blocks compared, 1,814 tokens, no excluded id) | 066d9f6719612849dca77229ea72559a02f2d931365fdcd462528fc70c5db8e1 |
| `specs/001-full-project-audit-remediation/decisions/owner-request-list.md` | Revision 7: OD-WP07-SONNET-FALLBACK paragraph cites ODG-19 and OA-2026-10-05-23 as the records hold them | 6b37e5ed54f424a94bcb743c49980edd92645fc9be0618f04ba545587c83bfbc |
| `scripts/governance/appendix_token_check.py` | covers every dotted anchor id (no exclusion), adds `--floor`, `--phrases`, `--write-floor` | 87f6f908463cf067874ffd66ef7c22c53e037b26171324a77cb5d030a9cb89d8 |
| `scripts/governance/appendix_wording_floor.tsv`, `appendix_required_phrases.tsv` | NEW data files of the wording floor (283 rows) and the required phrases (11 rows) | see the identity header of any `pin-fix2-*` file |
| `scripts/governance/regen_speckit_catalogue.py` (and the two golden fixtures) | the catalogue sentence reads "All N anchors the machine index lists" | 0991bf4a29a9690d9b00c173f6a46f89ff84ed0e511c0d73dd9993cfc02713eb |
| `docs/scripts/appendix_token_check.md`, `docs/scripts/regen_speckit_catalogue.md` | revisions 2 and 3 | see headers |

Not changed, on purpose: root `CLAUDE.md`, `AGENTS.md`, `GEMINI.md`, `tasks.md`, `scripts/register`, `scripts/build`, `scripts/repo`, `tools/evidence`, `docs/scripts/README.md`
(it carries another stream's uncommitted work; the two guides are therefore still not linked from it, WF9 G8, owed to its owner).

## 1a. What changed at Revision 1 (history, superseded by the table above)

| Path | Change | sha256 after |
|---|---|---|
| `submodules/constitution` (gitlink) | `10b7a06` -> `a71b1767`; working-tree submodule moved by `git -C submodules/constitution merge --ff-only a71b1767b40229c8e88140924352fbe990b3049e` (ORIG_HEAD = 10b7a06, `pin-merge.txt`), then `git -C submodules/constitution submodule update --init --recursive` | gitlink `a71b1767b40229c8e88140924352fbe990b3049e`; `Constitution.md` 95117cf3d96e4bb144d4e4cfed4031392abd91e22d55c95e6eb5aed940238a4e |
| nested engine checkouts under the constitution | the range moves 4 nested gitlinks, all objects were already local (no fetch): `claude-video` 03ceb42 -> 83da59f, `design-toolkit` 4ddc964 -> efd2c3f, `polyscreen-mcp` f2ed241 -> d0817cf, `verification` 9f06855 -> 8dcd729; `update --init --recursive` checked each out (`nested-update.txt` before and after) | no pointer drift row: `git status --short submodules` shows only ` M submodules/constitution` |
| `.specify/memory/constitution.md` | regenerated pinned paragraph and catalogue by `scripts/governance/regen_speckit_catalogue.py` (sha256 ac962b7d...); hand edits: revision header table (§11.4.44, Revision 6), Principle III project-stricter-than-canon note, Known Conflicts item 8 reworded, items 17 and 18 added, version 2.1.2 -> 2.2.0 and Last Amended 2026-10-06 | 413cb9f32fbbec8c4d68f0f4ec049a2cbf1ee8be050344750eb21c179bbc44e0 |
| `.specify/memory/constitution-appendix.md` | Canon pin row regenerated (script); hand-written digests (script `pin-raw/pin-raw-edit-appendix.py`, asserts every replaced text): 11.4.134 EXTENSION, 11.4.209 and 11.4.211 rewritten with the amended (A) and the project extension line, 11.4.230 clause (D) with (D.1) to (D.6), 11.4.231 (E) and (F.3), 11.4.235 clause (D) and gates, 11.4.240 (F)(2), 11.4.267 (E) EXTENSION, NEW 11.4.276; header Revision 2, Last modified 2026-10-06T18:20:00Z, "Generated from" line states the index lag | 73ee0bba1ac7203af6a90b8fda58a39a179889ba36a6ee81a857b24a4911e82e (1,042,417 bytes, under the 4 MiB governance-carrier bound; its one `PRIVATE KEY` line, line 1489, is unchanged from HEAD) |
| `specs/001-full-project-audit-remediation/decisions/owner-request-list.md` | one paragraph in section 4.2: owner question OD-WP07-SONNET-FALLBACK (not decided) | 05927a19ec719eee5c5f055569b428530d2816e11c53476786258d7c625fdaf4 |
| `specs/001-full-project-audit-remediation/evidence/wp07/` | new files: `tips-before-merge.txt`, `pin-held.txt`, `nested-update.txt`, `pin-merge.txt`, `sweep-pin.json`, `pin-upstream-findings.md`, `pin-t081a-ratification-citation.md`, `pin-review-package.md`, `pin-raw/` (15 files) | listed in `pin-raw/` and sweep-pin.json |

Not changed, on purpose: root `CLAUDE.md`, `AGENTS.md`, `GEMINI.md` (they carry no anchor block and cite no pin or hash; a grep for 10b7a06, e44f22f, a71b1767, 11.4.209 and 11.4.211 finds nothing in them; there is NO root `QWEN.md`); `tasks.md`; `scripts/register`, `scripts/build`, `scripts/repo`, `tools/evidence`, compose and Dockerfiles. `scripts/repo/private_key_carriers.tsv` does not exist yet (T040a/T040b), so no carrier-row change was possible: UNCONFIRMED whether S2 passes the regenerated appendix's `PRIVATE KEY` line (it is unchanged, so a row that passes it at HEAD passes it now).

## 2. Pre-conditions measured (T080)

* Live tips (`tips-before-merge.txt`, 2026-10-06T18:12Z, `git ls-remote <remote> refs/heads/main`, tracking refs not trusted): all 8 remotes read a71b1767. The tasks.md targets e44f22f and be06384 are both ancestors of it; the target was extended to the live tip per the owner's FR-017 answer and decision ODG-13/ODG-12.
* `pin-held.txt`: `merge-base --is-ancestor a71b1767 <tip>` holds for all 8 remotes.
* Backup (§9.2): the predecessor backup `.audit/backups/wp07-20261006T164710Z` re-verified (`sha256sum -c files.sha256`: 0 mismatches); a fresh copy of the 6 files this step edits is at `.audit/backups/wp07-pin-20261006T181153Z` with its sha256 list. The submodule move is a fast-forward (no commit lost; ORIG_HEAD kept).
* Range review: the predecessor's `range-review.md` covers 10b7a06..e44f22f and the tip table; this step read the full 10b7a06..a71b1767 diff of `Constitution.md` (23 hunks): the eight amended anchors and the new 11.4.276 above. UNCONFIRMED: the non-canon part of the range (fastcycle tooling, tests, 4 nested pointer moves) was not read file by file here.

## 3. Gates and checks (T083 run A; `sweep-pin.json` and `pin-raw/`; Revision 1 results, superseded where the next block says so)

Current results (revision 2, evidence `pin-fix2-*`): `regen_speckit_catalogue.py check` rc 0 on both real files; `test_regen_speckit_catalogue.py` 21 of 21 PASS x3;
`test_appendix_token_check.py` 53 of 53 PASS x3; the real token run: `canon_blocks=283 appendix_digests=283 tokens_checked=1814 gaps=0 no_digest=0 exempt=6 stale_exemptions=0`
with `weakened=0 compose_lost=0 phrase_gaps=0`; 34 code mutants and 19 data mutants (D1 the documented equivalent) as the mutation files record.
The Revision 1 table below is kept for the checks this fix pass did not touch (the constitution propagation family, ledger ratchet, anti-bluff scan, inheritance verifier); its
"17 of 17" regen-test count and the two `.specify` revision-header lines are superseded by the numbers above.

| Check | Result |
|---|---|
| `regen_speckit_catalogue.py check` on the real `.specify` files | exit 0, both files in sync (the INDEX-LAG note names 11.4.276) |
| `test_regen_speckit_catalogue.py` | 17 of 17 PASS |
| `check_revision_headers.sh` on both `.specify` files | exit 0 |
| `test_decision_intake.sh`, `test_no_atm_prefix.sh` | exit 0, exit 0 |
| constitution propagation family, top-level carriers, extracted at each tip | 82 of 82 PASS at 10b7a06, 86 of 86 PASS at a71b1767 (4 new wrappers) |
| same family on the real checkout with nested consumers | 86 of 86 FAIL; 82 of 82 FAIL at the old pin (predecessor lockstep.txt A2): pre-existing |
| `cm_gate_ledger_ratchet.sh` on the real checkout | PASS, unimplemented=394, baseline=402 (extraction FAIL 517/518 vs 402 is an artefact, see sweep-pin.json) |
| `anti-bluff-scan.sh` | 184 findings, exit 1, identical old and new for the constitution tree (10 and 10) and for the 4 moved nested submodules (4 and 4): no delta from the move |
| `verify_constitution_inheritance.sh` | exit 1, ONE FAIL (INV2), the SAME FAIL on an old-pin control tree: an instrument defect (SIGPIPE under pipefail), root cause proven by `pin-raw-inv2-sigpipe-probe`; see section 6 |

Run A verdict as written in sweep-pin.json: no FAIL beyond the baseline measured at the old pin; one baseline FAIL remains that T083 says is never a pass claim. Reviewer and owner judge GREEN for T085.

## 4. The conflict note (T081, instruction 4): canon Sonnet fallback versus the project's earlier Opus-only text (corrected at Revision 2)

* Canon at a71b1767 (11.4.209 code review, 11.4.211 merge-conflict resolution, amended 2026-10-04): Opus at `xhigh` is the PRIMARY substrate, and when Opus is genuinely unavailable (captured fact) the work MUST run on Sonnet instead of being blocked; blocked only when BOTH are unavailable; Fable and Haiku never; leaving it blocked while Sonnet is reachable is the forbidden escape hatch `--leave-review-blocked-while-sonnet-reachable` (merge side: `--leave-merge-blocked-while-sonnet-reachable`). 11.4.231 (E), (F.3), 11.4.235, 11.4.240 (F)(2) carry the matching wording.
* The project's earlier text (Principle III and the appendix 11.4.209 and 11.4.211 digests at the old pin) said Opus `xhigh` only, "BLOCKED, never substituted". That digested the PRE-amendment canon and is NOT an owner mandate. The records: ODG-19 (`owner-decisions.yaml` lines 355-358, status Operator-blocked) holds the plan's options ("Opus at xhigh as pinned; other") and recommendation ("as pinned (§11.4.209)"), which are not an owner answer; the owner's recorded answer is OA-2026-10-05-23 (line 1216, relayed by the conductor, not verbatim): "run independent reviews through the Workflow path with Opus xhigh and record effort". Neither says "no fallback" or "blocked".
* Handling (corrected): a project rule MAY tighten canon and MUST NOT weaken it; a rule that blocks where canon REQUIRES a fallback would weaken it, so the "stricter rule stands" wording of Revision 1 was wrong and is withdrawn. Principle III, Known Conflicts item 17 and the 11.4.209 and 11.4.211 appendix notes now state canon accurately, label the interim practice "an agent's interim choice and NOT an owner decision" (follow canon: Opus `xhigh` first, the Sonnet fallback only on a captured Opus-unavailability fact, recorded with model and effort), and name the open question.
* Not decided here: owner question OD-WP07-SONNET-FALLBACK (`decisions/owner-request-list.md` section 4.2), default "follow canon"; the only alternative to following canon is asking for a canon change. It still needs an explicit owner answer before the T085 push.
* Residual: none for agent behaviour (the project text now agrees with canon). The generated catalogue titles of 11.4.209 and 11.4.211 in `constitution.md` remain the index's pre-amendment titles (index lag) and are stated as stale in Principle III and item 18.

## 5. Not done and why

* T081a: the ratification record is NOT created (needs the T084 GO); the citation of the owner's blanket phase approval is in `pin-t081a-ratification-citation.md`, marked provisional, with the UNCONFIRMED limits.
* T082 (post_update_hook.sh, variant B): NOT run. (1) tasks.md orders it after T085 (the pin must be pushed first, so no hook output can sit below the commit carrying `WP-07.json`); T085 needs T084 GO and the T081a record, neither exists. (2) The hook calls `install_cli_agent_plugins.sh`, which writes the project `.claude/skills/`, `.claude/settings.json` and, when a `claude` binary is on PATH, registers a plugin in `$CLAUDE_CONFIG_DIR`; variant B (PATH without the claude binary) would skip the last part, but this agent session cannot prove its own PATH and `CLAUDE_CONFIG_DIR` conditions without touching shared host state, and the before and after hashes of `.claude/` and `.git/hooks` are not taken. Status: BLOCKED on T085 (and on proving variant B conditions at run time). T084a depends on it.
* T080 commit (CPA `cpa-host --awaits-review`), T084 verdict, T085 push: not mine (no commit by this step).
* T078 register item and the constitution-repository report: text only (`upstream-report.md`, `pin-upstream-findings.md`); the register is another stream's.
* `meta_test_constitution_inheritance.sh`: not run (it mutates `Constitution.md` in the submodule).

## 6. Ready register text for the one new FAIL (T083 says: filed as a tracked item, Status Queued, Type Bug; the register is not touched here)

Title: `verify_constitution_inheritance.sh` INV2 reports a false FAIL: `printf | grep -qF` under `set -o pipefail`.
Body: scripts/verify_constitution_inheritance.sh line 92 pipes a 35 KB variable into `grep -qF`; `grep -q` exits at the first match, `printf` gets SIGPIPE, pipefail makes the condition false, so INV2 reports the §11.4 anchor heading MISSING although line 454 of Constitution.md holds it at both 10b7a06 and a71b1767. The comment above the line names this exact failure and the code still has it. Probe: here-string form passes 100/100, printf-pipe form 0/100, absent-needle control matches 0 (`pin-raw-inv2-sigpipe-probe.out`). Reproduced in IMG-TESTUTIL on the real tree and on extracted old-pin and new-pin trees. Fix direction (not applied): `grep -qF "$A" <<<"$INV2_HEADINGS"` or `grep -E ... | grep -qF` replaced by one grep over the file; its paired meta-test must keep detecting a stripped heading. Class: §11.4.201 false-positive refusal.

## 7. Open points for the reviewer

1. Confirm the appendix digests are faithful to `Constitution.md` lines 9110-9111, 10576-10596, 10610-10622, 10986-11004, 11025-11027, 11113-11117, 11210-11214, 11730, 11928-11965 (the `Source:` lines name them). Nothing was invented; clause (D) of 11.4.230 and 11.4.276 are long and were condensed.
2. The generated catalogue in `constitution.md` still shows the old titles of 11.4.209 and 11.4.211 ("no fallback model"), because it is generated from the lagging index; Principle III and Known Conflicts item 18 (revision 2) now say that these titles are stale and that Principle III controls. Decide whether that is acceptable until upstream regenerates its index.
3. The `Revision` of `constitution.md` was set to 6 from the 5 prior commits of the file (`git log`); the header table is new (§11.4.44).
4. No `QWEN.md` exists at the repository root: T081 names it among the carriers; nothing to update, recorded as a gap.
