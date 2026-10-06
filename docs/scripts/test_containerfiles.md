# test_containerfiles.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T00:50:00Z |
| Status | new, untracked at writing (task T106); independent review owed (constitution 11.4.142) |
| Source | `scripts/containers/tests/test_containerfiles.sh`, mutation table `scripts/containers/tests/containerfiles_mutations.tsv`, the tree `build/containers/` |

## Purpose

Static checks over every image directory of `build/containers/` (docs/16 DR-16-1). One directory per image, each with `Containerfile`, `README.md`, `digests.lock`.

| Check | Meaning |
|---|---|
| C1 | every required directory exists: go gotools node playwright docs rust android mut sigverify infra-client infra-postgres infra-redis infra-ftp infra-smb infra-webdav kcov testutil |
| C2 | `Containerfile`, `README.md` and `digests.lock` exist and are non-empty |
| C3 | the `Containerfile` passes `scripts/containers/check_pins.sh` (digest-pinned `FROM`, no pipe-to-shell); ONLY a clean verdict passes: exit 0 with no `VIOLATION` line, or exit 1 with at least one (each is reported). Exit 2/3/127 or an inconsistent pair (exit 1 with no line, exit 0 with a line) is a C3 violation, so a crashing check cannot read as clean |
| C4 | every download line (`curl`/`wget` followed by an option or URL in a `RUN`) is followed in the same `RUN` by `sha256sum -c` or `shasum -a 256 -c`, and the `RUN` holds at least as many such checks as downloads; an `ADD <url>` carries `--checksum=sha256:<64 hex>` |
| C5 | `digests.lock` names every `FROM` digest and every SHA-256 the `Containerfile` checks |
| C7 | no `RUN` swallows a failure with `|| true` or `|| :` (WF10 p1 F6: the android licence step) |
| C6 | the directory has an `images.lock.yaml` entry (rust and android: written by T143 and T144), the entry carries a `class` from compile, interpreter, service, runtime, runtime-base (not required for go, kcov, testutil, whose entries predate the field), and equal digests where the entry reference is a `FROM` reference |

The checker runs on the real tree and on fixtures written to a temporary directory (golden-bad per rule, golden-good, a negative control). Twenty-three paired mutations of the checker
(`containerfiles_mutations.tsv`: 9 kept, the reviewer's 6 survivors CR1-CR6, 8 for the rules added by WF10 p1) must each make the fixtures FAIL. Run on the host or `scripts/containers/run_pinned.sh IMG-KCOV -- bash scripts/containers/tests/test_containerfiles.sh`.
Env: `CF_TEST_NO_MUTATIONS=1`, `CF_MUTATION_RECORD=<file>`. Exit non-zero on any failure.

## Honest limits

- Static only. It does not build an image; the builds and smokes are recorded in `evidence/wp11/t106-build-*.txt` and `t106-smoke-*.txt`.
- `infra-minio` is absent and not required: no MinIO server image is obtainable from quay.io, docker.io or ghcr.io (probed 2026-10-06). `infra-nfs` is absent: DR-16-2 (userspace NFS server) is undecided.
- C5 compares text, not the bytes of a download; the SHA-256 values of `rust` and `android` are values read from the publisher or computed from one download, not signatures.
- `digests.lock` is a hand-maintained record kept consistent with the `Containerfile` by C5; it is not a build input.
