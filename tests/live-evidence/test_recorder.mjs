// Tests of tests/live-evidence/index.mjs against a REAL separate process (python3 http.server) on loopback.
// The carrier case (11.4.289(D) "loopback and test-started real services stay LIVE") must verify as LIVE; a registered double must not.
import { spawn, spawnSync } from 'node:child_process';
import fs from 'node:fs'; import os from 'node:os'; import path from 'node:path'; import http from 'node:http';
import assert from 'node:assert/strict';
import { Recorder, sha256File, findListener } from './index.mjs';

const root = path.resolve(path.dirname(new URL(import.meta.url).pathname), '../..');
const verifier = path.join(root, 'scripts/test-infra/verify_live_evidence.sh');
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'lev-node-'));
const py = fs.realpathSync(spawnSync('which', ['python3']).stdout.toString().trim());
let fails = 0; const t = async (name, fn) => { try { await fn(); console.log('ok', name); } catch (e) { fails++; console.error('FAIL', name, e.message); } };
const get = (port) => new Promise((res, rej) => http.get({ host: '127.0.0.1', port, path: '/' }, (r) => { r.resume(); r.on('end', () => res(r.statusCode)); }).on('error', rej));
const verify = (p, extra = []) => spawnSync(verifier, [...extra, p], { encoding: 'utf8' });
const wait = async (port) => { for (let i = 0; i < 50; i++) { try { await get(port); return; } catch { await new Promise((r) => setTimeout(r, 100)); } } throw new Error('server not up'); };

const port = 20000 + Math.floor(Math.random() * 20000);
const srv = spawn(py, ['-m', 'http.server', String(port), '--bind', '127.0.0.1'], { cwd: tmp, stdio: 'ignore' });
try {
  await wait(port);
  await t('findListener resolves the real server pid/exe/hash', () => {
    const a = findListener(port);
    assert.equal(a.pid, srv.pid); assert.equal(a.exe_path, py); assert.equal(a.exe_sha256, sha256File(py));
  });
  await t('control needle: a port nobody listens on resolves to null', () => assert.equal(findListener(port === 65000 ? 65001 : 65000), null));
  await t('carrier: real loopback service stays LIVE', async () => {
    const r = new Recorder({ testId: 'carrier', command: 'node test_recorder.mjs' });
    r.setArtifact('sha256:' + sha256File(py));
    r.resolveAnswering(`127.0.0.1:${port}`, { expectedExeSha256: sha256File(py) });
    for (let i = 0; i < 3; i++) r.observe('GET', await get(port));
    r.finish('PASS', ['ok', 'ok', 'ok']);
    const v = verify(r.write(tmp)); assert.equal(v.status, 0, v.stdout + v.stderr); assert.match(v.stdout, /class=LIVE/);
    assert.equal(r.rec.responses.status_histogram['200'], 3);
  });
  await t('registered double (command line) is reclassified UNIT', async () => {
    const r = new Recorder({ testId: 'double', command: 'x' });
    r.setArtifact('sha256:' + sha256File(py));
    r.resolveAnswering(`127.0.0.1:${port}`, { expectedExeSha256: sha256File(py) });
    r.rec.answering.cmdline += ' tests/jev/fixtures/jev_server.py';
    r.finish('PASS', ['ok', 'ok', 'ok']);
    const v = verify(r.write(tmp)); assert.equal(v.status, 1); assert.match(v.stdout, /registered_double/);
  });
  await t('unresolvable answering (nobody listens) is UNIT', () => {
    const r = new Recorder({ testId: 'nobody', command: 'x' });
    r.setArtifact('sha256:abc'); r.resolveAnswering('127.0.0.1:65000', { expectedExeSha256: 'abc' });
    r.finish('PASS', ['ok', 'ok', 'ok']);
    const v = verify(r.write(tmp)); assert.equal(v.status, 1); assert.match(v.stdout, /answering_identity_missing/);
  });
  await t('skip with closed reason verifies; unknown reason throws', () => {
    const r = new Recorder({ testId: 'skipper', command: 'x' }); r.skip('credentials_absent');
    r.rec.runs = { n: 1, identical: true, signature: 'sha256:0' };
    const v = verify(r.write(tmp)); assert.equal(v.status, 0); assert.match(v.stdout, /class=SKIP/);
    assert.throws(() => r.skip('lazy'));
  });
} finally { srv.kill(); fs.rmSync(tmp, { recursive: true, force: true }); }
console.log(fails ? `node recorder tests: ${fails} FAILED` : 'node recorder tests: all passed');
process.exit(fails ? 1 : 0);
