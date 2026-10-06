# Catalogizer Constitution

| Field | Value |
|---|---|
| Revision | 7 |
| Created | 2026-10-02 |
| Last modified | 2026-10-06T18:55:00Z |
| Status | active, BINDING; the 2026-10-06 canon pin move (constitution version 2.2.0) is held for its G-PIN review (WP-07 T084) and owner ratification (T081a) |
| Binds | the Catalogizer Spec Kit layer, together with `.specify/memory/constitution-appendix.md` |

This is the Spec Kit governance layer for Catalogizer. It **incorporates** the Helix Universal
Constitution (`submodules/constitution`), the project constitution (`CONSTITUTION.md`) and every
project and module override (`CLAUDE.md`, `AGENTS.md`, `GEMINI.md` at the root and in each module).
It extends them and never weakens them.

**Precedence (highest first):** Helix canon (`submodules/constitution/Constitution.md`) >
this document > root `CONSTITUTION.md` / `CLAUDE.md` / `AGENTS.md` > module `CLAUDE.md` /
`AGENTS.md`. A project or module rule MAY tighten a canon rule and MUST NOT weaken it.

**Completeness model.** The canon is 11,900 lines (about 1.85 MB), so it is incorporated in four
layers rather than copied verbatim, which would breach its own token-efficiency (§11.4.141) and
no-duplicate (§11.4.227) rules:

1. **Binding by reference.** Every clause of the pinned canon binds in full, including each
   anchor's text, its gate names and its "no escape hatch" list. Nothing here narrows that.
2. **Stated here.** The principles, the digest of §1–§12, the project overrides, the technology
   and the known conflicts are written out below.
3. **Enumerated here.** All 283 anchors are listed in the Anchor Catalogue, generated from the
   canon's own machine index, so no anchor can be missed or mis-transcribed.
4. **Ported in the binding appendix.** `.specify/memory/constitution-appendix.md` (about 1 MB)
   states the operative rules of every one of the 283 anchors (every MUST clause, lettered clause,
   gate name, no-escape-hatch flag and honest boundary) and every project and module rule. It is
   part of this constitution and binds with it. It is a separate file only because Spec Kit loads
   the main file on every command; Spec Kit commands MUST read the appendix when a plan, task or
   review touches any anchor.

Where a digest here or in the appendix is shorter than canon, canon governs. Read the anchor block before relying
on it: `grep -n '§11\.4\.<N> —' submodules/constitution/Constitution.md`.

**Pinned sources (written by `scripts/governance/regen_speckit_catalogue.py`):** `submodules/constitution` at commit
`a71b1767b40229c8e88140924352fbe990b3049e`; `Constitution.md` sha256
`95117cf3d96e4bb144d4e4cfed4031392abd91e22d55c95e6eb5aed940238a4e`; `constitution_index.yaml` records sha256
`d915a5c10f46041b5ba8020c685d845a6a3e3c5fca75384a1a807a98eed90c72`, which differs, so the index lags this pin: the catalogue below lists the anchors the index knows. Anchors present in `Constitution.md` but absent from the index: 11.4.276.

## Core Principles

### I. Anti-Bluff End-User Usability (NON-NEGOTIABLE)

Green tests on a feature that does not work for the end user are a critical defect. The bar for
shipping is that users can use the feature, not that tests pass (§7.1, §8, §11.4).

- Every PASS MUST carry positive evidence captured during execution in the current session:
  a value match, a state delta, a captured frame or audio analysis. Metadata-only, config-only,
  absence-of-error and grep-without-runtime results MUST NOT be reported as PASS.
- Every runtime test MUST perform a real user-visible action, capture state before and after,
  and embed a unique per-run evidence token where it touches mutable state (§7.1).
- A FAIL caused by a faulty test or script is equally a bluff and MUST be fixed (§11.4.1).
- "Verified", "working", "fixed", "complete" and "passing" MUST NOT be used without evidence from
  this session. Unknowns MUST be stated as `UNCONFIRMED:`, `UNKNOWN:` or `PENDING_FORENSICS:`,
  never guessed (§11.4.6).
- Every gate and guard MUST assert the real condition: a false-positive refusal is a FAIL-bluff
  and a false-negative pass is a PASS-bluff (§11.4.201). A null measurement is not evidence until
  a control needle proves the instrument can see (§11.4.201, §11.4.273).
- Every user-interface interaction MUST be seen and understood by OCR, vision or screenshot proof
  before and after; blind typing is forbidden (§11.4.193).
- Tests, Challenges and HelixQA sessions are bound equally.

*Rationale:* This is the project's prime directive. The project reached a state where all tests
and Challenges passed while most features were unusable. Every gate exists to make that
mechanically impossible.

### II. Test-First on Real Systems (NON-NEGOTIABLE)

All executable work (features, wiring, gates, Challenges, bash scripts) MUST start with a test
observed to fail for the right reason before the implementation exists (§11.4.224, §11.4.43,
§11.4.115). A defect fix MUST first reproduce the defect on the broken artifact, using the
reproduction's exact sequence (§11.4.146, §11.4.199).

- A change needs four layers of coverage: a source gate, a post-build artifact gate, a runtime
  test of the user-visible behaviour, and a paired mutation proving the gate can fail (§1, §1.1).
  A gate whose mutation does not turn PASS into FAIL is a sham and MUST be rewritten.
- Mocks, stubs, fakes and placeholders are permitted ONLY in unit tests. Integration, E2E,
  automation, security, DDoS, scaling, chaos, stress, performance, benchmarking, UI, UX,
  Challenge and HelixQA tests MUST exercise the real system (§11.4.27).
- Every supported test type MUST cover the whole codebase, with HelixQA and Challenges fully
  incorporated (§11.4.27, §11.4.169, §11.4.25).
- Code coverage MUST be at least 85% with a target near 100%, bash included. Coverage is necessary
  and never sufficient (§11.4.224). The root rule requires 100% across all ten test types.
- `go vet ./...` MUST pass with zero warnings and no suppressions. At least 60% of assertions
  MUST verify observable behaviour. Mutation score MUST be at least 85%, enforced by
  `mutation_ratchet_challenge.sh`.
- Every fixed defect MUST gain a permanent regression guard (§11.4.135).
- Tests MUST leave the target quiescent on every exit path (§11.4.14), run deterministically
  (§11.4.50), and name their oracle before they are written (§11.4.245).

*Rationale:* A test written after the code only proves it agrees with the code. Mock-only
integration tests are the mechanism by which the prime-directive failure occurred.

### III. Evidence-Gated Independent Review

No change is accepted on the author's say-so. Every change, including a one-line documentation
edit, MUST pass an independent review before acceptance, commit or build (§11.4.142, §11.4.125).

- The reviewer MUST be structurally separate from the author and MUST run on the Opus model at
  `xhigh` effort. No Fable and no lower effort for Opus is permitted. Canon §11.4.209 (review) and
  §11.4.211 (merge-conflict resolution), as amended 2026-10-04 (pin `a71b1767`), REQUIRE a Sonnet
  fallback when Opus at `xhigh` is genuinely unavailable (a captured fact, never a guess): the work
  MUST then run on Sonnet at the highest effort the dispatch path can set, with the Opus
  unavailability fact and the substitution recorded in the evidence, and it is blocked only when
  both Opus and Sonnet are unavailable. Leaving it blocked or deferred while Sonnet is reachable is
  the canon no-escape-hatch flag `--leave-review-blocked-while-sonnet-reachable`. The earlier text
  of this principle ("BLOCKED, never substituted") digested the pre-amendment canon; it is not an
  owner mandate: the owner record for ODG-19 reads "Opus xhigh as pinned (§11.4.209)" and OA-2026-10-05-23
  reads "run independent reviews ... with Opus xhigh and record effort", and neither says "no
  fallback" or "blocked". Status: an OPEN canon conflict (Known Conflicts item 17, owner question
  OD-WP07-SONNET-FALLBACK, default: follow canon). Interim practice, an agent's interim choice and
  NOT an owner decision: follow canon, Opus at `xhigh` first and the Sonnet fallback only on a
  captured Opus-unavailability fact, recorded with model and effort. The same pin binds
  merge-conflict resolution (§11.4.209, §11.4.211).
- Review iterates to a zero-finding GO (§11.4.134), considers every scenario and angle, and
  verifies fixes against captured runtime evidence (§11.4.194).
- The producer of a change, artifact or verdict MUST NOT be the actor that gates it. Separation is
  enforced by capability, not by instruction (§11.4.240, §11.4.249).
- Investigation precedes fixes: no fix without a root cause established by systematic debugging,
  activated automatically (§11.4.102). Regressions are located by bisection, not blame (§11.4.242).
- Seemingly dead code MUST NOT be removed without git-history proof (§11.4.124), and a shipped
  component MUST NOT be removed without an explicit operator keep-or-remove decision (§11.4.122).
- The default working model is Sonnet; the lightest capable model and effort are chosen per task
  and escalated when complexity proves higher (§11.4.231). Haiku is excluded here (see Known
  Conflicts, item 3).

*Rationale:* Independence is what makes review evidence rather than agreement.

### IV. Containerized, Rootless, Distributed Builds

Every build of every component MUST run inside a build container provisioned through the
`containers` submodule, never on the bare host (§11.4.173, §11.4.76).

- Containers MUST be rootless Podman. `sudo`, `su`, rootful Docker and any privilege escalation
  are forbidden; use the `Containers` submodule instead (§11.4.161, module rules).
- Builds MUST be distributed to the designated remote build host and the artifacts brought back.
  Build-host targets and container definitions are config-injected.
- One immutable, content-addressed artifact is built once and promoted through every environment;
  it is never rebuilt per stage (§11.4.264). Builds MUST be reproducible and hermetic at SLSA
  Build Level 2 or better (§11.4.246).
- The brought-back artifact MUST still pass full-suite retest, runtime-signature verification on a
  clean target and installable-asset evidence (§11.4.40, §11.4.108, §11.4.38).
- Build and deploy begin the moment source fixes are proven correct; test-side hardening runs in
  parallel and never gates the build. Every deploy increments the version, and every manual-QA
  deploy opens a new development cycle (§11.4.235).

*Rationale:* Reproducibility, host isolation and capacity offload. A missing build capability is
added by extending the containers submodule, not by an ad-hoc host build.

### V. Reuse-First, Decoupled Modules

Catalogizer is composed of independent submodules under `submodules/`. Existing submodules MUST
be searched before any capability is written, and a missing capability MUST be added by extending
the owning submodule upstream (§11.4.74, §11.4.28).

- Submodules MUST stay decoupled and configuration-injected; no project-specific value may be
  hardcoded into them. No nested own-org submodule chains: dependencies are declared in
  `helix-deps.yaml` and laid out at depth one (§11.4.28, §11.4.31).
- Reusable components live in public `vasic-digital` submodules. Developer tooling MUST be
  project-agnostic and MUST NOT be wired into a shared PATH (§11.4.177).
- Near-identical code forks MUST NOT be maintained; extract role-specific values into data
  (§11.4.251). Every proposed dependency carries a verified existence verdict (§11.4.270).
- Names are lowercase snake_case (§11.4.29). Build artifacts are never versioned and `.gitignore`
  is maintained (§11.4.30). Scripts are documented (§11.4.18) and shell scripts parse on the
  target shell (§11.4.67).

*Rationale:* Shared ownership of reusable modules keeps a multi-repo system recoverable and free
of duplicated, drifting implementations.

### VI. Absolute Data, History and Host Safety

Destructive operations are safety-critical and non-negotiable (§9, §11.4.113, §12).

