# fix-r2 convergence assessment (constitution 11.4.276), written BEFORE the fixes

| Field | Value |
|---|---|
| Item | WP-12 PA-06 `submodules/filesystem/pkg/nfs3`, `test/nfs3fixture`, `docs/NFS3_CLIENT.md` |
| Review input | `WF19-REVIEW-nfs3.md`: independent one-pass review, verdict NO-GO (2 HIGH, 6 MEDIUM, 8 LOW source defects, 2 INFO, 5 test, 5 documentation findings) |
| Round | review round 1 of at most 5 to 7; this is the single fix pass for it (a re-review follows by the conductor) |
| Fixer | Sonnet, one pass, 2026-10-07 |

## Ground truth (11.4.276 B)

Everything here depends on the NFSv3 server's behaviour, which this repository does not own, so the model was taken from the sources, not
invented: RFC 1813 (JUKEBOX, READDIRPLUS cookie and verifier, rtmax), RFC 5531 (fetched this session: AUTH_NONE is section 10.1, the
AUTH_SYS `SHOULD NOT` sentence is section 14, section 5 allows xid reuse on retransmission), Linux knfsd (`nfsd_setuser_and_check_port`
answers an insecure port with `nfserr_perm`, quoted by the reviewer), and willscott/go-nfs v0.0.4 source (read this session: it parses the
call header with go-nfs-client's XDR reader and ignores the credential body; `ReadOpaque` of go-nfs-client does not consume padding, found
while building the oracle and worked around by reading the machine name as an XDR string). Every fix is checked against the reviewer's own
fixtures (probes P1 to P17 and killers A01 to A18, adopted verbatim) and against a real independent server (go-nfs), not only against the
author's fake.

## Defect classes and their members (11.4.276 C)

Every class below was enumerated with a control-needled instrument, not fixed instance by instance.

| Class | Members (reviewer ids) | Census / closure |
|---|---|---|
| C1 Server-driven loop without progress, or amplification of server work | S1 listing, S2 read-ahead on short ranges and early close, S3 short reads, S8 (a vanishing entry could keep a listing alive without growing it, found while closing S1) | loop census of the non-test sources: 28 `for` statements; needle `for frags := 0` found (1), negative control 0. Each one is bounded by its input, an explicit cap or demonstrated progress; the unbounded one (`listOnce`) is now bounded by four independent guards (cookie advance, repeated request cookie, dot-only pages, page cap), each with its own test and mutant. Read-ahead amplification is bounded by an adaptive ramp, a range limit and the rtmax and record clamps. |
| C2 Sticky or stale state surviving a failure or a change | S4 reader error, S6 handle cache, S7 privileged flag, S15 root handle, S17 xid sequence | every piece of mutable client state was listed (privileged, authNone, cache, root, fsinfo, xid, nfs conn, reader q/next/err/buf/cur/cw); each is either reset on its failure path, bounded in time (cache TTL), or documented. |
| C3 Connect-time error handling | S5 timeout arithmetic, S11 missing UMNT, S12 missing source port | all return paths of `connectOnce` after MNT now pass one deferred UMNT; the budget is derived from one function used by every caller. |
| C4 Unvalidated server or configuration number | S10 size at or above 2^63, S16 limits, S5 negative retries, S18 default identity | every `int64(...)`/`int(...)` conversion of a server value was listed (grep census; needle `int64(a.Size)` found); the size is rejected in the decoder, the only entry point; configuration limits are validated in `New`. |
| C5 Context not honoured | S14 blocked write, and the wait for the write slot (found while closing S14) | the three blocking points of `rpcConn.call` (slot, write, reply) all select on the context. |
| C6 Protocol-conformance of waiting | S13 JUKEBOX schedule | separate schedule, seconds-scale, context-bound. |
| C7 Names the path layer refuses but the list layer accepted | S9 NUL (and `/`, empty already) | one predicate. |
| C8 Test-side: tests that pass for the wrong reason / guards that cannot fail | T1 to T5 | every reviewer mutant and killer adopted; each flagged weak test rewritten so its own assertion is the only thing that can fail; a run-time allow-list at the single choke point replaces the name-based guard as the real barrier; an independent AUTH_SYS decoder in the fixture. Layers of every defence in depth got their own test so that removing one layer alone is detected (A08/remount, A10/rtmax, S4 two layers, S7 two layers, S1 four guards). |
| C9 Docs and evidence that claim more than was measured | D1 to D5 | every claim re-read against the measured behaviour; RFC citations re-checked against the RFC text. |

## Convergence assessment

* Is the approach converging? Yes: the first review found no protocol-encoding defect (wire formats verified field by field), only
  behaviour at the edges and test strength; all findings sit in the classes above and the fixes do not change an encoding.
* Same-class recurrence risk: the classes C1 and C2 are where a second review would look. Mitigation: each guard has an individual test
  and mutant (R01a to R01d, R04a/b, R07a/b), and the reviewer's probes pin the numbers (61,614 and 67,739 RPCs in 3 s before; 2 RPCs after).
* Structural round needed? No. The scope and threat boundary are those of the review (an untrusted or buggy NFSv3 server, a hostile
  network peer, a slow server); nothing was narrowed.
* UNCONFIRMED (stated, not hidden): real-NAS behaviour of JUKEBOX timing, rtmax and reserved ports; the blocked-write test depends on the
  kernel's socket buffers being smaller than 32 MiB (its control proves the write blocks on this host); S15 remount is tested against the
  in-process server only.
