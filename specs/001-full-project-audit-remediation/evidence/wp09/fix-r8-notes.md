# WP-09 fix round 8 - notes (review WP-09-T010 R-1..R-9)

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-08 |
| Last modified | 2026-10-08T15:40:00Z |
| Status | produced by the fixer; independent re-review owed (constitution 11.4.142), not claimed GO |
| Review | `evidence/reviews/WP-09-T010.json` (NO-GO: R-1..R-4 important, R-5..R-9 minor) |

| Finding | Fix | Test-first record (RED) | Paired mutations |
|---|---|---|---|
| R-1 `--out` set | `run_pinned.sh`: closed set; in/above tree only strictly below `.audit/out`, `.audit/scratch`, `evidence/wpNN*/.../smoke-out`; outside tree only strictly below `/tmp`, `/var/tmp` (TMPDIR only with RUNP_TEST_MODE=1) | `runp-red.txt` | out-allowlist, out-outside, x-out-root-equal, x-out-evidence-wide, x-out-reviews-excl, x-out-tmp-any, x-out-tmp-equal, x-out-tmpdir-ambient |
| R-2 env | `disk_headroom.sh`: CONF / REPO_ROOT (non-own) / CPA_HOST_ENTRY refused rc 2, EV ignored, outside `DISK_HEADROOM_TEST_MODE=1`; `run_pinned.sh` scrubs the five variables; both scripts' env reads classified and enforced by a suite | `dh-red.txt`, `runp-red.txt` | ov-gate-loose, ov-conf-dropped, ov-root-dropped, ov-root-always-refused, ov-host-dropped, ov-ev-dropped, x-scrub-* (5) |
| R-3 argv | exact golden of the whole argv (4 shapes, print mode + shim) | golden passed before the fix by construction (current argv is correct); the survival of reviewer mutants is the RED, see mutants | x-argv-privileged, x-argv-capadd, x-argv-label-disable, x-argv-net-host, x-argv-root-mount |
| R-4 smoke | re-run with the current launcher sha on the local images (no pull, no build) | n/a (evidence) | `fix-r8-smoke/` |
| R-5 | repeated `--need` / `--op-id` is usage error rc 2 | `runp-red.txt` | x-dup-need, x-dup-opid |
| R-6 | `probe_host.sh`: v1 only on a real controller file, a bare directory is `error` | `probe-red.txt` | cgroup-dir-as-v1, cgroup-v1-never, cgroup-v1-dir-counts, cgroup-missing-root-as-v1 |
| R-8 | unset or empty HOME (no XDG_STATE_HOME): `REFUSED reason=state_dir_unresolvable` | `runp-red.txt` | x-home-unset, x-home-empty |
| R-7 | process-doc, recorded not fixed: `disk_headroom.conf` keeps its provisional 6086120270-byte stand-in and `build_scratch_peak_bytes = 0`; re-deriving it from the lock sizes (largest WP-09 image IMG-GO 832636324) needs a dated measurement by the owner of T006; no value invented here | - | - |
| R-9 | process-doc, recorded not fixed: F-G, F-E (host_checklist CHK_* overrides), the stale "(round 4)" title of `wp09/README.md` (F-H) and the T005/T010 verdict files stay open | - | - |

Honest boundaries (11.4.6):

* The R-2 classification is by mechanical enumeration of default-expansion reads (`${NAME:-`, `${NAME-`, `${NAME:+`, `${NAME+`); a variable read in another form (a bare `$NAME`) would not be listed by the instrument. `DISK_HEADROOM_OUT_DIR` (documented CPA input) skips the commit-turn freeze by design; it is classified documented-safe because it never lifts the floor, not because it is inert.
* `scripts/build/tests/test_dispatch.sh` s23 sets `DISK_HEADROOM_TEST_MODE=1` (its `DISK_HEADROOM_CONF` is a test override); s23 alone was re-run (3 pass, 0 fail), the rest of that suite was not.
* R-4 evidence is a real `podman run` per image through the current launcher; it shows the four local images run and the controls behave, not that the lock entries are the right images.
