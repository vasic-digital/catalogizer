#!/usr/bin/env node
// Build catalogizer.live-evidence/1 records (tests/live-evidence/index.mjs) from N identical-protocol Playwright runs.
// Usage: node make_evidence.mjs <runsDir> <outDir> <webUrl> <apiUrl> <project>
//   <runsDir> holds run1/..runN/ each with report.json (Playwright JSON reporter) and obs/obs.jsonl (observed HTTP).
// One record per (spec file, project). Verdict PASS only when every test of the spec passed in every run; signature per run =
// sorted "title=status" lines. The artifact fingerprint is READ FROM THE TARGET now (/health body + the answering container's image digest).
import fs from 'node:fs';
import path from 'node:path';
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
const here = path.dirname(fileURLToPath(import.meta.url));
const { Recorder } = await import(path.resolve(here, '../../../tests/live-evidence/index.mjs'));

const [runsDir, outDir, webUrl, apiUrl, project] = process.argv.slice(2);
const podman = (...a) => execFileSync('podman', a, { encoding: 'utf8' }).trim();

function containerFor(name) {
  const [id, image, imageId] = podman('inspect', name, '--format', '{{.Id}}\t{{.ImageName}}\t{{.Image}}').split('\t');
  const digest = podman('image', 'inspect', imageId, '--format', '{{.Digest}}');
  return { name, id, image, digest };
}

const web = new URL(webUrl), api = new URL(apiUrl);
const webC = containerFor(process.env.LIVE_WEB_CONTAINER || 'catqa3-web'), apiC = containerFor(process.env.LIVE_API_CONTAINER || 'catqa3-api');
const health = await (await fetch(`${apiUrl}/health`)).json();
const fingerprint = `base=${webUrl};api=${apiUrl};api-image=${apiC.digest};api-health=${JSON.stringify({ version: health.version, build_number: health.build_number, build_date: health.build_date })};web-image=${webC.digest}`;
const expectedWeb = process.env.LIVE_EXPECTED_WEB_IMAGE_DIGEST;
if (!expectedWeb) throw new Error('LIVE_EXPECTED_WEB_IMAGE_DIGEST (pinned digest the claim names) is required');

const runs = fs.readdirSync(runsDir).filter((d) => /^run\d+$/.test(d)).sort();
const perRun = runs.map((r) => {
  const rep = JSON.parse(fs.readFileSync(path.join(runsDir, r, 'report.json'), 'utf8'));
  const bySpec = {};
  const walk = (suite) => {
    for (const s of suite.suites || []) walk(s);
    for (const sp of suite.specs || []) {
      for (const t of sp.tests || []) {
        if (t.projectName !== project) continue;
        const res = t.results?.[t.results.length - 1];
        const st = res?.status ?? 'skipped';
        (bySpec[suite.file || sp.file] ||= []).push(`${sp.title}=${st}`);
      }
    }
  };
  for (const s of rep.suites || []) walk(s);
  const obsFile = path.join(runsDir, r, 'obs', 'obs.jsonl');
  const obs = fs.existsSync(obsFile) ? fs.readFileSync(obsFile, 'utf8').split('\n').filter(Boolean).map((l) => JSON.parse(l)) : [];
  return { bySpec, obs };
});

// Fingerprint cross-check: EVERY test of every run recorded the build it ran against (read from /health at run time); each
// record must be an HTTP 200 with a defined build_number, and all must agree with each other and with the build read now.
for (const [i, r] of perRun.entries()) {
  const verdicts = r.obs.filter((o) => o.kind === 'verdict');
  const fps = r.obs.filter((o) => o.kind === 'fingerprint');
  if (verdicts.length === 0) throw new Error(`run${i + 1}: no verdict records (obs missing)`);
  if (fps.length !== verdicts.length) throw new Error(`run${i + 1}: ${fps.length} fingerprint records for ${verdicts.length} tests (every test must record one)`);
  const bad = fps.filter((o) => o.health_status !== 200 || !o.build_number);
  if (bad.length) throw new Error(`run${i + 1}: ${bad.length} fingerprint records without HTTP 200 / build_number (e.g. ${JSON.stringify(bad[0])})`);
}
const seenBuilds = new Set(perRun.flatMap((r) => r.obs.filter((o) => o.kind === 'fingerprint').map((o) => o.build_number)));
if (seenBuilds.size !== 1 || !seenBuilds.has(health.build_number)) {
  throw new Error(`artifact fingerprint changed during the runs: ${JSON.stringify([...seenBuilds])} vs final ${health.build_number}`);
}

const specs = [...new Set(perRun.flatMap((r) => Object.keys(r.bySpec)))].sort();
const written = [];
for (const spec of specs) {
  const rec = new Recorder({ testId: `catalog-web-live/${project}/${spec.replace(/\.live\.spec\.ts$/, '')}`, command: `catalog-web/e2e/live/run_live.sh ${project}` });
  rec.setArtifact(fingerprint, 'target');
  rec.rec.endpoint = { host: web.hostname, port: Number(web.port), resolved_ip: web.hostname, url: webUrl };
  rec.setAnsweringContainer({ image: webC.image, imageDigest: webC.digest, containerId: webC.id });
  rec.setExpected({ imageDigest: expectedWeb });
  const sigs = perRun.map((r) => (r.bySpec[spec] || []).slice().sort().join('\n'));
  for (const o of perRun[0].obs.filter((x) => x.kind === 'http' && x.spec === spec)) rec.observe(o.method, o.status);
  const allPass = perRun.every((r) => (r.bySpec[spec] || []).length > 0 && (r.bySpec[spec] || []).every((l) => l.endsWith('=passed')));
  const allSkipped = perRun.every((r) => (r.bySpec[spec] || []).length > 0 && (r.bySpec[spec] || []).every((l) => l.endsWith('=skipped')));
  if (allSkipped) rec.skip(process.env.LIVE_SKIP_REASON || 'topology_unsupported');
  else rec.finish(allPass ? 'PASS' : 'FAIL', sigs);
  rec.rec.tests = Object.fromEntries(perRun.map((r, i) => [`run${i + 1}`, (r.bySpec[spec] || []).slice().sort()]));
  written.push(rec.write(outDir));
}
console.log(written.join('\n'));
