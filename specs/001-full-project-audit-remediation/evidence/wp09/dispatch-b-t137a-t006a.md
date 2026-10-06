# T137a and T006a: status of the bootstrap host list and the build-host readiness (written with the dispatcher slice)

Revision 1, 2026-10-06. Independent review owed (11.4.142).

## T137a (bootstrap host list)
- DONE, not blocked: `build/hosts.env.example` (tracked path allowed by `.gitignore` lines `!/build/hosts.env.example`; variable names and meaning only: host `BUILD_HOST_<n>`, role, capacity, pinned
  fingerprint `BUILD_HOST_<n>_FP`, `BUILD_SSH_IDENTITY` key FILE outside the checkout, mode 0600), and `scripts/build/dispatch.sh` reads its host list only from `build/hosts.env` and refuses a submit when
  the file is absent or names no host (`REFUSED reason=no_qualified_host`; tests s5, mutant m_hosts_absent).
- BLOCKED on owner input, recorded honestly, NOT done: the host's own `build/hosts.env` for the bootstrap build host (the host recorded by T100 and confirmed at HC-1b, T102, ODG-07) and its host-key
  fingerprint from `docs/infrastructure/build-hosts.md`; hence `$EV/wp09/hosts-env.json` (file mode and fingerprint match) was NOT produced. No placeholder host was invented.

## T006a (bootstrap readiness of the build host) - BLOCKED-ON ODG-07
- Not run: no host is qualified or named (depends on T100, T102, T137a real file, T005c, T005d, T099). `$EV/wp09/build-host-bootstrap.json` was NOT written; no probe result was fabricated.
- What exists toward it: the dispatcher's submit-time qualification (reachable + rootless podman, recorded per attempt), the shipped-emitter path, the digest-verified bring-back and the once-only callback, all exercised
  against an ssh shim (test s15) and, for the container build itself, locally under the declared exception (`test_dispatch_e2e.sh`). The supervisor-restart leg (hub unit restart between submit and completion) needs the
  hub and its unit (owed), only its pump-restart analogue is tested (s19).
- While no host qualifies every WP-09 build step is `blocked-unavailable` (`no_qualified_host`) and P0 cannot exit; that is the intended state until the owner answers ODG-07.
