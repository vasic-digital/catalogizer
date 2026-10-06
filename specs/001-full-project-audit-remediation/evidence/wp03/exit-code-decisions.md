# verify_repos.sh exit-code decisions (T032)

Revision 2, 2026-10-05 (review fix of WF-REVIEW-wp02-wp03 finding I4: the reason of decision (b) was wrong and is corrected here).

The two values that docs/21 section 12.2 and data-model section 9 leave UNCONFIRMED, fixed by the T031 matrix and implemented in `scripts/repo/verify_repos.sh`. Both are **UNREVIEWED-by-owner**: they are decisions of this untracked work, pending the review that T011a and the WP-03 reviewer record and the owner's confirmation. The exit set stays exactly {0, 11, 12, 13, 14, 15, 20} either way.

| Decision | Value | Reason | Asserted by (T031) |
|---|---|---|---|
| (a) order among failing classes present at once | the first of 13 (dirty, stash included), 12 (diverged), 11 (unpushed), 15 (pointer drift or uninitialised, `--strict` only), 12 (a `behind` row, `--strict` only); any failing class precedes 14 (IC-37) | the working tree must be cleaned before history questions can be acted on safely; divergence needs a merge decision before an ordinary push; the strict-only classes come last because the plain routine run never fails on them | section 18 of the matrix: all 15 pairs of the six classes {13, 12 diverged, 11, 15, 12 behind, 14} plus each single class and a clean base, every pair with a check that both conditions really exist in the report |
| (b) code for a `behind` row under `--strict` | 12 | **Corrected reason.** A `LOCAL-BEHIND` remote means the remote has commits the local history lacks and the local commit is an ANCESTOR of the remote tip: the recovery is a fast-forward (fetch, then advance), with no merge decision and no conflict. It is NOT the divergence recovery (a true `DIVERGED` needs a real merge, the conflict path of constitution 11.4.211). 12 is chosen only because the closed exit set has no code that fits better: 11 would mislabel it as unpushed, 14 as unverified, 15 means pointer drift (which for a detached owned submodule is related: "the pin is older than the branch tip" is reported by the `pin` problem under `--strict` and outranks `behind`), and no code outside the documented set may be minted. | "upstream moved ahead, --fetch --strict: exit 12 (decision b)" |

Consequences, stated rather than hidden:

- 12 is ambiguous between "diverged" and "behind" for any consumer that reads only the exit code. The JSON is unambiguous: `problems` contains `diverged` or `behind`.
- On today's tree (2026-10-05) `--fetch --strict` would exit 12 for `submodules/constitution` (8 comparisons `UNKNOWN-DIFFERENT` without `--fetch`, `LOCAL-BEHIND` on 2026-10-03) although that repository is merely behind upstream and nothing is lost or conflicting. Plain mode is unaffected (a plain run never fails on `behind`).
- If the owner prefers that a behind row never share 12, the alternative inside the closed set is to route `behind` to 15 (pointer/ahead-of-pin class). That is an owner choice (exit-code table in docs/16 and data-model section 9), not made here.

Other values asserted by the matrix and taken from the documents: plain-mode 14 for every unproven remote class (UNREACHABLE, UNKNOWN-DIFFERENT, NO-REMOTE-BRANCH, and now a repository with no remote at all); 15 only under `--strict`, with a plain run reporting `summary.pin_drift > 0` and exiting 0; R4 (a pin no remote holds) gives 11 or 12 with `--fetch`, 14 without it when the remote tip object is absent locally, never 15; a blind control needle, an unexaminable repository (any git error), a missing worker result and any usage error give 20 (the POC's 2 is gone). Detached owned submodule: pin AND HEAD are compared, the worse class wins.

UNCONFIRMED: the owner has not accepted (a) and (b); the matrix now also runs in IMG-TESTUTIL (round 4, 375 ok / 0 FAIL) and shellcheck 0.11.0 at -S warning is clean; kcov could not attach in the sandboxed image (`evidence/wp03/round4-notes.md`).
