# Owner request list (tasks.md T013, DRAFT for HC-0)

| Field | Value |
|---|---|
| Revision | 7 (draft, 2026-10-06, WF9 pin-fix pass 2 (minor findings G1 and G7): the OD-WP07-SONNET-FALLBACK paragraph cites ODG-19 as the plan's recommendation and OA-2026-10-05-23 as the conductor-relayed answer with their `owner-decisions.yaml` locators, drops the UNCONFIRMED-locator caveat and names both canon flags; no other section changed); revision 6 (draft, 2026-10-06, WF8 pin-fix pass: the OD-WP07-SONNET-FALLBACK paragraph states canon accurately (REQUIRES the Sonnet fallback), drops the unsupported owner-mandate attribution and records an interim practice; no other section changed); revision 5 (draft, 2026-10-06, round 4 after the independent review WF3 governance: the ODG-11 precedence is one statement, and four owner items are added (items 14 to 17); revision 4 (draft, 2026-10-05, batch 3 and batch 4 answers folded in: items the owner answered are no longer listed as open; revision 3 added the CAT-prefix deviation, revision 2 followed the interactive and batch-2 answers) |
| Last modified | 2026-10-06T19:37:16Z |
| Status | DRAFT in the working tree; not committed; for the owner at HC-0 (T014) |
| Source | tasks.md T013; docs/21 section 8 (the decision rows) and section 12.2; spec.md Clarifications; `progress.yml` (the relayed 2026-10-05 answers) |
| Owner decisions file | `decisions/owner-decisions.yaml` (T012, revision 3): it carries every relayed 2026-10-05 answer as an `owner_answers` entry that names its `progress.yml` source; the answers are recorded verbatim by T012a after HC-0 |
| Values policy | no credential value, key, token or password appears anywhere in this file; credentials are named by environment-variable NAME only |

How to read this list. Every answer below marked "relayed 2026-10-05" was relayed to this list by the conductor from the owner's answers (in meaning, not verbatim quotations). It is not yet recorded verbatim in `$EV/hc/HC-0.json`: please confirm it at HC-0, or correct it. A row with a blank is still open. A row left unanswered stays an `Operator-blocked` register item; the plan proceeds only where this list says it can. Everything marked UNCONFIRMED is a statement I could not verify.

## 0. What is still open for you (the short list)

1. NFS host (ODG-08): answered for now: a user-space NFS server only. Real NFS host proof stays UNMET until you name a host; nothing is asked unless you decide to supply one. (Earlier, batch 2, you had chosen both; the batch-4 answer narrows it.)
2. Firebase (OD-43, OD-45 and the part of ODG-01 that concerns `google-services.json`): answered: use the existing project `catalogizer-7a3f1`. Still yours: run the Firebase CLI login once (owner action); I ask before enabling or creating anything new. OPEN until the login is done.
3. Meaning of the relayed answers that are not yet complete (details in the sections below): ODG-05 acceptance scope; ODG-17 SC-005 sample size and register category mapping; ODG-40 server-image/tool lists and per-repository licences; ODG-41 which of options (a), (b), (c) you meant; ODG-42: you answered per-class report in phase 2 (T525), then decide (UNCONFIRMED: T525 concerns ODG-43 only, so whether ODG-42 waits on it stays open).
4. ODG-16 custody (section 2): you answered: the key in your keyring, locked by a passphrase only you enter at signing time; the honest strength is recorded as `policy`, not mechanism. The C2-custody slot stays empty and whether the passphrase gate holds against the agents is UNCONFIRMED; please confirm at HC-0.
5. ODG-02 and ODG-03: please accept, in your own words, the residual risk of leaving the credentials in history (section 1.2).
6. The `thinker.local` and `amber.local` hosts (ODG-07) and the §11.4.173 remote-distribution deviation: acknowledge the deviation (section 1.4).
7. The `CAT` register prefix (ODG-11) deviates from the constitution: the constitution (11.4.54, 11.4.248) mandates the ATM prefix, and, by your answer, this project's `CAT` is an owner-approved exception to that literal, recorded, with the upstream change request pending (the exception conflicts with 11.4.54 and 11.4.248, which have no escape hatch, and with the CLAUDE.md precedence rule; it stays OPEN until you confirm it at HC-0 and the upstream change is accepted). You answered: keep `CAT` and request an upstream constitution change (a per-project prefix setting); until accepted this is recorded as an owner-approved exception. OPEN: the upstream change request is still to be prepared (I prepare it as a separate request; you approve it). You also answered yes to renaming the lower-case column names to `cat_id` in this project's register design; how the constitution engine accepts that (an adapter or an upstream change) is UNCONFIRMED. OWED: that rename is NOT applied yet and no task carries it; it is tracked as an owed item (item 15 below).
8. Devices (ODG-04): emulators only for now; send devices if you ever have them (section 1.1).
9. Floors adoption (reserved slot `floors-adoption`), IC-47 (section 4.6), the per-context licence lists (ODG-40), and optional answers on the 34 proposals (ODG-39).
10. The deferrals: ODG-14, ODG-23, ODG-42, ODG-43, OD-23, OD-49, OD-68, OD-75 wait for an investigation or a report; they are not decisions yet.
11. Evidence chain: you chose the constitution continuum chain. OPEN: the four points of the continuum engine are not yet answered by you: hash construction, record schema, exit codes, anchor format. The python chain of docs/06 stays interim until a container build exists.
12. Exceptions list: the list of recorded exceptions to the naming and layout conventions (for example the kebab-case script names) is still to be confirmed by you. UNCONFIRMED what the list must hold beyond the entries recorded in tasks.md.
13. Phase 0 close: you said to finish the reviews, pull the images, record HC-0, then ask you for the phase boundary approval. Nothing is asked now; the approval request follows HC-0.

14. Upstream wording for the register id prefix (ODG-11): a DRAFT change request, not sent and not filed, is at `docs/upstream/constitution-prefix-change-request.md` (it proposes a per-project prefix setting in place of the fixed literal in 11.4.54 and 11.4.248, and a migration plan). Please approve its wording, or correct it, and say whether you accept its option B: keep the engine's own id column and add a prefix-neutral view (the draft's recommendation; you have NOT accepted option B yet). The draft is the owner-approved route for the exception recorded in item 7; filing it is yours.
15. OWED: the lower-case identifiers `atm_id`, `new_atm_id` and `head_atm_id` rename to `cat_id` in this project's register design (you answered yes in batch 3). Nothing renames them yet and no tasks.md task carries it; the engine keeps its own column name, so the rename of the engine-owned column needs option B (views) or an upstream change; the project-owned names (`new_atm_id`, `head_atm_id`, `reg_ids.atm_id` in docs/04 and data-model.md) can be renamed in this project's design text once you accept option B. Until then the lower-case forms stay and `test_no_atm_prefix.sh` does not match them.
16. Question on the retained occurrences of the withdrawn prefix (you said it is to be removed from this repository's files): the files still carry it in these classes, counted 2026-10-06, and I need your confirmation of each class or a neutral rendering instead (for example a placeholder such as `<withdrawn>-001`): (a) captured command output of section 14 of docs/04, byte-true as the 2026-10-03 runs printed it (33 lines, marked); (b) every file under `specs/001-full-project-audit-remediation/evidence/` (the before-inventory, RED output and mutation logs quote it; about 27 files were counted); (c) the decision-history text of the intake, the request list, `progress.yml`, docs/21 and research.md that quotes your own ODG-11 answer and the question as asked (exact quoted fragments, counted by the test); (d) the constitution's own gate name for complete ticket ids, kept verbatim in the two ledger files (`scripts/ledger/`) for the ledger ratchet; (e) the read-only `.specify/memory` copies of the constitution; (f) the upstream draft of item 14, which must quote the constitution's text it asks to change (30 lines, marked as a constitution quotation). Retained occurrences are the price of not editing captured evidence; rewriting them would make fabricated evidence.
17. Evidence chain (decision DR-E1): the continuum decision brief is at `docs/upstream/continuum-decision-brief.md` (four questions: hash construction, record schema, exit codes, anchor format; options with costs and one recommendation). Please read it and answer, or answer the single DR-E1 question; items 11 and this one are the same decision.

## 1. Inputs only you can supply

### 1.1 Devices (ODG-04; blocks WP-54, WP-60, WP-61, WP-62, WP-71)

Relayed 2026-10-05: "emulators only for now; device-class claims UNMET until devices supplied". Without a device of a class, claims for that class are recorded `blocked-unavailable` and the feature cannot complete for it (docs/10 R-10-2). If you later supply devices, give model, serial and how each connects (serials are device identifiers, not secrets):

| Class | Needed | Your answer (only if you supply one) |
|---|---|---|
| Phone | one | |
| Tablet | one | |
| Android TV box | one | |

The emulator acceptance scope (ODG-05) is asked in section 5.

### 1.2 Credentials by environment-variable NAME only (ODG-01, ODG-02, ODG-03; blocks WP-13, WP-24, WP-35, WP-38, WP-50, WP-54, WP-60, WP-61, WP-62)

Relayed 2026-10-05: ODG-01 "credentials via `.env` by variable name". You place the values in `.env` (gitignored, mode 0600) on the host that runs the tests; nothing here, in the register, in the evidence or in any log carries a value. The authoritative variable list is the env-var contract that WP-24 and doc10 W10-02 create; the names below are those the plan documents already cite, and the full list is UNCONFIRMED until that contract exists.

| Need (decision) | Variable NAMES cited in the plan documents | You supply (yes / no / later) |
|---|---|---|
| Admin test account for real-service tests (ODG-01) | `ADMIN_USERNAME`, `ADMIN_PASSWORD` | |
| Metadata provider keys (ODG-01) | names UNCONFIRMED (defined by the WP-24 contract) | |
| NAS share accounts for SMB/FTP/NFS/WebDAV tests (ODG-01) | `FTP_USER` is cited; the rest UNCONFIRMED | |
| `google-services.json` values (ODG-01, OD-45) | names UNCONFIRMED | see OD-43 and OD-45 in section 7 |
| Optional HawkScan and Snyk accounts (ODG-01) | `HAWK_API_KEY`, `SNYK_TOKEN` | |
| Optional source-hosting tokens (ODG-10) | `GITHUB_TOKEN`, `GITLAB_TOKEN` (not needed: ODG-10 answer is no external tracker) | |

If you leave any of these absent, the tests that need them are recorded `blocked`, not passed (option (b) of ODG-01).

Credentials already committed:
- ODG-02 (Firebase Android key and the provider-key rotation list), ODG-03 (default admin credential literal): relayed 2026-10-05, "remove from current files only plus a scanner gate; no rotation requested". Residual risk, for your explicit acceptance: the credentials remain in git history, and constitution §11.4.209(D) treats a committed plaintext credential as compromised from the moment of commit. The plan's earlier recommendation was rotate and restrict. Please state that you accept the residual risk: ____

### 1.3 Desktop hosts and signing (ODG-06; blocks WP-53, WP-56, WP-61, WP-62)

Relayed 2026-10-05: "Linux only; updater and signing out of scope; macOS and Windows claims UNMET". Confirm: ____ (no host, certificate or notarisation variable is asked unless you change this).

### 1.4 Build and measurement hosts (ODG-07; blocks WP-09, WP-10, WP-14, WP-15, WP-24, WP-32, WP-33, WP-38, WP-41, WP-62)

Relayed 2026-10-05: the build host and the measurement host are the current host (`hostname` reads `anton`). Conductor-stated facts, not owner-confirmed: 16 CPU, about 31 GiB RAM, podman 5.7.0 rootless. This is relayed, not yet recorded verbatim in `$EV/hc/HC-0.json`; please confirm it at HC-0.

Recorded risks (progress.yml `risks_recorded`, also in the intake):
- Constitution §11.4.173 expects builds distributed to a REMOTE build host; you chose this host. The deviation from remote distribution is an owner decision to be cited in HC-0; builds must still run in rootless containers (§11.4.161, §11.4.173). Please acknowledge the deviation: ____
- Build and SC-011 measurement share one host (31 GiB RAM): measurements only while no build runs; the §12.6 60 percent memory ceiling (about 19 GiB) binds container builds.

Still open: do you want `thinker.local` and `amber.local` (the earlier proposal) kept as additional build or measurement hosts? ____ (if yes: their roles, capacity and reachability are UNCONFIRMED). Whether `anton` passes the T006a probe is UNCONFIRMED (no probe has run). Relayed (batch 3): pull the base images and pin them by digest on this host (rootless podman, the 60 percent memory and disk-headroom rules).

### 1.5 NFS target (ODG-08; blocks WP-13, WP-50, WP-51, WP-53)

Relayed 2026-10-05 (batch 2, narrowed by batch 4 below): BOTH a user-space NFS server in a container first AND an owner-supplied NFS host. Relayed 2026-10-05 (batch 4), replacing the earlier text: user-space NFS server only for now; real NFS host proof UNMET until a host is named. If you ever supply a host, name it (address or name, share, how it is reachable; no credential value): ____ (optional)

## 2. Answers you gave on 2026-10-04, quoted for confirmation

Source: docs/21 revision 15 (section 12.2 owner decisions). T012a records them in `owner-decisions.yaml` after HC-0 and CPA adoption.

- C1 (for ODG-07), verbatim: "We MUST HAVE maximal efficiency and zero waiting whenever is possible with comprehensive / exhaustive callbacks (events driven) mechanisms which all must be full safe and non error prone!"
- C2 (for ODG-16), verbatim: "As much as we need in order to keep producing deterministically validated and verified zero defects products!"
- FR-017 (for ODG-13, with OD-30 and OD-38), verbatim: "We shall aim for the latest versions of it all if and when it is possible and changes applied when new things are brought in into the System!"

Relayed 2026-10-05 (not yet recorded verbatim): C1, C2 and the FR-017 answer confirmed; the plan accepted at HC-0. Conductor caveats: C1, the build host is now this host (see the recorded risks in section 1.4); FR-017, report-only for non-submodule packages until T429a. Please confirm.

Open sub-questions that your answers leave:
- ODG-16 / custody (reserved slot `C2-custody`) is OPEN, flagged by the independent review, not decided by the conductor. Relayed 2026-10-05: "this host is the build platform; signing key in the owner's user keyring". This conflicts with the C2-custody requirement (T012a, T446): the provenance signing key is held only on the build host, out of the producing stream's reach. The agents run as uid 1000 on this single-uid host, the same user whose keyring would hold the key; whether the keyring is unlock-gated against the stream is UNCONFIRMED. On a single-uid host the separation reached is `instance`, never `capability` (§11.4.240(F)). Batch 3 answer: the key stays in your keyring, locked by a passphrase only you enter at signing time, and the honest strength is recorded as `policy`. Please confirm that: keep the keyring and acknowledge `instance` (never `capability`), or choose a different custody: ____
- ODG-13: third-party submodules are only ever moved by our pointer, never modified or pushed (spec FR-017 amendment pending). Relayed 2026-10-05: ODG-13 "update every pin to latest, test each".
- ODG-42 (reserved slot `ODG-42`): confirm the amendment of FR-008 and SC-003 for the legacy headerless class (one tracked, ratcheted register item; spec.md revision 7, pending your confirmation). Relayed 2026-10-05: "decide after T525 per-class report". UNCONFIRMED: T525 concerns ODG-43 (document classes C and D) only and does not produce the headerless count; ODG-42 needs its own wording. Please say whether ODG-42 should wait on T525 or be decided on its own: ____ Until you confirm, T576, T588 and T593 report FR-008 and SC-003 UNMET while the item is open.
- `floors-adoption` (reserved slot): the brownfield adoption of the absolute coverage floors under §11.4.224(E): immediate hard floor; one-time monotone-decrease ratchet; per-corpus phase-in; or changed-code-only with a scheduled full-corpus deadline. Your answer: ____ (relayed ODG-17: "new code 85% now; legacy after baselines" is related but is not this answer)

## 3. For information (not an owner decision): the run-primitive finding

T002 reads `submodules/containers` for a one-shot rootless run of a digest-pinned image with limits, labels and a read-only source mount. If it finds the primitive, T007 wraps it. If it finds none, the direct `podman run` of RUNP and the runners is tracked deviation (b) of the P0-P1 conventions, a `Task` item owned by T139 (upstream proposal in `submodules/containers`, T092 registers it). Status of the T002 record `$EV/wp09/containers-run-primitive.md`: UNCONFIRMED at the time of this draft (not read by this task).

## 4. Own-organisation classification, constitution and policy decisions

### 4.1 Own organisations (ODG-15; blocks WP-02, WP-03, WP-04, WP-55, WP-73; T017, T042)

Relayed 2026-10-05: yes to all three: `helixdevelopment1` and `milos85vasic` are own organisations; `master` counts as main. Confirm at HC-0: ____ (T017 then emits one classification). OD-36 is in section 7.

### 4.2 Constitution hook and pins (ODG-12, OD-31, OD-38; blocks WP-07, WP-73)

Relayed 2026-10-05: ODG-12 "variant B, always fetch and fast-forward pull the latest constitution; sweep must pass". The go-aheads for the WP-07 pin bump and the final WP-73 constitution update follow from this answer and the FR-017 confirmation; confirm: ____

WP-07 owner question OD-WP07-SONNET-FALLBACK (recorded 2026-10-06 by the pin-move step, corrected by the WF8 fix pass; NOT decided by an agent): the constitution at the new pin (`a71b1767`, amendment of 2026-10-04) changes 11.4.209 (code review) and 11.4.211 (merge-conflict resolution): Opus at `xhigh` stays the primary substrate, and canon REQUIRES that when Opus is genuinely unavailable (a captured fact) the work MUST run on Sonnet instead of being blocked; only when both Opus and Sonnet are unavailable is it blocked, and canon names leaving it blocked while Sonnet is reachable a forbidden escape hatch (`--leave-review-blocked-while-sonnet-reachable`; for merge-conflict resolution `--leave-merge-blocked-while-sonnet-reachable`). The project text before this correction said "blocked, never substituted"; that digested the old canon and is not an owner mandate. What the records hold: ODG-19 (`decisions/owner-decisions.yaml` lines 355-358, status Operator-blocked) carries the plan's options ("Opus at xhigh as pinned; other") and the plan's recommendation ("as pinned (§11.4.209)"), which are not an owner answer; your recorded answer is OA-2026-10-05-23 (`decisions/owner-decisions.yaml` line 1216, relayed by the conductor, not a verbatim quotation): "run independent reviews through the Workflow path with Opus xhigh and record effort". Neither says "no fallback" or "blocked". A project rule that blocks where canon requires a fallback would weaken canon, which a project may not do, so the stricter rule is not available as a project choice; the only alternative to following canon is asking for a canon change. Interim practice (an agent's interim choice, not an owner decision): follow canon, Opus `xhigh` first and the Sonnet fallback only on a captured Opus-unavailability fact, recorded with model and effort. Question: confirm following canon (default), or ask for a canon change that removes the Sonnet fallback? Answer: ____

### 4.3 Licence policy (ODG-40; blocks WP-35, WP-57; input to T269)

Relayed 2026-10-05, partial: "Apache-2.0-compatible for shipped code". OPEN, still to supply: (1) the allow-list and deny-list of licence identifiers for the server images and the development or QA tools; (2) a licence for each own-organisation repository that has no licence file. Lists: ____

### 4.4 The 34 doc18 `PROPOSAL` entries (ODG-39; asked again at HC-2, T222; blocks WP-20, WP-74)

Relayed 2026-10-05: "import 34 proposals as tracked items, nine ranked first" (option (a): each a tracked Feature item for a later feature, outside this feature's zero-open count; the nine ranked game changers of docs/18 section 13 are presented first at HC-2 with their experiments E1 to E9, the thresholds being the author's starting points, UNCONFIRMED). Nothing further is asked now. Optional: per-entry (b) drop (a recorded §11.4.122 decision) or (c) adopt via a spec amendment and re-plan: ____

### 4.5 Moving upstreams at the end (ODG-41; blocks WP-73, WP-74R)

Relayed 2026-10-05: "record exact commits verified at final strict run". This maps to none of the options cleanly: (a) compare against the reference tips and list each later foreign commit as a named explained exception; (b) an agreed push freeze on the own-organisation remotes from T579a to T595b; (c) re-run the closing steps until no upstream moves. Recommendation: (a). Which option did you mean? ____ Until answered, such a repository is reported `blocked: ODG-41` with its commits named.

### 4.6 P3 audited-input identity (docs/21 IC-47; T225, T300)

For confirmation with the plan: the audited-input identity of the P3 entry block that decides SC-002 (docs/21 IC-47 leaves the state vector to the plan owner). An amendment re-opens T225 and T300 before P3. Confirm the plan's definition stands: ____. The definition (docs/02 section 12.1, tasks.md T225): the two audit runs start from the same audited input, identified by `input_tree` and not by `main_head`; per repository, `input_tree` is the sha256 of the sorted `git ls-tree -r <commit>` lines of the paths that exist at the input commit and lie outside the reviewed `$AUD/input-exclusions.txt` (the main repository at `input_commit`, each submodule at the commit its gitlink records). A commit window between the runs that changes no input path leaves the identity unchanged. The run manifest also fixes `unit_partition_sha256` and `registry_snapshots`. The docs/02 text is authoritative.

## 5. The remaining decision groups, with the relayed 2026-10-05 answer

The answer shown is the conductor's relay of your answer, in meaning. Each is an `Operator-blocked` register item until HC-0 records it (T071, T072). "Confirm" means: reply only if it is wrong. The docs/21 section 8 row is authoritative for the question text.

| ID | Question (short) | Relayed 2026-10-05 | Still open |
|---|---|---|---|
| ODG-05 | Emulator acceptance scope | "emulators only for now" (with ODG-04) | OPEN: hardware-independent behaviour only, or broader? Your answer: ____ |
| ODG-09 | 1,495 closed-class plus 282 `wontfix` legacy items | re-verify by severity, sample low severity | confirm |
| ODG-10 | Which external trackers are configured | none; local register only; external trackers reported not configured | confirm |
| ODG-11 | Register prefix and location | id prefix `CAT` (never `ATM`; remove `ATM` from this repository's files); generic placeholder `XYZ-NNN`; constitution submodule `ATM` text needs a separate approved upstream change; location `docs/workable_items.db` kept, UNCONFIRMED | confirm the location; acknowledge that the prefix deviates from the constitution (the constitution (11.4.54, 11.4.248) mandates the ATM prefix, so, by your answer, this project's `CAT` is an owner-approved exception to that literal, recorded, with the upstream change request pending); the upstream change is a separate approval |
| ODG-14 | `submodules/llms_verifier` disposition | "investigate history first (S-LLMV-1)" (deferral, equals the recommendation); batch 3: it must be a real submodule at `submodules/llms_verifier` that always tracks the latest main (convert the plain copy, fix the `helix_qa` layout) | OPEN: decided after S-LLMV-1; the conversion is tracked work |
| ODG-17 | Coverage targets, SC-005 sample size, category mapping | new code 85% now; legacy after baselines | OPEN: SC-005 sample size and register category mapping. Your answer: ____ |
| ODG-18 | Escape-ratchet baseline, first manual-QA cycle | first manual QA after the remediation release candidate | confirm |
| ODG-19 | Reviewer substrate | independent reviews through the Workflow path with Opus xhigh, effort recorded | UNCONFIRMED: a prior review ran at an unrecorded effort (tasks.md T011a, commit a27d72a5 message) |
| ODG-20 | FTP, NFS, WebDAV scanners are empty bodies | implement | confirm |
| ODG-21 | Web features with fabricated data | replace with real backed features | confirm |
| ODG-22 | Phone offline layer, playback placeholder | wire and implement | confirm |
| ODG-23 | Desktop `shell`/`fs` plugins, unwired code | history investigation, then a per-item decision (deferral); batch 3: remove only what is not used at all, only with evidence of non-use, in separate commits (`test-results.json`, `catalogizer-api-client` dist outputs, `local.properties.backup`, unused Tauri `shell`/`fs` plugins); wire in and test the internal media | OPEN: decided after the investigation |
| ODG-24 | Purpose of `catalogizer-api-client` | generated or validated client for web and desktop | confirm |
| ODG-25 | Token handling | web: in-memory token plus httpOnly refresh cookie; desktop keychain; WebSocket tickets; signed media URLs | confirm |
| ODG-26 | Certificate model, Android cleartext | TLS with a self-managed CA; Android cleartext off | confirm |
| ODG-27 | Key management for stored share credentials | env-provided KEK with dual-read migration | confirm |
| ODG-28 | Public registration and public `/assets`, `/cover` | deny by default; registration off | confirm |
| ODG-29 | Android toolchain fallback | newest AGP that builds; a failing latest becomes a blocker item | confirm |
| ODG-30 | Contract tooling | file-based Pact; broker deferred | confirm |
| ODG-31 | Rust FTP and WebDAV crates | yes, after the existence check | confirm |
| ODG-32 | Performance parameters | adopt the proposed defaults pending A/A data | confirm; library sizes still to be stated by you |
| ODG-33 | Doc twin threshold, HelixQA links | 150 MB twin threshold; HelixQA links to `submodules/helix_qa` | confirm |
| ODG-34 | Quota and budget | 6 agents, Sonnet default, Opus xhigh for reviews | confirm (was UNCONFIRMED working default) |
| ODG-35 | SLA tiers | no SLA tiers | confirm (was UNCONFIRMED working default) |
| ODG-36 | Trivy exposure | run the check including the build host | confirm |
| ODG-37 | Atlas Pro | no Atlas Pro | confirm |
| ODG-38 | Spec-first for new endpoints | status quo plus gates | confirm |
| ODG-42 | FR-008 and SC-003 for the legacy headerless class | "decide after T525 per-class report" (deferral) | OPEN, see section 2: UNCONFIRMED that T525 produces the headerless count; ODG-42 needs its own wording |
| ODG-43 | Scope of document classes C and D under FR-012 | decide after the T525 per-class report (deferral) | OPEN: decided when the T525 per-class claim and mismatch counts exist; until then FR-012 and SC-006 are UNMET for classes C and D |

Other relayed answers are in the sections above: ODG-01, 02, 03 (1.2); ODG-04 (1.1); ODG-06 (1.3); ODG-07 (1.4); ODG-08 (1.5); ODG-12, ODG-15 (4.1, 4.2); ODG-13 and ODG-16 (2); ODG-39 (4.4); ODG-40 (4.3); ODG-41 (4.5).

## 6. Groups the plan proceeds on by default

These now have relayed answers (confirm only if wrong); the earlier plan defaults were replaced by them:

| ID | Decision | Relayed 2026-10-05 |
|---|---|---|
| ODG-11 | Register prefix and location | id prefix `CAT`, location `docs/workable_items.db` kept, UNCONFIRMED (the earlier plan default `ATM` is withdrawn) |
| ODG-13 | Third-party vendored pins | update every pin to latest, test each |
| ODG-15 | Branch and ownership interpretation | yes to all three |
| ODG-18 | Escape-ratchet baseline, first manual-QA cycle | first manual QA after the remediation release candidate |
| ODG-19 | Reviewer substrate | Workflow path, Opus xhigh, effort recorded |
| ODG-33 | Documentation: twin threshold, HelixQA links | 150 MB; `submodules/helix_qa/`; the SQL path decided by containerized proof (plan default) |

ODG-34 and ODG-35 were reversible working defaults under planning rule 6; they now have relayed answers (section 5).

## 7. The 15 open ungrouped `research.md` decisions (docs/21 section 8.6)

| `research.md` id | Question | Blocks | Relayed 2026-10-05 | Still open |
|---|---|---|---|---|
| OD-14 | register custody design, three open questions | WP-06 | accept with outside-SQL controls | the other two sub-questions of OD-14 in docs/21 are not covered by the relay |
| OD-20 | 30 shuffled race runs per new or changed test | WP-61, WP-71 | 30 shuffled runs | confirm |
| OD-23 | in-memory SQLite in non-unit tests as a real engine | WP-61 | read the production open path, then the owner confirms | OPEN: deferral |
| OD-25 | backward-compatibility window N for the can-i-deploy gate | WP-41 | N=2 | confirm |
| OD-26 | scaling tests where the API cannot run multiple replicas | WP-61, WP-70 | not applicable plus an architectural finding | confirm |
| OD-36 | add the missing GitLab remote for `websocket_client_ts` | WP-73 | add it | confirm |
| OD-43 | crash reporting on the phone | WP-54 | existing project `catalogizer-7a3f1` (batch 4); ADD Crashlytics; use the Firebase CLI for everything; create a Firebase project and apps for all platforms if none exists; a bash init script downloads all config files after clone; fully incorporate Crashlytics, App Distribution (dev and release variants and flavors), Analytics and other major Firebase services; everything dynamic, no manual downloads | OPEN: your Firebase CLI login (owner action, once); I ask before enabling or creating anything new |
| OD-45 | `google-services.json` for builds without Firebase values | WP-54 | same Firebase answer as OD-43 (existing project `catalogizer-7a3f1`) | OPEN: same as OD-43 |
| OD-49 | web state management (Zustand and dead aliases) | WP-52 | decide from the census | OPEN: deferral |
| OD-55 | user-perceived thresholds without a credible source | WP-62 | industry UX thresholds as adjustable defaults | confirm |
| OD-57 | `qa-ai-system/` and `catalog-api/challenges/` documentation live or legacy | WP-63 | live docs; bring into the doc tree | confirm |
| OD-68 | `go-sqlcipher` encryption keyed in production | WP-50 | unknown; verify; batch 3: wire in and test the internal media (also resolves the unkeyed DB, UNCONFIRMED until the production open path is read) | OPEN: to be verified |
| OD-74 | `catalog-api` auth change for the installer's SMB routes | WP-53 | yes, with tests | confirm |
| OD-75 | origin of `installer-wizard/test-results.json` | WP-23 | unknown; investigate; batch 3: removal only with evidence of non-use | OPEN: to be investigated |
| OD-76 | where large evidence blobs and the anchor live | WP-05, WP-74 | blobs local out of tree; anchor on a git remote, fast-forward only | confirm which remote |

OD-60 (binding path of the commit-push script) is resolved by IC-16 and is not asked.

## 8. Answers with no decision id (relayed 2026-10-05)

- Commit route: plain git, fast-forward only, small independently reviewed batches.
- Index writer memory floor (T022): add swap and retry. UNCONFIRMED that swap satisfies the 32 GiB MemAvailable check.
- Tracked but ignored files (1837): untrack build outputs (`catalogizer-api-client/dist`) and `catalogizer-android/local.properties.backup` after a secrets check; docs stay; files stay on disk.
- Evidence chain (batch 3): switch to the constitution continuum chain. OPEN: its four points (hash construction, record schema, exit codes, anchor format) are unanswered; the python chain of docs/06 is interim.
- `.gitignore` blocks (batch 3): keep the repo-wide secrets block and the cache/hosts block; the tasks.md text of the `.gitignore` task is amended accordingly (owner-approved).
- Verifier exit codes (batch 3): accepted as built (precedence 13, 12, 11, 15, strict-behind; strict behind exits 12); owner-approved.
- Anti-bluff scan path (batch 3): a thin wrapper at `scripts/anti-bluff-scan.sh` calls the audit scanner with `--files-from`; test-first.
- `env.properties` (batch 3): untrack it, gitignore it, add `env.properties.example` and a scanner gate; history stays (no force-push).
- Phase 0 close (batch 4): finish the reviews, pull the images, record HC-0, then ask you for the phase boundary approval.

## 9. Unconfirmed items in this draft
- UNCONFIRMED: full credential variable list (WP-24 contract not yet written).
- UNCONFIRMED: `$EV/wp09/containers-run-primitive.md` (T002) not read.
- UNCONFIRMED: whether `anton` can serve as the qualified build host under C1 (no probe run; no build allowed in this task).
- UNCONFIRMED: all 2026-10-05 answers are relayed by the conductor in meaning, not recorded verbatim by you in `$EV/hc/HC-0.json`.
- UNCONFIRMED: whether the keyring that would hold the signing key is unlock-gated (ODG-16).
- UNCONFIRMED: whether ODG-42 was meant to wait on T525.
- Answered (batch 3): yes, the lower-case identifiers `atm_id`, `new_atm_id`, `head_atm_id` rename to `cat_id` in this project's register design. UNCONFIRMED: how the constitution engine accepts the renamed columns (an adapter or an upstream change); `test_no_atm_prefix.sh` still does not match lower case.
- UNCONFIRMED (tasks.md conflict, not decided here): T011 asks for the verbatim docs/21 Default cell for ODG-34 and ODG-35 (`(no counterpart found)`), while T012 asks their `default_detail` to carry the docs/21 rule-6 working-default note; the intake follows T011 and the conflict is open with the tasks.md owner.
- The counts quoted here (43 groups, 15 open ungrouped decisions) were counted from the docs/21 section 8 tables; the T011 test (`scripts/governance/tests/test_decision_intake.sh`) re-derives them from those tables on every run, and binds each relayed answer in `owner-decisions.yaml` to its `progress.yml` source (see `$EV/wp01/`, `$EV` = `specs/001-full-project-audit-remediation/evidence`).
