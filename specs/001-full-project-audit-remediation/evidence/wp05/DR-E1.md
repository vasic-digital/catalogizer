# T048 / DR-E1: the evidence chain and the constitution continuum store (read-only reading task, owner decision of 2026-10-05 applied)

| Field | Value |
|---|---|
| Date | 2026-10-07 |
| Task | tasks.md T048 (the task text names `$EV/wp05/DR-E1.md`; this file is that record). It supersedes `T048-reuse-decision.md` (2026-10-05, same author line of work) where they differ and keeps it as the first reading. |
| Subject | `submodules/constitution/submodules/continuum` at `4639347` (module `github.com/vasic-digital/continuum`, go 1.22, standard library only, `helix-deps.yaml`: zero own-org dependencies), constitution 11.4.207 and 11.4.268 |
| Method | READ the source and tests (file and line below), RAN its own suite and its integrity CLI in IMG-GO against a real `ev/1` ledger: `tools/evidence/tests/continuum_probe.sh`, transcript `$EV/wp05/t048-continuum-probe.txt` (identity header, `# DONE`). Nothing of continuum was changed or copied. |
| Owner decision (2026-10-05) | the evidence chain SWITCHES to the continuum store with four points OPEN: hash construction, record schema, exit codes, anchor format |
| What this record does NOT do | invent continuum behaviour, settle the four points (they are the owner's and, for an upstream extension, continuum's), or build a continuum adapter. Everything below says what continuum DOES, from its source and from the run. |

## 1. What continuum offers (verified at 4639347)

| Capability | Where | Verified by |
|---|---|---|
| Chain record: nine fields (`seq`, `ts`, `command`, `exit_status`, `artifact_path`, `evidence_class`, `author_session_id`, `independence_tier`, `prev_digest`), declaration order is the canonical byte order, no `omitempty`, every field inside the digest | `pkg/chain/chain.go:76` | read |
| Digest = SHA-256 of the canonical JSON of the WHOLE record including `prev_digest`; genesis `prev_digest` is the empty string; no stored own-hash field | `pkg/chain/chain.go:57,143` | read |
| Strict decode: one canonical record per line, unknown fields REFUSED (`DisallowUnknownFields`) | `pkg/chain/chain.go:164-172` | read, and RUN (section 2) |
| Chain-alone walk in FILE ORDER, verdicts PASS / DETECTED / REFUSE, resync after a break to report several | `pkg/chain/chain.go:224` | read; its suite passes |
| Anchor: `{head_digest, entry_count, anchor_strength}`, closed strength set `policy / mechanism / unknown`, `Write` forward-only (refuses a lower count and a rewrite of an anchored head, idempotent on an identical anchor) with read-back confirmation | `pkg/anchor/anchor.go:54,153`, `pkg/anchor/strength.go` | read; its suite passes |
| Anchor check: a chain SHORTER than the anchor is DETECTED; a chain longer with an intact prefix is valid growth (lagging anchor, with its own negative control test) | `pkg/anchor/check.go:96` | read; its suite passes |
| Strength is PROBED, never assumed; `GitNonFastForwardProbe` has NO path that returns "mechanism" (a plain git remote cannot be read for protection; an unreachable remote is `unknown`, not `policy`) | `pkg/anchor/strength.go:63,94,138` | read |
| CLI `continuum-integrity`: `chain verify`, `anchor write`, `anchor verify`; exit 0 PASS, 3 DETECTED, 4 REFUSE, 5 SKIP, 2 usage, 1 operational error; SKIP is deliberately not 0 | `cmd/continuum-integrity/main.go:52-90,203` | read, and RUN |
| Covered-call union rule (write / exec / deploy require an entry, cited read requires one, uncited read none) | `pkg/chain/union_rule.go`, `pkg/chain/exec_call.go` | read |
| Store with lock and atomic writes (`pkg/store`, `pkg/lock`) | `pkg/store/store.go`, `pkg/lock/lock.go` | read only: fit for the multi-writer ledger of DR-E3 UNCONFIRMED |
| Its own tests | 25 test files | RUN: `go test -count=1 ./...` in IMG-GO (go1.25.14): every package with tests `ok`, `[no test files]` for `cmd/continuum`, `cmd/continuum-unionrule`, `pkg/config` (transcript `t048-continuum-probe.txt`). The earlier UNCONFIRMED test status of `T048-reuse-decision.md` is therefore settled. |

## 2. The four open points: what continuum settles and what it does not

