# identity: WP-12 PA-06, user-space NFSv3 client `submodules/filesystem/pkg/nfs3`, evidence index
# head: 4c8b07b7 (main), submodules/filesystem 66b6bc1 plus an UNCOMMITTED working tree (own-org submodule: nothing was committed or pushed, by instruction)
# run_at: 2026-10-07 (UTC); author: Sonnet worker; independent review (11.4.142): WF19 performed, verdict NO-GO; second-round fix (fix-r2-*) below, re-review OWED
# scope: no NAS contact, no git write, no credential read or printed; all tests ran in rootless digest-pinned IMG-GO via scripts/containers/run_pinned.sh

Verify: `cd specs/001-full-project-audit-remediation/evidence/wp12/nfs3 && sha256sum -c SHA256SUMS` (the evidence files) and, from the repository
root, `sha256sum -c specs/001-full-project-audit-remediation/evidence/wp12/nfs3/SOURCES.sha256` (the uncommitted source files these runs used).
Each captured `.txt` starts with identity lines. User documentation: `docs/NFS3_CLIENT.md`.


## Second-round fix of the WF19 review (fix-r2-*, constitution 11.4.276)

The independent one-pass review `WF19` (Opus, NO-GO: 2 HIGH, 6 MEDIUM, 8 LOW, 2 INFO source defects, 5 test and 5 documentation findings)
is closed in ONE fix pass. Order of work: `fix-r2-convergence.md` (assessment, defect classes and their members, written first), RED on the
pre-fix package, the fixes, GREEN x3 with `-race`, mutation with a negative control. Identity of every `fix-r2-*.txt`: head 4c8b07b7,
submodules/filesystem 66b6bc1 plus the uncommitted working tree whose hashes are in `SOURCES.sha256` (the `sources:` prefix in each header).

| Item | Number | File |
|---|---|---|
| RED: the reviewer's probes adopted verbatim, run on the PRE-FIX package | 15 of the 18 probe tests FAIL (the 3 that pass are controls: P13 empty flavor list, P15 PERM is access, P17 no goroutine leak); the 17 killers pass on it by design (they kill mutants) | `fix-r2-red-probes.txt` |
| RED measurements (pre-fix, from the same file) | P1 listing: 58,972 and 59,156 READDIRPLUS in 3 s; P2 `MaxRetries` -2/-5 break `Connect`; P4 `ReadRange(100)`: 31,462x, P4b 83,894x read amplification; P5 7.9x RPCs; P6 renamed tree still resolves; P7 NUL name listed; P8 one vanished entry fails the listing; P9 `SourcePort=0`; P10 `UMNT=0`; P11 size 2^64-1 reads 0 bytes without error; P12 xid reused; P14 uid 0; P16 silent EOF after an error | same |
| GREEN, `-race`, `-count=1`, three independent container runs | 124 top-level tests PASS each run (was 54), 0 FAIL, 0 data race, exit 0 | `fix-r2-green-race-run{1,2,3}.txt` |
| After the fixes, the same probes | P1: 2 RPCs then `ErrNotProgressing`; P4/P4b: 1 READ, 204 reply bytes; P5: 1.0x; P9: real port; P10: `UMNT=1`; P12: no xid reused; P16: `"alpha\n"` | `fix-r2-green-race-run1.txt` (PROBE lines) |
| Mutation, hermetic copy, whole suite per mutant, negative control first | 49 mutants: the reviewer's A01-A18 (re-addressed to the fixed source) plus 31 revert-the-fix mutants (one per fix and per layer of a defence in depth); **49 KILLED, 0 survived**, negative control PASS | `fix-r2-mutants.txt`, `tools/fixr2_mutants.sh`, `tools/fixr2_mutants/` |
| Independent AUTH_SYS oracle (go-nfs-client XDR decoder in the fixture) | 3 mutants of the client's AUTH_SYS body all KILLED by `TestGoNFSAuthSysCredentialIsWellFormed`, negative control PASS (10 credentials verified) | `fix-r2-authsys-mutants.txt` |
| go-nfs real-server leg x3 | 5 / 5 tests PASS each run (new: AUTH_SYS well-formedness, workflow leaves the tree unchanged by path/type/size/SHA-256) | `fix-r2-integration-gonfs-run{1,2,3}.txt` |
| Statement coverage | 90.7 percent (floor 85 in 11.4.224; branch coverage not measured) | `fix-r2-coverage.txt` |
| `go vet` (default and tag `nfs3fixture`), `gofmt -l` | clean | `fix-r2-vet.txt` |
| Fuzz after the decoder change | `FuzzDecoders` 30 s, `FuzzReadRecord` 20 s: PASS, no crash | `fix-r2-fuzz-*.txt` |

