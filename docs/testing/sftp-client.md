# Read-only SFTP client (`submodules/filesystem/pkg/sftp`)

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-07 |
| Last modified | 2026-10-08T05:00:00Z |
| Status | COMMITTED in the submodule `submodules/filesystem` at `83c0ac1f407f` (main repo pointer bump `e9d6883d`) in the state of fix round 2. History: round 1 independent review (WF19) NO-GO; fix round `fix-r2`; round 2 independent re-review of the COMMITTED fix-r2 tree (WF24, Opus xhigh, 11.4.142 / 11.4.209) NO-GO (4 MEDIUM source defects, 2 MEDIUM test-instrumentation, 1 MEDIUM process-doc, LOW items); fix round `fix-r3` (this revision) fixes every finding in the working tree of the submodule - at the time of writing fix-r3 is NOT committed (the conductor commits it; `git status` in the submodule shows it until then). The independent re-review of fix-r3 is OWED and has not been performed. Correction of a record: the commit message of `83c0ac1` / `e9d6883d` says each client passed "an independent one-pass review and one class-complete fix pass"; that was inaccurate when it was written (the round-1 review of this client was NO-GO and the re-review had not run). The commit is not rewritten (11.4.113); this sentence is the correction. |
| Task | WP-12 PA-05 of `specs/001-full-project-audit-remediation/evidence/wp12/protocol-analysis/REPORT.md` (sections 2.5, 3, 4) |
| Evidence | `specs/001-full-project-audit-remediation/evidence/wp12/sftp/` (README.md, `fix-r2-*` and `fix-r3-*` files: convergence assessments, RED, GREEN x3, mutation runs, SHA256SUMS, SOURCES.sha256) |
| Fixture | `docker-compose.test-infra.sftp.yml`, `scripts/test-infra/sftp_fixture.sh` (guide: `docs/scripts/sftp_fixture.md`) |

Every sentence in the "Behaviour" and "Honest limits" sections is either pinned by a test that fails when the sentence stops being true (the test is named in
the right-hand column) or is listed as a limit. This is checked by mutation, not asserted: `evidence/wp12/sftp/fix-r3-mutants.py` mutates each mechanism a sentence is about and
the suite must fail (the result is in `fix-r3-mutants-summary.txt`: 115 mutants, the survivors of the first pass and what each led to are listed there; the final state is all killed). Statements of earlier revisions that the reviews refuted by measurement
(six of them in round 2: see "Corrections" at the end) were rewritten, not kept.

## What it is

A client for the SFTP protocol that implements `client.Client` and `client.SeekableClient` of `digital.vasic.filesystem/pkg/client`, built on
`github.com/pkg/sftp` v1.13.11 (BSD-2-Clause, existence VERIFIED in REPORT 3.4) over `golang.org/x/crypto/ssh`. The factory
(`pkg/factory`) creates it for protocol `sftp`.

