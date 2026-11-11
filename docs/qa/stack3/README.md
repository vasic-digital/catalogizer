# Catalogizer QA stack 3

Rootless-podman QA stack built from a clean export of a `main` commit (default: the repo HEAD). Independent of stacks
1 (`catqa`, 38080/33000/33001) and 2 (`catqa2`, 38081/33010/33011); it touches no other project's containers.

| Service | Bind | Notes |
|---|---|---|
| catalog-api | 0.0.0.0:38082 -> 8080 | `/health` returns `version`/`build_number` = the full commit sha baked in by `build.sh` |
| catalog-web (nginx SPA, `/api` + `/ws` proxy to `127.0.0.1:8080`) | 0.0.0.0:33020 -> 3000 | |
| Website (VitePress static, nginx) | 0.0.0.0:33021 -> 80 | content from `$QA_DIR/website_dist` |
| PostgreSQL 15 | 127.0.0.1:35434 | loopback only |
| Redis 7 | 127.0.0.1:36381 | loopback only |

## Intentional LAN-QA settings
The API pod publishes `0.0.0.0` (LAN-reachable, no TLS) and runs with `APP_ENV=development`, `CORS_ENABLED=true`, `CORS_ORIGINS='*'`.
This is deliberate so QA devices and browsers on the LAN can reach the stack; it is NOT a production configuration and the stack must
not be exposed beyond a trusted LAN. Postgres and Redis stay on loopback. Secrets are passed to containers by variable name
(`-e NAME`), never as `-e NAME=value`, so no secret appears in `podman` argv or `ps`. The VALUES are still part of the container's configuration: the host user can read them with `podman inspect catqa3-api` (`.Config.Env`) and from `/proc/<pid>/environ`, including `METRICS_SCRAPE_TOKEN` (accepted risk for a single-tenant QA host, tracked as `QA-MON-FU-3`).
`TRUSTED_PROXIES` defaults to the host primary address /32 (see `lib.sh`): any client on the QA host can forge `X-Forwarded-For`; remote-LAN source preservation is UNCONFIRMED (`QA-MON-FU-4`).

## Script reference (11.4.18)
| Script | Purpose | Exit codes |
|---|---|---|
| `build.sh [commit]` | clean-export build of api/web images (+ website unless `WEBSITE=0`); writes `$QA_DIR/BUILT_COMMIT` | 0 ok, 2 missing gitlink |
| `up.sh [tag]` | replace this stack's pod, health-gated start, final probes | 0 all probes 200; 3 image/corpus missing; 4 low memory; 5 port held by a foreign owner (pod left untouched); 6 stack not healthy (a probe not 200 or web wait timed out) |
| `down.sh [--purge]` | remove this stack's pod (and volumes with `--purge`) | 0 |
| `test_scripts.sh` | sandbox tests with stub `podman/curl/systemd-run/sleep/ss`; touches no real container, port or data | 0 all pass |

## Use
```
docs/qa/stack3/build.sh [commit]   # images localhost/catqa3-{api,web}:main-<short8> + website_dist (WEBSITE=0 to skip)
docs/qa/stack3/up.sh [tag]         # idempotent; replaces its own pod, keeps volumes; prints curl status lines
docs/qa/stack3/down.sh [--purge]   # removes only pod catqa3 (+ its volumes with --purge)
```
Overrides (env): `STACK`, `QA_DIR` (default `~/.cache/claude-tmp/catqa3`), `MEDIA_DIR` (default `~/.cache/claude-tmp/media-corpus`),
`API_PORT WEB_PORT WEBSITE_PORT PG_PORT REDIS_PORT`, `MEDIA_MOUNT`.

* Secrets: `up.sh` generates `$QA_DIR/qa.env` (mode 0600, outside the repo) on first run - fresh random `JWT_SECRET`,
  Postgres password, admin password; user `qa_admin`. Nothing is printed. `REGISTRATION_MODE=approval`.
* Admin seeding: the production path - `ADMIN_USERNAME`/`ADMIN_PASSWORD` env, the API creates the admin on first boot.
* Storage: `STORAGE_TYPE=local`. ffmpeg + imagemagick come from the stock `catalog-api/Dockerfile` runtime stage.
* Media corpus: generate with `scripts/test-infra/gen_media_fixtures.sh --out "$MEDIA_DIR"`; it is mounted **read-only at `/media`**
  in the API container (`MEDIA_ROOT_PATH=/media`). Create a storage root of type local with path `/media` (or a sub-directory such as `/media/movies`).
* Local roots: `CATALOGIZER_LOCAL_ROOT_BASES=$MEDIA_MOUNT` (colon-separated absolute dirs; unset = deny every local root, see `catalog-api/filesystem/local_policy.go`). Roots must lie under `/media` after symlink resolution.
* Read roots: `CATALOGIZER_READ_ROOTS=$MEDIA_MOUNT:./data` is an explicit pin of what `pathguard.ReadRoots()` already defaults to when `MEDIA_ROOT_PATH` is set (`MEDIA_ROOT_PATH:./data`); it is NOT what makes the corpus readable (an earlier commit note, 9a15580d, overstated that). Conversion output stays confined to the default write root `./data` (`/app/data`).
* The only edit applied to the exported tree is the `go build -ldflags` line of `catalog-api/Dockerfile` (adds `-X main.Version=<sha>`);
  the repo itself is not modified.

