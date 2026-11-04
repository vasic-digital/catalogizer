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

## Review round 1 (Opus NO-GO) remediation, head after 3cae7498
- F1: session dial (connect, banner, Login, CWD/PWD) armed with ftpDialTimeout + ctx deadline + ctx-cancel expiry (armDial); applies to Connect and the reconnect after a cut. Tests TestRV_ReconnectAfterCutBounded (RED: 25 s HANG; GREEN 10.5 s = the 10 s reconnect bound), TestFTPConnectSilentBannerHonoursContext.
- F2: ftpOpTimeout is now an IDLE bound pushed forward on every Read/Write of a data connection (control channel deadline moves with it); downloads handed to the caller after the operation are uncapped as before. TestRV_LongUploadExceedsOpCap (RED: "i/o timeout" at 1.12 s; GREEN 3.3 s), TestRV_LongDownloadExceedsOpCap.
- F3: TestRV_SessionReplacedAfterCut, TestRV_CancelMidOpUnblocks, TestFTPSilentDataConnectionHonoursContext, TestFTPSilentDataConnectionEndsByIdleBound, TestFTPCancelUnblocksSilentDataConnection, TestFTPTurnReleasedWhenNotConnected.
- F4: release(opErr): the session is replaced only when the operation FAILED and ctx/deadline cut it; TestLiveRealFTP_SuccessKeepsSessionWhenContextEndsLate.
- F6: smbPath leading separators are trimmed (done); repeated INTERNAL separators ("a//b") are deliberately left as given, since that is today's behaviour for working callers.
- Evidence files here: red_review_r1_probes.txt (RED on 3cae7498), mutations_round2.txt (mutants), green_new_tests_race_x3.txt, green_filesystem_package_race.txt, green_go_vet.txt, full_package_verbose_race.txt. Commands: `systemd-run --user --scope -p MemoryMax=4G -p MemorySwapMax=0 go test ./filesystem -race -count=N -run ...` in catalog-api with FTP_PYFTPDLIB_DIR set; `go vet ./...`.
- Mutants killed (round 2): no-session-replacement, watcher no-op, data conns without initial deadline (idle test), cancel skipping a connection, turn leak on session error, data dial on Connect ctx, cut ignoring op error, no dial deadline, no idle progress push; round-1 mutants M1..M6 also still apply.

## Review round 2 (Opus NO-GO) remediation
- N-1: CopyFile's second session is dialled through dialWith(ctx, tr.dialFunc(ctx)): the tracker's connections are registered in the deadline group (TestR2_CopyFileSecondDialHonoursCtx: RED 21 s HANG, GREEN).
- N-3: disarm clears the cancelled flag (TestR2_StickyCancelledFlag).
- N-4: replacement of a cut session is LAZY (staleConn, replaced by the next operation under its own ctx and the dial bound); TestFTPCutOperationReturnsWithoutWaitingForRedial (RED 10.5 s, GREEN ~0.5 s).
- N-2 tests: TestFTPSessionReplacedAfterIdleTimeout (N3), TestLiveRealFTP_CopyFileLongAfterConnect (N2), TestFTPReadFileCutReplacesSession (N5); long-download test has its own 30 s guard.
- Mutants killed (mutations_round3.txt): N1, N2, N3, N3b (sticky flag), N4 (no stale replacement), N5 (ReadFile release(nil)).
- ST-F7 (backslash `..` in smbPath) registered in FOLLOW_UPS as UNCONFIRMED, not changed (hasDotDotSegment is shared with FTP/WebDAV).
- Evidence manifest: MANIFEST.txt (base commit, UTC timestamp, sha256 of every file here); the evidence was produced on the working tree committed in the commit that adds this manifest.

## Review round 3 (final fix round) remediation
- S1: smbPath replaces `\` by `/` before the `..` check (SMB-only). TestSmbPath_BackslashDotDotConfined (RED before: red_review_r3.txt). ST-F7 is FIXED.
- T1: reviewer tests TestR3_* adopted (ftp_review_r3_unit_test.go).
- M1: Disconnect of a cut (stale) session aborts it and ignores its dead write error; GetFileInfo returns the ctx error right after a cut SIZE instead of falling through to LIST. Tests TestFTPDisconnectAfterCutNoSpuriousError (10 rounds, timing dependent), TestFTPGetFileInfoCutReportsContextError.
- M2: a failed lazy replacement sets redialNeeded (tied to the Disconnect generation); a later operation retries under its own ctx. TestFTPFailedReplacementRetriedByLaterOp; TestFTPDisconnectAfterFailedReplacementNoRedial guards against resurrection after Disconnect.
- Mutants (mutations_round4.txt): L4 lockOps without replacement (killed by hanging until the 100 s test timeout, i.e. CopyFile ran on the desynced session), S1, M1a, M1b, M2, L6 (redial after Disconnect) all killed. Note: a first L6 variant (Disconnect without gen++) survived because Disconnect clears staleConn and reconnectIf's client==old guard also protects; the stronger variant (no gen check and no redialNeeded clear) is killed.
- Correction: the first S1 implementation used strings.ReplaceAll inside smbPath, which the existing guard TestWF22R3_A_EveryResolverUsesTheSharedRule forbids (it caught it in the whole-package run, commit 1400b74d was therefore red); replaced by strings.Map in the following commit. Commit 911913f8 was then still red once in two runs: TestFTPGetFileInfoCutReportsContextError was timing dependent (the connection deadline can fire before ctx.Err() turns non-nil). GetFileInfo now also compares the ctx deadline with the clock and returns early on a net timeout; the test passes 40/40 under -race. The green logs in this directory are from the final tree (new tests -race x3, whole package -race, go vet ./... all green).