It is READ-ORIENTED: every mutating method (`WriteFile`, `DeleteFile`, `CopyFile`, `CreateDirectory`, `DeleteDirectory`) returns `sftp.ErrReadOnly`
and no code path in the package sends a write request (the review's census of all remote calls found only read-class requests). This is a property of the
type, not a configuration flag.

## Behaviour

| Concern | Behaviour | Pinned by |
|---|---|---|
| Pipelining | `pkg/sftp` keeps a window of concurrent read requests per file (`Config.MaxConcurrentRequests`, default 64; `MaxPacket`, default 32768). `ReadFile` returns a reader whose `WriteTo` is the pipelined path, so `io.Copy` uses it. `ReadRange` with a length returns a reader whose `WriteTo` reads in windows of `MaxPacket x MaxConcurrentRequests` (capped at 4 MiB), which `pkg/sftp` serves with concurrent requests: HTTP Range streaming through `ReadRange` is pipelined too. The path the consumer's stream handler really uses, `http.ServeContent` over `OpenSeekable` (32 KiB reads), is pipelined by a sequential READ-AHEAD in `file.Read`: after two plain reads without a `Seek`, a `Read` with a buffer smaller than the read window fetches the next block (starting at two packets, doubling up to `MaxPacket x MaxConcurrentRequests`, capped at 4 MiB), which `pkg/sftp` serves with concurrent requests, and serves the following reads from it; a `Seek` inside the buffered bytes moves inside the buffer, any other `Seek` drops it. Cost, stated: an open file read sequentially holds up to one read window (2 MiB by default) and the server is asked for up to that much beyond the byte the reader stopped at; a bounded `ReadRange` never reads ahead past its end. Measured with a server-side gauge of concurrent READs (at least 3 in flight; 8 for `ServeContent` over `OpenSeekable` against 1 before), not inferred from correct bytes. | `TestPipelinedReadKeepsSeveralRequestsInFlight`, `TestRangedReadIsPipelinedToo`, `TestWF24_N7_ServeContentOverOpenSeekableIsNotPipelined`, `TestReadAheadSequentialReadsAreByteIdentical`, `TestReadAheadAgainstAModel`, `TestReadAheadSeekInsideAndOutsideTheBuffer`, `TestReadAheadThenWriteToLosesNothing`, `TestReadAheadOnADyingConnectionIsNotAnEOF`, `TestReadAheadEndOfFileConsultsTheConnection`, `TestBoundedRangeNeverReadsBeyondItsEnd`, `TestReadWindow` |
| Ranged reads | `ReadRange(ctx, path, offset, length)` and `OpenSeekable` (`Seek` + `Read`) for HTTP Range serving. A negative offset is refused before any connection is needed. | `TestServer_PipelinedCopyAndRange`, `TestReadRangeNegativeOffsetIsRefusedUpFront` |
| Listing | `ListDirectory` returns the attributes the server sent with each entry (no extra `stat` per entry). `.` and `..` are dropped (also when a server sends them as the last element of a longer name). Symbolic links are listed as links (`Mode&os.ModeSymlink`) and not followed. Names that are not file names (`/`, empty, containing NUL) are dropped like `.` and `..` (a server name `/` would otherwise become an entry whose `Path` is the listed directory itself). Every listing has its OWN budget of `Config.MaxDirEntries` entries (default 1,000,000; negative disables), counted per directory handle from the requests this client sent; the budget starts when the handle is opened and ends with its CLOSE, so overlapping listings never share it and a `REALPATH` reply is never counted. A listing that delivers more fails - that listing only - with `ErrDirTooLarge` (not transient); the connection and every other call and stream on it stay up. | `TestRaw_ListingDropsDotEntries`, `TestRaw_ListingLimit`, `TestServer_ListingReportsSymlinkAsLink`, `TestWF24_N2_*`, `TestWF24_N3_*`, `TestListingOverBudgetFailsOnlyThatListing`, `TestOversizeListingDoesNotDisturbAStream`, `TestListingDropsImplausibleNames`, `TestWireTracker_*`, `TestFrameReader_OverBudgetPageBecomesAStatusFailureNotAFault` |
| Host key | Verified against a PIN per `host:port` (see below). `ssh.InsecureIgnoreHostKey` is not used anywhere. | `TestHostKeyCallback_UnknownMismatchMatch`, `TestServer_HostKey_*` |
| Credentials | By `credential_ref`, resolved at connect time by a `CredentialResolver`; a private key (optionally with passphrase) and/or a password. The password is offered as `password` and, for servers that only offer it, as `keyboard-interactive` (echo-off prompts only, at most 3 rounds of at most 4 questions). No secret is stored in `Config`, returned by `GetConfig`, formatted by `String`, or put in an error message. A rejected login is an `*AuthError` that also answers `errors.Is(err, fabric.ErrAuth)` (a typed contract with `pkg/fabric`, the precedent being `pkg/ftp`), so the pool's "never log in again with a rejected secret" latch engages for both shapes a real server produces (the plain rejection and the keyboard-interactive failure) without reading any text. | `TestReview_P9_*`, `TestKbdAnswer`, `TestCredential_Redaction`, `TestGetConfigHasNoSecret`, `TestWF24_N5_*`, `TestAuthContractWithFabric`, `TestWrongPasswordIsFabricAuthInBothShapes` |
| Path confinement | All paths are confined to `Config.Root`: a path that climbs above the root with `..` is refused (`ErrPathEscape`), it is not clamped. Paths are opaque: nothing is rewritten, a backslash is an ordinary name byte (a file named `x\y` is read as `x\y`, never as `x/y`). After the lexical check, the server-resolved path (`RealPath`) must still be inside the root, which refuses symlinks that leave it. The check FAILS CLOSED: a `RealPath` error other than "does not exist" refuses the call (`cannot verify ... stays inside the root`); "does not exist" is returned as not-exist so the call reports it. A symlink whose resolved path stays inside the root works. | `TestConfine_Table`, `TestReview_P5_*`, `TestReview_P12_*`, `TestContainmentFailsClosedButReportsNotExist`, `TestIntegration_PathConfinement` |
| Cancellation | See "Cancellation and dead links" below. | `TestReview_P1/P2/P8/P11`, `TestSeekEndHonoursContextOnHungServer`, `TestCloseIsBoundedOnAStalledServer` |
| Retry | Bounded (`Config.MaxRetries`, default 3; negative disables) with exponential backoff (`RetryBase`, default 200 ms, doubled per retry, cap 2 s) on TRANSIENT errors only: network timeouts, connection reset or refused, a lost connection (`ErrSSHFxConnectionLost`, a closed channel), an unexpected EOF while connecting. Never retried: authentication failure, host key refusal, missing credential, malformed reply, path refusal, not found, permission denied, context cancellation, a non-timeout network error (a DNS "no such host"). A server STATUS reply, including `SSH_FX_EOF` to a stat or open, is a reply and not a lost connection: it is not retried and does not tear down the connection or the streams on it. A context error (`context.Canceled`, `context.DeadlineExceeded`, also wrapped) is neither a connection loss nor transient (`context.DeadlineExceeded` is a `net.Error` with `Timeout()==true`, which the classifier used to read as a transport timeout). The two predicates agree: every connection loss is transient, and the whole table of errors that can reach the call paths is asserted, with the drop decision. | `TestIsTransient_Table`, `TestClassification_LossImpliesTransient`, `TestClassification_TruthTable`, `TestWF24_N1a_*`, `TestReview_P6_*`, `TestBackoffIsExponentialAndCapped` |
| Re-dial | A lost connection is re-dialled by exactly ONE caller; concurrent callers wait for it (8 concurrent calls after one loss open one new connection). A client the caller disconnected is never re-dialled, also not by a re-dial that was already in flight when `Disconnect` ran; an explicit `Connect` that is still establishing its connection when `Disconnect` returns fails with `ErrDisconnected` (not transient) and closes that connection instead of installing it. `TestConnection` on a client the caller connected (also one whose connection was lost) uses and repairs that connection and leaves it connected; on a client that is not connected it uses a connection of its own, closes it, and does not change the client's state. | `TestReview_P3/P4/P10`, `TestConcurrentCallsAfterLossAreRaceFreeAndSucceed`, `TestWF24D_V03_*`, `TestWF24D_V04_*`, `TestWF24D_V18_*`, `TestDisconnectDuringConnectWins` |
| Malformed replies | Every reply frame from the server is parsed completely, with bounds checks, BEFORE `pkg/sftp` reads it (`frame.go`). A frame that does not follow the SFTP v3 reply grammar ends the connection and fails the call with `ErrMalformedReply` (not retried). This is what keeps a hostile or broken server from panicking the process: `pkg/sftp` v1.13.11 parses several replies with unchecked readers, some inside goroutines it starts itself, where no `recover` of this package could reach. A `recover` boundary in the calling goroutines is a second layer. | `TestValidateFrame_Table`, `TestFrameReader_*`, `TestRaw_MalformedData*`, `TestReview_P7_*` |
| End of file vs dead connection | `pkg/sftp` renders a STATUS `SSH_FX_EOF` and the write error of an already closed channel both as `io.EOF`. After an `io.EOF` (and after a clean `WriteTo`) the client asks the connection with one keepalive round trip whether it is alive - ONE round trip per file end, bounded by `KeepAliveTimeout` (default 15 s), the same bound as the periodic keepalive; the dead channel is checked first, without a round trip. A dead connection is reported as a lost connection, never as the end of the file. The same holds for the wrapped `io.EOF` that `pkg/sftp` returns when a request cannot be sent on a closed channel: `Seek` reports a lost connection, `Close` of such a handle is `nil` (deterministic, 40 of 40 iterations). | `TestReadOnADeadConnectionIsNeverAnEOF`, `TestReview_P3_*`, `TestWF24_N4_*`, `TestEndOfFileLivenessFollowsKeepAliveTimeout`, `TestLivenessAccessors`, `TestFile_CloseAndSeekOnADeadConnectionAreDeterministic`, `TestWF24D2_V09_*` |

## Cancellation and dead links (measured)

* `Connect(ctx)` honours `ctx` and `DialTimeout` through the TCP connect, the SSH handshake, the sftp subsystem start and the resolution of the root
  (closing the TCP connection ends any blocked read). `TestContextCancelEndsTheHandshake`, `TestDialTimeoutBoundsTheHandshake`, `TestReview_P2_*`.
* Every call that can block on the server (`GetFileInfo`, `FileExists`, `ListDirectory`, `ReadFile`/`OpenSeekable`/`ReadRange` open, and `Read`, `WriteTo`, `Seek`,
  `Close` of the returned reader) runs under one guard. When its context ends, the call gets `CancelGrace` (default 1 s; negative: none) to finish by itself;
  if it has not, the ssh connection is closed, which ends the call, and the call returns the context error. Measured with servers that never answer STAT, READDIR,
  READ or CLOSE: the call returns within `grace + a few ms` of the context ending (the review's probes use 3 s as the bound).
* CONSEQUENCE, stated plainly: a client holds ONE ssh connection. When the guard closes it, every other call and stream on that connection fails with a lost
  connection too (request/response calls are re-dialled once, single flight; a stream in the middle of a copy is not resumed). The grace period exists so that
  this happens only for a call that is really stuck: cancelling one stream on a healthy server does not disturb the others
  (`TestCancelOfOneStreamDoesNotKillTheSharedConnection`: 8 MiB stream A finishes byte-identical while stream B is cancelled; one connection in total). A call that
  returns by itself with the context error inside the grace - the typical `ListDirectory` with a deadline on a slow but healthy server - leaves the connection alone:
  `TestWF24_N1_*` (a 4 MiB stream finishes byte-identical, one connection, while a listing ends by its deadline), `TestWF24D_V06_*` (the armed grace timer is disarmed when the call returns),
  `TestWF24D2_V07_*` (a negative `CancelGrace` closes at once).
* An open file is closed on the server when its context ends, whether or not the caller closes it (`TestContextEndClosesTheRemoteFile`). `Close` of a file is bounded by
  `CloseTimeout` (default 10 s): after it the connection is closed, because a CLOSE request on a stalled server cannot be abandoned otherwise.
* `ListDirectory` stops at the context between server round trips even before the grace ends (`TestListDirectoryStopsAtTheContextOnASlowServer`).
* An SSH keepalive (`keepalive@openssh.com`, every `KeepAliveInterval`, default 30 s; negative disables) closes a connection whose peer stopped answering within
  `KeepAliveTimeout` (default 15 s; the one liveness bound of the client, also used after an end of file), so a half-open TCP link (NAS reboot, Wi-Fi drop, NAT expiry) ends calls that carry no deadline. Measured through a TCP proxy that
  swallows all traffic: the blocked call returns after about interval + timeout (`TestKeepaliveEndsACallOnAHalfOpenLink`, 100 ms / 300 ms). With the keepalive disabled the same call
  stays blocked (`TestKeepaliveCanBeDisabled`).
* `DiscoverAll`/`Discover`/`Pin` honour their context after the TCP connect (`TestDiscoverAllHonoursContextAfterConnect`).

## Host key pinning (trust on first use only with the owner's explicit confirmation)

* A host with NO pin is refused with `*UnknownHostKeyError`, which carries the fingerprint the server presented. No credential is sent
  (the handshake is refused in the host key callback, before authentication).
* A host whose key matches none of its pins is refused with `*HostKeyMismatchError`. A changed key is never accepted automatically.
* `sftp.Pin(ctx, store, host, port, sftp.Confirmation{Owner, Fingerprint})` is the only way to create a pin. It re-reads the keys the server holds
  (`DiscoverAll`: one aborted handshake per key family, no authentication), and records a pin only if one of them has EXACTLY the fingerprint the owner
  confirmed (`SHA256:...`, as `ssh-keygen -lf` prints it). A missing owner, a missing fingerprint or a different fingerprint writes nothing
  (`ErrPinNotConfirmed`).
* Pins live in a `PinStore`: `MemPinStore`, or `FilePinStore` (JSON, mode 0600, atomic rewrite: temp file, fsync, rename, fsync of the directory; a corrupt file is an error, never
  "no pins"). `FilePinStore` refuses to read or write (`ErrPinStoreInsecure`) a pin file or directory that another user could have changed: group/world-writable file,
  group/world-writable directory (a sticky directory is accepted), a pin file that is not a regular file (a symlink), or an owner other than the current user or root.
  Changes are serialised between goroutines by a mutex and between processes by an `flock` on `<file>.lock` (`TestFilePinStore_*`). The directory fsync after the rename is not
  observable by a test and is stated here as code, not as a tested property.
* `sftp.DefaultPinStore` is the store the factory-built clients use; it is nil until the application installs one, so a factory-built client refuses
  every host until then (`ErrNoPinStore`). It is followed at CONNECT time (`sftp.DefaultPinStoreRef()`), not captured when the client is created, so a store installed after the
  client was created is used (`TestDefaultFactory_SFTP_FollowsAPinStoreInstalledAfterTheClientWasCreated`); with a store holding the pin, a factory-built client connects and lists end to end
  (`TestDefaultFactory_SFTP_UsesTheInstalledPinStoreEndToEnd`).
* The pinned key type restricts the host key algorithms the client offers (as OpenSSH does for known_hosts), so a server holding several keys
  presents the pinned one.

## Credentials

`credential_ref` is an opaque name. The default resolver (`EnvCredentialResolver`, prefix `SFTP_CRED`) reads, for ref `NAS_1`:
`SFTP_CRED_NAS_1_PASSWORD`, `SFTP_CRED_NAS_1_PRIVATE_KEY` (PEM), `SFTP_CRED_NAS_1_PASSPHRASE`. The ref is used exactly as written, case included, and may
contain only letters, digits and `_`; any other character is refused (`ErrCredentialUnavailable`). The mapping from ref to variable names is therefore one to one: a secret
set for one ref cannot be read through another (`nas-1`, `nas.1`, `NAS.1` no longer fold onto one name, and `nas_1` does not read `NAS_1`). A different resolver can be
set per client (`Config.Resolver`) or globally (`sftp.DefaultCredentialResolver`).

Zeroing is best effort and is stated as such: the client overwrites the key and passphrase bytes the resolver returned after use (`TestClientWipesWhatTheResolverHandedOver`;
the resolver MUST return fresh slices on every call), but the password is a Go string: it can only be dropped, not overwritten, the `ssh` library keeps a copy of it in
its auth method for as long as the attempt needs, and the parsed private key lives on in the signer. Do not read this as a guarantee that no copy of the secret remains in memory.

## Factory settings (protocol `sftp`)

`host`, `port` (default 22; a number from 1 to 65535, anything else is an error and not a silent 22), `username`, `credential_ref`, `path` (or `root`: the confinement root,
default `/`; an empty `path` does not win over `root` and never widens the root). These six are the ONLY keys read; any other key is REFUSED (it would otherwise sit in the stored
settings and be ignored). A key that looks like a secret - compared case-insensitively with `-`, `_`, `.` and spaces removed, containing `pass`, `pwd`, `secret`, `token`, `key`,
`cred` or `auth` (`password`, `Password`, `passwd`, `pwd`, `private-key`, `ssh_key`, `credentials`, `api-token` ...) - is refused with the message "use credential_ref" and without the value.
`TestDefaultFactory_SFTP_*`, `TestWF24_N10_*`.

## Configuration limits

* `Config.MaxPacket` above 32768 (`MaxPacketLimit`) is refused at connect with `ErrInvalidConfig` before any traffic (`pkg/sftp` refuses packets above 32 KiB, which used to fail every connect with an
  opaque error); it is never retried (`TestMaxPacketAboveWhatPkgSftpAcceptsIsRefusedUpFront`).
* `Config.PinStore` is required. The factory passes a store that follows `sftp.DefaultPinStore` at connect time (above).
* `MaxConcurrentRequests x MaxPacket` is the pipelining window; the ranged-read window and the read-ahead are capped at 4 MiB (`TestReadWindow`).

## Tests

* Unit (no mocks beyond unit; a REAL in-process ssh server with the real `pkg/sftp` request server behind it, and a raw sftp peer that scripts exact reply bytes):
  `go test ./pkg/sftp/ ./pkg/factory/`.
* Integration against a real OpenSSH (`internal-sftp`, chroot, read-only data volume) in a rootless container: `scripts/test-infra/sftp_fixture.sh run -- -race -v ./pkg/sftp/ ./pkg/factory/`.
  The integration tests are behind the `integration` build tag and FAIL (never skip) when the fixture variables are missing. The wire validator is the part of the
  client most exposed to real servers; the OpenSSH run is what shows it accepts real replies (listing, stat, read, close, realpath, attributes with extensions).
* Both run only inside rootless containers (the pinned `IMG-GO` image through `scripts/containers/run_pinned.sh`).
* Mutation: `evidence/wp12/sftp/fix-r3-mutants.py` runs the 21 round-1 reviewer mutants, the author mutants of fix-r2, the round-2 reviewer's mutants V* and one mutant per fix-r3 mechanism and per
  defect put back (re-targeted onto the final code), with a negative control that must pass and one that must survive; the fixture script has its own mutants (`fix-r3-gate-test-mutants.txt`).

## Honest limits (UNCONFIRMED / not done / by design)

* TOCTOU: the containment check (`RealPath`) and the following open or stat are two requests. A server-side symlink swapped between them is followed. This client cannot close that window.
* SFTP v3 attributes are flag-gated and the client cannot tell "absent" from "zero" for them: an absent size or permission reads as `0` (a regular 0-byte file). An absent modification time is the zero
  `time.Time` (unknown), and a real mtime of exactly 1970-01-01T00:00:00Z reads as unknown too (`pkg/sftp` reports it as `Unix(0)`, the same value).
* Cancelling a call closes the shared connection once the grace has passed (see above). Streams in flight on it are not resumed.
* A successful `ListDirectory` is also refused if the connection is already known dead when it returns; a connection that dies in the few microseconds between the last reply and that check
  is not distinguishable by this client from a clean end.
* The `MaxDirEntries` budget is per listing (per directory handle); a listing is one handle opened and closed by `pkg/sftp`. The accounting depends on the request ids this client sends; a server that
  answers a READDIR with a NAME reply of another id is simply not counted (and `pkg/sftp` rejects the mismatch).
* The read-ahead trades memory and bytes beyond the reader's position (up to one read window, default 2 MiB per sequentially read open file) for pipelining; a consumer that opens many files and reads
  each one sequentially pays that per file. Set `MaxConcurrentRequests` / `MaxPacket` lower to shrink it.
* Every end of a file costs one global-request round trip (the liveness question). On a link whose answer takes longer than `KeepAliveTimeout` the read is reported as a lost connection and the shared
  connection is dropped; raise `KeepAliveTimeout` for such a link. Whether Synology DSM answers `keepalive@openssh.com` quickly under load is UNCONFIRMED (no NAS contacted).
* Password login is attempted as `password` and `keyboard-interactive`. Whether Synology DSM needs `keyboard-interactive`, and its `MaxSessions` / `MaxStartups` / `@eaDir` behaviour, are UNCONFIRMED: no NAS was
  contacted. The only real peer is OpenSSH 8.4p1.
* Behaviour against a server whose `REALPATH` and `OPEN` disagree (different symlink-depth limits, transient `REALPATH` errors) is tested only with a scripted server; with such a server this client refuses (fails
  closed) rather than reads.
* The step from the `x/crypto/ssh` error to `*AuthError` is text based (below); everything after it - `pkg/fabric`, the pool latch - is typed (`errors.Is(err, fabric.ErrAuth)`).
* An authentication failure is recognised from the text of the `x/crypto/ssh` error (`unable to authenticate`, and `unexpected message type 51 (expected 60)` for a server that advertises
  keyboard-interactive and answers its start with a plain failure, found against a real OpenSSH after a wrong password). `x/crypto/ssh` has no typed error for it; a future wording change would turn an `AuthError`
  into a handshake error (which is not retried either, but is not an `AuthError`).
* The wire validator proves that `pkg/sftp` can read a reply without reading past its end; it does not judge whether a reply is plausible (a server may still lie about sizes or contents), and unknown packet types are refused.
  The validator buffers one frame (at most 256 KiB, the `pkg/sftp` limit) at a time.
* The reply validator and the recover boundary protect against panics in the reply parsers. They do not make `pkg/sftp` safe against a server that holds a pinned key and misbehaves in other ways (slow replies, large
  listings are bounded as above; endless NAME frames are bounded by `MaxDirEntries` only while a listing is in flight).
* `FilePinStore` uses `flock` and the unix `stat` owner: the package builds and runs on unix systems only (the project's targets); it is not built for Windows.
* No benchmark of absolute throughput was run (pipelining is shown by concurrency, not by MB/s). The wire validator copies every reply frame once more on its way to `pkg/sftp`; the cost of that copy is UNMEASURED.
* Connection multiplexing (several sftp sessions on one ssh connection) is not implemented; one ssh connection and one sftp session per client.
* A name that `pkg/sftp` reduces with `path.Base` can still differ from what the server meant (a name containing `/` arrives as its last element); only the shapes listed above are dropped.
* `catalog-api` is not wired to the new client here (`go.mod` of catalog-api is untouched); see the evidence README for the module bumps it will need.

## Corrections (statements of revision 2 that the round 2 re-review refuted by measurement, and what is true now)

| Revision 2 said | Measured (WF24) | Revision 3 |
|---|---|---|
| every sentence in the Behaviour sections is pinned by a test that fails when it stops being true | 11 reviewer mutants on the new mechanisms survived (handshake deadline, default keepalive, single-flight waiters, `TestConnection` repair, grace timer, negative grace, short write, `Close` mapping, window cap, re-`Connect` leak, nested listing `Path`) | every one has a test (the reviewer's discriminators are adopted verbatim: `TestWF24D_*`, `TestWF24D2_*`), `fix-r3-mutants.py` re-runs them |
| cancelling one stream on a healthy server does not disturb the others; the grace period exists so that this happens only for a call that is really stuck | a `ListDirectory` ending by its own deadline on a healthy server dropped the connection and failed a 4 MiB stream | a call that returns by itself leaves the connection alone (`TestWF24_N1_*`) |
| the two predicates agree (every connection loss is transient) | false for `context.DeadlineExceeded` | context errors are neither; the whole table is asserted |
| concurrent huge listings can exhaust the budget together | small sequential listings exhausted it under overlap and failed unrelated calls | per-listing budget, only that listing fails |
| after an `io.EOF` the client asks the connection with one keepalive round trip | the round trip had an undocumented hard 3 s bound that reported live slow links as dead | bounded by `KeepAliveTimeout`, documented |
| HTTP Range streaming is pipelined too | not for `http.ServeContent` over `OpenSeekable`, the consumer's path | read-ahead in `file.Read` (measured) |
