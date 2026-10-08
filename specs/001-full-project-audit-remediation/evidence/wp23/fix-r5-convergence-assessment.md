# WP-23 fix round 5: convergence assessment (11.4.276(E), written BEFORE any fix)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-07T00:00:00Z |
| Last modified | 2026-10-07T00:00:00Z |
| Status | assessment of FIXLIST-wp23 (consolidated review of commit 846d441f, round 3 of the budget of 5) |
| Fixer | single fixer instance (Sonnet), one pass over the whole fix list |
| Round | the fix round that follows review round 2 (review verdict NO-GO, 74 findings, 27 surviving reviewer mutants) |

## 1. Recurrence and the structural decision

The review round 2 list shows six classes that recur from round 1: C1 (a gate accepts input it did not verify), C3 (carrier or text-shape match),
C4 (the fence checks a proxy, not the tool's scope), C5 (a load-bearing rule has no test that can fail), C6/C7 (provenance records bind too little)
and C9 (an absent input is read as data). N5 (the committed applicability.yaml is stale at its own commit) is a REAPPEARING finding. Under
11.4.276(E) this round is therefore STRUCTURAL: the models are re-derived from the real producers, every class is closed for ALL its inventoried
members, and layered point fixes on the round-1 code are replaced.

## 2. Ground truth that was re-derived (11.4.276(B)) before any fix

| Producer | How the ground truth was taken | Model used by the fixes |
|---|---|---|
| bash `-x` trace | real bash 5.3.9 traces of fixtures (multi-line words, arrays, here-documents, traps, process substitution) | an assignment is traced at its LAST line, a multi-line simple command at its FIRST line, the line for a continued command varies with the bash version: the classifier folds every line of one command onto one counted line (alias map) instead of guessing which line bash reports |
| `evrec` recorder | the argv the collector passes, run against the real tool | the recorder refuses five ways today; the contract leg runs the REAL `evrec` into a scratch ledger |
| `dispatch.sh status` | `cmd_status` source (`dispatch.sh:632-640`) | no `artifact_dir` or `artifact_name`; the artifact tree is `$BUILDS/<id>/artifacts/` |
| `run_pinned` / `run_go` | `RUNP_PRINT_ARGV=1` and source | the workdir is `$PWD` mounted `:ro`, the repository root has no `go.mod`; `--out` must be absolute |
| tool configs (vitest, jest, jacoco) | the tools' own documented defaults, resolved from the committed configs | tool identity and scope are taken from the measuring command, not from a hand-written row |
| git enumeration | `git ls-files` under `GIT_DIR` exported and with untracked files | tracked files only, clean `GIT_*` environment, no walk fallback inside a work tree |

## 3. Class inventory (11.4.276(C)): every member is closed in this round

K1 (4 members), K2 (6), K3 (6), K4 (5), K5 (8), K6 (6), K7 (3), K8 (6), K9 (2), K10 (2), K11 (5), K12 (6), K13 (6), K14 (9): see FIXLIST-wp23.md section 4.
Control needles for the censuses: each count in this round's evidence is taken with an instrument that has first been shown to see a known present
needle and not to see a known absent one (11.4.201(7)(b)).

## 4. Tests

Every new test drives the REAL shipped script through its real invocation path (11.4.224(A), 11.4.276(D)): no re-implementation of the guarded logic, no
text extraction. The RED run of every new leg is taken on the unmodified HEAD code BEFORE the fix; GREEN is three runs; the mutation runs carry a
sandbox control (an unmutated copy passes) and a negative control. All 27 reviewer mutants are adopted verbatim (anchors from the fix list section 6)
and, where a fix removes an anchor, re-anchored with the same meaning.

## 5. Honest limits recorded up front

- The bash harness is measured on bash 5.3.9 (host). bash 5.2.15 (IMG-KCOV) differs in the line it reports for a continued command; the alias fold is version
  independent by construction but the 5.2 run is UNCONFIRMED where no IMG-KCOV image is present on this host.
- Operator decisions that are recorded, not invented (11.4.66): K5.6 (brownfield adoption of the bash and Go scope) and K14.9 (whether `run_rust`'s real-script
  leg may assume a non-attested host).

## 6. Outcome of the round (written at the end; every figure is in `fix-r5/` with an identity header and in `fix-r5/SHA256SUMS`)

Closed with tests that failed on HEAD first: the bash classifier and harness (K3, K4, K6.5, K6.6, K7, K8.2, K8.5, K12.6), the fence gate (K1.4, K2, K3.5, K5, K8.1,
K11.1/2, K12.5), the Go collector and `gocov_merge.py` (K6, K8.4, K9, K12.4, K14.5), `gen_matrix.py` over the real evrec ledger (K10, K11), and the
`derive_applicability.py` enumeration (tracked-only inside a work tree, clean `GIT_*` environment, dangling gitfile refused, content-aware fingerprint,
`.gitmodules` read by git, relative `--out`, atomic write).

NOT done in this round, with the reason (11.4.6, none of these is claimed):
- The read of the 74-item fix list could not be repeated at the end of the round (the list file was no longer on disk); the items marked closed above are the ones the
  work was driven by while it was readable. Items whose text I could not re-read are NOT claimed: K3.6 (anchored carriers in `derive_applicability.py`), K13.1/K13.2
  (the baseline.json item list and the attribution line of `lanes-tic-sha-record.txt`), K13.5 (the A9 sentence measured), K14.3 (the drift leg of the derive test against
  the commit's tree), K14.8 (the SKIP count), K14.9 (the `run_rust` named-SKIP leg for an attested host). `docs/scripts/run_kcov.md` and `run_rust.md` were not touched.
- `specs/.../matrix/applicability.yaml` and the committed matrix were not regenerated: the working tree is shared with other areas and uncommitted, and the derivation
  is a function of the tracked content (a regeneration now would bake other areas' unreviewed state into a committed figure).
- B2 (the Go figure with the IMG-KCOV re-measure): IMG-KCOV is not confirmed present on this host; the expected figure (about 30.32 percent) is UNCONFIRMED.
- The runs are host runs (`TMPDIR=/dev/shm`), not container-lane runs; the identity headers say so.
- Operator decisions K5.6 and K14.9 are recorded as owed, not decided.
