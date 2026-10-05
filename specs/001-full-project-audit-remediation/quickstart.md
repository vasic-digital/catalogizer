# Quickstart: Validate the Planning Baseline

| Field | Value |
|---|---|
| Feature | `specs/001-full-project-audit-remediation` |
| Created | 2026-10-03 |
| Revision | 24 |
| Status | draft (revision 24, analyze-remediation of 2026-10-05: follows tasks.md rev 36 with its analyze-remediation (685 tasks: the 595 frozen ids T001 to T595 plus 90 suffix ids, the round adding T011a, T045a, T048a, T051a, T081a, T087a, T429a, T530a and T574a; 323 `[TDD]`, 87 `[REVIEW]`, 116 `[P]`, 80 `[SUBAGENT]`, counted by script; 1,502 edges from the `depends on` clauses and 1,517 with the `after` phrases, 43 forward edges over 32 tasks, no cycle); section 1 states that its remote build host prerequisite is unmet pending ODG-07 and that the gate commands of section 9 do not exist before implementation; the section 6 line of the POC `run1.json` of `repo_verify` is marked historical (it now fails `repo-verification-report/1` on the missing `no_remote`, re-checked 2026-10-05); G1 cites T176 for `workable-items diff`; the owner-decision counts are restated once by granularity. Revision 23: follows tasks.md rev 36, commit `f44e2bb9` (676 tasks: the 595 frozen ids T001 to T595 plus 81 suffix ids, rev 34, rev 35 and rev 36 adding no id and rewording 128 against rev 33, 32 of them in rev 36; 318 `[TDD]`, 86 `[REVIEW]`, 115 `[P]`, 80 `[SUBAGENT]`, counted by script; 1,471 edges from the `depends on` clauses and 1,486 with the `after` phrases, 43 forward edges over 32 tasks, no cycle) and docs/21 revision 21, amended in place for tasks.md rev 36 without a new revision number, because tasks.md rev 36 still cites it as "current revision, 21 at this writing": the G7 gate states the rev 34 to rev 36 changes (the first-parent adoption fast-forward, `reanchor` on its typed phrase, the C3 harness of every CPA fixture and the per-class refusal of an unset `CPA_APPROVED_DIR`); the final gate states that rev 34 removed the released-seam precheck, the release-seam scripts running only through `cpa-host --exec-approved`, the pre-release production form with `pre_release_output`, the `--op-id` expiry, the `--holder-pid` grant and the regenerated claim ledger; the launcher trust design frozen at C1 to C12 (docs/21 section 12.22). Revision 22: follows tasks.md rev 33, commit `711a8c30` (676 tasks: the 595 frozen ids T001 to T595 plus 81 suffix ids, rev 33 adding no id and rewording 71; 318 `[TDD]`, 86 `[REVIEW]`, 115 `[P]`, 80 `[SUBAGENT]`, counted by script; 1,451 edges from the `depends on` clauses and 1,466 with the `after` phrases, 43 forward edges over 32 tasks, no cycle) and docs/21 revision 21, amended in place for tasks.md rev 33 without a new revision number, because tasks.md rev 33 still cites it as "current revision, 21 at this writing": the G7 gate states the empty hooks directory of every CPA git call, the first-parent fast-forward, the pending exit of at least 14 for a keyed check row the owner has not approved and the refusal of a run directory that `cpa-host` did not write; the final gate states the C2 cancel of a suspended run, the released-seam precheck and T582's `trust-diff.json`. Revision 21: follows tasks.md rev 32, commit `87c6c478` (676 tasks: the 595 frozen ids T001 to T595 plus 81 suffix ids, rev 32 adding T046a and rewording 61; 318 `[TDD]`, 86 `[REVIEW]`, 115 `[P]`, 80 `[SUBAGENT]`, counted by script; 1,442 edges from the `depends on` clauses and 1,457 with the `after` phrases, 43 forward edges over 32 tasks, no cycle) and docs/21 revision 21, amended in place for tasks.md rev 32 without a new revision number, because tasks.md rev 32 still cites it as "current revision, 21 at this writing": the G7 gate states that from the T047 adoption on every commit-push run starts in the owner-installed host entry point `cpa-host` (the owner's decision of 2026-10-05; the tracked script started directly exits 20 `not_via_host_entry`), and the Final gate that the T580e conductor starts and resumes the granted run through `cpa-host`, takes over a crashed conductor's grant with `--grant --adopt`, and that the latest SC-005 iteration is read (`sc005_not_latest`). Revision 20: followed tasks.md rev 31, commit `14bbb979` (675 tasks: the 595 frozen ids T001 to T595 plus 80 suffix ids, rev 31 adding none and rewording 34; 318 `[TDD]`, 85 `[REVIEW]`, 115 `[P]`, 80 `[SUBAGENT]`, counted by script; 1,424 edges from the `depends on` clauses and 1,439 with the `after` phrases, 43 forward edges over 32 tasks, no cycle) and docs/21 revision 21, amended in place for tasks.md rev 31 without a new revision number, because tasks.md rev 31 still cites it as "current revision, 21 at this writing": the Final gate states that the commit-turn grant holder is the T580e conductor, which starts the granted run with `CPA --run-id <id>` through the installed launcher `.audit/bin/cpa`, and that `scripts/qa/sc005_verdict_check.py` is a release-seam file. Revision 19: followed tasks.md rev 30, commit `84145b2e` (675 tasks: the 595 frozen ids T001 to T595 plus 80 suffix ids, rev 30 adding none and rewording 45; 318 `[TDD]`, 85 `[REVIEW]`, 115 `[P]`, 80 `[SUBAGENT]`, counted by script; 1,422 edges from the `depends on` clauses and 1,437 with the `after` phrases, 43 forward edges over 32 tasks, no cycle) and docs/21 revision 21, amended in place for tasks.md rev 30 without a new revision number, because tasks.md rev 30 still cites it as "current revision, 21 at this writing": the Final gate states that T570a also holds the RED/GREEN sample change set `scripts/qa/redgreen_sample.py`, and that the commit-turn grant ends only by its release or a reap of a proven-dead holder. Revision 18: followed tasks.md rev 29, commit `7daa7938` (675 tasks: the 595 frozen ids T001 to T595 plus 80 suffix ids, rev 29 adding T335a and T570a; 318 `[TDD]`, 85 `[REVIEW]`, 115 `[P]`, 80 `[SUBAGENT]`, counted by script) and docs/21 revision 21, amended in place for tasks.md rev 29 without a new revision number, because tasks.md rev 29 cites it as "current revision, 21 at this writing": the Final gate states that T569 waits for the T570a review of the verdict-coverage change set and that the commit turn freezes the main working tree (`commit_turn_held`). Revision 17: followed tasks.md rev 28, commit `fb1d1982` (673 tasks, the same counts as rev 27, commit `13bc5de0`, which rev 28 rewrites without adding an id) and docs/21 revision 21: the Final gate states the SC-005 closure check `--require-final` before the HC-6 decision and the per-deploy increment verdict. Revision 16: followed tasks.md rev 27, commit `13bc5de0` (673 tasks, the same counts as rev 26, commit `616fb74a`, which rev 27 rewrites without adding an id; rev 24 is committed as `c2b01d73` with 670 and rev 25 as `2e1fd314` with 671) and docs/21 revision 20: the Final gate states that the SC-005 sample review releases fixes on a `scoped` GO and gives its `final` GO, which T579a awaits, only after the T580b (3c) re-check, and that a T569 refusal of `docs/security/SLSA_LEVEL.md`, an excluded record of the release-seam rule, is rewritten by T447b and the check re-run. Revision 15: followed tasks.md rev 26, commit `616fb74a` (673 tasks; rev 24 is committed as `c2b01d73` with 670 and rev 25 as `2e1fd314` with 671) and docs/21 revision 19: the build-host prerequisite names the true reason of a blocked baseline leg, `image_not_built` when a qualified host is reachable but the remote image has no built digest (tasks.md T203, T204, T220); the Final gate states that `--seam pre-qa` judges only the candidate's own cycle `qa-<fingerprint>` (`pre_qa_no_candidate_cycle`, accepted by the readiness gate for the ratchet part only) and that each QA deploy mints its own version increment (T564a, T580e). Revision 14: followed the tasks.md rev 24 candidate (670 tasks, since committed as `c2b01d73`; the committed rev 23, commit `d5a37bfe`, has 666) and docs/21 revision 18: the build-host prerequisite states that a WP-23 baseline leg with no reachable qualified host is a `blocked-unavailable` record, so the P2 exit does not wait on the host input (tasks.md T203, T204; docs/21 IC-63); the G2 gate counts the units of the unit partition (T225); G3 names WP-58; G6 states the four classes of third-party pins and the mandatory initial T440a run; the Final gate adds the QA-deploy-readiness gate (T564a), the owner's manual-QA cycle of the candidate (T580e) and the two modes of the escape gates. Revision 13: follows tasks.md rev 17 (641 tasks, commit `b34a03f6`) and docs/21 revision 17: the build-host prerequisite names the bootstrap host list `build/hosts.env` with the hub's SSH identity file (tasks.md T137a), the supervisor-restart leg of the round trip and `git` on the build host (T006a), and the driver secret kept in a mode-0600 file outside the checkout (T005b, docs/16 revision 14 §9.6); the Final gate states that T582 re-runs every release-seam check of T569 on the candidate fingerprint. Revision 12: follows tasks.md rev 14 (636 tasks, commit `9074c177`) and docs/21 revision 16: the build-host prerequisite names the image digests resolved first (tasks.md T005d) and the bootstrap round trip (T006a); the Final gate names the release-seam checks of T569; the closing paragraph states the intake count of tasks.md T072 (57 imported decision items, 55 of them `Operator-blocked`: the 40 open groups and the 15 open section 8.6 decisions, ODG-07 among them for its host identity) and the `research.md` revision 13 count (5 answered, 11 defaults, 1 resolved, 62 blocking). Revision 11: the plan owner's decisions of 2026-10-04 (C1, C2 and the FR-017 answer; docs/21 revision 15) applied: the prerequisites name the remote build host that every build needs, the G6 gate states the move-to-latest rule, and the closing paragraph carries the `research.md` revision 12 count (3 answered, 11 defaults, 1 resolved, 64 blocking). Revision 10: the G5 gate names the can-i-deploy run at the release seam for the candidate provider version (docs/21 IC-61), and the closing paragraph adds ODG-42, whether FR-008 and SC-003 are amended for the legacy headerless class (docs/21 IC-60; tasks.md rev 13 T576, T588, T593). Revision 9: the exit criterion of step 3, the G7 gate and the closing paragraph state the ODG-41 branch of the final verification (tasks.md rev 12 T582, T583, T595b; docs/21 WP-74R and IC-58, the round-11 content review m-6): a strict run that fails only on rows another actor's commits explain is reported `blocked: ODG-41` until the owner answers, and under option (a) passes when every failing row is an explained exception naming its foreign-only commits; the main repository's `foreign_outside_candidate` commits are the same kind; only any other failure reopens WP-73; no command, output or expectation of the executed run changed. Revision 8: `repo-verification-report.schema.json` now requires the mode boolean `no_remote` (contracts revision 9, tasks.md rev 11 T031, T033), which the POC verifier does not write (its jq assembly, `poc/repo_verify/verify_repo.sh` line 259, writes `fetch` and `strict` only), so steps 3 and 6 state that the POC's documents are no longer valid instances (measured on the stored `run1.json` on 2026-10-04, where the missing field is the only error; for a fresh run read from the POC's source, not executed) and that a re-run of the step 6 script now exits 1 for that reason; the other stored POC results stay VALID; no command of the executed run changed. Revision 7: step 6 notes that `review-verdict.schema.json` revision 8 (contracts revision 8) requires `blocking_findings`, and the syntax and meta-schema checks re-run on all 7 files, with the four stored POC results VALID against their schemas, while this revision was written; this header gains the §11.4.44 `Status` row it lacked (round-9 consistency review of commit `b9412d06`); no command, output or expectation of the executed run changed. Revision 6: step 6 notes the seventh contract schema, `review-verdict.schema.json`, added after the executed run (syntax and meta-schema checks re-run on all 7 files while revision 6 was written; its fixture results are in `contracts/README.md`); the G3 gate names the review verdict format; the closing paragraph names docs/21 ODG-41, which also has no `research.md` counterpart. Revision 5: the strict-run expectation of step 3 is dated: the two failing classes belong to the 11:44:31Z run, and a read-only re-measurement at HEAD `1a69eed5` (19:50:33Z) finds the main repository clean and the constitution's remote tip absent locally, so the expected result is 14 without `--fetch` and the strict-`behind` code with it; the fixture-validation count follows `finding/1` revision 4 (43 samples). Revision 4: the exit-code precedence is cited as tasks.md T032, the id in tasks.md rev 6; ODG-39 blocks WP-20 and WP-74 as in docs/21 revision 8. Revision 3: production verifier modes per data-model §9; expected strict exit stated with its open precedence; fixture count; G2 index projection; G3 phase mapping; golden file names) |
| Last modified | 2026-10-05 |
| Executed | 2026-10-03, from 11:44:31Z, main repository HEAD `e4852ce7e1a136818b7b63524e94b5f8ee1b68bf` |
| Scope | read-only commands that prove what the plan starts from; no build, no install, no write to any repository |

