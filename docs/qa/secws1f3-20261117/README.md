# SEC-WS-1 F3 / rc.1 gate G4 (secws1f3, review round 2 fixes F1..F8) - PROVISIONAL, UNREPRODUCED-as-open-defect

Base HEAD af02b7c35cdfc0a43d08998f01e1d12da8a6fd0b (tree 2a547e796f33078c41e73cb5812a2545ea7abf9d), original branch secws1f3; this
round is on branch secws1f3b (merged with local main). Docs/scripts only: no source change.

## What F3 requires
docs/FOLLOW_UPS_NEXT_PHASE.md L2104 (SEC-WS-1-F3): a connected /ws socket must be re-checked after upgrade (revoked session / suspended user stops
receiving) and BroadcastToClients must filter by audience. Gate G4 (docs/release/V2_5_0_GATE_CHECKLIST.md) records the F3 part as already on main
(merge a9a43b9ce, hermetic-real tests) and the OPEN part as the LIVE /ws run.

## Finding: F3 is already fixed on main - no new source change made
Commits 94936f3d2, 234f8da71, 59201f0e1 (websocket_handler.go authorize/authorizeMessage/recheckLoop, audience-filtered broadcast). No RED was
reproduced: there is no open defect to reproduce (11.4.292 gives no basis for a fix without an observed symptom).

## Evidence (host-fixture class: real DB, real middleware, real HTTP server, gorilla clients; NOT live)
- green.txt (re-run this round; command, HEAD and tree hash recorded in it): `cd catalog-api && go test -p 1 -count=1 -run 'TestF3_|TestWSRoute_|TestSEC_WS_1F3FU8' .`
  (package main; 20 PASS lines). TestSEC_WS_1F3FU8 matches no test in package main: it lives in package handlers and is covered by green_handlers.txt
  (`-run 'TestSEC_WS_1F3FU8|TestWS|TestWebSocket' ./handlers`, from the earlier round at HEAD af02b7c35).
- mutants_rerun.txt: four mutants re-run this round with the exact old/new strings recorded, both packages each, restore verified by cmp
  (driver: mutants_driver.py.txt). The earlier files (mutants.txt, mutants_main.txt, mutants_handlers.txt) are the first-round record:
  mutants_main.txt / mutants_handlers.txt came from an interrupted batch (M4 was re-run singly), and mutants.txt holds a wrong-package / build-broken
  first attempt. They did not record the old/new strings; mutants_rerun.txt supersedes them.

## Live part (still OPEN): LIVE_WS_CHECK.sh.txt + ws_probe.py.txt
SCOPE, stated plainly: the script is a SERVER-SIDE check using raw clients (curl + a stdlib python probe; websocat is no longer needed). It does NOT
satisfy gate G4's requirement of a live run from catalog-web AND Android through the production nginx edge, and it does NOT cover SEC-WS-1-F1.
The audience-filter leg is NOT covered (manual / uncovered; the script reports it INCONCLUSIVE).
It was NOT run against a real catalog-api by the author. The token field name of /auth/login and the suspension action are UNCONFIRMED; adjust.
- Verdicts: every check is PASS / FAIL / INCONCLUSIVE from comparing observed vs expected; INCONCLUSIVE is never PASS. Exit 0 / 1 / 3 (2 = prerequisite).
- F3 check: the socket is proven alive by ping/pong before the suspension step (else INCONCLUSIVE), the close code is parsed (expect 1008
  'authorization ended') and the time from suspension to close is compared with the bound (F3_BOUND, default 20 s).
- Credentials never in argv (jq -n --arg piped to curl --data @-); the ?token= probe uses a deliberately invalid JWT. Network calls have timeouts.
- Process identity (11.4.289(D)): recorded when SERVER_PID / SERVER_EXE_SHA256 are supplied, else the run prints `live_proven=NO (UNCONFIRMED)`.

## Gate-logic unit test of the script (NOT live evidence, 11.4.289(C))
GATE_LOGIC_TEST_run.sh.txt + GATE_LOGIC_TEST_mock_server.py.txt: a local MOCK server (python stdlib; websockets and websocat are absent on this host)
exercises the script's verdict logic on declared scenarios (good -> exit 3 only because the audience leg is uncovered; wrong close code -> FAIL;
never-close -> FAIL; refused ws -> INCONCLUSIVE) plus helper cases. Output: GATE_LOGIC_TEST_output.txt (the Python tracebacks in it are the mock
complaining about the handshake-only probe dropping its connection; harmless noise). It proves the script decides correctly given inputs and says
nothing about the real server. Static checks: `bash -n` clean; shellcheck reports only SC2317 info notes.
