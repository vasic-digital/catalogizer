# HelixQA against QA stack 3

Roadmap-Item: HELIXQA-STACK3

`run.sh` drives HelixQA (`submodules/helix_qa`) against live stack 3 (API :38082, web :33080/:33020).

## Credentials (11.4.10)
Only `ADMIN_USERNAME` / `ADMIN_PASSWORD` are parsed (KEY= lines, file never sourced) from `QA_ENV`
(default `/home/milosvasic/.cache/claude-tmp/catqa3/qa.env`; host-specific default, override it).
`helixqa http` has no env/file/stdin credential input (only `--admin-pass` argv), so `run.sh` logs in itself with
`curl -K -` (config on stdin, nothing secret in argv) and hands helixqa the bearer via `auth: raw:` in a
0600 temp copy of the bank (built with shell builtins, deleted on exit). The bearer sits on disk in that temp
file for the length of the run (seconds). Upstream follow-up (not done here, submodule untouched): a
`--admin-pass-file` / env option in `cmd/helixqa/http.go`.

## Exit map (closed)
0 all PASS incl. UI vision | 1 FAIL | 2 PARTIAL (API green, UI vision unverified) | 3 SKIP (binary or credentials absent)
| 4 tool/usage error (e.g. no JSON report) | 5 precheck BLOCKED (API/web unreachable, login failed).
**Exit 0 is unreachable today**: the UI leg never reaches PASS (no web vision bank exists), so a fully green API run exits 2.

## Evidence (per run, `evidence/<ts>/`, gitignored)
`verdict.txt` (timestamp, helixqa sha256, helix_qa commit at build time from the stamp, target build_number/version from /health, answering API
container + image tag, `ui_vision:` line, `overall:` line), `health.json`, `http_api.txt`, `http_api.json` (valid JSON, strictly
checked: expected case ids, total==passed==3, skipped/failed/errored==0), `http_api.err`, `run.log`.
Committed scrubbed pack: `evidence-pack-20261105/` (baseline + mutations M1 401->200, M3 action typo,
M5 fake binary exit 0, M6 fake binary exit 2, M4 missing credential; see SUMMARY.txt). The image digest is not
recorded (only the image tag, which embeds the commit); UNCONFIRMED beyond that.

## Build the binary (11.4.173: in a container)
From the repo root (needs the full `submodules/` tree; `OUT`, `GOC`, `GOM` are empty writable dirs):

    podman run --rm --memory 3g --cpus 3 -v "$PWD/submodules:/src:ro" -v "$OUT:/out" -v "$GOC:/gocache" -v "$GOM:/gomod" \
      -e GOCACHE=/gocache -e GOMODCACHE=/gomod -e GOMAXPROCS=3 -w /src/helix_qa \
      docker.io/library/golang:1.26 go build -o /out/helixqa ./cmd/helixqa
    git -C submodules/helix_qa rev-parse HEAD > "$OUT/helixqa.stamp"   # build stamp read by run.sh (F4)

Do not set `GOFLAGS=-mod=mod` or `GOWORK=off`. Put the binary and its `.stamp` at `bin/helixqa` / `bin/helixqa.stamp` (gitignored, never committed, 11.4.30) or set `HELIXQA_BIN`. `run.sh` logs out (`/auth/logout`) at exit and handles TERM/INT (kills only its recorded child pid).

## UI / Vision leg (11.4.277, 11.4.278) - NOT done
Needs `HELIX_VISION_ENDPOINT` backed by a validated llmctl vision profile
(`LLMCTL_VALIDATE_STRICT=1 llmctl require-validated vision`, which `run.sh` calls) and a web vision bank driven through a
CDP browser; none exists for stack 3 and `banks/full-qa-web.yaml` targets localhost:3000. Check capacity at run time with the
read-only `llmctl capacity --json` (verdict per profile); on 2026-11-05 every vision profile was `does_not_fit` (CPU budget in use).
Start/stop only with `llmctl start/stop`, never `enable`.
