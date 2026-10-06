# Index tooling notes (T015, read-only)

Read at constitution pin `10b7a06` (`git -C submodules/constitution log -1 --format=%h` = 10b7a06), 2026-10-05. Every statement below was read from the file named; nothing was executed against the live indexes. Closes the docs/02 section 2.1 and 4.5 UNCONFIRMED items as stated per row.

## codegraph_safe.sh (607 lines; `docs/scripts/codegraph_safe.md` still says 561, its line citations are stale, as its own text admits)

Grammar (header lines 34-95, parser lines 245-271, read): `codegraph_safe.sh [options] <op>`; exactly ONE op word, options may come before OR after it (the loop is a single while over all args); a second positional word exits 2 (`only one op allowed`), no op exits 2, an unknown op exits 2.

- ops: writers `init | index | sync`; readers `status | verify`; other `preflight | unlock`.
- options: `--project DIR` (default git toplevel of $PWD, else $PWD; path with whitespace refused, exit 2), `--bulk` (writers only, exit 2 with a read op), `--pending-threshold N` (default `$CG_SAFE_PENDING_BULK_MIN`, else 150000), `--patches IDS` (default `$CG_SAFE_PATCHES`, else `fkidx1,resolve1`; lockfix1 and datafrag1 opt-in only, no tests), `--wait`, `--poll-s N` (10), `--watch-interval N` (60), `--stall-min M` (10), `--tripwire-min-gib N` ([5,500], 20), `--tripwire-interval N` (30), `--reap-dead-lock`, `--expected-count N --tolerance-pct P` (verify), `--status-doc FILE`, `--scope-baseline FILE`, `--scope-exceptions FILE`.
- exit codes: 0 ok, 1 indexer failed or verify FAILED, 2 usage, 3 live writer or flock held, 4 stale lock not reaped, 5 safe runner unavailable (stock never a fallback), 6 host preflight failed, 7 STALL, 8 disk tripwire.
- the sanctioned T022 form `--project "$PWD" sync --wait` parses (option, op, option). It is NOT `sync "$PWD"`: docs/02 section 4.5's illustrative `codegraph_safe.sh sync "$PWD"` is WRONG, the second word is refused with exit 2 (resolves the 4.5 UNCONFIRMED grammar item; docs/02 should be amended under T011a).
- `sync` is BULK when pending refs >= threshold, else INCREMENTAL (stock, foreground, under the flock).
- heap and preflight (codegraph_safe.sh lines 317-365, read): `heap_mb = MemAvailable/2` in MiB, floored at 8192, capped at `MemTotal*60/100` MiB (the floor is applied before the cap, the cap wins); exported as `NODE_OPTIONS=--max-old-space-size` (line 569). The preflight also contains `[ MemAvailable -ge 32 GiB ] || PREFLIGHT FAIL memory` (line 322). docs/02 section 2.3 records host RAM 30 GiB total, so on that host this preflight cannot pass (writer exit 6) whatever the load: UNCONFIRMED until `preflight` is run on the measurement host (T022 / T006a host identity); T022 on a 30 GiB host is therefore expected to be blocked by the script's own check, not by the plan.
- test-only env overrides exist (`CG_SAFE_TEST_*`); they can only make a resource look scarcer.

## scope_render.py (446 lines) and scope.example.yaml (112 lines)

