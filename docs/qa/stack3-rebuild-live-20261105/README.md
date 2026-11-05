# Stack 3 rebuild + live re-verification (2026-11-05)

Base: main d9238b4743c85dbb9c987617bb4409041f41e840. Operator: Sonnet QA agent. No product code touched; evidence only.
Stack 3 (podman pod catqa3, pasta) torn down with docs/qa/stack3/down.sh (volumes kept) and rebuilt with build.sh (WEBSITE=0, website_dist reused) + up.sh main-d9238b47.
Build: 2026-11-05T02:27:39+01:00 .. 02:29:40+01:00 (about 2 min, layer cache warm). `/health` build_number = full sha above. All 5 containers healthy.

| Image | Digest / id |
|---|---|
| localhost/catqa3-api:main-d9238b47 | sha256:3ad0e11871603a62b2987e677b398053d46d4fead605f04747a8727ab1c956e5 (id fba8be9a6c45) |
| localhost/catqa3-web:main-d9238b47 | sha256:89e07bcf369604ef91ceb6aa2320d914a78881965779f0cedc599ad4cfdef07a (id bb1affdd75e1, identical to the 4c79b7e3 web image) |
| nginx:alpine (website) | sha256:a3554c0d... ; postgres:15-alpine sha256:f7d23353...; redis:7-alpine sha256:858f009f... |

## URL / port map
| Service | Bind | URL |
|---|---|---|
| catalog-api | 0.0.0.0:38082 | http://192.168.1.115:38082 (health: /health) |
| catalog-web | 0.0.0.0:33020 | http://192.168.1.115:33020 |
| Website | 0.0.0.0:33021 | http://192.168.1.115:33021 |
| PostgreSQL 15 | 127.0.0.1:35434 | loopback only |
| Redis 7 | 127.0.0.1:36381 | loopback only |
Login user qa_admin, password in /home/milosvasic/.cache/claude-tmp/catqa3/qa.env (0600), not reproduced here.

## Method
Real api binary, real PostgreSQL (migrations v24-v26 applied live: conversion_jobs.warning, subtitle_tracks.uploaded_by, file_text_index present; pg_migrations.txt), real ffmpeg/ffprobe/poppler in the api image, local storage root `/media` (corpus + added fixtures). Harness h/live3.py (3 rounds, 55 checks each, control needles included). Raw: run_r1..3.txt, h/results.jsonl, results_table.md. Fixtures added then removed: live3/ (md note with QUOKKA-9091, txt NARWHAL-5521, generated 26 s Burst.Movie.2021.mp4 with irregular audio bursts + .en.srt aligned + .out-of-sync.srt = +2.0 s).

