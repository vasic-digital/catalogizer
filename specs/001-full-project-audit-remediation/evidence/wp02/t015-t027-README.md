# WP-02 run notes (T015, T019, T022-T027), 2026-10-06

Identity: HEAD e7a6a9b9 (working tree dirty by other streams), constitution pin a71b176, host MemTotal 31717548 kB.

| Task | State | Evidence |
|---|---|---|
| T015 | already present (audit/index-tooling-notes.md at pin 10b7a06; scripts unchanged to a71b176) | audit/index-tooling-notes.md |
| T019 | authored + rendered, uncommitted (T029) | t019-scope-guard.txt, t019-scope-guard-report.json, t019-t004-test-rerun.txt, config/index/*, codegraph.json, .lumenignore |
| T022 | BLOCKED-ON-MEMORY, not run | t022-blocked-preflight.txt |
| T023 | already present, sha256 verified (golden.sha256 OK) | audit/golden.json, audit/lumen_golden_60.json |
| T024 | run on an index copy: recall 0.780, FAIL rc=1 | t024-lumen-verify.txt, audit/lumen-verify/, audit/lumen-recall-by-language.md |
| T025 | run twice, byte-identical: verdict FAIL (P1 FAIL, P2 FAIL, P3 FAIL, P4/P5/P6/P8 SKIP, P-Lumen PASS, P7 FAIL) | audit/refresh/index-health-run1.json, run2.json |
| T026 | BLOCKED: candidate preflight SUMMARY fail=3; no .mcp.json written | audit/mcp-preflight.txt, mcp-candidate.json, mcp-challenge.json |
| T027 | recorded as staging rows (register not live) | audit/findings-index.jsonl |
