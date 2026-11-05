# D11a / D11b / D7 fix evidence (2026-11-05) - PROVISIONAL-DONE

Branch fix-d11-d7. Each defect has red_*.txt (pre-fix), green_*.txt (post-fix), mutations_*.txt (reviewer-style mutations, all killed).
- D11a: truncated source converts completed-with-warning (job.warning, migration v25).
- D11b: scan imports sidecar subtitles (confined, idempotent).
- D7: empty JSON body no longer rejected by validation middleware before auth.
- root_package_tests.txt: full root-package run; one failure, TestOpenAPIContract_RealBinary_PathParamsAndUserRolePermissions (DELETE /logs/share/999999 expects 500, gets 404).
- baseline_openapi_contract_test.txt: same test on unmodified base 40c07e60 fails identically -> PRE-EXISTING, not caused by this branch (tracked in follow-ups).
Limits: PostgreSQL migration not run live; live rerun on real target still owed.
