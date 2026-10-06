# Decision brief: reuse of the continuum chain primitive (DR-E1) - four open questions

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06 |
| Status | DRAFT decision brief. Not sent, not filed. No owner answer exists yet. This document recommends; it does not decide. |
| Status summary | Frames the four questions left open by `evidence/wp05/T048-reuse-decision.md` (section 4) and `evidence/wp05/T050r3-DR-E1-open-deviation.md`, gives concrete options with costs, and recommends one path with its preconditions. |
| Issues | the shipped chain deviates from the T048 REUSE decision and is in tension with constitution 11.4.268 (open since 2026-10-05, DR-E1) |
| Issues summary | open deviation, owner question not yet filed in `decisions/owner-request-list.md` or `owner-decisions.yaml` (per the DR-E1 file, the conductor must file it) |
| Fixed | n/a |
| Fixed summary | n/a |
| Continuation | Owner answers the four questions (or the single DR-E1 question); then T049/T050/T054/T055 follow the answer. README links owed: last section. |

## Table of contents

1. The decision in one paragraph
2. Evidence base and its limits
3. Question 1: hash construction
4. Question 2: record schema
5. Question 3: exit codes
6. Question 4: anchor format and location
7. Whole-decision options and total cost
8. Recommendation
9. Preconditions before any option is executed
10. UNCONFIRMED
11. README links owed

## 1. The decision in one paragraph

Constitution 11.4.268 (its honest-boundary paragraph) says a project "MUST NOT stand up a parallel, divergent chain implementation when one already satisfies" the anchor properties, and names the continuum engine (11.4.207) as the mechanism to reuse. T048 (2026-10-05, read-only) concluded REUSE, conditional on four spec-level mismatches being decided first. T049 and T050 were built before those were answered: `tools/evidence/evcore.py` implements the frozen `ev/1` chain of docs/06 section 7, a second chain, byte-incompatible with continuum. The owner question is whether to keep that chain (and record an 11.4.268 exception with a reason), adopt continuum's primitive (and schedule the change set), or take a middle path. The answer must precede T054 (verdict deriver) and T055 (anchor writer), which build on whichever chain stands.

## 2. Evidence base and its limits

Facts cited from the two evidence files (read as written; not re-measured except where stated):

- Continuum at commit 4639347 (module `github.com/vasic-digital/continuum`, Go 1.22, standard library only, zero own-org deps). Chain and anchor feature commit f9149f8. The chain and anchor files named there: `pkg/chain/chain.go` (Record at :76, Digest at :143, Decode at :164, Verify at :224), `pkg/anchor/anchor.go`, `check.go`, `strength.go`, `pkg/verify/chain_verify.go`, `cmd/continuum-integrity/main.go`.
- The shipped tool: `tools/evidence/evcore.py` plus the `evrec` and `verify` entry points (confirmed present: directory listing `tools/evidence/` shows `evcore.py evrec tests verify`; `verify` calls `evcore.verify_main`).
- The T048 fit table: continuum satisfies edit, delete, reorder, delete-and-recompute (via anchor), tail truncation (via anchor); both implementations share the two documented non-detections (whole-ledger-plus-anchor replacement; a recompute with no anchor).
- The DR-E1 file corrects an earlier understatement: switching to continuum's record changes much more than `chain_walk` and `compute_entry_hash` (list in section 7 below).

Limits: continuum's tests were NOT run (no bare-host Go build, 11.4.173), no continuum binary exists on this host, and the container build path does not exist yet. Every statement about what continuum "does" is from reading its source and docs, not from running it. The T048 file also marks some line numbers approximate (`strength.go` probe, `anchor.go` Anchor type); they are not made exact here.

## 3. Question 1: hash construction

| | Shipped `ev/1` (docs/06 section 7) | Continuum |
|---|---|---|
| Digest | `entry_hash = SHA-256(prev_hash || canonical_json(entry without entry_hash))`, keys sorted, compact, UTF-8 | SHA-256 of canonical JSON of the WHOLE record including `prev_digest`; no stored own-hash field (recomputed on verify) |
| Genesis | `prev_hash` = 64 zeros | `prev_digest` = empty string (`GenesisPrev = ""`, chain.go:57) |
| Stored hash | each row carries `entry_hash` | none |

