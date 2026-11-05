# HelixQA against QA stack 3

Roadmap-Item: HELIXQA-STACK3

`run.sh` drives HelixQA (`submodules/helix_qa`) against the live stack 3
(API :38082, front door :33080, web :33020). Credentials are read by variable
name (`ADMIN_USERNAME`, `ADMIN_PASSWORD`) from `QA_ENV`
(default `/home/milosvasic/.cache/claude-tmp/catqa3/qa.env`) inside a subshell and never printed.

## Build the binary (constitution 11.4.173: in a container, not on the host)

The helix_qa module `replace`s sibling submodules (`../challenges`, `../containers`, ...)
and uses workspace mode, so mount the whole `submodules/` dir:

    podman run --rm --memory 3g --cpus 3 -v $PWD/submodules:/src:ro -v OUT:/out \
      -v GOCACHE_DIR:/gocache -v GOMOD_DIR:/gomod -e GOCACHE=/gocache -e GOMODCACHE=/gomod \
      -w /src/helix_qa docker.io/library/golang:1.26 go build -o /out/helixqa ./cmd/helixqa

(Do not set `GOFLAGS=-mod=mod` or `GOWORK=off`: the first is rejected in workspace mode, the second fails on missing go.sum entries.)
Place the result in `bin/helixqa` (gitignored) or set `HELIXQA_BIN`.

## Run

    docs/qa/stack3/helixqa/run.sh        # evidence in evidence/<ts>/

Exit codes: 0 all PASS (API and UI vision), 1 FAIL, 2 partial/blocked
(API green but UI vision not verified counts as 2, never 0), 3 SKIP (tool or credentials absent).
Evidence: `http_api.json` (real per-step status), `run.log`, `verdict.txt`.

## Honest scope (11.4.278 / 11.4.289)

* The API leg uses `helixqa http` with `smoke_bank.yaml` (3 live cases). Live, no stubs.
* The UI/Vision leg is NOT done: it needs `HELIX_VISION_ENDPOINT` backed by an llmctl vision
  profile with a fresh `llmctl require-validated vision` PASS under `LLMCTL_VALIDATE_STRICT=1`
  (11.4.277), plus a web vision bank driven through a CDP browser (`--browser-url`/`playwright:`
  steps; the existing `banks/full-qa-web.yaml` is prose with localhost:3000 and is not runnable
  against stack 3 as is). Without it the leg is reported SKIP, never faked.
* Host capacity (`llmctl capacity --json`, 2026-11-05): every vision profile `does_not_fit`
  ("no CPU left in the 12-CPU model budget", an `llmctl-onnx@decide-nli` unit already running);
  starting one needs the operator to free CPU or accept a start. Never `enable`.
