# Background Queue

| Field | Value |
|---|---|
| Revision | 1 |
| Created | 2026-10-12 |
| Status | active |
| Rule | §11.4.210 / §11.4.197 (scheduled work is never dropped) |

## BG-LLMCTL-001 - llmctl vision + decision models for HelixQA and catalogizer (CRITICAL, scheduled)

Source: operator request 2026-10-12 15:24 CEST (see `docs/requests/history.md`). To be mirrored into the
workable-items DB via `.helix/reporting.yaml` + `report_item.sh` once the register rework (WP-20/22, W2) lands.

### Facts captured 2026-10-12 (this session, host commands)
- `llmctl` IS installed: `~/.local/bin/llmctl` -> `/home/milosvasic/Projects/llmctl/bin/llmctl` (checkout with
  llama.cpp + colibri submodules initialised, `llama-server` built, doctor PASS on engine/cert/key checks).
- Host: Ryzen 7 2700X, 30974 MiB RAM (9327 MiB available at probe), 1x RTX 3060 12288 MiB VRAM.
- `llmctl status`: only `decide-nli` (onnx, cpu, port 8096) running and enabled. NO vision model running.
- `llmctl plan`: host tier `below-minimum` (RAM budget = available - 4 GiB = 5000 MiB at probe time).
  `vision` (port 8082, 2048 MiB RAM + 4030 MiB VRAM, gpu mode) and `vision-pro` are GATED by `min_tier`,
  decision profiles `decide-tiny/julia/kev-08b/laya/nli` FIT. The gate is RAM-availability driven, so it can
  clear when host RAM is freed; it is NOT silently overridden (that is an operator decision).
- `llmctl doctor`: one FAIL (service registry vs live set: `decide-nli`, fix `llmctl-decide registry reconcile`)
  and WARNs (cert SAN drift 192.168.1.115, decision maturity unmeasured).
- HelixQA submodule: `submodules/helix_qa` (already has VisionEngine + LLM provider selection); no llmctl
  support and no llmctl submodule yet.

### Work program (acceptance-defining)
1. Detection: HelixQA + catalogizer resolve llmctl in order: `LLMCTL_BIN` env, `command -v llmctl`, then the
   HelixQA-vendored submodule `submodules/helix_qa/submodules/llmctl` (add via the constitution submodule workflow,
   all upstreams, install_upstreams); fall back to setup/install (`llmctl setup`) before use.
2. Vision: `llmctl auto vision` / `llmctl enable vision` (systemd --user), health probe of port 8082 with a real
   image-in/text-out smoke, then bind HelixQA VisionEngine and every catalogizer vision consumer to it.
3. Decisions: route typed noul/choice/score decisions through the llmctl decide gateway (HTTPS, key in gitignored
   `.env`), measured maturity only (experimental profiles labelled, never trusted blindly).
4. Determinism: pinned model digests, temperature 0, seed, golden fixtures with control needles, N>=3 identical
   canonical hashes; vision verdicts accepted only with machine-readable evidence; no model verdict as a gate
   on its own (§11.4.269): the model proposes, a deterministic oracle (OCR/pixel/DOM) confirms.
5. Tests: unit + integration + e2e + challenges + HelixQA banks covering all apps (web, desktop, android, tv, api),
   edge cases (model down, gated tier, OOM, slow, wrong answer), paired mutations, no mocks outside unit tests.
6. Docs: manual, guide, FAQ, diagrams; README-reachable (§11.4.212); script docs under docs/scripts.
7. Host safety: model start only within llmctl's co-residency budget and the 60% memory ceiling; never force
   past a tier gate without an operator decision; no root/sudo.

### Open decision (operator, §11.4.66)
`vision` is tier-GATED on this host while RAM is contended. Options: (A) start it when RAM frees (recommended,
no override); (B) set the host tier explicitly via llmctl's supported override; (C) use `vision-pro` only after
more RAM. Not decided; nothing was forced.

## 2026-10-30 11:27 CET — continuous release train and 5-round cap (BACKGROUND :: CRITICAL)
Operator mandate (verbatim, typos preserved): "Make sure we do not have more than 5 rounds unless absolutely needed! Minor issues can stay but to be tackled in next iterations. We MUST aim to make as much as possible releases consicutivelly! Even after we do release something we continue testing, fixing and polishing in the background and preparation iof next better version that is goinf to be released! This is continuous process wehich MUST BE already defined and explained to details in our constitution Submodule! Add any details we may be missing there so this is absolutely clear!"
Status: applied to the working process now (memory `feedback_review_budget_policy.md`); constitution text being drafted by an author agent in a scratch clone of the v72.2 series, needs independent review before it joins canon v72.2. Not yet in canon.