Every command marked EXECUTED was run in this session and its output is quoted. Commands marked NOT EXECUTED are the planned checks whose tooling does not exist yet. Outputs go to a scratch directory, never into the repository (set `OUT` to any directory outside the tree).

## 1. Prerequisites

**State on 2026-10-05 (analyze-remediation A25).** The last row below, a qualified remote build host, is UNMET: its identity is the open owner input ODG-07, which tasks.md T014 makes an HC-0 entry criterion, so no step that builds can run until the owner answers. The read-only steps 2 to 8 need no build host. The producing commands named in the section 9 gate table (for example `scripts/repo/verify_repos.sh`, the export checker and the contract gate) do not exist before implementation; they are deliverables of the tasks cited there, and only the POC tools under `poc/` run today.

| Need | Check | Observed |
|---|---|---|
| bash, git, jq, ssh with access to the project remotes | `command -v git jq ssh` | present |
| Python 3 with PyYAML and `jsonschema` | `python3 -c "import yaml, jsonschema"` | Python 3.14.4, jsonschema 4.19.2 |
| CodeGraph CLI | `codegraph --version` | `1.6.0` |
| Lumen (MCP plugin) with Ollama | `health_check` tool | `Backend: ollama ... Model: ordis/jina-embeddings-v2-base-code Status: OK` |
| Network for `git ls-remote` (step 3) | SSH keys loaded, `BatchMode` | 8 remotes of the main repository answered |
| A qualified remote build host for any build (revision 11: the owner's decision C1 leaves no local build; docs/21 ODG-07, docs/16 §9.5 and §9.6) | `scripts/containers/probe_host.sh --remote <host>` (tasks.md T099, T006a); revision 12: the digests of the pulled bootstrap images resolved and recorded first (T005d), then one dispatcher round trip with a trivial pinned build whose `completed` event is consumed once (T006a); revision 13: the host list `build/hosts.env` (gitignored, mode 0600, naming the hub's SSH key file `BUILD_SSH_IDENTITY` outside the checkout; first form T137a), a supervisor-restart leg in that round trip, `git --version` recorded on the build host, and the dispatcher's driver secret in the mode-0600 file `${XDG_STATE_HOME:-$HOME/.local/state}/catalogizer/<checkout-id>/build_hmac.key`, outside the checkout and outside every container mount (T005b); revision 14: P0 cannot exit without a qualified host (T006a), while a later build that finds no reachable qualified host is `blocked-unavailable` with a reason from the closed set of T005a, never run locally and never a pass, and a WP-23 baseline leg in that state is recorded as such so that the P2 exit does not wait on the host input (T203, T204; docs/21 IC-63); revision 15: the recorded reason is the true one, decided by `scripts/coverage/blocked_reason.sh` in the order host, then image, then wrapper: `no_qualified_host` or `host_unreachable` only when the host check says so, `image_not_built` when a qualified host is reachable but the image's lock entry has no built digest, and `artifact_not_yet_built` (T203, T204, T220) | NOT EXECUTED: the tool does not exist yet and the host identity is an owner input; the steps of this file build nothing, so none needs it |

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

