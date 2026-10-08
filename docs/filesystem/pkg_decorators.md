# Package `digital.vasic.filesystem/pkg/decorators`

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-07 |
| Last modified | 2026-10-07T20:30:00Z |
| Status | implemented in the working tree of the `submodules/filesystem` submodule (own-org, NOT committed there); independent review round 1 (WF19, 2026-10-07) returned NO-GO, round 2 (this revision) fixes every finding, the re-review is OWED; not yet linked from docs/README.md (11.4.212, owed to the conductor) |
| Source | `submodules/filesystem/pkg/decorators/readonly.go`, `readonly_test.go`; task PA-02 of `specs/001-full-project-audit-remediation/evidence/wp12/protocol-analysis/REPORT.md`; evidence `specs/001-full-project-audit-remediation/evidence/wp12/fabric/` |

## Purpose

`decorators.ReadOnly(inner client.Client) client.Client` wraps any storage client so that a catalog scan can never modify a storage host. The five mutating methods of `client.Client`
(`WriteFile`, `DeleteFile`, `CopyFile`, `CreateDirectory`, `DeleteDirectory`) return an error satisfying `errors.Is(err, decorators.ErrReadOnly)`; the inner client is not called and the data reader of
`WriteFile` is not consumed. Connection management (`Connect`, `Disconnect`, `IsConnected`, `TestConnection`), reads (`ReadFile`, `GetFileInfo`, `FileExists`, `ListDirectory`) and metadata are forwarded.
If the inner client implements `client.SeekableClient` so does the result (opening for reading is not a mutation). A nil inner client, **including a typed nil** such as `(*local.Client)(nil)`, panics at construction.

### What the read methods hand out

Refusing the five mutators is not the whole guarantee, because `GetConfig`, `ReadFile` and `OpenSeekable` return values from the inner client:

- `GetConfig()` returns a **redacted defensive copy** (`guard.RedactConfig`): every exported field or map key whose name denotes a secret (password, passphrase, secret, token, private key, api key, credential) is blanked, and the copy is detached from the live config, so the holder of the read-only handle can neither read the credentials nor re-target the inner client (`Share`, `BasePath`, ...) by editing the returned value. Public fields are kept. Limit: pointers hidden in *unexported* fields are shared, not copied.
- `ReadFile` / `OpenSeekable` return a stream that exposes **only** `Read`, `Close` and, when the inner stream has them, `Seek`, `ReadAt` and `WriteTo` (`guard.Wrap`). The inner stream's own methods (for an `*os.File`: `Chmod`, `Chown`, `Truncate`, `Sync`, `Fd`) are not reachable, so the stream cannot change metadata on disk (reviewer probe RV22 showed `Chmod` working on a read-only descriptor before this change).

## Why a new method cannot slip through

The wrapper does not embed the interface, so a method added to `client.Client` later does not compile until the wrapper implements it explicitly. In addition
`TestReadOnly_ClassifiesEveryInterfaceMethod` reflects over `client.Client` and fails for any method that is not in the reviewed classification table, or whose name starts with a mutating verb
(Write, Delete, Create, Copy, Move, Rename, Remove, Mkdir, Chmod, Chown, Truncate, Symlink, Link, Put, Append, Upload, Set, Touch, Save, Update, Mv, Rm) but is not classified as a mutator.
The instrument is control-needled: it must see 15 methods including `WriteFile`, and `TestReadOnly_ClassifierSeesAnInjectedMutator` proves that an interface extended with `MoveFile` is reported.

## Usage

```go
c := decorators.ReadOnly(smbClient)          // scans get c, never smbClient
err := c.DeleteFile(ctx, "/x")               // errors.Is(err, decorators.ErrReadOnly)
```

Recommended chain, outermost first (see [pkg_fabric.md](pkg_fabric.md)): `Metered(Retrying(Limited(Confined(ReadOnly(protocolClient)))))`.

## Tests

`review_probes_test.go` holds the independent reviewer's probes (WF19) adopted verbatim as permanent regression tests (RV20 typed nil, RV21 config leak, RV22 stream `Chmod`), `fix_test.go` the class tests (read paths forwarded, config redaction per protocol, stream capabilities) and `guard/guard_test.go` the helper package. Unit tests use a recording fake (allowed for unit tests only); `TestReadOnly_RealLocalFilesystemUntouched` and `TestChain_RealLocalFilesystem` (fabric package) run against the real local client and a real
directory and check the disk afterwards. `FuzzReadOnly_Mutators` drives all mutators with arbitrary path text. Run only in the pinned Go container:
`bash scripts/containers/run_pinned.sh --out <dir> IMG-GO -- sh -c 'cd /src/submodules/filesystem && go test -race -count=1 ./pkg/decorators/'`.

## Limits (not claimed)

- It enforces read-only at the Go interface. A protocol client that exposes a second, non-`client.Client` write path to its caller is not covered; callers must hold only the decorated value. The statement "a catalog scan can never modify a storage host" therefore holds for the `client.Client` surface including what `GetConfig`, `ReadFile` and `OpenSeekable` return; it does not extend to a write path outside that interface or to the server side.
- Redaction is by field/key **name**: a secret stored under a name that does not denote one (for example `Settings["pw"]`) is not blanked. Protocol clients should keep secrets in fields named for what they are.
- Server-side protection of an open stream (an SMB open mask, an NFS export mounted read-write by legacy `pkg/nfs`, whose mount flags allow SETATTR) is the protocol client's job. The NFS case is UNCONFIRMED at runtime (it needs a root mount that was not attempted).
- It does not prove the remote account is read-only on the server. Use a read-only account as well; the NAS read-only legs (`scripts/test-infra/nas_readonly_leg.sh`) remain the server-side check.
