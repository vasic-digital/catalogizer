# verify_repos.sh in the pinned images: root-cause analysis (kcov baseline finding F3)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06 |
| Status | evidence note of WP-04 fix round 4; the section 11.4.44 header was added in the round-4 cleanup pass |
| Status summary | root-cause analysis of verify_repos.sh behaviour in the pinned images (kcov baseline finding F3) |
| Issues | none recorded in this note |
| Issues summary | see the body below |
| Fixed | section 11.4.44 header added |
| Fixed summary | header only; the body is unchanged |
| Continuation | the open items named in the body, if any |

| Field | Value |
|---|---|
| Date (UTC) | 2026-10-06 |
| Method | systematic debugging, phases 1-3 only (read-only; no repo file edited, nothing staged or committed) |
| Repo HEAD | a27d72a55c99da68a36a3359d6334f84e62a16a2 |
| Raw evidence | `specs/001-full-project-audit-remediation/evidence/wp04/rootcause-raw/` (stdout/stderr/rc per run) |
| Run tool | `scripts/containers/run_pinned.sh --network=none` (read-only /src, --rw only `.audit/scratch` for the patched COPIES used as experiments) |
| Images | IMG-KCOV `sha256:34e6c267...ab164`, IMG-TESTUTIL `sha256:56b9be70...f09` (images.lock.yaml) |

## sha256 of the inputs (as read)

```
c5468681d6bb5807ea7bb3a627e6b9486d9ceaa366a08c58f51ec54bba4fef7f  scripts/repo/verify_repos.sh
a9ff7f367554583ffab3bed7e6a4152567537f54ee1cfa43f7d3885b2635b005  scripts/repo/tests/test_verify_repos.sh
8a0e6daa71cf64cf5160dd6b0eef844a3135a062be8866afb0edbd6339f48c40  scripts/containers/run_pinned.sh
d2e8fa9d23863776a53549b292d771d7b0b0de9902c53ef668f475268cf1a1c3  build/containers/images.lock.yaml
6949901c8b1d9112152dc5caed9f07961fa5634667e3adf73ed327c12c003670  build/containers/testutil/Containerfile
0f210a61b3723d654e7923afda6fbc31a8cb2c9a60686bb58ba895d7712b3d4f  build/containers/kcov/Containerfile
deafc57603558c821746813f2329f2f508d9a32f04159d965f19a68989c852f4  specs/001-full-project-audit-remediation/contracts/repo-verification-report.schema.json
fe7e42ea8c45ea61de432f2f0b3de87fd958c5c6cbab60b2d4217444f3043145  scripts/repo/tests/test_planned_paths_tracked.sh
2689a4bd7b6fe915c5a409e301df2cb21566c073c43d0e2f904007ded41af43a  scripts/repo/tests/test_verify_schema.sh
3babdf3db5a6b4416b96c548e29cc32bd599dfeaab747fa1bd8fbbacce7fabcc  scripts/repo/tests/test_record_deferral.sh
```

Raw output hashes (selected):

