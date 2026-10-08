# FTP / explicit FTPS client (`pkg/ftp`)

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07 |
| Status | implemented in `submodules/filesystem` (working tree, uncommitted: own-org submodule) and wired into `pkg/factory` (`ftp` case); unit, factory and real-server tests green (numbers: `specs/001-full-project-audit-remediation/evidence/wp12/ftp/README.md`); the independent re-review of fix round 2 is owed (constitution 11.4.142 / 11.4.209) |
| Task | WP-12 PA-04 (`specs/001-full-project-audit-remediation/evidence/wp12/protocol-analysis/REPORT.md` sections 2.4, 3, 4); fix round 2 answers the WF21 review (`NO-GO`, findings F1-F16, T1-T3, P1-P2) |
| Evidence | `specs/001-full-project-audit-remediation/evidence/wp12/ftp/` |
| Protocol layer | `pkg/ftp/proto.go` (control and data channels), `pkg/ftp/mlsx.go` (MLSD/MLST/LIST parsers). The client no longer wraps `github.com/jlaffaye/ftp`: v0.2.4 bounds nothing but the TCP connect, hides the final reply of a transfer, drops unparseable listing lines silently and folds every login failure into one error (WF21 D1-D7, D11, D12, D16, D17), and it has no hook for any of that. `go.mod` still lists the module (`go mod tidy` removes it). |

## What changed against the old client

| Old (`ftp.go` at the submodule HEAD) | Now |
|---|---|
| Clear text only; password and data crossed the LAN unprotected | Explicit FTPS by default: `AUTH TLS`, TLS 1.2 minimum, `PBSZ 0` + `PROT P`, TLS session cache shared by control and data connections |
| No certificate policy | The server certificate must be **pinned per host**; unknown or changed certificate is refused before any credential is sent; `InsecureSkipVerify` is not used anywhere |
| Clear text always allowed | Clear text refused unless the root sets `trusted_lan=true` (`Config.TrustedLAN`) |
| `GetFileInfo` returned `time.Now()` and made 2 round trips | MLST attributes; a modification time the server did not send is the zero `time.Time` |
| `List` silently fell back to `LIST` | MLSD; a server without MLST is refused (`ErrNoMLSD`) unless `allow_degraded_list` is set, then `Degraded()` is true and ModTime is unknown (zero) |
| One shared control connection, no locking | One session per client: every operation (and every open read stream) holds it, so the connection is never used by two commands at once; parallel scanning = one client per worker (`NewWorkerPool`, built on `fabric.Pool`) |
| No resume | `ReadFileFrom(path, offset)` and `OpenSeekable` (REST) |
| Login failure surfaced as an anonymous error | Classified **by login phase** (see "Login classification") |
| Password in `Config`, returned by `GetConfig()` | `credential_ref` resolved at connect time; `GetConfig`, `%v`, `%+v`, `%#v` of the config, of a `Credential` (value or pointer) and of the worker pool, and errors never contain the secret |
| No path handling | Lexical confinement to the root; `..` escape, CR, LF and NUL refused before any command; a `0xFF` byte is sent doubled (Telnet IAC, RFC 854) |
| Scan path could write | `NewScanClient` / `NewWorkerPool` wrap the client in `decorators.ReadOnly` |
| Not reachable from `pkg/factory` | `pkg/factory` builds the scan client from the stored settings (see "Wiring") |

## Configuration (`ftp.Config`)

