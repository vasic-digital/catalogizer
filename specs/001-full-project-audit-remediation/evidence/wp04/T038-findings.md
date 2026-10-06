# T038 findings: existing commit/push tools (read-only)

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-05 |
| Last modified | 2026-10-06 |
| Status | read-only findings note of T038; the section 11.4.44 header was added in fix round 4 (WF3 review m9) |

Date: 2026-10-05. Mode: read only, nothing was executed that has side effects (no commit, push, hook install or tool run).
Task: tasks.md:181 (T038). Task text names `$EV/wp04/existing-tools-read.md`; this file was written at the path the caller requested. Copy or rename at integration.
Cites: docs/16 D-14 (docs/16*.md:140), V-03 (:1789), :1599, :1720.

## 1. `type -a commit` resolution

- `type -a commit` in this shell resolves ONE entry: `/home/milosvasic/Projects/project_toolkit/Upstreamable/commit` (shell script, 23 lines). Root `commit` of the repo is NOT on PATH (cwd is not in PATH).
- PATH wiring: `~/.bashrc:43-48` exports `SUBMODULES_HOME=$PROJECTS/project_toolkit` and appends `$SUBMODULES_HOME`, `Upstreamable`, `Installable` to PATH.
- D-14 premise confirmed: the repo root `commit` (tracked, `git ls-files commit`) delegates to an external binary outside version control of this repo. The external tool is in the git repo `project_toolkit` (HEAD 76cdfa9 "commit.sh no longer exits 0 on a failed commit"), a different repository from this one.

## 2. Call chain (all read)

1. Repo root `commit` (commit:3-35): sources `./env.properties` if present (relative path, cwd dependent), builds `MESSAGE` from `$1`/`PROJECT_NAME`/`DEFAULT_COMMIT_MESSAGE`, then `echo ... && commit "$MESSAGE" && echo "...completed"` (commit:34-36). `commit` here resolves via PATH to Upstreamable/commit.
2. `project_toolkit/Upstreamable/commit` (:1-24): requires `$SUBMODULES_HOME`, then `bash $SUBMODULES_HOME/Upstreamable/commit.sh [msg]`. Exit status of that last command is the script's status.
3. `project_toolkit/Upstreamable/commit.sh` (:1-56): sources `~/.zshrc` (preferred) else `~/.bashrc` (:26-27, output discarded), then `bash $SUBMODULES_HOME/Software-Toolkit/Utils/Git/commit.sh "$MESSAGE"`; failure prints "ERROR: Commit failure", exit 1 (:46-50).
4. `project_toolkit/Software-Toolkit/Utils/Git/commit.sh` (:1-49): `git add .` (:29), nothing staged -> "Nothing to commit", exit 0 (:37-41), `git commit -m` (:43), then `bash .../push_all.sh` (:49). Fixed 2026-09-22 per its comment (:25-28) so failed add/commit now exit 1.
5. `.../Utils/Git/push_all.sh` (:1-168): see section 3.

## 3. Behaviour: recursion, divergence, force