Historical line (analyze-remediation A25, 2026-10-05): the first `VALID` line of the transcript above (kept byte-unchanged as captured) was true on 2026-10-03 only. `repo-verification-report/1` later made `no_remote` required, and the stored POC run `poc/repo_verify/results/run1.json` now fails it with one error, "'no_remote' is a required property" (re-checked with `jsonschema` Draft 2020-12 on 2026-10-05; contracts/README.md records the same result). A fresh `verify_repo.sh --json-out` output is the instance to validate.

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
| G1 Register | US1 | P0 (WP-06), P2 (WP-20 to WP-22) | `v_unmapped_entries` empty; reconciliation (`docs/register/reconciliation.csv`) lists every source entry; three planted entries detected; `workable-items diff` (the engine's DB-to-documents comparison, run by tasks.md T176) in sync; external trackers each `SYNCED` or `SKIPPED` with reason | docs/04 §13, docs/03 §15 | register absent |
| G2 Audit | US2 | P3 (WP-30 to WP-39R) | every unit of the unit partition `specs/001-full-project-audit-remediation/audit/unit-partition.tsv` has a recorded result, and every tracked path of the input commit lies in exactly one unit or a reasoned exclusion (tasks.md T225; revision 14); each unit's index readiness recorded before its detectors run (T225c, T241a, T254a, T272a); two runs from one state give identical `findings.index.jsonl` files after sorting (the timestamp-free projection of docs/02 §9: sort key plus fingerprint); a planted defect is reported; every finding file (`specs/001-full-project-audit-remediation/audit/findings/<FND-NNNN>.json`) validates against `finding/1` and maps 1:1 to the index lines of each run that observed it | docs/02 §12, `contracts/finding.schema.json` | not started |
| G3 Fixes proven | US3 | P5 (WP-50 to WP-55, WP-58), P6 (WP-61), P7 (WP-70) | per fixed item: RED on pre-fix artifact, GREEN x3 identical, fingerprints differ, mutation caught, review GO (a verdict file valid against `review-verdict/1`); matrix shows zero absent applicable cells; zero `BLOCKED` counted as pass; reviewer sample with zero survivors | ledger entries in `specs/001-full-project-audit-remediation/evidence/ledger.jsonl` valid against `ev/1`, verdict files | not started |
| G4 Documentation | US4 | P3 (WP-37), P6 (WP-63, WP-64) | crawl: class A/B orphans 0, broken links 0, broken anchors 0; export check stale 0 and missing 0; schema/route/env diff reports empty; every diagram rendered non-blank | step 4, export checker, diff gates | 2,515 orphans, 123 broken links, 83 broken anchors |
| G5 Applications and contracts | US5 | P4 (WP-40, WP-41), P5 (WP-51 to WP-54) | coverage matrix row per application; can-i-deploy matrix all `compatible`, and at the release seam green for the candidate provider version `catalog-api@<candidate fingerprint>`, its verdict written under `.audit/out/<op_id>/` (docs/21 IC-61) | contract gate | 68 undocumented routes, 123 client calls without route (leads) |
| G6 Dependencies | US6 | P0 (WP-07), P5 (WP-55, WP-57) | every dependency listed with version, upstream version and status; every submodule at every depth and every package dependency at its latest upstream or carrying a `blocked` decision with its reason on an open register item (revision 11, the owner's FR-017 answer; tasks.md T440, T440a, T455); every move has full-suite evidence; revision 14: the 47 third-party pins are classed by the repository whose `.gitmodules` records them (1 in the main repository, 27 under an owned parent, 16 under the constitution, moved by T580b, and 3 under third-party parents, which follow their parents, `followed_parent_pin`, closed as `not our pointer`, tasks.md T440), and the mandatory initial T440a run reads every pin and package dependency against its latest upstream once in P5 | dependency report | constitution 8 remotes `LOCAL-BEHIND` |
| G7 Repository state | US7 | P0 (WP-03 baseline), P7 (WP-73) | `scripts/repo/verify_repos.sh --strict`: exit 0, failing 0, unproven 0, exceptions explained; under ODG-41 option (a) a failing row is accepted only as an explained exception naming its foreign-only commits, and until the owner answers such a row, and a `foreign_outside_candidate` commit of the main repository, is `blocked: ODG-41`, never a pass (revision 9); revision 21 (tasks.md rev 32): from the T047 adoption on every commit lands through a commit-push run started in the owner-installed host entry point `cpa-host`, which executes only the owner-approved copies of its code (the tracked `scripts/commit-push-all.sh` started directly exits 20 `not_via_host_entry`), and the adoption waits for the owner install checkpoint T046a, `$EV/hc/HC-0-host-entry.json` (T042, T046a, T047); revision 22 (tasks.md rev 33): every CPA git call runs with an empty hooks directory (`-c core.hooksPath=$CPA_RUN/no-hooks`), S1 fast-forwards only along the tip's first-parent chain, a keyed check row that the owner has not approved is reported `check_pending_release` and the run exits at least 14, never 0 or 20 (10, nothing committed, when a check of the same run ran and failed), and a run directory that `cpa-host` did not write is refused `snapshot_not_trusted` (T040, T042); revision 23 (tasks.md rev 34 to rev 36, the design then frozen at the central decisions C1 to C12 of T042): S1 fast-forwards only when the tip's first-parent chain holds the trust entry's `adoption_commit`, else a held `--no-ff` merge with HEAD first (CENTRAL C9), the owner may `reanchor` before or after the adoption on the typed phrase `REANCHOR <HEAD sha>` (CENTRAL C5), every fixture that runs the real CPA path uses the T039 harness with a throwaway bare remote and the recorded `--commit` replay (CENTRAL C3), and a helper or release-seam script run without `CPA_APPROVED_DIR` is refused `helper_not_approved` or `release_seam_unreleased` by class (CENTRAL C8) (T039, T042) | step 3 | POC exit 1 (main repository untracked planning files) |
| Final | SC-011, SC-012 | P6 (WP-62), P7 (WP-71, WP-74) | performance baselines and owner-approved targets for every critical operation with no regression; the release-seam checks of tasks.md T569 pass on the candidate fingerprint (revision 12: `scripts/perf/release_check.sh`, `scripts/qa/escape_gates.sh`, `scripts/docs/claim_ledger.py`, `mutation_ratchet_challenge.sh` and `scripts/qa/observable_assertions.sh` at 85% and 60% per application, `scripts/supply_chain/check_slsa.sh --check-record docs/security/SLSA_LEVEL.md` and the can-i-deploy check; an absent verdict is BLOCKED; revision 13: T582 re-runs every one of them, the SLSA record check included, on the T566 candidate fingerprint before its strict run; revision 14: the escape gates run `--seam pre-qa` at T569 and `--seam final` at T582; revision 15: `--seam pre-qa` judges only the cycle `qa-<fingerprint>` of the candidate and reports `pre_qa_no_candidate_cycle` while it does not exist, which the readiness gate accepts for the ratchet part only, the catchability part needing a PASS, T554, T564a); the owner's live manual QA of the candidate after HC-6: the QA-deploy-readiness gate `scripts/release/qa_handoff_gate.sh <fingerprint>` passes first (tasks.md T564a), the session is recorded as the manual-QA cycle `qa-<fingerprint>` with every finding in `reg_discovery` and none left open at HC-7 (T580e, T580f, T593); each QA deploy mints its own version increment, applied as the first held commit of its fix cycle when the session records a finding, only the final, finding-free candidate's increment deferred until T595b has passed (T580e; docs/21 revision 19); the SC-005 reviewer-mutation sample gives a `scoped` GO that releases the fixes it covers and a `final` GO, which the late pin catch-up awaits, only after each survivor routed `routed_to_T580b` is re-checked caught on the T580b (3c) commit, each verdict passing `scripts/qa/sc005_verdict_check.py` (T570, T571, T579a); a T569 refusal of `docs/security/SLSA_LEVEL.md`, an excluded record of the release-seam rule, is routed to a T447b rewrite held on `$EV/reviews/WP-56-slsa-<fingerprint>.json` and the check re-run (T447b, T569; docs/21 revision 20); before the HC-6 decision the closure mode `sc005_verdict_check.py --require-final` passes on the latest T571 verdict for the candidate fingerprint (`final_go_required`, `sc005_candidate_mismatch`, `final_go_stale`), and each QA deploy's increment is held on its own verdict `$EV/reviews/WP-73-increment-<fingerprint>.json` (T570, T580, T580e; docs/21 revision 21); since tasks.md rev 29 the T569 checks wait for the G-GATE review T570a of the verdict-coverage change set (`$EV/reviews/WP-71-verdict-coverage.json`), and from a commit-turn grant to that run's commit no stream writes the main working tree (`commit_turn_held`, T580e); since tasks.md rev 30 T570a also holds and reviews `scripts/qa/redgreen_sample.py`, a listed release-seam file, and the grant (`{run_id, paths_from_sha256, pid, cmdline, started_at}`) has no time-based expiry, `commit_turn_check.sh --reap` removing it only for a proven-dead holder (T570, T570a, T580e); since tasks.md rev 31 the grant holder is the T580e conductor, which mints the run id and starts the run with `CPA --run-id <id>` through `.audit/bin/cpa`, and `scripts/qa/sc005_verdict_check.py` is a listed release-seam file reviewed by the first T571 draw (T571, T580e); since tasks.md rev 32 the conductor starts that run as `cpa-host --run-id <id>` and resumes it as `cpa-host --resume <id>` (`.audit/bin/cpa` removed), a new conductor session takes over a crashed conductor's grant with `commit_turn_check.sh --grant --adopt <run id>` (`adopt_no_grant`, `adopt_run_mismatch`, `adopt_holder_alive`, release reasons `conductor_adopted` and `run_cancelled`), the T580d hook-commit run is the last CPA run before T582, and the latest T571 verdict is the highest iteration `$EV/reviews/WP-71-sc005-r<n>.json` (`sc005_not_latest`) (T570, T571, T580d, T580e); since tasks.md rev 33 a suspended run is cancelled by `scripts/longops/holder.sh` reporting it `expired` and `scripts/longops/acquire.sh --expire` run through `cpa-host --exec-approved` (central decision C2), the release-seam callers run the precheck `scripts/release/seam_released.sh` first (`release_seam_unreleased`), and T582 writes `.audit/out/<op_id>/trust-diff.json` (T564a, T569, T580e, T582); since tasks.md rev 34 that precheck is removed and every release-seam script runs only through `cpa-host --exec-approved`, a script not yet owner-approved runs in the pre-release production form whose `pre_release` outputs every seam refuses (`pre_release_output`), the outside-run expiry carries `--op-id <op_id>`, the grant takes `--holder-pid` (`holder_pid_not_ancestor`), and T548a regenerates the tracked claim ledger without the marker before T569 runs (T548a, T564a, T569, T580e, T582); report checker finds zero completion claims without `ledger#seq` | `specs/001-full-project-audit-remediation/perf/targets.yaml`, report checker | not started |

Completion additionally waits on the owner decisions in `research.md` section 5 that block the work in question (one set, counted three ways: 79 `research.md` ids OD-01 to OD-79; 42 docs/21 groups ODG-01 to ODG-42 plus 16 ungrouped OD ids, 15 open, with ODG-43 reserved for the FR-012 class C and D scope; 57 tasks.md register items, 55 `Operator-blocked`, 40 groups and the 15 open OD ids; 79 recorded; `research.md` revision 13: 5 answered by the owner on 2026-10-04 (OD-01, OD-30, OD-38, and OD-62 and OD-78, which tasks.md rev 14 decides by the FR-017 answer), 11 with reversible defaults, 1 resolved by a plan decision, 62 blocking, the build-host identity OD-04 among them), and on docs/21 ODG-39 (disposition of the doc18 innovation entries, WP-20, WP-74), ODG-40 (licence policy, WP-35 and WP-57) and ODG-41 (how the final strict verification treats commits that other actors push to an own-organisation repository after that repository's reference tips, and commits of other actors that a merge brings into the main repository after the release candidate was built, WP-73 and WP-74R) and ODG-42 (whether FR-008 and SC-003 are amended for the findings of the tracked `legacy headerless documents` item, answered on 2026-10-04 in spec.md revision 7, pending confirmation; until the owner confirms it, they are stated UNMET while that item is open, WP-72, WP-74 and WP-74R), which have no `research.md` counterpart. The owner-decision intake of tasks.md T070 to T072 imports 57 items (the 42 groups and the 15 open section 8.6 decisions), of which 55 stay `Operator-blocked`: 40 groups (Yes 2, Partial 3, UNCONFIRMED 4, No 31; ODG-07 among them for the build-host identity, which C1 left open) and the 15 decisions, while ODG-13 and ODG-16 carry the owner's answers (docs/21 revision 16 section 8, unchanged in revisions 17 and 18; revision 12). Index of every document of this folder: [README.md](README.md).
