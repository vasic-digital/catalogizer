# Dependency-Existence Verdict Register

| Field | Value |
|---|---|
| Revision | 4 (2026-10-22: rows appcrawler and rest_demo removed with their submodules by owner decision 2026-10-22; waiver WVR-CAT-2026-10-21-1 retired; CAT-DEP-001 closed) |
| Created | 2026-10-21T16:14:45Z |
| Last modified | 2026-10-22T09:05:00Z |
| Status | active |
| Status summary | 31 rows, VERIFIED=31 AMBIGUOUS=0 UNVERIFIED=0; no active waiver (section 5 records the retired one) |
| Issues | none |
| Issues summary | none open. The 2 former AMBIGUOUS rows (appcrawler, rest_demo) were removed with their submodules on 2026-10-22 (owner decision); waiver WVR-CAT-2026-10-21-1 retired, CAT-DEP-001 closed (section 5) |
| Fixed | n/a |
| Fixed summary | n/a |
| Continuation | Machine-readable twin: docs/dependency_existence_register.tsv; earlier PyPI/tool verdicts live in specs/001-full-project-audit-remediation/evidence/wp09/tool-existence.json |

## Table of contents

1. [Rules](#1-rules)
2. [Method and control needles](#2-method-and-control-needles)
3. [Register](#3-register)
4. [Findings: AMBIGUOUS and gaps](#4-findings-ambiguous-and-gaps)
5. [Waiver under canon section 11.4.271](#5-waiver-under-canon-section-114271)

## 1. Rules

Canon section 11.4.270: every proposed or adopted dependency carries one register row with a closed-set existence verdict (VERIFIED, AMBIGUOUS, UNVERIFIED) and citable evidence. Adoption is refused while the verdict is not VERIFIED. Existence is not fitness: a VERIFIED row proves only that the repository resolves and the pinned commit is real, not that the dependency works or is secure. Org class follows canon section 11.4.79 (own-org = vasic-digital or HelixDevelopment; everything else third-party).

VERIFIED requires all of: (a) `git ls-remote <url> HEAD` returned a ref; (b) `git -C <checkout> cat-file -t <pin>` = commit; (c) README identity matches the intended project. AMBIGUOUS: existence is proven but identity or provenance is in doubt. UNVERIFIED: any existence check failed or was not run. Licence is recorded as a status column and does not change the existence verdict; a missing licence is a separate adoption finding.

## 2. Method and control needles

Checked 2026-10-21T16:14:45Z (UTC), sequential `timeout 20 git ls-remote`, read-only. Positive control: the llmctl row returned a ref. Negative control (not in the register): URL `git@github.com:vasic-digital/this-repo-does-not-exist-xyz-9f3.git` with pin 0000000000000000000000000000000000000000 against checkout submodules/containers produced `ls-remote` failure and `pin_local=missing`, so both checks can FAIL (output recorded in the task report).

## 3. Register

| name | org | source | verdict | pinned sha | remote HEAD | pin=HEAD | licence (read from tree) | note |
|---|---|---|---|---|---|---|---|---|
| llmctl | own-org | constitution-hub | VERIFIED | `f42bad68325e` | `f42bad68325e` | yes | MIT [LICENSE] | re-checked 2026-10-22T00:21:13Z after canon pin `a3d80ce`: ls-remote HEAD returned `f42bad6`; pinned commit present locally as commit object; README identity consistent |
| claude_toolkit | own-org | constitution-hub | VERIFIED | `1f6dad3bca8a` | `1f6dad3bca8a` | yes | none (package.json: ISC) | re-checked 2026-10-22T00:59:54Z after canon pin `5f8503b`: ls-remote HEAD returned `1f6dad3` (the pushed flatten); pinned commit present locally as commit object; README identity consistent; nests only third-party `submodules/go`, ships `helix-deps.yaml`; no licence file in tree |
| open_design | third-party | constitution-hub | VERIFIED | `3b0df90f305c` | `8d820a698908` | no | Apache-2.0 [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| containers | own-org | project-flat(+hub pin 919ad1e) | VERIFIED | `4a8f04e05f35` | `919ad1e2026d` | no | none | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent; README states MIT; no LICENSE file in tree; hub pin 919ad1e also exists (cat-file=commit) but remote HEAD differs from project pin 4a8f04e |
| superspec | third-party | project-flat | VERIFIED | `c20ac6c1ba06` | `c20ac6c1ba06` | yes | MIT [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| design-toolkit | own-org | constitution-hub | VERIFIED | `4ddc9645d140` | `4ddc9645d140` | yes | none | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent; no licence file in tree; package.json says ISC |
| scrcpy | third-party | helix_qa-flat-tool | VERIFIED | `2322868e9e25` | `a60891aea193` | no | Apache-2.0 [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| allure2 | third-party | helix_qa-flat-tool | VERIFIED | `db2dbd8166fc` | `b2c4f3635fde` | no | Apache-2.0 [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| leakcanary | third-party | helix_qa-flat-tool | VERIFIED | `27b51013ce4a` | `eb60b0be5373` | no | Apache-2.0 [LICENSE.txt] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| docker_android | third-party | helix_qa-flat-tool | VERIFIED | `28478f3bce03` | `e46227886992` | no | Apache-2.0 [LICENSE.md] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| appium | third-party | helix_qa-flat-tool | VERIFIED | `0e4ecc49d6ae` | `f0d549c46ce7` | no | Apache-2.0 [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| midscene | third-party | helix_qa-flat-tool | VERIFIED | `4d37b88b2fe7` | `09799774791f` | no | MIT [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| mem0 | third-party | helix_qa-flat-tool | VERIFIED | `8d6b7c1d671a` | `2c81af3bbe27` | no | Apache-2.0 [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| moondream | third-party | helix_qa-flat-tool | VERIFIED | `6eccfceaf1aa` | `6eccfceaf1aa` | yes | Apache-2.0 [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| ui_tars | third-party | helix_qa-flat-tool | VERIFIED | `582f3a7ea5d2` | `582f3a7ea5d2` | yes | Apache-2.0 [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| perfetto | third-party | helix_qa-flat-tool | VERIFIED | `3ea71e6ee738` | `49aa3f5fd3d6` | no | Apache-2.0 [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| chroma | third-party | helix_qa-flat-tool | VERIFIED | `cbec464a012d` | `ddc17cd7c508` | no | Apache-2.0 [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| shortest | third-party | helix_qa-flat-tool | VERIFIED | `45a8dcfe8999` | `45a8dcfe8999` | yes | MIT [license.md] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| marker | third-party | helix_qa-flat-tool | VERIFIED | `5684e7655303` | `1749c21dcc36` | no | GPL(see file) [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| kiwi_tcms | third-party | helix_qa-flat-tool | VERIFIED | `1dba1ed1d096` | `a0672ff8709b` | no | GPL(see file) [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| testdriverai | third-party | helix_qa-flat-tool | VERIFIED | `42d2bc8004df` | `97d4d35d7727` | no | none | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent; no LICENSE file; package.json "license": ISC |
| stagehand | third-party | helix_qa-flat-tool | VERIFIED | `7224376585d6` | `d16562dafc40` | no | MIT [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| unstructured | third-party | helix_qa-flat-tool | VERIFIED | `f6eea758911f` | `9a376447d236` | no | Apache-2.0 [LICENSE.md] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| redroid | third-party | helix_qa-flat-tool | VERIFIED | `eba9a489990f` | `eba9a489990f` | yes | none | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent; no LICENSE file; README License section: Apache License (redroid-doc repo, docs/builder scripts) |
| signoz | third-party | helix_qa-flat-tool | VERIFIED | `7646aabb2b84` | `e69d3fcc8ce9` | no | MIT [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| docling | third-party | helix_qa-flat-tool | VERIFIED | `c7b1734217fc` | `29caea8381e8` | no | MIT [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| llama_index | third-party | helix_qa-flat-tool | VERIFIED | `5891d5f4f3e9` | `619d74664e84` | no | MIT [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| browser_use | third-party | helix_qa-flat-tool | VERIFIED | `c8e6e8f0fe61` | `aa093a843da5` | no | MIT [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| skyvern | third-party | helix_qa-flat-tool | VERIFIED | `d30575bb9a06` | `3dcc56776733` | no | AGPL-3.0 [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| anthropic_quickstarts | third-party | helix_qa-flat-tool | VERIFIED | `f37f1685e256` | `9ec32b91df50` | no | MIT [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |
| ui_tars_desktop | third-party | helix_qa-flat-tool | VERIFIED | `e9f3387288da` | `3ac2cb8d946d` | no | Apache-2.0 [LICENSE] | ls-remote HEAD returned a ref; pinned commit present locally as commit object; README identity consistent |

Full URLs, README first lines and per-row timestamps: docs/dependency_existence_register.tsv.

## 4. Findings: AMBIGUOUS and gaps

- appcrawler and rest_demo: REMOVED 2026-10-22 (owner decision). Both rows were AMBIGUOUS (appcrawler: unofficial personal repo `nicetester/AppCrawler`, no licence in tree; rest_demo: NiceHash REST API client examples under a generic name, no licence in tree). Investigation before the decision: nothing in Catalogizer or `helix_qa` executes either repository; `helix_qa` listed them only in `docs/flat-tools-manifest.tsv` (rows 23-24) and in docs, and its resolver tests used `rest-demo` only as a fixture name. Both submodules are de-registered here and dropped from the `helix_qa` manifest (27 -> 25 rows); their `.git/modules` directories are kept as a backup outside the repository.
- Licence not found in tree for: claude_toolkit, design-toolkit, containers (README says MIT), testdriverai (package.json ISC), redroid (README says Apache). These are licence findings, not existence failures.
- Hub vs project pin: constitution hub pins containers at 919ad1e while the project flat pin is 4a8f04e; both objects exist locally. The ls-remote HEAD moved past the pin for most rows, which is expected for pinned dependencies.
- open_design is third-party (nexu-io), Apache-2.0, pinned 3b0df90f305c8279e748c292accf336d02b924ef.

## 5. Waiver under canon section 11.4.271

**RETIRED 2026-10-22.** The owner decided on 2026-10-22 to remove `submodules/appcrawler` and `submodules/rest_demo` completely (section 11.4.122 keep-or-remove decision: remove), so there is nothing left for WVR-CAT-2026-10-21-1 to waive and no active waiver remains in this register. CAT-DEP-001 is CLOSED, disposition: removed by owner decision 2026-10-22 (step (3) of the owed work below, the remove branch); evidence: the investigation in section 4, the removed rows, and the `helix_qa` manifest change (27 -> 25 rows). The text below is the record of the waiver as it stood (history, not rewritten).

Canon section 11.4.270(C) refuses adoption while a verdict is AMBIGUOUS. The operator decided in this session to KEEP `submodules/appcrawler` and `submodules/rest_demo` as flat gitlinks (nothing removed, section 11.4.122), so the refusal is lifted for these two rows only through a waiver that has the three parts canon section 11.4.271 requires. The verdict columns stay AMBIGUOUS; the waiver is additional columns in the TSV (`waiver_id`, `waiver_authoriser`, `waiver_expiry`, `waiver_tracked_item`, `waiver_scope`).

| waiver | rows | authoriser | expiry | tracked item |
|---|---|---|---|---|
| WVR-CAT-2026-10-21-1 | appcrawler, rest_demo | operator, roster identity = this repo's git identity: user.name "Милош Васић", user.email i@mvasic.ru (handle milosvasic) | 2026-11-20 (30 days from 2026-10-21) | CAT-DEP-001 |

- Authoriser is the operator and not the producer of the change (the change is produced by the agent). UNCONFIRMED: this project has no separate authoriser roster file; the git identity above is the roster entry recorded here, as the operator instructed.
- CAT-DEP-001, owed work: (1) confirm the upstream identity of both repositories (appcrawler: canonical upstream of the declared `nicetester/AppCrawler`, an unofficial personal repo; rest_demo: `nicehash/rest-clients-demo`, NiceHash REST API client examples under a generic name, intent as a test-app fixture); (2) confirm the licence of both (neither carries one in the tree); (3) re-verdict each row VERIFIED, or remove it through an explicit operator keep-or-remove decision (section 11.4.122). If the work is not done by 2026-11-20 the waiver lapses and both rows revert to refused adoption (canon section 11.4.271(C)); a renewal without closing the debt is itself a finding.
- Tracked-item store: CAT-DEP-001 is an id minted by this change, by hand, with no workable-items store behind it: this project has no workable-items database in the working tree (searched docs/ and specs/001-full-project-audit-remediation/ for a store; none found), so the id is not engine-issued, is not checked for uniqueness or monotonicity by any tool (canon section 11.4.54), and must be imported into a store once one exists. It is recorded here (this register is its record), not in a tracker. The operator decision itself was given interactively in this session; it is not yet in `specs/001-full-project-audit-remediation/decisions/owner-decisions.yaml`, which is importer-governed (revision 5) and was left untouched. The key CAT follows owner decision ODG-11 (this project's key).
- Scope: the waiver covers keeping the two gitlinks at the recorded pins. It does not turn the verdict into VERIFIED, and it grants no waiver for any other row.
