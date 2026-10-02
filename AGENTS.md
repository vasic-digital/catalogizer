# AGENTS.md — Catalogizer

> ## INHERITED FROM submodules/constitution/AGENTS.md
> Base agent rules live in `submodules/constitution/AGENTS.md` — READ IT
> FIRST. The base file (and `submodules/constitution/Constitution.md`) is
> authoritative for any topic not covered here. Project-specific rules
> below extend them; they never weaken them.

> **NON-NEGOTIABLE PRIME DIRECTIVE:**
> **"We had been in position that all tests do execute with success and
> all Challenges as well, but in reality the most of the features does
> not work and can't be used! This MUST NOT be the case and execution
> of tests and Challenges MUST guarantee the quality, the completion
> and full usability by end users of the product!"**
> This statement is the foundational requirement of this project. Any
> agent dispatch, any local gate configuration, any code review that allows
> green tests on broken features is a violation and MUST be rejected.

> **Constitution v2.3.0**: [Read the Constitution](https://github.com/HelixDevelopment/HelixPlay/blob/main/docs/research/chapters/MVP/05_Response/01_Constitution.md)
> All rules in Constitution §1-§21 are MANDATORY. No exception.
>
> **Amendments (2026-05-02):**
> - Anti-bluff enforcement strengthened: no vacuous assertions, no
>   constructor-only tests, no mock-only integration/E2E tests, no
>   untriaged skips.
> - Usability evidence mandatory per §6.7.
> - Automatic negative-leg fault injection per §1.3 / §6.3 / §11.5.7.
> - `ValidateAntiBluff` unconditional; all challenges call `RecordAction()`.
> - Container verifier `execCommand()` executes real commands.
> - `go vet ./...` MUST pass with zero warnings — no suppressions, no exceptions.
> - Anti-bluff scan MUST fail the local validation stage (no CI/CD, §11.4.156): `scripts/anti-bluff-scan.sh` exits
>   non-zero on any violation. Process substitution (`< <(...)>`) required over
>   pipes for variable state propagation.
> - Observable behaviour assertion ratio: at least 60% of assertions must verify
>   observable behaviour per §1.2.
> - Mutation score >= 85% enforced by `mutation_ratchet_challenge.sh` per §6.4.
> - The 18 Contract Clauses (R-01..R-18) codified in §17.
> - Eight Architectural Pillars codified in §18 — binding architectural decisions.
> - Performance SLAs codified in §19 — <=30ms LAN, <=50ms WAN at p999.
> - Technology Stack codified in §20 — mandatory technology choices.
> - Implementation Roadmap codified in §21 — 14 phases (P00–P13).

## Repo state
This is a `vasic-digital` / `HelixDevelopment` submodule for HelixPlay.

## Critical constraints
- **Anti-bluff:** No placeholders, dead code, vacuous tests. Details in Constitution §1.
- **Containers only:** Every service, DB, build, test runs inside a container.
- **Decoupling:** Reusable components live in public `vasic-digital` submodules.
- **Tests:** 100% coverage across all ten types. Only Unit may use mocks.
- **R-18 Operational Integrity:** No command may suspend/hibernate/lock/terminate/crash the host.

## Git topology
`origin` fetch=GitHub, push=GitFlic and five further hosts (six push URLs). Eight remotes are configured in total.
Force-push is forbidden absolutely, with or without authorization (§11.4.113); fetch, merge onto latest `main` and push fast-forward only. `--no-verify` is forbidden.

**§11.4.173 — Containerized + distributed build mandate (User mandate, 2026-06-29).** EVERY build of EVERY component (source compile, artifact/package/installer/container-image production, codegen, asset render — for ANY language/platform: Go, Android/Gradle, desktop, web, native, firmware) MUST run INSIDE a specialized build container provisioned via the `digital.vasic.containers` submodule (§11.4.76) — NEVER on the bare host. The build containers MUST be DISTRIBUTED to the designated remote build host(s) (e.g. `thinker.local`) via the SAME containers-submodule distribution mechanism the infra uses (§11.4.76 Distributor / remote compose over SSH, §11.4.161 rootless), so the build EXECUTES on the remote build host (offloading the developer/main host); once the build completes the produced artifacts MUST be brought BACK to the originating main host (scp/rsync/volume copy) for use/flashing/distribution. Building outside a container, or on the bare host, is FORBIDDEN — a release blocker (the "works on my machine" / unreproducible-build class §11.4.76 exists to prevent). The build-host target + build-container definitions are config-injected (§11.4.28, never hardcoded in the submodule); a missing build-container capability is added by EXTENDING the containers submodule upstream (§11.4.74), never by an ad-hoc host build. Honest boundary (§11.4.6): the containerized+distributed build guarantees reproducibility + host-isolation + capacity offload — it does NOT replace the §11.4.40 full-suite retest, §11.4.108 four-layer artifact→runtime verification, or §11.4.38 installable-asset evidence (those still run against the brought-back artifact). Classification: universal (§11.4.17). Composes §11.4.76 (containers submodule — sole orchestration layer), §11.4.161 (rootless runtime), §11.4.74 (extend-don't-reimplement), §11.4.28 (config injection / decoupling), §11.4.24 (build-resource stats), §11.4.82 (iteration speedup — persistent caches in the container), §11.4.121 (no-commit-while-build-writes-artifacts), §11.4.38 (installable-asset evidence on the brought-back artifact), §11.4.108 (artifact→runtime verification), §12.6 (host memory ceiling — offloading the build preserves the main host). Propagation gate `CM-COVENANT-114-173-PROPAGATION` (literal `11.4.173`) + recommended gate `CM-CONTAINERIZED-DISTRIBUTED-BUILD` (every build runs via the containers submodule on the remote build host; a bare-host build is detected + FAILs) + paired §1.1 mutation. **Canonical authority:** constitution submodule [`Constitution.md`](Constitution.md) §11.4.173. Non-compliance is a release blocker. No escape hatch — no `--build-on-host`, `--skip-container-build`, `--local-build-ok`, `--no-distributed-build`, `--bare-host-build` flag.
