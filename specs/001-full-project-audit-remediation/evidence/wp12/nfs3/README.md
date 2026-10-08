# identity: WP-12 PA-06, user-space NFSv3 client `submodules/filesystem/pkg/nfs3`, evidence index
# head: revisions 1 and 2 are COMMITTED: submodules/filesystem 83c0ac1, main e9d6883d. The third fix pass (fix-r3-*) is an UNCOMMITTED working-tree change on top of 83c0ac1 (own-org submodule: nothing was committed or pushed by the fixer; the conductor commits)
# run_at: revisions 1-2 on 2026-10-07 (UTC), fix-r3 on 2026-10-08 (UTC); author: Sonnet worker; independent reviews (11.4.142): WF19 (NO-GO) and WF24 (NO-GO) performed; the third fix pass (fix-r3-*) is below, its re-review is OWED
# scope: no NAS contact, no git write, no credential read or printed; all tests ran in rootless digest-pinned IMG-GO via scripts/containers/run_pinned.sh

Verify: `cd specs/001-full-project-audit-remediation/evidence/wp12/nfs3 && sha256sum -c SHA256SUMS` (the evidence files) and, from the repository
root, `sha256sum -c specs/001-full-project-audit-remediation/evidence/wp12/nfs3/SOURCES.sha256` (the source files these runs used; the fix-r3 files verify once they are committed, because the runs used the uncommitted working tree).
Each captured `.txt` starts with identity lines. User documentation: `docs/NFS3_CLIENT.md`.


## Third-round fix of the WF24 re-review (fix-r3-*, constitution 11.4.276)

