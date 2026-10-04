# Quickstart: Validate the Planning Baseline

| Field | Value |
|---|---|
| Feature | `specs/001-full-project-audit-remediation` |
| Created | 2026-10-03 |
| Revision | 9 |
| Status | draft (revision 9: the exit criterion of step 3, the G7 gate and the closing paragraph state the ODG-41 branch of the final verification (tasks.md rev 12 T582, T583, T595b; docs/21 WP-74R and IC-58, the round-11 content review m-6): a strict run that fails only on rows another actor's commits explain is reported `blocked: ODG-41` until the owner answers, and under option (a) passes when every failing row is an explained exception naming its foreign-only commits; the main repository's `foreign_outside_candidate` commits are the same kind; only any other failure reopens WP-73; no command, output or expectation of the executed run changed. Revision 8: `repo-verification-report.schema.json` now requires the mode boolean `no_remote` (contracts revision 9, tasks.md rev 11 T031, T033), which the POC verifier does not write (its jq assembly, `poc/repo_verify/verify_repo.sh` line 259, writes `fetch` and `strict` only), so steps 3 and 6 state that the POC's documents are no longer valid instances (measured on the stored `run1.json` on 2026-10-04, where the missing field is the only error; for a fresh run read from the POC's source, not executed) and that a re-run of the step 6 script now exits 1 for that reason; the other stored POC results stay VALID; no command of the executed run changed. Revision 7: step 6 notes that `review-verdict.schema.json` revision 8 (contracts revision 8) requires `blocking_findings`, and the syntax and meta-schema checks re-run on all 7 files, with the four stored POC results VALID against their schemas, while this revision was written; this header gains the §11.4.44 `Status` row it lacked (round-9 consistency review of commit `b9412d06`); no command, output or expectation of the executed run changed. Revision 6: step 6 notes the seventh contract schema, `review-verdict.schema.json`, added after the executed run (syntax and meta-schema checks re-run on all 7 files while revision 6 was written; its fixture results are in `contracts/README.md`); the G3 gate names the review verdict format; the closing paragraph names docs/21 ODG-41, which also has no `research.md` counterpart. Revision 5: the strict-run expectation of step 3 is dated: the two failing classes belong to the 11:44:31Z run, and a read-only re-measurement at HEAD `1a69eed5` (19:50:33Z) finds the main repository clean and the constitution's remote tip absent locally, so the expected result is 14 without `--fetch` and the strict-`behind` code with it; the fixture-validation count follows `finding/1` revision 4 (43 samples). Revision 4: the exit-code precedence is cited as tasks.md T032, the id in tasks.md rev 6; ODG-39 blocks WP-20 and WP-74 as in docs/21 revision 8. Revision 3: production verifier modes per data-model §9; expected strict exit stated with its open precedence; fixture count; G2 index projection; G3 phase mapping; golden file names) |
| Last modified | 2026-10-04 |
| Executed | 2026-10-03, from 11:44:31Z, main repository HEAD `e4852ce7e1a136818b7b63524e94b5f8ee1b68bf` |
| Scope | read-only commands that prove what the plan starts from; no build, no install, no write to any repository |

Every command marked EXECUTED was run in this session and its output is quoted. Commands marked NOT EXECUTED are the planned checks whose tooling does not exist yet. Outputs go to a scratch directory, never into the repository (set `OUT` to any directory outside the tree).

## 1. Prerequisites

| Need | Check | Observed |
|---|---|---|
| bash, git, jq, ssh with access to the project remotes | `command -v git jq ssh` | present |
| Python 3 with PyYAML and `jsonschema` | `python3 -c "import yaml, jsonschema"` | Python 3.14.4, jsonschema 4.19.2 |
| CodeGraph CLI | `codegraph --version` | `1.6.0` |
| Lumen (MCP plugin) with Ollama | `health_check` tool | `Backend: ollama ... Model: ordis/jina-embeddings-v2-base-code Status: OK` |
| Network for `git ls-remote` (step 3) | SSH keys loaded, `BatchMode` | 8 remotes of the main repository answered |

