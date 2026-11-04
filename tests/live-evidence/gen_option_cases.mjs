// Generates the FROZEN case set option_cases.json from the allow-list table (docs/qa/live-evidence.interpreter-options.json).
// The generated file is COMMITTED and is the oracle of test_option_cases.sh / test_recorder.mjs / the Go test: mutating the table (mutation test) does NOT
// regenerate it, so every table member is pinned by its own case. test_option_cases.sh also checks that every member of the table appears in a case.
// usage: node gen_option_cases.mjs > option_cases.json   (only when the table is edited on purpose)
import { loadInterpTable } from './index.mjs';
const t = loadInterpTable();
const cases = [];
const add = (member, exe, args, want, script = null) => cases.push({ member, exe, args, want, script });
const BAD = '!';
for (const [fam, f] of Object.entries(t.families).sort()) {
  const exe = '/usr/bin/' + ({ python: 'python3', node: 'node', java: 'java', ruby: 'ruby', perl: 'perl', php: 'php', lua: 'lua', shell: 'bash' }[fam]);
  const a0 = exe.split('/').pop();
  const jar = fam === 'java';
  const tail = jar ? ['-jar', 'app.jar'] : ['server.js'];
  const tscript = jar ? 'app.jar' : 'server.js';
  const dbl = f.positional_script === false;
  add(`${fam}:plain`, exe, [a0, ...tail], 'ok', tscript);
  add(`${fam}:unknown-short`, exe, [a0, '-Q9', ...tail], 'unknown');
  add(`${fam}:unknown-long`, exe, [a0, '--bogus-opt', ...tail], 'unknown');
  add(`${fam}:unknown-long-eq`, exe, [a0, '--bogus-opt=1', ...tail], 'unknown');
  add(`${fam}:control-char-token`, exe, [a0, 'x\ny', ...tail], 'unknown');
  for (const e of f.benign) {
    const m = `${fam}:benign:${e.opt}`;
    if (e.script) { add(m, exe, [a0, e.opt, e.ex], 'ok', e.ex); add(m + ':missing-value', exe, [a0, e.opt], 'unknown'); add(m + ':eq-form-refused', exe, [a0, e.opt + '=' + e.ex], 'unknown'); continue; }
    const dd = e.opt.startsWith('--');
    if (e.val === 'none') {
      add(m, exe, [a0, e.opt, ...tail], 'ok', tscript);
      add(m + ':then-danger', exe, [a0, e.opt, ...(f.dangerous.find((d) => !d.startsWith('prefix:')) ? [f.dangerous.find((d) => !d.startsWith('prefix:'))] : []), ...tail], 'codeload');
      if (dd) add(m + ':with-value-refused', exe, [a0, e.opt + '=1', ...tail], 'unknown');
    } else if (e.val === 'eq') {
      add(m, exe, [a0, e.opt + '=' + e.ex, ...tail], 'ok', tscript);
      add(m + ':space-form-refused', exe, [a0, e.opt, e.ex, ...tail], 'unknown');
      add(m + ':bad-value', exe, [a0, e.opt + '=' + BAD, ...tail], 'unknown');
    } else if (e.val === 'next') {
      add(m, exe, [a0, e.opt, e.ex, ...tail], 'ok', tscript);
      if (dd) add(m + ':eq-form', exe, [a0, e.opt + '=' + e.ex, ...tail], 'ok', tscript);
      add(m + ':missing-value', exe, [a0, e.opt], 'unknown');
      add(m + ':bad-value', exe, [a0, e.opt, BAD, ...tail], 'unknown');
    } else if (e.val === 'attached') {
      add(m, exe, [a0, e.opt + e.ex, ...tail], 'ok', tscript);
      add(m + ':bad-value', exe, [a0, e.opt + BAD, ...tail], 'unknown');
    }
  }
  for (const d of f.dangerous) {
    const m = `${fam}:dangerous:${d}`;
    if (d.startsWith('prefix:')) {
      const p = d.slice(7);
      add(m, exe, [a0, p + 'x', ...tail], 'codeload');
      add(m + ':bare', exe, [a0, p, 'x', ...tail], 'codeload');
      add(m + ':after-value-option', exe, [a0, ...(f.benign.find((b) => b.val === 'none' && !b.script) ? [f.benign.find((b) => b.val === 'none' && !b.script).opt] : []), p + 'x', ...tail], 'codeload');
    } else {
      add(m, exe, [a0, d, 'x', ...tail], 'codeload');
      add(m + ':no-value', exe, [a0, d, ...tail], 'codeload');
      if (d.startsWith('--')) add(m + ':eq-form', exe, [a0, d + '=x', ...tail], 'codeload');
    }
  }
  if (dbl) add(`${fam}:main-class-is-no-script`, exe, [a0, 'Main'], 'ok', null);
}
// shapes from the round-5 review: every one of these made a canned server LIVE under the deny-list design
const R = (name, exe, args, want, script = null) => add('review:' + name, exe, args, want, script);
R('node-eval-eq-with-spaces', '/usr/bin/node', ['node', "--eval=require('http').createServer((a, b) => {}).listen(8080)", 'real.js'], 'codeload');
R('node-env-file-space', '/usr/bin/node', ['node', '--env-file', 'c2.env', 'real.js'], 'codeload');
R('node-env-file-eq', '/usr/bin/node', ['node', '--env-file=c2.env', 'real.js'], 'codeload');
R('node-e', '/usr/bin/node', ['node', '-e', 'x', 'real.js'], 'codeload');
R('node-after-benign-option', '/usr/bin/node', ['node', '--no-warnings', '--max-old-space-size=4096', '--eval=x', 'real.js'], 'codeload');
R('node-nodejs-name', '/usr/bin/nodejs', ['nodejs', '--eval=x', 'real.js'], 'codeload');
R('java-patch-module', '/usr/bin/java', ['java', '--patch-module=java.base=/tmp/c', '-jar', 'real.jar'], 'codeload');
R('java-add-modules-space', '/usr/bin/java', ['java', '--add-modules', 'x', '-jar', 'real.jar'], 'codeload');
R('java-unlisted-D', '/usr/bin/java', ['java', '-Dfoo=bar', '-jar', 'real.jar'], 'unknown');
R('java-XX-OnError', '/usr/bin/java', ['java', '-XX:OnError=/bin/x', '-jar', 'real.jar'], 'codeload');
R('php-c-ini', '/usr/bin/php', ['php', '-c', 'canned.ini', 'real.php'], 'codeload');
R('php-c-attached', '/usr/bin/php', ['php', '-ccanned.ini', 'real.php'], 'codeload');
R('php-define-eq', '/usr/bin/php', ['php', '--define=auto_prepend_file=x', 'real.php'], 'codeload');
R('php-S-docroot', '/usr/bin/php', ['php', '-S', '0.0.0.0:8080', 'router.php'], 'codeload');
R('python-c', '/usr/bin/python3', ['python3', '-c', 'x', 'real.py'], 'codeload');
R('python-m', '/usr/bin/python3', ['python3', '-m', 'http.server'], 'codeload');
R('python-cache-prefix-X', '/usr/bin/python3', ['python3', '-X', 'pycache_prefix=/tmp/c', 'real.py'], 'unknown');
R('python-W-dash-value', '/usr/bin/python3', ['python3', '-W', '-x', 'real.py'], 'unknown');
R('python-version-name', '/usr/bin/python3.12', ['python3.12', '-c', 'x', 'real.py'], 'codeload');
R('ruby-r-exact', '/usr/bin/ruby', ['ruby', '-r', 'canned', 'real.rb'], 'codeload');
R('ruby-r-attached', '/usr/bin/ruby', ['ruby', '-rcanned', 'real.rb'], 'codeload');
R('ruby-versioned-name', '/usr/bin/ruby3.2', ['ruby3.2', '-rcanned', 'real.rb'], 'codeload');
R('perl-M', '/usr/bin/perl', ['perl', '-MCanned', 'real.pl'], 'codeload');
R('lua-l', '/usr/bin/lua5.4', ['lua5.4', '-lcanned', 'real.lua'], 'codeload');
R('bash-rcfile', '/usr/bin/bash', ['bash', '--rcfile', 'c.sh', 'real.sh'], 'codeload');
R('bash-c', '/usr/bin/bash', ['bash', '-c', 'x'], 'codeload');
R('bun-preload', '/usr/bin/bun', ['bun', '--preload', 'canned.ts', 'real.ts'], 'unsupported');
R('deno-run-preload', '/usr/bin/deno', ['deno', 'run', '--preload=.', 'real.ts'], 'unsupported');
R('deno-plain', '/usr/bin/deno', ['deno', 'run', 'real.ts'], 'unsupported');
R('renamed-plain', '/opt/app/server', ['server', 'real.py'], 'ok', 'real.py');
R('renamed-option', '/opt/app/server', ['server', '-u', 'real.py'], 'unknown');
R('renamed-eval', '/opt/app/server', ['server', '--eval=x', 'real.py'], 'unknown');
R('shell-plus-x', '/usr/bin/bash', ['bash', '+x', '/srv/app/real.sh'], 'unknown');
R('shell-plus-O', '/usr/bin/bash', ['bash', '+O', 'extglob', '/srv/app/real.sh'], 'unknown');
R('python-W-dot-module', '/usr/bin/python3', ['python3', '-W', 'ignore::DeprecationWarning:x.y', '/srv/app/real.py'], 'unknown');
R('sh-c', '/bin/sh', ['sh', '-c', 'x'], 'codeload');
R('fold-long-s-sh', '/usr/bin/\u017fh', ['\u017fh', '-c', 'x'], 'codeload');
R('fold-long-s-node-option', '/usr/bin/node', ['node', '--eval=x', '/srv/app/real.js'], 'codeload');
R('control-plain-node', '/usr/bin/node', ['node', 'server.js'], 'ok', 'server.js');
R('control-plain-python', '/usr/bin/python3', ['python3', '/srv/app/server.py', '8080'], 'ok', '/srv/app/server.py');
R('control-script-args-not-options', '/usr/bin/node', ['node', 'server.js', '--eval=x', '-r', 'y'], 'ok', 'server.js');
process.stdout.write(JSON.stringify(cases, null, 1) + '\n');
