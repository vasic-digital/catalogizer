# Upstream change request DRAFT: per-project ticket-id prefix for the Helix constitution

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06 |
| Status | DRAFT, not sent, not filed anywhere. Owner approval of the request itself: 2026-10-05 (ODG-11, relayed by the conductor, not a verbatim quotation). Owner approval of THIS wording: pending. |
| Status summary | Proposes wording for 11.4.54 and 11.4.248, a prefix-neutral id column for the register engine, and a migration plan. Cites the constitution submodule at 10b7a06 read-only. Nothing in the constitution was edited. |
| Issues | none recorded for this document |
| Issues summary | see the UNCONFIRMED section |
| Fixed | n/a |
| Fixed summary | n/a |
| Continuation | Owner review; then filing as an upstream request by the owner (11.4.26 workflow, upstream owns the edit). README links owed: see the last section. |

## Table of contents

1. Why this request exists
2. What the constitution engine already does (cited)
3. Proposed wording changes
4. Prefix-neutral id column: three options and a recommendation
<!-- ATM-ALLOW-BEGIN:constitution-quotation -->
5. Every file and gate that mentions ATM (counts)
6. Migration and compatibility plan
7. Risks and what this change does not do
8. UNCONFIRMED
9. README links owed

## 1. Why this request exists

Owner answer ODG-11 (2026-10-05, `decisions/owner-decisions.yaml` lines ~1100-1120): this project's register id prefix is `CAT`; `ATM` is never used in this repo's files; the constitution's `ATM` text needs a separate approved upstream change; the lowercase `atm_id`-style columns are renamed to `cat_id` in this project's register design; "the constitution engine needs an upstream change or an adapter (UNCONFIRMED how)". The same file (line ~1728) records the standing consequence: this project's `CAT` is an owner-approved exception to that literal, recorded, with the upstream change request pending (the exception conflicts with 11.4.54 and 11.4.248, which have no escape hatch, and with the CLAUDE.md precedence rule, and it stays OPEN until the owner confirms it at HC-0 and the upstream change is accepted).

This draft supplies the missing "how". It is evidence plus proposal, not a decision.

## 2. What the constitution engine already does (read-only citations, submodule HEAD 10b7a06)

The prefix is already a runtime setting in the engine. Only the anchor TEXT, one schema column name and a few gate/test fixtures say `ATM`.

- `scripts/workable-items/cmd/workable-items/prefix.go`: header comment says the ticket-id KEY/PREFIX is DERIVED, never hardcoded to a real project name. `resolveReleasePrefix()` resolves (1) env `HELIX_RELEASE_PREFIX`, (2) `HELIX_RELEASE_PREFIX` in a `.env` at or above the CWD, (3) the snake_case project directory name (11.4.29). `deriveKeyPrefix()` turns that into three uppercase letters (`"atmosphere"` gives `ATM`, `"helix_code"` gives `HEL`); fewer than three letters are padded with `X`; no letters falls back to the neutral key `WIT`. `defaultKeyPrefix()` is the default of `add --prefix`.
- `crud.go` (`addCmd`, around lines 60-85): flag `--prefix` (usage text "3-letter id prefix for auto-generated ids (derived per 11.4.151)"); an explicit `--prefix` overrides the derived default; `--id` gives an explicit id. `nextID()` (around line 413) scans `atm_id LIKE '<prefix>-%'` and returns `<prefix>-(max+1)`, zero-padded to 3 digits.
- `parse.go:21` `issueHeadingRe = ^## ([A-Z]{3}-[0-9A-Za-z]+)(?: \([^)]*\))? — (.+)$`; `parse.go:755` `fixedTitleIDRe` and `subtask.go:30` `parentIDRe` use the same `[A-Z]{3}-` shape. Any three-uppercase-letter key already parses. `crud.go:23` lists `ATM-001`, `HXC-044`, `WIT-042` as accepted examples. So `CAT-001` parses today with no engine change.
- `intake_match.go` (comment around lines 345-355, call at line 619) and `scripts/reporting/report_item.sh` (lines 234, 392-393, 443-445): a consumer config key `id_prefix` is read into `CFG_ID_PREFIX` and passed as `add --prefix`. A per-project prefix is therefore already configurable on two paths (flag, config) plus two ambient paths (env, `.env`).
- `scripts/workable-items/schema.sql`: column `atm_id TEXT NOT NULL` in `items` (composite PRIMARY KEY `(atm_id, current_location)`, line ~98), `item_history`, and three more tables (lines ~130, 175, 200, 211), plus `doc_segments.atm_id` (line ~318) and indexes `idx_item_history_atm_id*` (lines 167, 169). The column NAME is the only `ATM`-bound schema element; its VALUES are already prefix-neutral. `schema.sql` and the embedded `cmd/workable-items/schema_embed.sql` DIFFER (`diff -q` reports a difference; reason UNCONFIRMED), so any schema change must be made in both and the difference understood first.
- The anchor text is behind the engine: 11.4.54 (Constitution.md:4890) mandates the heading form `## §X.Y. [ATM-NNN] <title>`, a helper `scripts/testing/assign_atm_ticket_ids.sh` and a state file `scripts/testing/.atm_ticket_state.json`; the engine's parser expects `## KEY-id — title` and the engine tree has no `scripts/testing/assign_atm_ticket_ids.sh` (UNCONFIRMED that none exists elsewhere; not found under `submodules/constitution/scripts`). The anchor therefore already describes a historical project layout, not the shipped engine.
- Gates: `CM-ATM-TICKET-IDS-COMPLETE`, `CM-ATM-TICKET-IDS-MONOTONIC`, `CM-ATM-TICKET-IDS-UNIQUE` appear only in prose (Constitution.md, groups, mirrors) and in the name ledger `scripts/gates/gate_ledger_prev_names.txt` lines 28-30. A search of the submodule found NO executable gate script carrying these names (grep over `.sh`, `.py`, `.go`, `.yaml`, `.txt` returned only the name ledger and fixtures). So no running gate enforces the `ATM` literal; the anchor is the only binder. Consistent with 11.4.227(A): names without a seam. UNCONFIRMED: whether a consumer project (outside this submodule) implements them.

