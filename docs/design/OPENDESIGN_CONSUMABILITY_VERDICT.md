# OpenDesign Consumability Verdict

| Field | Value |
|---|---|
| Revision | 2 |
| Created | 2026-06-25 |
| Last modified | 2026-10-21 |
| Status | Revised after reading the vendored checkout; supersedes the revision 1 verdict (kept below); independent review pending (section 4a) |
| Authority | §11.4.99 latest-source verification, §11.4.6 no guessing, §11.4.162 (amended 2026-10-21) |
| Subject | `constitution/submodules/open_design`, pinned `3b0df90f305c8279e748c292accf336d02b924ef`, root `package.json` version 0.25.1, Apache-2.0 (read-only; nothing was run, no network used) |

## 1. Verdict (revision 2)

1. Revision 1 conclusion "not an npm dependency" is CONFIRMED for the code packages: all 17 `packages/*` are `private: true` except `@open-design/dsh-runtime` (an unrelated runtime), and none is a token package.
2. Revision 1 was INCOMPLETE on tokens: the checkout ships 151 bundled design-system packages (`design-systems/<slug>/` with `manifest.json`, `DESIGN.md`, `tokens.css`, often `design-tokens.json` and `tailwind-v4.css`) in a documented, schema-checked format, so a **tokens.css contract and a Tailwind v4 binding DO exist as consumable files**.
3. Revised verdict: **consumable via file export / format conformance (pinned reference checkout), plus dev tool only for authoring; NOT consumable as a runtime/package dependency.** For Catalogizer this is still a project-authored token module, but now required to conform to the OpenDesign `tokens.css` contract instead of a loosely mirrored taxonomy.
4. Android Compose and Android TV: no artifact, no path; hand-authored Kotlin tokens generated from the canonical `tokens.css` remain the only route.
5. Running the app (`od`) is OPTIONAL and not required for any consumption path; if ever run, privacy opt-outs must be set first (section 5).

## 2. Evidence: what the checkout publishes

Commands run (read-only): `ls`, `git log -1`, a Python loop printing `name/private/license/exports` of every `packages/*/package.json`, `sed`/`head` of `design-systems/README.md`, `docs/design-systems.md`, `design-systems/default/*`, `PRIVACY.md`, `apps/daemon/src/cli.ts`, `apps/daemon/src/mcp.ts`.

### 2.1 Packages
- Root `package.json`: `"name": "open-design"`, `"private": true`, `"bin": {"od": "./apps/daemon/bin/od.mjs"}`, `engines node ~24, pnpm >=10.33.2 <11`.
- `packages/*` (agui-adapter, components, contracts, diagnostics, download, dsh-runtime, host, launcher-proto, metatool, platform, plugin-runtime, registry-protocol, release, sidecar, sidecar-proto, standalone): every one `"private": true` except `dsh-runtime` (`Apache-2.0`, no `private`; it is a runtime, not tokens). `@open-design/components` exports `./styles.css` but is private and an app UI kit, not Catalogizer tokens. Result: nothing installable from npm from this repo.
- `packages/contracts` exports the token SCHEMA used by `design-systems/_schema/tokens.schema.ts` (`export * from "../../packages/contracts/src/design-systems/token-schema.ts"`): a contract, not published.

