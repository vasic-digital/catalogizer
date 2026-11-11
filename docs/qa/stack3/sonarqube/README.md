# SonarQube QA service (on demand)

Static-analysis QA for `catalog-api` (Go) and `catalog-web` (TypeScript). It is **started for a scan window and stopped
afterwards**; nothing stays resident. Everything is plain shell + `curl` + `jq` + rootless `podman` - **no model/LLM calls,
zero token cost**. It builds on the images referenced by `docker-compose.security.yml` (SonarQube Community + PostgreSQL 15) but
runs them as an own rootless pod instead of compose.

| Script | Purpose |
|---|---|
| `up.sh` | create pod `catsonar` (own network namespace), start Postgres + SonarQube, wait for `UP`, replace the default admin password, mint a scan token. Idempotent. |
| `scan.sh` | run `sonar-scanner` in a container over both projects, wait for each compute-engine task and quality gate, export results, scrub secrets (control needle), write `SHA256SUMS`. |
| `down.sh [--purge]` | remove the pod (volumes = analysis history kept unless `--purge`). |
| `lib.sh` | shared settings/helpers. All settings are env-overridable (`SONAR_PORT`, `SONAR_MEM`, `SQ_DIR` ...). |
| `sonar-catalog-api.properties`, `sonar-catalog-web.properties` | per-project scanner properties (project keys `catalogizer-api`, `catalogizer-web`). |

## Use

```bash
docs/qa/stack3/sonarqube/up.sh                      # ~1-3 min (waits first if 1-min load > LOAD_MAX=20)
RUN_ID=run1 docs/qa/stack3/sonarqube/scan.sh        # [--fail-on-gate] -> docs/qa/sonarqube-<date>/run1/
docs/qa/stack3/sonarqube/down.sh
```

UI: `http://<LAN-IP>:19000` (LAN address auto-detected; override with `SONAR_BIND` or `QA_LAN_IP`, see `docs/qa/lib/lan_ip.sh`) (login `admin`; generated password and scan token are in
`~/.cache/claude-tmp/catsonar/sonar.env`, mode 0600, **outside the repo**; never printed, never committed). Credentials are passed
to curl via a 0600 netrc and to the scanner via a 0600 env-file, never in argv.

## Isolation and caps

* Rootless podman only; no sudo. Own pod `catsonar` -> own network namespace; the UI port is published on **one** LAN address
  (`<LAN-IP>:19000`, free high port) only. No custom podman network is created, so the shared aardvark-dns and the
  `catqa*` stacks are not touched.
* `--memory/--memory-swap/--cpus`: SonarQube 4g (swap = memory, i.e. none) / 2 cpus (JVM heaps: web 768m, CE 1g, ES 512m), Postgres 512m / 1 cpu, scanner 6g / 2 cpus (JVM -Xmx5g; the Go analyzer OOMed at -Xmx1g and -Xmx3g on catalog-api: measured).
* The caps above are the **only** real memory/CPU limits on the containers (podman cgroups). Wrapping the scripts in
  `systemd-run --user --scope -p MemoryMax=...` caps just the script processes (curl/jq/podman client), **not** the containers, which run
  under podman's own cgroups; this README no longer claims otherwise.
* Load gate: `up.sh`/`scan.sh` wait while the 1-min load is above `LOAD_MAX`, which defaults to 0.75 x `nproc` (12 on this 16-core host)
  and gives up after `SQ_LOAD_TIMEOUT` seconds (default 900). Deviation recorded: the first (pre-review) runs used `LOAD_MAX=30`
  because host load sat at 21-40; the re-runs after review used the derived default.
* Safety guards: `SQ_STACK` must match `^catsonar[a-z0-9-]*$`; pod, containers and volumes carry the label
  `io.catalogizer.service=sonarqube`; `up.sh`/`down.sh` refuse to reuse or remove any existing pod/volume of the stack that lacks it
  (nothing is removed if any check fails). `up.sh`/`scan.sh` refuse to start with < 10 GB free or > 98 % disk used. `scan.sh` refuses a
  non-empty `RUN_ID` directory.
* Tests: `test_guards.sh` (36 checks, podman/df/nproc/sleep shims in a mktemp sandbox, touches no real stack) and
  `mutation_guards.sh` (23 paired mutations, all must be killed).

## Elasticsearch prerequisites (no sudo)

