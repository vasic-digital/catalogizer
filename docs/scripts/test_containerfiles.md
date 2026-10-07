# test_containerfiles.sh - Companion Guide

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-06 |
| Last modified | 2026-10-07T04:45:00Z |
| Status | new, untracked at writing (task T106); independent review owed (constitution 11.4.142) |
| Source | `scripts/containers/tests/test_containerfiles.sh`, mutation table `scripts/containers/tests/containerfiles_mutations.tsv`, the tree `build/containers/` |

## Purpose

Static checks over every image directory of `build/containers/` (docs/16 DR-16-1). One directory per image, each with `Containerfile`, `README.md`, `digests.lock`.

| Check | Meaning |
|---|---|
| C1 | every required directory exists: go gotools node playwright docs rust android mut sigverify infra-client infra-postgres infra-redis infra-ftp infra-smb infra-webdav kcov testutil |
| C2 | `Containerfile`, `README.md` and `digests.lock` exist and are non-empty |
| C3 | the `Containerfile` passes `scripts/containers/check_pins.sh` (digest-pinned `FROM`, no pipe-to-shell); ONLY a clean verdict passes: exit 0 with no `VIOLATION` line, or exit 1 with at least one (each is reported). Exit 2/3/127 or an inconsistent pair (exit 1 with no line, exit 0 with a line) is a C3 violation, so a crashing check cannot read as clean |
| C4 | every download is followed, LATER in the same `RUN`, by its own SHA-256 check. The `RUN` text is PARSED into simple commands (`shlex` with punctuation, so `&&`, `||`, `;`, `|`, `(`, `)`, `{`, `}` split them; quotes are removed): a download is any command whose command word (after assignments and wrappers such as `sudo`, `env`, `timeout`) is `curl` or `wget`, with any arguments, a quoted or `$VAR` URL included, except a pure `--version`/`-V`/`--help`/`-h` call; `command -v curl`, `which curl`, `type curl` are lookups and are not downloads. A check is `sha256sum -c`/`--check` (also a bundled `-bc` style flag) or `shasum -a 256 -c`; each check consumes one EARLIER unchecked download, so a check before a download, or two downloads and one check, leaves a download unchecked. A `RUN` whose shell text cannot be parsed and names `curl`/`wget` is refused, not guessed. An `ADD <url>` carries `--checksum=sha256:<64 hex>` |
| C5 | `digests.lock` names every `FROM` digest and every SHA-256 the `Containerfile` checks |
| C7 | no `RUN` swallows a failure: `|| <command that only succeeds>` (`true`, `:`, `/bin/true`, `echo ...`, `printf ...`, `exit 0`, also as a `{ ...; }` or `( ... )` group of such commands), or `set +e`. An explicit handler (`|| exit 1`, `|| false`, `|| { echo x >&2; exit 1; }`, `|| echo x && exit 1`, `|| test -f y`) is not a swallow (WF10 p1 F6: the android licence step; WF13 N9: the other forms) |
| C6 | the directory has an `images.lock.yaml` entry (rust and android: written by T143 and T144), the entry carries a `class` from compile, interpreter, service, runtime, runtime-base (not required for go, kcov, testutil, whose entries predate the field), and equal digests where the entry reference is a `FROM` reference |

The checker runs on the real tree and on fixtures written to a temporary directory (golden-bad per rule, golden-good, a negative control). MUTCF paired mutations of the checker
(`containerfiles_mutations.tsv`: the original rows that still apply, the WF10 reviewer's survivors CR1-CR6, N-rows for the rules of the C4/C7 command parser, RC1-RC2 re-expressed from the WF13 reviewer's round-2 survivors; the regexes the WF13 survivors RC3 and RC4 mutated no longer exist) must each make the fixtures FAIL. Run on the host or `scripts/containers/run_pinned.sh IMG-KCOV -- bash scripts/containers/tests/test_containerfiles.sh`.
Env: `CF_TEST_NO_MUTATIONS=1`, `CF_MUTATION_RECORD=<file>`. Exit non-zero on any failure.

## Honest limits

- Static only. It does not build an image; the builds and smokes are recorded in `evidence/wp11/t106-build-*.txt` and `t106-smoke-*.txt`.
- `infra-minio` is absent and not required: no MinIO server image was obtainable from quay.io, docker.io or ghcr.io when probed (2026-10-06). Repo evidence for that claim is only `evidence/wp11/t114-minio-blocked.txt`, a trace of the Docker Hub tag query returning no `tags` field; the quay.io `unauthorized` and ghcr.io HTTP 403 readings, and the Hub repository API answering "object not found", are the WF10 reviewer's probes (stored in the reviewer's workspace, not in the repo): the ghcr.io probe is UNCONFIRMED by repo evidence (WF13 D3). `infra-nfs` is absent: DR-16-2 (userspace NFS server) is undecided.
- C5 compares text, not the bytes of a download; the SHA-256 values of `rust` and `android` are values read from the publisher or computed from one download, not signatures.
- `digests.lock` is a hand-maintained record kept consistent with the `Containerfile` by C5; it is not a build input.
- C4/C7 judge the parsed commands of one `RUN`; they do not follow a downloaded file into a script it later runs, do not read a shell script copied into the image, and do not parse a here-document body of a `RUN`. A download made by a tool other than `curl`/`wget` (`git clone`, `pip download`, `npm i`, `apt-get`) is outside C4 by design (package managers verify their own signatures; UNCONFIRMED for each).
