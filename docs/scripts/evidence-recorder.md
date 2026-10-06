# evrec, verify, evanchor: evidence recorder - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T01:00:00Z |
| Status | tracked from WP-05 (T050 recorder rounds 1 to 7, T051a token, T055 anchors); independent review owed (constitution 11.4.142, T058) |
| Source | `tools/evidence/evrec`, `tools/evidence/verify`, `tools/evidence/evcore.py`, `tools/evidence/evanchor.py` |

> `$EV` in this guide means the evidence root `specs/001-full-project-audit-remediation/evidence` (repository-relative). The contract is `specs/001-full-project-audit-remediation/contracts/evidence-record.schema.json` (ev/1, revision 7); the design is `specs/001-full-project-audit-remediation/docs/06-determinism-and-evidence-framework.md`.

## Purpose
`evrec run` executes one command and appends ONE hash-chained `ev/1` entry to `$EV/ledger.jsonl` (argv, cwd, start time, exit status, duration, stdout and stderr digests, the target fingerprint read from the target at run time, the test fingerprint, redactions). `verify` walks the whole chain, the ev/1 schema, the canonical form, the blobs and, when an anchor file exists, the anchors. `evrec anchor` writes the anchor rows that catch what the chain alone cannot (a delete-and-recompute forgery and a tail truncation).

## Commands
| Command | Meaning | Exit statuses (named reasons are printed as `reason=<name>` on stderr) |
|---|---|---|
| `evrec run ITEM POLARITY ITER CLASS REF [--oracle S --oracle-independent --evidence-class C --test-source F ... ] -- argv...` | run and record | 0 recorded; 64 usage; 65 record_invalid; 66 ledger_inconsistent; 67 schema_unavailable; 69 interpreter_without_test_sources / operand refusals; 75 lock_held; 76 commit_turn_held; 77 target_unreadable |
| `evrec token` | print a fresh random 128-bit hex run token (T051a) | 0 |
| `evrec anchor [--strength policy\|mechanism] [--probe-remote DIR]` | append an anchor row for the current ledger head | 0; 1 or 3 the ledger does not verify (nothing written); 2 the existing anchor rows disagree; 64 usage / probe_remote_not_local; 71 strength_unproven |
| `evrec rerecord --onto F --local F --out DIR [--onto-anchors F --local-anchors F] \| --append-only ...` | re-record the local entries onto the remote side after the common prefix; the anchor leg re-chains the anchors | 0; 64; 65; 68 side_unverifiable / no_common_prefix / map_incomplete; 70 |
| `evrec remap-refs --map FILE --files-from LIST` | rewrite every `ledger#<seq>` reference through the seq map (anchor entries of the map are ignored) | 0; 64; 68 |
| `evrec reconcile [--run TOKEN] --runner-count N`, `evrec check-record FILE` | recorder reconciliation, record check | 0 / 65 |
| `verify [--ledger FILE] [--blobs DIR]` | verify chain, schema, blobs, run tokens and anchors | 0 verified; 1 chain / blob / schema failure or `run_token_reused`; 2 anchor disagreement (`anchor_disagrees`, `anchor_inconsistent`); 3 UNVERIFIED (ledger absent or empty, blob missing, anchor unreadable or malformed, `mechanism` without probe evidence); 64 usage |

## The run token (T051a, constitution 7.1)
`evrec token` prints a fresh token. The caller passes it to a runner wrapper as `--run-token TOKEN` (the argv the recorder stores therefore holds it, no new ev/1 field exists). The wrapper exports it to the test as `EVREC_RUN_TOKEN`. A runtime test that changes mutable state writes the token into the state it changes and reads it back in its assertion. `tools/evidence/verdict` accepts a state-delta GREEN only when the captured stdout of that entry carries the token of ITS OWN entry; `verify` reports `run_token_reused` when two entries share one token. Redaction: the token is a nonce, not a secret, so `--run-token <32 hex>` is exempt from the credential-flag redaction of argv and the same token (exactly the one of this run's argv) is kept readable in the stored stdout and stderr blobs; every other secret is redacted as before.
Limit: the token proves the post-state was captured in the entry's own stdout, not that the state change is correct.

## Anchors (T055, docs/06 section 7 and 8, constitution 11.4.268)
An anchor row: `{"schema":"ev-anchor/1","head":<entry_hash of entry COUNT>,"count":N,"at":<UTC>,"strength":"policy"|"mechanism"[,"probe":{...}]}` in `$EV/anchors.jsonl` (or `EV_ANCHOR`, or `anchors.jsonl` beside an `EV_LEDGER`/`--ledger` ledger). `verify` compares EVERY row: the chain must hold at least COUNT entries and the entry at COUNT must carry the anchored head. A chain longer than the newest anchor with an intact prefix is valid growth (a lagging anchor). Strength is `policy` unless a config probe (`probe_kind=config`) shows that the remote refuses a rewrite: the probe only READS the configuration of a LOCAL bare repository (`git config --show-origin --get-all`, the config scopes of the probing environment included) and `rewrite_rejected` is true only when `receive.denyNonFastForwards` AND `receive.denyDeletes` are both effectively true; the exact values read, with their origins, are recorded in the row's `probe.config`. An executable `pre-receive` or `update` hook is recorded under `probe.hooks` as evidence only and is never proof. NO push of any kind is made: constitution 11.4.113 forbids a forced, plus-ref or lease-guarded push and any history rewrite on every repository, scratch ones included, so the retired scratch-rewrite probe and its `--probe-scratch-ok` flag no longer exist (the flag is a usage error), and a network remote is never contacted. A `mechanism` row without `probe_kind=config` evidence is UNVERIFIED `strength_unproven`. Configuration is weaker evidence than an observed rejection; that is why the strength stays `policy` unless both settings are read as true. The real anchor location and any `mechanism` claim are BLOCKED-ON OD-76; until it is answered the anchors stay in `$EV/anchors.jsonl` and are `policy`.

## Environment
`EV`, `EV_LEDGER`, `EV_ANCHOR`, `EV_BLOBS`, `EV_TEST_SOURCES`, `EV_LEDGER_BOUND`, `EV_BLOB_BOUND`, `EV_FLAKE_LEDGER`, `EVREC_REDACT_VARS`, `EVREC_LOCK_TIMEOUT`, `EVREC_TURN_RUN_ID`, `EVREC_REPO_ROOT`, `CPA_HOST_ENTRY`, `EVREC_OPERAND_DIR_MAX`, `EVREC_OPERAND_DIR_BYTES_MAX`, `EVREC_RUN_TOKEN` (reconcile scope). Test hooks: `EVREC_FAULT`, `EVREC_PROC_ROOT`. The tools read no other variable; the tests unset all of them (`tools/evidence/tests/hermetic.sh`).

## Tests and evidence
`tools/evidence/tests/`: `test_evrec*.sh` (nine suites, rounds 1 to 7, plus the T051a cases), `test_anchors.sh` (T055), `test_evidence_record_schema.sh` (T048a), `run_mutations.py` and `run_mutations_wp05b.py` (paired mutations). Evidence under `$EV/wp05/`.

## Stated limits (11.4.6)
Python 3.11 or newer is assumed (possessive quantifiers); behaviour on older interpreters is UNCONFIRMED. The decision on continuum reuse (DR-E1) and its four open points are in `$EV/wp05/DR-E1.md`; until the owner settles them this reference recorder is the implementation and the adapter seam is documented there. An anchor in the same directory as the ledger is rewritable by whoever can rewrite the ledger (docs/06 section 7, last row).