Honest notes on this round:

* **Killers vs the pre-fix code.** The reviewer's 17 killers are mutant killers: they pass on the pre-fix package and fail on their mutants
  (49 / 49 above). The probes are the tests that fail on the pre-fix code. The new tests of `fixr2_test.go` use API that does not exist
  pre-fix (config fields, `VanishedEntries`, the cache entry type), so they cannot compile there; their RED is the revert-the-fix mutants.
* **Deviations from the reviewer's text** are limited to the three probes that only logged (P12, P14, P15: now assert), the goroutine name in
  P17 (`topUp` became `issue`) and a `Disconnect` cleanup in P3, all marked `ADOPTED-CHANGE` in `review_probes_test.go`.
* **Masking found and removed while closing the findings.** The A10 mutant (no 1 MiB cap) survived because the 3 MiB fixture never let the
  ramp reach a larger chunk (test file now 24 MiB); the redundant `cookie == reqCookie` guard was equivalent to the repeated-cookie guard and
  was deleted rather than defended; the discard-the-window revert (R03a) survived until a test counted READs for ONE short reply;
  A04 survived once because its old killer depended on a connection race (replaced by `TestDialFailureIsRetried`); A08 and the new root
  remount would have masked each other (`TestKillA08bStaleParentNeedsNoRemount`).
* **Not done / UNCONFIRMED.** Real-NAS behaviour (JUKEBOX timing, rtmax, reserved ports); the image leg (`tools/image_leg.sh`) was not re-run
  (the fixture gained `-authsys`, the image build is unchanged); the independent re-review is owed; S15 (remount) and S14 (blocked write) are
  tested against in-process servers only; the blocked-write test relies on kernel socket buffers below 32 MiB (its control proves the
  write blocks on this host).
* **Fixture finding.** go-nfs-client's `xdr.ReadOpaque` does not consume the 4-byte padding, so the oracle reads the machine name as an XDR
  string; this is a limitation of that library, not of the client under test.

