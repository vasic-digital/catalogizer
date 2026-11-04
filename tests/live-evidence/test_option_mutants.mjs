// Reviewer-style table mutants for the Node recorder parser, ONE MEMBER AT A TIME (remove benign / remove dangerous / move dangerous into benign):
// each must be killed by the frozen cases of exactly that member. Usage: node test_option_mutants.mjs
import fs from 'node:fs';
import { loadInterpTable, parseInterpArgs } from './index.mjs';
const cases = JSON.parse(fs.readFileSync(new URL('./option_cases.json', import.meta.url), 'utf8'));
const real = loadInterpTable();
const clone = () => JSON.parse(JSON.stringify(real));
const fails = (t, member) => cases.filter((c) => c.member === member || c.member.startsWith(member + ':'))
  .filter((c) => { const r = parseInterpArgs(c.exe, c.args, t); return r.st !== c.want || (c.want === 'ok' && r.script !== c.script); }).length;
let n = 0, killed = 0;
const chk = (name, t, member) => { n++; if (fails(t, member) > 0) killed++; else console.error('SURVIVED: ' + name); };
for (const [fam, f] of Object.entries(real.families)) {
  f.benign.forEach((e, i) => { const t = clone(); t.families[fam].benign.splice(i, 1); chk(`remove benign ${fam} ${e.opt}`, t, `${fam}:benign:${e.opt}`); });
  f.dangerous.forEach((d, i) => {
    const t = clone(); t.families[fam].dangerous.splice(i, 1); chk(`remove dangerous ${fam} ${d}`, t, `${fam}:dangerous:${d}`);
    if (!d.startsWith('prefix:')) { const t2 = clone(); t2.families[fam].dangerous.splice(i, 1); t2.families[fam].benign.push({ opt: d, val: 'none' }); chk(`move dangerous->benign ${fam} ${d}`, t2, `${fam}:dangerous:${d}`); }
  });
}
// the unsupported-exe pattern and each family's exe pattern are pinned too
for (const fam of Object.keys(real.families)) { const t = clone(); delete t.families[fam]; n++; if (cases.some((c) => c.member.startsWith(fam + ':') && (() => { const r = parseInterpArgs(c.exe, c.args, t); return r.st !== c.want || (c.want === 'ok' && r.script !== c.script); })())) killed++; else console.error('SURVIVED: remove family ' + fam); }
{ const t = clone(); t.unsupported_exe = '^$never$'; n++; if (cases.some((c) => c.want === 'unsupported' && parseInterpArgs(c.exe, c.args, t).st !== 'unsupported')) killed++; else console.error('SURVIVED: unsupported_exe'); }
console.log(`node table mutants: ${n}, killed ${killed}`);
process.exit(n >= 150 && killed === n ? 0 : 1);
