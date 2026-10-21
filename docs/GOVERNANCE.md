# Governance and the Constitution

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-21 |
| Last modified | 2026-10-21T13:57:15Z |
| Status | active |
| Describes | the Spec Kit constitution `.specify/memory/constitution.md` (version 2.4.0) and `.specify/memory/constitution-appendix.md` (revision 6), pinned to `constitution` at `4bcb51236115b0592583f5a2f8fa7e33e1a328c3` |
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
Measured on 2026-10-21 at constitution `4bcb512`:

| Engine | Origin | What it is | Gitlink | In canon's `helix-deps.yaml` | Size |
|---|---|---|---|---|---|
| `llmctl` | `vasic-digital/llmctl` (own-org) | Local LLM orchestration: hardware probe, model plan, verified download, llama.cpp / colibri / onnx engines as user services, and the model validation gate (§11.4.277). | `5354354b`, 56 commits past tag `v3.2.2` | yes (`v3.2.2`, non-recursive) | 454 MB |
| `claude_toolkit` | `vasic-digital/claude_toolkit` (own-org) | Many Claude Code and Kimi Code accounts over one shared store, provider aliases for any LLM API key, OpenCode sync. The alias mechanism behind §11.4.196 and §11.4.198. | `223c7025`, tag `v1.30.5` | no | 407 MB |
| `open_design` | `nexu-io/open-design` (third-party, Apache-2.0) | The OpenDesign design-agent workspace: skills, design systems, templates, the `od` CLI and an MCP server. The UI design system of §11.4.162. | `3b0df90f30`, `package.json` 0.25.1 | no | 326 MB |

What the binding means for a developer, in short (the constitution has the full rules):

- Use an engine from `constitution/submodules/<name>`; never copy, fork or patch it here. A change is an upstream change plus a pin bump.
- Initialise it one level only (section 5). Never `--recursive` through the constitution.
- The engines are tooling. They are never imported by, copied into or shipped with a Catalogizer product artifact.
- `llmctl`: models are acquired and run only through it, validated with `LLMCTL_VALIDATE_STRICT=1` and `llmctl require-validated`.
- `claude_toolkit`: aliases are made only with its commands; secrets stay outside the repository.
- `open_design`: reference material and an optional authoring tool. Read its `PRIVACY.md` before running its application (analytics are
  on by default, with an opt-out, according to the vendor). The project's own token modules stay the single token source for now.

Canon itself names `llmctl` as a vendored engine and lists it in its manifest; it does not yet do so for `claude_toolkit` and `open_design`. Canon also contradicts itself on OpenDesign: §11.4.162 says it is consumed as a project dependency and never vendored, while the hub now vendors it; and the manifest lists a separate engine, `design-toolkit` (`vasic-digital/design-toolkit`), for the token and brand role (Known Conflicts item 26). The vendored `open_design` is therefore reference material here. The Spec Kit constitution binds them for this project and records the gap as a proposed canon change.

## 5. Safe submodule initialisation

`git submodule update --init --recursive` from the repository root descends into the constitution and then into the engines. That is
unsafe because `llmctl` and `claude_toolkit` each vendor a constitution of their own (a mutual reference), and canon's own cycle gate
`CM-LLMCTL-SUBMODULE-NONRECURSIVE` currently FAILs at the pinned commit. Use this instead:

```bash
# 1. every top-level submodule except the constitution, with its own nested submodules (as before)
git submodule status | awk '$2 != "constitution" {print $2}' | xargs -r git submodule update --init --recursive --

# 2. the constitution itself, one level only
git submodule update --init constitution

# 3. only the hub engines you need, one level each
git -C constitution submodule update --init submodules/llmctl submodules/claude_toolkit submodules/open_design
```

On a fresh clone, use `git clone <url>` (no `--recursive`) and then the three steps. This rule governs the constitution. The nested
submodules of the other top-level submodules follow their own manifests and are outside it.

## 6. Open items about the engines

Recorded in the Spec Kit constitution under Known Conflicts; none is changed by this documentation.

| Item | Summary | Owner |
|---|---|---|
| 21 | llmctl adoption: which llmctl binary the vision step resolves, the vendored copy versus the one on PATH, the host cannot run a Vision model yet. | project owner |
| 25 | Canon's cycle gate fails: llmctl's `constitution` pin contains `submodules/llmctl`. | constitution repository |
| 26 | Canon does not list `claude_toolkit` and `open_design` in `helix-deps.yaml` and no clause names them; a canon change is proposed. | constitution repository |
| 27 | `claude_toolkit` also vendors a constitution, with no gate; its nested submodules are all initialised here (407 MB); an ignored 158 MB `docs/research/decisions_models` copy sits in the hub checkout. | project owner, constitution repository |
| 28 | OpenDesign: the 2026-06-25 consumability verdict has not been re-verified against the vendored checkout; `design/opendesign/DESIGN.md` and tokens are planned in task T393. | project owner |
| 29 | Code-index scope data does not cover the engines; the scope renderer fails closed on `claude_toolkit`'s nested `go` gitlink; the Lumen scope's derivation input (`specs/.../audit/submodules.tsv`) still names `submodules/constitution/...`. | project owner |

## 7. Places that still advise a recursive update

Measured on 2026-10-21 by a search outside `constitution/`, `submodules/`, `specs/` and `node_modules/`. The list is PARTIAL: the search also finds the course and video-script documents under `docs/courses/`, `docs/video-course/` and `docs/phases/` and an error message in `tools/evidence/matrix/derive_applicability.py`. This page, `README.md`,
`docs/DEVELOPER_GUIDE.md`, `docs/SUBMODULE_DEPENDENCIES.md` and `GETTING_STARTED.md` carry the safe form. These still advise the
recursive form and are follow-up work (they are scripts, the website and a course script, and were not changed here):
`scripts/quick-start-phase0.sh` (line 237, runs it), `scripts/meta_test_constitution_inheritance.sh` (line 59, message),
`scripts/verify_constitution_inheritance.sh` (line 70, message), `scripts/build-test-release.sh` (line 60), `scripts/container-build.sh`
(line 219), `Website/getting-started.md`, `Website/contributing.md`, `Website/developer/contributing.md`,
`Website/docs/getting-started/quick-start.md` and `docs/VIDEO_COURSE_SCRIPTS.md`.

## 8. Verification commands

Read-only; no network. From the repository root:

```bash
git -C constitution rev-parse HEAD
git -C constitution submodule status -- submodules/llmctl submodules/claude_toolkit submodules/open_design
git -C constitution diff --stat 94b8836 HEAD
bash constitution/scripts/gates/cm_llmctl_submodule_nonrecursive.sh --root "$PWD/constitution"   # exit 1 at 4bcb512 (item 25)

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
