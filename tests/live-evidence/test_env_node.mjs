// Environment injection (constitution 11.4.289(D), cooperative-producer threat model) for the Node recorder: every key of the closed set TABLE.env_keys is detected,
// an empty value is not, an unreadable environ is env_checked:false (never guessed clean), and a relative script token never binds. Usage: node test_env_node.mjs
import assert from 'node:assert/strict';
import fs from 'node:fs'; import os from 'node:os'; import path from 'node:path';
import { loadInterpTable, envOf, scriptOf } from './index.mjs';
const t = loadInterpTable(); let n = 0;
const env = (...kv) => Buffer.from(kv.join('\0') + '\0');
const WANT = ['NODE_OPTIONS', 'JDK_JAVA_OPTIONS', 'JAVA_TOOL_OPTIONS', 'RUBYOPT', 'PERL5OPT', 'PERL5LIB', 'PYTHONPATH', 'PYTHONSTARTUP', 'LUA_INIT', 'BASH_ENV', 'ENV', 'PHPRC', 'PHP_INI_SCAN_DIR', 'LD_PRELOAD', 'LD_AUDIT'];
assert.deepEqual([...t.env_keys].sort(), [...WANT].sort(), 'the closed key set is exactly the mandated one'); n++;
for (const k of WANT) {
  assert.deepEqual(envOf(1, t, () => env('A=b', `${k}=x`)), { env_checked: true, env_injected: [k] }, `key ${k} detected`); n++;
  assert.deepEqual(envOf(1, t, () => env(`${k}=`)), { env_checked: true }, `empty ${k} is not injection`); n++;
  assert.deepEqual(envOf(1, t, () => env(`X${k}=x`, `${k}X=x`)), { env_checked: true }, `lookalike of ${k} is not injection`); n++;
}
assert.deepEqual(envOf(1, t, () => env('PATH=/bin', 'HOME=/h')), { env_checked: true }); n++;
assert.deepEqual(envOf(1, t, () => { throw new Error('EACCES'); }), { env_checked: false }); n++;
assert.deepEqual(envOf(1, t, () => env('NODE_OPTIONS=a', 'LD_PRELOAD=b', 'NODE_OPTIONS=c')), { env_checked: true, env_injected: ['LD_PRELOAD', 'NODE_OPTIONS'] }); n++;
const d = fs.mkdtempSync(path.join(os.tmpdir(), 'lev-env-')); fs.writeFileSync(path.join(d, 'server.js'), 'x\n'); process.chdir(d); // a REAL file at the relative path: only the absolute-only rule keeps it unbound
assert.notEqual(scriptOf(process.pid, '/usr/bin/node', ['node', path.join(d, 'server.js')]), null, 'control: the absolute form binds'); n++;
assert.equal(scriptOf(process.pid, '/usr/bin/node', ['node', 'server.js']), null, 'relative token never binds'); n++;
assert.equal(scriptOf(process.pid, '/usr/bin/node', ['node', './server.js']), null); n++;
console.log(`node env/binding tests: ${n} passed`);