## 3. Proposed wording changes

Principle: change the literal into a parameter, keep everything else of 11.4.54 intact (monotonic, never renumbered, never reused, no gaps, append-only, project-wide unique), and make `ATM` the DEFAULT only where the project declares nothing. Classification stays universal (11.4.17).

### 3.1 11.4.54 (Constitution.md:4890 and mirrors)

Title: replace "ATM-NNN ticket identifier mandate" with "Project-prefixed ticket identifier mandate (`<KEY>-NNN`)". Keep a one-line note "(historically `ATM-NNN`)" so existing cross-references stay greppable (11.4.227: an anchor number is never reused; the anchor number 11.4.54 does NOT change).

Replace the operative sentence

> Every workable item ... MUST carry a `[ATM-NNN]` ticket identifier in its heading, in the form `## §X.Y. [ATM-NNN] <title>` ...

with

> Every workable item MUST carry a `[<KEY>-NNN]` ticket identifier. `<KEY>` is the project's ticket-id KEY: exactly three uppercase ASCII letters, resolved once per project by the order (1) `ticket_id_prefix` in the project's consumer config, (2) `HELIX_RELEASE_PREFIX` per 11.4.151 derived as the first three letters, uppercased, (3) the neutral default `WIT`. `ATM` is the historical default of one consuming project and has no special standing. NNN is a positive integer zero-padded to at least 3 digits. The KEY is recorded in the project's tracked configuration and MUST NOT change after the first id is allocated; two KEYs in one register are refused (a rename is a migration under section 3.5 below, never an in-place edit).

Additions to the closed rules 1-4 (renumber, reuse, decrement, skip): add rule 5, "Re-key. Changing a project's KEY after allocation is a recorded migration with an old-id to new-id map kept append-only; the old ids stay resolvable." Add that the Issues/Fixed summary leftmost column is titled `ID` (the old `ATM ID` heading is accepted for existing documents).

State file: rename `scripts/testing/.atm_ticket_state.json` to `scripts/testing/.ticket_state.json` in the text, key `atm_id` to `ticket_id` in the record shape, and say the helper is `assign_ticket_ids.sh`; keep the old names as accepted aliases for one release (see section 6). Mark the helper path as consumer data per 11.4.35, since the engine does not ship it (see section 2).

Composition sentence: the join-key role is unchanged (11.4.15, .16, .19, .33, .55, .57 compose with `<KEY>-NNN`).

### 3.2 11.4.248 (Constitution.md:11254)