```bash
cd /home/milosvasic/Projects/catalogizer
P=specs/001-full-project-audit-remediation/poc
S=specs/001-full-project-audit-remediation   # feature directory; $AUD = $S/audit, $EV = $S/evidence
OUT=/tmp/claude-1000/qs        # any scratch directory outside the repository
mkdir -p "$OUT"
```

Do not run a Lumen `semantic_search` before step 2 is recorded: a search re-indexes (`EnsureFresh`) and writes the Lumen database (docs/02 §2.2).

## 2. Index health (FR-005, research R-06) - EXECUTED

```bash
codegraph status --json
ls .mcp.json .codegraph/config.json .lumenignore
```

Real output (abridged):

```json
{"initialized":true,"version":"1.6.0","lastIndexed":"2026-10-02T19:24:35.111Z","fileCount":7150,
 "nodeCount":128472,"edgeCount":414476,"pendingChanges":{"added":2,"modified":0,"removed":0},
 "worktreeMismatch":null,"index":{"state":"complete","pendingRefs":0,"reindexRecommended":false}}
```

```text
ls: cannot access '.mcp.json': No such file or directory
ls: cannot access '.codegraph/config.json': No such file or directory
ls: cannot access '.lumenignore': No such file or directory
```

Lumen (MCP tools `index_status` and `health_check`, EXECUTED):

```text
Files: 10397 | Indexed: 10397 | Chunks: 167256 | Model: ordis/jina-embeddings-v2-base-code
Last indexed: 2026-10-03T10:19:58Z | Stale: yes
Backend: ollama | Status: OK | Message: service and configured model are ready
```

Verdicts against the proof table (docs/02 §4.1):

| Proof | Verdict today | Reason |
|---|---|---|
| P1 CodeGraph state | FAIL | `pendingChanges.added = 2` |
| P2, P3, P4 | NOT EXECUTED | need the derived tracked set, scope DATA (`config/index/scope.yaml`) and the golden questions `$AUD/golden.json` (the 60-query Lumen set is `$AUD/lumen_golden_60.json`, docs/02 §4.4) |
| P5 CodeGraph freshness | FAIL | `lastIndexed` 2026-10-02T19:24Z is older than HEAD commit time 2026-10-03T10:56:02Z |
| P6 embedder dimensionality | NOT EXECUTED | needs the model card dimension and a real embed call |
| P7 Lumen complete and fresh | FAIL | `Stale: yes` |
| P8 scope parity | FAIL | 10,397 Lumen files vs 7,150 CodeGraph files, no shared scope DATA |

Exit criterion for the index gate: P1 to P8 all PASS in `specs/001-full-project-audit-remediation/audit/index-health.json` (`$AUD/index-health.json`, `index-health/1`), produced after the sanctioned writer refresh (`codegraph_safe.sh`, NOT EXECUTED here).

## 3. Repository state (FR-019, FR-020, SC-010) - EXECUTED

```bash
$P/repo_verify/verify_repo.sh --self-test
/usr/bin/time -p $P/repo_verify/verify_repo.sh --root . --jobs 8 --timeout 25 --quiet --json-out "$OUT/repo_verify.json"
echo "exit=$?"
```

Real output:

```text
SELF-TEST: pass=24 fail=0
exit=1        (real 45.62 s)
{"repos":98,"owned":51,"dirty":2,"dirty_excepted":1,"ahead":0,"diverged":0,"pin_drift":0,
 "unproven":0,"classes":{"LOCAL-BEHIND":8,"SAME":191},"failing":1}
failing:  "."  problems ["dirty"]  (0 tracked, 3 untracked: this feature's new planning files)
excepted: submodules/helix_qa/tools/opensource/docling (1 tracked, CRLF quirk, exceptions.tsv)
LOCAL-BEHIND x8: submodules/constitution (behind its upstream on all 8 remotes)
```

