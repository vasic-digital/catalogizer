# Review: spec.md rev 9, evidence-record schema rev 6, contracts README rev 26 (analyze remediation)

| Field | Value |
|---|---|
| Artifact | `spec.md` (rev 8 -> 9), `contracts/evidence-record.schema.json` (`ev/1` rev 5 -> 6), `contracts/README.md` (rev 25 -> 26) |
| Commits reviewed | `fa7a00ec` -> `1d7bb480` (changed lines only) |
| Reviewer | independent reviewer, model Opus (claude-opus-5-5); effort requested xhigh, not settable or reportable on this dispatch path, so recorded as `?` (§11.4.231(F.2)); producer != verifier |
| Date | 2026-10-05 |
| Verdict | **GO-with-fixes** (0 blocking, 3 important, 3 minor; no schema error; no lockout found that lacks an interim) |

Scope: the lines changed between the two commits, judged against the findings in `scratchpad/ra/rules.md` (A3, A10 to A13, A15, A16, A22, A23, A25, A26), against the rest of spec.md, and against plan and tasks as they stand at `1d7bb480`. The working tree also has uncommitted changes to research.md and docs/02, 05, 06 and others. Where this review cites them, it says so.

## Findings

### A-1 (important): SC-005 says T571 cites the formula, but T571 still restates a different interim n

- **Location:** spec.md SC-005 (rev 9): "the research decision OD-21 and the reviewer sampling task (T571) cite it rather than restate it"; formula `n = min(P, max(30, ceil(0.10 × P), 149))`.
- **Scenario:** tasks.md T571 at `1d7bb480` (line 1253) still says "while ODG-17 is unanswered the draw runs with the interim n of that formula, n = min(population, ceil(ln(1 - 0.95) / ln(1 - 0.02))) = min(population, 149)". The diff `fa7a00ec..1d7bb480` changes no T571 line (0 changed lines). The two formulas agree only while P <= 1490. For P = 2000 the spec gives n = 200 and T571 gives n = 149. A reviewer who follows T571 draws an undersized sample, and SC-005 is then judged on a sample the spec rejects. research.md OD-21 (working tree) and docs/05 §8.3 rev 19 (working tree) do cite the spec. T571 is the one consumer left out, and it is the one that runs the draw. A11 asked for "ONE formula and ONE population", so A11 is not closed.
- **Minimal fix:** in T571, replace the interim clause with "n per spec SC-005 (formula and population stated there; `interim_n` recorded with P)". Alternatively, remove the T571 citation claim from SC-005 until T571 is reworded.

### A-2 (important): the FR-005 parenthetical contradicts the D-02 threshold it cites

- **Location:** spec.md FR-005 (rev 9): "an index is correct when it meets the pass rules of the docs/02 §4.1 proof table (every question returns its recorded answer, and the deliberately unanswerable question returns nothing) and the thresholds of docs/02 decision D-02".
- **Scenario:** D-02 (docs/02 line 811) sets Lumen recall >= 0.85 over the Lumen golden set (`lumen_golden_60.json`, run through `lumen_verify.sh --min-recall`), so up to 15% of Lumen questions may miss. The per-question proof P4 binds the CodeGraph golden set only. The parenthetical says every question must return its answer, which makes the D-02 recall threshold dead. Read literally, one Lumen miss out of 60 makes the semantic index never "correct", and FR-005 says the audit "MUST first demonstrate" this. That is a potential lockout (B) with no interim in the spec. docs/02 itself keeps a fallback: a FAIL blocks Lumen use, and the audit uses grep plus reads instead. The parenthetical also describes the negative-control question (G-CG-4) but not the unsupported-class question G-LU-N, which must also return nothing.
- **Minimal fix:** "(each CodeGraph golden question returns its recorded answer at its recorded rank; the negative-control and unsupported-class questions return nothing; the Lumen set meets the D-02 recall threshold)".

### A-3 (important): the scope of the new SC-006 review clause does not match T525's measure

