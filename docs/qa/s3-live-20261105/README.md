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
