# CAT-TAURI-NATIVE-1: native Tauri QA evidence (2026-11-06)

Companion of [../tauri-boot-20261105/README.md](../tauri-boot-20261105/README.md). This folder carries the TEXT evidence
(driver scripts, per-run `checks.json` / `run_log.json`, secret-scan result) of the native desktop and installer-wizard QA runs
that drove the real Tauri binaries inside the rootless QA container (Xvfb + screenshot + OCR, see-before-type per the constitution
anti-blind-typing rule). Screenshots, OCR text files and the screen recording are NOT committed (`docs/qa/**/*.png` and `*.mp4`
are gitignored); they stay in the local evidence archive of the QA host (a `tauri-native-20261106/` directory under the operator's evidence archive; not part of the repository).

## What each folder holds

| Folder | What it is | Result (from its `checks.json`) |
|---|---|---|
| `run1`, `run2`, `run3` | desktop scenario on the rebuilt binary: first-run server-URL dialog, cancel, confirm, login, change-URL dialog, rejected file / credentials URLs | all 12 checks true in each of the 3 runs |
| `close_x` | window close-X scenario | all 4 checks true |
| `dev_run_checks_C6_C8_C9_expects_wrong` | deliberate deviation run: the checks C6, C8, C9 expect a wrong value | C6, C8, C9 false by design (a checker that cannot go false would be worthless) |
| `mutant_R5` | reviewer mutation R5 against the scenario (`run_log.json` only) | see the log; no `checks.json` was produced |
| `wz_red_run1`, `wz_red_run2`, `wz_red_run3` | installer-wizard RED runs against the pre-fix wizard (before the dialog capability fix) | runs 1 and 2: `W2_next_to_protocol_step2`, `W6_next_to_config_mgmt_step4`, `W14_summary_visible` false; run 3: all 10 true, so the RED was reproduced in 2 of 3 runs, not deterministically |
| `scripts/` | the drivers (`qa.py` helpers, `scenario.py`, `wz_scenario.py`, `runall.sh`, `runwz.sh`, ...) | reference drivers; locations come from environment variables |

## Honest boundary

- GREEN live run of the rebuilt installer wizard (same Save Configuration sequence, same target) is still owed (TAURI-R1-FU-7); the
  wizard item stays PROVISIONAL-DONE until then. The guard `scripts/tauri/test_wizard_capabilities.sh` proves only that the capability
  files grant the dialog permissions the frontend imports; it does not prove the native dialog opens.
- The wizard RED was not deterministic (2 of 3), see the table; it must not be cited as a stable RED.
- Passwords and tokens: the QA stack secrets are read from a 0600 env file by the scripts and never logged. The OCR of password
  fields is masked. Secret scan: `secret_scan.txt` (4 secret names, each with a control hit and 0 hits in the evidence) was repeated on
  the files committed here on 2026-11-10: 0 hits for each of the 4 values in this folder.

## Rerun

`scripts/runall.sh <RUNNO>` and `scripts/runwz.sh` start a fresh QA container through `scripts/tauri/run-qa.sh` and run the scenarios;
they take their locations from environment variables: `QA_SCRATCH` (dir holding `repo/`, `out/`, `evid/`), `QA_ENV_FILE` (0600 QA env file, outside the repository, read inside a subshell only), `QA_BIND` (host address for noVNC; `127.0.0.1` plus an SSH tunnel is the safe default; the LAN address changes, so it is never committed) and optionally `QA_DRIVERS` (default: the scripts directory).

Note: secret keystrokes are logged without their length (`qa.py` `typ()`); the committed run logs were scrubbed of `chars` on secret entries.