- usage: `scope_render.py --scope <scope.yaml> --root <root> [--write] [--check] [--out-config F] [--out-gitignore-block F]`; no mode flag prints the rendered `codegraph.json` on stdout; `--write` writes `<root>/codegraph.json` and the delimited `# BEGIN/END helix-codegraph-scope` block of the root `.gitignore` (block replaced in place and moved to the end); `--check` exits 1 on drift; exit 3 fail-closed (unparsable scope, missing required key or baseline class, unclassifiable gitlink, unreadable `.gitmodules`, usage). Needs python3 + PyYAML (host has PyYAML 6.0.3) + git.
- scope.yaml keys: `schema_version: 1`, `runner{config_filename, dist_dir}`, `own_orgs{hosts[], orgs[]}` (template lists 9 orgs incl. red-elf, ATMOSphere1234321 ...; the repository's own list is `vasic-digital`, `HelixDevelopment` per T017, so the template list must be narrowed in `config/index/scope.yaml`), `submodule_class_overrides{path: own|third_party}`, `baseline_excludes{build_outputs, caches, secrets, qa_corpora}` (a class may not be removed; ORDER matters, last match wins), `project_excludes`, `pathological_excludes`, `include_patterns`, `reinclude_negations` (written without leading `!`), `forbidden_classes[{name, regex, positive, negative}]` (control-needled), `enumeration_controls{present[], fabricated[]}`, `accepted_count` (0 = guard FAILS), `tolerance_pct` (2.0).
- own-org classification is by URL host AND org; nested submodules are classified by their own URL; third-party subtrees are excluded whole.

## gen_lumenignore.py (93 lines)

- `gen_lumenignore.py --repo R --scope scope.json [--out F]`; scope.json = `{"allow": [rel dirs], "deny": [patterns inside allowed roots], "root_files": bool (default true)}`. Empty `allow` -> SystemExit "refusing to emit a deny-everything file" (non-zero). Catch-all deny (`*`, `**`, `**/*`, `/*`, `/**`) refused. Output deterministic, sorted, header carries `# scope-sha256: <sha256 of the scope file bytes>` and `# allow: ...`; no timestamp. It reads real children of every ancestor of every allowed root, so output depends on the repo working tree (T018 golden must build a fixture tree).

## lumen_verify.sh (109 lines)

- `lumen_verify.sh --golden <file.json> --project <root> [--k 5] [--n 20] [--lumen <bin>] [--out <dir>] [--scope-key in_tierA] [--min-recall 0.85] [--baseline known_misses.json] [--no-update-baseline]`. Golden = JSON array of `{id, type: conceptual|structural|unsupported, q, gold: [paths; trailing "/" = any file under dir], in_tierA?: false => SKIPPED (not for type unsupported)}`. Recall = PASS/(PASS+FAIL) over in-scope indexable queries. Exit 0 all PASS, 1 any FAIL, 2 usage or errored/empty search. Output `<out>/results.tsv` + `summary.txt`, sorted, no timestamps. Side effect: `lumen search` runs EnsureFresh (writes the Lumen index chosen by XDG_DATA_HOME). Default lumen = newest `~/.claude*/plugins/cache/claude-plugins-official/lumen/*/bin/lumen-linux-amd64`.

## MCP wrapper candidates (the 4.5/2.1 question: does each pass `--no-watch`?)

- `codegraph_mcp.sh` (140 lines): NO. It accepts no `--no-watch` (or `--path`) flag; any argument other than `--project DIR`, `--project=DIR`, `--dry-run`, `serve`, `--mcp`, `-h` exits 2. It instead pins the environment before `exec "$RUNNER" serve --mcp`: `CODEGRAPH_NO_DAEMON=1 CODEGRAPH_NO_WATCH=1 CODEGRAPH_NO_UPDATE_CHECK=1 DO_NOT_TRACK=1 CODEGRAPH_TELEMETRY=0 CODEGRAPH_NO_PROMPT_HOOK=1 CODEGRAPH_QUERY_POOL_SIZE=0 CODEGRAPH_MCP_TOOLS=explore,node,search,callers,callees,impact,files,status`, and serves only through the receipt-verified mcpro1 runner. Exit: 0 ok, 2 usage, 3 no index, 4 live writer (decided from /proc fd modes and lock-holder liveness, never lock age), 5 runner missing or not a valid mcpro1 runner. `--dry-run` prints a JSON verdict line, the ENV lines and the EXEC plan without exec. So watching is disabled by environment (`CODEGRAPH_NO_WATCH=1`), not by a flag. UNCONFIRMED: that the installed runner honours `CODEGRAPH_NO_WATCH` at runtime (read only the wrapper and docs; the mcpro1 patch `runner_patches/mcpro1.py` was not read in this task, and a live DB-mtime check is the 2026-09-25 extension's proof method).
- `codegraph_mcp_preflight.sh` (173 lines): not a launcher; it is a checker (P1-P6) of `.mcp.json` plus a fixture run through the wrapper. It passes no watch flag and starts no server against the real project (only `codegraph_mcp.sh --dry-run`). Usage `[--project DIR] [--mcp-json FILE] [--settings FILE]...`; exit 0 no FAIL, 1 any FAIL, 2 usage; output `PREFLIGHT <id> PASS|FAIL|WARN` lines and `SUMMARY fail=N warn=M`. It FAILs P1 when `.mcp.json` is absent (true today, so T026 must write it before it can pass).
- `codegraph_mcp_serve.sh` (named by the 2026-09-25 extension) does not exist at this pin (consistent with docs/02 2.1).

## Evidence: read-only `preflight` run (2026-10-05, host, no DB touched)

Command: `bash submodules/constitution/scripts/codegraph/codegraph_safe.sh --project "$PWD" preflight` (the guide lists `preflight` as "Host check only (no DB touched)"). Output tail, verbatim:

```
free_bytes=130756874240
db_bytes=525885440
min_free_bytes=64424509440
mem_avail_kb=14397888
min_mem_avail_kb=33554432
PREFLIGHT FAIL memory: MemAvailable 14397888 kB < 32 GiB
thread_limit=123699
threads_used=3206
min_thread_headroom=1024
heap_mb=8192
heap_cap_mb=18584
```

So on this host (30 GiB total, `free -g` available 13 GiB) the writer preflight FAILs the memory check (exit 6): T022's `sync --wait` cannot start here regardless of load. No flag lowers the 32 GiB bar (only the test-only override that makes memory look scarcer). Needs a host with MemAvailable >= 32 GiB (the HC-0 ODG-07 build/measurement host) or a constitution change; recorded as the blocker for T022.
