# Catalogizer QA stack (main @ 96b7b5ec, scratch clone; rootless podman, project `catqa`)
Host: anton, LAN 192.168.1.115 (enp5s0); also ppp0 10.1.106.107. Firewall: ufw ENABLED=no (/etc/ufw/ufw.conf), firewalld/nftables not running -> nothing filters LAN traffic (checked without root; `nft list ruleset` needs root = UNVERIFIED).
Credentials: ./qa.env (0600): ADMIN_USERNAME/ADMIN_PASSWORD/JWT_SECRET/POSTGRES_PASSWORD (random).

| Service | Proto | Bind | URL | Health |
|---|---|---|---|---|
| catalog-api | HTTP | 0.0.0.0:38080 | http://192.168.1.115:38080 (also http://anton:38080 if resolvable) | /health 200, /health/deep 200 |
| catalog-web (nginx SPA + /api,/ws proxy to api) | HTTP | 0.0.0.0:33000 | http://192.168.1.115:33000 | / 200, /api/v1/health 200 |
| Website / docs (VitePress static) | HTTP | 0.0.0.0:33001 | http://192.168.1.115:33001 | / 200 |
| PostgreSQL 15 | TCP | 127.0.0.1:35432 | db catalogizer, user catalogizer | pg_isready healthy |
| Redis 7 (no auth) | TCP | 127.0.0.1:36379 | - | PING healthy |
DB/Redis are loopback-only on purpose (Redis has no password). Change the port mapping in qa-compose.yml to expose.
API HTTPS/HTTP3 listens on 28443 inside the container, not published.
Media fixtures: ./media (mp4, mp3, txt) mounted at /media; storage root "qa-local-media" (protocol local, /media).

NOT booted: MinIO (image unobtainable, see defects), Android/AndroidTV (no prebuilt APK in tree; Gradle build not run), Tauri desktop + installer-wizard (native apps, not built), catalogizer-api-client (library, no server), HelixQA/challenge services, Synology share (not configured: read-only SMB source not wired).

Scripts: qa_stack_start.sh (idempotent, refuses if <6 GiB free), qa_stack_stop.sh [--purge], smoke_journey.sh (evidence -> evidence/smoke_*/journey.log).
Rebuild images: cd $(cat ROOT_PATH)/src && podman build --network host -f catalog-api/Dockerfile -t localhost/catqa-api:main . (same for catalog-web/Dockerfile -> catqa-web:main). Website: re-run vitepress build into ./website_dist.

Firewall commands IF a firewall is later enabled (not run): sudo ufw allow from 192.168.1.0/24 to any port 38080,33000,33001 proto tcp

## Defects
1. MinIO: quay.io/minio/minio:latest and pinned RELEASE tags (and docker.io/minio/minio) return unauthorized/denied - upstream stopped public images. docker-compose*.yml all reference it -> `compose up` fails for the official stack. Workaround: STORAGE_TYPE=local (API ignores S3 when type != s3). Not substituted with third-party images (declined by policy).
2. Conversion never executes: POST /conversion/jobs returns 200, job stays `pending` forever. Root cause: ConversionService.ProcessJobQueue has no non-test caller (no worker); POST /jobs/:id/retry -> 500. Also runtime image lacks ffmpeg. 
3. GET /conversion/jobs without ?status returns `null` although jobs exist (handler passes pointer to empty string as status filter); with ?status=pending returns the job.
4. Invalid conversion request (wrong conversion_type) returns 500 "Failed to create conversion job" instead of 400 (validation failure masked).
5. Postgres boot logs 43 migration errors (ALTER ... ADD COLUMN already exists; type "datetime" does not exist; relation system_configuration missing) - API still boots; schema completeness UNVERIFIED.
6. catalog-web/nginx.conf default proxies to host.containers.internal:8080; QA uses an override conf (nginx-web.conf) pointing to api:8080.
