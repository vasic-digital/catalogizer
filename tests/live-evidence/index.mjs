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

// Decode a /proc/net/tcp{,6} hex address (32-bit words little-endian) to a canonical string ("127.0.0.1" or an IPv6 form), "" when undecodable.
function procAddr(h) {
  if (h.length !== 8 && h.length !== 32) return '';
  const bytes = [];
  for (let w = 0; w < h.length; w += 8) for (let i = 3; i >= 0; i--) bytes.push(parseInt(h.slice(w + i * 2, w + i * 2 + 2), 16));
  if (bytes.length === 4) return bytes.join('.');
  const mapped = bytes.slice(0, 10).every((x) => x === 0) && bytes[10] === 255 && bytes[11] === 255;
  if (mapped) return bytes.slice(12).join('.');
  const g = []; for (let i = 0; i < 16; i += 2) g.push(((bytes[i] << 8) | bytes[i + 1]).toString(16));
  return g.join(':');
}
const isWildcard = (a) => a === '0.0.0.0' || /^(0(:|$))+$/.test(a);

// Inodes of LISTEN sockets on `port` whose bound address covers `host` (wildcard binds cover every host; a bind to another address does not).
function procNetListeners(port, host) {
  const out = [];
  const want = host === '' || host == null ? null : (host === 'localhost' ? '127.0.0.1' : host);
  for (const f of ['/proc/net/tcp', '/proc/net/tcp6']) {
    let txt;
    try { txt = fs.readFileSync(f, 'utf8'); } catch { continue; }
    for (const line of txt.split('\n').slice(1)) {
      const c = line.trim().split(/\s+/);
      if (c.length < 10 || c[3] !== '0A') continue; // LISTEN
      const [ha, hp] = c[1].split(':');
      if (parseInt(hp, 16) !== port) continue;
      const bound = procAddr(ha);
      if (want === null || isWildcard(bound) || bound === want) out.push(c[9]); // inode
    }
  }
  return out;
}

const INTERP = /^(python[0-9.]*|pypy[0-9.]*|node|nodejs|deno|bun|java|ruby[0-9.]*|perl[0-9.]*|php[0-9.]*|lua[0-9.]*|bash|sh|dash|zsh|ksh|fish)$/i;
const S = (...x) => new Set(x);
// Per interpreter family: options without value, options taking the NEXT argument, inline-code options. An option in none of them makes the
// script ambiguous (no script recorded => verifier reports UNIT); deno/bun have no table, so nothing is guessed for them.
const OPTS = {
  python: { noval: S('-u', '-B', '-E', '-s', '-S', '-I', '-O', '-OO', '-v', '-vv', '-b', '-bb', '-q', '-d', '-R', '-x', '-P'), val: S('-W', '-X'), inline: S('-c', '-m') },
  node: { noval: S('--no-warnings', '--trace-warnings', '--expose-gc', '--enable-source-maps', '--experimental-vm-modules', '--harmony', '--use-strict', '--preserve-symlinks'), val: S('-r', '--require', '--import', '--loader', '--experimental-loader', '--conditions', '-C', '--input-type', '--title'), inline: S('-e', '-p', '--eval', '--print') },
  java: { noval: S('-server', '-ea', '-da', '-verbose'), val: S('-cp', '-classpath', '--class-path', '-p', '--module-path'), inline: S() },
  ruby: { noval: S('-w', '-W0', '-W1', '-W2', '-v'), val: S('-I', '-r'), inline: S('-e') },
  perl: { noval: S('-w', '-W', '-t', '-T', '-l'), val: S('-I'), inline: S('-e', '-E') },
  php: { noval: S('-q', '-n'), val: S('-d', '-c'), inline: S('-r') },
  lua: { noval: S('-W', '-i'), val: S('-l'), inline: S('-e') },
  shell: { noval: S('-x', '-e', '-u', '-v', '-eu', '-ex', '-eux', '-l'), val: S(), inline: S('-c') },
};
function family(base) {
  const b = base.toLowerCase();
  if (b.startsWith('python') || b.startsWith('pypy')) return 'python';
  if (b === 'node' || b === 'nodejs') return 'node';
  if (['bash', 'sh', 'dash', 'zsh', 'ksh', 'fish'].includes(b)) return 'shell';
  return ['java', 'ruby', 'perl', 'php', 'lua'].find((f) => b.startsWith(f)) || '';
}
export function scriptOf(pid, exe, args) {
  const fam = family(path.basename(exe)); const t = OPTS[fam];
  if (!t) return null;
  let cand = null;
  for (let i = 1; i < args.length && cand === null; i++) {
    const a = args[i];
    if (t.inline.has(a)) return null;
    if (fam === 'java' && a === '-jar') { if (i + 1 >= args.length) return null; cand = args[i + 1]; }
    else if (t.val.has(a)) i++;
    else if (t.noval.has(a) || (a.startsWith('--') && a.includes('='))) continue;
    else if (fam === 'java' && (a.startsWith('-X') || a.startsWith('-D'))) continue;
    else if (a.startsWith('-')) return null;
    else cand = a;
  }
  if (cand === null) return null;
  try {
    if (!path.isAbsolute(cand)) cand = path.join(fs.readlinkSync(`/proc/${pid}/cwd`), cand);
    return { script_path: cand, script_sha256: sha256File(cand) };
  } catch { return null; }
}
// A language-runtime library / module path mapped into the process marks an interpreter even when its binary was renamed (heuristic; static runtimes expose none).
const RUNTIME_RE = /(libpython[0-9.]*|libjvm|libruby[0-9.]*|libperl|libphp[0-9.]*|liblua[0-9.]*|libnode|libv8|\/lib-dynload\/|\/site-packages\/|\/node_modules\/)/;
function runtimeHint(pid) {
  try { const m = RUNTIME_RE.exec(fs.readFileSync(`/proc/${pid}/maps`, 'utf8')); return m ? m[1] : ''; } catch { return ''; }
}

