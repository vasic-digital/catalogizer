# fix-r2: convergence assessment for pkg/sftp (constitution 11.4.276), written BEFORE the fix

| Field | Value |
|---|---|
| Written | 2026-10-07, before any edit of `submodules/filesystem/pkg/sftp` for this round |
| Input | independent review `WF19-REVIEW-sftp.md` (verdict NO-GO: 1 HIGH + 7 MEDIUM source defects, 2 test-instrumentation, 3 process-doc findings; 12 probes, 21 reviewer mutants) |
| Pre-fix tree | sha256 of the six sources recorded in `fix-r2-pre-sha.txt` (equal to the review header) |
| Round | this is review round 1 of the reviewed package (R_max not reached); the fixer is a Sonnet single pass |

## Is the single pass enough? Classes and the one mechanism that closes each (all members, not instances)

| Class (review section 4) | Members (enumerated by the review) | Closing mechanism (class-wide) |
|---|---|---|
| A. server round trip not bounded by ctx / link | 12 call sites in `sftp.go` (NewClient, RealPath x2, Stat x2, Read, WriteTo, Seek, Close, Open, ReadDirContext) | ONE guard `Client.guard(ctx, conn)` around every blocking call (`with` for the 6 request/response calls, `file.call` for Read/WriteTo/Seek/Close); `dial` keeps the cancel hook + deadline until NewClient AND RealPath(root) are done; an SSH keepalive closes a dead link. No call into pkg/sftp remains outside one of the three wrappers. |
| B. connection-state machine | 4 state writers (dial install, Connect failure, Disconnect, with re-dial) | ONE owner `Client.cur` + single-flight `acquire`; install is conditional on the state still being the one the dial started from (a `Disconnect` sets "down"); `TestConnection` never touches the caller's state. |
| C. error classification | `isConnectionLoss(io.EOF)`, `IsTransient(io.EOF)`, net.Error breadth | ONE classifier pair with the invariant `isConnectionLoss => IsTransient`, a server STATUS reply (EOF) is wrapped as non-transient inside `with`; tested as a property over a table. |
| D. fail-open security checks | `checkWithin` RealPath error, `rootReal == ""` | `checkWithin` fails closed on both; not-exist is returned as not-exist (the op reports it). |
| E. reply parse panics | 30 unchecked parse sites in 10 pkg/sftp functions, some in pkg/sftp-owned goroutines | not per site: ONE wire validator (`frame.go`) parses every reply frame completely before pkg/sftp reads it, plus a recover boundary in the calling goroutines as a second layer. |
| F. path representation | `confine`, `root()` rewrite `\` | no separator translation anywhere; paths the server produced are opaque. |
| G. untestable claims | 16 reviewer-mutant survivors + 5 named tests | every reviewer probe adopted as a test; each surviving mutant gets a killing test; reviewer mutants re-targeted onto the new code by `fix-r2-mutants.py` and run with a negative control. |
| H. doc vs measured | 7 claims | docs rewritten to the measured behaviour; limits (TOCTOU, absent attributes, epoch mtime, case/charset of refs, shared-connection consequences of cancellation) stated. |

Low findings (SFTP-12..19) are closed in the same pass: keyboard-interactive auth, factory settings hygiene, listing entry cap, ranged-read pipelining, env credential name injectivity, pin file integrity, protocol semantics doc, evidence bookkeeping and fixture tree hash.

## Structural-round triggers (11.4.276(E))

* Two or more findings sharing a named class with a non-MINOR member: classes A, B, C, E each do. That is exactly why each is closed by one mechanism and not per probe.
* Ground truth (11.4.276(B)): the unowned external system is `pkg/sftp v1.13.11` (and OpenSSH). The review read its source; this round re-read the call sites (`client.go` Close/Read lock ordering, `conn.go` recv loop, worker goroutines of the pipelined read at `client.go` ~1280) before choosing the wire validator. The real-composition fixture is the OpenSSH run through `scripts/test-infra/sftp_fixture.sh`.
* Reappearing finding: none (first fix round for this review).

## Threat boundary (declared, not narrowed)

Hostile or buggy pinned server (malformed replies, stalled replies, symlink tricks, huge listings), network faults (half-open TCP), concurrent callers on one client. Out of scope and stated as such: a compromised pin store file owner, a server holding the pinned key that behaves perfectly while serving other data (TOCTOU between RealPath and Open), absent protocol attributes (v3 has no way to say "size unknown").
