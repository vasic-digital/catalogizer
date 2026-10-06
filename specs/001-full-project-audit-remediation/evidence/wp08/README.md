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