// Find the process that holds the LISTEN socket on host:port (same user only; others are unreadable -> null, never guessed).
// Several distinct pids holding the matching socket(s) is AMBIGUOUS: returns { error } and no identity (the first pid is never picked).
export function findListener(port, host = '') {
  const inodes = new Set(procNetListeners(port, host));
  if (inodes.size === 0) return null;
  const holders = [];
  for (const pid of fs.readdirSync('/proc').filter((d) => /^\d+$/.test(d))) {
    let fds;
    try { fds = fs.readdirSync(`/proc/${pid}/fd`); } catch { continue; }
    for (const fd of fds) {
      let l;
      try { l = fs.readlinkSync(`/proc/${pid}/fd/${fd}`); } catch { continue; }
      const m = /^socket:\[(\d+)\]$/.exec(l);
      if (m && inodes.has(m[1])) { holders.push(Number(pid)); break; }
    }
  }
  if (holders.length === 0) return null;
  if (holders.length > 1) return { error: `ambiguous listener: pids ${holders.join(',')} hold the same LISTEN socket; not choosing one` };
  const pid = holders[0];
  try {
    const exe = fs.realpathSync(`/proc/${pid}/exe`);
    const args = fs.readFileSync(`/proc/${pid}/cmdline`, 'utf8').split('\0').filter(Boolean);
    return { kind: 'process', pid, exe_path: exe, exe_sha256: sha256File(exe), cmdline: args.join(' '), ...(scriptOf(pid, exe, args) || {}), ...(runtimeHint(pid) ? { runtime_hint: runtimeHint(pid) } : {}) };
  } catch { return null; }
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
  // The real script an interpreter-hosted server must run (sha256 hex of the file).
  setExpectedScript(scriptSha256) { this.rec.expected = { ...(this.rec.expected || {}), script_sha256: scriptSha256 }; }
  setAnsweringContainer({ image, imageDigest, containerId }) { this.rec.answering = { kind: 'container', image, image_digest: imageDigest, container_id: containerId }; }
  // Resolve host:port to the answering process. Leaves answering null (=> verifier reclassifies UNIT) when it cannot be resolved.
  resolveAnswering(hostport, { expectedExeSha256 } = {}) {
    const i = hostport.lastIndexOf(':');
    const host = hostport.slice(0, i), port = Number(hostport.slice(i + 1));
    this.rec.endpoint = { host, port, resolved_ip: host };
    const f = findListener(port, host);
    if (f && f.error) { this.rec.answering = null; this.rec.resolve_error = f.error; } else this.rec.answering = f;
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