The independent re-review `WF24` (round 2, Opus, NO-GO: 1 MEDIUM and 4 LOW source defects plus 3 INFO, 2 MEDIUM and 3 LOW test findings plus 1 INFO, 6 LOW
documentation findings; all 28 WF19 findings verified closed) and the defect the real-NAS leg found (`../nas-clients/nfs-*.json`: `Connect` with
`TryPrivilegedPort` discarded the server's original refusal) are closed in ONE pass. This was a structural round: `fix-r3-convergence.md` (written first)
re-derives the recurring classes (which handle a status refers to; cache validity as a chain property; context and the effect of one caller's cancellation
on a shared connection; the allow-list key) and closes every member, with the censuses in `fix-r3-census.txt` (control-needled). Identity of every
`fix-r3-*.txt`: head f72d8757, submodules/filesystem 83c0ac1 plus the uncommitted working tree whose hashes are in `SOURCES.sha256`.

| Item | Number | File |
|---|---|---|
| RED on the COMMITTED code (83c0ac1 via `git archive` + only the adopted probes/killers and `fixr3_test.go`) | 16 FAIL: the 5 WF24 probes (W1 W2 W3 W4 W7) and 11 fix-r3 tests; the 8 WF24 killer tests (one test serves N01 and N02; they cover the 9 survivors) give 7 PASS and 1 SKIP (they kill mutants, not the committed code; N15 SKIPs: no reserved-port bind in the pinned container); 4 fix-r3 controls PASS (unchanged parent keeps children; write timeout with a live context fails the connection; cancelled PARTIAL write fails the connection; failed Connect after MNT leaves the client disconnected) | `fix-r3-red.txt` |
| The WF24 mutants, verbatim, on the committed code and committed suite | 18 mutants: 9 KILLED, 9 SURVIVED (N01 N02 N03 N08 N10 N11 N15 N16 N17): the reviewer's measurement is reproduced; negative control PASS | `fix-r3-mutants-committed.txt` |
| GREEN, `-race`, `-count=1`, three independent container runs | 161 top-level tests PASS each run (was 124), 0 FAIL, 1 SKIP (the N15 killer, honest: no privileged bind), 0 data race, exit 0 | `fix-r3-green-race-run{1,2,3}.txt` |
| Mutation on the fixed tree, whole suite per mutant, negative control first | 49 mutants: the WF24 reviewer's 18 (N01 and N02 re-addressed because the text moved) plus 31 revert mutants (one per fix and per layer of a defence in depth); **49 KILLED, 0 survived**, 0 uncompilable; negative control PASS | `fix-r3-mutants.txt`, `tools/fixr3_gen_mutants.py`, `tools/fixr3_mutants.sh`, `tools/fixr3_mutants/` |
| Independent AUTH_SYS oracle, now with DISTINCT uid/gid and gid values | 6 mutants of the client's AUTH_SYS body (X01 to X03 of round 2 re-run, plus X04 uid and gid swapped, X05 gid values shifted by one, X06 gids reversed), all 6 KILLED, negative control PASS (10 credentials verified, 0 violations); X04 was invisible to the round-2 oracle because uid = gid = 1000 | `fix-r3-authsys-mutants.txt`, `tools/fixr3_authsys_mutants.sh` |
| go-nfs real-server leg x3 | 5 / 5 tests PASS each run; AUTH_SYS tap: 10 credentials verified, 0 violations, identity uid 1234 gid 2345 gids 4,24,3456 | `fix-r3-integration-gonfs-run{1,2,3}.txt` |
| Fixture image (TN2) | built from the tree (`Containerfile` now copies every `.go` file; WF24 measured the old file set failing with `undefined: authCheck, tapListener, sysHandler`; this pass did not re-run the old Containerfile), run rootless read-only capability-free, tests via a shared network namespace: 4 PASS and 1 honest SKIP (the AUTH_SYS leg only runs in-container), exit 0 | `fix-r3-image-leg.txt` |
| Statement coverage | 92.7 percent (floor 85 in 11.4.224; branch coverage not measured) | `fix-r3-coverage.txt` |
| `go vet` (default and tag `nfs3fixture`), `gofmt -l` | clean | `fix-r3-vet.txt` |

What changed, by finding:

* W1: the attribute fallback of a listing judges an error by the call that failed (RFC 1813 3.3.3: the only handle in LOOKUP is the directory). On the
  LOOKUP leg only `NOENT` means "entry vanished"; `STALE`/`BADHANDLE` fail the listing. The GETATTR leg (the entry's handle) keeps all three.
* W2 and the cache class: every entry records the generation of its parent entry; an entry is trusted only while its whole chain up to the root is
  present, within `HandleCacheTTL`, and carries the generation the parent still has. A directory that gets a different handle gets a new generation.
* W3: the failure-path `UMNT` keeps a bound (2 s; 500 ms once the caller's context ended; never longer than `CallTimeout`) instead of waiting on `Background`.
* W4: the allow-list key is (program, version, procedure).
* W7: an interrupted write that sent 0 bytes keeps the shared connection; a partial write, or a write timeout with a live context, fails it, BEFORE the
  write slot is released, and a caller that waited for the slot re-checks the connection. Each READ of the read-ahead has its own context (I2).
* Class members found by the census: the remount mutex is a context-aware semaphore; a stale walk whose remount was cancelled reports the context.
* I1: the cleanup `UMNT` is registered when the MNT status reads OK, before the body is decoded. I3: `MaxPipeline` is bounded (64), `DirCount` above `DirMaxCount` is refused.
* Real-NAS finding: with `TryPrivilegedPort`, when the retry fails for another reason (cannot bind a reserved port) `Connect` returns both errors, the
  original refusal first; if the server refuses the reserved-port attempt too, that second `*AccessError` is returned alone.
* TN1 and TN5: all nine WF24 killers adopted; the reserved-port loop is executed through an injected bind (`dialPrivilegedFrom`). TN2: `Containerfile` copies
  `*.go`, the image was built and run. TN3: `run_integration.sh` is `bash`. TN4: the fixture checks uid, gid and the gid VALUES (default `-authuid 1234
  -authgid 2345 -authgids 4,24,3456` in the script; the oracle was blind to a uid/gid swap while both were 1000). TN6: asserted directly.
* DN1 to DN6: `docs/NFS3_CLIENT.md` revision 3 (status names the commit; the handle-cache subsection is no longer inside the failure list; the three
  claims the measurements contradicted are corrected; the reserved-port refusals include the PERM statuses); this README's statements on the image
  build and the "four guards" wording of `fix-r2-convergence.md` (there is no R01a) are corrected.

Honest notes on this round:

* **Test-first.** The adopted probes and the fix-r3 tests that compile against 83c0ac1 failed there (`fix-r3-red.txt`). The tests that need the new internals
  (`fixr3_api_test.go`: the leg of `vanishedErr`, the version in the allow-list key, `dialPrivilegedFrom`, `remountSem`, `truncateAt`) cannot compile there; their RED is the
  revert mutants. Four fix-r3 tests are controls and pass on the committed code by design (listed above).
* **Equivalent code removed rather than defended.** The first mutation run found two mutants that were equivalent (a wrapper branch in `listOnce`, a `min()`
  with `CallTimeout` that `oneShot` already implies) and five that did not compile; the code was simplified and the mutants repaired. The generation-chain cache
  needed neither the eager purge of descendants nor the "skip a path with an unknown parent" rule, so both were removed.
* **F17 (the write slot released before the connection is failed) was a RACE mutant.** The first version of the test (200 plain trials) killed it in only
  3 of 5 runs (measured with the harness: the main run plus 4 repeat runs; the raw lines of those superseded runs were not kept). It was replaced by a
  deterministic test, `TestFixR3FailedWriterKeepsTheSlotUntilTheConnectionIsFailed` (`fixr3_api_test.go`): `rpcConn.failHook` (called at the start of `fail`, set only by
  tests) holds the failing writer where a released slot would let a waiting caller write. `TestFixR3NoRecordIsAppendedToAHalfWrittenStream` keeps 100 plain
  trials of the same race (the re-check, mutant F16). The kills of F16 and F17 in `fix-r3-mutants.txt` are from this version.
* **Not done / UNCONFIRMED.** The real reserved-port bind (the pinned container refuses it even as container root: only the loop is executed, through an injected bind);
  real-NAS behaviour of JUKEBOX timing, `rtmax`, and whether a real NAS omits READDIRPLUS attributes (the W1 trigger); W7 in the real world (probabilistic: the rate on a
  real network is UNKNOWN); the two fuzzers were not re-run (the XDR and record decoders did not change); the NAS was not contacted. The compose file does not pin an
  image digest (it takes `TI_NFS3_IMAGE` from `build_image.sh`), so nothing had to be re-pinned; the digest printed in `image-leg.txt` (fix-r2) belongs to the older build.
* The independent re-review of this pass is OWED.

How to reproduce: `bash specs/001-full-project-audit-remediation/evidence/wp12/nfs3/tools/fixr3_run_evidence.sh` (about 30 minutes; needs the network once for the
fixture's modules; the launcher may wait for host memory).

---

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
  (the fixture gained `-authsys` in a new file, `authsys.go`, which the `Containerfile` did not copy, so the image no longer built from the tree: the statement made here at the time, "the image build is unchanged", was FALSE; found by WF24 TN2 and fixed in fix-r3); the independent re-review is owed; S15 (remount) and S14 (blocked write) are
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
