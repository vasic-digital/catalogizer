# NFSv3 user-space client (`pkg/nfs3`)

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-07 |
| Last modified | 2026-10-08 |
| Status | the revision-2 state is COMMITTED: `submodules/filesystem` 83c0ac1 (main repository e9d6883d). Revision 3 (this text) describes the third fix pass, which closes the independent re-review `WF24` and sits as an UNCOMMITTED working-tree change on top of 83c0ac1 when this was written (the conductor commits it; own-org submodule). Not yet wired into the factory. See "Review history" |
| Task | WP-12 PA-06 (`specs/001-full-project-audit-remediation/evidence/wp12/protocol-analysis/REPORT.md` sections 2.3, 3, 4) |
| Evidence | `specs/001-full-project-audit-remediation/evidence/wp12/nfs3/` |
| Licence | own code, Apache-2.0 |

A minimal, pure-Go, **read-only** NFSv3 client. No kernel mount, no root, no cgo, no third-party NFS code. It replaces the
`syscall.Mount` path of `pkg/nfs` for catalog crawling (that path needs `CAP_SYS_ADMIN`, cannot run in a rootless container and
leaves stale mounts after a crash).

## What it speaks

| Layer | Spec | Implemented |
|---|---|---|
| ONC RPC over TCP, record marking, xid matching, AUTH_SYS and AUTH_NONE | RFC 5531 sections 8, 9, 11, appendix A | yes |
| XDR | RFC 4506 | yes (strict bool/enum, bounded variable-length items) |
| Port mapper v2 | RFC 1833 section 3, `PMAPPROC_GETPORT` | yes (TCP only) |
| MOUNT v3 | RFC 1813 appendix I | `NULL` (server-side only in tests), `MNT`, `UMNT`, `EXPORT` |
| NFS v3 | RFC 1813 section 3 | `NULL`, `GETATTR`, `LOOKUP`, `ACCESS`, `READ`, `READDIRPLUS`, `FSINFO` |
| NFS v3 write and other read procedures | RFC 1813 | **not present**: `SETATTR`, `WRITE`, `CREATE`, `MKDIR`, `SYMLINK`, `MKNOD`, `REMOVE`, `RMDIR`, `RENAME`, `LINK`, `COMMIT` have no encoder and no procedure constant; `READLINK`, `READDIR`, `FSSTAT`, `PATHCONF` are not used |
| NFSv4, Kerberos (`RPCSEC_GSS`), NLM locking, NFS over UDP | | no |

Read-only is enforced three ways: (1) at run time, `rpcConn.call`, the single place where a request is written, only sends
(program, VERSION, procedure) triples on an allow-list and returns `ErrReadOnly` for anything else before a byte leaves (a new method that
names a write procedure, or a version of a program in which the same procedure number means something else, by constant, literal or
variable, is refused there; the version matters because NFSv4 procedure 1 is COMPOUND, which carries writes, while v3 procedure 1 is
GETATTR); (2) `TestNoWriteProcedureCompiledIn` parses the package source and
requires the set of `proc*` constants to be exactly the table above, and `TestKillA11OnlyAllowListedProcsReachTheWire` requires every call
site to pass an allow-listed identifier; (3) the test servers count every procedure they receive (zero write-class calls over a full
workflow). The five mutating methods of `client.Client` return `nfs3.ErrReadOnly` without sending anything.

Scope of the guarantee: NFS data and metadata procedures. It does not mean the server sees nothing: `MNT`/`UMNT` add and remove the
server's mount record (`rmtab`; a failed `Connect` sends a best-effort `UMNT` after a successful `MNT`), and `READ` updates access times on
exports that are not mounted `noatime`.

## Using it

