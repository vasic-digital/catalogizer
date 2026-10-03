# repo_verify - recursive repository verifier (read-only)

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-03 |
| Last modified | 2026-10-04 |
| Status | proof of concept, executed; read-only tool (revision 3: the contract note below states that `contracts/repo-verification-report.schema.json` requires the mode boolean `no_remote` from contracts revision 9 (tasks.md rev 11 T031, T033), which this POC does not write, so `results/run1.json` is no longer a valid instance; no command, result or self-test changed. Revision 2: this §11.4.44 header table added, which the revision-header check of tasks.md T040 reads in the first 40 lines; it lacked one through four review rounds, the last being the round-9 review of commit `b9412d06`; no command, result or self-test of the repository verifier changed. Revision 1 is commit `0f6b17da`) |

`verify_repo.sh` reports, for the main repository and every submodule at every depth: working-tree
dirtiness, pin state (`git submodule status` marker) and, for owned repos on a branch, how every remote's
branch tip relates to local HEAD (`SAME`, `REMOTE-BEHIND` = local ahead, `LOCAL-BEHIND`, `DIVERGED`,
`UNREACHABLE`, `NO-REMOTE-BRANCH`, `UNKNOWN-DIFFERENT`). It never uses `git submodule foreach`, so one
clean or broken repo cannot abort the run. Output: human table plus machine JSON (`--json-out`).

Requires: bash, git, jq, timeout, xargs. Network: only `git ls-remote` (and `git fetch` with `--fetch`).

```
verify_repo.sh --root . --jobs 8 --timeout 25 --json-out results/run1.json   # default read-only run
verify_repo.sh --fetch ...                                                    # fetch objects so ancestry can be decided
verify_repo.sh --strict ...                                                   # behind/pin drift/unproven also fail
verify_repo.sh --self-test                                                    # 24 deterministic checks, local fixtures only
```

Exit codes: `0` clear, `1` dirty / ahead / diverged (strict: also behind, pin drift, unproven),
`2` usage, `3` nothing failed but a remote comparison is unproven. "Could not verify" is never "clean".
Known quirks are listed in `exceptions.tsv` (`path<TAB>kind<TAB>reason`); default entry:
`submodules/helix_qa/tools/opensource/docling` dirty (tracked test-data file shows modified; reason recorded as CRLF quirk).

Design notes: a submodule's dirtiness is judged on its own row (`status --ignore-submodules=all` in the
parent) so an excepted child does not make every ancestor dirty; owned = any remote URL whose organisation is in
`--owned-orgs` (default `vasic-digital,HelixDevelopment,milos85vasic`); `ls-remote` output is filtered with
`^[0-9a-f]{40}[[:space:]]` so ssh banners cannot be mistaken for a tip.

Self-test (golden-good / golden-bad / negative control): clean+synced -> exit 0 `SAME`; dirty, ahead, diverged ->
exit 1 with the right class; behind is `UNKNOWN-DIFFERENT` without `--fetch` (never guessed) and `LOCAL-BEHIND` with it;
unreachable -> `UNREACHABLE` exit 3; controls: ssh-banner noise still parses `SAME`, a noise-only answer is not `SAME`,
a clean repo before a dirty sibling does not abort the run, exceptions are honoured and recorded.

Results: `results/run1.*` (command, timestamp, JSON, table, exit code, wall time), `results/selftest.txt`.

Contract note (revision 3): the JSON follows `repo-verification-report/1`, which from contracts revision 9 also requires the mode boolean `no_remote` (tasks.md T031: every report records its mode in `fetch`, `strict` and `no_remote`, so a `--no-remote` report is never mistaken for a plain report of repositories that have no remotes). This POC accepts `--no-remote` but its jq assembly (line 259) writes `fetch` and `strict` only, so `results/run1.json` now fails validation with exactly that one missing property (re-checked on 2026-10-04, quickstart.md step 6); the promoted verifier `scripts/repo/verify_repos.sh` (tasks.md T032) writes the field. The POC itself is not changed: its results stay the record of the executed run.
