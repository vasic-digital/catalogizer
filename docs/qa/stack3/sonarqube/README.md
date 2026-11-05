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

UI: `http://192.168.1.115:19000` (login `admin`; generated password and scan token are in
`~/.cache/claude-tmp/catsonar/sonar.env`, mode 0600, **outside the repo**; never printed, never committed). Credentials are passed
to curl via a 0600 netrc and to the scanner via a 0600 env-file, never in argv.

## Isolation and caps

* Rootless podman only; no sudo. Own pod `catsonar` -> own network namespace; the UI port is published on **one** LAN address
  (`192.168.1.115:19000`, free high port) only. No custom podman network is created, so the shared aardvark-dns and the
  `catqa*` stacks are not touched.
* `--memory/--memory-swap/--cpus`: SonarQube 4g (swap = memory, i.e. none) / 2 cpus (JVM heaps: web 768m, CE 1g, ES 512m), Postgres 512m / 1 cpu, scanner 6g / 2 cpus (JVM -Xmx5g; the Go analyzer OOMed at -Xmx1g and -Xmx3g on catalog-api: measured).
* Run the scripts under `systemd-run --user --scope -p MemoryMax=5G -p MemorySwapMax=0` for a host-side cap as well.

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
`.hotspots.json`, `.quality_gate.json`, `.ce_task.json`, `.scanner.log`, `gate_status.txt`, `run.json` (git HEAD, URL),
image digests, `scrub_control.txt`, `SHA256SUMS`.
Secret scrub: every secret value in `sonar.env` is replaced by `[REDACTED]`, then verified with a control needle (a planted token
must vanish, a clean file must stay clean) and a final scan of every exported file.

## Honest boundaries

* The gate is SonarQube's default "Sonar way", which judges NEW code only; measured: `conditions: []` in the exported gate JSON (no new-code period on a first analysis), so `OK` is vacuous. A gate `OK` is therefore NOT a statement that the code base is clean: measured run1/run2 gate `OK` for both projects while the issue lists hold 16 BLOCKER + 9 CRITICAL (web) and 666 CRITICAL (api). Read the issue/measure exports, not just the gate.
* Community Edition: no branch analysis, no taint-analysis rules of paid editions. No coverage report is fed in (not run here).
* Findings are exported as found; sources are never edited to hide them. Static-analysis counts are not proof the product works (not a substitute for live QA, section 11.4.289).
* The scanner container mounts the repo read-only and writes its work dir under a temp dir that is deleted afterwards.