Both are collision-resistant chains of equal strength; they are byte-incompatible.

Options:
- 1A. Adopt continuum's construction; amend docs/06 section 7 and the `ev/1` contract (`prev_hash`/`entry_hash`/`seq` are required fields of the contract and of the revision 7 schema pin per DR-E1). Cost: every hand-hashed fixture (`test_evrec_more.sh`, `test_evrec_r3.sh` golden-good cases, pinned vectors in `test_evcore_golden.py`), `compute_entry_hash`, `chain_walk`, schema, and doc text.
- 1B. Keep the shipped construction; record the 11.4.268 exception and why (a written reason is required; candidate reasons: the producer needs a stored per-row `entry_hash` for cheap partial verification and blob binding; the shell recorder cannot call a Go binary without a container build). Cost: an explicit deviation forever, a second implementation to maintain, and the standing risk that continuum's canonicalisation and ours drift (chain.go header, "Reused, not re-implemented (11.4.251)", is exactly the fork this would create).
- 1C. Make continuum's construction available as a second verifier: keep our rows, and add a continuum-shaped projection (see question 2, option 2b). Cost: two hash computations per entry; benefit: the continuum anchor and verifier can run unchanged over the projection.

Recommendation for Q1: 1A if the whole-decision path (section 8) is adopted; 1B only with a written exception.

## 4. Question 2: record schema

Continuum `Record` has nine fields, all inside the digest, no `omitempty`: seq, ts, command, exit_status (int), artifact_path, evidence_class, author_session_id, independence_tier, prev_digest. `Decode` rejects unknown fields (chain.go:164), so an `ev/1` row (argv, cwd, stream digests, target fingerprint read at run time, durations) does NOT decode as a continuum record; `pkg/chain/exec_call.go` documents that a producer exec row shares only `ts` and `command` with Record, and `exit_status` is a string on the wire against an int in Record.

Options:
- 2a (T048 section 4.2 option a): the audit ledger writes continuum Records; the rich evidence stays in a content-addressed blob referenced by `artifact_path`, so the rich fields are digest-bound by being in the record. No upstream change. `$FEAT/contracts/evidence-record.schema.json` (T048a) must then say which fields live in the blob. Cost: schema split, a blob per entry (docs/06 already has `$EV/blobs/<sha256>`; tasks.md notes in-tree blobs under `$EV/blobs/<sha256>`, large blobs blocked on OD-76), verifier must dereference the blob.
- 2b: keep the shipped rich row as the system of record, and emit a continuum-shaped chain row per entry (projection with `artifact_path` pointing at the rich row). Two chains over one logical ledger; double the write path; hash relation between them needs its own check.
- 2c (T048 option b): extend continuum upstream with an `evidence` payload-digest field (11.4.74 extend, not fork). Cost: an upstream change request and a release before use; schedule risk; benefit: one chain, rich rows first class.
- 2d: keep the shipped schema only (pairs with 1B).

Recommendation for Q2: 2a. It needs no upstream change and honours 11.4.74/11.4.251. 2c is the clean long-term shape and can follow as a separate request.

## 5. Question 3: exit codes

docs/06 (T048 cites :532-534): 0 verified, 1 chain failure, 2 anchor disagreement, 3 unverifiable. Continuum CLI (`cmd/continuum-integrity/main.go:85-90`): 0 PASS, 1 operational error, 2 usage, 3 DETECTED, 4 REFUSE, 5 SKIP (the comment at :52-57 explains why SKIP is not 0). The shipped evrec refusal codes occupy 64-77 (DR-E1 file). Note the collision: docs/06's `1` means chain failure, continuum's `1` means operational error, and continuum's `3` (DETECTED) maps to docs/06's `1` or `2`, while docs/06's `3` (unverifiable) maps to continuum's REFUSE 4 or SKIP 5.

