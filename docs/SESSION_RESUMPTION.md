# SESSION RESUMPTION — Catalogizer

> Standing, out-of-the-box resumption file per Constitution **§11.4.131**.
> Use this to resume the active program in a fresh session with zero ramp-up.
> The live, fine-grained work-state lives in
> [`docs/CONTINUATION.md`](./CONTINUATION.md) (§12.10).

---

## SHORT (first-sentence variant)

Resume the in-flight **Helix Constitution submodule onboarding + owned-submodule
reorganization** program for Catalogizer at
`/Volumes/T7/Projects/catalogizer` (parent `main` @
`fbca0f96c1134beb3aeb2a3ca0eb7a6ed3e33182`, `constitution` @
`2849155`): the constitution is onboarded, committed and pushed to all 6
upstreams, and the inheritance gate is authored but uncommitted — the next step
is to dry-run then execute `scripts/reorg_submodules.sh`, rebuild-verify, then
commit + push the parent and every owned submodule to all upstreams (NEVER
force-push).

---

## FULL block

### What this program is

A single major in-progress program with terminal goal:

- **(A)** onboard shared Helix Constitution as `constitution`;
- **(B)** relocate ALL owned submodules under `submodules/<snake_case>`;
- **(C)** wire inheritance + gate;
- **(D)** rebuild-verify everything;
- **(E)** commit + push EVERYTHING (constitution + parent + all owned
  submodules) to all upstreams.

**Phase:** mid-execution. (A) done; (C) authoring done (uncommitted); (B), (D),
(E) ahead.

### Moment-valid coordinates

| Item | Value |
|------|-------|
| Parent path | `/Volumes/T7/Projects/catalogizer` |
| Parent branch | `main` |
| Parent HEAD | `fbca0f96c1134beb3aeb2a3ca0eb7a6ed3e33182` |
| `constitution` | pinned `main`, HEAD `2849155cbce506e9ca60ccc681b9bc71d7432c40` (short `2849155`) |
| Backups | `/Volumes/T7/Projects/.catalogizer-backups/` |

### Done (real, captured)

- `.env` / `.env.example`: `HELIX_RELEASE_PREFIX=catalogizer` (§11.4.151).
- `constitution` added, pinned to main; 6 upstreams wired via
  `install_upstreams.sh`.
- Constitution edits (`helix_translate`→`myproject`; `find_constitution.sh`
  layout-aware; exports synced; meta-test PASS) — **COMMITTED + PUSHED to all 6
  upstreams** (HEAD `2849155`).
- Parent root `CLAUDE.md`/`AGENTS.md`/`CONSTITUTION.md`/`GEMINI.md`:
  inheritance pointers added *(uncommitted)*.
- Inheritance gate authored + validated *(uncommitted)*:
  `scripts/verify_constitution_inheritance.sh`,
  `scripts/meta_test_constitution_inheritance.sh`,
  `tests/test_constitution_inheritance.sh`.
- Distribution config: `thinker.local` + `amber.local` confirmed, `nezha`
  excluded (comments added to `.env.distributed` / `.env.spread` /
  `.env.roundrobin`) *(uncommitted)*.
- catalog-api Go build fixed (`go mod tidy` reconciled `go.mod`/`go.sum`; was
  RED on stale `go.sum`, now `go build ./...` GREEN) *(uncommitted)*.

### Known pre-existing issues (NOT yet fixed)

1. catalog-api `go vet ./...` fails:
   `tests/stress/concurrent_handlers_test.go: undefined: setupStressTestServer`.
2. `Challenges/challenges/fixtures/memprobe/go.mod` references nonexistent
   `helix_memory`.

### Next actions (strict order)

1. Review + dry-run + execute `scripts/reorg_submodules.sh` (moves 40 owned
   submodules to `submodules/<snake>`, rewrites ~128 refs — mapping + plan in
   the script and in `docs/scripts/reorg_submodules.md`).
2. Rebuild-verify against GREEN baseline (go build catalog-api, npm
   catalog-web, Dockerfile, HelixQA scripts).
3. Fix `go vet` stress-test gap (known issue #1).
4. Inject inheritance pointers into every owned submodule.
5. Commit parent via its mechanism + bump submodule pointers.
6. Push parent + every owned submodule to all upstreams on latest `main`
   (NEVER force-push — §11.4.113).
7. Verify every repo pushed.

### Binding constraints

- Anti-bluff §11.4 — real captured evidence only, no bluff, no green-on-broken.
- Never force-push (§11.4.113); always merge onto latest `main`.
- snake_case submodules under `submodules/` (§11.4.28 / §11.4.29).
- Backups at `/Volumes/T7/Projects/.catalogizer-backups/`.

### Where to look next

- Live work-state: `docs/CONTINUATION.md` (§12.10, §11.4.44 revision header).
- Reorg plan: `docs/scripts/reorg_submodules.md` + `scripts/reorg_submodules.sh`.