### 2.2 Design systems and token formats
- `design-systems/README.md`: "The bundled catalog currently contains **151 packages**. Every bundled package has the same minimum machine-readable shape: manifest.json, DESIGN.md, tokens.css"; "`tokens.css` is the canonical compiled semantic-token stylesheet." Counted: 154 entries in the directory, 152 with a `tokens.css` (one is `_schema`, README, LICENSE, PRIVACY).
- `manifest.json` schema `od-design-system-project/v1` (fields `id`, `name`, `category`, `description`, `source`, `files.design|tokens|designTokens|tailwind|components`).
- `design-systems/default/` (Neutral Modern) additionally ships `design-tokens.json` (`"format": "od-design-tokens/v1"`, 56 tokens, layers A1-identity / B-slot / A2 / A1-structure), `tailwind-v4.css` ("Derived from tokens.css. Keep tokens.css as the source of truth.": `@import "tailwindcss"; @import "./tokens.css"; @theme { --color-bg: var(--bg); ... }`), `components.html`, `components.manifest.json`.
- `docs/design-systems.md` section 4: `:root { --bg --fg --accent --font-display --font-body ... }` and dark as `[data-theme="dark"] { --bg: ...; --fg: ... }`; "A final `tokens.css` must contain every required shared slot; brand-specific extensions must use the documented extension allowlist". Standard names (default `tokens.css` header): `--bg --surface --fg --muted --border --accent --success --warn --danger --font-display`.
- Observation: the bundled `default/tokens.css` contains no `[data-theme` selector (grep count 0 for it); the dark override is a documented pattern, not guaranteed per bundled system.
- No registry/URL for fetching a design system as a versioned artifact was found in the files read; the `od design-systems` CLI talks to a running daemon.

### 2.3 CLI `od` and MCP
- `od design-systems`: `list`, `show <id>`, `rename`, `download <id> [--out]` ("Download a brand .zip (files + SKILLS.md)"), `import-local`, `import-github`, `import-shadcn`, `rebuild-token-contract <id>`; `od tools design-systems read --path <manifest-declared-path>`. All take `--daemon-url`, i.e. they need a RUNNING daemon (UNKNOWN whether any works offline).
- `od mcp [--daemon-url]` (stdio) and `od mcp install <agent>`; MCP tools in `apps/daemon/src/mcp.ts` include `collect_brief`, `confirm_brief`, `list_projects`, `get_project`, `get_file`, `search_files`, `list_files`, `create_artifact`, `write_file`, `delete_file`, `delete_project`, `create_project`, `list_skills`, `list_plugins`, `start_run`, `get_run`, `cancel_run`, `list_agents`, login tools. None is a "get tokens" tool; they operate on projects/artifacts.

## 3. What "consume as a project dependency" means per stack

Canon §11.4.162 (as amended 2026-10-21) says consume OpenDesign as a dependency, never fork, hub may carry a pinned reference checkout. With what exists:

| Stack | Mechanism available | Meaning here |
|---|---|---|
| catalog-web (React + Tailwind) | `tokens.css` contract + `tailwind-v4.css` pattern (`@theme` mapping `--color-*` to `var(--*)`) | Author `tokens.css` (Catalogizer Blue) conforming to the required slot list of `token-schema.ts`; map with Tailwind `@theme`/`theme.extend`. Pin = the reference checkout commit the contract was read from. NOT a package import. Note: the Tailwind v4 `@theme` form needs Tailwind v4 (UNKNOWN which version catalog-web uses; verify before copying). |
| catalogizer-desktop (Tauri, web view) | same CSS files | Same file consumed through the web layer; one canonical `tokens.css`. |
| catalogizer-android (Compose) | none | Hand-authored/generated `Color`/`ColorScheme` Kotlin from the canonical `tokens.css` (generator is Catalogizer work, §11.4.216). |
| catalogizer-androidtv (Compose TV) | none | Same as Android, TV palette from the same source. |
| Dev tooling | `od` + MCP + `design-systems download` | Authoring aid only (generate/review a `DESIGN.md`), an ephemeral local tool, not a build dependency. |

## 4. Revised verdict detail

Result: "token-module fallback" stands in practice, but is upgraded from "mirror the taxonomy loosely" to "conform to the published `tokens.css` slot contract and the manifest/DESIGN.md package shape, pinned to a recorded OpenDesign commit". Two honest levels:

- Consumable via export/format: YES for web and desktop (CSS files, schema-checked contract).
- Consumable as a package/runtime dependency: NO (all packages private, no registry artifact found).
- Dev tool only: YES for `od`/MCP (needs Node 24, pnpm, a daemon; not run).

