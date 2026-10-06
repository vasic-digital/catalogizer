# Index scope notes (T019, WP-02), 2026-10-06

Constitution pin read: `a71b176` (the T015 notes were written at `10b7a06`; `git diff --stat 10b7a06 a71b176 -- scripts/codegraph scripts/lumen` is empty, so the T015 grammar notes still hold).

## Authored DATA and rendered outputs (uncommitted; T029 commits after the T020 and WP-02 reviews)

| File | Origin |
|---|---|
| `config/index/scope.yaml` | authored from `scope.example.yaml`; own orgs `vasic-digital`, `HelixDevelopment` (scripts/audit/own_orgs.txt, T017); project rubbish: `/.codegraph/`, `/.audit/`, the audit evidence tree, `/docs/reports/qa-sessions/`, `/docs/qa/`; pathological: one 14 MB logcat; six re-include negations found with the guard's `--explain` census (Go package `submodules/doc_processor/pkg/coverage`, Android `res/xml`, `res/values`, `res/layout`: first-party source dropped by built-in directory skips); `accepted_count: 5269`, `tolerance_pct: 2.0` |
| `codegraph.json`, `.gitignore` scope block | rendered ONLY by `scope_render.py --write`; `--check` prints `drift: []`; the block is the last content of `.gitignore` and `scripts/repo/tests/test_planned_paths_tracked.sh` re-run: `TOTAL planned=107 control=187 failures=0` (evidence/wp02/t019-t004-test-rerun.txt) |
| `config/index/lumen_scope.json` | `scripts/audit/scope_to_lumen_json.py` over scope.yaml + `lumen_allow_roots.yaml` (see OWED-T019-1); `--check` exit 0 |
| `.lumenignore` | `gen_lumenignore.py --repo . --scope config/index/lumen_scope.json`; regenerated twice, byte-identical |

Accepted count 5269 = the engine's own enumeration (`codegraph_scope_guard.py --dry-run-dir ... --print-count`, installed runner discovery, read only): 50 own-org submodule roots non-empty, 44 checked-out third-party roots with zero files, scope guard PASS, violations []. UNCONFIRMED: the count moves with other agents' untracked files (the engine enumerates the git view); the 2 percent tolerance covers small drift only.

## OWED-T019-1 (tool conflict, not fixed here)

`scripts/audit/scope_to_lumen_json.py` documents the optional key `lumen_allow_roots` in scope.yaml, but `scope_render.py` fails closed (exit 3, "unknown top-level scope key(s)") on that key. Without allow roots the derived allow list holds only the 50 own-org submodule roots, so the main repository's own directories (catalog-api, catalog-web, ...) would be denied. Workaround used: the roots are DATA in `config/index/lumen_allow_roots.yaml`, merged into a scratch copy for the derivation:

```
python3 -I -c "import yaml;s=yaml.safe_load(open('config/index/scope.yaml'));s.update(yaml.safe_load(open('config/index/lumen_allow_roots.yaml')));yaml.safe_dump(s,open('/tmp/scope_merged.yaml','w'),sort_keys=True)"
python3 -I scripts/audit/scope_to_lumen_json.py --scope /tmp/scope_merged.yaml --submodules-tsv specs/001-full-project-audit-remediation/audit/submodules.tsv --out config/index/lumen_scope.json
```

Fix owed (T018 owner): let the tool read a second file or a different key the renderer accepts. The dropped negation `!*secret*/` (secretmgr-style directories lose semantic coverage) is recorded by the tool under `dropped_negations`.

## T022 BLOCKED-ON-MEMORY (not run)

`codegraph_safe.sh --project "$PWD" preflight` exit 6: `PREFLIGHT FAIL memory: MemAvailable 16960596 kB < 32 GiB` (evidence/wp02/t022-blocked-preflight.txt). MemTotal is 31717548 kB, below 32 GiB, so the check cannot pass on this host at any load; swap is not counted in MemAvailable (SwapFree was 252 kB of 8 GiB). The owner decision "add swap and retry" therefore cannot satisfy the check (needs root as well; none attempted). Options for the owner: run T022 on the 32 GiB+ build host (HC-0 ODG-07), or amend the constitution threshold. No writer, no sync, no Lumen refresh, no DEF-T022-1/DEF-T022-2 deferral row was written (the sync did not start; the shared `deferrals.jsonl` is untouched): owed rows when T022 runs: DEF-T022-1 (registration owed to T089) and DEF-T022-2 (bare-host patched-runner build awaiting T121b). Also UNCONFIRMED: the writer's patch tool defaults to the npm-global codegraph 1.6.1, on which the mcpro1 anchor is absent (F-INDEX-002); the fkidx1/resolve1 anchors on 1.6.1 were not tested.
