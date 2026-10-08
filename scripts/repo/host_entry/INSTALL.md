# Installing the host entry point cpa-host (owner procedure)

| Field | Value |
|---|---|
| Revision | 4 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T19:00:00Z |
| Status | draft by T042 (WP-04, slice 9); NOT performed: installing the entry point and approving the first manifest is the owner checkpoint T046a (constitution 11.4.66); no agent runs any step below |
| Status summary | install steps and the state layout; the `--owner-trust` operations that create and change the trust file are NOT built in this revision |
| Source | `scripts/repo/host_entry/cpa-host`, `docs/scripts/commit-push-all.md` |
| Issues | `cpa-host --owner-trust approve|retire|revoke|reanchor` answers 20 `owner_op_unimplemented` until T046a builds them; until then the trust file can only be written by hand, which this procedure does NOT recommend |
| Fixed | revision 4 (the consolidated review of revision 3, round 4 of the 11.4.276 budget): the environment is an ALLOWLIST behind `#!/bin/bash -p` (an imported function, `SHELLOPTS`, `BASHOPTS`, `BASH_ENV`, `TMOUT`, `PYTHONPATH`, `LD_PRELOAD` and every other name are dropped; `HOME`, `CPA_HOST_STATE`, `TMPDIR` and every `PATH` element must be absolute), `BASH_ENV` runs NOWHERE (revision 3 said it ran once in the entry process), the released copy is read-only, a run directory is born with its `run.pid`; revision 3 (WF14 review round 2): the caller's environment is dropped as a class, not by name (every exported `GIT_*` variable but the commit identity, the `LONGOPS_*`, `ANTIMESS_*`, `AM_*`, `VERIFY_*` and `DISK_HEADROOM_*` families, `XDG_CONFIG_HOME`, `BASH_ENV`, `ENV`, `CDPATH`), every catchable terminating signal and every shell error ends the host entry with 20, and `HOME` and `CPA_HOST_STATE` both unset is 20 `state_unresolved`; revision 2: the run-id grammar, an unexecutable approved core is 20 (never 126), a relative `--paths-from` is read from the directory cpa-host was started in (WF11 review F9, F10, F12) |

This document is the target of every refusal that `cpa-host` and the copied `commit-push-all.sh` print (`see: scripts/repo/host_entry/INSTALL.md`).

## What is installed and where

| Item | Default location | Mode | Owner |
|---|---|---|---|
| the entry point | `$CPA_HOST_ENTRY` (default `$HOME/.local/bin/cpa-host`), a copy of the reviewed `scripts/repo/host_entry/cpa-host` | 0755 | the owner only |
| the state directory | `$CPA_HOST_STATE` (default `$HOME/.local/state/cpa-host/`), outside every repository | 0700 | the owner only |
| the trust file | `$CPA_HOST_STATE/trust.json` (`cpa-host-trust/1`) | 0600 | changed only by `cpa-host --owner-trust` |
| the store | `$CPA_HOST_STATE/blobs/<sha256>`, approved file copies | 0400 | append-only |

## Steps

1. Prerequisites, checked with `command -v`: `git`, `jq`, `sha256sum`, `realpath`, `python3` (with `jsonschema`: without it the release of a held commit is refused 20 `tool_absent`, never decided on a reduced check), and `/bin/bash` and `/usr/bin/env` at those absolute paths (the interpreter line is `#!/bin/bash -p` and the environment re-exec uses `/usr/bin/env -i`). A missing one stops the install.
2. Read the file you are about to install: `less scripts/repo/host_entry/cpa-host` (it holds no review logic and writes nothing under a repository).
3. `install -d -m 0700 "${CPA_HOST_STATE:-$HOME/.local/state/cpa-host}"` and `install -m 0755 scripts/repo/host_entry/cpa-host "${CPA_HOST_ENTRY:-$HOME/.local/bin/cpa-host}"`.
4. Compare: `sha256sum "${CPA_HOST_ENTRY:-$HOME/.local/bin/cpa-host}" scripts/repo/host_entry/cpa-host` prints one hash twice. `cpa-host --show` prints the installed file's own sha256 and the project entry (read-only); a measurement of the installed file always uses `sha256sum` on the file, never the hash the file reports about itself.
5. The first approval (the adoption) and every later one are made with `cpa-host --owner-trust approve <verdict> [--commit <sha>]` at a terminal (the confirmation is read from `/dev/tty`). NOT BUILT in this revision (T046a).

