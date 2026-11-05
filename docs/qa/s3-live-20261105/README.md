# CAT-S3-1 live evidence (2026-11-05)
Target: real Garage v1.0.1 (see target_identity.txt). Test: catalog-api/filesystem/s3_live_test.go (`-tags s3live`).
- green_run1..3.txt / green_exits.txt: N=3, all 3 tests PASS, exit 0 each.
- skip_control.txt: without env -> honest SKIP (exit 0, SKIP lines; not counted as a pass).
- mutation_M1_copysource_unencoded.txt: RED. CopySource sent unencoded -> SpecialKeysCRUD FAILS (killed).
- mutation_M2_batch1001.txt: SURVIVED. Garage accepts >1000 keys per DeleteObjects, so this server cannot prove the 1000 cap (AWS enforces it). Honest gap.
- control_server_stopped.txt: server stopped -> all FAIL (the run really talks to the server).
- Covered: write/read/stat/list/copy/rename(copy+delete)/delete on keys with + & = : @ , $ space % ; ? # unicode; 2105-object paginated list + DeleteDirectory (multi-batch DeleteObjects incl. SDK checksum); root delete refusal; dir markers; '..' confinement.
- No secrets in this directory (grep for key id and secret returned nothing). manifest.sha256 lists file hashes.