```go
c, err := nfs3.New(nfs3.Config{
    Host:   "nas.example",
    Export: "/volume1/video",        // must be an export; nfs3.Client.Exports(ctx) lists them
    UID: 1000, GID: 1000,            // AUTH_SYS identity
})
if err != nil { ... }
if err := c.Connect(ctx); err != nil { ... }   // portmapper -> MNT -> FSINFO -> GETATTR(root)
defer c.Disconnect(ctx)                        // best-effort UMNT

entries, _ := c.ListDirectory(ctx, "/")        // READDIRPLUS, paged with cookie + cookieverf; no per-entry GETATTR
r, _ := c.ReadFile(ctx, "/a/b.mkv")            // adaptive read-ahead (below); io.Seeker too (OpenSeekable)
part, _ := c.ReadRange(ctx, "/a/b.mkv", off, n) // never reads past off+n
```

`*nfs3.Client` implements `client.Client` and `client.SeekableClient` (`GetProtocol()` returns `"nfs3"`). Paths are relative to the
export root, `/`-separated; any `..` segment or NUL byte is refused (`ErrInvalidPath`). Symlinks are reported (`os.ModeSymlink`), never
followed. Servers that offer no portmapper (user-space servers such as go-nfs) are reached with `MountPort` and `NFSPort` set.

Configuration highlights (`nfs3.Config`):

* Addressing and identity: `PortmapPort/MountPort/NFSPort`, `UID/GID/GIDs/Machine`. **An unset (zero) `UID` or `GID` is sent as 65534
  (nobody), not as root**, because on a `no_root_squash` export an unset identity would otherwise read the tree as root; set `AsRoot`
  to send 0 on purpose.
* Time and retries: `DialTimeout`, `CallTimeout` (per RPC), `MaxRetries` (default 2; negative means none; at most 1000) with
  `RetryBackoff` (doubled per attempt, capped at 5 s). `NFS3ERR_JUKEBOX` has its own schedule: `JukeboxRetries` (default 4, negative
  none) and `JukeboxBackoff` (default 2 s, doubled, capped at 30 s), independent of the transport retries, because tiered or spun-down
  storage needs seconds (RFC 1813: the client "should wait and then try the request"). The waits end with the caller's context.
* Reading: `MaxPipeline` (the ceiling of concurrent READs per open file, default 8, at most 64 because every READ in flight can hold up to
  1 MiB), `ReadSize` (the ceiling of one READ; default the
  server's `rtpref`; never above FSINFO `rtmax`, 1 MiB, or what fits `MaxRecord`).
* Listing: `DirCount/DirMaxCount` (`New` refuses `DirCount` above `DirMaxCount`; RFC 1813 3.3.17 defines `dircount` as the
  directory-information part of `maxcount`), `MaxEntries`.
* Limits: `MaxRecord` (default 8 MiB). `New` refuses a configuration whose `ReadSize` or `DirMaxCount` plus 4 KiB does not fit
  `MaxRecord` (every reply would be refused); unset sizes are derived to fit.
* `HandleCacheTTL` (default 10 s, negative disables the cache of intermediate directories): see "Handle cache".
* `TryPrivilegedPort`: see "Reserved source ports".

### Read-ahead

A reader starts with ONE READ of at most 64 KiB; the chunk size and the number of READs in flight then double with every chunk that is
consumed in sequence, up to the ceilings above (a seek restarts the ramp). Opening a file to look at its header therefore moves
kilobytes, not a window of megabytes, and `ReadRange(off, n)` never asks for more than `n` bytes. A short reply that is not EOF (the server's
`rtmax` is below what was asked) keeps the READs already in flight, requests only the missing gap and sizes the following READs to what the
server really returns. Closing a reader, a `Seek`, a failed READ and the end of the file cancel the READs still in flight on the CLIENT side
(each READ has its own context); a READ whose request was already written to the socket cannot be recalled, so the server still serves it.
Calls on one reader are serialised: a `Seek` or a second `Read` waits for a `Read` that is waiting for its reply (`Close` does not wait, it
cancels first).

## Reserved source ports

Many NAS products accept requests only from TCP source ports below 1024. The client connects from an ordinary ephemeral port first,
because an unprivileged process cannot do anything else. When the server then refuses (`MNT3ERR_ACCES` or `MNT3ERR_PERM`, `NFS3ERR_ACCES` or `NFS3ERR_PERM` (what the Linux kernel server
sends for a source port it does not accept), or an RPC `AUTH_ERROR`), `Connect` returns an `*nfs3.AccessError` that names the layer (`mount`, `nfs`), the local source port that was refused and
whether a reserved port was already used; `errors.As` also reaches the underlying `*MountError`, `*NFSError` or `*RPCError`.
With `TryPrivilegedPort` the client retries **once** from a port in 512..1023 (descending from 1023 like `mount.nfs`) and only after an
access error; when the process may not bind one, `Connect` returns BOTH errors, the server's original refusal first (an `*AccessError`
for an unprivileged source port, still reachable with `errors.As`) and then `nfs3.ErrPrivilegedPortUnavailable` (needs root,
`CAP_NET_BIND_SERVICE` or `net.ipv4.ip_unprivileged_port_start` lowered): the operator learns that the export rule refused, not only that a
local bind failed (a real-NAS run found the first version returning only the bind error). When the server refuses the reserved-port attempt
too, that second `*AccessError` (with `Privileged` set) is returned alone. A refusal of any other kind (unknown export, bad flavor) never triggers the retry.

