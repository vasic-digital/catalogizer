# Owed requests from WP-23 review round 1 (WF11)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T00:58:05Z |
| Status | open; each item is for the owner named, not for WP-23 |

## OR-1: per-call remote proof for compile-class wrappers (owner: scripts/build, T005b emitter)
- Finding I8 of WF11: `scripts/containers/run_rust.sh` trusted the environment variable `RUNNER_REMOTE_CALL=1` and claimed the emitter sets it. Measured: `git grep RUNNER_REMOTE_CALL HEAD -- scripts/build/` finds 0 lines (control: `git grep -c emit` on `scripts/build/remote/emit.sh` finds 16).
- WP-23 fix (done, in scope): the variable is ignored; the wrapper starts only on a host with a root-owned `/etc/catalogizer/build-host` naming that host. That proves the HOST, not that a call came from the emitter.
- Owed by the emitter owner: mint a per-call proof the wrapper can verify without trusting the caller's environment, for example an HMAC of `(build_id, argv digest, host)` under the per-build key the driver already sends to `emit.sh` on stdin (the key never enters the environment, `emit.sh` header), delivered to the wrapper through an inherited file descriptor or a file in the build directory that only the emitter's uid can create. `run_rust.sh` would then verify the proof before `runner_main`. Until then a local user on a build host can start a Rust build by hand.
- Operator action (host change, outside agents): create `/etc/catalogizer/build-host` on each designated build host: `printf 'build-host %s\n' "$(hostname -s)" | sudo tee /etc/catalogizer/build-host; sudo chmod 644 /etc/catalogizer/build-host`.

## OR-2: register database for first-party exclusion items (owner: register go-live, T069)
- `coverage/exclusions/*.yaml` first-party entries need a tracked item that EXISTS. `docs/workable_items.db` does not exist yet, so `check_exclusions.py` has no register to verify against (`--items-file` takes an export). Until then the only verifiable first-party route is `measured_by: {app, lane}`.
- catalogizer-desktop and installer-wizard fences FAIL honestly for first-party files the tool config excludes (`src/main.tsx`, `*.config.*`, `src/vite-env.d.ts`): they need tracked items once the register exists.

## OR-3: the 2 percent memory margin of test-in-container.sh is not enough on a loaded host (owner: commit 96779242, WP-09/10 envelope)
- Measured during the B2 re-baseline (wp23/review-r1/b2/run-1.txt, first attempt): `run_kcov: REFUSED reason=limit_exceeds_envelope --memory 10998109353 is above the envelope memory 10956050228`. TIC asked for 98 percent of its own reading (11.22 GB), the wrapper read 10.96 GB moments later (a 2.4 percent fall of MemAvailable on a host at load average 11 to 14), so the request was above the wrapper's reading and was refused.
- WP-23 did not touch the margin (it is owned by 96779242). The re-baseline script retries only that exact refusal (each attempt is recorded in the run file). Owed by the owner: a margin that is not a fixed fraction (for example the wrapper honouring any request at or below its reading plus a tolerance, or TIC and the wrapper sharing one envelope read).
