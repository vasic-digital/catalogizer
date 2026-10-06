# T048 / DR-E1: reuse of the continuum chaining primitive (read-only decision)

Date: 2026-10-05. Read only; `go test` was NOT run (a bare-host build would break 11.4.173 and the task is read-only), so test pass status is UNCONFIRMED here and only cited from the repo's own docs.
Task: tasks.md:205 (T048). Task text names `$EV/wp05/DR-E1.md`; written at the caller-requested path. Spec refs: docs/06 section 7 (line 486), section 8 (line 520), section 13.3 (line 1986), DR-E1 (line 2241), reuse clause (line 120).
Subject: `submodules/constitution/submodules/continuum` at 4639347 (module `github.com/vasic-digital/continuum`, go 1.22, stdlib only, go.mod; helix-deps.yaml: zero own-org deps, depth-1 carve-out 11.4.28(C)). Chain/anchor feature commit f9149f8.

## 1. DECISION

REUSE the chaining and anchor primitive of continuum, via its CLI/Go packages, as the verifier and anchor writer of the audit ledger. Do NOT build a second chain. BUT reuse is conditional on resolving four spec-level mismatches (section 4) that must be decided before T049/T050 (they change the frozen docs/06 definitions), and on one missing capability (a real `mechanism` anchor probe, section 5). The shell recorder PoC of docs/06 section 13 becomes a producer that emits rows in continuum's record shape, not a competing chain.

This is a fit of the PRIMITIVE (hash chain verify + anchor), not of the whole store: `pkg/store`/`snapshot` (content-addressed resume store, 11.4.207) is a different artifact (`pkg/store/store.go`: PutBlob, WriteRef, AppendEvent...) and is not needed for the ledger.

## 2. What continuum provides (with file:line)

- Chain record: `pkg/chain/chain.go:76` `type Record` with 9 fields, all inside the digest: seq, ts, command, exit_status (int), artifact_path, evidence_class, author_session_id, independence_tier, prev_digest. No `omitempty` (every field protected); `Decode` (:164) rejects unknown fields and multi-record lines and malformed lines (DisallowUnknownFields).
- Digest: `Digest(r)` (:143) = SHA-256 hex of canonical JSON of the whole record (`model.Canonical` pkg/model/model.go:123; `hash.Sum` pkg/hash/hash.go:16 uses crypto/sha256). prev link lives INSIDE the record (`prev_digest`), genesis value is empty string (`GenesisPrev = ""`, :57).
- Chain-alone verify: `Verify(recs)` (:224) walks in FILE ORDER (never sorted), reports every broken link, resyncs after a break to report multiple, verdicts PASS / DETECTED / REFUSE (undecided never PASS). Findings kinds chain_GAP, genesis_BREAK, digest_UNAVAILABLE.
- File verifier: `pkg/verify/chain_verify.go` `VerifyChainFile`: absent store, unreadable, undecodable -> REFUSE, never PASS (readRefusalReason). 
- Anchor: `pkg/anchor/anchor.go:30ish` `type Anchor {head_digest, entry_count, anchor_strength}`; `Validate` (:100) refuses missing/empty digest, entry_count<=0, bad strength; `Write` (:153) atomic temp+fsync+rename+dir-fsync, refuses REGRESSION (count goes down, ErrAnchorRegression :90) and REWRITE (same count different head, ErrAnchorRewrite), idempotent on identical anchor.
- Anchor check: `pkg/anchor/check.go:96` `Check`: chain shorter than anchor -> DETECTED ("N entries are missing"); anchored prefix re-hashed (PrefixHead accessor) differs -> DETECTED; accessor missing/empty -> REFUSE; equal or chain longer with intact prefix -> PASS (lagging anchor treated as valid growth, with a negative-control test file `anchor_lagging_negative_control_test.go`).
- Anchor strength: `pkg/anchor/strength.go` `ProbeStrength`, `ValidateRecordedStrength` (:94): a recorded `mechanism` the probe did not establish is refused (ErrStrengthOverstated / ErrStrengthUnprobed). Closed strength set policy|mechanism|unknown.
- Union rule (covered-call set, 11.4.268 A): `pkg/chain/union_rule.go` classes write/exec/deploy as requiring an entry, cited read requires one, uncited read requires none (FR-026/027/028), `VerifyComplete` (:128). Adapter for execution rows: `pkg/chain/exec_call.go`.
- CLI `cmd/continuum-integrity/main.go`: subcommands `chain verify`, `anchor write`, `anchor verify` (chain-alone and chain+anchor reported as distinct results), `--remote` strength probe, env `CONTINUUM_CHAIN` / `CONTINUUM_ANCHOR`; exit codes 0 PASS, 1 operational error, 2 usage, 3 DETECTED, 4 REFUSE, 5 SKIP (:85-90, comment :52-57 on why SKIP is not 0).
- Tests present (counts by `wc -l`, files): attack_deletion, attack_reorder, attack_mutation, attack_tail_truncation, attack_delete_rechain_boundary, attack_matrix, record_alteration, union_rule, exec_call; anchor_record, anchor_strength, anchor_lagging_negative_control; verify, verify_refusal; store race/tail-refusal; cmd tests (725 lines main_test.go), e2e (365), CLI test, fixture self-check. 4111 test lines total over 25 test files. Whether they pass on this host now: UNCONFIRMED (not run). `docs/chain_threat_model.md` records the measured attack matrix (rows 0-5) as results of its own runs.

