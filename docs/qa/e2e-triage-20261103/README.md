# E2E triage 2026-11-03 (constitution 11.4.289)

Mechanical REAL / MOCKED / MIXED classification of the browser-driven web tests.

- Tool: `scripts/test-infra/classify_web_tests.py --root <repo> --out-json classification.json --out-md classification.md`
- Test (RED first, golden-good/bad + control needles + independent test counter): `python3 -I scripts/test-infra/test_classify_web_tests.py`
- Output: `classification.json`, `classification.md` (table, domain map, per-file make-live plan).

Rules: a test is MOCKED when a double is in its path (mock* helpers from fixtures/api-mocks.ts and fixtures/auth.ts, page/context.route, route.fulfill, msw, init-script fetch/Tauri overrides, or a file-level helper doing one of these, also via beforeEach). Mocked is unit-class and cannot be cited as live evidence. REAL means only "no double found"; it is necessary, not sufficient (a live run with 11.4.289(D) machine fields is still required).
Honest limits: static analysis; loop-parametrized tests count once; `test.use`, fixtures that wrap routes in other files, and dynamic names are not followed beyond one level of same-file helpers.
