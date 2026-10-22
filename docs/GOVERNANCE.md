# Governance and the Constitution

| Field | Value |
|---|---|
| Revision | 3 |
| Created | 2026-10-21 |
| Last modified | 2026-10-22T00:30:00Z |
| Status | active |
| Describes | the Spec Kit constitution `.specify/memory/constitution.md` (version 2.5.0) and `.specify/memory/constitution-appendix.md` (revision 7), pinned to `constitution` at `5f8503b4f4f0f051b583d447a240c2a2a7ac253a` (the pin this change stages; canon follow-up of `a3d80ce`; previous committed pin `4bcb512`) |
| Authority | The governing text is the Spec Kit constitution and, above it, the canon in `constitution/Constitution.md`. This page is a guide to them and never replaces them. Where it disagrees with either, they win. |

## Table of contents

- [1. What governs this repository](#1-what-governs-this-repository)
- [2. How the Spec Kit constitution follows the canon](#2-how-the-spec-kit-constitution-follows-the-canon)
- [3. Moving the canon pin](#3-moving-the-canon-pin)
- [4. Constitution hub engines](#4-constitution-hub-engines)
- [5. Safe submodule initialisation](#5-safe-submodule-initialisation)
- [6. Open items about the engines](#6-open-items-about-the-engines)
- [7. Places that still advise a recursive update](#7-places-that-still-advise-a-recursive-update)
- [8. Verification commands](#8-verification-commands)

## 1. What governs this repository

Rules are layered. A lower layer may tighten a higher one and never weakens it.

| Rank | Layer | File | Role |
|---|---|---|---|
| 1 | Helix canon | `constitution/Constitution.md` (a git submodule) | The universal rules: anchors §11.4.1 to §11.4.278, host safety §12, data safety §9. The source of truth. |
| 2 | Spec Kit constitution | `.specify/memory/constitution.md` and `.specify/memory/constitution-appendix.md` | Catalogizer's governance layer for Spec Kit. It states the principles, the project overrides, the known conflicts, the hub-engine binding, and carries the operative digest of every canon anchor. |
| 3 | Project constitution | `CONSTITUTION.md`, `CLAUDE.md`, `AGENTS.md`, `GEMINI.md` at the root | Project rules and agent guidance. |
| 4 | Module rules | `CLAUDE.md` and `AGENTS.md` inside each module | Module-specific rules. |

Where canon and a lower layer disagree, canon wins and the conflict is recorded in the Spec Kit constitution under
"Known Conflicts and Open Decisions".

## 2. How the Spec Kit constitution follows the canon

The canon is about 2 MB, so it is incorporated in four layers instead of being copied (copying would break the canon's own
token-efficiency rule §11.4.141 and its no-duplicate rule §11.4.227):

1. **Binding by reference.** Every clause of the pinned canon binds in full.
2. **Stated** in the constitution file: the principles, the digest of §1 to §12, the overrides, the technology, the known conflicts and
   the hub-engine section.
3. **Enumerated** in the constitution file: the Anchor Catalogue, generated from the canon's machine index `constitution_index.yaml`.
4. **Ported** in the appendix: the operative rules of every canon anchor (Part 1), every project and module rule (Part 2), and the
   hub-engine register (Part 3).

The regions that are a pure function of the canon (the "Pinned sources" paragraph, the Anchor Catalogue, the appendix "Canon pin" row)
are written by `scripts/governance/regen_speckit_catalogue.py`; everything else is hand-written and reviewed. The appendix is compared
with the canon by `scripts/governance/appendix_token_check.py`, which checks every `CM-*` gate name and every `--flag` of every anchor,
a wording floor and a list of required phrases.

## 3. Moving the canon pin

Do these in order, in one change:

1. Fetch every remote of the constitution submodule and read what is incoming before accepting it:
   `git -C constitution fetch --all` then `git -C constitution diff --stat <old-pin> <new-pin>`. Note whether `Constitution.md`,
   `constitution_index.yaml`, `groups/`, `scripts/`, `helix-deps.yaml` or only `.gitmodules` and gitlinks changed.
2. Move the pointer fast-forward only. Never rewrite history or force-push (§11.4.113).
3. Regenerate the derived regions: `python3 -I scripts/governance/regen_speckit_catalogue.py write ...` (arguments in section 8), then
   run it again with `check`; it must exit 0.
4. Update the hand-written parts that the diff touches: the principles and digests when anchor text changed, the hub-engine section
   and Known Conflicts when an engine gitlink or `helix-deps.yaml` changed, the appendix digests when canon text changed.
5. Run `scripts/governance/appendix_token_check.py` (section 8); it must exit 0.
6. Bump the constitution version by the policy in its Governance section (a pin bump that adds anchors or a new section is MINOR).
7. Have the change reviewed independently before it is committed (§11.4.142, §11.4.209), then commit and push fast-forward to every
   configured upstream (§2.1).

## 4. Constitution hub engines

The canon is a hub (its `helix-deps.yaml` uses the §11.4.28 depth-1 carve-out): besides rules it vendors reusable engines under
`constitution/submodules/`. Catalogizer vendors the constitution, so the engines arrive at the constitution's pin. Three of them are
bound by the Spec Kit constitution's section "Constitution Hub Engines" (rules HE-1 to HE-8, operative digest in appendix Part 3).
Measured on 2026-10-22 at constitution `5f8503b` (working-tree sizes by `du -sh` the same day):

| Engine | Origin | What it is | Gitlink | In canon's `helix-deps.yaml` | Size |
|---|---|---|---|---|---|
| `llmctl` | `vasic-digital/llmctl` (own-org) | Local LLM orchestration: hardware probe, model plan, verified download, llama.cpp / colibri / onnx engines as user services, and the model validation gate (§11.4.277). | `f42bad68` (`helixconstitution-v70-68-gf42bad6`), nests only third-party llama.cpp and colibri | yes (ref `f42bad6`, non-recursive) | 301 MB |
| `claude_toolkit` | `vasic-digital/claude_toolkit` (own-org) | Many Claude Code and Kimi Code accounts over one shared store, provider aliases for any LLM API key, OpenCode sync. The alias mechanism behind §11.4.196 and §11.4.198. | `1f6dad3b` (`v1.30.5-14-g1f6dad3`), nests only the third-party Go toolchain, ships `helix-deps.yaml` | yes (ref `1f6dad3`, non-recursive) | 222 MB |
| `open_design` | `nexu-io/open-design` (third-party, Apache-2.0) | The OpenDesign design-agent workspace: skills, design systems, templates, the `od` CLI and an MCP server. The UI design system of §11.4.162. | `3b0df90f30`, `package.json` 0.25.1 | yes (`third_party`, ref `3b0df90`) | 326 MB |

Owner mandate (2026-10-21): one source of truth and no nested submodule chains; main submodules live in `<root>/submodules` or
`<root>/constitution/submodules`. Canon was amended on the constitution branch `feat/hub-engines-single-source`, merged and pushed to
canon `main` as `a3d80ce`, followed by `5f8503b` (claude_toolkit pin `1f6dad3`; `git ls-remote origin refs/heads/main` = `5f8503b` on 2026-10-22, and every remote-tracking `main` contains it; this change stages that pin): the hub may carry ONE pinned reference checkout of `open_design`
(§11.4.162; consumption stays a dependency), engines MUST NOT nest `constitution`, the hub, other engines or flat-provided
repositories (§11.4.28(C) extension, gate `CM-HUB-ENGINE-NO-OWNORG-NESTING`), and the hub vendors two more engines flat. The table
above describes `5f8503b`; the hub additions are:

| Engine | Origin | Hub pin at `5f8503b` |
|---|---|---|
| `containers` | `vasic-digital/Containers` (own-org) | `919ad1e` |
| `superspec` | `WangX0111/superspec` (third-party) | `c20ac6c` |

Canon's `helix-deps.yaml` at `5f8503b` lists `claude_toolkit`, `open_design` (third-party, Apache-2.0), `containers` and `superspec`.
Engine pins: `llmctl` `f42bad6` (no longer nests `constitution`, `containers`, `superspec` or `challenges`; resolves them through one
resolver: environment override, parent hub, `<root>`, sibling; host fallback only by explicit opt-in variable) and the root
`submodules/helix_qa` gitlink `ad8b12b`. `claude_toolkit` was recorded at `223c702` (tag `v1.30.5`) inside canon `a3d80ce`, where it nested own-org repositories and had no `helix-deps.yaml` (both gates RED; history). Canon `5f8503b` pins the pushed flatten `1f6dad3`: it nests only the third-party Go toolchain (`submodules/go`), resolves `HelixConstitution`, `containers`, `challenges`, `LLMsVerifier` and `claude-code-router` flat through `scripts/resolve_deps.sh`, and ships a tracked `helix-deps.yaml`. Measured 2026-10-22: `CM-HUB-ENGINE-NO-OWNORG-NESTING` PASS ("submodules/claude_toolkit@1f6dad3 nests only third-party repositories"), `CM-HELIX-DEPS-MANIFEST-PRESENT` PASS for canon (pass 11 / fail 0 / skip_third_party 17) and `scripts/verify_helix_deps_manifests.sh` `VERDICT: PASS (pass=54 fail=0 blind=0 skip_third_party=45 known=0)` (item 27, RESOLVED). `helix_qa` de-registered its 27
nested tool repositories (manifest `docs/flat-tools-manifest.tsv`); this project registers them flat as 27 submodules at
`submodules/<snake_name>/` (allure2, appium, scrcpy, perfetto, and so on; 27 gitlinks registered).

What the binding means for a developer, in short (the constitution has the full rules):

- Use an engine from `constitution/submodules/<name>`; never copy, fork or patch it here. A change is an upstream change plus a pin bump.
- Initialise it one level only (section 5). Never `--recursive` through the constitution.
- The engines are tooling. They are never imported by, copied into or shipped with a Catalogizer product artifact.
- `llmctl`: models are acquired and run only through it, validated with `LLMCTL_VALIDATE_STRICT=1` and `llmctl require-validated`.
- `claude_toolkit`: aliases are made only with its commands; secrets stay outside the repository.
- `open_design`: reference material and an optional authoring tool. Read its `PRIVACY.md` before running its application (analytics are
  on by default, with an opt-out, according to the vendor). The project's own token modules stay the single token source for now.

At `5f8503b` canon lists all three engines in its manifest, and §11.4.162 allows the hub ONE pinned reference checkout of `open_design` (consumption stays a dependency, never forked) with `design-toolkit` (`vasic-digital/design-toolkit`) carrying the token and brand role (Known Conflicts item 26). The vendored `open_design` is therefore reference material here. Still open in canon: the `claude-toolkit` / `claude_toolkit` spelling.

## 5. Safe submodule initialisation

Use the single safe entry, `scripts/safe_submodule_init.sh` (tests: `scripts/tests/test_safe_submodule_init.sh`). It initialises the
top-level submodules, the constitution one level only, and only the hub engines asked for, one level each. `scripts/quick-start-phase0.sh`,
`scripts/build-test-release.sh` and `scripts/container-build.sh` call it; the former `--recursive` run in `quick-start-phase0.sh` is
fixed. On a fresh clone use `git clone <url>` (no `--recursive`) and then the safe entry. The manual equivalent for one engine is:

```bash
git submodule update --init constitution
git -C constitution submodule update --init submodules/llmctl submodules/claude_toolkit submodules/open_design
```

At canon `5f8503b` no engine nests an own-org repository any more (`CM-LLMCTL-SUBMODULE-NONRECURSIVE` PASS,
`CM-HUB-ENGINE-NO-OWNORG-NESTING` PASS; items 25 and 27), but `llmctl` and `claude_toolkit` still nest third-party repositories
(llama.cpp and colibri; the Go toolchain) that are never initialised recursively (`recursive_init: false`), so an unqualified
`git submodule update --init --recursive` through `constitution/` stays unsupported here. Upgrade leftover:
when an engine drops a gitlink, git leaves a real populated directory behind; after initialising, the safe entry scans
`submodules/*` and `constitution` under the root, under `constitution/` and under every initialised `constitution/submodules/<engine>`,
and prints `WARN leftover: <path> ...` on stderr for each directory that holds a `.git` but is not a gitlink in its repository's index.
The exit status is unchanged and nothing is moved or deleted; remediation is manual only (fixture test
`scripts/tests/test_safe_submodule_init.sh`; paired mutation test `scripts/tests/test_safe_submodule_init_mutation.sh` applies six
mutants to a scratch copy, among them dropping the gitlink check, dropping the `constitution` leftover glob and moving a leftover, and
requires the fixture test to FAIL on each).

## 6. Open items about the engines

Recorded in the Spec Kit constitution under Known Conflicts; none is changed by this documentation.

| Item | Summary | Owner |
|---|---|---|
| 21 | llmctl adoption: which llmctl binary the vision step resolves, the vendored copy versus the one on PATH, the host cannot run a Vision model yet. | project owner |
| 25 | Canon's cycle gate failed at `4bcb512`. RESOLVED: canon `a3d80ce` (and `5f8503b`) pins the cleaned llmctl `f42bad6`; `cm_llmctl_submodule_nonrecursive.sh` PASSes (2026-10-22). | constitution repository |
| 26 | FIXED in canon `a3d80ce` (merged and pushed), except the `claude-toolkit` / `claude_toolkit` spelling. | constitution repository |
| 27 | RESOLVED (2026-10-22): canon `5f8503b` pins `claude_toolkit` at `1f6dad3` (pushed flatten; at `a3d80ce` it was `223c702`, nested own-org repositories and had no `helix-deps.yaml`). It nests only the third-party `submodules/go` and ships a `helix-deps.yaml`; `CM-HUB-ENGINE-NO-OWNORG-NESTING` and `CM-HELIX-DEPS-MANIFEST-PRESENT` PASS, `scripts/verify_helix_deps_manifests.sh` PASSes (pass 54 / fail 0). No nested own-org working tree remains on disk under `constitution/submodules/claude_toolkit`. | constitution repository |
| 28 | OpenDesign: the 2026-06-25 consumability verdict has not been re-verified against the vendored checkout; `design/opendesign/DESIGN.md` and tokens are planned in task T393. | project owner |
| 29 | RESOLVED (2026-10-22): the index scope was re-rendered from DATA with the project's own tools (`scripts/audit/derive_scope.sh` -> `specs/.../audit/submodules.tsv`, 160 -> 104 rows, only the rows of the old nested engine chains removed; `scripts/audit/scope_to_lumen_json.py` -> `config/index/lumen_scope.json`; `scope_render.py --write` -> `codegraph.json`; `scope_drift_check.sh --regen` -> `.lumenignore`), and `scripts/audit/scope_drift_check.sh` exits 0 (all three checks rc 0). The fresh-clone test D13 clones HEAD and passes only once this change is committed (an equivalent render from the staged index tree matches). Still owed: the §11.4.79 scope proof on a re-run index. | project owner |
| 30 | `containers` drift: the hub vendors `919ad1e`, the root `submodules/containers` pins `4a8f04e`; the product-side bump is deferred until after the constitution work. | project owner |
| 31 | Host credential exposure 2026-10-21: four token variables (`GITVERSE_TOKEN`, `GITHUB_MODELS_API_KEY`, `GITLAB_TOKEN`, `GITFLIC_TOKEN`; names only) were printed into session output; rotation recommended. | project owner |
| 32 | The owner typed macOS host credentials (`mistborn.local`, `amber.local`) in chat; none is recorded anywhere; move to a gitignored `.env` or SSH keys and rotate. | project owner |

## 7. Places that still advise a recursive update

Measured on 2026-10-21 by a search outside `constitution/`, `submodules/`, `specs/` and `node_modules/`; the scripts, the Website, the
course, video and phase documents now use the safe entry or the safe form (`scripts/quick-start-phase0.sh`, `scripts/build-test-release.sh`,
`scripts/container-build.sh`, `scripts/meta_test_constitution_inheritance.sh`, `scripts/verify_constitution_inheritance.sh`,
`tools/evidence/matrix/derive_applicability.py`, `Website/*`, `docs/VIDEO_COURSE_SCRIPTS.md`, `docs/courses/*`, `docs/video-course/*`,
`docs/phases/PHASE_0_FOUNDATION.md`). The remaining mentions are historical plan documents (`docs/superpowers/plans/*`) and test
fixtures that exercise other repositories; they are left as they are.

## 8. Verification commands

Read-only; no network. From the repository root:

```bash
git -C constitution rev-parse HEAD
git -C constitution submodule status -- submodules/llmctl submodules/claude_toolkit submodules/open_design
git -C constitution diff --stat 94b8836 HEAD
bash constitution/scripts/gates/cm_llmctl_submodule_nonrecursive.sh --root "$PWD/constitution"   # exit 1 at 4bcb512, PASS at a3d80ce and 5f8503b (item 25)
bash constitution/scripts/gates/cm_hub_engine_no_ownorg_nesting.sh   # FAIL at a3d80ce (claude_toolkit@223c702), PASS at 5f8503b (item 27)
bash constitution/scripts/gates/cm_helix_deps_ref_matches_gitlink.sh --root .   # BLIND only for the hub-layout deps
bash scripts/verify_helix_deps_manifests.sh   # PASS at 5f8503b: pass=54 fail=0 blind=0 skip_third_party=45 (item 27)

C=$(git -C constitution rev-parse HEAD)
python3 -I scripts/governance/regen_speckit_catalogue.py check \
  --index constitution/constitution_index.yaml --canon-md constitution/Constitution.md --commit "$C" \
  --constitution-file .specify/memory/constitution.md --appendix-file .specify/memory/constitution-appendix.md

python3 -I scripts/governance/appendix_token_check.py \
  --canon constitution/Constitution.md --appendix .specify/memory/constitution-appendix.md \
  --allow scripts/governance/appendix_token_allow.tsv --floor scripts/governance/appendix_wording_floor.tsv \
  --phrases scripts/governance/appendix_required_phrases.tsv
```

The governance tools use the Python standard library and PyYAML. The project normally runs them through the `IMG-TESTUTIL` image with
`scripts/containers/run_pinned.sh`; they also run directly on a host that has PyYAML.