## 3. Fit against 11.4.268 and docs/06 section 7 table

| Attack (docs/06 :486-518 table) | continuum chain-alone | chain+anchor | Evidence |
|---|---|---|---|
| Edit content | DETECTED | DETECTED | threat model row 1; record_alteration_test.go |
| Delete, no repair | DETECTED | DETECTED | row 2; attack_deletion_test.go |
| Reorder | DETECTED | DETECTED | row 3; attack_reorder_test.go, file-order walk chain.go:224 |
| Delete + recompute | PASS (documented limit) | DETECTED | row 5; attack_delete_rechain_boundary_test.go; chain.go header |
| Truncate tail | PASS (documented limit) | DETECTED | row 4; attack_tail_truncation_test.go |
| Replace whole ledger and anchor | not detected | not detected unless anchor sits out of reach | threat_model.md :65-67 (anchor is also a file the same UID owns) |

This matches docs/06's own table, including its two "no" rows, so the 11.4.268 B/C properties (deletion, reorder, truncation, recompute caught by anchor) ARE satisfied. 11.4.268 D (cannot complete -> not PASS) is satisfied at the chain level (REFUSE) and the CLI (exit 4/5, distinct from 0). 11.4.268 A's union rule exists. 11.4.268 anchor strength honesty (mechanism claim needs evidence) exists in code (`ValidateRecordedStrength`).

## 4. Mismatches with docs/06 (must be decided; these block "drop-in" reuse)

1. Hash construction. docs/06 :492: `entry_hash = SHA-256(prev_hash || canonical_json(entry without entry_hash))`, genesis prev = 64 zeros, entry carries its own `entry_hash`. continuum: digest = SHA-256(canonical_json(whole record including prev_digest)), genesis prev = empty string, no stored own-hash field (digest recomputed). Semantically equivalent in strength, byte-incompatible. Decision proposed: adopt continuum's construction, amend docs/06 section 7 (document revision owner).
2. Record schema. docs/06 section 3 (evidence record: argv, cwd, stream digests, target fingerprint read at run time, durations...) vs continuum `Record` (9 fields, `command` a single string, no argv, no stream digests, no fingerprint, no cwd, no duration). `Decode` REFUSES unknown fields (`chain.go:164`), so a docs/06-shaped row does not decode as a chain record; exec_call.go:1-50 documents exactly this: a producer exec row (ts cwd command argv exit_status duration_ms stdout_digest ...) shares only ts and command with Record, and `exit_status` is a string on the wire vs int. Resolution options: (a) the audit ledger writes continuum Records and keeps the rich evidence in a separate content-addressed blob referenced by `artifact_path` (digest-bound by being in the record); (b) extend continuum upstream (11.4.74 extend, not fork) with an `evidence` payload digest field. Option (a) needs no upstream change; the rich schema contract `$FEAT/contracts/evidence-record.schema.json` (T048a) must then say which fields live in the blob.
3. Exit codes. docs/06 :532-534: 0 verified, 1 chain failure, 2 anchor disagreement, 3 unverifiable. continuum: 0 PASS, 3 DETECTED, 4 REFUSE, 5 SKIP, 1 operational error, 2 usage. Different contract; T049/T050/T054 and tests that assert docs/06 codes need either an adapter in `tools/evidence/verify` mapping codes, or docs/06 amended. Do not pass continuum codes through unmapped.
4. Anchor location and format. docs/06 :524 says anchor log is a JSONL `$EV/anchors.jsonl` (periodic entries with head, count, time, strength). continuum `anchor write` keeps ONE anchor file (a single JSON object, atomically replaced; Write refuses regression/rewrite), no time field, no history. Periodic history would need either a git-committed sequence of that file (history = the log) or a wrapper that appends each anchor to a jsonl. Also continuum has no `seq`-contiguity detector by design (chain.go header explains; seq gaps are covered by anchor count), whereas docs/06 :504-510 cites a contiguity check as a bonus: not available, do not rely on it.

