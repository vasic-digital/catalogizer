# Review: tasks.md analyze-remediation (1d7bb480) and rev 36 patch 1 (fa7a00ec)

| Field | Value |
|---|---|
| Artifact | `specs/001-full-project-audit-remediation/tasks.md` (rev 36 patch 1 plus analyze-remediation) |
| Commits reviewed | `fa7a00ec..1d7bb480` (analyze-remediation, findings A1 to A26) and `f44e2bb9..fa7a00ec` (rev 36 patch 1). Scope: changed lines only |
| Reviewer | Independent reviewer subagent (producer is not the verifier), dispatched as Opus at xhigh. The Agent dispatch path cannot report the effort it ran at (§11.4.231(F.2)), so the effort is recorded as `?` |
| Date | 2026-10-05 |
| Verdict | **GO-with-fixes**: no blocking defect; 6 findings to fix (1 important, 5 minor) and 3 nits |

## Findings

**F1 (A, with B risk), important: T084a vs T083/T085.**
- What the change does: T083 now says that its run-A transcript `$EV/wp07/sweep-pin.json` is "reviewed by T084 and committed in the T085 change set". T084 agrees.
- What T084a still says: its changed line keeps "every T082 **or T083** output that a window commits is held on `$EV/reviews/WP-07-hook.json` (never on `WP-07.json`) ... committed after the pin's pushes".
- Scenario: read literally, the T083 run-A output is held on the hook verdict. That verdict waits on ODG-12, so T085's change set stalls on ODG-12. A2 exists to remove exactly this lockout.
- Minimal fix: in T084a, change "every T082 or T083 output" to "every T082 output (run B included)".

**F2 (A), minor: T045a naming claim is contradicted by other tasks.**
- T045a states that `scripts/commit-push-all.sh` and `cpa-host` are "the two kebab-case exceptions" and that "every new script name of this feature is snake_case".
- T051 and T053 create new kebab-case names: `tools/evidence/wrap-go.sh`, `wrap-bash.sh`, `wrap-vitest.sh`, `wrap-gradle.sh` and `wrap-cargo.sh`. A script over the file also finds `scripts/bash-coverage.sh`, `scripts/verify-codegraph.sh`, `scripts/test-in-container.sh` and `scripts/audit/{adb,android}-container.sh`, all untracked at HEAD. Whether each of these is planned as new is UNCONFIRMED.
- The companion record therefore states something false (§11.4.6).
- Minimal fix, either one:
  - rename the `wrap-*` wrappers in T051 and T053 to snake_case and verify the others;
  - or have T045a enumerate every kebab-case new name as a recorded exception.

**F3 (A), minor: T087a fixtures cannot all be produced as written.**
- T087a requires a golden-good and golden-bad fixture for every class of the closed §11.4.261 vocabulary (10 classes). It also says the sweep "adds no scanner of its own" and only reuses the anti-bluff scan, T087, T090 and T040.
- Some classes have no existing detector (for example unresolved §11.4.197 items and un-catalogued anti-patterns). For those, no golden-bad fixture can produce a detection.
- Minimal fix: a class that no reused scanner covers is listed in the ledger as `not_scanned`, with a tracked register item (an honest gap per §11.4.261), and gets no fixture; or allow a thin adapter over the register for the register-backed classes.

**F4 (A, with B risk), minor: T014 has no path to complete HC-0 later.**
- When the ODG-07 host identity is missing, T014 writes `entry_incomplete: host_identity_missing`, and HC-0 then "counts as not recorded" for every committing and building step of P0.
- No text says how a later owner answer clears that state. Read literally, every P0 commit (T047 included) waits forever.
- Minimal fix: add "a later owner answer is recorded by re-writing `$EV/hc/HC-0.json` (or an `HC-0-entry.json` addendum) without `entry_incomplete`, which records HC-0 from then on".

**F5 (A), minor: T530a final step is carried by no task and reviewed by no one.**
- T530a ends: "the generator is re-run with the final register export of T577".
- T577 (after T539) does not carry that re-run (no `changelog` or `gen_changelog` in its text), and T578 does not review its output.
- So T530a cannot close in order: its reviewer T539 comes before T577. The regenerated `docs/changelogs/CHANGELOG.md` would also land unreviewed (§11.4.142).
- Minimal fix: move the re-run into T577's text ("then re-run `scripts/docs/gen_changelog.py`") and name the changelog files in T578's review scope.

**F6 (A), minor: T011a has no edge on T014.**
- The Commit path bullet says that "every committing step of P0 depends on T014".
- T011a commits verdicts and an index (its first deliverables) but has no `depends on` clause.
- The T014 allow-list lets it proceed only up to its commit point, so the edge is missing, not optional.
- Minimal fix: add `(depends on T014)`, or state that only its review runs and drafts precede T014 and its commits follow it.

