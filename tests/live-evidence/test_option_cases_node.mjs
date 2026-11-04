// Runs the FROZEN option cases (option_cases.json) through the Node recorder parser. Usage: node test_option_cases_node.mjs
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { parseInterpArgs, scriptOf } from './index.mjs';
const cases = JSON.parse(fs.readFileSync(new URL('./option_cases.json', import.meta.url), 'utf8'));
let bad = 0;
for (const c of cases) {
  const r = parseInterpArgs(c.exe, c.args);
  if (r.st !== c.want || (c.want === 'ok' && r.script !== c.script)) { bad++; console.error(`FAIL ${c.member}: ${JSON.stringify(c.args)} got ${JSON.stringify(r)} want ${c.want}/${c.script}`); }
}
// a renamed / unknown binary never gets a script bound (nothing guessed); a known one does (control needle)
const d = fs.mkdtempSync(path.join(os.tmpdir(), 'oc-')); const sc = path.join(d, 'server.js'); fs.writeFileSync(sc, 'x\n');
const bump = (m) => { bad++; console.error('FAIL ' + m); };
if (scriptOf(0, '/opt/app/server', ['server', sc]) !== null) bump('renamed binary must yield no script');
if (scriptOf(0, '/usr/bin/node', ['node', sc])?.script_path !== sc) bump('control: plain node call must bind the script');
if (scriptOf(0, '/usr/bin/node', ['node', '--env-file=c2.env', sc]) !== null) bump('--env-file=... must leave the script unbound');
fs.rmSync(d, { recursive: true });
// synthetic table: entry shapes the real table does not exercise must still be refused by the code itself
const syn = { unsupported_exe: '^zz$', families: { x: { exe: '^x$', benign: [{ opt: '--eqnore', val: 'eq' }, { opt: '--nx', val: 'next' }, { opt: '--jar', val: 'next', script: true }, { opt: '-n', val: 'none' }], dangerous: [] } } };
for (const [args, st, scr, why] of [
  [['--eqnore=5', 's'], 'ok', 's', 'eq with =value and no regex'], [['--eqnore', '5', 's'], 'unknown', null, 'eq without =value'],
  [['--nx=v', 's'], 'ok', 's', 'next = form'], [['--nx', 'v', 's'], 'ok', 's', 'next space form'], [['--nx', '-v', 's'], 'unknown', null, 'next refuses dash-led value'],
  [['--jar=a.jar'], 'unknown', null, 'script option never takes = form'], [['--jar', 'a.jar'], 'ok', 'a.jar', 'script option binds its value'], [['-n', 's'], 'ok', 's', 'control']]) {
  const r = parseInterpArgs('/usr/bin/x', ['x', ...args], syn);
  if (r.st !== st || r.script !== scr) bump(`synthetic ${why}: ${JSON.stringify(args)} got ${JSON.stringify(r)}`);
}
console.log(`node option cases: ${cases.length} run, ${bad} failed`);
process.exit(bad ? 1 : 0);
