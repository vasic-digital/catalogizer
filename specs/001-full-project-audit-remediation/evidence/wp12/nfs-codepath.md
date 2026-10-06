# identity: T127 read-only V-08, how the NFS code path mounts
# head: 960c553a198d9c17218cb2ad4283a599bdc83b04 (main), submodules/filesystem 66b6bc1cd9385d8548535f8970ad9fd7049b4b1b
# run_at: 2026-10-06 (read-only: no source file was edited)
# note: tasks.md names the evidence directory wp13; this run was told to write wp12, so the file lives here (the T134 NFS records go to wp10 as instructed).

## Finding (FACT, from source)

Both NFS client implementations in the tree mount the share through the KERNEL, never through a user-space NFS client library:

| Path | Evidence |
|---|---|
| `submodules/filesystem/pkg/nfs/nfs.go` `(*Client).Connect` | `os.MkdirAll(mountPoint)`, then `syscall.Mount("<host>:<path>", mountPoint, "nfs", 0, "vers=3")`; `Disconnect` calls `syscall.Unmount`; `IsConnected` also reads `/proc/mounts` (`isMounted`); every file operation (`ReadFile`, `WriteFile`, `CopyFile`, list) is an ordinary `os.*` call on the mount point (`resolvePath` joins `mountPoint`). |
| `catalog-api/filesystem/nfs_client.go:65` | the same `syscall.Mount(source, mountPoint, "nfs", 0, options)` call (a second copy of the client in the application itself). |
| `submodules/filesystem/pkg/factory/nfs_linux.go` | builds `nfs.NewNFSClient` with `host`, `path`, `mount_point`, `options` (default `vers=3`) from the storage settings; `nfs_other.go` (non-Linux) returns the error `NFS protocol is only supported on Linux`. |
| `catalog-api/internal/services/protocol_handlers.go` (`NFSProtocolHandler`), `universal_scanner.go` (`NFSScanner`) | operate only through the `filesystem.FileSystemClient` interface (copy, delete, list); no mounting of their own. |
| `catalog-api/internal/tests/protocol_helper.go:157-171` | `NFSMockConfig` "doesn't mount an actual NFS share ... Real NFS testing would require proper mount permissions" (the existing NFS test is a mock, constitution 11.4.27 forbids that outside unit scope). |

`mount(2)` with filesystem type `nfs` needs CAP_SYS_ADMIN in the user namespace that owns the mount namespace and an in-kernel NFS client; an unprivileged rootless container has neither on this host (owner decision: no `--privileged`, no kernel mounts).

## Consequence for DR-16-2 (input, not a verdict)

1. The application's own NFS path (kernel mount) CANNOT be exercised by a rootless, unprivileged run on this host. That is a statement about the application code path, not about NFS servers.
2. A user-space NFS server container plus the libnfs user-space utilities of IMG-INFRA-CLIENT (`nfs-ls`, `nfs-cp`, `nfs-cat`, T134) can prove the NFS PROTOCOL round trip (server and client), but never the application's `syscall.Mount` path. A T134 `pass` therefore proves "an NFS server reachable and readable/writable over the protocol by a user-space client", and the application path stays UNCONFIRMED on this host (it needs a host that permits a kernel NFS mount: the remote build host or an owner NFS host, T134a / ODG-08).
3. The structural-impossibility scope of DR-16-2 item 3, if ever recorded, is bounded to: the application's kernel-mount path under rootless podman. It does not extend to FTP, SMB or WebDAV.

UNCONFIRMED: whether the application is ever expected to move to a user-space client (a code change, outside this task).