## Failure behaviour

* Every RPC has a per-call timeout (`CallTimeout`) and honours the caller's context, also while waiting for the write slot of a
  connection, while waiting for a remount, and while blocked in a write to a server that does not drain its socket. One caller's
  cancellation does not hurt the others: when it interrupts a write that had sent NOTHING the stream is intact and the shared connection
  stays open; when part of the record had left (or the write timed out with a live context) the connection is closed, because half a
  record is on the stream, and it is closed before the write slot is released so that no other record is appended to it. The one
  exception to "honours the caller's context" is the best-effort `UMNT` that cleans up a FAILED `Connect`: it is bounded by 2 s (or
  `CallTimeout` when smaller) while the context lives, and still gets 500 ms after the context ended, so the server's mount record is
  normally removed but the caller never waits for a mount daemon that accepts and does not answer. A timed-out or lost connection is dropped and the call is re-issued with a NEW xid on a NEW connection. That is
  this client's design choice (a connection that stopped answering may hold a half-sent record or a stuck server thread); RFC 5531 does not
  forbid retransmitting on the same connection (its section 5 even lets a client reuse the xid when retransmitting), so no RFC is cited
  for it. All procedures used are reads, so a repeat is always safe. One xid sequence is kept per `Client`, so a reconnect never reuses the
  xids of the earlier connection.
* Retries are bounded (`MaxRetries`, exponential backoff, context-aware) and only for transient classes: connection loss, call timeout,
  RPC `SYSTEM_ERR`. `NFS3ERR_JUKEBOX` is waited out on its own schedule (see Configuration). Permission, not-found,
  stale-handle-after-refresh, auth and decode errors are returned at once.
* `NFS3ERR_BAD_COOKIE` during a listing restarts it (at most twice). A listing that does not advance ends with `ErrNotProgressing`: a
  request cookie that repeats (a page that does not move the cookie asks for the same cookie again, so this covers it), more than two
  pages made only of `.` and `..`, or more pages than `MaxEntries` allows. A name that is empty or contains `/` or NUL is a protocol violation (`ErrBadXDR`). Servers may omit the
  attributes of a READDIRPLUS entry; the client then asks for them, and which handle an error refers to depends on the call (RFC 1813):
  in `LOOKUP(dir, name)` the only handle is the DIRECTORY, so only `NOENT` means that the entry vanished (it is skipped and counted,
  `VanishedEntries()`), while `STALE`/`BADHANDLE` mean that the directory itself is invalid and FAIL the listing (never an empty
  directory); in the following `GETATTR(entry handle)` `NOENT`, `STALE` and `BADHANDLE` all mean that the entry vanished.
