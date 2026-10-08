# Package `digital.vasic.filesystem/pkg/fabric`

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T20:30:00Z |
| Status | implemented in the working tree of the `submodules/filesystem` submodule (own-org, NOT committed there); independent review round 1 (WF19, 2026-10-07) returned NO-GO (1 HIGH, 6 MEDIUM, 8 LOW source-defects), round 2 (this revision) fixes every finding, the re-review is OWED; not yet linked from docs/README.md (11.4.212, owed to the conductor); not yet consumed by catalog-api |
| Source | `submodules/filesystem/pkg/fabric/*.go`, `submodules/filesystem/pkg/decorators/guard/*.go`; task PA-07 of `specs/001-full-project-audit-remediation/evidence/wp12/protocol-analysis/REPORT.md` (sections 3.2 items 4 to 6 and 4); evidence `specs/001-full-project-audit-remediation/evidence/wp12/fabric/` |

## What it is

Protocol-independent plumbing around `client.Client`: it contains no SMB/FTP/NFS/WebDAV code, so every present and future protocol client inherits it.

| Piece | File | Behaviour |
|---|---|---|
| `HostBudget`, `Budgets` | `budget.go` | per-host cap: `MaxConcurrent` operations in flight and `MaxReqPerSec` evenly spaced starts (no burst, idle time is not banked). A start slot reserved by a caller whose context ends while it waits is **given back** (the schedule front is wound back, or the slot is reused by the next caller if it is still in the future), and a caller whose deadline falls before its slot fails fast without reserving; so cancelled waiters never push the schedule ahead of real time. A rate whose spacing does not fit a `time.Duration` (below about 1.1e-10 per second) is rejected, never turned into "no spacing". `Budgets.For(host, cfg)` returns ONE budget per host key (`HostKey`), so SMB and SFTP to the same NAS share the cap. Both limits are required and positive (no silent default); `NASBudget()` (4 and 10/s) is an explicit opt-in. A different config for an already registered host is `ErrBudgetConflict`. `WithStreamLease(d)` optionally releases the slot of a stream that was never closed. The cap bounds **operations and open streams** in this process, not TCP connections: the connection count is the pool's `MaxPerKey` per storage root. |
| `BudgetStats` | `budget.go` | `InFlight` = held concurrency slots, including callers still sleeping for their start slot. `Acquired` = start-slot **reservations**, including ones later given back because the caller's context ended (it is not the number of operations the host saw). `TotalRateWait` = wait imposed at reservation time. `Held` / `OldestHeld` = slots held by callers that finished waiting (running operations and open streams) and the age of the oldest, so a leaked stream is visible. `LeaseExpired` = slots taken back by the lease. |
| `Pool` | `pool.go` | implements `client.ConnectionPool`. Per storage root (`StorageConfig.ID`): `MaxPerKey` cap (**per root, not per host**: N roots on one NAS can hold N x `MaxPerKey` connections, bound the host with a `HostBudget`), health check on borrow (`IsConnected` and `TestConnection` with `HealthTimeout`), `MaxLifetime` and `IdleTimeout` retirement, `GetClient` and `GetClientContext`, `Evict`, idempotent `CloseAll`, double or foreign return is `ErrNotFromPool`. Details below. |
| `Limited(c, budget)` | `limited.go` | every operation takes a budget slot; read streams (`ReadFile`, `OpenSeekable`) keep it until `Close`. `Disconnect` and `TestConnection` are never throttled (closing must always be possible; `TestConnection` is the pool's health probe, which a busy host would otherwise turn into a "dead connection" verdict). `Connect` is budgeted (a login). A stream that is never closed keeps its slot, across all protocols of the host, unless the budget was built with `WithStreamLease`. |
| `Retrying(c, policy)` | `retrying.go`, `errors.go` | repeats READ operations only, with exponential backoff, after errors classified transient. Never retries: mutations, `Disconnect`, authentication failures, permanent or unrecognised errors, or after the caller's context is done. The policy is validated: `MaxAttempts` 1..`MaxRetryAttempts` (100), and `BaseDelay` > 0 whenever `MaxAttempts` > 1 (a zero delay is a retry storm). Exhaustion wraps the last error with `ErrRetriesExhausted`. See "Error classification". |
| `Confined(c, root)` | `confined.go` | paths are normalised (backslash is a separator, every control character is refused - NUL, CR, LF, ESC, the rest of C0 and DEL -, relative paths resolve under root, `path.Clean`) and must stay inside root at a segment boundary; the inner client receives the cleaned path; escapes are `ErrOutsideRoot` and never reach it; `CopyFile` checks both paths first. The check is **lexical** (see Limits). |
| `Metered(c, sink, clock)` | `metered.go` | one `Sample` per operation (streams are reported at `Close` with the bytes read through `Read`, `ReadAt` and `WriteTo`; `WriteFile` counts bytes consumed). `CounterSink` is the dependency-free aggregate; the catalog-api metrics package can implement `Sink`. |
| `Op` table, `HostKey` | `op.go` | the single reviewed classification of every operation (mutating, retryable, stream, path arity). `HostKey` accepts a host or `host:port` only (a URL, path or `user@host` is an error), canonicalises IP literals (RFC 5952 text, IPv4-mapped IPv6 unmapped) and does **not** resolve names: `nas`, `nas.local` and the machine's IP are three keys. |
| `guard` (package `pkg/decorators/guard`) | `nil.go`, `config.go`, `stream.go` | shared helpers: typed-nil detection (every decorator and `ReadOnly` panic at construction for a typed nil too), `RedactConfig` and the capability-preserving stream wrapper. |

Every decorator preserves `client.SeekableClient` iff the inner client has it, and none embeds the interface (a new interface method breaks the build instead of being promoted unguarded). The streams they return preserve the inner stream's `io.Seeker`, `io.ReaderAt` and `io.WriterTo`, so a consumer that type-asserts `io.ReaderAt` (catalog-api's comic page handler) still gets ranged reads through any layer. `GetConfig` of every layer is a redacted, detached copy (see [pkg_decorators.md](pkg_decorators.md)).