How to reproduce: `bash specs/001-full-project-audit-remediation/evidence/wp12/nfs3/tools/fixr2_run_evidence.sh` (about 11 minutes; needs the
network once for the fixture's modules).

---

## Result numbers

(Revision 1, before the WF19 review. The section above supersedes them where it gives other numbers; the 22 mutants below measured the author's own mutants, which the review showed to be much weaker than the independent ones.)

| Item | Number | File |
|---|---|---|
| RED on a broken artifact (5 load-bearing functions stubbed "not implemented", API intact) | 39 of 54 top-level tests FAIL, 15 pass (the XDR-only tests that touch none of the stubs' callers) | `red-baseline.txt` |
| GREEN, `-race`, `-count=1`, three independent container runs | 54 / 54 top-level tests PASS each run (plus the fuzz seed corpora), exit 0 | `green-race-run{1,2,3}.txt` |
| Statement coverage (`-covermode=atomic`) | 88.5 percent (floor 85 in 11.4.224; branch coverage not measured) | `coverage.txt` |
| `go vet` (default and tag `nfs3fixture`), `gofmt -l` | clean | `vet.txt` |
| Fuzz `FuzzDecoders` (all decoders plus RPC reply parser), 60 s, 3 workers | 1,259,753 execs, 90 corpus entries, no crash, no amplification | `fuzz-decoders.txt` |
| Fuzz `FuzzReadRecord` (record marking), 30 s | 645,016 execs, 21 corpus entries, no crash | `fuzz-readrecord.txt` |
| Real server leg: go-nfs v0.0.4 (independent implementation) in the same pinned container, 3 runs | 4 / 4 tests PASS each run, exit 0 | `integration-gonfs-run{1,2,3}.txt` |
| Real server leg: go-nfs as a separate rootless read-only capability-free container built from `Containerfile`, test container joined to its network namespace | 4 / 4 PASS | `image-leg.txt` |
| Mutants (self-written) plus negative control | 22 mutants, 22 KILLED, 0 survived, negative control (unmutated tree) PASS | `mutants.txt` |

A first mutant run had 18 killed and 4 not: 2 uncompilable and 1 mis-addressed mutants (harness fixed: M05, M09, M18), and 1 real survivor
(M21, "read-ahead forced to one READ"): the in-flight counter of the test server decremented after the reply write and so saw overlap
that did not exist; the counter now releases before the write and M21 is killed. The survivor was a weakness in the test, not in the client.

## Protocol coverage

| Procedure / feature | Spec | Client | Unit (in-process server) | go-nfs (independent server) | Notes |
|---|---|---|---|---|---|
| Record marking, fragments, 8 MiB cap | RFC 5531 s11 | yes | `TestReadRecord*`, `FuzzReadRecord` | implicit | zero-length fragments and fragment floods bounded |
| Call/reply, xid matching, pipelining, out-of-order replies | RFC 5531 s8-9 | yes | `TestBuildCallGolden`, `TestParseReply`, `TestReadPipelines*`, `TestWrongXID*` | implicit | golden call bytes assembled by hand from the RFC |
| AUTH_SYS (uid, gid, 16 gids, machine name) and AUTH_NONE | RFC 5531 app A | yes | `TestAuthSys*`, `TestAuthNullOnlyExportUsesAuthNone` | AUTH_NONE (go-nfs lists only AUTH_NULL) | |
| Portmapper GETPORT (TCP) | RFC 1833 s3 | yes | `TestConnect*`, `TestDecodeGetport` | not offered by go-nfs | |
| MOUNT v3 MNT | RFC 1813 app I | yes | `TestConnect*`, `TestDecodeMnt`, `TestReservedPort*` | yes | `MNT3ERR_ACCES` becomes `*AccessError` |
| MOUNT v3 EXPORT | RFC 1813 app I | yes | `TestExports`, `TestDecodeExports` | not offered | |
| MOUNT v3 UMNT | RFC 1813 app I | yes | `TestConnectUsesPortmapperAndMounts` | best effort | |
| NFS NULL | RFC 1813 s3.3.0 | yes | `TestConnectUses*` | yes | |
| GETATTR | s3.3.1 | yes | `TestGetFileInfo` | yes | |
| LOOKUP (cached directory handles, stale-handle recovery) | s3.3.3 | yes | `TestGetFileInfo`, `TestStaleCachedParentIsDropped` | yes | |
| ACCESS | s3.3.4 | yes | `TestAccessAndReadOnly` | yes | |
| READ (ranged, pipelined, short-read resume, seek) | s3.3.6 | yes | `TestRead*` | yes (3 MiB plus 13 bytes, SHA-256 against the server manifest) | |
| READDIRPLUS (cookie, cookieverf, paging, bad cookie restart, attribute fallback) | s3.3.17 | yes | `TestListDirectory*`, `TestBadCookie*`, `TestListWithoutAttributes*` | yes (250-entry directory, whole tree) | |
| FSINFO | s3.3.19 | yes | `TestReadChunkFollowsFSInfoButIsCapped` | yes | |
| SETATTR, WRITE, CREATE, MKDIR, SYMLINK, MKNOD, REMOVE, RMDIR, RENAME, LINK, COMMIT | | **absent** | `TestNoWriteProcedureCompiledIn`, `TestAccessAndReadOnly` (server counters), M14 | `TestGoNFSWorkflowLeavesTheTreeUnchanged` | |
| READLINK, READDIR, FSSTAT, PATHCONF, NFSv4, UDP, Kerberos, NLM | | not implemented | | | out of scope |
| Reserved source port: precise refusal, one privileged retry, unavailable-privilege error | | yes | `TestReservedPort*`, `TestPrivileged*`, `TestRealPrivilegedBind` | | see UNCONFIRMED |
| Timeouts, context cancellation, bounded retries, reconnect | | yes | `TestCallTimeoutIsBounded`, `TestContextCancellation*`, `TestRetry*`, `TestNoRetry*`, `TestCloseUnblocksReader` | | |
| Malformed and hostile replies | | yes | `TestMalformedReplies*`, `TestGarbageStream*`, `TestDecode*`, both fuzzers | | |
| Path confinement (`..`, NUL) | | yes | `TestPathConfinement` | | |

## How to reproduce

```
bash specs/001-full-project-audit-remediation/evidence/wp12/nfs3/tools/run_evidence.sh   # RED, GREEN x3, coverage, vet, fuzz, go-nfs x3, mutants (about 10 min)
bash specs/001-full-project-audit-remediation/evidence/wp12/nfs3/tools/image_leg.sh      # image build, container run, test via shared netns
```

`tools/red.sh` and `tools/mutants.sh` run inside IMG-GO; the others are host-side drivers around `scripts/containers/run_pinned.sh`.

## UNCONFIRMED and deviations (stated, not hidden)

* **Test-first order.** The implementation was written before most tests in this session, so the RED run is a retroactive RED on a stubbed
  artifact plus the mutant kills, not a first-write RED. The negative control and the 22 kills are the stronger evidence.
* **Reserved source ports on a real NAS: UNCONFIRMED.** The privileged path is proven against an in-process server that gates on the
  client's source port, with the bind simulated by a dial hook. `TestRealPrivilegedBind` exercises the real bind and records which of the two
  allowed outcomes occurred: in the rootless container the bind of source port 1023 failed with `permission denied` and was classified `ErrPrivilegedPortUnavailable` (`green-race-run1.txt`), i.e. a rootless run cannot satisfy a reserved-port NAS and reports that precisely. Whether Synology/other NAS demand a reserved port, and
  whether `rtpref`/`READDIRPLUS` sizes behave as modelled, is UNCONFIRMED until the NAS read-only leg runs.
* **T134 unfs3 fixture not used.** It lives on the compose network and only `IMG-INFRA-CLIENT` (no Go toolchain) joins it. The rpcbind
  path is covered by the in-process server only; go-nfs has no portmapper. A libnfs `nfs-ls` oracle comparison is therefore not done
  (the oracle used is go-nfs plus its own server-side manifest).
* **Image leg deviation.** `run_pinned.sh` has no `--network` option other than none, so `image_leg.sh` takes its own composed argv
  (`RUNP_PRINT_ARGV=1`) and adds `--network container:<server>`. The compose file `docker-compose.test-infra.nfs3.yml` parses (podman-compose
  `config` with a stub network) but a full `up.sh` stack run was NOT done.
* **go-nfs timestamps.** The memory filesystem has no stable mtime, so the manifest compares path, type, size and SHA-256 only; mtime/nanosecond
  handling is tested against the in-process server.
* **Dependency register (11.4.270).** go-nfs v0.0.4 is VERIFIED to exist (Apache-2.0, per the protocol-analysis report) and is a fixture-only
  dependency of a separate nested module (`test/nfs3fixture/go.mod`); the `submodules/filesystem` go.mod is untouched by this task.
* **Not wired** into `pkg/factory` or `catalog-api`; `pkg/nfs` (kernel mount) is unchanged. No removal was made.
* Other streams were editing `submodules/filesystem` (`go.mod`, `go.sum`, `pkg/factory`, `pkg/sftp`, `pkg/decorators`, `pkg/fabric`) concurrently; this
  task touched none of them. All runs here saw those files as they were at run time.