Expected machine-readable output: a document in the `repo-verification-report/1` shape, which was valid against `contracts/repo-verification-report.schema.json` when this step ran (step 6). Revision 8: the schema now requires the mode boolean `no_remote` (contracts revision 9, tasks.md T031, T033), which this POC does not write, so its document is no longer a valid instance until the promoted verifier below writes the field. Exit 1 is the honest result of the POC while planning files are uncommitted.

The production verifier is `scripts/repo/verify_repos.sh`, promoted from this POC by tasks.md WP-03 with the same `repo-verification-report/1` JSON and the docs/16 exit codes (0 clean, 11 unpushed, 12 diverged, 13 dirty, 14 unverified remote, 15 pointer drift, 20 blind or internal; data-model.md §9). Codes 11 to 14 apply in both modes; `--strict` adds `behind` rows and pointer drift to the failing set, so 15 and the strict `behind` code apply only with it (docs/21 IC-37). It accepts `--json` and `--json-out`. NOT EXECUTED (the script does not exist yet):

```bash
scripts/repo/verify_repos.sh --strict --json "$OUT/repo_verify.json"; echo "exit=$?"
```

On the state of the EXECUTED run above (2026-10-03 from 11:44:31Z, HEAD `e4852ce7`, planning files uncommitted) the strict run would have had two failing classes: the dirty main repository (13) and the `submodules/constitution` row that is `LOCAL-BEHIND` on all 8 remotes (`behind` under `--strict`); the expected result was then 13 or the strict-`behind` code, whichever the WP-03 exit-code matrix puts first (the precedence is decided by tasks.md T032; a failing class always precedes 14), either being the counterpart of the POC's exit 1. Re-measured (EXECUTED 2026-10-03T19:50:33Z, read-only, HEAD `1a69eed5`): `git status --porcelain --ignore-submodules=all` prints nothing, so the main repository is clean; the constitution is still at `10b7a06` while `git ls-remote <remote> refs/heads/main` gives `be06384` on each of its 8 remotes, and `be06384` is not in the local object store (`git cat-file -t` fails). So the command above, which has no `--fetch`, finds the constitution's remotes `UNKNOWN-DIFFERENT` (unproven) and the expected result is 14. With `--fetch` (objects only) the remotes are decided: the local tracking refs at `e44f22f` contain `10b7a06`, and the independent review of commit `1a69eed5` measured `e44f22f` as an ancestor of `be06384` from a commit-only clone (not re-measured here), so the row is `LOCAL-BEHIND` and the expected strict result is the strict-`behind` code. Every such statement is dated, because each commit and each upstream push changes it. Exit criterion for US7 / SC-010: `scripts/repo/verify_repos.sh --strict` exit 0, `summary.failing = 0`, `summary.unproven = 0`, every exception carrying a reason; or, once the owner has chosen option (a) of docs/21 ODG-41, a non-zero exit whose every failing row is an explained exception naming the commits that other actors pushed after that repository's reference tips, with A-5, A-6 and, for the root row, A-7 passing without those rows (tasks.md T582, T583). Until the owner answers ODG-41 such a row is reported `blocked: ODG-41`, never a pass and never a loop that reopens WP-73; the commits of another actor that a main-repository merge brought in after the release candidate was built (`foreign_outside_candidate`, tasks.md T308a) are handled the same way; the rows left unsettled while the final constitution update waits on ODG-12 fail by name; any other failure reopens WP-73 (tasks.md T595b; revision 9).

## 4. Documentation reachability (FR-013, SC-006) - EXECUTED

```bash
python3 $P/doc_links/crawl_links.py --self-test
python3 $P/doc_links/crawl_links.py --root . > "$OUT/doc_links.json"
python3 $P/doc_links/crawl_links.py --root . --site-root Website > "$OUT/doc_links_site.json"
```

Real output:

```text
SELF-TEST: pass=31 fail=0
default scope : in_scope 2557, reachable 42, orphans 2515, docs_in_scope 2225, docs_reachable 36,
                docs_orphans 2189, broken_links 123, broken_anchors 83, max_depth 3
--site-root   : in_scope 2557, reachable 42, orphans 2515, broken_links 87, broken_anchors 84
```