Options:
- 3a. Adapter in `tools/evidence/verify` that runs continuum and maps codes by subcommand context (chain verify: DETECTED to 1, REFUSE to 3; anchor verify: DETECTED to 2, REFUSE to 3; SKIP to 3 with a distinct reason string; operational error and usage to a code outside 0-3, for example 64-77 range already used by evrec). Docs/06 stays the external contract. Cost: a small table plus tests of every cell; risk: a mapped code hides continuum's SKIP/REFUSE distinction, so the adapter must print the original verdict.
- 3b. Amend docs/06 to continuum's codes and update T049/T050/T054 and tests. Cost: wider doc and test churn; benefit: no translation layer.
- 3c. Never pass continuum codes through unmapped (T048 states this as a rule under any option).

Recommendation for Q3: 3a, with the rule of 3c. It keeps the frozen external contract and confines the change to one file.

## 6. Question 4: anchor format and location

docs/06 section 8: an anchor is a periodic record `{head hash, entry count, time, strength}` in a JSONL log `$EV/anchors.jsonl`, strength `mechanism` or `policy`, default `policy` on this single-uid host. Continuum `anchor write` keeps ONE anchor file (single JSON object, atomically replaced: temp, fsync, rename, directory fsync), no time field, no history; `Write` refuses regression (count goes down, ErrAnchorRegression) and rewrite (same count, different head, ErrAnchorRewrite), is idempotent on an identical anchor. Strength set `policy|mechanism|unknown`; a recorded `mechanism` the probe did not establish is refused. The stock probe `GitNonFastForwardProbe` has no path that returns true, so with stock code only `policy` or `unknown` is recordable. There is no seq-contiguity detector (by design) and no interval scheduler.

Options:
- 4a. Adapt to continuum: one anchor file; history is the git history of that file (each anchor a commit, ff-only per 11.4.113), with the time carried by the commit. Cost: anchor history depends on commit discipline (T055 commit turns); benefit: no new format.
- 4b. Wrapper appends each continuum anchor to `$EV/anchors.jsonl` with a time field (keeps docs/06 intact), and keeps continuum's single file as the latest. Cost: two writers of anchor state; a mismatch check is needed.
- 4c. Extend continuum upstream with an append-only anchor log. Cost: upstream change request; cleanest result.
- 4d. Keep the shipped anchor design (docs/06 JSONL, T055 not built yet) with a shipped writer. Pairs with 1B.

Location facts: anchor and chain live on the same host and uid; the threat model records that an adversary rewriting both is undetectable (docs/chain_threat_model.md :65-67, as cited by T048). An off-host anchor (a git remote with enforced fast-forward-only, DR-E1 open item "anchor location the owner can provide") is not provided by continuum and remains an owner input (docs/06 DR-E2). Until a real probe exists the honest recorded strength is `policy`.

