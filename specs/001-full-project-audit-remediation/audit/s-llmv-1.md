# S-LLMV-1: history and upstream of `submodules/llms_verifier` (T256, input to ODG-14)

| Field | Value |
|---|---|
| Revision | 1 |
| Last modified | 2026-10-08T10:45:00Z |
| Date (UTC) | 2026-10-08 |
| Task | T256 (WP-34), recorded as FACT for owner item ODG-14 ("investigate history first") |
| Superproject HEAD read | `b171ec79` |
| Upstream read | `git@github.com:vasic-digital/LLMsVerifier.git` branch `main` at `8f2090acb907d5c76e78d7c311378958e0ccbc40` (depth-1 fetch into a scratch repository; GitLab mirror `git@gitlab.com:vasic-digital/LLMsVerifier.git` answers `ls-remote` with the same `main` SHA) |
| Author statement | produced by a worker, not independently reviewed. Network: read-only `git ls-remote` and `git fetch`, nothing pushed |

## FACTS (each with the command that produced it)

1. **`submodules/llms_verifier` is a plain directory, never a git submodule.** `git ls-tree HEAD submodules/llms_verifier`
   gives mode `040000 tree e3ce15fe...`; `.gitmodules` has no entry for `llms_verifier` or `LLMsVerifier`
   (`git log -S"llms_verifier" -- .gitmodules` and `-S"LLMsVerifier"` both print nothing); no commit touching either path
   carries a `160000` (gitlink) entry. Control: the identical scan (`git log --all --raw --format=%h -- <path> | grep 160000`) on `submodules/helix_memory`
   prints `:000000 160000 ... A submodules/helix_memory`, so the scan can see a gitlink addition.
2. **It is a rename of the root directory `LLMsVerifier/`, done by one commit.** `05e0a2b2` (2026-06-23, "feat+fix:
   helix_memory submodule, LLMsVerifier relocate, anti-bluff scanner, wontfix rationale corrections"), a 42-file change with
   100 % similarity renames `LLMsVerifier/...` to `submodules/llms_verifier/...` (`git show -M --summary 05e0a2b2`).
   No commit has touched `submodules/llms_verifier` since (`git log 05e0a2b2..HEAD -- submodules/llms_verifier` prints
   nothing), so the content has been frozen since 2026-06-23.
3. **History of the root directory `LLMsVerifier/` (17 commits, 2026-03-22 to 2026-06-23, all by the superproject):**
   `00807341` (2026-03-22, first add, inside "feat(security): Add security tools installation script"), `510efef3`,
   `233eff34`, `c0f39d25` (03-30), `95a5a5d2`, `80d1d687`, `4a52ba17`, `b784f90d` (03-31, "add Upstreams directory for
   multi-remote push"), `7635665d` (03-31), `eb51a694`, `26d812a5` (04-04, 04-05), `6be28fa0` (04-07), `9fad0ffe` (04-08),
   `52371430` (04-13), `a90b504c` (04-14), `37ed4036` (04-22), `05e0a2b2` (06-23, the move).
   (`git log --format=... -- LLMsVerifier`; `git log --follow` is not usable on a directory and listed only the move.)
4. **The directory is a Catalogizer-specific subset, not a copy of the upstream repository.** It has 42 tracked files:
   `pkg/bridge` (3), `pkg/catalogizer` (2), `pkg/vision` (6), `docs`, `Upstreams/{GitHub,GitLab}.sh`, `go.mod`, `go.sum`,
   guides. `go.mod` says `module digital.vasic.llmsverifier`, `go 1.25`. `Upstreams/GitHub.sh` and `GitLab.sh` name the
   upstream `vasic-digital/LLMsVerifier` (`git@github.com:vasic-digital/LLMsVerifier.git`, `git@gitlab.com:...`).
5. **Upstream `main` does not contain these packages.** In the tree of `8f2090ac` (`git ls-tree -r`) there is no path matching
   `catalogizer`, `bridge/discovery` or `vision/strategy`; `pkg/bridge`, `pkg/catalogizer`, `pkg/vision` list 0 files. The same
   result for the branches `001-extend-llm-providers` and `feature/helixllm-full-extension` (0 matches each). Control of
   the same instrument: `go.mod` matched 3 paths in the fetched tree. So the 42 local files exist only in this repository: they
   would be lost if the directory were replaced by a clone of upstream.
6. **Upstream layout differs from the local one.** Upstream `main` has a root `go.mod` with `module llmsverifier` (go 1.25.3)
   and a second `llm-verifier/go.mod` with `module digital.vasic.llmsverifier`. The local directory has no `llm-verifier/`
   folder (`ls submodules/llms_verifier/llm-verifier`: no such file).
7. **A consumer already points at the upstream layout and it dangles.** `submodules/helix_qa/go.mod` line 131:
   `digital.vasic.llmsverifier => ../llms_verifier/llm-verifier` (and a require at line 137). The target
   `submodules/llms_verifier/llm-verifier` does not exist locally, so this `replace` cannot resolve against the current
   directory (UNCONFIRMED: whether `helix_qa` builds without it; not run, outside this task).
8. **Upstream is alive and ahead.** `main` is at `8f2090ac` (2026-09-25, "Merge origin/main (four-carrier lockstep) into main"),
   with other branches and tags (`helix_translate-2.3.0`, `-2.3.1`, `helix-code-1.0.0-dev-0.0.1`) on both hosts. GitHub
   additionally has `fix/verification-auth-cohere-metrics-modelid`.

## What this means for ODG-14 (options: real submodule; in-tree code; drop the `helix_qa` replace)

Stated as consequences of the facts, not as a decision (the decision is the owner's):

* "Convert to a real submodule of upstream" (batch-3 wish: always track latest `main`) loses the three Catalogizer packages
  (fact 5) unless they are first contributed upstream or kept in a separate Catalogizer-owned module; and the module path
  and layout differ (fact 6), so `helix_qa`'s replace should point at `llm-verifier` of the new checkout (fact 7), which
  already matches the upstream layout.
* "Keep in-tree" is the status quo: frozen since 2026-06-23 (fact 2), a subset of an actively developed upstream (fact 8).
* "Drop the `helix_qa` replace" is only safe after something provides `digital.vasic.llmsverifier` (fact 7).
* Who imports the three local packages inside this repository was NOT searched here: UNCONFIRMED (needed before any
  removal, section 11.4.124).

## Command log (re-runnable)

```
git ls-tree HEAD submodules/llms_verifier
git log -S"llms_verifier" -- .gitmodules ; git log -S"LLMsVerifier" -- .gitmodules
git show -M --summary 05e0a2b2
git log --format='%h %ad %s' --date=short -- LLMsVerifier
git log 05e0a2b2..HEAD -- submodules/llms_verifier
git ls-remote --heads --tags git@github.com:vasic-digital/LLMsVerifier.git
git fetch --depth 1 git@github.com:vasic-digital/LLMsVerifier.git main   # in an empty scratch repository
git ls-tree -r --name-only FETCH_HEAD | grep -iE "catalogizer|bridge/discovery|vision/strategy"
```
