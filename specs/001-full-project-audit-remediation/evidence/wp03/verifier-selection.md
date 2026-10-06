# Verifier selection (T030, read-only; reuse-first check per 11.4.74, docs/21 IC-17, IC-37)

Decision under test (IC-37, data-model section 9): promote the POC `poc/repo_verify/verify_repo.sh` to `scripts/repo/verify_repos.sh`.
Candidates scored: (P) the POC; (C) `submodules/constitution/scripts/fastcycle/verify/repo_verify.py` (constitution pin 10b7a06, 1464 lines).

## What was run (2026-10-05, host, read-only)

- (P) `bash poc/repo_verify/verify_repo.sh --self-test`: `SELF-TEST: pass=24 fail=0` in 6.2 s (transcript `poc-selftest.txt`). The stored 2026-10-03 live run `poc/repo_verify/results/run1.*` was re-read, not repeated: 98 rows in 45.6 s, summary `{"repos":98,"owned":51,"dirty":2,"dirty_excepted":1,"ahead":0,"diverged":0,"pin_drift":0,"unproven":0,"classes":{"LOCAL-BEHIND":8,"SAME":191},"failing":1}`.
- (C) `python3 repo_verify.py --recursive --root . --out evidence/wp03/constitution-repo-verify.json` (without `--recursive` it exits 2): ran about 12 minutes (12:28 to 12:40 by file times), rc=1, `overall NOT_CLEAN`, summary `{'blind': 0, 'clean': 71, 'not_clean': 27, 'repos': 98, reasons_histogram: DIRTY_TREE 3, REMOTE_AHEAD 24, REMOTE_TIP_DIFFERS 1, REMOTE_UNREACHABLE 1, UNTRACKED_FILES 1}` (transcript `constitution-repo-verify.stderr`, report `constitution-repo-verify.json`). It touched the network (`git ls-remote`, depth-1 scratch fetches) and wrote only the `--out` file; `git status` shows no tracked change beyond the pre-existing `.gitignore`.

## Score against docs/11 section 8.4 assertions

| Assertion | (P) POC | (C) repo_verify.py |
|---|---|---|
| A-1 rows = submodule status count + 1 | yes (98 = 97 + 1) | yes (98) |
| A-2 pin equals recorded gitlink | yes (`pin_state`) | yes (`submodule_pointer_matches_checkout`, `DETACHED_POINTER_DRIFT`) |
| A-3 dirty 0 except listed exceptions | yes (exceptions.tsv, `dirty_excepted`) | NO: no exception list; the one reviewed exception (docling CRLF) is reported `DIRTY_TREE` |
| A-4 no unpushed | yes (`ahead`) | yes (`UNPUSHED_COMMITS`) |
| A-5 no UNREACHABLE/DIVERGED on own-org rows | yes (owned-only comparison) | NO as measured: one `REMOTE_UNREACHABLE` on `submodules/constitution`, counted with 24 third-party rows |
| A-6 no BEHIND_UPSTREAM on own-org rows | yes (owned rows only; LOCAL-BEHIND is not a plain-mode problem) | NO: `REMOTE_AHEAD` on 24 rows, nearly all third-party (`helix_qa/tools/opensource/*`); no own-org awareness, so the docs/11 observed `BEHIND_UPSTREAM=24` is reproduced as NOT_CLEAN |
| A-7 root HEAD = tip on every remote | yes (all 8 root remotes `SAME` in `baseline-verify.json`) | yes (root row lists the remotes) |

## Score against docs/16 section 11.1 checks R1..R6 as revised

| Check | (P) | (C) |
|---|---|---|
| R1 clean tree, exceptions listed | yes | partial (`DIRTY_TREE`, `UNTRACKED_FILES`; no exceptions) |
| R2 ls-remote plus ancestry, never tracking refs | yes for ls-remote and ancestry; its `--fetch` is the plain `git fetch --no-tags <remote> <branch>`, which moves `refs/remotes/<remote>/<branch>` (data-model section 9, the form T032 corrects) | yes: live `ls-remote --symref`, bare-object fetch into a scratch object store, no ref written |
| R3 stash | NO | NO (header: "never stashes"; no `git stash list` check found by grep) |
| R4 pinned commit held by a remote | NO (pointer drift only) | PARTIAL-STRONG: `POINTER_UNFETCHABLE` by a depth-1 scratch fetch, tri-state, but only fetchability, not the IC-37 comparison with the `.gitmodules` branch tip and no own-org filter |
| R5 submodule initialised | yes (pin `-`) | UNCONFIRMED (not located by grep; `SUBMODULE_UNMAPPED` is a different check) |
| R6 history not rewritten | DIVERGED class only | `REMOTE_TIP_DIFFERS` only; reflog part UNCONFIRMED in both (docs/16) |

## Exit-code set 0/11/12/13/14/15/20

Neither tool matches. (P): 0, 1, 2, 3. (C): 0, 1, 2, 3 (needle failure), 4 (unverified). Both need a mapping layer or a rewrite; (P) is a 286-line bash script whose JSON is already the `repo-verification-report/1` shape the contract and consumers (S7, SC-010) expect; (C) emits its own schema (`body_hash`, `reasons_histogram`) and mandates `--recursive`.

## Verdict

The score CONFIRMS the IC-37 promotion of (P): (P) meets A-1..A-7 and R1, R2, R5 as it stands; (C) fails A-3, A-5, A-6 by design (no exceptions, no own-org split, third-party upstream movement read as NOT_CLEAN) and runs about 16 times slower (about 12 min against 45.6 s for the 98 rows), on the network for every repository. Gaps both share and T032 closes in the promoted file: R3 (stash), R4 (the `.gitmodules`-branch comparison), the object-only fetch form, the exit map, and the `no_remote` mode boolean. One idea worth borrowing from (C) is its by-SHA fetchability probe for R4, recorded as a candidate, not adopted (not required by IC-37, and it slows the run). No contradiction of IC-37 found; no silent tool switch.

UNCONFIRMED: (C)'s R5 behaviour; (C)'s behaviour when `--recursive` meets an owned/third-party list (it has none); whether (C)'s 12-minute time is dominated by the 24 third-party remotes (not measured separately).
