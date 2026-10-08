# settings contract (storage protocol settings keys) - Companion Guide (PA-01, WP-12)

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-10-07 |
| Last modified | 2026-10-08T00:00:00Z |
| Status | in the working tree (PA-01 / tasks T361, T362), not yet committed. Revision 2 is the fix round for the independent review WF22; a fresh independent review (11.4.142 / 11.4.209) is owed; its row in `docs/scripts/README.md` is owed (that file is not touched by this change) |
| Source | `catalog-api/filesystem/settings.go`, `catalog-api/filesystem/factory.go`; tests `catalog-api/filesystem/settings_test.go`, `wf22_fix_r2_test.go`, `catalog-api/internal/services/settings_contract_test.go`, `wf22_createclient_gate_test.go` |
| Evidence | `specs/001-full-project-audit-remediation/evidence/wp12/scanner/` |

## Purpose

One vocabulary of settings keys, one mapping from a `StorageRoot` row to the factory settings (`filesystem.SettingsFromRoot`, used by the universal scanner, the stream handler and the comic-pages
handler and the cover-art thumbnail copy - FOUR hand-built copies before; the review found the fourth), and a factory that REJECTS a key the protocol does not consume. Before: the scanner wrote `export_path` for NFS while the factory read `path` (the export was always empty),
and the FTP and WebDAV `path` was never forwarded.

## Vocabulary and per-protocol schema

| Protocol | Keys |
|---|---|
| local | `base_path` |
| smb | `host` `port`(int) `share` `username` `password` `domain` |
| ftp | `host` `port`(int) `username` `password` `path` |
| nfs | `host` `path` `mount_point` `options` |
| webdav | `url` `username` `password` `path` |
| registered (sftp, ftps, nfs3, ...) | whatever its `ProtocolSpec.Keys` declares; the shared names are `host port path username credential_ref tls_mode host_key_fingerprint cert_sha256` |

`share` is the SMB spelling of the root path (the share name). A retired spelling (`export_path`, `filepath`, `file_path`, `root`, ...) is rejected with a hint (`export_path (use path)`); the error names
keys only, never values. A wrong type (`port` as a string) is rejected too, and so is a number outside the 32-bit range (a `port` of `1e300` is not a port); a `port` must be 1-65535 (a stored port of 0 means "not set": the mapping leaves it out). No key is
required by the validator (Connect reports a missing host).

## Semantics of the shared keys (revision 2)

| Key | Meaning |
|---|---|
| `path` (ftp) | the base directory. `Connect` changes into it and records the server's own answer to `PWD` as the absolute base; every path is resolved against that, so a RELATIVE path (`movies`) is not applied twice. A server that does not answer `PWD` gets the configured path anchored at `/` |
| `path` (webdav) | the root collection UNDER the url's own path: url `https://nas/remote.php/dav/files/alice` + path `/Movies` addresses `.../alice/Movies`. It never replaces the url path |
| `path` (nfs) | the export path (`export_path` is rejected with the hint `use path`) |
| `domain` (smb) | a non-empty `domain` column wins over the identity's domain; an EMPTY column does not erase it |
| `options` | nfs: mount options (`vers=3,ro`); other protocols: a JSON object with `credential_ref`, `tls_mode`, `host_key_fingerprint`, `cert_sha256` |

Everything that builds a client reads its root through one set of columns, `models.StorageRootConnColumns` (`protocol host port path username password domain url mount_point options`) and one mapping. Callers (all in
`catalog-api`, enumerated from the source by `TestWF22_R1_Gate_EveryCreateClientCallerUsesTheSingleSettingsMapping`): `universal_scanner.go`, `cover_art_service.go`, `internal/handlers/stream_handler.go`,
`handlers/comic_pages_handler.go`, `handlers/pdf_pages_handler.go`. A sixth caller that builds its own map fails that test.

## Pluggable protocols

```go
filesystem.RegisterProtocol(filesystem.ProtocolSpec{
    Name: "sftp",
    Keys: map[string]filesystem.KeyType{filesystem.KeyHost: filesystem.KeyString, filesystem.KeyPort: filesystem.KeyInt, filesystem.KeyPath: filesystem.KeyString,
        filesystem.KeyUsername: filesystem.KeyString, filesystem.KeyCredentialRef: filesystem.KeyString, filesystem.KeyHostKeyFingerprint: filesystem.KeyString},
    New: func(s map[string]interface{}) (filesystem.FileSystemClient, error) { /* build the client from validated settings */ },
})
```

`DefaultClientFactory.CreateClient` validates the settings against the registered schema and calls `New`; `SupportedProtocols()` lists the built-ins first, then the registered ones sorted;
`UniversalScanner` scans a registered protocol with the generic scanner. For a non-built-in protocol `SettingsFromRoot` maps only root fields its schema allows and reads `credential_ref`, `tls_mode`,
`host_key_fingerprint`, `cert_sha256` from `Options` when it is a JSON object, so a schema without `password` never receives an inline secret.

**Not wired here:** `catalog-api` does not import `submodules/filesystem/pkg/{sftp,nfs3,ftp}` (concurrent work, and importing them needs `github.com/pkg/sftp` in `catalog-api/go.mod`). The registration call of
each is a one-liner for whoever adds the dependency; no FTPS client exists yet (UNCONFIRMED when PA-04 lands).
