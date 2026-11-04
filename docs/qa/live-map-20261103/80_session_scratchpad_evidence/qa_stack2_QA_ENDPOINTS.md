# Catalogizer QA stack 2 (main @ f28f8165, clean clone; rootless podman, project `catqa2`)
Built commit: f28f816581b10f2a4398979fa437ed4b8b67be03 (clean checkout in /home/milosvasic/.cache/claude-tmp/qa2root.ZRr5Wu/src; submodules copied from live repo at the recorded pins). Independent of project `catqa` (38080/33000/33001), which was not touched.
Host: anton, LAN 192.168.1.115. Firewall: ufw ENABLED=no, firewalld/nftables inactive; nothing filters LAN (nft ruleset itself UNVERIFIED without root).
Credentials: <session-scratchpad>/qa_stack2/qa.env (0600): ADMIN_USERNAME / ADMIN_PASSWORD / JWT_SECRET / POSTGRES_PASSWORD (fresh random; values not recorded here).

| Service | Bind | URL | Health |
|---|---|---|---|
| catalog-api | 0.0.0.0:38081 | http://192.168.1.115:38081 | /health 200, /health/deep 200 |
| catalog-web (nginx SPA, /api + /ws proxy via API_UPSTREAM=api:8080) | 0.0.0.0:33010 | http://192.168.1.115:33010 | / 200, /api/v1/health 200 |
| Website (VitePress static, rebuilt from main) | 0.0.0.0:33011 | http://192.168.1.115:33011 | / 200 |
| PostgreSQL 15 | 127.0.0.1:35433 | db catalogizer | healthy |
| Redis 7 (no auth) | 127.0.0.1:36380 | - | healthy |

Images: localhost/catqa2-api:main, localhost/catqa2-web:main, localhost/catqa2-website:main (nginx:alpine retag). Digests: evidence/final/images.txt.
Media fixtures: ./media (mp4, mp3, txt) mounted /media; storage root "qa-local-media" created by the smoke journey.
Scripts: qa_stack_start.sh, qa_stack_stop.sh [--purge], smoke_journey.sh, smoke_extra.sh. Evidence: evidence/smoke_*/, evidence/extra_*/, evidence/final/.
NOTE start order: podman-compose does not honour depends_on health, so the first API start can lose a race with Postgres (exit 1); re-run `podman start catqa2-api catqa2-web`.

NOT booted: MinIO (STORAGE_TYPE=local; upstream image unobtainable), Android/AndroidTV, Tauri desktop/installer wizard, HelixQA services.

## Known gaps observed on this build
1. Conversion still does not execute: job 1 stays `pending` (checked after ~10 s and ~90 s); no worker caller of ProcessJobQueue in main, no ffmpeg in the runtime image. Retry endpoint returns 500.
2. GET /conversion/jobs (no filter) returns `null` though a job exists; ?status=pending returns it.
3. Invalid conversion_type returns 500 (not 400).
4. No endpoint in main returns 501 (no StatusNotImplemented in catalog-api); probes of unknown paths return 404.
5. Postgres boot migration noise as in stack 1: UNVERIFIED on this build (logs not audited).
