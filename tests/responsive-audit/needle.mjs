import { chromium } from 'playwright-core'; import { oracleFn } from './oracle.mjs'; import fs from 'node:fs';
const bad = `<body style="margin:0"><p style="position:absolute;top:600px;left:0">flush text</p><div style="width:2000px;height:20px;background:#ccc">wide</div>
<button style="width:20px;height:20px">x</button><p style="font-size:8px">tiny text</p>
<div style="position:absolute;top:200px;left:0;width:100px;overflow:hidden;white-space:nowrap">this text is clipped by its overflow hidden box for sure</div>
<span style="position:absolute;top:300px;left:10px">overlap A overlap</span><span style="position:absolute;top:303px;left:12px">overlap B overlap</span>
<a href="#" style="position:absolute;top:400px;left:300px;width:100px;height:50px;display:block">covered link</a><div style="position:absolute;top:390px;left:290px;width:200px;height:80px;background:red;z-index:5"></div>
<button style="position:absolute;top:500px;left:-60px;width:100px;height:50px">offscreen</button></body>`;
const good = `<body style="margin:0;font-size:16px;padding:0 16px"><div style="width:150px"><span>Download</span> <span>1 file available wrapped text second</span></div><main style="max-width:100%"><p>Hello world readable text</p><button style="min-width:48px;min-height:48px">OK</button></main></body>`;
const b = await chromium.launch(); const p = await b.newPage({viewport:{width:375,height:700}});
const r = {};
for (const [n,h] of [['bad',bad],['good',good]]) { await p.setContent(h); r[n] = await p.evaluate(oracleFn,{touch:true,cap:8}); }
const need = ['h_overflow','tap_target_small','small_text','clipped_text','overlapping_text','occluded_control','offscreen_control','no_gutter'];
const miss = need.filter(t=>!r.bad['n_'+t]); const fp = Object.keys(r.good).filter(k=>k.startsWith('n_'));
const out = {bad_counts:Object.fromEntries(Object.entries(r.bad).filter(([k])=>k.startsWith('n_'))), good_counts:Object.fromEntries(fp.map(k=>[k,r.good[k]])), missed_in_bad:miss, false_positive_on_good:fp, verdict: (miss.length===0&&fp.length===0)?'ORACLE_VALID':'ORACLE_BROKEN'};

{ const mctx = await b.newContext({viewport:{width:375,height:700},isMobile:true,hasTouch:true}); const mp = await mctx.newPage(); await mp.setContent('<meta name=viewport content="width=device-width,initial-scale=1"><body style="margin:0"><div style="width:900px;height:20px;background:#ccc">wide</div>');
  const mr = await mp.evaluate(oracleFn,{touch:true,cap:4,vw:375,vh:700}); out.mobile_overflow_detected = !!mr.n_h_overflow; if(!mr.n_h_overflow) out.verdict='ORACLE_BROKEN'; await mctx.close(); }
console.log(JSON.stringify(out,null,1)); fs.writeFileSync('../out/oracle-needle-verdict.json',JSON.stringify(out,null,1)); await b.close();
