# repo_verify - recursive repository verifier (read-only)

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