```
e63aaab1c4f822d6a519ae91fc2b5fb3c531672d950271c9edc371e8d5550692  rootcause-raw/vr-IMG-KCOV.out        (169 ok / 167 FAIL)
cfd6d5140018768a1510940d6fdd546c5cb7295af7fce2e8478a1122600c2a90  rootcause-raw/vr-IMG-TESTUTIL.out    (169 ok / 167 FAIL)
67c891887231a7558b3609fb22c448ace8fb6d71760a66a2b5634fb926a1c883  rootcause-raw/h3.out   (awk NUL fix only: 323/13)
941a6cc8365c670cb1d9746a4439d6ca108d4ca2818e82e59c52fc0a2beb4126  rootcause-raw/h5.out   (+ interval fix + gitconfig: 332/4)
db25e143747135ecfa517894dd754c4246b950a08dc5bd0f76c5a9b07e10dc5c  rootcause-raw/h6.out   (+ safe.directory=*: 336/0)
f31fce7e1c9e05a26bc510688022a106c078aff79a678ecca400be1bb8c46b5e  rootcause-raw/nr-vr.out (non-root uid 1000, stock: 169/167)
d3867175ad9f91eb8246bd15eab2fd1fd595fe1befb30c4c79b94d0f7cf9f920  rootcause-raw/suites/verify_schema.out
34005bf2f94efe19f55c4154c6abd50e03998bb549a700a6a701136f9e094e0e  rootcause-raw/suites/planned_paths_tracked.out
348a1940baed832f0a0ac9aa9611959678d48241b469d4c56453689656ed6900  rootcause-raw/awk-container.txt
419bb53901f0caa16874b04de33d8e3c7070a6df1f5f8fa72c4e17e54436a16e  rootcause-raw/mawk-probes-container.txt
e4c1824dbbd37dd16688bcb28cd686b94312c05a01f8b572f267bd1946632a03  rootcause-raw/jsonschema-id-probe-container.txt
12edf4e283584794c9beab28012823aa6f512bbb9ebc2a926eaa8c957bb1aef6  rootcause-raw/real-tree-container.txt
```

## 1. Reproduction (deterministic)

`run_pinned.sh --network=none --out <dir> IMG-KCOV -- bash scripts/repo/tests/test_verify_repos.sh` -> rc=1, `SUMMARY pass=169 fail=167`.
The same command with IMG-TESTUTIL gives the identical 169/167 (identical FAIL set; the only diff line is the `want` count of one
assertion that depends on whether `git submodule status` was readable). Plain bash with no kcov already fails, so kcov is not involved.
IMG-GO was not run: it has no git-fixture toolchain for this suite (golang image; IMG-TESTUTIL/IMG-KCOV are the test images). UNCONFIRMED for IMG-GO.

Tool facts read inside the container (`awk-container.txt`, `mawk-probes-container.txt`):

```
awk -> /usr/bin/mawk, mawk 1.3.4 20200120 (host: gawk 5.3.2; host bash 5.3.9, container bash 5.2.15)
printf '.\t.\n' | awk -F'\t' '{printf "%06d\0%s\0%s\0", NR, $1, ($2==" "?"=":$2)}' | od -c
  container: 0000000   0   0   0   0   0   1            <- output stops at the first \0
  host     : 0000000   0   0   0   0   0   1  \0   .  \0   .  \0
echo <64 hex> | awk '$0~/^[0-9a-f]{64}$/{print MATCH}'  -> container: no match; host: MATCH
awk 'BEGIN{RS="\0"}' on NUL-separated input                -> works in mawk (the other 6 NUL-RS awk uses are NOT a problem)
```

## 2. Root causes, ranked by number of failing assertions explained

Experiment ladder (each step adds ONE correction to a COPY of the verifier/test placed in `.audit/scratch`; repo files untouched):

| Step | Change | pass/fail |
|---|---|---|
| stock | none | 169 / 167 |
| h1 | `safe.directory=/src` only | 169 / 167 (verifier unsets GIT_CONFIG_*; env-based config is dropped, see 2.3) |
| h2 | `safe.directory=*` + identity (env) | 159 / 135 |
| h3 | RC1 fix only (NUL printf) | 323 / 13 |
| h4 | RC1 + safe.directory + identity (env) | 325 / 11 |
| h5 | RC1 + RC2 + `/tmp/.gitconfig` (safe.directory=/src, identity) | 332 / 4 |
| h6 | RC1 + RC2 + `/tmp/.gitconfig` (safe.directory=`*`, identity) | **336 / 0** (equals the host) |

### RC1 (about 150 of the 167 failures): mawk truncates `printf "...\0..."`. REAL portability defect in `verify_repos.sh`

