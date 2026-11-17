# N7FU evidence (PROVISIONAL, unit-class: real router/service/repository on in-memory SQLite, no live server)
- fu2_red.log / fu2_green.log / fu2_mutant.log: N7-FU2 RED (5 of 6 routes fail), GREEN, mutant killed.
- fu1_green.log / fu1_mutant.diff / fu1_mutant.log: N7-FU1 guard and the killRun mutant.
- fu3_M7_reconstructed.diff/.log: reconstructed mutant (not the original M7).
- full_services_handlers.log: go test ./services ./handlers.
Host load ~55: GOMAXPROCS=2 nice -n 10 go test -p 1.