Compared with the stored POC run (`poc/doc_links/results/run1.json`: 2,551 in scope, 82 broken anchors), the 6 extra files are all new files under `specs/001-full-project-audit-remediation/` (docs 19 to 21 and the three POC READMEs), and the extra anchor is `#13-consolidated-recommendations` in docs/20. Exit criterion for US4: class A and B orphans 0, `broken_links = 0`, `broken_anchors = 0` with the `DOC_SCOPE.yaml` filter (filter NOT EXECUTED, file does not exist yet).

Export sync (NOT EXECUTED: `export_docs.sh`, `export_sync_check` with fingerprints and `docs/EXPORT_MANIFEST.json` do not exist yet). Exit criterion: `stale = 0`, `missing = 0`, independently recomputed `sha256(md)` equals each twin's embedded value.

## 5. API contract drift (FR-015, FR-016) - EXECUTED

```bash
python3 $P/route_drift/route_drift.py --self-test
python3 $P/route_drift/route_drift.py --root . --spec docs/api/openapi.yaml > "$OUT/route_drift.json"
```

Real output (`counts`):

```text
SELF-TEST: pass=26 fail=0
server routes 247 (unique 247), unwired mux routes 62, spec operations 181,
undocumented 68, stale spec 2, client calls api_client 59 / web 166 / android 42 / androidtv 37,
without route api_client 31 / web 69 / android 22 / androidtv 1, double prefix 30, unresolved 13
```

Identical to the stored run of 11:37Z. All items are leads; each becomes a finding only after a runtime router dump or request. Exit criterion: the router-table drift test over `gin.Engine.Routes()` (NOT EXECUTED, needs the router constructor extracted from `main.go`) reports zero undocumented and zero stale entries, and both-sided contract tests pass in the can-i-deploy gate.

## 6. Contract schemas against real outputs - EXECUTED

```bash
S=specs/001-full-project-audit-remediation
for f in $S/contracts/*.json; do python3 -m json.tool "$f" >/dev/null && echo "json OK $f"; done
python3 - "$OUT" <<'EOF'
import json, sys
from jsonschema import Draft202012Validator as V
S = 'specs/001-full-project-audit-remediation/'; O = sys.argv[1]
pairs = {
 'repo-verification-report': [S+'poc/repo_verify/results/run1.json', O+'/repo_verify.json'],
 'link-crawl-report': [S+'poc/doc_links/results/run1.json', S+'poc/doc_links/results/run2.json',
                       O+'/doc_links.json', O+'/doc_links_site.json'],
 'route-drift-report': [S+'poc/route_drift/results/run1.json', O+'/route_drift.json']}
bad = 0
for name, files in pairs.items():
    schema = json.load(open(S+'contracts/'+name+'.schema.json')); V.check_schema(schema); v = V(schema)
    for f in files:
        n = len(list(v.iter_errors(json.load(open(f))))); bad += n
        print('VALID  ' if n == 0 else 'INVALID', name, f)
sys.exit(1 if bad else 0)
EOF
```

Real output:

```text
json OK (all 6 schema files)
VALID   repo-verification-report poc/repo_verify/results/run1.json
VALID   repo-verification-report $OUT/repo_verify.json
VALID   link-crawl-report poc/doc_links/results/run1.json
VALID   link-crawl-report poc/doc_links/results/run2.json
VALID   link-crawl-report $OUT/doc_links.json
VALID   link-crawl-report $OUT/doc_links_site.json
VALID   route-drift-report poc/route_drift/results/run1.json
VALID   route-drift-report $OUT/route_drift.json
```