- Where: `scripts/repo/verify_repos.sh:328`, the xargs feed `awk -F'\t' '{printf "%06d\0%s\0%s\0", NR, $1, ...}' "$LIST" | xargs -0 -n 3 ...`.
- Mechanism: in mawk the `\0` ends the printf format string, so only `000001` is emitted; xargs -0 then hands the worker `idx=000001 rel="" pin=""`.
  Evidence (container, fixture clean/dirty repo): `jq -c .repos` -> `[{"path":"","pin":"",...,"pin_state":"?",...}]`,
  `"pin_drift":1` (pin_state `?` is counted as drift: `verify_repos.sh:351`), so the root row has `"path":""` instead of `"."` and every
  `row <name> . ...` lookup returns '' (the first failure: `FAIL dirty row lists problem dirty (got '', want 'true')`). Submodule rows
  are lost entirely (exit 20 instead of 11/12/13/15 in all submodule and pair/class cases).
- This also explains the earlier leads "path is empty" and "pin_drift is 1 on a plain repo" (they are the same defect).
- Fix layer: `verify_repos.sh` (use `printf '%s\0'` from a bash loop, or `tr`/`while read`; do not put `\0` in an awk printf format).
  Alternative/defence: add `gawk` to the testutil image (apt list in `build/containers/testutil/Containerfile`); the image has only mawk.

### RC2 (6 failures: "excepted dirty row", exception listed/counted, B3 x2, m-b control): mawk has no `{n}` interval expressions. REAL portability defect in `verify_repos.sh` (+ one test line)

- Where: `verify_repos.sh:144` (`$4~/^[0-9a-f]{64}$/ && $5~/^[0-9a-f]{64}$/`) and `tests/test_verify_repos.sh:420` (same regex in the "real exceptions.tsv" row check; this is the failure `the real exceptions.tsv has a legacy or malformed row`).
- Evidence: mawk 1.3.4 20200120 does not match `^[0-9a-f]{64}$` against a 64-hex string (probe above), so a correct exception row never matches and a fully-excepted dirty file is reported as unexcepted (`FAIL excepted dirty row, plain: exit 0 (got '13', want '0')`). Fixing it in a copy (h5): these 6 pass.
- Fix layer: `verify_repos.sh` and the test (use `length($4)==64 && $4!~/[^0-9a-f]/`). Alternative: gawk in the image (same fix as RC1).

### RC3 (about 4 failures in the verify_repos suite; ALL 294 in `planned_paths_tracked`; 0 elsewhere because the other suites build their own fixtures): git "dubious ownership" of `/src` and of every submodule. CONTAINER ENVIRONMENT gap

- Evidence: `fatal: detected dubious ownership in repository at '/src'` (and at `/src/submodules/assets`); `git -C /src rev-parse --show-toplevel` fails inside the container as `uid=0(root)` against a tree owned by 1000:1000 (`stat -c "%u:%g" /src /src/.git` -> `1000:1000`). Consequences: the suites' first line `cd "$(git rev-parse --show-toplevel)"` silently stays in /src (empty `cd ""`, `head=` blank in IDENTITY), the real-tree run exits 20 ("not a git repository: /src"), and `test_planned_paths_tracked.sh` gets `git check-ignore` rc=128 (`FAIL planned tools/evidence/evrec rc=128 (want 1)` x294).
- Why uid 0: `podman run --userns=keep-id` is used but the image runs as root (`kcov/Containerfile` has `USER root`; the effective user is 0 in `id`, `groups=0(root),1000`). run_pinned.sh adds no `--user`.
- Proof that a non-root uid removes it: the composed run_pinned argv + `--user 1000:1000` (`nr-id.out`): `uid=1000(milosvasic)`, `git -C /src rev-parse --show-toplevel` -> `/src`, `git -C /src/submodules/assets rev-parse --show-toplevel` -> `/submodules/assets` ok, `dubious` count 0; `test_planned_paths_tracked.sh` -> `TOTAL planned=107 control=187 failures=0`.
- Env-var config does NOT work for the verifier (h1): `verify_repos.sh:56-57` deliberately unsets `GIT_CONFIG*`; use a gitconfig FILE (`$HOME/.gitconfig`, HOME=/tmp tmpfs) or non-root.
- Fix layer: `run_pinned.sh` (add `--user "$(id -u):$(id -g)"` next to `--userns=keep-id`; verify with the nr probe) as the preferred fix. Fallback: write `/tmp/.gitconfig` with `safe.directory = *` in the run wrapper (works, h6, but keeps root).

