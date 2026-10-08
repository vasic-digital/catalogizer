# up.sh (test infrastructure) - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T16:40:16Z |
| Status | WF17 fix round 5 applied in the working tree (not committed); the independent review of that round is owed (constitution 11.4.142 / 11.4.209); evidence: `specs/001-full-project-audit-remediation/evidence/wp12/wf17/` |
| Source | `scripts/test-infra/up.sh`; tests `tests/infra/test_up_down.sh`, `tests/infra/test_concurrency.sh` |

## Purpose

Starts ONE test-infrastructure project `catalogizer-test-<build_id>`: per-project single-owner lease (11.4.119, registered long operation 11.4.232), per-run credentials and ports
(`gen_env.sh`), the deterministic corpus (`seed_corpus.sh` through `TIC tooling unit`), `podman-compose up -d`, then a bounded wait until every started service answers its protocol-level
probe (`probe.sh`). Rootless; images are digest-pinned and never pulled.

## Usage

```bash
scripts/test-infra/up.sh --build-id <id> [--services postgres,redis,ftp,smb,webdav,nfs] [--seed STR] [--timeout S] [--ev-dir DIR]
```

`nfs` adds `docker-compose.test-infra.nfs.yml` and builds the user-space NFS image (`nfs_build.sh`). `minio` is refused (BLOCKED, no obtainable image). `--ev-dir` receives the non-secret
`<project>.ports.env` and `<project>.corpus.sha256`.

## Lease

The purpose key is the project name. A live holder gives exit 3 `lease_held` (a second owner of the SAME project is refused; a DIFFERENT project is admitted at the same time); a dead holder gives
exit 4 `lease_stale` (never taken over silently: `scripts/longops/reap.sh --purpose <project>` decides). The holder is a keeper process that lives until `down.sh`; the registered operation is
`<project>-up-<UTC time>-<pid>` (`TI_OP_ID` of the env file); every service carries the labels `project=catalogizer`, `op_id`, `catalogizer.op_id`, `catalogizer.test_project=<project>`.

## Exits

0 up and every probe passing; 3 lease held; 4 stale lease; 1 failure (the partial project is torn down by label); 2 usage. TIC refusals that are transient on this shared host (`anti_mess_drift`,
`limit_exceeds_envelope`) are retried up to `TI_TIC_RETRIES` times (default 400, 5 s apart; the count is printed).

## WF12 review fixes (revision 2)

- Containers start WITHOUT a podman pod (`podman-compose --in-pod false`): the default pod was unlabelled, so the label-scoped teardown never removed it and every cycle leaked one (F2).
- A host port taken between `gen_env.sh`'s draw and the bind is retried: up to 3 attempts, each with fresh ports and credentials, the retry announced on stderr (F16).
- The corpus cache key now includes the digest of the image that builds the corpus, and the cache is RE-VERIFIED on every start (its digest recomputed from the files; a mismatch is refused, the printed `corpus_sha256` is the recomputed one) (F15). `TI_CORPUS_CACHE_DIR` relocates the cache (a test hook).
- A failed start tears down as ITS OWN owner (`down.sh --op-id <this start>`), so an earlier start's kept state cannot be mistaken for it (F17).
- TIC retries default to 400 (about 33 minutes at 5 s), not 90.

## WF17 fix round 5 (revision 3)

- Lifecycle contract: the registered operation carries an explicit no-progress budget (`TI_OP_BUDGET_S`, default 3600 s); its keeper (`ti-lease-keeper`) heartbeats ONLY while a container labelled with the operation is running, so a live stack is never `hung` and a dead one is not kept alive by its own keeper. Every container carries `catalogizer.op_id`, `catalogizer.test_project` and `catalogizer.test_root` (the first 16 hex of the sha256 of this checkout's real path); the compose files declare `x-podman: {in_pod: false}` so the project never gets a pod even when the file is used without `--in-pod`.
- Ownership: resources of the project that are not this checkout's (another root, a partial label set, an operation that is not in THIS registry) are REFUSED, exit 3 `foreign_owner`, and left alone; owned leftovers of an earlier run give `project_not_clean`; a non-terminal operation of the project whose owner is proven dead is closed `reaped` before registering. One per-user project lock serialises start and teardown.
- Teardown: an EXIT trap tears down what this start created on any failure and on INT, TERM and HUP (`ti_exit_on_signals` converts the three signals to a normal exit: a non-interactive bash killed by SIGHUP otherwise runs NO EXIT trap, measured). The operation id is persisted in `<state>/op_id` (mode 0600) and printed as `test-infra: registered op_id=<id>`. The corpus cache is installed with one `mv -T` (the loser of a race removes its temporary tree instead of nesting it). Before `exit 0` the lease is verified (claim, holder pid, live keeper).
- Inputs: valued options need a value (a trailing `--build-id` no longer spins); `--timeout` is a positive integer without a leading zero; the environment is scrubbed for compose (`ti_compose`: the caller's `TI_*` / `COMPOSE_*` never beat the env file); test hooks (`TI_COMPOSE_FILE`, `TI_CORPUS_CACHE_DIR`, `TI_TEST_SLEEP_*`, ...) are honoured only with `TI_TEST_MODE=1`.
