# Conversion defects fix, review round 1 follow-up (2026-11-04)

Base: 9bba3746 (round 0). This change: see `git log` for the commit that adds this directory.
Roadmap-Item: CAT-CONV. Status: PROVISIONAL-DONE (live re-run on a rebuilt stack owed).

- red-services.log, red-handlers.log: RED runs of the new TestRev_* tests before the fixes
  (F3, F4, F5 and the notification test fail in services; F1, F2 fail in handlers).
- mutations.log: each mutant applied, test run, mutant reverted (all killed).
- green-services-tests-full.log: full `go test ./services ./tests`.
- green-handlers-race-full.log: full `go test -race -timeout 30m ./handlers`.
- green-handlers-race-attempt1-timeout10m.log: first race attempt hit the default 10m timeout on a loaded host
  (no test failure, no DATA RACE); `./repository` -race ok (100.8s) is the last line pair in that log.
- vet.log: `go vet ./services ./handlers ./repository ./tests` (empty = clean).
- MANIFEST.sha256: sha256 of the files above.