### RC4 (2 failures + noise on stderr: `symref upload` / `symref dash` "no refusal message"; `Author identity unknown`, `fatal: : not a valid SHA1`): git committer identity absent. Container environment gap that is also a TEST fixture defect

- Evidence: stderr `Author identity unknown` / `unable to auto-detect email address (got 'root@<id>.(none)')`; `fatal: : not a valid SHA1` is `git commit-tree` failing (identity) and returning an empty id used on the next line. Sites without `$G`: `test_verify_repos.sh:362` and `:451` (`git ... commit-tree ...`) and the pre-commit-hook case near `:530` (`git commit -a`). The host passes only because of the host's global `user.name/email`; HOME=/tmp in the container has no `.gitconfig`.
- h3 -> h4 shows the two symref failures disappear when identity is supplied.
- Fix layer: test fixtures (put identity in the `$G` prefix, or export `GIT_AUTHOR_*`/`GIT_COMMITTER_*` at the top of the test); or the run wrapper. The fixture fix is preferred because it makes the suite hermetic regardless of image/host. `test_commit_recursive`, `test_scope_check`, `test_push_recursive`, `test_wp04c` etc. already pass in the container, so they set identity themselves.

### RC5 (not a failing assertion, a latent portability defect): `column` command is absent in the image

- Evidence: `column: ABSENT` in the tool probe; `verify_repos.sh:378` pipes through `column -t -s $'\t'` in non-quiet mode and prints `verify_repos.sh: line 378: column: command not found` (reproduced in the diag run). The tests pass `--quiet`, so no assertion fails.
- Fix layer: `verify_repos.sh` (replace by awk/printf column padding or fall back when `column` is missing) or the testutil Containerfile (`bsdextrautils` provides `column` on bookworm). Script fix preferred (the script must also run on hosts without bsdextrautils).

### RC6 (NOT in the original list): the two other suites fail in the container too

The task said the other 13 suites match the host; measured (stock, IMG-KCOV, `rootcause-raw/suites/`):
11 pass (check_classes 131/0, check_no_ci 30/0, check_revision_headers 25/0, commit_recursive 55/0, fixture_roots 30/0, integrate_ff_only 80/0, integrate_merge 117/0, push_recursive 92/0, record_deferral 24/0, scope_check 50/0, validate_cheap 106/0, wp04c 230/0). Two do NOT:

1. `test_planned_paths_tracked.sh`: 0 ok / 294 FAIL, rc 128 - RC3 (dubious ownership). Passes non-root (`failures=0`).
2. `test_verify_schema.sh`: 6 ok / 8 FAIL. Evidence (`jsonschema-id-probe-container.txt`): `Draft202012Validator(schema).iter_errors(good.json)` raises
   `RefResolutionError: unknown url type: 'repo-verification-report/repo-verification-report/1'` in the image (`python3-jsonschema 4.10.3-1`), but returns 0 errors on the host (jsonschema 4.19.2). The schema `$id` is the RELATIVE string `repo-verification-report/1` (`contracts/repo-verification-report.schema.json` line 3) and 4.10 mis-joins it for the `#/$defs/...` refs; with an absolute `$id` (`https://example.invalid/repo-verification-report/1`) the container gives 0 errors. The venv `/opt/cpa-tools` has no jsonschema (`ModuleNotFoundError`); the test calls the system `python3`.
   Fix layer: image (pin jsonschema to the host-proven 4.19.x in `build/containers/testutil/requirements.txt` hash-pinned venv and run the tests with the venv python) OR the schema (an absolute `$id` is valid and version-proof; it is a contract file under specs, so a contracts revision + the owner decision). UNCONFIRMED which of the two the owner prefers; the image pin is the smaller blast radius.