| Field | Meaning |
|---|---|
| `Host`, `Port` (21), `Username` | the server |
| `CredentialRef` | name of the secret; resolved by `Resolver` (default `EnvCredentialResolver`: `FTP_CRED_<REF>_PASSWORD`, ref upper-cased, non-alphanumerics become `_`) |
| `Password` | **deprecated** inline secret, used only when `CredentialRef` is empty; never serialised (`json:"-"`); the factory refuses it |
| `TLSMode` | `explicit` (default) or `none` |
| `TrustedLAN` | allows `TLSMode=none`; without it `Connect` returns `ErrClearTextRefused` before opening a socket |
| `PinStore` | certificate pins (`MemPinStore`, `FilePinStore` = JSON, mode 0600, atomic rewrite, a corrupt file is an error and is never overwritten); required for explicit TLS; the factory passes `ftp.DefaultPinStore` |
| `Path` | base directory; resolved to the server's own spelling at connect time (`CWD` + `PWD`, a doubled quote in the reply is one quote) |
| `AllowDegradedList` | permit `LIST` on servers without MLST/MLSD |
| `DialTimeout` (30 s) | ONE budget for the whole connect: TCP connect, greeting, `AUTH TLS`, TLS handshake, login, `CWD`/`PWD`. A silent server is detected after this time. |
| `IOTimeout` (60 s) | on the control channel: the whole of each reply (all its lines: banner, every command's reply, the final reply of a transfer) and each write; on data connections: every single read or write (`MLSD`/`LIST`/`RETR` data). The `ctx` deadline applies in addition: the earlier of the two wins. A data transfer as a whole is bounded only by `ctx` (a large file may legitimately take long); a listing is bounded by `MaxListEntries` too. |
| `MaxReplyBytes` (1 MiB) | one control reply, all its lines; more is `ErrReplyTooLarge` and the connection is dropped |
| `MaxListEntries` (1,048,576) | lines of one listing; more is `ErrListingTooLarge` and the connection is dropped (a line longer than 16 KiB is `ErrListingIncomplete`) |
| `DisableEPSV` | force PASV |

## Login classification

| Phase | Failure | Class |
|---|---|---|
| connect, greeting | network error, timeout, `4yz` (including `421`) | transient (retried) |
| connect, greeting | `5yz` | permanent |
| `AUTH TLS` | `5yz` (no explicit TLS) | permanent, **no clear-text fallback**, no credential sent |
| TLS handshake | unknown or changed certificate | `*UnknownCertError` / `*CertMismatchError` (permanent), before `USER` |
| `USER` | `421` or another `4yz`, network error | transient (a per-IP limit is not a bad password) |
| `USER` | `530`, `532`, `332` | authentication (`fabric.ErrAuth`), never retried |
| `PASS` | `530`, `430`, `534`, `535`, `332`, `532` | authentication, never retried |
| `PASS` | any other reply (`421`, `503`, ...) | permanent: not retried, because a retry would send the password again |
| `PASS` | the connection fails **before the reply** | permanent, not retried (the server may or may not have counted the attempt: lockout risk) |
| `FEAT` | refused | no features (not an error); network error: transient |
| `TYPE I`, `PBSZ`, `PROT P` | `5yz` | permanent configuration failure, **never authentication** |
| `OPTS UTF8 ON` | `5yz` | the client connects; a listing that then contains a name that is not valid UTF-8, or carries `0x7f`, fails with `ErrUTF8Refused` instead of cataloguing a garbled name (pure-ftpd advertises `UTF8` and answers `504`; its names are valid UTF-8) |
| anything | `ctx` ended | the context error (permanent) |

## Certificate pinning workflow

1. `ftp.DiscoverCert(ctx, host, port, timeout)` connects, sends `AUTH TLS` and returns the SHA-256 fingerprint of the presented leaf. It sends no credential. `timeout` bounds the whole discovery and `ctx` interrupts it; port 0 means 21.
   (An empty trust pool makes the handshake fail verification; the failure carries the presented chain, so no `InsecureSkipVerify` is needed to read it.)
2. The **owner** compares that fingerprint out of band (`openssl x509 -noout -fingerprint -sha256`, the NAS UI).
3. `ftp.Pin(ctx, store, host, port, ftp.Confirmation{Fingerprint: <what the owner saw>, ConfirmedBy: ...})` re-reads the certificate and records it **only if it has exactly the confirmed fingerprint** (a prefix is refused). One TLS server name is verified per host: a second pin whose certificate is valid for another name is refused with `ErrPinNameConflict` (remove the old pin first; rotating to a certificate with the same name needs no removal).
4. `Connect` uses the pinned certificate as the **only** trust anchor (`tls.Config.RootCAs`), with the name stored in the pin as `ServerName`, and a `VerifyConnection` hook that re-checks the fingerprint of the leaf (a CA-signed leaf under a pinned CA is refused).
   No pin: `*UnknownCertError` (carries the fingerprint to confirm). Different certificate: `*CertMismatchError`. In both cases the handshake fails before `USER`, so `USER`/`PASS` are never sent.
5. A pinned certificate that is expired, or whose name no longer matches, is a normal verification failure (not silently re-pinned).

## Read-only scan path and workers

```go
pool, _ := ftp.NewWorkerPool(&cfg, fabric.PoolOptions{MaxPerKey: 3}, ftp.ScanOptions{Budget: hostBudget})
c, _ := pool.GetClientContext(ctx, &client.StorageConfig{ID: rootID})   // one control connection per borrowed client
defer pool.ReturnClient(c)
```

Each pooled client is `Retrying( Limited( ReadOnly( ftp.Client ) ) )`: mutations return `decorators.ErrReadOnly` and never reach the server; reads that fail
transiently are retried with backoff (default 3 attempts, 200 ms, cap 2 s) **except authentication failures and failures after the password was sent**; the optional `fabric.HostBudget` bounds concurrency and request rate towards the host
(set `MaxPerKey` at or below the server's connections-per-IP cap; the repository fixture allows 5). The pool keeps its own copy of the config.

## Behaviour worth knowing

* **Deadlines.** Every blocking operation is bounded by the earlier of the `ctx` deadline and `IOTimeout` (and, while connecting, `DialTimeout`). A stalled server therefore costs at most `IOTimeout` per operation, whatever the command; the connection that timed out is dropped and the next operation re-dials.
  `ctx` cancellation interrupts a blocked read or write at once, on the control channel and on data connections; a command cancelled this way leaves the connection unusable, so it is dropped.
* **Lock-step.** Replies are matched to commands by exact code. An unexpected positive reply, `421`, an oversized reply, or bytes nobody asked for make the connection unusable (`ErrProtocol`, ...); the next operation re-dials. A stale reply is never read as the answer to a later command.
* **Streams.** A read stream holds the session until `Close`. A second command on the same client waits for it and gives up with its own `ctx`. Do not call another method of the same client from the goroutine that holds an unclosed stream.
  After a complete read (`io.EOF`) `Close` returns an error if the final reply is negative (426, 451, ...): the file may be incomplete. An early close is not an error; the client then reads whatever single reply the server sends and proves the control channel is in step with a `NOOP`
  (real pure-ftpd, measured: one reply, `150 <statistics>` over TLS and `226` in clear text, connection kept). A server that answers with two replies makes the `NOOP` fail and the connection is dropped. A stream that failed, timed out or was cancelled is not waited for: its connection is dropped.
* **`OpenSeekable`** never returns a truncated file as complete: when the transfer ends before the `SIZE` the server announced, the read returns `io.ErrUnexpectedEOF`, or the server's negative final reply. Each `Seek` ends the transfer (an early close as above) and the next `Read` resumes with `REST`.
* **`Disconnect`** aborts an open stream even when no `Read` is in flight (the next `Read` returns `ErrAborted`), says `QUIT` and closes the connection, and never re-dials implicitly. When its `ctx` ends before the session is free, the connection is closed by whoever releases the session next, so a timed-out `Disconnect` does not keep the server's slot. A lost connection (server closed it, network drop, timeout) is re-dialled by the next operation, which is what lets `Retrying` recover.
* **Listings** are parsed by this package (RFC 3659): fact names and `Type` values are case-insensitive, `Size` is decimal (`010` is ten), a fact value may contain spaces, an entry may have no facts, a name may start with spaces, `cdir`/`pdir` are skipped by type, `OS.unix=symlink`/`slink` is a symlink (`FileInfo.Mode = os.ModeSymlink`), directories are `os.ModeDir`, other types are `os.ModeIrregular`; a fractional-second `Modify` is read.
  A line that cannot be understood fails the listing with `ErrListingIncomplete` after the transfer was read to its end (the connection stays usable); a partial listing is never returned as complete. Server-supplied names containing `/` or NUL are skipped.
* **`550`.** RFC 959 uses it for "not found" and "no access". A 550 whose text says "no such file" is `os.ErrNotExist`; "permission denied" is `os.ErrPermission`; an unreadable 550 is the reply itself. `GetFileInfo`/`FileExists` settle an unreadable 550 from the parent listing (present = exists, absent from a readable parent = does not exist, parent unreadable = an error). `FileExists` never answers `false` for a permission failure.
* **Degraded mode** (`allow_degraded_list`, no MLST): `LIST <dir>` is always sent with an explicit argument (an argument-less `LIST` lists the login directory); `GetFileInfo` lists the parent; the root itself is checked with `CWD`; `ModTime` is zero. Limits (UNCONFIRMED per server): some servers glob `*`, `?`, `[` in the argument and hide dotfiles without `-a`.
* `PASV` replies: only the port is used; the host in the reply is never dialled (SSRF), the control connection's peer is. Data connections over TLS that end without `close_notify` are read as EOF; the verdict is the final reply on the TLS-protected control channel.
* `FileInfo.Mode` is `os.ModeDir` for directories, `os.ModeSymlink` for symlinks, `os.ModeIrregular` for other special files and 0 for files; permission bits are not available from MLSD.
* Path confinement is lexical and **POSIX-only** (separator `/`); a Windows FTP server that also accepts `\` is not confined by it.
* Implicit FTPS (port 990) is not supported (`ErrUnsupportedTLSMode`).

## Wiring

`pkg/factory` (`ftp_factory.go`) builds the scan client with `ftp.NewScanClient` from the stored settings. Settings: `host`, `port` (1-65535, default 21), `username`, `credential_ref`, `path`, `tls_mode` (`explicit` by default, or `none`), `trusted_lan` (boolean), `allow_degraded_list` (boolean), `disable_epsv` (boolean).
No inline secret is accepted (`password`, `passphrase`, `private_key`, `privatekey`, `key`, `secret`, `token` are refused); a quoted boolean is refused. The pins come from the package-level **`ftp.DefaultPinStore`**, which the application sets once at start (like `sftp.DefaultPinStore`); without it every explicit-TLS root is refused (`ErrNoPinStore`, fail closed).
The factory therefore returns a **read-only** client (mutations return `decorators.ErrReadOnly`). `catalog-api/filesystem/ftp_client.go` is a separate duplicate and is untouched: no production path outside `pkg/factory` uses this package yet.

## Tests

* `go test ./pkg/ftp/ ./pkg/factory/` : unit tests against an in-process FTP/FTPS server (`fakeserver_test.go`, unit only: it models the working directory, per-session hooks that stall, cut a transfer, send extra replies, `421`, a permission `550`, odd MLSD lines). `-tags integration` adds `integration_test.go`.
* `scripts/test-infra/ftps_fixture.sh run -- -race -v ./pkg/ftp/` : the same behaviours against a **real pure-ftpd** (clear text + explicit FTPS, certificate generated per run), with a sink-side digest of the served tree before and after (companion guide `docs/scripts/ftps_fixture.md`).
* Mutation harness: `specs/001-full-project-audit-remediation/evidence/wp12/ftp/fix-r2-mutate.py` (round 2: the reviewer's mutants, one mutant per fix class, the factory wiring, two negative controls).

## Not covered

No NAS was contacted. Unconfirmed against the Synology FTP service: its MLSD fact format (case of `Type`, `Size`, cdir naming), whether it sends `421` at `USER` for per-IP limits, the leaf certificate SANs it presents, and the exact text of its permission `550`. pure-ftpd (the fixture) answers an early-closed download with one reply.
