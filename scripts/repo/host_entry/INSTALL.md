# Installing the host entry point cpa-host (owner procedure)

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T01:00:00Z |
| Status | draft by T042 (WP-04, slice 9); NOT performed: installing the entry point and approving the first manifest is the owner checkpoint T046a (constitution 11.4.66); no agent runs any step below |
| Status summary | install steps and the state layout; the `--owner-trust` operations that create and change the trust file are NOT built in this revision |
| Source | `scripts/repo/host_entry/cpa-host`, `docs/scripts/commit-push-all.md` |
| Issues | `cpa-host --owner-trust approve|retire|revoke|reanchor` answers 20 `owner_op_unimplemented` until T046a builds them; until then the trust file can only be written by hand, which this procedure does NOT recommend |
| Fixed | revision 2: the run-id grammar, an unexecutable approved core is 20 (never 126), a relative `--paths-from` is read from the directory cpa-host was started in (WF11 review F9, F10, F12) |

This document is the target of every refusal that `cpa-host` and the copied `commit-push-all.sh` print (`see: scripts/repo/host_entry/INSTALL.md`).

## What is installed and where

| Item | Default location | Mode | Owner |
|---|---|---|---|
| the entry point | `$CPA_HOST_ENTRY` (default `$HOME/.local/bin/cpa-host`), a copy of the reviewed `scripts/repo/host_entry/cpa-host` | 0755 | the owner only |
| the state directory | `$CPA_HOST_STATE` (default `$HOME/.local/state/cpa-host/`), outside every repository | 0700 | the owner only |
| the trust file | `$CPA_HOST_STATE/trust.json` (`cpa-host-trust/1`) | 0600 | changed only by `cpa-host --owner-trust` |
| the store | `$CPA_HOST_STATE/blobs/<sha256>`, approved file copies | 0400 | append-only |

## Steps

1. Prerequisites, checked with `command -v`: `git`, `jq`, `sha256sum`, `realpath`, `python3`. A missing one stops the install.
2. Read the file you are about to install: `less scripts/repo/host_entry/cpa-host` (it holds no review logic and writes nothing under a repository).
3. `install -d -m 0700 "${CPA_HOST_STATE:-$HOME/.local/state/cpa-host}"` and `install -m 0755 scripts/repo/host_entry/cpa-host "${CPA_HOST_ENTRY:-$HOME/.local/bin/cpa-host}"`.
4. Compare: `sha256sum "${CPA_HOST_ENTRY:-$HOME/.local/bin/cpa-host}" scripts/repo/host_entry/cpa-host` prints one hash twice. `cpa-host --show` prints the installed file's own sha256 and the project entry (read-only); a measurement of the installed file always uses `sha256sum` on the file, never the hash the file reports about itself.
5. The first approval (the adoption) and every later one are made with `cpa-host --owner-trust approve <verdict> [--commit <sha>]` at a terminal (the confirmation is read from `/dev/tty`). NOT BUILT in this revision (T046a).

## Rules (the conductor rule, T042)

No agent, stream or task installs or edits the installed copy or the state directory, invokes `cpa-host --owner-trust`, or writes a run directory's `released/`; against the
owner's real state an agent may run only the read-only modes (`--show`). Test fixtures use their own `CPA_HOST_STATE` and `CPA_HOST_ENTRY` under a temporary directory.
