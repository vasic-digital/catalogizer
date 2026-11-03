// Live-evidence recorder for Node / Playwright tests (constitution 11.4.289(D)). Zero dependencies.
// Usage: const r = new Recorder({testId, command}); r.setArtifact(fp); await r.resolveAnswering('127.0.0.1:8080', {expectedExeSha256});
//        r.observe('GET', 200); r.finish('PASS', ['sig-run1','sig-run2','sig-run3']); r.write(dir)
// Verify with scripts/test-infra/verify_live_evidence.sh. Guide: docs/scripts/verify_live_evidence.md
import fs from 'node:fs';
import path from 'node:path';
import crypto from 'node:crypto';

export const SCHEMA = 'catalogizer.live-evidence/1';
export const SKIP_REASONS = ['hardware_not_present', 'credentials_absent', 'topology_unsupported', 'operator_attended', 'structurally_impossible', 'feature_disabled_by_config', 'artifact_not_yet_built'];

export function sha256File(p) {
  const h = crypto.createHash('sha256');
  h.update(fs.readFileSync(p));
  return h.digest('hex');
}
const sha256Str = (s) => crypto.createHash('sha256').update(s).digest('hex');

function procNetListeners(port) {
  const out = [];
  for (const f of ['/proc/net/tcp', '/proc/net/tcp6']) {
    let txt;
    try { txt = fs.readFileSync(f, 'utf8'); } catch { continue; }
    for (const line of txt.split('\n').slice(1)) {
      const c = line.trim().split(/\s+/);
      if (c.length < 10 || c[3] !== '0A') continue; // LISTEN
      if (parseInt(c[1].split(':')[1], 16) === port) out.push(c[9]); // inode
    }
  }
  return out;
}

// Find the process that holds the LISTEN socket on `port` (same user only; others are unreadable -> null, never guessed).
export function findListener(port) {
  const inodes = new Set(procNetListeners(port));
  if (inodes.size === 0) return null;
  for (const pid of fs.readdirSync('/proc').filter((d) => /^\d+$/.test(d))) {
    let fds;
    try { fds = fs.readdirSync(`/proc/${pid}/fd`); } catch { continue; }
    for (const fd of fds) {
      let l;
      try { l = fs.readlinkSync(`/proc/${pid}/fd/${fd}`); } catch { continue; }
      const m = /^socket:\[(\d+)\]$/.exec(l);
      if (m && inodes.has(m[1])) {
        try {
          const exe = fs.realpathSync(`/proc/${pid}/exe`);
          const cmd = fs.readFileSync(`/proc/${pid}/cmdline`, 'utf8').split('\0').filter(Boolean).join(' ');
          return { kind: 'process', pid: Number(pid), exe_path: exe, exe_sha256: sha256File(exe), cmdline: cmd };
        } catch { return null; }
      }
    }
  }
  return null;
}

export class Recorder {
  constructor({ testId, command }) {
    this.rec = { schema: SCHEMA, test_id: testId, class: 'LIVE', verdict: 'FAIL', timestamp: new Date().toISOString(), command,
      artifact: {}, endpoint: {}, answering: null, expected: null,
      requests: { total: 0, methods: {} }, responses: { status_histogram: {} }, runs: { n: 0, identical: false, signature: '' } };
  }
  setArtifact(fingerprint, source = 'target') { this.rec.artifact = { fingerprint, source }; }
  observe(method, status) {
    const r = this.rec; r.requests.total++; r.requests.methods[method] = (r.requests.methods[method] || 0) + 1;
    r.responses.status_histogram[String(status)] = (r.responses.status_histogram[String(status)] || 0) + 1;
  }
  setExpected({ exeSha256, imageDigest }) { this.rec.expected = exeSha256 ? { exe_sha256: exeSha256 } : { image_digest: imageDigest }; }
  setAnsweringContainer({ image, imageDigest, containerId }) { this.rec.answering = { kind: 'container', image, image_digest: imageDigest, container_id: containerId }; }
  // Resolve host:port to the answering process. Leaves answering null (=> verifier reclassifies UNIT) when it cannot be resolved.
  resolveAnswering(hostport, { expectedExeSha256 } = {}) {
    const i = hostport.lastIndexOf(':');
    const host = hostport.slice(0, i), port = Number(hostport.slice(i + 1));
    this.rec.endpoint = { host, port, resolved_ip: host };
    this.rec.answering = findListener(port);
    if (expectedExeSha256) this.setExpected({ exeSha256: expectedExeSha256 });
    return this.rec.answering;
  }
  // signatures: one canonical verdict string per run; identical iff all equal.
  finish(verdict, signatures) {
    this.rec.verdict = verdict;
    const ident = signatures.length > 0 && signatures.every((s) => s === signatures[0]);
    this.rec.runs = { n: signatures.length, identical: ident, signature: 'sha256:' + sha256Str(signatures.join('\n')) };
    return this.rec;
  }
  skip(reason) {
    if (!SKIP_REASONS.includes(reason)) throw new Error(`skip reason not in closed set: ${reason}`);
    this.rec.verdict = 'SKIP'; this.rec.skip_reason = reason; this.rec.answering = null; this.rec.expected = null;
    return this.rec;
  }
  write(dir) {
    fs.mkdirSync(dir, { recursive: true });
    const p = path.join(dir, `${this.rec.test_id.replace(/[^A-Za-z0-9._-]/g, '_')}.json`);
    fs.writeFileSync(p, JSON.stringify(this.rec, null, 2) + '\n');
    return p;
  }
}