- Recursion into submodules: NONE. `git add .` stages a submodule only as a pointer change; nothing in the chain iterates `.gitmodules` or pushes submodule repos. (Contrast `scripts/push_all_submodules.sh`, section 5.)
- Remote set: push_all.sh locates an `Upstreams/` (or lowercase `upstreams/`) recipe directory (:19-84), runs `install_upstreams.sh` on it (:88), then for each `*.sh` recipe, derives remote NAME from the lowercased filename and runs `git push "$NAME"` (:122, :130-144). This repo has `Upstreams/` with GitFlicVasicDigital, GitHub, GitHubVasicDigital, GitLab, GitLabVasicDigital, GitVerseVasicDigital .sh; `git remote -v` shows matching remotes (gitflicvasicdigital, github, githubvasicdigital, gitlab, gitlabvasicdigital, gitversevasicdigital) plus origin with 6 pushurls and an `upstream` remote.
- Divergence handling: only after a SUCCESSFUL `git push NAME` it runs `git config pull.rebase false && git fetch && git pull` (:122-125). A rejected (diverged) push is not handled: the `if` simply does not enter, nothing is reported, loop continues. The `git pull` after a good push is a merge pull (`pull.rebase=false` is set by the tool; local config already has pull.rebase=false) and can create a local merge commit that is not pushed in this run.
- Force path: NONE found. `grep -E 'force|--no-verify|reset --hard|\+refs'` over `Utils/Git/*.sh` and `Upstreamable/*.sh` returned no hit (only the sibling files were scanned; `install_upstreams.sh` was read for the first 80 lines only, rest UNCONFIRMED). The tool never passes `--force`, `--force-with-lease`, `+ref`, or `--no-verify`.
- Exit-code contract is WEAK (relevant to FR exit-code authority, docs/16 :1599): `PROCESS_UPSTREAM` result is not checked (:144 and no `||`); a failed push to any of the remotes does not change the exit status. Final exit is the status of `git push --tags >/dev/null 2>&1` (:155-163): tag push failure exit 1, otherwise 0. So the tool can exit 0 with a branch not pushed to one or more upstreams. Also tag push runs once against the default remote only (not per recipe remote).
- Tag push uses `git push --tags` without a ref check; no fetch-before-edit, no fast-forward verification, no post-push read-back of remote heads.
- `git add .` stages EVERYTHING (untracked included) with no scope check; for this repo that includes the current untracked audit/spec trees. Message is overridable via env/`$1`; no staging allow-list.
- Host env coupling: needs `SUBMODULES_HOME` and the user's rc files being sourceable (commit.sh:26-27); not hermetic, not reproducible in a container without the toolkit checkout.

## 4. Repo-owned finding: `env.properties` is tracked

- `git ls-files env.properties` lists it as TRACKED (last touched 4572d669). `git check-ignore -v env.properties` returned no match (rc=1), so it is not ignored. It is sourced by root `commit` (commit:5-9).
- It declares credential-class variable names (SHARECONNECT_*_PASSWORD, FIREBASE_TOKEN, FIREBASE_*_APP_CREDENTIALS_FILE ...). Values checked by length only (values not printed): all credential variables are currently EMPTY, PROJECT_NAME / DEFAULT_COMMIT_MESSAGE / two FIREBASE_DISTRIBUTION values are non-empty. So no secret value was observed, but the file is a tracked, sourced slot for secrets: the next time someone fills it, `git add .` in the toolkit chain commits and pushes it to 6 upstreams. Classify as a CONST-042 / 11.4.10 risk to route to the findings register (severity owner decision). Whether older revisions held values: UNCONFIRMED (history not scanned).

## 5. `scripts/hooks/pre-push-gate.sh` (151 lines)

- Gates, in order: 1 `scripts/detect-landmines.sh` hard (:33-49); 1b `scripts/audit/anti-bluff-scan.sh` hard (:59-75); 2 builds `/tmp/judge_prompt.md` from diff vs `origin/main` (:80-119); 3 prints operator instructions, optional `claude --headless` when `LLM_JUDGE_INTERACTIVE=1` (:136-148). Exit codes captured directly (:38, :61).
- Bypass: `LLM_JUDGE_BYPASS=1` continues past both hard gates (:40-42, :63-65, usage line :23). This is an env-var escape without expiry, authoriser roster or tracked item, i.e. not the 11.4.271 waiver schema and not recorded anywhere (no log written).
- Gate 3 is advisory: judge verdict is not machine-enforced unless `LLM_JUDGE_INTERACTIVE=1` and even then `claude --headless` flags (:138) are UNCONFIRMED as valid for the installed CLI; failures `|| true` (:138) and a missing/empty verdict file passes (:139-148).
- Writes fixed `/tmp/judge_prompt.md` and `/tmp/pr.diff` (shared, predictable paths, TOCTOU/overwrite across concurrent runs; also contradicts scratch-dir discipline).
- Empty diff -> exit 0 before gate 3 (:95-98), after the two hard gates already ran.
- Hook wiring is by symlink from `.git/hooks/pre-push` (comment :18-20). The relevant constraint from 11.4.234: automatic hooks must not block the commit/push mechanism; this body runs multi-step gates on every push.

## 6. `scripts/install_git_hooks.sh` (74 lines)