Replace the tag `[PROTECTED-SPEC: ATM-NNN]` with `[PROTECTED-SPEC: <KEY>-NNN]`, where `<KEY>-NNN` is any id of the project's register. State that the tag regex for tooling is `\[PROTECTED-SPEC: [A-Z]{3}-[0-9]{3,}\]` and that the old literal tag is a valid instance (KEY = `ATM`). Heading, compact summary, composes-line and the group files and mirrors take the same edit. Gate names that carry `ATM` are addressed in 3.4.

### 3.3 Other anchors that restate the literal

Constitution.md counts per anchor (strict match `ATM-[0-9N]|ATM-NNN|\bATM\b|atm_id`): 11.4.54 has 19 hits, 11.4.55 has 6, 11.4.93 has 5, 11.4.58 has 4, 12.12 has 2, 11.4.248 has 2, and one each in 11.4.56, .63, .83, .91, .135, .148, .197, .261, .266. Proposed rule: where `ATM-NNN` is used as an example of "a ticket id", write `<KEY>-NNN`; where it is a historical forensic quotation (the verbatim 2026-05-19 mandate, defect ids in fixtures), leave it unchanged and mark it as a quotation. 11.4.54's verbatim forensic quote keeps `ATM-` because it is a record of what the operator said.

### 3.4 Gate names

Rename in prose and in the ledger by ALIAS, not by deletion: `CM-TICKET-IDS-COMPLETE`, `CM-TICKET-IDS-MONOTONIC`, `CM-TICKET-IDS-UNIQUE`, with the three `CM-ATM-TICKET-IDS-*` names kept in `gate_ledger_prev_names.txt` (that file is exactly the removal-citation mechanism of 11.4.227(A): a vanished name needs a citation). Because no executable gate carries the old names (section 2), the rename costs prose edits only; the owed gate CODE is a separate work item as the anchor already says.

### 3.5 Engine (workable-items)

1. Add `ticket_id_prefix` as a recognised consumer config key beside the existing `id_prefix` (`report_item.sh` reads `id_prefix` today). Proposal: keep `id_prefix` as the canonical key (it exists), document that it is the project KEY, and extend `resolveReleasePrefix()` so the order becomes flag, config `id_prefix`, env/`.env` `HELIX_RELEASE_PREFIX`, directory name. Today the order is flag (explicit), then env, `.env`, directory name, and config is applied by the shell wrapper only; making config part of the engine's own default removes the divergence risk the comment at `intake_match.go:345-355` already worries about.
2. Add a single-KEY guard: `add` refuses an explicit `--prefix` that differs from the KEY already present in the register unless `--allow-new-key` is given (needed for multi-project registers that deliberately mix keys; `crud.go:23` lists HXC, ATM and WIT as accepted example keys, so mixing is tolerated today and the guard must stay opt-out safe). UNCONFIRMED whether upstream wants this guard; it is optional.
3. `deriveKeyPrefix()` pads short names with `X` and caps at three letters. `catalogizer` gives `CAT` today by derivation, so a project that sets nothing already gets `CAT` here. Document this in `prefix.go`; no code change is needed for this project.

## 4. Prefix-neutral id column: three options

Facts: `atm_id` appears 674 times in 60 non-test files and 233 times in 49 test files under `scripts` and `submodules` (grep `atm_id|atmID|AtmID`, `.go .sql .sh .py`). It is a PRIMARY KEY component, an index target, and a column in five tables plus `doc_segments`; `submodules/anti_bluff/seams/status_custody/custody_schema.sql` and `custody_triggers.sql` bind it too (18 hits in the triggers). The names `new_atm_id` and `head_atm_id` the owner mentioned do NOT occur in the constitution tree (grep returned nothing); they belong to this project's register design (UNCONFIRMED where they are defined).