## 5. What is missing (gaps)

- `mechanism` strength can never be produced by the shipped probe: `GitNonFastForwardProbe` (`pkg/anchor/strength.go` ~:120-169) has NO path returning true; it only returns (false,nil) reachable or error. So with the stock probe the recordable strength is `policy` (or `unknown`). A real `mechanism` claim needs a consumer-supplied probe that reads the host's branch-protection API with credentials or observes a rejected non-fast-forward push (operator-gated). This is consistent with docs/06 DR-E2 (default policy) and honest; it means the audit feature should plan for `policy` and track the upgrade.
- Anchor and chain live on the same host/UID: the threat model (docs/chain_threat_model.md :65-67) states an adversary rewriting both is undetectable; matches docs/06 row "replace entire ledger". An off-host anchor (the git remote with enforced ff-only, DR-E1 open item "anchor location the owner can provide") is not provided by continuum.
- No anchor interval scheduler, no per-session anchor trigger (docs/06 :536-538); the CLI writes on demand. T050/T05x must call `anchor write` at end of session and at the declared interval.
- No `read` of covered-call set declaration data; `union_rule` is a library, the project must supply the Call list (docs/06 :540-546 covered-call set) and a caller for `VerifyComplete`.
- No sequence allocation for a multi-writer ledger in the chain package; the store has lock-held seq derivation (`pkg/store`, cited at chain.go header: "T006/T316") and `pkg/lock`. DR-E3 (one writer, liveness-checked lock) must be mapped onto `pkg/lock` or the shell lock; UNCONFIRMED whether `pkg/lock` suits the ledger (not read in detail).
- CLI is Go: needs a build of `cmd/continuum-integrity`. Under 11.4.173 that build MUST run in the containerised build path, and the audit tooling is mostly shell; the shell recorder would call the built binary. Whether a prebuilt binary is vendored/available: UNCONFIRMED. 
- Test status on this host: UNCONFIRMED (not run).

## 6. Rejection of the alternative

A second chain (shell-only) is rejected: 11.4.268's final paragraph and docs/06 :117-121 forbid a divergent parallel implementation where an existing one satisfies the properties, and continuum's chain.go (header, "Reused, not re-implemented (11.4.251)") already refuses forks of canonicalisation/hash. Satisfied properties: deletion, reorder, tail truncation + recompute via anchor, refuse-when-cannot-complete, strength honesty, atomic anchor write, no-regression. Open properties: items 1-4 of section 4 and the five gaps above.

## 7. Follow-ups (route to tasks, not done here)

- Owner decision on section 4 items 1-4 before T049 (test), T050 (evrec/verify), T048a (schema revision). Suggested default: adopt continuum Record+digest+exit codes with an adapter; keep rich fields in a digest-referenced blob.
- Track as tracked item: real `mechanism` probe + off-host anchor remote (DR-E2).
- Record the continuum commit pin (4639347) in the evidence ledger when the wrapper is built.
- Run `go test ./...` of continuum in the build container to turn the UNCONFIRMED test status into evidence.

## UNCONFIRMED list

- Test results of continuum on this host.
- Fit of `pkg/lock` and `pkg/store` for the ledger writer (DR-E3).
- Whether a built `continuum-integrity` binary exists anywhere in the toolchain.
- Exact line numbers where marked "~" (strength.go probe, anchor.go Anchor type).
