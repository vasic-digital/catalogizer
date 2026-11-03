import { chromium, firefox, webkit } from 'playwright-core';
import fs from 'node:fs'; import path from 'node:path'; import crypto from 'node:crypto';
import { oracleFn } from './oracle.mjs';
const args = Object.fromEntries(process.argv.slice(2).map(a=>{const [k,v]=a.replace(/^--/,'').split('=');return [k,v??'1']}));
const W = process.env.AUDIT_WORK ? path.resolve(process.env.AUDIT_WORK) : path.resolve('..');
const API = process.env.AUDIT_API_URL || 'http://127.0.0.1:18400';
const WEBBASE = process.env.AUDIT_WEB_URL || 'http://127.0.0.1:18401'; const OUT = path.join(W,'out'); fs.mkdirSync(OUT,{recursive:true});
const browserName = args.browser||'chromium', appName = args.app||'catalog-web', profile = args.profile||'full';
const B = {chromium,firefox,webkit}[browserName];
const secrets = Object.fromEntries(fs.readFileSync(path.join(W,'run/secrets.env'),'utf8').trim().split('\n').map(l=>l.split(/=(.*)/s).slice(0,2)));

const BASE_VPS = [[320,568,3],[360,640,3],[375,812,3],[414,896,3],[768,1024,2],[820,1180,2],[1024,768,1],[1280,800,1],[1440,900,1],[1920,1080,1],[2560,1440,1],[3840,2160,1]];
const LAND = [[568,320,3],[640,360,3],[812,375,3],[896,414,3],[1180,820,2]];
const VPS = args.matrix==='followup' ? [[360,640,3],[390,844,3],[768,1024,2],[1024,768,1],[1440,900,1],[1920,1080,1]] : BASE_VPS.concat(LAND);
const KEY = [[375,812],[768,1024],[1280,800]];
const appNameEarly = args.app||'catalog-web';
const apps = {
 'catalog-web': { base:WEBBASE, pages: [
   ['login','/login',false],['register','/register',false],['forgot-password','/forgot-password',false],
   ['dashboard','/dashboard',true],['media','/media',true],['analytics','/analytics',true],['subtitles','/subtitles',true],['collections','/collections',true],
   ['favorites','/favorites',true],['playlists','/playlists',true],['conversion','/conversion',true],['admin','/admin',true],['browse','/browse',true],
   ['entity-movie','/entity/6',true],['entity-album','/entity/7',true],['settings','/settings',true],['ai','/ai',true],['identities','/identities',true],
   ['dashboard+menu','/dashboard',true,'menu'] ] },
 'website': { base:'http://127.0.0.1:18402', pages: (appNameEarly==='website' ? JSON.parse(fs.readFileSync(path.join(W,'website-pages.json'),'utf8')) : []).map(p=>[p.replace(/[^a-z0-9]+/gi,'_').replace(/^_|_$/g,'')||'index',p,false]) },
 'installer': { base:'http://127.0.0.1:18403', pages: [['wizard-0-welcome','/',false,'step0'],['wizard-1','/',false,'step1'],['wizard-2','/',false,'step2'],['wizard-3','/',false,'step3'],['wizard-4','/',false,'step4']] },
 'desktop': { base:'http://127.0.0.1:18404', pages: [['root','/',false],['login','/login',false],['settings','/settings',false],['library','/library',false],['search','/search',false],['media-detail','/media/1',false]] },
};
const app = apps[appName]; if (args.base) app.base = args.base; const TAG = args.tag ? '-'+args.tag : '';
const themes = (browserName==='chromium' || args.matrix==='followup') ? ['light','dark'] : ['light'];
async function token(){ const r = await fetch(API+'/api/v1/auth/login',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({username:secrets.ADMIN_USERNAME,password:secrets.ADMIN_PASSWORD})}); return r.json(); }
const red = t => String(t).replace(/eyJ[A-Za-z0-9_\-]+\.[A-Za-z0-9_\-.]*/g,'<JWT>').replace(/token=[^&\s'"]+/g,'token=<redacted>');
const sha = f => crypto.createHash('sha256').update(fs.readFileSync(f)).digest('hex');

const cases = []; // [vp, dpr, theme, motion]
const ap = profile==='quick';
for (const [w,h,d] of VPS) for (const th of themes) cases.push({w,h,dpr:d,theme:th,motion:'no-preference'});
if (browserName!=='chromium' && args.matrix!=='followup') for (const [w,h,d] of VPS) if (KEY.some(k=>k[0]===w)||w===320) cases.push({w,h,dpr:d,theme:'dark',motion:'no-preference'});
// dpr sweep on key viewports (chromium only), and reduced-motion pass
if (browserName==='chromium' && args.matrix!=='followup') { for (const [w,h] of KEY) for (const d of [1,2,3]) { if (!VPS.find(v=>v[0]===w&&v[1]===h&&v[2]===d)) cases.push({w,h,dpr:d,theme:'light',motion:'no-preference'}); }
  for (const [w,h] of KEY) cases.push({w,h,dpr:VPS.find(v=>v[0]===w&&v[1]===h)[2],theme:'light',motion:'reduce'}); }
const vpf = args.vps ? args.vps.split(',').map(Number) : null;
if (vpf) for (let i=cases.length-1;i>=0;i--) if(!vpf.includes(cases[i].w)) cases.splice(i,1);
const only = args.only ? args.only.split(',') : null;
const jl = path.join(OUT,`verdicts-${appName}-${browserName}${TAG}.jsonl`);
if (!args.append) fs.writeFileSync(jl,'');
const tok = (app.pages.some(p=>p[2]) || (appName==='desktop' && args.authed)) ? await token() : null;
const browser = await B.launch({ args: browserName==='chromium'?['--disable-gpu']:[] });
let n=0, bad=0; const t0=Date.now();
for (const c of cases) {
  const touch = c.w<=1024 && !(c.w===1024&&c.h===768&&false);
  const ctxOpts = { viewport:{width:c.w,height:c.h}, deviceScaleFactor:c.dpr, colorScheme:c.theme, reducedMotion:c.motion, hasTouch: touch, isMobile: touch && c.w<=900 && browserName!=='firefox' };
  const ctx = await browser.newContext(ctxOpts);
  if (appName==='desktop' && args.authed && tok) await ctx.addInitScript(([t,th])=>{ // emulate ONLY the Tauri IPC bridge; every HTTP call still goes to the real API
    window.__TAURI_INTERNALS__ = { metadata:{currentWindow:{label:'main'},currentWebview:{label:'main',windowLabel:'main'}}, transformCallback:(cb)=>{const id=Math.floor(Math.random()*1e9);window['_'+id]=cb;return id;},
      invoke: async (cmd)=> cmd==='get_config' ? {server_url:API,auth_token:t,theme:th,auto_start:false} : null };
  }, [tok.session_token, c.theme]);
  if (tok) await ctx.addInitScript(([t,u])=>{ try{localStorage.setItem('auth_token',t);localStorage.setItem('user',u);}catch{} }, [tok.session_token, JSON.stringify(tok.user)]);
  for (const [name,route,authed,act] of app.pages) {
    if (only && !only.includes(name)) continue;
    const page = await ctx.newPage(); const errs=[];
    page.on('console',m=>{ if(m.type()==='error') errs.push({k:'console',t:red(m.text()).slice(0,200)}); });
    page.on('pageerror',e=>errs.push({k:'pageerror',t:red(e).slice(0,200)}));
    page.on('response',r=>{ if(r.status()>=400) errs.push({k:'http',t:r.status()+' '+red(r.url().replace(/^https?:\/\/[^/]+/,'')).slice(0,120)}); });
    page.on('requestfailed',r=>{ errs.push({k:'reqfail',t:(r.failure()?.errorText||'')+' '+red(r.url().replace(/^https?:\/\/[^/]+/,'')).slice(0,120)}); });
    const rec = { app:appName, page:name, route, browser:browserName, vp:`${c.w}x${c.h}`, dpr:c.dpr, theme:c.theme, motion:c.motion, touch };
    try {
      await page.goto(app.base+route,{waitUntil:'domcontentloaded',timeout:20000});
      await page.waitForLoadState('networkidle',{timeout:8000}).catch(()=>{});
      if (act==='menu') { const b = page.locator('header button.md\\:hidden').first(); if (await b.count()) { await b.click({timeout:3000}).catch(()=>{}); await page.waitForTimeout(400); } }
      if (act && act.startsWith('step')) { const k=+act.slice(4); for (let i=0;i<k;i++){ const nb=page.getByRole('button',{name:/^next$/i}); if(!(await nb.count())||await nb.isDisabled()){ // try to select a choice that enables Next
            const opt=page.locator('input[type=radio],input[type=checkbox],[role=radio],[role=option]').first(); if(await opt.count()) await opt.click({timeout:2000}).catch(()=>{}); }
          if (await nb.count() && !(await nb.isDisabled())) { await nb.click({timeout:3000}).catch(()=>{}); await page.waitForTimeout(500);} else { rec.note=`could not advance to step ${k} (reached ${i})`; break; } } }
      if (appName!=='website') { const gone = await page.waitForFunction(()=>!/Your media collection, organized|Made with/.test(document.body.innerText)||document.body.innerText.length>400,null,{timeout:15000}).then(()=>true).catch(()=>false); rec.splashCleared=gone; await page.waitForLoadState('networkidle',{timeout:5000}).catch(()=>{}); }
      await page.waitForTimeout(c.motion==='reduce'?200:700);
      // Determinism (11.4.50): measure only once layout is at rest. Finite CSS/WAAPI animations must be finished and the
      // geometry of every control must be identical across two samples 150ms apart (framer-motion animates via rAF, so
      // getAnimations() alone does not see it). Gives up after 4s and records rec.unsettled instead of measuring a moving page.
      rec.unsettled = await page.evaluate(async () => {
        const snap = () => document.getAnimations().filter((a) => a.playState === 'running' && a.effect && a.effect.getTiming().iterations !== Infinity).length + '|' +
          Array.from(document.querySelectorAll('a,button,input,select,textarea,[role=tab],[role=button]')).map((e) => { const r = e.getBoundingClientRect(); return [r.x, r.y, r.width, r.height].map((n) => n.toFixed(2)).join(','); }).join(';');
        const t0 = Date.now(); let prev = snap();
        while (Date.now() - t0 < 4000) { await new Promise((res) => setTimeout(res, 150)); const cur = snap(); if (cur === prev && !/^[1-9]/.test(cur)) return false; prev = cur; }
        return true;
      });
      const res = await page.evaluate(oracleFn, {touch, cap:8, vw:c.w, vh:c.h});
      const dir = path.join(OUT,appName+TAG,browserName); fs.mkdirSync(dir,{recursive:true});
      const f = path.join(dir,`${name}__${c.w}x${c.h}__dpr${c.dpr}__${c.theme}__${c.motion}.png`);
      await page.screenshot({path:f});
      rec.tag = args.tag||'baseline'; rec.finalUrl = page.url().replace(app.base,''); rec.shot = path.relative(W,f); rec.sha256 = sha(f);
      const types = ['h_overflow','offscreen_control','occluded_control','tap_target_small','small_text','clipped_text','text_off_viewport','overlapping_text','no_gutter'];
      rec.counts = Object.fromEntries(Object.entries(res).filter(([k])=>k.startsWith('n_')).map(([k,v])=>[k.slice(2),v]));
      rec.findings = res.findings; rec.docScrollWidth=res.docScrollWidth; rec.textLen=res.textLen;
      rec.errors = errs.slice(0,6); rec.nErrors = errs.length;
      const fail = [];
      for (const t of types) if (rec.counts[t]) fail.push(t);
      if (res.textLen<15) fail.push('blank_page');
      if (rec.splashCleared===false) fail.push('splash_stuck');
      if (authed && /^\/login/.test(rec.finalUrl)) fail.push('auth_redirect');
      if (errs.some(e=>e.k==='pageerror')) fail.push('pageerror');
      if (errs.some(e=>e.k==='console'||e.k==='http'||e.k==='reqfail')) fail.push('console_or_network_errors');
      rec.status = fail.length?'FAIL':'PASS'; rec.failTypes = fail;
    } catch (e) { rec.status='ERROR'; rec.failTypes=['harness_error']; rec.err=String(e).slice(0,300); }
    if (rec.status!=='PASS') bad++;
    fs.appendFileSync(jl, JSON.stringify(rec)+'\n'); n++;
    await page.close();
  }
  await ctx.close();
  console.log(`${appName}/${browserName} ${c.w}x${c.h} dpr${c.dpr} ${c.theme} ${c.motion}: done n=${n} nonpass=${bad} t=${Math.round((Date.now()-t0)/1000)}s`);
}
await browser.close(); console.log('RUN_DONE',n,bad);
