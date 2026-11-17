# SEC-WS-1 F3 / rc.1 gate G4 (secws1f3, round 4 fixes R3-1..R3-6 on top of R2-1..R2-5) - PROVISIONAL, UNREPRODUCED-as-open-defect

Base HEAD af02b7c35cdfc0a43d08998f01e1d12da8a6fd0b (tree 2a547e796f33078c41e73cb5812a2545ea7abf9d), original branch secws1f3; rounds 2/3/4 are on branches secws1f3b / secws1f3c / secws1f3d (merged with local main). Docs/scripts only: no source change.

## What F3 requires
docs/FOLLOW_UPS_NEXT_PHASE.md L2104 (SEC-WS-1-F3): a connected /ws socket must be re-checked after upgrade (revoked session / suspended user stops
receiving) and BroadcastToClients must filter by audience. Gate G4 (docs/release/V2_5_0_GATE_CHECKLIST.md) records the F3 part as already on main
(merge a9a43b9ce, hermetic-real tests) and the OPEN part as the LIVE /ws run.

## Finding: F3 is already fixed on main - no new source change made
Commits 94936f3d2, 234f8da71, 59201f0e1 (websocket_handler.go authorize/authorizeMessage/recheckLoop, audience-filtered broadcast). No RED was
reproduced: there is no open defect to reproduce (11.4.292 gives no basis for a fix without an observed symptom).

## Evidence (host-fixture class: real DB, real middleware, real HTTP server, gorilla clients; NOT live)
- green.txt + green_run1.txt / green_run2.txt / green_run3.txt (round 4, R3-3): `cd catalog-api && go test -p 1 -count=1 -v -run 'TestF3_|TestWSRoute_|TestSEC_WS_1F3FU8' .`
  run three times (package main), each -v log stored: every run rc=0, 20 `--- PASS` lines, 0 `--- FAIL`. TestSEC_WS_1F3FU8 matches no test in package main:
  it lives in package handlers and is covered by green_handlers.txt (`-run 'TestSEC_WS_1F3FU8|TestWS|TestWebSocket' ./handlers`, earlier round at
  HEAD af02b7c35, NOT re-run this round).
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
- F3 check (round 3, R2-1): a 1008 alone proves nothing (authorize() also closes 1008 on token expiry). PASS now needs ALL of: victim socket and a
  CONTROL socket (admin, never suspended) alive by ping/pong before the suspension; victim JWT exp (decoded, never printed) beyond the window;
  SUSPEND_CMD exit 0 (a failed/absent suspension is INCONCLUSIVE, never FAIL; closed stdin on the manual prompt likewise); the suspension PROVEN
  effective (a new victim login / ticket mint refused with a status in VICTIM_REFUSED_STATUS, default "401 403", real value UNCONFIRMED; any 2xx
  = suspension had no effect = INCONCLUSIVE); close code 1008 after the suspension marker and within F3_BOUND; and the control socket still open
  answering a final ping at the end (a server that closes everything is INCONCLUSIVE). Anything unprovable is INCONCLUSIVE.
- Round 4 (R3-1): after the victim socket closes, the control socket is kept open for a further F3_BOUND seconds (one re-check interval plus margin)
  before its final ping, so "control survived" spans a full re-check cycle (mock scenario control-later: victim 1008 at +1 s, control 1008 at +5 s = INCONCLUSIVE).
- Round 4 (R3-2): an empty / unparseable victim probe result (probe SIGKILLed, OOM) is INCONCLUSIVE (end=noprobe), never "socket still open" FAIL.
- Round 4 (R3-4): a missing / empty BASE, ADMIN_USER, ADMIN_PASS, VICTIM_USER or VICTIM_PASS exits 2 (usage) with a message; an uncreatable or unwritable OUT exits 2 with a message.
- Round 4 (R3-5): the F3 record label shows the observed close code (`close code <n|none>`), not a fixed 1008.
- Round 4 (R3-6): ONE refused leg (new victim login OR ticket mint with a status in VICTIM_REFUSED_STATUS, the other not 2xx) is enough to prove the suspension
  effective. The JWT exp check uses the LOCAL clock: clock skew between this host and the server beyond the 75 s margin is NOT covered.
- Timing: marker = epoch when SUSPEND_CMD returns (or on Enter); elapsed = probe close_epoch (time of receipt in the probe) - marker, same host clock.
  A close in the ms between the command's effect and its return reads as "before suspension" = INCONCLUSIVE, never PASS.
- F3_BOUND must track the deployment's re-check interval (RecheckInterval, default 15 s) plus margin; default 20 s assumes the default interval.
- Probe: validates Sec-WebSocket-Accept (mismatch = no observation), tolerates masked server frames, ticket passed by env (not argv).
- Process identity (11.4.289(D)): recorded when SERVER_PID / SERVER_EXE_SHA256 are supplied, else the run prints `live_proven=NO (UNCONFIRMED)`.

## Gate-logic unit test of the script (NOT live evidence, 11.4.289(C))
GATE_LOGIC_TEST_run.sh.txt + GATE_LOGIC_TEST_mock_server.py.txt: a local MOCK server (python stdlib) exercises the script's verdict logic only. Each
scenario asserts the overall exit code AND the F3 verdict: good (F3 PASS, exit 3 only because the audience leg is uncovered); wrong close code (FAIL);
never closes (FAIL); closes 1008 8 s after suspension with bound 3 (FAIL); refused ws, drop right after 101, close 1008 before the suspension marker,
never answers ping, SUSPEND_CMD=false, close 1008 3 s after connect while the suspension has no effect, control socket closed too, control closes
later than the victim (control-later), short-lived token, victim probe SIGKILLed with no JSON (probe-killed) (all INCONCLUSIVE), plus exit-2 usage /
unwritable-OUT cases. Mutation check (round 4): the round-3 script (HEAD 90e041a69) run against mock mode control-later gives F3 PASS (the defect R3-1); the round-4 script gives INCONCLUSIVE. Full suite re-run after the last edit: 43 UNIT-PASS, 0 failures (mock output = gate-logic only, not live evidence). Plus helper-level f3_verdict cases. Output: GATE_LOGIC_TEST_output.txt. It proves the script decides correctly given inputs and says
nothing about the real server.
Credential argv check (round 4): strace_argv_check.txt (`strace -f -e trace=execve` over a run against the mock with needle passwords; 0 argv hits, 0 output
hits; positive controls: base URL in argv, a deliberate `bash -c 'true <needle>'`; negative control: fabricated needle 0 hits).
Static checks: `bash -n` clean on both; `shellcheck -s bash LIVE_WS_CHECK.sh.txt` reports only SC2317 (info, unreachable-code note on the sourced-only `return`
guard); python parse clean. (Earlier SC2034/SC2015/SC2086/SC1091 findings no longer apply: the script was rewritten; none was suppressed.)
