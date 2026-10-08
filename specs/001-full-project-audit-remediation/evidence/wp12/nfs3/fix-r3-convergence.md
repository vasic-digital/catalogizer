# fix-r3 convergence assessment (constitution 11.4.276), written BEFORE the fixes

| Field | Value |
|---|---|
| Item | WP-12 PA-06 `submodules/filesystem/pkg/nfs3`, `test/nfs3fixture`, `docs/NFS3_CLIENT.md`, `docker-compose.test-infra.nfs3.yml` |
| Review input | `WF24-REVIEW-nfs3.md` (independent re-review, round 2, NO-GO: 1 MEDIUM + 4 LOW + 3 INFO source defects, 2 MEDIUM + 3 LOW + 1 INFO test findings, 6 LOW documentation findings), plus the real-NAS finding in `../nas-clients/nfs-*.json` |
| Round | review round 3 of at most 5 to 7 (round 1 = WF19, round 2 = WF24); this is the single fix pass for round 2 |
| Starting point | committed code: `submodules/filesystem` 83c0ac1 (main e9d6883d); this pass leaves its changes uncommitted, the conductor commits |
| Fixer | Sonnet, one pass, 2026-10-08 |

## Why round 3 is a structural round (11.4.276 E)

WF24 reports that two classes recurred AFTER a fix pass: C5 (context: S14, then W3 and W7) and C2 (stale state: S6, then W2). Two findings of
one class with at least one non-MINOR is the trigger of 11.4.276(E)(2), and the reviewer's own reading was "not strictly met" only because
W1 is alone in its class this round. The conservative reading is taken: this pass does not add a fifth point fix, it re-derives the model
of each recurring class from the code and the RFC and closes every member of the class, replacing the mechanism where a point fix would
only have layered on the old one.

## Ground truth (11.4.276 B)

* RFC 1813 3.3.3: `LOOKUP3args` is `diropargs3 { dir, name }`; the ONLY file handle in a LOOKUP is the directory. NFS3ERR_STALE or
  NFS3ERR_BADHANDLE answering a LOOKUP therefore refers to the DIRECTORY. In GETATTR the only handle is the object asked about; after a
  LOOKUP that just returned that handle, STALE there refers to the entry. This is the model the WF19 fix direction for S8 got wrong
  ("skip NOENT and STALE" without saying whose handle).
* RFC 5531 / NFS v4 (RFC 7530): v4 procedure 1 is COMPOUND, which carries WRITE/REMOVE/RENAME operations, and shares its number with v3
  GETATTR. A procedure number identifies an operation only together with the program AND version.
* Go `net.Conn` contract: `Write` returns the number of bytes accepted; a write that returns `n == 0` with a deadline error sent nothing, so
  the byte stream is intact (there is no half record). `net.Pipe` implements deadlines with that contract and is used for the deterministic test.
* The model of the WF24 probes (W1, W2, W3, W4, W7) and killers (N01 to N17) is adopted verbatim; they were built from the same sources.

## Defect classes and their members (11.4.276 C)

Each census below was run with a control needle (`tools/fixr3_census.sh`, output `fix-r3-census.txt`).

