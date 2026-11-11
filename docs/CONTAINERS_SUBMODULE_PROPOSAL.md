# Proposal for the Containers submodule (upstream `digital.vasic.containers`)
Raised by CAT-QA-STACK-LIFECYCLE-1, 2026-11-11. Nothing here is implemented; `submodules/containers` was not modified.

## What it can do today (read at gitlink 22b49707, containers-2.0.0 line)
- `pkg/runtime`: `ContainerRuntime` (Start/Stop/Remove/Status/List/Stats/Logs/Exec...), `PodmanRuntime` driving the `podman` CLI through a `CommandExecutor`.
  `Status()` returns state + `Health` string from `podman inspect`. Used by `docs/qa/lib/ctrctl` for start/stop/status.
- `pkg/compose`, `pkg/boot` (BootManager), `pkg/health`, `pkg/lifecycle`, `pkg/distribution`/`scheduler`/`remote` (SSH multi-host), `cmd/deploy-stack`, `cmd/distributed-build`.
## Gaps found
1. **No environment hook on the executor.** `defaultExecutor` runs `exec.CommandContext` with the process environment; there is no per-runtime env
   overlay. Proposal: `runtime.WithEnv(map)` / an `EnvNormalizer` applied to every podman invocation, with a built-in rootless-podman normaliser that
   strips `,key=val` params from `DBUS_SESSION_BUS_ADDRESS` (crun sd-bus failure) and defaults `XDG_RUNTIME_DIR`. Then callers need no shell wrapper.
2. **No wait-for-health primitive on `ContainerRuntime`.** Proposal: `WaitHealthy(ctx, id, timeout)` built on `Status().Health` (treat "none" as running-ok, fail fast on exited/dead).
3. **No pod / run-spec / image-build API for Podman** (`pod create --network pasta`, `run --pod`, `-e NAME` by-name secrets, `--restart`, `build` under a memory cap). The stack-3 `up.sh`/`build.sh` need these; hence they still call podman directly.
4. **No guarded name-scope option** (refuse ids outside a prefix) - currently each consumer re-implements it (`QA_STACK_PREFIX`).
5. A reusable `cmd/` lifecycle CLI (`start|stop|status|wait`) so consumers need not carry their own `ctrctl`.