- **Location:** spec.md SC-006 (rev 9): "100% of existing documents in the FR-012 scope have a recorded review verdict against a named source-of-truth check with zero open differences". FR-012 (rev 9) says that until the owner decides, the requirement "is read in full for every class", and a lighter treatment of classes C and D is a recorded deviation.
- **Scenario:** under the spec's own interim, the "FR-012 scope" covers classes A to D, including the 1,778 class-C tickets. T525 at `1d7bb480` says "SC-006 is measured as every class A and B document holding a `docs/DOC_REVIEW_LEDGER.csv` row with a verdict". T525 also reports FR-012 UNMET for classes C and D. This produces two outcomes:
  - The T525 gate can report SC-006 met while FR-012 is UNMET for the same documents.
  - The spec reading leaves SC-006 unmet, and nothing measures it.

  The two criteria diverge on the classes that A16 left open for an owner decision.
- **Minimal fix:** say in SC-006 that, until the FR-012 class C and D decision, SC-006 is measured over classes A and B and stays unmet for classes C and D (or the reverse), and make T525 state the same rule.

### A-4 (minor): the "tasks.md names no field" statement is stale in the schema and the README

- **Location:** `evidence-record.schema.json` rev 6, top-level `description` ("The field name `test_state_gos` is UNCONFIRMED (tasks.md names no field)") and the `test_state_gos` description ("tasks.md says each such GO is 'named in the run's `ev/1` entry' but names no field"). The contracts/README rev 26 `ev/1` revision 6 note says the same.
- **Scenario:** at the same commit, tasks.md T042 CENTRAL C8 (ii) says "named in the run's `ev/1` entry (field `test_state_gos`, ev/1 schema revision 6; field name UNCONFIRMED until T048a fixes it)". T048a builds its tests on `test_state_gos`. Both contract texts now state something the tasks file contradicts.
- **Minimal fix:** "tasks.md T042 CENTRAL C8 (ii) and T048a name `test_state_gos`; the name is UNCONFIRMED until T048a lands". Change only the descriptions; leave every schema keyword as it is.

### A-5 (minor): the SC-005 justification overstates "smallest", and the 30 term never applies

- **Location:** spec.md SC-005: "149 = ceil(ln(0.05) / ln(0.98)) is the smallest sample that sees at least one weak test with probability 0.95 when 2% of the tests are weak".
- **Scenario:** 149 is the binomial (independent-draw) bound. A seeded draw without replacement from a finite P needs fewer draws, so "smallest" is not exact. The bound is conservative, which is the safe direction. Separately, `max(30, ..., 149)` always exceeds 30, so the 30 term never takes effect.
- **Minimal fix:** "(binomial bound; conservative for a draw without replacement)". Optionally drop the 30 term, or say it is kept for continuity with the earlier proposal.

### D-1 (minor): the interim report-only rule covers third-party packages only; governance item 16 limits updates to submodules

- **Location:** spec.md FR-017 "Pending amendment", SC-009 and the Clarifications entry dated 2026-10-05.
- **Scenario:** `.specify/memory/constitution.md` Known Conflicts item 16 says "'keep dependencies current' means submodules only … Third-party package dependencies are reported, not bulk updated". The rev 9 interim makes only third-party package dependencies report-only. An own-organisation package dependency that is not consumed through a submodule pin would still be moved before the amendment, which is outside item 16's "submodules only" scope. UNCONFIRMED: this review did not establish whether any such dependency exists. The own-organisation Go modules appear to be submodules.
- **Minimal fix:** make the interim read "package dependencies other than those moved through an own-organisation submodule pin are report-only until the amendment". Alternatively, state, with evidence, that every own-organisation package dependency is a submodule.

## Checks run