- Force-push, `--force-with-lease`, `+ref` pushes and history rewrites are forbidden on every
  repository and submodule, with or without approval. Integrate by fetching, merging onto the
  latest `main` and pushing fast-forward to every upstream (§11.4.113, §9.2, §2.1). `--no-verify`
  is never the routine path.
- Any other destructive operation MUST first take a hardlinked backup, record metadata, verify
  after the operation, and restore on any check failure (§9.1, §9.3, §9.4).
- Secrets MUST NOT be committed, printed, logged or echoed. `.env` files stay gitignored with mode
  0600. Credentials are handled per §11.4.10 and audited before storage (§11.4.10.A).
- The host MUST NOT be suspended, hibernated, rebooted, powered off or logged out, directly or
  indirectly (§12.1, CONST-033). Project procedures MUST NOT use more than 60% of total RAM, with no override
  (§12.6). Thread and process headroom MUST be checked before scaling parallelism (§12.12).
- A process MUST be verified as ours before it is inspected or signalled (§11.4.174). Signals
  MUST NEVER target a process group at or below 1 (§11.4.263).
- All CI/CD automation (GitHub Actions, GitLab pipelines) MUST be disabled in this repository and
  every owned submodule, existing configurations included; none is added. Enforcement is local
  (§11.4.156).

*Rationale:* Data loss from a wrong force-push is irreversible once a remote garbage-collects, and
a session-killing OOM destroys the operator's work.

### VII. Continuity and Zero-Loss Traceability

Work MUST be resumable from any agent at any moment, and no request or requirement may be lost.

- `docs/CONTINUATION.md` MUST exist and be updated in the same commit as every non-trivial state
  change, with a timestamp and a verbatim resumption prompt (§12.10, §11.4.127, §11.4.131).
- Every operator request MUST be captured, tracked and processed; none may be skipped
  (§11.4.210, §11.4.208). Reports become fully populated workable items synced to the database,
  the documents and every external tracker (§11.4.202, §11.4.213).
- Workable items live in the tracked SQLite single source of truth with status, type and a stable
  ATM-NNN id. A returning defect reopens its item and is never re-minted (§11.4.93, §11.4.95,
  §11.4.15, §11.4.16, §11.4.54, §11.4.214).
- Research and kicked-off work MUST be driven to completion and wiring, or explicitly closed
  with evidence; it is never left in the backlog (§11.4.197).
- The live in-session task tracker MUST always match the real work state (§11.4.229).
- Documentation, exports and the database stay in sync through the docs chain, and the main
  README is the entry point to every document (§11.4.106, §11.4.12, §11.4.65, §11.4.212).

*Rationale:* The most expensive failure after a false PASS is silently losing work or context.

### VIII. Manual-QA-Final and an Honest Definition of Done

Automation is necessary and never the last word (§11.4.185, §8).

- Manual QA-team testing is the mandatory final confirmation of done. Automated QA MUST be the
  discoverer, and anything manual QA finds is a coverage escape (§11.4.185, §11.4.238).
- Every release candidate passes the full-suite retest before tagging (§11.4.40); a QA hand-off
  requires a candidate-fingerprinted PASS verdict (§11.4.236); every advertised capability has a
  passing challenge (§11.4.266).
- Money, safety, availability and integrity code MUST include failure-path scenarios as gates
  (§11.4.239). The project holds zero known shortcomings, gaps, weak spots and danger zones, under
  a monotone-decreasing audit ratchet (§11.4.261, §11.4.260).
- Fixes are verified at four layers with a runtime signature on a clean artifact (§11.4.108,
  §11.4.139). Deferrals are documented with concrete blockers, never "fix next cycle" (§8).

*Rationale:* The operator is the first user-layer oracle most claims ever meet unless the pipeline
guarantees otherwise.

## Inherited Constitution Rules

These are the numbered Helix sections (`submodules/constitution/Constitution.md`). They bind in
full; the digests below state the operative rules.

### §1–§10 Foundations

- **§1 Test coverage is mandatory for every change.** Four invariants: change present in source;
  survives compilation or packaging; behaves correctly at runtime; the gate itself is not
  bluffing (paired mutation). A change without all four is not ready to merge.
- **§1.1 False-positive immunity.** A gate that cannot fail is worse than no gate. Every new gate
  is paired with a mutation that breaks the assertion, re-runs the gate, requires FAIL, and
  restores. This is the single most important rule in the constitution.
- **§2 Single commit/push entrypoint, locked.** Use the project's multi-remote commit wrapper;
  direct `git commit`, `git push` and `git add` on the main repo are prohibited, except tag
  creation (`git tag -a`) and tag push. Wrappers hold an advisory `flock` released by `trap`. Never
  `rm -f` a lock unless the owning process was just killed. See also §11.4.180, §11.4.234.
- **§2.1 Multi-upstream push.** Every commit goes to ALL configured upstream remotes, via
  `install_upstreams.sh` and `Upstreams/*.sh` declarations.
- **§3 Submodule first.** Commit inside the submodule, push it to all its remotes, then commit the
  parent pointer. Skipping step one yields parent commits pointing at missing source.
- **§4 Tags mirrored.** Every main-repo tag is created on every owned submodule at its pointed
  HEAD and pushed to every remote of every owned repo. Third-party submodules are excluded.
- **§5 Changelog.** Each tag ships `docs/changelogs/<tag>.md` plus `.html`, `.json`, `.txt`
  exports and an updated cumulative `docs/changelogs/CHANGELOG.md`.
- **§6 Documentation to nano-detail.** Every change updates the Applied Fixes table in
  CLAUDE.md/AGENTS.md, `docs/guides/`, architecture diagrams and the changelog. Documentation
  drift is a violation.
- **§7 No false success.** Every gate reports PASS, FAIL or SKIP with explicit reason text; SKIP
  is mechanically distinguishable from PASS; FAILs count toward exit status; runtime results are
  cross-referenced against host-side evidence where introspection is insufficient.
- **§7.1 NO BLUFF.** Five constraints on every runtime test: real action, state delta, positive
  evidence, unique evidence token, and captured audio or video evidence for AV features.
- **§8 Quality bar.** Forbidden: source rebrand without artifact rebuild, tests pass but feature
  broken, configuration-only tests, "works on my workstation", and "will fix next cycle".
- **§9 Data safety.** §9.1 seven-step destructive-operation protocol; §9.2 force-push needs
  explicit authorization every time (superseded by the absolute ban in §11.4.113); §9.3 hardlinked
  backups are the standard; §9.4 audit trail for history rewrites.
- **§10 Enforcement.** Violating §1–§9 is non-compliant with no exception for speed. Data-safety
  and host-safety violations are catastrophic and block the release cycle.

### §11.4 End-User Quality Covenant

§11.4 and its anchors §11.4.1 to §11.4.275 are the covenant that a PASS means the feature works for
the end user. Every anchor is listed in the Anchor Catalogue with its group and title; the
operative text is in `Constitution.md` and in `groups/*.md`. The ten rules the index file lists as
binding unconditionally before anything else map to this document as follows: anti-bluff (§11.4,
Principle I); no guessing (§11.4.6, Principle I); never force-push (§11.4.113, Principle VI); host
safety (§12, Principle VI); never silently remove a shipped component (§11.4.122, Principle III);
investigate before fixing (§11.4.102, Principle III); never remove seemingly dead code on sight
(§11.4.124, Principle III); no CI/CD (§11.4.156, Principle VI); independent review of every change
(§11.4.142, Principle III); and no secrets in output (§11.4.10, Principle VI).

### §12 Host-Session Safety

- **§12.1 Forbidden:** suspend, hibernate, hybrid-sleep, reboot or power off (CONST-033), logout or
  terminate-session, unbounded
  memory inside `user@<uid>.service`, rfkill, lid-switch or power-button handlers, and disabling
  logind or session managers.
- **§12.2 Required safeguards:** heavy scripts source the host-safety library, run its pre-flight
  and abort on failure, wrap any process expected to exceed about 4 GiB RSS in a bounded execution
  scope, and cap parallelism to fit memory.
- **§12.3 Container hygiene:** explicit memory limit, `OOMPolicy=stop`, exponential-backoff
  restarts, and clean-slate rebuild after any host crash.
- **§12.6 Memory ceiling:** at most 60% of total RAM, `HOST_SAFETY_MAX_MEM_PCT` defaulting to 60;
  parallelism is `min(nproc, floor(budget_gb / per_job_peak_rss_gb))`; no override flag exists.
- **§12.10 Continuation document:** `docs/CONTINUATION.md` is a sacred invariant (Principle VII).
- **§12.11 Containerized builds:** a build in its own container cgroup may use the maximal safe
  fraction of the host, computed dynamically from measured `nproc`, `MemTotal` and `RLIMIT_NPROC`,
  never a fixed low `-j`. This scopes §12.6 to user-session-resident work and does not weaken it.
- **§12.12 Thread limits:** check `ulimit -u` headroom before scaling parallel agents; treat
  exhaustion as a host-safety event; free threads only with operator authorization; beware
  `pkill -f` and `pgrep -f` matching their own command line.

### Appendices

- **Appendix A:** academic and industrial foundations of mutation testing (basis of §1.1).
- **Appendix B:** recursive inheritance and path independence; locate canon from any depth with
  `find_constitution.sh`.
- **Appendix C:** every constitution commit is pushed to all configured upstreams.

### Constitution Tooling Exposed to This Project

| Surface | Contents (in `submodules/constitution/`) |
|---------|------------------------------------------|
| Groups | 12 topic groups under `groups/` (see the Anchor Catalogue) |
| Index | `constitution_index.yaml`, machine-readable list of all 283 anchors |
| Action prefixes | `actions/registry.yaml`: BACKGROUND, REMINDER, CRITICAL, IMPORTANT, NOTE, BUG, TASK, ISSUE, FEATURE and sub-system shortcuts, in the six grammar forms of §11.4.140 and §11.4.202 |
| Model tiering | `actions/subagent_tiering.yaml`, `scripts/subagent_tier.sh` (see Known Conflicts, item 3) |
| Skills | `action-prefix-system`, `media-validator`, `multitrack`, `reporting-workable-items`, `scheduled-work-queue`, `session-sync`, `skill-catalog`, `workable-item-lifecycle` |
| MCP and plugins | `mcp/media-validator-mcp.json`, `mcp/scheduled-work-mcp.json`; plugins `helix`, `scheduled-work` |
| Script families | `scripts/`: gates, hooks, multitrack, codegraph, lumen, sonarqube, gitleaks, trivy, zap, hawkscan, translation, workable-items, doc_integrity, fastcycle, reporting, token_efficiency, validation, skill_activation and more |
| Pull hook | `scripts/post_update_hook.sh`, run after every constitution pull (§11.4.26, §11.4.32, §11.4.164) |
| Multi-upstream | `install_upstreams.sh`, `Upstreams/` |
| Path discovery | `find_constitution.sh` |

## Project and Module Overrides

This section is a summary. The complete per-module rules (identity, commands, architecture and
ownership, coordination rules, constraints, commit conventions) are in the appendix, Part 2.

### Root (`CONSTITUTION.md`, `CLAUDE.md`, `AGENTS.md`, `GEMINI.md`)

- The prime directive (Principle I) is the foundational requirement; any dispatch, CI configuration
  or review that allows green tests on broken features MUST be rejected.