Revision 6: `contracts/` now holds 7 schema files; the seventh, `review-verdict.schema.json` (`review-verdict/1`, the format of every review verdict file, docs/21 IC-46), was added after the executed run above. `python3 -m json.tool` and `Draft202012Validator.check_schema` pass on all 7 files (re-run on 2026-10-03 while revision 6 was written); `review-verdict/1` has no real instance yet and its fixture validation is in `contracts/README.md`. Revision 7: `review-verdict.schema.json` now requires `blocking_findings` (0 on a GO, at least 1 on a NO-GO; contracts revision 8); `python3 -m json.tool` and `Draft202012Validator.check_schema` were re-run on all 7 files on 2026-10-03 while revision 7 was written and pass, and `poc/repo_verify/results/run1.json`, `poc/doc_links/results/run1.json`, `run2.json` and `poc/route_drift/results/run1.json` validate against their schemas (all VALID). Revision 8 (2026-10-04): `repo-verification-report.schema.json` requires `no_remote` from contracts revision 9 (tasks.md T031, T033); `python3 -m json.tool` and `Draft202012Validator.check_schema` pass on all 7 files; `poc/doc_links/results/run1.json`, `run2.json` and `poc/route_drift/results/run1.json` stay VALID, while `poc/repo_verify/results/run1.json` is now INVALID with exactly one error, `'no_remote' is a required property`, because the POC predates the field; a re-run of the script above therefore exits 1: for `run1.json` as measured, and for a fresh POC output `$OUT/repo_verify.json` as the POC's source shows (its jq assembly, line 259, writes no `no_remote`; that leg was not executed in this session), which is the expected effect of the contract change, not a defect of the POC run; the same document with `no_remote` added is VALID (contracts/README.md, its row).

Negative controls (EXECUTED, mutated copies of the fresh outputs): unknown remote class, missing `summary.failing`, unknown broken-link reason and unknown route source were each REJECTED. The fixture validation of `ev/1`, `finding/1` and `bank-case/3` is summarised in `contracts/README.md`: the current fixture script gives 43 of 43 expected outcomes (28 `ev/1`, 15 `finding/1`, after the `finding/1` revision 4 fix of the `artifact` path; re-run on 2026-10-03, 0 mismatches, and the same fixtures against the previous schema give the 5 expected mismatches), and the `bank-case/3` checks are the docs/12 §6.5 worked example (2 cases VALID), the real placeholder `api-auth-login` and a blind centre tap (both REJECTED).

## 7. HelixQA bank baseline (US3, research R-23) - EXECUTED

```bash
python3 - <<'EOF'
import json, yaml, glob
from jsonschema import Draft202012Validator as V
v = V(json.load(open('specs/001-full-project-audit-remediation/contracts/bank-case.schema.json')))
tot = ok = 0
for f in sorted(glob.glob('challenges/helixqa-banks/*.yaml')):
    b = yaml.safe_load(open(f)); lst = b.get('test_cases') if isinstance(b, dict) else b
    lst = lst if lst is not None else next((x for x in b.values() if isinstance(x, list)), [])
    tot += len(lst); ok += sum(1 for c in lst if not list(v.iter_errors(c)))
print(f"cases {tot}, valid against bank-case/3: {ok}")
EOF
```

Real output: `cases 1269, valid against bank-case/3: 0` (15 bank files). Exit criterion: every deterministic-lane case valid, `TODO: Convert to executable` count 0 (1,178 per docs/03 F-6), three identical runs and a caught reviewer mutation per counted case.

## 8. Register baseline (US1) - partly EXECUTED

```bash
git ls-files 'docs/workable_items.db'            # EXECUTED in docs/03 F-1: no register exists
ls docs/issues | wc -l                           # 1,778 ticket files (docs/03 F-2)
```

NOT EXECUTED (the register does not exist yet): applying `register_ext.sql`, the import stages, and the SC-001 queries of docs/04 §13.1, for example `SELECT count(*) FROM v_unmapped_entries;` (expected 0) and `SELECT * FROM v_custody_violations;` (expected empty).

## 9. Gate exit criteria by user story

The gates below follow the spec's user stories in priority order. They are not the docs/21 phases P0 to P7 (which follow dependency order); the "docs/21 phase" column maps each gate to the phase and work packages that produce it. Each criterion is a machine output, never a statement.