1. FR and SC ids, rev 8 against rev 9 (`git show` of both, then a regex over `- **FR-nnn**` and `- **SC-nnn**`): FR 25 -> 25, SC 12 -> 12, ordered id lists identical (`diff` empty).
2. `[NEEDS CLARIFICATION]` markers: 0 in rev 8, 0 in rev 9. FR-008, FR-017 and SC-003 still carry "pending confirmation". The phrase "pending confirmation" occurs 10 times in rev 9.
3. SC-005 constant: `ln(0.05)/ln(0.98)` = 148.2837…, ceil = 149, which matches the text. Population P is "every test in the evidence ledger".
4. SC-005 consumers: research.md OD-21 (working tree) cites the spec; docs/05 line 670 rev 19 (working tree) cites the spec; tasks.md T571 at `1d7bb480` restates `min(population, 149)`, with 0 T571 lines changed in the commit range (finding A-1).
5. Cross-references in the new text all resolve:
   - docs/02 §4.1 (line 183), §4.4 (line 234) and decision D-02 (line 811)
   - docs/14 §7 (line 283), §7.2 (line 294) and §7.3 (line 309)
   - docs/13 §3 (line 133), whose class C and D meanings match the spec glossary wording
   - docs/21 §8 (line 811) and §12.22 (line 1788)
   - constitution Known Conflicts item 16 (line 1016)
6. Plan alignment of FR-021 (remote-only builds): plan.md line 55, docs/16 rev 12, docs/21 line 830 and tasks.md (deviation (a) retired) all state that every build runs on the remote build host. No contradiction.
7. T518 against SC-011: T518 counts a `none` target as `sc011: UNMET`, which is consistent. A missing baseline (a `blocked-unavailable` record) is not stated UNMET in T518. That is not a contradiction, and residual A15 already logs it.
8. Bare labels in spec rev 9: bare "C1" and "C2" appear only on line 258, the glossary line that defines the namespaces. The Clarifications text uses "OA-C1" and "OA-C2".
9. Schema dialect: `Draft202012Validator.check_schema` passes for revision 5 (`git show fa7a00ec:`) and for revision 6 (`git show 1d7bb480:`). Tools: Python 3.14.4, `jsonschema` 4.19.2, with `FormatChecker`. Property count: 31 -> 33.
10. Base entry: the docs/06 full ledger line (GREEN, seq 2) is invalid under both revisions as stored (POC `item` `ITEM-good`; no `oracle`, `evidence_class` or `test_fingerprint`). It was completed with those 4 fields, independently of the author's fixtures, and is then VALID under both revisions.
11. Six requested fixtures, built on the base entry and run against rev 6, every result as the README states:

    | Fixture | rev 6 result |
    |---|---|
    | golden-good (`pre_release: true`, two GO names) | VALID |
    | `pre_release: "true"` (string marker) | REFUSED |
    | marker without list | REFUSED |
    | list without marker | REFUSED |
    | list with `pre_release: false` | REFUSED |
    | empty list | REFUSED |

    Extra cases, also as the README states: `pre_release: false` alone VALID; duplicate names REFUSED; empty-string name REFUSED. Revision 5 refuses all 9 cases (`additionalProperties: false`).
12. Old/new equivalence on entries with neither new field: 5 docs/06 samples (the full line and 4 summary lines) plus 50 seeded variants (seed 20261005). The variants vary polarity, verdict, exit status, a missing `target_class`, an alias `item`, `redacted`, an unknown property, a malformed `mutation` and the blocked fields. Of the 55 entries, revision 5 accepts 19 and refuses 36. Mismatches between revision 5 and revision 6: 0.
13. Mutation check: with the two new `allOf` pairing conditions removed, marker-without-list, list-without-marker and list-with-false all become VALID. The pairing conditions are therefore load-bearing.
14. POC `poc/repo_verify/results/run1.json` against `repo-verification-report/1`: exactly one error, `'no_remote' is a required property`, which matches the README rev 26 note.
15. Field naming in tasks: `test_state_gos` occurs in T042 (line 187) and T048a (line 206) at `1d7bb480` (finding A-4).

Scratch files (outside the repository): `scratchpad/rv/chk.py`, `old.json`, `new.json`, `base_valid.json`, `full.json`, `summ.jsonl`.

## Not reported (outside this review's scope, or not a defect)

- docs/21 line 896 imports ODG-13 and ODG-16 as "decided items", while spec rev 9 Clarifications keeps the 2026-10-04 answers "pending confirmation". That line was not changed in this range, and the operational effect is the same (the stricter reading applies). It is a pre-existing wording tension for the plan slice.
- Constitution check: no new technology-specific mechanism enters the spec beyond what the constitution already mandates (rootless remote builds, §11.4.173). "Commit-and-push launcher" and "host entry point" are named generically. No violation found.