- `CONSTITUTION.md` (11 lines) adopts the HelixPlay Constitution v2.3.0 in full, §1–§21, with no
  local weakening. The 2026-05-02 amendments, stated in the root `CLAUDE.md` and `AGENTS.md`: no vacuous assertions, constructor-only tests, mock-only
  integration or E2E tests, or permanently skipped tests without containerization plans;
  usability evidence per §6.7; automatic negative-leg fault injection per §1.3, §6.3 and §11.5.7;
  `ValidateAntiBluff` unconditional with every challenge calling `RecordAction()`; the container
  verifier's `execCommand()` executes real commands; `go vet` clean; `scripts/anti-bluff-scan.sh`
  exits non-zero on any violation and uses process substitution rather than pipes for state; at
  least 60% observable-behaviour assertions (§1.2); mutation score of at least 85% (§6.4);
  18 Contract Clauses R-01 to R-18 (§17); eight Architectural Pillars (§18); performance SLAs of
  at most 30 ms on LAN and 50 ms on WAN at p999 (§19); a mandatory Technology Stack (§20); and a
  14-phase roadmap P00 to P13 (§21).
- Critical constraints: no placeholders, dead code or vacuous tests; every service, database,
  build and test runs in a container; reusable components live in public `vasic-digital`
  submodules; 100% coverage across all ten test types with mocks only in unit tests; R-18
  Operational Integrity forbids any command that suspends, hibernates, locks, terminates or
  crashes the host.
- Git topology: `origin` fetches from GitHub and pushes to GitFlic, eight remotes are configured (`origin` pushes to six),
  force-push is absolutely banned (§11.4.113; the root `AGENTS.md` wording that it could be
  authorized has been corrected), and `--no-verify` is forbidden.
- §11.4.173 (containerized and distributed builds) is restated with no escape flag: no
  `--build-on-host`, `--skip-container-build`, `--local-build-ok`, `--no-distributed-build` or
  `--bare-host-build`.
- `GEMINI.md` and `CLAUDE.md` MUST stay in lockstep (§11.4.157).
- The HelixPlay §17–§21 text is external to this repository (see Known Conflicts, item 5).

### `catalog-api` (Go)

- Architecture is Handler, Service, Repository, Database. Top-level packages hold domain logic;
  `internal/` holds infrastructure. No `internal/` package may import top-level domain packages
  except `models/`.
- Always use `database.DB` (never raw `*sql.DB`), `internal/httpclient` (never
  `http.DefaultClient`) and `internal/concurrency` semaphores. Imports of submodules use
  `digital.vasic.*` paths wired by `replace` in `go.mod`.
- Dual dialect SQLite and PostgreSQL. A shipped migration is never edited; add a new version with
  both dialect implementations plus a `.up.sql` and `.sqlite.up.sql` pair, and keep
  `TestRunMigrationsSQLite` green. Use `?` placeholders, `INSERT OR IGNORE` and
  `InsertReturningID()`.
- Config precedence is environment variables, then `.env`, then `config.json`, then defaults.
  SQLite runs in WAL mode; pool defaults MaxOpen 25, MaxIdle 10, MaxLifetime 5 m, MaxIdleTime 3 m.
- Goroutine services call `wg.Add(1)` before `go`, use `sync.Once` cleanup, expose a shutdown
  method, and are closed in tests. Shutdown order is metrics, WebSocket, cache, media entity
  handler, log adapter, middleware registry, then HTTP server.
- Metadata providers degrade gracefully when an API key is missing and never block the pipeline.
  New challenge groups register in `RegisterAll()` and never "pass as stub"; they return
  `StatusSkipped` with a reason.
- Errors are wrapped with `%w`, never discarded (`_ = err`), and typed for control flow. New
  routes need an `httptest` unit test and the existing auth middleware.
- Tests run with `GOMAXPROCS=3 go test -race ./... -p 2 -parallel 2`; package coverage MUST NOT
  drop below baseline. Resource use MUST NOT exceed 30 to 40% of host CPU and RAM.
- Containers build with `podman build --network host`, `GOTOOLCHAIN=local` and fully qualified
  image names. The server writes its port to `.service-port`.
- Zero-warning policy: no console errors, failed network requests or deprecation warnings.
- Every module forbids `sudo`, `su` and root execution and mandates **Zero Unfinished Work**: no
  TODO, FIXME, empty implementation, silent error swallow, fake data or empty catch; when an issue
  is found, fix all instances.

### `catalog-web` (React)

- Provider order is AuthProvider, WebSocketProvider, Router, Pages. Server state uses React Query
  only and client UI state uses Zustand only; auth lives solely in `AuthProvider`.
- All REST calls use `@vasic-digital/catalogizer-api-client` (never raw `fetch` or `axios`) through
  React Query hooks; dev API URLs are relative so the Vite proxy works.
- Forms use React Hook Form with Zod. Components take a `className` merged with `cn()`, export a
  props interface and have Vitest tests; pages register a route, use `ProtectedRoute` when
  authenticated, and add Vitest plus Playwright tests.
- ESLint runs with `--max-warnings 0`; no console errors or failed requests. `postcss.config.js`
  stays CommonJS. Kill any process on port 3000 first. Prefer `getByRole` in tests.

### `catalogizer-desktop` and `installer-wizard` (Tauri 2)

- Config is owned by the Rust backend and accessed only through `get_config` and `update_config`;
  no auth tokens in `localStorage`. `make_http_request` MUST keep SSRF validation against the
  configured server and MUST NOT be bypassed.
- A new IPC command needs a `#[tauri::command]`, registration in `generate_handler!`, a TypeScript
  bridge and tests on both sides. Rust uses `anyhow` or `thiserror` and never `unwrap()` on
  fallible operations (installer-wizard).
- The wizard flow is Welcome, Protocol Selection, Network Scan, protocol config, Configuration
  Management, Summary. A new protocol needs a Rust tester, an IPC command, a config component, a
  type, a `ProtocolSelection` entry, a bridge update and tests. Configs save to
  `~/.catalogizer/config.json`.

### `catalogizer-android` and `catalogizer-androidtv` (Kotlin)

- MVVM with `StateFlow<UiState>`; manual `DependencyContainer` (not Hilt), no singletons outside
  it. A shipped Room migration is never modified; changes add a migration, bump the version and
  add a destructive-fallback test. DAOs return `Flow` or `suspend`, never block the main thread.
- REST goes through the Retrofit interface in `data/remote/`; cleartext traffic is allowed only to
  local networks via `network_security_config.xml`. Exceptions are never swallowed.
- Android TV is D-pad only: every interactive element is focusable with a visible focus indicator,
  no touch-only handlers, 10-foot typography. For ADB-driven QA press `dpad_center` before `type`
  and use `KEYCODE_TAB` between fields, and run `adb reverse tcp:8080 tcp:8080` per device.
  Watch Next and channels are cleaned on logout; deep links use `catalogizer://media/{id}?type=`.
- The phone app targets JDK 21 and the TV app JDK 17, both with the `--add-opens` kapt flags and
  `android.useNewJdkImageTransform=false`. Gradle JVM is limited to `-Xmx4096m -XX:MaxMetaspaceSize=1024m` per `gradle.properties` (the TV guide's
  Kotlin daemon `-Xmx1024m` is documentation only: no such line exists in its `gradle.properties`,
  so it is UNCONFIRMED). Release signing reads `../docker/signing/signing.properties`. Container
  builds need the Android SDK (compileSdk 35 for the phone app) in the builder image.

### `catalogizer-api-client` (TypeScript)

- New endpoints go in the matching service class, are wired in `CatalogizerClient`, and are typed.
  Errors use the `CatalogizerError` hierarchy mapped from HTTP status. `HttpClient` owns token
  injection and refresh; consumers MUST NOT duplicate token logic. Unit tests mock Axios and make
  no real network calls.

### `Build` (shell) and `Website` (VitePress)

- `Build` is a reusable Bash 4+ framework (versioning via `versions.json`, SHA256 change detection,
  orchestration); projects define `BUILD_COMPONENTS`, `BUILD_COMPONENT_PATTERNS` and
  `build_single_component()`.
- `Website` is VitePress 1.x Markdown with relative internal links. It is a website under
  §11.4.190: fully responsive, SEO-complete, OpenDesign-authored and proven with captured evidence.
  Its `ignoreDeadLinks: true` setting tolerates links to unimplemented pages, which is in tension
  with §11.4.261 (see Known Conflicts, item 6).
- Every module uses Conventional Commits and the Co-Authored-By trailer for AI-assisted commits.

## Technology Stack

| Layer | Technology | Purpose |
|-------|-----------|---------|
| Backend API | Go 1.25.7, Gin, module `catalogizer` in `catalog-api/` | REST API, media detection, metadata |
| Transport | `quic-go/http3` with generated self-signed TLS, Brotli, WebSocket, Prometheus `/metrics` | HTTP/3, compression, live updates, telemetry |
| Auth | JWT with role-based access (`internal/auth`) | Authentication and authorization |
| Storage | SQLite with SQLCipher (dev), PostgreSQL (production), dual dialect | Encrypted catalog database |
| Media sources | SMB, FTP, NFS, WebDAV, local filesystem via `UnifiedClient` | Multi-protocol ingestion |
| Metadata | TMDB, OMDB, IMDB, TVDB, MusicBrainz, OpenLibrary, IGDB, Spotify, Steam | External metadata providers |
| Web UI | React 18, TypeScript, Vite 6, Tailwind, React Query, Zustand, React Hook Form, Zod, Recharts, framer-motion, axios | Browser interface (`catalog-web/`) |
| Web tests | Vitest, React Testing Library, Playwright | Unit and E2E |
| Desktop | Tauri 2, Rust 2021, tokio, reqwest, serde; React 18 front end | Desktop client and installer wizard |
| Android | Kotlin, Jetpack Compose (BOM 2024.12.01 phone, 2024.06.00 TV), Material 3, Room 2.6.1, Retrofit 2.9.0, OkHttp 4.12, Media3, Coil, DataStore, Paging 3, WorkManager; phone compileSdk 35, TV compileSdk 34, targetSdk 34, minSdk 26; JDK 21 | Phone and tablet client |
| Android TV | Kotlin, Compose for TV, Leanback 1.0.0, TV Provider, Media3 session; JDK 17 | Big-screen client |
| API client | TypeScript strict, axios ^1.4, ws ^8.13, Vitest | Shared client library |
| Website | VitePress ^1.5 | Documentation and product site |
| Build framework | Bash 4+, Python 3, sha256sum, Git (`Build/`) | Versioned, change-detected builds |
| Containers | Rootless Podman via the `containers` submodule; Compose files at the repository root | Builds, services and test infrastructure |
| QA | `challenges`, `helix_qa`, `vision_engine`, `screen_diff`, `replay_buffer`, `visual_regression`, `training_collector` | Challenges and autonomous QA |
| Security tooling | SonarQube, gitleaks, Trivy, OWASP ZAP, HawkScan (§11.4.184) | Static and dynamic analysis, secrets |
| Code intelligence | CodeGraph and Lumen (§11.4.78 to §11.4.80, §11.4.275) | Structural and semantic index |
| Governance | `submodules/constitution`, `submodules/superspec` | Inherited rules and Spec Kit pipeline |

**Submodules (44 declared in `.gitmodules`):** `websocket_client_ts`, `ui_components_react`,
`challenges`, `assets`, `concurrency`, `config`, `filesystem`, `database`, `auth`, `middleware`,
`rate_limiter`, `observability`, `media`, `watcher`, `event_bus`, `cache`, `security`, `storage`,
`streaming`, `discovery`, `entities`, `media_types_ts`, `catalogizer_api_client_ts`,
`auth_context_react`, `media_browser_react`, `dashboard_analytics_react`, `media_player_react`,
`collection_manager_react`, `containers`, `lazy`, `memory`, `recovery`, `helix_qa`,
`doc_processor`, `llm_orchestrator`, `llm_provider`, `vision_engine`, `screen_diff`,
`replay_buffer`, `visual_regression`, `training_collector`, `constitution`, `helix_memory`,
`superspec`. Pointers are bumped in the same commit as the cascade work (§11.4.26).

