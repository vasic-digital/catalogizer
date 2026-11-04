# Storage clients fix (ST-D1, ST-D2, ST-D3), PROVISIONAL-DONE
Roadmap-Item: CAT-ST. Base main 4d04560f. Files: catalog-api/filesystem/{ftp_client.go,ftp_ops.go,smb_client.go} + tests ftp_realserver_live_test.go, smb_ctx_unit_test.go.

## RED (pre-fix code, same tests, real pyftpdlib FTP server process on loopback)
- TestLiveRealFTP_SharedClientParallelStat: FAIL, "4 goroutine(s) saw wrong results, first: g1 stat f15.bin: size=&{f15.bin 149 ...} err=<nil> want 205" (a stat returned ANOTHER file's reply).
- TestFTPOperationHonoursContextDeadlineOnSilentServer: FAIL after 15.01s, "HANG: operation ignored its context deadline on a silent server".
## GREEN (fixed code)
- `go test ./filesystem -race -count=3` of the new tests: ok (8.2 s); whole filesystem package with -race: ok (63.7 s); `go vet ./...` in catalog-api: clean.
## Fix
- FTP: per-client operation turn (channel semaphore, honours ctx while waiting) around TestConnection/ReadFile(RETR command)/WriteFile/GetFileInfo/ListDirectory/FileExists/Create/Delete*/CopyFile; read/write deadline on the control and passive data connections = earlier of ctx deadline and 5 min (ftpOpTimeout), cancellation mid-operation forces an immediate deadline; a cut-short operation replaces the (possibly desynced) session; data connections are dialled with the operation ctx instead of the Connect ctx; ctx.Err() checked before any network traffic.
- SMB: Connect uses DialContext and smb2.Dialer.DialContext; every operation checks ctx first and uses a ctx-bound share handle; ReadFile/OpenSeekable/CopyFile check ctx up front (returned handles are not ctx-bound, to avoid breaking callers that read after their request ctx ends).
- ST-D3: smbPath reads a leading "/" as share-relative ("/x" -> "x", "/" -> "."). go-smb2 rejects such paths, so no working caller changes; relative paths are untouched.
## Paired mutations (each makes its test FAIL; restored afterwards)
M1 remove op semaphore -> parallel-stat FAIL; M2 remove ctx.Err() check (FTP) -> FTPCancelled FAIL; M3 remove deadline arming -> silent-server FAIL (15 s hang); M4 SMB DialTimeout/Dial instead of ctx -> mid-handshake FAIL (20 s); M5 remove SMB ctx checks -> OperationsFail FAIL; M6 disable leading-slash rule -> SmbPath FAIL.
## Known limits
- ReadFile: the turn covers the RETR exchange only; reading the returned transfer while using the same client concurrently still interleaves (one client, one active transfer).
- Live Synology (SMB/FTP) confirmation of these changes was NOT run (no Synology access used); owed. FTP real-server tests need python3 + pyftpdlib (FTP_PYFTPDLIB_DIR; Python >= 3.12 also pyasynchat/pyasyncore) and SKIP honestly otherwise.