- Installs `.git/hooks/pre-push` as a symlink to `scripts/hooks/pre-push-gate.sh` (:44-72), honours `core.hooksPath` if set (:47-51), backs up a non-symlink existing pre-push as `.bak.<epoch>` (:64-67). Idempotent. Never touches tracked files. Re-running it is what would CONNECT the hook, so it must not be run before T040-class decisions.

## 7. `scripts/push_all_submodules.sh` (147 lines)

- Scope: every path under `submodules/*` from `.gitmodules` (:59-60). Default `DRY_RUN=1` prints a plan only (:31, :18-21), `DRY_RUN=0` executes.
- Recursion: ONE level only: `git -C <p>`; nested submodules inside a submodule are not walked (e.g. helix_qa nested gitlinks are only protected by `restore --staged -- tools/opensource tools/external`, :88-89).
- Staging: `git -C p add -A` (:88) despite the header comment saying governance pointer files only (:8-10 say stage ONLY CLAUDE.md/AGENTS.md; code stages all). Comment/code mismatch: it would commit any local change in every owned submodule with the fixed constitution-pointer message (:49-55).
- Divergence: `fetch --all --prune --tags`, then if behind `git merge --no-edit origin/<branch>`; on conflict `merge --abort` and SKIP for manual (:101-117). No force path; no `--force`, `--force-with-lease`, `+ref`, `--no-verify` (verified by reading all 147 lines).
- Push: to every remote of the submodule `git push r br` (:122-133), skipping no-op pushes via `rev-list r/br..HEAD` count (:125-126; uses possibly stale remote refs unless the earlier fetch succeeded). Failures are collected; final exit is `[ ${#PUSH_FAIL[@]} -eq 0 ]` (:147) so a failed push gives nonzero (better than the toolkit chain). SKIPPED submodules (detached, conflict) do NOT change the exit status: exit 0 can hide skipped repos (:142-144 only warn).
- Writes `qa-results/phasef_push.log` (:38-40) with `: >` truncation on every run, even in dry-run (side effect in repo tree, unversioned). A committed message hardcodes a stale co-author line "Claude Opus 4.8 (1M context)" (:55), mismatching the current attribution rule.
- Uses `git -C p restore --staged ... 2>/dev/null || true` which silences real errors (:89).

## 8. Hook state confirmations (T038 required)

- `git config --get core.hooksPath` -> unset (empty output, rc=1). CONFIRMED.
- `.git/hooks/` contains only `*.sample` files; `ls .git/hooks | grep -v sample` printed nothing, `git rev-parse --git-dir` = `.git` (not a worktree file). So NO `pre-push` is installed. CONFIRMED.
- Therefore `scripts/hooks/pre-push-gate.sh` is currently NOT wired; nothing blocks a push today.
- `scripts/commit-push-all.sh` does not exist yet (grep warned no such file), consistent with T042 being unimplemented.

## 9. Recommendation for T042 / adapter decision (docs/16 :1599)

- D-14 resolves to: behaviour now KNOWN (this file). Fit: the toolkit chain is NOT suitable as the audit feature's commit path without an adapter, because of the unchecked branch-push exit status, `git add .` with no scope, no submodule recursion, no divergence report, host-env coupling and tag push to default remote only. Whether it passes a throwaway-clone behavioural test (V-03 second half "test it in a throwaway clone"): NOT DONE (task was read-only), UNCONFIRMED.
- `push_all_submodules.sh` fits submodule integration better (dry-run default, ff/merge, no force) but needs: allow-list staging, skipped=nonzero, no log truncation in dry-run, fixed message/attribution.
- Hook: pre-push-gate.sh should not become an automatic blocking hook (11.4.234 B); its two hard gates should run as an explicit stage of the dedicated script with a recorded deferral, not an env bypass.

## UNCONFIRMED list

- Rest of `install_upstreams.sh` (read lines 1-80 only) and `strings.sh`; whether recipes mutate remotes beyond adding them.
- Behaviour on a real diverged remote (never executed).
- History of `env.properties` for past secret values.
- `claude --headless` flag validity at pre-push-gate.sh:138.