| Class | Members (WF24 ids) | Census of the class over the non-test sources | Closure |
|---|---|---|---|
| A Error attribution per procedure (whose handle does STALE/BADHANDLE/NOENT refer to) | W1 | every site that interprets NFS3ERR_NOENT/STALE/BADHANDLE: `resolve` (STALE of the walk: a cached directory handle or the root: drop cache, remount: correct), `vanishedErr` + its two callers in `listOnce` (LOOKUP leg: STALE is the DIRECTORY: wrong; GETATTR leg: STALE is the entry: right), `NFSError.Is`/mapping in proto.go (no recovery), reader.go and `Access` (no interpretation) | `vanishedErr` takes the leg: LOOKUP leg = NOENT only; GETATTR leg = NOENT/STALE/BADHANDLE. A STALE/BADHANDLE from the LOOKUP leg fails the listing with the NFS error. |
| B Cache validity is a chain property | W2 (+ the eviction/orphan member found by the census, + the expiry member of S6) | every writer/reader of `c.cache`: `cached`, `remember`, `forgetCache`, `remount`, `connectOnce` reset, `listOnce` (via `remember`), tests that replace the map | An entry records the generation of the parent entry it was learned under (`pgen`); `cached` accepts an entry only when its own TTL holds AND its parent entry exists with that generation AND that parent is valid (recursively to the root). A handle change allocates a new generation, so children of the old tree can never be reached, whatever order the entries expired in. The eager purge on a handle change is kept (memory). |
| C Context honoured at every blocking point, and what one caller's cancellation does to others | W3, W7, I2, plus the members found by the census: `remountMu.Lock()` (not context-aware) | every blocking construct of the non-test sources (`<-`, `Lock`, `Write`, `ReadFull`, dial, timers, `context.Background`): the census lists every line (`fix-r3-census.txt`); the mutex sites (`c.mu`, `r.mu`) hold no I/O and no wait, the channel, dial, read and write sites each select on a context or are bounded by a timeout or a close, and the two that were not (the `Background` UMNT, the `remountMu` mutex) are the members fixed here | W3: the failure-path UMNT keeps a short bound even when the caller's context ended (`WithoutCancel`, bound 500 ms), instead of CallTimeout on `Background`. W7: a cancelled caller whose interrupted Write sent 0 bytes returns the context error and keeps the connection; the connection is failed BEFORE the write slot is released when it must fail. I2: each READ future has its own context, cancelled when the queue is dropped (Seek, error, EOF). `remountMu` becomes a context-aware semaphore. |
| D Allow-list key is the full procedure identity | W4 | `allowedCalls` (11 entries) and every `rpcConn.call` site (AST census: 2 sites, `oneShot` and `callNFS`, the version argument of each pinned by `TestWF24KillN17CallSitesPinTheVersion`) | key = `(prog, vers, proc)`; N17 killer adopted. |
| E Cleanup after a successful MNT | I1 (+ W3 above) | every return path of `connectOnce` after the MNT call | the cleanup is registered once the MNT status word reads OK (before the body is decoded). |
| F Unvalidated configuration | I3 | every `Config` numeric field (census) | `MaxPipeline` bounded, `DirCount > DirMaxCount` refused in `validate`. |
| G Real-NAS finding | `Connect` with `TryPrivilegedPort` discards the original refusal | the two return statements of `Connect` | both errors are returned (original first) unless the second is itself the server's access refusal. |
| T Test instrumentation | TN1 (9 survivors), TN2 (image build), TN3 (`/dev/tcp` under dash), TN4 (uid = gid oracle), TN5 (no executed reserved-port loop), TN6 (indirect kill) | the WF24 mutant set N01 to N18 | all 9 killers adopted verbatim; N15 gets an injectable bind (`dialPrivilegedFrom`) so the loop is executed; the fixture checks distinct uid/gid and the gid VALUES; `Containerfile` copies every `.go` file; `run_integration.sh` is `bash`; N14 asserted directly. |
| P Docs and evidence that claim more than was measured | DN1 to DN6 | every status/claim sentence of `docs/NFS3_CLIENT.md` and the evidence README | rewritten against the measured behaviour; status headers name the commit. |

## Convergence assessment

* Is the approach converging? Yes, with a qualification. No protocol-encoding defect was found by either independent review (round 1 and 2
  verified the wire format field by field, the go-nfs leg passes). The defect rate is falling (WF19: 2 HIGH + 6 MEDIUM; WF24: 1 MEDIUM + 4
  LOW) and the remaining findings sit in two classes (B, C) that this pass re-derives instead of patching. The qualification: the reviewer's
  probe W7 is probabilistic; the fix is therefore tested by a deterministic `net.Pipe` test as well.
* Same-class recurrence risk after this pass: B (the chain rule is a property, not a list of cases; the test enumerates expiry order,
  eviction and handle change), C (the census names every blocking point; the new member found by the census, `remountMu`, shows the list
  was incomplete before), A (the leg is now a parameter of `vanishedErr`, so a third leg cannot silently reuse the wrong rule).
* Structural round needed? This IS the structural round (see above). Scope and threat boundary unchanged: an untrusted or buggy NFSv3
  server, a hostile network peer, a slow server, concurrent callers on one client. Nothing was narrowed.
* UNCONFIRMED (stated, not hidden): real-NAS behaviour of JUKEBOX timing, rtmax and reserved ports (the NAS leg found host 1 refusing MNT
  and hosts 2-7 refusing by address; nothing here contacted the NAS); the real reserved-port bind (the pinned container refuses it, the
  loop is tested through an injected bind); whether a real NAS omits READDIRPLUS attributes (the W1 trigger).

## Corrections made after the assessment was written (stated, not hidden)

* The census counts in the class table (call sites, blocking points) were first written as estimates before the census had been run; they were
  replaced by the measured numbers of `fix-r3-census.txt`. The classes, the decisions and the order of work are unchanged.
* While building the mutants two pieces of defensive code turned out to be equivalent (a wrapper branch in `listOnce` whose error path was
  identical, and a `min()` with `CallTimeout` that `oneShot`'s own call timeout already implies); they were removed from the source instead of
  being defended with a test (F02 and F29 are therefore absent from the mutant set). The cache needed less than first designed: the eager purge
  of descendants and the "skip a path whose parent is unknown" rule are redundant given the generation chain and were removed for the same reason.
* At 83c0ac1 the class-C census found 1 `context.Background` (the cleanup UMNT) and 3 uses of the non-context-aware `remountMu`; after this pass it finds 0
  and 0 (`fix-r3-census.txt` is the after-state; the before-state was read from the same script run on the committed tree and is not kept as a file).
* The write-slot ordering of W7 (fail the connection BEFORE releasing the slot) is a race: the first mutation run killed its mutant (F17) in 3 of 5 runs only.
  Instead of defending a probabilistic kill, `rpcConn` got a test hook (`failHook`, called at the start of `fail`) that holds the failing writer where a
  released slot would let a waiting caller write; the kill is now deterministic.