## The environment of a run

A run is steered by what the owner approved, through an ALLOWLIST. `cpa-host` and the copied script start with `#!/bin/bash -p` (bash then ignores imported functions, `SHELLOPTS`, `BASHOPTS`, `BASH_ENV` and `ENV`), install their signal handlers, and
re-exec themselves once through `/usr/bin/env -i` when ANY exported name is outside `HOME PATH TMPDIR CPA_HOST_STATE SKIP_LONG SSH_AUTH_SOCK GIT_AUTHOR_NAME GIT_AUTHOR_EMAIL GIT_COMMITTER_NAME GIT_COMMITTER_EMAIL LONGOPS_ALLOW_TMPFS LC_ALL PWD SHLVL`
(and the three values of a run, `CPA_ROOT CPA_RUN_ID CPA_ADOPTION_COMMIT`). Everything else is dropped (the one thing a script cannot drop is what the caller's loader did BEFORE its first line: a library named by `LD_PRELOAD` is loaded into the `cpa-host` process and into `env` that re-executes it, and into no process after that; matrix 18.7 asserts exactly those two): `GIT_*` (config injection, directories, `GIT_EXEC_PATH`, `GIT_SSH_COMMAND`), `PYTHON*`, `LD_*`, `TMOUT`, `XDG_CONFIG_HOME`, `LANG` and the `LC_*` but `LC_ALL`,
and the `LONGOPS_*`, `ANTIMESS_*`, `AM_*`, `VERIFY_*` and `DISK_HEADROOM_*` families. `LC_ALL=C` is exported. The git settings of a run come from the owner's `$HOME/.gitconfig` (a deploy key's `core.sshCommand`, a `url.<base>.insteadOf` belong there, not in the environment) and
from the repository's own configuration, which must hold only the keys of the approved table `scripts/repo/repo_config_allow.tsv` (a key that names a program or rewrites a url refuses the run with 20 `repo_config_not_approved`).
`HOME`, `CPA_HOST_STATE` and `TMPDIR` must be ABSOLUTE and every `PATH` element absolute and outside the repository you stand in (20 `env_value_unsafe`). `SSH_AUTH_SOCK` is passed because the SSH pushes of 2.1 need the owner's agent. `HOME`, `PATH`, `TMPDIR`, `CPA_HOST_STATE`, `SKIP_LONG` and
`SSH_AUTH_SOCK` are the owner's own process environment: the trust root lives under `CPA_HOST_STATE`, so it is the boundary, not a leak. `BASH_ENV` and the like run NOWHERE.

Ownership: a repository may be pushed only when EVERY url and push url of EVERY remote names an organisation of `scripts/audit/own_orgs.txt` (the main repository included). The real checkout has push URLs under `milos85vasic`: add that organisation to `own_orgs.txt`
(or remove those push URLs) before the first real run, else S0 refuses with 20 `repo_not_owned` (UNCONFIRMED against a live run; the audit area owns that file).

The released copy of a run (`.audit/commit-push/<run id>/released/`) is READ-ONLY (files 0444/0555, directories 0555): remove an old run directory with `chmod -R u+w .audit/commit-push/<run id> && rm -rf .audit/commit-push/<run id>`.

## Rules (the conductor rule, T042)

No agent, stream or task installs or edits the installed copy or the state directory, invokes `cpa-host --owner-trust`, or writes a run directory's `released/`; against the
owner's real state an agent may run only the read-only modes (`--show`). Test fixtures use their own `CPA_HOST_STATE` and `CPA_HOST_ENTRY` under a temporary directory.