Recommendation for Q4: 4b now (keeps docs/06 and the T055 plan, history has a time field, continuum's refusal of regression and rewrite still guards the latest anchor), with 4c as the upstream follow-up. 4a is acceptable if the owner prefers fewer moving parts and accepts commit-bound history.

## 7. Whole-decision options and total cost

| Path | Q1 | Q2 | Q3 | Q4 | Summary |
|---|---|---|---|---|---|
| R. Adopt continuum, adapter | 1A | 2a | 3a | 4b | Satisfies 11.4.268; changes the `ev/1` record, hashing, hand-hashed fixtures, exit-code map, anchor writer, and every consumer of `entry_hash` (verdict deriver T054, register importer); T054 and T055 must wait for the decision |
| K. Keep shipped chain, record exception | 1B | 2d | n/a | 4d | Zero code churn now; permanent 11.4.268 deviation with a written reason; ongoing double maintenance; the DR-E1 file's own wording: "the owner accepts a second implementation against 11.4.268 and records why" |
| H. Hybrid (continuum projection) | 1C | 2b | 3a | 4b | Both chains exist; strongest verification, double write path; largest ongoing complexity; least attractive unless R is blocked by the Go build path |

The DR-E1 file lists what path R changes together (this brief repeats it, not extends it): the `ev/1` record shape (`prev_hash`, `entry_hash`, `seq` are required contract fields, T048a and the revision 7 schema pin them); `compute_entry_hash` and `chain_walk`; every hand-hashed fixture (`test_evrec_more.sh`, `test_evrec_r3.sh` golden-good cases built with `jq | sha256sum`, the pinned vectors in `test_evcore_golden.py`); the exit-code map (verify 0/1/2/3 and the evrec refusal codes 64-77); the anchor file format and writer (T055, not yet built); every consumer of `entry_hash`. Costs in hours or lines were not measured (UNCONFIRMED); they are sized only by that list.

## 8. Recommendation

Choose path R in this order, because the deviation is cheapest to close before T054 and T055 exist:

1. Q2 first (option 2a): it decides what a ledger row is, and everything else follows it.
2. Q1 follows Q2 (1A), Q3 (3a) and Q4 (4b) are small once the record is fixed.
3. Obtain a built `continuum-integrity` binary through the containerised build path (11.4.173) and run its own tests there before any wrapper work, so the claim "continuum satisfies docs/06's table" stops being a reading and becomes captured evidence. If the build path cannot exist in time, path K with a written exception is the honest fallback, with R scheduled as tracked work, not a silent deferral (11.4.197).
4. File two upstream requests only if the owner wants them: the payload-digest field (2c) and the append-only anchor log (4c). Neither blocks R.

Why not K: it is the cheapest now and the most expensive later; each module built on `entry_hash` (T054, T055, the register importer) raises the cost of the later switch and the 11.4.268 text calls this exact shape a divergent chain. Why not H: two chains over one ledger doubles the surface the independent reviews must cover.

If the owner chooses K, the required artefact is a recorded exception stating the reason (candidate reasons listed under 1B) and an expiry or revisit trigger, filed as a tracked item.

## 9. Preconditions before any option is executed

- The DR-E1 owner question is filed in `decisions/owner-request-list.md` and `owner-decisions.yaml` (the DR-E1 file says those files were out of scope of that change set, so the conductor must file it).
- The task path discrepancy is reconciled: tasks.md names `$EV/wp05/DR-E1.md`, the file that exists is `T048-reuse-decision.md` (DR-E1 file item 4).
- The continuum commit pin 4639347 is recorded in the evidence ledger when a wrapper is built (T048 follow-up).
- DR-E3 (one writer, liveness-checked lock) is mapped onto `pkg/lock` or the shell lock; whether `pkg/lock` suits the ledger is UNCONFIRMED (T048 did not read it in detail).
- Anchor interval trigger (end of session plus the declared interval) and the covered-call set declaration are supplied by the project; continuum ships the `union_rule` library but not the data (T048 gaps).

## 10. UNCONFIRMED

- The owner's answer to any question.
- Whether continuum's tests pass on this host (not run).
- Whether continuum's Record plus digest can carry the `ev/1` fields without loss under option 2a (T048 lists the mismatches; no probe of a real binary was possible).
- Whether any built `continuum-integrity` binary exists anywhere in the toolchain.
- Fit of `pkg/lock` and `pkg/store` for the ledger writer.
- Exact line numbers marked approximate in T048 (`strength.go` probe, `anchor.go` Anchor type).
- The size in lines or hours of path R (only an enumerated change list exists).
- That the exit-code mapping in section 5 matches every existing assertion in T049/T050/T054 tests (the mapping is proposed, not checked against all tests).
- Whether the 11.4.268 text in the current upstream main differs from the pinned submodule copy read for T048 (no network used).

## 11. README links owed (nothing links this file yet)

- `specs/001-full-project-audit-remediation/README.md`: list under "Upstream change requests and decision briefs" together with `constitution-prefix-change-request.md`.
- `specs/001-full-project-audit-remediation/docs/06-determinism-and-evidence-framework.md`: from section 7 (chain), section 8 (anchors) and the DR-E1 row (line ~2241).
- `specs/001-full-project-audit-remediation/docs/21-master-plan-phases-risks-and-traceability.md`: from the risk or decision row for DR-E1.
- `specs/001-full-project-audit-remediation/decisions/owner-request-list.md`: the DR-E1 owner question, linking this brief as its supporting document.
- `specs/001-full-project-audit-remediation/evidence/wp05/T048-reuse-decision.md` and `T050r3-DR-E1-open-deviation.md`: a "see also" line (these are evidence files; edit only through the process that owns them).