## Pool behaviour

- **Cancellation is not a health verdict.** A context that is already done never reaches the pool; if the caller's context ends during the health probe, the probed connection is put back and the context error is returned. Only a probe failure under a live context retires the connection.
- **No repeated logins after a rejected one.** A login classified `ClassAuth` is remembered per storage root (as a hash of the settings, never the settings); further borrows fail fast with that error without contacting the host, so a scan that borrows per file cannot trip a NAS auto-block. The memory is cleared by `Evict(id)`, by `CloseAll`, or when the settings change. Transient connect failures are never remembered.
- **Bounded connect.** A new connection is opened under `ConnectTimeout` (default 30 s) on top of the caller's context, so `GetClient` ("without waiting" means: never waits for a free pool slot; `ErrPoolExhausted`) cannot block without bound.
- **Retirement is lazy.** There is no reaper goroutine; every borrow and return sweeps the expired idle connections of all roots. A connection can outlive `IdleTimeout` until the next pool call.
- A client whose dynamic value is not comparable is refused, never panics, and cannot wedge the pool (the check inspects the dynamic value; the lock is released by `defer`).
- `Pool` keys connections by `StorageConfig.ID`; changed settings under the same ID reset a remembered rejection but need `Evict` to retire idle connections.

## Error classification

`Classify` order: (1) an error marked with `MarkAuth` is auth; (2) a structured FTP reply (`textproto.Error`) is decided by its code: 530, 332 and 532 are auth; every 4yz is transient (RFC 959: "the action may be requested again") **unless the reply text says the login failed**, in which case auth wins so that a lockout reply is never retried; every other 5yz (including 550 and the RFC 2228 policy replies 533 to 536, and 430 which is not an RFC reply) is permanent; (3) otherwise an auth text marker in a **leaf** cause (never in a wrapper that embeds a file name or in a `*fs.PathError`) is auth; (4) `ErrTransient` marks transient; (5) caller cancellation, not-found, permission and `ErrOutsideRoot` are permanent; (6) only positively identified network faults are transient (reset, abort, pipe, timeout, refused, host/net unreachable, EOF, temporary DNS, `net.Error` timeout); everything else is permanent. Limit: a leaf error whose own message embeds a file name and was built without `%w` cannot be told from a server message; protocol glue that knows its login-failure type should call `fabric.MarkAuth`. A wrapper prefix such as `fmt.Errorf("login incorrect: %w", err)` is deliberately not trusted.