**F7 (A), nit: T051a scope versus edges.**
- T051a extends "the runner wrapper (T051, T053)" with `--run-token`, but it has no edge on T053.
- T053 already lies earlier in the file, so the edge would point backward.
- Minimal fix: add T053 to T051a's `depends on`.

**F8 (A), nit: T042 marks a field name it now fixes as UNCONFIRMED.**
- T042 names the field `test_state_gos` but adds "field name UNCONFIRMED until T048a fixes it".
- T048a only tests contracts revision 19, which already defines the field (verified: 33 properties, and the `if`/`then` pair makes `pre_release: true` and `test_state_gos` require each other).
- Minimal fix: drop the UNCONFIRMED clause.

**F9 (A), nit: T568 tag format.**
- T568 now carries two story tags, `[US3] [US6]`, the only such task in the file.
- The Format line defines one `[USn]` per task.
- Minimal fix: add one sentence to the Format section allowing a cross-tag.

**Residual risk noted, not a finding: T083 run-A GREEN rule.**
- T083 says a run-A FAIL "beyond its ratchet baseline" stops the T085 push "never waived".
- Gates for newly added anchors have no baseline at the old pin. If such a FAIL could only be fixed by post-T085 work (T082 and later), WP-07 would have no stated escape.
- I found no concrete instance: `scripts/verify_constitution_inheritance.sh` checks only the INV2 to INV4 literals.
- Suggested minimal guard: a run-A FAIL whose fix needs post-T085 work becomes an `Operator-blocked` owner decision (keep the old target through `ORIG_HEAD`), never a waiver.

**Checked and found clean:**
- **B (lockouts and cycles):** no new cycle, and no producer → review → approval → producer loop. The T081a, T085 and T083 chain is ordered and has an owner recovery path.
- **C (forge or bypass):** none introduced. The T081a ratification is procedural and adds no trust path.
- **D (constitution):**
  - T081 versioning matches "A pin bump that only adds anchors is MINOR".
  - The T429a amendment procedure matches the Governance section.
  - The T093a continuation path matches Known Conflicts item 7.
  - Deviation (d) records an existing pre-adoption condition with a per-commit review; it does not introduce a new violation.
- **Patch 1:** the T042/T088 order of the `helper_not_approved` check, the stand-in GO form in T042 and T121b, the working-tree approval replay in T046a, the T358/T457 output rename and the T548a/T569/T582 post-review step are mutually consistent. No dangling `can-i-deploy` reference.

## Checks run (on copies under the session scratchpad)

1. `count.py` on `1d7bb480`:
   - 685 tasks and 90 suffix ids.
   - Tags: TDD 323, REVIEW 87, P 116, SUBAGENT 80; no task without a `[USn]` tag.
   - P4 to P7: 329 tasks (179 TDD, 48 REVIEW).
   - Edges: 1502 `depends on`, 1517 with `after`; no dangling id.
   - Forward edges: 43 over 32 tasks.
   - Cycles: 0 under both rules.
   - Baseline (`fa7a00ec` and `f44e2bb9`): 676 / 81 / 318 / 86 / 115 / 1471 / 1486 / 43 over 32 / 0.
2. Edge diff: 32 edges added and 1 removed (T083 on T082). Every added edge points backward. The `after` set is unchanged.
3. New ids each occur exactly once, with the tags listed in `tasks.newtasks.md`:
   - T011a has no `depends on` clause, by design ("none"); see F6.
   - The other 8 each have exactly one clause.
4. Transitive orderings:
   - T085 ← T083: yes
   - T085 ← T081a: yes
   - T159 ← T139a: yes
   - T049 and T050 ← T048a: yes
   - T094 and T575 ← T087a: yes
   - T046 ← T045a: yes
   - T082 and T084a ← T083: yes
5. File positions follow the dependencies: T083 (line 99) comes before T084 (100), T081a (101) and T085 (102). T045a precedes T046; T048a precedes T049.
6. Contracts revision 19 schema at `1d7bb480`:
   - 33 properties, `additionalProperties` false.
   - `pre_release` and `test_state_gos` are present, each requiring the other.
   - The schema at `fa7a00ec` has 31 properties.
   - The T048a fixtures can therefore produce their stated outcomes.
7. Task lines over 10,000 bytes: exactly the 23 listed in the decomposition note.
8. No `C13` anywhere in the file; each of `CENTRAL C1` to `C12` has exactly one `(definition)`.
9. `TIC docs docs`, `RUNP IMG-TESTUTIL` and `ST-GOV` are existing conventions. `GEMINI.md` exists, `QWEN.md` does not, and `docs/CONTINUATION.md` is tracked, all matching the new text.
10. Kebab-case scan: 11 untracked kebab-case script references in the file (basis of F2).
