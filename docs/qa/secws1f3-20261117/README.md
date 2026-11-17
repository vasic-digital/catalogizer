# SEC-WS-1 F3 / rc.1 gate G4 (secws1f3) - PROVISIONAL, UNREPRODUCED-as-open-defect

Base HEAD af02b7c35cdfc0a43d08998f01e1d12da8a6fd0b (tree 2a547e796f33078c41e73cb5812a2545ea7abf9d), branch secws1f3.

## What F3 requires
docs/FOLLOW_UPS_NEXT_PHASE.md L2104 (SEC-WS-1-F3): a connected /ws socket must be re-checked after upgrade (revoked session / suspended user stops receiving) and BroadcastToClients must filter by audience. Gate G4 (docs/release/V2_5_0_GATE_CHECKLIST.md) records the F3 part as already on main (merge a9a43b9ce, hermetic-real tests) and the OPEN part as the LIVE /ws run.

## Finding: F3 is already fixed on main - no new source change made
Commits 94936f3d2, 234f8da71, 59201f0e1 (websocket_handler.go authorize/authorizeMessage/recheckLoop, audience-filtered broadcast). I did not reproduce a RED: there is no open defect to reproduce (11.4.292 gives no basis for a fix without an observed symptom). Invented work avoided.

## Evidence (host-fixture class: real DB, real middleware, real HTTP server, gorilla clients; NOT live)
- green.txt: package main TestF3_|TestWSRoute_ ok; green_handlers.txt: handlers WS tests ok.
- Mutants (all caught): M1 session check removed (5 tests FAIL, mutants.txt), M2 expiry check dropped in authorizeMessage (FAIL TestSEC_WS_1F3FU8_..., mutants_handlers.txt), M3 admin flag always true (5 FAIL, mutants_main.txt), M4 expiry check dropped in authorize (TestF3_TokenExpiryClosesSocket FAIL).
- mutants.txt M2/M3 lines there are from the wrong package/build-broken first attempt (M2 ran in package main where the covering test is not; M3 build failed); superseded by mutants_handlers.txt / mutants_main.txt. Kept for honesty.
- Source restored byte-identical after each mutant (cmp).

## Live part (still OPEN): LIVE_WS_CHECK.sh.txt
Non-executable text script for the operator (needs running server, credentials via env names, websocat, a suspendable test user). Not run. Token field names of /auth/login and the exact scan-trigger step are UNCONFIRMED; adjust. It does not record the process identity automatically (manual step noted).
