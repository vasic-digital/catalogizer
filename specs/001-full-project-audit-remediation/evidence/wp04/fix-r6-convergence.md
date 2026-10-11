# commit-push-all, fix round 6 (11.4.276 round 5 of the item): convergence assessment and result

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Last modified | 2026-10-08T16:30:00Z |
| Status | assessment written before the fixes (11.4.276(E)); the section "Result" holds the measured results; NOT reviewed, NOT committed |
| Status summary | one class-complete fix round for review `evidence/reviews/WP-04-T046.json` (NO-GO, R-01 blocking, R-02..R-04 important, R-05..R-07 minor) |
| Source | `evidence/reviews/WP-04-T046.json`, tasks.md T038-T047, reviewed head `20f74abf` |
| Issues | R-03 (kcov and pinned shellcheck), see "Result"; R-04 is a process finding with an open point for the next review, see below |
| Fixed | R-01, R-02, R-05, R-06, R-07 in the scripts, tests and contract; R-03 and R-04 as recorded below |

## Assessment (before the fix)

Root cause of R-01 (class MODEL/MSG, third recurrence): the same fact, "which lines are the trailers of this commit", was described by THREE parsers: a whole-message awk (`cpa_runid`, `integrate_merge` held scan),
git's `interpret-trailers --parse` (`push_recursive` hold check), and git's `%(trailers)` (Foreign-Commit dedupe). They agree only on messages without a `---` line. Fix by REPLACEMENT, not a fourth layer:
one awk grammar in `lib_safe.sh` (`cpa_trailer`, `cpa_runid`, `cpa_hold_state`, `cpa_foreign_listed`) used by every reader, plus `cpa_msg_check` in front of both writers.

Enumeration of every place a CPA commit message is parsed or written (grep over scripts/ for `%B`, `trailers`, `interpret-trailers`, `Awaits-Review`, `CPA-Run`, `Foreign-Commit`, `Deferred-Gates`):
readers `lib_safe.sh cpa_runid` (callers: commit-push-all S1 x2, S6, integrate_ff_only is_cpa, push_recursive, integrate_merge is_cpa), `push_recursive.sh trailer()`, `integrate_merge.sh` held scan and Foreign dedupe,
`commit_recursive.sh` Foreign dedupe; writers `commit_recursive.sh make_commit`, `integrate_merge.sh` message, launcher S0 impersonation check. No other reader exists (`record_deferral.sh` only mentions the trailer name).
Message shapes that desync parsers, each a case of `scripts/repo/tests/test_wp04j.sh` (parser level, 26 shapes) and, for the hold-relevant ones, an end-to-end `push_recursive` case: `---` (body, diff header, inside the block), blank-line variants
(whitespace-only, trailing, leading), folded trailers (space, tab), CRLF, CR in a value, control character, duplicate Awaits-Review / CPA-Run / Deferred-Gates, hold alone in the last paragraph with CPA-Run earlier, reserved key before the
block (any case, indented), block-less messages. The matrix section `19 r6` drives the launcher with the divider, CR, reserved key and blank-line variants.

R-02 root cause, found by reading the chain: the line the reviewer's mutant mB replaced is DEAD CODE. When `mergeblock=1` the `elif [ "$mergeblock" = 1 ] || [ "$cut" = 0 ]` branch above has already set the same target, so mB is equivalent,
and the dead line also hid the mutation of the real condition (dropping `mergeblock` from the `elif` survived too). Git history (`git log -S`) shows both lines landed in the same commit `26755ca5`, so there is no earlier wiring (11.4.124).
Fix: remove the dead line, keep the single decision with a comment, add end-to-end cases (held merge after 1 and after 3 unheld commits, plus golden-false cases) and mutants J22/J23; the driver's `--only PRE` entries put the dead
line back and show that mB AND the hidden condition mutant both survive while it exists (evidence in the mutation transcript).

R-05, R-06: a precondition that cannot be verified fails closed (11.4.252); a status is read before its output is used (the LIST class the author closed elsewhere).

R-07: the review used the 11.4.235(D) set in `finding_layer`, the schema (contracts rev 18) allowed only the evidence-class set. Resolved in the schema (contracts rev 19): both sets are valid in `finding_layer`, the evidence-class sense of a
235(D) finding goes in the new optional `evidence_layer`. The real verdict file validates (test case).

## R-04 (process-doc)

Commit `58cb30b8` "Auto-commit" brought the fix-round-5 bytes of `commit-push-all.sh` and its helpers into `main`. It was not made through CPA, was not independently reviewed first (11.4.142) and carries no `CPA-Run` trailer, so no run
id exists for it. The next review of WP-04 MUST read and list it. OPEN POINT: `contracts/review-verdict.schema.json` `covers_runs[].cpa_run` requires a run id, so a non-CPA commit cannot be listed there; the next review lists it in a
free field of its verdict (suggested name `covers_non_cpa_commits`: `[{"repository":".","commit":"58cb30b8..."}]`) and the plan owner decides whether the schema gets that field (not changed here: it would change what S6 releases). The same applies to
`20f74abf` and any later "Auto-commit" that touches the CPA files.

## Result

Measured, scratch repositories and local bare remotes only, nothing committed or pushed.
- RED (`fix-r6-red.txt`): against the pre-fix scripts the new helper test gives 61 ok / 131 failed; the review's needle is reproduced (`PUSHED` for a held commit whose message has a `---` line); matrix section `19 r6` gives 10 ok / 22 failed.
- GREEN (`fix-r6-green.txt`): three runs, `test_wp04j.sh` 192 ok / 0 failed and matrix section `19 r6` 32 ok / 0 failed each time; the helper test also passes under mawk.
- Regression (`fix-r6-regression.txt`): full matrix 1076 ok / 0 failed, push_recursive 153, commit_recursive 58, integrate_ff_only 84, wp04g 30, tooling_decoupled 20; integrate_merge (130) and wp04h (75) after aligning fixtures that wrote a trailer block the writer never writes.
- Mutation (`fix-r6-mutation.txt`): 29 mutants, 28 caught, J09 documented equivalent (empty output on a non-zero status); R-02: the reviewer's mB is EQUIVALENT and the mutant of the condition it hid survived while the dead line existed (both shown with `--only PRE`); on the fixed tree J22 and J23 are caught. The older driver `run_wp04i_mutations.sh` was NOT re-run (UNCONFIRMED that its mutant texts all still apply).
- R-03: pinned shellcheck (IMG-SHELLCHECK, digest bb596a0d...) run on the six files (`fix-r6-shellcheck.txt`): no warning in lines this round changed; the remaining warnings (SC2034, SC2154, SC1007, SC2064, SC2221) are in untouched lines. kcov: BLOCKED, interrupted (`fix-r6-kcov.txt`), plus a new finding: in a container the matrix cannot lock (process group 1), pre-existing, not fixed here.
- Not changed: the real origin push URLs name `milos85vasic` (owner decision, R-07 note); `commit-push-all.sh` is a G-GATE file, so the owner re-approval of its new bytes is still owed.