* `vm.max_map_count` is read from `/proc`; measured 1048576 here (>= 262144 required). If a host has less, `up.sh` falls back to
  `-Dnode.store.allow_mmap=false` through `SONAR_SEARCH_JAVAADDITIONALOPTS` (slower, documented) - bootstrap checks are **not** disabled.
* nofile: container started with `--ulimit nofile=131072:131072` (host hard limit must be >= 65535, checked).
* **Disk**: Elasticsearch refuses to work above its 95 % flood-stage watermark; SonarQube then aborts at boot with
  `Background initialization failed`. Measured: this host's `/` is **98 % used (37 GB free)** and the first start failed exactly
  that way. When the volume store is >= 90 % used, `up.sh` adds `-Dcluster.routing.allocation.disk.threshold_enabled=false`
  (hook read by SonarQube's `EsSettings`). Risk: ES will not stop writing if the disk fills; the index of two projects is small,
  but free disk space is the real fix.

## Results (`docs/qa/sonarqube-20261105/<RUN_ID>/`)

`<project>.issues.json` (all open issues, `reported_total` vs `exported` recorded), `.measures.json`, `.issue_facets.json`,
`.hotspots.json`, `.quality_gate.json`, `.ce_task.json`, `.scanner.txt` (scrubbed log; `.txt` because `*.log` is gitignored and `SHA256SUMS`
must verify from the committed tree), `gate_status.txt`, `run.json` (git HEAD, URL, `es_disk_threshold_disabled`, measured disk %/free GB, load),
`image_digests.txt` (digests of the sonarqube and postgres containers actually running, plus the pinned scanner), `scrub_control.txt`, `SHA256SUMS`.
All three images are pinned by digest (sonarqube and postgres identical to `docker-compose.security.yml`).
Secret scrub: every secret value in `sonar.env` is replaced by `[REDACTED]`, then verified with a control needle (a planted token
must vanish, a clean file must stay clean) and a final scan of every exported file.

## Honest boundaries

* The gate is SonarQube's default "Sonar way", which judges NEW code only. Measured in the committed (old, first-run) evidence: in **run1** both
  projects' `quality_gate.json` show `status: OK` with `conditions: []` (no new-code period on a first analysis, so OK is vacuous); in **run2**
  both show `status: OK` with one condition, `new_violations: OK`. A gate OK is therefore NOT a statement that the code base is clean: the issue
  lists hold 16 BLOCKER + 9 CRITICAL (web) and 666 CRITICAL (api). Read the issue/measure exports, not just the gate.
* run1 vs run2 were two analyses on the SAME SonarQube instance (re-analysis of an unchanged tree): identical per-project issue sets show
  stability of re-analysis only. They do NOT demonstrate fresh-instance reproducibility; that needs a run on a freshly created instance.
* Community Edition: no branch analysis, no taint-analysis rules of paid editions. No coverage report is fed in (not run here).
* Findings are exported as found; sources are never edited to hide them. Static-analysis counts are not proof the product works (not a substitute for live QA, section 11.4.289).
* The scanner container mounts the repo read-only and writes its work dir under a temp dir that is deleted afterwards.

## Status of the committed evidence (read this)

`docs/qa/sonarqube-20261105/run1` and `run2` are the OLD first run, produced with images that are no longer in the local podman store and
with the pre-review scripts. See `docs/qa/sonarqube-20261105/EVIDENCE_NOTE.md`. Their `SHA256SUMS` omit the two gitignored scanner logs and
verify from the committed tree (`cd run1 && sha256sum -c SHA256SUMS`). New runs write `*.scanner.txt` instead. A re-run is owed.

## Docker Hub rate limit

All three images are pinned by digest in `lib.sh` (sonarqube and postgres identical to `docker-compose.security.yml`). `up.sh` / `scan.sh`
use a local copy when present; otherwise they pull. On Docker Hub's anonymous pull limit (`toomanyrequests`) they stop **before creating
anything** with an explicit message. Remedies: `podman login docker.io` (your own interactive session; this service never stores
credentials), or pre-pull / `podman load` the pinned images and re-run. Covered by `test_guards.sh` T9/T9b (no network: podman shim) and
mutations M13/M14. Note: the first re-run attempt after review failed exactly this way once the local images had been pruned.

## Tracker id

The tracker id for this work is `CAT-QA-SONAR-1`. The first commit (`8c3ebe9a`) carries the provisional trailer `Roadmap-Item: QA-SONARQUBE`;
history is not rewritten, so read that trailer as `CAT-QA-SONAR-1`.