* `NFS3ERR_STALE` or `NFS3ERR_BADHANDLE` on a cached handle drops the handle cache and repeats the walk once; if the walk is still stale
  the export is mounted again (`MNT`) for a new root handle and the walk repeated one last time. A READ error is sticky until `Seek`
  (to any position, also the current one), which clears it; the next read starts from a clean position and never reports a silent EOF.
* A file size at or above 2^63 in an attribute reply is a protocol violation (`ErrBadXDR`).

* A reply with an unknown xid (late, duplicate, unsolicited) is counted and dropped; a record that cannot hold an xid and message type
  is a framing violation and closes the connection.
* The decoder never allocates from an announced length: every variable-length item is checked against its limit and the bytes actually
  received first (filehandle 64, names 4096, RPC record 8 MiB by default). `FuzzDecoders` and `FuzzReadRecord` guard this.

### Handle cache

Directory handles are cached by path so that a deep path costs one LOOKUP, but NFS handles survive renames, so nothing turns stale on its
own. The last component of a path is always looked up. Validity of a cached entry is a property of the whole CHAIN up to the root, not of the
entry alone: the entry is trusted only while (1) its own age is within `HandleCacheTTL` (default 10 s; the root always), (2) its parent entry
is still present under the generation the entry was learned under, and (3) the parent is valid in turn. A path gets a new generation whenever
it is remembered with a handle that differs from the one it held (or none), so every entry learned below the old directory is out of reach
from that moment, whichever order the entries expired or were evicted in. A directory whose handle did not change keeps its generation, so
its children stay usable after the directory itself was looked up again. A path whose parent is not remembered is not reachable. When the
cache is full (8192 entries) it starts over from the root and keeps the chain of the entry being added. Changes made by other clients (a
rename, a replaced directory) become visible to a walk that goes through cached parents within that TTL at the latest; a negative TTL walks
from the root every time.

## Security notes

AUTH_SYS sends uid/gid in the clear and the server trusts them on the strength of the client address; RFC 5531 section 14 (Security
Considerations) says it SHOULD NOT be used for services that permit clients to modify data. This client never modifies data, which is why
it is acceptable here. Use it on trusted networks
only. The client holds no password, so `GetConfig()` exposes nothing secret. Exports that list only `AUTH_NULL` (go-nfs) are served
with AUTH_NONE credentials; exports that list neither are refused (`ErrAuthFlavor`).

## Tests

All run in the digest-pinned `IMG-GO` container, rootless, through `scripts/containers/run_pinned.sh`:

```
bash scripts/containers/run_pinned.sh IMG-GO -- sh -c 'cd /src/submodules/filesystem && go test -race -count=1 ./pkg/nfs3/'
bash scripts/containers/run_pinned.sh IMG-GO -- sh -c 'cd /src/submodules/filesystem && go test ./pkg/nfs3/ -run xxx -fuzz FuzzDecoders -fuzztime 60s'
bash scripts/containers/run_pinned.sh IMG-GO -- bash /src/submodules/filesystem/test/nfs3fixture/run_integration.sh   # needs bash, not sh (the readiness probe uses /dev/tcp; /bin/sh is dash)
```

* `fakeserver_test.go`: an in-process NFSv3 + MOUNT v3 + portmapper server with fault injection (short reads, paging, bad cookie,
  dropped connections, JUKEBOX, out-of-order replies, wrong xid, corrupt replies, privileged-port gate). Test code only.
* `review_probes_test.go`, `review_killers_test.go`: the WF19 reviewer's 18 probes and 17 mutant killers; `review_wf24_probes_test.go`,
  `review_wf24_killers_test.go`: the WF24 reviewer's probes W1 to W4, W7 and killers N01 to N17; all adopted verbatim as permanent tests
  (deviations are marked `ADOPTED-CHANGE` in the file header); `fixr2_test.go`, `fixr3_test.go`, `fixr3_api_test.go`: the tests the fixes
  needed beyond them. The reserved-port loop is executed through an injected bind (`dialPrivilegedFrom`) because the pinned container
  refuses a real reserved-port bind even as container root.
