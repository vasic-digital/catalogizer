# verify_live_evidence.sh - LIVE-EVIDENCE verifier

Script: `scripts/test-infra/verify_live_evidence.sh`. Rule: constitution 11.4.289(D). Schema: `docs/qa/live-evidence.schema.json`. Double register: `docs/qa/live-evidence.doubles.txt`.

## What it does
Reads LIVE-EVIDENCE records (JSON) and decides, from the machine fields alone, whether a record that claims `class: LIVE` really proves a real target. A LIVE PASS/FAIL record is **reclassified UNIT-CLASS (exit 1)** when:

| reason | meaning |
|---|---|
| `registered_double` | answering executable sha256 / basename / command line / image matches a rule in the double register |
| `identity_mismatch` | answering exe sha256 or image digest differs from `expected` (the real artifact the claim names) |
| `answering_identity_missing` | nothing resolved what answered (no pid/exe/sha256, or no image digest) |
| `expected_identity_missing` | the claim does not name the real artifact |
| `artifact_fingerprint_missing` | no fingerprint read from the target |
| `endpoint_identity_missing` | no host/port |

Loopback, test-started services, ephemeral ports and scratch directories do **not** disqualify a record: a real service the test boots stays LIVE when its executable hash matches the artifact the claim names. A `SKIP` record needs `skip_reason` from the closed set (hardware_not_present, credentials_absent, topology_unsupported, operator_attended, structurally_impossible, feature_disabled_by_config, artifact_not_yet_built). PASS/FAIL records need `runs.identical: true` and a signature.

## Usage
```
scripts/test-infra/verify_live_evidence.sh [--require-live] [--doubles FILE] <record.json|dir>...
```
Output per record: `<path> class=LIVE|UNIT|SKIP|INVALID reasons=...`. Exit: `0` all fine; `1` a LIVE claim reclassified UNIT (or `--require-live` met a UNIT record); `2` invalid record, usage error, unreadable register or no records (fail closed; an empty set is never a pass). `LIVE_EVIDENCE_DOUBLES` overrides the register path. Requires `jq`.

## Writing records
* Go: `catalog-api/internal/liveevidence` (`NewRecord`, `SetArtifact`, `ResolveAnswering("127.0.0.1:PORT", expectedExeSHA256)`, `Observe`, `Finish`, `Skip`, `Write`).
* Node / Playwright: `tests/live-evidence/index.mjs` (`Recorder`, same calls).
* Shell: write the JSON per the schema (e.g. with `jq -n`); resolve the listener pid from `ss -ltnp` and hash `/proc/<pid>/exe`.

`ResolveAnswering` reads `/proc/net/tcp{,6}` and `/proc/<pid>/fd` (same-user processes only). If it cannot resolve, `answering` stays null and the verifier reclassifies the record UNIT: the conservative default.

## Tests
`tests/live-evidence/test_verify_live_evidence.sh` (golden-good, golden-bad, golden-FALSE carrier, control needles), `tests/live-evidence/test_verify_live_evidence_mutation.sh` (identity check removed / register ignored must fail the suite), `node tests/live-evidence/test_recorder.mjs`, `go test ./internal/liveevidence` (real python3 http.server process on loopback).

## Honest limits
The verifier trusts the fields it is given: it does not re-read `/proc/<pid>` after the fact, so a producer able to forge fields is bounded only by harness-written verdicts and tamper evidence (11.4.115(F), 11.4.268). The double register is a deny-list; an unregistered fixture server with an unremarkable name and a matching `expected` hash would pass. Container resolution (image digest from the runtime) is caller-supplied, not auto-resolved.