## Composition

`fabric.Chain(c, Metered, Retrying, Limited, Confined, ReadOnly)` applies the FIRST decorator outermost. Recommended order: Metered outermost (one sample per final outcome), Retrying around Limited
(each attempt takes a fresh budget slot), Confined and ReadOnly next to the protocol client.

## Streams and the budget

A `Limited` stream holds its slot until `Close`; `Close` is idempotent. `Stats().Held` and `OldestHeld` show unclosed streams. With `WithStreamLease(d)` the slot is released after `d` even if the holder never closes (the stream stays readable, so the host can briefly see more than `MaxConcurrent` operations); without it a leaked stream starves every protocol sharing the host, which is the documented contract.

## Class-exhaustive guard

`TestOpTable_CoversEveryClientMethod` reflects over `client.Client` and `client.SeekableClient`: every method must be an `Op` or a declared local pass-through (`IsConnected`, `GetProtocol`, `GetConfig`).
It is control-needled (sees 15 + 1 methods, `ListDirectory` visible) and `TestOpTable_InstrumentDetectsInjectedMethod` shows that an added `MoveFile` is reported. The decorator tests then loop over
`AllOps()`, so a new operation is exercised by the existing tests the moment it is classified.

## Tests

Unit (fake client, fake clock, allowed in unit tests only), real-filesystem chain test (`TestChain_RealLocalFilesystem`), race tests (pool and budget under load), fuzz (`FuzzConfined`, which now also asserts that no control character reaches the inner client). Round 2 adds: `review_probes_test.go` (every probe RV01 to RV22 of the independent reviewer, adopted verbatim as regression tests), `budget_fix_test.go`, `pool_fix_test.go`, `fix_classes_test.go` (one test per member of each defect class) and a context-honouring fake clock (`parkClock`, whose `Sleep` blocks until woken or cancelled) and context-honouring pool fakes, because the round-1 fakes ignored cancellation and could not see the cancellation defects.
Run only in the pinned Go container (`scripts/containers/run_pinned.sh ... IMG-GO`, `go test -race -count=1`). Paired mutation harness and results: `specs/001-full-project-audit-remediation/evidence/wp12/fabric/`.

## Not done / UNCONFIRMED

- Not wired into catalog-api; no protocol client constructs these yet (PA-08 to PA-10 own that). The Prober, Selector, capability table and generic Scanner of REPORT.md section 3.2 are NOT part of this change.
- The chaos lane (drop a connection mid-list against a real server, task PA-13) and the throughput benchmark lane (PA-12) need the WP-13 fixtures and are not run here; the retry-class behaviour is proven with injected errors and the real error types (`syscall`, `net`, `textproto`), not against a live server that resets.
- Real SMB/SFTP/FTP error values were not captured from servers: the auth text markers and FTP codes are from the protocol specifications and the library error shapes named in the code, UNCONFIRMED against each live server. A protocol glue that knows its own login-failure type should call `fabric.MarkAuth`.
- Windows-style trailing dot/space and Unicode normalisation tricks in `Confined` are not modelled; the comparison is exact and therefore stricter than a case-insensitive server, but a server that maps a differing name onto the same file is UNCONFIRMED.
- `Confined` is lexical: a server-side symlink or junction inside the root that points outside it is followed by the server. This follows from the code and was not probed (no server). Use a server-side jail (a read-only account, an export root) as well.
- Nested retries: `pkg/sftp` `Connect` already retries internally; `Retrying` treats `Connect` as retryable, so the attempts multiply. Wrap such a client with a policy that accounts for it.
- An instance of `Limited` around a pooled client shares the budget with the pool's login (`Connect`): a host whose budget is held by long streams delays new logins up to `ConnectTimeout`.
- Stream read errors are not part of the `Metered` sample (only bytes and the open outcome).
