# evidence/wp01 round 4 notes (governance area, review WF3, 2026-10-06)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06 |
| Status | current for round 4; transcripts are the `r8-*` files, each starting with `#` identity lines (git HEAD and the sha256 of the tests, the intake, the request list, `progress.yml`, docs/04, docs/21, research.md, the two upstream drafts and tasks.md); a different hash means a transcript is stale |
| Issues | the owed items below |
| Issues summary | scope decision of the owner for this round: every real-defect important finding fixed (I1 to I5); minor findings only where small; guard-adequacy gaps beyond that are listed here as OWED, not fixed |
| Fixed | I1, I2, I3 (tracked, not applied), I4, I5, plus m7, m8 (revision cell, grammar) |
| Fixed summary | see "What changed" |
| Continuation | the owed items; the owner items 14 to 17 of `decisions/owner-request-list.md` |

## Transcripts (round 4)

- `r8-red-I4.txt`: RED of the guard on the real tree (30 hits in the new upstream draft, kind constitution-quotation absent).
- `r8-red-no-atm.txt`: RED of the revised no-ATM test (inventory 33 and 30 expected, E7: five docs/04 lines with a concrete CAT id).
- `r8-red-intake.txt`: RED of the revised intake test (19 failures: precedence, owed items, request-list items).
- `r8-green-1..3.txt`: three runs of the five tests. Intake, no-ATM, schema and evidence-record schema tests exit 0; `test_evidence_root.sh` exits 1 for FOREIGN reasons only (residue hits in files of other work streams under `scripts/repo/tests/` and `tools/evidence/`, not in this area's scope; the list changes while those streams work). This area's own file no longer contributes a hit.
- `r8-mutations.txt`: 22 mutants on a scratch git tree (never the real tree), all with the wanted exit code: undated and misnamed answer blocks (reviewer's MI1, MI3), an unknown top-level key, an answer-like key nested under `tasks`, a dated block not carried by the intake, an invented `amends` (MI4), the precedence statement removed or re-contradicted, the OWED marker, the option B caveat and the continuum brief path removed, a concrete CAT id in docs/04, a restored paragraph losing its marker, a missing BEGIN marker, an unmarked line in the upstream draft, a constitution-quotation region in a non-upstream file, a region re-kinded, the disabled quote restriction (scenario suite fails), and the load-bearing check (with the I5 allow-list disabled, MI1 survives rc 0).

## What changed

1. I1: docs/04 prose lines that reported an executed result with a CAT id no run printed. Three (v1 review paragraph, limitation 1, limitation 7) are byte-true text of HEAD again, each with a captured-output line marker; two mixed design and result passages (the mint-trigger paragraph and the compliance row) are id-neutral. The guard (E7 in `test_no_atm_prefix.sh`) fails on any concrete CAT-NNN id in docs/04. The rule is extended in the docs/04 revision entry: prose that reports an execution is evidence. Revision 26 of docs/04.
2. I2: one precedence statement everywhere: this project's CAT is an owner-approved exception to that literal, recorded, with the upstream change request pending; the exception conflicts with 11.4.54 and 11.4.248 (no escape hatch) and with the CLAUDE.md precedence rule, and stays OPEN until the owner confirms it at HC-0 and the upstream change is accepted. Aligned in `progress.yml` (the `risks_recorded` copy), the intake (risks copy and the ODG-11 note, with `NOTE_FLAGS` in the test), the request list, docs/04, docs/21, research.md and the upstream draft. The intake test fails any of these files that says "the constitution wins" or lacks the one statement.
3. I3: tracked, NOT applied (the rename and the engine-compatibility choice need the owner): D-8 in docs/04 section 15.3, request-list items 15 and 16, the no-ATM test header (no longer calls it unanswered). Owner approval of option B is NOT given.
4. I4: kind `constitution-quotation`, legal only in a `*.md` file under `docs/upstream/`, with scenarios (inside upstream, outside, a `.txt`, unmarked line), an inventory entry for the draft (30 lines) and a kind pin (only that kind). Two marked regions (82 and 80 lines) in the draft. `continuum-decision-brief.md` holds no hit (0, counted).
5. I5: closed allow-list of the top-level keys of `progress.yml` plus a dated-name pattern; any key containing `answer` that is not a dated block fails; a control needle checks the pattern.
6. Small: relayed entries must carry `amends: null` (m7); the request list revision cell and the intake revision number (m8); `an CAT` grammar (m9).

## Owed items (not fixed this round; guard-adequacy and minor findings beyond the owner scope)

- OWED-1 (I3a): rename of the lower-case identifiers `atm_id`, `new_atm_id`, `head_atm_id` to `cat_id` in this project's register design (docs/04 about 150 lines, data-model.md 4). Needs the owner's acceptance of option B of `docs/upstream/constitution-prefix-change-request.md` (the engine owns its `atm_id` column; a rename would break the engine). No tasks.md task carries it; adding one is a tasks.md amendment (counts 685 tasks, edges 1505 and 1520, 43 forward over 32, 0 cycles must be re-run on a scratch copy) not made here.
- OWED-2 (m3): pin a sha256 per captured region of docs/04 section 14; today only 8 anchor lines are pinned, so a fabricated edit of another captured line (reviewer MN1, MN1b) survives. The EXPECT comment for docs/04 now counts 33 lines.
- OWED-3 (m4): no-ATM blind spots: UTF-16 text (MN2), compressed containers `.docx`, `.pdf`, `.zip` (MN3; 14 tracked PDFs, 0 hits when counted by the reviewer), and file names (MN5).
- OWED-4 (m5): evidence-root R2 scan false negatives for the forms ME1, ME3, ME4, ME5, ME6, ME7 (variable-defaulted, `os.path.join`, `Path(...) /`, `${ROOT_DIR}/evidence`, `cd ... && mkdir evidence`, `git rev-parse --show-toplevel`); R1 is the runtime backstop and the real tree has none of these forms today.
- OWED-5 (m6): schema test fixtures for a non-string id (MS1) and a non-ASCII digit id (MS2).
- OWED-6 (m1): revision headers not bumped in data-model.md, docs/02, 03, 05, 06, 07, 08, 09, 10, 12, 15 and tasks.md; docs/05 and docs/12 Status cells edited in place. Other areas' files, outside this round's touch list.
- OWED-7 (I2b): docs/03 DR-1 (about line 690) still says 11.4.54 requires a stable prefixed id and was rewritten in place to CAT while line 286 says it is superseded; restore DR-1 as recorded and add a superseding note citing ODG-11. docs/03 is outside this round's touch list.
- OWED-8 (m2): the wp01 README index still calls `r7-*` current and does not index the `e6-*` files; this round indexes `r8-*` only. The `e6` mutation ran a mutant copy inside the live repository (use a scratch tree: this round did).
- OWED-9 (m8): the request list does not flag the overlap of the interactive answer on `local.properties.backup` and the batch-3 removal answer, and has no line confirming the six repo-wide cache lines of OAU-2026-10-05-5 (tasks.md T004 asserts approval).
- OWED-10 (m10): contracts/README does not list `build-event.schema.json` (another area).
- OWED-11 (m11): no task owns the rename guards (`test_no_atm_prefix.sh`, `test_evidence_root.sh`, `test_register_id_schemas.sh`); `progress.yml` is untracked until the commit.
- OWED-12 (new): the upstream drafts are not linked from the README, docs/06 or docs/21 (the drafts list the links owed); not done here because README.md is outside the touch list.

## UNCONFIRMED

- Whether a re-run of the section 14 proof of concept with the CAT DDL prints the same lines with CAT in place of the old prefix.
- Whether the owner accepts option B, and whether the lower-case rename to `cat_id` is wanted before or after the engine accepts it.
- The independent re-review of this round: not run here; this round is UNREVIEWED.
- `test_evidence_root.sh` is RED on files of other work streams (`scripts/repo/tests/`, `tools/evidence/`); not touched.
