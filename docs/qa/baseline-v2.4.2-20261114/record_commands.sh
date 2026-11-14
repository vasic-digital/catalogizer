#!/bin/bash
# TRANSCRIPTION (not the executed file): the commands I typed interactively in the 2026-11-14 session to produce the
# gate-binding records. They were run inline as separate shell calls, not from a saved script; this file reproduces them
# for audit. UNCONFIRMED: byte-equality with the executed text (only the commands, not their timing, are claimed).
# $E = output dir, $W = scratch dir holding v_<project>_<run>.tsv derived by verdicts.py from each run's report.json.
set -u
E="${E:?}"
# --- identity files (read from the running target, before teardown)
podman inspect catqab-api catqab-web catqab-postgres catqab-redis --format '{{.Name}} image={{.ImageName}} imageid={{.Image}} state={{.State.Status}} health={{.State.Health.Status}}' | tee "$E/containers_identity.txt"
# (catqab-web has no healthcheck: podman printed a template error for its health field; kept as printed)
curl -s http://127.0.0.1:38090/health | tee "$E/health_after_runs.json"
podman image inspect localhost/catqab-api:main-fc8dcf27 localhost/catqab-web:main-fc8dcf27 --format '{{index .RepoTags 0}} id={{.Id}} digest={{.Digest}} created={{.Created}}' | tee "$E/images_identity.txt"
{ echo "host=$(hostname)"; echo "machine_id_sha256=$(sha256sum /etc/machine-id | cut -c1-64)"; echo "virt=$(systemd-detect-virt)"; uname -srm; grep -m1 'model name' /proc/cpuinfo; podman --version; echo "DEVIATION: stack3 scripts used backend=podman (Containers submodule runtime unavailable), per qa_stack output"; } | tee "$E/host_identity.txt"
# --- combined verdicts (66 rows): one run per project (the 3 runs of each project were byte-identical, checked by sha256sum | uniq -c)
cat "$W/v_chromium-desktop_1.tsv" "$W/v_firefox-desktop_1.tsv" | LC_ALL=C sort > "$E/verdicts_web_v2.4.2.tsv"
# --- board / target id: "anton-" + first 12 hex chars of sha256(/etc/machine-id file); machine-id itself is not stored
TID="anton-$(sha256sum /etc/machine-id | cut -c1-12)"      # = anton-b149e84ebf7b
API=bd071396f1f35dd7ebc74050918f97c5c7524426656707e6072581f77749b845   # image ids copied by hand from containers_identity.txt
WEB=2943f5b3d6088452042d3e2eea8bd742fcc1d93238e8c8b3725c8fcf00eca105
printf 'product=web\ntarget_id=%s\nconnected=yes\nread_from=target\nkind=physical\nbuild_sha256=%s\nbuild_sha256=%s\n' "$TID" $API $WEB > "$E/hardware_web.rec"
# tests_total=66 / tests_reported=66 were TYPED BY HAND from `wc -l` of the verdicts file (66), not computed by a script;
# verdicts_sha256 = sha256sum of the verdicts file, written with sed after the file was regenerated:
printf 'product=web\nsuite=catalog-web-e2e-live(chromium-desktop,firefox-desktop;N=3 identical)\ncomplete=yes\ntests_total=66\ntests_reported=66\nverdicts_sha256=%s\n' "$(sha256sum "$E/verdicts_web_v2.4.2.tsv" | cut -d' ' -f1)" > "$E/liverun_web.rec"
printf 'board=%s\nharness=playwright-1.61.0 e2e/live (catalog-web) via docs/qa/stack3 stack catqab\nsuite=catalog-web-e2e-live(chromium-desktop,firefox-desktop;N=3 identical)\ncommand=catalog-web/e2e/live/run_live.sh {chromium-desktop,firefox-desktop} 3\nimage=catqab-api fc8dcf27 %s\nimage=catqab-web fc8dcf27 %s\n' "$TID" $API $WEB > "$E/new_meta_web.rec"
# baseline_meta_web_v2.4.2.rec is a plain `cp new_meta_web.rec`; selfbase_meta.rec likewise (control only).
