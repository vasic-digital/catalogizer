# Lead-scan scope record (T167, doc03 section 10 step 4)

| Field | Value |
|---|---|
| Created | 2026-10-08 |
| Status | DECISION RECORD, owner approval UNCONFIRMED; to be answered at HC-2 as `lead_scan_scope` (T222) |
| Source | `scripts/register/lead_scan_population.py`, `docs/scripts/lead_scan.md` |

## Scope implemented

The lead scan reads only **tracked Markdown of the main repository outside every gitlink**: the entries of the frozen listing that end in `.md` and lie outside every `gitlinks` path of the freeze json. T167's measured plan-time numbers: 2,552 files outside every gitlink, 3,713 Markdown files under the third-party `submodules/helix_qa/tools/opensource/` (and the rest of the submodules). On the `git archive HEAD` scratch snapshot used by this pass (8,163 files, only the helix_qa banks, baselines and anchors materialised under `submodules/`, one gitlink path `submodules/helix_qa` supplied) the population is 2,717 files.

## Reason

Third-party code and vendored documentation are not findings of this project; the submodules are separate repositories with their own reconciliation (doc03 5.x, WP-21..). The reason is recorded here for the owner's approval; it is NOT an owner decision yet.

## The one widening

The owner's recorded answer, `lead_scan_extra_roots` of the HC-2 record at the fixed path `$EV/hc/HC-2.json` (a list of gitlink paths whose Markdown comes into scope). There is no command-line option. An absent record is `[]`; a malformed record is refused `hc2_malformed`; a root that is not a gitlink path is refused `lead_scan_extra_root_invalid`. Tests: `scripts/register/tests/test_lead_scan.py` (the needle: the same lead line in a submodule Markdown yields no row, with the owner's extra root it yields one).

## Inflections

The vocabulary is the closed doc03 13 list WITH its inflections (stubbed, known issues, todos, workarounds, not-implemented ...) and word boundaries; `unimplemented` (31 lines on the HEAD population) is outside the doc03 vocabulary and is NOT a lead word here (a vocabulary extension is the owner's).
