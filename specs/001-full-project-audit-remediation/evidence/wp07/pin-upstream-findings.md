# WP-07 pin move: upstream findings at constitution tip a71b1767 (to report to the constitution repository)

Identity: catalogizer / 001-full-project-audit-remediation / WP-07 T080 to T085 | Revision 1 | 2026-10-06 | draft, UNREVIEWED | agent wp07-pin | git HEAD 960c553a198d9c17218cb2ad4283a599bdc83b04 (tree uncommitted) (these files were generated while HEAD was a91860162b5f; the commits a9186016..960c553a touch none of the WP-07 files, checked by the WF8 reviewer; identity re-stamped by the WF8 fix pass, F6) | complements `upstream-report.md` (T078, same repository state)

| Field | Value |
|---|---|
| Revision | 1 |
| Last modified | 2026-10-06T18:40:00Z |
| Status | draft, UNREVIEWED |

Facts measured on `git -C submodules/constitution` objects of 10b7a06 and a71b1767; nothing was pushed to any constitution remote.

| # | Finding | Measurement |
|---|---|---|
| U1 | `groups/*.md` do not carry the amendments of the range | `git diff --stat 10b7a06 a71b1767 -- groups` is empty (0 files), although `Constitution.md` changes 8 existing anchors (11.4.134, 209, 211, 230, 231, 235, 240, 267) and adds 11.4.276. `grep -c 'Sonnet fallback' groups/code-review-and-quality.md groups/project-lifecycle-and-release.md groups/multi-track-and-parallelism.md` = 0, 0, 0; `grep -c 'INCREMENTAL DELIVERY GRANULARITY' groups/multi-track-and-parallelism.md` = 0; `grep -rnF '11.4.276' groups/` finds 0 lines (the anchor is present in `Constitution.md`, CLAUDE.md, AGENTS.md, QWEN.md, GEMINI.md, README.md and CHANGELOG.md of the constitution, not in any groups file). |
| U2 | `constitution_index.yaml` lags | byte-identical at 10b7a06, e44f22f and a71b1767 (see upstream-report.md): records the `Constitution.md` hash of the old canon, lacks 11.4.276; `gate_constitution_generate_no_drift.sh` stops at a71b1767 with `FATAL: anchor '11.4.276' matches no group rule` (rc 6). |
| U3 | Consequence for this project | the appendix digests for the 8 changed anchors and the new 11.4.276 were written from `Constitution.md` lines (see the `Source:` lines in the appendix), not from the groups files the appendix header names as its source. The header line was amended to say so. |
| U4 | Canon text is the authority | per `.specify/memory/constitution.md` precedence, canon (`Constitution.md`) wins; the groups files and the index are derived views. |

Ready text for the constitution repository (FR-006 route: fix in the shared module, with review; not filed here): "At a71b1767 the derived views lag the canon: groups/*.md lack the 2026-10-04 and 2026-10-05 amendments and 11.4.276; constitution_index.yaml records the old Constitution.md hash and lacks 11.4.276; gate_constitution_generate_no_drift.sh fails with UnclassifiedAnchorError (ID_RANGE_GROUPS has no range for 276)."
