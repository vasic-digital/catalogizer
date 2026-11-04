# Conversion defects fix, review round 1 follow-up (2026-11-04)

Base: 9bba3746 (round 0); round-1 follow-up bd9d2ada; round-2 (N1) commit follows bd9d2ada. Manifest regenerated 2026-11-04T23:54:11+01:00. This change: see `git log` for the commit that adds this directory.
Roadmap-Item: CAT-CONV. Status: PROVISIONAL-DONE (live re-run on a rebuilt stack owed).

- red-services.log, red-handlers.log: RED runs of the new TestRev_* tests before the fixes
  (F3, F4, F5 and the notification test fail in services; F1, F2 fail in handlers).
- mutations.log: each mutant applied, test run, mutant reverted (all killed).
- green-services-tests-full.log: full `go test ./services ./tests`.
- green-handlers-repository-race-full.log: full `go test -race -timeout 30m ./handlers ./repository` (round 2).
- red-n1-services.log: RED run of the N1 path-after-colon / quoted-path cases before the fix.
- vet.log: `go vet ./services ./handlers ./repository ./tests` (empty = clean).
- MANIFEST.sha256: sha256 of the files above.

Round 3: fd3ac5db (R3-1 unclosed-quote paths; evidence logs were gitignored before and are now force-added). Merged main a61d56c6 in 6a43eaa8.
- red-r3-services.log: RED for the unclosed-quote cases.
- green-merged-conversion-subtitle-race.log: `go test -race` of conversion/subtitle/TestRev/TestD tests on the merged tree (services, handlers, repository, tests), plus go build ./... and vet.log clean.
- green-services-tests-full.log, green-handlers-repository-race-full.log: full-package runs of round 2 (code 8cc5b709).

Round 4: root package. red-root-conversion-tools.log = old TestConversionToolsMissing_RealBinary red on the branch (expected 200, got 400); green-root-package.log = full root package: only the pre-existing main failure TestOpenAPIContract_RealBinary_PathParamsAndUserRolePermissions (DELETE /api/v1/logs/share/999999 404 vs 500) fails; TestConversionToolsMissing_RealBinary passes.
