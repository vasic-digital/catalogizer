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
| `endpoint_identity_incomplete` | process record without `cmdline` (the `cmd:` register rules could not be evaluated, so absent is not a pass) |
| `fingerprint_mismatch` | `artifact.fingerprint` is a sha256 but differs from the hash of the artifact the claim names (`expected.exe_sha256`, `expected.image_digest`, or `expected.script_sha256` for interpreter-hosted servers) |
| `interpreter_script_missing` / `interpreter_script_unbound` | the exe is an interpreter (python, node, deno, bun, java, ruby, perl, php, lua, shell): the exe hash only proves the interpreter, so the record needs `script_path` + `script_sha256`, and `expected.script_sha256` must equal it |
| `runs_below_3` | a PASS with fewer than 3 identical runs (11.4.50) is not LIVE |
| `listener_unresolved_or_ambiguous` | the recorder set `resolve_error` (nothing found, or several pids hold the same LISTEN socket) |

Loopback, test-started services, ephemeral ports and scratch directories do **not** disqualify a record: a real service the test boots stays LIVE when its executable hash matches the artifact the claim names. A `SKIP` record needs `skip_reason` from the closed set (hardware_not_present, credentials_absent, topology_unsupported, operator_attended, structurally_impossible, feature_disabled_by_config, artifact_not_yet_built). PASS/FAIL records need `runs.identical: true` and a signature; every record needs a `runs` object, a RFC3339 `timestamp`, an integer port 1..65535, and the file must hold exactly ONE JSON value (a second value could hide a UNIT verdict: INVALID). A `socket` answering needs pid, exe path and exe hash.

## Usage
```
scripts/test-infra/verify_live_evidence.sh [--require-live] [--doubles FILE] <record.json|dir>...
```
Output per record: `<path> class=LIVE|UNIT|SKIP|INVALID reasons=...`. Exit: `0` all fine; `1` a LIVE claim reclassified UNIT (or `--require-live` met a UNIT record); `--require-live` also exits 1 on a SKIP record (a SKIP is never live proof). `2` invalid record, usage error, unreadable register or no records (fail closed; an empty set is never a pass). `LIVE_EVIDENCE_DOUBLES` overrides the register path. Requires `jq`.

## Writing records
* Go: `catalog-api/internal/liveevidence` (`NewRecord`, `SetArtifact`, `ResolveAnswering("127.0.0.1:PORT", expectedExeSHA256)`, `Observe`, `Finish`, `Skip`, `Write`).
* Node / Playwright: `tests/live-evidence/index.mjs` (`Recorder`, same calls).
* Shell: write the JSON per the schema (e.g. with `jq -n`); resolve the listener pid from `ss -ltnp` and hash `/proc/<pid>/exe`.

`ResolveAnswering` reads `/proc/net/tcp{,6}` and `/proc/<pid>/fd` (same-user processes only). Only LISTEN sockets whose bound address covers the claimed host count (a wildcard bind covers every host). If it cannot resolve, or if SEVERAL pids hold the matching socket (forked workers), `answering` stays null, `resolve_error` is set and the verifier reclassifies the record UNIT: the first pid is never picked. For interpreter processes the recorder also records `script_path`/`script_sha256` (first non-option argument, `-jar` for java; `-c`/`-e`/`-m` code stays unbound), so call `SetExpectedScript` / `setExpectedScript`.

## Tests
`tests/live-evidence/test_verify_live_evidence.sh` (golden-good, golden-bad, golden-FALSE carrier, control needles), `tests/live-evidence/test_verify_live_evidence_mutation.sh` (identity check removed / register ignored must fail the suite), `node tests/live-evidence/test_recorder.mjs`, `go test ./internal/liveevidence` (real python3 http.server process on loopback).

## Honest limits
* Interpreter-hosted servers: the exe hash identifies python/node/java, not the program. The record must bind the script (`script_sha256` == `expected.script_sha256`), and when the fingerprint is a sha256 it must equal the bound artifact's hash. The script is found heuristically from the command line; a server run from inline code or a module (`-c`, `-e`, `-m`) is reported UNIT (unbound). A script that itself `exec`s or imports a fixture is not seen: only the file named on the command line is hashed.
* Fingerprint binding applies only when the fingerprint has the form `sha256:<64 hex>` (or bare 64 hex); a version-string fingerprint is recorded but cannot be compared.
The verifier trusts the fields it is given: it does not re-read `/proc/<pid>` after the fact, so a producer able to forge fields is bounded only by harness-written verdicts and tamper evidence (11.4.115(F), 11.4.268). The double register is a deny-list; an unregistered fixture server with an unremarkable name and a matching `expected` hash would pass. Container resolution (image digest from the runtime) is caller-supplied, not auto-resolved.
