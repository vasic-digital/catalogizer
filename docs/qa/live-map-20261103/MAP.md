# Catalogizer live map — measured 2026-11-03 (host `anton`, LAN 192.168.1.115)

Every row below is backed by a file in this directory (machine output, no narration). Credentials are named, never valued:
`qa.env` files (0600, outside the repo) hold `ADMIN_USERNAME`, `ADMIN_PASSWORD`, `JWT_SECRET`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`.
Stack 1 env: `<scratchpad>/qa_stack/qa.env`; stack 2 env: `<scratchpad>/qa_stack2/qa.env`
(`<scratchpad>` = `/home/milosvasic/.cache/claude-tmp/claude-1000/-home-milosvasic-Projects-catalogizer/261db589-353c-4105-9405-009dbfe0bd33/scratchpad`).

## 1. Why the ports looked dead (root cause, evidence 01-09)

| Fact | Evidence |
|---|---|
| All 10 `catqa*` containers (and every other rootless container on the host) were reported `Exited (0) ... 292 years ago` by `podman ps -a`, and the pods `Exited`, yet the real processes (`/app/catalogizer-api`, nginx, postgres, conmon) were alive for 9-23 h. | `01_before_podman.txt`, `06_real_state.txt` |
| Every one of the 6 app ports had a live `rootlessport` listener but plain GET got `curl rc=56` (reset) and TLS `rc=35` on both http and https. Not TLS: the container API itself answered `200` on `127.0.0.1:8080/health` from inside its netns. | `02_before_ports.txt`, `07_netns.txt` |
| Podman's state DB was reset: `/run/user/1000/libpod/tmp/alive.lck` was re-created at 15:12 today (podman treats this as a reboot and marks all containers exited without killing them). Afterwards the rootless-netns (pasta) that the `rootlessport` forwarders connect to was a different namespace than the one holding the bridges/containers, so forwarded connections were reset. | `09_netns_identity.txt` (alive.lck 15:12:22), `08_rootless_netns.txt` |
| A `podman-compose up` started 23 h earlier was still hung (pid 818992) and my own first re-run of `qa_stack_start.sh` hung too (compose blocks, no output). | `05_restart_stack1.txt` (empty), ps listing in session |
| After tearing down and recreating containers, the API crash-looped with `lookup postgres on 10.89.13.1:53: i/o timeout` (and nginx `host not found in upstream "api"`): the single old `aardvark-dns` (pid 6042) serves only the OLD netns, so the re-created netns has no DNS. | `12_stack1_fail_logs.txt` |

Other stacks on this host (lava-*, pod_mcp, pod_helix_agent, helixcode-autoboot-*, boba, ...) have the same desynced podman state; I did NOT touch them (out of authorization). They are probably running but unreachable from the host and shown as Exited.

## 2. What I did (exact commands; evidence 10-15)

1. Stopped only our `catqa`/`catqa2` container processes (children of conmon whose `-n` name matched `catqa-*`/`catqa2-*`; named volumes untouched, so DB data persisted — smoke test later gets `409` on "storage root exists"): `10_teardown_stale.txt`.
2. Staged restart with `restart_stack.sh` (this dir): Postgres+Redis first, wait `pg_isready` + `PONG`, then API with `extra_hosts postgres/redis` pinned to the live container IPs (works around the dead aardvark-dns), wait `/health` 200, then web with `extra_hosts api:<ip>`, then website; finally `podman start` of anything left `Created` (compose double-start leaves api/web in `Created` with `rootlessport ... address already in use`).
   ```
   <this dir>/restart_stack.sh <scratchpad>/qa_stack  catqa  38080     # log 13/14
   <this dir>/restart_stack.sh <scratchpad>/qa_stack2 catqa2 38081     # log 15
   ```
   This fixes the "API before Postgres is healthy" race (the repo compose `depends_on: service_healthy` is not honoured when podman-compose is run with `--no-deps`/in the desynced state).
3. Re-ran the repo's own `smoke_journey.sh` for both stacks: `40_smoke_qa_stack.log`, `40_smoke_qa_stack2.log`.

## 3. Live status table

Measured `2026-11-03T23:06+01:00` (full matrix with localhost AND 192.168.1.115, http AND https: `30_after_matrix.txt`; containers/digests: `31_after_containers.txt`; listeners: `32_after_listeners.txt`). https returns `000` everywhere (no TLS on these ports; API's TLS is on 28443 inside the container, unpublished). Everything published is bound `0.0.0.0` (LAN reachable; ufw disabled, nft ruleset unverified without root) except DB/Redis (`127.0.0.1` only).

| Component | Kind | URL (LAN / localhost) | Auth | How to start | LIVE now | Existing LIVE evidence |
|---|---|---|---|---|---|---|
| catalog-api (stack 1, image `localhost/catqa-api:main` sha256:c2282d1d..) | API (Go/Gin) | http://192.168.1.115:38080 , http://127.0.0.1:38080 | `/health`,`/health/deep`,`/api/v1/auth/init-status`: none; rest JWT via `POST /api/v1/auth/login` (`ADMIN_*` in qa.env) | `restart_stack.sh <scratchpad>/qa_stack catqa 38080` | `/health` 200, `/api/v1/health` 200, `/` 404 (no root route) — 23:06 | `40_smoke_qa_stack.log` (22 steps, 20x 2xx; 401 for bad pw is expected; 409 = root already exists); older: scratchpad `qa_stack/evidence/smoke_20261102T*` |
| catalog-web (stack 1, nginx SPA + /api,/ws proxy, `catqa-web:main`) | web UI | http://192.168.1.115:33000 , http://127.0.0.1:33000 | UI login = API user; `/` public | same script | `/` 200, `/health` 200, `/api/v1/health` 200 | `40_smoke_qa_stack.log` steps 03,04,09 (HTTP only, no browser); Playwright suites exist in `catalog-web/e2e/*.spec.ts` (`real-backend-pages.spec.ts`) — no run in this dir |
| Website / docs (stack 1, nginx:alpine + VitePress dist) | static site | http://192.168.1.115:33001 | none | same script | `/` 200 | `30_after_matrix.txt` only |
| catalog-api (stack 2, `localhost/catqa2-api:main` sha256:4dfc736b..) | API | http://192.168.1.115:38081 | as above | `restart_stack.sh <scratchpad>/qa_stack2 catqa2 38081` | `/health` 200 | `40_smoke_qa_stack2.log` (20x 2xx); scratchpad `qa_stack2/evidence/{smoke_20261103T133400,extra_*,final}` |
| catalog-web (stack 2, `catqa2-web:main`) | web UI | http://192.168.1.115:33010 | as above | same | `/` 200, `/api/v1/health` 200 | `40_smoke_qa_stack2.log` |
| Website (stack 2, `catqa2-website:main`) | static site | http://192.168.1.115:33011 | none | same | `/` 200 | `30_after_matrix.txt` only |
| PostgreSQL 15 (x2: `catqa-postgres` 127.0.0.1:35432, `catqa2-postgres` 127.0.0.1:35433) | DB | loopback only — NOT LAN | `POSTGRES_*` | in the same scripts | healthy (podman healthcheck) | `31_after_containers.txt` |
| Redis 7 (x2: 127.0.0.1:36379, :36380, no auth) | cache | loopback only — NOT LAN | none | same | healthy | `31_after_containers.txt` |
| Reviewer-owned host process `catalog-api.bin` (pid 2600206, from `claude-tmp/resp.jA8VTf/`) | API (not ours) | 127.0.0.1:28443 tcp+udp | n/a | n/a (another session's scope) | listening | `32_after_listeners.txt` (not probed) |
| Stray `helixcode` on `*:8080` (pid 6973) | unrelated project | LAN :8080 | n/a | n/a | listening — NOT Catalogizer | `32_after_listeners.txt` |
| catalogizer-desktop (Tauri 2.4.0, `com.catalogizer.desktop`) | desktop client | n/a | login to API | `cd catalogizer-desktop && npm run tauri dev` (needs display + Rust) | NOT BUILT: no `src-tauri/target` | NONE live (compose `docker-compose.test.yml` profile `desktop` exists) |
| installer-wizard (Tauri 2.4.0, `com.catalogizer.installer-wizard`) | desktop client | n/a | n/a | `cd installer-wizard && npm run tauri dev` | NOT BUILT | NONE live (`docker-compose.test.yml` profile `wizard`) |
| catalogizer-android (`com.catalogizer.android`, 2.4.0) | Android app | n/a | API login | `catalogizer-android/gradlew assembleDebug` (containerized per §11.4.173) | NOT BUILT: no `app/build/outputs`; `adb devices` empty | older: `docs/qa/device_qa_androidtv_20260626.md`, `docs/qa/androidtv-players-20260629/` (Android TV only) |
| catalogizer-androidtv (`com.catalogizer.androidtv`, 2.4.0) | Android TV app | n/a | API login | `catalogizer-androidtv/gradlew assembleDebug` | NOT BUILT; no device attached | `docs/qa/helixqa-androidtv-20260629/qa-report.md`, `docs/qa/androidtv-players-20260629/Status.md` |
| catalogizer-api-client (TS lib) | library, no server | n/a | n/a | `npm run build` | n/a | unit tests only |
| Prometheus (compose profile `monitoring`, host :9090) / Grafana (:3001) | dashboard | n/a | Grafana admin | `docker-compose.yml --profile monitoring` (configs in `monitoring/`) | DOWN: never started on this host (no listener on 9090/3001, `32_after_listeners.txt`) | NONE |
| MinIO (S3 alternative, compose :9000/9001; dev :29000/29001) | object store | n/a | MinIO keys | compose `minio` service | DOWN: image `quay.io/minio/minio` unobtainable (documented defect 1 in the stack's `QA_ENDPOINTS.md`); QA stacks use `STORAGE_TYPE=local` | NONE |
| nginx production proxy / transcoder / backup (`deployment/docker-compose.yml`, profile `production`/`backup`) | service | :80/:443 | n/a | `deployment/docker-compose.yml` | DOWN, not started | NONE |
| pgAdmin (:25050) / redis-commander (:28081) (`docker-compose.dev.yml` profile `tools`) | dashboard | n/a | pgAdmin login | profile `tools` | DOWN | NONE |
| HelixQA (`docker-compose.qa.yml` helixqa-web/api, `docker-compose.qa-robot.yml`, `submodules/helix_qa`; banks in `challenges/helixqa-banks`) | QA services | no ports | n/a | profiles `web`/`api` | DOWN (job containers, no listeners) | `docs/qa/helixqa-androidtv-20260629/` |
| Test-infra (`docker-compose.test-infra*.yml`: FTP/SMB/WebDAV/NFS/SFTP/FTPS fixtures on 127.0.0.1 `TI_PORT_*`) | fixtures | loopback | n/a | `docker-compose.test-infra.yml` | DOWN | `docs/qa/webdav-yandex-live-20261103/` (live Yandex WebDAV) |
| Security stack (SonarQube :9000, trivy, semgrep, ... `docker-compose.security.yml`) | scanners | :9000 | SonarQube | profiles | DOWN | NONE here |
| OCU-CUDA-Sidecar (Go, Dockerfile) | sidecar service | n/a | n/a | build from `OCU-CUDA-Sidecar/Dockerfile` | not run | NONE |
| catalog-web dev server (Vite :3000, proxies to `.service-port`) | dev UI | :3000 | n/a | `cd catalog-web && npm run dev` | DOWN | NONE |

Component inventory source: `20_compose_services.txt`, `21_components_inventory.txt`, `50_other_components.txt`.

## 4. Honest gaps (UNCONFIRMED / UNKNOWN)

- The two stacks run images built from earlier clones (`catqa` from main @ 96b7b5ec, `catqa2` from main @ f28f8165, per their `QA_ENDPOINTS.md`), NOT the current `main` (e1695dfd); image digests are in `31_after_containers.txt`. Whether the behaviour under test matches the current source is UNKNOWN until rebuilt.
- Root trigger of the 15:12 podman state reset (what removed `/run/user/1000/libpod`) is UNKNOWN; only the effect is evidenced. Whether the rootlessport resets were caused by that reset, by my 22:51 `podman` call re-creating `rootless-netns`, or both, is UNCONFIRMED (they were already resetting before I acted, per `02_before_ports.txt`, taken before any change except read-only `podman ps`).
- Present podman state is still split: two rootless netns exist (old pasta pid 7638 holds other stacks; the new one holds catqa/catqa2). The other stacks' host ports are probably still dead. Durable fix = stop all rootless containers and `systemctl --user`/reboot-free restart of podman (operator decision, not done).
- `restart_stack.sh` pins hostnames via `extra_hosts` to container IPs: IPs change on every recreate; it is a workaround, not a repair of aardvark-dns. Compose's own `depends_on` health gating is not exercised.
- No browser (Playwright/HelixQA) run was done in this task; "LIVE" for web UIs means HTTP 200 only. Firewall: ufw off, but `nft list ruleset` needs root = UNVERIFIED; LAN reachability was measured only from the host itself via its own LAN IP, not from a second machine.
- The older evidence mentioned (scratchpad `qa_stack*/evidence`) lives outside the repo and is not copied here.