### 4a. UNKNOWN / needs running or further reading (not done here)
1. Exact required-slot list and extension allowlist in `packages/contracts/src/design-systems/token-schema.ts` (file not read in full; must be read to write a conforming `tokens.css`).
2. Whether the repo guard (`pnpm guard`) that validates `tokens.css` can be run standalone on a Catalogizer file without installing the whole workspace.
3. Whether `od design-systems download/show` and `rebuild-token-contract` work without a model provider/BYOK key and without network.
4. Which Tailwind major version catalog-web uses (affects `tailwind-v4.css` reuse).
5. Whether any bundled system ships a vetted dark override; how contrast (WCAG AA) is validated by OpenDesign's own tooling.
6. Telemetry destinations and exact opt-out switch when the daemon runs headless (PRIVACY.md describes Settings UI; `OD_TELEMETRY_ENV` exists in `apps/daemon/src/telemetry-environment.ts`, its semantics were not read). Whether "safety and reliability telemetry" can be disabled at all: PRIVACY.md says it is NOT disabled by the analytics toggle in builds with a telemetry destination.
7. Release cadence / compatibility promise of the token contract between versions (0.25.1 is pre-1.0).
8. Independent review of this revision (§11.4.142) has not been performed.

## 5. Licence and privacy

- Licence: `LICENSE` is Apache-2.0 (and `package.json` `"license": "Apache-2.0"`); copying or deriving `tokens.css` structure is permitted with attribution/NOTICE obligations; per-system provenance is in each `manifest.json` `source` (brand-named systems such as apple, ferrari, bmw may derive from third-party brand identities: do NOT copy those; UNKNOWN their licensing).
- Privacy (PRIVACY.md): product analytics and quality traces are ON BY DEFAULT (opt-out; first-run banner), covering anonymous metrics, privacy-masked session replay and, if enabled, conversation/tool content (prompts, outputs) sent to PostHog and, via a relay, Langfuse. "Safety and reliability telemetry" is always enabled in builds configured with a telemetry destination and is not disabled by the analytics toggle; forks and dev builds without destination credentials send neither class. Source IP is visible to the receiving service. Local-first: projects and BYOK keys stay local.
- Implication: reading the checkout and copying CSS has no privacy effect. Running `od`/the packaged app from this repo would need: Settings, Privacy, "Don't share" before use, no project content, preferably a source build without telemetry credentials, a network-egress block, and recorded evidence of what leaves the host (UNKNOWN until observed; not done).

## 6. Impact on task T393 (tokens.css canonical + design/opendesign/DESIGN.md)

T393 (specs/001-full-project-audit-remediation/tasks.md line 923) says: `catalog-web/src/styles/tokens.css` canonical with `:root` and `[data-theme="dark"]`, `tokens.ts` regenerated from it, `design/opendesign/DESIGN.md` and `design/opendesign/tokens.css` (§11.4.216, §11.4.217). Effects of revision 2:
- Direction CONFIRMED and strengthened: CSS canonical generating TS (OD-58) matches OpenDesign's own "tokens.css is canonical, tailwind-v4.css is derived" rule.
- Add to T393: names of the CSS variables follow the OpenDesign standard slots (`--bg --surface --fg --muted --border --accent --success --warn --danger --font-display ...`) and the required-slot list of `token-schema.ts` (read it first, UNKNOWN 1); a `[data-theme="dark"]` block is mandatory for Catalogizer even where bundled systems lack one.
- Add `design/opendesign/manifest.json` (schema `od-design-system-project/v1`) next to `DESIGN.md` and `tokens.css`, and a provenance note recording the pinned OpenDesign commit `3b0df90f...` the contract was read at (§11.4.99, §11.4.223 provenance markers).
- `design/opendesign/tokens.css` should be the single repo-level canonical; `catalog-web/src/styles/tokens.css` and `catalogizer-desktop/src/styles/tokens.css` become generated copies or imports (no hand copies, §11.4.216).
- Optional deliverable: `tailwind` binding file only after UNKNOWN 4 is resolved.
- Android/TV Kotlin token generation is NOT in T393's current text; it needs its own task (generator from the canonical `tokens.css`) or an explicit statement that it is outside T393.
- A conformance check (required slots present, no raw hex outside `:root`) can reuse the OpenDesign rule set only if UNKNOWN 2 resolves; otherwise Catalogizer writes its own detector (already in T393 as the raw-hex ratchet).
- Do not run `od` for T393; no telemetry exposure is introduced.

