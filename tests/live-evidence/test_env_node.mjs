// Environment injection (constitution 11.4.289(D), cooperative-producer threat model) for the Node recorder: every key of the closed set TABLE.env_keys is detected,
// an empty value is not, an unreadable environ is env_checked:false (never guessed clean), and a relative script token never binds. Usage: node test_env_node.mjs
import assert from 'node:assert/strict';
import fs from 'node:fs'; import os from 'node:os'; import path from 'node:path';
import { loadInterpTable, envOf, scriptOf } from './index.mjs';
const t = loadInterpTable(); let n = 0;
const env = (...kv) => Buffer.from(kv.join('\0') + '\0');
const WANT = ['NODE_OPTIONS',  'NODE_PATH',  'JDK_JAVA_OPTIONS',  'JAVA_TOOL_OPTIONS',  '_JAVA_OPTIONS',  'RUBYOPT',  'RUBYLIB',  'GEM_HOME',  'GEM_PATH',  'BUNDLE_GEMFILE',  'BUNDLE_PATH',  'PERL5OPT',  'PERL5LIB',  'PERLLIB',  'PYTHONPATH',  'PYTHONSTARTUP',  'PYTHONHOME',  'PYTHONUSERBASE',  'PYTHONBREAKPOINT',  'PYTHONPLATLIBDIR',  'PYTHONINSPECT',  'PYTHONWARNINGS',  'LUA_INIT',  'LUA_PATH',  'LUA_CPATH',  'BASH_ENV',  'ENV',  'SHELLOPTS',  'BASHOPTS',  'ZDOTDIR',  'PHPRC',  'PHP_INI_SCAN_DIR',  'OPENSSL_CONF',  'OPENSSL_MODULES',  'OPENSSL_ENGINES',  'LD_PRELOAD',  'LD_AUDIT',  'LD_LIBRARY_PATH',  'GCONV_PATH'];
assert.deepEqual([...t.env_keys].sort(), [...WANT].sort(), 'the closed key set is exactly the mandated one'); n++;
const PFX = ['BASH_FUNC_', 'LUA_INIT_', 'LUA_PATH_', 'LUA_CPATH_']; const LOADER = ['LD_PRELOAD',  'LD_AUDIT',  'LD_LIBRARY_PATH',  'GCONV_PATH'];
assert.deepEqual([...t.env_prefixes].sort(), [...PFX].sort(), 'the closed prefix set is exactly the mandated one'); n++;
assert.deepEqual([...t.env_loader].sort(), [...LOADER].sort(), 'the loader key set (applies to every process kind) is exactly the mandated one'); n++;
for (const p of PFX) {
  const k = p + (p === 'BASH_FUNC_' ? 'foo%%' : '5_4');
  assert.deepEqual(envOf(1, t, () => env('A=b', `${k}=x`)), { env_checked: true, env_injected: [k] }, `prefix key ${k} detected`); n++;
  assert.deepEqual(envOf(1, t, () => env(`${k}=`)), { env_checked: true }, `empty ${k} is not injection`); n++;
  assert.deepEqual(envOf(1, t, () => env(`X${k}=x`, `${p}=x`)), { env_checked: true }, `lookalike of prefix ${p} (not a prefix / bare prefix) is not injection`); n++;
}
for (const k of WANT) {
  assert.deepEqual(envOf(1, t, () => env('A=b', `${k}=x`)), { env_checked: true, env_injected: [k] }, `key ${k} detected`); n++;
  assert.deepEqual(envOf(1, t, () => env(`${k}=`)), { env_checked: true }, `empty ${k} is not injection`); n++;
  assert.deepEqual(envOf(1, t, () => env(`${k}=--require=/x.js=y`)), { env_checked: true, env_injected: [k] }, `value containing '=' still detects ${k}`); n++;
  assert.deepEqual(envOf(1, t, () => env(`X${k}=x`, `${k}X=x`)), { env_checked: true }, `lookalike of ${k} is not injection`); n++;
}
assert.deepEqual(envOf(1, t, () => env('PATH=/bin', 'HOME=/h')), { env_checked: true }); n++;
// ordinary environments are not refused: common developer / CI variables that are NOT in the closed set stay clean
assert.deepEqual(envOf(1, t, () => env('PATH=/usr/bin', 'HOME=/h', 'LANG=C.UTF-8', 'USER=u', 'SHELL=/bin/bash', 'PWD=/w', 'TERM=xterm', 'XDG_CONFIG_HOME=/h/.config', 'XDG_DATA_DIRS=/usr/share', 'GOPATH=/h/go', 'GOFLAGS=-mod=mod', 'JAVA_HOME=/jdk', 'CLASSPATH=/x.jar', 'PYTHONUNBUFFERED=1', 'PYTHONDONTWRITEBYTECODE=1', 'NODE_ENV=production', 'LD_ASSUME_KERNEL=', 'RUBY_GC_HEAP_INIT_SLOTS=1', 'BUNDLE_APP_CONFIG=/c', 'SSL_CERT_FILE=/c.pem', 'OPENSSL_FOO=1', 'GEM_HOMEX=1')), { env_checked: true }); n++;
assert.deepEqual(envOf(1, t, () => { throw new Error('EACCES'); }), { env_checked: false }); n++;
assert.deepEqual(envOf(1, t, () => env('NODE_OPTIONS=a', 'LD_PRELOAD=b', 'NODE_OPTIONS=c')), { env_checked: true, env_injected: ['LD_PRELOAD', 'NODE_OPTIONS'] }); n++;
const d = fs.mkdtempSync(path.join(os.tmpdir(), 'lev-env-')); fs.writeFileSync(path.join(d, 'server.js'), 'x\n'); process.chdir(d); // a REAL file at the relative path: only the absolute-only rule keeps it unbound
assert.notEqual(scriptOf(process.pid, '/usr/bin/node', ['node', path.join(d, 'server.js')]), null, 'control: the absolute form binds'); n++;
assert.equal(scriptOf(process.pid, '/usr/bin/node', ['node', 'server.js']), null, 'relative token never binds'); n++;
assert.equal(scriptOf(process.pid, '/usr/bin/node', ['node', './server.js']), null); n++;
console.log(`node env/binding tests: ${n} passed`);