## Results (3 rounds each; 54 PASS, 1 SKIP, 0 FAIL per round, details per check in results_table.md)
| Item | Result |
|---|---|
| D1 /conversion/formats truthful (document: pdf->txt only); pdf->txt content has ZEBRA-4242 + all 3 page lines; pdf->png, txt->pdf, epub->txt refused 400 with reason | PASS x3 |
| D2 DELETE completed job (then GET 404, DELETE again 404, download 404); DELETE running job 409 | PASS x3 |
| D10 target ext mismatch 400; matching ext mp3 produced (ID3 magic) | PASS x3 |
| D11a truncated-tail.mp4 -> webm completes with `warning`; control good mp4 no warning | PASS x3 |
| D7 conversion error_message has no /app/data or /media path; subtitle bad-id 400 without err echo; translate no provider 503 | PASS x3 |
| D8 anonymous empty-JSON-body POST cancel/retry/logout -> 401 (control: non-empty malformed JSON still rejected) | PASS x3 |
| D4 search ZEBRA-4242 (pdf), OKAPI-1357 (txt), QUOKKA-9091 (md), NARWHAL-5521 (txt) each return exactly the one expected path; absent token 0; name search control | PASS x3 |
| D3 pdf-pages: total 3, pages 0-2 distinct PNG (stable sha), page 3 404, -1/x 400, malformed PDF 422 with no fitz/path leak, unauth 401 | PASS x3 |
| D11b sidecars imported idempotently (media 15: 4 rows source sidecar; media 201: 2 rows) | PASS x3 |
| D6 uploads (srt/vtt/ass/oos) 200 with real subtitle_id listed; unknown media 404; binary garbage 422 | PASS x3 |
| D5 verify-sync on generated bursty media 201: aligned sidecar offset ~0, +2.0 s sidecar ~+2000 ms, uploaded +3.5 s ~+3500 ms, all status `measured`; unknown sub/media 404 | PASS x3 |
| D5 on the corpus Movie.Name mp4 (media 15): `undetermined` (continuous tone, no activity variation) / out-of-sync `invalid` (cues past 5 s end), confidence 0, never the old constant 0.8 | PASS x3 (honest outcome, see note) |
| DELETE /subtitles/:id (unauth 401, 204, repeat 404, row gone) | PASS x3 |
| D9 qa_admin listed `admin`; PUT role admin applied (listed admin, can log in); unknown role 400; self-demotion 409; user cleanup | PASS x3 |
| Cross-user job access by a second admin: 200/200/409/200 (get/download/cancel/delete) = by design (view:media holders, FU-1) | PASS x3 (recorded) |
| Non-privileged user job isolation 404 | SKIP: no API path grants a plain user conversion permission (register -> 403) |
| Yandex WebDAV (real client, sandbox /Testing only, N=3 `-count=3`, 17 subtests each: auth ok/wrong password, mkdir, sha256 upload/download, range, copy/move, concurrent 4 uploads, unsized upload, timeout/cancel, delete, guard refuses outside sandbox) | PASS x3 (351-372 s each), yandex_evidence.json: guard_violations=2 = the 2 deliberate out-of-sandbox probes of subtest 17 (expected, refused before any byte left), 0 lost acked writes, 0 idle-cancel errors, 0 requests outside /Testing |
| Synology SMB + FTP live (read-only, `-tags live`, -count=3): SMB ListStatReadWalk, NameRoundTrip, NegativeControls; FTP ListStatReadWalk, NameRoundTrip, NegativeControls, PartialCloseAndDisconnectSemantics; Helpers | PASS x3 each |
| Synology NFS | SKIP x3 topology_unsupported (no exports) |
| Storage idle-deadline / cut-recovery on real Synology | NOT RUN: those scenarios (silent server, cut session, reconnect) exist only as loopback pyftpdlib / unit tests (needs python pyftpdlib), not as live-NAS tests; the live NAS suite above (passes) exercises the fixed clients on real SMB and FTP. No SMB/FTP writes to work20/Testing were made (no live test writes there). |

## Test-expectation corrections (not product defects)
First attempt of round 1 had 5 FAIL from wrong expectations of mine: (a) D5 on the corpus media expected `measured`, but the corpus audio is a continuous tone (ffmpeg silencedetect: 0 events) so `undetermined` is the documented design, and the out-of-sync cues (7.5-11.5 s) lie past the 5 s media so `invalid` is documented design; real measurement was proven on the generated bursty media instead; (b) cross-user check expected 404 for a second ADMIN, but the service lets view:media holders see other users' jobs. Harness updated; rounds 1-3 then ran on the final harness.

## Observations / residue
* The old verification text (follow-ups) says D5 "must show measured on the generated media": it only does on media with speech-like on/off audio; the stock corpus mp4 cannot be measured. Suggest adding a bursty fixture to gen_media_fixtures.sh (generated here: see method).
* Residue: files rows for removed fixtures (live3/*) remain in the stack-3 DB (re-scan does not delete them: after removal the rescan still lists QUOKKA-9091 and Burst.Movie rows); file_text_index has 5 rows (4 ok, negative/not-a-pdf.pdf failed as expected). Earlier residue (subtitle ids 1-12, jobs <=134) unchanged. Users created by the harness were deleted.
* PostgreSQL (not SQLite) is the stack DB, so D11-F4, D4-5, D9-F2 "UNCONFIRMED on PostgreSQL" are partly closed: migrations v24/v25/v26 applied and the D4/D9/D11 paths ran on PostgreSQL; the multi-process last-admin race on PG was not exercised.
* Temporary export, Go cache and credential env files used for the container-run Go live tests were deleted. Secret scan of this directory (passwords/tokens from the 0600 env files): 0 password hits; only non-secret words (usernames, db name, "Testing") match.
* Go live tests ran inside docker.io/library/golang:1.26 on --network host under systemd-run MemoryMax=5G (11.4.173 container; local host, not the remote build host).