---

# Superseded revision 1

The text below is the 2026-06-25 verdict, kept verbatim except that its headings are demoted one level. Its claim that no token artifact exists is corrected by section 2.2 above.

## OpenDesign Consumability Verdict

**Revision:** 1
**Last modified:** 2026-06-25T00:00:00Z
**Authority:** §11.4.99 latest-source verification · §11.4.6 no-guessing — every claim cited to a fetched URL
**Scope:** Resolves the shared `UNCONFIRMED` blocker in the OpenDesign (§11.4.162) integration plans for the catalogizer web, Tauri desktop, and androidtv clients.

---

### VERDICT

**NOT-CLEANLY-CONSUMABLE → token-module fallback.**

`nexu-io/open-design` is a **local-first desktop application + CLI + MCP server**, NOT a publishable design-token / design-system code dependency. It publishes **no npm package that exports design tokens** for programmatic import by any application stack. Its "design systems" are `DESIGN.md` **markdown prose files** authored to be fed into an AI coding agent's prompt — they are not an importable token artifact.

- **npm token package for direct import: NONE.** (See evidence below — the only nexu-io-adjacent npm packages are an MCP bridge and markdown-template installers, neither of which is a consumable token library.)
- **Hand-authored design-token module fallback: CONFIRMED CORRECT for all three stacks** (web/React+Tailwind+CSS-custom-properties, Tauri desktop, Android/Jetpack Compose).

---

### What `nexu-io/open-design` actually is

A **native desktop app** (Electron shell; Next.js 16 / React 18 / TypeScript frontend; Node 24 / Express / SQLite daemon) — self-described as a "Local-first, open-source Claude Design alternative." It generates design artifacts (web/desktop/mobile prototypes, slides, images, videos, HyperFrames) and exports HTML/PDF/PPTX/MP4/ZIP/Markdown.

Its distribution model is explicitly **"skills, a CLI, and an MCP server — not an npm package"** (consumed as a native app, or wired into coding agents via `od mcp install <agent>` over the Model Context Protocol). Each "design system" is a portable `DESIGN.md` file (a markdown schema following the `awesome-claude-design` / VoltAgent `awesome-design-md` 9-section format) read as part of an agent's system prompt. Some systems additionally ship a `tokens.css` ("canonical compiled CSS custom properties") — but this is a per-system file inside the repo / template install, NOT a versioned, importable npm token library.

### npm reality (the blocker, resolved)

Registry search for the nexu-io/open-design design-token package returns **none**. The closest npm artifacts are:

| npm package | What it is | Consumable as a token dependency? |
|---|---|---|
| `open-design-mcp` (v0.16.1, publisher `nano-step001`) | MCP stdio server bridging coding agents to the Open Design daemon | No — it is agent tooling, not a token export |
| `open-design-templates` (v1.1.0, publisher `deceivedleaped`) | CLI (`bin: od-templates`) that **copies `DESIGN.md` / `SKILL.md` markdown** into Open Design's directories | No — installs markdown, not JS/JSON/CSS tokens; "not a React, Tailwind, Android, or Compose dependency" |
| `getdesign` (v0.6.24, repo `VoltAgent/awesome-design-md`, **not** nexu-io) | CLI (`bin: getdesign`) — `npx getdesign add <slug>` installs pre-built `DESIGN.md` markdown templates | No — distributes markdown templates, not token objects |

There is no `@nexu-io/open-design`, no `@nexu/open-design`, no nexu-io-published design-token library. The `od` command referenced in the README is the desktop app's CLI, not an installable token package.

### Per-stack consumption mechanism

