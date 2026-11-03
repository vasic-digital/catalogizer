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