## Development Workflow

This project follows **specification-driven development** using the superspec pipeline:

1. **Constitution** (`/speckit-constitution`): Establish and maintain these governance principles
2. **Specification** (`/speckit-specify`): Define feature requirements before any code is written
3. **Brainstorming** (`/speckit-superspec-brainstorm`): Challenge assumptions and discover edge cases
4. **Planning** (`/speckit-plan`): Design technical approach with constitution compliance check
5. **Task Decomposition** (`/speckit-superspec-tasks`): Break down into executable, trackable tasks
6. **Execution** (`/speckit-superspec-execute`): Implement test-first, using subagents where work is parallel
7. **Review** (`/speckit-superspec-review`): Verify implementation against spec and constitution

### Workflow Rules

- No feature code is written before a spec is approved. Defect fixes follow systematic debugging
  and reproduce-first (§11.4.102, §11.4.146) and need no spec; the autonomous loop is the default
  working mode (§11.4.126).
- Every spec goes through at least one brainstorm session.
- Implementation plans MUST pass a constitution compliance check against Principles I to VIII.
- Feature phase checkpoints require explicit human approval; nothing else waits for per-step
  approval (§11.4.101).
- Subagent-driven execution is the default, in parallel where work is independent, bounded by the
  memory ceiling, thread headroom and the single-resource-owner rule (§11.4.20, §11.4.58,
  §11.4.70, §11.4.103, §11.4.183, §11.4.230).
- Commits and pushes go through the project's commit wrapper, onto the latest `main` fast-forward
  only, to every upstream (§2, §2.1, §11.4.113). Pushes run detached after commit
  (§11.4.88). Fetch before edit and before push (§11.4.37, §11.4.71). The tree is quiescent
  before a subagent commits (§11.4.84). Git hooks MUST NOT block the commit/push mechanism
  (§11.4.234(A) to (D)); disconnecting a hook never drops its check, which moves to a named stage
  of the dedicated script (§11.4.234(C)).
- Branches use `feat/<slug>`, `product/<slug>` or `flavor/<slug>`, one canonical name across the
  main repo and every owned submodule; feature branches merge `main` into themselves regularly and
  merge to `main` only after live QA (§11.4.181, §11.4.188, §11.4.195); spec 001 is the one
  exception (Known Conflicts item 15: all work on main branches).
- Release tags are project-prefixed and mirrored on owned submodules (§4, §11.4.151).
- Operator action prefixes (`BUG ::`, `TASK ::`, `ISSUE ::`, `FEATURE ::`, `NOTE ::` and the rest)
  are recognized and create tracked items (§11.4.140, §11.4.202, §11.4.213).
- New rules are classified universal or project-specific on landing (§11.4.17).

## Quality Gates

### Testing Requirements

- [x] **Unit tests**: REQUIRED. At least 85% coverage, target near 100%; mocks allowed here only.
- [x] **Integration tests**: REQUIRED. Real infrastructure booted through the containers submodule.
- [x] **Contract tests**: REQUIRED across `catalog-api`, web, desktop, Android, Android TV and the
  generated clients, with a can-i-deploy gate (§11.4.244).
- [x] **TDD discipline**: REQUIRED. Tasks marked `[TDD]` follow RED-GREEN-REFACTOR with RED observed.
- [x] **Mutation**: REQUIRED. A paired mutation per gate and a mutation score of at least 85%.
- [x] **Stress and chaos**: REQUIRED for concurrency and protocol paths (§11.4.85, §11.4.253).
- [x] **End-to-end, Challenges and HelixQA**: REQUIRED for user-visible behaviour, with captured
  evidence and, for video or audio, validated recordings (§11.4.158 to §11.4.160, §11.4.163).
- [x] **Visual proof**: REQUIRED for every user interface: host-rendered pixels per screen, state and
  theme, validated by golden diff plus an OCR oracle (§11.4.170), using OpenDesign tokens and the
  design rules of §11.4.162 and §11.4.216 to §11.4.223.

### Review Requirements

- [x] **Code review**: REQUIRED. Independent, Opus at `xhigh`, iterated to zero-finding GO.
- [x] **Spec compliance**: REQUIRED. Every acceptance scenario passes with captured evidence.
- [x] **Security review**: REQUIRED for authentication, credentials, encrypted storage, network
  protocols, SSRF surfaces and any new dependency; SonarQube, gitleaks, Trivy and ZAP are
  available locally (§11.4.184).
- [x] **Performance review**: REQUIRED for scanning, indexing and streaming; targets come from the
  spec and are measured (see Known Conflicts, item 5 for the HelixPlay SLA).
- [x] **Documentation**: REQUIRED. Changelog, guides, diagrams, manuals and FAQs updated and
  exported (§5, §6, §11.4.257, §11.4.258).

### Deployment Gates

- [ ] All tests pass, with real captured evidence rather than a summary line
- [ ] All review items resolved to a clean GO
- [ ] Constitution compliance verified, including the post-pull sweep (§11.4.32)
- [ ] Built in a rootless container on the designated build host; artifact verified on a clean target
- [ ] Full-suite retest passed before any release tag
- [ ] Manual QA completed for user-visible work
- [ ] README badge row, production-readiness tracker and claim-vs-reality ledger current
  (§11.4.259, §11.4.260, §11.4.266)
- [ ] No force-push, no rewritten history, no secret in the diff, no increase in the zero-findings ledger (§11.4.261)

## Anchor Catalogue

All 283 anchors of the pinned canon, grouped as the canon groups them. Each line is the anchor id
and its title (long titles are shortened with an ellipsis). The operative rules of each anchor are
in `.specify/memory/constitution-appendix.md`, Part 1, in the same order. Anchors
numbered 62, 64, 175 and 203 to 206 do not exist in canon and are cited-but-undefined elsewhere.

### Anti-bluff and evidence (33 anchors)

Canon: `submodules/constitution/groups/anti-bluff-and-evidence.md`

