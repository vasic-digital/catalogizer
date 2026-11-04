# Live storage/scan investigation r1 (clone of main 4c79b7e3; no live-repo change)
Evidence bundle: /home/milosvasic/.cache/claude-tmp/live-storage-r1/ (yandex/, yandex_scan/, smb/, ftp/, nfs/, old_runs/, tests/). Credentials: 0 hits (password strings of both env files grepped). The 6-char SMB username matches only as an ordinary substring inside a Go runtime goroutine-dump word in old_runs/ftp_run3.raw (not a leak).
Clone-only test files (not committed anywhere): tests/live_synology_{sandbox,clean,nfs}_test.go. Yandex used the product's own existing live suites (filesystem/live_webdav_yandex_test.go via scripts/test-infra/webdav_yandex_live.sh, internal/services/live_webdav_scan_test.go).

## Results (N=3 per scenario; machine fields in the JSON files)
| Target | Client | Result |
|---|---|---|
| Yandex WebDAV (https, /Testing only, guard transport) | WebDAVClient, 17 steps (connect, wrong-password 401 w/o secret, mkdir, upload sha256, list, download+stat, Range, copy/move, overwrite, nonexistent, unicode, 200-entry listing, 4 concurrent uploads, unsized upload, timeout/cancel, delete, guard) | 3/3 PASS, signatures identical; VERDICT=PASS_WITH_ANOMALIES: 4 transient "connection reset" retries across 3 runs, p50 ~225 ms, p95 1.4-2.1 s, max single request ~129 s (service stall); 0 lost acked writes |
| Yandex scan | GenericScanner (WebDAV) + real client, recorder seam | 3/3 PASS (8 entries, 0 scan errors, 0 guard violations, sha256 verified) |
| Synology SMB work20/Testing (192.168.1.157 LAN, SMB2/3 NTLM) | SmbClient 12 ops steps + real SMBScanner into real sqlite (13b) | 11 of 12 ops steps + scan 13b PASS in 3/3; 1 FAIL/run: Connect with cancelled ctx (defect D1). 5 MiB sha256 identical every run (6ee0aa74...). scan: 12 DB rows, 0 errors |
| Synology FTP (banner "220 Synology5 FTP server ready", root /WORK20/Testing) | FTPClient + generic FTP scanner | 3/3 PASS (13 steps, step 9 skipped by design, see D2); scan 10 entries 0 errors |
| Synology NFS | NFSClient | SKIP: mountd up (rpcinfo) but `showmount -e` export list empty for this client; uid 1000 has no CAP_SYS_ADMIN and no NFS mount exists. Product Connect fails cleanly in 1 ms ("operation not permitted", IsConnected false) 3/3. Not faked. |

Read-only reader creds from repo .env were not needed/used.

## Defects / findings
D1 (product, low-medium): SmbClient.Connect ignores ctx (smb_client.go:42-47 uses net.DialTimeout(10s) and session Dial with no ctx); all SMB ops (stat/read) also ignore a cancelled ctx (returned success). FTP Connect honours ctx; FTP stat/read with cancelled ctx also ignore it. Reproduced 3/3 (step 12). Fix: DialContext + check ctx.Err() at op entry / ctx-aware deadlines.
D2 (product hazard, documented limit, severity depends on callers): one FTPClient used from 4 goroutines hangs/corrupts the control connection (parallel GetFileInfo: 2-3 of 4 failed, then FileExists(nonexistent) returned a 550 instead of (false,nil), list of nonexistent dir "succeeded", scan failed with 550 Data connection; in one run the process hung in textproto.ReadResponse inside GetFileInfo->fileModTime->GetTime with no read deadline: stack in old_runs/ftp_run3.raw). Code comment (ftp_client.go:54-58) admits ServerConn is not goroutine-safe and production creates one client per job; "other handlers not audited". Proposed fix: per-client operation mutex (serialise commands) plus per-command read deadlines tied to ctx. Without the shared-client step FTP is 3/3 green, confirming the cascade was from interleaving.
D3 (API inconsistency, not a defect in the scan path): SmbClient rejects leading-slash paths ("leading '\' is not allowed"), whereas FTP/WebDAV/Local take "/x". Production SMB scan uses SMBScanner with ConfineScanPath keeping relative form, which works live (13b). Using the generic scanner for SMB (my first attempt) fails with root_unreadable: test defect, not product.
D4 (service behaviour, environment): Yandex WebDAV intermittent connection resets and up-to-129 s stalls; product retries/handles; not a product defect.
Test defects found and fixed in my own test: wrong SMB base path; my cleanup trusted FileExists on a desynced FTP connection and left 4 run dirs (see cleaned below).

## Leftovers cleaned
Yandex /Testing: empty after runs (suite deletes own run dir; verified by PROPFIND). Synology work20/Testing: 4 FTP run dirs from the early broken (shared-client) runs were left; deleted by exact path (curl DELE/RMD + product FTPClient for the unicode names). Final listing: only the pre-existing rev_1793656583_3863029. SMB runs left nothing (verified by listing in each run). Scratch NFS mountpoint dirs removed. Not modified: live repo (status clean). No processes left except none.
Note: I sent SIGQUIT to my own hung test binary (pid-targeted) to obtain the goroutine dump.