* `integration_test.go` (tag `nfs3fixture`): the real-server leg against **willscott/go-nfs v0.0.4**
  (`submodules/filesystem/test/nfs3fixture`, Apache-2.0, an independent RFC 1813 implementation, fixture only, never a runtime
  dependency). The oracle is the server's own manifest (path, type, size, SHA-256) of its tree; the client's recursive crawl and reads
  must reproduce it. A second fixture process (`-authsys`) advertises AUTH_SYS and decodes every call's credential with go-nfs-client's XDR
  reader, an implementation the client shares no code with (`test/nfs3fixture/authsys.go`); it checks uid, gid and the supplementary gid
  VALUES against distinct configured numbers (1234, 2345, 4/24/3456), so an encoder that swaps or shifts them is seen;
  `TestGoNFSAuthSysCredentialIsWellFormed` requires the verified-credential log to be non-empty, free of violations, and to carry that identity. The fixture also builds as a rootless, read-only, capability-free image (`Containerfile`, `build_image.sh`) and as
  a compose service (`docker-compose.test-infra.nfs3.yml`).
* Mutation: `evidence/wp12/nfs3/tools/mutants.sh` (22 self-written mutants of revision 1), `tools/fixr2_mutants.sh` (the WF19 reviewer's 18
  mutants re-addressed to the fixed source plus one mutant per fix) and `tools/fixr3_mutants.sh` (the WF24 reviewer's 18 mutants, run
  verbatim on the committed code and re-addressed on the fixed code, plus one revert mutant per fix), each with a negative control. The
  numbers are in the evidence README.

## Not covered / open

* Not wired into `pkg/factory` or `catalog-api` (a later task, together with the settings-key work PA-01).
* Not run against the NAS (another stream owns that; a later read-only validation).
* The T134 unfs3 fixture (rpcbind-registered, libnfs oracle) was not used: its stack lives on the compose network and only
  `IMG-INFRA-CLIENT` (no Go toolchain) joins it. The portmapper, `EXPORT` and `MNT` paths are covered by the in-process server
  and, for `MNT`/NFS, by go-nfs.
* Real-NAS behaviour of `rtpref`, `READDIRPLUS` `dircount/maxcount`, `JUKEBOX` timing and reserved ports is UNCONFIRMED until the NAS leg runs.
* The write-blocked and `JUKEBOX` paths are exercised against in-process servers only.

## Review history

Revision 2 closes the independent review `WF19` (one pass, Opus, verdict NO-GO: 2 HIGH, 6 MEDIUM, 8 LOW source defects, 5 test and 5
documentation findings). Corrected claims: the earlier "22 of 22 mutants killed" measured the author's own mutants, not the strength of the
suite (the reviewer's 18 independent mutants: 17 survived); "malformed and hostile replies: yes" did not hold for a non-advancing
READDIRPLUS; "`MaxRetries` negative disables" did not hold below -1; the nfs-layer `AccessError` did not state the source port; the retransmission
and AUTH_SYS citations pointed at RFC text that does not say what was claimed.

Revision 3 closes the independent re-review `WF24` (round 2, Opus, verdict NO-GO: 1 MEDIUM and 4 LOW source defects, 2 MEDIUM and 3 LOW test
findings, 6 LOW documentation findings; all 28 WF19 findings verified closed) and the defect the real-NAS leg found (the privileged-port
retry hid the server's refusal). The third pass was a structural round: the recurring classes (which handle a status refers to, cache
validity as a chain property, context and the effect of one caller's cancellation, the allow-list key) were re-derived and closed for every
member (`fix-r3-convergence.md`). Corrected claims: "when a path gets a different handle, everything cached below it is dropped" (it was
not, when the parent had expired first); "every RPC honours the caller's context" (not the cleanup `UMNT` of a failed `Connect`, and one
caller's cancellation could fail the shared connection); "refuses a write procedure by constant, literal or variable" (the version was not
part of the key); "the image build is unchanged" (the fixture image did not build from the committed tree).