| Gate | Story | docs/21 phase (work packages) | Exit criterion (all must hold) | Producing command or artifact | Status today |
|---|---|---|---|---|---|
| G0 Index readiness | FR-005 | P0 (WP-02), repeated in P3 (WP-39) | P1 to P8 PASS in `specs/001-full-project-audit-remediation/audit/index-health.json` | step 2 plus writer refresh and goldens | FAIL (P1, P5, P7, P8) |
| G1 Register | US1 | P0 (WP-06), P2 (WP-20 to WP-22) | `v_unmapped_entries` empty; reconciliation (`docs/register/reconciliation.csv`) lists every source entry; three planted entries detected; `workable-items diff` in sync; external trackers each `SYNCED` or `SKIPPED` with reason | docs/04 §13, docs/03 §15 | register absent |
| G2 Audit | US2 | P3 (WP-30 to WP-39R) | every component has a recorded result; two runs from one state give identical `findings.index.jsonl` files after sorting (the timestamp-free projection of docs/02 §9: sort key plus fingerprint); a planted defect is reported; every finding file (`specs/001-full-project-audit-remediation/audit/findings/<FND-NNNN>.json`) validates against `finding/1` and maps 1:1 to the index lines of each run that observed it | docs/02 §12, `contracts/finding.schema.json` | not started |
| G3 Fixes proven | US3 | P5 (WP-50 to WP-55), P6 (WP-61), P7 (WP-70) | per fixed item: RED on pre-fix artifact, GREEN x3 identical, fingerprints differ, mutation caught, review GO (a verdict file valid against `review-verdict/1`); matrix shows zero absent applicable cells; zero `BLOCKED` counted as pass; reviewer sample with zero survivors | ledger entries in `specs/001-full-project-audit-remediation/evidence/ledger.jsonl` valid against `ev/1`, verdict files | not started |
| G4 Documentation | US4 | P3 (WP-37), P6 (WP-63, WP-64) | crawl: class A/B orphans 0, broken links 0, broken anchors 0; export check stale 0 and missing 0; schema/route/env diff reports empty; every diagram rendered non-blank | step 4, export checker, diff gates | 2,515 orphans, 123 broken links, 83 broken anchors |
| G5 Applications and contracts | US5 | P4 (WP-40, WP-41), P5 (WP-51 to WP-54) | coverage matrix row per application; can-i-deploy matrix all `compatible` | contract gate | 68 undocumented routes, 123 client calls without route (leads) |
| G6 Dependencies | US6 | P0 (WP-07), P5 (WP-55, WP-57) | every dependency listed with version, upstream version and status; behind items carry a decision; accepted updates have full-suite evidence | dependency report | constitution 8 remotes `LOCAL-BEHIND` |
| G7 Repository state | US7 | P0 (WP-03 baseline), P7 (WP-73) | `scripts/repo/verify_repos.sh --strict`: exit 0, failing 0, unproven 0, exceptions explained; under ODG-41 option (a) a failing row is accepted only as an explained exception naming its foreign-only commits, and until the owner answers such a row, and a `foreign_outside_candidate` commit of the main repository, is `blocked: ODG-41`, never a pass (revision 9) | step 3 | POC exit 1 (main repository untracked planning files) |
| Final | SC-011, SC-012 | P6 (WP-62), P7 (WP-71, WP-74) | performance baselines and owner-approved targets for every critical operation with no regression; report checker finds zero completion claims without `ledger#seq` | `specs/001-full-project-audit-remediation/perf/targets.yaml`, report checker | not started |

Completion additionally waits on the owner decisions in `research.md` section 5 that block the work in question (79 recorded: 12 with reversible defaults, 1 resolved by a plan decision, 66 blocking), and on docs/21 ODG-39 (disposition of the doc18 innovation entries, WP-20, WP-74), ODG-40 (licence policy, WP-35 and WP-57) and ODG-41 (how the final strict verification treats commits that other actors push to an own-organisation repository after that repository's reference tips, and commits of other actors that a merge brings into the main repository after the release candidate was built, WP-73 and WP-74R), which have no `research.md` counterpart. Index of every document of this folder: [README.md](README.md).
