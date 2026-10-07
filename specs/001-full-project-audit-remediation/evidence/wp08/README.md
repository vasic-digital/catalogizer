# WP-08 host-side evidence (T086, T087)

Identity: catalogizer / 001 / WP-08 | rev 3 (WF3 fix round 4 appended at the end; the rev 2 text above is history) | 2026-10-06 | draft, UNREVIEWED (the independent re-review of this fix round is owed)

Rev 1 (earlier today) was reviewed by WF2 (`WF2-REVIEW-wp06-wp08.md`, NO-GO, I-5, I-6, minors m-8, m-9). Rev 2 is fix round 3.

## Status of T086 and T087: NOT DONE (acceptance is unmet, recorded exactly)

T086 (the failing test) and the mechanism of T087 exist and pass on the host. The task acceptance of T087 is NOT met. The criteria that remain open:

| # | T087 acceptance text | State |
|---|---|---|
| A1 | the ledger and the ratchet live under `scripts/gates/` | NOT MET. They live under `scripts/ledger/` by the caller's scope. A tasks.md amendment is OWED (task paths T086, T087 and every reference to `scripts/gates/project_gate_ledger*`); no tasks.md edit was made here. |
| A2 | every gate name is "implemented or a deferral with a tracked item" | NOT MET. 1 of 13 rows is IMPLEMENTED (`CM-PROJECT-GATE-LEDGER-RATCHET`). The other 12 are DEFERRED to the interim reference `PENDING-REGISTER-ITEM(after T069)`, because the register (T069) is not live and no item id exists. Since rev 2 the ratchet refuses such a row unless `--allow-pending` is given and then reports `PASS-INTERIM pending_untracked=N`, never `PASS` (WF2 I-6). Minting the items is T092 (OWED). |
| A3 | "the ratchet baselines of T040, T105 and T210 are ledger rows too" | NOT MET. There are no rows for them; those ratchets do not exist yet (OWED with T040, T105, T210). |
| A4 | the extraction command and count are saved | MET and regenerated: `gate-names.txt` (rev 2 command, count 13 names; the ratchet's own wider extraction also sees the family reference, warn only). One name was added since rev 1: `CM-ATM-TICKET-IDS-COMPLETE`, named by docs/04 line 1221 (a canon gate; the ledger row is DEFERRED, baseline 12). |
| A5 | the T086 RED is captured with `RUNP IMG-KCOV` | NOT MET. RUNP and IMG-KCOV (T007) do not exist; every run is host-side (bash, git 3rd party tools of the host). The container image digest is UNCONFIRMED. |
| A6 | T086 GREEN x3 | MET for the host run: `T086-GREENx3-rev2.txt` (3 runs, 43 checks each). |
| A7 | mutation `CM-PROJECT-GATE-LEDGER-RATCHET` (a name with no implementation and no deferral) observed failing | MET: `no_F1` and 25 further mutants, `mutations-rev2.tsv` (machine-written, 26 caught of 26). |

## What changed in rev 2 (WF2 findings)

| Finding | Change | Test and mutant |
|---|---|---|
| I-5a carrier site accepted | IMPLEMENTED needs a code file (`sh bash py go js ts rb pl`), the token on a non-comment line, not one of the ratchet's own input files, no `..` | 5 fixtures; mutants `I5a_*` |
| I-5b baseline parse | exactly one non-comment line holding one integer; `tr -dc 0-9` is gone | 5 fixtures; mutants `I5b_*` |
| I-5c rename/delete | the previous names are the working file UNION the version at `HEAD`; the result line names `prev_source=working+HEAD` or `working-only`. The files are UNTRACKED today, so the guard reads `working-only`: it becomes active at the first commit (honest, printed). | fixture on a real temporary git repository; mutants `I5c_*` |
| I-5d no ratchet-down | the DEFERRED count must EQUAL the baseline: above and below both FAIL. The rev 1 assertion "count below baseline passes" is REPLACED by "count below baseline FAILs; a lowered baseline passes" (a deliberate rule change, not a weakening). | `count below baseline FAILs`, `count equal ... passes`; mutant `I5d_no_ratchet_down` |
| I-5e hyphenated prose | a gate name directly followed by a hyphen and lowercase prose (`NAME-style`) is the name without the suffix | fixture and `I5e_*` |
| I-5f tracked item | a DEFERRED reference must be an item id `^[A-Z][A-Z0-9]*-[0-9]+$` or the interim placeholder with `--allow-pending` | 5 fixtures; mutants `I5f_*` |
| I-6 | this status table; real run with and without the flag | `real-run-rev2.txt` |
| LM1, LM2, LM3 survivors | empty or one-character removal reason, trailing-dash ledger row, digit-bearing names now have fixtures | mutants `LM1`, `LM1b`, `LM2`, `LM3` caught |
| m-8 identity | every rev 2 transcript carries utc, host, git head, the command and the sha256 of the script and test | see the files |
| m-9 | see LM1 to LM3 | |
| extra | duplicate ledger names refused (F1c); an option without a value exits 2 | fixtures and mutants |

## Files

`T086-RED-rev2.txt` (the rev 2 test run against the rev 1 ratchet: 22 failing checks), `T086-GREENx3-rev2.txt`, `mutations-rev2.tsv` (+ `.summary`), `gate-names.txt`, `real-run-rev2.txt` (the real ledger run without and with `--allow-pending`). The rev 1 files (`T086-RED.txt`, `T086-GREENx3.txt`, `mutations.txt`, `real-run.txt`, rev 1 identity-less) are kept as history and are superseded; `real-run.txt` shows a plain `PASS` that rev 2 would not give.

## Owed (not done here)

1. tasks.md: amend T086/T087 paths (`scripts/gates/` to `scripts/ledger/`) or relocate; add the A2 and A3 follow-ups. No tasks.md edit was made (instruction).
2. T092: mint the register items for the 12 deferrals, replace `PENDING-REGISTER-ITEM(...)` and drop `--allow-pending`; the baseline then drops to the tracked-item count only as gates are implemented.
3. Ledger rows for the T040, T105 and T210 ratchet baselines.
4. A companion document under `docs/scripts/` for `project_gate_ledger_ratchet.sh` (WF2 m-7; `docs/scripts/` is outside this fix round's scope).
5. UNCONFIRMED: the ratchet's `IMPLEMENTED` rule proves a code file carries the token on a code line, not that the gate works; a functional proof per implemented gate stays owed with each gate.


## Round 4: WF3 fix round (ratchet rev 3), UNREVIEWED, the independent re-review is owed

Status of T087 acceptance is unchanged: still NOT DONE (A1 path, A2 12 interim deferrals, A3 T040/T105/T210 rows, A5 IMG-KCOV, see the table above).

Identity (sha256 prefixes; every file below carries the full values in its header): `project_gate_ledger_ratchet.sh fb354ebc...` (rev 3), `test_project_gate_ledger.sh 18dc33ca...`, `test_project_gate_ledger_mutations.sh c3243c08...`.

| Finding | Change | RED / GREEN / mutants |
|---|---|---|
| **WF3-I3 route 1** a name never listed in prev-names vanished uncited | F2b: every non-family name found in the documents must be registered in `--prev-names` (working file UNION `HEAD`). A new gate name therefore enters prev-names in the same change and from then on is protected by F2 | fixtures I3-1, I3-1b, I3-1c, I3-1d, I3-1e (the reviewer's A1-A3 sequence in a real git repo); mutant `I3_F2b_unregistered_name_ok` |
| **route 2** missing `--prev-names` file disabled F2 | an absent file FAILs (`F2 prev-names file absent`) | fixture I3-2; mutant `I3_prev_names_absent_ok` |
| **route 3** orphan DEFERRED rows kept freed slack | F1d: a ledger row whose name occurs in no document FAILs (every row is then also reference-validated by F1) | fixtures I3-3, I3-3b, I3-3c; mutant `I3_F1d_orphan_row_ok` |
| m4 W6 / W7 | fixtures W6 (`CAT-12-later`), W7 (`//` comment in a `.go` site), W7b (golden-false) | mutants `W6_item_id_regex_unanchored_at_end`, `W7_slash_comment_counts_as_code` |

Rule changes (not weakenings): the fixture LM3 golden-false now also registers its new name in prev-names (F2b); the rev 2 fixtures otherwise unchanged. `T086-RED-rev3.txt`: 4 failing checks against the ratchet reconstructed from the rev 2 text (the other new fixtures W6/W7 pass on the shipped ratchet by design, they are mutation-RED). `T086-GREENx3-rev3.txt`: 55 checks x3. `mutations-rev3.tsv`: 32 mutants, 32 caught (rev 2: 26). `real-run-rev3.txt`: the real ledger run is RED today on the five names `CM-ATM-TICKET-IDS-MONOTONIC`, `CM-ATM-TICKET-IDS-UNIQUE`, `CM-TICKET-IDS-COMPLETE`, `CM-TICKET-IDS-MONOTONIC`, `CM-TICKET-IDS-UNIQUE` (named in the documents by another stream; no ledger row, now also not in prev-names). They are owed by that stream or by a policy for quoted upstream names; this round did NOT add rows for them (a row changes the baseline, an owner decision). The new rules add no other failure to the real run.

Owed after round 4: tasks.md paths (A1), T092 (mint the items, drop `--allow-pending`), rows for the five names above, `docs/scripts/` companion for the ratchet (m5), the carrier residual of IMPLEMENTED (m6). UNCONFIRMED: F2b needs the tracked prev-names file committed to be protected against a full local rewrite; until it is committed the guard is `working-only` (the result line says so).

## WF7 fix round 7 (2026-10-06): ledger ratchet token-boundary match

* WF7-1 (important, fixed): `scripts/ledger/project_gate_ledger_ratchet.sh` decided IMPLEMENTED with a substring match (`grep -qF`), so a gate named `ALPHA` read as implemented from a file that carried only `ALPHA-ONE` or an unrelated `ALPHAX_HELPER` (fail-open). The site check now matches the name as a whole token: the characters before and after must not be identifier characters (letters, digits, underscore, hyphen). RED first: `RED-wf7-1-ledger-substring.txt` (fixtures WF7-P1, P1b, P1c, P1d, P1e FAIL on the shipped ratchet). Golden-false fixtures WF7-P1g (the exact token after `echo`, in quotes, in a case pattern, in a call, before a comma, alone on a line) pass. GREEN: `GREEN-wf7-ledger-runs1-3.txt` (72/0 x3).
* WF7-3 (fixed): fixtures WF7-LR1 (F2 exact-name), WF7-LR2 (F1d case-sensitive), WF7-LR3 (F2b case-sensitive) added; mutants LR1, LR2, LR3 and P1, P1b, P1c, P1d, P1e added to `test_project_gate_ledger_mutations.sh`. `ledger-mutations-wf7.tsv`: 42 mutants, 42 caught, 0 survived. (The pre-existing mutant `I5a_comment_line_counts` needed its target text updated to the new match command; that is not a weakening: it still removes the comment filter and is caught.)
* WF7-2: see the wp06 README round 7 section; real run `real-run-wf7.txt` shows only the five known unledgered names, baseline untouched.
* Owed: no baseline, prev-names or ledger row was added for the five known names (owner decision). The docs scope of the ratchet (`--docs` covers the whole spec tree including evidence READMEs) is unchanged; whether evidence should be excluded is an owner decision.
* This README's older identity line (round 4 test hash, "ledger files untracked") is stale (WF7-5): the identity headers of the round 7 transcripts above are authoritative; the ledger files are committed in HEAD.

## WF11 fix round (long-op registry and anti-mess sweep), UNREVIEWED, the independent re-review is owed

Review `WF11-REVIEW-longops-antimess` (NO-GO: F1-F4 HIGH, F5-F10 F15 F16 IMPORTANT, F11 F12 MEDIUM, minors) remediated under 11.4.276 (defect classes named, every member closed; see `docs/scripts/longops.md` and `anti_mess_sweep.md` "WF11 fix round"). Evidence, all produced in this session: `WF11-RED-test_registry.txt` (new tests on the OLD scripts: pass=165 fail=63), `WF11-RED-test_sweep.txt` (pass=112 fail=43), `WF11-GREEN-test_registry-run1..3.txt` (228/0 x3), `WF11-GREEN-test_sweep-run1..3.txt` (155/0 x3), `WF11-GREEN-test_mutation_safety-run1..3.txt` (65/0 x3), `WF11-mutations-registry.txt` (M01-M42 caught, M27 and M37 re-run), `WF11-mutations-sweep.txt` (S01-S32 caught, S07 and S13 re-run), `WF11-containment-negative-control.txt` (the containment test run against a shim whose process-group check is neutered: it FAILS, 6 checks). Reviewer mutants RM1-RM4 are M22-M25 and RMS1 is S17; all caught.
Owed / not done: `docs/scripts/README.md` index rows (another stream's modified file, F19); the label VALUE `catalogizer.op_id=dispatch-<id>` that `scripts/build/dispatch.sh` passes (out of scope, the sweep matches the op id run_pinned.sh labels); container leg of every run is host-side (UNCONFIRMED).

## WF14 round 3 (STRUCTURAL, constitution 11.4.276 E), UNREVIEWED, the independent re-review is owed

Review `WF14-REVIEW-longops-antimess-r2` (NO-GO: R2-1, R2-2, R2-3, R2-11 important source-defects; R2-T1, R2-T2, R2-T4 important test-instrumentation; minors R2-4..R2-10, R2-T3, R2-T5, R2-T6, R2-D1..D3). This is round 3 of a 5-7 budget.

**Convergence assessment (round 3, recorded before the fixes).** Two or more findings share one named class and three of the four important source-defects were introduced BY the round-1 fixes, so this round was STRUCTURAL: the ground truth was re-derived from the real producers before any edit (`scripts/build/dispatch.sh`: the pump's TERM trap runs `release.sh --state handoff` under the same purpose lock, `reg_adopt` registers `<id>-aN` and never marks the old record, container label `catalogizer.op_id=dispatch-<id>`; `scripts/containers/runner_lib.sh`: `podman stop --time STOP_GRACE_S` (10 s) before the EXIT trap releases the op; `scripts/containers/run_pinned.sh`: label `catalogizer.op_id=<op id>`), the classes were enumerated with control-needled instruments (below), and the class was fixed, not the instance. The shared cause of round 2: each round-1 fix and its test used a simplified model of the real composition path (a heartbeat-then-release test that never ran the release inside the reap's critical section; a handoff test that edited a field no producer writes; a rebind test that never looked at the claim holder; a label test that assumed op id == label).

Class inventories (needle and post-fix outputs preserved as `WF14-inv-classA.txt`, `WF14-inv-classB.txt`, `WF14-inv-classC.txt`; the class D inventory is the test list below; a count is a lead, the lines were read):
* A, a lock held across waiting or a slow external call. Instrument: awk over the bodies of every function that runs under `lo_with_lock` (`_rc _reap_op _reap_a _reap_b _hb _rel _s _u _a _e lo_claim lo_unclaim`), tokens `sleep|PODMAN|podman|timeout|ls-remote|wait|flock`, comment-only lines and the definition line excluded. NEEDLE: the pre-fix `reap.sh` (git HEAD) gives 3 hits (`podman ps`, `podman stop`, the `sleep 0.2` grace loop); post-fix: 0 hits. (`lo_cs_pause`, a test hook in `acquire.sh`/`register.sh`, is a function call and is not matched by the tokens: test-only, never set in production.)
* B, owner identity versus claim holder. Instrument: grep of every write form of a pid or start time into an op or holder record. Needle: the pre-fix `heartbeat.sh` shows its single `.pid=` write (op only). Post-fix: the op-owner rebind has ONE site (`heartbeat.sh`) and it writes op and holder together; the other holder writers (`register.sh`, `acquire.sh --adopt`, `lo_holder_json`) write the holder of an op-less or newly claimed purpose.
* C, fields a reader depends on that no producer writes. Instrument: `jq` field reads of `sweep.sh` and `lib.sh` minus the keys `register.sh`/`heartbeat.sh`/`release.sh`/`reap.sh` write. Needle: the PRE-FIX sweep lists `adopted_by`, `attached_to`, `superseded_by`. The instrument is coarse (report fields of the sweep's own JSON appear, and multi-line object keys of `register.sh` and `acquire.sh` are missed), so every hit was classified by hand: the only op-record readers with no production writer are `adopted_by`/`attached_to`/`superseded_by`, now accepted but no longer the only basis of adoption (a later op of the same purpose, which is what `reg_adopt` really writes, adopts it).
* D, valid JSON with one bad field read as a definite fact. Inventory by reading `lo_classify_op`, `_lo_holder_status_of`, `_ops` and `_p1_class`: every field a branch reads now has a validation and a test that fails without it: op `pid` and `start_time` (N11, M54), numeric fields (N11, M43), process holder `pid` (N12, M44) and `start_time` (N13, M57), suspended-run `builds`/`state`/`callback_state` (N13d, M58), sweep `op_id`/`purpose_key`/`state` (BF1-BF5, S33, S34, S39).
* E, containment of mutant side effects: the four bypasses and podman are closed or honestly scoped (`docs/scripts/longops.md`, "Mutant containment"); the layer-1 token set grew (python signal APIs, glob-built absolute paths, containment tampering, podman side effects), layer 2 gained a podman PATH stub and read-only shim functions, every fixture defaults `LONGOPS_PODMAN` to a null stub; `mutate_safety.sh` (MS01-MS08) mutates the containment library itself.
* F, a record written and then reported as a failed CAS: `release.sh --op-id` and `reap.sh --op-id` release only their own claim (`lo_unclaim_own`).
* G, a destructive auto-action on an unproven staleness predicate: `orphan_container` is reported, never stopped (a container whose op is not in THIS registry may belong to another checkout).

Transcripts here: `WF14-RED-*.txt` (the new tests against the round-2 code: registry 257 pass / 32 fail, sweep 162 / 14, mutation_safety 72 / 29; final GREEN registry 305/0, sweep 176/0, mutation_safety 104/0 (three runs each); mutants registry 58/58, sweep 40/40, safety 8/8), `WF14-GREEN-*-run{1,2,3}.txt`, `WF14-mutations-*.txt`. Honest boundaries: (1) R2-9 (a pid recycled between the identity check and the signal) is closed by a start-time re-check immediately before the signal but has no deterministic test (UNCONFIRMED race); (2) the `docs/scripts/README.md` index rows stay owed (the file is another stream's and was not edited; no tracked item was created in this round); (3) the RED transcripts of the registry suite were taken before the holder-identity tests N13 and the W3 rewrite were added (those were adopted as the inventory of class D was finished); (4) host-side run, the container leg is still UNCONFIRMED (T007/T008).