### Dubious-ownership exposure of the other suites

All 14 suites print `fatal: detected dubious ownership in repository at '/src'` once (the opening `git rev-parse --show-toplevel`) but 11 stay green because they build their fixtures under `/tmp` and use relative paths from `cd ""` (= `/src`). That is a silent weakness (they run in /src only by accident of `cd ""`); with the RC3 fix it disappears (`dub=0` in the non-root runs).

## 3. Root-skips (record_deferral)

`test_record_deferral.sh:54` and `:58-59` skip three assertions when `id -u` is 0 (`ok   unwritable case skipped (root)`; the read-only-file case is a silent skip, 24 ok as root). As uid 1000 (run_pinned argv + `--user 1000:1000`) the same suite runs them: `---- 26 ok, 0 failed`. Proposal: run all containers as the mapped host uid (RC3 fix in `run_pinned.sh`); no test change needed.

## 4. Recommended fix per layer (nothing changed here)

| # | Cause | Fix layer | Change |
|---|---|---|---|
| RC1 | mawk NUL printf | `scripts/repo/verify_repos.sh:328` | feed xargs with a bash `while read` + `printf '%s\0'`; add a golden test with mawk (`awk=mawk` in the image) |
| RC2 | mawk `{64}` | `verify_repos.sh:144`, `tests/test_verify_repos.sh:420` | `length($4)==64 && $4!~/[^0-9a-f]/` |
| RC3 | dubious ownership as root | `scripts/containers/run_pinned.sh` | add `--user "$(id -u):$(id -g)"` (keep-id remains); re-verify with the nr probe |
| RC4 | no git identity | `tests/test_verify_repos.sh` fixtures | add identity to every plain git call (`commit-tree`, the hook `commit -a`) or export GIT_* identity once |
| RC5 | `column` missing | `verify_repos.sh:378` | awk/printf padding or `command -v column` guard (or `bsdextrautils` in testutil) |
| RC6a | planned_paths_tracked 294 | same as RC3 | none beyond RC3 |
| RC6b | verify_schema 8 | `build/containers/testutil/requirements.txt` + test python, or schema `$id` absolute | owner choice |
| defence | mawk in image | `build/containers/testutil/Containerfile` | optional `gawk` (fixes RC1+RC2 for all scripts at once but changes the image digest, relock, rebuild of IMG-KCOV) |

Order of work: RC3 (one line, unblocks 294+4 and root-skips) -> RC1 -> RC2 -> RC4 -> RC6b -> RC5.

## 5. Constraints and UNCONFIRMED

- No `pgrep -f`; long runs were backgrounded with DONE markers; evidence only under `rootcause-raw/`; scratch copies of the verifier/test were placed in `.audit/scratch` (gitignored `/.audit/`, not tracked) and are not part of the repo change set. No credentials used.
- UNCONFIRMED: IMG-GO behaviour (not run; no git-fixture suite uses it). UNCONFIRMED: whether `--user 1000:1000` combined with the read-only /src mount affects kcov (`/out` ownership) - the nr probes did not run kcov. UNCONFIRMED: that RC1-RC4 are the complete set on a host whose awk is not gawk and git config differs (only the two images and this host were exercised). UNCONFIRMED: the four-case `h2` numbers (159/135, an aborted/partially-run suite: total 294 not 336) - not analysed beyond "improves".
- Verified: all 167 stock failures are explained: after RC1+RC2+RC3(gitconfig)+RC4(identity) the suite is 336/0 (h6).
