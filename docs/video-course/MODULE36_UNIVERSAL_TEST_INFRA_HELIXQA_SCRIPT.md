# Module 36 — Building Universal Test Infrastructure with HelixQA

**Duration:** 22 minutes
**Prerequisites:** Module 22 (HelixQA Autonomous), Module 29 (Module Architecture)

## Learning objectives

1. Understand the Universal Solution Principle: fix the test infrastructure, never the app under test.
2. Use HelixQA's adapter-per-platform pattern to add a new platform target.
3. Author a test bank with executable actions (not prose descriptions).
4. Validate a QA campaign end-to-end, interpret evidence, and avoid false-positive "passed as stub" results.

## Segment 1 — The Universal Solution Principle (0:00 – 4:00)

**Core rule**: the app under test must require ZERO modifications to be testable.

**Anti-pattern**: adding a `QAInputReceiver` broadcast receiver to the Android app so tests can inject text. This pollutes production code with test-only paths and breaks if the test is run against a real release build.

**Correct pattern**: HelixQA navigates the on-screen keyboard via DPAD and `input keyevent`. No app-side changes.

Why this matters:
- Production builds are what we ship. Testing a different binary gives false confidence.
- Test-only paths are a maintenance tax and a security hole.
- Any solution that modifies the app under test is INVALID and must be reimplemented.

## Segment 2 — The adapter-per-platform pattern (4:00 – 10:00)

HelixQA uses `digital.vasic.challenges/pkg/userflow/` which defines 8 adapter interfaces:

- `BrowserAdapter` — Playwright CLI, Selenium, Cypress, Puppeteer
- `MobileAdapter` — ADB CLI, Appium, Maestro, Espresso
- `DesktopAdapter` — Tauri WebDriver
- `APIAdapter` — HTTP via pkg/httpclient
- `GRPCAdapter` — grpcurl CLI
- `WebSocketFlowAdapter` — gorilla/websocket
- `BuildAdapter` — Gradle, Cargo, npm, Robolectric
- `RecorderAdapter` — Panoptic CDP, ADB screenrecord

Each test is written against the adapter interface, not a specific implementation. Swapping Playwright for Cypress is a one-line change in the platform group config.

**Show on screen:** `Challenges/pkg/userflow/adapters/adb_cli.go` — the ADB adapter implementing `MobileAdapter.Tap`, `Type`, `DPAD`, etc.

## Segment 3 — LLM-vision-driven navigation (10:00 – 14:00)

Every interaction follows: **screenshot → LLM analysis → action decision**.

```
[Autonomous pipeline]
  screenshot (ADB pull)
    ↓
  LLM vision provider (llama.cpp RPC → navigation preset)
    ↓
  Structured JSON action: { "type": "dpad", "direction": "down", "count": 3 }
    ↓
  ADB executor performs the action
    ↓
  verifyOutcome (UI dump + next screenshot)
    ↓
  if screen changed → continue; if stuck → report STAGNATION
```

**Non-negotiable rules**:
- Never write hardcoded tap coordinates.
- Never use sleep timers (use UI state polling).
- If the vision provider fails, RETRY — never substitute a hardcoded action.
- Every phase has an "Evidence gate" — UI dump MUST NOT contain "Sign In" after login.

## Segment 4 — Executable test banks (14:00 – 18:00)

**Anti-pattern** (prose description):
```json
{
  "action": "Enter admin/<ADMIN_PASSWORD> credentials",
  "expected": "Home screen loads"
}
```

**Correct** (executable actions):
```yaml
- name: Open keyboard
  action: "adb_shell: input keyevent DPAD_CENTER"
  expected: "keyboard visible in UI dump"
- name: Type username
  action: "adb_shell: input text admin"
  expected: "username field populated"
- name: Tab to password field
  action: "adb_shell: input keyevent KEYCODE_TAB"
  expected: "password field focused"
- name: Type password
  action: "adb_shell: input text <ADMIN_PASSWORD>"
  expected: "password field populated"
- name: Submit
  action: "adb_shell: input keyevent KEYCODE_ENTER"
  expected: "screen transitions away from login"
```

Each action is directly executable — no ambiguity, no prose interpretation.

## Segment 5 — Honest skip vs false-pass (18:00 – 22:00)

**Anti-pattern**: if the WebSocket endpoint isn't reachable, the challenge returns:
```go
return c.CreateResult(challenge.StatusPassed, ..., "WebSocket not available; passes as stub"), nil
```

This is FRAUD — the challenge reports PASSED when it did no actual work.

**Correct**: return `StatusSkipped` with a structured reason:
```go
return c.CreateResult(
    challenge.StatusSkipped, start, assertions, nil, outputs,
    "websocket endpoint not reachable",
), nil
```

The downstream QA report honestly shows the test as skipped, the operator sees it, and the gap is visible. False-positive passes erode trust in the whole campaign.

## Exercise

1. Pick an unused ADB key code (e.g., `KEYCODE_MENU`).
2. Add a test bank entry that uses it as an executable action.
3. Run HelixQA autonomous against a local Android TV emulator.
4. Verify the video recording shows the menu opening.

## Assessment

1. What does the Evidence gate after login assert? Answer: UI dump must NOT contain "Sign In" text.
2. Why is adding `android:exported="true"` to a test-only receiver a policy violation? Answer: it pollutes the production manifest with test-only paths that could be exploited.
3. What should a QA session do if the vision provider is down? Answer: SKIP the phase with a structured reason — never substitute hardcoded steps.
