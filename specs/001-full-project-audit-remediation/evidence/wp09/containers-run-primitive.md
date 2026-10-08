# T002: reuse check for a one-shot run primitive in `submodules/containers`

| Field | Value |
|---|---|
| Revision | 1 |
| Last modified | 2026-10-08T10:25:00Z |
| Date (UTC) | 2026-10-08 |
| Task | T002 (WP-09, read-only reuse check, section 11.4.74, 11.4.76, 11.4.161) |
| Subject | `submodules/containers` at commit `4a8f04e05f3535d77c48f69b89687f9fcc896fbf` (the commit recorded in the superproject index: `git ls-tree HEAD submodules/containers`) |
| Method | read of the files listed below; two greps (limit and digest vocabulary) each with a positive control; no file of the submodule was modified |
| Author statement | produced by a worker, not independently reviewed |

## Verdict

**The primitive EXISTS; its typed surface does NOT cover what `run_pinned.sh` needs.** FACT, from `pkg/runtime/run.go`
and `pkg/runtime/runtime.go`:

* `ContainerRuntime.Run(ctx, image, cmd, opts ...RunOption) (*ExecResult, error)` is a one-shot `run --rm` in one call
  (runtime.go lines 40-52). `PodmanRuntime.Run` (podman.go line 361) delegates to `runViaCLI`.
* Typed options present: `WithRunStdin`, `WithRunRemove` (default true), `WithRunName`, `WithRunEntrypoint`,
  `WithRunWorkDir`, `WithRunUser`, `WithRunNetwork`, `WithRunEnv`, `WithRunVolumes` (raw `-v` strings, so a read-only
  source mount is expressible as `src:/src:ro`), `WithRunExtraArgs` (raw argv appended before the image).
* Typed options ABSENT: memory, memory-swap, pids-limit, cpus, labels, security options, a cgroup parent, a
  digest requirement. `buildRunArgs` never emits `--memory`, `--pids-limit`, `--cpus` or `--label`; the only route is
  `WithRunExtraArgs`, i.e. the caller writes the flags as free strings, which is what `run_pinned.sh` does today with
  more validation than a pass-through would give.
* The image argument is an unvalidated string: no check that it carries `@sha256:<64 hex>`, no lock-file lookup.
* `ResolveRunSpec` (run.go, exported) returns the composed argv; `pkg/remote/runtime.go` line 239 uses it for the SSH
  runtime, so a remote one-shot run exists too but refuses stdin (`ErrStdinUnsupported`).
* Other packages read: `pkg/boot` (`manager.go`, `options.go`) and `pkg/compose` (`orchestrator.go`, `options.go`)
  manage long-lived service groups and contain no one-shot run with limits; `pkg/remoteexec/remoteexec.go` runs shell
  commands over SSH (`r.Run(ctx, "<shell>")`), not container runs; `cmd/distributed-test/main.go` carries
  `MemoryMB`/`CPUCores` only as scheduler requests (`pkg/scheduler`), not as `podman run` flags.

## Consequence for T007 and the conventions

1. T007 cannot wrap the Go primitive and keep the limits typed: the wrap would still be `WithRunExtraArgs("--memory", ...)`,
   an untyped pass-through, plus a Go binary the shell launcher does not have.
2. Direct `podman run` in `scripts/containers/run_pinned.sh` therefore stays **tracked deviation (b)** of the P0-P1
   conventions. The wording in the launcher header ("the Go run primitive ... has no typed memory/pids/label options and no
   digest validation") is confirmed by this note, with one correction of precision: the primitive is present, `--rm`,
   user, network, env, volumes and raw extra arguments are typed or expressible; only the limit, label and digest
   vocabulary is absent.
3. Upstream proposal (for T139, section 11.4.74 extend-don't-reimplement): add typed `WithRunMemory`, `WithRunMemorySwap`,
   `WithRunPidsLimit`, `WithRunCPUs`, `WithRunLabels`, `WithRunReadOnlyRoot` and a `RequireDigest` image check to
   `pkg/runtime/run.go`, each with a RED test beside `TestRun_OptionsMapToFlags` (run_test.go line 183). Not done here
   (read-only task, and a submodule edit is outside the W6 write set).
4. The owner sees the deviation in the T013 request list (carried by the conductor, not by this note).

## Files read

`pkg/runtime/runtime.go`, `pkg/runtime/run.go`, `pkg/runtime/podman.go` (Run, lines 350-365), `pkg/runtime/types.go`,
`pkg/runtime/options.go`, `pkg/runtime/run_test.go` (test names only), `pkg/remote/runtime.go` (lines 225-260),
`pkg/boot/manager.go`, `pkg/boot/options.go`, `pkg/compose/orchestrator.go`, `pkg/compose/options.go`,
`pkg/remoteexec/remoteexec.go`, `cmd/distributed-test/main.go`, `pkg/scheduler/scheduler.go` and `scorer.go` (the
`MemoryMB` hits only).

## Measurement controls (section 11.4.273)

* Negative-looking result "no digest vocabulary in `pkg/runtime`": the grep
  `grep -rniE "digest|@sha256" pkg/runtime/*.go | grep -v _test` printed nothing. Positive control of the same path:
  `grep -c "WithRunExtraArgs" pkg/runtime/run.go` printed `2` (a token known present is found), so the empty result is
  not a blind instrument.
* Negative-looking result "no `--memory` / `--pids-limit` / `--cpus` / `--label` in the run argv builder": the grep over
  `run.go` for `pids|memory|cpus` printed nothing; the same file was shown in full above (`buildRunArgs`), which emits
  only `--rm`, `-i`, `--name`, `--entrypoint`, `-w`, `-u`, `--network`, `-e`, `-v`, the extra args and the image.
  The `--label` absence is read from `buildRunArgs` itself (the whole function was read), not from a grep.
* The `Labels` hits in `pkg/runtime/podman.go` (lines 130, 161-193) are the LIST filter and list result, not run options.