## Why a pod on `pasta` (the DNS / netns problem)
Observed on this host (podman 5.7.0, netavark + aardvark-dns, rootless): the rootless network namespace was re-created
(`/run/user/1000/containers/networks/rootless-netns`, 2026-11-03 22:51) while the single `aardvark-dns` process (pid 6042,
started 2026-11-02 10:48 inside the OLD namespace) kept running. Containers created afterwards live in the new namespace, but the only DNS
server is bound inside the old one: `lookup postgres on 10.89.x.1:53: i/o timeout`, nginx `host not found in upstream "api"`; and the
`rootlessport` forwarders connect into the wrong namespace (connection resets on published ports). The previous workaround pinned container
IPs with `extra_hosts` (IPs change on each recreate).

Durable fix for this project, with **no change to anything else**: each stack is ONE POD with `--network pasta`. Containers in a pod share
one loopback, so services reach each other on `127.0.0.1` (no names, no DNS, no bridge, no aardvark, no rootless-netns); ports are
published by pasta in the pod's own namespace. Verified 2026-11-03: published ports answer, egress/DNS works (pasta forwards the host resolver).

Repairing the shared netavark state for every other bridge-network project (kill the stale `aardvark-dns`, let netavark respawn it in the
current namespace, or restart all rootless containers) would disturb other projects' containers and is **a proposal for the operator**, not done.

## Start-order race (API vs Postgres)
Postgres/Redis/API carry podman healthchecks and `up.sh` gates each start on `healthy` (Postgres+Redis -> API -> web/website). The API
also has `--restart on-failure:3`. (`podman-compose` ignores `depends_on: condition: service_healthy`, which is why this is a script.)

## Documented deviation from constitution 11.4.173 (containerized + distributed build)
11.4.173 requires every build to run in a build container provisioned through the `digital.vasic.containers` submodule and distributed
to a remote build host, with artifacts brought back. `build.sh` does NOT do that: it runs `podman build --network host` locally on the
QA host (under a `systemd-run` memory cap), and `--network host` also weakens build hermeticity (the build sees the host network).
This is a known, accepted-for-now deviation for the QA stack, not a claim of compliance. Tracked follow-up: move the stack-3 image and
website builds onto the containers-submodule build path (remote build host, no host networking, pinned inputs) and retire this script's
local build; until that lands these images are QA artifacts only and must not be promoted as release artifacts.

## Container lifecycle in agent shells (CAT-QA-STACK-LIFECYCLE-1)
**Root cause (measured 2026-11-11):** agent/harness shells export `DBUS_SESSION_BUS_ADDRESS='unix:path=/run/user/1000/bus,guid=<hex>'`. crun's
sd-bus call then fails (`sd-bus call: Operation not permitted: Permission denied: OCI permission denied`) for ANY `podman run/start`, even a pod-less
alpine container. The same address without `,guid=...` works. Live control: `evidence-lifecycle-20261111/probe_containers_backend.txt` (raw
`podman start` rc=125 with the guid address; wrapper rc=0).

**Fix:** `docs/qa/lib/podman_env.sh` (sourced by `lib.sh`, `monitoring/lib.sh`, `sonarqube/lib.sh`): `qa_podman_env_normalize` strips every
`,key=val` suffix from the address and defaults `XDG_RUNTIME_DIR`; `qa_stack_start <c>` / `qa_stack_stop <c>` normalise, run the lifecycle op, and
(start) wait for health with a bounded loop (`QA_HEALTH_TIMEOUT`, default 120 s). They only address the one named container, which must start with
`${QA_STACK_PREFIX:-${STACK}-}` (hyphen included, so catqa1/catqa2/cattsrv-* are refused with rc 64); nothing prunes or acts pod-wide.
Usage: `. docs/qa/lib/podman_env.sh; qa_stack_start catqa3-api`.

**Backend / deviation status:** when `go` and `submodules/containers` are present, start/stop go through the Containers submodule's runtime abstraction
(`digital.vasic.containers/pkg/runtime` `PodmanRuntime`, via `docs/qa/lib/ctrctl`, a tiny Go CLI built into `~/.cache/catalogizer-qa/ctrctl`). Otherwise
(or `QA_STACK_BACKEND=podman`) rootless podman is called directly and a `DEVIATION 11.4.76/11.4.173` line is logged. `up.sh`/`down.sh`/`build.sh`
still call podman directly (pod create, run, build) - the submodule has no pod/run-spec API that fits them today; see
`docs/CONTAINERS_SUBMODULE_PROPOSAL.md`. Tracked follow-up: full submodule-driven stack lifecycle (CAT-QA-STACK-LIFECYCLE-1 part 2). The `go build`
of `ctrctl` itself runs on the host, not in a build container (same class of 11.4.173 deviation as `build.sh`).

**Tests:** `docs/qa/lib/test_podman_env.sh` (gate-logic unit test with a fake podman shim that fails on `,guid=`; not proof that containers start) and
`test_podman_env_mutations.sh` (6 mutants: no normalisation, first-param-only, no health wait, no name check start/stop, loose prefix; all killed).
Live proof: `lifecycle_probe.sh` -> `evidence-lifecycle-20261111/`.