| Option | What changes | Cost | Risk | Reversible |
|---|---|---|---|---|
| A. Column rename `atm_id` to `item_id` (upstream) | schema.sql, schema_embed.sql, all SQL strings in about 60 files, custody schema and triggers, migrations for every existing DB (there is already a `migrate_v3_to_v4` test, so a migration path exists) | high: about 900 occurrences, a schema version bump, every consumer DB migrated | high: a primary-key column rename in live registers; custody triggers must be re-created in the same transaction | only with a down-migration |
| B. Keep `atm_id`, add a prefix-neutral VIEW | add `CREATE VIEW items_v AS SELECT atm_id AS item_id, ...` per table (and `item_history_v`, ...); consumers query views | low: additive, no data move | low for the engine; medium for confusion (two names) | yes, drop the view |
| C. Adapter in the consumer (`cat_id`) | consumer-side only: the project keeps its own register DB with `cat_id` and a thin adapter that renames at the engine boundary | medium for this project, zero upstream | high: the engine opens the DB by its own embedded schema (`schema_embed.sql`); a renamed column breaks `nextID`, `sync`, `close`, custody triggers unless every call is wrapped; that is a fork in effect and 11.4.251 (byte-identical-fork prohibition) and 11.4.74 (extend, do not reimplement) argue against it | yes, but it diverges silently |

Recommendation: B now, A as a later major version.

- B is the smallest upstream diff that gives this project a `cat_id` surface without touching 900 call sites: the engine keeps its internal column name, the views expose the neutral name `item_id`, and this project's tooling reads `item_id`. The owner's wish ("`cat_id` in this project's register design") is met in this project's DOCS and contract schemas, where the neutral name is `item_id` or `cat_id` as the owner prefers; only the engine-owned DB column keeps the legacy name. UNCONFIRMED: whether the owner accepts that the DB column name `atm_id` stays inside the engine's own SQLite file; the ODG-11 answer says "rename ... to cat_id in this project's register design", which is satisfied for design documents and contracts but not for the engine's physical column under option B.
- Reject C unless upstream refuses both A and B: it creates a divergent chain of the same kind as the open DR-E1 deviation (see the continuum brief).
- A is correct long term (the physical name is a historical accident) but should ride a planned schema version with a migration, the existing `migrate_v3_to_v4` pattern, and a deprecation window.

If the owner prefers the literal name `cat_id` for the project-visible view column, option B can emit it as an alias per consumer (`SELECT atm_id AS cat_id`), configured through a view-template parameter; recommended default is the neutral `item_id` to avoid reintroducing a project name into universal code (the same objection that rejected `ATM`).
<!-- ATM-ALLOW-END -->

<!-- ATM-ALLOW-BEGIN:constitution-quotation -->
## 5. Every constitution file and gate that mentions ATM (read-only, counts)

Method: `grep -rIEc 'ATM-[0-9N]|ATM-NNN|\bATM\b|atm_id|\.atm_|assign_atm|atm_ticket'` over `submodules/constitution`, excluding `.git`, `*.html`, `*.pdf`, `*.docx` (generated twins, stale per 11.4.106(E)) and, for the first table, `*_test.go`. A bare `ATM` substring search is wrong: it also matches "atmosphere" fixtures and inflated counts earlier; the strict pattern above is used. Control: the pattern returns the known hits in `Constitution.md:4890` (needle present) and 0 for a file with none. Totals: 1851 hits in 274 files (non-test-Go), plus 679 hits in 168 `_test.go` files.

Governance text (the part the change request edits):

| File | Hits |
|---|---|
| `Constitution.md` | 59 |
| `CLAUDE_ANCHORS_FULL.md` | 37 |
| `AGENTS.md` | 18 |
| `CLAUDE.md`, `QWEN.md`, `GEMINI.md` | 4 each |
| `groups/workable-items-and-tracking.md` | 43 |
| `groups/host-and-resource-safety.md` | 4 |
| `groups/testing-and-tdd.md` | 3 |
| `groups/project-lifecycle-and-release.md`, `groups/documentation-and-export.md` | 2 each |
| `groups/multi-track-and-parallelism.md`, `groups/governance-and-constitution-meta.md`, `groups/anti-bluff-and-evidence.md` | 1 each |
| `constitution_index.yaml` | 2 |
| `CHANGELOG.md` | 3 |
| `submodules/docs_chain/{CLAUDE,AGENTS,QWEN,CONSTITUTION}.md` | 1 each |

The four mirrors are a lockstep set (11.4.157, 11.4.227(B)); the `.html/.pdf/.docx` twins of each also carry the literal and need regeneration, which the submodule itself records as not runnable in-repo (stale). Counts for the twins were not taken (excluded by design).

Engine and tooling (non-test), top by hits:

| Area | Files (hits) |
|---|---|
| `scripts/workable-items/cmd/workable-items/` | db.go 43, sync.go 41, parse.go 25, mutate.go 25, crud.go 18, subtask.go 16, diary.go 15, classify.go 14, repair_bodies.go 12, intake_match.go 8, version_tags.go 7, group.go 6, export.go 6, assign.go 6, obsolete.go 5, diary_cmd.go 4, report.go 3, correct_evidence.go 3, validate_groups.go 2, main.go 2, prefix.go 1, occurred_at.go 1, closure_seam.go 1; `schema_embed.sql` 29; `testdata/fx_shape2_atm248.md` 2 |
| `scripts/workable-items/` | schema.sql 14, README (docs) 1 in UPSTREAM-EXTENSION-EVIDENCE.md |
| `submodules/anti_bluff/seams/status_custody/` | custody_triggers.sql 18, custody_sweep.sh 8, custody_schema.sql 4, apply_custody.sh 3; `submodules/anti_bluff/test/test_status_custody.sh` 23 |
| `scripts/fastcycle/` | cycle/cycle_report.py 49, cycle/select_sample.py 43, tokens/dispatch_stamp.sh 19, closure/escape_classify.py 19, context/anchor_citations.py 16, cycle/baseline_replay.sh 13, cycle/collect_baseline.py 10, tokens/transcript_ingest.py 7, closure/reopen_rate.py 6, verify/plan_struct_check.py 5, orchestration/ (custody_sweep.py 2, completion_probe.sh 2, sc006_exercise.sh 1), gates/verdict_cache.py 2, others 1 each; `fastcycle/tests/` 556 hits in 101 files |
| `scripts/multitrack/` | multitrack_work_binding.sh 10, multitrack_resolve_worktree.sh 4, multitrack_cwd_hook.sh 4, multitrack_claim.sh 3, multitrack_checkout_owner_lock.sh 3, multitrack_persistent_launch.sh 2, multitrack_config.sh 1 |
| `scripts/hooks/` | guard-track-branch-label.sh 4, guard-work-track-binding.sh 3, credential_scan_lib.sh 1; tests: test_guard_work_track_binding.sh 17, test_guard_track_branch_label.sh 10 |
| `scripts/gates/` | cm_escape_ratchet.sh 10 (fixture defect ids `ATM-1`, `ATM-2`), cm_zero_findings_monotone_ratchet_mutation_test.sh 10, cm_escape_ratchet_selftest.sh 5, cm_every_finding_closed_or_tracked_mutation_test.sh 3, gate_ledger_prev_names.txt 3 (the three `CM-ATM-TICKET-IDS-*` names), cm_reporting_directives.sh 1, cm_feature_directive.sh 1, cm_mutation_score_on_diff.sh 1, mutation_score_threshold.txt 1 |
| `scripts/doc_integrity/` | 54 hits in 24 files, mostly golden fixtures (`internal/selfcheck/golden/**`), plus normalize.go 3, record.go 2, markdown.go 2, adapter.go 2, config.go 1, selfcheck.go 1 |
| `scripts/catalog-engine/lib/` | render.py 4, parse_shell.py 4, parse_gate.py 3, parse_mutation.py 2, parse_helixqa.py 2 |
| `scripts/anchors/constitution_wiring_audit.py` 2; `scripts/codegraph/runner_patches/resolve2.py` 1; `submodules/session_orchestrator` (alias/registry.go 2, claim/select.go 1); `submodules/token_optimizer` (5 files, 1 each); `submodules/clickup_sync/docs/design/DESIGN.md` 9 | |
| Research and analysis docs | `docs/research/quality/**` 185 hits in 20 files, `docs/research/extensions/**` 59, `docs/optimization/SPECKIT004_SUPERPOWERS_ANALYSIS.md` 28 (historical analysis, quotations; do not rewrite) |

Gates that mention ATM: only the three name-ledger entries above. No executable gate script carries an `ATM` literal in its NAME. Gates whose FIXTURES use `ATM-n` ids (escape ratchet, status custody, mutation tests) need no logic change because their parsers are shape-based (to be confirmed per file; UNCONFIRMED: not every fixture was read).

Observation for the owner: most hits are historical (fixtures, research quotations, test data). The set that must change for a clean prefix-neutral constitution is the anchor text (about 130 hits across the five governance files plus groups) plus the schema column question of section 4; the rest are fixtures that remain valid because `ATM` is a legal KEY.

## 6. Migration and compatibility plan