#### (a) React + Tailwind + CSS-custom-properties (catalogizer web & Tauri desktop)
**No clean dependency path.** There is no npm package to `npm install` that yields importable tokens. The only OpenDesign-native artifact is `DESIGN.md` (markdown prose) plus, per-system, a `tokens.css`. A `tokens.css` could in principle be hand-copied into the web/desktop app and referenced as CSS custom properties + a Tailwind `theme.extend` mapping — but that is a manual copy of one repo file, not a maintained, versioned dependency, and it does not carry the project's "Catalogizer Blue" palette (those tokens would still have to be authored). **Concrete mechanism: author the tokens locally** (see fallback).

#### (b) Jetpack Compose / Android (catalogizer androidtv)
**No consumption mechanism exists.** OpenDesign produces no Kotlin/Compose artifact, no Android resource set, and no Gradle-publishable token library. `tokens.css` is not consumable by Compose. There is no path from `nexu-io/open-design` into an Android build. **Compose tokens must be hand-authored.**

### Confirmed fallback (the plans' call is correct)

For **all three stacks**, the correct approach is a **hand-authored design-token module that mirrors the OpenDesign `DESIGN.md` / `tokens.css` token structure** (the 9-section schema: visual theme, color palette light+dark, typography, spacing, etc.), encoding the project's **"Catalogizer Blue" light + dark palette**:

- **Web / Tauri desktop:** a single source-of-truth token file (e.g. `tokens.css` CSS custom properties for light + dark `:root` / `[data-theme="dark"]`), consumed by Tailwind via `theme.extend` referencing `var(--…)`. Mirror OpenDesign's section structure so a future OpenDesign export can be diffed against it.
- **Android TV / Compose:** a Kotlin token object / `MaterialTheme` `ColorScheme` (light + dark) mirroring the same token names/values.

This satisfies §11.4.162's intent (use a design-token/theme system with light+dark variants from brand assets, no ad-hoc one-off CSS) while honestly reflecting that OpenDesign is not consumable as a dependency. The §11.4.74 "extend upstream, don't reimplement" obligation does not bite here: there is no upstream token *library* to extend — the upstream is an app + markdown, so authoring a project-local token module mirroring its schema is the sanctioned path, not a duplication of a reusable package.

---

### Sources verified 2026-06-25

- https://github.com/nexu-io/open-design — repo README, tech stack, "skills/CLI/MCP — not an npm package" distribution model (fetched 2026-06-25)
- https://github.com/nexu-io (org), https://open-design.ai/ — project identity: local-first Claude Design alternative, Apache-2.0, BYOK (via web search, 2026-06-25)
- https://github.com/nexu-io/open-design/blob/main/design-systems/README.md — `DESIGN.md` = "canonical design prose for agents", `tokens.css` = "canonical compiled CSS custom properties" (fetched 2026-06-25)
- https://github.com/nexu-io/open-design/blob/main/docs/spec.md — design systems follow the external `awesome-claude-design` 9-section schema; the 9 sections are defined in that external repo, not inline (fetched 2026-06-25)
- https://registry.npmjs.org/getdesign — `getdesign` v0.6.24, "CLI for installing DESIGN.md templates", repo `VoltAgent/awesome-design-md`, `bin: getdesign`, NOT nexu-io (fetched 2026-06-25)
- https://registry.npmjs.org/-/v1/search?text=nexu-io%20open-design — full npm search; no nexu-io design-token library; surfaces `open-design-mcp` (MCP bridge) + `open-design-templates` (template installer) (fetched 2026-06-25)
- https://registry.npmjs.org/open-design-templates — `open-design-templates` v1.1.0, installs `DESIGN.md`/`SKILL.md` markdown, `bin: od-templates`, "not a React, Tailwind, Android, or Compose dependency" (fetched 2026-06-25)
- https://www.npmjs.com/package/getdesign, https://www.npmjs.com/search?q=open-design — npmjs.com returned HTTP 403 to the fetcher; the authoritative `registry.npmjs.org` API (above) was used instead (attempted 2026-06-25)