| Point | What continuum fixes (its own choice) | What the repository has | Settled for this repository? | Evidence |
|---|---|---|---|---|
| 1 Hash construction | `sha256(canonical_json(record with prev_digest))`, genesis `""`, digest recomputed, never stored | `entry_hash = sha256(prev_hash ‖ canonical_json(entry without entry_hash))`, genesis 64 zeros, `entry_hash` stored (docs/06 section 7, ev/1) | NO. Equivalent in strength, byte-incompatible. Open: adopt continuum's construction and amend docs/06 section 7, or extend continuum upstream (11.4.74) to carry ours. | read |
| 2 Record schema | the nine fields above, strictly decoded | ev/1 (docs/06 section 3: argv list, cwd, stream digests, target fingerprint read at run time, test fingerprint, oracle, evidence class, redaction flag, ...) | NO, and shown: `continuum-integrity chain verify` on a real ev/1 ledger gives `REFUSE` exit 4: `malformed record: line 1: json: unknown field "argv"`; `anchor verify` the same. Gaps against `Record`: no `argv` (a single `command` string), no `cwd`, no stream digests, no target or test fingerprint, no `duration_ms`, no `oracle`, no `polarity` or `iteration` or `item`; `author_session_id` and `independence_tier` have no ev/1 counterpart. Options: (a) write continuum Records and keep the rich entry in a digest-referenced blob named by `artifact_path`; (b) extend continuum upstream with an evidence-payload digest field. | RUN |
| 3 Exit codes | 0 PASS, 3 DETECTED, 4 REFUSE, 5 SKIP, 2 usage, 1 operational | docs/06 section 8 and `verify`: 0 verified, 1 chain failure, 2 anchor disagreement, 3 UNVERIFIED, 64 usage | NO. An adapter mapping is the only way to keep both contracts. Proposed (not implemented): PASS 0; DETECTED 1 for the chain-alone result and 2 for the chain-plus-anchor result (the CLI reports the two as distinct results); REFUSE 4 and SKIP 5 and operational 1 all map to 3 UNVERIFIED (never to 0); usage 2 to 64. Continuum's chain-alone `chain verify` reports `chain_plus_anchor=SKIP`: with no anchor the repository's `verify` says exit 0 plus a note, continuum says 5, so the mapping of SKIP must be decided. | read, RUN |
| 4 Anchor format | ONE anchor object in a file, atomically replaced, forward-only, no time, no history, strength `policy / mechanism / unknown` | `$EV/anchors.jsonl`, append-only JSONL history of `ev-anchor/1` rows with `at` and an optional `probe` record, strength `policy / mechanism`; an unreachable probe remote is recorded as `policy` (continuum: `unknown`) | NO. History would need a git-committed sequence of continuum's single file or a wrapper; the `unknown` strength differs from the task text ("downgrades to policy"). The anchor LOCATION is BLOCKED-ON OD-76. | read, RUN |

What continuum DOES settle, as design knowledge the reference recorder already follows: an anchor needs an entry count (not only a head, or wholesale deletion is silent); a lagging anchor over an intact prefix is valid growth, a shorter chain is not; "undecided" is a third state that is never PASS; strength is evidence, never configuration; an anchor never moves backwards.

## 3. The adapter seam in the reference recorder (python), where continuum cannot be consumed yet

T049 to T056 are implemented on `tools/evidence/evcore.py` and `evanchor.py`. The places a continuum engine would replace, and nothing else, are:

| Seam | Where | What an adapter must provide |
|---|---|---|
| Hash construction and chain walk | `evcore.compute_entry_hash`, `evcore.chain_walk` | append and walk with continuum's digest; `evrec rerecord` re-chains with the same function |
| Record schema | `evcore.schema_errors`, `contracts/evidence-record.schema.json` | the ev/1 to Record mapping of point 2 (option a or b) |
| Exit-code contract | `evcore.verify_main` | the mapping of point 3 |
| Anchor rows | `evanchor.read_rows`, `verify_anchors`, `cmd_anchor`, `rerecord_anchor_leg` | anchor write through `continuum-integrity anchor write` (forward-only, read-back) and the history of point 4 |
| Verdict deriver | `tools/evidence/evverdict.py` | none: it consumes `evcore.chain_walk` entries only, so it keeps working over any engine that yields ev/1 entries |

Each seam is one function, so the choice can be made without touching the recorder CLI (`evrec`, `verify`, `verdict`) or the wrappers. The reference recorder does not call continuum.

## 4. Still open and owed
1. The four points above: owner decision (and upstream continuum work for options 2(b) and 4).
2. A real `mechanism` anchor needs an operator-gated probe or protection-API read and the off-host anchor location (OD-76); until then both continuum and the reference recorder honestly report `policy`.
3. continuum has no seq-contiguity detector by design; `verify` keeps its own contiguity check (docs/06 section 7).
4. Fit of `pkg/store` and `pkg/lock` for the single-writer ledger of DR-E3: UNCONFIRMED (not read in detail).
5. A prebuilt `continuum-integrity` binary does not exist in the toolchain; it was built in IMG-GO for the probe only (11.4.173) and not kept.
6. The continuum commit pin `4639347` is to be recorded in the ledger when a wrapper is built.