- **§7.1** — NO BLUFF — positive-evidence-only validation
- **§11.4** — End-user quality guarantee — forensic anchor (User mandate, 2026-04-28)
- **§11.4.1** — FAIL-bluffs are equally forbidden
- **§11.4.2** — Recorded-evidence requirement
- **§11.4.3** — Per-environment-topology test dispatch
- **§11.4.4** — Test-interrupt-on-discovery + retest-from-clean-baseline
- **§11.4.5** — Captured-evidence quality analysis
- **§11.4.6** — No-guessing mandate
- **§11.4.7** — Demotion-evidence rule
- **§11.4.13** — Out-of-band sink-side captured-evidence
- **§11.4.38** — Installable-Asset Evidence Mandate (User mandate, 2026-05-17)
- **§11.4.68** — Positive sink-side / downstream evidence mandate (User mandate, 2026-05-20)
- **§11.4.69** — Universal Sink-Side Positive-Evidence Taxonomy + Mechanical Enforcement (User mandate, 2026-05-20)
- **§11.4.83** — docs/qa/ end-user evidence mandate (User mandate, 2026-05-22)
- **§11.4.105** — Natural-language intent recognition & clarification (User mandate, 2026-05-31)
- **§11.4.107** — Anti-bluff AV/test-validation techniques mandate (User-driven research, 2026-06-02)
- **§11.4.108** — Four-layer fix-verification + runtime-signature-as-definition-of-done mandate (systematic-debugging Phase 4.5, 2026-06-03)
- **§11.4.110** — Pre-build build-readiness verdict + change-impact clash detection mandate (operator mandate, 2026-06-03)
- **§11.4.123** — Rock-solid-proof-or-deep-research mandate (User mandate, 2026-06-03)
- **§11.4.139** — Fresh-process clean-artifact runtime-signature mandate (User mandate, 2026-06-08)
- **§11.4.146** — Reproduce-first test + same-test-confirms-fix + mandatory extend-to-all-cases workflow (User mandate, 2026-06-10)
- **§11.4.158** — Intensive all-feature/flow/edge-case video-recording + read-the-screen content-verification mandate (User mandate, 2026-06-16)
- **§11.4.159** — Mandatory window-specific video recording + vision validation mandate (User mandate, 2026-06-20)
- **§11.4.160** — Vision-verified recording + HelixQA bridge mandate (User mandate, 2026-06-21)
- **§11.4.163** — Universal Media Validation & Verification Mandate (User mandate, 2026-06-21)
- **§11.4.193** — Anti-blind-typing mandate: every UI interaction MUST be "seen" and "understood" via OCR/vision/screenshot proof, NEVER blind-typed (User mandate, 2026-07-13).
- **§11.4.201** — Every guard/gate MUST assert the REAL condition: a false-positive refusal is a FAIL-bluff, a false-negative pass is a PASS-bluff (research-derived, 2026-07-15)
- **§11.4.226** — Evidence-class-at-closure + standing detection pressure: machinery presence does NOT predict whether a fix holds — the EVIDENCE CLASS at closure (runtime vs source) under real DETECTION PRESSURE …
- **§11.4.262** — Machine-created evidence at every gate: every claim of "works" / "passes" / "verified" cites a captured, machine-derived, machine-verifiable evidence artifact produced by the gate — no operator …
- **§11.4.268** — Tamper-evident evidence chain + periodic anchor record: deletion, reordering, and tail-truncation of an accepted evidence record are DETECTED, not merely content-mutation (spec-derived, speckit …
- **§11.4.269** — Critic/consensus advisory-only ban AT THE EVIDENCE-ACCEPTANCE SEAM: an ungoverned confidence/consensus signal MAY inform but MUST NEVER substitute for a receipt or adjudicate producer-verifier …
- **§11.4.270** — Dependency-existence-verdict register: every proposed dependency carries a closed-set existence verdict {VERIFIED, AMBIGUOUS, UNVERIFIED} with citable evidence; adoption is gated behind resolution …
- **§11.4.271** — Waiver mechanism: the strict FORMALIZATION of allow-with-tracked-debt (unifying §11.4.234(D)'s recorded-deferral, §11.4.236(4)'s explicit-operator-override, and §11.4.248(A)'s deadline-bound …

### Code review and quality (11 anchors)

Canon: `submodules/constitution/groups/code-review-and-quality.md`

- **§11.4.124** — Dead/unwired-code investigate-before-remove mandate (User mandate, 2026-06-04)
- **§11.4.125** — Code-review-agent gate before pre-build + main build (mandatory multi-layer review) (User mandate, 2026-06-04)
- **§11.4.134** — Code-review iterate-until-GO + rock-solid-evidence mandate (User mandate, 2026-06-08)
- **§11.4.142** — Universal code-review mandate — every change reviewed, always, no exception (User mandate, 2026-06-09)
- **§11.4.145** — Independent multi-angle impact-research per change (User mandate, 2026-06-10)
- **§11.4.165** — Universal Independent Verification Agent Mandate (User mandate, 2026-06-21)
- **§11.4.194** — Exhaustive all-scenario, all-angle code-review + verify-against-captured-runtime-evidence mandate (User mandate, 2026-07-14)
- **§11.4.209** — Code-review MUST run on the Opus model at xhigh effort, ALWAYS — no Fable, no escalation tier, no fallback model (User mandate, 2026-09-26, supersedes 2026-09-15 and 2026-07-15)
- **§11.4.240** — Producer ≠ Verifier scope separation: the actor that produces a change/artifact/verdict cannot be the one that gates it — least privilege over cleverness (research-derived, 2026-08-15)
- **§11.4.241** — Illegal-state-unrepresentability preference: types before API-shape before lint before property-test before runtime-assertion (research-derived, 2026-08-15)
- **§11.4.251** — Byte-identical-fork prohibition + role-as-data-pack (research-derived, 2026-08-15)

### Design system and UI (11 anchors)

Canon: `submodules/constitution/groups/design-system-and-ui.md`

- **§11.4.162** — OpenDesign UI design system mandate (User mandate, 2026-06-21)
- **§11.4.170** — Device-independent host-side rendered-UI visual-proof mandate (User mandate, 2026-06-25).
- **§11.4.190** — Website engineering-quality mandate: every project website MUST be fully responsive + completely SEO-optimized + uniquely OpenDesign-authored + bleeding-edge enterprise-quality, each PROVEN with …
- **§11.4.216** — Canonical machine-readable design-token source: one CSS custom-property file (`:root` + `[data-theme="dark"]`), generated bindings everywhere else (User mandate, 2026-07-22)
- **§11.4.217** — OpenDesign brand contract: every UI-shipping product carries a 9-section DESIGN.md + tokens.css twin with disjoint color roles, AA-pinned accents, and a locked attribution footer (User mandate, …
- **§11.4.218** — Living design-library catalogue: every component × state × theme rendered self-contained + a cross-platform reusability matrix with per-cell provenance (User mandate, 2026-07-22)
- **§11.4.219** — Per-app screen catalogues: full-IA coverage per surface, per-platform chrome variants, honesty markers for unbuilt behavior (User mandate, 2026-07-22)
- **§11.4.220** — Open-first design tooling: self-hosted open platforms are primary; proprietary tools are import/export targets only; design sources live in-repo in open formats (User mandate, 2026-07-22)
- **§11.4.221** — Motion discipline: token-bound durations/easings + machine-readable manifest + reduced-motion static fallbacks everywhere (User mandate, 2026-07-22)
- **§11.4.222** — Design-completion export wave: a design is not done until token codegen, the honest-output raster matrix, both-theme renders, hand-off bundles, and doc twins land (User mandate, 2026-07-22)
- **§11.4.223** — Provenance-marker discipline for design documentation + central open-item registry in the area index (User mandate, 2026-07-22)

### Documentation and export (27 anchors)

Canon: `submodules/constitution/groups/documentation-and-export.md`

- **§11.4.12** — Auto-generated docs sync mandate
- **§11.4.18** — Script documentation mandate (User mandate, 2026-05-14)
- **§11.4.19** — Fixed-document column-alignment mandate (User mandate, 2026-05-14)
- **§11.4.22** — Document-sync commit discipline (User mandate, 2026-05-14)
- **§11.4.23** — Visual-cue & grouping mandate for Issues docs (User mandate, 2026-05-14)
- **§11.4.44** — Document Revision Header Mandate (User mandate, 2026-05-18)
- **§11.4.45** — Integration-Status-Doc Maintenance Mandate (User mandate, 2026-05-18)
- **§11.4.53** — Fixed_Summary parity mandate (User mandate, 2026-05-18)
- **§11.4.56** — Status_Summary parity + two-audience format (User mandate, 2026-05-19)
- **§11.4.57** — README.md doc-link section + revision metadata (User mandate, 2026-05-19)
- **§11.4.59** — README always-sync mandate (User mandate, 2026-05-19)
- **§11.4.60** — Documentation always-sync composite covenant (User mandate, 2026-05-19)
- **§11.4.61** — Mandatory Markdown metadata table + structured-doc ToC (User mandate, 2026-05-19)
- **§11.4.63** — Workable-items procedure docs as single source of truth (User mandate, 2026-05-19)
- **§11.4.65** — Universal Markdown export mandate (User mandate, 2026-05-19)
- **§11.4.73** — Main-specification document versioning + revision discipline (User mandate, 2026-05-20)
- **§11.4.86** — Roster/corpus-backed Status-doc auto-sync mandate (User mandate, 2026-05-25)
- **§11.4.99** — Latest-Source Documentation Cross-Reference Mandate — instructions, guides, and manuals MUST be verified against the latest official online sources BEFORE publication (User mandate, 2026-05-28)
- **§11.4.106** — Docs Chain — mechanical documentation/DB sync engine (Operator mandate, 2026-05-31)
- **§11.4.153** — Comprehensive per-feature Status + Status_Summary document set with mandatory video-recording confirmation (User mandate, 2026-06-15)
- **§11.4.168** — Exported-document independent content + textual + full-visual validation mandate (User mandate, 2026-06-23)
- **§11.4.186** — Anti-divergence enforcement: cross-document consistency is a mandatory-before-export/commit gate, never an after-the-fact audit (research-derived, 2026-07-08).
- **§11.4.212** — Main README is the canonical starting-point / entry point for ALL project documentation — no doc may be an orphan unreachable from README (User mandate, 2026-07-16)
- **§11.4.215** — A doc that BINDS work MUST live, tracked, in the repository where that work happens (research-derived, 2026-07-17)
- **§11.4.257** — Comprehensive user-manual + guide + FAQ coverage: every component / service / feature / user-visible workflow ships an operator-usable manual, task-oriented guide, and FAQ, always in sync, no orphan …
- **§11.4.258** — Architectural + data-flow + state-machine + sequence-diagram coverage: every project ships accurate, exported, machine-derivable-or-honestly-authored diagrams embedded into the docs that describe the …
- **§11.4.259** — README quality-status badge row: at the top of every project README, a comprehensive row of quality badges with a closed green→amber→red vocabulary, each backed by machine-derived source data, …

### Git and data safety (23 anchors)

Canon: `submodules/constitution/groups/git-and-data-safety.md`

- **§9.1** — Mandatory safety protocol for destructive operations
- **§9.2** — Force-push requires explicit user authorization every time
- **§9.3** — Hardlinked backup is the standard — there is no excuse
- **§9.4** — Commit-message audit trail for history rewrites
- **§11.4.10** — Credentials-handling mandate
- **§11.4.10.A** — Pre-store credential leak audit (User mandate, 2026-05-17)
- **§11.4.30** — .gitignore + No-Versioned-Build-Artifacts Mandate (User mandate, 2026-05-15)
- **§11.4.36** — Mandatory install_upstreams on clone/add Mandate (User mandate, 2026-05-15)
- **§11.4.37** — Fetch-before-edit mandate (User mandate, 2026-05-15)
- **§11.4.41** — Pre-Force-Push Merge-First Mandate (User mandate, 2026-05-17)
- **§11.4.71** — Pre-Push Fetch + Investigate + Integrate Mandate (User mandate, 2026-05-20)
- **§11.4.84** — Working-tree quiescence rule for subagent commits (User mandate, 2026-05-22)
- **§11.4.88** — Background-push mandate: commit-lock release immediately after commit, push runs detached (User mandate, 2026-05-26)
- **§11.4.113** — Absolute no-force-push + merge-onto-latest-main mandate (User mandate, 2026-06-03)
- **§11.4.121** — No-commit-while-build-writes-tracked-artifacts mandate (1.1.8-dev remediation, 2026-06-03)
- **§11.4.179** — Corruption-isolated parallel git streams (own-.git independent clones, not shared-common-dir worktrees) (User mandate, 2026-07-04).
- **§11.4.180** — Commit/push (single-writer) wrappers MUST auto-reap provably-stale locks before acquiring (User mandate, 2026-07-04).
- **§11.4.181** — Consistent controlled feature-branch naming: one feature/logic-group ⇒ exactly ONE canonical branch name across the main repo + all owned submodules (User mandate, 2026-07-05).
- **§11.4.188** — Regular main→feature merge cadence: every track / feature-branch MUST FREQUENTLY merge canonical `main` into itself DURING the work, never only at the end (User mandate, 2026-07-09).
- **§11.4.195** — Branch-structure governance: taxonomy + merge-after-live-QA + flavor/product non-merge (User mandate, 2026-07-14)
- **§11.4.234** — Dedicated hook-validation script: hook checks run as an explicit stage of a dedicated commit/push script, and the commit/push mechanism is ALWAYS unblocked (operator mandate, 2026-07-26)
- **§11.4.252** — Fail-closed-on-dangerous-combination for mutating / credential surfaces (research-derived, 2026-08-15)
- **§11.4.253** — Idempotency under retry + DB-level durable uniqueness guard (research-derived, 2026-08-15)

### Governance and constitution meta (37 anchors)

Canon: `submodules/constitution/groups/governance-and-constitution-meta.md`

- **§11.4.11** — File-layout discipline
- **§11.4.17** — Universal-vs-project classification of new rules (User mandate, 2026-05-14)
- **§11.4.26** — Constitution-Submodule Update Workflow Mandate (User mandate, 2026-05-15)
- **§11.4.28** — Submodules-As-Equal-Codebase + Decoupling + Dependency-Layout Mandate (User mandate, 2026-05-15)
- **§11.4.29** — Lowercase-Snake_Case-Naming Mandate (User mandate, 2026-05-15)
- **§11.4.31** — Submodule-Dependency-Manifest Mandate (User mandate, 2026-05-15)
- **§11.4.32** — Post-Constitution-Pull Validation Mandate (User mandate, 2026-05-15)
- **§11.4.35** — Canonical-root inheritance clarity (User mandate, 2026-05-15)
- **§11.4.74** — Submodule-catalogue-first discovery + extend-don't-reimplement (User mandate, 2026-05-20)
- **§11.4.75** — Mechanical Enforcement Without Exception (User mandate, 2026-05-20)
- **§11.4.76** — Containers-submodule mandate (User mandate, 2026-05-20)
- **§11.4.77** — Regeneration-mechanism-required mandate (User mandate, 2026-05-20)
- **§11.4.78** — CodeGraph code-intelligence mandate (User mandate, 2026-05-20)
- **§11.4.79** — Own-org submodules MUST be included in the CodeGraph index (User mandate, 2026-05-21)
- **§11.4.80** — CodeGraph regular-update + sync automation mandate (User mandate, 2026-05-21)
- **§11.4.100** — RETIRED.
- **§11.4.109** — Mandatory Anti-Forgetting Enforcement: PreToolUse Guard Hook + Subagent Constitutional Preamble + Orchestrator Pre-Action Checklist (Operator mandate)
- **§11.4.140** — Universal action-prefix system (`ACTION_NAME ::`) (User mandate, 2026-06-09)
- **§11.4.141** — Token-efficiency mandate (research-derived + operator mandate, 2026-06-09)
- **§11.4.156** — All CI/CD automation (GitHub Actions / GitLab pipelines / equivalents) MUST be disabled (User mandate, 2026-06-15)
- **§11.4.157** — GEMINI.md maintained in lockstep with CLAUDE.md / AGENTS.md / QWEN.md (User mandate, 2026-06-15)
- **§11.4.161** — Rootless container runtime mandate (User mandate, 2026-06-21)
- **§11.4.164** — Universal Constitution Auto-Propagation & Hook System (User mandate, 2026-06-21)
- **§11.4.166** — REPEALED (operator decision, 2026-06-22).
- **§11.4.173** — Containerized + distributed build mandate (User mandate, 2026-06-29).
- **§11.4.177** — Developer-tooling project-decoupling + invocation-directory operation (User mandate, 2026-07-04).
- **§11.4.184** — Mandatory SonarQube static-analysis CLI + local tooling installed and PATH-discoverable (User mandate, 2026-07-06).
- **§11.4.184(I)** — HawkScan (DAST) + OWASP ZAP + gitleaks + Trivy: mandatory local security-tooling extension (operator mandate, 2026-09-18).
- **§11.4.196** — Native-alias-first priority + per-alias real-signal limit/subscription tracking + auto-rebind-on-recovery + resource-detection-by-real-identity-not-substring-match (User mandate, 2026-07-14)
- **§11.4.197** — Research / kicked-off-work completion mandate: every research effort MUST be driven to full, wired, verified completion or explicitly closed — never left un-wired in the backlog (User mandate, …
- **§11.4.198** — Default working mechanisms: multi-alias native-first orchestration AND heavy token-optimization are ALWAYS-ON defaults for single- and multi-track work (User mandate, 2026-07-15)
- **§11.4.227** — Governance-corpus self-custody: the rule corpus is bound by the same custody it imposes — every named gate is implemented-or-registered-deferral under a monotone ratchet, and propagation gates count …
- **§11.4.228** — Cross-agent extension lifecycle mandate: per-platform compatibility declaration, source tracking, per-extension documentation with compatibility matrix, and auto-wiring of constitution extensions …
- **§11.4.272** — Dynamic, on-demand skill/extension activation: the active capability surface is the MINIMUM needed now, everything else stays DISCOVERABLE and one command away (User mandate, 2026-09-07)
- **§11.4.273** — Measuring-instrument verification: a census that INFORMS a decision must be control-needled before its result is believed (research-derived, 2026-09-08)
- **§11.4.274** — Mechanical work belongs in a script, not in an agent's context: extract it, test it, and re-scan for it on a cadence (User mandate, 2026-09-08)
- **§11.4.275** — Semantic (Lumen) index + indexing-efficiency gate + universal agent/subagent accessibility: every indexed space reaches every agent and subagent, is proven COMPLETE before it is called ready, and …

### Host and resource safety (21 anchors)

Canon: `submodules/constitution/groups/host-and-resource-safety.md`

- **§11.4.24** — Build-resource stats tracking mandate (User mandate, 2026-05-14)
- **§11.4.58** — Parallel-development methodology (User mandate, 2026-05-19)
- **§11.4.96** — Safe-parallel-work-with-long-build catalogue + mandate (User mandate, 2026-05-27)
- **§11.4.111** — Resolve-by-stable-name-not-by-enumeration-index mandate (research-derived, 2026-06-03)
- **§11.4.119** — Single-resource-owner partitioning for parallel hardware testing mandate (1.1.8-dev remediation, 2026-06-03)
- **§11.4.128** — Always-on device-recording mandate (User mandate, 2026-06-06)
- **§11.4.144** — Tracked/recorded-device availability-following mandate (User mandate, 2026-06-10)
- **§11.4.147** — Crashed-agent respawn-until-complete + no-work-loss registry mandate (User mandate, 2026-06-10)
- **§11.4.154** — Window-scoped capture + fresh-corpus rotation for feature/QA recordings (User mandate, 2026-06-15)
- **§11.4.155** — Project-name-prefixed feature/QA recording filenames (User mandate, 2026-06-15)
- **§11.4.174** — Shared-host process-ownership verification mandate (User mandate, 2026-06-29).
- **§11.4.225** — Scheduler-quota burst-throttling telemetry + interactive-scope isolation from bursty fleets: progressive interactive degradation is diagnosed from throttling accounting over time, never from …
- **§11.4.254** — Boot-time invariant assertion + capability matrix (research-derived, 2026-08-15)
- **§11.4.263** — Process-group signal-safety mandate: NEVER signal pgid ≤ 1, NEVER trust a mock-derived pid/pgid — validate as int > 1 before every `killpg` / `kill(-pid, sig)` / `pkill` / `killall` call (BOB-126 …
- **§12.1** — Forbidden operations — directly OR indirectly
- **§12.2** — Required safeguards
- **§12.3** — Container hygiene
- **§12.6** — Memory-Budget Ceiling — 60% MAXIMUM
- **§12.10** — Continuation document — sacred invariant
- **§12.11** — Maximal dynamic resource utilization for containerized builds (User mandate, 2026-07-03)
- **§12.12** — Process/thread-limit (RLIMIT_NPROC) awareness for parallel subagent/multi-process work (User mandate, 2026-07-07)

### Multi-track and parallelism (13 anchors)

Canon: `submodules/constitution/groups/multi-track-and-parallelism.md`

- **§11.4.103** — Continuous parallel-stream working routine (User mandate, 2026-05-29)
- **§11.4.167** — Big-work-item feature work-stream lifecycle (own copy-on-write project copy + own branch/tags + per-feature builds, no-merge-until-approved, trunk-merged-into-every-stream) (User mandate, 2026-06-23)
- **§11.4.176** — Conflict-free multi-track work-division + exactly-once claim registry + capability-aware deadlock-proof device-lock (User mandate, 2026-07-02).
- **§11.4.178** — Track-qualified identity for parallel work streams (session-name collision ban) (User mandate, 2026-07-04).
- **§11.4.182** — Track+branch work-stream identity label `(T<N>/<branch>)` on every agent/subagent/work-stream label and every operator-facing work-stream reference (User mandate, 2026-07-05).
- **§11.4.183** — Maximal multi-agent utilization per work-stream + full-constitution-application + zero-bluff mandate (User mandate, 2026-07-07).
- **§11.4.187** — Automatic multi-track ruler orchestration (User mandate, 2026-07-04; landed 2026-07-09).
- **§11.4.191** — Work-to-track/branch binding enforcement: a logic-group's work can NEVER be committed OR dispatched onto the wrong track/branch (research-derived, 2026-07-10).
- **§11.4.192** — Continuous multi-track auto-backfill: a FREE track MUST be IMMEDIATELY re-assigned its next-highest-priority domain work-set, never left idle while its domain has actionable items (User mandate, …
- **§11.4.230** — Parallelized-pipeline methodology: build↔validate overlap + affected-stages-only re-run + parallel multi-device testing/recording + fan-out-subagents-always (research-derived, 2026-07-26)
- **§11.4.231** — Nano-precision model-tier selection: lightest-capable-model-first with dynamic mid-task escalation (User mandate, 2026-07-26, `CRITICAL`)
- **§11.4.232** — Anti-mess long-op orchestration: every long-op is registered, single-owned-per-purpose, liveness-PROVEN, handed-off-on-stop, safely-reaped, consistency-checked, and escape-bounded — the single source …
- **§11.4.233** — Standing anti-mess control plane: a level-triggered, idempotent, invariant-catalogue reconciliation loop over the WHOLE persistent System state, orchestrating every per-domain enforcer, refusing …

### Project lifecycle and release (46 anchors)

Canon: `submodules/constitution/groups/project-lifecycle-and-release.md`

- **§11.4.8** — Deep-web-research-before-implementation
- **§11.4.9** — Batch-source-fixes-before-rebuild
- **§11.4.20** — Subagent-driven-by-default mandate (User mandate, 2026-05-14)
- **§11.4.40** — Full-suite retest before release tag mandate (User mandate, 2026-05-17)
- **§11.4.42** — Iteration-discipline mandate (User mandate, 2026-05-18)
- **§11.4.46** — Validate-recent-work-before-post-flash-tests mandate (User mandate, 2026-05-18)
- **§11.4.47** — Firebase Data Review Mandate (User mandate, 2026-05-18)
- **§11.4.52** — Autonomous-Validation Mandate (User mandate, 2026-05-18)
- **§11.4.66** — Blocker-resolution interactive-clarification mandate (User mandate, 2026-05-19)
- **§11.4.70** — Subagent-Driven Execution Is The Default (User mandate, 2026-05-20)
- **§11.4.72** — Audio Top-Priority Mandate (User mandate, 2026-05-20)
- **§11.4.82** — Iteration-speedup discipline mandate (User mandate, 2026-05-22)
- **§11.4.87** — Endless-loop autonomous work + zero-idle agent dispatch + anti-bluff testing mandate (User mandate, 2026-05-26)
- **§11.4.89** — Background test execution mandate (User mandate, 2026-05-27)
- **§11.4.94** — Zero-idle priority-first parallel-by-default operating mode (User mandate, 2026-05-27)
- **§11.4.97** — Maximum-use-of-idle-time mandate + progress-update cadence (User mandate, 2026-05-27)
- **§11.4.101** — Autonomous-decision-over-blocking mandate (User mandate, 2026-05-28)
- **§11.4.102** — Mandatory systematic-debugging activation + always-loaded skill-discovery + plugin-dependency availability (User mandate, 2026-05-29)
- **§11.4.122** — No-silent-removal-of-existing-components-without-operator-confirmation mandate (User mandate, 2026-06-03)
- **§11.4.126** — Default autonomous-loop working mode from first prompt (User mandate, 2026-06-04)
- **§11.4.127** — Session-handoff resumption-prompt mandate (User mandate, 2026-06-06)
- **§11.4.129** — Huge-blocker release protocol (User mandate, 2026-06-06)
- **§11.4.130** — Post-remediation validate-the-fix-FIRST-after-redeploy (User mandate, 2026-06-06)
- **§11.4.131** — Standing session-resumption file mandate (User mandate, 2026-06-07)
- **§11.4.132** — Risk-ordered validation priority mandate (User mandate, 2026-06-07)
- **§11.4.133** — Target-System + hardware safety mandate (User mandate, 2026-06-08)
- **§11.4.150** — Mandatory deep multi-angle web research per change/issue, before declaring fixed or structural (User mandate, 2026-06-11)
- **§11.4.151** — Project-prefixed release-tag/version-naming mandate (User mandate, 2026-06-12)
- **§11.4.152** — Crashlytics-recorded-data continuous monitoring + systematic-debug + regression-test-coverage mandate (User mandate, 2026-06-13)
- **§11.4.172** — Mandatory production-readiness planning with realistic timeline projection (User mandate, 2026-06-29).
- **§11.4.185** — Manual QA-team testing as the mandatory FINAL confirmation of done (User mandate, 2026-07-07).
- **§11.4.200** — Non-targetable deploy/flash tooling MUST isolate exactly ONE eligible target and VERIFY-AFTER-WRITE on the INTENDED target (research-derived, 2026-07-15)
- **§11.4.207** — Instant multi-stream resume engine: whole-fleet continuation state is a durable, content-addressed, atomically-committed snapshot resumed in O(changed) reads, not an O(history) re-read (User mandate, …
- **§11.4.208** — Operator-request-history document: every project maintains a project-local, always-in-sync ledger of every operator request/prompt (content + timestamp+timezone + track + alias + model + effort) …
- **§11.4.210** — Zero-loss request/prompt intake: every operator request MUST be mechanically captured, tracked, and processed — never skipped, ignored, avoided, or lost (User mandate, 2026-07-15)
- **§11.4.211** — Merge-conflict resolution during main→feature/product/flavor merges MUST run on the Opus model at xhigh effort, ALWAYS — no Fable, no fallback model (User mandate, 2026-07-16; amended 2026-09-26)
- **§11.4.213** — FEATURE research-scheduling directive: recognized via all §11.4.140 forms, SCHEDULES (never synchronously executes) a deep, enterprise-grade research + implementation-planning effort as a tracked …
- **§11.4.229** — Live in-session task/todo tracker MUST always be up to date and fully in sync with the real work state — never stale, never showing completed work as pending nor pending as done (User mandate, …
- **§11.4.235** — Build-and-deploy the moment source fixes are proven-correct (test-side hardening is a parallel stage, never a build gate) + post-deploy version increment (operator mandate, 2026-07-27)
- **§11.4.236** — QA-deploy-readiness gate: no manual-QA hand-off until the mandated validation produced a candidate-fingerprinted PASS verdict; a blocker MUST bind to a seam, never prose (research-derived, 2026-07-31)
- **§11.4.260** — Cutting-edge enterprise quality + production-deployment readiness: every work product is built for production deployment, maximal stability, zero nasty surprises, and enterprise-grade robustness — …
- **§11.4.261** — Zero-shortcomings / zero-gaps / zero-weak-spots / zero-danger-zones invariant with a mechanical audit ratchet: at all times the project holds exactly zero of these; every finding is closed OR …
- **§11.4.264** — Build once, promote ONE immutable content-addressed artifact through every environment; never rebuild per stage (research-derived, 2026-08-20)
- **§11.4.265** — Progressive delivery: traffic-shifted rollout gated by automated analysis on SLO **and business** metrics, with automatic abort to stable (research-derived, 2026-08-20)
- **§11.4.266** — Claim-vs-reality ledger: every ADVERTISED capability is a row, typed from the closed bluff-type vocabulary, and a capability with no passing challenge is a release blocker (research-derived, …
- **§11.4.267** — Shared attempt record + converge-on-evidence / escalate-on-stall: a failed approach is never silently retried, and a non-converging loop is itself a signal (research-derived, 2026-08-20)

### Testing and TDD (39 anchors)

Canon: `submodules/constitution/groups/testing-and-tdd.md`

- **§11.4.14** — Test playback cleanup mandate
- **§11.4.25** — Full-Automation-Coverage Mandate (User mandate, 2026-05-15)
- **§11.4.27** — No-Fakes-Beyond-Unit-Tests + 100%-Test-Type-Coverage Mandate (User mandate, 2026-05-15)
- **§11.4.39** — Per-Feature On-Device End-User Validation Mandate (iter-76, 2026-05-17)
- **§11.4.43** — TDD-Fix-Discipline Mandate (User mandate, 2026-05-18)
- **§11.4.48** — UI-Driven Video Testing Mandate (User mandate, 2026-05-18)
- **§11.4.49** — Dual-Approach Testing Mandate (User mandate, 2026-05-18)
- **§11.4.50** — Deterministic Consistency Mandate (User mandate, 2026-05-18)
- **§11.4.51** — Live-ADB-First Maximization Mandate (User mandate, 2026-05-18)
- **§11.4.67** — Shell-script target-shell-parseability mandate (User mandate, 2026-05-19)
- **§11.4.81** — Cross-platform-parity mandate (User mandate, 2026-05-21)
- **§11.4.85** — Stress + Chaos Test Mandate (User mandate, 2026-05-24)
- **§11.4.98** — Full-Automation Anti-Bluff Mandate — Live tests MUST be re-runnable end-to-end without manual intervention (User mandate, 2026-05-28)
- **§11.4.114** — Last-known-good-tag regression isolation mandate (1.1.8-dev remediation, 2026-06-03)
- **§11.4.115** — RED-baseline-on-the-broken-artifact + polarity-switch mandate (1.1.8-dev remediation, 2026-06-03)
- **§11.4.116** — Real-time conductor↔autonomous-test-framework sync channel mandate (1.1.8-dev remediation, 2026-06-03)
- **§11.4.117** — Computer-vision / OCR pixel-oracle fallback for non-introspectable UIs mandate (1.1.8-dev remediation, 2026-06-03)
- **§11.4.118** — Discovery-pressure to confirm known-issue-set completeness mandate (1.1.8-dev remediation, 2026-06-03)
- **§11.4.120** — Fix-breaks-its-own-gate reconciliation mandate (1.1.8-dev remediation, 2026-06-03)
- **§11.4.135** — Standing regression-guard suite + every-fixed-defect-gets-a-permanent-regression-test (User mandate, 2026-06-08)
- **§11.4.136** — Real-content end-to-end playback-test mandate (User mandate, 2026-06-08)
- **§11.4.137** — Subtitle/caption content-correctness oracle + secure-display-proxy-honesty mandate (User mandate, 2026-06-08)
- **§11.4.138** — Operator-escape => mandatory bluff-audit + permanent guard (User mandate, 2026-06-08)
- **§11.4.143** — Real-user-journey mandate for video-streaming-app full-automation tests (User mandate, 2026-06-10)
- **§11.4.169** — Mandatory comprehensive test-type coverage with anti-bluff captured evidence (User mandate, 2026-06-25)
- **§11.4.189** — Most-reopened cases get extra-depth live-testing scrutiny FIRST (User mandate, 2026-07-10).
- **§11.4.199** — Exact-reproduction-sequence mandate: when a working reproduction exists, the investigation MUST use ITS exact sequence — a deviating repro proves NOTHING (research-derived, 2026-07-15)
- **§11.4.224** — Test-first (TDD) for ALL work, not only fixes + a minimum code-coverage floor that is NECESSARY-never-sufficient (User mandate, 2026-07-22)
- **§11.4.238** — Automated QA must be the DISCOVERER: manual QA finds nothing new, and anything it finds is a coverage escape (User mandate, 2026-08-08)
- **§11.4.239** — Critical-invariant work-class Definition of Done mandate — money/safety/availability/integrity code MUST include failure-path scenarios as gates, not merely happy-path passes (research-derived, …
- **§11.4.242** — Bisection-not-blame regression cause-finding + failure modes (research-derived, 2026-08-15)
- **§11.4.243** — Characterization / golden-master safety net for legacy code before behaviour-changing modification (research-derived, 2026-08-15)
- **§11.4.244** — Cross-boundary contract tests + can-i-deploy gate (research-derived, 2026-08-15)
- **§11.4.245** — Oracle-problem-first test authoring: identify the oracle BEFORE authoring the test (research-derived, 2026-08-15)
- **§11.4.246** — Reproducible + hermetic builds + supply-chain integrity at SLSA Build Level 2 (operator decision, 2026-08-15)
- **§11.4.247** — Composite-output layer-move must move every layer (research-derived, 2026-08-15)
- **§11.4.248** — Flaky-test quarantine + protected-regression-spec gate via `[PROTECTED-SPEC: ATM-NNN]` tag + CODEOWNERS required reviewer (operator decision, 2026-08-15)
- **§11.4.249** — Producer ≠ oracle ≠ gate ≠ verifier + flight recorder (research-derived, 2026-08-15)
- **§11.4.250** — Heuristic-tower signals a primitive defect: when N heuristic layers stack to compensate, the primitive is broken (research-derived, 2026-08-15)

### Translation and localization (3 anchors)

Canon: `submodules/constitution/groups/translation-and-localization.md`

- **§11.4.237** — Mandatory exhaustive context-and-spirit-aware translation review: every localized artifact independently reviewed for CONTEXT (source) + SPIRIT (target-language idiom/register) + …
- **§11.4.255** — Mandatory HelixTranslate canonical translation pipeline (User mandate, 2026-06-25)
- **§11.4.256** — Mandatory independent per-language translation review (User mandate, 2026-06-25)

### Workable items and tracking (19 anchors)

Canon: `submodules/constitution/groups/workable-items-and-tracking.md`

- **§11.4.15** — Item-status tracking mandate
- **§11.4.16** — Item-type tracking mandate
- **§11.4.21** — Operator-blocked status + self-resolution exhaustion mandate (User mandate, 2026-05-14)
- **§11.4.33** — Type-aware closure-status vocabulary (User mandate, 2026-05-15)
- **§11.4.34** — Reopened-source attribution mandate (User mandate, 2026-05-15)
- **§11.4.54** — ATM-NNN ticket identifier mandate (User mandate, 2026-05-19)
- **§11.4.55** — Reopens-history tracking + per-item Reopens.md doc (User mandate, 2026-05-19)
- **§11.4.90** — Obsolete status + per-item obsolescence audit mandate (User mandate, 2026-05-27)
- **§11.4.91** — Summary-doc clarity mandate (User mandate, 2026-05-27)
- **§11.4.92** — Multi-pass change-evaluation discipline (User mandate, 2026-05-27)
- **§11.4.93** — SQLite-backed single-source-of-truth for workable items (User mandate, 2026-05-27)
- **§11.4.95** — Workable-items SQLite DB is TRACKED in git, NEVER gitignored (User mandate, 2026-05-27)
- **§11.4.104** — Participant identity, attribution & notification-tagging (User mandate, 2026-05-31)
- **§11.4.112** — Structural-impossibility won't-fix classification mandate (research-derived, 2026-06-03)
- **§11.4.148** — Workable-item integrity (status+type+id) + comprehensive structured description + bidirectional external-tracker sync + BLOCKED unblock-choices mandate (User mandate, 2026-06-10)
- **§11.4.149** — Per-workable-item testing-diary mandate (User mandate, 2026-06-10)
- **§11.4.171** — Mandatory comprehensive human-readable workable-item descriptions (User mandate, 2026-06-29).
- **§11.4.202** — Reporting directives: a report MUST auto-create a fully-populated, fully-synced workable item — never a prose acknowledgement (User mandate, 2026-07-15).
- **§11.4.214** — Recurrence-links-not-mints: a defect that returns MUST reopen its existing item, never enter as a new id (research-derived, 2026-07-17)

## Known Conflicts and Open Decisions

These were found while reading every source in full. Canon wins on each. Items marked
`UNCONFIRMED` could not be verified from this repository and are recorded rather than assumed
(§11.4.6). Items 2 to 4, 7, 11 and 13 carry a mixed FIXED/OPEN status inline; the others are: 1 DECIDED (the
stricter project limit governs), 5 DECIDED (operator), 6 OPEN, 8 NOTE (informational), 9 DECIDED
(operator, per-application phase-in), 10 OPEN, 12 DECIDED (workflow narrowed), 14 NOTE (state at
commit time), 15 DECIDED (operator), 16 DECIDED (operator). Statuses: FIXED in this change
(main-repo files only), DECIDED (an autonomous reversible default under §11.4.101 where no
"operator" is named, otherwise a decision the operator gave), OPEN (an operator decision or a
follow-up), NOTE (informational).

1. **Test resource limits.** `catalog-api` caps test runs at `GOMAXPROCS=3`, `-p 2 -parallel 2`
   and 30 to 40% of host CPU and RAM. Canon allows up to 60% of RAM for session-resident work
   (§12.6) and a dynamic maximal fraction for containerized builds (§12.11). These are
   compatible because a project may tighten canon: the stricter `catalog-api` limit governs
   host-resident test runs, and §12.11 governs containerized builds.
2. **CI and hook wording.** Module `AGENTS.md` files say "Pre-commit hooks block them; CI fails
   on them", and the root `AGENTS.md` refers to failing "the CI lane". Canon forbids active CI/CD
   pipelines (§11.4.156) and forbids hooks that can block commit or push (§11.4.234). Enforcement
   is local, run as explicit stages of a dedicated script. **FIXED** in the root and the seven
   module `AGENTS.md` files and in the root `CLAUDE.md`. **OPEN:** the same template wording remains
   in owned submodules' own files, which belong to separate repositories.
3. **Haiku as a mechanical tier.** `actions/subagent_tiering.yaml` names `haiku` as the
   mechanical model. §11.4.231 (D.1) prohibits `haiku` in the reference repository because its
   governing context (about 421k tokens) exceeds the tier's 200k window (request
   `req_011CfSBXm7Kn6enyUT1UEaWd`). Whether the same holds for Catalogizer is `UNCONFIRMED`: this
   repository has not measured it. Until it does, `haiku` is treated as excluded. **FIXED:**
   `config/subagent_tiering.yaml` sets the mechanical tier to `sonnet` (select it with
   `HELIX_SUBAGENT_TIERING`). The judgment tier is `inherit`, but reviews and merge-conflict
   resolution stay pinned to Opus at `xhigh` regardless of tiering (§11.4.209, §11.4.211).
4. **Bare-host build and run commands.** The root `GEMINI.md` (`go mod tidy`, `go run`,
   `npm install`, `docker-compose up -d`), `catalog-api/CLAUDE.md` (`go build`) and the module
   guides document builds on the bare host, and one names `docker-compose`. §11.4.173 forbids
   building anywhere but in a container on the remote build host, and §11.4.161 requires rootless
   Podman. The development-run commands are conveniences and MUST NOT produce release
   artifacts. **FIXED** in `GEMINI.md` (development-run banner, `podman compose`) and
   `catalog-api/CLAUDE.md` (`go build` marked dev-only). **FIXED** the same way in the six other module `CLAUDE.md`
   command sections (a development-run banner), `README.md` (every `docker-compose` command, 10 more),
   `Website/docs/getting-started/index.md` and 11 guide documents under `docs/` (`podman compose`,
   no `sudo`). **OPEN:** historical reports under `docs/status/` still contain `docker-compose
   up`; they are records and are not rewritten.
5. **HelixPlay §17–§21.** `CONSTITUTION.md` adopts the HelixPlay Constitution v2.3.0 "in full"
   from an external URL, and `CLAUDE.md` states "This submodule is part of the HelixPlay system".
   The R-01 to R-18 clauses, the eight pillars, the technology stack of §20 and the 14-phase
   roadmap are not present in this repository, so their text is `UNKNOWN` here. Whether the
   30 ms LAN and 50 ms WAN p999 latency SLA of §19 applies to the Catalogizer REST API (it was
   written for HelixPlay) is `UNCONFIRMED`; it is therefore not made a hard gate here. The
   operator should decide, and vendor the text if it is binding (§11.4.215: a binding document
   must be tracked in this repository). **DECIDED by the operator (2026-10-03):** the latency SLA
   does NOT bind Catalogizer (it belongs to HelixPlay streaming); Catalogizer sets its own
   performance targets and aims for the best achievable performance (spec 001, SC-011). The
   HelixPlay text therefore need not be vendored for the SLA; the other HelixPlay clauses stay
   `UNKNOWN` here.
6. **Website dead links.** `Website/.vitepress/config.ts` sets `ignoreDeadLinks: true`, tolerating
   links to unimplemented pages. That is a gap against the zero-gaps invariant (§11.4.261) and
   the documentation-coverage rules (§11.4.257). **OPEN:** turning it off may break the site build,
   and a build on the bare host is forbidden (§11.4.173), so it needs a containerized build first.
7. **Compliance artifacts.** `docs/requests/history.md` (§11.4.208) did not exist. **FIXED:**
   it is created and tracked in this change, starting 2026-10-02 with earlier sessions not
   reconstructed. The §2 single commit entrypoint is still not confirmed: a root `commit` script
   exists, but no `scripts/commit_all.sh` or `commit-push-all.sh` was found (**OPEN**).
   `docs/CONTINUATION.md` exists.
8. **Host-level index lag.** The host index file loaded by the agent lists anchors only through
   §11.4.271; the pinned canon defines through §11.4.276 (§11.4.272 to §11.4.276 are real). The
   Anchor Catalogue above is generated from the canon's machine index `constitution_index.yaml`,
   and at the current pin that index itself lags `Constitution.md` (see item 18): the catalogue
   lists the 283 anchors the index knows and omits §11.4.276, whose operative digest is in
   `.specify/memory/constitution-appendix.md`, Part 1. `Constitution.md` is authoritative.
9. **Owed operator decision.** Brownfield adoption of the 85% coverage floor (immediate hard
   floor, one-time monotone ratchet, per-corpus phase-in, or changed-code-only with a deadline)
   is the operator's call under §11.4.224 and §11.4.66. **DECIDED by the operator (2026-10-03):**
   per-application phase-in (spec 001, FR-011): each application records a coverage baseline and
   a dated target, and no application may fall below its baseline. This replaces the earlier
   interim ratchet. During the phase-in the 85% floor gates new and changed code in full, while
   existing code is held to its recorded baseline and dated target.
10. **Gate code owed.** Canon recommends many mechanism gates (named `CM-*`) whose code is a
    separate, unshipped work item. This document does not claim any of them is implemented
    (§11.4.227).
11. **Stale Android documentation.** The Android `CLAUDE.md` and `AGENTS.md` files stated compileSdk
    34, Compose BOM 2024.01 and a 2048m Gradle heap, while the build files use compileSdk 35 (phone),
    BOM 2024.12.01 (phone) and 2024.06.00 (TV), and `-Xmx4096m -XX:MaxMetaspaceSize=1024m`.
    **FIXED:** the documents now match the build files. Whether the heap was meant to stay at 2048m
    is `UNCONFIRMED` and is an operator decision. The TV guide's Kotlin daemon `-Xmx1024m` figure has no
    matching `gradle.properties` line and stays `UNCONFIRMED` (see item 13).
12. **Spec-first versus autonomy.** Spec-first development (Principle workflow below) is the rule
    for new feature work. Canon makes the autonomous loop the default working mode with no per-step
    approval (§11.4.126, §11.4.101, §11.4.87), and defect fixes follow systematic debugging and
    reproduce-first (§11.4.102, §11.4.146) without a spec. The workflow rules are narrowed to say so.
13. **Further doc-versus-build discrepancies (found by the module digests, verified against the
    files).** **FIXED:** Android app version (now 2.4.0, versionCode 6), Android TV (2.4.0,
    versionCode 8) and the TV Compose artifacts (`tv-foundation` 1.0.0-alpha11, `tv-material`
    1.0.0) in the module `CLAUDE.md` files; the desktop guide now lists the optional `vlc-player`
    feature and the Playwright scripts; the root `AGENTS.md` no longer says force-push can be
    authorized (it is absolutely banned, §11.4.113) and now says eight remotes (it said four).
    **OPEN:** both Android `gradle.properties`
    set `org.gradle.java.version=17` while the guides call JDK 21 the default; the TV guide
    describes a `kotlin.daemon.jvmargs` line that does not exist in its `gradle.properties`;
    `catalog-web` uses TypeScript 4.9 and `catalogizer-api-client` uses TypeScript 5. Each needs
    an operator decision on the intended value.
14. **Repository state at commit time (verified with `git ls-remote` and `git status`).** The
    `constitution` submodule's remote is at least 6 commits ahead of the pin as of the last fetch
    (those 6 change hook and fastcycle tooling, not `Constitution.md`, `constitution_index.yaml` or
    `groups/`); later remote commits are `UNCONFIRMED` because the remote keeps moving. The pin is
    deliberately not moved here. `submodules/websocket_client_ts` had a local `github` remote with the wrong owner
    (`nickkvasic`, which returns 404 even when authenticated as `milos85vasic`; `.gitmodules` and every
    other owned submodule use `vasic-digital`). Found by systematic debugging and **FIXED** by
    pointing the remote at `vasic-digital/WebSocket-Client-TS` (local git configuration, not tracked);
    `git ls-remote github` now returns the same commit as the local HEAD (6e624db0). `submodules/helix_qa/tools/opensource/docling` is a vendored
    third-party repository with one modified data file; it is not committed or pushed.
15. **Branch policy for spec 001.** The operator instructed (2026-10-03) that all work is done on
    the main branch of the main repository and of every submodule. This is a decision for the audit
    and remediation work and overrides the default branch-per-feature convention of §11.4.195 for
    it; the rest of §11.4.195 and §11.4.113 stand (fast-forward only, no force-push, push to every
    upstream, nothing committed without review). Because work lands directly on `main`, the live
    manual QA of §11.4.185 still gates every release, as the merge step of §11.4.195(B) is skipped.
    **DECIDED (operator).**
16. **Dependency update scope for spec 001.** The operator decided (2026-10-03) that "keep
    dependencies current" means submodules only: fetch and pull the latest codebases from all of
    their upstreams (spec 001, FR-017). Third-party package dependencies are reported, not bulk
    updated. Pulling the governance submodule includes its post-pull sweep and hook (§11.4.26,
    §11.4.32, §11.4.164). **DECIDED (operator).**

17. **Canon Sonnet fallback (found by the 2026-10-06 pin move to `a71b1767`).** Canon §11.4.209
    (code review) and §11.4.211 (merge-conflict resolution), amended 2026-10-04, REQUIRE a fallback:
    when Opus at `xhigh` is genuinely unavailable (a captured fact) the work MUST be dispatched on
    Sonnet rather than blocked, and it is blocked only when both Opus and Sonnet are unavailable;
    canon names `--leave-review-blocked-while-sonnet-reachable` as a forbidden escape hatch. An
    earlier text of this document (Principle III) said "blocked, never substituted"; that digested
    the pre-amendment canon and was never an owner mandate (ODG-19: "Opus xhigh as pinned
    (§11.4.209)"; OA-2026-10-05-23: "run independent reviews ... with Opus xhigh and record effort";
    the id OA-2026-10-05-23 was not found in any tracked markdown by the 2026-10-06 fix pass, so it
    is cited as the review reported it, UNCONFIRMED locator). **OPEN canon conflict, owner decision,
    not made by an agent:** whether the project wants the canon fallback (default, follow canon) or
    to ask for a canon change; a project rule that blocks where canon requires a fallback would
    weaken canon, which a project may not do. Interim practice, an agent's interim choice and not an
    owner rule: follow canon (Opus `xhigh` first, Sonnet fallback only on a captured Opus
    unavailability fact, recorded with model and effort). Recorded as OD-WP07-SONNET-FALLBACK in
    `decisions/owner-request-list.md`.
18. **Upstream index and groups lag at `a71b1767` (reported, not fixed here).** At the new pin the
    machine index `constitution_index.yaml` is byte-identical to the one at the old pin: it records
    the `Constitution.md` hash of the old canon and lacks §11.4.276, and its generator guard
    stops with `anchor 11.4.276 matches no group rule`. The `groups/*.md` files also lack the
    amendments to §11.4.134, §11.4.209, §11.4.211, §11.4.230, §11.4.231, §11.4.235, §11.4.240 and
    §11.4.267 and the whole of §11.4.276. The appendix digests of those anchors were written from
    `Constitution.md` (the pinned canon), not from the groups files. Evidence:
    `specs/001-full-project-audit-remediation/evidence/wp07/pin-upstream-findings.md`.

## Governance

This constitution is the highest governing document for Spec Kit development activities in this
repository, subordinate only to the Helix Universal Constitution.

**Amendment procedure.** An amendment requires a documented rationale, an update to dependent
specs and plans, and verification that Principles I to VIII are not weakened. Amendments MUST be
reviewed under Principle III before acceptance. The appendix is amended with this document and
regenerated whenever the canon pin moves. A principle MUST NOT be removed or weakened to
make a change pass. When the `submodules/constitution` pointer moves, the pinned commit and hash
above MUST be updated, the Anchor Catalogue regenerated from `constitution_index.yaml`, and the
post-pull sweep run (§11.4.26, §11.4.32, §11.4.164). Rules landing in canon are classified
universal or project-specific (§11.4.17) and never duplicated here beyond a digest (§11.4.227).

**Versioning policy.** Semantic versioning applies. MAJOR for backward-incompatible principle
removals or redefinitions. MINOR for a new principle or section, or materially expanded guidance.
PATCH for clarifications and wording. A pin bump that only adds anchors is MINOR.

**Compliance review.** Each plan MUST include a constitution compliance check. Reviews MUST reject
changes that violate a principle or a canon anchor. Complexity beyond what a principle requires
MUST be justified in the plan. Canon conflicts are resolved in canon's favour, and any conflict
between a module rule and canon is recorded under Known Conflicts.

**Owed items.** The items marked OPEN in Known Conflicts are open, and the DECIDED defaults (those not marked as operator decisions) may be
replaced by the operator.

**Version**: 2.2.0 | **Ratified**: 2026-10-02 | **Last Amended**: 2026-10-06