1. Phase 0 (no code): the owner files the request with sections 3.1-3.4. Until accepted, this project keeps `CAT` as an owner-approved exception to that literal, recorded, with the upstream change request pending (already recorded in `owner-decisions.yaml`), and its own docs avoid the literal `ATM` (ODG-11). No change to the constitution submodule from this repo (read-only pointer discipline; any pin move follows the T011a/T530a sweep rules of tasks.md, not decided here).
2. Phase 1 (text only, backward compatible): upstream changes the anchors to `<KEY>-NNN`, keeps `ATM` as the documented historical default, keeps old gate names and old state-file names as aliases. Existing consumers using `ATM` are compliant without any edit (ATM is a valid KEY). Mirrors and twins regenerate together (11.4.157).
3. Phase 2 (engine, additive): `id_prefix` joins `resolveReleasePrefix()`; optional single-KEY guard; option B views added with a schema minor version; `schema.sql` and `schema_embed.sql` reconciled first.
4. Phase 3 (optional, major): option A rename with migration and one-release dual-read (views continue to answer the old name).
5. Compatibility matrix: consumer on `ATM` with old engine: unchanged. Consumer on `CAT` with old engine: works for `add`, parse and sync today (regex accepts any three letters, `nextID` is prefix-scoped) but the anchor says it is non-compliant, hence this request. Consumer on `CAT` with new text and old engine: compliant by text, engine unchanged. UNCONFIRMED: no run of the engine with `CAT` was done here (read-only task, no build per 11.4.173); the statements above are from source reading.
6. Verification the upstream owners should require (anti-bluff, 11.4.224): a RED test that fails today for a `CAT`-keyed register against any gate that still hardcodes `ATM` (none found), a golden-good with `CAT-001` and `XYZ-007`, a golden-bad with a lowercase or four-letter key, and a paired mutation that reintroduces the literal into `issueHeadingRe`.

## 7. Risks and what this change does not do

- It does not change the numbering rules (monotonic, never reused, no gaps); only the literal.
- A KEY change after allocation is a migration; the request forbids in-place edits.
- Two-letter or four-letter keys: the engine regex is exactly three letters; the proposal keeps three. A project that wants a different length needs a separate request (not asked).
- Third-party docs that quote `ATM-NNN` as an example (11.4.54's verbatim forensic anchor, historical research docs) stay as quotations; do not rewrite history.
- Generated twins stay stale until the exporter exists (the submodule's own finding).
- The anchor is currently only text-bound (no executable gate for the three names); a prefix change cannot break a gate that does not run, but it also means the change will not be verifiable by a running gate until gate code lands (11.4.227).

## 8. UNCONFIRMED

- The owner's acceptance of THIS wording and of option B for the column (section 4).
- Whether the physical column `atm_id` staying in the engine's SQLite file satisfies "rename to cat_id in this project's register design" (relayed, not verbatim).
- Where this project's `new_atm_id` and `head_atm_id` names are defined (not in the constitution tree).
- Why `schema.sql` and `schema_embed.sql` differ.
- Whether `scripts/testing/assign_atm_ticket_ids.sh` exists in any consumer (absent in the submodule).
- Whether any consumer implements the three `CM-ATM-TICKET-IDS-*` gates.
- Engine behaviour with a `CAT` register at runtime (not run).
- Whether upstream wants the optional single-KEY guard.
- Counts for html/pdf/docx twins (excluded by method) and a per-file read of every fixture.
- The submodule HEAD cited (10b7a06) is the checked-out pin of this repo, not a fetched latest main (no network used).

## 9. README links owed (nothing links this file yet)

- `specs/001-full-project-audit-remediation/README.md`: add this file (and `continuum-decision-brief.md`) under a section "Upstream change requests".
- `specs/001-full-project-audit-remediation/docs/21-master-plan-phases-risks-and-traceability.md`: reference from the ODG-11 row and from the risk "constitution mandates ATM".
- `specs/001-full-project-audit-remediation/decisions/owner-request-list.md`: file the owner question "approve this upstream wording and the option B column view" as a separate request, as `owner-decisions.yaml` line ~1728 already asks.
<!-- ATM-ALLOW-END -->
- `specs/001-full-project-audit-remediation/docs/04-findings-register-design.md`: link from the section that defines the register id prefix and the `cat_id` columns.
- Root `README.md` (11.4.212): only if the project decides upstream drafts are in documentation scope (UNCONFIRMED).
