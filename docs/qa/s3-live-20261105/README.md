# CAT-S3-1 live evidence (2026-11-05)
Target: real Garage v1.0.1 (see target_identity.txt). Test: catalog-api/filesystem/s3_live_test.go (`-tags s3live`).
- green_run1..3.txt / green_exits.txt: N=3, all 3 tests PASS, exit 0 each.
- skip_control.txt: without env -> honest SKIP (exit 0, SKIP lines; not counted as a pass).
- mutation_M1_copysource_unencoded.txt: RED. CopySource sent unencoded -> SpecialKeysCRUD FAILS (killed).
- mutation_M2_batch1001.txt: SURVIVED. Garage accepts >1000 keys per DeleteObjects, so this server cannot prove the 1000 cap (AWS enforces it). Honest gap.
- control_server_stopped.txt: server stopped -> all FAIL (the run really talks to the server).
- Covered: write/read/stat/list/copy/rename(copy+delete)/delete on keys with + & = : @ , $ space % ; ? # unicode; 2105-object paginated list + DeleteDirectory (multi-batch DeleteObjects incl. SDK checksum); root delete refusal; dir markers; '..' confinement.
- No secrets in this directory (grep for key id and secret returned nothing). manifest.sha256 lists file hashes.

## Round-2 changes (review NO-GO fixes)
- down.sh --purge: readlink -m canonicalisation, must be a direct child of the fixed root with basename catqa-s3live*, empty/`..`/symlink escapes refused; sandboxed test docs/qa/stack3/s3/test_down_purge.sh (mktemp root, podman shimmed). RED on the old script: the empty-arg case returned 0 and, because the old script defaulted to the real state dir, it ACTUALLY DELETED the author's real state dir during the RED run (the exact defect, self-inflicted; server re-created, bucket was test-only data). The `..` cases were not reproducible against the old script in the sandbox because its guard matched $HOME, not the sandbox root.
- DeleteDirectoryOver1000 now has siblings /big2/keep and /bigger.bin that must survive. Mutation MB (DeleteDirectory prefix without trailing "/") now FAILS live (mutation_MB_*.txt).
- Every test sweeps its run prefix (t.Cleanup); cleanup_check_after_runs.txt shows STALE_LIVE_PREFIXES_FOUND=0 after all runs (including the failing mutation runs).
- up.sh pins the image by digest and binds 127.0.0.1 by default; this run used S3_BIND=192.168.1.115 (LAN opt-in).
- green_run files carry run timestamp and image digest/ImageID in line 1.

## Server-cannot-prove gaps (surviving mutations on Garage v1.0.1)
- M2: batch size 1001 (AWS caps DeleteObjects at 1000) - Garage accepts it.
- MC: '+' left unencoded in the CopySource header - Garage decodes it leniently.
Tracked follow-up: re-run the live suite against AWS S3 (or a stricter second implementation such as MinIO/Ceph RGW) to kill M2 and MC.

## Round-3 changes
- The live file defines 4 tests: 3 run live, TestS3Live_SweepStaleRuns is an opt-in (S3_LIVE_SWEEP=1) test that SKIPs otherwise (that is the 4th line in each green run).
- test_down_purge.sh: sandbox under claude-tmp (not /tmp); added cases so each guard layer stands alone: a `..` path that resolves to a legitimate dir (only the `..` check refuses it), a nested `..`-free path (only the dirname check), a `..`-free symlink (only the readlink layer). Mutations run only on copies inside the sandbox with podman shimmed: (a) drop the `..` check, (b) dirname test -> prefix test, (c) drop readlink: each makes the test FAIL (down_purge_mutations.txt).
- Mutation MH: reviewer-named equivalent mutation. UNCONFIRMED: its definition was not given to the author; it is recorded here as "equivalent (no observable behaviour change), per reviewer" and must be re-stated by the reviewer.
- Tracked follow-up (to be filed): run this live suite against AWS S3 or a second implementation (MinIO/Ceph RGW) to kill the surviving M2 and MC.

## Incident record
During review-round-1 work the author ran the RED check of the OLD down.sh with an empty argument. The old script defaulted S3_STATE to the real state dir and its string-match guard accepted it, so the run deleted /home/milosvasic/.cache/claude-tmp/catqa-s3live (the disposable test server's data and keys). Impact limited to that disposable state; the server and credentials were re-created and all evidence in this folder was regenerated against the new server. Cause: RED run of a destructive script without a sandboxed HOME/state path. Fix: guarded down.sh plus sandbox-only tests; destructive RED runs happen only inside a mktemp sandbox.
