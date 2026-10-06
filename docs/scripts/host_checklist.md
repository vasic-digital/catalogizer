# host_checklist.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-06T16:00:00Z |
| Status | tracked since commit 26755ca5; revision 2 re-aligned to the script after rounds 5 to 7 (independent review of this revision owed, constitution 11.4.142); indexed by `docs/scripts/README.md` |
| Source | `scripts/containers/host_checklist.sh`; test `scripts/containers/tests/test_host_checklist.sh` |

## Purpose

Runs the docs/16 section 9.5 host checklist (items 1 to 8) for the one host that exists. Owner decision 2026-10-06: the build and
measurement host is this host (anton) and there is no remote host. The checks that only mean something against a remote host are
therefore recorded `status=not_applicable_local_host`, `verdict=na`, with the decision cited in the item; they are never faked as `pass`
and never dropped. It complements `probe_host.sh` (which writes `$EV/host-probe.json`; `$EV` is the evidence root, default `specs/001-full-project-audit-remediation/evidence`) and does not edit it, and it reads the
absolute `min_free_bytes` rule of `disk_headroom.conf` without calling `disk_headroom.sh`. It uses no network and pulls nothing.

## Usage

```bash
scripts/containers/host_checklist.sh [--strict]
```

Writes `$EV/wp10/host-checklist.json` (schema `host-checklist/1`; default `EV` is `specs/001-full-project-audit-remediation/evidence`)
through a unique temp name, so a TERM/INT/HUP leaves no residue. Exit 0 whenever the record was written; with `--strict`, 1 when any
item has `verdict=fail`; 2 on usage or a missing `jq`; 3 when the record cannot be written.

## Checks

| Id | 9.5 item | What is read | Verdicts |
|---|---|---|---|
| C01 | 1 | SSH reachability and key login | `na` (remote only) |
| C02, C03 | 2 | `podman --version`; `podman info` rootless | pass, fail (error when unreadable) |
| C04 | 2 | cgroup v2 (`cgroup.controllers`) | pass, fail |
| C05, C06 | 2 | MemTotal and MemAvailable; `nproc` | pass, fail |
| C07 | 2 | `/dev/kvm` exists and is readable+writable by the user | pass, fail |
| C08 | 2 | `max_user_namespaces`, subuid and subgid ranges, `newuidmap` | pass, fail |
| C09 | 2 | `ulimit -u` (at least 4096) and `-n` | pass, fail |
| C10, C11 | 2 | free bytes under the podman graphroot and on the repository filesystem | pass, fail |
| C12 | 2 | both free values against `min_free_bytes` of `disk_headroom.conf` (need 0) | pass, fail |
| C13 | 3 | every image of `build/containers/images.lock.yaml` present locally and pinned (the lock `digest` must itself be present, as a RepoDigest `@<digest>` or as the stored `.Digest`; when the lock lists a `platform_digest` it must ALSO be present, and it never substitutes for the digest: round 7, F-A). The lock is read with python3 and PyYAML (the reader `run_pinned.sh` uses), which are required for C13; each entry shape is validated and every digest must be exactly `sha256:` plus 64 lowercase hex characters. Per-image statuses: `present_pinned`, `absent`, `digest_mismatch`, `lock_malformed`; whole-lock errors: `lock_unreadable`, `lock_reader_failed` | pass, fail (error when the lock cannot be read) |
| C14 | 3 | outbound pull policy | `unconfirmed` (no network use here) |
| C15 | 4 | instantaneous load only; whether other work shares the host is an owner fact | `unconfirmed` |
| C16 | 5 | `timedatectl` NTPSynchronized | pass, fail |
| C17, C18, C19 | 6, 7, 8 | remote roles, two-way artifact path, host-key pinning | `na` (remote only) |

Each item is `{id, section_9_5_item, status, verdict, detail, value?}`. Control needles (11.4.201(7)(b)): the same `-e` instrument must
see this script (positive) and not see a nonexistent path (negative), and the lock-file parser must return at least one image (an empty
parse fails C13, it is never read as "no images missing"). The record carries `identity` (sha256 of the script, the lock and the
disk-headroom conf) and a `summary` of counts.

## Test

`scripts/containers/tests/test_host_checklist.sh` drives the script with `podman`, `df`, `nproc` and `timedatectl` shims and scratch files
for every `/proc`, `/sys` and `/etc` input (baseline, usage and `--strict`, one broken input per check with a no-collateral assertion,
a real-host leg, and 35 paired mutations (`mut_case` rows, round 7 count: 31 plus `c13pdreq`, `c13pdalone`, `c13iso`, `c13digz`) each of which must change a fixture result). `HOST_CHECKLIST_SUT` points it at another script
(the RED run uses an absent path); `HOST_CHECKLIST_MUTATION_RECORD` names the mutation record file.

## Honest boundary (11.4.6)

A pass means the local host meets the check at the time of the run; it does not prove a build will succeed. C14 and C15 stay
`UNCONFIRMED` until the owner answers or T005d records a pull. The remote-only checks (SSH, host keys, distribution) are
NOT-APPLICABLE-LOCAL-HOST by owner decision; if a remote host is ever introduced, T099/T100 must run for it and the `na` items change.
